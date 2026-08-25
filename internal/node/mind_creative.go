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
		"historia corta", "versos sobre", "rima sobre",
	}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// extractCreativeTheme pulls topic after sobre/de/del.
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
	// fallback: strip write verbs
	t := extractTopic(normalizeUserInput(s))
	for _, junk := range []string{"poema", "cuento", "historia", "versos", "rima"} {
		t = strings.ReplaceAll(t, junk, "")
	}
	return strings.TrimSpace(t)
}

// mindComposeCreative — composición determinista (plantillas + memoria/corpus), no LLM.
func mindComposeCreative(userText string, ethicsState int, memSpeak, knowSpeak string) string {
	if ethicsState == 2 {
		return "Ethics en veto: no compongo texto creativo ahora."
	}
	low := strings.ToLower(userText)
	theme := extractCreativeTheme(userText)
	if theme == "" {
		theme = "el instante presente"
	}
	kind := "poema"
	if strings.Contains(low, "cuento") || strings.Contains(low, "historia") {
		kind = "cuento"
	}

	// Anclas: corpus literario/factual, scout limpio. Nunca soft-memory ni pedidos previos.
	anchor := pickCreativeAnchor(theme, memSpeak, knowSpeak)
	device := pickLiteraryDevice(theme)

	var b strings.Builder
	if kind == "cuento" {
		b.WriteString(composeShortTale(theme, anchor, device))
	} else {
		b.WriteString(composePoem(theme, anchor, device))
	}
	b.WriteString("\n\n— Composición ternaria (plantilla + memoria/corpus), no predicción de tokens.")
	if anchor != "" {
		b.WriteString("\nAncla factual usada: sí (recortada).")
	} else {
		b.WriteString("\nAncla factual: ninguna en memoria; imagen simbólica genérica.")
	}
	return b.String()
}

func composePoem(theme, anchor, device string) string {
	theme = strings.TrimSpace(theme)
	if device == "" {
		device = "imagen concreta"
	}
	lines := []string{
		fmt.Sprintf("Sobre %s — con %s —", theme, device),
	}
	switch {
	case strings.Contains(device, "haiku"):
		lines = []string{
			fmt.Sprintf("agua o cielo: %s", truncateRunes(theme, 24)),
			"un corte de silencio en medio",
			"eco que no explica",
		}
	case strings.Contains(device, "anáfora") || strings.Contains(device, "anafora"):
		lines = []string{
			fmt.Sprintf("No es solo %s lo que nombra el pedido,", theme),
			fmt.Sprintf("no es solo %s lo que cabe en un verso,", theme),
			fmt.Sprintf("no es solo %s: es el modo de mirar.", theme),
		}
	case strings.Contains(device, "símil") || strings.Contains(device, "simil"):
		lines = []string{
			fmt.Sprintf("%s avanza como marea en calma,", capitalizeFirst(theme)),
			"deja marcas en la arena del oído,",
			"y al retirarse, deja una forma legible.",
		}
	default: // metáfora / verso libre
		lines = []string{
			fmt.Sprintf("%s no es un cartel: es una puerta,", capitalizeFirst(theme)),
			"cruzarla pide ritmo, no inventario,",
			"tres pasos: ver, nombrar, callar a tiempo.",
		}
	}
	if anchor != "" {
		lines = append(lines, "", "Nota de ancla (corpus, no biografía inventada):", compressVoiceBlock(anchor, 110))
	}
	return strings.Join(lines, "\n")
}

func capitalizeFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}

func composeShortTale(theme, anchor, device string) string {
	theme = strings.TrimSpace(theme)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Alguien pidió un relato sobre «%s».\n", theme))
	b.WriteString("Hubo un deseo (entender), un obstáculo (no inventar hechos) y un cierre (dejar constancia del acto).\n")
	if device != "" {
		b.WriteString("Recurso elegido: " + device + ".\n")
	}
	if anchor != "" {
		b.WriteString("Marco del corpus: ")
		b.WriteString(compressVoiceBlock(anchor, 120))
		b.WriteString("\n")
	}
	b.WriteString("Cierre: el gen —si de eso se trata— no es magia: es una célula con clave, misión y memoria CID.")
	return b.String()
}

func isBadCreativeAnchor(s string) bool {
	low := strings.ToLower(s)
	if low == "" {
		return true
	}
	bad := []string{
		"me suena esto", "escribe un poema", "escribe un cuento", "hallazgo sonda",
		"scout-", "¿seguimos por ahí", "seguimos por ahi", "prefers-color",
		"composición ternaria", "composicion ternaria", "write=2",
	}
	for _, b := range bad {
		if strings.Contains(low, b) {
			return true
		}
	}
	return false
}

func pickCreativeAnchor(theme, memSpeak, knowSpeak string) string {
	// 1) knowledge on theme or literary definition
	if k := speakFromKnowledge(theme); k != "" && !isBadCreativeAnchor(k) {
		return compressVoiceBlock(k, 160)
	}
	if knowSpeak != "" && !isBadCreativeAnchor(knowSpeak) {
		return compressVoiceBlock(knowSpeak, 160)
	}
	if k := speakFromKnowledge("qué es un poema"); k != "" && theme != "" {
		// literary craft note only if no thematic knowledge
		_ = k
	}
	if lit := speakFromKnowledge("recursos literarios"); lit != "" && !isBadCreativeAnchor(lit) {
		// prefer thematic scout over pure craft when available
	}
	if _, prev, ok := recallScoutFinding(theme); ok && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) && !isBadCreativeAnchor(prev) {
		return compressVoiceBlock(prev, 140)
	}
	if memSpeak != "" && !isBadCreativeAnchor(memSpeak) {
		return compressVoiceBlock(memSpeak, 120)
	}
	return ""
}

func pickLiteraryDevice(theme string) string {
	// rotate deterministically by theme length
	devices := []string{"metáfora", "símil", "anáfora", "verso libre", "haiku"}
	if theme == "" {
		return devices[0]
	}
	i := 0
	for _, r := range theme {
		i += int(r)
	}
	return devices[i%len(devices)]
}
