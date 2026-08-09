package node

import (
	"redalset/internal/httpapi"
	"context"
	"encoding/json"
	"fmt"
	"log"
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



	h.Extra["/api/audit/log"] = n.handleAuditLog

	h.Extra["/api/debug/estado"] = n.handleDebugEstado

	h.Extra["/api/prism/verificar"] = n.handlePrismVerificar

	h.Extra["/api/prism/revocar"] = n.handlePrismRevocar

	h.Extra["/api/prism/sellar"] = n.handlePrismSellar

	h.Extra["/api/admin/update-pass"] = n.handleAdminUpdatePass

	h.Extra["/api/admin/login"] = n.handleAdminLogin

	h.Extra["/api/apps/register"] = n.handleAppsRegister

	h.Extra["/api/apps/list"] = n.handleAppsList

	h.Extra["/api/poh/event"] = n.handlePoHEvent

	h.Extra["/api/poh/proof"] = n.handlePoHProof

	h.Extra["/api/sync/status"] = n.handleSyncStatus

	h.Extra["/api/sync/full"] = n.handleSyncFull

	h.Extra["/api/sync/quick"] = n.handleSyncQuick

	h.Extra["/api/sync/config"] = n.handleSyncConfig

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

	n.registerSalesBridge(h.Extra)
	n.registerAPIv2(h.Extra)

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
