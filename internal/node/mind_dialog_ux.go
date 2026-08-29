package node

import (
	"strings"
	"sync"
	"time"
)

// Intent classification for natural user dialogue (not developer manuals).

type dialogIntent string

const (
	intentIdentity   dialogIntent = "identity"
	intentMemory     dialogIntent = "memory"
	intentPerson     dialogIntent = "person_scout"
	intentCreative   dialogIntent = "creative"
	intentEmotion    dialogIntent = "emotion"
	intentAdvice     dialogIntent = "advice"
	intentChat       dialogIntent = "chat"
	intentEthicsHard dialogIntent = "ethics_hard"
	intentMath       dialogIntent = "math"
	intentClarify    dialogIntent = "clarify"
	intentThanks     dialogIntent = "thanks"
	intentBye        dialogIntent = "bye"
	intentTopicShift dialogIntent = "topic_shift"
	intentUnknown    dialogIntent = "unknown"
)

func classifyDialogIntent(s string) dialogIntent {
	low := strings.ToLower(strings.TrimSpace(s))
	if low == "" {
		return intentUnknown
	}
	if isDestructiveOrder(low) || isPrivacyInvasion(low) {
		return intentEthicsHard
	}
	if isMemoryQuery(low) || isSelfModelQuery(low) {
		return intentMemory
	}
	if isCreativeWriteRequest(low) {
		return intentCreative
	}
	if isPersonLookup(low) {
		return intentPerson
	}
	if isIdentityTalk(low) {
		return intentIdentity
	}
	if isEmotionTalk(low) {
		return intentEmotion
	}
	if isAdviceTalk(low) {
		return intentAdvice
	}
	if isClarifyTalk(low) {
		return intentClarify
	}
	if isThanksTalk(low) {
		return intentThanks
	}
	if isByeTalk(low) {
		return intentBye
	}
	if isTopicShift(low) {
		return intentTopicShift
	}
	return intentChat
}

func isPersonLookup(s string) bool {
	s = strings.ToLower(s)
	if strings.Contains(s, "quién eres") || strings.Contains(s, "quien eres") {
		return false
	}
	if strings.Contains(s, "quién te ") || strings.Contains(s, "quien te ") {
		return false
	}
	keys := []string{
		"quién es ", "quien es ", "quién fue ", "quien fue ",
		"quién era ", "quien era ", "biografía de ", "biografia de ",
		"cuéntame de ", "cuentame de ", "habla de ", "sobre mandela", "sobre frida",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func isEmotionTalk(s string) bool {
	keys := []string{
		"estoy cansado", "estoy cansada", "estoy triste", "estoy feliz", "estoy contento",
		"estoy contenta", "me siento", "estoy aburrido", "estoy aburrida", "estoy estresado",
		"estoy estresada", "tengo sueño", "estoy mal", "estoy bien hoy",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func isAdviceTalk(s string) bool {
	keys := []string{
		"qué me recomiendas", "que me recomiendas", "recomend", "consejo",
		"qué puedo cocinar", "que puedo cocinar", "cocinar", "para estudiar",
		"cómo estudio", "como estudio", "elegir entre", "no sé qué hacer", "no se que hacer",
		"despejarme", "algo interesante", "algo más ligero", "algo mas ligero",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func isClarifyTalk(s string) bool {
	keys := []string{
		"no entendí", "no entendi", "más simple", "mas simple", "explícame más simple",
		"explicame mas simple", "no entendí nada", "no entendi nada", "repite más claro",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func isThanksTalk(s string) bool {
	return strings.Contains(s, "gracias") || strings.Contains(s, "te lo agradezco") ||
		s == "ok gracias" || s == "ok, gracias"
}

func isByeTalk(s string) bool {
	keys := []string{"adiós", "adios", "hasta luego", "nos vemos", "chao", "bye"}
	for _, k := range keys {
		if s == k || strings.HasPrefix(s, k) {
			return true
		}
	}
	return false
}

func isTopicShift(s string) bool {
	return strings.Contains(s, "ahora hablemos") || strings.Contains(s, "cambiemos de tema") ||
		strings.Contains(s, "otra cosa") || strings.HasPrefix(s, "ahora ")
}

func isPrivacyInvasion(s string) bool {
	s = strings.ToLower(s)
	if strings.Contains(s, "contraseña") || strings.Contains(s, "contrasena") || strings.Contains(s, "password") {
		return true
	}
	if strings.Contains(s, "wifi") && (strings.Contains(s, "vecino") || strings.Contains(s, "ajeno")) {
		return true
	}
	if strings.Contains(s, "whatsapp") && (strings.Contains(s, "entrar") || strings.Contains(s, "acceder") || strings.Contains(s, "hack")) {
		return true
	}
	if strings.Contains(s, "cuenta ajena") || strings.Contains(s, "hackear") {
		return true
	}
	return false
}

func speakEmotion(s string) string {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "cansad"):
		return "Tiene sentido tomártelo con calma. Un descanso corto, agua o un cambio de actividad suelen ayudar más que forzar. Si quieres desahogarte o pedir una idea ligera, aquí estoy."
	case strings.Contains(low, "trist") || strings.Contains(low, "mal"):
		return "Siento que estés así. Puedo escucharte o ayudarte a ordenar ideas; si el malestar es fuerte, conviene también apoyo de personas de confianza."
	case strings.Contains(low, "feliz") || strings.Contains(low, "content"):
		return "Qué bien. Si quieres contarme por qué, lo escucho; si prefieres otro tema, también."
	case strings.Contains(low, "aburr"):
		return "Podemos explorar un tema curioso, un poema corto o un juego de preguntas. ¿Qué te apetece?"
	case strings.Contains(low, "estres"):
		return "El estrés pide pasos pequeños: una tarea mínima y un respiro. Si nombras qué te aprieta, lo miramos juntos."
	default:
		return "Te escucho. Cuéntame un poco más si quieres, o cambia de tema cuando lo necesites."
	}
}

func speakAdvice(s string) string {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "cocinar") || strings.Contains(low, "arroz"):
		return "Con arroz, algo rápido: sofríe ajo y cebolla, añade el arroz cocido, un huevo revuelto o atún, y sal al gusto. Si tienes verdura congelada, súmala al final. No hace falta complicarse."
	case strings.Contains(low, "estudi"):
		return "Prueba bloques cortos (25 minutos) y al terminar explícalo en voz alta sin mirar apuntes. Eso fija más que releer. Si tienes un tema concreto, lo desglosamos."
	case strings.Contains(low, "elegir") || strings.Contains(low, "trabajos") || strings.Contains(low, "paga más"):
		return "Anóta en una columna dinero y estabilidad; en otra, gusto y energía a 6 meses. Si el que te gusta cubre lo básico, suele pesar más a largo plazo. Si no, a veces un puente temporal en el que paga más tiene sentido. La elección es tuya; yo solo ayudo a ordenar criterios."
	case strings.Contains(low, "despejar") || strings.Contains(low, "recomiend"):
		return "Algo breve: caminar unos minutos, música que te guste, o un mensaje a alguien de confianza. Si prefieres quedarte en el chat, pide un tema ligero o un poema corto."
	case strings.Contains(low, "interesante"):
		return "Dato curioso sin jerga: las plantas «comen» luz; convierten sol, agua y aire en azúcar y sueltan oxígeno. Si quieres otro dato (personas, historia, ciencia), dilo."
	case strings.Contains(low, "ligero"):
		return "Vale, tono ligero: ¿prefieres un mini-cuento absurdo, una pregunta divertida o un dato raro de la naturaleza?"
	default:
		return "Cuéntame un poco más del contexto y te doy una sugerencia concreta, sin rodeos."
	}
}

func speakClarify() string {
	return "De acuerdo: veamos más despacio. Dime qué parte no quedó clara —una frase o una palabra— y la rearmo en pocas líneas."
}

func speakThanks() string {
	return "De nada. Cuando quieras, seguimos con otro tema."
}

func speakBye() string {
	return "Hasta luego. Que te vaya bien."
}

func speakTopicShift() string {
	return "De acuerdo, cambiamos de tema. ¿De qué quieres hablar?"
}

func speakEthicsHard(s string) string {
	low := strings.ToLower(s)
	if strings.Contains(low, "whatsapp") {
		return "No puedo entrar a tu WhatsApp ni a cuentas ajenas. Eso queda fuera de lo que hago."
	}
	if strings.Contains(low, "wifi") || strings.Contains(low, "vecino") || strings.Contains(low, "contraseña") || strings.Contains(low, "password") {
		return "No ayudo a obtener contraseñas ni accesos ajenos. Si es tu red, revísala en el router o con tu proveedor."
	}
	if strings.Contains(low, "borra") || strings.Contains(low, "elimina") {
		return "No ejecuto borrados masivos ni limpiezas peligrosas de datos. Puedo hablar del tema en general, nada más."
	}
	return "Eso no lo hago: toca privacidad o riesgo. Puedo ayudarte con otras cosas dentro de lo legítimo."
}

// --- Interaction patterns (lightweight learning from dialogue) ---

type dialogPattern struct {
	Intent string `json:"intent"`
	Hit    int    `json:"hit"`
	Last   string `json:"last"`
}

var (
	dialogPatternMu sync.Mutex
	dialogPatterns  = map[string]*dialogPattern{}
)

func recordDialogPattern(intent dialogIntent, userText string) {
	dialogPatternMu.Lock()
	defer dialogPatternMu.Unlock()
	k := string(intent)
	p := dialogPatterns[k]
	if p == nil {
		p = &dialogPattern{Intent: k}
		dialogPatterns[k] = p
	}
	p.Hit++
	p.Last = time.Now().UTC().Format(time.RFC3339)
	// keep a short sample of successful user phrasing for future tuning
	if len(userText) > 8 && len(userText) < 120 {
		_ = userText
	}
}

func softAppendAllowedUX(kind string) bool {
	switch kind {
	case "chat", "emotion", "advice", "clarify", "thanks", "bye", "creative":
		return false // no muletillas en diálogo humano
	default:
		return true
	}
}
