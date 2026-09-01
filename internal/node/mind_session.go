package node

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"time"
)

// mindSessionState — hilo de diálogo aislado por cliente (no mezcla Lucía/Diego).
type mindSessionState struct {
	ID             string
	LastMath       float64
	LastMathOK     bool
	LastCreative   string
	TopicStack     []string
	LastQuery      string
	LastVoice      string
	LastScoutTopic string
	LastKnow       string
	LastMem        string
	Phase          string // opening | ongoing | closing
	LastAct        string // social | content | task | meta
	TurnCount      int
	UpdatedAt      time.Time
}

func normalizeSessionID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 80 {
		s = s[:80]
	}
	// sanitize
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			out = append(out, r)
		}
	}
	return string(out)
}

// deriveSessionFallback: si el cliente no manda session, no inventamos una estable
// entre usuarios (mejor vacío = solo hilo de proceso actual sin contaminar disco).
func deriveSessionFallback(text string) string {
	return ""
}

func (n *NodoAlset) getOrCreateSession(id string) *mindSessionState {
	id = normalizeSessionID(id)
	if n == nil {
		return &mindSessionState{ID: id}
	}
	n.mindSessionMu.Lock()
	defer n.mindSessionMu.Unlock()
	if n.mindSessions == nil {
		n.mindSessions = make(map[string]*mindSessionState)
	}
	if id == "" {
		// sesión anónima de proceso: un solo bucket "anon" por nodo
		id = "anon"
	}
	st, ok := n.mindSessions[id]
	if !ok || st == nil {
		st = &mindSessionState{ID: id, TopicStack: nil}
		n.mindSessions[id] = st
	}
	st.UpdatedAt = time.Now().UTC()
	return st
}

func (n *NodoAlset) bindSessionThread(sess *mindSessionState) {
	if n == nil || sess == nil {
		return
	}
	n.mindLastMu.Lock()
	defer n.mindLastMu.Unlock()
	n.mindLastMathResult = sess.LastMath
	n.mindLastMathOK = sess.LastMathOK
	n.mindLastCreative = sess.LastCreative
	n.mindTopicStack = append([]string{}, sess.TopicStack...)
	n.mindLastQuery = sess.LastQuery
	n.mindLastVoice = sess.LastVoice
	n.mindLastScoutTopic = sess.LastScoutTopic
	n.mindLastKnow = sess.LastKnow
	n.mindLastMem = sess.LastMem
}

func (n *NodoAlset) saveSessionThread(sess *mindSessionState) {
	if n == nil || sess == nil {
		return
	}
	n.mindLastMu.Lock()
	sess.LastMath = n.mindLastMathResult
	sess.LastMathOK = n.mindLastMathOK
	sess.LastCreative = n.mindLastCreative
	sess.TopicStack = append([]string{}, n.mindTopicStack...)
	sess.LastQuery = n.mindLastQuery
	sess.LastVoice = n.mindLastVoice
	sess.LastScoutTopic = n.mindLastScoutTopic
	sess.LastKnow = n.mindLastKnow
	sess.LastMem = n.mindLastMem
	sess.UpdatedAt = time.Now().UTC()
	n.mindLastMu.Unlock()

	n.mindSessionMu.Lock()
	if n.mindSessions == nil {
		n.mindSessions = make(map[string]*mindSessionState)
	}
	n.mindSessions[sess.ID] = sess
	n.mindSessionMu.Unlock()
}

func filterEpisodesBySession(episodes []mindEpisodePayload, session string) []mindEpisodePayload {
	session = normalizeSessionID(session)
	out := make([]mindEpisodePayload, 0, len(episodes))
	if session == "" || session == "anon" {
		// Anónimos: no heredar hechos de sesiones nominadas (Lucía/Diego ajenos)
		for _, ep := range episodes {
			sid := normalizeSessionID(ep.Session)
			if sid == "" || sid == "anon" {
				out = append(out, ep)
			}
		}
		return out
	}
	for _, ep := range episodes {
		if normalizeSessionID(ep.Session) == session {
			out = append(out, ep)
		}
	}
	return out
}

func shortSessionHash(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:6])
}
