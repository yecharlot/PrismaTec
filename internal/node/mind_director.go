package node

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Mind dialogue director: single path for fluent, intentional voice.
// Priority: ethics → gen/tools → math → codegen → memory → knowledge → open dialogue.
// Soft curiosity/humor only when the primary answer is social/open and thin.

// normalizeUserInput: orthography / typos before routing (not an LLM spellcheck).
func normalizeUserInput(q string) string {
	q = strings.TrimSpace(q)
	q = normalizeKnowledgeQuery(q) // shared typo table
	reps := []struct{ a, b string }{
		{"quie es", "quién es"},
		{"quien es", "quién es"},
		{"qien", "quien"},
		{"mision", "misión"},
		{"qual es", "cuál es"},
		{"cual es", "cuál es"},
		{"ke ", "que "},
		{"xq", "por qué"},
		{"porq ", "porque "},
		{"tbn", "también"},
		{"tb ", "también "},
		{"xa ", "para "},
		{"dnd ", "dónde "},
		{"dnd?", "dónde?"},
		{"kiero", "quiero"},
		{"qiero", "quiero"},
		{"acer ", "hacer "},
		{"asi ", "así "},
		{"tambien", "también"},
		{"mas ", "más "},
		{"comose", "cómo se"},
		{"comos ", "cómo es "},
		{"cuantos genes", "cuántos genes"},
		{"cuanto es", "cuánto es"},
		{"cuanto son", "cuánto son"},
	}
	low := strings.ToLower(q)
	for _, r := range reps {
		if strings.Contains(low, r.a) {
			// case-insensitive replace on original via lower map is hard; replace on lower rebuild
			low = strings.ReplaceAll(low, r.a, r.b)
		}
	}
	return low
}

func isGenToolIntent(s string) bool {
	s = strings.ToLower(s)
	// no confundir codegen "crea una función"
	if strings.Contains(s, "función") || strings.Contains(s, "funcion") || strings.Contains(s, "código") || strings.Contains(s, "codigo") {
		if !strings.Contains(s, "gen") && !strings.Contains(s, "sonda") {
			return false
		}
	}
	keys := []string{
		"crea gen", "crear gen", "lista gen", "listar gen", "lista genes", "listar genes",
		"despacha", "envía gen", "envia gen", "manda gen", "manda una sonda", "envía una sonda", "envia una sonda",
		"pregunta al gen", "habla con gen", "dile al gen", "di al gen", "dialoga",
		"explora", "explorar", "sirve gen", "gen memoria", "salva en gen", "guarda en gen",
		"vincula memoria", "genes memoria", "qué sabe el gen", "que sabe el gen",
		"elimina gen", "retorna gen", "trae de vuelta", "trae la sonda", "borra gen",
		"qué genes", "que genes", "muestra los genes", "sondas activas", "mis genes",
		"crea una sonda", "crear sonda", "haz un gen", "haz una sonda", "nueva sonda",
		"al borde", "a cloudflare", "red de borde",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	if (strings.Contains(s, "gen ") || strings.Contains(s, "sonda")) &&
		(strings.Contains(s, "explora") || strings.Contains(s, "despacha") || strings.Contains(s, "cloudflare") ||
			strings.Contains(s, "busca") || strings.Contains(s, "investiga") || strings.Contains(s, "elimina") ||
			strings.Contains(s, "borra") || strings.Contains(s, "retorna") || strings.Contains(s, "trae")) {
		return true
	}
	return false
}

func isCodeGenStrict(s string) bool {
	return isCodeGenRequest(s)
}

func isCapabilityQuestion(s string) bool {
	s = strings.ToLower(s)
	// "cuántos genes puedes crear" is capacity, NOT create
	if strings.Contains(s, "puedes crear") || strings.Contains(s, "puedo crear") {
		return true
	}
	if strings.Contains(s, "cuántos genes") || strings.Contains(s, "cuantos genes") {
		return true
	}
	if strings.Contains(s, "puedes programar") || strings.Contains(s, "sabes programar") {
		return true
	}
	if strings.Contains(s, "qué puedes hacer") || strings.Contains(s, "que puedes hacer") {
		return true
	}
	return false
}

func isReferentialFollowUp(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	keys := []string{
		"qué significa esto", "que significa esto", "qué es eso", "que es eso",
		"y eso", "eso qué", "eso que", "a qué te refieres", "a que te refieres",
		"qué fue eso", "que fue eso", "explica eso", "eso significa",
	}
	for _, k := range keys {
		if s == k || strings.HasPrefix(s, k) {
			return true
		}
	}
	if (s == "eso" || s == "esto" || s == "y eso?") && len(s) < 12 {
		return true
	}
	return false
}

var mathExprRe = regexp.MustCompile(`(?i)(?:cu[aá]nto\s+(?:es|son)\s+)?(-?\d+(?:\.\d+)?)\s*([+\-*/x×]|\s+por\s+)\s*(-?\d+(?:\.\d+)?)`)
var mathSumRe = regexp.MustCompile(`(?i)suma\s+(-?\d+(?:\.\d+)?)\s+y\s+(-?\d+(?:\.\d+)?)`)

// tryMindMath: arithmetic via LispAI when possible; pure Go fallback.
func (n *NodoAlset) tryMindMath(text string) string {
	s := normalizeUserInput(text)
	var a, b float64
	var op string
	if m := mathSumRe.FindStringSubmatch(s); len(m) == 3 {
		a, _ = strconv.ParseFloat(m[1], 64)
		b, _ = strconv.ParseFloat(m[2], 64)
		op = "+"
	} else if m := mathExprRe.FindStringSubmatch(s); len(m) == 4 {
		a, _ = strconv.ParseFloat(m[1], 64)
		op = m[2]
		b, _ = strconv.ParseFloat(m[3], 64)
		op = strings.TrimSpace(op)
		if op == "x" || op == "×" || strings.EqualFold(op, "por") {
			op = "*"
		}
	} else {
		return ""
	}
	lispOp := op
	if lispOp == "+" || lispOp == "-" || lispOp == "*" || lispOp == "/" {
		if n != nil && n.lisp != nil {
			code := fmt.Sprintf("(%s %v %v)", lispOp, a, b)
			if res, err := n.lisp.Eval(code); err == nil {
				return fmt.Sprintf("Resultado: %v (cálculo vía LispAI: %s).", res, code)
			}
		}
		// Go fallback
		var r float64
		switch op {
		case "+":
			r = a + b
		case "-":
			r = a - b
		case "*":
			r = a * b
		case "/":
			if b == 0 {
				return "No puedo dividir entre cero."
			}
			r = a / b
		}
		return fmt.Sprintf("Resultado: %v.", r)
	}
	return ""
}

func capabilityVoice(s string) string {
	s = strings.ToLower(s)
	if strings.Contains(s, "gen") {
		return "Puedo crear muchos genes en este nodo (el límite práctico es memoria y claridad de nombres). Cada uno tiene clave ANS y RootCID. Di «crea gen nombre» o «crea gen memoria mem-nodo» para una salva de CIDs. No hay un cupo mágico de marketing: hay células reales en el registro."
	}
	if strings.Contains(s, "program") {
		return "Puedo ensamblar código desde plantillas curadas (Go, Lisp, Python, JS) bajo ethics — no invento sistemas enteros como un LLM. Prueba: «genera código función sumar en go» o «escribe código factorial en lisp»."
	}
	return "Puedo dialogar, conversar, recordar lo que me digas, explorar temas públicos, calcular y proponer esquemas de código sencillos. No invento datos ni ejecuto pedidos peligrosos."
}

// resolveReferential uses thread state for "qué significa esto".
func (n *NodoAlset) resolveReferential(text string) string {
	if n == nil {
		return ""
	}
	n.mindLastMu.Lock()
	topic := n.mindLastTopic
	expl := n.mindLastExplore
	gen := n.mindLastGen
	code := n.mindLastCode
	voice := n.mindLastVoice
	n.mindLastMu.Unlock()

	switch topic {
	case "explore":
		if expl != "" {
			return "Me refería al último explore: " + compressVoiceBlock(expl, 400) + "\nEso es un fragmento de la página observada por el gen; no es una definición filosófica."
		}
	case "gen":
		if gen != "" {
			return fmt.Sprintf("Hablábamos del gen «%s». Puedes preguntar su misión, despacharlo o pedirle otro estímulo.", gen)
		}
	case "code":
		if code != "" {
			return "Me refería al último esqueleto de código que armé. Si quieres otro lenguaje o plantilla, dilo con claridad."
		}
	case "math":
		return "Me refería al cálculo anterior. Puedes pedir otra operación (suma, resta, producto, división)."
	}
	if voice != "" && (strings.Contains(voice, "explore") || strings.Contains(voice, "http") || strings.Contains(voice, "Mandé")) {
		return "El turno anterior trataba de una acción en el nodo o un gen. Reformula con el nombre del gen o la URL si quieres precisión."
	}
	return ""
}

// softAppendAllowed: only on thin open chat — never on tools/math/codegen/solid knowledge.
func softAppendAllowed(primaryKind string, voice string) bool {
	// Diálogo fluido: sin coletas de curiosity/humor salvo chat muy corto y abierto.
	switch primaryKind {
	case "tool", "math", "codegen", "memory", "capability", "referential", "identity", "veto", "seed",
		"creative", "action_memory", "patterns", "knowledge", "reason":
		return false
	case "chat":
		return len([]rune(voice)) < 60
	default:
		return false
	}
}

// rememberThreadRefs updates gen/explore/code topic after tools.
func (n *NodoAlset) rememberThreadRefs(topic, gen, explore, code string) {
	if n == nil {
		return
	}
	n.mindLastMu.Lock()
	defer n.mindLastMu.Unlock()
	if topic != "" {
		n.mindLastTopic = topic
	}
	if gen != "" {
		n.mindLastGen = normalizeGenKey(gen)
	}
	if explore != "" {
		n.mindLastExplore = explore
	}
	if code != "" {
		n.mindLastCode = code
	}
}
