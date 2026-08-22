package node

import "strings"

func isContinuePrompt(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	keys := []string{
		"amplia el angulo", "amplía el ángulo", "amplia el ángulo", "amplía el angulo",
		"amplia el tema", "amplía el tema", "profundiza", "cuéntame más", "cuentame mas",
		"cuentame más", "cuéntame mas", "más sobre eso", "mas sobre eso", "más sobre ello",
		"sigue con eso", "sigue con ello", "sigue el hilo", "desarrolla", "expande",
		"más detalle", "mas detalle", "otro ángulo", "otro angulo", "retoma", "retomemos",
		"desde la memoria", "desde el corpus", "continúa", "continua",
	}
	for _, k := range keys {
		if s == k || strings.HasPrefix(s, k+" ") || strings.Contains(s, k) {
			return true
		}
	}
	// short "amplia" / "sigue" alone
	if s == "amplia" || s == "amplía" || s == "sigue" || s == "continúa" || s == "continua" {
		return true
	}
	return false
}

func (n *NodoAlset) rememberMindThread(query, know, mem, voice string) {
	if n == nil {
		return
	}
	n.mindLastMu.Lock()
	defer n.mindLastMu.Unlock()
	if query != "" && !isContinuePrompt(query) {
		n.mindLastQuery = query
	}
	if know != "" {
		n.mindLastKnow = know
	}
	if mem != "" {
		n.mindLastMem = mem
	}
	if voice != "" {
		n.mindLastVoice = voice
	}
}

func (n *NodoAlset) continueMindThread(text string) string {
	if n == nil {
		return ""
	}
	n.mindLastMu.Lock()
	q, k, m := n.mindLastQuery, n.mindLastKnow, n.mindLastMem
	n.mindLastMu.Unlock()
	low := strings.ToLower(text)
	wantMem := strings.Contains(low, "memoria") || strings.Contains(low, "recuerdo")
	wantCorp := strings.Contains(low, "corpus") || strings.Contains(low, "conocimiento")

	if wantMem && m != "" {
		return "Desde lo que recordamos:\n\n" + m
	}
	if wantCorp && k != "" {
		if rel := relatedKnowledge(q, k); rel != "" {
			return "Desde el corpus, otro ángulo:\n\n" + rel
		}
		return "Desde el corpus, volviendo al hilo:\n\n" + k
	}
	if k != "" {
		if rel := relatedKnowledge(q, k); rel != "" {
			return "Siguiendo el hilo:\n\n" + rel
		}
		return "Ampliando lo anterior:\n\n" + k
	}
	if m != "" {
		return "Siguiendo desde lo recordado:\n\n" + m
	}
	return "No tengo aún un hilo claro que ampliar. Pregunta por un tema o un hecho que hayamos guardado."
}
