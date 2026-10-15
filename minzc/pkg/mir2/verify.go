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
//     dominating definition (independent of block layout).
//   - Side-effecting ops (OpStore, OpPatch) have Dst == NoReg.
//   - Branch targets exist within the function.
//   - Block-argument arity matches target block params. Numeric edge conversions
//     retain the VM contract; strict type checking is not performed here.
//   - [host] calls appear only inside [comptime] functions.
func Verify(m *Module) error {
	if m == nil {
		return fmt.Errorf("mir2: nil module")
	}
	// Check containers before any semantic/type scan can dereference them.
	for fi, f := range m.Funcs {
		if f == nil {
			return fmt.Errorf("mir2: nil function at %d", fi)
		}
		for bi, b := range f.Blocks {
			if b == nil {
				return fmt.Errorf("mir2: func @%s: nil block at %d", f.Name, bi)
			}
			for ii, i := range b.Insts {
				if i == nil {
					return fmt.Errorf("mir2: func @%s block @%s: nil instruction at %d", f.Name, b.Label, ii)
				}
			}
		}
	}
	if err := VerifyStructReturnBuffers(m); err != nil {
		return err
	}
	if err := validateNamedSMC(m); err != nil {
		return err
	}
	for _, f := range m.Funcs {
		if f.Attrs.FallibleCY {
			if f.Attrs.IsExtern || len(f.Contract.Returns) > 1 || len(f.Contract.Returns) == 1 && (f.Contract.Returns[0].Ty != TyU8 || f.Contract.Returns[0].Class != ClassAcc) {
				return fmt.Errorf("invalid fallible CY ABI: %s", f.Name)
			}
		}
		info := collectRegInfo(f)
		for _, b := range f.Blocks {
			if term, ok := b.Term.(*TermErrorRet); ok {
				if !f.Attrs.FallibleCY || info[term.Code].Ty != TyU8 {
					return fmt.Errorf("invalid error return: %s", f.Name)
				}
			}
			for _, i := range b.Insts {
				callee := m.FuncByName(i.Sym)
				if i.FallibleCY {
					if i.Op != OpCall || callee == nil || !callee.Attrs.FallibleCY || len(i.ExtraRets) != 2 || len(i.ExtraRetTys) != 2 || i.ExtraRetTys[0] != TyBool || i.ExtraRetTys[1] != TyU8 {
						return fmt.Errorf("invalid fallible call: %s", i.Sym)
					}
				} else if i.Op == OpCall && callee != nil && callee.Attrs.FallibleCY {
					return fmt.Errorf("missing fallible call outputs: %s", i.Sym)
				}
			}
		}
		if err := verifyDirectCallShape(m, f); err != nil {
			return err
		}
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
	for label, origin := range f.LoopOrigins {
		if f.BlockByLabel(label) == nil {
			return fmt.Errorf("loop origin references missing block @%s", label)
		}
		if origin.SiteID == "" || origin.Role == "" {
			return fmt.Errorf("block @%s: loop origin requires site ID and role", label)
		}
	}
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

	// Collect definitions first: block layout is not execution order.
	defs := make(map[Reg]regDefinition)
	define := func(r Reg, block *Block, position int) error {
		if r == NoReg {
			return nil
		}
		if _, exists := defs[r]; exists {
			return fmt.Errorf("duplicate definition of %%r%d", r)
		}
		defs[r] = regDefinition{block: block, position: position}
		return nil
	}
	for _, p := range f.Contract.Params {
		if err := define(p.Reg, nil, -1); err != nil {
			return err
		}
	}
	for _, b := range f.Blocks {
		if b.Term == nil {
			return fmt.Errorf("block @%s has no terminator", b.Label)
		}
		if err := verifyTermArgs(b.Label, b.Term, blockMap); err != nil {
			return err
		}
		for _, p := range b.Params {
			if err := define(p.Dst, b, -1); err != nil {
				return err
			}
		}
		for i, inst := range b.Insts {
			if err := define(inst.Dst, b, i); err != nil {
				return err
			}
			for _, r := range inst.ExtraRets {
				if err := define(r, b, i); err != nil {
					return err
				}
			}
		}
	}
	dominators := reachableDominators(f)
	checkUse := func(r Reg, block *Block, position int) error {
		if r == NoReg {
			return nil
		}
		d, exists := defs[r]
		if !exists {
			return fmt.Errorf("use of undefined %%r%d", r)
		}
		if d.block == nil {
			return nil
		} // function parameter
		if d.block == block {
			if d.position >= position {
				return fmt.Errorf("use of %%r%d before its definition", r)
			}
		} else if dom, reachable := dominators[block.Label]; reachable && !dom[d.block.Label] {
			return fmt.Errorf("definition of %%r%d in @%s does not dominate its use", r, d.block.Label)
		}
		// Cross-block dominance is vacuous in unreachable blocks, but undefined
		// registers and local forward uses are still rejected there.
		return nil
	}

	for _, blk := range f.Blocks {
		for ii, inst := range blk.Insts {
			if !sequentialByteOriginsValid(f, inst) {
				return fmt.Errorf("sequential byte flag requires u16 load/store")
			}
			// Check all uses are defined.
			for _, u := range inst.Uses() {
				if err := checkUse(u, blk, ii); err != nil {
					return fmt.Errorf("block @%s inst[%d] (%s): %w", blk.Label, ii, inst.Op, err)
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
		}

		// Check terminator uses.
		for _, u := range blk.Term.termUses() {
			if err := checkUse(u, blk, len(blk.Insts)); err != nil {
				return fmt.Errorf("block @%s terminator: %w", blk.Label, err)
			}
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

// regDefinition records an SSA definition; nil block denotes a function input.
type regDefinition struct {
	block    *Block
	position int
}

func reachableDominators(f *Func) map[string]map[string]bool {
	blocks := make(map[string]*Block, len(f.Blocks))
	for _, b := range f.Blocks {
		blocks[b.Label] = b
	}
	entry := f.Blocks[0].Label
	reachable := map[string]bool{entry: true}
	queue := []string{entry}
	preds := make(map[string][]string)
	for len(queue) > 0 {
		label := queue[0]
		queue = queue[1:]
		for _, next := range blocks[label].Successors() {
			preds[next] = append(preds[next], label)
			if !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}
	dom := make(map[string]map[string]bool, len(reachable))
	for label := range reachable {
		dom[label] = map[string]bool{label: true}
		if label != entry {
			for other := range reachable {
				dom[label][other] = true
			}
		}
	}
	changed := true
	for changed {
		changed = false
		for label := range reachable {
			if label == entry {
				continue
			}
			next := map[string]bool{label: true}
			for candidate := range dom[label] {
				if candidate == label {
					continue
				}
				all := true
				for _, pred := range preds[label] {
					if !dom[pred][candidate] {
						all = false
						break
					}
				}
				if all {
					next[candidate] = true
				}
			}
			if len(next) != len(dom[label]) {
				dom[label] = next
				changed = true
			}
		}
	}
	return dom
}
