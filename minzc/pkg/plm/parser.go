package plm

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseModule parses a PL/M-80 source string into a Module AST.
//
// Accepts both the standard module form:
//
//	<Name>: DO; ... END <Name>;
//
// and a bare sequence of declarations (without outer DO block).
//
// The source is preprocessed first (LITERALLY substitution) before lexing.
func ParseModule(src string) (*Module, error) {
	src = Preprocess(src)
	l, err := NewLexer(src)
	if err != nil {
		return nil, err
	}
	p := &parser{l: l}
	return p.parseModule()
}

// ParseModuleFile parses a PL/M-80 source file, resolving $INCLUDE directives
// relative to the file's directory.
func ParseModuleFile(path, src string) (*Module, error) {
	src = PreprocessFile(src, filepath.Dir(path))
	l, err := NewLexer(src)
	if err != nil {
		return nil, err
	}
	p := &parser{l: l}
	return p.parseModule()
}

type parser struct{ l *Lexer }

// ── Module ────────────────────────────────────────────────────────────────────

func (p *parser) parseModule() (*Module, error) {
	m := &Module{}

	// Skip bare numeric address labels like "100H:" (ORG-style markers).
	if p.l.IsKind(TokNumber) {
		p.l.Next()
		if p.l.IsKind(TokColon) {
			p.l.Next() // consume ':'
		}
	}

	// Speculatively detect a module header: NAME ':' DO ';'
	// We do two-token lookahead by consuming NAME then checking what follows.
	if p.l.IsKind(TokIdent) {
		nameTok := p.l.Next()
		if p.l.IsKind(TokColon) {
			p.l.Next() // consume ':'
			if p.l.Is("DO") {
				// Full module wrapper: NAME: DO; ... END NAME;
				p.l.Next() // consume DO
				if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
					return nil, err
				}
				m.Name = nameTok.Val
			} else if p.l.Is("PROCEDURE") {
				// Bare top-level procedure (no module wrapper): NAME: PROCEDURE ...
				pd, err := p.parseProcDecl(nameTok.Val)
				if err != nil {
					return nil, err
				}
				m.Decls = append(m.Decls, pd)
			} else {
				return nil, fmt.Errorf("line %d: expected DO or PROCEDURE after '%s:'", nameTok.Line, nameTok.Val)
			}
		} else if nameTok.Val == "DECLARE" {
			// Bare DECLARE at top level (no module wrapper).
			vds, err := p.parseVarDeclGroupList()
			if err != nil {
				return nil, err
			}
			for _, vd := range vds {
				m.Decls = append(m.Decls, vd)
			}
		} else {
			return nil, fmt.Errorf("line %d: unexpected %q at top level (expected module header or DECLARE)", nameTok.Line, nameTok.Val)
		}
	}

	// Parse remaining declarations until END or EOF.
	for !p.l.IsKind(TokEOF) && !p.l.Is("END") {
		ds, err := p.parseDeclList()
		if err != nil {
			return nil, err
		}
		m.Decls = append(m.Decls, ds...)
	}

	// Optional: END <name>;
	if p.l.Is("END") {
		p.l.Next()
		if p.l.IsKind(TokIdent) {
			p.l.Next() // skip module name after END
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// ── Declarations ──────────────────────────────────────────────────────────────

// parseDeclList parses one top-level declaration item and returns 1..N Decls.
// DECLARE may expand to multiple VarDecls (comma-separated groups).
func (p *parser) parseDeclList() ([]Decl, error) {
	if p.l.Is("DECLARE") {
		p.l.Next()
		vds, err := p.parseVarDeclGroupList()
		if err != nil {
			return nil, err
		}
		decls := make([]Decl, len(vds))
		for i, vd := range vds {
			decls[i] = vd
		}
		return decls, nil
	}
	if p.l.IsKind(TokIdent) {
		t2 := p.l.Peek()
		// Check for IDENT ':' pattern (label or procedure declaration).
		// We need two-token lookahead: consume ident, check for colon.
		if t2.Val != "DECLARE" { // DECLARE handled above
			nameTok := p.l.Next()
			if p.l.IsKind(TokColon) {
				p.l.Next() // consume ':'
				if p.l.Is("PROCEDURE") || p.l.Is("PROC") {
					pd, err := p.parseProcDecl(nameTok.Val)
					if err != nil {
						return nil, err
					}
					return []Decl{pd}, nil
				}
				// Module-level label before executable statements.
				// Parse and discard the following statement sequence.
				if err := p.skipModuleBodyStmts(); err != nil {
					return nil, err
				}
				return nil, nil
			}
			// Bare identifier (including keywords like CALL, IF used as statement starters).
			// Re-inject as a statement.
			if err := p.skipOneStmtWithName(nameTok); err != nil {
				return nil, err
			}
			return nil, nil
		}
	}
	// Any other token at module level — try parsing as a statement and discard.
	if !p.l.IsKind(TokEOF) && !p.l.Is("END") {
		if err := p.skipModuleBodyStmts(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	tok := p.l.Peek()
	return nil, fmt.Errorf("line %d: unexpected token %q in declaration context", tok.Line, tok.Val)
}

// skipModuleBodyStmts parses and discards executable statements at module level
// until we hit a token that looks like a new declaration (DECLARE, IDENT:PROCEDURE, END, EOF).
func (p *parser) skipModuleBodyStmts() error {
	for {
		t := p.l.Peek()
		if t.Kind == TokEOF || p.l.Is("END") {
			return nil
		}
		if p.l.Is("DECLARE") {
			// DECLARE at module-body level: skip it.
			p.l.Next()
			_, err := p.parseVarDeclGroupList()
			if err != nil {
				return err
			}
			continue
		}
		// Check for IDENT ':' PROCEDURE — start of new procedure (stop).
		if t.Kind == TokIdent {
			// Peek two ahead to check for IDENT: PROCEDURE pattern.
			// We do this by consuming the ident, checking colon, then checking PROCEDURE.
			saved := p.l.Peek()
			p.l.Next()
			if p.l.IsKind(TokColon) {
				p.l.Next()
				if p.l.Is("PROCEDURE") || p.l.Is("PROC") {
					// Looks like a new procedure — push back label for parseDeclList.
					// We can't un-consume, so synthesize parsing from here.
					pd, err := p.parseProcDecl(saved.Val)
					if err != nil {
						return err
					}
					_ = pd // discard — we're only skipping
					return nil
				}
				// Label before statement — consume statement.
				if p.l.IsKind(TokSemicolon) || p.l.Is("END") {
					continue
				}
				if err := p.skipOneStmt(); err != nil {
					return err
				}
				continue
			}
			// Ident followed by something other than colon — probably an assignment/call.
			// Re-use the consumed ident.
			if err := p.skipOneStmtWithName(saved); err != nil {
				return err
			}
			continue
		}
		if err := p.skipOneStmt(); err != nil {
			return err
		}
	}
}

// skipOneStmt parses and discards one statement.
func (p *parser) skipOneStmt() error {
	_, err := p.parseStmt()
	return err
}

// skipOneStmtWithName parses and discards one statement where the first ident is already consumed.
// If the token is a known PL/M statement keyword, it is handled appropriately.
func (p *parser) skipOneStmtWithName(name Token) error {
	// All helpers below assume the keyword in name.Val is already consumed.
	switch name.Val {
	case "CALL":
		_, err := p.parseCallStmtBody()
		return err
	case "IF":
		_, err := p.parseIfStmtBody()
		return err
	case "DO":
		_, err := p.parseDoStmtBody()
		return err
	case "RETURN":
		_, err := p.parseReturnStmtBody()
		return err
	case "ENABLE", "DISABLE", "HALT":
		_, err := p.l.ExpectKind(TokSemicolon)
		return err
	case "GO", "GOTO":
		return p.skipGoToBody(name)
	case "DECLARE":
		_, err := p.parseVarDeclGroupList()
		return err
	default:
		_, err := p.parseAssignOrCallStmtWithName(name)
		return err
	}
}

// parseVarDeclGroupList parses a comma-separated sequence of variable
// declaration groups within a single DECLARE statement.
// Each group is: [name-list] [BASED x] type [modifiers] [initializer]
// The list is terminated by ';'.
func (p *parser) parseVarDeclGroupList() ([]*VarDecl, error) {
	var result []*VarDecl
	for {
		vd, err := p.parseVarDeclAfterKeyword(p.l.Peek().Line)
		if err != nil {
			return nil, err
		}
		result = append(result, vd)
		// parseVarDeclAfterKeyword already consumed ';' if it was the last one,
		// but for comma-separated groups we need to handle ','  before ';'.
		// Actually, we restructure: parseVarDeclAfterKeyword stops at ',' or ';'
		// and returns.  The caller (this function) decides to continue or stop.
		// See the updated parseVarDeclAfterKeyword for this.
		if p.l.IsKind(TokSemicolon) {
			p.l.Next() // consume ';'
			break
		}
		if p.l.IsKind(TokComma) {
			p.l.Next() // consume ',' and continue
			continue
		}
		break
	}
	return result, nil
}

// parseVarDecl parses a DECLARE statement, consuming the DECLARE keyword first.
func (p *parser) parseVarDecl() (*VarDecl, error) {
	if err := p.l.Expect("DECLARE"); err != nil {
		return nil, err
	}
	return p.parseVarDeclAfterKeyword(p.l.Peek().Line)
}

// parseVarDeclAfterKeyword parses the rest of a DECLARE after the keyword has
// been consumed.  line is used only for error messages.
func (p *parser) parseVarDeclAfterKeyword(_ int) (*VarDecl, error) {
	// If the declaration name is a TokNumber (LITERALLY macro expanded a name to a constant),
	// skip the entire item to the next ',' or ';' and return a dummy VarDecl.
	if p.l.IsKind(TokNumber) {
		for !p.l.IsKind(TokSemicolon) && !p.l.IsKind(TokComma) && !p.l.IsKind(TokEOF) {
			if p.l.IsKind(TokLParen) {
				p.l.Next()
				p.skipBalancedParenContent()
			} else {
				p.l.Next()
			}
		}
		return &VarDecl{Names: []string{"_skip"}, Ty: PLMByte}, nil
	}

	vd := &VarDecl{}

	// Name list: either (A, B, C) or a bare name.
	// BUT: (A, B) BYTE is a name list; we must not confuse with a single-name
	// array like BUFFER(128) BYTE.  Disambiguation: after consuming '(' peek
	// ahead — if we see NUMBER ')' TYPE or IDENT ',' → it's a name list.
	// Actually simpler: if inside '(' the next token after one ident is ',' → name list.
	// If next after ident is ')' followed by TYPE → it's a single-element name list
	// (but also fine; arrays use NAME '(' SIZE ')' TYPE, which is a single bare name
	// followed by '(' NUMBER ')' before TYPE, NOT inside a multi-name '(...)').
	//
	// Rule:
	//   DECLARE (A, B, C) TYPE   → multi-name list (starts with '(' ident ',')
	//   DECLARE (A) TYPE         → single-name list (effectively DECLARE A TYPE)
	//   DECLARE A(N) TYPE        → array declaration
	//   DECLARE A BASED B TYPE   → based overlay

	if p.l.IsKind(TokLParen) {
		p.l.Next()
		for {
			t, err := p.l.ExpectKind(TokIdent)
			if err != nil {
				return nil, err
			}
			vd.Names = append(vd.Names, t.Val)
			// Optional per-name BASED in multi-name list: (s BASED a, d BASED b, l)
			if p.l.Is("BASED") {
				p.l.Next()
				if p.l.IsKind(TokIdent) {
					if vd.Based == "" {
						vd.Based = p.l.Next().Val // keep first BASED
					} else {
						p.l.Next()
					}
				}
			}
			if !p.l.IsKind(TokComma) {
				break
			}
			p.l.Next()
		}
		if _, err := p.l.ExpectKind(TokRParen); err != nil {
			return nil, err
		}
		// Optional shared array size: DECLARE (a, b, c)(N) TYPE
		if p.l.IsKind(TokLParen) {
			p.l.Next()
			n := 0
			if p.l.IsKind(TokNumber) {
				sizeTok := p.l.Next()
				sz, err := parseNumber(sizeTok.Val)
				if err == nil {
					n = int(sz)
				}
			}
			vd.Size = &n
			p.skipBalancedParenContent()
		}
	} else {
		t, err := p.l.ExpectKind(TokIdent)
		if err != nil {
			return nil, err
		}
		vd.Names = []string{t.Val}

		// Array: DECLARE name(size-expr) TYPE
		// size-expr may be a number, *, an unresolved macro name, or an expression.
		if p.l.IsKind(TokLParen) {
			p.l.Next() // consume '('
			n := 0
			if p.l.IsKind(TokNumber) {
				sizeTok := p.l.Next()
				sz, err := parseNumber(sizeTok.Val)
				if err == nil {
					n = int(sz)
				}
			}
			vd.Size = &n
			// Consume everything up to and including the matching ')'.
			p.skipBalancedParenContent()
		}
	}

	// Optional BASED <name> — pointer overlay.
	if p.l.Is("BASED") {
		p.l.Next()
		baseTok, err := p.l.ExpectKind(TokIdent)
		if err != nil {
			return nil, err
		}
		vd.Based = baseTok.Val

		// Optional array size after BASED base-name: DECLARE x BASED y (N) TYPE
		if p.l.IsKind(TokLParen) {
			p.l.Next() // consume '('
			n := 0
			if p.l.IsKind(TokNumber) {
				sizeTok := p.l.Next()
				sz, _ := parseNumber(sizeTok.Val)
				n = int(sz)
			}
			vd.Size = &n
			p.skipBalancedParenContent() // consume rest up to and including ')'
		}
	}

	// Type: BYTE / WORD / ADDRESS / STRUCTURE (STRUCTURE body is skipped for now)
	// If the next token is DATA/INITIAL/EXTERNAL/PUBLIC/AT/';'/','  — no explicit
	// type; default to BYTE (PL/M-80 allows typeless DATA declarations).
	if p.l.Is("DATA") || p.l.Is("INITIAL") || p.l.Is("AT") ||
		p.l.Is("EXTERNAL") || p.l.Is("PUBLIC") ||
		p.l.IsKind(TokSemicolon) || p.l.IsKind(TokComma) {
		vd.Ty = PLMByte // default
	} else {
		ty, err := p.parseType()
		if err != nil {
			return nil, err
		}
		vd.Ty = ty
	}

	// Optional modifiers: EXTERNAL, PUBLIC — record but otherwise ignore for now.
	for p.l.Is("EXTERNAL") || p.l.Is("PUBLIC") {
		p.l.Next()
	}

	// Optional AT (address): AT (expr)  where expr may be a number or .label
	if p.l.Is("AT") {
		p.l.Next()
		if _, err := p.l.ExpectKind(TokLParen); err != nil {
			return nil, err
		}
		// Skip the AT expression (may be .LABEL-N or a number).
		p.skipBalancedParenContent()
	}

	// Optional DATA/INITIAL (...) initializer — skip for now (not lowered).
	for p.l.Is("DATA") || p.l.Is("INITIAL") {
		p.l.Next()
		if _, err := p.l.ExpectKind(TokLParen); err != nil {
			return nil, err
		}
		p.skipBalancedParenContent()
	}

	// Caller (parseVarDeclGroupList) handles ',' and ';'.
	// Legacy single-use callers: consume ';' here if we're not in group mode.
	// We detect "group mode" by NOT consuming ';' — the caller must do it.
	// For backward compatibility with direct callers of parseVarDeclAfterKeyword
	// (proc local DECLAREs), those now call parseVarDeclGroupList too.
	return vd, nil
}

// skipBalancedParenContent skips tokens until the matching ')' is consumed.
// The opening '(' has already been consumed; this reads until depth reaches 0.
func (p *parser) skipBalancedParenContent() {
	depth := 1
	for depth > 0 && !p.l.IsKind(TokEOF) {
		tok := p.l.Next()
		if tok.Kind == TokLParen {
			depth++
		} else if tok.Kind == TokRParen {
			depth--
		}
	}
}

func (p *parser) parseType() (PLMType, error) {
	t := p.l.Peek()
	if t.Kind != TokIdent {
		return PLMVoid, fmt.Errorf("line %d: expected type (BYTE/WORD/ADDRESS), got %q", t.Line, t.Val)
	}
	switch t.Val {
	case "BYTE":
		p.l.Next()
		return PLMByte, nil
	case "WORD":
		p.l.Next()
		return PLMWord, nil
	case "ADDRESS", "ADDR":
		p.l.Next()
		return PLMAddress, nil
	case "STRUCTURE":
		// STRUCTURE (field1 TYPE, field2 TYPE, ...) — skip for now.
		p.l.Next()
		if p.l.IsKind(TokLParen) {
			p.l.Next()
			p.skipBalancedParenContent()
		}
		return PLMByte, nil // treat as BYTE placeholder
	case "LABEL":
		// LABEL is a code-pointer type (like ADDRESS).
		p.l.Next()
		return PLMAddress, nil
	default:
		return PLMVoid, fmt.Errorf("line %d: unknown type %q (expected BYTE, WORD, or ADDRESS)", t.Line, t.Val)
	}
}

func (p *parser) parseProcDecl(name string) (*ProcDecl, error) {
	// Accept both PROCEDURE and PROC (shortened form used in some PL/M dialects).
	if p.l.Is("PROCEDURE") || p.l.Is("PROC") {
		p.l.Next()
	} else {
		return nil, fmt.Errorf("line %d: expected PROCEDURE", p.l.Peek().Line)
	}
	pd := &ProcDecl{Name: name, RetTy: PLMVoid}

	// Optional parameter list: (name, name, ...)
	if p.l.IsKind(TokLParen) {
		p.l.Next()
		if !p.l.IsKind(TokRParen) {
			for {
				t, err := p.l.ExpectKind(TokIdent)
				if err != nil {
					return nil, err
				}
				pd.Params = append(pd.Params, t.Val)
				if !p.l.IsKind(TokComma) {
					break
				}
				p.l.Next()
			}
		}
		if _, err := p.l.ExpectKind(TokRParen); err != nil {
			return nil, err
		}
	}

	// Optional modifiers before return type: PUBLIC, INTERRUPT N, REENTRANT
	for p.l.Is("PUBLIC") || p.l.Is("REENTRANT") {
		p.l.Next()
	}
	if p.l.Is("INTERRUPT") {
		p.l.Next()
		if p.l.IsKind(TokNumber) {
			p.l.Next() // skip interrupt number
		}
	}

	// Optional return type: BYTE / WORD / ADDRESS
	if p.l.Is("BYTE") || p.l.Is("WORD") || p.l.Is("ADDRESS") {
		ty, err := p.parseType()
		if err != nil {
			return nil, err
		}
		pd.RetTy = ty
	}

	// Optional EXTERNAL / PUBLIC after return type.
	isExternal := false
	for p.l.Is("EXTERNAL") || p.l.Is("PUBLIC") {
		if p.l.Is("EXTERNAL") {
			isExternal = true
		}
		p.l.Next()
	}

	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}

	// EXTERNAL procedures have no body.
	if isExternal {
		return pd, nil
	}

	// Local DECLARE statements (includes param type annotations).
	for p.l.Is("DECLARE") {
		p.l.Next()
		vds, err := p.parseVarDeclGroupList()
		if err != nil {
			return nil, err
		}
		pd.Decls = append(pd.Decls, vds...)
	}

	// Body statements up to END.
	for !p.l.Is("END") && !p.l.IsKind(TokEOF) {
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		pd.Body = append(pd.Body, s)
	}

	// END <name>;
	if err := p.l.Expect("END"); err != nil {
		return nil, err
	}
	if p.l.IsKind(TokIdent) {
		endName := p.l.Next().Val
		if endName != name {
			return nil, fmt.Errorf("expected END %s, got END %s", name, endName)
		}
	}
	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}
	return pd, nil
}

// ── Statements ────────────────────────────────────────────────────────────────

func (p *parser) parseStmt() (Stmt, error) {
	t := p.l.Peek()
	// Empty / null statement: just a semicolon.
	if t.Kind == TokSemicolon {
		p.l.Next()
		return &DoBlock{}, nil // empty block as no-op
	}
	switch {
	case t.Kind == TokIdent && t.Val == "CALL":
		return p.parseCallStmt()
	case t.Kind == TokIdent && t.Val == "RETURN":
		return p.parseReturnStmt()
	case t.Kind == TokIdent && t.Val == "IF":
		return p.parseIfStmt()
	case t.Kind == TokIdent && t.Val == "DO":
		return p.parseDoStmt()
	case t.Kind == TokIdent && t.Val == "ENABLE":
		p.l.Next()
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		return &EnableStmt{}, nil
	case t.Kind == TokIdent && t.Val == "DISABLE":
		p.l.Next()
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		return &DisableStmt{}, nil
	case t.Kind == TokIdent && t.Val == "HALT":
		p.l.Next()
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		return &HaltStmt{}, nil
	case t.Kind == TokIdent && (t.Val == "GO" || t.Val == "GOTO"):
		// GO TO <label>;  or  GOTO <label>;
		p.l.Next() // consume GO or GOTO
		if t.Val == "GO" {
			if err := p.l.Expect("TO"); err != nil {
				return nil, fmt.Errorf("line %d: expected 'TO' after 'GO'", t.Line)
			}
		}
		// Label may be an ident or (after macro expansion) a number.
		var label string
		if p.l.IsKind(TokIdent) {
			label = p.l.Next().Val
		} else if p.l.IsKind(TokNumber) {
			label = "_" + p.l.Next().Val // numeric label → mangle to valid ident
		} else {
			return nil, fmt.Errorf("line %d: expected label after GO TO", t.Line)
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		return &GoToStmt{Label: label}, nil
	case t.Kind == TokIdent && t.Val == "DECLARE":
		// DECLARE inside a DO block — treat as local declaration (no-op statement).
		p.l.Next()
		_, err := p.parseVarDeclGroupList()
		if err != nil {
			return nil, err
		}
		return &DoBlock{}, nil // discard declarations; return empty block
	case t.Kind == TokIdent:
		// Check for statement label or nested procedure: IDENT ':'
		saved := p.l.Peek()
		p.l.Next() // consume ident
		if p.l.IsKind(TokColon) {
			p.l.Next() // consume ':'
			// IDENT: PROCEDURE — nested procedure declaration (PL/M-80 allows nesting).
			if p.l.Is("PROCEDURE") || p.l.Is("PROC") {
				pd, err := p.parseProcDecl(saved.Val)
				if err != nil {
					return nil, err
				}
				// Discard nested proc body — return empty block.
				_ = pd
				return &DoBlock{}, nil
			}
			// After a label, there may be a real statement or END/semicolon.
			if p.l.IsKind(TokSemicolon) || p.l.Is("END") {
				return &DoBlock{}, nil // label-only line
			}
			return p.parseStmt() // recurse for the labelled statement
		}
		// Not a label — re-use already-consumed ident.
		return p.parseAssignOrCallStmtWithName(saved)
	default:
		return nil, fmt.Errorf("line %d: unexpected token %q in statement", t.Line, t.Val)
	}
}

func (p *parser) parseCallStmt() (Stmt, error) {
	p.l.Next() // consume CALL
	name, err := p.l.ExpectKind(TokIdent)
	if err != nil {
		return nil, err
	}
	s := &CallStmt{Fn: name.Val}
	if p.l.IsKind(TokLParen) {
		p.l.Next()
		if !p.l.IsKind(TokRParen) {
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			s.Args = args
		}
		if _, err := p.l.ExpectKind(TokRParen); err != nil {
			return nil, err
		}
	}
	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}
	return s, nil
}

// parseCallStmtBody parses a CALL statement after CALL keyword is already consumed.
func (p *parser) parseCallStmtBody() (Stmt, error) {
	name, err := p.l.ExpectKind(TokIdent)
	if err != nil {
		return nil, err
	}
	s := &CallStmt{Fn: name.Val}
	if p.l.IsKind(TokLParen) {
		p.l.Next()
		if !p.l.IsKind(TokRParen) {
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			s.Args = args
		}
		if _, err := p.l.ExpectKind(TokRParen); err != nil {
			return nil, err
		}
	}
	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}
	return s, nil
}

// parseReturnStmtBody parses RETURN after the keyword is already consumed.
func (p *parser) parseReturnStmtBody() (Stmt, error) {
	s := &ReturnStmt{}
	if !p.l.IsKind(TokSemicolon) {
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		s.Val = val
	}
	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}
	return s, nil
}

// skipGoToBody parses GO TO / GOTO after the first keyword is already consumed.
func (p *parser) skipGoToBody(tok Token) error {
	if tok.Val == "GO" {
		if err := p.l.Expect("TO"); err != nil {
			return err
		}
	}
	if p.l.IsKind(TokIdent) {
		p.l.Next()
	} else if p.l.IsKind(TokNumber) {
		p.l.Next()
	}
	_, err := p.l.ExpectKind(TokSemicolon)
	return err
}

func (p *parser) parseReturnStmt() (Stmt, error) {
	p.l.Next() // consume RETURN
	s := &ReturnStmt{}
	if !p.l.IsKind(TokSemicolon) {
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		s.Val = val
	}
	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}
	return s, nil
}

func (p *parser) parseIfStmt() (Stmt, error) {
	p.l.Next() // consume IF
	return p.parseIfStmtBody()
}

func (p *parser) parseIfStmtBody() (Stmt, error) {
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("THEN"); err != nil {
		return nil, err
	}
	then, err := p.parseStmt()
	if err != nil {
		return nil, err
	}
	s := &IfStmt{Cond: cond, Then: then}
	if p.l.Is("ELSE") {
		p.l.Next()
		els, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		s.Else = els
	}
	return s, nil
}

func (p *parser) parseDoStmt() (Stmt, error) {
	p.l.Next() // consume DO
	return p.parseDoStmtBody()
}

func (p *parser) parseDoStmtBody() (Stmt, error) {
	if p.l.Is("WHILE") {
		p.l.Next() // consume WHILE
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		s := &DoWhileStmt{Cond: cond}
		for !p.l.Is("END") && !p.l.IsKind(TokEOF) {
			stmt, err := p.parseStmt()
			if err != nil {
				return nil, err
			}
			s.Body = append(s.Body, stmt)
		}
		if err := p.l.Expect("END"); err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		return s, nil
	}

	if p.l.Is("CASE") {
		p.l.Next() // consume CASE
		sel, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		s := &DoCaseStmt{Sel: sel}
		for !p.l.Is("END") && !p.l.IsKind(TokEOF) {
			stmt, err := p.parseStmt()
			if err != nil {
				return nil, err
			}
			s.Arms = append(s.Arms, stmt)
		}
		if err := p.l.Expect("END"); err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		return s, nil
	}

	// DO var = start TO end [BY step]; — counted loop.
	// Detected by: next token is IDENT followed by '='.
	if p.l.IsKind(TokIdent) {
		varTok := p.l.Next()
		if p.l.IsKind(TokEq) {
			p.l.Next() // consume '='
			start, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.l.Expect("TO"); err != nil {
				return nil, err
			}
			end, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			s := &DoToStmt{Var: varTok.Val, Start: start, End: end}
			if p.l.Is("BY") {
				p.l.Next()
				step, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				s.Step = step
			}
			if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
				return nil, err
			}
			for !p.l.Is("END") && !p.l.IsKind(TokEOF) {
				stmt, err := p.parseStmt()
				if err != nil {
					return nil, err
				}
				s.Body = append(s.Body, stmt)
			}
			if err := p.l.Expect("END"); err != nil {
				return nil, err
			}
			if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
				return nil, err
			}
			return s, nil
		}
		// Not a DO-TO: that token was consumed, we can't push it back.
		// It's an error — DO must be followed by WHILE/CASE/';'/VAR'='.
		return nil, fmt.Errorf("line %d: expected '=', WHILE, CASE or ';' after DO %s", varTok.Line, varTok.Val)
	}

	// Bare DO block: DO; stmts END;
	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}
	s := &DoBlock{}
	for !p.l.Is("END") && !p.l.IsKind(TokEOF) {
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		s.Body = append(s.Body, stmt)
	}
	if err := p.l.Expect("END"); err != nil {
		return nil, err
	}
	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}
	return s, nil
}

// parseAssignOrCallStmt handles:
//
//	NAME = expr;         (assignment)
//	NAME(args);          (call without CALL keyword)
func (p *parser) parseAssignOrCallStmt() (Stmt, error) {
	name := p.l.Next() // consume NAME
	return p.parseAssignOrCallStmtWithName(name)
}

func (p *parser) parseAssignOrCallStmtWithName(name Token) (Stmt, error) {
	if p.l.IsKind(TokEq) {
		p.l.Next() // consume '='
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		return &AssignStmt{Name: name.Val, Val: val}, nil
	}
	if p.l.IsKind(TokLParen) {
		p.l.Next()
		var args []Expr
		if !p.l.IsKind(TokRParen) {
			var err error
			args, err = p.parseArgList()
			if err != nil {
				return nil, err
			}
		}
		if _, err := p.l.ExpectKind(TokRParen); err != nil {
			return nil, err
		}
		// arr(n) = val;  or  arr(n), arr2(m) = val;  (multi-target array assignment)
		if p.l.IsKind(TokEq) || p.l.IsKind(TokComma) {
			for p.l.IsKind(TokComma) {
				p.l.Next()
				if p.l.IsKind(TokIdent) {
					p.l.Next()
				}
				if p.l.IsKind(TokLParen) {
					p.l.Next()
					p.skipBalancedParenContent()
				}
			}
			if p.l.IsKind(TokEq) {
				p.l.Next()
				val, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
					return nil, err
				}
				return &AssignStmt{Name: name.Val, Val: val}, nil
			}
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
		return &CallStmt{Fn: name.Val, Args: args}, nil
	}
	// TEMP4, I = 0;  or  A, B(n) = 0;  — multi-variable assignment
	if p.l.IsKind(TokComma) {
		for p.l.IsKind(TokComma) {
			p.l.Next()
			if p.l.IsKind(TokIdent) {
				p.l.Next() // consume extra name
			}
			if p.l.IsKind(TokLParen) {
				p.l.Next()
				p.skipBalancedParenContent()
			}
		}
		if p.l.IsKind(TokEq) {
			p.l.Next()
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
				return nil, err
			}
			return &AssignStmt{Name: name.Val, Val: val}, nil
		}
	}
	// NAME; — no-arg call (bare procedure call without parentheses)
	if p.l.IsKind(TokSemicolon) {
		p.l.Next()
		return &CallStmt{Fn: name.Val}, nil
	}
	// NAME<EOF> — bare word at end of file (PL/M-80 'eof' marker); treat as no-op.
	if p.l.IsKind(TokEOF) {
		return &DoBlock{}, nil
	}
	return nil, fmt.Errorf("line %d: expected '=' or '(' after %q", name.Line, name.Val)
}

// ── Expressions ───────────────────────────────────────────────────────────────

// Operator precedence (PL/M-80 §4.10), lowest to highest:
//   OR, XOR  →  AND  →  NOT  →  relational  →  additive  →  multiplicative  →  unary  →  primary

func (p *parser) parseArgList() ([]Expr, error) {
	var args []Expr
	for {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, e)
		if !p.l.IsKind(TokComma) {
			break
		}
		p.l.Next()
	}
	return args, nil
}

func (p *parser) parseExpr() (Expr, error) {
	e, err := p.parseOrExpr()
	if err != nil {
		return nil, err
	}
	// ALGOL-M / PL/M variant: expr := expr  (value-returning assignment)
	// Treat as RHS value (LHS side-effect ignored for parsing purposes).
	if p.l.IsKind(TokColon) {
		saved := p.l.Peek()
		p.l.Next() // consume ':'
		if p.l.IsKind(TokEq) {
			p.l.Next() // consume '='
			rhs, err := p.parseOrExpr()
			if err != nil {
				return nil, err
			}
			return rhs, nil
		}
		// Not ':=' — put ':' back by returning an error referencing it
		return nil, fmt.Errorf("line %d: unexpected token %q", saved.Line, ":")
	}
	return e, nil
}

func (p *parser) parseOrExpr() (Expr, error) {
	l, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for p.l.Is("OR") || p.l.Is("XOR") {
		op := p.l.Next().Val
		r, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		l = &BinOp{Op: op, L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseAndExpr() (Expr, error) {
	l, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}
	for p.l.Is("AND") {
		p.l.Next()
		r, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		l = &BinOp{Op: "AND", L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseNotExpr() (Expr, error) {
	if p.l.Is("NOT") {
		p.l.Next()
		x, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		return &UnOp{Op: "NOT", X: x}, nil
	}
	return p.parseRelExpr()
}

func (p *parser) parseRelExpr() (Expr, error) {
	l, err := p.parseAddExpr()
	if err != nil {
		return nil, err
	}
	var op string
	switch {
	case p.l.IsKind(TokLt):
		op = "<"
	case p.l.IsKind(TokGt):
		op = ">"
	case p.l.IsKind(TokEq):
		op = "="
	case p.l.IsKind(TokNotEq):
		op = "<>"
	case p.l.IsKind(TokLtEq):
		op = "<="
	case p.l.IsKind(TokGtEq):
		op = ">="
	}
	if op != "" {
		p.l.Next()
		r, err := p.parseAddExpr()
		if err != nil {
			return nil, err
		}
		l = &BinOp{Op: op, L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseAddExpr() (Expr, error) {
	l, err := p.parseMulExpr()
	if err != nil {
		return nil, err
	}
	for p.l.IsKind(TokPlus) || p.l.IsKind(TokMinus) || p.l.Is("PLUS") || p.l.Is("MINUS") {
		tok := p.l.Next()
		op := tok.Val
		if op == "PLUS" {
			op = "+"
		} else if op == "MINUS" {
			op = "-"
		}
		r, err := p.parseMulExpr()
		if err != nil {
			return nil, err
		}
		l = &BinOp{Op: op, L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseMulExpr() (Expr, error) {
	l, err := p.parseUnaryExpr()
	if err != nil {
		return nil, err
	}
	for p.l.IsKind(TokStar) || p.l.IsKind(TokSlash) || p.l.Is("MOD") {
		op := p.l.Next().Val
		r, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		l = &BinOp{Op: op, L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseUnaryExpr() (Expr, error) {
	// Unary minus.
	if p.l.IsKind(TokMinus) {
		p.l.Next()
		x, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		return &UnOp{Op: "-", X: x}, nil
	}
	// .HIGH. and .LOW. — written with dots in standard PL/M-80.
	// Tokenised as: TokDot  TokIdent("HIGH"|"LOW")  TokDot
	if p.l.IsKind(TokDot) {
		p.l.Next() // consume leading '.'
		kw := p.l.Peek()
		if kw.Kind == TokIdent && (kw.Val == "HIGH" || kw.Val == "LOW") {
			p.l.Next() // consume HIGH or LOW
			if _, err := p.l.ExpectKind(TokDot); err != nil {
				return nil, err
			}
			x, err := p.parseUnaryExpr()
			if err != nil {
				return nil, err
			}
			return &UnOp{Op: "." + kw.Val + ".", X: x}, nil
		}
		// .IDENT — address-of operator (like C's &).
		// .IDENT(idx) — address of array element: consume the subscript.
		if kw.Kind == TokIdent {
			p.l.Next() // consume ident
			if p.l.IsKind(TokLParen) {
				p.l.Next()
				p.skipBalancedParenContent()
			}
			return &VarRef{Name: kw.Val}, nil
		}
		// .('string') — string literal address (DATA init context); return placeholder 0
		if kw.Kind == TokLParen {
			p.l.Next()
			p.skipBalancedParenContent()
			return &NumberLit{Val: 0}, nil
		}
		// .'string' — address of string literal; return placeholder 0
		if kw.Kind == TokString {
			p.l.Next()
			return &NumberLit{Val: 0}, nil
		}
		return nil, fmt.Errorf("line %d: unexpected '.' in expression", kw.Line)
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.l.Peek()
	switch t.Kind {
	case TokNumber:
		p.l.Next()
		v, err := parseNumber(t.Val)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", t.Line, err)
		}
		// NUMBER(args) — LITERALLY macro replaced an array name with a number;
		// skip the subscript so parsing can continue.
		if p.l.IsKind(TokLParen) {
			p.l.Next()
			p.skipBalancedParenContent()
		}
		return &NumberLit{Val: uint16(v)}, nil

	case TokString:
		// Character literal: 'A' → ASCII value (single char → byte)
		p.l.Next()
		var val uint16
		if len(t.Val) == 1 {
			val = uint16(t.Val[0])
		}
		return &NumberLit{Val: val}, nil

	case TokLParen:
		p.l.Next()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokRParen); err != nil {
			return nil, err
		}
		return e, nil

	case TokIdent:
		name := p.l.Next()

		// Type coercions: BYTE(expr), WORD(expr), ADDRESS(expr)
		switch name.Val {
		case "BYTE", "WORD", "ADDRESS":
			var ty PLMType
			switch name.Val {
			case "BYTE":
				ty = PLMByte
			case "WORD":
				ty = PLMWord
			case "ADDRESS":
				ty = PLMAddress
			}
			if _, err := p.l.ExpectKind(TokLParen); err != nil {
				return nil, err
			}
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.ExpectKind(TokRParen); err != nil {
				return nil, err
			}
			return &CastExpr{Ty: ty, X: e}, nil

		// HIGH(x) / LOW(x) as identifier forms (without dots).
		case "HIGH":
			if p.l.IsKind(TokLParen) {
				p.l.Next()
				e, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				if _, err := p.l.ExpectKind(TokRParen); err != nil {
					return nil, err
				}
				return &UnOp{Op: ".HIGH.", X: e}, nil
			}
		case "LOW":
			if p.l.IsKind(TokLParen) {
				p.l.Next()
				e, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				if _, err := p.l.ExpectKind(TokRParen); err != nil {
					return nil, err
				}
				return &UnOp{Op: ".LOW.", X: e}, nil
			}
		}

		// Function call: NAME(args)
		if p.l.IsKind(TokLParen) {
			p.l.Next()
			var args []Expr
			if !p.l.IsKind(TokRParen) {
				var err error
				args, err = p.parseArgList()
				if err != nil {
					return nil, err
				}
			}
			if _, err := p.l.ExpectKind(TokRParen); err != nil {
				return nil, err
			}
			return &CallExpr{Fn: name.Val, Args: args}, nil
		}

		// Plain variable reference.
		return &VarRef{Name: name.Val}, nil

	default:
		return nil, fmt.Errorf("line %d: unexpected token %q in expression", t.Line, t.Val)
	}
}

// parseNumber parses a PL/M-80 integer literal: decimal or hex (0FFH).
func parseNumber(s string) (uint64, error) {
	s = strings.ToUpper(s)
	if strings.HasSuffix(s, "H") {
		return strconv.ParseUint(s[:len(s)-1], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}
