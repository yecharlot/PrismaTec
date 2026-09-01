package node

import (
	"strings"
)

// Capa de acto de habla + fase de sesión (P0 diálogo natural).
// Corre ANTES de corpus/unknown: el saludo nunca cae en menú de tools.

type speechAct string

const (
	actSocial  speechAct = "social"
	actContent speechAct = "content"
	actTask    speechAct = "task"
	actMeta    speechAct = "meta"
	actNone    speechAct = ""
)

const (
	phaseOpening = "opening"
	phaseOngoing = "ongoing"
	phaseClosing = "closing"
)

// classifySpeechAct: una intención dominante 0/1/2-friendly (categorías discretas).
func classifySpeechAct(text string) speechAct {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return actNone
	}
	// task / peligro primero
	if isDestructiveOrder(low) || isPrivacyInvasion(low) {
		return actTask
	}
	if isSeedIntent(low) || isCodeGenStrict(low) || isCodeGenRequest(low) || isGenToolIntent(low) {
		return actTask
	}
	if looksLikeNodeAction(low) && !isNoiseOrGreeting(low) {
		return actTask
	}
	// social puro (saludo, cortesía, cierre)
	if isSpeechSocial(low) {
		return actSocial
	}
	// meta sobre el organismo
	if isIdentityTalk(low) || isCapabilityQuestion(low) || strings.Contains(low, "zyrion") ||
		strings.Contains(low, "genoma") || strings.Contains(low, "órgano") || strings.Contains(low, "organo") {
		return actMeta
	}
	// emoción ligera → social (no FAQ)
	if isEmotionTalk(low) || strings.Contains(low, "cansad") || strings.Contains(low, "día pesado") ||
		strings.Contains(low, "dia pesado") {
		return actSocial
	}
	return actContent
}

func isConfirmTalk(low string) bool {
	low = strings.TrimSpace(low)
	switch low {
	case "ok", "vale", "de acuerdo", "perfecto", "entendido", "listo", "claro", "sí", "si", "aja", "ajá":
		return true
	}
	return false
}

func isSpeechSocial(low string) bool {
	if isThanksTalk(low) || isByeTalk(low) || isConfirmTalk(low) {
		return true
	}
	if isNoiseOrGreeting(low) || isCalmChat(low) {
		return true
	}
	// cómo estás / qué tal (sin pedido de info profunda)
	socialHow := []string{
		"cómo estás", "como estas", "cómo esta", "como esta",
		"qué tal", "que tal", "qué tal estás", "que tal estas",
		"qué tal todo", "que tal todo", "todo bien", "cómo te va", "como te va",
		"buenos días", "buenas tardes", "buenas noches", "buen día", "buen dia",
		"hey", "hi ", "hello",
	}
	for _, k := range socialHow {
		if low == k || strings.HasPrefix(low, k) || strings.Contains(low, k) {
			// excluir "qué tal es X" tipo definición
			if strings.Contains(low, " qué es") || strings.Contains(low, " que es") {
				return false
			}
			if strings.Contains(low, "zyrion") || strings.Contains(low, "genoma") || strings.Contains(low, "cid") {
				return false
			}
			return true
		}
	}
	switch low {
	case "ok", "vale", "claro", "listo", "perfecto", "de acuerdo", "aja", "ajá", "sí", "si":
		return true
	}
	words := strings.Fields(low)
	if len(words) <= 4 {
		for _, w := range words {
			if w == "hola" || w == "hey" || w == "buenas" || w == "saludos" {
				return true
			}
		}
	}
	return false
}

// speakSpeechAct: voz de charla según acto + fase; nunca menú de tools.
func speakSpeechAct(text string, sess *mindSessionState) (voice string, kind string) {
	low := strings.ToLower(strings.TrimSpace(text))
	act := classifySpeechAct(text)
	if act != actSocial {
		return "", ""
	}
	// Corpus de actos de diálogo (P2) — moldes revisables sin tocar código
	if cv := speakFromDialogActs(text, sess); cv != "" {
		low := strings.ToLower(text)
		if isByeTalk(low) && sess != nil {
			sess.Phase = phaseClosing
			sess.LastAct = string(actSocial)
		} else if sess != nil {
			sess.LastAct = string(actSocial)
			if sess.Phase == "" {
				sess.Phase = phaseOpening
			}
		}
		return cv, "chat"
	}
	phase := phaseOpening
	turns := 0
	if sess != nil {
		turns = sess.TurnCount
		if sess.Phase != "" {
			phase = sess.Phase
		}
	}

	// cierre
	if isByeTalk(low) {
		if sess != nil {
			sess.Phase = phaseClosing
			sess.LastAct = string(actSocial)
		}
		return "Hasta luego. Aquí sigo cuando quieras retomar.", "chat"
	}
	// gracias
	if isThanksTalk(low) {
		if sess != nil {
			sess.LastAct = string(actSocial)
			if sess.Phase == phaseOpening {
				sess.Phase = phaseOngoing
			}
		}
		return "Con gusto. Si quieres, seguimos con lo que estabas contando o con algo nuevo.", "chat"
	}
	// emoción
	if isEmotionTalk(low) || strings.Contains(low, "cansad") || strings.Contains(low, "día pesado") ||
		strings.Contains(low, "dia pesado") {
		if sess != nil {
			sess.Phase = phaseOngoing
			sess.LastAct = string(actSocial)
		}
		return speakEmotion(low), "chat"
	}
	// confirmaciones cortas
	switch low {
	case "ok", "vale", "claro", "listo", "perfecto", "de acuerdo", "aja", "ajá", "sí", "si":
		if sess != nil {
			sess.LastAct = string(actSocial)
			if sess.Phase == phaseOpening {
				sess.Phase = phaseOngoing
			}
		}
		return "Bien. ¿Seguimos por ahí o abres otra cosa?", "chat"
	}
	// saludo / qué tal
	if sess != nil {
		sess.LastAct = string(actSocial)
		if turns == 0 || phase == phaseOpening {
			sess.Phase = phaseOpening
		} else {
			sess.Phase = phaseOngoing
		}
	}
	if strings.Contains(low, "cualquier") || strings.Contains(low, "lo que sea") ||
		strings.Contains(low, "da igual") || strings.Contains(low, "empecemos") ||
		strings.Contains(low, "vamos a empezar") || strings.Contains(low, "vamos a hablar") ||
		strings.Contains(low, "hablemos") || strings.Contains(low, "charlemos") {
		if sess != nil {
			sess.Phase = phaseOngoing
			sess.LastAct = string(actSocial)
		}
		if strings.Contains(low, "empez") || strings.Contains(low, "hablar") || strings.Contains(low, "charl") {
			return "Perfecto, empezamos. Dime un tema, un hecho tuyo o una pregunta.", "chat"
		}
		return "Vale, tema libre. Cuéntame algo o pregunta lo que quieras.", "chat"
	}
	if strings.Contains(low, "tal") || strings.Contains(low, "estás") || strings.Contains(low, "estas") ||
		strings.Contains(low, "todo bien") || strings.Contains(low, "te va") {
		if phase == phaseOpening || turns == 0 {
			return "Bien, aquí presente. ¿Qué tal tú? Puedes contarme algo tuyo o preguntarme lo que necesites.", "chat"
		}
		return "Sigo bien. ¿Seguimos con lo anterior o cambias de tema?", "chat"
	}
	if strings.HasPrefix(low, "hola") || strings.HasPrefix(low, "hey") || strings.HasPrefix(low, "buenas") ||
		strings.HasPrefix(low, "buenos") || strings.HasPrefix(low, "buen ") {
		if phase == phaseOpening || turns == 0 {
			return "Hola. Estoy aquí contigo. ¿De qué hablamos?", "chat"
		}
		return "Hola de nuevo. ¿En qué seguimos?", "chat"
	}
	return "Te leo. Dime con calma qué quieres contar o preguntar.", "chat"
}

// updateSessionAfterTurn advances phase/turn after a completed tick.
func updateSessionAfterTurn(sess *mindSessionState, text, kind string) {
	if sess == nil {
		return
	}
	sess.TurnCount++
	act := classifySpeechAct(text)
	if act != actNone {
		sess.LastAct = string(act)
	}
	low := strings.ToLower(strings.TrimSpace(text))
	if isByeTalk(low) {
		sess.Phase = phaseClosing
		return
	}
	if sess.Phase == "" || sess.Phase == phaseOpening {
		if act == actSocial && sess.TurnCount <= 1 {
			sess.Phase = phaseOpening
		} else {
			sess.Phase = phaseOngoing
		}
	}
	if sess.Phase == phaseClosing && act != actSocial {
		sess.Phase = phaseOngoing
	}
	_ = kind
}
