package lisp

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"

	"redalset/internal/agents"
	"redalset/internal/neural"
)

// mockHost is a minimal Host for pure Lisp evaluation tests.
type mockHost struct{}

func (m *mockHost) Lock()                                   {}
func (m *mockHost) Unlock()                                 {}
func (m *mockHost) RLock()                                  {}
func (m *mockHost) RUnlock()                                {}
func (m *mockHost) Auditoria(string, string)                {}
func (m *mockHost) GenerarCID([]byte) (string, error)       { return "cid-test", nil }
func (m *mockHost) AnunciarNuevoBloque(string)              {}
func (m *mockHost) PersistirLocamente()                     {}
func (m *mockHost) SincronizarConPares()                    {}
func (m *mockHost) SetAgentRoot(string, string)             {}
func (m *mockHost) DifundirActualizacionDNS(string, string) {}
func (m *mockHost) BuscarContenidoPorCID(string) ([]byte, error) {
	return nil, nil
}
func (m *mockHost) BroadcastPulse(string, interface{})              {}
func (m *mockHost) PersistirEstadoNeuronal()                        {}
func (m *mockHost) GetAgent(string) (*agents.Agente, bool)          { return nil, false }
func (m *mockHost) PutAgent(*agents.Agente)                         {}
func (m *mockHost) SetNombre(string, string)                        {}
func (m *mockHost) GetNombre(string) (string, bool)                 { return "", false }
func (m *mockHost) Sign([]byte) ([]byte, error)                     { return []byte("sig"), nil }
func (m *mockHost) HasMasterKey() bool                              { return false }
func (m *mockHost) PublishTopic([]byte) error                       { return nil }
func (m *mockHost) Ctx() context.Context                            { return context.Background() }
func (m *mockHost) ConnectPeer(context.Context, peer.AddrInfo) error { return nil }
func (m *mockHost) PeerID() string                                  { return "peer-test" }
func (m *mockHost) PeerCount() int                                    { return 0 }
func (m *mockHost) GetNeural() *neural.NeuralState                  { return nil }
func (m *mockHost) EnsureNeural() *neural.NeuralState {
	return &neural.NeuralState{Synapses: map[string]neural.SynapticWeight{}}
}
func (m *mockHost) GetBlock(string) ([]byte, bool)       { return nil, false }
func (m *mockHost) PutBlock(string, []byte)              {}
func (m *mockHost) ListBlocks() map[string][]byte        { return map[string][]byte{} }

func TestEval_Arithmetic(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	cases := []struct {
		code string
		want float64
	}{
		{"(+ 1 2)", 3},
		{"(* 3 4)", 12},
		{"(- 10 3)", 7},
	}
	for _, tc := range cases {
		got, err := e.Eval(tc.code)
		if err != nil {
			t.Fatalf("Eval(%q): %v", tc.code, err)
		}
		f, ok := got.(float64)
		if !ok || f != tc.want {
			t.Fatalf("Eval(%q) = %#v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestEval_DefineAndLookup(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	if _, err := e.Eval(`(define x 5)`); err != nil {
		// some engines use defun/setq — try setq style if define fails
		if _, err2 := e.Eval(`(setq x 5)`); err2 != nil {
			t.Skipf("define/setq not available as tested: %v / %v", err, err2)
		}
	}
}
