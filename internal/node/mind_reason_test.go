package node

import (
	"strings"
	"testing"
)

func TestDeduceTransitive(t *testing.T) {
	facts := []ternaryFact{
		{Subj: "socrates", Rel: "es", Obj: "hombre", Conf: 2, Src: "test"},
		{Subj: "hombre", Rel: "es", Obj: "mortal", Conf: 2, Src: "test"},
	}
	d := deduceTransitive(facts)
	if len(d) == 0 {
		t.Fatal("expected deduction")
	}
	found := false
	for _, x := range d {
		if x.Subj == "socrates" && x.Obj == "mortal" && x.Conf == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("socrates mortal not found: %+v", d)
	}
}

func TestReasonAboutQueryChain(t *testing.T) {
	v := reasonAboutQuery("Sócrates es hombre y hombre es mortal; entonces qué se deduce", nil)
	if v == "" || !strings.Contains(strings.ToLower(v), "mortal") {
		t.Fatalf("voice=%q", v)
	}
	if !strings.Contains(strings.ToLower(v), "deducción") {
		t.Fatalf("expected deduction label: %q", v)
	}
}
