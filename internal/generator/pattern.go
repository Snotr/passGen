package generator

import (
	"fmt"
	"strings"

	"github.com/Snotr/passGen/internal/parser"
)

// ParsePattern extracts an ordered list of tag names from a pattern template
// like "{names}{numbers}{symbols}". Returns an error if a referenced tag
// does not exist in the provided groups.
func ParsePattern(pattern string, groups []parser.TagGroup) ([]string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}

	tagSet := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		tagSet[g.Tag] = struct{}{}
	}

	var tags []string
	rest := pattern
	for rest != "" {
		open := strings.IndexByte(rest, '{')
		if open == -1 {
			return nil, fmt.Errorf("unexpected text %q outside braces in pattern", rest)
		}
		if open > 0 {
			return nil, fmt.Errorf("unexpected text %q before '{' in pattern", rest[:open])
		}
		close := strings.IndexByte(rest, '}')
		if close == -1 {
			return nil, fmt.Errorf("unclosed '{' in pattern")
		}
		tag := strings.ToLower(strings.TrimSpace(rest[1:close]))
		if tag == "" {
			return nil, fmt.Errorf("empty tag name in pattern")
		}
		if _, ok := tagSet[tag]; !ok {
			return nil, fmt.Errorf("unknown tag %q in pattern; available: %v", tag, availableTags(groups))
		}
		tags = append(tags, tag)
		rest = rest[close+1:]
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("pattern contains no tag references")
	}

	return tags, nil
}

func availableTags(groups []parser.TagGroup) []string {
	tags := make([]string, len(groups))
	for i, g := range groups {
		tags[i] = g.Tag
	}
	return tags
}
