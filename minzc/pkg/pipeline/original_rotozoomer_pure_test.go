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

// Book v7 §3.2 signed stored-bit semantics. Models use signed16 modular
// intermediates; axis tables are frozen published application data, not codegen.
func TestOriginalRotozoomerPureFunctions(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/rotozoomer.nanz")
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile("../../../reports/rotozoomer-pure/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SHA      string `json:"source_sha256"`
		A        []int  `json:"axis_a"`
		B        []int  `json:"axis_b"`
		Boundary []int  `json:"texture_boundary"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil || fmt.Sprintf("%x", sha256.Sum256(src)) != freeze.SHA {
		t.Fatal("sourcefreeze", e)
	}
	save := func(n string, v any) {
		if dir := os.Getenv("MINZ_ROTOZOOMER_PURE_ARTIFACTS"); dir != "" {
			var b []byte
			switch x := v.(type) {
			case string:
				b = []byte(x)
			case []byte:
				b = x
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			p := filepath.Join(dir, n)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), "../../../examples/nanz/rotozoomer.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	abs := func(x int16) int16 {
		if x < 0 {
			return -x
		}
		return x
	}
	texture := func(x, y int16) uint16 {
		d := int16(abs(x-8) + abs(y-8))
		if d < 3 {
			return 5
		}
		if d < 6 {
			return 4
		}
		if (x/4+y/4)%2 == 0 {
			return 3
		}
		if (x^y)&2 == 0 {
			return 2
		}
		return 1
	}
	type call struct {
		Function string
		Args     []int64
		Want     uint16
	}
	var calls []call
	for x := 0; x < 65536; x++ {
		calls = append(calls, call{"abs16", []int64{int64(int16(x))}, uint16(abs(int16(x)))})
	}
	for x := 0; x < 256; x++ {
		i := x
		if i > 30 {
			i = 31
		}
		calls = append(calls, call{"axis_a", []int64{int64(x)}, uint16(int16(freeze.A[i]))}, call{"axis_b", []int64{int64(x)}, uint16(int16(freeze.B[i]))})
	}
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			calls = append(calls, call{"texture", []int64{int64(x), int64(y)}, texture(int16(x), int16(y))})
		}
	}
	for _, x := range freeze.Boundary {
		for _, y := range freeze.Boundary {
			calls = append(calls, call{"texture", []int64{int64(x), int64(y)}, texture(int16(x), int16(y))})
		}
	}
	save("workload.json", calls)
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		raw := hir.LowerModule(parse())
		s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", UseHIRGrace: mode != "ordinary", UseGrace: mode != "ordinary", OptSize: mode == "grace-size", ContractOpt: true})
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", s.MIR2Module.Dump())
		save(mode+"/target.a80", s.Assembly)
		save(mode+"/target.bin", a.Binary)
		// No reachable extern is bound to a fake implementation. Raw/final calls
		// would reject a missing host, native entry fetch into any extern is refused.
		forbidden := map[uint16]bool{}
		for _, f := range s.MIR2Module.Funcs {
			if f.Attrs.IsExtern {
				forbidden[uint16(a.Symbols[f.Name])] = true
			}
		}
		vmRaw, vmFinal := mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)
		for _, vm := range []*mir2.VM{vmRaw, vmFinal} {
			for i := 0; i < s.MIR2Module.Strings.Len(); i++ {
				if _, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", i)); e != nil {
					t.Fatal(e)
				}
			}
			vm.EnsureHeap(65536)
		}
		beforeRaw := append([]byte(nil), vmRaw.ReadHeap(0, 65536)...)
		beforeFinal := append([]byte(nil), vmFinal.ReadHeap(0, 65536)...)
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		mem := z.Memory()
		base := bytes.Repeat([]byte{0xa5}, 65536)
		copy(base[0x4000:], a.Binary)
		var totalT int64
		for _, c := range calls {
			args := make([]mir2.Value, len(c.Args))
			for i, v := range c.Args {
				args[i] = mir2.Value{I: v}
			}
			for _, vm := range []*mir2.VM{vmRaw, vmFinal} {
				events := 0
				vm.ObserveDataAccess = func(v mir2.VMDataAccess) { events++ }
				v, e := vm.Call(c.Function, args)
				if e != nil || len(v) != 1 || uint16(v[0].I) != c.Want {
					t.Fatal("VM", mode, c, v, e)
				}
				before := beforeRaw
				if vm == vmFinal {
					before = beforeFinal
				}
				if events != 0 || !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
					t.Fatal("pure VM RAM/effects", mode, c, events)
				}
			}
			copy(mem, base)
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			before := append([]byte(nil), mem...)
			r := emulator.Registers{PC: uint16(a.Symbols[c.Function]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777}
			f := s.MIR2Module.FuncByName(c.Function)
			for j, v := range c.Args {
				loc := assertParamLocation(f.Contract.Params[j], s.Allocation)
				switch loc.Name {
				case "A":
					r.A = byte(v)
				case "B":
					r.BC = r.BC&255 | uint16(byte(v))<<8
				case "C":
					r.BC = r.BC&0xff00 | uint16(byte(v))
				case "HL":
					r.HL = uint16(v)
				case "DE":
					r.DE = uint16(v)
				case "BC":
					r.BC = uint16(v)
				case "IX":
					r.IX = uint16(v)
				case "IY":
					r.IY = uint16(v)
				default:
					t.Fatal("ABI", c.Function, loc)
				}
			}
			z.SetRegisters(r)
			startCycles := z.GetCycles()
			minSP := uint16(0xeff0)
			done := false
			for i := 0; i < 100000; i++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					done = true
					break
				}
				if forbidden[z.GetPC()] {
					t.Fatal("unexpected external runtime", c, z.GetPC())
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("SP")
				}
			}
			if !done || z.GetRegisters().HL != c.Want {
				t.Fatal("native", mode, c, z.GetRegisters(), c.Want)
			}
			totalT += int64(z.GetCycles() - startCycles)
			copy(before[minSP:0xeff0], mem[minSP:0xeff0])
			for _, l := range a.Listing {
				if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
					for j := range l.Bytes {
						before[l.Address+j] = mem[l.Address+j]
					}
				}
			}
			if !bytes.Equal(before, mem) {
				for j, b := range mem {
					if before[j] != b {
						t.Fatal("nativeRAM", mode, c, j)
					}
				}
			}
			if len(z.GetOutput()) != 0 {
				t.Fatal("pureoutput")
			}
		}
		save(mode+"/summary.json", map[string]any{"calls": len(calls), "native": "PASS", "raw_final_vm": "PASS", "full_module_bytes": len(a.Binary), "scope": "4purefunctions only; no canvas/render/main", "source_sha256": freeze.SHA, "TStates": totalT})
		t.Log(mode, len(calls), "originalpurefunctions raw/final/native RAM/code/PCSP PASS")
	}
}
