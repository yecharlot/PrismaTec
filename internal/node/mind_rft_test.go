package node

import (
	"strings"
	"testing"
)

func TestRFTTiempoMemoria(t *testing.T) {
	q := "El tiempo es una ilusión y la memoria es tiempo; entonces qué se deduce"
	v := reasonRFT(q, nil)
	if v == "" {
		t.Fatal("empty RFT")
	}
	low := strings.ToLower(v)
	if !strings.Contains(low, "memoria") || !strings.Contains(low, "ilusión") && !strings.Contains(low, "ilusion") {
		t.Fatalf("expected memoria/ilusión: %q", v)
	}
	if !strings.Contains(low, "salto") || !strings.Contains(low, "rft") {
		t.Fatalf("expected RFT labels: %q", v)
	}
	// salto a realidad no es tiempo (vía ilusión)
	if !strings.Contains(low, "realidad") {
		t.Fatalf("expected salto realidad: %q", v)
	}
}

func TestRFTSocratesStillWorks(t *testing.T) {
	v := reasonAboutQuery("Sócrates es hombre y hombre es mortal; entonces", nil)
	if !strings.Contains(strings.ToLower(v), "mortal") {
		t.Fatalf("%q", v)
	}
}
