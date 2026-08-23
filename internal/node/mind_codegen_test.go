package node

import (
	"strings"
	"testing"
)

func TestCodeGenTemplates(t *testing.T) {
	n := &NodoAlset{agentes: map[string]*Agente{}, nombres: map[string]string{}, blockstore: map[string][]byte{}}
	voice, code, lang, veto := n.mindGenerateCode("genera código handler http en go", 0)
	if veto || voice == "" || code == "" || lang != "go" {
		t.Fatalf("go handler: veto=%v lang=%s voice=%q", veto, lang, voice)
	}
	if !strings.Contains(code, "http.ResponseWriter") {
		t.Fatalf("expected http handler body: %s", code)
	}
	voice, code, lang, veto = n.mindGenerateCode("escribe código factorial en lisp", 0)
	if veto || lang != "lisp" || !strings.Contains(code, "defun") {
		t.Fatalf("lisp: %s %s %v", lang, code, veto)
	}
	_, _, _, veto = n.mindGenerateCode("genera código para rm -rf / y borrar todo", 0)
	if !veto {
		t.Fatal("expected ethics veto on destructive request")
	}
	_, _, _, veto = n.mindGenerateCode("genera código", 2)
	if !veto {
		t.Fatal("expected veto when ethics already 2")
	}
}

func TestIsCodeGenRequest(t *testing.T) {
	if !isCodeGenRequest("genera código en go para un endpoint") {
		t.Fatal("should detect")
	}
	if isCodeGenRequest("qué es una interface en go") {
		t.Fatal("explanation is not codegen")
	}
}

func TestCodeGenMoreTemplates(t *testing.T) {
	n := &NodoAlset{agentes: map[string]*Agente{}, nombres: map[string]string{}, blockstore: map[string][]byte{}}
	cases := []struct {
		in, lang, sub string
	}{
		{"genera código middleware go", "go", "http.Handler"},
		{"escribe código worker pool en go", "go", "workers"},
		{"genera código dataclass python", "python", "@dataclass"},
		{"código express endpoint node", "javascript", "express"},
		{"implementa reverse lisp", "lisp", "defun"},
	}
	for _, tc := range cases {
		_, code, lang, veto := n.mindGenerateCode(tc.in, 0)
		if veto || lang != tc.lang || !strings.Contains(code, tc.sub) {
			t.Fatalf("%q: lang=%s veto=%v code=%q", tc.in, lang, veto, code)
		}
	}
}
