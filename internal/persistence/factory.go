package persistence

import (
	"fmt"
	"os"
)

// Default Supabase credentials (can be overridden by environment variables).
// Prefer setting SUPABASE_URL and SUPABASE_SERVICE_KEY / SUPABASE_KEY in production.
const (
	defaultSupabaseURL = "https://uysvbxawytsegxcufdds.supabase.co"
	defaultSupabaseKey = "sb_publishable_t9lx4M-VCqNvCaEOHGYzxQ_RHEcqm4Q"
)

// NewFromEnv creates the appropriate Store implementation.
//
// Priority:
//  1. SUPABASE_URL + (SUPABASE_SERVICE_KEY or SUPABASE_KEY) from environment
//  2. Built-in defaults (the project Supabase instance)
//  3. Fallback to LocalStore under dataDir
func NewFromEnv(dataDir string) (Store, error) {
	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_SERVICE_KEY")
	if key == "" {
		key = os.Getenv("SUPABASE_KEY")
	}

	// Use project defaults when env is empty
	if url == "" {
		url = defaultSupabaseURL
	}
	if key == "" {
		key = defaultSupabaseKey
	}

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
		fmt.Println("✅ Persistencia: Supabase →", url)
		return store, nil
	}

	if dataDir == "" {
		dataDir = "alset_data"
	}
	store, err := NewLocalStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("persistence: local: %w", err)
	}
	fmt.Println("✅ Persistencia: Local (disco) —", dataDir)
	return store, nil
}
