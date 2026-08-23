package gennode

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// Pulse over UDP: light protocol so the gen can live on the network path
// without depending on HTTP for peer discovery and dialogue on a LAN / edge.
//
// This is NOT "code executing inside foreign IP packets". It is:
// - a small process on connectivity gear (OpenWrt, Pi, gateway)
// - UDP beacons + request/response pulses carrying JSON cargo (identity, findings)
// - optional HTTP still available for browsers / Mind over the tunnel

const (
	DefaultUDPPort = 9091
	maxUDPPayload  = 1200 // stay under typical MTU for single datagram
)

// UDPPulse is the wire format for gen residency on the network path.
type UDPPulse struct {
	V      int                    `json:"v"` // protocol version
	Type   string                 `json:"type"` // BEACON | CONSULTA | RESPUESTA | HALLAZGO | CARGO
	Key    string                 `json:"key"`
	From   string                 `json:"from,omitempty"`
	Text   string                 `json:"text,omitempty"`
	Data   map[string]interface{} `json:"data,omitempty"`
	TS     int64                  `json:"ts"`
}

func (d *Daemon) StartUDP(port int) error {
	if port <= 0 {
		port = DefaultUDPPort
	}
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.udpConn = conn
	d.udpPort = port
	d.mu.Unlock()
	log.Printf("📡 Pulse-over-UDP escuchando :%d (beacon + consulta)", port)
	go d.udpReadLoop(conn)
	go d.udpBeaconLoop(conn)
	return nil
}

func (d *Daemon) udpReadLoop(conn *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		var p UDPPulse
		if json.Unmarshal(buf[:n], &p) != nil {
			continue
		}
		d.handleUDPPulse(conn, remote, p)
	}
}

func (d *Daemon) handleUDPPulse(conn *net.UDPConn, remote *net.UDPAddr, p UDPPulse) {
	switch strings.ToUpper(p.Type) {
	case "BEACON":
		// peer presence — record in name map
		if p.Key != "" && remote != nil {
			d.mu.Lock()
			d.nameRecord[p.Key] = fmt.Sprintf("udp://%s", remote.String())
			d.mu.Unlock()
		}
	case "CONSULTA":
		dlg := d.Dialogue(p.Text)
		voice, _ := dlg["voice"].(string)
		resp := UDPPulse{
			V: 1, Type: "RESPUESTA", Key: d.Pkg.Key, From: d.Pkg.Key,
			Text: voice, TS: time.Now().Unix(),
			Data: map[string]interface{}{
				"findings": dlg["findings_count"],
				"root_cid": d.Pkg.CurrentRootCID,
			},
		}
		d.sendUDP(conn, remote, resp)
	case "HALLAZGO":
		if p.Data != nil {
			d.addFinding(p.Data)
		}
	case "CARGO":
		// Fragment of package identity — log for reassembly demos
		d.mu.Lock()
		d.pulses = append(d.pulses, map[string]interface{}{
			"type": "CARGO", "from": p.From, "data": p.Data, "ts": p.TS,
		})
		d.mu.Unlock()
	case "RESPUESTA":
		d.mu.Lock()
		d.pulses = append(d.pulses, map[string]interface{}{
			"type": "RESPUESTA", "from": p.Key, "text": p.Text, "ts": p.TS,
		})
		d.mu.Unlock()
	}
}

func (d *Daemon) udpBeaconLoop(conn *net.UDPConn) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for range t.C {
		d.mu.RLock()
		nFind := len(d.findings)
		d.mu.RUnlock()
		p := UDPPulse{
			V: 1, Type: "BEACON", Key: d.Pkg.Key, From: d.Pkg.Key,
			TS: time.Now().Unix(),
			Data: map[string]interface{}{
				"root_cid": d.Pkg.CurrentRootCID,
				"findings": nFind,
				"http":     d.PublicURL,
				"udp_port": d.udpPort,
			},
		}
		// Broadcast on local subnet (best-effort)
		baddr := &net.UDPAddr{IP: net.IPv4bcast, Port: d.udpPort}
		d.sendUDP(conn, baddr, p)
		// Also 255.255.255.255 may fail on some OS; try 192.168.255.255 style via interface addrs
		_ = d.beaconToLocalSubnets(conn, p)
	}
}

func (d *Daemon) beaconToLocalSubnets(conn *net.UDPConn, p UDPPulse) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			bcast := broadcastAddr(ipnet)
			if bcast == nil {
				continue
			}
			d.sendUDP(conn, &net.UDPAddr{IP: bcast, Port: d.udpPort}, p)
		}
	}
	return nil
}

func broadcastAddr(n *net.IPNet) net.IP {
	ip := n.IP.To4()
	mask := n.Mask
	if ip == nil || mask == nil {
		return nil
	}
	out := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		out[i] = ip[i] | ^mask[i]
	}
	return out
}

func (d *Daemon) sendUDP(conn *net.UDPConn, to *net.UDPAddr, p UDPPulse) {
	if conn == nil || to == nil {
		return
	}
	b, err := json.Marshal(p)
	if err != nil || len(b) > maxUDPPayload {
		return
	}
	_, _ = conn.WriteToUDP(b, to)
}

// SendUDPConsulta sends a CONSULTA pulse to a UDP endpoint (e.g. another edge gen).
func (d *Daemon) SendUDPConsulta(hostPort, text string) error {
	d.mu.RLock()
	conn := d.udpConn
	d.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("UDP no activo")
	}
	raddr, err := net.ResolveUDPAddr("udp", hostPort)
	if err != nil {
		return err
	}
	p := UDPPulse{
		V: 1, Type: "CONSULTA", Key: d.Pkg.Key, From: d.Pkg.Key,
		Text: text, TS: time.Now().Unix(),
	}
	d.sendUDP(conn, raddr, p)
	return nil
}

// EmitCargoFragments sends package identity as small UDP CARGO messages (demo of "traveling as packets").
func (d *Daemon) EmitCargoFragments(to *net.UDPAddr) {
	d.mu.RLock()
	conn := d.udpConn
	d.mu.RUnlock()
	if conn == nil || to == nil {
		return
	}
	parts := []map[string]interface{}{
		{"part": 1, "of": 4, "claim": "identity", "key": d.Pkg.Key},
		{"part": 2, "of": 4, "claim": "root", "root_cid": d.Pkg.CurrentRootCID},
		{"part": 3, "of": 4, "claim": "service", "service_cid": d.Pkg.ServiceCID},
		{"part": 4, "of": 4, "claim": "note", "text": "Alset-Gen cargo — ensamblar en el borde, no en el payload ajeno"},
	}
	for _, part := range parts {
		p := UDPPulse{V: 1, Type: "CARGO", Key: d.Pkg.Key, From: d.Pkg.Key, TS: time.Now().Unix(), Data: part}
		d.sendUDP(conn, to, p)
		time.Sleep(30 * time.Millisecond)
	}
}
