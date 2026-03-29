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
// Registers use named format %vN to avoid LLVM's sequential numbering constraint.
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
	preds   map[string][][]predEdge // block → param → [(predLabel, argReg)]
	nextTmp int                     // counter for synthetic temp registers
	regTy   map[mir2.Reg]string     // tracks LLVM type of each register
	phiRegs  map[mir2.Reg]bool           // regs defined by phi nodes (block params)
	instDefs map[string]map[mir2.Reg]bool // block label → set of regs defined by instructions
	phiRemap  map[mir2.Reg]map[string]string // reg → block label → unique LLVM name (for multi-block phis)
	curBlock  string                         // current block label being emitted
}

type predEdge struct {
	label string
	reg   mir2.Reg
}

// reg formats a MIR2 register as LLVM named register %vN
func reg(r mir2.Reg) string {
	return fmt.Sprintf("%%v%d", r)
}

// use returns the correct LLVM name for reading register r in the current block.
func (g *gen) use(r mir2.Reg) string {
	// Check if this reg has a block-specific phi rename in the current block
	if remap, ok := g.phiRemap[r]; ok {
		if name, ok := remap[g.curBlock]; ok {
			return name
		}
	}
	return reg(r)
}

// tmp allocates a fresh temporary register name
func (g *gen) tmp() string {
	g.nextTmp++
	return fmt.Sprintf("%%_t%d", g.nextTmp)
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

	// Collect defined function names
	defined := make(map[string]bool)
	for _, f := range g.mod.Funcs {
		defined[f.Name] = true
	}

	// Collect external function references (called but not defined)
	declared := make(map[string]bool)
	for _, f := range g.mod.Funcs {
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op == mir2.OpCall && !defined[inst.Sym] && !declared[inst.Sym] {
					declared[inst.Sym] = true
					// Emit declaration with generic signature
					callRetTy := llvmType(inst.Ty)
					var argTys []string
					for range inst.Args {
						argTys = append(argTys, "i8")
					}
					g.sb.WriteString(fmt.Sprintf("declare %s @%s(%s)\n",
						callRetTy, llvmSym(inst.Sym), strings.Join(argTys, ", ")))
				}
			}
		}
	}
	if len(declared) > 0 {
		g.sb.WriteString("\n")
	}

	// Functions
	for _, f := range g.mod.Funcs {
		if len(f.Blocks) == 0 {
			// External function — emit as declare
			retTy := llvmType(contractRetTy(f))
			var params []string
			for _, p := range f.Contract.Params {
				params = append(params, llvmType(p.Ty))
			}
			g.sb.WriteString(fmt.Sprintf("declare %s @%s(%s)\n\n",
				retTy, llvmSym(f.Name), strings.Join(params, ", ")))
			continue
		}
		g.emitFunc(f)
	}
}

func (g *gen) emitFunc(f *mir2.Func) {
	// Reset per-function state
	g.preds = buildPredMap(f)
	g.nextTmp = 0
	g.regTy = make(map[mir2.Reg]string)

	// Collect phi-defined regs (block params) — these may conflict with
	// instruction defs in earlier blocks. We rename the instruction def
	// to %_init_vN and update phi predecessors accordingly.
	g.phiRegs = make(map[mir2.Reg]bool)
	g.instDefs = make(map[string]map[mir2.Reg]bool)
	g.phiRemap = make(map[mir2.Reg]map[string]string)

	// Track how many blocks define each reg as a phi
	phiCount := make(map[mir2.Reg]int)
	for _, b := range f.Blocks {
		for _, bp := range b.Params {
			g.phiRegs[bp.Dst] = true
			phiCount[bp.Dst]++
		}
		defs := make(map[mir2.Reg]bool)
		for _, inst := range b.Insts {
			if inst.Dst != mir2.NoReg {
				defs[inst.Dst] = true
			}
		}
		g.instDefs[b.Label] = defs
	}

	// For regs that appear as phi in multiple blocks, assign unique names
	for _, b := range f.Blocks {
		for _, bp := range b.Params {
			if phiCount[bp.Dst] > 1 {
				if g.phiRemap[bp.Dst] == nil {
					g.phiRemap[bp.Dst] = make(map[string]string)
				}
				g.phiRemap[bp.Dst][b.Label] = fmt.Sprintf("%%_phi_%s_v%d", llvmLabel(b.Label), bp.Dst)
			}
		}
	}

	// Build register type map from all definitions
	for _, p := range f.Contract.Params {
		g.regTy[p.Reg] = llvmType(p.Ty)
	}
	for _, b := range f.Blocks {
		for _, bp := range b.Params {
			g.regTy[bp.Dst] = llvmType(bp.Ty)
		}
		for _, inst := range b.Insts {
			if inst.Dst != mir2.NoReg {
				g.regTy[inst.Dst] = llvmType(inst.Ty)
				if inst.Op == mir2.OpCmp {
					g.regTy[inst.Dst] = "i1"
				}
				if inst.Op == mir2.OpAlloca || inst.Op == mir2.OpField || inst.Op == mir2.OpPtrAdd || inst.Op == mir2.OpPtrBump {
					g.regTy[inst.Dst] = "ptr"
				}
				if inst.Op == mir2.OpAddrOf && inst.Ty == mir2.TyPtr {
					g.regTy[inst.Dst] = "ptr"
				}
			}
		}
	}

	retTy := llvmType(contractRetTy(f))

	var params []string
	for _, p := range f.Contract.Params {
		params = append(params, fmt.Sprintf("%s %s", llvmType(p.Ty), reg(p.Reg)))
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
	g.curBlock = b.Label
	label := llvmLabel(b.Label)
	g.sb.WriteString(fmt.Sprintf("%s:\n", label))

	// Phi nodes for block params
	for pi, bp := range b.Params {
		if preds, ok := g.preds[b.Label]; ok && pi < len(preds) {
			ty := llvmType(bp.Ty)
			// Use unique name if this reg has phi in multiple blocks
			phiName := reg(bp.Dst)
			if remap, ok := g.phiRemap[bp.Dst]; ok {
				if name, ok := remap[b.Label]; ok {
					phiName = name
				}
			}
			g.sb.WriteString(fmt.Sprintf("  %s = phi %s", phiName, ty))
			for j, pe := range preds[pi] {
				if j > 0 {
					g.sb.WriteString(",")
				}
				predRef := g.resolveRef(pe.reg, pe.label)
				g.sb.WriteString(fmt.Sprintf(" [ %s, %%%s ]", predRef, llvmLabel(pe.label)))
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

// initReg returns the LLVM name for a register that also has a phi definition.
// Instructions that define a phi-conflicting reg get renamed to %_init_vN.
func initReg(r mir2.Reg) string {
	return fmt.Sprintf("%%_init_v%d", r)
}

func (g *gen) emitInst(inst *mir2.Inst) {
	d := reg(inst.Dst)
	// If this reg is also defined by a phi, rename the instruction def
	if g.phiRegs[inst.Dst] && inst.Dst != mir2.NoReg {
		d = initReg(inst.Dst)
	}
	s0 := g.use(inst.Src[0])
	s1 := g.use(inst.Src[1])
	ty := llvmType(inst.Ty)

	switch inst.Op {
	case mir2.OpConst:
		if inst.Ty == mir2.TyPtr || ty == "ptr" {
			// ptr constant: use inttoptr
			g.sb.WriteString(fmt.Sprintf("  %s = inttoptr i64 %d to ptr\n", d, inst.Imm))
		} else {
			g.sb.WriteString(fmt.Sprintf("  %s = add %s 0, %d\n", d, ty, inst.Imm))
		}

	case mir2.OpMove:
		if ty == "ptr" {
			// ptr move: bitcast (no-op in opaque ptr mode)
			g.sb.WriteString(fmt.Sprintf("  %s = getelementptr i8, ptr %s, i32 0\n", d, s0))
		} else {
			g.sb.WriteString(fmt.Sprintf("  %s = add %s 0, %s\n", d, ty, s0))
		}

	case mir2.OpAdd:
		g.sb.WriteString(fmt.Sprintf("  %s = add %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpSub:
		g.sb.WriteString(fmt.Sprintf("  %s = sub %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpMul:
		g.sb.WriteString(fmt.Sprintf("  %s = mul %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpDiv:
		g.sb.WriteString(fmt.Sprintf("  %s = udiv %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpSDiv:
		g.sb.WriteString(fmt.Sprintf("  %s = sdiv %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpMod:
		g.sb.WriteString(fmt.Sprintf("  %s = urem %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpAnd:
		g.sb.WriteString(fmt.Sprintf("  %s = and %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpOr:
		g.sb.WriteString(fmt.Sprintf("  %s = or %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpXor:
		g.sb.WriteString(fmt.Sprintf("  %s = xor %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpShl:
		g.sb.WriteString(fmt.Sprintf("  %s = shl %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpShr:
		g.sb.WriteString(fmt.Sprintf("  %s = lshr %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpSar:
		g.sb.WriteString(fmt.Sprintf("  %s = ashr %s %s, %s\n", d, ty, s0, s1))

	case mir2.OpNeg:
		g.sb.WriteString(fmt.Sprintf("  %s = sub %s 0, %s\n", d, ty, s0))

	case mir2.OpNot:
		g.sb.WriteString(fmt.Sprintf("  %s = xor %s %s, -1\n", d, ty, s0))

	case mir2.OpCmp:
		pred := llvmCmpPred(inst.Cond)
		cmpTy := "i32"
		if inst.SrcTy != nil {
			cmpTy = llvmType(inst.SrcTy)
		}
		g.sb.WriteString(fmt.Sprintf("  %s = icmp %s %s %s, %s\n", d, pred, cmpTy, s0, s1))

	case mir2.OpLoad:
		elemTy := llvmType(inst.Ty)
		g.sb.WriteString(fmt.Sprintf("  %s = load %s, ptr %s\n", d, elemTy, s0))

	case mir2.OpStore:
		valTy := llvmType(inst.Ty)
		g.sb.WriteString(fmt.Sprintf("  store %s %s, ptr %s\n", valTy, s1, s0))

	case mir2.OpExt:
		srcTy := llvmType(inst.SrcTy)
		if srcTy == ty {
			g.sb.WriteString(fmt.Sprintf("  %s = add %s 0, %s\n", d, ty, s0)) // no-op
		} else {
			g.sb.WriteString(fmt.Sprintf("  %s = zext %s %s to %s\n", d, srcTy, s0, ty))
		}

	case mir2.OpSext:
		srcTy := llvmType(inst.SrcTy)
		if srcTy == ty {
			g.sb.WriteString(fmt.Sprintf("  %s = add %s 0, %s\n", d, ty, s0))
		} else {
			g.sb.WriteString(fmt.Sprintf("  %s = sext %s %s to %s\n", d, srcTy, s0, ty))
		}

	case mir2.OpTrunc:
		srcTy := llvmType(inst.SrcTy)
		if srcTy == ty {
			g.sb.WriteString(fmt.Sprintf("  %s = add %s 0, %s\n", d, ty, s0))
		} else {
			g.sb.WriteString(fmt.Sprintf("  %s = trunc %s %s to %s\n", d, srcTy, s0, ty))
		}

	case mir2.OpCall:
		callRetTy := llvmType(inst.Ty)
		var args []string
		callee := g.mod.FuncByName(inst.Sym)
		for i, arg := range inst.Args {
			// Determine expected param type
			paramTy := "i8"
			if callee != nil && i < len(callee.Contract.Params) {
				paramTy = llvmType(callee.Contract.Params[i].Ty)
			}
			// Get actual register type
			actualTy := g.regTy[arg]
			if actualTy == "" {
				actualTy = "i8"
			}
			// Insert conversion if types mismatch
			argRef := g.use(arg)
			if actualTy != paramTy && actualTy != "" && paramTy != "" {
				tmp := g.tmp()
				g.emitConvert(tmp, g.use(arg), actualTy, paramTy)
				argRef = tmp
			}
			args = append(args, fmt.Sprintf("%s %s", paramTy, argRef))
		}
		callName := llvmSym(inst.Sym)
		if inst.Dst != mir2.NoReg {
			g.sb.WriteString(fmt.Sprintf("  %s = call %s @%s(%s)\n",
				d, callRetTy, callName, strings.Join(args, ", ")))
		} else {
			g.sb.WriteString(fmt.Sprintf("  call %s @%s(%s)\n",
				callRetTy, callName, strings.Join(args, ", ")))
		}

	case mir2.OpAddrOf:
		if ty == "ptr" {
			// Global address → ptr directly
			g.sb.WriteString(fmt.Sprintf("  %s = getelementptr i8, ptr @%s, i32 0\n",
				d, llvmSym(inst.Sym)))
		} else {
			// Address as integer
			g.sb.WriteString(fmt.Sprintf("  %s = ptrtoint ptr @%s to %s\n",
				d, llvmSym(inst.Sym), ty))
		}

	case mir2.OpAlloca:
		bytes := inst.Imm
		if bytes <= 0 {
			bytes = 1
		}
		g.sb.WriteString(fmt.Sprintf("  %s = alloca [%d x i8]\n", d, bytes))

	case mir2.OpField:
		// Struct field access: base ptr + offset
		offset := inst.Imm
		g.sb.WriteString(fmt.Sprintf("  %s = getelementptr i8, ptr %s, i64 %d\n", d, s0, offset))

	case mir2.OpPtrAdd:
		// Pointer arithmetic: ptr + offset reg
		g.sb.WriteString(fmt.Sprintf("  %s = getelementptr i8, ptr %s, i32 %s\n", d, s0, s1))

	case mir2.OpPtrBump:
		// Like PtrAdd but always i8 step
		g.sb.WriteString(fmt.Sprintf("  %s = getelementptr i8, ptr %s, i32 %s\n", d, s0, s1))

	default:
		g.sb.WriteString(fmt.Sprintf("  ; TODO: op %v\n", inst.Op))
	}
}

func (g *gen) emitTerm(term mir2.Term, f *mir2.Func) {
	switch t := term.(type) {
	case *mir2.TermRet:
		if len(t.Vals) > 0 && t.Vals[0] != mir2.NoReg {
			retTy := llvmType(contractRetTy(f))
			valTy := g.regTy[t.Vals[0]]
			if valTy == "" {
				valTy = "i32"
			}
			retRef := g.use(t.Vals[0])
			if valTy != retTy && retTy != "void" {
				tmp := g.tmp()
				g.emitConvert(tmp, retRef, valTy, retTy)
				retRef = tmp
			}
			g.sb.WriteString(fmt.Sprintf("  ret %s %s\n", retTy, retRef))
		} else {
			retTy := llvmType(contractRetTy(f))
			if retTy == "void" {
				g.sb.WriteString("  ret void\n")
			} else if retTy == "ptr" {
				g.sb.WriteString("  ret ptr null\n")
			} else {
				g.sb.WriteString(fmt.Sprintf("  ret %s 0\n", retTy))
			}
		}

	case *mir2.TermJmp:
		g.sb.WriteString(fmt.Sprintf("  br label %%%s\n", llvmLabel(t.Target)))

	case *mir2.TermBrIf:
		g.sb.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n",
			g.use(t.Cond), llvmLabel(t.Then), llvmLabel(t.Else)))

	case *mir2.TermCondRet:
		retTy := llvmType(contractRetTy(f))
		tmpName := g.tmp()
		g.sb.WriteString(fmt.Sprintf("  %s = icmp eq i1 %s, 0\n", tmpName, g.use(t.Cond)))
		thenLabel := llvmLabel(t.Then)
		retLabel := fmt.Sprintf("_condret%d", g.nextTmp)
		g.sb.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n",
			tmpName, retLabel, thenLabel))
		g.sb.WriteString(fmt.Sprintf("%s:\n", retLabel))
		if len(t.Vals) > 0 && t.Vals[0] != mir2.NoReg {
			g.sb.WriteString(fmt.Sprintf("  ret %s %s\n", retTy, g.use(t.Vals[0])))
		} else {
			g.sb.WriteString("  ret void\n")
		}

	case *mir2.TermBrIf2:
		// Three-way branch: eq/lt/gt
		// Emit as two nested comparisons
		tmpEq := g.tmp()
		tmpLt := g.tmp()
		g.sb.WriteString(fmt.Sprintf("  %s = icmp eq i32 %s, %s\n", tmpEq, g.use(t.Lhs), g.use(t.Rhs)))
		g.sb.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%_brif2_%d\n",
			tmpEq, llvmLabel(t.Eq), g.nextTmp))
		g.sb.WriteString(fmt.Sprintf("_brif2_%d:\n", g.nextTmp))
		g.sb.WriteString(fmt.Sprintf("  %s = icmp ult i32 %s, %s\n", tmpLt, g.use(t.Lhs), g.use(t.Rhs)))
		g.sb.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n",
			tmpLt, llvmLabel(t.Lt), llvmLabel(t.Gt)))

	case nil:
		// No terminator — emit default return
		retTy := llvmType(contractRetTy(f))
		if retTy == "void" {
			g.sb.WriteString("  ret void\n")
		} else if retTy == "ptr" {
			g.sb.WriteString("  ret ptr null\n")
		} else {
			g.sb.WriteString(fmt.Sprintf("  ret %s 0\n", retTy))
		}

	default:
		g.sb.WriteString("  ; TODO: terminator\n")
		retTy := llvmType(contractRetTy(f))
		if retTy == "void" {
			g.sb.WriteString("  ret void\n")
		} else if retTy == "ptr" {
			g.sb.WriteString("  ret ptr null\n")
		} else {
			g.sb.WriteString(fmt.Sprintf("  ret %s 0\n", retTy))
		}
	}
}

// emitConvert emits a type conversion instruction.
func (g *gen) emitConvert(dst, src, fromTy, toTy string) {
	if fromTy == toTy {
		g.sb.WriteString(fmt.Sprintf("  %s = add %s 0, %s\n", dst, toTy, src))
	} else if fromTy == "ptr" {
		g.sb.WriteString(fmt.Sprintf("  %s = ptrtoint ptr %s to %s\n", dst, src, toTy))
	} else if toTy == "ptr" {
		g.sb.WriteString(fmt.Sprintf("  %s = inttoptr %s %s to ptr\n", dst, fromTy, src))
	} else if llvmTypeWidth(fromTy) > llvmTypeWidth(toTy) {
		g.sb.WriteString(fmt.Sprintf("  %s = trunc %s %s to %s\n", dst, fromTy, src, toTy))
	} else if llvmTypeWidth(fromTy) < llvmTypeWidth(toTy) {
		g.sb.WriteString(fmt.Sprintf("  %s = zext %s %s to %s\n", dst, fromTy, src, toTy))
	} else {
		// Same width, different type — shouldn't happen with uniform i32
		g.sb.WriteString(fmt.Sprintf("  %s = add %s 0, %s\n", dst, toTy, src))
	}
}

// resolveRef returns the correct LLVM name for a register reference
// from a specific predecessor block. Handles init renames and multi-block phi renames.
func (g *gen) resolveRef(r mir2.Reg, fromLabel string) string {
	// If this reg has multi-block phi remap and the predecessor block is one of them
	if remap, ok := g.phiRemap[r]; ok {
		if name, ok := remap[fromLabel]; ok {
			return name
		}
	}
	// If defined by an instruction in the predecessor (and also a phi elsewhere)
	if g.phiRegs[r] {
		if defs, ok := g.instDefs[fromLabel]; ok && defs[r] {
			return initReg(r)
		}
	}
	return reg(r)
}

// ── helpers ──────────────────────────────────────────────────────────────

func buildPredMap(f *mir2.Func) map[string][][]predEdge {
	preds := make(map[string][][]predEdge)
	for _, b := range f.Blocks {
		b.Term.ForEachEdge(func(target string, args []mir2.Reg, idx int) {
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
		return "i32"
	}
	switch ty {
	case mir2.TyVoid:
		return "void"
	case mir2.TyBool:
		return "i1"
	case mir2.TyPtr:
		return "ptr"
	}
	// Use i32 for all integer types to avoid type mismatches across
	// phi nodes and call boundaries. MIR2 SSA doesn't always propagate
	// types consistently (e.g. u8 block param fed by u16 producer).
	// Masking to u8/u16 range happens at the Z80 level, not here.
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

func llvmTypeWidth(ty string) int {
	switch ty {
	case "i1":
		return 1
	case "i8":
		return 8
	case "i16":
		return 16
	case "i32":
		return 32
	case "i64":
		return 64
	case "ptr":
		return 64
	}
	return 8
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
