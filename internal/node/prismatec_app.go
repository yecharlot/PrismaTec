package node

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//go:embed embedded/prismatec_index.html
var prismatecAppHTML []byte

const prismatecAppID = "app-prismatec-landing"
const prismatecAlias = "prismatec.app.ans"

// ensurePrismatecApp writes the institutional landing to disk, stores a CID block,
// and registers the name prismatec.app.ans so /w/prismatec.app.ans resolves via RootCID.
func (n *NodoAlset) ensurePrismatecApp() {
	if len(prismatecAppHTML) == 0 {
		fmt.Println("⚠️ Prism@.TEC landing embed vacío")
		return
	}
	dir := filepath.Join(StaticDir, "apps", "prismatec")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "index.html")
	if err := os.WriteFile(path, prismatecAppHTML, 0644); err != nil {
		fmt.Println("⚠️ No se pudo escribir landing Prism@.TEC:", err)
	}

	cid, err := n.GenerarCID(prismatecAppHTML)
	if err != nil || cid == "" {
		fmt.Println("⚠️ CID Prism@.TEC:", err)
		// still serve from disk via /w file path
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
	n.agentes[prismatecAppID] = &Agente{
		ID:           prismatecAppID,
		RootCID:      cid,
		BalanceUTXO:  0,
		UltimaActual: time.Now().Unix(),
	}
	n.nombres[prismatecAlias] = prismatecAppID
	fmt.Printf("✅ Prism@.TEC landing registrada: /w/%s (CID %s)\n", prismatecAlias, cid)
}
