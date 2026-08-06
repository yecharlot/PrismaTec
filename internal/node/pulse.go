package node

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"redalset/internal/pulse"
)

func (n *NodoAlset) startPulseClients() {
	// Si estamos en Render, no nos conectamos a nosotros mismos ni a otros servidores (por ahora)
	if os.Getenv("RENDER") != "" {
		// En Render solo actuamos como servidor de pulsos, no como cliente
		return
	}

	// Si no estamos en Render (es decir, estamos en un nodo local), conectamos a los servidores de pulsos conocidos
	knownServers := pulse.DefaultServers
	for _, url := range knownServers {
		go n.runPulseClient(url)
	}
}
func (n *NodoAlset) runPulseClient(url string) {
	n.pulseClientsMu.Lock()
	if _, exists := n.pulseClients[url]; exists {
		n.pulseClientsMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &PulseClient{
		url:       url,
		ctx:       ctx,
		cancel:    cancel,
		reconnect: make(chan bool, 1),
	}
	n.pulseClients[url] = client
	n.pulseClientsMu.Unlock()

	defer func() {
		n.pulseClientsMu.Lock()
		delete(n.pulseClients, url)
		n.pulseClientsMu.Unlock()
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := n.connectAndListen(client)
			if err != nil {
				log.Printf("Pulse client %s: %v, reconectando en 5s", url, err)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (n *NodoAlset) connectAndListen(client *PulseClient) error {
	client.connected = true
	defer func() { client.connected = false }()
	return pulse.ListenSSE(client.ctx, client.url, n.processPulseEvent)
}

// processPulseEvent maneja los eventos entrantes desde el servidor de pulsos (SSE).
// Es el corazón de la sincronización por HTTP resiliente.
func (n *NodoAlset) processPulseEvent(eventType string, data string) {
	// Log para depuración (puedes comentar en producción)
	// log.Printf("📨 Evento recibido: %s -> %s", eventType, data)

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		log.Printf("⚠️ Error parseando evento %s: %v", eventType, err)
		return
	}

	switch eventType {

	// ============================================================
	// EVENTOS DE AGENTES, DNS Y ROOT
	// ============================================================
	case "agent_created":
		id, _ := payload["id"].(string)
		if id == "" {
			return
		}
		n.mu.Lock()
		if _, exists := n.agentes[id]; !exists {
			n.agentes[id] = &Agente{
				ID:           id,
				RootCID:      "",
				UltimaActual: time.Now().Unix(),
				BalanceUTXO:  0,
			}
			n.mu.Unlock()
			log.Printf("📥 Agente %s recibido por pulso", id)
			n.PersistirLocamente()
		} else {
			n.mu.Unlock()
		}

	case "root_updated":
		id, _ := payload["id"].(string)
		root, _ := payload["root"].(string)
		if id == "" {
			return
		}
		n.mu.Lock()
		if a, exists := n.agentes[id]; exists {
			a.RootCID = root
			a.UltimaActual = time.Now().Unix()
			n.mu.Unlock()
			log.Printf("📥 Root actualizado para %s -> %s", id, root)
			n.PersistirLocamente()
		} else {
			n.mu.Unlock()
		}

	case "dns_registered":
		alias, _ := payload["alias"].(string)
		agent, _ := payload["agent"].(string)
		if alias == "" || agent == "" {
			return
		}
		n.mu.Lock()
		n.nombres[alias] = agent
		n.mu.Unlock()
		log.Printf("📥 DNS %s -> %s recibido por pulso", alias, agent)
		n.PersistirLocamente()

	case "agent_deleted":
		id, _ := payload["id"].(string)
		if id == "" {
			return
		}
		n.mu.Lock()
		delete(n.agentes, id)
		n.mu.Unlock()
		log.Printf("🗑️ Agente %s eliminado por pulso", id)
		n.PersistirLocamente()

	case "agent_updated":
		id, _ := payload["id"].(string)
		balance, _ := payload["balance"].(float64)
		root, _ := payload["root"].(string)
		if id == "" {
			return
		}
		n.mu.Lock()
		if a, exists := n.agentes[id]; exists {
			if balance != 0 {
				a.BalanceUTXO = balance
			}
			if root != "" {
				a.RootCID = root
			}
			a.UltimaActual = time.Now().Unix()
			n.mu.Unlock()
			log.Printf("📥 Agente %s actualizado por pulso", id)
			n.PersistirLocamente()
		} else {
			n.mu.Unlock()
		}

	// ============================================================
	// EVENTOS DE BLOQUES IPFS
	// ============================================================
	case "new_block":
		cid, _ := payload["cid"].(string)
		if cid == "" {
			return
		}
		// Verificar si ya tenemos el bloque
		n.mu.RLock()
		_, exists := n.blockstore[cid]
		n.mu.RUnlock()
		if exists {
			return
		}

		// Intentar obtener los datos del bloque (si vienen en base64)
		dataB64, _ := payload["data"].(string)
		if dataB64 != "" {
			blockData, err := base64.StdEncoding.DecodeString(dataB64)
			if err == nil {
				n.mu.Lock()
				n.blockstore[cid] = blockData
				n.mu.Unlock()
				// Guardar en disco
				os.WriteFile(filepath.Join(BlocksDir, cid), blockData, 0644)
				log.Printf("📦 Bloque %s recibido por pulso (%d bytes)", cid, len(blockData))
				return
			}
		}

		// Si no tenemos los datos, solicitarlos al emisor (o confiar en que llegará después)
		// Por simplicidad, podemos pedir el bloque directamente al servidor de pulsos
		// o usar el método existente BuscarContenidoPorCID (que usa P2P).
		// En una red puramente de pulsos, deberíamos tener un mecanismo para pedir el bloque.
		// Por ahora, lo dejamos así y confiamos en que el bloque llegue con los datos.
		// Si no, se puede implementar un evento "request_block" para solicitarlo.
	case "request_block":
		cid, _ := payload["cid"].(string)
		if cid == "" {
			return
		}
		// Buscar el bloque en el blockstore del servidor
		n.mu.RLock()
		blockData, exists := n.blockstore[cid]
		n.mu.RUnlock()
		if exists {
			b64 := base64.StdEncoding.EncodeToString(blockData)
			n.broadcastPulse("new_block", map[string]interface{}{
				"cid":  cid,
				"data": b64,
			})
			log.Printf("📤 Bloque %s enviado en respuesta a request_block", cid)
		} else {
			log.Printf("⚠️ Bloque %s solicitado pero no encontrado en el servidor", cid)
		}

	case "state_announce":
		cid, _ := payload["cid"].(string)
		if cid == "" {
			return
		}
		data, err := n.BuscarContenidoPorCID(cid)
		if err != nil {
			return
		}
		var remoteState struct {
			Agentes map[string]*Agente `json:"agentes"`
			Nombres map[string]string  `json:"nombres"`
		}
		json.Unmarshal(data, &remoteState)
		n.mu.Lock()
		for k, v := range remoteState.Agentes {
			if local, ok := n.agentes[k]; !ok || v.UltimaActual > local.UltimaActual {
				n.agentes[k] = v
			}
		}
		for k, v := range remoteState.Nombres {
			if _, ok := n.nombres[k]; !ok {
				n.nombres[k] = v
			}
		}
		n.mu.Unlock()
		n.PersistirLocamente()

	// ============================================================
	// EVENTOS NEURONALES (SPIKES, SINAPSIS, ESTADO)
	// ============================================================
	case "neural_spike":
		// Convertir payload a map[string]string y procesar
		go n.procesarSpikeNeuronal(convertMapToStringMap(payload), peer.ID("pulse"))

	case "synaptic_update":
		go n.actualizarPesosSinapsis(convertMapToStringMap(payload), peer.ID("pulse"))

	case "neural_state_sync":
		go n.sincronizarEstadoNeuronal(convertMapToStringMap(payload), peer.ID("pulse"))

	// ============================================================
	// EVENTOS DE INFERENCIA DISTRIBUIDA
	// ============================================================
	case "inference_request":
		reqData, _ := payload["data"].(string)
		if reqData == "" {
			return
		}
		// Convertir a map para usar con manejarInferenciaDistribuida
		var reqMap map[string]string
		if err := json.Unmarshal([]byte(reqData), &reqMap); err != nil {
			log.Printf("⚠️ Error parseando inference_request: %v", err)
			return
		}
		go n.manejarInferenciaDistribuida(reqMap, peer.ID("pulse"))

	case "inference_response":
		respData, _ := payload["data"].(string)
		if respData == "" {
			return
		}
		go n.procesarRespuestaInferencia(map[string]string{"data": respData})

	// ============================================================
	// EVENTOS DE MEMORIA DISTRIBUIDA
	// ============================================================
	case "memory_query":
		go n.manejarConsultaMemoria(convertMapToStringMap(payload), peer.ID("pulse"))

	case "memory_response":
		respData, _ := payload["data"].(string)
		if respData == "" {
			return
		}
		go n.procesarRespuestaMemoria(map[string]string{"data": respData})

	case "memory_distributed":
		go n.manejarMemoriaDistribuida(convertMapToStringMap(payload), peer.ID("pulse"))

	// ============================================================
	// EVENTOS DE ADMINISTRACIÓN
	// ============================================================
	case "admin_panel_announce":
		go n.handleAdminPanelAnnounce(convertMapToStringMap(payload))

	// ============================================================
	// EVENTO DE PRUEBA / HEARTBEAT
	// ============================================================
	case "ping":
		// Ignorar (ya se maneja en el servidor SSE)
		// Puedes usarlo para mantener la conexión viva

	default:
		log.Printf("⚠️ Evento desconocido: %s", eventType)
	}
}

func convertMapToStringMap(m map[string]interface{}) map[string]string {
	res := make(map[string]string)
	for k, v := range m {
		res[k] = fmt.Sprintf("%v", v)
	}
	return res
}

// =============================================================================
// SERVIDOR HTTP – INCLUYE /api/pulse
// =============================================================================
