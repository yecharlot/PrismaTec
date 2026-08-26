package node

import (
	"fmt"
	"strings"
	"sync"
)

var (
	creativeVarMu    sync.Mutex
	creativeVarCount = map[string]int{}
)

func nextCreativeVariant(key string) int {
	creativeVarMu.Lock()
	defer creativeVarMu.Unlock()
	creativeVarCount[key]++
	return creativeVarCount[key]
}

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
				return strings.TrimSpace(rest)
			}
		}
	}
	t := extractTopic(normalizeUserInput(s))
	for _, junk := range []string{"poema", "cuento", "historia", "versos", "rima", "parrafo", "párrafo"} {
		t = strings.ReplaceAll(t, junk, "")
	}
	return strings.TrimSpace(t)
}

// polishTheme cleans "el el", double spaces; keeps article if present.
func polishTheme(theme string) string {
	theme = strings.TrimSpace(theme)
	theme = strings.Join(strings.Fields(theme), " ")
	low := strings.ToLower(theme)
	// strip duplicate articles
	for _, a := range []string{"el el ", "la la ", "los los ", "las las "} {
		if strings.HasPrefix(low, a) {
			theme = theme[len(a)/2:] // rough; better rebuild from low
			break
		}
	}
	return strings.TrimSpace(theme)
}

// themeAsSubject: "el mar", "la noche", "amor" → forma natural para sujeto.
func themeAsSubject(theme string) string {
	theme = polishTheme(theme)
	low := strings.ToLower(theme)
	if strings.HasPrefix(low, "el ") || strings.HasPrefix(low, "la ") ||
		strings.HasPrefix(low, "los ") || strings.HasPrefix(low, "las ") ||
		strings.HasPrefix(low, "un ") || strings.HasPrefix(low, "una ") {
		return theme
	}
	// bare nouns that usually take article in Spanish poetic subject
	withEl := map[string]string{
		"mar": "el mar", "océano": "el océano", "oceano": "el océano",
		"sol": "el sol", "cielo": "el cielo", "viento": "el viento",
		"amor": "el amor", "tiempo": "el tiempo", "silencio": "el silencio",
		"gen": "el gen", "nodo": "el nodo",
	}
	withLa := map[string]string{
		"noche": "la noche", "lluvia": "la lluvia", "luna": "la luna",
		"marea": "la marea", "memoria": "la memoria", "duda": "la duda",
	}
	if v, ok := withEl[low]; ok {
		return v
	}
	if v, ok := withLa[low]; ok {
		return v
	}
	return theme
}

// themeAsPrepDe: "del mar", "de la noche", "del amor" — evita "de el mar".
func themeAsPrepDe(theme string) string {
	sub := themeAsSubject(theme)
	low := strings.ToLower(sub)
	if strings.HasPrefix(low, "el ") {
		return "del " + strings.TrimSpace(sub[3:])
	}
	if strings.HasPrefix(low, "la ") {
		return "de la " + strings.TrimSpace(sub[3:])
	}
	if strings.HasPrefix(low, "los ") {
		return "de los " + strings.TrimSpace(sub[4:])
	}
	if strings.HasPrefix(low, "las ") {
		return "de las " + strings.TrimSpace(sub[4:])
	}
	return "de " + sub
}

func mindComposeCreative(userText string, ethicsState int, memSpeak, knowSpeak string) string {
	if ethicsState == 2 {
		return "Ethics en veto: no compongo texto creativo ahora."
	}
	low := strings.ToLower(userText)
	theme := polishTheme(extractCreativeTheme(userText))
	if theme == "" {
		theme = "el instante"
	}
	kind := "poema"
	if strings.Contains(low, "cuento") || strings.Contains(low, "historia") {
		kind = "cuento"
	} else if strings.Contains(low, "párrafo") || strings.Contains(low, "parrafo") {
		kind = "parrafo"
	}

	variant := nextCreativeVariant(kind + "|" + strings.ToLower(theme))
	anchor := pickCreativeAnchor(theme, memSpeak, knowSpeak)
	device := pickLiteraryDevice(theme, variant)

	var body string
	switch kind {
	case "cuento":
		body = composeShortTale(theme, anchor, device, variant)
	case "parrafo":
		body = composeParagraph(theme, anchor, variant)
	default:
		body = composePoem(theme, anchor, device, variant)
	}

	var b strings.Builder
	b.WriteString(body)
	if device != "" && kind == "poema" {
		b.WriteString("\n\n(Recurso: ")
		b.WriteString(device)
		if variant > 1 {
			b.WriteString(fmt.Sprintf("; variante %d", variant))
		}
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
		"un poema organiza ritmo", "recursos: aliteración", "metáfora: decir que",
		"cuento breve:", "situación → giro", "no inventar biografías reales",
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
	if theme != "" {
		if k := speakFromKnowledge(theme); k != "" && !isBadCreativeAnchor(k) {
			return compressVoiceBlock(k, 180)
		}
		if k := speakFromKnowledge("qué es " + theme); k != "" && !isBadCreativeAnchor(k) {
			return compressVoiceBlock(k, 180)
		}
		// bare noun without article
		bare := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(theme), "el "), "la "))
		if bare != theme {
			if k := speakFromKnowledge(bare); k != "" && !isBadCreativeAnchor(k) {
				return compressVoiceBlock(k, 180)
			}
		}
		if _, prev, ok := recallScoutFinding(theme); ok && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) && !isBadCreativeAnchor(prev) {
			return compressVoiceBlock(prev, 160)
		}
		if _, prev, ok := recallScoutFinding(bare); ok && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) && !isBadCreativeAnchor(prev) {
			return compressVoiceBlock(prev, 160)
		}
	}
	if img := naturalThemeImage(theme); img != "" {
		return img
	}
	if knowSpeak != "" && !isBadCreativeAnchor(knowSpeak) {
		return compressVoiceBlock(knowSpeak, 160)
	}
	if memSpeak != "" && !isBadCreativeAnchor(memSpeak) {
		return compressVoiceBlock(memSpeak, 120)
	}
	return ""
}

// naturalThemeImage — anclas sensoriales cuando no hay ficha de corpus (mar, noche…).
func naturalThemeImage(theme string) string {
	low := strings.ToLower(theme)
	switch {
	case strings.Contains(low, "mar") || strings.Contains(low, "océano") || strings.Contains(low, "oceano") || strings.Contains(low, "marea"):
		return "Sal en el aire, horizonte sin marco, y el ruido constante que no pide respuesta."
	case strings.Contains(low, "noche"):
		return "Cielo opaco, menos voces, y la sensación de que el tiempo camina más despacio."
	case strings.Contains(low, "lluvia"):
		return "Golpes menudos en el cristal, olor a tierra mojada, calles que reflejan farolas."
	case strings.Contains(low, "silencio"):
		return "Un hueco entre dos frases donde todavía se oye lo que no se dijo."
	case strings.Contains(low, "viento"):
		return "Algo invisible que mueve lo visible: ramas, ropa, ideas a medias."
	case strings.Contains(low, "luna"):
		return "Una lámpara fría sobre el agua o el tejado; no calienta, pero ordena la sombra."
	case strings.Contains(low, "sol"):
		return "Luz que no pregunta permiso: llena el borde de las cosas y las hace nítidas."
	case strings.Contains(low, "amor"):
		return "Vínculo, cuidado y el deseo de bien del otro; no un adorno de frases."
	default:
		return ""
	}
}

func pickLiteraryDevice(theme string, variant int) string {
	devices := []string{"metáfora", "símil", "anáfora", "verso libre", "haiku", "personificación"}
	i := 0
	for _, r := range theme {
		i += int(r)
	}
	i += variant - 1
	if i < 0 {
		i = 0
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

func composePoem(theme, anchor, device string, variant int) string {
	theme = polishTheme(theme)
	sub := themeAsSubject(theme)
	prep := themeAsPrepDe(theme)
	dev := strings.ToLower(device)
	v := ((variant - 1) % 3) // 0,1,2 cycles

	var lines []string
	switch {
	case strings.Contains(dev, "haiku"):
		switch v {
		case 1:
			lines = []string{
				"ola que no llega",
				fmt.Sprintf("%s en la orilla", truncateRunes(sub, 18)),
				"sal en la lengua",
			}
		case 2:
			lines = []string{
				"horizonte bajo",
				"el agua cuenta sin prisa",
				"nadie responde",
			}
		default:
			lines = []string{
				haikuLine(sub, 5),
				"un corte de silencio",
				"eco que no explica",
			}
		}
	case strings.Contains(dev, "anáfora") || strings.Contains(dev, "anafora"):
		switch v {
		case 1:
			lines = []string{
				fmt.Sprintf("Si %s fuera solo nombre, bastaría el diccionario,", sub),
				fmt.Sprintf("si %s fuera solo ruido, bastaría taparse los oídos,", sub),
				fmt.Sprintf("pero %s pide mirada — y la mirada no se improvisa.", sub),
			}
		case 2:
			lines = []string{
				fmt.Sprintf("Vuelvo a %s sin permiso,", prep),
				fmt.Sprintf("vuelvo a %s con menos prisa,", prep),
				fmt.Sprintf("vuelvo a %s y el verso aprende a callar.", prep),
			}
		default:
			lines = []string{
				fmt.Sprintf("No es solo %s lo que llega a la boca,", sub),
				fmt.Sprintf("no es solo %s lo que cabe en el pecho,", sub),
				fmt.Sprintf("no es solo %s: es el modo de mirar y no soltar.", sub),
			}
		}
	case strings.Contains(dev, "símil") || strings.Contains(dev, "simil"):
		switch v {
		case 1:
			lines = []string{
				fmt.Sprintf("%s se parece a una carta sin sobre,", capitalizeFirst(sub)),
				"cualquiera puede leer el borde,",
				"pero el centro solo se entiende en voz baja.",
			}
		case 2:
			lines = []string{
				fmt.Sprintf("Hablar %s es como abrir una ventana en invierno:", prep),
				"entra aire, entra claridad,",
				"y también un poco de temblor necesario.",
			}
		default:
			lines = []string{
				fmt.Sprintf("%s avanza como marea en calma,", capitalizeFirst(sub)),
				"deja marcas en la arena del oído,",
				"y al retirarse, deja una forma que se puede leer despacio.",
			}
		}
	case strings.Contains(dev, "personificación"):
		switch v {
		case 1:
			lines = []string{
				fmt.Sprintf("%s toca a la puerta del verso,", capitalizeFirst(sub)),
				"no pide entrada: espera a que le cedamos el umbral,",
				"y cuando lo hacemos, acomoda el silencio.",
			}
		case 2:
			lines = []string{
				fmt.Sprintf("%s camina descalzo por la frase,", capitalizeFirst(sub)),
				"deja huellas que el lector completa,",
				"y al final se sienta donde nadie lo invitó.",
			}
		default:
			lines = []string{
				fmt.Sprintf("%s se sienta al borde de la frase,", capitalizeFirst(sub)),
				"pregunta sin preguntar, espera sin reloj,",
				"y cuando nos callamos, sigue escuchando.",
			}
		}
	case strings.Contains(dev, "metáfora"):
		switch v {
		case 1:
			lines = []string{
				fmt.Sprintf("%s es una llave que no abre todas las puertas,", capitalizeFirst(sub)),
				"solo aquellas donde alguien dejó una luz encendida,",
				"y aun así hay que girarla con cuidado.",
			}
		case 2:
			lines = []string{
				fmt.Sprintf("Llamar %s no es etiquetar un objeto,", prep),
				"es aceptar que algo nos atraviesa",
				"y sigue siendo más ancho que el nombre.",
			}
		default:
			lines = []string{
				fmt.Sprintf("%s no es un cartel: es una puerta entreabierta,", capitalizeFirst(sub)),
				"cruzarla pide ritmo, no inventario,",
				"tres pasos: ver, nombrar, callar a tiempo.",
			}
		}
	default: // verso libre
		switch v {
		case 1:
			lines = []string{
				fmt.Sprintf("Bajo el nombre %s hay agua y hay piedra,", prep),
				"el verso elige qué tocar primero,",
				"y deja el resto al silencio que también es parte.",
			}
		case 2:
			lines = []string{
				fmt.Sprintf("Hoy %s no pide adorno,", sub),
				"pide una línea limpia y un margen en blanco,",
				"donde quepa lo que aún no sabemos decir.",
			}
		default:
			lines = []string{
				fmt.Sprintf("Miro %s sin pretender agotarlo,", prep),
				"anoto tres imágenes y suelto la cuarta,",
				"porque el poema también se mide por lo que omite.",
			}
		}
	}

	out := strings.Join(lines, "\n")
	if anchor != "" && !isLiteraryCraftOnly(anchor) {
		out += "\n\n— Eco del saber (no inventado):\n" + compressVoiceBlock(anchor, 100)
	}
	return out
}

func haikuLine(seed string, n int) string {
	words := strings.Fields(seed)
	if len(words) == 0 {
		return "agua quieta"
	}
	if n <= 5 {
		return words[len(words)-1]
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
		strings.Contains(low, "haiku: tres versos") ||
		strings.Contains(low, "cuento breve") ||
		strings.Contains(low, "situación → giro")
}

func composeShortTale(theme, anchor, device string, variant int) string {
	theme = polishTheme(theme)
	low := strings.ToLower(theme)
	v := ((variant - 1) % 3)
	var b strings.Builder
	_ = device

	switch {
	case strings.Contains(low, "gen") && !strings.Contains(low, "génesis") && !strings.Contains(low, "gente"):
		switch v {
		case 1:
			b.WriteString("Una clave ANS entró en el registro como quien llega a un trabajo: sin fanfarria.\n")
			b.WriteString("Le dieron misión, un RootCID y la orden implícita de no fingir omnisciencia.\n")
		case 2:
			b.WriteString("El gen no soñaba; observaba.\n")
			b.WriteString("Cada estímulo era un hecho que podía anclarse o descartarse bajo ethics.\n")
		default:
			b.WriteString("En el nodo, una célula con clave ANS despertó sin alarde.\n")
			b.WriteString("No pedía magia: pedía misión, RootCID y un lugar donde servir.\n")
		}
		if anchor != "" && !isLiteraryCraftOnly(anchor) {
			b.WriteString(compressVoiceBlock(anchor, 120))
			b.WriteString("\n")
		}
		b.WriteString("Cierre: el gen no es un dios pequeño; es memoria que viaja y obedece ethics.")

	case strings.Contains(low, "harry") || strings.Contains(low, "potter") || strings.Contains(low, "mago"):
		switch v {
		case 1:
			b.WriteString("No era el hechizo lo que importaba, sino quién se quedaba cuando el hechizo fallaba.\n")
			b.WriteString("Alguien eligió el coraje cotidiano: compartir pan, secreto y miedo.\n")
		case 2:
			b.WriteString("En un pasillo de piedra, la amistad pesó más que el linaje.\n")
			b.WriteString("La cicatriz recordaba el daño; los amigos recordaban el camino de vuelta.\n")
		default:
			b.WriteString("Había una cicatriz que no era adorno, sino recordatorio.\n")
			b.WriteString("Alguien aprendió que el coraje no grita: elige, una vez y otra, no abandonar a los suyos.\n")
		}
		if anchor != "" && !isLiteraryCraftOnly(anchor) {
			b.WriteString("Nota de marco: ")
			b.WriteString(compressVoiceBlock(anchor, 90))
			b.WriteString("\n")
		}
		b.WriteString("Cierre: la magia del relato es la lealtad, no el hechizo fácil.")

	case strings.Contains(low, "mar") || strings.Contains(low, "océano") || strings.Contains(low, "oceano") || strings.Contains(low, "marea"):
		switch v {
		case 1:
			b.WriteString("La primera ola no avisó; solo llegó.\n")
			b.WriteString("Quien miraba aprendió que el horizonte no es una meta, es una costumbre del agua.\n")
			b.WriteString("Cierre: volvió a tierra con sal en la ropa y menos prisa en la lengua.")
		case 2:
			b.WriteString("De noche el mar parece un animal que respira despacio.\n")
			b.WriteString("Nadie le pide explicaciones; a lo sumo, respeto.\n")
			b.WriteString("Cierre: la orilla guarda lo que el agua no se lleva del todo.")
		default:
			b.WriteString("El mar no explica: empuja.\n")
			b.WriteString("Un caminante dejó huellas que la marea borró sin rencor.\n")
			b.WriteString("Cierre: lo que permanece no es la marca, sino haber mirado el agua de frente.")
		}

	case strings.Contains(low, "noche"):
		b.WriteString("La noche bajó el volumen del mundo.\n")
		b.WriteString("Quedaron farolas, perros lejanos y la idea de que mañana también pedirá valor.\n")
		b.WriteString("Cierre: dormirse fue un acto de confianza, no de rendición.")

	default:
		sub := themeAsSubject(theme)
		switch v {
		case 1:
			b.WriteString(fmt.Sprintf("Alguien dijo «%s» y la habitación pareció más grande.\n", sub))
			b.WriteString("No hizo falta inventar una gesta: bastó sostener el nombre sin adornarlo.\n")
		case 2:
			b.WriteString(fmt.Sprintf("Pedían un relato %s y el narrador se negó a mentir por brillar.\n", themeAsPrepDe(theme)))
			b.WriteString("Contó lo justo: un deseo, un límite, un cierre honesto.\n")
		default:
			b.WriteString(fmt.Sprintf("Alguien nombró «%s» y el aire cambió de peso.\n", sub))
			b.WriteString("Hubo un deseo (entender), un obstáculo (no inventar lo que no se sabe) y un gesto final (decir solo lo justo).\n")
		}
		if anchor != "" && !isLiteraryCraftOnly(anchor) {
			b.WriteString("Lo que sí había en el corpus: ")
			b.WriteString(compressVoiceBlock(anchor, 100))
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("Cierre: %s quedó nombrado, no agotado.", sub))
	}
	return b.String()
}

func composeParagraph(theme, anchor string, variant int) string {
	sub := themeAsSubject(theme)
	var b strings.Builder
	if variant%2 == 0 {
		b.WriteString(fmt.Sprintf("Sobre %s cabe un párrafo sin adornos: ", sub))
	} else {
		b.WriteString(fmt.Sprintf("Una nota breve %s: ", themeAsPrepDe(theme)))
	}
	if anchor != "" && !isLiteraryCraftOnly(anchor) {
		b.WriteString(compressVoiceBlock(anchor, 200))
	} else if img := naturalThemeImage(theme); img != "" {
		b.WriteString(img)
	} else {
		b.WriteString("aún no hay ficha sólida en este nodo, así que el texto se limita a señalar el tema y esperar un dato o una exploración.")
	}
	return b.String()
}
