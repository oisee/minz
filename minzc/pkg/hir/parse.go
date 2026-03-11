package hir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// ParseHIR parses the text format produced by (*Module).Dump().
// Returns a *Module on success, or an error describing the first parse failure.
func ParseHIR(src, name string) (*Module, error) {
	p := &hirParser{
		lines: strings.Split(src, "\n"),
		pos:   0,
		name:  name,
	}
	return p.parseModule()
}

// hirParser is a line-oriented parser for the HIR text format.
type hirParser struct {
	lines []string
	pos   int
	name  string

	// struct registry for type lookup by name
	structs map[string]*mir2.StructTy
}

// peek returns the current line without consuming it (trimmed of trailing whitespace).
// Returns "" at EOF.
func (p *hirParser) peek() string {
	for p.pos < len(p.lines) {
		return p.lines[p.pos]
	}
	return ""
}

// peekTrimmed returns the current non-empty, non-comment line's trimmed content,
// advancing past empty/comment lines first. Does NOT consume the line.
func (p *hirParser) peekContent() (string, bool) {
	for p.pos < len(p.lines) {
		line := p.lines[p.pos]
		trimmed := strings.TrimRight(line, " \t\r")
		if trimmed == "" {
			p.pos++
			continue
		}
		return line, true
	}
	return "", false
}

// next consumes and returns the current line.
func (p *hirParser) next() string {
	if p.pos >= len(p.lines) {
		return ""
	}
	line := p.lines[p.pos]
	p.pos++
	return line
}

// indentOf counts leading spaces in a line.
func (p *hirParser) indentOf(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else {
			break
		}
	}
	return count
}

// skipBlanksAndComments advances past blank lines and comment-only lines,
// but NOT past lines that start with ';' that are the module header.
func (p *hirParser) skipBlanksAndComments() {
	for p.pos < len(p.lines) {
		line := strings.TrimRight(p.lines[p.pos], " \t\r")
		if line == "" {
			p.pos++
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, ";") {
			p.pos++
			continue
		}
		break
	}
}

// parseModule parses the entire HIR text format.
func (p *hirParser) parseModule() (*Module, error) {
	p.structs = make(map[string]*mir2.StructTy)
	m := &Module{}

	// Parse module header: "; HIR module: name"
	// Look for it in first few lines.
	modName := p.name
	for p.pos < len(p.lines) {
		line := strings.TrimRight(p.lines[p.pos], " \t\r")
		p.pos++
		if strings.HasPrefix(line, "; HIR module:") {
			modName = strings.TrimSpace(strings.TrimPrefix(line, "; HIR module:"))
			break
		}
		// blank line before header is ok
		if line == "" {
			continue
		}
		// non-comment, non-blank — put it back and continue
		p.pos--
		break
	}
	m.Name = modName

	// Parse top-level items
	for {
		p.skipBlanksAndComments()
		if p.pos >= len(p.lines) {
			break
		}
		line := strings.TrimRight(p.lines[p.pos], " \t\r")
		if line == "" {
			p.pos++
			continue
		}

		switch {
		case strings.HasPrefix(line, "struct @"):
			s, err := p.parseStruct(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", p.pos, err)
			}
			m.Structs = append(m.Structs, s)
			p.structs[s.Name] = s
			p.pos++

		case strings.HasPrefix(line, "global "):
			g, err := p.parseGlobal(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", p.pos, err)
			}
			m.Globals = append(m.Globals, g)
			p.pos++

		case strings.HasPrefix(line, "string #"):
			s, err := p.parseString(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", p.pos, err)
			}
			m.Strings = append(m.Strings, s)
			p.pos++

		case strings.HasPrefix(line, "fun @") || strings.HasPrefix(line, "extern fun @"):
			f, err := p.parseFunc(m)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", p.pos, err)
			}
			m.Funcs = append(m.Funcs, f)

		default:
			// Skip unknown top-level lines silently
			p.pos++
		}
	}

	return m, nil
}

// parseStruct parses: struct @Name { field: type, field: type }
func (p *hirParser) parseStruct(line string) (*mir2.StructTy, error) {
	// "struct @Name { f1: t1, f2: t2 }"
	line = strings.TrimPrefix(line, "struct @")
	brace := strings.Index(line, "{")
	if brace < 0 {
		return nil, fmt.Errorf("struct: missing '{'")
	}
	name := strings.TrimSpace(line[:brace])
	rest := line[brace+1:]
	end := strings.Index(rest, "}")
	if end < 0 {
		return nil, fmt.Errorf("struct: missing '}'")
	}
	body := strings.TrimSpace(rest[:end])

	st := &mir2.StructTy{Name: name}
	if body != "" {
		for _, part := range strings.Split(body, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			colon := strings.Index(part, ":")
			if colon < 0 {
				return nil, fmt.Errorf("struct field missing ':': %q", part)
			}
			fname := strings.TrimSpace(part[:colon])
			ftypeStr := strings.TrimSpace(part[colon+1:])
			fty := p.parseType(ftypeStr)
			st.Fields = append(st.Fields, mir2.StructField{Name: fname, Ty: fty})
		}
	}
	return st, nil
}

// parseGlobal parses: global [const] @name : type
func (p *hirParser) parseGlobal(line string) (mir2.Global, error) {
	line = strings.TrimPrefix(line, "global ")
	isConst := false
	if strings.HasPrefix(line, "const ") {
		isConst = true
		line = strings.TrimPrefix(line, "const ")
	}
	line = strings.TrimPrefix(line, "@")
	colon := strings.Index(line, ":")
	if colon < 0 {
		return mir2.Global{}, fmt.Errorf("global: missing ':'")
	}
	gname := strings.TrimSpace(line[:colon])
	tyStr := strings.TrimSpace(line[colon+1:])
	ty := p.parseType(tyStr)
	return mir2.Global{Name: gname, Ty: ty, IsConst: isConst}, nil
}

// parseString parses: string #N = "..."
func (p *hirParser) parseString(line string) (string, error) {
	// "string #N = "..."
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", fmt.Errorf("string: missing '='")
	}
	quoted := strings.TrimSpace(line[eq+1:])
	s, err := strconv.Unquote(quoted)
	if err != nil {
		return "", fmt.Errorf("string: bad quoted string %q: %w", quoted, err)
	}
	return s, nil
}

// parseFunc parses a function header and optional body.
// Advances p.pos past the function (header + all body lines).
func (p *hirParser) parseFunc(m *Module) (*Func, error) {
	line := strings.TrimRight(p.lines[p.pos], " \t\r")
	p.pos++

	isExtern := false
	if strings.HasPrefix(line, "extern ") {
		isExtern = true
		line = strings.TrimPrefix(line, "extern ")
	}
	line = strings.TrimPrefix(line, "fun @")

	// Parse name
	paren := strings.Index(line, "(")
	if paren < 0 {
		return nil, fmt.Errorf("fun: missing '('")
	}
	fname := strings.TrimSpace(line[:paren])
	rest := line[paren+1:]

	// Find closing ')'
	close := strings.Index(rest, ")")
	if close < 0 {
		return nil, fmt.Errorf("fun: missing ')'")
	}
	paramStr := strings.TrimSpace(rest[:close])
	afterParen := strings.TrimSpace(rest[close+1:])

	// Parse parameters
	var params []Param
	if paramStr != "" {
		for _, part := range splitParams(paramStr) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			colon := strings.Index(part, ":")
			if colon < 0 {
				return nil, fmt.Errorf("fun param missing ':': %q", part)
			}
			pname := strings.TrimSpace(part[:colon])
			ptypeStr := strings.TrimSpace(part[colon+1:])
			pty := p.parseType(ptypeStr)
			params = append(params, Param{Name: pname, Ty: pty})
		}
	}

	// Parse return type
	retTy := mir2.Ty(mir2.TyVoid)
	if strings.HasPrefix(afterParen, "->") {
		retStr := strings.TrimSpace(strings.TrimPrefix(afterParen, "->"))
		retTy = p.parseType(retStr)
	}

	f := &Func{
		Name:     fname,
		Params:   params,
		RetTy:    retTy,
		IsExtern: isExtern,
	}

	// Parse body: lines with indent >= 2
	if p.pos < len(p.lines) {
		body, err := p.parseBlock(2)
		if err != nil {
			return nil, fmt.Errorf("fun %s body: %w", fname, err)
		}
		if len(body.Body) > 0 {
			f.Body = body
		}
	}

	return f, nil
}

// splitParams splits a parameter list by commas, respecting nested parens/brackets.
func splitParams(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range s {
		switch ch {
		case '(', '[', '<':
			depth++
		case ')', ']', '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// parseBlock parses a block of statements at the given indent level.
// Stops when it encounters a line with less indentation (or EOF).
func (p *hirParser) parseBlock(indent int) (*Block, error) {
	blk := &Block{}
	for {
		// Skip purely blank lines
		for p.pos < len(p.lines) {
			line := p.lines[p.pos]
			if strings.TrimRight(line, " \t\r") == "" {
				p.pos++
			} else {
				break
			}
		}
		if p.pos >= len(p.lines) {
			break
		}
		line := p.lines[p.pos]
		lineIndent := p.indentOf(line)

		// Stop if this line is at a lower indent level than expected
		if lineIndent < indent {
			break
		}

		// Skip comment lines at any indent
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ";") {
			p.pos++
			continue
		}

		stmt, err := p.parseStmt(indent)
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			blk.Body = append(blk.Body, stmt)
		}
	}
	return blk, nil
}

// parseStmt parses a single statement at the given indent level.
// Assumes the current line has the correct indentation.
func (p *hirParser) parseStmt(indent int) (Stmt, error) {
	if p.pos >= len(p.lines) {
		return nil, nil
	}
	line := p.lines[p.pos]
	lineIndent := p.indentOf(line)
	if lineIndent < indent {
		return nil, nil
	}
	content := strings.TrimSpace(line)
	p.pos++

	switch {
	case strings.HasPrefix(content, "var "):
		return p.parseVarDecl(content)

	case strings.HasPrefix(content, "return"):
		return p.parseReturn(content)

	case strings.HasPrefix(content, "if "):
		return p.parseIf(content, indent)

	case content == "else":
		// Should not reach here directly; handled by parseIf
		return nil, fmt.Errorf("unexpected 'else'")

	case strings.HasPrefix(content, "while "):
		return p.parseWhile(content, indent)

	case strings.HasPrefix(content, "for "):
		return p.parseFor(content, indent)

	case strings.HasPrefix(content, "foreach "):
		return p.parseForEach(content, indent)

	case content == "break":
		return &BreakStmt{}, nil

	case content == "continue":
		return &ContinueStmt{}, nil

	case strings.HasPrefix(content, "switch "):
		return p.parseSwitch(content, indent)

	case strings.HasPrefix(content, "*("):
		// Store: *(ptr_expr) = val_expr
		return p.parseStore(content)

	case strings.HasPrefix(content, "let ("):
		return p.parseTupleLet(content)

	default:
		// Try assignment: target_expr = val_expr
		// Or expression statement
		return p.parseAssignOrExpr(content)
	}
}

// parseVarDecl parses: var name: type [= expr] [at 0xADDR]
//                  or: var name: [N]type [= { exprs }]
func (p *hirParser) parseVarDecl(content string) (Stmt, error) {
	content = strings.TrimPrefix(content, "var ")
	// Find the colon
	colon := strings.Index(content, ":")
	if colon < 0 {
		return nil, fmt.Errorf("var decl: missing ':'")
	}
	vname := strings.TrimSpace(content[:colon])
	rest := strings.TrimSpace(content[colon+1:])

	// Check for array type: [N]type
	arrayLen := 0
	var elemTy mir2.Ty
	if strings.HasPrefix(rest, "[") {
		end := strings.Index(rest, "]")
		if end < 0 {
			return nil, fmt.Errorf("var decl: array missing ']'")
		}
		n, err := strconv.Atoi(rest[1:end])
		if err != nil {
			return nil, fmt.Errorf("var decl: bad array length: %w", err)
		}
		arrayLen = n
		rest = strings.TrimSpace(rest[end+1:])
		// rest is now the element type, possibly followed by = { ... }
		eqPos := strings.Index(rest, "=")
		var tyStr string
		if eqPos >= 0 {
			tyStr = strings.TrimSpace(rest[:eqPos])
			rest = strings.TrimSpace(rest[eqPos+1:])
		} else {
			tyStr = rest
			rest = ""
		}
		elemTy = p.parseType(tyStr)
	}

	if arrayLen > 0 {
		vd := &VarDeclStmt{Name: vname, Ty: elemTy, ArrayLen: arrayLen}
		if rest != "" && strings.HasPrefix(rest, "{") {
			// Parse initializer list: { expr, expr, ... }
			end := strings.LastIndex(rest, "}")
			if end >= 0 {
				body := strings.TrimSpace(rest[1:end])
				if body != "" {
					for _, part := range splitParams(body) {
						part = strings.TrimSpace(part)
						if part == "" {
							continue
						}
						e, err := p.parseExpr(part)
						if err != nil {
							return nil, fmt.Errorf("var decl array init: %w", err)
						}
						vd.Initial = append(vd.Initial, e)
					}
				}
			}
		}
		return vd, nil
	}

	// Scalar: split type from optional init/at
	// Type may be followed by = expr or at 0xADDR
	var tyStr string
	var initStr string
	var atAddr *uint16

	// Check for " at 0x"
	atIdx := strings.Index(rest, " at 0x")
	eqIdx := strings.Index(rest, " = ")
	if atIdx >= 0 && (eqIdx < 0 || atIdx < eqIdx) {
		tyStr = strings.TrimSpace(rest[:atIdx])
		addrStr := strings.TrimSpace(rest[atIdx+len(" at 0x"):])
		addr, err := strconv.ParseUint(addrStr, 16, 16)
		if err != nil {
			return nil, fmt.Errorf("var decl at: bad address %q: %w", addrStr, err)
		}
		a := uint16(addr)
		atAddr = &a
	} else if eqIdx >= 0 {
		tyStr = strings.TrimSpace(rest[:eqIdx])
		initStr = strings.TrimSpace(rest[eqIdx+3:])
	} else {
		tyStr = rest
	}

	ty := p.parseType(tyStr)
	vd := &VarDeclStmt{Name: vname, Ty: ty, At: atAddr}
	if initStr != "" {
		e, err := p.parseExpr(initStr)
		if err != nil {
			return nil, fmt.Errorf("var decl init: %w", err)
		}
		vd.Init = e
	}
	return vd, nil
}

// parseReturn parses: return | return expr | return (e1, e2)
func (p *hirParser) parseReturn(content string) (Stmt, error) {
	rest := strings.TrimPrefix(content, "return")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return &ReturnStmt{}, nil
	}
	// Multi-return: return (e1, e2) — detect by top-level comma inside outer parens.
	// A plain return (expr) without comma is a single-value return.
	if strings.HasPrefix(rest, "(") {
		// Find the closing ')' that matches the opening '('
		closeIdx := findCloseParen(rest[1:])
		if closeIdx >= 0 && closeIdx+2 == len(rest) {
			// The outer parens wrap the entire rest — could be multi-return or single expr.
			inner := rest[1 : len(rest)-1]
			// Check for top-level comma
			if hasTopLevelComma(inner) {
				parts := splitParams(inner)
				var vals []Expr
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					e, err := p.parseExpr(part)
					if err != nil {
						return nil, fmt.Errorf("return multi: %w", err)
					}
					vals = append(vals, e)
				}
				return &ReturnStmt{Vals: vals}, nil
			}
		}
	}
	// Single-value return: parse entire rest as one expression.
	e, err := p.parseExpr(rest)
	if err != nil {
		return nil, fmt.Errorf("return expr: %w", err)
	}
	return &ReturnStmt{Val: e}, nil
}

// hasTopLevelComma reports whether s contains a comma at depth 0 (not inside parens/brackets).
func hasTopLevelComma(s string) bool {
	depth := 0
	for _, ch := range s {
		switch ch {
		case '(', '[', '<':
			depth++
		case ')', ']', '>':
			depth--
		case ',':
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

// parseIf parses: if expr + body [else body]
func (p *hirParser) parseIf(content string, indent int) (Stmt, error) {
	condStr := strings.TrimPrefix(content, "if ")
	cond, err := p.parseExpr(condStr)
	if err != nil {
		return nil, fmt.Errorf("if cond: %w", err)
	}
	then, err := p.parseBlock(indent + 2)
	if err != nil {
		return nil, fmt.Errorf("if then: %w", err)
	}

	var els *Block
	// Check if next non-blank line at this indent is "else"
	savedPos := p.pos
	for p.pos < len(p.lines) {
		line := p.lines[p.pos]
		if strings.TrimRight(line, " \t\r") == "" {
			p.pos++
			continue
		}
		li := p.indentOf(line)
		trimmed := strings.TrimSpace(line)
		if li == indent && trimmed == "else" {
			p.pos++
			els, err = p.parseBlock(indent + 2)
			if err != nil {
				return nil, fmt.Errorf("if else: %w", err)
			}
			break
		}
		_ = savedPos
		break
	}
	return &IfStmt{Cond: cond, Then: then, Else: els}, nil
}

// parseWhile parses: while expr + body
func (p *hirParser) parseWhile(content string, indent int) (Stmt, error) {
	condStr := strings.TrimPrefix(content, "while ")
	cond, err := p.parseExpr(condStr)
	if err != nil {
		return nil, fmt.Errorf("while cond: %w", err)
	}
	body, err := p.parseBlock(indent + 2)
	if err != nil {
		return nil, fmt.Errorf("while body: %w", err)
	}
	return &WhileStmt{Cond: cond, Body: body}, nil
}

// parseFor parses: for var in start..end + body
func (p *hirParser) parseFor(content string, indent int) (Stmt, error) {
	// "for VAR in START..END"
	content = strings.TrimPrefix(content, "for ")
	inIdx := strings.Index(content, " in ")
	if inIdx < 0 {
		return nil, fmt.Errorf("for: missing ' in '")
	}
	varName := strings.TrimSpace(content[:inIdx])
	rest := strings.TrimSpace(content[inIdx+4:])

	// Find ".." not inside parens
	ddot := findDotDot(rest)
	if ddot < 0 {
		return nil, fmt.Errorf("for: missing '..'")
	}
	startStr := rest[:ddot]
	endStr := rest[ddot+2:]

	start, err := p.parseExpr(startStr)
	if err != nil {
		return nil, fmt.Errorf("for start: %w", err)
	}
	end, err := p.parseExpr(endStr)
	if err != nil {
		return nil, fmt.Errorf("for end: %w", err)
	}
	body, err := p.parseBlock(indent + 2)
	if err != nil {
		return nil, fmt.Errorf("for body: %w", err)
	}
	return &ForRangeStmt{Var: varName, Start: start, End: end, Body: body}, nil
}

// findDotDot finds ".." not inside parentheses.
func findDotDot(s string) int {
	depth := 0
	for i := 0; i < len(s)-1; i++ {
		switch s[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case '.':
			if depth == 0 && s[i+1] == '.' {
				return i
			}
		}
	}
	return -1
}

// parseForEach parses: foreach var: ElemTy in *(ptr)[0..len]
func (p *hirParser) parseForEach(content string, indent int) (Stmt, error) {
	// "foreach VAR: ELEMTY in *(PTR)[0..LEN]"
	content = strings.TrimPrefix(content, "foreach ")
	colon := strings.Index(content, ":")
	if colon < 0 {
		return nil, fmt.Errorf("foreach: missing ':'")
	}
	varName := strings.TrimSpace(content[:colon])
	rest := strings.TrimSpace(content[colon+1:])

	inIdx := strings.Index(rest, " in ")
	if inIdx < 0 {
		return nil, fmt.Errorf("foreach: missing ' in '")
	}
	elemTyStr := strings.TrimSpace(rest[:inIdx])
	rangeStr := strings.TrimSpace(rest[inIdx+4:])

	elemTy := p.parseType(elemTyStr)

	// Parse "*(PTR)[0..LEN]"
	// Find the inner ptr expression in *(...)
	if !strings.HasPrefix(rangeStr, "*(") {
		return nil, fmt.Errorf("foreach: expected '*(' got %q", rangeStr)
	}
	// Find matching closing paren
	ptrEnd := findMatchingParen(rangeStr[1:], 0)
	if ptrEnd < 0 {
		return nil, fmt.Errorf("foreach: unmatched '(' in ptr")
	}
	ptrStr := rangeStr[2 : ptrEnd+1]
	afterPtr := rangeStr[ptrEnd+2:]

	ptr, err := p.parseExpr(ptrStr)
	if err != nil {
		return nil, fmt.Errorf("foreach ptr: %w", err)
	}

	// Parse [0..LEN]
	if !strings.HasPrefix(afterPtr, "[") {
		return nil, fmt.Errorf("foreach: expected '[' after ptr")
	}
	bracketEnd := strings.Index(afterPtr, "]")
	if bracketEnd < 0 {
		return nil, fmt.Errorf("foreach: missing ']'")
	}
	rangeInner := afterPtr[1:bracketEnd]
	ddot := strings.Index(rangeInner, "..")
	if ddot < 0 {
		return nil, fmt.Errorf("foreach: missing '..' in range")
	}
	startStr := rangeInner[:ddot]
	lenStr := rangeInner[ddot+2:]

	var startExpr Expr
	if strings.TrimSpace(startStr) != "0" {
		startExpr, err = p.parseExpr(startStr)
		if err != nil {
			return nil, fmt.Errorf("foreach start: %w", err)
		}
	} else {
		startExpr = &IntLitExpr{Val: 0, Ty: mir2.TyU8}
	}

	lenExpr, err := p.parseExpr(lenStr)
	if err != nil {
		return nil, fmt.Errorf("foreach len: %w", err)
	}

	body, err := p.parseBlock(indent + 2)
	if err != nil {
		return nil, fmt.Errorf("foreach body: %w", err)
	}

	return &ForEachStmt{
		Var:    varName,
		ElemTy: elemTy,
		Ptr:    ptr,
		Start:  startExpr,
		Len:    lenExpr,
		Body:   body,
	}, nil
}

// findMatchingParen finds the index of the matching ')' for the '(' at position start in s.
// s[start] must be '('. Returns index of ')', or -1.
func findMatchingParen(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseSwitch parses: switch expr + cases
func (p *hirParser) parseSwitch(content string, indent int) (Stmt, error) {
	valStr := strings.TrimPrefix(content, "switch ")
	val, err := p.parseExpr(valStr)
	if err != nil {
		return nil, fmt.Errorf("switch val: %w", err)
	}

	sw := &SwitchStmt{Val: val}

	// Parse cases at indent+2
	for {
		// Skip blank lines
		for p.pos < len(p.lines) {
			if strings.TrimRight(p.lines[p.pos], " \t\r") == "" {
				p.pos++
			} else {
				break
			}
		}
		if p.pos >= len(p.lines) {
			break
		}
		line := p.lines[p.pos]
		li := p.indentOf(line)
		if li < indent+2 {
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "case ") && strings.HasSuffix(trimmed, ":") {
			p.pos++
			numStr := strings.TrimSuffix(strings.TrimPrefix(trimmed, "case "), ":")
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("switch case: bad value %q", numStr)
			}
			body, err := p.parseBlock(indent + 4)
			if err != nil {
				return nil, fmt.Errorf("switch case body: %w", err)
			}
			sw.Cases = append(sw.Cases, &SwitchCase{Val: n, Body: body})
		} else if trimmed == "default:" {
			p.pos++
			body, err := p.parseBlock(indent + 4)
			if err != nil {
				return nil, fmt.Errorf("switch default: %w", err)
			}
			sw.Default = body
		} else {
			break
		}
	}
	return sw, nil
}

// parseStore parses: *(ptr_expr) = val_expr
func (p *hirParser) parseStore(content string) (Stmt, error) {
	// Find the closing ')' of the ptr expr
	ptrEnd := findMatchingParen(content, 1)
	if ptrEnd < 0 {
		return nil, fmt.Errorf("store: unmatched '('")
	}
	ptrStr := content[2:ptrEnd]
	rest := strings.TrimSpace(content[ptrEnd+1:])
	if !strings.HasPrefix(rest, "=") {
		return nil, fmt.Errorf("store: missing '='")
	}
	valStr := strings.TrimSpace(rest[1:])

	ptr, err := p.parseExpr(ptrStr)
	if err != nil {
		return nil, fmt.Errorf("store ptr: %w", err)
	}
	val, err := p.parseExpr(valStr)
	if err != nil {
		return nil, fmt.Errorf("store val: %w", err)
	}
	return &StoreStmt{Ptr: ptr, Val: val}, nil
}

// parseTupleLet parses: let (name, name) = call @fn(...)
func (p *hirParser) parseTupleLet(content string) (Stmt, error) {
	// "let (names) = call @fn(args):type"
	content = strings.TrimPrefix(content, "let (")
	close := strings.Index(content, ")")
	if close < 0 {
		return nil, fmt.Errorf("tuple let: missing ')'")
	}
	nameList := content[:close]
	rest := strings.TrimSpace(content[close+1:])
	if !strings.HasPrefix(rest, "=") {
		return nil, fmt.Errorf("tuple let: missing '='")
	}
	callStr := strings.TrimSpace(rest[1:])

	var names []string
	for _, n := range strings.Split(nameList, ",") {
		names = append(names, strings.TrimSpace(n))
	}

	callExpr, err := p.parseExpr(callStr)
	if err != nil {
		return nil, fmt.Errorf("tuple let call: %w", err)
	}
	ce, ok := callExpr.(*CallExpr)
	if !ok {
		return nil, fmt.Errorf("tuple let: expected call expr, got %T", callExpr)
	}
	return &TupleLetStmt{Names: names, Call: ce}, nil
}

// parseAssignOrExpr tries to parse as assignment (target = val) or expression statement.
func (p *hirParser) parseAssignOrExpr(content string) (Stmt, error) {
	// Find " = " that's not inside parentheses, brackets, or call expressions.
	// We need to distinguish:
	//   (x:u8) = expr      → assignment
	//   call @f(...)       → expression stmt
	//   (expr):type        → expression stmt (BinExpr etc.)
	// Strategy: find " = " at top-level (not inside parens).
	eqIdx := findTopLevelAssign(content)
	if eqIdx >= 0 {
		targetStr := strings.TrimSpace(content[:eqIdx])
		valStr := strings.TrimSpace(content[eqIdx+3:])
		target, err := p.parseExpr(targetStr)
		if err != nil {
			return nil, fmt.Errorf("assign target: %w", err)
		}
		val, err := p.parseExpr(valStr)
		if err != nil {
			return nil, fmt.Errorf("assign val: %w", err)
		}
		return &AssignStmt{Target: target, Val: val}, nil
	}
	// Expression statement
	e, err := p.parseExpr(content)
	if err != nil {
		return nil, fmt.Errorf("expr stmt: %w", err)
	}
	return &ExprStmt{Expr: e}, nil
}

// findTopLevelAssign finds the position of " = " not inside parentheses.
// Returns -1 if not found.
func findTopLevelAssign(s string) int {
	depth := 0
	for i := 0; i < len(s)-2; i++ {
		switch s[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		}
		if depth == 0 && s[i] == ' ' && s[i+1] == '=' && s[i+2] == ' ' {
			return i
		}
	}
	return -1
}

// parseType parses a type string into a mir2.Ty.
func (p *hirParser) parseType(s string) mir2.Ty {
	s = strings.TrimSpace(s)
	switch s {
	case "u8":
		return mir2.TyU8
	case "u16":
		return mir2.TyU16
	case "u24":
		return mir2.TyU24
	case "u32":
		return mir2.TyU32
	case "i8":
		return mir2.TyI8
	case "i16":
		return mir2.TyI16
	case "i24":
		return mir2.TyI24
	case "i32":
		return mir2.TyI32
	case "bool":
		return mir2.TyBool
	case "void":
		return mir2.TyVoid
	case "ptr":
		return mir2.TyPtr
	case "f.8":
		return mir2.TyF0_8
	case "f.16":
		return mir2.TyF0_16
	case "f8.8":
		return mir2.TyF8_8
	case "f8.16":
		return mir2.TyF8_16
	case "f16.8":
		return mir2.TyF16_8
	case "f16.16":
		return mir2.TyF16_16
	}
	// Array type: [N]elem
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end >= 0 {
			n, err := strconv.Atoi(s[1:end])
			if err == nil {
				elem := p.parseType(s[end+1:])
				return mir2.NewArray(elem, n)
			}
		}
	}
	// Ranged type: base<lo..hi> — lo/hi inclusive in display, exclusive hi in storage
	if idx := strings.Index(s, "<"); idx >= 0 {
		base := p.parseType(s[:idx])
		rest := s[idx+1:]
		if end := strings.Index(rest, ">"); end >= 0 {
			rangeStr := rest[:end]
			ddot := strings.Index(rangeStr, "..")
			if ddot >= 0 {
				lo, err1 := strconv.ParseInt(rangeStr[:ddot], 10, 64)
				hiInc, err2 := strconv.ParseInt(rangeStr[ddot+2:], 10, 64)
				if err1 == nil && err2 == nil {
					return mir2.NewRanged(base, lo, hiInc+1)
				}
			}
		}
	}
	// Struct type by name
	if st, ok := p.structs[s]; ok {
		return st
	}
	// Unknown: fall back to u8 to avoid nil
	return mir2.TyU8
}

// parseExpr parses an expression in the HIR dump format.
// Expressions are fully parenthesized with :type suffixes.
func (p *hirParser) parseExpr(s string) (Expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty expression")
	}

	// true:bool / false:bool
	if s == "true:bool" {
		return &BoolLitExpr{Val: true}, nil
	}
	if s == "false:bool" {
		return &BoolLitExpr{Val: false}, nil
	}

	// Integer literal: N:type (N may be negative)
	// Must not start with '(' to distinguish from VarRef
	if !strings.HasPrefix(s, "(") && !strings.HasPrefix(s, "call") &&
		!strings.HasPrefix(s, "addr") && !strings.HasPrefix(s, "load") &&
		!strings.HasPrefix(s, "*") && !strings.HasPrefix(s, "cast") &&
		!strings.HasPrefix(s, "@ptr") {
		if colon := strings.LastIndex(s, ":"); colon >= 0 {
			numStr := s[:colon]
			tyStr := s[colon+1:]
			// Verify it's a number (may be negative)
			if n, err := strconv.ParseInt(strings.TrimSpace(numStr), 10, 64); err == nil {
				ty := p.parseType(tyStr)
				return &IntLitExpr{Val: n, Ty: ty}, nil
			}
		}
	}

	// call @fn(args):type
	if strings.HasPrefix(s, "call @") {
		return p.parseCallExpr(s)
	}

	// addr @sym:ptr
	if strings.HasPrefix(s, "addr @") {
		sym := strings.TrimPrefix(s, "addr @")
		sym = strings.TrimSuffix(sym, ":ptr")
		return &AddrOfExpr{Sym: sym}, nil
	}

	// load(expr):type
	if strings.HasPrefix(s, "load(") {
		return p.parseLoadExpr(s)
	}

	// *(expr):type  — DerefExpr
	if strings.HasPrefix(s, "*(") {
		return p.parseDerefExpr(s)
	}

	// cast(expr):type
	if strings.HasPrefix(s, "cast(") {
		return p.parseCastExpr(s)
	}

	// @ptr(elemTy, 0xADDR):ptr
	if strings.HasPrefix(s, "@ptr(") {
		return p.parseConstPtrExpr(s)
	}

	// Parenthesized expression: starts with '('
	if strings.HasPrefix(s, "(") {
		return p.parseParenExpr(s)
	}

	return nil, fmt.Errorf("unrecognized expression: %q", s)
}

// parseCallExpr parses: call @fn(args):type
func (p *hirParser) parseCallExpr(s string) (Expr, error) {
	s = strings.TrimPrefix(s, "call @")
	// find '('
	paren := strings.Index(s, "(")
	if paren < 0 {
		return nil, fmt.Errorf("call: missing '('")
	}
	fname := s[:paren]
	rest := s[paren+1:]

	// Find matching ')' for arg list
	closeIdx := findMatchingParen(rest, 0) - 1
	// Actually we need '(' to be at index 0 of rest, but the '(' is already consumed.
	// We need to find the close paren of the arg list.
	closeIdx = findCloseParen(rest)
	if closeIdx < 0 {
		return nil, fmt.Errorf("call: unmatched '('")
	}
	argStr := rest[:closeIdx]
	afterArgs := rest[closeIdx+1:]

	// Parse type suffix: :type
	ty := mir2.Ty(mir2.TyVoid)
	if strings.HasPrefix(afterArgs, ":") {
		tyStr := strings.TrimPrefix(afterArgs, ":")
		ty = p.parseType(tyStr)
	}

	var args []Expr
	if strings.TrimSpace(argStr) != "" {
		for _, part := range splitArgs(argStr) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			e, err := p.parseExpr(part)
			if err != nil {
				return nil, fmt.Errorf("call arg: %w", err)
			}
			args = append(args, e)
		}
	}
	return &CallExpr{Fn: fname, Args: args, Ty: ty}, nil
}

// findCloseParen finds the index of ')' that closes the first '(' implied before s.
// s is the content after the opening '(' — so we look for the matching ')'.
func findCloseParen(s string) int {
	depth := 1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitArgs splits a comma-separated argument list, respecting nested parens.
func splitArgs(s string) []string {
	return splitParams(s)
}

// parseLoadExpr parses: load(expr):type
func (p *hirParser) parseLoadExpr(s string) (Expr, error) {
	s = strings.TrimPrefix(s, "load(")
	// Find matching ')'
	closeIdx := findCloseParen(s)
	if closeIdx < 0 {
		return nil, fmt.Errorf("load: unmatched '('")
	}
	ptrStr := s[:closeIdx]
	rest := strings.TrimSpace(s[closeIdx+1:])
	ty := mir2.Ty(mir2.TyU8)
	if strings.HasPrefix(rest, ":") {
		ty = p.parseType(rest[1:])
	}
	ptr, err := p.parseExpr(ptrStr)
	if err != nil {
		return nil, fmt.Errorf("load ptr: %w", err)
	}
	return &LoadExpr{Ptr: ptr, Ty: ty}, nil
}

// parseDerefExpr parses: *(expr):type
func (p *hirParser) parseDerefExpr(s string) (Expr, error) {
	// s = "*(inner):type"
	s = s[1:] // remove leading '*'
	// s = "(inner):type"
	closeIdx := findCloseParen(s[1:]) // find ')' after '('
	if closeIdx < 0 {
		return nil, fmt.Errorf("deref: unmatched '('")
	}
	innerStr := s[1 : closeIdx+1]
	rest := strings.TrimSpace(s[closeIdx+2:])
	ty := mir2.Ty(mir2.TyU8)
	if strings.HasPrefix(rest, ":") {
		ty = p.parseType(rest[1:])
	}
	inner, err := p.parseExpr(innerStr)
	if err != nil {
		return nil, fmt.Errorf("deref inner: %w", err)
	}
	return &DerefExpr{Ptr: inner, Ty: ty}, nil
}

// parseCastExpr parses: cast(expr):type
func (p *hirParser) parseCastExpr(s string) (Expr, error) {
	s = strings.TrimPrefix(s, "cast(")
	closeIdx := findCloseParen(s)
	if closeIdx < 0 {
		return nil, fmt.Errorf("cast: unmatched '('")
	}
	innerStr := s[:closeIdx]
	rest := strings.TrimSpace(s[closeIdx+1:])
	ty := mir2.Ty(mir2.TyU8)
	if strings.HasPrefix(rest, ":") {
		ty = p.parseType(rest[1:])
	}
	inner, err := p.parseExpr(innerStr)
	if err != nil {
		return nil, fmt.Errorf("cast inner: %w", err)
	}
	return &CastExpr{X: inner, Ty: ty}, nil
}

// parseConstPtrExpr parses: @ptr(elemTy, 0xADDR):ptr
func (p *hirParser) parseConstPtrExpr(s string) (Expr, error) {
	s = strings.TrimPrefix(s, "@ptr(")
	closeIdx := findCloseParen(s)
	if closeIdx < 0 {
		return nil, fmt.Errorf("@ptr: unmatched '('")
	}
	inner := s[:closeIdx]
	comma := strings.Index(inner, ",")
	if comma < 0 {
		return nil, fmt.Errorf("@ptr: missing ','")
	}
	elemTyStr := strings.TrimSpace(inner[:comma])
	addrStr := strings.TrimSpace(inner[comma+1:])
	addrStr = strings.TrimPrefix(addrStr, "0x")
	addr, err := strconv.ParseUint(addrStr, 16, 16)
	if err != nil {
		return nil, fmt.Errorf("@ptr addr: %w", err)
	}
	elemTy := p.parseType(elemTyStr)
	return &ConstPtrExpr{ElemTy: elemTy, Addr: uint16(addr)}, nil
}

// parseParenExpr handles parenthesized expressions:
//   (varname:type)           → VarRefExpr
//   ((L) op (R)):type        → BinExpr
//   (op(X)):type             → UnaryExpr
//   (expr).field[+N]:type    → FieldExpr
//   (expr)[idx]:elemTy       → IndexExpr
func (p *hirParser) parseParenExpr(s string) (Expr, error) {
	if !strings.HasPrefix(s, "(") {
		return nil, fmt.Errorf("expected '(' got %q", s)
	}

	// Find the matching ')' for the outer '('
	outerClose := findCloseParen(s[1:])
	if outerClose < 0 {
		return nil, fmt.Errorf("unmatched '(' in: %q", s)
	}
	outerClose++ // adjust for offset

	inner := s[1:outerClose]
	after := s[outerClose+1:] // what comes after the outer ')'

	// Check for suffix: .field[+N]:type or [idx]:elemTy or :type
	// First, check what 'after' starts with.
	// If empty or ends with :type — this is a VarRef or the outer expr.
	// If starts with '.' — FieldExpr.
	// If starts with '[' — IndexExpr.

	// FieldExpr: (base_expr).field[+N]:type
	if strings.HasPrefix(after, ".") {
		// after = ".fieldname[+N]:type"
		rest := after[1:] // remove '.'
		dotdot := strings.Index(rest, "[+")
		if dotdot < 0 {
			return nil, fmt.Errorf("field expr: expected '[+' in %q", after)
		}
		fieldName := rest[:dotdot]
		rest = rest[dotdot+2:] // after "[+"
		// Find "]"
		bracketEnd := strings.Index(rest, "]")
		if bracketEnd < 0 {
			return nil, fmt.Errorf("field expr: missing ']'")
		}
		offsetStr := rest[:bracketEnd]
		rest = rest[bracketEnd+1:]
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			return nil, fmt.Errorf("field expr offset: %w", err)
		}
		ty := mir2.Ty(mir2.TyU8)
		if strings.HasPrefix(rest, ":") {
			ty = p.parseType(rest[1:])
		}
		base, err := p.parseExpr("(" + inner + ")")
		if err != nil {
			return nil, fmt.Errorf("field base: %w", err)
		}
		return &FieldExpr{X: base, Field: fieldName, Offset: offset, Ty: ty}, nil
	}

	// IndexExpr: (base_expr)[idx]:elemTy
	if strings.HasPrefix(after, "[") {
		// Find matching ']'
		bracketClose := strings.Index(after[1:], "]")
		if bracketClose < 0 {
			return nil, fmt.Errorf("index expr: missing ']'")
		}
		idxStr := after[1 : bracketClose+1]
		rest := after[bracketClose+2:]
		ty := mir2.Ty(mir2.TyU8)
		if strings.HasPrefix(rest, ":") {
			ty = p.parseType(rest[1:])
		}
		base, err := p.parseExpr("(" + inner + ")")
		if err != nil {
			return nil, fmt.Errorf("index base: %w", err)
		}
		idx, err := p.parseExpr(idxStr)
		if err != nil {
			return nil, fmt.Errorf("index idx: %w", err)
		}
		return &IndexExpr{Base: base, Idx: idx, ElemTy: ty}, nil
	}

	// :type suffix after outer ')': BinExpr or UnaryExpr
	if strings.HasPrefix(after, ":") {
		ty := p.parseType(after[1:])
		// Try to parse inner as BinExpr: (L) op (R)
		// or UnaryExpr: opX
		binExpr, err := p.tryParseBinExpr(inner, ty)
		if err == nil {
			return binExpr, nil
		}
		unaryExpr, err2 := p.tryParseUnaryExpr(inner, ty)
		if err2 == nil {
			return unaryExpr, nil
		}
		return nil, fmt.Errorf("parenthesized expr: can't parse %q as bin or unary: %v / %v", inner, err, err2)
	}

	// No suffix: VarRefExpr: (varname:type) — inner is "varname:type"
	colon := strings.LastIndex(inner, ":")
	if colon >= 0 {
		vname := strings.TrimSpace(inner[:colon])
		tyStr := strings.TrimSpace(inner[colon+1:])
		// Make sure vname looks like an identifier (no spaces, no parens)
		if !strings.ContainsAny(vname, " ()[]") {
			ty := p.parseType(tyStr)
			return &VarRefExpr{Name: vname, Ty: ty}, nil
		}
	}

	// Fall back: try to parse inner as a full expression
	return p.parseExpr(inner)
}

// tryParseBinExpr tries to parse inner as "(L) op R" and returns BinExpr with ty.
// L is always parenthesized; R may be any expression (parenthesized or literal).
func (p *hirParser) tryParseBinExpr(inner string, ty mir2.Ty) (*BinExpr, error) {
	inner = strings.TrimSpace(inner)
	if !strings.HasPrefix(inner, "(") {
		return nil, fmt.Errorf("not a bin expr (no leading '(')")
	}

	// Find end of L expression (matching ')' for the leading '(')
	lClose := findCloseParen(inner[1:])
	if lClose < 0 {
		return nil, fmt.Errorf("bin expr: unmatched '('")
	}
	lClose++ // adjust for s[1:] offset
	lStr := inner[:lClose+1]                    // "(L)"
	rest := strings.TrimSpace(inner[lClose+1:]) // "op R"

	if rest == "" {
		return nil, fmt.Errorf("bin expr: no operator after L")
	}

	// Extract operator: the operator is the first "word" (may include symbols like <=, >=, ==, !=)
	// Strategy: find where the RHS expression starts.
	// The operator ends at the first space, then skip space, then R begins.
	// But operators can be multi-char (<=, >=, etc.) and have no space before R if R is '('.
	// Safest: split on the first occurrence of ' ' that separates op from the rest.
	firstSpace := strings.Index(rest, " ")
	if firstSpace < 0 {
		return nil, fmt.Errorf("bin expr: can't find operator end in %q", rest)
	}
	op := rest[:firstSpace]
	rStr := strings.TrimSpace(rest[firstSpace+1:])
	if op == "" {
		return nil, fmt.Errorf("bin expr: empty operator")
	}
	if rStr == "" {
		return nil, fmt.Errorf("bin expr: empty RHS")
	}

	l, err := p.parseExpr(lStr)
	if err != nil {
		return nil, fmt.Errorf("bin expr L: %w", err)
	}
	r, err := p.parseExpr(rStr)
	if err != nil {
		return nil, fmt.Errorf("bin expr R: %w", err)
	}
	return &BinExpr{Op: op, L: l, R: r, Ty: ty}, nil
}

// tryParseUnaryExpr tries to parse inner as "op(X)" and returns UnaryExpr with ty.
func (p *hirParser) tryParseUnaryExpr(inner string, ty mir2.Ty) (*UnaryExpr, error) {
	inner = strings.TrimSpace(inner)
	// Find the first '('
	paren := strings.Index(inner, "(")
	if paren < 0 {
		return nil, fmt.Errorf("unary expr: no '('")
	}
	op := inner[:paren]
	if op == "" {
		return nil, fmt.Errorf("unary expr: empty operator")
	}
	xStr := inner[paren:]
	x, err := p.parseExpr(xStr)
	if err != nil {
		return nil, fmt.Errorf("unary expr X: %w", err)
	}
	return &UnaryExpr{Op: op, X: x, Ty: ty}, nil
}
