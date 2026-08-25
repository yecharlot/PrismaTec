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
		"quien es michel jordan":                 "michael jordan",
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

func TestScoutFindingRecall(t *testing.T) {
	storeScoutFinding("harry potter", "Harry Potter: serie de novelas…")
	_, report, ok := recallScoutFinding("quien es harry potter")
	if !ok || !strings.Contains(report, "Harry Potter") {
		t.Fatalf("recall failed: ok=%v report=%q", ok, report)
	}
	// different topic must not soft-collide on short tokens only
	_, _, ok2 := recallScoutFinding("lord voldemort")
	if ok2 {
		t.Fatal("must not recall harry for voldemort")
	}
}
