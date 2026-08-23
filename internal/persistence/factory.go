package persistence

import (
	"fmt"
	"os"
)

// NewFromEnv creates the appropriate Store implementation.
//
// Priority:
//  1. ALSET_CF_STORE_URL or ALSET_CLOUDFLARE_NETWORK (+ optional STORE secret) → Cloudflare DO
//  2. SUPABASE_URL + key → Supabase
//  3. LocalStore under dataDir
func NewFromEnv(dataDir string) (Store, error) {
	if os.Getenv("ALSET_CF_STORE_URL") != "" || os.Getenv("ALSET_PERSIST") == "cloudflare" {
		if os.Getenv("ALSET_CF_STORE_URL") != "" || os.Getenv("ALSET_CLOUDFLARE_NETWORK") != "" {
			store, err := NewCloudflareStoreFromEnv()
			if err == nil {
				base := os.Getenv("ALSET_CF_STORE_URL")
				if base == "" {
					base = os.Getenv("ALSET_CLOUDFLARE_NETWORK")
				}
				fmt.Println("✅ Persistencia: Cloudflare Durable Object →", base)
				return store, nil
			}
			fmt.Println("⚠️ Cloudflare store:", err, "— intentando siguiente backend")
		}
	}

	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_SERVICE_KEY")
	if key == "" {
		key = os.Getenv("SUPABASE_KEY")
	}

	if url != "" && key != "" {
		cfg := SupabaseConfig{
			URL:        url,
			ServiceKey: key,
			Table:      os.Getenv("SUPABASE_TABLE"),
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
	fmt.Println("   (Define ALSET_CF_STORE_URL o SUPABASE_URL+KEY para store durable)")
	return store, nil
}
