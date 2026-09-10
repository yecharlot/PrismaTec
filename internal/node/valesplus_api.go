package node

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const valesplusDataDir = "data/valesplus"
const valesplusMaxBody = 10 << 20

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
	Precio    string              `json:"precio,omitempty"`
	Moneda    string              `json:"moneda,omitempty"`
	MonedaFlag string             `json:"monedaFlag,omitempty"`
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


func vpFormatPhone(s string) string {
	d := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			d = append(d, r)
		}
	}
	if len(d) == 0 {
		return strings.TrimSpace(s)
	}
	// Cuba mobile 53 + 8 digits
	if len(d) == 10 && d[0] == '5' {
		// 5xxxxxxx -> +53 5 xxx xxxx if 8 after? 
	}
	if len(d) >= 10 && d[0] == '5' && d[1] == '3' {
		rest := string(d[2:])
		if len(rest) == 8 {
			return "+53 " + rest[0:1] + " " + rest[1:4] + " " + rest[4:]
		}
		return "+53 " + rest
	}
	if len(d) == 8 && d[0] == '5' {
		return "+53 " + string(d[0:1]) + " " + string(d[1:4]) + " " + string(d[4:])
	}
	if len(d) == 8 {
		return string(d[0:4]) + " " + string(d[4:])
	}
	return string(d)
}

func vpDecodeImageData(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, os.ErrInvalid
	}
	payload := s
	if i := strings.Index(s, ","); i >= 0 && strings.HasPrefix(s, "data:") {
		payload = s[i+1:]
	}
	payload = strings.ReplaceAll(payload, "\n", "")
	payload = strings.ReplaceAll(payload, "\r", "")
	payload = strings.ReplaceAll(payload, " ", "")
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		return nil, err
	}
	if len(raw) < 32 || len(raw) > 5<<20 {
		return nil, os.ErrInvalid
	}
	return raw, nil
}

func vpWritePhotoBytes(id string, raw []byte) bool {
	if len(raw) < 32 {
		_ = vpRenderOG(id, nil, "ValesPlus", "")
		return false
	}
	_ = vpEnsureDir()
	// normalize to jpeg when possible
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err == nil {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82}); err == nil {
			raw = buf.Bytes()
		}
		_ = os.WriteFile(filepath.Join(vpDir(), id+".jpg"), raw, 0o644)
		_ = vpRenderOG(id, img, "ValesPlus", format)
		return true
	}
	// keep original bytes as jpg name (may still be jpeg)
	_ = os.WriteFile(filepath.Join(vpDir(), id+".jpg"), raw, 0o644)
	_ = vpRenderOG(id, nil, "ValesPlus", "")
	return true
}

func vpWritePhotoAndOG(id string, dataURL string) bool {
	raw, err := vpDecodeImageData(dataURL)
	if err != nil {
		_ = vpRenderOG(id, nil, "ValesPlus", "")
		return false
	}
	return vpWritePhotoBytes(id, raw)
}

// vpRenderOG builds a 1200x630 JPEG for WhatsApp / Open Graph previews.
func vpRenderOG(id string, product image.Image, title, subtitle string) error {
	const W, H = 1200, 630
	dst := image.NewRGBA(image.Rect(0, 0, W, H))
	// background dark
	bg := color.RGBA{10, 11, 15, 255}
	draw.Draw(dst, dst.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	// gold bar top
	gold := color.RGBA{255, 196, 0, 255}
	draw.Draw(dst, image.Rect(0, 0, W, 12), &image.Uniform{gold}, image.Point{}, draw.Src)
	if product != nil {
		// fit product into left/center area
		pb := product.Bounds()
		pw, ph := pb.Dx(), pb.Dy()
		if pw < 1 || ph < 1 {
			return nil
		}
		// target box 1200x480
		boxW, boxH := W, 480
		scale := float64(boxW) / float64(pw)
		if float64(ph)*scale > float64(boxH) {
			scale = float64(boxH) / float64(ph)
		}
		tw := int(float64(pw) * scale)
		th := int(float64(ph) * scale)
		ox := (W - tw) / 2
		oy := 20
		// nearest-neighbor scale
		for y := 0; y < th; y++ {
			sy := pb.Min.Y + y*ph/th
			for x := 0; x < tw; x++ {
				sx := pb.Min.X + x*pw/tw
				dst.Set(ox+x, oy+y, product.At(sx, sy))
			}
		}
		// bottom strip
		draw.Draw(dst, image.Rect(0, 500, W, H), &image.Uniform{color.RGBA{22, 25, 34, 255}}, image.Point{}, draw.Src)
		draw.Draw(dst, image.Rect(0, 500, W, 504), &image.Uniform{gold}, image.Point{}, draw.Src)
	} else {
		// gold gradient-ish block
		draw.Draw(dst, image.Rect(80, 80, W-80, 480), &image.Uniform{color.RGBA{255, 160, 0, 255}}, image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(vpDir(), id+"_og.jpg"), buf.Bytes(), 0o644)
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
		Precio    string              `json:"precio"`
		Moneda    string              `json:"moneda"`
		MonedaFlag string             `json:"monedaFlag"`
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
	hasPhoto := vpWritePhotoAndOG(id, in.Photo)
	v := vpVale{
		ID: id, Code: in.Code, Text: in.Text, Cliente: in.Cliente, Producto: in.Producto,
		Cantidad: in.Cantidad, Gestor: in.Gestor, TelGestor: in.TelGestor, Negocio: in.Negocio,
		Extra: in.Extra, Precio: in.Precio, Moneda: in.Moneda, MonedaFlag: in.MonedaFlag, HasPhoto: hasPhoto, Created: time.Now().UTC().Format(time.RFC3339), DeviceID: in.DeviceID,
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
	img := base + "/api/valesplus/og/" + id
	// fallback chain
	if _, err := os.Stat(filepath.Join(vpDir(), id+"_og.jpg")); err != nil {
		if v.HasPhoto {
			img = base + "/api/valesplus/photo/" + id
		} else {
			img = base + "/static/apps/valesplus/icon-512.png"
		}
	}
	photoBlock := ""
	if v.HasPhoto {
		photoBlock = `<div class="hero"><img src="` + base + `/api/valesplus/photo/` + id + `" alt="producto" width="420" height="220"></div>`
	} else {
		photoBlock = `<div class="hero default"><span>ValesPlus</span></div>`
	}
	clienteLine := html.EscapeString(strings.TrimSpace(v.Cliente))
	pedidoLine := html.EscapeString(strings.TrimSpace(v.Producto))
	if strings.TrimSpace(v.Cantidad) != "" {
		pedidoLine += " × " + html.EscapeString(v.Cantidad)
	}
	if strings.TrimSpace(v.Precio) != "" {
		pedidoLine += "<br>" + html.EscapeString(strings.TrimSpace(v.MonedaFlag+" "+v.Moneda+" "+v.Precio))
	}
	gestorLine := html.EscapeString(strings.TrimSpace(v.Gestor))
	if strings.TrimSpace(v.TelGestor) != "" {
		gestorLine += "<br>" + html.EscapeString(vpFormatPhone(v.TelGestor))
	}
	if strings.TrimSpace(v.Negocio) != "" {
		gestorLine += "<br>" + html.EscapeString(v.Negocio)
	}
	sections := `<div class="sec"><h4>Cliente</h4><p>` + clienteLine + `</p></div>` +
		`<div class="sec"><h4>Pedido</h4><p>` + pedidoLine + `</p></div>` +
		`<div class="sec"><h4>Gestor</h4><p>` + gestorLine + `</p></div>`
	page := `<!DOCTYPE html><html lang="es"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + title + `</title>
<meta property="og:type" content="website">
<meta property="og:title" content="` + title + `">
<meta property="og:description" content="` + desc + `">
<meta property="og:image" content="` + html.EscapeString(img) + `">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="` + title + `">
<meta name="twitter:description" content="` + desc + `">
<meta name="twitter:image" content="` + html.EscapeString(img) + `">
<style>
body{margin:0;font-family:system-ui,sans-serif;background:#0a0b0f;color:#f5f3ee;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:1rem}
.card{width:100%;max-width:420px;border-radius:22px;overflow:hidden;background:#161922;border:1px solid rgba(255,196,0,.22);box-shadow:0 24px 60px rgba(0,0,0,.45);text-align:center}
.hero{height:220px;background:#111;position:relative;display:flex;align-items:center;justify-content:center}
.hero img{width:100%;height:100%;object-fit:cover;display:block}
.hero.default{background:linear-gradient(135deg,#FFC400,#FF9100);color:#111;font-weight:800;font-size:1.5rem}
.body{padding:1.35rem 1.4rem 1.55rem;text-align:center}
.code{color:#FFC400;font-weight:700;letter-spacing:.1em;font-size:.9rem;margin-bottom:1rem}
.sec{margin:0.85rem 0;padding:0.75rem 0;border-top:1px solid rgba(255,196,0,.12)}
.sec:first-of-type{border-top:none;padding-top:0}
.sec h4{color:#FFC400;font-size:.68rem;letter-spacing:.14em;text-transform:uppercase;margin:0 0 .45rem;font-weight:700}
.sec p{color:#d8d4cc;font-size:.95rem;line-height:1.55;margin:0;white-space:pre-wrap}
.foot{margin-top:1.15rem;font-size:.78rem;color:#8a8680}
</style></head><body>
<div class="card">` + photoBlock + `
<div class="body"><div class="code">` + html.EscapeString(v.Code) + `</div>
` + sections + `
<div class="foot">ValesPlus · Prism@.TEC</div></div></div>
</body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write([]byte(page))
}

func (n *NodoAlset) handleValesPlusOG(w http.ResponseWriter, r *http.Request) {
	vpCORS(w)
	id := strings.TrimPrefix(r.URL.Path, "/api/valesplus/og/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "..") {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(vpDir(), id+"_og.jpg")
	b, err := os.ReadFile(p)
	if err != nil {
		// fallback to product photo
		b, err = os.ReadFile(filepath.Join(vpDir(), id+".jpg"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(b)
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


func (n *NodoAlset) handleValesPlusAttachPhoto(w http.ResponseWriter, r *http.Request) {
	vpCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/valesplus/attach/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "..") {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "id"})
		return
	}
	if _, err := loadVPVale(id); err != nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, valesplusMaxBody)
	ct := r.Header.Get("Content-Type")
	var raw []byte
	if strings.Contains(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(valesplusMaxBody); err != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "multipart"})
			return
		}
		f, _, err := r.FormFile("photo")
		if err != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "photo field"})
			return
		}
		defer f.Close()
		buf := bytes.NewBuffer(nil)
		_, _ = buf.ReadFrom(f)
		raw = buf.Bytes()
	} else {
		var in struct {
			Photo string `json:"photo"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "json"})
			return
		}
		var err error
		raw, err = vpDecodeImageData(in.Photo)
		if err != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "decode"})
			return
		}
	}
	ok := vpWritePhotoBytes(id, raw)
	if ok {
		// mark hasPhoto on json
		if v, err := loadVPVale(id); err == nil {
			v.HasPhoto = true
			b, _ := json.MarshalIndent(v, "", "  ")
			_ = os.WriteFile(filepath.Join(vpDir(), id+".json"), b, 0o644)
		}
	}
	base := vpBaseURL(r)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": ok, "id": id, "hasPhoto": ok,
		"photoUrl": base + "/api/valesplus/photo/" + id,
		"ogUrl":    base + "/api/valesplus/og/" + id,
		"cardUrl":  base + "/api/valesplus/card/" + id,
	})
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
	case strings.HasPrefix(path, "attach/"):
		n.handleValesPlusAttachPhoto(w, r)
	case strings.HasPrefix(path, "og/"):
		n.handleValesPlusOG(w, r)
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
