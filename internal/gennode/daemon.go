package gennode

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const PulseProtocol = "/alset/gen/pulse/1.0.0"

// Daemon is a minimal autonomous gen process (maceta propia).
type Daemon struct {
	Pkg         *FrontierPackage
	DataDir     string
	HTTPAddr    string
	EnableP2P   bool
	AnnounceURL string // PrismaTec base URL to register reachability for Mind
	PublicURL   string // how Mind should reach this daemon (e.g. https://x.ngrok.io)
	host        host.Host
	mu          sync.Mutex
	pulses      []map[string]interface{}
	findings    []map[string]interface{}
	startedAt   time.Time
	nameRecord  map[string]string // ANS key -> peer id / http
}

func (d *Daemon) Start(ctx context.Context) error {
	d.startedAt = time.Now().UTC()
	if d.DataDir == "" {
		d.DataDir = "gen_data"
	}
	_ = os.MkdirAll(d.DataDir, 0o755)
	d.nameRecord = map[string]string{}
	d.loadNameRecord()
	d.loadFindings()

	if d.EnableP2P {
		if err := d.startP2P(ctx); err != nil {
			log.Printf("⚠️ libp2p no arrancó (%v); sigo en modo HTTP-only", err)
		}
	}

	// Register local name → how to reach this gen
	reach := "http://" + d.publicHTTPHint()
	if d.host != nil {
		reach = d.host.ID().String()
	}
	d.nameRecord[d.Pkg.Key] = reach
	d.nameRecord[strings.TrimSuffix(d.Pkg.Key, ".ans")] = reach
	d.saveNameRecord()
	if d.AnnounceURL != "" {
		go d.announceLoop(ctx)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", d.handleRoot)
	mux.HandleFunc("/health", d.handleHealth)
	mux.HandleFunc("/api/info", d.handleInfo)
	mux.HandleFunc("/api/pulse", d.handlePulseHTTP)
	mux.HandleFunc("/api/resolve", d.handleResolve)
	mux.HandleFunc("/api/explore", d.handleExplore)
	mux.HandleFunc("/api/dialogue", d.handleDialogue)
	mux.HandleFunc("/api/findings", d.handleFindings)

	srv := &http.Server{Addr: d.HTTPAddr, Handler: mux}
	log.Printf("🧬 Alset-Gen daemon %s escuchando HTTP en %s", d.Pkg.Key, d.HTTPAddr)
	if d.host != nil {
		log.Printf("🔗 Peer ID %s", d.host.ID())
		for _, a := range d.host.Addrs() {
			log.Printf("   %s/p2p/%s", a, d.host.ID())
		}
	}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
		if d.host != nil {
			_ = d.host.Close()
		}
	}()
	return srv.ListenAndServe()
}

func (d *Daemon) publicHTTPHint() string {
	addr := d.HTTPAddr
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func (d *Daemon) startP2P(ctx context.Context) error {
	keyPath := filepath.Join(d.DataDir, "identity.key")
	priv, err := loadOrCreateKey(keyPath)
	if err != nil {
		return err
	}
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
	)
	if err != nil {
		return err
	}
	d.host = h
	h.SetStreamHandler(PulseProtocol, d.handlePulseStream)
	_ = ctx
	return nil
}

func loadOrCreateKey(path string) (crypto.PrivKey, error) {
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return crypto.UnmarshalPrivateKey(b)
	}
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	if err != nil {
		return nil, err
	}
	b, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	_ = os.WriteFile(path, b, 0o600)
	return priv, nil
}

func (d *Daemon) handlePulseStream(s network.Stream) {
	defer s.Close()
	b, err := io.ReadAll(io.LimitReader(s, 64*1024))
	if err != nil {
		return
	}
	var msg map[string]interface{}
	if json.Unmarshal(b, &msg) != nil {
		return
	}
	d.mu.Lock()
	d.pulses = append(d.pulses, msg)
	if len(d.pulses) > 64 {
		d.pulses = d.pulses[len(d.pulses)-64:]
	}
	d.mu.Unlock()
	resp, _ := json.Marshal(map[string]interface{}{
		"ok": true, "key": d.Pkg.Key, "echo": msg, "ts": time.Now().Unix(),
	})
	_, _ = s.Write(resp)
}

func (d *Daemon) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Alset-Gen", d.Pkg.Key)
	_, _ = w.Write([]byte(d.Pkg.ServicePage()))
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "key": d.Pkg.Key, "uptime_s": int(time.Since(d.startedAt).Seconds()),
	})
}

func (d *Daemon) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	info := map[string]interface{}{
		"ok":               true,
		"key":              d.Pkg.Key,
		"id":               d.Pkg.ID,
		"current_root_cid": d.Pkg.CurrentRootCID,
		"service_cid":      d.Pkg.ServiceCID,
		"mode":             "autonomous-daemon",
		"http":             d.HTTPAddr,
		"started_at":       d.startedAt.Format(time.RFC3339),
		"note":             "gen fuera del monolito PrismaTec; misma identidad ANS",
	}
	if d.host != nil {
		info["peer_id"] = d.host.ID().String()
		addrs := []string{}
		for _, a := range d.host.Addrs() {
			addrs = append(addrs, fmt.Sprintf("%s/p2p/%s", a, d.host.ID()))
		}
		info["addrs"] = addrs
	}
	d.mu.Lock()
	info["pulses_received"] = len(d.pulses)
	d.mu.Unlock()
	_ = json.NewEncoder(w).Encode(info)
}

func (d *Daemon) handlePulseHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var msg map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	d.mu.Lock()
	d.pulses = append(d.pulses, msg)
	if len(d.pulses) > 64 {
		d.pulses = d.pulses[len(d.pulses)-64:]
	}
	n := len(d.pulses)
	d.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "key": d.Pkg.Key, "received": n, "echo": msg,
	})
}

func (d *Daemon) handleResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query().Get("key")
	if q == "" {
		q = d.Pkg.Key
	}
	d.mu.Lock()
	v, ok := d.nameRecord[q]
	if !ok {
		v, ok = d.nameRecord[q+".ans"]
	}
	d.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": ok, "key": q, "reach": v,
	})
}

func (d *Daemon) loadNameRecord() {
	path := filepath.Join(d.DataDir, "names.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &d.nameRecord)
}

func (d *Daemon) saveNameRecord() {
	path := filepath.Join(d.DataDir, "names.json")
	b, _ := json.MarshalIndent(d.nameRecord, "", "  ")
	_ = os.WriteFile(path, b, 0o644)
}

// SendPulseToPeer opens a stream to another gen/node peer id (multiaddr optional).
func (d *Daemon) SendPulseToPeer(ctx context.Context, peerAddr string, payload map[string]interface{}) error {
	if d.host == nil {
		return fmt.Errorf("p2p no activo")
	}
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return err
	}
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return err
	}
	if err := d.host.Connect(ctx, *info); err != nil {
		return err
	}
	s, err := d.host.NewStream(ctx, info.ID, PulseProtocol)
	if err != nil {
		return err
	}
	defer s.Close()
	b, _ := json.Marshal(payload)
	_, err = s.Write(b)
	return err
}


func (d *Daemon) addFinding(report map[string]interface{}) {
	d.mu.Lock()
	d.findings = append(d.findings, report)
	if len(d.findings) > 48 {
		d.findings = d.findings[len(d.findings)-48:]
	}
	d.mu.Unlock()
	d.saveFindings()
}

func (d *Daemon) loadFindings() {
	path := filepath.Join(d.DataDir, "findings.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var list []map[string]interface{}
	if json.Unmarshal(b, &list) == nil {
		d.findings = list
	}
}

func (d *Daemon) saveFindings() {
	d.mu.Lock()
	b, _ := json.MarshalIndent(d.findings, "", "  ")
	d.mu.Unlock()
	_ = os.WriteFile(filepath.Join(d.DataDir, "findings.json"), b, 0o644)
}

func (d *Daemon) handleExplore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var req struct {
		URL     string `json:"url"`
		Mission string `json:"mission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	res, err := d.Explore(req.URL, req.Mission)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}

func (d *Daemon) handleDialogue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var req struct {
		Text      string `json:"text"`
		Stimulus  string `json:"stimulus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	stim := req.Text
	if stim == "" {
		stim = req.Stimulus
	}
	_ = json.NewEncoder(w).Encode(d.Dialogue(stim))
}

func (d *Daemon) handleFindings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	d.mu.Lock()
	list := append([]map[string]interface{}{}, d.findings...)
	d.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "count": len(list), "findings": list})
}

func (d *Daemon) announceLoop(ctx context.Context) {
	d.doAnnounce()
	t := time.NewTicker(45 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d.doAnnounce()
		}
	}
}

func (d *Daemon) doAnnounce() {
	base := strings.TrimRight(strings.TrimSpace(d.AnnounceURL), "/")
	if base == "" {
		return
	}
	reach := strings.TrimSpace(d.PublicURL)
	if reach == "" {
		reach = "http://" + d.publicHTTPHint()
	}
	payload := map[string]interface{}{
		"key":        d.Pkg.Key,
		"http_base":  reach,
		"root_cid":   d.Pkg.CurrentRootCID,
		"mode":       "daemon",
		"findings":   0,
	}
	d.mu.Lock()
	payload["findings"] = len(d.findings)
	d.mu.Unlock()
	if d.host != nil {
		payload["peer_id"] = d.host.ID().String()
	}
	b, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodPost, base+"/api/gen/announce", bytes.NewReader(b))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️ announce → Mind/nodo: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		log.Printf("⚠️ announce status %d: %s", resp.StatusCode, body)
		return
	}
	log.Printf("📡 anunciado a %s como %s → %s", base, d.Pkg.Key, reach)
}
