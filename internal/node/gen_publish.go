package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// PublicPackageURLs returns URLs where a package CID can be fetched.
// Primary: this node. Optional: IPFS_GATEWAY env (comma-separated), e.g. https://ipfs.io/ipfs/
func PublicPackageURLs(cid string) []string {
	cid = strings.TrimSpace(cid)
	if cid == "" {
		return nil
	}
	var urls []string
	base := strings.TrimRight(os.Getenv("ALSET_PUBLIC_BASE"), "/")
	if base == "" {
		base = strings.TrimRight(os.Getenv("RENDER_EXTERNAL_URL"), "/")
	}
	if base != "" {
		urls = append(urls, base+"/api/gen/by-cid?cid="+cid)
	}
	// Always document relative path for same-origin clients
	urls = append(urls, "/api/gen/by-cid?cid="+cid)
	for _, g := range strings.Split(os.Getenv("IPFS_GATEWAY"), ",") {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if !strings.HasSuffix(g, "/") {
			g += "/"
		}
		// Only useful if the CID is pinned on that network; still listed as attempt URL
		urls = append(urls, g+cid)
	}
	return urls
}

// FetchPackageBytes tries local blockstore then optional HTTP gateways.
func (n *NodoAlset) FetchPackageBytes(cid string) ([]byte, string, error) {
	cid = strings.TrimSpace(cid)
	if cid == "" {
		return nil, "", fmt.Errorf("cid vacío")
	}
	if raw, err := n.BuscarContenidoPorCID(cid); err == nil && len(raw) > 0 {
		return raw, "local", nil
	}
	client := &http.Client{Timeout: 20 * time.Second}
	for _, u := range PublicPackageURLs(cid) {
		if strings.HasPrefix(u, "/") {
			continue // relative — skip without base
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && len(body) > 0 {
			// Seal into local store so revive works offline next time
			if _, err := n.GenerarCID(body); err == nil {
				_ = err
			}
			return body, u, nil
		}
	}
	return nil, "", fmt.Errorf("paquete %s no encontrado local ni en gateways", truncateCID(cid))
}

func (n *NodoAlset) handleGenByCID(w http.ResponseWriter, r *http.Request) {
	cid := r.URL.Query().Get("cid")
	if cid == "" {
		// path style /api/gen/by-cid/bafk...
		cid = strings.TrimPrefix(r.URL.Path, "/api/gen/by-cid/")
		cid = strings.Trim(cid, "/")
	}
	if cid == "" {
		http.Error(w, "cid required", 400)
		return
	}
	raw, err := n.BuscarContenidoPorCID(cid)
	if err != nil || len(raw) == 0 {
		http.Error(w, "not found", 404)
		return
	}
	// Validate it looks like a frontier package (optional soft check)
	var probe map[string]interface{}
	_ = json.Unmarshal(raw, &probe)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("X-Alset-CID", cid)
	_, _ = w.Write(raw)
}

// PublishGenPackage seals package and returns recovery URLs (IPFS-oriented).
func (n *NodoAlset) PublishGenPackage(key string) (map[string]interface{}, error) {
	sealed, err := n.SealFrontierPackage(key)
	if err != nil {
		return nil, err
	}
	cid, _ := sealed["package_cid"].(string)
	urls := PublicPackageURLs(cid)
	out := map[string]interface{}{
		"ok":          true,
		"key":         normalizeGenKey(key),
		"package_cid": cid,
		"urls":        urls,
		"note":        "guarda package_cid; cualquiera puede GET by-cid o revive. Pin en IPFS_GATEWAY si lo configuras.",
	}
	return out, nil
}

func (n *NodoAlset) handleGenPublish(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	res, err := n.PublishGenPackage(req.Key)
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}

// ResolveGenDNS looks up TXT records for name resolution.
// Formats accepted in TXT:
//
//	alset-pkg=bafk...
//	alset-reach=https://...
//	alset-udp=host:port
//
// Lookup host: ALSET_DNS_SUFFIX empty → try name as FQDN if it contains a dot;
// else name + "." + suffix (e.g. demo-cell.alset.example.com).
func ResolveGenDNS(key string) map[string]string {
	out := map[string]string{}
	key = strings.TrimSuffix(strings.TrimSpace(strings.ToLower(key)), ".ans")
	if key == "" {
		return out
	}
	suffix := strings.TrimSpace(os.Getenv("ALSET_DNS_SUFFIX"))
	hosts := []string{}
	if strings.Contains(key, ".") {
		hosts = append(hosts, key)
	}
	if suffix != "" {
		hosts = append(hosts, key+"."+strings.TrimPrefix(suffix, "."))
		hosts = append(hosts, "_alset."+key+"."+strings.TrimPrefix(suffix, "."))
	}
	resolver := &net.Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, h := range hosts {
		txts, err := resolver.LookupTXT(ctx, h)
		if err != nil {
			continue
		}
		for _, tx := range txts {
			tx = strings.TrimSpace(tx)
			for _, part := range strings.Fields(tx) {
				if strings.HasPrefix(part, "alset-pkg=") {
					out["package_cid"] = strings.TrimPrefix(part, "alset-pkg=")
				}
				if strings.HasPrefix(part, "alset-reach=") {
					out["http_base"] = strings.TrimPrefix(part, "alset-reach=")
				}
				if strings.HasPrefix(part, "alset-udp=") {
					out["udp"] = strings.TrimPrefix(part, "alset-udp=")
				}
			}
			// whole TXT as single token styles
			if strings.HasPrefix(tx, "alset-pkg=") {
				out["package_cid"] = strings.TrimPrefix(tx, "alset-pkg=")
			}
			if strings.HasPrefix(tx, "alset-reach=") {
				out["http_base"] = strings.TrimPrefix(tx, "alset-reach=")
			}
		}
		if len(out) > 0 {
			out["dns_host"] = h
			break
		}
	}
	return out
}

func (n *NodoAlset) handleGenResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key required", 400)
		return
	}
	res := map[string]interface{}{"ok": true, "key": normalizeGenKey(key)}
	// 1) local / announced
	n.mu.RLock()
	if g, ok := n.gens[normalizeGenKey(key)]; ok {
		res["local"] = true
		res["location"] = g.State.Location
		if g.State.Metadata != nil {
			if rh, ok := g.State.Metadata["remote_http"].(string); ok {
				res["remote_http"] = rh
			}
			if pc, ok := g.State.Metadata["package_cid"].(string); ok {
				res["package_cid"] = pc
			}
		}
	}
	n.mu.RUnlock()
	// 2) DNS TXT
	dns := ResolveGenDNS(key)
	if len(dns) > 0 {
		res["dns"] = dns
		if res["package_cid"] == nil && dns["package_cid"] != "" {
			res["package_cid"] = dns["package_cid"]
		}
		if res["remote_http"] == nil && dns["http_base"] != "" {
			res["remote_http"] = dns["http_base"]
		}
	}
	if pc, _ := res["package_cid"].(string); pc != "" {
		res["urls"] = PublicPackageURLs(pc)
	}
	_ = json.NewEncoder(w).Encode(res)
}

