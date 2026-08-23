package node

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"redalset/internal/agents"
	"redalset/internal/persistence"
)

// FrontierPackage is a self-contained, content-addressed survival unit for one gen.
// Anyone with the package CID can revive the cell on any Alset node — no single host required.
type FrontierPackage struct {
	Type           string             `json:"type"` // alset_gen_frontier_package
	Version        string             `json:"version"`
	Key            string             `json:"key"`
	ID             string             `json:"id"`
	CurrentRootCID string             `json:"current_root_cid"`
	History        []string           `json:"history,omitempty"`
	Manifest       agents.GenManifest `json:"manifest"`
	Organs         agents.GenOrganState `json:"organs"`
	State          agents.GenState    `json:"state"`
	EpisodeCIDs    []string           `json:"episode_cids,omitempty"`
	ServiceCID     string             `json:"service_cid,omitempty"`
	ServicePath    string             `json:"service_path,omitempty"`
	ServiceHTML    string             `json:"service_html,omitempty"` // embedded so revive works even if block missing
	SealedAt       string             `json:"sealed_at"`
	OriginNote     string             `json:"origin_note"`
}

// SealFrontierPackage builds and CID-seals an autonomous package for the gen.
func (n *NodoAlset) SealFrontierPackage(key string) (map[string]interface{}, error) {
	n.ensureGens()
	key = normalizeGenKey(key)
	n.mu.RLock()
	g, ok := n.gens[key]
	if !ok {
		n.mu.RUnlock()
		return nil, fmt.Errorf("gen not found: %s", key)
	}
	cp := *g
	n.mu.RUnlock()

	svcCID, _ := "", ""
	svcPath := ""
	if cp.State.Metadata != nil {
		svcCID, _ = cp.State.Metadata["service_cid"].(string)
		svcPath, _ = cp.State.Metadata["service_path"].(string)
	}
	html := ""
	if svcCID != "" {
		if data, err := n.BuscarContenidoPorCID(svcCID); err == nil {
			html = string(data)
		}
	}

	pkg := FrontierPackage{
		Type:           "alset_gen_frontier_package",
		Version:        "1.0",
		Key:            cp.Key,
		ID:             cp.ID,
		CurrentRootCID: cp.CurrentRootCID,
		History:        append([]string{}, cp.History...),
		Manifest:       cp.Manifest,
		Organs:         cp.Organs,
		State:          cp.State,
		EpisodeCIDs:    append([]string{}, cp.EpisodeCIDs...),
		ServiceCID:     svcCID,
		ServicePath:    svcPath,
		ServiceHTML:    html,
		SealedAt:       time.Now().UTC().Format(time.RFC3339),
		OriginNote:     "paquete autónomo: revive en cualquier nodo Alset con el CID",
	}
	raw, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	pkgCID, err := n.GenerarCID(raw)
	if err != nil || pkgCID == "" {
		return nil, fmt.Errorf("no se pudo sellar el paquete")
	}

	n.mu.Lock()
	if gg, ok := n.gens[key]; ok {
		if gg.State.Metadata == nil {
			gg.State.Metadata = map[string]interface{}{}
		}
		gg.State.Metadata["package_cid"] = pkgCID
		gg.State.Metadata["package_sealed_at"] = pkg.SealedAt
	}
	n.mu.Unlock()
	n.saveGensToDisk()

	// Durable index key → package CID
	if n.store != nil {
		idx := n.loadGenPackageIndex()
		idx[key] = pkgCID
		n.saveGenPackageIndex(idx)
	}

	// Static mirror under static/gens/{short}/ (helps process-local recovery)
	n.writeGenStaticMirror(key, html, pkgCID)

	go n.BroadcastPulse("GEN_PACKAGE", map[string]interface{}{
		"key": key, "package_cid": pkgCID,
	})

	return map[string]interface{}{
		"ok":          true,
		"key":         key,
		"package_cid": pkgCID,
		"service_path": svcPath,
		"note":        "guarda este package_cid: es la semilla autónoma; cualquier nodo puede POST /api/gen/revive",
	}, nil
}

// ReviveFromPackageCID restores a gen from a frontier package CID (autonomous survival).
func (n *NodoAlset) ReviveFromPackageCID(pkgCID string) (map[string]interface{}, error) {
	pkgCID = strings.TrimSpace(pkgCID)
	if pkgCID == "" {
		return nil, fmt.Errorf("package_cid requerido")
	}
	raw, err := n.BuscarContenidoPorCID(pkgCID)
	if err != nil || len(raw) == 0 {
		if raw2, _, err2 := n.FetchPackageBytes(pkgCID); err2 == nil && len(raw2) > 0 {
			raw = raw2
		} else {
			return nil, fmt.Errorf("paquete no encontrado (CID %s). Publica con /api/gen/publish o importa el bloque", truncateCID(pkgCID))
		}
	}
	var pkg FrontierPackage
	if json.Unmarshal(raw, &pkg) != nil || pkg.Type != "alset_gen_frontier_package" {
		return nil, fmt.Errorf("CID no es un paquete de frontera Alset-Gen")
	}
	key := normalizeGenKey(pkg.Key)

	// Re-seal service HTML if embedded
	if pkg.ServiceHTML != "" {
		if cid, err := n.GenerarCID([]byte(pkg.ServiceHTML)); err == nil {
			pkg.ServiceCID = cid
		}
	}

	g := &agents.AlsetGen{
		ID:             pkg.ID,
		Key:            key,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
		CurrentRootCID: pkg.CurrentRootCID,
		History:        pkg.History,
		State:          pkg.State,
		Organs:         pkg.Organs,
		Manifest:       pkg.Manifest,
		EpisodeCIDs:    pkg.EpisodeCIDs,
		OriginNode:     "",
	}
	if g.ID == "" {
		g.ID = genID()
	}
	if g.State.Metadata == nil {
		g.State.Metadata = map[string]interface{}{}
	}
	g.State.Metadata["package_cid"] = pkgCID
	g.State.Metadata["revived_at"] = time.Now().UTC().Format(time.RFC3339)
	g.State.Metadata["mission"] = "service"
	g.State.Metadata["mission_status"] = "revived"
	if pkg.ServiceCID != "" {
		g.State.Metadata["service_cid"] = pkg.ServiceCID
	}
	if pkg.ServicePath != "" {
		g.State.Metadata["service_path"] = pkg.ServicePath
		g.State.Location = "service:" + pkg.ServicePath
	}
	if n.host != nil {
		g.OriginNode = n.host.ID().String()
	}

	n.ensureGens()
	n.mu.Lock()
	n.gens[key] = g
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	n.nombres[key] = g.ID
	n.nombres[strings.TrimSuffix(key, ".ans")] = g.ID
	n.nombres["service."+key] = g.ID
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	n.agentes[g.ID] = &Agente{ID: g.ID, RootCID: g.CurrentRootCID, BalanceUTXO: g.State.Balance, UltimaActual: time.Now().Unix()}
	n.mu.Unlock()
	n.saveGensToDisk()
	n.writeGenStaticMirror(key, pkg.ServiceHTML, pkgCID)

	go n.BroadcastPulse("GEN_REVIVE", map[string]interface{}{"key": key, "package_cid": pkgCID})

	return map[string]interface{}{
		"ok":           true,
		"key":          key,
		"package_cid":  pkgCID,
		"service_path": pkg.ServicePath,
		"service_cid":  pkg.ServiceCID,
		"note":         "célula revivida desde paquete autónomo; identidad ANS preservada",
	}, nil
}

func (n *NodoAlset) loadGenPackageIndex() map[string]string {
	idx := map[string]string{}
	if n.store == nil {
		return idx
	}
	b, err := n.store.Load(context.Background(), "gen_packages.json")
	if err != nil || len(b) == 0 {
		return idx
	}
	_ = json.Unmarshal(b, &idx)
	return idx
}

func (n *NodoAlset) saveGenPackageIndex(idx map[string]string) {
	if n.store == nil {
		return
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return
	}
	if err := n.store.Save(context.Background(), "gen_packages.json", b); err != nil {
		log.Printf("⚠️ gen package index: %v", err)
	}
}

func (n *NodoAlset) writeGenStaticMirror(key, html, pkgCID string) {
	short := strings.TrimSuffix(normalizeGenKey(key), ".ans")
	dir := filepath.Join(StaticDir, "gens", short)
	_ = os.MkdirAll(dir, 0o755)
	if html == "" {
		html = fmt.Sprintf("<!DOCTYPE html><html><body><h1>%s</h1><p>package %s</p></body></html>",
			htmlEscapeBasic(short), htmlEscapeBasic(pkgCID))
	}
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644)
	meta, _ := json.MarshalIndent(map[string]string{
		"key": normalizeGenKey(key), "package_cid": pkgCID,
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "package.json"), meta, 0o644)
}

func htmlEscapeBasic(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func (n *NodoAlset) handleGenPackage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	res, err := n.SealFrontierPackage(req.Key)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}

func (n *NodoAlset) handleGenRevive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		PackageCID string `json:"package_cid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	res, err := n.ReviveFromPackageCID(req.PackageCID)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}

// Ensure persistence key referenced (avoid unused import if build tags change)
var _ = persistence.KeyGens


// ExportFrontierPackageJSON returns the raw package JSON (for alset-gen -package file).
func (n *NodoAlset) ExportFrontierPackageJSON(key string) ([]byte, error) {
	res, err := n.SealFrontierPackage(key)
	if err != nil {
		return nil, err
	}
	cid, _ := res["package_cid"].(string)
	raw, err := n.BuscarContenidoPorCID(cid)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (n *NodoAlset) handleGenExport(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key required", 400)
		return
	}
	raw, err := n.ExportFrontierPackageJSON(key)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+normalizeGenKey(key)+".package.json\"")
	_, _ = w.Write(raw)
}
