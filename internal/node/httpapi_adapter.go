package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"time"

	"redalset/internal/httpapi"
)

type httpAPIBackend struct{ n *NodoAlset }

func (b *httpAPIBackend) CreateAgent() (*httpapi.Agent, error) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generando llave: %w", err)
	}
	id := hex.EncodeToString(pub[:8])
	ag := &Agente{ID: id, BalanceUTXO: 1000.0, UltimaActual: time.Now().Unix()}
	b.n.mu.Lock()
	b.n.agentes[id] = ag
	b.n.mu.Unlock()
	b.n.Auditoria("AGENTE_REGISTRADO_HTTP", fmt.Sprintf("ID: %s", id))
	b.n.PersistirLocamente()
	go b.n.SincronizarConPares()
	go b.n.broadcastPulse("agent_created", map[string]interface{}{
		"id": id, "root": "", "time": time.Now().Unix(),
	})
	return toAPIAgent(ag), nil
}

func (b *httpAPIBackend) ListAgents() map[string]*httpapi.Agent {
	b.n.mu.RLock()
	defer b.n.mu.RUnlock()
	out := make(map[string]*httpapi.Agent, len(b.n.agentes))
	for id, a := range b.n.agentes {
		if a != nil {
			out[id] = toAPIAgent(a)
		}
	}
	return out
}

func (b *httpAPIBackend) GetAgent(id string) (*httpapi.Agent, bool) {
	b.n.mu.RLock()
	defer b.n.mu.RUnlock()
	a, ok := b.n.agentes[id]
	if !ok || a == nil {
		return nil, false
	}
	return toAPIAgent(a), true
}

func (b *httpAPIBackend) DeleteAgent(id string) error {
	b.n.mu.Lock()
	defer b.n.mu.Unlock()
	if _, ok := b.n.agentes[id]; !ok {
		return fmt.Errorf("agente no encontrado")
	}
	delete(b.n.agentes, id)
	b.n.Auditoria("AGENTE_ELIMINADO_HTTP", "ID: "+id)
	go b.n.PersistirLocamente()
	go b.n.broadcastPulse("agent_deleted", map[string]interface{}{"id": id})
	return nil
}

func (b *httpAPIBackend) UpdateAgent(id string, rootCID string, balance *float64) (*httpapi.Agent, error) {
	b.n.mu.Lock()
	defer b.n.mu.Unlock()
	a, ok := b.n.agentes[id]
	if !ok || a == nil {
		return nil, fmt.Errorf("agente no encontrado")
	}
	if rootCID != "" {
		a.RootCID = rootCID
	}
	if balance != nil {
		a.BalanceUTXO = *balance
	}
	a.UltimaActual = time.Now().Unix()
	b.n.Auditoria("AGENTE_MODIFICADO_HTTP", "ID: "+id)
	go b.n.PersistirLocamente()
	return toAPIAgent(a), nil
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
		list = append(list, map[string]interface{}{"cid": cid, "size": len(data), "preview": preview})
	}
	return list
}

func (b *httpAPIBackend) FetchBlock(cid string) ([]byte, error) {
	return b.n.BuscarContenidoPorCID(cid)
}

func (b *httpAPIBackend) PutBlock(cid string, data []byte) error {
	if cid == "" {
		var err error
		cid, err = b.n.GenerarCID(data)
		if err != nil {
			return err
		}
	}
	b.n.mu.Lock()
	if b.n.blockstore == nil {
		b.n.blockstore = make(map[string][]byte)
	}
	b.n.blockstore[cid] = data
	b.n.mu.Unlock()
	_ = os.MkdirAll(BlocksDir, 0o755)
	_ = os.WriteFile(BlocksDir+"/"+cid, data, 0o644)
	b.n.Auditoria("IPFS_BLOCK_ADDED", cid)
	return nil
}

func (b *httpAPIBackend) DeleteBlock(cid string) error {
	b.n.mu.Lock()
	defer b.n.mu.Unlock()
	if _, ok := b.n.blockstore[cid]; !ok {
		return fmt.Errorf("bloque no encontrado")
	}
	delete(b.n.blockstore, cid)
	_ = os.Remove(BlocksDir + "/" + cid)
	b.n.Auditoria("IPFS_BLOCK_DELETED", cid)
	return nil
}

func (b *httpAPIBackend) ClearBlocks() (int, error) {
	b.n.mu.Lock()
	defer b.n.mu.Unlock()
	n := len(b.n.blockstore)
	b.n.blockstore = make(map[string][]byte)
	_ = os.RemoveAll(BlocksDir)
	_ = os.MkdirAll(BlocksDir, 0o755)
	b.n.Auditoria("IPFS_BLOCKSTORE_LIMPIADA", fmt.Sprintf("Bloques eliminados: %d", n))
	return n, nil
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

func (b *httpAPIBackend) ResolveDNS(alias string) (string, bool) {
	b.n.mu.RLock()
	defer b.n.mu.RUnlock()
	v, ok := b.n.nombres[alias]
	return v, ok
}

func (b *httpAPIBackend) DeleteDNS(alias string) error {
	b.n.mu.Lock()
	defer b.n.mu.Unlock()
	if _, ok := b.n.nombres[alias]; !ok {
		return fmt.Errorf("alias no encontrado")
	}
	delete(b.n.nombres, alias)
	b.n.Auditoria("DNS_DELETED", alias)
	go b.n.PersistirLocamente()
	return nil
}

func (b *httpAPIBackend) EvalLisp(cmd string) (interface{}, error) {
	if b.n.lisp == nil {
		return nil, fmt.Errorf("lisp no inicializado")
	}
	return b.n.lisp.Eval(cmd)
}

func (b *httpAPIBackend) ConfigureNeural(cfg httpapi.NeuralConfig) (map[string]interface{}, error) {
	b.n.mu.Lock()
	defer b.n.mu.Unlock()
	if b.n.neuralState == nil {
		b.n.neuralState = &NeuralState{
			SpikeThreshold: 0.6, LeakRate: 0.01, NeuronType: "hidden",
			Synapses: make(map[string]SynapticWeight),
		}
	}
	if cfg.NeuronType != "" {
		b.n.neuralState.NeuronType = cfg.NeuronType
	}
	if cfg.SpikeThreshold > 0 && cfg.SpikeThreshold <= 1 {
		b.n.neuralState.SpikeThreshold = cfg.SpikeThreshold
	}
	if cfg.LeakRate > 0 && cfg.LeakRate <= 1 {
		b.n.neuralState.LeakRate = cfg.LeakRate
	}
	go b.n.persistirEstadoNeuronal()
	return map[string]interface{}{"status": "configured", "config": b.n.neuralState}, nil
}

func (b *httpAPIBackend) NeuralState() map[string]interface{} {
	b.n.mu.RLock()
	defer b.n.mu.RUnlock()
	if b.n.neuralState == nil {
		return map[string]interface{}{"status": "empty"}
	}
	return map[string]interface{}{"status": "ok", "state": b.n.neuralState}
}

func (b *httpAPIBackend) NeuralLearn(rate float64) map[string]interface{} {
	if rate <= 0 {
		rate = 0.01
	}
	b.n.mu.Lock()
	if b.n.neuralState != nil {
		for target, syn := range b.n.neuralState.Synapses {
			newWeight := syn.Weight + rate*(1-syn.Weight)
			if newWeight > 1 {
				newWeight = 1
			}
			syn.Weight = newWeight
			syn.SuccessfulFires++
			b.n.neuralState.Synapses[target] = syn
		}
	}
	b.n.mu.Unlock()
	go b.n.persistirEstadoNeuronal()
	return map[string]interface{}{"status": "learning_completed", "tasa": rate}
}

func (b *httpAPIBackend) NeuralInfer(input []float64) map[string]interface{} {
	if len(input) == 0 {
		input = []float64{0}
	}
	var sum float64
	for _, v := range input {
		sum += v
	}
	output := sum / float64(len(input))
	output = 1.0 / (1.0 + math.Exp(-output))
	nodeID := "local"
	if b.n.host != nil {
		nodeID = b.n.host.ID().String()
	}
	return map[string]interface{}{
		"status": "success", "output": []float64{output},
		"processed_by": nodeID, "process_time": time.Now().UnixNano(),
	}
}

func (b *httpAPIBackend) SearchMemory(query string, limit int) map[string]interface{} {
	results := b.n.buscarEnMemoriaLocal(query)
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return map[string]interface{}{"status": "ok", "results": results, "query": query}
}

func toAPIAgent(a *Agente) *httpapi.Agent {
	return &httpapi.Agent{
		ID: a.ID, RootCID: a.RootCID, UltimaActual: a.UltimaActual, BalanceUTXO: a.BalanceUTXO,
	}
}
