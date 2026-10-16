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

// ISO C90 6.5.7: static address constants initialize real pointer values;
// array elements and struct fields retain offsets and target byte layout.
func TestC89GlobalRelocationsVMZ80(t *testing.T) {
	source, e := os.ReadFile(filepath.Join("testdata/global_relocations", "basic.c"))
	if e != nil {
		t.Fatal(e)
	}
	parse := func(t *testing.T) *hir.Module {
		t.Helper()
		m, e := c89.CompileWithOpts(string(source), "basic.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		for _, f := range m.Funcs {
			if f.Name == "make_pair" {
				if _, ok := f.RetTy.(*mir2.StructTy); !ok {
					t.Fatalf("address-taken return scalarized: %v", f.RetTy)
				}
			}
			if f.Name == "consume_pair" {
				if len(f.Params) != 1 {
					t.Fatalf("address-taken input scalarized: %v", f.Params)
				}
			}
		}
		if e = hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	inputs := []int64{0, 1, 2, 3, 5, 6, 7, 127, 128, 254, 255}
	model := func(f string, x int64) int64 {
		switch f {
		case "string_check":
			return int64("packet\x00"[x%7]) + int64("cket\x00"[x%5]) + int64("ABC"[x%3]) + int64("xy\x00"[x%3])
		case "address_check":
			return int64([]byte{7, 11, 13}[x%3]) + 0x1234 + x + 17
		case "pair_check":
			return (x << 8) | (x ^ 90)
		case "offset_check":
			return int64([]byte{4, 8}[x%2]) + int64([]byte{7, 11, 13}[x%3]) + 34
		case "static_check":
			return int64("ab\x00"[x%3]) + 9 + x + 17
		}
		panic(f)
	}
	check := func(t *testing.T, mm *mir2.Module) {
		t.Helper()
		if e := mir2.Verify(mm); e != nil {
			t.Fatal(e)
		}
		vm := mir2.NewVM(mm)
		for _, f := range []string{"string_check", "address_check", "static_check", "pair_check", "offset_check"} {
			for _, x := range inputs {
				v, e := vm.Call(f, []mir2.Value{{I: x}})
				if e != nil || len(v) != 1 || v[0].I != model(f, x) {
					t.Fatalf("VM %s(%d)=%v err=%v want=%d", f, x, v, e, model(f, x))
				}
			}
		}
	}
	t.Run("raw", func(t *testing.T) { check(t, hir.LowerModule(parse(t))) })
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			m := parse(t)
			for _, f := range []string{"string_check", "address_check", "static_check", "pair_check", "offset_check"} {
				for _, x := range inputs {
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: f, Args: []int64{x}, Expected: model(f, x), Via: "z80", Source: fmt.Sprintf("%s(%d)", f, x)})
				}
			}
			s, e := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if e != nil {
				t.Fatal(e)
			}
			check(t, s.MIR2Module)
		})
	}
}

// Typed void calls retain the caller's live local-object address.
func TestC89IndirectOutPointerAdmission(t *testing.T) {
	src, err := os.ReadFile("testdata/global_relocations/indirect_outptr.c")
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *hir.Module {
		m, err := c89.CompileWithOpts(string(src), "indirect_outptr.c", c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	check := func(mm *mir2.Module) {
		vm := mir2.NewVM(mm)
		for _, x := range []int64{0, 1, 127, 128, 255} {
			v, err := vm.Call("outptr_check", []mir2.Value{{I: x}})
			if err != nil || len(v) != 1 || v[0].I != (x<<8)|(x^90) {
				t.Fatal(x, v, err)
			}
		}
	}
	check(hir.LowerModule(parse()))
	for _, mode := range []string{"default", "grace", "size"} {
		m := parse()
		for _, x := range []int64{0, 1, 127, 128, 255} {
			m.Asserts = append(m.Asserts, hir.Assert{FuncName: "outptr_check", Args: []int64{x}, Expected: (x << 8) | (x ^ 90), Via: "z80"})
		}
		steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		check(steps.MIR2Module)
	}
}
