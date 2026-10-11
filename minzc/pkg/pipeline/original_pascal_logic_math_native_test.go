package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// Pascal Byte=TyU8, Integer=TyI16; Book v7 §3.1 widths/modular integer
// representation. Original module-level asserts are executed explicitly.
func TestOriginalPascalLogicMathDomains(t *testing.T) {
	data, e := os.ReadFile("../../../reports/pascal-logic-math-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		Sources map[string]string `json:"sources"`
		Third   []int64           `json:"add3_third_values"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil || len(freeze.Third) != 16 {
		t.Fatal("freeze", e)
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_PASCAL_LOGIC_MATH_ARTIFACTS")
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
	for _, module := range []string{"logic_test", "math_test"} {
		path := "../../../examples/pascal/" + module + ".pas"
		source, e := os.ReadFile(path)
		if e != nil || freeze.Sources[module] != fmt.Sprintf("%x", sha256.Sum256(source)) {
			t.Fatal("sourcefreeze", module, e)
		}
		parse := func() *hir.Module {
			h, e := pascal.Compile(string(source), path)
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
						if entry == "DOUBLE" {
							decoded = int64(int16(uint16(decoded)))
						}
					}
					if entry != "main" && (len(v) != 1 || decoded != want) {
						t.Fatal(module, mode, label, entry, args, v, want)
					}
					if !bytes.Equal(vm.ReadHeap(0, 65536), vmBefore[label]) {
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
				for n := 0; n < 10000; n++ {
					if (entry == "main" && z.IsHalted() && z.GetSP() == 0xeff0) || (entry != "main" && z.GetPC() == 0x7000 && z.GetSP() == 0xeff2) {
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
					if entry == "DOUBLE" {
						if f.Contract.Returns[0].Ty != mir2.TyI16 {
							t.Fatal("declared Double return type")
						}
						r := z.GetRegisters()
						var word uint16
						switch mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty) {
						case "HL":
							word = r.HL
						case "DE":
							word = r.DE
						case "BC":
							word = r.BC
						default:
							t.Fatal("Integer return home")
						}
						if got := int64(int16(word)); got != want {
							t.Fatal(module, mode, entry, args, got, want)
						}
					} else if e = checkAssertZ80Result(z, hir.Assert{FuncName: entry, Args: args, Expected: want}, f, map[string]*hir.Func{entry: h.FuncByName(entry)}); e != nil {
						t.Fatal(module, mode, entry, args, want, e)
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
			if module == "logic_test" {
				for x := int64(0); x < 256; x++ {
					check("MULBY3", []int64{x}, 3*x&255)
					check("MULBY5", []int64{x}, 5*x&255)
					check("HALVE", []int64{x}, x/2)
				}
				for a := int64(0); a < 256; a++ {
					for b := int64(0); b < 256; b++ {
						check("DIFF", []int64{a, b}, (a-b)&255)
						for _, c := range freeze.Third {
							check("ADD3", []int64{a, b, c}, (a+b+c)&255)
						}
					}
				}
			} else {
				for x := int64(0); x < 256; x++ {
					check("ADDONE", []int64{x}, (x+1)&255)
					check("SQUARE", []int64{x}, x*x&255)
					check("TRIPLE", []int64{x}, 3*x&255)
					even := int64(0)
					if x%2 == 0 {
						even = 1
					}
					check("ISEVEN", []int64{x}, even)
				}
				for x := 0; x < 65536; x++ {
					v := int64(int16(uint16(x)))
					check("DOUBLE", []int64{v}, int64(int16(uint16(x*2))))
				}
				for a := int64(0); a < 256; a++ {
					for b := int64(0); b < 256; b++ {
						lo, hi := a, b
						if lo > hi {
							lo, hi = hi, lo
						}
						check("MAX", []int64{a, b}, hi)
						check("MIN", []int64{a, b}, lo)
						check("ABSDIFF", []int64{a, b}, hi-lo)
					}
				}
			}
			for _, a := range h.Asserts {
				check(a.FuncName, a.Args, a.Expected)
			}
			check("main", nil, 0)
			save(prefix+"/evidence.json", map[string]any{"calls": count, "T_states": totalT, "source_asserts": len(h.Asserts), "source_sha256": freeze.Sources[module], "bin_sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary))})
			t.Log(module, mode, count, "native/raw/final independent fullstate PASS")
		}
	}
}
