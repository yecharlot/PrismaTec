package node

import (
	"strings"
	"testing"
)

func TestDialogActsCorpusLoaded(t *testing.T) {
	e := loadDialogActs()
	if len(e) < 8 {
		t.Fatalf("want >=8 acts, got %d", len(e))
	}
}

func TestDialogActsGreetingNoMenu(t *testing.T) {
	sess := &mindSessionState{Phase: phaseOpening}
	v := speakFromDialogActs("hola, qué tal", sess)
	if v == "" {
		v, _ = speakSpeechAct("hola, qué tal", sess)
	}
	if v == "" {
		t.Fatal("empty")
	}
	low := strings.ToLower(v)
	if strings.Contains(low, "generar código") || strings.Contains(low, "no tengo una respuesta firme") {
		t.Fatalf("menu/unknown: %s", v)
	}
}

func TestDialogActsNaturalnessScore(t *testing.T) {
	ok, total, details := scoreDialogActsNaturalness()
	if total == 0 {
		t.Fatal("no cases")
	}
	if ok*2 < total { // at least 50%
		t.Fatalf("naturalness low %d/%d %v", ok, total, details)
	}
}
