package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"redalset/internal/agents"
)

const (
	maxGenEpisodes = 32
	maxGenFindings = 48
)

// genMutateAuthorized: G2 gate — if GEN_MUTATE_SECRET or BOOTSTRAP_SECRET is set, auth_note must match.
// If neither env is set (dev), mutation stays open (G1) but marked open_mode.
func genMutateAuthorized(authNote string) (ok bool, mode string) {
	sec := strings.TrimSpace(os.Getenv("GEN_MUTATE_SECRET"))
	if sec == "" {
		sec = strings.TrimSpace(os.Getenv("BOOTSTRAP_SECRET"))
	}
	if sec == "" {
		return true, "open_dev"
	}
	if authNote != "" && authNote == sec {
		return true, "secret"
	}
	return false, "denied"
}

// ObserveIntoGen records a non-invasive hallazgo (finding) as a content-addressed note on the gen.
// The gen does not alter foreign state; it only appends observation CIDs to its own chain of memory.
func (n *NodoAlset) ObserveIntoGen(key, source, detail string) (map[string]interface{}, error) {
	n.ensureGens()
	key = normalizeGenKey(key)
	if detail == "" {
		detail = source
	}
	payload := map[string]interface{}{
		"type":    "gen_hallazgo",
		"key":     key,
		"source":  source,
		"detail":  detail,
		"ts":      time.Now().UTC().Format(time.RFC3339),
		"species": "Alset-Gen",
	}
	raw, _ := json.Marshal(payload)
	cid, err := n.GenerarCID(raw)
	if err != nil || cid == "" {
		return nil, fmt.Errorf("no se pudo sellar hallazgo")
	}

	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.Unlock()
		return nil, fmt.Errorf("gen not found: %s", key)
	}
	if g.State.Metadata == nil {
		g.State.Metadata = map[string]interface{}{}
	}
	// findings ring in metadata (light index) + episode_cids
	var findings []string
	if prev, ok := g.State.Metadata["findings"].([]interface{}); ok {
		for _, x := range prev {
			if s, ok := x.(string); ok {
				findings = append(findings, s)
			}
		}
	} else if prev, ok := g.State.Metadata["findings"].([]string); ok {
		findings = append(findings, prev...)
	}
	findings = append(findings, cid)
	if len(findings) > maxGenFindings {
		findings = findings[len(findings)-maxGenFindings:]
	}
	g.State.Metadata["findings"] = findings
	g.State.Metadata["last_hallazgo"] = detail
	g.State.Metadata["last_hallazgo_cid"] = cid
	g.EpisodeCIDs = append(g.EpisodeCIDs, cid)
	if len(g.EpisodeCIDs) > maxGenEpisodes {
		g.EpisodeCIDs = g.EpisodeCIDs[len(g.EpisodeCIDs)-maxGenEpisodes:]
	}
	// mild organ nudge: observation raises mem/curiosity without act
	if g.Organs.Mem < 1 {
		g.Organs.Mem = 1
	}
	if g.Organs.Curiosity < 1 {
		g.Organs.Curiosity = 1
	}
	g.State.LastSeen = time.Now().Unix()
	g.UpdatedAt = g.State.LastSeen
	outKey := g.Key
	findCount := len(findings)
	n.mu.Unlock()
	n.saveGensToDisk()
	go n.BroadcastPulse(agents.PulseHallazgo, map[string]interface{}{
		"key": outKey, "cid": cid, "source": source,
	})
	return map[string]interface{}{
		"ok":           true,
		"key":          outKey,
		"hallazgo_cid": cid,
		"findings":     findCount,
		"note":         "observación sellada; el gen no invade — solo registra",
	}, nil
}

// ResonateGensOnPulse: every gen on this node can quietly absorb a network event as hallazgo.
// Non-blocking; skips high-frequency pulse types that would flood.
func (n *NodoAlset) ResonateGensOnPulse(eventType string, data interface{}) {
	if n == nil {
		return
	}
	et := strings.ToUpper(strings.TrimSpace(eventType))
	// Skip own gen lifecycle pulses to avoid hallazgo feedback loops
	switch et {
	case agents.PulseHallazgo, agents.PulseEstado, agents.PulseConsulta,
		agents.PulseGenCreated, agents.PulseGenMutated, agents.PulseGenTravel:
		return
	case "MIND_EPISODE", "AGENT_CREATED", "ROOT_UPDATED", "DNS_REGISTERED", "ERROR", "AUDIT":
		// external ecosystem events
	default:
		low := strings.ToLower(eventType)
		if !strings.Contains(low, "error") && !strings.Contains(low, "fail") &&
			!strings.Contains(low, "episode") && !strings.Contains(low, "agent") {
			return
		}
	}
	detail := fmt.Sprintf("%s | %v", eventType, data)
	if len(detail) > 240 {
		detail = detail[:240] + "…"
	}
	n.ensureGens()
	n.mu.RLock()
	keys := make([]string, 0, len(n.gens))
	for k := range n.gens {
		keys = append(keys, k)
	}
	n.mu.RUnlock()
	for _, k := range keys {
		_, _ = n.ObserveIntoGen(k, "pulse:"+eventType, detail)
	}
}

func (n *NodoAlset) handleGenObserve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key    string `json:"key"`
		Source string `json:"source"`
		Detail string `json:"detail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if req.Key == "" {
		http.Error(w, "key required", 400)
		return
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	res, err := n.ObserveIntoGen(req.Key, req.Source, req.Detail)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}
