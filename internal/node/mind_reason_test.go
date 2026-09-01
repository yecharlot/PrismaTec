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

func TestParseNoPollutedObj(t *testing.T) {
	f, ok := parseRelationFact("Sócrates es hombre. Hombre es mortal. Por transitividad de es, Sócrates es mortal.", 2, "t")
	if !ok {
		t.Fatal("parse fail")
	}
	if strings.Contains(f.Obj, "transitividad") || strings.Contains(f.Obj, "hombre es") {
		t.Fatalf("polluted obj: %q", f.Obj)
	}
	if f.Obj != "hombre" {
		t.Fatalf("obj=%q want hombre", f.Obj)
	}
}

func TestSilogismoNotWrongDeduction(t *testing.T) {
	if isReasoningRequest("qué es un silogismo") {
		t.Fatal("definition should not be reasoning request")
	}
	if softReasonFromKnowledge("qué es un silogismo") != "" {
		t.Fatal("soft reason should not fire on definition")
	}
}

func TestReasonAboutQueryChain(t *testing.T) {
	v := reasonAboutQuery("Sócrates es hombre y hombre es mortal; entonces qué se deduce", nil)
	if v == "" || !strings.Contains(strings.ToLower(v), "mortal") {
		t.Fatalf("voice=%q", v)
	}
	if strings.Contains(strings.ToLower(v), "transitividad de es") && strings.Contains(v, "Premisa") && strings.Count(v, "transitividad") > 0 {
		// premise line should not include meta
		for _, line := range strings.Split(v, "\n") {
			if strings.Contains(line, "Premisa") && strings.Contains(line, "transitividad") {
				t.Fatalf("meta in premise: %s", line)
			}
		}
	}
}

func TestReasonImplicaQuery(t *testing.T) {
	v := reasonAboutQuery("lluvia implica suelo mojado y suelo mojado implica barro; entonces", nil)
	if !strings.Contains(strings.ToLower(v), "barro") {
		t.Fatalf("voice=%q", v)
	}
}

func TestExtractClausesGrammar(t *testing.T) {
	parts := extractClauses("Sócrates es hombre. Hombre es mortal.")
	if len(parts) < 2 {
		t.Fatalf("parts=%v", parts)
	}
}

func TestReasonAnchorWeaves(t *testing.T) {
	body := "Verso sobre el mortal."
	out := weaveReasonIntoCreative(body, "sócrates es mortal")
	if !strings.Contains(out, "sócrates es mortal") {
		t.Fatalf("%q", out)
	}
}

func TestReasonAnchorForThemeCID(t *testing.T) {
	a := reasonAnchorForTheme("cid", "CID es identificador y identificador es dirección por hash", nil)
	if a == "" {
		t.Fatal("empty anchor")
	}
	if !strings.Contains(strings.ToLower(a), "cid") && !strings.Contains(strings.ToLower(a), "hash") {
		t.Fatalf("anchor=%q", a)
	}
}

func TestTransWithArticles(t *testing.T) {
	facts := []ternaryFact{
		{Subj: "el tiempo", Rel: "es_un", Obj: "ilusión", Conf: 2, Src: "usuario"},
		{Subj: "la memoria", Rel: "es", Obj: "tiempo", Conf: 2, Src: "usuario"},
	}
	// after normalize in parse, subj may already be stripped — also test raw
	d := deduceAll(facts)
	found := false
	for _, x := range d {
		if strings.Contains(x.Subj, "memoria") && (strings.Contains(x.Obj, "ilusión") || strings.Contains(x.Obj, "ilusion")) {
			found = true
		}
	}
	if !found {
		// try normalized forms
		facts2 := []ternaryFact{
			{Subj: "tiempo", Rel: "es", Obj: "ilusión", Conf: 2, Src: "usuario"},
			{Subj: "memoria", Rel: "es", Obj: "tiempo", Conf: 2, Src: "usuario"},
		}
		d = deduceAll(facts2)
		for _, x := range d {
			if x.Subj == "memoria" && (x.Obj == "ilusión" || x.Obj == "ilusion") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected memoria→ilusión: %+v", d)
	}
}

func TestExtractClausesCommaPremises(t *testing.T) {
	cs := extractClauses("el perro es un animal, pepe es un perro entonces que deduces")
	if len(cs) < 2 {
		t.Fatalf("want >=2 clauses, got %v", cs)
	}
	var facts []ternaryFact
	for _, c := range cs {
		if f, ok := parseRelationFact(c, 2, "usuario"); ok {
			facts = append(facts, f)
		}
	}
	if len(facts) < 2 {
		t.Fatalf("want 2 facts, got %#v from clauses %#v", facts, cs)
	}
	der := deduceAll(facts)
	found := false
	for _, f := range der {
		if termsMatch(f.Subj, "pepe") && termsMatch(f.Obj, "animal") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want pepe→animal, derived=%#v", der)
	}
}

func TestUniversalSyllogismHumanos(t *testing.T) {
	q := "si todos los humanos son mortales y Sócrates es humano entonces"
	got := reasonAboutQuery(q, nil)
	if got == "" {
		t.Fatal("empty reason")
	}
	low := strings.ToLower(got)
	if !(strings.Contains(low, "mortal") || strings.Contains(low, "sócrates") || strings.Contains(low, "socrates") || strings.Contains(low, "premisa")) {
		t.Fatalf("unexpected: %s", got)
	}
}
