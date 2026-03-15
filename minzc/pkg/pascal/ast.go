// Package pascal implements a Turbo Pascal 3.0 subset frontend for MinZ.
//
// Pipeline:
//
//	Pascal source → lex → parse → pascal AST → lower → *hir.Module → MIR2 → Z80
package pascal

import "github.com/minz/minzc/pkg/mir2"

// ── Types ────────────────────────────────────────────────────────────────────

// PasType represents a Pascal data type.
type PasType interface{ pasType() }

// BasicType is a named scalar: Integer, Byte, Word, Boolean, Char.
type BasicType struct{ Name string }

func (*BasicType) pasType() {}

// ArrayType is a static array: array[Lo..Hi] of ElemTy.
type ArrayType struct {
	Lo, Hi int
	Elem   PasType
}

func (*ArrayType) pasType() {}

// RecordType is a record..end with named fields.
type RecordType struct {
	Fields []RecordField
}

type RecordField struct {
	Names []string
	Ty    PasType
}

func (*RecordType) pasType() {}

// PointerType is ^BaseType.
type PointerType struct{ Base PasType }

func (*PointerType) pasType() {}

// StringType is string[MaxLen].
type StringType struct{ MaxLen int }

func (*StringType) pasType() {}

// TypeRef is a reference to a named type (resolved later).
type TypeRef struct{ Name string }

func (*TypeRef) pasType() {}

// ── Module ───────────────────────────────────────────────────────────────────

// Program is the top-level Pascal compilation unit.
type Program struct {
	Name  string
	Decls []Decl
	Body  []Stmt // main begin..end.
}

// ── Declarations ─────────────────────────────────────────────────────────────

type Decl interface{ pasDecl() }

// ConstDecl: const Name = Value;
type ConstDecl struct {
	Name string
	Val  Expr
}

func (*ConstDecl) pasDecl() {}

// TypeDecl: type Name = Type;
type TypeDecl struct {
	Name string
	Ty   PasType
}

func (*TypeDecl) pasDecl() {}

// VarDecl: var Name1, Name2: Type;
type VarDecl struct {
	Names []string
	Ty    PasType
}

func (*VarDecl) pasDecl() {}

// TypedConstDecl: const Name: Type = Value; (TP-specific mutable)
type TypedConstDecl struct {
	Name string
	Ty   PasType
	Val  Expr
}

func (*TypedConstDecl) pasDecl() {}

// ProcDecl is a procedure or function.
type ProcDecl struct {
	Name    string
	Params  []ParamGroup
	RetTy   PasType // nil for procedure
	Locals  []Decl  // const/type/var inside the proc
	SubProc []*ProcDecl
	Body    []Stmt
}

func (*ProcDecl) pasDecl() {}

// ParamGroup: (var? Name1, Name2: Type)
type ParamGroup struct {
	Names []string
	Ty    PasType
	IsVar bool // var parameter (pass by reference)
}

// ── Statements ───────────────────────────────────────────────────────────────

type Stmt interface{ pasStmt() }

// AssignStmt: Target := Val;
type AssignStmt struct {
	Target Expr
	Val    Expr
}

func (*AssignStmt) pasStmt() {}

// CallStmt: Name(Args);
type CallStmt struct {
	Name string
	Args []Expr
}

func (*CallStmt) pasStmt() {}

// IfStmt: if Cond then Then else Else
type IfStmt struct {
	Cond Expr
	Then Stmt
	Else Stmt // nil if no else
}

func (*IfStmt) pasStmt() {}

// WhileStmt: while Cond do Body
type WhileStmt struct {
	Cond Expr
	Body Stmt
}

func (*WhileStmt) pasStmt() {}

// RepeatStmt: repeat Body until Cond
type RepeatStmt struct {
	Body []Stmt
	Cond Expr
}

func (*RepeatStmt) pasStmt() {}

// ForStmt: for Var := Start to/downto End do Body
type ForStmt struct {
	Var    string
	Start  Expr
	End    Expr
	Downto bool
	Body   Stmt
}

func (*ForStmt) pasStmt() {}

// CaseStmt: case Sel of Arms else Default end
type CaseStmt struct {
	Sel     Expr
	Arms    []CaseArm
	Default []Stmt // may be nil
}

type CaseArm struct {
	Vals []int64 // one or more constant values
	Body Stmt
}

func (*CaseStmt) pasStmt() {}

// Block: begin Stmts end
type Block struct {
	Stmts []Stmt
}

func (*Block) pasStmt() {}

// ExitStmt: Exit; (TP extension)
type ExitStmt struct{}

func (*ExitStmt) pasStmt() {}

// AssertStmt: assert Func(Args) = Expected; (MinZ extension for compile-time testing)
type AssertStmt struct {
	FuncName string
	Args     []Expr
	Op       string // "=" or "<>"
	Expected Expr
}

func (*AssertStmt) pasStmt() {}

// ── Expressions ──────────────────────────────────────────────────────────────

type Expr interface{ pasExpr() }

// IntLit: 42, $FF
type IntLit struct{ Val int64 }

func (*IntLit) pasExpr() {}

// CharLit: 'A' or #65
type CharLit struct{ Val byte }

func (*CharLit) pasExpr() {}

// StrLit: 'Hello'
type StrLit struct{ Val string }

func (*StrLit) pasExpr() {}

// BoolLit: true/false
type BoolLit struct{ Val bool }

func (*BoolLit) pasExpr() {}

// VarRef: identifier reference
type VarRef struct{ Name string }

func (*VarRef) pasExpr() {}

// BinOp: L op R
type BinOp struct {
	Op   string
	L, R Expr
}

func (*BinOp) pasExpr() {}

// UnaryOp: op X
type UnaryOp struct {
	Op string
	X  Expr
}

func (*UnaryOp) pasExpr() {}

// CallExpr: Name(Args) in expression context
type CallExpr struct {
	Name string
	Args []Expr
}

func (*CallExpr) pasExpr() {}

// IndexExpr: Base[Index] (array or string indexing)
type IndexExpr struct {
	Base Expr
	Idx  Expr
}

func (*IndexExpr) pasExpr() {}

// FieldExpr: Base.Field (record field access)
type FieldExpr struct {
	Base  Expr
	Field string
}

func (*FieldExpr) pasExpr() {}

// DerefExpr: Ptr^ (pointer dereference)
type DerefExpr struct {
	X Expr
}

func (*DerefExpr) pasExpr() {}

// AddrExpr: @Var (address-of, TP extension)
type AddrExpr struct {
	X Expr
}

func (*AddrExpr) pasExpr() {}

// ── Type mapping helper ──────────────────────────────────────────────────────

// PasTypeToMIR2 converts a resolved PasType to a mir2.Ty.
func PasTypeToMIR2(t PasType) mir2.Ty {
	switch t := t.(type) {
	case *BasicType:
		switch t.Name {
		case "INTEGER":
			return mir2.TyI16
		case "WORD":
			return mir2.TyU16
		case "BYTE", "CHAR":
			return mir2.TyU8
		case "BOOLEAN":
			return mir2.TyBool
		default:
			return mir2.TyU16 // fallback
		}
	case *ArrayType:
		elem := PasTypeToMIR2(t.Elem)
		return &mir2.ArrayTy{Elem: elem, Len: t.Hi - t.Lo + 1}
	case *PointerType:
		return mir2.TyPtr
	case *StringType:
		return &mir2.ArrayTy{Elem: mir2.TyU8, Len: t.MaxLen + 1}
	case *TypeRef:
		// Unresolved — treat as u16 for now
		return mir2.TyU16
	default:
		return mir2.TyVoid
	}
}
