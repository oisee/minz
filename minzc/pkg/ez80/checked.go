package ez80

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
)

// MIR2CodegenChecked is the deliberately narrow, experimental flat24 entry.
// It accepts straight-line byte-memory kernels without calls or parameters.
// The older MIR2Codegen API remains experimental and unchecked. Expand this
// admission policy only alongside CPU execution tests for the new operations.
func MIR2CodegenChecked(m *mir2.Module, ar *mir2.AllocResult) (string, error) {
	if m.PointerBits != 24 {
		return "", fmt.Errorf("flat24 requires an explicit 24-bit pointer ABI")
	}
	if err := mir2.Verify(m); err != nil {
		return "", err
	}
	if ar == nil {
		return "", fmt.Errorf("flat24 requires allocation results")
	}
	if len(m.Globals) != 0 || m.Strings.Len() != 0 {
		return "", fmt.Errorf("flat24 native globals and strings are not admitted yet")
	}
	ct := EZ80CostTable{}
	classicByte := func(r mir2.Reg) bool { return strings.Contains(" A B C D E H L ", " "+ar.Loc(r).Name+" ") }
	for _, f := range m.Funcs {
		if len(f.Contract.Params) != 0 || len(f.Blocks) != 1 || len(f.Blocks[0].Params) != 0 {
			return "", fmt.Errorf("flat24 native %s: only straight-line kernels without parameters are admitted", f.Name)
		}
		ret, ok := f.Blocks[0].Term.(*mir2.TermRet)
		if !ok || len(ret.Vals) != 1 || len(f.Contract.Returns) != 1 || f.Contract.Returns[0].Ty != mir2.TyU8 || f.Contract.Returns[0].Class != mir2.ClassAcc {
			return "", fmt.Errorf("flat24 native %s: requires one u8 return", f.Name)
		}
		for _, i := range f.Blocks[0].Insts {
			if i.Dst != mir2.NoReg && !ct.Compatible(i.Ty, ar.Loc(i.Dst)) {
				return "", fmt.Errorf("flat24 native %s: incompatible allocation %s for %s", f.Name, ar.Loc(i.Dst).Name, i.Ty)
			}
			switch i.Op {
			case mir2.OpConst, mir2.OpMove:
				if i.Ty.Width() > 24 {
					return "", fmt.Errorf("flat24 native value exceeds register width")
				}
			case mir2.OpExt:
				if i.SrcTy != mir2.TyU8 || i.Ty.Width() <= 8 || i.Ty.Width() > 24 || !classicByte(i.Src[0]) {
					return "", fmt.Errorf("flat24 native extension is not admitted")
				}
			case mir2.OpPtrAdd:
				if i.Ty.Width() != 24 || !isPair(ar.Loc(i.Src[0]).Name) || !isPair(ar.Loc(i.Src[1]).Name) {
					return "", fmt.Errorf("flat24 native pointer addition requires pairs")
				}
			case mir2.OpLoad:
				if i.Ty != mir2.TyU8 || !classicByte(i.Dst) || !isPair(ar.Loc(i.Src[0]).Name) || i.Imm != 0 {
					return "", fmt.Errorf("flat24 native load is not admitted")
				}
			default:
				return "", fmt.Errorf("flat24 native %s: unsupported %s", f.Name, i.Op)
			}
		}
	}
	asm := MIR2Codegen(m, ar)
	if strings.Contains(asm, "TODO") {
		return "", fmt.Errorf("flat24 native emitter reached an unimplemented operation")
	}
	return asm, nil
}
