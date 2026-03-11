package mir2

// Liveness analysis for MIR2 functions.
//
// Algorithm: classic backward dataflow over basic blocks.
//
//	LiveOut[b] = ∪  LiveIn[s]   (for all successors s of b)
//	LiveIn[b]  = Use[b] ∪ (LiveOut[b] \ Def[b])
//
// Block-argument semantics:
//   - Block params (Dst regs) are treated as definitions at block entry → in Def[b].
//   - Terminator argument lists (jmp/br_if args) are treated as uses at block exit → in Use[b].
//   - Function contract params are treated as definitions in the entry block.
//
// Iteration order: reverse postorder (RPO) of the CFG; typically converges in
// 2–3 passes even for loops.

// ── RegSet ─────────────────────────────────────────────────────────────────────

// RegSet is a dense bitset of virtual Reg numbers.
// Reg is 1-based; slot 0 is unused.
type RegSet struct {
	words []uint64
}

func newRegSet() *RegSet { return &RegSet{} }

// NewRegSetForTest exports newRegSet for use in external test packages.
func NewRegSetForTest() *RegSet { return newRegSet() }

func (s *RegSet) grow(r Reg) {
	need := int(r)/64 + 1
	for len(s.words) < need {
		s.words = append(s.words, 0)
	}
}

// Add inserts r into the set.
func (s *RegSet) Add(r Reg) {
	if r == NoReg {
		return
	}
	s.grow(r)
	s.words[int(r)/64] |= 1 << (uint(r) % 64)
}

// Remove deletes r from the set.
func (s *RegSet) Remove(r Reg) {
	if r == NoReg {
		return
	}
	idx := int(r) / 64
	if idx >= len(s.words) {
		return
	}
	s.words[idx] &^= 1 << (uint(r) % 64)
}

// Has reports whether r is in the set.
func (s *RegSet) Has(r Reg) bool {
	if r == NoReg {
		return false
	}
	idx := int(r) / 64
	if idx >= len(s.words) {
		return false
	}
	return s.words[idx]&(1<<(uint(r)%64)) != 0
}

// UnionWith adds all registers from o into s.  Returns true if s changed.
func (s *RegSet) UnionWith(o *RegSet) (changed bool) {
	if len(o.words) > len(s.words) {
		extra := make([]uint64, len(o.words)-len(s.words))
		s.words = append(s.words, extra...)
	}
	for i, w := range o.words {
		before := s.words[i]
		s.words[i] |= w
		if s.words[i] != before {
			changed = true
		}
	}
	return changed
}

// SubtractWith removes all registers in o from s.
func (s *RegSet) SubtractWith(o *RegSet) {
	n := len(o.words)
	if len(s.words) < n {
		n = len(s.words)
	}
	for i := range n {
		s.words[i] &^= o.words[i]
	}
}

// Clone returns a deep copy of s.
func (s *RegSet) Clone() *RegSet {
	c := &RegSet{words: make([]uint64, len(s.words))}
	copy(c.words, s.words)
	return c
}

// Equal reports whether s and o represent the same set.
func (s *RegSet) Equal(o *RegSet) bool {
	// Normalise lengths: trailing zero words are irrelevant.
	sw := trimTrailingZero(s.words)
	ow := trimTrailingZero(o.words)
	if len(sw) != len(ow) {
		return false
	}
	for i := range sw {
		if sw[i] != ow[i] {
			return false
		}
	}
	return true
}

// Each calls fn for every register in the set (in ascending order).
func (s *RegSet) Each(fn func(Reg)) {
	for wi, w := range s.words {
		for w != 0 {
			bit := w & (-w) // lowest set bit
			pos := trailingZeros64(bit)
			fn(Reg(wi*64 + pos))
			w &^= bit
		}
	}
}

// Len returns the number of registers in the set.
func (s *RegSet) Len() int {
	n := 0
	for _, w := range s.words {
		n += popcount64(w)
	}
	return n
}

// Slice returns all registers as a sorted slice.
func (s *RegSet) Slice() []Reg {
	out := make([]Reg, 0, s.Len())
	s.Each(func(r Reg) { out = append(out, r) })
	return out
}

func trimTrailingZero(ws []uint64) []uint64 {
	n := len(ws)
	for n > 0 && ws[n-1] == 0 {
		n--
	}
	return ws[:n]
}

func trailingZeros64(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	for x&1 == 0 {
		x >>= 1
		n++
	}
	return n
}

func popcount64(x uint64) int {
	n := 0
	for x != 0 {
		n += int(x & 1)
		x >>= 1
	}
	return n
}

// ── LivenessResult ─────────────────────────────────────────────────────────────

// LivenessResult holds per-block liveness sets for a function.
// Blocks are indexed by their position in f.Blocks.
type LivenessResult struct {
	// LiveIn[i]: registers live at the *entry* of f.Blocks[i].
	LiveIn []*RegSet
	// LiveOut[i]: registers live at the *exit* of f.Blocks[i].
	LiveOut []*RegSet
}

// LiveInOf returns the live-in set for block b (nil if block not found).
func (lr *LivenessResult) LiveInOf(f *Func, b *Block) *RegSet {
	for i, blk := range f.Blocks {
		if blk == b {
			return lr.LiveIn[i]
		}
	}
	return nil
}

// LiveOutOf returns the live-out set for block b.
func (lr *LivenessResult) LiveOutOf(f *Func, b *Block) *RegSet {
	for i, blk := range f.Blocks {
		if blk == b {
			return lr.LiveOut[i]
		}
	}
	return nil
}

// ── Main analysis ──────────────────────────────────────────────────────────────

// ComputeLiveness runs backward dataflow liveness analysis on f.
// Iteration order is reverse postorder for fast convergence on loops.
func ComputeLiveness(f *Func) *LivenessResult {
	n := len(f.Blocks)
	if n == 0 {
		return &LivenessResult{}
	}

	// Build block-label → index map.
	idx := make(map[string]int, n)
	for i, b := range f.Blocks {
		idx[b.Label] = i
	}

	// Compute per-block Use and Def sets.
	use := make([]*RegSet, n)
	def := make([]*RegSet, n)
	for i, b := range f.Blocks {
		use[i], def[i] = blockUseDef(f, b, i == 0)
	}

	// Compute reverse postorder (RPO) for efficient iteration.
	rpo := reversePostorder(f, idx)

	// Initialise live sets.
	liveIn := make([]*RegSet, n)
	liveOut := make([]*RegSet, n)
	for i := range n {
		liveIn[i] = newRegSet()
		liveOut[i] = newRegSet()
	}

	// Fixed-point iteration in RPO (backward: iterate RPO reversed = postorder).
	for {
		changed := false
		for ri := n - 1; ri >= 0; ri-- {
			i := rpo[ri]
			b := f.Blocks[i]

			// LiveOut[i] = ∪ LiveIn[succ] for all successors.
			newOut := newRegSet()
			for _, succLabel := range termSuccessors(b.Term) {
				if si, ok := idx[succLabel]; ok {
					newOut.UnionWith(liveIn[si])
				}
			}

			// LiveIn[i] = Use[i] ∪ (LiveOut[i] \ Def[i]).
			newIn := newOut.Clone()
			newIn.SubtractWith(def[i])
			newIn.UnionWith(use[i])

			if !newOut.Equal(liveOut[i]) || !newIn.Equal(liveIn[i]) {
				changed = true
				liveOut[i] = newOut
				liveIn[i] = newIn
			}
		}
		if !changed {
			break
		}
	}

	return &LivenessResult{LiveIn: liveIn, LiveOut: liveOut}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// blockUseDef computes the upward-exposed uses and definitions for block b.
//
//   - Block parameters are definitions (defined at block entry).
//   - Function contract params are extra definitions for the entry block.
//   - Instruction sources (not yet defined in the block) are upward-exposed uses.
//   - Instruction destinations are definitions.
//   - Terminator operands (cond, args) are uses (they must be live at exit).
func blockUseDef(f *Func, b *Block, isEntry bool) (use *RegSet, def *RegSet) {
	use = newRegSet()
	def = newRegSet()

	// Function params are "defined" at the start of the entry block.
	if isEntry {
		for _, p := range f.Contract.Params {
			def.Add(p.Reg)
		}
	}

	// Block parameters: defined at block entry.
	for _, p := range b.Params {
		def.Add(p.Dst)
	}

	// Instructions: sources before defs are upward-exposed.
	addUse := func(r Reg) {
		if r != NoReg && !def.Has(r) {
			use.Add(r)
		}
	}
	for _, inst := range b.Insts {
		// Operand sources.
		for _, src := range inst.Src {
			addUse(src)
		}
		// Call / asm argument lists.
		for _, arg := range inst.Args {
			addUse(arg)
		}
		if inst.Asm != nil {
			for _, r := range inst.Asm.Ins {
				addUse(r)
			}
		}
		// Destination is a new definition.
		if inst.Dst != NoReg {
			def.Add(inst.Dst)
		}
		// Extra return-value regs for multi-return calls are also defs.
		for _, r := range inst.ExtraRets {
			if r != NoReg {
				def.Add(r)
			}
		}
	}

	// Terminator operands are uses at block exit.
	switch t := b.Term.(type) {
	case *TermJmp:
		for _, arg := range t.Args {
			addUse(arg)
		}
	case *TermBrIf:
		addUse(t.Cond)
		for _, arg := range t.ThenArgs {
			addUse(arg)
		}
		for _, arg := range t.ElseArgs {
			addUse(arg)
		}
	case *TermBrIf2:
		addUse(t.Lhs)
		addUse(t.Rhs)
		for _, arg := range t.EqArgs {
			addUse(arg)
		}
		for _, arg := range t.LtArgs {
			addUse(arg)
		}
		for _, arg := range t.GtArgs {
			addUse(arg)
		}
	case *TermDJNZ:
		addUse(t.Counter)
		for _, arg := range t.BodyArgs {
			addUse(arg)
		}
		for _, arg := range t.ExitArgs {
			addUse(arg)
		}
	case *TermCondRet:
		addUse(t.Cond)
		for _, v := range t.Vals {
			addUse(v)
		}
		for _, arg := range t.ThenArgs {
			addUse(arg)
		}
	case *TermRet:
		for _, v := range t.Vals {
			addUse(v)
		}
	}

	return use, def
}

// termSuccessors returns the labels of all successors of terminator t.
func termSuccessors(t Term) []string {
	switch t := t.(type) {
	case *TermJmp:
		return []string{t.Target}
	case *TermBrIf:
		return []string{t.Then, t.Else}
	case *TermBrIf2:
		return []string{t.Eq, t.Lt, t.Gt}
	case *TermDJNZ:
		return []string{t.Body, t.Exit}
	case *TermCondRet:
		return []string{t.Then}
	case *TermRet, *TermUnreachable:
		return nil
	}
	return nil
}

// reversePostorder returns block indices in reverse postorder (entry first).
// RPO visits a block before any of its successors that weren't already visited —
// ideal for forward dataflow; reversed RPO (postorder) is ideal for backward.
func reversePostorder(f *Func, idx map[string]int) []int {
	n := len(f.Blocks)
	visited := make([]bool, n)
	po := make([]int, 0, n)

	var dfs func(i int)
	dfs = func(i int) {
		if visited[i] {
			return
		}
		visited[i] = true
		b := f.Blocks[i]
		for _, succ := range termSuccessors(b.Term) {
			if si, ok := idx[succ]; ok {
				dfs(si)
			}
		}
		po = append(po, i)
	}

	dfs(0) // start from entry
	// Any blocks unreachable from entry (dead code) — append in order.
	for i := range n {
		if !visited[i] {
			po = append(po, i)
		}
	}

	// Reverse to get RPO.
	for l, r := 0, len(po)-1; l < r; l, r = l+1, r-1 {
		po[l], po[r] = po[r], po[l]
	}
	return po
}
