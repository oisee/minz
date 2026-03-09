package mir2

import "strings"

// EliminateDeadBlocks removes blocks unreachable from the entry block (f.Blocks[0]).
//
// A block is unreachable if no control-flow path from the entry leads to it.
// This happens after constant-folding eliminates branches, or when code is
// written after an unconditional return.
//
// The pass is a simple BFS over the CFG successor relation, then a compaction
// of f.Blocks to only live blocks. It is idempotent and safe to run before
// ReorderBlocks (which also ignores unreachable blocks, but keeps them).
func EliminateDeadBlocks(f *Func) {
	if len(f.Blocks) <= 1 {
		return
	}

	// Build label → index map.
	labelIdx := make(map[string]int, len(f.Blocks))
	for i, b := range f.Blocks {
		labelIdx[b.Label] = i
	}

	// BFS from the entry block.
	reachable := make([]bool, len(f.Blocks))
	reachable[0] = true
	queue := []int{0}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, succ := range termSuccessors(f.Blocks[cur].Term) {
			si, ok := labelIdx[succ]
			if !ok || reachable[si] {
				continue
			}
			reachable[si] = true
			queue = append(queue, si)
		}
	}

	// Compact to only reachable blocks (preserves relative order).
	live := f.Blocks[:0]
	for i, b := range f.Blocks {
		if reachable[i] {
			live = append(live, b)
		}
	}
	f.Blocks = live
}

// ReorderBlocks reorders f.Blocks to maximise fall-through opportunities.
//
// Algorithm: DFS preorder with hot-successor-first ordering.
//
//   - Hot successors (non-cold blocks) are visited before cold ones.
//   - Cold blocks: label contains "exit", "error", "fail", "cold", "base",
//     "end", "done", "slow", "unlikely".
//   - Entry block always stays first.
//
// This naturally produces layouts like:
//
//	entry → loop_init → loop_head → loop_body → exit(cold) → base(cold)
//
// Combined with the fall-through JP elimination in z80codegen, this eliminates
// most unconditional jumps on the hot path, and places rare code (error handling,
// early-exit cases) at the end of the function where JP forward-jumps are cheap.
//
// Compared to the original definition order, this typically saves 1–3 JP
// instructions on the hot path per loop.
func ReorderBlocks(f *Func) {
	if len(f.Blocks) <= 1 {
		return
	}

	order := dfsPreorder(f)

	newBlocks := make([]*Block, len(order))
	for i, idx := range order {
		newBlocks[i] = f.Blocks[idx]
	}
	f.Blocks = newBlocks
}

// dfsPreorder returns block indices in DFS preorder, hot-first.
func dfsPreorder(f *Func) []int {
	n := len(f.Blocks)

	// Build label → index map.
	idx := make(map[string]int, n)
	for i, b := range f.Blocks {
		idx[b.Label] = i
	}

	order := make([]int, 0, n)
	visited := make([]bool, n)

	var dfs func(i int)
	dfs = func(i int) {
		if visited[i] {
			return
		}
		visited[i] = true
		order = append(order, i)

		succs := termSuccessors(f.Blocks[i].Term)

		// Partition successors: hot first, cold last.
		// Within each partition, preserve the original successor order
		// (then before else for TermBrIf → naturally biases toward the
		// "then" path being hot, matching how code is written).
		hot := make([]int, 0, len(succs))
		cold := make([]int, 0, len(succs))
		for _, s := range succs {
			si, ok := idx[s]
			if !ok {
				continue
			}
			if isColdBlock(f.Blocks[si]) {
				cold = append(cold, si)
			} else {
				hot = append(hot, si)
			}
		}

		for _, si := range hot {
			dfs(si)
		}
		for _, si := range cold {
			dfs(si)
		}
	}

	dfs(0) // always start from entry (index 0)

	// Append any unreachable blocks (dead code) at the end.
	for i := range n {
		if !visited[i] {
			order = append(order, i)
		}
	}

	return order
}

// isColdBlock returns true when a block is unlikely to be on the hot path.
// Heuristic: block label contains a "cold" keyword.
func isColdBlock(b *Block) bool {
	label := strings.ToLower(b.Label)
	for _, kw := range coldKeywords {
		if strings.Contains(label, kw) {
			return true
		}
	}
	return false
}

var coldKeywords = []string{
	"exit", "error", "err", "fail",
	"cold", "slow", "unlikely",
	"base",  // early-exit (e.g. fibonacci base case n<=1)
	"end", "done",
	"ret",    // explicit return block
	"abort", "panic",
}
