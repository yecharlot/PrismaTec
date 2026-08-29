package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	creativeVarMu    sync.Mutex
	creativeVarCount = map[string]int{}
)

type creativeIndexEntry struct {
	Theme  string `json:"theme"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	When   string `json:"when"`
	Device string `json:"device,omitempty"`
}

const creativeIndexFile = "mind_creativity_index.json"
const creativeIndexMax = 80

var creativeIndexMu sync.Mutex

func storeCreativeWork(theme, kind, text, device string) {
	theme = strings.TrimSpace(strings.ToLower(theme))
	text = strings.TrimSpace(text)
	if theme == "" || len(text) < 20 {
		return
	}
	_ = os.MkdirAll("alset_data", 0o755)
	path := filepath.Join("alset_data", creativeIndexFile)
	creativeIndexMu.Lock()
	defer creativeIndexMu.Unlock()
	var entries []creativeIndexEntry
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &entries)
	}
	entries = append(entries, creativeIndexEntry{
		Theme: theme, Kind: kind, Text: text,
		When: time.Now().UTC().Format(time.RFC3339), Device: device,
	})
	if len(entries) > creativeIndexMax {
		entries = entries[len(entries)-creativeIndexMax:]
	}
	if raw, err := json.MarshalIndent(entries, "", "  "); err == nil {
		_ = os.WriteFile(path, raw, 0o644)
	}
}

func recallCreativeWork(theme, kind string) string {
	theme = strings.TrimSpace(strings.ToLower(theme))
	if theme == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join("alset_data", creativeIndexFile))
	if err != nil {
		return ""
	}
	var entries []creativeIndexEntry
	if json.Unmarshal(raw, &entries) != nil {
		return ""
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if kind != "" && e.Kind != kind {
			continue
		}
		if e.Theme == theme || strings.Contains(e.Theme, theme) || strings.Contains(theme, e.Theme) {
			return e.Text
		}
	}
	return ""
}

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

// themeAsPrepA: "al mar", "a la noche" — evita "a el mar" / "a del mar".
func themeAsPrepA(theme string) string {
	sub := themeAsSubject(theme)
	low := strings.ToLower(sub)
	if strings.HasPrefix(low, "el ") {
		return "al " + strings.TrimSpace(sub[3:])
	}
	if strings.HasPrefix(low, "la ") {
		return "a la " + strings.TrimSpace(sub[3:])
	}
	if strings.HasPrefix(low, "los ") {
		return "a los " + strings.TrimSpace(sub[4:])
	}
	if strings.HasPrefix(low, "las ") {
		return "a las " + strings.TrimSpace(sub[4:])
	}
	return "a " + sub
}

// themeLooksPlural — sujetos compuestos ("gen y zyrion").
func themeLooksPlural(theme string) bool {
	low := strings.ToLower(theme)
	return strings.Contains(low, " y ") || strings.Contains(low, " e ") ||
		strings.HasPrefix(low, "los ") || strings.HasPrefix(low, "las ")
}

func mindComposeCreative(userText string, ethicsState int, memSpeak, knowSpeak, reasonAnchor string) string {
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
	if ra := strings.TrimSpace(reasonAnchor); ra != "" && !isBadCreativeAnchor(ra) {
		anchor = ra
	}
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
	out := b.String()
	if strings.TrimSpace(reasonAnchor) != "" {
		out = weaveReasonIntoCreative(out, reasonAnchor)
	}
	storeCreativeWork(theme, kind, out, device)
	return out
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
		"car =", "cdr =", "cons =", "defun ", "evaluar-zyrion", ":entradas",
		"(quote", "lispai", "átomos de manipular", "atomos de manipular",
		"```", "func (",
	}
	for _, b := range bad {
		if strings.Contains(low, b) {
			return true
		}
	}
	return false
}

// anchorFitsTheme exige solape real (evita mar→car por edit-distance en corpus).
func anchorFitsTheme(theme, anchor string) bool {
	theme = strings.ToLower(strings.TrimSpace(theme))
	anchor = strings.ToLower(strings.TrimSpace(anchor))
	if theme == "" || anchor == "" {
		return false
	}
	stop := map[string]bool{
		"el": true, "la": true, "los": true, "las": true, "un": true, "una": true,
		"de": true, "del": true, "y": true, "e": true, "o": true, "en": true,
		"que": true, "qué": true, "sobre": true, "para": true, "con": true,
	}
	hit := 0
	for _, w := range strings.Fields(theme) {
		w = strings.Trim(w, ".,;:¡!¿?")
		if len([]rune(w)) < 3 || stop[w] {
			continue
		}
		if strings.Contains(anchor, w) {
			hit++
		}
	}
	return hit > 0
}

func pickCreativeAnchor(theme, memSpeak, knowSpeak string) string {
	theme = strings.TrimSpace(theme)
	try := func(k string) string {
		if k == "" || isBadCreativeAnchor(k) {
			return ""
		}
		// imágenes naturales no necesitan solape de tokens
		if strings.Contains(strings.ToLower(k), "sal en el aire") ||
			strings.Contains(strings.ToLower(k), "cielo opaco") ||
			strings.Contains(strings.ToLower(k), "golpes menudos") ||
			strings.Contains(strings.ToLower(k), "vínculo, cuidado") {
			return compressVoiceBlock(k, 180)
		}
		if !anchorFitsTheme(theme, k) {
			return ""
		}
		return compressVoiceBlock(k, 180)
	}
	// 1) imagen sensorial primero (evita corpus técnico ruidoso tipo car/cdr)
	if img := naturalThemeImage(theme); img != "" {
		return img
	}
	if theme != "" {
		if k := try(speakFromKnowledge(theme)); k != "" {
			return k
		}
		if k := try(speakFromKnowledge("qué es " + theme)); k != "" {
			return k
		}
		bare := strings.ToLower(theme)
		for _, p := range []string{"el ", "la ", "los ", "las ", "un ", "una "} {
			bare = strings.TrimPrefix(bare, p)
		}
		bare = strings.TrimSpace(bare)
		if bare != "" {
			if k := try(speakFromKnowledge(bare)); k != "" {
				return k
			}
		}
		if _, prev, ok := recallScoutFinding(theme); ok && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) {
			if k := try(prev); k != "" {
				return k
			}
		}
		if bare != "" {
			if _, prev, ok := recallScoutFinding(bare); ok && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) {
				if k := try(prev); k != "" {
					return k
				}
			}
		}
	}
	if k := try(knowSpeak); k != "" {
		return k
	}
	if k := try(memSpeak); k != "" {
		return k
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
	devices := []string{
		"metáfora", "símil", "anáfora", "verso libre", "haiku", "personificación",
		"aliteración", "hipérbole", "paradoja", "sinestesia", "paralelismo",
		"interrogación retórica", "apóstrofe", "gradación",
	}
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
	aPrep := themeAsPrepA(theme)
	dev := strings.ToLower(device)
	// 8 moldes por recurso → ciclo largo antes de repetir
	v := ((variant - 1) % 8)

	var lines []string
	switch {
	case strings.Contains(dev, "haiku"):
		molds := [][]string{
			{haikuLine(sub, 5), "un corte de silencio", "eco que no explica"},
			{"ola que no llega", fmt.Sprintf("%s en la orilla", truncateRunes(sub, 18)), "sal en la lengua"},
			{"horizonte bajo", "el agua cuenta sin prisa", "nadie responde"},
			{"piedra y espuma", fmt.Sprintf("nombre: %s", truncateRunes(sub, 12)), "el resto es bruma"},
			{"viento en la tapa", "del cuaderno abierto", "tinta que no seca"},
			{"tres gotas, un reloj", "la orilla no pregunta", "el verso sí"},
			{"luz de media tarde", fmt.Sprintf("%s sin marco", truncateRunes(sub, 14)), "sombra que se queda"},
			{"un barco pequeño", "carga solo preguntas", "no trae respuestas"},
		}
		lines = molds[v]
	case strings.Contains(dev, "anáfora") || strings.Contains(dev, "anafora"):
		molds := [][]string{
			{
				fmt.Sprintf("No es solo %s lo que llega a la boca,", sub),
				fmt.Sprintf("no es solo %s lo que cabe en el pecho,", sub),
				fmt.Sprintf("no es solo %s: es el modo de mirar y no soltar.", sub),
			},
			{
				fmt.Sprintf("Si %s fuera solo nombre, bastaría el diccionario,", sub),
				fmt.Sprintf("si %s fuera solo ruido, bastaría taparse los oídos,", sub),
				fmt.Sprintf("pero %s pide mirada — y la mirada no se improvisa.", sub),
			},
			{
				fmt.Sprintf("Vuelvo %s sin permiso,", aPrep),
				fmt.Sprintf("vuelvo %s con menos prisa,", aPrep),
				fmt.Sprintf("vuelvo %s y el verso aprende a callar.", aPrep),
			},
			{
				fmt.Sprintf("Digo %s y el aire cambia de peso,", sub),
				fmt.Sprintf("digo %s y el reloj pierde un segundo,", sub),
				fmt.Sprintf("digo %s y aún así no lo agoto.", sub),
			},
			{
				fmt.Sprintf("Antes %s era un rumor lejano,", prep),
				fmt.Sprintf("antes %s era una foto en la pared,", prep),
				fmt.Sprintf("ahora %s pide sitio en la mesa.", sub),
			},
			{
				fmt.Sprintf("Ni el mapa explica %s,", sub),
				fmt.Sprintf("ni el diccionario cierra %s,", sub),
				fmt.Sprintf("ni el silencio niega %s.", sub),
			},
			{
				fmt.Sprintf("Una vez %s,", sub),
				fmt.Sprintf("otra vez %s,", sub),
				fmt.Sprintf("y de nuevo %s: el verso no se cansa de intentar.", sub),
			},
			{
				fmt.Sprintf("Por %s dejo una silla vacía,", prep),
				fmt.Sprintf("por %s enciendo una luz baja,", prep),
				fmt.Sprintf("por %s escribo despacio, sin prisa de acabar.", prep),
			},
		}
		lines = molds[v]
	case strings.Contains(dev, "símil") || strings.Contains(dev, "simil"):
		molds := [][]string{
			{
				fmt.Sprintf("%s %s como marea en calma,", capitalizeFirst(sub), map[bool]string{true: "avanzan", false: "avanza"}[themeLooksPlural(theme)]),
				"deja marcas en la arena del oído,",
				"y al retirarse, deja una forma que se puede leer despacio.",
			},
			{
				fmt.Sprintf("%s se parece a una carta sin sobre,", capitalizeFirst(sub)),
				"cualquiera puede leer el borde,",
				"pero el centro solo se entiende en voz baja.",
			},
			{
				fmt.Sprintf("Hablar %s es como abrir una ventana en invierno:", prep),
				"entra aire, entra claridad,",
				"y también un poco de temblor necesario.",
			},
			{
				fmt.Sprintf("%s es como un faro sin isla:", capitalizeFirst(sub)),
				"alumbra, pero no promete puerto,",
				"y aun así orienta a quien navega de noche.",
			},
			{
				fmt.Sprintf("Pensar %s es como sostener agua en las manos:", prep),
				"algo se queda, algo se escapa,",
				"y lo que queda basta para mojar la sed.",
			},
			{
				fmt.Sprintf("%s se parece a un camino sin carteles,", capitalizeFirst(sub)),
				"cada paso inventa la siguiente curva,",
				"y el final no es un punto: es otra curva.",
			},
			{
				fmt.Sprintf("Mirar %s es como oír una canción en otro idioma:", prep),
				"no captas cada palabra,",
				"pero el tono te dice si puedes quedarte.",
			},
			{
				fmt.Sprintf("%s es como la sombra al mediodía:", capitalizeFirst(sub)),
				"casi no se ve, y sin embargo marca el cuerpo,",
				"recuerda que hay sol aunque no lo nombres.",
			},
		}
		lines = molds[v]
	case strings.Contains(dev, "personificación"):
		molds := [][]string{
			{
				fmt.Sprintf("%s toca a la puerta del verso,", capitalizeFirst(sub)),
				"no pide entrada: espera a que le cedamos el umbral,",
				"y cuando lo hacemos, acomoda el silencio.",
			},
			{
				fmt.Sprintf("%s camina descalzo por la frase,", capitalizeFirst(sub)),
				"deja huellas que el lector completa,",
				"y al final se sienta donde nadie lo invitó.",
			},
			{
				fmt.Sprintf("%s se sienta al borde de la frase,", capitalizeFirst(sub)),
				"pregunta sin preguntar, espera sin reloj,",
				"y cuando nos callamos, sigue escuchando.",
			},
			{
				fmt.Sprintf("%s abre el cuaderno por la mitad,", capitalizeFirst(sub)),
				"tacha una palabra que sobraba,",
				"y deja el margen más ancho de lo justo.",
			},
			{
				fmt.Sprintf("%s no grita: susurra al oído del papel,", capitalizeFirst(sub)),
				"pide una línea más y luego otra,",
				"hasta que el blanco se convence de ceder.",
			},
			{
				fmt.Sprintf("%s mira el reloj y no lo cree,", capitalizeFirst(sub)),
				"dice que el tiempo aquí se mide en versos,",
				"no en minutos que se van sin decir adiós.",
			},
			{
				fmt.Sprintf("%s recoge las palabras caídas,", capitalizeFirst(sub)),
				"las ordena como quien guarda semillas,",
				"y espera a que alguna germine en la página.",
			},
			{
				fmt.Sprintf("%s se asoma al borde del poema,", capitalizeFirst(sub)),
				"duda un segundo, luego entra,",
				"y deja la puerta entreabierta por si vuelves.",
			},
		}
		lines = molds[v]
	case strings.Contains(dev, "metáfora"):
		molds := [][]string{
			{
				fmt.Sprintf("%s no es un cartel: es una puerta entreabierta,", capitalizeFirst(sub)),
				"cruzarla pide ritmo, no inventario,",
				"tres pasos: ver, nombrar, callar a tiempo.",
			},
			{
				fmt.Sprintf("%s es una llave que no abre todas las puertas,", capitalizeFirst(sub)),
				"solo aquellas donde alguien dejó una luz encendida,",
				"y aun así hay que girarla con cuidado.",
			},
			{
				fmt.Sprintf("Llamar %s no es etiquetar un objeto,", prep),
				"es aceptar que algo nos atraviesa",
				"y sigue siendo más ancho que el nombre.",
			},
			{
				fmt.Sprintf("%s es un puente de madera vieja,", capitalizeFirst(sub)),
				"cruje, pero aguanta el paso,",
				"y al otro lado el paisaje no es el mismo.",
			},
			{
				fmt.Sprintf("%s es un vaso a medio llenar de lluvia,", capitalizeFirst(sub)),
				"no sirve para brindar ni para regar del todo,",
				"sirve para recordar que el cielo también cae.",
			},
			{
				fmt.Sprintf("%s es el pliegue de un mapa gastado,", capitalizeFirst(sub)),
				"donde las rutas se cruzan sin permiso,",
				"y aun así alguien llega a casa.",
			},
			{
				fmt.Sprintf("%s es una habitación con la ventana abierta,", capitalizeFirst(sub)),
				"entra el ruido de la calle y también el aire,",
				"y uno decide qué dejar pasar.",
			},
			{
				fmt.Sprintf("%s es el hilo que sobra al terminar el tejido,", capitalizeFirst(sub)),
				"no decora, no sostiene,",
				"pero sin él no sabrías dónde cortar.",
			},
		}
		lines = molds[v]
	case strings.Contains(dev, "aliteración") || strings.Contains(dev, "aliteracion"):
		molds := [][]string{
			{
				fmt.Sprintf("Pasa %s, pasa el peso, pasa la pausa,", sub),
				"poco a poco el poema pone orden,",
				"sin prisa, sin ruido de más.",
			},
			{
				fmt.Sprintf("Suenan sílabas suaves %s", prep),
				"sal, sombra, silencio que se sostiene,",
				"y el oído elige qué guardar.",
			},
			{
				fmt.Sprintf("Lento, limpio, lejos %s", prep),
				"la lengua deja letras en el aire,",
				"y el verso las recoge una a una.",
			},
			{
				fmt.Sprintf("Murmullo %s, memoria muda,", prep),
				"marca el margen con una mano quieta,",
				"mientras el otro oído escucha el fondo.",
			},
			{
				fmt.Sprintf("Cae, crece, cierra %s", prep),
				"ciclo corto de cuatro sílabas,",
				"y otra vez el blanco pide turno.",
			},
			{
				fmt.Sprintf("Viento, verso, viaje %s", prep),
				"tres voces que no se empujan,",
				"comparten el mismo aliento breve.",
			},
			{
				fmt.Sprintf("Dura, densa, distinta %s", prep),
				"la palabra se apoya en la siguiente,",
				"sin tropiezo, sin adorno de más.",
			},
			{
				fmt.Sprintf("Ronda el ritmo %s", prep),
				"rueda baja, casi no se oye,",
				"y aun así empuja el poema adelante.",
			},
		}
		lines = molds[v]
	case strings.Contains(dev, "hipérbole") || strings.Contains(dev, "hiperbole"):
		molds := [][]string{
			{
				fmt.Sprintf("Cabe el mundo entero en un trazo %s,", prep),
				"y aun así sobra margen para el silencio,",
				"que también pesa como un continente.",
			},
			{
				fmt.Sprintf("%s cabría en un grano de arena", capitalizeFirst(sub)),
				"y a la vez no cabe en ninguna biblioteca,",
				"porque el tamaño aquí lo decide el asombro.",
			},
			{
				fmt.Sprintf("Mil puertas se abren al nombrar %s,", sub),
				"y detrás de cada una hay otra más ancha,",
				"hasta que el mapa se rinde y pide un verso.",
			},
			{
				fmt.Sprintf("Un solo segundo %s dura una vida,", prep),
				"o una vida se comprime en un segundo,",
				"según mire quien sostiene el lápiz.",
			},
			{
				fmt.Sprintf("%s llena la habitación sin entrar,", capitalizeFirst(sub)),
				"empuja las paredes un centímetro,",
				"y el techo aprende a ser cielo un rato.",
			},
			{
				fmt.Sprintf("Quepa o no, %s se queda,", sub),
				"ocupa el asiento de al lado,",
				"y el resto del día habla en voz baja de eso.",
			},
			{
				fmt.Sprintf("Más ancho que el mapa, más corto que un suspiro: %s", capitalizeFirst(sub)),
				"así mide quien no usa regla,",
				"solo el pulso y una línea en blanco.",
			},
			{
				fmt.Sprintf("Toda la noche cabe en una sílaba %s,", prep),
				"si la dices despacio,",
				"y al amanecer aún resuena en la mesa.",
			},
		}
		lines = molds[v]
	default: // verso libre
		molds := [][]string{
			{
				fmt.Sprintf("Miro %s sin pretender agotarlo,", sub),
				"anoto tres imágenes y suelto la cuarta,",
				"porque el poema también se mide por lo que omite.",
			},
			{
				fmt.Sprintf("Bajo el nombre %s hay agua y hay piedra,", prep),
				"el verso elige qué tocar primero,",
				"y deja el resto al silencio que también es parte.",
			},
			{
				fmt.Sprintf("Hoy %s no pide adorno,", sub),
				"pide una línea limpia y un margen en blanco,",
				"donde quepa lo que aún no sabemos decir.",
			},
			{
				fmt.Sprintf("Entre %s y la página hay un paso,", sub),
				"no siempre se da,",
				"pero cuando se da, el suelo cambia de textura.",
			},
			{
				fmt.Sprintf("No traigo definiciones %s:", prep),
				"traigo una mesa, una silla y tiempo,",
				"para que el tema se siente si quiere.",
			},
			{
				fmt.Sprintf("Escribo cerca %s, no encima,", prep),
				"dejo hueco por si algo quiere crecer,",
				"y no firmo hasta que el silencio asienta.",
			},
			{
				fmt.Sprintf("Lo que sé %s cabe en tres líneas;", prep),
				"lo que no sé llena el resto del cuaderno,",
				"y por eso el poema no termina del todo.",
			},
			{
				fmt.Sprintf("Una lista breve %s:", prep),
				"luz, peso, distancia, una pregunta,",
				"y al final un punto que no cierra nada.",
			},
		}
		lines = molds[v]
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

	case strings.Contains(low, "gen") || strings.Contains(low, "sonda"):
		switch v {
		case 1:
			b.WriteString("Una semilla con nombre ANS cruzó el borde sin pedir aplauso.\n")
			b.WriteString("No conquistaba: observaba, registraba un hallazgo y volvía con la voz justa.\n")
			b.WriteString("Cierre: el gen no era héroe; era memoria que aprendió a viajar.")
		case 2:
			b.WriteString("En la frontera no había bandera, solo paquetes y preguntas.\n")
			b.WriteString("La sonda tomó nota, selló un CID y no invadió lo que no le tocaba.\n")
			b.WriteString("Cierre: explorar no es poseer.")
		default:
			b.WriteString("Alguien soltó una célula digital al torrente de la red.\n")
			b.WriteString("Volvió más liviana de ego y más pesada de hechos.\n")
			b.WriteString("Cierre: la misión cabía en una clave, no en un imperio.")
		}

	case strings.Contains(low, "nodo") || strings.Contains(low, "mind"):
		b.WriteString("El nodo no dormía: escuchaba HTTP y contaba órganos.\n")
		b.WriteString("Cada latido era un juicio en tres tonos, no un sueño de tokens.\n")
		b.WriteString("Cierre: permanecer despierto era su forma de cuidar el campo.")

	case strings.Contains(low, "memoria") || strings.Contains(low, "cid"):
		b.WriteString("Guardaron una frase bajo un hash y el tiempo ya no pudo negar que había sido dicha.\n")
		b.WriteString("No era nostalgia: era prueba.\n")
		b.WriteString("Cierre: recordar con CID es prometer no reescribir el pasado a conveniencia.")

	case strings.Contains(low, "zyrion") || strings.Contains(low, "ternar"):
		b.WriteString("Tres caminos: seguir, matizar o sumidero.\n")
		b.WriteString("El dos no negociaba promedios; el cero no fingía alarma.\n")
		b.WriteString("Cierre: decidir con tres tonos es más pobre en adorno y más rico en honestidad.")

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
