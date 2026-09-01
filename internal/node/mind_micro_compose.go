package node

import (
	"strings"
)

// Micro-compositor de charla (P1): moldes deterministas + eco del usuario.
// No es un LLM: ensambla piezas según acto, fase y ancla de memoria.

func microComposeChat(text string, sess *mindSessionState, profile UserProfile, recent []mindEpisodePayload) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return ""
	}
	if isMemoryQuery(low) || isSelfModelQuery(low) || isIdentityTalk(low) || isCapabilityQuestion(low) {
		return ""
	}
	if isReasoningRequest(low) || isRFTRequest(low) {
		return ""
	}
	if strings.Contains(low, "chatgpt") || strings.Contains(low, "gpt") || strings.Contains(low, "llm") {
		return ""
	}
	if strings.Contains(low, "zyrion") || strings.Contains(low, "genoma") || strings.Contains(low, "alset mind") {
		return ""
	}
	act := classifySpeechAct(text)
	if act == actTask || act == actMeta {
		return ""
	}
	if cv := speakFromDialogActs(text, sess); cv != "" && act != actSocial {
		return cv
	}
	phase := phaseOpening
	turns := 0
	lastQ := ""
	if sess != nil {
		phase = sess.Phase
		if phase == "" {
			phase = phaseOpening
		}
		turns = sess.TurnCount
		lastQ = sess.LastQuery
	}
	name := ""
	if profile.Nombre != "" {
		name = profile.Nombre
	}

	// Social ya tiene speakSpeechAct; aquí reforzamos content liviano y continuidad
	if act == actSocial {
		return "" // leave to speech-act
	}

	// Continuidad: "y tú", "cuéntame más", "en serio", eco
	if isContinuePrompt(low) || isElaborationRequest(low) || strings.HasPrefix(low, "y ") ||
		low == "sigue" || low == "continúa" || low == "continua" {
		if lastQ != "" {
			return "Seguimos con eso. ¿Qué parte quieres abrir: un detalle, un ejemplo o el cruce con algo tuyo?"
		}
		return "Dime el hilo: un hecho, una pregunta o un pedido al nodo."
	}

	// Cambio de tema suave
	if isTopicShift(low) {
		return "De acuerdo, cambiamos de tema. ¿Hacia dónde?"
	}

	// Content liviano: pregunta abierta corta sin corpus
	if looksLikeInfoQuestion(low) && len(strings.Fields(low)) <= 8 {
		// no interceptar dominios que knowledge debe cubrir
		if strings.Contains(low, "zyrion") || strings.Contains(low, "alset") || strings.Contains(low, "genoma") ||
			strings.Contains(low, "cid") || strings.Contains(low, "órgano") || strings.Contains(low, "organo") {
			return ""
		}
	}

	// Ancla de nombre solo en charla afirmativa (nunca preguntas)
	if phase == phaseOngoing && turns >= 1 && name != "" && isPureDialogue(low) &&
		!strings.Contains(low, "?") && !looksLikeInfoQuestion(low) {
		return "Te sigo, " + name + ". Cuéntame más o pregunta con calma; si es un hecho que quieras anclar, dilo claro."
	}

	// Eco mínimo: si declara algo corto y no es personal fact ya manejado
	if isPersonalFact(low) {
		return ""
	}

	// Content genérico corto (opinión / filosofía liviana)
	if isPureDialogue(low) && len(low) > 20 {
		echo := compressVoiceBlock(strings.TrimSpace(text), 60)
		if echo != "" {
			return "Te escucho en eso de «" + echo + "». Puedo sostener el hilo, anclar un hecho o mirar el nodo si hace falta."
		}
	}
	return ""
}

// softSocialBridge: si el content path falló y el mensaje es social-adyacente, no menú.
func softSocialBridge(text, currentVoice string) string {
	if currentVoice != "" {
		return currentVoice
	}
	low := strings.ToLower(strings.TrimSpace(text))
	if isSpeechSocial(low) || isNoiseOrGreeting(low) {
		return "Hola. Estoy aquí. ¿Qué quieres contar o preguntar?"
	}
	return ""
}
