package node

import "testing"

func TestNormalizeGenList(t *testing.T) {
	got := normalizeGenUserIntent("qué genes tienes activos?")
	if got != "lista genes" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeGenCreate(t *testing.T) {
	got := normalizeGenUserIntent("crea una sonda llamada aurora")
	if got != "crea gen aurora" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeGenExplore(t *testing.T) {
	got := normalizeGenUserIntent("manda una sonda a explorar https://example.com")
	if !containsStr(got, "explorar") && !containsStr(got, "explora") {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeKeepsLab(t *testing.T) {
	in := "crea gen genesis"
	if normalizeGenUserIntent(in) != in {
		t.Fatal("lab command rewritten")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexStr(s, sub) >= 0)
}
func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
