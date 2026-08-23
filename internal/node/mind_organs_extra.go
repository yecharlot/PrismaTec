package node

import (
	"encoding/json"
	"net/http"
	"strings"
)

// evaluateCuriosity: novel / reflective topics → urge to ask (never absorbs ethics).
// 0 quiet · 1 mild follow-up · 2 strong open question.
// Knowledge does not kill curiosity: a solid answer can still invite a follow-up.
func evaluateCuriosity(text string, g MindGenome, memSpeak string) MindOrganResult {
	s := strings.ToLower(strings.TrimSpace(text))
	words := strings.Fields(s)
	score := 0.0

	// Explicit teaching / learning invitations — highest attention
	teach := []string{"enséñame", "ensename", "aprende esto", "quiero que aprendas", "recuerda que",
		"guarda que", "ancla esto", "te cuento", "descubrí", "descubri", "aprendí", "aprendi",
		"me enteré", "me entere", "documental", "no está en tu corpus", "no esta en tu corpus"}
	for _, k := range teach {
		if strings.Contains(s, k) {
			score = 0.95
			break
		}
	}

	// Open gaps: user signals ignorance or asks Mind to discover
	if score < 0.9 {
		gap := []string{"no sé", "no se ", "no tengo idea", "qué opinas de", "que opinas de",
			"has oído", "has oido", "sabes algo de", "te suena", "explícame de cero", "explicame de cero"}
		for _, k := range gap {
			if strings.Contains(s, k) {
				score = 0.88
				break
			}
		}
	}

	// Reflective / philosophical hooks
	if score < 0.85 {
		reflect := []string{"metáfora", "metafora", "significa", "existencia", "consciencia",
			"conciencia", "pensamiento", "el todo", "esencia", "humano", "vivimos",
			"primero debemos", "la vida es", "como si", "pareces", "eres como"}
		for _, r := range reflect {
			if strings.Contains(s, r) {
				score += 0.5
				break
			}
		}
	}

	// Domains outside curated core (raise curiosity)
	if score < 0.9 {
		unknown := []string{"agujero negro", "agujeros negros", "cuántica", "galaxia",
			"neurociencia", "bitcoin", "blockchain", "filosofía de", "filosofia de",
			"relatividad", "dinosaurio", "océano", "oceano", "mito", "poesía", "poesia",
			"receta", "cocina", "fútbol", "futbol", "política", "politica"}
		for _, u := range unknown {
			if strings.Contains(s, u) {
				score = 0.9
				break
			}
		}
	}

	// Network expertise: if asking about Alset and we have knowledge, moderate curiosity (answer first)
	know := speakFromKnowledge(text)
	alsetTopic := strings.Contains(s, "alset") || strings.Contains(s, "gen ") || strings.Contains(s, "red ") ||
		strings.Contains(s, "cloudflare") || strings.Contains(s, "cid") || strings.Contains(s, "nodo")
	if alsetTopic && know != "" {
		if score < 0.35 {
			score = 0.35 // soft follow-up possible, not silence
		}
	} else if alsetTopic && know == "" {
		score = 0.85 // should know network — gap is urgent
	}

	if know != "" && score < 0.5 && !alsetTopic {
		score = 0.4
	}
	if memSpeak != "" && score < 0.35 {
		score = 0.32
	}

	// Length / richness of utterance
	if len(words) >= 6 {
		score += 0.2
	}
	if len(s) > 60 {
		score += 0.12
	}
	if strings.Contains(s, "?") {
		score += 0.08
	}

	if score > 1 {
		score = 1
	}

	cut := g.CuriosityCut
	if cut <= 0 {
		cut = 0.38 // slightly more attentive default
	}
	st := 0
	if score >= cut+0.22 {
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
	s := strings.ToLower(strings.TrimSpace(text))
	if st >= 2 {
		if strings.Contains(s, "alset") || strings.Contains(s, "red ") {
			return "Quiero afinar este punto de la red: ¿me das el detalle que falta o un ejemplo concreto para anclarlo?"
		}
		if strings.Contains(s, "aprend") || strings.Contains(s, "enséñ") || strings.Contains(s, "ensenh") {
			return "Si lo formularas en una frase cerrada del tipo «recuerda que X es Y», lo puedo marcar para memoria."
		}
		return "No lo tengo completo en corpus. ¿Me cuentas un hecho concreto para aprenderlo sin inventar?"
	}
	// st == 1
	if speakFromKnowledge(text) != "" {
		return "Si quieres, bajamos a un detalle más fino o lo cruzamos con un gen o un recuerdo tuyo."
	}
	return "¿Seguimos este hilo o hay algo de la red Alset / del nodo que quieras revisar?"
}


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
