package abap

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Parse invokes the abaplint bridge to parse ABAP source, then converts the
// JSON AST into a semantic Program suitable for lowering to HIR.
func Parse(src, name string) (*Program, error) {
	// Locate bridge script relative to this Go source file.
	_, thisFile, _, _ := runtime.Caller(0)
	bridgeDir := filepath.Join(filepath.Dir(thisFile), "bridge")
	bridgeScript := filepath.Join(bridgeDir, "parse.mjs")

	cmd := exec.Command("node", bridgeScript, "--stdin")
	cmd.Dir = bridgeDir
	cmd.Stdin = strings.NewReader(src)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("abaplint bridge: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("abaplint bridge: %w (is Node.js installed? run: cd %s && npm install)", err, bridgeDir)
	}

	var result ParseResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("abaplint JSON: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("abaplint: %s", strings.Join(result.Errors, "; "))
	}

	return buildProgram(name, &result)
}

// buildProgram converts the raw JSON AST into a semantic Program.
func buildProgram(name string, r *ParseResult) (*Program, error) {
	prog := &Program{
		Name:   name,
		Events: make(map[string][]Stmt_),
	}

	if r.Structure != nil {
		decls, err := walkStructure(r.Structure)
		if err != nil {
			return nil, err
		}
		prog.Decls = decls
	} else {
		for _, s := range r.Statements {
			d, err := convertStmt(s)
			if err != nil {
				continue // skip unsupported statements
			}
			if d != nil {
				prog.Decls = append(prog.Decls, d)
			}
		}
	}

	// Post-process: extract PARAMETERS and events from Decls
	var filtered []Decl
	for _, d := range prog.Decls {
		switch d := d.(type) {
		case *ParamDecl:
			prog.Params = append(prog.Params, d)
			filtered = append(filtered, d) // keep as global DATA too
		case *EventDecl:
			prog.Events[d.Event] = d.Body
		default:
			filtered = append(filtered, d)
		}
	}
	prog.Decls = filtered

	return prog, nil
}

// walkStructure recursively walks a StructureNode tree.
// It handles event blocks (INITIALIZATION, START-OF-SELECTION, etc.) by
// collecting subsequent Normal blocks as the event body.
func walkStructure(sn *StructNode) ([]Decl, error) {
	var decls []Decl
	var currentEvent *EventDecl // currently open event

	for _, child := range sn.Children {
		switch {
		case child.Type == "Token":
			continue

		case isEventType(child.Type):
			// Close previous event
			if currentEvent != nil {
				decls = append(decls, currentEvent)
			}
			// Start new event
			eventName := eventTypeName(child.Type)
			currentEvent = &EventDecl{Event: eventName}

		case isControlStructure(child.Type):
			// While, If, Do, Case → convert to semantic statement
			stmt, err := convertControlStructure(child)
			if err == nil && stmt != nil {
				d := &formBodyDecl{stmt: stmt}
				if currentEvent != nil {
					currentEvent.Body = append(currentEvent.Body, stmt)
				} else {
					decls = append(decls, d)
				}
			}

		case isStructureType(child.Type):
			sub := &StructNode{Type: child.Type, Children: child.Children}
			subDecls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			if currentEvent != nil {
				for _, d := range subDecls {
					if fbd, ok := d.(*formBodyDecl); ok {
						currentEvent.Body = append(currentEvent.Body, fbd.stmt)
					}
				}
			} else {
				decls = append(decls, subDecls...)
			}

		default:
			stmt := &StmtNode{
				Type:     child.Type,
				Tokens:   child.Tokens,
				Children: child.Children,
			}
			d, err := convertStmt(stmt)
			if err != nil {
				// If inside an event, check if this is an event boundary
				if ed, ok := d.(*EventDecl); ok {
					if currentEvent != nil {
						decls = append(decls, currentEvent)
					}
					currentEvent = ed
				}
				continue
			}
			if d != nil {
				if currentEvent != nil {
					if fbd, ok := d.(*formBodyDecl); ok {
						currentEvent.Body = append(currentEvent.Body, fbd.stmt)
					} else {
						decls = append(decls, d)
					}
				} else {
					decls = append(decls, d)
				}
			}
		}
	}
	// Close last event
	if currentEvent != nil {
		decls = append(decls, currentEvent)
	}
	return decls, nil
}

func isControlStructure(t string) bool {
	return t == "While" || t == "If" || t == "Do" || t == "Case"
}

// convertControlStructure converts a structured control flow node to a semantic Stmt_.
func convertControlStructure(node *ASTChild) (Stmt_, error) {
	switch node.Type {
	case "While":
		return convertWhile(node)
	case "If":
		return convertIf(node)
	case "Do":
		return convertDo(node)
	default:
		return nil, fmt.Errorf("unsupported control structure: %s", node.Type)
	}
}

func convertWhile(node *ASTChild) (Stmt_, error) {
	var cond Expr_
	var body []Stmt_

	for _, child := range node.Children {
		switch child.Type {
		case "While":
			// The WHILE statement itself — extract condition from tokens
			tokens := collectChildTokens(child.Children)
			// tokens like: ["WHILE", "lv_i", "<=", "5", "."]
			// Skip "WHILE" and ".", parse the rest as condition
			condTokens := filterTokens(tokens, "WHILE", ".")
			cond = parseCondition(condTokens)
		case "Body":
			// Body contains the loop statements
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, _ := walkStructure(sub)
			body = collectStmtsFromDecls(decls)
		}
	}

	if cond == nil {
		cond = &IntLit{Val: 1} // fallback: infinite loop
	}
	return &WhileStmt{Cond: cond, Body: body}, nil
}

func convertIf(node *ASTChild) (Stmt_, error) {
	var cond Expr_
	var thenBody, elseBody []Stmt_

	for _, child := range node.Children {
		switch child.Type {
		case "If":
			tokens := collectChildTokens(child.Children)
			condTokens := filterTokens(tokens, "IF", ".")
			cond = parseCondition(condTokens)
		case "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, _ := walkStructure(sub)
			thenBody = collectStmtsFromDecls(decls)
		case "Else":
			sub := &StructNode{Type: "Else", Children: child.Children}
			decls, _ := walkStructure(sub)
			elseBody = collectStmtsFromDecls(decls)
		}
	}

	if cond == nil {
		cond = &IntLit{Val: 1}
	}
	return &IfStmt{Cond: cond, Then: thenBody, Else: elseBody}, nil
}

func convertDo(node *ASTChild) (Stmt_, error) {
	var times *Expr_
	var body []Stmt_

	for _, child := range node.Children {
		switch child.Type {
		case "Do":
			tokens := collectChildTokens(child.Children)
			// Look for "TIMES" keyword
			for i, t := range tokens {
				if strings.ToUpper(t) == "TIMES" && i > 0 {
					expr := parseTokenExpr(tokens[i-1])
					times = &expr
				}
			}
		case "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, _ := walkStructure(sub)
			body = collectStmtsFromDecls(decls)
		}
	}

	return &DoStmt{Times: times, Body: body}, nil
}

// parseCondition parses condition tokens into an Expr_.
// e.g. ["lv_i", "<=", "5"] → BinOp{Op:"<=", LHS:VarRef, RHS:IntLit}
func parseCondition(tokens []string) Expr_ {
	if len(tokens) == 0 {
		return &IntLit{Val: 1}
	}
	if len(tokens) == 1 {
		return parseTokenExpr(tokens[0])
	}
	// Find operator
	for i, t := range tokens {
		upper := strings.ToUpper(t)
		switch upper {
		case "=", "<>", "<", ">", "<=", ">=",
			"EQ", "NE", "LT", "GT", "LE", "GE",
			"AND", "OR":
			lhs := parseCondition(tokens[:i])
			rhs := parseCondition(tokens[i+1:])
			return &BinOp{Op: t, LHS: lhs, RHS: rhs}
		}
	}
	// No operator found — might be arithmetic like "lv_i + 1"
	for i, t := range tokens {
		if t == "+" || t == "-" || t == "*" || t == "/" {
			lhs := parseTokenExpr(tokens[0])
			if i+1 < len(tokens) {
				rhs := parseTokenExpr(tokens[i+1])
				return &BinOp{Op: t, LHS: lhs, RHS: rhs}
			}
		}
	}
	return parseTokenExpr(tokens[0])
}

func filterTokens(tokens []string, skip ...string) []string {
	skipSet := make(map[string]bool)
	for _, s := range skip {
		skipSet[strings.ToUpper(s)] = true
	}
	var out []string
	for _, t := range tokens {
		if !skipSet[strings.ToUpper(t)] {
			out = append(out, t)
		}
	}
	return out
}

func collectStmtsFromDecls(decls []Decl) []Stmt_ {
	var stmts []Stmt_
	for _, d := range decls {
		if fbd, ok := d.(*formBodyDecl); ok {
			stmts = append(stmts, fbd.stmt)
		}
	}
	return stmts
}

func isEventType(t string) bool {
	return t == "Initialization" || t == "StartOfSelection" ||
		t == "EndOfSelection" || t == "AtSelectionScreen"
}

func eventTypeName(t string) string {
	switch t {
	case "Initialization":
		return "INITIALIZATION"
	case "StartOfSelection":
		return "START-OF-SELECTION"
	case "EndOfSelection":
		return "END-OF-SELECTION"
	case "AtSelectionScreen":
		return "AT-SELECTION-SCREEN"
	default:
		return t
	}
}

func isStructureType(t string) bool {
	structTypes := map[string]bool{
		"Any": true, "Normal": true, "Body": true,
		"ClassGlobal": true, "ClassDefinition": true, "ClassImplementation": true,
		"InterfaceGlobal": true, "Interface": true,
		"PublicSection": true, "ProtectedSection": true, "PrivateSection": true,
		"Method": true, "Form": true,
		"If": true, "Else": true, "ElseIf": true,
		"Do": true, "While": true, "Loop": true,
		"Case": true, "When": true, "WhenOthers": true,
		"Try": true, "Catch": true,
	}
	return structTypes[t]
}

// convertStmt converts a single abaplint statement to a semantic Decl or wraps
// it as a statement-bearing decl. Returns nil for unsupported statements.
func convertStmt(s *StmtNode) (Decl, error) {
	tokens := collectTokenStrings(s)
	upper := strings.ToUpper(s.Type)

	switch {
	case upper == "DATA" || s.Type == "Data":
		return parseDataStmt(tokens)
	case upper == "WRITE" || s.Type == "Write":
		return parseWriteStmt(tokens)
	case upper == "PARAMETER" || upper == "PARAMETERS" || s.Type == "Parameter":
		return parseParamStmt(tokens)
	case upper == "MOVE" || s.Type == "Move":
		return parseMoveStmt(tokens)
	default:
		return nil, fmt.Errorf("unsupported: %s", s.Type)
	}
}

func collectTokenStrings(s *StmtNode) []string {
	// Collect from Tokens first
	if len(s.Tokens) > 0 {
		strs := make([]string, len(s.Tokens))
		for i, t := range s.Tokens {
			strs[i] = t.Str
		}
		return strs
	}
	// Fall back to Children
	return collectChildTokens(s.Children)
}

func collectChildTokens(children []*ASTChild) []string {
	var strs []string
	for _, c := range children {
		if c.Type == "Token" {
			strs = append(strs, c.Str)
		}
		if len(c.Tokens) > 0 {
			for _, t := range c.Tokens {
				strs = append(strs, t.Str)
			}
		}
		if len(c.Children) > 0 {
			strs = append(strs, collectChildTokens(c.Children)...)
		}
	}
	return strs
}

// parseDataStmt: DATA varname TYPE typename [VALUE val].
func parseDataStmt(tokens []string) (Decl, error) {
	if len(tokens) < 2 {
		return nil, fmt.Errorf("DATA: too few tokens")
	}
	d := &DataDecl{Name: tokens[1], AbapTy: "i"} // default to integer

	for i := 2; i < len(tokens); i++ {
		upper := strings.ToUpper(tokens[i])
		switch upper {
		case "TYPE":
			if i+1 < len(tokens) {
				d.AbapTy = strings.ToLower(tokens[i+1])
				i++
			}
		case "LENGTH":
			if i+1 < len(tokens) {
				if n, err := strconv.Atoi(tokens[i+1]); err == nil {
					d.Length = n
				}
				i++
			}
		case "VALUE":
			if i+1 < len(tokens) {
				val := parseTokenExpr(tokens[i+1])
				d.Value = &val
				i++
			}
		}
	}
	return d, nil
}

// parseWriteStmt: WRITE expr | WRITE: expr1, expr2, ...
func parseWriteStmt(tokens []string) (Decl, error) {
	if len(tokens) < 2 {
		return nil, fmt.Errorf("WRITE: too few tokens")
	}
	w := &WriteStmt{}
	// Skip "WRITE" and optional "/" (new-line) and "."
	for i := 1; i < len(tokens); i++ {
		t := tokens[i]
		if t == "/" || t == ":" || t == "," || t == "." {
			continue
		}
		w.Exprs = append(w.Exprs, parseTokenExpr(t))
	}
	// Wrap in a trivial decl that holds a statement
	return &formBodyDecl{stmt: w}, nil
}

// formBodyDecl wraps a statement as a top-level decl (for flat programs without FORM).
type formBodyDecl struct {
	stmt Stmt_
}

func (*formBodyDecl) abapDecl() {}

// parseParamStmt: PARAMETERS p_name TYPE c LENGTH 20 DEFAULT 'Hello'.
func parseParamStmt(tokens []string) (Decl, error) {
	if len(tokens) < 2 {
		return nil, fmt.Errorf("PARAMETERS: too few tokens")
	}
	p := &ParamDecl{Name: tokens[1], AbapTy: "c", Length: 1}

	for i := 2; i < len(tokens); i++ {
		upper := strings.ToUpper(tokens[i])
		switch upper {
		case "TYPE":
			if i+1 < len(tokens) {
				p.AbapTy = strings.ToLower(tokens[i+1])
				i++
			}
		case "LENGTH":
			if i+1 < len(tokens) {
				if n, err := strconv.Atoi(tokens[i+1]); err == nil {
					p.Length = n
				}
				i++
			}
		case "DEFAULT":
			if i+1 < len(tokens) {
				val := parseTokenExpr(tokens[i+1])
				p.Default = &val
				i++
			}
		}
	}
	return p, nil
}

// parseMoveStmt: target = source.  (abaplint type: "Move")
// tokens: [target, "=", source, "."]
func parseMoveStmt(tokens []string) (Decl, error) {
	if len(tokens) < 3 {
		return nil, fmt.Errorf("MOVE: too few tokens")
	}
	// Find "=" to split target and source
	eqIdx := -1
	for i, t := range tokens {
		if t == "=" {
			eqIdx = i
			break
		}
	}
	if eqIdx < 1 || eqIdx+1 >= len(tokens) {
		return nil, fmt.Errorf("MOVE: missing = in %v", tokens)
	}
	target := tokens[eqIdx-1]
	// Collect source tokens (skip "." at end)
	var srcTokens []string
	for i := eqIdx + 1; i < len(tokens); i++ {
		if tokens[i] != "." {
			srcTokens = append(srcTokens, tokens[i])
		}
	}
	if len(srcTokens) == 0 {
		return nil, fmt.Errorf("MOVE: no source value")
	}

	// Build expression: simple case = single token, complex = binary op
	var val Expr_
	if len(srcTokens) == 1 {
		val = parseTokenExpr(srcTokens[0])
	} else if len(srcTokens) == 3 {
		// a OP b
		val = &BinOp{Op: srcTokens[1], LHS: parseTokenExpr(srcTokens[0]), RHS: parseTokenExpr(srcTokens[2])}
	} else {
		val = parseTokenExpr(srcTokens[0]) // fallback: first token
	}

	return &formBodyDecl{stmt: &AssignStmt{Target: target, Val: val}}, nil
}

// parseTokenExpr converts a single token string to an Expr_.
func parseTokenExpr(s string) Expr_ {
	// SY-FIELD references
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "SY-") {
		return &SYField{Field: upper[3:]} // "SY-INDEX" → "INDEX"
	}
	// Remove surrounding quotes for strings
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return &StringLit{Val: s[1 : len(s)-1]}
	}
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return &StringLit{Val: s[1 : len(s)-1]}
	}
	// Try integer
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return &IntLit{Val: n}
	}
	// Variable reference
	return &VarRef{Name: s}
}
