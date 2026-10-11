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

// Book v7 §4.4 lowers impl self to a typed pointer. This test retains all
// original output/main functions but grants execution only to pure methods.
func TestOriginalImplCPMPureMethods(t *testing.T) {
	const path = "examples/nanz/51_impl_cpm_demo.nanz"
	source, e := os.ReadFile("../../../" + path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/impl-cpm-methods/source.nanz")
	if e != nil || !bytes.Equal(source, frozen) {
		t.Fatal("sourcefreeze", e)
	}
	save := func(name string, data []byte) {
		if root := os.Getenv("MINZ_IMPL_CPM_METHODS_ARTIFACTS"); root != "" {
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
		if len(h.Funcs) != 6 {
			t.Fatal("wholemodulefunctions", len(h.Funcs))
		}
		return h
	}
	raw := hir.LowerModule(parse())
	save("raw.mir", []byte(raw.Dump()))
	for _, mode := range []struct {
		name string
		o    Options
	}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true}}} {
		t.Run(mode.name, func(t *testing.T) {
			s, e := CompileHIRSteps(parse(), mode.o)
			if e != nil {
				t.Fatal(e)
			}
			asm := s.Assembly
			for _, name := range []string{"Circle_area", "Rect_area"} {
				asm += fmt.Sprintf("\noracle_%s:\n CALL %s\n HALT\n", name, name)
			}
			image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
			if e != nil || image == nil || len(image.Errors) > 0 {
				t.Fatal("assemble", e, image)
			}
			module, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || len(module.Errors) > 0 {
				t.Fatal(e)
			}
			save(mode.name+"/program.a80", []byte(s.Assembly))
			save(mode.name+"/program.bin", module.Binary)
			save(mode.name+"/execution.a80", []byte(asm))
			save(mode.name+"/execution.bin", image.Binary)
			save(mode.name+"/final.mir", []byte(s.MIR2Module.Dump()))
			meta, _ := json.MarshalIndent(map[string]any{"options": mode.o, "functions": s.MIR2Module.Funcs, "allocation": s.Allocation, "symbols": image.Symbols}, "", "  ")
			save(mode.name+"/abi.json", meta)
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			for i := range mem {
				mem[i] = byte(i*73 + 29)
			}
			z.LoadMemory(0x8000, image.Binary)
			before := append([]byte(nil), mem...)
			rawVM, finalVM := mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)
			rawVM.EnsureHeap(65536)
			finalVM.EnsureHeap(65536)
			counts := map[string]int{}
			totals := map[string]int64{}
			minSP := uint16(0xf000)
			allowed := map[int]bool{}
			// The full image stays intact. Restrict PCs to the two actual pure method
			// ranges and their drivers; never install a dummy BDOS or execute main.
			reachable := map[uint16]bool{}
			for _, name := range []string{"Circle_area", "Rect_area"} {
				start := image.Symbols[name]
				end := 0x8000 + len(image.Binary)
				for _, other := range s.MIR2Module.Funcs {
					if a, ok := image.Symbols[other.Name]; ok && a > start && a < end {
						end = a
					}
				}
				for a := start; a < end; a++ {
					reachable[uint16(a)] = true
				}
				d := image.Symbols["oracle_"+name]
				for a := d; a < d+3; a++ {
					reachable[uint16(a)] = true
				}
			}
			// Native software multiply is a compiler-emitted dependency, not BDOS.
			if start, ok := image.Symbols["__mul8"]; ok {
				end := 0x8000 + len(image.Binary)
				for _, name := range []string{"oracle_Circle_area", "oracle_Rect_area"} {
					if a := image.Symbols[name]; a > start && a < end {
						end = a
					}
				}
				for a := start; a < end; a++ {
					reachable[uint16(a)] = true
				}
			}
			run := func(name string, addr uint16, obj []byte, want byte) {
				counts[name]++
				f := s.MIR2Module.FuncByName(name)
				for _, vm := range []*mir2.VM{rawVM, finalVM} {
					for i, v := range obj {
						vm.WriteHeap(int64(uint16(addr+uint16(i))), v)
					}
					vm.WriteHeap(int64(addr-1), 0xa7)
					vm.WriteHeap(int64(uint16(addr+uint16(len(obj)))), 0xb9)
					var events []mir2.VMDataAccess
					vm.ObserveDataAccess = func(e mir2.VMDataAccess) { events = append(events, e) }
					got, e := vm.Call(name, []mir2.Value{{I: int64(addr)}})
					if e != nil || len(got) != 1 || got[0].I != int64(want) {
						t.Fatal("VM", name, addr, obj, got, want, e)
					}
					// Source has two radius references; forwarding ordinary RAM may remove one.
					if len(events) < 1 || len(events) > 2 {
						t.Fatal("VMdatareads", name, events)
					}
					if name == "Rect_area" && len(events) != 2 {
						t.Fatal("Recttwofields", events)
					}
					for i, event := range events {
						off := 0
						if name == "Rect_area" {
							off = i
						}
						if event.Op != mir2.OpLoad || uint16(event.Addr) != addr+uint16(off) || event.Value != obj[off] {
							t.Fatal("VMfieldreadorder", name, events)
						}
					}
					for i, v := range obj {
						if vm.ReadHeap(int64(uint16(addr+uint16(i))), 1)[0] != v {
							t.Fatal("VMobjectchanged")
						}
					}
					if vm.ReadHeap(int64(addr-1), 1)[0] != 0xa7 || vm.ReadHeap(int64(uint16(addr+uint16(len(obj)))), 1)[0] != 0xb9 {
						t.Fatal("VMguard")
					}
				}
				for i, v := range obj {
					a := uint16(addr + uint16(i))
					mem[a] = v
					allowed[int(a)] = true
				}
				guardLow, guardHigh := mem[addr-1], mem[uint16(addr+uint16(len(obj)))]
				driver := image.Symbols["oracle_"+name]
				r := emulator.Registers{PC: uint16(driver), SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468}
				loc := assertParamLocation(f.Contract.Params[0], s.Allocation)
				switch loc.Name {
				case "HL":
					r.HL = addr
				case "DE":
					r.DE = addr
				case "BC":
					r.BC = addr
				case "IX":
					r.IX = addr
				case "IY":
					r.IY = addr
				default:
					t.Fatal("pointerABI", loc)
				}
				z.SetRegisters(r)
				var events []mir2.VMDataAccess
				done := false
				callMin := uint16(0xf000)
				start := z.GetCycles()
				for step := 0; step < 10000; step++ {
					if z.GetPC() == uint16(driver+3) && z.GetSP() == 0xf000 {
						done = true
						break
					}
					pc := z.GetPC()
					if !reachable[pc] {
						t.Fatal("puremethodenteredruntime", name, pc)
					}
					events = append(events, widemathNativeByteAccess(mem, pc, z.GetRegisters())...)
					op, rr := mem[pc], z.GetRegisters()
					z.Step()
					word := rr.BC
					push := true
					switch op {
					case 0xc5:
					case 0xd5:
						word = rr.DE
					case 0xe5:
						word = rr.HL
					case 0xf5:
						word = uint16(rr.A)<<8 | uint16(rr.F)
					case 0xcd:
						word = pc + 3
					default:
						push = false
					}
					if push && (z.GetSP() != rr.SP-2 || mem[rr.SP-2] != byte(word) || mem[rr.SP-1] != byte(word>>8)) {
						t.Fatal("nativePUSHword", name)
					}
					if z.GetSP() < callMin {
						callMin = z.GetSP()
					}
				}
				if !done {
					t.Fatal("nativePC/SP", name, addr, obj, z.GetRegisters())
				}
				if len(events) < 1 || len(events) > 2 || name == "Rect_area" && len(events) != 2 {
					t.Fatal("nativefieldcount", name, events)
				}
				for i, event := range events {
					off := 0
					if name == "Rect_area" {
						off = i
					}
					if event.Op != mir2.OpLoad || uint16(event.Addr) != addr+uint16(off) || event.Value != obj[off] {
						t.Fatal("nativefieldorder", name, addr, obj, events)
					}
				}
				if len(f.Contract.Returns) != 1 || mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty) != "A" || z.GetRegisters().A != want {
					t.Fatal("nativevalue", name, addr, obj, z.GetRegisters().A, want)
				}
				for i, v := range obj {
					if mem[addr+uint16(i)] != v {
						t.Fatal("nativewholeobject")
					}
				}
				if mem[addr-1] != guardLow || mem[addr+uint16(len(obj))] != guardHigh {
					t.Fatal("nativeguard")
				}
				if callMin < minSP {
					minSP = callMin
				}
				totals[name] += int64(z.GetCycles() - start)
			}
			for _, addr := range []uint16{0x6000, 0xffff} {
				for x := 0; x < 256; x++ {
					run("Circle_area", addr, []byte{byte(x)}, byte(3*x*x))
					for y := 0; y < 256; y++ {
						run("Rect_area", addr, []byte{byte(x), byte(y)}, byte(x*y))
					}
				}
			}
			for a, v := range before {
				if allowed[a] || a >= int(minSP) && a < 0xf000 {
					continue
				}
				if mem[a] != v {
					t.Fatalf("nativeRAM/codechanged%04x", a)
				}
			}
			result, _ := json.MarshalIndent(map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "calls": counts, "cycles_including_CALL": totals, "module_bytes": len(module.Binary), "stack_depth": int(0xf000 - minSP), "whole_module_functions_retained": len(s.MIR2Module.Funcs), "whole_main_status": "not executed; BDOS/runtime still required", "execution_sha256": fmt.Sprintf("%x", sha256.Sum256(image.Binary))}, "", "  ")
			save(mode.name+"/results.json", result)
			t.Log(string(result))
		})
	}
}
