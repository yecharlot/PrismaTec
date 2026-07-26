package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
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
// GITHUB PERSISTENCE
// =============================================================================

type GitHubPersistence struct {
	Owner     string
	Repo      string
	Path      string
	Token     string
	Branch    string
	commitSHA string
	mu        sync.Mutex
}

func NewGitHubPersistence(owner, repo, path, token string) *GitHubPersistence {
	return &GitHubPersistence{
		Owner:  owner,
		Repo:   repo,
		Path:   path,
		Token:  token,
		Branch: "main",
	}
}

func (g *GitHubPersistence) Save(data []byte, filename string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	content := base64.StdEncoding.EncodeToString(data)
	existingSHA, _ := g.getFileSHA(filename)

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/%s", g.Owner, g.Repo, g.Path, filename)

	payload := map[string]interface{}{
		"message": fmt.Sprintf("Update %s at %s", filename, time.Now().Format(time.RFC3339)),
		"content": content,
		"branch":  g.Branch,
	}

	if existingSHA != "" {
		payload["sha"] = existingSHA
	}

	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if content, ok := result["content"].(map[string]interface{}); ok {
		if sha, ok := content["sha"].(string); ok {
			g.commitSHA = sha
			return sha, nil
		}
	}

	return "", nil
}

func (g *GitHubPersistence) Load(filename string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/%s", g.Owner, g.Repo, g.Path, filename)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if content, ok := result["content"].(string); ok {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, err
		}
		return decoded, nil
	}

	return nil, fmt.Errorf("no content found")
}

func (g *GitHubPersistence) getFileSHA(filename string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/%s", g.Owner, g.Repo, g.Path, filename)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", nil
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("error getting file SHA")
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if sha, ok := result["sha"].(string); ok {
		return sha, nil
	}

	return "", nil
}

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
}

type BlockInfo struct {
	CID     string `json:"cid"`
	Size    int    `json:"size"`
	Preview string `json:"preview"`
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

// eval es el método privado de evaluación
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

// =============================================================================
// FUNCIONES PRIMITIVAS DEL LISP (RESUMIDAS PARA RENDER)
// =============================================================================

func (e *LispEvaluator) initBuiltins() {
	// === OPERADORES ARITMÉTICOS Y LÓGICOS ===
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

	// === PRIMITIVAS DE LISTAS ===
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

	// === FUNCIONES DE AGENTES ===
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

	// === FUNCIONES DE SISTEMA ===
	e.globalEnv.SetFunction("current-time", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		return float64(time.Now().Unix())
	}))

	e.globalEnv.SetFunction("println", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		for _, arg := range args {
			fmt.Printf("%v ", e.eval(arg, env))
		}
		fmt.Println()
		return nil
	}))

	// === FUNCIONES DE CONTROL ===
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
		env.SetFunction(funcName, userFunc)
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
}

// =============================================================================
// FUNCIONES AUXILIARES
// =============================================================================

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

// =============================================================================
// SSESubscriber Y PulseClient
// =============================================================================

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
	github               *GitHubPersistence
	pulseSubscribers     map[*SSESubscriber]bool
	pulseSubscribersMu   sync.RWMutex
	pulseClients         map[string]*PulseClient
	pulseClientsMu       sync.RWMutex
}

// =============================================================================
// MÉTODOS DEL NODO
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

	// Persistencia local
	dAg, _ := json.MarshalIndent(n.agentes, "", "  ")
	_ = os.WriteFile("alset_state.json", dAg, 0644)

	dAn, _ := json.MarshalIndent(n.nombres, "", "  ")
	_ = os.WriteFile("alset_names.json", dAn, 0644)

	n.persistirEstadoNeuronal()

	// Persistencia en GitHub (si está configurado)
	if n.github != nil {
		go func() {
			if err := n.PersistirEnGitHub(); err != nil {
				log.Printf("⚠️ Error guardando en GitHub: %v", err)
			}
		}()
	}
}

func (n *NodoAlset) PersistirEnGitHub() error {
	if n.github == nil {
		return fmt.Errorf("GitHub persistence not configured")
	}

	// Guardar agentes
	agentesData, _ := json.MarshalIndent(n.agentes, "", "  ")
	if _, err := n.github.Save(agentesData, "alset_state.json"); err != nil {
		return fmt.Errorf("error saving agents: %v", err)
	}

	// Guardar nombres DNS
	nombresData, _ := json.MarshalIndent(n.nombres, "", "  ")
	if _, err := n.github.Save(nombresData, "alset_names.json"); err != nil {
		return fmt.Errorf("error saving names: %v", err)
	}

	// Guardar estado neuronal
	if n.neuralState != nil {
		neuralData, _ := json.MarshalIndent(n.neuralState, "", "  ")
		if _, err := n.github.Save(neuralData, "neural_state.json"); err != nil {
			return fmt.Errorf("error saving neural state: %v", err)
		}
	}

	// Guardar blocks
	blocksData, _ := json.Marshal(n.blockstore)
	if _, err := n.github.Save(blocksData, "blocks.json"); err != nil {
		return fmt.Errorf("error saving blocks: %v", err)
	}

	n.Auditoria("PERSISTENCIA_GITHUB", "Estado guardado exitosamente")
	return nil
}

func (n *NodoAlset) CargarDesdeGitHub() error {
	if n.github == nil {
		return fmt.Errorf("GitHub persistence not configured")
	}

	// Cargar agentes
	if data, err := n.github.Load("alset_state.json"); err == nil && data != nil {
		var agentes map[string]*Agente
		if err := json.Unmarshal(data, &agentes); err == nil {
			n.mu.Lock()
			for k, v := range agentes {
				n.agentes[k] = v
			}
			n.mu.Unlock()
		}
	}

	// Cargar nombres DNS
	if data, err := n.github.Load("alset_names.json"); err == nil && data != nil {
		var nombres map[string]string
		if err := json.Unmarshal(data, &nombres); err == nil {
			n.mu.Lock()
			for k, v := range nombres {
				n.nombres[k] = v
			}
			n.mu.Unlock()
		}
	}

	// Cargar estado neuronal
	if data, err := n.github.Load("neural_state.json"); err == nil && data != nil {
		var neural NeuralState
		if err := json.Unmarshal(data, &neural); err == nil {
			n.mu.Lock()
			n.neuralState = &neural
			n.mu.Unlock()
		}
	}

	// Cargar blocks
	if data, err := n.github.Load("blocks.json"); err == nil && data != nil {
		var blocks map[string][]byte
		if err := json.Unmarshal(data, &blocks); err == nil {
			n.mu.Lock()
			for k, v := range blocks {
				n.blockstore[k] = v
				os.WriteFile(filepath.Join(BlocksDir, k), v, 0644)
			}
			n.mu.Unlock()
		}
	}

	n.Auditoria("PERSISTENCIA_GITHUB", "Estado cargado exitosamente")
	return nil
}

func (n *NodoAlset) CargarEstado() {
	// Primero cargar desde GitHub si está configurado
	if n.github != nil {
		if err := n.CargarDesdeGitHub(); err == nil {
			fmt.Println("📂 Estado cargado desde GitHub")
			return
		}
	}

	// Fallback a archivos locales
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

func (n *NodoAlset) SetAgentRoot(agentID string, rootCID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if a, ok := n.agentes[agentID]; ok {
		a.RootCID = rootCID
		a.UltimaActual = time.Now().Unix()
	}
}

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
// MÉTODOS DE IA DISTRIBUIDA
// =============================================================================func (n *NodoAlset) puedeProcesarInferencia(input []float64) bool {
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
// NETWORKING & GOSSIP SYNC
// =============================================================================

func (n *NodoAlset) AnunciarNuevoBloque(cidStr string) {
	// 1. Publicar en gossip
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
		b64 := base64.StdEncoding.EncodeToString(blockData)
		n.broadcastPulse("new_block", map[string]interface{}{
			"cid":  cidStr,
			"data": b64,
		})
		log.Printf("📤 Bloque %s emitido por pulso (%d bytes)", cidStr, len(blockData))
	} else {
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
		cidReq := scanner.Text()
		n.mu.RLock()
		data, ok := n.blockstore[cidReq]
		n.mu.RUnlock()
		if ok {
			s.Write(data)
		}
	}
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
// SISTEMA DE PULSOS (SSE)
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
		}
	}
}

func (n *NodoAlset) startPulseClients() {
	// En Render no actuamos como cliente, solo como servidor
	if os.Getenv("RENDER") != "" {
		return
	}
	// Para nodos locales (no usamos en Render)
}

func (n *NodoAlset) processPulseEvent(eventType string, data string) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		log.Printf("⚠️ Error parseando evento %s: %v", eventType, err)
		return
	}

	switch eventType {
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
				BalanceUTXO:  1000.0,
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

	case "new_block":
		cid, _ := payload["cid"].(string)
		if cid == "" {
			return
		}
		n.mu.RLock()
		_, exists := n.blockstore[cid]
		n.mu.RUnlock()
		if exists {
			return
		}
		dataB64, _ := payload["data"].(string)
		if dataB64 != "" {
			blockData, err := base64.StdEncoding.DecodeString(dataB64)
			if err == nil {
				n.mu.Lock()
				n.blockstore[cid] = blockData
				n.mu.Unlock()
				os.WriteFile(filepath.Join(BlocksDir, cid), blockData, 0644)
				log.Printf("📦 Bloque %s recibido por pulso (%d bytes)", cid, len(blockData))
				return
			}
		}
		go n.SolicitarBloqueAPar(cid, peer.ID(""))

	case "synaptic_update":
		go n.actualizarPesosSinapsis(convertMapToStringMap(payload), peer.ID("pulse"))

	case "neural_spike":
		go n.procesarSpikeNeuronal(convertMapToStringMap(payload), peer.ID("pulse"))

	case "neural_state_sync":
		go n.sincronizarEstadoNeuronal(convertMapToStringMap(payload), peer.ID("pulse"))

	case "memory_distributed":
		go n.manejarMemoriaDistribuida(convertMapToStringMap(payload), peer.ID("pulse"))

	case "inference_response":
		respData, _ := payload["data"].(string)
		if respData != "" {
			go n.procesarRespuestaInferencia(map[string]string{"data": respData})
		}

	case "force_sync":
		if n.github != nil {
			go func() {
				if err := n.CargarDesdeGitHub(); err != nil {
					log.Printf("⚠️ Error en force sync desde GitHub: %v", err)
				} else {
					log.Printf("✅ Force sync completado desde GitHub")
				}
			}()
		}
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
// HANDLERS HTTP
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "verificado",
		"cid":    certCID,
		"data":   vc,
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

func (n *NodoAlset) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "authorized", "node": n.host.ID().String()})
}

// =============================================================================
// SYNC MANAGER
// =============================================================================

type SyncManager struct {
	nodo         *NodoAlset
	config       SyncConfig
	isSyncing    bool
	syncProgress float64
	syncCancel   context.CancelFunc
	mu           sync.RWMutex
}

func (n *NodoAlset) InitSyncManager() *SyncManager {
	config := SyncConfig{
		Mode:           SyncModeQuick,
		AutoSyncDays:   7,
		MaxQuickBlocks: 100,
	}
	if data, err := os.ReadFile("sync_config.json"); err == nil {
		json.Unmarshal(data, &config)
	}
	sm := &SyncManager{
		nodo:   n,
		config: config,
	}
	n.syncManager = sm
	return sm
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
		// Intentar cargar desde GitHub
		if sm.nodo.github != nil {
			sm.nodo.CargarDesdeGitHub()
		}
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
		sm.config.LastSyncTime = time.Now().Unix()
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
		// Intentar cargar desde GitHub
		if sm.nodo.github != nil {
			sm.nodo.CargarDesdeGitHub()
			if progressCallback != nil {
				progressCallback(1.0)
			}
			return nil
		}
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

func (sm *SyncManager) SaveLastSyncTime() {
	data, _ := json.Marshal(map[string]int64{"timestamp": time.Now().Unix()})
	os.WriteFile("last_sync.json", data, 0644)
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

// =============================================================================
// INICIALIZACIÓN DEL NODO
// =============================================================================

func (n *NodoAlset) Init() {
	n.LoadMasterKey()
	n.startTime = time.Now().Unix()
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		log.Fatal("Error generando clave privada:", err)
	}
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
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
	n.pulseSubscribers = make(map[*SSESubscriber]bool)
	n.pulseClients = make(map[string]*PulseClient)

	// Inicializar sync manager
	n.syncManager = n.InitSyncManager()

	// Cargar estado
	n.CargarEstado()

	// Inicializar estado neuronal
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

	// Configurar datastore y pubsub
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

	// Configurar handlers
	n.host.SetStreamHandler(AlsetDataExchangeID, n.handleDataExchange)

	// Inicializar DHT
	n.kademlia, err = dht.New(n.ctx, n.host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal("Error creando DHT:", err)
	}
	go n.kademlia.Bootstrap(n.ctx)

	// Inicializar Lisp
	n.lisp = NewLispEvaluator(n)

	// Descubrimiento mDNS
	mdns.NewMdnsService(n.host, "alset-mesh", &discoveryNotifee{h: n.host}).Start()

	// Iniciar gossip
	go n.EscucharGossip()

	// Sincronización inicial
	go func() {
		time.Sleep(3 * time.Second)
		if n.shouldQuickSync() {
			n.syncManager.PerformQuickSync()
		}
	}()

	// Sincronización periódica con GitHub (cada 30 segundos)
	if n.github != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			for range ticker.C {
				if err := n.CargarDesdeGitHub(); err != nil {
					log.Printf("⚠️ Error cargando desde GitHub: %v", err)
				}
				if err := n.PersistirEnGitHub(); err != nil {
					log.Printf("⚠️ Error guardando en GitHub: %v", err)
				}
			}
		}()
		fmt.Println("✅ Sincronización automática con GitHub iniciada (cada 30s)")
	}

	fmt.Println("✅ Nodo operativo (sincronización en background)")
}

type discoveryNotifee struct{ h host.Host }

func (d *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	d.h.Connect(context.Background(), pi)
}

// =============================================================================
// SERVIDOR HTTP
// =============================================================================

func (n *NodoAlset) startHTTPServer(port string) {
	mux := http.NewServeMux()

	// Static files
	os.MkdirAll(StaticDir, 0755)
	os.MkdirAll(filepath.Join(StaticDir, "apps"), 0755)

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(StaticDir))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/static/index.html", http.StatusFound)
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	})

	// Apps
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

	// === PULSO SSE ===
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

	// === PULSO EMIT ===
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

		var payload map[string]interface{}
		if err := json.Unmarshal(req.Data, &payload); err != nil {
			http.Error(w, "Invalid data", 400)
			return
		}

		go n.broadcastPulse(req.EventType, payload)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "emitted"})
	})

	// === API ENDPOINTS ===
	mux.HandleFunc("/api/sync/status", n.handleSyncStatus)
	mux.HandleFunc("/api/sync/full", n.handleSyncFull)
	mux.HandleFunc("/api/sync/quick", n.handleSyncQuick)
	mux.HandleFunc("/api/sync/force", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if n.github != nil {
			if err := n.CargarDesdeGitHub(); err != nil {
				http.Error(w, "Error loading from GitHub: "+err.Error(), 500)
				return
			}
		}
		n.broadcastPulse("force_sync", map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"node_id":   n.host.ID().String(),
		})
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "sync_forced",
			"message": "Sincronización forzada completada",
		})
	})

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

	mux.HandleFunc("/api/dns/list", n.handleDNSList)
	mux.HandleFunc("/api/dns/resolve", n.handleDNSResolve)

	mux.HandleFunc("/api/network/peers", n.handleNetworkPeers)
	mux.HandleFunc("/api/audit/log", n.handleAuditLog)
	mux.HandleFunc("/api/debug/estado", n.handleDebugEstado)

	mux.HandleFunc("/api/apps/register", n.handleAppsRegister)
	mux.HandleFunc("/api/apps/list", n.handleAppsList)

	mux.HandleFunc("/api/prism/verificar", n.handlePrismVerificar)
	mux.HandleFunc("/api/prism/sellar", n.handlePrismSellar)

	mux.HandleFunc("/api/admin/login", n.handleAdminLogin)

	// IPFS endpoints
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

	mux.HandleFunc("/api/ipfs/add", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cidStr, _ := n.GenerarCID(body)
		n.AnunciarNuevoBloque(cidStr)
		json.NewEncoder(w).Encode(map[string]string{"cid": cidStr})
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

	// Lisp AI
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

	// PoH
	mux.HandleFunc("/api/poh/event", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	fmt.Printf("🚀 API activa en puerto %s (incluye /api/pulse)\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// =============================================================================
// MAIN
// =============================================================================

func main() {
	fmt.Println("🌐 PRISM@.TEC ALSET NET (P.TEC-AN) v4.0 - RENDER NODE")
	fmt.Println("📦 Sistema con persistencia en GitHub")
	fmt.Println("⚡ Con sistema de pulsos SSE para comunicación")

	if os.Getenv("RENDER") != "" {
		fmt.Println("🟢 Nodo ejecutándose en Render (servidor de pulsos)")
	} else {
		fmt.Println("🟢 Nodo ejecutándose localmente")
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

	// Configurar GitHub Persistence
	githubToken := os.Getenv("GITHUB_TOKEN")
	githubOwner := os.Getenv("GITHUB_OWNER")
	githubRepo := os.Getenv("GITHUB_REPO")

	if githubToken != "" && githubOwner != "" && githubRepo != "" {
		nodo.github = NewGitHubPersistence(githubOwner, githubRepo, "alset_data", githubToken)
		fmt.Println("✅ GitHub persistence configurado correctamente")
		fmt.Printf("   Repo: %s/%s\n", githubOwner, githubRepo)
	} else {
		fmt.Println("⚠️ GitHub persistence NO configurado (faltan variables de entorno)")
		fmt.Println("   GITHUB_TOKEN, GITHUB_OWNER, GITHUB_REPO")
	}

	mathrand.Seed(time.Now().UnixNano())
	nodo.Init()

	// Cargar estado inicial desde GitHub si está configurado
	if nodo.github != nil {
		if err := nodo.CargarDesdeGitHub(); err != nil {
			fmt.Printf("⚠️ Error cargando estado inicial desde GitHub: %v\n", err)
		} else {
			fmt.Println("✅ Estado inicial cargado desde GitHub")
		}
	}

	nodo.Auditoria("SISTEMA_START", fmt.Sprintf("Nodo Online en puerto %s", port))
	go nodo.startHTTPServer(port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	fmt.Println("👋 Apagando nodo...")
	nodo.Auditoria("SISTEMA_STOP", "Apagado del nodo")
	nodo.PersistirLocamente()
	fmt.Println("👋 Nodo apagado correctamente")
}
