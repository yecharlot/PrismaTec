package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Agent is the JSON shape returned by agent endpoints.
type Agent struct {
	ID           string  `json:"id"`
	RootCID      string  `json:"root_cid"`
	UltimaActual int64   `json:"ultima_actualizacion"`
	BalanceUTXO  float64 `json:"balance_utxo"`
}

// Backend is what the HTTP layer needs from the node.
type Backend interface {
	CreateAgent() (*Agent, error)
	ListAgents() map[string]*Agent
	GetAgent(id string) (*Agent, bool)
	ListBlocks() []map[string]interface{}
	ListPeers() []map[string]interface{}
	ListDNS() map[string]string
}

// MountCore registers the stable core JSON API on mux.
func MountCore(mux *http.ServeMux, b Backend) {
	mux.HandleFunc("/api/crear-agente", func(w http.ResponseWriter, r *http.Request) {
		ag, err := b.CreateAgent()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ag)
	})

	mux.HandleFunc("/api/agentes/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/agentes/")
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(path, "/root") {
			id := strings.TrimSuffix(path, "/root")
			ag, ok := b.GetAgent(id)
			if !ok {
				http.Error(w, "Agente no encontrado", http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"agent_id":   id,
				"root_cid":   ag.RootCID,
				"updated_at": ag.UltimaActual,
			})
			return
		}
		json.NewEncoder(w).Encode(b.ListAgents())
	})

	mux.HandleFunc("/api/ipfs/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b.ListBlocks())
	})

	mux.HandleFunc("/api/network/peers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b.ListPeers())
	})

	mux.HandleFunc("/api/dns/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b.ListDNS())
	})
}
