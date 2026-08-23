package gennode

import "testing"

func TestLoadPackageFile(t *testing.T) {
	p, err := LoadPackageFile("testdata_demo.package.json")
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "demo-cell.ans" {
		t.Fatalf("key=%s", p.Key)
	}
	if p.ServicePage() == "" {
		t.Fatal("empty page")
	}
}
