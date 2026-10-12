package mir2

import "strings"

// These are the complete existing Pascal-generated output bodies, not a
// general privilege for compiler-named ASM. The callable ABI and physical
// input home must also match; CALL remains an all-clobber effect.
func cpmPascalOutputAdapter(f *Func, ar *AllocResult) []string {
	if f == nil || f.Attrs.IsExtern || f.Attrs.ExternAddr != 0 || f.Attrs.FallibleCY || len(f.Contract.Returns) != 0 || len(f.Blocks) != 1 {
		return nil
	}
	b := f.Blocks[0]
	ret, ok := b.Term.(*TermRet)
	if !ok || len(ret.Vals) != 0 || len(b.Insts) != 1 {
		return nil
	}
	i := b.Insts[0]
	if i.Op != OpAsm || i.Asm == nil || i.Dst != NoReg || len(i.Asm.Outs) != 0 || i.Asm.Clobbers != AllClasses {
		return nil
	}
	var body []string
	switch f.Name {
	case "WriteI16", "WriteU16":
		if len(f.Contract.Params) != 1 {
			return nil
		}
		p := f.Contract.Params[0]
		ty := TyU16
		code := PascalUnsignedDecimalASM
		if f.Name == "WriteI16" {
			ty = TyI16
			code = PascalSignedDecimalPrefix + " / " + strings.TrimPrefix(PascalUnsignedDecimalASM, "PUSH IX / ")
		}
		if p.Ty != ty || ar.Loc(p.Reg).Name != "HL" || len(i.Asm.Ins) != 1 || i.Asm.Ins[0] != p.Reg {
			return nil
		}
		return cpmExactDecimalBody(i, code)
	case "_wr16_dig":
		if len(f.Contract.Params) != 0 || len(i.Asm.Ins) != 0 {
			return nil
		}
		return cpmExactDecimalBody(i, PascalDecimalDigitASM)
	case "WriteCrLf":
		if len(f.Contract.Params) != 0 || len(i.Asm.Ins) != 0 {
			return nil
		}
		body = []string{"LDE,13", "LDC,2", "CALL5", "LDE,10", "LDC,2", "CALL5"}
	case "WriteStr":
		if len(f.Contract.Params) != 1 {
			return nil
		}
		p := f.Contract.Params[0]
		if p.Ty != TyU16 || ar.Loc(p.Reg).Name != "HL" || len(i.Asm.Ins) != 1 || i.Asm.Ins[0] != p.Reg {
			return nil
		}
		body = []string{"EXDE,HL", "LDC,9", "CALL0X0005"}
	default:
		return nil
	}
	lines := strings.Split(i.Asm.Template, "\n")
	if len(lines) != len(body) {
		return nil
	}
	for n, line := range lines {
		if strings.Join(strings.Fields(strings.ToUpper(line)), "") != body[n] {
			return nil
		}
	}
	return append(body, "RET")
}

func cpmExactDecimalBody(i *Inst, code string) []string {
	expected := []string{}
	for _, line := range strings.Split(code, "/") {
		line = strings.TrimSpace(line)
		if line != "" {
			expected = append(expected, line)
		}
	}
	actual := strings.Split(i.Asm.Template, "\n")
	if len(actual) != len(expected) {
		return nil
	}
	body := []string{}
	for n, line := range expected {
		if strings.Join(strings.Fields(strings.ToUpper(actual[n])), "") != strings.Join(strings.Fields(strings.ToUpper(line)), "") {
			return nil
		}

		if line != "" {
			body = append(body, line)
		}
	}
	return append(body, "RET")
}
