package node

import "testing"

func TestDeleteAndReturnGen(t *testing.T) {
	n := &NodoAlset{agentes: map[string]*Agente{}, nombres: map[string]string{}, blockstore: map[string][]byte{}}
	g, err := n.CreateAlsetGen("probe-x", "", "seed", "test")
	if err != nil {
		t.Fatal(err)
	}
	n.mu.Lock()
	g.State.Location = "frontier:https://example.com"
	g.State.Metadata = map[string]interface{}{"remote_http": "https://edge.example/g/probe-x"}
	n.mu.Unlock()
	snap, err := n.ReturnGenHome("probe-x")
	if err != nil || snap.Location == "frontier:https://example.com" {
		t.Fatalf("return: %v %#v", err, snap)
	}
	if err := n.DeleteAlsetGen("probe-x"); err != nil {
		t.Fatal(err)
	}
	if err := n.DeleteAlsetGen("probe-x"); err == nil {
		t.Fatal("expected not found")
	}
}
