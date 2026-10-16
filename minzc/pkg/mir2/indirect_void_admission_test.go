package mir2

import "testing"

func TestIndirectVoidCallerAdmissionRequiresParameterABI(t *testing.T) {
	for _, typed := range []bool{false, true} {
		call := &Inst{Op: OpCallIndirect, Ty: TyVoid, Args: []Reg{2}, Src: [2]Reg{1}}
		if typed {
			call.IndirectParamTys = []Ty{TyU16}
		}
		m := &Module{Funcs: []*Func{{Name: "caller", Blocks: []*Block{{Label: "entry", Insts: []*Inst{call}, Term: &TermRet{Vals: []Reg{2}}}}}}}
		err := ValidateZ80IndirectCalls(m)
		if typed && err != nil {
			t.Fatal("known void ABI rejected", err)
		}
		if !typed && err == nil {
			t.Fatal("allocation-inferred parameter ABI admitted with live caller")
		}
	}
}
