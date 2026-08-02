package poh

import "sync"

type Event struct {
	Timestamp   int64  `json:"timestamp"`
	EventType   string `json:"event_type"`
	Metadata    string `json:"metadata"`
	Signature   string `json:"signature,omitempty"`
	HumanitySig string `json:"humanity_sig"`
}

type Proof struct {
	SessionID string  `json:"session_id"`
	Events    []Event `json:"events"`
	FinalSig  string  `json:"final_signature"`
}

type Store struct {
	mu        sync.Mutex
	sessionID string
	events    []Event
}

var Global = &Store{}

func (s *Store) Lock()   { s.mu.Lock() }
func (s *Store) Unlock() { s.mu.Unlock() }

func (s *Store) SessionID() string { return s.sessionID }

func (s *Store) SetSessionID(id string) { s.sessionID = id }

func (s *Store) Events() []Event {
	return s.events // callers must hold Lock for mutation safety
}

func (s *Store) SetEvents(ev []Event) { s.events = ev }

func (s *Store) Append(e Event) {
	s.events = append(s.events, e)
}

func (s *Store) ClearEvents() { s.events = nil }

func (s *Store) Reset(sessionID string) {
	s.sessionID = sessionID
	s.events = nil
}

func (s *Store) Len() int { return len(s.events) }
