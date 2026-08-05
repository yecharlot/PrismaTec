package poh

import "testing"

func TestStore_SessionAndEvents(t *testing.T) {
	s := &Store{}
	s.Lock()
	defer s.Unlock()

	if s.SessionID() != "" {
		t.Fatalf("empty session expected")
	}
	s.SetSessionID("sess-1")
	if s.SessionID() != "sess-1" {
		t.Fatalf("SessionID = %q", s.SessionID())
	}

	s.Append(Event{EventType: "click", Timestamp: 1})
	s.Append(Event{EventType: "scroll", Timestamp: 2})
	if s.Len() != 2 {
		t.Fatalf("Len = %d, want 2", s.Len())
	}
	ev := s.Events()
	if ev[0].EventType != "click" || ev[1].EventType != "scroll" {
		t.Fatalf("events = %#v", ev)
	}

	s.ClearEvents()
	if s.Len() != 0 {
		t.Fatalf("ClearEvents failed, Len = %d", s.Len())
	}
}

func TestStore_Reset(t *testing.T) {
	s := &Store{}
	s.SetSessionID("old")
	s.Append(Event{EventType: "x"})
	s.Reset("new")
	if s.SessionID() != "new" || s.Len() != 0 {
		t.Fatalf("Reset failed: id=%q len=%d", s.SessionID(), s.Len())
	}
}

func TestStore_SetEvents(t *testing.T) {
	s := &Store{}
	s.SetEvents([]Event{{EventType: "a"}, {EventType: "b"}})
	if s.Len() != 2 {
		t.Fatalf("Len = %d", s.Len())
	}
}
