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

const latatiAppID = "app-latati"
const latatiAlias = "latati.app.ans"

func (n *NodoAlset) ensureLaTatiApp() {
	if len(latatiAppHTML) == 0 {
		fmt.Println("⚠️ La Tati embed vacío")
		return
	}
	dir := filepath.Join(StaticDir, "apps", "latati")
	_ = os.MkdirAll(dir, 0755)
	if err := os.WriteFile(filepath.Join(dir, "index.html"), latatiAppHTML, 0644); err != nil {
		fmt.Println("⚠️ La Tati write", err)
	}
	cid, err := n.GenerarCID(latatiAppHTML)
	if err != nil || cid == "" {
		fmt.Println("⚠️ CID La Tati:", err)
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
	n.agentes[latatiAppID] = &Agente{ID: latatiAppID, RootCID: cid, BalanceUTXO: 0, UltimaActual: time.Now().Unix()}
	n.nombres[latatiAlias] = latatiAppID
	fmt.Printf("✅ La Tati registrada: /w/%s\n", latatiAlias)
}
