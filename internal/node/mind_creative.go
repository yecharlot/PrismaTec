package node

import (
	"fmt"
	"strings"
)

// isCreativeWriteRequest — poema/cuento/historia/redactar (no codegen).
func isCreativeWriteRequest(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if isCodeGenRequest(low) {
		return false
	}
	keys := []string{
		"escribe un poema", "escribe poema", "escribe un cuento", "escribe cuento",
		"escribe una historia", "redacta un", "compón un", "compon un",
		"haz un poema", "hazme un poema", "poema sobre", "cuento sobre",
		"historia corta", "versos sobre", "rima sobre", "escribe un párrafo", "escribe un parrafo",
	}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

func extractCreativeTheme(s string) string {
	low := strings.ToLower(s)
	for _, p := range []string{"sobre ", "de la ", "de el ", "del ", "de ", "acerca de "} {
		if i := strings.Index(low, p); i >= 0 {
			rest := strings.TrimSpace(s[i+len(p):])
			rest = strings.Trim(rest, ".,;:!?¡¿\"'")
			if rest != "" {
				return rest
			}
		}
	}
	t := extractTopic(normalizeUserInput(s))
	for _, junk := range []string{"poema", "cuento", "historia", "versos", "rima", "parrafo", "párrafo"} {
		t = strings.ReplaceAll(t, junk, "")
	}
	return strings.TrimSpace(t)
}

func mindComposeCreative(userText string, ethicsState int, memSpeak, knowSpeak string) string {
	if ethicsState == 2 {
		return "Ethics en veto: no compongo texto creativo ahora."
	}
	low := strings.ToLower(userText)
	theme := extractCreativeTheme(userText)
	if theme == "" {
		theme = "el instante"
	}
	kind := "poema"
	if strings.Contains(low, "cuento") || strings.Contains(low, "historia") {
		kind = "cuento"
	} else if strings.Contains(low, "párrafo") || strings.Contains(low, "parrafo") {
		kind = "parrafo"
	}

	anchor := pickCreativeAnchor(theme, memSpeak, knowSpeak)
	device := pickLiteraryDevice(theme)

	var body string
	switch kind {
	case "cuento":
		body = composeShortTale(theme, anchor, device)
	case "parrafo":
		body = composeParagraph(theme, anchor)
	default:
		body = composePoem(theme, anchor, device)
	}

	var b strings.Builder
	b.WriteString(body)
	if device != "" && kind == "poema" {
		b.WriteString("\n\n(Recurso: ")
		b.WriteString(device)
		b.WriteString(".)")
	}
	return b.String()
}

func isBadCreativeAnchor(s string) bool {
	low := strings.ToLower(s)
	if low == "" {
		return true
	}
	bad := []string{
		"me suena esto", "escribe un poema", "escribe un cuento", "hallazgo sonda",
		"scout-", "seguimos por ahí", "seguimos por ahi", "prefers-color",
		"composición ternaria", "composicion ternaria", "write=2",
		"un poema organiza ritmo", // craft-only: not thematic anchor
		"recursos: aliteración", "metáfora: decir que",
	}
	for _, b := range bad {
		if strings.Contains(low, b) {
			return true
		}
	}
	return false
}

func pickCreativeAnchor(theme, memSpeak, knowSpeak string) string {
	theme = strings.TrimSpace(theme)
	// Thematic knowledge first (amor, gen, harry potter…)
	if theme != "" {
		if k := speakFromKnowledge(theme); k != "" && !isBadCreativeAnchor(k) {
			return compressVoiceBlock(k, 180)
		}
		if k := speakFromKnowledge("qué es " + theme); k != "" && !isBadCreativeAnchor(k) {
			return compressVoiceBlock(k, 180)
		}
		if _, prev, ok := recallScoutFinding(theme); ok && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) && !isBadCreativeAnchor(prev) {
			return compressVoiceBlock(prev, 160)
		}
	}
	if knowSpeak != "" && !isBadCreativeAnchor(knowSpeak) {
		return compressVoiceBlock(knowSpeak, 160)
	}
	if memSpeak != "" && !isBadCreativeAnchor(memSpeak) {
		return compressVoiceBlock(memSpeak, 120)
	}
	return ""
}

func pickLiteraryDevice(theme string) string {
	devices := []string{"metáfora", "símil", "anáfora", "verso libre", "haiku", "personificación"}
	if theme == "" {
		return devices[0]
	}
	i := 0
	for _, r := range theme {
		i += int(r)
	}
	return devices[i%len(devices)]
}

func capitalizeFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(strings.ToLower(s))
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}

func composePoem(theme, anchor, device string) string {
	theme = strings.TrimSpace(theme)
	dev := strings.ToLower(device)

	var lines []string
	switch {
	case strings.Contains(dev, "haiku"):
		lines = []string{
			haikuLine(theme, 5),
			haikuLine(theme+" silencio", 7),
			"eco sin explicación",
		}
	case strings.Contains(dev, "anáfora") || strings.Contains(dev, "anafora"):
		lines = []string{
			fmt.Sprintf("No es solo %s lo que llega a la boca,", theme),
			fmt.Sprintf("no es solo %s lo que cabe en el pecho,", theme),
			fmt.Sprintf("no es solo %s: es el modo de mirar y no soltar.", theme),
		}
	case strings.Contains(dev, "símil") || strings.Contains(dev, "simil"):
		lines = []string{
			fmt.Sprintf("%s avanza como marea en calma,", capitalizeFirst(theme)),
			"deja marcas en la arena del oído,",
			"y al retirarse, deja una forma que se puede leer en voz baja.",
		}
	case strings.Contains(dev, "personificación"):
		lines = []string{
			fmt.Sprintf("%s se sienta al borde de la frase,", capitalizeFirst(theme)),
			"pregunta sin preguntar, espera sin reloj,",
			"y cuando nos callamos, sigue escuchando.",
		}
	case strings.Contains(dev, "metáfora"):
		lines = []string{
			fmt.Sprintf("%s no es un cartel: es una puerta entreabierta,", capitalizeFirst(theme)),
			"cruzarla pide ritmo, no inventario,",
			"tres pasos: ver, nombrar, callar a tiempo.",
		}
	default:
		lines = []string{
			fmt.Sprintf("Bajo el nombre de %s hay agua y hay piedra,", theme),
			"el verso elige qué tocar primero,",
			"y deja el resto al silencio que también es parte.",
		}
	}

	out := strings.Join(lines, "\n")
	if anchor != "" && !isLiteraryCraftOnly(anchor) {
		out += "\n\n— Eco del saber (no inventado):\n" + compressVoiceBlock(anchor, 100)
	}
	return out
}

func haikuLine(seed string, n int) string {
	// approximate syllable budget with words, not true Spanish metrics
	words := strings.Fields(seed)
	if len(words) == 0 {
		return "agua quieta"
	}
	if n <= 5 {
		return words[0]
	}
	if len(words) >= 2 {
		return words[0] + " " + words[1]
	}
	return words[0] + " en la orilla"
}

func isLiteraryCraftOnly(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "un poema organiza") ||
		strings.Contains(low, "metáfora: decir") ||
		strings.Contains(low, "recursos: aliteración") ||
		strings.Contains(low, "haiku: tres versos")
}

func composeShortTale(theme, anchor, device string) string {
	theme = strings.TrimSpace(theme)
	low := strings.ToLower(theme)
	var b strings.Builder

	// Domain-aware opening
	switch {
	case strings.Contains(low, "gen") && !strings.Contains(low, "génesis") && !strings.Contains(low, "gente"):
		b.WriteString("En el nodo, una célula con clave ANS despertó sin alarde.\n")
		b.WriteString("No pedía magia: pedía misión, RootCID y un lugar donde servir.\n")
		if anchor != "" {
			b.WriteString(compressVoiceBlock(anchor, 140))
			b.WriteString("\n")
		}
		b.WriteString("Cierre: el gen no es un dios pequeño; es memoria que viaja y obedece ethics.")
	case strings.Contains(low, "harry") || strings.Contains(low, "potter") || strings.Contains(low, "mago"):
		b.WriteString("Había una cicatriz que no era adorno, sino recordatorio.\n")
		b.WriteString("Alguien aprendió que el coraje no grita: elige, una vez y otra, no abandonar a los suyos.\n")
		if anchor != "" {
			b.WriteString("Marco (serie, no fanfic de trama nueva): ")
			b.WriteString(compressVoiceBlock(anchor, 120))
			b.WriteString("\n")
		}
		b.WriteString("Cierre: la magia del relato es la lealtad, no el hechizo fácil.")
	case strings.Contains(low, "mar") || strings.Contains(low, "océano") || strings.Contains(low, "oceano"):
		b.WriteString("El mar no explica: empuja.\n")
		b.WriteString("Un caminante dejó huellas que la marea borró sin rencor.\n")
		b.WriteString("Cierre: lo que permanece no es la marca, sino haber mirado el agua de frente.")
	default:
		b.WriteString(fmt.Sprintf("Alguien nombró «%s» y el aire cambió de peso.\n", theme))
		b.WriteString("Hubo un deseo (entender), un obstáculo (no inventar lo que no se sabe) y un gesto final (decir solo lo justo).\n")
		if anchor != "" {
			b.WriteString("Lo que sí había en el corpus: ")
			b.WriteString(compressVoiceBlock(anchor, 120))
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("Cierre: %s quedó nombrado, no agotado.", theme))
	}
	_ = device
	return b.String()
}

func composeParagraph(theme, anchor string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Sobre %s cabe un párrafo sin adornos: ", theme))
	if anchor != "" {
		b.WriteString(compressVoiceBlock(anchor, 200))
	} else {
		b.WriteString("aún no hay ficha sólida en este nodo, así que el texto se limita a señalar el tema y esperar un dato o una exploración.")
	}
	return b.String()
}
