package persistence

import (
	"context"
	"encoding/json"
	"testing"
)

// End-to-end style flow on local disk: names + agents + blocks together.
func TestIntegration_LocalFullStateCycle(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	agents := map[string][]byte{"x1": []byte(`{"id":"x1"}`)}
	if err := store.SaveAgents(ctx, agents); err != nil {
		t.Fatal(err)
	}
	names, _ := json.Marshal(map[string]string{"app.ans": "x1"})
	if err := store.Save(ctx, KeyNames, names); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlocks(ctx, map[string][]byte{"cid-x": []byte("<html/>")}); err != nil {
		t.Fatal(err)
	}
	neural, _ := json.Marshal(map[string]float64{"membrane_potential": 0.2})
	if err := store.SaveNeuralState(ctx, "main", neural); err != nil {
		t.Fatal(err)
	}

	// reopen-style: new store same dir is already the same instance;
	// verify loads
	a, err := store.LoadAgents(ctx)
	if err != nil || len(a) != 1 {
		t.Fatalf("agents: %v %#v", err, a)
	}
	n, err := store.Load(ctx, KeyNames)
	if err != nil || len(n) == 0 {
		t.Fatalf("names: %v", err)
	}
	b, err := store.LoadBlocks(ctx)
	if err != nil || string(b["cid-x"]) != "<html/>" {
		t.Fatalf("blocks: %v %#v", err, b)
	}
	ns, err := store.LoadNeuralState(ctx, "main")
	if err != nil || len(ns) == 0 {
		t.Fatalf("neural: %v", err)
	}
}
