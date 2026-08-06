package node

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (n *NodoAlset) persistirEstadoNeuronal() {
	if n.neuralState == nil {
		return
	}
	n.mu.RLock()
	data, _ := json.MarshalIndent(n.neuralState, "", "  ")
	n.mu.RUnlock()
	_ = os.WriteFile("neural_state.json", data, 0644)
}

func (n *NodoAlset) cargarPesosSinapsis() {
	if data, err := os.ReadFile("neural_state.json"); err == nil {
		var state NeuralState
		if err := json.Unmarshal(data, &state); err == nil {
			n.neuralState = &state
			if n.neuralState.Synapses == nil {
				n.neuralState.Synapses = make(map[string]SynapticWeight)
			}
			fmt.Println("🧠 Estado neuronal cargado desde disco")
		}
	}
	if n.neuralState == nil {
		return
	}
	for target, syn := range n.neuralState.Synapses {
		n.hebbianMemory[target] = syn.Weight
	}
}

func (n *NodoAlset) puedeProcesarInferencia(input []float64) bool {
	return n.neuralState != nil && n.neuralState.NeuronType == "input"
}

func (n *NodoAlset) seleccionarMejorVecinoParaInferencia(input []float64) string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.neuralState == nil {
		return ""
	}
	var mejorNodo string
	var mayorPeso float64
	for targetID, sinapsis := range n.neuralState.Synapses {
		if sinapsis.Weight > mayorPeso {
			mayorPeso = sinapsis.Weight
			mejorNodo = targetID
		}
	}
	return mejorNodo
}

func (n *NodoAlset) reenviarSolicitudInferencia(req InferenceRequest, destino string) {
	data, _ := json.Marshal(req)
	msg := map[string]string{
		"tipo": "inference_request",
		"data": string(data),
	}
	msgData, _ := json.Marshal(msg)
	n.topic.Publish(n.ctx, msgData)
}

func (n *NodoAlset) buscarEnMemoriaLocal(consulta string) []string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	resultados := []string{}
	for cid, data := range n.blockstore {
		if strings.Contains(string(data), consulta) || strings.Contains(cid, consulta) {
			if len(resultados) < 10 {
				resultados = append(resultados, cid)
			}
		}
	}
	return resultados
}

func (n *NodoAlset) buscarEnMemoriaLocalConContenido(consulta string) []MemoryResponse {
	n.mu.RLock()
	defer n.mu.RUnlock()
	resultados := []MemoryResponse{}
	for cid, data := range n.blockstore {
		if strings.Contains(string(data), consulta) {
			resultados = append(resultados, MemoryResponse{
				Results:       []string{cid},
				Contents:      []string{string(data)},
				ResponderNode: n.host.ID().String(),
			})
			if len(resultados) >= 5 {
				break
			}
		}
	}
	return resultados
}

func (n *NodoAlset) propagarMemoriaDistribuida(data string, cid string) {
	query := MemoryQuery{
		QueryID:    generarUUID(),
		Content:    data,
		OriginNode: n.host.ID().String(),
		TTL:        3,
	}
	msg := map[string]string{
		"tipo":     "memory_distributed",
		"query_id": query.QueryID,
		"content":  data,
		"cid":      cid,
		"origin":   query.OriginNode,
		"ttl":      "3",
	}
	msgData, _ := json.Marshal(msg)
	if n.topic != nil {
		n.topic.Publish(n.ctx, msgData)
	}
}

func (n *NodoAlset) manejarMemoriaDistribuida(update map[string]string, origen peer.ID) {
	ttl, _ := strconv.Atoi(update["ttl"])
	if ttl <= 0 {
		return
	}
	cid := update["cid"]
	content := update["content"]
	n.mu.RLock()
	_, existe := n.blockstore[cid]
	n.mu.RUnlock()
	if !existe {
		n.GenerarCID([]byte(content))
		fmt.Printf("📚 Memoria distribuida recibida y almacenada: %s\n", cid)
	}
	if ttl > 1 {
		update["ttl"] = strconv.Itoa(ttl - 1)
		msgData, _ := json.Marshal(update)
		peers := n.host.Network().Peers()
		for _, p := range peers {
			if p != origen && n.topic != nil {
				go n.topic.Publish(n.ctx, msgData)
			}
		}
	}
}

func (n *NodoAlset) buscarMemoriaDistribuida(query string, maxHops int) string {
	queryID := generarUUID()
	responseChan := make(chan MemoryResponse, 10)
	n.memoryMu.Lock()
	n.pendingMemoryQueries[queryID] = responseChan
	n.memoryMu.Unlock()
	defer func() {
		time.Sleep(5 * time.Second)
		n.memoryMu.Lock()
		delete(n.pendingMemoryQueries, queryID)
		n.memoryMu.Unlock()
	}()
	msg := map[string]string{
		"tipo":     "memory_query",
		"query_id": queryID,
		"query":    query,
		"origin":   n.host.ID().String(),
		"ttl":      strconv.Itoa(maxHops),
	}
	msgData, _ := json.Marshal(msg)
	if n.topic != nil {
		n.topic.Publish(n.ctx, msgData)
	}
	select {
	case resp := <-responseChan:
		if len(resp.Contents) > 0 {
			return resp.Contents[0]
		}
		return ""
	case <-time.After(3 * time.Second):
		return ""
	}
}

func (n *NodoAlset) manejarConsultaMemoria(update map[string]string, origen peer.ID) {
	query := update["query"]
	queryID := update["query_id"]
	ttl, _ := strconv.Atoi(update["ttl"])
	resultados := n.buscarEnMemoriaLocalConContenido(query)
	if len(resultados) > 0 {
		resp := MemoryResponse{
			QueryID:       queryID,
			Results:       resultados[0].Results,
			Contents:      resultados[0].Contents,
			ResponderNode: n.host.ID().String(),
		}
		respData, _ := json.Marshal(resp)
		respMsg := map[string]string{
			"tipo": "memory_response",
			"data": string(respData),
		}
		msgData, _ := json.Marshal(respMsg)
		if n.topic != nil {
			n.topic.Publish(n.ctx, msgData)
		}
	} else if ttl > 1 {
		update["ttl"] = strconv.Itoa(ttl - 1)
		msgData, _ := json.Marshal(update)
		peers := n.host.Network().Peers()
		for _, p := range peers {
			if p != origen && n.topic != nil {
				go n.topic.Publish(n.ctx, msgData)
			}
		}
	}
}

func (n *NodoAlset) procesarRespuestaMemoria(update map[string]string) {
	var resp MemoryResponse
	if err := json.Unmarshal([]byte(update["data"]), &resp); err != nil {
		return
	}
	n.memoryMu.RLock()
	ch, exists := n.pendingMemoryQueries[resp.QueryID]
	n.memoryMu.RUnlock()
	if exists {
		select {
		case ch <- resp:
		default:
		}
	}
}

func (n *NodoAlset) propagarSpikeASinapsis(intensidad float64, timestamp int64) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.neuralState == nil {
		return
	}
	for targetID, sinapsis := range n.neuralState.Synapses {
		senalSalida := intensidad * sinapsis.Weight
		spikeMsg := map[string]string{
			"tipo":       "neural_spike",
			"intensidad": fmt.Sprintf("%f", senalSalida),
			"timestamp":  fmt.Sprintf("%d", timestamp),
			"origen":     n.host.ID().String(),
			"target":     targetID,
		}
		data, _ := json.Marshal(spikeMsg)
		if n.topic != nil {
			go n.topic.Publish(n.ctx, data)
		}
	}
}

func (n *NodoAlset) procesarSpikeNeuronal(update map[string]string, origen peer.ID) {
	intensidad, _ := strconv.ParseFloat(update["intensidad"], 64)
	timestamp, _ := strconv.ParseInt(update["timestamp"], 10, 64)
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.neuralState == nil {
		return
	}
	ahora := time.Now().UnixNano()
	if n.neuralState.LastSpikeTime > 0 {
		tiempoTranscurrido := float64(ahora - n.neuralState.LastSpikeTime)
		decaimiento := math.Exp(-tiempoTranscurrido * n.neuralState.LeakRate)
		n.neuralState.MembranePotential *= decaimiento
	}
	n.neuralState.MembranePotential += intensidad
	if n.neuralState.MembranePotential >= n.neuralState.SpikeThreshold {
		n.neuralState.LastSpikeTime = ahora
		n.neuralState.MembranePotential = 0
		go n.propagarSpikeASinapsis(intensidad, timestamp)
	}
}

func (n *NodoAlset) manejarInferenciaDistribuida(update map[string]string, origen peer.ID) {
	var req InferenceRequest
	if err := json.Unmarshal([]byte(update["data"]), &req); err != nil {
		return
	}
	if req.TTL <= 0 {
		respuesta := InferenceResponse{
			RequestID:      req.RequestID,
			OutputData:     []float64{-1},
			ProcessingNode: n.host.ID().String(),
			ProcessingTime: time.Now().UnixNano(),
		}
		n.publicarRespuestaInferencia(respuesta)
		return
	}
	req.TTL--
	puedeProcesar := n.puedeProcesarInferencia(req.InputData)
	if puedeProcesar {
		go n.procesarInferenciaLocal(req)
	} else {
		nodoDestino := n.seleccionarMejorVecinoParaInferencia(req.InputData)
		if nodoDestino != "" {
			n.reenviarSolicitudInferencia(req, nodoDestino)
		} else {
			go n.procesarInferenciaLocal(req)
		}
	}
}

func (n *NodoAlset) procesarInferenciaLocal(req InferenceRequest) {
	var output float64 = 0
	for _, val := range req.InputData {
		output += val
	}
	if len(req.InputData) > 0 {
		output = output / float64(len(req.InputData))
	}
	output = 1.0 / (1.0 + math.Exp(-output))
	respuesta := InferenceResponse{
		RequestID:      req.RequestID,
		OutputData:     []float64{output},
		ProcessingNode: n.host.ID().String(),
		ProcessingTime: time.Now().UnixNano(),
	}
	n.publicarRespuestaInferencia(respuesta)
}

func (n *NodoAlset) publicarRespuestaInferencia(respuesta InferenceResponse) {
	data, _ := json.Marshal(respuesta)
	msg := map[string]string{
		"tipo": "inference_response",
		"data": string(data),
	}
	msgData, _ := json.Marshal(msg)
	n.topic.Publish(n.ctx, msgData)
}

func (n *NodoAlset) procesarRespuestaInferencia(update map[string]string) {
	var respuesta InferenceResponse
	if err := json.Unmarshal([]byte(update["data"]), &respuesta); err != nil {
		return
	}
	n.inferenceMu.RLock()
	ch, exists := n.pendingInferences[respuesta.RequestID]
	n.inferenceMu.RUnlock()
	if exists {
		select {
		case ch <- respuesta:
		default:
		}
		go func() {
			time.Sleep(5 * time.Second)
			n.inferenceMu.Lock()
			delete(n.pendingInferences, respuesta.RequestID)
			n.inferenceMu.Unlock()
		}()
	}
}

func (n *NodoAlset) actualizarPesosSinapsis(update map[string]string, origen peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.neuralState == nil {
		return
	}
	neuronasPre := strings.Split(update["neuronas_pre"], ",")
	neuronasPost := strings.Split(update["neuronas_post"], ",")
	exito := update["exito"] == "true"
	tasaAprendizaje := 0.01
	if pesoStr, ok := update["peso"]; ok {
		if peso, err := strconv.ParseFloat(pesoStr, 64); err == nil && peso > 0 {
			tasaAprendizaje = peso * 0.01
		}
	}
	for _, pre := range neuronasPre {
		for _, post := range neuronasPost {
			key := pre + "->" + post
			if sinapsis, exists := n.neuralState.Synapses[key]; exists {
				if exito {
					sinapsis.Weight += tasaAprendizaje * (1 - sinapsis.Weight)
					sinapsis.SuccessfulFires++
				} else {
					sinapsis.Weight *= (1 - tasaAprendizaje)
				}
				if sinapsis.Weight > 1 {
					sinapsis.Weight = 1
				}
				if sinapsis.Weight < 0 {
					sinapsis.Weight = 0
				}
				sinapsis.LastUpdated = time.Now().Unix()
				n.neuralState.Synapses[key] = sinapsis
				n.hebbianMemory[key] = sinapsis.Weight
			}
		}
	}
	go n.persistirEstadoNeuronal()
}

func (n *NodoAlset) sincronizarEstadoNeuronal(update map[string]string, origen peer.ID) {
	n.mu.RLock()
	if n.neuralState == nil {
		n.mu.RUnlock()
		return
	}
	n.mu.RUnlock()
	stateJSON, _ := json.Marshal(n.neuralState)
	respuesta := map[string]string{
		"tipo":        "neural_state_sync_response",
		"estado":      string(stateJSON),
		"nodo_origen": n.host.ID().String(),
	}
	data, _ := json.Marshal(respuesta)
	n.topic.Publish(n.ctx, data)
}

// =============================================================================
// NETWORKING & GOSSIP SYNC (EXISTENTES)
// =============================================================================

func (n *NodoAlset) AnunciarNuevoBloque(cidStr string) {
	// 1. Publicar en gossip (opcional, lo dejamos por compatibilidad)
	update := map[string]string{"tipo": "new_block", "cid": cidStr}
	data, _ := json.Marshal(update)
	if n.topic != nil {
		n.topic.Publish(n.ctx, data)
	}

	// 2. Emitir por pulsos (HTTP)
	n.mu.RLock()
	blockData, exists := n.blockstore[cidStr]
	n.mu.RUnlock()

	if exists {
		// Codificar el bloque en base64 para transmitirlo
		b64 := base64.StdEncoding.EncodeToString(blockData)
		n.broadcastPulse("new_block", map[string]interface{}{
			"cid":  cidStr,
			"data": b64,
		})
		log.Printf("📤 Bloque %s emitido por pulso (%d bytes)", cidStr, len(blockData))
	} else {
		// Si no tenemos el bloque localmente (caso raro), solo anunciamos el CID
		n.broadcastPulse("new_block", map[string]interface{}{
			"cid": cidStr,
		})
		log.Printf("📤 Anuncio de bloque %s (sin datos) emitido por pulso", cidStr)
	}
}
