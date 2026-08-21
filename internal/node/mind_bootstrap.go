package node

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	mindAgentID = "mind-alset"
	mindAlias   = "mind.alset.ans"
	mindAppName = "mind"
)

//go:embed embedded/mind_index.html
var embeddedMindHTML []byte

// ensureMindApp writes the Alset Mind UI and registers agent + DNS name.
func (n *NodoAlset) ensureMindApp() {
	dir := filepath.Join(StaticDir, "apps", mindAppName)
	_ = os.MkdirAll(dir, 0755)
	indexPath := filepath.Join(dir, "index.html")
	if len(embeddedMindHTML) > 0 {
		_ = os.WriteFile(indexPath, embeddedMindHTML, 0644)
	}

	// Agent + alias
	n.mu.Lock()
	if n.agentes == nil {
		n.agentes = make(map[string]*Agente)
	}
	if n.nombres == nil {
		n.nombres = make(map[string]string)
	}
	if _, ok := n.agentes[mindAgentID]; !ok {
		n.agentes[mindAgentID] = &Agente{
			ID:           mindAgentID,
			RootCID:      "",
			UltimaActual: time.Now().Unix(),
			BalanceUTXO:  0,
		}
	}
	n.nombres[mindAlias] = mindAgentID
	n.nombres["mind.alset"] = mindAgentID
	n.nombres["mind"] = mindAgentID
	n.mu.Unlock()

	// Seed self-model block
	selfModel := map[string]interface{}{
		"species":     "Alset Mind",
		"kind":        "nonconventional-ternary-field",
		"primitive":   "zyrion-0-1-2-absorbing",
		"agent_id":    mindAgentID,
		"alias":       mindAlias,
		"organs":      []string{"dialog", "act", "mem", "self", "ethics", "curiosity", "humor"},
		"thesis":      "docs/ALSET_MIND_THESIS.md",
		"born":        time.Now().UTC().Format(time.RFC3339),
		"voice":       "symbolic-field-reading",
		"sovereignty": "resident-in-node",
	}
	raw, _ := json.Marshal(selfModel)
	cid, err := n.GenerarCID(raw)
	if err == nil && cid != "" {
		n.mu.Lock()
		if a, ok := n.agentes[mindAgentID]; ok && a != nil {
			a.RootCID = cid
			a.UltimaActual = time.Now().Unix()
		}
		n.mu.Unlock()
	}

	// App name for /w/mind.app.ans
	if len(embeddedMindHTML) > 0 {
		appCID, err := n.GenerarCID(embeddedMindHTML)
		if err == nil && appCID != "" {
			appID := "app-mind-ui"
			n.mu.Lock()
			n.agentes[appID] = &Agente{
				ID:           appID,
				RootCID:      appCID,
				UltimaActual: time.Now().Unix(),
				BalanceUTXO:  0,
			}
			n.nombres["mind.app.ans"] = appID
			n.mu.Unlock()
		}
	}

	loadMindGenomeFromDisk()
	n.seedMindLispGenome()
	fmt.Println("🧠 Alset Mind: semilla lista · alias", mindAlias, "· app /w/mind.app.ans")
}

// seedMindLispGenome defines organ helpers and mind-latido in the Lisp environment.
func (n *NodoAlset) seedMindLispGenome() {
	if n.lisp == nil {
		return
	}
	// Topologies must use quote so (s1 s2 s3) is data, not a call.
	cmds := []string{
		`(defun mind-eval-organ (s1 s2 s3)
   (evaluar-zyrion
     (quote (ORG :entradas (s1 s2 s3) :salidas ((0 SEGUIR) (1 MATIZAR) (2 VETO))))
     (list (quote s1) s1 (quote s2) s2 (quote s3) s3)))`,
		`(defun mind-latido (claridad orden_nodo riesgo permiso novedad)
   (let* ((d (mind-eval-organ claridad orden_nodo riesgo))
          (a (mind-eval-organ permiso riesgo orden_nodo))
          (m (mind-eval-organ novedad claridad riesgo))
          (s (mind-eval-organ claridad riesgo permiso))
          (e (mind-eval-organ riesgo permiso orden_nodo)))
     (list
       (list "species" "Alset-Mind")
       (list "dialog" d)
       (list "act" a)
       (list "mem" m)
       (list "self" s)
       (list "ethics" e)
       (list "note" "campo-ternario-nativo"))))`,
	}
	for _, c := range cmds {
		if _, err := n.lisp.Eval(c); err != nil {
			fmt.Println("⚠️ Mind genome:", err)
		}
	}
}

