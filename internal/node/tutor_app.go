package node

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//go:embed embedded/tutor_index.html
var tutorAppHTML []byte

const tutorAppID = "app-tutor-academia"
const tutorAlias = "tutor.app.ans"

// ensureTutorApp writes Alset Tutor Academia and registers tutor.app.ans → /w/tutor.app.ans
func (n *NodoAlset) ensureTutorApp() {
	if len(tutorAppHTML) == 0 {
		fmt.Println("⚠️ Tutor Academia embed vacío")
		return
	}
	dir := filepath.Join(StaticDir, "apps", "tutor")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "index.html")
	if err := os.WriteFile(path, tutorAppHTML, 0644); err != nil {
		fmt.Println("⚠️ No se pudo escribir Tutor Academia:", err)
	}

	cid, err := n.GenerarCID(tutorAppHTML)
	if err != nil || cid == "" {
		fmt.Println("⚠️ CID Tutor Academia:", err)
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
	n.agentes[tutorAppID] = &Agente{
		ID:           tutorAppID,
		RootCID:      cid,
		BalanceUTXO:  0,
		UltimaActual: time.Now().Unix(),
	}
	n.nombres[tutorAlias] = tutorAppID
	fmt.Printf("✅ Alset Tutor Academia: /w/%s (CID %s)\n", tutorAlias, cid)
}
