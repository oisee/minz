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

// ParseNode invokes the research/reference abaplint bridge to parse ABAP source, then converts the
// JSON AST into a semantic Program suitable for lowering to HIR.
func ParseNode(src, name string) (*Program, error) {
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
			return nil, fmt.Errorf("%s%w", name, err)
		}
		prog.Decls = decls
	} else {
		for _, s := range r.Statements {
			d, err := convertStmt(s)
			if err != nil {
				return nil, statementError(name, s, err)
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
	for _, d := range prog.Decls {
		if cd, ok := d.(*ClassDecl); ok {
			for _, m := range cd.Methods {
				if m.Body == nil {
					return nil, fmt.Errorf("%s: METHOD %s=>%s has no implementation", name, cd.Name, m.Name)
				}
			}
		}
	}

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

		case child.Type == "Form":
			fd, err := convertFormStructure(child)
			if err != nil {
				return nil, err
			}
			if fd != nil {
				decls = append(decls, fd)
			}
			continue

		case child.Type == "FunctionModule":
			fd, err := convertFunctionStructure(child)
			if err != nil {
				return nil, err
			}
			if fd != nil {
				decls = append(decls, fd)
			}
			continue

		case child.Type == "ClassDefinition":
			cd, err := convertClassDefinition(child)
			if err != nil {
				return nil, err
			}
			for _, d := range decls {
				if prev, ok := d.(*ClassDecl); ok && prev.Name == cd.Name {
					return nil, childError(child, fmt.Errorf("duplicate CLASS definition %s", cd.Name))
				}
			}
			decls = append(decls, cd)
			continue

		case child.Type == "ClassImplementation":
			// Find matching ClassDecl and fill in method bodies
			className, methods, err := convertClassImplementation(child)
			if err != nil {
				return nil, err
			}
			matched := false
			for _, d := range decls {
				if cd, ok := d.(*ClassDecl); ok && cd.Name == className {
					if err := mergeMethodBodies(cd, methods); err != nil {
						return nil, childError(child, err)
					}
					matched = true
				}
			}
			if !matched {
				return nil, childError(child, fmt.Errorf("CLASS IMPLEMENTATION %s has no matching definition", className))
			}
			continue

		case isEventType(child.Type):
			if child.Type == "AtSelectionScreen" {
				tokens := collectTokenStrings(&StmtNode{Tokens: child.Tokens, Children: child.Children})
				// abaplint can tokenize SELECTION-SCREEN as three tokens.
				if strings.ToUpper(strings.Join(tokens, "")) != "ATSELECTION-SCREEN." {
					return nil, childError(child, fmt.Errorf("unsupported event addition: %s", strings.Join(tokens, " ")))
				}
			}
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
			if err != nil {
				return nil, childError(child, err)
			}
			if stmt != nil {
				d := &formBodyDecl{stmt: stmt}
				if currentEvent != nil {
					currentEvent.Body = append(currentEvent.Body, stmt)
				} else {
					decls = append(decls, d)
				}
			}

		case child.Type == "Try" || child.Type == "Catch":
			return nil, childError(child, fmt.Errorf("TRY/CATCH: catchable exceptions are not implemented"))

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
					} else {
						// DATA in report events has report scope, including
						// declarations wrapped in a Body or chained DATA node.
						decls = append(decls, d)
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
				return nil, statementError("", stmt, err)
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

// convertFormStructure converts a Form structure node to a FormDecl.
// Form node children: Form (header with FormName/FormUsing/FormChanging children), Body, EndForm.
func convertFormStructure(node *ASTChild) (*FormDecl, error) {
	fd := &FormDecl{}

	for _, child := range node.Children {
		switch child.Type {
		case "Form":
			// Header — extract name from FormName child, params from FormUsing/FormChanging
			for _, sub := range child.Children {
				switch sub.Type {
				case "FormName":
					if len(sub.Tokens) > 0 {
						fd.Name = strings.ToLower(sub.Tokens[0].Str)
					}
				case "FormUsing":
					var err error
					fd.Using, err = parseReferenceFormParams(sub.Tokens)
					if err != nil {
						return nil, childError(sub, err)
					}
				case "FormChanging":
					var err error
					fd.Changing, err = parseReferenceFormParams(sub.Tokens)
					if err != nil {
						return nil, childError(sub, err)
					}
				case "Token":
					for _, tok := range sub.Tokens {
						if !strings.EqualFold(tok.Str, "FORM") && tok.Str != "." {
							return nil, childError(sub, fmt.Errorf("FORM header addition %s is not implemented", tok.Str))
						}
					}
				default:
					return nil, childError(sub, fmt.Errorf("FORM header section %s is not implemented", sub.Type))
				}
			}
			// Fallback: parse name from header tokens directly
			if fd.Name == "" {
				for _, t := range child.Tokens {
					u := strings.ToUpper(t.Str)
					if u != "FORM" && u != "USING" && u != "CHANGING" && u != "TYPE" && u != "." &&
						u != "I" && u != "C" && u != "N" && u != "P" && u != "STRING" {
						fd.Name = strings.ToLower(t.Str)
						break
					}
				}
			}
		case "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			fd.Body = collectStmtsFromDecls(decls)
		}
	}

	if fd.Name == "" {
		return nil, childError(node, fmt.Errorf("missing procedure name"))
	}
	return fd, nil
}

// convertFunctionStructure converts a FunctionModule structure to a FunctionDecl.
// Reuses FORM infrastructure — FUNCTION is essentially FORM with IMPORTING/EXPORTING.
func convertFunctionStructure(node *ASTChild) (*FunctionDecl, error) {
	fd := &FunctionDecl{}

	for _, child := range node.Children {
		switch {
		case child.Type == "FunctionModule":
			// Header — extract name and params
			for _, t := range child.Tokens {
				u := strings.ToUpper(t.Str)
				if u != "FUNCTION" && u != "." && u != "IMPORTING" && u != "EXPORTING" &&
					u != "CHANGING" && u != "TYPE" && u != "I" && u != "C" && u != "STRING" {
					if fd.Name == "" {
						// Strip quotes from 'Z_FUNC_NAME'
						name := strings.Trim(t.Str, "'\"")
						fd.Name = strings.ToLower(name)
					}
				}
			}
			// Parse sub-sections
			for _, sub := range child.Children {
				switch sub.Type {
				case "FunctionImporting":
					fd.Importing = parseFormParams(sub.Tokens)
				case "FunctionExporting":
					fd.Exporting = parseFormParams(sub.Tokens)
				case "FunctionChanging":
					fd.Changing = parseFormParams(sub.Tokens)
				}
			}
		case child.Type == "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			fd.Body = collectStmtsFromDecls(decls)
		}
	}

	if fd.Name == "" {
		return nil, childError(node, fmt.Errorf("missing procedure name"))
	}
	return fd, nil
}

// parseFormParams extracts FORM parameters from tokens like "USING pv_a TYPE i pv_b TYPE i".
func parseFormParams(tokens []*Token) []FormParam {
	var params []FormParam
	var toks []string
	for _, t := range tokens {
		toks = append(toks, t.Str)
	}

	// Skip leading USING/CHANGING keyword
	i := 0
	if i < len(toks) {
		u := strings.ToUpper(toks[i])
		if u == "USING" || u == "CHANGING" || u == "VALUE" {
			i++
		}
	}

	// Parse pairs: name TYPE type
	for i < len(toks) {
		name := strings.ToLower(toks[i])
		if strings.ToUpper(name) == "VALUE" {
			i++ // skip VALUE( wrapper
			continue
		}
		if name == "." || name == ")" {
			i++
			continue
		}
		p := FormParam{Name: name}
		i++
		if i < len(toks) && strings.ToUpper(toks[i]) == "TYPE" {
			i++
			if i < len(toks) {
				p.AbapTy = strings.ToLower(toks[i])
				i++
			}
		}
		params = append(params, p)
	}
	return params
}

// convertClassDefinition preserves only declarations represented by the ABI.
// Class additions, section members and signature tokens must never disappear.
func convertClassDefinition(node *ASTChild) (*ClassDecl, error) {
	cd := &ClassDecl{}
	names := map[string]bool{}
	for _, child := range node.Children {
		switch child.Type {
		case "ClassDefinition":
			toks := collectTokenStrings(&StmtNode{Tokens: child.Tokens, Children: child.Children})
			if len(toks) != 4 || !strings.EqualFold(toks[0], "CLASS") || !strings.EqualFold(toks[2], "DEFINITION") || toks[3] != "." {
				return nil, childError(child, fmt.Errorf("CLASS: definition additions are not implemented"))
			}
			cd.Name = strings.ToLower(toks[1])
		case "PublicSection", "ProtectedSection", "PrivateSection":
			for _, sec := range child.Children {
				switch sec.Type {
				case "Public", "Protected", "Private": // section header, no runtime effect
					continue
				case "SectionContents":
					for _, item := range sec.Children {
						if item.Type == "Comment" || item.Type == "Empty" {
							continue
						}
						toks := collectTokenStrings(&StmtNode{Tokens: item.Tokens, Children: item.Children})
						if len(toks) >= 3 && strings.EqualFold(toks[0], "CLASS") && toks[1] == "-" && strings.EqualFold(toks[2], "METHODS") {
							toks = append([]string{"CLASS-METHODS"}, toks[3:]...)
						}
						var name string
						if len(toks) >= 2 && (strings.EqualFold(toks[0], "METHODS") || strings.EqualFold(toks[0], "CLASS-METHODS")) {
							md, err := parseMethodSignature(toks)
							if err != nil {
								return nil, childError(item, err)
							}
							cd.Methods = append(cd.Methods, md)
							name = md.Name
						} else if len(toks) >= 2 && strings.EqualFold(toks[0], "DATA") {
							decl, err := parseDataStmt(toks)
							if err != nil {
								return nil, childError(item, err)
							}
							attr, ok := decl.(*DataDecl)
							if !ok {
								return nil, childError(item, fmt.Errorf("CLASS: unsupported attribute declaration"))
							}
							if attr.Value != nil {
								return nil, childError(item, fmt.Errorf("CLASS: attribute VALUE initialization is not implemented"))
							}
							cd.Attrs = append(cd.Attrs, attr)
							name = attr.Name
						} else {
							return nil, childError(item, fmt.Errorf("CLASS: unsupported section member %s", item.Type))
						}
						if names[name] {
							return nil, childError(item, fmt.Errorf("CLASS: duplicate member %s", name))
						}
						names[name] = true
					}
				case "Comment", "Empty":
				default:
					return nil, childError(sec, fmt.Errorf("CLASS: unsupported section node %s", sec.Type))
				}
			}
		case "EndClass", "Comment", "Empty":
		default:
			return nil, childError(child, fmt.Errorf("CLASS: unsupported definition node %s", child.Type))
		}
	}
	if cd.Name == "" {
		return nil, childError(node, fmt.Errorf("CLASS: missing definition name"))
	}
	return cd, nil
}

// IMPORTING scalar arguments and one VALUE RETURNING result are represented
// by the current concrete method ABI. Other sections require a distinct ABI.
func parseMethodSignature(toks []string) (*MethodDecl, error) {
	if len(toks) < 3 || toks[len(toks)-1] != "." {
		return nil, fmt.Errorf("METHODS: incomplete signature")
	}
	md := &MethodDecl{Name: strings.ToLower(toks[1]), Static: strings.EqualFold(toks[0], "CLASS-METHODS")}
	i := 2
	names := map[string]bool{}
	param := func(requireValue bool) (FormParam, error) {
		p := FormParam{}
		value := i < len(toks) && strings.EqualFold(toks[i], "VALUE")
		if requireValue && !value {
			return p, fmt.Errorf("METHODS: RETURNING requires VALUE(name) TYPE")
		}
		if value {
			i++
			if i >= len(toks) || toks[i] != "(" {
				return p, fmt.Errorf("METHODS: expected VALUE(name)")
			}
			i++
		}
		if i >= len(toks) || toks[i] == "." || toks[i] == ")" {
			return p, fmt.Errorf("METHODS: missing parameter name")
		}
		p.Name = strings.ToLower(toks[i])
		i++
		if value {
			if i >= len(toks) || toks[i] != ")" {
				return p, fmt.Errorf("METHODS: expected closing VALUE parenthesis")
			}
			i++
		}
		if i >= len(toks) || !strings.EqualFold(toks[i], "TYPE") {
			return p, fmt.Errorf("METHODS parameter %s: fully typed parameters are required", p.Name)
		}
		i++
		if i >= len(toks) || toks[i] == "." {
			return p, fmt.Errorf("METHODS parameter %s: missing type", p.Name)
		}
		p.AbapTy = strings.ToLower(toks[i])
		i++
		if names[p.Name] {
			return p, fmt.Errorf("METHODS: duplicate parameter %s", p.Name)
		}
		names[p.Name] = true
		return p, nil
	}
	importing := false
	for i < len(toks)-1 {
		switch strings.ToUpper(toks[i]) {
		case "IMPORTING":
			if importing {
				return nil, fmt.Errorf("METHODS: duplicate IMPORTING section")
			}
			importing = true
			i++
			before := len(md.Importing)
			for i < len(toks)-1 && !strings.EqualFold(toks[i], "RETURNING") {
				switch strings.ToUpper(toks[i]) {
				case "CHANGING", "EXPORTING", "RAISING", "EXCEPTIONS":
					return nil, fmt.Errorf("METHODS: %s ABI is not implemented", strings.ToUpper(toks[i]))
				}
				p, err := param(false)
				if err != nil {
					return nil, err
				}
				md.Importing = append(md.Importing, p)
			}
			if len(md.Importing) == before {
				return nil, fmt.Errorf("METHODS: empty IMPORTING section")
			}
		case "RETURNING":
			if md.Returning != nil {
				return nil, fmt.Errorf("METHODS: duplicate RETURNING section")
			}
			i++
			p, err := param(true)
			if err != nil {
				return nil, err
			}
			md.Returning = &p
		default:
			return nil, fmt.Errorf("METHODS: unsupported signature addition %s", toks[i])
		}
	}
	return md, nil
}

// convertClassImplementation extracts class name and method bodies from a ClassImplementation node.
func convertClassImplementation(node *ASTChild) (string, map[string]*MethodDecl, error) {
	var className string
	methods := make(map[string]*MethodDecl)

	for _, child := range node.Children {
		switch child.Type {
		case "ClassImplementation":
			for _, sub := range child.Children {
				if sub.Type == "ClassName" && len(sub.Tokens) > 0 {
					className = strings.ToLower(sub.Tokens[0].Str)
				}
			}
		case "Method":
			name, body, err := convertMethodImpl(child)
			if err != nil {
				return "", nil, err
			}
			if name == "" {
				return "", nil, childError(child, fmt.Errorf("METHOD: missing implementation name"))
			}
			if methods[name] != nil {
				return "", nil, childError(child, fmt.Errorf("METHOD: duplicate implementation %s", name))
			}
			methods[name] = &MethodDecl{Name: name, Body: body}
		case "EndClass", "Comment", "Empty":
		default:
			return "", nil, childError(child, fmt.Errorf("CLASS: unsupported implementation node %s", child.Type))
		}
	}
	return className, methods, nil
}

// convertMethodImpl extracts method name and body from a Method node.
func convertMethodImpl(node *ASTChild) (string, []Stmt_, error) {
	var name string
	body := make([]Stmt_, 0)

	for _, child := range node.Children {
		switch child.Type {
		case "MethodImplementation":
			// Tokens: METHOD add .
			for _, t := range child.Tokens {
				u := strings.ToUpper(t.Str)
				if u != "METHOD" && u != "." {
					name = strings.ToLower(t.Str)
				}
			}
		case "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return "", nil, err
			}
			body = append(body, collectStmtsFromDecls(decls)...)
		}
	}
	return name, body, nil
}

// mergeMethodBodies merges method implementations into a ClassDecl's method signatures.
func mergeMethodBodies(cd *ClassDecl, implMethods map[string]*MethodDecl) error {
	declared := map[string]*MethodDecl{}
	for _, md := range cd.Methods {
		declared[md.Name] = md
	}
	for name, impl := range implMethods {
		md := declared[name]
		if md == nil {
			return fmt.Errorf("METHOD %s=>%s has no matching declaration", cd.Name, name)
		}
		if md.Body != nil {
			return fmt.Errorf("METHOD %s=>%s has duplicate implementations", cd.Name, name)
		}
		md.Body = impl.Body
	}
	return nil
}

func isControlStructure(t string) bool {
	return t == "While" || t == "If" || t == "Do" || t == "Case" || t == "Select"
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
	case "Select":
		return convertSelect(node)
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
			// Extract condition from the Cond child (structured), not raw tokens
			cond = extractCondFromStmt(child)
		case "Body":
			// Body contains the loop statements
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			body = collectStmtsFromDecls(decls)
		}
	}

	if cond == nil {
		return nil, childError(node, fmt.Errorf("WHILE: missing condition"))
	}
	return &WhileStmt{Cond: cond, Body: body}, nil
}

func convertIf(node *ASTChild) (Stmt_, error) {
	var cond Expr_
	var thenBody, elseBody []Stmt_
	elseTail := &elseBody

	for _, child := range node.Children {
		switch child.Type {
		case "If":
			cond = extractCondFromStmt(child)
		case "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			thenBody = collectStmtsFromDecls(decls)
		case "ElseIf":
			if node.Type == "ElseIf" {
				cond = extractCondFromStmt(child)
				continue
			}
			nested, err := convertIf(child)
			if err != nil {
				return nil, err
			}
			*elseTail = []Stmt_{nested}
			elseTail = &nested.(*IfStmt).Else
		case "Else":
			sub := &StructNode{Type: "Else", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			*elseTail = collectStmtsFromDecls(decls)
		}
	}

	if cond == nil {
		return nil, childError(node, fmt.Errorf("IF: missing condition"))
	}
	return &IfStmt{Cond: cond, Then: thenBody, Else: elseBody}, nil
}

func convertDo(node *ASTChild) (Stmt_, error) {
	var times *Expr_
	var body []Stmt_

	for _, child := range node.Children {
		switch child.Type {
		case "Do":
			tokens := expressionTokens(child)
			if len(tokens) > 0 && strings.EqualFold(tokens[0], "DO") {
				tokens = tokens[1:]
			}
			if len(tokens) > 0 && tokens[len(tokens)-1] == "." {
				tokens = tokens[:len(tokens)-1]
			}
			if len(tokens) > 0 {
				if !strings.EqualFold(tokens[len(tokens)-1], "TIMES") {
					return nil, childError(child, fmt.Errorf("unsupported DO additions"))
				}
				expr, err := parseExpression(tokens[:len(tokens)-1])
				if err != nil {
					return nil, childError(child, err)
				}
				times = &expr
			}
		case "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			body = collectStmtsFromDecls(decls)
		}
	}

	return &DoStmt{Times: times, Body: body}, nil
}

func convertSelect(node *ASTChild) (Stmt_, error) {
	var selectTokens []string
	var body []Stmt_

	for _, child := range node.Children {
		switch child.Type {
		case "Select", "SelectLoop":
			// Use the flat header once; descendant tokens duplicate its prefix.
			selectTokens = append(selectTokens, expressionTokens(child)...)

		case "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			body = collectStmtsFromDecls(decls)
		}
	}

	if len(selectTokens) == 0 {
		return nil, fmt.Errorf("SELECT: no tokens found")
	}

	// Parse SELECT statement using the existing token parser
	decl, err := parseSelectStmt(selectTokens)
	if err != nil {
		return nil, err
	}
	if fb, ok := decl.(*formBodyDecl); ok {
		if sel, ok := fb.stmt.(*SelectStmt); ok {
			sel.Body = body
			return sel, nil
		}
	}
	return nil, fmt.Errorf("SELECT: failed to parse")
}

// extractCondFromStmt extracts a condition expression from a While/If statement node.
// It looks for Compare/Cond children with structured Source + CompareOperator,
// falling back to flat token parsing.
func extractCondFromStmt(stmtNode *ASTChild) Expr_ {
	tokens := expressionTokens(stmtNode)
	if len(tokens) > 0 {
		switch strings.ToUpper(tokens[0]) {
		case "WHILE", "IF", "ELSEIF":
			tokens = tokens[1:]
		}
	}
	if len(tokens) > 0 && tokens[len(tokens)-1] == "." {
		tokens = tokens[:len(tokens)-1]
	}
	e := parseCondition(tokens)
	if bad, ok := e.(*InvalidExpr); ok {
		bad.Message = childError(stmtNode, fmt.Errorf("%s", bad.Message)).Error()
	}
	return e
}

// A node's token list already covers its descendants. Never concatenate both:
// doing so duplicates operands in the native parser's full AST.
func expressionTokens(node *ASTChild) []string {
	if len(node.Tokens) > 0 {
		out := make([]string, len(node.Tokens))
		for i, t := range node.Tokens {
			out[i] = t.Str
		}
		return out
	}
	if len(node.Children) == 0 {
		if node.Str != "" {
			return []string{node.Str}
		}
		return nil
	}
	var out []string
	for _, child := range node.Children {
		out = append(out, expressionTokens(child)...)
	}
	return out
}

func extractCompare(node *ASTChild) Expr_ { return parseCondition(expressionTokens(node)) }
func extractSource(node *ASTChild) Expr_  { return parseCondition(expressionTokens(node)) }

// getAllTokens collects all Token.Str from a node and ALL descendants (breadth-first).
// Unlike collectLeafTokens, this includes tokens on intermediate nodes too.
func getAllTokens(node *ASTChild) []string {
	var result []string
	for _, t := range node.Tokens {
		result = append(result, t.Str)
	}
	for _, c := range node.Children {
		result = append(result, getAllTokens(c)...)
	}
	return result
}

// collectLeafTokens collects only the deepest token strings from a node tree,
// avoiding duplicates from intermediate nodes that also carry tokens.
func collectLeafTokens(node *ASTChild) []string {
	if len(node.Children) == 0 {
		// Leaf node
		var toks []string
		if node.Str != "" {
			toks = append(toks, node.Str)
		}
		for _, t := range node.Tokens {
			toks = append(toks, t.Str)
		}
		return toks
	}
	// Has children — collect from children only
	var toks []string
	for _, c := range node.Children {
		toks = append(toks, collectLeafTokens(c)...)
	}
	return toks
}

// parseCondition parses condition tokens into an Expr_.
// e.g. ["lv_i", "<=", "5"] → BinOp{Op:"<=", LHS:VarRef, RHS:IntLit}
func parseCondition(tokens []string) Expr_ {
	e, err := parseExpression(tokens)
	if err != nil {
		return &InvalidExpr{Message: err.Error()}
	}
	return e
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
		switch d := d.(type) {
		case *formBodyDecl:
			stmts = append(stmts, d.stmt)
		case *DataDecl:
			stmts = append(stmts, d)
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
	}
	return structTypes[t]
}

// convertStmt converts a single abaplint statement to a semantic Decl or wraps
// it as a statement-bearing decl. Unsupported statements are errors.
func convertStmt(s *StmtNode) (Decl, error) {
	tokens := collectTokenStrings(s)
	upper := strings.ToUpper(s.Type)

	switch {
	case upper == "REPORT":
		// Plain report identity is metadata; additions alter list/runtime behavior.
		if len(tokens) != 3 || !strings.EqualFold(tokens[0], "REPORT") || tokens[2] != "." {
			return nil, fmt.Errorf("REPORT additions are not implemented")
		}
		return nil, nil
	case upper == "COMMENT" || upper == "EMPTY" || upper == "ELSE":
		// These syntax nodes have no executable effect. ELSE bodies are walked separately.
		return nil, nil
	case upper == "DATA" || s.Type == "Data":
		return parseDataStmt(tokens)
	case upper == "WRITE" || s.Type == "Write":
		return parseWriteStmt(tokens)
	case upper == "PARAMETER" || upper == "PARAMETERS" || s.Type == "Parameter":
		return parseParamStmt(tokens)
	case upper == "RETURN" || upper == "EXIT" || upper == "CONTINUE":
		if len(tokens) != 2 || tokens[1] != "." {
			return nil, fmt.Errorf("%s: unsupported addition", upper)
		}
		var statement Stmt_
		switch upper {
		case "RETURN":
			statement = &ReturnStmt{}
		case "EXIT":
			statement = &ExitStmt{}
		case "CONTINUE":
			statement = &ContinueStmt{}
		}
		return &formBodyDecl{stmt: statement}, nil
	case upper == "CLEAR":
		if len(tokens) != 3 || tokens[2] != "." {
			return nil, fmt.Errorf("CLEAR: only a single declared scalar target is supported")
		}
		return &formBodyDecl{stmt: &ClearStmt{Target: strings.ToLower(tokens[1])}}, nil
	case upper == "PERFORM":
		return parsePerformStmt(tokens)
	case upper == "MOVE" || s.Type == "Move":
		return parseMoveStmt(tokens)
	case upper == "EXEC" || s.Type == "ExecSQL":
		return parseExecSQL(tokens)
	case upper == "SELECT" || s.Type == "Select" || s.Type == "SelectionScreen":
		if s.Type == "SelectionScreen" {
			return nil, fmt.Errorf("SELECTION-SCREEN layout and dialog statements are not implemented; PARAMETERS input is supported separately")
		}
		return parseSelectStmt(tokens)
	case upper == "ULINE":
		return nil, fmt.Errorf("ULINE: list width and horizontal-line layout are not implemented")
	case upper == "SKIP" || upper == "NEW-LINE":
		if len(tokens) != 2 || tokens[1] != "." {
			return nil, fmt.Errorf("%s: additions are not implemented", upper)
		}
		// SKIP / NEW-LINE → WRITE newline
		w := &WriteStmt{Exprs: []Expr_{&StringLit{Val: "\r\n"}}}
		return &formBodyDecl{stmt: w}, nil
	default:
		// Check for cl_salv_table=>factory( table_var ) pattern
		joined := strings.Join(tokens, " ")
		if strings.Contains(strings.ToLower(joined), "cl_salv_table") &&
			strings.Contains(strings.ToLower(joined), "factory") {
			return parseALVFactory(tokens)
		}
		return nil, fmt.Errorf("unsupported: %s", s.Type)
	}
}

// parseALVFactory: cl_salv_table=>factory( IMPORTING r_salv_table = lo_salv CHANGING t_table = lt_table ).
// Simplified: cl_salv_table=>factory( lt_table ).
func parseALVFactory(tokens []string) (Decl, error) {
	// Find the table variable — last identifier before ) or .
	var tableVar string
	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]
		if t == ")" || t == "." || t == "(" || strings.Contains(t, "=>") ||
			strings.ToLower(t) == "cl_salv_table" || strings.ToLower(t) == "factory" ||
			strings.ToUpper(t) == "IMPORTING" || strings.ToUpper(t) == "CHANGING" ||
			strings.ToUpper(t) == "EXPORTING" || t == "=" {
			continue
		}
		// Skip known keyword params like r_salv_table, lo_salv
		if strings.HasPrefix(t, "r_") || strings.HasPrefix(t, "lo_") {
			continue
		}
		tableVar = t
		break
	}
	if tableVar == "" {
		return nil, fmt.Errorf("cl_salv_table: could not find table variable")
	}
	return &formBodyDecl{stmt: &ALVDisplayStmt{TableVar: tableVar}}, nil
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
	name, ty, length, value, err := parseSimpleDeclaration(tokens, "VALUE")
	if err != nil {
		return nil, err
	}
	return &DataDecl{Name: name, AbapTy: ty, Length: length, Value: value}, nil
}

// parseExecSQL: EXEC SQL. raw_sql ENDEXEC.
// Collects tokens between EXEC SQL and ENDEXEC as raw SQL for sqlite_exec.
func parseExecSQL(tokens []string) (Decl, error) {
	// Tokens: EXEC SQL <sql_tokens...> ENDEXEC
	// Skip "EXEC" and "SQL", collect until "ENDEXEC" or "."
	var sqlParts []string
	started := false
	for _, t := range tokens {
		upper := strings.ToUpper(t)
		if upper == "EXEC" || upper == "SQL" {
			started = true
			continue
		}
		if upper == "ENDEXEC" || (started && t == ".") {
			break
		}
		if started {
			sqlParts = append(sqlParts, t)
		}
	}
	if len(sqlParts) == 0 {
		return nil, nil // empty EXEC SQL
	}
	sql := strings.Join(sqlParts, " ")
	return &formBodyDecl{stmt: &ExecSQLStmt{SQL: sql}}, nil
}

// parseSelectStmt: SELECT [SINGLE] f1 f2 ... FROM table [INTO (v1,v2,...)] [WHERE cond].
// Transpiles Open SQL to SQLite query calls.
func parseSelectStmt(tokens []string) (Decl, error) {
	if len(tokens) < 4 {
		return nil, fmt.Errorf("SELECT: too few tokens")
	}

	if err := validateSelectTokenAdmission(tokens); err != nil {
		return nil, err
	}
	sel := &SelectStmt{}
	i := 1 // skip SELECT

	// Optional SINGLE
	if strings.ToUpper(tokens[i]) == "SINGLE" {
		sel.Single = true
		i++
	}

	// Collect field names until FROM, aggregating function calls like COUNT(*)
	for i < len(tokens) {
		upper := strings.ToUpper(tokens[i])
		if upper == "FROM" {
			break
		}
		if tokens[i] == "," || tokens[i] == "." {
			i++
			continue
		}
		// Check if next token is "(" — aggregate into function call
		if i+1 < len(tokens) && tokens[i+1] == "(" {
			// Collect fn(args) as a single field
			fn := tokens[i]
			i++ // skip fn name
			depth := 0
			for i < len(tokens) {
				fn += tokens[i]
				if tokens[i] == "(" {
					depth++
				}
				if tokens[i] == ")" {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				i++
			}
			sel.Fields = append(sel.Fields, fn)
		} else {
			sel.Fields = append(sel.Fields, tokens[i])
			i++
		}
	}

	// FROM table
	if i < len(tokens) && strings.ToUpper(tokens[i]) == "FROM" {
		i++
		if i < len(tokens) {
			sel.Table = tokens[i]
			i++
		}
	}

	// INNER JOIN / LEFT [OUTER] JOIN
	for i < len(tokens) {
		upper := strings.ToUpper(tokens[i])
		joinType := ""
		if upper == "INNER" && i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "JOIN" {
			joinType = "INNER"
			i += 2
		} else if upper == "LEFT" {
			joinType = "LEFT"
			i++ // skip LEFT
			if i < len(tokens) && strings.ToUpper(tokens[i]) == "OUTER" {
				i++ // skip OUTER
			}
			if i < len(tokens) && strings.ToUpper(tokens[i]) == "JOIN" {
				i++ // skip JOIN
			}
		} else if upper == "JOIN" {
			joinType = "INNER"
			i++
		} else {
			break
		}
		jc := JoinClause{Type: joinType}
		if i < len(tokens) {
			jc.Table = tokens[i]
			i++
		}
		// ON condition — collect until WHERE / INTO / ORDER / . / next JOIN
		if i < len(tokens) && strings.ToUpper(tokens[i]) == "ON" {
			i++
			var onParts []string
			for i < len(tokens) {
				tu := strings.ToUpper(tokens[i])
				if tu == "WHERE" || tu == "INTO" || tu == "ORDER" || tu == "INNER" ||
					tu == "LEFT" || tu == "JOIN" || tokens[i] == "." {
					break
				}
				// Convert table~field to table.field for SQLite
				onParts = append(onParts, strings.ReplaceAll(tokens[i], "~", "."))
				i++
			}
			jc.On = strings.Join(onParts, " ")
		}
		sel.Joins = append(sel.Joins, jc)
	}

	// WHERE and INTO can appear in either order in ABAP Open SQL.
	// Unified loop handles: INTO TABLE ... WHERE ... or WHERE ... INTO TABLE ...
	for i < len(tokens) && tokens[i] != "." {
		upper := strings.ToUpper(tokens[i])

		if upper == "WHERE" && sel.Where == "" {
			i++
			var whereParts []string
			for i < len(tokens) {
				t := tokens[i]
				tu := strings.ToUpper(t)
				if t == "." || tu == "ORDER" || tu == "ENDSELECT" || tu == "INTO" {
					break
				}
				switch tu {
				case "EQ":
					whereParts = append(whereParts, "=")
				case "NE":
					whereParts = append(whereParts, "!=")
				case "LT":
					whereParts = append(whereParts, "<")
				case "GT":
					whereParts = append(whereParts, ">")
				case "LE":
					whereParts = append(whereParts, "<=")
				case "GE":
					whereParts = append(whereParts, ">=")
				default:
					whereParts = append(whereParts, t)
				}
				i++
			}
			sel.Where = strings.Join(whereParts, " ")
			continue
		}

		if upper == "INTO" && sel.IntoTable == "" && len(sel.Into) == 0 {
			i++
			if i < len(tokens) && strings.ToUpper(tokens[i]) == "TABLE" {
				i++ // skip TABLE
				if i < len(tokens) {
					varName := tokens[i]
					if varName == "@" {
						sel.IntoTableInline = true
						i++
						if i < len(tokens) && strings.ToUpper(tokens[i]) == "DATA" {
							i++
						}
						if i < len(tokens) && tokens[i] == "(" {
							i++
						}
						if i < len(tokens) && tokens[i] != ")" && tokens[i] != "." {
							sel.IntoTable = tokens[i]
							i++
						}
						if i < len(tokens) && tokens[i] == ")" {
							i++
						}
					} else if strings.HasPrefix(varName, "@") {
						inner := varName
						if strings.HasPrefix(strings.ToUpper(inner), "@DATA(") {
							inner = inner[6:]
						} else {
							inner = inner[1:]
						}
						inner = strings.TrimSuffix(inner, ")")
						sel.IntoTable = inner
						sel.IntoTableInline = true
						i++
					} else {
						sel.IntoTable = varName
						i++
					}
				}
			} else {
				// INTO (v1, v2, ...) or INTO v1
				if i < len(tokens) && tokens[i] == "(" {
					i++
				}
				for i < len(tokens) {
					t := tokens[i]
					tu := strings.ToUpper(t)
					if tu == "WHERE" || tu == "ORDER" || t == "." || t == ")" {
						if t == ")" {
							i++
						}
						break
					}
					if t != "," && t != "@" {
						sel.Into = append(sel.Into, strings.ToLower(strings.TrimPrefix(t, "@")))
					}
					i++
				}
			}
			continue
		}

		return nil, fmt.Errorf("SELECT: unsupported clause or token %q", tokens[i])
	}

	// Default: no INTO → use field names as target variables
	if sel.IntoTable == "" && len(sel.Into) == 0 {
		for _, f := range sel.Fields {
			if f != "*" {
				sel.Into = append(sel.Into, f)
			}
		}
	}

	return &formBodyDecl{stmt: sel}, nil
}

// formBodyDecl wraps a statement as a top-level decl (for flat programs without FORM).
type formBodyDecl struct {
	stmt Stmt_
}

func (*formBodyDecl) abapDecl() {}

// parseParamStmt: PARAMETERS p_name TYPE c LENGTH 20 DEFAULT 'Hello'.
func parseParamStmt(tokens []string) (Decl, error) {
	name, ty, length, value, err := parseSimpleDeclaration(tokens, "DEFAULT")
	if err != nil {
		return nil, err
	}
	return &ParamDecl{Name: name, AbapTy: ty, Length: length, Default: value}, nil
}

// Only consume additions whose meaning is represented in the semantic AST.
// Unspecified elementary DATA/PARAMETERS types are c of length one in ABAP.
func parseSimpleDeclaration(tokens []string, initializer string) (name, ty string, length int, value *Expr_, err error) {
	if len(tokens) < 2 {
		err = fmt.Errorf("declaration: too few tokens")
		return
	}
	name, ty = tokens[1], "c"
	end := len(tokens)
	if tokens[end-1] == "." || tokens[end-1] == "," {
		end--
	}
	seen := map[string]bool{}
	for i := 2; i < end; {
		key := strings.ToUpper(tokens[i])
		if seen[key] {
			err = fmt.Errorf("%s %s: duplicate %s", tokens[0], name, key)
			return
		}
		seen[key] = true
		switch key {
		case "TYPE":
			if i+1 >= end {
				err = fmt.Errorf("%s %s: missing TYPE", tokens[0], name)
				return
			}
			ty = strings.ToLower(tokens[i+1])
			i += 2
		case "LENGTH":
			if i+1 >= end {
				err = fmt.Errorf("%s %s: missing LENGTH", tokens[0], name)
				return
			}
			length, err = strconv.Atoi(tokens[i+1])
			if err != nil || length < 1 {
				err = fmt.Errorf("%s %s: LENGTH must be a positive integer literal", tokens[0], name)
				return
			}
			i += 2
		case initializer:
			if initializer == "VALUE" && i+3 == end && strings.EqualFold(tokens[i+1], "IS") && strings.EqualFold(tokens[i+2], "INITIAL") {
				i = end
				continue
			}
			var v Expr_
			v, err = parseDeclarationLiteral(tokens[i+1 : end])
			if err != nil {
				err = fmt.Errorf("%s %s %s: %w", tokens[0], name, initializer, err)
				return
			}
			value = &v
			i = end
		default:
			err = fmt.Errorf("%s %s: unsupported declaration addition %s", tokens[0], name, tokens[i])
			return
		}
	}
	if seen["LENGTH"] && ty != "c" && ty != "n" && ty != "x" {
		err = fmt.Errorf("%s %s: LENGTH is not admitted for TYPE %s", tokens[0], name, ty)
	}
	return
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

	if eqIdx != 1 {
		return nil, fmt.Errorf("MOVE: unsupported target %v", tokens[:eqIdx])
	}
	val, err := parseExpression(srcTokens)
	if err != nil {
		return nil, fmt.Errorf("MOVE: %w", err)
	}

	return &formBodyDecl{stmt: &AssignStmt{Target: target, Val: val}}, nil
}

// parseTokenExpr converts a single token string to an Expr_.
func parseTokenExpr(s string) Expr_ {
	e, err := expressionAtom(s)
	if err != nil {
		return &InvalidExpr{Message: err.Error()}
	}
	return e
}

// statementError keeps admission failures tied to the offending source token.
func statementError(name string, s *StmtNode, err error) error {
	if len(s.Tokens) > 0 {
		t := s.Tokens[0]
		return fmt.Errorf("%s:%d:%d: %s: %w", name, t.Row, t.Col, s.Type, err)
	}
	return fmt.Errorf("%s: %s: %w", name, s.Type, err)
}
func childError(c *ASTChild, err error) error {
	return statementError("", &StmtNode{Type: c.Type, Tokens: c.Tokens}, err)
}

func parsePerformStmt(tokens []string) (Decl, error) {
	if len(tokens) < 3 || tokens[len(tokens)-1] != "." {
		return nil, fmt.Errorf("PERFORM: malformed call")
	}
	p := &PerformStmt{FormName: strings.ToLower(tokens[1])}
	var section string
	for _, t := range tokens[2 : len(tokens)-1] {
		switch strings.ToUpper(t) {
		case "USING", "CHANGING":
			section = strings.ToUpper(t)
			continue
		}
		if section == "" {
			return nil, fmt.Errorf("PERFORM: unsupported addition %s", t)
		}
		e := parseTokenExpr(t)
		if _, ok := e.(*VarRef); !ok && section == "CHANGING" {
			return nil, fmt.Errorf("PERFORM: only variable arguments are admitted, got %s", t)
		}
		if section == "USING" {
			p.Using = append(p.Using, e)
		} else {
			p.Changing = append(p.Changing, e)
		}
	}
	return &formBodyDecl{stmt: p}, nil
}

// parseReferenceFormParams admits fully typed, reference-mode FORM parameters.
// VALUE/LIKE are rejected explicitly until their distinct semantics are implemented.
func parseReferenceFormParams(tokens []*Token) ([]FormParam, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	i := 1
	params := []FormParam{}
	for i < len(tokens) {
		if tokens[i].Str == "." {
			if i != len(tokens)-1 {
				return nil, fmt.Errorf("FORM: tokens after period")
			}
			break
		}
		if i+1 < len(tokens) && strings.EqualFold(tokens[i].Str, "VALUE") && tokens[i+1].Str == "(" {
			return nil, fmt.Errorf("FORM VALUE copy semantics are not yet admitted")
		}
		if i+2 >= len(tokens) || !strings.EqualFold(tokens[i+1].Str, "TYPE") {
			return nil, fmt.Errorf("FORM: parameter %s needs an explicit supported TYPE", tokens[i].Str)
		}
		params = append(params, FormParam{Name: strings.ToLower(tokens[i].Str), AbapTy: strings.ToLower(tokens[i+2].Str)})
		i += 3
	}
	return params, nil
}
