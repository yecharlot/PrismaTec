package node

import (
	"context"
	"sync"

	"redalset/internal/agents"
	"redalset/internal/neural"
	"redalset/internal/poh"
)

// Domain aliases
type Agente = agents.Agente
type Modulo = agents.Modulo
type EntidadProgramatica = agents.EntidadProgramatica
type RelacionEntidad = agents.RelacionEntidad
type TokenAlset = agents.TokenAlset
type UsuarioRoles = agents.UsuarioRoles

type SynapticWeight = neural.SynapticWeight
type NeuralState = neural.NeuralState
type InferenceRequest = neural.InferenceRequest
type InferenceResponse = neural.InferenceResponse
type MemoryQuery = neural.MemoryQuery
type MemoryResponse = neural.MemoryResponse

type PoHEvent = poh.Event
type HumanityProof = poh.Proof

type NodoConfig struct {
	AdminPassHash string `json:"admin_pass_hash"`
	LastUpdate    int64  `json:"last_update"`
	Version       string `json:"version"`
	AdminPanelCID string `json:"admin_panel_cid"`
	IsGenesis     bool   `json:"is_genesis"`
}

type SyncMode int

const (
	SyncModeQuick       SyncMode = 1
	SyncModeFull        SyncMode = 2
	SyncModeIncremental SyncMode = 3
)

type SyncConfig struct {
	Mode           SyncMode `json:"mode"`
	LastSyncTime   int64    `json:"last_sync_time"`
	AutoSyncDays   int      `json:"auto_sync_days"`
	MaxQuickBlocks int      `json:"max_quick_blocks"`
}

type SyncManager struct {
	nodo         *NodoAlset
	config       SyncConfig
	isSyncing    bool
	syncProgress float64
	syncCancel   context.CancelFunc
	mu           sync.RWMutex
}

type SyncProgress struct {
	Current int     `json:"current"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
	Status  string  `json:"status"`
	Stage   string  `json:"stage"`
}

var globalSyncProgress = &SyncProgress{
	Status: "idle",
	Stage:  "none",
}
