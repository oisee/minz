package mir2

// ConstantCallElim evaluates direct function calls at compile time when all
// arguments are known compile-time constants and the callee is defined in the
// same module.
//
// Algorithm:
//  1. Seed consts map from OpConst instructions (same seed as FoldConstants).
//  2. Walk every instruction in every block (in definition order within block).
//  3. For each OpCall with all-constant args: evaluate via MIR2 VM.
//  4. If evaluation succeeds and returns exactly one value: replace OpCall
//     with OpConst(result) and record the result as a new known constant.
//
// The pass is conservative: if the VM returns any error (side effect, stack
// overflow, step limit exceeded, external call), the call is kept as-is.
//
// Current limitation: void calls and multi-return calls are not folded.
// Both are straightforward to add when needed.
//
// Returns true if any call was replaced.
func ConstantCallElim(m *Module, f *Func) bool {
	vm := NewVM(m)
	vm.MaxSteps = 1_000_000 // safety limit per call evaluation

	consts := collectKnownConsts(f)
	changed := false

	for _, b := range f.Blocks {
		for i, inst := range b.Insts {
			if !isConstEliminableCall(inst, m) {
				continue
			}

			// Gather arguments — all must be known constants.
			callArgs := make([]Value, len(inst.Args))
			allConst := true
			for j, arg := range inst.Args {
				v, ok := consts[arg]
				if !ok {
					allConst = false
					break
				}
				callArgs[j] = Value{I: v}
			}
			if !allConst {
				continue
			}

			// Reset step counter between calls (each evaluation gets the full budget).
			vm.steps = 0

			results, err := vm.Call(inst.Sym, callArgs)
			if err != nil {
				continue // keep original call; VM couldn't evaluate (side effect, etc.)
			}
			if len(results) != 1 {
				continue // void or multi-return — not yet handled
			}

			// Replace the call with a constant.
			b.Insts[i] = &Inst{
				Op:  OpConst,
				Dst: inst.Dst,
				Imm: results[0].I,
				Ty:  inst.Ty,
				Cls: inst.Cls,
			}
			consts[inst.Dst] = results[0].I
			changed = true
		}
	}
	return changed
}

// isConstEliminableCall returns true when inst is a direct call that can
// potentially be folded: it must have a result register, name a function
// defined in the module (not an @-intrinsic or external declaration), and
// that function must be non-recursive (to avoid infinite VM evaluation).
func isConstEliminableCall(inst *Inst, m *Module) bool {
	if inst.Op != OpCall || inst.Dst == NoReg || inst.Sym == "" {
		return false
	}
	// @-prefixed names are compiler intrinsics (I/O, SMC, etc.) — skip.
	if inst.Sym[0] == '@' {
		return false
	}
	callee := m.FuncByName(inst.Sym)
	if callee == nil {
		return false // external / not defined in this module
	}
	if callee.Attrs.IsExtern {
		return false
	}
	// Recursive functions would loop forever in the VM.
	// The VM's MaxSteps handles this, but we can skip them cheaply.
	if callee.Attrs.IsRecursive {
		return false
	}
	return true
}
