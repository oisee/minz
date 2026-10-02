package mir2

import "testing"

func TestZ80RuntimeSymbolsReserved(t *testing.T) {
	for _, name := range []string{"__mul8", "__mul16", "__call_ix", "__rotate_0", "__rotate_4", "__rotate_7", "_mir2_str_0", "_mir2_str_999", "@mir2.str.0", "_spill_main_r1", "_tsmc_main_r1_0"} {
		for _, global := range []bool{false, true} {
			t.Run(name, func(t *testing.T) {
				m := &Module{}
				if global {
					m.Globals = []Global{{Name: name, Ty: TyU8}}
				} else {
					f := m.AddFunc(name)
					f.Attrs.IsExtern = true
					f.Contract.ExternClobbers = []string{}
				}
				if err := ValidateZ80Symbols(m); err == nil {
					t.Fatal("accepted reserved symbol")
				}
				if asm, err := Z80Codegen(m, &AllocResult{}); err == nil || asm != "" {
					t.Fatalf("codegen = %q, %v", asm, err)
				}
			})
		}
	}
}

func TestZ80GeneratedAliasesReserved(t *testing.T) {
	t.Run("field", func(t *testing.T) {
		m := &Module{Globals: []Global{{Name: "obj", Ty: &StructTy{Name: "S", Fields: []StructField{{Name: "value", Ty: TyU8}}}}}}
		m.AddFunc("obj__value")
		if err := ValidateZ80Symbols(m); err == nil {
			t.Fatal("accepted field alias collision")
		}
	})
	t.Run("patcher", func(t *testing.T) {
		m := &Module{}
		f := m.AddFunc("draw")
		f.Blocks = []*Block{{Insts: []*Inst{{Op: OpPatchSlot, Sym: "draw$x", Ty: TyU16}}}}
		m.AddFunc("draw_set_x")
		if err := ValidateZ80Symbols(m); err == nil {
			t.Fatal("accepted patcher collision")
		}
	})
}
