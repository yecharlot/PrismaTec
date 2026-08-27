package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Dispatch destinations: local | cloudflare | (future: daemon url)
func cloudflareNetworkBase() string {
	v := strings.TrimRight(strings.TrimSpace(os.Getenv("ALSET_CLOUDFLARE_NETWORK")), "/")
	if v != "" {
		return v
	}
	// Red edge de PrismaTec por defecto (sobreescribible con env o alset_data/cloudflare.env)
	return "https://alset-network.lhmolam-877.workers.dev"
}

// DispatchGenToCloudflare seals package, spawns gen on CF network, announces reach.
func (n *NodoAlset) DispatchGenToCloudflare(key, mission string) (map[string]interface{}, error) {
	base := cloudflareNetworkBase()
	if base == "" {
		return nil, fmt.Errorf("ALSET_CLOUDFLARE_NETWORK no configurada (ej. https://alset-network.xxx.workers.dev)")
	}
	key = normalizeGenKey(key)
	// Ensure gen exists locally first
	if _, err := n.CreateAlsetGen(key, "", "seed", mission); err != nil {
		// may already exist
		_ = err
	}
	var packageCID, rootCID string
	if sealed, err := n.SealFrontierPackage(key); err == nil {
		packageCID, _ = sealed["package_cid"].(string)
	}
	n.mu.RLock()
	if g, ok := n.gens[key]; ok {
		rootCID = g.CurrentRootCID
	}
	n.mu.RUnlock()

	body, _ := json.Marshal(map[string]interface{}{
		"key": key, "package_cid": packageCID, "root_cid": rootCID, "mission": mission,
	})
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Post(base+"/api/network/dispatch", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cloudflare network: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]interface{}{"ok": false, "body": string(raw)}
	}
	if resp.StatusCode >= 300 {
		out["ok"] = false
		out["http_status"] = resp.StatusCode
		return out, fmt.Errorf("cloudflare HTTP %d", resp.StatusCode)
	}
	reach, _ := out["reach"].(string)
	if reach != "" {
		_ = n.AnnounceRemoteGen(key, reach, "", rootCID, 0, 0)
		n.mu.Lock()
		if g, ok := n.gens[key]; ok {
			if g.State.Metadata == nil {
				g.State.Metadata = map[string]interface{}{}
			}
			g.State.Metadata["dispatch"] = "cloudflare"
			g.State.Metadata["remote_http"] = reach
			if packageCID != "" {
				g.State.Metadata["package_cid"] = packageCID
			}
			if mission != "" {
				g.State.Metadata["mission"] = mission
			}
			g.State.Location = "cloudflare:" + reach
			g.UpdatedAt = time.Now().Unix()
		}
		n.mu.Unlock()
		n.saveGensToDisk()
	}
	out["destination"] = "cloudflare"
	out["key"] = key
	return out, nil
}

func (n *NodoAlset) handleGenDispatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var req struct {
		Key         string `json:"key"`
		Destination string `json:"destination"` // cloudflare | local
		Mission     string `json:"mission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	dest := strings.ToLower(strings.TrimSpace(req.Destination))
	if dest == "" {
		dest = "cloudflare"
	}
	switch dest {
	case "cloudflare", "cf", "edge":
		res, err := n.DispatchGenToCloudflare(req.Key, req.Mission)
		if err != nil {
			w.WriteHeader(502)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error(), "partial": res})
			return
		}
		_ = json.NewEncoder(w).Encode(res)
	case "local":
		g, err := n.CreateAlsetGen(req.Key, "", "seed", req.Mission)
		if err != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true, "destination": "local", "key": g.Key, "location": g.State.Location,
		})
	default:
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": "destination debe ser cloudflare o local",
		})
	}
}
