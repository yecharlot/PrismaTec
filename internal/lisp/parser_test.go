package lisp

import (
	"testing"
)

func TestTokenize_Basic(t *testing.T) {
	tokens := tokenizeLisp(`(+ 1 2)`)
	if len(tokens) < 3 {
		t.Fatalf("tokens = %v", tokens)
	}
	if tokens[0] != "(" || tokens[len(tokens)-1] != ")" {
		t.Fatalf("bad delimiters: %v", tokens)
	}
}

func TestTokenize_IgnoresComments(t *testing.T) {
	tokens := tokenizeLisp("; solo comentario\n(+ 1 2)")
	joined := ""
	for _, tok := range tokens {
		joined += tok
	}
	if len(tokens) == 0 {
		t.Fatal("expected tokens after comment line")
	}
	// no raw comment text as token
	for _, tok := range tokens {
		if len(tok) > 0 && tok[0] == ';' {
			t.Fatalf("comment leaked as token: %q", tok)
		}
	}
}

func TestParse_NumberAndList(t *testing.T) {
	p := NewLispParser("42")
	v, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse number: %v", err)
	}
	if f, ok := v.(float64); !ok || f != 42 {
		t.Fatalf("got %#v", v)
	}

	p = NewLispParser("(+ 1 2)")
	v, err = p.Parse()
	if err != nil {
		t.Fatalf("Parse list: %v", err)
	}
	list, ok := v.(LispList)
	if !ok || len(list) != 3 {
		t.Fatalf("got %#v", v)
	}
	if sym, ok := list[0].(LispSymbol); !ok || string(sym) != "+" {
		t.Fatalf("first element = %#v", list[0])
	}
}

func TestParse_String(t *testing.T) {
	p := NewLispParser(`"hola"`)
	v, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse string: %v", err)
	}
	s, ok := v.(string)
	if !ok || s != "hola" {
		t.Fatalf("got %#v", v)
	}
}

func TestEnvironment_LookupSet(t *testing.T) {
	env := NewLispEnvironment(nil)
	env.Set(LispSymbol("x"), float64(10))
	val, ok := env.Lookup(LispSymbol("x"))
	if !ok || val.(float64) != 10 {
		t.Fatalf("Lookup = %#v ok=%v", val, ok)
	}
	_, ok = env.Lookup(LispSymbol("y"))
	if ok {
		t.Fatal("missing symbol should not be found")
	}
}

func TestEnvironment_ParentShadow(t *testing.T) {
	parent := NewLispEnvironment(nil)
	parent.Set(LispSymbol("a"), float64(1))
	child := NewLispEnvironment(parent)
	child.Set(LispSymbol("a"), float64(2))
	val, _ := child.Lookup(LispSymbol("a"))
	if val.(float64) != 2 {
		t.Fatalf("child shadow = %#v", val)
	}
	val, _ = parent.Lookup(LispSymbol("a"))
	if val.(float64) != 1 {
		t.Fatalf("parent = %#v", val)
	}
}
