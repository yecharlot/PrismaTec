package node

import (
	"context"
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
	"redalset/internal/persistence"
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
	Status      string   `json:"status"` // requested | pending | ready | delivered | cancelled
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
	Rev      int64                 `json:"rev"` // catalog/orders/chat revision for live clients
}

var (
	ltMu  sync.Mutex
	ltMem *ltStore
)

func ltDir() string { return filepath.Join("alset_data", "latati") }
func ltStorePath() string { return filepath.Join(ltDir(), "store.json") }
func ltPhotoPath(id string) string { return filepath.Join(ltDir(), "photos", id+".jpg") }
func ltEnsure() { _ = os.MkdirAll(filepath.Join(ltDir(), "photos"), 0o755) }

const ltCFKeyStore = "latati/v1/store"
const ltCFKeyPhotoPrefix = "latati/v1/photo/"

func ltCFEnabled() bool {
	u := strings.TrimSpace(os.Getenv("ALSET_CF_STORE_URL"))
	if u == "" {
		u = strings.TrimSpace(os.Getenv("ALSET_CLOUDFLARE_NETWORK"))
	}
	return u != ""
}

func ltCFClient() (*persistence.CloudflareStore, error) {
	u := strings.TrimSpace(os.Getenv("ALSET_CF_STORE_URL"))
	if u == "" {
		u = strings.TrimSpace(os.Getenv("ALSET_CLOUDFLARE_NETWORK"))
	}
	sec := strings.TrimSpace(os.Getenv("ALSET_CF_STORE_SECRET"))
	if sec == "" {
		sec = strings.TrimSpace(os.Getenv("STORE_SECRET"))
	}
	return persistence.NewCloudflareStore(u, sec)
}

func ltLoadFromCF() (*ltStore, error) {
	cli, err := ltCFClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	raw, err := cli.Load(ctx, ltCFKeyStore)
	if err != nil {
		return nil, err
	}
	st := &ltStore{Products: map[string]*ltProduct{}, Tokens: map[string]int64{}}
	if err := json.Unmarshal(raw, st); err != nil {
		return nil, err
	}
	if st.Products == nil {
		st.Products = map[string]*ltProduct{}
	}
	if st.Tokens == nil {
		st.Tokens = map[string]int64{}
	}
	return st, nil
}

func ltSaveToCF(st *ltStore) error {
	if !ltCFEnabled() {
		return nil
	}
	cli, err := ltCFClient()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return cli.Save(ctx, ltCFKeyStore, raw)
}

func ltSavePhotoCF(id string, data []byte) error {
	if !ltCFEnabled() || len(data) == 0 {
		return nil
	}
	cli, err := ltCFClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return cli.Save(ctx, ltCFKeyPhotoPrefix+id, data)
}

func ltLoadPhotoCF(id string) ([]byte, error) {
	if !ltCFEnabled() {
		return nil, fmt.Errorf("cf off")
	}
	cli, err := ltCFClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return cli.Load(ctx, ltCFKeyPhotoPrefix+id)
}

func ltDeletePhotoCF(id string) {
	if !ltCFEnabled() {
		return
	}
	cli, err := ltCFClient()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = cli.Delete(ctx, ltCFKeyPhotoPrefix+id)
}


func ltRand(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func loadLT() *ltStore {
	if ltMem != nil {
		// Memoria vacía tras arranque fallido: reintentar Durable Object
		if ltCFEnabled() && len(ltMem.Products) == 0 {
			if cfSt, err := ltLoadFromCF(); err == nil && cfSt != nil && len(cfSt.Products) > 0 {
				fmt.Printf("📦 La Tati: recarga desde Cloudflare DO (rev=%d, products=%d)\n", cfSt.Rev, len(cfSt.Products))
				ltMem = cfSt
				raw, _ := json.MarshalIndent(cfSt, "", "  ")
				_ = os.WriteFile(ltStorePath(), raw, 0o644)
			}
		}
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
	loaded := false
	if ltCFEnabled() {
		if cfSt, err := ltLoadFromCF(); err == nil && cfSt != nil {
			st = cfSt
			loaded = true
			fmt.Printf("📦 La Tati: store cargado desde Cloudflare DO (rev=%d, products=%d)\n", st.Rev, len(st.Products))
			raw, _ := json.MarshalIndent(st, "", "  ")
			_ = os.WriteFile(ltStorePath(), raw, 0o644)
		} else if err != nil {
			fmt.Printf("⚠️ La Tati CF load: %v — intentando disco local\n", err)
		}
	}
	if !loaded {
		b, err := os.ReadFile(ltStorePath())
		if err == nil {
			_ = json.Unmarshal(b, st)
			loaded = true
			// Nunca subir a CF un catálogo vacío (evitar borrar publicaciones)
			if ltCFEnabled() && len(st.Products) > 0 {
				go func(copy *ltStore) { _ = ltSaveToCF(copy) }(st)
			}
		}
	}
	if st.Products == nil {
		st.Products = map[string]*ltProduct{}
	}
	if st.Tokens == nil {
		st.Tokens = map[string]int64{}
	}
	if strings.TrimSpace(st.Profile.Name) == "" || st.Profile.Name == "La Tati" {
		st.Profile.Name = "Dayanis Perez Soria"
	}
	if strings.TrimSpace(st.Profile.WhatsApp) == "" {
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
	diskErr := os.WriteFile(ltStorePath(), b, 0o644)
	if ltCFEnabled() {
		if len(st.Products) == 0 {
			if cfSt, err := ltLoadFromCF(); err == nil && cfSt != nil && len(cfSt.Products) > 0 {
				fmt.Printf("⚠️ La Tati: no se pisa CF (%d productos) con store vacío\n", len(cfSt.Products))
				return diskErr
			}
		}
		if err := ltSaveToCF(st); err != nil {
			fmt.Printf("⚠️ La Tati CF save: %v\n", err)
			if diskErr != nil {
				return err
			}
		}
	}
	return diskErr
}

// ltBump marks store dirty and returns the new revision (caller holds ltMu).
func ltBump(st *ltStore) int64 {
	st.Rev++
	if st.Rev <= 0 {
		st.Rev = 1
	}
	return st.Rev
}

// ltNotify pushes a La Tati event over the node pulse/gossip SSE bus.
func (n *NodoAlset) ltNotify(kind string, extra map[string]interface{}) {
	if n == nil {
		return
	}
	payload := map[string]interface{}{
		"app":  "latati",
		"kind": kind,
		"ts":   time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range extra {
		payload[k] = v
	}
	go n.BroadcastPulse("latati_"+kind, payload)
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

// ltTokenOK checks token against store (caller may already hold ltMu).
func ltTokenOK(st *ltStore, tok string) bool {
	if st == nil || tok == "" {
		return false
	}
	exp, ok := st.Tokens[tok]
	return ok && time.Now().Unix() <= exp
}

func ltAuth(r *http.Request) bool {
	tok := r.Header.Get("X-LaTati-Token")
	if tok == "" {
		return false
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	return ltTokenOK(loadLT(), tok)
}

func ltAuthFrom(r *http.Request, st *ltStore) bool {
	return ltTokenOK(st, r.Header.Get("X-LaTati-Token"))
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
	case parts[0] == "product" && len(parts) >= 2 && r.Method == http.MethodGet:
		n.ltProductGet(w, parts[1])
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
	case parts[0] == "events":
		n.ltEventsSSE(w, r)
	default:
		http.NotFound(w, r)
	}
}


func (n *NodoAlset) ltProductGet(w http.ResponseWriter, id string) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	p, ok := st.Products[id]
	if !ok || p == nil {
		ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
		return
	}
	ltJSON(w, 200, map[string]interface{}{"ok": true, "item": p, "profile": publicProfile(st.Profile)})
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
	ltJSON(w, 200, map[string]interface{}{
		"ok": true, "items": list, "rev": st.Rev, "profile": publicProfile(st.Profile),
	})
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
		if cf, err2 := ltLoadPhotoCF(id); err2 == nil && len(cf) > 0 {
			b = cf
			_ = os.MkdirAll(filepath.Join(ltDir(), "photos"), 0o755)
			_ = os.WriteFile(ltPhotoPath(id), b, 0o644)
		} else {
			http.NotFound(w, r)
			return
		}
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
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
	rev := ltBump(st)
	_ = saveLT(st)
	n.ltNotify("catalog", map[string]interface{}{"rev": rev, "action": "profile"})
	ltJSON(w, 200, map[string]interface{}{"ok": true, "profile": publicProfile(st.Profile), "rev": rev})
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
		// Nueva publicación: si no marcan vendido y stock quedó en 0, abrir con 1 unidad
		// para que el catálogo cliente la vea al instante (filtra stock<=0).
		if !in.Sold && in.Stock <= 0 {
			in.Stock = 1
		}
		st.Products[in.ID] = &in
		rev := ltBump(st)
		_ = saveLT(st)
		n.ltNotify("catalog", map[string]interface{}{"rev": rev, "product_id": in.ID, "action": "upsert"})
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": in, "rev": rev})
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
		if err := ltSavePhotoCF(id, b); err != nil {
			fmt.Printf("⚠️ La Tati photo CF: %v\n", err)
		}
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT()
		rev := int64(0)
		if p, ok := st.Products[id]; ok && p != nil {
			p.Photo = true
			p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			rev = ltBump(st)
			_ = saveLT(st)
		}
		n.ltNotify("catalog", map[string]interface{}{"rev": rev, "product_id": id, "action": "photo"})
		ltJSON(w, 200, map[string]interface{}{"ok": true, "photo": "/api/latati/photo/" + id, "rev": rev})
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
			ltDeletePhotoCF(id)
			rev := ltBump(st)
			_ = saveLT(st)
			n.ltNotify("catalog", map[string]interface{}{"rev": rev, "product_id": id, "action": "delete"})
			ltJSON(w, 200, map[string]interface{}{"ok": true, "rev": rev})
			return
		default:
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "action"})
			return
		}
		p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		rev := ltBump(st)
		_ = saveLT(st)
		n.ltNotify("catalog", map[string]interface{}{"rev": rev, "product_id": id, "action": action})
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": p, "rev": rev})
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
	if strings.TrimSpace(in.ClientPhone) == "" {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "phone"})
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
		it.Qty = 1 // un producto = una unidad por pedido
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
		for _, prev := range lines {
			if prev.ProductID == it.ProductID {
				ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "producto ya en el pedido: " + p.Title})
				return
			}
		}
		// No se descuenta stock aún: el gestor confirma con el negocio primero.
		lines = append(lines, ltLine{
			ProductID: p.ID, Title: p.Title, Qty: it.Qty,
			Price: p.Price, Currency: p.Currency, Photo: p.Photo,
		})
		total += p.Price * float64(it.Qty)
		currency = p.Currency
	}

	st.SeqOrder++
	ord := ltOrder{
		ID: ltRand(8), Code: fmt.Sprintf("LT-%04d", st.SeqOrder),
		Thread: in.Thread, ClientName: strings.TrimSpace(in.ClientName),
		ClientPhone: strings.TrimSpace(in.ClientPhone),
		Lines: lines, Total: total, Currency: currency,
		Status: "requested", Address: st.Profile.Address,
		GestorName: st.Profile.Name, GestorPhone: st.Profile.WhatsApp,
		Note: strings.TrimSpace(in.Note),
		Ts: time.Now().UTC().Format(time.RFC3339),
	}
	st.Orders = append([]ltOrder{ord}, st.Orders...)
	if len(st.Orders) > 1000 {
		st.Orders = st.Orders[:1000]
	}
	st.Messages = append(st.Messages, ltMsg{
		ID: ltRand(6), Thread: in.Thread, From: "client",
		Text: fmt.Sprintf("Solicitud %s · total %s %.2f · %d ítem(s) · esperando confirmación", ord.Code, ord.Currency, ord.Total, len(ord.Lines)),
		Ts:   ord.Ts,
	})
	rev := ltBump(st)
	_ = saveLT(st)
	n.ltNotify("order", map[string]interface{}{"rev": rev, "order_id": ord.ID, "code": ord.Code, "thread": ord.Thread, "status": "requested"})
	ltJSON(w, 200, map[string]interface{}{"ok": true, "order": ord, "rev": rev, "message": "Solicitud enviada. La gestora confirmará disponibilidad."})
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
	if !ltAuthFrom(r, st) {
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
			if ltAuthFrom(r, st) || (thread != "" && o.Thread == thread) {
				ltJSON(w, 200, map[string]interface{}{"ok": true, "order": o})
				return
			}
			// allow by code for vale share (read-only)
			if thread == "" {
				ltJSON(w, 200, map[string]interface{}{"ok": true, "order": o})
				return
			}
		}
	}
	ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
}

func ltApplyStock(st *ltStore, lines []ltLine, dir int) string {
	// dir +1 consume, -1 restore. Returns error reason or "".
	for _, l := range lines {
		p, ok := st.Products[l.ProductID]
		if !ok || p == nil {
			if dir > 0 {
				return "product gone: " + l.Title
			}
			continue
		}
		if dir > 0 {
			if p.Sold || p.Stock < l.Qty {
				return "stock: " + l.Title
			}
			p.Stock -= l.Qty
			if p.Stock <= 0 {
				p.Stock = 0
				p.Sold = true
			}
		} else {
			p.Stock += l.Qty
			if p.Stock > 0 {
				p.Sold = false
			}
		}
		p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return ""
}

func (n *NodoAlset) ltOrderStatus(w http.ResponseWriter, r *http.Request, id string) {
	var in struct {
		Status string `json:"status"`
		Thread string `json:"thread"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	in.Status = strings.TrimSpace(strings.ToLower(in.Status))
	if in.Status == "" {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "status"})
		return
	}
	// Alias amigables
	if in.Status == "confirm" || in.Status == "aceptar" || in.Status == "confirmed" {
		in.Status = "pending" // vale confirmado, listo para coordinar recogida
	}
	if in.Status == "cancel" || in.Status == "reject" || in.Status == "rechazar" {
		in.Status = "cancelled"
	}

	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	gestor := ltAuthFrom(r, st)
	for i := range st.Orders {
		o := &st.Orders[i]
		if o.ID != id && o.Code != id {
			continue
		}
		// Cliente solo puede cancelar su propia solicitud (requested)
		if !gestor {
			th := strings.TrimSpace(in.Thread)
			hth := strings.TrimSpace(r.Header.Get("X-LaTati-Thread"))
			okClient := (th != "" && th == o.Thread) || (hth != "" && hth == o.Thread)
			if !okClient {
				ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
				return
			}
			if in.Status != "cancelled" {
				ltJSON(w, 403, map[string]interface{}{"ok": false, "error": "solo puedes cancelar"})
				return
			}
			if o.Status != "requested" && o.Status != "pending" && o.Status != "ready" {
				ltJSON(w, 403, map[string]interface{}{"ok": false, "error": "este pedido ya no se puede cancelar"})
				return
			}
		}

		prev := o.Status
		if prev == in.Status {
			ltJSON(w, 200, map[string]interface{}{"ok": true, "order": *o})
			return
		}

		// Confirmar solicitud → descontar stock y emitir vale (pending)
		if in.Status == "pending" || in.Status == "ready" {
			if prev == "requested" {
				if err := ltApplyStock(st, o.Lines, +1); err != "" {
					ltJSON(w, 400, map[string]interface{}{"ok": false, "error": err})
					return
				}
				o.Address = st.Profile.Address
				o.GestorName = st.Profile.Name
				o.GestorPhone = st.Profile.WhatsApp
				st.Messages = append(st.Messages, ltMsg{
					ID: ltRand(6), Thread: o.Thread, From: "gestor",
					Text: fmt.Sprintf("✅ Pedido %s confirmado. Ya puedes usar tu vale de recogida.", o.Code),
					Ts:   time.Now().UTC().Format(time.RFC3339),
				})
			} else if prev == "cancelled" || prev == "delivered" {
				ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "estado no permite esta acción"})
				return
			}
			o.Status = in.Status
		} else if in.Status == "cancelled" {
			// Si ya se había confirmado (stock descontado), devolver stock
			if prev == "pending" || prev == "ready" {
				_ = ltApplyStock(st, o.Lines, -1)
			}
			o.Status = "cancelled"
			who := "client"
			if gestor {
				who = "gestor"
			}
			st.Messages = append(st.Messages, ltMsg{
				ID: ltRand(6), Thread: o.Thread, From: who,
				Text: fmt.Sprintf("Pedido %s cancelado.", o.Code),
				Ts:   time.Now().UTC().Format(time.RFC3339),
			})
		} else if in.Status == "delivered" {
			if prev != "pending" && prev != "ready" {
				ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "solo pedidos confirmados"})
				return
			}
			o.Status = "delivered"
		} else {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "status inválido"})
			return
		}

		rev := ltBump(st)
		_ = saveLT(st)
		n.ltNotify("order", map[string]interface{}{"rev": rev, "order_id": o.ID, "code": o.Code, "status": o.Status})
		if prev == "requested" && (o.Status == "pending" || o.Status == "ready") {
			n.ltNotify("catalog", map[string]interface{}{"rev": rev, "action": "stock_after_confirm"})
		}
		if o.Status == "cancelled" && (prev == "pending" || prev == "ready") {
			n.ltNotify("catalog", map[string]interface{}{"rev": rev, "action": "stock_after_cancel"})
		}
		ltJSON(w, 200, map[string]interface{}{"ok": true, "order": *o, "rev": rev})
		return
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
			Thread      string `json:"thread"`
			From        string `json:"from"`
			Text        string `json:"text"`
			ProductID   string `json:"product_id"`
			Token       string `json:"token"`
			ClientName  string `json:"client_name"`
			ClientPhone string `json:"client_phone"`
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
		// Anclar nombre del cliente al hilo (visible en lista de chats)
		if in.From == "client" {
			cname := strings.TrimSpace(r.Header.Get("X-LaTati-Client-Name"))
			// body fields not in struct - parse from Text prefix if JSON had client_name: extend struct
		}
		if in.From == "client" && strings.TrimSpace(in.ClientName) != "" {
			labeled := false
			for _, m := range st.Messages {
				if m.Thread == in.Thread && strings.HasPrefix(m.Text, "👤 ") {
					labeled = true
					break
				}
			}
			if !labeled {
				label := "👤 " + strings.TrimSpace(in.ClientName)
				if strings.TrimSpace(in.ClientPhone) != "" {
					label += " · " + strings.TrimSpace(in.ClientPhone)
				}
				st.Messages = append(st.Messages, ltMsg{
					ID: ltRand(6), Thread: in.Thread, From: "client",
					Text: label, Ts: time.Now().UTC().Format(time.RFC3339),
				})
			}
		}
		st.Messages = append(st.Messages, msg)
		if len(st.Messages) > 2000 {
			st.Messages = st.Messages[len(st.Messages)-2000:]
		}
		rev := ltBump(st)
		_ = saveLT(st)
		n.ltNotify("chat", map[string]interface{}{"rev": rev, "thread": msg.Thread, "from": msg.From})
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": msg, "rev": rev})
		return
	}
	http.Error(w, "method", 405)
}

func (n *NodoAlset) ltThreads(w http.ResponseWriter, r *http.Request) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT()
	if !ltAuthFrom(r, st) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
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
		if strings.HasPrefix(msg.Text, "👤 ") && t.Name == "" {
			t.Name = strings.TrimPrefix(msg.Text, "👤 ")
		}
	}
	for _, o := range st.Orders {
		if t, ok := m[o.Thread]; ok {
			if t.Name == "" {
				t.Name = o.ClientName
			}
			if o.ClientPhone != "" && t.Name != "" && !strings.Contains(t.Name, o.ClientPhone) {
				t.Name = o.ClientName + " · " + o.ClientPhone
			}
		}
	}
	list := make([]*th, 0, len(m))
	for _, t := range m {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Ts > list[j].Ts })
	ltJSON(w, 200, map[string]interface{}{"ok": true, "items": list})
}

func (n *NodoAlset) ltEventsSSE(w http.ResponseWriter, r *http.Request) {
	ltCORS(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", 500)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	sub := &SSESubscriber{ch: make(chan string, 128), ctx: ctx, cancel: cancel}
	n.pulseSubscribersMu.Lock()
	n.pulseSubscribers[sub] = true
	n.pulseSubscribersMu.Unlock()
	defer func() {
		n.pulseSubscribersMu.Lock()
		delete(n.pulseSubscribers, sub)
		n.pulseSubscribersMu.Unlock()
		cancel()
	}()
	ltMu.Lock()
	rev := loadLT().Rev
	ltMu.Unlock()
	hello, _ := json.Marshal(map[string]interface{}{"ok": true, "app": "latati", "rev": rev})
	fmt.Fprintf(w, "event: latati_hello\ndata: %s\n\n", hello)
	flusher.Flush()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg := <-sub.ch:
			// Only forward La Tati pulse events to this stream
			if strings.Contains(msg, "event: latati_") {
				fmt.Fprint(w, msg)
				flusher.Flush()
			}
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func (n *NodoAlset) registerLaTatiAPI(extra map[string]http.HandlerFunc) {
	extra["/api/latati/"] = n.handleLaTatiAPI
	extra["/api/latati"] = n.handleLaTatiAPI
	extra["/api/latati/events"] = n.ltEventsSSE
}
