package neural

type SynapticWeight struct {
	TargetNeuronID  string  `json:"target_neuron_id"`
	Weight          float64 `json:"weight"`
	LastUpdated     int64   `json:"last_updated"`
	SuccessfulFires int64   `json:"successful_fires"`
}

type NeuralState struct {
	MembranePotential float64                   `json:"membrane_potential"`
	LastSpikeTime     int64                     `json:"last_spike_time"`
	SpikeThreshold    float64                   `json:"spike_threshold"`
	LeakRate          float64                   `json:"leak_rate"`
	RefractoryPeriod  int64                     `json:"refractory_period"`
	Synapses          map[string]SynapticWeight `json:"synapses"`
	NeuronType        string                    `json:"neuron_type"`
}

type InferenceRequest struct {
	RequestID    string    `json:"request_id"`
	InputData    []float64 `json:"input_data"`
	OriginNodeID string    `json:"origin_node_id"`
	TTL          int       `json:"ttl"`
}

type InferenceResponse struct {
	RequestID      string    `json:"request_id"`
	OutputData     []float64 `json:"output_data"`
	ProcessingNode string    `json:"processing_node"`
	ProcessingTime int64     `json:"processing_time"`
}

type MemoryQuery struct {
	QueryID    string `json:"query_id"`
	Content    string `json:"content"`
	OriginNode string `json:"origin_node"`
	TTL        int    `json:"ttl"`
}

type MemoryResponse struct {
	QueryID       string   `json:"query_id"`
	Results       []string `json:"results"`
	Contents      []string `json:"contents"`
	ResponderNode string   `json:"responder_node"`
}
