package nodeiface

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"

	"redalset/internal/agents"
	"redalset/internal/neural"
)

// Host is the surface the Lisp engine and other packages need from the node.
type Host interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()

	Auditoria(accion, detalle string)
	GenerarCID(data []byte) (string, error)
	AnunciarNuevoBloque(cid string)
	PersistirLocamente()
	SincronizarConPares()
	SetAgentRoot(agentID, rootCID string)
	DifundirActualizacionDNS(alias, agentID string)
	BuscarContenidoPorCID(cid string) ([]byte, error)
	BroadcastPulse(eventType string, data interface{})
	PersistirEstadoNeuronal()

	GetAgent(id string) (*agents.Agente, bool)
	PutAgent(a *agents.Agente)

	SetNombre(alias, agentID string)
	GetNombre(alias string) (string, bool)

	Sign(msg []byte) ([]byte, error)
	HasMasterKey() bool

	PublishTopic(msg []byte) error
	Ctx() context.Context
	ConnectPeer(ctx context.Context, pi peer.AddrInfo) error
	PeerID() string
	PeerCount() int

	GetNeural() *neural.NeuralState
	EnsureNeural() *neural.NeuralState

	GetBlock(cid string) ([]byte, bool)
	PutBlock(cid string, data []byte)
	ListBlocks() map[string][]byte
}
