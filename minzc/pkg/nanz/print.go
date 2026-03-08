// Package nanz implements Nanz — a tiny, HIR-native language with MinZ-like syntax.
//
// Nanz ("nano MinZ") is designed to map 1:1 to HIR constructs. It serves two
// purposes:
//
//  1. Standalone language: write programs that compile directly to HIR → MIR2 → Z80.
//  2. Transpilation target: PL/M-80 source can be converted to human-readable Nanz
//     via the PL/M→HIR lowerer + this printer.
//
// Nanz syntax highlights:
//
//	^T            — pointer-to-T type
//	[T; N]        — array of N elements of type T
//	^expr         — dereference a pointer (load)
//	&x            — address of variable x
//	arr[i]        — index an array/pointer (sugar for ^(arr + i))
//	at(addr)      — place variable at absolute address (memory-mapped I/O)
//	= [1,2,3]     — array initializer
//
// Pipeline:
//
//	Nanz source → Parse() → hir.Module
//	hir.Module  → Print() → Nanz source
//	plm.Module  → plm.LowerModule() → hir.Module → Print() → Nanz source
package nanz

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Print renders a HIR module as Nanz source text.
func Print(m *hir.Module) string {
	var b strings.Builder
	p := &printer{b: &b}
	p.module(m)
	return b.String()
}

type printer struct {
	b      *strings.Builder
	indent int
}

func (p *printer) nl()              { p.b.WriteByte('\n') }
func (p *printer) tab()            { p.b.WriteString(strings.Repeat("    ", p.indent)) }
func (p *printer) write(s string)  { p.b.WriteString(s) }
func (p *printer) writef(f string, args ...any) { p.write(fmt.Sprintf(f, args...)) }
func (p *printer) line(s string)   { p.tab(); p.write(s); p.nl() }
func (p *printer) linef(f string, args ...any) { p.tab(); p.writef(f, args...); p.nl() }

// ── Module ────────────────────────────────────────────────────────────────────

func (p *printer) module(m *hir.Module) {
	// Struct declarations
	for _, st := range m.Structs {
		p.structDecl(st)
		p.nl()
	}

	// Global variables
	for _, g := range m.Globals {
		p.global(g)
	}
	if len(m.Globals) > 0 {
		p.nl()
	}

	// Functions
	for i, f := range m.Funcs {
		if i > 0 {
			p.nl()
		}
		p.function(f)
	}
}

func (p *printer) structDecl(st *mir2.StructTy) {
	p.writef("struct %s {\n", st.Name)
	for _, f := range st.Fields {
		p.writef("    %s: %s\n", f.Name, tyStr(f.Ty))
	}
	p.write("}\n")
}

func (p *printer) global(g mir2.Global) {
	p.write("global ")
	p.write(g.Name)
	p.write(": ")
	p.write(tyStr(g.Ty))
	if g.At != nil {
		p.writef(" at(0x%04X)", *g.At)
	}
	if len(g.Init) > 0 {
		switch ty := g.Ty.(type) {
		case *mir2.ArrayTy:
			p.write(" = [")
			ew := mir2.ByteWidth(ty.Elem)
			for i := 0; i < ty.Len && i*ew < len(g.Init); i++ {
				if i > 0 {
					p.write(", ")
				}
				p.writeInitBytes(g.Init[i*ew:(i+1)*ew], ty.Elem)
			}
			p.write("]")
		default:
			p.write(" = ")
			p.writeInitBytes(g.Init, g.Ty)
		}
	}
	p.nl()
}

func (p *printer) writeInitBytes(data []byte, ty mir2.Ty) {
	switch ty.Width() {
	case 8:
		p.writef("%d", data[0])
	case 16:
		if len(data) >= 2 {
			v := uint16(data[0]) | uint16(data[1])<<8
			p.writef("%d", v)
		}
	default:
		p.writef("0x%X", data)
	}
}

// ── Function ──────────────────────────────────────────────────────────────────

func (p *printer) function(f *hir.Func) {
	if f.IsExtern {
		p.write("@extern ")
	}
	p.write("fun ")
	p.write(f.Name)
	p.write("(")
	for i, param := range f.Params {
		if i > 0 {
			p.write(", ")
		}
		p.writef("%s: %s", param.Name, tyStr(param.Ty))
	}
	p.write(")")
	if f.RetTy != mir2.TyVoid {
		p.writef(" -> %s", tyStr(f.RetTy))
	}
	if f.Body == nil {
		p.nl()
		return
	}
	p.write(" {\n")
	p.indent++
	for _, s := range f.Body.Body {
		p.stmt(s)
	}
	p.indent--
	p.write("}\n")
}

// ── Statements ────────────────────────────────────────────────────────────────

func (p *printer) stmt(s hir.Stmt) {
	switch s := s.(type) {
	case *hir.VarDeclStmt:
		p.varDecl(s)
	case *hir.AssignStmt:
		p.tab()
		p.expr(s.Target)
		p.write(" = ")
		p.expr(s.Val)
		p.nl()
	case *hir.StoreStmt:
		// ptr^ = val  (postfix ^)
		p.tab()
		p.exprParen(s.Ptr)
		p.write("^ = ")
		p.expr(s.Val)
		p.nl()
	case *hir.ReturnStmt:
		if s.Val == nil {
			p.line("return")
		} else {
			p.tab()
			p.write("return ")
			p.expr(s.Val)
			p.nl()
		}
	case *hir.IfStmt:
		p.tab()
		p.write("if ")
		p.expr(s.Cond)
		p.write(" {\n")
		p.indent++
		for _, bs := range s.Then.Body {
			p.stmt(bs)
		}
		p.indent--
		if s.Else != nil && len(s.Else.Body) > 0 {
			p.line("} else {")
			p.indent++
			for _, bs := range s.Else.Body {
				p.stmt(bs)
			}
			p.indent--
		}
		p.line("}")
	case *hir.WhileStmt:
		p.tab()
		p.write("while ")
		p.expr(s.Cond)
		p.write(" {\n")
		p.indent++
		for _, bs := range s.Body.Body {
			p.stmt(bs)
		}
		p.indent--
		p.line("}")
	case *hir.ForRangeStmt:
		p.tab()
		p.writef("for %s in ", s.Var)
		p.expr(s.Start)
		p.write("..")
		p.expr(s.End)
		p.write(" {\n")
		p.indent++
		for _, bs := range s.Body.Body {
			p.stmt(bs)
		}
		p.indent--
		p.line("}")
	case *hir.ExprStmt:
		p.tab()
		p.expr(s.Expr)
		p.nl()
	case *hir.BreakStmt:
		p.line("break")
	case *hir.ContinueStmt:
		p.line("continue")
	case *hir.SwitchStmt:
		p.tab()
		p.write("switch ")
		p.expr(s.Val)
		p.write(" {\n")
		p.indent++
		for _, c := range s.Cases {
			p.linef("case %d:", c.Val)
			p.indent++
			for _, bs := range c.Body.Body {
				p.stmt(bs)
			}
			p.indent--
		}
		if s.Default != nil {
			p.line("default:")
			p.indent++
			for _, bs := range s.Default.Body {
				p.stmt(bs)
			}
			p.indent--
		}
		p.indent--
		p.line("}")
	case *hir.Block:
		p.line("{")
		p.indent++
		for _, bs := range s.Body {
			p.stmt(bs)
		}
		p.indent--
		p.line("}")
	default:
		p.linef("/* unhandled stmt %T */", s)
	}
}

func (p *printer) varDecl(s *hir.VarDeclStmt) {
	p.tab()
	p.write("var ")
	p.write(s.Name)
	p.write(": ")
	if s.ArrayLen > 0 {
		p.writef("[%s; %d]", tyStr(s.Ty), s.ArrayLen)
	} else {
		p.write(tyStr(s.Ty))
	}
	if s.At != nil {
		p.writef(" at(0x%04X)", *s.At)
	}
	if len(s.Initial) > 0 {
		p.write(" = [")
		for i, e := range s.Initial {
			if i > 0 {
				p.write(", ")
			}
			p.expr(e)
		}
		p.write("]")
	} else if s.Init != nil {
		p.write(" = ")
		p.expr(s.Init)
	}
	p.nl()
}

// ── Expressions ───────────────────────────────────────────────────────────────

func (p *printer) expr(e hir.Expr) {
	switch e := e.(type) {
	case *hir.IntLitExpr:
		p.writef("%d", e.Val)
	case *hir.BoolLitExpr:
		if e.Val {
			p.write("true")
		} else {
			p.write("false")
		}
	case *hir.VarRefExpr:
		p.write(e.Name)
	case *hir.BinExpr:
		p.exprParen(e.L)
		p.writef(" %s ", e.Op)
		p.exprParen(e.R)
	case *hir.UnaryExpr:
		p.write(e.Op)
		p.exprParen(e.X)
	case *hir.CallExpr:
		p.write(e.Fn)
		p.write("(")
		for i, a := range e.Args {
			if i > 0 {
				p.write(", ")
			}
			p.expr(a)
		}
		p.write(")")
	case *hir.ConstPtrExpr:
		p.writef("@ptr(%s, 0x%04X)", tyStr(e.ElemTy), e.Addr)
	case *hir.AddrOfExpr:
		p.writef("&%s", e.Sym)
	case *hir.LoadExpr:
		// ptr^  (postfix ^)
		p.exprParen(e.Ptr)
		p.write("^")
	case *hir.DerefExpr:
		p.exprParen(e.Ptr)
		p.write("^")
	case *hir.CastExpr:
		p.writef("%s(", tyStr(e.Ty))
		p.expr(e.X)
		p.write(")")
	case *hir.IndexExpr:
		p.exprParen(e.Base)
		p.write("[")
		p.expr(e.Idx)
		p.write("]")
	case *hir.FieldExpr:
		p.exprParen(e.X)
		p.writef(".%s", e.Field)
	default:
		p.writef("/* expr %T */", e)
	}
}

// exprParen wraps compound expressions in parens for clarity.
func (p *printer) exprParen(e hir.Expr) {
	switch e := e.(type) {
	case *hir.BinExpr:
		p.write("(")
		p.expr(e)
		p.write(")")
	default:
		p.expr(e)
	}
}

// ── Type printing ─────────────────────────────────────────────────────────────

func tyStr(ty mir2.Ty) string {
	switch ty := ty.(type) {
	case *mir2.ArrayTy:
		return fmt.Sprintf("[%s; %d]", tyStr(ty.Elem), ty.Len)
	default:
		return ty.String()
	}
}
