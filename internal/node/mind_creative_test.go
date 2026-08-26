package node

import (
	"strings"
	"testing"
)

func TestThemePrepGrammar(t *testing.T) {
	if themeAsSubject("mar") != "el mar" {
		t.Fatalf("subject mar: %q", themeAsSubject("mar"))
	}
	if themeAsPrepDe("mar") != "del mar" {
		t.Fatalf("de mar: %q", themeAsPrepDe("mar"))
	}
	if themeAsPrepA("mar") != "al mar" {
		t.Fatalf("a mar: %q", themeAsPrepA("mar"))
	}
	if themeAsPrepA("noche") != "a la noche" {
		t.Fatalf("a noche: %q", themeAsPrepA("noche"))
	}
	if !themeLooksPlural("el gen y zyrion") {
		t.Fatal("expected plural compound")
	}
}

func TestAnchorRejectsLispCar(t *testing.T) {
	lisp := "car = primer elemento de la lista; cdr = el resto; cons = construir par/lista."
	if !isBadCreativeAnchor(lisp) {
		t.Fatal("lisp should be bad anchor")
	}
	if anchorFitsTheme("mar", lisp) {
		t.Fatal("mar must not fit lisp car")
	}
	img := naturalThemeImage("mar")
	if img == "" {
		t.Fatal("expected natural image for mar")
	}
	a := pickCreativeAnchor("mar", "", lisp)
	if a == "" || strings.Contains(strings.ToLower(a), "car =") {
		t.Fatalf("anchor for mar should be natural not lisp: %q", a)
	}
}

func TestPoemNoDelMarBug(t *testing.T) {
	out := composePoem("mar", naturalThemeImage("mar"), "verso libre", 1)
	if strings.Contains(out, "Miro del mar") || strings.Contains(out, "de el mar") {
		t.Fatalf("bad grammar: %s", out)
	}
	if !strings.Contains(out, "Miro el mar") {
		t.Fatalf("expected Miro el mar in: %s", out)
	}
	out2 := composePoem("mar", naturalThemeImage("mar"), "anáfora", 3)
	if strings.Contains(out2, "a del mar") || strings.Contains(out2, "Vuelvo a del") {
		t.Fatalf("bad anáfora: %s", out2)
	}
	if !strings.Contains(out2, "Vuelvo al mar") {
		t.Fatalf("expected Vuelvo al mar in: %s", out2)
	}
}
