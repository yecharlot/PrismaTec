package gennode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EdgePeer is another gen daemon on the network path (store-and-forward hop).
type EdgePeer struct {
	Key      string `json:"key,omitempty"`
	HTTPBase string `json:"http_base,omitempty"`
	UDPAddr  string `json:"udp_addr,omitempty"` // host:port
	LastSeen int64  `json:"last_seen,omitempty"`
}

// CargoEnvelope travels hop-to-hop without claiming to live inside foreign IP stacks.
type CargoEnvelope struct {
	V         int                    `json:"v"`
	Key       string                 `json:"key"`
	PackageCID string                `json:"package_cid,omitempty"`
	RootCID   string                 `json:"root_cid,omitempty"`
	TTL       int                    `json:"ttl"` // hops remaining
	Seen      []string               `json:"seen,omitempty"` // peer keys/urls already visited
	Payload   map[string]interface{} `json:"payload,omitempty"`
	TS        int64                  `json:"ts"`
}

func (d *Daemon) loadPeers() []EdgePeer {
	path := filepath.Join(d.DataDir, "edge_peers.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []EdgePeer
	_ = json.Unmarshal(b, &list)
	return list
}

func (d *Daemon) savePeers(list []EdgePeer) {
	path := filepath.Join(d.DataDir, "edge_peers.json")
	b, _ := json.MarshalIndent(list, "", "  ")
	_ = os.WriteFile(path, b, 0o644)
}

func (d *Daemon) AddEdgePeer(p EdgePeer) {
	list := d.loadPeers()
	key := p.HTTPBase
	if key == "" {
		key = p.UDPAddr
	}
	found := false
	for i := range list {
		if list[i].HTTPBase == p.HTTPBase && p.HTTPBase != "" || list[i].UDPAddr == p.UDPAddr && p.UDPAddr != "" {
			list[i] = p
			list[i].LastSeen = time.Now().Unix()
			found = true
			break
		}
	}
	if !found {
		p.LastSeen = time.Now().Unix()
		list = append(list, p)
	}
	if len(list) > 32 {
		list = list[len(list)-32:]
	}
	d.savePeers(list)
}

// AcceptCargo stores identity cargo and optionally forwards to other edge peers (TTL).
func (d *Daemon) AcceptCargo(env CargoEnvelope) map[string]interface{} {
	if env.TTL <= 0 {
		env.TTL = 0
	}
	if env.Key == "" {
		env.Key = d.Pkg.Key
	}
	d.mu.Lock()
	d.pulses = append(d.pulses, map[string]interface{}{
		"type": "CARGO_ACCEPT", "key": env.Key, "ttl": env.TTL, "ts": time.Now().Unix(),
		"package_cid": env.PackageCID, "payload": env.Payload,
	})
	if len(d.pulses) > 64 {
		d.pulses = d.pulses[len(d.pulses)-64:]
	}
	d.mu.Unlock()

	// Record as soft finding for dialogue
	d.addFinding(map[string]interface{}{
		"ok": true, "mission": "cargo", "url": "cargo://hop",
		"title": "CARGO " + env.Key, "snippet": fmt.Sprintf("ttl=%d root=%s pkg=%s", env.TTL, env.RootCID, env.PackageCID),
		"ts": time.Now().UTC().Format(time.RFC3339),
	})

	forwarded := 0
	if env.TTL > 1 {
		env.TTL--
		me := d.Pkg.Key
		if d.PublicURL != "" {
			me = d.PublicURL
		}
		env.Seen = append(env.Seen, me)
		env.TS = time.Now().Unix()
		for _, peer := range d.loadPeers() {
			if peerSeen(env.Seen, peer) {
				continue
			}
			if d.forwardCargo(peer, env) {
				forwarded++
			}
		}
	}
	return map[string]interface{}{
		"ok": true, "key": d.Pkg.Key, "accepted": true, "forwarded": forwarded, "ttl_left": env.TTL,
		"note": "store-and-forward en el borde — no es ejecución dentro de routers ajenos",
	}
}

func peerSeen(seen []string, p EdgePeer) bool {
	for _, s := range seen {
		if p.HTTPBase != "" && strings.Contains(s, p.HTTPBase) {
			return true
		}
		if p.Key != "" && s == p.Key {
			return true
		}
	}
	return false
}

func (d *Daemon) forwardCargo(p EdgePeer, env CargoEnvelope) bool {
	if p.HTTPBase != "" {
		base := strings.TrimRight(p.HTTPBase, "/")
		b, _ := json.Marshal(env)
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Post(base+"/api/cargo", "application/json", bytes.NewReader(b))
		if err != nil {
			log.Printf("⚠️ cargo hop HTTP %s: %v", base, err)
			return false
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 256))
		resp.Body.Close()
		return resp.StatusCode < 300
	}
	if p.UDPAddr != "" && d.udpConn != nil {
		raddr, err := net.ResolveUDPAddr("udp", p.UDPAddr)
		if err != nil {
			return false
		}
		// Split into CARGO pulses (fits UDP)
		parts := []map[string]interface{}{
			{"claim": "key", "key": env.Key, "ttl": env.TTL},
			{"claim": "root", "root_cid": env.RootCID},
			{"claim": "package", "package_cid": env.PackageCID},
		}
		for _, part := range parts {
			d.sendUDP(d.udpConn, raddr, UDPPulse{
				V: 1, Type: "CARGO", Key: d.Pkg.Key, From: d.Pkg.Key,
				TS: time.Now().Unix(), Data: part,
			})
		}
		return true
	}
	return false
}

// SeedCargoFromSelf emits this gen's identity into the edge mesh.
func (d *Daemon) SeedCargoFromSelf(ttl int) map[string]interface{} {
	if ttl <= 0 {
		ttl = 3
	}
	env := CargoEnvelope{
		V: 1, Key: d.Pkg.Key, RootCID: d.Pkg.CurrentRootCID,
		PackageCID: d.Pkg.ServiceCID, TTL: ttl, TS: time.Now().Unix(),
		Payload: map[string]interface{}{
			"mode": "seed", "service_path": d.Pkg.ServicePath,
		},
	}
	return d.AcceptCargo(env)
}

func (d *Daemon) handleCargo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var env CargoEnvelope
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	_ = json.NewEncoder(w).Encode(d.AcceptCargo(env))
}

func (d *Daemon) handleCargoSeed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ttl := 3
	if r.Method == http.MethodPost {
		var req struct {
			TTL int `json:"ttl"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.TTL > 0 {
			ttl = req.TTL
		}
	}
	_ = json.NewEncoder(w).Encode(d.SeedCargoFromSelf(ttl))
}

func (d *Daemon) handlePeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		var p EdgePeer
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		d.AddEdgePeer(p)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "peers": d.loadPeers()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "peers": d.loadPeers()})
}
