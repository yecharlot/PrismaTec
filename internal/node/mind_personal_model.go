package node

import (
	"strings"
)

// UserProfile — modelo personal estructurado (no LLM).
// Se construye desde episodios CID: slots + afirmaciones "yo soy / yo no soy".
// Sirve para responder "quién soy / qué soy / mis datos" sin parches sueltos.

type UserProfile struct {
	Nombre    string
	Apellidos string
	Soy       []string // afirmaciones positivas ("un hombre")
	NoSoy     []string // negaciones ("Socrates")
	Otros     []string // otros hechos personales cortos
}

func buildUserProfile(episodes []mindEpisodePayload) UserProfile {
	var p UserProfile
	seenSoy := map[string]bool{}
	seenNo := map[string]bool{}

	// Recorrer de más reciente a más antiguo si el slice viene reciente-primero;
	// si no, el último valor no vacío gana en nombre/apellido.
	for _, ep := range episodes {
		t := strings.TrimSpace(ep.Text)
		if t == "" {
			continue
		}
		low := strings.ToLower(t)

		if name := extractDeclaredName(t); name != "" {
			if p.Nombre == "" {
				p.Nombre = name
			}
		}
		if slot, val := extractPersonalDeclaration(t); slot == "apellido" && val != "" {
			if p.Apellidos == "" {
				p.Apellidos = val
			}
		}
		// "Te llamas X" en respuestas previas
		for _, pref := range []string{"te llamas ", "perfecto, te llamas "} {
			if i := strings.Index(low, pref); i >= 0 && p.Nombre == "" {
				rest := strings.TrimSpace(t[i+len(pref):])
				for _, cut := range []string{".", ",", "!", "?"} {
					if j := strings.Index(rest, cut); j > 0 {
						rest = rest[:j]
					}
				}
				if n := strings.TrimSpace(rest); len(n) >= 2 {
					p.Nombre = strings.Fields(n)[0]
					if len(strings.Fields(n)) > 1 {
						p.Nombre = strings.Join(strings.Fields(n)[:min(2, len(strings.Fields(n)))], " ")
					}
				}
			}
		}

		// yo soy / soy / yo no soy
		if strings.HasPrefix(low, "yo no soy ") {
			v := strings.TrimSpace(t[len("yo no soy "):])
			v = strings.Trim(v, ".!")
			if v != "" && !seenNo[strings.ToLower(v)] {
				seenNo[strings.ToLower(v)] = true
				p.NoSoy = append(p.NoSoy, v)
			}
			continue
		}
		if strings.HasPrefix(low, "yo soy ") {
			v := strings.TrimSpace(t[len("yo soy "):])
			v = strings.Trim(v, ".!")
			if v != "" && !seenSoy[strings.ToLower(v)] {
				seenSoy[strings.ToLower(v)] = true
				p.Soy = append(p.Soy, v)
			}
			continue
		}
		if strings.HasPrefix(low, "soy ") && !strings.HasPrefix(low, "soy alset") {
			v := strings.TrimSpace(t[len("soy "):])
			v = strings.Trim(v, ".!")
			if v != "" && len([]rune(v)) < 40 && !seenSoy[strings.ToLower(v)] {
				seenSoy[strings.ToLower(v)] = true
				p.Soy = append(p.Soy, v)
			}
		}
	}

	// knownUserName fallback
	if p.Nombre == "" {
		p.Nombre = knownUserNameFromEpisodes(episodes)
	}
	return p
}

func (p UserProfile) empty() bool {
	return p.Nombre == "" && p.Apellidos == "" && len(p.Soy) == 0 && len(p.NoSoy) == 0
}

// isSelfModelQuery: preguntas sobre la identidad del usuario (no de Mind).
func isSelfModelQuery(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if isAskingMindName(s) || isIdentityTalk(s) {
		// "quién eres" es Mind; "quién soy" es usuario
		if strings.Contains(s, "quién eres") || strings.Contains(s, "quien eres") ||
			strings.Contains(s, "qué eres") || strings.Contains(s, "que eres") {
			return false
		}
	}
	keys := []string{
		"qué soy yo", "que soy yo", "quién soy yo", "quien soy yo",
		"quién soy", "quien soy", "qué soy", "que soy",
		"qué sabes de mí", "que sabes de mi", "qué sabes de mi",
		"quién soy según tú", "quien soy segun tu",
		"descríbeme", "describeme", "cómo me ves", "como me ves",
	}
	for _, k := range keys {
		if s == k || strings.HasPrefix(s, k) {
			return true
		}
	}
	return false
}

// speakFromProfile sintetiza una respuesta de diálogo desde el modelo personal.
func speakFromProfile(query string, p UserProfile) string {
	if p.empty() {
		return "Aún no tengo un perfil tuyo anclado. Dime tu nombre, apellidos o hechos («yo soy…») y los guardo."
	}
	q := strings.ToLower(strings.TrimSpace(query))

	// Solo apellidos
	if strings.Contains(q, "apellido") && !isSelfModelQuery(q) {
		if p.Apellidos != "" {
			return "Tus apellidos son " + p.Apellidos + "."
		}
		return "Aún no tengo tus apellidos. Dímelos con «mis apellidos son…»."
	}
	// Solo nombre
	if (strings.Contains(q, "nombre") || strings.Contains(q, "llamo")) && !isSelfModelQuery(q) {
		if p.Nombre != "" {
			return "Te llamas " + p.Nombre + "."
		}
		return "Aún no tengo tu nombre. Dilo con «me llamo…»."
	}

	// Perfil completo / qué soy yo
	var parts []string
	if p.Nombre != "" && p.Apellidos != "" {
		parts = append(parts, "te llamas "+p.Nombre+" "+p.Apellidos)
	} else if p.Nombre != "" {
		parts = append(parts, "te llamas "+p.Nombre)
	} else if p.Apellidos != "" {
		parts = append(parts, "tus apellidos son "+p.Apellidos)
	}
	if len(p.Soy) > 0 {
		parts = append(parts, "me dijiste que eres "+joinNatural(p.Soy))
	}
	if len(p.NoSoy) > 0 {
		parts = append(parts, "y que no eres "+joinNatural(p.NoSoy))
	}
	if len(parts) == 0 {
		return "Tengo poco anclado sobre ti. Añade un hecho claro y lo integro."
	}
	return "Por lo que me confiaste: " + strings.Join(parts, "; ") + "."
}

func joinNatural(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " y " + items[len(items)-1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
