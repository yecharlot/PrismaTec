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
	if !strings.Contains(low, "memoria") {
		t.Fatalf("expected memoria: %q", v)
	}
	if !strings.Contains(low, "ilusión") && !strings.Contains(low, "ilusion") {
		t.Fatalf("expected ilusión: %q", v)
	}
	if strings.Contains(low, "sócrates") || strings.Contains(low, "socrates") || strings.Contains(low, "lisp") {
		t.Fatalf("corpus flood in RFT: %q", v)
	}
	if strings.Count(low, "[l0 premisa") > 6 {
		t.Fatalf("too many L0 premises: %q", v)
	}
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
