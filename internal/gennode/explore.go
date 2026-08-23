package gennode

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxExploreBody = 32 * 1024

// Explore does a non-invasive GET of a public URL and returns a report.
func (d *Daemon) Explore(rawURL, mission string) (map[string]interface{}, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("url requerida")
	}
	if mission == "" {
		mission = "explore"
	}
	safe, err := validatePublicURL(rawURL)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("demasiados redirects")
			}
			if _, err := validatePublicURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
	req, err := http.NewRequest(http.MethodGet, safe, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Alset-Gen-Daemon/1.0 (+explore)")
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		report := map[string]interface{}{
			"ok": false, "url": safe, "mission": mission, "error": err.Error(), "latency_ms": latency,
		}
		d.addFinding(report)
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
		"ok":           resp.StatusCode >= 200 && resp.StatusCode < 400,
		"url":          safe,
		"mission":      mission,
		"status":       resp.StatusCode,
		"title":        title,
		"snippet":      snippet,
		"latency_ms":   latency,
		"content_type": resp.Header.Get("Content-Type"),
		"ts":           time.Now().UTC().Format(time.RFC3339),
		"key":          d.Pkg.Key,
	}
	d.addFinding(report)
	return report, nil
}

func validatePublicURL(raw string) (string, error) {
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
