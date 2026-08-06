package node

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"os"
	"os/signal"
	"path/filepath"
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

	"redalset/internal/config"
	"redalset/internal/lisp"
	"redalset/internal/persistence"
)

// Constants re-exported from internal/config for call-site compatibility.
const (
	AlsetProtocolID     = config.AlsetProtocolID
	AlsetDataExchangeID = config.AlsetDataExchangeID
	AlsetGossipTopic    = config.AlsetGossipTopic
	BlocksDir           = config.BlocksDir
	StaticDir           = config.StaticDir
	AdminPanelCIDKey    = config.AdminPanelCIDKey
)

const (
	NeuralSpikeTopic       = config.NeuralSpikeTopic
	InferenceRequestTopic  = config.InferenceRequestTopic
	InferenceResponseTopic = config.InferenceResponseTopic
	SynapticUpdateTopic    = config.SynapticUpdateTopic
	MemoryQueryTopic       = config.MemoryQueryTopic
	MemoryResponseTopic    = config.MemoryResponseTopic
	NeuralStateSyncTopic   = config.NeuralStateSyncTopic
	MemoryDistributedTopic = config.MemoryDistributedTopic
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
	nodeID := "local"
	if n.host != nil {
		nodeID = n.host.ID().String()
	}
	line := AuditLine{
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    accion,
		Detail:    detalle,
		NodeID:    nodeID,
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
	if n.kademlia == nil || n.host == nil {
		return nil, fmt.Errorf("no encontrado")
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
