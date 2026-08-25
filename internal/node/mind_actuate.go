package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ActuateState — sub-efectores ternarios bajo el órgano act (no es un 8.º órgano).
// 0 = no actuar, 1 = preparar / confirmar, 2 = ejecutar.
type ActuateState struct {
	Write       int `json:"write"`
	Read        int `json:"read"`
	Reason      int `json:"reason"`
	Explore     int `json:"explore"`
	Execute     int `json:"execute"`
	Communicate int `json:"communicate"`
	Delete      int `json:"delete"`
}

// ActionMemoryEntry — registro de una acción ejecutada (trazabilidad + aprendizaje).
type ActionMemoryEntry struct {
	Timestamp      string         `json:"timestamp"`
	Channel        string         `json:"channel"` // write|read|reason|explore|execute|communicate|delete
	Value          int            `json:"value"`  // 0|1|2
	Trigger        string         `json:"trigger"`
	OrgansSnapshot map[string]int `json:"organs_state"`
	Result         string         `json:"result,omitempty"`
	CID            string         `json:"cid,omitempty"`
	PrimaryKind    string         `json:"primary_kind,omitempty"`
}

const actionMemoryMax = 64
const actionMemoryFile = "mind_action_memory.json"

var (
	actionMemoryMu   sync.RWMutex
	actionMemoryRing []ActionMemoryEntry
)

func organStatesMap(organs []MindOrganResult) map[string]int {
	m := make(map[string]int, len(organs))
	for _, o := range organs {
		m[o.Name] = o.State
	}
	return m
}

// evaluateActuate decide canales 0/1/2 a partir del texto + órganos.
// ethics=2 o act=2 (bloqueo) → todo a 0. Capa de efecto bajo act, no órgano nuevo.
func evaluateActuate(input string, organs []MindOrganResult) ActuateState {
	st := ActuateState{}
	get := func(name string) int {
		for _, o := range organs {
			if o.Name == name {
				return o.State
			}
		}
		return 0
	}
	if get("ethics") == 2 {
		return st
	}
	// act=2 = BLOQUEO en labels actuales → no ejecutar efecto
	if get("act") == 2 {
		return st
	}

	low := strings.ToLower(strings.TrimSpace(input))
	norm := normalizeUserInput(input)

	// explore — sondas / web (reutiliza MindScoutWeb)
	if forceWebScout(norm) || isScoutableQuestion(norm) ||
		strings.Contains(low, "explora") || strings.Contains(low, "busca ") ||
		strings.Contains(low, "investiga") {
		st.Explore = 2
		if get("act") == 1 {
			st.Explore = 1
		}
	}

	// write — codegen / composición (no LLM libre)
	if isCodeGenRequest(low) || isCodeGenStrict(norm) ||
		strings.Contains(low, "escribe") || strings.Contains(low, "genera código") ||
		strings.Contains(low, "genera codigo") || strings.Contains(low, "redacta") {
		st.Write = 2
		if get("act") == 1 {
			st.Write = 1
		}
	}

	// reason — LispAI / matemática / zyrion
	if strings.Contains(low, "razona") || strings.Contains(low, "calcula") ||
		strings.Contains(low, "evalúa") || strings.Contains(low, "evalua") ||
		strings.Contains(low, "zyrion") || strings.Contains(low, "checkpoint") {
		st.Reason = 2
	}

	// read — memoria / estado / listados
	if isMemoryQuery(low) || strings.Contains(low, "dame ") ||
		strings.Contains(low, "lista ") || strings.Contains(low, "muestra ") ||
		strings.Contains(low, "estado") || strings.Contains(low, "recuerda") {
		st.Read = 2
	}

	// execute / communicate — tools Gen
	if isGenToolIntent(norm) {
		st.Execute = 2
		st.Communicate = 2
		if strings.Contains(low, "pregunta al gen") || strings.Contains(low, "dile al gen") {
			st.Communicate = 2
			st.Execute = 1
		}
	}

	// delete — sondas / genes
	if strings.Contains(low, "elimina gen") || strings.Contains(low, "borra gen") ||
		strings.Contains(low, "delete gen") || strings.Contains(low, "retorna gen") {
		st.Delete = 2
	}

	return st
}

func (s ActuateState) anyActive() bool {
	return s.Write > 0 || s.Read > 0 || s.Reason > 0 || s.Explore > 0 ||
		s.Execute > 0 || s.Communicate > 0 || s.Delete > 0
}

func (s ActuateState) activeChannels() []string {
	var out []string
	add := func(name string, v int) {
		if v > 0 {
			out = append(out, fmt.Sprintf("%s=%d", name, v))
		}
	}
	add("write", s.Write)
	add("read", s.Read)
	add("reason", s.Reason)
	add("explore", s.Explore)
	add("execute", s.Execute)
	add("communicate", s.Communicate)
	add("delete", s.Delete)
	return out
}

// recordAction appends to the in-memory ring and optionally persists.
func (n *NodoAlset) recordAction(channel string, value int, trigger, result, cid, primaryKind string, organs []MindOrganResult) {
	if value <= 0 || channel == "" {
		return
	}
	entry := ActionMemoryEntry{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Channel:        channel,
		Value:          value,
		Trigger:        compressVoiceBlock(trigger, 200),
		OrgansSnapshot: organStatesMap(organs),
		Result:         compressVoiceBlock(result, 400),
		CID:            cid,
		PrimaryKind:    primaryKind,
	}
	actionMemoryMu.Lock()
	actionMemoryRing = append(actionMemoryRing, entry)
	if len(actionMemoryRing) > actionMemoryMax {
		actionMemoryRing = actionMemoryRing[len(actionMemoryRing)-actionMemoryMax:]
	}
	snapshot := append([]ActionMemoryEntry(nil), actionMemoryRing...)
	actionMemoryMu.Unlock()

	go persistActionMemory(snapshot)
	if n != nil {
		n.Auditoria("MIND_ACTION", fmt.Sprintf("%s=%d kind=%s", channel, value, primaryKind))
	}
}

func persistActionMemory(entries []ActionMemoryEntry) {
	dir := "alset_data"
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, actionMemoryFile)
	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func loadActionMemory() {
	path := filepath.Join("alset_data", actionMemoryFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var entries []ActionMemoryEntry
	if json.Unmarshal(raw, &entries) != nil {
		return
	}
	actionMemoryMu.Lock()
	if len(entries) > actionMemoryMax {
		entries = entries[len(entries)-actionMemoryMax:]
	}
	actionMemoryRing = entries
	actionMemoryMu.Unlock()
}

// recentActions returns the last n action records.
func recentActions(n int) []ActionMemoryEntry {
	actionMemoryMu.RLock()
	defer actionMemoryMu.RUnlock()
	if n <= 0 || len(actionMemoryRing) == 0 {
		return nil
	}
	if n > len(actionMemoryRing) {
		n = len(actionMemoryRing)
	}
	out := make([]ActionMemoryEntry, n)
	copy(out, actionMemoryRing[len(actionMemoryRing)-n:])
	return out
}

func speakFromActionMemory(query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return ""
	}
	qf := foldSpanish(q)
	ask := strings.Contains(q, "qué hice") || strings.Contains(q, "que hice") ||
		strings.Contains(qf, "que hice") || strings.Contains(qf, "que hiciste") ||
		strings.Contains(q, "última acción") || strings.Contains(q, "ultima accion") ||
		strings.Contains(q, "acciones recientes") || strings.Contains(q, "qué ejecutaste") ||
		strings.Contains(q, "que ejecutaste") || strings.Contains(q, "historial de acción") ||
		strings.Contains(q, "historial de accion") || strings.Contains(q, "qué acciones") ||
		strings.Contains(q, "que acciones") || strings.Contains(q, "por qué hiciste") ||
		strings.Contains(q, "por que hiciste") || strings.Contains(q, "por qué lo hiciste") ||
		strings.Contains(q, "por que lo hiciste") || strings.Contains(qf, "porque lo hiciste") ||
		strings.Contains(qf, "porque hiciste") || strings.Contains(q, "cómo decides") ||
		strings.Contains(q, "como decides") || strings.Contains(q, "qué patrón") ||
		strings.Contains(q, "que patron") || strings.Contains(q, "explica tu acción") ||
		strings.Contains(q, "explica la acción")
	if !ask {
		return ""
	}
	recs := recentActions(8)
	if len(recs) == 0 {
		return "Aún no tengo acciones registradas en esta sesión. Cuando explore, escriba o calcule, quedará aquí con el porqué (órganos)."
	}
	var b strings.Builder
	b.WriteString("Registro de acciones (action_memory):\n")
	channelCount := map[string]int{}
	for i := len(recs) - 1; i >= 0; i-- {
		r := recs[i]
		channelCount[r.Channel]++
		b.WriteString(fmt.Sprintf("- %s · %s=%d", r.Timestamp, r.Channel, r.Value))
		if r.PrimaryKind != "" {
			b.WriteString(" · " + r.PrimaryKind)
		}
		// why: ethics/act/dialog snapshot
		if r.OrgansSnapshot != nil {
			why := []string{}
			for _, k := range []string{"ethics", "act", "dialog", "curiosity"} {
				if v, ok := r.OrgansSnapshot[k]; ok {
					why = append(why, fmt.Sprintf("%s=%d", k, v))
				}
			}
			if len(why) > 0 {
				b.WriteString(" · porque " + strings.Join(why, ","))
			}
		}
		if r.Trigger != "" {
			b.WriteString(" · pedido «" + compressVoiceBlock(r.Trigger, 40) + "»")
		}
		b.WriteByte('\n')
	}
	// simple pattern line
	var pats []string
	for ch, n := range channelCount {
		if n >= 2 {
			pats = append(pats, fmt.Sprintf("%s×%d", ch, n))
		}
	}
	if len(pats) > 0 {
		b.WriteString("Patrón en esta sesión: " + strings.Join(pats, ", ") + ". Repito canales que ya funcionaron bajo ethics=0.")
	}
	return strings.TrimSpace(b.String())
}

func foldSpanish(s string) string {
	repl := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "¿", "", "¡", "",
	)
	return repl.Replace(strings.ToLower(s))
}
