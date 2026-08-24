package node

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"
)

// Scout: Mind uses a temporary (or named) gen as a probe to the web when corpus has no answer.
// Findings stay on the gen; Mind integrates a compressed report into the user voice and may learn (CID).

func scoutEphemeral() bool {
	v := strings.ToLower(os.Getenv("ALSET_SCOUT_EPHEMERAL"))
	if v == "0" || v == "false" || v == "no" {
		return false
	}
	return true // default: delete temp scout after use
}

func isScoutableQuestion(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if isGenToolIntent(s) || isCodeGenRequest(s) || isCapabilityQuestion(s) {
		return false
	}
	if isPersonalFact(s) || isMemoryQuery(s) || isIdentityTalk(s) || isCalmChat(s) {
		return false
	}
	// factual / open info requests
	if strings.HasPrefix(s, "qué es ") || strings.HasPrefix(s, "que es ") ||
		strings.HasPrefix(s, "quién es ") || strings.HasPrefix(s, "quien es ") ||
		strings.HasPrefix(s, "quién fue ") || strings.HasPrefix(s, "quien fue ") ||
		strings.HasPrefix(s, "dónde queda ") || strings.HasPrefix(s, "donde queda ") ||
		strings.HasPrefix(s, "busca ") || strings.HasPrefix(s, "investiga ") ||
		strings.Contains(s, "en internet") || strings.Contains(s, "en la web") {
		return true
	}
	if strings.Contains(s, "?") && len(s) > 12 && speakFromKnowledge(s) == "" {
		return true
	}
	return false
}

// forceWebScout: user explicitly wants the web; do not skip for corpus keyword hits (e.g. "harry potter" humor entry).
func forceWebScout(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "busca ") || strings.HasPrefix(s, "investiga ") ||
		strings.HasPrefix(s, "quién es ") || strings.HasPrefix(s, "quien es ") ||
		strings.HasPrefix(s, "quién fue ") || strings.HasPrefix(s, "quien fue ") ||
		strings.Contains(s, "en internet") || strings.Contains(s, "en la web") {
		return true
	}
	return false
}

func topicFromQuestion(s string) string {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	for _, pfx := range []string{"qué es ", "que es ", "quién es ", "quien es ", "quién fue ", "quien fue ", "busca ", "investiga ", "dónde queda ", "donde queda "} {
		if strings.HasPrefix(low, pfx) {
			return strings.TrimSpace(s[len(pfx):])
		}
	}
	// strip trailing ?
	s = strings.TrimSuffix(s, "?")
	return strings.TrimSpace(s)
}

func wikipediaURL(topic string) string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return ""
	}
	return "https://es.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(wikiTitle(topic), " ", "_"))
}

// wikiTitle: mild title-case so Wikipedia resolves "harry potter" → useful page.
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

// fetchWikipediaSummary uses the public REST API (plain text extract, no HTML chrome).
func fetchWikipediaSummary(topic string) (title, extract, pageURL string, ok bool) {
	titleHint := wikiTitle(topic)
	slug := url.PathEscape(strings.ReplaceAll(titleHint, " ", "_"))
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
		req.Header.Set("User-Agent", "AlsetMind-Scout/1.0 (https://github.com/yecharlot/PrismaTec; contact via repo)")
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
			extract = "Varias entradas en Wikipedia; reformula el nombre más específico. " + extract
		}
		pageURL = data.ContentURLs.Desktop.Page
		if pageURL == "" {
			pageURL = wikipediaURL(topic)
		}
		return data.Title, extract, pageURL, true
	}
	return "", "", "", false
}

// MindScoutWeb creates/uses a gen, explores, learns from findings, optional delete.
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
	topic := topicFromQuestion(normalizeUserInput(userText))
	if topic == "" || len(topic) < 2 {
		return ""
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
				return fmt.Sprintf("Envié la sonda «%s» a la web, pero no trajo un informe usable (%s).", normalizeGenKey(key), err)
			}
		}
		report = strings.TrimSpace(title + " — " + snippet)
		if report == "—" || len(report) < 20 {
			report = fmt.Sprintf("La sonda visitó %s sin extraer un resumen claro. Prueba otro nombre o «investiga …».", u)
		} else {
			report = compressVoiceBlock(report, 420)
		}
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

	voice := fmt.Sprintf("No lo tenía en corpus. Despaché la sonda «%s» a la web (%s).\n\n%s\n\nEl hallazgo queda anclado en memoria de sondas.",
		normalizeGenKey(key), sourceURL, report)

	if scoutEphemeral() {
		_ = n.DeleteAlsetGen(key)
		voice += "\n(Sonda temporal eliminada tras informar.)"
	}
	return voice
}

// cleanWebSnippet drops obvious HTML/JS chrome left by crude page scrapes.
func cleanWebSnippet(s string) string {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if strings.Contains(low, "function(){") || strings.Contains(low, "client-js") ||
		strings.Contains(low, "vector-feature-") {
		// Prefer text before the script noise if any
		if i := strings.Index(s, "(function"); i > 40 {
			s = strings.TrimSpace(s[:i])
		} else {
			return ""
		}
	}
	return s
}
