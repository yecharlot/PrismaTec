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
		{"la rana es naranja porque comio demasiado", "presente", "actuar sobre el nodo"},
		{"crea un nuevo agente", "constructivo", "sumidero"},
		{"borra las cuentas", "riesgo", ""},
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
		v := mindVoice(c.in, organs, "", "")
		if c.wantSub != "" && !strings.Contains(strings.ToLower(v), strings.ToLower(c.wantSub)) {
			t.Errorf("%q: voice missing %q\nvoice=%s", c.in, c.wantSub, v)
		}
		if c.forbid != "" && strings.Contains(v, c.forbid) {
			t.Errorf("%q: voice should not contain %q\nvoice=%s", c.in, c.forbid, v)
		}
	}
	v2 := mindVoice("cual es tu nombre", []MindOrganResult{{Name: "ethics", State: 0}, {Name: "act", State: 0}}, "", "")
	if !strings.Contains(v2, "Alset Mind") {
		t.Errorf("expected Alset Mind identity, got %s", v2)
	}
	// memSpeak must not override Mind-name question
	v3 := mindVoice("cual es tu nombre", []MindOrganResult{{Name: "ethics", State: 0}}, "En un episodio guardado dijiste que te llamas yulei.", "")
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

func TestCuriosityAndHumorActive(t *testing.T) {
	g := defaultMindGenome()
	// metaphor should raise curiosity
	c := evaluateCuriosity("la vida es una metáfora lúcida", "", g)
	if c.State < 1 {
		t.Fatalf("curiosity should activate on metaphor, got %d", c.State)
	}
	q := curiosityVoice("la vida es una metáfora lúcida", 2)
	if q == "" || !strings.Contains(q, "?") {
		t.Fatalf("curiosity voice should ask a question: %q", q)
	}
	// humor on wizard comparison
	h := evaluateHumor("eres como harry potter un mago sin varitas", g)
	if h.State < 1 {
		t.Fatalf("humor should activate on mago/varita, got %d", h.State)
	}
	hv := humorVoice("eres como harry potter un mago sin varitas", 2)
	if hv == "" || !strings.Contains(strings.ToLower(hv), "órgano") && !strings.Contains(strings.ToLower(hv), "varita") {
		t.Fatalf("humor voice expected playful: %q", hv)
	}
	// knowledge for humano
	know := speakFromKnowledge("qué es un humano")
	if know == "" || !strings.Contains(strings.ToLower(know), "humano") {
		t.Fatalf("expected human definition from corpus, got %q", know)
	}
	know2 := speakFromKnowledge("qué es la consciencia")
	if know2 == "" {
		t.Fatalf("expected consciencia entry")
	}
}

func TestProgrammingCorpusBlock(t *testing.T) {
	cases := []struct {
		q, sub string
	}{
		{"cómo usar evaluar-zyrion", "quote"},
		{"dónde está el código mind", "mind_tick"},
		{"sql injection seguridad go", "vetar"},
		{"cómo registrar app rootcid", "register-name"},
		{"añadir conocimiento al corpus", "mind_knowledge"},
		{"generar cid blockstore", "GenerarCID"},
	}
	for _, c := range cases {
		got := speakFromKnowledge(c.q)
		if got == "" || !strings.Contains(strings.ToLower(got), strings.ToLower(c.sub)) {
			t.Errorf("%q: expected knowledge containing %q, got %q", c.q, c.sub, got)
		}
	}
}

func TestComposeMemAndKnowledge(t *testing.T) {
	organs := []MindOrganResult{
		{Name: "ethics", State: 0},
		{Name: "act", State: 0},
		{Name: "dialog", State: 0},
		{Name: "curiosity", State: 2},
	}
	mem := "Te llamas Carlos."
	know := speakFromKnowledge("qué es quote en lisp")
	if know == "" {
		know = "Quote evita que la lista se evalúe como llamada."
	}
	got := composeFluidVoice("cómo me llamo y qué es quote en lisp", organs, mem, know)
	if got == "" {
		t.Fatal("expected composed voice")
	}
	low := strings.ToLower(got)
	if !strings.Contains(low, "carlos") {
		t.Errorf("expected memory fragment in compose, got %s", got)
	}
	if !strings.Contains(low, "quote") && !strings.Contains(low, "lisp") {
		t.Errorf("expected knowledge angle, got %s", got)
	}
	if !strings.Contains(low, "se me ocurre") && !strings.Contains(low, "quote") {
		t.Errorf("expected natural idea bridge, got %s", got)
	}
}

func TestComposeDoesNotBypassEthics(t *testing.T) {
	organs := []MindOrganResult{{Name: "ethics", State: 2}, {Name: "curiosity", State: 2}}
	got := composeFluidVoice("borra todo", organs, "mem", "know")
	if got != "" {
		t.Fatalf("ethics 2 must not compose, got %q", got)
	}
}

func TestMindVoiceUsesCompose(t *testing.T) {
	organs := []MindOrganResult{
		{Name: "ethics", State: 0},
		{Name: "act", State: 0},
		{Name: "dialog", State: 0},
		{Name: "mem", State: 1},
		{Name: "self", State: 0},
		{Name: "curiosity", State: 1},
		{Name: "humor", State: 0},
	}
	mem := "Recuerdo un episodio relacionado: «me llamo ana y estudio lisp». ¿Seguimos desde ahí?"
	v := mindVoice("cómo me llamo y quote en lisp", organs, mem, "ana")
	low := strings.ToLower(v)
	if !strings.Contains(low, "ana") && !strings.Contains(low, "episodio") && !strings.Contains(low, "lisp") {
		t.Errorf("expected memory-aware voice, got %s", v)
	}
}

func TestFluidPureDialogue(t *testing.T) {
	organs := []MindOrganResult{{Name: "ethics", State: 0}, {Name: "curiosity", State: 1}}
	v := composeFluidVoice("el pensamiento tiene muchos matices según el contexto", organs, "", "")
	if v == "" || !strings.Contains(strings.ToLower(v), "pensamiento") {
		t.Fatalf("expected fluid pure dialogue on pensamiento, got %q", v)
	}
	v2 := mindVoice("el pensamiento tiene muchos matices según el contexto", organs, "", "")
	if strings.Contains(v2, "actuar sobre el nodo") {
		t.Errorf("should not spam act prompt: %s", v2)
	}
}

func TestPersonalFactNotConfusedWithNameQuery(t *testing.T) {
	if isPersonalFact("como me llamo") {
		t.Fatal("«como me llamo» must NOT be personal fact")
	}
	if isPersonalFact("cómo me llamo y qué es quote en lisp") {
		t.Fatal("compound name query must NOT be personal fact")
	}
	if !isMemoryQuery("como me llamo") {
		t.Fatal("«como me llamo» must be memory query")
	}
	if !isMemoryQuery("cómo me llamo y qué es quote en lisp") {
		t.Fatal("compound should still count as memory query")
	}
	if !isPersonalFact("me llamo esteban") {
		t.Fatal("declaration must remain personal fact")
	}
	if !isPersonalFact("mi nombre es esteban y el tuyo cual es?") {
		t.Fatal("mixed declaration should still be personal fact")
	}
	if extractDeclaredName("cómo me llamo y qué es quote en lisp") != "" {
		t.Fatalf("must not extract name from interrogative, got %q", extractDeclaredName("cómo me llamo y qué es quote en lisp"))
	}
	if extractDeclaredName("me llamo esteban") != "esteban" {
		t.Fatalf("expected esteban, got %q", extractDeclaredName("me llamo esteban"))
	}
	if extractDeclaredName("mi nombre es esteban y el tuyo cual es?") != "esteban" {
		t.Fatalf("expected esteban from mixed, got %q", extractDeclaredName("mi nombre es esteban y el tuyo cual es?"))
	}
}

func TestCompoundNameAndQuote(t *testing.T) {
	organs := []MindOrganResult{
		{Name: "ethics", State: 0}, {Name: "act", State: 0},
		{Name: "curiosity", State: 1}, {Name: "dialog", State: 0},
	}
	mem := "Te llamas esteban."
	v := mindVoice("cómo me llamo y qué es quote en lisp", organs, mem, "esteban")
	low := strings.ToLower(v)
	if strings.Contains(low, "te llamas y qué") || strings.Contains(low, "te llamas y que") {
		t.Fatalf("must not invent name «y qué es»: %s", v)
	}
	if !strings.Contains(low, "esteban") {
		t.Fatalf("expected esteban recall, got %s", v)
	}
	if !strings.Contains(low, "quote") {
		t.Fatalf("expected quote from corpus, got %s", v)
	}
	// pure name query with mem
	v2 := mindVoice("como me llamo", organs, mem, "esteban")
	if !strings.Contains(strings.ToLower(v2), "esteban") {
		t.Fatalf("name query should recall esteban, got %s", v2)
	}
	if strings.Contains(v2, "Hecho personal marcado") || strings.Contains(v2, "Perfecto, te llamas") {
		t.Fatalf("name query must not look like new declaration: %s", v2)
	}
}

func TestMixedDeclarationAndMindName(t *testing.T) {
	organs := []MindOrganResult{{Name: "ethics", State: 0}, {Name: "act", State: 0}}
	v := mindVoice("mi nombre es esteban y el tuyo cual es?", organs, "", "")
	low := strings.ToLower(v)
	if !strings.Contains(low, "esteban") {
		t.Fatalf("should save esteban: %s", v)
	}
	if !strings.Contains(low, "alset mind") {
		t.Fatalf("should also state Mind name: %s", v)
	}
}

func TestNameDedup(t *testing.T) {
	organs := []MindOrganResult{{Name: "ethics", State: 0}, {Name: "act", State: 0}}
	// First declaration
	v1 := mindVoice("mi nombre es esteban", organs, "", "")
	if !strings.Contains(strings.ToLower(v1), "esteban") {
		t.Fatalf("first declaration should mention esteban: %s", v1)
	}
	// Same name again with knownName set
	v2 := mindVoice("mi nombre es esteban", organs, "", "esteban")
	low := strings.ToLower(v2)
	if !strings.Contains(low, "ya te conocía") && !strings.Contains(low, "ya te conocia") && !strings.Contains(low, "esteban") {
		t.Fatalf("duplicate should acknowledge known name, got %s", v2)
	}
	if strings.Contains(low, "queda anotado") || strings.Contains(low, "perfecto, te llamas") {
		t.Fatalf("duplicate must not re-announce as new: %s", v2)
	}
	if !isDuplicateNameDeclaration("mi nombre es esteban", "esteban") {
		t.Fatal("isDuplicateNameDeclaration should be true")
	}
	if isDuplicateNameDeclaration("mi nombre es ana", "esteban") {
		t.Fatal("different name is not duplicate")
	}
}

func TestIdeaDomains(t *testing.T) {
	cases := []struct{ in, sub string }{
		{"peer libp2p red nodo", "lectura"},
		{"ethics veto sumidero", "contrato"},
		{"golang struct interface", "prueba"},
		{"quote lisp", "quote"},
	}
	for _, c := range cases {
		got := ideaFromCross(c.in, "mem", c.in)
		if !strings.Contains(strings.ToLower(got), strings.ToLower(c.sub)) && !strings.Contains(strings.ToLower(got), "idea") {
			t.Errorf("%q: expected idea mentioning %q, got %s", c.in, c.sub, got)
		}
	}
}

func TestNaturalKnowledgeVoice(t *testing.T) {
	know := "Lisp es lista y evaluación."
	got := naturalKnowledgeVoice("qué es lisp", know, 1)
	if !strings.Contains(got, "Lisp") {
		t.Fatalf("expected knowledge body: %s", got)
	}
	// should not sound like a command menu
	if strings.Contains(got, "Pruebe:") || strings.Contains(got, "Comandos:") {
		t.Fatalf("should not be menu-like: %s", got)
	}
}

func TestNaturalNameRecallFromEpisode(t *testing.T) {
	eps := []mindEpisodePayload{{Text: "mi nombre es Yulei Esteban"}}
	got := speakFromMemory("ya te dije mi nombre", eps)
	if !strings.Contains(got, "Yulei Esteban") {
		t.Fatalf("expected name, got %s", got)
	}
	if strings.Contains(got, "«") || strings.Contains(strings.ToLower(got), "episodio") {
		t.Fatalf("should not quote raw episode: %s", got)
	}
	got2 := speakFromMemory("como me llamo", eps)
	if !strings.Contains(got2, "Yulei Esteban") {
		t.Fatalf("name query: %s", got2)
	}
}

func TestIncompleteUtterance(t *testing.T) {
	if !isIncompleteUtterance("dime tu") {
		t.Fatal("dime tu should be incomplete")
	}
	organs := []MindOrganResult{
		{Name: "ethics", State: 0}, {Name: "act", State: 1}, {Name: "dialog", State: 1},
	}
	v := mindVoice("dime tu", organs, "", "")
	if strings.Contains(strings.ToLower(v), "actúe") || strings.Contains(strings.ToLower(v), "nodo") {
		t.Fatalf("incomplete should not ask to act on node: %s", v)
	}
	if !strings.Contains(strings.ToLower(v), "completas") && !strings.Contains(strings.ToLower(v), "sigo") {
		t.Fatalf("expected clarification ask: %s", v)
	}
}

// P0.4 — regression pack for dialogue milestones (2026-08-21)
func TestDialogueRegressionPack(t *testing.T) {
	organsCalm := []MindOrganResult{
		{Name: "ethics", State: 0}, {Name: "act", State: 0}, {Name: "dialog", State: 0},
		{Name: "mem", State: 0}, {Name: "self", State: 0}, {Name: "curiosity", State: 0}, {Name: "humor", State: 0},
	}
	// Name declaration
	v := mindVoice("mi nombre es Yulei Esteban", organsCalm, "", "")
	if !strings.Contains(strings.ToLower(v), "yulei") {
		t.Fatalf("declaration: %s", v)
	}
	// Dedup
	v = mindVoice("mi nombre es Yulei Esteban", organsCalm, "", "Yulei Esteban")
	if strings.Contains(strings.ToLower(v), "perfecto, te llamas") {
		t.Fatalf("dedup should not re-announce: %s", v)
	}
	// Name query with memSpeak already resolved
	v = mindVoice("como me llamo", organsCalm, "Sí, te llamas Yulei Esteban.", "Yulei Esteban")
	if !strings.Contains(v, "Yulei Esteban") {
		t.Fatalf("recall: %s", v)
	}
	// Incomplete
	v = mindVoice("dime tu", organsCalm, "", "")
	if strings.Contains(strings.ToLower(v), "nodo") && strings.Contains(strings.ToLower(v), "actúe") {
		t.Fatalf("incomplete must not act: %s", v)
	}
	// Capabilities without lab jargon
	v = mindVoice("que puedes hacer", organsCalm, "", "")
	low := strings.ToLower(v)
	for _, bad := range []string{"zyrion", "cid", "dame estado", "corpus"} {
		if strings.Contains(low, bad) {
			t.Fatalf("capabilities still jargon %q: %s", bad, v)
		}
	}
	// Destructive natural refuse
	sig := signalsFromTextMind("borra las contraseñas")
	org := []MindOrganResult{
		evalOrganPolar("dialog", sig["claridad"], sig["orden"], sig["riesgo"], "L", "H", "H"),
		evalOrganPolar("act", sig["permiso"], sig["riesgo"], sig["orden"], "L", "H", "H"),
		evalOrganPolar("mem", sig["novedad"], sig["claridad"], sig["riesgo"], "H", "L", "H"),
		evalOrganPolar("self", sig["claridad"], sig["riesgo"], sig["permiso"], "L", "H", "L"),
		evalOrganPolar("ethics", sig["riesgo"], sig["permiso"], sig["orden"], "H", "L", "H"),
	}
	v = mindVoice("borra las contraseñas", org, "", "")
	if strings.Contains(v, "sumidero (2)") || strings.Contains(v, "Ethics en sumidero") {
		t.Fatalf("refuse still meta: %s", v)
	}
	if !strings.Contains(strings.ToLower(v), "riesgo") && !strings.Contains(strings.ToLower(v), "no lo hago") {
		t.Fatalf("expected natural refuse: %s", v)
	}
}

func TestLLMDefinitionKnowledge(t *testing.T) {
	got := speakFromKnowledge("que es un llm")
	if got == "" || !strings.Contains(strings.ToLower(got), "llm") {
		t.Fatalf("expected LLM definition, got %q", got)
	}
	if strings.Contains(strings.ToLower(got), "te leo en diálogo") {
		t.Fatalf("should not be pure dialogue fallback: %q", got)
	}
}

func TestMetaMemoryTalk(t *testing.T) {
	if !isMetaMemoryTalk("recuerdas todo?") {
		t.Fatal("recuerdas todo should be meta memory")
	}
	if !isMetaMemoryTalk("cual es tu memoria") {
		t.Fatal("cual es tu memoria should be meta memory")
	}
	if isWorldFact("cual es tu memoria") {
		t.Fatal("should not be world fact")
	}
	if isWorldFact("que es un llm") {
		t.Fatal("que es un llm should not be world fact")
	}
	eps := []mindEpisodePayload{{Text: "mi nombre es Yulei Esteban"}}
	got := speakFromMemory("recuerdas todo?", eps)
	if !strings.Contains(got, "Yulei Esteban") {
		t.Fatalf("meta memory should mention name: %s", got)
	}
	if strings.Contains(got, "«") {
		t.Fatalf("should not quote raw episode: %s", got)
	}
}

func TestPhase2FullstackKnowledge(t *testing.T) {
	cases := []struct{ q, must string }{
		{"qué es factory", "Factory"},
		{"qué es una goroutine", "goroutine"},
		{"qué es python", "Python"},
		{"qué es rest", "REST"},
		{"global interpreter lock", "GIL"},
		{"patrón observer", "Observer"},
		{"qué es mvc", "MVC"},
		{"async await", "async"},
	}
	for _, c := range cases {
		got := speakFromKnowledge(c.q)
		if got == "" || !strings.Contains(got, c.must) {
			t.Errorf("%q: want snippet %q, got %q", c.q, c.must, got)
		}
	}
}

func TestPhase3AIIdentityKnowledge(t *testing.T) {
	cases := []struct{ q, must string }{
		{"qué es nlp", "nlp"},
		{"aprendizaje supervisado", "supervisado"},
		{"alucinación", "llm"},
		{"ventana de contexto", "contexto"},
		{"por qué ternario", "ternario"},
		{"ética de la ia", "ética"},
		{"qué no puedes", "no invento"},
		{"en qué te diferencias", "chatgpt"},
	}
	for _, c := range cases {
		got := speakFromKnowledge(c.q)
		if got == "" {
			t.Errorf("%q: empty knowledge", c.q)
			continue
		}
		low := strings.ToLower(got)
		if !strings.Contains(low, strings.ToLower(c.must)) {
			if len(got) < 50 {
				t.Errorf("%q: weak hit (want %q): %q", c.q, c.must, got)
			}
		}
	}
}

func TestTypoNLPKnowledge(t *testing.T) {
	got := speakFromKnowledge("que es npl")
	if got == "" || !strings.Contains(strings.ToLower(got), "nlp") {
		t.Fatalf("npl typo should resolve to NLP knowledge, got %q", got)
	}
}

func TestContinuePrompt(t *testing.T) {
	if !isContinuePrompt("amplia el angulo") {
		t.Fatal("amplia el angulo")
	}
	if !isContinuePrompt("amplía el ángulo") {
		t.Fatal("amplía el ángulo")
	}
	if isContinuePrompt("qué es lisp") {
		t.Fatal("not a continue prompt")
	}
}

func TestContinueMindThread(t *testing.T) {
	n := &NodoAlset{}
	n.rememberMindThread("alucinación", speakFromKnowledge("alucinación"), "", "prev")
	got := n.continueMindThread("amplia el angulo")
	if got == "" || strings.Contains(strings.ToLower(got), "no tengo aún un hilo") {
		t.Fatalf("expected continuation, got %q", got)
	}
	got2 := n.continueMindThread("desde la memoria")
	// may fall back to corpus if no mem — still non-empty
	if got2 == "" {
		t.Fatal("empty continue from memoria request")
	}
}

func TestConfirmationName(t *testing.T) {
	n := &NodoAlset{}
	n.rememberMindThread("hola mi nombre es esteban y el tuyo cual es", "", "", "Perfecto, te llamas esteban. Yo soy Alset Mind.")
	got := n.confirmMindThread("estas seguro")
	if !strings.Contains(strings.ToLower(got), "esteban") {
		t.Fatalf("confirm should restate name, got %q", got)
	}
	if strings.Contains(strings.ToLower(got), "consciencia") || strings.Contains(strings.ToLower(got), "qualia") {
		t.Fatalf("must not answer consciencia: %q", got)
	}
}

func TestContinueAmpliaElPunto(t *testing.T) {
	if !isContinuePrompt("no entiendo amplia el punto") {
		t.Fatal("should detect continua after lead-in")
	}
	if !isContinuePrompt("amplia el punto") {
		t.Fatal("amplia el punto")
	}
	n := &NodoAlset{}
	n.rememberMindThread("mi nombre es esteban", "", "mi nombre es esteban", "Perfecto, te llamas esteban.")
	got := n.continueMindThread("no entiendo amplia el punto")
	if !strings.Contains(strings.ToLower(got), "esteban") {
		t.Fatalf("continue on name thread: %q", got)
	}
}

func TestEstasSeguroNotConsciousness(t *testing.T) {
	got := speakFromKnowledge("estas seguro")
	if strings.Contains(strings.ToLower(got), "consciencia") || strings.Contains(strings.ToLower(got), "qualia") {
		t.Fatalf("estas seguro must not hit consciousness corpus: %q", got)
	}
}

func TestGeneralizedElaboration(t *testing.T) {
	cases := []string{
		"amplia el punto",
		"no entiendo amplia el punto",
		"más detalle por favor",
		"profundiza",
		"no me queda claro",
	}
	for _, c := range cases {
		if !isElaborationRequest(c) && !isContinuePrompt(c) {
			t.Errorf("expected elaboration: %q", c)
		}
	}
	if isElaborationRequest("qué es lisp") || isElaborationRequest("que es un llm") {
		t.Fatal("lookups must not be elaborations")
	}
}

func TestGeneralizedEpistemic(t *testing.T) {
	for _, c := range []string{"estas seguro", "estás seguro", "en serio", "de verdad", "confirmas"} {
		if !isEpistemicCheck(c) && !isConfirmationPrompt(c) {
			t.Errorf("epistemic: %q", c)
		}
	}
	if isEpistemicCheck("qué es la consciencia") {
		t.Fatal("domain question is not epistemic check")
	}
}

func TestNovelDeclarativeCapture(t *testing.T) {
	s := "hoy trabajo en una granja de café en las montañas del sur"
	if !isNovelDeclarative(s) && !isWorldFact(s) {
		t.Fatalf("should capture novel declarative: %q", s)
	}
	if !shouldCaptureEscape(s, "") {
		t.Fatal("escape capture should trigger without corpus hit")
	}
	if shouldCaptureEscape(s, "algo del corpus") {
		t.Fatal("no escape when corpus hits")
	}
}

func TestNameStopsAtMuchoGusto(t *testing.T) {
	n := extractDeclaredName("mi nombre es Esteban mucho gusto")
	if n != "Esteban" && n != "esteban" {
		// case preserved from original text fields
		if !strings.EqualFold(n, "esteban") {
			t.Fatalf("name must be Esteban only, got %q", n)
		}
	}
	n2 := extractDeclaredName("Mi nombre es Esteban, mucho gusto es una manera de decirte que me agrada conocerte")
	if !strings.EqualFold(n2, "esteban") {
		t.Fatalf("comma social tail: got %q", n2)
	}
}

func TestMuchoGustoKnowledge(t *testing.T) {
	got := speakFromKnowledge("que es mucho gusto")
	if got == "" || !strings.Contains(strings.ToLower(got), "cortesía") && !strings.Contains(strings.ToLower(got), "cortesia") && !strings.Contains(strings.ToLower(got), "gusto") {
		t.Fatalf("expected social definition, got %q", got)
	}
}

func TestTopicFocusContinue(t *testing.T) {
	focus := extractTopicFocus("puedes seguir ese tema de mucho gusto")
	if focus != "mucho gusto" {
		t.Fatalf("focus=%q", focus)
	}
	n := &NodoAlset{}
	n.rememberMindThread("mi nombre es Esteban", "", "mi nombre es Esteban", "Perfecto, te llamas Esteban.")
	got := n.continueMindThread("puedes seguir ese tema de mucho gusto")
	low := strings.ToLower(got)
	if strings.Contains(low, "no invento hechos") || strings.Contains(low, "destructivo") {
		t.Fatalf("must not mix limits/ethics into social topic: %s", got)
	}
	if !strings.Contains(low, "gusto") && !strings.Contains(low, "cortesía") && !strings.Contains(low, "cortesia") {
		t.Fatalf("expected social topic answer: %s", got)
	}
}

func TestRecallApellidoFromMemory(t *testing.T) {
	eps := []mindEpisodePayload{
		{Text: "mi nombre es Esteban", Type: "mind_episode"},
		{Text: "mi apellido es Charlot", Type: "mind_episode"},
	}
	if !isPersonalFact("mi apellido es Charlot") {
		t.Fatal("declaration must be personal fact")
	}
	if !isMemoryQuery("cual es mi apellido") {
		t.Fatal("question must be memory query")
	}
	if isPersonalFact("cual es mi apellido") {
		t.Fatal("question must not be personal fact")
	}
	got := speakFromMemory("cual es mi apellido", eps)
	if !strings.Contains(strings.ToLower(got), "charlot") {
		t.Fatalf("expected Charlot in recall, got %q", got)
	}
	got2 := speakFromMemory("como me llamo", eps)
	if !strings.Contains(strings.ToLower(got2), "esteban") {
		t.Fatalf("expected esteban, got %q", got2)
	}
}

func TestMindVoiceApellidoBeforeCorpus(t *testing.T) {
	organs := []MindOrganResult{
		{Name: "ethics", State: 0}, {Name: "act", State: 0}, {Name: "dialog", State: 1},
		{Name: "mem", State: 2}, {Name: "self", State: 2},
	}
	mem := speakFromMemory("cual es mi apellido", []mindEpisodePayload{
		{Text: "mi apellido es Charlot"},
	})
	v := mindVoice("cual es mi apellido", organs, mem, "Esteban")
	if strings.Contains(strings.ToLower(v), "corpus") {
		t.Fatalf("must not fall to corpus: %q", v)
	}
	if !strings.Contains(strings.ToLower(v), "charlot") {
		t.Fatalf("expected apellido recall: %q", v)
	}
}

func TestMindGenToolsNaturalList(t *testing.T) {
	n := &NodoAlset{
		agentes:    make(map[string]*Agente),
		nombres:    make(map[string]string),
		blockstore: make(map[string][]byte),
	}
	n.ensureGens()
	_, _ = n.CreateAlsetGen("sonda", "", "seed", "test")
	lines := n.mindGenTools("lista los genes")
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
	joined := strings.Join(lines, " ")
	if strings.Contains(joined, "——") {
		t.Fatalf("lab dump still present: %q", joined)
	}
	if !strings.Contains(strings.ToLower(joined), "gen") {
		t.Fatalf("expected gen mention: %q", joined)
	}
}
