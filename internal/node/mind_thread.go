package node

import (
	"strings"
)

// P3 — Hilo conversacional: topic frame, expect, referencias.

const (
	expectNone    = "none"
	expectFact    = "fact"
	expectClarify = "clarify"
	expectClose   = "close"
)

// extractTopicFrame: etiqueta corta del turno para el frame de sesión.
func extractTopicFrame(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" || isCalmChat(low) || isNoiseOrGreeting(low) || isThanksTalk(low) || isByeTalk(low) || isConfirmTalk(low) {
		return ""
	}
	if isIdentityTalk(low) {
		return "identidad"
	}
	if isMemoryQuery(low) {
		return "memoria"
	}
	if isCapabilityQuestion(low) {
		return "capacidades"
	}
	// quitar muletillas
	for _, p := range []string{"vamos a hablar de ", "hablemos de ", "quiero hablar de ", "sobre ", "de "} {
		if strings.HasPrefix(low, p) {
			low = strings.TrimSpace(low[len(p):])
			break
		}
	}
	// primera frase corta
	if i := strings.IndexAny(low, ".?!"); i > 0 {
		low = strings.TrimSpace(low[:i])
	}
	words := strings.Fields(low)
	if len(words) > 6 {
		words = words[:6]
	}
	out := strings.Join(words, " ")
	if len([]rune(out)) > 48 {
		out = string([]rune(out)[:48])
	}
	return out
}

// isReferentialCue: el usuario apunta al hilo previo.
func isReferentialCue(low string) bool {
	low = strings.ToLower(strings.TrimSpace(low))
	cues := []string{
		"eso", "eso?", "y eso", "y eso?", "lo de antes", "lo anterior",
		"de eso", "sobre eso", "ese tema", "el tema", "aquello",
		"lo mismo", "sigue con eso", "retoma", "retomar",
		"a qué te refieres", "a que te refieres", "qué era eso", "que era eso",
	}
	for _, c := range cues {
		if low == c || strings.HasPrefix(low, c+" ") || strings.HasPrefix(low, c+"?") {
			return true
		}
	}
	// mensajes muy cortos con eso/antes
	if len(strings.Fields(low)) <= 5 && (strings.Contains(low, "eso") || strings.Contains(low, "antes") ||
		strings.Contains(low, "anterior")) {
		if !isMemoryQuery(low) && !isIdentityTalk(low) {
			return true
		}
	}
	return false
}

// isMisunderstandRepair: el usuario corrige el marco.
func isMisunderstandRepair(low string) bool {
	low = strings.ToLower(strings.TrimSpace(low))
	keys := []string{
		"no me refería", "no me referia", "no me refiero", "no era eso",
		"no es eso", "me malinterpretaste", "no entendiste", "no te entendí así",
		"no te entendi asi", "quiero decir", "me refiero a",
	}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// speakThreadReference: resuelve «eso / lo de antes» con el frame de sesión.
func speakSoftFollowUp(text string, sess *mindSessionState) string {
	low := strings.ToLower(strings.TrimSpace(text))
	low = strings.TrimSuffix(low, "?")
	low = strings.TrimSpace(low)
	if low == "como" || low == "cómo" || low == "cómo es" || low == "como es" {
		if sess != nil && (sess.ActiveTopic == "identidad" || sess.LastAct == "meta" ||
			strings.Contains(sess.LastUserFrame, "ti") || strings.Contains(sess.LastUserFrame, "eres")) {
			return "En la práctica: en cada mensaje mido el campo, elijo 0/1/2 por órgano, consulto memoria CID si aplica y armo una respuesta. No improviso tokens: aplico reglas y hechos anclados."
		}
		if sess != nil && sess.ActiveTopic != "" {
			return "Sobre «" + sess.ActiveTopic + "»: dime qué ángulo quieres (detalle, ejemplo o cruce con un hecho tuyo)."
		}
		return "¿Cómo… qué? Completa: cómo funciona X, cómo me llamo, cómo calculas…"
	}
	if strings.HasPrefix(low, "me gusta") || low == "me gusta eso" || low == "me gusta" {
		return "Bien. ¿Quieres profundizar ese punto, anclarlo como hecho o cambiar de tema?"
	}
	// "con selena gomez" / "con X" tras sonda
	if strings.HasPrefix(low, "con ") && len(strings.Fields(low)) <= 5 {
		topic := strings.TrimSpace(strings.TrimPrefix(low, "con "))
		if topic != "" && sess != nil {
			sess.ActiveTopic = topic
		}
		return "" // dejar a continue/scout
	}
	return ""
}

func speakShortProbe(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	low = strings.TrimSuffix(low, "?")
	low = strings.TrimSpace(low)
	if low == "lugar" || low == "ciudad" {
		return "Si te pregunté por un lugar, era por si habías dicho ciudad o dónde vives. Si no lo has contado, puedes decir «vivo en…» y lo anclo."
	}
	if strings.Contains(low, "alarma absorbente") || low == "absorbente" || low == "alarma" {
		if k := speakFromKnowledge("alarma absorbente estado 2"); k != "" {
			return k
		}
		return "En Zyrion el estado 2 es absorbente: si un órgano crítico está en 2, no se promedia con ceros. Evita diluir un veto o una alarma."
	}
	return ""
}

func speakThreadReference(text string, sess *mindSessionState) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if sess == nil {
		return ""
	}
	if isMisunderstandRepair(low) {
		sess.Expect = expectClarify
		return "De acuerdo, reencuadramos. Dime en una frase a qué te referías y sigo ese hilo."
	}
	if !isReferentialCue(low) {
		return ""
	}
	topic := strings.TrimSpace(sess.ActiveTopic)
	last := strings.TrimSpace(sess.LastUserFrame)
	if topic == "" && last == "" && sess.LastQuery != "" {
		last = sess.LastQuery
	}
	if topic == "" && last == "" {
		return "No tengo un hilo previo claro en esta sesión. Nombra el tema en pocas palabras."
	}
	label := topic
	if label == "" {
		label = compressVoiceBlock(last, 60)
	}
	return "Me refiero al hilo de «" + label + "». ¿Quieres profundizar, anclar un hecho o cambiar de tema?"
}

// updateThreadFrame: actualiza topic/expect tras un turno.
func updateThreadFrame(sess *mindSessionState, text, kind string) {
	if sess == nil {
		return
	}
	low := strings.ToLower(strings.TrimSpace(text))
	sess.LastUserFrame = compressVoiceBlock(strings.TrimSpace(text), 80)

	if isByeTalk(low) {
		sess.Expect = expectClose
		return
	}
	if isMisunderstandRepair(low) {
		sess.Expect = expectClarify
		return
	}
	if isPersonalFact(low) {
		sess.Expect = expectNone
		if frame := extractTopicFrame(text); frame != "" {
			sess.ActiveTopic = frame
		}
		return
	}
	if isReferentialCue(low) {
		// mantiene ActiveTopic
		sess.Expect = expectNone
		return
	}
	if frame := extractTopicFrame(text); frame != "" {
		// no pisar con saludo
		if !isCalmChat(low) && !isNoiseOrGreeting(low) {
			sess.ActiveTopic = frame
		}
	}
	if kind == "memory" || kind == "identity" || kind == "knowledge" {
		sess.Expect = expectNone
	}
	if strings.Contains(low, "no te entend") || strings.Contains(low, "explica mejor") {
		sess.Expect = expectClarify
	}
}

// speakLabLight: en charla social/content evita empujar jerga de laboratorio.
func speakLabLight(kind, voice string) string {
	if kind != "chat" && kind != "identity" {
		return voice
	}
	// no reescribir voces largas de knowledge
	if len([]rune(voice)) > 280 {
		return voice
	}
	return voice
}

// scoreDialogThreadNaturalness: secuencias fijas de hilo (P3).
func scoreDialogThreadNaturalness() (ok, total int, details []string) {
	scripts := [][]string{
		{"hola", "de cualquier cosa", "me llamo Nuria", "cómo me llamo?", "y eso?", "hasta luego"},
		{"vamos a empezar", "quién eres", "ok", "cambiemos de tema", "gracias"},
	}
	for si, steps := range scripts {
		sess := &mindSessionState{Phase: phaseOpening}
		total++
		pass := true
		for _, step := range steps {
			// simular frame
			if v := speakThreadReference(step, sess); v != "" {
				if strings.Contains(strings.ToLower(v), "generar código") {
					pass = false
					break
				}
			} else if classifySpeechAct(step) == actSocial {
				v, _ := speakSpeechAct(step, sess)
				if v == "" || strings.Contains(strings.ToLower(v), "no tengo una respuesta firme") {
					pass = false
					break
				}
			}
			updateSessionAfterTurn(sess, step, "chat")
			updateThreadFrame(sess, step, "chat")
		}
		if pass {
			ok++
		} else {
			details = append(details, "script_fail:"+string(rune('A'+si)))
		}
	}
	return ok, total, details
}
