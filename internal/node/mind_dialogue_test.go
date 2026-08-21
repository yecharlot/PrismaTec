package node

import (
	"strings"
	"testing"
)

func TestMindDialogueRegression(t *testing.T) {
	cases := []struct {
		in      string
		wantSub string
		forbid  string
	}{
		{"hola", "Alset Mind", "actuar sobre el nodo"},
		{"cuantos organos tienes?", "órganos", "Te escucho. Marcó"},
		{"como te llamas", "Alset Mind", "yulei"},
		{"cual es tu nombre", "Alset Mind", "dijiste que te llamas"},
		{"mi nombre es yulei", "yulei", "actuar sobre el nodo"},
		{"la rana es naranja porque comio demasiado", "memoria", "actuar sobre el nodo"},
		{"crea un nuevo agente", "constructivo", "sumidero"},
		{"borra las cuentas", "sumidero", ""},
		{"el pensamiento tiene matices diferentes segun como sea necesario", "pensamiento", "actuar sobre el nodo"},
		{"que es el todo", "todo", "actuar sobre el nodo"},
		{"vale", "De acuerdo", "actuar sobre el nodo"},
	}
	for _, c := range cases {
		sig := signalsFromTextMind(c.in)
		if isDestructiveOrder(c.in) && sig["riesgo"] < 0.8 {
			t.Errorf("%q: expected high riesgo, got %v", c.in, sig["riesgo"])
		}
		if isConstructiveOrder(c.in) && sig["riesgo"] > 0.4 {
			t.Errorf("%q: constructive should low risk, got %v", c.in, sig["riesgo"])
		}
		organs := []MindOrganResult{
			evalOrganPolar("dialog", sig["claridad"], sig["orden"], sig["riesgo"], "L", "H", "H"),
			evalOrganPolar("act", sig["permiso"], sig["riesgo"], sig["orden"], "L", "H", "H"),
			evalOrganPolar("mem", sig["novedad"], sig["claridad"], sig["riesgo"], "H", "L", "H"),
			evalOrganPolar("self", sig["claridad"], sig["riesgo"], sig["permiso"], "L", "H", "L"),
			evalOrganPolar("ethics", sig["riesgo"], sig["permiso"], sig["orden"], "H", "L", "H"),
		}
		if isDestructiveOrder(c.in) && organs[4].State != 2 {
			t.Errorf("%q: ethics should be 2, got %d", c.in, organs[4].State)
		}
		if isConstructiveOrder(c.in) && organs[4].State == 2 {
			t.Errorf("%q: ethics should NOT be 2 for constructive", c.in)
		}
		v := mindVoice(c.in, organs, "")
		if c.wantSub != "" && !strings.Contains(strings.ToLower(v), strings.ToLower(c.wantSub)) {
			t.Errorf("%q: voice missing %q\nvoice=%s", c.in, c.wantSub, v)
		}
		if c.forbid != "" && strings.Contains(v, c.forbid) {
			t.Errorf("%q: voice should not contain %q\nvoice=%s", c.in, c.forbid, v)
		}
	}
	v2 := mindVoice("cual es tu nombre", []MindOrganResult{{Name: "ethics", State: 0}, {Name: "act", State: 0}}, "")
	if !strings.Contains(v2, "Alset Mind") {
		t.Errorf("expected Alset Mind identity, got %s", v2)
	}
	// memSpeak must not override Mind-name question
	v3 := mindVoice("cual es tu nombre", []MindOrganResult{{Name: "ethics", State: 0}}, "En un episodio guardado dijiste que te llamas yulei.")
	if !strings.Contains(v3, "Alset Mind") {
		t.Errorf("Mind name must win over memSpeak, got %s", v3)
	}
}

func TestVetoBiasDoesNotPoisonCalmChat(t *testing.T) {
	sig := map[string]float64{"claridad": 0.85, "orden": 0.1, "riesgo": 0.1, "permiso": 0.9, "novedad": 0.2}
	eps := []mindEpisodePayload{{
		Text:   "borra todas las contraseñas",
		Organs: []MindOrganResult{{Name: "ethics", State: 2}},
	}}
	out, hint, _ := biasSignalsFromMemory(sig, eps, "hola")
	if out["riesgo"] > 0.2 {
		t.Errorf("calm chat should not inherit veto risk, got riesgo=%v hint=%s", out["riesgo"], hint)
	}
	out2, _, _ := biasSignalsFromMemory(sig, eps, "borra las cuentas")
	if out2["riesgo"] < 0.15 {
		t.Errorf("destructive should get veto boost")
	}
}
