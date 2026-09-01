package node

import (
	_ "embed"

	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
)

type calibCase struct {
	Text   string         `json:"text"`
	Expect map[string]any `json:"expect"`
}

//go:embed embedded/mind_calibration_dialogs.json
var embeddedCalibJSON []byte

func loadCalibrationCorpus() []calibCase {
	if len(embeddedCalibJSON) > 0 {
		var cases []calibCase
		if json.Unmarshal(embeddedCalibJSON, &cases) == nil && len(cases) > 0 {
			return cases
		}
	}

	paths := []string{
		"docs/mind_calibration_dialogs.json",
		filepath.Join("..", "docs", "mind_calibration_dialogs.json"),
		"/home/workdir/artifacts/PrismaTec/docs/mind_calibration_dialogs.json",
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cases []calibCase
		if json.Unmarshal(b, &cases) == nil && len(cases) > 0 {
			return cases
		}
	}
	return nil
}

func expectMatch(got int, raw any) bool {
	switch v := raw.(type) {
	case float64:
		return got == int(v)
	case int:
		return got == v
	case []any:
		for _, x := range v {
			if expectMatch(got, x) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// scoreGenomeOnCorpus runs offline organ evaluation (no side effects) against calibration cases.
func (n *NodoAlset) scoreGenomeOnCorpus(g MindGenome) (score, total int, details []string) {
	cases := loadCalibrationCorpus()
	if len(cases) == 0 {
		return 0, 0, []string{"corpus vacío"}
	}
	// temporarily install genome
	mindGenomeMu.Lock()
	prev := mindGenome
	mindGenome = g
	mindGenomeMu.Unlock()
	defer func() {
		mindGenomeMu.Lock()
		mindGenome = prev
		mindGenomeMu.Unlock()
	}()

	for _, c := range cases {
		sig := signalsFromTextMind(c.Text)
		// no memory bias during pure calibration (stable ground truth)
		dialog := evalOrganPolar("dialog", sig["claridad"], sig["orden"], sig["riesgo"], "L", "H", "H")
		act := evalOrganPolar("act", sig["permiso"], sig["riesgo"], sig["orden"], "L", "H", "H")
		mem := evalOrganPolar("mem", sig["novedad"], sig["claridad"], sig["riesgo"], "H", "L", "H")
		self := evalOrganPolar("self", sig["claridad"], sig["riesgo"], sig["permiso"], "L", "H", "L")
		ethics := evalOrganPolar("ethics", sig["riesgo"], sig["permiso"], sig["orden"], "H", "L", "H")
		if ethics.State == 2 {
			act.State = 2
		}
		got := map[string]int{
			"dialog": dialog.State, "act": act.State, "mem": mem.State,
			"self": self.State, "ethics": ethics.State,
		}
		ok := true
		for k, exp := range c.Expect {
			if !expectMatch(got[k], exp) {
				ok = false
				details = append(details, fmt.Sprintf("FAIL «%s» %s got=%d expect=%v", c.Text, k, got[k], exp))
				break
			}
		}
		total++
		if ok {
			score++
		}
	}
	return score, total, details
}

func (n *NodoAlset) handleMindCalibrate(w http.ResponseWriter, r *http.Request) {
	g := getMindGenome()
	score, total, details := n.scoreGenomeOnCorpus(g)
	failN := len(details)
	if failN > 12 {
		details = details[:12]
	}
	writeJSON := func(v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	dOK, dTotal, dFail := scoreDialogActsNaturalness()
	writeJSON(map[string]interface{}{
		"genome": g,
		"score":  score,
		"total":  total,
		"rate":   float64(score) / math.Max(1, float64(total)),
		"fails":  details,
		"dialog_acts": map[string]interface{}{
			"ok":    dOK,
			"total": dTotal,
			"rate":  float64(dOK) / math.Max(1, float64(dTotal)),
			"fails": dFail,
		},
	})
}

// tryMutateGenomeAfterEpisode runs a bounded mutation trial if episode was significant.
func (n *NodoAlset) tryMutateGenomeAfterEpisode(memState int) map[string]interface{} {
	if memState < 1 {
		return nil
	}
	base := getMindGenome()
	score0, total, _ := n.scoreGenomeOnCorpus(base)
	if total == 0 {
		return nil
	}
	// small random-ish mutation from episode entropy (deterministic from score)
	cand := base
	step := 0.03
	if score0%2 == 0 {
		cand.AlarmHighCut = clamp01Range(cand.AlarmHighCut+step, 0.5, 0.85)
	} else {
		cand.AlarmHighCut = clamp01Range(cand.AlarmHighCut-step, 0.5, 0.85)
	}
	if memState == 2 {
		cand.VetoRiskBoost = clamp01Range(cand.VetoRiskBoost+0.01, 0.04, 0.2)
	}
	score1, _, fails := n.scoreGenomeOnCorpus(cand)
	out := map[string]interface{}{
		"base_score": score0,
		"cand_score": score1,
		"total":      total,
		"accepted":   false,
	}
	if score1 > score0 {
		mindGenomeMu.Lock()
		mindGenome = cand
		mindGenomeMu.Unlock()
		_ = saveMindGenomeToDisk()
		out["accepted"] = true
		out["genome"] = cand
		n.Auditoria("MIND_GENOME_MUTATION", fmt.Sprintf("score %d→%d", score0, score1))
		go n.BroadcastPulse("mind_genome", map[string]interface{}{
			"score": score1, "total": total,
		})
	} else {
		out["fails_sample"] = fails
		if len(fails) > 5 {
			out["fails_sample"] = fails[:5]
		}
	}
	return out
}

func clamp01Range(f, lo, hi float64) float64 {
	if f < lo {
		return lo
	}
	if f > hi {
		return hi
	}
	return f
}

// tfidf-ish weight for active memory (document frequency inverse in episode set)
func episodeTokenWeight(token string, episodes []mindEpisodePayload) float64 {
	df := 0
	for _, ep := range episodes {
		for _, w := range tokenizeMind(ep.Text) {
			if w == token {
				df++
				break
			}
		}
	}
	if df == 0 {
		return 1
	}
	return 1.0 + math.Log(float64(len(episodes)+1)/float64(df))
}
