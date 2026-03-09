package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// LoopVarHinter annotates local variables that are heavily used inside loops
// with register hints (attractors), so the register allocator can keep them
// in physical registers instead of spilling to $F0xx memory.
//
// Heuristic assignment (Z80 register roles):
//   - u8 loop counter (INC/DEC, used in comparison against loop bound): RegHintB → DJNZ-compatible
//   - u16 accumulator (16-bit ADD destination, read back immediately):   RegHintHL
//   - u16 second operand in ADD (paired with the accumulator):           RegHintDE
//   - u8 scalar used as arithmetic operand or temporary:                 RegHintA
//
// The hinter only assigns hints; it never overrides existing non-None hints.
type LoopVarHinter struct{}

// NewLoopVarHinter creates a new loop variable hinter pass.
func NewLoopVarHinter() *LoopVarHinter { return &LoopVarHinter{} }

// Run processes all functions in the module.
func (h *LoopVarHinter) Run(module *ir.Module) {
	for _, fn := range module.Functions {
		h.annotateFunction(fn)
	}
}

// loopVarStats tracks usage statistics for one local variable.
type loopVarStats struct {
	addSrc    int // times used as src in 16-bit Add inside a loop
	addDest   int // times used as dest in 16-bit Add inside a loop
	incDec    int // times incremented/decremented
	cmpOps    int // times used in comparison (<=, <, >, >=, ==)
	u8        bool
	u16       bool
}

func (h *LoopVarHinter) annotateFunction(fn *ir.Function) {
	if len(fn.Locals) == 0 {
		return
	}

	// Build local register → Local index map for fast lookup.
	localByReg := make(map[ir.Register]int, len(fn.Locals))
	for i, loc := range fn.Locals {
		localByReg[loc.Reg] = i
	}

	// Build virtual-register → local-register map:
	// OpLoadVar(dest=rN, src1=localReg) → rN tracks localReg.
	// OpStoreVar(dest=localReg, src1=rM)
	tracksLocal := make(map[ir.Register]ir.Register) // virtReg → localReg

	stats := make([]loopVarStats, len(fn.Locals))
	for i, loc := range fn.Locals {
		if bt, ok := loc.Type.(*ir.BasicType); ok {
			switch bt.Kind {
			case ir.TypeU8, ir.TypeI8, ir.TypeBool:
				stats[i].u8 = true
			case ir.TypeU16, ir.TypeI16:
				stats[i].u16 = true
			}
		}
	}

	// Single pass over instructions — detect loops and count usage.
	inLoop := false
	for _, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpLabel:
			// Any label that ends with a back-edge target heuristic: if we see a
			// "loop_N" label we're entering a loop body. Simple heuristic: track
			// whether we've passed any loop label at all.
			inLoop = true // conservative: treat everything after first label as potential loop
		case ir.OpLoadVar:
			// dest = load localReg
			localReg := inst.Src1
			if inst.Symbol != "" {
				// symbol-based load — find by name
				for _, loc := range fn.Locals {
					if loc.Name == inst.Symbol {
						localReg = loc.Reg
						break
					}
				}
			}
			if _, ok := localByReg[localReg]; ok {
				tracksLocal[inst.Dest] = localReg
			}
		case ir.OpStoreVar:
			// store localReg, src1
			localReg := inst.Dest
			if inst.Symbol != "" {
				for _, loc := range fn.Locals {
					if loc.Name == inst.Symbol {
						localReg = loc.Reg
						break
					}
				}
			}
			if idx, ok := localByReg[localReg]; ok && inLoop {
				_ = idx // recorded via src analysis below
			}
		case ir.OpAdd:
			if !inLoop {
				break
			}
			// Check if either src tracks a local variable.
			if srcLocal, ok := tracksLocal[inst.Src1]; ok {
				if idx, ok2 := localByReg[srcLocal]; ok2 {
					stats[idx].addSrc++
				}
			}
			if srcLocal, ok := tracksLocal[inst.Src2]; ok {
				if idx, ok2 := localByReg[srcLocal]; ok2 {
					stats[idx].addSrc++
				}
			}
			// dest of Add: if it will be stored back to same local, it's addDest
			if destLocal, ok := tracksLocal[inst.Dest]; ok {
				if idx, ok2 := localByReg[destLocal]; ok2 {
					stats[idx].addDest++
				}
			}
		case ir.OpInc, ir.OpDec:
			if !inLoop {
				break
			}
			if srcLocal, ok := tracksLocal[inst.Src1]; ok {
				if idx, ok2 := localByReg[srcLocal]; ok2 {
					stats[idx].incDec++
				}
			}
		case ir.OpCmp, ir.OpEq, ir.OpNe, ir.OpLt, ir.OpGt, ir.OpLe, ir.OpGe:
			if !inLoop {
				break
			}
			for _, src := range []ir.Register{inst.Src1, inst.Src2} {
				if srcLocal, ok := tracksLocal[src]; ok {
					if idx, ok2 := localByReg[srcLocal]; ok2 {
						stats[idx].cmpOps++
					}
				}
			}
		}
	}

	// Assign hints based on statistics.
	// Priority: counter (B) > accumulator (HL) > second operand (DE) > temp (A).
	hlTaken := false
	deTaken := false
	bTaken  := false

	for i := range fn.Locals {
		if fn.Locals[i].Hint != ir.RegHintNone {
			// Respect existing hints (e.g., from iterator.go).
			switch fn.Locals[i].Hint {
			case ir.RegHintB, ir.RegHintBC:
				bTaken = true
			case ir.RegHintHL:
				hlTaken = true
			case ir.RegHintDE:
				deTaken = true
			}
		}
	}

	for i := range fn.Locals {
		if fn.Locals[i].Hint != ir.RegHintNone {
			continue // already has a hint
		}
		s := stats[i]
		if s.u8 && s.incDec > 0 && s.cmpOps > 0 && !bTaken {
			// Counter variable: incremented AND compared → B register (DJNZ-compatible)
			fn.Locals[i].Hint = ir.RegHintB
			bTaken = true
		} else if s.u16 && s.addDest > 0 && !hlTaken {
			// 16-bit accumulator (direct ADD destination, e.g. sum += x) → HL
			fn.Locals[i].Hint = ir.RegHintHL
			hlTaken = true
		} else if s.u16 && s.addSrc > 0 && !deTaken {
			// 16-bit ADD operand → DE (second operand of ADD HL, DE)
			fn.Locals[i].Hint = ir.RegHintDE
			deTaken = true
		} else if s.u8 && s.cmpOps > 0 && !bTaken {
			// u8 comparison operand → B (still useful even without inc/dec)
			fn.Locals[i].Hint = ir.RegHintB
			bTaken = true
		}
	}
}
