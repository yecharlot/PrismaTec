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

func TestKnowledgeNoCondFalsePositive(t *testing.T) {
	got := speakFromKnowledge("qué es la mitocondria")
	if strings.Contains(strings.ToLower(got), "lisp") || strings.Contains(got, "(if ") {
		t.Fatalf("corpus false positive lisp for mitocondria: %q", got)
	}
}

func TestClassifyCreativeAndPatterns(t *testing.T) {
	if classifyMindRoute("escribe un poema sobre el mar").Source != RouteCreative {
		t.Fatal("expected creative")
	}
	if classifyMindRoute("qué patrones aprendiste").Source != RoutePatterns {
		t.Fatal("expected patterns")
	}
}

func TestCreativeComposeNotEmpty(t *testing.T) {
	v := mindComposeCreative("escribe un poema sobre el río", 0, "", "")
	if v == "" || !strings.Contains(v, "río") && !strings.Contains(v, "rio") {
		// theme may keep accent
		if !strings.Contains(strings.ToLower(v), "río") && !strings.Contains(strings.ToLower(v), "rio") && !strings.Contains(v, "Composición ternaria") {
			t.Fatalf("bad creative: %q", v)
		}
	}
	if strings.Contains(strings.ToLower(v), "function(){") {
		t.Fatal("junk in creative")
	}
}

func TestRecordAnomaliesLearns(t *testing.T) {
	patternsMu.Lock()
	patternsRing = nil
	patternsMu.Unlock()
	recordVoiceAnomalies("test q", "antagonismo @media (prefers-color-scheme: dark) function(){")
	if !policyFlag("prefer_wiki_es_only", 1) {
		t.Fatal("expected learned prefer_wiki_es_only")
	}
}

func TestVolumeRouteMatrix(t *testing.T) {
	// large matrix of intents → expected family
	web := []string{"quién es cervantes", "quién es borges", "qué es la democracia", "qué es un átomo",
		"quién es galileo", "quién es newton", "qué es el feudalismo", "quién es ada lovelace"}
	for _, q := range web {
		if classifyMindRoute(q).Source != RouteWeb {
			t.Errorf("%q not web", q)
		}
	}
	math := []string{"cuánto es 2+2", "suma 10 y 11", "cuanto es 100-3"}
	for _, q := range math {
		if classifyMindRoute(q).Source != RouteMath {
			t.Errorf("%q not math", q)
		}
	}
	act := []string{"qué hice", "que hiciste", "porque lo hiciste", "por qué lo hiciste"}
	for _, q := range act {
		if classifyMindRoute(q).Source != RouteAction {
			t.Errorf("%q not action", q)
		}
	}
}
