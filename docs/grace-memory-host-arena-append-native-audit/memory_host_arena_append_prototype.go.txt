package pipeline

import "github.com/minz/minzc/pkg/hir"

// constructMemoryHostArenaAppend is private candidate construction. A separate
// aggregate validator must prove the whole-module delta before compilation.
func constructMemoryHostArenaAppend(m *hir.Module, w hir.MemoryHostWorkload, arena, appendCopy bool) error {
	if arena {
		if e := peelMemoryHostArenaFirstByte(m, w); e != nil {
			return e
		}
	}
	if appendCopy {
		if e := hoistMemoryHostAppendBases(m, w); e != nil {
			return e
		}
	}
	return nil
}
