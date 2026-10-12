package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Frozen before execution: Book v7 §2.2/§7.5 tuple order and §3.1 u8 arithmetic.
func TestOriginalOutparamSwapNative(t *testing.T) {
	const path = "examples/paper/outparam_swap.nanz"
	source, e := os.ReadFile("../../../" + path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/outparam-swap-native/source.nanz")
	if e != nil || !bytes.Equal(source, frozen) {
		t.Fatal("source freeze", e)
	}
	save := func(name string, data []byte) {
		if root := os.Getenv("MINZ_OUTPARAM_SWAP_ARTIFACTS"); root != "" {
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
	entries := []string{"swap", "use_swap", "main"}
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
			run := func(name string, x, y int, want []int64) {
				counts[name]++
				f := s.MIR2Module.FuncByName(name)
				args := []mir2.Value{{I: int64(x)}, {I: int64(y)}}
				if name == "main" {
					args = nil
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
					v := byte([]int{x, y}[i])
					if loc.Kind == mir2.LocMem {
						addr, ok := image.Symbols[mir2.Z80SpillLabel(name, p.Reg)]
						if !ok {
							t.Fatal("param spill", p)
						}
						allowed[addr] = true
						mem[addr] = v
					} else if e := z.SetRegister8(loc.Name, v); e != nil {
						t.Fatal(e)
					}
				}
				callMin := uint16(0xf000)
				start := z.GetCycles()
				done := false
				for step := 0; step < 1000; step++ {
					if z.GetPC() == uint16(driver+3) && z.GetSP() == 0xf000 {
						done = true
						break
					}
					// The complete generated slice has no memory instruction except CALL.
					// Audit executed writes immediately, including their exact return word;
					// the final full-RAM comparison then covers every other address.
					pc, sp := z.GetPC(), z.GetSP()
					op := mem[pc]
					switch op {
					case 0xcd, 0xc9, 0x3e, 0x81: // CALL nn, RET, LD A,n, ADD A,C
					default:
						t.Fatalf("unreviewed native memory/effect opcode at %04x: %02x", pc, op)
					}
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
					got := int64(read(mir2.ReturnLocation(r.Class, r.Ty)) & 255)
					if got != w {
						t.Fatal("native result", name, x, y, i, got, w, z.GetRegisters())
					}
				}
				totals[name] += int64(z.GetCycles() - start)
			}
			for x := 0; x < 256; x++ {
				for y := 0; y < 256; y++ {
					run("swap", x, y, []int64{int64(y), int64(x)})
					run("use_swap", x, y, []int64{int64((x + y) & 255)})
				}
			}
			run("main", 0, 0, []int64{10})
			for addr, v := range before {
				if allowed[addr] || addr >= int(minSP) && addr < 0xf000 {
					continue
				}
				if mem[addr] != v {
					t.Fatalf("code/non-stack RAM modified %04x", addr)
				}
			}
			result, _ := json.MarshalIndent(map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "calls": counts, "t_states_including_CALL": totals, "stack_depths": depths, "module_bytes": len(originalImage.Binary), "execution_sha256": fmt.Sprintf("%x", sha256.Sum256(image.Binary)), "allowed_spill_bytes": allowed, "raw_and_final_vm_data_effects": effects}, "", "  ")
			save(mode.name+"/results.json", result)
			t.Log(string(result))
		})
	}
}
