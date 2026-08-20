package node

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	mindEpisodeIndexFile = "mind_episodes.json"
	mindEpisodeMaxKeep   = 32
)

// mindEpisodeIndex is a local ring of recent episode CIDs (efficient, node-local).
type mindEpisodeIndex struct {
	CIDs      []string `json:"cids"`
	UpdatedAt int64    `json:"updated_at"`
}

type mindEpisodePayload struct {
	Type   string                 `json:"type"`
	Text   string                 `json:"text"`
	Signals map[string]float64    `json:"signals"`
	Voice  string                 `json:"voice"`
	TS     string                 `json:"ts"`
	Agent  string                 `json:"agent"`
	Organs []MindOrganResult      `json:"organs"`
}

func (n *NodoAlset) loadMindEpisodeIndex() mindEpisodeIndex {
	var idx mindEpisodeIndex
	b, err := os.ReadFile(mindEpisodeIndexFile)
	if err == nil {
		_ = json.Unmarshal(b, &idx)
	}
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
	if len(idx.CIDs) > mindEpisodeMaxKeep {
		idx.CIDs = idx.CIDs[len(idx.CIDs)-mindEpisodeMaxKeep:]
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(mindEpisodeIndexFile, b, 0600)
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
	if vetoStreak > 0 {
		stack := minInt(vetoStreak, g.MaxVetoStack)
		if stack <= 0 {
			stack = minInt(vetoStreak, 3)
		}
		out["riesgo"] = clamp01(out["riesgo"] + g.VetoRiskBoost*float64(stack))
		out["permiso"] = clamp01(out["permiso"] - g.VetoPermDrop*float64(stack))
		snip := lastVetoText
		if len(snip) > 60 {
			snip = snip[:60] + "…"
		}
		hints = append(hints, fmt.Sprintf("%d veto(s) reciente(s); último «%s»", vetoStreak, snip))
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

func isMemoryQuery(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	keys := []string{
		"cómo me llamo", "como me llamo", "cuál es mi nombre", "cual es mi nombre",
		"mi nombre", "cómo me llamo?", "como me llamo?",
		"qué te dije", "que te dije", "qué te conté", "que te conte",
		"te acuerdas", "te acuerda", "recuerdas", "recuerdas lo",
		"qué sabes de mí", "que sabes de mi", "qué sabes de mi",
		"me conoces", "quién soy", "quien soy",
		"qué dije antes", "que dije antes", "lo que te dije",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func isPersonalFact(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Contains(s, "me llamo") || strings.Contains(s, "mi nombre es") ||
		strings.Contains(s, "mi nombre:") || strings.HasPrefix(s, "soy ") && len(s) < 40 ||
		strings.Contains(s, "me gusta") || strings.Contains(s, "vivo en") ||
		strings.Contains(s, "mi ciudad") || strings.Contains(s, "prefiero") ||
		strings.Contains(s, "recuerda que") || strings.Contains(s, "no olvides") ||
		strings.Contains(s, "mi proyecto") || strings.Contains(s, "trabajo en")
}

// extractDeclaredName pulls a name from "me llamo X" / "mi nombre es X".
func extractDeclaredName(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	for _, pref := range []string{"me llamo ", "mi nombre es ", "mi nombre:"} {
		if i := strings.Index(low, pref); i >= 0 {
			rest := strings.TrimSpace(text[i+len(pref):])
			// first token(s) until punctuation
			for _, sep := range []string{",", ".", "!", "?", " y ", " —", " -"} {
				if j := strings.Index(strings.ToLower(rest), sep); j > 0 {
					rest = rest[:j]
				}
			}
			rest = strings.TrimSpace(rest)
			parts := strings.Fields(rest)
			if len(parts) == 0 {
				return ""
			}
			if len(parts) > 3 {
				parts = parts[:3]
			}
			return strings.Join(parts, " ")
		}
	}
	return ""
}

// speakFromMemory builds a natural reply from CID episodes when the user asks to recall.
func speakFromMemory(query string, episodes []mindEpisodePayload) string {
	if len(episodes) == 0 {
		return ""
	}
	q := strings.ToLower(strings.TrimSpace(query))
	// Name questions
	if strings.Contains(q, "llamo") || strings.Contains(q, "nombre") ||
		strings.Contains(q, "quién soy") || strings.Contains(q, "quien soy") {
		for _, ep := range episodes {
			if name := extractDeclaredName(ep.Text); name != "" {
				return "En un episodio guardado dijiste que te llamas " + name + ". Eso quedó en memoria CID, no en una ventana de tokens."
			}
		}
		if isMemoryQuery(q) {
			return "No tengo aún un episodio donde digas tu nombre. Si me dices «me llamo …», lo grabo y podré recuperarlo después."
		}
	}
	if !isMemoryQuery(q) {
		// Soft echo on strong overlap even without explicit "recuerdas"
		rel, score := bestEpisodeOverlap(query, episodes)
		if score >= 3 && rel != "" {
			snip := truncateRunes(rel, 100)
			return "Esto resuena con un episodio previo: «" + snip + "». ¿Seguimos desde ahí?"
		}
		return ""
	}
	// Explicit memory query: best episode by overlap, or any personal-fact episode
	rel, score := bestEpisodeOverlap(query, episodes)
	if score >= 1 && rel != "" {
		return "Sí — en memoria CID tengo: «" + truncateRunes(rel, 120) + "». Eso no se borra al cerrar el chat."
	}
	for _, ep := range episodes {
		if isPersonalFact(ep.Text) {
			return "Recuerdo un hecho personal que guardaste: «" + truncateRunes(ep.Text, 120) + "»."
		}
	}
	if len(episodes) > 0 {
		return "Tengo " + fmt.Sprintf("%d", len(episodes)) + " episodio(s) recientes, pero ninguno encaja del todo con esa pregunta. Prueba a recordar un detalle o vuelve a decir el hecho."
	}
	return ""
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
