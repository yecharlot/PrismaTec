package node

import (
	"fmt"
	"strings"
)

// Capa de razón ternaria (no LLM): hechos con confianza 0/1/2 y reglas de
// combinación. 0 = no afirmar, 1 = matizar, 2 = afirmar.
// Las reglas se aplican a premisas del usuario, memoria y corpus.

type ternaryFact struct {
	Subj string
	Rel  string // es | es_un | implica | parte_de | tiene | no_es
	Obj  string
	Conf int
	Src  string
}

func zyrionMinConf(a, b int) int {
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	if a > 2 {
		a = 2
	}
	if b > 2 {
		b = 2
	}
	if a < b {
		return a
	}
	return b
}

func normalizeReasonToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, ".,;:!?«»\"'")
	return s
}

func splitReasonClauses(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	low := strings.ToLower(text)
	for _, sep := range []string{"; entonces", ", entonces", " entonces", "; por tanto", " por tanto", "; por lo tanto", " por lo tanto"} {
		if i := strings.Index(low, sep); i > 0 {
			text = strings.TrimSpace(text[:i])
			low = strings.ToLower(text)
		}
	}
	// Prefer " y " splits when both sides look like facts
	var parts []string
	if strings.Count(low, " y ") >= 1 {
		parts = strings.Split(text, " y ")
	} else {
		parts = strings.Split(text, ", ")
	}
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}

// parseRelationFact: X REL Y
func parseRelationFact(text string, conf int, src string) (ternaryFact, bool) {
	low := strings.ToLower(strings.TrimSpace(text))
	if strings.Contains(low, "?") || strings.HasPrefix(low, "qué ") || strings.HasPrefix(low, "que ") ||
		strings.HasPrefix(low, "quién ") || strings.HasPrefix(low, "quien ") ||
		strings.HasPrefix(low, "cómo ") || strings.HasPrefix(low, "como ") {
		return ternaryFact{}, false
	}
	type pat struct {
		needle string
		rel    string
	}
	patterns := []pat{
		{" no es un ", "no_es"},
		{" no es una ", "no_es"},
		{" no es ", "no_es"},
		{" implica que ", "implica"},
		{" implica ", "implica"},
		{" es parte de ", "parte_de"},
		{" forma parte de ", "parte_de"},
		{" pertenece a ", "parte_de"},
		{" tiene ", "tiene"},
		{" es un ", "es_un"},
		{" es una ", "es_un"},
		{" es ", "es"},
	}
	for _, p := range patterns {
		i := strings.Index(low, p.needle)
		if i <= 0 {
			continue
		}
		subj := normalizeReasonToken(text[:i])
		rest := strings.TrimSpace(text[i+len(p.needle):])
		for _, sep := range []string{",", ";", " y ", " pero ", " porque ", " entonces "} {
			if j := strings.Index(strings.ToLower(rest), sep); j > 0 {
				rest = strings.TrimSpace(rest[:j])
			}
		}
		obj := normalizeReasonToken(rest)
		if len([]rune(subj)) < 2 || len([]rune(obj)) < 2 || len([]rune(subj)) > 56 || len([]rune(obj)) > 72 {
			continue
		}
		if subj == "esto" || subj == "eso" || obj == "esto" || obj == "eso" {
			continue
		}
		if conf < 0 {
			conf = 0
		}
		if conf > 2 {
			conf = 2
		}
		return ternaryFact{Subj: subj, Rel: p.rel, Obj: obj, Conf: conf, Src: src}, true
	}
	return ternaryFact{}, false
}

func parseEsFact(text string, conf int, src string) (ternaryFact, bool) {
	return parseRelationFact(text, conf, src)
}

func factKey(f ternaryFact) string {
	return f.Subj + "|" + f.Rel + "|" + f.Obj
}

func collectReasonFacts(userText string) []ternaryFact {
	var out []ternaryFact
	seen := map[string]bool{}
	add := func(f ternaryFact) {
		k := factKey(f)
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, f)
	}
	for _, part := range splitReasonClauses(userText) {
		if f, ok := parseRelationFact(part, 2, "usuario"); ok {
			add(f)
		}
	}
	for _, e := range loadMindKnowledge() {
		if f, ok := parseRelationFact(e.Text, 2, "corpus"); ok {
			add(f)
		}
		sent := e.Text
		if i := strings.Index(sent, "."); i > 0 {
			sent = sent[:i]
		}
		if f, ok := parseRelationFact(sent, 1, "corpus"); ok {
			add(f)
		}
		// keys as weak "topic es type" not used — avoid noise
	}
	return out
}

func isIdentityRel(rel string) bool {
	return rel == "es" || rel == "es_un"
}

// deduceAll applies the full rule set (combinaciones de premisas).
func deduceAll(facts []ternaryFact) []ternaryFact {
	var derived []ternaryFact
	seen := map[string]bool{}
	for _, f := range facts {
		seen[factKey(f)] = true
	}
	add := func(f ternaryFact) {
		k := factKey(f)
		if seen[k] || f.Conf == 0 {
			return
		}
		seen[k] = true
		derived = append(derived, f)
	}

	for i := range facts {
		for j := range facts {
			if i == j {
				continue
			}
			a, b := facts[i], facts[j]
			conf := zyrionMinConf(a.Conf, b.Conf)

			// 1) Transitividad de es / es_un: A es B, B es C ⇒ A es C
			if isIdentityRel(a.Rel) && isIdentityRel(b.Rel) && a.Obj == b.Subj {
				add(ternaryFact{Subj: a.Subj, Rel: "es", Obj: b.Obj, Conf: conf, Src: "regla:trans-es"})
			}

			// 2) Transitividad de implica: A implica B, B implica C ⇒ A implica C
			if a.Rel == "implica" && b.Rel == "implica" && a.Obj == b.Subj {
				add(ternaryFact{Subj: a.Subj, Rel: "implica", Obj: b.Obj, Conf: conf, Src: "regla:trans-implica"})
			}

			// 3) Transitividad parte_de: A parte_de B, B parte_de C ⇒ A parte_de C
			if a.Rel == "parte_de" && b.Rel == "parte_de" && a.Obj == b.Subj {
				add(ternaryFact{Subj: a.Subj, Rel: "parte_de", Obj: b.Obj, Conf: conf, Src: "regla:trans-parte"})
			}

			// 4) Cadena es + implica: A es B, B implica C ⇒ A implica C
			if isIdentityRel(a.Rel) && b.Rel == "implica" && a.Obj == b.Subj {
				add(ternaryFact{Subj: a.Subj, Rel: "implica", Obj: b.Obj, Conf: conf, Src: "regla:es+implica"})
			}

			// 5) Cadena implica + es: A implica B, B es C ⇒ A implica C
			if a.Rel == "implica" && isIdentityRel(b.Rel) && a.Obj == b.Subj {
				add(ternaryFact{Subj: a.Subj, Rel: "implica", Obj: b.Obj, Conf: conf, Src: "regla:implica+es"})
			}

			// 6) Cadena es + parte_de: A es B, B parte_de C ⇒ A parte_de C (matiz)
			if isIdentityRel(a.Rel) && b.Rel == "parte_de" && a.Obj == b.Subj {
				c := conf
				if c > 1 {
					c = 1 // matiz: no siempre idéntico a pertenencia
				}
				add(ternaryFact{Subj: a.Subj, Rel: "parte_de", Obj: b.Obj, Conf: c, Src: "regla:es+parte"})
			}

			// 7) Herencia de tiene: A es B, B tiene C ⇒ A tiene C (matiz)
			if isIdentityRel(a.Rel) && b.Rel == "tiene" && a.Obj == b.Subj {
				c := conf
				if c > 1 {
					c = 1
				}
				add(ternaryFact{Subj: a.Subj, Rel: "tiene", Obj: b.Obj, Conf: c, Src: "regla:es+tiene"})
			}

			// 8) Contraposición débil: A implica B, X no_es B no genera A automáticamente
			//    (omitida: arriesgada sin cuantificadores)

			// 9) Conflicto: A es B y A no_es B ⇒ no afirmar (conf 0 señal)
			if isIdentityRel(a.Rel) && b.Rel == "no_es" && a.Subj == b.Subj && a.Obj == b.Obj {
				add(ternaryFact{Subj: a.Subj, Rel: "no_es", Obj: a.Obj, Conf: 2, Src: "regla:conflicto"})
			}
		}
	}

	// Segunda pasada: cerrar un nivel más (A→B→C→D)
	base := append([]ternaryFact{}, facts...)
	base = append(base, derived...)
	n0 := len(derived)
	for i := range base {
		for j := range base {
			if i == j {
				continue
			}
			a, b := base[i], base[j]
			conf := zyrionMinConf(a.Conf, b.Conf)
			if isIdentityRel(a.Rel) && isIdentityRel(b.Rel) && a.Obj == b.Subj {
				add(ternaryFact{Subj: a.Subj, Rel: "es", Obj: b.Obj, Conf: conf, Src: "regla:trans-es-2"})
			}
			if a.Rel == "implica" && b.Rel == "implica" && a.Obj == b.Subj {
				add(ternaryFact{Subj: a.Subj, Rel: "implica", Obj: b.Obj, Conf: conf, Src: "regla:trans-implica-2"})
			}
		}
	}
	_ = n0
	return derived
}

// deduceTransitive kept as alias for tests
func deduceTransitive(facts []ternaryFact) []ternaryFact {
	return deduceAll(facts)
}

func relVoice(rel string) string {
	switch rel {
	case "es_un":
		return "es un/a"
	case "implica":
		return "implica"
	case "parte_de":
		return "es parte de"
	case "tiene":
		return "tiene"
	case "no_es":
		return "no es"
	default:
		return "es"
	}
}

func formatDeductionVoice(premises []ternaryFact, concl ternaryFact) string {
	var b strings.Builder
	b.WriteString("Deducción ternaria (no predicción):\n")
	shown := 0
	for _, p := range premises {
		if shown >= 4 {
			break
		}
		if p.Subj == concl.Subj || p.Obj == concl.Subj || p.Obj == concl.Obj || p.Subj == concl.Obj {
			b.WriteString(fmt.Sprintf("· Premisa (conf %d): «%s» %s «%s».\n", p.Conf, p.Subj, relVoice(p.Rel), p.Obj))
			shown++
		}
	}
	mod := "afirmo"
	switch concl.Conf {
	case 0:
		mod = "no afirmo"
	case 1:
		mod = "matizo"
	}
	if strings.Contains(concl.Src, "conflicto") {
		b.WriteString(fmt.Sprintf("· Conflicto (conf %d): «%s» no puede ser y no ser «%s» a la vez — no afirmo la identidad.\n", concl.Conf, concl.Subj, concl.Obj))
		return strings.TrimSpace(b.String())
	}
	b.WriteString(fmt.Sprintf("· Conclusión (%s, conf %d): «%s» %s «%s»", mod, concl.Conf, concl.Subj, relVoice(concl.Rel), concl.Obj))
	if strings.HasPrefix(concl.Src, "regla:") {
		b.WriteString(" — vía " + concl.Src + ".")
	}
	return b.String()
}

func scoreFactAgainstQuery(f ternaryFact, qtoks []string) int {
	sc := f.Conf
	for _, t := range qtoks {
		if len(t) < 3 {
			continue
		}
		if strings.Contains(f.Subj, t) || strings.Contains(f.Obj, t) {
			sc += 3
		}
	}
	if strings.HasPrefix(f.Src, "regla:") {
		sc += 2
	}
	return sc
}

func reasonAboutQuery(userText string, extra []ternaryFact) string {
	low := strings.ToLower(strings.TrimSpace(userText))
	want := isReasoningRequest(low)
	facts := collectReasonFacts(userText)
	facts = append(facts, extra...)
	// Deduplicate
	seen := map[string]bool{}
	var uniq []ternaryFact
	for _, f := range facts {
		k := factKey(f)
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, f)
	}
	facts = uniq
	derived := deduceAll(facts)
	if len(derived) == 0 && !want {
		return ""
	}
	if want && len(derived) == 0 {
		var lines []string
		for _, f := range facts {
			if f.Src == "usuario" || f.Conf >= 1 {
				lines = append(lines, fmt.Sprintf("Premisa (conf %d): «%s» %s «%s» [%s].", f.Conf, f.Subj, relVoice(f.Rel), f.Obj, f.Src))
			}
		}
		if len(lines) > 0 {
			lines = append(lines, "Aún no cierro una regla (falta eslabón compartido B). Aporta otra premisa «X es/implica/parte de Y».")
			return strings.Join(lines, "\n")
		}
		return ""
	}
	qtoks := tokenizeMind(low)
	best := ternaryFact{}
	bestSc := -1
	for _, f := range derived {
		sc := scoreFactAgainstQuery(f, qtoks)
		if sc > bestSc {
			bestSc = sc
			best = f
		}
	}
	if bestSc < 0 {
		return ""
	}
	return formatDeductionVoice(facts, best)
}

func isReasoningRequest(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(low, "entonces") || strings.Contains(low, "por tanto") ||
		strings.Contains(low, "por lo tanto") || strings.Contains(low, "se deduce") ||
		strings.Contains(low, "deduce") || strings.Contains(low, "silogismo") ||
		strings.Contains(low, "qué implica") || strings.Contains(low, "que implica") ||
		strings.Contains(low, "lógicamente") || strings.Contains(low, "logicamente") ||
		strings.Contains(low, "qué se sigue") || strings.Contains(low, "que se sigue") {
		return true
	}
	if strings.Contains(low, "si ") && strings.Contains(low, " entonces") {
		return true
	}
	if strings.Count(low, " es ") >= 2 && (strings.Contains(low, " y ") || strings.Contains(low, ",")) {
		return true
	}
	if strings.Count(low, " implica ") >= 1 && (strings.Contains(low, " y ") || strings.Contains(low, "es ")) {
		return true
	}
	return false
}

func factsFromEpisodes(episodes []mindEpisodePayload) []ternaryFact {
	var out []ternaryFact
	for _, ep := range episodes {
		for _, part := range splitReasonClauses(ep.Text) {
			if f, ok := parseRelationFact(part, 1, "memoria"); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

// softReasonFromKnowledge: si la pregunta toca entidades del corpus con cadena deducible, habla con regla.
func softReasonFromKnowledge(userText string) string {
	low := strings.ToLower(userText)
	if isCalmChat(low) || isIdentityTalk(low) || isCreativeWriteRequest(low) {
		return ""
	}
	facts := collectReasonFacts(userText)
	if len(facts) < 2 {
		return ""
	}
	derived := deduceAll(facts)
	if len(derived) == 0 {
		return ""
	}
	// solo si hay token de consulta que coincida con conclusión
	qtoks := tokenizeMind(low)
	best := ternaryFact{}
	bestSc := -1
	for _, f := range derived {
		sc := scoreFactAgainstQuery(f, qtoks)
		if sc > bestSc {
			bestSc = sc
			best = f
		}
	}
	if bestSc < 5 {
		return ""
	}
	return formatDeductionVoice(facts, best)
}
