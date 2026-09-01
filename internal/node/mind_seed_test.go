package node

import (
	"strings"
	"testing"
)

func TestSeedRoundTripDeterministic(t *testing.T) {
	a := SeedFromText("Hola mundo Alset")
	b := SeedFromText("hola mundo alset")
	if a.Hash == "" || a.Compact == "" {
		t.Fatal("empty seed")
	}
	if a.Hash != b.Hash {
		t.Fatalf("normalize should match hash %s vs %s", a.Hash, b.Hash)
	}
	if a.Conf < 1 {
		t.Fatal("conf")
	}
}

func TestSeedSimilaritySelf(t *testing.T) {
	s := SeedFromText("memoria cid genoma zyrion")
	score, j := SeedSimilarity(s, s)
	if score < 0.99 || j != 2 {
		t.Fatalf("self sim score=%v j=%d", score, j)
	}
}

func TestSeedDifferent(t *testing.T) {
	a := SeedFromText("vender herramientas en valparaiso")
	b := SeedFromText("receta de tortilla de patatas")
	_, j := SeedSimilarity(a, b)
	if j == 2 {
		t.Fatal("unrelated texts should not be judgment 2")
	}
}

func TestSpeakSeed(t *testing.T) {
	v := speakSeed("comprime este texto: el genoma muta con el corpus")
	if v == "" || !strings.Contains(v, "Semilla CFT") {
		t.Fatalf("voice=%q", v)
	}
	if speakSeed("hola") != "" {
		t.Fatal("non-seed intent")
	}
}

func TestRLE(t *testing.T) {
	s := rleTernary([]int{0, 0, 0, 1, 2, 2})
	if s != "0x3 1x1 2x2" {
		t.Fatalf("got %q", s)
	}
}
