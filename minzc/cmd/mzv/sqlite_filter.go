package main

import (
	"fmt"
	"strings"
	"unicode"
)

// sqliteFilteredQuery preserves the caller's SELECT, including ORDER/LIMIT and
// existing WHERE, by filtering its result rows. Column authority is a single
// output-column identifier, never executable SQL; filter data is bound.
func sqliteFilteredQuery(base, column, filter string) (string, []any, error) {
	if filter == "" || filter == "*" || filter == "%" {
		return base, nil, nil
	}
	if column == "" {
		return "", nil, fmt.Errorf("empty filter column")
	}
	for i, r := range column {
		if !(r == '_' || unicode.IsLetter(r) || i > 0 && unicode.IsDigit(r)) {
			return "", nil, fmt.Errorf("invalid filter column %q", column)
		}
	}
	base = strings.TrimSpace(base)
	base = strings.TrimSuffix(base, ";")
	if base == "" {
		return "", nil, fmt.Errorf("empty base query")
	}
	filter = strings.ReplaceAll(filter, "*", "%")
	op := "="
	if strings.Contains(filter, "%") {
		op = "LIKE"
	}
	return fmt.Sprintf("SELECT * FROM (%s) AS _minz_filtered WHERE _minz_filtered.\"%s\" %s ?", base, column, op), []any{filter}, nil
}
