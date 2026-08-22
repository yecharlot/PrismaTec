package agents

import "time"

// Pulse type constants (Alset-Gen protocol).
const (
	PulseConsulta      = "CONSULTA"
	PulseMutateRootCID = "MUTATE_ROOTCID"
	PulseEstado        = "ESTADO"
	PulseHallazgo      = "HALLAZGO"
	PulseGenCreated    = "GEN_CREATED"
	PulseGenMutated    = "GEN_MUTATED"
	PulseGenTravel     = "GEN_TRAVEL"
)

// GenOrganState is the local ternary field of an Alset-Gen (same axes as Mind).
type GenOrganState struct {
	Dialog    int `json:"dialog"`
	Act       int `json:"act"`
	Mem       int `json:"mem"`
	Self      int `json:"self"`
	Ethics    int `json:"ethics"`
	Curiosity int `json:"curiosity"`
	Humor     int `json:"humor"`
}

// GenState is mutable local state that may persist across mutations.
type GenState struct {
	Balance    float64                `json:"balance"`
	Reputation float64                `json:"reputation"`
	LastSeen   int64                  `json:"last_seen"`
	Location   string                 `json:"location"` // peer ID or node label
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// GenManifest describes the active form of a gen (content-addressed when stored).
type GenManifest struct {
	Type        string   `json:"type"`
	LogicCID    string   `json:"logic_cid,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	CreatedAt   int64    `json:"created_at"`
}

// AlsetGen is the metamorphic digital cell (manifesto v1.0).
// Identity (Key) is stable; RootCID and form are mutable.
type AlsetGen struct {
	ID             string        `json:"id"`
	Key            string        `json:"key"` // ANS name, e.g. gen-alfa.ans
	CreatedAt      int64         `json:"created_at"`
	CurrentRootCID string        `json:"current_root_cid"`
	History        []string      `json:"history"` // prior RootCIDs (metamorphosis chain)
	State          GenState      `json:"state"`
	Organs         GenOrganState `json:"organs"`
	Manifest       GenManifest   `json:"manifest"`
	EpisodeCIDs    []string      `json:"episode_cids,omitempty"`
	OriginNode     string        `json:"origin_node,omitempty"`
	UpdatedAt      int64         `json:"updated_at"`
}

// NewAlsetGen constructs a gen with stable key and initial root form.
func NewAlsetGen(id, key, rootCID, origin string, manifest GenManifest) *AlsetGen {
	now := time.Now().Unix()
	if manifest.CreatedAt == 0 {
		manifest.CreatedAt = now
	}
	if manifest.Version == "" {
		manifest.Version = "1.0"
	}
	if manifest.Type == "" {
		manifest.Type = "seed"
	}
	return &AlsetGen{
		ID:             id,
		Key:            key,
		CreatedAt:      now,
		UpdatedAt:      now,
		CurrentRootCID: rootCID,
		History:        nil,
		State: GenState{
			Balance:    1000,
			Reputation: 0,
			LastSeen:   now,
			Location:   origin,
			Metadata:   map[string]interface{}{},
		},
		Organs:     GenOrganState{},
		Manifest:   manifest,
		OriginNode: origin,
	}
}
