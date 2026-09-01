package node

import (
	"strings"
	"testing"
)

// Batería de conversación (secuencia humana), no solo dominios aislados.
func TestConversationSequenceSocial(t *testing.T) {
	sess := &mindSessionState{Phase: phaseOpening, TurnCount: 0}
	steps := []struct {
		in   string
		must []string
		ban  []string
	}{
		{"hola, qué tal", []string{"bien", "aquí", "hola", "tal", "presente", "hablamos"}, []string{"generar código", "no tengo una respuesta firme"}},
		{"me llamo Rita", []string{}, []string{"generar código"}},
		{"cómo me llamo?", []string{}, []string{"generar código", "no tengo una respuesta firme"}},
		{"hasta luego", []string{"luego", "aquí", "retomar", "vaya"}, []string{"generar código"}},
	}
	for _, st := range steps {
		v, _ := speakSpeechAct(st.in, sess)
		updateSessionAfterTurn(sess, st.in, "chat")
		if v == "" && classifySpeechAct(st.in) == actSocial {
			t.Fatalf("empty social voice for %q", st.in)
		}
		low := strings.ToLower(v)
		for _, b := range st.ban {
			if v != "" && strings.Contains(low, b) {
				t.Fatalf("ban %q in %q → %q", b, st.in, v)
			}
		}
		if len(st.must) > 0 && v != "" {
			ok := false
			for _, m := range st.must {
				if strings.Contains(low, m) {
					ok = true
					break
				}
			}
			if !ok {
				t.Fatalf("expected one of %v in %q", st.must, v)
			}
		}
	}
	if sess.Phase != phaseClosing {
		t.Fatalf("phase=%s want closing", sess.Phase)
	}
}

func TestSyllogismEntoncesTail(t *testing.T) {
	q := "si todos los humanos son mortales y Sócrates es humano entonces"
	got := reasonAboutQuery(q, nil)
	low := strings.ToLower(got)
	if strings.Contains(low, "humano entonces") {
		t.Fatalf("tail leak: %s", got)
	}
	if !(strings.Contains(low, "mortal") || strings.Contains(low, "sócrates") || strings.Contains(low, "socrates")) {
		t.Fatalf("weak reason: %s", got)
	}
}
