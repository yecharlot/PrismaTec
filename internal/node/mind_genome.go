package node

import (
	"encoding/json"
	"os"
	"sync"
)

const mindGenomeFile = "mind_genome.json"

// MindGenome holds runtime-mutable thresholds (foundation for bounded mutation).
// Not hard-coded forever: load/save JSON on disk; ethics can gate writes later.
type MindGenome struct {
	Version        string  `json:"version"`
	AlarmLowCut    float64 `json:"alarm_low_cut"`  // < this → level 0 for high-polarity
	AlarmHighCut   float64 `json:"alarm_high_cut"` // >= this → level 2 for high-polarity
	VetoRiskBoost  float64 `json:"veto_risk_boost"`
	VetoPermDrop   float64 `json:"veto_perm_drop"`
	MaxVetoStack   int     `json:"max_veto_stack"`
	EpisodeKeep    int     `json:"episode_keep"`
	ActiveMemMinScore int  `json:"active_mem_min_score"`
}

func defaultMindGenome() MindGenome {
	return MindGenome{
		Version:           "1.0.0",
		AlarmLowCut:       0.33,
		AlarmHighCut:      0.66,
		VetoRiskBoost:     0.08,
		VetoPermDrop:      0.05,
		MaxVetoStack:      3,
		EpisodeKeep:       32,
		ActiveMemMinScore: 1,
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
