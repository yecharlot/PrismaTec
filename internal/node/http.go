package node

import (
	"redalset/internal/httpapi"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// buildHTTPHandler registers all HTTP routes and returns the handler.
// Used by startHTTPServer and by integration tests.
func (n *NodoAlset) buildHTTPHandler() http.Handler {
	n.ensureStaticFiles()
	mux := http.NewServeMux()
	h := n.httpHandlers()
	// Core API also registered via Backend adapter (crear-agente, listados, …)
	httpapi.MountCore(mux, &httpAPIBackend{n: n})
	httpapi.Mount(mux, h)
	return mux
}

func (n *NodoAlset) httpHandlers() httpapi.Handlers {
	h := httpapi.Handlers{
		StaticDir: StaticDir,
		Extra:     make(map[string]http.HandlerFunc),
	}

	h.Extra["/"] = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/static/index.html", http.StatusFound)
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	}

	h.Extra["/w/"] = func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.Error(w, "Not found", 404)
			return
		}
		alias := strings.TrimSuffix(parts[2], ".app.ans")
		appPath := filepath.Join(StaticDir, "apps", alias, "index.html")
		if _, err := os.Stat(appPath); err == nil {
			http.ServeFile(w, r, appPath)
			return
		}
		n.mu.RLock()
		targetID, ok := n.nombres[alias+".app.ans"]
		if !ok {
			targetID = alias
		}
		agente, ok := n.agentes[targetID]
		n.mu.RUnlock()
		if !ok || agente.RootCID == "" {
			http.Error(w, "App no encontrada: "+alias, 404)
			return
		}
		data, err := n.BuscarContenidoPorCID(agente.RootCID)
		if err != nil {
			http.Error(w, "Error cargando contenido", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	}

	h.Extra["/apps/"] = func(w http.ResponseWriter, r *http.Request) {
		filePath := strings.TrimPrefix(r.URL.Path, "/apps/")
		fullPath := filepath.Join(StaticDir, "apps", filePath)
		if _, err := os.Stat(fullPath); err == nil {
			ext := filepath.Ext(fullPath)
			switch ext {
			case ".js":
				w.Header().Set("Content-Type", "application/javascript")
			case ".css":
				w.Header().Set("Content-Type", "text/css")
			case ".html":
				w.Header().Set("Content-Type", "text/html")
			case ".json":
				w.Header().Set("Content-Type", "application/json")
			default:
				w.Header().Set("Content-Type", "application/octet-stream")
			}
			http.ServeFile(w, r, fullPath)
			return
		}
		http.Error(w, "Archivo no encontrado", 404)
	}

	h.Extra["/api/pulse"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", 500)
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		sub := &SSESubscriber{
			ch:     make(chan string, 10),
			ctx:    ctx,
			cancel: cancel,
		}

		n.pulseSubscribersMu.Lock()
		n.pulseSubscribers[sub] = true
		n.pulseSubscribersMu.Unlock()

		defer func() {
			n.pulseSubscribersMu.Lock()
			delete(n.pulseSubscribers, sub)
			n.pulseSubscribersMu.Unlock()
			close(sub.ch)
			cancel()
		}()

		state := map[string]interface{}{
			"node_id": n.host.ID().String(),
			"agents":  len(n.agentes),
			"blocks":  len(n.blockstore),
			"time":    time.Now().Unix(),
		}
		stateJSON, _ := json.Marshal(state)
		fmt.Fprintf(w, "event: connected\ndata: %s\n\n", stateJSON)
		flusher.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg := <-sub.ch:
				fmt.Fprint(w, msg)
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
				flusher.Flush()
			case <-ctx.Done():
				return
			}
		}
	}

	h.Extra["/api/pulse/emit"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}

		var req struct {
			EventType string          `json:"eventType"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", 400)
			return
		}

		// Convertir data a map para broadcast
		var payload map[string]interface{}
		if err := json.Unmarshal(req.Data, &payload); err != nil {
			http.Error(w, "Invalid data", 400)
			return
		}

		// Retransmitir a todos los suscriptores
		go n.broadcastPulse(req.EventType, payload)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "emitted"})
	}

	h.Extra["/api/sync/github/save"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if true { // GitHub persistence removed
			http.Error(w, "GitHub persistence not configured", 400)
			return
		}

		n.mu.RLock()
		agentsCount := len(n.agentes)
		blocksCount := len(n.blockstore)
		n.mu.RUnlock()

		if err := n.PersistirEnGitHub(); err != nil {
			http.Error(w, "Error saving to GitHub: "+err.Error(), 500)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "saved",
			"agents":  agentsCount,
			"blocks":  blocksCount,
			"message": "Estado guardado en GitHub correctamente",
		})
	}

	h.Extra["/api/sync/github/load"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if true { // GitHub persistence removed
			http.Error(w, "GitHub persistence not configured", 400)
			return
		}

		if err := n.CargarDesdeGitHub(); err != nil {
			http.Error(w, "Error loading from GitHub: "+err.Error(), 500)
			return
		}

		n.mu.RLock()
		agentsCount := len(n.agentes)
		blocksCount := len(n.blockstore)
		n.mu.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "loaded",
			"agents":  agentsCount,
			"blocks":  blocksCount,
			"message": "Estado cargado desde GitHub correctamente",
		})
	}

	h.Extra["/api/sync/github"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if true { // GitHub persistence removed
			http.Error(w, "GitHub persistence not configured", 400)
			return
		}

		// Primero cargar
		if err := n.CargarDesdeGitHub(); err != nil {
			http.Error(w, "Error loading from GitHub: "+err.Error(), 500)
			return
		}

		// Luego guardar
		if err := n.PersistirEnGitHub(); err != nil {
			http.Error(w, "Error saving to GitHub: "+err.Error(), 500)
			return
		}

		n.mu.RLock()
		agentsCount := len(n.agentes)
		blocksCount := len(n.blockstore)
		n.mu.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "synced",
			"agents":  agentsCount,
			"blocks":  blocksCount,
			"message": "Sincronización con GitHub completada",
		})
	}

	h.Extra["/api/sync/github/status"] = func(w http.ResponseWriter, r *http.Request) {
		if true { // GitHub persistence removed
			json.NewEncoder(w).Encode(map[string]interface{}{
				"configured": false,
				"message":    "GitHub persistence not configured",
			})
			return
		}

		n.mu.RLock()
		agentsCount := len(n.agentes)
		blocksCount := len(n.blockstore)
		n.mu.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"configured": false,
			"backend":    "local_or_supabase",
			"message":    "GitHub persistence has been removed. Use SUPABASE_URL / SUPABASE_SERVICE_KEY or local disk.",
			"agents":     agentsCount,
			"blocks":     blocksCount,
		})
	}

	h.Extra["/api/dns/resolve"] = n.handleDNSResolve

	h.Extra["/api/dns/delete"] = n.handleDNSDelete

	h.Extra["/api/ipfs/fetch"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			CID string `json:"cid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		data, err := n.BuscarContenidoPorCID(req.CID)
		if err != nil {
			http.Error(w, "No encontrado", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cid":  req.CID,
			"data": string(data),
			"size": len(data),
		})
	}

	h.Extra["/api/audit/log"] = n.handleAuditLog

	h.Extra["/api/eliminar-agente"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido: "+err.Error(), 400)
			return
		}
		if req.ID == "" {
			http.Error(w, "ID de agente requerido", 400)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		if _, exists := n.agentes[req.ID]; !exists {
			http.Error(w, "Agente no encontrado", 404)
			return
		}
		delete(n.agentes, req.ID)
		n.Auditoria("AGENTE_ELIMINADO", fmt.Sprintf("ID: %s", req.ID))
		dAg, _ := json.MarshalIndent(n.agentes, "", "  ")
		if err := os.WriteFile("alset_state.json", dAg, 0644); err != nil {
			fmt.Printf("⚠️ Error guardando estado: %v\n", err)
		}
		go n.SincronizarConPares()
		go n.broadcastPulse("agent_deleted", map[string]interface{}{
			"id":   req.ID,
			"time": time.Now().Unix(),
		})
		fmt.Printf("🗑️ Agente eliminado: %s\n", req.ID)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "deleted",
			"id":      req.ID,
			"message": "Agente eliminado correctamente",
		})
	}

	h.Extra["/api/modificar-agente"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "PUT" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			ID          string  `json:"id"`
			BalanceUTXO float64 `json:"balance_utxo"`
			RootCID     string  `json:"root_cid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if req.ID == "" {
			http.Error(w, "ID de agente requerido", 400)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		if agent, exists := n.agentes[req.ID]; exists {
			if req.BalanceUTXO != 0 {
				agent.BalanceUTXO = req.BalanceUTXO
			}
			if req.RootCID != "" {
				agent.RootCID = req.RootCID
			}
			agent.UltimaActual = time.Now().Unix()
			n.Auditoria("AGENTE_MODIFICADO", fmt.Sprintf("ID: %s | Balance: %.2f | RootCID: %s", req.ID, req.BalanceUTXO, req.RootCID))
			n.PersistirLocamente()
			go n.broadcastPulse("agent_updated", map[string]interface{}{
				"id":      req.ID,
				"balance": req.BalanceUTXO,
				"root":    req.RootCID,
				"time":    time.Now().Unix(),
			})
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "updated",
				"agent":  agent,
			})
		} else {
			http.Error(w, "Agente no encontrado", 404)
		}
	}

	h.Extra["/api/debug/estado"] = n.handleDebugEstado

	h.Extra["/api/prism/verificar"] = n.handlePrismVerificar

	h.Extra["/api/prism/revocar"] = n.handlePrismRevocar

	h.Extra["/api/prism/sellar"] = n.handlePrismSellar

	h.Extra["/api/admin/update-pass"] = n.handleAdminUpdatePass

	h.Extra["/api/admin/login"] = n.handleAdminLogin

	h.Extra["/api/ipfs/add"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cidStr, _ := n.GenerarCID(body)
		n.AnunciarNuevoBloque(cidStr)
		json.NewEncoder(w).Encode(map[string]string{"cid": cidStr})
	}

	h.Extra["/api/ipfs/delete"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			CID string `json:"cid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if req.CID == "" {
			http.Error(w, "CID requerido", 400)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		if _, exists := n.blockstore[req.CID]; exists {
			delete(n.blockstore, req.CID)
			diskPath := filepath.Join(BlocksDir, req.CID)
			if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
				fmt.Printf("⚠️ No se pudo eliminar archivo de disco: %v\n", err)
			}
			n.Auditoria("IPFS_BLOQUE_ELIMINADO", fmt.Sprintf("CID: %s", req.CID))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "deleted",
				"cid":    req.CID,
			})
		} else {
			http.Error(w, "Bloque no encontrado", 404)
		}
	}

	h.Extra["/api/ipfs/get"] = func(w http.ResponseWriter, r *http.Request) {
		data, err := n.BuscarContenidoPorCID(r.URL.Query().Get("cid"))
		if err != nil {
			http.Error(w, "Not found", 404)
			return
		}
		w.Write(data)
	}

	h.Extra["/api/ipfs/clear"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			Confirm string `json:"confirm"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Confirm != "YES" {
			http.Error(w, "Confirmación requerida: 'confirm': 'YES'", 400)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		count := len(n.blockstore)
		n.blockstore = make(map[string][]byte)
		if err := os.RemoveAll(BlocksDir); err != nil {
			fmt.Printf("⚠️ Error limpiando directorio: %v\n", err)
		}
		os.MkdirAll(BlocksDir, 0755)
		n.Auditoria("IPFS_BLOCKSTORE_LIMPIADA", fmt.Sprintf("Bloques eliminados: %d", count))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "cleared",
			"blocks_deleted": count,
		})
	}

	h.Extra["/api/apps/register"] = n.handleAppsRegister

	h.Extra["/api/apps/list"] = n.handleAppsList

	h.Extra["/api/lispai"] = func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Cmd string `json:"cmd"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		res, err := n.lisp.Eval(req.Cmd)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"resultado": res})
	}

	h.Extra["/api/poh/event"] = n.handlePoHEvent

	h.Extra["/api/poh/proof"] = n.handlePoHProof

	h.Extra["/api/sync/status"] = n.handleSyncStatus

	h.Extra["/api/sync/full"] = n.handleSyncFull

	h.Extra["/api/sync/quick"] = n.handleSyncQuick

	h.Extra["/api/sync/config"] = n.handleSyncConfig

	h.Extra["/api/ia/configurar"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			NeuronType     string  `json:"neuron_type"`
			SpikeThreshold float64 `json:"spike_threshold"`
			LeakRate       float64 `json:"leak_rate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		done := make(chan bool, 1)
		go func() {
			n.mu.Lock()
			defer n.mu.Unlock()
			if n.neuralState == nil {
				n.neuralState = &NeuralState{
					MembranePotential: 0,
					SpikeThreshold:    0.6,
					LeakRate:          0.01,
					NeuronType:        "hidden",
					Synapses:          make(map[string]SynapticWeight),
				}
			}
			if req.NeuronType != "" {
				n.neuralState.NeuronType = req.NeuronType
			}
			if req.SpikeThreshold > 0 && req.SpikeThreshold <= 1 {
				n.neuralState.SpikeThreshold = req.SpikeThreshold
			}
			if req.LeakRate > 0 && req.LeakRate <= 1 {
				n.neuralState.LeakRate = req.LeakRate
			}
			done <- true
		}()
		select {
		case <-done:
			go n.persistirEstadoNeuronal()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "configured",
				"config": n.neuralState,
			})
		case <-time.After(5 * time.Second):
			http.Error(w, "Timeout", 500)
		}
	}

	h.Extra["/api/ia/inferir"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			Entrada []float64 `json:"entrada"`
			Timeout int       `json:"timeout"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if len(req.Entrada) == 0 {
			req.Entrada = []float64{0}
		}
		var output float64 = 0
		for _, val := range req.Entrada {
			output += val
		}
		if len(req.Entrada) > 0 {
			output = output / float64(len(req.Entrada))
		}
		output = 1.0 / (1.0 + math.Exp(-output))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "success",
			"output":       []float64{output},
			"processed_by": n.host.ID().String(),
			"process_time": time.Now().UnixNano(),
		})
		go func() {
			if n.topic != nil {
				requestID := generarUUID()
				inferenceReq := InferenceRequest{
					RequestID:    requestID,
					InputData:    req.Entrada,
					OriginNodeID: n.host.ID().String(),
					TTL:          3,
				}
				data, _ := json.Marshal(inferenceReq)
				update := map[string]string{
					"tipo": "inference_request",
					"data": string(data),
				}
				msgData, _ := json.Marshal(update)
				n.topic.Publish(n.ctx, msgData)
			}
		}()
	}

	h.Extra["/api/ia/aprender"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			Entrada         []float64 `json:"entrada"`
			SalidaEsperada  []float64 `json:"salida_esperada"`
			TasaAprendizaje float64   `json:"tasa_aprendizaje"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		tasa := req.TasaAprendizaje
		if tasa <= 0 {
			tasa = 0.01
		}
		n.mu.Lock()
		if n.neuralState != nil {
			for target, syn := range n.neuralState.Synapses {
				newWeight := syn.Weight + tasa*(1-syn.Weight)
				if newWeight > 1 {
					newWeight = 1
				}
				syn.Weight = newWeight
				syn.SuccessfulFires++
				n.neuralState.Synapses[target] = syn
			}
		}
		n.mu.Unlock()
		go n.persistirEstadoNeuronal()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "learning_completed",
			"tasa":   tasa,
		})
	}

	h.Extra["/api/ia/memoria/buscar"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			Consulta string `json:"consulta"`
			Limit    int    `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if req.Limit <= 0 {
			req.Limit = 10
		}
		resultados := []map[string]interface{}{}
		n.mu.RLock()
		for cid, data := range n.blockstore {
			if strings.Contains(string(data), req.Consulta) {
				resultados = append(resultados, map[string]interface{}{
					"cid":  cid,
					"data": string(data),
				})
				if len(resultados) >= req.Limit {
					break
				}
			}
		}
		n.mu.RUnlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"query":   req.Consulta,
			"results": resultados,
			"count":   len(resultados),
		})
	}

	h.Extra["/api/ia/topologia"] = func(w http.ResponseWriter, r *http.Request) {
		peers := n.host.Network().Peers()
		vecinosInfo := []map[string]interface{}{}
		n.mu.RLock()
		neuronType := "hidden"
		synapsesCount := 0
		if n.neuralState != nil {
			neuronType = n.neuralState.NeuronType
			synapsesCount = len(n.neuralState.Synapses)
		}
		for _, p := range peers {
			weight := 0.0
			fires := int64(0)
			tieneSinapsis := false
			if n.neuralState != nil {
				if s, ok := n.neuralState.Synapses[p.String()]; ok {
					weight = s.Weight
					fires = s.SuccessfulFires
					tieneSinapsis = true
				}
			}
			vecinosInfo = append(vecinosInfo, map[string]interface{}{
				"peer_id":   p.String(),
				"connected": n.host.Network().Connectedness(p).String(),
				"synaptic_weight": map[string]interface{}{
					"exists": tieneSinapsis,
					"weight": weight,
					"fires":  fires,
				},
			})
		}
		n.mu.RUnlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"neuron_id":   n.host.ID().String(),
			"neuron_type": neuronType,
			"peers_count": len(peers),
			"synapses":    synapsesCount,
			"peers":       vecinosInfo,
		})
	}

	h.Extra["/api/ia/estado"] = func(w http.ResponseWriter, r *http.Request) {
		n.mu.RLock()
		defer n.mu.RUnlock()
		if n.neuralState == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "not_initialized",
			})
			return
		}
		sinapsisList := []map[string]interface{}{}
		for target, s := range n.neuralState.Synapses {
			sinapsisList = append(sinapsisList, map[string]interface{}{
				"target":           target,
				"weight":           s.Weight,
				"successful_fires": s.SuccessfulFires,
				"last_updated":     s.LastUpdated,
			})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":             "active",
			"neuron_type":        n.neuralState.NeuronType,
			"membrane_potential": n.neuralState.MembranePotential,
			"spike_threshold":    n.neuralState.SpikeThreshold,
			"leak_rate":          n.neuralState.LeakRate,
			"last_spike_time":    n.neuralState.LastSpikeTime,
			"synapses_count":     len(n.neuralState.Synapses),
			"synapses":           sinapsisList,
		})
	}

	h.Extra["/api/ia/metricas"] = func(w http.ResponseWriter, r *http.Request) {
		n.mu.RLock()
		defer n.mu.RUnlock()
		totalSpikes := int64(0)
		pesoPromedio := 0.0
		synapseCount := 0
		if n.neuralState != nil {
			synapseCount = len(n.neuralState.Synapses)
			for _, s := range n.neuralState.Synapses {
				totalSpikes += s.SuccessfulFires
				pesoPromedio += s.Weight
			}
			if synapseCount > 0 {
				pesoPromedio /= float64(synapseCount)
			}
		}
		membrane := 0.0
		if n.neuralState != nil {
			membrane = n.neuralState.MembranePotential
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_synaptic_connections": synapseCount,
			"average_synaptic_weight":    pesoPromedio,
			"total_successful_spikes":    totalSpikes,
			"current_membrane_potential": membrane,
			"uptime":                     time.Now().Unix() - n.startTime,
		})
	}

	h.Extra["/api/ia/sinapsis/conectar"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			NodoDestino string  `json:"nodo_destino"`
			PesoInicial float64 `json:"peso_inicial"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if req.NodoDestino == "" {
			http.Error(w, "nodo_destino requerido", 400)
			return
		}
		if req.PesoInicial <= 0 {
			req.PesoInicial = 0.5
		}
		if req.PesoInicial > 1 {
			req.PesoInicial = 1
		}
		done := make(chan bool, 1)
		go func() {
			n.mu.Lock()
			defer n.mu.Unlock()
			if n.neuralState == nil {
				n.neuralState = &NeuralState{
					MembranePotential: 0,
					SpikeThreshold:    0.6,
					LeakRate:          0.01,
					NeuronType:        "hidden",
					Synapses:          make(map[string]SynapticWeight),
				}
			}
			n.neuralState.Synapses[req.NodoDestino] = SynapticWeight{
				TargetNeuronID: req.NodoDestino,
				Weight:         req.PesoInicial,
				LastUpdated:    time.Now().Unix(),
			}
			done <- true
		}()
		select {
		case <-done:
			go n.persistirEstadoNeuronal()
			go func() {
				update := map[string]string{
					"tipo":          "synaptic_update",
					"neuronas_pre":  n.host.ID().String(),
					"neuronas_post": req.NodoDestino,
					"exito":         "true",
					"peso":          fmt.Sprintf("%f", req.PesoInicial),
				}
				if data, err := json.Marshal(update); err == nil && n.topic != nil {
					n.topic.Publish(n.ctx, data)
				}
			}()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "connected",
				"target": req.NodoDestino,
				"weight": req.PesoInicial,
			})
		case <-time.After(5 * time.Second):
			http.Error(w, "Timeout", 500)
		}
	}

	h.Extra["/api/ia/sinapsis/clear"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		if n.neuralState != nil {
			oldCount := len(n.neuralState.Synapses)
			n.neuralState.Synapses = make(map[string]SynapticWeight)
			n.persistirEstadoNeuronal()
			n.Auditoria("SINAPSIS_LIMPIADAS", fmt.Sprintf("Sinapsis eliminadas: %d", oldCount))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":           "cleared",
				"synapses_removed": oldCount,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":           "already_empty",
				"synapses_removed": 0,
			})
		}
	}

	h.Extra["/api/modulos"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarModulos(w, r)
		case "POST":
			n.crearModulo(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	h.Extra["/api/modulos/"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.obtenerModulo(w, r)
		case "PUT":
			n.actualizarModulo(w, r)
		case "DELETE":
			n.eliminarModulo(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	h.Extra["/api/entidades"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarEntidades(w, r)
		case "POST":
			n.crearEntidad(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	h.Extra["/api/entidades/"] = func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/relaciones") {
			n.obtenerRelacionesDeEntidad(w, r)
			return
		}
		switch r.Method {
		case "GET":
			n.obtenerEntidad(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	h.Extra["/api/relaciones"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarRelaciones(w, r)
		case "POST":
			n.crearRelacion(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	h.Extra["/api/auth/token"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.crearTokenEndpoint(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	h.Extra["/api/auth/validate"] = n.validarTokenEndpoint

	h.Extra["/api/auth/revoke"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.revocarTokenEndpoint(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	h.Extra["/api/roles"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.asignarRol(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	h.Extra["/api/roles/"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.obtenerRoles(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	}

	return h
}

func (n *NodoAlset) startHTTPServer(port string) {
	mux := n.buildHTTPHandler()
	fmt.Printf("🚀 Prisma Tec API activa en puerto %s (incluye /api/pulse)\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// En algún lugar de tu nodo (ej. en QuickStartup o en un goroutine)
func (n *NodoAlset) publishStateSnapshot() {
	state := struct {
		Agentes map[string]*Agente `json:"agentes"`
		Nombres map[string]string  `json:"nombres"`
	}{
		Agentes: n.agentes,
		Nombres: n.nombres,
	}
	data, _ := json.Marshal(state)
	cid, _ := n.GenerarCID(data)
	n.broadcastPulse("state_announce", map[string]interface{}{
		"cid":  cid,
		"node": n.host.ID().String(),
	})
}

// =============================================================================
// RUN – punto de entrada del nodo (llamado desde cmd/prisma-tec)
// =============================================================================
