package mir2

import "fmt"

// ValidateZ80IndirectCalls rejects calls whose live caller values cannot yet
// be protected reliably by the indirect standard ABI. Run before emission so
// an unsupported call is a compilation error rather than executable wrong code.
func ValidateZ80IndirectCalls(m *Module) error {
	for _, f := range m.Funcs {
		g := &z80cg{fn: f}
		for _, b := range f.Blocks {
			g.curBlock = b
			for _, inst := range b.Insts {
				if inst.Op != OpCallIndirect {
					continue
				}
				live := g.regsLiveAfterInst(inst)
				delete(live, inst.Dst)
				for _, r := range inst.ExtraRets {
					delete(live, r)
				}
				if len(live) > 0 {
					return fmt.Errorf("Z80: indirect call in %s has live values across it; caller preservation for the indirect ABI is unsupported", f.Name)
				}
			}
		}
	}
	return nil
}
