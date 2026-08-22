package node

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxExploreBody = 32 * 1024

// ExploreFrontier sends the gen to observe a public URL without requiring an Alset node landing.
// Non-invasive: GET only, size-limited, SSRF-guarded. Records hallazgo CID + mission report.
func (n *NodoAlset) ExploreFrontier(key, rawURL, mission string) (map[string]interface{}, error) {
	n.ensureGens()
	key = normalizeGenKey(key)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("url requerida")
	}
	if mission == "" {
		mission = "explore"
	}
	mission = strings.ToLower(strings.TrimSpace(mission))

	safe, err := validatePublicExploreURL(rawURL)
	if err != nil {
		return nil, err
	}

	n.mu.RLock()
	_, ok := n.gens[key]
	n.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("gen not found: %s", key)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("demasiados redirects")
			}
			if _, err := validatePublicExploreURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
	req, err := http.NewRequest(http.MethodGet, safe, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Alset-Gen/1.0 (+explore; non-invasive)")
	req.Header.Set("Accept", "text/html,application/json,text/plain,*/*")

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		report := map[string]interface{}{
			"mission": mission, "url": safe, "latency_ms": latency, "ok": false, "error": err.Error(),
		}
		_, _ = n.ObserveIntoGen(key, "explore:error", fmt.Sprintf("%s → %v", safe, err))
		n.markGenMission(key, mission, "failed", safe, report)
		return report, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxExploreBody))
	snippet := collapseWS(stripTags(string(body)))
	if len(snippet) > 400 {
		snippet = snippet[:400] + "…"
	}
	title := extractTitle(string(body))

	report := map[string]interface{}{
		"mission":      mission,
		"url":          safe,
		"latency_ms":   latency,
		"ok":           resp.StatusCode >= 200 && resp.StatusCode < 400,
		"status":       resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"bytes":        len(body),
		"snippet":      snippet,
		"title":        title,
	}
	detail := fmt.Sprintf("mission=%s status=%d type=%s title=%q snippet=%s",
		mission, resp.StatusCode, report["content_type"], title, snippet)
	obs, _ := n.ObserveIntoGen(key, "explore:"+mission, detail)
	n.markGenMission(key, mission, "reported", safe, report)

	out := map[string]interface{}{
		"ok":           true,
		"key":          key,
		"mission":      mission,
		"url":          safe,
		"status":       resp.StatusCode,
		"latency_ms":   latency,
		"title":        title,
		"snippet":      snippet,
		"hallazgo_cid": obs["hallazgo_cid"],
		"note":         "exploración no invasiva (GET acotado); el gen no aterriza ni muta el destino",
	}
	if mission == "service" {
		out["service_note"] = "misión service: registra disponibilidad en la frontera; no ejecuta mutaciones remotas"
	}
	go n.BroadcastPulse("GEN_EXPLORE", map[string]interface{}{
		"key": key, "url": safe, "mission": mission, "status": resp.StatusCode,
	})
	return out, nil
}

func (n *NodoAlset) markGenMission(key, mission, status, frontier string, report map[string]interface{}) {
	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.Unlock()
		return
	}
	if g.State.Metadata == nil {
		g.State.Metadata = map[string]interface{}{}
	}
	g.State.Metadata["mission"] = mission
	g.State.Metadata["mission_status"] = status
	g.State.Metadata["last_frontier"] = frontier
	g.State.Metadata["last_frontier_report"] = report
	g.State.Location = "frontier:" + truncateCID(frontier)
	g.State.LastSeen = time.Now().Unix()
	g.UpdatedAt = g.State.LastSeen
	if g.Organs.Curiosity < 2 {
		g.Organs.Curiosity = 2
	}
	if g.Organs.Mem < 1 {
		g.Organs.Mem = 1
	}
	n.mu.Unlock()
	n.saveGensToDisk()
}

func validatePublicExploreURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("url inválida")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("solo http/https")
	}
	host := u.Hostname()
	low := strings.ToLower(host)
	if low == "localhost" || strings.HasSuffix(low, ".local") || low == "0.0.0.0" {
		return "", fmt.Errorf("host no permitido (SSRF)")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return "", fmt.Errorf("IP privada/local no permitida")
		}
	}
	if strings.Contains(low, "metadata") || low == "169.254.169.254" {
		return "", fmt.Errorf("host no permitido")
	}
	return u.String(), nil
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func extractTitle(html string) string {
	low := strings.ToLower(html)
	i := strings.Index(low, "<title")
	if i < 0 {
		return ""
	}
	rest := html[i:]
	gt := strings.Index(rest, ">")
	if gt < 0 {
		return ""
	}
	start := i + gt + 1
	endRel := strings.Index(strings.ToLower(html[start:]), "</title>")
	if endRel < 0 {
		return ""
	}
	t := collapseWS(html[start : start+endRel])
	if len(t) > 120 {
		t = t[:120]
	}
	return t
}

func (n *NodoAlset) handleGenExplore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key     string `json:"key"`
		URL     string `json:"url"`
		Mission string `json:"mission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	res, err := n.ExploreFrontier(req.Key, req.URL, req.Mission)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}
