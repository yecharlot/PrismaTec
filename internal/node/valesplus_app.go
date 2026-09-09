package node

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//go:embed embedded/valesplus_index.html
var valesplusAppHTML []byte

//go:embed embedded/valesplus_manifest.webmanifest
var valesplusManifest []byte

//go:embed embedded/valesplus_sw.js
var valesplusSW []byte

//go:embed embedded/valesplus_icon_192.png
var valesplusIcon192 []byte

//go:embed embedded/valesplus_icon_512.png
var valesplusIcon512 []byte

//go:embed embedded/valesplus_apple.png
var valesplusApple []byte

//go:embed embedded/valesplus_badge.png
var valesplusBadge []byte

const valesplusAppID = "app-valesplus"
const valesplusAlias = "valesplus.app.ans"

func (n *NodoAlset) ensureValesPlusApp() {
	if len(valesplusAppHTML) == 0 {
		fmt.Println("⚠️ ValesPlus embed vacío")
		return
	}
	dir := filepath.Join(StaticDir, "apps", "valesplus")
	_ = os.MkdirAll(dir, 0755)
	writes := map[string][]byte{
		"index.html":            valesplusAppHTML,
		"manifest.webmanifest":  valesplusManifest,
		"sw.js":                 valesplusSW,
		"icon-192.png":          valesplusIcon192,
		"icon-512.png":          valesplusIcon512,
		"apple-touch-icon.png":  valesplusApple,
		"prismatec-badge.png":   valesplusBadge,
	}
	for name, data := range writes {
		if len(data) == 0 {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			fmt.Println("⚠️ ValesPlus asset", name, err)
		}
	}
	cid, err := n.GenerarCID(valesplusAppHTML)
	if err != nil || cid == "" {
		fmt.Println("⚠️ CID ValesPlus:", err)
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	n.agentes[valesplusAppID] = &Agente{
		ID:           valesplusAppID,
		RootCID:      cid,
		BalanceUTXO:  0,
		UltimaActual: time.Now().Unix(),
	}
	n.nombres[valesplusAlias] = valesplusAppID
	fmt.Printf("✅ ValesPlus registrado: /w/%s (CID %s)\n", valesplusAlias, cid)
}
