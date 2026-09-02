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
