package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (n *NodoAlset) getAdminPanelHTML() string {
	if len(embeddedAdminPanelHTML) > 0 {
		return string(embeddedAdminPanelHTML)
	}
	return "<!DOCTYPE html><html><body><h1>Panel no embebido</h1></body></html>"
}

func (n *NodoAlset) publishAdminPanelCID() {
	html := n.getAdminPanelHTML()
	cid, err := n.GenerarCID([]byte(html))
	if err != nil {
		fmt.Println("❌ Error generando CID del panel de administración:", err)
		return
	}
	config := NodoConfig{
		AdminPanelCID: cid,
		IsGenesis:     true,
		Version:       "4.0.0-PTEC-AN",
		LastUpdate:    time.Now().Unix(),
	}
	configBytes, _ := json.Marshal(config)
	n.GenerarCID(configBytes)
	os.WriteFile("nodo_config.json", configBytes, 0644)
	announce := map[string]string{
		"tipo": "admin_panel_announce",
		"cid":  cid,
	}
	announceBytes, _ := json.Marshal(announce)
	if n.topic != nil {
		n.topic.Publish(n.ctx, announceBytes)
	}
	fmt.Println("📢 Panel de administración publicado en IPFS con CID:", cid)
}

func (n *NodoAlset) handleAdminPanelAnnounce(update map[string]string) {
	cid := update["cid"]
	if cid == "" {
		return
	}
	panelPath := filepath.Join(StaticDir, "index.html")
	if _, err := os.Stat(panelPath); err == nil {
		return
	}
	fmt.Println("📥 Descargando panel de administración desde la red...")
	data, err := n.BuscarContenidoPorCID(cid)
	if err != nil {
		fmt.Println("❌ Error descargando panel de administración:", err)
		return
	}
	os.MkdirAll(StaticDir, 0755)
	err = os.WriteFile(panelPath, data, 0644)
	if err != nil {
		fmt.Println("❌ Error guardando panel de administración:", err)
		return
	}
	fmt.Println("✅ Panel de administración descargado y guardado en:", panelPath)
}

func (n *NodoAlset) ensureStaticFiles() {
	os.MkdirAll(StaticDir, 0755)
	os.MkdirAll(filepath.Join(StaticDir, "apps"), 0755)
	panelPath := filepath.Join(StaticDir, "index.html")
	html := n.getAdminPanelHTML()
	if err := os.WriteFile(panelPath, []byte(html), 0644); err != nil {
		fmt.Println("⚠️ No se pudo escribir panel de administración:", err)
	} else {
		fmt.Println("✅ Panel de administración listo en:", panelPath)
	}
	// Publish CID once if no local config (genesis announce)
	if _, err := os.Stat("nodo_config.json"); os.IsNotExist(err) {
		fmt.Println("🌟 Nodo genesis: publicando CID del panel de administración...")
		n.publishAdminPanelCID()
	}
}

// =============================================================================
// SISTEMA DE SINCRONIZACIÓN (EXISTENTE)
// =============================================================================
