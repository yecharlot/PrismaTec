package node

import (
	"fmt"
	"strings"
)

// Diálogo profundo consolidado: una respuesta estructurada cuando hay conocimiento,
// y una salida limpia cuando no (sin ruido de laboratorio).

// deepenKnowledgeAnswer turns a corpus hit into a fuller conversational turn.
func deepenKnowledgeAnswer(userText, know string) string {
	know = strings.TrimSpace(stripKnowledgeEchoLead(know))
	if know == "" {
		return ""
	}
	low := strings.ToLower(strings.TrimSpace(userText))

	// Already long / multi-paragraph — don't bloat
	if len([]rune(know)) > 420 {
		return know
	}

	var extra string
	switch {
	case strings.Contains(low, "por qué") || strings.Contains(low, "porque") || strings.Contains(low, "por que"):
		extra = "En la práctica: conviene enlazarlo con un ejemplo o un límite (qué no implica)."
	case strings.Contains(low, "cómo") || strings.Contains(low, "como "):
		extra = "Si quieres el paso a paso en este nodo, dilo con el verbo de acción (crear, explorar, calcular, generar código)."
	case strings.Contains(low, "diferencia") || strings.Contains(low, "vs") || strings.Contains(low, "versus"):
		extra = "La distinción útil suele ser mecanismo (cómo opera) más que el eslogan."
	case strings.HasPrefix(low, "qué es") || strings.HasPrefix(low, "que es") || strings.HasPrefix(low, "quién es") || strings.HasPrefix(low, "quien es"):
		extra = "Si el término tiene un uso distinto en Alset (red, gen, CID), puedes pedirme el ángulo de la red."
	case strings.Contains(low, "ejemplo"):
		extra = "Puedo aterrizarlo en un caso del nodo (Mind, gen o Lisp) si me dices el contexto."
	default:
		if len([]rune(know)) < 160 {
			extra = "¿Quieres profundidad, un ejemplo o el cruce con tu caso?"
		}
	}

	if extra == "" {
		return know
	}
	// Avoid stacking if knowledge already ends with a question
	if strings.HasSuffix(strings.TrimSpace(know), "?") {
		return know
	}
	return know + "\n\n" + extra
}

// unknownTopicVoice: consolidado cuando no hay corpus ni memoria ni tool.
func unknownTopicVoice(userText string) string {
	low := strings.ToLower(strings.TrimSpace(userText))
	if low == "" {
		return "Te leo. Di un hecho, una pregunta o un pedido al nodo."
	}
	// Prefer teach-me over lab noise
	if strings.HasPrefix(low, "qué es") || strings.HasPrefix(low, "que es") ||
		strings.HasPrefix(low, "quién") || strings.HasPrefix(low, "quien") ||
		strings.Contains(low, "explica") {
		topic := strings.TrimSpace(strings.TrimSuffix(userText, "?"))
		return fmt.Sprintf("No lo tengo sólido en corpus todavía. Puedo lanzar una sonda a la web, o tú me das un hecho claro sobre «%s» y lo anclo en esta sesión.", compressVoiceBlock(topic, 80))
	}
	return "No tengo una respuesta firme a eso aún. Reformula en una pregunta concreta, dame un hecho para recordar, o pide una acción del nodo (gen, cálculo, código)."
}

// enrichDeepTurn: post-process primary knowledge/chat answers for depth without soft-organ spam.
func enrichDeepTurn(kind, userText, voice string) string {
	voice = strings.TrimSpace(voice)
	if voice == "" {
		return voice
	}
	switch kind {
	case "knowledge":
		return deepenKnowledgeAnswer(userText, voice)
	case "chat":
		// If chat is the generic thin filler, replace with unknown path when question-like
		if isThinChatVoice(voice) && looksLikeInfoQuestion(userText) {
			return unknownTopicVoice(userText)
		}
		return voice
	default:
		return voice
	}
}

func isThinChatVoice(v string) bool {
	low := strings.ToLower(v)
	thins := []string{
		"te leo. campo",
		"puedes seguir hablando en natural",
		"campo en seguir",
		"si quieres, bajamos a un detalle",
		"¿seguimos por este tema",
	}
	for _, t := range thins {
		if strings.Contains(low, t) {
			return true
		}
	}
	return len([]rune(v)) < 48 && !strings.Contains(v, "\n")
}

func looksLikeInfoQuestion(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(low, "?") {
		return true
	}
	for _, p := range []string{"qué ", "que ", "quién ", "quien ", "cómo ", "como ", "dónde ", "donde ", "por qué", "explica", "define"} {
		if strings.HasPrefix(low, p) || strings.Contains(low, p) {
			return true
		}
	}
	return false
}
