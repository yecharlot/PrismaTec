package node

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed embedded/mind_knowledge.json
var embeddedKnowledgeJSON []byte

type mindKnowledgeEntry struct {
	Type string   `json:"type"`
	Keys []string `json:"keys"`
	Text string   `json:"text"`
}

var (
	mindKnowledgeCache []mindKnowledgeEntry
	mindKnowledgeMu    sync.RWMutex
	liveKnowledgeFile  = "mind_knowledge_live.json"
)

func loadMindKnowledge() []mindKnowledgeEntry {
	mindKnowledgeMu.RLock()
	if len(mindKnowledgeCache) > 0 {
		out := mindKnowledgeCache
		mindKnowledgeMu.RUnlock()
		return out
	}
	mindKnowledgeMu.RUnlock()

	mindKnowledgeMu.Lock()
	defer mindKnowledgeMu.Unlock()
	if len(mindKnowledgeCache) > 0 {
		return mindKnowledgeCache
	}
	var entries []mindKnowledgeEntry
	if len(embeddedKnowledgeJSON) > 0 {
		_ = json.Unmarshal(embeddedKnowledgeJSON, &entries)
	}
	// Live corpus from sondas (alset_data) — amplía el corpus sin LLM
	livePath := filepath.Join("alset_data", liveKnowledgeFile)
	if raw, err := os.ReadFile(livePath); err == nil && len(raw) > 0 {
		var live []mindKnowledgeEntry
		if json.Unmarshal(raw, &live) == nil {
			entries = append(entries, live...)
		}
	}
	mindKnowledgeCache = entries
	return mindKnowledgeCache
}

// promoteScoutToKnowledge writes a durable ES finding into the live corpus.
func promoteScoutToKnowledge(topic, report string) {
	topic = strings.TrimSpace(strings.ToLower(topic))
	report = strings.TrimSpace(report)
	if topic == "" || len(report) < 60 || scoutReportLowQuality(report) || isMostlyEnglish(report) {
		return
	}
	entry := mindKnowledgeEntry{
		Type: "scout_live",
		Keys: []string{topic, "quién es " + topic, "quien es " + topic},
		Text: report,
	}
	dir := "alset_data"
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, liveKnowledgeFile)

	mindKnowledgeMu.Lock()
	defer mindKnowledgeMu.Unlock()
	var live []mindKnowledgeEntry
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &live)
	}
	// upsert by primary key
	found := false
	for i := range live {
		for _, k := range live[i].Keys {
			if strings.EqualFold(k, topic) {
				live[i] = entry
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		live = append(live, entry)
	}
	// ring cap
	if len(live) > 200 {
		live = live[len(live)-200:]
	}
	if raw, err := json.MarshalIndent(live, "", "  "); err == nil {
		_ = os.WriteFile(path, raw, 0o644)
	}
	// invalidate cache so next load merges
	mindKnowledgeCache = nil
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
		{"quie es", "quién es"},
		{"mision", "misión"},
		{"cuantos genes", "cuántos genes"},
		{"cuanto es", "cuánto es"},
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
				sc += 20 + len([]rune(k))
			} else if k == "cid" && (strings.Contains(q, " cid") || strings.HasSuffix(q, " cid") || q == "cid" || strings.Contains(q, "que es cid") || strings.Contains(q, "qué es cid") || strings.Contains(q, "significa cid")) {
				sc += 30
			} else if len(strings.Fields(q)) == 1 && len(strings.Fields(k)) > 0 && q == strings.Fields(k)[0] {
				sc += 18
			} else if len([]rune(k)) >= 5 && strings.Contains(q, k) {
				// Prefer longer keys so "gil python" beats bare "python"
				// keys < 5 chars (e.g. "cond") must NOT match inside "mitocondria"
				sc += 5 + len([]rune(k))/2
			} else if len([]rune(q)) >= 5 && strings.Contains(k, q) {
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
		techShort := map[string]bool{"cid": true, "gen": true, "ans": true, "ipfs": true, "utxo": true, "zkp": true}
		for _, w := range tokenizeMind(q) {
			if weakTok[w] {
				continue
			}
			if len([]rune(w)) < 4 && !techShort[w] {
				continue
			}
			for _, k := range e.Keys {
				kl := strings.ToLower(k)
				if kl == w || strings.Contains(" "+kl+" ", " "+w+" ") || strings.HasPrefix(kl, w+" ") || strings.HasSuffix(kl, " "+w) {
					sc += 8
				} else if len([]rune(w)) >= 4 && strings.Contains(kl, w) {
					sc += 2
				}
			}
			if len([]rune(w)) >= 4 && strings.Contains(e.Type, w) {
				sc++
			}
		}
		// Fuzzy only for single-token queries (typos like npl)
		// Fuzzy solo palabras ≥4: evita mar↔car, gen↔gel, etc.
		if len(strings.Fields(q)) == 1 {
			w := strings.Fields(q)[0]
			if len([]rune(w)) >= 4 && !weakTok[w] {
				for _, k := range e.Keys {
					for _, kw := range strings.Fields(strings.ToLower(k)) {
						if len([]rune(kw)) >= 4 && editDistanceOne(w, kw) {
							sc += 4
						}
					}
				}
			}
		}
		// Identidad de especie: boost fuerte
		if (strings.Contains(q, "alset mind") || q == "alset mind" || q == "quién eres" || q == "quien eres") &&
			(e.Type == "identidad" || e.Type == "dialogo") {
			for _, k := range e.Keys {
				kl := strings.ToLower(k)
				if strings.Contains(kl, "alset mind") || strings.Contains(kl, "quién eres") || strings.Contains(kl, "quien eres") {
					sc += 25
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
			if k != "" && len([]rune(k)) >= 4 && (strings.Contains(q, k) || strings.Contains(k, q) || strings.Contains(strings.ToLower(lastKnow), k)) {
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
