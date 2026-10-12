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
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestOriginalTUIDemoWholeMain(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/tui_demo.nanz")
	if e != nil {
		t.Fatal(e)
	}
	f, e := os.ReadFile("../../../reports/tui-demo-main/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SHA   string `json:"source_sha256"`
		Cases []struct {
			Input  string `json:"input"`
			Value  int64  `json:"value"`
			Stdout string `json:"stdout"`
		} `json:"cases"`
	}
	if json.Unmarshal(f, &freeze) != nil || fmt.Sprintf("%x", sha256.Sum256(src)) != freeze.SHA {
		t.Fatal("freeze")
	}
	save := func(n string, v any) {
		if root := os.Getenv("MINZ_TUI_DEMO_ARTIFACTS"); root != "" {
			p := filepath.Join(root, n)
			os.MkdirAll(filepath.Dir(p), 0755)
			var b []byte
			switch v := v.(type) {
			case string:
				b = []byte(v)
			case []byte:
				b = v
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), "tui_demo.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		raw := hir.LowerModule(parse())
		s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
		if e != nil {
			t.Fatal(e)
		}
		linked := s.Assembly
		for _, f := range s.MIR2Module.Funcs {
			if f.Attrs.IsExtern {
				linked = strings.Replace(linked, f.Name+":", f.Name+":\n RET", 1)
			}
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + linked)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		save(mode+"/program.a80", "ORG 0x8000\n"+linked)
		save(mode+"/program.bin", a.Binary)
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", s.MIR2Module.Dump())
		save(mode+"/allocation.json", s.Allocation)
		for ci, c := range freeze.Cases {
			newHost := func(read func(int64) byte) (func(string, []int64) []mir2.Value, func() []string) {
				events := []string{}
				value := int64(0)
				registered := false
				shown := false
				return func(name string, args []int64) []mir2.Value {
						switch name {
						case "sel_register_int":
							if registered || len(args) != 2 {
								t.Fatal("registration")
							}
							str := ""
							for n := int64(0); n < 256; n++ {
								b := read(args[0] + n)
								if b == 0 {
									break
								}
								str += string(b)
							}
							if str != "COUNT" || args[1] != 42 {
								t.Fatal("field", str, args)
							}
							registered = true
							value = args[1]
							events = append(events, "register COUNT=42")
							return nil
						case "sel_show":
							if !registered || shown || len(args) != 0 {
								t.Fatal("show")
							}
							line := strings.TrimRight(c.Input, "\r\n")
							if line != "" {
								v, e := strconv.ParseInt(line, 10, 64)
								if e == nil {
									value = v
								}
							}
							shown = true
							events = append(events, "show->1")
							return []mir2.Value{{I: 1}}
						case "sel_get_int":
							if !shown || len(args) != 1 || args[0] != 0 {
								t.Fatal("get")
							}
							events = append(events, fmt.Sprintf("get0->%d", value))
							return []mir2.Value{{I: value}}
						}
						t.Fatal("host", name)
						return nil
					}, func() []string {
						if !registered || !shown || value != c.Value {
							t.Fatal("hoststate")
						}
						return events
					}
			}
			wantEvents := []string{"register COUNT=42", "show->1", fmt.Sprintf("get0->%d", c.Value)}
			for _, engine := range []struct {
				name string
				m    *mir2.Module
			}{{"raw", raw}, {"final", s.MIR2Module}} {
				vm := mir2.NewVM(engine.m)
				allowed := map[int64]bool{}
				for n := 0; n < engine.m.Strings.Len(); n++ {
					p, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", n))
					if e != nil {
						t.Fatal(e)
					}
					for j := 0; j <= len(engine.m.Strings.At(n)); j++ {
						allowed[p+int64(j)] = true
					}
				}
				host, check := newHost(func(p int64) byte {
					if !allowed[p] {
						t.Fatal("CString foreignread", p)
					}
					return vm.ReadHeap(p, 1)[0]
				})
				for _, f := range engine.m.Funcs {
					if f.Attrs.IsExtern {
						name := f.Name
						vm.Hosts[name] = func(v []mir2.Value) ([]mir2.Value, error) {
							args := []int64{}
							for _, x := range v {
								args = append(args, x.I)
							}
							return host(name, args), nil
						}
					}
				}
				out := ""
				vm.Hosts["@mir.io.print.u8"] = func(v []mir2.Value) ([]mir2.Value, error) { out += fmt.Sprint(byte(v[0].I)); return nil, nil }
				vm.EnsureHeap(65536)
				before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				v, e := vm.Call("main", nil)
				if e != nil || len(v) != 0 || out != c.Stdout || !reflect.DeepEqual(check(), wantEvents) {
					t.Fatal("VM", engine.name, mode, v, e, out)
				}
				if !bytes.Equal(before, vm.ReadHeap(0, len(before))) {
					t.Fatal("VMRAM")
				}
				save(fmt.Sprintf("%s/case%d/%s-events.json", mode, ci, engine.name), check())
				save(fmt.Sprintf("%s/case%d/%s-stdout.txt", mode, ci, engine.name), out)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x8000:], a.Binary)
			p := a.Symbols["main"]
			copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
			before := append([]byte(nil), mem...)
			strp := a.Symbols["_mir2_str_0"]
			host, check := newHost(func(p int64) byte {
				if p < int64(strp) || p >= int64(strp+6) {
					t.Fatal("nativeCString", p)
				}
				return mem[p]
			})
			out := []byte{}
			z.SetIOHandlers(func(p uint16) byte { t.Fatal("IN"); return 0 }, func(p uint16, v byte) {
				if byte(p) != 0x23 || byte(p>>8) != v {
					t.Fatal("OUT", p, v)
				}
				out = append(out, v)
			})
			externs := map[uint16]*mir2.Func{}
			for _, f := range s.MIR2Module.Funcs {
				if f.Attrs.IsExtern {
					externs[uint16(a.Symbols[f.Name])] = f
				}
			}
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			minSP := uint16(0xf000)
			done := false
			for step := 0; step < 10000; step++ {
				pc := z.GetPC()
				if pc == 0x7003 {
					done = true
					break
				}
				if f := externs[pc]; f != nil {
					r := z.GetRegisters()
					args := []int64{}
					for _, p := range f.Contract.Params {
						l := assertParamLocation(p, s.Allocation)
						var v uint16
						switch l.Name {
						case "A":
							v = uint16(r.A)
						case "B":
							v = r.BC >> 8
						case "C":
							v = r.BC & 255
						case "HL":
							v = r.HL
						case "DE":
							v = r.DE
						case "BC":
							v = r.BC
						default:
							t.Fatal("formal", l)
						}
						args = append(args, int64(v))
					}
					vs := host(f.Name, args)
					r.A = 0x91
					r.F = 0xd7
					r.BC = 0xdead
					r.DE = 0xbeef
					r.HL = 0x7654
					r.IX = 0x1357
					r.IY = 0x2468
					if len(vs) > 0 {
						if f.Contract.Returns[0].Ty.Width() == 8 {
							r.A = byte(vs[0].I)
						} else {
							r.HL = uint16(vs[0].I)
						}
					}
					r.PC = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if mem[pc] == 0x76 {
					if z.GetSP() != 0xeffe {
						t.Fatal("haltSP")
					}
					done = true
					break
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if !done || string(out) != c.Stdout || !reflect.DeepEqual(check(), wantEvents) {
				t.Fatal("native", mode, ci, z.GetRegisters(), string(out))
			}
			copy(before[minSP:0xf000], mem[minSP:0xf000])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					copy(before[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
				}
			}
			if !bytes.Equal(before, mem) {
				t.Fatal("nativeRAM")
			}
			save(fmt.Sprintf("%s/case%d/native-events.json", mode, ci), check())
			save(fmt.Sprintf("%s/case%d/native-stdout.txt", mode, ci), out)
			save(fmt.Sprintf("%s/case%d/registers.json", mode, ci), z.GetRegisters())
			save(fmt.Sprintf("%s/case%d/summary.json", mode, ci), map[string]any{"result": "PASS", "T_states": z.GetCycles(), "image_bytes": len(a.Binary), "full_ram": true, "minimum_sp": minSP, "SP": z.GetSP(), "stdout": string(out), "extern_clobber_stress": true})
		}
	}
}
