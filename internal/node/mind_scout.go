package node

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Scout: Mind uses a temporary (or named) gen as a probe to the web when corpus has no answer.
// Findings are indexed by topic for reuse; ephemeral gens are deleted after reporting.

var scoutFindingIndex = struct {
	mu      sync.RWMutex
	byTopic map[string]string // normalized topic → report text
}{byTopic: map[string]string{}}

func scoutEphemeral() bool {
	v := strings.ToLower(os.Getenv("ALSET_SCOUT_EPHEMERAL"))
	if v == "0" || v == "false" || v == "no" {
		return false
	}
	return true
}

func isScoutableQuestion(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if isGenToolIntent(s) || isCodeGenRequest(s) || isCapabilityQuestion(s) {
		return false
	}
	if isPersonalFact(s) || isMemoryQuery(s) || isIdentityTalk(s) || isCalmChat(s) {
		return false
	}
	if strings.HasPrefix(s, "qué es ") || strings.HasPrefix(s, "que es ") ||
		strings.HasPrefix(s, "quién es ") || strings.HasPrefix(s, "quien es ") ||
		strings.HasPrefix(s, "quién fue ") || strings.HasPrefix(s, "quien fue ") ||
		strings.HasPrefix(s, "dónde queda ") || strings.HasPrefix(s, "donde queda ") ||
		strings.HasPrefix(s, "busca ") || strings.HasPrefix(s, "investiga ") ||
		strings.Contains(s, "en internet") || strings.Contains(s, "en la web") {
		return true
	}
	// Follow-ups: profundiza / más sobre / cuéntame de …
	if isDeepenScout(s) {
		return true
	}
	// "quién martin luther king" (sin "es")
	if isQuienBareName(s) {
		return true
	}
	if strings.Contains(s, "?") && len(s) > 12 && speakFromKnowledge(s) == "" {
		return true
	}
	return false
}

func isDeepenScout(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	keys := []string{
		"profundiza ", "profundiza sobre ", "profundiza más sobre ", "profundiza mas sobre ",
		"más sobre ", "mas sobre ", "cuéntame más sobre ", "cuentame mas sobre ",
		"cuéntame de ", "cuentame de ", "dime más de ", "dime mas de ",
		"más de ", "mas de ", "explícame más sobre ", "explicame mas sobre ",
	}
	for _, k := range keys {
		if strings.HasPrefix(s, k) {
			return true
		}
	}
	return strings.Contains(s, "profundiza") && strings.Contains(s, "sobre ")
}

func isQuienBareName(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "quién eres") || strings.HasPrefix(s, "quien eres") ||
		strings.HasPrefix(s, "quién soy") || strings.HasPrefix(s, "quien soy") ||
		strings.HasPrefix(s, "quién eres ") || strings.HasPrefix(s, "quien eres ") {
		return false
	}
	// "quién X" / "quien X" with at least one more token that is not "es"/"fue"
	for _, pfx := range []string{"quién ", "quien "} {
		if !strings.HasPrefix(s, pfx) {
			continue
		}
		rest := strings.TrimSpace(s[len(pfx):])
		if rest == "" {
			return false
		}
		if strings.HasPrefix(rest, "es ") || strings.HasPrefix(rest, "fue ") {
			return true // already covered elsewhere; still scoutable
		}
		// bare name: quien martin luter king
		if len(rest) >= 3 {
			return true
		}
	}
	return false
}

// forceWebScout: user explicitly wants the web; do not skip for corpus keyword hits.
func forceWebScout(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "busca ") || strings.HasPrefix(s, "investiga ") ||
		strings.HasPrefix(s, "quién es ") || strings.HasPrefix(s, "quien es ") ||
		strings.HasPrefix(s, "quién fue ") || strings.HasPrefix(s, "quien fue ") ||
		strings.Contains(s, "en internet") || strings.Contains(s, "en la web") {
		return true
	}
	if isDeepenScout(s) || isQuienBareName(s) {
		return true
	}
	return false
}

// extractTopic: A — clean topic from natural language query.
func extractTopic(query string) string {
	s := strings.ToLower(strings.TrimSpace(query))
	s = strings.TrimSuffix(s, "?")
	s = strings.TrimSpace(s)

	// Strip known prefixes (longest first)
	prefixes := []string{
		"profundiza más sobre ", "profundiza mas sobre ", "profundiza sobre ", "profundiza ",
		"cuéntame más sobre ", "cuentame mas sobre ", "cuéntame de ", "cuentame de ",
		"dime más de ", "dime mas de ", "explícame más sobre ", "explicame mas sobre ",
		"más sobre ", "mas sobre ", "más de ", "mas de ",
		"busca información sobre ", "busca informacion sobre ",
		"busca info sobre ", "información sobre ", "informacion sobre ",
		"busca en la web ", "busca en internet ",
		"investiga sobre ", "investiga ",
		"qué es la ", "que es la ", "qué es el ", "que es el ", "qué es ", "que es ",
		"quién es ", "quien es ", "quién fue ", "quien fue ", "quién ", "quien ",
		"dónde queda ", "donde queda ", "busca ",
	}
	for _, pfx := range prefixes {
		if strings.HasPrefix(s, pfx) {
			s = strings.TrimSpace(s[len(pfx):])
			break
		}
	}

	// Trailing noise phrases
	for _, suf := range []string{" en la web", " en internet", " por favor", " please"} {
		if strings.HasSuffix(s, suf) {
			s = strings.TrimSpace(strings.TrimSuffix(s, suf))
		}
	}

	// Stopwords: drop query glue, but KEEP linking prepositions (de/del/of/…)
	// between content words so "juana de arco" does not become "juana arco".
	stop := map[string]bool{
		"busca": true, "información": true, "informacion": true, "info": true,
		"sobre": true, "que": true, "qué": true, "es": true, "la": true, "el": true,
		"los": true, "las": true, "un": true, "una": true,
		"quien": true, "quién": true, "quienes": true, "quiénes": true, "fue": true,
		"en": true, "web": true, "internet": true, "mas": true, "más": true,
		"profundiza": true, "investiga": true, "dime": true, "cuéntame": true, "cuentame": true,
	}
	linkers := map[string]bool{
		"de": true, "del": true, "of": true, "van": true, "von": true,
		"da": true, "di": true, "y": true, "and": true,
	}
	words := strings.Fields(s)
	var kept []string
	for i, w := range words {
		w = strings.Trim(w, ".,;:¡!¿\"'")
		if w == "" {
			continue
		}
		if stop[w] {
			continue
		}
		if linkers[w] {
			hasPrev := len(kept) > 0 && !linkers[kept[len(kept)-1]]
			hasNext := false
			for j := i + 1; j < len(words); j++ {
				nw := strings.Trim(words[j], ".,;:¡!¿\"'")
				if nw == "" || stop[nw] || linkers[nw] {
					continue
				}
				hasNext = true
				break
			}
			if !hasPrev || !hasNext {
				continue
			}
		}
		kept = append(kept, w)
	}
	for len(kept) > 0 && linkers[kept[len(kept)-1]] {
		kept = kept[:len(kept)-1]
	}
	topic := strings.Join(kept, " ")
	topic = applyTopicTypos(topic)
	return strings.TrimSpace(topic)
}

func applyTopicTypos(topic string) string {
	repl := []struct{ a, b string }{
		{"luter", "luther"},
		{"lutther", "luther"},
		{"joana de arco", "juana de arco"},
		{"joana arco", "juana de arco"},
		{"juana darc", "juana de arco"},
		{"juana arco", "juana de arco"},
		{"juana d arco", "juana de arco"},
		{"joan of arc", "juana de arco"},
		{"bio informatica", "bioinformática"},
		{"bio-informatica", "bioinformática"},
		{"bioinformatica", "bioinformática"},
		{"lebrom", "lebron"},
		{"martin luther king", "martin luther king"},
		{"michel jordan", "michael jordan"},
		{"michael jordan", "michael jordan"},
		{"miquel jordan", "michael jordan"},
		{"maicol jordan", "michael jordan"},
		{"tiziano ferro", "tiziano ferro"},
	}
	low := strings.ToLower(topic)
	for _, r := range repl {
		if strings.Contains(low, r.a) {
			low = strings.ReplaceAll(low, r.a, r.b)
		}
	}
	return low
}

func topicFromQuestion(s string) string {
	return extractTopic(s)
}

func normalizeTopicKey(topic string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(topic))), " ")
}

func storeScoutFinding(topic, report string) {
	k := normalizeTopicKey(topic)
	if k == "" || report == "" {
		return
	}
	scoutFindingIndex.mu.Lock()
	scoutFindingIndex.byTopic[k] = report
	scoutFindingIndex.mu.Unlock()
}

func recallScoutFinding(query string) (topic string, report string, ok bool) {
	q := normalizeTopicKey(extractTopic(query))
	if q == "" {
		q = normalizeTopicKey(query)
	}
	scoutFindingIndex.mu.RLock()
	defer scoutFindingIndex.mu.RUnlock()
	if r, hit := scoutFindingIndex.byTopic[q]; hit {
		return q, r, true
	}
	// substring: stored topic contained in query or vice-versa
	bestK, bestR, bestN := "", "", 0
	for k, r := range scoutFindingIndex.byTopic {
		if len(k) < 4 {
			continue
		}
		if strings.Contains(q, k) || strings.Contains(k, q) {
			if len(k) > bestN {
				bestK, bestR, bestN = k, r, len(k)
			}
		}
	}
	if bestK != "" {
		return bestK, bestR, true
	}
	return "", "", false
}

func wikipediaURL(topic string) string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return ""
	}
	return "https://es.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(wikiTitle(topic), " ", "_"))
}

func wikiTitle(topic string) string {
	parts := strings.Fields(strings.TrimSpace(topic))
	for i, p := range parts {
		runes := []rune(strings.ToLower(p))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

// resolveWikipediaTitleOn searches one Wikipedia host; returns title or "".
func resolveWikipediaTitleOn(topic, host string) string {
	q := strings.TrimSpace(topic)
	if q == "" || host == "" {
		return ""
	}
	variants := []string{q}
	// accent / capitalization variants help ES search (felix → Félix)
	if !strings.Contains(q, "é") && strings.Contains(strings.ToLower(q), "felix") {
		variants = append(variants, strings.ReplaceAll(q, "felix", "félix"), strings.ReplaceAll(q, "Felix", "Félix"))
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, v := range variants {
		api := fmt.Sprintf("https://%s/w/api.php?action=query&list=search&srsearch=%s&srlimit=3&format=json",
			host, url.QueryEscape(v))
		req, err := http.NewRequest(http.MethodGet, api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "AlsetMind-Scout/1.3 (https://github.com/yecharlot/PrismaTec)")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		if resp.StatusCode != 200 {
			continue
		}
		var data struct {
			Query struct {
				Search []struct {
					Title string `json:"title"`
				} `json:"search"`
			} `json:"query"`
		}
		if json.Unmarshal(body, &data) != nil {
			continue
		}
		qTokens := strings.Fields(strings.ToLower(v))
		bestTitle, bestScore := "", -1
		for _, hit := range data.Query.Search {
			if hit.Title == "" {
				continue
			}
			low := strings.ToLower(hit.Title)
			if strings.Contains(low, "desambiguación") || strings.Contains(low, "disambiguation") {
				continue
			}
			sc := 0
			for _, tok := range qTokens {
				if len(tok) < 2 {
					continue
				}
				if strings.Contains(low, tok) {
					sc += 2 + len(tok)/3
				}
			}
			// prefer exact-ish multi-word match
			if strings.Contains(low, strings.ToLower(v)) {
				sc += 10
			}
			if sc > bestScore {
				bestScore, bestTitle = sc, hit.Title
			}
		}
		if bestTitle != "" && bestScore > 0 {
			return bestTitle
		}
		// if no token overlap, still take first non-disambig only when single token query
		if len(qTokens) <= 1 && len(data.Query.Search) > 0 {
			for _, hit := range data.Query.Search {
				low := strings.ToLower(hit.Title)
				if hit.Title != "" && !strings.Contains(low, "desambiguación") && !strings.Contains(low, "disambiguation") {
					return hit.Title
				}
			}
		}
	}
	return ""
}

// resolveWikipediaTitle: ES first, then EN (title only; summary prefers ES).
func resolveWikipediaTitle(topic string) string {
	if t := resolveWikipediaTitleOn(topic, "es.wikipedia.org"); t != "" {
		return t
	}
	if t := resolveWikipediaTitleOn(topic, "en.wikipedia.org"); t != "" {
		return t
	}
	return wikiTitle(topic)
}

// fetchWikipediaSummaryLang fetches REST summary from one language wiki.
func fetchWikipediaSummaryLang(title, langHost string) (outTitle, extract, pageURL string, ok bool) {
	if title == "" || langHost == "" {
		return "", "", "", false
	}
	slug := url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	api := fmt.Sprintf("https://%s/api/rest_v1/page/summary/%s", langHost, slug)
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return "", "", "", false
	}
	req.Header.Set("User-Agent", "AlsetMind-Scout/1.3 (https://github.com/yecharlot/PrismaTec)")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", "", false
	}
	var data struct {
		Title       string `json:"title"`
		Extract     string `json:"extract"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Lang        string `json:"lang"`
		ContentURLs struct {
			Desktop struct {
				Page string `json:"page"`
			} `json:"desktop"`
		} `json:"content_urls"`
	}
	if json.Unmarshal(body, &data) != nil {
		return "", "", "", false
	}
	extract = strings.TrimSpace(data.Extract)
	if extract == "" {
		extract = strings.TrimSpace(data.Description)
	}
	if extract == "" || data.Type == "disambiguation" {
		return "", "", "", false
	}
	pageURL = data.ContentURLs.Desktop.Page
	if pageURL == "" {
		pageURL = fmt.Sprintf("https://%s/wiki/%s", langHost, slug)
	}
	outTitle = data.Title
	if outTitle == "" {
		outTitle = title
	}
	return outTitle, extract, pageURL, true
}

// fetchWikipediaSummary prioritizes Spanish. Falls back to EN only if ES missing.
func fetchWikipediaSummary(topic string) (title, extract, pageURL string, ok bool) {
	esTitle := resolveWikipediaTitleOn(topic, "es.wikipedia.org")
	if esTitle != "" {
		if t, e, u, hit := fetchWikipediaSummaryLang(esTitle, "es.wikipedia.org"); hit {
			return t, e, u, true
		}
	}
	// try same topic slug on ES without search hit
	if t, e, u, hit := fetchWikipediaSummaryLang(wikiTitle(topic), "es.wikipedia.org"); hit {
		return t, e, u, true
	}
	enTitle := resolveWikipediaTitleOn(topic, "en.wikipedia.org")
	if enTitle == "" {
		enTitle = wikiTitle(topic)
	}
	if t, e, u, hit := fetchWikipediaSummaryLang(enTitle, "en.wikipedia.org"); hit {
		// mark English so voice layer can label it
		return t, e, u, true
	}
	return "", "", "", false
}

// isMostlyEnglish heuristic: if we only have EN wiki, voice will say so.
func isMostlyEnglish(s string) bool {
	low := strings.ToLower(s)
	esHints := []string{" el ", " la ", " de ", " que ", " fue ", " una ", " los ", " las ", " del ", " por ", " con "}
	enHints := []string{" the ", " was ", " and ", " with ", " from ", " that ", " his ", " her ", " is ", " are "}
	esN, enN := 0, 0
	for _, h := range esHints {
		if strings.Contains(low, h) {
			esN++
		}
	}
	for _, h := range enHints {
		if strings.Contains(low, h) {
			enN++
		}
	}
	return enN >= 2 && enN > esN
}

// formatScoutVoice: cuerpo limpio + fuente debajo (sin jerga de laboratorio).
func formatScoutVoice(body, sourceURL string, fromCache bool) string {
	body = strings.TrimSpace(body)
	body = compressVoiceBlock(body, 560)
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n")
	if sourceURL != "" {
		b.WriteString("Fuente: ")
		b.WriteString(sourceURL)
	}
	if fromCache {
		b.WriteString("\n(Recuperado de memoria de sondas.)")
	}
	return strings.TrimSpace(b.String())
}

// fetchDuckDuckGoAnswer: primary professional scout source (Instant Answer API).
// Returns heading, abstract text, source URL. Prefer Abstract over RelatedTopics.
func fetchDuckDuckGoAnswer(topic string) (title, extract, pageURL string, ok bool) {
	q := strings.TrimSpace(topic)
	if q == "" {
		return "", "", "", false
	}
	api := "https://api.duckduckgo.com/?q=" + url.QueryEscape(q) + "&format=json&no_html=1&skip_disambig=1&kl=es-es"
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return "", "", "", false
	}
	req.Header.Set("User-Agent", "AlsetMind-Scout/1.2 (https://github.com/yecharlot/PrismaTec)")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", "", false
	}
	var data struct {
		Heading      string `json:"Heading"`
		Abstract     string `json:"Abstract"`
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Answer       string `json:"Answer"`
		AnswerType   string `json:"AnswerType"`
		Definition   string `json:"Definition"`
		DefinitionURL string `json:"DefinitionURL"`
		RelatedTopics []struct {
			Text string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	if json.Unmarshal(body, &data) != nil {
		return "", "", "", false
	}
	extract = strings.TrimSpace(data.AbstractText)
	if extract == "" {
		extract = strings.TrimSpace(data.Abstract)
	}
	if extract == "" {
		extract = strings.TrimSpace(data.Answer)
	}
	if extract == "" {
		extract = strings.TrimSpace(data.Definition)
	}
	title = strings.TrimSpace(data.Heading)
	pageURL = strings.TrimSpace(data.AbstractURL)
	if pageURL == "" {
		pageURL = strings.TrimSpace(data.DefinitionURL)
	}
	// RelatedTopics as last resort (first non-empty)
	if extract == "" {
		for _, rt := range data.RelatedTopics {
			if t := strings.TrimSpace(rt.Text); len(t) > 40 {
				extract = t
				if pageURL == "" {
					pageURL = strings.TrimSpace(rt.FirstURL)
				}
				if title == "" {
					title = q
				}
				break
			}
		}
	}
	if extract == "" || len(extract) < 40 {
		return "", "", "", false
	}
	if title == "" {
		title = wikiTitle(q)
	}
	if pageURL == "" {
		pageURL = "https://duckduckgo.com/?q=" + url.QueryEscape(q)
	}
	return title, extract, pageURL, true
}

// scoutReportLowQuality: reject disambiguation / empty filler from cache or sources.
func scoutReportLowQuality(report string) bool {
	r := strings.ToLower(strings.TrimSpace(report))
	if len(r) < 40 {
		return true
	}
	bad := []string{
		"desambiguación", "disambiguation", "varias entradas",
		"puede referirse a", "may refer to", "reformula con un nombre",
		"prefers-color-scheme", "function(){", "client-js", "header-wrap",
		"var dc_enabled", "baselinkurl", "set-theme--dark", "@media",
		"html.no-theme", "duckduckgo.com@media",
	}
	for _, b := range bad {
		if strings.Contains(r, b) {
			return true
		}
	}
	return false
}


// tryScoutFollowUp: "cómo se llama su madre" after a scout topic → memory first, then admit gap or re-scout.
func (n *NodoAlset) tryScoutFollowUp(userText string, ethicsState int) string {
	if n == nil || ethicsState == 2 {
		return ""
	}
	low := strings.ToLower(strings.TrimSpace(userText))
	if !isPronounFollowUp(low) {
		return ""
	}
	n.mindLastMu.Lock()
	subj := strings.TrimSpace(n.mindLastScoutTopic)
	lastReport := n.mindLastExplore
	n.mindLastMu.Unlock()
	if subj == "" {
		return ""
	}
	// 1) Memory of sondas for the subject
	if _, prev, ok := recallScoutFinding(subj); ok && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) {
		// answer from known report if it already contains a clue
		if strings.Contains(low, "madre") || strings.Contains(low, "padre") || strings.Contains(low, "esposa") {
			return formatScoutVoice(
				"Sobre «"+subj+"» tengo este resumen, pero no incluye el nombre que pides:\n\n"+prev+
					"\n\nPuedo buscar «"+followUpSearchTopic(low, subj)+"» si quieres que lance otra sonda.",
				"", true)
		}
		return formatScoutVoice(
			"Seguimos con «"+subj+"». De lo ya explorado:\n\n"+compressVoiceBlock(prev, 400)+
				"\n\nSi quieres otro detalle concreto, dilo (lugar de nacimiento, equipo, obra…).",
			"", true)
	}
	if lastReport != "" && !scoutReportLowQuality(lastReport) {
		return formatScoutVoice(
			"Seguimos con «"+subj+"». Del último hallazgo:\n\n"+compressVoiceBlock(lastReport, 400)+
				"\n\nEse resumen no responde del todo a «"+strings.TrimSpace(userText)+"». Puedo sondear «"+followUpSearchTopic(low, subj)+"».",
			"", true)
	}
	// 2) New probe with composed topic
	composed := followUpSearchTopic(low, subj)
	if composed == "" {
		return ""
	}
	// reuse MindScoutWeb path via synthetic question
	return n.MindScoutWeb("quién es "+composed, ethicsState)
}

func followUpSearchTopic(follow, subject string) string {
	follow = strings.ToLower(follow)
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ""
	}
	if strings.Contains(follow, "madre") {
		return "madre de " + subject
	}
	if strings.Contains(follow, "padre") {
		return "padre de " + subject
	}
	if strings.Contains(follow, "esposa") || strings.Contains(follow, "mujer") {
		return "esposa de " + subject
	}
	if strings.Contains(follow, "nació") || strings.Contains(follow, "nacio") {
		return "nacimiento de " + subject
	}
	if strings.Contains(follow, "equipo") {
		return subject + " equipo"
	}
	return subject
}

// MindScoutWeb: A topic → B resolve → D recall → explore → store.
func (n *NodoAlset) MindScoutWeb(userText string, ethicsState int) string {
	if ethicsState == 2 || n == nil {
		return ""
	}
	if !isScoutableQuestion(normalizeUserInput(userText)) {
		return ""
	}
	norm := normalizeUserInput(userText)
	if !forceWebScout(norm) && speakFromKnowledge(userText) != "" {
		return ""
	}
	topic := extractTopic(norm)
	if topic == "" || len(topic) < 2 {
		return ""
	}

	// D — reuse prior scout finding for the same topic
	if tKey, prev, hit := recallScoutFinding(topic); hit {
		nt := normalizeTopicKey(topic)
		tk := normalizeTopicKey(tKey)
		same := nt == tk || strings.Contains(tk, nt) || strings.Contains(nt, tk)
		if same && !scoutReportLowQuality(prev) && !isMostlyEnglish(prev) {
			return formatScoutVoice(prev, "", true)
		}
		// EN-only cache → re-scout seeking ES source
	}

	slug := strings.ToLower(topic)
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, slug)
	if len(slug) > 24 {
		slug = slug[:24]
	}
	key := "scout-" + strings.Trim(slug, "-")
	if key == "scout-" || key == "scout" {
		key = fmt.Sprintf("scout-%d", time.Now().Unix()%100000)
	}

	_, _ = n.CreateAlsetGen(key, "", "scout", "sonda temporal: "+topic)

	var report, sourceURL string
	langNote := ""

	// 1) Wikipedia ES primero (voz en español)
	if title, extract, pageURL, ok := fetchWikipediaSummary(topic); ok && !scoutReportLowQuality(extract) {
		sourceURL = pageURL
		report = strings.TrimSpace(title + ". " + extract)
		if isMostlyEnglish(report) {
			langNote = "Resumen disponible solo en inglés (no hay artículo ES fiable)."
		}
		report = compressVoiceBlock(report, 520)
	}

	// 2) DuckDuckGo Instant Answer (región es-es)
	if report == "" {
		if title, extract, pageURL, ok := fetchDuckDuckGoAnswer(topic); ok && !scoutReportLowQuality(extract) {
			sourceURL = pageURL
			report = strings.TrimSpace(title + ". " + extract)
			if isMostlyEnglish(report) {
				langNote = "Resumen en inglés (Instant Answer)."
			}
			report = compressVoiceBlock(report, 520)
		}
	}

	// 3) Explore gen solo a página Wikipedia ES (evitar HTML de buscadores)
	if report == "" {
		u := wikipediaURL(topic)
		sourceURL = u
		res := n.ExploreRemoteGen(key, u, "scout:"+topic)
		snippet, title := "", ""
		if res != nil {
			if s, ok := res["snippet"].(string); ok {
				snippet = cleanWebSnippet(s)
			}
			if t, ok := res["title"].(string); ok {
				title = t
			}
			if err, ok := res["error"].(string); ok && err != "" && snippet == "" {
				_ = n.DeleteAlsetGen(key)
				return "No encontré una fuente fiable en español para «" + topic + "». Prueba el nombre completo."
			}
		}
		report = strings.TrimSpace(strings.TrimSpace(title) + ". " + snippet)
		if report == "." || len(report) < 40 || cleanWebSnippet(report) == "" || scoutReportLowQuality(report) {
			_ = n.DeleteAlsetGen(key)
			return "No encontré una fuente fiable en español para «" + topic + "». Prueba con más contexto."
		}
		report = compressVoiceBlock(report, 420)
	}

	if langNote != "" {
		report = report + "\n\n" + langNote
	}

	if !scoutReportLowQuality(report) {
		storeScoutFinding(topic, report)
	}

	learnText := fmt.Sprintf("hallazgo sonda %s sobre %s: %s", key, topic, report)
	if _, g, err := n.SaveTextToMemoryGen(key, learnText, "scout_finding"); err == nil && g != nil {
		if len(g.EpisodeCIDs) > 0 {
			_, _ = n.PinCIDToMemoryGen("mem-nodo", g.EpisodeCIDs[len(g.EpisodeCIDs)-1], "from_scout")
		}
	} else {
		_, _ = n.CreateMemoryGen("mem-nodo", "memoria de sondas")
		_, _, _ = n.SaveTextToMemoryGen("mem-nodo", learnText, "scout_finding")
	}

	n.rememberThreadRefs("explore", key, report, "")
	n.mindLastMu.Lock()
	n.mindLastScoutTopic = topic
	n.mindLastMu.Unlock()

	voice := formatScoutVoice(report, sourceURL, false)
	if scoutEphemeral() {
		_ = n.DeleteAlsetGen(key)
	}
	return voice
}


func cleanWebSnippet(s string) string {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if strings.Contains(low, "function(){") || strings.Contains(low, "client-js") ||
		strings.Contains(low, "vector-feature-") || strings.Contains(low, "prefers-color-scheme") ||
		strings.Contains(low, "header-wrap") || strings.Contains(low, "var dc_enabled") ||
		strings.Contains(low, "@media") || strings.Contains(low, "baselinkurl") {
		if i := strings.Index(s, "(function"); i > 40 {
			s = strings.TrimSpace(s[:i])
		} else {
			return ""
		}
	}
	// drop residual CSS/JS lines
	if scoutReportLowQuality(s) {
		return ""
	}
	return s
}
