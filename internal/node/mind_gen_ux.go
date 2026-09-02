package node

import (
	"strings"
)

// P4 — órdenes humanas Mind→Gen: el usuario no tiene que memorizar jerga de lab.

// normalizeGenUserIntent: reescribe frases naturales a formas que mindGenTools ya entiende.
// No cambia ética ni APIs; solo el texto de entrada efectivo.
func normalizeGenUserIntent(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return text
	}
	// Ya es jerga de lab: no tocar
	lab := []string{"crea gen", "crear gen", "lista gen", "elimina gen", "retorna gen",
		"despacha", "salva en gen", "pregunta al gen", "explora http", "explora https"}
	for _, k := range lab {
		if strings.Contains(low, k) {
			return text
		}
	}

	// --- listar ---
	if strings.Contains(low, "qué genes") || strings.Contains(low, "que genes") ||
		strings.Contains(low, "cuáles genes") || strings.Contains(low, "cuales genes") ||
		strings.Contains(low, "lista de genes") || strings.Contains(low, "muestra los genes") ||
		strings.Contains(low, "qué sondas") || strings.Contains(low, "que sondas") ||
		(strings.Contains(low, "genes") && (strings.Contains(low, "tienes") || strings.Contains(low, "hay") || strings.Contains(low, "activos"))) {
		return "lista genes"
	}

	// --- crear ---
	if name := extractHumanGenName(low, []string{
		"crea una sonda llamada ", "crea sonda ", "crea el gen ", "crear el gen ",
		"haz un gen ", "haz una sonda ", "nuevo gen ", "nueva sonda ",
		"crea gen ", "crear gen ",
	}); name != "" {
		if strings.Contains(low, "memoria") || strings.Contains(low, "guardar") {
			return "crea gen memoria " + name
		}
		return "crea gen " + name
	}
	if (strings.Contains(low, "crea") || strings.Contains(low, "crear") || strings.Contains(low, "haz ")) &&
		(strings.Contains(low, "sonda") || strings.Contains(low, " gen")) &&
		!strings.Contains(low, "función") && !strings.Contains(low, "funcion") {
		name := extractGenNameFromText(low)
		if name == "" || name == "gen" {
			name = "sonda"
		}
		if strings.Contains(low, "memoria") {
			return "crea gen memoria " + name
		}
		return "crea gen " + name
	}

	// --- explorar / buscar en la web con gen ---
	if (strings.Contains(low, "busca") || strings.Contains(low, "investiga") || strings.Contains(low, "averigua") ||
		strings.Contains(low, "explora") || strings.Contains(low, "mira en internet") || strings.Contains(low, "en la web")) &&
		(strings.Contains(low, "gen") || strings.Contains(low, "sonda") || strings.Contains(low, "con un gen") ||
			strings.Contains(low, "manda a") || strings.Contains(low, "envía a") || strings.Contains(low, "envia a")) {
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
	// "manda una sonda a explorar X" / "envía sonda a mirar X"
	if (strings.Contains(low, "sonda") || strings.Contains(low, "gen")) &&
		(strings.Contains(low, "explorar") || strings.Contains(low, "explora") || strings.Contains(low, "buscar")) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "scout"
		}
		topic := extractExploreTopic(low)
		if topic != "" {
			return "envia al gen " + name + " a explorar " + topic
		}
	}

	// --- despachar a edge / cloudflare ---
	if (strings.Contains(low, "cloudflare") || strings.Contains(low, "al borde") || strings.Contains(low, "al edge") ||
		strings.Contains(low, "fuera del nodo") || strings.Contains(low, "a la red de borde")) &&
		(strings.Contains(low, "gen") || strings.Contains(low, "sonda")) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "sonda"
		}
		return "despacha gen " + name + " a cloudflare"
	}

	// --- traer de vuelta ---
	if (strings.Contains(low, "trae") || strings.Contains(low, "devuelve") || strings.Contains(low, "recupera") ||
		strings.Contains(low, "retorna") || strings.Contains(low, "vuelve")) &&
		(strings.Contains(low, "gen") || strings.Contains(low, "sonda")) {
		name := extractGenNameFromText(low)
		if name == "" {
			return "retorna gen"
		}
		return "retorna gen " + name
	}

	// --- eliminar ---
	if (strings.Contains(low, "elimina") || strings.Contains(low, "borra") || strings.Contains(low, "quita") ||
		strings.Contains(low, "destruye")) &&
		(strings.Contains(low, "gen") || strings.Contains(low, "sonda")) {
		name := extractGenNameFromText(low)
		if name == "" {
			return "elimina gen"
		}
		return "elimina gen " + name
	}

	// --- preguntar / hablar con gen ---
	if (strings.Contains(low, "pregúntale") || strings.Contains(low, "preguntale") ||
		strings.Contains(low, "dile a") || strings.Contains(low, "habla con") ||
		strings.Contains(low, "pregunta al") || strings.Contains(low, "di al gen")) &&
		(strings.Contains(low, "gen") || strings.Contains(low, "sonda")) {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "sonda"
		}
		// payload tras el nombre
		q := low
		for _, p := range []string{"pregúntale al gen ", "preguntale al gen ", "pregunta al gen ", "dile al gen ", "di al gen ", "habla con el gen ", "habla con gen "} {
			if i := strings.Index(q, p); i >= 0 {
				rest := strings.TrimSpace(text[i+len(p):])
				// quitar nombre al inicio del rest si está
				fields := strings.Fields(rest)
				if len(fields) > 0 && strings.Contains(strings.ToLower(fields[0]), strings.ToLower(name)) {
					rest = strings.Join(fields[1:], " ")
				}
				if rest != "" {
					return "pregunta al gen " + name + " " + rest
				}
			}
		}
		return "pregunta al gen " + name
	}

	// --- guardar en gen memoria ---
	if (strings.Contains(low, "guarda") || strings.Contains(low, "salva") || strings.Contains(low, "ancla") ||
		strings.Contains(low, "pin ")) &&
		(strings.Contains(low, "gen") || strings.Contains(low, "sonda")) &&
		!strings.Contains(low, "cloudflare") {
		name := extractGenNameFromText(low)
		if name == "" {
			name = "mem-nodo"
		}
		payload := ""
		for _, sep := range []string{":", " texto ", " esto ", " que "} {
			if i := strings.Index(low, sep); i >= 0 {
				payload = strings.TrimSpace(text[i+len(sep):])
				break
			}
		}
		if payload != "" {
			return "salva en gen " + name + " : " + payload
		}
		return "salva en gen " + name
	}

	return text
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
			if name != "" && name != "memoria" && name != "gen" {
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
			rest := strings.TrimSpace(low[i+len(m):])
			// cortar colas
			for _, cut := range []string{" con ", " y ", " por favor", " please"} {
				if j := strings.Index(rest, cut); j > 0 {
					rest = strings.TrimSpace(rest[:j])
				}
			}
			if rest != "" && rest != "gen" && rest != "sonda" {
				return rest
			}
		}
	}
	return ""
}

// humanizeGenVoice: quita tono de lab cuando la respuesta viene de herramientas gen.
func humanizeGenVoice(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	// ya natural
	if strings.HasPrefix(s, "Listo") || strings.HasPrefix(s, "He ") || strings.HasPrefix(s, "Gen «") {
		// suavizar prefijos duros
	}
	repl := []struct{ old, neu string }{
		{"Gen «", "La sonda «"},
		{"gen «", "sonda «"},
		{"No pude eliminar:", "No pude quitar esa sonda:"},
		{"No pude retornar el gen:", "No pude traer la sonda de vuelta:"},
		{"No pude despachar:", "No pude enviar la sonda al borde:"},
		{"listo. Misión:", "Lista. Sirve para"},
	}
	out := s
	for _, r := range repl {
		out = strings.ReplaceAll(out, r.old, r.neu)
	}
	return out
}

// isHumanGenIntent: frases naturales que deben entrar al pipeline gen.
func isHumanGenIntent(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if isGenToolIntent(s) {
		return true
	}
	// si normalizar cambia el texto, había intención gen
	n := normalizeGenUserIntent(s)
	return n != s && isGenToolIntent(n)
}
