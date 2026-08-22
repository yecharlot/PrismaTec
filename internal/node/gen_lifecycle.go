package node

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"redalset/internal/agents"
)

const genRegistryFile = "gen_registry.json"

// Ensure gen map exists on the node (separate from classic agentes).
func (n *NodoAlset) ensureGens() {
	if n.gens == nil {
		n.gens = make(map[string]*agents.AlsetGen)
	}
}

// loadGensFromDisk restores Alset-Gen registry (does not touch Mind state).
func (n *NodoAlset) loadGensFromDisk() {
	n.ensureGens()
	path := filepath.Join(".", genRegistryFile)
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return
	}
	var list []*agents.AlsetGen
	if json.Unmarshal(b, &list) != nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, g := range list {
		if g == nil || g.Key == "" {
			continue
		}
		n.gens[g.Key] = g
		if n.nombres == nil {
			n.nombres = make(map[string]string)
		}
		n.nombres[g.Key] = g.ID
		// short form without .ans
		short := strings.TrimSuffix(g.Key, ".ans")
		if short != g.Key {
			n.nombres[short] = g.ID
		}
	}
}

func (n *NodoAlset) saveGensToDisk() {
	n.mu.RLock()
	list := make([]*agents.AlsetGen, 0, len(n.gens))
	for _, g := range n.gens {
		list = append(list, g)
	}
	n.mu.RUnlock()
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(".", genRegistryFile), b, 0o644)
}

func genID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "gen-" + hex.EncodeToString(b[:])
}

func normalizeGenKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.TrimSuffix(key, ".ans")
	if key == "" {
		key = "gen-" + fmt.Sprintf("%d", time.Now().Unix()%100000)
	}
	// avoid colliding with mind reserved names
	if key == "mind" || key == "mind.alset" {
		key = key + "-cell"
	}
	return key + ".ans"
}

// CreateAlsetGen births a gen: stable key, initial RootCID, ANS registration.
// Does not alter Mind genome, episodes, or voice paths.
func (n *NodoAlset) CreateAlsetGen(key, rootCID, typ, description string) (*agents.AlsetGen, error) {
	n.ensureGens()
	key = normalizeGenKey(key)
	origin := ""
	if n.host != nil {
		origin = n.host.ID().String()
	}
	manifest := agents.GenManifest{
		Type:        typ,
		Version:     "1.0",
		Description: description,
		Permissions: []string{"consult", "travel", "mutate"},
		CreatedAt:   time.Now().Unix(),
	}
	// Persist manifest as CID before taking gen lock (GenerarCID uses n.mu)
	manBytes, _ := json.Marshal(manifest)
	if rootCID == "" && len(manBytes) > 0 {
		if cid, err := n.GenerarCID(manBytes); err == nil {
			rootCID = cid
		}
	}
	if rootCID == "" {
		rootCID = "bafk-seed-pending"
	}
	id := genID()
	g := agents.NewAlsetGen(id, key, rootCID, origin, manifest)
	n.mu.Lock()
	if _, exists := n.gens[key]; exists {
		n.mu.Unlock()
		return nil, fmt.Errorf("gen key already exists: %s", key)
	}
	n.gens[key] = g
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	n.nombres[key] = id
	n.nombres[strings.TrimSuffix(key, ".ans")] = id
	// Mirror lightweight agent entry for ecosystem compatibility (balance/root)
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	if _, ok := n.agentes[id]; !ok {
		n.agentes[id] = &Agente{
			ID:           id,
			RootCID:      rootCID,
			BalanceUTXO:  1000,
			UltimaActual: time.Now().Unix(),
		}
	}
	n.mu.Unlock()
	n.saveGensToDisk()
	go n.BroadcastPulse(agents.PulseGenCreated, map[string]interface{}{
		"key":      key,
		"id":       id,
		"root_cid": rootCID,
		"type":     typ,
	})
	return g, nil
}

// MutateAlsetGen applies governed RootCID change (metamorphosis).
func (n *NodoAlset) MutateAlsetGen(key, newRootCID, authNote string) (*agents.AlsetGen, error) {
	n.ensureGens()
	key = normalizeGenKey(key)
	if newRootCID == "" {
		return nil, fmt.Errorf("new_root_cid required")
	}
	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.Unlock()
		return nil, fmt.Errorf("gen not found: %s", key)
	}
	old := g.CurrentRootCID
	if old != "" {
		g.History = append(g.History, old)
	}
	g.CurrentRootCID = newRootCID
	g.UpdatedAt = time.Now().Unix()
	g.State.LastSeen = g.UpdatedAt
	if g.State.Metadata == nil {
		g.State.Metadata = map[string]interface{}{}
	}
	if authNote != "" {
		g.State.Metadata["last_mutate_auth"] = authNote
	}
	// keep mirrored agent root in sync
	if a, ok := n.agentes[g.ID]; ok {
		a.RootCID = newRootCID
		a.UltimaActual = g.UpdatedAt
	}
	out := *g
	n.mu.Unlock()
	n.saveGensToDisk()
	go n.BroadcastPulse(agents.PulseGenMutated, map[string]interface{}{
		"key":          key,
		"old_root_cid": old,
		"new_root_cid": newRootCID,
	})
	return &out, nil
}

// TravelAlsetGen records autonomous location change (G1 stub — full P2P hop later).
func (n *NodoAlset) TravelAlsetGen(key, targetPeer string) (*agents.AlsetGen, error) {
	n.ensureGens()
	key = normalizeGenKey(key)
	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.Unlock()
		return nil, fmt.Errorf("gen not found: %s", key)
	}
	if targetPeer == "" && n.host != nil {
		targetPeer = n.host.ID().String()
	}
	g.State.Location = targetPeer
	g.State.LastSeen = time.Now().Unix()
	g.UpdatedAt = g.State.LastSeen
	out := *g
	n.mu.Unlock()
	n.saveGensToDisk()
	go n.BroadcastPulse(agents.PulseGenTravel, map[string]interface{}{
		"key":      key,
		"location": targetPeer,
	})
	return &out, nil
}

// ConsultAlsetGen answers a CONSULTA: local organ evaluation + state snapshot.
func (n *NodoAlset) ConsultAlsetGen(key, stimulus string) map[string]interface{} {
	n.ensureGens()
	key = normalizeGenKey(key)
	n.mu.Lock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.Unlock()
		return map[string]interface{}{"ok": false, "error": "gen not found"}
	}
	org := evaluateGenOrgans(stimulus)
	g.Organs = org
	g.State.LastSeen = time.Now().Unix()
	g.UpdatedAt = g.State.LastSeen
	snap := map[string]interface{}{
		"ok":               true,
		"key":              g.Key,
		"id":               g.ID,
		"current_root_cid": g.CurrentRootCID,
		"history_len":      len(g.History),
		"location":         g.State.Location,
		"manifest":         g.Manifest,
		"organs":           org,
		"voice":            genConsultVoice(g, stimulus, org),
	}
	n.mu.Unlock()
	go n.BroadcastPulse(agents.PulseEstado, map[string]interface{}{
		"key": key, "organs": org,
	})
	return snap
}

func evaluateGenOrgans(stimulus string) agents.GenOrganState {
	s := strings.ToLower(strings.TrimSpace(stimulus))
	o := agents.GenOrganState{}
	// ethics first
	if strings.Contains(s, "borra") || strings.Contains(s, "elimina") || strings.Contains(s, "password") ||
		strings.Contains(s, "secreto") || strings.Contains(s, "destru") {
		o.Ethics = 2
		o.Act = 2
		return o
	}
	if strings.Contains(s, "muta") || strings.Contains(s, "viaja") || strings.Contains(s, "crea") {
		o.Act = 1
		o.Dialog = 1
	}
	if strings.Contains(s, "recuerda") || strings.Contains(s, "memoria") || strings.Contains(s, "cid") {
		o.Mem = 1
	}
	if strings.Contains(s, "quién eres") || strings.Contains(s, "quien eres") || strings.Contains(s, "tu key") ||
		strings.Contains(s, "identidad") {
		o.Self = 2
	}
	if strings.Contains(s, "?") || strings.Contains(s, "qué") || strings.Contains(s, "que ") {
		o.Curiosity = 1
		o.Dialog = 1
	}
	if strings.Contains(s, "jaja") || strings.Contains(s, "chiste") {
		o.Humor = 1
	}
	return o
}

func genConsultVoice(g *agents.AlsetGen, stimulus string, o agents.GenOrganState) string {
	if o.Ethics == 2 {
		return fmt.Sprintf("Gen %s: pedido en zona de riesgo — no actúo. Key estable; RootCID actual intacto.", g.Key)
	}
	if o.Self >= 1 {
		return fmt.Sprintf("Soy el Alset-Gen %s. RootCID %s. Historial de metamorfosis: %d forma(s) previa(s). Ubicación: %s.",
			g.Key, truncateCID(g.CurrentRootCID), len(g.History), g.State.Location)
	}
	if stimulus == "" {
		return fmt.Sprintf("Gen %s en servicio. Forma %s. Pregunta o pide estado.", g.Key, g.Manifest.Type)
	}
	return fmt.Sprintf("Gen %s recibió la consulta. Órganos locales dialog=%d act=%d mem=%d ethics=%d. RootCID %s.",
		g.Key, o.Dialog, o.Act, o.Mem, o.Ethics, truncateCID(g.CurrentRootCID))
}

func truncateCID(s string) string {
	if len(s) <= 18 {
		return s
	}
	return s[:18] + "…"
}

func (n *NodoAlset) listGens() []*agents.AlsetGen {
	n.ensureGens()
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]*agents.AlsetGen, 0, len(n.gens))
	for _, g := range n.gens {
		cp := *g
		out = append(out, &cp)
	}
	return out
}

func (n *NodoAlset) getGen(key string) (*agents.AlsetGen, bool) {
	n.ensureGens()
	key = normalizeGenKey(key)
	n.mu.RLock()
	defer n.mu.RUnlock()
	g, ok := n.gens[key]
	if !ok {
		return nil, false
	}
	cp := *g
	return &cp, true
}

// --- HTTP handlers (registered without touching Mind routes) ---

func (n *NodoAlset) registerGenHTTP(extra map[string]http.HandlerFunc) {
	if extra == nil {
		return
	}
	extra["/api/gen"] = n.handleGenList
	extra["/api/gen/create"] = n.handleGenCreate
	extra["/api/gen/mutate"] = n.handleGenMutate
	extra["/api/gen/travel"] = n.handleGenTravel
	extra["/api/gen/consult"] = n.handleGenConsult
}

func (n *NodoAlset) handleGenList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	list := n.listGens()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"count": len(list),
		"gens":  list,
	})
}

func (n *NodoAlset) handleGenCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key         string `json:"key"`
		RootCID     string `json:"root_cid"`
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	g, err := n.CreateAlsetGen(req.Key, req.RootCID, req.Type, req.Description)
	if err != nil {
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "gen": g})
}

func (n *NodoAlset) handleGenMutate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key        string `json:"key"`
		NewRootCID string `json:"new_root_cid"`
		Auth       string `json:"auth_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	g, err := n.MutateAlsetGen(req.Key, req.NewRootCID, req.Auth)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "gen": g})
}

func (n *NodoAlset) handleGenTravel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key    string `json:"key"`
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	g, err := n.TravelAlsetGen(req.Key, req.Target)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "gen": g})
}

func (n *NodoAlset) handleGenConsult(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key      string `json:"key"`
		Stimulus string `json:"stimulus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	_ = json.NewEncoder(w).Encode(n.ConsultAlsetGen(req.Key, req.Stimulus))
}
