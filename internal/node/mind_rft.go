package node

import (
	"fmt"
	"strings"
)

// Razonamiento Fractal-Ternario (RFT)
// 0 = cierra camino; 1 = sostiene en este nivel; 2 = sumidero + salto de nivel.
// No es predicción de tokens: árbol de hechos, reglas y saltos etiquetados.

type rftNode struct {
	Fact     ternaryFact
	Level    int    // 0 base, 1 derivado lineal, 2+ salto fractal
	Kind     string // premisa | trans | salto-inv | salto-neg | salto-abstrae | salto-concreta
	Parent   string // factKey del origen
	Children []string
}

type rftTree struct {
	Nodes   []rftNode
	Saltos  []rftNode
	Voice   string
	Primary ternaryFact
}

func isRFTRequest(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(low, "fractal") || strings.Contains(low, "rft") ||
		strings.Contains(low, "salto lógico") || strings.Contains(low, "salto logico") ||
		strings.Contains(low, "autosimilar") || strings.Contains(low, "no lineal") {
		return true
	}
	// cadenas con ≥2 premisas + entonces / por tanto
	if isReasoningRequest(low) {
		n := 0
		for _, c := range extractClauses(s) {
			if _, ok := parseRelationFact(c, 1, "x"); ok {
				n++
			}
		}
		return n >= 2
	}
	return false
}

// confZyrionPath: combina confianzas; 2 propaga como sumidero de incertidumbre.
func confZyrionPath(vals ...int) int {
	if len(vals) == 0 {
		return 0
	}
	has2 := false
	min := 2
	for _, v := range vals {
		if v < 0 {
			v = 0
		}
		if v > 2 {
			v = 2
		}
		if v == 2 {
			has2 = true
		}
		if v < min {
			min = v
		}
	}
	// Si alguna premisa es 2, el camino lineal queda en 2 (sumidero) → dispara salto
	if has2 {
		return 2
	}
	return min
}

func buildRFTTree(facts []ternaryFact) rftTree {
	var tree rftTree
	seen := map[string]bool{}
	add := func(n rftNode) {
		k := factKey(n.Fact)
		if seen[k] && n.Kind != "salto-inv" && n.Kind != "salto-neg" && n.Kind != "salto-abstrae" {
			return
		}
		// saltos pueden coexistir con misma key distinta kind
		sk := k + "|" + n.Kind + "|" + fmt.Sprintf("%d", n.Level)
		if seen[sk] {
			return
		}
		seen[sk] = true
		seen[k] = true
		tree.Nodes = append(tree.Nodes, n)
		if strings.HasPrefix(n.Kind, "salto") {
			tree.Saltos = append(tree.Saltos, n)
		}
	}

	for _, f := range facts {
		if !validReasonTerm(f.Subj) || !validReasonTerm(f.Obj) {
			continue
		}
		add(rftNode{Fact: f, Level: 0, Kind: "premisa"})
	}

	// Nivel 1: cierre lineal (mismas reglas que deduceAll)
	linear := deduceAll(facts)
	for _, f := range linear {
		add(rftNode{Fact: f, Level: 1, Kind: "trans", Parent: f.Src})
	}

	// Detectar caminos con sumidero 2 o premisas con conf 2
	needSalto := false
	for _, f := range facts {
		if f.Conf == 2 {
			// conf 2 en RFT = indeterminado en este nivel, no "verdadero fuerte"
			// En premisas del usuario tratamos 2 como "afirmado con matiz de salto"
			needSalto = true
		}
	}
	// Si solo hay premisas conf 2 (usuario suele mandar 2), el salto se activa
	// por estructura multi-premisa, no por valor.
	if len(facts) >= 2 {
		needSalto = true
	}

	pool := append([]ternaryFact{}, facts...)
	pool = append(pool, linear...)

	if needSalto {
		for _, f := range pool {
			// Salto-inversión (escala superior): A es B → explorar B es A (conf 1 si conf origen ≥1)
			if isIdentityRel(f.Rel) && f.Conf >= 1 {
				inv := ternaryFact{
					Subj: f.Obj, Rel: "es", Obj: f.Subj,
					Conf: 1, Src: "rft:salto-inv",
				}
				if validReasonTerm(inv.Subj) && validReasonTerm(inv.Obj) {
					add(rftNode{Fact: inv, Level: 2, Kind: "salto-inv", Parent: factKey(f)})
				}
			}
			// Salto-abstrae: A es B → B es (categoría débil) no; en su lugar
			// Salto-negación acotada: si obj es "ilusión/falso/vacío", reality no_es subj
			obj := f.Obj
			if isIdentityRel(f.Rel) && isIllusionLike(obj) {
				neg := ternaryFact{
					Subj: "realidad", Rel: "no_es", Obj: f.Subj,
					Conf: 1, Src: "rft:salto-neg",
				}
				add(rftNode{Fact: neg, Level: 2, Kind: "salto-neg", Parent: factKey(f)})
			}
			// Si conclusión lineal X es ilusión → realidad no_es X
			if isIdentityRel(f.Rel) && isIllusionLike(f.Obj) {
				neg2 := ternaryFact{
					Subj: "realidad", Rel: "no_es", Obj: f.Subj,
					Conf: 1, Src: "rft:salto-neg-lineal",
				}
				add(rftNode{Fact: neg2, Level: 2, Kind: "salto-neg", Parent: factKey(f)})
			}
		}
		// Cadena especial tiempo/memoria/ilusión ya cubierta por trans + salto-neg
		// Salto-concreta: desactivado en masa (el corpus no debe inundar el árbol).
		// La concreción se limita a collectReasonFactsOpts filtrado.
		// Segundo cierre fractal sobre nodos de salto (autosimilitud)
		var saltoFacts []ternaryFact
		for _, n := range tree.Saltos {
			saltoFacts = append(saltoFacts, n.Fact)
		}
		base := append(pool, saltoFacts...)
		for _, f := range deduceAll(base) {
			add(rftNode{Fact: f, Level: 3, Kind: "trans", Parent: "rft:cierre-fractal"})
		}
	}

	// Conclusión: priorizar cierre L1 de premisas Src=usuario
	var userFacts []ternaryFact
	userTerms := map[string]bool{}
	for _, n := range tree.Nodes {
		if n.Kind == "premisa" && n.Fact.Src == "usuario" {
			userFacts = append(userFacts, n.Fact)
			for _, w := range strings.Fields(n.Fact.Subj + " " + n.Fact.Obj) {
				if len([]rune(w)) >= 3 {
					userTerms[w] = true
				}
			}
		}
	}
	best := ternaryFact{}
	bestSc := -1
	// Preferir cierres L1 (transitividad) sobre cualquier salto
	for _, f := range deduceAll(userFacts) {
		if f.Subj == f.Obj {
			continue
		}
		sc := 30 + f.Conf
		// Priorizar sujeto que aparece en premisas como individuo (pepe), no inversión
		for _, uf := range userFacts {
			if termsMatch(f.Subj, uf.Subj) {
				sc += 5
			}
		}
		if sc > bestSc {
			bestSc = sc
			best = f
		}
	}
	// Saltos solo si NO hay cierre lineal
	if bestSc < 0 {
		for _, n := range tree.Nodes {
			if n.Kind != "salto-neg" {
				continue // inversión sola no es conclusión guía
			}
			if n.Fact.Subj == n.Fact.Obj {
				continue
			}
			hit := false
			for w := range userTerms {
				if strings.Contains(n.Fact.Subj, w) || strings.Contains(n.Fact.Obj, w) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			sc := 12 + n.Fact.Conf
			if sc > bestSc {
				bestSc = sc
				best = n.Fact
			}
		}
	}
	tree.Primary = best
	tree.Voice = formatRFTVoice(tree)
	return tree
}

func isIllusionLike(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	keys := []string{"ilusión", "ilusion", "engaño", enganoFix(), "falso", "vacío", "vacio", "sueño", "sueno"}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

func enganoFix() string { return "engano" }

func wantsLabReasoning(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	keys := []string{
		"muestra el razonamiento", "muestra el rft", "explica el rft", "explica el razonamiento",
		"árbol lógico", "arbol logico", "pasos de la deducción", "pasos de la deduccion",
		"modo laboratorio", "modo lab", "detalle ternario", "ver premisas",
	}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

func formatRFTVoice(tree rftTree) string {
	return formatRFTVoiceMode(tree, false)
}

func formatRFTVoiceMode(tree rftTree, lab bool) string {
	if len(tree.Nodes) == 0 {
		return ""
	}
	userTerms := map[string]bool{}
	var premises []ternaryFact
	for _, n := range tree.Nodes {
		if n.Kind == "premisa" && n.Fact.Src == "usuario" {
			premises = append(premises, n.Fact)
			for _, w := range strings.Fields(n.Fact.Subj + " " + n.Fact.Obj) {
				if len([]rune(w)) >= 3 {
					userTerms[w] = true
				}
			}
		}
	}
	if tree.Primary.Subj != "" {
		setLastReasonConclusion(conclusionSentence(tree.Primary))
	}

	if !lab {
		return formatRFTNatural(premises, tree)
	}

	var b strings.Builder
	b.WriteString("Razonamiento Fractal-Ternario (detalle de laboratorio):\n")
	for _, p := range premises {
		b.WriteString(fmt.Sprintf("· Premisa: «%s» %s «%s»\n", p.Subj, relVoice(p.Rel), p.Obj))
	}
	shown := 0
	for _, n := range tree.Nodes {
		if n.Kind != "trans" || n.Level != 1 || n.Fact.Subj == n.Fact.Obj {
			continue
		}
		hit := len(userTerms) == 0
		for w := range userTerms {
			if strings.Contains(n.Fact.Subj, w) || strings.Contains(n.Fact.Obj, w) {
				hit = true
				break
			}
		}
		if !hit || shown >= 3 {
			continue
		}
		b.WriteString(fmt.Sprintf("· Cierre: «%s» %s «%s»\n", n.Fact.Subj, relVoice(n.Fact.Rel), n.Fact.Obj))
		shown++
	}
	ns := 0
	for _, n := range tree.Saltos {
		if ns >= 3 {
			break
		}
		ok := len(userTerms) == 0
		for w := range userTerms {
			if strings.Contains(n.Fact.Subj, w) || strings.Contains(n.Fact.Obj, w) {
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("· Salto: «%s» %s «%s»\n", n.Fact.Subj, relVoice(n.Fact.Rel), n.Fact.Obj))
		ns++
	}
	if tree.Primary.Subj != "" {
		b.WriteString(fmt.Sprintf("· Conclusión: «%s» %s «%s».\n", tree.Primary.Subj, relVoice(tree.Primary.Rel), tree.Primary.Obj))
	}
	return strings.TrimSpace(b.String())
}

// formatRFTNatural — lo que ve el usuario: diálogo, no laboratorio.
func formatRFTNatural(premises []ternaryFact, tree rftTree) string {
	if tree.Primary.Subj == "" && len(premises) == 0 {
		return ""
	}
	var b strings.Builder
	if len(premises) >= 2 {
		b.WriteString("Si ")
		for i, p := range premises {
			if i >= 3 {
				break
			}
			if i > 0 {
				b.WriteString(" y ")
			}
			b.WriteString(p.Subj + " " + relVoice(p.Rel) + " " + p.Obj)
		}
		b.WriteString(", entonces ")
	} else if len(premises) == 1 {
		b.WriteString("Partiendo de que " + premises[0].Subj + " " + relVoice(premises[0].Rel) + " " + premises[0].Obj + ", ")
	}
	if tree.Primary.Subj != "" {
		if len(premises) >= 2 {
			b.WriteString(tree.Primary.Subj + " " + relVoice(tree.Primary.Rel) + " " + tree.Primary.Obj + ".")
		} else {
			b.WriteString("se sostiene que " + tree.Primary.Subj + " " + relVoice(tree.Primary.Rel) + " " + tree.Primary.Obj + ".")
		}
	}
	// Un salto útil como matiz conversacional (no etiqueta de lab)
	for _, n := range tree.Saltos {
		if n.Kind != "salto-neg" {
			continue
		}
		hit := false
		for _, p := range premises {
			if strings.Contains(n.Fact.Obj, p.Subj) || strings.Contains(n.Fact.Obj, p.Obj) ||
				strings.Contains(p.Subj, n.Fact.Obj) || strings.Contains(p.Obj, n.Fact.Obj) {
				hit = true
				break
			}
		}
		if hit {
			b.WriteString(" En otro sentido, " + n.Fact.Subj + " " + relVoice(n.Fact.Rel) + " " + n.Fact.Obj + ".")
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func reasonRFT(userText string, extra []ternaryFact) string {
	// Premisas SOLO del mensaje actual. La memoria de otros temas no entra al árbol.
	facts := collectReasonFactsOpts(userText, false)
	seed := map[string]bool{}
	for _, f := range facts {
		for _, w := range strings.Fields(f.Subj + " " + f.Obj) {
			if len([]rune(w)) >= 3 {
				seed[w] = true
			}
		}
	}
	for _, w := range tokenizeMind(userText) {
		if len([]rune(w)) >= 4 {
			seed[w] = true
		}
	}
	// Extra (episodios) solo si solapa de verdad con este mensaje
	for _, f := range extra {
		if f.Src != "memoria" && f.Src != "usuario" {
			continue
		}
		hit := false
		for w := range seed {
			if strings.Contains(f.Subj, w) || strings.Contains(f.Obj, w) {
				hit = true
				break
			}
		}
		if hit {
			facts = append(facts, f)
		}
	}
	seen := map[string]bool{}
	var uniq []ternaryFact
	for _, f := range facts {
		k := factKey(f)
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, f)
	}
	if len(uniq) < 1 {
		return ""
	}
	tree := buildRFTTree(uniq)
	if len(tree.Nodes) == 0 {
		return ""
	}
	if !isRFTRequest(userText) && len(tree.Saltos) == 0 && len(uniq) < 2 {
		return ""
	}
	lab := wantsLabReasoning(userText)
	return formatRFTVoiceMode(tree, lab)
}
