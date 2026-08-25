package node

import (
	"strings"
	"testing"
)

func TestClassifyMindRouteVolume(t *testing.T) {
	cases := []struct {
		in   string
		want RouteSource
	}{
		{"quién es michael jordan", RouteWeb},
		{"quien es michel jordan", RouteWeb},
		{"quién es benjamin franklin", RouteWeb},
		{"qué es el antagonismo", RouteWeb},
		{"que es la fotosíntesis", RouteWeb},
		{"cuánto es 12 + 7", RouteMath},
		{"suma 3 y 5", RouteMath},
		{"qué hice", RouteAction},
		{"que hiciste", RouteAction},
		{"porque lo hiciste", RouteAction},
		{"por qué lo hiciste", RouteAction},
		{"genera código función sumar en go", RouteCodegen},
		{"cómo me llamo", RouteMemory},
		{"me llamo Esteban", RouteMemory},
		{"como se llama su madre", RouteThread},
		{"qué significa esto", RouteThread},
		{"hola", RouteChat},
		{"crea gen demo-cell", RouteGenTool},
	}
	for _, c := range cases {
		got := classifyMindRoute(c.in)
		if got.Source != c.want {
			t.Errorf("classify(%q)=%s rule=%s want %s", c.in, got.Source, got.Rule, c.want)
		}
	}
}

func TestDetectVoiceAnomaliesPatterns(t *testing.T) {
	bad := []struct {
		voice string
		code  string
	}{
		{"antagonismo at DuckDuckGo @media (prefers-color-scheme: dark)", "html_css_junk"},
		{"Me suena esto: «hallazgo sonda scout-x sobre y». ¿Seguimos?", "soft_memory_hijack"},
		{"Varias entradas en Wikipedia; puede referirse a…", "disambiguation"},
		{"Sobre «x» tengo este resumen, pero no incluye el nombre que pides:\n\n\n(Recuperado)", "empty_summary_body"},
		{"Despaché la sonda «scout-x» a la web.", "lab_jargon"},
		{"Félix Varela was a Cuban Catholic priest and independence leader.", "english_only"},
		{"", "empty_voice"},
	}
	for _, c := range bad {
		hits := DetectVoiceAnomalies(c.voice)
		found := false
		for _, h := range hits {
			if h.Code == c.code {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected code %s in anomalies for %q; got %#v", c.code, c.voice, hits)
		}
	}
	good := DetectVoiceAnomalies("Michael Jordan. Exjugador de baloncesto estadounidense.\n\nFuente: https://es.wikipedia.org/wiki/Michael_Jordan")
	for _, h := range good {
		if h.Code == "html_css_junk" || h.Code == "lab_jargon" {
			t.Errorf("false positive on clean ES voice: %#v", h)
		}
	}
}

func TestExtractTopicVolume(t *testing.T) {
	pairs := map[string]string{
		"quién es michel jordan":    "michael jordan",
		"quien es juana de arco":    "juana de arco",
		"qué es el antagonismo":     "antagonismo",
		"busca información sobre bioinformatica": "bioinformática",
	}
	for in, want := range pairs {
		got := extractTopic(normalizeUserInput(in))
		if got != want {
			t.Errorf("topic(%q)=%q want %q", in, got, want)
		}
	}
}

func TestAnomalyCorpusVsWebPriority(t *testing.T) {
	// Routing must prefer web for open factual, not chat
	for _, q := range []string{
		"quién fue newton", "quién es marie curie", "qué es la mitocondria",
		"quién es tiziano ferro", "qué es el feudalismo",
	} {
		d := classifyMindRoute(q)
		if d.Source != RouteWeb {
			t.Errorf("%q routed to %s (%s), want web", q, d.Source, d.Rule)
		}
		if d.Topic == "" {
			t.Errorf("%q missing topic", q)
		}
	}
}

func TestAnomalyNoSoftHijackPattern(t *testing.T) {
	// Pattern learning: any voice with Me suena + scout is forbidden class
	samples := []string{
		"Me suena esto: «hallazgo sonda scout-juana». ¿Seguimos por ahí?",
		"Me suena esto: «hallazgo sonda scout-antagonismo sobre antagonismo: antagonismo at DuckD».",
	}
	for _, s := range samples {
		if !strings.Contains(strings.ToLower(s), "me suena") {
			continue
		}
		hits := DetectVoiceAnomalies(s)
		ok := false
		for _, h := range hits {
			if h.Code == "soft_memory_hijack" || h.Code == "raw_scout_echo" {
				ok = true
			}
		}
		if !ok {
			t.Errorf("pattern not caught: %q", s)
		}
	}
}
