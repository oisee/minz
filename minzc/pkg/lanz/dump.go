package lanz

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Dump renders a HIR module as Lanz S-expressions (round-trippable).
func Dump(m *hir.Module) string {
	var sb strings.Builder
	sb.WriteString("; Lanz module: " + m.Name + "\n")
	for _, st := range m.Structs {
		dumpStruct(&sb, st)
		sb.WriteByte('\n')
	}
	for _, g := range m.Globals {
		dumpGlobal(&sb, g)
		sb.WriteByte('\n')
	}
	for _, f := range m.Funcs {
		dumpFunc(&sb, f)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func dumpStruct(sb *strings.Builder, st *mir2.StructTy) {
	sb.WriteString("(struct ")
	sb.WriteString(st.Name)
	sb.WriteString(" (")
	for i, f := range st.Fields {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(sb, "(%s %s)", f.Name, tyStr(f.Ty))
	}
	sb.WriteString("))")
}

func dumpGlobal(sb *strings.Builder, g mir2.Global) {
	sb.WriteString("(global ")
	sb.WriteString(g.Name)
	sb.WriteByte(' ')
	sb.WriteString(tyStr(g.Ty))
	if len(g.Init) > 0 {
		v := int64(g.Init[0])
		if len(g.Init) >= 2 {
			v = int64(g.Init[0]) | int64(g.Init[1])<<8
		}
		fmt.Fprintf(sb, " %d", v)
	}
	sb.WriteByte(')')
}

func dumpFunc(sb *strings.Builder, f *hir.Func) {
	if f.IsExtern {
		sb.WriteString("(extern ")
	} else {
		sb.WriteString("(fun ")
	}
	sb.WriteString(f.Name)
	sb.WriteString(" (")
	for i, p := range f.Params {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(sb, "(%s %s)", p.Name, tyStr(p.Ty))
	}
	sb.WriteString(") ")
	sb.WriteString(tyStr(f.RetTy))
	if f.IsExtern {
		if f.ExternAddr != 0 {
			fmt.Fprintf(sb, " 0x%04X", f.ExternAddr)
		}
		sb.WriteByte(')')
		return
	}
	if f.Body != nil {
		for _, s := range f.Body.Body {
			sb.WriteByte('\n')
			sb.WriteString("  ")
			dumpStmt(sb, s, 1)
		}
	}
	sb.WriteByte(')')
}

func dumpStmt(sb *strings.Builder, s hir.Stmt, depth int) {
	indent := strings.Repeat("  ", depth)
	switch s := s.(type) {
	case *hir.VarDeclStmt:
		sb.WriteString("(var ")
		sb.WriteString(s.Name)
		sb.WriteByte(' ')
		sb.WriteString(tyStr(s.Ty))
		if s.Init != nil {
			sb.WriteByte(' ')
			dumpExpr(sb, s.Init)
		}
		sb.WriteByte(')')
	case *hir.AssignStmt:
		sb.WriteString("(set ")
		dumpExpr(sb, s.Target)
		sb.WriteByte(' ')
		dumpExpr(sb, s.Val)
		sb.WriteByte(')')
	case *hir.ReturnStmt:
		if s.Val == nil {
			sb.WriteString("(return)")
		} else {
			sb.WriteString("(return ")
			dumpExpr(sb, s.Val)
			sb.WriteByte(')')
		}
	case *hir.IfStmt:
		sb.WriteString("(if ")
		dumpExpr(sb, s.Cond)
		sb.WriteByte('\n')
		sb.WriteString(indent + "  ")
		dumpBlock(sb, s.Then, depth+1)
		if s.Else != nil {
			sb.WriteByte('\n')
			sb.WriteString(indent + "  ")
			dumpBlock(sb, s.Else, depth+1)
		}
		sb.WriteByte(')')
	case *hir.WhileStmt:
		sb.WriteString("(while ")
		dumpExpr(sb, s.Cond)
		for _, bs := range s.Body.Body {
			sb.WriteByte('\n')
			sb.WriteString(indent + "  ")
			dumpStmt(sb, bs, depth+1)
		}
		sb.WriteByte(')')
	case *hir.ForRangeStmt:
		sb.WriteString("(for ")
		sb.WriteString(s.Var)
		sb.WriteByte(' ')
		dumpExpr(sb, s.Start)
		sb.WriteByte(' ')
		dumpExpr(sb, s.End)
		for _, bs := range s.Body.Body {
			sb.WriteByte('\n')
			sb.WriteString(indent + "  ")
			dumpStmt(sb, bs, depth+1)
		}
		sb.WriteByte(')')
	case *hir.ExprStmt:
		dumpExpr(sb, s.Expr)
	case *hir.StoreStmt:
		sb.WriteString("(store ")
		dumpExpr(sb, s.Ptr)
		sb.WriteByte(' ')
		dumpExpr(sb, s.Val)
		sb.WriteByte(')')
	case *hir.Block:
		dumpBlock(sb, s, depth)
	case *hir.BreakStmt:
		sb.WriteString("(break)")
	case *hir.ContinueStmt:
		sb.WriteString("(continue)")
	case *hir.SwitchStmt:
		sb.WriteString("(switch ")
		dumpExpr(sb, s.Val)
		for _, c := range s.Cases {
			sb.WriteByte('\n')
			sb.WriteString(indent + "  ")
			fmt.Fprintf(sb, "(case %d", c.Val)
			for _, bs := range c.Body.Body {
				sb.WriteByte('\n')
				sb.WriteString(indent + "    ")
				dumpStmt(sb, bs, depth+2)
			}
			sb.WriteByte(')')
		}
		if s.Default != nil {
			sb.WriteByte('\n')
			sb.WriteString(indent + "  ")
			sb.WriteString("(default")
			for _, bs := range s.Default.Body {
				sb.WriteByte('\n')
				sb.WriteString(indent + "    ")
				dumpStmt(sb, bs, depth+2)
			}
			sb.WriteByte(')')
		}
		sb.WriteByte(')')
	default:
		fmt.Fprintf(sb, "(?stmt %T)", s)
	}
}

func dumpBlock(sb *strings.Builder, b *hir.Block, depth int) {
	indent := strings.Repeat("  ", depth)
	sb.WriteString("(block")
	for _, s := range b.Body {
		sb.WriteByte('\n')
		sb.WriteString(indent + "  ")
		dumpStmt(sb, s, depth+1)
	}
	sb.WriteByte(')')
}

func dumpExpr(sb *strings.Builder, e hir.Expr) {
	switch e := e.(type) {
	case *hir.IntLitExpr:
		fmt.Fprintf(sb, "%d", e.Val)
	case *hir.BoolLitExpr:
		if e.Val {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case *hir.VarRefExpr:
		sb.WriteString(e.Name)
	case *hir.BinExpr:
		fmt.Fprintf(sb, "(%s ", e.Op)
		dumpExpr(sb, e.L)
		sb.WriteByte(' ')
		dumpExpr(sb, e.R)
		sb.WriteByte(')')
	case *hir.UnaryExpr:
		switch e.Op {
		case "-":
			sb.WriteString("(neg ")
		case "!":
			sb.WriteString("(not ")
		case "~":
			sb.WriteString("(bitnot ")
		default:
			fmt.Fprintf(sb, "(%s ", e.Op)
		}
		dumpExpr(sb, e.X)
		sb.WriteByte(')')
	case *hir.CallExpr:
		sb.WriteByte('(')
		sb.WriteString(e.Fn)
		for _, a := range e.Args {
			sb.WriteByte(' ')
			dumpExpr(sb, a)
		}
		sb.WriteByte(')')
	case *hir.LoadExpr:
		sb.WriteString("(load ")
		dumpExpr(sb, e.Ptr)
		if e.Ty != mir2.TyU8 {
			sb.WriteByte(' ')
			sb.WriteString(tyStr(e.Ty))
		}
		sb.WriteByte(')')
	case *hir.AddrOfExpr:
		sb.WriteString("(addr ")
		sb.WriteString(e.Sym)
		sb.WriteByte(')')
	case *hir.CastExpr:
		sb.WriteString("(cast ")
		dumpExpr(sb, e.X)
		sb.WriteByte(' ')
		sb.WriteString(tyStr(e.Ty))
		sb.WriteByte(')')
	case *hir.IndexExpr:
		sb.WriteString("(index ")
		dumpExpr(sb, e.Base)
		sb.WriteByte(' ')
		dumpExpr(sb, e.Idx)
		if e.ElemTy != mir2.TyU8 {
			sb.WriteByte(' ')
			sb.WriteString(tyStr(e.ElemTy))
		}
		sb.WriteByte(')')
	case *hir.CondExpr:
		sb.WriteString("(if ")
		dumpExpr(sb, e.Cond)
		sb.WriteByte(' ')
		dumpExpr(sb, e.Then)
		sb.WriteByte(' ')
		dumpExpr(sb, e.Else)
		sb.WriteByte(')')
	case *hir.RangeSourceExpr:
		sb.WriteString("(range ")
		dumpExpr(sb, e.Lo)
		sb.WriteByte(' ')
		dumpExpr(sb, e.Hi)
		sb.WriteByte(')')
	case *hir.FieldExpr:
		fmt.Fprintf(sb, "(field ")
		dumpExpr(sb, e.X)
		fmt.Fprintf(sb, " %s %d)", e.Field, e.Offset)
	default:
		fmt.Fprintf(sb, "(?expr %T)", e)
	}
}

func tyStr(ty mir2.Ty) string {
	switch ty {
	case mir2.TyU8:
		return "u8"
	case mir2.TyU16:
		return "u16"
	case mir2.TyI8:
		return "i8"
	case mir2.TyI16:
		return "i16"
	case mir2.TyBool:
		return "bool"
	case mir2.TyVoid:
		return "void"
	case mir2.TyPtr:
		return "ptr"
	default:
		return fmt.Sprintf("ty%d", ty)
	}
}
