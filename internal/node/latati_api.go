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
	Source       string  `json:"source,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type ltLine struct {
	ProductID string  `json:"product_id"`
	Title     string  `json:"title"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	Photo     bool    `json:"photo"`
}

type ltOrder struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Thread      string   `json:"thread"`
	ClientName  string   `json:"client_name"`
	ClientPhone string   `json:"client_phone"`
	Lines       []ltLine `json:"lines"`
	Total       float64  `json:"total"`
	Currency    string   `json:"currency"`
	Status      string   `json:"status"` // pending | ready | delivered | cancelled
	Address     string   `json:"address"`
	GestorName  string   `json:"gestor_name"`
	GestorPhone string   `json:"gestor_phone"`
	Note        string   `json:"note,omitempty"`
	Ts          string   `json:"ts"`
}

type ltMsg struct {
	ID        string `json:"id"`
	Thread    string `json:"thread"`
	From      string `json:"from"`
	Text      string `json:"text"`
	ProductID string `json:"product_id,omitempty"`
	Ts        string `json:"ts"`
}

type ltProfile struct {
	Name     string `json:"name"`
	WhatsApp string `json:"whatsapp"`
	Address  string `json:"address"`
	Bio      string `json:"bio,omitempty"`
	Pin      string `json:"pin,omitempty"`
}

type ltStore struct {
	Profile  ltProfile             `json:"profile"`
	Products map[string]*ltProduct `json:"products"`
	Orders   []ltOrder             `json:"orders"`
	Messages []ltMsg               `json:"messages"`
	Tokens   map[string]int64      `json:"tokens"`
	SeqOrder int                   `json:"seq_order"`
}

var (
	ltMu  sync.Mutex
	ltMem *ltStore
)

func ltDir() string { return filepath.Join("alset_data", "latati") }
func ltStorePath() string { return filepath.Join(ltDir(), "store.json") }
func ltPhotoPath(id string) string { return filepath.Join(ltDir(), "photos", id+".jpg") }
func ltEnsure() { _ = os.MkdirAll(filepath.Join(ltDir(), "photos"), 0o755) }

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
			Name:     "Dayanis Perez Soria",
			WhatsApp: "5351069717",
			Address:  "",
			Bio:      "La Tati · catálogo y pedidos en la app",
			Pin:      "tati2026",
		},
		Products: map[string]*ltProduct{},
		Tokens:   map[string]int64{},
		Orders:   []ltOrder{},
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
	// migrate defaults if empty name
	if strings.TrimSpace(st.Profile.Name) == "" || st.Profile.Name == "La Tati" {
		st.Profile.Name = "Dayanis Perez Soria"
	}
	if strings.TrimSpace(st.Profile.WhatsApp) == "" || st.Profile.WhatsApp == "5351069717" {
		st.Profile.WhatsApp = "5351069717"
	}
	if st.Profile.Pin == "" {
		st.Profile.Pin = "tati2026"
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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-LaTati-Token, X-LaTati-Thread")
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
	return ok && time.Now().Unix() <= exp
}

func publicProfile(p ltProfile) map[string]string {
	return map[string]string{
		"name": p.Name, "whatsapp": p.WhatsApp, "phone": p.WhatsApp,
		"address": p.Address, "bio": p.Bio,
	}
}

func (n *NodoAlset) handleLaTatiAPI(w http.ResponseWriter, r *http.Request) {
	ltCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/latati"), "/")
	parts := strings.Split(path, "/")
	if path == "" {
		ltJSON(w, 200, map[string]interface{}{
			"ok": true, "name": "La Tati",
			"endpoints": []string{"catalog", "profile", "order", "orders", "chat", "gestor/login"},
		})
		return
	}
	switch {
	case parts[0] == "catalog" && r.Method == http.MethodGet:
		n.ltCatalog(w)
	case parts[0] == "profile" && r.Method == http.MethodGet:
		n.ltProfileGet(w)
	case parts[0] == "photo" && len(parts) >= 2 && r.Method == http.MethodGet:
		n.ltPhotoGet(w, r, parts[1])
	case parts[0] == "order" && r.Method == http.MethodPost:
		n.ltPlaceOrder(w, r)
	case parts[0] == "orders" && r.Method == http.MethodGet:
		n.ltListOrders(w, r)
	case parts[0] == "order" && len(parts) >= 2 && r.Method == http.MethodGet:
		n.ltGetOrder(w, r, parts[1])
	case parts[0] == "order" && len(parts) >= 3 && parts[2] == "status" && r.Method == http.MethodPost:
		n.ltOrderStatus(w, r, parts[1])
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "login":
		n.ltLogin(w, r)
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "profile":
		n.ltProfileSave(w, r)
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "products":
		n.ltProducts(w, r, parts[2:])
	case parts[0] == "chat":
		n.ltChat(w, r)
	case parts[0] == "threads" && r.Method == http.MethodGet:
		n.ltThreads(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (n *NodoAlset) ltCatalog(w http.ResponseWriter) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	list := make([]*ltProduct, 0)
	for _, p := range st.Products {
		if p == nil || p.Sold {
			continue
		}
		if p.Stock <= 0 {
			continue
		}
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt > list[j].CreatedAt })
	ltJSON(w, 200, map[string]interface{}{"ok": true, "items": list, "profile": publicProfile(st.Profile)})
}

func (n *NodoAlset) ltProfileGet(w http.ResponseWriter) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	ltJSON(w, 200, map[string]interface{}{"ok": true, "profile": publicProfile(st.Profile)})
}

func (n *NodoAlset) ltPhotoGet(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSuffix(id, ".jpg")
	b, err := os.ReadFile(ltPhotoPath(id))
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
	ltJSON(w, 200, map[string]interface{}{"ok": true, "token": tok, "profile": publicProfile(st.Profile)})
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
	st.Profile.Address = in.Address
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
	if r.Method == http.MethodPost && len(rest) == 0 {
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
		} else {
			in.PricePending = false
		}
		st.Products[in.ID] = &in
		_ = saveLT(st)
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": in})
		return
	}
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
		_ = os.WriteFile(ltPhotoPath(id), b, 0o644)
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
	if len(rest) >= 2 && r.Method == http.MethodPost {
		id, action := rest[0], rest[1]
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
			} else {
				p.Sold = false
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

func (n *NodoAlset) ltPlaceOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Thread      string `json:"thread"`
		ClientName  string `json:"client_name"`
		ClientPhone string `json:"client_phone"`
		Note        string `json:"note"`
		Items       []struct {
			ProductID string `json:"product_id"`
			Qty       int    `json:"qty"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "json"})
		return
	}
	in.Thread = strings.TrimSpace(in.Thread)
	if in.Thread == "" || len(in.Items) == 0 {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "thread/items"})
		return
	}
	if strings.TrimSpace(in.ClientName) == "" {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "name"})
		return
	}

	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()

	var lines []ltLine
	total := 0.0
	currency := "USD"
	for _, it := range in.Items {
		if it.Qty < 1 {
			it.Qty = 1
		}
		p, ok := st.Products[it.ProductID]
		if !ok || p == nil || p.Sold {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "product unavailable: " + it.ProductID})
			return
		}
		if p.PricePending || p.Price <= 0 {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "product without price: " + p.Title})
			return
		}
		if p.Stock < it.Qty {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "stock: " + p.Title})
			return
		}
		lines = append(lines, ltLine{
			ProductID: p.ID, Title: p.Title, Qty: it.Qty,
			Price: p.Price, Currency: p.Currency, Photo: p.Photo,
		})
		total += p.Price * float64(it.Qty)
		currency = p.Currency
		p.Stock -= it.Qty
		if p.Stock <= 0 {
			p.Stock = 0
			p.Sold = true
		}
		p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	st.SeqOrder++
	ord := ltOrder{
		ID: ltRand(8), Code: fmt.Sprintf("LT-%04d", st.SeqOrder),
		Thread: in.Thread, ClientName: strings.TrimSpace(in.ClientName),
		ClientPhone: strings.TrimSpace(in.ClientPhone),
		Lines: lines, Total: total, Currency: currency,
		Status: "pending", Address: st.Profile.Address,
		GestorName: st.Profile.Name, GestorPhone: st.Profile.WhatsApp,
		Note: strings.TrimSpace(in.Note),
		Ts: time.Now().UTC().Format(time.RFC3339),
	}
	st.Orders = append([]ltOrder{ord}, st.Orders...)
	if len(st.Orders) > 1000 {
		st.Orders = st.Orders[:1000]
	}
	// system chat note
	st.Messages = append(st.Messages, ltMsg{
		ID: ltRand(6), Thread: in.Thread, From: "client",
		Text: fmt.Sprintf("Pedido %s · total %s %.2f · %d ítem(s)", ord.Code, ord.Currency, ord.Total, len(ord.Lines)),
		Ts:   ord.Ts,
	})
	_ = saveLT(st)
	ltJSON(w, 200, map[string]interface{}{"ok": true, "order": ord})
}

func (n *NodoAlset) ltListOrders(w http.ResponseWriter, r *http.Request) {
	thread := strings.TrimSpace(r.URL.Query().Get("thread"))
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	if thread != "" {
		var out []ltOrder
		for _, o := range st.Orders {
			if o.Thread == thread {
				out = append(out, o)
			}
		}
		ltJSON(w, 200, map[string]interface{}{"ok": true, "items": out})
		return
	}
	if !ltAuth(r) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	ltJSON(w, 200, map[string]interface{}{"ok": true, "items": st.Orders})
}

func (n *NodoAlset) ltGetOrder(w http.ResponseWriter, r *http.Request, id string) {
	thread := strings.TrimSpace(r.URL.Query().Get("thread"))
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	for _, o := range st.Orders {
		if o.ID == id || o.Code == id {
			// client may only see own; gestor with auth sees all
			if ltAuth(r) || (thread != "" && o.Thread == thread) {
				ltJSON(w, 200, map[string]interface{}{"ok": true, "order": o})
				return
			}
			// also allow by id alone for vale display if they have the code link
			if thread == "" && !ltAuth(r) {
				ltJSON(w, 200, map[string]interface{}{"ok": true, "order": o})
				return
			}
		}
	}
	ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
}

func (n *NodoAlset) ltOrderStatus(w http.ResponseWriter, r *http.Request, id string) {
	if !ltAuth(r) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	in.Status = strings.TrimSpace(in.Status)
	if in.Status == "" {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "status"})
		return
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	for i := range st.Orders {
		if st.Orders[i].ID == id || st.Orders[i].Code == id {
			st.Orders[i].Status = in.Status
			_ = saveLT(st)
			ltJSON(w, 200, map[string]interface{}{"ok": true, "order": st.Orders[i]})
			return
		}
	}
	ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
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
			Token     string `json:"token"`
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
			ok := ltAuth(r)
			if !ok && in.Token != "" {
				ltMu.Lock()
				st := loadLT()
				exp, exists := st.Tokens[in.Token]
				ltMu.Unlock()
				ok = exists && time.Now().Unix() <= exp
			}
			if !ok {
				ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
				return
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
		Name   string `json:"name,omitempty"`
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
	for _, o := range st.Orders {
		if t, ok := m[o.Thread]; ok && t.Name == "" {
			t.Name = o.ClientName
		}
	}
	list := make([]*th, 0, len(m))
	for _, t := range m {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Ts > list[j].Ts })
	ltJSON(w, 200, map[string]interface{}{"ok": true, "items": list})
}

func (n *NodoAlset) registerLaTatiAPI(extra map[string]http.HandlerFunc) {
	extra["/api/latati/"] = n.handleLaTatiAPI
	extra["/api/latati"] = n.handleLaTatiAPI
}
