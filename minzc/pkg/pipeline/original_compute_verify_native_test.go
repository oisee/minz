package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Frozen before execution: reports/original-compute-verify-native/freeze.json; Book v7 §3.1 byte arithmetic.
func TestOriginalComputeVerifyNative(t *testing.T) {
	const path = "examples/tests/z80_compute_verify.nanz"
	source, e := os.ReadFile("../../../" + path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/original-compute-verify-native/source.nanz")
	if e != nil || !bytes.Equal(source, frozen) {
		t.Fatal("source freeze", e)
	}
	save := func(name string, data []byte) {
		if root := os.Getenv("MINZ_COMPUTE_VERIFY_ARTIFACTS"); root != "" {
			p := filepath.Join(root, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, data, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	raw := hir.LowerModule(parse())
	save("source.nanz", source)
	save("raw.mir", []byte(raw.Dump()))
	entries := []string{"add8", "sub8", "mul2", "mul3", "shl1", "shr1", "band", "bor", "max8", "min8", "clamp", "abs_diff", "add16", "sub16", "sum_to", "count_bits", "is_power_of_2", "double_it", "quad", "compose", "widen"}
	if len(raw.Funcs) != len(entries) {
		t.Fatal("original callable count", len(raw.Funcs))
	}
	for _, mode := range []struct {
		name string
		o    Options
	}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true}}} {
		t.Run(mode.name, func(t *testing.T) {
			s, e := CompileHIRSteps(parse(), mode.o)
			if e != nil {
				t.Fatal(e)
			}
			assembly := s.Assembly
			for _, name := range entries {
				assembly += fmt.Sprintf("\noracle_%s:\n CALL %s\n HALT\n", name, name)
			}
			image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
			if e != nil || image == nil || len(image.Errors) != 0 {
				t.Fatal("assemble", e, image)
			}
			save(mode.name+"/program.a80", []byte(s.Assembly))
			save(mode.name+"/source-assert-count.txt", []byte(fmt.Sprintf("%d original assertions; default CompileHIRSteps executes active via-z80 assertions\n", len(parse().Asserts))))
			save(mode.name+"/execution.a80", []byte(assembly))
			save(mode.name+"/execution.bin", image.Binary)
			save(mode.name+"/final.mir", []byte(s.MIR2Module.Dump()))
			originalImage, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || len(originalImage.Errors) != 0 {
				t.Fatal(e)
			}
			save(mode.name+"/program.bin", originalImage.Binary)
			meta, _ := json.MarshalIndent(map[string]any{"options": mode.o, "functions": s.MIR2Module.Funcs, "allocation": s.Allocation, "symbols": image.Symbols}, "", "  ")
			save(mode.name+"/abi.json", meta)
			z := emulator.NewRemogattoZ80()
			profiler := emulator.NewProfiler()
			z.SetProfiler(profiler)
			mem := z.Memory()
			for i := range mem {
				mem[i] = byte(i*73 + 29)
			}
			if e := z.LoadMemory(0x8000, image.Binary); e != nil {
				t.Fatal(e)
			}
			before := append([]byte(nil), mem...)
			allowed := map[int]bool{}
			for _, f := range s.MIR2Module.Funcs {
				for _, b := range f.Blocks {
					for _, inst := range b.Insts {
						if inst.Dst != mir2.NoReg {
							if addr, ok := image.Symbols[mir2.Z80SpillLabel(f.Name, inst.Dst)]; ok {
								for j := 0; j < mir2.NativeSpillWidth(inst.Ty); j++ {
									allowed[addr+j] = true
								}
							}
						}
					}
				}
			}
			rawVM, finalVM := mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)
			effects := 0
			rawVM.ObserveDataAccess = func(mir2.VMDataAccess) { effects++ }
			finalVM.ObserveDataAccess = func(mir2.VMDataAccess) { effects++ }
			counts := map[string]int{}
			totals := map[string]int64{}
			depths := map[string]int{}
			minSP := uint16(0xf000)
			read := func(name string) uint16 {
				r := z.GetRegisters()
				switch name {
				case "A":
					return uint16(r.A)
				case "B":
					return r.BC >> 8
				case "C":
					return r.BC & 255
				case "D":
					return r.DE >> 8
				case "E":
					return r.DE & 255
				case "H":
					return r.HL >> 8
				case "L":
					return r.HL & 255
				case "HL":
					return r.HL
				case "DE":
					return r.DE
				case "BC":
					return r.BC
				default:
					t.Fatal("return location", name)
					return 0
				}
			}
			run := func(name string, inputs []int64, expected int64) {
				x, y := inputs, expected
				want := []int64{expected}
				counts[name]++
				f := s.MIR2Module.FuncByName(name)
				args := make([]mir2.Value, len(inputs))
				for i, v := range inputs {
					args[i] = mir2.Value{I: v}
				}
				for _, vm := range []*mir2.VM{rawVM, finalVM} {
					got, e := vm.Call(name, args)
					if e != nil || len(got) != len(want) {
						t.Fatal("VM", name, x, y, got, want, e)
					}
					for i, w := range want {
						if got[i].I != w {
							t.Fatal("VM result", name, x, y, i, got, want)
						}
					}
				}
				if effects != 0 {
					t.Fatal("unexpected VM data effects", effects)
				}
				driver, ok := image.Symbols["oracle_"+name]
				if !ok {
					t.Fatal("driver", name)
				}
				z.SetRegisters(emulator.Registers{PC: uint16(driver), SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
				for i, p := range f.Contract.Params {
					loc := assertParamLocation(p, s.Allocation)
					v := uint16(inputs[i])
					if loc.Kind == mir2.LocMem {
						addr, ok := image.Symbols[mir2.Z80SpillLabel(name, p.Reg)]
						if !ok {
							t.Fatal("param spill", p)
						}
						allowed[addr] = true
						mem[addr] = byte(v)
						if p.Ty.Width() > 8 {
							mem[addr+1] = byte(v >> 8)
							allowed[addr+1] = true
						}
					} else if p.Ty.Width() <= 8 {
						if e := z.SetRegister8(loc.Name, byte(v)); e != nil {
							t.Fatal(e)
						}
					} else {
						regs := z.GetRegisters()
						switch loc.Name {
						case "HL":
							regs.HL = v
						case "DE":
							regs.DE = v
						case "BC":
							regs.BC = v
						case "IX":
							regs.IX = v
						case "IY":
							regs.IY = v
						default:
							t.Fatal("word param location", loc)
						}
						z.SetRegisters(regs)
					}
				}
				callMin := uint16(0xf000)
				start := z.GetCycles()
				done := false
				for step := 0; step < 100000; step++ {
					if z.GetPC() == uint16(driver+3) && z.GetSP() == 0xf000 {
						done = true
						break
					}
					pc, sp := z.GetPC(), z.GetSP()
					op := mem[pc]
					z.Step()
					if op == 0xcd {
						if z.GetSP() != sp-2 || mem[sp-2] != byte(pc+3) || mem[sp-1] != byte((pc+3)>>8) {
							t.Fatal("native CALL stack write", name, x, y, pc, sp)
						}
					}
					if z.GetSP() < callMin {
						callMin = z.GetSP()
					}
				}
				if !done {
					t.Fatal("native return PC/SP", name, x, y, z.GetRegisters())
				}
				if callMin < minSP {
					minSP = callMin
				}
				if d := int(0xf000 - callMin); d > depths[name] {
					depths[name] = d
				}
				if len(f.Contract.Returns) != len(want) {
					t.Fatal("tuple ABI arity", name)
				}
				for i, w := range want {
					r := f.Contract.Returns[i]
					mask := uint16(255)
					if r.Ty.Width() > 8 {
						mask = 65535
					}
					got := int64(read(mir2.ReturnLocation(r.Class, r.Ty)) & mask)
					if got != w {
						t.Fatal("native result", name, x, y, i, got, w, z.GetRegisters())
					}
				}
				totals[name] += int64(z.GetCycles() - start)
			}

			for a := int64(0); a < 256; a++ {
				for b := int64(0); b < 256; b++ {
					v := []int64{a, b}
					mx, mn, d := a, a, a-b
					if b > a {
						mx = b
						d = b - a
					}
					if b < a {
						mn = b
					}
					for name, w := range map[string]int64{"add8": (a + b) & 255, "sub8": (a - b) & 255, "band": a & b, "bor": a | b, "max8": mx, "min8": mn, "abs_diff": d, "add16": a + b, "sub16": (a - b) & 65535} {
						run(name, v, w)
					}
				}
			}
			bounds := []int64{0, 1, 2, 63, 127, 128, 192, 254, 255}
			for x := int64(0); x < 256; x++ {
				bits := int64(0)
				for v := x; v > 0; v >>= 1 {
					bits += v & 1
				}
				power := int64(0)
				if x != 0 && x&(x-1) == 0 {
					power = 1
				}
				for name, w := range map[string]int64{"mul2": (2 * x) & 255, "mul3": (3 * x) & 255, "shl1": (x << 1) & 255, "shr1": x >> 1, "count_bits": bits, "is_power_of_2": power, "double_it": (2 * x) & 255, "quad": (4 * x) & 255, "compose": (5 * x) & 255, "widen": x} {
					run(name, []int64{x}, w)
				}
				for _, lo := range bounds {
					for _, hi := range bounds {
						w := x
						if x < lo {
							w = lo
						} else if x > hi {
							w = hi
						}
						run("clamp", []int64{x, lo, hi}, w)
					}
				}
				if x < 255 {
					run("sum_to", []int64{x}, x*(x+1)/2&255)
				}
			}
			words := []int64{0, 1, 2, 127, 128, 255, 256, 257, 32767, 32768, 65534, 65535}
			for _, a := range words {
				for _, b := range words {
					run("add16", []int64{a, b}, (a+b)&65535)
					run("sub16", []int64{a, b}, (a-b)&65535)
				}
			}
			// Source i:u8 wraps after255, so i<=255 never becomes false.
			for _, vm := range []*mir2.VM{rawVM, finalVM} {
				vm.MaxSteps = 10000
				if _, err := vm.Call("sum_to", []mir2.Value{{I: 255}}); err == nil || !strings.Contains(err.Error(), "gas limit") {
					t.Fatal("sum255 expected bounded nonreturn", err)
				}
			}
			f := s.MIR2Module.FuncByName("sum_to")
			driver := image.Symbols["oracle_sum_to"]
			z.SetRegisters(emulator.Registers{PC: uint16(driver), SP: 0xf000})
			loc := assertParamLocation(f.Contract.Params[0], s.Allocation)
			if loc.Kind == mir2.LocMem {
				t.Fatal("nonreturn probe memory param needs explicit driver")
			}
			if e := z.SetRegister8(loc.Name, 255); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 10000; i++ {
				z.Step()
				if z.GetPC() == uint16(driver+3) && z.GetSP() == 0xf000 {
					t.Fatal("sum255 unexpectedly returned")
				}
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}

			for addr, v := range before {
				if allowed[addr] || addr >= int(minSP) && addr < 0xf000 {
					continue
				}
				if profiler.WriteCount[addr] != 0 || mem[addr] != v {
					t.Fatalf("code/non-stack RAM modified %04x", addr)
				}
			}
			result, _ := json.MarshalIndent(map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "calls": counts, "t_states_including_CALL": totals, "stack_depths": depths, "module_bytes": len(originalImage.Binary), "execution_sha256": fmt.Sprintf("%x", sha256.Sum256(image.Binary)), "allowed_spill_bytes": allowed, "raw_and_final_vm_data_effects": effects}, "", "  ")
			save(mode.name+"/results.json", result)
			t.Log(string(result))
		})
	}
}
