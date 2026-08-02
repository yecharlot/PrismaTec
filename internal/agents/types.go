package agents

// Agente is a network agent identity and state.
type Agente struct {
	ID           string  `json:"id"`
	RootCID      string  `json:"root_cid"`
	UltimaActual int64   `json:"ultima_actualizacion"`
	BalanceUTXO  float64 `json:"balance_utxo"`
}

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
