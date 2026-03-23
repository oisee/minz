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
func LowerBlock(b *mir2.Block, desc *MachineDesc, mod *mir2.Module) ([]VIROp, error) {
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

	return ops, nil
}

// LowerFunc converts an entire MIR2 function into VIR blocks.
func LowerFunc(f *mir2.Func, desc *MachineDesc, mod *mir2.Module) (*Func, error) {
	vf := &Func{Name: f.Name}

	for _, b := range f.Blocks {
		ops, err := LowerBlock(b, desc, mod)
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
		// Division → runtime call __div8 or __div16
		sym := "__div8"
		if w == 16 {
			sym = "__div16"
		}
		return []VIROp{{
			Op: OpCall, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])},
			Sym: sym, Width: w,
		}}, nil

	case mir2.OpMod:
		// Modulo → runtime call __mod8 or __mod16
		sym := "__mod8"
		if w == 16 {
			sym = "__mod16"
		}
		return []VIROp{{
			Op: OpCall, Dst: int(inst.Dst),
			Src: [2]int{int(inst.Src[0]), int(inst.Src[1])},
			Sym: sym, Width: w,
		}}, nil

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

// translateMul converts OpMul to runtime call (__mul8 or __mul16).
func translateMul(inst *mir2.Inst, desc *MachineDesc) ([]VIROp, error) {
	w := mirWidth(inst, desc)
	sym := "__mul8"
	if w == 16 {
		sym = "__mul16"
	}
	return []VIROp{{
		Op:  OpCall,
		Dst: int(inst.Dst),
		Src: [2]int{int(inst.Src[0]), int(inst.Src[1])},
		Sym: sym, Width: w,
	}}, nil
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
				// Move arg vreg to the callee's param vreg.
				// No DstHint — let Z3 pick the register.
				// The callee expects args in specific regs but the
				// solver handles this via pattern constraints.
				ops = append(ops, VIROp{
					Op: OpMove, Dst: int(cp.Reg),
					Src: [2]int{int(argReg), -1}, Width: w,
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
	case mir2.ClassPointer:
		return desc.LocSetByNames("HL")
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
