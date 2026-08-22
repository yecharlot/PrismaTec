package node

import (
	"redalset/internal/agents"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
)

// DeployGenService puts a gen on a frontier service path on THIS node.
// The gen does not inject into third-party hosts; it serves from /g/ and /work/
// on the Alset node — the "frontera" is a mission surface under our control.
func (n *NodoAlset) DeployGenService(key, mountPath, pageHTML, title string) (map[string]interface{}, error) {
	n.ensureGens()
	key = normalizeGenKey(key)
	mountPath = normalizeServicePath(mountPath, key)
	if title == "" {
		title = "Alset-Gen · " + key
	}

	n.mu.RLock()
	g, ok := n.gens[key]
	n.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("gen not found: %s", key)
	}

	if strings.TrimSpace(pageHTML) == "" {
		pageHTML = defaultGenServiceHTML(g, title)
	}
	// size guard
	if len(pageHTML) > 512*1024 {
		return nil, fmt.Errorf("página demasiado grande (máx 512KB)")
	}

	cid, err := n.GenerarCID([]byte(pageHTML))
	if err != nil || cid == "" {
		return nil, fmt.Errorf("no se pudo sellar la página de servicio")
	}

	n.mu.Lock()
	g, ok = n.gens[key]
	if !ok {
		n.mu.Unlock()
		return nil, fmt.Errorf("gen not found: %s", key)
	}
	if g.State.Metadata == nil {
		g.State.Metadata = map[string]interface{}{}
	}
	g.State.Metadata["mission"] = "service"
	g.State.Metadata["mission_status"] = "serving"
	g.State.Metadata["service_path"] = mountPath
	g.State.Metadata["service_cid"] = cid
	g.State.Metadata["service_title"] = title
	g.State.Metadata["service_since"] = time.Now().Unix()
	g.State.Location = "service:" + mountPath
	g.State.LastSeen = time.Now().Unix()
	g.UpdatedAt = g.State.LastSeen
	if g.Organs.Act < 1 {
		g.Organs.Act = 1
	}
	if g.Organs.Self < 1 {
		g.Organs.Self = 1
	}
	// ANS alias for discovery
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	n.nombres["service."+key] = g.ID
	n.nombres["service."+strings.TrimSuffix(key, ".ans")] = g.ID
	outKey := g.Key
	n.mu.Unlock()
	n.saveGensToDisk()

	go n.BroadcastPulse("GEN_SERVICE", map[string]interface{}{
		"key": outKey, "path": mountPath, "cid": cid,
	})

	out := map[string]interface{}{
		"ok":           true,
		"key":          outKey,
		"mission":      "service",
		"service_path": mountPath,
		"service_cid":  cid,
		"urls": []string{
			mountPath,
			"/g/" + strings.TrimSuffix(key, ".ans") + "/",
			"/work/" + strings.TrimSuffix(key, ".ans") + "/",
		},
		"note": "el gen sirve en la frontera del nodo Alset; no inyecta páginas en dominios ajenos",
	}
	// Autonomous package: content-addressed survival unit
	if sealed, err := n.SealFrontierPackage(outKey); err == nil {
		out["package_cid"] = sealed["package_cid"]
		out["autonomy"] = "package_cid permite revive en cualquier nodo Alset"
	}
	return out, nil
}

func normalizeServicePath(path, key string) string {
	path = strings.TrimSpace(path)
	short := strings.TrimSuffix(normalizeGenKey(key), ".ans")
	if path == "" || path == "/work" || path == "/work/" {
		return "/work/" + short + "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	// only allow /work/ or /g/ prefixes for public surface
	if !strings.HasPrefix(path, "/work/") && !strings.HasPrefix(path, "/g/") {
		return "/work/" + short + "/"
	}
	return path
}

func defaultGenServiceHTML(g *agents.AlsetGen, title string) string {
	key := ""
	root := ""
	loc := ""
	mission := ""
	findN := 0
	desc := ""
	if g != nil {
		key = g.Key
		root = g.CurrentRootCID
		loc = g.State.Location
		desc = g.Manifest.Description
		if g.State.Metadata != nil {
			if m, ok := g.State.Metadata["mission"].(string); ok {
				mission = m
			}
			switch f := g.State.Metadata["findings"].(type) {
			case []interface{}:
				findN = len(f)
			case []string:
				findN = len(f)
			}
			if rep, ok := g.State.Metadata["last_frontier_report"].(map[string]interface{}); ok {
				if sn, ok := rep["snippet"].(string); ok && sn != "" {
					desc = desc + " · último informe: " + truncateRunesGen(sn, 180)
				}
			}
		}
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>%s</title>
<style>
  :root { --bg:#0b1220; --card:#141e33; --accent:#5eead4; --text:#e8eefc; --muted:#94a3b8; }
  *{box-sizing:border-box} body{margin:0;font-family:system-ui,sans-serif;background:var(--bg);color:var(--text);
    min-height:100vh;display:flex;align-items:center;justify-content:center;padding:1.5rem}
  .card{background:var(--card);border:1px solid #243044;border-radius:16px;padding:2rem;max-width:520px;width:100%%;
    box-shadow:0 20px 50px rgba(0,0,0,.35)}
  h1{margin:0 0 .5rem;font-size:1.35rem;color:var(--accent)}
  .badge{display:inline-block;font-size:.75rem;padding:.2rem .55rem;border-radius:999px;background:#0f766e33;color:var(--accent);margin-bottom:1rem}
  p{color:var(--muted);line-height:1.5;font-size:.95rem}
  code{color:#a5f3fc;font-size:.8rem;word-break:break-all}
  .meta{margin-top:1.25rem;padding-top:1rem;border-top:1px solid #243044;font-size:.8rem;color:var(--muted)}
</style>
</head>
<body>
  <div class="card">
    <div class="badge">Alset-Gen · misión service</div>
    <h1>%s</h1>
    <p>Esta frontera es atendida por una <strong>célula madre digital</strong>. Observa, registra en CID y sirve sin invadir dominios ajenos.</p>
    <p>%s</p>
    <div class="meta">
      <div>key: <code>%s</code></div>
      <div>RootCID: <code>%s</code></div>
      <div>ubicación: <code>%s</code></div>
      <div>misión: <code>%s</code> · hallazgos: %d</div>
    </div>
  </div>
</body>
</html>`,
		html.EscapeString(title),
		html.EscapeString(title),
		html.EscapeString(desc),
		html.EscapeString(key),
		html.EscapeString(truncateCID(root)),
		html.EscapeString(loc),
		html.EscapeString(mission),
		findN,
	)
}

func truncateRunesGen(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// handleGenPublicServe serves pages for /g/{name}/ and /work/{name}/
func (n *NodoAlset) handleGenPublicServe(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	name := extractServiceNameFromPath(path)
	if name == "" {
		http.NotFound(w, r)
		return
	}
	key := normalizeGenKey(name)
	n.mu.RLock()
	g, ok := n.gens[key]
	var cid string
	if ok && g.State.Metadata != nil {
		cid, _ = g.State.Metadata["service_cid"].(string)
	}
	n.mu.RUnlock()
	if !ok || cid == "" {
		http.Error(w, "gen sin servicio activo en esta frontera", 404)
		return
	}
	data, err := n.BuscarContenidoPorCID(cid)
	if err != nil || len(data) == 0 {
		http.Error(w, "contenido de servicio no encontrado", 404)
		return
	}
	// touch last_seen without blocking response long
	go func() {
		n.mu.Lock()
		if gg, ok := n.gens[key]; ok {
			gg.State.LastSeen = time.Now().Unix()
		}
		n.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Alset-Gen", key)
	w.Header().Set("X-Alset-Service-CID", cid)
	w.WriteHeader(200)
	_, _ = w.Write(data)
}

func extractServiceNameFromPath(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	// g/name or work/name or work/name/index.html
	if len(parts) < 2 {
		return ""
	}
	if parts[0] != "g" && parts[0] != "work" {
		return ""
	}
	return parts[1]
}

func (n *NodoAlset) handleGenServiceAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key   string `json:"key"`
		Path  string `json:"path"`
		HTML  string `json:"html"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	res, err := n.DeployGenService(req.Key, req.Path, req.HTML, req.Title)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}
