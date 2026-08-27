package node

import (
	"strings"
)

// RFT en diálogo: no solo cuando el humano dice «entonces».
// Extrae hechos de corpus/memoria/genes relacionados al tema y, si hay
// cierre L1, lo incorpora de forma breve a la voz (sin volcar el árbol entero).

func dedupeEpisodesByText(episodes []mindEpisodePayload) []mindEpisodePayload {
	seen := map[string]bool{}
	var out []mindEpisodePayload
	for _, ep := range episodes {
		k := strings.ToLower(strings.TrimSpace(ep.Text))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, ep)
	}
	return out
}

func factsFromTextBlob(blob, src string, conf int) []ternaryFact {
	var out []ternaryFact
	seen := map[string]bool{}
	for _, sent := range extractSentences(blob) {
		if len([]rune(sent)) > 120 {
			continue
		}
		f, ok := parseRelationFact(sent, conf, src)
		if !ok {
			continue
		}
		k := factKey(f)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	return out
}

func factsRelatedToQuery(userText string, maxCorpus int) []ternaryFact {
	seed := map[string]bool{}
	for _, w := range tokenizeMind(userText) {
		if len([]rune(w)) >= 4 {
			seed[w] = true
		}
	}
	for _, f := range collectReasonFactsOpts(userText, false) {
		for _, w := range strings.Fields(f.Subj + " " + f.Obj) {
			if len([]rune(w)) >= 3 {
				seed[w] = true
			}
		}
	}
	if len(seed) == 0 {
		return nil
	}
	var out []ternaryFact
	n := 0
	for _, e := range loadMindKnowledge() {
		if maxCorpus > 0 && n >= maxCorpus {
			break
		}
		blob := strings.ToLower(strings.Join(e.Keys, " ") + " " + e.Text)
		hit := false
		for w := range seed {
			if strings.Contains(blob, w) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		for _, f := range factsFromTextBlob(e.Text, "corpus", 1) {
			out = append(out, f)
			n++
			if maxCorpus > 0 && n >= maxCorpus {
				break
			}
		}
	}
	return out
}

func (n *NodoAlset) factsFromGens(userText string, maxN int) []ternaryFact {
	if n == nil || maxN <= 0 {
		return nil
	}
	seed := map[string]bool{}
	for _, w := range tokenizeMind(userText) {
		if len([]rune(w)) >= 4 {
			seed[w] = true
		}
	}
	var out []ternaryFact
	n.mu.Lock()
	defer n.mu.Unlock()
	count := 0
	for _, g := range n.gens {
		if g == nil || g.State.Metadata == nil {
			continue
		}
		lh, _ := g.State.Metadata["last_hallazgo"].(string)
		if lh == "" {
			continue
		}
		low := strings.ToLower(lh)
		hit := len(seed) == 0
		for w := range seed {
			if strings.Contains(low, w) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		for _, f := range factsFromTextBlob(lh, "gen:"+g.Key, 1) {
			out = append(out, f)
			count++
			if count >= maxN {
				return out
			}
		}
	}
	return out
}

func factsFromEpisodesRelated(userText string, episodes []mindEpisodePayload, maxN int) []ternaryFact {
	seed := map[string]bool{}
	for _, w := range tokenizeMind(userText) {
		if len([]rune(w)) >= 4 {
			seed[w] = true
		}
	}
	var out []ternaryFact
	n := 0
	for _, ep := range dedupeEpisodesByText(episodes) {
		if n >= maxN {
			break
		}
		low := strings.ToLower(ep.Text)
		if strings.Contains(low, "hallazgo sonda") || strings.Contains(low, "deducción ternaria") ||
			strings.Contains(low, "razonamiento fractal") {
			// aún pueden aportar hechos si solapan
		}
		hit := false
		for w := range seed {
			if strings.Contains(low, w) {
				hit = true
				break
			}
		}
		if !hit && len(seed) > 0 {
			continue
		}
		for _, f := range factsFromTextBlob(ep.Text, "memoria", 1) {
			out = append(out, f)
			n++
			if n >= maxN {
				break
			}
		}
	}
	return out
}

// briefRFTLine: una sola línea de cierre si hay L1 anclado al query.
func briefRFTLine(userText string, extra []ternaryFact) string {
	if isCalmChat(userText) || isIdentityTalk(strings.ToLower(userText)) ||
		isCreativeWriteRequest(userText) || isDestructiveOrder(userText) {
		return ""
	}
	facts := collectReasonFactsOpts(userText, false)
	facts = append(facts, extra...)
	// corpus + related already in extra from caller
	userOnly := []ternaryFact{}
	seed := map[string]bool{}
	for _, f := range facts {
		if f.Src == "usuario" {
			userOnly = append(userOnly, f)
		}
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
	derived := deduceAll(facts)
	if len(derived) == 0 {
		derived = deduceAll(userOnly)
	}
	if len(derived) == 0 {
		return ""
	}
	best := ternaryFact{}
	bestSc := -1
	for _, f := range derived {
		if f.Subj == f.Obj || !validReasonTerm(f.Subj) || !validReasonTerm(f.Obj) {
			continue
		}
		sc := f.Conf
		for w := range seed {
			if strings.Contains(f.Subj, w) || strings.Contains(f.Obj, w) {
				sc += 4
			}
		}
		if sc > bestSc {
			bestSc = sc
			best = f
		}
	}
	if bestSc < 5 || best.Subj == "" {
		return ""
	}
	setLastReasonConclusion(conclusionSentence(best))
	return "· Lectura RFT: «" + best.Subj + "» " + relVoice(best.Rel) + " «" + best.Obj + "» (cierre ternario)."
}

// enrichVoiceWithRFT añade lectura breve si no es ya un bloque RFT completo.
func enrichVoiceWithRFT(userText, voice string, extra []ternaryFact) string {
	if voice == "" {
		return voice
	}
	low := strings.ToLower(voice)
	if strings.Contains(low, "razonamiento fractal-ternario") || strings.Contains(low, "deducción ternaria") {
		return voice
	}
	line := briefRFTLine(userText, extra)
	if line == "" {
		return voice
	}
	if strings.Contains(strings.ToLower(voice), strings.ToLower(conclusionSentence(ternaryFact{}))) {
		return voice
	}
	// evitar duplicar si la voz ya afirma lo mismo
	if strings.Contains(low, strings.ToLower(line[strings.Index(line, "«"):])) {
		return voice
	}
	return strings.TrimSpace(voice) + "\n\n" + line
}
