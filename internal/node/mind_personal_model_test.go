package node

import (
	"strings"
	"testing"
)

func TestUserProfileSynthesis(t *testing.T) {
	eps := []mindEpisodePayload{
		{Text: "me llamo Esteban"},
		{Text: "mis apellidos son Charlot Poll"},
		{Text: "yo soy un hombre"},
		{Text: "yo no soy Socrates"},
	}
	p := buildUserProfile(eps)
	if !strings.EqualFold(p.Nombre, "Esteban") {
		t.Fatalf("nombre=%q", p.Nombre)
	}
	if !strings.Contains(strings.ToLower(p.Apellidos), "charlot") {
		t.Fatalf("apellidos=%q", p.Apellidos)
	}
	if len(p.Soy) == 0 || len(p.NoSoy) == 0 {
		t.Fatalf("soy/nosoy empty: %#v", p)
	}
	v := speakFromProfile("qué soy yo", p)
	low := strings.ToLower(v)
	for _, need := range []string{"esteban", "charlot", "hombre"} {
		if !strings.Contains(low, need) {
			t.Fatalf("missing %s in %s", need, v)
		}
	}
	if strings.Contains(low, "aún no tengo un perfil") {
		t.Fatalf("should have profile: %s", v)
	}
}

func TestSelfModelQuery(t *testing.T) {
	if !isSelfModelQuery("qué soy yo") {
		t.Fatal("qué soy yo")
	}
	if isSelfModelQuery("quién eres") {
		t.Fatal("quién eres is Mind, not user")
	}
}
