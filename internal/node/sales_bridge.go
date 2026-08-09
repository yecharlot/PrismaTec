package node

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Sales bridge: fallback identity + profile store when Supabase Auth is limited.
// Transparent alternative for Alset Sales Hub.

type salesUser struct {
	ID           string                 `json:"id"`
	Username     string                 `json:"username"`
	Role         string                 `json:"role"`
	PasswordHash string                 `json:"password_hash"`
	Salt         string                 `json:"salt"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ProfileCID   string                 `json:"profile_cid,omitempty"`
	CreatedAt    string                 `json:"created_at"`
}

type salesStore struct {
	Users  map[string]*salesUser `json:"users"` // key = username lower
	Tokens map[string]string     `json:"tokens"` // token -> username
}

var (
	salesMu    sync.Mutex
	salesMem   *salesStore
	salesFile  = "alset_data/sales_users.json"
)

func (n *NodoAlset) registerSalesBridge(h map[string]http.HandlerFunc) {
	h["/api/sales/register"] = n.handleSalesRegister
	h["/api/sales/login"] = n.handleSalesLogin
	h["/api/sales/me"] = n.handleSalesMe
	h["/api/sales/profile"] = n.handleSalesProfile
	h["/api/sales/info"] = n.handleSalesInfo
}

func (n *NodoAlset) handleSalesInfo(w http.ResponseWriter, r *http.Request) {
	corsSales(w, r)
	if r.Method == http.MethodOptions {
		return
	}
	st := n.loadSalesStore()
	salesMu.Lock()
	nUsers := len(st.Users)
	salesMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    "Alset Sales Bridge",
		"backend": "alset+ipfs",
		"users":   nUsers,
		"endpoints": []string{
			"POST /api/sales/register",
			"POST /api/sales/login",
			"GET  /api/sales/me",
			"POST /api/sales/profile",
			"GET  /api/sales/info",
		},
	})
}

func corsSales(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
	}
}

func (n *NodoAlset) loadSalesStore() *salesStore {
	salesMu.Lock()
	defer salesMu.Unlock()
	if salesMem != nil {
		return salesMem
	}
	st := &salesStore{Users: map[string]*salesUser{}, Tokens: map[string]string{}}
	data, err := os.ReadFile(salesFile)
	if err == nil {
		_ = json.Unmarshal(data, st)
		if st.Users == nil {
			st.Users = map[string]*salesUser{}
		}
		if st.Tokens == nil {
			st.Tokens = map[string]string{}
		}
	}
	salesMem = st
	return salesMem
}

func (n *NodoAlset) saveSalesStore() error {
	salesMu.Lock()
	defer salesMu.Unlock()
	if salesMem == nil {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(salesFile), 0o755)
	data, err := json.MarshalIndent(salesMem, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(salesFile, data, 0o600); err != nil {
		return err
	}
	// Also pin snapshot in blockstore / IPFS-style CID
	cid, err := n.GenerarCID(data)
	if err == nil && cid != "" {
		n.PutBlock(cid, data)
	}
	return nil
}

func hashSalesPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

func newSalesSalt() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func newSalesToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "als_" + hex.EncodeToString(b)
}

func (n *NodoAlset) handleSalesRegister(w http.ResponseWriter, r *http.Request) {
	corsSales(w, r)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Username string                 `json:"username"`
		Password string                 `json:"password"`
		Role     string                 `json:"role"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	user := strings.ToLower(strings.TrimSpace(req.Username))
	if user == "" || len(req.Password) < 6 {
		http.Error(w, "username y password (>=6) requeridos", 400)
		return
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = "cliente"
	}
	if role != "empresa" && role != "gestor" && role != "cliente" {
		role = "cliente"
	}

	st := n.loadSalesStore()
	salesMu.Lock()
	if _, exists := st.Users[user]; exists {
		salesMu.Unlock()
		http.Error(w, "usuario ya existe en Alset", 409)
		return
	}
	salt := newSalesSalt()
	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)
	meta := req.Metadata
	if meta == nil {
		meta = map[string]interface{}{}
	}
	meta["source"] = "alset"
	meta["username"] = user
	meta["role"] = role

	// Profile blob to IPFS/blockstore
	profileDoc := map[string]interface{}{
		"id":       id,
		"username": user,
		"role":     role,
		"metadata": meta,
		"created":  time.Now().UTC().Format(time.RFC3339),
	}
	profBytes, _ := json.Marshal(profileDoc)
	cid, _ := n.GenerarCID(profBytes)
	if cid != "" {
		n.PutBlock(cid, profBytes)
	}

	u := &salesUser{
		ID:           id,
		Username:     user,
		Role:         role,
		PasswordHash: hashSalesPassword(req.Password, salt),
		Salt:         salt,
		Metadata:     meta,
		ProfileCID:   cid,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	st.Users[user] = u
	token := newSalesToken()
	st.Tokens[token] = user
	salesMu.Unlock()

	if err := n.saveSalesStore(); err != nil {
		fmt.Println("sales save:", err)
	}
	n.PersistirLocamente()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"source":  "alset",
		"token":   token,
		"user": map[string]interface{}{
			"id":       id,
			"username": user,
			"role":     role,
			"metadata": meta,
			"profile_cid": cid,
		},
	})
}

func (n *NodoAlset) handleSalesLogin(w http.ResponseWriter, r *http.Request) {
	corsSales(w, r)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	user := strings.ToLower(strings.TrimSpace(req.Username))
	st := n.loadSalesStore()
	salesMu.Lock()
	u, ok := st.Users[user]
	if !ok || hashSalesPassword(req.Password, u.Salt) != u.PasswordHash {
		salesMu.Unlock()
		http.Error(w, "credenciales inválidas", 401)
		return
	}
	token := newSalesToken()
	st.Tokens[token] = user
	out := map[string]interface{}{
		"ok":     true,
		"source": "alset",
		"token":  token,
		"user": map[string]interface{}{
			"id":          u.ID,
			"username":    u.Username,
			"role":        u.Role,
			"metadata":    u.Metadata,
			"profile_cid": u.ProfileCID,
		},
	}
	salesMu.Unlock()
	_ = n.saveSalesStore()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (n *NodoAlset) salesUserFromAuth(r *http.Request) *salesUser {
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	token = strings.TrimSpace(token)
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return nil
	}
	st := n.loadSalesStore()
	salesMu.Lock()
	defer salesMu.Unlock()
	user, ok := st.Tokens[token]
	if !ok {
		return nil
	}
	return st.Users[user]
}

func (n *NodoAlset) handleSalesMe(w http.ResponseWriter, r *http.Request) {
	corsSales(w, r)
	if r.Method == http.MethodOptions {
		return
	}
	u := n.salesUserFromAuth(r)
	if u == nil {
		http.Error(w, "no autorizado", 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"source": "alset",
		"user": map[string]interface{}{
			"id":          u.ID,
			"username":    u.Username,
			"role":        u.Role,
			"metadata":    u.Metadata,
			"profile_cid": u.ProfileCID,
			"created_at":  u.CreatedAt,
		},
	})
}

func (n *NodoAlset) handleSalesProfile(w http.ResponseWriter, r *http.Request) {
	corsSales(w, r)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	u := n.salesUserFromAuth(r)
	if u == nil {
		http.Error(w, "no autorizado", 401)
		return
	}
	var req struct {
		Metadata map[string]interface{} `json:"metadata"`
		Role     string                 `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	st := n.loadSalesStore()
	salesMu.Lock()
	uu := st.Users[u.Username]
	if uu == nil {
		salesMu.Unlock()
		http.Error(w, "usuario no encontrado", 404)
		return
	}
	if req.Metadata != nil {
		if uu.Metadata == nil {
			uu.Metadata = map[string]interface{}{}
		}
		for k, v := range req.Metadata {
			uu.Metadata[k] = v
		}
	}
	if req.Role != "" {
		uu.Role = req.Role
	}
	profileDoc := map[string]interface{}{
		"id":       uu.ID,
		"username": uu.Username,
		"role":     uu.Role,
		"metadata": uu.Metadata,
		"updated":  time.Now().UTC().Format(time.RFC3339),
	}
	profBytes, _ := json.Marshal(profileDoc)
	cid, _ := n.GenerarCID(profBytes)
	if cid != "" {
		n.PutBlock(cid, profBytes)
		uu.ProfileCID = cid
	}
	salesMu.Unlock()
	_ = n.saveSalesStore()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":          true,
		"source":      "alset",
		"profile_cid": cid,
		"user": map[string]interface{}{
			"id": uu.ID, "username": uu.Username, "role": uu.Role, "metadata": uu.Metadata, "profile_cid": cid,
		},
	})
}
