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

const valesplusAppID = "app-valesplus"
const valesplusAlias = "valesplus.app.ans"

func (n *NodoAlset) ensureValesPlusApp() {
	if len(valesplusAppHTML) == 0 {
		fmt.Println("⚠️ ValesPlus embed vacío")
		return
	}
	dir := filepath.Join(StaticDir, "apps", "valesplus")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "index.html")
	if err := os.WriteFile(path, valesplusAppHTML, 0644); err != nil {
		fmt.Println("⚠️ No se pudo escribir ValesPlus:", err)
		return
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
