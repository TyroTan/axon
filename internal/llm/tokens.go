// Package llm — naive token utilities.
package llm

import "strings"

// CountTokensNaive estimates token count using the ~4 chars/token rule.
// Pure function, deterministic, zero dependencies.
// Accurate to ±15% for English prose; good enough for budget gating.
func CountTokensNaive(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4 // ceiling division
}

// CountTokensMap sums CountTokensNaive over all values in a map.
func CountTokensMap(files map[string]string) int {
	total := 0
	for _, v := range files {
		total += CountTokensNaive(v)
	}
	return total
}

// TruncateToTokenLimit trims s to approximately maxTokens tokens.
// Returns the trimmed string and whether truncation occurred.
func TruncateToTokenLimit(s string, maxTokens int) (string, bool) {
	maxChars := maxTokens * 4
	if len(s) <= maxChars {
		return s, false
	}
	// Trim at a newline boundary near the limit.
	cut := s[:maxChars]
	if idx := strings.LastIndex(cut, "\n"); idx > maxChars/2 {
		cut = cut[:idx]
	}
	return cut, true
}
