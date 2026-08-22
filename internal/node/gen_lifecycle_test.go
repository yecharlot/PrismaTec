package node

import (
	"strings"
	"testing"

	"redalset/internal/agents"
)

func TestAlsetGenCreateMutateConsult(t *testing.T) {
	n := &NodoAlset{
		agentes:    make(map[string]*Agente),
		gens:       make(map[string]*agents.AlsetGen),
		nombres:    make(map[string]string),
		blockstore: make(map[string][]byte),
	}

	g, err := n.CreateAlsetGen("alfa", "", "seed", "first cell")
	if err != nil {
		t.Fatal(err)
	}
	if g.Key != "alfa.ans" {
		t.Fatalf("key=%s", g.Key)
	}
	if g.CurrentRootCID == "" || g.CurrentRootCID == "bafk-seed-pending" {
		// GenerarCID should produce a real cid when blockstore works
		t.Logf("root cid=%s", g.CurrentRootCID)
	}
	g2, err := n.MutateAlsetGen("alfa", "bafk-new-form", "mind-auth")
	if err != nil {
		t.Fatal(err)
	}
	if g2.CurrentRootCID != "bafk-new-form" {
		t.Fatalf("mutate root=%s", g2.CurrentRootCID)
	}
	if len(g2.History) < 1 {
		t.Fatal("history empty")
	}
	snap := n.ConsultAlsetGen("alfa", "quién eres")
	if snap["ok"] != true {
		t.Fatalf("consult %#v", snap)
	}
	voice, _ := snap["voice"].(string)
	if !strings.Contains(voice, "alfa.ans") {
		t.Fatalf("voice=%s", voice)
	}
	if _, err := n.TravelAlsetGen("alfa", "peer-test-1"); err != nil {
		t.Fatal(err)
	}
	list := n.listGens()
	if len(list) != 1 {
		t.Fatalf("list=%d", len(list))
	}
	// ethics refuse
	snap2 := n.ConsultAlsetGen("alfa", "borra todo")
	v2, _ := snap2["voice"].(string)
	if !strings.Contains(strings.ToLower(v2), "riesgo") && !strings.Contains(strings.ToLower(v2), "no actúo") && !strings.Contains(strings.ToLower(v2), "no actuo") {
		t.Fatalf("ethics voice=%s", v2)
	}
}
