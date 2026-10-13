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
	"strings"
	"testing"
)

// Book v7 §4.4: implicitself becomes ^Struct and a direct Type_method call.
// Host modular area/perimeter models never execute compiler IR.
func TestOriginalImplShowcaseVMZ80(t *testing.T) {
	const path = "../../../examples/nanz/50_impl_showcase.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	type invocation struct {
		Entry  string
		Args   []int64
		RAM    []byte
		Want   byte
		Events []byte
	}
	calls := []invocation{
		{Entry: "test_circle_area", Want: 75, Events: []byte{5, 5, 5}},
		{Entry: "test_circle_perim", Want: 24, Events: []byte{4, 4}},
		{Entry: "test_rect_area", Want: 50, Events: []byte{10, 5, 10, 5}},
		{Entry: "test_rect_perim", Want: 22, Events: []byte{8, 3, 8, 3}},
	}
	for radius := 0; radius < 256; radius++ {
		for _, entry := range []string{"Circle_area", "Circle_perimeter"} {
			want := byte(3 * radius * radius)
			events := []byte{byte(radius), byte(radius)}
			if entry == "Circle_perimeter" {
				want = byte(6 * radius)
				events = []byte{byte(radius)}
			}
			calls = append(calls, invocation{entry, []int64{1}, []byte{0x71, 0x15, 0x36, byte(radius), 0xb9}, want, events})
		}
	}
	for width := 0; width < 256; width++ {
		for _, height := range []int{0, 1, 2, 7, 127, 128, 254, 255} {
			for _, entry := range []string{"Rect_area", "Rect_perimeter"} {
				want := byte(width * height)
				if entry == "Rect_perimeter" {
					want = byte(2*width + 2*height)
				}
				calls = append(calls, invocation{entry, []int64{1}, []byte{0x71, 0x15, 0x36, byte(width), byte(height), 0xb9}, want, []byte{byte(width), byte(height)}})
			}
		}
	}
	save := func(name string, value any) {
		dir := os.Getenv("MINZ_IMPL_ORACLE_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		switch v := value.(type) {
		case []byte:
			b = v
		case string:
			b = []byte(v)
		default:
			b, e = json.MarshalIndent(value, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	save("source.nanz", src)
	save("workload.json", calls)
	save("provenance.json", map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256(src)), "source_edits": 0, "adapter": "ABI invocation only, no source driver"})
	parse := func() *hir.Module {
		m, e := nanz.Parse(string(src), path)
		if e != nil {
			t.Fatal(e)
		}
		return m
	}
	raw := hir.LowerModule(parse())
	for _, mode := range []struct {
		name    string
		options Options
	}{{"ordinary", Options{}}, {"grace-speed", Options{UseGrace: true, UseHIRGrace: true}}, {"grace-size", Options{UseGrace: true, UseHIRGrace: true, OptSize: true}}} {
		steps, e := CompileHIRSteps(parse(), mode.options)
		if e != nil {
			t.Fatal(e)
		}
		program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil || len(program.Errors) > 0 {
			t.Fatal(e, program.Errors)
		}
		save(mode.name+"/target.a80", steps.Assembly)
		save(mode.name+"/target.bin", program.Binary)
		save(mode.name+"/final.mir2", steps.MIR2Module.Dump())
		save(mode.name+"/allocation.json", steps.Allocation)
		spills := map[int]bool{}
		for _, line := range program.Listing {
			if strings.HasPrefix(line.Label, "_spill_") {
				for j := range line.Bytes {
					spills[line.Address+j] = true
				}
			}
		}
		z := emulator.NewRemogattoZ80()
		if e = z.LoadMemory(0x8000, program.Binary); e != nil {
			t.Fatal(e)
		}
		mem := z.Memory()
		var total int64
		for i, c := range calls {
			for stage, module := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
				vm := mir2.NewVM(module)
				vm.MaxSteps = 10000
				args := []mir2.Value{}
				var base int64
				if c.RAM != nil {
					base = vm.AllocHeap(c.RAM).I
					args = append(args, mir2.Value{I: base + 1})
				}
				var events []mir2.VMDataAccess
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
				r, e := vm.Call(c.Entry, args)
				if e != nil || len(r) != 1 || r[0].I != int64(c.Want) {
					save(mode.name+"/failure.json", map[string]any{"stage": stage, "call": i, "input": c, "result": r, "error": fmt.Sprint(e)})
					t.Fatal(mode.name, stage, i, c.Entry, r, c.Want, e)
				}
				if c.RAM != nil && !bytes.Equal(vm.ReadHeap(base, len(c.RAM)), c.RAM) {
					t.Fatal("VM receiver changed", i)
				}
				if len(events) != len(c.Events) {
					t.Fatal(stage, c.Entry, "eventcount", len(events), len(c.Events), events)
				}
				for j, event := range events {
					wantOp := mir2.OpLoad
					stores := 0
					if c.RAM == nil {
						stores = 1
						if strings.Contains(c.Entry, "rect") {
							stores = 2
						}
					}
					if j < stores {
						wantOp = mir2.OpStore
					}
					if event.Op != wantOp {
						t.Fatal("independent field effect", stage, c.Entry, j, event.Op, wantOp)
					}
					if c.RAM != nil {
						offset := int64(3)
						if strings.HasPrefix(c.Entry, "Rect_") {
							offset += int64(j)
						}
						if event.Addr != base+offset {
							t.Fatal("typed field byte offset", stage, c.Entry, event, base+offset)
						}
					}
					if event.Value != c.Events[j] {
						t.Fatal(stage, c.Entry, "eventvalue", j, event, c.Events)
					}
				}
			}
			if c.RAM != nil {
				copy(mem[0x9000:], c.RAM)
			}
			mem[0xe000], mem[0xeff2] = 0x9b, 0xad
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			regs := emulator.Registers{PC: uint16(program.Symbols[c.Entry]), SP: 0xeff0, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999, A: 0x42}
			f := steps.MIR2Module.FuncByName(c.Entry)
			if c.RAM != nil {
				loc := assertParamLocation(f.Contract.Params[0], steps.Allocation)
				switch loc.Name {
				case "HL":
					regs.HL = 0x9001
				case "DE":
					regs.DE = 0x9001
				case "BC":
					regs.BC = 0x9001
				case "IX":
					regs.IX = 0x9001
				case "IY":
					regs.IY = 0x9001
				default:
					t.Fatal("typed receiver ABI", loc)
				}
			}
			prof := emulator.NewProfiler()
			z.SetProfiler(prof)
			before := append([]byte(nil), mem...)
			z.SetRegisters(regs)
			start := z.GetCycles()
			minSP := uint16(0xeff0)
			done := false
			for j := 0; j < 10000; j++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					done = true
					break
				}
				if z.GetSP() <= 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("stack bounds", i)
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if !done {
				t.Fatal("native return", i)
			}
			total += int64(z.GetCycles() - start)
			if e = checkAssertZ80Result(z, hir.Assert{FuncName: c.Entry, Expected: int64(c.Want)}, f, map[string]*hir.Func{c.Entry: parse().FuncByName(c.Entry)}); e != nil {
				t.Fatal(mode.name, i, c.Entry, e)
			}
			for addr := range mem {
				if mem[addr] != before[addr] && !(addr >= int(minSP) && addr < 0xeff0) && !spills[addr] {
					t.Fatal("native unexpected RAM", i, addr)
				}
			}
			if c.RAM != nil {
				for offset := range c.RAM {
					wantReads := uint32(0)
					if strings.HasPrefix(c.Entry, "Circle_") && offset == 3 {
						wantReads = 1
						if c.Entry == "Circle_area" {
							wantReads = 2
						}
					}
					if strings.HasPrefix(c.Entry, "Rect_") && (offset == 3 || offset == 4) {
						wantReads = 1
					}
					if prof.ReadCount[0x9000+offset] != wantReads || prof.WriteCount[0x9000+offset] != 0 {
						t.Fatal("native field effects", c.Entry, offset, prof.ReadCount[0x9000+offset], wantReads, prof.WriteCount[0x9000+offset])
					}
				}
			}
			if c.RAM != nil && !bytes.Equal(mem[0x9000:0x9000+len(c.RAM)], c.RAM) {
				t.Fatal("native receiver changed")
			}
		}
		save(mode.name+"/summary.json", map[string]any{"calls": len(calls), "t_states": total, "bin_bytes": len(program.Binary), "sha256": fmt.Sprintf("%x", sha256.Sum256(program.Binary))})
		t.Logf("%s %dcalls/fulloriginal8functions/4asserts %dT %dB SHA%x", mode.name, len(calls), total, len(program.Binary), sha256.Sum256(program.Binary))
	}
}
