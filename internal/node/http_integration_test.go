package node

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"redalset/internal/persistence"
)

// testNode builds a minimal node suitable for HTTP integration tests.
// It does not start libp2p or background network loops.
func testNode(t *testing.T) *NodoAlset {
	t.Helper()
	dir := t.TempDir()
	// keep static dir writable for ensureStaticFiles
	staticDir := filepath.Join(dir, "static")
	if err := os.MkdirAll(filepath.Join(staticDir, "apps"), 0o755); err != nil {
		t.Fatal(err)
	}
	// work from temp so ensureStaticFiles writes under temp when possible
	// StaticDir is a package const "static" — create it in cwd is undesirable.
	// ensureStaticFiles may write to package StaticDir; we tolerate that.

	store, err := persistence.NewLocalStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}

	n := &NodoAlset{
		ctx:                  context.Background(),
		agentes:              make(map[string]*Agente),
		nombres:              make(map[string]string),
		blockstore:           make(map[string][]byte),
		pendingInferences:    make(map[string]chan InferenceResponse),
		pendingMemoryQueries: make(map[string]chan MemoryResponse),
		hebbianMemory:        make(map[string]float64),
		pulseSubscribers:     make(map[*SSESubscriber]bool),
		pulseClients:         make(map[string]*PulseClient),
		store:                store,
		startTime:            time.Now().Unix(),
	}
	return n
}

func TestIntegration_CrearYListarAgente(t *testing.T) {
	n := testNode(t)
	h := n.buildHTTPHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/crear-agente", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("crear-agente status = %d body=%s", rec.Code, rec.Body.String())
	}

	var created Agente
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	if created.ID == "" {
		t.Fatal("expected non-empty agent id")
	}

	// list
	req = httptest.NewRequest(http.MethodGet, "/api/agentes/", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var agents map[string]*Agente
	if err := json.Unmarshal(rec.Body.Bytes(), &agents); err != nil {
		t.Fatalf("list json: %v", err)
	}
	if _, ok := agents[created.ID]; !ok {
		t.Fatalf("agent %s not in list: %#v", created.ID, agents)
	}

	// persisted to local store
	blobs, err := n.store.LoadAgents(context.Background())
	if err != nil {
		t.Fatalf("LoadAgents: %v", err)
	}
	if len(blobs) == 0 {
		t.Fatal("expected agents persisted to store")
	}
}

func TestIntegration_IPFSListEmpty(t *testing.T) {
	n := testNode(t)
	h := n.buildHTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/ipfs/list", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestIntegration_NetworkPeers(t *testing.T) {
	n := testNode(t)
	h := n.buildHTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/network/peers", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// without libp2p host may error or return empty — accept 200
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIntegration_PersistReloadAgents(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.NewLocalStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}

	n1 := &NodoAlset{
		ctx:        context.Background(),
		agentes:    make(map[string]*Agente),
		nombres:    make(map[string]string),
		blockstore: make(map[string][]byte),
		store:      store,
	}
	n1.agentes["abc123"] = &Agente{ID: "abc123", BalanceUTXO: 10}
	n1.PersistirLocamente()

	n2 := &NodoAlset{
		ctx:        context.Background(),
		agentes:    make(map[string]*Agente),
		nombres:    make(map[string]string),
		blockstore: make(map[string][]byte),
		store:      store,
	}
	n2.CargarEstado()
	if n2.agentes["abc123"] == nil || n2.agentes["abc123"].BalanceUTXO != 10 {
		t.Fatalf("reload failed: %#v", n2.agentes)
	}
}
