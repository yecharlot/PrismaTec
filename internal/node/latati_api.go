package node

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ltProduct struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Description  string  `json:"description,omitempty"`
	Price        float64 `json:"price"`
	PricePending bool    `json:"price_pending"`
	Currency     string  `json:"currency"`
	Stock        int     `json:"stock"`
	Sold         bool    `json:"sold"`
	Photo        bool    `json:"photo"`
	Source       string  `json:"source,omitempty"` // negocio que te lo dio
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type ltMsg struct {
	ID        string `json:"id"`
	Thread    string `json:"thread"` // client session id
	From      string `json:"from"`   // gestor | client
	Text      string `json:"text"`
	ProductID string `json:"product_id,omitempty"`
	Ts        string `json:"ts"`
}

type ltVale struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	ProductID string  `json:"product_id"`
	Title     string  `json:"title"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	Qty       int     `json:"qty"`
	Client    string  `json:"client"`
	Phone     string  `json:"phone"`
	Address   string  `json:"address"`
	Note      string  `json:"note,omitempty"`
	Ts        string  `json:"ts"`
}

type ltProfile struct {
	Name     string `json:"name"`
	WhatsApp string `json:"whatsapp"`
	Address  string `json:"address"`
	Bio      string `json:"bio,omitempty"`
	Pin      string `json:"pin,omitempty"`
}

type ltStore struct {
	Profile  ltProfile            `json:"profile"`
	Products map[string]*ltProduct `json:"products"`
	Messages []ltMsg              `json:"messages"`
	Vales    []ltVale             `json:"vales"`
	Tokens   map[string]int64     `json:"tokens"` // token -> expiry unix
	SeqVale  int                  `json:"seq_vale"`
}

var (
	ltMu   sync.Mutex
	ltMem  *ltStore
)

func ltDir() string {
	return filepath.Join("alset_data", "latati")
}

func ltStorePath() string {
	return filepath.Join(ltDir(), "store.json")
}

func ltPhotoPath(id string) string {
	return filepath.Join(ltDir(), "photos", id+".jpg")
}

func ltEnsure() {
	_ = os.MkdirAll(filepath.Join(ltDir(), "photos"), 0o755)
}

func ltRand(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func loadLT() *ltStore {
	if ltMem != nil {
		return ltMem
	}
	ltEnsure()
	st := &ltStore{
		Profile: ltProfile{
			Name:     "La Tati",
			WhatsApp: "5351069717",
			Address:  "",
			Bio:      "Catálogo del gestor · pedidos por WhatsApp",
			Pin:      "tati2026",
		},
		Products: map[string]*ltProduct{},
		Tokens:   map[string]int64{},
	}
	b, err := os.ReadFile(ltStorePath())
	if err == nil {
		_ = json.Unmarshal(b, st)
	}
	if st.Products == nil {
		st.Products = map[string]*ltProduct{}
	}
	if st.Tokens == nil {
		st.Tokens = map[string]int64{}
	}
	ltMem = st
	return st
}

func saveLT(st *ltStore) error {
	ltEnsure()
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ltStorePath(), b, 0o644)
}

func ltCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-LaTati-Token")
}

func ltJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ltAuth(r *http.Request) bool {
	tok := r.Header.Get("X-LaTati-Token")
	if tok == "" {
		return false
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	exp, ok := st.Tokens[tok]
	if !ok || time.Now().Unix() > exp {
		return false
	}
	return true
}

func (n *NodoAlset) handleLaTatiAPI(w http.ResponseWriter, r *http.Request) {
	ltCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/latati")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 1 && parts[0] == "" {
		ltJSON(w, 200, map[string]interface{}{
			"ok": true, "name": "La Tati",
			"endpoints": []string{"catalog", "profile", "gestor/login", "gestor/products", "chat", "vale"},
		})
		return
	}
	switch {
	case parts[0] == "catalog" && r.Method == http.MethodGet:
		n.ltCatalog(w, r)
	case parts[0] == "profile" && r.Method == http.MethodGet:
		n.ltProfileGet(w, r)
	case parts[0] == "photo" && len(parts) >= 2 && r.Method == http.MethodGet:
		n.ltPhotoGet(w, r, parts[1])
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "login":
		n.ltLogin(w, r)
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "profile":
		n.ltProfileSave(w, r)
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "products":
		n.ltProducts(w, r, parts[2:])
	case parts[0] == "chat":
		n.ltChat(w, r)
	case parts[0] == "vale":
		n.ltVale(w, r)
	case parts[0] == "threads" && r.Method == http.MethodGet:
		n.ltThreads(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (n *NodoAlset) ltCatalog(w http.ResponseWriter, r *http.Request) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	list := make([]*ltProduct, 0, len(st.Products))
	for _, p := range st.Products {
		if p == nil || p.Sold {
			continue
		}
		if p.Stock <= 0 && !p.PricePending {
			// allow showing pending-price items even with 0 stock if not marked sold
		}
		if p.Stock <= 0 && !p.PricePending {
			continue
		}
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt > list[j].CreatedAt })
	ltJSON(w, 200, map[string]interface{}{"ok": true, "items": list, "profile": publicProfile(st.Profile)})
}

func publicProfile(p ltProfile) map[string]string {
	return map[string]string{
		"name": p.Name, "whatsapp": p.WhatsApp, "address": p.Address, "bio": p.Bio,
	}
}

func (n *NodoAlset) ltProfileGet(w http.ResponseWriter, r *http.Request) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	ltJSON(w, 200, map[string]interface{}{"ok": true, "profile": publicProfile(st.Profile)})
}

func (n *NodoAlset) ltPhotoGet(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSuffix(id, ".jpg")
	path := ltPhotoPath(id)
	b, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(b)
}

func (n *NodoAlset) ltLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", 405)
		return
	}
	var in struct {
		Pin string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "json"})
		return
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	if strings.TrimSpace(in.Pin) == "" || in.Pin != st.Profile.Pin {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "pin"})
		return
	}
	tok := ltRand(16)
	st.Tokens[tok] = time.Now().Add(30 * 24 * time.Hour).Unix()
	_ = saveLT(st)
	ltJSON(w, 200, map[string]interface{}{
		"ok": true, "token": tok, "profile": publicProfile(st.Profile),
	})
}

func (n *NodoAlset) ltProfileSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", 405)
		return
	}
	if !ltAuth(r) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	var in ltProfile
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "json"})
		return
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	if in.Name != "" {
		st.Profile.Name = in.Name
	}
	if in.WhatsApp != "" {
		st.Profile.WhatsApp = strings.TrimPrefix(strings.ReplaceAll(in.WhatsApp, " ", ""), "+")
	}
	if in.Address != "" {
		st.Profile.Address = in.Address
	}
	st.Profile.Bio = in.Bio
	if in.Pin != "" && len(in.Pin) >= 4 {
		st.Profile.Pin = in.Pin
	}
	_ = saveLT(st)
	ltJSON(w, 200, map[string]interface{}{"ok": true, "profile": publicProfile(st.Profile)})
}

func (n *NodoAlset) ltProducts(w http.ResponseWriter, r *http.Request, rest []string) {
	if !ltAuth(r) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	// GET list all
	if r.Method == http.MethodGet && len(rest) == 0 {
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT()
		list := make([]*ltProduct, 0, len(st.Products))
		for _, p := range st.Products {
			list = append(list, p)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt > list[j].CreatedAt })
		ltJSON(w, 200, map[string]interface{}{"ok": true, "items": list})
		return
	}
	// POST create/update
	if r.Method == http.MethodPost && (len(rest) == 0 || rest[0] == "") {
		var in ltProduct
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "json"})
			return
		}
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT()
		now := time.Now().UTC().Format(time.RFC3339)
		if in.ID == "" {
			in.ID = ltRand(8)
			in.CreatedAt = now
		} else if old, ok := st.Products[in.ID]; ok && old != nil {
			in.CreatedAt = old.CreatedAt
			in.Photo = old.Photo || in.Photo
		} else {
			in.CreatedAt = now
		}
		in.UpdatedAt = now
		if in.Currency == "" {
			in.Currency = "USD"
		}
		if in.PricePending || in.Price <= 0 {
			in.PricePending = true
		}
		st.Products[in.ID] = &in
		_ = saveLT(st)
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": in})
		return
	}
	// photo upload
	if len(rest) >= 2 && rest[1] == "photo" && r.Method == http.MethodPost {
		id := rest[0]
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "multipart"})
			return
		}
		f, _, err := r.FormFile("photo")
		if err != nil {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "file"})
			return
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		if err != nil || len(b) < 32 {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "empty"})
			return
		}
		ltEnsure()
		if err := os.WriteFile(ltPhotoPath(id), b, 0o644); err != nil {
			ltJSON(w, 500, map[string]interface{}{"ok": false, "error": "write"})
			return
		}
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT()
		if p, ok := st.Products[id]; ok && p != nil {
			p.Photo = true
			p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			_ = saveLT(st)
		}
		ltJSON(w, 200, map[string]interface{}{"ok": true, "photo": "/api/latati/photo/" + id})
		return
	}
	// actions: sold, stock
	if len(rest) >= 2 && r.Method == http.MethodPost {
		id := rest[0]
		action := rest[1]
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT()
		p, ok := st.Products[id]
		if !ok || p == nil {
			ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
			return
		}
		switch action {
		case "sold":
			p.Sold = true
			p.Stock = 0
		case "unsold":
			p.Sold = false
		case "stock":
			var in struct {
				Stock int `json:"stock"`
				Delta int `json:"delta"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in.Delta != 0 {
				p.Stock += in.Delta
			} else {
				p.Stock = in.Stock
			}
			if p.Stock < 0 {
				p.Stock = 0
			}
			if p.Stock == 0 {
				p.Sold = true
			}
		case "delete":
			delete(st.Products, id)
			_ = os.Remove(ltPhotoPath(id))
			_ = saveLT(st)
			ltJSON(w, 200, map[string]interface{}{"ok": true})
			return
		default:
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "action"})
			return
		}
		p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_ = saveLT(st)
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": p})
		return
	}
	http.Error(w, "method", 405)
}

func (n *NodoAlset) ltChat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		thread := r.URL.Query().Get("thread")
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT()
		var out []ltMsg
		for _, m := range st.Messages {
			if thread == "" || m.Thread == thread {
				out = append(out, m)
			}
		}
		// last 200
		if len(out) > 200 {
			out = out[len(out)-200:]
		}
		ltJSON(w, 200, map[string]interface{}{"ok": true, "items": out})
		return
	}
	if r.Method == http.MethodPost {
		var in struct {
			Thread    string `json:"thread"`
			From      string `json:"from"`
			Text      string `json:"text"`
			ProductID string `json:"product_id"`
			Token     string `json:"token"` // optional gestor token in body
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "json"})
			return
		}
		in.Text = strings.TrimSpace(in.Text)
		in.Thread = strings.TrimSpace(in.Thread)
		if in.Text == "" || in.Thread == "" {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "thread/text"})
			return
		}
		if in.From == "gestor" {
			// require auth header or token
			if !ltAuth(r) && in.Token == "" {
				ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
				return
			}
			if in.Token != "" {
				ltMu.Lock()
				st := loadLT()
				exp, ok := st.Tokens[in.Token]
				ltMu.Unlock()
				if !ok || time.Now().Unix() > exp {
					ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
					return
				}
			}
		} else {
			in.From = "client"
		}
		msg := ltMsg{
			ID: ltRand(6), Thread: in.Thread, From: in.From, Text: in.Text,
			ProductID: in.ProductID, Ts: time.Now().UTC().Format(time.RFC3339),
		}
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT()
		st.Messages = append(st.Messages, msg)
		if len(st.Messages) > 2000 {
			st.Messages = st.Messages[len(st.Messages)-2000:]
		}
		_ = saveLT(st)
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": msg})
		return
	}
	http.Error(w, "method", 405)
}

func (n *NodoAlset) ltThreads(w http.ResponseWriter, r *http.Request) {
	if !ltAuth(r) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	type th struct {
		Thread string `json:"thread"`
		Last   string `json:"last"`
		Ts     string `json:"ts"`
		Count  int    `json:"count"`
	}
	m := map[string]*th{}
	for _, msg := range st.Messages {
		t, ok := m[msg.Thread]
		if !ok {
			t = &th{Thread: msg.Thread}
			m[msg.Thread] = t
		}
		t.Count++
		t.Last = msg.Text
		t.Ts = msg.Ts
	}
	list := make([]*th, 0, len(m))
	for _, t := range m {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Ts > list[j].Ts })
	ltJSON(w, 200, map[string]interface{}{"ok": true, "items": list})
}

func (n *NodoAlset) ltVale(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		id := r.URL.Query().Get("id")
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT()
		if id != "" {
			for _, v := range st.Vales {
				if v.ID == id || v.Code == id {
					ltJSON(w, 200, map[string]interface{}{"ok": true, "item": v})
					return
				}
			}
			ltJSON(w, 404, map[string]interface{}{"ok": false})
			return
		}
		if !ltAuth(r) {
			ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
			return
		}
		ltJSON(w, 200, map[string]interface{}{"ok": true, "items": st.Vales})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST", 405)
		return
	}
	if !ltAuth(r) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	var in ltVale
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "json"})
		return
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	st.SeqVale++
	in.ID = ltRand(8)
	in.Code = fmt.Sprintf("LT-%04d", st.SeqVale)
	in.Ts = time.Now().UTC().Format(time.RFC3339)
	if in.Qty < 1 {
		in.Qty = 1
	}
	if in.Address == "" {
		in.Address = st.Profile.Address
	}
	if p, ok := st.Products[in.ProductID]; ok && p != nil {
		if in.Title == "" {
			in.Title = p.Title
		}
		if in.Price == 0 {
			in.Price = p.Price
		}
		if in.Currency == "" {
			in.Currency = p.Currency
		}
		// reduce stock
		p.Stock -= in.Qty
		if p.Stock <= 0 {
			p.Stock = 0
			p.Sold = true
		}
		p.UpdatedAt = in.Ts
	}
	st.Vales = append([]ltVale{in}, st.Vales...)
	if len(st.Vales) > 500 {
		st.Vales = st.Vales[:500]
	}
	_ = saveLT(st)
	// text for WhatsApp
	text := fmt.Sprintf("Vale %s · %s × %d", in.Code, in.Title, in.Qty)
	if in.Price > 0 {
		text += fmt.Sprintf(" · %s %.2f", in.Currency, in.Price)
	}
	text += fmt.Sprintf("\nCliente: %s %s\nRecogida: %s", in.Client, in.Phone, in.Address)
	if in.Note != "" {
		text += "\n" + in.Note
	}
	ltJSON(w, 200, map[string]interface{}{"ok": true, "item": in, "text": text})
}

func (n *NodoAlset) registerLaTatiAPI(extra map[string]http.HandlerFunc) {
	extra["/api/latati/"] = n.handleLaTatiAPI
	extra["/api/latati"] = n.handleLaTatiAPI
}
