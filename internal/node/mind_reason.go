package node

import (
	"fmt"
	"strings"
)

// Capa de razón ternaria (no LLM): hechos A—rel—B con confianza 0/1/2
// y cierre transitivo / modus debil bajo Zyrion.
// 0 = no afirmar, 1 = afirmar con matiz, 2 = afirmar (o sumidero si premisas chocan).

type ternaryFact struct {
	Subj string
	Rel  string // "es", "es_un", "parte_de", "implica"
	Obj  string
	Conf int // 0,1,2
	Src  string
}

// zyrionAnd: conjunción ternaria absorbente en 2 solo si hay conflicto fuerte;
// aquí 2 = "afirmado con fuerza", 1 = parcial, 0 = no.
// Para premisas: min confiable estilo Kleene: conf(C) = min(conf(A), conf(B)) si cadena limpia.
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
	// cut trailing "entonces…"
	low := strings.ToLower(text)
	for _, sep := range []string{"; entonces", ", entonces", " entonces", "; por tanto", " por tanto"} {
		if i := strings.Index(low, sep); i > 0 {
			text = strings.TrimSpace(text[:i])
			low = strings.ToLower(text)
		}
	}
	parts := strings.Split(text, " y ")
	if len(parts) == 1 {
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

// parseEsFact: "X es Y" / "X es un Y" (una cláusula corta).
func parseEsFact(text string, conf int, src string) (ternaryFact, bool) {
	low := strings.ToLower(strings.TrimSpace(text))
	if strings.Contains(low, "?") || strings.HasPrefix(low, "qué ") || strings.HasPrefix(low, "que ") ||
		strings.HasPrefix(low, "quién ") || strings.HasPrefix(low, "quien ") {
		return ternaryFact{}, false
	}
	rel := "es"
	var idx int
	if i := strings.Index(low, " es un "); i > 0 {
		idx = i
		rel = "es_un"
	} else if i := strings.Index(low, " es una "); i > 0 {
		idx = i
		rel = "es_un"
	} else if i := strings.Index(low, " es "); i > 0 {
		idx = i
		rel = "es"
	} else {
		return ternaryFact{}, false
	}
	subj := normalizeReasonToken(text[:idx])
	rest := strings.TrimSpace(text[idx:])
	// skip "es un/una/es "
	lowRest := strings.ToLower(rest)
	for _, p := range []string{"es un ", "es una ", "es "} {
		if strings.HasPrefix(lowRest, p) {
			rest = strings.TrimSpace(rest[len(p):])
			break
		}
	}
	// cut at comma or second clause
	for _, sep := range []string{",", ";", " y ", " pero ", " porque "} {
		if j := strings.Index(strings.ToLower(rest), sep); j > 0 {
			rest = strings.TrimSpace(rest[:j])
		}
	}
	obj := normalizeReasonToken(rest)
	if len([]rune(subj)) < 2 || len([]rune(obj)) < 2 || len([]rune(subj)) > 48 || len([]rune(obj)) > 64 {
		return ternaryFact{}, false
	}
	// avoid noise
	if subj == "esto" || subj == "eso" || obj == "esto" || obj == "eso" {
		return ternaryFact{}, false
	}
	if conf < 0 {
		conf = 0
	}
	if conf > 2 {
		conf = 2
	}
	return ternaryFact{Subj: subj, Rel: rel, Obj: obj, Conf: conf, Src: src}, true
}

func collectReasonFacts(userText string) []ternaryFact {
	var out []ternaryFact
	seen := map[string]bool{}
	add := func(f ternaryFact) {
		k := f.Subj + "|" + f.Rel + "|" + f.Obj
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, f)
	}
	// From user utterance (possibly chained: "A es B y B es C")
	for _, part := range splitReasonClauses(userText) {
		if f, ok := parseEsFact(part, 2, "usuario"); ok {
			add(f)
		}
	}
	// From corpus (short definitions)
	for _, e := range loadMindKnowledge() {
		if f, ok := parseEsFact(e.Text, 2, "corpus"); ok {
			add(f)
		}
		// also first sentence
		sent := e.Text
		if i := strings.Index(sent, "."); i > 0 {
			sent = sent[:i]
		}
		if f, ok := parseEsFact(sent, 1, "corpus"); ok {
			add(f)
		}
	}
	// From recent episodes (light)
	// (caller can pass; here we keep pure for tests)
	return out
}

// deduceTransitive: si A—es—B y B—es—C ⇒ A—es—C con conf = min(conf1, conf2).
func deduceTransitive(facts []ternaryFact) []ternaryFact {
	var derived []ternaryFact
	for i := range facts {
		for j := range facts {
			if i == j {
				continue
			}
			a, b := facts[i], facts[j]
			// A es B, B es C
			if a.Obj == b.Subj && (a.Rel == "es" || a.Rel == "es_un") && (b.Rel == "es" || b.Rel == "es_un") {
				conf := zyrionMinConf(a.Conf, b.Conf)
				if conf == 0 {
					continue
				}
				derived = append(derived, ternaryFact{
					Subj: a.Subj,
					Rel:  "es",
					Obj:  b.Obj,
					Conf: conf,
					Src:  "deducción:" + a.Src + "+" + b.Src,
				})
			}
		}
	}
	return derived
}

// reasonAboutQuery: responde por deducción si el usuario pide conclusión o pregunta "entonces…".
func reasonAboutQuery(userText string, extra []ternaryFact) string {
	low := strings.ToLower(strings.TrimSpace(userText))
	want := strings.Contains(low, "entonces") || strings.Contains(low, "por tanto") ||
		strings.Contains(low, "por lo tanto") || strings.Contains(low, "se deduce") ||
		strings.Contains(low, "deduce") || strings.Contains(low, "silogismo") ||
		strings.Contains(low, "si ") && strings.Contains(low, " entonces") ||
		strings.Contains(low, "qué implica") || strings.Contains(low, "que implica") ||
		strings.Contains(low, "lógicamente") || strings.Contains(low, "logicamente")
	facts := collectReasonFacts(userText)
	facts = append(facts, extra...)
	derived := deduceTransitive(facts)
	if len(derived) == 0 && !want {
		return ""
	}
	// Also try one-step: user asks "X es Z?" by matching derived or facts
	var lines []string
	if want && len(derived) == 0 && len(facts) >= 1 {
		// state premises only
		for _, f := range facts {
			if f.Src == "usuario" || f.Conf >= 1 {
				lines = append(lines, fmt.Sprintf("Premisa (%d): «%s» %s «%s» [%s].", f.Conf, f.Subj, relVoice(f.Rel), f.Obj, f.Src))
			}
		}
		if len(lines) > 0 {
			lines = append(lines, "Aún no cierro una transitividad (falta un eslabón B compartido). Aporta otra premisa «X es Y».")
			return strings.Join(lines, "\n")
		}
		return ""
	}
	// Prefer derived matching query tokens
	qtoks := tokenizeMind(low)
	scoreFact := func(f ternaryFact) int {
		sc := f.Conf
		for _, t := range qtoks {
			if len(t) < 3 {
				continue
			}
			if strings.Contains(f.Subj, t) || strings.Contains(f.Obj, t) {
				sc += 3
			}
		}
		return sc
	}
	best := ternaryFact{}
	bestSc := -1
	for _, f := range derived {
		sc := scoreFact(f)
		if sc > bestSc {
			bestSc = sc
			best = f
		}
	}
	if bestSc < 0 && want {
		for _, f := range derived {
			return formatDeductionVoice(facts, f)
		}
	}
	if bestSc >= 0 {
		return formatDeductionVoice(facts, best)
	}
	return ""
}

func relVoice(rel string) string {
	switch rel {
	case "es_un":
		return "es un/a"
	default:
		return "es"
	}
}

func formatDeductionVoice(premises []ternaryFact, concl ternaryFact) string {
	var b strings.Builder
	b.WriteString("Deducción ternaria (no predicción):\n")
	shown := 0
	for _, p := range premises {
		if (p.Obj == concl.Subj || p.Subj == concl.Subj || p.Obj == concl.Obj) && shown < 3 {
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
	case 2:
		mod = "afirmo"
	}
	b.WriteString(fmt.Sprintf("· Conclusión (%s, conf %d): «%s» es «%s»", mod, concl.Conf, concl.Subj, concl.Obj))
	if strings.HasPrefix(concl.Src, "deducción") {
		b.WriteString(" — por transitividad de «es».")
	}
	return b.String()
}

// isReasoningRequest — gatillo de la capa de razón.
func isReasoningRequest(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(low, "entonces") || strings.Contains(low, "por tanto") ||
		strings.Contains(low, "por lo tanto") || strings.Contains(low, "se deduce") ||
		strings.Contains(low, "deduce") || strings.Contains(low, "silogismo") ||
		strings.Contains(low, "qué implica") || strings.Contains(low, "que implica") {
		return true
	}
	if strings.Contains(low, "si ") && strings.Contains(low, " entonces") {
		return true
	}
	// "A es B y B es C" style
	if strings.Count(low, " es ") >= 2 && (strings.Contains(low, " y ") || strings.Contains(low, ",")) {
		return true
	}
	return false
}

// factsFromEpisodes pulls short "X es Y" from recent mind episodes.
func factsFromEpisodes(episodes []mindEpisodePayload) []ternaryFact {
	var out []ternaryFact
	for _, ep := range episodes {
		if f, ok := parseEsFact(ep.Text, 1, "memoria"); ok {
			out = append(out, f)
		}
	}
	return out
}
