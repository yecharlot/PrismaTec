package persistence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type SupabaseConfig struct {
	URL        string
	ServiceKey string
	Table      string // KV table, default alset_kv
}

type SupabaseStore struct {
	cfg    SupabaseConfig
	client *http.Client
	mu     sync.Mutex
}

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
			Timeout: 20 * time.Second,
		},
	}, nil
}

func (s *SupabaseStore) headers() http.Header {
	h := make(http.Header)
	h.Set("apikey", s.cfg.ServiceKey)
	h.Set("Authorization", "Bearer "+s.cfg.ServiceKey)
	h.Set("Content-Type", "application/json")
	h.Set("Prefer", "resolution=merge-duplicates,return=minimal")
	return h
}

func (s *SupabaseStore) rest(table string) string {
	return fmt.Sprintf("%s/rest/v1/%s", s.cfg.URL, table)
}

func (s *SupabaseStore) do(req *http.Request) ([]byte, int, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

// ---------- KV ----------

func (s *SupabaseStore) Save(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload := []map[string]interface{}{
		{"key": key, "value": json.RawMessage(data)},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.rest(s.cfg.Table), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header = s.headers()
	q := req.URL.Query()
	q.Set("on_conflict", "key")
	req.URL.RawQuery = q.Encode()

	b, code, err := s.do(req)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("supabase kv save %d: %s", code, string(b))
	}
	return nil
}

func (s *SupabaseStore) Load(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := fmt.Sprintf("%s?key=eq.%s&select=value", s.rest(s.cfg.Table), url.QueryEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header = s.headers()
	req.Header.Set("Accept", "application/json")

	b, code, err := s.do(req)
	if err != nil {
		return nil, err
	}
	if code == 404 || code >= 300 {
		if code == 404 {
			return nil, nil
		}
		return nil, fmt.Errorf("supabase kv load %d: %s", code, string(b))
	}
	var rows []struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return []byte(rows[0].Value), nil
}

func (s *SupabaseStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := fmt.Sprintf("%s?key=eq.%s", s.rest(s.cfg.Table), url.QueryEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	req.Header = s.headers()
	b, code, err := s.do(req)
	if err != nil {
		return err
	}
	if code >= 300 && code != 404 {
		return fmt.Errorf("supabase kv delete %d: %s", code, string(b))
	}
	return nil
}

// ---------- Agents (alset_agents) ----------

func (s *SupabaseStore) SaveAgents(ctx context.Context, agents map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(agents) == 0 {
		return nil
	}
	rows := make([]map[string]interface{}, 0, len(agents))
	for id, data := range agents {
		rows = append(rows, map[string]interface{}{
			"id":   id,
			"data": json.RawMessage(data),
		})
	}
	body, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.rest("alset_agents"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header = s.headers()
	q := req.URL.Query()
	q.Set("on_conflict", "id")
	req.URL.RawQuery = q.Encode()

	b, code, err := s.do(req)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("supabase agents save %d: %s", code, string(b))
	}
	return nil
}

func (s *SupabaseStore) LoadAgents(ctx context.Context) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.rest("alset_agents") + "?select=id,data"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header = s.headers()
	req.Header.Set("Accept", "application/json")

	b, code, err := s.do(req)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return nil, fmt.Errorf("supabase agents load %d: %s", code, string(b))
	}
	var rows []struct {
		ID   string          `json:"id"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(rows))
	for _, r := range rows {
		out[r.ID] = []byte(r.Data)
	}
	return out, nil
}

// ---------- Blocks (alset_blocks) ----------

func (s *SupabaseStore) SaveBlock(ctx context.Context, cid string, data []byte) error {
	return s.SaveBlocks(ctx, map[string][]byte{cid: data})
}

func (s *SupabaseStore) SaveBlocks(ctx context.Context, blocks map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(blocks) == 0 {
		return nil
	}
	rows := make([]map[string]interface{}, 0, len(blocks))
	for cid, data := range blocks {
		rows = append(rows, map[string]interface{}{
			"cid":  cid,
			"data": base64.StdEncoding.EncodeToString(data),
			"size": len(data),
		})
	}
	body, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.rest("alset_blocks"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header = s.headers()
	q := req.URL.Query()
	q.Set("on_conflict", "cid")
	req.URL.RawQuery = q.Encode()

	b, code, err := s.do(req)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("supabase blocks save %d: %s", code, string(b))
	}
	return nil
}

func (s *SupabaseStore) LoadBlocks(ctx context.Context) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.rest("alset_blocks") + "?select=cid,data"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header = s.headers()
	req.Header.Set("Accept", "application/json")

	b, code, err := s.do(req)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return nil, fmt.Errorf("supabase blocks load %d: %s", code, string(b))
	}
	var rows []struct {
		CID  string `json:"cid"`
		Data string `json:"data"` // base64
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(rows))
	for _, r := range rows {
		raw, err := base64.StdEncoding.DecodeString(r.Data)
		if err != nil {
			// try raw bytes if not base64
			out[r.CID] = []byte(r.Data)
			continue
		}
		out[r.CID] = raw
	}
	return out, nil
}

// ---------- Neural (alset_neural_state) ----------

func (s *SupabaseStore) SaveNeuralState(ctx context.Context, id string, state []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		id = "main"
	}
	payload := []map[string]interface{}{
		{"id": id, "state": json.RawMessage(state)},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.rest("alset_neural_state"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header = s.headers()
	q := req.URL.Query()
	q.Set("on_conflict", "id")
	req.URL.RawQuery = q.Encode()

	b, code, err := s.do(req)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("supabase neural save %d: %s", code, string(b))
	}
	return nil
}

func (s *SupabaseStore) LoadNeuralState(ctx context.Context, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		id = "main"
	}
	u := fmt.Sprintf("%s?id=eq.%s&select=state", s.rest("alset_neural_state"), url.QueryEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header = s.headers()
	req.Header.Set("Accept", "application/json")

	b, code, err := s.do(req)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return nil, fmt.Errorf("supabase neural load %d: %s", code, string(b))
	}
	var rows []struct {
		State json.RawMessage `json:"state"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return []byte(rows[0].State), nil
}

func (s *SupabaseStore) Close() error { return nil }
