package sync

import (
	"context"
	"sync"
)

type Mode int

const (
	ModeQuick       Mode = 1
	ModeFull        Mode = 2
	ModeIncremental Mode = 3
)

type Config struct {
	Mode           Mode  `json:"mode"`
	LastSyncTime   int64 `json:"last_sync_time"`
	AutoSyncDays   int   `json:"auto_sync_days"`
	MaxQuickBlocks int   `json:"max_quick_blocks"`
}

type Progress struct {
	Current int     `json:"current"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
	Status  string  `json:"status"`
	Stage   string  `json:"stage"`
}

// Manager holds sync state. The concrete node pointer stays in package node;
// this type is the portable config/progress side.
type Manager struct {
	Config       Config
	IsSyncing    bool
	SyncProgress float64
	SyncCancel   context.CancelFunc
	Mu           sync.RWMutex
}

func NewProgress() *Progress {
	return &Progress{Status: "idle", Stage: "none"}
}
