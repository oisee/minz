package abap

import (
	"fmt"
	"strconv"
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

// An unescaped RHS name in classic Open SQL can denote an ABAP host object.
// Sending it unchanged to SQLite can instead read a same-named database column.
// Until binding is implemented, admit literal RHS values and explicitly
// table-qualified column references, rather than guessing from schema names.
func validateSelectPredicateOperands(tokens []string) error {
	between := false
	for i, token := range tokens {
		switch strings.ToUpper(token) {
		case "+", "-", "*", "/":
			// Keep signed literal RHS values; do not send a host-containing
			// arithmetic expression to SQLite merely because it starts with a literal.
			if (token == "+" || token == "-") && i > 0 {
				switch strings.ToUpper(tokens[i-1]) {
				case "=", "!=", "<>", "<", ">", "<=", ">=", "BETWEEN", "AND":
					continue
				}
			}
			return fmt.Errorf("SELECT: predicate arithmetic expressions are not implemented")
		case "BETWEEN":
			between = true
		case "AND":
			if !between {
				continue
			}
			between = false
		case "=", "!=", "<>", "<", ">", "<=", ">=", "LIKE", "IN":
		default:
			continue
		}
		if i+1 >= len(tokens) {
			return fmt.Errorf("SELECT: incomplete predicate")
		}
		rhs := tokens[i+1]
		if strings.HasPrefix(rhs, "'") || strings.EqualFold(rhs, "NULL") {
			continue
		}
		if _, err := strconv.ParseFloat(rhs, 64); err == nil && len(rhs) > 0 && ((rhs[0] >= '0' && rhs[0] <= '9') || rhs[0] == '-' || rhs[0] == '+') {
			continue
		}
		if (rhs == "-" || rhs == "+") && i+2 < len(tokens) {
			if _, err := strconv.ParseFloat(tokens[i+2], 64); err == nil {
				continue
			}
		}
		if strings.Contains(rhs, "~") {
			continue
		}
		if i+2 < len(tokens) && tokens[i+2] == "~" {
			continue
		}
		return fmt.Errorf("SELECT: unescaped host operands or predicate expressions are not implemented: %s", rhs)
	}
	return nil
}
