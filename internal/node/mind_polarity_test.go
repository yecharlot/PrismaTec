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
		Text:   "borra todo",
		Organs: []MindOrganResult{{Name: "ethics", State: 2}},
	}}
	out, hint, _ := biasSignalsFromMemory(sig, eps, "borra datos")
	if out["riesgo"] <= sig["riesgo"] {
		t.Fatal("risk should rise")
	}
	if hint == "" {
		t.Fatal("expected hint")
	}
}

func TestSpeakFromMemoryName(t *testing.T) {
	eps := []mindEpisodePayload{{Text: "me llamo Esteban"}, {Text: "hola"}}
	got := speakFromMemory("cómo me llamo?", eps)
	if got == "" || !containsFold(got, "Esteban") {
		t.Fatalf("expected name recall, got %q", got)
	}
}

func TestExtractDeclaredName(t *testing.T) {
	if extractDeclaredName("me llamo Esteban") != "Esteban" {
		t.Fatal("me llamo")
	}
	if extractDeclaredName("Mi nombre es Ana María.") != "Ana María" {
		t.Fatal("mi nombre es")
	}
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	ls, lsub := []rune(s), []rune(sub)
	// simple case-insensitive search
	for i := 0; i+len(lsub) <= len(ls); i++ {
		ok := true
		for j := 0; j < len(lsub); j++ {
			a, b := ls[i+j], lsub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func TestSpeakFromKnowledgeLife(t *testing.T) {
	got := speakFromKnowledge("qué es la vida")
	if got == "" {
		t.Fatal("expected knowledge on life")
	}
	if !containsFold(got, "CID") && !containsFold(got, "0/1/2") && !containsFold(got, "latido") {
		t.Fatalf("unexpected knowledge: %q", got)
	}
}

func TestSpeakFromKnowledgeDefun(t *testing.T) {
	got := speakFromKnowledge("qué es defun")
	if got == "" {
		t.Fatal("expected defun knowledge")
	}
}

func TestSpeakFromKnowledgeFactorial(t *testing.T) {
	got := speakFromKnowledge("factorial lisp")
	if got == "" || !containsFold(got, "defun") {
		t.Fatalf("factorial knowledge: %q", got)
	}
}

func TestSpeakFromKnowledgeQuote(t *testing.T) {
	got := speakFromKnowledge("qué es quote en lisp")
	if got == "" {
		t.Fatal("expected quote knowledge")
	}
}
