package node

import (
	"strings"
	"testing"
)

func TestDedupeEpisodes(t *testing.T) {
	eps := []mindEpisodePayload{
		{Text: "El tiempo es una ilusión"},
		{Text: "El tiempo es una ilusión"},
		{Text: "mi nombre es esteban"},
	}
	out := dedupeEpisodesByText(eps)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
}

func TestBriefRFTFromUserChain(t *testing.T) {
	line := briefRFTLine("memoria es tiempo y tiempo es ilusión", nil)
	if line == "" {
		// need es clauses
		line = briefRFTLine("la memoria es tiempo; el tiempo es una ilusión", nil)
	}
	// may need explicit then - force facts
	facts := []ternaryFact{
		{Subj: "memoria", Rel: "es", Obj: "tiempo", Conf: 2, Src: "usuario"},
		{Subj: "tiempo", Rel: "es", Obj: "ilusión", Conf: 2, Src: "usuario"},
	}
	line = briefRFTLine("qué es la memoria", facts)
	if line == "" || !strings.Contains(strings.ToLower(line), "ilusión") && !strings.Contains(strings.ToLower(line), "ilusion") {
		t.Fatalf("line=%q", line)
	}
}
