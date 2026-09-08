package node

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const finanzasEngageFile = "data/finanzas_engagement.json"
const finanzasMaxComments = 200
const finanzasMaxName = 80
const finanzasMaxBody = 1200

type finanzasComment struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

type finanzasEngageStore struct {
	Visits           int64             `json:"visits"`
	PrismatecVisits    int64             `json:"prismatec_visits"`
	PrismatecComments []finanzasComment `json:"prismatec_comments"`
	Comments           []finanzasComment `json:"comments"`
}

var (
	finanzasEngageMu sync.Mutex
)

func finanzasEngagePath() string {
	return filepath.Join(".", finanzasEngageFile)
}

func loadFinanzasEngage() finanzasEngageStore {
	var st finanzasEngageStore
	b, err := os.ReadFile(finanzasEngagePath())
	if err != nil {
		return st
	}
	_ = json.Unmarshal(b, &st)
	if st.Comments == nil {
		st.Comments = []finanzasComment{}
	}
	if st.PrismatecComments == nil {
		st.PrismatecComments = []finanzasComment{}
	}
	return st
}

func saveFinanzasEngage(st finanzasEngageStore) error {
	dir := filepath.Dir(finanzasEngagePath())
	_ = os.MkdirAll(dir, 0o755)
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(finanzasEngagePath(), b, 0o644)
}

func finanzasEngageCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

// GET /api/finanzas/stats  — visits + comment count
// POST /api/finanzas/visit — increment visit (once per call; client should call once per session)
func (n *NodoAlset) handleFinanzasStats(w http.ResponseWriter, r *http.Request) {
	finanzasEngageCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	finanzasEngageMu.Lock()
	defer finanzasEngageMu.Unlock()
	st := loadFinanzasEngage()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":            true,
		"visits":        st.Visits,
		"comments":      len(st.Comments),
		"comments_list": st.Comments,
	})
}

func (n *NodoAlset) handleFinanzasVisit(w http.ResponseWriter, r *http.Request) {
	finanzasEngageCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "POST required"})
		return
	}
	finanzasEngageMu.Lock()
	defer finanzasEngageMu.Unlock()
	st := loadFinanzasEngage()
	st.Visits++
	_ = saveFinanzasEngage(st)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "visits": st.Visits})
}

// GET/POST /api/finanzas/comments
func (n *NodoAlset) handleFinanzasComments(w http.ResponseWriter, r *http.Request) {
	finanzasEngageCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	finanzasEngageMu.Lock()
	defer finanzasEngageMu.Unlock()
	st := loadFinanzasEngage()

	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"comments": st.Comments,
			"count":    len(st.Comments),
		})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "GET or POST"})
		return
	}

	var in struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "JSON inválido"})
		return
	}
	name := strings.TrimSpace(in.Name)
	body := strings.TrimSpace(in.Body)
	if name == "" {
		name = "Anónimo"
	}
	if utf8.RuneCountInString(name) > finanzasMaxName {
		name = string([]rune(name)[:finanzasMaxName])
	}
	if body == "" {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "Escribe un comentario"})
		return
	}
	if utf8.RuneCountInString(body) > finanzasMaxBody {
		body = string([]rune(body)[:finanzasMaxBody])
	}
	// light sanitize: strip tags
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	body = strings.ReplaceAll(body, "<", "")
	body = strings.ReplaceAll(body, ">", "")

	c := finanzasComment{
		ID:      time.Now().UTC().Format("20060102T150405.000"),
		Name:    name,
		Body:    body,
		Created: time.Now().UTC().Format(time.RFC3339),
	}
	st.Comments = append([]finanzasComment{c}, st.Comments...)
	if len(st.Comments) > finanzasMaxComments {
		st.Comments = st.Comments[:finanzasMaxComments]
	}
	if err := saveFinanzasEngage(st); err != nil {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "No se pudo guardar"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "comment": c, "count": len(st.Comments)})
}



func (n *NodoAlset) handlePrismatecStats(w http.ResponseWriter, r *http.Request) {
	finanzasEngageCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	finanzasEngageMu.Lock()
	defer finanzasEngageMu.Unlock()
	st := loadFinanzasEngage()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"visits": st.PrismatecVisits,
	})
}

func (n *NodoAlset) handlePrismatecVisit(w http.ResponseWriter, r *http.Request) {
	finanzasEngageCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "POST required"})
		return
	}
	finanzasEngageMu.Lock()
	defer finanzasEngageMu.Unlock()
	st := loadFinanzasEngage()
	st.PrismatecVisits++
	_ = saveFinanzasEngage(st)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "visits": st.PrismatecVisits})
}


func (n *NodoAlset) handlePrismatecComments(w http.ResponseWriter, r *http.Request) {
	finanzasEngageCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	finanzasEngageMu.Lock()
	defer finanzasEngageMu.Unlock()
	st := loadFinanzasEngage()

	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"comments": st.PrismatecComments,
			"count":    len(st.PrismatecComments),
		})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "GET or POST"})
		return
	}

	var in struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "JSON inválido"})
		return
	}
	name := strings.TrimSpace(in.Name)
	body := strings.TrimSpace(in.Body)
	if name == "" {
		name = "Anónimo"
	}
	if utf8.RuneCountInString(name) > finanzasMaxName {
		name = string([]rune(name)[:finanzasMaxName])
	}
	if body == "" {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "Escribe un comentario"})
		return
	}
	if utf8.RuneCountInString(body) > finanzasMaxBody {
		body = string([]rune(body)[:finanzasMaxBody])
	}
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	body = strings.ReplaceAll(body, "<", "")
	body = strings.ReplaceAll(body, ">", "")

	c := finanzasComment{
		ID:      time.Now().UTC().Format("20060102T150405.000"),
		Name:    name,
		Body:    body,
		Created: time.Now().UTC().Format(time.RFC3339),
	}
	st.PrismatecComments = append([]finanzasComment{c}, st.PrismatecComments...)
	if len(st.PrismatecComments) > finanzasMaxComments {
		st.PrismatecComments = st.PrismatecComments[:finanzasMaxComments]
	}
	if err := saveFinanzasEngage(st); err != nil {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "No se pudo guardar"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "comment": c, "count": len(st.PrismatecComments)})
}

func (n *NodoAlset) registerFinanzasEngagement(extra map[string]http.HandlerFunc) {
	extra["/api/finanzas/stats"] = n.handleFinanzasStats
	extra["/api/finanzas/visit"] = n.handleFinanzasVisit
	extra["/api/finanzas/comments"] = n.handleFinanzasComments
	extra["/api/prismatec/stats"] = n.handlePrismatecStats
	extra["/api/prismatec/visit"] = n.handlePrismatecVisit
	extra["/api/prismatec/comments"] = n.handlePrismatecComments
}
