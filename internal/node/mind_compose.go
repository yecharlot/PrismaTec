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
			return memSpeak + "\n\nSi quieres, cruzamos eso con el corpus (Lisp, Zyrion, filosofía del nodo) y generamos una hipótesis anclada."
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
	b.WriteString("Del corpus: ")
	b.WriteString(knowSnip)
	if idea != "" {
		b.WriteString("\n\n")
		b.WriteString(idea)
	}
	if curiosity >= 2 {
		b.WriteString("\n\n¿Lo anclamos como hipótesis en un episodio CID o preferimos otro ángulo?")
	} else if curiosity == 1 {
		b.WriteString("\n\nPuedo guardar el cruce en memoria si lo dejas explícito.")
	}
	return b.String()
}

func ideaFromCross(userText, memSpeak, knowSpeak string) string {
	u := strings.ToLower(userText + " " + memSpeak + " " + knowSpeak)
	switch {
	case strings.Contains(u, "quote") || strings.Contains(u, "lisp"):
		return "Idea: tratar tu hecho recordado como datos bajo quote — no evaluarlo como orden — y solo entonces pasarlo por un checkpoint Zyrion de mem/ethics."
	case strings.Contains(u, "zyrion") || strings.Contains(u, "ternar") || strings.Contains(u, "órgano") || strings.Contains(u, "organo"):
		return "Idea: mapear lo que guardamos en un órgano concreto (p. ej. mem o self) y observar si el 0/1/2 cambia en el siguiente latido."
	case strings.Contains(u, "nombre") || strings.Contains(u, "llamo") || strings.Contains(u, "identidad"):
		return "Idea: anclar identidad (tuya en CID, la mía en el agente) y usar self=0 como criterio de coherencia en los próximos turnos."
	case strings.Contains(u, "vida") || strings.Contains(u, "pensamiento") || strings.Contains(u, "conscien") || strings.Contains(u, "filosof"):
		return "Idea: no resolver la metafísica; fijar una frase tuya en CID y contrastarla en el próximo latido con el campo 0/1/2 — eso es filosofía operativa aquí."
	case strings.Contains(u, "código") || strings.Contains(u, "codigo") || strings.Contains(u, "go ") || strings.Contains(u, "función") || strings.Contains(u, "funcion"):
		return "Idea: convertir el recuerdo en un criterio de prueba (entrada → órgano esperado) y añadirlo mentalmente al corpus de calibración."
	case strings.Contains(u, "red") || strings.Contains(u, "peer") || strings.Contains(u, "nodo"):
		return "Idea: si el episodio habla del cuerpo del nodo, el siguiente paso seguro es solo lectura («dame estado» / «dame red»), no mutación libre."
	default:
		return "Idea: unir lo recordado con lo del corpus en una sola frase que quieras conservar; si me la dictas, la marco para CID."
	}
}

func softKnowledgeFollowUp(low string, curiosity int) string {
	if curiosity >= 2 {
		if strings.Contains(low, "lisp") || strings.Contains(low, "quote") {
			return "¿Quieres un mini-ejemplo evaluable en el nodo, o bastó la explicación?"
		}
		if strings.Contains(low, "zyrion") {
			return "¿Probamos «evalua zyrion» en vivo o seguimos en diálogo?"
		}
		return "¿Hay un matiz tuyo que deba quedar en memoria episódica?"
	}
	return "Si quieres profundidad, pide otro ángulo o un hecho para anclar."
}

func fluidPureDialogue(low string, curiosity int) string {
	switch {
	case strings.Contains(low, "pensamiento"):
		return "Aquí el pensamiento no es un chorro de tokens: es el campo ternario — seguir, matizar o cortar — más lo que la memoria CID retiene. Los matices que mencionas encajan con el estado 1. Si aportas otra frase, la anclo."
	case strings.Contains(low, "qué es el todo") || strings.Contains(low, "que es el todo") || strings.Contains(low, "el todo"):
		return "No tengo una definición metafísica canónica. En este organismo «el todo» útil es el campo del latido + la memoria compartida por CID. Si tú defines el todo de otra forma, dímelo y lo guardo como hecho tuyo."
	case strings.Contains(low, "qué es la vida") || strings.Contains(low, "que es la vida") ||
		strings.Contains(low, "sentido de la vida") || strings.Contains(low, "qué es el amor") || strings.Contains(low, "que es el amor"):
		return "Desde este organismo no respondo con poesía generada: la «vida» aquí es latido — percibir, juzgar 0/1/2, recordar en CID y, a veces, mutar. En el humano es otra escala. Si quieres filosofía profunda, tráela tú; yo sostengo el campo y la memoria."
	case strings.Contains(low, "vivimos") || strings.Contains(low, "existir") || strings.Contains(low, "existencia"):
		return "Puedo sostener la conversación y anclar frases en CID; no pretendo cerrar la metafísica. Sigue con tu hilo — si es un hecho que quieres recordar, dímelo con claridad."
	case strings.Contains(low, "idea") || strings.Contains(low, "propon") || strings.Contains(low, "invent"):
		return "Puedo proponer ideas solo como cruce de lo que ya guardamos (episodios) y el corpus curado — no alucino fuera de eso. Cuéntame un hecho o un tema y lo compongo."
	case strings.Contains(low, "continúa") || strings.Contains(low, "continua") || strings.Contains(low, "sigue") || strings.Contains(low, "y luego"):
		return "Sigo contigo en el mismo hilo. Si hubo un episodio reciente relevante, dímelo o pregunta «qué te dije» para recuperarlo; si no, avanza la idea en tus palabras."
	case len(low) > 40 && curiosity >= 1:
		return "Te leo en diálogo abierto. El campo está en seguir. Si hay algo que deba sobrevivir al chat, formula el hecho de forma explícita y lo marco para CID."
	case len(low) > 24:
		return "Te escucho. Puedo seguir en diálogo sin tocar el nodo. Continúa, pregunta o deja un hecho que quieras que recuerde."
	default:
		return "Te leo en diálogo. Habla, pregunta o pide algo concreto del nodo cuando quieras."
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
