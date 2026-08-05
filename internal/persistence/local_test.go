package persistence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStore_SaveLoadDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	key := "hello.json"
	payload := []byte(`{"ok":true}`)

	if err := store.Save(ctx, key, payload); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("Load = %q, want %q", got, payload)
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err = store.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after delete, got %q", got)
	}
}

func TestLocalStore_LoadMissingReturnsNil(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(context.Background(), "no-existe.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got %q", got)
	}
}

func TestLocalStore_AgentsRoundTrip(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	in := map[string][]byte{
		"a1": []byte(`{"id":"a1"}`),
		"a2": []byte(`{"id":"a2"}`),
	}
	if err := store.SaveAgents(ctx, in); err != nil {
		t.Fatalf("SaveAgents: %v", err)
	}
	out, err := store.LoadAgents(ctx)
	if err != nil {
		t.Fatalf("LoadAgents: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if string(out["a1"]) != string(in["a1"]) {
		t.Fatalf("a1 mismatch: %q", out["a1"])
	}
}

func TestLocalStore_BlocksRoundTrip(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	blocks := map[string][]byte{
		"cid-1": []byte("hola"),
		"cid-2": []byte("mundo"),
	}
	if err := store.SaveBlocks(ctx, blocks); err != nil {
		t.Fatalf("SaveBlocks: %v", err)
	}
	out, err := store.LoadBlocks(ctx)
	if err != nil {
		t.Fatalf("LoadBlocks: %v", err)
	}
	if len(out) != 2 || string(out["cid-1"]) != "hola" {
		t.Fatalf("unexpected blocks: %#v", out)
	}
}

func TestLocalStore_NeuralStateRoundTrip(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	state := []byte(`{"membrane_potential":0.5}`)
	if err := store.SaveNeuralState(ctx, "main", state); err != nil {
		t.Fatalf("SaveNeuralState: %v", err)
	}
	got, err := store.LoadNeuralState(ctx, "main")
	if err != nil {
		t.Fatalf("LoadNeuralState: %v", err)
	}
	if string(got) != string(state) {
		t.Fatalf("got %q, want %q", got, state)
	}
}

func TestLocalStore_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "data")
	store, err := NewLocalStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("directory not created: %v", err)
	}
	_ = store
}
