package node

import (
	"fmt"
	"os"
	"strings"
)

// Mind ↔ Gen bridge: Mind orquesta; Gen ejecuta/reside; ethics de Mind manda.
// Comunicación: local consult, remote dialogue, memory pin, dispatch CF.

// mindGenBridgeStatus for /api/mind/self and lab.
func (n *NodoAlset) mindGenBridgeStatus() map[string]interface{} {
	n.ensureGens()
	n.mu.RLock()
	defer n.mu.RUnlock()
	total := len(n.gens)
	memN, remoteN, pins := 0, 0, 0
	keys := make([]string, 0, 8)
	for k, g := range n.gens {
		if len(keys) < 8 {
			keys = append(keys, k)
		}
		if isMemoryMissionGen(g) {
			memN++
			pins += len(g.EpisodeCIDs)
		}
		if g.State.Metadata != nil {
			if rh, ok := g.State.Metadata["remote_http"].(string); ok && rh != "" {
				remoteN++
			}
		}
	}
	cf := strings.TrimSpace(os.Getenv("ALSET_CLOUDFLARE_NETWORK"))
	return map[string]interface{}{
		"gens_total":     total,
		"gens_memory":    memN,
		"gens_remote":    remoteN,
		"memory_pins":    pins,
		"sample_keys":    keys,
		"cloudflare_net": cf != "",
		"cf_url":         cf,
		"auto_pin_mem":   os.Getenv("ALSET_AUTO_PIN_MEM") == "1" || os.Getenv("ALSET_AUTO_PIN_MEM") == "true",
		"note":           "Mind orquesta genes bajo ethics; gen-memoria ancla CIDs; edge opcional",
	}
}

// BridgeDialogueGen: Mind asks a gen and returns voice for the human (ethics-gated).
func (n *NodoAlset) BridgeDialogueGen(key, stimulus string, ethicsState int) string {
	if ethicsState == 2 {
		return "No retransmito al gen: ethics en sumidero. Reformula el pedido."
	}
	key = normalizeGenKey(key)
	if key == "" || key == ".ans" {
		return "Indica el gen: «pregunta al gen demo-cell: quién eres»."
	}
	if stimulus == "" {
		stimulus = "quién eres"
	}
	res := n.DialogueRemoteGen(key, stimulus)
	if v, ok := res["voice"].(string); ok && v != "" {
		where := "local"
		if rh, ok := res["remote_http"].(string); ok && rh != "" {
			where = "borde " + rh
		}
		return fmt.Sprintf("Gen «%s» (%s) responde: %s", key, where, v)
	}
	if err, ok := res["error"].(string); ok && err != "" {
		return fmt.Sprintf("No hubo vínculo con «%s»: %s. Prueba despacharlo o crearlo antes.", key, err)
	}
	return fmt.Sprintf("Gen «%s» sin voz útil aún. Consulta o despacho pendientes.", key)
}

// BridgePinEpisodeToMemGen pins a Mind episode CID onto default or named memory gen.

// BridgeSpeakGenFindings — Mind trae a voz humana el último hallazgo del gen (no solo el conteo).
func (n *NodoAlset) BridgeSpeakGenFindings(key string) string {
	key = normalizeGenKey(key)
	n.ensureGens()
	n.mu.RLock()
	g, ok := n.gens[key]
	n.mu.RUnlock()
	if !ok {
		return "No tengo el gen «" + key + "» en este nodo."
	}
	findN := 0
	last := ""
	if g.State.Metadata != nil {
		if lh, ok := g.State.Metadata["last_hallazgo"].(string); ok {
			last = lh
		}
		switch v := g.State.Metadata["findings"].(type) {
		case []interface{}:
			findN = len(v)
		case []string:
			findN = len(v)
		}
	}
	loc := g.State.Location
	if loc == "" {
		loc = "local"
	}
	if findN == 0 && last == "" {
		return fmt.Sprintf("El gen «%s» (%s) aún no tiene hallazgos. Puedes decir: explora gen %s https://ejemplo.org", key, loc, strings.TrimSuffix(key, ".ans"))
	}
	msg := fmt.Sprintf("Desde el gen «%s» (%s, %d hallazgo(s))", key, loc, findN)
	if last != "" {
		snip := last
		if len([]rune(snip)) > 320 {
			r := []rune(snip)
			snip = string(r[:320]) + "…"
		}
		msg += ":\n" + snip
	} else {
		msg += "."
	}
	return msg
}

func (n *NodoAlset) BridgePinEpisodeToMemGen(genKey, episodeCID, note string) string {
	if episodeCID == "" {
		return "No hay CID de episodio para anclar."
	}
	if genKey == "" {
		genKey = "mem-nodo"
	}
	if _, err := n.CreateMemoryGen(genKey, "salva vinculada a Mind"); err != nil &&
		!strings.Contains(err.Error(), "already") {
		// CreateMemoryGen may return existing via ensure
	}
	g, err := n.PinCIDToMemoryGen(genKey, episodeCID, note)
	if err != nil {
		return "No pude vincular el episodio al gen: " + err.Error()
	}
	return fmt.Sprintf("Episodio %s anclado en gen memoria «%s» (%d anclas).", truncateCID(episodeCID), g.Key, len(g.EpisodeCIDs))
}

// maybeAutoPinMemGen after MindTick saves episode — optional env ALSET_AUTO_PIN_MEM=1.
func (n *NodoAlset) maybeAutoPinMemGen(episodeCID string) {
	if episodeCID == "" {
		return
	}
	v := strings.ToLower(os.Getenv("ALSET_AUTO_PIN_MEM"))
	if v != "1" && v != "true" && v != "yes" {
		return
	}
	_, _ = n.CreateMemoryGen("mem-nodo", "auto-pin desde Mind")
	_, _ = n.PinCIDToMemoryGen("mem-nodo", episodeCID, "auto_mind_episode")
}

// extractGenDialogueStimulus parses "pregunta al gen X: mensaje" / "dile al gen X que ..."
func extractGenDialogueStimulus(text string) (key, stim string) {
	s := strings.ToLower(strings.TrimSpace(text))
	key = extractGenNameFromText(s)
	stim = "quién eres"
	if i := strings.Index(text, ":"); i >= 0 && i+1 < len(text) {
		stim = strings.TrimSpace(text[i+1:])
		return key, stim
	}
	for _, pfx := range []string{"dile al gen", "di al gen", "pregunta al gen", "habla con gen", "dialoga con gen"} {
		if j := strings.Index(s, pfx); j >= 0 {
			rest := strings.TrimSpace(text[j+len(pfx):])
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				return key, stim
			}
			// first field may be key
			if key == "" {
				key = strings.Trim(fields[0], ".,;:")
			}
			if len(fields) > 1 {
				// drop "que" if present
				start := 1
				if strings.EqualFold(fields[1], "que") || strings.EqualFold(fields[1], "qué") {
					start = 2
				}
				if start < len(fields) {
					stim = strings.Join(fields[start:], " ")
				}
			}
			return key, stim
		}
	}
	return key, stim
}
