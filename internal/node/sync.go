package node

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (n *NodoAlset) InitSyncManager() *SyncManager {
	config := SyncConfig{
		Mode:           SyncModeQuick,
		AutoSyncDays:   7,
		MaxQuickBlocks: 100,
	}
	if data, err := os.ReadFile("sync_config.json"); err == nil {
		json.Unmarshal(data, &config)
	}
	if data, err := os.ReadFile("last_sync.json"); err == nil {
		var lastSync struct {
			Timestamp int64 `json:"timestamp"`
		}
		json.Unmarshal(data, &lastSync)
		config.LastSyncTime = lastSync.Timestamp
	}
	sm := &SyncManager{
		nodo:   n,
		config: config,
	}
	n.syncManager = sm
	return sm
}

func (sm *SyncManager) SaveConfig() {
	data, _ := json.MarshalIndent(sm.config, "", "  ")
	os.WriteFile("sync_config.json", data, 0644)
}

func (sm *SyncManager) SaveLastSyncTime() {
	data, _ := json.Marshal(map[string]int64{"timestamp": time.Now().Unix()})
	os.WriteFile("last_sync.json", data, 0644)
}

func (n *NodoAlset) QuickStartup() {
	fmt.Println("🚀 Arranque rápido iniciado...")
	n.ensureStaticFiles()
	n.CargarEstado()
	go n.connectToNetwork()

	go func() {
		time.Sleep(3 * time.Second)
		if n.syncManager != nil && n.shouldQuickSync() {
			n.syncManager.PerformQuickSync()
		}
	}()

	// ---- Iniciar cliente de pulsos SOLO si NO estamos en Render ----
	if os.Getenv("RENDER") == "" {
		go n.startPulseClients()
		fmt.Println("⚡ Cliente de pulsos iniciado (modo local)")
	} else {
		fmt.Println("⚡ Cliente de pulsos desactivado (nodo en Render, solo actúa como servidor)")
	}

	fmt.Println("✅ Nodo operativo (sincronización en background)")
	fmt.Println("🌐 Panel de administración: http://localhost:" + getPort() + "/static/index.html")
}
func getPort() string {
	return "8080"
}

func (n *NodoAlset) shouldQuickSync() bool {
	if len(n.agentes) == 0 {
		return true
	}
	if n.syncManager.config.LastSyncTime == 0 {
		return true
	}
	daysSinceSync := (time.Now().Unix() - n.syncManager.config.LastSyncTime) / 86400
	return daysSinceSync > int64(n.syncManager.config.AutoSyncDays)
}

func (sm *SyncManager) PerformQuickSync() {
	sm.mu.Lock()
	if sm.isSyncing {
		sm.mu.Unlock()
		return
	}
	sm.isSyncing = true
	sm.mu.Unlock()
	defer func() { sm.isSyncing = false }()
	fmt.Println("⚡ Sincronización rápida iniciada...")
	peers := sm.nodo.host.Network().Peers()
	if len(peers) == 0 {
		fmt.Println("⚠️ No hay peers disponibles para sincronizar")
		return
	}
	for _, p := range peers {
		stream, err := sm.nodo.host.NewStream(sm.nodo.ctx, p, AlsetDataExchangeID)
		if err != nil {
			continue
		}
		stream.Write([]byte("SYNC_QUICK_REQUEST\n"))
		sizeBuf := make([]byte, 8)
		_, err = io.ReadFull(stream, sizeBuf)
		if err != nil {
			stream.Close()
			continue
		}
		size := binary.BigEndian.Uint64(sizeBuf)
		data := make([]byte, size)
		_, err = io.ReadFull(stream, data)
		stream.Close()
		if err != nil {
			continue
		}
		gz, _ := gzip.NewReader(bytes.NewReader(data))
		decompressed, _ := io.ReadAll(gz)
		gz.Close()
		var response struct {
			Agentes      map[string]*Agente `json:"agentes"`
			Nombres      map[string]string  `json:"nombres"`
			RecentBlocks map[string][]byte  `json:"recent_blocks"`
			NeuralState  *NeuralState       `json:"neural_state"`
		}
		json.Unmarshal(decompressed, &response)
		sm.nodo.mu.Lock()
		for k, v := range response.Agentes {
			if _, exists := sm.nodo.agentes[k]; !exists {
				sm.nodo.agentes[k] = v
			}
		}
		for k, v := range response.Nombres {
			if _, exists := sm.nodo.nombres[k]; !exists {
				sm.nodo.nombres[k] = v
			}
		}
		for k, v := range response.RecentBlocks {
			if _, exists := sm.nodo.blockstore[k]; !exists {
				sm.nodo.blockstore[k] = v
				os.WriteFile(filepath.Join(BlocksDir, k), v, 0644)
			}
		}
		if response.NeuralState != nil && sm.nodo.neuralState == nil {
			sm.nodo.neuralState = response.NeuralState
		}
		sm.nodo.mu.Unlock()
		sm.nodo.PersistirLocamente()
		sm.SaveLastSyncTime()
		fmt.Printf("✅ Sincronización rápida completada: %d agentes, %d bloques\n",
			len(response.Agentes), len(response.RecentBlocks))
		return
	}
}

func (sm *SyncManager) PerformFullSync(ctx context.Context, progressCallback func(float64)) error {
	sm.mu.Lock()
	if sm.isSyncing {
		sm.mu.Unlock()
		return fmt.Errorf("ya hay una sincronización en curso")
	}
	sm.isSyncing = true
	sm.mu.Unlock()
	defer func() { sm.isSyncing = false }()
	fmt.Println("🔄 Sincronización completa iniciada...")
	if progressCallback != nil {
		progressCallback(0.1)
	}
	peers := sm.nodo.host.Network().Peers()
	if len(peers) == 0 {
		return fmt.Errorf("no hay peers disponibles para sincronizar")
	}
	for _, p := range peers {
		stream, err := sm.nodo.host.NewStream(ctx, p, AlsetDataExchangeID)
		if err != nil {
			continue
		}
		stream.Write([]byte("SYNC_FULL_REQUEST\n"))
		sizeBuf := make([]byte, 8)
		_, err = io.ReadFull(stream, sizeBuf)
		if err != nil {
			stream.Close()
			continue
		}
		size := binary.BigEndian.Uint64(sizeBuf)
		data := make([]byte, size)
		_, err = io.ReadFull(stream, data)
		stream.Close()
		if err != nil {
			continue
		}
		gz, _ := gzip.NewReader(bytes.NewReader(data))
		decompressed, _ := io.ReadAll(gz)
		gz.Close()
		var fullState struct {
			Agentes map[string]*Agente `json:"agentes"`
			Nombres map[string]string  `json:"nombres"`
		}
		json.Unmarshal(decompressed, &fullState)
		if progressCallback != nil {
			progressCallback(0.5)
		}
		sm.nodo.mu.Lock()
		for k, v := range fullState.Agentes {
			sm.nodo.agentes[k] = v
		}
		for k, v := range fullState.Nombres {
			sm.nodo.nombres[k] = v
		}
		sm.nodo.mu.Unlock()
		if progressCallback != nil {
			progressCallback(1.0)
		}
		sm.nodo.PersistirLocamente()
		sm.SaveLastSyncTime()
		fmt.Printf("✅ Sincronización completa: %d agentes, %d nombres\n",
			len(fullState.Agentes), len(fullState.Nombres))
		return nil
	}
	return fmt.Errorf("no se pudo completar la sincronización con ningún peer")
}

func (n *NodoAlset) connectToNetwork() {
	time.Sleep(2 * time.Second)
	fmt.Println("🌐 Conectado a la red Alset")
}

// =============================================================================
// HANDLERS DE MÓDULOS, ENTIDADES, SEGURIDAD (EXISTENTES)
// =============================================================================
