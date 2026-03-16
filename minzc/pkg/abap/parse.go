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
	prog := &Program{Name: name}

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

	return prog, nil
}

// walkStructure recursively walks a StructureNode tree.
func walkStructure(sn *StructNode) ([]Decl, error) {
	var decls []Decl
	for _, child := range sn.Children {
		switch {
		case child.Type == "Token":
			continue
		case isStructureType(child.Type):
			// Recurse into structure
			sub := &StructNode{Type: child.Type, Children: child.Children}
			subDecls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			decls = append(decls, subDecls...)
		default:
			// Statement node
			stmt := &StmtNode{
				Type:     child.Type,
				Tokens:   child.Tokens,
				Children: child.Children,
			}
			d, err := convertStmt(stmt)
			if err != nil {
				continue
			}
			if d != nil {
				decls = append(decls, d)
			}
		}
	}
	return decls, nil
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

// parseTokenExpr converts a single token string to an Expr_.
func parseTokenExpr(s string) Expr_ {
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
