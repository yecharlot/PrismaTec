package node

import (
	"redalset/internal/agents"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnnounceRemoteGen registers how Mind can reach a daemon gen (anywhere).
func (n *NodoAlset) AnnounceRemoteGen(key, httpBase, peerID, rootCID string, findings int) error {
	n.ensureGens()
	key = normalizeGenKey(key)
	httpBase = strings.TrimRight(strings.TrimSpace(httpBase), "/")
	if httpBase == "" {
		return fmt.Errorf("http_base requerido")
	}
	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		// Auto-create lightweight shell so Mind can address it
		g = &agents.AlsetGen{
			ID:             genID(),
			Key:            key,
			CreatedAt:      time.Now().Unix(),
			CurrentRootCID: rootCID,
			State: agents.GenState{
				Balance:  1000,
				Location: "remote:" + httpBase,
				Metadata: map[string]interface{}{},
			},
			Manifest: agents.GenManifest{
				Type:        "seed",
				Permissions: []string{"consult", "travel", "mutate"},
				Version:     "1.0",
				Description: "daemon remoto anunciado",
				CreatedAt:   time.Now().Unix(),
			},
		}
		n.gens[key] = g
		if n.nombres == nil {
			n.nombres = make(map[string]string)
		}
		n.nombres[key] = g.ID
		n.nombres[strings.TrimSuffix(key, ".ans")] = g.ID
	}
	if g.State.Metadata == nil {
		g.State.Metadata = map[string]interface{}{}
	}
	g.State.Metadata["remote_http"] = httpBase
	g.State.Metadata["remote_mode"] = "daemon"
	g.State.Metadata["last_announce"] = time.Now().Unix()
	g.State.Metadata["remote_findings"] = findings
	if peerID != "" {
		g.State.Metadata["remote_peer_id"] = peerID
	}
	if rootCID != "" {
		g.CurrentRootCID = rootCID
	}
	g.State.Location = "remote:" + httpBase
	g.State.LastSeen = time.Now().Unix()
	g.UpdatedAt = g.State.LastSeen
	n.mu.Unlock()
	n.saveGensToDisk()
	return nil
}

func (n *NodoAlset) remoteHTTPBase(key string) string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	g, ok := n.gens[normalizeGenKey(key)]
	if !ok || g.State.Metadata == nil {
		return ""
	}
	s, _ := g.State.Metadata["remote_http"].(string)
	return strings.TrimRight(s, "/")
}

// DialogueRemoteGen talks to a resident daemon (or returns local consult if no remote).
func (n *NodoAlset) DialogueRemoteGen(key, stimulus string) map[string]interface{} {
	key = normalizeGenKey(key)
	base := n.remoteHTTPBase(key)
	if base == "" {
		// local consult fallback
		return n.ConsultAlsetGen(key, stimulus)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	body, _ := json.Marshal(map[string]string{"text": stimulus})
	resp, err := client.Post(base+"/api/dialogue", "application/json", bytes.NewReader(body))
	if err != nil {
		return map[string]interface{}{
			"ok": false, "key": key, "error": err.Error(),
			"note": "daemon no alcanzable en " + base,
		}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var out map[string]interface{}
	if json.Unmarshal(raw, &out) != nil {
		return map[string]interface{}{"ok": false, "key": key, "error": "bad remote json", "body": string(raw)}
	}
	out["remote_http"] = base
	return out
}

// ExploreRemoteGen asks the resident daemon to explore (stays at frontier process).
func (n *NodoAlset) ExploreRemoteGen(key, url, mission string) map[string]interface{} {
	key = normalizeGenKey(key)
	base := n.remoteHTTPBase(key)
	if base == "" {
		res, err := n.ExploreFrontier(key, url, mission)
		if err != nil {
			return map[string]interface{}{"ok": false, "error": err.Error()}
		}
		return res
	}
	client := &http.Client{Timeout: 20 * time.Second}
	body, _ := json.Marshal(map[string]string{"url": url, "mission": mission})
	resp, err := client.Post(base+"/api/explore", "application/json", bytes.NewReader(body))
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error(), "remote_http": base}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]interface{}{"ok": false, "body": string(raw)}
	}
	out["remote_http"] = base
	out["key"] = key
	return out
}

func (n *NodoAlset) handleGenAnnounce(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key      string `json:"key"`
		HTTPBase string `json:"http_base"`
		PeerID   string `json:"peer_id"`
		RootCID  string `json:"root_cid"`
		Findings int    `json:"findings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if err := n.AnnounceRemoteGen(req.Key, req.HTTPBase, req.PeerID, req.RootCID, req.Findings); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "key": normalizeGenKey(req.Key), "http_base": req.HTTPBase,
		"note": "Mind puede localizar y dialogar con este gen en la frontera",
	})
}

func (n *NodoAlset) handleGenDialogue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key      string `json:"key"`
		Text     string `json:"text"`
		Stimulus string `json:"stimulus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	stim := req.Text
	if stim == "" {
		stim = req.Stimulus
	}
	_ = json.NewEncoder(w).Encode(n.DialogueRemoteGen(req.Key, stim))
}
