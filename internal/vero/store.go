package vero

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Store struct {
	mu       sync.RWMutex
	dir      string
	users    map[string]User            // email -> user
	sessions map[string]Session         // token -> session
	byID     map[string]Business        // id -> business
	bySlug   map[string]string          // slug -> id
	products map[string][]Product       // businessID -> products
	reviews  map[string][]Review        // businessID -> reviews
	stats    map[string]Stats           // businessID -> stats
}

func NewStore(dataDir string) *Store {
	if dataDir == "" {
		dataDir = "alset_data"
	}
	dir := filepath.Join(dataDir, "vero")
	_ = os.MkdirAll(dir, 0755)
	s := &Store{
		dir:      dir,
		users:    map[string]User{},
		sessions: map[string]Session{},
		byID:     map[string]Business{},
		bySlug:   map[string]string{},
		products: map[string][]Product{},
		reviews:  map[string][]Review{},
		stats:    map[string]Stats{},
	}
	s.load()
	return s
}

func (s *Store) path(name string) string { return filepath.Join(s.dir, name) }

func (s *Store) load() {
	s.loadJSON("users.json", &s.users)
	s.loadJSON("sessions.json", &s.sessions)
	s.loadJSON("businesses.json", &s.byID)
	s.loadJSON("products.json", &s.products)
	s.loadJSON("reviews.json", &s.reviews)
	s.loadJSON("stats.json", &s.stats)
	s.bySlug = map[string]string{}
	for id, b := range s.byID {
		s.bySlug[strings.ToLower(b.Slug)] = id
	}
}

func (s *Store) loadJSON(name string, dest interface{}) {
	b, err := os.ReadFile(s.path(name))
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, dest)
}

func (s *Store) saveJSON(name string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(name), b, 0600)
}

func (s *Store) persistAll() error {
	if err := s.saveJSON("users.json", s.users); err != nil {
		return err
	}
	if err := s.saveJSON("sessions.json", s.sessions); err != nil {
		return err
	}
	if err := s.saveJSON("businesses.json", s.byID); err != nil {
		return err
	}
	if err := s.saveJSON("products.json", s.products); err != nil {
		return err
	}
	if err := s.saveJSON("reviews.json", s.reviews); err != nil {
		return err
	}
	return s.saveJSON("stats.json", s.stats)
}

func (s *Store) PutUser(u User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[strings.ToLower(u.Email)] = u
	return s.saveJSON("users.json", s.users)
}

func (s *Store) GetUser(email string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[strings.ToLower(email)]
	return u, ok
}

func (s *Store) PutSession(sess Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.Token] = sess
	return s.saveJSON("sessions.json", s.sessions)
}

func (s *Store) GetSession(token string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[token]
	return sess, ok
}

func (s *Store) DeleteSession(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	_ = s.saveJSON("sessions.json", s.sessions)
	s.mu.Unlock()
}

func (s *Store) PutBusiness(b Business) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.byID[b.ID]; ok && old.Slug != b.Slug {
		delete(s.bySlug, strings.ToLower(old.Slug))
	}
	// unique slug
	slug := strings.ToLower(b.Slug)
	if other, ok := s.bySlug[slug]; ok && other != b.ID {
		return fmt.Errorf("slug already taken")
	}
	s.byID[b.ID] = b
	s.bySlug[slug] = b.ID
	return s.saveJSON("businesses.json", s.byID)
}

func (s *Store) GetBusiness(id string) (Business, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.byID[id]
	return b, ok
}

func (s *Store) GetBySlug(slug string) (Business, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.bySlug[strings.ToLower(slug)]
	if !ok {
		return Business{}, false
	}
	b, ok := s.byID[id]
	return b, ok
}

func (s *Store) ListByOwner(userID string) []Business {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Business{}
	for _, b := range s.byID {
		if b.OwnerUserID == userID {
			out = append(out, b)
		}
	}
	return out
}

func (s *Store) SetProducts(businessID string, list []Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[businessID] = list
	return s.saveJSON("products.json", s.products)
}

func (s *Store) GetProducts(businessID string) []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Product(nil), s.products[businessID]...)
}

func (s *Store) AddReview(r Review) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reviews[r.BusinessID] = append(s.reviews[r.BusinessID], r)
	return s.saveJSON("reviews.json", s.reviews)
}

func (s *Store) GetReviews(businessID string) []Review {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Review(nil), s.reviews[businessID]...)
}

func (s *Store) IncrStat(businessID, kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats[businessID]
	switch kind {
	case "profile_viewed":
		st.ProfileViews++
	case "catalog_viewed":
		st.CatalogViews++
	case "product_viewed":
		st.ProductViews++
	case "whatsapp_clicked":
		st.WhatsAppClicks++
	case "qr_scanned":
		st.QRScans++
	case "business_shared":
		st.Shares++
	}
	s.stats[businessID] = st
	_ = s.saveJSON("stats.json", s.stats)
}

func (s *Store) GetStats(businessID string) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats[businessID]
}
