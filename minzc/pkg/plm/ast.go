package plm

// PLMType represents a PL/M-80 data type.
type PLMType int

const (
	PLMByte    PLMType = iota // BYTE — 8-bit unsigned
	PLMWord                   // WORD — 16-bit unsigned
	PLMAddress                // ADDRESS — 16-bit pointer/address
	PLMVoid                   // no return type (PROCEDURE with no declared type)
)

func (t PLMType) String() string {
	switch t {
	case PLMByte:
		return "BYTE"
	case PLMWord:
		return "WORD"
	case PLMAddress:
		return "ADDRESS"
	default:
		return "VOID"
	}
}

// ── Module ────────────────────────────────────────────────────────────────────

// Module is the top-level PL/M-80 compilation unit.
//
// Standard form:
//
//	<Name>: DO; <Decls...> END <Name>;
//
// Also accepted without a module wrapper (bare sequence of declarations).
type Module struct {
	Name  string // module name, or "" for file-scope
	Decls []Decl
}

// ── Declarations ──────────────────────────────────────────────────────────────

// Decl is a module- or procedure-level declaration.
type Decl interface{ plmDecl() }

// VarDecl declares one or more variables.
//
//	DECLARE <name> <type> [AT (<addr>)];
//	DECLARE (<name>, <name>, ...) <type> [AT (<addr>)];
//	DECLARE <name>(<size>) <type>;        ← array declaration
//	DECLARE <name> BASED <base> <type>;   ← pointer overlay (BASED)
type VarDecl struct {
	Names  []string
	Ty     PLMType
	Size   *int    // non-nil for array: DECLARE arr(N) BYTE;
	AtAddr *uint16 // non-nil when AT(address) is present
	Based  string  // non-empty: DECLARE X BASED Y BYTE; — overlay on var Y
}

func (*VarDecl) plmDecl() {}

// ProcDecl is a PROCEDURE declaration.
//
//	<name>: PROCEDURE [(<param-names>)] [<rettype>]; <body> END <name>;
//
// Parameter types are resolved from the DECLARE statements in Decls.
type ProcDecl struct {
	Name   string
	Params []string   // param names; types looked up in Decls
	RetTy  PLMType    // PLMVoid if no return type
	Decls  []*VarDecl // local DECLARE statements (includes param type annotations)
	Body   []Stmt
}

func (*ProcDecl) plmDecl() {}

// ── Statements ────────────────────────────────────────────────────────────────

// Stmt is a PL/M-80 statement.
type Stmt interface{ plmStmt() }

// AssignStmt: <name> = <expr>;  or  <name>(<idx>) = <expr>;  (array element write)
type AssignStmt struct {
	Name string
	Idx  Expr   // non-nil: array element assignment — arr(idx) = val
	Val  Expr
}

func (*AssignStmt) plmStmt() {}

// CallStmt: CALL <name> [(<args>)];
// Also generated for bare procedure calls: <name>(<args>);
type CallStmt struct {
	Fn   string
	Args []Expr
}

func (*CallStmt) plmStmt() {}

// ReturnStmt: RETURN [<expr>];
type ReturnStmt struct {
	Val Expr // nil for void return
}

func (*ReturnStmt) plmStmt() {}

// IfStmt: IF <cond> THEN <stmt> [ELSE <stmt>]
// Each arm is a single statement (often a DoBlock).
type IfStmt struct {
	Cond Expr
	Then Stmt
	Else Stmt // nil if no ELSE
}

func (*IfStmt) plmStmt() {}

// DoBlock: DO; <stmts> END;
type DoBlock struct {
	Body []Stmt
}

func (*DoBlock) plmStmt() {}

// DoWhileStmt: DO WHILE <cond>; <stmts> END;
type DoWhileStmt struct {
	Cond Expr
	Body []Stmt
}

func (*DoWhileStmt) plmStmt() {}

// DoCaseStmt: DO CASE <expr>; <stmt> <stmt> ... END;
// Arms[i] is the statement executed for case value i.
type DoCaseStmt struct {
	Sel  Expr
	Arms []Stmt
}

func (*DoCaseStmt) plmStmt() {}

// DoToStmt: DO <var> = <start> TO <end> [BY <step>]; <stmts> END;
// Equivalent to a counted loop: var = start; DO WHILE var <= end; ...; var = var+step; END;
type DoToStmt struct {
	Var   string
	Start Expr
	End   Expr
	Step  Expr // nil = step 1
	Body  []Stmt
}

func (*DoToStmt) plmStmt() {}

// EnableStmt emits Z80 EI (enable interrupts).
type EnableStmt struct{}

func (*EnableStmt) plmStmt() {}

// DisableStmt emits Z80 DI (disable interrupts).
type DisableStmt struct{}

func (*DisableStmt) plmStmt() {}

// HaltStmt emits Z80 HALT.
type HaltStmt struct{}

func (*HaltStmt) plmStmt() {}

// GoToStmt: GO TO <label>;  (unconditional jump — not lowered yet)
type GoToStmt struct{ Label string }

func (*GoToStmt) plmStmt() {}

// ── Expressions ───────────────────────────────────────────────────────────────

// Expr is a PL/M-80 expression.
type Expr interface{ plmExpr() }

// NumberLit is a decimal or hex (0FFH) integer literal.
type NumberLit struct{ Val uint16 }

func (*NumberLit) plmExpr() {}

// VarRef references a named variable or parameter.
type VarRef struct{ Name string }

func (*VarRef) plmExpr() {}

// BinOp is a binary operation.
// Op: "+", "-", "*", "/", "AND", "OR", "XOR", "<", ">", "=", "<>", "<=", ">="
type BinOp struct {
	Op   string
	L, R Expr
}

func (*BinOp) plmExpr() {}

// UnOp is a unary operation.
// Op: "NOT", "-", ".HIGH.", ".LOW."
type UnOp struct {
	Op string
	X  Expr
}

func (*UnOp) plmExpr() {}

// CallExpr calls a procedure that returns a value (used in expression context).
type CallExpr struct {
	Fn   string
	Args []Expr
}

func (*CallExpr) plmExpr() {}

// CastExpr coerces a value: BYTE(expr), WORD(expr), ADDRESS(expr)
type CastExpr struct {
	Ty PLMType
	X  Expr
}

func (*CastExpr) plmExpr() {}
