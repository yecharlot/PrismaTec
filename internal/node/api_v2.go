package node

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// registerAPIv2 mounts developer-oriented v2 endpoints on the Extra map.
func (n *NodoAlset) registerAPIv2(h map[string]http.HandlerFunc) {
	h["/api/v2/info"] = n.handleInfoV2
	h["/api/v2/agente/crear"] = n.apiCrearAgenteV2
	h["/api/v2/transferir"] = n.apiTransferirV2
	h["/api/v2/app/publicar"] = n.apiPublicarAppV2
	h["/api/v2/app/instalar"] = n.apiInstalarAppV2
	h["/api/v2/app/ejecutar"] = n.apiEjecutarAppV2
}

func (n *NodoAlset) handleInfoV2(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	peers := 0
	if n.host != nil {
		peers = len(n.host.Network().Peers())
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":        "Alset / PrismaTec",
		"api_version": "v2",
		"peer_id":     n.PeerID(),
		"peers":       peers,
		"agents":      len(n.agentes),
		"endpoints": []string{
			"GET  /api/v2/info",
			"POST /api/v2/agente/crear",
			"POST /api/v2/transferir",
			"POST /api/v2/app/publicar",
			"POST /api/v2/app/instalar",
			"POST /api/v2/app/ejecutar",
			"POST /api/sales/register",
			"POST /api/sales/login",
			"GET  /api/sales/me",
			"GET  /api/sales/info",
		},
	})
}

func (n *NodoAlset) apiCrearAgenteV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		ID      string  `json:"id"`
		Balance float64 `json:"balance"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	id := req.ID
	if id == "" {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id = hex.EncodeToString(b)
	}
	bal := req.Balance
	if bal == 0 {
		bal = 1000
	}
	n.mu.Lock()
	if _, exists := n.agentes[id]; exists {
		n.mu.Unlock()
		http.Error(w, "agente ya existe", 409)
		return
	}
	ag := &Agente{ID: id, BalanceUTXO: bal, UltimaActual: time.Now().Unix()}
	n.agentes[id] = ag
	n.mu.Unlock()
	n.PersistirLocamente()
	go n.SincronizarConPares()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ag)
}

func (n *NodoAlset) apiTransferirV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		From   string  `json:"from"`
		To     string  `json:"to"`
		Amount float64 `json:"amount"`
		Token  string  `json:"token"` // optional Alset auth token
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if req.From == "" || req.To == "" || req.Amount <= 0 {
		http.Error(w, "from, to y amount requeridos", 400)
		return
	}
	if req.Token != "" {
		if _, err := n.validarToken(req.Token); err != nil {
			http.Error(w, "token inválido: "+err.Error(), 401)
			return
		}
	}
	n.mu.Lock()
	from, okFrom := n.agentes[req.From]
	to, okTo := n.agentes[req.To]
	if !okFrom || !okTo {
		n.mu.Unlock()
		http.Error(w, "agente origen o destino no encontrado", 404)
		return
	}
	if from.BalanceUTXO < req.Amount {
		n.mu.Unlock()
		http.Error(w, "saldo insuficiente", 400)
		return
	}
	from.BalanceUTXO -= req.Amount
	to.BalanceUTXO += req.Amount
	from.UltimaActual = time.Now().Unix()
	to.UltimaActual = time.Now().Unix()
	n.mu.Unlock()
	n.PersistirLocamente()
	go n.SincronizarConPares()
	txid := fmt.Sprintf("tx-%d", time.Now().UnixNano())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"txid":    txid,
		"from":    req.From,
		"to":      req.To,
		"amount":  req.Amount,
	})
}

func (n *NodoAlset) apiPublicarAppV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "body requerido (HTML o bytes)", 400)
		return
	}
	var meta struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	content := body
	name := "app"
	if json.Unmarshal(body, &meta) == nil && meta.Content != "" {
		content = []byte(meta.Content)
		if meta.Name != "" {
			name = meta.Name
		}
	}
	cid, err := n.GenerarCID(content)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	appID := fmt.Sprintf("app-%s-%d", name, time.Now().Unix())
	n.mu.Lock()
	n.agentes[appID] = &Agente{ID: appID, RootCID: cid, BalanceUTXO: 1000, UltimaActual: time.Now().Unix()}
	if n.nombres == nil {
		n.nombres = map[string]string{}
	}
	n.nombres[name+".app.ans"] = appID
	n.mu.Unlock()
	n.AnunciarNuevoBloque(cid)
	n.PersistirLocamente()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "published",
		"name":     name,
		"agent_id": appID,
		"cid":      cid,
		"url":      "/w/" + name + ".app.ans",
	})
}

func (n *NodoAlset) apiInstalarAppV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		CID  string `json:"cid"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CID == "" {
		http.Error(w, "cid requerido", 400)
		return
	}
	data, err := n.BuscarContenidoPorCID(req.CID)
	if err != nil || len(data) == 0 {
		http.Error(w, "no se pudo obtener CID", 404)
		return
	}
	name := req.Name
	if name == "" {
		name = "installed"
	}
	appID := fmt.Sprintf("app-%s-%d", name, time.Now().Unix())
	n.mu.Lock()
	n.agentes[appID] = &Agente{ID: appID, RootCID: req.CID, BalanceUTXO: 1000, UltimaActual: time.Now().Unix()}
	if n.nombres == nil {
		n.nombres = map[string]string{}
	}
	n.nombres[name+".app.ans"] = appID
	n.blockstore[req.CID] = data
	n.mu.Unlock()
	n.PersistirLocamente()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "installed",
		"name":     name,
		"agent_id": appID,
		"cid":      req.CID,
		"url":      "/w/" + name + ".app.ans",
	})
}

func (n *NodoAlset) apiEjecutarAppV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Name string `json:"name"`
		CID  string `json:"cid"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	cid := req.CID
	if cid == "" && req.Name != "" {
		n.mu.RLock()
		agentID, ok := n.nombres[req.Name+".app.ans"]
		if !ok {
			agentID, ok = n.nombres[req.Name]
		}
		if ok {
			if a, exists := n.agentes[agentID]; exists {
				cid = a.RootCID
			}
		}
		n.mu.RUnlock()
	}
	if cid == "" {
		http.Error(w, "name o cid requerido", 400)
		return
	}
	data, err := n.BuscarContenidoPorCID(cid)
	if err != nil {
		http.Error(w, "contenido no encontrado", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
