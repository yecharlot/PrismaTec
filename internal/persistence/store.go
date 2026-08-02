package persistence

import "context"

// Store is the persistence contract used by the Alset node.
type Store interface {
	// --- Key/value (generic blobs, e.g. names map) ---
	Save(ctx context.Context, key string, data []byte) error
	Load(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error

	// --- Structured tables ---
	SaveAgents(ctx context.Context, agents map[string][]byte) error // id -> JSON
	LoadAgents(ctx context.Context) (map[string][]byte, error)

	SaveBlock(ctx context.Context, cid string, data []byte) error
	SaveBlocks(ctx context.Context, blocks map[string][]byte) error
	LoadBlocks(ctx context.Context) (map[string][]byte, error)

	SaveNeuralState(ctx context.Context, id string, state []byte) error
	LoadNeuralState(ctx context.Context, id string) ([]byte, error)

	Close() error
}

// Logical keys still used for leftover KV data (names, etc.).
const (
	KeyState       = "alset_state.json"
	KeyNames       = "alset_names.json"
	KeyBlocks      = "blocks.json"
	KeyNeuralState = "neural_state.json"
)
