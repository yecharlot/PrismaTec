package node

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed embedded/mind_dialog_acts.json
var embeddedDialogActsJSON []byte

type dialogActEntry struct {
	ID      string   `json:"id"`
	Act     string   `json:"act"`
	Keys    []string `json:"keys"`
	Phase   string   `json:"phase"` // opening | ongoing | closing | any
	Replies []string `json:"replies"`
}

var (
	dialogActsCache []dialogActEntry
	dialogActsMu    sync.RWMutex
)

func loadDialogActs() []dialogActEntry {
	dialogActsMu.RLock()
	if len(dialogActsCache) > 0 {
		out := dialogActsCache
		dialogActsMu.RUnlock()
		return out
	}
	dialogActsMu.RUnlock()

	dialogActsMu.Lock()
	defer dialogActsMu.Unlock()
	if len(dialogActsCache) > 0 {
		return dialogActsCache
	}
	var entries []dialogActEntry
	if len(embeddedDialogActsJSON) > 0 {
		_ = json.Unmarshal(embeddedDialogActsJSON, &entries)
	}
	dialogActsCache = entries
	return dialogActsCache
}

// matchDialogAct finds best act entry by key overlap + phase affinity.
func matchDialogAct(text string, sess *mindSessionState) (dialogActEntry, int) {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return dialogActEntry{}, 0
	}
	phase := phaseOpening
	if sess != nil && sess.Phase != "" {
		phase = sess.Phase
	}
	best := dialogActEntry{}
	bestSc := 0
	for _, e := range loadDialogActs() {
		sc := 0
		for _, k := range e.Keys {
			k = strings.ToLower(k)
			if k == "" {
				continue
			}
			if low == k {
				sc += 6
			} else if strings.HasPrefix(low, k+" ") || strings.HasPrefix(low, k+",") || strings.HasPrefix(low, k+"?") {
				sc += 4
			} else if len([]rune(k)) <= 3 {
				// claves muy cortas (ok, si, ty): solo exactas o token completo
				if containsWholeToken(low, k) {
					sc += 4
				}
			} else if containsWholeToken(low, k) || (len([]rune(k)) >= 5 && strings.Contains(low, k)) {
				sc += 2
			}
		}
		if sc == 0 {
			continue
		}
		if e.Phase == "any" || e.Phase == phase {
			sc += 2
		} else if e.Phase == phaseOngoing && phase == phaseOpening && sess != nil && sess.TurnCount > 0 {
			sc++
		}
		if sc > bestSc {
			bestSc = sc
			best = e
		}
	}
	return best, bestSc
}

// speakFromDialogActs: respuesta de corpus de actos (charla), no knowledge factual.
func speakFromDialogActs(text string, sess *mindSessionState) string {
	e, sc := matchDialogAct(text, sess)
	if sc < 2 || len(e.Replies) == 0 {
		return ""
	}
	// elegir respuesta estable por hash del texto (determinista)
	idx := 0
	sum := 0
	for _, r := range text {
		sum += int(r)
	}
	idx = sum % len(e.Replies)
	if idx < 0 {
		idx = 0
	}
	return e.Replies[idx]
}

// scoreDialogActsNaturalness: % de casos sociales del corpus de actos
// que no generan "menú de tools" / unknown (proxy de naturalidad).
func scoreDialogActsNaturalness() (ok, total int, details []string) {
	entries := loadDialogActs()
	if len(entries) == 0 {
		return 0, 0, []string{"dialog acts vacío"}
	}
	for _, e := range entries {
		if len(e.Keys) == 0 || len(e.Replies) == 0 {
			continue
		}
		total++
		probe := e.Keys[0]
		sess := &mindSessionState{Phase: e.Phase, TurnCount: 1}
		if e.Phase == phaseOpening {
			sess.TurnCount = 0
		}
		v := speakFromDialogActs(probe, sess)
		if v == "" {
			v, _ = speakSpeechAct(probe, sess)
		}
		low := strings.ToLower(v)
		if v == "" || strings.Contains(low, "no tengo una respuesta firme") ||
			strings.Contains(low, "generar código") || strings.Contains(low, "cálculo, código") {
			details = append(details, "fail:"+e.ID)
			continue
		}
		ok++
	}
	return ok, total, details
}

func containsWholeToken(s, tok string) bool {
	if tok == "" {
		return false
	}
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '?' || r == '!' || r == ';' || r == ':'
	}) {
		if w == tok {
			return true
		}
	}
	return false
}
