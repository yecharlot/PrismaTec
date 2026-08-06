package config

// Protocol and network identifiers for the Alset mesh.
const (
	AlsetProtocolID     = "/ptec-an/sync/1.0.0"
	AlsetDataExchangeID = "/ptec-an/data/1.0.0"
	AlsetGossipTopic    = "ptec-an-v4.0"
	BlocksDir           = "blocks"
	StaticDir           = "static"
	AdminPanelCIDKey    = "admin_panel_cid"
)

// PubSub topics used by the distributed neural layer.
const (
	NeuralSpikeTopic       = "ptec-an-neural-spike"
	InferenceRequestTopic  = "ptec-an-inference-request"
	InferenceResponseTopic = "ptec-an-inference-response"
	SynapticUpdateTopic    = "ptec-an-synaptic-update"
	MemoryQueryTopic       = "ptec-an-memory-query"
	MemoryResponseTopic    = "ptec-an-memory-response"
	NeuralStateSyncTopic   = "ptec-an-neural-sync"
	MemoryDistributedTopic = "ptec-an-memory-distributed"
)
