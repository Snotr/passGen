// Package parser reads tagged data from TXT and CSV files and groups values by tag.
package parser

import (
	"strings"
	"unicode"
)

// TagGroup holds all values belonging to a single #tag.
type TagGroup struct {
	Tag    string
	Values []string
}

// ParseResult aggregates groups from all parsed files.
type ParseResult struct {
	Groups []TagGroup
}

// MergeGroups consolidates TagGroups with the same Tag name,
// deduplicating values while preserving insertion order.
func MergeGroups(groups []TagGroup) []TagGroup {
	order := []string{}
	index := map[string]int{}
	seen := map[string]map[string]struct{}{}

	for _, g := range groups {
		tag := g.Tag
		if _, exists := index[tag]; !exists {
			index[tag] = len(order)
			order = append(order, tag)
			seen[tag] = map[string]struct{}{}
		}
	}

	merged := make([]TagGroup, len(order))
	for i, tag := range order {
		merged[i] = TagGroup{Tag: tag}
	}

	for _, g := range groups {
		idx := index[g.Tag]
		for _, v := range g.Values {
			if _, dup := seen[g.Tag][v]; !dup {
				seen[g.Tag][v] = struct{}{}
				merged[idx].Values = append(merged[idx].Values, v)
			}
		}
	}

	return merged
}

// ClassifyValue determines a default tag for a value based on its content.
// Returns "numbers" if all runes are digits, "symbols" if all runes are
// punctuation/symbols, or "words" otherwise.
func ClassifyValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "words"
	}

	allDigits := true
	allSymbols := true
	for _, r := range s {
		if !unicode.IsDigit(r) {
			allDigits = false
		}
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			allSymbols = false
		}
	}

	if allDigits {
		return "numbers"
	}
	if allSymbols {
		return "symbols"
	}
	return "words"
}
