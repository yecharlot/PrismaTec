package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	operatorStateFile   = "operator_state.json"
	defaultOperatorID   = "agent-operator-prismatec"
	defaultOperatorAlias = "admin.prismatec.ans"
)

// OperatorState links a human-readable alias to the admin config CID and agent.
type OperatorState struct {
	Alias       string `json:"alias"`
	AgentID     string `json:"agent_id"`
	ConfigCID   string `json:"config_cid"`
	PeerID      string `json:"peer_id,omitempty"`
	Initialized bool   `json:"initialized"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (n *NodoAlset) loadOperatorState() OperatorState {
	var st OperatorState
	data, err := os.ReadFile(operatorStateFile)
	if err == nil {
		_ = json.Unmarshal(data, &st)
	}
	return st
}

func (n *NodoAlset) saveOperatorState(st OperatorState) error {
	st.UpdatedAt = time.Now().Unix()
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(operatorStateFile, b, 0600)
}

func bootstrapSecretOK(provided string) bool {
	env := strings.TrimSpace(os.Getenv("BOOTSTRAP_SECRET"))
	if env == "" {
		// Dev/local: allow first setup without secret, but never on empty provided when env is set
		return true
	}
	return provided != "" && provided == env
}

func bootstrapSecretConfigured() bool {
	return strings.TrimSpace(os.Getenv("BOOTSTRAP_SECRET")) != ""
}

func normalizeAlias(alias string) string {
	alias = strings.TrimSpace(strings.ToLower(alias))
	alias = strings.Trim(alias, "\"")
	if alias == "" {
		return ""
	}
	if !strings.Contains(alias, ".") {
		alias = alias + ".ans"
	}
	return alias
}

func (n *NodoAlset) resolveConfigCIDFromAlias(alias string) (string, string, error) {
	alias = normalizeAlias(alias)
	st := n.loadOperatorState()
	if st.Initialized && st.ConfigCID != "" {
		if alias == "" || alias == normalizeAlias(st.Alias) || strings.TrimSuffix(alias, ".ans") == strings.TrimSuffix(normalizeAlias(st.Alias), ".ans") {
			return st.ConfigCID, st.Alias, nil
		}
	}
	n.mu.RLock()
	agentID, ok := n.nombres[alias]
	if !ok {
		agentID, ok = n.nombres[strings.TrimSuffix(alias, ".ans")]
	}
	var root string
	if ok {
		if a, exists := n.agentes[agentID]; exists && a != nil {
			root = a.RootCID
		}
	}
	n.mu.RUnlock()
	if root != "" {
		return root, alias, nil
	}
	if st.ConfigCID != "" && st.Initialized {
		return st.ConfigCID, st.Alias, nil
	}
	return "", "", fmt.Errorf("alias no encontrado")
}

func (n *NodoAlset) ensureOperatorAgent(alias, configCID string) (agentID string) {
	agentID = defaultOperatorID
	alias = normalizeAlias(alias)
	if alias == "" {
		alias = defaultOperatorAlias
	}
	n.mu.Lock()
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	if _, ok := n.agentes[agentID]; !ok {
		n.agentes[agentID] = &Agente{
			ID:           agentID,
			RootCID:      configCID,
			UltimaActual: time.Now().Unix(),
			BalanceUTXO:  0,
		}
	} else {
		n.agentes[agentID].RootCID = configCID
		n.agentes[agentID].UltimaActual = time.Now().Unix()
	}
	n.nombres[alias] = agentID
	// short form without .ans
	short := strings.TrimSuffix(alias, ".ans")
	if short != alias {
		n.nombres[short] = agentID
	}
	n.mu.Unlock()
	n.Auditoria("OPERATOR_BOUND", fmt.Sprintf("alias=%s agent=%s config=%s", alias, agentID, configCID))
	return agentID
}

// handleAdminStatus reports whether the node is initialized and if bootstrap secret is required.
func (n *NodoAlset) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	st := n.loadOperatorState()
	peer := ""
	if n.host != nil {
		peer = n.host.ID().String()
	}
	writeJSON := func(v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	writeJSON(map[string]interface{}{
		"initialized":         st.Initialized && st.ConfigCID != "",
		"bootstrap_required":  bootstrapSecretConfigured() && !(st.Initialized && st.ConfigCID != ""),
		"bootstrap_configured": bootstrapSecretConfigured(),
		"alias":               st.Alias,
		"peer_id":             peer,
		"has_operator":        st.AgentID != "",
	})
}
