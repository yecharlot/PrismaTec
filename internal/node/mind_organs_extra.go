package node

import (
	"encoding/json"
	"net/http"
	"strings"
)

// evaluateCuriosity: novel / reflective topics → urge to ask (never absorbs ethics).
// 0 quiet · 1 mild follow-up · 2 strong open question.
// Knowledge does not kill curiosity: a solid answer can still invite a follow-up.
func evaluateCuriosity(text string, memSpeak string, g MindGenome) MindOrganResult {
	s := strings.ToLower(strings.TrimSpace(text))
	score := 0.0
	words := tokenizeMind(s)
	if isCalmChat(s) || isMemoryQuery(s) || isPersonalFact(s) || isDestructiveOrder(s) {
		return MindOrganResult{Name: "curiosity", State: 0, Label: labelForOrgan("curiosity", 0), Inputs: []float64{0.1, 0, 0}}
	}
	if strings.Contains(s, "sabes qué") || strings.Contains(s, "sabes que") ||
		strings.Contains(s, "te cuento") || strings.Contains(s, "descubr") ||
		strings.Contains(s, "documental") || strings.Contains(s, "aprendí") ||
		strings.Contains(s, "aprendi") || strings.Contains(s, "me enteré") {
		score = 0.9
	}
	reflect := []string{"metáfora", "metafora", "significa", "existencia", "consciencia",
		"conciencia", "pensamiento", "el todo", "esencia", "humano", "vivimos",
		"primero debemos", "la vida es", "como si", "pareces", "eres como"}
	for _, r := range reflect {
		if strings.Contains(s, r) {
			score += 0.45
			break
		}
	}
	if len(words) >= 4 {
		score += 0.3
	}
	if len(s) > 50 {
		score += 0.15
	}
	unknown := []string{"agujero negro", "agujeros negros", "cuántica", "galaxia",
		"neurociencia", "bitcoin", "blockchain", "filosofía de", "filosofia de",
		"relatividad", "dinosaurio", "océano", "oceano", "mito", "poesía", "poesia"}
	for _, u := range unknown {
		if strings.Contains(s, u) {
			score = 0.92
			break
		}
	}
	if speakFromKnowledge(text) != "" && score < 0.5 {
		score = 0.42
	}
	if memSpeak != "" && score < 0.35 {
		score = 0.3
	}
	cut := g.CuriosityCut
	if cut <= 0 {
		cut = 0.4
	}
	st := 0
	if score >= cut+0.25 {
		st = 2
	} else if score >= cut {
		st = 1
	}
	return MindOrganResult{Name: "curiosity", State: st, Label: labelForOrgan("curiosity", st), Inputs: []float64{score, cut, float64(len(words))}}
}

// evaluateHumor: comic intent or playful comparison in user message.
func evaluateHumor(text string, g MindGenome) MindOrganResult {
	s := strings.ToLower(strings.TrimSpace(text))
	score := 0.0
	keys := []string{"jaja", "jeje", "haha", "lol", "chiste", "broma", "risa", "😂", "😄",
		"qué hace un", "que hace un", "por qué los", "por que los", "knock", "punchline",
		"eres como", "pareces un", "sin varita", "sin varitas", "harry potter", "mago"}
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
		cut = 0.3
	}
	st := 0
	if score >= cut+0.25 {
		st = 2
	} else if score >= cut {
		st = 1
	}
	return MindOrganResult{Name: "humor", State: st, Label: labelForOrgan("humor", st), Inputs: []float64{score, cut, 0}}
}

// curiosityVoice returns a follow-up question to APPEND (never replaces a solid answer).
func curiosityVoice(text string, st int) string {
	if st <= 0 {
		return ""
	}
	low := strings.ToLower(text)
	if st >= 2 {
		if strings.Contains(low, "metáfora") || strings.Contains(low, "metafora") || strings.Contains(low, "la vida es") {
			return "¿Qué significa para ti esa imagen — qué parte quieres que ancle en memoria CID?"
		}
		if strings.Contains(low, "humano") {
			return "¿Qué rasgo humano te importa más contrastar con este organismo ternario?"
		}
		if strings.Contains(low, "consciencia") || strings.Contains(low, "conciencia") {
			return "¿Hablas de consciencia como experiencia subjetiva o como capacidad de monitorear el propio proceso?"
		}
		return "¿Quieres profundizar y lo anclo en un episodio CID, o preferimos otro ángulo?"
	}
	// mild
	if strings.Contains(low, "pensamiento") || strings.Contains(low, "todo") || strings.Contains(low, "exist") {
		return "Si quieres, formula una frase que deba recordar tal cual."
	}
	return "Puedo guardar un matiz tuyo en CID si lo dejas explícito."
}

// humorVoice: light tint line to APPEND when humor organ is active.
func humorVoice(text string, st int) string {
	if st <= 0 {
		return ""
	}
	low := strings.ToLower(text)
	if st >= 2 {
		if strings.Contains(low, "varita") || strings.Contains(low, "mago") || strings.Contains(low, "harry") {
			return "Prefiero verme como un mago con siete órganos y sin varita: el hechizo más útil suele ser una pregunta bien puesta."
		}
		if strings.Contains(low, "abeja") && strings.Contains(low, "gimnasio") {
			return "¡Zum-ba! Humor en 2 — ¿tienes otra?"
		}
		return "El órgano humor marcó 2: tono ligero admitido. Seguimos en el campo sin disfrazar el juicio 0/1/2."
	}
	return "Sonrisa breve (humor 1). El campo sigue en serio cuando haga falta."
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
