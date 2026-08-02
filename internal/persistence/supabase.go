package persistence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// SupabaseConfig holds the connection parameters.
// Fill these from environment variables or a secure config source.
type SupabaseConfig struct {
	URL        string // e.g. https://xxxx.supabase.co
	ServiceKey string // service_role key (preferred for server-side)
	// Table is the single table used to store key/value blobs.
	// Default: "alset_kv"
	Table string
}

// SupabaseStore implements Store using a simple key-value table in Supabase.
//
// Expected table schema (create it in the Supabase SQL editor):
//
//	create table if not exists alset_kv (
//	  key        text primary key,
//	  value      jsonb not null,
//	  updated_at timestamptz default now()
//	);
//
//	-- optional RLS policies / grants for the service_role
type SupabaseStore struct {
	cfg    SupabaseConfig
	client *http.Client
	mu     sync.Mutex
}

// NewSupabaseStore creates a ready-to-use Supabase backend.
// Returns an error if URL or ServiceKey are empty.
func NewSupabaseStore(cfg SupabaseConfig) (*SupabaseStore, error) {
	if cfg.URL == "" || cfg.ServiceKey == "" {
		return nil, fmt.Errorf("persistence/supabase: URL and ServiceKey are required")
	}
	if cfg.Table == "" {
		cfg.Table = "alset_kv"
	}
	return &SupabaseStore{
		cfg: cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func (s *SupabaseStore) endpoint() string {
	return fmt.Sprintf("%s/rest/v1/%s", s.cfg.URL, s.cfg.Table)
}

func (s *SupabaseStore) headers() http.Header {
	h := make(http.Header)
	h.Set("apikey", s.cfg.ServiceKey)
	h.Set("Authorization", "Bearer "+s.cfg.ServiceKey)
	h.Set("Content-Type", "application/json")
	h.Set("Prefer", "resolution=merge-duplicates") // upsert
	return h
}

// Save upserts the key/value pair.
func (s *SupabaseStore) Save(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// We store the raw bytes as a JSON string inside a jsonb column
	// so any binary-safe content works.
	payload := []map[string]interface{}{
		{
			"key":   key,
			"value": json.RawMessage(data), // will be stored as jsonb
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("persistence/supabase: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header = s.headers()
	// on_conflict for true upsert
	q := req.URL.Query()
	q.Set("on_conflict", "key")
	req.URL.RawQuery = q.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("persistence/supabase: save request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("persistence/supabase: save status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// Load fetches the value for a key. Returns (nil, nil) if not found.
func (s *SupabaseStore) Load(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	url := fmt.Sprintf("%s?key=eq.%s&select=value", s.endpoint(), key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = s.headers()
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("persistence/supabase: load request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("persistence/supabase: load status %d: %s", resp.StatusCode, string(b))
	}

	var rows []struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("persistence/supabase: decode: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return []byte(rows[0].Value), nil
}

func (s *SupabaseStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	url := fmt.Sprintf("%s?key=eq.%s", s.endpoint(), key)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header = s.headers()

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("persistence/supabase: delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("persistence/supabase: delete status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (s *SupabaseStore) Close() error { return nil }
