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

// Native execution of original main and inline LD (HL),A. The model follows
// typed modular arithmetic in Book v7 3.2, not VM results or generated LUT data.
func TestOriginalPlasmaWholeMainNative(t *testing.T) {
	const packet = "../../../reports/plasma-whole-main/frozen/"
	src, e := os.ReadFile(packet + "source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	current, e := os.ReadFile("../../../examples/zx/plasma.nanz")
	if e != nil || !bytes.Equal(src, current) {
		t.Fatal("source freeze", e)
	}
	var freeze struct {
		SHA string `json:"source_sha256"`
	}
	b, e := os.ReadFile(packet + "freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &freeze); e != nil || freeze.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) {
		t.Fatal("freeze", e)
	}
	save := func(n string, v any) {
		if d := os.Getenv("MINZ_PLASMA_MAIN_EXPORT"); d != "" {
			var b []byte
			switch x := v.(type) {
			case []byte:
				b = x
			case string:
				b = []byte(x)
			default:
				var e error
				b, e = json.MarshalIndent(v, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
			}
			p := filepath.Join(d, n)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	sin := func(a byte) byte {
		q := a & 63
		oct := a >> 6
		p := q
		if oct == 1 || oct == 3 {
			p = 63 - q
		}
		d := uint16(64 - p)
		corr := byte((d * d * 7) >> 8)
		if oct < 2 {
			return 240 - corr
		}
		return corr + 16
	}
	dither := func(v byte) byte {
		limits := []int{28, 56, 85, 113, 141, 170, 198, 227}
		patterns := []byte{0, 8, 0x22, 0x55, 0x6b, 0xaa, 0xbb, 0xdd}
		for i, n := range limits {
			if int(v) < n {
				return patterns[i]
			}
		}
		return 255
	}
	type write struct {
		Address uint16
		Value   byte
	}
	var writes []write
	for y := 0; y < 192; y++ {
		for x := 0; x < 32; x++ {
			xx := byte(x * 8)
			yy := byte(y)
			v := sin(xx) + sin(yy+43) + sin((xx>>1)+(yy>>1))
			addr := uint16(0x4000 | ((y & 0xc0) << 5) | ((y & 7) << 8) | ((y & 0x38) << 2) | x)
			writes = append(writes, write{addr, dither(v)})
		}
	}
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			v := sin(byte(x*8)) + sin(byte(y*11)+64)
			a := v >> 5
			if v > 210 {
				a |= 0x40
			}
			writes = append(writes, write{uint16(0x5800 + y*32 + x), a})
		}
	}
	if len(writes) != 6912 {
		t.Fatal("model size")
	}
	save("expected-writes.json", writes)
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.Parse(string(src), "plasma.nanz")
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			// VM receives an explicit model of the sole inline ASM primitive.
			// The native run below executes that primitive, without interception.
			rawHIR, err := nanz.Parse(string(src), "plasma.nanz")
			if err != nil {
				t.Fatal(err)
			}
			for _, engine := range []struct {
				name   string
				module *mir2.Module
			}{{"raw", hir.LowerModule(rawHIR)}, {"final", s.MIR2Module}} {
				vm := mir2.NewVM(engine.module)
				vm.MaxSteps = 10000000
				vm.EnsureHeap(65536)
				before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				expected := append([]byte(nil), before...)
				for _, w := range writes {
					expected[w.Address] = w.Value
				}
				calls := 0
				vm.Hosts["zx_poke"] = func(args []mir2.Value) ([]mir2.Value, error) {
					if len(args) != 2 || calls >= len(writes) || args[0].I != int64(writes[calls].Address) || args[1].I != int64(writes[calls].Value) {
						t.Fatalf("%s VM write %d: %v", engine.name, calls, args)
					}
					vm.WriteHeap(args[0].I, byte(args[1].I))
					calls++
					return nil, nil
				}
				values, err := vm.Call("main", nil)
				if err != nil || len(values) != 0 || calls != len(writes) || !bytes.Equal(vm.ReadHeap(0, 65536), expected) {
					t.Fatal(engine.name, "VM modeled ASM/main", err, calls, values)
				}
				save(mode+"/"+engine.name+"-vm-screen.bin", vm.ReadHeap(0x4000, 6912))
				t.Log(engine.name, "VM with explicit zx_poke model PASS")
			}
			asm := "ORG 0x8000\n" + calledMainTestAssembly(t, s)
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			save(mode+"/program.a80", asm)
			save(mode+"/program.bin", a.Binary)
			save(mode+"/final.mir", s.MIR2Module.Dump())
			save(mode+"/allocation.json", s.Allocation)
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.Hooks = nil
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x8000:], a.Binary)
			main, ok := a.Symbols["main"]
			if !ok {
				t.Fatal("main")
			}
			poke, ok := a.Symbols["zx_poke"]
			if !ok || mem[poke] != 0x77 {
				t.Fatal("actual inline store", poke)
			}
			copy(mem[0x7000:], []byte{0xcd, byte(main), byte(main >> 8), 0})
			want := append([]byte(nil), mem...)
			for _, w := range writes {
				want[w.Address] = w.Value
			}
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, HL: 0x9abc, DE: 0x5678, BC: 0x1234, IX: 0x2468, IY: 0x1357})
			minSP := uint16(0xf000)
			count := 0
			returned := false
			for n := 0; n < 20000000; n++ {
				pc := z.GetPC()
				r := z.GetRegisters()
				if pc == 0x7003 {
					if r.SP != 0xf000 {
						t.Fatal("return SP", r)
					}
					returned = true
					break
				}
				if pc != 0x7000 && (pc < 0x8000 || int(pc) >= 0x8000+len(a.Binary)) {
					t.Fatal("foreign fetch", pc)
				}
				if pc == uint16(poke) {
					if count >= len(writes) || r.HL != writes[count].Address || r.A != writes[count].Value {
						t.Fatalf("write %d got %04x/%02x", count, r.HL, r.A)
					}
					count++
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xe000 || z.GetSP() > 0xf000 {
					t.Fatal("stack bounds")
				}
			}
			if !returned || count != len(writes) {
				t.Fatal("incomplete main", returned, count)
			}
			copy(want[minSP:0xf000], mem[minSP:0xf000])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
				}
			}
			if !bytes.Equal(want, mem) {
				for p := range mem {
					if want[p] != mem[p] {
						t.Fatalf("RAM %04x got %02x want %02x", p, mem[p], want[p])
					}
				}
			}
			save(mode+"/ram.bin", mem)
			save(mode+"/screen.bin", mem[0x4000:0x5b00])
			save(mode+"/summary.json", map[string]any{"bytes": len(a.Binary), "T_states": z.GetCycles(), "ordered_writes": count, "minimum_sp": minSP, "full_ram": true, "PC": z.GetPC(), "SP": z.GetSP()})
			t.Log(len(a.Binary), z.GetCycles(), count, "ordered screen/full RAM/main return PASS")
		})
	}
}
