package persistence

import (
	"fmt"
	"os"
)

// NewFromEnv creates the appropriate Store implementation.
//
// Priority:
//  1. SUPABASE_URL + (SUPABASE_SERVICE_KEY or SUPABASE_KEY) from environment → Supabase
//  2. Fallback to LocalStore under dataDir
//
// Configure in Render / local:
//
//	export SUPABASE_URL="https://uysvbxawytsegxcufdds.supabase.co"
//	export SUPABASE_SERVICE_KEY="sb_secret_..."   # or the classic service_role JWT
//	export SUPABASE_TABLE="alset_kv"              # optional
func NewFromEnv(dataDir string) (Store, error) {
	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_SERVICE_KEY")
	if key == "" {
		key = os.Getenv("SUPABASE_KEY")
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
	fmt.Println("   (Define SUPABASE_URL + SUPABASE_SERVICE_KEY para usar Supabase)")
	return store, nil
}
