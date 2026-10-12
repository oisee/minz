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
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Executes original inline ASM and source calls. Only the two declared ROM
// services are intercepted; this is a ROM output protocol proof, not a screen
// or physical ZX ROM implementation proof.
func TestOriginalTUIZXWholeMainNative(t *testing.T) {
	const packet = "../../../reports/tui-zx-main/frozen/"
	read := func(n string) []byte {
		b, e := os.ReadFile(packet + n)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var f struct {
		SHA    string `json:"source_sha256"`
		Output string `json:"output_sha256"`
		Bytes  int    `json:"output_bytes"`
	}
	if e := json.Unmarshal(read("freeze.json"), &f); e != nil {
		t.Fatal(e)
	}
	src := read("source.nanz")
	current, e := os.ReadFile("../../../examples/nanz/tui_zx.nanz")
	wantOutput := read("rom-output.bin")
	if e != nil || !bytes.Equal(src, current) || fmt.Sprintf("%x", sha256.Sum256(src)) != f.SHA || fmt.Sprintf("%x", sha256.Sum256(wantOutput)) != f.Output || len(wantOutput) != f.Bytes {
		t.Fatal("freeze", e)
	}
	save := func(n string, v any) {
		if d := os.Getenv("MINZ_TUI_ZX_EXPORT"); d != "" {
			var b []byte
			switch x := v.(type) {
			case string:
				b = []byte(x)
			case []byte:
				b = x
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
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
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.Parse(string(src), "tui_zx.nanz")
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			asm := "ORG 0x8000\n" + s.Assembly
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
			z.Hooks = nil // execute real RST; explicit ROM boundary below owns callbacks
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x8000:], a.Binary)
			p, ok := a.Symbols["main"]
			if !ok {
				t.Fatal("main")
			}
			copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
			want := append([]byte(nil), mem...)
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, HL: 0x9abc, DE: 0x5678, BC: 0x1234, IX: 0x2468, IY: 0x1357})
			minSP := uint16(0xf000)
			output := []byte{}
			initCalls := 0
			halt := false
			for n := 0; n < 1000000; n++ {
				pc := z.GetPC()
				r := z.GetRegisters()
				if pc == 0x1601 || pc == 0x10 {
					ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					if ret < 0x8000 || int(ret) >= 0x8000+len(a.Binary) {
						t.Fatal("ROM continuation", ret)
					}
					if pc == 0x1601 {
						if initCalls != 0 || len(output) != 0 || r.A != 2 || mem[ret-3] != 0xcd || mem[ret-2] != 1 || mem[ret-1] != 0x16 {
							t.Fatal("initial ROM call", r)
						}
						initCalls++
					} else {
						if initCalls != 1 || mem[ret-1] != 0xd7 {
							t.Fatal("RST authority", ret)
						}
						if len(output) >= len(wantOutput) || r.A != wantOutput[len(output)] {
							t.Fatalf("ROM output byte%d got%02x", len(output), r.A)
						}
						output = append(output, r.A)
					}
					r.PC = ret
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if pc != 0x7000 && (pc < 0x8000 || int(pc) >= 0x8000+len(a.Binary)) {
					t.Fatal("foreign fetch", pc)
				}
				if mem[pc] == 0x76 {
					if mem[pc-1] != 0xf3 || r.SP != 0xeffe {
						t.Fatal("HALT contract", r)
					}
					halt = true
					break
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xe000 || z.GetSP() > 0xf000 {
					t.Fatal("stack bounds")
				}
			}
			if !halt || initCalls != 1 || !bytes.Equal(output, wantOutput) {
				t.Fatal("incomplete main", halt, initCalls, len(output))
			}
			if uint16(mem[0xeffe])|uint16(mem[0xefff])<<8 != 0x7003 {
				t.Fatal("loader return word")
			}
			copy(want[minSP:0xf000], mem[minSP:0xf000])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
				}
			}
			if !bytes.Equal(want, mem) {
				for p := range mem {
					if mem[p] != want[p] {
						t.Fatalf("RAM%04x got%x want%x", p, mem[p], want[p])
					}
				}
			}
			save(mode+"/rom-output.bin", output)
			save(mode+"/ram.bin", mem)
			save(mode+"/registers.json", z.GetRegisters())
			save(mode+"/summary.json", map[string]any{"result": "PASS", "bytes": len(a.Binary), "T_states": z.GetCycles(), "output_bytes": len(output), "minimum_sp": minSP, "termination": "original DI/HALT with SP EFFE", "full_ram": true})
			t.Log(len(a.Binary), z.GetCycles(), len(output), "ROM protocol/full RAM PASS")
		})
	}
}
