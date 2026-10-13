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
	"github.com/minz/minzc/pkg/mir2"
)

// macros holds user-defined macros: name → (params, body template).
type macro struct {
	params []string  // parameter names
	body   lanz.Node // template body (params substituted on expansion)
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
	// Sanitize identifiers: Lisp allows hyphens in names (sum-loop),
	// but Z80 assembler treats '-' as minus. Replace with '_'.
	for i := range desugared {
		desugared[i] = sanitizeIdents(desugared[i])
	}
	// Pass desugared nodes directly to Lanz compiler (no serialize roundtrip).
	m, err := lanz.CompileNodes(desugared, name)
	if err != nil {
		return nil, err
	}
	for _, f := range m.Funcs {
		if f.IsExtern || f.RetTy == mir2.TyVoid {
			continue
		}
		if err := verifyValueReturns(f.Body); err != nil {
			return nil, fmt.Errorf("lizp function %s: %w", f.Name, err)
		}
	}
	return m, nil
}

// Reject void-valued returns even when a statement-shaped tail was parsed
// as a call. This also catches known void callees and bare early returns.
func verifyValueReturns(b *hir.Block) error {
	if b == nil {
		return nil
	}
	for _, s := range b.Body {
		switch s := s.(type) {
		case *hir.ReturnStmt:
			if len(s.Vals) == 0 && (s.Val == nil || s.Val.ExprTy() == mir2.TyVoid) {
				return fmt.Errorf("return requires a non-void value")
			}
		case *hir.Block:
			if err := verifyValueReturns(s); err != nil {
				return err
			}
		case *hir.IfStmt:
			if err := verifyValueReturns(s.Then); err != nil {
				return err
			}
			if err := verifyValueReturns(s.Else); err != nil {
				return err
			}
		case *hir.WhileStmt:
			if err := verifyValueReturns(s.Body); err != nil {
				return err
			}
		case *hir.ForRangeStmt:
			if err := verifyValueReturns(s.Body); err != nil {
				return err
			}
		case *hir.ForEachStmt:
			if err := verifyValueReturns(s.Body); err != nil {
				return err
			}
		case *hir.SwitchStmt:
			for _, c := range s.Cases {
				if err := verifyValueReturns(c.Body); err != nil {
					return err
				}
			}
			if err := verifyValueReturns(s.Default); err != nil {
				return err
			}
		}
	}
	return nil
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
		expanded := n
		expanded.List = append([]lanz.Node(nil), n.List...)
		var err error
		// Function names and typed parameters are declarations, not macro calls.
		for i := 3; i < len(n.List); i++ {
			if n.List[i].IsAtom() && n.List[i].Atom == "->" {
				for j := i + 2; j < len(n.List); j++ {
					expanded.List[j], err = expandBodyMacros(n.List[j], macros, 0)
					if err != nil {
						return nil, err
					}
				}
				break
			}
		}
		return desugarDefun(expanded)
	case "define":
		// Scheme R5RS: (define (name params...) body) → defun
		return desugarDefine(n)
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

// Expand macros inside function bodies before ordinary form desugaring.
// A bounded expansion depth reports recursive templates instead of emitting
// an unresolved CALL or recursing without a limit.
func expandBodyMacros(n lanz.Node, macros map[string]*macro, depth int) (lanz.Node, error) {
	if depth > 128 {
		return lanz.Node{}, fmt.Errorf("line %d: macro expansion depth exceeded", n.Line)
	}
	if n.IsAtom() || len(n.List) == 0 {
		return n, nil
	}
	if n.List[0].IsAtom() {
		if m, ok := macros[n.List[0].Atom]; ok {
			if len(n.List)-1 != len(m.params) {
				return lanz.Node{}, fmt.Errorf("line %d: macro %s arity %d, want %d", n.Line, n.List[0].Atom, len(n.List)-1, len(m.params))
			}
			return expandBodyMacros(expandMacro(m, n), macros, depth+1)
		}
	}
	children := make([]lanz.Node, len(n.List))
	for i, ch := range n.List {
		var err error
		children[i], err = expandBodyMacros(ch, macros, depth)
		if err != nil {
			return lanz.Node{}, err
		}
	}
	return lanz.Node{List: children, Line: n.Line}, nil
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

	// Preserve parameters named t/nil: these names are identifiers when bound,
	// and only unbound occurrences are Lisp truth/zero aliases.
	aliases := map[string]string{}
	aliasID := 0
	reserveAliasNames(n, aliases)
	params = renameTruthBindings(params, aliases, &aliasID)

	// Desugar body statements
	var body []lanz.Node
	for _, s := range n.List[bodyStart:] {
		ds := desugarExpr(shadowTruthAliases(s, aliases, &aliasID))
		body = append(body, ds)
	}

	// Value functions return the last form, after syntax normalization.
	if retType.Atom != "void" {
		if len(body) == 0 {
			return nil, fmt.Errorf("line %d: defun %s: missing result form", n.Line, name.Atom)
		}
		tail, err := returnTail(body[len(body)-1])
		if err != nil {
			return nil, fmt.Errorf("defun %s: %w", name.Atom, err)
		}
		body[len(body)-1] = tail
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

// returnTail consumes normalized Lanz forms, preserving explicit returns.
func returnTail(n lanz.Node) (lanz.Node, error) {
	if n.IsList() && len(n.List) > 0 && n.List[0].IsAtom() {
		switch n.List[0].Atom {
		case "return":
			if len(n.List) != 2 {
				return n, fmt.Errorf("line %d: value function requires a return value", n.Line)
			}
			return n, nil
		case "block":
			if len(n.List) < 2 {
				return n, fmt.Errorf("line %d: empty result block", n.Line)
			}
			items := append([]lanz.Node(nil), n.List...)
			tail, err := returnTail(items[len(items)-1])
			items[len(items)-1] = tail
			return lanz.Node{List: items, Line: n.Line}, err
		case "if":
			if len(n.List) != 4 {
				return n, fmt.Errorf("line %d: result if requires both branches", n.Line)
			}
			left, err := returnTail(n.List[2])
			if err != nil {
				return n, err
			}
			right, err := returnTail(n.List[3])
			return list(n.Line, n.List[0], n.List[1], left, right), err
		case "set", "var", "store", "while", "for", "break", "continue":
			return n, fmt.Errorf("line %d: %s does not supply a result", n.Line, n.List[0].Atom)
		}
	}
	return list(n.Line, atom("return", n.Line), n), nil
}

// (define (name ((param type) ...)) -> rettype body...)
// Scheme R5RS shorthand: (define (square ((x u8))) -> u8 (return (* x x)))
// Also supports: (define name value) for simple constants → defglobal
func desugarDefine(n lanz.Node) ([]lanz.Node, error) {
	if len(n.List) < 3 {
		return nil, fmt.Errorf("line %d: define: too few arguments", n.Line)
	}
	nameOrSig := n.List[1]
	if nameOrSig.IsList() {
		// (define (name params...) -> rettype body...) → (defun name params -> rettype body...)
		if len(nameOrSig.List) < 1 {
			return nil, fmt.Errorf("line %d: define: empty signature", n.Line)
		}
		// Rewrite as defun
		defunList := []lanz.Node{atom("defun", n.Line)}
		defunList = append(defunList, nameOrSig.List[0]) // name
		if len(nameOrSig.List) > 1 {
			// params as list
			defunList = append(defunList, lanz.Node{List: nameOrSig.List[1:], Line: n.Line})
		} else {
			defunList = append(defunList, lanz.Node{List: nil, Line: n.Line}) // no params
		}
		defunList = append(defunList, n.List[2:]...) // -> rettype body...
		return desugarDefun(lanz.Node{List: defunList, Line: n.Line})
	}
	// (define name value) → (defglobal name u8 value) — simple constant
	elems := []lanz.Node{atom("global", n.Line), nameOrSig, atom("u8", n.Line)}
	if len(n.List) >= 3 {
		elems = append(elems, desugarExpr(n.List[2]))
	}
	return []lanz.Node{{List: elems, Line: n.Line}}, nil
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

// Alpha-renaming happens before sugar expansion, so truth aliases remain
// available without rewriting a declared parameter or let variable into a literal.
func reserveAliasNames(n lanz.Node, scope map[string]string) {
	if n.IsAtom() {
		scope["\x00"+n.Atom] = n.Atom
		return
	}
	for _, child := range n.List {
		reserveAliasNames(child, scope)
	}
}

func renameTruthBindings(bindings lanz.Node, scope map[string]string, id *int) lanz.Node {
	bindings.List = append([]lanz.Node(nil), bindings.List...)
	for i, b := range bindings.List {
		if !b.IsList() || len(b.List) == 0 {
			continue
		}
		name := b.List[0].Atom
		if name != "t" && name != "nil" {
			continue
		}
		fresh := ""
		for {
			(*id)++
			fresh = fmt.Sprintf("__lizp_alias_%s_%d", name, *id)
			if _, occupied := scope["\x00"+fresh]; !occupied {
				break
			}
		}
		scope["\x00"+fresh] = fresh
		scope[name] = fresh
		b.List = append([]lanz.Node(nil), b.List...)
		b.List[0] = atom(fresh, b.Line)
		bindings.List[i] = b
	}
	return bindings
}

func shadowTruthAliases(n lanz.Node, scope map[string]string, id *int) lanz.Node {
	if n.IsAtom() {
		if name, ok := scope[n.Atom]; ok {
			n.Atom = name
		}
		return n
	}
	if len(n.List) == 0 {
		return n
	}
	n.List = append([]lanz.Node(nil), n.List...)
	head := n.List[0].Atom
	if (head == "let" || head == "fn" || head == "lambda") && len(n.List) >= 3 {
		inner := make(map[string]string, len(scope))
		for k, v := range scope {
			inner[k] = v
		}
		bindings := n.List[1]
		if head == "let" {
			bindings.List = append([]lanz.Node(nil), bindings.List...)
			for i, b := range bindings.List {
				b.List = append([]lanz.Node(nil), b.List...)
				for j := 2; j < len(b.List); j++ {
					b.List[j] = shadowTruthAliases(b.List[j], scope, id)
				}
				bindings.List[i] = b
			}
		}
		n.List[1] = renameTruthBindings(bindings, inner, id)
		start := 2
		if head != "let" {
			start = 3
		}
		for i := start; i < len(n.List); i++ {
			n.List[i] = shadowTruthAliases(n.List[i], inner, id)
		}
		return n
	}
	for i := 1; i < len(n.List); i++ {
		n.List[i] = shadowTruthAliases(n.List[i], scope, id)
	}
	return n
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
	case "@print_u8":
		if len(n.List) != 2 {
			return desugarList(n)
		}
		return list(n.Line, atom("@mir.io.print.u8", n.Line), desugarExpr(n.List[1]))
	case "@print_nl":
		if len(n.List) != 1 {
			return desugarList(n)
		}
		return list(n.Line, atom("@mir.io.print.nl", n.Line))
	case "return":
		if len(n.List) == 2 {
			return desugarReturnValue(n.List[1], n.Line)
		}
		return n
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
	case "zerop", "zero?", "null?":
		// (zerop x) / (zero? x) / (null? x) → (== x 0)
		if len(n.List) == 2 {
			x := desugarExpr(n.List[1])
			return list(n.Line, atom("==", n.Line), x, atom("0", n.Line))
		}
		return n
	case "positive?":
		// (positive? x) → (> x 0)
		if len(n.List) == 2 {
			x := desugarExpr(n.List[1])
			return list(n.Line, atom(">", n.Line), x, atom("0", n.Line))
		}
		return n
	case "negative?":
		// (negative? x) → (< x 0) — always false for u8!
		if len(n.List) == 2 {
			x := desugarExpr(n.List[1])
			return list(n.Line, atom("<", n.Line), x, atom("0", n.Line))
		}
		return n
	case "not":
		// (not x) → (== x 0) — logical not
		if len(n.List) == 2 {
			x := desugarExpr(n.List[1])
			return list(n.Line, atom("==", n.Line), x, atom("0", n.Line))
		}
		return n
	case "even?":
		// (even? x) → (== (% x 2) 0)
		if len(n.List) == 2 {
			x := desugarExpr(n.List[1])
			mod := list(n.Line, atom("%", n.Line), x, atom("2", n.Line))
			return list(n.Line, atom("==", n.Line), mod, atom("0", n.Line))
		}
		return n
	case "odd?":
		// (odd? x) → (!= (% x 2) 0)
		if len(n.List) == 2 {
			x := desugarExpr(n.List[1])
			mod := list(n.Line, atom("%", n.Line), x, atom("2", n.Line))
			return list(n.Line, atom("!=", n.Line), mod, atom("0", n.Line))
		}
		return n
	case "min":
		// (min a b) → (if (< a b) a b)
		if len(n.List) == 3 {
			a := desugarExpr(n.List[1])
			b := desugarExpr(n.List[2])
			return list(n.Line, atom("if", n.Line),
				list(n.Line, atom("<", n.Line), a, b), a, b)
		}
		return n
	case "max":
		// (max a b) → (if (> a b) a b)
		if len(n.List) == 3 {
			a := desugarExpr(n.List[1])
			b := desugarExpr(n.List[2])
			return list(n.Line, atom("if", n.Line),
				list(n.Line, atom(">", n.Line), a, b), a, b)
		}
		return n
	case "abs":
		// (abs x) → (if (> x 127) (- 0 x) x) — u8 twos complement
		if len(n.List) == 2 {
			x := desugarExpr(n.List[1])
			return list(n.Line, atom("if", n.Line),
				list(n.Line, atom(">", n.Line), x, atom("127", n.Line)),
				list(n.Line, atom("-", n.Line), atom("0", n.Line), x),
				x)
		}
		return n
	case "fn", "lambda":
		// Scheme-style lambda: (fn ((x u8)) u8 body) or (lambda ((x u8)) u8 body)
		return desugarRenamed(n, "lambda")
	case "do":
		// Scheme R5RS do-loop:
		// (do ((var init type step)) (test result) body...)
		// → (block (var var type init) (while (not test) body... (set var step)) result)
		return desugarDo(n)
	case "letrec":
		// (letrec ((name type val)) body) — same as let for our purposes
		// (recursive bindings resolved at compile time)
		return desugarLet(n)
	case "modulo", "remainder", "mod":
		// (modulo a b) → (% a b) — Lanz uses % for modulo
		return desugarBinOp(n, "%")
	case "quotient":
		// (quotient a b) → (/ a b) — integer division
		return desugarBinOp(n, "/")
	case "char->integer":
		// (char->integer c) → c (identity on Z80, chars are u8)
		if len(n.List) == 2 {
			return desugarExpr(n.List[1])
		}
		return n
	case "integer->char":
		// (integer->char n) → n (identity on Z80)
		if len(n.List) == 2 {
			return desugarExpr(n.List[1])
		}
		return n
	case "peek":
		if len(n.List) == 2 {
			return list(n.Line, atom("load", n.Line), desugarExpr(n.List[1]), atom("u8", n.Line))
		}
		return n
	case "poke":
		if len(n.List) == 3 {
			// Poke writes one byte even when the value expression is a word.
			val := list(n.Line, atom("cast", n.Line), desugarExpr(n.List[2]), atom("u8", n.Line))
			return list(n.Line, atom("store", n.Line), desugarExpr(n.List[1]), val)
		}
		return n
	case "string-ref":
		// (string-ref str idx) → (load (+ str idx))
		if len(n.List) == 3 {
			str := desugarExpr(n.List[1])
			idx := desugarExpr(n.List[2])
			addr := list(n.Line, atom("+", n.Line), str, idx)
			return list(n.Line, atom("load", n.Line), addr)
		}
		return n
	case "string-length":
		// (string-length str) → strlen(str) — needs runtime, approximate with 0
		if len(n.List) == 2 {
			return desugarExpr(n.List[1])
		}
		return n
	case "display":
		// (display x) → (call putchar x) — single char output
		if len(n.List) == 2 {
			return list(n.Line, atom("putchar", n.Line), desugarExpr(n.List[1]))
		}
		return n
	case "newline":
		// (newline) → (block (putchar 13) (putchar 10))
		return list(n.Line, atom("block", n.Line),
			list(n.Line, atom("putchar", n.Line), atom("13", n.Line)),
			list(n.Line, atom("putchar", n.Line), atom("10", n.Line)))
	case "number->string":
		// stub — return the number itself
		if len(n.List) == 2 {
			return desugarExpr(n.List[1])
		}
		return n
	case "bitwise-and":
		return desugarBinOp(n, "&")
	case "bitwise-or":
		return desugarBinOp(n, "|")
	case "bitwise-xor":
		return desugarBinOp(n, "^")
	case "bitwise-not":
		// (bitwise-not x) → (^ x 255)
		if len(n.List) == 2 {
			return list(n.Line, atom("^", n.Line), desugarExpr(n.List[1]), atom("255", n.Line))
		}
		return n
	case "arithmetic-shift":
		// (arithmetic-shift x n) → (<< x n) if n>0, (>> x (- 0 n)) if n<0
		return desugarBinOp(n, "<<")
	case "exact->inexact", "inexact->exact":
		// identity on Z80 (all integers)
		if len(n.List) == 2 {
			return desugarExpr(n.List[1])
		}
		return n
	case "case":
		// Scheme-style case: (case expr (val body) ...) → (match expr (val body) ...)
		return desugarRenamed(n, "match")
	case "let*":
		// Scheme let*: (let* ((name type val)) body-expr)
		// → (let-in name type val body-expr)
		return desugarLetStar(n)
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

// (do ((var type init step)) (test result) body...)
// → (block (var var type init) (while (not test) body... (set var step)) result)
// Scheme R5RS iteration construct.
func desugarDo(n lanz.Node) lanz.Node {
	if len(n.List) < 3 || !n.List[1].IsList() || !n.List[2].IsList() {
		return n
	}
	bindings := n.List[1]   // ((var type init step) ...)
	testClause := n.List[2] // (test result)
	body := n.List[3:]

	var stmts []lanz.Node
	stmts = append(stmts, atom("block", n.Line))

	// Declare loop variables
	var stepSets []lanz.Node
	for _, b := range bindings.List {
		if !b.IsList() || len(b.List) < 3 {
			continue
		}
		varName := b.List[0]
		varType := b.List[1]
		initVal := desugarExpr(b.List[2])
		// (var name type init)
		stmts = append(stmts, lanz.Node{
			List: []lanz.Node{atom("var", b.Line), varName, varType, initVal},
			Line: b.Line,
		})
		// step expression (if provided)
		if len(b.List) >= 4 {
			step := desugarExpr(b.List[3])
			stepSets = append(stepSets, list(b.Line, atom("set", b.Line), varName, step))
		}
	}

	// Test and result
	test := desugarExpr(testClause.List[0])
	var result lanz.Node
	if len(testClause.List) >= 2 {
		result = desugarExpr(testClause.List[1])
	} else {
		result = atom("0", n.Line)
	}

	// Build while loop: (while (not test) body... steps...)
	notTest := list(n.Line, atom("==", n.Line), test, atom("0", n.Line))
	var whileBody []lanz.Node
	whileBody = append(whileBody, atom("while", n.Line), notTest)
	for _, s := range body {
		whileBody = append(whileBody, desugarExpr(s))
	}
	whileBody = append(whileBody, stepSets...)

	stmts = append(stmts, lanz.Node{List: whileBody, Line: n.Line})
	stmts = append(stmts, result)

	return lanz.Node{List: stmts, Line: n.Line}
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

// In a return context, progn must run its prefix before returning its final
// value. Convert branches to statement control flow rather than a void call
// to the syntactic block form.
func desugarReturnValue(n lanz.Node, line int) lanz.Node {
	if n.IsList() && len(n.List) > 0 {
		switch n.List[0].Atom {
		case "return":
			return desugarExpr(n)
		case "let":
			return desugarReturnValue(desugarLet(n), line)
		case "cond":
			return desugarReturnValue(desugarCond(n), line)
		case "if":
			if len(n.List) == 4 {
				return list(line, atom("if", line), desugarExpr(n.List[1]), desugarReturnValue(n.List[2], line), desugarReturnValue(n.List[3], line))
			}
		case "progn", "begin", "block":
			if len(n.List) > 1 {
				body := []lanz.Node{atom("block", line)}
				for _, s := range n.List[1 : len(n.List)-1] {
					body = append(body, desugarExpr(s))
				}
				body = append(body, desugarReturnValue(n.List[len(n.List)-1], line))
				return lanz.Node{List: body, Line: line}
			}
		}
	}
	return list(line, atom("return", line), desugarExpr(n))
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
	if len(clause.List) > 2 {
		body = desugarProgn(lanz.Node{List: append([]lanz.Node{atom("progn", line)}, clause.List[1:]...), Line: line})
	}

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

// sanitizeIdents replaces hyphens in identifiers with underscores.
// Lisp convention: sum-loop, is-even. Z80 assembler: sum_loop, is_even.
func sanitizeIdents(n lanz.Node) lanz.Node {
	if n.IsAtom() {
		if strings.Contains(n.Atom, "-") {
			// Don't touch operators, numbers, hex literals, arrows.
			//
			// "let-in" is a lanz reserved form, not an identifier: desugarLetStar
			// expands (let* ...) into a chain of it, and this pass runs after
			// desugaring, so mangling it to let_in leaves a form lanz's lowerer
			// does not recognise — the enclosing function then fails to lower and
			// silently disappears from the module.
			if n.Atom == "-" || n.Atom == "->" || n.Atom == "->>" ||
				n.Atom == "let-in" ||
				strings.HasPrefix(n.Atom, "0x") || strings.HasPrefix(n.Atom, "#") {
				return n
			}
			// Check if it's a negative number
			if len(n.Atom) > 1 && n.Atom[0] == '-' && n.Atom[1] >= '0' && n.Atom[1] <= '9' {
				return n
			}
			return lanz.Node{Atom: strings.ReplaceAll(n.Atom, "-", "_"), Line: n.Line}
		}
		return n
	}
	result := make([]lanz.Node, len(n.List))
	for i, child := range n.List {
		result[i] = sanitizeIdents(child)
	}
	return lanz.Node{List: result, Line: n.Line}
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
			children = append(children, acc)          // threaded value
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

// desugarRenamed replaces the head atom with newHead, then desugars children.
func desugarRenamed(n lanz.Node, newHead string) lanz.Node {
	result := make([]lanz.Node, len(n.List))
	result[0] = atom(newHead, n.Line)
	for i := 1; i < len(n.List); i++ {
		result[i] = desugarExpr(n.List[i])
	}
	return lanz.Node{List: result, Line: n.Line}
}

// (let* ((name type val) ...) body-expr)
// → nested let-in chain: (let-in name type val (let-in name2 type2 val2 body))
func desugarLetStar(n lanz.Node) lanz.Node {
	if len(n.List) < 3 || !n.List[1].IsList() {
		return n
	}
	bindings := n.List[1].List
	// Body is the last element (expression, not statement)
	body := desugarExpr(n.List[2])

	// Build inside-out
	for i := len(bindings) - 1; i >= 0; i-- {
		b := bindings[i]
		if !b.IsList() || len(b.List) < 3 {
			continue
		}
		name := b.List[0]
		ty := b.List[1]
		val := desugarExpr(b.List[2])
		body = lanz.Node{
			List: []lanz.Node{atom("let-in", n.Line), name, ty, val, body},
			Line: n.Line,
		}
	}
	return body
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func atom(s string, line int) lanz.Node {
	return lanz.Node{Atom: s, Line: line}
}

func list(line int, children ...lanz.Node) lanz.Node {
	return lanz.Node{List: children, Line: line}
}
