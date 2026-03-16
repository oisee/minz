package mir2

// SplitJoinRet splits trivial join-blocks that only contain a Ret into
// separate Ret blocks at each predecessor. This enables CondRetSink and
// FuseAbsDiff to fire on patterns like:
//
//	var d = a - b; if (a < b) d = b - a; return d
//
// Before:
//
//	block @entry:
//	  ...
//	  br_if %f, @then(...), @join(%d)
//	block @then:
//	  ...
//	  jmp @join(%d2)
//	block @join(%result):
//	  ret %result
//
// After:
//
//	block @entry:
//	  ...
//	  br_if %f, @then(...), @else()
//	block @then:
//	  ...
//	  ret %d2
//	block @else:
//	  ret %d
//
// Conditions:
//   - Join block has exactly one instruction: none (empty body)
//   - Join block's terminator is TermRet with one value = the block param
//   - Join block has exactly one block param
//   - All predecessors jump to join with exactly one argument
func SplitJoinRet(f *Func) bool {
	changed := false
	for _, joinBlk := range f.Blocks {
		// Must have exactly 1 param and 0 instructions.
		if len(joinBlk.Params) != 1 || len(joinBlk.Insts) != 0 {
			continue
		}
		// Terminator must be Ret(param).
		ret, ok := joinBlk.Term.(*TermRet)
		if !ok || len(ret.Vals) != 1 || ret.Vals[0] != joinBlk.Params[0].Dst {
			continue
		}

		// Find all predecessors that jump to this join block.
		type predInfo struct {
			block *Block
			arg   Reg // the value passed as the join param
		}
		var preds []predInfo
		for _, b := range f.Blocks {
			if b == joinBlk {
				continue
			}
			switch t := b.Term.(type) {
			case *TermJmp:
				if t.Target == joinBlk.Label && len(t.Args) == 1 {
					preds = append(preds, predInfo{b, t.Args[0]})
				}
			case *TermBrIf:
				if t.Then == joinBlk.Label && len(t.ThenArgs) == 1 {
					// Then-branch jumps to join. Replace with a new ret block.
					retLabel := joinBlk.Label + "_ret_then"
					retBlk := &Block{Label: retLabel, Term: &TermRet{Vals: []Reg{t.ThenArgs[0]}}}
					f.Blocks = append(f.Blocks, retBlk)
					t.Then = retLabel
					t.ThenArgs = nil
					changed = true
				}
				if t.Else == joinBlk.Label && len(t.ElseArgs) == 1 {
					retLabel := joinBlk.Label + "_ret_else"
					retBlk := &Block{Label: retLabel, Term: &TermRet{Vals: []Reg{t.ElseArgs[0]}}}
					f.Blocks = append(f.Blocks, retBlk)
					t.Else = retLabel
					t.ElseArgs = nil
					changed = true
				}
			}
		}

		// Replace TermJmp predecessors with TermRet.
		for _, p := range preds {
			p.block.Term = &TermRet{Vals: []Reg{p.arg}}
			changed = true
		}
	}

	return changed
}
