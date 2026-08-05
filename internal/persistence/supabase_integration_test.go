package persistence

import (
	"context"
	"os"
	"testing"
	"time"
)

// Runs only when SUPABASE_URL and SUPABASE_SERVICE_KEY are set.
func TestIntegration_SupabaseKVAndAgents(t *testing.T) {
	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_SERVICE_KEY")
	if key == "" {
		key = os.Getenv("SUPABASE_KEY")
	}
	if url == "" || key == "" {
		t.Skip("skip: set SUPABASE_URL and SUPABASE_SERVICE_KEY to run")
	}

	store, err := NewSupabaseStore(SupabaseConfig{URL: url, ServiceKey: key})
	if err != nil {
		t.Fatalf("NewSupabaseStore: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	testKey := "test_integration_kv.json"
	payload := []byte(`{"source":"integration-test","ts":1}`)
	if err := store.Save(ctx, testKey, payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load(ctx, testKey)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("Load = %q, want %q", got, payload)
	}
	_ = store.Delete(ctx, testKey)

	agentID := "test-agent-integration"
	agents := map[string][]byte{
		agentID: []byte(`{"id":"` + agentID + `","balance_utxo":0}`),
	}
	if err := store.SaveAgents(ctx, agents); err != nil {
		t.Fatalf("SaveAgents: %v", err)
	}
	loaded, err := store.LoadAgents(ctx)
	if err != nil {
		t.Fatalf("LoadAgents: %v", err)
	}
	if _, ok := loaded[agentID]; !ok {
		t.Fatalf("agent %s not found in %#v", agentID, loaded)
	}
}
