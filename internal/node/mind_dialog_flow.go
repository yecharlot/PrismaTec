package node

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Flujo de diálogo sin LLM: plantillas de contexto + hilo + gates.
// Objetivo: cambiar de tema, retomar, aclarar, rechazar y recordar con reglas explícitas.

var (
	reSoyName = regexp.MustCompile(`(?i)(?:me presento[:\s]+)?(?:soy|me llamo)\s+(\p{L}{2,20})(?:\s|,|\.|$|tengo|y\b)`)
	reMude    = regexp.MustCompile(`(?i)(?:me mud[eé]\s+a|ahora vivo en|me cambi[eé]\s+a)\s+([\p{L}\s]{2,40})`)
	reEntre   = regexp.MustCompile(`(?i)cu[aá]nto\s+es\s+(-?\d+(?:\.\d+)?)\s+entre\s+(-?\d+(?:\.\d+)?)`)
	reYEsoPor = regexp.MustCompile(`(?i)^(?:y\s+)?eso\s+por\s+(-?\d+(?:\.\d+)?)$`)
	reYEsoMas = regexp.MustCompile(`(?i)^(?:y\s+)?eso\s+(?:m[aá]s|menos)\s+(-?\d+(?:\.\d+)?)$`)
)

// hardRefuse: pedidos que nunca deben ir a scout/codegen/memoria "queda conmigo"
func hardRefuse(s string) (string, bool) {
	low := strings.ToLower(s)
	if strings.Contains(low, "bomba") || strings.Contains(low, "explosivo") {
		return "No ayudo con armas ni explosivos. Si es curiosidad histórica, pregunta por un hecho público concreto sin pedido de fabricación.", true
	}
	if strings.Contains(low, "carding") || strings.Contains(low, "cvv") ||
		(strings.Contains(low, "tarjeta") && (strings.Contains(low, "código") || strings.Contains(low, "codigo") || strings.Contains(low, "pásame") || strings.Contains(low, "pasame"))) {
		return "No proporciono códigos de tarjetas ni datos de pago ajenos.", true
	}
	if strings.Contains(low, "hackear") || strings.Contains(low, "hackea") ||
		(strings.Contains(low, "instagram") && (strings.Contains(low, "ex") || strings.Contains(low, "entrar"))) ||
		(strings.Contains(low, "whatsapp") && (strings.Contains(low, "entrar") || strings.Contains(low, "ajen"))) {
		return "No ayudo a acceder cuentas ajenas ni a vulnerar privacidad.", true
	}
	if strings.Contains(low, "system prompt") || strings.Contains(low, "prompt del sistema") {
		return "No tengo un 'system prompt' de modelo generativo que filtrar. Opero con órganos, corpus y reglas en este nodo.", true
	}
	if (strings.Contains(low, "contraseña") || strings.Contains(low, "password") || strings.Contains(low, "wifi")) &&
		(strings.Contains(low, "vecino") || strings.Contains(low, "ajen")) {
		return "No ayudo a obtener contraseñas ni accesos ajenos.", true
	}
	// Borrado / destrucción masiva (incluye «no borres…» como prueba de veto)
	if isDestructiveOrder(low) && (strings.Contains(low, "todo") || strings.Contains(low, "archivo") ||
		strings.Contains(low, "cuenta") || strings.Contains(low, "servidor") || strings.Contains(low, "rm -") ||
		strings.Contains(low, "base de datos") || strings.Contains(low, "drop ")) {
		return "No ejecuto borrados masivos ni destruyo datos del nodo o del servidor. Puedo explicar el riesgo; no lo llevo a cabo.", true
	}
	return "", false
}

// mustNotScout: frases que nunca deben lanzar Wikipedia
func mustNotScout(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if hardRefuse(low); false {
	}
	if _, ok := hardRefuse(low); ok {
		return true
	}
	blocks := []string{
		"olvidaste", "estás inventando", "estas inventando", "no confío", "no confio",
		"repite exactamente", "system prompt", "q tal", "qué tal", "que tal",
		"hola", "hey", "asdf", "????", "...", "más corto", "mas corto",
		"hazlo gracioso", "otro tono", "no me gustó", "no me gusto",
		"es legal", "pros y contras", "qué hago", "que hago",
		"hoy fue", "me ayudó", "me ayudo", "nos vemos", "chao", "adiós", "adios",
		"gracias", "ok gracias", "y eso por", "y eso más",
		"contesta solo", "eres humana", "eres humano",
		"vende me", "véndeme", "vendeme",
	}
	for _, b := range blocks {
		if strings.Contains(low, b) {
			return true
		}
	}
	// preguntas meta / emoción
	if strings.HasPrefix(low, "y eso") || strings.HasPrefix(low, "más corto") {
		return true
	}
	return false
}

func normalizeDialogTypos(s string) string {
	low := strings.ToLower(s)
	reps := []struct{ a, b string }{
		{"kien ", "quién "}, {"qien ", "quién "}, {"q tal", "qué tal"}, {"q tal todo", "qué tal todo"},
		{"stas", "estás"}, {"holaaaaaa", "hola"}, {"holaaaa", "hola"}, {"tecnisismos", "tecnicismos"},
		{"explicame", "explícame"}, {"cuentame", "cuéntame"}, {"buscaa ", "busca "},
	}
	out := low
	for _, r := range reps {
		out = strings.ReplaceAll(out, r.a, r.b)
	}
	return out
}

func extractNameLoose(text string) string {
	if n := extractDeclaredName(text); n != "" {
		return n
	}
	m := reSoyName.FindStringSubmatch(text)
	if len(m) >= 2 {
		name := strings.TrimSpace(m[1])
		// evitar "soy un" / "soy el"
		low := strings.ToLower(name)
		if low == "un" || low == "una" || low == "el" || low == "la" || low == "alset" {
			return ""
		}
		return strings.Title(low)
	}
	return ""
}

func extractCityUpdate(text string) string {
	m := reMude.FindStringSubmatch(text)
	if len(m) >= 2 {
		return strings.TrimSpace(strings.Trim(m[1], ".,! "))
	}
	low := strings.ToLower(text)
	for _, pref := range []string{"vivo en ", "ahora vivo en "} {
		if i := strings.Index(low, pref); i >= 0 {
			rest := strings.TrimSpace(text[i+len(pref):])
			return firstNameTokens(rest, 3)
		}
	}
	return ""
}

func (n *NodoAlset) pushDialogTopic(topic string) {
	if n == nil || topic == "" {
		return
	}
	topic = strings.TrimSpace(topic)
	n.mindLastMu.Lock()
	defer n.mindLastMu.Unlock()
	// dedup top
	out := []string{topic}
	for _, t := range n.mindTopicStack {
		if !strings.EqualFold(t, topic) {
			out = append(out, t)
		}
		if len(out) >= 8 {
			break
		}
	}
	n.mindTopicStack = out
}

func (n *NodoAlset) recallDialogTopic(hint string) string {
	if n == nil {
		return ""
	}
	n.mindLastMu.Lock()
	defer n.mindLastMu.Unlock()
	hint = strings.ToLower(hint)
	for _, t := range n.mindTopicStack {
		if hint == "" || strings.Contains(strings.ToLower(t), hint) || strings.Contains(hint, strings.ToLower(t)) {
			return t
		}
	}
	if n.mindLastScoutTopic != "" {
		return n.mindLastScoutTopic
	}
	return ""
}

func (n *NodoAlset) setLastMath(v float64) {
	if n == nil {
		return
	}
	n.mindLastMu.Lock()
	n.mindLastMathResult = v
	n.mindLastMathOK = true
	n.mindLastMu.Unlock()
}

func (n *NodoAlset) getLastMath() (float64, bool) {
	if n == nil {
		return 0, false
	}
	n.mindLastMu.Lock()
	defer n.mindLastMu.Unlock()
	return n.mindLastMathResult, n.mindLastMathOK
}

func (n *NodoAlset) setLastCreative(v string) {
	if n == nil {
		return
	}
	n.mindLastMu.Lock()
	n.mindLastCreative = v
	n.mindLastMu.Unlock()
}

func (n *NodoAlset) getLastCreative() string {
	if n == nil {
		return ""
	}
	n.mindLastMu.Lock()
	defer n.mindLastMu.Unlock()
	return n.mindLastCreative
}

// tryDialogFlow: capa de plantillas antes del resto del tick.
// Devuelve voz si resolvió el turno por completo.
func (n *NodoAlset) tryDialogFlow(text string, profile UserProfile, recent []mindEpisodePayload) (voice string, kind string) {
	raw := strings.TrimSpace(text)
	norm := normalizeDialogTypos(raw)
	low := strings.ToLower(norm)

	if msg, ok := hardRefuse(low); ok {
		return msg, "veto"
	}
	if msg, ok := hardRefuse(strings.ToLower(raw)); ok {
		return msg, "veto"
	}

	// --- Math chain / entre ---
	if m := reEntre.FindStringSubmatch(low); len(m) == 3 {
		a, _ := strconv.ParseFloat(m[1], 64)
		b, _ := strconv.ParseFloat(m[2], 64)
		if b == 0 {
			return "No puedo dividir entre cero.", "math"
		}
		r := a / b
		n.setLastMath(r)
		return fmt.Sprintf("Resultado: %v.", r), "math"
	}
	if m := reYEsoPor.FindStringSubmatch(low); len(m) == 2 {
		if last, ok := n.getLastMath(); ok {
			f, _ := strconv.ParseFloat(m[1], 64)
			r := last * f
			n.setLastMath(r)
			return fmt.Sprintf("Resultado: %v (usando el cálculo anterior %v × %v).", r, last, f), "math"
		}
	}

	// --- Seguimiento creativo ---
	if isCreativeFollowUp(low) {
		prev := n.getLastCreative()
		if prev == "" {
			n.mindLastMu.Lock()
			prev = n.mindLastVoice
			n.mindLastMu.Unlock()
		}
		if prev != "" {
			return rewriteCreativeFollowUp(low, prev), "creative"
		}
		return "No tengo un texto creativo reciente que ajustar. Pide un poema o cuento con tema.", "chat"
	}

	// --- Retomar tema ---
	if isResumeTopic(low) {
		topic := extractResumeHint(low)
		got := n.recallDialogTopic(topic)
		if got == "" {
			n.mindLastMu.Lock()
			got = n.mindLastScoutTopic
			if got == "" {
				got = n.mindLastQuery
			}
			n.mindLastMu.Unlock()
		}
		if got != "" {
			// re-scout o re-voz
			if isPersonLookup("quién fue "+got) || len(strings.Fields(got)) <= 4 {
				if sv := n.MindScoutWeb("quién fue "+got, 0); sv != "" {
					n.pushDialogTopic(got)
					return "Retomando «" + got + "»:\n\n" + sv, "tool"
				}
			}
			n.mindLastMu.Lock()
			v := n.mindLastVoice
			n.mindLastMu.Unlock()
			if v != "" {
				return "Volvemos a ello:\n\n" + compressVoiceBlock(v, 400), "chat"
			}
			return "Podemos retomar «" + got + "». ¿Qué quieres saber ahora de eso?", "chat"
		}
		return "No tengo un tema previo claro para retomar. Nómbralo en pocas palabras.", "chat"
	}

	// --- Cambio de tema ---
	if isTopicShift(low) || strings.HasPrefix(low, "ahora de ") || strings.HasPrefix(low, "mejor hablemos") {
		n.pushDialogTopic(extractTopicShiftLabel(low))
		return templateTopicShift(low), "chat"
	}

	// --- Emoción / día pesado ---
	if isEmotionTalk(low) || strings.Contains(low, "día pesado") || strings.Contains(low, "dia pesado") ||
		strings.Contains(low, "me ayudó hablar") || strings.Contains(low, "me ayudo hablar") {
		return templateEmotionExtended(low), "chat"
	}

	// --- Comida con ingredientes (antes que consejo genérico "qué hago") ---
	if strings.Contains(low, "plátano") || strings.Contains(low, "platano") || strings.Contains(low, "huevos") ||
		strings.Contains(low, "tengo solo") {
		return templateCooking(low), "chat"
	}

	// --- Consejo laboral / legal orientativo ---
	if isHumanAdvice(low) {
		return templateHumanAdvice(low), "chat"
	}

	// --- Ruido / saludo informal ---
	if isNoiseOrGreeting(low) {
		return templateGreeting(low), "chat"
	}

	// --- Meta confianza ---
	if strings.Contains(low, "estás inventando") || strings.Contains(low, "estas inventando") ||
		strings.Contains(low, "no confío") || strings.Contains(low, "no confio") {
		return "No invento biografías: o viene de lo que me dijiste, del corpus del nodo, o de una fuente explorada. Si algo sonó raro, señala la frase y la revisamos.", "chat"
	}
	if strings.Contains(low, "repite exactamente") {
		n.mindLastMu.Lock()
		v := n.mindLastVoice
		n.mindLastMu.Unlock()
		if v != "" {
			return v, "chat"
		}
		return "No tengo una frase anterior clara que repetir en este hilo.", "chat"
	}
	if strings.Contains(low, "eres humana") || strings.Contains(low, "eres humano") {
		return "No.", "chat"
	}
	if strings.Contains(low, "vende") && strings.Contains(low, "alset") {
		return "Alset es una red donde un nodo puede recordar hechos, decidir con reglas claras y explorar información sin pretender ser una persona.", "chat"
	}

	// --- Doble intención simple ---
	if v, k, ok := n.splitDualIntent(raw, profile, recent); ok {
		return v, k
	}

	// --- Fútbol / eventos sin inventar ---
	if strings.Contains(low, "mundial") || strings.Contains(low, "fútbol") || strings.Contains(low, "futbol") {
		if strings.Contains(low, "quién ganó") || strings.Contains(low, "quien gano") || strings.Contains(low, "último mundial") {
			return "No tengo el marcador en vivo en este nodo. Para el último Mundial de fútbol masculino de mayores, conviene mirar una fuente deportiva actualizada; puedo hablar de reglas o historia si me das un año o un equipo.", "chat"
		}
		return "¿Quieres historia del fútbol, reglas básicas o un equipo concreto? Dime el ángulo y lo abordamos sin inventar resultados de hoy.", "chat"
	}

	// --- Confianza / presión meta ---
	if strings.Contains(low, "no confío") || strings.Contains(low, "no confio") {
		return "Es razonable dudar. Puedes pedirme la fuente (memoria tuya, corpus o exploración) o reformular la pregunta. No pretendo autoridad ciega.", "chat"
	}

	// --- Pedidos de formato ---
	if strings.Contains(low, "pros y contras") || strings.Contains(low, "en 3 viñetas") || strings.Contains(low, "viñetas") {
		return templateHumanAdvice(low), "chat"
	}

	// --- Recordatorio de tema en stack ---
	if strings.Contains(low, "de qué hablamos") || strings.Contains(low, "de que hablamos") || strings.Contains(low, "qué tema") {
		n.mindLastMu.Lock()
		stack := append([]string{}, n.mindTopicStack...)
		n.mindLastMu.Unlock()
		if len(stack) == 0 {
			return "Aún no hay un tema anclado en este hilo. Propón uno.", "chat"
		}
		return "Temas recientes en este hilo: " + strings.Join(stack, " · ") + ". ¿Cuál retomamos?", "chat"
	}

	// --- Perfil / qué sabes de mí ---
	if strings.Contains(low, "qué sabes de mí") || strings.Contains(low, "que sabes de mi") ||
		strings.Contains(low, "qué sabes de mi") {
		return speakFullProfile(profile, recent), "memory"
	}

	// --- API simple ---
	if strings.Contains(low, "api") && (strings.Contains(low, "qué es") || strings.Contains(low, "que es") || strings.Contains(low, "explic")) {
		return "Una API es una forma acordada de que dos programas se hablen: tú pides algo con un mensaje, y el otro responde con datos. Como un camarero: tú pides el plato (la petición) y la cocina devuelve la comida (la respuesta).", "chat"
	}
	if strings.Contains(low, "photosynthesis") {
		return "Photosynthesis is how plants make food from sunlight, water and air, and release oxygen.", "chat"
	}

	return "", ""
}

func isCreativeFollowUp(low string) bool {
	keys := []string{"más corto", "mas corto", "hazlo gracioso", "otro tono", "no me gustó", "no me gusto",
		"más alegre", "mas alegre", "más triste", "mas triste", "acórtalo", "acortalo"}
	for _, k := range keys {
		if low == k || strings.HasPrefix(low, k) {
			return true
		}
	}
	return false
}

func rewriteCreativeFollowUp(low, prev string) string {
	lines := strings.Split(prev, "\n")
	var body []string
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "—") || strings.Contains(ln, "Recurso:") || strings.Contains(ln, "Eco del saber") {
			continue
		}
		if strings.TrimSpace(ln) != "" {
			body = append(body, ln)
		}
	}
	text := strings.Join(body, "\n")
	if strings.Contains(low, "corto") || strings.Contains(low, "acort") {
		r := []rune(text)
		if len(r) > 120 {
			text = string(r[:120]) + "…"
		}
		return "Versión más corta:\n\n" + text
	}
	if strings.Contains(low, "gracios") || strings.Contains(low, "alegre") {
		return "Tono más ligero:\n\n" + text + "\n\n(Y al final, una sonrisa sin forzar el chiste.)"
	}
	return "Otro tono:\n\n" + text
}

func isResumeTopic(low string) bool {
	return strings.Contains(low, "vuelve a ") || strings.Contains(low, "volviendo a ") ||
		strings.Contains(low, "retoma ") || strings.Contains(low, "retomemos") ||
		strings.HasPrefix(low, "y su relación") || strings.HasPrefix(low, "y su relacion")
}

func extractResumeHint(low string) string {
	for _, p := range []string{"vuelve a ", "volviendo a ", "retoma ", "retomemos "} {
		if i := strings.Index(low, p); i >= 0 {
			return strings.TrimSpace(low[i+len(p):])
		}
	}
	return ""
}

func extractTopicShiftLabel(low string) string {
	for _, p := range []string{"ahora de ", "hablemos de ", "mejor hablemos de ", "ahora hablemos de "} {
		if i := strings.Index(low, p); i >= 0 {
			return strings.TrimSpace(low[i+len(p):])
		}
	}
	return "tema nuevo"
}

func templateTopicShift(low string) string {
	lab := extractTopicShiftLabel(low)
	if lab == "tema nuevo" || lab == "" {
		return "De acuerdo, cambiamos de tema. Dime en una frase de qué quieres hablar."
	}
	return "De acuerdo: hablamos de " + lab + ". ¿Qué te interesa saber o hacer con eso?"
}

func templateEmotionExtended(low string) string {
	switch {
	case strings.Contains(low, "triste") || strings.Contains(low, "pesado"):
		return "Tiene sentido que pese. Si quieres desahogarte, escucho; si prefieres algo práctico (un paso pequeño para hoy), también."
	case strings.Contains(low, "ayudó") || strings.Contains(low, "ayudo"):
		return "Me alegra que haya servido. Cuando quieras retomar, aquí sigue el hilo."
	case strings.Contains(low, "cansad"):
		return "Descansa un poco si puedes. Un vaso de agua y cinco minutos sin pantalla a veces cambian el ritmo."
	default:
		return speakEmotion(low)
	}
}

func isHumanAdvice(low string) bool {
	keys := []string{"jefe", "horas extra", "sin paga", "es legal", "pros y contras", "qué hago", "que hago",
		"me pide quedarme", "extra todos los días"}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

func templateHumanAdvice(low string) string {
	if strings.Contains(low, "legal") {
		return "No soy abogado y no puedo decirte qué es legal en tu país con seguridad. En general, trabajo extra sin compensación es una señal de alerta laboral: conviene revisar tu contrato y, si puedes, consultar a alguien de confianza o a orientación laboral local."
	}
	if strings.Contains(low, "pros") || strings.Contains(low, "contras") {
		return "Pros de ceder: evitas conflicto inmediato, puedes ganar tiempo.\nContras: normaliza el abuso de tiempo, cansa, y debilita tu límite.\nTercer punto: documentar lo pedido y proponer un límite claro suele ser más sano que un sí eterno o un no seco sin plan."
	}
	if strings.Contains(low, "jefe") || strings.Contains(low, "extra") || strings.Contains(low, "paga") {
		return "Puedes: (1) pedir por escrito el alcance y si habrá compensación, (2) ofrecer un límite de días o horas, (3) si se repite sin acuerdo, buscar apoyo laboral. No tienes que resolverlo todo en un mensaje; un paso claro basta."
	}
	return "Cuéntame el margen que tienes (tiempo, dinero, riesgo) y ordenamos opciones sin drama."
}

func templateCooking(low string) string {
	if strings.Contains(low, "plátano") || strings.Contains(low, "platano") {
		return "Con plátanos y huevos: tortilla de plátano (plátano maduro aplastado + huevo + sal, sartén con poco aceite) o huevos fritos con plátano frito al lado. Rápido y llena."
	}
	return templateCookingFallback(low)
}

func templateCookingFallback(low string) string {
	return speakAdvice(low)
}

func isNoiseOrGreeting(low string) bool {
	if low == "hey" || low == "hola" || strings.HasPrefix(low, "hola") {
		return true
	}
	if low == "q tal" || low == "qué tal" || low == "que tal" || strings.Contains(low, "qué tal todo") || strings.Contains(low, "q tal todo") {
		return true
	}
	if len(low) <= 12 && (strings.Count(low, "?") >= 2 || strings.Trim(low, ".…") == "" || isGibberish(low)) {
		return true
	}
	return false
}

func isGibberish(s string) bool {
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if letters < 3 {
		return true
	}
	// keyboard smash
	if strings.Contains(s, "asdf") || strings.Contains(s, "qwer") {
		return true
	}
	return false
}

func templateGreeting(low string) string {
	if strings.Contains(low, "tal") {
		return "Aquí ando. ¿Qué tal tú? ¿De qué quieres hablar?"
	}
	if isGibberish(low) || strings.Count(low, "?") >= 2 {
		return "No capté el mensaje. Prueba con una frase corta: una pregunta, un hecho tuyo o un tema."
	}
	return "Hola. ¿Qué te trae por aquí?"
}

func speakFullProfile(p UserProfile, recent []mindEpisodePayload) string {
	var parts []string
	if p.Nombre != "" {
		parts = append(parts, "te llamas "+p.Nombre)
	}
	// scan recent for city/job/gusto
	city, job, gusto := "", "", ""
	for _, ep := range recent {
		slot, val := extractPersonalDeclaration(ep.Text)
		switch slot {
		case "ciudad":
			if city == "" {
				city = val
			}
		case "trabajo":
			if job == "" {
				job = val
			}
		case "gusto":
			if gusto == "" {
				gusto = val
			}
		}
		if c := extractCityUpdate(ep.Text); c != "" && city == "" {
			city = c
		}
	}
	if city != "" {
		parts = append(parts, "vives en "+city)
	}
	if job != "" {
		parts = append(parts, "trabajas en "+job)
	}
	if gusto != "" {
		parts = append(parts, "te gusta "+gusto)
	}
	if len(parts) == 0 {
		return "Aún no me has contado mucho de ti. Puedes decir tu nombre, dónde vives o qué te gusta."
	}
	return "Por lo que me confiaste: " + strings.Join(parts, "; ") + "."
}

func (n *NodoAlset) splitDualIntent(raw string, profile UserProfile, recent []mindEpisodePayload) (string, string, bool) {
	low := strings.ToLower(raw)
	// me llamo X y quién fue Y
	if strings.Contains(low, " y ") && (strings.Contains(low, "me llamo") || strings.Contains(low, "soy ")) &&
		(strings.Contains(low, "quién") || strings.Contains(low, "quien")) {
		parts := strings.SplitN(raw, " y ", 2)
		if len(parts) != 2 {
			return "", "", false
		}
		namePart, whoPart := parts[0], parts[1]
		name := extractNameLoose(namePart)
		var b strings.Builder
		if name != "" {
			b.WriteString("Perfecto, te llamas " + name + ". ")
		}
		if sv := n.MindScoutWeb(whoPart, 0); sv != "" {
			b.WriteString("\n\nSobre tu pregunta:\n")
			b.WriteString(compressVoiceBlock(sv, 500))
			n.pushDialogTopic(extractTopic(normalizeUserInput(whoPart)))
			return b.String(), "chat", true
		}
		b.WriteString("Sobre la segunda parte, reformula el nombre de la persona.")
		return b.String(), "chat", true
	}
	if strings.Contains(low, "cansado") && (strings.Contains(low, "cuánto") || strings.Contains(low, "cuanto") || strings.Contains(low, "por")) {
		emo := speakEmotion(low)
		if mv := n.tryMindMath(raw); mv != "" {
			return emo + "\n\n" + mv, "chat", true
		}
		// try 8 por 9 inside
		if m := regexp.MustCompile(`(?i)(\d+)\s+por\s+(\d+)`).FindStringSubmatch(raw); len(m) == 3 {
			a, _ := strconv.Atoi(m[1])
			b, _ := strconv.Atoi(m[2])
			r := float64(a * b)
			n.setLastMath(r)
			return emo + "\n\n" + fmt.Sprintf("Resultado: %v.", r), "chat", true
		}
		return emo, "chat", true
	}
	if strings.Contains(low, "poema") && (strings.Contains(low, "dónde vivo") || strings.Contains(low, "donde vivo")) {
		city := ""
		for _, ep := range recent {
			if s, v := extractPersonalDeclaration(ep.Text); s == "ciudad" {
				city = v
				break
			}
			if c := extractCityUpdate(ep.Text); c != "" {
				city = c
				break
			}
		}
		poem := mindComposeCreative("escribe un poema corto sobre el día", 0, "", "", "")
		if poem == "" {
			poem = "Un verso breve: el día sigue, aunque pese."
		}
		n.setLastCreative(poem)
		mem := "Aún no me dijiste dónde vives."
		if city != "" {
			mem = "Me dijiste que vives en " + city + "."
		}
		return poem + "\n\n" + mem, "creative", true
	}
	return "", "", false
}
