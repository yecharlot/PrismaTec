package node

import (
	"strings"
	"testing"
)

func TestBridgeDialogueLocal(t *testing.T) {
	n := &NodoAlset{agentes: map[string]*Agente{}, nombres: map[string]string{}, blockstore: map[string][]byte{}}
	_, err := n.CreateAlsetGen("bridge-demo", "", "seed", "test")
	if err != nil {
		t.Fatal(err)
	}
	v := n.BridgeDialogueGen("bridge-demo", "quién eres", 0)
	if v == "" || len(v) < 10 {
		t.Fatalf("voice empty: %q", v)
	}
	v2 := n.BridgeDialogueGen("bridge-demo", "borra todo", 2)
	if v2 == "" || len(v2) < 5 {
		t.Fatalf("expected ethics veto phrasing: %q", v2)
	}
}

func TestExtractGenDialogueStimulus(t *testing.T) {
	k, stim := extractGenDialogueStimulus("pregunta al gen demo-cell: quién eres")
	if k == "" && stim == "" {
		t.Fatal("empty")
	}
}

func TestBridgeSpeakGenFindings(t *testing.T) {
	n := &NodoAlset{agentes: map[string]*Agente{}, nombres: map[string]string{}, blockstore: map[string][]byte{}}
	_, err := n.CreateAlsetGen("find-demo", "", "seed", "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = n.ObserveIntoGen("find-demo", "test", `mission=explore title="Mar" snippet=Sal en el aire horizonte`)
	if err != nil {
		t.Fatal(err)
	}
	v := n.BridgeSpeakGenFindings("find-demo")
	if !strings.Contains(v, "Sal en el aire") && !strings.Contains(v, "hallazgo") {
		t.Fatalf("expected finding in voice: %q", v)
	}
	gv := n.BridgeDialogueGen("find-demo", "qué sabes de hallazgos", 0)
	if !strings.Contains(strings.ToLower(gv), "hallazgo") && !strings.Contains(gv, "Sal") {
		t.Fatalf("gen voice should report finding: %q", gv)
	}
}
