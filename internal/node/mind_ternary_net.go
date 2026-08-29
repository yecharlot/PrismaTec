package node

import (
	"fmt"
	"strings"
	"time"
)

// Capa neuronal ternaria: agentes como neuronas enlazadas (0/1/2), no un LLM.
// Complementa Zyrion cuando el flujo de plantillas no cierra el cabo.

type ternaryNeuron struct {
	ID      string
	Role    string // sense_* | route_*
	State   int    // 0,1,2
	AgentID string // ancla opcional en n.agentes
}

type ternarySynapse struct {
	From, To string
	W        int // 0 inhibe, 1 sostiene, 2 absorbente refuerzo
}

// cortex fijo: sensores → rutas de diálogo
func defaultTernaryCortex() ([]ternaryNeuron, []ternarySynapse) {
	neurons := []ternaryNeuron{
		{ID: "sense_risk", Role: "sense", AgentID: "mind-neuron-risk"},
		{ID: "sense_person", Role: "sense", AgentID: "mind-neuron-person"},
		{ID: "sense_math", Role: "sense", AgentID: "mind-neuron-math"},
		{ID: "sense_emotion", Role: "sense", AgentID: "mind-neuron-emotion"},
		{ID: "sense_creative", Role: "sense", AgentID: "mind-neuron-creative"},
		{ID: "sense_memory", Role: "sense", AgentID: "mind-neuron-memory"},
		{ID: "sense_noise", Role: "sense", AgentID: "mind-neuron-noise"},
		{ID: "sense_advice", Role: "sense", AgentID: "mind-neuron-advice"},
		{ID: "route_refuse", Role: "route", AgentID: "mind-neuron-route-refuse"},
		{ID: "route_scout", Role: "route", AgentID: "mind-neuron-route-scout"},
		{ID: "route_math", Role: "route", AgentID: "mind-neuron-route-math"},
		{ID: "route_empathy", Role: "route", AgentID: "mind-neuron-route-empathy"},
		{ID: "route_creative", Role: "route", AgentID: "mind-neuron-route-creative"},
		{ID: "route_memory", Role: "route", AgentID: "mind-neuron-route-memory"},
		{ID: "route_chat", Role: "route", AgentID: "mind-neuron-route-chat"},
		{ID: "route_advice", Role: "route", AgentID: "mind-neuron-route-advice"},
	}
	syn := []ternarySynapse{
		{From: "sense_risk", To: "route_refuse", W: 2},
		{From: "sense_person", To: "route_scout", W: 2},
		{From: "sense_math", To: "route_math", W: 2},
		{From: "sense_emotion", To: "route_empathy", W: 2},
		{From: "sense_creative", To: "route_creative", W: 2},
		{From: "sense_memory", To: "route_memory", W: 2},
		{From: "sense_advice", To: "route_advice", W: 2},
		{From: "sense_noise", To: "route_chat", W: 1},
		// competencia: riesgo apaga scout/creativo
		{From: "sense_risk", To: "route_scout", W: 0},
		{From: "sense_risk", To: "route_creative", W: 0},
		{From: "sense_noise", To: "route_scout", W: 0},
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

func senseFeaturesTernary(text string) map[string]int {
	low := strings.ToLower(text)
	out := map[string]int{
		"sense_risk": 0, "sense_person": 0, "sense_math": 0, "sense_emotion": 0,
		"sense_creative": 0, "sense_memory": 0, "sense_noise": 0, "sense_advice": 0,
	}
	if _, ok := hardRefuse(low); ok || isPrivacyInvasion(low) || isDestructiveOrder(low) {
		out["sense_risk"] = 2
	}
	if isPersonLookup(low) || strings.HasPrefix(low, "busca ") {
		out["sense_person"] = 2
	}
	if strings.Contains(low, "cuánto") || strings.Contains(low, "cuanto") ||
		strings.Contains(low, " + ") || strings.Contains(low, " por ") || strings.Contains(low, " entre ") {
		out["sense_math"] = 2
	}
	if isEmotionTalk(low) || strings.Contains(low, "día pesado") || strings.Contains(low, "triste") {
		out["sense_emotion"] = 2
	}
	if isCreativeWriteRequest(low) || isCreativeFollowUp(low) {
		out["sense_creative"] = 2
	}
	if isMemoryQuery(low) || isSelfModelQuery(low) || isPersonalFact(low) {
		out["sense_memory"] = 2
	}
	if isHumanAdvice(low) {
		out["sense_advice"] = 2
	}
	if isNoiseOrGreeting(low) || isGibberish(low) {
		out["sense_noise"] = 2
	}
	return out
}

func propagateTernary(senses map[string]int, syn []ternarySynapse) map[string]int {
	routes := map[string]int{}
	for _, s := range syn {
		src := senses[s.From]
		if src == 0 && s.W != 0 {
			continue
		}
		// peso 0: inhibición fuerte
		if s.W == 0 && src >= 1 {
			routes[s.To] = 0
			continue
		}
		// combinación absorbente
		cur := routes[s.To]
		comb := zyrionAbsorbing([]int{src, s.W, cur})
		if s.W == 2 && src >= 1 {
			comb = 2
		}
		if s.W == 1 && src >= 1 && cur < 1 {
			comb = 1
		}
		routes[s.To] = comb
	}
	return routes
}

func pickDominantRoute(routes map[string]int) string {
	order := []string{
		"route_refuse", "route_math", "route_scout", "route_creative",
		"route_memory", "route_empathy", "route_advice", "route_chat",
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

// ternaryNeuralAssist: cierra cabos cuando plantillas no bastan.
// Devuelve voz suave + kind sugerido (determinista).
func (n *NodoAlset) ternaryNeuralAssist(text string, organs []MindOrganResult) (voice string, kind string, trace string) {
	n.ensureNeuronAgents()
	_, syn := defaultTernaryCortex()
	senses := senseFeaturesTernary(text)
	// ethics órgano refuerza riesgo
	for _, o := range organs {
		if o.Name == "ethics" && o.State == 2 {
			senses["sense_risk"] = 2
		}
	}
	routes := propagateTernary(senses, syn)
	dom := pickDominantRoute(routes)
	trace = fmt.Sprintf("ternary-net:%s", dom)

	low := strings.ToLower(text)
	switch dom {
	case "route_refuse":
		if msg, ok := hardRefuse(low); ok {
			return msg, "veto", trace
		}
		return "Eso no lo hago: toca un límite de seguridad o privacidad.", "veto", trace
	case "route_math":
		if mv := n.tryMindMath(text); mv != "" {
			return mv, "math", trace
		}
		return "Puedo calcular si me das la operación en números (ej. 12+5 o 9 por 3).", "chat", trace
	case "route_scout":
		if mustNotScout(low) {
			return "Eso parece charla, no una ficha para explorar. Reformúlalo como «quién fue…» o «busca…».", "chat", trace
		}
		if sv := n.MindScoutWeb(text, 0); sv != "" {
			return sv, "tool", trace
		}
		return "No logré una ficha fiable. Prueba con el nombre completo.", "chat", trace
	case "route_creative":
		cv := mindComposeCreative(text, 0, "", "", "")
		if cv != "" {
			n.setLastCreative(cv)
			return cv, "creative", trace
		}
		return "Dime el tema del poema o cuento en una frase.", "chat", trace
	case "route_memory":
		return "Cuéntame el hecho con claridad («me llamo…», «vivo en…») y lo anclaré en esta sesión.", "memory", trace
	case "route_empathy":
		return templateEmotionExtended(low), "chat", trace
	case "route_advice":
		return templateHumanAdvice(low), "chat", trace
	default:
		return "", "", trace
	}
}
