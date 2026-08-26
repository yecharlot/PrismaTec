package node

import (
	"strings"
	"testing"
)

func TestCleanExploreSnippetDropsJS(t *testing.T) {
	html := `<html><head><title>Mar - Wikipedia</title><script>function(){var className="client-js"}</script></head><body><p>El mar es una masa de agua salada.</p></body></html>`
	sn := cleanExploreSnippet("Mar - Wikipedia", html)
	if strings.Contains(sn, "function()") || strings.Contains(sn, "client-js") {
		t.Fatalf("js leak: %q", sn)
	}
	if !strings.Contains(strings.ToLower(sn), "mar") {
		t.Fatalf("expected content: %q", sn)
	}
}
