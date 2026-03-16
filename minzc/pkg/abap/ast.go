// Package abap implements an ABAP→HIR frontend for the MinZ compiler.
//
// It uses Lars Hvam Petersen's abaplint (TypeScript) as an external parser
// via a JSON bridge, then lowers the resulting AST to HIR for compilation
// through the standard MIR2→Z80 pipeline.
//
// Architecture:
//
//	ABAP source
//	    → abaplint (Node.js)  → JSON AST
//	    → Go deserializer     → abap.Program (this package)
//	    → Lowerer             → *hir.Module
//	    → MIR2 pipeline       → Z80 assembly
package abap

// ── JSON AST types (deserialized from abaplint bridge output) ────────────────

// ParseResult is the top-level JSON output from the bridge.
type ParseResult struct {
	Structure  *StructNode `json:"structure,omitempty"`
	Statements []*StmtNode `json:"statements,omitempty"`
	Errors     []string    `json:"errors,omitempty"`
}

// StructNode is a hierarchical grouping (class body, method body, if-block).
type StructNode struct {
	Type     string      `json:"type"`
	Children []*ASTChild `json:"children,omitempty"`
}

// StmtNode is a single ABAP statement (DATA, WRITE, IF, etc.).
type StmtNode struct {
	Type     string      `json:"type"`
	Tokens   []*Token    `json:"tokens,omitempty"`
	Children []*ASTChild `json:"children,omitempty"`
}

// ASTChild can be a StructNode, StmtNode, or ExprNode — distinguished by Type.
// We use a unified struct and dispatch on context.
type ASTChild struct {
	Type     string      `json:"type"`
	Str      string      `json:"str,omitempty"`      // for Token nodes
	Row      int         `json:"row,omitempty"`
	Col      int         `json:"col,omitempty"`
	Tokens   []*Token    `json:"tokens,omitempty"`
	Children []*ASTChild `json:"children,omitempty"`
}

// Token is a lexical token from abaplint.
type Token struct {
	Str string `json:"str"`
	Row int    `json:"row"`
	Col int    `json:"col"`
}

// ── Semantic AST (simplified, for lowering to HIR) ───────────────────────────

// Program is the top-level semantic unit after AST simplification.
type Program struct {
	Name  string
	Decls []Decl
}

// Decl is any top-level or local declaration.
type Decl interface{ abapDecl() }

// DataDecl is a DATA statement: DATA x TYPE i VALUE 42.
type DataDecl struct {
	Name    string
	AbapTy  string // "i", "c", "string", "n", "p", "x", "d", "t", "f", etc.
	Length  int    // for TYPE c LENGTH n, TYPE n LENGTH n, TYPE x LENGTH n
	Value   *Expr_ // initial value (nil = default)
}

func (*DataDecl) abapDecl() {}

// FormDecl is a FORM/ENDFORM subroutine.
type FormDecl struct {
	Name     string
	Using    []FormParam
	Changing []FormParam
	Body     []Stmt_
}

func (*FormDecl) abapDecl() {}

// FormParam is a USING/CHANGING parameter.
type FormParam struct {
	Name   string
	AbapTy string // type after TYPE/LIKE
}

// ClassDecl is a CLASS ... DEFINITION/IMPLEMENTATION.
type ClassDecl struct {
	Name       string
	Super      string // INHERITING FROM
	Interfaces []string
	Attrs      []*DataDecl  // class-data / instance data
	Methods    []*MethodDecl
}

func (*ClassDecl) abapDecl() {}

// MethodDecl is a METHOD ... ENDMETHOD.
type MethodDecl struct {
	Name      string
	Importing []FormParam
	Exporting []FormParam
	Changing  []FormParam
	Returning *FormParam // RETURNING VALUE(rv)
	Body      []Stmt_
}

// InterfaceDecl is an INTERFACE ... ENDINTERFACE.
type InterfaceDecl struct {
	Name    string
	Methods []string // method names
}

func (*InterfaceDecl) abapDecl() {}

// ── Statements ───────────────────────────────────────────────────────────────

// Stmt_ is a semantic ABAP statement.
type Stmt_ interface{ abapStmt() }

type WriteStmt struct {
	Exprs []Expr_ // WRITE: expr1, expr2, ...
}
func (*WriteStmt) abapStmt() {}

type AssignStmt struct {
	Target string // variable name
	Val    Expr_
}
func (*AssignStmt) abapStmt() {}

type IfStmt struct {
	Cond Expr_
	Then []Stmt_
	ElseIf []struct {
		Cond Expr_
		Body []Stmt_
	}
	Else []Stmt_
}
func (*IfStmt) abapStmt() {}

type DoStmt struct {
	Times *Expr_ // nil = infinite (needs EXIT)
	Body  []Stmt_
}
func (*DoStmt) abapStmt() {}

type WhileStmt struct {
	Cond Expr_
	Body []Stmt_
}
func (*WhileStmt) abapStmt() {}

type LoopStmt struct {
	Table string  // LOOP AT table
	Into  string  // INTO wa
	Where *Expr_  // WHERE condition
	Body  []Stmt_
}
func (*LoopStmt) abapStmt() {}

type PerformStmt struct {
	FormName string
	Using    []Expr_
	Changing []Expr_
}
func (*PerformStmt) abapStmt() {}

type ExitStmt struct{} // EXIT
func (*ExitStmt) abapStmt() {}

type ContinueStmt struct{} // CONTINUE
func (*ContinueStmt) abapStmt() {}

type ReturnStmt struct{}
func (*ReturnStmt) abapStmt() {}

type CallMethodStmt struct {
	Receiver string // object or class name
	Method   string
	Args     map[string]Expr_ // named parameters
}
func (*CallMethodStmt) abapStmt() {}

type CreateObjectStmt struct {
	Target string // variable name
	Class  string
	Args   map[string]Expr_
}
func (*CreateObjectStmt) abapStmt() {}

type CaseStmt struct {
	Val   Expr_
	Whens []struct {
		Vals []Expr_
		Body []Stmt_
	}
	Others []Stmt_
}
func (*CaseStmt) abapStmt() {}

// ── Expressions ──────────────────────────────────────────────────────────────

// Expr_ is a semantic ABAP expression.
type Expr_ interface{ abapExpr() }

type IntLit struct{ Val int64 }
func (*IntLit) abapExpr() {}

type StringLit struct{ Val string }
func (*StringLit) abapExpr() {}

type VarRef struct{ Name string }
func (*VarRef) abapExpr() {}

type BinOp struct {
	Op  string // "+", "-", "*", "/", "MOD", "=", "<>", "<", ">", "<=", ">="
	LHS Expr_
	RHS Expr_
}
func (*BinOp) abapExpr() {}

type UnaryOp struct {
	Op  string // "-", "NOT"
	Val Expr_
}
func (*UnaryOp) abapExpr() {}

type FieldAccess struct {
	Obj   Expr_
	Field string // struct-field or attribute
}
func (*FieldAccess) abapExpr() {}

type MethodCall struct {
	Receiver Expr_
	Method   string
	Args     map[string]Expr_
}
func (*MethodCall) abapExpr() {}

type FuncCall struct {
	Name string
	Args []Expr_
}
func (*FuncCall) abapExpr() {}
