package node

import (
	"redalset/internal/pulse"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"redalset/internal/agents"
	"redalset/internal/poh"
)

func (n *NodoAlset) crearModulo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nombre    string                 `json:"nombre"`
		Rol       string                 `json:"rol"`
		Atributos map[string]interface{} `json:"atributos"`
		Owner     string                 `json:"owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	id := generarUUID()
	modulo := &Modulo{
		ID:         id,
		Nombre:     req.Nombre,
		Rol:        req.Rol,
		Atributos:  req.Atributos,
		Relaciones: []string{},
		Owner:      req.Owner,
		CreatedAt:  time.Now().Unix(),
	}
	agents.Global.MuModulos.Lock()
	agents.Global.Modulos[id] = modulo
	agents.Global.MuModulos.Unlock()
	n.Auditoria("MODULO_CREADO", fmt.Sprintf("ID: %s | Nombre: %s", id, req.Nombre))
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) listarModulos(w http.ResponseWriter, r *http.Request) {
	agents.Global.MuModulos.RLock()
	defer agents.Global.MuModulos.RUnlock()
	lista := make([]*Modulo, 0, len(agents.Global.Modulos))
	for _, m := range agents.Global.Modulos {
		lista = append(lista, m)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	agents.Global.MuModulos.RLock()
	modulo, exists := agents.Global.Modulos[id]
	agents.Global.MuModulos.RUnlock()
	if !exists {
		http.Error(w, "Módulo no encontrado", 404)
		return
	}
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) actualizarModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	agents.Global.MuModulos.Lock()
	defer agents.Global.MuModulos.Unlock()
	modulo, exists := agents.Global.Modulos[id]
	if !exists {
		http.Error(w, "Módulo no encontrado", 404)
		return
	}
	if nombre, ok := updates["nombre"]; ok {
		modulo.Nombre = nombre.(string)
	}
	if rol, ok := updates["rol"]; ok {
		modulo.Rol = rol.(string)
	}
	if atributos, ok := updates["atributos"]; ok {
		modulo.Atributos = atributos.(map[string]interface{})
	}
	json.NewEncoder(w).Encode(modulo)
}

func (n *NodoAlset) eliminarModulo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/modulos/")
	agents.Global.MuModulos.Lock()
	delete(agents.Global.Modulos, id)
	agents.Global.MuModulos.Unlock()
	n.Auditoria("MODULO_ELIMINADO", id)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (n *NodoAlset) crearEntidad(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tipo      string                 `json:"tipo"`
		Atributos map[string]interface{} `json:"atributos"`
		HeredaDe  string                 `json:"hereda_de"`
		ModuloID  string                 `json:"modulo_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	atributosFinales := make(map[string]interface{})
	if req.HeredaDe != "" {
		agents.Global.MuEntidades.RLock()
		if padre, exists := agents.Global.Entidades[req.HeredaDe]; exists {
			for k, v := range padre.Atributos {
				atributosFinales[k] = v
			}
		}
		agents.Global.MuEntidades.RUnlock()
	}
	for k, v := range req.Atributos {
		atributosFinales[k] = v
	}
	id := generarUUID()
	entidad := &EntidadProgramatica{
		ID:        id,
		Tipo:      req.Tipo,
		Atributos: atributosFinales,
		HeredaDe:  req.HeredaDe,
		ModuloID:  req.ModuloID,
	}
	agents.Global.MuEntidades.Lock()
	agents.Global.Entidades[id] = entidad
	agents.Global.MuEntidades.Unlock()
	n.Auditoria("ENTIDAD_CREADA", fmt.Sprintf("Tipo: %s | ID: %s", req.Tipo, id))
	json.NewEncoder(w).Encode(entidad)
}

func (n *NodoAlset) listarEntidades(w http.ResponseWriter, r *http.Request) {
	agents.Global.MuEntidades.RLock()
	defer agents.Global.MuEntidades.RUnlock()
	lista := make([]*EntidadProgramatica, 0, len(agents.Global.Entidades))
	for _, e := range agents.Global.Entidades {
		lista = append(lista, e)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerEntidad(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/entidades/")
	agents.Global.MuEntidades.RLock()
	entidad, exists := agents.Global.Entidades[id]
	agents.Global.MuEntidades.RUnlock()
	if !exists {
		http.Error(w, "Entidad no encontrada", 404)
		return
	}
	json.NewEncoder(w).Encode(entidad)
}

func (n *NodoAlset) crearRelacion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntidadA     string `json:"entidad_a"`
		EntidadB     string `json:"entidad_b"`
		Tipo         string `json:"tipo"`
		Cardinalidad string `json:"cardinalidad"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	id := generarUUID()
	relacion := &RelacionEntidad{
		ID:           id,
		EntidadA:     req.EntidadA,
		EntidadB:     req.EntidadB,
		Tipo:         req.Tipo,
		Cardinalidad: req.Cardinalidad,
	}
	agents.Global.Relaciones[id] = relacion
	json.NewEncoder(w).Encode(relacion)
}

func (n *NodoAlset) listarRelaciones(w http.ResponseWriter, r *http.Request) {
	lista := make([]*RelacionEntidad, 0, len(agents.Global.Relaciones))
	for _, rel := range agents.Global.Relaciones {
		lista = append(lista, rel)
	}
	json.NewEncoder(w).Encode(lista)
}

func (n *NodoAlset) obtenerRelacionesDeEntidad(w http.ResponseWriter, r *http.Request) {
	entidadID := strings.TrimPrefix(r.URL.Path, "/api/entidades/")
	entidadID = strings.TrimSuffix(entidadID, "/relaciones")
	var resultado []*RelacionEntidad
	for _, rel := range agents.Global.Relaciones {
		if rel.EntidadA == entidadID || rel.EntidadB == entidadID {
			resultado = append(resultado, rel)
		}
	}
	json.NewEncoder(w).Encode(resultado)
}

func (n *NodoAlset) generarTokenAlset(agentID string, roles []string, duracionHoras int) (*TokenAlset, error) {
	agente, exists := n.agentes[agentID]
	if !exists {
		return nil, fmt.Errorf("agente no encontrado")
	}
	tokenID := generarUUID()
	expiresAt := time.Now().Add(time.Duration(duracionHoras) * time.Hour).Unix()
	payload := fmt.Sprintf("%s|%s|%d|%s", tokenID, agentID, expiresAt, strings.Join(roles, ","))
	signature := hex.EncodeToString([]byte(payload + n.host.ID().String()))[:64]
	token := &TokenAlset{
		Token:     tokenID,
		AgentID:   agentID,
		RootCID:   agente.RootCID,
		ExpiresAt: expiresAt,
		Roles:     roles,
		Permisos:  n.rolesAPermisos(roles),
		Signature: signature,
	}
	agents.Global.MuTokens.Lock()
	agents.Global.Tokens[tokenID] = token
	agents.Global.MuTokens.Unlock()
	return token, nil
}

func (n *NodoAlset) rolesAPermisos(roles []string) []string {
	permisosMap := make(map[string]bool)
	for _, rol := range roles {
		switch rol {
		case "admin":
			permisosMap["*"] = true
		case "editor":
			permisosMap["modulo:crear"] = true
			permisosMap["modulo:editar"] = true
			permisosMap["entidad:crear"] = true
			permisosMap["entidad:editar"] = true
		case "viewer":
			permisosMap["modulo:ver"] = true
			permisosMap["entidad:ver"] = true
		case "cliente":
			permisosMap["producto:ver"] = true
			permisosMap["compra:crear"] = true
		case "vendedor":
			permisosMap["producto:crear"] = true
			permisosMap["producto:editar"] = true
			permisosMap["venta:ver"] = true
		}
	}
	var permisos []string
	for p := range permisosMap {
		permisos = append(permisos, p)
	}
	return permisos
}

func (n *NodoAlset) validarToken(tokenString string) (*TokenAlset, error) {
	agents.Global.MuTokens.RLock()
	token, exists := agents.Global.Tokens[tokenString]
	agents.Global.MuTokens.RUnlock()
	if !exists {
		return nil, fmt.Errorf("token inválido")
	}
	if time.Now().Unix() > token.ExpiresAt {
		agents.Global.MuTokens.Lock()
		delete(agents.Global.Tokens, tokenString)
		agents.Global.MuTokens.Unlock()
		return nil, fmt.Errorf("token expirado")
	}
	payload := fmt.Sprintf("%s|%s|%d|%s", token.Token, token.AgentID, token.ExpiresAt, strings.Join(token.Roles, ","))
	expectedSig := hex.EncodeToString([]byte(payload + n.host.ID().String()))[:64]
	if token.Signature != expectedSig {
		return nil, fmt.Errorf("firma inválida")
	}
	return token, nil
}

func (n *NodoAlset) verificarPermiso(token *TokenAlset, permisoRequerido string) bool {
	if token == nil {
		return false
	}
	for _, p := range token.Permisos {
		if p == "*" || p == permisoRequerido {
			return true
		}
	}
	return false
}

func (n *NodoAlset) asignarRol(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string   `json:"agent_id"`
		Roles   []string `json:"roles"`
		Modulos []string `json:"modulos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if _, exists := n.agentes[req.AgentID]; !exists {
		http.Error(w, "Agente no encontrado", 404)
		return
	}
	usuarioRoles := &UsuarioRoles{
		AgentID: req.AgentID,
		Roles:   req.Roles,
		Modulos: req.Modulos,
	}
	rolesData, _ := json.Marshal(usuarioRoles)
	cid, _ := n.GenerarCID(rolesData)
	agents.Global.Roles[req.AgentID] = req.Roles
	n.Auditoria("ROL_ASIGNADO", fmt.Sprintf("Agente: %s | Roles: %v", req.AgentID, req.Roles))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"cid":    cid,
	})
}

func (n *NodoAlset) obtenerRoles(w http.ResponseWriter, r *http.Request) {
	agentID := strings.TrimPrefix(r.URL.Path, "/api/roles/")
	roles, exists := agents.Global.Roles[agentID]
	if !exists {
		json.NewEncoder(w).Encode([]string{})
		return
	}
	json.NewEncoder(w).Encode(roles)
}

func (n *NodoAlset) crearTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID       string   `json:"agent_id"`
		Roles         []string `json:"roles"`
		DuracionHoras int      `json:"duracion_horas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	duracion := req.DuracionHoras
	if duracion <= 0 {
		duracion = 24
	}
	token, err := n.generarTokenAlset(req.AgentID, req.Roles, duracion)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(token)
}

func (n *NodoAlset) validarTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Token requerido", 400)
		return
	}
	token, err := n.validarToken(tokenStr)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	json.NewEncoder(w).Encode(token)
}

func (n *NodoAlset) revocarTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	agents.Global.MuTokens.Lock()
	delete(agents.Global.Tokens, req.Token)
	agents.Global.MuTokens.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"status": "revocado"})
}

func (n *NodoAlset) SetAgentRoot(agentID string, rootCID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if a, ok := n.agentes[agentID]; ok {
		a.RootCID = rootCID
		a.UltimaActual = time.Now().Unix()
	}
}

// =============================================================================
// HANDLERS DE SINCRONIZACIÓN HTTP (EXISTENTES)
// =============================================================================

func (n *NodoAlset) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if n.syncManager == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not_initialized",
		})
		return
	}
	n.syncManager.mu.RLock()
	defer n.syncManager.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "idle",
		"is_syncing":     n.syncManager.isSyncing,
		"last_sync":      n.syncManager.config.LastSyncTime,
		"mode":           n.syncManager.config.Mode,
		"agents_count":   len(n.agentes),
		"blocks_count":   len(n.blockstore),
		"auto_sync_days": n.syncManager.config.AutoSyncDays,
		"node_id":        n.host.ID().String(),
		"progress":       globalSyncProgress,
	})
}

func (n *NodoAlset) handleSyncFull(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if n.syncManager == nil {
		http.Error(w, "Sync manager no inicializado", 500)
		return
	}
	go func() {
		globalSyncProgress.Status = "syncing"
		globalSyncProgress.Stage = "full_sync"
		err := n.syncManager.PerformFullSync(context.Background(), func(progress float64) {
			globalSyncProgress.Percent = progress
			globalSyncProgress.Current = int(progress * 100)
			globalSyncProgress.Total = 100
		})
		if err != nil {
			globalSyncProgress.Status = "error"
			globalSyncProgress.Stage = err.Error()
		} else {
			globalSyncProgress.Status = "idle"
			globalSyncProgress.Stage = "completed"
		}
	}()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "sync_started",
		"message": "Sincronización completa iniciada en background",
	})
}

func (n *NodoAlset) handleSyncQuick(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if n.syncManager == nil {
		http.Error(w, "Sync manager no inicializado", 500)
		return
	}
	go n.syncManager.PerformQuickSync()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "sync_started",
		"message": "Sincronización rápida iniciada",
	})
}

func (n *NodoAlset) handleSyncConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var config struct {
		Mode           string `json:"mode"`
		AutoSyncDays   int    `json:"auto_sync_days"`
		MaxQuickBlocks int    `json:"max_quick_blocks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if n.syncManager != nil {
		switch config.Mode {
		case "quick":
			n.syncManager.config.Mode = SyncModeQuick
		case "full":
			n.syncManager.config.Mode = SyncModeFull
		case "incremental":
			n.syncManager.config.Mode = SyncModeIncremental
		}
		if config.AutoSyncDays > 0 {
			n.syncManager.config.AutoSyncDays = config.AutoSyncDays
		}
		if config.MaxQuickBlocks > 0 {
			n.syncManager.config.MaxQuickBlocks = config.MaxQuickBlocks
		}
		n.syncManager.SaveConfig()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "configured",
		"config": config,
	})
}

// =============================================================================
// HANDLERS HTTP ADICIONALES (EXISTENTES)
// =============================================================================

func (n *NodoAlset) handlePoHEvent(w http.ResponseWriter, r *http.Request) {
	var event PoHEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid event data", 400)
		return
	}
	poh.Global.Lock()
	if poh.Global.SessionID() == "" {
		poh.Global.SetSessionID(hex.EncodeToString([]byte(time.Now().String()))[:16])
	}
	event.Timestamp = time.Now().Unix()
	poh.Global.Append(event)
	poh.Global.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"status": "event_received"})
}

func (n *NodoAlset) handlePoHProof(w http.ResponseWriter, r *http.Request) {
	poh.Global.Lock()
	defer poh.Global.Unlock()
	if len(poh.Global.Events()) == 0 {
		http.Error(w, "No events collected", 400)
		return
	}
	var eventsData []byte
	for _, ev := range poh.Global.Events() {
		evData, _ := json.Marshal(ev)
		eventsData = append(eventsData, evData...)
	}
	hash := make([]byte, 32)
	copy(hash, eventsData[:32])
	proof := HumanityProof{
		SessionID: poh.Global.SessionID(),
		Events:    poh.Global.Events(),
		FinalSig:  hex.EncodeToString(hash),
	}
	proofBytes, _ := json.Marshal(proof)
	proofCID, _ := n.GenerarCID(proofBytes)
	poh.Global.ClearEvents()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"proof_cid": proofCID,
		"session":   proof.SessionID,
		"events":    len(proof.Events),
	})
}

func (n *NodoAlset) handleDNSList(w http.ResponseWriter, r *http.Request) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nombres": n.nombres,
	})
}

func (n *NodoAlset) handleDNSResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	n.mu.RLock()
	agentID, exists := n.nombres[req.Alias]
	n.mu.RUnlock()
	if !exists {
		http.Error(w, "Nombre no encontrado", 404)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alias":    req.Alias,
		"agent_id": agentID,
		"status":   "active",
	})
}

func (n *NodoAlset) handleDNSDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	n.mu.Lock()
	delete(n.nombres, req.Alias)
	n.mu.Unlock()
	n.PersistirLocamente()
	n.Auditoria("DNS_ELIMINADO", fmt.Sprintf("Alias: %s", req.Alias))
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"alias":  req.Alias,
	})
}

func (n *NodoAlset) handleNetworkPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if n.host == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	peers := n.host.Network().Peers()
	peerInfo := make([]map[string]interface{}, 0, len(peers))
	for _, p := range peers {
		peerInfo = append(peerInfo, map[string]interface{}{
			"id":        p.String(),
			"addresses": n.host.Network().Peerstore().Addrs(p),
			"connected": n.host.Network().Connectedness(p).String(),
		})
	}
	json.NewEncoder(w).Encode(peerInfo)
}

func (n *NodoAlset) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("audit.jsonl")
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	lines := strings.Split(string(data), "\n")
	logs := make([]map[string]interface{}, 0)
	for _, line := range lines {
		if line == "" {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
			logs = append(logs, logEntry)
		}
	}
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	json.NewEncoder(w).Encode(logs)
}

func (n *NodoAlset) handleDebugEstado(w http.ResponseWriter, r *http.Request) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agentes_count": len(n.agentes),
		"nombres_count": len(n.nombres),
		"agentes":       n.agentes,
	})
}

func (n *NodoAlset) handleAppsRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err := r.ParseMultipartForm(50 << 20)
		if err != nil {
			http.Error(w, "Error parsing form: "+err.Error(), 400)
			return
		}
		appName := r.FormValue("appName")
		if appName == "" {
			http.Error(w, "appName required", 400)
			return
		}
		appDir := filepath.Join(StaticDir, "apps", appName)
		if err := os.MkdirAll(appDir, 0755); err != nil {
			http.Error(w, "Error creating app directory: "+err.Error(), 500)
			return
		}
		files := r.MultipartForm.File["files"]
		var savedFiles []string
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue
			}
			defer file.Close()
			filename := fileHeader.Filename
			targetPath := filepath.Join(appDir, filename)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				continue
			}
			data, err := io.ReadAll(file)
			if err != nil {
				continue
			}
			if err := os.WriteFile(targetPath, data, 0644); err != nil {
				continue
			}
			savedFiles = append(savedFiles, filename)
		}
		if len(savedFiles) == 0 {
			http.Error(w, "No files were saved", 400)
			return
		}
		fmt.Printf("📁 Archivos guardados en: %s (%d archivos)\n", appDir, len(savedFiles))
		cid, err := n.IpfsAddDirectory(appDir)
		if err != nil {
			fmt.Printf("⚠️ Error uploading to IPFS: %v\n", err)
		}
		appID := fmt.Sprintf("app-%s-%d", appName, time.Now().Unix())
		createCmd := fmt.Sprintf(`(crear-agente "%s")`, appID)
		n.lisp.Eval(createCmd)
		if cid != "" {
			setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, appID, cid)
			n.lisp.Eval(setRootCmd)
		}
		registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, appName, appID)
		n.lisp.Eval(registerCmd)
		indexPath := filepath.Join(appDir, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			indexContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        html, body { width: 100%%; height: 100%%; overflow: hidden; background: #000; }
        #app { width: 100vw; height: 100vh; display: flex; }
    </style>
</head>
<body>
    <div id="app"></div>
    <script type="module" src="/apps/%s/%s.js"></script>
</body>
</html>`, appName, appName, appName)
			os.WriteFile(indexPath, []byte(indexContent), 0644)
			fmt.Printf("📄 Index.html creado: %s\n", indexPath)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "registered",
			"name":     appName,
			"cid":      cid,
			"url":      fmt.Sprintf("/w/%s.app.ans", appName),
			"agent_id": appID,
			"path":     appDir,
			"files":    len(savedFiles),
		})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), 400)
		return
	}
	if req.Name == "" {
		http.Error(w, "App name required", 400)
		return
	}
	appPath := filepath.Join(StaticDir, "apps", req.Name)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		http.Error(w, "App folder not found: "+req.Name, 404)
		return
	}
	cid, err := n.IpfsAddDirectory(appPath)
	if err != nil {
		http.Error(w, "Error uploading to IPFS: "+err.Error(), 500)
		return
	}
	appID := fmt.Sprintf("app-%s-%d", req.Name, time.Now().Unix())
	createCmd := fmt.Sprintf(`(crear-agente "%s")`, appID)
	n.lisp.Eval(createCmd)
	setRootCmd := fmt.Sprintf(`(set-agent-root "%s" "%s")`, appID, cid)
	n.lisp.Eval(setRootCmd)
	registerCmd := fmt.Sprintf(`(register-name "%s.app.ans" "%s")`, req.Name, appID)
	n.lisp.Eval(registerCmd)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "registered",
		"name":     req.Name,
		"cid":      cid,
		"url":      fmt.Sprintf("/w/%s.app.ans", req.Name),
		"agent_id": appID,
	})
}

func (n *NodoAlset) handleAppsList(w http.ResponseWriter, r *http.Request) {
	appsDir := filepath.Join(StaticDir, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	var apps []map[string]interface{}
	for _, entry := range entries {
		if entry.IsDir() {
			apps = append(apps, map[string]interface{}{
				"name": entry.Name(),
				"path": fmt.Sprintf("/static/apps/%s", entry.Name()),
			})
		}
	}
	json.NewEncoder(w).Encode(apps)
}

func (n *NodoAlset) handlePrismVerificar(w http.ResponseWriter, r *http.Request) {
	certCID := r.URL.Query().Get("cid")
	if certCID == "" {
		http.Error(w, "CID requerido", 400)
		return
	}
	certBytes, err := n.BuscarContenidoPorCID(certCID)
	if err != nil {
		n.Auditoria("VERIFICACION_FALLIDA", fmt.Sprintf("CID: %s | Motivo: No encontrado", certCID))
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Certificado no encontrado"})
		return
	}
	var vc map[string]interface{}
	if err := json.Unmarshal(certBytes, &vc); err != nil {
		n.Auditoria("VERIFICACION_ERROR", fmt.Sprintf("CID: %s | Motivo: JSON inválido", certCID))
		http.Error(w, "JSON inválido", 400)
		return
	}
	proofInterface, hasProof := vc["proof"]
	if !hasProof {
		n.Auditoria("VERIFICACION_ERROR", fmt.Sprintf("CID: %s | Motivo: Sin proof", certCID))
		http.Error(w, "Certificado sin prueba", 400)
		return
	}
	proof, ok := proofInterface.(map[string]interface{})
	if !ok {
		http.Error(w, "Proof inválido", 400)
		return
	}
	signatureStr := ""
	if pv, ok := proof["proofValue"]; ok {
		signatureStr, _ = pv.(string)
	} else if jws, ok := proof["jws"]; ok {
		signatureStr, _ = jws.(string)
	}
	if signatureStr == "" {
		http.Error(w, "Firma no encontrada", 400)
		return
	}
	firmaBytes, err := hex.DecodeString(signatureStr)
	if err != nil {
		http.Error(w, "Firma inválida", 400)
		return
	}
	vcWithoutProof := make(map[string]interface{})
	for k, v := range vc {
		if k != "proof" {
			vcWithoutProof[k] = v
		}
	}
	canonicalBytes, err := canonicalizeJSON(vcWithoutProof)
	if err != nil {
		http.Error(w, "Error canonicalizando", 500)
		return
	}
	rawKey, _ := n.masterPrivKey.GetPublic().Raw()
	pubNative := ed25519.PublicKey(rawKey)
	esFirmaValida := ed25519.Verify(pubNative, canonicalBytes, firmaBytes)
	estaRevocado := false
	motivoRevocacion := ""
	n.mu.RLock()
	for _, blockData := range n.blockstore {
		var rev map[string]interface{}
		if err := json.Unmarshal(blockData, &rev); err != nil {
			continue
		}
		if revType, ok := rev["type"]; ok {
			var typeStr string
			switch t := revType.(type) {
			case string:
				typeStr = t
			case []interface{}:
				if len(t) > 0 {
					if s, ok := t[0].(string); ok {
						typeStr = s
					}
				}
			}
			if typeStr == "RevocationList2020Credential" {
				if subject, ok := rev["credentialSubject"].(map[string]interface{}); ok {
					if revokedCID, ok := subject["revokedCredential"].(string); ok && revokedCID == certCID {
						estaRevocado = true
						if reason, ok := subject["revocationReason"].(string); ok {
							motivoRevocacion = reason
						}
						break
					}
				}
			}
		}
	}
	n.mu.RUnlock()
	statusVerdad := "AUTÉNTICO"
	auditAccion := "VC_VERIFICADO_OK"
	if !esFirmaValida {
		statusVerdad = "FALSIFICADO / INVÁLIDO"
		auditAccion = "ALERTA_FRAUDE_FIRMA"
	} else if estaRevocado {
		statusVerdad = "REVOCADO"
		auditAccion = "CONSULTA_VC_REVOCADO"
	}
	n.Auditoria(auditAccion, fmt.Sprintf("CID: %s | Status: %s", certCID, statusVerdad))
	credentialSubject := map[string]interface{}{}
	if cs, ok := vc["credentialSubject"]; ok {
		if csMap, ok := cs.(map[string]interface{}); ok {
			credentialSubject = csMap
		}
	}
	response := map[string]interface{}{
		"cid_consultado":  certCID,
		"status_integral": statusVerdad,
		"firma_valida":    esFirmaValida,
		"revocado":        estaRevocado,
		"info_revocacion": map[string]string{"motivo": motivoRevocacion},
		"detalles": map[string]interface{}{
			"issuer":            vc["issuer"],
			"issuanceDate":      vc["issuanceDate"],
			"credentialSubject": credentialSubject,
		},
		"nodo_verificador": n.host.ID().String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *NodoAlset) handlePrismRevocar(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		CID    string `json:"cid"`
		Motivo string `json:"motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if req.CID == "" {
		http.Error(w, "CID requerido", 400)
		return
	}
	if req.Motivo == "" {
		req.Motivo = "Revocado por administrador"
	}
	_, err := n.BuscarContenidoPorCID(req.CID)
	if err != nil {
		http.Error(w, "Certificado no encontrado", 404)
		return
	}
	fecha := time.Now().Format(time.RFC3339)
	mensaje := fmt.Sprintf("REVOKE|%s|%s", req.CID, fecha)
	firma, err := n.masterPrivKey.Sign([]byte(mensaje))
	if err != nil {
		http.Error(w, "Error firmando", 500)
		return
	}
	revocationTicket := map[string]interface{}{
		"@context":     "https://www.w3.org/2018/credentials/v1",
		"id":           fmt.Sprintf("urn:uuid:%s", req.CID),
		"type":         []string{"RevocationList2020Credential"},
		"issuer":       "did:prism:tec:institutional",
		"issuanceDate": fecha,
		"credentialSubject": map[string]interface{}{
			"id":                fmt.Sprintf("did:prism:%s", req.CID[:16]),
			"revokedCredential": req.CID,
			"revocationReason":  req.Motivo,
			"revocationDate":    fecha,
		},
		"proof": map[string]interface{}{
			"type":               "Ed25519Signature2020",
			"created":            fecha,
			"verificationMethod": "did:prism:tec:institutional#key-1",
			"proofPurpose":       "assertionMethod",
			"jws":                hex.EncodeToString(firma),
		},
	}
	ticketBytes, _ := json.Marshal(revocationTicket)
	revokeCID, err := n.GenerarCID(ticketBytes)
	if err != nil {
		http.Error(w, "Error generando CID", 500)
		return
	}
	n.Auditoria("CERTIFICADO_REVOCADO",
		fmt.Sprintf("Cert: %s | Motivo: %s", req.CID, req.Motivo))
	update := map[string]string{"tipo": "revocacion_update", "cid": revokeCID}
	msg, _ := json.Marshal(update)
	n.topic.Publish(n.ctx, msg)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":               "revocado",
		"ticket_cid":           revokeCID,
		"certificado_revocado": req.CID,
		"fecha":                fecha,
	})
}

func (n *NodoAlset) handlePrismSellar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CID string `json:"cid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	res, _ := n.lisp.Eval(fmt.Sprintf(`(sellar-documento "%s")`, req.CID))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "Certificado Generado",
		"entidad":         "Prism@.TEC - Garante de la Verdad Digital",
		"titular":         "Dayanis Pérez Soria",
		"certificado_cid": res,
	})
}

func (n *NodoAlset) handleAdminUpdatePass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NuevaClave string `json:"nuevaClave"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte(req.NuevaClave), bcrypt.DefaultCost)
	config := NodoConfig{
		AdminPassHash: string(hashedPass),
		LastUpdate:    time.Now().Unix(),
		Version:       "4.0.0-PTEC-AN",
	}
	configBytes, _ := json.Marshal(config)
	cidStr, _ := n.GenerarCID(configBytes)
	n.Auditoria("SEGURIDAD_PASSWORD_UPDATE", fmt.Sprintf("Nuevo CID de config: %s", cidStr))
	n.AnunciarNuevoBloque(cidStr)
	fmt.Printf("🔒 [SEGURIDAD] Nueva configuración sellada con CID: %s\n", cidStr)
	json.NewEncoder(w).Encode(map[string]string{"config_cid": cidStr})
}

func (n *NodoAlset) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CID   string `json:"config_cid"`
		Clave string `json:"clave"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Solicitud inválida", 400)
		return
	}
	configBytes, err := n.BuscarContenidoPorCID(req.CID)
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Configuración no encontrada"})
		return
	}
	var config NodoConfig
	json.Unmarshal(configBytes, &config)
	err = bcrypt.CompareHashAndPassword([]byte(config.AdminPassHash), []byte(req.Clave))
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Clave incorrecta"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "authorized", "node": n.host.ID().String()})
}

// =============================================================================
// SERVICIO DE PULSOS – NUEVO
// =============================================================================

func (n *NodoAlset) broadcastPulse(eventType string, data interface{}) {
	payload, _ := json.Marshal(data)
	msg := pulse.FormatSSE(eventType, payload)

	n.pulseSubscribersMu.RLock()
	defer n.pulseSubscribersMu.RUnlock()
	for sub := range n.pulseSubscribers {
		select {
		case sub.ch <- msg:
		default:
			// canal lleno, omitir
		}
	}
}
