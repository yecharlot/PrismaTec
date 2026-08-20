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

// speakFromKnowledge retrieves structured polymath knowledge by key/token overlap.
// Not an LLM: fixed curated entries, ranked by match strength.
func speakFromKnowledge(query string) string {
	entries := loadMindKnowledge()
	if len(entries) == 0 {
		return ""
	}
	q := strings.ToLower(strings.TrimSpace(query))
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
			if q == k || strings.Contains(q, k) {
				sc += 5 + len([]rune(k))/4
			} else if strings.Contains(k, q) && len(q) > 4 {
				sc += 3
			}
		}
		// token overlap with keys and type
		for _, w := range tokenizeMind(q) {
			for _, k := range e.Keys {
				if strings.Contains(strings.ToLower(k), w) {
					sc++
				}
			}
			if strings.Contains(e.Type, w) {
				sc++
			}
		}
		if sc > bestScore {
			bestScore = sc
			bestText = e.Text
		}
	}
	if bestScore >= 3 && bestText != "" {
		return bestText
	}
	return ""
}
