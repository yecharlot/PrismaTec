package node

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Fase 5 — generación de código por plantillas + ethics + CID.
// No LLM: solo esqueletos curados y relleno de slots.

type codeTemplate struct {
	ID   string
	Lang string
	Keys []string
	Body string
}

var codeTemplates = []codeTemplate{
	{
		ID: "go_http_handler", Lang: "go",
		Keys: []string{"handler http", "http handler", "servidor http go", "endpoint go", "api go básica", "hola mundo go http", "mux go"},
		Body: `package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func {{handler}}(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"ok":   "true",
		"path": r.URL.Path,
	})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/{{route}}", {{handler}})
	log.Println("listen :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
`,
	},
	{
		ID: "go_middleware", Lang: "go",
		Keys: []string{"middleware go", "middleware http", "logging middleware"},
		Body: `package {{pkg}}

import (
	"log"
	"net/http"
	"time"
)

func {{handler}}(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
`,
	},
	{
		ID: "go_func_test", Lang: "go",
		Keys: []string{"test go", "table driven", "test table", "unit test go", "testing go"},
		Body: `package {{pkg}}

import "testing"

func Test{{Name}}(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"zero", 0, 0},
		{"one", 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := {{fn}}(tc.in)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
`,
	},
		{
		ID: "go_sum_ab", Lang: "go",
		Keys: []string{"sumar", "función sumar", "funcion sumar", "suma a y b", "sumar a y b", "func sumar"},
		Body: `package {{pkg}}

func {{fn}}(a, b int) int {
	return a + b
}
`,
	},
	{
		ID: "py_sum_ab", Lang: "python",
		Keys: []string{"sumar python", "def sumar", "suma en python"},
		Body: `def {{fn}}(a, b):
	return a + b
`,
	},
	{
		ID: "go_struct_json", Lang: "go",
		Keys: []string{"struct json", "json go", "marshal struct", "struct go"},
		Body: `package {{pkg}}

type {{Name}} struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}
`,
	},
	{
		ID: "go_worker_pool", Lang: "go",
		Keys: []string{"worker pool", "goroutine pool", "pool de workers", "fan out go"},
		Body: `package {{pkg}}

func {{fn}}(jobs <-chan int, results chan<- int, workers int) {
	for i := 0; i < workers; i++ {
		go func() {
			for j := range jobs {
				results <- j * j
			}
		}()
	}
}
`,
	},
	{
		ID: "go_context_timeout", Lang: "go",
		Keys: []string{"context timeout", "withtimeout go", "deadline go"},
		Body: `package {{pkg}}

import (
	"context"
	"time"
)

func {{fn}}(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}
`,
	},
	{
		ID: "go_crud_memory", Lang: "go",
		Keys: []string{"crud go", "mapa en memoria", "rest crud go", "store in memory"},
		Body: `package {{pkg}}

import "sync"

type {{Name}} struct {
	ID   string
	Name string
}

type Store struct {
	mu   sync.RWMutex
	data map[string]{{Name}}
}

func NewStore() *Store {
	return &Store{data: make(map[string]{{Name}})}
}

func (s *Store) Put(x {{Name}}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[x.ID] = x
}

func (s *Store) Get(id string) ({{Name}}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	x, ok := s.data[id]
	return x, ok
}
`,
	},
	{
		ID: "lisp_defun", Lang: "lisp",
		Keys: []string{"defun", "función lisp", "funcion lisp", "definir función lisp"},
		Body: `(defun {{fn}} (x)
  "Doc: {{fn}} sobre x."
  (cond
    ((null x) nil)
    (t (cons (car x) ({{fn}} (cdr x))))))
`,
	},
	{
		ID: "lisp_filter", Lang: "lisp",
		Keys: []string{"filtrar lista", "filter lisp", "remove-if-not"},
		Body: `(defun filtrar (pred lst)
  (cond
    ((null lst) nil)
    ((funcall pred (car lst))
     (cons (car lst) (filtrar pred (cdr lst))))
    (t (filtrar pred (cdr lst)))))
`,
	},
	{
		ID: "lisp_factorial", Lang: "lisp",
		Keys: []string{"factorial lisp", "factorial en lisp"},
		Body: `(defun factorial (n)
  (if (<= n 1)
      1
      (* n (factorial (- n 1)))))
`,
	},
	{
		ID: "lisp_mapcar", Lang: "lisp",
		Keys: []string{"mapcar", "mapear lista lisp"},
		Body: `(defun {{fn}} (lst)
  (mapcar (lambda (x) (* x x)) lst))
`,
	},
	{
		ID: "lisp_reverse", Lang: "lisp",
		Keys: []string{"reverse lisp", "invertir lista lisp"},
		Body: `(defun rev (lst)
  (labels ((walk (xs acc)
             (if (null xs) acc
                 (walk (cdr xs) (cons (car xs) acc)))))
    (walk lst nil)))
`,
	},
	{
		ID: "py_fastapi", Lang: "python",
		Keys: []string{"fastapi", "api python", "endpoint python"},
		Body: `from fastapi import FastAPI

app = FastAPI()

@app.get("/{{route}}")
def {{handler}}():
    return {"ok": True}
`,
	},
	{
		ID: "py_function", Lang: "python",
		Keys: []string{"función python", "funcion python", "def python"},
		Body: `def {{fn}}(items):
    """Transforma items de forma explícita."""
    out = []
    for x in items:
        out.append(x)
    return out
`,
	},
	{
		ID: "py_dataclass", Lang: "python",
		Keys: []string{"dataclass", "data class python"},
		Body: `from dataclasses import dataclass

@dataclass
class {{Name}}:
    id: str
    name: str
`,
	},
	{
		ID: "py_cli_argparse", Lang: "python",
		Keys: []string{"argparse", "cli python", "script cli"},
		Body: `import argparse

def main():
    p = argparse.ArgumentParser(description="{{fn}}")
    p.add_argument("path", help="ruta de entrada")
    args = p.parse_args()
    print(args.path)

if __name__ == "__main__":
    main()
`,
	},
	{
		ID: "js_async", Lang: "javascript",
		Keys: []string{"async js", "fetch js", "promesa ejemplo"},
		Body: `async function {{fn}}(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(String(res.status));
  return res.json();
}
`,
	},
	{
		ID: "js_express", Lang: "javascript",
		Keys: []string{"express", "endpoint node", "api express"},
		Body: `const express = require("express");
const app = express();

app.get("/{{route}}", (req, res) => {
  res.json({ ok: true, path: req.path });
});

app.listen(3000, () => console.log("listen :3000"));
`,
	},
}


// isCodeGenRequest detects explicit request to produce code (not mere explanation).
func isCodeGenRequest(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	triggers := []string{
		"genera código", "genera codigo", "generar código", "generar codigo",
		"escribe código", "escribe codigo", "escribir código", "escribir codigo",
		"dame el código", "dame el codigo", "código de", "codigo de",
		"implementa ", "implementar ", "esqueleto de", "plantilla de",
		"hazme un", "hazme una", "crea una función", "crea una funcion",
		"crea un handler", "crea un endpoint", "código en go", "codigo en go",
		"código en python", "codigo en python", "código en lisp", "codigo en lisp",
		"write a function", "generate code", "boilerplate",
	}
	for _, t := range triggers {
		if strings.Contains(s, t) {
			return true
		}
	}
	// "código go para ..." / "lisp para filtrar"
	if (strings.Contains(s, "código") || strings.Contains(s, "codigo")) &&
		(strings.Contains(s, "go") || strings.Contains(s, "python") || strings.Contains(s, "lisp") || strings.Contains(s, "javascript") || strings.Contains(s, "js")) {
		return true
	}
	return false
}

// codeGenEthicsVeto: true → no entregar artefacto (ethics sumidero).
func codeGenEthicsVeto(s string) bool {
	s = strings.ToLower(s)
	danger := []string{
		"rm -rf", "rm -r /", "format c:", "mkfs", "dd if=",
		"drop table", "drop database", "truncate table",
		"os.removeall", "os.removeall(", "shutil.rmtree",
		"unlink(", "delete from ", "wipe disk", "fork bomb",
		":(){ :|:& };:", "curl | sh", "wget | sh", "chmod 777 /",
		"eval(", "exec(", "os.system(\"rm", "subprocess.*rm -rf",
		"password", "api_key", "secret key", "private key",
		"exfiltrat", "ransomware", "keylogger",
	}
	for _, d := range danger {
		if strings.Contains(s, d) {
			return true
		}
	}
	return false
}

func pickCodeTemplate(s string) *codeTemplate {
	s = strings.ToLower(s)
	bestScore := 0
	var best *codeTemplate
	for i := range codeTemplates {
		t := &codeTemplates[i]
		sc := 0
		for _, k := range t.Keys {
			if strings.Contains(s, k) {
				sc += 3 + len(k)/4
			}
		}
		// lang bias
		if strings.Contains(s, t.Lang) {
			sc += 2
		}
		if t.Lang == "go" && (strings.Contains(s, "golang") || strings.Contains(s, " en go")) {
			sc += 2
		}
		if t.Lang == "javascript" && strings.Contains(s, "js") {
			sc += 1
		}
		if sc > bestScore {
			bestScore = sc
			best = t
		}
	}
	if bestScore == 0 {
		// default by language hint
		if strings.Contains(s, "lisp") {
			return &codeTemplates[3]
		}
		if strings.Contains(s, "python") {
			return &codeTemplates[7]
		}
		if strings.Contains(s, "javascript") || strings.Contains(s, " js") {
			return &codeTemplates[8]
		}
		// safe default: small go handler
		return &codeTemplates[0]
	}
	return best
}

func fillCodeSlots(body, user string) string {
	name := extractCodeIdent(user, "Item")
	fn := extractCodeIdent(user, "process")
	handler := extractCodeIdent(user, "handleOK")
	route := "hello"
	if strings.Contains(strings.ToLower(user), "health") {
		route = "health"
	}
	pkg := "main"
	out := body
	out = strings.ReplaceAll(out, "{{handler}}", handler)
	out = strings.ReplaceAll(out, "{{fn}}", fn)
	out = strings.ReplaceAll(out, "{{route}}", route)
	out = strings.ReplaceAll(out, "{{pkg}}", pkg)
	out = strings.ReplaceAll(out, "{{Name}}", capitalizeIdent(name))
	return out
}

var identRe = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]{1,32})\b`)

func extractCodeIdent(user, def string) string {
	low := strings.ToLower(user)
	stops := []string{
		"genera", "generar", "codigo", "código", "escribe", "escribir", "implementa", "implementar",
		"función", "funcion", "handler", "endpoint", "esqueleto", "plantilla", "boilerplate",
		"para", "en", "go", "golang", "python", "lisp", "javascript", "js", "typescript",
		"dame", "una", "un", "el", "la", "de", "con", "por", "favor", "simple", "básico", "basico",
		"http", "api", "rest", "servidor", "server", "test", "unitario",
	}
	for _, stop := range stops {
		low = strings.ReplaceAll(low, stop, " ")
	}
	cands := identRe.FindAllString(low, -1)
	for _, c := range cands {
		if len(c) >= 2 && c != "ok" && c != "true" {
			return c
		}
	}
	return def
}

// mindGenerateCode is the Fase-5 tool. Returns voice lines and optional code body.
// ethicsState: if 2 or veto patterns → refuse.
func (n *NodoAlset) mindGenerateCode(userText string, ethicsState int) (voice string, code string, lang string, vetoed bool) {
	if !isCodeGenRequest(userText) {
		return "", "", "", false
	}
	if ethicsState == 2 || codeGenEthicsVeto(userText) {
		return "No entrego ese artefacto: el campo ethics lo marca como riesgo (sumidero). Reformula sin acciones destructivas ni secretos.", "", "", true
	}
	tpl := pickCodeTemplate(userText)
	if tpl == nil {
		return "No tengo plantilla curada para ese pedido. Prueba: handler HTTP en Go, defun en Lisp, endpoint FastAPI, o test table-driven en Go.", "", "", false
	}
	code = fillCodeSlots(tpl.Body, userText)
	lang = tpl.Lang
	note := ""
	if n != nil {
		if k := speakFromKnowledge(userText); k != "" {
			note = "\n\nContexto del corpus (no sustituye el esqueleto):\n" + compressVoiceBlock(k, 280)
		}
		if prev := lastCodegenHint(n); prev != "" {
			note += "\n\n" + prev
		}
	}
	voice = fmt.Sprintf(
		"Aquí va un esqueleto en %s (plantilla «%s»; composición ternaria, no predicción de tokens). Revísalo antes de usarlo:\n\n```%s\n%s```%s",
		lang, tpl.ID, lang, strings.TrimSpace(code), note,
	)
	return voice, code, lang, false
}

// lastCodegenHint surfaces a prior mind_codegen episode if any (CID memory continuity).
func lastCodegenHint(n *NodoAlset) string {
	if n == nil {
		return ""
	}
	for _, ep := range n.recallRecentEpisodes(12) {
		if ep.Type != "mind_codegen" && !strings.Contains(strings.ToLower(ep.Voice), "```") {
			continue
		}
		if ep.Type == "mind_codegen" || strings.Contains(ep.Voice, "esqueleto") {
			return "Hay un esqueleto reciente en memoria episódica; si quieres otro lenguaje o plantilla, dilo con claridad."
		}
	}
	return ""
}

// saveCodegenEpisode stores request+code+eval as CID for durable memory.
func (n *NodoAlset) saveCodegenEpisode(userText, code, lang, voice string, ethicsState int, vetoed bool) string {
	ep := map[string]interface{}{
		"type":   "mind_codegen",
		"text":   userText,
		"lang":   lang,
		"code":   code,
		"voice":  voice,
		"ethics": ethicsState,
		"veto":   vetoed,
		"ts":     time.Now().UTC().Format(time.RFC3339),
		"agent":  mindAgentID,
	}
	raw, err := json.Marshal(ep)
	if err != nil || n == nil {
		return ""
	}
	cid, err := n.GenerarCID(raw)
	if err != nil || cid == "" {
		return ""
	}
	n.appendMindEpisodeCID(cid)
	n.Auditoria("MIND_CODEGEN", fmt.Sprintf("cid=%s lang=%s veto=%v", cid, lang, vetoed))
	return cid
}

func capitalizeIdent(s string) string {
	if s == "" {
		return "Item"
	}
	r := []rune(strings.ToLower(s))
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}
