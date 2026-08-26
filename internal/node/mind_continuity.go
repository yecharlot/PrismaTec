package node

import "strings"

func isContinuePrompt(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	// strip soft lead-ins: "no entiendo …", "no me queda …"
	for _, lead := range []string{"no entiendo ", "no comprendo ", "no me queda claro ", "no me queda ", "es que "} {
		if strings.HasPrefix(s, lead) {
			s = strings.TrimSpace(s[len(lead):])
			break
		}
	}
	keys := []string{
		"amplia el angulo", "amplía el ángulo", "amplia el ángulo", "amplía el angulo",
		"amplia el punto", "amplía el punto", "amplia el tema", "amplía el tema",
		"amplia un poco", "amplía un poco", "profundiza", "cuéntame más", "cuentame mas",
		"cuentame más", "cuéntame mas", "más sobre eso", "mas sobre eso", "más sobre ello",
		"sigue con eso", "sigue con ello", "sigue el hilo", "desarrolla", "expande",
		"más detalle", "mas detalle", "otro ángulo", "otro angulo", "retoma", "retomemos",
		"desde la memoria", "desde el corpus", "continúa", "continua", "explica mejor",
		"no entiendo", "más claro", "mas claro",
	}
	for _, k := range keys {
		if s == k || strings.HasPrefix(s, k+" ") || strings.Contains(s, k) {
			return true
		}
	}
	if s == "amplia" || s == "amplía" || s == "sigue" || s == "continúa" || s == "continua" {
		return true
	}
	// "amplia el X" / "amplía el X"
	if strings.HasPrefix(s, "amplia ") || strings.HasPrefix(s, "amplía ") {
		return true
	}
	return false
}

// isConfirmationPrompt: user asks if the last claim is right (name, fact).
func isConfirmationPrompt(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	keys := []string{
		"estás seguro", "estas seguro", "está seguro", "esta seguro",
		"seguro?", "seguro de eso", "en serio", "de verdad", "confirmas",
		"es cierto", "lo confirmas", "seguro seguro",
	}
	for _, k := range keys {
		if s == k || strings.Contains(s, k) {
			return true
		}
	}
	if s == "seguro" || s == "seguro?" {
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
	if query != "" && !isContinuePrompt(query) && !isConfirmationPrompt(query) {
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

func (n *NodoAlset) confirmMindThread(text string) string {
	if n == nil {
		return ""
	}
	n.mindLastMu.Lock()
	q, k, m, v := n.mindLastQuery, n.mindLastKnow, n.mindLastMem, n.mindLastVoice
	n.mindLastMu.Unlock()
	// Prefer name from memory episodes text
	if name := extractDeclaredName(m); name != "" {
		return "Sí: te llamas " + name + ". Lo tengo de lo que me dijiste, no lo estoy inventando."
	}
	if name := extractDeclaredName(q); name != "" {
		return "Sí: te llamas " + name + ". Lo anoté cuando me lo dijiste."
	}
	if name := extractDeclaredName(v); name != "" {
		return "Sí: te llamas " + name + "."
	}
	if m != "" {
		return "Sí, me baso en lo que guardamos: «" + truncateRunes(m, 100) + "»."
	}
	if k != "" {
		return "Sí, eso sale del corpus curado de este nodo — no es una predicción suelta."
	}
	if v != "" {
		snip := compressVoiceBlock(v, 120)
		return "Sí, me mantengo en lo último que dije: " + snip
	}
	return "No tengo un hecho reciente que confirmar. Si me dices el detalle, lo anclo."
}

// extractTopicFocus pulls an explicit topic the user wants to follow
// ("sigue ese tema de mucho gusto", "hablemos de X", "sobre X").
func extractTopicFocus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	markers := []string{
		"tema de ", "tema del ", "tema de la ", "sobre ", "acerca de ",
		"respecto a ", "respecto de ", "del tema ", "ese tema de ", "este tema de ",
	}
	for _, m := range markers {
		i := strings.Index(s, m)
		if i < 0 {
			continue
		}
		rest := strings.TrimSpace(s[i+len(m):])
		for _, sep := range []string{"?", ".", ",", "!", " y ", " por favor"} {
			if j := strings.Index(rest, sep); j > 0 {
				rest = rest[:j]
			}
		}
		rest = strings.TrimSpace(rest)
		parts := strings.Fields(rest)
		if len(parts) > 5 {
			parts = parts[:5]
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}
	return ""
}

func (n *NodoAlset) continueMindThread(text string) string {
	if n == nil {
		return ""
	}
	n.mindLastMu.Lock()
	q, k, m, v := n.mindLastQuery, n.mindLastKnow, n.mindLastMem, n.mindLastVoice
	n.mindLastMu.Unlock()
	low := strings.ToLower(text)
	wantMem := strings.Contains(low, "memoria") || strings.Contains(low, "recuerdo")
	wantCorp := strings.Contains(low, "corpus") || strings.Contains(low, "conocimiento")

	// Explicit topic from user wins over sticky name thread
	if focus := extractTopicFocus(low); focus != "" {
		if hit := speakFromKnowledge(focus); hit != "" {
			return "Sobre «" + focus + "»:\n\n" + hit
		}
		// Reuse last voice only if focus IS the last scout subject
		// (not a name mentioned inside another article, e.g. Harry text → Voldemort).
		n.mindLastMu.Lock()
		lastScout := strings.TrimSpace(n.mindLastScoutTopic)
		n.mindLastMu.Unlock()
		if lastScout != "" && topicKeysMatch(focus, lastScout) {
			if m != "" && strings.Contains(strings.ToLower(m), strings.ToLower(focus)) {
				return "Sobre «" + focus + "», de lo que hablamos:\n\n" + m
			}
			if v != "" {
				return "Sobre «" + focus + "»:\n\n" + compressVoiceBlock(v, 280)
			}
		}
		// New entity focus → empty so runMindTick can dispatch MindScoutWeb
		return ""
	}

	// Name thread only if user did not point elsewhere
	nameQ := strings.Contains(strings.ToLower(q), "nombre") || strings.Contains(strings.ToLower(q), "llamo") ||
		extractDeclaredName(q) != "" || extractDeclaredName(m) != "" || extractDeclaredName(v) != ""
	if nameQ && !wantCorp && !wantMem {
		name := extractDeclaredName(m)
		if name == "" {
			name = extractDeclaredName(q)
		}
		if name == "" {
			name = extractDeclaredName(v)
		}
		if name != "" {
			return "Sobre tu nombre: te llamas " + name + ". Lo guardé cuando me lo dijiste."
		}
	}

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
	if v != "" {
		return "Siguiendo el hilo:\n\n" + compressVoiceBlock(v, 280)
	}
	return "No tengo aún un hilo claro que ampliar. Pregunta por un tema o un hecho que hayamos guardado."
}
