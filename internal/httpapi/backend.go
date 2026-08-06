package httpapi

import (
	"encoding/json"
	"io"
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

// NeuralConfig is used by IA configuration endpoints.
type NeuralConfig struct {
	NeuronType     string  `json:"neuron_type"`
	SpikeThreshold float64 `json:"spike_threshold"`
	LeakRate       float64 `json:"leak_rate"`
}

// Backend is the domain surface the HTTP layer needs from the node.
// HTTP decoding/encoding lives in this package; business logic stays in the node.
type Backend interface {
	// Agents
	CreateAgent() (*Agent, error)
	ListAgents() map[string]*Agent
	GetAgent(id string) (*Agent, bool)
	DeleteAgent(id string) error
	UpdateAgent(id string, rootCID string, balance *float64) (*Agent, error)

	// Blocks / IPFS-like store
	ListBlocks() []map[string]interface{}
	FetchBlock(cid string) ([]byte, error)
	PutBlock(cid string, data []byte) error
	DeleteBlock(cid string) error
	ClearBlocks() (int, error)

	// Network / DNS
	ListPeers() []map[string]interface{}
	ListDNS() map[string]string
	ResolveDNS(alias string) (string, bool)
	DeleteDNS(alias string) error

	// Lisp
	EvalLisp(cmd string) (interface{}, error)

	// Neural / IA
	ConfigureNeural(cfg NeuralConfig) (map[string]interface{}, error)
	NeuralState() map[string]interface{}
	NeuralLearn(rate float64) map[string]interface{}
	NeuralInfer(input []float64) map[string]interface{}
	SearchMemory(query string, limit int) map[string]interface{}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// MountCore registers domain-backed JSON routes on mux.
func MountCore(mux *http.ServeMux, b Backend) {
	mux.HandleFunc("/api/crear-agente", func(w http.ResponseWriter, r *http.Request) {
		ag, err := b.CreateAgent()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, ag)
	})

	mux.HandleFunc("/api/agentes/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/agentes/")
		if strings.HasSuffix(path, "/root") {
			id := strings.TrimSuffix(path, "/root")
			ag, ok := b.GetAgent(id)
			if !ok {
				http.Error(w, "Agente no encontrado", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"agent_id": id, "root_cid": ag.RootCID, "updated_at": ag.UltimaActual,
			})
			return
		}
		writeJSON(w, http.StatusOK, b.ListAgents())
	})

	mux.HandleFunc("/api/eliminar-agente", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			http.Error(w, "ID de agente requerido", http.StatusBadRequest)
			return
		}
		if err := b.DeleteAgent(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": req.ID})
	})

	mux.HandleFunc("/api/modificar-agente", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPut {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID      string   `json:"id"`
			RootCID string   `json:"root_cid"`
			Balance *float64 `json:"balance_utxo"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			http.Error(w, "JSON inválido o ID faltante", http.StatusBadRequest)
			return
		}
		ag, err := b.UpdateAgent(req.ID, req.RootCID, req.Balance)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, ag)
	})

	mux.HandleFunc("/api/ipfs/list", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, b.ListBlocks())
	})

	mux.HandleFunc("/api/ipfs/fetch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			CID string `json:"cid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		data, err := b.FetchBlock(req.CID)
		if err != nil {
			http.Error(w, "No encontrado", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cid": req.CID, "data": string(data), "size": len(data),
		})
	})

	mux.HandleFunc("/api/ipfs/get", func(w http.ResponseWriter, r *http.Request) {
		cid := r.URL.Query().Get("cid")
		if cid == "" {
			http.Error(w, "cid requerido", http.StatusBadRequest)
			return
		}
		data, err := b.FetchBlock(cid)
		if err != nil {
			http.Error(w, "No encontrado", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(data)
	})

	mux.HandleFunc("/api/ipfs/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error leyendo body", http.StatusBadRequest)
			return
		}
		cid := r.URL.Query().Get("cid")
		if err := b.PutBlock(cid, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "size": len(data), "cid": cid})
	})

	mux.HandleFunc("/api/ipfs/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			CID string `json:"cid"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.CID == "" {
			req.CID = r.URL.Query().Get("cid")
		}
		if req.CID == "" {
			http.Error(w, "cid requerido", http.StatusBadRequest)
			return
		}
		if err := b.DeleteBlock(req.CID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "cid": req.CID})
	})

	mux.HandleFunc("/api/ipfs/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Confirm string `json:"confirm"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Confirm != "YES" {
			http.Error(w, "Confirmación requerida: 'confirm': 'YES'", http.StatusBadRequest)
			return
		}
		n, err := b.ClearBlocks()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "cleared", "blocks_deleted": n})
	})

	mux.HandleFunc("/api/network/peers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, b.ListPeers())
	})

	mux.HandleFunc("/api/dns/list", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, b.ListDNS())
	})

	mux.HandleFunc("/api/dns/resolve", func(w http.ResponseWriter, r *http.Request) {
		alias := r.URL.Query().Get("alias")
		if alias == "" {
			var req struct {
				Alias string `json:"alias"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			alias = req.Alias
		}
		agent, ok := b.ResolveDNS(alias)
		if !ok {
			http.Error(w, "No encontrado", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"alias": alias, "agent_id": agent})
	})

	mux.HandleFunc("/api/dns/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Alias string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Alias == "" {
			http.Error(w, "alias requerido", http.StatusBadRequest)
			return
		}
		if err := b.DeleteDNS(req.Alias); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "alias": req.Alias})
	})

	mux.HandleFunc("/api/lispai", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Cmd string `json:"cmd"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		res, err := b.EvalLisp(req.Cmd)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"resultado": res})
	})

	mux.HandleFunc("/api/ia/configurar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var cfg NeuralConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		out, err := b.ConfigureNeural(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("/api/ia/estado", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, b.NeuralState())
	})

	mux.HandleFunc("/api/ia/aprender", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			TasaAprendizaje float64 `json:"tasa_aprendizaje"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, http.StatusOK, b.NeuralLearn(req.TasaAprendizaje))
	})

	mux.HandleFunc("/api/ia/inferir", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Entrada []float64 `json:"entrada"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, b.NeuralInfer(req.Entrada))
	})

	mux.HandleFunc("/api/ia/memoria/buscar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Consulta string `json:"consulta"`
			Limit    int    `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, b.SearchMemory(req.Consulta, req.Limit))
	})
}
