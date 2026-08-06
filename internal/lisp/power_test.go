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

func TestEvaluarZyrion_BasicReject(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	// Both inputs high continuous → ternary 2 → take salida 2
	cmd := `(evaluar-zyrion
		(quote (NODO :entradas (A B) :salidas ((0 RECHAZO) (1 MANUAL) (2 APROBADO))))
		(quote (A 0.9 B 0.8)))`
	got, err := e.Eval(cmd)
	if err != nil {
		t.Fatal(err)
	}
	// A=0.9→2, B=0.8→2 → zyrion all 2 → 2 → APROBADO
	if got != "APROBADO" {
		t.Fatalf("got %#v want APROBADO", got)
	}
}

func TestEvaluarZyrion_AllZero(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	cmd := `(evaluar-zyrion
		(quote (N :entradas (X Y) :salidas ((0 BLOQUEO) (1 OK) (2 RARO))))
		(quote (X 0.1 Y 0.1)))`
	got, err := e.Eval(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if got != "BLOQUEO" {
		t.Fatalf("got %#v want BLOQUEO", got)
	}
}

func TestEvaluarZyrion_Nested(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	cmd := `(evaluar-zyrion
		(quote (PADRE :entradas (A B) :salidas (
			(0 RECHAZO)
			(1 MANUAL)
			(2 (HIJO :entradas (C) :salidas ((0 BAJO) (1 MEDIO) (2 ALTO)))))))
		(quote (A 0.9 B 0.9 C 0.2)))`
	got, err := e.Eval(cmd)
	if err != nil {
		t.Fatal(err)
	}
	// Parent → 2, child C=0.2→0 → BAJO
	if got != "BAJO" {
		t.Fatalf("nested got %#v want BAJO", got)
	}
}

