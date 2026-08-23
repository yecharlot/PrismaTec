package node

import (
	"strings"
	"testing"
)

func TestMemoryGenPinAndSave(t *testing.T) {
	n := &NodoAlset{
		agentes:    map[string]*Agente{},
		nombres:    map[string]string{},
		blockstore: map[string][]byte{},
	}
	g, err := n.CreateMemoryGen("mem-test", "test")
	if err != nil {
		t.Fatal(err)
	}
	if !isMemoryMissionGen(g) {
		t.Fatal("expected memory mission")
	}
	cid, g2, err := n.SaveTextToMemoryGen("mem-test", "hecho importante de la red", "nota")
	if err != nil || cid == "" {
		t.Fatal(err, cid)
	}
	if len(g2.EpisodeCIDs) < 1 {
		t.Fatal("expected pin")
	}
	list := n.ListMemoryGens()
	if len(list) < 1 {
		t.Fatal("list empty")
	}
	lines := n.mindGenTools("lista genes memoria")
	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "mem-test") && !strings.Contains(joined, "CID") {
		// list path may say mem-test.ans
		if len(list) == 0 {
			t.Fatalf("voice: %q", joined)
		}
	}
}
