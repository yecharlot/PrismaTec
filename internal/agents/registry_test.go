package agents

import "testing"

func TestNewRegistry_EmptyMaps(t *testing.T) {
	r := NewRegistry()
	if r.Modulos == nil || r.Entidades == nil || r.Relaciones == nil || r.Tokens == nil || r.Roles == nil {
		t.Fatal("maps must be initialized")
	}
	if len(r.Modulos) != 0 {
		t.Fatalf("want empty Modulos")
	}
}

func TestRegistry_ModuloCRUD(t *testing.T) {
	r := NewRegistry()
	m := &Modulo{ID: "m1", Nombre: "ventas", Rol: "admin"}
	r.MuModulos.Lock()
	r.Modulos[m.ID] = m
	r.MuModulos.Unlock()

	r.MuModulos.RLock()
	got, ok := r.Modulos["m1"]
	r.MuModulos.RUnlock()
	if !ok || got.Nombre != "ventas" {
		t.Fatalf("got %#v", got)
	}
}

func TestRegistry_TokenAndRoles(t *testing.T) {
	r := NewRegistry()
	r.MuTokens.Lock()
	r.Tokens["tok1"] = &TokenAlset{Token: "tok1", AgentID: "a1", Roles: []string{"owner"}}
	r.MuTokens.Unlock()

	r.Roles["a1"] = []string{"owner", "editor"}
	if len(r.Roles["a1"]) != 2 {
		t.Fatalf("roles = %v", r.Roles["a1"])
	}
	if r.Tokens["tok1"].AgentID != "a1" {
		t.Fatalf("token agent = %s", r.Tokens["tok1"].AgentID)
	}
}
