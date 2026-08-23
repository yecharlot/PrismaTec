package gennode

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// FrontierPackage mirrors node.FrontierPackage for autonomous revival without the full node.
type FrontierPackage struct {
	Type           string          `json:"type"`
	Version        string          `json:"version"`
	Key            string          `json:"key"`
	ID             string          `json:"id"`
	CurrentRootCID string          `json:"current_root_cid"`
	History        []string        `json:"history,omitempty"`
	Manifest       json.RawMessage `json:"manifest"`
	Organs         json.RawMessage `json:"organs"`
	State          json.RawMessage `json:"state"`
	EpisodeCIDs    []string        `json:"episode_cids,omitempty"`
	ServiceCID     string          `json:"service_cid,omitempty"`
	ServicePath    string          `json:"service_path,omitempty"`
	ServiceHTML    string          `json:"service_html,omitempty"`
	SealedAt       string          `json:"sealed_at"`
	OriginNote     string          `json:"origin_note"`
}

func LoadPackageFile(path string) (*FrontierPackage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p FrontierPackage
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	if p.Type != "" && p.Type != "alset_gen_frontier_package" {
		return nil, fmt.Errorf("tipo de paquete no reconocido: %s", p.Type)
	}
	if p.Key == "" {
		return nil, fmt.Errorf("paquete sin key")
	}
	if !strings.HasSuffix(p.Key, ".ans") {
		p.Key = p.Key + ".ans"
	}
	return &p, nil
}

func (p *FrontierPackage) ServicePage() string {
	if strings.TrimSpace(p.ServiceHTML) != "" {
		return p.ServiceHTML
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="es"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>%s</title>
<style>body{font-family:system-ui;background:#0b1220;color:#e8eefc;display:flex;min-height:100vh;align-items:center;justify-content:center;margin:0}
.card{background:#141e33;border:1px solid #243044;border-radius:16px;padding:2rem;max-width:480px}
h1{color:#5eead4;font-size:1.25rem}code{color:#a5f3fc;font-size:.8rem;word-break:break-all}</style>
</head><body><div class="card">
<div style="color:#5eead4;font-size:.75rem;margin-bottom:.5rem">Alset-Gen · daemon autónomo</div>
<h1>%s</h1>
<p>Célula viva fuera del monolito. Escucha HTTP y pulsos locales.</p>
<p>RootCID: <code>%s</code></p>
<p>Arranque: %s</p>
</div></body></html>`, p.Key, p.Key, p.CurrentRootCID, time.Now().UTC().Format(time.RFC3339))
}
