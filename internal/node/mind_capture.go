package node

import (
	"strings"
	"unicode"
)

// Generalized dialogue intent — structural, not infinite phrase lists.
// Escapes from corpus must raise novelty and land in episodic memory.

func hasAnyRoot(s string, roots []string) bool {
	for _, r := range roots {
		if strings.Contains(s, r) {
			return true
		}
	}
	return false
}

func wordCount(s string) int {
	return len(strings.Fields(s))
}

// isElaborationRequest: user wants to deepen the *current* thread.
// Works for "amplía el punto", "no entiendo amplia…", "más detalle", etc.
func isElaborationRequest(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || wordCount(s) > 12 {
		return false
	}
	// New lookup questions are not elaborations of the prior thread
	if isNewTopicLookup(s) {
		return false
	}
	expandRoots := []string{
		"ampli", "profund", "desarroll", "expand", "detalle", "ángulo", "angulo",
		"punto", "hilo", "sigue", "continu", "retom", "aclara", "mejor",
		"ese tema", "este tema", "puedes seguir", "sigamos",
	}
	confusionRoots := []string{"no entiendo", "no comprendo", "no me queda", "confus", "no capto"}
	hasExpand := hasAnyRoot(s, expandRoots)
	hasConf := hasAnyRoot(s, confusionRoots)
	if hasConf && hasExpand {
		return true
	}
	if hasExpand && wordCount(s) <= 8 {
		return true
	}
	if hasConf && wordCount(s) <= 6 {
		return true
	}
	return false
}

// isNewTopicLookup: explicit request for a definition/topic (should hit corpus, not continue).
func isNewTopicLookup(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	prefixes := []string{"qué es ", "que es ", "qué son ", "que son ", "quién es ", "quien es ",
		"cómo funciona ", "como funciona ", "explica ", "define "}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			rest := strings.TrimSpace(s[len(p):])
			// strip trailing elaborations
			rest = strings.Split(rest, " y ")[0]
			if len([]rune(rest)) >= 2 {
				return true
			}
		}
	}
	return false
}

// isEpistemicCheck: "are you sure?" about the last claim — not a domain question.
func isEpistemicCheck(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if wordCount(s) > 8 {
		return false
	}
	if isNewTopicLookup(s) {
		return false
	}
	roots := []string{
		"seguro", "confirm", "cierto", "verdad", "en serio", "de veras",
		"estás segur", "estas segur", "lo afirm", "no mientes",
	}
	return hasAnyRoot(s, roots)
}

// isNovelDeclarative: statement that is not chat/command/query — candidate for memory.
func isNovelDeclarative(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || isCalmChat(s) || isIdentityTalk(s) || isMemoryQuery(s) || isPersonalFact(s) {
		return false
	}
	if strings.Contains(s, "no me refiero") || strings.Contains(s, "me refiero a") {
		return false
	}
	if isElaborationRequest(s) || isEpistemicCheck(s) || isContinuePrompt(s) || isConfirmationPrompt(s) {
		return false
	}
	if isNewTopicLookup(s) || isDestructiveOrder(s) || isConstructiveOrder(s) {
		return false
	}
	if strings.Contains(s, "?") {
		return false
	}
	if wordCount(s) < 4 && len([]rune(s)) < 20 {
		return false
	}
	// Looks like a claim: has content letters and is not pure order
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if letters < 12 {
		return false
	}
	// Heuristic predicates / linking
	preds := []string{" es ", " son ", " está ", " esta ", " tiene ", " tienen ",
		" fue ", " será ", " vivo ", " trabajo ", " estudio ", " me gusta ", " odio ",
		" creo que ", " pienso que ", " hoy ", " ayer ", " siempre ", " nunca "}
	if hasAnyRoot(s, preds) {
		return true
	}
	// Long enough free statement
	return wordCount(s) >= 6
}

// shouldCaptureEscape: corpus miss + novel material → organs/memory must catch it.
func shouldCaptureEscape(text string, knowHit string) bool {
	if strings.TrimSpace(knowHit) != "" {
		return false
	}
	return isNovelDeclarative(text) || isPersonalFact(text) || isWorldFact(text)
}
