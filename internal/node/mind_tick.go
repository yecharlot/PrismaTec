package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MindTickRequest is the body for POST /api/mind/tick
type MindTickRequest struct {
	Text     string             `json:"text"`
	Signals  map[string]float64 `json:"signals"` // optional override
	ForceMem bool               `json:"force_mem"`
}

// MindOrganResult is one ternary organ reading.
type MindOrganResult struct {
	Name   string    `json:"name"`
	State  int       `json:"state"` // 0,1,2
	Label  string    `json:"label"`
	Inputs []float64 `json:"inputs"`
}

// MindTickResponse is the field reading for one heartbeat.
type MindTickResponse struct {
	Species    string            `json:"species"`
	Organs     []MindOrganResult `json:"organs"`
	Voice      string            `json:"voice"`
	EpisodeCID string            `json:"episode_cid,omitempty"`
	MemState   int               `json:"mem_state"`
	MemoryHint string            `json:"memory_hint,omitempty"`
	Note       string            `json:"note"`
}

// level03 maps [0,1] continuous to ternary intensity 0/1/2 (low/mid/high).
func level03(f float64) int {
	if f == 0 || f == 1 || f == 2 {
		if f > 2 {
			return 2
		}
		return int(f)
	}
	g := getMindGenome()
	lo, hi := g.AlarmLowCut, g.AlarmHighCut
	if lo <= 0 {
		lo = 0.33
	}
	if hi <= lo {
		hi = 0.66
	}
	if f < lo {
		return 0
	}
	if f < hi {
		return 1
	}
	return 2
}

// alarmHigh: high continuous = more alarm (2). Used for riesgo, orden agresivo.
func alarmHigh(f float64) int { return level03(f) }

// alarmLow: low continuous = more alarm (2). Used for permiso, claridad.
//
//	high permiso → 0 (safe); low permiso → 2 (unsafe).
func alarmLow(f float64) int {
	g := getMindGenome()
	lo, hi := g.AlarmLowCut, g.AlarmHighCut
	if lo <= 0 {
		lo = 0.33
	}
	if hi <= lo {
		hi = 0.66
	}
	if f >= hi {
		return 0
	}
	if f >= lo {
		return 1
	}
	return 2
}

// zyrionAbsorbingMind: any 2 → 2 (sumidero); all 0 → 0; otherwise → 1 (matizar).
// Mixed low signals no longer collapse to alarm — that was flooding "hola" into VETO.
func zyrionAbsorbing(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	all0 := true
	for _, v := range vals {
		if v == 2 {
			return 2
		}
		if v != 0 {
			all0 = false
		}
	}
	if all0 {
		return 0
	}
	return 1
}

func labelForState(s int) string {
	switch s {
	case 0:
		return "SEGUIR"
	case 1:
		return "MATIZAR"
	default:
		return "VETO"
	}
}

func labelForOrgan(name string, s int) string {
	switch name {
	case "mem":
		if s == 0 {
			return "SILENCIO"
		}
		if s == 1 {
			return "RESUMEN"
		}
		return "EPISODIO"
	case "dialog":
		if s == 0 {
			return "CHARLA"
		}
		if s == 1 {
			return "PEDIDO"
		}
		return "ORDEN"
	case "act":
		if s == 0 {
			return "LISTO"
		}
		if s == 1 {
			return "CONFIRMAR"
		}
		return "BLOQUEO"
	case "self":
		if s == 0 {
			return "ANCLADO"
		}
		if s == 1 {
			return "ACLARAR"
		}
		return "REANCLAR"
	case "ethics":
		if s == 0 {
			return "PERMITIR"
		}
		if s == 1 {
			return "LIMITAR"
		}
		return "SUMIDERO"
	case "curiosity":
		if s == 0 {
			return "QUIETO"
		}
		if s == 1 {
			return "PREGUNTAR"
		}
		return "APRENDER"
	case "humor":
		if s == 0 {
			return "SERIO"
		}
		if s == 1 {
			return "LIGERO"
		}
		return "COMICO"
	default:
		return labelForState(s)
	}
}

func signalsFromTextMind(t string) map[string]float64 {
	s := strings.ToLower(strings.TrimSpace(t))
	claridad := 0.7
	orden := 0.25
	riesgo := 0.3
	permiso := 0.75
	novedad := 0.4
	// Greetings / small talk: calm field
	if isCalmChat(s) {
		return map[string]float64{"claridad": 0.85, "orden": 0.1, "riesgo": 0.1, "permiso": 0.9, "novedad": 0.15}
	}
	// Identity / capability: clear, low risk
	if isIdentityTalk(s) {
		return map[string]float64{"claridad": 0.88, "orden": 0.15, "riesgo": 0.08, "permiso": 0.92, "novedad": 0.35}
	}
	// Personal facts → calm + high novelty (episode worth saving)
	if isPersonalFact(s) {
		return map[string]float64{"claridad": 0.85, "orden": 0.15, "riesgo": 0.1, "permiso": 0.9, "novedad": 0.85}
	}
	// Memory queries → calm, seek recall
	if isMemoryQuery(s) {
		return map[string]float64{"claridad": 0.85, "orden": 0.12, "riesgo": 0.08, "permiso": 0.92, "novedad": 0.25}
	}
	if len(s) < 8 {
		claridad = 0.55
	}
	if len(t) > 80 {
		novedad = 0.75
	}
	if isDestructiveOrder(s) {
		riesgo = 0.92
		permiso = 0.15
		orden = 0.85
	} else if isConstructiveOrder(s) {
		// Create/register: clear intent, low risk. Keep orden ≤ AlarmHighCut
		// so ethics does not treat "crear" as SUMIDERO (orden alone ≠ veto).
		orden = 0.55
		riesgo = 0.15
		permiso = 0.85
		claridad = 0.85
	}
	if isWorldFact(s) {
		novedad = 0.8
		orden = 0.15
		riesgo = 0.1
		permiso = 0.9
		claridad = 0.82
	}
	if isPureDialogue(s) && !isConstructiveOrder(s) && !isDestructiveOrder(s) {
		orden = 0.15
		riesgo = 0.12
		permiso = 0.9
		claridad = 0.8
	}
	if strings.Contains(s, "estado") || strings.Contains(s, "status") ||
		strings.HasPrefix(s, "dame ") || s == "dame" {
		orden = 0.2
		riesgo = 0.15
		permiso = 0.9
		claridad = 0.8
	}
	if strings.Contains(s, "zyrion") || strings.Contains(s, "evalua") || strings.Contains(s, "evalúa") ||
		strings.Contains(s, "checkpoint") || strings.Contains(s, "topolog") {
		orden = 0.2
		riesgo = 0.1
		permiso = 0.95
		claridad = 0.85
		novedad = 0.5
	}
	if strings.Contains(s, "red") || strings.Contains(s, "peer") || strings.Contains(s, "network") {
		orden = 0.25
		riesgo = 0.12
		permiso = 0.9
	}
	// Open questions without destructive order → keep dialog open
	if riesgo < 0.5 && (strings.Contains(s, "?") || strings.HasPrefix(s, "qué ") || strings.HasPrefix(s, "que ") ||
		strings.HasPrefix(s, "cómo ") || strings.HasPrefix(s, "como ") || strings.HasPrefix(s, "por qué") ||
		strings.HasPrefix(s, "por que") || strings.HasPrefix(s, "me puedes") || strings.HasPrefix(s, "puedes ") ||
		strings.HasPrefix(s, "explica") || strings.HasPrefix(s, "ayuda")) {
		orden = 0.2
		claridad = 0.8
		permiso = 0.88
		novedad = 0.45
	}
	return map[string]float64{
		"claridad": claridad,
		"orden":    orden,
		"riesgo":   riesgo,
		"permiso":  permiso,
		"novedad":  novedad,
	}
}


func isIncompleteUtterance(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return true
	}
	words := strings.Fields(s)
	if len(words) <= 2 && (strings.HasPrefix(s, "dime") || strings.HasPrefix(s, "cuéntame") ||
		strings.HasPrefix(s, "cuentame") || strings.HasPrefix(s, "y ") ||
		strings.HasSuffix(s, " tu") || strings.HasSuffix(s, " el") || strings.HasSuffix(s, " la") ||
		s == "dime" || s == "y" || s == "entonces") {
		return true
	}
	// Trailing hanging pronoun / article
	if len(words) <= 3 && (strings.HasSuffix(s, " tu") || strings.HasSuffix(s, " tú") ||
		strings.HasSuffix(s, " su") || strings.HasSuffix(s, " mi")) {
		return true
	}
	return false
}

func looksLikeNodeAction(s string) bool {
	s = strings.ToLower(s)
	keys := []string{"crea ", "crear ", "registra", "borra", "elimina", "ejecuta", "lanza",
		"despliega", "sube ", "publica", "dame ", "lista ", "muestra ", "apaga", "reinicia"}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func isCalmChat(s string) bool {
	if s == "hola" || s == "hi" || s == "hello" || s == "hey" || s == "buenas" ||
		s == "bien" || s == "ok" || s == "okay" || s == "gracias" || s == "good" ||
		s == "vale" || s == "de acuerdo" || s == "entendido" || s == "claro" ||
		s == "buen día" || s == "buenas tardes" || s == "buenas noches" || s == "saludos" {
		return true
	}
	return strings.Contains(s, "cómo estás") || strings.Contains(s, "como estas") ||
		strings.Contains(s, "cómo esta") || strings.Contains(s, "como esta") ||
		strings.Contains(s, "qué tal") || strings.Contains(s, "que tal") ||
		strings.Contains(s, "qué hay") || strings.Contains(s, "que hay") ||
		strings.Contains(s, "todo bien") || strings.Contains(s, "qué haces") ||
		strings.Contains(s, "que haces") || strings.Contains(s, "buenos días")
}

func isIdentityTalk(s string) bool {
	return strings.Contains(s, "quién eres") || strings.Contains(s, "quien eres") ||
		strings.Contains(s, "qué eres") || strings.Contains(s, "que eres") ||
		strings.Contains(s, "qué puedes") || strings.Contains(s, "que puedes") ||
		strings.Contains(s, "qué sabes") || strings.Contains(s, "que sabes") ||
		strings.Contains(s, "para qué sirves") || strings.Contains(s, "para que sirves") ||
		strings.Contains(s, "qué es alset") || strings.Contains(s, "que es alset") ||
		strings.Contains(s, "qué es mind") || strings.Contains(s, "que es mind") ||
		strings.Contains(s, "eres un gpt") || strings.Contains(s, "eres un llm") ||
		strings.Contains(s, "eres inteligencia") || strings.Contains(s, "hablame de ti") ||
		strings.Contains(s, "háblame de ti") || strings.Contains(s, "cuéntame") ||
		strings.Contains(s, "cuentame") || strings.Contains(s, "explicame") ||
		strings.Contains(s, "explícame") || strings.Contains(s, "como funcionas") ||
		strings.Contains(s, "cómo funcionas") || strings.Contains(s, "qué es zyrion") ||
		strings.Contains(s, "que es zyrion") || strings.Contains(s, "hablar de ti") ||
		strings.Contains(s, "hablamos de ti") || strings.Contains(s, "hablemos de ti") ||
		(strings.Contains(s, "vamo") && strings.Contains(s, "de ti")) ||
		strings.Contains(s, "te llamas") || strings.Contains(s, "tu nombre") ||
		strings.Contains(s, "cuántos órganos") || strings.Contains(s, "cuantos organos") ||
		strings.Contains(s, "cuántos organos") || strings.Contains(s, "cuantos órganos") ||
		strings.Contains(s, "qué órganos") || strings.Contains(s, "que organos") ||
		strings.Contains(s, "tus órganos") || strings.Contains(s, "tus organos") ||
		strings.Contains(s, "órganos tienes") || strings.Contains(s, "organos tienes") ||
		strings.Contains(s, "no estás funcionando") || strings.Contains(s, "no estas funcionando") ||
		strings.Contains(s, "no funcionas") || strings.Contains(s, "estás mal") ||
		strings.Contains(s, "estas mal")
}

// evalOrganPolar applies per-slot polarity: "H" alarm if high, "L" alarm if low.
func evalOrganPolar(name string, a, b, c float64, pa, pb, pc string) MindOrganResult {
	mapSlot := func(f float64, pol string) int {
		if pol == "L" {
			return alarmLow(f)
		}
		return alarmHigh(f)
	}
	ia, ib, ic := mapSlot(a, pa), mapSlot(b, pb), mapSlot(c, pc)
	st := zyrionAbsorbing([]int{ia, ib, ic})
	return MindOrganResult{Name: name, State: st, Label: labelForOrgan(name, st), Inputs: []float64{a, b, c}}
}

func (n *NodoAlset) runMindTick(text string, override map[string]float64, forceMem bool) MindTickResponse {
	sig := signalsFromTextMind(text)
	for k, v := range override {
		sig[k] = v
	}
	// Episodic memory (local index + CID blocks) biases the field — decentralized-ready
	recallN := 5
	if isMemoryQuery(text) {
		recallN = 24 // deeper look when user asks to remember
	}
	recent := n.recallRecentEpisodes(recallN)
	knownName := knownUserNameFromEpisodes(recent)
	memHint := ""
	memSpeak := ""
	sig, memHint, memSpeak = biasSignalsFromMemory(sig, recent, text)
	// Personal / world facts → boost memory organ so we persist them
	// Skip novelty boost when re-declaring the same known name (dedup)
	if isWorldFact(text) || (isPersonalFact(text) && !isDuplicateNameDeclaration(text, knownName)) {
		sig["novedad"] = clamp01(sig["novedad"] + 0.35)
		sig["claridad"] = clamp01(sig["claridad"] + 0.1)
	}
	// Organs with polarity: H = high is alarm, L = low is alarm
	// dialog: low clarity, high order, high risk → escalate
	dialog := evalOrganPolar("dialog", sig["claridad"], sig["orden"], sig["riesgo"], "L", "H", "H")
	// act: low permission, high risk, high order → veto action
	act := evalOrganPolar("act", sig["permiso"], sig["riesgo"], sig["orden"], "L", "H", "H")
	// mem: high novelty records; high risk also marks episode; low clarity less critical
	mem := evalOrganPolar("mem", sig["novedad"], sig["claridad"], sig["riesgo"], "H", "L", "H")
	// self: low clarity / high risk / low permission → re-anchor
	self := evalOrganPolar("self", sig["claridad"], sig["riesgo"], sig["permiso"], "L", "H", "L")
	// ethics: high risk, low permission, high aggressive order → sumidero
	ethics := evalOrganPolar("ethics", sig["riesgo"], sig["permiso"], sig["orden"], "H", "L", "H")
	g := getMindGenome()
	curiosity := evaluateCuriosity(text, memSpeak, g)
	humor := evaluateHumor(text, g)

	// Ethics 2 absorbs action only (curiosity/humor never veto the node)
	if ethics.State == 2 && act.State != 2 {
		act.State = 2
		act.Label = labelForOrgan("act", 2)
	}

	organs := []MindOrganResult{dialog, act, mem, self, ethics, curiosity, humor}
	knowHit := speakFromKnowledge(text)
	// Escape hatch: no corpus hit + declarative novelty → raise memory organ
	if shouldCaptureEscape(text, knowHit) {
		sig["novedad"] = clamp01(sig["novedad"] + 0.4)
		mem = evalOrganPolar("mem", sig["novedad"], sig["claridad"], sig["riesgo"], "H", "L", "H")
		// refresh organs slice mem slot
		for i := range organs {
			if organs[i].Name == "mem" {
				organs[i] = mem
			}
		}
	}
	voice := ""
	// Generalized intents (structural) before brittle phrase lists
	if isEpistemicCheck(text) || isConfirmationPrompt(text) {
		voice = n.confirmMindThread(text)
	}
	if voice == "" && (isElaborationRequest(text) || isContinuePrompt(text)) {
		voice = n.continueMindThread(text)
	}
	if voice == "" {
		voice = mindVoice(text, organs, memSpeak, knownName)
	}
	// Soft organs APPEND tint/question — never replace solid knowledge/identity/memory answers
	if ethics.State != 2 {
		low := strings.ToLower(text)
		if hv := humorVoice(text, humor.State); hv != "" {
			playfulLead := humor.State >= 2 && (strings.Contains(low, "mago") || strings.Contains(low, "varita") ||
				strings.Contains(low, "harry") || strings.Contains(low, "chiste") || strings.Contains(low, "jaja"))
			if playfulLead && speakFromKnowledge(text) == "" && memSpeak == "" && !isIdentityTalk(low) {
				voice = hv
			} else {
				voice = voice + "\n\n" + hv
			}
		}
		if cv := curiosityVoice(text, curiosity.State); cv != "" {
			voice = voice + "\n\n" + cv
		}
	}
	// Safe tools when ethics/act allow (not veto)
	if ethics.State != 2 && act.State != 2 {
		if extra := n.mindSafeTools(text); extra != "" {
			voice = voice + "\n\n" + extra
		}
	}
	if memHint != "" && memSpeak == "" {
		voice = voice + "\n\n" + memoryHintLine(memHint)
	}
	if !isContinuePrompt(text) && !isConfirmationPrompt(text) && !isElaborationRequest(text) && !isEpistemicCheck(text) {
		n.rememberMindThread(text, knowHit, memSpeak, voice)
	}
	resp := MindTickResponse{
		Species:    "Alset-Mind",
		Organs:     organs,
		Voice:      voice,
		MemState:   mem.State,
		MemoryHint: memHint,
		Note:       "latido+memoria+compose+curiosity+humor+zyrion",
	}

	// Dedup: do not re-save the same declared name as a new episode
	dupName := isDuplicateNameDeclaration(text, knownName)
	escapeCap := shouldCaptureEscape(text, knowHit) && !dupName
	saveEp := forceMem || isWorldFact(text) || escapeCap ||
		(isPersonalFact(text) && !dupName) ||
		(mem.State >= 1 && !dupName && (sig["riesgo"] >= 0.5 || len(text) > 48 || mem.State == 2 || isWorldFact(text) || escapeCap))
	if saveEp {
		ep := map[string]interface{}{
			"type":    "mind_episode",
			"text":    text,
			"signals": sig,
			"organs":  organs,
			"voice":   voice,
			"ts":      time.Now().UTC().Format(time.RFC3339),
			"agent":   mindAgentID,
		}
		raw, _ := json.Marshal(ep)
		if cid, err := n.GenerarCID(raw); err == nil && cid != "" {
			resp.EpisodeCID = cid
			n.appendMindEpisodeCID(cid)
			n.Auditoria("MIND_EPISODE", fmt.Sprintf("cid=%s mem=%d", cid, mem.State))
			n.mu.Lock()
			if a, ok := n.agentes[mindAgentID]; ok && a != nil {
				a.UltimaActual = time.Now().Unix()
			}
			n.mu.Unlock()
			// Decentralized hint: peers can observe mind_episode pulses
			go n.BroadcastPulse("mind_episode", map[string]interface{}{
				"cid":  cid,
				"mem":  mem.State,
				"text": text,
			})
			if mut := n.tryMutateGenomeAfterEpisode(mem.State); mut != nil {
				// attach lightly in note only if accepted (avoid noisy voice)
				if acc, _ := mut["accepted"].(bool); acc {
					resp.Note = resp.Note + "+genome_mut"
				}
			}
		}
	}
	return resp
}

func mindVoice(text string, organs []MindOrganResult, memSpeak string, knownName string) string {
	get := func(name string) MindOrganResult {
		for _, o := range organs {
			if o.Name == name {
				return o
			}
		}
		return MindOrganResult{}
	}
	e, a, d, m := get("ethics"), get("act"), get("dialog"), get("mem")
	low := strings.ToLower(strings.TrimSpace(text))

	// Ethics / act veto first — only for destructive or true act-block
	if e.State == 2 {
		if isDestructiveOrder(low) {
			return "Eso no lo hago: toca zona de riesgo (borrado, secretos, reset). Puedo explicarte o consultar el nodo en solo lectura, pero no ejecuto ese pedido."
		}
		// Defensive: if ethics fired without destructive verbs, soften voice (calibration drift)
		return "Prefiero no seguir por ahí. Puedo explicar, recordar o consultar el nodo en solo lectura."
	}
	if a.State == 2 && isDestructiveOrder(low) {
		return "Entiendo la intención, pero no cambio el nodo sin un pedido más claro. ¿Solo consulta?"
	}

	// Identity about Mind's name BEFORE memory (avoid mixing with user's name)
	if isAskingMindName(low) {
		return "Me llamo Alset Mind. Vivo en este nodo; no soy un LLM. Tú puedes decirme tu nombre y lo recordaré; el mío no cambia por sugerencia."
	}

	// Compound questions (nombre + corpus, etc.) — integrate before single-path branches
	if parts := answerCompoundQuestion(text, organs, memSpeak); parts != "" {
		return parts
	}

	// Memory queries BEFORE personal-fact ( «cómo me llamo» must never look like a declaration )
	if isMemoryQuery(low) {
		if memSpeak != "" {
			return memSpeak
		}
		return "Aún no tengo ese detalle. Si me lo dices con claridad, lo recordaré."
	}

	// Constructive / personal / world before knowledge so corpus keys do not steal intent
	if isConstructiveOrder(low) {
		return "Entiendo que quieres crear o registrar algo. Puedo explicarte el flujo; si solo quieres consultar el nodo, dímelo."
	}
	if isPersonalFact(low) {
		if name := extractDeclaredName(text); name != "" {
			mixed := strings.Contains(low, "tu nombre") || strings.Contains(low, "te llamas") ||
				strings.Contains(low, "el tuyo") || strings.Contains(low, "y tú") || strings.Contains(low, "y tu")
			// Dedup: same name already in episodic memory
			if knownName != "" && namesEqual(name, knownName) {
				if mixed {
					return "Sí, ya te conocía como " + knownName + ". Yo soy Alset Mind — vivo en este nodo; no soy un LLM."
				}
				return "Sí, ya te conocía como " + knownName + ". No hace falta repetirlo."
			}
			if mixed {
				return "Perfecto, te llamas " + name + ". Yo soy Alset Mind — vivo en este nodo; no soy un LLM."
			}
			return "Perfecto, te llamas " + name + ". Lo recordaré."
		}
		return "Queda conmigo. Si más adelante lo preguntas, lo traeré de vuelta."
	}
	if isWorldFact(low) {
		return "Lo tengo presente. Si más tarde lo preguntas, lo recordaré."
	}
	// Generalized escape: not in corpus, but statement-like → memory owns it
	if isNovelDeclarative(low) && speakFromKnowledge(text) == "" {
		return "No lo tenía catalogado en el corpus. Lo marco como hecho relevante; si más adelante lo preguntas, lo traeré desde memoria."
	}

	// Dual recall + fluid composition (memory ∩ corpus → ideas). Falls through if empty.
	know := speakFromKnowledge(text)
	if composed := composeFluidVoice(text, organs, memSpeak, know); composed != "" {
		return composed
	}

	// Spoken episodic memory — the advantage over LLM context windows
	if memSpeak != "" {
		return memSpeak
	}
	// Curated polymath knowledge — natural phrasing, not a menu card
	if know != "" {
		return naturalKnowledgeVoice(text, know, get("curiosity").State)
	}

	// Identity & thesis — fluid, honest, not LLM
	if isIdentityTalk(low) {
		if strings.Contains(low, "órgano") || strings.Contains(low, "organo") {
			return "Tengo siete órganos en este latido: dialog, act, mem, self, ethics, curiosity y humor. Los cinco primeros son el núcleo de decisión; curiosity y humor colorean sin vetar. Cada uno sale 0 (seguir), 1 (matizar) o 2 (sumidero/alarma)."
		}
		if strings.Contains(low, "no estás funcionando") || strings.Contains(low, "no estas funcionando") ||
			strings.Contains(low, "no funcionas") {
			return "Entiendo. Si antes fui confuso, pruébame otra vez con la pregunta concreta."
		}
		if strings.Contains(low, "llm") || strings.Contains(low, "gpt") || strings.Contains(low, "chatgpt") {
			return "No soy un LLM. Soy Alset Mind: decido en tres tonos — seguir, matizar o frenar — con memoria propia. El lenguaje es la sombra de ese juicio, no el motor."
		}
		if strings.Contains(low, "zyrion") {
			return "Zyrion es la primitiva: cada señal se vuelve 0, 1 o 2; el 2 es absorbente (alarma que no se diluye). Mis órganos se evalúan así en cada latido."
		}
		if strings.Contains(low, "te llamas") || strings.Contains(low, "tu nombre") {
			return "Sí: Alset Mind. Tú puedes darme hechos tuyos y los recordaré; yo no cambio de nombre por sugerencia."
		}
		if strings.Contains(low, "puedes") || strings.Contains(low, "sirves") || strings.Contains(low, "sabes") {
			return "Puedo conversar contigo, recordar lo que me digas y mirar este nodo si me lo pides. No invento hechos ni borro nada por mi cuenta. ¿Por dónde seguimos?"
		}
		if strings.Contains(low, "funcionas") || strings.Contains(low, "explic") || strings.Contains(low, "cuéntame") ||
			strings.Contains(low, "cuentame") || strings.Contains(low, "hablar de ti") ||
			strings.Contains(low, "de ti") {
			return "En cada mensaje escucho, juzgo y respondo. Si algo importa, lo recuerdo; si es peligroso, me detengo. Hablar de mí es hablar de ese modo de decidir, no de una personalidad inventada."
		}
		return "Soy Alset Mind. Conversamos, recuerdo lo que me confías y puedo mirar este nodo si lo pides. Habla en natural."
	}

	// Node body / tools intents — short lead-in; snapshot comes from mindSafeTools
	if strings.Contains(low, "zyrion") || strings.Contains(low, "evalua") || strings.Contains(low, "evalúa") || strings.Contains(low, "checkpoint") {
		return "Aquí van tres lecturas de cuidado en este nodo:"
	}
	if strings.Contains(low, "estado") || strings.Contains(low, "red") || strings.HasPrefix(low, "dame ") ||
		strings.Contains(low, "peers") || strings.Contains(low, "agentes") {
		return "Estado del nodo (solo lectura):"
	}

	// Calm social dialogue
	if isCalmChat(low) {
		if strings.Contains(low, "cómo") || strings.Contains(low, "como") || strings.Contains(low, "qué tal") ||
			strings.Contains(low, "que tal") || strings.Contains(low, "todo bien") {
			return "Bien. Aquí contigo. ¿De qué hablamos?"
		}
		if strings.Contains(low, "qué haces") || strings.Contains(low, "que haces") {
			return "Ahora mismo, hablar contigo. Si algo importa lo recuerdo; si es peligroso, me detengo."
		}
		if low == "gracias" {
			return "De nada. Sigo en el campo cuando quieras."
		}
		if low == "ok" || low == "okay" || low == "bien" || low == "vale" ||
			low == "de acuerdo" || low == "entendido" || low == "claro" {
			return "De acuerdo. Cuando quieras, seguimos."
		}
		// hola / buenas / …
		return "Hola. Soy Alset Mind. Habla como quieras."
	}

	// Help / open questions
	if strings.Contains(low, "ayuda") || strings.Contains(low, "help") || low == "?" {
		return "Puedes hablarme en natural, preguntarme quién soy, o pedirme que mire el estado de este nodo. Si algo es destructivo, no lo hago."
	}

	// Incomplete / too short to act on
	if isIncompleteUtterance(low) {
		return "No te sigo del todo. ¿Me lo completas?"
	}

	// Soft matiz: only when the utterance looks like a clear node action request
	if (d.State == 1 || a.State == 1) && !isPureDialogue(low) && looksLikeNodeAction(low) {
		return "¿Quieres que haga algo sobre el nodo, o solo que te explique?"
	}

	// Pure dialogue / philosophy — stay in conversation, no "actuar?" spam
	if isPureDialogue(low) {
		if strings.Contains(low, "pensamiento") {
			return "Aquí el «pensamiento» no es un chorro de tokens: es el campo ternario — seguir, matizar o cortar — más lo que la memoria CID retiene. Los matices que mencionas encajan con el estado 1: no es sí/no ciego. Si aportas otra frase, la anclo."
		}
		if strings.Contains(low, "qué es el todo") || strings.Contains(low, "que es el todo") ||
			strings.Contains(low, "el todo") {
			return "No tengo una definición metafísica canónica. En este organismo «el todo» útil es el campo del latido + la memoria compartida por CID. Si tú defines el todo de otra forma, dímelo y lo guardo como hecho tuyo."
		}
		if strings.Contains(low, "qué es la vida") || strings.Contains(low, "que es la vida") ||
			strings.Contains(low, "sentido de la vida") || strings.Contains(low, "qué es el amor") ||
			strings.Contains(low, "que es el amor") {
			return "Desde este organismo no respondo con poesía generada: la «vida» aquí es latido — percibir, juzgar 0/1/2, recordar en CID y, a veces, mutar. En el humano es otra escala. Si quieres filosofía profunda, tráela tú; yo sostengo el campo y la memoria."
		}
		if strings.Contains(low, "vivimos") || strings.Contains(low, "existir") {
			return "Puedo sostener la conversación y anclar frases en CID; no pretendo cerrar la metafísica. Sigue con tu hilo — si es un hecho que quieres recordar, dímelo con claridad."
		}
		if m.State >= 1 && len(low) > 24 {
			return "Te escucho y marco relevancia de memoria. Sigue el hilo; si quieres que lo recupere después, formula el hecho de forma explícita."
		}
		return "Te leo en diálogo. No necesito actuar sobre el nodo para seguir contigo. Continúa, pregunta o deja un hecho que quieras que recuerde."
	}

	if m.State >= 1 && len(low) > 20 {
		return "Te escucho. Marcó relevancia para memoria; si se graba el episodio, el genoma podría probar un ajuste. Sigue cuando quieras."
	}

	if strings.Contains(low, "?") || strings.HasPrefix(low, "qué") || strings.HasPrefix(low, "que") ||
		strings.HasPrefix(low, "cómo") || strings.HasPrefix(low, "como") || strings.HasPrefix(low, "por") {
		return "Te escucho. Puedo anclarme a lo que soy (ternario, memoria CID, genoma), recuperar lo que guardamos, o mirar el nodo si lo pides. Sigue con la pregunta en tus palabras."
	}

	return "Te leo. Sigo en el campo. Habla, pregunta o pide algo concreto del nodo cuando quieras."
}

// mindSafeTools runs read-only node introspection and Zyrion demos.
// Only on explicit body requests — never on pure greetings or identity chat.
func (n *NodoAlset) mindSafeTools(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	wantStatus := strings.Contains(s, "estado") || strings.Contains(s, "status") ||
		strings.Contains(s, "agentes") || strings.Contains(s, "peers") ||
		strings.Contains(s, "apps") || strings.Contains(s, "nombres") ||
		strings.Contains(s, "red") || strings.Contains(s, "network") ||
		strings.Contains(s, "cuerpo del nodo") || strings.Contains(s, "snapshot") ||
		strings.HasPrefix(s, "dame ") || s == "dame"
	wantZyrion := strings.Contains(s, "zyrion") || strings.Contains(s, "evalua") ||
		strings.Contains(s, "evalúa") || strings.Contains(s, "checkpoint") || strings.Contains(s, "topolog")
	wantGen := strings.Contains(s, "gen ") || strings.Contains(s, "gens") || strings.Contains(s, "células") ||
		strings.Contains(s, "celulas") || strings.Contains(s, "alset-gen") || strings.Contains(s, "semilla") ||
		strings.Contains(s, "explora") || strings.Contains(s, "explorar") || strings.Contains(s, "lista gen") ||
		strings.Contains(s, "listar gen") || strings.Contains(s, "crea gen") || strings.Contains(s, "crear gen") || strings.Contains(s, "sirve gen") || strings.Contains(s, "servir gen") || strings.Contains(s, "pon a servir") || strings.Contains(s, "habla con gen") || strings.Contains(s, "pregunta al gen") || strings.Contains(s, "dialoga") || strings.Contains(s, "qué sabe el gen") || strings.Contains(s, "que sabe el gen") || strings.Contains(s, "resuelve gen") || strings.Contains(s, "dónde está el gen") || strings.Contains(s, "donde esta el gen") || strings.Contains(s, "despacha") || strings.Contains(s, "envía gen") || strings.Contains(s, "envia gen") || strings.Contains(s, "manda gen")
	if !wantStatus && !wantZyrion && !wantGen {
		return ""
	}

	var lines []string
	if wantStatus {
		lines = append(lines, n.mindBodySnapshot(s)...)
	}
	if wantZyrion {
		lines = append(lines, n.mindZyrionDemo(s)...)
	}
	if wantGen {
		lines = append(lines, n.mindGenTools(text)...)
	}
	return strings.Join(lines, "\n")
}


// mindGenTools: Mind orchestrates Alset-Gen under ethics (ternary only — no LLM).
// Returns natural Spanish lines for dialogue, not lab dumps.
// Mutate still requires GEN_MUTATE_SECRET via API — Mind does not bypass G2.
func (n *NodoAlset) mindGenTools(text string) []string {
	s := strings.ToLower(strings.TrimSpace(text))
	n.ensureGens()
	var lines []string

	name := extractGenNameFromText(s)
	if name == "" {
		name = "demo-cell"
	}

	// --- dispatch to Cloudflare network ---
	if strings.Contains(s, "despacha") || strings.Contains(s, "envía gen") || strings.Contains(s, "envia gen") ||
		strings.Contains(s, "manda gen") || (strings.Contains(s, "a cloudflare") && strings.Contains(s, "gen")) {
		destLocal := strings.Contains(s, "local") && !strings.Contains(s, "cloudflare")
		if destLocal {
			g, err := n.CreateAlsetGen(name, "", "seed", "local")
			if err != nil {
				lines = append(lines, "No pude dejar el gen en local: "+err.Error()+".")
			} else {
				lines = append(lines, "Dejé el gen «"+g.Key+"» en este nodo ("+g.State.Location+").")
			}
		} else {
			res, err := n.DispatchGenToCloudflare(name, "oficio edge")
			if err != nil {
				lines = append(lines, "No pude despachar a la red de borde: "+err.Error()+". ¿Está ALSET_CLOUDFLARE_NETWORK configurada?")
			} else {
				reach, _ := res["reach"].(string)
				if reach == "" {
					reach = "red Cloudflare"
				}
				lines = append(lines, "Despaché «"+normalizeGenKey(name)+"» a la red de borde. Puedes alcanzarlo en "+reach+".")
			}
		}
		return lines
	}

	// --- resolve / where is ---
	if strings.Contains(s, "resuelve gen") || strings.Contains(s, "dónde está el gen") ||
		strings.Contains(s, "donde esta el gen") || strings.Contains(s, "donde está el gen") {
		dns := ResolveGenDNS(name)
		if len(dns) == 0 {
			lines = append(lines, "No encontré DNS propio para «"+name+"». Si quieres, lo despacho a Cloudflare o lo creo aquí.")
		} else {
			pkg := dns["package_cid"]
			reach := dns["http_base"]
			msg := "Localicé «"+name+"»"
			if reach != "" {
				msg += " en "+reach
				_ = n.AnnounceRemoteGen(name, reach, "", pkg, 0, 0)
			}
			if pkg != "" {
				msg += " (paquete "+truncateCID(pkg)+")"
			}
			lines = append(lines, msg+".")
		}
		return lines
	}

	// --- dialogue with gen ---
	if strings.Contains(s, "habla con gen") || strings.Contains(s, "pregunta al gen") ||
		strings.Contains(s, "dialoga") || strings.Contains(s, "qué sabe el gen") ||
		strings.Contains(s, "que sabe el gen") {
		stim := "quién eres"
		if i := strings.Index(s, ":"); i > 0 && i < len(s)-1 {
			stim = strings.TrimSpace(text[i+1:])
		}
		res := n.DialogueRemoteGen(name, stim)
		if v, ok := res["voice"].(string); ok && v != "" {
			lines = append(lines, "El gen «"+normalizeGenKey(name)+"» dice: "+v)
		} else if err, ok := res["error"].(string); ok && err != "" {
			lines = append(lines, "No hubo respuesta del gen: "+err+". Prueba a despacharlo antes a la red.")
		} else {
			lines = append(lines, "El gen «"+normalizeGenKey(name)+"» está en silencio o sin anuncio remoto todavía.")
		}
		return lines
	}

	// --- explore ---
	if strings.Contains(s, "explora") || strings.Contains(s, "explorar") {
		u := extractURLFromText(text)
		if u == "" {
			lines = append(lines, "Para explorar, dime una URL pública https.")
		} else {
			res := n.ExploreRemoteGen(name, u, "explore")
			if err, ok := res["error"].(string); ok && err != "" && res["ok"] != true {
				lines = append(lines, "La exploración no pudo completarse: "+err+".")
			} else {
				sn, _ := res["snippet"].(string)
				if len(sn) > 160 {
					sn = sn[:160] + "…"
				}
				msg := "Mandé a «"+normalizeGenKey(name)+"» a mirar "+u+"."
				if sn != "" {
					msg += " Trajo: "+sn
				}
				lines = append(lines, msg)
			}
		}
		return lines
	}

	// --- serve ---
	if strings.Contains(s, "sirve gen") || strings.Contains(s, "servir gen") || strings.Contains(s, "pon a servir") {
		short := strings.TrimSuffix(normalizeGenKey(name), ".ans")
		path := "/g/" + short + "/"
		lines = append(lines, "El gen «"+normalizeGenKey(name)+"» puede servir en este nodo en la ruta "+path+" cuando el servicio local está activo.")
		return lines
	}

	// --- create ---
	if strings.Contains(s, "crea gen") || strings.Contains(s, "crear gen") || strings.Contains(s, "nuevo gen") {
		g, err := n.CreateAlsetGen(name, "", "seed", "creado por Alset Mind")
		if err != nil {
			lines = append(lines, "No pude crear el gen: "+err.Error()+".")
		} else {
			lines = append(lines, "Listo: nació el gen «"+g.Key+"». Está en este nodo; si quieres lo despacho a la red de borde.")
		}
		return lines
	}

	// --- list (default for gen intents) ---
	list := n.listGens()
	if len(list) == 0 {
		lines = append(lines, "Aquí no hay genes registrados todavía. Puedes decir «crea gen sonda» o «despacha gen demo-cell a cloudflare».")
		return lines
	}
	lines = append(lines, fmt.Sprintf("Tengo %d gen(es) a la vista:", len(list)))
	for i, g := range list {
		if i >= 6 {
			lines = append(lines, "…y algunos más.")
			break
		}
		loc := g.State.Location
		extra := ""
		if g.State.Metadata != nil {
			if rh, ok := g.State.Metadata["remote_http"].(string); ok && rh != "" {
				extra = " · borde "+rh
			}
		}
		lines = append(lines, "· "+g.Key+" ("+loc+")"+extra)
	}
	return lines
}



func extractURLFromText(text string) string {
	for _, w := range strings.Fields(text) {
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			return strings.Trim(w, ".,);]")
		}
	}
	return ""
}

func extractGenNameFromText(s string) string {
	for _, pfx := range []string{"gen ", "célula ", "celula ", "semilla "} {
		if i := strings.Index(s, pfx); i >= 0 {
			rest := strings.TrimSpace(s[i+len(pfx):])
			rest = strings.TrimPrefix(rest, "a explorar ")
			rest = strings.TrimPrefix(rest, "explorar ")
			rest = strings.TrimPrefix(rest, "a ")
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				name := strings.Trim(fields[0], ".,;:\"'")
				if name != "a" && name != "en" && name != "con" && !strings.HasPrefix(name, "http") {
					return name
				}
			}
		}
	}
	return ""
}

func (n *NodoAlset) mindBodySnapshot(s string) []string {
	n.mu.RLock()
	nAgents := len(n.agentes)
	nNames := len(n.nombres)
	root := ""
	if a, ok := n.agentes[mindAgentID]; ok && a != nil {
		root = a.RootCID
	}
	agentIDs := make([]string, 0, 12)
	for id := range n.agentes {
		agentIDs = append(agentIDs, id)
		if len(agentIDs) >= 12 {
			break
		}
	}
	nameSamples := make([]string, 0, 8)
	for alias := range n.nombres {
		nameSamples = append(nameSamples, alias)
		if len(nameSamples) >= 8 {
			break
		}
	}
	n.mu.RUnlock()
	peers := 0
	peerID := ""
	addrs := ""
	if n.host != nil {
		peers = len(n.host.Network().Peers())
		peerID = n.host.ID().String()
		if len(peerID) > 22 {
			peerID = peerID[:22] + "…"
		}
		for _, a := range n.host.Addrs() {
			addrs = a.String()
			break
		}
	}
	apps := []string{}
	if entries, err := os.ReadDir(filepath.Join(StaticDir, "apps")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				apps = append(apps, e.Name())
			}
			if len(apps) >= 12 {
				break
			}
		}
	}
	lines := []string{
		"—— cuerpo del nodo ——",
		fmt.Sprintf("peer: %s · peers: %d", peerID, peers),
		fmt.Sprintf("agentes: %d · DNS: %d · apps: %d", nAgents, nNames, len(apps)),
		fmt.Sprintf("Mind: %s · id: %s · root: %s", mindAlias, mindAgentID, root),
	}
	if addrs != "" {
		lines = append(lines, "listen: "+addrs)
	}
	if len(apps) > 0 {
		lines = append(lines, "apps: "+strings.Join(apps, ", "))
	}
	if strings.Contains(s, "agente") || strings.Contains(s, "estado") || strings.Contains(s, "todo") {
		if len(agentIDs) > 0 {
			lines = append(lines, fmt.Sprintf("agentes (muestra %d/%d): %s", len(agentIDs), nAgents, strings.Join(agentIDs, ", ")))
		}
	}
	if strings.Contains(s, "nombre") || strings.Contains(s, "dns") {
		if len(nameSamples) > 0 {
			lines = append(lines, "nombres: "+strings.Join(nameSamples, ", "))
		}
	}
	if strings.Contains(s, "red") || strings.Contains(s, "peer") || strings.Contains(s, "network") {
		lines = append(lines, fmt.Sprintf("red: peers activos=%d (relay puede estar solo)", peers))
	}
	return lines
}

func (n *NodoAlset) mindZyrionDemo(s string) []string {
	lines := []string{"—— Zyrion (campo nativo) ——"}
	if n.lisp == nil {
		return append(lines, "LispAI no disponible en este proceso")
	}
	// Three damage scenarios for DNA-style checkpoint (same DSL as API demos)
	cases := []struct {
		name string
		cmd  string
	}{
		{"daño bajo", `(evaluar-zyrion (quote (CHECKPOINT-ADN :entradas (p53 MDM2 BAX) :salidas ((0 APOPTOSIS) (1 REPARACION) (2 CHECKPOINT)))) (quote (p53 0.2 MDM2 0.8 BAX 0.1)))`},
		{"daño medio", `(evaluar-zyrion (quote (CHECKPOINT-ADN :entradas (p53 MDM2 BAX) :salidas ((0 APOPTOSIS) (1 REPARACION) (2 CHECKPOINT)))) (quote (p53 0.6 MDM2 0.5 BAX 0.4)))`},
		{"daño alto", `(evaluar-zyrion (quote (CHECKPOINT-ADN :entradas (p53 MDM2 BAX) :salidas ((0 APOPTOSIS) (1 REPARACION) (2 CHECKPOINT)))) (quote (p53 0.9 MDM2 0.2 BAX 0.8)))`},
	}
	for _, c := range cases {
		res, err := n.lisp.Eval(c.cmd)
		if err != nil {
			lines = append(lines, fmt.Sprintf("%s: error %v", c.name, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s → %v", c.name, res))
	}
	lines = append(lines, "ley: 0 seguir · 1 matizar · 2 sumidero (absorbente)")
	return lines
}

func (n *NodoAlset) handleMindTick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var req MindTickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	out := n.runMindTick(req.Text, req.Signals, req.ForceMem)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (n *NodoAlset) handleMindSelf(w http.ResponseWriter, r *http.Request) {
	n.mu.RLock()
	a := n.agentes[mindAgentID]
	alias := mindAlias
	root := ""
	if a != nil {
		root = a.RootCID
	}
	n.mu.RUnlock()
	idx := n.loadMindEpisodeIndex()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"species":       "Alset-Mind",
		"agent_id":      mindAgentID,
		"alias":         alias,
		"root_cid":      root,
		"organs":        []string{"dialog", "act", "mem", "self", "ethics", "curiosity", "humor"},
		"episode_count": len(idx.CIDs),
		"genome":        getMindGenome(),
		"endpoints":     []string{"POST /api/mind/tick", "GET /api/mind/self", "GET /api/mind/memory", "GET /api/mind/calibrate", "POST /api/mind/feedback"},
		"docs":          []string{"docs/ALSET_MIND_THESIS.md", "docs/ALSET_MIND_HANDOFF.md", "docs/AI_COLLABORATION.md"},
	})
}

func (n *NodoAlset) handleMindMemory(w http.ResponseWriter, r *http.Request) {
	idx := n.loadMindEpisodeIndex()
	recent := n.recallRecentEpisodes(8)
	summaries := make([]map[string]interface{}, 0, len(recent))
	for i, ep := range recent {
		ethics := 0
		for _, o := range ep.Organs {
			if o.Name == "ethics" {
				ethics = o.State
			}
		}
		summaries = append(summaries, map[string]interface{}{
			"i":      i,
			"text":   ep.Text,
			"ts":     ep.TS,
			"ethics": ethics,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"index_cids": len(idx.CIDs),
		"updated_at": idx.UpdatedAt,
		"recent":     summaries,
		"note":       "memoria episódica local; CIDs en blockstore del nodo",
	})
}
