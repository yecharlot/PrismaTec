package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	qrcode "github.com/skip2/go-qrcode"

	"redalset/internal/vero"
)

var (
	veroOnce sync.Once
	veroSvc  *vero.Service
)

func (n *NodoAlset) veroService() *vero.Service {
	veroOnce.Do(func() {
		veroSvc = vero.NewService("alset_data")
	})
	return veroSvc
}

func (n *NodoAlset) registerVero(extra map[string]http.HandlerFunc) {
	extra["/api/vero/auth/register"] = n.handleVeroRegister
	extra["/api/vero/auth/login"] = n.handleVeroLogin
	extra["/api/vero/auth/logout"] = n.handleVeroLogout
	extra["/api/vero/auth/me"] = n.handleVeroMe
	extra["/api/vero/businesses"] = n.handleVeroBusinesses
	extra["/api/vero/businesses/"] = n.handleVeroBusinessByID
	extra["/api/vero/public/"] = n.handleVeroPublic
	extra["/api/vero/track/"] = n.handleVeroTrack
	extra["/api/vero/qr/"] = n.handleVeroQR
	extra["/z/"] = n.handleVeroPublicPage
}

func writeVeroJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (n *NodoAlset) veroSession(r *http.Request) (*vero.Session, int, string) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return nil, 401, "authorization required"
	}
	tok := strings.TrimSpace(auth[7:])
	sess, ok := n.veroService().Session(tok)
	if !ok {
		return nil, 401, "session expired"
	}
	return sess, 0, ""
}

func (n *NodoAlset) handleVeroRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeVeroJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	u, sess, err := n.veroService().Register(req.Email, req.Password, req.Name)
	if err != nil {
		writeVeroJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeVeroJSON(w, 201, map[string]interface{}{"user": u, "token": sess.Token, "session": sess})
}

func (n *NodoAlset) handleVeroLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeVeroJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	u, sess, err := n.veroService().Login(req.Email, req.Password)
	if err != nil {
		writeVeroJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	writeVeroJSON(w, 200, map[string]interface{}{"user": u, "token": sess.Token, "session": sess})
}

func (n *NodoAlset) handleVeroLogout(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		n.veroService().Logout(strings.TrimSpace(auth[7:]))
	}
	writeVeroJSON(w, 200, map[string]string{"status": "ok"})
}

func (n *NodoAlset) handleVeroMe(w http.ResponseWriter, r *http.Request) {
	sess, code, msg := n.veroSession(r)
	if code != 0 {
		writeVeroJSON(w, code, map[string]string{"error": msg})
		return
	}
	list := n.veroService().Store().ListByOwner(sess.UserID)
	writeVeroJSON(w, 200, map[string]interface{}{"session": sess, "businesses": list})
}

func (n *NodoAlset) handleVeroBusinesses(w http.ResponseWriter, r *http.Request) {
	sess, code, msg := n.veroSession(r)
	if code != 0 {
		writeVeroJSON(w, code, map[string]string{"error": msg})
		return
	}
	svc := n.veroService()
	switch r.Method {
	case http.MethodGet:
		writeVeroJSON(w, 200, map[string]interface{}{"businesses": svc.Store().ListByOwner(sess.UserID)})
	case http.MethodPost:
		var in vero.CreateBusinessInput
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			writeVeroJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		b, err := svc.CreateBusiness(sess.UserID, in)
		if err != nil {
			writeVeroJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeVeroJSON(w, 201, b)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (n *NodoAlset) handleVeroBusinessByID(w http.ResponseWriter, r *http.Request) {
	sess, code, msg := n.veroSession(r)
	if code != 0 {
		writeVeroJSON(w, code, map[string]string{"error": msg})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/vero/businesses/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	svc := n.veroService()

	// products
	if len(parts) >= 2 && parts[1] == "products" {
		switch r.Method {
		case http.MethodGet:
			b, ok := svc.Store().GetBusiness(id)
			if !ok || b.OwnerUserID != sess.UserID {
				writeVeroJSON(w, 403, map[string]string{"error": "forbidden"})
				return
			}
			writeVeroJSON(w, 200, map[string]interface{}{"products": svc.Store().GetProducts(id)})
		case http.MethodPost:
			var in vero.ProductInput
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				writeVeroJSON(w, 400, map[string]string{"error": "invalid json"})
				return
			}
			p, err := svc.AddProduct(sess.UserID, id, in)
			if err != nil {
				writeVeroJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
			writeVeroJSON(w, 201, p)
		default:
			http.Error(w, "method not allowed", 405)
		}
		return
	}

	// product toggle/delete: /products/:pid
	if len(parts) >= 3 && parts[1] == "products" {
		pid := parts[2]
		if r.Method == http.MethodDelete {
			if err := svc.DeleteProduct(sess.UserID, id, pid); err != nil {
				writeVeroJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
			writeVeroJSON(w, 200, map[string]string{"status": "deleted"})
			return
		}
		if r.Method == http.MethodPatch || r.Method == http.MethodPost {
			var req struct {
				Active bool `json:"active"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if err := svc.ToggleProduct(sess.UserID, id, pid, req.Active); err != nil {
				writeVeroJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
			writeVeroJSON(w, 200, map[string]string{"status": "ok"})
			return
		}
	}

	// stats
	if len(parts) >= 2 && parts[1] == "stats" {
		b, ok := svc.Store().GetBusiness(id)
		if !ok || b.OwnerUserID != sess.UserID {
			writeVeroJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		writeVeroJSON(w, 200, svc.Store().GetStats(id))
		return
	}

	// reviews public post on business
	if len(parts) >= 2 && parts[1] == "reviews" && r.Method == http.MethodPost {
		// allow without auth for MVP simplicity — optional
		var req struct {
			Rating  int    `json:"rating"`
			Comment string `json:"comment"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			writeVeroJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		rev, err := svc.AddReview(id, req.Rating, req.Comment)
		if err != nil {
			writeVeroJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeVeroJSON(w, 201, rev)
		return
	}

	switch r.Method {
	case http.MethodGet:
		b, ok := svc.Store().GetBusiness(id)
		if !ok || b.OwnerUserID != sess.UserID {
			writeVeroJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		writeVeroJSON(w, 200, map[string]interface{}{
			"business": b,
			"products": svc.Store().GetProducts(id),
			"stats":    svc.Store().GetStats(id),
			"reviews":  svc.Store().GetReviews(id),
		})
	case http.MethodPut, http.MethodPatch:
		var in vero.CreateBusinessInput
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			writeVeroJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		b, err := svc.UpdateBusiness(sess.UserID, id, in)
		if err != nil {
			writeVeroJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeVeroJSON(w, 200, b)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (n *NodoAlset) handleVeroPublic(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/vero/public/")
	slug = strings.Trim(slug, "/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	// review via public API
	if strings.HasSuffix(slug, "/reviews") && r.Method == http.MethodPost {
		slug = strings.TrimSuffix(slug, "/reviews")
		slug = strings.Trim(slug, "/")
		b, ok := n.veroService().Store().GetBySlug(slug)
		if !ok {
			writeVeroJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		var req struct {
			Rating  int    `json:"rating"`
			Comment string `json:"comment"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			writeVeroJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		rev, err := n.veroService().AddReview(b.ID, req.Rating, req.Comment)
		if err != nil {
			writeVeroJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeVeroJSON(w, 201, rev)
		return
	}
	prof, err := n.veroService().PublicProfile(slug)
	if err != nil {
		writeVeroJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeVeroJSON(w, 200, prof)
}

func (n *NodoAlset) handleVeroTrack(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/vero/track/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		writeVeroJSON(w, 400, map[string]string{"error": "slug/event required"})
		return
	}
	n.veroService().Track(parts[0], parts[1])
	writeVeroJSON(w, 200, map[string]string{"status": "ok"})
}

func (n *NodoAlset) handleVeroQR(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/vero/qr/")
	slug = strings.Trim(slug, "/")
	if slug == "" {
		http.Error(w, "missing slug", 400)
		return
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xf := r.Header.Get("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	}
	url := fmt.Sprintf("%s://%s/z/%s", scheme, r.Host, slug)
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	n.veroService().Track(slug, "qr_scanned")
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

func (n *NodoAlset) handleVeroPublicPage(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/z/")
	slug = strings.Trim(slug, "/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	// Serve SPA public view via same app with hash — redirect to app public mode
	http.Redirect(w, r, "/w/vero.app.ans#/p/"+slug, http.StatusFound)
}
