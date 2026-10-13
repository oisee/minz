package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Book v7 §3.1 u8 wrap arithmetic; all original source entries and assertions.
func TestOriginalLizpLanzFamilyState(t *testing.T) {
	data, e := os.ReadFile("../../../reports/lizp-lanz-family-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		Sources map[string]string `json:"sources"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil {
		t.Fatal("freeze", e)
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_LIZP_LANZ_FAMILY_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		switch x := v.(type) {
		case string:
			b = []byte(x)
		case []byte:
			b = x
		default:
			b, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for _, module := range []string{"examples/lizp/factorial.lizp", "examples/lizp/hello.lizp", "reports/showcase-src/2026-03-15/ex29_lanz.lanz"} {
		path := "../../../" + module
		source, e := os.ReadFile(path)
		if e != nil || freeze.Sources[module] != fmt.Sprintf("%x", sha256.Sum256(source)) {
			t.Fatal("sourcefreeze", module, e)
		}
		parse := func() *hir.Module {
			var h *hir.Module
			var e error
			switch filepath.Ext(module) {
			case ".lanz":
				h, e = lanz.Compile(string(source), path)
			case ".lizp":
				h, e = lizp.Compile(string(source), path)
			}
			if e != nil {
				t.Fatal(e)
			}
			return h
		}

		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			raw := hir.LowerModule(parse())
			h := parse()
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			if len(a.Binary) > 0x6000 {
				t.Fatal("imagestackoverlap")
			}
			prefix := module + "/" + mode
			save(prefix+"/target.a80", s.Assembly)
			save(prefix+"/target.bin", a.Binary)
			save(prefix+"/raw.mir", raw.Dump())
			save(prefix+"/final.mir", s.MIR2Module.Dump())
			save(prefix+"/allocation.json", s.Allocation)
			vms := map[string]*mir2.VM{}
			vmBefore := map[string][]byte{}
			for label, m := range map[string]*mir2.Module{"raw": raw, "final": s.MIR2Module} {
				vm := mir2.NewVM(m)
				vm.EnsureHeap(65536)
				vms[label] = vm
				vmBefore[label] = append([]byte(nil), vm.ReadHeap(0, 65536)...)
			}
			z := emulator.NewRemogattoZ80()
			copy(z.Memory(), bytes.Repeat([]byte{0xa5}, 65536))
			z.LoadMemory(0x8000, a.Binary)
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			baseline := append([]byte(nil), mem...)
			spills := map[int]bool{}
			for _, l := range a.Listing {
				if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
					for j := range l.Bytes {
						spills[l.Address+j] = true
					}
				}
			}
			count := 0
			totalT := 0
			check := func(entry string, args []int64, want int64) {
				count++
				for label, vm := range vms {
					vs := make([]mir2.Value, len(args))
					for j, v := range args {
						vs[j] = mir2.Value{I: v}
					}
					v, e := vm.Call(entry, vs)
					if e != nil {
						t.Fatal(module, mode, label, entry, args, e)
					}
					decoded := int64(0)
					if len(v) == 1 {
						decoded = v[0].I

					}
					if entry != "main" && (len(v) != 1 || decoded != want) {
						t.Fatal(module, mode, label, entry, args, v, want)
					}
					expectedMemory := append([]byte(nil), vmBefore[label]...)
					if entry == "main" {
						if strings.HasSuffix(module, "cond_test.lizp") {
							addr, e := vm.ResolveSymbol("score")
							if e != nil {
								t.Fatal(e)
							}
							expectedMemory[addr] = 2
						}
						if strings.HasSuffix(module, "factorial.lizp") {
							addr, e := vm.ResolveSymbol("result")
							if e != nil {
								t.Fatal(e)
							}
							expectedMemory[addr], expectedMemory[addr+1] = 120, 0
						}
					}
					if !bytes.Equal(vm.ReadHeap(0, 65536), expectedMemory) {
						t.Fatal("VMRAM", entry, args)
					}
				}
				f := s.MIR2Module.FuncByName(entry)
				r := emulator.Registers{PC: uint16(a.Symbols[entry]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999}
				for j, arg := range args {
					v := uint16(arg)
					loc := assertParamLocation(f.Contract.Params[j], s.Allocation)
					switch loc.Name {
					case "A":
						r.A = byte(v)
					case "B":
						r.BC = r.BC&255 | v<<8
					case "C":
						r.BC = r.BC&0xff00 | v&255
					case "D":
						r.DE = r.DE&255 | v<<8
					case "E":
						r.DE = r.DE&0xff00 | v&255
					case "H":
						r.HL = r.HL&255 | v<<8
					case "L":
						r.HL = r.HL&0xff00 | v&255
					case "HL":
						r.HL = v
					case "DE":
						r.DE = v
					case "BC":
						r.BC = v
					default:
						t.Fatal("ABIhome", entry, loc)
					}
				}
				z.SetRegisters(r)
				start := z.GetCycles()
				minSP := uint16(0xeff0)
				done := false
				for n := 0; n < 1000000; n++ {
					if (entry == "main" && z.IsHalted() && z.GetSP() == 0xeff0) || (z.GetPC() == 0x7000 && z.GetSP() == 0xeff2) {
						done = true
						break
					}
					if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
						t.Fatal("stackbounds", entry, args)
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
				}
				if !done {
					t.Fatal("nativetermination", entry, args)
				}
				if entry == "main" {
					if z.GetRegisters().A != 0 {
						t.Fatal("mainstatus")
					}
				} else {
					if e = checkAssertZ80Result(z, hir.Assert{FuncName: entry, Args: args, Expected: want}, f, map[string]*hir.Func{entry: h.FuncByName(entry)}); e != nil {
						t.Fatal(module, mode, entry, args, want, e)
					}
				}
				if entry == "main" {
					if strings.HasSuffix(module, "cond_test.lizp") {
						baseline[a.Symbols["score"]] = 2
					}
					if strings.HasSuffix(module, "factorial.lizp") {
						addr := a.Symbols["result"]
						baseline[addr], baseline[addr+1] = 120, 0
					}
				}
				copy(baseline[minSP:0xeff0], mem[minSP:0xeff0])
				for p := range spills {
					baseline[p] = mem[p]
				}
				if !bytes.Equal(mem, baseline) {
					for p := range mem {
						if mem[p] != baseline[p] {
							t.Fatalf("%s %s %s %v RAM%x", module, mode, entry, args, p)
						}
					}
				}
				if len(z.GetOutput()) != 0 {
					t.Fatal("output")
				}
				totalT += z.GetCycles() - start
			}
			if strings.HasSuffix(module, "cond_test.lizp") {
				for x := int64(0); x < 256; x++ {
					w := int64(255)
					if x == 0 {
						w = 0
					} else if x < 10 {
						w = 1
					} else if x < 100 {
						w = 2
					}
					check("classify", []int64{x}, w)
				}
			}
			if strings.HasSuffix(module, "factorial.lizp") {
				f := int64(1)
				for x := int64(0); x < 256; x++ {
					if x > 0 {
						f = f * x & 65535
					}
					check("factorial", []int64{x}, f)
				}
			}
			if strings.HasSuffix(module, ".lanz") {
				for x := int64(0); x < 256; x++ {
					check("double", []int64{x}, 2*x&255)
					if x < 255 {
						check("sum_to", []int64{x}, x*(x+1)/2&255)
					}
				}
				for a := int64(0); a < 256; a++ {
					for b := int64(0); b < 256; b++ {
						w := a - b
						if w < 0 {
							w = -w
						}
						check("abs_diff", []int64{a, b}, w)
					}
				}
			}

			if strings.HasSuffix(module, ".lanz") {
				for label, m := range map[string]*mir2.Module{"raw": raw, "final": s.MIR2Module} {
					vm := mir2.NewVM(m)
					vm.MaxSteps = 10000
					if v, e := vm.Call("sum_to", []mir2.Value{{I: 255}}); e == nil || !strings.Contains(e.Error(), "gas limit") {
						t.Fatal("expected source nontermination", label, v, e)
					}
				}
				r := emulator.Registers{PC: uint16(a.Symbols["sum_to"]), SP: 0xeff0}
				f := s.MIR2Module.FuncByName("sum_to")
				loc := assertParamLocation(f.Contract.Params[0], s.Allocation)
				switch loc.Name {
				case "A":
					r.A = 255
				case "C":
					r.BC = 255
				case "B":
					r.BC = 0xff00
				default:
					t.Fatal("sum_to param", loc)
				}
				z.SetRegisters(r)
				for j := 0; j < 10000; j++ {
					if z.GetPC() == 0x7000 || z.IsHalted() {
						t.Fatal("sum_to255 unexpectedly terminates", mode)
					}
					z.Step()
				}
				if z.GetSP() != 0xeff0 {
					t.Fatal("nontermination stack drift")
				}
			}

			for _, a := range h.Asserts {
				check(a.FuncName, a.Args, a.Expected)
			}
			if s.MIR2Module.FuncByName("main") != nil {
				check("main", nil, 0)
			}
			save(prefix+"/evidence.json", map[string]any{"calls": count, "T_states": totalT, "source_asserts": len(h.Asserts), "source_sha256": freeze.Sources[module], "bin_sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary))})
			t.Log(module, mode, count, "native/raw/final independent fullstate PASS")
		}
	}
}

// Lizp value defun returns its final form (Book v7 §21.4).
func TestOriginalLizpCondLastFormNative(t *testing.T) {
	const path = "../../../examples/lizp/cond_test.lizp"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saveLizpTailArtifact(t, "original/source.lizp", source)
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			parse := func() *hir.Module {
				h, e := lizp.Compile(string(source), path)
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			raw := hir.LowerModule(parse())
			steps, e := CompileHIRSteps(parse(), Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(assembled.Errors) > 0 {
				t.Fatal(e, assembled.Errors)
			}
			saveLizpTailArtifact(t, "original/"+mode+"/program.a80", steps.Assembly)
			saveLizpTailArtifact(t, "original/"+mode+"/program.bin", assembled.Binary)
			saveLizpTailArtifact(t, "original/"+mode+"/raw.mir", raw.Dump())
			saveLizpTailArtifact(t, "original/"+mode+"/final.mir", steps.MIR2Module.Dump())
			for n := 0; n < 256; n++ {
				want := byte(255)
				if n == 0 {
					want = 0
				} else if n < 10 {
					want = 1
				} else if n < 100 {
					want = 2
				}
				for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
					values, e := mir2.NewVM(m).Call("classify", []mir2.Value{{I: int64(n)}})
					if e != nil || len(values) != 1 || values[0].I != int64(want) {
						t.Fatal(n, values, want, e)
					}
				}
				z := emulator.NewRemogattoZ80()
				z.LoadMemory(0x8000, assembled.Binary)
				z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
				z.SetRegisters(emulator.Registers{PC: uint16(assembled.Symbols["classify"]), SP: 0xeff0, A: byte(n)})
				for j := 0; j < 10000 && !(z.GetPC() == 0x7000 && z.GetSP() == 0xeff2); j++ {
					z.Step()
				}
				if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 || z.GetRegisters().A != want {
					t.Fatal(n, want, z.GetRegisters())
				}
				if !bytes.Equal(z.Memory()[0x8000:0x8000+len(assembled.Binary)], assembled.Binary) {
					t.Fatal("code changed")
				}
			}
			for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
				vm := mir2.NewVM(m)
				if _, e := vm.Call("main", nil); e != nil {
					t.Fatal(e)
				}
				if vm.ReadHeap(int64(vm.GlobalAddr("score")), 1)[0] != 2 {
					t.Fatal("main score")
				}
			}
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, assembled.Binary)
			z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
			z.SetRegisters(emulator.Registers{PC: uint16(assembled.Symbols["main"]), SP: 0xeff0})
			for j := 0; j < 10000 && !z.IsHalted(); j++ {
				z.Step()
			}
			if !z.IsHalted() || z.GetSP() != 0xeff0 || z.Memory()[uint16(assembled.Symbols["score"])] != 2 {
				t.Fatal("main", z.GetRegisters())
			}
			saveLizpTailArtifact(t, "original/"+mode+"/result.json", map[string]any{"native_classify_calls": 256, "native_main_calls": 1, "raw_vm_calls": 257, "final_vm_calls": 257, "checks": "classify independent thresholds; A, return PC/SP and code unchanged; main HALT, SP unchanged, score=2", "PASS": true})
		})
	}
}
