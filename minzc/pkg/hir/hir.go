// Package hir implements the High-level Intermediate Representation for MinZ.
//
// HIR sits between the AST (from pkg/ast) and MIR2 (from pkg/mir2).
// It is a "TypedAST": names are resolved, types are attached to every
// expression, but control flow is still structured (if/while/for) and
// variables are named (not SSA virtual registers).
//
// Pipeline:
//
//	MinZ source
//	    → Parser     → pkg/ast (raw AST)
//	    → Analyzer2  → pkg/hir (TypedAST)   ← this package
//	    → Lowerer    → pkg/mir2              ← lower.go in this package
//	    → Allocate   → AllocResult
//	    → Z80Codegen → .a80 assembly
//
// Compared to MIR1 (pkg/ir):
//   - No Z80-specific opcodes (no OpDJNZ, no SMC opcodes, no RegisterHint)
//   - No virtual register numbers (variables are named)
//   - Structured control flow (not basic blocks)
//   - Types reuse mir2.Ty (both are target-neutral)
//   - No optimizer passes — optimizations happen at MIR2 level
package hir

import "github.com/minz/minzc/pkg/mir2"

// ── Module ────────────────────────────────────────────────────────────────────

// InterfaceDecl is a Go-style interface declaration.
// It records the set of method names required by the interface.
// For now, Nanz interfaces are structural (duck-typed): any type that has all
// listed methods satisfies the interface.  No vtable is emitted; dispatch is
// monomorphised at call sites.
type InterfaceDecl struct {
	Name    string   // e.g. "Animal"
	Methods []string // method names, e.g. ["speak", "move"]
}

// Module is the top-level HIR unit.
type Module struct {
	Name       string
	Funcs      []*Func
	Globals    []mir2.Global      // reuse mir2.Global directly
	Structs    []*mir2.StructTy   // named struct type declarations
	Interfaces []*InterfaceDecl   // interface declarations (zero-cost, monomorphised)
	Strings    []string           // interned string literals (index = position)
}

// FuncByName returns the first HIR function with the given name, or nil.
func (m *Module) FuncByName(name string) *Func {
	for _, f := range m.Funcs {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// ── Function ──────────────────────────────────────────────────────────────────

// Func is a HIR function.
type Func struct {
	Name     string
	Params   []Param
	RetTy    mir2.Ty  // mir2.TyVoid for void functions
	Body     *Block   // nil for extern
	IsExtern bool
}

// Param is a typed, named function parameter.
type Param struct {
	Name string
	Ty   mir2.Ty
}

// ── Statements ────────────────────────────────────────────────────────────────

// Stmt is the HIR statement interface.
type Stmt interface{ hirStmt() }

// Block is a sequence of statements.
type Block struct{ Body []Stmt }

func (*Block) hirStmt() {}

// VarDeclStmt declares a new local variable.
// In HIR, variables are mutable (assignments create new bindings in lowering).
type VarDeclStmt struct {
	Name     string
	Ty       mir2.Ty  // element type (for arrays) or scalar type
	ArrayLen int      // if > 0: this is a [ArrayLen]Ty array; Ty is the element type
	Init     Expr     // nil = zero-initialised (scalar only; use Initial for arrays)
	Initial  []Expr   // array initializer values (len must equal ArrayLen or be 0)
	At       *uint16  // if non-nil: variable is at this absolute address (AT attribute)
}

func (*VarDeclStmt) hirStmt() {}

// AssignStmt assigns a new value to an existing variable or through a pointer.
// For simple var = expr: Target is VarRefExpr.
// For *ptr = expr: Target is a DerefExpr.
type AssignStmt struct {
	Target Expr // VarRefExpr or DerefExpr
	Val    Expr
}

func (*AssignStmt) hirStmt() {}

// ReturnStmt exits the current function.  Val is nil for void returns.
type ReturnStmt struct{ Val Expr }

func (*ReturnStmt) hirStmt() {}

// IfStmt is a conditional branch.  Else may be nil.
type IfStmt struct {
	Cond Expr
	Then *Block
	Else *Block // nil if no else branch
}

func (*IfStmt) hirStmt() {}

// WhileStmt is a pre-condition loop.
type WhileStmt struct {
	Cond Expr
	Body *Block
}

func (*WhileStmt) hirStmt() {}

// ForRangeStmt is a counted loop: for Var in Start..End.
// Equivalent to: Var = Start; while Var < End { ...; Var++ }
type ForRangeStmt struct {
	Var   string
	Start Expr
	End   Expr
	Body  *Block
}

func (*ForRangeStmt) hirStmt() {}

// ExprStmt evaluates an expression for its side effects (typically a call).
type ExprStmt struct{ Expr Expr }

func (*ExprStmt) hirStmt() {}

// ForEachStmt iterates over a contiguous slice of memory:
//
//	for x: ElemTy in ptr[start..end] { body }
//
// Lowers to a DJNZ-friendly loop in MIR2:
//
//	ptr → ClassPointer (HL); counter → ClassCounter (B)
//	loop: x = *ptr; body; ptr += stride; counter--; if counter != 0 goto loop
type ForEachStmt struct {
	Var           string  // element variable name (bound in body)
	ElemTy        mir2.Ty // type of each element
	Ptr           Expr    // base pointer (TyPtr)
	Start         Expr    // start index (often IntLitExpr{0})
	Len           Expr    // element count (= end - start when start=0)
	Body          *Block
	MutateInPlace bool    // if true: store transformed element back to ptr after body
}

func (*ForEachStmt) hirStmt() {}

// BreakStmt exits the nearest enclosing loop.
type BreakStmt struct{}

func (*BreakStmt) hirStmt() {}

// ContinueStmt jumps to the next iteration of the nearest enclosing loop.
type ContinueStmt struct{}

func (*ContinueStmt) hirStmt() {}

// SwitchCase is one arm of a SwitchStmt.
type SwitchCase struct {
	Val  int64  // constant case value
	Body *Block
}

// SwitchStmt dispatches on a value: switch Val { case C1: body1; case C2: body2; ... default: bodyD }
// Lowered to a chain of if/else comparisons (suitable for small N).
type SwitchStmt struct {
	Val     Expr
	Cases   []*SwitchCase
	Default *Block // nil if no default
}

func (*SwitchStmt) hirStmt() {}

// StoreStmt writes through a pointer: *Ptr = Val
type StoreStmt struct {
	Ptr Expr // must have type mir2.TyPtr
	Val Expr
}

func (*StoreStmt) hirStmt() {}

// ── Expressions ───────────────────────────────────────────────────────────────

// Expr is the HIR expression interface.
// Every expression carries its resolved type.
type Expr interface {
	hirExpr()
	ExprTy() mir2.Ty
}

// IntLitExpr is an integer constant.
type IntLitExpr struct {
	Val int64
	Ty  mir2.Ty // TyU8, TyU16, TyI8, TyI16 …
}

func (*IntLitExpr) hirExpr()          {}
func (e *IntLitExpr) ExprTy() mir2.Ty { return e.Ty }

// BoolLitExpr is a boolean constant.
type BoolLitExpr struct{ Val bool }

func (*BoolLitExpr) hirExpr()          {}
func (*BoolLitExpr) ExprTy() mir2.Ty   { return mir2.TyBool }

// VarRefExpr references a named variable or parameter.
type VarRefExpr struct {
	Name string
	Ty   mir2.Ty
}

func (*VarRefExpr) hirExpr()          {}
func (e *VarRefExpr) ExprTy() mir2.Ty { return e.Ty }

// BinExpr is a binary operation.
// Op is one of: + - * / % & | ^ << >> == != < <= > >=
type BinExpr struct {
	Op   string
	L, R Expr
	Ty   mir2.Ty
}

func (*BinExpr) hirExpr()          {}
func (e *BinExpr) ExprTy() mir2.Ty { return e.Ty }

// UnaryExpr is a unary operation.  Op is one of: - ! ~
type UnaryExpr struct {
	Op string
	X  Expr
	Ty mir2.Ty
}

func (*UnaryExpr) hirExpr()          {}
func (e *UnaryExpr) ExprTy() mir2.Ty { return e.Ty }

// CallExpr calls a named function.
type CallExpr struct {
	Fn   string
	Args []Expr
	Ty   mir2.Ty // return type; TyVoid for void calls
}

func (*CallExpr) hirExpr()          {}
func (e *CallExpr) ExprTy() mir2.Ty { return e.Ty }

// FieldExpr accesses a struct field.  Offset is the byte offset from the base.
type FieldExpr struct {
	X      Expr
	Field  string
	Offset int
	Ty     mir2.Ty
}

func (*FieldExpr) hirExpr()          {}
func (e *FieldExpr) ExprTy() mir2.Ty { return e.Ty }

// AddrOfExpr takes the address of a global symbol or string literal.
type AddrOfExpr struct {
	Sym string // global or string symbol name
}

func (*AddrOfExpr) hirExpr()          {}
func (*AddrOfExpr) ExprTy() mir2.Ty   { return mir2.TyPtr }

// LoadExpr dereferences a pointer: *Ptr
type LoadExpr struct {
	Ptr Expr // must have type TyPtr
	Ty  mir2.Ty
}

func (*LoadExpr) hirExpr()          {}
func (e *LoadExpr) ExprTy() mir2.Ty { return e.Ty }

// DerefExpr is an lvalue dereference (used as AssignStmt.Target).
// Same as LoadExpr but semantically marks "write target".
type DerefExpr struct {
	Ptr Expr
	Ty  mir2.Ty
}

func (*DerefExpr) hirExpr()          {}
func (e *DerefExpr) ExprTy() mir2.Ty { return e.Ty }

// CastExpr converts a value to another type (zero-extend, sign-extend, truncate).
type CastExpr struct {
	X  Expr
	Ty mir2.Ty
}

func (*CastExpr) hirExpr()          {}
func (e *CastExpr) ExprTy() mir2.Ty { return e.Ty }

// ConstPtrExpr is a typed pointer to a compile-time constant address.
// Syntax: @ptr(T, addr) — used for memory-mapped I/O and AT variables.
// Lowered to OpConst(addr, TyPtr) in MIR2 → LD HL, addr on Z80.
type ConstPtrExpr struct {
	ElemTy mir2.Ty // element type (for documentation; codegen uses TyPtr)
	Addr   uint16
}

func (*ConstPtrExpr) hirExpr()          {}
func (*ConstPtrExpr) ExprTy() mir2.Ty   { return mir2.TyPtr }

// IndexExpr reads element i from an array pointer: base[i]
// Base must have type TyPtr (pointing to element type ElemTy).
// ElemStride is the byte size of each element (1 for u8, 2 for u16, etc.).
type IndexExpr struct {
	Base      Expr   // TyPtr — pointer to first element
	Idx       Expr   // TyU8 or TyU16 — index
	ElemTy    mir2.Ty
	ElemStride int   // bytes per element; 0 means derive from ElemTy.Width()/8
}

func (*IndexExpr) hirExpr()          {}
func (e *IndexExpr) ExprTy() mir2.Ty { return e.ElemTy }

// ── Convenience constructors ──────────────────────────────────────────────────

func U8(n int64) *IntLitExpr  { return &IntLitExpr{Val: n, Ty: mir2.TyU8} }
func U16(n int64) *IntLitExpr { return &IntLitExpr{Val: n, Ty: mir2.TyU16} }
func Bool(v bool) *BoolLitExpr { return &BoolLitExpr{Val: v} }
func Var(name string, ty mir2.Ty) *VarRefExpr { return &VarRefExpr{Name: name, Ty: ty} }
func Add(l, r Expr, ty mir2.Ty) *BinExpr { return &BinExpr{Op: "+", L: l, R: r, Ty: ty} }
func Sub(l, r Expr, ty mir2.Ty) *BinExpr { return &BinExpr{Op: "-", L: l, R: r, Ty: ty} }
func Mul(l, r Expr, ty mir2.Ty) *BinExpr { return &BinExpr{Op: "*", L: l, R: r, Ty: ty} }
func Lt(l, r Expr) *BinExpr              { return &BinExpr{Op: "<", L: l, R: r, Ty: mir2.TyBool} }
func Le(l, r Expr) *BinExpr              { return &BinExpr{Op: "<=", L: l, R: r, Ty: mir2.TyBool} }
func Gt(l, r Expr) *BinExpr              { return &BinExpr{Op: ">", L: l, R: r, Ty: mir2.TyBool} }
func Eq(l, r Expr) *BinExpr              { return &BinExpr{Op: "==", L: l, R: r, Ty: mir2.TyBool} }
func Ne(l, r Expr) *BinExpr              { return &BinExpr{Op: "!=", L: l, R: r, Ty: mir2.TyBool} }
func Call(fn string, ty mir2.Ty, args ...Expr) *CallExpr { return &CallExpr{Fn: fn, Args: args, Ty: ty} }
func Addr(sym string) *AddrOfExpr       { return &AddrOfExpr{Sym: sym} }
func Load(ptr Expr, ty mir2.Ty) *LoadExpr { return &LoadExpr{Ptr: ptr, Ty: ty} }
func Cast(x Expr, ty mir2.Ty) *CastExpr { return &CastExpr{X: x, Ty: ty} }

func Ret(val Expr) *ReturnStmt { return &ReturnStmt{Val: val} }
func RetVoid() *ReturnStmt     { return &ReturnStmt{} }
func Decl(name string, ty mir2.Ty, init Expr) *VarDeclStmt {
	return &VarDeclStmt{Name: name, Ty: ty, Init: init}
}
func Assign(target Expr, val Expr) *AssignStmt { return &AssignStmt{Target: target, Val: val} }
func If(cond Expr, then *Block, els *Block) *IfStmt {
	return &IfStmt{Cond: cond, Then: then, Else: els}
}
func While(cond Expr, body *Block) *WhileStmt { return &WhileStmt{Cond: cond, Body: body} }
func For(v string, start, end Expr, body *Block) *ForRangeStmt {
	return &ForRangeStmt{Var: v, Start: start, End: end, Body: body}
}
func Blk(stmts ...Stmt) *Block { return &Block{Body: stmts} }

func Break() *BreakStmt    { return &BreakStmt{} }
func Continue() *ContinueStmt { return &ContinueStmt{} }
func Switch(val Expr, cases []*SwitchCase, def *Block) *SwitchStmt {
	return &SwitchStmt{Val: val, Cases: cases, Default: def}
}
func Case(val int64, body *Block) *SwitchCase { return &SwitchCase{Val: val, Body: body} }

// Index reads arr[i] where arr is a TyPtr to elements of elemTy.
// Stride is derived from elemTy if stride == 0.
func Index(base, idx Expr, elemTy mir2.Ty) *IndexExpr {
	return &IndexExpr{Base: base, Idx: idx, ElemTy: elemTy}
}
