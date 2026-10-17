package abap

import (
	"fmt"
	"strings"
)

// Every WRITE operand consumes its complete token segment. Formatting additions
// cannot masquerade as extra operands even when a similarly named object exists.
func parseWriteStmt(tokens []string) (Decl, error) {
	if len(tokens) < 3 || tokens[len(tokens)-1] != "." {
		return nil, fmt.Errorf("WRITE: incomplete statement")
	}
	start := 1
	if tokens[start] == ":" {
		start++
	}
	w := &WriteStmt{}
	for start < len(tokens)-1 {
		end := start
		template := false
		for end < len(tokens)-1 {
			token := tokens[end]
			if !template && token == "," {
				break
			}
			if strings.HasPrefix(token, "|") {
				template = true
			}
			if template && strings.HasSuffix(token, "|") {
				template = false
			}
			end++
		}
		chunk := tokens[start:end]
		if len(chunk) > 0 && chunk[0] == "/" {
			w.Exprs = append(w.Exprs, &StringLit{Val: "\r\n"})
			chunk = chunk[1:]
		}
		if len(chunk) > 0 {
			if strings.HasPrefix(chunk[0], "|") {
				parts, err := parseWriteTemplate(chunk)
				if err != nil {
					return nil, err
				}
				w.Exprs = append(w.Exprs, parts...)
			} else {
				expr, err := parseExpression(chunk)
				if err != nil {
					return nil, fmt.Errorf("WRITE: %w", err)
				}
				w.Exprs = append(w.Exprs, expr)
			}
		} else if end == start {
			return nil, fmt.Errorf("WRITE: empty operand")
		}
		start = end + 1
	}
	if len(w.Exprs) == 0 {
		return nil, fmt.Errorf("WRITE: missing operand")
	}
	return &formBodyDecl{stmt: w}, nil
}

// abaplint retains literal text in the boundary tokens and splits expression
// tokens. Joining adds whitespace inside interpolation expressions, which is
// ignored by the expression parser, while preserving the literal boundary text.
func parseWriteTemplate(tokens []string) ([]Expr_, error) {
	text := strings.Join(tokens, " ")
	if len(text) < 2 || text[0] != '|' || text[len(text)-1] != '|' {
		return nil, fmt.Errorf("WRITE template: incomplete or trailing formatting addition")
	}
	text = text[1 : len(text)-1]
	var out []Expr_
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			out = append(out, &StringLit{Val: literal.String(), IsString: true})
			literal.Reset()
		}
	}
	for i := 0; i < len(text); {
		switch text[i] {
		case '\\':
			i++
			if i == len(text) {
				return nil, fmt.Errorf("WRITE template: incomplete escape")
			}
			switch text[i] {
			case 'n':
				literal.WriteByte('\n')
			case 'r':
				literal.WriteByte('\r')
			case 't':
				literal.WriteByte('\t')
			case '\\', '|', '{', '}':
				literal.WriteByte(text[i])
			default:
				return nil, fmt.Errorf("WRITE template: unsupported escape \\%c", text[i])
			}
			i++
		case '{':
			flush()
			i++
			begin := i
			var quote byte
			for i < len(text) {
				ch := text[i]
				if quote != 0 {
					if ch == quote {
						if i+1 < len(text) && text[i+1] == quote {
							i += 2
							continue
						}
						quote = 0
					}
				} else {
					if ch == '\'' || ch == '`' {
						quote = ch
					} else if ch == '}' {
						break
					} else if ch == '{' {
						return nil, fmt.Errorf("WRITE template: nested templates are not implemented")
					}
				}
				i++
			}
			if i == len(text) {
				return nil, fmt.Errorf("WRITE template: missing closing brace")
			}
			parts, err := writeTemplateExpressionTokens(text[begin:i])
			if err != nil {
				return nil, err
			}
			value, err := parseExpression(parts)
			if err != nil {
				return nil, fmt.Errorf("WRITE template: %w", err)
			}
			out = append(out, &TemplateValue{Val: value})
			i++
		case '}', '|':
			return nil, fmt.Errorf("WRITE template: unescaped delimiter")
		default:
			literal.WriteByte(text[i])
			i++
		}
	}
	flush()
	// An empty template is an explicit zero-length text operand.
	if len(out) == 0 {
		out = append(out, &StringLit{Val: "", IsString: true})
	}
	return out, nil
}

func writeTemplateExpressionTokens(text string) ([]string, error) {
	var tokens []string
	for i := 0; i < len(text); {
		if strings.ContainsRune(" \t\r\n", rune(text[i])) {
			i++
			continue
		}
		begin := i
		if text[i] == '\'' || text[i] == '`' {
			quote := text[i]
			i++
			closed := false
			for i < len(text) {
				if text[i] == quote {
					if i+1 < len(text) && text[i+1] == quote {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("WRITE template: unterminated expression literal")
			}
		} else {
			for i < len(text) && !strings.ContainsRune(" \t\r\n", rune(text[i])) {
				i++
			}
		}
		tokens = append(tokens, text[begin:i])
	}
	return tokens, nil
}
