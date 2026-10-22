package mir2

// NaturalLoop contains proven CFG facts, independent of block layout. Only
// reachable, single-latch loops with a unique unconditional preheader are
// returned. Affine facts describe modular arithmetic, not a no-wrap proof.
type NaturalLoop struct {
	Header, Preheader, Latch *Block
	Entry, Back              *TermJmp
	Blocks                   map[string]bool
	Exits                    []*Block
	Invariant                map[Reg]Reg
	Inductions               []LoopInduction
	Loads, Stores            []*Inst
	Opaque                   bool
	LiveOut                  map[Reg]bool
}

type LoopInduction struct {
	Param         BlockParam
	Initial, Next Reg
	Step          int64
}

// AnalyzeNaturalLoops extracts the dominance/backedge analysis formerly local
// to HoistInvariantAddresses. Invariance means SSA invariance; loads require a
// separate alias proof before they can be evaluated outside the loop.
func AnalyzeNaturalLoops(f *Func) []*NaturalLoop {
	if f == nil || len(f.Blocks) == 0 {
		return nil
	}
	dom := reachableDominators(f)
	pred := map[string][]*Block{}
	defs := map[Reg]*Block{}
	insts := map[Reg]*Inst{}
	for _, b := range f.Blocks {
		if b.Term != nil {
			for _, s := range b.Term.Successors() {
				pred[s] = append(pred[s], b)
			}
		}
		for _, p := range b.Params {
			defs[p.Dst] = b
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				defs[x.Dst] = b
				insts[x.Dst] = x
			}
		}
	}
	var out []*NaturalLoop
	for _, head := range f.Blocks {
		var latch, pre *Block
		bad := false
		for _, p := range pred[head.Label] {
			if !dom[p.Label][p.Label] {
				continue
			}
			if dom[p.Label][head.Label] {
				if latch != nil && latch != p {
					bad = true
				}
				latch = p
			} else {
				if pre != nil && pre != p {
					bad = true
				}
				pre = p
			}
		}
		if bad || pre == nil || latch == nil {
			continue
		}
		entry, ok := pre.Term.(*TermJmp)
		if !ok || entry.Target != head.Label || len(entry.Args) != len(head.Params) {
			continue
		}
		l := &NaturalLoop{Header: head, Preheader: pre, Latch: latch, Entry: entry, Blocks: map[string]bool{head.Label: true}, Invariant: map[Reg]Reg{}, LiveOut: map[Reg]bool{}}
		l.Back, _ = latch.Term.(*TermJmp)
		stack := []*Block{latch}
		for len(stack) > 0 {
			b := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if l.Blocks[b.Label] {
				continue
			}
			l.Blocks[b.Label] = true
			stack = append(stack, pred[b.Label]...)
		}
		for name := range l.Blocks {
			if !dom[name][head.Label] {
				bad = true
			}
		}
		if bad {
			continue
		}
		for _, p := range f.Contract.Params {
			l.Invariant[p.Reg] = p.Reg
		}
		for r, b := range defs {
			if !l.Blocks[b.Label] && dom[pre.Label][b.Label] {
				l.Invariant[r] = r
			}
		}
		for i, p := range head.Params {
			initial, known := l.Invariant[entry.Args[i]]
			same := known
			args, ok := invariantEdgeArgs(latch.Term, head.Label)
			if !ok || len(args) != len(head.Params) || args[i] != p.Dst {
				same = false
			}
			if same {
				l.Invariant[p.Dst] = initial
			}
			if !ok || len(args) != len(head.Params) {
				continue
			}
			next := insts[args[i]]
			if next == nil || next.ClsHard || next.Ty != p.Ty {
				continue
			}
			var step int64
			switch next.Op {
			case OpPtrBump, OpField:
				if next.Src[0] != p.Dst || next.Imm == -1 {
					continue
				}
				step = next.Imm
			case OpAdd, OpSub:
				c := insts[next.Src[1]]
				if next.Src[0] != p.Dst || c == nil || c.Op != OpConst || c.ClsHard || c.Ty != p.Ty {
					continue
				}
				step = c.Imm
				if next.Op == OpSub {
					step = -step
				}
			default:
				continue
			}
			l.Inductions = append(l.Inductions, LoopInduction{p, entry.Args[i], args[i], step})
		}
		for _, b := range f.Blocks {
			if l.Blocks[b.Label] {
				exit := false
				for _, s := range b.Successors() {
					if !l.Blocks[s] {
						exit = true
					}
				}
				switch b.Term.(type) {
				case *TermRet, *TermCondRet, *TermErrorRet:
					exit = true
				}
				if exit {
					l.Exits = append(l.Exits, b)
				}
				for _, x := range b.Insts {
					if x.Op == OpLoad {
						l.Loads = append(l.Loads, x)
					}
					if x.Op == OpStore {
						l.Stores = append(l.Stores, x)
					}
					if x.Volatile || x.ClsHard || x.SequentialBytes || x.FallibleCY || len(x.SequentialByteOrigins) != 0 || len(x.SequentialAddressDefs) != 0 || x.Op == OpAsm || x.Op == OpCall || x.Op == OpCallIndirect || x.Op == OpCallCond || x.Op == OpPatch || x.Op == OpPatchSlot || x.Op == OpLoadPatched || x.Op == OpIn8 || x.Op == OpIn16 || x.Op == OpOut8 || x.Op == OpOut16 {
						l.Opaque = true
					}
				}
			} else {
				mark := func(r Reg) {
					if d := defs[r]; d != nil && l.Blocks[d.Label] {
						l.LiveOut[r] = true
					}
				}
				for _, x := range b.Insts {
					for _, r := range x.Uses() {
						mark(r)
					}
				}
				if b.Term != nil {
					for _, r := range b.Term.termUses() {
						mark(r)
					}
				}
			}
		}
		// A conditional return is itself an exit and can observe loop SSA values.
		for _, b := range l.Exits {
			if t, ok := b.Term.(*TermCondRet); ok {
				for _, r := range t.Vals {
					if d := defs[r]; d != nil && l.Blocks[d.Label] {
						l.LiveOut[r] = true
					}
				}
			}
		}
		out = append(out, l)
	}
	return out
}
