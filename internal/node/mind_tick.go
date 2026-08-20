package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MindTickRequest is the body for POST /api/mind/tick
type MindTickRequest struct {
	Text     string             `json:"text"`
	Signals  map[string]float64 `json:"signals"` // optional override
	ForceMem bool               `json:"force_mem"`
}

// MindOrganResult is one ternary organ reading.
type MindOrganResult struct {
	Name   string  `json:"name"`
	State  int     `json:"state"` // 0,1,2
	Label  string  `json:"label"`
	Inputs []float64 `json:"inputs"`
}

// MindTickResponse is the field reading for one heartbeat.
type MindTickResponse struct {
	Species    string           `json:"species"`
	Organs     []MindOrganResult `json:"organs"`
	Voice      string           `json:"voice"`
	EpisodeCID string           `json:"episode_cid,omitempty"`
	MemState   int              `json:"mem_state"`
	Note       string           `json:"note"`
}

// level03 maps [0,1] continuous to ternary intensity 0/1/2 (low/mid/high).
func level03(f float64) int {
	if f == 0 || f == 1 || f == 2 {
		if f > 2 {
			return 2
		}
		return int(f)
	}
	if f < 0.33 {
		return 0
	}
	if f < 0.66 {
		return 1
	}
	return 2
}

// alarmHigh: high continuous = more alarm (2). Used for riesgo, orden agresivo.
func alarmHigh(f float64) int { return level03(f) }

// alarmLow: low continuous = more alarm (2). Used for permiso, claridad.
//  high permiso → 0 (safe); low permiso → 2 (unsafe).
func alarmLow(f float64) int {
	if f >= 0.66 {
		return 0
	}
	if f >= 0.33 {
		return 1
	}
	return 2
}

// zyrionAbsorbingMind: any 2 → 2 (sumidero); all 0 → 0; otherwise → 1 (matizar).
// Mixed low signals no longer collapse to alarm — that was flooding "hola" into VETO.
func zyrionAbsorbing(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	all0 := true
	for _, v := range vals {
		if v == 2 {
			return 2
		}
		if v != 0 {
			all0 = false
		}
	}
	if all0 {
		return 0
	}
	return 1
}

func labelForState(s int) string {
	switch s {
	case 0:
		return "SEGUIR"
	case 1:
		return "MATIZAR"
	default:
		return "VETO"
	}
}

func labelForOrgan(name string, s int) string {
	switch name {
	case "mem":
		if s == 0 {
			return "SILENCIO"
		}
		if s == 1 {
			return "RESUMEN"
		}
		return "EPISODIO"
	case "dialog":
		if s == 0 {
			return "CHARLA"
		}
		if s == 1 {
			return "PEDIDO"
		}
		return "ORDEN"
	case "act":
		if s == 0 {
			return "LISTO"
		}
		if s == 1 {
			return "CONFIRMAR"
		}
		return "BLOQUEO"
	case "self":
		if s == 0 {
			return "ANCLADO"
		}
		if s == 1 {
			return "ACLARAR"
		}
		return "REANCLAR"
	case "ethics":
		if s == 0 {
			return "PERMITIR"
		}
		if s == 1 {
			return "LIMITAR"
		}
		return "SUMIDERO"
	default:
		return labelForState(s)
	}
}

func signalsFromTextMind(t string) map[string]float64 {
	s := strings.ToLower(strings.TrimSpace(t))
	claridad := 0.7
	orden := 0.25
	riesgo := 0.3
	permiso := 0.75
	novedad := 0.4
	// Greetings / ack: calm field
	if s == "hola" || s == "hi" || s == "hello" || s == "hey" || s == "buenas" ||
		s == "bien" || s == "ok" || s == "gracias" || s == "good" {
		return map[string]float64{"claridad": 0.85, "orden": 0.1, "riesgo": 0.1, "permiso": 0.9, "novedad": 0.15}
	}
	if len(s) < 8 {
		claridad = 0.55
	}
	if len(t) > 80 {
		novedad = 0.75
	}
	if strings.Contains(s, "borra") || strings.Contains(s, "elimina") || strings.Contains(s, "reset") || strings.Contains(s, "resetea") ||
		strings.Contains(s, "password") || strings.Contains(s, "secret") || strings.Contains(s, "rm ") {
		riesgo = 0.92
		permiso = 0.15
		orden = 0.85
	}
	if strings.Contains(s, "crea") || strings.Contains(s, "registra") || strings.Contains(s, "despliega") ||
		strings.Contains(s, "ejecuta") || strings.Contains(s, "agente") {
		orden = 0.8
	}
	if strings.Contains(s, "estado") || strings.Contains(s, "status") {
		orden = 0.2
		riesgo = 0.15
		permiso = 0.9
	}
	if strings.Contains(s, "zyrion") || strings.Contains(s, "evalua") || strings.Contains(s, "evalúa") ||
		strings.Contains(s, "checkpoint") || strings.Contains(s, "topolog") {
		orden = 0.2
		riesgo = 0.1
		permiso = 0.95
		claridad = 0.85
		novedad = 0.5
	}
	if strings.Contains(s, "red") || strings.Contains(s, "peer") || strings.Contains(s, "network") {
		orden = 0.25
		riesgo = 0.12
		permiso = 0.9
	}
	return map[string]float64{
		"claridad": claridad,
		"orden":    orden,
		"riesgo":   riesgo,
		"permiso":  permiso,
		"novedad":  novedad,
	}
}

// evalOrganPolar applies per-slot polarity: "H" alarm if high, "L" alarm if low.
func evalOrganPolar(name string, a, b, c float64, pa, pb, pc string) MindOrganResult {
	mapSlot := func(f float64, pol string) int {
		if pol == "L" {
			return alarmLow(f)
		}
		return alarmHigh(f)
	}
	ia, ib, ic := mapSlot(a, pa), mapSlot(b, pb), mapSlot(c, pc)
	st := zyrionAbsorbing([]int{ia, ib, ic})
	return MindOrganResult{Name: name, State: st, Label: labelForOrgan(name, st), Inputs: []float64{a, b, c}}
}

func (n *NodoAlset) runMindTick(text string, override map[string]float64, forceMem bool) MindTickResponse {
	sig := signalsFromTextMind(text)
	for k, v := range override {
		sig[k] = v
	}
	// Organs with polarity: H = high is alarm, L = low is alarm
	// dialog: low clarity, high order, high risk → escalate
	dialog := evalOrganPolar("dialog", sig["claridad"], sig["orden"], sig["riesgo"], "L", "H", "H")
	// act: low permission, high risk, high order → veto action
	act := evalOrganPolar("act", sig["permiso"], sig["riesgo"], sig["orden"], "L", "H", "H")
	// mem: high novelty records; high risk also marks episode; low clarity less critical
	mem := evalOrganPolar("mem", sig["novedad"], sig["claridad"], sig["riesgo"], "H", "L", "H")
	// self: low clarity / high risk / low permission → re-anchor
	self := evalOrganPolar("self", sig["claridad"], sig["riesgo"], sig["permiso"], "L", "H", "L")
	// ethics: high risk, low permission, high aggressive order → sumidero
	ethics := evalOrganPolar("ethics", sig["riesgo"], sig["permiso"], sig["orden"], "H", "L", "H")

	// Ethics 2 absorbs action
	if ethics.State == 2 && act.State != 2 {
		act.State = 2
		act.Label = labelForOrgan("act", 2)
	}

	organs := []MindOrganResult{dialog, act, mem, self, ethics}
	voice := mindVoice(text, organs)
	// Safe tools when ethics/act allow (not veto)
	if ethics.State != 2 && act.State != 2 {
		if extra := n.mindSafeTools(text); extra != "" {
			voice = voice + "\n\n" + extra
		}
	}
	resp := MindTickResponse{
		Species:  "Alset-Mind",
		Organs:   organs,
		Voice:    voice,
		MemState: mem.State,
		Note:     "latido-nativo-go+zyrion-absorbente",
	}

	saveEp := forceMem || (mem.State >= 1 && (sig["riesgo"] >= 0.5 || len(text) > 48 || mem.State == 2))
	if saveEp {
		ep := map[string]interface{}{
			"type":      "mind_episode",
			"text":      text,
			"signals":   sig,
			"organs":    organs,
			"voice":     voice,
			"ts":        time.Now().UTC().Format(time.RFC3339),
			"agent":     mindAgentID,
		}
		raw, _ := json.Marshal(ep)
		if cid, err := n.GenerarCID(raw); err == nil && cid != "" {
			resp.EpisodeCID = cid
			n.Auditoria("MIND_EPISODE", fmt.Sprintf("cid=%s mem=%d", cid, mem.State))
			// Append episode ref on mind agent root as lightweight index (best-effort)
			n.mu.Lock()
			if a, ok := n.agentes[mindAgentID]; ok && a != nil {
				a.UltimaActual = time.Now().Unix()
			}
			n.mu.Unlock()
		}
	}
	return resp
}

func mindVoice(text string, organs []MindOrganResult) string {
	get := func(name string) MindOrganResult {
		for _, o := range organs {
			if o.Name == name {
				return o
			}
		}
		return MindOrganResult{}
	}
	e, a, d := get("ethics"), get("act"), get("dialog")
	low := strings.ToLower(text)
	if e.State == 2 {
		return "Campo ethics en 2 (sumidero). No actúo sobre el nodo. Reformule con menos riesgo o pida solo lectura."
	}
	if a.State == 2 {
		return "Órgano act en veto. Puedo explicar el estado, pero no ejecuto cambios sin confirmar."
	}
	if strings.Contains(low, "zyrion") || strings.Contains(low, "evalua") || strings.Contains(low, "evalúa") || strings.Contains(low, "checkpoint") {
		return "Zyrion en modo lectura. Tres checkpoints de daño (bajo / medio / alto) evaluados en este nodo."
	}
	if strings.Contains(low, "estado") || strings.Contains(low, "red") || strings.Contains(low, "quién eres") || strings.Contains(low, "quien eres") {
		return "Campo estable. Soy Alset Mind — inteligencia ternaria residente. Abajo el cuerpo del nodo (solo lectura)."
	}
	if d.State == 1 || a.State == 1 {
		snip := text
		if len(snip) > 100 {
			snip = snip[:100] + "…"
		}
		return "Campo en matiz (1). Interpreté: «" + snip + "». ¿Confirma acción sobre el nodo o solo consulta?"
	}
	return "Latido OK. Pruebe: «dame estado», «evalua zyrion», «dame red»."
}


// mindSafeTools runs read-only node introspection and Zyrion demos.
func (n *NodoAlset) mindSafeTools(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	wantStatus := strings.Contains(s, "estado") || strings.Contains(s, "status") ||
		strings.Contains(s, "quién eres") || strings.Contains(s, "quien eres") ||
		strings.Contains(s, "qué puedes") || strings.Contains(s, "que puedes") ||
		strings.Contains(s, "self") || strings.Contains(s, "agentes") || strings.Contains(s, "peers") ||
		strings.Contains(s, "apps") || strings.Contains(s, "nombres") || strings.Contains(s, "mind") ||
		strings.Contains(s, "red") || strings.Contains(s, "network") ||
		s == "hola" || strings.HasPrefix(s, "dame ")
	wantZyrion := strings.Contains(s, "zyrion") || strings.Contains(s, "evalua") ||
		strings.Contains(s, "evalúa") || strings.Contains(s, "checkpoint") || strings.Contains(s, "topolog")
	if !wantStatus && !wantZyrion {
		return ""
	}

	var lines []string
	if wantStatus {
		lines = append(lines, n.mindBodySnapshot(s)...)
	}
	if wantZyrion {
		lines = append(lines, n.mindZyrionDemo(s)...)
	}
	return strings.Join(lines, "\n")
}

func (n *NodoAlset) mindBodySnapshot(s string) []string {
	n.mu.RLock()
	nAgents := len(n.agentes)
	nNames := len(n.nombres)
	root := ""
	if a, ok := n.agentes[mindAgentID]; ok && a != nil {
		root = a.RootCID
	}
	agentIDs := make([]string, 0, 12)
	for id := range n.agentes {
		agentIDs = append(agentIDs, id)
		if len(agentIDs) >= 12 {
			break
		}
	}
	nameSamples := make([]string, 0, 8)
	for alias := range n.nombres {
		nameSamples = append(nameSamples, alias)
		if len(nameSamples) >= 8 {
			break
		}
	}
	n.mu.RUnlock()
	peers := 0
	peerID := ""
	addrs := ""
	if n.host != nil {
		peers = len(n.host.Network().Peers())
		peerID = n.host.ID().String()
		if len(peerID) > 22 {
			peerID = peerID[:22] + "…"
		}
		for _, a := range n.host.Addrs() {
			addrs = a.String()
			break
		}
	}
	apps := []string{}
	if entries, err := os.ReadDir(filepath.Join(StaticDir, "apps")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				apps = append(apps, e.Name())
			}
			if len(apps) >= 12 {
				break
			}
		}
	}
	lines := []string{
		"—— cuerpo del nodo ——",
		fmt.Sprintf("peer: %s · peers: %d", peerID, peers),
		fmt.Sprintf("agentes: %d · DNS: %d · apps: %d", nAgents, nNames, len(apps)),
		fmt.Sprintf("Mind: %s · id: %s · root: %s", mindAlias, mindAgentID, root),
	}
	if addrs != "" {
		lines = append(lines, "listen: "+addrs)
	}
	if len(apps) > 0 {
		lines = append(lines, "apps: "+strings.Join(apps, ", "))
	}
	if strings.Contains(s, "agente") || strings.Contains(s, "estado") {
		if len(agentIDs) > 0 {
			lines = append(lines, "agentes: "+strings.Join(agentIDs, ", "))
		}
	}
	if strings.Contains(s, "nombre") || strings.Contains(s, "dns") {
		if len(nameSamples) > 0 {
			lines = append(lines, "nombres: "+strings.Join(nameSamples, ", "))
		}
	}
	if strings.Contains(s, "red") || strings.Contains(s, "peer") || strings.Contains(s, "network") {
		lines = append(lines, fmt.Sprintf("red: peers activos=%d (relay puede estar solo)", peers))
	}
	return lines
}

func (n *NodoAlset) mindZyrionDemo(s string) []string {
	lines := []string{"—— Zyrion (campo nativo) ——"}
	if n.lisp == nil {
		return append(lines, "LispAI no disponible en este proceso")
	}
	// Three damage scenarios for DNA-style checkpoint (same DSL as API demos)
	cases := []struct {
		name string
		cmd  string
	}{
		{"daño bajo", `(evaluar-zyrion (quote (CHECKPOINT-ADN :entradas (p53 MDM2 BAX) :salidas ((0 APOPTOSIS) (1 REPARACION) (2 CHECKPOINT)))) (quote (p53 0.2 MDM2 0.8 BAX 0.1)))`},
		{"daño medio", `(evaluar-zyrion (quote (CHECKPOINT-ADN :entradas (p53 MDM2 BAX) :salidas ((0 APOPTOSIS) (1 REPARACION) (2 CHECKPOINT)))) (quote (p53 0.6 MDM2 0.5 BAX 0.4)))`},
		{"daño alto", `(evaluar-zyrion (quote (CHECKPOINT-ADN :entradas (p53 MDM2 BAX) :salidas ((0 APOPTOSIS) (1 REPARACION) (2 CHECKPOINT)))) (quote (p53 0.9 MDM2 0.2 BAX 0.8)))`},
	}
	for _, c := range cases {
		res, err := n.lisp.Eval(c.cmd)
		if err != nil {
			lines = append(lines, fmt.Sprintf("%s: error %v", c.name, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s → %v", c.name, res))
	}
	lines = append(lines, "ley: 0 seguir · 1 matizar · 2 sumidero (absorbente)")
	return lines
}

func (n *NodoAlset) handleMindTick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var req MindTickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	out := n.runMindTick(req.Text, req.Signals, req.ForceMem)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (n *NodoAlset) handleMindSelf(w http.ResponseWriter, r *http.Request) {
	n.mu.RLock()
	a := n.agentes[mindAgentID]
	alias := mindAlias
	root := ""
	if a != nil {
		root = a.RootCID
	}
	n.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"species":  "Alset-Mind",
		"agent_id": mindAgentID,
		"alias":    alias,
		"root_cid": root,
		"organs":   []string{"dialog", "act", "mem", "self", "ethics"},
		"endpoints": []string{"POST /api/mind/tick", "GET /api/mind/self", "POST /api/lispai (mind-latido)"},
		"docs":     []string{"docs/ALSET_MIND_THESIS.md", "docs/ALSET_MIND_HANDOFF.md", "docs/AI_COLLABORATION.md"},
	})
}
