package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// Book v7 §19.2-19.3: named pipes splice doubled then +1; §6 range
// counts n..1. §17 imports change names, not arithmetic. Host oracles use
// uint8 arithmetic; no HIR/compiler execution computes the expected values.
func TestOriginalPipeAndImportNativeModels(t *testing.T) {
	type invocation struct {
		Entry    string
		Expected byte
		Args     []int64
	}
	rangeSum := func(n int, composed bool) byte {
		var acc byte
		for x := n; x > 0; x-- {
			v := byte(2 * x)
			if composed {
				v++
			}
			acc += v
		}
		return acc
	}
	cases := []struct {
		file  string
		calls []invocation
	}{
		{"ex28_pipe.nanz", []invocation{{Entry: "sum_doubled", Expected: rangeSum(5, false)}, {Entry: "sum_tripled", Expected: rangeSum(3, true)}}},
		{"ex23_import_unqualified.nanz", nil},
		{"ex24_import_qualified.nanz", []invocation{{Entry: "compute", Expected: 25}}},
		{"ex25_import_alias_glob.nanz", []invocation{{Entry: "compute", Expected: 10}}},
		{"mathlib/ops.nanz", nil},
	}
	for ci := range cases {
		c := cases[ci]
		if c.file == "ex23_import_unqualified.nanz" {
			for x := 0; x < 256; x++ {
				c.calls = append(c.calls, invocation{"compute", byte(2*x + 1), []int64{int64(x)}})
			}
		}
		if c.file == "mathlib/ops.nanz" {
			for x := 0; x < 256; x++ {
				for _, f := range []string{"double", "triple"} {
					k := 2
					if f == "triple" {
						k = 3
					}
					c.calls = append(c.calls, invocation{f, byte(k * x), []int64{int64(x)}})
				}
				for _, y := range []int64{0, 1, 127, 128, 255} {
					c.calls = append(c.calls, invocation{"add", byte(x + int(y)), []int64{int64(x), y}})
				}
			}
		}
		t.Run(filepath.Base(c.file), func(t *testing.T) {
			path := filepath.Join("../../../reports/showcase-src/2026-03-14", c.file)
			source, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			parse := func() *hir.Module {
				h, e := nanz.ParseWithOpts(string(source), filepath.Base(path), nanz.ParseOpts{BaseDir: filepath.Dir(path)})
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			// Keep every original assertion: default CompileHIRSteps executes its
			// expected values on both final VM and actual native target.
			base := parse()
			assertions := len(base.Asserts)
			raw := mir2.NewVM(hir.LowerModule(parse()))
			raw.MaxSteps = 1000000
			steps, e := CompileHIRSteps(base)
			if e != nil {
				t.Fatal(e)
			}
			final := mir2.NewVM(steps.MIR2Module)
			final.MaxSteps = 1000000
			// Actual ABI-aware bootstrap for each source entry. Only workload code is
			// added to assembly; original functions/algorithm stay byte-for-byte compiled.
			asm := "ORG 0x8000\n" + steps.Assembly
			for i, call := range c.calls {
				f := steps.MIR2Module.FuncByName(call.Entry)
				if f == nil {
					t.Fatalf("missingentry%s", call.Entry)
				}
				asm += fmt.Sprintf("\noracle_call_%d:\n", i)
				for j, v := range call.Args {
					loc := assertParamLocation(f.Contract.Params[j], steps.Allocation)
					if loc.Kind != mir2.LocReg {
						t.Fatalf("unsupported bootstrapABI%s", loc.Name)
					}
					asm += fmt.Sprintf(" LD %s,%d\n", loc.Name, v)
				}
				asm += fmt.Sprintf(" CALL %s\n RET\n", call.Entry)
			}
			assembled, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(assembled.Errors) != 0 {
				t.Fatalf("assembly%v %v", e, assembled.Errors)
			}
			if len(assembled.Binary) > 0x7000 {
				t.Fatal("image overlapsstack")
			}
			z := emulator.NewRemogattoZ80()
			if e = z.LoadMemory(0x8000, assembled.Binary); e != nil {
				t.Fatal(e)
			}
			results := []byte{}
			for i, call := range c.calls {
				args := []mir2.Value{}
				for _, v := range call.Args {
					args = append(args, mir2.Value{I: v})
				}
				for mode, vm := range map[string]*mir2.VM{"raw": raw, "final": final} {
					r, e := vm.Call(call.Entry, args)
					if e != nil || len(r) != 1 || r[0].I != int64(call.Expected) {
						t.Fatalf("%s call%d%s got%v err%v want%d", mode, i, call.Entry, r, e, call.Expected)
					}
				}
				mem := z.Memory()
				mem[0xeff0], mem[0xeff1] = 0, 0x70
				mem[0xe000], mem[0xeff2] = 0x9b, 0xad
				z.SetRegisters(emulator.Registers{PC: uint16(assembled.Symbols[fmt.Sprintf("oracle_call_%d", i)]), SP: 0xeff0})
				done := false
				for j := 0; j < 100000; j++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						done = true
						break
					}
					if z.GetSP() <= 0xe000 || z.GetSP() > 0xeff2 {
						t.Fatal("stackbounds")
					}
					z.Step()
				}
				if !done || z.GetRegisters().A != call.Expected || mem[0xe000] != 0x9b || mem[0xeff2] != 0xad || !bytes.Equal(mem[0x8000:0x8000+len(assembled.Binary)], assembled.Binary) {
					t.Fatalf("nativecall%d%s return%t A%d want%d", i, call.Entry, done, z.GetRegisters().A, call.Expected)
				}
				results = append(results, z.GetRegisters().A)
			}
			if dir := os.Getenv("MINZ_PIPE_IMPORT_ORACLE_ARTIFACTS"); dir != "" {
				out := filepath.Join(dir, filepath.Base(c.file))
				if e = os.MkdirAll(out, 0755); e != nil {
					t.Fatal(e)
				}
				b, _ := json.MarshalIndent(map[string]any{"source_path": path, "source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "calls": c.calls, "native_results": results, "source_assertions": assertions}, "", "  ")
				for name, data := range map[string][]byte{"source.nanz": source, "target.a80": []byte(asm), "target.bin": assembled.Binary, "oracle.json": b} {
					if e = os.WriteFile(filepath.Join(out, name), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			t.Logf("exact %s: %d calls raw/finalVM/native model, %d original assertions retained; SP/guards/immutableimage %d B SHA %x", c.file, len(c.calls), assertions, len(assembled.Binary), sha256.Sum256(assembled.Binary))
		})
	}
}
