package node

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed embedded/mind_knowledge.json
var embeddedKnowledgeJSON []byte

type mindKnowledgeEntry struct {
	Type string   `json:"type"`
	Keys []string `json:"keys"`
	Text string   `json:"text"`
}

var mindKnowledgeCache []mindKnowledgeEntry

func loadMindKnowledge() []mindKnowledgeEntry {
	if len(mindKnowledgeCache) > 0 {
		return mindKnowledgeCache
	}
	if len(embeddedKnowledgeJSON) == 0 {
		return nil
	}
	var entries []mindKnowledgeEntry
	if json.Unmarshal(embeddedKnowledgeJSON, &entries) != nil || len(entries) == 0 {
		return nil
	}
	mindKnowledgeCache = entries
	return mindKnowledgeCache
}


// normalizeKnowledgeQuery fixes common typos before corpus overlap.
func normalizeKnowledgeQuery(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	reps := []struct{ a, b string }{
		{"npl", "nlp"},
		{"lnp", "nlp"},
		{"lln", "llm"},
		{"lmm", "llm"},
		{"chatgtp", "chatgpt"},
		{"chatgt", "chatgpt"},
		{"zyron", "zyrion"},
		{"zyron", "zyrion"},
		{"gorutine", "goroutine"},
		{"gorutines", "goroutines"},
		{"microservcio", "microservicio"},
		{"autentificacion", "autenticación"},
		{"conscienca", "consciencia"},
		{"concienica", "consciencia"},
		{"ternaro", "ternario"},
		{"interfaz go", "interface"},
		{"gorutina", "goroutine"},
		{"chanel", "channel"},
		{"promesa js", "promesa"},
		{"tipado", "typescript"},
		{"micro servicio", "microservicio"},
		{"base datos", "base de datos"},

		{"alucinacion", "alucinación"},
	}
	for _, r := range reps {
		q = strings.ReplaceAll(q, r.a, r.b)
	}
	return q
}

func editDistanceOne(a, b string) bool {
	if a == b {
		return true
	}
	ra, rb := []rune(a), []rune(b)
	na, nb := len(ra), len(rb)
	if na > nb {
		ra, rb = rb, ra
		na, nb = nb, na
	}
	if nb-na > 1 {
		return false
	}
	i, j, edits := 0, 0, 0
	for i < na && j < nb {
		if ra[i] != rb[j] {
			edits++
			if edits > 1 {
				return false
			}
			if na == nb {
				i++
			}
			j++
			continue
		}
		i++
		j++
	}
	if j < nb || i < na {
		edits++
	}
	return edits <= 1
}

// speakFromKnowledge retrieves structured polymath knowledge by key/token overlap.
// Not an LLM: fixed curated entries, ranked by match strength.
func speakFromKnowledge(query string) string {
	entries := loadMindKnowledge()
	if len(entries) == 0 {
		return ""
	}
	q := normalizeKnowledgeQuery(query)
	if q == "" {
		return ""
	}
	bestScore := 0
	bestText := ""
	for _, e := range entries {
		sc := 0
		for _, k := range e.Keys {
			k = strings.ToLower(k)
			if k == "" {
				continue
			}
			if q == k {
				sc += 12 + len([]rune(k))/2
			} else if strings.Contains(q, k) {
				// Prefer longer keys so "gil python" beats bare "python"
				sc += 5 + len([]rune(k))/2
			} else if strings.Contains(k, q) && len(q) > 4 {
				sc += 3
			}
		}
		// token overlap with keys and type (skip stopwords / weak fillers)
		weakTok := map[string]bool{
			"estas": true, "estás": true, "esto": true, "esta": true, "este": true,
			"seguro": true, "ser": true, "eres": true, "tiene": true, "tienen": true,
			"como": true, "cómo": true, "qué": true, "que": true, "cual": true, "cuál": true,
			"para": true, "por": true, "con": true, "una": true, "unos": true, "algo": true,
		}
		for _, w := range tokenizeMind(q) {
			if weakTok[w] || len([]rune(w)) < 4 {
				continue
			}
			for _, k := range e.Keys {
				kl := strings.ToLower(k)
				if strings.Contains(kl, w) {
					sc += 2
				}
			}
			if strings.Contains(e.Type, w) {
				sc++
			}
		}
		// Fuzzy only for single-token queries (typos like npl)
		if len(strings.Fields(q)) == 1 {
			w := strings.Fields(q)[0]
			if len([]rune(w)) >= 3 && !weakTok[w] {
				for _, k := range e.Keys {
					for _, kw := range strings.Fields(strings.ToLower(k)) {
						if editDistanceOne(w, kw) {
							sc += 4
						}
					}
				}
			}
		}
		if sc > bestScore {
			bestScore = sc
			bestText = e.Text
		}
	}
	need := 3
	ql := strings.ToLower(strings.TrimSpace(query))
	if len([]rune(ql)) < 28 || strings.Contains(ql, "llm") || strings.Contains(ql, "qué es") || strings.Contains(ql, "que es") {
		need = 2
	}
	if bestScore >= need && bestText != "" {
		return bestText
	}
	return ""
}

// relatedKnowledge finds another corpus entry near the last topic (for "amplía el ángulo").
func relatedKnowledge(lastQuery, lastKnow string) string {
	entries := loadMindKnowledge()
	if len(entries) == 0 || lastKnow == "" {
		return ""
	}
	q := normalizeKnowledgeQuery(lastQuery)
	bestScore := 0
	bestText := ""
	for _, e := range entries {
		if strings.TrimSpace(e.Text) == strings.TrimSpace(lastKnow) {
			continue
		}
		sc := 0
		for _, k := range e.Keys {
			k = strings.ToLower(k)
			if k != "" && (strings.Contains(q, k) || strings.Contains(k, q) || strings.Contains(strings.ToLower(lastKnow), k)) {
				sc += 4 + len([]rune(k))/3
			}
		}
		// same type as best previous match boosts continuity
		for _, w := range tokenizeMind(q + " " + lastKnow) {
			if len(w) < 4 {
				continue
			}
			if strings.Contains(strings.ToLower(e.Text), w) {
				sc += 2
			}
			if strings.Contains(e.Type, w) {
				sc += 2
			}
		}
		if sc > bestScore {
			bestScore = sc
			bestText = e.Text
		}
	}
	if bestScore >= 4 && bestText != "" {
		return bestText
	}
	return ""
}
