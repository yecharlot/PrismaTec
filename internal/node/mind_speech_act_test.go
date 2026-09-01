package node

import (
	"strings"
	"testing"
)

func TestSpeechActGreeting(t *testing.T) {
	if classifySpeechAct("hola, qué tal") != actSocial {
		t.Fatal("expected social")
	}
	v, k := speakSpeechAct("hola, qué tal", &mindSessionState{Phase: phaseOpening})
	if v == "" || k != "chat" {
		t.Fatalf("voice=%q kind=%q", v, k)
	}
	low := strings.ToLower(v)
	if strings.Contains(low, "generar código") || strings.Contains(low, "cálculo, código") {
		t.Fatalf("menu leak: %s", v)
	}
}

func TestSpeechActBye(t *testing.T) {
	sess := &mindSessionState{Phase: phaseOngoing, TurnCount: 3}
	v, _ := speakSpeechAct("hasta luego", sess)
	if v == "" || sess.Phase != phaseClosing {
		t.Fatalf("bye phase=%s v=%q", sess.Phase, v)
	}
}

func TestSpeechActNotSocialContent(t *testing.T) {
	if classifySpeechAct("qué es zyrion") == actSocial {
		t.Fatal("zyrion is not pure social")
	}
	v, _ := speakSpeechAct("qué es zyrion", nil)
	if v != "" {
		t.Fatal("should not speak speech-act for content")
	}
}

func TestSpeechActHowAreYou(t *testing.T) {
	v, _ := speakSpeechAct("cómo estás", &mindSessionState{TurnCount: 0})
	if v == "" {
		t.Fatal("empty")
	}
	if strings.Contains(strings.ToLower(v), "no tengo una respuesta firme") {
		t.Fatal("unknown path")
	}
}
