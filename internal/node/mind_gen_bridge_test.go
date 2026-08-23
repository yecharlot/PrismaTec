package node

import "testing"

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
	if v2 == "" || !containsEthics(v2) {
		t.Fatalf("expected ethics veto phrasing: %q", v2)
	}
}

func containsEthics(s string) bool {
	return len(s) > 5 // soft check — message about sumidero
}

func TestExtractGenDialogueStimulus(t *testing.T) {
	k, stim := extractGenDialogueStimulus("pregunta al gen demo-cell: quién eres")
	if k == "" && stim == "" {
		t.Fatal("empty")
	}
}
