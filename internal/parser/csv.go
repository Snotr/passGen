package parser

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// ParseCSV reads a .csv file and returns parsed TagGroups.
// The first row is treated as headers. If headers start with '#', they define
// tag names; each column's data belongs to that tag. Extra columns beyond
// the headers are ignored; missing columns produce no entry for that row.
func ParseCSV(path string) ([]TagGroup, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // allow ragged rows
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv %s: %w", path, err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	headers := records[0]
	tags := make([]string, len(headers))
	for i, h := range headers {
		h = strings.TrimSpace(h)
		h = strings.TrimPrefix(h, "#")
		h = strings.ToLower(strings.TrimSpace(h))
		tags[i] = h
	}

	groups := make([]TagGroup, len(tags))
	for i, tag := range tags {
		groups[i] = TagGroup{Tag: tag}
	}

	for _, row := range records[1:] {
		for col, val := range row {
			if col >= len(tags) {
				break
			}
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			groups[col].Values = append(groups[col].Values, val)
		}
	}

	// Remove groups with no values
	result := groups[:0]
	for _, g := range groups {
		if len(g.Values) > 0 {
			result = append(result, g)
		}
	}

	return result, nil
}
