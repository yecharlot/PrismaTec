package node

import (
	"context"
	"crypto/rand"
	"log"
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

	"redalset/internal/lisp"
)

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
		libp2p.EnableNATService(),
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
	n.lisp = lisp.NewEvaluator(n)
	mdns.NewMdnsService(n.host, "alset-mesh", &discoveryNotifee{h: n.host}).Start()
	go n.EscucharGossip()

	go n.QuickStartup()
}

type discoveryNotifee struct{ h host.Host }

func (d *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	d.h.Connect(context.Background(), pi)
}
