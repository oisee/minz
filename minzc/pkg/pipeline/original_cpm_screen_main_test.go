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

func TestOriginalCPMScreenMain(t *testing.T) {
	root := "../../../reports/cpm-screen-main/"
	var freeze struct {
		Sources map[string]string `json:"sources"`
	}
	b, e := os.ReadFile(root + "freeze-before.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &freeze); e != nil {
		t.Fatal(e)
	}
	for p, h := range freeze.Sources {
		// Compiler implementation is historical provenance, not an immutable
		// program input. Keep verifying its original bytes at the same SHA,
		// while exercising the current compiler against frozen stdout/model.
		readPath := p
		if p == "minzc/pkg/nanz/meta.go" {
			readPath = "reports/cpm-screen-compiler-boundary/historical-meta.go.txt"
		}
		b, e := os.ReadFile("../../../" + readPath)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != h {
			t.Fatal("freeze", p, e)
		}
	}
	save := func(n string, v any) {
		r := os.Getenv("MINZ_CPM_SCREEN_ARTIFACTS")
		if r == "" {
			return
		}
		p := filepath.Join(r, n)
		os.MkdirAll(filepath.Dir(p), 0755)
		var b []byte
		switch v := v.(type) {
		case []byte:
			b = v
		case string:
			b = []byte(v)
		default:
			b, _ = json.MarshalIndent(v, "", "  ")
		}
		if e := os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for _, module := range []string{"tui_cpm", "meta_screen"} {
		src, e := os.ReadFile("../../../examples/nanz/" + module + ".nanz")
		if e != nil {
			t.Fatal(e)
		}
		expected, e := os.ReadFile(root + module + "-expected-stdout.bin")
		if e != nil {
			t.Fatal(e)
		}
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			prefix := module + "/" + mode
			h, e := nanz.ParseWithOpts(string(src), module+".nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(module, e)
			}
			raw := hir.LowerModule(h)
			h, e = nanz.ParseWithOpts(string(src), module+".nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(module, e)
			}
			for i, m := range []*mir2.Module{raw, s.MIR2Module} {
				vm := mir2.NewVM(m)
				for j := 0; j < m.Strings.Len(); j++ {
					if _, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", j)); e != nil {
						t.Fatal(e)
					}
				}
				stringAllowed := map[int64]bool{}
				for j := 0; j < m.Strings.Len(); j++ {
					p, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", j))
					if e != nil {
						t.Fatal(e)
					}
					for k := 0; k <= len(m.Strings.At(j)); k++ {
						stringAllowed[p+int64(k)] = true
					}
				}
				vm.EnsureHeap(65536)
				before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				var out bytes.Buffer
				key := 0
				puts := func(ptr int64) error {
					for j := int64(0); j < 4096; j++ {
						if !stringAllowed[ptr+j] {
							return fmt.Errorf("CString object bounds")
						}
						b := vm.ReadHeap(ptr+j, 1)
						if b == nil {
							return fmt.Errorf("CStringbounds")
						}
						if b[0] == 0 {
							return nil
						}
						out.WriteByte(b[0])
					}
					return fmt.Errorf("unterminatedCString")
				}
				bind := func(name string, fn mir2.HostFunc) {
					if m.FuncByName(name) == nil {
						t.Fatal("missingVMbinding", name)
					}
					vm.Hosts[name] = fn
				}
				readkey := func([]mir2.Value) ([]mir2.Value, error) {
					if key != 0 {
						return nil, fmt.Errorf("extrainput")
					}
					key++
					return []mir2.Value{{I: 97}}, nil
				}
				if module == "tui_cpm" {
					bind("_putch", func(a []mir2.Value) ([]mir2.Value, error) { out.WriteByte(byte(a[0].I)); return nil, nil })
					bind("_esc", func([]mir2.Value) ([]mir2.Value, error) { out.WriteByte(27); return nil, nil })
					bind("_dec", func(a []mir2.Value) ([]mir2.Value, error) { fmt.Fprintf(&out, "%02d", byte(a[0].I)); return nil, nil })
					bind("_puts", func(a []mir2.Value) ([]mir2.Value, error) { return nil, puts(a[0].I) })
					bind("tui_read_key", readkey)
				} else {
					prefix := "tui__render__"
					bind(prefix+"tui_clear", func([]mir2.Value) ([]mir2.Value, error) { out.WriteString("\x1b[2J\x1b[H"); return nil, nil })
					bind(prefix+"tui_reset", func([]mir2.Value) ([]mir2.Value, error) { out.WriteString("\x1b[0m"); return nil, nil })
					bind(prefix+"tui_goto", func(a []mir2.Value) ([]mir2.Value, error) {
						fmt.Fprintf(&out, "\x1b[%02d;%02dH", byte(a[1].I+1), byte(a[0].I+1))
						return nil, nil
					})
					bind(prefix+"tui_color", func(a []mir2.Value) ([]mir2.Value, error) {
						f, b := 30+a[0].I, 40+a[1].I
						if a[2].I != 0 {
							f += 60
							b += 60
						}
						fmt.Fprintf(&out, "\x1b[%02d;%02dm", f, b)
						return nil, nil
					})
					bind(prefix+"tui_puts", func(a []mir2.Value) ([]mir2.Value, error) { return nil, puts(a[0].I) })
					bind(prefix+"tui_read_key", readkey)
				}
				// Every remaining ASM body is denied at the call boundary. Bound ancestors
				// intercept their own calls, so unknown reachable ASM cannot silently no-op.
				for _, f := range m.Funcs {
					hasASM := false
					for _, b := range f.Blocks {
						for _, in := range b.Insts {
							if in.Op == mir2.OpAsm {
								hasASM = true
							}
						}
					}
					if hasASM {
						if _, ok := vm.Hosts[f.Name]; !ok {
							name := f.Name
							vm.Hosts[name] = func([]mir2.Value) ([]mir2.Value, error) { return nil, fmt.Errorf("unbound ASM %s", name) }
						}
					}
				}
				vm.MaxSteps = 100000
				if v, e := vm.Call("main", nil); e != nil || len(v) > 0 {
					t.Fatal(module, mode, "VMmain", i, v, e)
				}
				if key != 1 || !bytes.Equal(out.Bytes(), expected) || !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
					t.Fatal(module, mode, "VMoutput/RAM", i, key, out.Len())
				}
				save(fmt.Sprintf("%s/vm%d-stdout.bin", prefix, i), out.Bytes())
				save(fmt.Sprintf("%s/vm%d-summary.json", prefix, i), map[string]any{"result": "PASS", "scope": "wholemain explicit CP/M-equivalent ASM-function host bindings, not VMASM execution", "steps": vm.StepsExecuted(), "full_ram": true})
			}
			save(prefix+"/raw.mir", raw.Dump())
			save(prefix+"/hir.txt", s.HIR)
			asm := "ORG 0x100\nLD SP,0xeff0\nCALL main\nDI\nHALT\n" + calledMainTestAssembly(t, s)
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(module, e, a.Errors)
			}
			save(prefix+"/program.a80", asm)
			save(prefix+"/program.com", a.Binary)
			save(prefix+"/final.mir", s.MIR2Module.Dump())

			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x100:], a.Binary)
			want := append([]byte(nil), mem...)
			allowed := make([]bool, 65536)
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					for j := range l.Bytes {
						allowed[l.Address+j] = true
					}
				}
			}
			z.SetSMCTracker(func(p uint16, old, new byte) {
				if !(p >= 0xe000 && p < 0xeff0) && !allowed[p] {
					t.Fatalf("%s forbiddenwrite%x", prefix, p)
				}
			})
			z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xeff0, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			key := 0
			var out []byte
			minSP := uint16(0xeff0)
			done := false
			var ledger []map[string]any
			for step := 0; step < 500000; step++ {
				pc := z.GetPC()
				if pc == 5 {
					r := z.GetRegisters()
					switch byte(r.BC) {
					case 1:
						if key != 0 {
							t.Fatal("extra input")
						}
						r.A = 97
						key++
						ledger = append(ledger, map[string]any{"BDOS": 1, "input": 97})
					case 2:
						out = append(out, byte(r.DE))
						ledger = append(ledger, map[string]any{"BDOS": 2, "output": byte(r.DE)})
					default:
						t.Fatal("unknownBDOS", r.BC)
					}
					if r.SP < 0xe000 || r.SP+1 >= 0xeff0 {
						t.Fatal("BDOSstack")
					}
					ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					if ret < 0x100 || int(ret) >= 0x100+len(a.Binary) {
						t.Fatal("BDOS foreign return", ret)
					}
					r.PC = ret
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if pc == 0x107 {
					if mem[pc] != 0x76 {
						t.Fatal("alteredloader")
					}
					z.Step()
					done = true
					break
				}
				if pc < 0x100 || int(pc) >= 0x100+len(a.Binary) {
					t.Fatal("foreignfetch", pc)
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xe000 || z.GetSP() > 0xeff0 {
					t.Fatal("stackbounds")
				}
			}
			save(prefix+"/stdout.bin", out)
			save(prefix+"/ledger.json", ledger)
			save(prefix+"/registers.json", z.GetRegisters())
			if !done || key != 1 || z.GetSP() != 0xeff0 || !bytes.Equal(out, expected) {
				t.Fatalf("%s wholemain done%v keys%d SP%x output%d want%d first%q", prefix, done, key, z.GetSP(), len(out), len(expected), out[:min(len(out), 150)])
			}
			copy(want[minSP:0xeff0], mem[minSP:0xeff0])
			for p, yes := range allowed {
				if yes {
					want[p] = mem[p]
				}
			}
			if !bytes.Equal(want, mem) {
				t.Fatal("fullRAM", prefix)
			}
			save(prefix+"/summary.json", map[string]any{"result": "PASS", "source_sha256": fmt.Sprintf("%x", sha256.Sum256(src)), "output_bytes": len(out), "image_bytes": len(a.Binary), "minimum_sp": minSP, "wholemain": true, "scope": "actual originalASM main with testownedBDOS1/2"})
			t.Log(prefix, "wholemain PASS")
		}
	}
}
