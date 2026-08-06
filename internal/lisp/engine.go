package lisp

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	mathrand "math/rand"
	"os"
	"strconv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"redalset/internal/agents"
	"redalset/internal/neural"
	"redalset/internal/nodeiface"
	"redalset/internal/poh"
)

type LispValue interface{}
type LispSymbol string
type LispList []LispValue

type LispFunction func(args []LispValue, env *LispEnvironment) LispValue
type LispMacro func(args []LispValue, env *LispEnvironment) LispValue

type LispUserFunction struct {
	Params     []LispSymbol
	Body       []LispValue
	Env        *LispEnvironment
	IsVariadic bool
	RestParam  *LispSymbol
	OptParams  map[LispSymbol]LispValue
}

type LispEnvironment struct {
	parent    *LispEnvironment
	values    map[LispSymbol]LispValue
	functions map[LispSymbol]LispValue
}

func NewLispEnvironment(parent *LispEnvironment) *LispEnvironment {
	return &LispEnvironment{
		parent:    parent,
		values:    make(map[LispSymbol]LispValue),
		functions: make(map[LispSymbol]LispValue),
	}
}

func (e *LispEnvironment) Lookup(sym LispSymbol) (LispValue, bool) {
	if val, ok := e.values[sym]; ok {
		return val, true
	}
	if e.parent != nil {
		return e.parent.Lookup(sym)
	}
	return nil, false
}

func (e *LispEnvironment) LookupFunction(sym LispSymbol) (LispValue, bool) {
	if val, ok := e.functions[sym]; ok {
		return val, true
	}
	if e.parent != nil {
		return e.parent.LookupFunction(sym)
	}
	return nil, false
}

func (e *LispEnvironment) Set(sym LispSymbol, val LispValue) {
	e.values[sym] = val
}

func (e *LispEnvironment) SetFunction(sym LispSymbol, val LispValue) {
	e.functions[sym] = val
}

// =============================================================================
// PARSER LISP
// =============================================================================

type LispParser struct {
	tokens []string
	pos    int
}

func NewLispParser(input string) *LispParser {
	return &LispParser{
		tokens: tokenizeLisp(input),
		pos:    0,
	}
}

func tokenizeLisp(s string) []string {
	var tokens []string
	var current strings.Builder
	i := 0
	n := len(s)

	for i < n {
		r := rune(s[i])

		if r == ';' {
			for i < n && s[i] != '\n' {
				i++
			}
			continue
		}

		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			i++
			continue
		}

		if r == '"' {
			current.WriteRune(r)
			i++
			for i < n && (s[i] != '"' || (i > 0 && s[i-1] == '\\')) {
				current.WriteRune(rune(s[i]))
				i++
			}
			if i < n {
				current.WriteRune('"')
				i++
			}
			tokens = append(tokens, current.String())
			current.Reset()
			continue
		}

		if r == '`' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, "`")
			i++
			continue
		}

		if r == ',' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			if i+1 < n && s[i+1] == '@' {
				tokens = append(tokens, ",@")
				i += 2
			} else {
				tokens = append(tokens, ",")
				i++
			}
			continue
		}

		if r == '(' || r == ')' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
			i++
			continue
		}

		current.WriteRune(r)
		i++
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func (p *LispParser) Peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos]
}

func (p *LispParser) Next() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *LispParser) Parse() (LispValue, error) {
	tok := p.Peek()
	if tok == "" {
		return nil, fmt.Errorf("unexpected EOF")
	}

	switch tok {
	case "(":
		p.Next()
		return p.parseList()
	case "'":
		p.Next()
		expr, err := p.Parse()
		if err != nil {
			return nil, err
		}
		return LispList{LispSymbol("quote"), expr}, nil
	case "`":
		p.Next()
		expr, err := p.Parse()
		if err != nil {
			return nil, err
		}
		return LispList{LispSymbol("quasiquote"), expr}, nil
	case ",":
		p.Next()
		expr, err := p.Parse()
		if err != nil {
			return nil, err
		}
		return LispList{LispSymbol("unquote"), expr}, nil
	case ",@":
		p.Next()
		expr, err := p.Parse()
		if err != nil {
			return nil, err
		}
		return LispList{LispSymbol("unquote-splicing"), expr}, nil
	}

	p.Next()
	return p.parseAtom(tok)
}

func (p *LispParser) parseList() (LispValue, error) {
	list := LispList{}
	for p.Peek() != "" && p.Peek() != ")" {
		expr, err := p.Parse()
		if err != nil {
			return nil, err
		}
		list = append(list, expr)
	}
	if p.Peek() != ")" {
		return nil, fmt.Errorf("missing closing parenthesis")
	}
	p.Next()
	return list, nil
}

func (p *LispParser) parseAtom(tok string) (LispValue, error) {
	if val, err := strconv.ParseFloat(tok, 64); err == nil {
		return val, nil
	}
	if val, err := strconv.ParseInt(tok, 10, 64); err == nil {
		return float64(val), nil
	}
	if tok == "t" || tok == "T" {
		return true, nil
	}
	if tok == "nil" || tok == "NIL" || tok == "null" {
		return nil, nil
	}
	if strings.HasPrefix(tok, "\"") && strings.HasSuffix(tok, "\"") {
		return strings.Trim(tok, "\""), nil
	}
	return LispSymbol(tok), nil
}

// =============================================================================
// EVALUADOR LISP
// =============================================================================

type Evaluator struct {
	globalEnv *LispEnvironment
	host      nodeiface.Host
	mu        sync.RWMutex
}

func NewEvaluator(host nodeiface.Host) *Evaluator {
	eval := &Evaluator{
		globalEnv: NewLispEnvironment(nil),
		host:      host,
	}
	eval.initBuiltins()
	return eval
}

func (e *Evaluator) expandQuasiquote(expr LispValue, env *LispEnvironment) LispValue {
	switch v := expr.(type) {
	case LispList:
		if len(v) > 0 {
			if first, ok := v[0].(LispSymbol); ok {
				switch first {
				case "unquote":
					if len(v) == 2 {
						return e.eval(v[1], env)
					}
				case "quasiquote":
					if len(v) == 2 {
						return e.expandQuasiquote(v[1], env)
					}
				}
			}
		}
		result := make(LispList, len(v))
		for i, item := range v {
			result[i] = e.expandQuasiquote(item, env)
		}
		return result
	default:
		return v
	}
}

func (e *Evaluator) macroexpand1(form LispValue, env *LispEnvironment) LispValue {
	if list, ok := form.(LispList); ok && len(list) > 0 {
		if sym, ok := list[0].(LispSymbol); ok {
			if macro, ok := env.LookupFunction(sym); ok {
				if macroFunc, ok := macro.(LispMacro); ok {
					return macroFunc(list[1:], env)
				}
			}
		}
	}
	return form
}

func (e *Evaluator) macroexpand(form LispValue, env *LispEnvironment) LispValue {
	expanded := e.macroexpand1(form, env)
	if expanded == form {
		return expanded
	}
	return e.macroexpand(expanded, env)
}

func (e *Evaluator) expandMacros(expr LispValue, env *LispEnvironment) LispValue {
	list, ok := expr.(LispList)
	if !ok || len(list) == 0 {
		return expr
	}

	first, ok := list[0].(LispSymbol)
	if !ok {
		result := make(LispList, len(list))
		for i, item := range list {
			result[i] = e.expandMacros(item, env)
		}
		return result
	}

	specialForms := map[string]bool{
		"quote": true, "if": true, "progn": true, "let": true, "let*": true,
		"lambda": true, "defun": true, "defmacro": true, "setq": true,
		"cond": true, "and": true, "or": true,
	}

	if specialForms[string(first)] {
		result := make(LispList, len(list))
		result[0] = first
		for i := 1; i < len(list); i++ {
			result[i] = e.expandMacros(list[i], env)
		}
		return result
	}

	if macroValue, exists := env.functions[first]; exists {
		if macroFunc, isMacro := macroValue.(LispMacro); isMacro {
			expanded := macroFunc(list[1:], env)
			if expandedList, ok := expanded.(LispList); ok && len(expandedList) > 0 {
				if q, ok := expandedList[0].(LispSymbol); ok && q == "quasiquote" && len(expandedList) == 2 {
					return e.expandQuasiquote(expandedList[1], env)
				}
			}
			return e.expandMacros(expanded, env)
		}
	}

	result := make(LispList, len(list))
	result[0] = first
	for i := 1; i < len(list); i++ {
		result[i] = e.expandMacros(list[i], env)
	}
	return result
}

func (e *Evaluator) Eval(code string) (LispValue, error) {
	parser := NewLispParser(code)
	expr, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	expanded := e.expandMacros(expr, e.globalEnv)
	return e.eval(expanded, e.globalEnv), nil
}

func (e *Evaluator) ExpandDebug(code string) (LispValue, error) {
	parser := NewLispParser(code)
	expr, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	fmt.Printf("Original: %#v\n", expr)
	expanded := e.expandMacros(expr, e.globalEnv)
	fmt.Printf("Expandido: %#v\n", expanded)
	return expanded, nil
}

func convertStringsToSymbols(expr LispValue) LispValue {
	switch v := expr.(type) {
	case LispList:
		result := make(LispList, len(v))
		for i, item := range v {
			result[i] = convertStringsToSymbols(item)
		}
		return result
	case string:
		return LispSymbol(v)
	default:
		return v
	}
}

// =============================================================================
// FUNCIONES PRIMITIVAS DEL LISP
// =============================================================================

func (e *Evaluator) initBuiltins() {
	// =====================================================================
	// OPERADORES ARITMÉTICOS, LÓGICOS, LISTAS, MATEMÁTICOS, E/S, ETC.
	// =====================================================================

	e.globalEnv.SetFunction("+", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		sum := 0.0
		for _, arg := range args {
			sum += toFloat(e.eval(arg, env))
		}
		return sum
	}))

	e.globalEnv.SetFunction("-", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) == 0 {
			return 0.0
		}
		first := toFloat(e.eval(args[0], env))
		if len(args) == 1 {
			return -first
		}
		result := first
		for i := 1; i < len(args); i++ {
			result -= toFloat(e.eval(args[i], env))
		}
		return result
	}))

	e.globalEnv.SetFunction("*", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		result := 1.0
		for _, arg := range args {
			result *= toFloat(e.eval(arg, env))
		}
		return result
	}))

	e.globalEnv.SetFunction("/", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) == 0 {
			return 1.0
		}
		first := toFloat(e.eval(args[0], env))
		if len(args) == 1 {
			return 1.0 / first
		}
		result := first
		for i := 1; i < len(args); i++ {
			div := toFloat(e.eval(args[i], env))
			if div != 0 {
				result /= div
			}
		}
		return result
	}))

	e.globalEnv.SetFunction("<", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return true
		}
		prev := toFloat(e.eval(args[0], env))
		for i := 1; i < len(args); i++ {
			curr := toFloat(e.eval(args[i], env))
			if prev >= curr {
				return false
			}
			prev = curr
		}
		return true
	}))

	e.globalEnv.SetFunction("<=", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return true
		}
		prev := toFloat(e.eval(args[0], env))
		for i := 1; i < len(args); i++ {
			curr := toFloat(e.eval(args[i], env))
			if prev > curr {
				return false
			}
			prev = curr
		}
		return true
	}))

	e.globalEnv.SetFunction(">", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return true
		}
		prev := toFloat(e.eval(args[0], env))
		for i := 1; i < len(args); i++ {
			curr := toFloat(e.eval(args[i], env))
			if prev <= curr {
				return false
			}
			prev = curr
		}
		return true
	}))

	e.globalEnv.SetFunction(">=", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return true
		}
		prev := toFloat(e.eval(args[0], env))
		for i := 1; i < len(args); i++ {
			curr := toFloat(e.eval(args[i], env))
			if prev < curr {
				return false
			}
			prev = curr
		}
		return true
	}))

	e.globalEnv.SetFunction("=", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return true
		}
		first := e.eval(args[0], env)
		for i := 1; i < len(args); i++ {
			if !equalValue(first, e.eval(args[i], env)) {
				return false
			}
		}
		return true
	}))

	e.globalEnv.SetFunction("and", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		for _, arg := range args {
			val := e.eval(arg, env)
			if !isTruthy(val) {
				return false
			}
		}
		return true
	}))

	e.globalEnv.SetFunction("or", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		for _, arg := range args {
			val := e.eval(arg, env)
			if isTruthy(val) {
				return true
			}
		}
		return false
	}))

	e.globalEnv.SetFunction("not", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) == 0 {
			return true
		}
		return !isTruthy(e.eval(args[0], env))
	}))

	e.globalEnv.SetFunction("list", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		result := make(LispList, len(args))
		for i, arg := range args {
			result[i] = e.eval(arg, env)
		}
		return result
	}))

	e.globalEnv.SetFunction("car", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) == 0 {
			return nil
		}
		list := e.eval(args[0], env)
		if l, ok := list.(LispList); ok && len(l) > 0 {
			return l[0]
		}
		return nil
	}))

	e.globalEnv.SetFunction("cdr", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) == 0 {
			return LispList{}
		}
		list := e.eval(args[0], env)
		if l, ok := list.(LispList); ok && len(l) > 1 {
			return l[1:]
		}
		return LispList{}
	}))

	e.globalEnv.SetFunction("cons", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return args
		}
		first := e.eval(args[0], env)
		rest := e.eval(args[1], env)
		if l, ok := rest.(LispList); ok {
			return append(LispList{first}, l...)
		}
		return LispList{first, rest}
	}))

	e.globalEnv.SetFunction("append", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		result := LispList{}
		for _, arg := range args {
			val := e.eval(arg, env)
			if l, ok := val.(LispList); ok {
				result = append(result, l...)
			} else {
				result = append(result, val)
			}
		}
		return result
	}))

	e.globalEnv.SetFunction("concat", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		var result strings.Builder
		for _, arg := range args {
			val := e.eval(arg, env)
			result.WriteString(fmt.Sprintf("%v", val))
		}
		return result.String()
	}))

	e.globalEnv.SetFunction("format", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return ""
		}
		formatStr := fmt.Sprintf("%v", e.eval(args[0], env))
		var printfArgs []interface{}
		for i := 1; i < len(args); i++ {
			printfArgs = append(printfArgs, e.eval(args[i], env))
		}
		return fmt.Sprintf(formatStr, printfArgs...)
	}))

	e.globalEnv.SetFunction("length", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) == 0 {
			return 0.0
		}
		val := e.eval(args[0], env)
		if l, ok := val.(LispList); ok {
			return float64(len(l))
		}
		return 1.0
	}))

	// Funciones matemáticas
	mathFuncs := map[string]func(float64) float64{
		"sin":   math.Sin,
		"cos":   math.Cos,
		"tan":   math.Tan,
		"asin":  math.Asin,
		"acos":  math.Acos,
		"atan":  math.Atan,
		"sinh":  math.Sinh,
		"cosh":  math.Cosh,
		"tanh":  math.Tanh,
		"exp":   math.Exp,
		"sqrt":  math.Sqrt,
		"abs":   math.Abs,
		"floor": math.Floor,
		"round": math.Round,
	}
	for name, fn := range mathFuncs {
		f := fn
		e.globalEnv.SetFunction(LispSymbol(name), LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
			if len(args) == 0 {
				return 0.0
			}
			return f(toFloat(e.eval(args[0], env)))
		}))
	}

	// expt
	e.globalEnv.SetFunction("expt", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return 1.0
		}
		return math.Pow(toFloat(e.eval(args[0], env)), toFloat(e.eval(args[1], env)))
	}))

	e.globalEnv.SetFunction("second", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return nil
		}
		if l, ok := args[0].(LispList); ok && len(l) >= 2 {
			return l[1]
		}
		return nil
	}))

	e.globalEnv.SetFunction("third", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return nil
		}
		if l, ok := args[0].(LispList); ok && len(l) >= 3 {
			return l[2]
		}
		return nil
	}))

	e.globalEnv.SetFunction("setq", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: setq requires at least 2 arguments"
		}
		var result LispValue = nil
		for i := 0; i < len(args); i += 2 {
			if i+1 >= len(args) {
				break
			}
			if sym, ok := args[i].(LispSymbol); ok {
				val := e.eval(args[i+1], env)
				env.Set(sym, val)
				result = val
			}
		}
		return result
	}))

	// =====================================================================
	// FUNCIONES DE VERIFICACIÓN DE CREDENCIALES (VC, PoH, ZKP)
	// =====================================================================

	e.globalEnv.SetFunction("sellar-documento", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: requiere cid"
		}
		cidOriginal := fmt.Sprintf("%v", e.eval(args[0], env))
		fecha := time.Now().Format(time.RFC3339)

		vc := map[string]interface{}{
			"@context": []string{
				"https://www.w3.org/2018/credentials/v1",
				"https://w3id.org/security/suites/ed25519-2020/v1",
			},
			"id":           fmt.Sprintf("urn:uuid:%s", cidOriginal),
			"type":         []string{"VerifiableCredential", "PrismCertificate"},
			"issuer":       "did:prism:tec:institutional",
			"issuanceDate": fecha,
			"credentialSubject": map[string]interface{}{
				"id": func() string {
					if len(cidOriginal) > 16 {
						return cidOriginal[:16]
					}
					return cidOriginal
				}(),
				"documentCID": cidOriginal,
				"garante":     "Prism@.TEC - Garante de la Verdad Digital",
				"titular":     "Dayanis Pérez Soria",
				"sealType":    "PrismSeal",
				"timestamp":   fecha,
			},
		}

		canonicalBytes, err := canonicalizeJSON(vc)
		if err != nil {
			return fmt.Sprintf("error_canonicalize: %v", err)
		}

		firmaBytes, err := e.host.Sign(canonicalBytes)
		if err != nil {
			return fmt.Sprintf("error_firma: %v", err)
		}

		proof := map[string]interface{}{
			"type":               "Ed25519Signature2020",
			"created":            fecha,
			"verificationMethod": "did:prism:tec:institutional#key-1",
			"proofPurpose":       "assertionMethod",
			"proofValue":         hex.EncodeToString(firmaBytes),
		}

		vc["proof"] = proof

		finalVCBytes, err := canonicalizeJSON(vc)
		if err != nil {
			return fmt.Sprintf("error_final: %v", err)
		}

		certCID, err := e.host.GenerarCID(finalVCBytes)
		if err != nil {
			return fmt.Sprintf("error_cid: %v", err)
		}

		e.host.Auditoria("VC_EMITIDO", fmt.Sprintf("Doc: %s | VC: %s", cidOriginal, certCID))
		e.host.AnunciarNuevoBloque(certCID)
		return certCID
	}))

	e.globalEnv.SetFunction("revocar-certificado", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: requiere cid y motivo"
		}
		if false {
			return "error: el nodo no está unido al tópico de la red"
		}
		if !e.host.HasMasterKey() {
			return "error: clave maestra no cargada"
		}

		certCID := fmt.Sprintf("%v", e.eval(args[0], env))
		motivo := fmt.Sprintf("%v", e.eval(args[1], env))
		fecha := time.Now().Format(time.RFC3339)

		mensaje := fmt.Sprintf("REVOKE|%s|%s", certCID, fecha)
		firma, err := e.host.Sign([]byte(mensaje))
		if err != nil {
			return "error_firma: " + err.Error()
		}

		revocationTicket := map[string]interface{}{
			"@context":     "https://www.w3.org/2018/credentials/v1",
			"id":           fmt.Sprintf("urn:uuid:%s", certCID),
			"type":         []string{"RevocationList2020Credential"},
			"issuer":       "did:prism:tec:institutional",
			"issuanceDate": fecha,
			"credentialSubject": map[string]interface{}{
				"id":                fmt.Sprintf("did:prism:%s", certCID[:16]),
				"revokedCredential": certCID,
				"revocationReason":  motivo,
				"revocationDate":    fecha,
			},
			"proof": map[string]interface{}{
				"type":               "Ed25519Signature2020",
				"created":            fecha,
				"verificationMethod": "did:prism:tec:institutional#key-1",
				"proofPurpose":       "assertionMethod",
				"jws":                hex.EncodeToString(firma),
			},
		}

		data, _ := json.Marshal(revocationTicket)
		revCID, _ := e.host.GenerarCID(data)
		e.host.Auditoria("VC_REVOCADO", fmt.Sprintf("VC: %s | Motivo: %s", certCID, motivo))

		update := map[string]string{"tipo": "revocacion_update", "cid": revCID}
		msg, _ := json.Marshal(update)
		if err := e.host.PublishTopic(msg); err != nil {
			return "error_difusion: " + err.Error()
		}
		return revCID
	}))

	e.globalEnv.SetFunction("zkp-prove", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: requiere declaración y prueba"
		}
		statement := fmt.Sprintf("%v", e.eval(args[0], env))
		witness := fmt.Sprintf("%v", e.eval(args[1], env))

		proofBytes := []byte(statement + witness)
		hash := make([]byte, 32)
		copy(hash, proofBytes[:32])

		return map[string]interface{}{
			"proof":     hex.EncodeToString(hash),
			"statement": statement,
			"type":      "simple-zkp-sha256",
		}
	}))

	e.globalEnv.SetFunction("humanity-proof", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		poh.Global.Lock()
		defer poh.Global.Unlock()

		if len(poh.Global.Events()) == 0 {
			return "No hay suficientes eventos de humanidad registrados"
		}

		var eventsData []byte
		for _, ev := range poh.Global.Events() {
			evData, _ := json.Marshal(ev)
			eventsData = append(eventsData, evData...)
		}

		hash := make([]byte, 32)
		copy(hash, eventsData[:32])

		proof := poh.Proof{
			SessionID: poh.Global.SessionID(),
			Events:    poh.Global.Events(),
			FinalSig:  hex.EncodeToString(hash),
		}

		proofBytes, _ := json.Marshal(proof)
		proofCID, _ := e.host.GenerarCID(proofBytes)

		poh.Global.ClearEvents()

		return map[string]interface{}{
			"status":       "prueba_humanidad_generada",
			"proof_cid":    proofCID,
			"events_count": len(proof.Events),
			"session_id":   proof.SessionID,
		}
	}))

	e.globalEnv.SetFunction("emitir-sello-humanidad", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		poh.Global.Lock()
		defer poh.Global.Unlock()

		if poh.Global.SessionID() == "" {
			poh.Global.SetSessionID(hex.EncodeToString([]byte(time.Now().String()))[:16])
		}

		eventType := "custom"
		metadata := ""
		if len(args) > 0 {
			eventType = fmt.Sprintf("%v", e.eval(args[0], env))
		}
		if len(args) > 1 {
			metadata = fmt.Sprintf("%v", e.eval(args[1], env))
		}

		event := poh.Event{
			Timestamp: time.Now().Unix(),
			EventType: eventType,
			Metadata:  metadata,
		}

		poh.Global.Append(event)

		return fmt.Sprintf("Evento de humanidad registrado (%d total)", len(poh.Global.Events()))
	}))

	// =====================================================================
	// FUNCIONES DE AGENTES, DNS, IPFS, LISP
	// =====================================================================

	e.globalEnv.SetFunction("crear-agente", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: id requerido"
		}
		agentID := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")

		e.host.Lock()
		if _, existe := e.host.GetAgent(agentID); !existe {
			e.host.PutAgent(&agents.Agente{
				ID:           agentID,
				RootCID:      "",
				UltimaActual: time.Now().Unix(),
				BalanceUTXO:  1000.0,
			})
			e.host.Unlock()
			e.host.Auditoria("AGENTE_CREADO", "ID: "+agentID)
			e.host.PersistirLocamente()
			e.host.SincronizarConPares()
			// ---- PULSO: emitir evento ----
			go e.host.BroadcastPulse("agent_created", map[string]interface{}{
				"id":   agentID,
				"root": "",
				"time": time.Now().Unix(),
			})
			return "Agente " + agentID + " creado"
		}
		e.host.Unlock()
		return "error: ya existe"
	}))

	e.globalEnv.SetFunction("set-agent-root", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error"
		}
		agentID := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		cidStr := strings.Trim(fmt.Sprintf("%v", e.eval(args[1], env)), "\"")
		e.host.Lock()
		if a, ok := e.host.GetAgent(agentID); ok {
			a.RootCID = cidStr
			a.UltimaActual = time.Now().Unix()
			e.host.PutAgent(a)
		}
		e.host.Unlock()
		e.host.PersistirLocamente()
		e.host.SincronizarConPares()
		// ---- PULSO: emitir evento ----
		go e.host.BroadcastPulse("root_updated", map[string]interface{}{
			"id":   agentID,
			"root": cidStr,
			"time": time.Now().Unix(),
		})
		return "Root actualizado"
	}))

	e.globalEnv.SetFunction("register-name", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error"
		}
		alias := fmt.Sprintf("%v", e.eval(args[0], env))
		agentID := fmt.Sprintf("%v", e.eval(args[1], env))
		e.host.Lock()
		e.host.SetNombre(alias, agentID)
		e.host.Unlock()
		e.host.Auditoria("DNS_REGISTRO", fmt.Sprintf("Alias: %s -> Agente: %s", alias, agentID))
		e.host.PersistirLocamente()
		e.host.DifundirActualizacionDNS(alias, agentID)
		// ---- PULSO: emitir evento ----
		go e.host.BroadcastPulse("dns_registered", map[string]interface{}{
			"alias": alias,
			"agent": agentID,
			"time":  time.Now().Unix(),
		})
		return "OK"
	}))

	e.globalEnv.SetFunction("ipfs-add", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error"
		}
		data := []byte(fmt.Sprintf("%v", e.eval(args[0], env)))
		cidStr, _ := e.host.GenerarCID(data)
		e.host.AnunciarNuevoBloque(cidStr)
		return cidStr
	}))

	e.globalEnv.SetFunction("fetch-cid", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error"
		}
		data, _ := e.host.BuscarContenidoPorCID(fmt.Sprintf("%v", e.eval(args[0], env)))
		return string(data)
	}))

	e.globalEnv.SetFunction("to-json", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "ERROR: falta argumento"
		}
		val := e.eval(args[0], env)
		jsonBytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return string(jsonBytes)
	}))

	e.globalEnv.SetFunction("from-json", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "ERROR: falta argumento"
		}
		jsonStr := fmt.Sprintf("%v", e.eval(args[0], env))
		var result interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return result
	}))

	e.globalEnv.SetFunction("log", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		msg := fmt.Sprintf("🤖 [LISPAI]: %v", e.eval(args[0], env))
		fmt.Fprintln(os.Stdout, msg)
		return msg
	}))

	e.globalEnv.SetFunction("println", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		for _, arg := range args {
			fmt.Printf("%v ", e.eval(arg, env))
		}
		fmt.Println()
		return nil
	}))

	// =====================================================================
	// CONEXIÓN PEER (connect-to-peer)
	// =====================================================================

	e.globalEnv.SetFunction("connect-to-peer", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: se requiere la multiaddr del peer"
		}
		addrStr, ok := args[0].(string)
		if !ok {
			return "error: la multiaddr debe ser un string"
		}
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			return fmt.Sprintf("error al parsear multiaddr: %v", err)
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return fmt.Sprintf("error al obtener info del peer: %v", err)
		}
		ctx, cancel := context.WithTimeout(e.host.Ctx(), 10*time.Second)
		defer cancel()
		if err := e.host.ConnectPeer(ctx, *peerInfo); err != nil {
			return fmt.Sprintf("error al conectar: %v", err)
		}
		return fmt.Sprintf("conectado a %s", peerInfo.ID.String())
	}))

	// =====================================================================
	// SISTEMA NEURONAL
	// =====================================================================

	e.globalEnv.SetFunction("neuron-state", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if e.host.GetNeural() == nil {
			return "No inicializado"
		}
		return fmt.Sprintf("Membrane:%.4f Threshold:%.4f LeakRate:%.4f Type:%s Synapses:%d LastSpike:%d",
			e.host.GetNeural().MembranePotential,
			e.host.GetNeural().SpikeThreshold,
			e.host.GetNeural().LeakRate,
			e.host.GetNeural().NeuronType,
			len(e.host.GetNeural().Synapses),
			e.host.GetNeural().LastSpikeTime)
	}))

	e.globalEnv.SetFunction("neuron-stats", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if e.host.GetNeural() == nil {
			return "No inicializado"
		}
		avgWeight := 0.0
		totalFires := int64(0)
		for _, s := range e.host.GetNeural().Synapses {
			avgWeight += s.Weight
			totalFires += s.SuccessfulFires
		}
		if len(e.host.GetNeural().Synapses) > 0 {
			avgWeight /= float64(len(e.host.GetNeural().Synapses))
		}
		return fmt.Sprintf("Type:%s Membrane:%.4f Threshold:%.4f Leak:%.4f Synapses:%d AvgWeight:%.4f TotalSpikes:%d",
			e.host.GetNeural().NeuronType,
			e.host.GetNeural().MembranePotential,
			e.host.GetNeural().SpikeThreshold,
			e.host.GetNeural().LeakRate,
			len(e.host.GetNeural().Synapses),
			avgWeight,
			totalFires)
	}))

	e.globalEnv.SetFunction("sigmoid", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return 0.0
		}
		x := toFloat(args[0])
		return 1.0 / (1.0 + math.Exp(-x))
	}))

	e.globalEnv.SetFunction("local-inference", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return 0.0
		}
		input := toFloat(args[0])
		if e.host.GetNeural() == nil {
			return input
		}
		potencial := input
		for _, syn := range e.host.GetNeural().Synapses {
			potencial += syn.Weight * 0.1
		}
		return 1.0 / (1.0 + math.Exp(-potencial))
	}))

	e.globalEnv.SetFunction("connect-to-neuron", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: requiere target"
		}
		target := fmt.Sprintf("%v", args[0])
		initialWeight := 0.5
		if len(args) >= 2 {
			initialWeight = toFloat(args[1])
		}
		if initialWeight < 0 {
			initialWeight = 0
		}
		if initialWeight > 1 {
			initialWeight = 1
		}

		e.host.Lock()
		if e.host.GetNeural() == nil {
			ns := e.host.EnsureNeural(); *ns = neural.NeuralState{
				Synapses: make(map[string]neural.SynapticWeight),
			}
		}
		e.host.GetNeural().Synapses[target] = neural.SynapticWeight{
			TargetNeuronID: target,
			Weight:         initialWeight,
			LastUpdated:    time.Now().Unix(),
		}
		e.host.Unlock()

		update := map[string]string{
			"tipo":          "synaptic_update",
			"neuronas_pre":  e.host.PeerID(),
			"neuronas_post": target,
			"exito":         "true",
			"peso":          fmt.Sprintf("%f", initialWeight),
		}
		if data, err := json.Marshal(update); err == nil && true {
			go e.host.PublishTopic(data)
		}

		return fmt.Sprintf("Conectado a %s con peso %.2f", target, initialWeight)
	}))

	e.globalEnv.SetFunction("list-synapses", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if e.host.GetNeural() == nil || len(e.host.GetNeural().Synapses) == 0 {
			return "No hay sinapsis"
		}
		result := "Sinapsis:\n"
		for target, syn := range e.host.GetNeural().Synapses {
			result += fmt.Sprintf("  -> %s : peso=%.4f (spikes=%d)\n", target, syn.Weight, syn.SuccessfulFires)
		}
		return result
	}))

	e.globalEnv.SetFunction("memorize", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: requiere datos"
		}
		var dataStr string
		switch v := args[0].(type) {
		case string:
			dataStr = v
		case LispSymbol:
			dataStr = string(v)
		default:
			dataStr = fmt.Sprintf("%v", v)
		}
		dataStr = strings.Trim(dataStr, "\"")
		if dataStr == "" {
			return "error: datos vacíos"
		}
		cidStr, err := e.host.GenerarCID([]byte(dataStr))
		if err != nil {
			return fmt.Sprintf("error al generar CID: %v", err)
		}
		e.host.Auditoria("MEMORIA_GUARDADA", fmt.Sprintf("CID: %s | Data: %s", cidStr, dataStr))
		go func() {
			if true {
				msg := map[string]string{
					"tipo":    "memory_distributed",
					"cid":     cidStr,
					"content": dataStr,
					"origin":  e.host.PeerID(),
					"ttl":     "3",
				}
				if data, err := json.Marshal(msg); err == nil {
					e.host.PublishTopic(data)
				}
			}
		}()
		return cidStr
	}))

	e.globalEnv.SetFunction("recall", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: requiere consulta"
		}
		var query string
		switch v := args[0].(type) {
		case string:
			query = v
		case LispSymbol:
			query = string(v)
		default:
			query = fmt.Sprintf("%v", v)
		}
		query = strings.Trim(query, "\"")
		if query == "" {
			return "error: consulta vacía"
		}
		e.host.RLock()
		defer e.host.RUnlock()
		for cid, data := range e.host.ListBlocks() {
			dataStr := string(data)
			if strings.Contains(strings.ToLower(dataStr), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(cid), strings.ToLower(query)) {
				if len(dataStr) > 1000 {
					dataStr = dataStr[:1000] + "..."
				}
				return dataStr
			}
		}
		return "No encontrado"
	}))

	e.globalEnv.SetFunction("hebbian-update", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: requiere (target tasa)"
		}
		target := fmt.Sprintf("%v", args[0])
		tasa := toFloat(args[1])
		if tasa <= 0 {
			tasa = 0.01
		}
		if tasa > 1 {
			tasa = 0.1
		}
		e.host.Lock()
		defer e.host.Unlock()
		if e.host.GetNeural() == nil {
			return "error: neurona no inicializada"
		}
		if syn, ok := e.host.GetNeural().Synapses[target]; ok {
			oldWeight := syn.Weight
			newWeight := oldWeight + tasa*(1-oldWeight)
			if newWeight > 1 {
				newWeight = 1
			}
			syn.Weight = newWeight
			syn.SuccessfulFires++
			syn.LastUpdated = time.Now().Unix()
			e.host.GetNeural().Synapses[target] = syn
			go func() {
				update := map[string]string{
					"tipo":             "synaptic_update",
					"neuronas_pre":     e.host.PeerID(),
					"neuronas_post":    target,
					"exito":            "true",
					"tasa_aprendizaje": fmt.Sprintf("%f", tasa),
				}
				if data, err := json.Marshal(update); err == nil && true {
					e.host.PublishTopic(data)
				}
			}()
			return fmt.Sprintf("Hebbian update: %.4f -> %.4f", oldWeight, newWeight)
		}
		return fmt.Sprintf("Sinapsis no encontrada: %s", target)
	}))

	e.globalEnv.SetFunction("set-synaptic-weight", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: requiere (target peso)"
		}
		target := fmt.Sprintf("%v", args[0])
		weight := toFloat(args[1])
		if weight < 0 {
			weight = 0
		}
		if weight > 1 {
			weight = 1
		}
		e.host.Lock()
		defer e.host.Unlock()
		if e.host.GetNeural() == nil {
			return "error: neurona no inicializada"
		}
		if syn, ok := e.host.GetNeural().Synapses[target]; ok {
			oldWeight := syn.Weight
			syn.Weight = weight
			syn.LastUpdated = time.Now().Unix()
			e.host.GetNeural().Synapses[target] = syn
			go e.host.PersistirEstadoNeuronal()
			return fmt.Sprintf("Peso actualizado: %.4f -> %.4f", oldWeight, weight)
		}
		return fmt.Sprintf("Sinapsis no encontrada: %s", target)
	}))

	e.globalEnv.SetFunction("train", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: requiere (input esperado)"
		}
		input := toFloat(args[0])
		esperado := toFloat(args[1])
		output := 1.0 / (1.0 + math.Exp(-input))
		error := esperado - output
		tasa := 0.1 * error
		return fmt.Sprintf("Entrenamiento: input=%.4f, output=%.4f, esperado=%.4f, error=%.4f, tasa=%.4f",
			input, output, esperado, error, tasa)
	}))

	// =====================================================================
	// FUNCIONES ESPECIALES DE LISP
	// =====================================================================

	e.globalEnv.SetFunction("quote", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) == 0 {
			return nil
		}
		return args[0]
	}))

	e.globalEnv.SetFunction("progn", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		var result LispValue = nil
		for _, arg := range args {
			result = e.eval(arg, env)
		}
		return result
	}))

	e.globalEnv.SetFunction("defun", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 3 {
			return "error: defun requiere nombre, parámetros y cuerpo"
		}
		funcName, ok := args[0].(LispSymbol)
		if !ok {
			return "error: defun requiere un símbolo como nombre"
		}
		paramsValue := args[1]
		body := args[2:]
		var params []LispSymbol
		if paramList, ok := paramsValue.(LispList); ok {
			for _, p := range paramList {
				if sym, ok := p.(LispSymbol); ok {
					params = append(params, sym)
				}
			}
		}
		userFunc := LispUserFunction{
			Params: params,
			Body:   body,
			Env:    env,
		}
		e.globalEnv.SetFunction(funcName, userFunc)
		return funcName
	}))

	e.globalEnv.SetFunction("defvar", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: defvar requiere (nombre valor)"
		}
		varName, ok := args[0].(LispSymbol)
		if !ok {
			return "error: defvar requiere un símbolo como nombre"
		}
		valor := e.eval(args[1], env)
		if _, exists := env.Lookup(varName); !exists {
			env.Set(varName, valor)
		}
		return varName
	}))

	e.globalEnv.SetFunction("defmacro", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 3 {
			return "error: defmacro requiere (nombre parámetros cuerpo)"
		}
		var macroName LispSymbol
		switch v := args[0].(type) {
		case LispSymbol:
			macroName = v
		case string:
			macroName = LispSymbol(v)
		default:
			return fmt.Sprintf("error: primer argumento debe ser símbolo, got %T", args[0])
		}
		var paramNames []LispSymbol
		switch v := args[1].(type) {
		case LispList:
			for _, p := range v {
				if sym, ok := p.(LispSymbol); ok {
					paramNames = append(paramNames, sym)
				} else if str, ok := p.(string); ok {
					paramNames = append(paramNames, LispSymbol(str))
				} else {
					paramNames = append(paramNames, LispSymbol(fmt.Sprintf("%v", p)))
				}
			}
		case []LispValue:
			for _, p := range v {
				if sym, ok := p.(LispSymbol); ok {
					paramNames = append(paramNames, sym)
				}
			}
		default:
			fmt.Printf("⚠️ DEBUG defmacro: tipo de parámetros = %T, valor = %v\n", args[1], args[1])
			return fmt.Sprintf("error: segundo argumento debe ser lista de parámetros, got %T", args[1])
		}
		body := args[2:]
		macro := LispMacro(func(macroArgs []LispValue, macroEnv *LispEnvironment) LispValue {
			newEnv := NewLispEnvironment(macroEnv)
			for i, param := range paramNames {
				if i < len(macroArgs) {
					newEnv.Set(param, macroArgs[i])
				} else {
					newEnv.Set(param, nil)
				}
			}
			if len(body) == 1 {
				return body[0]
			}
			progn := make(LispList, 0, len(body)+1)
			progn = append(progn, LispSymbol("progn"))
			progn = append(progn, body...)
			return progn
		})
		e.globalEnv.functions[macroName] = macro
		fmt.Printf("✅ Macro definida correctamente: %s (parámetros: %d)\n", macroName, len(paramNames))
		return macroName
	}))

	e.globalEnv.SetFunction("macroexpand-1", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: macroexpand-1 requiere expresión"
		}
		expr := args[0]
		list, ok := expr.(LispList)
		if !ok || len(list) == 0 {
			return expr
		}
		first, ok := list[0].(LispSymbol)
		if !ok {
			return expr
		}
		if macroValue, exists := e.globalEnv.functions[first]; exists {
			if macroFunc, isMacro := macroValue.(LispMacro); isMacro {
				expanded := macroFunc(list[1:], e.globalEnv)
				if expandedList, ok := expanded.(LispList); ok && len(expandedList) > 0 {
					if q, ok := expandedList[0].(LispSymbol); ok && q == "quasiquote" {
						return e.expandQuasiquote(expandedList[1], e.globalEnv)
					}
				}
				return expanded
			}
		}
		return expr
	}))

	e.globalEnv.SetFunction("list-macros", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		result := make(LispList, 0)
		for k, v := range env.functions {
			if _, ok := v.(LispMacro); ok {
				result = append(result, k)
			}
		}
		return result
	}))

	e.globalEnv.SetFunction("macro-type", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: requiere nombre"
		}
		name, ok := args[0].(LispSymbol)
		if !ok {
			return "error: requiere símbolo"
		}
		if val, exists := e.globalEnv.functions[name]; exists {
			return fmt.Sprintf("existe, tipo: %T", val)
		}
		return "no existe"
	}))

	var gensymCounter = 0
	e.globalEnv.SetFunction("gensym", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		gensymCounter++
		return LispSymbol(fmt.Sprintf("G%d", gensymCounter))
	}))

	e.globalEnv.SetFunction("let", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		bindings, ok := args[0].(LispList)
		if !ok {
			return nil
		}
		newEnv := NewLispEnvironment(env)
		for _, binding := range bindings {
			if pair, ok := binding.(LispList); ok && len(pair) >= 2 {
				varName, ok := pair[0].(LispSymbol)
				if ok {
					val := e.eval(pair[1], env)
					newEnv.Set(varName, val)
				}
			}
		}
		var result LispValue = nil
		for i := 1; i < len(args); i++ {
			result = e.eval(args[i], newEnv)
		}
		return result
	}))

	e.globalEnv.SetFunction("let*", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return nil
		}
		bindings, ok := args[0].(LispList)
		if !ok {
			return nil
		}
		newEnv := NewLispEnvironment(env)
		for _, binding := range bindings {
			if pair, ok := binding.(LispList); ok && len(pair) >= 2 {
				if varName, ok := pair[0].(LispSymbol); ok {
					val := e.eval(pair[1], newEnv)
					newEnv.Set(varName, val)
				}
			}
		}
		var result LispValue = nil
		for i := 1; i < len(args); i++ {
			result = e.eval(args[i], newEnv)
		}
		return result
	}))

	e.globalEnv.SetFunction("mapcar", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		fn := args[0]
		lists := make([]LispList, len(args)-1)
		maxLen := 0
		for i := 1; i < len(args); i++ {
			lst, ok := args[i].(LispList)
			if !ok {
				return fmt.Sprintf("error: argumento %d no es una lista", i)
			}
			lists[i-1] = lst
			if len(lst) > maxLen {
				maxLen = len(lst)
			}
		}
		result := make(LispList, 0, maxLen)
		for i := 0; i < maxLen; i++ {
			callArgs := make([]LispValue, len(lists))
			for j, lst := range lists {
				if i < len(lst) {
					callArgs[j] = lst[i]
				} else {
					callArgs[j] = nil
				}
			}
			val := e.apply(fn, callArgs, env)
			result = append(result, val)
		}
		return result
	}))

	e.globalEnv.SetFunction("apply", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: apply requiere al menos 2 argumentos"
		}
		fn := args[0]
		var callArgs []LispValue
		if len(args) == 2 {
			lst, ok := args[1].(LispList)
			if !ok {
				return "error: segundo argumento debe ser una lista"
			}
			callArgs = lst
		} else {
			callArgs = make([]LispValue, 0)
			for i := 1; i < len(args)-1; i++ {
				callArgs = append(callArgs, e.eval(args[i], env))
			}
			lastList, ok := args[len(args)-1].(LispList)
			if !ok {
				return "error: último argumento debe ser una lista"
			}
			callArgs = append(callArgs, lastList...)
		}
		return e.apply(fn, callArgs, env)
	}))

	e.globalEnv.SetFunction("funcall", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return nil
		}
		fn := args[0]
		callArgs := make([]LispValue, len(args)-1)
		for i := 1; i < len(args); i++ {
			callArgs[i-1] = e.eval(args[i], env)
		}
		return e.apply(fn, callArgs, env)
	}))

	e.globalEnv.SetFunction("function", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return nil
		}
		return args[0]
	}))

	e.globalEnv.SetFunction("cadr", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return nil
		}
		lst := e.eval(args[0], env)
		if l, ok := lst.(LispList); ok && len(l) >= 2 {
			return l[1]
		}
		return nil
	}))

	e.globalEnv.SetFunction("caddr", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return nil
		}
		lst := e.eval(args[0], env)
		if l, ok := lst.(LispList); ok && len(l) >= 3 {
			return l[2]
		}
		return nil
	}))

	e.globalEnv.SetFunction("cadddr", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return nil
		}
		lst := e.eval(args[0], env)
		if l, ok := lst.(LispList); ok && len(l) >= 4 {
			return l[3]
		}
		return nil
	}))

	e.globalEnv.SetFunction("remove-if-not", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		fn := args[0]
		lst, ok := args[1].(LispList)
		if !ok {
			return nil
		}
		result := make(LispList, 0)
		for _, item := range lst {
			test := e.apply(fn, []LispValue{item}, env)
			if isTruthy(test) {
				result = append(result, item)
			}
		}
		return result
	}))

	e.globalEnv.SetFunction("reduce", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		fn := args[0]
		lst, ok := args[1].(LispList)
		if !ok {
			return nil
		}
		if len(lst) == 0 {
			return nil
		}
		result := lst[0]
		for i := 1; i < len(lst); i++ {
			result = e.apply(fn, []LispValue{result, lst[i]}, env)
		}
		return result
	}))

	e.globalEnv.SetFunction("every", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return true
		}
		fn := args[0]
		lst, ok := args[1].(LispList)
		if !ok {
			return false
		}
		for _, item := range lst {
			if !isTruthy(e.apply(fn, []LispValue{item}, env)) {
				return false
			}
		}
		return true
	}))

	e.globalEnv.SetFunction("some", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return false
		}
		fn := args[0]
		lst, ok := args[1].(LispList)
		if !ok {
			return false
		}
		for _, item := range lst {
			if isTruthy(e.apply(fn, []LispValue{item}, env)) {
				return true
			}
		}
		return false
	}))

	e.globalEnv.SetFunction("lambda", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: lambda requiere parámetros y cuerpo"
		}
		paramsValue := args[0]
		body := args[1:]
		var params []LispSymbol
		if paramList, ok := paramsValue.(LispList); ok {
			for _, p := range paramList {
				if sym, ok := p.(LispSymbol); ok {
					params = append(params, sym)
				}
			}
		}
		return LispUserFunction{
			Params: params,
			Body:   body,
			Env:    env,
		}
	}))

	e.globalEnv.Set("t", true)
	e.globalEnv.Set("nil", nil)

	e.globalEnv.SetFunction("while", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		var result LispValue = nil
		for {
			test := e.eval(args[0], env)
			if !isTruthy(test) {
				break
			}
			for i := 1; i < len(args); i++ {
				result = e.eval(args[i], env)
			}
		}
		return result
	}))

	e.globalEnv.SetFunction("dotimes", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		binding, ok := args[0].(LispList)
		if !ok || len(binding) < 2 {
			return "error: dotimes requiere (variable n)"
		}
		varName, ok := binding[0].(LispSymbol)
		if !ok {
			return "error: dotimes requiere un símbolo como variable"
		}
		nVal := e.eval(binding[1], env)
		n, ok := nVal.(float64)
		if !ok {
			return "error: dotimes requiere un número como límite"
		}
		body := args[1:]
		var result LispValue = nil
		oldEnv := env
		for i := 0; i < int(n); i++ {
			env.Set(varName, float64(i))
			for _, expr := range body {
				result = e.eval(expr, env)
			}
		}
		env = oldEnv
		return result
	}))

	e.globalEnv.SetFunction("dolist", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		binding, ok := args[0].(LispList)
		if !ok || len(binding) < 2 {
			return "error: dolist requiere (variable lista)"
		}
		varName, ok := binding[0].(LispSymbol)
		if !ok {
			return "error: dolist requiere un símbolo como variable"
		}
		listVal := e.eval(binding[1], env)
		lst, ok := listVal.(LispList)
		if !ok {
			return "error: dolist requiere una lista para iterar"
		}
		body := args[1:]
		var result LispValue = nil
		for _, item := range lst {
			env.Set(varName, item)
			for _, expr := range body {
				result = e.eval(expr, env)
			}
		}
		env.Set(varName, nil)
		return result
	}))

	e.globalEnv.SetFunction("current-time", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		return float64(time.Now().Unix())
	}))

	e.globalEnv.SetFunction("strcat", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		result := ""
		for _, arg := range args {
			result += fmt.Sprintf("%v", e.eval(arg, env))
		}
		return result
	}))

	e.globalEnv.SetFunction("assoc", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		key := fmt.Sprintf("%v", e.eval(args[0], env))
		lst, ok := e.eval(args[1], env).(LispList)
		if !ok {
			return nil
		}
		for _, item := range lst {
			if pair, ok := item.(LispList); ok && len(pair) > 0 {
				if fmt.Sprintf("%v", pair[0]) == key {
					return pair
				}
			}
		}
		return nil
	}))

	// En Evaluator.initBuiltins(), agrega:
	e.globalEnv.SetFunction("getenv", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: getenv requiere un nombre de variable"
		}
		varName := fmt.Sprintf("%v", e.eval(args[0], env))
		val := os.Getenv(varName)
		if val == "" {
			return nil // o return "" para cadena vacía
		}
		return val
	}))

	// También agrega setenv para completar
	e.globalEnv.SetFunction("setenv", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: setenv requiere nombre y valor"
		}
		varName := fmt.Sprintf("%v", e.eval(args[0], env))
		varValue := fmt.Sprintf("%v", e.eval(args[1], env))
		os.Setenv(varName, varValue)
		return "ok"
	}))

	// =====================================================================
	// ZYRION Y PRIMITIVAS DE IA
	// =====================================================================

	e.registerUnifiedFunctions()
	e.registerPowerBuiltins()
	e.registerEvaluarZyrion()

	e.globalEnv.SetFunction("zyrion", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) == 0 {
			return float64(0)
		}
		list, ok := args[0].(LispList)
		if !ok {
			return float64(0)
		}
		hasPartial := false
		sum := 0.0
		total := 0.0
		for _, item := range list {
			var v float64
			switch val := item.(type) {
			case float64:
				v = val
			case int:
				v = float64(val)
			default:
				v = 0
			}
			if v == 0 || v == 1 || v == 2 {
			} else {
				v = 0
			}
			if v == 2 {
				hasPartial = true
			} else {
				sum += v
				total++
			}
		}
		if hasPartial {
			return float64(2)
		}
		if total == 0 {
			return float64(0)
		}
		if sum == 0 {
			return float64(0)
		}
		if sum == total {
			return float64(1)
		}
		return float64(2)
	}))

	e.globalEnv.SetFunction("zyrion-network", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: se requieren topology y externals"
		}
		topology, ok := args[0].(LispList)
		if !ok {
			return "error: topology debe ser una lista"
		}
		externals, ok := args[1].(LispList)
		if !ok {
			return "error: externals debe ser una lista"
		}
		extVals := make([]float64, len(externals))
		for i, v := range externals {
			f, ok := v.(float64)
			if !ok {
				return fmt.Sprintf("error: external[%d] no es número", i)
			}
			if f != 0 && f != 1 && f != 2 {
				f = 0
			}
			extVals[i] = f
		}
		results := make([]float64, len(topology))
		for i, topItem := range topology {
			inputs, ok := topItem.(LispList)
			if !ok {
				return fmt.Sprintf("error: topology[%d] no es una lista", i)
			}
			var entradas []float64
			for _, idxVal := range inputs {
				idx, ok := idxVal.(float64)
				if !ok {
					return fmt.Sprintf("error: índice no numérico en topology[%d]", i)
				}
				idxInt := int(idx)
				if idxInt < 0 {
					extIdx := -idxInt - 1
					if extIdx < 0 || extIdx >= len(extVals) {
						return fmt.Sprintf("error: índice externo %d fuera de rango", idxInt)
					}
					entradas = append(entradas, extVals[extIdx])
				} else {
					if idxInt >= len(results) {
						return fmt.Sprintf("error: índice interno %d fuera de rango", idxInt)
					}
					entradas = append(entradas, results[idxInt])
				}
			}
			zyrionFn, _ := e.globalEnv.LookupFunction("zyrion")
			if zyrionFn == nil {
				return "error: función zyrion no encontrada"
			}
			inputList := floatSliceToList(entradas)
			res := e.apply(zyrionFn, []LispValue{inputList}, env)
			rFloat, ok := res.(float64)
			if !ok {
				return fmt.Sprintf("error: zyrion devolvió %T, se esperaba número", res)
			}
			results[i] = rFloat
		}
		return floatSliceToList(results)
	}))

	e.globalEnv.SetFunction("zyrion-network-parallel", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: se requieren topology y lista-de-externals"
		}
		topology, ok := args[0].(LispList)
		if !ok {
			return "error: topology debe ser una lista"
		}
		externalsList, ok := args[1].(LispList)
		if !ok {
			return "error: externals debe ser una lista de listas"
		}
		var topo [][]int
		for _, item := range topology {
			sub, ok := item.(LispList)
			if !ok {
				return fmt.Sprintf("error: elemento de topology no es lista: %v", item)
			}
			indices := make([]int, len(sub))
			for j, idxVal := range sub {
				idxFloat, ok := idxVal.(float64)
				if !ok {
					return fmt.Sprintf("error: índice no numérico en topology: %v", idxVal)
				}
				indices[j] = int(idxFloat)
			}
			topo = append(topo, indices)
		}
		externalSets := make([][]float64, len(externalsList))
		for i, extList := range externalsList {
			lst, ok := extList.(LispList)
			if !ok {
				return fmt.Sprintf("error: externalSet[%d] no es lista", i)
			}
			vals := make([]float64, len(lst))
			for j, v := range lst {
				f, ok := v.(float64)
				if !ok {
					return fmt.Sprintf("error: external[%d][%d] no es número", i, j)
				}
				if f != 0 && f != 1 && f != 2 {
					f = 0
				}
				vals[j] = f
			}
			externalSets[i] = vals
		}
		evaluateOne := func(externals []float64) []float64 {
			results := make([]float64, len(topo))
			for i, node := range topo {
				var entradas []float64
				for _, idx := range node {
					if idx < 0 {
						extIdx := -idx - 1
						if extIdx >= len(externals) {
							entradas = append(entradas, 0)
						} else {
							entradas = append(entradas, externals[extIdx])
						}
					} else {
						if idx >= len(results) {
							entradas = append(entradas, 0)
						} else {
							entradas = append(entradas, results[idx])
						}
					}
				}
				zyrionFn, _ := e.globalEnv.LookupFunction("zyrion")
				if zyrionFn == nil {
					hasPartial := false
					sum := 0.0
					total := 0.0
					for _, v := range entradas {
						if v == 2 {
							hasPartial = true
						} else {
							sum += v
							total++
						}
					}
					if hasPartial {
						results[i] = 2
					} else if total == 0 {
						results[i] = 0
					} else if sum == 0 {
						results[i] = 0
					} else if sum == total {
						results[i] = 1
					} else {
						results[i] = 2
					}
					continue
				}
				inputList := make(LispList, len(entradas))
				for j, v := range entradas {
					inputList[j] = v
				}
				res := e.apply(zyrionFn, []LispValue{inputList}, env)
				rFloat, ok := res.(float64)
				if !ok {
					results[i] = 0
				} else {
					results[i] = rFloat
				}
			}
			return results
		}
		type result struct {
			index int
			out   []float64
		}
		resultChan := make(chan result, len(externalSets))
		var wg sync.WaitGroup
		for idx, ext := range externalSets {
			wg.Add(1)
			go func(i int, extVals []float64) {
				defer wg.Done()
				out := evaluateOne(extVals)
				resultChan <- result{i, out}
			}(idx, ext)
		}
		go func() {
			wg.Wait()
			close(resultChan)
		}()
		resultsSlice := make([][]float64, len(externalSets))
		for res := range resultChan {
			resultsSlice[res.index] = res.out
		}
		finalList := make(LispList, len(resultsSlice))
		for i, row := range resultsSlice {
			rowList := make(LispList, len(row))
			for j, v := range row {
				rowList[j] = v
			}
			finalList[i] = rowList
		}
		return finalList
	}))

	e.globalEnv.SetFunction("expandir-fractal", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: se requiere topología y niveles (entero)"
		}
		topology, ok := args[0].(LispList)
		if !ok {
			return "error: topology debe ser una lista"
		}
		levelsFloat, ok := args[1].(float64)
		if !ok {
			return "error: niveles debe ser un número entero"
		}
		levels := int(levelsFloat)
		if levels < 0 {
			return "error: niveles debe ser >= 0"
		}
		var convert func(LispList) ([][]int, error)
		convert = func(lst LispList) ([][]int, error) {
			var result [][]int
			for _, item := range lst {
				sub, ok := item.(LispList)
				if !ok {
					return nil, fmt.Errorf("elemento no es lista")
				}
				var indices []int
				for _, idxVal := range sub {
					idxFloat, ok := idxVal.(float64)
					if !ok {
						if sym, ok := idxVal.(LispSymbol); ok && sym == "self" {
							indices = append(indices, -1)
							continue
						}
						return nil, fmt.Errorf("índice no numérico: %v", idxVal)
					}
					indices = append(indices, int(idxFloat))
				}
				result = append(result, indices)
			}
			return result, nil
		}
		baseTopo, err := convert(topology)
		if err != nil {
			return fmt.Sprintf("error en topología base: %v", err)
		}
		var expand func(topo [][]int, depth int) [][]int
		expand = func(topo [][]int, depth int) [][]int {
			if depth <= 0 {
				var result [][]int
				for _, node := range topo {
					newIndices := make([]int, 0, len(node))
					for _, idx := range node {
						if idx == -1 {
							continue
						}
						newIndices = append(newIndices, idx)
					}
					result = append(result, newIndices)
				}
				return result
			}
			var result [][]int
			for _, node := range topo {
				var newIndices []int
				for _, idx := range node {
					if idx == -1 {
						subTopo := expand(baseTopo, depth-1)
						offset := len(result)
						result = append(result, subTopo...)
						for i := 0; i < len(subTopo); i++ {
							newIndices = append(newIndices, offset+i)
						}
					} else {
						newIndices = append(newIndices, idx)
					}
				}
				if len(newIndices) > 0 {
					result = append(result, newIndices)
				}
			}
			return result
		}
		expanded := expand(baseTopo, levels)
		resultList := make(LispList, len(expanded))
		for i, node := range expanded {
			nodeList := make(LispList, len(node))
			for j, idx := range node {
				nodeList[j] = float64(idx)
			}
			resultList[i] = nodeList
		}
		return resultList
	}))

	e.globalEnv.SetFunction("contar-zyrions", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: se requiere topología"
		}
		topo, ok := args[0].(LispList)
		if !ok {
			return "error: topology debe ser una lista"
		}
		count := 0
		for range topo {
			count++
		}
		return float64(count)
	}))

	e.globalEnv.SetFunction("topologia-aleatoria", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: se requieren num-nodos y num-entradas-externas"
		}
		numNodesFloat, ok := args[0].(float64)
		if !ok {
			return "error: num-nodos debe ser número"
		}
		numInputsFloat, ok := args[1].(float64)
		if !ok {
			return "error: num-entradas debe ser número"
		}
		n := int(numNodesFloat)
		m := int(numInputsFloat)
		topo := make(LispList, n)
		for i := 0; i < n; i++ {
			numConns := 1 + mathrand.Intn(3)
			conns := make(LispList, numConns)
			for j := 0; j < numConns; j++ {
				if mathrand.Float64() < 0.5 && m > 0 {
					extIdx := -(1 + mathrand.Intn(m))
					conns[j] = float64(extIdx)
				} else if i > 0 {
					prev := mathrand.Intn(i)
					conns[j] = float64(prev)
				} else {
					conns[j] = float64(-1)
				}
			}
			topo[i] = conns
		}
		return topo
	}))

	e.globalEnv.SetFunction("mutar-topologia", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: se requieren topologia y probabilidad"
		}
		topo, ok := args[0].(LispList)
		if !ok {
			return "error: topologia debe ser lista"
		}
		probFloat, ok := args[1].(float64)
		if !ok {
			return "error: probabilidad debe ser número"
		}
		prob := probFloat
		newTopo := make(LispList, len(topo))
		for i, node := range topo {
			nodeList, ok := node.(LispList)
			if !ok {
				return "error: nodo no es lista"
			}
			newConn := make(LispList, len(nodeList))
			for j, conn := range nodeList {
				newConn[j] = conn
				if mathrand.Float64() < prob {
					if mathrand.Float64() < 0.5 {
						extIdx := -(1 + mathrand.Intn(3))
						newConn[j] = float64(extIdx)
					} else if i > 0 {
						prev := mathrand.Intn(i)
						newConn[j] = float64(prev)
					} else {
						newConn[j] = float64(-1)
					}
				}
			}
			newTopo[i] = newConn
		}
		return newTopo
	}))

	e.globalEnv.SetFunction("cruzar-topologias", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: se requieren topologia1 y topologia2"
		}
		topo1, ok := args[0].(LispList)
		if !ok {
			return "error: topologia1 no es lista"
		}
		topo2, ok := args[1].(LispList)
		if !ok {
			return "error: topologia2 no es lista"
		}
		minLen := len(topo1)
		if len(topo2) < minLen {
			minLen = len(topo2)
		}
		if minLen == 0 {
			return LispList{}
		}
		crossoverPoint := mathrand.Intn(minLen)
		newTopo := make(LispList, minLen)
		for i := 0; i < minLen; i++ {
			if i < crossoverPoint {
				newTopo[i] = topo1[i]
			} else {
				newTopo[i] = topo2[i]
			}
		}
		return newTopo
	}))

	e.globalEnv.SetFunction("evolucionar-xor", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		popSize := 100
		gens := 200
		mutProb := 0.2
		if len(args) >= 1 {
			if f, ok := args[0].(float64); ok {
				popSize = int(f)
			}
		}
		if len(args) >= 2 {
			if f, ok := args[1].(float64); ok {
				gens = int(f)
			}
		}
		if len(args) >= 3 {
			if f, ok := args[2].(float64); ok {
				mutProb = f
			}
		}
		xorInputs := []LispList{
			{0.0, 0.0},
			{0.0, 1.0},
			{1.0, 0.0},
			{1.0, 1.0},
		}
		xorOutputs := []float64{0.0, 1.0, 1.0, 0.0}
		entradasLisp := make(LispList, len(xorInputs))
		for i, inp := range xorInputs {
			entradasLisp[i] = inp
		}
		salidasLisp := make(LispList, len(xorOutputs))
		for i, out := range xorOutputs {
			salidasLisp[i] = out
		}
		randTopoFn, _ := e.globalEnv.LookupFunction("topologia-aleatoria")
		mutarFn, _ := e.globalEnv.LookupFunction("mutar-topologia")
		cruzarFn, _ := e.globalEnv.LookupFunction("cruzar-topologias")
		zyrionNetworkFn, _ := e.globalEnv.LookupFunction("zyrion-network")
		if randTopoFn == nil || mutarFn == nil || cruzarFn == nil || zyrionNetworkFn == nil {
			return "error: faltan funciones primitivas"
		}
		numNodos := 7
		poblacion := make([]LispValue, popSize)
		for i := 0; i < popSize; i++ {
			poblacion[i] = e.apply(randTopoFn, []LispValue{float64(numNodos), float64(2)}, env)
		}
		var mejorTopologia LispValue = nil
		mejorFitness := -1.0
		for gen := 0; gen < gens; gen++ {
			fitnesses := make([]float64, popSize)
			for i, indiv := range poblacion {
				maxAciertos := 0.0
				for outputIdx := 0; outputIdx < numNodos; outputIdx++ {
					aciertos := 0.0
					total := 0.0
					for idx := 0; idx < len(entradasLisp); idx++ {
						entradas := entradasLisp[idx]
						esperado := salidasLisp[idx]
						res := e.apply(zyrionNetworkFn, []LispValue{indiv, entradas}, env)
						resList, ok := res.(LispList)
						if !ok || len(resList) <= outputIdx {
							continue
						}
						salida := resList[outputIdx]
						salidaFloat, ok := salida.(float64)
						if !ok {
							continue
						}
						esperadoFloat, ok := esperado.(float64)
						if !ok {
							continue
						}
						if salidaFloat == esperadoFloat {
							aciertos++
						}
						total++
					}
					if total > 0 {
						fit := aciertos / total
						if fit > maxAciertos {
							maxAciertos = fit
						}
					}
				}
				fitnesses[i] = maxAciertos
			}
			bestIdx := 0
			for i := 1; i < popSize; i++ {
				if fitnesses[i] > fitnesses[bestIdx] {
					bestIdx = i
				}
			}
			if fitnesses[bestIdx] > mejorFitness {
				mejorFitness = fitnesses[bestIdx]
				mejorTopologia = poblacion[bestIdx]
			}
			fmt.Printf("Gen %d: mejor fitness = %.4f\n", gen, fitnesses[bestIdx])
			if mejorFitness >= 0.999 {
				break
			}
			nuevaPoblacion := make([]LispValue, 0, popSize)
			nuevaPoblacion = append(nuevaPoblacion, poblacion[bestIdx])
			for len(nuevaPoblacion) < popSize {
				i1 := mathrand.Intn(popSize)
				i2 := mathrand.Intn(popSize)
				padre1 := poblacion[i1]
				padre2 := poblacion[i2]
				if fitnesses[i1] < fitnesses[i2] {
					padre1, padre2 = padre2, padre1
				}
				hijo := e.apply(cruzarFn, []LispValue{padre1, padre2}, env)
				if mathrand.Float64() < mutProb {
					hijo = e.apply(mutarFn, []LispValue{hijo, mutProb}, env)
				}
				nuevaPoblacion = append(nuevaPoblacion, hijo)
			}
			poblacion = nuevaPoblacion
		}
		if mejorTopologia == nil {
			return "no se encontró ninguna topología"
		}
		return mejorTopologia
	}))

	e.globalEnv.SetFunction("fitness-topologia", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 3 {
			return "error: se requieren topologia, lista-entradas, lista-salidas-esperadas"
		}
		topo, ok := args[0].(LispList)
		if !ok {
			return "error: topologia no es lista"
		}
		entradasList, ok := args[1].(LispList)
		if !ok {
			return "error: lista-entradas debe ser lista de listas"
		}
		esperadosList, ok := args[2].(LispList)
		if !ok {
			return "error: lista-salidas-esperadas debe ser lista de números"
		}
		if len(entradasList) != len(esperadosList) {
			return "error: número de casos diferente"
		}
		zyrionNetworkFn, _ := e.globalEnv.LookupFunction("zyrion-network")
		if zyrionNetworkFn == nil {
			return "error: falta zyrion-network"
		}
		aciertos := 0.0
		total := 0.0
		for idx := 0; idx < len(entradasList); idx++ {
			entradas := entradasList[idx]
			esperadoVal := esperadosList[idx]
			res := e.apply(zyrionNetworkFn, []LispValue{topo, entradas}, env)
			resList, ok := res.(LispList)
			if !ok || len(resList) == 0 {
				continue
			}
			salida := resList[len(resList)-1]
			salidaFloat, ok := salida.(float64)
			if !ok {
				continue
			}
			esperadoFloat, ok := esperadoVal.(float64)
			if !ok {
				continue
			}
			if salidaFloat == esperadoFloat {
				aciertos++
			}
			total++
		}
		if total == 0 {
			return 0.0
		}
		return aciertos / total
	}))
}

// =============================================================================
// FUNCIONES UNIFICADAS (GENERAR PÁGINA, OBTENER BALANCE, ETC.)
// =============================================================================

func (e *Evaluator) registerUnifiedFunctions() {
	e.globalEnv.SetFunction("generar-pagina-web", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: requiere nombre-agente"
		}
		agenteID := fmt.Sprintf("%v", e.eval(args[0], env))
		html := generarHTMLBase(agenteID)
		cid, err := e.host.GenerarCID([]byte(html))
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		e.host.SetAgentRoot(agenteID, cid)
		return cid
	}))

	e.globalEnv.SetFunction("get-agent-balance", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return 0.0
		}
		agentID := fmt.Sprintf("%v", e.eval(args[0], env))
		e.host.RLock()
		defer e.host.RUnlock()
		if a, ok := e.host.GetAgent(agentID); ok {
			return a.BalanceUTXO
		}
		return 0.0
	}))

	e.globalEnv.SetFunction("inventario-listar", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		resultado := make(LispList, 0)
		e.host.RLock()
		defer e.host.RUnlock()
		for cid := range e.host.ListBlocks() {
			resultado = append(resultado, cid)
		}
		return resultado
	}))

	e.globalEnv.SetFunction("crear-relacion", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 3 {
			return "error: requiere tipo, origen y destino"
		}
		tipo := fmt.Sprintf("%v", e.eval(args[0], env))
		origen := fmt.Sprintf("%v", e.eval(args[1], env))
		destino := fmt.Sprintf("%v", e.eval(args[2], env))
		cardinalidad := "*..*"
		if len(args) >= 4 {
			cardinalidad = fmt.Sprintf("%v", e.eval(args[3], env))
		}
		relacionId := generarUUID()
		relacion := &agents.RelacionEntidad{
			ID:           relacionId,
			EntidadA:     origen,
			EntidadB:     destino,
			Tipo:         tipo,
			Cardinalidad: cardinalidad,
		}
		agents.Global.Relaciones[relacionId] = relacion
		e.host.Lock()
		if e.host.GetNeural() == nil {
			ns := e.host.EnsureNeural(); *ns = neural.NeuralState{
				Synapses: make(map[string]neural.SynapticWeight),
			}
		}
		e.host.GetNeural().Synapses[destino] = neural.SynapticWeight{
			TargetNeuronID: destino,
			Weight:         0.5,
			LastUpdated:    time.Now().Unix(),
		}
		e.host.Unlock()
		e.host.Auditoria("RELACION_CREADA", fmt.Sprintf("%s: %s -> %s (%s)", tipo, origen, destino, cardinalidad))
		return relacionId
	}))

	e.globalEnv.SetFunction("listar-relaciones", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		resultado := make(LispList, 0)
		for _, rel := range agents.Global.Relaciones {
			relData := map[string]interface{}{
				"id":           rel.ID,
				"tipo":         rel.Tipo,
				"origen":       rel.EntidadA,
				"destino":      rel.EntidadB,
				"cardinalidad": rel.Cardinalidad,
			}
			resultado = append(resultado, relData)
		}
		return resultado
	}))

	e.globalEnv.SetFunction("crear-entidad", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: requiere tipo y atributos"
		}
		tipo := fmt.Sprintf("%v", e.eval(args[0], env))
		atributosVal := e.eval(args[1], env)
		atributos := make(map[string]interface{})
		if m, ok := atributosVal.(map[string]interface{}); ok {
			atributos = m
		}
		id := generarUUID()
		entidad := &agents.EntidadProgramatica{
			ID:        id,
			Tipo:      tipo,
			Atributos: atributos,
			HeredaDe:  "",
			ModuloID:  "editor",
		}
		agents.Global.MuEntidades.Lock()
		agents.Global.Entidades[id] = entidad
		agents.Global.MuEntidades.Unlock()
		e.host.Auditoria("ENTIDAD_CREADA", fmt.Sprintf("Tipo: %s | ID: %s", tipo, id))
		return id
	}))

	e.globalEnv.SetFunction("listar-entidades", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		resultado := make(LispList, 0)
		agents.Global.MuEntidades.RLock()
		defer agents.Global.MuEntidades.RUnlock()
		for _, ent := range agents.Global.Entidades {
			entData := map[string]interface{}{
				"id":        ent.ID,
				"tipo":      ent.Tipo,
				"atributos": ent.Atributos,
				"hereda_de": ent.HeredaDe,
			}
			resultado = append(resultado, entData)
		}
		return resultado
	}))

	e.globalEnv.SetFunction("crear-app-desde-plantilla", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: requiere nombre y tipo"
		}
		nombre := fmt.Sprintf("%v", e.eval(args[0], env))
		tipo := fmt.Sprintf("%v", e.eval(args[1], env))
		agentID := fmt.Sprintf("%s-%d", strings.ReplaceAll(strings.ToLower(nombre), " ", "-"), time.Now().Unix())
		e.host.Lock()
		if _, existe := e.host.GetAgent(agentID); !existe {
			e.host.PutAgent(&agents.Agente{
				ID:           agentID,
				RootCID:      "",
				UltimaActual: time.Now().Unix(),
				BalanceUTXO:  1000.0,
			})
		}
		e.host.Unlock()
		html := generarHTMLParaPlantilla(nombre, tipo)
		cid, err := e.host.GenerarCID([]byte(html))
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		e.host.SetAgentRoot(agentID, cid)
		alias := strings.ReplaceAll(strings.ToLower(nombre), " ", "-") + ".negocio.ans"
		e.host.SetNombre(alias, agentID)
		e.host.Auditoria("APP_CREADA_DESDE_PLANTILLA", fmt.Sprintf("Nombre: %s | Tipo: %s | URL: /w/%s", nombre, tipo, alias))
		return fmt.Sprintf("✅ App creada: /w/%s", alias)
	}))
}

// =============================================================================
// FUNCIONES AUXILIARES DE EVALUACIÓN LISP
// =============================================================================

func (e *Evaluator) eval(expr LispValue, env *LispEnvironment) LispValue {
	switch v := expr.(type) {
	case nil:
		return nil
	case bool:
		return v
	case float64:
		return v
	case string:
		return v
	case LispSymbol:
		if val, ok := env.Lookup(v); ok {
			return val
		}
		if val, ok := env.LookupFunction(v); ok {
			return val
		}
		return v
	case LispList:
		if len(v) == 0 {
			return LispList{}
		}
		first, ok := v[0].(LispSymbol)
		if !ok {
			return e.evalList(v, env)
		}
		switch first {
		case "quote":
			if len(v) >= 2 {
				return v[1]
			}
			return nil
		case "if":
			if len(v) < 3 {
				return nil
			}
			test := e.eval(v[1], env)
			if isTruthy(test) {
				return e.eval(v[2], env)
			}
			if len(v) >= 4 {
				return e.eval(v[3], env)
			}
			return nil
		case "progn":
			var result LispValue = nil
			for i := 1; i < len(v); i++ {
				result = e.eval(v[i], env)
			}
			return result
		case "let":
			return e.evalSpecialLet(v, env)
		case "let*":
			return e.evalSpecialLetStar(v, env)
		case "lambda":
			return e.evalSpecialLambda(v, env)
		case "defun":
			return e.evalSpecialDefun(v, env)
		case "defmacro":
			return e.evalSpecialDefmacro(v, env)
		case "quasiquote":
			if len(v) >= 2 {
				return e.expandQuasiquote(v[1], env)
			}
			return nil
		default:
			fn := e.eval(v[0], env)
			args := make([]LispValue, len(v)-1)
			for i, arg := range v[1:] {
				args[i] = e.eval(arg, env)
			}
			return e.apply(fn, args, env)
		}
	default:
		return expr
	}
}

func (e *Evaluator) evalList(list LispList, env *LispEnvironment) LispValue {
	result := make(LispList, len(list))
	for i, item := range list {
		result[i] = e.eval(item, env)
	}
	return result
}

func (e *Evaluator) evalSpecialLet(list LispList, env *LispEnvironment) LispValue {
	if len(list) < 3 {
		return nil
	}
	bindings, ok := list[1].(LispList)
	if !ok {
		return nil
	}
	newEnv := NewLispEnvironment(env)
	for _, binding := range bindings {
		if pair, ok := binding.(LispList); ok && len(pair) >= 2 {
			if varName, ok := pair[0].(LispSymbol); ok {
				val := e.eval(pair[1], env)
				newEnv.Set(varName, val)
			}
		}
	}
	var result LispValue = nil
	for i := 2; i < len(list); i++ {
		result = e.eval(list[i], newEnv)
	}
	return result
}

func (e *Evaluator) evalSpecialLambda(list LispList, env *LispEnvironment) LispValue {
	if len(list) < 3 {
		return "error: lambda requiere parámetros y cuerpo"
	}
	paramsValue := list[1]
	body := list[2:]
	var params []LispSymbol
	if paramList, ok := paramsValue.(LispList); ok {
		for _, p := range paramList {
			if sym, ok := p.(LispSymbol); ok {
				params = append(params, sym)
			}
		}
	}
	return LispUserFunction{
		Params: params,
		Body:   body,
		Env:    env,
	}
}

func (e *Evaluator) evalSpecialDefmacro(list LispList, env *LispEnvironment) LispValue {
	if len(list) < 4 {
		return "error: defmacro requiere (nombre parámetros cuerpo)"
	}
	macroName, ok := list[1].(LispSymbol)
	if !ok {
		return "error: defmacro requiere símbolo como nombre"
	}
	var paramNames []LispSymbol
	hasRest := false
	restIndex := -1
	var restName LispSymbol
	if params, ok := list[2].(LispList); ok {
		for i, p := range params {
			if sym, ok := p.(LispSymbol); ok {
				s := string(sym)
				if s == "&rest" || s == "&body" {
					hasRest = true
					restIndex = i
					if i+1 < len(params) {
						if restSym, ok := params[i+1].(LispSymbol); ok {
							restName = restSym
						}
					}
					break
				}
				paramNames = append(paramNames, sym)
			}
		}
	}
	body := list[3:]
	macro := LispMacro(func(macroArgs []LispValue, macroEnv *LispEnvironment) LispValue {
		newEnv := NewLispEnvironment(macroEnv)
		if hasRest && restName != "" {
			for i := 0; i < restIndex; i++ {
				if i < len(macroArgs) {
					newEnv.Set(paramNames[i], macroArgs[i])
				} else {
					newEnv.Set(paramNames[i], nil)
				}
			}
			restArgs := make(LispList, 0)
			for i := restIndex; i < len(macroArgs); i++ {
				restArgs = append(restArgs, macroArgs[i])
			}
			newEnv.Set(restName, restArgs)
		} else {
			for i, p := range paramNames {
				if i < len(macroArgs) {
					newEnv.Set(p, macroArgs[i])
				} else {
					newEnv.Set(p, nil)
				}
			}
		}
		var expansion LispValue = nil
		for _, expr := range body {
			expansion = e.eval(expr, newEnv)
		}
		expansion = cleanMacroExpansion(expansion)
		return fixExpansion(expansion)
	})
	e.globalEnv.functions[macroName] = macro
	fmt.Printf("📦 Macro definida: %s\n", macroName)
	return macroName
}

func fixExpansion(expr LispValue) LispValue {
	switch v := expr.(type) {
	case LispList:
		result := make(LispList, len(v))
		for i, item := range v {
			result[i] = fixExpansion(item)
		}
		return result
	case LispSymbol:
		return v
	case string:
		if v == "if" || v == "progn" || v == "quote" {
			return LispSymbol(v)
		}
		return v
	case nil:
		return nil
	default:
		return v
	}
}

func (e *Evaluator) evalSpecialDefun(list LispList, env *LispEnvironment) LispValue {
	if len(list) < 4 {
		return "error: defun requiere nombre, parámetros y cuerpo"
	}
	funcName, ok := list[1].(LispSymbol)
	if !ok {
		return "error: defun requiere un símbolo como nombre"
	}
	paramsValue := list[2]
	body := list[3:]
	var params []LispSymbol
	if paramList, ok := paramsValue.(LispList); ok {
		for _, p := range paramList {
			if sym, ok := p.(LispSymbol); ok {
				params = append(params, sym)
			}
		}
	}
	userFunc := LispUserFunction{
		Params: params,
		Body:   body,
		Env:    env,
	}
	env.SetFunction(funcName, userFunc)
	return funcName
}

func (e *Evaluator) apply(fn LispValue, args []LispValue, env *LispEnvironment) LispValue {
	switch f := fn.(type) {
	case LispFunction:
		return f(args, env)
	case LispUserFunction:
		newEnv := NewLispEnvironment(f.Env)
		for i, param := range f.Params {
			if i < len(args) {
				newEnv.Set(param, args[i])
			} else {
				newEnv.Set(param, nil)
			}
		}
		var result LispValue = nil
		for _, expr := range f.Body {
			result = e.eval(expr, newEnv)
		}
		return result
	default:
		return fmt.Sprintf("error: %v no es una función", fn)
	}
}

func (e *Evaluator) evalSpecialLetStar(list LispList, env *LispEnvironment) LispValue {
	if len(list) < 3 {
		return nil
	}
	bindings, ok := list[1].(LispList)
	if !ok {
		return nil
	}
	newEnv := NewLispEnvironment(env)
	for _, binding := range bindings {
		if pair, ok := binding.(LispList); ok && len(pair) >= 2 {
			if varName, ok := pair[0].(LispSymbol); ok {
				val := e.eval(pair[1], newEnv)
				newEnv.Set(varName, val)
			}
		}
	}
	var result LispValue = nil
	for i := 2; i < len(list); i++ {
		result = e.eval(list[i], newEnv)
	}
	return result
}

func cleanMacroExpansion(expr LispValue) LispValue {
	switch v := expr.(type) {
	case LispList:
		result := make(LispList, len(v))
		for i, item := range v {
			result[i] = cleanMacroExpansion(item)
		}
		return result
	case LispFunction:
		return LispSymbol("progn")
	default:
		return v
	}
}

func canonicalizeJSON(data interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	enc := json.NewEncoder(buffer)
	enc.SetEscapeHTML(false)
	if err := encodeCanonical(buffer, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodeCanonical(w *bytes.Buffer, v interface{}) error {
	switch val := v.(type) {
	case map[string]interface{}:
		w.WriteByte('{')
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				w.WriteByte(',')
			}
			keyBytes, _ := json.Marshal(k)
			w.Write(keyBytes)
			w.WriteByte(':')
			if err := encodeCanonical(w, val[k]); err != nil {
				return err
			}
		}
		w.WriteByte('}')
	case []interface{}:
		w.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				w.WriteByte(',')
			}
			if err := encodeCanonical(w, item); err != nil {
				return err
			}
		}
		w.WriteByte(']')
	case string:
		b, _ := json.Marshal(val)
		w.Write(b)
	case float64:
		w.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
	case bool:
		if val {
			w.WriteString("true")
		} else {
			w.WriteString("false")
		}
	case nil:
		w.WriteString("null")
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		w.Write(b)
	}
	return nil
}

func toFloat(v LispValue) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

func isTruthy(v LispValue) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if f, ok := v.(float64); ok {
		return f != 0
	}
	if s, ok := v.(string); ok {
		return s != ""
	}
	if _, ok := v.(LispList); ok {
		return true
	}
	return true
}

func equalValue(a, b LispValue) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func generarUUID() string {
	return hex.EncodeToString([]byte(time.Now().String()))[:16]
}

func floatSliceToList(vals []float64) LispList {
	lst := make(LispList, len(vals))
	for i, v := range vals {
		lst[i] = v
	}
	return lst
}

func generarHTMLBase(agenteID string) string {
	return `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>` + agenteID + ` - Alset App</title><style>*{margin:0;padding:0;box-sizing:border-box;}body{font-family:system-ui;background:#0A0A0A;color:#FFF;}.header{background:#141414;padding:1rem 2rem;border-bottom:2px solid #F4B400;}.container{padding:2rem;max-width:1200px;margin:0 auto;}.card{background:#141414;border-radius:12px;padding:1.5rem;margin-bottom:1.5rem;border:1px solid rgba(244,180,0,0.2);}button{background:#F4B400;border:none;padding:0.5rem 1rem;border-radius:8px;cursor:pointer;}</style></head><body><div class="header"><h1>` + agenteID + `</h1></div><div class="container" id="app"><div class="card"><h3>Bienvenido</h3><p>App generada por Alset</p></div></div></body></html>`
}

func generarHTMLParaPlantilla(nombre string, tipo string) string {
	return `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>` + nombre + ` - Alset App</title><style>*{margin:0;padding:0;box-sizing:border-box;}body{font-family:system-ui;background:#0A0A0A;color:#FFF;}.header{background:#141414;padding:2rem;text-align:center;border-bottom:3px solid #F4B400;}.header h1{font-size:2rem;}.container{padding:2rem;max-width:1200px;margin:0 auto;}.card{background:#141414;border-radius:12px;padding:1.5rem;margin-bottom:1.5rem;border:1px solid rgba(244,180,0,0.2);}button{background:#F4B400;border:none;padding:0.5rem 1rem;border-radius:8px;cursor:pointer;}</style></head><body><div class="header"><h1>` + nombre + `</h1><p>Tu app en Alset Network</p></div><div class="container"><div class="card"><h3>Bienvenido</h3><p>App tipo: ` + tipo + `</p><button onclick="alert('App funcionando')">Probar</button></div></div></body></html>`
}

// =============================================================================
