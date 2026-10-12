package mir2

// TransferLoopOrigin copies attribution when a pass clones, moves, or folds
// source's contents into target. Both labels must exist. Callers moving a body
// must delete the source attribution after a successful transfer.
//
// The single-origin representation cannot describe fusion of different sites,
// roles, or rules. In that case this returns false without changing metadata;
// callers must retain the separate blocks. This is an attribution constraint,
// independent of the semantic safety checks required by the transformation.
func (f *Func) TransferLoopOrigin(source, target string) bool {
	if !f.canTransferLoopOrigin(source, target) {
		return false
	}
	if origin, ok := f.LoopOrigins[source]; ok {
		f.LoopOrigins[target] = origin
	}
	return true
}

func (f *Func) canTransferLoopOrigin(source, target string) bool {
	if f.BlockByLabel(source) == nil || f.BlockByLabel(target) == nil {
		return false
	}
	origin, ok := f.LoopOrigins[source]
	if !ok {
		return true
	}
	if existing, ok := f.LoopOrigins[target]; ok && existing != origin {
		return false
	}
	return true
}

// removeBlock removes only the named block and its attribution. It does not
// repair CFG edges or silently discard unrelated invalid metadata.
func (f *Func) removeBlock(label string) {
	for i, b := range f.Blocks {
		if b.Label == label {
			f.Blocks = append(f.Blocks[:i], f.Blocks[i+1:]...)
			delete(f.LoopOrigins, label)
			return
		}
	}
}
