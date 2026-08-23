package gennode

import (
	"fmt"
	"strings"
)

// Dialogue answers Mind (or any peer) about identity, findings, and package knowledge.
func (d *Daemon) Dialogue(stimulus string) map[string]interface{} {
	s := strings.ToLower(strings.TrimSpace(stimulus))
	d.mu.Lock()
	findings := append([]map[string]interface{}{}, d.findings...)
	nPulse := len(d.pulses)
	d.mu.Unlock()

	voice := d.composeVoice(s, findings)
	return map[string]interface{}{
		"ok":             true,
		"key":            d.Pkg.Key,
		"voice":          voice,
		"findings_count": len(findings),
		"pulses_received": nPulse,
		"root_cid":       d.Pkg.CurrentRootCID,
		"mode":           "autonomous-daemon",
	}
}

func (d *Daemon) composeVoice(s string, findings []map[string]interface{}) string {
	key := d.Pkg.Key
	root := d.Pkg.CurrentRootCID
	if len(root) > 18 {
		root = root[:18] + "…"
	}

	if s == "" || strings.Contains(s, "quién eres") || strings.Contains(s, "quien eres") ||
		strings.Contains(s, "quién sos") || strings.Contains(s, "identidad") {
		return fmt.Sprintf(
			"Soy Alset-Gen %s en modo daemon autónomo. Root %s. Hallazgos %d. No dependo del monolito para existir; Mind puede localizarme si me anuncié.",
			key, root, len(findings),
		)
	}

	if strings.Contains(s, "qué sabes") || strings.Contains(s, "que sabes") ||
		strings.Contains(s, "hallazgo") || strings.Contains(s, "explor") ||
		strings.Contains(s, "viste") || strings.Contains(s, "informe") ||
		strings.Contains(s, "qué encontr") || strings.Contains(s, "que encontr") {
		if len(findings) == 0 {
			return "Aún no tengo hallazgos de frontera. Pide explorar una URL pública y los guardaré aquí."
		}
		var parts []string
		parts = append(parts, fmt.Sprintf("Tengo %d hallazgo(s) de frontera:", len(findings)))
		start := 0
		if len(findings) > 3 {
			start = len(findings) - 3
		}
		for _, f := range findings[start:] {
			u, _ := f["url"].(string)
			title, _ := f["title"].(string)
			snip, _ := f["snippet"].(string)
			st, _ := f["status"]
			line := fmt.Sprintf("- %s (status %v)", u, st)
			if title != "" {
				line += " · " + title
			}
			if snip != "" {
				if len(snip) > 100 {
					snip = snip[:100] + "…"
				}
				line += " · «" + snip + "»"
			}
			parts = append(parts, line)
		}
		return strings.Join(parts, "\n")
	}

	if strings.Contains(s, "estado") || strings.Contains(s, "status") || strings.Contains(s, "info") {
		return fmt.Sprintf("Estado: key=%s root=%s hallazgos=%d modo=daemon.", key, root, len(findings))
	}

	// Default: acknowledge and offer knowledge
	if len(findings) > 0 {
		last := findings[len(findings)-1]
		u, _ := last["url"].(string)
		return fmt.Sprintf(
			"Semilla %s recibió: «%s». Última frontera observada: %s. Pregunta por hallazgos, identidad o estado.",
			key, truncate(s, 80), u,
		)
	}
	return fmt.Sprintf(
		"Semilla %s en daemon. Root %s. Puedo explorar URLs, guardar hallazgos y responder a Mind. ¿Qué necesitas?",
		key, root,
	)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
