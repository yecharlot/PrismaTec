package node

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (n *NodoAlset) SincronizarConPares() {
	n.mu.RLock()
	data, _ := json.Marshal(n.agentes)
	n.mu.RUnlock()
	cidStr, _ := n.GenerarCID(data)
	update := map[string]string{
		"tipo": "new_block",
		"cid":  cidStr,
	}
	msgBytes, _ := json.Marshal(update)
	if n.topic != nil {
		n.topic.Publish(n.ctx, msgBytes)
	}
}

func (n *NodoAlset) DifundirActualizacionDNS(alias string, agentID string) {
	update := map[string]string{"tipo": "dns_update", "alias": alias, "agent_id": agentID}
	data, _ := json.Marshal(update)
	if n.topic != nil {
		n.topic.Publish(n.ctx, data)
	}
}

func (n *NodoAlset) SolicitarBloqueAPar(cidStr string, p peer.ID) {
	s, err := n.host.NewStream(n.ctx, p, AlsetDataExchangeID)
	if err != nil {
		return
	}
	defer s.Close()
	s.Write([]byte(cidStr + "\n"))
	data, _ := io.ReadAll(s)
	if len(data) > 0 {
		n.GenerarCID(data)
		var remAg map[string]*Agente
		if err := json.Unmarshal(data, &remAg); err == nil && len(remAg) > 0 {
			n.mu.Lock()
			for k, v := range remAg {
				n.agentes[k] = v
			}
			n.mu.Unlock()
			n.PersistirLocamente()
		}
	}
}

func (n *NodoAlset) handleDataExchange(s network.Stream) {
	defer s.Close()
	scanner := bufio.NewScanner(s)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "SYNC_FULL_REQUEST") {
			n.handleFullSyncRequest(s)
			return
		}
		if strings.HasPrefix(line, "SYNC_QUICK_REQUEST") {
			n.handleQuickSyncRequest(s)
			return
		}
		cidReq := line
		n.mu.RLock()
		data, ok := n.blockstore[cidReq]
		n.mu.RUnlock()
		if ok {
			s.Write(data)
		}
	}
}

func (n *NodoAlset) handleFullSyncRequest(s network.Stream) {
	fmt.Println("📡 Recibida solicitud de sincronización completa")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	state := struct {
		Agentes map[string]*Agente `json:"agentes"`
		Nombres map[string]string  `json:"nombres"`
	}{
		Agentes: n.agentes,
		Nombres: n.nombres,
	}
	stateJSON, _ := json.Marshal(state)
	gz.Write(stateJSON)
	gz.Close()
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, uint64(buf.Len()))
	s.Write(sizeBuf)
	s.Write(buf.Bytes())
	fmt.Printf("✅ Estado completo enviado: %d bytes comprimidos\n", buf.Len())
}

func (n *NodoAlset) handleQuickSyncRequest(s network.Stream) {
	fmt.Println("⚡ Recibida solicitud de sincronización rápida")
	response := struct {
		Agentes      map[string]*Agente `json:"agentes"`
		Nombres      map[string]string  `json:"nombres"`
		RecentBlocks map[string][]byte  `json:"recent_blocks"`
		NeuralState  *NeuralState       `json:"neural_state"`
	}{
		Agentes:      n.agentes,
		Nombres:      n.nombres,
		NeuralState:  n.neuralState,
		RecentBlocks: make(map[string][]byte),
	}
	count := 0
	for cid, data := range n.blockstore {
		if count >= 100 {
			break
		}
		response.RecentBlocks[cid] = data
		count++
	}
	data, _ := json.Marshal(response)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(data)
	gz.Close()
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, uint64(buf.Len()))
	s.Write(sizeBuf)
	s.Write(buf.Bytes())
}

func (n *NodoAlset) EscucharGossip() {
	sub, _ := n.topic.Subscribe()
	for {
		msg, err := sub.Next(n.ctx)
		if err != nil {
			return
		}
		if msg.ReceivedFrom == n.host.ID() {
			continue
		}
		var update map[string]string
		if err := json.Unmarshal(msg.Data, &update); err == nil {
			switch update["tipo"] {
			case "dns_update":
				n.mu.Lock()
				n.nombres[update["alias"]] = update["agent_id"]
				n.mu.Unlock()
				n.PersistirLocamente()
			case "new_block":
				n.mu.RLock()
				_, existe := n.blockstore[update["cid"]]
				n.mu.RUnlock()
				if !existe {
					go n.SolicitarBloqueAPar(update["cid"], msg.ReceivedFrom)
				}
			case "admin_panel_announce":
				go n.handleAdminPanelAnnounce(update)
			case "neural_spike":
				go n.procesarSpikeNeuronal(update, msg.ReceivedFrom)
			case "inference_request":
				go n.manejarInferenciaDistribuida(update, msg.ReceivedFrom)
			case "inference_response":
				go n.procesarRespuestaInferencia(update)
			case "synaptic_update":
				go n.actualizarPesosSinapsis(update, msg.ReceivedFrom)
			case "memory_query":
				go n.manejarConsultaMemoria(update, msg.ReceivedFrom)
			case "memory_response":
				go n.procesarRespuestaMemoria(update)
			case "memory_distributed":
				go n.manejarMemoriaDistribuida(update, msg.ReceivedFrom)
			case "neural_state_sync":
				go n.sincronizarEstadoNeuronal(update, msg.ReceivedFrom)
			}
		}
	}
}

// =============================================================================
// ADMIN PANEL DISTRIBUTION (EXISTENTE)
// =============================================================================
