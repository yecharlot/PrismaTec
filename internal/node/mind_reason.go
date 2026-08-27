package node

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	lastReasonMu         sync.Mutex
	lastReasonConclusion string
	lastReasonAt         time.Time
)

func setLastReasonConclusion(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	lastReasonMu.Lock()
	lastReasonConclusion = s
	lastReasonAt = time.Now()
	lastReasonMu.Unlock()
}

func getLastReasonConclusion() string {
	lastReasonMu.Lock()
	defer lastReasonMu.Unlock()
	if lastReasonConclusion == "" {
		return ""
	}
	if time.Since(lastReasonAt) > 30*time.Minute {
		return ""
	}
	return lastReasonConclusion
}


// Capa de razón ternaria (no LLM): hechos 0/1/2 + reglas + extracción gramatical
// de cláusulas en textos largos. Autosimilitud: las mismas reglas se reaplican
// en un segundo nivel (cierre fractal).

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
	// una sola frase / sin basura
	if i := strings.IndexAny(s, ".!?;"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// extractSentences: gramática mínima — cortar por . ! ? y por ;
func extractSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sents []string
	start := 0
	runes := []rune(text)
	for i, r := range runes {
		if r == '.' || r == '!' || r == '?' || r == ';' {
			part := strings.TrimSpace(string(runes[start:i]))
			if part != "" {
				sents = append(sents, part)
			}
			start = i + 1
		}
	}
	if start < len(runes) {
		part := strings.TrimSpace(string(runes[start:]))
		if part != "" {
			sents = append(sents, part)
		}
	}
	if len(sents) == 0 {
		return []string{text}
	}
	return sents
}

// extractClauses: oraciones → cláusulas por " y " / " pero " cuando ambas parecen predicados
func extractClauses(text string) []string {
	var out []string
	for _, sent := range extractSentences(text) {
		low := strings.ToLower(sent)
		// quitar coletillas meta de deducción
		for _, noise := range []string{
			"por transitividad de es", "por transitividad", "ejemplo canónico",
			"ejemplo de cadena", "no predicción", "deducción ternaria",
		} {
			if i := strings.Index(low, noise); i >= 0 {
				sent = strings.TrimSpace(sent[:i])
				low = strings.ToLower(sent)
			}
		}
		if sent == "" {
			continue
		}
		// split y solo si ambos lados tienen verbo relacional
		if strings.Count(low, " y ") == 1 {
			i := strings.Index(low, " y ")
			left, right := strings.TrimSpace(sent[:i]), strings.TrimSpace(sent[i+3:])
			if clauseLooksRelational(left) && clauseLooksRelational(right) {
				out = append(out, left, right)
				continue
			}
		}
		out = append(out, sent)
	}
	return out
}

func clauseLooksRelational(s string) bool {
	low := strings.ToLower(s)
	keys := []string{" es ", " implica ", " tiene ", " parte de ", " pertenece ", " no es "}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

func splitReasonClauses(text string) []string {
	return extractClauses(text)
}

func parseRelationFact(text string, conf int, src string) (ternaryFact, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return ternaryFact{}, false
	}
	// solo primera oración
	if sents := extractSentences(text); len(sents) > 0 {
		text = sents[0]
	}
	low := strings.ToLower(strings.TrimSpace(text))
	if strings.Contains(low, "?") || strings.HasPrefix(low, "qué ") || strings.HasPrefix(low, "que ") ||
		strings.HasPrefix(low, "quién ") || strings.HasPrefix(low, "quien ") ||
		strings.HasPrefix(low, "cómo ") || strings.HasPrefix(low, "como ") ||
		strings.HasPrefix(low, "por transitividad") || strings.HasPrefix(low, "ejemplo") {
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
		// cortar en puntuación y conjunciones
		for _, sep := range []string{".", "!", "?", ";", ",", " y ", " pero ", " porque ", " entonces ", " —", " - "} {
			if j := strings.Index(strings.ToLower(rest), sep); j > 0 {
				rest = strings.TrimSpace(rest[:j])
			}
		}
		obj := normalizeReasonToken(rest)
		if !validReasonTerm(subj) || !validReasonTerm(obj) {
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

func validReasonTerm(s string) bool {
	s = strings.TrimSpace(s)
	if len([]rune(s)) < 2 || len([]rune(s)) > 48 {
		return false
	}
	// no párrafos ni meta
	if strings.Contains(s, " transitividad") || strings.Contains(s, "ejemplo") ||
		strings.Contains(s, "silogismo") || strings.Count(s, " ") > 6 {
		return false
	}
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters >= 2
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
	for _, part := range extractClauses(userText) {
		if f, ok := parseRelationFact(part, 2, "usuario"); ok {
			add(f)
		}
	}
	for _, e := range loadMindKnowledge() {
		// solo oraciones cortas del corpus (gramática limpia)
		for _, sent := range extractSentences(e.Text) {
			if len([]rune(sent)) > 100 {
				continue
			}
			low := strings.ToLower(sent)
			if strings.HasPrefix(low, "por ") || strings.HasPrefix(low, "ejemplo") ||
				strings.Contains(low, "no predice") || strings.Contains(low, "léxico") {
				continue
			}
			if f, ok := parseRelationFact(sent, 1, "corpus"); ok {
				add(f)
			}
		}
	}
	return out
}

func isIdentityRel(rel string) bool {
	return rel == "es" || rel == "es_un"
}

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
		// evitar conclusiones basura
		if !validReasonTerm(f.Subj) || !validReasonTerm(f.Obj) {
			return
		}
		seen[k] = true
		derived = append(derived, f)
	}

	applyOnce := func(pool []ternaryFact) {
		for i := range pool {
			for j := range pool {
				if i == j {
					continue
				}
				a, b := pool[i], pool[j]
				conf := zyrionMinConf(a.Conf, b.Conf)

				if isIdentityRel(a.Rel) && isIdentityRel(b.Rel) && a.Obj == b.Subj {
					add(ternaryFact{Subj: a.Subj, Rel: "es", Obj: b.Obj, Conf: conf, Src: "regla:trans-es"})
				}
				if a.Rel == "implica" && b.Rel == "implica" && a.Obj == b.Subj {
					add(ternaryFact{Subj: a.Subj, Rel: "implica", Obj: b.Obj, Conf: conf, Src: "regla:trans-implica"})
				}
				if a.Rel == "parte_de" && b.Rel == "parte_de" && a.Obj == b.Subj {
					add(ternaryFact{Subj: a.Subj, Rel: "parte_de", Obj: b.Obj, Conf: conf, Src: "regla:trans-parte"})
				}
				if isIdentityRel(a.Rel) && b.Rel == "implica" && a.Obj == b.Subj {
					add(ternaryFact{Subj: a.Subj, Rel: "implica", Obj: b.Obj, Conf: conf, Src: "regla:es+implica"})
				}
				if a.Rel == "implica" && isIdentityRel(b.Rel) && a.Obj == b.Subj {
					add(ternaryFact{Subj: a.Subj, Rel: "implica", Obj: b.Obj, Conf: conf, Src: "regla:implica+es"})
				}
				if isIdentityRel(a.Rel) && b.Rel == "parte_de" && a.Obj == b.Subj {
					c := conf
					if c > 1 {
						c = 1
					}
					add(ternaryFact{Subj: a.Subj, Rel: "parte_de", Obj: b.Obj, Conf: c, Src: "regla:es+parte"})
				}
				if isIdentityRel(a.Rel) && b.Rel == "tiene" && a.Obj == b.Subj {
					c := conf
					if c > 1 {
						c = 1
					}
					add(ternaryFact{Subj: a.Subj, Rel: "tiene", Obj: b.Obj, Conf: c, Src: "regla:es+tiene"})
				}
				// A tiene B, B es C ⇒ A tiene C (matiz)
				if a.Rel == "tiene" && isIdentityRel(b.Rel) && a.Obj == b.Subj {
					c := conf
					if c > 1 {
						c = 1
					}
					add(ternaryFact{Subj: a.Subj, Rel: "tiene", Obj: b.Obj, Conf: c, Src: "regla:tiene+es"})
				}
				// A parte_de B, B es C ⇒ A parte_de C (matiz)
				if a.Rel == "parte_de" && isIdentityRel(b.Rel) && a.Obj == b.Subj {
					c := conf
					if c > 1 {
						c = 1
					}
					add(ternaryFact{Subj: a.Subj, Rel: "parte_de", Obj: b.Obj, Conf: c, Src: "regla:parte+es"})
				}
				if isIdentityRel(a.Rel) && b.Rel == "no_es" && a.Subj == b.Subj && a.Obj == b.Obj {
					add(ternaryFact{Subj: a.Subj, Rel: "no_es", Obj: a.Obj, Conf: 2, Src: "regla:conflicto"})
				}
			}
		}
	}

	// Nivel 0 → 1 (misma ley)
	applyOnce(facts)
	// Nivel fractal 2: reaplicar sobre hechos+derivados (autosimilitud)
	pool := append([]ternaryFact{}, facts...)
	pool = append(pool, derived...)
	applyOnce(pool)

	return derived
}

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
	shown := map[string]bool{}
	n := 0
	for _, p := range premises {
		if n >= 4 {
			break
		}
		if !validReasonTerm(p.Subj) || !validReasonTerm(p.Obj) {
			continue
		}
		if p.Subj == concl.Subj || p.Obj == concl.Subj || p.Obj == concl.Obj || p.Subj == concl.Obj {
			k := factKey(p)
			if shown[k] {
				continue
			}
			shown[k] = true
			b.WriteString(fmt.Sprintf("· Premisa (conf %d): «%s» %s «%s».\n", p.Conf, p.Subj, relVoice(p.Rel), p.Obj))
			n++
		}
	}
	if strings.Contains(concl.Src, "conflicto") {
		b.WriteString(fmt.Sprintf("· Conflicto: «%s» no puede ser y no ser «%s» — no afirmo.\n", concl.Subj, concl.Obj))
		return strings.TrimSpace(b.String())
	}
	mod := "afirmo"
	switch concl.Conf {
	case 0:
		mod = "no afirmo"
	case 1:
		mod = "matizo"
	}
	b.WriteString(fmt.Sprintf("· Conclusión (%s, conf %d): «%s» %s «%s»", mod, concl.Conf, concl.Subj, relVoice(concl.Rel), concl.Obj))
	if strings.HasPrefix(concl.Src, "regla:") {
		b.WriteString(" — vía " + concl.Src + ".")
	}
	return b.String()
}

func scoreFactAgainstQuery(f ternaryFact, qtoks []string) int {
	sc := f.Conf
	hit := false
	for _, t := range qtoks {
		if len(t) < 4 {
			continue
		}
		if strings.Contains(f.Subj, t) || strings.Contains(f.Obj, t) {
			sc += 4
			hit = true
		}
	}
	if !hit {
		return 0 // sin solape léxico con la pregunta, no empujar deducción
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
			if f.Src == "usuario" && validReasonTerm(f.Subj) && validReasonTerm(f.Obj) {
				lines = append(lines, fmt.Sprintf("Premisa (conf %d): «%s» %s «%s».", f.Conf, f.Subj, relVoice(f.Rel), f.Obj))
			}
		}
		if len(lines) > 0 {
			lines = append(lines, "Aún no cierro una regla (falta eslabón B). Aporta otra premisa.")
			return strings.Join(lines, "\n")
		}
		return ""
	}
	qtoks := tokenizeMind(low)
	best := ternaryFact{}
	bestSc := -1
	for _, f := range derived {
		sc := scoreFactAgainstQuery(f, qtoks)
		// si el usuario aportó premisas, preferir conclusiones de sus términos
		for _, p := range facts {
			if p.Src == "usuario" && (f.Subj == p.Subj || f.Obj == p.Obj) {
				sc += 5
			}
		}
		if sc > bestSc {
			bestSc = sc
			best = f
		}
	}
	if bestSc < 1 {
		return ""
	}
	if sent := conclusionSentence(best); sent != "" {
		setLastReasonConclusion(sent)
	}
	return formatDeductionVoice(facts, best)
}

func isReasoningRequest(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	// no confundir definiciones
	if strings.HasPrefix(low, "qué es ") || strings.HasPrefix(low, "que es ") ||
		strings.HasPrefix(low, "qué es un ") || strings.HasPrefix(low, "que es un ") ||
		strings.HasPrefix(low, "cómo razona") || strings.HasPrefix(low, "como razona") {
		return false
	}
	if strings.Contains(low, "entonces") || strings.Contains(low, "por tanto") ||
		strings.Contains(low, "por lo tanto") || strings.Contains(low, "se deduce") ||
		strings.Contains(low, "deduce") || strings.Contains(low, "qué implica") ||
		strings.Contains(low, "que implica") || strings.Contains(low, "qué se sigue") ||
		strings.Contains(low, "que se sigue") {
		return true
	}
	if strings.Contains(low, "si ") && strings.Contains(low, " entonces") {
		return true
	}
	if strings.Count(low, " es ") >= 2 && (strings.Contains(low, " y ") || strings.Contains(low, ",")) {
		return true
	}
	if strings.Count(low, " implica ") >= 2 {
		return true
	}
	return false
}

func factsFromEpisodes(episodes []mindEpisodePayload) []ternaryFact {
	var out []ternaryFact
	for _, ep := range episodes {
		for _, part := range extractClauses(ep.Text) {
			if f, ok := parseRelationFact(part, 1, "memoria"); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

// softReasonFromKnowledge: solo si hay solape real con la pregunta (no regurgitar Sócrates).
func softReasonFromKnowledge(userText string) string {
	low := strings.ToLower(userText)
	if isCalmChat(low) || isIdentityTalk(low) || isCreativeWriteRequest(low) {
		return ""
	}
	if strings.HasPrefix(low, "qué es") || strings.HasPrefix(low, "que es") ||
		strings.HasPrefix(low, "cómo ") || strings.HasPrefix(low, "como ") ||
		strings.HasPrefix(low, "quién ") || strings.HasPrefix(low, "quien ") {
		return ""
	}
	if !isReasoningRequest(low) {
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
	if bestSc < 6 {
		return ""
	}
	return formatDeductionVoice(facts, best)
}

func tokenOverlapTheme(theme, text string) bool {
	for _, w := range strings.Fields(theme) {
		if len([]rune(w)) >= 3 && strings.Contains(text, w) {
			return true
		}
	}
	return false
}

// conclusionSentence — frase corta usable como ancla creativa/compose.
func conclusionSentence(f ternaryFact) string {
	if !validReasonTerm(f.Subj) || !validReasonTerm(f.Obj) {
		return ""
	}
	return f.Subj + " " + relVoice(f.Rel) + " " + f.Obj
}

// reasonAnchorForTheme: deduce sobre el tema (usuario+corpus+memoria) y devuelve
// la mejor conclusión como ancla factual (no predicción libre).
func reasonAnchorForTheme(theme, userText string, extra []ternaryFact) string {
	theme = normalizeReasonToken(theme)
	for _, a := range []string{"el ", "la ", "los ", "las ", "un ", "una ", "lo "} {
		if strings.HasPrefix(theme, a) {
			theme = strings.TrimSpace(theme[len(a):])
		}
	}
	// Última conclusión del latido de razón (prioridad sobre corpus genérico)
	if last := getLastReasonConclusion(); last != "" {
		ll := strings.ToLower(last)
		th := strings.ToLower(theme)
		if th == "" || strings.Contains(ll, th) || tokenOverlapTheme(th, ll) {
			return last
		}
	}
	if theme == "" {
		return ""
	}
	facts := collectReasonFacts(userText)
	facts = append(facts, extra...)
	// hechos del corpus que toquen el tema
	for _, e := range loadMindKnowledge() {
		blob := strings.ToLower(strings.Join(e.Keys, " ") + " " + e.Text)
		if theme != "" && !strings.Contains(blob, theme) && !strings.Contains(blob, strings.TrimSpace(theme)) {
			continue
		}
		for _, sent := range extractSentences(e.Text) {
			if f, ok := parseRelationFact(sent, 1, "corpus-tema"); ok {
				facts = append(facts, f)
			}
		}
	}
	// dedupe
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
	derived := deduceAll(uniq)
	if len(derived) == 0 {
		// sin cadena: si hay un hecho directo sobre el tema, úsalo
		for _, f := range uniq {
			if strings.Contains(f.Subj, theme) || strings.Contains(f.Obj, theme) ||
				strings.Contains(theme, f.Subj) || strings.Contains(theme, f.Obj) {
				if s := conclusionSentence(f); s != "" {
					return s
				}
			}
		}
		return ""
	}
	qtoks := tokenizeMind(theme + " " + strings.ToLower(userText))
	best := ternaryFact{}
	bestSc := -1
	for _, f := range derived {
		sc := scoreFactAgainstQuery(f, qtoks)
		if strings.Contains(f.Subj, theme) || strings.Contains(f.Obj, theme) {
			sc += 6
		}
		if sc > bestSc {
			bestSc = sc
			best = f
		}
	}
	if bestSc < 1 {
		return ""
	}
	return conclusionSentence(best)
}

// composeWithReason: bloque opcional "antes deduje…" + cuerpo creativo.
func weaveReasonIntoCreative(body, reasonAnchor string) string {
	reasonAnchor = strings.TrimSpace(reasonAnchor)
	body = strings.TrimSpace(body)
	if reasonAnchor == "" {
		return body
	}
	if strings.Contains(strings.ToLower(body), strings.ToLower(reasonAnchor)) {
		return body
	}
	return body + "\n\n— Ancla deducida (no inventada):\n" + reasonAnchor
}
