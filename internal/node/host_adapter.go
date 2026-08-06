package node

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"

	"redalset/internal/agents"
	"redalset/internal/neural"
)

func (n *NodoAlset) Lock()    { n.mu.Lock() }
func (n *NodoAlset) Unlock()  { n.mu.Unlock() }
func (n *NodoAlset) RLock()   { n.mu.RLock() }
func (n *NodoAlset) RUnlock() { n.mu.RUnlock() }

func (n *NodoAlset) BroadcastPulse(eventType string, data interface{}) {
	n.broadcastPulse(eventType, data)
}

func (n *NodoAlset) PersistirEstadoNeuronal() {
	n.persistirEstadoNeuronal()
}

func (n *NodoAlset) GetAgent(id string) (*agents.Agente, bool) {
	a, ok := n.agentes[id]
	return a, ok
}

func (n *NodoAlset) PutAgent(a *agents.Agente) {
	if a == nil {
		return
	}
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	n.agentes[a.ID] = a
}

func (n *NodoAlset) SetNombre(alias, agentID string) {
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	n.nombres[alias] = agentID
}

func (n *NodoAlset) GetNombre(alias string) (string, bool) {
	v, ok := n.nombres[alias]
	return v, ok
}

func (n *NodoAlset) Sign(msg []byte) ([]byte, error) {
	if n.masterPrivKey == nil {
		return nil, fmt.Errorf("no master key")
	}
	return n.masterPrivKey.Sign(msg)
}

func (n *NodoAlset) HasMasterKey() bool {
	return n.masterPrivKey != nil
}

func (n *NodoAlset) PublishTopic(msg []byte) error {
	if n.topic == nil {
		return fmt.Errorf("no pubsub topic")
	}
	return n.topic.Publish(n.ctx, msg)
}

func (n *NodoAlset) Ctx() context.Context {
	if n.ctx == nil {
		return context.Background()
	}
	return n.ctx
}

func (n *NodoAlset) ConnectPeer(ctx context.Context, pi peer.AddrInfo) error {
	return n.host.Connect(ctx, pi)
}

func (n *NodoAlset) GetNeural() *neural.NeuralState {
	return n.neuralState
}

func (n *NodoAlset) EnsureNeural() *neural.NeuralState {
	if n.neuralState == nil {
		n.neuralState = &NeuralState{
			SpikeThreshold: 1.0,
			LeakRate:       0.1,
			Synapses:       make(map[string]SynapticWeight),
			NeuronType:     "relay",
		}
	}
	return n.neuralState
}

func (n *NodoAlset) GetBlock(cid string) ([]byte, bool) {
	d, ok := n.blockstore[cid]
	return d, ok
}

func (n *NodoAlset) PutBlock(cid string, data []byte) {
	if n.blockstore == nil {
		n.blockstore = make(map[string][]byte)
	}
	n.blockstore[cid] = data
}

// Compile-time check: *NodoAlset implements nodeiface.Host
var _ interface {
	Lock()
	Unlock()
} = (*NodoAlset)(nil)

func (n *NodoAlset) PeerID() string {
	if n.host == nil {
		return ""
	}
	return n.host.ID().String()
}

func (n *NodoAlset) ListBlocks() map[string][]byte {
	if n.blockstore == nil {
		return map[string][]byte{}
	}
	out := make(map[string][]byte, len(n.blockstore))
	for k, v := range n.blockstore {
		out[k] = v
	}
	return out
}

func (n *NodoAlset) PeerCount() int {
	if n.host == nil {
		return 0
	}
	return len(n.host.Network().Peers())
}
