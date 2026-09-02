package node

import (
	"strings"
	"testing"
)

func TestNodeNormalizeStatus(t *testing.T) {
	got := normalizeNodeUserIntent("mira el nodo por favor")
	if !strings.Contains(got, "estado") && got == "mira el nodo por favor" {
		// should rewrite
		if got == "mira el nodo por favor" {
			t.Fatalf("expected rewrite, got %q", got)
		}
	}
	got = normalizeNodeUserIntent("mira el nodo")
	if got != "dame estado del nodo" {
		t.Fatalf("got %q", got)
	}
}

func TestOperatorHelp(t *testing.T) {
	v := speakOperatorHelp("qué puedo hacer con el nodo")
	if v == "" || !strings.Contains(strings.ToLower(v), "sonda") {
		t.Fatalf("help: %q", v)
	}
}
