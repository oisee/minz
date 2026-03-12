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

// Assert is a module-level compile-time assertion.
//
//	assert fn(arg1, arg2) == expected           — check both MIR2 VM and Z80 binary
//	assert fn(arg1, arg2) == expected via mir2  — check MIR2 VM only (fast)
//	assert fn(arg1, arg2) == expected via z80   — check Z80 binary only
//
// Assertions are checked after MIR2 optimisation.  They are NOT emitted into
// the generated binary.
type Assert struct {
	FuncName       string   // function to call
	Args           []int64  // literal integer arguments
	Expected       int64    // expected return value (single-return)
	ExpectedMulti  []int64  // expected values for multi-return; non-nil overrides Expected
	Source         string   // original source text (for error messages)
	Line           int      // source line number
	Via            string   // "" = both MIR2+Z80; "mir2" = MIR2 VM only; "z80" = Z80 only
}

// Module is the top-level HIR unit.
type Module struct {
	Name       string
	Funcs      []*Func
	Globals    []mir2.Global      // reuse mir2.Global directly
	Structs    []*mir2.StructTy   // named struct type declarations
	Interfaces []*InterfaceDecl   // interface declarations (zero-cost, monomorphised)
	Strings    []string           // interned string literals (index = position)
	Warnings   []string           // use-before-init and other diagnostic warnings
	Asserts    []Assert           // compile-time assertions (checked via MIR2 VM)
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
	Name       string
	Params     []Param
	RetTy      mir2.Ty   // mir2.TyVoid for void functions (single-return)
	RetTys     []mir2.Ty // non-empty → multiple return values (overrides RetTy)
	Body       *Block    // nil for extern
	IsExtern   bool
	ExternAddr uint16    // non-zero → @extern(addr) — call via CALL/RST to fixed address
}

// Param is a typed, named function parameter.
type Param struct {
	Name     string
	Ty       mir2.Ty
	RegClass mir2.RegClass // 0 (ClassGeneral) = auto-assign; non-zero overrides classForParam
	SMC      bool          // @smc: baked as LD HL,imm16 immediate — not a register param
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

// ReturnStmt exits the current function.
//   - Single-return: Val is the (possibly nil for void) value.
//   - Multi-return:  Vals is non-empty (Val is nil); corresponds to Func.RetTys.
type ReturnStmt struct {
	Val  Expr   // single return value (nil for void or multi-return)
	Vals []Expr // multi-return values; non-empty overrides Val
}

func (*ReturnStmt) hirStmt() {}

// TupleLetStmt binds the multiple return values of a function call:
//
//	let (a, b) = minmax(x, y)
//
// Names and Tys correspond positionally to the callee's RetTys.
// Use "_" in Names to discard a position (like Go's blank identifier).
type TupleLetStmt struct {
	Names []string  // variable names; "_" = discard
	Tys   []mir2.Ty // type for each position (must match callee's RetTys)
	Call  *CallExpr // the multi-return call expression
}

func (*TupleLetStmt) hirStmt() {}

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

// AsmStmt is an inline assembly block.
//
//	asm z80 { LD A, $42 / OUT ($FE), A }         — clobber all (conservative)
//	asm z80 (in x @z80_a) clobbers(f) { ADD A, 1 } — explicit operands
//
// Target is the architecture tag ("z80", "ez80", "6502", ...).  An empty
// target matches the current compilation target.  A non-matching target block
// is silently skipped during lowering.
//
// Code is the raw assembly text, passed verbatim to the backend codegen.
// The "/" character may be used as an instruction separator (each "/" becomes
// a newline in the final output for readability).
//
// When ClobberAll is true (default when no explicit clobber list is given),
// every register class is considered clobbered.  This is conservative but safe.
// AsmOperand names a variable that the asm block reads (Ins) or writes (Outs).
// The lowerer resolves Name to the MIR2 virtual register currently bound in env.
type AsmOperand struct {
	Name string // variable name in scope
}

type AsmStmt struct {
	Target     string        // architecture tag, e.g. "z80"; "" = any
	Code       string        // verbatim assembly text
	ClobberAll bool          // true when no explicit clobber list
	Ins        []AsmOperand  // explicit input operands  — (in x, y)
	Outs       []AsmOperand  // explicit output operands — (out x)
}

func (*AsmStmt) hirStmt() {}

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

// FieldInit is one field initializer inside a StructLitExpr.
type FieldInit struct {
	Name string
	Val  Expr
}

// StructLitExpr constructs a struct value inline: Color{r: 255, g: 0, b: 0}
// Lowered to a sequence of field stores.  When used as an expression value it
// allocates a temporary on the stack and returns a pointer (TyPtr) to it.
// When used directly as the RHS of an assignment whose target is a DerefExpr
// or a global VarRefExpr the lowerer writes fields in-place (no alloca needed).
type StructLitExpr struct {
	St     *mir2.StructTy
	Fields []FieldInit
}

func (*StructLitExpr) hirExpr()          {}
func (*StructLitExpr) ExprTy() mir2.Ty   { return mir2.TyPtr }

// RangeSourceExpr is produced by range(lo..hi) / range(hi..lo).
// It is a pure counter source — no memory pointer, no Load.
// Rev=true means hi < lo in source (e.g. range(n..0)): counts from Lo down to Hi+1.
// When used as an iterator source the element value equals the current counter value.
type RangeSourceExpr struct {
	Lo, Hi Expr
	Rev    bool // true when written as range(hi..lo), i.e. counting down
}

func (*RangeSourceExpr) hirExpr()        {}
func (*RangeSourceExpr) ExprTy() mir2.Ty { return mir2.TyU8 }

// CondExpr is an if-as-expression: if Cond { Then } else { Else }.
// Both branches must produce the same type Ty.
// Used for: let x = if c { a } else { b }
type CondExpr struct {
	Cond Expr
	Then Expr // value from then-branch
	Else Expr // value from else-branch
	Ty   mir2.Ty
}

func (*CondExpr) hirExpr()          {}
func (e *CondExpr) ExprTy() mir2.Ty { return e.Ty }

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

func Ret(val Expr) *ReturnStmt        { return &ReturnStmt{Val: val} }
func RetVoid() *ReturnStmt            { return &ReturnStmt{} }
func RetMulti(vals ...Expr) *ReturnStmt { return &ReturnStmt{Vals: vals} }
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
