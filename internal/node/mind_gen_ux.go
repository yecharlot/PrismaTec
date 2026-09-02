package node

import (
	"fmt"
	"strings"
)

// P4+ — comunicación humana Mind↔Gen: el usuario habla natural; el lab se traduce por debajo.

func normalizeGenUserIntent(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return text
	}
	lab := []string{"crea gen", "crear gen", "lista gen", "elimina gen", "retorna gen",
		"despacha gen", "salva en gen", "pregunta al gen", "lista genes"}
	for _, k := range lab {
		if strings.HasPrefix(low, k) || strings.Contains(low, k+" ") {
			// ya es comando de lab; dejar (salvo que sea frase mixta)
			if !strings.Contains(low, "sonda llamada") && !strings.Contains(low, "una sonda") {
				return text
			}
		}
	}

	// listar
	if matchAny(low, []string{
		"qué genes", "que genes", "cuáles genes", "cuales genes", "lista de genes",
		"muestra los genes", "qué sondas", "que sondas", "mis sondas", "mis genes",
		"sondas activas", "genes activos", "qué sondas tienes", "que sondas tienes",
		"enséñame los genes", "ensename los genes", "ver genes", "ver sondas",
	}) || (strings.Contains(low, "genes") && matchAny(low, []string{"tienes", "hay", "listar", "lista"})) {
		return "lista genes"
	}

	// crear
	if name := extractHumanGenName(low, []string{
		"crea una sonda llamada ", "crea sonda llamada ", "crea la sonda ",
		"crea un gen llamado ", "crea gen llamado ", "crear gen ",
		"haz un gen llamado ", "haz una sonda llamada ", "nueva sonda ",
		"nuevo gen ", "crea gen ", "crear el gen ",
	}); name != "" {
		if strings.Contains(low, "memoria") || strings.Contains(low, "para guardar") {
			return "crea gen memoria " + name
		}
		return "crea gen " + name
	}
	if matchAny(low, []string{"crea una sonda", "crear una sonda", "haz una sonda", "nueva sonda"}) &&
		!strings.Contains(low, "función") && !strings.Contains(low, "funcion") {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "sonda"
		}
		if strings.Contains(low, "memoria") {
			return "crea gen memoria " + name
		}
		return "crea gen " + name
	}

	// explorar / buscar
	if matchAny(low, []string{"explora", "explorar", "busca en", "buscar en", "investiga", "averigua",
		"mira en internet", "en la web", "en internet"}) &&
		(matchAny(low, []string{"gen", "sonda", "sonda a", "manda", "envía", "envia"}) ||
			strings.Contains(low, "http")) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "scout"
		}
		topic := extractExploreTopic(low)
		if topic != "" {
			return "envia al gen " + name + " a explorar " + topic
		}
		return "gen " + name + " explora"
	}

	// despachar borde
	if matchAny(low, []string{"cloudflare", "al borde", "al edge", "red de borde", "fuera del nodo",
		"a la frontera", "envíala al borde", "enviala al borde", "mándala al borde", "mandala al borde"}) &&
		matchAny(low, []string{"gen", "sonda"}) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "sonda"
		}
		return "despacha gen " + name + " a cloudflare"
	}
	if matchAny(low, []string{"manda la sonda", "envía la sonda", "envia la sonda", "manda el gen", "envía el gen"}) &&
		matchAny(low, []string{"cloudflare", "borde", "edge", "fuera"}) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "sonda"
		}
		return "despacha gen " + name + " a cloudflare"
	}

	// retornar
	if matchAny(low, []string{"trae", "devuelve", "recupera", "retorna", "haz volver", "vuelve a casa",
		"trae de vuelta", "trae la sonda", "devuelve la sonda", "recupera la sonda", "del borde", "en el borde"}) &&
		(matchAny(low, []string{"gen", "sonda", "borde", "cloudflare"}) || strings.Contains(low, "de vuelta")) {
		name := extractGenNameFromText(low)
		if name == "" && (strings.Contains(low, "borde") || strings.Contains(low, "cloudflare")) {
			return "retorna gen" // mindGenTools resolverá la del borde
		}
		if name == "" {
			return "retorna gen"
		}
		return "retorna gen " + name
	}
	// «se llama X» como continuación de traer sonda
	if strings.HasPrefix(low, "se llama ") {
		fields := strings.Fields(low)
		if len(fields) >= 3 {
			return "retorna gen " + fields[2]
		}
	}

	// eliminar
	if matchAny(low, []string{"elimina", "borra", "quita", "destruye", "deshaz"}) &&
		matchAny(low, []string{"gen", "sonda"}) {
		name := extractGenNameFromText(low)
		if name == "" {
			return "elimina gen"
		}
		return "elimina gen " + name
	}

	// preguntar
	if matchAny(low, []string{"pregúntale", "preguntale", "pregunta al", "dile a", "di al",
		"habla con", "pregunta a la sonda", "dile a la sonda"}) &&
		matchAny(low, []string{"gen", "sonda"}) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "sonda"
		}
		rest := extractAfterGenTalk(text, low, name)
		if rest != "" {
			return "pregunta al gen " + name + " " + rest
		}
		return "pregunta al gen " + name
	}

	// estado / qué hace / dónde está
	if matchAny(low, []string{
		"qué está haciendo", "que esta haciendo", "qué esta haciendo", "que está haciendo",
		"explícame qué está", "explicame que esta", "qué hace la", "que hace la",
		"estado de la sonda", "estado del gen", "dónde está la sonda", "donde esta la sonda",
		"dónde está el gen", "donde esta el gen",
	}) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = extractTrailingName(low)
		}
		if name != "" {
			return "estado gen " + name
		}
	}

	// hallazgos / qué ve / resultado
	if matchAny(low, []string{
		"dame el resultado", "qué ve", "que ve", "qué vio", "que vio", "ve algo",
		"dime qué ve", "dime que ve", "hallazgos de", "resultado de", "qué encontró", "que encontro",
		"qué encontró", "qué ha visto", "que ha visto",
	}) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = extractTrailingName(low)
		}
		if name != "" {
			return "hallazgos gen " + name
		}
	}

	// elimina a X (sin decir sonda)
	if matchAny(low, []string{"elimina a ", "borra a ", "quita a ", "elimina ", "borra "}) &&
		!matchAny(low, []string{"todos", "todas", "archivo", "cuenta", "disco"}) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = extractTrailingName(low)
		}
		if name != "" && name != "sonda" {
			return "elimina gen " + name
		}
		if strings.Contains(low, "elimina a ") || strings.Contains(low, "borra a ") {
			rest := low
			for _, p := range []string{"elimina a ", "borra a ", "quita a "} {
				if i := strings.Index(low, p); i >= 0 {
					rest = strings.TrimSpace(low[i+len(p):])
					break
				}
			}
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				return "elimina gen " + strings.Trim(fields[0], ".,")
			}
		}
	}

	// trae a X / tráela / traela
	if matchAny(low, []string{"trae a ", "tráela", "traela", "tráelo", "traelo", "devuélvela", "devuelvela",
		"trae de vuelta", "traela de vuelta", "tráela de vuelta"}) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = extractTrailingName(low)
		}
		if name != "" {
			return "retorna gen " + name
		}
		if strings.Contains(low, "borde") {
			return "retorna gen"
		}
		return "retorna gen"
	}

	// guardar
	if matchAny(low, []string{"guarda en", "salva en", "ancla en", "pin en", "guarda esto en", "salva esto en"}) &&
		matchAny(low, []string{"gen", "sonda"}) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "mem-nodo"
		}
		payload := ""
		for _, sep := range []string{":", " texto ", " esto:", " esto ", " que "} {
			if i := strings.Index(low, strings.TrimSpace(sep)); i >= 0 {
				payload = strings.TrimSpace(text[i+len(strings.TrimSpace(sep)):])
				if payload != "" {
					break
				}
			}
		}
		if payload != "" {
			return "salva en gen " + name + " : " + payload
		}
		return "salva en gen " + name
	}

	return text
}

func matchAny(s string, keys []string) bool {
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func extractHumanGenName(low string, prefixes []string) string {
	for _, p := range prefixes {
		if i := strings.Index(low, p); i >= 0 {
			rest := strings.TrimSpace(low[i+len(p):])
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				continue
			}
			name := strings.Trim(fields[0], ".,;:«»\"'")
			if name != "" && name != "memoria" && name != "gen" && name != "sonda" {
				return name
			}
		}
	}
	return ""
}

func extractExploreTopic(low string) string {
	markers := []string{
		"explorar ", "explora ", "buscar ", "busca ", "investiga ", "averigua ",
		"a mirar ", "sobre ", "acerca de ", "http://", "https://",
	}
	for _, m := range markers {
		if i := strings.Index(low, m); i >= 0 {
			rest := strings.TrimSpace(low[i:])
			if strings.HasPrefix(rest, "http") {
				// keep from http
			} else {
				rest = strings.TrimSpace(low[i+len(m):])
			}
			if strings.HasPrefix(m, "http") {
				rest = strings.TrimSpace(low[i:])
			}
			for _, cut := range []string{" con el gen", " con la sonda", " por favor", " please", " y "} {
				if j := strings.Index(rest, cut); j > 0 {
					rest = strings.TrimSpace(rest[:j])
				}
			}
			if rest != "" && rest != "gen" && rest != "sonda" {
				return rest
			}
		}
	}
	// URL anywhere
	if i := strings.Index(low, "https://"); i >= 0 {
		return strings.Fields(low[i:])[0]
	}
	if i := strings.Index(low, "http://"); i >= 0 {
		return strings.Fields(low[i:])[0]
	}
	return ""
}

func extractAfterGenTalk(text, low, name string) string {
	for _, p := range []string{
		"pregúntale al gen ", "preguntale al gen ", "pregunta al gen ",
		"dile al gen ", "di al gen ", "habla con el gen ", "habla con gen ",
		"pregunta a la sonda ", "dile a la sonda ", "habla con la sonda ",
	} {
		if i := strings.Index(low, p); i >= 0 {
			rest := strings.TrimSpace(text[i+len(p):])
			fields := strings.Fields(rest)
			if len(fields) > 0 && strings.EqualFold(strings.Trim(fields[0], ".,"), name) {
				rest = strings.Join(fields[1:], " ")
			}
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func humanizeGenVoice(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	// reescrituras de tono lab → humano
	pairs := []struct{ old, neu string }{
		{"Gen «", "La sonda «"},
		{"gen «", "sonda «"},
		{"Gen memoria «", "La sonda de memoria «"},
		{"eliminado del registro de este nodo.", "quedó eliminada de este nodo."},
		{"de vuelta en el nodo (antes:", "volvió a este nodo (antes estaba en"},
		{"No pude eliminar:", "No pude quitarla:"},
		{"No pude retornar el gen:", "No pude traerla de vuelta:"},
		{"No pude despachar a la red de borde:", "No pude enviarla al borde de la red:"},
		{"No pude preparar el gen memoria:", "No pude preparar la sonda de memoria:"},
		{"No pude salvar en el gen:", "No pude guardar eso en la sonda:"},
		{"Para explorar, dime una URL pública https.", "Para explorar necesito una dirección web pública (https://…)."},
		{"La exploración no pudo completarse:", "No pude completar la exploración:"},
		{"Indica el gen: «elimina gen genesis».", "Dime el nombre de la sonda, por ejemplo: «elimina la sonda genesis»."},
		{"Indica el gen: «retorna gen genesis».", "Dime el nombre de la sonda, por ejemplo: «trae la sonda genesis»."},
		{"No hay genes con misión memoria. Di «crea gen memoria mem-nodo».", "Aún no hay una sonda de memoria. Puedes decir: «crea una sonda de memoria»."},
		{"No hay episodios recientes que vincular. Guarda un hecho primero.", "Aún no hay un recuerdo reciente para guardar. Cuéntame un hecho primero."},
		{"Vínculo falló:", "No pude vincular el recuerdo:"},
		{"Despaché «", "Envié «"},
		{"a la red de borde. Puedes alcanzarlo en ", "al borde de la red. Puedes abrirla en "},
		{"Listo: nació el sonda «", "Listo: creé la sonda «"},
		{"Listo: nació el gen «", "Listo: creé la sonda «"},
		{"nació el sonda", "creé la sonda"},
		{"nació el gen", "creé la sonda"},
		{"Está en este nodo; si quieres lo despacho a la red de borde.", "Está en este nodo. Si quieres, la mando al borde de la red."},
		{"Tengo ", "Ahora mismo tengo "},
		{"gen(es) a la vista:", "sonda(s) a la vista:"},
	}
	out := s
	for _, r := range pairs {
		out = strings.ReplaceAll(out, r.old, r.neu)
	}
	// CID en tono más suave
	if strings.Contains(out, " · CID ") {
		out = strings.ReplaceAll(out, " · CID ", " (referencia ")
		if !strings.HasSuffix(out, ".") && !strings.HasSuffix(out, ")") {
			out += ")"
		}
	}
	return out
}

func extractTrailingName(low string) string {
	skip := map[string]bool{
		"a": true, "la": true, "el": true, "de": true, "del": true, "qué": true, "que": true,
		"está": true, "esta": true, "haciendo": true, "sonda": true, "gen": true, "dame": true,
		"resultado": true, "dime": true, "ve": true, "algo": true, "explicame": true,
		"explícame": true, "elimina": true, "borra": true, "trae": true, "traela": true, "tráela": true,
		"vuelta": true, "ahora": true, "tienes": true, "en": true, "borde": true,
	}
	fields := strings.Fields(low)
	for i := len(fields) - 1; i >= 0; i-- {
		w := strings.Trim(fields[i], ".,;:¿?¡!")
		if w == "" || skip[w] || strings.HasPrefix(w, "http") {
			continue
		}
		if len([]rune(w)) < 2 {
			continue
		}
		return w
	}
	return ""
}

func isHumanGenIntent(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if isGenToolIntent(s) {
		return true
	}
	n := normalizeGenUserIntent(s)
	return n != s && isGenToolIntent(n)
}

// speakGenHelp: explicación humana de la unión Mind–Gen.
func speakGenHelp(s string) string {
	low := strings.ToLower(s)
	if !(strings.Contains(low, "sonda") || strings.Contains(low, "gen")) {
		return ""
	}
	if !(strings.Contains(low, "qué puedo") || strings.Contains(low, "que puedo") ||
		strings.Contains(low, "cómo uso") || strings.Contains(low, "como uso") ||
		strings.Contains(low, "para qué sirven") || strings.Contains(low, "para que sirven") ||
		strings.Contains(low, "qué hago con") || strings.Contains(low, "que hago con") ||
		strings.Contains(low, "ayuda") && (strings.Contains(low, "sonda") || strings.Contains(low, "gen"))) {
		return ""
	}
	return strings.TrimSpace(`Las sondas (genes) son piezas que puedo crear, mandar al borde de la red, poner a explorar una web y traer de vuelta.

Puedes hablarme así, sin jerga:
· «crea una sonda llamada aurora»
· «qué sondas tienes»
· «manda la sonda aurora al borde» o «a cloudflare»
· «que explore https://ejemplo.com»
· «pregúntale a la sonda aurora quién eres»
· «trae la sonda aurora»
· «elimina la sonda aurora»
· «guarda esto en la sonda de memoria: …»

Yo traduzco eso a la red Alset, respeto los límites de ethics y te cuento el resultado en claro.`)
}

// ensureHumanExploreURL soft message already in tools; helper for empty URL topic words
func formatExploreHuman(name, url, snippet string) string {
	msg := fmt.Sprintf("Mandé la sonda «%s» a mirar %s.", normalizeGenKey(name), url)
	if snippet != "" {
		sn := snippet
		if len([]rune(sn)) > 220 {
			sn = string([]rune(sn)[:220]) + "…"
		}
		msg += "\n\nLo que trajo:\n" + sn
	}
	msg += "\n\nSi quieres, pide otro ángulo, que la traiga de vuelta o que guarde un hallazgo."
	return msg
}
