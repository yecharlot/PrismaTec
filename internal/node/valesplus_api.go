package node

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const valesplusDataDir = "data/valesplus"
const valesplusMaxBody = 3 << 20

type vpVale struct {
	ID        string              `json:"id"`
	Code      string              `json:"code"`
	Text      string              `json:"text"`
	Cliente   string              `json:"cliente"`
	Producto  string              `json:"producto"`
	Cantidad  string              `json:"cantidad"`
	Gestor    string              `json:"gestor"`
	TelGestor string              `json:"telGestor"`
	Negocio   string              `json:"negocio"`
	Extra     []map[string]string `json:"extra,omitempty"`
	HasPhoto  bool                `json:"hasPhoto"`
	Created   string              `json:"created"`
	DeviceID  string              `json:"deviceId,omitempty"`
}

type vpDevice struct {
	ID        string `json:"id"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
	UA        string `json:"ua,omitempty"`
}

type vpAnnouncement struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

type vpMeta struct {
	Devices       map[string]*vpDevice `json:"devices"`
	Announcements []vpAnnouncement     `json:"announcements"`
}

var vpMu sync.Mutex

func vpDir() string { return filepath.Join(".", valesplusDataDir) }

func vpEnsureDir() error { return os.MkdirAll(vpDir(), 0o755) }

func vpRandID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return strings.ToLower(hexEncode(b))
}

func hexEncode(b []byte) string {
	const h = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = h[v>>4]
		out[i*2+1] = h[v&0x0f]
	}
	return string(out)
}

func loadVPMeta() vpMeta {
	var m vpMeta
	b, err := os.ReadFile(filepath.Join(vpDir(), "meta.json"))
	if err != nil {
		m.Devices = map[string]*vpDevice{}
		return m
	}
	_ = json.Unmarshal(b, &m)
	if m.Devices == nil {
		m.Devices = map[string]*vpDevice{}
	}
	return m
}

func saveVPMeta(m vpMeta) error {
	if err := vpEnsureDir(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(vpDir(), "meta.json"), b, 0o644)
}

func vpCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-ValesPlus-Admin")
}

func vpBaseURL(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return proto + "://" + host
}

func loadVPVale(id string) (*vpVale, error) {
	b, err := os.ReadFile(filepath.Join(vpDir(), id+".json"))
	if err != nil {
		return nil, err
	}
	var v vpVale
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (n *NodoAlset) handleValesPlusVale(w http.ResponseWriter, r *http.Request) {
	vpCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, valesplusMaxBody)
	var in struct {
		Code      string              `json:"code"`
		Text      string              `json:"text"`
		Cliente   string              `json:"cliente"`
		Producto  string              `json:"producto"`
		Cantidad  string              `json:"cantidad"`
		Gestor    string              `json:"gestor"`
		TelGestor string              `json:"telGestor"`
		Negocio   string              `json:"negocio"`
		Extra     []map[string]string `json:"extra"`
		Photo     string              `json:"photo"`
		DeviceID  string              `json:"deviceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "JSON inválido"})
		return
	}
	if strings.TrimSpace(in.Code) == "" {
		in.Code = "VP"
	}
	id := vpRandID()
	_ = vpEnsureDir()
	hasPhoto := false
	if strings.HasPrefix(in.Photo, "data:image/") {
		parts := strings.SplitN(in.Photo, ",", 2)
		if len(parts) == 2 {
			raw, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				// try raw std without padding issues
				raw, err = base64.RawStdEncoding.DecodeString(parts[1])
			}
			if err == nil && len(raw) > 32 && len(raw) < 2<<20 {
				_ = os.WriteFile(filepath.Join(vpDir(), id+".jpg"), raw, 0o644)
				hasPhoto = true
			}
		}
	}
	v := vpVale{
		ID: id, Code: in.Code, Text: in.Text, Cliente: in.Cliente, Producto: in.Producto,
		Cantidad: in.Cantidad, Gestor: in.Gestor, TelGestor: in.TelGestor, Negocio: in.Negocio,
		Extra: in.Extra, HasPhoto: hasPhoto, Created: time.Now().UTC().Format(time.RFC3339), DeviceID: in.DeviceID,
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(filepath.Join(vpDir(), id+".json"), b, 0o644)
	base := vpBaseURL(r)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "id": id, "cardUrl": base + "/api/valesplus/card/" + id, "hasPhoto": hasPhoto,
	})
}

func (n *NodoAlset) handleValesPlusCard(w http.ResponseWriter, r *http.Request) {
	vpCORS(w)
	id := strings.TrimPrefix(r.URL.Path, "/api/valesplus/card/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "..") {
		http.NotFound(w, r)
		return
	}
	v, err := loadVPVale(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	base := vpBaseURL(r)
	title := html.EscapeString(v.Code + " · " + v.Producto)
	desc := html.EscapeString(strings.TrimSpace(v.Cliente + " · " + v.Producto + " × " + v.Cantidad + " · Gestor: " + v.Gestor))
	img := base + "/static/apps/valesplus/icon-512.png"
	if v.HasPhoto {
		img = base + "/api/valesplus/photo/" + id
	}
	textHTML := html.EscapeString(v.Text)
	textHTML = strings.ReplaceAll(textHTML, "\n", "<br>")
	photoBlock := ""
	if v.HasPhoto {
		photoBlock = `<div class="hero" style="background-image:url('/api/valesplus/photo/` + id + `')"></div>`
	} else {
		photoBlock = `<div class="hero default"><span>ValesPlus</span></div>`
	}
	page := `<!DOCTYPE html><html lang="es"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + title + `</title>
<meta property="og:type" content="website">
<meta property="og:title" content="` + title + `">
<meta property="og:description" content="` + desc + `">
<meta property="og:image" content="` + html.EscapeString(img) + `">
<meta property="og:image:width" content="512">
<meta property="og:image:height" content="512">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="` + title + `">
<meta name="twitter:description" content="` + desc + `">
<meta name="twitter:image" content="` + html.EscapeString(img) + `">
<style>
body{margin:0;font-family:system-ui,sans-serif;background:#0a0b0f;color:#f5f3ee;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:1rem}
.card{width:100%;max-width:400px;border-radius:20px;overflow:hidden;background:#161922;border:1px solid rgba(255,196,0,.2);box-shadow:0 24px 60px rgba(0,0,0,.45)}
.hero{height:200px;background-size:cover;background-position:center;position:relative}
.hero::after{content:"";position:absolute;inset:0;backdrop-filter:blur(0px);background:linear-gradient(to top,rgba(10,11,15,.85),transparent 55%)}
.hero.default{display:flex;align-items:center;justify-content:center;background:linear-gradient(135deg,#FFC400,#FF9100);color:#111;font-weight:800;font-size:1.4rem}
.hero.default::after{display:none}
.body{padding:1.15rem 1.25rem 1.4rem}
.code{color:#FFC400;font-weight:700;letter-spacing:.06em;font-size:.85rem}
.meta{color:#9a968c;font-size:.9rem;margin-top:.75rem;line-height:1.55}
.foot{margin-top:1rem;font-size:.75rem;color:#7a7975}
</style></head><body>
<div class="card">` + photoBlock + `
<div class="body"><div class="code">` + html.EscapeString(v.Code) + `</div>
<div class="meta">` + textHTML + `</div>
<div class="foot">ValesPlus · Prism@.TEC</div></div></div>
</body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write([]byte(page))
}

func (n *NodoAlset) handleValesPlusPhoto(w http.ResponseWriter, r *http.Request) {
	vpCORS(w)
	id := strings.TrimPrefix(r.URL.Path, "/api/valesplus/photo/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "..") {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(vpDir(), id+".jpg")
	b, err := os.ReadFile(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(b)
}

func (n *NodoAlset) handleValesPlusDevice(w http.ResponseWriter, r *http.Request) {
	vpCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var in struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if strings.TrimSpace(in.ID) == "" {
		in.ID = vpRandID()
	}
	vpMu.Lock()
	defer vpMu.Unlock()
	_ = vpEnsureDir()
	m := loadVPMeta()
	now := time.Now().UTC().Format(time.RFC3339)
	if d, ok := m.Devices[in.ID]; ok {
		d.LastSeen = now
		d.UA = r.UserAgent()
	} else {
		m.Devices[in.ID] = &vpDevice{ID: in.ID, FirstSeen: now, LastSeen: now, UA: r.UserAgent()}
	}
	_ = saveVPMeta(m)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": in.ID, "devices": len(m.Devices)})
}

func (n *NodoAlset) handleValesPlusStats(w http.ResponseWriter, r *http.Request) {
	vpCORS(w)
	vpMu.Lock()
	defer vpMu.Unlock()
	m := loadVPMeta()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "devices": len(m.Devices)})
}

func (n *NodoAlset) handleValesPlusAnnouncements(w http.ResponseWriter, r *http.Request) {
	vpCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	vpMu.Lock()
	defer vpMu.Unlock()
	m := loadVPMeta()
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "items": m.Announcements})
		return
	}
	if r.Method == http.MethodPost {
		key := os.Getenv("VALESPLUS_ADMIN_KEY")
		if key == "" || r.Header.Get("X-ValesPlus-Admin") != key {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "unauthorized"})
			return
		}
		var in struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Title) == "" {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "title required"})
			return
		}
		a := vpAnnouncement{ID: vpRandID(), Title: in.Title, Body: in.Body, Created: time.Now().UTC().Format(time.RFC3339)}
		m.Announcements = append([]vpAnnouncement{a}, m.Announcements...)
		if len(m.Announcements) > 50 {
			m.Announcements = m.Announcements[:50]
		}
		_ = saveVPMeta(m)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "item": a})
		return
	}
	http.Error(w, "method", 405)
}

func (n *NodoAlset) handleValesPlusAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/valesplus")
	path = strings.TrimPrefix(path, "/")
	switch {
	case path == "vale" || path == "vale/":
		n.handleValesPlusVale(w, r)
	case path == "device" || path == "device/":
		n.handleValesPlusDevice(w, r)
	case path == "stats" || path == "stats/":
		n.handleValesPlusStats(w, r)
	case path == "announcements" || path == "announcements/":
		n.handleValesPlusAnnouncements(w, r)
	case strings.HasPrefix(path, "card/"):
		n.handleValesPlusCard(w, r)
	case strings.HasPrefix(path, "photo/"):
		n.handleValesPlusPhoto(w, r)
	default:
		vpCORS(w)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"service": "valesplus",
			"endpoints": []string{"vale", "device", "stats", "announcements", "card/{id}", "photo/{id}"},
		})
	}
}

func (n *NodoAlset) registerValesPlusAPI(extra map[string]http.HandlerFunc) {
	extra["/api/valesplus/"] = n.handleValesPlusAPI
	extra["/api/valesplus"] = n.handleValesPlusAPI
}
