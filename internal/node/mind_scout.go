package node

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
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
	// Prefer Spanish Wikipedia as Mind speaks Spanish
	return "https://es.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(topic, " ", "_"))
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
	// Corpus keyword hits (e.g. humor "harry potter") must not block explicit web search.
	if !forceWebScout(norm) && speakFromKnowledge(userText) != "" {
		return "" // corpus already knows
	}
	topic := topicFromQuestion(normalizeUserInput(userText))
	if topic == "" || len(topic) < 2 {
		return ""
	}
	u := wikipediaURL(topic)
	if u == "" {
		return ""
	}
	// stable-ish scout name per topic (reuse if still present)
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
	res := n.ExploreRemoteGen(key, u, "scout:"+topic)
	snippet := ""
	title := ""
	if res != nil {
		if s, ok := res["snippet"].(string); ok {
			snippet = s
		}
		if t, ok := res["title"].(string); ok {
			title = t
		}
		if err, ok := res["error"].(string); ok && err != "" && snippet == "" {
			return fmt.Sprintf("Envié la sonda «%s» a la web, pero no trajo un informe usable (%s). Reformula el tema o prueba otra fuente.", normalizeGenKey(key), err)
		}
	}
	report := strings.TrimSpace(title + " — " + snippet)
	if report == "—" || len(report) < 8 {
		report = fmt.Sprintf("La sonda «%s» visitó %s sin extraer texto útil.", normalizeGenKey(key), u)
	} else {
		report = compressVoiceBlock(report, 420)
	}

	// Learn: store as gen_memory note + mind episode ring via SaveTextToMemoryGen on mem-nodo optional
	learnText := fmt.Sprintf("hallazgo sonda %s sobre %s: %s", key, topic, report)
	if _, g, err := n.SaveTextToMemoryGen(key, learnText, "scout_finding"); err == nil && g != nil {
		// also pin on mem-nodo for network memory
		_, _ = n.PinCIDToMemoryGen("mem-nodo", g.EpisodeCIDs[len(g.EpisodeCIDs)-1], "from_scout")
	} else {
		// fallback: generate CID only via episode path on mem gen create
		_, _ = n.CreateMemoryGen("mem-nodo", "memoria de sondas")
		_, _, _ = n.SaveTextToMemoryGen("mem-nodo", learnText, "scout_finding")
	}

	n.rememberThreadRefs("explore", key, report, "")

	voice := fmt.Sprintf("No lo tenía en corpus. Despaché la sonda «%s» a la web (%s).\n\n%s\n\nEsto queda en los hallazgos del gen; puedes preguntarle, retornarlo o eliminarlo.",
		normalizeGenKey(key), u, report)

	if scoutEphemeral() {
		// keep findings learned on mem-nodo; remove temporary scout cell
		_ = n.DeleteAlsetGen(key)
		voice += "\n(Sonda temporal eliminada tras informar; el hallazgo quedó anclado en memoria.)"
	}
	return voice
}
