package vero

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Service struct {
	store *Store
}

func NewService(dataDir string) *Service {
	return &Service{store: NewStore(dataDir)}
}

func (s *Service) Store() *Store { return s.store }

func rid(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}

func veroPublicID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "VERO-" + strings.ToUpper(hex.EncodeToString(b[:]))
}

func slugify(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return rid("biz")
	}
	return out
}

func (s *Service) Register(email, password, name string) (*User, *Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, nil, fmt.Errorf("email inválido")
	}
	if len(password) < 6 {
		return nil, nil, fmt.Errorf("contraseña mínima 6 caracteres")
	}
	if _, ok := s.store.GetUser(email); ok {
		return nil, nil, fmt.Errorf("email ya registrado")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}
	u := User{
		ID: rid("usr"), Email: email, Name: strings.TrimSpace(name),
		PasswordHash: string(hash), CreatedAt: time.Now().UTC(), Active: true,
	}
	if u.Name == "" {
		u.Name = strings.Split(email, "@")[0]
	}
	if err := s.store.PutUser(u); err != nil {
		return nil, nil, err
	}
	sess, err := s.createSession(&u)
	out := u
	out.PasswordHash = ""
	return &out, sess, err
}

func (s *Service) Login(email, password string) (*User, *Session, error) {
	u, ok := s.store.GetUser(email)
	if !ok || !u.Active {
		return nil, nil, fmt.Errorf("credenciales inválidas")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, nil, fmt.Errorf("credenciales inválidas")
	}
	sess, err := s.createSession(&u)
	out := u
	out.PasswordHash = ""
	return &out, sess, err
}

func (s *Service) createSession(u *User) (*Session, error) {
	var b [16]byte
	_, _ = rand.Read(b[:])
	sess := Session{
		Token: hex.EncodeToString(b[:]), UserID: u.ID, Email: u.Email, Name: u.Name,
		ExpiresAt: time.Now().UTC().Add(72 * time.Hour),
	}
	if err := s.store.PutSession(sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Service) Session(token string) (*Session, bool) {
	sess, ok := s.store.GetSession(token)
	if !ok || time.Now().UTC().After(sess.ExpiresAt) {
		return nil, false
	}
	return &sess, true
}

func (s *Service) Logout(token string) { s.store.DeleteSession(token) }

type CreateBusinessInput struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Phone    string `json:"phone"`
	WhatsApp string `json:"whatsapp"`
	Category string `json:"category"`
	City     string `json:"city"`
	Zone     string `json:"zone"`
	Bio      string `json:"bio"`
}

func (s *Service) CreateBusiness(ownerID string, in CreateBusinessInput) (*Business, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("nombre del negocio requerido")
	}
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	if slug == "" {
		slug = slugify(name)
	}
	if !slugRe.MatchString(slug) {
		return nil, fmt.Errorf("slug inválido (usa minúsculas, números y guiones)")
	}
	if _, taken := s.store.GetBySlug(slug); taken {
		return nil, fmt.Errorf("ese enlace ya está en uso: %s", slug)
	}
	wa := strings.TrimSpace(in.WhatsApp)
	if wa == "" {
		wa = strings.TrimSpace(in.Phone)
	}
	now := time.Now().UTC()
	b := Business{
		ID: rid("biz"), ZyrID: veroPublicID(), Slug: slug, Name: name,
		OwnerUserID: ownerID, Phone: strings.TrimSpace(in.Phone), WhatsApp: wa,
		Category: strings.TrimSpace(in.Category), City: strings.TrimSpace(in.City),
		Zone: strings.TrimSpace(in.Zone), Bio: strings.TrimSpace(in.Bio),
		VerificationLevel: 0, Plan: "free", Published: true,
		Score: 40, CreatedAt: now, UpdatedAt: now,
	}
	b.Score = ComputeScore(&b, nil, Stats{})
	if err := s.store.PutBusiness(b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) UpdateBusiness(ownerID string, id string, in CreateBusinessInput) (*Business, error) {
	b, ok := s.store.GetBusiness(id)
	if !ok {
		return nil, fmt.Errorf("negocio no encontrado")
	}
	if b.OwnerUserID != ownerID {
		return nil, fmt.Errorf("forbidden")
	}
	if n := strings.TrimSpace(in.Name); n != "" {
		b.Name = n
	}
	if in.Phone != "" {
		b.Phone = strings.TrimSpace(in.Phone)
	}
	if in.WhatsApp != "" {
		b.WhatsApp = strings.TrimSpace(in.WhatsApp)
	}
	if in.Category != "" {
		b.Category = strings.TrimSpace(in.Category)
	}
	if in.City != "" {
		b.City = strings.TrimSpace(in.City)
	}
	if in.Zone != "" {
		b.Zone = strings.TrimSpace(in.Zone)
	}
	if in.Bio != "" {
		b.Bio = strings.TrimSpace(in.Bio)
	}
	if in.Slug != "" && strings.ToLower(in.Slug) != b.Slug {
		slug := strings.ToLower(strings.TrimSpace(in.Slug))
		if !slugRe.MatchString(slug) {
			return nil, fmt.Errorf("slug inválido")
		}
		if other, ok := s.store.GetBySlug(slug); ok && other.ID != b.ID {
			return nil, fmt.Errorf("slug en uso")
		}
		b.Slug = slug
	}
	b.UpdatedAt = time.Now().UTC()
	revs := s.store.GetReviews(b.ID)
	st := s.store.GetStats(b.ID)
	b.Score = ComputeScore(&b, revs, st)
	if err := s.store.PutBusiness(b); err != nil {
		return nil, err
	}
	return &b, nil
}

type ProductInput struct {
	Title    string  `json:"title"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Active   *bool   `json:"active"`
}

func (s *Service) AddProduct(ownerID, businessID string, in ProductInput) (*Product, error) {
	b, ok := s.store.GetBusiness(businessID)
	if !ok || b.OwnerUserID != ownerID {
		return nil, fmt.Errorf("forbidden")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("título requerido")
	}
	cur := in.Currency
	if cur == "" {
		cur = "USD"
	}
	list := s.store.GetProducts(businessID)
	// free plan soft limit
	if b.Plan == "free" && len(list) >= 15 {
		return nil, fmt.Errorf("plan free: máximo 15 productos — mejora a Pro")
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	p := Product{
		ID: rid("prd"), BusinessID: businessID, Title: title, Price: in.Price,
		Currency: cur, Active: active, Sort: len(list), CreatedAt: time.Now().UTC(),
	}
	list = append(list, p)
	if err := s.store.SetProducts(businessID, list); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) ToggleProduct(ownerID, businessID, productID string, active bool) error {
	b, ok := s.store.GetBusiness(businessID)
	if !ok || b.OwnerUserID != ownerID {
		return fmt.Errorf("forbidden")
	}
	list := s.store.GetProducts(businessID)
	found := false
	for i := range list {
		if list[i].ID == productID {
			list[i].Active = active
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("producto no encontrado")
	}
	return s.store.SetProducts(businessID, list)
}

func (s *Service) DeleteProduct(ownerID, businessID, productID string) error {
	b, ok := s.store.GetBusiness(businessID)
	if !ok || b.OwnerUserID != ownerID {
		return fmt.Errorf("forbidden")
	}
	list := s.store.GetProducts(businessID)
	out := list[:0]
	for _, p := range list {
		if p.ID != productID {
			out = append(out, p)
		}
	}
	return s.store.SetProducts(businessID, out)
}

func (s *Service) AddReview(businessID string, rating int, comment string) (*Review, error) {
	if _, ok := s.store.GetBusiness(businessID); !ok {
		return nil, fmt.Errorf("negocio no encontrado")
	}
	if rating < 1 || rating > 5 {
		return nil, fmt.Errorf("rating 1-5")
	}
	r := Review{
		ID: rid("rev"), BusinessID: businessID, Rating: rating,
		Comment: strings.TrimSpace(comment), Status: "visible",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.AddReview(r); err != nil {
		return nil, err
	}
	s.recomputeBusiness(businessID)
	return &r, nil
}

func (s *Service) recomputeBusiness(id string) {
	b, ok := s.store.GetBusiness(id)
	if !ok {
		return
	}
	revs := s.store.GetReviews(id)
	var sum, n int
	for _, r := range revs {
		if r.Status != "visible" {
			continue
		}
		sum += r.Rating
		n++
	}
	b.ReviewCount = n
	if n > 0 {
		b.RatingAvg = float64(sum) / float64(n)
	}
	b.Score = ComputeScore(&b, revs, s.store.GetStats(id))
	b.UpdatedAt = time.Now().UTC()
	_ = s.store.PutBusiness(b)
}

func (s *Service) PublicProfile(slug string) (map[string]interface{}, error) {
	b, ok := s.store.GetBySlug(slug)
	if !ok || !b.Published {
		return nil, fmt.Errorf("not found")
	}
	s.store.IncrStat(b.ID, "profile_viewed")
	prods := s.store.GetProducts(b.ID)
	active := []Product{}
	for _, p := range prods {
		if p.Active {
			active = append(active, p)
		}
	}
	revs := []Review{}
	for _, r := range s.store.GetReviews(b.ID) {
		if r.Status == "visible" {
			revs = append(revs, r)
		}
	}
	return map[string]interface{}{
		"business": b,
		"products": active,
		"reviews":  revs,
	}, nil
}

func (s *Service) Track(slug, event string) {
	b, ok := s.store.GetBySlug(slug)
	if !ok {
		return
	}
	s.store.IncrStat(b.ID, event)
}
