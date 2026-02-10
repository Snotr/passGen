package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseTXT reads a .txt file and returns parsed TagGroups.
// Lines starting with '#' followed by a non-whitespace identifier switch the
// active tag. All subsequent non-empty lines are added to that tag's values.
// Lines before any explicit tag are classified by content heuristic.
func ParseTXT(path string) ([]TagGroup, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var groups []TagGroup
	tagIndex := map[string]int{}
	currentTag := ""

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if tag, ok := strings.CutPrefix(line, "#"); ok {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			// Take only the first word as the tag name
			if idx := strings.IndexFunc(tag, func(r rune) bool {
				return r == ' ' || r == '\t'
			}); idx != -1 {
				tag = tag[:idx]
			}
			tag = strings.ToLower(tag)
			currentTag = tag
			if _, exists := tagIndex[tag]; !exists {
				tagIndex[tag] = len(groups)
				groups = append(groups, TagGroup{Tag: tag})
			}
			continue
		}

		// Determine which tag this value belongs to
		tag := currentTag
		if tag == "" {
			tag = ClassifyValue(line)
			currentTag = "" // don't stick; classify each untagged line individually
		}

		idx, exists := tagIndex[tag]
		if !exists {
			idx = len(groups)
			tagIndex[tag] = idx
			groups = append(groups, TagGroup{Tag: tag})
		}
		groups[idx].Values = append(groups[idx].Values, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return groups, nil
}
