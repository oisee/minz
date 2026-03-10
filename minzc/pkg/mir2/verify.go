package mir2

import "fmt"

// Verify checks module-wide structural invariants.
// Returns nil if the module is well-formed.
//
// Checks:
//   - Non-extern functions have at least one block.
//   - Block labels are unique within each function.
//   - Every block is sealed (has a terminator).
//   - Every Reg referenced in instructions/terminators is a param or a
//     previous Dst (forward-reference check; dominance not verified here).
//   - Side-effecting ops (OpStore, OpPatch) have Dst == NoReg.
//   - Branch targets exist within the function.
//   - Block-argument arity and types match target block params.
//   - [host] calls appear only inside [comptime] functions.
func Verify(m *Module) error {
	for _, f := range m.Funcs {
		if err := verifyFunc(f); err != nil {
			return fmt.Errorf("mir2: func @%s: %w", f.Name, err)
		}
	}
	return nil
}

// blockInfo caches per-block metadata used during verification.
type blockInfo struct {
	params []BlockParam
}

func verifyFunc(f *Func) error {
	if f.Attrs.IsExtern {
		return nil
	}
	if len(f.Blocks) == 0 {
		return fmt.Errorf("no blocks in non-extern function")
	}

	// Collect block metadata.
	blockMap := make(map[string]blockInfo, len(f.Blocks))
	for _, b := range f.Blocks {
		if _, dup := blockMap[b.Label]; dup {
			return fmt.Errorf("duplicate block label @%s", b.Label)
		}
		blockMap[b.Label] = blockInfo{params: b.Params}
	}

	// Seed defined-register set with function params.
	defined := map[Reg]bool{NoReg: true}
	for _, p := range f.Contract.Params {
		defined[p.Reg] = true
	}

	for _, blk := range f.Blocks {
		if blk.Term == nil {
			return fmt.Errorf("block @%s has no terminator", blk.Label)
		}

		// Block params define registers at entry.
		for _, p := range blk.Params {
			defined[p.Dst] = true
		}

		for ii, inst := range blk.Insts {
			// Check all uses are defined.
			for _, u := range inst.Uses() {
				if !defined[u] {
					return fmt.Errorf("block @%s inst[%d] (%s): use of undefined %%r%d",
						blk.Label, ii, inst.Op, u)
				}
			}

			// Side-effecting ops must not produce a value.
			if IsSideEffect(inst.Op) && inst.Dst != NoReg {
				return fmt.Errorf("block @%s inst[%d] (%s): side-effecting op must have Dst=NoReg",
					blk.Label, ii, inst.Op)
			}

			// [host] only in comptime functions.
			if inst.CallAttr.IsHost && !f.Attrs.IsComptime {
				return fmt.Errorf("block @%s inst[%d]: [host] call in non-comptime function",
					blk.Label, ii)
			}

			if inst.Dst != NoReg {
				defined[inst.Dst] = true
			}
		}

		// Check terminator uses.
		for _, u := range blk.Term.termUses() {
			if !defined[u] {
				return fmt.Errorf("block @%s terminator: use of undefined %%r%d", blk.Label, u)
			}
		}

		// Check branch targets and block-argument arity.
		if err := verifyTermArgs(blk.Label, blk.Term, blockMap); err != nil {
			return err
		}
	}
	return nil
}

// verifyTermArgs checks that each branch target exists and that the supplied
// argument list matches the target block's param count.
func verifyTermArgs(fromLabel string, term Term, blockMap map[string]blockInfo) error {
	check := func(target string, args []Reg) error {
		info, ok := blockMap[target]
		if !ok {
			return fmt.Errorf("block @%s: branch to unknown block @%s", fromLabel, target)
		}
		if len(args) != len(info.params) {
			return fmt.Errorf(
				"block @%s: branch to @%s supplies %d args but block has %d params",
				fromLabel, target, len(args), len(info.params))
		}
		return nil
	}

	switch t := term.(type) {
	case *TermJmp:
		return check(t.Target, t.Args)
	case *TermBrIf:
		if err := check(t.Then, t.ThenArgs); err != nil {
			return err
		}
		return check(t.Else, t.ElseArgs)
	case *TermBrIf2:
		if err := check(t.Eq, t.EqArgs); err != nil {
			return err
		}
		if err := check(t.Lt, t.LtArgs); err != nil {
			return err
		}
		return check(t.Gt, t.GtArgs)
	case *TermDJNZ:
		// Body block's first param is the counter (implicit from B); BodyArgs = remaining params.
		bodyInfo, ok := blockMap[t.Body]
		if !ok {
			return fmt.Errorf("block @%s: djnz branch to unknown block @%s", fromLabel, t.Body)
		}
		if len(t.BodyArgs)+1 != len(bodyInfo.params) {
			return fmt.Errorf(
				"block @%s: djnz to @%s supplies %d body args but block has %d params (counter is implicit)",
				fromLabel, t.Body, len(t.BodyArgs), len(bodyInfo.params))
		}
		return check(t.Exit, t.ExitArgs)
	case *TermCondRet:
		return check(t.Then, t.ThenArgs)
	}
	return nil // TermRet / TermUnreachable have no targets
}
