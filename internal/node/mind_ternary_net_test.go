package node

import "testing"

func TestCortexClarifyIncomplete(t *testing.T) {
	s := senseFeaturesTernary("dime tu")
	if s["sense_clarify"] < 2 {
		t.Fatalf("expected sense_clarify=2 got %v", s)
	}
	routes := propagateTernary(s, mustSyn())
	if pickDominantRoute(routes) != "route_clarify" {
		t.Fatalf("route=%s want route_clarify routes=%v", pickDominantRoute(routes), routes)
	}
}

func TestCortexMath(t *testing.T) {
	s := senseFeaturesTernary("cuánto es 12+5")
	if s["sense_math"] < 2 {
		t.Fatalf("math sense %v", s)
	}
	if pickDominantRoute(propagateTernary(s, mustSyn())) != "route_math" {
		t.Fatalf("want math route")
	}
}

func TestCortexReason(t *testing.T) {
	s := senseFeaturesTernary("si todos los humanos son mortales y socrates es humano entonces")
	if s["sense_reason"] < 2 {
		t.Fatalf("reason sense %v", s)
	}
	dom := pickDominantRoute(propagateTernary(s, mustSyn()))
	if dom != "route_reason" {
		t.Fatalf("got %s", dom)
	}
}

func TestCortexAlsetBeatsScout(t *testing.T) {
	s := senseFeaturesTernary("qué es zyrion en alset mind")
	if s["sense_alset"] < 2 {
		t.Fatalf("alset %v", s)
	}
	dom := pickDominantRoute(propagateTernary(s, mustSyn()))
	if dom != "route_alset" {
		t.Fatalf("got %s want route_alset", dom)
	}
}

func TestCortexRiskInhibitsScout(t *testing.T) {
	// hard refuse text - may vary; force sense map
	s := map[string]int{"sense_risk": 2, "sense_person": 2}
	routes := propagateTernary(s, mustSyn())
	if routes["route_scout"] != 0 && pickDominantRoute(routes) == "route_scout" {
		t.Fatalf("risk should not yield scout: %v", routes)
	}
	if pickDominantRoute(routes) != "route_refuse" {
		t.Fatalf("got %s", pickDominantRoute(routes))
	}
}

func TestCortexAssistEmptyVoice(t *testing.T) {
	if !cortexShouldAssist("hola", "", "chat") {
		t.Fatal("empty voice should assist")
	}
	if !cortexShouldAssist("dime tu", "algo", "chat") {
		t.Fatal("incomplete should assist")
	}
}

func mustSyn() []ternarySynapse {
	_, syn := defaultTernaryCortex()
	return syn
}

func TestCortexEthicsNotMath(t *testing.T) {
	s := senseFeaturesTernary("explica ethics en estado 2")
	if s["sense_math"] >= 2 {
		t.Fatalf("ethics estado 2 must not be math: %v", s)
	}
	if s["sense_alset"] < 1 {
		t.Fatalf("expected alset sense: %v", s)
	}
	dom := pickDominantRoute(propagateTernary(s, mustSyn()))
	if dom == "route_math" {
		t.Fatalf("got route_math")
	}
}

func TestLooksLikeArithmeticNoExplica(t *testing.T) {
	if looksLikeArithmetic("explica ethics en estado 2") {
		t.Fatal("explica…estado 2 is not arithmetic")
	}
	if !looksLikeArithmetic("12+5") {
		t.Fatal("12+5 should be arithmetic")
	}
}

func TestDestructiveBorres(t *testing.T) {
	if !isDestructiveOrder("no borres todos los archivos del servidor") {
		t.Fatal("borres todos los archivos should be destructive")
	}
}
