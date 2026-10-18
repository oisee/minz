package mir2

import "testing"

func TestAgonFrontendIntrinsicOwnedBuffers(t *testing.T) {
	for _, bad := range []string{"", "missing", "short", "fixed", "constant", "word array", "argument", "wide result"} {
		t.Run(bad, func(t *testing.T) {
			call := &Inst{Op: OpCall, Sym: "@mir.agon.fs.read", Ty: TyU8, Dst: 1}
			m := &Module{Target: 2, Funcs: []*Func{{Name: "main", Blocks: []*Block{{Label: "entry", Insts: []*Inst{call}, Term: &TermRet{}}}}}, Globals: []Global{
				{Name: "_abap_fs_dma_buffer", Ty: NewArray(TyU8, 128)},
				{Name: "_abap_fs_agon_name_buffer", Ty: NewArray(TyU8, 13)},
				{Name: "_abap_fs_agon_info_buffer", Ty: NewArray(TyU8, 278)},
			}}
			switch bad {
			case "missing":
				m.Globals = m.Globals[1:]
			case "short":
				m.Globals[0].Ty = NewArray(TyU8, 127)
			case "fixed":
				addr := uint16(0xF000)
				m.Globals[0].At = &addr
			case "constant":
				m.Globals[0].IsConst = true
			case "word array":
				m.Globals[0].Ty = NewArray(TyU16, 128)
			case "argument":
				call.Args = []Reg{1}
			case "wide result":
				call.Ty = TyU16
			}
			if e := validateAgonZ80Runtime(m); (e == nil) != (bad == "") {
				t.Fatalf("%s admitted or valid refused: %v", bad, e)
			}
		})
	}
}
