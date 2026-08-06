package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (n *NodoAlset) getAdminPanelHTML() string {
	return `<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Alset Network - Panel de Administración</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%);
            color: #fff;
            min-height: 100vh;
        }
        .header {
            background: rgba(0,0,0,0.8);
            backdrop-filter: blur(10px);
            padding: 1rem 2rem;
            border-bottom: 2px solid #f4b400;
            position: sticky;
            top: 0;
            z-index: 100;
        }
        .header h1 {
            font-size: 1.5rem;
            display: inline-block;
        }
        .header .node-id {
            float: right;
            font-family: monospace;
            color: #f4b400;
            margin-top: 0.3rem;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }
        .card {
            background: rgba(20,20,40,0.9);
            backdrop-filter: blur(5px);
            border-radius: 12px;
            padding: 1.5rem;
            border: 1px solid rgba(244,180,0,0.2);
            transition: transform 0.2s, border-color 0.2s;
        }
        .card:hover {
            transform: translateY(-2px);
            border-color: rgba(244,180,0,0.5);
        }
        .card h3 {
            color: #f4b400;
            margin-bottom: 1rem;
            font-size: 0.9rem;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .card .value {
            font-size: 2.5rem;
            font-weight: bold;
            margin-bottom: 0.5rem;
        }
        .card .label {
            color: #888;
            font-size: 0.85rem;
        }
        .section {
            background: rgba(20,20,40,0.9);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
            border: 1px solid rgba(244,180,0,0.2);
        }
        .section h2 {
            color: #f4b400;
            margin-bottom: 1rem;
            font-size: 1.2rem;
        }
        .sync-btn {
            background: #f4b400;
            color: #000;
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            cursor: pointer;
            font-weight: bold;
            margin-right: 0.5rem;
            transition: opacity 0.2s;
        }
        .sync-btn:hover { opacity: 0.8; }
        .sync-progress {
            width: 100%;
            height: 20px;
            background: rgba(255,255,255,0.1);
            border-radius: 10px;
            overflow: hidden;
            margin-top: 1rem;
        }
        .sync-progress-bar {
            height: 100%;
            background: #f4b400;
            width: 0%;
            transition: width 0.3s;
        }
        .log-container {
            background: #0a0a0a;
            border-radius: 8px;
            padding: 1rem;
            max-height: 300px;
            overflow-y: auto;
            font-family: monospace;
            font-size: 0.8rem;
        }
        .log-entry {
            padding: 0.25rem 0;
            border-bottom: 1px solid rgba(255,255,255,0.05);
        }
        button {
            background: rgba(244,180,0,0.2);
            border: 1px solid #f4b400;
            color: #f4b400;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.2s;
        }
        button:hover {
            background: #f4b400;
            color: #000;
        }
        .agent-list {
            max-height: 400px;
            overflow-y: auto;
        }
        .agent-item {
            padding: 0.5rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            font-family: monospace;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        .syncing { animation: pulse 1s infinite; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🌐 Alset Network</h1>
        <div class="node-id" id="nodeId">Cargando...</div>
    </div>
    <div class="container">
        <div class="stats-grid">
            <div class="card">
                <h3>Agentes</h3>
                <div class="value" id="agentesCount">-</div>
                <div class="label">Agentes registrados en la red</div>
            </div>
            <div class="card">
                <h3>Bloques IPFS</h3>
                <div class="value" id="bloquesCount">-</div>
                <div class="label">Bloques almacenados localmente</div>
            </div>
            <div class="card">
                <h3>Peers Conectados</h3>
                <div class="value" id="peersCount">-</div>
                <div class="label">Nodos en la red</div>
            </div>
            <div class="card">
                <h3>Estado Sincronización</h3>
                <div class="value" id="syncStatus">-</div>
                <div class="label">Última sincronización</div>
            </div>
        </div>
        
        <div class="section">
            <h2>🔄 Sincronización</h2>
            <button class="sync-btn" onclick="startFullSync()">Sincronización Completa</button>
            <button class="sync-btn" onclick="startQuickSync()">Sincronización Rápida</button>
            <button onclick="refreshStatus()">Actualizar Estado</button>
            <div id="syncProgressContainer" style="display:none;">
                <div class="sync-progress">
                    <div class="sync-progress-bar" id="syncProgressBar"></div>
                </div>
                <p id="syncStatusText" style="margin-top: 0.5rem;"></p>
            </div>
        </div>
        
        <div class="section">
            <h2>📋 Agentes Registrados</h2>
            <div class="agent-list" id="agentList">
                <div>Cargando...</div>
            </div>
        </div>
        
        <div class="section">
            <h2>📊 Últimos Eventos</h2>
            <div class="log-container" id="logContainer">
                <div>Cargando...</div>
            </div>
        </div>
    </div>
    
    <script>
        let refreshInterval;
        
        async function fetchAPI(endpoint) {
            try {
                const response = await fetch(endpoint);
                return await response.json();
            } catch (error) {
                console.error('Error:', error);
                return null;
            }
        }
        
        async function refreshStatus() {
            const agentes = await fetchAPI('/api/agentes/');
            const ipfsList = await fetchAPI('/api/ipfs/list');
            const peers = await fetchAPI('/api/network/peers');
            const syncStatus = await fetchAPI('/api/sync/status');
            
            if (agentes) document.getElementById('agentesCount').innerText = Object.keys(agentes).length;
            if (ipfsList) document.getElementById('bloquesCount').innerText = ipfsList.length;
            if (peers) document.getElementById('peersCount').innerText = peers.length;
            
            if (syncStatus) {
                const lastSync = syncStatus.last_sync ? new Date(syncStatus.last_sync * 1000).toLocaleString() : 'Nunca';
                if (syncStatus.is_syncing) {
                    document.getElementById('syncStatus').innerHTML = '<span class="syncing">🔄 Sincronizando...</span>';
                } else {
                    document.getElementById('syncStatus').innerHTML = lastSync;
                }
            }
            
            if (agentes) {
                const agentListDiv = document.getElementById('agentList');
                if (Object.keys(agentes).length === 0) {
                    agentListDiv.innerHTML = '<div>No hay agentes registrados</div>';
                } else {
                    let html = '';
                    for (const [id, agent] of Object.entries(agentes)) {
                        html += '<div class="agent-item">' + id + ' - Root: ' + (agent.root_cid || 'Ninguno') + ' - Balance: ' + agent.balance_utxo + '</div>';
                    }
                    agentListDiv.innerHTML = html;
                }
            }
        }
        
        async function startFullSync() {
            document.getElementById('syncProgressContainer').style.display = 'block';
            document.getElementById('syncStatusText').innerText = 'Iniciando sincronización completa...';
            
            const response = await fetch('/api/sync/full', { method: 'POST' });
            const result = await response.json();
            
            document.getElementById('syncStatusText').innerText = result.message;
            
            const interval = setInterval(async () => {
                const status = await fetchAPI('/api/sync/status');
                if (status && status.progress) {
                    document.getElementById('syncProgressBar').style.width = (status.progress.percent * 100) + '%';
                    document.getElementById('syncStatusText').innerText = status.progress.status;
                }
                if (status && !status.is_syncing) {
                    clearInterval(interval);
                    setTimeout(() => {
                        document.getElementById('syncProgressContainer').style.display = 'none';
                    }, 2000);
                    refreshStatus();
                }
            }, 1000);
        }
        
        async function startQuickSync() {
            document.getElementById('syncProgressContainer').style.display = 'block';
            document.getElementById('syncStatusText').innerText = 'Iniciando sincronización rápida...';
            
            const response = await fetch('/api/sync/quick', { method: 'POST' });
            const result = await response.json();
            
            document.getElementById('syncStatusText').innerText = result.message;
            setTimeout(() => {
                document.getElementById('syncProgressContainer').style.display = 'none';
                refreshStatus();
            }, 3000);
        }
        
        async function loadLogs() {
            const logs = await fetchAPI('/api/audit/log');
            if (logs && logs.length > 0) {
                const logContainer = document.getElementById('logContainer');
                let html = '';
                for (let i = 0; i < Math.min(logs.length, 50); i++) {
                    const log = logs[i];
                    html += '<div class="log-entry">[' + log.ts + '] ' + log.action + ': ' + (log.detail ? log.detail.substring(0, 100) : '') + '</div>';
                }
                logContainer.innerHTML = html;
            }
        }
        
        async function loadNodeId() {
            const status = await fetchAPI('/api/sync/status');
            if (status && status.node_id) {
                document.getElementById('nodeId').innerText = status.node_id;
            }
        }
        
        refreshStatus();
        loadLogs();
        loadNodeId();
        refreshInterval = setInterval(function() {
            refreshStatus();
            loadLogs();
        }, 5000);
    </script>
</body>
</html>`
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
	if _, err := os.Stat(panelPath); err == nil {
		return
	}
	if configData, err := os.ReadFile("nodo_config.json"); err == nil {
		var config NodoConfig
		if json.Unmarshal(configData, &config) == nil && config.AdminPanelCID != "" {
			fmt.Println("📥 Restaurando panel de administración desde configuración local...")
			data, err := n.BuscarContenidoPorCID(config.AdminPanelCID)
			if err == nil {
				os.WriteFile(panelPath, data, 0644)
				fmt.Println("✅ Panel de administración restaurado")
				return
			}
		}
	}
	fmt.Println("🌟 Nodo genesis: creando panel de administración inicial...")
	n.publishAdminPanelCID()
	html := n.getAdminPanelHTML()
	os.WriteFile(panelPath, []byte(html), 0644)
	fmt.Println("✅ Panel de administración creado en:", panelPath)
}

// =============================================================================
// SISTEMA DE SINCRONIZACIÓN (EXISTENTE)
// =============================================================================
