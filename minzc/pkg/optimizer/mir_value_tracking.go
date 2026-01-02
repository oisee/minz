package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// MIRValueTrackingPass tracks constant values through MIR and sets CodegenHints
// to enable the code generator to emit optimal instructions (INC/DEC/XOR/eliminate).
//
// This pass analyzes the flow of constant values and detects patterns like:
// - r1 = 0xFE; r1 = 0xFF  → delta +1, hint CanUseINC
// - r1 = 0xFF; r1 = 0xFE  → delta -1, hint CanUseDEC
// - r1 = 0x42; r1 = 0x42  → delta 0, hint CanEliminate
// - r1 = 0               → hint CanUseXOR (for A register)
//
// ADR: docs/297_MIR_Value_Tracking_ADR.md
type MIRValueTrackingPass struct {
	name              string
	optimizationsCount int
}

// ValueInfo tracks known information about a virtual register's value
type ValueInfo struct {
	IsConstant bool
	Value      int64
	IsZero     bool
}

// NewMIRValueTrackingPass creates a new MIR value tracking pass
func NewMIRValueTrackingPass() *MIRValueTrackingPass {
	return &MIRValueTrackingPass{
		name: "MIR Value Tracking",
	}
}

// Name returns the name of the pass
func (p *MIRValueTrackingPass) Name() string {
	return p.name
}

// OptimizationsCount returns the number of hints set
func (p *MIRValueTrackingPass) OptimizationsCount() int {
	return p.optimizationsCount
}

// Run executes the MIR value tracking pass
func (p *MIRValueTrackingPass) Run(module *ir.Module) (bool, error) {
	changed := false

	for _, fn := range module.Functions {
		if p.trackFunction(fn) {
			changed = true
		}
	}

	return changed, nil
}

// trackFunction analyzes a function and sets CodegenHints
func (p *MIRValueTrackingPass) trackFunction(fn *ir.Function) bool {
	changed := false
	values := make(map[ir.Register]*ValueInfo)

	// Helper to invalidate all tracking (at control flow boundaries)
	invalidateAll := func() {
		values = make(map[ir.Register]*ValueInfo)
	}

	// Helper to check if next instruction uses flags
	usesFlags := func(nextIdx int) bool {
		if nextIdx >= len(fn.Instructions) {
			return false
		}
		next := &fn.Instructions[nextIdx]
		switch next.Op {
		case ir.OpJumpIf, ir.OpJumpIfNot, ir.OpJumpIfZero, ir.OpJumpIfNotZero:
			return true
		}
		return false
	}

	for i := range fn.Instructions {
		inst := &fn.Instructions[i]

		// Control flow invalidates all tracking
		switch inst.Op {
		case ir.OpLabel, ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot,
			ir.OpJumpIfZero, ir.OpJumpIfNotZero,
			ir.OpCall, ir.OpCallIndirect, ir.OpReturn:
			invalidateAll()
			continue
		}

		// Track OpLoadConst - main target for optimization
		if inst.Op == ir.OpLoadConst {
			newValue := inst.Imm
			prev, hasPrev := values[inst.Dest]

			// Initialize hints struct if needed
			if inst.CodegenHint == nil {
				inst.CodegenHint = &ir.CodegenHints{}
			}
			hints := inst.CodegenHint

			// Set constant info
			hints.IsConstant = true
			hints.ConstValue = newValue

			// Check for zero - can use XOR A
			if newValue == 0 {
				hints.CanUseXOR = true
				hints.IsZero = true
			}

			// Check delta from previous value
			if hasPrev && prev.IsConstant {
				hints.PrevValue = prev.Value
				delta := newValue - prev.Value

				// Check if next instruction uses flags (INC/DEC modify flags)
				flagsSafe := !usesFlags(i + 1)

				if delta == 1 || (prev.Value == 0xFF && newValue == 0x00) {
					// Can use INC (handles wrapping)
					if flagsSafe {
						hints.CanUseINC = true
						p.optimizationsCount++
						changed = true
					}
				} else if delta == -1 || (prev.Value == 0x00 && newValue == 0xFF) {
					// Can use DEC (handles wrapping)
					if flagsSafe {
						hints.CanUseDEC = true
						p.optimizationsCount++
						changed = true
					}
				} else if delta == 0 {
					// Same value - can eliminate entirely
					hints.CanEliminate = true
					p.optimizationsCount++
					changed = true
				}
			}

			// Update tracking
			values[inst.Dest] = &ValueInfo{
				IsConstant: true,
				Value:      newValue,
				IsZero:     newValue == 0,
			}
			continue
		}

		// Track OpMove - copy propagation for value tracking
		if inst.Op == ir.OpMove {
			if srcInfo, ok := values[inst.Src1]; ok {
				values[inst.Dest] = &ValueInfo{
					IsConstant: srcInfo.IsConstant,
					Value:      srcInfo.Value,
					IsZero:     srcInfo.IsZero,
				}
			} else {
				delete(values, inst.Dest)
			}
			continue
		}

		// Track arithmetic that produces known results
		switch inst.Op {
		case ir.OpXor:
			// r = r XOR r => 0
			if inst.Src1 == inst.Src2 {
				values[inst.Dest] = &ValueInfo{
					IsConstant: true,
					Value:      0,
					IsZero:     true,
				}
				if inst.CodegenHint == nil {
					inst.CodegenHint = &ir.CodegenHints{}
				}
				inst.CodegenHint.IsZero = true
			} else {
				delete(values, inst.Dest)
			}

		case ir.OpAnd:
			// r = x AND 0 => 0
			src1Info, _ := values[inst.Src1]
			src2Info, _ := values[inst.Src2]
			if (src1Info != nil && src1Info.IsZero) || (src2Info != nil && src2Info.IsZero) {
				values[inst.Dest] = &ValueInfo{
					IsConstant: true,
					Value:      0,
					IsZero:     true,
				}
			} else {
				delete(values, inst.Dest)
			}

		case ir.OpMul:
			// r = x * 0 => 0
			src1Info, _ := values[inst.Src1]
			src2Info, _ := values[inst.Src2]
			if (src1Info != nil && src1Info.IsZero) || (src2Info != nil && src2Info.IsZero) {
				values[inst.Dest] = &ValueInfo{
					IsConstant: true,
					Value:      0,
					IsZero:     true,
				}
			} else {
				delete(values, inst.Dest)
			}

		case ir.OpSub:
			// r = x - x => 0
			if inst.Src1 == inst.Src2 {
				values[inst.Dest] = &ValueInfo{
					IsConstant: true,
					Value:      0,
					IsZero:     true,
				}
			} else {
				delete(values, inst.Dest)
			}

		case ir.OpInc:
			// Increment known value
			if info, ok := values[inst.Src1]; ok && info.IsConstant {
				newVal := (info.Value + 1) & 0xFF // 8-bit wrap
				values[inst.Src1] = &ValueInfo{
					IsConstant: true,
					Value:      newVal,
					IsZero:     newVal == 0,
				}
			} else {
				delete(values, inst.Src1)
			}

		case ir.OpDec:
			// Decrement known value
			if info, ok := values[inst.Src1]; ok && info.IsConstant {
				newVal := (info.Value - 1) & 0xFF // 8-bit wrap
				values[inst.Src1] = &ValueInfo{
					IsConstant: true,
					Value:      newVal,
					IsZero:     newVal == 0,
				}
			} else {
				delete(values, inst.Src1)
			}

		default:
			// Any other instruction writing to a register invalidates tracking
			if inst.Dest != 0 {
				delete(values, inst.Dest)
			}
		}
	}

	return changed
}
