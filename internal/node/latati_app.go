package node

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//go:embed embedded/latati_index.html
var latatiAppHTML []byte

//go:embed embedded/latati_gestion.html
var latatiGestionHTML []byte

//go:embed embedded/latati_icon_192.png
var latatiIcon192 []byte

//go:embed embedded/latati_icon_512.png
var latatiIcon512 []byte

//go:embed embedded/latati_icon.svg
var latatiIconSVG []byte

const latatiAppID = "app-latati"
const latatiAlias = "latati.app.ans"
const latatiGestionID = "app-latati-gestion"
const latatiGestionAlias = "gestion-latati.app.ans"

func (n *NodoAlset) ensureLaTatiApp() {
	if len(latatiAppHTML) == 0 {
		fmt.Println("⚠️ La Tati embed vacío")
		return
	}
	dir := filepath.Join(StaticDir, "apps", "latati")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "index.html"), latatiAppHTML, 0644)

	gdir := filepath.Join(StaticDir, "apps", "gestion-latati")
	_ = os.MkdirAll(gdir, 0755)
	if len(latatiGestionHTML) > 0 {
		_ = os.WriteFile(filepath.Join(gdir, "index.html"), latatiGestionHTML, 0644)
	}

	cid, err := n.GenerarCID(latatiAppHTML)
	if err != nil || cid == "" {
		fmt.Println("⚠️ CID La Tati:", err)
		return
	}
	gCID := cid
	if len(latatiGestionHTML) > 0 {
		if c2, err2 := n.GenerarCID(latatiGestionHTML); err2 == nil && c2 != "" {
			gCID = c2
		}
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	n.agentes[latatiAppID] = &Agente{ID: latatiAppID, RootCID: cid, BalanceUTXO: 0, UltimaActual: time.Now().Unix()}
	n.nombres[latatiAlias] = latatiAppID
	n.agentes[latatiGestionID] = &Agente{ID: latatiGestionID, RootCID: gCID, BalanceUTXO: 0, UltimaActual: time.Now().Unix()}
	n.nombres[latatiGestionAlias] = latatiGestionID
	fmt.Printf("✅ La Tati: /w/%s · gestión /w/%s\n", latatiAlias, latatiGestionAlias)
	go n.ltBootTenants()
}

func (n *NodoAlset) ltBootTenants() {
	ltMu.Lock()
	reg := n.ltLoadRegistry()
	ltMu.Unlock()
	for _, meta := range reg {
		if meta.Slug == "" || meta.Slug == "latati" || !meta.Active {
			continue
		}
		n.ltRegisterTenantApps(meta.Slug)
	}
}
