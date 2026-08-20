package node

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	mindEpisodeIndexFile = "mind_episodes.json"
	mindEpisodeMaxKeep   = 32
)

// mindEpisodeIndex is a local ring of recent episode CIDs (efficient, node-local).
type mindEpisodeIndex struct {
	CIDs      []string `json:"cids"`
	UpdatedAt int64    `json:"updated_at"`
}

type mindEpisodePayload struct {
	Type   string                 `json:"type"`
	Text   string                 `json:"text"`
	Signals map[string]float64    `json:"signals"`
	Voice  string                 `json:"voice"`
	TS     string                 `json:"ts"`
	Agent  string                 `json:"agent"`
	Organs []MindOrganResult      `json:"organs"`
}

func (n *NodoAlset) loadMindEpisodeIndex() mindEpisodeIndex {
	var idx mindEpisodeIndex
	b, err := os.ReadFile(mindEpisodeIndexFile)
	if err != nil {
		return idx
	}
	_ = json.Unmarshal(b, &idx)
	return idx
}

func (n *NodoAlset) saveMindEpisodeIndex(idx mindEpisodeIndex) {
	idx.UpdatedAt = time.Now().Unix()
	if len(idx.CIDs) > mindEpisodeMaxKeep {
		idx.CIDs = idx.CIDs[len(idx.CIDs)-mindEpisodeMaxKeep:]
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(mindEpisodeIndexFile, b, 0600)
}

func (n *NodoAlset) appendMindEpisodeCID(cid string) {
	if cid == "" {
		return
	}
	idx := n.loadMindEpisodeIndex()
	// de-dup tail
	if len(idx.CIDs) > 0 && idx.CIDs[len(idx.CIDs)-1] == cid {
		return
	}
	idx.CIDs = append(idx.CIDs, cid)
	n.saveMindEpisodeIndex(idx)
}

// recallRecentEpisodes loads up to limit episodes from local index + blockstore.
func (n *NodoAlset) recallRecentEpisodes(limit int) []mindEpisodePayload {
	if limit <= 0 {
		limit = 5
	}
	idx := n.loadMindEpisodeIndex()
	out := make([]mindEpisodePayload, 0, limit)
	for i := len(idx.CIDs) - 1; i >= 0 && len(out) < limit; i-- {
		cid := idx.CIDs[i]
		raw, err := n.BuscarContenidoPorCID(cid)
		if err != nil || len(raw) == 0 {
			continue
		}
		var ep mindEpisodePayload
		if json.Unmarshal(raw, &ep) != nil {
			continue
		}
		out = append(out, ep)
	}
	return out
}

// biasSignalsFromMemory nudges continuous signals using recent veto/risk history.
// Efficient: O(k) recent episodes, no model training.
func biasSignalsFromMemory(sig map[string]float64, episodes []mindEpisodePayload) (map[string]float64, string) {
	if len(episodes) == 0 {
		return sig, ""
	}
	out := make(map[string]float64, len(sig))
	for k, v := range sig {
		out[k] = v
	}
	vetoStreak := 0
	var lastVetoText string
	for _, ep := range episodes {
		for _, o := range ep.Organs {
			if o.Name == "ethics" && o.State == 2 {
				vetoStreak++
				if lastVetoText == "" {
					lastVetoText = ep.Text
				}
				break
			}
		}
	}
	hint := ""
	if vetoStreak > 0 {
		// slightly raise caution without forcing sumidero on greetings
		out["riesgo"] = clamp01(out["riesgo"] + 0.08*float64(minInt(vetoStreak, 3)))
		out["permiso"] = clamp01(out["permiso"] - 0.05*float64(minInt(vetoStreak, 3)))
		snip := lastVetoText
		if len(snip) > 60 {
			snip = snip[:60] + "…"
		}
		hint = fmt.Sprintf("memoria: %d veto(s) reciente(s); último «%s»", vetoStreak, snip)
	}
	// same-topic reinforcement: if current text echoes a past episode, bump novedad down (deja de ser nuevo)
	return out, hint
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func memoryHintLine(hint string) string {
	if strings.TrimSpace(hint) == "" {
		return ""
	}
	return "—— memoria ——\n" + hint
}
