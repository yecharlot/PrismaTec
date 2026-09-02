package node

import (
	"strings"
	"testing"
)

func TestHumanListGenes(t *testing.T) {
	for _, in := range []string{"qué genes tienes?", "mis sondas", "sondas activas"} {
		got := normalizeGenUserIntent(in)
		if got != "lista genes" {
			t.Fatalf("%q -> %q", in, got)
		}
	}
}

func TestHumanCreateSonda(t *testing.T) {
	got := normalizeGenUserIntent("crea una sonda llamada aurora")
	if got != "crea gen aurora" {
		t.Fatalf("got %q", got)
	}
}

func TestHumanDispatch(t *testing.T) {
	got := normalizeGenUserIntent("manda la sonda aurora a cloudflare")
	if !strings.Contains(got, "despacha") || !strings.Contains(got, "aurora") {
		t.Fatalf("got %q", got)
	}
}

func TestGenHelp(t *testing.T) {
	v := speakGenHelp("cómo uso las sondas?")
	if v == "" || !strings.Contains(strings.ToLower(v), "sonda") {
		t.Fatalf("help: %q", v)
	}
}

func TestHumanizeVoice(t *testing.T) {
	h := humanizeGenVoice("Despaché «aurora» a la red de borde. Puedes alcanzarlo en https://x.test")
	if strings.Contains(h, "Despaché") {
		t.Fatalf("not humanized: %s", h)
	}
}
