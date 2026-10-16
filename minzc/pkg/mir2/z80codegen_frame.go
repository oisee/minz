package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/z80effects"
)

// nativeAllocaDepths proves a unique outstanding local allocation depth at each
// reachable block exit. Conditional disjoint returns are supported; unequal
// joins and allocations on cycles require a dynamic frame and are refused.
func nativeAllocaDepths(f *Func) (map[string]int, bool, error) {
	has := false
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op == OpAlloca {
				has = true
			}
		}
	}
	depths := map[string]int{}
	if !has || f.Entry() == nil {
		return depths, has, nil
	}
	incoming := map[string]int{f.Entry().Label: 0}
	queue := []string{f.Entry().Label}
	for len(queue) > 0 {
		label := queue[0]
		queue = queue[1:]
		b := f.BlockByLabel(label)
		if b == nil {
			return nil, true, fmt.Errorf("%s: local frame missing block %s", f.Name, label)
		}
		n := incoming[label]
		for _, i := range b.Insts {
			switch i.Op {
			case OpAlloca:
				if i.Imm < 0 || i.Imm > 65535 || int64(n)+i.Imm > 65535 {
					return nil, true, fmt.Errorf("%s: invalid local frame size", f.Name)
				}
				n += int(i.Imm)
			case OpPush, OpPop:
				return nil, true, fmt.Errorf("%s: local frame with explicit stack operations unsupported", f.Name)
			case OpAsm:
				if i.Asm == nil {
					return nil, true, fmt.Errorf("%s: local frame with missing ASM contract", f.Name)
				}
				// Re-read the actual template; lexical metadata is not a
				// preservation certificate. A straight-line block with no
				// SP access cannot change the outstanding allocation depth.
				for _, effect := range z80effects.Block(i.Asm.Template) {
					if !effect.Known || effect.Flow != z80effects.Fallthrough || (effect.Reads|effect.Writes)&z80effects.SP != 0 {
						return nil, true, fmt.Errorf("%s: local frame requires known fallthrough ASM without SP access", f.Name)
					}
				}
			}
		}
		depths[label] = n
		if b.Term == nil {
			continue
		}
		for _, s := range b.Term.Successors() {
			if old, ok := incoming[s]; ok {
				if old != n {
					return nil, true, fmt.Errorf("%s: variable local frame depth at %s (%d versus %d)", f.Name, s, old, n)
				}
			} else {
				incoming[s] = n
				queue = append(queue, s)
			}
		}
	}
	return depths, true, nil
}

// INC SP preserves all return registers and comparison flags. Allocation
// pointers are no longer used after return copies have been resolved.
func (g *z80cg) emitLocalFrameCleanup(n int) {
	for range n {
		g.emit("    INC SP    ; release local frame")
	}
}
