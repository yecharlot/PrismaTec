package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// LocalStore persists data as files on disk.
type LocalStore struct {
	dir string
	mu  sync.Mutex
}

func NewLocalStore(dir string) (*LocalStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("persistence/local: mkdir %s: %w", dir, err)
	}
	_ = os.MkdirAll(filepath.Join(dir, "agents"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "blocks"), 0o755)
	return &LocalStore{dir: dir}, nil
}

func (s *LocalStore) path(key string) string {
	return filepath.Join(s.dir, key)
}

func (s *LocalStore) readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

func (s *LocalStore) Save(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp := s.path(key) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(key))
}

func (s *LocalStore) Load(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readFile(s.path(key))
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *LocalStore) SaveAgents(_ context.Context, agents map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.dir, "agents")
	for id, data := range agents {
		if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o644); err != nil {
			return err
		}
	}
	all, _ := json.MarshalIndent(agents, "", "  ")
	return os.WriteFile(s.path(KeyState), all, 0o644)
}

func (s *LocalStore) LoadAgents(_ context.Context) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]byte)
	dir := filepath.Join(s.dir, "agents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if data, err2 := s.readFile(s.path(KeyState)); err2 == nil && data != nil {
			var m map[string]json.RawMessage
			if json.Unmarshal(data, &m) == nil {
				for k, v := range m {
					out[k] = []byte(v)
				}
			}
		}
		return out, nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		id := e.Name()
		if filepath.Ext(id) == ".json" {
			id = id[:len(id)-5]
		}
		out[id] = data
	}
	return out, nil
}

func (s *LocalStore) SaveBlock(_ context.Context, cid string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(filepath.Join(s.dir, "blocks", cid), data, 0o644)
}

func (s *LocalStore) SaveBlocks(ctx context.Context, blocks map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for cid, data := range blocks {
		if err := os.WriteFile(filepath.Join(s.dir, "blocks", cid), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s *LocalStore) LoadBlocks(_ context.Context) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]byte)
	dir := filepath.Join(s.dir, "blocks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out, nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err == nil {
			out[e.Name()] = data
		}
	}
	return out, nil
}

func (s *LocalStore) SaveNeuralState(_ context.Context, id string, state []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = "main"
	}
	return os.WriteFile(s.path("neural_"+id+".json"), state, 0o644)
}

func (s *LocalStore) LoadNeuralState(_ context.Context, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = "main"
	}
	data, err := s.readFile(s.path("neural_" + id + ".json"))
	if err != nil {
		return nil, err
	}
	if data != nil {
		return data, nil
	}
	// legacy key
	return s.readFile(s.path(KeyNeuralState))
}

func (s *LocalStore) Close() error { return nil }
