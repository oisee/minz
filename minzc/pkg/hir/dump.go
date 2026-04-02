package hir

import (
	"fmt"
	"strings"
)

// Dump returns a structured textual representation of the HIR module.
//
// Unlike --emit=nanz (which round-trips to surface syntax), Dump shows the
// internal structure: types on every node, explicit control-flow constructs,
// and typed variable declarations.  Useful for debugging the HIR→MIR2
// lowering stage.
//
// Example:
//
//	; HIR module: report_demo
//
//	fun @ABS_DIFF(A: u8, B: u8) -> u8
//	  if (A: u8) > (B: u8) : bool
//	    return (A: u8) - (B: u8) : u8
//	  else
//	    return (B: u8) - (A: u8) : u8
//
//	fun @FIB(N: u8) -> u8
//	  var A: u8 = 0
//	  var B: u8 = 1
//	  while (N: u8) > 0 : bool
//	    var T: u8 = (B: u8)
//	    B = (A: u8) + (B: u8) : u8
//	    A = (T: u8)
//	    N = (N: u8) - 1 : u8
//	  return (A: u8)
func (m *Module) Dump() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "; HIR module: %s\n", m.Name)

	for _, s := range m.Structs {
		fmt.Fprintf(&sb, "struct @%s { ", s.Name)
		for i, f := range s.Fields {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s: %s", f.Name, f.Ty)
		}
		sb.WriteString(" }\n")
	}
	for _, g := range m.Globals {
		qual := ""
		if g.IsConst {
			qual = "const "
		}
		fmt.Fprintf(&sb, "global %s@%s : %s\n", qual, g.Name, g.Ty)
	}
	for i, s := range m.Strings {
		fmt.Fprintf(&sb, "string #%d = %q\n", i, s)
	}
	if len(m.Structs) > 0 || len(m.Globals) > 0 || len(m.Strings) > 0 {
		sb.WriteByte('\n')
	}

	for _, f := range m.Funcs {
		dumpFunc(&sb, f)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func dumpFunc(sb *strings.Builder, f *Func) {
	if f.IsExtern {
		sb.WriteString("extern ")
	}
	sb.WriteString("fun @")
	sb.WriteString(f.Name)
	sb.WriteString("(")
	for i, p := range f.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%s: %s", p.Name, p.Ty)
	}
	sb.WriteString(")")
	if f.RetTy.String() != "void" {
		fmt.Fprintf(sb, " -> %s", f.RetTy)
	}
	sb.WriteByte('\n')
	if f.Body != nil {
		dumpBlock(sb, f.Body, "  ")
	}
}

func dumpBlock(sb *strings.Builder, b *Block, indent string) {
	for _, s := range b.Body {
		dumpStmt(sb, s, indent)
	}
}

func dumpStmt(sb *strings.Builder, s Stmt, indent string) {
	switch st := s.(type) {
	case *VarDeclStmt:
		if st.ArrayLen > 0 {
			fmt.Fprintf(sb, "%svar %s: [%d]%s", indent, st.Name, st.ArrayLen, st.Ty)
			if len(st.Initial) > 0 {
				sb.WriteString(" = {")
				for i, e := range st.Initial {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(dumpExpr(e))
				}
				sb.WriteByte('}')
			}
		} else {
			fmt.Fprintf(sb, "%svar %s: %s", indent, st.Name, st.Ty)
			if st.Init != nil {
				fmt.Fprintf(sb, " = %s", dumpExpr(st.Init))
			}
			if st.At != nil {
				fmt.Fprintf(sb, " at 0x%04X", *st.At)
			}
		}
		sb.WriteByte('\n')

	case *AssignStmt:
		fmt.Fprintf(sb, "%s%s = %s\n", indent, dumpExpr(st.Target), dumpExpr(st.Val))

	case *ReturnStmt:
		if len(st.Vals) > 0 {
			strs := make([]string, len(st.Vals))
			for i, v := range st.Vals {
				strs[i] = dumpExpr(v)
			}
			fmt.Fprintf(sb, "%sreturn (%s)\n", indent, strings.Join(strs, ", "))
		} else if st.Val != nil {
			fmt.Fprintf(sb, "%sreturn %s\n", indent, dumpExpr(st.Val))
		} else {
			fmt.Fprintf(sb, "%sreturn\n", indent)
		}

	case *TupleLetStmt:
		fmt.Fprintf(sb, "%slet (%s) = %s(...)\n", indent, strings.Join(st.Names, ", "), st.Call.Fn)

	case *IfStmt:
		fmt.Fprintf(sb, "%sif %s\n", indent, dumpExpr(st.Cond))
		dumpBlock(sb, st.Then, indent+"  ")
		if st.Else != nil {
			fmt.Fprintf(sb, "%selse\n", indent)
			dumpBlock(sb, st.Else, indent+"  ")
		}

	case *WhileStmt:
		fmt.Fprintf(sb, "%swhile %s\n", indent, dumpExpr(st.Cond))
		dumpBlock(sb, st.Body, indent+"  ")

	case *ForRangeStmt:
		fmt.Fprintf(sb, "%sfor %s in %s..%s\n", indent, st.Var, dumpExpr(st.Start), dumpExpr(st.End))
		dumpBlock(sb, st.Body, indent+"  ")

	case *ForEachStmt:
		fmt.Fprintf(sb, "%sforeach %s: %s in *(%s)[0..%s]\n",
			indent, st.Var, st.ElemTy, dumpExpr(st.Ptr), dumpExpr(st.Len))
		dumpBlock(sb, st.Body, indent+"  ")

	case *ExprStmt:
		fmt.Fprintf(sb, "%s%s\n", indent, dumpExpr(st.Expr))

	case *StoreStmt:
		fmt.Fprintf(sb, "%s*(%s) = %s\n", indent, dumpExpr(st.Ptr), dumpExpr(st.Val))

	case *SwitchStmt:
		fmt.Fprintf(sb, "%sswitch %s\n", indent, dumpExpr(st.Val))
		for _, c := range st.Cases {
			fmt.Fprintf(sb, "%s  case %d:\n", indent, c.Val)
			dumpBlock(sb, c.Body, indent+"    ")
		}
		if st.Default != nil {
			fmt.Fprintf(sb, "%s  default:\n", indent)
			dumpBlock(sb, st.Default, indent+"    ")
		}

	case *Block:
		dumpBlock(sb, st, indent)

	case *BreakStmt:
		fmt.Fprintf(sb, "%sbreak\n", indent)

	case *ContinueStmt:
		fmt.Fprintf(sb, "%scontinue\n", indent)

	default:
		fmt.Fprintf(sb, "%s<unknown stmt %T>\n", indent, s)
	}
}

func dumpExpr(e Expr) string {
	switch ex := e.(type) {
	case *IntLitExpr:
		return fmt.Sprintf("%d:%s", ex.Val, ex.Ty)

	case *BoolLitExpr:
		if ex.Val {
			return "true:bool"
		}
		return "false:bool"

	case *VarRefExpr:
		return fmt.Sprintf("(%s:%s)", ex.Name, ex.Ty)

	case *BinExpr:
		return fmt.Sprintf("(%s %s %s):%s", dumpExpr(ex.L), ex.Op, dumpExpr(ex.R), ex.Ty)

	case *UnaryExpr:
		return fmt.Sprintf("(%s%s):%s", ex.Op, dumpExpr(ex.X), ex.Ty)

	case *CallExpr:
		args := make([]string, len(ex.Args))
		for i, a := range ex.Args {
			args[i] = dumpExpr(a)
		}
		return fmt.Sprintf("call @%s(%s):%s", ex.Fn, strings.Join(args, ", "), ex.Ty)

	case *FieldExpr:
		return fmt.Sprintf("(%s).%s[+%d]:%s", dumpExpr(ex.X), ex.Field, ex.Offset, ex.Ty)

	case *AddrOfExpr:
		if ex.X != nil {
			return fmt.Sprintf("addr(%s):ptr", dumpExpr(ex.X))
		}
		return fmt.Sprintf("addr @%s:ptr", ex.Sym)

	case *LoadExpr:
		return fmt.Sprintf("load(%s):%s", dumpExpr(ex.Ptr), ex.Ty)

	case *DerefExpr:
		return fmt.Sprintf("*(%s):%s", dumpExpr(ex.Ptr), ex.Ty)

	case *CastExpr:
		return fmt.Sprintf("cast(%s):%s", dumpExpr(ex.X), ex.Ty)

	case *ConstPtrExpr:
		return fmt.Sprintf("@ptr(%s, 0x%04X):ptr", ex.ElemTy, ex.Addr)

	case *IndexExpr:
		return fmt.Sprintf("(%s)[%s]:%s", dumpExpr(ex.Base), dumpExpr(ex.Idx), ex.ElemTy)

	case *CondExpr:
		return fmt.Sprintf("(if %s then %s else %s):%s", dumpExpr(ex.Cond), dumpExpr(ex.Then), dumpExpr(ex.Else), ex.Ty)

	case *LetInExpr:
		return fmt.Sprintf("(let %s:%s = %s in %s)", ex.Name, ex.Ty, dumpExpr(ex.Init), dumpExpr(ex.Body))

	case nil:
		return "<nil>"

	default:
		return fmt.Sprintf("<unknown expr %T>", e)
	}
}
