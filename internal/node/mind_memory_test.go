package node

import (
	"strings"
	"testing"
)

func TestRecallFullNameFromUtterance(t *testing.T) {
	q := "si mi nombre es esteban y mis apellidos son charlot poll, entonces cual es mi nombre completo"
	ans := recallFullName(q, nil, q)
	if ans == "" || !strings.Contains(strings.ToLower(ans), "charlot") || !strings.Contains(strings.ToLower(ans), "esteban") {
		t.Fatalf("ans=%q", ans)
	}
}

func TestSpeakFromMemorySkipsScoutForName(t *testing.T) {
	eps := []mindEpisodePayload{
		{Text: "hallazgo sonda scout-newton sobre isaac newton: Isaac Newton fue físico"},
		{Text: "mi nombre es Esteban"},
	}
	got := speakFromMemory("como me llamo", eps)
	if strings.Contains(strings.ToLower(got), "newton") || strings.Contains(strings.ToLower(got), "hallazgo") {
		t.Fatalf("scout leak: %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "esteban") {
		t.Fatalf("want name: %q", got)
	}
}
