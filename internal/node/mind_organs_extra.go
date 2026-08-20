package node

import (
	"encoding/json"
	"net/http"
	"strings"
)

// evaluateCuriosity: unknown / novel topics outside knowledge+memory → ask to learn.
// 0 quiet · 1 mild · 2 strong urge to ask (never absorbs ethics).
func evaluateCuriosity(text string, memSpeak string, g MindGenome) MindOrganResult {
	s := strings.ToLower(strings.TrimSpace(text))
	score := 0.0
	// known knowledge or memory → low curiosity
	if speakFromKnowledge(text) != "" || memSpeak != "" {
		return MindOrganResult{Name: "curiosity", State: 0, Label: labelForOrgan("curiosity", 0), Inputs: []float64{0, 0, 0}}
	}
	if isCalmChat(s) || isIdentityTalk(s) || isMemoryQuery(s) || isPersonalFact(s) {
		return MindOrganResult{Name: "curiosity", State: 0, Label: labelForOrgan("curiosity", 0), Inputs: []float64{0.1, 0, 0}}
	}
	// explicit teaching invitation
	if strings.Contains(s, "sabes qué") || strings.Contains(s, "sabes que") ||
		strings.Contains(s, "te cuento") || strings.Contains(s, "descubr") ||
		strings.Contains(s, "documental") || strings.Contains(s, "aprendí") ||
		strings.Contains(s, "aprendi") || strings.Contains(s, "me enteré") {
		score = 0.85
	}
	// long novel statement without our domain keywords
	words := tokenizeMind(s)
	if len(words) >= 4 {
		score += 0.35
	}
	if len(s) > 60 {
		score += 0.2
	}
	// domain we don't cover yet (physics, biology, etc.)
	unknown := []string{"agujero negro", "agujeros negros", "cuántica", "adn", "galaxia",
		"neurociencia", "bitcoin", "blockchain", "filosofía de", "filosofia de",
		"relatividad", "dinosaurio", "océano", "oceano"}
	for _, u := range unknown {
		if strings.Contains(s, u) {
			score = 0.9
			break
		}
	}
	cut := g.CuriosityCut
	if cut <= 0 {
		cut = 0.55
	}
	st := 0
	if score >= cut+0.25 {
		st = 2
	} else if score >= cut {
		st = 1
	}
	return MindOrganResult{Name: "curiosity", State: st, Label: labelForOrgan("curiosity", st), Inputs: []float64{score, cut, float64(len(words))}}
}

// evaluateHumor: comic intent in user message.
func evaluateHumor(text string, g MindGenome) MindOrganResult {
	s := strings.ToLower(strings.TrimSpace(text))
	score := 0.0
	keys := []string{"jaja", "jeje", "haha", "lol", "chiste", "broma", "risa", "😂", "😄",
		"qué hace un", "que hace un", "por qué los", "por que los", "knock", "punchline"}
	for _, k := range keys {
		if strings.Contains(s, k) {
			score = 0.85
			break
		}
	}
	if strings.Contains(s, "?") && (strings.Contains(s, "abeja") || strings.Contains(s, "cruzar") ||
		strings.Contains(s, "médico") || strings.Contains(s, "medico")) {
		score = 0.8
	}
	cut := g.HumorCut
	if cut <= 0 {
		cut = 0.5
	}
	st := 0
	if score >= cut+0.25 {
		st = 2
	} else if score >= cut {
		st = 1
	}
	return MindOrganResult{Name: "humor", State: st, Label: labelForOrgan("humor", st), Inputs: []float64{score, cut, 0}}
}

func curiosityVoice(text string, st int) string {
	if st >= 2 {
		return "Eso no está en mi conocimiento curado ni en episodios recientes. ¿Me cuentas un poco más? Si lo explicas, puedo guardarlo en memoria CID y aprender en este nodo."
	}
	if st == 1 {
		return "Me suena a terreno nuevo para este organismo. Si quieres, profundiza y lo anclo en un episodio."
	}
	return ""
}

func humorVoice(text string, st int) string {
	if st >= 2 {
		low := strings.ToLower(text)
		if strings.Contains(low, "abeja") && strings.Contains(low, "gimnasio") {
			return "¡Zum-ba! Buena. El campo registró humor alto — ¿tienes otra?"
		}
		return "Ja — el órgano humor está en 2. Buena esa. ¿Sigues en tono ligero o volvemos a lo serio del nodo?"
	}
	if st == 1 {
		return "Sonrisa registrada (humor 1). Sigo en el campo — puedes seguir con el chiste o cambiar de tema."
	}
	return ""
}

// handleMindFeedback: POST {"feedback":"up"|"down"} for auto-calibration.
func (n *NodoAlset) handleMindFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var body struct {
		Feedback string `json:"feedback"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "bad json", 400)
		return
	}
	fb := strings.ToLower(strings.TrimSpace(body.Feedback))
	up := fb == "up" || fb == "thumbsup" || fb == "+1" || fb == "bien"
	down := fb == "down" || fb == "thumbsdown" || fb == "-1" || fb == "mal"
	if !up && !down {
		http.Error(w, `feedback must be "up" or "down"`, 400)
		return
	}
	g := applyMindFeedback(up)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"feedback": map[string]bool{"up": up, "down": down},
		"genome":   g,
		"note":     "auto-calibrate nudge applied (ethics thresholds protected)",
	})
}
