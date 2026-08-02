package persistence

import (
	"fmt"
	"os"
)

// NewFromEnv creates the appropriate Store implementation.
//
// Priority:
//  1. If SUPABASE_URL and SUPABASE_SERVICE_KEY are set → SupabaseStore
//  2. Otherwise → LocalStore under the given dataDir (default "alset_data")
func NewFromEnv(dataDir string) (Store, error) {
	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_SERVICE_KEY")

	if url != "" && key != "" {
		cfg := SupabaseConfig{
			URL:        url,
			ServiceKey: key,
			Table:      os.Getenv("SUPABASE_TABLE"), // optional, defaults to alset_kv
		}
		store, err := NewSupabaseStore(cfg)
		if err != nil {
			return nil, fmt.Errorf("persistence: supabase: %w", err)
		}
		fmt.Println("✅ Persistencia: Supabase")
		return store, nil
	}

	if dataDir == "" {
		dataDir = "alset_data"
	}
	store, err := NewLocalStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("persistence: local: %w", err)
	}
	fmt.Println("✅ Persistencia: Local (disco) — " + dataDir)
	return store, nil
}
