package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// LearnedPattern — corrección determinista aprendida de fallos reales (no LLM).
// Ej.: "si la voz tenía html_css_junk en ruta web → preferir solo wiki ES".
type LearnedPattern struct {
	Code       string    `json:"code"`
	Count      int       `json:"count"`
	LastSeen   string    `json:"last_seen"`
	SampleQuery string   `json:"sample_query,omitempty"`
	Correction string    `json:"correction"` // machine-readable policy flag
}

const patternsFile = "mind_learned_patterns.json"
const patternsMax = 80

var (
	patternsMu   sync.RWMutex
	patternsRing []LearnedPattern
)

var (
	reQueryWho = regexp.MustCompile(`(?i)^\s*(qui[eé]n\s+(es|fue)|que\s+es|qu[eé]\s+es)\b`)
	reQueryMath = regexp.MustCompile(`(?i)(cu[aá]nto\s+es|suma\s+\d)`)
	reQueryWrite = regexp.MustCompile(`(?i)(escribe\s+(un\s+)?(poema|cuento|historia)|redacta|comp[oó]n)`)
)

// loadLearnedPatterns from disk (bootstrap).
func loadLearnedPatterns() {
	path := filepath.Join("alset_data", patternsFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var list []LearnedPattern
	if json.Unmarshal(raw, &list) != nil {
		return
	}
	patternsMu.Lock()
	patternsRing = list
	if len(patternsRing) > patternsMax {
		patternsRing = patternsRing[len(patternsRing)-patternsMax:]
	}
	patternsMu.Unlock()
}

func persistLearnedPatterns() {
	patternsMu.RLock()
	snap := append([]LearnedPattern(nil), patternsRing...)
	patternsMu.RUnlock()
	_ = os.MkdirAll("alset_data", 0o755)
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join("alset_data", patternsFile), raw, 0o644)
}

// recordVoiceAnomalies learns from bad voices and strengthens policies.
func recordVoiceAnomalies(query, voice string) {
	hits := DetectVoiceAnomalies(voice)
	if len(hits) == 0 {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	patternsMu.Lock()
	defer patternsMu.Unlock()
	for _, h := range hits {
		corr := correctionForCode(h.Code)
		found := false
		for i := range patternsRing {
			if patternsRing[i].Code == h.Code {
				patternsRing[i].Count++
				patternsRing[i].LastSeen = now
				patternsRing[i].SampleQuery = compressVoiceBlock(query, 80)
				patternsRing[i].Correction = corr
				found = true
				break
			}
		}
		if !found {
			patternsRing = append(patternsRing, LearnedPattern{
				Code:        h.Code,
				Count:       1,
				LastSeen:    now,
				SampleQuery: compressVoiceBlock(query, 80),
				Correction:  corr,
			})
		}
	}
	if len(patternsRing) > patternsMax {
		patternsRing = patternsRing[len(patternsRing)-patternsMax:]
	}
	snap := append([]LearnedPattern(nil), patternsRing...)
	go func() {
		_ = os.MkdirAll("alset_data", 0o755)
		raw, _ := json.MarshalIndent(snap, "", "  ")
		_ = os.WriteFile(filepath.Join("alset_data", patternsFile), raw, 0o644)
	}()
}

func correctionForCode(code string) string {
	switch code {
	case "html_css_junk":
		return "prefer_wiki_es_only"
	case "english_only":
		return "reject_en_summaries"
	case "soft_memory_hijack", "raw_scout_echo":
		return "block_scout_as_personal"
	case "disambiguation":
		return "skip_disambig_pages"
	case "empty_summary_body":
		return "require_nonempty_body"
	case "lab_jargon":
		return "strip_lab_voice"
	case "empty_voice":
		return "fallback_chat"
	default:
		return "log_only"
	}
}

// policyFlag true if learned count exceeds threshold for this correction.
func policyFlag(flag string, minCount int) bool {
	if minCount <= 0 {
		minCount = 2
	}
	patternsMu.RLock()
	defer patternsMu.RUnlock()
	for _, p := range patternsRing {
		if p.Correction == flag && p.Count >= minCount {
			return true
		}
	}
	return false
}

// applyLearnedPolicies mutates/filters a candidate scout report before store/voice.
func applyLearnedPolicies(report, sourceURL string) (string, string, bool) {
	if report == "" {
		return "", sourceURL, false
	}
	if policyFlag("prefer_wiki_es_only", 1) && strings.Contains(strings.ToLower(sourceURL), "duckduckgo") {
		if scoutReportLowQuality(report) || reHTMLCSSJunk.MatchString(report) {
			return "", sourceURL, false
		}
	}
	if policyFlag("reject_en_summaries", 1) && isMostlyEnglish(report) {
		return "", sourceURL, false
	}
	if policyFlag("skip_disambig_pages", 1) && reDisambig.MatchString(report) {
		return "", sourceURL, false
	}
	if policyFlag("require_nonempty_body", 1) && reEmptyBody.MatchString(report) {
		return "", sourceURL, false
	}
	if DetectVoiceAnomalies(report) != nil {
		// still filter current anomalies even before learning threshold
		for _, h := range DetectVoiceAnomalies(report) {
			if h.Code == "html_css_junk" || h.Code == "disambiguation" {
				return "", sourceURL, false
			}
		}
	}
	return report, sourceURL, true
}

// speakFromPatterns — Mind explica qué ha aprendido de sus errores.
func speakFromPatterns(query string) string {
	q := foldSpanish(strings.ToLower(query))
	if !(strings.Contains(q, "que aprendiste") || strings.Contains(q, "patrones") ||
		strings.Contains(q, "como te corriges") || strings.Contains(q, "auto correccion") ||
		strings.Contains(q, "autocorreccion") || strings.Contains(q, "que fallos")) {
		return ""
	}
	patternsMu.RLock()
	defer patternsMu.RUnlock()
	if len(patternsRing) == 0 {
		return "Aún no tengo patrones de corrección registrados. Cuando una respuesta salga anómala (HTML, inglés forzado, eco de sonda…), la anoto y endurezco la política."
	}
	var b strings.Builder
	b.WriteString("Patrones de auto-corrección (aprendidos de fallos reales):\n")
	for _, p := range patternsRing {
		if p.Count < 1 {
			continue
		}
		b.WriteString("- ")
		b.WriteString(p.Code)
		b.WriteString(" ×")
		b.WriteString(itoa(p.Count))
		b.WriteString(" → política ")
		b.WriteString(p.Correction)
		if p.SampleQuery != "" {
			b.WriteString(" (ej. «")
			b.WriteString(compressVoiceBlock(p.SampleQuery, 40))
			b.WriteString("»)")
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
