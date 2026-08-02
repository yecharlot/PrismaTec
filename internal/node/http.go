package node

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
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

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

func (n *NodoAlset) startHTTPServer(port string) {
	mux := http.NewServeMux()

	n.ensureStaticFiles()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(StaticDir))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/static/index.html", http.StatusFound)
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	})

	mux.HandleFunc("/w/", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/apps/", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// ---- PULSO: endpoint SSE ----
	mux.HandleFunc("/api/pulse", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Dentro de startHTTPServer, junto a los otros endpoints
	mux.HandleFunc("/api/pulse/emit", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// =============================================================================
	// ENDPOINTS DE GITHUB SYNC
	// =============================================================================

	// Guardar estado en GitHub
	mux.HandleFunc("/api/sync/github/save", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Cargar estado desde GitHub
	mux.HandleFunc("/api/sync/github/load", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Sincronización completa (guardar + cargar)
	mux.HandleFunc("/api/sync/github", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Verificar estado de GitHub
	mux.HandleFunc("/api/sync/github/status", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// ---- RESTO DE ENDPOINTS (copiados del original) ----
	mux.HandleFunc("/api/ipfs/list", func(w http.ResponseWriter, r *http.Request) {
		n.mu.RLock()
		defer n.mu.RUnlock()
		blocks := make([]BlockInfo, 0, len(n.blockstore))
		for cid, data := range n.blockstore {
			preview := string(data)
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			blocks = append(blocks, BlockInfo{
				CID:     cid,
				Size:    len(data),
				Preview: preview,
			})
		}
		json.NewEncoder(w).Encode(blocks)
	})

	mux.HandleFunc("/api/network/peers", n.handleNetworkPeers)
	mux.HandleFunc("/api/dns/list", n.handleDNSList)
	mux.HandleFunc("/api/dns/resolve", n.handleDNSResolve)
	mux.HandleFunc("/api/dns/delete", n.handleDNSDelete)
	mux.HandleFunc("/api/agentes/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/agentes/")
		if strings.HasSuffix(path, "/root") {
			id := strings.TrimSuffix(path, "/root")
			n.mu.RLock()
			agent, exists := n.agentes[id]
			n.mu.RUnlock()
			if !exists {
				http.Error(w, "Agente no encontrado", 404)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"agent_id":   id,
				"root_cid":   agent.RootCID,
				"updated_at": agent.UltimaActual,
			})
			return
		}
		n.mu.RLock()
		defer n.mu.RUnlock()
		json.NewEncoder(w).Encode(n.agentes)
	})

	mux.HandleFunc("/api/ipfs/fetch", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/audit/log", n.handleAuditLog)
	mux.HandleFunc("/api/crear-agente", func(w http.ResponseWriter, r *http.Request) {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			http.Error(w, "Error generando llave", 500)
			return
		}
		id := hex.EncodeToString(pub[:8])
		balanceInicial := 0.0
		nuevoAgente := &Agente{
			ID:           id,
			BalanceUTXO:  balanceInicial,
			UltimaActual: time.Now().Unix(),
		}
		n.mu.Lock()
		n.agentes[id] = nuevoAgente
		n.mu.Unlock()
		n.Auditoria("AGENTE_REGISTRADO_HTTP", fmt.Sprintf("ID: %s | InitBalance: %f", id, balanceInicial))
		n.PersistirLocamente()
		go n.SincronizarConPares()
		go n.broadcastPulse("agent_created", map[string]interface{}{
			"id":   id,
			"root": "",
			"time": time.Now().Unix(),
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nuevoAgente)
	})

	mux.HandleFunc("/api/eliminar-agente", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/modificar-agente", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/debug/estado", n.handleDebugEstado)
	mux.HandleFunc("/api/prism/verificar", n.handlePrismVerificar)
	mux.HandleFunc("/api/prism/revocar", n.handlePrismRevocar)
	mux.HandleFunc("/api/prism/sellar", n.handlePrismSellar)
	mux.HandleFunc("/api/admin/update-pass", n.handleAdminUpdatePass)
	mux.HandleFunc("/api/admin/login", n.handleAdminLogin)
	mux.HandleFunc("/api/ipfs/add", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cidStr, _ := n.GenerarCID(body)
		n.AnunciarNuevoBloque(cidStr)
		json.NewEncoder(w).Encode(map[string]string{"cid": cidStr})
	})

	mux.HandleFunc("/api/ipfs/delete", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ipfs/get", func(w http.ResponseWriter, r *http.Request) {
		data, err := n.BuscarContenidoPorCID(r.URL.Query().Get("cid"))
		if err != nil {
			http.Error(w, "Not found", 404)
			return
		}
		w.Write(data)
	})

	mux.HandleFunc("/api/ipfs/clear", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/apps/register", n.handleAppsRegister)
	mux.HandleFunc("/api/apps/list", n.handleAppsList)
	mux.HandleFunc("/api/lispai", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/poh/event", n.handlePoHEvent)
	mux.HandleFunc("/api/poh/proof", n.handlePoHProof)
	mux.HandleFunc("/api/sync/status", n.handleSyncStatus)
	mux.HandleFunc("/api/sync/full", n.handleSyncFull)
	mux.HandleFunc("/api/sync/quick", n.handleSyncQuick)
	mux.HandleFunc("/api/sync/config", n.handleSyncConfig)

	// IA endpoints
	mux.HandleFunc("/api/ia/configurar", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ia/inferir", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ia/aprender", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ia/memoria/buscar", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ia/topologia", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ia/estado", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ia/metricas", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ia/sinapsis/conectar", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/ia/sinapsis/clear", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Módulos, entidades y seguridad endpoints
	mux.HandleFunc("/api/modulos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarModulos(w, r)
		case "POST":
			n.crearModulo(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})
	mux.HandleFunc("/api/modulos/", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/entidades", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarEntidades(w, r)
		case "POST":
			n.crearEntidad(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})
	mux.HandleFunc("/api/entidades/", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/api/relaciones", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarRelaciones(w, r)
		case "POST":
			n.crearRelacion(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})

	mux.HandleFunc("/api/auth/token", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.crearTokenEndpoint(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})
	mux.HandleFunc("/api/auth/validate", n.validarTokenEndpoint)
	mux.HandleFunc("/api/auth/revoke", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.revocarTokenEndpoint(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})

	mux.HandleFunc("/api/roles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.asignarRol(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})
	mux.HandleFunc("/api/roles/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.obtenerRoles(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})

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
// INICIALIZACIÓN DEL NODO (MODIFICADA)
// =============================================================================

func (n *NodoAlset) Init() {
	n.LoadMasterKey()
	n.startTime = time.Now().Unix()
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		log.Fatal("Error generando clave privada:", err)
	}
	// Habilitar relay y usar puerto fijo opcional
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableRelayService(),
	)
	if err != nil {
		log.Fatal("Error creando el host libp2p:", err)
	}
	n.host = h
	n.ctx = context.Background()
	n.blockstore = make(map[string][]byte)
	n.agentes = make(map[string]*Agente)
	n.nombres = make(map[string]string)
	n.pendingInferences = make(map[string]chan InferenceResponse)
	n.pendingMemoryQueries = make(map[string]chan MemoryResponse)
	n.hebbianMemory = make(map[string]float64)

	// Inicializar pulse
	n.pulseSubscribers = make(map[*SSESubscriber]bool)
	n.pulseClients = make(map[string]*PulseClient)

	n.syncManager = n.InitSyncManager()

	n.CargarEstado()
	n.neuralState = &NeuralState{
		MembranePotential: 0,
		LastSpikeTime:     0,
		SpikeThreshold:    0.6,
		LeakRate:          0.01,
		RefractoryPeriod:  1000000,
		Synapses:          make(map[string]SynapticWeight),
		NeuronType:        "input",
	}
	n.cargarPesosSinapsis()
	n.datastore = ds_sync.MutexWrap(datastore.NewMapDatastore())
	ps, err := pubsub.NewGossipSub(n.ctx, n.host)
	if err != nil {
		log.Fatal("Error creando GossipSub:", err)
	}
	n.pubsub = ps
	n.topic, err = n.pubsub.Join(AlsetGossipTopic)
	if err != nil {
		log.Fatal("Error uniéndose al tópico:", err)
	}
	n.host.SetStreamHandler(AlsetDataExchangeID, n.handleDataExchange)
	n.kademlia, err = dht.New(n.ctx, n.host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal("Error creando DHT:", err)
	}
	go n.kademlia.Bootstrap(n.ctx)
	n.lisp = NewLispEvaluator(n)
	mdns.NewMdnsService(n.host, "alset-mesh", &discoveryNotifee{h: n.host}).Start()
	go n.EscucharGossip()

	go n.QuickStartup()
}

type discoveryNotifee struct{ h host.Host }

func (d *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	d.h.Connect(context.Background(), pi)
}

// =============================================================================
// RUN – punto de entrada del nodo (llamado desde cmd/prisma-tec)
// =============================================================================
