package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DeleteAlsetGen removes a gen from the node registry (local). Does not wipe remote DO data.
func (n *NodoAlset) DeleteAlsetGen(key string) error {
	n.ensureGens()
	key = normalizeGenKey(key)
	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.Unlock()
		return fmt.Errorf("gen not found: %s", key)
	}
	id := g.ID
	delete(n.gens, key)
	if n.nombres != nil {
		delete(n.nombres, key)
		delete(n.nombres, strings.TrimSuffix(key, ".ans"))
		for alias, aid := range n.nombres {
			if aid == id {
				delete(n.nombres, alias)
			}
		}
	}
	// keep agent entry optional — remove lightweight mirror
	if n.agentes != nil && id != "" {
		delete(n.agentes, id)
	}
	n.mu.Unlock()
	n.saveGensToDisk()
	n.Auditoria("GEN_DELETE", key)
	go n.BroadcastPulse("GEN_DELETED", map[string]interface{}{"key": key})
	return nil
}

// ReturnGenHome brings a gen back from frontier explore or Cloudflare dispatch to local node.
func (n *NodoAlset) ReturnGenHome(key string) (*agentsGenSnap, error) {
	n.ensureGens()
	key = normalizeGenKey(key)
	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.Unlock()
		return nil, fmt.Errorf("gen not found: %s", key)
	}
	if g.State.Metadata == nil {
		g.State.Metadata = map[string]interface{}{}
	}
	prev := g.State.Location
	// Preserve last remote for audit
	if rh, ok := g.State.Metadata["remote_http"].(string); ok && rh != "" {
		g.State.Metadata["last_remote_http"] = rh
	}
	delete(g.State.Metadata, "remote_http")
	g.State.Metadata["dispatch"] = "returned"
	g.State.Metadata["travel_status"] = "home"
	g.State.Metadata["returned_from"] = prev
	g.State.Location = "local"
	if n.host != nil {
		g.State.Location = n.host.ID().String()
	}
	g.UpdatedAt = time.Now().Unix()
	g.State.LastSeen = g.UpdatedAt
	snap := &agentsGenSnap{Key: g.Key, Location: g.State.Location, Prev: prev}
	n.mu.Unlock()
	n.saveGensToDisk()
	n.Auditoria("GEN_RETURN", fmt.Sprintf("%s from %s", key, prev))
	return snap, nil
}

// small snap to avoid importing cycles in messages
type agentsGenSnap struct {
	Key      string
	Location string
	Prev     string
}

func (n *NodoAlset) handleGenDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "POST or DELETE", 405)
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Key == "" {
		req.Key = r.URL.Query().Get("key")
	}
	if err := n.DeleteAlsetGen(req.Key); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "deleted": normalizeGenKey(req.Key)})
}

func (n *NodoAlset) handleGenReturn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	snap, err := n.ReturnGenHome(req.Key)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "key": snap.Key, "location": snap.Location, "returned_from": snap.Prev,
	})
}
