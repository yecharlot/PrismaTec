package node

import "testing"

func TestZyrionAbsorbing(t *testing.T) {
	if zyrionAbsorbing([]int{0, 0, 0}) != 0 {
		t.Fatal("all 0")
	}
	if zyrionAbsorbing([]int{1, 0, 0}) != 1 {
		t.Fatal("mixed without 2 -> 1")
	}
	if zyrionAbsorbing([]int{0, 2, 0}) != 2 {
		t.Fatal("any 2 -> 2")
	}
}

func TestAlarmPolarity(t *testing.T) {
	if alarmHigh(0.9) != 2 {
		t.Fatal("high risk")
	}
	if alarmLow(0.9) != 0 {
		t.Fatal("high permission safe")
	}
	if alarmLow(0.1) != 2 {
		t.Fatal("low permission alarm")
	}
}

func TestBiasMemoryRaisesRisk(t *testing.T) {
	sig := map[string]float64{"claridad": 0.7, "orden": 0.2, "riesgo": 0.2, "permiso": 0.9, "novedad": 0.3}
	eps := []mindEpisodePayload{{
		Text: "borra todo",
		Organs: []MindOrganResult{{Name: "ethics", State: 2}},
	}}
	out, hint := biasSignalsFromMemory(sig, eps)
	if out["riesgo"] <= sig["riesgo"] {
		t.Fatal("risk should rise")
	}
	if hint == "" {
		t.Fatal("expected hint")
	}
}
