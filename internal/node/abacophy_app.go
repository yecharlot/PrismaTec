package node

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//go:embed embedded/abacophy_index.html
var abacophyAppHTML []byte

const abacophyAppID = "app-abacophy"
const abacophyAlias = "abacophy.app.ans"

// ensureAbacoPhyApp escribe la UI contable embebida y registra el nombre ANS.
// La API REST vive en el servicio dedicado abacophy.onrender.com; este embed
// garantiza que /w/abacophy.app.ans sobreviva a los redeploys del nodo.
func (n *NodoAlset) ensureAbacoPhyApp() {
	if len(abacophyAppHTML) == 0 {
		fmt.Println("⚠️ ÁbacoPhy embed vacío")
		return
	}
	dir := filepath.Join(StaticDir, "apps", "abacophy")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "index.html")
	if err := os.WriteFile(path, abacophyAppHTML, 0644); err != nil {
		fmt.Println("⚠️ No se pudo escribir ÁbacoPhy:", err)
		return
	}
	cid, err := n.GenerarCID(abacophyAppHTML)
	if err != nil || cid == "" {
		fmt.Println("⚠️ CID ÁbacoPhy:", err)
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
	n.agentes[abacophyAppID] = &Agente{
		ID:           abacophyAppID,
		RootCID:      cid,
		BalanceUTXO:  0,
		UltimaActual: time.Now().Unix(),
	}
	n.nombres[abacophyAlias] = abacophyAppID
	fmt.Printf("✅ ÁbacoPhy registrado: /w/%s (CID %s)\n", abacophyAlias, cid)
}
