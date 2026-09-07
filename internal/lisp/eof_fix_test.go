package lisp

import (
	"strings"
	"testing"
)

func TestEval_NoRawEOF(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	cases := []string{
		"",
		"(",
		"(+ 1",
		"(list 1 2",
		"\"hola",
	}
	for _, c := range cases {
		_, err := e.Eval(c)
		if err == nil {
			t.Fatalf("expected error for %q", c)
		}
		if err.Error() == "EOF" || err.Error() == "unexpected EOF" {
			t.Fatalf("raw EOF for %q: %v", c, err)
		}
	}
}

func TestEval_BasicsAndQuote(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	for _, c := range []struct {
		code string
		want float64
	}{
		{"(+ 1 2 3)", 6},
		{"(* 6 7)", 42},
		{"(- 10 3)", 7},
	} {
		got, err := e.Eval(c.code)
		if err != nil {
			t.Fatalf("%s: %v", c.code, err)
		}
		if got != c.want {
			t.Fatalf("%s = %#v want %v", c.code, got, c.want)
		}
	}
	q, err := e.Eval("'(1 2)")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(LispList); !ok {
		t.Fatalf("quote list: %#v", q)
	}
}

func TestExportJSON(t *testing.T) {
	v := ExportJSON(LispList{LispSymbol("a"), float64(1)})
	arr, ok := v.([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("%#v", v)
	}
	if arr[0] != "a" {
		t.Fatalf("symbol export: %#v", arr[0])
	}
}

func TestEval_ZyrionAndList(t *testing.T) {
	e := NewEvaluator(&mockHost{})
	got, err := e.Eval("(list 1 2 3)")
	if err != nil {
		t.Fatal(err)
	}
	if ExportJSON(got) == nil {
		t.Fatal("nil export")
	}
	_, err = e.Eval("(zyrion (list 1 1 0))")
	if err != nil && strings.Contains(err.Error(), "EOF") {
		t.Fatalf("eof: %v", err)
	}
}
