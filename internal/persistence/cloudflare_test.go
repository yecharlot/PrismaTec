package persistence

import "testing"

func TestNewCloudflareStoreRequiresURL(t *testing.T) {
	_, err := NewCloudflareStore("", "")
	if err == nil {
		t.Fatal("expected error")
	}
	s, err := NewCloudflareStore("https://example.workers.dev", "sec")
	if err != nil || s.base != "https://example.workers.dev" {
		t.Fatalf("%v %#v", err, s)
	}
}
