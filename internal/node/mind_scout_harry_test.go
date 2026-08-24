package node

import (
	"strings"
	"testing"
)

func TestScoutableQuienEsYBusca(t *testing.T) {
	cases := []string{
		"quien es harry potter",
		"quién es harry potter",
		"busca harry potter",
		"busca lebron james",
		"quien es lebron james",
		"investiga teneso",
	}
	for _, c := range cases {
		n := normalizeUserInput(c)
		if !isScoutableQuestion(n) {
			t.Errorf("expected scoutable: %q → %q", c, n)
		}
		if isWorldFact(n) {
			t.Errorf("must NOT be world-fact: %q → %q", c, n)
		}
		if !forceWebScout(n) && (strings.HasPrefix(n, "busca ") || strings.HasPrefix(n, "quién es ") || strings.HasPrefix(n, "quien es ")) {
			t.Errorf("forceWebScout expected for %q", n)
		}
	}
}

func TestWorldFactStillWorks(t *testing.T) {
	if !isWorldFact("el cielo es azul por la dispersión de la luz") {
		t.Fatal("expected world fact")
	}
}
