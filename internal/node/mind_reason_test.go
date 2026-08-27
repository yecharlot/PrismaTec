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
	d := deduceAll(facts)
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

func TestDeduceImplicaChain(t *testing.T) {
	facts := []ternaryFact{
		{Subj: "lluvia", Rel: "implica", Obj: "suelo mojado", Conf: 2, Src: "t"},
		{Subj: "suelo mojado", Rel: "implica", Obj: "barro", Conf: 2, Src: "t"},
	}
	d := deduceAll(facts)
	ok := false
	for _, x := range d {
		if x.Subj == "lluvia" && x.Obj == "barro" && x.Rel == "implica" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("lluvia implica barro: %+v", d)
	}
}

func TestDeduceEsMasTiene(t *testing.T) {
	facts := []ternaryFact{
		{Subj: "gen", Rel: "es", Obj: "célula", Conf: 2, Src: "t"},
		{Subj: "célula", Rel: "tiene", Obj: "rootcid", Conf: 2, Src: "t"},
	}
	d := deduceAll(facts)
	ok := false
	for _, x := range d {
		if x.Subj == "gen" && x.Obj == "rootcid" && x.Rel == "tiene" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("gen tiene rootcid: %+v", d)
	}
}

func TestReasonAboutQueryChain(t *testing.T) {
	v := reasonAboutQuery("Sócrates es hombre y hombre es mortal; entonces qué se deduce", nil)
	if v == "" || !strings.Contains(strings.ToLower(v), "mortal") {
		t.Fatalf("voice=%q", v)
	}
}

func TestReasonImplicaQuery(t *testing.T) {
	v := reasonAboutQuery("lluvia implica suelo mojado y suelo mojado implica barro; entonces", nil)
	if !strings.Contains(strings.ToLower(v), "barro") {
		t.Fatalf("voice=%q", v)
	}
}
