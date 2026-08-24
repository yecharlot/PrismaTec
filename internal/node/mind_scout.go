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

	// Stopwords (keep content words)
	stop := map[string]bool{
		"busca": true, "información": true, "informacion": true, "info": true,
		"sobre": true, "que": true, "qué": true, "es": true, "la": true, "el": true,
		"los": true, "las": true, "un": true, "una": true, "de": true, "del": true,
		"quien": true, "quién": true, "quienes": true, "quiénes": true, "fue": true,
		"en": true, "web": true, "internet": true, "mas": true, "más": true,
		"profundiza": true, "investiga": true, "dime": true, "cuéntame": true, "cuentame": true,
	}
	var kept []string
	for _, w := range strings.Fields(s) {
		w = strings.Trim(w, ".,;:¡!¿\"'")
		if w == "" || stop[w] {
			continue
		}
		kept = append(kept, w)
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
		{"juana darc", "juana de arco"},
		{"bio informatica", "bioinformática"},
		{"bio-informatica", "bioinformática"},
		{"bioinformatica", "bioinformática"},
		{"lebrom", "lebron"},
		{"martin luther king", "martin luther king"},
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

// resolveWikipediaTitle: B — search API → canonical page title.
func resolveWikipediaTitle(topic string) string {
	q := strings.TrimSpace(topic)
	if q == "" {
		return ""
	}
	hosts := []string{"es.wikipedia.org", "en.wikipedia.org"}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, host := range hosts {
		api := fmt.Sprintf("https://%s/w/api.php?action=query&list=search&srsearch=%s&srlimit=1&format=json",
			host, url.QueryEscape(q))
		req, err := http.NewRequest(http.MethodGet, api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "AlsetMind-Scout/1.1 (https://github.com/yecharlot/PrismaTec)")
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
		if len(data.Query.Search) > 0 && data.Query.Search[0].Title != "" {
			return data.Query.Search[0].Title
		}
	}
	return wikiTitle(topic)
}

func fetchWikipediaSummary(topic string) (title, extract, pageURL string, ok bool) {
	resolved := resolveWikipediaTitle(topic)
	if resolved == "" {
		resolved = wikiTitle(topic)
	}
	slug := url.PathEscape(strings.ReplaceAll(resolved, " ", "_"))
	hosts := []string{
		"https://es.wikipedia.org/api/rest_v1/page/summary/",
		"https://en.wikipedia.org/api/rest_v1/page/summary/",
	}
	client := &http.Client{Timeout: 12 * time.Second}
	for _, host := range hosts {
		req, err := http.NewRequest(http.MethodGet, host+slug, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "AlsetMind-Scout/1.1 (https://github.com/yecharlot/PrismaTec)")
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
			Title       string `json:"title"`
			Extract     string `json:"extract"`
			Description string `json:"description"`
			Type        string `json:"type"`
			ContentURLs struct {
				Desktop struct {
					Page string `json:"page"`
				} `json:"desktop"`
			} `json:"content_urls"`
		}
		if json.Unmarshal(body, &data) != nil {
			continue
		}
		extract = strings.TrimSpace(data.Extract)
		if extract == "" && data.Description != "" {
			extract = strings.TrimSpace(data.Description)
		}
		if extract == "" {
			continue
		}
		if data.Type == "disambiguation" {
			extract = "Varias entradas en Wikipedia; reformula con un nombre más específico. " + extract
		}
		pageURL = data.ContentURLs.Desktop.Page
		if pageURL == "" {
			pageURL = wikipediaURL(resolved)
		}
		return data.Title, extract, pageURL, true
	}
	return "", "", "", false
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
		if same {
			return fmt.Sprintf("Desde memoria de sondas («%s»):\n\n%s\n\n(Sin nueva sonda: ya teníamos este hallazgo.)", tKey, prev)
		}
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
	if title, extract, pageURL, ok := fetchWikipediaSummary(topic); ok {
		sourceURL = pageURL
		report = strings.TrimSpace(title + ": " + extract)
		report = compressVoiceBlock(report, 520)
	} else {
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
				return fmt.Sprintf("Envié la sonda «%s» a la web, pero no encontré una página fiable (%s). Reformula el nombre.", normalizeGenKey(key), err)
			}
		}
		report = strings.TrimSpace(title + " — " + snippet)
		if report == "—" || len(report) < 20 || cleanWebSnippet(report) == "" {
			_ = n.DeleteAlsetGen(key)
			return fmt.Sprintf("No encontré una página fiable en Wikipedia para «%s». Prueba otro nombre (ej. «quién es bioinformática»).", topic)
		}
		report = compressVoiceBlock(report, 420)
	}

	storeScoutFinding(topic, report)

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

	voice := fmt.Sprintf("No lo tenía en corpus. Despaché la sonda «%s» a la web (%s).\n\n%s\n\nEl hallazgo queda anclado en memoria de sondas.",
		normalizeGenKey(key), sourceURL, report)

	if scoutEphemeral() {
		_ = n.DeleteAlsetGen(key)
		voice += "\n(Sonda temporal eliminada tras informar.)"
	}
	return voice
}

func cleanWebSnippet(s string) string {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if strings.Contains(low, "function(){") || strings.Contains(low, "client-js") ||
		strings.Contains(low, "vector-feature-") {
		if i := strings.Index(s, "(function"); i > 40 {
			s = strings.TrimSpace(s[:i])
		} else {
			return ""
		}
	}
	return s
}
