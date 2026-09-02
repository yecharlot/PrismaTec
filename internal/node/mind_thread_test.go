package node

import (
	"strings"
	"testing"
)

func TestThreadReferenceEso(t *testing.T) {
	sess := &mindSessionState{Phase: phaseOngoing, ActiveTopic: "valparaíso", LastUserFrame: "vivo en valparaíso", TurnCount: 3}
	v := speakThreadReference("y eso?", sess)
	if v == "" || !strings.Contains(strings.ToLower(v), "valpara") {
		t.Fatalf("ref: %q", v)
	}
}

func TestThreadMisunderstand(t *testing.T) {
	sess := &mindSessionState{Phase: phaseOngoing, ActiveTopic: "x"}
	v := speakThreadReference("no me refería a eso", sess)
	if v == "" || sess.Expect != expectClarify {
		t.Fatalf("repair: %q expect=%s", v, sess.Expect)
	}
}

func TestThreadScore(t *testing.T) {
	ok, total, _ := scoreDialogThreadNaturalness()
	if total == 0 || ok == 0 {
		t.Fatalf("thread score %d/%d", ok, total)
	}
}

func TestExtractTopicFrame(t *testing.T) {
	f := extractTopicFrame("vamos a hablar de genes en la red")
	if f == "" {
		t.Fatal("empty frame")
	}
}
