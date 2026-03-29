// Package mir2llvm translates MIR2 modules to LLVM IR text.
//
// LLVM IR is SSA-based, like MIR2, so translation is nearly 1:1.
// Output can be compiled via `llc` or `clang` to native code for
// any LLVM-supported target (x86_64, ARM64, RISC-V, WASM, etc.)
//
// Primary uses:
//   - Native execution for testing/benchmarking
//   - Correctness oracle (alongside QBE + WASM)
//   - Access to LLVM's optimization passes
//   - Cross-compilation to targets MinZ doesn't have native backends for
//
// Type mapping:
//   TyBool       → i1 (promoted to i8 in memory)
//   TyU8, TyI8   → i8
//   TyU16, TyI16  → i16
//   TyPtr         → ptr (opaque pointer, LLVM 15+)
//   TyVoid        → void
//
// MIR2 block params → LLVM phi nodes (same as QBE backend).
package mir2llvm

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// Compile translates a MIR2 module to LLVM IR text (.ll format).
func Compile(m *mir2.Module) (string, error) {
	g := &gen{sb: &strings.Builder{}, mod: m}
	g.emitModule()
	return g.sb.String(), nil
}

type gen struct {
	sb  *strings.Builder
	mod *mir2.Module

	// Per-function state
	preds map[string][][]predEdge // block → param → [(predLabel, argReg)]
}

type predEdge struct {
	label string
	reg   mir2.Reg
}

func (g *gen) emitModule() {
	g.sb.WriteString("; ModuleID = 'minz'\n")
	g.sb.WriteString("source_filename = \"minz\"\n")
	g.sb.WriteString("target datalayout = \"e-m:e-i8:8:32-i16:16:32-i64:64-n32:64-S128\"\n\n")

	// Global variables
	for _, gl := range g.mod.Globals {
		w := mir2.ByteWidth(gl.Ty)
		if w == 0 {
			w = 2
		}
		ty := llvmArrayType(w)
		name := llvmSym(gl.Name)
		g.sb.WriteString(fmt.Sprintf("@%s = global %s zeroinitializer\n", name, ty))
	}
	if len(g.mod.Globals) > 0 {
		g.sb.WriteString("\n")
	}

	// String constants
	for i := 0; i < g.mod.Strings.Len(); i++ {
		s := g.mod.Strings.At(i)
		sym := llvmSym(g.mod.Strings.Symbol(i))
		g.sb.WriteString(fmt.Sprintf("@%s = private constant [%d x i8] c\"%s\\00\"\n",
			sym, len(s)+1, llvmEscapeString(s)))
	}
	if g.mod.Strings.Len() > 0 {
		g.sb.WriteString("\n")
	}

	// Functions
	for _, f := range g.mod.Funcs {
		g.emitFunc(f)
	}
}

func (g *gen) emitFunc(f *mir2.Func) {
	// Build predecessor map for phi nodes
	g.preds = buildPredMap(f)

	retTy := llvmType(contractRetTy(f))

	var params []string
	for _, p := range f.Contract.Params {
		params = append(params, fmt.Sprintf("%s %%%d", llvmType(p.Ty), p.Reg))
	}

	name := llvmSym(f.Name)
	g.sb.WriteString(fmt.Sprintf("define %s @%s(%s) {\n",
		retTy, name, strings.Join(params, ", ")))

	// Emit blocks
	for _, b := range f.Blocks {
		g.emitBlock(b, f)
	}

	g.sb.WriteString("}\n\n")
}

func (g *gen) emitBlock(b *mir2.Block, f *mir2.Func) {
	label := llvmLabel(b.Label)
	g.sb.WriteString(fmt.Sprintf("%s:\n", label))

	// Phi nodes for block params
	for pi, bp := range b.Params {
		if preds, ok := g.preds[b.Label]; ok && pi < len(preds) {
			ty := llvmType(bp.Ty)
			g.sb.WriteString(fmt.Sprintf("  %%%d = phi %s", bp.Dst, ty))
			for j, pe := range preds[pi] {
				if j > 0 {
					g.sb.WriteString(",")
				}
				g.sb.WriteString(fmt.Sprintf(" [ %%%d, %%%s ]", pe.reg, llvmLabel(pe.label)))
			}
			g.sb.WriteString("\n")
		}
	}

	// Instructions
	for _, inst := range b.Insts {
		g.emitInst(inst)
	}

	// Terminator
	g.emitTerm(b.Term, f)
}

func (g *gen) emitInst(inst *mir2.Inst) {
	dst := inst.Dst
	src0 := inst.Src[0]
	src1 := inst.Src[1]
	ty := llvmType(inst.Ty)

	switch inst.Op {
	case mir2.OpConst:
		g.sb.WriteString(fmt.Sprintf("  %%%d = add %s 0, %d\n", dst, ty, inst.Imm))

	case mir2.OpMove:
		g.sb.WriteString(fmt.Sprintf("  %%%d = add %s 0, %%%d\n", dst, ty, src0))

	case mir2.OpAdd:
		g.sb.WriteString(fmt.Sprintf("  %%%d = add %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpSub:
		g.sb.WriteString(fmt.Sprintf("  %%%d = sub %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpMul:
		g.sb.WriteString(fmt.Sprintf("  %%%d = mul %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpAnd:
		g.sb.WriteString(fmt.Sprintf("  %%%d = and %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpOr:
		g.sb.WriteString(fmt.Sprintf("  %%%d = or %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpXor:
		g.sb.WriteString(fmt.Sprintf("  %%%d = xor %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpShl:
		g.sb.WriteString(fmt.Sprintf("  %%%d = shl %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpShr:
		g.sb.WriteString(fmt.Sprintf("  %%%d = lshr %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpSar:
		g.sb.WriteString(fmt.Sprintf("  %%%d = ashr %s %%%d, %%%d\n", dst, ty, src0, src1))

	case mir2.OpNeg:
		g.sb.WriteString(fmt.Sprintf("  %%%d = sub %s 0, %%%d\n", dst, ty, src0))

	case mir2.OpNot:
		g.sb.WriteString(fmt.Sprintf("  %%%d = xor %s %%%d, -1\n", dst, ty, src0))

	case mir2.OpCmp:
		pred := llvmCmpPred(inst.Cond)
		// CMP result is i1 but operands are the source type
		cmpTy := "i8" // default for u8 operands
		if inst.SrcTy != nil {
			cmpTy = llvmType(inst.SrcTy)
		}
		g.sb.WriteString(fmt.Sprintf("  %%%d = icmp %s %s %%%d, %%%d\n", dst, pred, cmpTy, src0, src1))

	case mir2.OpLoad:
		elemTy := llvmType(inst.Ty)
		g.sb.WriteString(fmt.Sprintf("  %%%d = load %s, ptr %%%d\n", dst, elemTy, src0))

	case mir2.OpStore:
		valTy := llvmType(inst.Ty)
		g.sb.WriteString(fmt.Sprintf("  store %s %%%d, ptr %%%d\n", valTy, src1, src0))

	case mir2.OpExt:
		srcTy := llvmType(inst.SrcTy)
		g.sb.WriteString(fmt.Sprintf("  %%%d = zext %s %%%d to %s\n", dst, srcTy, src0, ty))

	case mir2.OpSext:
		srcTy := llvmType(inst.SrcTy)
		g.sb.WriteString(fmt.Sprintf("  %%%d = sext %s %%%d to %s\n", dst, srcTy, src0, ty))

	case mir2.OpTrunc:
		srcTy := llvmType(inst.SrcTy)
		g.sb.WriteString(fmt.Sprintf("  %%%d = trunc %s %%%d to %s\n", dst, srcTy, src0, ty))

	case mir2.OpCall:
		callRetTy := llvmType(inst.Ty)
		var args []string
		callee := g.mod.FuncByName(inst.Sym)
		for i, arg := range inst.Args {
			argTy := "i8" // default
			if callee != nil && i < len(callee.Contract.Params) {
				argTy = llvmType(callee.Contract.Params[i].Ty)
			}
			args = append(args, fmt.Sprintf("%s %%%d", argTy, arg))
		}
		callName := llvmSym(inst.Sym)
		if dst != mir2.NoReg {
			g.sb.WriteString(fmt.Sprintf("  %%%d = call %s @%s(%s)\n",
				dst, callRetTy, callName, strings.Join(args, ", ")))
		} else {
			g.sb.WriteString(fmt.Sprintf("  call %s @%s(%s)\n",
				callRetTy, callName, strings.Join(args, ", ")))
		}

	case mir2.OpAddrOf:
		g.sb.WriteString(fmt.Sprintf("  %%%d = ptrtoint ptr @%s to %s\n",
			dst, llvmSym(inst.Sym), ty))

	default:
		g.sb.WriteString(fmt.Sprintf("  ; TODO: op %d\n", inst.Op))
	}
}

func (g *gen) emitTerm(term mir2.Term, f *mir2.Func) {
	switch t := term.(type) {
	case *mir2.TermRet:
		if len(t.Vals) > 0 && t.Vals[0] != mir2.NoReg {
			retTy := llvmType(contractRetTy(f))
			g.sb.WriteString(fmt.Sprintf("  ret %s %%%d\n", retTy, t.Vals[0]))
		} else {
			g.sb.WriteString("  ret void\n")
		}

	case *mir2.TermJmp:
		g.sb.WriteString(fmt.Sprintf("  br label %%%s\n", llvmLabel(t.Target)))

	case *mir2.TermBrIf:
		g.sb.WriteString(fmt.Sprintf("  br i1 %%%d, label %%%s, label %%%s\n",
			t.Cond, llvmLabel(t.Then), llvmLabel(t.Else)))

	case *mir2.TermCondRet:
		// cond_ret %cond, [vals], @then
		// if cond==0 → return vals, else → jump to then
		retTy := llvmType(contractRetTy(f))
		// Need a temp for i1 eqz
		tmpReg := int(t.Cond) + 10000
		g.sb.WriteString(fmt.Sprintf("  %%%d = icmp eq i1 %%%d, 0\n", tmpReg, t.Cond))
		thenLabel := llvmLabel(t.Then)
		retLabel := fmt.Sprintf("condret_%d", t.Cond)
		g.sb.WriteString(fmt.Sprintf("  br i1 %%%d, label %%%s, label %%%s\n",
			tmpReg, retLabel, thenLabel))
		// Return block
		g.sb.WriteString(fmt.Sprintf("%s:\n", retLabel))
		if len(t.Vals) > 0 && t.Vals[0] != mir2.NoReg {
			g.sb.WriteString(fmt.Sprintf("  ret %s %%%d\n", retTy, t.Vals[0]))
		} else {
			g.sb.WriteString("  ret void\n")
		}

	case nil:
		g.sb.WriteString("  unreachable\n")

	default:
		g.sb.WriteString("  ; TODO: terminator\n")
		g.sb.WriteString("  unreachable\n")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────

func buildPredMap(f *mir2.Func) map[string][][]predEdge {
	preds := make(map[string][][]predEdge)
	for _, b := range f.Blocks {
		b.Term.ForEachEdge(func(target string, args []mir2.Reg, idx int) {
			// Ensure target has enough param slots
			for len(preds[target]) < len(args) {
				preds[target] = append(preds[target], nil)
			}
			for i, arg := range args {
				preds[target][i] = append(preds[target][i], predEdge{
					label: b.Label,
					reg:   arg,
				})
			}
		})
	}
	return preds
}

func llvmType(ty mir2.Ty) string {
	if ty == nil {
		return "i8"
	}
	switch ty {
	case mir2.TyVoid:
		return "void"
	case mir2.TyBool:
		return "i1"
	case mir2.TyU8, mir2.TyI8:
		return "i8"
	case mir2.TyU16, mir2.TyI16:
		return "i16"
	}
	if ty == mir2.TyPtr {
		return "ptr"
	}
	w := ty.Width()
	if w <= 8 {
		return "i8"
	}
	if w <= 16 {
		return "i16"
	}
	return "i32"
}

func llvmCmpPred(cond mir2.CmpCond) string {
	switch cond {
	case mir2.CmpEq:
		return "eq"
	case mir2.CmpNe:
		return "ne"
	case mir2.CmpLt:
		return "slt"
	case mir2.CmpLe:
		return "sle"
	case mir2.CmpGt:
		return "sgt"
	case mir2.CmpGe:
		return "sge"
	case mir2.CmpUlt:
		return "ult"
	case mir2.CmpUle:
		return "ule"
	case mir2.CmpUgt:
		return "ugt"
	case mir2.CmpUge:
		return "uge"
	default:
		return "eq"
	}
}

func llvmSym(name string) string {
	s := strings.ReplaceAll(name, "@", "_")
	s = strings.ReplaceAll(s, "$", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func llvmLabel(label string) string {
	return strings.ReplaceAll(strings.ReplaceAll(label, "-", "_"), "@", "")
}

func llvmArrayType(bytes int) string {
	return fmt.Sprintf("[%d x i8]", bytes)
}

func contractRetTy(f *mir2.Func) mir2.Ty {
	if len(f.Contract.Returns) > 0 {
		return f.Contract.Returns[0].Ty
	}
	return mir2.TyVoid
}

func llvmEscapeString(s string) string {
	var buf strings.Builder
	for _, b := range []byte(s) {
		if b >= 32 && b < 127 && b != '"' && b != '\\' {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "\\%02X", b)
		}
	}
	return buf.String()
}
