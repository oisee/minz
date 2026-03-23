// bridge.go — MIR2 → VIR conversion.
//
// Straightforward 1:1 translation of MIR2 instructions to VIROps.
// No pattern selection, no register assignment — just opcode mapping.
// The unified solver handles everything else.
package vir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// LowerBlock converts one MIR2 basic block into VIROps.
// The optional func parameter enables return-value ABI enforcement.
func LowerBlock(b *mir2.Block, desc *MachineDesc, mod *mir2.Module, fn ...*mir2.Func) ([]VIROp, error) {
	var ops []VIROp

	for _, inst := range b.Insts {
		translated, err := translateInst(inst, desc, mod)
		if err != nil {
			return nil, fmt.Errorf("block %s: %w", b.Label, err)
		}
		ops = append(ops, translated...)
	}

	// Fuse addr_of + store/load → direct global access
	ops = fuseGlobalAccess(ops)

	// Fold const + ALU → immediate ALU (enables INC/DEC patterns)
	ops = foldConstIntoALU(ops)

	// ISLE combining: identity elimination, dead const removal
	ops = ISLECombine(ops)

	// NOTE: Parameter register pinning (injectParamPins) is available but disabled.
	// It breaks more cases than it fixes because DstHint creates hard Z3 constraints
	// that conflict with tied ALU patterns. Needs softer approach (cost penalty, not hard).
	// See: abs_diff(b-a) case in verify_test.go.

	// Return-value ABI: if block ends with TermRet or TermCondRet,
	// move result to A (u8) or HL (u16) before the return.
	// Only if the result vreg isn't already guaranteed to be in A/HL
	// (e.g., from a tied ALU pattern like ADD A, r).
	if len(fn) > 0 {
		ops = appendReturnMove(ops, b, desc, fn[0])
	} else {
		ops = appendReturnMove(ops, b, desc)
	}

	// Dead const elimination — safe here because return move has been appended
	ops = EliminateDeadConsts(ops)

	return ops, nil
}

// EliminateDeadConsts removes OpConst ops whose dst vreg is never used.
func EliminateDeadConsts(ops []VIROp) []VIROp {
	used := make(map[int]bool)
	for _, op := range ops {
		for _, s := range op.Src {
			if s > 0 { used[s] = true }
		}
	}
	var result []VIROp
	for _, op := range ops {
		if op.Op == OpConst && op.Dst > 0 && !used[op.Dst] && op.Sym == "" {
			continue
		}
		result = append(result, op)
	}
	return result
}

// LowerFunc converts an entire MIR2 function into VIR blocks.
func LowerFunc(f *mir2.Func, desc *MachineDesc, mod *mir2.Module) (*Func, error) {
	vf := &Func{Name: f.Name}

	for _, b := range f.Blocks {
		ops, err := LowerBlock(b, desc, mod, f)
		if err != nil {
			return nil, fmt.Errorf("func %s: %w", f.Name, err)
		}
		vf.Blocks = append(vf.Blocks, Block{
			Label: b.Label,
			Ops:   ops,
		})
	}

	return vf, nil
}

// mirWidth extracts bit width from a MIR2 instruction's Ty field.
func mirWidth(inst *mir2.Inst, desc *MachineDesc) int {
	w := 8
	if inst.Ty != nil {
		if tw := inst.Ty.Width(); tw > 0 {
			w = tw
		}
	}
	if w < 8 {
		w = 8
	}
	maxW := 16
	if desc.WordSize > maxW {
		maxW = desc.WordSize
	}
	if w > maxW {
		w = maxW
	}
	return w
}

// translateInst maps a single MIR2 instruction to one or more VIROps.
func translateInst(inst *mir2.Inst, desc *MachineDesc, mod *mir2.Module) ([]VIROp, error) {
	w := mirWidth(inst, desc)

	switch inst.Op {
	case mir2.OpConst:
		return []VIROp{{
			Op: OpConst, Dst: int(inst.Dst), Imm: inst.Imm,
			Width: w, Sym: inst.Sym,
		}}, nil

	case mir2.OpMove:
		return []VIROp{{
			Op: OpMove, Dst: int(inst.Dst), Src: [2]int{int(inst.Src[0]), -1},
			Width: w,
		}}, nil

	case mir2.OpAdd:
		return []VIROp{{
			Op: OpAdd, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpSub:
		return []VIROp{{
			Op: OpSub, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpMul:
		return translateMul(inst, desc)

	case mir2.OpAnd:
		return []VIROp{{
			Op: OpAnd, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpOr:
		return []VIROp{{
			Op: OpOr, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpXor:
		return []VIROp{{
			Op: OpXor, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpCmp:
		return []VIROp{{
			Op: OpCmp, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpLoad:
		return []VIROp{{
			Op: OpLoad, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), -1}, Width: w,
		}}, nil

	case mir2.OpStore:
		return []VIROp{{
			Op: OpStore, Dst: -1,
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpCall, mir2.OpCallIndirect:
		return translateCall(inst, desc, mod)

	case mir2.OpShl:
		return []VIROp{{
			Op: OpShl, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpShr:
		return []VIROp{{
			Op: OpShr, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: w,
		}}, nil

	case mir2.OpNeg:
		return []VIROp{{
			Op: OpNeg, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), -1}, Width: w,
		}}, nil

	case mir2.OpNot:
		// Bitwise NOT: XOR A, 0xFF (complement)
		// Implemented as XOR with immediate 0xFF
		return []VIROp{{
			Op: OpXorImm, Dst: int(inst.Dst), Src: [2]int{int(inst.Src[0]), -1},
			Imm: 0xFF, Width: w,
		}}, nil

	case mir2.OpDiv, mir2.OpSDiv:
		return translateDivMod(inst, desc, w, false)

	case mir2.OpMod:
		return translateDivMod(inst, desc, w, true)

	case mir2.OpAddrOf:
		return []VIROp{{
			Op: OpConst, Dst: int(inst.Dst), Sym: inst.Sym, Width: 16,
		}}, nil

	case mir2.OpExt, mir2.OpTrunc:
		return []VIROp{{
			Op: OpMove, Dst: int(inst.Dst), Src: [2]int{int(inst.Src[0]), -1},
			Width: w,
		}}, nil

	case mir2.OpPtrAdd:
		// ptr_add base, offset → ADD HL, DE (16-bit pointer arithmetic)
		return []VIROp{{
			Op: OpAdd, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])}, Width: 16,
		}}, nil

	case mir2.OpField:
		// field base, imm=byte_offset → base + offset
		if inst.Imm == 0 {
			// Zero offset: just a copy
			return []VIROp{{
				Op: OpMove, Dst: int(inst.Dst), Src: [2]int{int(inst.Src[0]), -1},
				Width: 16,
			}}, nil
		}
		// Non-zero offset: add immediate
		return []VIROp{{
			Op: OpAddImm, Dst: int(inst.Dst), Src: [2]int{int(inst.Src[0]), -1},
			Imm: inst.Imm, Width: 16,
		}}, nil

	case mir2.OpPtrBump:
		// Same as OpField semantically
		if inst.Imm == 0 {
			return []VIROp{{
				Op: OpMove, Dst: int(inst.Dst), Src: [2]int{int(inst.Src[0]), -1},
				Width: 16,
			}}, nil
		}
		return []VIROp{{
			Op: OpAddImm, Dst: int(inst.Dst), Src: [2]int{int(inst.Src[0]), -1},
			Imm: inst.Imm, Width: 16,
		}}, nil

	case mir2.OpAlloca:
		// alloca N → SP adjustment, return pointer
		// For now, treat as const (frame address will be resolved later)
		return []VIROp{{
			Op: OpConst, Dst: int(inst.Dst), Imm: inst.Imm, Width: 16,
		}}, nil

	case mir2.OpAsm:
		// Inline assembly — skip (not expressible in VIR)
		return nil, nil

	default:
		// Skip side-effect-free ops with no result
		if inst.Dst == mir2.NoReg {
			return nil, nil
		}
		return nil, fmt.Errorf("unsupported MIR2 op: %v", inst.Op)
	}
}

// translateMul converts OpMul to runtime call with proper arg setup.
// __mul8: A × B → A.  __mul16: HL × DE → HL.
func translateMul(inst *mir2.Inst, desc *MachineDesc) ([]VIROp, error) {
	w := mirWidth(inst, desc)
	return translateRuntimeCall(inst, desc, w, true)
}

// translateRuntimeCall emits arg setup moves + CALL for runtime routines.
// Runtime ABI:  8-bit: arg0→A, arg1→B, result→A
//              16-bit: arg0→HL, arg1→DE, result→HL
func translateRuntimeCall(inst *mir2.Inst, desc *MachineDesc, w int, isMul bool) ([]VIROp, error) {
	var ops []VIROp
	var sym string

	if isMul {
		if w <= 8 { sym = "__mul8" } else { sym = "__mul16" }
	} else {
		// div/mod determined by caller
		return nil, fmt.Errorf("use translateDiv/translateMod")
	}

	if w <= 8 {
		// 8-bit: arg0 → A, arg1 → B
		ops = append(ops, VIROp{
			Op: OpMove, Dst: int(inst.Src[0]) + 7000,
			Src: [2]int{int(inst.Src[0]), -1}, Width: 8,
			DstHint: desc.LocSetByNames("A"),
		})
		ops = append(ops, VIROp{
			Op: OpMove, Dst: int(inst.Src[1]) + 7100,
			Src: [2]int{int(inst.Src[1]), -1}, Width: 8,
			DstHint: desc.LocSetByNames("B"),
		})
	} else {
		// 16-bit: arg0 → HL, arg1 → DE
		ops = append(ops, VIROp{
			Op: OpMove, Dst: int(inst.Src[0]) + 7000,
			Src: [2]int{int(inst.Src[0]), -1}, Width: 16,
			DstHint: desc.LocSetByNames("HL"),
		})
		ops = append(ops, VIROp{
			Op: OpMove, Dst: int(inst.Src[1]) + 7100,
			Src: [2]int{int(inst.Src[1]), -1}, Width: 16,
			DstHint: desc.LocSetByNames("DE"),
		})
	}

	// Clobbers: all GPR + flags
	clobbers := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L", "F")
	if desc.Name == "z80" {
		clobbers = clobbers.Or(desc.LocSetByNames("BC", "DE", "HL"))
	}

	// Return value hint
	var retHint LocSet
	if w <= 8 { retHint = desc.LocSetByNames("A") } else { retHint = desc.LocSetByNames("HL") }

	ops = append(ops, VIROp{
		Op: OpCall, Dst: int(inst.Dst),
		Src: [2]int{-1, -1}, Sym: sym, Width: w,
		Clobbers: clobbers, DstHint: retHint,
	})

	return ops, nil
}

// translateDivMod emits arg setup + CALL for __div8/__mod8 runtime.
// ABI: A=dividend, B=divisor → A=quotient (div) or A=remainder (mod).
func translateDivMod(inst *mir2.Inst, desc *MachineDesc, w int, isMod bool) ([]VIROp, error) {
	if w > 8 {
		return nil, fmt.Errorf("16-bit div/mod: use PBQP fallback")
	}
	sym := "__div8"
	if isMod { sym = "__mod8" }
	if w == 16 {
		sym = "__div16"
		if isMod { sym = "__mod16" }
	}

	var ops []VIROp

	if w <= 8 {
		ops = append(ops, VIROp{
			Op: OpMove, Dst: int(inst.Src[0]) + 7000,
			Src: [2]int{int(inst.Src[0]), -1}, Width: 8,
			DstHint: desc.LocSetByNames("A"),
		})
		ops = append(ops, VIROp{
			Op: OpMove, Dst: int(inst.Src[1]) + 7100,
			Src: [2]int{int(inst.Src[1]), -1}, Width: 8,
			DstHint: desc.LocSetByNames("B"),
		})
	}

	clobbers := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L", "F")
	if desc.Name == "z80" {
		clobbers = clobbers.Or(desc.LocSetByNames("BC", "DE", "HL"))
	}

	ops = append(ops, VIROp{
		Op: OpCall, Dst: int(inst.Dst),
		Src: [2]int{-1, -1}, Sym: sym, Width: w,
		Clobbers: clobbers, DstHint: desc.LocSetByNames("A"),
	})

	return ops, nil
}

// translateCall converts OpCall to VIROps: arg setup moves + CALL + result move.
func translateCall(inst *mir2.Inst, desc *MachineDesc, mod *mir2.Module) ([]VIROp, error) {
	clobbers := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L", "F")
	if desc.Name == "z80" {
		clobbers = clobbers.Or(desc.LocSetByNames("BC", "DE", "HL"))
	}

	var ops []VIROp

	// Emit argument setup moves: each arg vreg → register matching callee convention
	if mod != nil && inst.Sym != "" {
		callee := mod.FuncByName(inst.Sym)
		if callee != nil && len(callee.Contract.Params) > 0 {
			for i, argReg := range inst.Args {
				if i >= len(callee.Contract.Params) {
					break
				}
				cp := callee.Contract.Params[i]
				w := 8
				if cp.Ty != nil {
					if tw := cp.Ty.Width(); tw > 0 {
						w = tw
					}
				}
				if w < 8 {
					w = 8
				}
				// Move arg vreg to the callee's expected register.
				// DstHint from callee's contract class — ensures @z80_ pins
				// are respected (e.g., @z80_b → B, @z80_c → C).
				hint := regClassToLocSet(desc, cp.Class, w)
				ops = append(ops, VIROp{
					Op: OpMove, Dst: int(cp.Reg),
					Src: [2]int{int(argReg), -1}, Width: w,
					DstHint: hint,
				})
			}
		}
	}

	// The CALL itself
	callOp := VIROp{
		Op:       OpCall,
		Dst:      int(inst.Dst),
		Src:      [2]int{-1, -1},
		Sym:      inst.Sym,
		Width:    mirWidth(inst, desc),
		Clobbers: clobbers,
	}
	// Return value hint: u8→A, u16→HL
	w := mirWidth(inst, desc)
	if inst.Dst != mir2.NoReg {
		if w <= 8 {
			callOp.DstHint = desc.LocSetByNames("A")
		} else {
			callOp.DstHint = desc.LocSetByNames("HL")
		}
	}
	ops = append(ops, callOp)

	return ops, nil
}

// regClassToLocSet maps a MIR2 RegClass to a VIR LocSet.
func regClassToLocSet(desc *MachineDesc, cls mir2.RegClass, width int) LocSet {
	switch cls {
	case mir2.ClassAcc:
		if width <= 8 {
			return desc.LocSetByNames("A")
		}
		return desc.LocSetByNames("HL")
	case mir2.ClassCounter:
		return desc.LocSetByNames("B")
	case mir2.ClassRegC:
		return desc.LocSetByNames("C")
	case mir2.ClassRegD:
		return desc.LocSetByNames("D")
	case mir2.ClassRegE:
		return desc.LocSetByNames("E")
	case mir2.ClassRegH:
		return desc.LocSetByNames("H")
	case mir2.ClassRegL:
		return desc.LocSetByNames("L")
	case mir2.ClassPointer:
		return desc.LocSetByNames("HL")
	case mir2.ClassIndex:
		return desc.LocSetByNames("DE")
	case mir2.ClassGeneral:
		if width <= 8 {
			return desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
		}
		return desc.LocSetByNames("BC", "DE", "HL")
	case mir2.ClassPair:
		return desc.LocSetByNames("BC", "DE", "HL")
	case mir2.ClassFlag:
		return desc.LocSetByNames("F")
	default:
		if width <= 8 {
			return desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
		}
		return desc.LocSetByNames("BC", "DE", "HL")
	}
}

// fuseGlobalAccess fuses addr_of + store/load into direct global ops.
func fuseGlobalAccess(ops []VIROp) []VIROp {
	if len(ops) < 2 {
		return ops
	}

	defMap := make(map[int]int)
	for i, op := range ops {
		if op.Dst >= 0 {
			defMap[op.Dst] = i
		}
	}

	var result []VIROp
	skip := make(map[int]bool)

	for i, op := range ops {
		if skip[i] {
			continue
		}

		if op.Op == OpStore && op.Src[0] >= 0 {
			if defIdx, ok := defMap[op.Src[0]]; ok {
				def := ops[defIdx]
				if def.Op == OpConst && def.Sym != "" {
					skip[defIdx] = true
					result = append(result, VIROp{
						Op: OpStoreGlobal, Dst: -1,
						Src: [2]int{op.Src[1], op.Src[1]},
						Imm: def.Imm, Sym: def.Sym, Width: op.Width,
					})
					continue
				}
			}
		}

		if op.Op == OpLoad && op.Src[0] >= 0 {
			if defIdx, ok := defMap[op.Src[0]]; ok {
				def := ops[defIdx]
				if def.Op == OpConst && def.Sym != "" {
					skip[defIdx] = true
					result = append(result, VIROp{
						Op: OpLoadGlobal, Dst: op.Dst,
						Src: [2]int{-1, -1},
						Imm: def.Imm, Sym: def.Sym, Width: op.Width,
					})
					continue
				}
			}
		}

		result = append(result, op)
	}

	return result
}

// injectParamPins emits identity moves at block entry for function parameter
// vregs that are used in this block but defined in the entry block (or as
// function params). The DstHint constrains each param to its ABI register.
//
// This tells the Z3 solver: "vreg v1 is in A, v2 is in B" at block entry.
// Without this, the solver treats cross-block vregs as unconstrained and
// may assign them to wrong registers (e.g., both to A via tied patterns).
func injectParamPins(ops []VIROp, b *mir2.Block, f *mir2.Func, desc *MachineDesc) []VIROp {
	if len(f.Contract.Params) == 0 {
		return ops
	}

	// Collect vregs used in this block's VIROps (as sources)
	usedInBlock := make(map[int]bool)
	for _, op := range ops {
		for _, s := range op.Src {
			if s > 0 { usedInBlock[s] = true }
		}
	}

	// For each function param used in this block but not defined here,
	// prepend a pin move: pinReg = move(paramReg) with DstHint = ABI register.
	// Then rewrite all VIROp uses of paramReg → pinReg.
	var pins []VIROp

	for _, cp := range f.Contract.Params {
		paramReg := int(cp.Reg)
		if !usedInBlock[paramReg] {
			continue
		}
		// Skip if vreg is defined in this block
		definedHere := false
		for _, op := range ops {
			if op.Dst == paramReg {
				definedHere = true
				break
			}
		}
		if definedHere {
			continue
		}

		w := 8
		if cp.Ty != nil {
			if tw := cp.Ty.Width(); tw > 0 { w = tw }
		}
		if w < 8 { w = 8 }

		hint := regClassToLocSet(desc, cp.Class, w)
		if hint.IsEmpty() {
			continue
		}

		pinReg := 8800 + paramReg
		pins = append(pins, VIROp{
			Op: OpMove, Dst: pinReg, Src: [2]int{paramReg, -1},
			Width: w, DstHint: hint,
		})

		// Rewrite VIROp uses: paramReg → pinReg
		for i := range ops {
			for j, s := range ops[i].Src {
				if s == paramReg {
					ops[i].Src[j] = pinReg
				}
			}
		}
	}

	if len(pins) > 0 {
		return append(pins, ops...)
	}
	return ops
}

// foldConstIntoALU converts `const N; add(x, const_vreg)` → `addImm(x, N)`.
// This enables INC/DEC patterns (OpAddImm with imm=1, OpSubImm with imm=1).
func foldConstIntoALU(ops []VIROp) []VIROp {
	// Build const map: vreg → immediate value
	consts := make(map[int]int64)
	for _, op := range ops {
		if op.Op == OpConst && op.Dst > 0 && op.Sym == "" {
			consts[op.Dst] = op.Imm
		}
	}
	if len(consts) == 0 {
		return ops
	}

	// Map ALU op → immediate ALU op
	immOp := map[Op]Op{
		OpAdd: OpAddImm,
		OpSub: OpSubImm,
		OpAnd: OpAndImm,
		OpOr:  OpOrImm,
		OpXor: OpXorImm,
		OpCmp: OpCmpImm,
	}

	skip := make(map[int]bool) // const ops to remove
	var result []VIROp

	for i, op := range ops {
		if skip[i] {
			continue
		}

		newOp, ok := immOp[op.Op]
		if !ok {
			result = append(result, op)
			continue
		}

		// Check if src1 is a constant
		if op.Src[1] > 0 {
			if imm, isConst := consts[op.Src[1]]; isConst {
				// Fold: add(x, const_N) → addImm(x, N)
				result = append(result, VIROp{
					Op: newOp, Dst: op.Dst, Src: [2]int{op.Src[0], -1},
					Imm: imm, Width: op.Width,
					DstHint: op.DstHint, SrcHint: op.SrcHint,
				})
				// Mark the const op for removal (if single-use)
				for j, cop := range ops {
					if cop.Op == OpConst && cop.Dst == op.Src[1] {
						// Check if const is used elsewhere
						usedElsewhere := false
						for k, other := range ops {
							if k == i { continue }
							for _, s := range other.Src {
								if s == cop.Dst { usedElsewhere = true }
							}
						}
						if !usedElsewhere {
							skip[j] = true
						}
					}
				}
				continue
			}
		}

		// Check if src0 is a constant (for commutative ops)
		if op.Src[0] > 0 && (op.Op == OpAdd || op.Op == OpAnd || op.Op == OpOr || op.Op == OpXor) {
			if imm, isConst := consts[op.Src[0]]; isConst {
				result = append(result, VIROp{
					Op: newOp, Dst: op.Dst, Src: [2]int{op.Src[1], -1},
					Imm: imm, Width: op.Width,
					DstHint: op.DstHint, SrcHint: op.SrcHint,
				})
				for j, cop := range ops {
					if cop.Op == OpConst && cop.Dst == op.Src[0] {
						usedElsewhere := false
						for k, other := range ops {
							if k == i { continue }
							for _, s := range other.Src {
								if s == cop.Dst { usedElsewhere = true }
							}
						}
						if !usedElsewhere { skip[j] = true }
					}
				}
				continue
			}
		}

		result = append(result, op)
	}

	return result
}

// appendReturnMove adds an OpMove to place the return value in A (u8) or HL (u16)
// before a TermRet/TermCondRet. Uses a synthetic vreg (9900+) as the
// "return register" with DstHint constraining it to A or HL.
func appendReturnMove(ops []VIROp, b *mir2.Block, desc *MachineDesc, f ...*mir2.Func) []VIROp {
	term := b.Term
	var retVals []mir2.Reg

	switch t := term.(type) {
	case *mir2.TermRet:
		retVals = t.Vals
	case *mir2.TermCondRet:
		retVals = t.Vals
	default:
		return ops
	}

	if len(retVals) == 0 {
		return ops
	}

	retReg := int(retVals[0])
	if retReg <= 0 {
		return ops
	}

	// Determine width from the vreg's definition
	w := 8
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i].Dst == retReg && ops[i].Width > 0 {
			w = ops[i].Width
			break
		}
		// Also check if it's used as src (cross-block param)
		for _, s := range ops[i].Src {
			if s == retReg && ops[i].Width > 0 {
				w = ops[i].Width
				break
			}
		}
	}

	// Check if the result vreg is already defined by an op that constrains
	// it to A (8-bit ALU) or HL (16-bit ALU). If so, no move needed.
	alreadyConstrained := false

	// Check if result is a block param (function parameter already in correct register)
	for _, bp := range b.Params {
		if int(bp.Dst) == retReg {
			alreadyConstrained = true
			break
		}
	}
	// Check function-level contract params: only skip move if the param's
	// ABI register matches the return ABI register (A for u8, HL for u16).
	if len(f) > 0 && f[0] != nil {
		for _, cp := range f[0].Contract.Params {
			if int(cp.Reg) == retReg {
				if w <= 8 && cp.Class == mir2.ClassAcc {
					alreadyConstrained = true // param already in A = return register
				} else if w > 8 && cp.Class == mir2.ClassPointer {
					alreadyConstrained = true // param already in HL = return register
				}
				break
			}
		}
	}

	for _, op := range ops {
		if op.Dst == retReg {
			switch op.Op {
			case OpAdd, OpSub, OpAnd, OpOr, OpXor, OpNeg,
				OpAddImm, OpSubImm, OpAndImm, OpOrImm, OpXorImm:
				alreadyConstrained = true
			case OpMove:
				if !op.DstHint.IsEmpty() {
					alreadyConstrained = true
				}
			}
		}
	}

	if alreadyConstrained {
		return ops
	}

	// Synthetic vreg for the return-ABI move
	retABIReg := 9900 + retReg

	var retHint LocSet
	if w <= 8 {
		retHint = desc.LocSetByNames("A")
	} else {
		retHint = desc.LocSetByNames("HL")
	}

	// If the return vreg is a function param, set SrcHint to its PBQP-allocated
	// register (not just the class). The assert bootstrap loads args based on
	// PBQP allocation, so VIR must match.
	var srcHint LocSet
	if len(f) > 0 && f[0] != nil {
		for _, cp := range f[0].Contract.Params {
			if int(cp.Reg) == retReg {
				// Use PBQP allocation if available (exact register).
			// This is passed via a module-level allocResults map.
			// For now, fall through to class-based hint.
				// Fallback to class-based hint
				srcHint = regClassToLocSet(desc, cp.Class, w)
				break
			}
		}
	}

	ops = append(ops, VIROp{
		Op: OpMove, Dst: retABIReg, Src: [2]int{retReg, -1},
		Width: w, DstHint: retHint, SrcHint: [2]LocSet{srcHint},
	})

	return ops
}
