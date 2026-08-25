package node

import (
	"regexp"
	"strings"
)

// RouteSource is where the director should prefer to answer from.
type RouteSource string

const (
	RouteVeto     RouteSource = "veto"
	RouteAction   RouteSource = "action_memory"
	RouteThread   RouteSource = "thread"
	RouteMath     RouteSource = "math"
	RouteGenTool  RouteSource = "gen_tool"
	RouteCodegen  RouteSource = "codegen"
	RouteMemory   RouteSource = "memory"
	RouteCorpus   RouteSource = "corpus"
	RouteWeb      RouteSource = "web"
	RouteChat     RouteSource = "chat"
)

// RouteDecision documents the ordered rule that won (for tests + learning).
type RouteDecision struct {
	Source RouteSource `json:"source"`
	Rule   string      `json:"rule"`
	Topic  string      `json:"topic,omitempty"`
}

// classifyMindRoute — reglas explícitas (corpus → memoria → web → hechos no son un revoltijo).
// Orden de evaluación (primero que gana):
//  1. veto ethics
//  2. action_memory ("qué hice / por qué")
//  3. thread / follow-up pronominal
//  4. math
//  5. gen tools / codegen
//  6. memoria personal / episodios
//  7. corpus (knowledge)
//  8. web scout (quién es / qué es factual)
//  9. chat
func classifyMindRoute(text string) RouteDecision {
	norm := normalizeUserInput(text)
	low := strings.ToLower(strings.TrimSpace(norm))
	if low == "" {
		return RouteDecision{Source: RouteChat, Rule: "empty"}
	}

	if isActionMemoryQuery(low) {
		return RouteDecision{Source: RouteAction, Rule: "action_memory_query"}
	}
	if isReferentialFollowUp(norm) || isPronounFollowUp(low) {
		return RouteDecision{Source: RouteThread, Rule: "thread_or_pronoun_followup"}
	}
	if isCapabilityQuestion(norm) {
		return RouteDecision{Source: RouteChat, Rule: "capability"}
	}
	// math-like
	if mathExprRe.MatchString(low) || mathSumRe.MatchString(low) {
		return RouteDecision{Source: RouteMath, Rule: "arithmetic"}
	}
	if isGenToolIntent(norm) {
		return RouteDecision{Source: RouteGenTool, Rule: "gen_tool"}
	}
	if isCodeGenStrict(norm) || isCodeGenRequest(low) {
		return RouteDecision{Source: RouteCodegen, Rule: "codegen"}
	}
	if isMemoryQuery(low) || isPersonalFact(low) {
		return RouteDecision{Source: RouteMemory, Rule: "memory_or_personal"}
	}
	// Factual who/what → web if not pure identity
	if forceWebScout(norm) || isScoutableQuestion(norm) {
		topic := extractTopic(norm)
		return RouteDecision{Source: RouteWeb, Rule: "factual_scout", Topic: topic}
	}
	if isEpistemicCheck(text) || isConfirmationPrompt(text) || isElaborationRequest(text) || isContinuePrompt(text) {
		return RouteDecision{Source: RouteThread, Rule: "continuity"}
	}
	return RouteDecision{Source: RouteChat, Rule: "default_chat"}
}

func isActionMemoryQuery(low string) bool {
	qf := foldSpanish(low)
	keys := []string{
		"que hice", "que hiciste", "que ejecute", "que ejecutaste",
		"acciones recientes", "ultima accion", "historial de accion",
		"porque lo hiciste", "porque hiciste", "por que lo hiciste", "por que hiciste",
		"como decides", "que patron", "explica tu accion", "explica la accion",
	}
	for _, k := range keys {
		if strings.Contains(qf, k) {
			return true
		}
	}
	return false
}

// --- Anomaly detectors (regex / heuristics) for dialogue quality ---

var (
	reHTMLCSSJunk = regexp.MustCompile(`(?i)(prefers-color-scheme|function\s*\(\)|var\s+dc_enabled|header-wrap|@media\s*\()`)
	reScoutRaw    = regexp.MustCompile(`(?i)hallazgo\s+sonda\s+scout-`)
	reDisambig    = regexp.MustCompile(`(?i)(desambiguaci[oó]n|may refer to|varias entradas|puede referirse a)`)
	reEmptyBody   = regexp.MustCompile(`(?i)tengo este resumen[^\n]*:\s*\n\s*\n`)
	reLabJargon   = regexp.MustCompile(`(?i)(despach[eé]\s+la\s+sonda|sonda temporal eliminada|sin nueva sonda)`)
)

// AnomalyFinding is one rule hit on a voice string.
type AnomalyFinding struct {
	Code    string `json:"code"`
	Detail  string `json:"detail"`
	Snippet string `json:"snippet,omitempty"`
}

// DetectVoiceAnomalies scans a Mind voice for structural defects.
func DetectVoiceAnomalies(voice string) []AnomalyFinding {
	v := strings.TrimSpace(voice)
	if v == "" {
		return []AnomalyFinding{{Code: "empty_voice", Detail: "respuesta vacía"}}
	}
	var out []AnomalyFinding
	if reHTMLCSSJunk.MatchString(v) {
		out = append(out, AnomalyFinding{Code: "html_css_junk", Detail: "basura HTML/CSS de scraper", Snippet: truncateRunes(v, 80)})
	}
	if reScoutRaw.MatchString(v) {
		out = append(out, AnomalyFinding{Code: "raw_scout_echo", Detail: "eco crudo de hallazgo sonda en voz", Snippet: truncateRunes(v, 80)})
	}
	if reDisambig.MatchString(v) {
		out = append(out, AnomalyFinding{Code: "disambiguation", Detail: "página de desambiguación como respuesta", Snippet: truncateRunes(v, 80)})
	}
	if reEmptyBody.MatchString(v) {
		out = append(out, AnomalyFinding{Code: "empty_summary_body", Detail: "promete resumen pero el cuerpo está vacío"})
	}
	if reLabJargon.MatchString(v) {
		out = append(out, AnomalyFinding{Code: "lab_jargon", Detail: "jerga de laboratorio en voz de usuario"})
	}
	// English-only long factual answer without Spanish note
	lowV := strings.ToLower(v)
	if !strings.Contains(lowV, "inglés") && !strings.Contains(lowV, "ingles") {
		enHits := 0
		for _, w := range []string{" was ", " were ", " the ", " and ", " with ", " from ", " his ", " her "} {
			if strings.Contains(" "+lowV+" ", w) {
				enHits++
			}
		}
		if enHits >= 2 && len([]rune(v)) > 50 {
			out = append(out, AnomalyFinding{Code: "english_only", Detail: "respuesta factual predominantemente en inglés"})
		}
	}
	// Soft-memory dead-end that hijacks a factual follow-up
	if strings.Contains(strings.ToLower(v), "me suena esto") && (strings.Contains(strings.ToLower(v), "scout-") || strings.Contains(strings.ToLower(v), "hallazgo")) {
		out = append(out, AnomalyFinding{Code: "soft_memory_hijack", Detail: "soft memory con basura de sonda"})
	}
	return out
}

// ExpectRouteAnomaly if the actual primary path disagrees with classifyMindRoute.
func ExpectRouteAnomaly(text, primaryKind string) *AnomalyFinding {
	d := classifyMindRoute(text)
	// map primaryKind from tick to RouteSource
	got := mapPrimaryKind(primaryKind)
	if d.Source == RouteWeb && got != RouteWeb && got != RouteCorpus && got != RouteMemory {
		// web expected but got pure chat without content is anomaly
		if got == RouteChat {
			return &AnomalyFinding{Code: "route_miss_web", Detail: "se esperaba web/corpus y cayó a chat: " + d.Rule}
		}
	}
	if d.Source == RouteAction && got != RouteAction {
		return &AnomalyFinding{Code: "route_miss_action", Detail: "se esperaba action_memory"}
	}
	return nil
}

func mapPrimaryKind(k string) RouteSource {
	switch k {
	case "tool":
		return RouteWeb // scout currently marks tool
	case "math":
		return RouteMath
	case "codegen":
		return RouteCodegen
	case "memory", "action_memory":
		if k == "action_memory" {
			return RouteAction
		}
		return RouteMemory
	case "knowledge":
		return RouteCorpus
	case "referential":
		return RouteThread
	case "veto":
		return RouteVeto
	case "chat", "capability", "identity":
		return RouteChat
	default:
		return RouteChat
	}
}
