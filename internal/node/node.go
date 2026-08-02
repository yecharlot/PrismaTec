package node

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	mathrand "math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"golang.org/x/crypto/bcrypt"

	"redalset/internal/agents"
	"redalset/internal/poh"
	"redalset/internal/lisp"
	"redalset/internal/persistence"
)

// =============================================================================
// CONSTANTES
// =============================================================================

const AlsetProtocolID = "/ptec-an/sync/1.0.0"
const AlsetDataExchangeID = "/ptec-an/data/1.0.0"
const AlsetGossipTopic = "ptec-an-v4.0"
const BlocksDir = "blocks"
const StaticDir = "static"
const AdminPanelCIDKey = "admin_panel_cid"

const (
	NeuralSpikeTopic       = "ptec-an-neural-spike"
	InferenceRequestTopic  = "ptec-an-inference-request"
	InferenceResponseTopic = "ptec-an-inference-response"
	SynapticUpdateTopic    = "ptec-an-synaptic-update"
	MemoryQueryTopic       = "ptec-an-memory-query"
	MemoryResponseTopic    = "ptec-an-memory-response"
	NeuralStateSyncTopic   = "ptec-an-neural-sync"
	MemoryDistributedTopic = "ptec-an-memory-distributed"
)

// =============================================================================
// GITHUB PERSISTENCE
// =============================================================================

// NODO ALSET – ESTRUCTURA PRINCIPAL
// =============================================================================

type NodoAlset struct {
	host                 host.Host
	ctx                  context.Context
	agentes              map[string]*Agente
	mu                   sync.RWMutex
	lisp                 *lisp.Evaluator
	kademlia             *dht.IpfsDHT
	pubsub               *pubsub.PubSub
	topic                *pubsub.Topic
	datastore            datastore.Batching
	blockstore           map[string][]byte
	nombres              map[string]string
	masterPrivKey        crypto.PrivKey
	neuralState          *NeuralState
	pendingInferences    map[string]chan InferenceResponse
	pendingMemoryQueries map[string]chan MemoryResponse
	inferenceMu          sync.RWMutex
	memoryMu             sync.RWMutex
	hebbianMemory        map[string]float64
	startTime            int64
	syncManager          *SyncManager

	// ---- NUEVO SISTEMA DE PULSOS ----
	pulseSubscribers   map[*SSESubscriber]bool
	pulseSubscribersMu sync.RWMutex
	pulseClients       map[string]*PulseClient
	pulseClientsMu     sync.RWMutex
	pulseKnownServers  []string

	// Persistencia pluggable (Local o Supabase)
	store persistence.Store
}

type BlockInfo struct {
	CID     string `json:"cid"`
	Size    int    `json:"size"`
	Preview string `json:"preview"`
}

type SSESubscriber struct {
	ch     chan string
	ctx    context.Context
	cancel context.CancelFunc
}

type PulseClient struct {
	url       string
	ctx       context.Context
	cancel    context.CancelFunc
	connected bool
	lastEvent time.Time
	reconnect chan bool
}

// =============================================================================
// MÉTODOS DEL NODO – EXISTENTES
// =============================================================================

func (n *NodoAlset) Auditoria(accion string, detalle string) {
	type AuditLine struct {
		Timestamp string `json:"ts"`
		Action    string `json:"action"`
		Detail    string `json:"detail"`
		NodeID    string `json:"node_id"`
	}
	line := AuditLine{
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    accion,
		Detail:    detalle,
		NodeID:    n.host.ID().String(),
	}
	data, _ := json.Marshal(line)
	f, _ := os.OpenFile("audit.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.Write(data)
	f.WriteString("\n")
	f.Sync()
}

func (n *NodoAlset) LoadMasterKey() {
	keyFile := "master_identity.key"
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		priv, _, _ := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
		raw, _ := crypto.MarshalPrivateKey(priv)
		os.WriteFile(keyFile, raw, 0600)
		n.masterPrivKey = priv
		fmt.Println("🔑 Nueva Clave Maestra generada y guardada.")
	} else {
		raw, _ := os.ReadFile(keyFile)
		priv, _ := crypto.UnmarshalPrivateKey(raw)
		n.masterPrivKey = priv
		fmt.Println("🔑 Clave Maestra institucional cargada correctamente.")
	}
}

func (n *NodoAlset) GenerarCID(data []byte) (string, error) {
	pref := cid.Prefix{Version: 1, Codec: cid.Raw, MhType: multihash.SHA2_256, MhLength: -1}
	c, _ := pref.Sum(data)
	cidStr := c.String()
	n.mu.Lock()
	n.blockstore[cidStr] = data
	n.mu.Unlock()
	_ = os.MkdirAll(BlocksDir, 0755)
	_ = os.WriteFile(filepath.Join(BlocksDir, cidStr), data, 0644)
	return cidStr, nil
}

func (n *NodoAlset) BuscarContenidoPorCID(cidStr string) ([]byte, error) {
	n.mu.RLock()
	data, existe := n.blockstore[cidStr]
	n.mu.RUnlock()
	if existe {
		return data, nil
	}
	if diskData, err := os.ReadFile(filepath.Join(BlocksDir, cidStr)); err == nil {
		n.mu.Lock()
		n.blockstore[cidStr] = diskData
		n.mu.Unlock()
		return diskData, nil
	}
	c, _ := cid.Decode(cidStr)
	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	defer cancel()
	providers := n.kademlia.FindProvidersAsync(ctx, c, 5)
	for p := range providers {
		if p.ID == n.host.ID() {
			continue
		}
		s, err := n.host.NewStream(n.ctx, p.ID, AlsetDataExchangeID)
		if err != nil {
			continue
		}
		s.Write([]byte(cidStr + "\n"))
		res, _ := io.ReadAll(s)
		s.Close()
		if len(res) > 0 {
			n.GenerarCID(res)
			return res, nil
		}
	}
	return nil, fmt.Errorf("no encontrado")
}

func (n *NodoAlset) PersistirLocamente() {
	n.mu.RLock()
	defer n.mu.RUnlock()

	ctx := context.Background()
	if n.store == nil {
		dAg, _ := json.MarshalIndent(n.agentes, "", "  ")
		_ = os.WriteFile("alset_data/alset_state.json", dAg, 0644)
		dAn, _ := json.MarshalIndent(n.nombres, "", "  ")
		_ = os.WriteFile("alset_data/alset_names.json", dAn, 0644)
		n.persistirEstadoNeuronal()
		return
	}

	// Agentes → alset_agents (uno por fila)
	agentBlobs := make(map[string][]byte, len(n.agentes))
	for id, ag := range n.agentes {
		if b, err := json.Marshal(ag); err == nil {
			agentBlobs[id] = b
		}
	}
	if err := n.store.SaveAgents(ctx, agentBlobs); err != nil {
		log.Printf("⚠️ Error guardando agentes: %v", err)
	}
	// Backup KV del mapa completo (compatibilidad)
	if dAg, err := json.Marshal(n.agentes); err == nil {
		_ = n.store.Save(ctx, persistence.KeyState, dAg)
	}

	// Nombres → alset_kv
	if dAn, err := json.Marshal(n.nombres); err == nil {
		if err := n.store.Save(ctx, persistence.KeyNames, dAn); err != nil {
			log.Printf("⚠️ Error guardando nombres: %v", err)
		}
	}

	// Neural → alset_neural_state
	if n.neuralState != nil {
		if dN, err := json.Marshal(n.neuralState); err == nil {
			if err := n.store.SaveNeuralState(ctx, "main", dN); err != nil {
				log.Printf("⚠️ Error guardando neural state: %v", err)
			}
		}
	}

	// Blocks → alset_blocks
	if len(n.blockstore) > 0 {
		if err := n.store.SaveBlocks(ctx, n.blockstore); err != nil {
			log.Printf("⚠️ Error guardando blocks: %v", err)
		}
	}
}

func (n *NodoAlset) CargarEstado() {
	ctx := context.Background()

	if n.store != nil {
		// Agentes desde tabla estructurada
		if blobs, err := n.store.LoadAgents(ctx); err == nil && len(blobs) > 0 {
			n.mu.Lock()
			if n.agentes == nil {
				n.agentes = make(map[string]*Agente)
			}
			for id, raw := range blobs {
				var ag Agente
				if json.Unmarshal(raw, &ag) == nil {
					n.agentes[id] = &ag
				}
			}
			n.mu.Unlock()
		} else if d, err := n.store.Load(ctx, persistence.KeyState); err == nil && d != nil {
			// Fallback legacy KV
			n.mu.Lock()
			_ = json.Unmarshal(d, &n.agentes)
			n.mu.Unlock()
		}

		if d, err := n.store.Load(ctx, persistence.KeyNames); err == nil && d != nil {
			n.mu.Lock()
			_ = json.Unmarshal(d, &n.nombres)
			n.mu.Unlock()
		}

		if d, err := n.store.LoadNeuralState(ctx, "main"); err == nil && d != nil {
			n.mu.Lock()
			if n.neuralState == nil {
				n.neuralState = &NeuralState{}
			}
			_ = json.Unmarshal(d, n.neuralState)
			n.mu.Unlock()
		}

		if blocks, err := n.store.LoadBlocks(ctx); err == nil && len(blocks) > 0 {
			n.mu.Lock()
			n.blockstore = blocks
			n.mu.Unlock()
		}
	}

	// Complemento: bloques en disco local
	files, _ := os.ReadDir(BlocksDir)
	n.mu.Lock()
	if n.blockstore == nil {
		n.blockstore = make(map[string][]byte)
	}
	for _, f := range files {
		if !f.IsDir() {
			if _, ok := n.blockstore[f.Name()]; !ok {
				if d, err := os.ReadFile(filepath.Join(BlocksDir, f.Name())); err == nil {
					n.blockstore[f.Name()] = d
				}
			}
		}
	}
	n.mu.Unlock()
	fmt.Printf("📂 Alset Engine: %d agentes, %d nombres y %d bloques en RAM.\n", len(n.agentes), len(n.nombres), len(n.blockstore))
}

// =============================================================================
// PERSISTENCIA EN GITHUB
// =============================================================================

func (n *NodoAlset) PersistirEnGitHub() error {
	// GitHub persistence has been removed.
	// Use the new internal/persistence layer (Local or Supabase) instead.
	return nil
}

func (n *NodoAlset) CargarDesdeGitHub() error {
	// GitHub persistence has been removed.
	// Use the new internal/persistence layer (Local or Supabase) instead.
	return nil
}

func (n *NodoAlset) IpfsAddDirectory(dirPath string) (string, error) {
	files := make(map[string][]byte)
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(dirPath, path)
		files[relPath] = data
		return nil
	})
	if err != nil {
		return "", err
	}
	jsonData, _ := json.Marshal(files)
	cid, err := n.GenerarCID(jsonData)
	if err != nil {
		return "", err
	}
	fmt.Printf("📁 Directorio subido a IPFS: %s → %s\n", dirPath, cid)
	return cid, nil
}

func (n *NodoAlset) RegisterApp(appName string) (string, error) {
	appPath := filepath.Join(StaticDir, "apps", appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return "", fmt.Errorf("app no encontrada: %s", appName)
	}
	cid, err := n.IpfsAddDirectory(appPath)
	if err != nil {
		return "", err
	}
	createCmd := fmt.Sprintf(`(crear-agente "%s")`, appName)
	_, err = n.lisp.Eval(createCmd)
	if err != nil {
		return "", err
	}
	var agentID string
	n.mu.RLock()
	for id, agent := range n.agentes {
		if agent.ID == appName {
			agentID = id
			break
		}
	}
	n.mu.RUnlock()
	if agentID == "" {
		return "", fmt.Errorf("no se pudo crear el agente para: %s", appName)
	}
	setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, agentID, cid)
	_, err = n.lisp.Eval(setRootCmd)
	if err != nil {
		return "", err
	}
	registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, appName, agentID)
	_, err = n.lisp.Eval(registerCmd)
	if err != nil {
		return "", err
	}
	fmt.Printf("✅ App registrada: %s → %s (CID: %s)\n", appName, agentID, cid)
	return agentID, nil
}

// =============================================================================
// MÉTODOS DE IA DISTRIBUIDA (EXISTENTES)
// =============================================================================

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

func (n *NodoAlset) getAdminPanelHTML() string {
	return `<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Alset Network - Panel de Administración</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%);
            color: #fff;
            min-height: 100vh;
        }
        .header {
            background: rgba(0,0,0,0.8);
            backdrop-filter: blur(10px);
            padding: 1rem 2rem;
            border-bottom: 2px solid #f4b400;
            position: sticky;
            top: 0;
            z-index: 100;
        }
        .header h1 {
            font-size: 1.5rem;
            display: inline-block;
        }
        .header .node-id {
            float: right;
            font-family: monospace;
            color: #f4b400;
            margin-top: 0.3rem;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }
        .card {
            background: rgba(20,20,40,0.9);
            backdrop-filter: blur(5px);
            border-radius: 12px;
            padding: 1.5rem;
            border: 1px solid rgba(244,180,0,0.2);
            transition: transform 0.2s, border-color 0.2s;
        }
        .card:hover {
            transform: translateY(-2px);
            border-color: rgba(244,180,0,0.5);
        }
        .card h3 {
            color: #f4b400;
            margin-bottom: 1rem;
            font-size: 0.9rem;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .card .value {
            font-size: 2.5rem;
            font-weight: bold;
            margin-bottom: 0.5rem;
        }
        .card .label {
            color: #888;
            font-size: 0.85rem;
        }
        .section {
            background: rgba(20,20,40,0.9);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
            border: 1px solid rgba(244,180,0,0.2);
        }
        .section h2 {
            color: #f4b400;
            margin-bottom: 1rem;
            font-size: 1.2rem;
        }
        .sync-btn {
            background: #f4b400;
            color: #000;
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            cursor: pointer;
            font-weight: bold;
            margin-right: 0.5rem;
            transition: opacity 0.2s;
        }
        .sync-btn:hover { opacity: 0.8; }
        .sync-progress {
            width: 100%;
            height: 20px;
            background: rgba(255,255,255,0.1);
            border-radius: 10px;
            overflow: hidden;
            margin-top: 1rem;
        }
        .sync-progress-bar {
            height: 100%;
            background: #f4b400;
            width: 0%;
            transition: width 0.3s;
        }
        .log-container {
            background: #0a0a0a;
            border-radius: 8px;
            padding: 1rem;
            max-height: 300px;
            overflow-y: auto;
            font-family: monospace;
            font-size: 0.8rem;
        }
        .log-entry {
            padding: 0.25rem 0;
            border-bottom: 1px solid rgba(255,255,255,0.05);
        }
        button {
            background: rgba(244,180,0,0.2);
            border: 1px solid #f4b400;
            color: #f4b400;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.2s;
        }
        button:hover {
            background: #f4b400;
            color: #000;
        }
        .agent-list {
            max-height: 400px;
            overflow-y: auto;
        }
        .agent-item {
            padding: 0.5rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            font-family: monospace;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        .syncing { animation: pulse 1s infinite; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🌐 Alset Network</h1>
        <div class="node-id" id="nodeId">Cargando...</div>
    </div>
    <div class="container">
        <div class="stats-grid">
            <div class="card">
                <h3>Agentes</h3>
                <div class="value" id="agentesCount">-</div>
                <div class="label">Agentes registrados en la red</div>
            </div>
            <div class="card">
                <h3>Bloques IPFS</h3>
                <div class="value" id="bloquesCount">-</div>
                <div class="label">Bloques almacenados localmente</div>
            </div>
            <div class="card">
                <h3>Peers Conectados</h3>
                <div class="value" id="peersCount">-</div>
                <div class="label">Nodos en la red</div>
            </div>
            <div class="card">
                <h3>Estado Sincronización</h3>
                <div class="value" id="syncStatus">-</div>
                <div class="label">Última sincronización</div>
            </div>
        </div>
        
        <div class="section">
            <h2>🔄 Sincronización</h2>
            <button class="sync-btn" onclick="startFullSync()">Sincronización Completa</button>
            <button class="sync-btn" onclick="startQuickSync()">Sincronización Rápida</button>
            <button onclick="refreshStatus()">Actualizar Estado</button>
            <div id="syncProgressContainer" style="display:none;">
                <div class="sync-progress">
                    <div class="sync-progress-bar" id="syncProgressBar"></div>
                </div>
                <p id="syncStatusText" style="margin-top: 0.5rem;"></p>
            </div>
        </div>
        
        <div class="section">
            <h2>📋 Agentes Registrados</h2>
            <div class="agent-list" id="agentList">
                <div>Cargando...</div>
            </div>
        </div>
        
        <div class="section">
            <h2>📊 Últimos Eventos</h2>
            <div class="log-container" id="logContainer">
                <div>Cargando...</div>
            </div>
        </div>
    </div>
    
    <script>
        let refreshInterval;
        
        async function fetchAPI(endpoint) {
            try {
                const response = await fetch(endpoint);
                return await response.json();
            } catch (error) {
                console.error('Error:', error);
                return null;
            }
        }
        
        async function refreshStatus() {
            const agentes = await fetchAPI('/api/agentes/');
            const ipfsList = await fetchAPI('/api/ipfs/list');
            const peers = await fetchAPI('/api/network/peers');
            const syncStatus = await fetchAPI('/api/sync/status');
            
            if (agentes) document.getElementById('agentesCount').innerText = Object.keys(agentes).length;
            if (ipfsList) document.getElementById('bloquesCount').innerText = ipfsList.length;
            if (peers) document.getElementById('peersCount').innerText = peers.length;
            
            if (syncStatus) {
                const lastSync = syncStatus.last_sync ? new Date(syncStatus.last_sync * 1000).toLocaleString() : 'Nunca';
                if (syncStatus.is_syncing) {
                    document.getElementById('syncStatus').innerHTML = '<span class="syncing">🔄 Sincronizando...</span>';
                } else {
                    document.getElementById('syncStatus').innerHTML = lastSync;
                }
            }
            
            if (agentes) {
                const agentListDiv = document.getElementById('agentList');
                if (Object.keys(agentes).length === 0) {
                    agentListDiv.innerHTML = '<div>No hay agentes registrados</div>';
                } else {
                    let html = '';
                    for (const [id, agent] of Object.entries(agentes)) {
                        html += '<div class="agent-item">' + id + ' - Root: ' + (agent.root_cid || 'Ninguno') + ' - Balance: ' + agent.balance_utxo + '</div>';
                    }
                    agentListDiv.innerHTML = html;
                }
            }
        }
        
        async function startFullSync() {
            document.getElementById('syncProgressContainer').style.display = 'block';
            document.getElementById('syncStatusText').innerText = 'Iniciando sincronización completa...';
            
            const response = await fetch('/api/sync/full', { method: 'POST' });
            const result = await response.json();
            
            document.getElementById('syncStatusText').innerText = result.message;
            
            const interval = setInterval(async () => {
                const status = await fetchAPI('/api/sync/status');
                if (status && status.progress) {
                    document.getElementById('syncProgressBar').style.width = (status.progress.percent * 100) + '%';
                    document.getElementById('syncStatusText').innerText = status.progress.status;
                }
                if (status && !status.is_syncing) {
                    clearInterval(interval);
                    setTimeout(() => {
                        document.getElementById('syncProgressContainer').style.display = 'none';
                    }, 2000);
                    refreshStatus();
                }
            }, 1000);
        }
        
        async function startQuickSync() {
            document.getElementById('syncProgressContainer').style.display = 'block';
            document.getElementById('syncStatusText').innerText = 'Iniciando sincronización rápida...';
            
            const response = await fetch('/api/sync/quick', { method: 'POST' });
            const result = await response.json();
            
            document.getElementById('syncStatusText').innerText = result.message;
            setTimeout(() => {
                document.getElementById('syncProgressContainer').style.display = 'none';
                refreshStatus();
            }, 3000);
        }
        
        async function loadLogs() {
            const logs = await fetchAPI('/api/audit/log');
            if (logs && logs.length > 0) {
                const logContainer = document.getElementById('logContainer');
                let html = '';
                for (let i = 0; i < Math.min(logs.length, 50); i++) {
                    const log = logs[i];
                    html += '<div class="log-entry">[' + log.ts + '] ' + log.action + ': ' + (log.detail ? log.detail.substring(0, 100) : '') + '</div>';
                }
                logContainer.innerHTML = html;
            }
        }
        
        async function loadNodeId() {
            const status = await fetchAPI('/api/sync/status');
            if (status && status.node_id) {
                document.getElementById('nodeId').innerText = status.node_id;
            }
        }
        
        refreshStatus();
        loadLogs();
        loadNodeId();
        refreshInterval = setInterval(function() {
            refreshStatus();
            loadLogs();
        }, 5000);
    </script>
</body>
</html>`
}

func (n *NodoAlset) publishAdminPanelCID() {
	html := n.getAdminPanelHTML()
	cid, err := n.GenerarCID([]byte(html))
	if err != nil {
		fmt.Println("❌ Error generando CID del panel de administración:", err)
		return
	}
	config := NodoConfig{
		AdminPanelCID: cid,
		IsGenesis:     true,
		Version:       "4.0.0-PTEC-AN",
		LastUpdate:    time.Now().Unix(),
	}
	configBytes, _ := json.Marshal(config)
	n.GenerarCID(configBytes)
	os.WriteFile("nodo_config.json", configBytes, 0644)
	announce := map[string]string{
		"tipo": "admin_panel_announce",
		"cid":  cid,
	}
	announceBytes, _ := json.Marshal(announce)
	if n.topic != nil {
		n.topic.Publish(n.ctx, announceBytes)
	}
	fmt.Println("📢 Panel de administración publicado en IPFS con CID:", cid)
}

func (n *NodoAlset) handleAdminPanelAnnounce(update map[string]string) {
	cid := update["cid"]
	if cid == "" {
		return
	}
	panelPath := filepath.Join(StaticDir, "index.html")
	if _, err := os.Stat(panelPath); err == nil {
		return
	}
	fmt.Println("📥 Descargando panel de administración desde la red...")
	data, err := n.BuscarContenidoPorCID(cid)
	if err != nil {
		fmt.Println("❌ Error descargando panel de administración:", err)
		return
	}
	os.MkdirAll(StaticDir, 0755)
	err = os.WriteFile(panelPath, data, 0644)
	if err != nil {
		fmt.Println("❌ Error guardando panel de administración:", err)
		return
	}
	fmt.Println("✅ Panel de administración descargado y guardado en:", panelPath)
}

func (n *NodoAlset) ensureStaticFiles() {
	os.MkdirAll(StaticDir, 0755)
	os.MkdirAll(filepath.Join(StaticDir, "apps"), 0755)
	panelPath := filepath.Join(StaticDir, "index.html")
	if _, err := os.Stat(panelPath); err == nil {
		return
	}
	if configData, err := os.ReadFile("nodo_config.json"); err == nil {
		var config NodoConfig
		if json.Unmarshal(configData, &config) == nil && config.AdminPanelCID != "" {
			fmt.Println("📥 Restaurando panel de administración desde configuración local...")
			data, err := n.BuscarContenidoPorCID(config.AdminPanelCID)
			if err == nil {
				os.WriteFile(panelPath, data, 0644)
				fmt.Println("✅ Panel de administración restaurado")
				return
			}
		}
	}
	fmt.Println("🌟 Nodo genesis: creando panel de administración inicial...")
	n.publishAdminPanelCID()
	html := n.getAdminPanelHTML()
	os.WriteFile(panelPath, []byte(html), 0644)
	fmt.Println("✅ Panel de administración creado en:", panelPath)
}

// =============================================================================
// SISTEMA DE SINCRONIZACIÓN (EXISTENTE)
// =============================================================================

func (n *NodoAlset) InitSyncManager() *SyncManager {
	config := SyncConfig{
		Mode:           SyncModeQuick,
		AutoSyncDays:   7,
		MaxQuickBlocks: 100,
	}
	if data, err := os.ReadFile("sync_config.json"); err == nil {
		json.Unmarshal(data, &config)
	}
	if data, err := os.ReadFile("last_sync.json"); err == nil {
		var lastSync struct {
			Timestamp int64 `json:"timestamp"`
		}
		json.Unmarshal(data, &lastSync)
		config.LastSyncTime = lastSync.Timestamp
	}
	sm := &SyncManager{
		nodo:   n,
		config: config,
	}
	n.syncManager = sm
	return sm
}

func (sm *SyncManager) SaveConfig() {
	data, _ := json.MarshalIndent(sm.config, "", "  ")
	os.WriteFile("sync_config.json", data, 0644)
}

func (sm *SyncManager) SaveLastSyncTime() {
	data, _ := json.Marshal(map[string]int64{"timestamp": time.Now().Unix()})
	os.WriteFile("last_sync.json", data, 0644)
}

func (n *NodoAlset) QuickStartup() {
	fmt.Println("🚀 Arranque rápido iniciado...")
	n.ensureStaticFiles()
	n.CargarEstado()
	go n.connectToNetwork()

	go func() {
		time.Sleep(3 * time.Second)
		if n.syncManager != nil && n.shouldQuickSync() {
			n.syncManager.PerformQuickSync()
		}
	}()

	// ---- Iniciar cliente de pulsos SOLO si NO estamos en Render ----
	if os.Getenv("RENDER") == "" {
		go n.startPulseClients()
		fmt.Println("⚡ Cliente de pulsos iniciado (modo local)")
	} else {
		fmt.Println("⚡ Cliente de pulsos desactivado (nodo en Render, solo actúa como servidor)")
	}

	fmt.Println("✅ Nodo operativo (sincronización en background)")
	fmt.Println("🌐 Panel de administración: http://localhost:" + getPort() + "/static/index.html")
}
func getPort() string {
	return "8080"
}

func (n *NodoAlset) shouldQuickSync() bool {
	if len(n.agentes) == 0 {
		return true
	}
	if n.syncManager.config.LastSyncTime == 0 {
		return true
	}
	daysSinceSync := (time.Now().Unix() - n.syncManager.config.LastSyncTime) / 86400
	return daysSinceSync > int64(n.syncManager.config.AutoSyncDays)
}

func (sm *SyncManager) PerformQuickSync() {
	sm.mu.Lock()
	if sm.isSyncing {
		sm.mu.Unlock()
		return
	}
	sm.isSyncing = true
	sm.mu.Unlock()
	defer func() { sm.isSyncing = false }()
	fmt.Println("⚡ Sincronización rápida iniciada...")
	peers := sm.nodo.host.Network().Peers()
	if len(peers) == 0 {
		fmt.Println("⚠️ No hay peers disponibles para sincronizar")
		return
	}
	for _, p := range peers {
		stream, err := sm.nodo.host.NewStream(sm.nodo.ctx, p, AlsetDataExchangeID)
		if err != nil {
			continue
		}
		stream.Write([]byte("SYNC_QUICK_REQUEST\n"))
		sizeBuf := make([]byte, 8)
		_, err = io.ReadFull(stream, sizeBuf)
		if err != nil {
			stream.Close()
			continue
		}
		size := binary.BigEndian.Uint64(sizeBuf)
		data := make([]byte, size)
		_, err = io.ReadFull(stream, data)
		stream.Close()
		if err != nil {
			continue
		}
		gz, _ := gzip.NewReader(bytes.NewReader(data))
		decompressed, _ := io.ReadAll(gz)
		gz.Close()
		var response struct {
			Agentes      map[string]*Agente `json:"agentes"`
			Nombres      map[string]string  `json:"nombres"`
			RecentBlocks map[string][]byte  `json:"recent_blocks"`
			NeuralState  *NeuralState       `json:"neural_state"`
		}
		json.Unmarshal(decompressed, &response)
		sm.nodo.mu.Lock()
		for k, v := range response.Agentes {
			if _, exists := sm.nodo.agentes[k]; !exists {
				sm.nodo.agentes[k] = v
			}
		}
		for k, v := range response.Nombres {
			if _, exists := sm.nodo.nombres[k]; !exists {
				sm.nodo.nombres[k] = v
			}
		}
		for k, v := range response.RecentBlocks {
			if _, exists := sm.nodo.blockstore[k]; !exists {
				sm.nodo.blockstore[k] = v
				os.WriteFile(filepath.Join(BlocksDir, k), v, 0644)
			}
		}
		if response.NeuralState != nil && sm.nodo.neuralState == nil {
			sm.nodo.neuralState = response.NeuralState
		}
		sm.nodo.mu.Unlock()
		sm.nodo.PersistirLocamente()
		sm.SaveLastSyncTime()
		fmt.Printf("✅ Sincronización rápida completada: %d agentes, %d bloques\n",
			len(response.Agentes), len(response.RecentBlocks))
		return
	}
}

func (sm *SyncManager) PerformFullSync(ctx context.Context, progressCallback func(float64)) error {
	sm.mu.Lock()
	if sm.isSyncing {
		sm.mu.Unlock()
		return fmt.Errorf("ya hay una sincronización en curso")
	}
	sm.isSyncing = true
	sm.mu.Unlock()
	defer func() { sm.isSyncing = false }()
	fmt.Println("🔄 Sincronización completa iniciada...")
	if progressCallback != nil {
		progressCallback(0.1)
	}
	peers := sm.nodo.host.Network().Peers()
	if len(peers) == 0 {
		return fmt.Errorf("no hay peers disponibles para sincronizar")
	}
	for _, p := range peers {
		stream, err := sm.nodo.host.NewStream(ctx, p, AlsetDataExchangeID)
		if err != nil {
			continue
		}
		stream.Write([]byte("SYNC_FULL_REQUEST\n"))
		sizeBuf := make([]byte, 8)
		_, err = io.ReadFull(stream, sizeBuf)
		if err != nil {
			stream.Close()
			continue
		}
		size := binary.BigEndian.Uint64(sizeBuf)
		data := make([]byte, size)
		_, err = io.ReadFull(stream, data)
		stream.Close()
		if err != nil {
			continue
		}
		gz, _ := gzip.NewReader(bytes.NewReader(data))
		decompressed, _ := io.ReadAll(gz)
		gz.Close()
		var fullState struct {
			Agentes map[string]*Agente `json:"agentes"`
			Nombres map[string]string  `json:"nombres"`
		}
		json.Unmarshal(decompressed, &fullState)
		if progressCallback != nil {
			progressCallback(0.5)
		}
		sm.nodo.mu.Lock()
		for k, v := range fullState.Agentes {
			sm.nodo.agentes[k] = v
		}
		for k, v := range fullState.Nombres {
			sm.nodo.nombres[k] = v
		}
		sm.nodo.mu.Unlock()
		if progressCallback != nil {
			progressCallback(1.0)
		}
		sm.nodo.PersistirLocamente()
		sm.SaveLastSyncTime()
		fmt.Printf("✅ Sincronización completa: %d agentes, %d nombres\n",
			len(fullState.Agentes), len(fullState.Nombres))
		return nil
	}
	return fmt.Errorf("no se pudo completar la sincronización con ningún peer")
}

func (n *NodoAlset) connectToNetwork() {
	time.Sleep(2 * time.Second)
	fmt.Println("🌐 Conectado a la red Alset")
}

// =============================================================================
// HANDLERS DE MÓDULOS, ENTIDADES, SEGURIDAD (EXISTENTES)
// =============================================================================

func (n *NodoAlset) crearModulo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nombre    string                 `json:"nombre"`
		Rol       string                 `json:"rol"`
		Atributos map[string]interface{} `json:"atributos"`
		Owner     string                 `json:"owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	id := generarUUID()
	modulo := &Modulo{
		ID:         id,
		Nombre:     req.Nombre,
		Rol:        req.Rol,
		Atributos:  req.Atributos,
		Relaciones: []string{},
		Owner:      req.Owner,
		CreatedAt:  time.Now().Unix(),
	}
	agents.Global.MuModulos.Lock()
	agents.Global.Modulos[id] = modulo
	agents.Global.MuModulos.Unlock()
	n.Auditoria("MODULO_CREADO", fmt.Sprintf("ID: %s | Nombre: %s", id, req.Nombre))
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) listarModulos(w http.ResponseWriter, r *http.Request) {
	agents.Global.MuModulos.RLock()
	defer agents.Global.MuModulos.RUnlock()
	lista := make([]*Modulo, 0, len(agents.Global.Modulos))
	for _, m := range agents.Global.Modulos {
		lista = append(lista, m)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	agents.Global.MuModulos.RLock()
	modulo, exists := agents.Global.Modulos[id]
	agents.Global.MuModulos.RUnlock()
	if !exists {
		http.Error(w, "Módulo no encontrado", 404)
		return
	}
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) actualizarModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	agents.Global.MuModulos.Lock()
	defer agents.Global.MuModulos.Unlock()
	modulo, exists := agents.Global.Modulos[id]
	if !exists {
		http.Error(w, "Módulo no encontrado", 404)
		return
	}
	if nombre, ok := updates["nombre"]; ok {
		modulo.Nombre = nombre.(string)
	}
	if rol, ok := updates["rol"]; ok {
		modulo.Rol = rol.(string)
	}
	if atributos, ok := updates["atributos"]; ok {
		modulo.Atributos = atributos.(map[string]interface{})
	}
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) eliminarModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	agents.Global.MuModulos.Lock()
	delete(agents.Global.Modulos, id)
	agents.Global.MuModulos.Unlock()
	n.Auditoria("MODULO_ELIMINADO", id)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (n *NodoAlset) crearEntidad(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tipo      string                 `json:"tipo"`
		Atributos map[string]interface{} `json:"atributos"`
		HeredaDe  string                 `json:"hereda_de"`
		ModuloID  string                 `json:"modulo_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	atributosFinales := make(map[string]interface{})
	if req.HeredaDe != "" {
		agents.Global.MuEntidades.RLock()
		if padre, exists := agents.Global.Entidades[req.HeredaDe]; exists {
			for k, v := range padre.Atributos {
				atributosFinales[k] = v
			}
		}
		agents.Global.MuEntidades.RUnlock()
	}
	for k, v := range req.Atributos {
		atributosFinales[k] = v
	}
	id := generarUUID()
	entidad := &EntidadProgramatica{
		ID:        id,
		Tipo:      req.Tipo,
		Atributos: atributosFinales,
		HeredaDe:  req.HeredaDe,
		ModuloID:  req.ModuloID,
	}
	agents.Global.MuEntidades.Lock()
	agents.Global.Entidades[id] = entidad
	agents.Global.MuEntidades.Unlock()
	n.Auditoria("ENTIDAD_CREADA", fmt.Sprintf("Tipo: %s | ID: %s", req.Tipo, id))
	json.NewEncoder(w).Encode(entidad)
}

func (n *NodoAlset) listarEntidades(w http.ResponseWriter, r *http.Request) {
	agents.Global.MuEntidades.RLock()
	defer agents.Global.MuEntidades.RUnlock()
	lista := make([]*EntidadProgramatica, 0, len(agents.Global.Entidades))
	for _, e := range agents.Global.Entidades {
		lista = append(lista, e)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerEntidad(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/entidades/")
	agents.Global.MuEntidades.RLock()
	entidad, exists := agents.Global.Entidades[id]
	agents.Global.MuEntidades.RUnlock()
	if !exists {
		http.Error(w, "Entidad no encontrada", 404)
		return
	}
	json.NewEncoder(w).Encode(entidad)
}

func (n *NodoAlset) crearRelacion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntidadA     string `json:"entidad_a"`
		EntidadB     string `json:"entidad_b"`
		Tipo         string `json:"tipo"`
		Cardinalidad string `json:"cardinalidad"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	id := generarUUID()
	relacion := &RelacionEntidad{
		ID:           id,
		EntidadA:     req.EntidadA,
		EntidadB:     req.EntidadB,
		Tipo:         req.Tipo,
		Cardinalidad: req.Cardinalidad,
	}
	agents.Global.Relaciones[id] = relacion
	json.NewEncoder(w).Encode(relacion)
}

func (n *NodoAlset) listarRelaciones(w http.ResponseWriter, r *http.Request) {
	lista := make([]*RelacionEntidad, 0, len(agents.Global.Relaciones))
	for _, rel := range agents.Global.Relaciones {
		lista = append(lista, rel)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerRelacionesDeEntidad(w http.ResponseWriter, r *http.Request) {
	entidadID := strings.TrimPrefix(r.URL.Path, "/api/entidades/")
	entidadID = strings.TrimSuffix(entidadID, "/relaciones")
	var resultado []*RelacionEntidad
	for _, rel := range agents.Global.Relaciones {
		if rel.EntidadA == entidadID || rel.EntidadB == entidadID {
			resultado = append(resultado, rel)
		}
	}
	json.NewEncoder(w).Encode(resultado)
}

func (n *NodoAlset) generarTokenAlset(agentID string, roles []string, duracionHoras int) (*TokenAlset, error) {
	agente, exists := n.agentes[agentID]
	if !exists {
		return nil, fmt.Errorf("agente no encontrado")
	}
	tokenID := generarUUID()
	expiresAt := time.Now().Add(time.Duration(duracionHoras) * time.Hour).Unix()
	payload := fmt.Sprintf("%s|%s|%d|%s", tokenID, agentID, expiresAt, strings.Join(roles, ","))
	signature := hex.EncodeToString([]byte(payload + n.host.ID().String()))[:64]
	token := &TokenAlset{
		Token:     tokenID,
		AgentID:   agentID,
		RootCID:   agente.RootCID,
		ExpiresAt: expiresAt,
		Roles:     roles,
		Permisos:  n.rolesAPermisos(roles),
		Signature: signature,
	}
	agents.Global.MuTokens.Lock()
	agents.Global.Tokens[tokenID] = token
	agents.Global.MuTokens.Unlock()
	return token, nil
}

func (n *NodoAlset) rolesAPermisos(roles []string) []string {
	permisosMap := make(map[string]bool)
	for _, rol := range roles {
		switch rol {
		case "admin":
			permisosMap["*"] = true
		case "editor":
			permisosMap["modulo:crear"] = true
			permisosMap["modulo:editar"] = true
			permisosMap["entidad:crear"] = true
			permisosMap["entidad:editar"] = true
		case "viewer":
			permisosMap["modulo:ver"] = true
			permisosMap["entidad:ver"] = true
		case "cliente":
			permisosMap["producto:ver"] = true
			permisosMap["compra:crear"] = true
		case "vendedor":
			permisosMap["producto:crear"] = true
			permisosMap["producto:editar"] = true
			permisosMap["venta:ver"] = true
		}
	}
	var permisos []string
	for p := range permisosMap {
		permisos = append(permisos, p)
	}
	return permisos
}

func (n *NodoAlset) validarToken(tokenString string) (*TokenAlset, error) {
	agents.Global.MuTokens.RLock()
	token, exists := agents.Global.Tokens[tokenString]
	agents.Global.MuTokens.RUnlock()
	if !exists {
		return nil, fmt.Errorf("token inválido")
	}
	if time.Now().Unix() > token.ExpiresAt {
		agents.Global.MuTokens.Lock()
		delete(agents.Global.Tokens, tokenString)
		agents.Global.MuTokens.Unlock()
		return nil, fmt.Errorf("token expirado")
	}
	payload := fmt.Sprintf("%s|%s|%d|%s", token.Token, token.AgentID, token.ExpiresAt, strings.Join(token.Roles, ","))
	expectedSig := hex.EncodeToString([]byte(payload + n.host.ID().String()))[:64]
	if token.Signature != expectedSig {
		return nil, fmt.Errorf("firma inválida")
	}
	return token, nil
}

func (n *NodoAlset) verificarPermiso(token *TokenAlset, permisoRequerido string) bool {
	if token == nil {
		return false
	}
	for _, p := range token.Permisos {
		if p == "*" || p == permisoRequerido {
			return true
		}
	}
	return false
}

func (n *NodoAlset) asignarRol(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string   `json:"agent_id"`
		Roles   []string `json:"roles"`
		Modulos []string `json:"modulos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if _, exists := n.agentes[req.AgentID]; !exists {
		http.Error(w, "Agente no encontrado", 404)
		return
	}
	usuarioRoles := &UsuarioRoles{
		AgentID: req.AgentID,
		Roles:   req.Roles,
		Modulos: req.Modulos,
	}
	rolesData, _ := json.Marshal(usuarioRoles)
	cid, _ := n.GenerarCID(rolesData)
	agents.Global.Roles[req.AgentID] = req.Roles
	n.Auditoria("ROL_ASIGNADO", fmt.Sprintf("Agente: %s | Roles: %v", req.AgentID, req.Roles))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"cid":    cid,
	})
}

func (n *NodoAlset) obtenerRoles(w http.ResponseWriter, r *http.Request) {
	agentID := strings.TrimPrefix(r.URL.Path, "/api/roles/")
	roles, exists := agents.Global.Roles[agentID]
	if !exists {
		json.NewEncoder(w).Encode([]string{})
		return
	}
	json.NewEncoder(w).Encode(roles)
}

func (n *NodoAlset) crearTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID       string   `json:"agent_id"`
		Roles         []string `json:"roles"`
		DuracionHoras int      `json:"duracion_horas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	duracion := req.DuracionHoras
	if duracion <= 0 {
		duracion = 24
	}
	token, err := n.generarTokenAlset(req.AgentID, req.Roles, duracion)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(token)
}

func (n *NodoAlset) validarTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Token requerido", 400)
		return
	}
	token, err := n.validarToken(tokenStr)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	json.NewEncoder(w).Encode(token)
}

func (n *NodoAlset) revocarTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	agents.Global.MuTokens.Lock()
	delete(agents.Global.Tokens, req.Token)
	agents.Global.MuTokens.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"status": "revocado"})
}

func (n *NodoAlset) SetAgentRoot(agentID string, rootCID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if a, ok := n.agentes[agentID]; ok {
		a.RootCID = rootCID
		a.UltimaActual = time.Now().Unix()
	}
}

// =============================================================================
// HANDLERS DE SINCRONIZACIÓN HTTP (EXISTENTES)
// =============================================================================

func (n *NodoAlset) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if n.syncManager == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not_initialized",
		})
		return
	}
	n.syncManager.mu.RLock()
	defer n.syncManager.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "idle",
		"is_syncing":     n.syncManager.isSyncing,
		"last_sync":      n.syncManager.config.LastSyncTime,
		"mode":           n.syncManager.config.Mode,
		"agents_count":   len(n.agentes),
		"blocks_count":   len(n.blockstore),
		"auto_sync_days": n.syncManager.config.AutoSyncDays,
		"node_id":        n.host.ID().String(),
		"progress":       globalSyncProgress,
	})
}

func (n *NodoAlset) handleSyncFull(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if n.syncManager == nil {
		http.Error(w, "Sync manager no inicializado", 500)
		return
	}
	go func() {
		globalSyncProgress.Status = "syncing"
		globalSyncProgress.Stage = "full_sync"
		err := n.syncManager.PerformFullSync(context.Background(), func(progress float64) {
			globalSyncProgress.Percent = progress
			globalSyncProgress.Current = int(progress * 100)
			globalSyncProgress.Total = 100
		})
		if err != nil {
			globalSyncProgress.Status = "error"
			globalSyncProgress.Stage = err.Error()
		} else {
			globalSyncProgress.Status = "idle"
			globalSyncProgress.Stage = "completed"
		}
	}()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "sync_started",
		"message": "Sincronización completa iniciada en background",
	})
}

func (n *NodoAlset) handleSyncQuick(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if n.syncManager == nil {
		http.Error(w, "Sync manager no inicializado", 500)
		return
	}
	go n.syncManager.PerformQuickSync()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "sync_started",
		"message": "Sincronización rápida iniciada",
	})
}

func (n *NodoAlset) handleSyncConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var config struct {
		Mode           string `json:"mode"`
		AutoSyncDays   int    `json:"auto_sync_days"`
		MaxQuickBlocks int    `json:"max_quick_blocks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if n.syncManager != nil {
		switch config.Mode {
		case "quick":
			n.syncManager.config.Mode = SyncModeQuick
		case "full":
			n.syncManager.config.Mode = SyncModeFull
		case "incremental":
			n.syncManager.config.Mode = SyncModeIncremental
		}
		if config.AutoSyncDays > 0 {
			n.syncManager.config.AutoSyncDays = config.AutoSyncDays
		}
		if config.MaxQuickBlocks > 0 {
			n.syncManager.config.MaxQuickBlocks = config.MaxQuickBlocks
		}
		n.syncManager.SaveConfig()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "configured",
		"config": config,
	})
}

// =============================================================================
// HANDLERS HTTP ADICIONALES (EXISTENTES)
// =============================================================================

func (n *NodoAlset) handlePoHEvent(w http.ResponseWriter, r *http.Request) {
	var event PoHEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid event data", 400)
		return
	}
	poh.Global.Lock()
	if poh.Global.SessionID() == "" {
		poh.Global.SetSessionID(hex.EncodeToString([]byte(time.Now().String()))[:16])
	}
	event.Timestamp = time.Now().Unix()
	poh.Global.Append(event)
	poh.Global.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"status": "event_received"})
}

func (n *NodoAlset) handlePoHProof(w http.ResponseWriter, r *http.Request) {
	poh.Global.Lock()
	defer poh.Global.Unlock()
	if len(poh.Global.Events()) == 0 {
		http.Error(w, "No events collected", 400)
		return
	}
	var eventsData []byte
	for _, ev := range poh.Global.Events() {
		evData, _ := json.Marshal(ev)
		eventsData = append(eventsData, evData...)
	}
	hash := make([]byte, 32)
	copy(hash, eventsData[:32])
	proof := HumanityProof{
		SessionID: poh.Global.SessionID(),
		Events:    poh.Global.Events(),
		FinalSig:  hex.EncodeToString(hash),
	}
	proofBytes, _ := json.Marshal(proof)
	proofCID, _ := n.GenerarCID(proofBytes)
	poh.Global.ClearEvents()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"proof_cid": proofCID,
		"session":   proof.SessionID,
		"events":    len(proof.Events),
	})
}

func (n *NodoAlset) handleDNSList(w http.ResponseWriter, r *http.Request) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nombres": n.nombres,
	})
}

func (n *NodoAlset) handleDNSResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	n.mu.RLock()
	agentID, exists := n.nombres[req.Alias]
	n.mu.RUnlock()
	if !exists {
		http.Error(w, "Nombre no encontrado", 404)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alias":    req.Alias,
		"agent_id": agentID,
		"status":   "active",
	})
}

func (n *NodoAlset) handleDNSDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	n.mu.Lock()
	delete(n.nombres, req.Alias)
	n.mu.Unlock()
	n.PersistirLocamente()
	n.Auditoria("DNS_ELIMINADO", fmt.Sprintf("Alias: %s", req.Alias))
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"alias":  req.Alias,
	})
}

func (n *NodoAlset) handleNetworkPeers(w http.ResponseWriter, r *http.Request) {
	peers := n.host.Network().Peers()
	peerInfo := make([]map[string]interface{}, 0, len(peers))
	for _, p := range peers {
		peerInfo = append(peerInfo, map[string]interface{}{
			"id":        p.String(),
			"addresses": n.host.Network().Peerstore().Addrs(p),
			"connected": n.host.Network().Connectedness(p).String(),
		})
	}
	json.NewEncoder(w).Encode(peerInfo)
}

func (n *NodoAlset) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("audit.jsonl")
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	lines := strings.Split(string(data), "\n")
	logs := make([]map[string]interface{}, 0)
	for _, line := range lines {
		if line == "" {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
			logs = append(logs, logEntry)
		}
	}
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	json.NewEncoder(w).Encode(logs)
}

func (n *NodoAlset) handleDebugEstado(w http.ResponseWriter, r *http.Request) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agentes_count": len(n.agentes),
		"nombres_count": len(n.nombres),
		"agentes":       n.agentes,
	})
}

func (n *NodoAlset) handleAppsRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err := r.ParseMultipartForm(50 << 20)
		if err != nil {
			http.Error(w, "Error parsing form: "+err.Error(), 400)
			return
		}
		appName := r.FormValue("appName")
		if appName == "" {
			http.Error(w, "appName required", 400)
			return
		}
		appDir := filepath.Join(StaticDir, "apps", appName)
		if err := os.MkdirAll(appDir, 0755); err != nil {
			http.Error(w, "Error creating app directory: "+err.Error(), 500)
			return
		}
		files := r.MultipartForm.File["files"]
		var savedFiles []string
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue
			}
			defer file.Close()
			filename := fileHeader.Filename
			targetPath := filepath.Join(appDir, filename)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				continue
			}
			data, err := io.ReadAll(file)
			if err != nil {
				continue
			}
			if err := os.WriteFile(targetPath, data, 0644); err != nil {
				continue
			}
			savedFiles = append(savedFiles, filename)
		}
		if len(savedFiles) == 0 {
			http.Error(w, "No files were saved", 400)
			return
		}
		fmt.Printf("📁 Archivos guardados en: %s (%d archivos)\n", appDir, len(savedFiles))
		cid, err := n.IpfsAddDirectory(appDir)
		if err != nil {
			fmt.Printf("⚠️ Error uploading to IPFS: %v\n", err)
		}
		appID := fmt.Sprintf("app-%s-%d", appName, time.Now().Unix())
		createCmd := fmt.Sprintf(`(crear-agente "%s")`, appID)
		n.lisp.Eval(createCmd)
		if cid != "" {
			setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, appID, cid)
			n.lisp.Eval(setRootCmd)
		}
		registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, appName, appID)
		n.lisp.Eval(registerCmd)
		indexPath := filepath.Join(appDir, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			indexContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        html, body { width: 100%%; height: 100%%; overflow: hidden; background: #000; }
        #app { width: 100vw; height: 100vh; display: flex; }
    </style>
</head>
<body>
    <div id="app"></div>
    <script type="module" src="/apps/%s/%s.js"></script>
</body>
</html>`, appName, appName, appName)
			os.WriteFile(indexPath, []byte(indexContent), 0644)
			fmt.Printf("📄 Index.html creado: %s\n", indexPath)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "registered",
			"name":     appName,
			"cid":      cid,
			"url":      fmt.Sprintf("/w/%s.app.ans", appName),
			"agent_id": appID,
			"path":     appDir,
			"files":    len(savedFiles),
		})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), 400)
		return
	}
	if req.Name == "" {
		http.Error(w, "App name required", 400)
		return
	}
	appPath := filepath.Join(StaticDir, "apps", req.Name)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		http.Error(w, "App folder not found: "+req.Name, 404)
		return
	}
	cid, err := n.IpfsAddDirectory(appPath)
	if err != nil {
		http.Error(w, "Error uploading to IPFS: "+err.Error(), 500)
		return
	}
	appID := fmt.Sprintf("app-%s-%d", req.Name, time.Now().Unix())
	createCmd := fmt.Sprintf(`(crear-agente "%s")`, appID)
	n.lisp.Eval(createCmd)
	setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, appID, cid)
	n.lisp.Eval(setRootCmd)
	registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, req.Name, appID)
	n.lisp.Eval(registerCmd)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "registered",
		"name":     req.Name,
		"cid":      cid,
		"url":      fmt.Sprintf("/w/%s.app.ans", req.Name),
		"agent_id": appID,
	})
}

func (n *NodoAlset) handleAppsList(w http.ResponseWriter, r *http.Request) {
	appsDir := filepath.Join(StaticDir, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	var apps []map[string]interface{}
	for _, entry := range entries {
		if entry.IsDir() {
			apps = append(apps, map[string]interface{}{
				"name": entry.Name(),
				"path": fmt.Sprintf("/static/apps/%s", entry.Name()),
			})
		}
	}
	json.NewEncoder(w).Encode(apps)
}

func (n *NodoAlset) handlePrismVerificar(w http.ResponseWriter, r *http.Request) {
	certCID := r.URL.Query().Get("cid")
	if certCID == "" {
		http.Error(w, "CID requerido", 400)
		return
	}
	certBytes, err := n.BuscarContenidoPorCID(certCID)
	if err != nil {
		n.Auditoria("VERIFICACION_FALLIDA", fmt.Sprintf("CID: %s | Motivo: No encontrado", certCID))
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Certificado no encontrado"})
		return
	}
	var vc map[string]interface{}
	if err := json.Unmarshal(certBytes, &vc); err != nil {
		n.Auditoria("VERIFICACION_ERROR", fmt.Sprintf("CID: %s | Motivo: JSON inválido", certCID))
		http.Error(w, "JSON inválido", 400)
		return
	}
	proofInterface, hasProof := vc["proof"]
	if !hasProof {
		n.Auditoria("VERIFICACION_ERROR", fmt.Sprintf("CID: %s | Motivo: Sin proof", certCID))
		http.Error(w, "Certificado sin prueba", 400)
		return
	}
	proof, ok := proofInterface.(map[string]interface{})
	if !ok {
		http.Error(w, "Proof inválido", 400)
		return
	}
	signatureStr := ""
	if pv, ok := proof["proofValue"]; ok {
		signatureStr, _ = pv.(string)
	} else if jws, ok := proof["jws"]; ok {
		signatureStr, _ = jws.(string)
	}
	if signatureStr == "" {
		http.Error(w, "Firma no encontrada", 400)
		return
	}
	firmaBytes, err := hex.DecodeString(signatureStr)
	if err != nil {
		http.Error(w, "Firma inválida", 400)
		return
	}
	vcWithoutProof := make(map[string]interface{})
	for k, v := range vc {
		if k != "proof" {
			vcWithoutProof[k] = v
		}
	}
	canonicalBytes, err := canonicalizeJSON(vcWithoutProof)
	if err != nil {
		http.Error(w, "Error canonicalizando", 500)
		return
	}
	rawKey, _ := n.masterPrivKey.GetPublic().Raw()
	pubNative := ed25519.PublicKey(rawKey)
	esFirmaValida := ed25519.Verify(pubNative, canonicalBytes, firmaBytes)
	estaRevocado := false
	motivoRevocacion := ""
	n.mu.RLock()
	for _, blockData := range n.blockstore {
		var rev map[string]interface{}
		if err := json.Unmarshal(blockData, &rev); err != nil {
			continue
		}
		if revType, ok := rev["type"]; ok {
			var typeStr string
			switch t := revType.(type) {
			case string:
				typeStr = t
			case []interface{}:
				if len(t) > 0 {
					if s, ok := t[0].(string); ok {
						typeStr = s
					}
				}
			}
			if typeStr == "RevocationList2020Credential" {
				if subject, ok := rev["credentialSubject"].(map[string]interface{}); ok {
					if revokedCID, ok := subject["revokedCredential"].(string); ok && revokedCID == certCID {
						estaRevocado = true
						if reason, ok := subject["revocationReason"].(string); ok {
							motivoRevocacion = reason
						}
						break
					}
				}
			}
		}
	}
	n.mu.RUnlock()
	statusVerdad := "AUTÉNTICO"
	auditAccion := "VC_VERIFICADO_OK"
	if !esFirmaValida {
		statusVerdad = "FALSIFICADO / INVÁLIDO"
		auditAccion = "ALERTA_FRAUDE_FIRMA"
	} else if estaRevocado {
		statusVerdad = "REVOCADO"
		auditAccion = "CONSULTA_VC_REVOCADO"
	}
	n.Auditoria(auditAccion, fmt.Sprintf("CID: %s | Status: %s", certCID, statusVerdad))
	credentialSubject := map[string]interface{}{}
	if cs, ok := vc["credentialSubject"]; ok {
		if csMap, ok := cs.(map[string]interface{}); ok {
			credentialSubject = csMap
		}
	}
	response := map[string]interface{}{
		"cid_consultado":  certCID,
		"status_integral": statusVerdad,
		"firma_valida":    esFirmaValida,
		"revocado":        estaRevocado,
		"info_revocacion": map[string]string{"motivo": motivoRevocacion},
		"detalles": map[string]interface{}{
			"issuer":            vc["issuer"],
			"issuanceDate":      vc["issuanceDate"],
			"credentialSubject": credentialSubject,
		},
		"nodo_verificador": n.host.ID().String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *NodoAlset) handlePrismRevocar(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		CID    string `json:"cid"`
		Motivo string `json:"motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if req.CID == "" {
		http.Error(w, "CID requerido", 400)
		return
	}
	if req.Motivo == "" {
		req.Motivo = "Revocado por administrador"
	}
	_, err := n.BuscarContenidoPorCID(req.CID)
	if err != nil {
		http.Error(w, "Certificado no encontrado", 404)
		return
	}
	fecha := time.Now().Format(time.RFC3339)
	mensaje := fmt.Sprintf("REVOKE|%s|%s", req.CID, fecha)
	firma, err := n.masterPrivKey.Sign([]byte(mensaje))
	if err != nil {
		http.Error(w, "Error firmando", 500)
		return
	}
	revocationTicket := map[string]interface{}{
		"@context":     "https://www.w3.org/2018/credentials/v1",
		"id":           fmt.Sprintf("urn:uuid:%s", req.CID),
		"type":         []string{"RevocationList2020Credential"},
		"issuer":       "did:prism:tec:institutional",
		"issuanceDate": fecha,
		"credentialSubject": map[string]interface{}{
			"id":                fmt.Sprintf("did:prism:%s", req.CID[:16]),
			"revokedCredential": req.CID,
			"revocationReason":  req.Motivo,
			"revocationDate":    fecha,
		},
		"proof": map[string]interface{}{
			"type":               "Ed25519Signature2020",
			"created":            fecha,
			"verificationMethod": "did:prism:tec:institutional#key-1",
			"proofPurpose":       "assertionMethod",
			"jws":                hex.EncodeToString(firma),
		},
	}
	ticketBytes, _ := json.Marshal(revocationTicket)
	revokeCID, err := n.GenerarCID(ticketBytes)
	if err != nil {
		http.Error(w, "Error generando CID", 500)
		return
	}
	n.Auditoria("CERTIFICADO_REVOCADO",
		fmt.Sprintf("Cert: %s | Motivo: %s", req.CID, req.Motivo))
	update := map[string]string{"tipo": "revocacion_update", "cid": revokeCID}
	msg, _ := json.Marshal(update)
	n.topic.Publish(n.ctx, msg)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":               "revocado",
		"ticket_cid":           revokeCID,
		"certificado_revocado": req.CID,
		"fecha":                fecha,
	})
}

func (n *NodoAlset) handlePrismSellar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CID string `json:"cid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	res, _ := n.lisp.Eval(fmt.Sprintf(`(sellar-documento "%s")`, req.CID))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "Certificado Generado",
		"entidad":         "Prism@.TEC - Garante de la Verdad Digital",
		"titular":         "Dayanis Pérez Soria",
		"certificado_cid": res,
	})
}

func (n *NodoAlset) handleAdminUpdatePass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NuevaClave string `json:"nuevaClave"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte(req.NuevaClave), bcrypt.DefaultCost)
	config := NodoConfig{
		AdminPassHash: string(hashedPass),
		LastUpdate:    time.Now().Unix(),
		Version:       "4.0.0-PTEC-AN",
	}
	configBytes, _ := json.Marshal(config)
	cidStr, _ := n.GenerarCID(configBytes)
	n.Auditoria("SEGURIDAD_PASSWORD_UPDATE", fmt.Sprintf("Nuevo CID de config: %s", cidStr))
	n.AnunciarNuevoBloque(cidStr)
	fmt.Printf("🔒 [SEGURIDAD] Nueva configuración sellada con CID: %s\n", cidStr)
	json.NewEncoder(w).Encode(map[string]string{"config_cid": cidStr})
}

func (n *NodoAlset) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CID   string `json:"config_cid"`
		Clave string `json:"clave"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Solicitud inválida", 400)
		return
	}
	configBytes, err := n.BuscarContenidoPorCID(req.CID)
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Configuración no encontrada"})
		return
	}
	var config NodoConfig
	json.Unmarshal(configBytes, &config)
	err = bcrypt.CompareHashAndPassword([]byte(config.AdminPassHash), []byte(req.Clave))
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Clave incorrecta"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "authorized", "node": n.host.ID().String()})
}

// =============================================================================
// SERVICIO DE PULSOS – NUEVO
// =============================================================================

func (n *NodoAlset) broadcastPulse(eventType string, data interface{}) {
	payload, _ := json.Marshal(data)
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(payload))

	n.pulseSubscribersMu.RLock()
	defer n.pulseSubscribersMu.RUnlock()
	for sub := range n.pulseSubscribers {
		select {
		case sub.ch <- msg:
		default:
			// canal lleno, omitir
		}
	}
}

func Run(port string) {
	fmt.Println("🌐 PRISM@.TEC ALSET NET (P.TEC-AN) v4.0")
	fmt.Println("📦 Sistema Híbrido Go + Lisp con IA Distribuida, VC, UTXO, PoH y ZKP")
	fmt.Println("🧠 Con IA Distribuida: Neuronas, Sinapsis, Inferencia Distribuida y Memoria Distribuida")
	fmt.Println("⚡ Con sistema de pulsos SSE para comunicación resiliente")

	if os.Getenv("RENDER") != "" {
		fmt.Println("🟢 Nodo ejecutándose en Render (servidor de pulsos)")
	} else {
		fmt.Println("🟢 Nodo ejecutándose localmente (cliente de pulsos)")
	}

	if port == "" {
		port = "8080"
	}

	nodo := &NodoAlset{
		ctx:                  context.Background(),
		agentes:              make(map[string]*Agente),
		pendingInferences:    make(map[string]chan InferenceResponse),
		pendingMemoryQueries: make(map[string]chan MemoryResponse),
		hebbianMemory:        make(map[string]float64),
		blockstore:           make(map[string][]byte),
		nombres:              make(map[string]string),
	}

	// Inicializar persistencia (Supabase si hay credenciales, sino Local)
	store, err := persistence.NewFromEnv("alset_data")
	if err != nil {
		log.Printf("⚠️ Error inicializando persistencia: %v – usando solo disco de emergencia", err)
	} else {
		nodo.store = store
	}

	mathrand.Seed(time.Now().UnixNano())
	nodo.Init()
	nodo.Auditoria("SISTEMA_START", fmt.Sprintf("Nodo Online en puerto %s", port))
	go nodo.startHTTPServer(port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	nodo.Auditoria("SISTEMA_STOP", "Apagado del nodo")
	nodo.PersistirLocamente()
	fmt.Println("👋 Nodo apagado correctamente")
}
