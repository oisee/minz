package mir2

import (
	"fmt"
	"strings"
)

// SMCSetterName is the source-level name. Codegen sanitizes it like any function.
func SMCSetterName(sym string) string {
	parts := strings.SplitN(sym, "$", 2)
	if len(parts) != 2 {
		return sym
	}
	return parts[0] + "_set_" + parts[1]
}

// SMCImmediateLabel retains the slot separator while canonicalizing identifiers.
func SMCImmediateLabel(sym string) string {
	parts := strings.SplitN(sym, "$", 2)
	if len(parts) != 2 {
		return sanitizeIdent(sym) + "$imm"
	}
	return sanitizeIdent(parts[0]) + "$" + sanitizeIdent(parts[1]) + "$imm"
}

// PatchNamed updates an owned named immediate; it has no slot register operand.
func (b *Builder) PatchNamed(sym string, value Reg, ty Ty) {
	b.emit(&Inst{Op: OpPatch, Src: [2]Reg{NoReg, value}, Sym: sym, Ty: ty})
}

func validateNamedSMC(m *Module) error {
	slots := map[string]*Inst{}
	for _, f := range m.Funcs {
		if f == nil {
			return fmt.Errorf("named SMC: nil function")
		}
		for _, b := range f.Blocks {
			if b == nil {
				return fmt.Errorf("named SMC: nil block in %s", f.Name)
			}
			for _, i := range b.Insts {
				if i == nil {
					return fmt.Errorf("named SMC: nil instruction in %s", f.Name)
				}
				if i.Op == OpPatchSlot && i.Sym != "" {
					parts := strings.SplitN(i.Sym, "$", 2)
					if len(parts) != 2 || parts[0] != f.Name || slots[i.Sym] != nil || i.Ty == nil || i.Ty.Width() != 16 {
						return fmt.Errorf("invalid or duplicate named SMC slot %s", i.Sym)
					}
					slots[i.Sym] = i
				}
			}
		}
	}
	for _, f := range m.Funcs {
		if f.SMCSetterFor != "" {
			sym := f.SMCSetterFor
			if slots[sym] == nil || f.Name != SMCSetterName(sym) || f.Attrs.IsExtern || !f.Attrs.FixedIndirectABI || len(f.Contract.Params) != 1 || f.Contract.Params[0].Ty == nil || f.Contract.Params[0].Ty.Width() != 16 || f.Contract.Params[0].Class != ClassPointer || len(f.Contract.Returns) != 0 || len(f.Blocks) != 1 || len(f.Blocks[0].Insts) != 1 {
				return fmt.Errorf("invalid SMC setter contract %s", f.Name)
			}
			i := f.Blocks[0].Insts[0]
			ret, ok := f.Blocks[0].Term.(*TermRet)
			if !ok || len(ret.Vals) != 0 || i.Op != OpPatch || i.Dst != NoReg || i.Sym != sym || i.Src[0] != NoReg || i.Src[1] != f.Contract.Params[0].Reg {
				return fmt.Errorf("invalid SMC setter body %s", f.Name)
			}
		}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == OpPatch && i.Sym != "" && (slots[i.Sym] == nil || i.Src[0] != NoReg || i.Src[1] == NoReg || i.Ty == nil || i.Ty.Width() != 16) {
					return fmt.Errorf("invalid named SMC patch %s", i.Sym)
				}
			}
		}
	}
	return nil
}
