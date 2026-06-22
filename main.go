package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	mathrand "math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"encoding/base64"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/multiformats/go-multihash"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"

	"golang.org/x/crypto/bcrypt"
)

// =============================================================================
// CONSTANTES
// =============================================================================

const AlsetProtocolID = "/ptec-an/sync/1.0.0"
const AlsetDataExchangeID = "/ptec-an/data/1.0.0"
const AlsetGossipTopic = "ptec-an-v4.0"
const BlocksDir = "blocks"
const StaticDir = "static"
const AdminPanelCIDKey = "admin_panel_cid"

const (
	NeuralSpikeTopic       = "ptec-an-neural-spike"
	InferenceRequestTopic  = "ptec-an-inference-request"
	InferenceResponseTopic = "ptec-an-inference-response"
	SynapticUpdateTopic    = "ptec-an-synaptic-update"
	MemoryQueryTopic       = "ptec-an-memory-query"
	MemoryResponseTopic    = "ptec-an-memory-response"
	NeuralStateSyncTopic   = "ptec-an-neural-sync"
	MemoryDistributedTopic = "ptec-an-memory-distributed"
)

// =============================================================================
// TIPOS ESTRUCTURALES
// =============================================================================

type Agente struct {
	ID           string  `json:"id"`
	RootCID      string  `json:"root_cid"`
	UltimaActual int64   `json:"ultima_actualizacion"`
	BalanceUTXO  float64 `json:"balance_utxo"`
}

type NodoConfig struct {
	AdminPassHash string `json:"admin_pass_hash"`
	LastUpdate    int64  `json:"last_update"`
	Version       string `json:"version"`
	AdminPanelCID string `json:"admin_panel_cid"`
	IsGenesis     bool   `json:"is_genesis"`
}

type PoHEvent struct {
	Timestamp   int64  `json:"timestamp"`
	EventType   string `json:"event_type"`
	Metadata    string `json:"metadata"`
	Signature   string `json:"signature,omitempty"`
	HumanitySig string `json:"humanity_sig"`
}

type HumanityProof struct {
	SessionID string     `json:"session_id"`
	Events    []PoHEvent `json:"events"`
	FinalSig  string     `json:"final_signature"`
}

var globalPoH = struct {
	sync.Mutex
	sessionID string
	events    []PoHEvent
}{
	sessionID: "",
	events:    []PoHEvent{},
}

type SynapticWeight struct {
	TargetNeuronID  string  `json:"target_neuron_id"`
	Weight          float64 `json:"weight"`
	LastUpdated     int64   `json:"last_updated"`
	SuccessfulFires int64   `json:"successful_fires"`
}

type NeuralState struct {
	MembranePotential float64                   `json:"membrane_potential"`
	LastSpikeTime     int64                     `json:"last_spike_time"`
	SpikeThreshold    float64                   `json:"spike_threshold"`
	LeakRate          float64                   `json:"leak_rate"`
	RefractoryPeriod  int64                     `json:"refractory_period"`
	Synapses          map[string]SynapticWeight `json:"synapses"`
	NeuronType        string                    `json:"neuron_type"`
}

type InferenceRequest struct {
	RequestID    string    `json:"request_id"`
	InputData    []float64 `json:"input_data"`
	OriginNodeID string    `json:"origin_node_id"`
	TTL          int       `json:"ttl"`
}

type InferenceResponse struct {
	RequestID      string    `json:"request_id"`
	OutputData     []float64 `json:"output_data"`
	ProcessingNode string    `json:"processing_node"`
	ProcessingTime int64     `json:"processing_time"`
}

type MemoryQuery struct {
	QueryID    string `json:"query_id"`
	Content    string `json:"content"`
	OriginNode string `json:"origin_node"`
	TTL        int    `json:"ttl"`
}

type MemoryResponse struct {
	QueryID       string   `json:"query_id"`
	Results       []string `json:"results"`
	Contents      []string `json:"contents"`
	ResponderNode string   `json:"responder_node"`
}

// =============================================================================
// EXTENSIONES: MÓDULOS, ENTIDADES, SEGURIDAD
// =============================================================================

type Modulo struct {
	ID         string                 `json:"id"`
	Nombre     string                 `json:"nombre"`
	Rol        string                 `json:"rol"`
	Atributos  map[string]interface{} `json:"atributos"`
	Relaciones []string               `json:"relaciones"`
	RootCID    string                 `json:"root_cid"`
	Owner      string                 `json:"owner"`
	CreatedAt  int64                  `json:"created_at"`
}

type EntidadProgramatica struct {
	ID        string                 `json:"id"`
	Tipo      string                 `json:"tipo"`
	Atributos map[string]interface{} `json:"atributos"`
	HeredaDe  string                 `json:"hereda_de"`
	ModuloID  string                 `json:"modulo_id"`
}

type RelacionEntidad struct {
	ID           string `json:"id"`
	EntidadA     string `json:"entidad_a"`
	EntidadB     string `json:"entidad_b"`
	Tipo         string `json:"tipo"`
	Cardinalidad string `json:"cardinalidad"`
}

type TokenAlset struct {
	Token     string   `json:"token"`
	AgentID   string   `json:"agent_id"`
	RootCID   string   `json:"root_cid"`
	ExpiresAt int64    `json:"expires_at"`
	Roles     []string `json:"roles"`
	Permisos  []string `json:"permisos"`
	Signature string   `json:"signature"`
}

type UsuarioRoles struct {
	AgentID string   `json:"agent_id"`
	Roles   []string `json:"roles"`
	Modulos []string `json:"modulos"`
}

var (
	modulosGlobales    = make(map[string]*Modulo)
	entidadesGlobales  = make(map[string]*EntidadProgramatica)
	relacionesGlobales = make(map[string]*RelacionEntidad)
	tokensActivos      = make(map[string]*TokenAlset)
	rolesGlobales      = make(map[string][]string)
	muModulos          sync.RWMutex
	muEntidades        sync.RWMutex
	muTokens           sync.RWMutex
)

// =============================================================================
// SISTEMA DE SINCRONIZACIÓN HÍBRIDA
// =============================================================================

type SyncMode int

const (
	SyncModeQuick       SyncMode = 1
	SyncModeFull        SyncMode = 2
	SyncModeIncremental SyncMode = 3
)

type SyncConfig struct {
	Mode           SyncMode `json:"mode"`
	LastSyncTime   int64    `json:"last_sync_time"`
	AutoSyncDays   int      `json:"auto_sync_days"`
	MaxQuickBlocks int      `json:"max_quick_blocks"`
}

type SyncManager struct {
	nodo         *NodoAlset
	config       SyncConfig
	isSyncing    bool
	syncProgress float64
	syncCancel   context.CancelFunc
	mu           sync.RWMutex
}

type SyncProgress struct {
	Current int     `json:"current"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
	Status  string  `json:"status"`
	Stage   string  `json:"stage"`
}

var globalSyncProgress = &SyncProgress{
	Status: "idle",
	Stage:  "none",
}

// =============================================================================
// TIPOS LISP
// =============================================================================

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

type LispEvaluator struct {
	globalEnv *LispEnvironment
	nodo      *NodoAlset
	mu        sync.RWMutex
}

func NewLispEvaluator(nodo *NodoAlset) *LispEvaluator {
	eval := &LispEvaluator{
		globalEnv: NewLispEnvironment(nil),
		nodo:      nodo,
	}
	eval.initBuiltins()
	return eval
}

func (e *LispEvaluator) expandQuasiquote(expr LispValue, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) macroexpand1(form LispValue, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) macroexpand(form LispValue, env *LispEnvironment) LispValue {
	expanded := e.macroexpand1(form, env)
	if expanded == form {
		return expanded
	}
	return e.macroexpand(expanded, env)
}

func (e *LispEvaluator) expandMacros(expr LispValue, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) Eval(code string) (LispValue, error) {
	parser := NewLispParser(code)
	expr, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	expanded := e.expandMacros(expr, e.globalEnv)
	return e.eval(expanded, e.globalEnv), nil
}

func (e *LispEvaluator) ExpandDebug(code string) (LispValue, error) {
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

func (e *LispEvaluator) initBuiltins() {
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

		firmaBytes, err := e.nodo.masterPrivKey.Sign(canonicalBytes)
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

		certCID, err := e.nodo.GenerarCID(finalVCBytes)
		if err != nil {
			return fmt.Sprintf("error_cid: %v", err)
		}

		e.nodo.Auditoria("VC_EMITIDO", fmt.Sprintf("Doc: %s | VC: %s", cidOriginal, certCID))
		e.nodo.AnunciarNuevoBloque(certCID)
		return certCID
	}))

	e.globalEnv.SetFunction("revocar-certificado", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: requiere cid y motivo"
		}
		if e.nodo.topic == nil {
			return "error: el nodo no está unido al tópico de la red"
		}
		if e.nodo.masterPrivKey == nil {
			return "error: clave maestra no cargada"
		}

		certCID := fmt.Sprintf("%v", e.eval(args[0], env))
		motivo := fmt.Sprintf("%v", e.eval(args[1], env))
		fecha := time.Now().Format(time.RFC3339)

		mensaje := fmt.Sprintf("REVOKE|%s|%s", certCID, fecha)
		firma, err := e.nodo.masterPrivKey.Sign([]byte(mensaje))
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
		revCID, _ := e.nodo.GenerarCID(data)
		e.nodo.Auditoria("VC_REVOCADO", fmt.Sprintf("VC: %s | Motivo: %s", certCID, motivo))

		update := map[string]string{"tipo": "revocacion_update", "cid": revCID}
		msg, _ := json.Marshal(update)
		if err := e.nodo.topic.Publish(e.nodo.ctx, msg); err != nil {
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
		globalPoH.Lock()
		defer globalPoH.Unlock()

		if len(globalPoH.events) == 0 {
			return "No hay suficientes eventos de humanidad registrados"
		}

		var eventsData []byte
		for _, ev := range globalPoH.events {
			evData, _ := json.Marshal(ev)
			eventsData = append(eventsData, evData...)
		}

		hash := make([]byte, 32)
		copy(hash, eventsData[:32])

		proof := HumanityProof{
			SessionID: globalPoH.sessionID,
			Events:    globalPoH.events,
			FinalSig:  hex.EncodeToString(hash),
		}

		proofBytes, _ := json.Marshal(proof)
		proofCID, _ := e.nodo.GenerarCID(proofBytes)

		globalPoH.events = []PoHEvent{}

		return map[string]interface{}{
			"status":       "prueba_humanidad_generada",
			"proof_cid":    proofCID,
			"events_count": len(proof.Events),
			"session_id":   proof.SessionID,
		}
	}))

	e.globalEnv.SetFunction("emitir-sello-humanidad", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		globalPoH.Lock()
		defer globalPoH.Unlock()

		if globalPoH.sessionID == "" {
			globalPoH.sessionID = hex.EncodeToString([]byte(time.Now().String()))[:16]
		}

		eventType := "custom"
		metadata := ""
		if len(args) > 0 {
			eventType = fmt.Sprintf("%v", e.eval(args[0], env))
		}
		if len(args) > 1 {
			metadata = fmt.Sprintf("%v", e.eval(args[1], env))
		}

		event := PoHEvent{
			Timestamp: time.Now().Unix(),
			EventType: eventType,
			Metadata:  metadata,
		}

		globalPoH.events = append(globalPoH.events, event)

		return fmt.Sprintf("Evento de humanidad registrado (%d total)", len(globalPoH.events))
	}))

	// =====================================================================
	// FUNCIONES DE AGENTES, DNS, IPFS, LISP
	// =====================================================================

	e.globalEnv.SetFunction("crear-agente", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: id requerido"
		}
		agentID := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")

		e.nodo.mu.Lock()
		if _, existe := e.nodo.agentes[agentID]; !existe {
			e.nodo.agentes[agentID] = &Agente{
				ID:           agentID,
				RootCID:      "",
				UltimaActual: time.Now().Unix(),
				BalanceUTXO:  1000.0,
			}
			e.nodo.mu.Unlock()
			e.nodo.Auditoria("AGENTE_CREADO", "ID: "+agentID)
			e.nodo.PersistirLocamente()
			e.nodo.SincronizarConPares()
			// ---- PULSO: emitir evento ----
			go e.nodo.broadcastPulse("agent_created", map[string]interface{}{
				"id":   agentID,
				"root": "",
				"time": time.Now().Unix(),
			})
			return "Agente " + agentID + " creado"
		}
		e.nodo.mu.Unlock()
		return "error: ya existe"
	}))

	e.globalEnv.SetFunction("set-agent-root", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error"
		}
		agentID := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		cidStr := strings.Trim(fmt.Sprintf("%v", e.eval(args[1], env)), "\"")
		e.nodo.mu.Lock()
		if a, ok := e.nodo.agentes[agentID]; ok {
			a.RootCID = cidStr
			a.UltimaActual = time.Now().Unix()
		}
		e.nodo.mu.Unlock()
		e.nodo.PersistirLocamente()
		e.nodo.SincronizarConPares()
		// ---- PULSO: emitir evento ----
		go e.nodo.broadcastPulse("root_updated", map[string]interface{}{
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
		e.nodo.mu.Lock()
		e.nodo.nombres[alias] = agentID
		e.nodo.mu.Unlock()
		e.nodo.Auditoria("DNS_REGISTRO", fmt.Sprintf("Alias: %s -> Agente: %s", alias, agentID))
		e.nodo.PersistirLocamente()
		e.nodo.DifundirActualizacionDNS(alias, agentID)
		// ---- PULSO: emitir evento ----
		go e.nodo.broadcastPulse("dns_registered", map[string]interface{}{
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
		cidStr, _ := e.nodo.GenerarCID(data)
		e.nodo.AnunciarNuevoBloque(cidStr)
		return cidStr
	}))

	e.globalEnv.SetFunction("fetch-cid", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error"
		}
		data, _ := e.nodo.BuscarContenidoPorCID(fmt.Sprintf("%v", e.eval(args[0], env)))
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
		ctx, cancel := context.WithTimeout(e.nodo.ctx, 10*time.Second)
		defer cancel()
		if err := e.nodo.host.Connect(ctx, *peerInfo); err != nil {
			return fmt.Sprintf("error al conectar: %v", err)
		}
		return fmt.Sprintf("conectado a %s", peerInfo.ID.String())
	}))

	// =====================================================================
	// SISTEMA NEURONAL
	// =====================================================================

	e.globalEnv.SetFunction("neuron-state", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if e.nodo.neuralState == nil {
			return "No inicializado"
		}
		return fmt.Sprintf("Membrane:%.4f Threshold:%.4f LeakRate:%.4f Type:%s Synapses:%d LastSpike:%d",
			e.nodo.neuralState.MembranePotential,
			e.nodo.neuralState.SpikeThreshold,
			e.nodo.neuralState.LeakRate,
			e.nodo.neuralState.NeuronType,
			len(e.nodo.neuralState.Synapses),
			e.nodo.neuralState.LastSpikeTime)
	}))

	e.globalEnv.SetFunction("neuron-stats", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if e.nodo.neuralState == nil {
			return "No inicializado"
		}
		avgWeight := 0.0
		totalFires := int64(0)
		for _, s := range e.nodo.neuralState.Synapses {
			avgWeight += s.Weight
			totalFires += s.SuccessfulFires
		}
		if len(e.nodo.neuralState.Synapses) > 0 {
			avgWeight /= float64(len(e.nodo.neuralState.Synapses))
		}
		return fmt.Sprintf("Type:%s Membrane:%.4f Threshold:%.4f Leak:%.4f Synapses:%d AvgWeight:%.4f TotalSpikes:%d",
			e.nodo.neuralState.NeuronType,
			e.nodo.neuralState.MembranePotential,
			e.nodo.neuralState.SpikeThreshold,
			e.nodo.neuralState.LeakRate,
			len(e.nodo.neuralState.Synapses),
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
		if e.nodo.neuralState == nil {
			return input
		}
		potencial := input
		for _, syn := range e.nodo.neuralState.Synapses {
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

		e.nodo.mu.Lock()
		if e.nodo.neuralState == nil {
			e.nodo.neuralState = &NeuralState{
				Synapses: make(map[string]SynapticWeight),
			}
		}
		e.nodo.neuralState.Synapses[target] = SynapticWeight{
			TargetNeuronID: target,
			Weight:         initialWeight,
			LastUpdated:    time.Now().Unix(),
		}
		e.nodo.mu.Unlock()

		update := map[string]string{
			"tipo":          "synaptic_update",
			"neuronas_pre":  e.nodo.host.ID().String(),
			"neuronas_post": target,
			"exito":         "true",
			"peso":          fmt.Sprintf("%f", initialWeight),
		}
		if data, err := json.Marshal(update); err == nil && e.nodo.topic != nil {
			go e.nodo.topic.Publish(e.nodo.ctx, data)
		}

		return fmt.Sprintf("Conectado a %s con peso %.2f", target, initialWeight)
	}))

	e.globalEnv.SetFunction("list-synapses", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if e.nodo.neuralState == nil || len(e.nodo.neuralState.Synapses) == 0 {
			return "No hay sinapsis"
		}
		result := "Sinapsis:\n"
		for target, syn := range e.nodo.neuralState.Synapses {
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
		cidStr, err := e.nodo.GenerarCID([]byte(dataStr))
		if err != nil {
			return fmt.Sprintf("error al generar CID: %v", err)
		}
		e.nodo.Auditoria("MEMORIA_GUARDADA", fmt.Sprintf("CID: %s | Data: %s", cidStr, dataStr))
		go func() {
			if e.nodo.topic != nil {
				msg := map[string]string{
					"tipo":    "memory_distributed",
					"cid":     cidStr,
					"content": dataStr,
					"origin":  e.nodo.host.ID().String(),
					"ttl":     "3",
				}
				if data, err := json.Marshal(msg); err == nil {
					e.nodo.topic.Publish(e.nodo.ctx, data)
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
		e.nodo.mu.RLock()
		defer e.nodo.mu.RUnlock()
		for cid, data := range e.nodo.blockstore {
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
		e.nodo.mu.Lock()
		defer e.nodo.mu.Unlock()
		if e.nodo.neuralState == nil {
			return "error: neurona no inicializada"
		}
		if syn, ok := e.nodo.neuralState.Synapses[target]; ok {
			oldWeight := syn.Weight
			newWeight := oldWeight + tasa*(1-oldWeight)
			if newWeight > 1 {
				newWeight = 1
			}
			syn.Weight = newWeight
			syn.SuccessfulFires++
			syn.LastUpdated = time.Now().Unix()
			e.nodo.neuralState.Synapses[target] = syn
			go func() {
				update := map[string]string{
					"tipo":             "synaptic_update",
					"neuronas_pre":     e.nodo.host.ID().String(),
					"neuronas_post":    target,
					"exito":            "true",
					"tasa_aprendizaje": fmt.Sprintf("%f", tasa),
				}
				if data, err := json.Marshal(update); err == nil && e.nodo.topic != nil {
					e.nodo.topic.Publish(e.nodo.ctx, data)
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
		e.nodo.mu.Lock()
		defer e.nodo.mu.Unlock()
		if e.nodo.neuralState == nil {
			return "error: neurona no inicializada"
		}
		if syn, ok := e.nodo.neuralState.Synapses[target]; ok {
			oldWeight := syn.Weight
			syn.Weight = weight
			syn.LastUpdated = time.Now().Unix()
			e.nodo.neuralState.Synapses[target] = syn
			go e.nodo.persistirEstadoNeuronal()
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

	// =====================================================================
	// ZYRION Y PRIMITIVAS DE IA
	// =====================================================================

	e.registerUnifiedFunctions()

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

func (e *LispEvaluator) registerUnifiedFunctions() {
	e.globalEnv.SetFunction("generar-pagina-web", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return "error: requiere nombre-agente"
		}
		agenteID := fmt.Sprintf("%v", e.eval(args[0], env))
		html := generarHTMLBase(agenteID)
		cid, err := e.nodo.GenerarCID([]byte(html))
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		e.nodo.SetAgentRoot(agenteID, cid)
		return cid
	}))

	e.globalEnv.SetFunction("get-agent-balance", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return 0.0
		}
		agentID := fmt.Sprintf("%v", e.eval(args[0], env))
		e.nodo.mu.RLock()
		defer e.nodo.mu.RUnlock()
		if a, ok := e.nodo.agentes[agentID]; ok {
			return a.BalanceUTXO
		}
		return 0.0
	}))

	e.globalEnv.SetFunction("inventario-listar", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		resultado := make(LispList, 0)
		e.nodo.mu.RLock()
		defer e.nodo.mu.RUnlock()
		for cid := range e.nodo.blockstore {
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
		relacion := &RelacionEntidad{
			ID:           relacionId,
			EntidadA:     origen,
			EntidadB:     destino,
			Tipo:         tipo,
			Cardinalidad: cardinalidad,
		}
		relacionesGlobales[relacionId] = relacion
		e.nodo.mu.Lock()
		if e.nodo.neuralState == nil {
			e.nodo.neuralState = &NeuralState{
				Synapses: make(map[string]SynapticWeight),
			}
		}
		e.nodo.neuralState.Synapses[destino] = SynapticWeight{
			TargetNeuronID: destino,
			Weight:         0.5,
			LastUpdated:    time.Now().Unix(),
		}
		e.nodo.mu.Unlock()
		e.nodo.Auditoria("RELACION_CREADA", fmt.Sprintf("%s: %s -> %s (%s)", tipo, origen, destino, cardinalidad))
		return relacionId
	}))

	e.globalEnv.SetFunction("listar-relaciones", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		resultado := make(LispList, 0)
		for _, rel := range relacionesGlobales {
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
		entidad := &EntidadProgramatica{
			ID:        id,
			Tipo:      tipo,
			Atributos: atributos,
			HeredaDe:  "",
			ModuloID:  "editor",
		}
		muEntidades.Lock()
		entidadesGlobales[id] = entidad
		muEntidades.Unlock()
		e.nodo.Auditoria("ENTIDAD_CREADA", fmt.Sprintf("Tipo: %s | ID: %s", tipo, id))
		return id
	}))

	e.globalEnv.SetFunction("listar-entidades", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		resultado := make(LispList, 0)
		muEntidades.RLock()
		defer muEntidades.RUnlock()
		for _, ent := range entidadesGlobales {
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
		e.nodo.mu.Lock()
		if _, existe := e.nodo.agentes[agentID]; !existe {
			e.nodo.agentes[agentID] = &Agente{
				ID:           agentID,
				RootCID:      "",
				UltimaActual: time.Now().Unix(),
				BalanceUTXO:  1000.0,
			}
		}
		e.nodo.mu.Unlock()
		html := generarHTMLParaPlantilla(nombre, tipo)
		cid, err := e.nodo.GenerarCID([]byte(html))
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		e.nodo.SetAgentRoot(agentID, cid)
		alias := strings.ReplaceAll(strings.ToLower(nombre), " ", "-") + ".negocio.ans"
		e.nodo.nombres[alias] = agentID
		e.nodo.Auditoria("APP_CREADA_DESDE_PLANTILLA", fmt.Sprintf("Nombre: %s | Tipo: %s | URL: /w/%s", nombre, tipo, alias))
		return fmt.Sprintf("✅ App creada: /w/%s", alias)
	}))
}

// =============================================================================
// FUNCIONES AUXILIARES DE EVALUACIÓN LISP
// =============================================================================

func (e *LispEvaluator) eval(expr LispValue, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) evalList(list LispList, env *LispEnvironment) LispValue {
	result := make(LispList, len(list))
	for i, item := range list {
		result[i] = e.eval(item, env)
	}
	return result
}

func (e *LispEvaluator) evalSpecialLet(list LispList, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) evalSpecialLambda(list LispList, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) evalSpecialDefmacro(list LispList, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) evalSpecialDefun(list LispList, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) apply(fn LispValue, args []LispValue, env *LispEnvironment) LispValue {
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

func (e *LispEvaluator) evalSpecialLetStar(list LispList, env *LispEnvironment) LispValue {
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
// NODO ALSET – ESTRUCTURA PRINCIPAL
// =============================================================================

type NodoAlset struct {
	host                 host.Host
	ctx                  context.Context
	agentes              map[string]*Agente
	mu                   sync.RWMutex
	lisp                 *LispEvaluator
	kademlia             *dht.IpfsDHT
	pubsub               *pubsub.PubSub
	topic                *pubsub.Topic
	datastore            datastore.Batching
	blockstore           map[string][]byte
	nombres              map[string]string
	masterPrivKey        crypto.PrivKey
	neuralState          *NeuralState
	pendingInferences    map[string]chan InferenceResponse
	pendingMemoryQueries map[string]chan MemoryResponse
	inferenceMu          sync.RWMutex
	memoryMu             sync.RWMutex
	hebbianMemory        map[string]float64
	startTime            int64
	syncManager          *SyncManager

	// ---- NUEVO SISTEMA DE PULSOS ----
	pulseSubscribers   map[*SSESubscriber]bool
	pulseSubscribersMu sync.RWMutex
	pulseClients       map[string]*PulseClient
	pulseClientsMu     sync.RWMutex
	pulseKnownServers  []string
}

type BlockInfo struct {
    CID     string `json:"cid"`
    Size    int    `json:"size"`
    Preview string `json:"preview"`
}

type SSESubscriber struct {
	ch     chan string
	ctx    context.Context
	cancel context.CancelFunc
}

type PulseClient struct {
	url       string
	ctx       context.Context
	cancel    context.CancelFunc
	connected bool
	lastEvent time.Time
	reconnect chan bool
}

// =============================================================================
// MÉTODOS DEL NODO – EXISTENTES
// =============================================================================

func (n *NodoAlset) Auditoria(accion string, detalle string) {
	type AuditLine struct {
		Timestamp string `json:"ts"`
		Action    string `json:"action"`
		Detail    string `json:"detail"`
		NodeID    string `json:"node_id"`
	}
	line := AuditLine{
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    accion,
		Detail:    detalle,
		NodeID:    n.host.ID().String(),
	}
	data, _ := json.Marshal(line)
	f, _ := os.OpenFile("audit.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.Write(data)
	f.WriteString("\n")
	f.Sync()
}

func (n *NodoAlset) LoadMasterKey() {
	keyFile := "master_identity.key"
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		priv, _, _ := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
		raw, _ := crypto.MarshalPrivateKey(priv)
		os.WriteFile(keyFile, raw, 0600)
		n.masterPrivKey = priv
		fmt.Println("🔑 Nueva Clave Maestra generada y guardada.")
	} else {
		raw, _ := os.ReadFile(keyFile)
		priv, _ := crypto.UnmarshalPrivateKey(raw)
		n.masterPrivKey = priv
		fmt.Println("🔑 Clave Maestra institucional cargada correctamente.")
	}
}

func (n *NodoAlset) GenerarCID(data []byte) (string, error) {
	pref := cid.Prefix{Version: 1, Codec: cid.Raw, MhType: multihash.SHA2_256, MhLength: -1}
	c, _ := pref.Sum(data)
	cidStr := c.String()
	n.mu.Lock()
	n.blockstore[cidStr] = data
	n.mu.Unlock()
	_ = os.MkdirAll(BlocksDir, 0755)
	_ = os.WriteFile(filepath.Join(BlocksDir, cidStr), data, 0644)
	return cidStr, nil
}

func (n *NodoAlset) BuscarContenidoPorCID(cidStr string) ([]byte, error) {
	n.mu.RLock()
	data, existe := n.blockstore[cidStr]
	n.mu.RUnlock()
	if existe {
		return data, nil
	}
	if diskData, err := os.ReadFile(filepath.Join(BlocksDir, cidStr)); err == nil {
		n.mu.Lock()
		n.blockstore[cidStr] = diskData
		n.mu.Unlock()
		return diskData, nil
	}
	c, _ := cid.Decode(cidStr)
	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	defer cancel()
	providers := n.kademlia.FindProvidersAsync(ctx, c, 5)
	for p := range providers {
		if p.ID == n.host.ID() {
			continue
		}
		s, err := n.host.NewStream(n.ctx, p.ID, AlsetDataExchangeID)
		if err != nil {
			continue
		}
		s.Write([]byte(cidStr + "\n"))
		res, _ := io.ReadAll(s)
		s.Close()
		if len(res) > 0 {
			n.GenerarCID(res)
			return res, nil
		}
	}
	return nil, fmt.Errorf("no encontrado")
}

func (n *NodoAlset) PersistirLocamente() {
	n.mu.RLock()
	defer n.mu.RUnlock()
	dAg, _ := json.MarshalIndent(n.agentes, "", "  ")
	_ = os.WriteFile("alset_state.json", dAg, 0644)
	dAn, _ := json.MarshalIndent(n.nombres, "", "  ")
	_ = os.WriteFile("alset_names.json", dAn, 0644)
	n.persistirEstadoNeuronal()
}

func (n *NodoAlset) CargarEstado() {
	if d, err := os.ReadFile("alset_state.json"); err == nil {
		n.mu.Lock()
		_ = json.Unmarshal(d, &n.agentes)
		n.mu.Unlock()
	}
	if d, err := os.ReadFile("alset_names.json"); err == nil {
		n.mu.Lock()
		_ = json.Unmarshal(d, &n.nombres)
		n.mu.Unlock()
	}
	files, _ := os.ReadDir(BlocksDir)
	n.mu.Lock()
	for _, f := range files {
		if !f.IsDir() {
			if d, err := os.ReadFile(filepath.Join(BlocksDir, f.Name())); err == nil {
				n.blockstore[f.Name()] = d
			}
		}
	}
	n.mu.Unlock()
	fmt.Printf("📂 Alset Engine: %d agentes, %d nombres y %d bloques en RAM.\n", len(n.agentes), len(n.nombres), len(n.blockstore))
}

func (n *NodoAlset) IpfsAddDirectory(dirPath string) (string, error) {
	files := make(map[string][]byte)
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(dirPath, path)
		files[relPath] = data
		return nil
	})
	if err != nil {
		return "", err
	}
	jsonData, _ := json.Marshal(files)
	cid, err := n.GenerarCID(jsonData)
	if err != nil {
		return "", err
	}
	fmt.Printf("📁 Directorio subido a IPFS: %s → %s\n", dirPath, cid)
	return cid, nil
}

func (n *NodoAlset) RegisterApp(appName string) (string, error) {
	appPath := filepath.Join(StaticDir, "apps", appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return "", fmt.Errorf("app no encontrada: %s", appName)
	}
	cid, err := n.IpfsAddDirectory(appPath)
	if err != nil {
		return "", err
	}
	createCmd := fmt.Sprintf(`(crear-agente "%s")`, appName)
	_, err = n.lisp.Eval(createCmd)
	if err != nil {
		return "", err
	}
	var agentID string
	n.mu.RLock()
	for id, agent := range n.agentes {
		if agent.ID == appName {
			agentID = id
			break
		}
	}
	n.mu.RUnlock()
	if agentID == "" {
		return "", fmt.Errorf("no se pudo crear el agente para: %s", appName)
	}
	setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, agentID, cid)
	_, err = n.lisp.Eval(setRootCmd)
	if err != nil {
		return "", err
	}
	registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, appName, agentID)
	_, err = n.lisp.Eval(registerCmd)
	if err != nil {
		return "", err
	}
	fmt.Printf("✅ App registrada: %s → %s (CID: %s)\n", appName, agentID, cid)
	return agentID, nil
}

// =============================================================================
// MÉTODOS DE IA DISTRIBUIDA (EXISTENTES)
// =============================================================================

func (n *NodoAlset) persistirEstadoNeuronal() {
	if n.neuralState == nil {
		return
	}
	n.mu.RLock()
	data, _ := json.MarshalIndent(n.neuralState, "", "  ")
	n.mu.RUnlock()
	_ = os.WriteFile("neural_state.json", data, 0644)
}

func (n *NodoAlset) cargarPesosSinapsis() {
	if data, err := os.ReadFile("neural_state.json"); err == nil {
		var state NeuralState
		if err := json.Unmarshal(data, &state); err == nil {
			n.neuralState = &state
			if n.neuralState.Synapses == nil {
				n.neuralState.Synapses = make(map[string]SynapticWeight)
			}
			fmt.Println("🧠 Estado neuronal cargado desde disco")
		}
	}
	if n.neuralState == nil {
		return
	}
	for target, syn := range n.neuralState.Synapses {
		n.hebbianMemory[target] = syn.Weight
	}
}

func (n *NodoAlset) puedeProcesarInferencia(input []float64) bool {
	return n.neuralState != nil && n.neuralState.NeuronType == "input"
}

func (n *NodoAlset) seleccionarMejorVecinoParaInferencia(input []float64) string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.neuralState == nil {
		return ""
	}
	var mejorNodo string
	var mayorPeso float64
	for targetID, sinapsis := range n.neuralState.Synapses {
		if sinapsis.Weight > mayorPeso {
			mayorPeso = sinapsis.Weight
			mejorNodo = targetID
		}
	}
	return mejorNodo
}

func (n *NodoAlset) reenviarSolicitudInferencia(req InferenceRequest, destino string) {
	data, _ := json.Marshal(req)
	msg := map[string]string{
		"tipo": "inference_request",
		"data": string(data),
	}
	msgData, _ := json.Marshal(msg)
	n.topic.Publish(n.ctx, msgData)
}

func (n *NodoAlset) buscarEnMemoriaLocal(consulta string) []string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	resultados := []string{}
	for cid, data := range n.blockstore {
		if strings.Contains(string(data), consulta) || strings.Contains(cid, consulta) {
			if len(resultados) < 10 {
				resultados = append(resultados, cid)
			}
		}
	}
	return resultados
}

func (n *NodoAlset) buscarEnMemoriaLocalConContenido(consulta string) []MemoryResponse {
	n.mu.RLock()
	defer n.mu.RUnlock()
	resultados := []MemoryResponse{}
	for cid, data := range n.blockstore {
		if strings.Contains(string(data), consulta) {
			resultados = append(resultados, MemoryResponse{
				Results:       []string{cid},
				Contents:      []string{string(data)},
				ResponderNode: n.host.ID().String(),
			})
			if len(resultados) >= 5 {
				break
			}
		}
	}
	return resultados
}

func (n *NodoAlset) propagarMemoriaDistribuida(data string, cid string) {
	query := MemoryQuery{
		QueryID:    generarUUID(),
		Content:    data,
		OriginNode: n.host.ID().String(),
		TTL:        3,
	}
	msg := map[string]string{
		"tipo":     "memory_distributed",
		"query_id": query.QueryID,
		"content":  data,
		"cid":      cid,
		"origin":   query.OriginNode,
		"ttl":      "3",
	}
	msgData, _ := json.Marshal(msg)
	if n.topic != nil {
		n.topic.Publish(n.ctx, msgData)
	}
}

func (n *NodoAlset) manejarMemoriaDistribuida(update map[string]string, origen peer.ID) {
	ttl, _ := strconv.Atoi(update["ttl"])
	if ttl <= 0 {
		return
	}
	cid := update["cid"]
	content := update["content"]
	n.mu.RLock()
	_, existe := n.blockstore[cid]
	n.mu.RUnlock()
	if !existe {
		n.GenerarCID([]byte(content))
		fmt.Printf("📚 Memoria distribuida recibida y almacenada: %s\n", cid)
	}
	if ttl > 1 {
		update["ttl"] = strconv.Itoa(ttl - 1)
		msgData, _ := json.Marshal(update)
		peers := n.host.Network().Peers()
		for _, p := range peers {
			if p != origen && n.topic != nil {
				go n.topic.Publish(n.ctx, msgData)
			}
		}
	}
}

func (n *NodoAlset) buscarMemoriaDistribuida(query string, maxHops int) string {
	queryID := generarUUID()
	responseChan := make(chan MemoryResponse, 10)
	n.memoryMu.Lock()
	n.pendingMemoryQueries[queryID] = responseChan
	n.memoryMu.Unlock()
	defer func() {
		time.Sleep(5 * time.Second)
		n.memoryMu.Lock()
		delete(n.pendingMemoryQueries, queryID)
		n.memoryMu.Unlock()
	}()
	msg := map[string]string{
		"tipo":     "memory_query",
		"query_id": queryID,
		"query":    query,
		"origin":   n.host.ID().String(),
		"ttl":      strconv.Itoa(maxHops),
	}
	msgData, _ := json.Marshal(msg)
	if n.topic != nil {
		n.topic.Publish(n.ctx, msgData)
	}
	select {
	case resp := <-responseChan:
		if len(resp.Contents) > 0 {
			return resp.Contents[0]
		}
		return ""
	case <-time.After(3 * time.Second):
		return ""
	}
}

func (n *NodoAlset) manejarConsultaMemoria(update map[string]string, origen peer.ID) {
	query := update["query"]
	queryID := update["query_id"]
	ttl, _ := strconv.Atoi(update["ttl"])
	resultados := n.buscarEnMemoriaLocalConContenido(query)
	if len(resultados) > 0 {
		resp := MemoryResponse{
			QueryID:       queryID,
			Results:       resultados[0].Results,
			Contents:      resultados[0].Contents,
			ResponderNode: n.host.ID().String(),
		}
		respData, _ := json.Marshal(resp)
		respMsg := map[string]string{
			"tipo": "memory_response",
			"data": string(respData),
		}
		msgData, _ := json.Marshal(respMsg)
		if n.topic != nil {
			n.topic.Publish(n.ctx, msgData)
		}
	} else if ttl > 1 {
		update["ttl"] = strconv.Itoa(ttl - 1)
		msgData, _ := json.Marshal(update)
		peers := n.host.Network().Peers()
		for _, p := range peers {
			if p != origen && n.topic != nil {
				go n.topic.Publish(n.ctx, msgData)
			}
		}
	}
}

func (n *NodoAlset) procesarRespuestaMemoria(update map[string]string) {
	var resp MemoryResponse
	if err := json.Unmarshal([]byte(update["data"]), &resp); err != nil {
		return
	}
	n.memoryMu.RLock()
	ch, exists := n.pendingMemoryQueries[resp.QueryID]
	n.memoryMu.RUnlock()
	if exists {
		select {
		case ch <- resp:
		default:
		}
	}
}

func (n *NodoAlset) propagarSpikeASinapsis(intensidad float64, timestamp int64) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.neuralState == nil {
		return
	}
	for targetID, sinapsis := range n.neuralState.Synapses {
		senalSalida := intensidad * sinapsis.Weight
		spikeMsg := map[string]string{
			"tipo":       "neural_spike",
			"intensidad": fmt.Sprintf("%f", senalSalida),
			"timestamp":  fmt.Sprintf("%d", timestamp),
			"origen":     n.host.ID().String(),
			"target":     targetID,
		}
		data, _ := json.Marshal(spikeMsg)
		if n.topic != nil {
			go n.topic.Publish(n.ctx, data)
		}
	}
}

func (n *NodoAlset) procesarSpikeNeuronal(update map[string]string, origen peer.ID) {
	intensidad, _ := strconv.ParseFloat(update["intensidad"], 64)
	timestamp, _ := strconv.ParseInt(update["timestamp"], 10, 64)
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.neuralState == nil {
		return
	}
	ahora := time.Now().UnixNano()
	if n.neuralState.LastSpikeTime > 0 {
		tiempoTranscurrido := float64(ahora - n.neuralState.LastSpikeTime)
		decaimiento := math.Exp(-tiempoTranscurrido * n.neuralState.LeakRate)
		n.neuralState.MembranePotential *= decaimiento
	}
	n.neuralState.MembranePotential += intensidad
	if n.neuralState.MembranePotential >= n.neuralState.SpikeThreshold {
		n.neuralState.LastSpikeTime = ahora
		n.neuralState.MembranePotential = 0
		go n.propagarSpikeASinapsis(intensidad, timestamp)
	}
}

func (n *NodoAlset) manejarInferenciaDistribuida(update map[string]string, origen peer.ID) {
	var req InferenceRequest
	if err := json.Unmarshal([]byte(update["data"]), &req); err != nil {
		return
	}
	if req.TTL <= 0 {
		respuesta := InferenceResponse{
			RequestID:      req.RequestID,
			OutputData:     []float64{-1},
			ProcessingNode: n.host.ID().String(),
			ProcessingTime: time.Now().UnixNano(),
		}
		n.publicarRespuestaInferencia(respuesta)
		return
	}
	req.TTL--
	puedeProcesar := n.puedeProcesarInferencia(req.InputData)
	if puedeProcesar {
		go n.procesarInferenciaLocal(req)
	} else {
		nodoDestino := n.seleccionarMejorVecinoParaInferencia(req.InputData)
		if nodoDestino != "" {
			n.reenviarSolicitudInferencia(req, nodoDestino)
		} else {
			go n.procesarInferenciaLocal(req)
		}
	}
}

func (n *NodoAlset) procesarInferenciaLocal(req InferenceRequest) {
	var output float64 = 0
	for _, val := range req.InputData {
		output += val
	}
	if len(req.InputData) > 0 {
		output = output / float64(len(req.InputData))
	}
	output = 1.0 / (1.0 + math.Exp(-output))
	respuesta := InferenceResponse{
		RequestID:      req.RequestID,
		OutputData:     []float64{output},
		ProcessingNode: n.host.ID().String(),
		ProcessingTime: time.Now().UnixNano(),
	}
	n.publicarRespuestaInferencia(respuesta)
}

func (n *NodoAlset) publicarRespuestaInferencia(respuesta InferenceResponse) {
	data, _ := json.Marshal(respuesta)
	msg := map[string]string{
		"tipo": "inference_response",
		"data": string(data),
	}
	msgData, _ := json.Marshal(msg)
	n.topic.Publish(n.ctx, msgData)
}

func (n *NodoAlset) procesarRespuestaInferencia(update map[string]string) {
	var respuesta InferenceResponse
	if err := json.Unmarshal([]byte(update["data"]), &respuesta); err != nil {
		return
	}
	n.inferenceMu.RLock()
	ch, exists := n.pendingInferences[respuesta.RequestID]
	n.inferenceMu.RUnlock()
	if exists {
		select {
		case ch <- respuesta:
		default:
		}
		go func() {
			time.Sleep(5 * time.Second)
			n.inferenceMu.Lock()
			delete(n.pendingInferences, respuesta.RequestID)
			n.inferenceMu.Unlock()
		}()
	}
}

func (n *NodoAlset) actualizarPesosSinapsis(update map[string]string, origen peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.neuralState == nil {
		return
	}
	neuronasPre := strings.Split(update["neuronas_pre"], ",")
	neuronasPost := strings.Split(update["neuronas_post"], ",")
	exito := update["exito"] == "true"
	tasaAprendizaje := 0.01
	if pesoStr, ok := update["peso"]; ok {
		if peso, err := strconv.ParseFloat(pesoStr, 64); err == nil && peso > 0 {
			tasaAprendizaje = peso * 0.01
		}
	}
	for _, pre := range neuronasPre {
		for _, post := range neuronasPost {
			key := pre + "->" + post
			if sinapsis, exists := n.neuralState.Synapses[key]; exists {
				if exito {
					sinapsis.Weight += tasaAprendizaje * (1 - sinapsis.Weight)
					sinapsis.SuccessfulFires++
				} else {
					sinapsis.Weight *= (1 - tasaAprendizaje)
				}
				if sinapsis.Weight > 1 {
					sinapsis.Weight = 1
				}
				if sinapsis.Weight < 0 {
					sinapsis.Weight = 0
				}
				sinapsis.LastUpdated = time.Now().Unix()
				n.neuralState.Synapses[key] = sinapsis
				n.hebbianMemory[key] = sinapsis.Weight
			}
		}
	}
	go n.persistirEstadoNeuronal()
}

func (n *NodoAlset) sincronizarEstadoNeuronal(update map[string]string, origen peer.ID) {
	n.mu.RLock()
	if n.neuralState == nil {
		n.mu.RUnlock()
		return
	}
	n.mu.RUnlock()
	stateJSON, _ := json.Marshal(n.neuralState)
	respuesta := map[string]string{
		"tipo":        "neural_state_sync_response",
		"estado":      string(stateJSON),
		"nodo_origen": n.host.ID().String(),
	}
	data, _ := json.Marshal(respuesta)
	n.topic.Publish(n.ctx, data)
}

// =============================================================================
// NETWORKING & GOSSIP SYNC (EXISTENTES)
// =============================================================================

func (n *NodoAlset) AnunciarNuevoBloque(cidStr string) {
    // 1. Publicar en gossip (opcional, lo dejamos por compatibilidad)
    update := map[string]string{"tipo": "new_block", "cid": cidStr}
    data, _ := json.Marshal(update)
    if n.topic != nil {
        n.topic.Publish(n.ctx, data)
    }

    // 2. Emitir por pulsos (HTTP)
    n.mu.RLock()
    blockData, exists := n.blockstore[cidStr]
    n.mu.RUnlock()

    if exists {
        // Codificar el bloque en base64 para transmitirlo
        b64 := base64.StdEncoding.EncodeToString(blockData)
        n.broadcastPulse("new_block", map[string]interface{}{
            "cid":  cidStr,
            "data": b64,
        })
        log.Printf("📤 Bloque %s emitido por pulso (%d bytes)", cidStr, len(blockData))
    } else {
        // Si no tenemos el bloque localmente (caso raro), solo anunciamos el CID
        n.broadcastPulse("new_block", map[string]interface{}{
            "cid": cidStr,
        })
        log.Printf("📤 Anuncio de bloque %s (sin datos) emitido por pulso", cidStr)
    }
}

func (n *NodoAlset) SincronizarConPares() {
	n.mu.RLock()
	data, _ := json.Marshal(n.agentes)
	n.mu.RUnlock()
	cidStr, _ := n.GenerarCID(data)
	update := map[string]string{
		"tipo": "new_block",
		"cid":  cidStr,
	}
	msgBytes, _ := json.Marshal(update)
	if n.topic != nil {
		n.topic.Publish(n.ctx, msgBytes)
	}
}

func (n *NodoAlset) DifundirActualizacionDNS(alias string, agentID string) {
	update := map[string]string{"tipo": "dns_update", "alias": alias, "agent_id": agentID}
	data, _ := json.Marshal(update)
	if n.topic != nil {
		n.topic.Publish(n.ctx, data)
	}
}

func (n *NodoAlset) SolicitarBloqueAPar(cidStr string, p peer.ID) {
	s, err := n.host.NewStream(n.ctx, p, AlsetDataExchangeID)
	if err != nil {
		return
	}
	defer s.Close()
	s.Write([]byte(cidStr + "\n"))
	data, _ := io.ReadAll(s)
	if len(data) > 0 {
		n.GenerarCID(data)
		var remAg map[string]*Agente
		if err := json.Unmarshal(data, &remAg); err == nil && len(remAg) > 0 {
			n.mu.Lock()
			for k, v := range remAg {
				n.agentes[k] = v
			}
			n.mu.Unlock()
			n.PersistirLocamente()
		}
	}
}

func (n *NodoAlset) handleDataExchange(s network.Stream) {
	defer s.Close()
	scanner := bufio.NewScanner(s)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "SYNC_FULL_REQUEST") {
			n.handleFullSyncRequest(s)
			return
		}
		if strings.HasPrefix(line, "SYNC_QUICK_REQUEST") {
			n.handleQuickSyncRequest(s)
			return
		}
		cidReq := line
		n.mu.RLock()
		data, ok := n.blockstore[cidReq]
		n.mu.RUnlock()
		if ok {
			s.Write(data)
		}
	}
}

func (n *NodoAlset) handleFullSyncRequest(s network.Stream) {
	fmt.Println("📡 Recibida solicitud de sincronización completa")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	state := struct {
		Agentes map[string]*Agente `json:"agentes"`
		Nombres map[string]string  `json:"nombres"`
	}{
		Agentes: n.agentes,
		Nombres: n.nombres,
	}
	stateJSON, _ := json.Marshal(state)
	gz.Write(stateJSON)
	gz.Close()
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, uint64(buf.Len()))
	s.Write(sizeBuf)
	s.Write(buf.Bytes())
	fmt.Printf("✅ Estado completo enviado: %d bytes comprimidos\n", buf.Len())
}

func (n *NodoAlset) handleQuickSyncRequest(s network.Stream) {
	fmt.Println("⚡ Recibida solicitud de sincronización rápida")
	response := struct {
		Agentes      map[string]*Agente `json:"agentes"`
		Nombres      map[string]string  `json:"nombres"`
		RecentBlocks map[string][]byte  `json:"recent_blocks"`
		NeuralState  *NeuralState       `json:"neural_state"`
	}{
		Agentes:      n.agentes,
		Nombres:      n.nombres,
		NeuralState:  n.neuralState,
		RecentBlocks: make(map[string][]byte),
	}
	count := 0
	for cid, data := range n.blockstore {
		if count >= 100 {
			break
		}
		response.RecentBlocks[cid] = data
		count++
	}
	data, _ := json.Marshal(response)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(data)
	gz.Close()
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, uint64(buf.Len()))
	s.Write(sizeBuf)
	s.Write(buf.Bytes())
}

func (n *NodoAlset) EscucharGossip() {
	sub, _ := n.topic.Subscribe()
	for {
		msg, err := sub.Next(n.ctx)
		if err != nil {
			return
		}
		if msg.ReceivedFrom == n.host.ID() {
			continue
		}
		var update map[string]string
		if err := json.Unmarshal(msg.Data, &update); err == nil {
			switch update["tipo"] {
			case "dns_update":
				n.mu.Lock()
				n.nombres[update["alias"]] = update["agent_id"]
				n.mu.Unlock()
				n.PersistirLocamente()
			case "new_block":
				n.mu.RLock()
				_, existe := n.blockstore[update["cid"]]
				n.mu.RUnlock()
				if !existe {
					go n.SolicitarBloqueAPar(update["cid"], msg.ReceivedFrom)
				}
			case "admin_panel_announce":
				go n.handleAdminPanelAnnounce(update)
			case "neural_spike":
				go n.procesarSpikeNeuronal(update, msg.ReceivedFrom)
			case "inference_request":
				go n.manejarInferenciaDistribuida(update, msg.ReceivedFrom)
			case "inference_response":
				go n.procesarRespuestaInferencia(update)
			case "synaptic_update":
				go n.actualizarPesosSinapsis(update, msg.ReceivedFrom)
			case "memory_query":
				go n.manejarConsultaMemoria(update, msg.ReceivedFrom)
			case "memory_response":
				go n.procesarRespuestaMemoria(update)
			case "memory_distributed":
				go n.manejarMemoriaDistribuida(update, msg.ReceivedFrom)
			case "neural_state_sync":
				go n.sincronizarEstadoNeuronal(update, msg.ReceivedFrom)
			}
		}
	}
}

// =============================================================================
// ADMIN PANEL DISTRIBUTION (EXISTENTE)
// =============================================================================

func (n *NodoAlset) getAdminPanelHTML() string {
	return `<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Alset Network - Panel de Administración</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%);
            color: #fff;
            min-height: 100vh;
        }
        .header {
            background: rgba(0,0,0,0.8);
            backdrop-filter: blur(10px);
            padding: 1rem 2rem;
            border-bottom: 2px solid #f4b400;
            position: sticky;
            top: 0;
            z-index: 100;
        }
        .header h1 {
            font-size: 1.5rem;
            display: inline-block;
        }
        .header .node-id {
            float: right;
            font-family: monospace;
            color: #f4b400;
            margin-top: 0.3rem;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }
        .card {
            background: rgba(20,20,40,0.9);
            backdrop-filter: blur(5px);
            border-radius: 12px;
            padding: 1.5rem;
            border: 1px solid rgba(244,180,0,0.2);
            transition: transform 0.2s, border-color 0.2s;
        }
        .card:hover {
            transform: translateY(-2px);
            border-color: rgba(244,180,0,0.5);
        }
        .card h3 {
            color: #f4b400;
            margin-bottom: 1rem;
            font-size: 0.9rem;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .card .value {
            font-size: 2.5rem;
            font-weight: bold;
            margin-bottom: 0.5rem;
        }
        .card .label {
            color: #888;
            font-size: 0.85rem;
        }
        .section {
            background: rgba(20,20,40,0.9);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
            border: 1px solid rgba(244,180,0,0.2);
        }
        .section h2 {
            color: #f4b400;
            margin-bottom: 1rem;
            font-size: 1.2rem;
        }
        .sync-btn {
            background: #f4b400;
            color: #000;
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            cursor: pointer;
            font-weight: bold;
            margin-right: 0.5rem;
            transition: opacity 0.2s;
        }
        .sync-btn:hover { opacity: 0.8; }
        .sync-progress {
            width: 100%;
            height: 20px;
            background: rgba(255,255,255,0.1);
            border-radius: 10px;
            overflow: hidden;
            margin-top: 1rem;
        }
        .sync-progress-bar {
            height: 100%;
            background: #f4b400;
            width: 0%;
            transition: width 0.3s;
        }
        .log-container {
            background: #0a0a0a;
            border-radius: 8px;
            padding: 1rem;
            max-height: 300px;
            overflow-y: auto;
            font-family: monospace;
            font-size: 0.8rem;
        }
        .log-entry {
            padding: 0.25rem 0;
            border-bottom: 1px solid rgba(255,255,255,0.05);
        }
        button {
            background: rgba(244,180,0,0.2);
            border: 1px solid #f4b400;
            color: #f4b400;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.2s;
        }
        button:hover {
            background: #f4b400;
            color: #000;
        }
        .agent-list {
            max-height: 400px;
            overflow-y: auto;
        }
        .agent-item {
            padding: 0.5rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            font-family: monospace;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        .syncing { animation: pulse 1s infinite; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🌐 Alset Network</h1>
        <div class="node-id" id="nodeId">Cargando...</div>
    </div>
    <div class="container">
        <div class="stats-grid">
            <div class="card">
                <h3>Agentes</h3>
                <div class="value" id="agentesCount">-</div>
                <div class="label">Agentes registrados en la red</div>
            </div>
            <div class="card">
                <h3>Bloques IPFS</h3>
                <div class="value" id="bloquesCount">-</div>
                <div class="label">Bloques almacenados localmente</div>
            </div>
            <div class="card">
                <h3>Peers Conectados</h3>
                <div class="value" id="peersCount">-</div>
                <div class="label">Nodos en la red</div>
            </div>
            <div class="card">
                <h3>Estado Sincronización</h3>
                <div class="value" id="syncStatus">-</div>
                <div class="label">Última sincronización</div>
            </div>
        </div>
        
        <div class="section">
            <h2>🔄 Sincronización</h2>
            <button class="sync-btn" onclick="startFullSync()">Sincronización Completa</button>
            <button class="sync-btn" onclick="startQuickSync()">Sincronización Rápida</button>
            <button onclick="refreshStatus()">Actualizar Estado</button>
            <div id="syncProgressContainer" style="display:none;">
                <div class="sync-progress">
                    <div class="sync-progress-bar" id="syncProgressBar"></div>
                </div>
                <p id="syncStatusText" style="margin-top: 0.5rem;"></p>
            </div>
        </div>
        
        <div class="section">
            <h2>📋 Agentes Registrados</h2>
            <div class="agent-list" id="agentList">
                <div>Cargando...</div>
            </div>
        </div>
        
        <div class="section">
            <h2>📊 Últimos Eventos</h2>
            <div class="log-container" id="logContainer">
                <div>Cargando...</div>
            </div>
        </div>
    </div>
    
    <script>
        let refreshInterval;
        
        async function fetchAPI(endpoint) {
            try {
                const response = await fetch(endpoint);
                return await response.json();
            } catch (error) {
                console.error('Error:', error);
                return null;
            }
        }
        
        async function refreshStatus() {
            const agentes = await fetchAPI('/api/agentes/');
            const ipfsList = await fetchAPI('/api/ipfs/list');
            const peers = await fetchAPI('/api/network/peers');
            const syncStatus = await fetchAPI('/api/sync/status');
            
            if (agentes) document.getElementById('agentesCount').innerText = Object.keys(agentes).length;
            if (ipfsList) document.getElementById('bloquesCount').innerText = ipfsList.length;
            if (peers) document.getElementById('peersCount').innerText = peers.length;
            
            if (syncStatus) {
                const lastSync = syncStatus.last_sync ? new Date(syncStatus.last_sync * 1000).toLocaleString() : 'Nunca';
                if (syncStatus.is_syncing) {
                    document.getElementById('syncStatus').innerHTML = '<span class="syncing">🔄 Sincronizando...</span>';
                } else {
                    document.getElementById('syncStatus').innerHTML = lastSync;
                }
            }
            
            if (agentes) {
                const agentListDiv = document.getElementById('agentList');
                if (Object.keys(agentes).length === 0) {
                    agentListDiv.innerHTML = '<div>No hay agentes registrados</div>';
                } else {
                    let html = '';
                    for (const [id, agent] of Object.entries(agentes)) {
                        html += '<div class="agent-item">' + id + ' - Root: ' + (agent.root_cid || 'Ninguno') + ' - Balance: ' + agent.balance_utxo + '</div>';
                    }
                    agentListDiv.innerHTML = html;
                }
            }
        }
        
        async function startFullSync() {
            document.getElementById('syncProgressContainer').style.display = 'block';
            document.getElementById('syncStatusText').innerText = 'Iniciando sincronización completa...';
            
            const response = await fetch('/api/sync/full', { method: 'POST' });
            const result = await response.json();
            
            document.getElementById('syncStatusText').innerText = result.message;
            
            const interval = setInterval(async () => {
                const status = await fetchAPI('/api/sync/status');
                if (status && status.progress) {
                    document.getElementById('syncProgressBar').style.width = (status.progress.percent * 100) + '%';
                    document.getElementById('syncStatusText').innerText = status.progress.status;
                }
                if (status && !status.is_syncing) {
                    clearInterval(interval);
                    setTimeout(() => {
                        document.getElementById('syncProgressContainer').style.display = 'none';
                    }, 2000);
                    refreshStatus();
                }
            }, 1000);
        }
        
        async function startQuickSync() {
            document.getElementById('syncProgressContainer').style.display = 'block';
            document.getElementById('syncStatusText').innerText = 'Iniciando sincronización rápida...';
            
            const response = await fetch('/api/sync/quick', { method: 'POST' });
            const result = await response.json();
            
            document.getElementById('syncStatusText').innerText = result.message;
            setTimeout(() => {
                document.getElementById('syncProgressContainer').style.display = 'none';
                refreshStatus();
            }, 3000);
        }
        
        async function loadLogs() {
            const logs = await fetchAPI('/api/audit/log');
            if (logs && logs.length > 0) {
                const logContainer = document.getElementById('logContainer');
                let html = '';
                for (let i = 0; i < Math.min(logs.length, 50); i++) {
                    const log = logs[i];
                    html += '<div class="log-entry">[' + log.ts + '] ' + log.action + ': ' + (log.detail ? log.detail.substring(0, 100) : '') + '</div>';
                }
                logContainer.innerHTML = html;
            }
        }
        
        async function loadNodeId() {
            const status = await fetchAPI('/api/sync/status');
            if (status && status.node_id) {
                document.getElementById('nodeId').innerText = status.node_id;
            }
        }
        
        refreshStatus();
        loadLogs();
        loadNodeId();
        refreshInterval = setInterval(function() {
            refreshStatus();
            loadLogs();
        }, 5000);
    </script>
</body>
</html>`
}

func (n *NodoAlset) publishAdminPanelCID() {
	html := n.getAdminPanelHTML()
	cid, err := n.GenerarCID([]byte(html))
	if err != nil {
		fmt.Println("❌ Error generando CID del panel de administración:", err)
		return
	}
	config := NodoConfig{
		AdminPanelCID: cid,
		IsGenesis:     true,
		Version:       "4.0.0-PTEC-AN",
		LastUpdate:    time.Now().Unix(),
	}
	configBytes, _ := json.Marshal(config)
	n.GenerarCID(configBytes)
	os.WriteFile("nodo_config.json", configBytes, 0644)
	announce := map[string]string{
		"tipo": "admin_panel_announce",
		"cid":  cid,
	}
	announceBytes, _ := json.Marshal(announce)
	if n.topic != nil {
		n.topic.Publish(n.ctx, announceBytes)
	}
	fmt.Println("📢 Panel de administración publicado en IPFS con CID:", cid)
}

func (n *NodoAlset) handleAdminPanelAnnounce(update map[string]string) {
	cid := update["cid"]
	if cid == "" {
		return
	}
	panelPath := filepath.Join(StaticDir, "index.html")
	if _, err := os.Stat(panelPath); err == nil {
		return
	}
	fmt.Println("📥 Descargando panel de administración desde la red...")
	data, err := n.BuscarContenidoPorCID(cid)
	if err != nil {
		fmt.Println("❌ Error descargando panel de administración:", err)
		return
	}
	os.MkdirAll(StaticDir, 0755)
	err = os.WriteFile(panelPath, data, 0644)
	if err != nil {
		fmt.Println("❌ Error guardando panel de administración:", err)
		return
	}
	fmt.Println("✅ Panel de administración descargado y guardado en:", panelPath)
}

func (n *NodoAlset) ensureStaticFiles() {
	os.MkdirAll(StaticDir, 0755)
	os.MkdirAll(filepath.Join(StaticDir, "apps"), 0755)
	panelPath := filepath.Join(StaticDir, "index.html")
	if _, err := os.Stat(panelPath); err == nil {
		return
	}
	if configData, err := os.ReadFile("nodo_config.json"); err == nil {
		var config NodoConfig
		if json.Unmarshal(configData, &config) == nil && config.AdminPanelCID != "" {
			fmt.Println("📥 Restaurando panel de administración desde configuración local...")
			data, err := n.BuscarContenidoPorCID(config.AdminPanelCID)
			if err == nil {
				os.WriteFile(panelPath, data, 0644)
				fmt.Println("✅ Panel de administración restaurado")
				return
			}
		}
	}
	fmt.Println("🌟 Nodo genesis: creando panel de administración inicial...")
	n.publishAdminPanelCID()
	html := n.getAdminPanelHTML()
	os.WriteFile(panelPath, []byte(html), 0644)
	fmt.Println("✅ Panel de administración creado en:", panelPath)
}

// =============================================================================
// SISTEMA DE SINCRONIZACIÓN (EXISTENTE)
// =============================================================================

func (n *NodoAlset) InitSyncManager() *SyncManager {
	config := SyncConfig{
		Mode:           SyncModeQuick,
		AutoSyncDays:   7,
		MaxQuickBlocks: 100,
	}
	if data, err := os.ReadFile("sync_config.json"); err == nil {
		json.Unmarshal(data, &config)
	}
	if data, err := os.ReadFile("last_sync.json"); err == nil {
		var lastSync struct {
			Timestamp int64 `json:"timestamp"`
		}
		json.Unmarshal(data, &lastSync)
		config.LastSyncTime = lastSync.Timestamp
	}
	sm := &SyncManager{
		nodo:   n,
		config: config,
	}
	n.syncManager = sm
	return sm
}

func (sm *SyncManager) SaveConfig() {
	data, _ := json.MarshalIndent(sm.config, "", "  ")
	os.WriteFile("sync_config.json", data, 0644)
}

func (sm *SyncManager) SaveLastSyncTime() {
	data, _ := json.Marshal(map[string]int64{"timestamp": time.Now().Unix()})
	os.WriteFile("last_sync.json", data, 0644)
}

func (n *NodoAlset) QuickStartup() {
    fmt.Println("🚀 Arranque rápido iniciado...")
    n.ensureStaticFiles()
    n.CargarEstado()
    go n.connectToNetwork()

    go func() {
        time.Sleep(3 * time.Second)
        if n.syncManager != nil && n.shouldQuickSync() {
            n.syncManager.PerformQuickSync()
        }
    }()

    // ---- Iniciar cliente de pulsos SOLO si NO estamos en Render ----
    if os.Getenv("RENDER") == "" {
        go n.startPulseClients()
        fmt.Println("⚡ Cliente de pulsos iniciado (modo local)")
    } else {
        fmt.Println("⚡ Cliente de pulsos desactivado (nodo en Render, solo actúa como servidor)")
    }

    fmt.Println("✅ Nodo operativo (sincronización en background)")
    fmt.Println("🌐 Panel de administración: http://localhost:" + getPort() + "/static/index.html")
}
func getPort() string {
	return "8080"
}

func (n *NodoAlset) shouldQuickSync() bool {
	if len(n.agentes) == 0 {
		return true
	}
	if n.syncManager.config.LastSyncTime == 0 {
		return true
	}
	daysSinceSync := (time.Now().Unix() - n.syncManager.config.LastSyncTime) / 86400
	return daysSinceSync > int64(n.syncManager.config.AutoSyncDays)
}

func (sm *SyncManager) PerformQuickSync() {
	sm.mu.Lock()
	if sm.isSyncing {
		sm.mu.Unlock()
		return
	}
	sm.isSyncing = true
	sm.mu.Unlock()
	defer func() { sm.isSyncing = false }()
	fmt.Println("⚡ Sincronización rápida iniciada...")
	peers := sm.nodo.host.Network().Peers()
	if len(peers) == 0 {
		fmt.Println("⚠️ No hay peers disponibles para sincronizar")
		return
	}
	for _, p := range peers {
		stream, err := sm.nodo.host.NewStream(sm.nodo.ctx, p, AlsetDataExchangeID)
		if err != nil {
			continue
		}
		stream.Write([]byte("SYNC_QUICK_REQUEST\n"))
		sizeBuf := make([]byte, 8)
		_, err = io.ReadFull(stream, sizeBuf)
		if err != nil {
			stream.Close()
			continue
		}
		size := binary.BigEndian.Uint64(sizeBuf)
		data := make([]byte, size)
		_, err = io.ReadFull(stream, data)
		stream.Close()
		if err != nil {
			continue
		}
		gz, _ := gzip.NewReader(bytes.NewReader(data))
		decompressed, _ := io.ReadAll(gz)
		gz.Close()
		var response struct {
			Agentes      map[string]*Agente `json:"agentes"`
			Nombres      map[string]string  `json:"nombres"`
			RecentBlocks map[string][]byte  `json:"recent_blocks"`
			NeuralState  *NeuralState       `json:"neural_state"`
		}
		json.Unmarshal(decompressed, &response)
		sm.nodo.mu.Lock()
		for k, v := range response.Agentes {
			if _, exists := sm.nodo.agentes[k]; !exists {
				sm.nodo.agentes[k] = v
			}
		}
		for k, v := range response.Nombres {
			if _, exists := sm.nodo.nombres[k]; !exists {
				sm.nodo.nombres[k] = v
			}
		}
		for k, v := range response.RecentBlocks {
			if _, exists := sm.nodo.blockstore[k]; !exists {
				sm.nodo.blockstore[k] = v
				os.WriteFile(filepath.Join(BlocksDir, k), v, 0644)
			}
		}
		if response.NeuralState != nil && sm.nodo.neuralState == nil {
			sm.nodo.neuralState = response.NeuralState
		}
		sm.nodo.mu.Unlock()
		sm.nodo.PersistirLocamente()
		sm.SaveLastSyncTime()
		fmt.Printf("✅ Sincronización rápida completada: %d agentes, %d bloques\n",
			len(response.Agentes), len(response.RecentBlocks))
		return
	}
}

func (sm *SyncManager) PerformFullSync(ctx context.Context, progressCallback func(float64)) error {
	sm.mu.Lock()
	if sm.isSyncing {
		sm.mu.Unlock()
		return fmt.Errorf("ya hay una sincronización en curso")
	}
	sm.isSyncing = true
	sm.mu.Unlock()
	defer func() { sm.isSyncing = false }()
	fmt.Println("🔄 Sincronización completa iniciada...")
	if progressCallback != nil {
		progressCallback(0.1)
	}
	peers := sm.nodo.host.Network().Peers()
	if len(peers) == 0 {
		return fmt.Errorf("no hay peers disponibles para sincronizar")
	}
	for _, p := range peers {
		stream, err := sm.nodo.host.NewStream(ctx, p, AlsetDataExchangeID)
		if err != nil {
			continue
		}
		stream.Write([]byte("SYNC_FULL_REQUEST\n"))
		sizeBuf := make([]byte, 8)
		_, err = io.ReadFull(stream, sizeBuf)
		if err != nil {
			stream.Close()
			continue
		}
		size := binary.BigEndian.Uint64(sizeBuf)
		data := make([]byte, size)
		_, err = io.ReadFull(stream, data)
		stream.Close()
		if err != nil {
			continue
		}
		gz, _ := gzip.NewReader(bytes.NewReader(data))
		decompressed, _ := io.ReadAll(gz)
		gz.Close()
		var fullState struct {
			Agentes map[string]*Agente `json:"agentes"`
			Nombres map[string]string  `json:"nombres"`
		}
		json.Unmarshal(decompressed, &fullState)
		if progressCallback != nil {
			progressCallback(0.5)
		}
		sm.nodo.mu.Lock()
		for k, v := range fullState.Agentes {
			sm.nodo.agentes[k] = v
		}
		for k, v := range fullState.Nombres {
			sm.nodo.nombres[k] = v
		}
		sm.nodo.mu.Unlock()
		if progressCallback != nil {
			progressCallback(1.0)
		}
		sm.nodo.PersistirLocamente()
		sm.SaveLastSyncTime()
		fmt.Printf("✅ Sincronización completa: %d agentes, %d nombres\n",
			len(fullState.Agentes), len(fullState.Nombres))
		return nil
	}
	return fmt.Errorf("no se pudo completar la sincronización con ningún peer")
}

func (n *NodoAlset) connectToNetwork() {
	time.Sleep(2 * time.Second)
	fmt.Println("🌐 Conectado a la red Alset")
}

// =============================================================================
// HANDLERS DE MÓDULOS, ENTIDADES, SEGURIDAD (EXISTENTES)
// =============================================================================

func (n *NodoAlset) crearModulo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nombre    string                 `json:"nombre"`
		Rol       string                 `json:"rol"`
		Atributos map[string]interface{} `json:"atributos"`
		Owner     string                 `json:"owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	id := generarUUID()
	modulo := &Modulo{
		ID:         id,
		Nombre:     req.Nombre,
		Rol:        req.Rol,
		Atributos:  req.Atributos,
		Relaciones: []string{},
		Owner:      req.Owner,
		CreatedAt:  time.Now().Unix(),
	}
	muModulos.Lock()
	modulosGlobales[id] = modulo
	muModulos.Unlock()
	n.Auditoria("MODULO_CREADO", fmt.Sprintf("ID: %s | Nombre: %s", id, req.Nombre))
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) listarModulos(w http.ResponseWriter, r *http.Request) {
	muModulos.RLock()
	defer muModulos.RUnlock()
	lista := make([]*Modulo, 0, len(modulosGlobales))
	for _, m := range modulosGlobales {
		lista = append(lista, m)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	muModulos.RLock()
	modulo, exists := modulosGlobales[id]
	muModulos.RUnlock()
	if !exists {
		http.Error(w, "Módulo no encontrado", 404)
		return
	}
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) actualizarModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	muModulos.Lock()
	defer muModulos.Unlock()
	modulo, exists := modulosGlobales[id]
	if !exists {
		http.Error(w, "Módulo no encontrado", 404)
		return
	}
	if nombre, ok := updates["nombre"]; ok {
		modulo.Nombre = nombre.(string)
	}
	if rol, ok := updates["rol"]; ok {
		modulo.Rol = rol.(string)
	}
	if atributos, ok := updates["atributos"]; ok {
		modulo.Atributos = atributos.(map[string]interface{})
	}
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) eliminarModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	muModulos.Lock()
	delete(modulosGlobales, id)
	muModulos.Unlock()
	n.Auditoria("MODULO_ELIMINADO", id)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (n *NodoAlset) crearEntidad(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tipo      string                 `json:"tipo"`
		Atributos map[string]interface{} `json:"atributos"`
		HeredaDe  string                 `json:"hereda_de"`
		ModuloID  string                 `json:"modulo_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	atributosFinales := make(map[string]interface{})
	if req.HeredaDe != "" {
		muEntidades.RLock()
		if padre, exists := entidadesGlobales[req.HeredaDe]; exists {
			for k, v := range padre.Atributos {
				atributosFinales[k] = v
			}
		}
		muEntidades.RUnlock()
	}
	for k, v := range req.Atributos {
		atributosFinales[k] = v
	}
	id := generarUUID()
	entidad := &EntidadProgramatica{
		ID:        id,
		Tipo:      req.Tipo,
		Atributos: atributosFinales,
		HeredaDe:  req.HeredaDe,
		ModuloID:  req.ModuloID,
	}
	muEntidades.Lock()
	entidadesGlobales[id] = entidad
	muEntidades.Unlock()
	n.Auditoria("ENTIDAD_CREADA", fmt.Sprintf("Tipo: %s | ID: %s", req.Tipo, id))
	json.NewEncoder(w).Encode(entidad)
}

func (n *NodoAlset) listarEntidades(w http.ResponseWriter, r *http.Request) {
	muEntidades.RLock()
	defer muEntidades.RUnlock()
	lista := make([]*EntidadProgramatica, 0, len(entidadesGlobales))
	for _, e := range entidadesGlobales {
		lista = append(lista, e)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerEntidad(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/entidades/")
	muEntidades.RLock()
	entidad, exists := entidadesGlobales[id]
	muEntidades.RUnlock()
	if !exists {
		http.Error(w, "Entidad no encontrada", 404)
		return
	}
	json.NewEncoder(w).Encode(entidad)
}

func (n *NodoAlset) crearRelacion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntidadA     string `json:"entidad_a"`
		EntidadB     string `json:"entidad_b"`
		Tipo         string `json:"tipo"`
		Cardinalidad string `json:"cardinalidad"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	id := generarUUID()
	relacion := &RelacionEntidad{
		ID:           id,
		EntidadA:     req.EntidadA,
		EntidadB:     req.EntidadB,
		Tipo:         req.Tipo,
		Cardinalidad: req.Cardinalidad,
	}
	relacionesGlobales[id] = relacion
	json.NewEncoder(w).Encode(relacion)
}

func (n *NodoAlset) listarRelaciones(w http.ResponseWriter, r *http.Request) {
	lista := make([]*RelacionEntidad, 0, len(relacionesGlobales))
	for _, rel := range relacionesGlobales {
		lista = append(lista, rel)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerRelacionesDeEntidad(w http.ResponseWriter, r *http.Request) {
	entidadID := strings.TrimPrefix(r.URL.Path, "/api/entidades/")
	entidadID = strings.TrimSuffix(entidadID, "/relaciones")
	var resultado []*RelacionEntidad
	for _, rel := range relacionesGlobales {
		if rel.EntidadA == entidadID || rel.EntidadB == entidadID {
			resultado = append(resultado, rel)
		}
	}
	json.NewEncoder(w).Encode(resultado)
}

func (n *NodoAlset) generarTokenAlset(agentID string, roles []string, duracionHoras int) (*TokenAlset, error) {
	agente, exists := n.agentes[agentID]
	if !exists {
		return nil, fmt.Errorf("agente no encontrado")
	}
	tokenID := generarUUID()
	expiresAt := time.Now().Add(time.Duration(duracionHoras) * time.Hour).Unix()
	payload := fmt.Sprintf("%s|%s|%d|%s", tokenID, agentID, expiresAt, strings.Join(roles, ","))
	signature := hex.EncodeToString([]byte(payload + n.host.ID().String()))[:64]
	token := &TokenAlset{
		Token:     tokenID,
		AgentID:   agentID,
		RootCID:   agente.RootCID,
		ExpiresAt: expiresAt,
		Roles:     roles,
		Permisos:  n.rolesAPermisos(roles),
		Signature: signature,
	}
	muTokens.Lock()
	tokensActivos[tokenID] = token
	muTokens.Unlock()
	return token, nil
}

func (n *NodoAlset) rolesAPermisos(roles []string) []string {
	permisosMap := make(map[string]bool)
	for _, rol := range roles {
		switch rol {
		case "admin":
			permisosMap["*"] = true
		case "editor":
			permisosMap["modulo:crear"] = true
			permisosMap["modulo:editar"] = true
			permisosMap["entidad:crear"] = true
			permisosMap["entidad:editar"] = true
		case "viewer":
			permisosMap["modulo:ver"] = true
			permisosMap["entidad:ver"] = true
		case "cliente":
			permisosMap["producto:ver"] = true
			permisosMap["compra:crear"] = true
		case "vendedor":
			permisosMap["producto:crear"] = true
			permisosMap["producto:editar"] = true
			permisosMap["venta:ver"] = true
		}
	}
	var permisos []string
	for p := range permisosMap {
		permisos = append(permisos, p)
	}
	return permisos
}

func (n *NodoAlset) validarToken(tokenString string) (*TokenAlset, error) {
	muTokens.RLock()
	token, exists := tokensActivos[tokenString]
	muTokens.RUnlock()
	if !exists {
		return nil, fmt.Errorf("token inválido")
	}
	if time.Now().Unix() > token.ExpiresAt {
		muTokens.Lock()
		delete(tokensActivos, tokenString)
		muTokens.Unlock()
		return nil, fmt.Errorf("token expirado")
	}
	payload := fmt.Sprintf("%s|%s|%d|%s", token.Token, token.AgentID, token.ExpiresAt, strings.Join(token.Roles, ","))
	expectedSig := hex.EncodeToString([]byte(payload + n.host.ID().String()))[:64]
	if token.Signature != expectedSig {
		return nil, fmt.Errorf("firma inválida")
	}
	return token, nil
}

func (n *NodoAlset) verificarPermiso(token *TokenAlset, permisoRequerido string) bool {
	if token == nil {
		return false
	}
	for _, p := range token.Permisos {
		if p == "*" || p == permisoRequerido {
			return true
		}
	}
	return false
}

func (n *NodoAlset) asignarRol(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string   `json:"agent_id"`
		Roles   []string `json:"roles"`
		Modulos []string `json:"modulos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if _, exists := n.agentes[req.AgentID]; !exists {
		http.Error(w, "Agente no encontrado", 404)
		return
	}
	usuarioRoles := &UsuarioRoles{
		AgentID: req.AgentID,
		Roles:   req.Roles,
		Modulos: req.Modulos,
	}
	rolesData, _ := json.Marshal(usuarioRoles)
	cid, _ := n.GenerarCID(rolesData)
	rolesGlobales[req.AgentID] = req.Roles
	n.Auditoria("ROL_ASIGNADO", fmt.Sprintf("Agente: %s | Roles: %v", req.AgentID, req.Roles))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"cid":    cid,
	})
}

func (n *NodoAlset) obtenerRoles(w http.ResponseWriter, r *http.Request) {
	agentID := strings.TrimPrefix(r.URL.Path, "/api/roles/")
	roles, exists := rolesGlobales[agentID]
	if !exists {
		json.NewEncoder(w).Encode([]string{})
		return
	}
	json.NewEncoder(w).Encode(roles)
}

func (n *NodoAlset) crearTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID       string   `json:"agent_id"`
		Roles         []string `json:"roles"`
		DuracionHoras int      `json:"duracion_horas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	duracion := req.DuracionHoras
	if duracion <= 0 {
		duracion = 24
	}
	token, err := n.generarTokenAlset(req.AgentID, req.Roles, duracion)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(token)
}

func (n *NodoAlset) validarTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Token requerido", 400)
		return
	}
	token, err := n.validarToken(tokenStr)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	json.NewEncoder(w).Encode(token)
}

func (n *NodoAlset) revocarTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	muTokens.Lock()
	delete(tokensActivos, req.Token)
	muTokens.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"status": "revocado"})
}

func (n *NodoAlset) SetAgentRoot(agentID string, rootCID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if a, ok := n.agentes[agentID]; ok {
		a.RootCID = rootCID
		a.UltimaActual = time.Now().Unix()
	}
}

// =============================================================================
// HANDLERS DE SINCRONIZACIÓN HTTP (EXISTENTES)
// =============================================================================

func (n *NodoAlset) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if n.syncManager == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not_initialized",
		})
		return
	}
	n.syncManager.mu.RLock()
	defer n.syncManager.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "idle",
		"is_syncing":     n.syncManager.isSyncing,
		"last_sync":      n.syncManager.config.LastSyncTime,
		"mode":           n.syncManager.config.Mode,
		"agents_count":   len(n.agentes),
		"blocks_count":   len(n.blockstore),
		"auto_sync_days": n.syncManager.config.AutoSyncDays,
		"node_id":        n.host.ID().String(),
		"progress":       globalSyncProgress,
	})
}

func (n *NodoAlset) handleSyncFull(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if n.syncManager == nil {
		http.Error(w, "Sync manager no inicializado", 500)
		return
	}
	go func() {
		globalSyncProgress.Status = "syncing"
		globalSyncProgress.Stage = "full_sync"
		err := n.syncManager.PerformFullSync(context.Background(), func(progress float64) {
			globalSyncProgress.Percent = progress
			globalSyncProgress.Current = int(progress * 100)
			globalSyncProgress.Total = 100
		})
		if err != nil {
			globalSyncProgress.Status = "error"
			globalSyncProgress.Stage = err.Error()
		} else {
			globalSyncProgress.Status = "idle"
			globalSyncProgress.Stage = "completed"
		}
	}()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "sync_started",
		"message": "Sincronización completa iniciada en background",
	})
}

func (n *NodoAlset) handleSyncQuick(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if n.syncManager == nil {
		http.Error(w, "Sync manager no inicializado", 500)
		return
	}
	go n.syncManager.PerformQuickSync()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "sync_started",
		"message": "Sincronización rápida iniciada",
	})
}

func (n *NodoAlset) handleSyncConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var config struct {
		Mode           string `json:"mode"`
		AutoSyncDays   int    `json:"auto_sync_days"`
		MaxQuickBlocks int    `json:"max_quick_blocks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if n.syncManager != nil {
		switch config.Mode {
		case "quick":
			n.syncManager.config.Mode = SyncModeQuick
		case "full":
			n.syncManager.config.Mode = SyncModeFull
		case "incremental":
			n.syncManager.config.Mode = SyncModeIncremental
		}
		if config.AutoSyncDays > 0 {
			n.syncManager.config.AutoSyncDays = config.AutoSyncDays
		}
		if config.MaxQuickBlocks > 0 {
			n.syncManager.config.MaxQuickBlocks = config.MaxQuickBlocks
		}
		n.syncManager.SaveConfig()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "configured",
		"config": config,
	})
}

// =============================================================================
// HANDLERS HTTP ADICIONALES (EXISTENTES)
// =============================================================================

func (n *NodoAlset) handlePoHEvent(w http.ResponseWriter, r *http.Request) {
	var event PoHEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid event data", 400)
		return
	}
	globalPoH.Lock()
	if globalPoH.sessionID == "" {
		globalPoH.sessionID = hex.EncodeToString([]byte(time.Now().String()))[:16]
	}
	event.Timestamp = time.Now().Unix()
	globalPoH.events = append(globalPoH.events, event)
	globalPoH.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"status": "event_received"})
}

func (n *NodoAlset) handlePoHProof(w http.ResponseWriter, r *http.Request) {
	globalPoH.Lock()
	defer globalPoH.Unlock()
	if len(globalPoH.events) == 0 {
		http.Error(w, "No events collected", 400)
		return
	}
	var eventsData []byte
	for _, ev := range globalPoH.events {
		evData, _ := json.Marshal(ev)
		eventsData = append(eventsData, evData...)
	}
	hash := make([]byte, 32)
	copy(hash, eventsData[:32])
	proof := HumanityProof{
		SessionID: globalPoH.sessionID,
		Events:    globalPoH.events,
		FinalSig:  hex.EncodeToString(hash),
	}
	proofBytes, _ := json.Marshal(proof)
	proofCID, _ := n.GenerarCID(proofBytes)
	globalPoH.events = []PoHEvent{}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"proof_cid": proofCID,
		"session":   proof.SessionID,
		"events":    len(proof.Events),
	})
}

func (n *NodoAlset) handleDNSList(w http.ResponseWriter, r *http.Request) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nombres": n.nombres,
	})
}

func (n *NodoAlset) handleDNSResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	n.mu.RLock()
	agentID, exists := n.nombres[req.Alias]
	n.mu.RUnlock()
	if !exists {
		http.Error(w, "Nombre no encontrado", 404)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alias":    req.Alias,
		"agent_id": agentID,
		"status":   "active",
	})
}

func (n *NodoAlset) handleDNSDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	n.mu.Lock()
	delete(n.nombres, req.Alias)
	n.mu.Unlock()
	n.PersistirLocamente()
	n.Auditoria("DNS_ELIMINADO", fmt.Sprintf("Alias: %s", req.Alias))
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"alias":  req.Alias,
	})
}

func (n *NodoAlset) handleNetworkPeers(w http.ResponseWriter, r *http.Request) {
	peers := n.host.Network().Peers()
	peerInfo := make([]map[string]interface{}, 0, len(peers))
	for _, p := range peers {
		peerInfo = append(peerInfo, map[string]interface{}{
			"id":        p.String(),
			"addresses": n.host.Network().Peerstore().Addrs(p),
			"connected": n.host.Network().Connectedness(p).String(),
		})
	}
	json.NewEncoder(w).Encode(peerInfo)
}

func (n *NodoAlset) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("audit.jsonl")
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	lines := strings.Split(string(data), "\n")
	logs := make([]map[string]interface{}, 0)
	for _, line := range lines {
		if line == "" {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
			logs = append(logs, logEntry)
		}
	}
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	json.NewEncoder(w).Encode(logs)
}

func (n *NodoAlset) handleDebugEstado(w http.ResponseWriter, r *http.Request) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agentes_count": len(n.agentes),
		"nombres_count": len(n.nombres),
		"agentes":       n.agentes,
	})
}

func (n *NodoAlset) handleAppsRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err := r.ParseMultipartForm(50 << 20)
		if err != nil {
			http.Error(w, "Error parsing form: "+err.Error(), 400)
			return
		}
		appName := r.FormValue("appName")
		if appName == "" {
			http.Error(w, "appName required", 400)
			return
		}
		appDir := filepath.Join(StaticDir, "apps", appName)
		if err := os.MkdirAll(appDir, 0755); err != nil {
			http.Error(w, "Error creating app directory: "+err.Error(), 500)
			return
		}
		files := r.MultipartForm.File["files"]
		var savedFiles []string
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue
			}
			defer file.Close()
			filename := fileHeader.Filename
			targetPath := filepath.Join(appDir, filename)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				continue
			}
			data, err := io.ReadAll(file)
			if err != nil {
				continue
			}
			if err := os.WriteFile(targetPath, data, 0644); err != nil {
				continue
			}
			savedFiles = append(savedFiles, filename)
		}
		if len(savedFiles) == 0 {
			http.Error(w, "No files were saved", 400)
			return
		}
		fmt.Printf("📁 Archivos guardados en: %s (%d archivos)\n", appDir, len(savedFiles))
		cid, err := n.IpfsAddDirectory(appDir)
		if err != nil {
			fmt.Printf("⚠️ Error uploading to IPFS: %v\n", err)
		}
		appID := fmt.Sprintf("app-%s-%d", appName, time.Now().Unix())
		createCmd := fmt.Sprintf(`(crear-agente "%s")`, appID)
		n.lisp.Eval(createCmd)
		if cid != "" {
			setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, appID, cid)
			n.lisp.Eval(setRootCmd)
		}
		registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, appName, appID)
		n.lisp.Eval(registerCmd)
		indexPath := filepath.Join(appDir, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			indexContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        html, body { width: 100%%; height: 100%%; overflow: hidden; background: #000; }
        #app { width: 100vw; height: 100vh; display: flex; }
    </style>
</head>
<body>
    <div id="app"></div>
    <script type="module" src="/apps/%s/%s.js"></script>
</body>
</html>`, appName, appName, appName)
			os.WriteFile(indexPath, []byte(indexContent), 0644)
			fmt.Printf("📄 Index.html creado: %s\n", indexPath)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "registered",
			"name":     appName,
			"cid":      cid,
			"url":      fmt.Sprintf("/w/%s.app.ans", appName),
			"agent_id": appID,
			"path":     appDir,
			"files":    len(savedFiles),
		})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), 400)
		return
	}
	if req.Name == "" {
		http.Error(w, "App name required", 400)
		return
	}
	appPath := filepath.Join(StaticDir, "apps", req.Name)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		http.Error(w, "App folder not found: "+req.Name, 404)
		return
	}
	cid, err := n.IpfsAddDirectory(appPath)
	if err != nil {
		http.Error(w, "Error uploading to IPFS: "+err.Error(), 500)
		return
	}
	appID := fmt.Sprintf("app-%s-%d", req.Name, time.Now().Unix())
	createCmd := fmt.Sprintf(`(crear-agente "%s")`, appID)
	n.lisp.Eval(createCmd)
	setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, appID, cid)
	n.lisp.Eval(setRootCmd)
	registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, req.Name, appID)
	n.lisp.Eval(registerCmd)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "registered",
		"name":     req.Name,
		"cid":      cid,
		"url":      fmt.Sprintf("/w/%s.app.ans", req.Name),
		"agent_id": appID,
	})
}

func (n *NodoAlset) handleAppsList(w http.ResponseWriter, r *http.Request) {
	appsDir := filepath.Join(StaticDir, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	var apps []map[string]interface{}
	for _, entry := range entries {
		if entry.IsDir() {
			apps = append(apps, map[string]interface{}{
				"name": entry.Name(),
				"path": fmt.Sprintf("/static/apps/%s", entry.Name()),
			})
		}
	}
	json.NewEncoder(w).Encode(apps)
}

func (n *NodoAlset) handlePrismVerificar(w http.ResponseWriter, r *http.Request) {
	certCID := r.URL.Query().Get("cid")
	if certCID == "" {
		http.Error(w, "CID requerido", 400)
		return
	}
	certBytes, err := n.BuscarContenidoPorCID(certCID)
	if err != nil {
		n.Auditoria("VERIFICACION_FALLIDA", fmt.Sprintf("CID: %s | Motivo: No encontrado", certCID))
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Certificado no encontrado"})
		return
	}
	var vc map[string]interface{}
	if err := json.Unmarshal(certBytes, &vc); err != nil {
		n.Auditoria("VERIFICACION_ERROR", fmt.Sprintf("CID: %s | Motivo: JSON inválido", certCID))
		http.Error(w, "JSON inválido", 400)
		return
	}
	proofInterface, hasProof := vc["proof"]
	if !hasProof {
		n.Auditoria("VERIFICACION_ERROR", fmt.Sprintf("CID: %s | Motivo: Sin proof", certCID))
		http.Error(w, "Certificado sin prueba", 400)
		return
	}
	proof, ok := proofInterface.(map[string]interface{})
	if !ok {
		http.Error(w, "Proof inválido", 400)
		return
	}
	signatureStr := ""
	if pv, ok := proof["proofValue"]; ok {
		signatureStr, _ = pv.(string)
	} else if jws, ok := proof["jws"]; ok {
		signatureStr, _ = jws.(string)
	}
	if signatureStr == "" {
		http.Error(w, "Firma no encontrada", 400)
		return
	}
	firmaBytes, err := hex.DecodeString(signatureStr)
	if err != nil {
		http.Error(w, "Firma inválida", 400)
		return
	}
	vcWithoutProof := make(map[string]interface{})
	for k, v := range vc {
		if k != "proof" {
			vcWithoutProof[k] = v
		}
	}
	canonicalBytes, err := canonicalizeJSON(vcWithoutProof)
	if err != nil {
		http.Error(w, "Error canonicalizando", 500)
		return
	}
	rawKey, _ := n.masterPrivKey.GetPublic().Raw()
	pubNative := ed25519.PublicKey(rawKey)
	esFirmaValida := ed25519.Verify(pubNative, canonicalBytes, firmaBytes)
	estaRevocado := false
	motivoRevocacion := ""
	n.mu.RLock()
	for _, blockData := range n.blockstore {
		var rev map[string]interface{}
		if err := json.Unmarshal(blockData, &rev); err != nil {
			continue
		}
		if revType, ok := rev["type"]; ok {
			var typeStr string
			switch t := revType.(type) {
			case string:
				typeStr = t
			case []interface{}:
				if len(t) > 0 {
					if s, ok := t[0].(string); ok {
						typeStr = s
					}
				}
			}
			if typeStr == "RevocationList2020Credential" {
				if subject, ok := rev["credentialSubject"].(map[string]interface{}); ok {
					if revokedCID, ok := subject["revokedCredential"].(string); ok && revokedCID == certCID {
						estaRevocado = true
						if reason, ok := subject["revocationReason"].(string); ok {
							motivoRevocacion = reason
						}
						break
					}
				}
			}
		}
	}
	n.mu.RUnlock()
	statusVerdad := "AUTÉNTICO"
	auditAccion := "VC_VERIFICADO_OK"
	if !esFirmaValida {
		statusVerdad = "FALSIFICADO / INVÁLIDO"
		auditAccion = "ALERTA_FRAUDE_FIRMA"
	} else if estaRevocado {
		statusVerdad = "REVOCADO"
		auditAccion = "CONSULTA_VC_REVOCADO"
	}
	n.Auditoria(auditAccion, fmt.Sprintf("CID: %s | Status: %s", certCID, statusVerdad))
	credentialSubject := map[string]interface{}{}
	if cs, ok := vc["credentialSubject"]; ok {
		if csMap, ok := cs.(map[string]interface{}); ok {
			credentialSubject = csMap
		}
	}
	response := map[string]interface{}{
		"cid_consultado":  certCID,
		"status_integral": statusVerdad,
		"firma_valida":    esFirmaValida,
		"revocado":        estaRevocado,
		"info_revocacion": map[string]string{"motivo": motivoRevocacion},
		"detalles": map[string]interface{}{
			"issuer":            vc["issuer"],
			"issuanceDate":      vc["issuanceDate"],
			"credentialSubject": credentialSubject,
		},
		"nodo_verificador": n.host.ID().String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *NodoAlset) handlePrismRevocar(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		CID    string `json:"cid"`
		Motivo string `json:"motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if req.CID == "" {
		http.Error(w, "CID requerido", 400)
		return
	}
	if req.Motivo == "" {
		req.Motivo = "Revocado por administrador"
	}
	_, err := n.BuscarContenidoPorCID(req.CID)
	if err != nil {
		http.Error(w, "Certificado no encontrado", 404)
		return
	}
	fecha := time.Now().Format(time.RFC3339)
	mensaje := fmt.Sprintf("REVOKE|%s|%s", req.CID, fecha)
	firma, err := n.masterPrivKey.Sign([]byte(mensaje))
	if err != nil {
		http.Error(w, "Error firmando", 500)
		return
	}
	revocationTicket := map[string]interface{}{
		"@context":     "https://www.w3.org/2018/credentials/v1",
		"id":           fmt.Sprintf("urn:uuid:%s", req.CID),
		"type":         []string{"RevocationList2020Credential"},
		"issuer":       "did:prism:tec:institutional",
		"issuanceDate": fecha,
		"credentialSubject": map[string]interface{}{
			"id":                fmt.Sprintf("did:prism:%s", req.CID[:16]),
			"revokedCredential": req.CID,
			"revocationReason":  req.Motivo,
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
	ticketBytes, _ := json.Marshal(revocationTicket)
	revokeCID, err := n.GenerarCID(ticketBytes)
	if err != nil {
		http.Error(w, "Error generando CID", 500)
		return
	}
	n.Auditoria("CERTIFICADO_REVOCADO",
		fmt.Sprintf("Cert: %s | Motivo: %s", req.CID, req.Motivo))
	update := map[string]string{"tipo": "revocacion_update", "cid": revokeCID}
	msg, _ := json.Marshal(update)
	n.topic.Publish(n.ctx, msg)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":               "revocado",
		"ticket_cid":           revokeCID,
		"certificado_revocado": req.CID,
		"fecha":                fecha,
	})
}

func (n *NodoAlset) handlePrismSellar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CID string `json:"cid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	res, _ := n.lisp.Eval(fmt.Sprintf(`(sellar-documento "%s")`, req.CID))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "Certificado Generado",
		"entidad":         "Prism@.TEC - Garante de la Verdad Digital",
		"titular":         "Dayanis Pérez Soria",
		"certificado_cid": res,
	})
}

func (n *NodoAlset) handleAdminUpdatePass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NuevaClave string `json:"nuevaClave"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte(req.NuevaClave), bcrypt.DefaultCost)
	config := NodoConfig{
		AdminPassHash: string(hashedPass),
		LastUpdate:    time.Now().Unix(),
		Version:       "4.0.0-PTEC-AN",
	}
	configBytes, _ := json.Marshal(config)
	cidStr, _ := n.GenerarCID(configBytes)
	n.Auditoria("SEGURIDAD_PASSWORD_UPDATE", fmt.Sprintf("Nuevo CID de config: %s", cidStr))
	n.AnunciarNuevoBloque(cidStr)
	fmt.Printf("🔒 [SEGURIDAD] Nueva configuración sellada con CID: %s\n", cidStr)
	json.NewEncoder(w).Encode(map[string]string{"config_cid": cidStr})
}

func (n *NodoAlset) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CID   string `json:"config_cid"`
		Clave string `json:"clave"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Solicitud inválida", 400)
		return
	}
	configBytes, err := n.BuscarContenidoPorCID(req.CID)
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Configuración no encontrada"})
		return
	}
	var config NodoConfig
	json.Unmarshal(configBytes, &config)
	err = bcrypt.CompareHashAndPassword([]byte(config.AdminPassHash), []byte(req.Clave))
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Clave incorrecta"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "authorized", "node": n.host.ID().String()})
}

// =============================================================================
// SERVICIO DE PULSOS – NUEVO
// =============================================================================

func (n *NodoAlset) broadcastPulse(eventType string, data interface{}) {
	payload, _ := json.Marshal(data)
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(payload))

	n.pulseSubscribersMu.RLock()
	defer n.pulseSubscribersMu.RUnlock()
	for sub := range n.pulseSubscribers {
		select {
		case sub.ch <- msg:
		default:
			// canal lleno, omitir
		}
	}
}

func (n *NodoAlset) startPulseClients() {
	// Si estamos en Render, no nos conectamos a nosotros mismos ni a otros servidores (por ahora)
	if os.Getenv("RENDER") != "" {
		// En Render solo actuamos como servidor de pulsos, no como cliente
		return
	}

	// Si no estamos en Render (es decir, estamos en un nodo local), conectamos a los servidores de pulsos conocidos
	knownServers := []string{
		"https://prismatec.onrender.com/api/pulse",
		// Aquí puedes añadir más URLs de otros nodos
	}
	for _, url := range knownServers {
		go n.runPulseClient(url)
	}
}
func (n *NodoAlset) runPulseClient(url string) {
	n.pulseClientsMu.Lock()
	if _, exists := n.pulseClients[url]; exists {
		n.pulseClientsMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &PulseClient{
		url:       url,
		ctx:       ctx,
		cancel:    cancel,
		reconnect: make(chan bool, 1),
	}
	n.pulseClients[url] = client
	n.pulseClientsMu.Unlock()

	defer func() {
		n.pulseClientsMu.Lock()
		delete(n.pulseClients, url)
		n.pulseClientsMu.Unlock()
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := n.connectAndListen(client)
			if err != nil {
				log.Printf("Pulse client %s: %v, reconectando en 5s", url, err)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (n *NodoAlset) connectAndListen(client *PulseClient) error {
	req, err := http.NewRequestWithContext(client.ctx, "GET", client.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status: %s", resp.Status)
	}

	client.connected = true
	defer func() { client.connected = false }()

	reader := bufio.NewReader(resp.Body)
	var eventType string
	var dataBuffer strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if dataBuffer.Len() > 0 {
				n.processPulseEvent(eventType, dataBuffer.String())
				dataBuffer.Reset()
			}
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataBuffer.WriteString(strings.TrimPrefix(line, "data: "))
		} else {
			dataBuffer.WriteString(line)
		}
	}
}

// processPulseEvent maneja los eventos entrantes desde el servidor de pulsos (SSE).
// Es el corazón de la sincronización por HTTP resiliente.
func (n *NodoAlset) processPulseEvent(eventType string, data string) {
    // Log para depuración (puedes comentar en producción)
    // log.Printf("📨 Evento recibido: %s -> %s", eventType, data)

    var payload map[string]interface{}
    if err := json.Unmarshal([]byte(data), &payload); err != nil {
        log.Printf("⚠️ Error parseando evento %s: %v", eventType, err)
        return
    }

    switch eventType {

    // ============================================================
    // EVENTOS DE AGENTES, DNS Y ROOT
    // ============================================================
    case "agent_created":
        id, _ := payload["id"].(string)
        if id == "" {
            return
        }
        n.mu.Lock()
        if _, exists := n.agentes[id]; !exists {
            n.agentes[id] = &Agente{
                ID:           id,
                RootCID:      "",
                UltimaActual: time.Now().Unix(),
                BalanceUTXO:  0,
            }
            n.mu.Unlock()
            log.Printf("📥 Agente %s recibido por pulso", id)
            n.PersistirLocamente()
        } else {
            n.mu.Unlock()
        }

    case "root_updated":
        id, _ := payload["id"].(string)
        root, _ := payload["root"].(string)
        if id == "" {
            return
        }
        n.mu.Lock()
        if a, exists := n.agentes[id]; exists {
            a.RootCID = root
            a.UltimaActual = time.Now().Unix()
            n.mu.Unlock()
            log.Printf("📥 Root actualizado para %s -> %s", id, root)
            n.PersistirLocamente()
        } else {
            n.mu.Unlock()
        }

    case "dns_registered":
        alias, _ := payload["alias"].(string)
        agent, _ := payload["agent"].(string)
        if alias == "" || agent == "" {
            return
        }
        n.mu.Lock()
        n.nombres[alias] = agent
        n.mu.Unlock()
        log.Printf("📥 DNS %s -> %s recibido por pulso", alias, agent)
        n.PersistirLocamente()

    case "agent_deleted":
        id, _ := payload["id"].(string)
        if id == "" {
            return
        }
        n.mu.Lock()
        delete(n.agentes, id)
        n.mu.Unlock()
        log.Printf("🗑️ Agente %s eliminado por pulso", id)
        n.PersistirLocamente()

    case "agent_updated":
        id, _ := payload["id"].(string)
        balance, _ := payload["balance"].(float64)
        root, _ := payload["root"].(string)
        if id == "" {
            return
        }
        n.mu.Lock()
        if a, exists := n.agentes[id]; exists {
            if balance != 0 {
                a.BalanceUTXO = balance
            }
            if root != "" {
                a.RootCID = root
            }
            a.UltimaActual = time.Now().Unix()
            n.mu.Unlock()
            log.Printf("📥 Agente %s actualizado por pulso", id)
            n.PersistirLocamente()
        } else {
            n.mu.Unlock()
        }

    // ============================================================
    // EVENTOS DE BLOQUES IPFS
    // ============================================================
    case "new_block":
        cid, _ := payload["cid"].(string)
        if cid == "" {
            return
        }
        // Verificar si ya tenemos el bloque
        n.mu.RLock()
        _, exists := n.blockstore[cid]
        n.mu.RUnlock()
        if exists {
            return
        }

        // Intentar obtener los datos del bloque (si vienen en base64)
        dataB64, _ := payload["data"].(string)
        if dataB64 != "" {
            blockData, err := base64.StdEncoding.DecodeString(dataB64)
            if err == nil {
                n.mu.Lock()
                n.blockstore[cid] = blockData
                n.mu.Unlock()
                // Guardar en disco
                os.WriteFile(filepath.Join(BlocksDir, cid), blockData, 0644)
                log.Printf("📦 Bloque %s recibido por pulso (%d bytes)", cid, len(blockData))
                return
            }
        }

        // Si no tenemos los datos, solicitarlos al emisor (o confiar en que llegará después)
        // Por simplicidad, podemos pedir el bloque directamente al servidor de pulsos
        // o usar el método existente BuscarContenidoPorCID (que usa P2P).
        // En una red puramente de pulsos, deberíamos tener un mecanismo para pedir el bloque.
        // Por ahora, lo dejamos así y confiamos en que el bloque llegue con los datos.
        // Si no, se puede implementar un evento "request_block" para solicitarlo.
case "request_block":
    cid, _ := payload["cid"].(string)
    if cid == "" {
        return
    }
    // Buscar el bloque en el blockstore del servidor
    n.mu.RLock()
    blockData, exists := n.blockstore[cid]
    n.mu.RUnlock()
    if exists {
        b64 := base64.StdEncoding.EncodeToString(blockData)
        n.broadcastPulse("new_block", map[string]interface{}{
            "cid":  cid,
            "data": b64,
        })
        log.Printf("📤 Bloque %s enviado en respuesta a request_block", cid)
    } else {
        log.Printf("⚠️ Bloque %s solicitado pero no encontrado en el servidor", cid)
    }
    // ============================================================
    // EVENTOS NEURONALES (SPIKES, SINAPSIS, ESTADO)
    // ============================================================
    case "neural_spike":
        // Convertir payload a map[string]string y procesar
        go n.procesarSpikeNeuronal(convertMapToStringMap(payload), peer.ID("pulse"))

    case "synaptic_update":
        go n.actualizarPesosSinapsis(convertMapToStringMap(payload), peer.ID("pulse"))

    case "neural_state_sync":
        go n.sincronizarEstadoNeuronal(convertMapToStringMap(payload), peer.ID("pulse"))

    // ============================================================
    // EVENTOS DE INFERENCIA DISTRIBUIDA
    // ============================================================
    case "inference_request":
        reqData, _ := payload["data"].(string)
        if reqData == "" {
            return
        }
        // Convertir a map para usar con manejarInferenciaDistribuida
        var reqMap map[string]string
        if err := json.Unmarshal([]byte(reqData), &reqMap); err != nil {
            log.Printf("⚠️ Error parseando inference_request: %v", err)
            return
        }
        go n.manejarInferenciaDistribuida(reqMap, peer.ID("pulse"))

    case "inference_response":
        respData, _ := payload["data"].(string)
        if respData == "" {
            return
        }
        go n.procesarRespuestaInferencia(map[string]string{"data": respData})

    // ============================================================
    // EVENTOS DE MEMORIA DISTRIBUIDA
    // ============================================================
    case "memory_query":
        go n.manejarConsultaMemoria(convertMapToStringMap(payload), peer.ID("pulse"))

    case "memory_response":
        respData, _ := payload["data"].(string)
        if respData == "" {
            return
        }
        go n.procesarRespuestaMemoria(map[string]string{"data": respData})

    case "memory_distributed":
        go n.manejarMemoriaDistribuida(convertMapToStringMap(payload), peer.ID("pulse"))

    // ============================================================
    // EVENTOS DE ADMINISTRACIÓN
    // ============================================================
    case "admin_panel_announce":
        go n.handleAdminPanelAnnounce(convertMapToStringMap(payload))

    // ============================================================
    // EVENTO DE PRUEBA / HEARTBEAT
    // ============================================================
    case "ping":
        // Ignorar (ya se maneja en el servidor SSE)
        // Puedes usarlo para mantener la conexión viva

    default:
        log.Printf("⚠️ Evento desconocido: %s", eventType)
    }
}

func convertMapToStringMap(m map[string]interface{}) map[string]string {
	res := make(map[string]string)
	for k, v := range m {
		res[k] = fmt.Sprintf("%v", v)
	}
	return res
}

// =============================================================================
// SERVIDOR HTTP – INCLUYE /api/pulse
// =============================================================================

func (n *NodoAlset) startHTTPServer(port string) {
	mux := http.NewServeMux()

	n.ensureStaticFiles()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(StaticDir))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/static/index.html", http.StatusFound)
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	})

	mux.HandleFunc("/w/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.Error(w, "Not found", 404)
			return
		}
		alias := strings.TrimSuffix(parts[2], ".app.ans")
		appPath := filepath.Join(StaticDir, "apps", alias, "index.html")
		if _, err := os.Stat(appPath); err == nil {
			http.ServeFile(w, r, appPath)
			return
		}
		n.mu.RLock()
		targetID, ok := n.nombres[alias+".app.ans"]
		if !ok {
			targetID = alias
		}
		agente, ok := n.agentes[targetID]
		n.mu.RUnlock()
		if !ok || agente.RootCID == "" {
			http.Error(w, "App no encontrada: "+alias, 404)
			return
		}
		data, err := n.BuscarContenidoPorCID(agente.RootCID)
		if err != nil {
			http.Error(w, "Error cargando contenido", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	mux.HandleFunc("/apps/", func(w http.ResponseWriter, r *http.Request) {
		filePath := strings.TrimPrefix(r.URL.Path, "/apps/")
		fullPath := filepath.Join(StaticDir, "apps", filePath)
		if _, err := os.Stat(fullPath); err == nil {
			ext := filepath.Ext(fullPath)
			switch ext {
			case ".js":
				w.Header().Set("Content-Type", "application/javascript")
			case ".css":
				w.Header().Set("Content-Type", "text/css")
			case ".html":
				w.Header().Set("Content-Type", "text/html")
			case ".json":
				w.Header().Set("Content-Type", "application/json")
			default:
				w.Header().Set("Content-Type", "application/octet-stream")
			}
			http.ServeFile(w, r, fullPath)
			return
		}
		http.Error(w, "Archivo no encontrado", 404)
	})

	// ---- PULSO: endpoint SSE ----
	mux.HandleFunc("/api/pulse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", 500)
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		sub := &SSESubscriber{
			ch:     make(chan string, 10),
			ctx:    ctx,
			cancel: cancel,
		}

		n.pulseSubscribersMu.Lock()
		n.pulseSubscribers[sub] = true
		n.pulseSubscribersMu.Unlock()

		defer func() {
			n.pulseSubscribersMu.Lock()
			delete(n.pulseSubscribers, sub)
			n.pulseSubscribersMu.Unlock()
			close(sub.ch)
			cancel()
		}()

		state := map[string]interface{}{
			"node_id": n.host.ID().String(),
			"agents":  len(n.agentes),
			"blocks":  len(n.blockstore),
			"time":    time.Now().Unix(),
		}
		stateJSON, _ := json.Marshal(state)
		fmt.Fprintf(w, "event: connected\ndata: %s\n\n", stateJSON)
		flusher.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg := <-sub.ch:
				fmt.Fprint(w, msg)
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
				flusher.Flush()
			case <-ctx.Done():
				return
			}
		}
	})

	// Dentro de startHTTPServer, junto a los otros endpoints
	mux.HandleFunc("/api/pulse/emit", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", 405)
        return
    }

    var req struct {
        EventType string          `json:"eventType"`
        Data      json.RawMessage `json:"data"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", 400)
        return
    }

    // Convertir data a map para broadcast
    var payload map[string]interface{}
    if err := json.Unmarshal(req.Data, &payload); err != nil {
        http.Error(w, "Invalid data", 400)
        return
    }

    // Retransmitir a todos los suscriptores
    go n.broadcastPulse(req.EventType, payload)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "emitted"})
	})
	
	// ---- RESTO DE ENDPOINTS (copiados del original) ----
	mux.HandleFunc("/api/ipfs/list", func(w http.ResponseWriter, r *http.Request) {
		n.mu.RLock()
		defer n.mu.RUnlock()
		blocks := make([]BlockInfo, 0, len(n.blockstore))
		for cid, data := range n.blockstore {
			preview := string(data)
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			blocks = append(blocks, BlockInfo{
				CID:     cid,
				Size:    len(data),
				Preview: preview,
			})
		}
		json.NewEncoder(w).Encode(blocks)
	})

	mux.HandleFunc("/api/network/peers", n.handleNetworkPeers)
	mux.HandleFunc("/api/dns/list", n.handleDNSList)
	mux.HandleFunc("/api/dns/resolve", n.handleDNSResolve)
	mux.HandleFunc("/api/dns/delete", n.handleDNSDelete)
	mux.HandleFunc("/api/agentes/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/agentes/")
		if strings.HasSuffix(path, "/root") {
			id := strings.TrimSuffix(path, "/root")
			n.mu.RLock()
			agent, exists := n.agentes[id]
			n.mu.RUnlock()
			if !exists {
				http.Error(w, "Agente no encontrado", 404)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"agent_id":   id,
				"root_cid":   agent.RootCID,
				"updated_at": agent.UltimaActual,
			})
			return
		}
		n.mu.RLock()
		defer n.mu.RUnlock()
		json.NewEncoder(w).Encode(n.agentes)
	})

	mux.HandleFunc("/api/ipfs/fetch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			CID string `json:"cid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		data, err := n.BuscarContenidoPorCID(req.CID)
		if err != nil {
			http.Error(w, "No encontrado", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cid":  req.CID,
			"data": string(data),
			"size": len(data),
		})
	})

	mux.HandleFunc("/api/audit/log", n.handleAuditLog)
	mux.HandleFunc("/api/crear-agente", func(w http.ResponseWriter, r *http.Request) {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			http.Error(w, "Error generando llave", 500)
			return
		}
		id := hex.EncodeToString(pub[:8])
		balanceInicial := 0.0
		nuevoAgente := &Agente{
			ID:           id,
			BalanceUTXO:  balanceInicial,
			UltimaActual: time.Now().Unix(),
		}
		n.mu.Lock()
		n.agentes[id] = nuevoAgente
		n.mu.Unlock()
		n.Auditoria("AGENTE_REGISTRADO_HTTP", fmt.Sprintf("ID: %s | InitBalance: %f", id, balanceInicial))
		n.PersistirLocamente()
		go n.SincronizarConPares()
		go n.broadcastPulse("agent_created", map[string]interface{}{
			"id":   id,
			"root": "",
			"time": time.Now().Unix(),
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nuevoAgente)
	})

	mux.HandleFunc("/api/eliminar-agente", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido: "+err.Error(), 400)
			return
		}
		if req.ID == "" {
			http.Error(w, "ID de agente requerido", 400)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		if _, exists := n.agentes[req.ID]; !exists {
			http.Error(w, "Agente no encontrado", 404)
			return
		}
		delete(n.agentes, req.ID)
		n.Auditoria("AGENTE_ELIMINADO", fmt.Sprintf("ID: %s", req.ID))
		dAg, _ := json.MarshalIndent(n.agentes, "", "  ")
		if err := os.WriteFile("alset_state.json", dAg, 0644); err != nil {
			fmt.Printf("⚠️ Error guardando estado: %v\n", err)
		}
		go n.SincronizarConPares()
		go n.broadcastPulse("agent_deleted", map[string]interface{}{
			"id":   req.ID,
			"time": time.Now().Unix(),
		})
		fmt.Printf("🗑️ Agente eliminado: %s\n", req.ID)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "deleted",
			"id":      req.ID,
			"message": "Agente eliminado correctamente",
		})
	})

	mux.HandleFunc("/api/modificar-agente", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "PUT" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			ID          string  `json:"id"`
			BalanceUTXO float64 `json:"balance_utxo"`
			RootCID     string  `json:"root_cid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if req.ID == "" {
			http.Error(w, "ID de agente requerido", 400)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		if agent, exists := n.agentes[req.ID]; exists {
			if req.BalanceUTXO != 0 {
				agent.BalanceUTXO = req.BalanceUTXO
			}
			if req.RootCID != "" {
				agent.RootCID = req.RootCID
			}
			agent.UltimaActual = time.Now().Unix()
			n.Auditoria("AGENTE_MODIFICADO", fmt.Sprintf("ID: %s | Balance: %.2f | RootCID: %s", req.ID, req.BalanceUTXO, req.RootCID))
			n.PersistirLocamente()
			go n.broadcastPulse("agent_updated", map[string]interface{}{
				"id":      req.ID,
				"balance": req.BalanceUTXO,
				"root":    req.RootCID,
				"time":    time.Now().Unix(),
			})
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "updated",
				"agent":  agent,
			})
		} else {
			http.Error(w, "Agente no encontrado", 404)
		}
	})

	mux.HandleFunc("/api/debug/estado", n.handleDebugEstado)
	mux.HandleFunc("/api/prism/verificar", n.handlePrismVerificar)
	mux.HandleFunc("/api/prism/revocar", n.handlePrismRevocar)
	mux.HandleFunc("/api/prism/sellar", n.handlePrismSellar)
	mux.HandleFunc("/api/admin/update-pass", n.handleAdminUpdatePass)
	mux.HandleFunc("/api/admin/login", n.handleAdminLogin)
	mux.HandleFunc("/api/ipfs/add", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cidStr, _ := n.GenerarCID(body)
		n.AnunciarNuevoBloque(cidStr)
		json.NewEncoder(w).Encode(map[string]string{"cid": cidStr})
	})

	mux.HandleFunc("/api/ipfs/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			CID string `json:"cid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if req.CID == "" {
			http.Error(w, "CID requerido", 400)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		if _, exists := n.blockstore[req.CID]; exists {
			delete(n.blockstore, req.CID)
			diskPath := filepath.Join(BlocksDir, req.CID)
			if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
				fmt.Printf("⚠️ No se pudo eliminar archivo de disco: %v\n", err)
			}
			n.Auditoria("IPFS_BLOQUE_ELIMINADO", fmt.Sprintf("CID: %s", req.CID))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "deleted",
				"cid":    req.CID,
			})
		} else {
			http.Error(w, "Bloque no encontrado", 404)
		}
	})

	mux.HandleFunc("/api/ipfs/get", func(w http.ResponseWriter, r *http.Request) {
		data, err := n.BuscarContenidoPorCID(r.URL.Query().Get("cid"))
		if err != nil {
			http.Error(w, "Not found", 404)
			return
		}
		w.Write(data)
	})

	mux.HandleFunc("/api/ipfs/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			Confirm string `json:"confirm"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Confirm != "YES" {
			http.Error(w, "Confirmación requerida: 'confirm': 'YES'", 400)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		count := len(n.blockstore)
		n.blockstore = make(map[string][]byte)
		if err := os.RemoveAll(BlocksDir); err != nil {
			fmt.Printf("⚠️ Error limpiando directorio: %v\n", err)
		}
		os.MkdirAll(BlocksDir, 0755)
		n.Auditoria("IPFS_BLOCKSTORE_LIMPIADA", fmt.Sprintf("Bloques eliminados: %d", count))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "cleared",
			"blocks_deleted": count,
		})
	})

	mux.HandleFunc("/api/apps/register", n.handleAppsRegister)
	mux.HandleFunc("/api/apps/list", n.handleAppsList)
	mux.HandleFunc("/api/lispai", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Cmd string `json:"cmd"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		res, err := n.lisp.Eval(req.Cmd)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"resultado": res})
	})

	mux.HandleFunc("/api/poh/event", n.handlePoHEvent)
	mux.HandleFunc("/api/poh/proof", n.handlePoHProof)
	mux.HandleFunc("/api/sync/status", n.handleSyncStatus)
	mux.HandleFunc("/api/sync/full", n.handleSyncFull)
	mux.HandleFunc("/api/sync/quick", n.handleSyncQuick)
	mux.HandleFunc("/api/sync/config", n.handleSyncConfig)

	// IA endpoints
	mux.HandleFunc("/api/ia/configurar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			NeuronType     string  `json:"neuron_type"`
			SpikeThreshold float64 `json:"spike_threshold"`
			LeakRate       float64 `json:"leak_rate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		done := make(chan bool, 1)
		go func() {
			n.mu.Lock()
			defer n.mu.Unlock()
			if n.neuralState == nil {
				n.neuralState = &NeuralState{
					MembranePotential: 0,
					SpikeThreshold:    0.6,
					LeakRate:          0.01,
					NeuronType:        "hidden",
					Synapses:          make(map[string]SynapticWeight),
				}
			}
			if req.NeuronType != "" {
				n.neuralState.NeuronType = req.NeuronType
			}
			if req.SpikeThreshold > 0 && req.SpikeThreshold <= 1 {
				n.neuralState.SpikeThreshold = req.SpikeThreshold
			}
			if req.LeakRate > 0 && req.LeakRate <= 1 {
				n.neuralState.LeakRate = req.LeakRate
			}
			done <- true
		}()
		select {
		case <-done:
			go n.persistirEstadoNeuronal()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "configured",
				"config": n.neuralState,
			})
		case <-time.After(5 * time.Second):
			http.Error(w, "Timeout", 500)
		}
	})

	mux.HandleFunc("/api/ia/inferir", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			Entrada []float64 `json:"entrada"`
			Timeout int       `json:"timeout"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if len(req.Entrada) == 0 {
			req.Entrada = []float64{0}
		}
		var output float64 = 0
		for _, val := range req.Entrada {
			output += val
		}
		if len(req.Entrada) > 0 {
			output = output / float64(len(req.Entrada))
		}
		output = 1.0 / (1.0 + math.Exp(-output))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "success",
			"output":       []float64{output},
			"processed_by": n.host.ID().String(),
			"process_time": time.Now().UnixNano(),
		})
		go func() {
			if n.topic != nil {
				requestID := generarUUID()
				inferenceReq := InferenceRequest{
					RequestID:    requestID,
					InputData:    req.Entrada,
					OriginNodeID: n.host.ID().String(),
					TTL:          3,
				}
				data, _ := json.Marshal(inferenceReq)
				update := map[string]string{
					"tipo": "inference_request",
					"data": string(data),
				}
				msgData, _ := json.Marshal(update)
				n.topic.Publish(n.ctx, msgData)
			}
		}()
	})

	mux.HandleFunc("/api/ia/aprender", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			Entrada         []float64 `json:"entrada"`
			SalidaEsperada  []float64 `json:"salida_esperada"`
			TasaAprendizaje float64   `json:"tasa_aprendizaje"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		tasa := req.TasaAprendizaje
		if tasa <= 0 {
			tasa = 0.01
		}
		n.mu.Lock()
		if n.neuralState != nil {
			for target, syn := range n.neuralState.Synapses {
				newWeight := syn.Weight + tasa*(1-syn.Weight)
				if newWeight > 1 {
					newWeight = 1
				}
				syn.Weight = newWeight
				syn.SuccessfulFires++
				n.neuralState.Synapses[target] = syn
			}
		}
		n.mu.Unlock()
		go n.persistirEstadoNeuronal()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "learning_completed",
			"tasa":   tasa,
		})
	})

	mux.HandleFunc("/api/ia/memoria/buscar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			Consulta string `json:"consulta"`
			Limit    int    `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if req.Limit <= 0 {
			req.Limit = 10
		}
		resultados := []map[string]interface{}{}
		n.mu.RLock()
		for cid, data := range n.blockstore {
			if strings.Contains(string(data), req.Consulta) {
				resultados = append(resultados, map[string]interface{}{
					"cid":  cid,
					"data": string(data),
				})
				if len(resultados) >= req.Limit {
					break
				}
			}
		}
		n.mu.RUnlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"query":   req.Consulta,
			"results": resultados,
			"count":   len(resultados),
		})
	})

	mux.HandleFunc("/api/ia/topologia", func(w http.ResponseWriter, r *http.Request) {
		peers := n.host.Network().Peers()
		vecinosInfo := []map[string]interface{}{}
		n.mu.RLock()
		neuronType := "hidden"
		synapsesCount := 0
		if n.neuralState != nil {
			neuronType = n.neuralState.NeuronType
			synapsesCount = len(n.neuralState.Synapses)
		}
		for _, p := range peers {
			weight := 0.0
			fires := int64(0)
			tieneSinapsis := false
			if n.neuralState != nil {
				if s, ok := n.neuralState.Synapses[p.String()]; ok {
					weight = s.Weight
					fires = s.SuccessfulFires
					tieneSinapsis = true
				}
			}
			vecinosInfo = append(vecinosInfo, map[string]interface{}{
				"peer_id":   p.String(),
				"connected": n.host.Network().Connectedness(p).String(),
				"synaptic_weight": map[string]interface{}{
					"exists": tieneSinapsis,
					"weight": weight,
					"fires":  fires,
				},
			})
		}
		n.mu.RUnlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"neuron_id":   n.host.ID().String(),
			"neuron_type": neuronType,
			"peers_count": len(peers),
			"synapses":    synapsesCount,
			"peers":       vecinosInfo,
		})
	})

	mux.HandleFunc("/api/ia/estado", func(w http.ResponseWriter, r *http.Request) {
		n.mu.RLock()
		defer n.mu.RUnlock()
		if n.neuralState == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "not_initialized",
			})
			return
		}
		sinapsisList := []map[string]interface{}{}
		for target, s := range n.neuralState.Synapses {
			sinapsisList = append(sinapsisList, map[string]interface{}{
				"target":           target,
				"weight":           s.Weight,
				"successful_fires": s.SuccessfulFires,
				"last_updated":     s.LastUpdated,
			})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":             "active",
			"neuron_type":        n.neuralState.NeuronType,
			"membrane_potential": n.neuralState.MembranePotential,
			"spike_threshold":    n.neuralState.SpikeThreshold,
			"leak_rate":          n.neuralState.LeakRate,
			"last_spike_time":    n.neuralState.LastSpikeTime,
			"synapses_count":     len(n.neuralState.Synapses),
			"synapses":           sinapsisList,
		})
	})

	mux.HandleFunc("/api/ia/metricas", func(w http.ResponseWriter, r *http.Request) {
		n.mu.RLock()
		defer n.mu.RUnlock()
		totalSpikes := int64(0)
		pesoPromedio := 0.0
		synapseCount := 0
		if n.neuralState != nil {
			synapseCount = len(n.neuralState.Synapses)
			for _, s := range n.neuralState.Synapses {
				totalSpikes += s.SuccessfulFires
				pesoPromedio += s.Weight
			}
			if synapseCount > 0 {
				pesoPromedio /= float64(synapseCount)
			}
		}
		membrane := 0.0
		if n.neuralState != nil {
			membrane = n.neuralState.MembranePotential
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_synaptic_connections": synapseCount,
			"average_synaptic_weight":    pesoPromedio,
			"total_successful_spikes":    totalSpikes,
			"current_membrane_potential": membrane,
			"uptime":                     time.Now().Unix() - n.startTime,
		})
	})

	mux.HandleFunc("/api/ia/sinapsis/conectar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		var req struct {
			NodoDestino string  `json:"nodo_destino"`
			PesoInicial float64 `json:"peso_inicial"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if req.NodoDestino == "" {
			http.Error(w, "nodo_destino requerido", 400)
			return
		}
		if req.PesoInicial <= 0 {
			req.PesoInicial = 0.5
		}
		if req.PesoInicial > 1 {
			req.PesoInicial = 1
		}
		done := make(chan bool, 1)
		go func() {
			n.mu.Lock()
			defer n.mu.Unlock()
			if n.neuralState == nil {
				n.neuralState = &NeuralState{
					MembranePotential: 0,
					SpikeThreshold:    0.6,
					LeakRate:          0.01,
					NeuronType:        "hidden",
					Synapses:          make(map[string]SynapticWeight),
				}
			}
			n.neuralState.Synapses[req.NodoDestino] = SynapticWeight{
				TargetNeuronID: req.NodoDestino,
				Weight:         req.PesoInicial,
				LastUpdated:    time.Now().Unix(),
			}
			done <- true
		}()
		select {
		case <-done:
			go n.persistirEstadoNeuronal()
			go func() {
				update := map[string]string{
					"tipo":          "synaptic_update",
					"neuronas_pre":  n.host.ID().String(),
					"neuronas_post": req.NodoDestino,
					"exito":         "true",
					"peso":          fmt.Sprintf("%f", req.PesoInicial),
				}
				if data, err := json.Marshal(update); err == nil && n.topic != nil {
					n.topic.Publish(n.ctx, data)
				}
			}()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "connected",
				"target": req.NodoDestino,
				"weight": req.PesoInicial,
			})
		case <-time.After(5 * time.Second):
			http.Error(w, "Timeout", 500)
		}
	})

	mux.HandleFunc("/api/ia/sinapsis/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método no permitido", 405)
			return
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		if n.neuralState != nil {
			oldCount := len(n.neuralState.Synapses)
			n.neuralState.Synapses = make(map[string]SynapticWeight)
			n.persistirEstadoNeuronal()
			n.Auditoria("SINAPSIS_LIMPIADAS", fmt.Sprintf("Sinapsis eliminadas: %d", oldCount))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":           "cleared",
				"synapses_removed": oldCount,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":           "already_empty",
				"synapses_removed": 0,
			})
		}
	})

	// Módulos, entidades y seguridad endpoints
	mux.HandleFunc("/api/modulos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarModulos(w, r)
		case "POST":
			n.crearModulo(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})
	mux.HandleFunc("/api/modulos/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.obtenerModulo(w, r)
		case "PUT":
			n.actualizarModulo(w, r)
		case "DELETE":
			n.eliminarModulo(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})

	mux.HandleFunc("/api/entidades", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarEntidades(w, r)
		case "POST":
			n.crearEntidad(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})
	mux.HandleFunc("/api/entidades/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/relaciones") {
			n.obtenerRelacionesDeEntidad(w, r)
			return
		}
		switch r.Method {
		case "GET":
			n.obtenerEntidad(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})

	mux.HandleFunc("/api/relaciones", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.listarRelaciones(w, r)
		case "POST":
			n.crearRelacion(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})

	mux.HandleFunc("/api/auth/token", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.crearTokenEndpoint(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})
	mux.HandleFunc("/api/auth/validate", n.validarTokenEndpoint)
	mux.HandleFunc("/api/auth/revoke", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.revocarTokenEndpoint(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})

	mux.HandleFunc("/api/roles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			n.asignarRol(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})
	mux.HandleFunc("/api/roles/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n.obtenerRoles(w, r)
		default:
			http.Error(w, "Método no permitido", 405)
		}
	})

	fmt.Printf("🚀 Prisma Tec API activa en puerto %s (incluye /api/pulse)\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// =============================================================================
// INICIALIZACIÓN DEL NODO (MODIFICADA)
// =============================================================================

func (n *NodoAlset) Init() {
	n.LoadMasterKey()
	n.startTime = time.Now().Unix()
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		log.Fatal("Error generando clave privada:", err)
	}
	// Habilitar relay y usar puerto fijo opcional
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableRelayService(),
	)
	if err != nil {
		log.Fatal("Error creando el host libp2p:", err)
	}
	n.host = h
	n.ctx = context.Background()
	n.blockstore = make(map[string][]byte)
	n.agentes = make(map[string]*Agente)
	n.nombres = make(map[string]string)
	n.pendingInferences = make(map[string]chan InferenceResponse)
	n.pendingMemoryQueries = make(map[string]chan MemoryResponse)
	n.hebbianMemory = make(map[string]float64)

	// Inicializar pulse
	n.pulseSubscribers = make(map[*SSESubscriber]bool)
	n.pulseClients = make(map[string]*PulseClient)

	n.syncManager = n.InitSyncManager()

	n.CargarEstado()
	n.neuralState = &NeuralState{
		MembranePotential: 0,
		LastSpikeTime:     0,
		SpikeThreshold:    0.6,
		LeakRate:          0.01,
		RefractoryPeriod:  1000000,
		Synapses:          make(map[string]SynapticWeight),
		NeuronType:        "input",
	}
	n.cargarPesosSinapsis()
	n.datastore = ds_sync.MutexWrap(datastore.NewMapDatastore())
	ps, err := pubsub.NewGossipSub(n.ctx, n.host)
	if err != nil {
		log.Fatal("Error creando GossipSub:", err)
	}
	n.pubsub = ps
	n.topic, err = n.pubsub.Join(AlsetGossipTopic)
	if err != nil {
		log.Fatal("Error uniéndose al tópico:", err)
	}
	n.host.SetStreamHandler(AlsetDataExchangeID, n.handleDataExchange)
	n.kademlia, err = dht.New(n.ctx, n.host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal("Error creando DHT:", err)
	}
	go n.kademlia.Bootstrap(n.ctx)
	n.lisp = NewLispEvaluator(n)
	mdns.NewMdnsService(n.host, "alset-mesh", &discoveryNotifee{h: n.host}).Start()
	go n.EscucharGossip()

	go n.QuickStartup()
}

type discoveryNotifee struct{ h host.Host }

func (d *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	d.h.Connect(context.Background(), pi)
}

// =============================================================================
// MAIN
// =============================================================================

func main() {
	fmt.Println("🌐 PRISM@.TEC ALSET NET (P.TEC-AN) v4.0")
	fmt.Println("📦 Sistema Híbrido Go + Lisp con IA Distribuida, VC, UTXO, PoH y ZKP")
	fmt.Println("🧠 Con IA Distribuida: Neuronas, Sinapsis, Inferencia Distribuida y Memoria Distribuida")
	fmt.Println("⚡ Con sistema de pulsos SSE para comunicación resiliente")
	if os.Getenv("RENDER") != "" {
        fmt.Println("🟢 Nodo ejecutándose en Render (servidor de pulsos)")
    } else {
        fmt.Println("🟢 Nodo ejecutándose localmente (cliente de pulsos)")
    }
	
	port := "8080"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	nodo := &NodoAlset{
		ctx:                  context.Background(),
		agentes:              make(map[string]*Agente),
		pendingInferences:    make(map[string]chan InferenceResponse),
		pendingMemoryQueries: make(map[string]chan MemoryResponse),
		hebbianMemory:        make(map[string]float64),
	}

	mathrand.Seed(time.Now().UnixNano())
	nodo.Init()
	nodo.Auditoria("SISTEMA_START", fmt.Sprintf("Nodo Online en puerto %s", port))
	go nodo.startHTTPServer(port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	nodo.Auditoria("SISTEMA_STOP", "Apagado del nodo")
	nodo.PersistirLocamente()
	fmt.Println("👋 Nodo apagado correctamente")
}
