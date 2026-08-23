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
	"os"
	"strings"
	"sync"
	"time"
)

// CloudflareStore persists KV and content blocks via Alset Network Worker + Durable Object.
// Env: ALSET_CF_STORE_URL (e.g. https://alset-network….workers.dev)
//      ALSET_CF_STORE_SECRET (optional, matches Worker STORE_SECRET)
type CloudflareStore struct {
	base   string
	secret string
	client *http.Client
	mu     sync.Mutex
}

// NewCloudflareStore from env or explicit base URL.
func NewCloudflareStore(baseURL, secret string) (*CloudflareStore, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("cloudflare store: empty base URL")
	}
	return &CloudflareStore{
		base:   baseURL,
		secret: secret,
		client: &http.Client{Timeout: 12 * time.Second},
	}, nil
}

func (s *CloudflareStore) headers() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "application/json")
	if s.secret != "" {
		h.Set("X-Alset-Store-Secret", s.secret)
	}
	return h
}

func (s *CloudflareStore) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header = s.headers()
	res, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	return b, res.StatusCode, nil
}

func (s *CloudflareStore) Save(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := url.Values{"key": {key}}
	_, code, err := s.do(ctx, http.MethodPut, "/api/store/kv?"+q.Encode(), map[string]string{
		"data": base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("cf store kv save %d", code)
	}
	return nil
}

func (s *CloudflareStore) Load(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := url.Values{"key": {key}}
	b, code, err := s.do(ctx, http.MethodGet, "/api/store/kv?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if code == 404 {
		return nil, fmt.Errorf("not found")
	}
	if code >= 300 {
		return nil, fmt.Errorf("cf store kv load %d", code)
	}
	var out struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out.Data)
}

func (s *CloudflareStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := url.Values{"key": {key}}
	_, code, err := s.do(ctx, http.MethodDelete, "/api/store/kv?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	if code >= 300 && code != 404 {
		return fmt.Errorf("cf store kv delete %d", code)
	}
	return nil
}

func (s *CloudflareStore) SaveAgents(ctx context.Context, agents map[string][]byte) error {
	raw, err := json.Marshal(agents)
	if err != nil {
		return err
	}
	return s.Save(ctx, "alset_agents_blob", raw)
}

func (s *CloudflareStore) LoadAgents(ctx context.Context) (map[string][]byte, error) {
	b, err := s.Load(ctx, "alset_agents_blob")
	if err != nil {
		return map[string][]byte{}, nil
	}
	out := map[string][]byte{}
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func (s *CloudflareStore) SaveBlock(ctx context.Context, cid string, data []byte) error {
	return s.SaveBlocks(ctx, map[string][]byte{cid: data})
}

func (s *CloudflareStore) SaveBlocks(ctx context.Context, blocks map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(blocks) == 0 {
		return nil
	}
	payload := map[string]string{}
	for cid, data := range blocks {
		payload[cid] = base64.StdEncoding.EncodeToString(data)
	}
	_, code, err := s.do(ctx, http.MethodPost, "/api/store/blocks", map[string]interface{}{"blocks": payload})
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("cf store blocks save %d", code)
	}
	return nil
}

func (s *CloudflareStore) LoadBlocks(ctx context.Context) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, code, err := s.do(ctx, http.MethodGet, "/api/store/blocks", nil)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return nil, fmt.Errorf("cf store blocks load %d", code)
	}
	var out struct {
		Blocks map[string]string `json:"blocks"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	res := make(map[string][]byte, len(out.Blocks))
	for cid, b64 := range out.Blocks {
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			res[cid] = []byte(b64)
			continue
		}
		res[cid] = raw
	}
	return res, nil
}

func (s *CloudflareStore) SaveNeuralState(ctx context.Context, id string, state []byte) error {
	if id == "" {
		id = "main"
	}
	return s.Save(ctx, "neural:"+id, state)
}

func (s *CloudflareStore) LoadNeuralState(ctx context.Context, id string) ([]byte, error) {
	if id == "" {
		id = "main"
	}
	return s.Load(ctx, "neural:"+id)
}

func (s *CloudflareStore) Close() error { return nil }

// NewCloudflareStoreFromEnv if ALSET_CF_STORE_URL is set.
func NewCloudflareStoreFromEnv() (*CloudflareStore, error) {
	u := os.Getenv("ALSET_CF_STORE_URL")
	if u == "" {
		u = os.Getenv("ALSET_CLOUDFLARE_NETWORK") // same worker host is fine
	}
	if u == "" {
		return nil, fmt.Errorf("ALSET_CF_STORE_URL not set")
	}
	secret := os.Getenv("ALSET_CF_STORE_SECRET")
	if secret == "" {
		secret = os.Getenv("STORE_SECRET")
	}
	return NewCloudflareStore(u, secret)
}
