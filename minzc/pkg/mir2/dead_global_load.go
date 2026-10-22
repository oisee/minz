package mir2

// EliminateDeadGlobalLoads removes unused reads of ordinary global storage.
// Unknown pointers, absolute-address globals and volatile storage retain their
// reads: on Z80 these may be memory-mapped devices. Only direct global addresses
// are admitted; extending this to address arithmetic requires a bounds proof.
// Iterate with DSE because an unused expression can hide an unused load.
func EliminateDeadGlobalLoads(m *Module, f *Func) {
	ordinary := map[string]int{}
	for _, g := range m.Globals {
		if g.At == nil && g.Storage == StorageNormal && !g.Volatile && !HasVolatileStorage(g.Ty) {
			ordinary[g.Name] = ByteWidth(g.Ty)
		}
	}
	for {
		DeadStoreElim(f)
		addresses := map[Reg]int{}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == OpAddrOf && ordinary[i.Sym] > 0 {
					addresses[i.Dst] = ordinary[i.Sym]
				}
			}
		}
		uses := countRegUses(f)
		changed := false
		for _, b := range f.Blocks {
			keep := b.Insts[:0]
			for _, i := range b.Insts {
				if i.Op == OpLoad && i.Dst != NoReg && uses[i.Dst] == 0 && !i.Volatile && !i.SequentialBytes && len(i.SequentialByteOrigins) == 0 && len(i.SequentialAddressDefs) == 0 && i.Ty != nil {
					if size, ok := addresses[i.Src[0]]; ok && ByteWidth(i.Ty) > 0 && ByteWidth(i.Ty) <= size {
						changed = true
						continue
					}
				}
				keep = append(keep, i)
			}
			b.Insts = keep
		}
		if !changed {
			return
		}
	}
}
