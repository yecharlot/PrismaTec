package node

import "testing"

func TestEvaluateActuateEthicsVeto(t *testing.T) {
	organs := []MindOrganResult{
		{Name: "ethics", State: 2},
		{Name: "act", State: 0},
	}
	st := evaluateActuate("busca harry potter", organs)
	if st.anyActive() {
		t.Fatalf("ethics=2 should zero all channels, got %+v", st)
	}
}

func TestEvaluateActuateExplore(t *testing.T) {
	organs := []MindOrganResult{
		{Name: "ethics", State: 0},
		{Name: "act", State: 0},
	}
	st := evaluateActuate("quién es juana de arco", organs)
	if st.Explore < 1 {
		t.Fatalf("expected explore active, got %+v", st)
	}
}

func TestSpeakFromActionMemoryEmpty(t *testing.T) {
	actionMemoryMu.Lock()
	actionMemoryRing = nil
	actionMemoryMu.Unlock()
	s := speakFromActionMemory("qué hice")
	if s == "" {
		t.Fatal("expected empty-session message")
	}
}
