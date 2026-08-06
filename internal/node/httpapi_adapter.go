package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"redalset/internal/httpapi"
)

// httpAPIBackend adapts NodoAlset to httpapi.Backend.
type httpAPIBackend struct{ n *NodoAlset }

func (b *httpAPIBackend) CreateAgent() (*httpapi.Agent, error) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generando llave: %w", err)
	}
	id := hex.EncodeToString(pub[:8])
	ag := &Agente{
		ID:           id,
		BalanceUTXO:  0,
		UltimaActual: time.Now().Unix(),
	}
	b.n.mu.Lock()
	b.n.agentes[id] = ag
	b.n.mu.Unlock()
	b.n.Auditoria("AGENTE_REGISTRADO_HTTP", fmt.Sprintf("ID: %s | InitBalance: 0", id))
	b.n.PersistirLocamente()
	go b.n.SincronizarConPares()
	go b.n.broadcastPulse("agent_created", map[string]interface{}{
		"id": id, "root": "", "time": time.Now().Unix(),
	})
	return &httpapi.Agent{
		ID: ag.ID, RootCID: ag.RootCID, UltimaActual: ag.UltimaActual, BalanceUTXO: ag.BalanceUTXO,
	}, nil
}

func (b *httpAPIBackend) GetAgent(id string) (*httpapi.Agent, bool) {
	b.n.mu.RLock()
	defer b.n.mu.RUnlock()
	a, ok := b.n.agentes[id]
	if !ok || a == nil {
		return nil, false
	}
	return &httpapi.Agent{
		ID: a.ID, RootCID: a.RootCID, UltimaActual: a.UltimaActual, BalanceUTXO: a.BalanceUTXO,
	}, true
}

func (b *httpAPIBackend) ListAgents() map[string]*httpapi.Agent {
	b.n.mu.RLock()
	defer b.n.mu.RUnlock()
	out := make(map[string]*httpapi.Agent, len(b.n.agentes))
	for id, a := range b.n.agentes {
		if a == nil {
			continue
		}
		out[id] = &httpapi.Agent{
			ID: a.ID, RootCID: a.RootCID, UltimaActual: a.UltimaActual, BalanceUTXO: a.BalanceUTXO,
		}
	}
	return out
}

func (b *httpAPIBackend) ListBlocks() []map[string]interface{} {
	b.n.mu.RLock()
	defer b.n.mu.RUnlock()
	list := make([]map[string]interface{}, 0, len(b.n.blockstore))
	for cid, data := range b.n.blockstore {
		preview := string(data)
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		list = append(list, map[string]interface{}{
			"cid": cid, "size": len(data), "preview": preview,
		})
	}
	return list
}

func (b *httpAPIBackend) ListPeers() []map[string]interface{} {
	if b.n.host == nil {
		return []map[string]interface{}{}
	}
	peers := b.n.host.Network().Peers()
	out := make([]map[string]interface{}, 0, len(peers))
	for _, p := range peers {
		out = append(out, map[string]interface{}{
			"id":        p.String(),
			"addresses": b.n.host.Network().Peerstore().Addrs(p),
			"connected": b.n.host.Network().Connectedness(p).String(),
		})
	}
	return out
}

func (b *httpAPIBackend) ListDNS() map[string]string {
	b.n.mu.RLock()
	defer b.n.mu.RUnlock()
	out := make(map[string]string, len(b.n.nombres))
	for k, v := range b.n.nombres {
		out[k] = v
	}
	return out
}
