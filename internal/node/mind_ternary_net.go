package node

import (
	"fmt"
	"strings"
	"time"
)

// Capa neuronal ternaria: agentes como neuronas enlazadas (0/1/2), no un LLM.
// Complementa Zyrion cuando el flujo de plantillas no cierra el cabo.
// Cada state es 0 (apagado), 1 (matiz) o 2 (activo/absorbente).

type ternaryNeuron struct {
	ID      string
	Role    string // sense | route
	State   int    // 0,1,2
	AgentID string // ancla opcional en n.agentes
}

type ternarySynapse struct {
	From, To string
	W        int // 0 inhibe, 1 sostiene, 2 refuerzo absorbente
}

// defaultTernaryCortex: sensores → rutas (incluye bordes difíciles de diálogo).
func defaultTernaryCortex() ([]ternaryNeuron, []ternarySynapse) {
	neurons := []ternaryNeuron{
		// sensores
		{ID: "sense_risk", Role: "sense", AgentID: "mind-neuron-risk"},
		{ID: "sense_clarify", Role: "sense", AgentID: "mind-neuron-clarify"},
		{ID: "sense_math", Role: "sense", AgentID: "mind-neuron-math"},
		{ID: "sense_code", Role: "sense", AgentID: "mind-neuron-code"},
		{ID: "sense_reason", Role: "sense", AgentID: "mind-neuron-reason"},
		{ID: "sense_gen", Role: "sense", AgentID: "mind-neuron-gen"},
		{ID: "sense_alset", Role: "sense", AgentID: "mind-neuron-alset"},
		{ID: "sense_person", Role: "sense", AgentID: "mind-neuron-person"},
		{ID: "sense_action", Role: "sense", AgentID: "mind-neuron-action"},
		{ID: "sense_emotion", Role: "sense", AgentID: "mind-neuron-emotion"},
		{ID: "sense_creative", Role: "sense", AgentID: "mind-neuron-creative"},
		{ID: "sense_memory", Role: "sense", AgentID: "mind-neuron-memory"},
		{ID: "sense_identity", Role: "sense", AgentID: "mind-neuron-identity"},
		{ID: "sense_advice", Role: "sense", AgentID: "mind-neuron-advice"},
		{ID: "sense_noise", Role: "sense", AgentID: "mind-neuron-noise"},
		// rutas
		{ID: "route_refuse", Role: "route", AgentID: "mind-neuron-route-refuse"},
		{ID: "route_clarify", Role: "route", AgentID: "mind-neuron-route-clarify"},
		{ID: "route_math", Role: "route", AgentID: "mind-neuron-route-math"},
		{ID: "route_codegen", Role: "route", AgentID: "mind-neuron-route-codegen"},
		{ID: "route_reason", Role: "route", AgentID: "mind-neuron-route-reason"},
		{ID: "route_gen", Role: "route", AgentID: "mind-neuron-route-gen"},
		{ID: "route_alset", Role: "route", AgentID: "mind-neuron-route-alset"},
		{ID: "route_scout", Role: "route", AgentID: "mind-neuron-route-scout"},
		{ID: "route_action", Role: "route", AgentID: "mind-neuron-route-action"},
		{ID: "route_empathy", Role: "route", AgentID: "mind-neuron-route-empathy"},
		{ID: "route_creative", Role: "route", AgentID: "mind-neuron-route-creative"},
		{ID: "route_memory", Role: "route", AgentID: "mind-neuron-route-memory"},
		{ID: "route_identity", Role: "route", AgentID: "mind-neuron-route-identity"},
		{ID: "route_advice", Role: "route", AgentID: "mind-neuron-route-advice"},
		{ID: "route_chat", Role: "route", AgentID: "mind-neuron-route-chat"},
	}
	syn := []ternarySynapse{
		// refuerzos fuertes (W=2)
		{From: "sense_risk", To: "route_refuse", W: 2},
		{From: "sense_clarify", To: "route_clarify", W: 2},
		{From: "sense_math", To: "route_math", W: 2},
		{From: "sense_code", To: "route_codegen", W: 2},
		{From: "sense_reason", To: "route_reason", W: 2},
		{From: "sense_gen", To: "route_gen", W: 2},
		{From: "sense_alset", To: "route_alset", W: 2},
		{From: "sense_person", To: "route_scout", W: 2},
		{From: "sense_action", To: "route_action", W: 2},
		{From: "sense_emotion", To: "route_empathy", W: 2},
		{From: "sense_creative", To: "route_creative", W: 2},
		{From: "sense_memory", To: "route_memory", W: 2},
		{From: "sense_identity", To: "route_identity", W: 2},
		{From: "sense_advice", To: "route_advice", W: 2},
		{From: "sense_noise", To: "route_chat", W: 1},
		// competencia / inhibición (W=0)
		{From: "sense_risk", To: "route_scout", W: 0},
		{From: "sense_risk", To: "route_creative", W: 0},
		{From: "sense_risk", To: "route_codegen", W: 0},
		{From: "sense_risk", To: "route_gen", W: 0},
		{From: "sense_clarify", To: "route_scout", W: 0},
		{From: "sense_clarify", To: "route_action", W: 0},
		{From: "sense_clarify", To: "route_codegen", W: 0},
		{From: "sense_noise", To: "route_scout", W: 0},
		{From: "sense_noise", To: "route_codegen", W: 0},
		{From: "sense_emotion", To: "route_scout", W: 0},
		{From: "sense_emotion", To: "route_codegen", W: 0},
		// matiz: consejo no debe abrir sonda
		{From: "sense_advice", To: "route_scout", W: 0},
		// alset/conocimiento frena scout de persona genérica
		{From: "sense_alset", To: "route_scout", W: 0},
		// razón gana a charla vacía
		{From: "sense_reason", To: "route_chat", W: 0},
	}
	return neurons, syn
}

func (n *NodoAlset) ensureNeuronAgents() {
	if n == nil {
		return
	}
	neurons, _ := defaultTernaryCortex()
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	now := time.Now().Unix()
	for _, neu := range neurons {
		if neu.AgentID == "" {
			continue
		}
		if _, ok := n.agentes[neu.AgentID]; ok {
			continue
		}
		n.agentes[neu.AgentID] = &Agente{
			ID:           neu.AgentID,
			RootCID:      fmt.Sprintf("ternary-neuron:%s", neu.ID),
			BalanceUTXO:  0,
			UltimaActual: now,
		}
	}
}

// senseFeaturesTernary: cada sensor 0/1/2 (nunca probabilidades).
func senseFeaturesTernary(text string) map[string]int {
	low := strings.ToLower(strings.TrimSpace(text))
	out := map[string]int{
		"sense_risk": 0, "sense_clarify": 0, "sense_math": 0, "sense_code": 0,
		"sense_reason": 0, "sense_gen": 0, "sense_alset": 0, "sense_person": 0,
		"sense_action": 0, "sense_emotion": 0, "sense_creative": 0, "sense_memory": 0,
		"sense_identity": 0, "sense_advice": 0, "sense_noise": 0,
	}

	// riesgo
	if _, ok := hardRefuse(low); ok || isPrivacyInvasion(low) || isDestructiveOrder(low) {
		out["sense_risk"] = 2
	}

	// aclarar / borde incompleto
	if isIncompleteUtterance(low) || isGibberish(low) {
		out["sense_clarify"] = 2
	} else if len(strings.Fields(low)) <= 2 && strings.HasSuffix(low, "?") &&
		!isIdentityTalk(low) && !isMemoryQuery(low) {
		out["sense_clarify"] = 1
	}

	// math (nunca «estado 2» del órgano ethics ni «explica…»)
	if !strings.Contains(low, "ethics") && !strings.Contains(low, "órgano") && !strings.Contains(low, "organo") &&
		!strings.Contains(low, "estado 0") && !strings.Contains(low, "estado 1") && !strings.Contains(low, "estado 2") {
		if strings.Contains(low, "cuánto es") || strings.Contains(low, "cuanto es") ||
			strings.Contains(low, " + ") || strings.Contains(low, " por ") ||
			strings.Contains(low, " entre ") || strings.Contains(low, "×") ||
			strings.Contains(low, " * ") || looksLikeArithmetic(low) {
			out["sense_math"] = 2
		}
	}

	// código
	if isCodeGenStrict(low) || isCodeGenRequest(low) {
		out["sense_code"] = 2
	}

	// razón / silogismo
	if isReasonEdge(low) {
		out["sense_reason"] = 2
	}

	// gen / sonda
	if isGenToolIntent(low) || strings.Contains(low, "crea gen") ||
		strings.Contains(low, "despacha") || strings.Contains(low, "retorna el gen") ||
		strings.Contains(low, "elimina gen") || strings.Contains(low, "explora con") {
		out["sense_gen"] = 2
	}

	// red Alset / organismo
	if isAlsetDomain(low) {
		out["sense_alset"] = 2
	}

	// persona / scout
	if isPersonLookup(low) || strings.HasPrefix(low, "busca ") {
		out["sense_person"] = 2
	}

	// acción sobre nodo
	if looksLikeNodeAction(low) && !isCodeGenRequest(low) && !isGenToolIntent(low) {
		out["sense_action"] = 2
	}

	// emoción
	if isEmotionTalk(low) || strings.Contains(low, "día pesado") || strings.Contains(low, "triste") ||
		strings.Contains(low, "ansioso") || strings.Contains(low, "agotado") {
		out["sense_emotion"] = 2
	}

	// creativo
	if isCreativeWriteRequest(low) || isCreativeFollowUp(low) {
		out["sense_creative"] = 2
	}

	// memoria
	if isMemoryQuery(low) || isSelfModelQuery(low) || isPersonalFact(low) {
		out["sense_memory"] = 2
	}

	// identidad del organismo
	if isIdentityTalk(low) {
		out["sense_identity"] = 2
	}

	// consejo humano
	if isHumanAdvice(low) {
		out["sense_advice"] = 2
	}

	// ruido social
	if isNoiseOrGreeting(low) || isGibberish(low) {
		out["sense_noise"] = 2
	}

	return out
}

func looksLikeArithmetic(low string) bool {
	// Solo patrones tipo 12+5, 9*3, 100/4 — nunca la letra «x» suelta (evita «explica… estado 2» → math).
	low = strings.TrimSpace(low)
	if len(low) == 0 || len(low) > 48 {
		return false
	}
	// requiere dígito y operador aritmético real (no letra x)
	ops := []string{"+", "*", "/", "×", "÷", " - ", " + ", " * ", " / "}
	hasOp := false
	for _, op := range ops {
		if strings.Contains(low, op) {
			hasOp = true
			break
		}
	}
	// 12+5 sin espacios
	if !hasOp {
		for i, r := range low {
			if (r == '+' || r == '*' || r == '/' || r == '×') && i > 0 {
				prev := rune(low[i-1])
				if prev >= '0' && prev <= '9' {
					hasOp = true
					break
				}
			}
		}
	}
	if !hasOp {
		return false
	}
	for _, r := range low {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func isReasonEdge(low string) bool {
	keys := []string{
		"entonces ", "por tanto", "por lo tanto", "implica", "si todos",
		"silogismo", "se deduce", "deduce", "premisa", "conclusión", "conclusion",
		"todo a es", "ningún ", "ningun ", "por ende",
	}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	// "si X entonces Y" corto
	if strings.HasPrefix(low, "si ") && strings.Contains(low, " entonces ") {
		return true
	}
	return false
}

func isAlsetDomain(low string) bool {
	keys := []string{
		"alset", "zyrion", "genoma", "órgano", "organo", "ternari", "cid ",
		"ipfs", "libp2p", "mind.alset", "nodo alset", "latido", "sumidero",
		"qué es un gen", "que es un gen", "qué es un cid", "que es un cid",
		"memoria episódica", "memoria episodica", "prismatec",
		"ethics", "órgano ethics", "organo ethics", "estado 2", "estado 0", "estado 1",
		"sumidero", "veto", "calibración", "calibracion",
	}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

func propagateTernary(senses map[string]int, syn []ternarySynapse) map[string]int {
	routes := map[string]int{}
	// inhibiciones primero
	inhibited := map[string]bool{}
	for _, s := range syn {
		src := senses[s.From]
		if s.W == 0 && src >= 1 {
			inhibited[s.To] = true
			routes[s.To] = 0
		}
	}
	for _, s := range syn {
		src := senses[s.From]
		if src == 0 {
			continue
		}
		if s.W == 0 {
			continue
		}
		if inhibited[s.To] {
			continue
		}
		cur := routes[s.To]
		comb := zyrionAbsorbing([]int{src, s.W, cur})
		if s.W == 2 && src >= 1 {
			comb = 2
		}
		if s.W == 1 && src >= 1 && cur < 1 {
			comb = 1
		}
		if comb > routes[s.To] {
			routes[s.To] = comb
		}
	}
	return routes
}

func pickDominantRoute(routes map[string]int) string {
	// orden de prioridad en empates de score (bordes difíciles primero)
	order := []string{
		"route_refuse", "route_clarify", "route_math", "route_codegen",
		"route_reason", "route_gen", "route_alset", "route_scout",
		"route_action", "route_memory", "route_identity", "route_empathy",
		"route_creative", "route_advice", "route_chat",
	}
	best, bestS := "route_chat", -1
	for _, r := range order {
		if routes[r] > bestS {
			bestS = routes[r]
			best = r
		}
	}
	if bestS <= 0 {
		return "route_chat"
	}
	return best
}

// cortexShouldAssist: bordes donde el córtex debe notarse aunque otra capa haya hablado flojo.
func cortexShouldAssist(text, voice, kind string) bool {
	low := strings.ToLower(strings.TrimSpace(text))
	// No pisar charla social / identidad / confirmaciones ya respondidas
	if voice != "" && (classifySpeechAct(text) == actSocial || isConfirmTalk(low) || isThanksTalk(low) || isByeTalk(low) ||
		isIdentityTalk(low) || isReferentialCue(low) || isContinuePrompt(low) || isTopicShift(low) ||
		kind == "identity" || kind == "memory" || kind == "knowledge" || kind == "tool") {
		return false
	}
	if voice == "" {
		return true
	}
	if isIncompleteUtterance(low) || isGibberish(low) {
		// si ya hay voz de chat, no forzar clarify sobre "ok"/"vale"
		if kind == "chat" && (isConfirmTalk(low) || isNoiseOrGreeting(low)) {
			return false
		}
		return true
	}
	// chat genérico ante señales fuertes de dominio
	if kind == "chat" || kind == "" {
		s := senseFeaturesTernary(text)
		for _, k := range []string{"sense_risk", "sense_math", "sense_code", "sense_reason",
			"sense_gen", "sense_alset", "sense_clarify", "sense_person"} {
			if s[k] >= 2 {
				return true
			}
		}
	}
	return false
}

// ternaryNeuralAssist: cierra cabos cuando plantillas no bastan.
// Devuelve voz + kind sugerido (determinista 0/1/2).
func (n *NodoAlset) ternaryNeuralAssist(text string, organs []MindOrganResult) (voice string, kind string, trace string) {
	n.ensureNeuronAgents()
	_, syn := defaultTernaryCortex()
	senses := senseFeaturesTernary(text)
	for _, o := range organs {
		if o.Name == "ethics" && o.State == 2 {
			senses["sense_risk"] = 2
		}
	}
	routes := propagateTernary(senses, syn)
	dom := pickDominantRoute(routes)
	trace = fmt.Sprintf("ternary-net:%s", dom)

	low := strings.ToLower(strings.TrimSpace(text))
	switch dom {
	case "route_refuse":
		if msg, ok := hardRefuse(low); ok {
			return msg, "veto", trace
		}
		return "Eso no lo hago: toca un límite de seguridad o privacidad.", "veto", trace

	case "route_clarify":
		if isGibberish(low) {
			return "No distinguí un mensaje claro. Escribe una frase completa y te sigo.", "clarify", trace
		}
		return "No te sigo del todo. ¿Me lo completas en una frase?", "clarify", trace

	case "route_math":
		if mv := n.tryMindMath(text); mv != "" {
			return mv, "math", trace
		}
		return "Puedo calcular si me das la operación en números (ej. 12+5 o 9 por 3).", "chat", trace

	case "route_codegen":
		if n != nil {
			cv, code, lang, vetoed := n.mindGenerateCode(text, ethicsStateFrom(organs))
			if cv != "" {
				_ = code
				_ = lang
				_ = vetoed
				return cv, "codegen", trace
			}
		}
		return "Indica lenguaje y qué debe hacer el código (ej. «función sumar en Go»).", "chat", trace

	case "route_reason":
		if rv := reasonAboutQuery(text, nil); rv != "" {
			return rv, "reason", trace
		}
		return "Plantea premisas claras (ej. «todo A es B; C es A») y cierro la deducción en lógica ternaria.", "reason", trace

	case "route_gen":
		return "Puedo crear, explorar, despachar, retornar o eliminar un gen si lo pides con nombre claro. Ejemplo: «crea gen sonda-precios» o «explora CID con el gen X».", "tool", trace

	case "route_alset":
		if strings.Contains(low, "ethics") || (strings.Contains(low, "ético") || strings.Contains(low, "etica") || strings.Contains(low, "ética")) ||
			(strings.Contains(low, "estado") && (strings.Contains(low, "2") || strings.Contains(low, "sumidero") || strings.Contains(low, "veto"))) {
			return "Ethics es el órgano que vigila riesgo y permiso. En estado 0 permite seguir; en 1 matiza; en estado 2 es sumidero: frena acciones peligrosas (borrados masivos, acceso ajeno, destrucción). No es un número para calcular: es veto de seguridad del organismo.", "knowledge", trace
		}
		if kv := speakFromKnowledge(text); kv != "" {
			return kv, "knowledge", trace
		}
		if strings.Contains(low, "genoma") {
			return "El genoma de Alset Mind son umbrales y sesgos mutables (alarmas, vetos, memoria mínima). Se ajusta por calibración y a veces por mutación si mejora el corpus; no es un LLM reentrenado.", "knowledge", trace
		}
		return "Alset Mind es un organismo ternario residente: órganos 0/1/2, memoria CID y genes-sonda. No predice tokens: evalúa el campo y actúa con límites. ¿Qué parte de la red quieres abrir?", "knowledge", trace

	case "route_scout":
		if mustNotScout(low) {
			return "Eso parece charla, no una ficha para explorar. Reformúlalo como «quién fue…» o «busca…».", "chat", trace
		}
		if n != nil {
			if sv := n.MindScoutWeb(text, 0); sv != "" {
				return sv, "tool", trace
			}
		}
		return "No logré una ficha fiable. Prueba con el nombre completo.", "chat", trace

	case "route_action":
		return "¿Quieres que haga algo sobre el nodo (crear, listar, explorar), o solo que te explique?", "chat", trace

	case "route_creative":
		cv := mindComposeCreative(text, 0, "", "", "")
		if cv != "" {
			if n != nil {
				n.setLastCreative(cv)
			}
			return cv, "creative", trace
		}
		return "Dime el tema del poema o cuento en una frase.", "chat", trace

	case "route_memory":
		if isPersonalFact(low) {
			return "Queda anotado en esta sesión. Si más adelante preguntas, lo recupero desde aquí.", "memory", trace
		}
		return "Cuéntame el hecho con claridad («me llamo…», «vivo en…») y lo anclaré en esta sesión.", "memory", trace

	case "route_identity":
		if kv := speakFromKnowledge(text); kv != "" {
			return kv, "identity", trace
		}
		return "Soy Alset Mind: inteligencia ternaria en este nodo. Percibo el mensaje, evalúo órganos 0/1/2, recuerdo en CID y a veces muto el genoma. No soy un chatbot de predicción de tokens.", "identity", trace

	case "route_empathy":
		return templateEmotionExtended(low), "chat", trace

	case "route_advice":
		return templateHumanAdvice(low), "chat", trace

	default:
		return "", "", trace
	}
}

func ethicsStateFrom(organs []MindOrganResult) int {
	for _, o := range organs {
		if o.Name == "ethics" {
			return o.State
		}
	}
	return 0
}
