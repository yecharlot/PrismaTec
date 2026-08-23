package gennode

import "testing"

func TestCargoAcceptTTL(t *testing.T) {
	d := &Daemon{
		Pkg:     &FrontierPackage{Key: "demo-cell.ans", CurrentRootCID: "bafk"},
		DataDir: t.TempDir(),
	}
	res := d.AcceptCargo(CargoEnvelope{Key: "demo-cell.ans", RootCID: "bafk", TTL: 1, Payload: map[string]interface{}{"x": 1}})
	if res["ok"] != true {
		t.Fatalf("%v", res)
	}
	if res["forwarded"].(int) != 0 {
		t.Fatalf("no peers → forwarded 0, got %v", res["forwarded"])
	}
}
