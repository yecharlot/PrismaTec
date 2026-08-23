package gennode

import (
	"strings"
	"testing"
)

func TestDialogueFindings(t *testing.T) {
	d := &Daemon{Pkg: &FrontierPackage{Key: "demo-cell.ans", CurrentRootCID: "bafk"}}
	d.findings = []map[string]interface{}{
		{"url": "https://example.com", "title": "Example Domain", "snippet": "docs", "status": 200},
	}
	res := d.Dialogue("qué sabes")
	v, _ := res["voice"].(string)
	if !strings.Contains(strings.ToLower(v), "example") {
		t.Fatalf("voice=%q", v)
	}
	id := d.Dialogue("quién eres")
	v2, _ := id["voice"].(string)
	if !strings.Contains(v2, "demo-cell") {
		t.Fatalf("identity=%q", v2)
	}
}
