package persistence

import "context"

// Store defines the persistence contract used by the Alset node.
// Both local disk and Supabase implementations must satisfy this interface.
type Store interface {
	// Save stores arbitrary data under a logical key (e.g. "alset_state.json").
	Save(ctx context.Context, key string, data []byte) error

	// Load retrieves data by key. Returns nil, nil if the key does not exist.
	Load(ctx context.Context, key string) ([]byte, error)

	// Delete removes a key (optional for some backends).
	Delete(ctx context.Context, key string) error

	// Close releases any resources (connections, etc.).
	Close() error
}

// Keys used by the node. Centralising them avoids magic strings.
const (
	KeyState       = "alset_state.json"   // agents + core state
	KeyNames       = "alset_names.json"   // DNS / alias map
	KeyBlocks      = "blocks.json"        // blockstore
	KeyNeuralState = "neural_state.json"  // neural weights & state
)
