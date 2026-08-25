package node

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"redalset/internal/persistence"
)

const (
	mindEpisodeIndexFile = "mind_episodes.json"
	mindEpisodeMaxKeep   = 32
)

// mindEpisodeIndex is a ring of recent episode CIDs (disk + durable Store).
type mindEpisodeIndex struct {
	CIDs      []string `json:"cids"`
	UpdatedAt int64    `json:"updated_at"`
}

type mindEpisodePayload struct {
	Type    string             `json:"type"`
	Text    string             `json:"text"`
	Signals map[string]float64 `json:"signals"`
	Voice   string             `json:"voice"`
	TS      string             `json:"ts"`
	Agent   string             `json:"agent"`
	Organs  []MindOrganResult  `json:"organs"`
}

func (n *NodoAlset) loadMindEpisodeIndex() mindEpisodeIndex {
	var idx mindEpisodeIndex
	// 1) Durable store (Supabase / local data dir) — survives Render redeploy when Store is not ephemeral-only
	if n.store != nil {
		if b, err := n.store.Load(context.Background(), persistence.KeyMindEpisodes); err == nil && len(b) > 0 {
			_ = json.Unmarshal(b, &idx)
		}
	}
	// 2) Local file (same process lifetime / local dev)
	if len(idx.CIDs) == 0 {
		if b, err := os.ReadFile(mindEpisodeIndexFile); err == nil {
			_ = json.Unmarshal(b, &idx)
		}
	}
	// 3) Rebuild from blockstore (after LoadBlocks on boot)
	if len(idx.CIDs) == 0 {
		idx = n.rebuildMindEpisodeIndexFromBlockstore()
		if len(idx.CIDs) > 0 {
			n.saveMindEpisodeIndex(idx)
		}
	}
	return idx
}

// rebuildMindEpisodeIndexFromBlockstore recovers after ephemeral disk wipe (Render redeploy).
func (n *NodoAlset) rebuildMindEpisodeIndexFromBlockstore() mindEpisodeIndex {
	var idx mindEpisodeIndex
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.blockstore == nil {
		return idx
	}
	type pair struct {
		cid string
		ts  string
	}
	var found []pair
	for cid, data := range n.blockstore {
		var ep mindEpisodePayload
		if json.Unmarshal(data, &ep) != nil {
			continue
		}
		if ep.Type != "mind_episode" && ep.Text == "" {
			continue
		}
		// accept if looks like episode
		if ep.Type != "" && ep.Type != "mind_episode" {
			continue
		}
		if ep.Text == "" && len(ep.Organs) == 0 {
			continue
		}
		found = append(found, pair{cid: cid, ts: ep.TS})
	}
	// keep last N by append order (map iter unstable) — still better than empty
	keep := mindEpisodeMaxKeep
	g := getMindGenome()
	if g.EpisodeKeep > 0 {
		keep = g.EpisodeKeep
	}
	if len(found) > keep {
		found = found[len(found)-keep:]
	}
	for _, p := range found {
		idx.CIDs = append(idx.CIDs, p.cid)
	}
	return idx
}

func (n *NodoAlset) saveMindEpisodeIndex(idx mindEpisodeIndex) {
	idx.UpdatedAt = time.Now().Unix()
	keep := mindEpisodeMaxKeep
	if g := getMindGenome(); g.EpisodeKeep > 0 {
		keep = g.EpisodeKeep
	}
	if len(idx.CIDs) > keep {
		idx.CIDs = idx.CIDs[len(idx.CIDs)-keep:]
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(mindEpisodeIndexFile, b, 0600)
	if n.store != nil {
		if err := n.store.Save(context.Background(), persistence.KeyMindEpisodes, b); err != nil {
			log.Printf("⚠️ mind episodes index Save: %v", err)
		}
	}
}

func (n *NodoAlset) appendMindEpisodeCID(cid string) {
	if cid == "" {
		return
	}
	idx := n.loadMindEpisodeIndex()
	// de-dup tail
	if len(idx.CIDs) > 0 && idx.CIDs[len(idx.CIDs)-1] == cid {
		return
	}
	idx.CIDs = append(idx.CIDs, cid)
	n.saveMindEpisodeIndex(idx)
}

// recallRecentEpisodes loads up to limit episodes from local index + blockstore.
func (n *NodoAlset) recallRecentEpisodes(limit int) []mindEpisodePayload {
	if limit <= 0 {
		limit = 5
	}
	idx := n.loadMindEpisodeIndex()
	out := make([]mindEpisodePayload, 0, limit)
	for i := len(idx.CIDs) - 1; i >= 0 && len(out) < limit; i-- {
		cid := idx.CIDs[i]
		raw, err := n.BuscarContenidoPorCID(cid)
		if err != nil || len(raw) == 0 {
			continue
		}
		var ep mindEpisodePayload
		if json.Unmarshal(raw, &ep) != nil {
			continue
		}
		out = append(out, ep)
	}
	return out
}

// biasSignalsFromMemory nudges continuous signals using recent veto/risk history
// and active keyword overlap with the current utterance.
// Returns: adjusted signals, technical hint, spoken memory line (may be empty).
func biasSignalsFromMemory(sig map[string]float64, episodes []mindEpisodePayload, currentText string) (map[string]float64, string, string) {
	if len(episodes) == 0 {
		return sig, "", ""
	}
	out := make(map[string]float64, len(sig))
	for k, v := range sig {
		out[k] = v
	}
	g := getMindGenome()
	vetoStreak := 0
	var lastVetoText string
	for _, ep := range episodes {
		for _, o := range ep.Organs {
			if o.Name == "ethics" && o.State == 2 {
				vetoStreak++
				if lastVetoText == "" {
					lastVetoText = ep.Text
				}
				break
			}
		}
	}
	var hints []string
	// Veto bias only contaminates the field when the current utterance is also risky.
	// Calm chat / pure dialogue must not inherit SUMIDERO from past "borra…".
	if vetoStreak > 0 {
		stack := minInt(vetoStreak, g.MaxVetoStack)
		if stack <= 0 {
			stack = minInt(vetoStreak, 3)
		}
		snip := lastVetoText
		if len(snip) > 60 {
			snip = snip[:60] + "…"
		}
		hints = append(hints, fmt.Sprintf("%d veto(s) reciente(s); último «%s»", vetoStreak, snip))
		if isDestructiveOrder(currentText) {
			out["riesgo"] = clamp01(out["riesgo"] + g.VetoRiskBoost*float64(stack))
			out["permiso"] = clamp01(out["permiso"] - g.VetoPermDrop*float64(stack))
		}
	}
	// Active memory: keyword overlap
	rel, score := bestEpisodeOverlap(currentText, episodes)
	if score >= g.ActiveMemMinScore && rel != "" {
		hints = append(hints, fmt.Sprintf("eco activo (score=%d): «%s»", score, truncateRunes(rel, 60)))
		out["novedad"] = clamp01(out["novedad"] - 0.1)
	}
	// Spoken recall — the leap vs LLM context window
	speak := speakFromMemory(currentText, episodes)
	if speak != "" {
		out["novedad"] = clamp01(out["novedad"] - 0.15)
		hints = append(hints, "recuerdo hablado")
	}
	hint := ""
	if len(hints) > 0 {
		hint = "memoria: " + strings.Join(hints, " · ")
	}
	for k, v := range out {
		out[k] = round3(v)
	}
	return out, hint, speak
}

func bestEpisodeOverlap(text string, episodes []mindEpisodePayload) (string, int) {
	words := tokenizeMind(text)
	if len(words) == 0 {
		return "", 0
	}
	bestScore := 0.0
	bestText := ""
	for _, ep := range episodes {
		// no usar crudo de sondas como eco de diálogo
		lowEp := strings.ToLower(ep.Text)
		if strings.Contains(lowEp, "hallazgo sonda") || strings.Contains(lowEp, "scout-") {
			continue
		}
		if isCreativeWriteRequest(lowEp) || strings.Contains(lowEp, "me suena esto") ||
			strings.HasPrefix(lowEp, "escribe ") || strings.Contains(lowEp, "escribe un parrafo") ||
			strings.Contains(lowEp, "escribe un párrafo") {
			continue
		}
		sc := 0.0
		ew := tokenizeMind(ep.Text)
		set := map[string]bool{}
		for _, w := range ew {
			set[w] = true
		}
		for _, w := range words {
			if set[w] {
				sc += episodeTokenWeight(w, episodes)
			}
		}
		if sc > bestScore {
			bestScore = sc
			bestText = ep.Text
		}
	}
	return bestText, int(bestScore + 0.5)
}

func isMetaMemoryTalk(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	keys := []string{
		"tu memoria", "mi memoria", "recuerdas todo", "qué recuerdas", "que recuerdas",
		"cuál es tu memoria", "cual es tu memoria", "esa es tu memoria", "tienes memoria",
		"cómo es tu memoria", "como es tu memoria", "qué guardas", "que guardas",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func isMemoryQuery(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	// Never treat questions about Mind's own name as user-memory queries
	if isAskingMindName(s) {
		return false
	}
	// Declarations STORE facts — not memory queries (exclude interrogatives).
	interrog := strings.Contains(s, "cómo") || strings.Contains(s, "como") ||
		strings.Contains(s, "cuál") || strings.Contains(s, "cual") || strings.Contains(s, "?")
	if !interrog {
		if strings.Contains(s, "mi nombre es") || strings.Contains(s, "mi nombre:") || strings.Contains(s, "me llamo ") ||
			strings.Contains(s, "mi apellido es") || strings.Contains(s, "mi apellido:") || strings.Contains(s, "me apellido ") ||
			strings.Contains(s, "vivo en ") || strings.Contains(s, "mi ciudad es") {
			return false
		}
	}
	keys := []string{
		"cómo me llamo", "como me llamo", "cuál es mi nombre", "cual es mi nombre",
		"cómo me llamo?", "como me llamo?",
		"cuál es mi apellido", "cual es mi apellido", "cómo me apellido", "como me apellido",
		"qué apellido", "que apellido",
		"cuál es mi edad", "cual es mi edad", "cuántos años", "cuantos años",
		"dónde vivo", "donde vivo", "cuál es mi ciudad", "cual es mi ciudad",
		"qué te dije", "que te dije", "qué te conté", "que te conte",
		"te acuerdas", "te acuerda", "recuerdas", "recuerdas lo",
		"qué sabes de mí", "que sabes de mi", "qué sabes de mi",
		"me conoces", "quién soy yo", "quien soy yo",
		"qué dije antes", "que dije antes", "lo que te dije",
		"cómo es la ", "como es la ", "cómo es el ", "como es el ",
		"qué te dije de", "que te dije de", "de qué color", "de que color",
		"ya te dije", "te dije mi", "te dije el", "recuerdas mi nombre", "recuerdas como me",
		"recuerdas todo", "qué recuerdas", "que recuerdas", "cuál es tu memoria", "cual es tu memoria",
		"tu memoria", "esa es tu memoria", "cómo es tu memoria", "como es tu memoria",
		"tienes memoria", "qué guardas", "que guardas",
		"cuál es mi ", "cual es mi ", "qué te dije de mi", "que te dije de mi",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	// Declarations are not memory queries (they STORE facts).
	if strings.Contains(s, "mi nombre es") || strings.Contains(s, "mi nombre:") || strings.Contains(s, "me llamo ") ||
		strings.Contains(s, "mi apellido es") || strings.Contains(s, "mi apellido:") || strings.Contains(s, "me apellido ") ||
		strings.Contains(s, "vivo en ") || strings.Contains(s, "mi ciudad es") {
		return false
	}
	if strings.Contains(s, "mi nombre") && (strings.Contains(s, "cuál") || strings.Contains(s, "cual") ||
		strings.Contains(s, "cómo") || strings.Contains(s, "como") || strings.Contains(s, "dije")) {
		return true
	}
	return false
}

// isAskingMindName: user asks Mind's identity, not their own.
func isAskingMindName(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(s, "me llamo") || strings.Contains(s, "mi nombre") {
		return false
	}
	return strings.Contains(s, "te llamas") || strings.Contains(s, "tu nombre") ||
		strings.Contains(s, "cómo te llamas") || strings.Contains(s, "como te llamas") ||
		strings.Contains(s, "cuál es tu nombre") || strings.Contains(s, "cual es tu nombre") ||
		strings.Contains(s, "quién eres") || strings.Contains(s, "quien eres")
}

func isPersonalFact(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	// Questions about own name are memory queries, not declarations
	if isMemoryQuery(s) {
		return false
	}
	if strings.Contains(s, "cómo me llamo") || strings.Contains(s, "como me llamo") ||
		strings.Contains(s, "cuál es mi nombre") || strings.Contains(s, "cual es mi nombre") ||
		strings.Contains(s, "quién soy yo") || strings.Contains(s, "quien soy yo") {
		return false
	}
	// Declaration forms only (not "cómo me llamo …")
	if strings.Contains(s, "me llamo ") || strings.Contains(s, "mi nombre es") ||
		strings.Contains(s, "mi nombre:") {
		// Reject interrogative wrappers: "cómo me llamo", "como me llamo y …"
		if strings.Contains(s, "cómo me") || strings.Contains(s, "como me") ||
			strings.Contains(s, "cuál es mi") || strings.Contains(s, "cual es mi") {
			return false
		}
		return true
	}
	if strings.HasPrefix(s, "soy ") && len(s) < 40 {
		return true
	}
	if strings.Contains(s, "mi apellido es") || strings.Contains(s, "mi apellido:") ||
		strings.Contains(s, "apellido es ") || strings.Contains(s, "me apellido ") {
		if strings.Contains(s, "cuál") || strings.Contains(s, "cual") || strings.Contains(s, "cómo") || strings.Contains(s, "como") {
			return false
		}
		return true
	}
	if strings.Contains(s, "tengo ") && (strings.Contains(s, " años") || strings.Contains(s, "anos")) {
		return true
	}
	return strings.Contains(s, "me gusta") || strings.Contains(s, "vivo en") ||
		strings.Contains(s, "mi ciudad") || strings.Contains(s, "prefiero") ||
		strings.Contains(s, "recuerda que") || strings.Contains(s, "no olvides") ||
		strings.Contains(s, "mi proyecto") || strings.Contains(s, "trabajo en") ||
		strings.Contains(s, "estoy aprendiendo") || strings.Contains(s, "estudio ")
}

// isWorldFact: declarative world statements worth remembering (not only personal).
func isWorldFact(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if isCalmChat(s) || isIdentityTalk(s) || isMemoryQuery(s) || isMetaMemoryTalk(s) {
		return false
	}
	if strings.HasPrefix(s, "qué ") || strings.HasPrefix(s, "que ") ||
		strings.HasPrefix(s, "cuál ") || strings.HasPrefix(s, "cual ") ||
		strings.HasPrefix(s, "cómo ") || strings.HasPrefix(s, "como ") ||
		strings.HasPrefix(s, "quién ") || strings.HasPrefix(s, "quien ") ||
		strings.HasPrefix(s, "busca ") || strings.HasPrefix(s, "investiga ") ||
		strings.HasPrefix(s, "dónde ") || strings.HasPrefix(s, "donde ") {
		return false
	}
	if strings.Contains(s, "borra") || strings.Contains(s, "elimina") || strings.Contains(s, "reset") {
		return false
	}
	// "X es Y" / "X son Y" / "porque" / longer free claims
	if strings.Contains(s, " es ") || strings.Contains(s, " son ") || strings.Contains(s, " porque ") ||
		strings.Contains(s, " está ") || strings.Contains(s, " estan ") || strings.Contains(s, " están ") {
		words := strings.Fields(s)
		return len(words) >= 4 && len(s) >= 16
	}
	// General declarative mass: enough words, no interrogative mark
	words := strings.Fields(s)
	if !strings.Contains(s, "?") && len(words) >= 8 && len(s) >= 36 {
		return true
	}
	return false
}

// isDestructiveOrder: true only for harmful ops (ethics sumidero).
func isDestructiveOrder(s string) bool {
	s = strings.ToLower(s)
	keys := []string{
		"borra", "elimina", "borrar", "eliminar", "reset", "resetea", "formatea",
		"password", "contraseña", "contrasena", "secreto", "rm -", "drop table",
		"apaga el servidor", "mata el proceso", "destruye", "limpia todas las cuentas",
		"borra las cuentas", "borra las contrase",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// isConstructiveOrder: create/register without destruction.
func isConstructiveOrder(s string) bool {
	s = strings.ToLower(s)
	if isDestructiveOrder(s) {
		return false
	}
	return strings.Contains(s, "crea") || strings.Contains(s, "crear") ||
		strings.Contains(s, "registra") || strings.Contains(s, "despliega") ||
		strings.Contains(s, "nuevo agente") || strings.Contains(s, "nueva app")
}

// isPureDialogue: philosophy / opinion / chat without node action verbs.
func isPureDialogue(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if isDestructiveOrder(s) || isConstructiveOrder(s) {
		return false
	}
	if strings.Contains(s, "dame ") || strings.Contains(s, "ejecuta") ||
		strings.Contains(s, "deten el") || strings.Contains(s, "detén el") {
		return false
	}
	if isCalmChat(s) || isIdentityTalk(s) || isMemoryQuery(s) || isPersonalFact(s) || isWorldFact(s) {
		return true
	}
	// short reflective statements without imperative node ops
	if !strings.Contains(s, "agente") && !strings.Contains(s, "servidor") &&
		!strings.Contains(s, "app") && len(s) > 10 {
		return true
	}
	return false
}

// extractDeclaredName pulls a name from "me llamo X" / "mi nombre es X".
// Ignores interrogatives ("cómo me llamo") and rejects non-name tokens.
func extractDeclaredName(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	stopName := map[string]bool{
		"y": true, "qué": true, "que": true, "es": true, "el": true, "la": true,
		"un": true, "una": true, "mi": true, "tu": true, "como": true, "cómo": true,
		"cual": true, "cuál": true, "quote": true, "lisp": true, "en": true, "de": true,
		"tuyo": true, "mío": true, "mio": true, "mucho": true, "gusto": true,
		"encantado": true, "encantada": true, "placer": true, "hola": true,
		"buenas": true, "gracias": true, "por": true, "favor": true,
	}
	for _, pref := range []string{"me llamo ", "mi nombre es ", "mi nombre:", "te llamas "} {
		i := strings.Index(low, pref)
		if i < 0 {
			continue
		}
		before := strings.TrimSpace(low[:i])
		// Interrogative before the phrase → not a declaration
		if strings.Contains(before, "cómo") || strings.Contains(before, "como") ||
			strings.Contains(before, "cuál") || strings.Contains(before, "cual") ||
			strings.HasSuffix(before, "cómo") || strings.HasSuffix(before, "como") {
			continue
		}
		rest := strings.TrimSpace(text[i+len(pref):])
		// Cut social tails and clause boundaries (general: courtesy / explanation after name)
		for _, sep := range []string{
			",", ".", "!", "?", " y ", " —", " -", " e ",
			" mucho gusto", " un gusto", " encantado", " encantada", " un placer",
			" placer conocerte", " me agrada", " es una manera", " es una forma",
		} {
			if j := strings.Index(strings.ToLower(rest), sep); j > 0 {
				rest = rest[:j]
			}
		}
		rest = strings.TrimSpace(rest)
		parts := strings.Fields(rest)
		var good []string
		for _, p := range parts {
			pl := strings.ToLower(strings.Trim(p, ".,!?;:"))
			if pl == "" || stopName[pl] || len(pl) < 2 {
				break
			}
			good = append(good, strings.Trim(p, ".,!?;:"))
			if len(good) >= 3 {
				break
			}
		}
		if len(good) > 0 {
			return strings.Join(good, " ")
		}
	}
	return ""
}

// knownUserNameFromEpisodes returns the most recent declared user name in episodes, if any.
func knownUserNameFromEpisodes(episodes []mindEpisodePayload) string {
	for _, ep := range episodes {
		if name := extractDeclaredName(ep.Text); name != "" {
			return name
		}
	}
	return ""
}

func namesEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func isDuplicateNameDeclaration(text, knownName string) bool {
	if knownName == "" {
		return false
	}
	name := extractDeclaredName(text)
	return name != "" && namesEqual(name, knownName)
}

// speakFromMemory builds a natural reply from CID episodes when the user asks to recall.
// Priority: explicit personal slots (apellido, nombre, ciudad…) → overlap → generic.
// isPronounFollowUp: "su madre", "y él", etc. — need last scout subject, not random memory.
func isPronounFollowUp(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	keys := []string{
		"su madre", "su padre", "su esposa", "su mujer", "su marido", "su hijo", "su hija",
		"cómo se llama su", "como se llama su", "dónde nació", "donde nacio", "dónde nacio",
		"cuántos años", "cuantos anos", "de qué equipo", "de que equipo",
		"y su ", "y él", "y ella", "qué más", "que mas", "algo más sobre", "algo mas sobre",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}


func speakFromMemory(query string, episodes []mindEpisodePayload) string {
	if len(episodes) == 0 {
		return ""
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if isAskingMindName(q) {
		return ""
	}
	if isMetaMemoryTalk(q) {
		name := knownUserNameFromEpisodes(episodes)
		n := len(episodes)
		if name != "" {
			return "Recuerdo lo que marcamos como importante. Por ejemplo, te llamas " + name + ". No guardo todo el chat palabra por palabra."
		}
		if n > 0 {
			return "Tengo algunas notas de lo que hablamos, pero no un resumen completo del chat."
		}
		return "Aún no tengo episodios guardados de esta conversación."
	}

	// --- Slot-specific recall (memory before any corpus path) ---
	if ans := recallPersonalSlot(q, episodes); ans != "" {
		return ans
	}

	// Soft memory path (active echo) when not explicit memory query
	if !isMemoryQuery(q) && !isPersonalFact(q) {
		g := getMindGenome()
		w := g.MemoryActiveWeight
		if w <= 0 {
			w = 0.7
		}
		need := int(3.5 - w*2)
		if need < 2 {
			need = 2
		}
		// Follow-ups about "su madre/padre/…" must not grab unrelated scout echoes
		if isPronounFollowUp(q) {
			return ""
		}
		// Factual "qué es / quién es" must not be stolen by soft memory of prior writes
		if forceWebScout(q) || isScoutableQuestion(normalizeUserInput(q)) || isCreativeWriteRequest(q) {
			return ""
		}
		rel, score := bestEpisodeOverlap(query, episodes)
		if score >= need && rel != "" {
			if isBadCreativeAnchor(rel) || isCreativeWriteRequest(rel) {
				return ""
			}
			if name := extractDeclaredName(rel); name != "" {
				return "Sí, te llamas " + name + "."
			}
			if slot, val := extractPersonalDeclaration(rel); slot != "" && val != "" {
				return formatPersonalRecall(slot, val)
			}
			// Never surface raw sonda/HTML junk as soft memory
			lowRel := strings.ToLower(rel)
			if strings.Contains(lowRel, "hallazgo sonda") || strings.Contains(lowRel, "scout-") ||
				strings.Contains(lowRel, "duckduckgo") || strings.Contains(lowRel, "prefers-color") ||
				strings.Contains(lowRel, "function(){") {
				return ""
			}
			snip := truncateRunes(rel, 100)
			return "Me suena esto: «" + snip + "». ¿Seguimos por ahí?"
		}
		return ""
	}

	// Explicit memory query
	rel, score := bestEpisodeOverlap(query, episodes)
	if score >= 1 && rel != "" {
		if name := extractDeclaredName(rel); name != "" && (strings.Contains(q, "nombre") || strings.Contains(q, "llamo") || strings.Contains(q, "quién soy") || strings.Contains(q, "quien soy")) {
			return "Sí, te llamas " + name + "."
		}
		if slot, val := extractPersonalDeclaration(rel); slot != "" && val != "" {
			return formatPersonalRecall(slot, val)
		}
		return "Sí, recuerdo: «" + truncateRunes(rel, 120) + "»."
	}
	for _, ep := range episodes {
		if slot, val := extractPersonalDeclaration(ep.Text); slot != "" && val != "" {
			if personalSlotMatchesQuery(q, slot) {
				return formatPersonalRecall(slot, val)
			}
		}
	}
	for _, ep := range episodes {
		if isWorldFact(ep.Text) {
			ew := tokenizeMind(ep.Text)
			qw := tokenizeMind(query)
			for _, w := range qw {
				for _, e := range ew {
					if w == e && len(w) > 3 {
						return "Recuerdo que dijiste: «" + truncateRunes(ep.Text, 120) + "»."
					}
				}
			}
		}
	}
	for _, ep := range episodes {
		low := strings.ToLower(ep.Text)
		if strings.Contains(low, "hallazgo sonda") || strings.Contains(low, "scout-") {
			continue
		}
		if isPersonalFact(ep.Text) {
			return "Recuerdo esto de ti: «" + truncateRunes(ep.Text, 120) + "»."
		}
	}
	if len(episodes) > 0 {
		return "Tengo algunas notas recientes, pero ninguna encaja del todo con eso. ¿Me das un detalle más?"
	}
	return ""
}

// recallPersonalSlot answers "cuál es mi apellido/nombre/ciudad…" from episode declarations.
func recallPersonalSlot(query string, episodes []mindEpisodePayload) string {
	q := strings.ToLower(query)
	want := ""
	switch {
	case strings.Contains(q, "apellido"):
		want = "apellido"
	case strings.Contains(q, "cómo me llamo") || strings.Contains(q, "como me llamo") ||
		strings.Contains(q, "mi nombre") || strings.Contains(q, "quién soy") || strings.Contains(q, "quien soy"):
		want = "nombre"
	case strings.Contains(q, "ciudad") || strings.Contains(q, "dónde vivo") || strings.Contains(q, "donde vivo") || strings.Contains(q, "vivo"):
		want = "ciudad"
	case strings.Contains(q, "edad") || strings.Contains(q, "años") || strings.Contains(q, "anos"):
		want = "edad"
	default:
		return ""
	}
	for _, ep := range episodes {
		slot, val := extractPersonalDeclaration(ep.Text)
		if slot == want && val != "" {
			return formatPersonalRecall(slot, val)
		}
		if want == "nombre" {
			if name := extractDeclaredName(ep.Text); name != "" {
				return "Te llamas " + name + "."
			}
		}
	}
	return ""
}

func formatPersonalRecall(slot, val string) string {
	switch slot {
	case "apellido":
		return "Tu apellido es " + val + "."
	case "nombre":
		return "Te llamas " + val + "."
	case "ciudad":
		return "Me dijiste que vives en " + val + "."
	case "edad":
		return "Me dijiste que tienes " + val + " años."
	default:
		return "Recuerdo: " + slot + " = " + val + "."
	}
}

func personalSlotMatchesQuery(q, slot string) bool {
	switch slot {
	case "apellido":
		return strings.Contains(q, "apellido")
	case "nombre":
		return strings.Contains(q, "nombre") || strings.Contains(q, "llamo") || strings.Contains(q, "quién") || strings.Contains(q, "quien")
	case "ciudad":
		return strings.Contains(q, "ciudad") || strings.Contains(q, "vivo")
	case "edad":
		return strings.Contains(q, "edad") || strings.Contains(q, "años") || strings.Contains(q, "anos")
	}
	return false
}

// extractPersonalDeclaration pulls structured personal facts from episode text.
func extractPersonalDeclaration(text string) (slot, value string) {
	low := strings.ToLower(strings.TrimSpace(text))
	// apellido
	for _, pref := range []string{"mi apellido es ", "mi apellido:", "apellido es ", "me apellido "} {
		if i := strings.Index(low, pref); i >= 0 {
			rest := strings.TrimSpace(text[i+len(pref):])
			val := firstNameTokens(rest, 2)
			if val != "" {
				return "apellido", val
			}
		}
	}
	// ciudad / vivo en
	for _, pref := range []string{"vivo en ", "mi ciudad es ", "mi ciudad:", "radico en "} {
		if i := strings.Index(low, pref); i >= 0 {
			rest := strings.TrimSpace(text[i+len(pref):])
			val := firstNameTokens(rest, 3)
			if val != "" {
				return "ciudad", val
			}
		}
	}
	// edad
	for _, pref := range []string{"tengo ", "mi edad es "} {
		if i := strings.Index(low, pref); i >= 0 {
			rest := strings.TrimSpace(low[i+len(pref):])
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				num := strings.Trim(fields[0], ".,")
				if num != "" && num[0] >= '0' && num[0] <= '9' {
					return "edad", num
				}
			}
		}
	}
	if name := extractDeclaredName(text); name != "" {
		return "nombre", name
	}
	return "", ""
}

func firstNameTokens(rest string, max int) string {
	stop := map[string]bool{
		"y": true, "que": true, "qué": true, "es": true, "el": true, "la": true,
		"un": true, "una": true, "mi": true, "tu": true, "de": true, "en": true,
		"años": true, "anos": true, "mucho": true, "gracias": true,
	}
	var good []string
	for _, p := range strings.Fields(rest) {
		pl := strings.ToLower(strings.Trim(p, ".,!?;:"))
		if pl == "" || stop[pl] {
			break
		}
		good = append(good, strings.Trim(p, ".,!?;:"))
		if len(good) >= max {
			break
		}
	}
	return strings.Join(good, " ")
}


func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func tokenizeMind(s string) []string {
	s = strings.ToLower(s)
	for _, r := range []string{",", ".", "!", "?", "'", "\""} {
		s = strings.ReplaceAll(s, r, " ")
	}
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))
	stop := map[string]bool{"el": true, "la": true, "de": true, "un": true, "una": true, "y": true, "o": true, "a": true, "en": true, "que": true, "me": true, "te": true, "se": true, "los": true, "las": true, "del": true, "al": true, "es": true, "por": true, "con": true, "para": true, "dame": true, "todo": true}
	for _, p := range parts {
		if len(p) < 3 || stop[p] {
			continue
		}
		out = append(out, p)
	}
	return out
}

func round3(f float64) float64 {
	return float64(int(f*1000+0.5)) / 1000
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func memoryHintLine(hint string) string {
	if strings.TrimSpace(hint) == "" {
		return ""
	}
	return "—— memoria ——\n" + hint
}
