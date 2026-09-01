package node

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

// CFT-v0: semilla ternaria de TEXTO/HECHOS (no códec de audio/video).
// Pipeline: texto → embedding determinista (SHA256 bytes) → ternarizar →
// patrones repetidos (RLE) → huella comparable. El texto original sigue
// siendo la fuente de verdad; la semilla es índice y transporte ligero.

// MindSeed es la forma auditable de una "semilla fractal-ternaria" simbólica.
type MindSeed struct {
	// Digits: trayectoria 0/1/2 (post-ternarizar del embedding).
	Digits []int `json:"digits"`
	// Compact: RLE "0x3 1x2 2x1 …" sobre Digits (patrones repetidos).
	Compact string `json:"compact"`
	// Hash: huella corta de Digits (hex).
	Hash string `json:"hash"`
	// Conf: confianza ternaria del sellado (2 = texto suficiente).
	Conf int `json:"conf"`
	// Source: texto normalizado usado (recortado).
	Source string `json:"source,omitempty"`
}

// ternarizeFloat mirrors LispAI (ternarizar): <0.33→0, <0.66→1, else→2.
func ternarizeFloat(f float64) int {
	if f < 0.33 {
		return 0
	}
	if f < 0.66 {
		return 1
	}
	return 2
}

// textEmbedding01 mirrors LispAI embedding: SHA256 bytes / 255.
func textEmbedding01(text string) []float64 {
	sum := sha256.Sum256([]byte(text))
	out := make([]float64, len(sum))
	for i, b := range sum {
		out[i] = float64(b) / 255.0
	}
	return out
}

func normalizeSeedSource(text string) string {
	text = strings.TrimSpace(text)
	// colapsar espacios
	var b strings.Builder
	prevSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.TrimSpace(b.String())
}

// rleTernary: compresión de patrones repetidos sobre alfabeto {0,1,2}.
func rleTernary(digits []int) string {
	if len(digits) == 0 {
		return ""
	}
	var parts []string
	cur, n := digits[0], 1
	for i := 1; i < len(digits); i++ {
		if digits[i] == cur {
			n++
			continue
		}
		parts = append(parts, fmt.Sprintf("%dx%d", cur, n))
		cur, n = digits[i], 1
	}
	parts = append(parts, fmt.Sprintf("%dx%d", cur, n))
	return strings.Join(parts, " ")
}

// SeedFromText builds a reversible-index seed (digits reconstructable; text is separate).
func SeedFromText(text string) MindSeed {
	src := normalizeSeedSource(text)
	if src == "" {
		return MindSeed{Conf: 0}
	}
	emb := textEmbedding01(src)
	digits := make([]int, len(emb))
	for i, f := range emb {
		digits[i] = ternarizeFloat(f)
	}
	// hash of digit string
	var db strings.Builder
	for _, d := range digits {
		db.WriteByte(byte('0' + d))
	}
	h := sha256.Sum256([]byte(db.String()))
	conf := 1
	if len(src) >= 12 {
		conf = 2
	} else if len(src) < 3 {
		conf = 0
	}
	srcOut := src
	if len(srcOut) > 120 {
		srcOut = srcOut[:120] + "…"
	}
	return MindSeed{
		Digits:  digits,
		Compact: rleTernary(digits),
		Hash:    hex.EncodeToString(h[:8]),
		Conf:    conf,
		Source:  srcOut,
	}
}

// SeedSimilarity: proporción de posiciones iguales (0.0–1.0), luego juicio ternario.
// Devuelve (score continuo, juicio 0/1/2).
func SeedSimilarity(a, b MindSeed) (float64, int) {
	if len(a.Digits) == 0 || len(b.Digits) == 0 {
		return 0, 0
	}
	n := len(a.Digits)
	if len(b.Digits) < n {
		n = len(b.Digits)
	}
	same := 0
	for i := 0; i < n; i++ {
		if a.Digits[i] == b.Digits[i] {
			same++
		}
	}
	score := float64(same) / float64(n)
	// umbrales ternarios (distintos del embedding continuo)
	if score < 0.40 {
		return score, 0
	}
	if score < 0.72 {
		return score, 1
	}
	return score, 2
}

// isSeedIntent: usuario pide comprimir / huella / semilla / comparar.
func isSeedIntent(low string) bool {
	keys := []string{
		"comprime este texto", "comprime esto", "comprime:", "comprimir texto",
		"semilla de ", "semilla fractal", "huella ternaria", "huella de ",
		"genera semilla", "generar semilla", "cft",
		"descomprime la semilla", "qué semilla", "compara semilla",
	}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	if strings.HasPrefix(low, "comprime ") || strings.HasPrefix(low, "semilla ") {
		return true
	}
	return false
}

func extractSeedPayload(text string) string {
	low := strings.ToLower(text)
	// after colon or keywords
	for _, sep := range []string{"comprime este texto:", "comprime esto:", "comprime:", "texto:", "semilla de "} {
		if i := strings.Index(low, sep); i >= 0 {
			return strings.TrimSpace(text[i+len(sep):])
		}
	}
	for _, pfx := range []string{"comprime este texto ", "comprime esto ", "comprime ", "semilla de ", "huella de "} {
		if strings.HasPrefix(low, pfx) {
			return strings.TrimSpace(text[len(pfx):])
		}
	}
	return strings.TrimSpace(text)
}

// speakSeed handles CFT-v0 dialogue (compress / show seed).
func speakSeed(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if !isSeedIntent(low) {
		return ""
	}
	if strings.Contains(low, "descomprime") {
		return "La semilla CFT-v0 es índice ternario (huella), no un códec reversible del texto completo. El contenido se recupera del episodio/CID original, no expandiendo solo la semilla."
	}
	payload := extractSeedPayload(text)
	if payload == "" || len(payload) < 2 {
		return "Pasa el texto a sellar: «comprime este texto: …» o «semilla de …»."
	}
	// evitar comprimir el propio comando vacío
	if isSeedIntent(strings.ToLower(payload)) && len(payload) < 24 {
		return "Indica el contenido después del comando."
	}
	s := SeedFromText(payload)
	if s.Conf == 0 {
		return "Texto demasiado corto para una semilla fiable."
	}
	// mostrar compacto + hash (no volcar 32 dígitos enteros al usuario)
	return fmt.Sprintf(
		"Semilla CFT-v0 (texto→ternario, no códec media).\n· hash %s · confianza %d\n· patrones RLE: %s\n· fuente: «%s»\nLa semilla sirve para comparar y transportar huella; el texto sigue siendo la verdad en memoria/CID.",
		s.Hash, s.Conf, s.Compact, s.Source,
	)
}

// bestSeedMatch finds most similar episode text by ternary seed (juicio >= 1).
func bestSeedMatch(query string, episodes []mindEpisodePayload) (match string, score float64, judgment int) {
	q := SeedFromText(query)
	if q.Conf == 0 {
		return "", 0, 0
	}
	bestS := -1.0
	bestT := ""
	bestJ := 0
	for _, ep := range episodes {
		if ep.Text == "" {
			continue
		}
		s, j := SeedSimilarity(q, SeedFromText(ep.Text))
		if s > bestS {
			bestS = s
			bestT = ep.Text
			bestJ = j
		}
	}
	if bestJ < 1 {
		return "", bestS, bestJ
	}
	return bestT, bestS, bestJ
}

func formatSeedScore(score float64) string {
	return fmt.Sprintf("%.0f%%", score*100)
}
