package httpapi

import (
	"net/http"
)

// Handlers holds every HTTP endpoint implementation.
// Package node fills this struct; path registration lives only here.
type Handlers struct {
	StaticDir string

	// Public / static
	Index   http.HandlerFunc // /
	WebApp  http.HandlerFunc // /w/
	AppsFS  http.HandlerFunc // /apps/

	// Pulse
	PulseSSE  http.HandlerFunc // /api/pulse
	PulseEmit http.HandlerFunc // /api/pulse/emit

	// Legacy github sync stubs
	GitHubSave   http.HandlerFunc
	GitHubLoad   http.HandlerFunc
	GitHubSync   http.HandlerFunc
	GitHubStatus http.HandlerFunc

	// Core (also available via Backend/MountCore)
	CreateAgent  http.HandlerFunc
	ListAgents   http.HandlerFunc
	DeleteAgent  http.HandlerFunc
	ModifyAgent  http.HandlerFunc
	ListBlocks   http.HandlerFunc
	FetchBlock   http.HandlerFunc
	AddBlock     http.HandlerFunc
	DeleteBlock  http.HandlerFunc
	GetBlock     http.HandlerFunc
	ClearBlocks  http.HandlerFunc
	ListPeers    http.HandlerFunc
	ListDNS      http.HandlerFunc
	ResolveDNS   http.HandlerFunc
	DeleteDNS    http.HandlerFunc

	// Audit / debug
	AuditLog    http.HandlerFunc
	DebugEstado http.HandlerFunc

	// Prism / admin
	PrismVerify     http.HandlerFunc
	PrismRevoke     http.HandlerFunc
	PrismSeal       http.HandlerFunc
	AdminUpdatePass http.HandlerFunc
	AdminLogin      http.HandlerFunc

	// Apps / Lisp
	AppsRegister http.HandlerFunc
	AppsList     http.HandlerFunc
	LispAI       http.HandlerFunc

	// PoH / sync
	PoHEvent    http.HandlerFunc
	PoHProof    http.HandlerFunc
	SyncStatus  http.HandlerFunc
	SyncFull    http.HandlerFunc
	SyncQuick   http.HandlerFunc
	SyncConfig  http.HandlerFunc

	// Neural / IA
	IAConfigurar http.HandlerFunc
	IAInferir    http.HandlerFunc
	IAEstado     http.HandlerFunc
	IASinapsis   http.HandlerFunc
	IAMemoria    http.HandlerFunc

	// Modules / entities / tokens (from modules.go registration if any)
	Extra map[string]http.HandlerFunc
}

// Mount registers all routes on mux. Missing HandlerFuncs are skipped.
func Mount(mux *http.ServeMux, h Handlers) {
	if h.StaticDir != "" {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(h.StaticDir))))
	}

	bind := func(path string, fn http.HandlerFunc) {
		if fn != nil {
			mux.HandleFunc(path, fn)
		}
	}

	bind("/", h.Index)
	bind("/w/", h.WebApp)
	bind("/apps/", h.AppsFS)

	bind("/api/pulse", h.PulseSSE)
	bind("/api/pulse/emit", h.PulseEmit)

	bind("/api/sync/github/save", h.GitHubSave)
	bind("/api/sync/github/load", h.GitHubLoad)
	bind("/api/sync/github", h.GitHubSync)
	bind("/api/sync/github/status", h.GitHubStatus)

	bind("/api/crear-agente", h.CreateAgent)
	bind("/api/agentes/", h.ListAgents)
	bind("/api/eliminar-agente", h.DeleteAgent)
	bind("/api/modificar-agente", h.ModifyAgent)

	bind("/api/ipfs/list", h.ListBlocks)
	bind("/api/ipfs/fetch", h.FetchBlock)
	bind("/api/ipfs/add", h.AddBlock)
	bind("/api/ipfs/delete", h.DeleteBlock)
	bind("/api/ipfs/get", h.GetBlock)
	bind("/api/ipfs/clear", h.ClearBlocks)

	bind("/api/network/peers", h.ListPeers)
	bind("/api/dns/list", h.ListDNS)
	bind("/api/dns/resolve", h.ResolveDNS)
	bind("/api/dns/delete", h.DeleteDNS)

	bind("/api/audit/log", h.AuditLog)
	bind("/api/debug/estado", h.DebugEstado)

	bind("/api/prism/verificar", h.PrismVerify)
	bind("/api/prism/revocar", h.PrismRevoke)
	bind("/api/prism/sellar", h.PrismSeal)
	bind("/api/admin/update-pass", h.AdminUpdatePass)
	bind("/api/admin/login", h.AdminLogin)

	bind("/api/apps/register", h.AppsRegister)
	bind("/api/apps/list", h.AppsList)
	bind("/api/lispai", h.LispAI)

	bind("/api/poh/event", h.PoHEvent)
	bind("/api/poh/proof", h.PoHProof)
	bind("/api/sync/status", h.SyncStatus)
	bind("/api/sync/full", h.SyncFull)
	bind("/api/sync/quick", h.SyncQuick)
	bind("/api/sync/config", h.SyncConfig)

	bind("/api/ia/configurar", h.IAConfigurar)
	bind("/api/ia/inferir", h.IAInferir)
	bind("/api/ia/estado", h.IAEstado)
	bind("/api/ia/sinapsis", h.IASinapsis)
	bind("/api/ia/memoria", h.IAMemoria)

	for path, fn := range h.Extra {
		bind(path, fn)
	}
}
