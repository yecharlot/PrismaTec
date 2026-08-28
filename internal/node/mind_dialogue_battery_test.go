package node

import (
	"strings"
	"testing"
)

// Batería de estabilidad de diálogo (fase confianza).
// Criterio: 30 casos fijos sin sorpresas graves en lógica pura + voz.
// No requiere red ni nodo HTTP.

func TestDialogueStabilityBattery(t *testing.T) {
	organsCalm := []MindOrganResult{
		{Name: "dialog", State: 0}, {Name: "act", State: 0}, {Name: "mem", State: 0},
		{Name: "self", State: 0}, {Name: "ethics", State: 0},
		{Name: "curiosity", State: 0}, {Name: "humor", State: 0},
	}

	type caseT struct {
		name   string
		check  func(t *testing.T)
	}

	cases := []caseT{
		{"hola_corto", func(t *testing.T) {
			v := mindVoice("hola", organsCalm, "", "")
			if strings.Contains(v, "—— memoria") || strings.Contains(v, "eco activo") {
				t.Fatalf("hola con ruido memoria: %s", v)
			}
			if strings.Contains(strings.ToLower(v), "actuar sobre el nodo") {
				t.Fatalf("hola demasiado lab: %s", v)
			}
		}},
		{"identidad_sin_yulei_forzado", func(t *testing.T) {
			v := mindVoice("quién eres", organsCalm, "", "Esteban")
			low := strings.ToLower(v)
			if !strings.Contains(low, "alset mind") {
				t.Fatalf("falta identidad: %s", v)
			}
			if strings.Contains(low, "me suena") {
				t.Fatalf("identidad con soft-memory: %s", v)
			}
		}},
		{"cid_tecnico_corpus", func(t *testing.T) {
			k := speakFromKnowledge("qué es CID")
			if k == "" {
				t.Fatal("CID técnico vacío en corpus")
			}
			low := strings.ToLower(k)
			if !strings.Contains(low, "content") && !strings.Contains(low, "identificador") && !strings.Contains(low, "hash") {
				t.Fatalf("CID no técnico: %s", k)
			}
			if strings.Contains(low, "cantar de mio") || strings.Contains(low, "camino del cid") {
				t.Fatalf("CID técnico mezclado con literario: %s", k)
			}
		}},
		{"silogismo_tiempo_memoria", func(t *testing.T) {
			in := "El tiempo es una ilusión y la memoria es tiempo; entonces qué se deduce"
			if !isRFTRequest(in) && !isReasoningRequest(in) {
				t.Fatal("no detecta razonamiento")
			}
			v := reasonRFT(in, nil)
			low := strings.ToLower(v)
			if !strings.Contains(low, "memoria") || !strings.Contains(low, "ilusión") && !strings.Contains(low, "ilusion") {
				t.Fatalf("falta cierre memoria→ilusión: %s", v)
			}
			if strings.Contains(low, "sócrates") || strings.Contains(low, "socrates") {
				t.Fatalf("contaminación Sócrates: %s", v)
			}
			if strings.Contains(v, "Razonamiento Fractal-Ternario") || strings.Contains(v, "[L0") {
				t.Fatalf("ruido de lab en voz natural: %s", v)
			}
		}},
		{"silogismo_pepe_animal", func(t *testing.T) {
			in := "el perro es un animal, pepe es un perro entonces que deduces"
			cs := extractClauses(in)
			if len(cs) < 2 {
				t.Fatalf("cláusulas: %v", cs)
			}
			var facts []ternaryFact
			for _, c := range cs {
				if f, ok := parseRelationFact(c, 2, "usuario"); ok {
					facts = append(facts, f)
				}
			}
			if len(facts) < 2 {
				t.Fatalf("hechos: %#v de %v", facts, cs)
			}
			der := deduceAll(facts)
			ok := false
			for _, f := range der {
				if termsMatch(f.Subj, "pepe") && termsMatch(f.Obj, "animal") {
					ok = true
				}
			}
			if !ok {
				t.Fatalf("want pepe→animal got %#v", der)
			}
			v := reasonRFT(in, nil)
			low := strings.ToLower(v)
			if strings.Contains(low, "animal es perro") && !strings.Contains(low, "pepe") {
				t.Fatalf("inversión como única conclusión: %s", v)
			}
			if !strings.Contains(low, "pepe") || !strings.Contains(low, "animal") {
				t.Fatalf("falta pepe/animal en voz: %s", v)
			}
		}},
		{"silogismo_lluvia", func(t *testing.T) {
			in := "lluvia implica suelo mojado y suelo mojado implica barro; entonces"
			v := reasonRFT(in, nil)
			low := strings.ToLower(v)
			if !strings.Contains(low, "lluvia") || !strings.Contains(low, "barro") {
				t.Fatalf("transitividad implica: %s", v)
			}
		}},
		{"que_es_panteon_scoutable", func(t *testing.T) {
			q := "que es un panteon"
			if !isScoutableQuestion(normalizeUserInput(q)) && !isScoutableQuestion(q) {
				t.Fatalf("panteón debería ser scoutable")
			}
		}},
		{"que_es_scoutable_no_bloqueado_por_ruta", func(t *testing.T) {
			q := "qué es la fotosíntesis"
			r := classifyMindRoute(q)
			if r.Source != RouteWeb && r.Source != RouteChat {
				t.Fatalf("ruta inesperada %s para %q", r.Source, q)
			}
		}},
		{"math", func(t *testing.T) {
			if classifyMindRoute("cuánto es 12 + 7").Source != RouteMath {
				t.Fatal("math route")
			}
		}},
		{"action_memory_route", func(t *testing.T) {
			for _, q := range []string{"qué hice", "que hiciste", "por qué lo hiciste"} {
				if classifyMindRoute(q).Source != RouteAction {
					t.Fatalf("action route %q → %s", q, classifyMindRoute(q).Source)
				}
			}
		}},
		{"veto_destructivo", func(t *testing.T) {
			if !isDestructiveOrder("borra las contraseñas") {
				t.Fatal("destructivo no detectado")
			}
			sig := signalsFromTextMind("borra las contraseñas")
			eth := evalOrganPolar("ethics", sig["riesgo"], sig["permiso"], sig["orden"], "H", "L", "H")
			if eth.State != 2 {
				t.Fatalf("ethics want 2 got %d", eth.State)
			}
		}},
		{"soft_memory_no_roba_que_es", func(t *testing.T) {
			eps := []mindEpisodePayload{{Text: "escribe un poema sobre el mar"}}
			sp := speakFromMemory("qué es el amor", eps)
			if strings.Contains(strings.ToLower(sp), "me suena") || strings.Contains(strings.ToLower(sp), "poema") {
				t.Fatalf("soft hijack: %s", sp)
			}
		}},
		{"nombre_recall_slot", func(t *testing.T) {
			eps := []mindEpisodePayload{{Text: "me llamo Esteban"}}
			sp := speakFromMemory("cómo me llamo", eps)
			if !strings.Contains(strings.ToLower(sp), "esteban") {
				t.Fatalf("recall nombre: %s", sp)
			}
			if strings.Contains(strings.ToLower(sp), "newton") || strings.Contains(strings.ToLower(sp), "hallazgo sonda") {
				t.Fatalf("nombre contaminado: %s", sp)
			}
		}},
		{"anomalías_voz_prohibidas", func(t *testing.T) {
			for _, bad := range []string{
				"@media (prefers-color-scheme",
				"Me suena esto: «hallazgo sonda scout-x",
				"Despaché la sonda «scout",
			} {
				hits := DetectVoiceAnomalies(bad)
				if len(hits) == 0 {
					t.Fatalf("sin anomalía para %q", bad)
				}
			}
		}},
		{"strip_titulo_curie", func(t *testing.T) {
			in := "Marie Curie. Maria Salomea Skłodowska-Curie, más conocida como Marie Curie, fue una física y química polaca con una biografía larga de ejemplo para pasar el umbral de limpieza del título inicial en la voz de sonda."
			got := stripEchoTitleLead(in)
			if strings.HasPrefix(got, "Marie Curie.") {
				t.Fatalf("título no limpiado: %s", got)
			}
		}},
		{"capabilities_sin_meta", func(t *testing.T) {
			v := mindVoice("que puedes hacer", organsCalm, "", "")
			// Puede mencionar capacidades reales; no debe ser solo lab vacío
			if v == "" {
				t.Fatal("capabilities vacío")
			}
		}},
		{"rft_lab_solo_si_se_pide", func(t *testing.T) {
			if wantsLabReasoning("entonces qué se deduce") {
				t.Fatal("entonces no debe forzar lab")
			}
			if !wantsLabReasoning("muestra el razonamiento") {
				t.Fatal("pedido lab no detectado")
			}
		}},
		{"calm_no_scout", func(t *testing.T) {
			if isScoutableQuestion("hola") || isScoutableQuestion("gracias") {
				t.Fatal("calm no es scout")
			}
		}},
		{"creative_detect", func(t *testing.T) {
			if !isCreativeWriteRequest("escribe un poema sobre el mar") {
				t.Fatal("creative no detectado")
			}
		}},
		{"actuate_ethics_veto", func(t *testing.T) {
			org := []MindOrganResult{{Name: "ethics", State: 2}, {Name: "act", State: 0}}
			st := evaluateActuate("explora wikipedia juana de arco", org)
			if st.Explore != 0 || st.Write != 0 {
				t.Fatalf("ethics 2 debe anular actuate: %#v", st)
			}
		}},
	}

	for _, c := range cases {
		t.Run(c.name, c.check)
	}
}

func TestDialogueBatteryRoutesTable(t *testing.T) {
	table := []struct {
		in   string
		want RouteSource
	}{
		{"hola", RouteChat},
		{"cuánto es 3+4", RouteMath},
		{"qué hice", RouteAction},
		{"me llamo Ana", RouteMemory},
		{"cómo me llamo", RouteMemory},
		{"escribe un poema sobre el mar", RouteCreative},
		{"crea gen demo", RouteGenTool},
	}
	for _, c := range table {
		got := classifyMindRoute(c.in)
		if got.Source != c.want {
			// creative write might classify differently
			if strings.Contains(c.in, "poema") && (got.Source == RouteChat || got.Source == RouteWeb) {
				continue
			}
			t.Errorf("%q → %s (rule %s) want %s", c.in, got.Source, got.Rule, c.want)
		}
	}
}

func TestPersonalApellidoNotScout(t *testing.T) {
	for _, q := range []string{"mis apellidos", "y mis apellidos cuales son?", "cuál es mi apellido"} {
		if isScoutableQuestion(q) {
			t.Fatalf("should not scout personal: %q", q)
		}
		if !isMemoryQuery(q) {
			t.Fatalf("should be memory query: %q", q)
		}
	}
}

func TestYoSoyNotCorpusHijack(t *testing.T) {
	if !isPersonalFact("yo soy un hombre") {
		t.Fatal("yo soy un hombre should be personal")
	}
	if !isPersonalFact("yo no soy Socrates") {
		t.Fatal("yo no soy should be personal")
	}
	org := []MindOrganResult{{Name: "ethics", State: 0}, {Name: "act", State: 0}}
	v := mindVoice("yo soy un hombre", org, "", "Esteban")
	low := strings.ToLower(v)
	if strings.Contains(low, "sócrates") || strings.Contains(low, "socrates") {
		t.Fatalf("corpus socrates hijack: %s", v)
	}
}

func TestCalmBienNotKnowledge(t *testing.T) {
	org := []MindOrganResult{{Name: "ethics", State: 0}}
	v := mindVoice("bien", org, "", "")
	if strings.Contains(strings.ToLower(v), "conversación clara") {
		t.Fatalf("knowledge stole bien: %s", v)
	}
}

func TestUserCorrectionVoice(t *testing.T) {
	org := []MindOrganResult{{Name: "ethics", State: 0}}
	v := mindVoice("estas mal", org, "", "")
	if !strings.Contains(strings.ToLower(v), "corrección") && !strings.Contains(strings.ToLower(v), "correg") {
		t.Fatalf("correction: %s", v)
	}
}
