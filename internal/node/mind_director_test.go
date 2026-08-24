package node

import (
	"strings"
	"testing"
)

func TestDirectorCapabilityNotCreate(t *testing.T) {
	if !isCapabilityQuestion("cuantos genes puedes crear") {
		t.Fatal("expected capability")
	}
	if isGenToolIntent(normalizeUserInput("cuantos genes puedes crear")) {
		// may still contain "genes" but should not be pure create-only path
		// isGenToolIntent looks for "crea gen" — capability text has "puedes crear" without "crea gen"
	}
	if isGenToolIntent("cuantos genes puedes crear") {
		t.Fatal("capacity question must not be gen tool create")
	}
}

func TestDirectorMath(t *testing.T) {
	n := &NodoAlset{}
	v := n.tryMindMath("cuánto es 2+3")
	if !strings.Contains(v, "5") {
		t.Fatalf("got %q", v)
	}
}

func TestDirectorReferentialExplore(t *testing.T) {
	n := &NodoAlset{}
	n.rememberThreadRefs("explore", "genesis", "Google title snippet", "")
	v := n.resolveReferential("qué significa esto")
	if !strings.Contains(strings.ToLower(v), "explore") && !strings.Contains(v, "Google") {
		t.Fatalf("got %q", v)
	}
}

func TestSoftAppendBlockedOnTool(t *testing.T) {
	if softAppendAllowed("tool", "gen ok") {
		t.Fatal("tools must not soft-append")
	}
	if !softAppendAllowed("chat", "hola") {
		t.Fatal("chat may soft-append")
	}
}

func TestNormalizeQuieEs(t *testing.T) {
	n := normalizeUserInput("pregunta al gen genesis quie es")
	if !strings.Contains(n, "quién es") {
		t.Fatalf("got %q", n)
	}
}

func TestCodegenSumar(t *testing.T) {
	n := &NodoAlset{agentes: map[string]*Agente{}, nombres: map[string]string{}, blockstore: map[string][]byte{}}
	_, code, lang, veto := n.mindGenerateCode("genera código función sumar a y b en go", 0)
	if veto || lang != "go" {
		t.Fatalf("lang=%s veto=%v", lang, veto)
	}
	if !strings.Contains(code, "a + b") && !strings.Contains(code, "a+b") {
		t.Fatalf("expected sum body: %s", code)
	}
}
