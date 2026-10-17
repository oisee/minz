package abap

import (
	"fmt"
	"strconv"
	"strings"
)

// parseExpression consumes every token. Unsupported syntax is an admission
// error, never a truncated expression. ABAP NOT binds less tightly than a
// comparison, whereas arithmetic unary signs bind more tightly than products.
func parseExpression(tokens []string) (Expr_, error) {
	p := expressionParser{tokens: tokens}
	e, err := p.parse(1)
	if err != nil {
		return nil, err
	}
	if p.pos != len(tokens) {
		return nil, fmt.Errorf("unsupported expression token %q at token %d", tokens[p.pos], p.pos+1)
	}
	return e, nil
}

type expressionParser struct {
	tokens []string
	pos    int
}

func expressionPrecedence(op string) int {
	switch strings.ToUpper(op) {
	case "OR":
		return 1
	case "AND":
		return 2
	case "=", "<>", "<", ">", "<=", ">=", "EQ", "NE", "LT", "GT", "LE", "GE":
		return 3
	case "+", "-":
		return 4
	case "*", "/", "DIV", "MOD":
		return 5
	}
	return 0
}

func (p *expressionParser) parse(min int) (Expr_, error) {
	if p.pos == len(p.tokens) {
		return nil, fmt.Errorf("missing expression operand")
	}
	t := p.tokens[p.pos]
	p.pos++
	// abaplint represents a system-field chain as SY, -, INDEX tokens.
	// SY is the reserved system structure, not an arithmetic scalar operand.
	if strings.EqualFold(t, "SY") && p.pos+1 < len(p.tokens) && p.tokens[p.pos] == "-" {
		t += "-" + p.tokens[p.pos+1]
		p.pos += 2
	}
	var lhs Expr_
	var err error
	switch strings.ToUpper(t) {
	case "NOT", "+", "-":
		binding := 6
		if strings.EqualFold(t, "NOT") {
			binding = 3
		}
		var val Expr_
		val, err = p.parse(binding)
		lhs = &UnaryOp{Op: strings.ToUpper(t), Val: val}
	case "(":
		lhs, err = p.parse(1)
		if err == nil {
			if p.pos == len(p.tokens) || p.tokens[p.pos] != ")" {
				return nil, fmt.Errorf("missing closing parenthesis")
			}
			p.pos++
		}
	default:
		lhs, err = expressionAtom(t)
	}
	if err != nil {
		return nil, err
	}
	for p.pos < len(p.tokens) {
		op := p.tokens[p.pos]
		prec := expressionPrecedence(op)
		if prec < min {
			break
		}
		p.pos++
		rhs, err := p.parse(prec + 1)
		if err != nil {
			return nil, err
		}
		lhs = &BinOp{Op: strings.ToUpper(op), LHS: lhs, RHS: rhs}
	}
	return lhs, nil
}

func expressionAtom(s string) (Expr_, error) {
	if len(s) >= 2 && ((s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '`' && s[len(s)-1] == '`')) {
		quote := string(s[0])
		return &StringLit{Val: strings.ReplaceAll(s[1:len(s)-1], quote+quote, quote), IsString: s[0] == '`'}, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return &IntLit{Val: n}, nil
	}
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "SY-") {
		field := upper[3:]
		if expressionIdentifier(field) {
			return &SYField{Field: field}, nil
		}
	}
	if expressionIdentifier(s) {
		return &VarRef{Name: s}, nil
	}
	return nil, fmt.Errorf("unsupported expression operand %q", s)
}

func expressionIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			continue
		}
		if i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

func parseDeclarationLiteral(tokens []string) (Expr_, error) {
	e, err := parseExpression(tokens)
	if err != nil {
		return nil, err
	}
	if u, ok := e.(*UnaryOp); ok {
		if lit, ok := u.Val.(*IntLit); ok {
			switch u.Op {
			case "+":
				e = lit
			case "-":
				e = &IntLit{Val: -lit.Val}
			}
		}
	}
	switch e.(type) {
	case *IntLit, *StringLit:
		return e, nil
	default:
		return nil, fmt.Errorf("only numeric or character literal initialization is admitted")
	}
}
