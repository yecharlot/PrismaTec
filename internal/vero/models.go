package vero

import "time"

// Business is a portable commercial identity ("tu Vero").
type Business struct {
	ID                 string    `json:"id"`
	ZyrID              string    `json:"vero_id"` // public ID e.g. VERO-8F4A92
	Slug               string    `json:"slug"`
	Name               string    `json:"name"`
	OwnerUserID        string    `json:"owner_user_id"`
	Phone              string    `json:"phone,omitempty"`
	WhatsApp           string    `json:"whatsapp,omitempty"`
	Category           string    `json:"category,omitempty"`
	City               string    `json:"city,omitempty"`
	Zone               string    `json:"zone,omitempty"`
	Bio                string    `json:"bio,omitempty"`
	LogoURL            string    `json:"logo_url,omitempty"`
	Hours              string    `json:"hours,omitempty"`
	VerificationLevel  int       `json:"verification_level"` // 0-3
	Plan               string    `json:"plan"`                // free|pro|business
	Published          bool      `json:"published"`
	Score              int       `json:"score"` // 0-100 Vero Score
	ReviewCount        int       `json:"review_count"`
	RatingAvg          float64   `json:"rating_avg"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Product struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"business_id"`
	Title      string    `json:"title"`
	Price      float64   `json:"price"`
	Currency   string    `json:"currency"`
	PhotoURL   string    `json:"photo_url,omitempty"`
	Active     bool      `json:"active"`
	Sort       int       `json:"sort"`
	CreatedAt  time.Time `json:"created_at"`
}

type Review struct {
	ID                 string    `json:"id"`
	BusinessID         string    `json:"business_id"`
	Rating             int       `json:"rating"` // 1-5
	Comment            string    `json:"comment,omitempty"`
	VerifiedOperation  bool      `json:"verified_operation"`
	Status             string    `json:"status"` // visible|hidden
	CreatedAt          time.Time `json:"created_at"`
}

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
	Active       bool      `json:"active"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Analytics counters per business (daily aggregates optional later).
type Stats struct {
	ProfileViews    int `json:"profile_views"`
	CatalogViews    int `json:"catalog_views"`
	ProductViews    int `json:"product_views"`
	WhatsAppClicks  int `json:"whatsapp_clicks"`
	QRScans         int `json:"qr_scans"`
	Shares          int `json:"shares"`
}
