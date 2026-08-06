package pulse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFormatSSE(t *testing.T) {
	got := FormatSSE("ping", []byte(`{"ok":true}`))
	want := "event: ping\ndata: {\"ok\":true}\n\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestListenSSE_ReceivesEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: hello\ndata: {\"n\":1}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var gotType, gotData string
	err := ListenSSE(ctx, srv.URL, func(et, data string) {
		gotType, gotData = et, data
		cancel()
	})
	// connection ends when server closes or ctx cancelled
	_ = err
	if gotType != "hello" || gotData != `{"n":1}` {
		t.Fatalf("event=%q data=%q", gotType, gotData)
	}
}
