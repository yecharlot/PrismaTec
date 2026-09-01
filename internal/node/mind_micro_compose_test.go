package node

import (
	"strings"
	"testing"
)

func TestMicroComposeDoesNotStealMemory(t *testing.T) {
	sess := &mindSessionState{Phase: phaseOngoing, TurnCount: 3}
	profile := UserProfile{Nombre: "Laura"}
	v := microComposeChat("cómo me llamo?", sess, profile, nil)
	if v != "" {
		t.Fatalf("must not steal memory query: %q", v)
	}
	v = microComposeChat("y dónde vivo?", sess, profile, nil)
	if v != "" {
		t.Fatalf("must not steal place query: %q", v)
	}
	v = microComposeChat("en qué te diferencias de un chatgpt?", sess, profile, nil)
	if v != "" {
		t.Fatalf("must not steal vs-llm: %q", v)
	}
}

func TestConfirmIsSocial(t *testing.T) {
	if classifySpeechAct("ok") != actSocial {
		t.Fatal("ok should be social")
	}
	v, _ := speakSpeechAct("ok", &mindSessionState{Phase: phaseOngoing, TurnCount: 2})
	if v == "" || strings.Contains(strings.ToLower(v), "no distinguí") {
		t.Fatalf("bad ok: %q", v)
	}
}
