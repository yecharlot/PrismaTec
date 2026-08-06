package node

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"redalset/internal/persistence"
)

func (n *NodoAlset) PersistirLocamente() {
	n.mu.RLock()
	defer n.mu.RUnlock()

	ctx := context.Background()
	if n.store == nil {
		dAg, _ := json.MarshalIndent(n.agentes, "", "  ")
		_ = os.WriteFile("alset_data/alset_state.json", dAg, 0644)
		dAn, _ := json.MarshalIndent(n.nombres, "", "  ")
		_ = os.WriteFile("alset_data/alset_names.json", dAn, 0644)
		n.persistirEstadoNeuronal()
		return
	}

	// Agentes → alset_agents (uno por fila)
	agentBlobs := make(map[string][]byte, len(n.agentes))
	for id, ag := range n.agentes {
		if b, err := json.Marshal(ag); err == nil {
			agentBlobs[id] = b
		}
	}
	if err := n.store.SaveAgents(ctx, agentBlobs); err != nil {
		log.Printf("⚠️ Error guardando agentes: %v", err)
	}
	// Backup KV del mapa completo (compatibilidad)
	if dAg, err := json.Marshal(n.agentes); err == nil {
		_ = n.store.Save(ctx, persistence.KeyState, dAg)
	}

	// Nombres → alset_kv
	if dAn, err := json.Marshal(n.nombres); err == nil {
		if err := n.store.Save(ctx, persistence.KeyNames, dAn); err != nil {
			log.Printf("⚠️ Error guardando nombres: %v", err)
		}
	}

	// Neural → alset_neural_state
	if n.neuralState != nil {
		if dN, err := json.Marshal(n.neuralState); err == nil {
			if err := n.store.SaveNeuralState(ctx, "main", dN); err != nil {
				log.Printf("⚠️ Error guardando neural state: %v", err)
			}
		}
	}

	// Blocks → alset_blocks
	if len(n.blockstore) > 0 {
		if err := n.store.SaveBlocks(ctx, n.blockstore); err != nil {
			log.Printf("⚠️ Error guardando blocks: %v", err)
		}
	}
}

func (n *NodoAlset) CargarEstado() {
	ctx := context.Background()

	if n.store != nil {
		// Agentes desde tabla estructurada
		if blobs, err := n.store.LoadAgents(ctx); err == nil && len(blobs) > 0 {
			n.mu.Lock()
			if n.agentes == nil {
				n.agentes = make(map[string]*Agente)
			}
			for id, raw := range blobs {
				var ag Agente
				if json.Unmarshal(raw, &ag) == nil {
					n.agentes[id] = &ag
				}
			}
			n.mu.Unlock()
		} else if d, err := n.store.Load(ctx, persistence.KeyState); err == nil && d != nil {
			// Fallback legacy KV
			n.mu.Lock()
			_ = json.Unmarshal(d, &n.agentes)
			n.mu.Unlock()
		}

		if d, err := n.store.Load(ctx, persistence.KeyNames); err == nil && d != nil {
			n.mu.Lock()
			_ = json.Unmarshal(d, &n.nombres)
			n.mu.Unlock()
		}

		if d, err := n.store.LoadNeuralState(ctx, "main"); err == nil && d != nil {
			n.mu.Lock()
			if n.neuralState == nil {
				n.neuralState = &NeuralState{}
			}
			_ = json.Unmarshal(d, n.neuralState)
			n.mu.Unlock()
		}

		if blocks, err := n.store.LoadBlocks(ctx); err == nil && len(blocks) > 0 {
			n.mu.Lock()
			n.blockstore = blocks
			n.mu.Unlock()
		}
	}

	// Complemento: bloques en disco local
	files, _ := os.ReadDir(BlocksDir)
	n.mu.Lock()
	if n.blockstore == nil {
		n.blockstore = make(map[string][]byte)
	}
	for _, f := range files {
		if !f.IsDir() {
			if _, ok := n.blockstore[f.Name()]; !ok {
				if d, err := os.ReadFile(filepath.Join(BlocksDir, f.Name())); err == nil {
					n.blockstore[f.Name()] = d
				}
			}
		}
	}
	n.mu.Unlock()
	fmt.Printf("📂 Alset Engine: %d agentes, %d nombres y %d bloques en RAM.\n", len(n.agentes), len(n.nombres), len(n.blockstore))
}

// =============================================================================
// PERSISTENCIA EN GITHUB
// =============================================================================

func (n *NodoAlset) PersistirEnGitHub() error {
	// GitHub persistence has been removed.
	// Use the new internal/persistence layer (Local or Supabase) instead.
	return nil
}

func (n *NodoAlset) CargarDesdeGitHub() error {
	// GitHub persistence has been removed.
	// Use the new internal/persistence layer (Local or Supabase) instead.
	return nil
}

func (n *NodoAlset) IpfsAddDirectory(dirPath string) (string, error) {
	files := make(map[string][]byte)
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(dirPath, path)
		files[relPath] = data
		return nil
	})
	if err != nil {
		return "", err
	}
	jsonData, _ := json.Marshal(files)
	cid, err := n.GenerarCID(jsonData)
	if err != nil {
		return "", err
	}
	fmt.Printf("📁 Directorio subido a IPFS: %s → %s\n", dirPath, cid)
	return cid, nil
}

func (n *NodoAlset) RegisterApp(appName string) (string, error) {
	appPath := filepath.Join(StaticDir, "apps", appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return "", fmt.Errorf("app no encontrada: %s", appName)
	}
	cid, err := n.IpfsAddDirectory(appPath)
	if err != nil {
		return "", err
	}
	createCmd := fmt.Sprintf(`(crear-agente "%s")`, appName)
	_, err = n.lisp.Eval(createCmd)
	if err != nil {
		return "", err
	}
	var agentID string
	n.mu.RLock()
	for id, agent := range n.agentes {
		if agent.ID == appName {
			agentID = id
			break
		}
	}
	n.mu.RUnlock()
	if agentID == "" {
		return "", fmt.Errorf("no se pudo crear el agente para: %s", appName)
	}
	setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, agentID, cid)
	_, err = n.lisp.Eval(setRootCmd)
	if err != nil {
		return "", err
	}
	registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, appName, agentID)
	_, err = n.lisp.Eval(registerCmd)
	if err != nil {
		return "", err
	}
	fmt.Printf("✅ App registrada: %s → %s (CID: %s)\n", appName, agentID, cid)
	return agentID, nil
}

// =============================================================================
// MÉTODOS DE IA DISTRIBUIDA (EXISTENTES)
// =============================================================================
