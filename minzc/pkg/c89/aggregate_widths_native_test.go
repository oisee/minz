package c89_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

// C90 6.3.2.2/6.6.6.4: aggregate argument/result values carry all fields.
// Exercise all eleven ordered compositions of 1/2-byte scalar fields up to
// four bytes, plus signed value transport in two layouts. One-field structs
// and each position of a word field are included; arithmetic avoids C signed UB.
func TestC89AggregateScalarWidthVMZ80(t *testing.T) {
	src, e := os.ReadFile("testdata/aggregate_widths/shapes.c")
	if e != nil {
		t.Fatal(e)
	}
	specs := []struct {
		name  string
		types []mir2.Ty
		want  func(byte) uint16
	}{
		{"bytesingle", []mir2.Ty{mir2.TyU8}, func(x byte) uint16 { return uint16(x^90)*257 + uint16(x) }},
		{"wordsingle", []mir2.Ty{mir2.TyU16}, func(x byte) uint16 { return ((uint16(x)*257 + 0x1357) ^ 0x5a5a) + uint16(x) }},
		{"bytepair", []mir2.Ty{mir2.TyU8, mir2.TyU8}, func(x byte) uint16 { return ((uint16(x) << 4) ^ uint16(x^90)) + uint16(x) }},
		{"byteword", []mir2.Ty{mir2.TyU8, mir2.TyU16}, func(x byte) uint16 { return ((uint16(x) << 4) ^ (uint16(x)*257 + 0x1357)) + uint16(x) }},
		{"wordbyte", []mir2.Ty{mir2.TyU16, mir2.TyU8}, func(x byte) uint16 { return ((uint16(x)*257 + 0x1357) ^ (uint16(x^90) << 4)) + uint16(x) }},
		{"middleword", []mir2.Ty{mir2.TyU8, mir2.TyU16, mir2.TyU8}, func(x byte) uint16 { return ((uint16(x) << 4) ^ (uint16(x)*257 + 0x1357) ^ uint16(x^90)) + uint16(x) }},
		{"tailword", []mir2.Ty{mir2.TyU8, mir2.TyU8, mir2.TyU16}, func(x byte) uint16 { return ((uint16(x) << 4) ^ uint16(x^90) ^ (uint16(x)*257 + 0x1357)) + uint16(x) }},
		{"signedmixed", []mir2.Ty{mir2.TyI16, mir2.TyI8, mir2.TyI8}, func(x byte) uint16 {
			return uint16((int16(x)-128)*32 + (int16(x&127)-64)*4 + (int16(x>>1) - 64) + int16(x))
		}},
		{"signedtail", []mir2.Ty{mir2.TyI8, mir2.TyI8, mir2.TyI16}, func(x byte) uint16 {
			return uint16((int16(x)-128)*32 + (int16(x&127)-64)*4 + (int16(x>>1) - 64) + int16(x))
		}},
		{"triple", []mir2.Ty{mir2.TyU8, mir2.TyU8, mir2.TyU8}, func(x byte) uint16 { return ((uint16(x) << 8) ^ (uint16(x+1) << 4) ^ uint16(x^90)) + uint16(x) }},
		{"quad", []mir2.Ty{mir2.TyU8, mir2.TyU8, mir2.TyU8, mir2.TyU8}, func(x byte) uint16 {
			return ((uint16(x) << 8) ^ (uint16(x+1) << 4) ^ (uint16(x^90) << 2) ^ uint16(x+3)) + uint16(x)
		}},
		{"words", []mir2.Ty{mir2.TyU16, mir2.TyU16}, func(x byte) uint16 { v := uint16(x) * 257; return (v ^ (v + 3)) + uint16(x) }},
		{"mixed", []mir2.Ty{mir2.TyU16, mir2.TyU8, mir2.TyU8}, func(x byte) uint16 { return (uint16(x)*257 ^ (uint16(x+1) << 4) ^ uint16(x^90)) + uint16(x) }},
	}
	parse := func() *hir.Module {
		m, e := c89.CompileWithOpts(string(src), "shapes.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	inputs := []int64{0, 1, 2, 7, 15, 16, 63, 127, 128, 253, 254, 255}
	check := func(mm *mir2.Module) {
		for _, s := range specs {
			maker := mm.FuncByName("make_" + s.name)
			consumer := mm.FuncByName("consume_" + s.name)
			if maker == nil || consumer == nil || len(maker.Contract.Returns) != len(s.types) || len(consumer.Contract.Params) != len(s.types) {
				t.Fatalf("%s not decomposed: maker%v consumer%v", s.name, maker, consumer)
			}
			for i, ty := range s.types {
				if maker.Contract.Returns[i].Ty != ty || consumer.Contract.Params[i].Ty != ty {
					t.Fatal("tuple type lost", s.name, i)
				}
			}
		}
		for _, f := range mm.Funcs {
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					if i.Op == mir2.OpAlloca {
						t.Fatal("eligible scalar shape still allocates", f.Name)
					}
				}
			}
		}
		vm := mir2.NewVM(mm)
		for _, s := range specs {
			for _, x := range inputs {
				v, e := vm.Call(s.name+"_check", []mir2.Value{{I: x}})
				if e != nil || len(v) != 1 || v[0].I != int64(s.want(byte(x))) {
					t.Fatal(s.name, x, v, e, s.want(byte(x)))
				}
			}
		}
	}
	raw := hir.LowerModule(parse())
	if e = mir2.Verify(raw); e != nil {
		t.Fatal(e)
	}
	check(raw)
	for _, mode := range []string{"default", "grace", "size", "no-contract"} {
		t.Run(mode, func(t *testing.T) {
			m := parse()
			for _, s := range specs {
				for _, x := range inputs {
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: s.name + "_check", Args: []int64{x}, Expected: int64(s.want(byte(x))), Via: "z80", Source: fmt.Sprintf("%s(%d)", s.name, x)})
				}
			}
			steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: mode != "no-contract", UseGrace: mode == "grace" || mode == "size", UseHIRGrace: mode == "grace" || mode == "size", OptSize: mode == "size"})
			if dir := os.Getenv("MINZ_C89_WIDTH_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				final := ""
				if steps.MIR2Module != nil {
					final = steps.MIR2Module.Dump()
				}
				for ext, data := range map[string]string{"a80": steps.Assembly, "raw.mir": steps.MIR2Raw, "final.mir": final} {
					if err := os.WriteFile(filepath.Join(dir, mode+"."+ext), []byte(data), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if e != nil {
				t.Fatal(e)
			}
			check(steps.MIR2Module)
		})
	}
}
