package node

import (
	"context"
	"sync"

	"redalset/internal/agents"
	"redalset/internal/neural"
)

type Agente = agents.Agente

type NodoConfig struct {
	AdminPassHash string `json:"admin_pass_hash"`
	LastUpdate    int64  `json:"last_update"`
	Version       string `json:"version"`
	AdminPanelCID string `json:"admin_panel_cid"`
	IsGenesis     bool   `json:"is_genesis"`
}

type PoHEvent struct {
	Timestamp   int64  `json:"timestamp"`
	EventType   string `json:"event_type"`
	Metadata    string `json:"metadata"`
	Signature   string `json:"signature,omitempty"`
	HumanitySig string `json:"humanity_sig"`
}

type HumanityProof struct {
	SessionID string     `json:"session_id"`
	Events    []PoHEvent `json:"events"`
	FinalSig  string     `json:"final_signature"`
}

var globalPoH = struct {
	sync.Mutex
	sessionID string
	events    []PoHEvent
}{
	sessionID: "",
	events:    []PoHEvent{},
}

type SynapticWeight = neural.SynapticWeight

type NeuralState = neural.NeuralState

type InferenceRequest = neural.InferenceRequest

type InferenceResponse = neural.InferenceResponse

type MemoryQuery = neural.MemoryQuery

type MemoryResponse = neural.MemoryResponse

// =============================================================================
// EXTENSIONES: MÓDULOS, ENTIDADES, SEGURIDAD
// =============================================================================

type Modulo struct {
	ID         string                 `json:"id"`
	Nombre     string                 `json:"nombre"`
	Rol        string                 `json:"rol"`
	Atributos  map[string]interface{} `json:"atributos"`
	Relaciones []string               `json:"relaciones"`
	RootCID    string                 `json:"root_cid"`
	Owner      string                 `json:"owner"`
	CreatedAt  int64                  `json:"created_at"`
}

type EntidadProgramatica struct {
	ID        string                 `json:"id"`
	Tipo      string                 `json:"tipo"`
	Atributos map[string]interface{} `json:"atributos"`
	HeredaDe  string                 `json:"hereda_de"`
	ModuloID  string                 `json:"modulo_id"`
}

type RelacionEntidad struct {
	ID           string `json:"id"`
	EntidadA     string `json:"entidad_a"`
	EntidadB     string `json:"entidad_b"`
	Tipo         string `json:"tipo"`
	Cardinalidad string `json:"cardinalidad"`
}

type TokenAlset struct {
	Token     string   `json:"token"`
	AgentID   string   `json:"agent_id"`
	RootCID   string   `json:"root_cid"`
	ExpiresAt int64    `json:"expires_at"`
	Roles     []string `json:"roles"`
	Permisos  []string `json:"permisos"`
	Signature string   `json:"signature"`
}

type UsuarioRoles struct {
	AgentID string   `json:"agent_id"`
	Roles   []string `json:"roles"`
	Modulos []string `json:"modulos"`
}

var (
	modulosGlobales    = make(map[string]*Modulo)
	entidadesGlobales  = make(map[string]*EntidadProgramatica)
	relacionesGlobales = make(map[string]*RelacionEntidad)
	tokensActivos      = make(map[string]*TokenAlset)
	rolesGlobales      = make(map[string][]string)
	muModulos          sync.RWMutex
	muEntidades        sync.RWMutex
	muTokens           sync.RWMutex
)

// =============================================================================
// SISTEMA DE SINCRONIZACIÓN HÍBRIDA
// =============================================================================

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

// =============================================================================
// TIPOS LISP
// =============================================================================
