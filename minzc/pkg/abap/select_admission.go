package abap

import (
	"fmt"
	"strings"
)

// Clauses whose semantics are not represented by SelectStmt must not be
// consumed as INTO targets, embedded in WHERE text, or silently discarded.
// String literal tokens are kept opaque: their contents are data, not clauses.
func validateSelectTokenAdmission(tokens []string) error {
	phrases := [][]string{
		{"ORDER", "BY"}, {"GROUP", "BY"}, {"UP", "TO"}, {"PACKAGE", "SIZE"},
		{"FOR", "ALL", "ENTRIES"}, {"BYPASSING", "BUFFER"}, {"CLIENT", "SPECIFIED"},
		{"USING", "CLIENT"}, {"WITH", "PRIVILEGED", "ACCESS"},
	}
	for i, token := range tokens {
		if strings.HasPrefix(token, "'") || strings.HasPrefix(token, "`") {
			continue
		}
		for _, phrase := range phrases {
			if i+len(phrase) > len(tokens) {
				continue
			}
			match := true
			for j, word := range phrase {
				if !strings.EqualFold(tokens[i+j], word) {
					match = false
					break
				}
			}
			if match {
				return fmt.Errorf("SELECT: unsupported addition %s", strings.Join(phrase, " "))
			}
		}
		switch strings.ToUpper(token) {
		case "DISTINCT", "HAVING", "UNION", "INTERSECT", "EXCEPT", "OFFSET", "CONNECTION", "APPENDING":
			return fmt.Errorf("SELECT: unsupported addition %s", strings.ToUpper(token))
		}
		if token == "@" || strings.HasPrefix(token, "@") {
			// Host escaping is implemented for INTO destinations, not for WHERE or
			// expressions in the SQL sent to the database. Validate that context below.
			into := -1
			for j := 0; j < i; j++ {
				if strings.EqualFold(tokens[j], "INTO") {
					into = j
				}
			}
			inDestination := into >= 0
			for j := into + 1; j < i && inDestination; j++ {
				if strings.EqualFold(tokens[j], "WHERE") {
					inDestination = false
				}
			}
			if !inDestination {
				return fmt.Errorf("SELECT: host expressions outside INTO are not implemented")
			}
		}
	}
	return nil
}
