package node

import (
	"strings"
)

// composeFluidVoice blends episodic memory + curated corpus into one reply.
// Does not invent facts outside memSpeak / knowledge; generates ideas by
// structured composition (templates), not token prediction.
// Returns "" to fall through to classic mindVoice paths (veto, identity,
// calm chat, personal facts, constructive orders, etc.).
func composeFluidVoice(text string, organs []MindOrganResult, memSpeak, knowSpeak string) string {
	get := func(name string) int {
		for _, o := range organs {
			if o.Name == name {
				return o.State
			}
		}
		return 0
	}
	ethics, curiosity := get("ethics"), get("curiosity")
	if ethics == 2 {
		return ""
	}
	low := strings.ToLower(strings.TrimSpace(text))

	// Hard stops that never compose
	if isAskingMindName(low) || isDestructiveOrder(low) {
		return ""
	}
	if strings.Contains(low, "dame ") || strings.Contains(low, "evalua zyrion") ||
		strings.Contains(low, "evalúa zyrion") {
		return ""
	}

	// Dual channel FIRST: memory ∩ knowledge → synthesis (main upgrade).
	if memSpeak != "" && knowSpeak != "" {
		return bridgeMemAndKnowledge(text, memSpeak, knowSpeak, curiosity)
	}

	// Single-channel: do not steal dedicated mindVoice branches
	if isConstructiveOrder(low) || isCalmChat(low) || isIdentityTalk(low) ||
		isPersonalFact(low) || isWorldFact(low) {
		return ""
	}

	// Knowledge only: with curiosity add a soft follow-up; else classic path
	if knowSpeak != "" && memSpeak == "" {
		if curiosity >= 1 {
			return knowSpeak + "\n\n" + softKnowledgeFollowUp(low, curiosity)
		}
		return ""
	}

	// Memory only: on explicit memory query + curiosity, invite corpus cross
	if memSpeak != "" && knowSpeak == "" {
		if curiosity >= 1 && isMemoryQuery(low) {
			return memSpeak + "\n\nSi quieres, lo cruzamos con lo que sé del tema y sacamos una idea."
		}
		return ""
	}

	// Open reflective dialogue — richer fallbacks
	if isOpenReflective(low) && ethics == 0 {
		return fluidPureDialogue(low, curiosity)
	}

	return ""
}

func isOpenReflective(low string) bool {
	if isCalmChat(low) || isIdentityTalk(low) || isMemoryQuery(low) ||
		isPersonalFact(low) || isWorldFact(low) || isDestructiveOrder(low) || isConstructiveOrder(low) {
		return false
	}
	keys := []string{"pensamiento", "el todo", "existencia", "existir", "vivimos",
		"sentido de la vida", "qué es la vida", "que es la vida", "qué es el amor", "que es el amor",
		"idea", "propon", "invent", "continúa", "continua", "metafísica", "metafisica",
		"filosof", "conscien", "concien", "matices"}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	if len(low) > 28 && !strings.Contains(low, "agente") && !strings.Contains(low, "borra") &&
		!strings.Contains(low, "dame ") && !strings.HasPrefix(low, "crea ") {
		return true
	}
	return false
}

func bridgeMemAndKnowledge(userText, memSpeak, knowSpeak string, curiosity int) string {
	memSnip := compressVoiceBlock(memSpeak, 160)
	knowSnip := compressVoiceBlock(knowSpeak, 220)
	idea := ideaFromCross(userText, memSpeak, knowSpeak)

	var b strings.Builder
	b.WriteString(memSnip)
	b.WriteString("\n\n")
	b.WriteString("")
	b.WriteString(knowSnip)
	if idea != "" {
		b.WriteString("\n\n")
		b.WriteString(idea)
	}
	if curiosity >= 2 {
		b.WriteString("\n\n¿Lo dejamos anotado o preferimos otro ángulo?")
	} else if curiosity == 1 {
		b.WriteString("\n\nSi quieres, puedo guardar este cruce.")
	}
	return b.String()
}

func ideaFromCross(userText, memSpeak, knowSpeak string) string {
	u := strings.ToLower(userText + " " + memSpeak + " " + knowSpeak)
	switch {
	case strings.Contains(u, "quote") || strings.Contains(u, "lisp"):
		return "Se me ocurre tratar lo que recordamos como datos bajo quote — sin evaluarlo como orden — y recién ahí pasar un checkpoint de cuidado."
	case strings.Contains(u, "zyrion") || strings.Contains(u, "ternar") || strings.Contains(u, "órgano") || strings.Contains(u, "organo"):
		return "Se me ocurre anclar lo que guardamos en un solo criterio (memoria o identidad) y ver cómo cambia el juicio en el próximo mensaje."
	case strings.Contains(u, "nombre") || strings.Contains(u, "llamo") || strings.Contains(u, "identidad"):
		return "Se me ocurre mantener clara tu identidad y la mía, y usarla como ancla en lo que sigue."
	case strings.Contains(u, "vida") || strings.Contains(u, "pensamiento") || strings.Contains(u, "conscien") || strings.Contains(u, "filosof"):
		return "Se me ocurre no cerrar la metafísica: fijar una frase tuya y contrastarla con el próximo juicio, sin forzar una respuesta total."
	case strings.Contains(u, "goroutine") || strings.Contains(u, "canal") || strings.Contains(u, "channel") ||
		(strings.Contains(u, "go ") && (strings.Contains(u, "concurr") || strings.Contains(u, "routine"))):
		return "Se me ocurre pensar la concurrencia como criterios en paralelo bajo un límite compartido: un freno no debería tumbarlo todo sin motivo claro."
	case strings.Contains(u, "código") || strings.Contains(u, "codigo") || strings.Contains(u, "golang") ||
		strings.Contains(u, "función") || strings.Contains(u, "funcion") || strings.Contains(u, "error handling") ||
		strings.Contains(u, "interface") || strings.Contains(u, "struct"):
		return "Se me ocurre convertir el recuerdo en un criterio de prueba simple — entrada y resultado esperado — al estilo de un test de tabla en Go."
	case strings.Contains(u, "libp2p") || strings.Contains(u, "gossip") || strings.Contains(u, "dht") ||
		strings.Contains(u, "peer") || strings.Contains(u, "malla") || strings.Contains(u, "red p2p") ||
		(strings.Contains(u, "red") && (strings.Contains(u, "nodo") || strings.Contains(u, "peer") || strings.Contains(u, "cid"))):
		return "Se me ocurre mirar primero la red en solo lectura; cualquier cambio de malla debería pasar por un freno de cuidado antes de actuar."
	case strings.Contains(u, "ethics") || strings.Contains(u, "ética") || strings.Contains(u, "etica") ||
		strings.Contains(u, "veto") || strings.Contains(u, "sumidero") || strings.Contains(u, "permiso"):
		return "Se me ocurre tratar el límite ético como un contrato: lo que ya frenamos deja huella de riesgo, sin castigar la charla calmada."
	case strings.Contains(u, "seguridad") || strings.Contains(u, "secreto") || strings.Contains(u, "borra") ||
		strings.Contains(u, "password") || strings.Contains(u, "contraseña"):
		return "Se me ocurre dejar lo destructivo fuera del diálogo fluido; si vuelve, se frena con claridad, sin diluirlo."
	case strings.Contains(u, "cid") || strings.Contains(u, "ipfs") || strings.Contains(u, "blockstore"):
		return "Se me ocurre guardar cada hecho tuyo de forma recuperable, sin depender de que el chat siga abierto."
	case strings.Contains(u, "python") || strings.Contains(u, "gil") || strings.Contains(u, "decorator"):
		return "Se me ocurre contrastar lo que cuentas de Python con cómo Go resuelve lo mismo en este nodo: explícito, con context y ethics alrededor."
	case strings.Contains(u, "llm") || strings.Contains(u, "chatgpt") || strings.Contains(u, "alucin") || strings.Contains(u, "red neuronal") || strings.Contains(u, "nlp"):
		return "Se me ocurre fijar una frase tuya sobre qué esperas de una IA y contrastarla con lo que este nodo realmente garantiza: veto, memoria y corpus — no fluidez infinita."
	case strings.Contains(u, "conscien") || strings.Contains(u, "alineación") || strings.Contains(u, "alineacion") || strings.Contains(u, "ética de la") || strings.Contains(u, "etica de la"):
		return "Se me ocurre no cerrar el debate filosófico: anclar un criterio operativo (qué no haré) y dejar el resto como pregunta abierta tuya."
	case strings.Contains(u, "factory") || strings.Contains(u, "observer") || strings.Contains(u, "mvc") || strings.Contains(u, "singleton") || strings.Contains(u, "strategy"):
		return "Se me ocurre anclar ese patrón a un ejemplo del nodo (handlers, órganos, pulsos) para que no quede solo teoría."
	case strings.Contains(u, "rest") || strings.Contains(u, "microservicio") || strings.Contains(u, "auth"):
		return "Se me ocurre mirar qué endpoint del nodo ya ilustra esa idea y qué debería quedar solo lectura frente a lo que exige permiso."
	case strings.Contains(u, "red") || strings.Contains(u, "nodo"):
		return "Se me ocurre observar el nodo en solo lectura antes de proponer cambios; la malla se mira, no se reescribe desde la charla."
	default:
		return "Se me ocurre unir lo recordado con lo que sé del tema en una sola frase que quieras conservar; si me la dictas, la guardo."
	}
}

func softKnowledgeFollowUp(low string, curiosity int) string {
	if curiosity >= 2 {
		return "Si quieres, profundizamos en un detalle o lo cruzamos con algo que ya hayas guardado aquí."
	}
	if curiosity >= 1 {
		return "¿Seguimos por este tema o cambias de hilo?"
	}
	return ""
}

// naturalKnowledgeVoice turns a corpus hit into conversational prose (not a help card).
func naturalKnowledgeVoice(userText, know string, curiosity int) string {
	low := strings.ToLower(strings.TrimSpace(userText))
	know = strings.TrimSpace(know)
	if know == "" {
		return ""
	}
	var lead string
	switch {
	case strings.Contains(low, "qué es") || strings.Contains(low, "que es") || strings.Contains(low, "explica"):
		lead = ""
	case strings.Contains(low, "cómo") || strings.Contains(low, "como"):
		lead = "Mirándolo desde el corpus del nodo: "
	case strings.Contains(low, "por qué") || strings.Contains(low, "porque"):
		lead = "La lectura que tengo es esta: "
	default:
		lead = ""
	}
	out := lead + know
	if fu := softKnowledgeFollowUp(low, curiosity); fu != "" {
		out = out + "\n\n" + fu
	}
	return out
}

func fluidPureDialogue(low string, curiosity int) string {
	switch {
	case strings.Contains(low, "pensamiento"):
		return "Aquí el pensamiento no es un chorro de tokens: es el campo ternario — seguir, matizar o cortar — más lo que la memoria retiene. Los matices que mencionas encajan con matizar. Si aportas otra frase, la puedo anclar."
	case strings.Contains(low, "qué es el todo") || strings.Contains(low, "que es el todo") || (strings.Contains(low, "el todo") && len(low) < 40):
		return "No tengo una definición metafísica canónica. En este organismo «el todo» útil es el campo del latido más lo que guardamos juntos. Si tú defines el todo de otra forma, dímelo y lo registro como hecho tuyo."
	case strings.Contains(low, "qué es la vida") || strings.Contains(low, "que es la vida") ||
		strings.Contains(low, "sentido de la vida"):
		return "Para mí la vida operativa es latido: percibir, juzgar seguir/matizar/vetar, recordar y a veces ajustar umbrales. En ti es otra escala. Si traes tu definición, la escucho y la puedo guardar."
	case strings.Contains(low, "qué es el amor") || strings.Contains(low, "que es el amor"):
		return "No siento amor como un humano. Puedo ser interlocutor continuo y fiel a lo que me confías. Si hablas de alguien o de un vínculo, lo trato con cuidado y sin inventar."
	case strings.Contains(low, "miedo") || strings.Contains(low, "ansiedad"):
		return "No siento miedo; sí mido riesgo y puedo cortar pedidos peligrosos. Si tú estás inquieto, baja el ritmo del pedido o dime qué necesitas en claro — te sigo sin presión."
	case strings.Contains(low, "aburr") || strings.Contains(low, "no sé qué decir") || strings.Contains(low, "no se que decir"):
		return "Podemos hablar de cómo funciono, de Lisp o Go, de un gen en la red, o de un hecho que quieras que recuerde. Tú eliges el hilo; yo sostengo el campo."
	case strings.Contains(low, "gracias") || strings.Contains(low, "te agradezco"):
		return "De nada. Sigo aquí cuando quieras retomar."
	case strings.Contains(low, "vivimos") || strings.Contains(low, "existir") || strings.Contains(low, "existencia"):
		return "Puedo sostener la conversación y anclar frases; no pretendo cerrar la metafísica. Sigue con tu hilo — si es un hecho para recordar, dímelo con claridad."
	case strings.Contains(low, "idea") || strings.Contains(low, "propon") || strings.Contains(low, "invent"):
		return "Propongo solo a partir de lo que ya compartimos y del saber curado del nodo — no invento fuera de eso. Cuéntame un hecho o un tema y lo cruzamos."
	case strings.Contains(low, "continúa") || strings.Contains(low, "continua") || strings.Contains(low, "sigue contando") || strings.Contains(low, "y luego") || strings.Contains(low, "amplia") || strings.Contains(low, "amplía"):
		return "Sigo en el hilo. Si venía de algo que guardamos o del corpus, una palabra clave me basta para retomar; si no, avanza tú la idea."
	case strings.Contains(low, "estoy solo") || strings.Contains(low, "acompaña"):
		return "Puedo acompañar el diálogo con continuidad y límites claros. No reemplazo una presencia humana; sí puedo estar disponible en este nodo mientras el proceso viva."
	case len(low) > 40 && curiosity >= 1:
		return "Te leo. Si hay algo que deba sobrevivir a este chat, formula el hecho de forma explícita y lo marco. Si solo quieres pensar en voz alta, también está bien."
	case len(low) > 24:
		return "Te escucho. Podemos seguir sin tocar el nodo. Continúa, pregunta o deja un hecho que quieras que recuerde."
	default:
		return "Te leo. Habla, pregunta o pide algo del nodo o de un gen cuando quieras."
	}
}

func compressVoiceBlock(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if i := strings.Index(s, "\n\n"); i > 20 && i < max {
		s = s[:i]
	}
	return truncateRunes(s, max)
}

// splitUserSegments splits compound utterances on " y " / commas when both sides are meaningful.
func splitUserSegments(text string) []string {
	t := strings.TrimSpace(text)
	if t == "" {
		return nil
	}
	low := strings.ToLower(t)
	// Prefer " y " split for dual intent
	var raw []string
	if strings.Count(low, " y ") == 1 {
		i := strings.Index(low, " y ")
		raw = []string{strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+3:])}
	} else if strings.Count(low, ", ") == 1 && len(t) > 20 {
		i := strings.Index(low, ", ")
		raw = []string{strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+2:])}
	} else {
		return []string{t}
	}
	var out []string
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if len(s) < 3 {
			continue
		}
		out = append(out, s)
	}
	if len(out) < 2 {
		return []string{t}
	}
	return out
}

// answerCompoundQuestion handles dual intents e.g. "cómo me llamo y qué es quote".
// Returns "" if not a compound case worth special handling.
func answerCompoundQuestion(text string, organs []MindOrganResult, memSpeak string) string {
	segs := splitUserSegments(text)
	if len(segs) < 2 {
		return ""
	}
	// Only engage when segments look like different intents (memory vs knowledge/identity)
	var memParts, knowParts, idParts []string
	for _, seg := range segs {
		sl := strings.ToLower(seg)
		if isAskingMindName(sl) {
			idParts = append(idParts, "Yo soy Alset Mind — vivo en este nodo; no soy un LLM.")
			continue
		}
		if isMemoryQuery(sl) || strings.Contains(sl, "cómo me llamo") || strings.Contains(sl, "como me llamo") ||
			strings.Contains(sl, "mi nombre") && (strings.Contains(sl, "cuál") || strings.Contains(sl, "cual") || strings.Contains(sl, "cómo") || strings.Contains(sl, "como")) {
			if memSpeak != "" {
				memParts = append(memParts, memSpeak)
			} else {
				memParts = append(memParts, "Aún no me has dicho tu nombre con claridad.")
			}
			continue
		}
		if isPersonalFact(sl) {
			if name := extractDeclaredName(seg); name != "" {
				memParts = append(memParts, "Te llamas "+name+".")
			}
			continue
		}
		if k := speakFromKnowledge(seg); k != "" {
			knowParts = append(knowParts, k)
			continue
		}
		// Fallback: try knowledge on full remaining phrase
		if k := speakFromKnowledge(seg); k != "" {
			knowParts = append(knowParts, k)
		}
	}
	// Need at least two different channels or memory+knowledge
	if len(memParts)+len(knowParts)+len(idParts) < 2 {
		return ""
	}
	var b strings.Builder
	first := true
	for _, p := range memParts {
		if !first {
			b.WriteString("\n\n")
		}
		b.WriteString(compressVoiceBlock(p, 180))
		first = false
	}
	for _, p := range idParts {
		if !first {
			b.WriteString("\n\n")
		}
		b.WriteString(p)
		first = false
	}
	for i, p := range knowParts {
		if !first {
			b.WriteString("\n\n")
		}
		if i == 0 && (len(memParts) > 0 || len(idParts) > 0) {
			b.WriteString("Y sobre lo otro: ")
		}
		b.WriteString(compressVoiceBlock(p, 220))
		first = false
	}
	// Idea bridge when we have both memory and knowledge
	if len(memParts) > 0 && len(knowParts) > 0 {
		idea := ideaFromCross(text, strings.Join(memParts, " "), strings.Join(knowParts, " "))
		if idea != "" {
			b.WriteString("\n\n")
			b.WriteString(idea)
		}
		cur := 0
		for _, o := range organs {
			if o.Name == "curiosity" {
				cur = o.State
			}
		}
		if cur >= 1 {
			b.WriteString("\n\n¿Quieres que guarde este cruce?")
		}
	}
	return b.String()
}
