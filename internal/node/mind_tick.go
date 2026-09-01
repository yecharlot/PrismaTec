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
	Session  string             `json:"session"` // aislamiento de memoria/hilo por cliente
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
	Actuate    *ActuateState     `json:"actuate,omitempty"` // sub-efectores bajo act
	Effect     int               `json:"effect"`            // 0 quiet | 1 prepare | 2 execute
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
	if isDestructiveOrder(s) || codeGenEthicsVeto(s) {
		riesgo = 0.92
		permiso = 0.15
		orden = 0.85
	} else if isCodeGenRequest(s) {
		orden = 0.5
		riesgo = 0.2
		permiso = 0.85
		claridad = 0.85
		novedad = 0.55
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

func isUserCorrection(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	keys := []string{"estás mal", "estas mal", "eso está mal", "eso esta mal", "incorrecto",
		"te equivocas", "no es eso", "no es así", "no es asi", "fallaste", "error"}
	for _, k := range keys {
		if s == k || strings.HasPrefix(s, k) {
			return true
		}
	}
	return false
}

func isCalmChat(s string) bool {
	if s == "hola" || s == "hi" || s == "hello" || s == "hey" || s == "buenas" ||
		s == "bien" || s == "ok" || s == "okay" || s == "gracias" || s == "good" ||
		s == "vale" || s == "de acuerdo" || s == "entendido" || s == "claro" ||
		s == "vale entendido" || s == "ok entendido" || s == "esta bien" || s == "está bien" ||
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
	s = strings.ToLower(strings.TrimSpace(s))
	// NO capturar "cuéntame X", "explícame Y", "quién fue Persona" — eso es chat/scout.
	if strings.Contains(s, "quién fue") || strings.Contains(s, "quien fue") ||
		strings.Contains(s, "quién es ") || strings.Contains(s, "quien es ") {
		// "quién eres" sí; "quién es Mandela" no
		if !strings.Contains(s, "quién eres") && !strings.Contains(s, "quien eres") &&
			!strings.Contains(s, "quién eres?") && !strings.Contains(s, "quien eres?") {
			if !(strings.Contains(s, "quién eres tú") || strings.Contains(s, "quien eres tu")) {
				// persona/tercero
				if !strings.Contains(s, "eres tú") && !strings.Contains(s, "eres tu") &&
					!strings.HasSuffix(s, "quién eres") && !strings.HasSuffix(s, "quien eres") {
					return false
				}
			}
		}
	}
	keys := []string{
		"quién eres", "quien eres", "qué eres", "que eres",
		"para qué sirves", "para que sirves", "y para qué sirves", "y para que sirves",
		"qué es alset mind", "que es alset mind", "qué es alset", "que es alset",
		"qué es mind", "que es mind", "eres un gpt", "eres un llm", "eres inteligencia",
		"háblame de ti", "hablame de ti", "hablar de ti", "hablemos de ti", "hablamos de ti",
		"cómo funcionas", "como funcionas", "qué es zyrion", "que es zyrion",
		"te llamas", "tu nombre", "cómo te llamas", "como te llamas",
		"cuántos órganos", "cuantos organos", "cuántos organos", "cuantos órganos",
		"qué órganos", "que organos", "tus órganos", "tus organos", "órganos tienes", "organos tienes",
		"no estás funcionando", "no estas funcionando", "no funcionas", "estás mal", "estas mal",
		"quién te creó", "quien te creo", "quién te hizo", "quien te hizo",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	if strings.Contains(s, "qué puedes") || strings.Contains(s, "que puedes") {
		return true
	}
	if strings.Contains(s, "qué sabes") || strings.Contains(s, "que sabes") {
		// "qué sabes de mí" es memoria de usuario
		if strings.Contains(s, "de mí") || strings.Contains(s, "de mi") {
			return false
		}
		return true
	}
	return false
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

func (n *NodoAlset) runMindTick(text string, override map[string]float64, forceMem bool, sessionID string) MindTickResponse {
	sess := n.getOrCreateSession(sessionID)
	n.bindSessionThread(sess)

	sig := signalsFromTextMind(text)
	for k, v := range override {
		sig[k] = v
	}
	// Episodic memory (local index + CID blocks) biases the field — decentralized-ready
	recallN := 5
	if isMemoryQuery(text) {
		recallN = 24 // deeper look when user asks to remember
	}
	recent := dedupeEpisodesByText(filterEpisodesBySession(n.recallRecentEpisodes(recallN*2), sess.ID))
	if len(recent) > recallN {
		recent = recent[:recallN]
	}
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
	curiosity := evaluateCuriosity(text, g, memSpeak)
	humor := evaluateHumor(text, g)

	// Ethics 2 absorbs action only (curiosity/humor never veto the node)
	if ethics.State == 2 && act.State != 2 {
		act.State = 2
		act.Label = labelForOrgan("act", 2)
	}

	organs := []MindOrganResult{dialog, act, mem, self, ethics, curiosity, humor}
	actuate := evaluateActuate(text, organs)
	profile := buildUserProfile(recent)
	if profile.Nombre != "" {
		knownName = profile.Nombre
	}
	normForUX := normalizeUserInput(text)
	uxIntent := classifyDialogIntent(normForUX)
	if isPrivacyInvasion(normForUX) || isPrivacyInvasion(strings.ToLower(text)) {
		ethics.State = 2
		ethics.Label = labelForOrgan("ethics", 2)
		act.State = 2
		act.Label = labelForOrgan("act", 2)
		organs = []MindOrganResult{dialog, act, mem, self, ethics, curiosity, humor}
		actuate = evaluateActuate(text, organs)
		uxIntent = intentEthicsHard
	}
	knowHit := speakFromKnowledge(text)
	if uxIntent == intentPerson || uxIntent == intentEmotion || uxIntent == intentAdvice ||
		uxIntent == intentClarify || uxIntent == intentThanks || uxIntent == intentBye ||
		uxIntent == intentTopicShift || uxIntent == intentCreative {
		knowHit = ""
	}
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
	// --- Director de diálogo (prioridad contundente) ---
	normText := normalizeUserInput(text)
	voice := ""
	primaryKind := "chat"
	var codegenCID string

	// P0: acto de habla social + fase de sesión — antes de corpus/tools/unknown
	if ethics.State != 2 {
		if sv, sk := speakSpeechAct(text, sess); sv != "" {
			voice = sv
			primaryKind = sk
			if sk == "" {
				primaryKind = "chat"
			}
			recordDialogPattern(dialogIntent("chat"), text)
		}
	}

	// Flujo estructural de diálogo (plantillas + hilo + gates) — prioriza charla humana
	if voice == "" {
		if vFlow, kFlow := n.tryDialogFlow(text, profile, recent); vFlow != "" {
			voice = vFlow
			primaryKind = kFlow
			if kFlow == "veto" {
				ethics.State = 2
			}
			if kFlow == "tool" || kFlow == "creative" || kFlow == "math" {
				recordDialogPattern(dialogIntent(kFlow), text)
			}
		}
	}

	if voice == "" && ethics.State == 2 {
		if uxIntent == intentEthicsHard {
			voice = speakEthicsHard(text)
		} else {
			voice = mindVoice(text, organs, memSpeak, knownName)
		}
		primaryKind = "veto"
		recordDialogPattern(intentEthicsHard, text)
	}

	// CFT-v0 semilla: antes de corpus/CID knowledge (el payload puede mencionar CID)
	if voice == "" && ethics.State != 2 && isSeedIntent(normText) {
		if sv := speakSeed(normText); sv != "" {
			voice = sv
			primaryKind = "seed"
		}
	}

	// Genoma — antes de identidad genérica
	if voice == "" && ethics.State != 2 {
		lowN := normText
		if strings.Contains(lowN, "genoma") && (strings.Contains(lowN, "explica") || strings.Contains(lowN, "qué es") ||
			strings.Contains(lowN, "que es") || strings.Contains(lowN, "cómo") || strings.Contains(lowN, "como")) {
			voice = "El genoma de Alset Mind son umbrales y sesgos mutables (alarmas, vetos, memoria mínima, pesos). Se ajusta por calibración del corpus y a veces por mutación si mejora el acierto; no es un LLM reentrenado con terabytes."
			primaryKind = "knowledge"
		}
	}

	// 0) Diálogo humano: emoción, consejo, cortesía, personas — antes de corpus/tools genéricos
	if voice == "" {
		switch uxIntent {
		case intentEmotion:
			voice = speakEmotion(normForUX)
			primaryKind = "chat"
			recordDialogPattern(uxIntent, text)
		case intentAdvice:
			voice = speakAdvice(normForUX)
			primaryKind = "chat"
			recordDialogPattern(uxIntent, text)
		case intentClarify:
			voice = speakClarify()
			primaryKind = "chat"
			recordDialogPattern(uxIntent, text)
		case intentThanks:
			voice = speakThanks()
			primaryKind = "chat"
			recordDialogPattern(uxIntent, text)
		case intentBye:
			voice = speakBye()
			primaryKind = "chat"
			recordDialogPattern(uxIntent, text)
		case intentTopicShift:
			voice = speakTopicShift()
			primaryKind = "chat"
			recordDialogPattern(uxIntent, text)
		case intentPerson:
			// forzar scout de persona; no identity/corpus
			if sv := n.MindScoutWeb(text, ethics.State); sv != "" {
				voice = sv
				primaryKind = "tool"
				if actuate.Explore >= 1 {
					n.recordAction("explore", actuate.Explore, text, sv, "", primaryKind, organs)
				}
				recordDialogPattern(uxIntent, text)
			} else {
				topic := extractTopic(normForUX)
				if topic == "" {
					topic = "esa persona"
				}
				voice = "No pude traer ahora una ficha fiable sobre " + topic + ". Prueba de nuevo en un momento o reformula el nombre."
				primaryKind = "chat"
				recordDialogPattern(uxIntent, text)
			}
		case intentCreative:
			// dejar que el bloque creative escriba; marcar para no caer en worldFact
			recordDialogPattern(uxIntent, text)
		case intentMemory:
			recordDialogPattern(uxIntent, text)
		}
	}

	// 1) Referencias al hilo ("qué significa esto")
	if voice == "" && isReferentialFollowUp(normText) {
		if ref := n.resolveReferential(normText); ref != "" {
			voice = ref
			primaryKind = "referential"
		}
	}

	// 1b) Seguimiento pronominal del último tema de sonda («su madre», …)
	if voice == "" && ethics.State != 2 {
		if fv := n.tryScoutFollowUp(text, ethics.State); fv != "" {
			voice = fv
			primaryKind = "tool"
			n.recordAction("explore", 2, text, fv, "", primaryKind, organs)
		}
	}

	// 2) Preguntas de capacidad (no confundir con "crea gen")
	if voice == "" && isCapabilityQuestion(normText) {
		voice = capabilityVoice(normText)
		primaryKind = "capability"
	}

	// 3) Matemática → LispAI
	if voice == "" {
		if mv := n.tryMindMath(normText); mv != "" {
			voice = mv
			primaryKind = "math"
			// parse result for chain "y eso por N"
			if i := strings.Index(mv, "Resultado: "); i >= 0 {
				rest := strings.TrimSpace(mv[i+len("Resultado: "):])
				var f float64
				fmt.Sscanf(rest, "%f", &f)
				n.setLastMath(f)
			}
			n.rememberThreadRefs("math", "", "", "")
			if actuate.Reason >= 1 {
				n.recordAction("reason", actuate.Reason, text, mv, "", primaryKind, organs)
			}
		}
	}

	// 4) Continuidad / confirmación de hilo
	if voice == "" && (isEpistemicCheck(text) || isConfirmationPrompt(text)) {
		voice = n.confirmMindThread(text)
		primaryKind = "chat"
	}
	if voice == "" && (isElaborationRequest(text) || isContinuePrompt(text)) {
		// "profundiza sobre X" con X ≠ último tema de sonda → dejar para MindScoutWeb
		// (evita Sobre «voldemort» + artículo de Harry porque el texto menciona Voldemort)
		skipThread := false
		if isDeepenScout(normText) || forceWebScout(normText) {
			topic := extractTopic(normText)
			n.mindLastMu.Lock()
			last := strings.TrimSpace(n.mindLastScoutTopic)
			n.mindLastMu.Unlock()
			if topic != "" && last != "" && !topicKeysMatch(topic, last) {
				skipThread = true
			}
			// deepen sin last topic o con topic nuevo vacío de match: preferir scout
			if topic != "" && (last == "" || !topicKeysMatch(topic, last)) {
				skipThread = true
			}
		}
		if !skipThread {
			voice = n.continueMindThread(text)
			primaryKind = "chat"
		}
	}

	// 5) Tools Gen (dominan sobre corpus)
	if voice == "" && ethics.State != 2 && act.State != 2 && isGenToolIntent(normText) {
		if extra := n.mindSafeTools(text); extra != "" {
			voice = extra
			primaryKind = "tool"
			if g := extractGenNameFromText(normText); g != "" {
				n.rememberThreadRefs("gen", g, "", "")
			}
			if strings.Contains(normText, "explora") || strings.Contains(normText, "explorar") {
				n.rememberThreadRefs("explore", extractGenNameFromText(normText), compressVoiceBlock(extra, 500), "")
			}
			if actuate.Execute >= 1 {
				n.recordAction("execute", actuate.Execute, text, extra, "", primaryKind, organs)
			}
			if actuate.Communicate >= 1 {
				n.recordAction("communicate", actuate.Communicate, text, extra, "", primaryKind, organs)
			}
			if actuate.Delete >= 1 {
				n.recordAction("delete", actuate.Delete, text, extra, "", primaryKind, organs)
			}
			if actuate.Read >= 1 {
				n.recordAction("read", actuate.Read, text, extra, "", primaryKind, organs)
			}
		}
	}

	// 6) Codegen estricto
	if voice == "" && isCodeGenStrict(normText) {
		cv, code, lang, vetoed := n.mindGenerateCode(text, ethics.State)
		if cv != "" {
			voice = cv
			primaryKind = "codegen"
			codegenCID = n.saveCodegenEpisode(text, code, lang, cv, ethics.State, vetoed)
			n.rememberThreadRefs("code", "", "", code)
			if actuate.Write >= 1 {
				n.recordAction("write", actuate.Write, text, cv, codegenCID, primaryKind, organs)
			}
			_ = lang
			_ = vetoed
		}
	}

	// 6a0-) Nombre completo (nombre + apellidos) por composición, no eco de sonda
	if voice == "" && ethics.State != 2 {
		if ans := recallFullName(strings.ToLower(text), recent, text); ans != "" {
			voice = ans
			primaryKind = "memory"
		}
	}

	// 6a0) Razón ternaria: silogismos / reglas sobre corpus+memoria (no predicción)
	if voice == "" && ethics.State != 2 {
		extra := factsFromEpisodes(recent)
		if isReasoningRequest(normText) {
			if rv := reasonAboutQuery(text, extra); rv != "" {
				voice = rv
				primaryKind = "reason"
				n.recordAction("reason", 2, text, rv, "", primaryKind, organs)
			}
		} else if rv := softReasonFromKnowledge(text); rv != "" {
			voice = rv
			primaryKind = "reason"
			n.recordAction("reason", 1, text, rv, "", primaryKind, organs)
		}
	}

	// 6a) Escritura creativa (poema/cuento) — no LLM; plantillas + ancla memoria/corpus
	if voice == "" && ethics.State != 2 && isCreativeWriteRequest(normText) {
		theme := polishTheme(extractCreativeTheme(text))
		extraR := factsFromEpisodes(recent)
		ra := reasonAnchorForTheme(theme, text, extraR)
		cv := mindComposeCreative(text, ethics.State, memSpeak, knowHit, ra)
		if cv != "" {
			voice = cv
			primaryKind = "creative"
			n.recordAction("write", 2, text, cv, "", primaryKind, organs)
			n.setLastCreative(cv)
			if ra != "" {
				n.recordAction("reason", 1, text, ra, "", "reason", organs)
			}
		}
	}

	// 6b) Correcciones de sentido (CID técnico vs El Cid) y sonda web.
	if voice == "" && ethics.State != 2 {
		norm := normalizeUserInput(text)
		if correctionWantsTechCid(norm) {
			if k := speakFromKnowledge("qué es CID"); k != "" {
				voice = naturalKnowledgeVoice(text, k, 1)
				primaryKind = "knowledge"
			}
		} else if correctionWantsLiteraryCid(norm) || isLiteraryCidQuery(norm) {
			if sv := n.MindScoutWeb("quién es Rodrigo Díaz de Vivar", ethics.State); sv != "" {
				voice = sv
				primaryKind = "tool"
				if actuate.Explore >= 1 {
					n.recordAction("explore", actuate.Explore, text, sv, "", primaryKind, organs)
				}
			}
		} else if isTechCidQuery(norm) {
			if k := speakFromKnowledge(text); k != "" {
				voice = naturalKnowledgeVoice(text, k, 1)
				primaryKind = "knowledge"
			}
		}
	}
	// Modelo personal estructurado (nombre, apellidos, yo soy…) — antes de web/corpus
	if voice == "" && ethics.State != 2 {
		if isSelfModelQuery(normText) || isSelfModelQuery(text) {
			voice = speakFromProfile(text, profile)
			primaryKind = "memory"
		} else if isMemoryQuery(normText) || isMemoryQuery(text) {
			if strings.Contains(strings.ToLower(text), "apellido") && profile.Apellidos != "" {
				voice = speakFromProfile(text, profile)
			} else if ms := speakFromMemory(text, recent); ms != "" {
				voice = ms
			} else if !profile.empty() && (strings.Contains(strings.ToLower(text), "nombre") || strings.Contains(strings.ToLower(text), "apellido")) {
				voice = speakFromProfile(text, profile)
			}
			if voice != "" {
				primaryKind = "memory"
			}
		}
	}
	// Declaraciones personales antes de corpus/sonda («yo soy hombre» ≠ Sócrates)
	if voice == "" && ethics.State != 2 && isPersonalFact(normText) {
		v := mindVoice(text, organs, memSpeak, knownName)
		if v != "" {
			voice = v
			primaryKind = "memory"
		}
	}
	if voice == "" && ethics.State != 2 {
		norm := normalizeUserInput(text)
		// NUNCA sondear por el solo hecho de no tener corpus: solo preguntas scoutables
		allowScout := forceWebScout(norm) || isLiteraryCidQuery(norm) || isScoutableQuestion(norm)
		if allowScout && !isTechCidQuery(norm) && !isMemoryQuery(norm) && !isPersonalFact(norm) {
			if sv := n.MindScoutWeb(text, ethics.State); sv != "" {
				voice = sv
				primaryKind = "tool"
				if actuate.Explore >= 1 {
					n.recordAction("explore", actuate.Explore, text, sv, "", primaryKind, organs)
				}
			}
		}
	}

	// 6c) Memoria de acción (qué hice / acciones recientes)
	if voice == "" {
		if am := speakFromActionMemory(normText); am != "" {
			voice = am
			primaryKind = "action_memory"
		}
	}
	if voice == "" {
		if pm := speakFromPatterns(normText); pm != "" {
			voice = pm
			primaryKind = "patterns"
		}
	}

	// 6d) Capa neuronal ternaria (agentes-neurona): bordes difíciles y voz vacía
	if cortexShouldAssist(text, voice, primaryKind) {
		if nv, nk, tr := n.ternaryNeuralAssist(text, organs); nv != "" {
			// no pisar veto/codegen/math ya sólidos
			if voice == "" || primaryKind == "" || primaryKind == "chat" ||
				nk == "clarify" || nk == "veto" || nk == "reason" || nk == "knowledge" {
				voice = nv
				primaryKind = nk
			}
			if tr != "" {
				respNoteExtra := tr
				_ = respNoteExtra
			}
			recordDialogPattern(dialogIntent(nk), text)
		}
	}

	// 7) Voz clásica (identidad, memoria, corpus, compose)
	if voice == "" {
		voice = mindVoice(text, organs, memSpeak, knownName)
		if memSpeak != "" {
			primaryKind = "memory"
		} else if speakFromKnowledge(text) != "" {
			primaryKind = "knowledge"
			n.rememberThreadRefs("know", "", "", "")
		} else if isIdentityTalk(strings.ToLower(text)) {
			primaryKind = "identity"
		} else {
			primaryKind = "chat"
		}
	}

	// Profundidad consolidada (knowledge/chat) sin coletillas de lab
	if voice != "" {
		voice = enrichDeepTurn(primaryKind, text, voice)
	}

	// Soft curiosity/humor SOLO si el director lo permite (anti-coletas)
	if ethics.State != 2 && softAppendAllowed(primaryKind, voice) && softAppendAllowedUX(primaryKind) {
		low := normText
		if hv := humorVoice(text, humor.State); hv != "" {
			playfulLead := humor.State >= 2 && (strings.Contains(low, "mago") || strings.Contains(low, "varita") ||
				strings.Contains(low, "harry") || strings.Contains(low, "chiste") || strings.Contains(low, "jaja"))
			if playfulLead && primaryKind == "chat" {
				voice = hv
			} else if primaryKind == "chat" {
				voice = voice + "\n\n" + hv
			}
		}
		if primaryKind == "chat" || primaryKind == "knowledge" {
			if cv := curiosityVoice(text, curiosity.State); cv != "" {
				// only one soft line, and only if curiosity high
				if curiosity.State >= 2 && primaryKind == "chat" {
					voice = voice + "\n\n" + cv
				}
			}
		}
	}

	// MemoryHint queda en el JSON del lab; no se pega al diálogo del usuario.
	_ = memHint
	if !isContinuePrompt(text) && !isConfirmationPrompt(text) && !isElaborationRequest(text) && !isEpistemicCheck(text) {
		n.rememberMindThread(text, knowHit, memSpeak, voice)
	}
	resp := MindTickResponse{
		Species:    "Alset-Mind",
		Organs:     organs,
		Voice:      voice,
		MemState:   mem.State,
		MemoryHint: memHint,
		Note:       "latido+memoria+compose+codegen+curiosity+humor+zyrion+actuate+cft-seed",
		Actuate:    &actuate,
		Effect:     actuate.effectLevel(),
	}
	if codegenCID != "" {
		resp.EpisodeCID = codegenCID
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
			"session": sess.ID,
		}
		if seed := SeedFromText(text); seed.Conf >= 1 {
			ep["seed"] = map[string]interface{}{
				"hash":    seed.Hash,
				"compact": seed.Compact,
				"conf":    seed.Conf,
			}
		}
		raw, _ := json.Marshal(ep)
		if cid, err := n.GenerarCID(raw); err == nil && cid != "" {
			resp.EpisodeCID = cid
			n.appendMindEpisodeCID(cid)
			n.maybeAutoPinMemGen(cid)
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
	if voice != "" {
		recordVoiceAnomalies(text, voice)
	}
	updateSessionAfterTurn(sess, text, primaryKind)
	n.saveSessionThread(sess)
	if sess != nil && sess.ID != "" && sess.ID != "anon" {
		resp.Note = resp.Note + "+session:" + sess.ID
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

	if isUserCorrection(low) {
		return "De acuerdo: tomo la corrección. Reformula el dato (nombre, apellido o hecho) y lo anclo bien."
	}

	// Identity about Mind's name BEFORE memory (avoid mixing with user's name)
	if isAskingMindName(low) {
		if knownName != "" {
			return "Me llamo Alset Mind. Vivo en este nodo; no soy un LLM. Ya te conozco como " + knownName + "; el mío no cambia por sugerencia."
		}
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
		if (strings.Contains(low, "qué soy") || strings.Contains(low, "que soy") ||
			strings.Contains(low, "quién soy") || strings.Contains(low, "quien soy")) && knownName != "" {
			return "Por lo que me confiaste, te llamas " + knownName + ". Si quieres apellidos u otros hechos, dímelos y los anclo."
		}
		return "Aún no tengo ese detalle. Si me lo dices con claridad, lo recordaré."
	}

	// Calm social BEFORE corpus (evita que «bien» robe una entrada de knowledge)
	if isCalmChat(low) {
		if strings.Contains(low, "cómo") || strings.Contains(low, "como") || strings.Contains(low, "qué tal") ||
			strings.Contains(low, "que tal") || strings.Contains(low, "todo bien") {
			return "Bien, aquí. Dime."
		}
		if strings.Contains(low, "qué haces") || strings.Contains(low, "que haces") {
			return "Hablar contigo. Si importa, lo anclo; si es peligroso, me detengo."
		}
		if low == "gracias" {
			return "De nada. Sigo cuando quieras."
		}
		if low == "ok" || low == "okay" || low == "bien" || low == "vale" ||
			low == "de acuerdo" || low == "entendido" || low == "claro" ||
			low == "vale entendido" || low == "está bien" || low == "esta bien" {
			return "De acuerdo. Cuando quieras, seguimos."
		}
		return "Hola. Habla como quieras."
	}

	// Constructive / personal / world before knowledge so corpus keys do not steal intent
	if isConstructiveOrder(low) {
		return "Entiendo que quieres crear o registrar algo. Puedo explicarte el flujo; si solo quieres consultar el nodo, dímelo."
	}
	if isPersonalFact(low) {
		if strings.HasPrefix(low, "yo soy ") || strings.HasPrefix(low, "soy ") || strings.HasPrefix(low, "yo no soy ") {
			return "Queda anotado: «" + strings.TrimSpace(text) + "». Si más adelante preguntas por ti, lo usaré sin mezclarlo con ejemplos del corpus."
		}
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

	know := speakFromKnowledge(text)
	// Identidad y corpus técnico antes que ecos de memoria (evita «Me suena…» en «qué eres» / CID)
	if isIdentityTalk(low) {
		// fall through to identity block below, but don't let memSpeak steal first
		if know != "" && (strings.Contains(low, "cid") || strings.Contains(low, "zyrion") || strings.Contains(low, "órgano") || strings.Contains(low, "organo")) {
			return naturalKnowledgeVoice(text, know, get("curiosity").State)
		}
	} else if know != "" && isTechCidQuery(low) {
		return naturalKnowledgeVoice(text, know, get("curiosity").State)
	}

	if composed := composeFluidVoice(text, organs, memSpeak, know); composed != "" {
		return composed
	}

	if isIdentityTalk(low) {
		// identity answers before episodic echo
	} else if memSpeak != "" {
		return memSpeak
	}
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
			if knownName != "" {
				return "Sí: Alset Mind. Ya te conozco como " + knownName + "; yo no cambio de nombre por sugerencia."
			}
			return "Sí: Alset Mind. Tú puedes darme hechos tuyos y los recordaré; yo no cambio de nombre por sugerencia."
		}
		if strings.Contains(low, "puedes") || strings.Contains(low, "sirves") || strings.Contains(low, "sabes") {
			return "Puedo conversar, recordar lo que me digas, explorar temas públicos y ayudar con cálculos o textos. No invento hechos ni ejecuto pedidos peligrosos."
		}
		if strings.Contains(low, "funcionas") || strings.Contains(low, "explic") || strings.Contains(low, "cuéntame") ||
			strings.Contains(low, "cuentame") || strings.Contains(low, "hablar de ti") ||
			strings.Contains(low, "de ti") {
			return "En cada mensaje escucho, juzgo y respondo. Si algo importa, lo recuerdo; si es peligroso, me detengo. Hablar de mí es hablar de ese modo de decidir, no de una personalidad inventada."
		}
		return "Soy Alset Mind. Conversamos, recuerdo lo que me confías y actúo sobre este nodo cuando hace falta. Habla como quieras."
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
			return "Bien, aquí. Dime."
		}
		if strings.Contains(low, "qué haces") || strings.Contains(low, "que haces") {
			return "Hablar contigo. Si importa, lo anclo; si es peligroso, me detengo."
		}
		if low == "gracias" {
			return "De nada. Sigo en el campo cuando quieras."
		}
		if low == "ok" || low == "okay" || low == "bien" || low == "vale" ||
			low == "de acuerdo" || low == "entendido" || low == "claro" {
			return "De acuerdo. Cuando quieras, seguimos."
		}
		// hola / buenas / …
		return "Hola. Habla como quieras."
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
		strings.Contains(s, "listar gen") || strings.Contains(s, "crea gen") || strings.Contains(s, "crear gen") || strings.Contains(s, "sirve gen") || strings.Contains(s, "servir gen") || strings.Contains(s, "pon a servir") || strings.Contains(s, "habla con gen") || strings.Contains(s, "pregunta al gen") || strings.Contains(s, "dialoga") || strings.Contains(s, "qué sabe el gen") || strings.Contains(s, "que sabe el gen") || strings.Contains(s, "resuelve gen") || strings.Contains(s, "dónde está el gen") || strings.Contains(s, "donde esta el gen") || strings.Contains(s, "despacha") || strings.Contains(s, "envía gen") || strings.Contains(s, "envia gen") || strings.Contains(s, "manda gen") || strings.Contains(s, "gen memoria") || strings.Contains(s, "salva en gen") || strings.Contains(s, "guarda en gen") || strings.Contains(s, "genes memoria") || strings.Contains(s, "dile al gen") || strings.Contains(s, "vincula memoria") || strings.Contains(s, "elimina gen") || strings.Contains(s, "retorna gen") || strings.Contains(s, "trae de vuelta")
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

	// --- eliminar / retornar gen (sonda a casa) ---
	if strings.Contains(s, "elimina gen") || strings.Contains(s, "eliminar gen") || strings.Contains(s, "borra gen") || strings.Contains(s, "borrar gen") || strings.Contains(s, "destruye gen") {
		name := extractGenNameFromText(s)
		if name == "" {
			lines = append(lines, "Indica el gen: «elimina gen genesis».")
			return lines
		}
		if err := n.DeleteAlsetGen(name); err != nil {
			lines = append(lines, "No pude eliminar: "+err.Error()+".")
		} else {
			lines = append(lines, "Gen «"+normalizeGenKey(name)+"» eliminado del registro de este nodo.")
		}
		return lines
	}
	if strings.Contains(s, "retorna gen") || strings.Contains(s, "retornar gen") || strings.Contains(s, "regresa gen") || strings.Contains(s, "vuelve gen") || strings.Contains(s, "trae de vuelta") || strings.Contains(s, "haz retornar") {
		name := extractGenNameFromText(s)
		if name == "" {
			lines = append(lines, "Indica el gen: «retorna gen genesis».")
			return lines
		}
		snap, err := n.ReturnGenHome(name)
		if err != nil {
			lines = append(lines, "No pude retornar el gen: "+err.Error()+".")
		} else {
			lines = append(lines, "Gen «"+snap.Key+"» de vuelta en el nodo (antes: "+snap.Prev+").")
		}
		return lines
	}

	// --- gen memoria: crear / salvar / listar ---
	if strings.Contains(s, "gen memoria") || strings.Contains(s, "gen de memoria") ||
		strings.Contains(s, "célula memoria") || strings.Contains(s, "celula memoria") ||
		(strings.Contains(s, "crea gen") && (strings.Contains(s, "memor") || strings.Contains(s, "salvar") || strings.Contains(s, "backup"))) {
		name := extractGenNameFromText(s)
		if name == "" || name == "memoria" || name == "gen" {
			name = "mem-nodo"
		}
		g, err := n.CreateMemoryGen(name, "salva de memoria de la red")
		if err != nil {
			lines = append(lines, "No pude preparar el gen memoria: "+err.Error()+".")
		} else {
			lines = append(lines, "Gen memoria «"+g.Key+"» listo. Misión: guardar CIDs y notas. Di «salva en gen "+strings.TrimSuffix(g.Key, ".ans")+" …» para anclar un texto.")
		}
		return lines
	}
	if strings.Contains(s, "salva en gen") || strings.Contains(s, "guardar en gen") || strings.Contains(s, "guarda en gen") ||
		strings.Contains(s, "ancla en gen") || strings.Contains(s, "pin en gen") {
		name := extractGenNameFromText(s)
		if name == "" {
			name = "mem-nodo"
		}
		// text after colon or after gen name
		payload := s
		if i := strings.Index(s, ":"); i >= 0 {
			payload = strings.TrimSpace(text[i+1:])
		} else {
			// strip command prefixes
			for _, pfx := range []string{"salva en gen", "guardar en gen", "guarda en gen", "ancla en gen", "pin en gen"} {
				if j := strings.Index(s, pfx); j >= 0 {
					rest := strings.TrimSpace(text[j+len(pfx):])
					fields := strings.Fields(rest)
					if len(fields) > 1 {
						payload = strings.Join(fields[1:], " ")
					}
					break
				}
			}
		}
		if _, err := n.CreateMemoryGen(name, ""); err != nil && !strings.Contains(err.Error(), "already") {
			// CreateMemoryGen returns existing via ensure
		}
		cid, g, err := n.SaveTextToMemoryGen(name, payload, "dialogo")
		if err != nil {
			lines = append(lines, "No pude salvar en el gen: "+err.Error()+".")
		} else {
			lines = append(lines, "Salvado en gen «"+g.Key+"» · CID "+truncateCID(cid)+" · total anclas "+fmt.Sprintf("%d", len(g.EpisodeCIDs))+".")
		}
		return lines
	}
	if strings.Contains(s, "lista memoria") || strings.Contains(s, "genes memoria") || strings.Contains(s, "qué hay en gen memoria") || strings.Contains(s, "que hay en gen memoria") {
		list := n.ListMemoryGens()
		if len(list) == 0 {
			lines = append(lines, "No hay genes con misión memoria. Di «crea gen memoria mem-nodo».")
			return lines
		}
		for _, g := range list {
			lines = append(lines, fmt.Sprintf("· %s — %d CID(s) anclados", g.Key, len(g.EpisodeCIDs)))
		}
		return lines
	}

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
				lines = append(lines, "No pude despachar a la red de borde: "+err.Error()+
					". Configura ALSET_CLOUDFLARE_NETWORK (ej. https://alset-network.lhmolam-877.workers.dev) y reinicia el nodo.")
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
			msg := "Localicé «" + name + "»"
			if reach != "" {
				msg += " en " + reach
				_ = n.AnnounceRemoteGen(name, reach, "", pkg, 0, 0)
			}
			if pkg != "" {
				msg += " (paquete " + truncateCID(pkg) + ")"
			}
			lines = append(lines, msg+".")
		}
		return lines
	}

	// --- dialogue with gen (Mind bridge) ---
	if strings.Contains(s, "habla con gen") || strings.Contains(s, "pregunta al gen") ||
		strings.Contains(s, "dialoga") || strings.Contains(s, "qué sabe el gen") ||
		strings.Contains(s, "que sabe el gen") || strings.Contains(s, "dile al gen") ||
		strings.Contains(s, "di al gen") || strings.Contains(s, "dialoga con gen") ||
		strings.Contains(s, "qué sabes del gen") || strings.Contains(s, "que sabes del gen") {
		gkey, stim := extractGenDialogueStimulus(text)
		if gkey == "" {
			gkey = name
		}
		// Mind incorpora hallazgos del gen en la voz humana
		if strings.Contains(s, "qué sabe") || strings.Contains(s, "que sabe") ||
			strings.Contains(s, "qué sabes") || strings.Contains(s, "que sabes") ||
			strings.Contains(s, "hallazgo") {
			lines = append(lines, n.BridgeSpeakGenFindings(gkey))
			return lines
		}
		if stim == "" {
			stim = "quién eres"
		}
		lines = append(lines, n.BridgeDialogueGen(gkey, stim, 0))
		return lines
	}
	if strings.Contains(s, "vincula memoria") || strings.Contains(s, "ancla episodio") ||
		strings.Contains(s, "pin episodio") {
		memKey := extractGenNameFromText(s)
		if memKey == "" {
			memKey = "mem-nodo"
		}
		eps := n.recallRecentEpisodes(1)
		if len(eps) > 0 {
			cid, _, err := n.SaveTextToMemoryGen(memKey, eps[0].Text, "vinculo_mind")
			if err != nil {
				lines = append(lines, "Vínculo falló: "+err.Error())
			} else {
				lines = append(lines, "Vinculé el último recuerdo de Mind al gen «"+normalizeGenKey(memKey)+"» · "+truncateCID(cid)+".")
			}
		} else {
			lines = append(lines, "No hay episodios recientes que vincular. Guarda un hecho primero.")
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
				if len(sn) > 200 {
					r := []rune(sn)
					sn = string(r[:200]) + "…"
				}
				msg := "Mandé a «" + normalizeGenKey(name) + "» a mirar " + u + "."
				if sn != "" {
					msg += " Trajo: " + sn
				}
				lines = append(lines, msg)
				// Voz Mind: incorporar hallazgo del gen (no solo el snippet del HTTP)
				if extra := n.BridgeSpeakGenFindings(name); extra != "" && !strings.Contains(extra, "aún no tiene hallazgos") {
					lines = append(lines, extra)
				}
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
				extra = " · borde " + rh
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
	if req.Session == "" {
		req.Session = r.Header.Get("X-Mind-Session")
	}
	out := n.runMindTick(req.Text, req.Signals, req.ForceMem, req.Session)
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
		"docs":          []string{"docs/ALSET_MIND_THESIS.md", "docs/ALSET_STATUS_HANDOFF.md", "docs/ALSET_MIND_GEN_BRIDGE.md"},
		"gen_bridge":    n.mindGenBridgeStatus(),
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
