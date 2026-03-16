// Package lizp implements a Lisp dialect frontend for MinZ.
//
// Lizp is a thin syntactic sugar layer over Lanz (the S-expression HIR frontend).
// It adds familiar Lisp forms: defun, let, cond, setq, progn, dotimes, defstruct,
// while keeping Lanz's typed semantics underneath.
//
// Syntax:
//
//	(defun name ((param type) ...) -> rettype body...)    → function
//	(let ((name type val) ...) body...)                   → scoped variables
//	(cond (test1 expr1) (test2 expr2) ... (t default))    → chained if/else
//	(setq name val)                                       → assignment
//	(progn body...)                                       → block
//	(dotimes (var count) body...)                          → for range 0..count
//	(defstruct name (field1 type1) (field2 type2) ...)    → struct
//	(defglobal name type [init])                          → global variable
//	(defextern name ((param type) ...) rettype [addr])    → extern function
//	(assert expr1 op expr2)                               → compile-time assert
//
// Standard Lanz forms are also accepted (pass-through):
//
//	(+ a b), (- a b), (* a b), (/ a b), etc.
//	(if cond then else)
//	(while cond body...)
//	(call fn args...) or (fn args...)
//	(return expr)
//	(cast expr type)
//	(index base idx)
//	(addr sym)
//	(load ptr)
//	(store ptr val)
//
// Pipeline: Lizp source → desugar → Lanz S-expressions → lanz.Compile → *hir.Module
package lizp

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
)

// macros holds user-defined macros: name → (params, body template).
type macro struct {
	params []string   // parameter names
	body   lanz.Node  // template body (params substituted on expansion)
}

// Compile parses Lizp source and returns a HIR module.
func Compile(src, name string) (*hir.Module, error) {
	nodes, err := lanz.ParseSExpr(src)
	if err != nil {
		return nil, fmt.Errorf("lizp parse: %w", err)
	}
	macros := map[string]*macro{}
	desugared := make([]lanz.Node, 0, len(nodes))
	for _, n := range nodes {
		ds, err := desugar(n, macros)
		if err != nil {
			return nil, fmt.Errorf("lizp desugar: %w", err)
		}
		desugared = append(desugared, ds...)
	}
	// Pass desugared nodes directly to Lanz compiler (no serialize roundtrip).
	return lanz.CompileNodes(desugared, name)
}

// desugar transforms a Lizp top-level form into one or more Lanz forms.
func desugar(n lanz.Node, macros map[string]*macro) ([]lanz.Node, error) {
	if n.IsAtom() {
		return []lanz.Node{n}, nil
	}
	if len(n.List) == 0 {
		return nil, fmt.Errorf("line %d: empty form", n.Line)
	}
	head := n.List[0]
	if !head.IsAtom() {
		return []lanz.Node{n}, nil
	}

	switch head.Atom {
	case "defmacro":
		return desugarDefmacro(n, macros)
	case "defun":
		return desugarDefun(n)
	case "defstruct":
		return desugarDefstruct(n)
	case "defglobal":
		return desugarDefglobal(n)
	case "defextern":
		return desugarDefextern(n)
	case "assert":
		// (assert fn arg1 arg2 == expected) — desugar #x hex literals, pass to Lanz
		elems := make([]lanz.Node, len(n.List))
		elems[0] = n.List[0]
		for i := 1; i < len(n.List); i++ {
			elems[i] = desugarExpr(n.List[i])
		}
		return []lanz.Node{{List: elems, Line: n.Line}}, nil
	default:
		// Check if it's a macro invocation
		if m, ok := macros[head.Atom]; ok {
			expanded := expandMacro(m, n)
			return desugar(expanded, macros)
		}
		// Pass through to Lanz as-is (fun, global, struct, etc.)
		return []lanz.Node{n}, nil
	}
}

// (defun name ((param type) ...) -> rettype body...)
// → (fun name ((param type) ...) rettype body'...)
func desugarDefun(n lanz.Node) ([]lanz.Node, error) {
	// Minimum: (defun name params -> rettype body)
	if len(n.List) < 5 {
		return nil, fmt.Errorf("line %d: defun: expected (defun name params -> rettype body...)", n.Line)
	}
	name := n.List[1]
	params := n.List[2]

	// Find -> arrow to locate return type
	arrowIdx := -1
	for i := 3; i < len(n.List); i++ {
		if n.List[i].IsAtom() && n.List[i].Atom == "->" {
			arrowIdx = i
			break
		}
	}
	if arrowIdx < 0 || arrowIdx+1 >= len(n.List) {
		return nil, fmt.Errorf("line %d: defun %s: missing -> rettype", n.Line, name.Atom)
	}
	retType := n.List[arrowIdx+1]
	bodyStart := arrowIdx + 2

	// Desugar body statements
	var body []lanz.Node
	for _, s := range n.List[bodyStart:] {
		ds := desugarExpr(s)
		body = append(body, ds)
	}

	// Build (fun name params rettype body...)
	result := lanz.Node{
		List: append([]lanz.Node{
			atom("fun", n.Line),
			name,
			params,
			retType,
		}, body...),
		Line: n.Line,
	}
	return []lanz.Node{result}, nil
}

// (defstruct name (field1 type1) (field2 type2) ...)
// → (struct name ((field1 type1) (field2 type2) ...))
func desugarDefstruct(n lanz.Node) ([]lanz.Node, error) {
	if len(n.List) < 3 {
		return nil, fmt.Errorf("line %d: defstruct: expected (defstruct name fields...)", n.Line)
	}
	name := n.List[1]
	fields := lanz.Node{List: n.List[2:], Line: n.Line}
	result := lanz.Node{
		List: []lanz.Node{atom("struct", n.Line), name, fields},
		Line: n.Line,
	}
	return []lanz.Node{result}, nil
}

// (defglobal name type [init])
// → (global name type [init])
func desugarDefglobal(n lanz.Node) ([]lanz.Node, error) {
	if len(n.List) < 3 {
		return nil, fmt.Errorf("line %d: defglobal: expected (defglobal name type [init])", n.Line)
	}
	result := lanz.Node{
		List: append([]lanz.Node{atom("global", n.Line)}, n.List[1:]...),
		Line: n.Line,
	}
	return []lanz.Node{result}, nil
}

// (defextern name ((param type) ...) rettype [addr])
// → (extern name ((param type) ...) rettype [addr])
func desugarDefextern(n lanz.Node) ([]lanz.Node, error) {
	if len(n.List) < 4 {
		return nil, fmt.Errorf("line %d: defextern: expected (defextern name params rettype [addr])", n.Line)
	}
	elems := make([]lanz.Node, len(n.List))
	elems[0] = atom("extern", n.Line)
	copy(elems[1:], n.List[1:])
	// Desugar address token (#x0005 → 0x0005) so strconv.ParseUint recognizes it.
	if len(elems) >= 5 {
		elems[4] = desugarExpr(elems[4])
	}
	result := lanz.Node{List: elems, Line: n.Line}
	return []lanz.Node{result}, nil
}

// desugarExpr transforms Lizp expression sugar into Lanz forms.
func desugarExpr(n lanz.Node) lanz.Node {
	if n.IsAtom() {
		// Transform #xFF → 0xFF (Lisp hex syntax)
		if strings.HasPrefix(n.Atom, "#x") || strings.HasPrefix(n.Atom, "#X") {
			return lanz.Node{Atom: "0x" + n.Atom[2:], Line: n.Line}
		}
		// Transform #b1010 → 0b1010 (Lisp binary syntax)
		if strings.HasPrefix(n.Atom, "#b") || strings.HasPrefix(n.Atom, "#B") {
			return lanz.Node{Atom: "0b" + n.Atom[2:], Line: n.Line}
		}
		// Transform #'name → (addr name) — function reference (Common Lisp style)
		if strings.HasPrefix(n.Atom, "#'") {
			fname := n.Atom[2:]
			return list(n.Line, atom("addr", n.Line), atom(fname, n.Line))
		}
		// Transform t → true (Lisp convention)
		if n.Atom == "t" {
			return lanz.Node{Atom: "true", Line: n.Line}
		}
		if n.Atom == "nil" {
			return lanz.Node{Atom: "0", Line: n.Line}
		}
		return n
	}
	if len(n.List) == 0 {
		return n
	}
	head := n.List[0]
	if !head.IsAtom() {
		return desugarList(n)
	}

	switch head.Atom {
	case "let":
		return desugarLet(n)
	case "cond":
		return desugarCond(n)
	case "setq", "setf":
		return desugarSetq(n)
	case "progn", "begin":
		return desugarProgn(n)
	case "dotimes":
		return desugarDotimes(n)
	case "when":
		return desugarWhen(n)
	case "unless":
		return desugarUnless(n)
	case "1+":
		return desugarIncr(n, "+")
	case "1-":
		return desugarIncr(n, "-")
	case "=":
		// Lisp = → Lanz ==
		return desugarBinOp(n, "==")
	case "/=":
		// Lisp /= → Lanz !=
		return desugarBinOp(n, "!=")
	case "and":
		// (and a b) → (&  a b) — bitwise for Z80 (no short-circuit)
		return desugarBinOp(n, "&")
	case "or":
		// (or a b) → (| a b)
		return desugarBinOp(n, "|")
	case "logxor", "xor":
		return desugarBinOp(n, "^")
	case "ash":
		// (ash x n) → (<< x n) if n>0, (>> x (neg n)) if n<0
		// Simplified: just pass through as << for now
		return desugarBinOp(n, "<<")
	case "zerop":
		// (zerop x) → (== x 0)
		if len(n.List) == 2 {
			x := desugarExpr(n.List[1])
			return list(n.Line, atom("==", n.Line), x, atom("0", n.Line))
		}
		return n
	case "->":
		// Thread-first: (-> x (f a) (g b)) → (g (f x a) b)
		return desugarThreadFirst(n)
	case "->>":
		// Thread-last: (->> x (f a) (g b)) → (g a (f b x))
		return desugarThreadLast(n)
	default:
		// Recursively desugar children
		return desugarList(n)
	}
}

// (let ((name type val) (name2 type2 val2)) body...)
// → (block (var name type val) (var name2 type2 val2) body...)
func desugarLet(n lanz.Node) lanz.Node {
	if len(n.List) < 3 || !n.List[1].IsList() {
		return n
	}
	bindings := n.List[1]
	body := n.List[2:]

	var stmts []lanz.Node
	stmts = append(stmts, atom("block", n.Line))
	for _, b := range bindings.List {
		if !b.IsList() || len(b.List) < 3 {
			continue
		}
		// (name type val) → (var name type val)
		varForm := lanz.Node{
			List: append([]lanz.Node{atom("var", b.Line)}, desugarNodes(b.List)...),
			Line: b.Line,
		}
		stmts = append(stmts, varForm)
	}
	for _, s := range body {
		stmts = append(stmts, desugarExpr(s))
	}
	return lanz.Node{List: stmts, Line: n.Line}
}

// (cond (test1 expr1) (test2 expr2) ... (t default))
// → (if test1 expr1 (if test2 expr2 ... default))
func desugarCond(n lanz.Node) lanz.Node {
	clauses := n.List[1:]
	return buildCondChain(clauses, n.Line)
}

func buildCondChain(clauses []lanz.Node, line int) lanz.Node {
	if len(clauses) == 0 {
		return atom("0", line) // default: return 0
	}
	clause := clauses[0]
	if !clause.IsList() || len(clause.List) < 2 {
		return atom("0", line)
	}
	test := desugarExpr(clause.List[0])
	body := desugarExpr(clause.List[1])

	// (t expr) → unconditional (default clause)
	if test.IsAtom() && (test.Atom == "true" || test.Atom == "t") {
		return body
	}

	rest := buildCondChain(clauses[1:], line)
	return list(line, atom("if", line), test, body, rest)
}

// (setq name val) → (set name val)
func desugarSetq(n lanz.Node) lanz.Node {
	if len(n.List) != 3 {
		return n
	}
	return list(n.Line, atom("set", n.Line), desugarExpr(n.List[1]), desugarExpr(n.List[2]))
}

// (progn body...) → (block body...)
func desugarProgn(n lanz.Node) lanz.Node {
	stmts := []lanz.Node{atom("block", n.Line)}
	for _, s := range n.List[1:] {
		stmts = append(stmts, desugarExpr(s))
	}
	return lanz.Node{List: stmts, Line: n.Line}
}

// (dotimes (var count) body...) → (for var 0 count body...)
func desugarDotimes(n lanz.Node) lanz.Node {
	if len(n.List) < 3 || !n.List[1].IsList() || len(n.List[1].List) < 2 {
		return n
	}
	binding := n.List[1]
	varName := binding.List[0]
	count := desugarExpr(binding.List[1])

	stmts := []lanz.Node{atom("for", n.Line), varName, atom("0", n.Line), count}
	for _, s := range n.List[2:] {
		stmts = append(stmts, desugarExpr(s))
	}
	return lanz.Node{List: stmts, Line: n.Line}
}

// (when test body...) → (if test (block body...))
func desugarWhen(n lanz.Node) lanz.Node {
	if len(n.List) < 3 {
		return n
	}
	test := desugarExpr(n.List[1])
	body := desugarProgn(lanz.Node{List: append([]lanz.Node{atom("progn", n.Line)}, n.List[2:]...), Line: n.Line})
	return list(n.Line, atom("if", n.Line), test, body)
}

// (unless test body...) → (if (not test) (block body...))
func desugarUnless(n lanz.Node) lanz.Node {
	if len(n.List) < 3 {
		return n
	}
	test := desugarExpr(n.List[1])
	notTest := list(n.Line, atom("not", n.Line), test)
	body := desugarProgn(lanz.Node{List: append([]lanz.Node{atom("progn", n.Line)}, n.List[2:]...), Line: n.Line})
	return list(n.Line, atom("if", n.Line), notTest, body)
}

// (1+ x) → (+ x 1)
func desugarIncr(n lanz.Node, op string) lanz.Node {
	if len(n.List) != 2 {
		return n
	}
	return list(n.Line, atom(op, n.Line), desugarExpr(n.List[1]), atom("1", n.Line))
}

// Rename operator: (= a b) → (== a b), (/= a b) → (!= a b), etc.
func desugarBinOp(n lanz.Node, newOp string) lanz.Node {
	if len(n.List) != 3 {
		return n
	}
	return list(n.Line, atom(newOp, n.Line), desugarExpr(n.List[1]), desugarExpr(n.List[2]))
}

// Recursively desugar all children of a list node.
func desugarList(n lanz.Node) lanz.Node {
	result := make([]lanz.Node, len(n.List))
	for i, child := range n.List {
		result[i] = desugarExpr(child)
	}
	return lanz.Node{List: result, Line: n.Line}
}

func desugarNodes(nodes []lanz.Node) []lanz.Node {
	result := make([]lanz.Node, len(nodes))
	for i, n := range nodes {
		result[i] = desugarExpr(n)
	}
	return result
}

// ── Threading macros ─────────────────────────────────────────────────────────

// (-> x (f a) (g b)) → (g (f x a) b)
// Thread x as FIRST arg of each successive form.
func desugarThreadFirst(n lanz.Node) lanz.Node {
	if len(n.List) < 2 {
		return n
	}
	acc := desugarExpr(n.List[1])
	for _, form := range n.List[2:] {
		if form.IsAtom() {
			// (-> x f) → (f x)
			acc = list(n.Line, form, acc)
		} else {
			// (-> x (f a b)) → (f x a b)
			children := make([]lanz.Node, 0, len(form.List)+1)
			children = append(children, form.List[0]) // function name
			children = append(children, acc)           // threaded value
			children = append(children, form.List[1:]...)
			acc = lanz.Node{List: children, Line: n.Line}
		}
	}
	return desugarExpr(acc)
}

// (->> x (f a) (g b)) → (g a (f b x))
// Thread x as LAST arg of each successive form.
func desugarThreadLast(n lanz.Node) lanz.Node {
	if len(n.List) < 2 {
		return n
	}
	acc := desugarExpr(n.List[1])
	for _, form := range n.List[2:] {
		if form.IsAtom() {
			acc = list(n.Line, form, acc)
		} else {
			// (->> x (f a b)) → (f a b x)
			children := make([]lanz.Node, 0, len(form.List)+1)
			children = append(children, form.List...)
			children = append(children, acc)
			acc = lanz.Node{List: children, Line: n.Line}
		}
	}
	return desugarExpr(acc)
}

// ── Defmacro ─────────────────────────────────────────────────────────────────

// (defmacro name (param1 param2 ...) body)
// Registers a syntactic macro. On invocation, params are substituted into body.
func desugarDefmacro(n lanz.Node, macros map[string]*macro) ([]lanz.Node, error) {
	if len(n.List) < 4 {
		return nil, fmt.Errorf("line %d: defmacro: expected (defmacro name (params...) body)", n.Line)
	}
	name := n.List[1]
	if !name.IsAtom() {
		return nil, fmt.Errorf("line %d: defmacro: name must be atom", n.Line)
	}
	paramList := n.List[2]
	if paramList.IsAtom() {
		return nil, fmt.Errorf("line %d: defmacro %s: params must be list", n.Line, name.Atom)
	}
	var params []string
	for _, p := range paramList.List {
		if !p.IsAtom() {
			return nil, fmt.Errorf("line %d: defmacro %s: param must be atom", n.Line, name.Atom)
		}
		params = append(params, p.Atom)
	}
	macros[name.Atom] = &macro{params: params, body: n.List[3]}
	return nil, nil // defmacro produces no output
}

// expandMacro substitutes macro arguments into the template body.
func expandMacro(m *macro, call lanz.Node) lanz.Node {
	// Build substitution map: param name → argument node
	bindings := map[string]lanz.Node{}
	for i, pname := range m.params {
		if i+1 < len(call.List) {
			bindings[pname] = call.List[i+1]
		}
	}
	return substituteNode(m.body, bindings, call.Line)
}

func substituteNode(n lanz.Node, bindings map[string]lanz.Node, line int) lanz.Node {
	if n.IsAtom() {
		if repl, ok := bindings[n.Atom]; ok {
			return repl
		}
		return n
	}
	result := make([]lanz.Node, len(n.List))
	for i, child := range n.List {
		result[i] = substituteNode(child, bindings, line)
	}
	return lanz.Node{List: result, Line: line}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func atom(s string, line int) lanz.Node {
	return lanz.Node{Atom: s, Line: line}
}

func list(line int, children ...lanz.Node) lanz.Node {
	return lanz.Node{List: children, Line: line}
}
