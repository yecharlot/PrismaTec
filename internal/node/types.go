package node

import (
	"context"
	"sync"

	"redalset/internal/agents"
	"redalset/internal/neural"
	"redalset/internal/poh"
	syncpkg "redalset/internal/sync"
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

type SyncMode = syncpkg.Mode
type SyncConfig = syncpkg.Config
type SyncProgress = syncpkg.Progress

const (
	SyncModeQuick       = syncpkg.ModeQuick
	SyncModeFull        = syncpkg.ModeFull
	SyncModeIncremental = syncpkg.ModeIncremental
)

// SyncManager keeps a back-reference to the node (not portable to syncpkg alone).
type SyncManager struct {
	nodo         *NodoAlset
	config       SyncConfig
	isSyncing    bool
	syncProgress float64
	syncCancel   context.CancelFunc
	mu           sync.RWMutex
}

var globalSyncProgress = syncpkg.NewProgress()
