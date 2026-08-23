package node

import (
	"encoding/json"
	"os"
	"sync"
)

const mindGenomeFile = "mind_genome.json"

// MindGenome holds runtime-mutable thresholds (foundation for bounded mutation).
type MindGenome struct {
	Version             string  `json:"version"`
	AlarmLowCut         float64 `json:"alarm_low_cut"`
	AlarmHighCut        float64 `json:"alarm_high_cut"`
	VetoRiskBoost       float64 `json:"veto_risk_boost"`
	VetoPermDrop        float64 `json:"veto_perm_drop"`
	MaxVetoStack        int     `json:"max_veto_stack"`
	EpisodeKeep         int     `json:"episode_keep"`
	ActiveMemMinScore   int     `json:"active_mem_min_score"`
	CuriosityCut        float64 `json:"curiosity_cut"`         // continuous novelty/unknown → organ level
	HumorCut            float64 `json:"humor_cut"`             // comic signal → organ level
	MemoryActiveWeight  float64 `json:"memory_active_weight"`  // 0..1 influence of proactive recall
	AutoCalibrateEnabled bool   `json:"auto_calibrate_enabled"`
	FeedbackUp          int     `json:"feedback_up,omitempty"`
	FeedbackDown        int     `json:"feedback_down,omitempty"`
}

func defaultMindGenome() MindGenome {
	return MindGenome{
		Version:              "1.1.0",
		AlarmLowCut:          0.33,
		AlarmHighCut:         0.66,
		VetoRiskBoost:        0.08,
		VetoPermDrop:         0.05,
		MaxVetoStack:         3,
		EpisodeKeep:          32,
		ActiveMemMinScore:    1,
		CuriosityCut:         0.38,
		HumorCut:             0.30,
		MemoryActiveWeight:   0.7,
		AutoCalibrateEnabled: true,
	}
}

var (
	mindGenomeMu sync.RWMutex
	mindGenome   = defaultMindGenome()
)

func loadMindGenomeFromDisk() {
	b, err := os.ReadFile(mindGenomeFile)
	if err != nil {
		mindGenome = defaultMindGenome()
		_ = saveMindGenomeToDisk()
		return
	}
	var g MindGenome
	if json.Unmarshal(b, &g) != nil || g.AlarmHighCut <= g.AlarmLowCut {
		mindGenome = defaultMindGenome()
		return
	}
	// fill new fields if missing from old JSON
	if g.CuriosityCut <= 0 {
		g.CuriosityCut = 0.38
	}
	if g.HumorCut <= 0 {
		g.HumorCut = 0.30
	}
	if g.MemoryActiveWeight <= 0 {
		g.MemoryActiveWeight = 0.7
	}
	if g.Version == "" || g.Version == "1.0.0" {
		g.Version = "1.1.0"
	}
	mindGenomeMu.Lock()
	mindGenome = g
	mindGenomeMu.Unlock()
}

func saveMindGenomeToDisk() error {
	mindGenomeMu.RLock()
	g := mindGenome
	mindGenomeMu.RUnlock()
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mindGenomeFile, b, 0600)
}

func getMindGenome() MindGenome {
	mindGenomeMu.RLock()
	defer mindGenomeMu.RUnlock()
	return mindGenome
}

func setMindGenome(g MindGenome) {
	mindGenomeMu.Lock()
	mindGenome = g
	mindGenomeMu.Unlock()
	_ = saveMindGenomeToDisk()
}

// mutateMindGenomeBounded applies a small, ethics-gated change (manual/API later).
func mutateMindGenomeBounded(deltaRisk, deltaPerm float64) MindGenome {
	mindGenomeMu.Lock()
	defer mindGenomeMu.Unlock()
	g := mindGenome
	g.VetoRiskBoost = clamp01(g.VetoRiskBoost + deltaRisk)
	g.VetoPermDrop = clamp01(g.VetoPermDrop + deltaPerm)
	if g.VetoRiskBoost > 0.2 {
		g.VetoRiskBoost = 0.2
	}
	if g.VetoPermDrop > 0.15 {
		g.VetoPermDrop = 0.15
	}
	mindGenome = g
	_ = saveMindGenomeToDisk()
	return g
}

// applyFeedback adjusts genome slightly from user 👍/👎 (auto-calibrate).
func applyMindFeedback(up bool) MindGenome {
	mindGenomeMu.Lock()
	defer mindGenomeMu.Unlock()
	g := mindGenome
	if !g.AutoCalibrateEnabled {
		return g
	}
	step := 0.02
	if up {
		g.FeedbackUp++
		// reward: slightly more open curiosity, keep ethics thresholds stable
		g.CuriosityCut = clamp01Range(g.CuriosityCut-step*0.5, 0.35, 0.75)
		g.MemoryActiveWeight = clamp01Range(g.MemoryActiveWeight+step, 0.3, 1.0)
	} else {
		g.FeedbackDown++
		// dampen humor/curiosity if disliked
		g.HumorCut = clamp01Range(g.HumorCut+step, 0.35, 0.85)
		g.CuriosityCut = clamp01Range(g.CuriosityCut+step*0.5, 0.35, 0.85)
	}
	// every 10 feedbacks, nudge alarm band slightly toward better separation
	total := g.FeedbackUp + g.FeedbackDown
	if total > 0 && total%10 == 0 {
		if g.FeedbackUp > g.FeedbackDown {
			g.AlarmHighCut = clamp01Range(g.AlarmHighCut-0.01, 0.55, 0.85)
		} else {
			g.AlarmHighCut = clamp01Range(g.AlarmHighCut+0.01, 0.55, 0.85)
		}
	}
	mindGenome = g
	_ = saveMindGenomeToDisk()
	return g
}
