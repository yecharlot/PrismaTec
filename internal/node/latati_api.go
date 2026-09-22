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
	Stock        int     `json:"stock"` // solo informativo; no bloquea catálogo
	Unlimited    bool    `json:"unlimited"` // sin control de cantidad
	Sold         bool    `json:"sold"` // legado
	SoldOut      bool    `json:"sold_out"`
	SoldOutAt    string  `json:"sold_out_at,omitempty"`
	Photo        bool    `json:"photo"`
	Category     string  `json:"category,omitempty"`
	DeliveryMode   string `json:"delivery_mode,omitempty"` // pickup | delivery | both
	PickupAddress  string `json:"pickup_address,omitempty"` // opcional por producto
	OwnerName      string `json:"owner_name,omitempty"`     // dueño/a del negocio (opcional por producto)
	Source         string `json:"source,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type ltInterest struct {
	ID          string `json:"id"`
	ProductID   string `json:"product_id"`
	Title       string `json:"title"`
	Thread      string `json:"thread"`
	ClientName  string `json:"client_name"`
	ClientPhone string `json:"client_phone"`
	Ts          string `json:"ts"`
	Seen        bool   `json:"seen"`
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
	Status       string   `json:"status"` // requested | pending | ready | delivered | cancelled
	Address      string   `json:"address"`
	DeliveryType  string `json:"delivery_type,omitempty"` // pickup | delivery
	DeliveryAddr  string `json:"delivery_address,omitempty"`
	DeliveryNote  string `json:"delivery_note,omitempty"` // hora aprox. / detalles entrega
	PickupAddress string `json:"pickup_address,omitempty"`
	OwnerName     string `json:"owner_name,omitempty"`
	GestorName    string `json:"gestor_name"`
	GestorPhone   string `json:"gestor_phone"`
	Note          string `json:"note,omitempty"`
	CancelReason  string `json:"cancel_reason,omitempty"`
	Ts            string `json:"ts"`
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
	// Branding (SaaS / multi-tenant)
	Accent   string `json:"accent,omitempty"`
	Accent2  string `json:"accent2,omitempty"`
	Bg       string `json:"bg,omitempty"`
	Surface  string `json:"surface,omitempty"`
	Tagline  string `json:"tagline,omitempty"`
}

type ltTenantMeta struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	Active    bool   `json:"active"`
}

type ltStore struct {
	Profile    ltProfile             `json:"profile"`
	Products   map[string]*ltProduct `json:"products"`
	Orders     []ltOrder             `json:"orders"`
	Messages   []ltMsg               `json:"messages"`
	Interests  []ltInterest          `json:"interests"`
	Tokens     map[string]int64      `json:"tokens"`
	SeqOrder   int                   `json:"seq_order"`
	Rev        int64                 `json:"rev"`
	OrdersSeen int64                 `json:"orders_seen"`
	Categories []string              `json:"categories,omitempty"`
}

type ltTenKey struct{}

var (
	ltMu     sync.Mutex
	ltMemMap = map[string]*ltStore{} // tenant -> store
)

func ltNormTenant(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	s = b.String()
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" || s == "gestion-latati" {
		return "latati"
	}
	if strings.HasPrefix(s, "gestion-") {
		s = strings.TrimPrefix(s, "gestion-")
	}
	if s == "" {
		return "latati"
	}
	return s
}

func ltTen(r *http.Request) string {
	if r == nil {
		return "latati"
	}
	if v, ok := r.Context().Value(ltTenKey{}).(string); ok && v != "" {
		return ltNormTenant(v)
	}
	return "latati"
}

func ltDir(tenant string) string {
	tenant = ltNormTenant(tenant)
	if tenant == "latati" {
		return filepath.Join("alset_data", "latati")
	}
	return filepath.Join("alset_data", "latati", "t", tenant)
}
func ltStorePath(tenant string) string { return filepath.Join(ltDir(tenant), "store.json") }
func ltPhotoPath(tenant, id string) string {
	return filepath.Join(ltDir(tenant), "photos", id+".jpg")
}
func ltEnsure(tenant string) {
	_ = os.MkdirAll(filepath.Join(ltDir(tenant), "photos"), 0o755)
}

func ltCFKeyStore(tenant string) string {
	tenant = ltNormTenant(tenant)
	if tenant == "latati" {
		return "latati/v1/store" // legacy key — no romper La Tati
	}
	return "latati/v1/t/" + tenant + "/store"
}
func ltCFKeyPhotoPrefix(tenant string) string {
	tenant = ltNormTenant(tenant)
	if tenant == "latati" {
		return "latati/v1/photo/"
	}
	return "latati/v1/t/" + tenant + "/photo/"
}

const ltRegistryKey = "latati/v1/_tenants"

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

func ltLoadFromCF(tenant string) (*ltStore, error) {
	cli, err := ltCFClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	raw, err := cli.Load(ctx, ltCFKeyStore(tenant))
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
	if st.Interests == nil {
		st.Interests = []ltInterest{}
	}
	return st, nil
}

func ltSaveToCF(tenant string, st *ltStore) error {
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
	return cli.Save(ctx, ltCFKeyStore(tenant), raw)
}

func ltSavePhotoCF(tenant, id string, data []byte) error {
	if !ltCFEnabled() || len(data) == 0 {
		return nil
	}
	cli, err := ltCFClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return cli.Save(ctx, ltCFKeyPhotoPrefix(tenant)+id, data)
}

func ltLoadPhotoCF(tenant, id string) ([]byte, error) {
	if !ltCFEnabled() {
		return nil, fmt.Errorf("cf off")
	}
	cli, err := ltCFClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return cli.Load(ctx, ltCFKeyPhotoPrefix(tenant)+id)
}

func ltDeletePhotoCF(tenant, id string) {
	if !ltCFEnabled() {
		return
	}
	cli, err := ltCFClient()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = cli.Delete(ctx, ltCFKeyPhotoPrefix(tenant)+id)
}


func ltRand(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func loadLT(tenant string) *ltStore {
	tenant = ltNormTenant(tenant)
	if st, ok := ltMemMap[tenant]; ok && st != nil {
		if ltCFEnabled() && len(st.Products) == 0 {
			if cfSt, err := ltLoadFromCF(tenant); err == nil && cfSt != nil && len(cfSt.Products) > 0 {
				fmt.Printf("📦 La Tati[%s]: recarga CF (rev=%d, products=%d)\n", tenant, cfSt.Rev, len(cfSt.Products))
				ltMemMap[tenant] = cfSt
				raw, _ := json.MarshalIndent(cfSt, "", "  ")
				_ = os.WriteFile(ltStorePath(tenant), raw, 0o644)
				return cfSt
			}
		}
		return st
	}
	ltEnsure(tenant)
	st := &ltStore{
		Profile:  ltProfile{Name: "Mi tienda", Pin: "1234", Bio: "Catálogo y pedidos"},
		Products: map[string]*ltProduct{},
		Tokens:   map[string]int64{},
		Orders:   []ltOrder{},
	}
	if tenant == "latati" {
		st.Profile = ltProfile{
			Name: "Dayanis Perez Soria", WhatsApp: "5351069717",
			Bio: "La Tati · catálogo y pedidos en la app", Pin: "tati2026",
		}
	}
	loaded := false
	if ltCFEnabled() {
		if cfSt, err := ltLoadFromCF(tenant); err == nil && cfSt != nil {
			st = cfSt
			loaded = true
			fmt.Printf("📦 La Tati[%s]: CF (rev=%d, products=%d)\n", tenant, st.Rev, len(st.Products))
			raw, _ := json.MarshalIndent(st, "", "  ")
			_ = os.WriteFile(ltStorePath(tenant), raw, 0o644)
		} else if err != nil {
			fmt.Printf("⚠️ La Tati[%s] CF load: %v\n", tenant, err)
		}
	}
	if !loaded {
		b, err := os.ReadFile(ltStorePath(tenant))
		if err == nil {
			_ = json.Unmarshal(b, st)
			loaded = true
			if ltCFEnabled() && len(st.Products) > 0 {
				go func(tn string, copy *ltStore) { _ = ltSaveToCF(tn, copy) }(tenant, st)
			}
		}
	}
	if st.Products == nil {
		st.Products = map[string]*ltProduct{}
	}
	if st.Tokens == nil {
		st.Tokens = map[string]int64{}
	}
	if st.Interests == nil {
		st.Interests = []ltInterest{}
	}
	if tenant == "latati" {
		if strings.TrimSpace(st.Profile.Name) == "" || st.Profile.Name == "La Tati" {
			st.Profile.Name = "Dayanis Perez Soria"
		}
		if strings.TrimSpace(st.Profile.WhatsApp) == "" {
			st.Profile.WhatsApp = "5351069717"
		}
		if st.Profile.Pin == "" {
			st.Profile.Pin = "tati2026"
		}
	}
	if st.Profile.Pin == "" {
		st.Profile.Pin = "1234"
	}
	ltMemMap[tenant] = st
	return st
}

func saveLT(tenant string, st *ltStore) error {
	tenant = ltNormTenant(tenant)
	ltEnsure(tenant)
	ltMemMap[tenant] = st
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	diskErr := os.WriteFile(ltStorePath(tenant), b, 0o644)
	if ltCFEnabled() {
		if len(st.Products) == 0 {
			if cfSt, err := ltLoadFromCF(tenant); err == nil && cfSt != nil && len(cfSt.Products) > 0 {
				fmt.Printf("⚠️ La Tati[%s]: no se pisa CF (%d productos) con store vacío\n", tenant, len(cfSt.Products))
				return diskErr
			}
		}
		if err := ltSaveToCF(tenant, st); err != nil {
			fmt.Printf("⚠️ La Tati[%s] CF save: %v\n", tenant, err)
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

// ltPurgeSoldOut removes products marked sold-out for more than 24h.
func ltPurgeSoldOut(tenant string, st *ltStore) (removed int) {
	if st == nil || st.Products == nil {
		return 0
	}
	now := time.Now().UTC()
	for id, p := range st.Products {
		if p == nil || !p.SoldOut || p.SoldOutAt == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339, p.SoldOutAt)
		if err != nil {
			continue
		}
		if now.Sub(ts) >= 24*time.Hour {
			delete(st.Products, id)
			_ = os.Remove(ltPhotoPath(tenant, id))
			ltDeletePhotoCF(tenant, id)
			removed++
		}
	}
	return removed
}

func ltProductAvailable(p *ltProduct) bool {
	if p == nil {
		return false
	}
	if p.SoldOut || p.Sold {
		return false
	}
	return true
}

// ltNotify pushes a La Tati event over the node pulse/gossip SSE bus.
// Always includes tenant so multi-tenant clients can filter.
// Uses BroadcastPulse (host_adapter) so gens still resonate on the same event.
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
	if _, ok := payload["tenant"]; !ok {
		payload["tenant"] = "latati"
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
	return ltTokenOK(loadLT(ltTen(r)), tok)
}

func ltAuthFrom(r *http.Request, st *ltStore) bool {
	return ltTokenOK(st, r.Header.Get("X-LaTati-Token"))
}

func publicProfile(p ltProfile) map[string]string {
	return map[string]string{
		"name": p.Name, "whatsapp": p.WhatsApp, "phone": p.WhatsApp,
		"address": p.Address, "bio": p.Bio,
		"accent": p.Accent, "accent2": p.Accent2, "bg": p.Bg, "surface": p.Surface,
		"tagline": p.Tagline,
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
	tenant := "latati"
	if len(parts) >= 2 && parts[0] == "t" {
		tenant = ltNormTenant(parts[1])
		parts = parts[2:]
		path = strings.Join(parts, "/")
	}
	r = r.WithContext(context.WithValue(r.Context(), ltTenKey{}, tenant))
	if path == "" {
		ltJSON(w, 200, map[string]interface{}{
			"ok": true, "name": "La Tati core", "tenant": tenant,
			"endpoints": []string{"catalog", "profile", "order", "orders", "chat", "gestor/login", "tenants"},
		})
		return
	}
	switch {
	case parts[0] == "tenants":
		n.ltTenants(w, r)
		return
	case parts[0] == "catalog" && r.Method == http.MethodGet:
		n.ltCatalog(w, r)
	case parts[0] == "product" && len(parts) >= 2 && r.Method == http.MethodGet:
		n.ltProductGet(w, r, parts[1])
	case parts[0] == "profile" && r.Method == http.MethodGet:
		n.ltProfileGet(w, r)
	case parts[0] == "photo" && len(parts) >= 2 && r.Method == http.MethodGet:
		n.ltPhotoGet(w, r, parts[1])
	case parts[0] == "order" && len(parts) >= 3 && parts[2] == "status" && r.Method == http.MethodPost:
		n.ltOrderStatus(w, r, parts[1])
	case parts[0] == "order" && len(parts) >= 2 && r.Method == http.MethodGet:
		n.ltGetOrder(w, r, parts[1])
	case parts[0] == "order" && r.Method == http.MethodPost && len(parts) <= 1:
		n.ltPlaceOrder(w, r)
	case parts[0] == "orders" && r.Method == http.MethodGet:
		n.ltListOrders(w, r)
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
	case parts[0] == "interest" && r.Method == http.MethodPost:
		n.ltInterest(w, r)
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "interests":
		n.ltInterests(w, r)
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "orders-seen" && r.Method == http.MethodPost:
		n.ltOrdersSeen(w, r)
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "chat-clear" && r.Method == http.MethodPost:
		n.ltChatClear(w, r)
	case parts[0] == "gestor" && len(parts) >= 2 && parts[1] == "reset" && r.Method == http.MethodPost:
		n.ltStoreReset(w, r)
	case parts[0] == "order" && len(parts) >= 3 && parts[2] == "delete" && r.Method == http.MethodPost:
		n.ltOrderDelete(w, r, parts[1])
	case parts[0] == "events":
		n.ltEventsSSE(w, r)
	default:
		http.NotFound(w, r)
	}
}


func (n *NodoAlset) ltProductGet(w http.ResponseWriter, r *http.Request, id string) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	p, ok := st.Products[id]
	if !ok || p == nil {
		ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
		return
	}
	ltJSON(w, 200, map[string]interface{}{"ok": true, "item": p, "profile": publicProfile(st.Profile)})
}

func (n *NodoAlset) ltCatalog(w http.ResponseWriter, r *http.Request) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	if nrem := ltPurgeSoldOut(ltTen(r), st); nrem > 0 {
		ltBump(st)
		_ = saveLT(ltTen(r), st)
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	cat := strings.TrimSpace(r.URL.Query().Get("category"))
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort")) // newest | oldest | price_asc | price_desc
	if sortBy == "" {
		sortBy = "newest"
	}
	list := make([]*ltProduct, 0)
	catsMap := map[string]int{}
	for _, p := range st.Products {
		if !ltProductAvailable(p) {
			continue
		}
		c := strings.TrimSpace(p.Category)
		if c == "" {
			c = "General"
		}
		catsMap[c]++
		if cat != "" && !strings.EqualFold(c, cat) {
			continue
		}
		if q != "" {
			blob := strings.ToLower(p.Title + " " + p.Description + " " + c)
			if !strings.Contains(blob, q) {
				continue
			}
		}
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		switch sortBy {
		case "oldest":
			return a.CreatedAt < b.CreatedAt
		case "price_asc":
			pa, pb := a.Price, b.Price
			if a.PricePending {
				pa = 1e18
			}
			if b.PricePending {
				pb = 1e18
			}
			if pa == pb {
				return a.CreatedAt > b.CreatedAt
			}
			return pa < pb
		case "price_desc":
			pa, pb := a.Price, b.Price
			if a.PricePending {
				pa = -1
			}
			if b.PricePending {
				pb = -1
			}
			if pa == pb {
				return a.CreatedAt > b.CreatedAt
			}
			return pa > pb
		default: // newest
			return a.CreatedAt > b.CreatedAt
		}
	})
	cats := make([]string, 0, len(catsMap))
	for c := range catsMap {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	// merge configured categories
	for _, c := range st.Categories {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		found := false
		for _, x := range cats {
			if strings.EqualFold(x, c) {
				found = true
				break
			}
		}
		if !found {
			cats = append(cats, c)
		}
	}
	ltJSON(w, 200, map[string]interface{}{
		"ok": true, "items": list, "rev": st.Rev, "profile": publicProfile(st.Profile),
		"categories": cats, "sort": sortBy, "q": q, "category": cat,
	})
}

func (n *NodoAlset) ltProfileGet(w http.ResponseWriter, r *http.Request) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	ltJSON(w, 200, map[string]interface{}{"ok": true, "profile": publicProfile(st.Profile)})
}

func (n *NodoAlset) ltPhotoGet(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSuffix(id, ".jpg")
	b, err := os.ReadFile(ltPhotoPath(ltTen(r), id))
	if err != nil {
		if cf, err2 := ltLoadPhotoCF(ltTen(r), id); err2 == nil && len(cf) > 0 {
			b = cf
			_ = os.MkdirAll(filepath.Join(ltDir(ltTen(r)), "photos"), 0o755)
			_ = os.WriteFile(ltPhotoPath(ltTen(r), id), b, 0o644)
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
	st := loadLT(ltTen(r))
	if strings.TrimSpace(in.Pin) == "" || in.Pin != st.Profile.Pin {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "pin"})
		return
	}
	tok := ltRand(16)
	st.Tokens[tok] = time.Now().Add(30 * 24 * time.Hour).Unix()
	_ = saveLT(ltTen(r), st)
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
	st := loadLT(ltTen(r))
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
	if in.Tagline != "" {
		st.Profile.Tagline = in.Tagline
	}
	if in.Accent != "" {
		st.Profile.Accent = in.Accent
	}
	if in.Accent2 != "" {
		st.Profile.Accent2 = in.Accent2
	}
	if in.Bg != "" {
		st.Profile.Bg = in.Bg
	}
	if in.Surface != "" {
		st.Profile.Surface = in.Surface
	}
	rev := ltBump(st)
	_ = saveLT(ltTen(r), st)
	n.ltNotify("catalog", map[string]interface{}{"rev": rev, "action": "profile", "tenant": ltTen(r)})
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
		st := loadLT(ltTen(r))
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
		st := loadLT(ltTen(r))
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
		// Sin control de cantidad por defecto (el negocio no informa stock).
		if in.Stock < 0 {
			in.Stock = 0
		}
		in.Unlimited = true
		if in.SoldOut {
			if in.SoldOutAt == "" {
				in.SoldOutAt = now
			}
		} else {
			in.SoldOutAt = ""
		}
		in.Category = strings.TrimSpace(in.Category)
		if in.Category == "" {
			in.Category = "General"
		}
		dm := strings.ToLower(strings.TrimSpace(in.DeliveryMode))
		if dm != "pickup" && dm != "delivery" && dm != "both" {
			dm = "both"
		}
		in.DeliveryMode = dm
		in.PickupAddress = strings.TrimSpace(in.PickupAddress)
		in.OwnerName = strings.TrimSpace(in.OwnerName)
		// registrar categoría en el catálogo de la tienda
		foundCat := false
		for _, c := range st.Categories {
			if strings.EqualFold(c, in.Category) {
				foundCat = true
				break
			}
		}
		if !foundCat {
			st.Categories = append(st.Categories, in.Category)
		}
		st.Products[in.ID] = &in
		rev := ltBump(st)
		_ = saveLT(ltTen(r), st)
		n.ltNotify("catalog", map[string]interface{}{"rev": rev, "product_id": in.ID, "action": "upsert", "tenant": ltTen(r)})
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
		ltEnsure(ltTen(r))
		_ = os.WriteFile(ltPhotoPath(ltTen(r), id), b, 0o644)
		if err := ltSavePhotoCF(ltTen(r), id, b); err != nil {
			fmt.Printf("⚠️ La Tati photo CF: %v\n", err)
		}
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT(ltTen(r))
		rev := int64(0)
		if p, ok := st.Products[id]; ok && p != nil {
			p.Photo = true
			p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			rev = ltBump(st)
			_ = saveLT(ltTen(r), st)
		}
		n.ltNotify("catalog", map[string]interface{}{"rev": rev, "product_id": id, "action": "photo", "tenant": ltTen(r)})
		ten := ltTen(r)
		photoURL := "/api/latati/photo/" + id
		if ten != "latati" {
			photoURL = "/api/latati/t/" + ten + "/photo/" + id
		}
		ltJSON(w, 200, map[string]interface{}{"ok": true, "photo": photoURL, "rev": rev})
		return
	}
	if len(rest) >= 2 && r.Method == http.MethodPost {
		id, action := rest[0], rest[1]
		ltMu.Lock()
		defer ltMu.Unlock()
		st := loadLT(ltTen(r))
		p, ok := st.Products[id]
		if !ok || p == nil {
			ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
			return
		}
		switch action {
		case "sold", "soldout", "agotado":
			p.Sold = true
			p.SoldOut = true
			p.SoldOutAt = time.Now().UTC().Format(time.RFC3339)
			p.UpdatedAt = p.SoldOutAt
		case "unsold", "restore", "disponible":
			p.Sold = false
			p.SoldOut = false
			p.SoldOutAt = ""
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
			_ = os.Remove(ltPhotoPath(ltTen(r), id))
			ltDeletePhotoCF(ltTen(r), id)
			rev := ltBump(st)
			_ = saveLT(ltTen(r), st)
			n.ltNotify("catalog", map[string]interface{}{"rev": rev, "product_id": id, "action": "delete", "tenant": ltTen(r)})
			ltJSON(w, 200, map[string]interface{}{"ok": true, "rev": rev})
			return
		default:
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "action"})
			return
		}
		p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		rev := ltBump(st)
		_ = saveLT(ltTen(r), st)
		n.ltNotify("catalog", map[string]interface{}{"rev": rev, "product_id": id, "action": action, "tenant": ltTen(r)})
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": p, "rev": rev})
		return
	}
	http.Error(w, "method", 405)
}

func (n *NodoAlset) ltPlaceOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Thread         string `json:"thread"`
		ClientName     string `json:"client_name"`
		ClientPhone    string `json:"client_phone"`
		Note           string `json:"note"`
		DeliveryType   string `json:"delivery_type"` // pickup | delivery
		DeliveryAddr   string `json:"delivery_address"`
		DeliveryNote   string `json:"delivery_note"` // hora / detalles
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
	st := loadLT(ltTen(r))

	var lines []ltLine
	total := 0.0
	currency := "USD"
	for _, it := range in.Items {
		if it.Qty < 1 {
			it.Qty = 1
		}
		it.Qty = 1 // un producto = una unidad por pedido
		p, ok := st.Products[it.ProductID]
		if !ok || p == nil || !ltProductAvailable(p) {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "product unavailable: " + it.ProductID})
			return
		}
		if p.PricePending || p.Price <= 0 {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "product without price: " + p.Title})
			return
		}
		// Stock solo se valida si el producto no es unlimited y stock>0 explícito de control
		if !p.Unlimited && p.Stock > 0 && p.Stock < it.Qty {
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

	// Modalidad de entrega
	dtype := strings.ToLower(strings.TrimSpace(in.DeliveryType))
	if dtype != "delivery" {
		dtype = "pickup"
	}
	daddr := strings.TrimSpace(in.DeliveryAddr)
	// Restricciones por producto
	for _, l := range lines {
		p := st.Products[l.ProductID]
		if p == nil {
			continue
		}
		mode := strings.ToLower(strings.TrimSpace(p.DeliveryMode))
		if mode == "" {
			mode = "both"
		}
		if mode == "delivery" && dtype != "delivery" {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "«" + p.Title + "» solo se entrega a domicilio"})
			return
		}
		if mode == "pickup" && dtype == "delivery" {
			ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "«" + p.Title + "» solo es para recogida"})
			return
		}
	}
	if dtype == "delivery" && daddr == "" {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "indica la dirección de entrega"})
		return
	}

	// Dirección / dueño de recogida: del producto si existe, si no del perfil de la tienda
	pickupAddr := strings.TrimSpace(st.Profile.Address)
	ownerName := strings.TrimSpace(st.Profile.Name)
	for _, l := range lines {
		p := st.Products[l.ProductID]
		if p == nil {
			continue
		}
		if strings.TrimSpace(p.PickupAddress) != "" {
			pickupAddr = strings.TrimSpace(p.PickupAddress)
		}
		if strings.TrimSpace(p.OwnerName) != "" {
			ownerName = strings.TrimSpace(p.OwnerName)
		}
		break
	}
	st.SeqOrder++
	ord := ltOrder{
		ID: ltRand(8), Code: fmt.Sprintf("LT-%04d", st.SeqOrder),
		Thread: in.Thread, ClientName: strings.TrimSpace(in.ClientName),
		ClientPhone: strings.TrimSpace(in.ClientPhone),
		Lines: lines, Total: total, Currency: currency,
		Status: "requested", Address: pickupAddr,
		DeliveryType: dtype, DeliveryAddr: daddr,
		DeliveryNote: strings.TrimSpace(in.DeliveryNote),
		PickupAddress: pickupAddr, OwnerName: ownerName,
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
		Text: fmt.Sprintf("Solicitud %s · %s · total %s %.2f · %d ítem(s) · esperando confirmación", ord.Code, func() string {
			if ord.DeliveryType == "delivery" {
				return "domicilio"
			}
			return "recogida"
		}(), ord.Currency, ord.Total, len(ord.Lines)),
		Ts:   ord.Ts,
	})
	rev := ltBump(st)
	_ = saveLT(ltTen(r), st)
	n.ltNotify("order", map[string]interface{}{"rev": rev, "order_id": ord.ID, "code": ord.Code, "thread": ord.Thread, "status": "requested", "tenant": ltTen(r)})
	ltJSON(w, 200, map[string]interface{}{"ok": true, "order": ord, "rev": rev, "message": "Solicitud enviada. La gestora confirmará disponibilidad."})
}

func (n *NodoAlset) ltListOrders(w http.ResponseWriter, r *http.Request) {
	thread := strings.TrimSpace(r.URL.Query().Get("thread"))
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
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
	pending := 0
	for _, o := range st.Orders {
		if o.Status == "requested" {
			pending++
		}
	}
	unseenInterests := 0
	for _, it := range st.Interests {
		if !it.Seen {
			unseenInterests++
		}
	}
	ltJSON(w, 200, map[string]interface{}{
		"ok": true, "items": st.Orders, "rev": st.Rev, "orders_seen": st.OrdersSeen,
		"pending_count": pending, "interest_count": unseenInterests,
	})
}

func (n *NodoAlset) ltGetOrder(w http.ResponseWriter, r *http.Request, id string) {
	thread := strings.TrimSpace(r.URL.Query().Get("thread"))
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
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
		if p.Unlimited {
			continue
		}
		if dir > 0 {
			if p.SoldOut || p.Sold {
				return "agotado: " + l.Title
			}
			if p.Stock > 0 && p.Stock < l.Qty {
				return "stock: " + l.Title
			}
			if p.Stock > 0 {
				p.Stock -= l.Qty
			}
		} else {
			if p.Stock >= 0 {
				p.Stock += l.Qty
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
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	in.Status = strings.TrimSpace(strings.ToLower(in.Status))
	in.Reason = strings.TrimSpace(in.Reason)
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
	st := loadLT(ltTen(r))
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
				// Datos de la gestora (perfil de la tienda) siempre en el vale
				o.GestorName = st.Profile.Name
				o.GestorPhone = st.Profile.WhatsApp
				if o.PickupAddress == "" {
					o.PickupAddress = st.Profile.Address
				}
				if o.Address == "" {
					o.Address = o.PickupAddress
					if o.Address == "" {
						o.Address = st.Profile.Address
					}
				}
				if o.OwnerName == "" {
					o.OwnerName = st.Profile.Name
				}
				msg := fmt.Sprintf("✅ Pedido %s confirmado. Preparando entrega.", o.Code)
				if in.Status == "ready" {
					if o.DeliveryType == "delivery" {
						msg = fmt.Sprintf("✅ Pedido %s listo. Muestre su vale al domicilio.", o.Code)
					} else {
						msg = fmt.Sprintf("✅ Pedido %s listo. Muestre su vale para recoger.", o.Code)
					}
				}
				st.Messages = append(st.Messages, ltMsg{
					ID: ltRand(6), Thread: o.Thread, From: "gestor",
					Text: msg,
					Ts:   time.Now().UTC().Format(time.RFC3339),
				})
			} else if prev == "cancelled" || prev == "delivered" {
				ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "estado no permite esta acción"})
				return
			}
			// pending → ready: mensaje de listo
			if prev == "pending" && in.Status == "ready" {
				o.GestorName = st.Profile.Name
				o.GestorPhone = st.Profile.WhatsApp
				msg := fmt.Sprintf("✅ Pedido %s listo. Muestre su vale para recoger.", o.Code)
				if o.DeliveryType == "delivery" {
					msg = fmt.Sprintf("✅ Pedido %s listo. Muestre su vale al domicilio.", o.Code)
				}
				st.Messages = append(st.Messages, ltMsg{
					ID: ltRand(6), Thread: o.Thread, From: "gestor",
					Text: msg,
					Ts:   time.Now().UTC().Format(time.RFC3339),
				})
			}
			o.Status = in.Status
		} else if in.Status == "cancelled" {
			// Si ya se había confirmado (stock descontado), devolver stock
			if prev == "pending" || prev == "ready" {
				_ = ltApplyStock(st, o.Lines, -1)
			}
			o.Status = "cancelled"
			if gestor && in.Reason != "" {
				o.CancelReason = in.Reason
			}
			who := "client"
			if gestor {
				who = "gestor"
			}
			txt := fmt.Sprintf("Pedido %s cancelado.", o.Code)
			if o.CancelReason != "" {
				txt = fmt.Sprintf("Pedido %s cancelado. Motivo: %s", o.Code, o.CancelReason)
			}
			st.Messages = append(st.Messages, ltMsg{
				ID: ltRand(6), Thread: o.Thread, From: who,
				Text: txt,
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
		_ = saveLT(ltTen(r), st)
		n.ltNotify("order", map[string]interface{}{"rev": rev, "order_id": o.ID, "code": o.Code, "status": o.Status, "thread": o.Thread, "cancel_reason": o.CancelReason, "tenant": ltTen(r)})
		if prev == "requested" && (o.Status == "pending" || o.Status == "ready") {
			n.ltNotify("catalog", map[string]interface{}{"rev": rev, "action": "stock_after_confirm", "tenant": ltTen(r)})
		}
		if o.Status == "cancelled" {
			n.ltNotify("chat", map[string]interface{}{"rev": rev, "thread": o.Thread, "from": "gestor", "tenant": ltTen(r)})
			if prev == "pending" || prev == "ready" {
				n.ltNotify("catalog", map[string]interface{}{"rev": rev, "action": "stock_after_cancel", "tenant": ltTen(r)})
			}
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
		st := loadLT(ltTen(r))
		var out []ltMsg
		for _, m := range st.Messages {
			if thread == "" || m.Thread == thread {
				out = append(out, m)
			}
		}
		if len(out) > 200 {
			out = out[len(out)-200:]
		}
		ltJSON(w, 200, map[string]interface{}{"ok": true, "items": out, "messages": out})
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
				st := loadLT(ltTen(r))
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
		st := loadLT(ltTen(r))
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
		_ = saveLT(ltTen(r), st)
		n.ltNotify("chat", map[string]interface{}{"rev": rev, "thread": msg.Thread, "from": msg.From, "tenant": ltTen(r)})
		ltJSON(w, 200, map[string]interface{}{"ok": true, "item": msg, "rev": rev})
		return
	}
	http.Error(w, "method", 405)
}

func (n *NodoAlset) ltThreads(w http.ResponseWriter, r *http.Request) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
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
	tenant := ltTen(r)
	ltMu.Lock()
	rev := loadLT(tenant).Rev
	ltMu.Unlock()
	hello, _ := json.Marshal(map[string]interface{}{"ok": true, "app": "latati", "rev": rev, "tenant": tenant})
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


func (n *NodoAlset) ltInterest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID   string `json:"product_id"`
		Thread      string `json:"thread"`
		ClientName  string `json:"client_name"`
		ClientPhone string `json:"client_phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "json"})
		return
	}
	in.ProductID = strings.TrimSpace(in.ProductID)
	in.Thread = strings.TrimSpace(in.Thread)
	in.ClientName = strings.TrimSpace(in.ClientName)
	in.ClientPhone = strings.TrimSpace(in.ClientPhone)
	if in.ProductID == "" || in.Thread == "" || in.ClientName == "" || in.ClientPhone == "" {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "datos incompletos"})
		return
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	p, ok := st.Products[in.ProductID]
	if !ok || p == nil || !ltProductAvailable(p) {
		ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "producto no disponible"})
		return
	}
	if !p.PricePending && p.Price > 0 {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "el producto ya tiene precio"})
		return
	}
	// evitar duplicados del mismo cliente+producto abiertos
	for i := range st.Interests {
		it := &st.Interests[i]
		if it.ProductID == in.ProductID && it.Thread == in.Thread && !it.Seen {
			ltJSON(w, 200, map[string]interface{}{"ok": true, "item": *it, "duplicate": true})
			return
		}
	}
	item := ltInterest{
		ID: ltRand(8), ProductID: in.ProductID, Title: p.Title,
		Thread: in.Thread, ClientName: in.ClientName, ClientPhone: in.ClientPhone,
		Ts: time.Now().UTC().Format(time.RFC3339),
	}
	st.Interests = append([]ltInterest{item}, st.Interests...)
	if len(st.Interests) > 500 {
		st.Interests = st.Interests[:500]
	}
	st.Messages = append(st.Messages, ltMsg{
		ID: ltRand(6), Thread: in.Thread, From: "client",
		Text: "Interés en precio: " + p.Title,
		ProductID: p.ID, Ts: item.Ts,
	})
	rev := ltBump(st)
	_ = saveLT(ltTen(r), st)
	n.ltNotify("interest", map[string]interface{}{"rev": rev, "product_id": p.ID, "title": p.Title, "client": in.ClientName, "tenant": ltTen(r)})
	n.ltNotify("chat", map[string]interface{}{"rev": rev, "thread": in.Thread, "from": "client", "tenant": ltTen(r)})
	ltJSON(w, 200, map[string]interface{}{"ok": true, "item": item, "rev": rev})
}

func (n *NodoAlset) ltInterests(w http.ResponseWriter, r *http.Request) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	if !ltAuthFrom(r, st) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	if r.Method == http.MethodPost {
		var in struct {
			ID     string `json:"id"`
			Seen   bool   `json:"seen"`
			Delete bool   `json:"delete"`
			Clear  bool   `json:"clear_read"` // borrar todos los leídos
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Clear {
			out := st.Interests[:0]
			for _, it := range st.Interests {
				if !it.Seen {
					out = append(out, it)
				}
			}
			st.Interests = out
			_ = saveLT(ltTen(r), st)
			ltJSON(w, 200, map[string]interface{}{"ok": true, "items": st.Interests})
			return
		}
		if in.Delete && in.ID != "" {
			out := st.Interests[:0]
			found := false
			for _, it := range st.Interests {
				if it.ID == in.ID {
					found = true
					continue
				}
				out = append(out, it)
			}
			if !found {
				ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
				return
			}
			st.Interests = out
			_ = saveLT(ltTen(r), st)
			ltJSON(w, 200, map[string]interface{}{"ok": true, "deleted": in.ID})
			return
		}
		for i := range st.Interests {
			if st.Interests[i].ID == in.ID {
				st.Interests[i].Seen = in.Seen
				_ = saveLT(ltTen(r), st)
				ltJSON(w, 200, map[string]interface{}{"ok": true, "item": st.Interests[i]})
				return
			}
		}
		ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
		return
	}
	ltJSON(w, 200, map[string]interface{}{"ok": true, "items": st.Interests})
}

func (n *NodoAlset) ltOrdersSeen(w http.ResponseWriter, r *http.Request) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	if !ltAuthFrom(r, st) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	st.OrdersSeen = st.Rev
	_ = saveLT(ltTen(r), st)
	ltJSON(w, 200, map[string]interface{}{"ok": true, "orders_seen": st.OrdersSeen, "rev": st.Rev})
}

func (n *NodoAlset) ltOrderDelete(w http.ResponseWriter, r *http.Request, id string) {
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	if !ltAuthFrom(r, st) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	out := st.Orders[:0]
	found := false
	for _, o := range st.Orders {
		if o.ID == id || o.Code == id {
			if o.Status != "cancelled" && o.Status != "delivered" {
				ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "solo cancelados o entregados"})
				return
			}
			found = true
			continue
		}
		out = append(out, o)
	}
	if !found {
		ltJSON(w, 404, map[string]interface{}{"ok": false, "error": "not found"})
		return
	}
	st.Orders = out
	rev := ltBump(st)
	_ = saveLT(ltTen(r), st)
	n.ltNotify("order", map[string]interface{}{"rev": rev, "action": "delete", "order_id": id, "tenant": ltTen(r)})
	ltJSON(w, 200, map[string]interface{}{"ok": true, "rev": rev})
}

// ltChatClear elimina mensajes de un hilo o de todo el historial (solo gestora).
func (n *NodoAlset) ltChatClear(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Thread string `json:"thread"`
		All    bool   `json:"all"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	in.Thread = strings.TrimSpace(in.Thread)

	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	if !ltAuthFrom(r, st) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	if in.All {
		st.Messages = nil
	} else if in.Thread != "" {
		out := st.Messages[:0]
		for _, m := range st.Messages {
			if m.Thread != in.Thread {
				out = append(out, m)
			}
		}
		st.Messages = out
	} else {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "indica thread o all"})
		return
	}
	rev := ltBump(st)
	_ = saveLT(ltTen(r), st)
	n.ltNotify("chat", map[string]interface{}{"rev": rev, "action": "clear", "thread": in.Thread, "all": in.All, "tenant": ltTen(r)})
	ltJSON(w, 200, map[string]interface{}{"ok": true, "rev": rev, "all": in.All, "thread": in.Thread})
}

// ltStoreReset borra datos operativos y reinicia el contador de pedidos a 0
// (siguiente pedido será LT-0001). Conserva perfil, productos y categorías.
func (n *NodoAlset) ltStoreReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirm string `json:"confirm"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if strings.TrimSpace(strings.ToUpper(in.Confirm)) != "RESET" {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "escribe confirm: RESET"})
		return
	}
	ltMu.Lock()
	defer ltMu.Unlock()
	st := loadLT(ltTen(r))
	if !ltAuthFrom(r, st) {
		ltJSON(w, 401, map[string]interface{}{"ok": false, "error": "auth"})
		return
	}
	st.Orders = nil
	st.Messages = nil
	st.Interests = nil
	st.Tokens = map[string]int64{}
	st.SeqOrder = 0
	st.OrdersSeen = 0
	rev := ltBump(st)
	_ = saveLT(ltTen(r), st)
	n.ltNotify("order", map[string]interface{}{"rev": rev, "action": "reset", "tenant": ltTen(r)})
	n.ltNotify("catalog", map[string]interface{}{"rev": rev, "action": "reset", "tenant": ltTen(r)})
	n.ltNotify("chat", map[string]interface{}{"rev": rev, "action": "reset", "tenant": ltTen(r)})
	ltJSON(w, 200, map[string]interface{}{
		"ok": true, "rev": rev,
		"message": "Datos operativos borrados. El próximo pedido será LT-0001. Perfil y catálogo conservados.",
	})
}


func (n *NodoAlset) ltLoadRegistry() []ltTenantMeta {
	list := []ltTenantMeta{{Slug: "latati", Name: "La Tati", Active: true}}
	if !ltCFEnabled() {
		// disco
		b, err := os.ReadFile(filepath.Join("alset_data", "latati", "tenants.json"))
		if err == nil {
			var reg []ltTenantMeta
			if json.Unmarshal(b, &reg) == nil && len(reg) > 0 {
				return reg
			}
		}
		return list
	}
	cli, err := ltCFClient()
	if err != nil {
		return list
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := cli.Load(ctx, ltRegistryKey)
	if err != nil || len(raw) == 0 {
		return list
	}
	var reg []ltTenantMeta
	if json.Unmarshal(raw, &reg) != nil || len(reg) == 0 {
		return list
	}
	// ensure latati present
	has := false
	for _, x := range reg {
		if x.Slug == "latati" {
			has = true
			break
		}
	}
	if !has {
		reg = append([]ltTenantMeta{{Slug: "latati", Name: "La Tati", Active: true}}, reg...)
	}
	return reg
}

func (n *NodoAlset) ltSaveRegistry(reg []ltTenantMeta) error {
	raw, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Join("alset_data", "latati"), 0o755)
	_ = os.WriteFile(filepath.Join("alset_data", "latati", "tenants.json"), raw, 0o644)
	if !ltCFEnabled() {
		return nil
	}
	cli, err := ltCFClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return cli.Save(ctx, ltRegistryKey, raw)
}

func (n *NodoAlset) ltRegisterTenantApps(slug string) {
	slug = ltNormTenant(slug)
	if slug == "" || slug == "latati" {
		return
	}
	if len(latatiAppHTML) == 0 {
		return
	}
	cid, err := n.GenerarCID(latatiAppHTML)
	if err != nil || cid == "" {
		return
	}
	gCID := cid
	if len(latatiGestionHTML) > 0 {
		if c2, err2 := n.GenerarCID(latatiGestionHTML); err2 == nil && c2 != "" {
			gCID = c2
		}
	}
	shopAlias := slug + ".app.ans"
	gestionAlias := "gestion-" + slug + ".app.ans"
	shopID := "app-lt-" + slug
	gestionID := "app-lt-gestion-" + slug
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	n.agentes[shopID] = &Agente{ID: shopID, RootCID: cid, BalanceUTXO: 0, UltimaActual: time.Now().Unix()}
	n.nombres[shopAlias] = shopID
	n.agentes[gestionID] = &Agente{ID: gestionID, RootCID: gCID, BalanceUTXO: 0, UltimaActual: time.Now().Unix()}
	n.nombres[gestionAlias] = gestionID
	// materializar HTML en static para resolución por filesystem
	dir := filepath.Join(StaticDir, "apps", slug)
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "index.html"), latatiAppHTML, 0644)
	gdir := filepath.Join(StaticDir, "apps", "gestion-"+slug)
	_ = os.MkdirAll(gdir, 0755)
	if len(latatiGestionHTML) > 0 {
		_ = os.WriteFile(filepath.Join(gdir, "index.html"), latatiGestionHTML, 0644)
	}
	fmt.Printf("✅ Tenant La Tati [%s]: /w/%s · /w/%s\n", slug, shopAlias, gestionAlias)
}

func (n *NodoAlset) ltTenants(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		ltMu.Lock()
		reg := n.ltLoadRegistry()
		ltMu.Unlock()
		ltJSON(w, 200, map[string]interface{}{"ok": true, "items": reg})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var in struct {
		Slug     string `json:"slug"`
		Name     string `json:"name"`
		Pin      string `json:"pin"`
		WhatsApp string `json:"whatsapp"`
		Address  string `json:"address"`
		Bio      string `json:"bio"`
		Tagline  string `json:"tagline"`
		Accent   string `json:"accent"`
		Accent2  string `json:"accent2"`
		Bg       string `json:"bg"`
		Surface  string `json:"surface"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "json"})
		return
	}
	slug := ltNormTenant(in.Slug)
	if slug == "" || slug == "latati" || slug == "t" || slug == "tenants" {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "slug inválido"})
		return
	}
	if len(slug) < 2 || len(slug) > 32 {
		ltJSON(w, 400, map[string]interface{}{"ok": false, "error": "slug 2-32 caracteres"})
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = slug
	}
	pin := strings.TrimSpace(in.Pin)
	if pin == "" {
		pin = "1234"
	}

	ltMu.Lock()
	reg := n.ltLoadRegistry()
	for _, x := range reg {
		if x.Slug == slug {
			ltMu.Unlock()
			ltJSON(w, 409, map[string]interface{}{"ok": false, "error": "slug ya existe"})
			return
		}
	}
	// store inicial
	st := &ltStore{
		Profile: ltProfile{
			Name: name, WhatsApp: strings.TrimSpace(in.WhatsApp),
			Address: strings.TrimSpace(in.Address), Bio: strings.TrimSpace(in.Bio),
			Pin: pin, Tagline: strings.TrimSpace(in.Tagline),
			Accent: strings.TrimSpace(in.Accent), Accent2: strings.TrimSpace(in.Accent2),
			Bg: strings.TrimSpace(in.Bg), Surface: strings.TrimSpace(in.Surface),
		},
		Products: map[string]*ltProduct{},
		Tokens:   map[string]int64{},
		Orders:   []ltOrder{},
	}
	if st.Profile.Accent == "" {
		st.Profile.Accent = "#e8b4c8"
	}
	if st.Profile.Accent2 == "" {
		st.Profile.Accent2 = "#c4789a"
	}
	ltMemMap[slug] = st
	_ = saveLT(slug, st)
	meta := ltTenantMeta{Slug: slug, Name: name, CreatedAt: time.Now().UTC().Format(time.RFC3339), Active: true}
	reg = append(reg, meta)
	_ = n.ltSaveRegistry(reg)
	ltMu.Unlock()

	n.ltRegisterTenantApps(slug)

	ltJSON(w, 200, map[string]interface{}{
		"ok": true, "tenant": meta,
		"shop": "/w/" + slug + ".app.ans",
		"gestion": "/w/gestion-" + slug + ".app.ans",
		"api": "/api/latati/t/" + slug,
		"pin": pin,
	})
}

func (n *NodoAlset) registerLaTatiAPI(extra map[string]http.HandlerFunc) {
	extra["/api/latati/"] = n.handleLaTatiAPI
	extra["/api/latati"] = n.handleLaTatiAPI
	extra["/api/latati/events"] = n.ltEventsSSE
}
