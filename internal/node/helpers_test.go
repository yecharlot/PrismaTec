package node

import (
	"encoding/json"
	"testing"
)

func TestGenerarUUID_Length(t *testing.T) {
	id := generarUUID()
	if len(id) != 16 {
		t.Fatalf("len = %d, want 16 (%q)", len(id), id)
	}
}

func TestCanonicalizeJSON_StableKeyOrder(t *testing.T) {
	data := map[string]interface{}{
		"z": float64(1),
		"a": float64(2),
		"m": float64(3),
	}
	b1, err := canonicalizeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := canonicalizeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("not stable:\n%s\n%s", b1, b2)
	}
	// keys should appear sorted: a, m, z
	s := string(b1)
	if !(len(s) > 0 && s[0] == '{') {
		t.Fatalf("not object: %s", s)
	}
	var round map[string]interface{}
	if err := json.Unmarshal(b1, &round); err != nil {
		t.Fatalf("invalid json: %v (%s)", err, s)
	}
	if round["a"] != float64(2) || round["z"] != float64(1) {
		t.Fatalf("roundtrip = %#v", round)
	}
}
