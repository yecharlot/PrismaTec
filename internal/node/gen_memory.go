package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"redalset/internal/agents"
)

const (
	genMissionMemory = "memory"
	metaMissionKey   = "mission"
	metaMemNotes     = "mem_notes" // short labels
	maxGenEpisodeCIDs = 64
	maxMemNotes       = 48
)

// isMemoryMissionGen reports whether this gen is dedicated to holding memory CIDs.
func isMemoryMissionGen(g *agents.AlsetGen) bool {
	if g == nil {
		return false
	}
	if strings.EqualFold(g.Manifest.Type, genMissionMemory) || strings.EqualFold(g.Manifest.Type, "memoria") {
		return true
	}
	if g.State.Metadata != nil {
		if m, ok := g.State.Metadata[metaMissionKey].(string); ok {
			m = strings.ToLower(m)
			if strings.Contains(m, "memor") || strings.Contains(m, "salvar") || strings.Contains(m, "backup") {
				return true
			}
		}
	}
	return false
}

// CreateMemoryGen births a gen whose mission is to store CIDs / notes for the network.
func (n *NodoAlset) CreateMemoryGen(key, description string) (*agents.AlsetGen, error) {
	if key == "" {
		key = "mem-nodo"
	}
	if description == "" {
		description = "gen-memoria: salva de episodios y notas de la red Alset"
	}
	g, err := n.CreateAlsetGen(key, "", genMissionMemory, description)
	if err != nil {
		// if exists, ensure mission flags
		if strings.Contains(err.Error(), "already exists") {
			return n.ensureMemoryMission(key)
		}
		return nil, err
	}
	n.mu.Lock()
	if gg, ok := n.gens[normalizeGenKey(key)]; ok && gg != nil {
		if gg.State.Metadata == nil {
			gg.State.Metadata = map[string]interface{}{}
		}
		gg.State.Metadata[metaMissionKey] = "memory"
		gg.State.Metadata["role"] = "distributed_memory"
		gg.Manifest.Type = genMissionMemory
		gg.UpdatedAt = time.Now().Unix()
	}
	n.mu.Unlock()
	n.saveGensToDisk()
	return g, nil
}

func (n *NodoAlset) ensureMemoryMission(key string) (*agents.AlsetGen, error) {
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
	g.State.Metadata[metaMissionKey] = "memory"
	g.State.Metadata["role"] = "distributed_memory"
	g.Manifest.Type = genMissionMemory
	g.UpdatedAt = time.Now().Unix()
	n.mu.Unlock()
	n.saveGensToDisk()
	return g, nil
}

// PinCIDToMemoryGen appends a content CID to the gen's EpisodeCIDs ring (salva).
func (n *NodoAlset) PinCIDToMemoryGen(key, cid, note string) (*agents.AlsetGen, error) {
	key = normalizeGenKey(key)
	cid = strings.TrimSpace(cid)
	if cid == "" {
		return nil, fmt.Errorf("cid requerido")
	}
	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.Unlock()
		return nil, fmt.Errorf("gen not found: %s — crea antes un gen memoria", key)
	}
	if !isMemoryMissionGen(g) {
		// allow pin only if operator marks mission, else soft-promote
		if g.State.Metadata == nil {
			g.State.Metadata = map[string]interface{}{}
		}
		g.State.Metadata[metaMissionKey] = "memory"
		g.Manifest.Type = genMissionMemory
	}
	// dedup
	for _, c := range g.EpisodeCIDs {
		if c == cid {
			n.mu.Unlock()
			return g, nil
		}
	}
	g.EpisodeCIDs = append(g.EpisodeCIDs, cid)
	if len(g.EpisodeCIDs) > maxGenEpisodeCIDs {
		g.EpisodeCIDs = g.EpisodeCIDs[len(g.EpisodeCIDs)-maxGenEpisodeCIDs:]
	}
	if note != "" {
		notes, _ := g.State.Metadata[metaMemNotes].([]interface{})
		notes = append(notes, map[string]string{"cid": cid, "note": note, "ts": time.Now().UTC().Format(time.RFC3339)})
		if len(notes) > maxMemNotes {
			notes = notes[len(notes)-maxMemNotes:]
		}
		g.State.Metadata[metaMemNotes] = notes
	}
	g.UpdatedAt = time.Now().Unix()
	g.State.LastSeen = g.UpdatedAt
	n.mu.Unlock()
	n.saveGensToDisk()
	n.Auditoria("GEN_MEM_PIN", fmt.Sprintf("key=%s cid=%s", key, truncateCID(cid)))
	return g, nil
}

// SaveTextToMemoryGen stores text as a block CID and pins it on the memory gen.
func (n *NodoAlset) SaveTextToMemoryGen(key, text, note string) (cid string, g *agents.AlsetGen, err error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil, fmt.Errorf("texto vacío")
	}
	payload := map[string]interface{}{
		"type": "gen_memory_note",
		"text": text,
		"note": note,
		"ts":   time.Now().UTC().Format(time.RFC3339),
		"gen":  normalizeGenKey(key),
	}
	raw, _ := json.Marshal(payload)
	cid, err = n.GenerarCID(raw)
	if err != nil || cid == "" {
		return "", nil, fmt.Errorf("no se pudo generar CID")
	}
	// also index on Mind episode ring for node-local recall
	n.appendMindEpisodeCID(cid)
	g, err = n.PinCIDToMemoryGen(key, cid, note)
	return cid, g, err
}

// ListMemoryGens returns gens with memory mission.
func (n *NodoAlset) ListMemoryGens() []*agents.AlsetGen {
	n.ensureGens()
	n.mu.RLock()
	defer n.mu.RUnlock()
	var out []*agents.AlsetGen
	for _, g := range n.gens {
		if isMemoryMissionGen(g) {
			out = append(out, g)
		}
	}
	return out
}

// RecallMemoryGenCIDs lists pinned CIDs for a gen.
func (n *NodoAlset) RecallMemoryGenCIDs(key string) []string {
	key = normalizeGenKey(key)
	n.mu.RLock()
	defer n.mu.RUnlock()
	g, ok := n.gens[key]
	if !ok || g == nil {
		return nil
	}
	out := make([]string, len(g.EpisodeCIDs))
	copy(out, g.EpisodeCIDs)
	return out
}

func (n *NodoAlset) handleGenMemoryCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key         string `json:"key"`
		Description string `json:"description"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	g, err := n.CreateMemoryGen(req.Key, req.Description)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "key": g.Key, "type": g.Manifest.Type, "mission": "memory",
	})
}

func (n *NodoAlset) handleGenMemorySave(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key  string `json:"key"`
		Text string `json:"text"`
		CID  string `json:"cid"`
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if req.Key == "" {
		req.Key = "mem-nodo"
	}
	// ensure gen exists
	if _, err := n.CreateMemoryGen(req.Key, ""); err != nil && !strings.Contains(err.Error(), "already") {
		// CreateMemoryGen handles already exists
	}
	var cid string
	var g *agents.AlsetGen
	var err error
	if req.CID != "" {
		g, err = n.PinCIDToMemoryGen(req.Key, req.CID, req.Note)
		cid = req.CID
	} else {
		cid, g, err = n.SaveTextToMemoryGen(req.Key, req.Text, req.Note)
	}
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "key": g.Key, "cid": cid, "episode_cids": len(g.EpisodeCIDs),
	})
}

func (n *NodoAlset) handleGenMemoryList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	list := n.ListMemoryGens()
	type row struct {
		Key   string   `json:"key"`
		CIDs  []string `json:"episode_cids"`
		Count int      `json:"count"`
	}
	var rows []row
	for _, g := range list {
		rows = append(rows, row{Key: g.Key, CIDs: g.EpisodeCIDs, Count: len(g.EpisodeCIDs)})
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "gens": rows})
}
