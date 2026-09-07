package node

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//go:embed embedded/finanzas_index.html
var finanzasAppHTML []byte

const finanzasAppID = "app-finanzas-mapa"
const finanzasAlias = "finanzas.app.ans"

// ensureFinanzasApp writes the financial-map landing, stores a CID block,
// and registers finanzas.app.ans so /w/finanzas.app.ans resolves.
func (n *NodoAlset) ensureFinanzasApp() {
	if len(finanzasAppHTML) == 0 {
		fmt.Println("⚠️ Mapa financiero embed vacío")
		return
	}
	dir := filepath.Join(StaticDir, "apps", "finanzas")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "index.html")
	if err := os.WriteFile(path, finanzasAppHTML, 0644); err != nil {
		fmt.Println("⚠️ No se pudo escribir mapa financiero:", err)
	}

	cid, err := n.GenerarCID(finanzasAppHTML)
	if err != nil || cid == "" {
		fmt.Println("⚠️ CID mapa financiero:", err)
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
	n.agentes[finanzasAppID] = &Agente{
		ID:           finanzasAppID,
		RootCID:      cid,
		BalanceUTXO:  0,
		UltimaActual: time.Now().Unix(),
	}
	n.nombres[finanzasAlias] = finanzasAppID
	fmt.Printf("✅ Mapa financiero registrado: /w/%s (CID %s)\n", finanzasAlias, cid)
}
