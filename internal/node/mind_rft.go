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
		// Salto-concreta: de abstracto a instancia corpus (un hecho del corpus que comparta obj)
		for _, f := range linear {
			for _, e := range loadMindKnowledge() {
				blob := strings.ToLower(e.Text)
				if !strings.Contains(blob, f.Obj) && !strings.Contains(blob, f.Subj) {
					continue
				}
				for _, sent := range extractSentences(e.Text) {
					cf, ok := parseRelationFact(sent, 1, "rft:corpus")
					if !ok {
						continue
					}
					if cf.Subj == f.Subj || cf.Obj == f.Obj || cf.Subj == f.Obj {
						add(rftNode{Fact: cf, Level: 2, Kind: "salto-concreta", Parent: factKey(f)})
						break
					}
				}
			}
		}
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

	// Elegir conclusión primaria: preferir nivel 1 trans; si hay salto-neg relevante, mencionarlo
	best := ternaryFact{}
	bestSc := -1
	for _, n := range tree.Nodes {
		if n.Kind == "premisa" {
			continue
		}
		sc := n.Fact.Conf + n.Level
		if n.Kind == "trans" && n.Level == 1 {
			sc += 5
		}
		if n.Kind == "salto-neg" {
			sc += 3
		}
		if sc > bestSc {
			bestSc = sc
			best = n.Fact
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

func formatRFTVoice(tree rftTree) string {
	if len(tree.Nodes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Razonamiento Fractal-Ternario (RFT — no lineal, no predicción):\n")
	// Premisas nivel 0
	for _, n := range tree.Nodes {
		if n.Kind != "premisa" {
			continue
		}
		b.WriteString(fmt.Sprintf("· [L0 premisa conf %d] «%s» %s «%s»\n", n.Fact.Conf, n.Fact.Subj, relVoice(n.Fact.Rel), n.Fact.Obj))
	}
	// Trans lineales
	shown := 0
	for _, n := range tree.Nodes {
		if n.Kind != "trans" || n.Level != 1 {
			continue
		}
		if shown >= 3 {
			break
		}
		b.WriteString(fmt.Sprintf("· [L1 cierre conf %d] «%s» %s «%s»\n", n.Fact.Conf, n.Fact.Subj, relVoice(n.Fact.Rel), n.Fact.Obj))
		shown++
	}
	// Saltos
	if len(tree.Saltos) > 0 {
		b.WriteString("· Sumidero/salto (2 → otro nivel):\n")
		for i, n := range tree.Saltos {
			if i >= 4 {
				break
			}
			b.WriteString(fmt.Sprintf("  — %s [L%d conf %d]: «%s» %s «%s»\n", n.Kind, n.Level, n.Fact.Conf, n.Fact.Subj, relVoice(n.Fact.Rel), n.Fact.Obj))
		}
	}
	// Cierre fractal nivel 3
	shown3 := 0
	for _, n := range tree.Nodes {
		if n.Level != 3 {
			continue
		}
		if shown3 >= 2 {
			break
		}
		b.WriteString(fmt.Sprintf("· [L3 fractal conf %d] «%s» %s «%s»\n", n.Fact.Conf, n.Fact.Subj, relVoice(n.Fact.Rel), n.Fact.Obj))
		shown3++
	}
	if tree.Primary.Subj != "" {
		b.WriteString(fmt.Sprintf("· Conclusión guía: «%s» %s «%s» (conf %d).\n", tree.Primary.Subj, relVoice(tree.Primary.Rel), tree.Primary.Obj, tree.Primary.Conf))
		setLastReasonConclusion(conclusionSentence(tree.Primary))
	}
	b.WriteString("El 2 no es «quizás»: es cambio de nivel (inversión, negación acotada o concreción).")
	return strings.TrimSpace(b.String())
}

// reasonRFT: entrada principal RFT
func reasonRFT(userText string, extra []ternaryFact) string {
	facts := collectReasonFacts(userText)
	facts = append(facts, extra...)
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
	if tree.Voice == "" {
		return ""
	}
	// Si el usuario no pidió RFT explícito pero hay ≥2 premisas y saltos, usar RFT
	if !isRFTRequest(userText) && len(tree.Saltos) == 0 && len(uniq) < 2 {
		return ""
	}
	return tree.Voice
}
