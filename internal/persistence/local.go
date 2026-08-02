package persistence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// LocalStore persists data as files on disk (the original behaviour).
type LocalStore struct {
	dir string
	mu  sync.Mutex
}

// NewLocalStore creates a store that writes under the given directory.
// The directory is created if it does not exist.
func NewLocalStore(dir string) (*LocalStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("persistence/local: mkdir %s: %w", dir, err)
	}
	return &LocalStore{dir: dir}, nil
}

func (s *LocalStore) path(key string) string {
	return filepath.Join(s.dir, key)
}

func (s *LocalStore) Save(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmp := s.path(key) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("persistence/local: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path(key)); err != nil {
		return fmt.Errorf("persistence/local: rename: %w", err)
	}
	return nil
}

func (s *LocalStore) Load(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path(key))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("persistence/local: read %s: %w", key, err)
	}
	return data, nil
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

func (s *LocalStore) Close() error { return nil }
