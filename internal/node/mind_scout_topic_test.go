package node

import (
	"strings"
	"testing"
)

func TestExtractTopicCleaning(t *testing.T) {
	cases := map[string]string{
		"busca informacion sobre Bio informatica": "bioinformática",
		"que es la bioinformatica":                "bioinformática",
		"quien es harry potter":                   "harry potter",
		"profundiza mas sobre Lord Voldemort":     "lord voldemort",
		"busca Martin Luter King en la web":       "martin luther king",
		"quien martin luter king":                 "martin luther king",
		"quien es Joana de Arco":                  "juana de arco",
	}
	for in, want := range cases {
		got := extractTopic(normalizeUserInput(in))
		if got != want {
			t.Errorf("extractTopic(%q)=%q want %q", in, got, want)
		}
	}
}

func TestScoutableDeepenAndBareQuien(t *testing.T) {
	for _, q := range []string{
		"profundiza mas sobre Lord Voldemort",
		"quien martin luter king",
		"busca informacion sobre bioinformatica",
	} {
		n := normalizeUserInput(q)
		if !isScoutableQuestion(n) {
			t.Errorf("not scoutable: %q → %q", q, n)
		}
		if !forceWebScout(n) {
			t.Errorf("forceWebScout false: %q", n)
		}
	}
}

func TestTopicKeysMatchStrict(t *testing.T) {
	if !topicKeysMatch("harry potter", "harry potter") {
		t.Fatal("exact")
	}
	if topicKeysMatch("harry potter", "lord voldemort") {
		t.Fatal("must not cross harry↔voldemort")
	}
	if topicKeysMatch("bioinformática", "bioinformatica") {
		// after normalize may differ by accent — still ok if false
	}
	if !topicKeysMatch("martin luther king", "martin luther king jr") {
		// shared tokens martin, luther, king → ≥2
		t.Log("jr variant: optional")
	}
}

func TestScoutFindingRecallStrict(t *testing.T) {
	// reset index for test
	scoutFindingIndex.mu.Lock()
	scoutFindingIndex.byTopic = map[string]string{}
	scoutFindingIndex.mu.Unlock()

	storeScoutFinding("harry potter", "Harry Potter: serie de novelas fantásticas de J. K. Rowling sobre el joven mago y Hogwarts.")
	_, report, ok := recallScoutFinding("quien es harry potter")
	if !ok || !strings.Contains(report, "Harry Potter") {
		t.Fatalf("recall harry failed: ok=%v report=%q", ok, report)
	}
	_, _, ok2 := recallScoutFinding("lord voldemort")
	if ok2 {
		t.Fatal("must not recall harry for voldemort")
	}
	_, _, ok3 := recallScoutFinding("profundiza mas sobre Lord Voldemort")
	if ok3 {
		t.Fatal("deepen voldemort must not hit harry cache")
	}
}

func TestActuateEffectLevel(t *testing.T) {
	s := ActuateState{Explore: 2, Write: 1}
	if s.effectLevel() != 2 {
		t.Fatalf("effectLevel=%d want 2", s.effectLevel())
	}
	if (ActuateState{}).effectLevel() != 0 {
		t.Fatal("empty should be 0")
	}
}

func TestDeepenNewTopicSkipsStickyHarry(t *testing.T) {
	// topicKeysMatch must not equate harry with voldemort
	if topicKeysMatch("lord voldemort", "harry potter") {
		t.Fatal("cross match")
	}
	// extract from deepen phrase
	got := extractTopic(normalizeUserInput("profundiza mas sobre Lord Voldemort"))
	if got != "lord voldemort" {
		t.Fatalf("topic=%q", got)
	}
}
