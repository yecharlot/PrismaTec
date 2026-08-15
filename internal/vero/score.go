package vero

// ComputeScore returns 0-100 "Vero Score" — reputation/activity indicator, not a safety guarantee.
func ComputeScore(b *Business, reviews []Review, st Stats) int {
	if b == nil {
		return 0
	}
	score := 25 // base for existing profile
	if b.Name != "" {
		score += 5
	}
	if b.WhatsApp != "" || b.Phone != "" {
		score += 10
	}
	if b.Category != "" {
		score += 5
	}
	if b.City != "" {
		score += 5
	}
	if b.Bio != "" {
		score += 5
	}
	if b.VerificationLevel >= 1 {
		score += 10
	}
	if b.VerificationLevel >= 2 {
		score += 10
	}
	// reviews
	visible := 0
	sum := 0
	for _, r := range reviews {
		if r.Status != "visible" {
			continue
		}
		visible++
		sum += r.Rating
	}
	if visible > 0 {
		avg := float64(sum) / float64(visible)
		score += int(avg * 4) // up to ~20
		if visible >= 5 {
			score += 5
		}
		if visible >= 20 {
			score += 5
		}
	}
	// activity
	if st.ProfileViews > 10 {
		score += 3
	}
	if st.WhatsAppClicks > 5 {
		score += 5
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}
