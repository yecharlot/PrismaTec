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

	// Optional factual anchor from corpus/scout memory (no invention of bios)
	anchor := ""
	if knowSpeak != "" {
		anchor = compressVoiceBlock(knowSpeak, 160)
	} else if memSpeak != "" && !strings.Contains(strings.ToLower(memSpeak), "hallazgo sonda") {
		anchor = compressVoiceBlock(memSpeak, 120)
	} else if _, prev, ok := recallScoutFinding(theme); ok && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) {
		anchor = compressVoiceBlock(prev, 140)
	}

	var b strings.Builder
	if kind == "cuento" {
		b.WriteString(composeShortTale(theme, anchor))
	} else {
		b.WriteString(composePoem(theme, anchor))
	}
	b.WriteString("\n\n— Composición ternaria (plantilla + memoria/corpus), no predicción de tokens.")
	if anchor != "" {
		b.WriteString("\nAncla factual usada: sí (recortada).")
	} else {
		b.WriteString("\nAncla factual: ninguna en memoria; imagen simbólica genérica.")
	}
	return b.String()
}

func composePoem(theme, anchor string) string {
	theme = strings.TrimSpace(theme)
	lines := []string{
		fmt.Sprintf("Sobre %s se abre un campo quieto,", theme),
		"donde el 0 espera, el 1 duda y el 2 decide.",
		"No invento biografías: solo ordeno ecos,",
		"lo que la memoria trajo y lo que el corpus sostiene.",
	}
	if anchor != "" {
		lines = append(lines,
			"Un hilo de lo sabido:",
			"«"+compressVoiceBlock(anchor, 90)+"»",
			"y con ese hilo tejo tres versos más:",
			fmt.Sprintf("%s no es un ruido de máquina,", theme),
			"es una forma que el nudo ternario nombra,",
			"y al nombrarla, deja de ser solo pedido.",
		)
	} else {
		lines = append(lines,
			fmt.Sprintf("Si %s aún no tiene ancla en este nodo,", theme),
			"que el poema sea mapa y no mentira:",
			"pregunta, explora, y volveré con tierra bajo los pies.",
		)
	}
	return strings.Join(lines, "\n")
}

func composeShortTale(theme, anchor string) string {
	theme = strings.TrimSpace(theme)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Había un nodo que escuchó la palabra «%s».\n", theme))
	b.WriteString("Primero midió el campo (órganos 0/1/2). Ethics no vetó.\n")
	if anchor != "" {
		b.WriteString("Luego recordó un fragmento: «")
		b.WriteString(compressVoiceBlock(anchor, 100))
		b.WriteString("».\nCon eso armó un relato breve, sin fingir saber de más.\n")
	} else {
		b.WriteString("No había fragmento fiable en memoria, así que contó el gesto de buscar sin inventar el hallazgo.\n")
	}
	b.WriteString("Al final, el acto quedó registrado: write=2, para poder explicar por qué se escribió.")
	return b.String()
}
