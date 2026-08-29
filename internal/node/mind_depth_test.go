package node

import (
	"strings"
	"testing"
)

func TestDeepenKnowledge(t *testing.T) {
	got := deepenKnowledgeAnswer("qué es la democracia", "Democracia es participación ciudadana.")
	if !strings.Contains(got, "Democracia") {
		t.Fatal(got)
	}
	if len(got) < 40 {
		t.Fatal("expected depth add-on")
	}
}

func TestUnknownTopicVoice(t *testing.T) {
	v := unknownTopicVoice("qué es el flubber cristalino?")
	if strings.Contains(strings.ToLower(v), "campo en seguir") {
		t.Fatal(v)
	}
	if !strings.Contains(strings.ToLower(v), "corpus") && !strings.Contains(strings.ToLower(v), "sonda") {
		t.Fatal(v)
	}
}

func TestEnrichThinChat(t *testing.T) {
	v := enrichDeepTurn("chat", "qué es xyzzy-desconocido-123?", "Te leo. Campo en seguir.")
	if strings.Contains(strings.ToLower(v), "campo en seguir") {
		t.Fatal(v)
	}
}
