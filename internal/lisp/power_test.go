package lisp

import (
	"testing"
)

func TestPower_EmbeddingAndTernary(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	emb, err := e.Eval(`(embedding "hola")`)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := emb.(LispList)
	if !ok || len(list) != 32 {
		t.Fatalf("embedding len = %v", emb)
	}
	ter, err := e.Eval(`(ternarizar (embedding "hola"))`)
	if err != nil {
		t.Fatal(err)
	}
	tl, ok := ter.(LispList)
	if !ok || len(tl) != 32 {
		t.Fatalf("ternarizar = %#v", ter)
	}
	for _, v := range tl {
		f := toFloat(v)
		if f != 0 && f != 1 && f != 2 {
			t.Fatalf("non-ternary value %v", f)
		}
	}
}

func TestPower_HostIDAndPeers(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	id, err := e.Eval(`(host-id)`)
	if err != nil {
		t.Fatal(err)
	}
	if id != "peer-test" {
		t.Fatalf("host-id = %v", id)
	}
	n, err := e.Eval(`(net-peers)`)
	if err != nil {
		t.Fatal(err)
	}
	if toFloat(n) != 0 {
		t.Fatalf("net-peers = %v", n)
	}
}

func TestPower_SimilitudSameText(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	got, err := e.Eval(`(similitud "red" "red")`)
	if err != nil {
		t.Fatal(err)
	}
	if toFloat(got) != 1.0 {
		t.Fatalf("similitud same text = %v, want 1", got)
	}
}

func TestPower_ElementoAndRange(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	got, err := e.Eval(`(elemento (list 10 20 30) 1)`)
	if err != nil {
		t.Fatal(err)
	}
	if toFloat(got) != 20 {
		t.Fatalf("elemento = %v", got)
	}
	r, err := e.Eval(`(range 3)`)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := r.(LispList)
	if !ok || len(list) != 3 {
		t.Fatalf("range = %#v", r)
	}
}

func TestPower_CrearCapaYModelo(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	// mock GenerarCID always returns same cid-test; still should not error
	got, err := e.Eval(`(crear-capa-lineal "capa-t" 4 2)`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "capa-t" {
		t.Fatalf("crear-capa-lineal = %v", got)
	}
	got, err = e.Eval(`(crear-modelo "mod-t" (list "capa-t"))`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "mod-t" {
		t.Fatalf("crear-modelo = %v", got)
	}
}
