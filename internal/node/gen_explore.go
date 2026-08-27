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

	// Wikipedia: REST summary (texto limpio) antes del HTML de la página
	if title, extract, ok := tryWikipediaSummary(safe); ok && extract != "" {
		snippet := extract
		if len([]rune(snippet)) > 320 {
			r := []rune(snippet)
			snippet = string(r[:320]) + "…"
		}
		if title != "" && !strings.Contains(snippet, title) {
			snippet = title + ". " + snippet
		}
		report := map[string]interface{}{
			"mission": mission, "url": safe, "ok": true, "status": 200,
			"title": title, "snippet": snippet, "source": "wikipedia_rest",
		}
		obs, _ := n.ObserveIntoGen(key, "explore:"+mission, snippet)
		n.markGenMission(key, mission, "reported", safe, report)
		out := map[string]interface{}{
			"ok": true, "key": key, "mission": mission, "url": safe,
			"status": 200, "title": title, "snippet": snippet,
			"hallazgo_cid": obs["hallazgo_cid"],
			"note":         "exploración Wikipedia REST (texto limpio)",
		}
		go n.BroadcastPulse("GEN_EXPLORE", map[string]interface{}{
			"key": key, "url": safe, "mission": mission, "status": 200,
		})
		return out, nil
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
	title := extractTitle(string(body))
	snippet := cleanExploreSnippet(title, string(body))

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
	// detail humano: título + snippet limpio (sin volcar status/type a la voz)
	detail := snippet
	if title != "" && !strings.Contains(snippet, title) {
		detail = title + " — " + snippet
	}
	if detail == "" {
		detail = fmt.Sprintf("mission=%s status=%d url=%s", mission, resp.StatusCode, safe)
	}
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
		out["service_note"] = "misión service: informe de frontera + superficie local /work/{gen}/"
		if srv, err := n.DeployGenService(key, "", "", "Servicio · "+key); err == nil {
			out["service"] = srv
		}
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
	low := strings.ToLower(s)
	// Quitar script/style enteros (Wikipedia mete JS en el body)
	for _, tag := range []string{"script", "style", "noscript"} {
		for {
			open := strings.Index(low, "<"+tag)
			if open < 0 {
				break
			}
			close := strings.Index(low[open:], "</"+tag)
			if close < 0 {
				s = s[:open]
				low = strings.ToLower(s)
				break
			}
			end := open + close + len("</"+tag+">")
			if end > len(s) {
				end = len(s)
			}
			s = s[:open] + " " + s[end:]
			low = strings.ToLower(s)
		}
	}
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
	out := b.String()
	// residual JS noise
	for _, junk := range []string{"(function(){", "function(){", "var className=", "client-js", "vector-feature-"} {
		if i := strings.Index(out, junk); i >= 0 {
			// cut from junk if early in string after title-ish content
			if i < 80 {
				out = out[:i]
			}
		}
	}
	return out
}

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// cleanExploreSnippet prioritizes title + readable prose, drops JS leftovers.
func cleanExploreSnippet(title, raw string) string {
	raw = extractMainHTML(raw)
	raw = collapseWS(stripTags(raw))
	for _, junk := range []string{
		"(function()", "function(){", "var className", "mw.loader", "RLCONF", "RLSTATE",
		"Ir al contenido", "Menú principal", "Menu principal", "mover a la barra lateral",
		"ocultar Navegación", "Portal de la comunidad", "Cambios recientes", "Páginas nuevas",
		"Página aleatoria", "Notificar un error", "Páginas especiales", "Crear una cuenta",
		"Herramientas", "Imprimir/exportar", "En otros proyectos", "Buscar Buscar Apariencia",
	} {
		raw = strings.ReplaceAll(raw, junk, " ")
	}
	raw = collapseWS(raw)
	for _, junk := range []string{"(function()", "function(){", "var className", "mw.loader"} {
		if i := strings.Index(raw, junk); i >= 0 {
			raw = strings.TrimSpace(raw[:i])
		}
	}
	if title != "" && (raw == "" || len([]rune(raw)) < 40) {
		return title
	}
	// Prefer text that looks like a definition (contains period, length)
	if len([]rune(raw)) > 320 {
		r := []rune(raw)
		raw = string(r[:320]) + "…"
	}
	return strings.TrimSpace(raw)
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


func tryWikipediaSummary(pageURL string) (title, extract string, ok bool) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(u.Host)
	if !strings.Contains(host, "wikipedia.org") {
		return "", "", false
	}
	// path /wiki/Title
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	var page string
	for i, p := range parts {
		if p == "wiki" && i+1 < len(parts) {
			page = parts[i+1]
			break
		}
	}
	if page == "" || page == "Wiki" {
		return "", "", false
	}
	lang := "en"
	if h := strings.Split(host, "."); len(h) > 0 && len(h[0]) <= 3 {
		lang = h[0]
	}
	api := fmt.Sprintf("https://%s.wikipedia.org/api/rest_v1/page/summary/%s", lang, page)
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return "", "", false
	}
	req.Header.Set("User-Agent", "Alset-Gen/1.0 (+explore; non-invasive)")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", false
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var data struct {
		Title       string `json:"title"`
		Extract     string `json:"extract"`
		Description string `json:"description"`
	}
	if json.Unmarshal(raw, &data) != nil || data.Extract == "" {
		return "", "", false
	}
	return data.Title, data.Extract, true
}

func extractMainHTML(html string) string {
	low := strings.ToLower(html)
	for _, marker := range []string{`id="mw-content-text"`, `class="mw-parser-output"`, `id="bodycontent"`} {
		i := strings.Index(low, marker)
		if i < 0 {
			continue
		}
		// take a window after marker
		start := i
		if start > 0 {
			// find next >
			gt := strings.Index(html[start:], ">")
			if gt >= 0 {
				start = start + gt + 1
			}
		}
		end := start + 12000
		if end > len(html) {
			end = len(html)
		}
		return html[start:end]
	}
	return html
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
