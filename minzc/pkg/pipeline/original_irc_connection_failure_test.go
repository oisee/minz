package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Book v7 chapters 5 and 18: explicit return and CString. The failure path
// terminates the original main; no stop callback is used to terminate its loop.
func TestOriginalIRCConnectionFailureWholeMain(t *testing.T) {
	const packet = "../../../reports/irc-connection-failure/frozen/"
	read := func(n string) []byte {
		b, e := os.ReadFile(packet + n)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var freeze struct {
		Sources   map[string]string
		EventsSHA string `json:"events_sha256"`
	}
	if e := json.Unmarshal(read("freeze.json"), &freeze); e != nil {
		t.Fatal(e)
	}
	for p, w := range freeze.Sources {
		b, e := os.ReadFile("../../../" + p)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != w {
			t.Fatal("source freeze", p, e)
		}
	}
	payload := read("events.json")
	if fmt.Sprintf("%x", sha256.Sum256(payload)) != freeze.EventsSHA {
		t.Fatal("event freeze")
	}
	var expected []commanderEvent
	if e := json.Unmarshal(payload, &expected); e != nil {
		t.Fatal(e)
	}
	save := func(n string, v any) {
		if d := os.Getenv("MINZ_IRC_FAILURE_EXPORT"); d != "" {
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
	isHost := func(f *mir2.Func) bool {
		if f.Attrs.IsExtern {
			return true
		}
		switch f.Name {
		case "tui__render__tui_clear", "tui__render__tui_goto", "tui__render__tui_color", "tui__render__tui_putch", "tui__render__tui_puts", "tui__render__tui_width", "tui__render__tui_reset":
			return true
		}
		return false
	}
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(string(read("irc_client.nanz")), "irc_client.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
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
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", s.MIR2Module.Dump())
		save(mode+"/allocation.json", s.Allocation)
		// Final globals follow set_str and initial declarations independently.
		globalModel := func(m *mir2.Module) map[string][]byte {
			out := map[string][]byte{}
			for _, g := range m.Globals {
				b := make([]byte, mir2.ByteWidth(g.Ty))
				copy(b, g.Init)
				if g.Name == "NICK" {
					copy(b, []byte("minz_user\x00"))
				}
				if g.Name == "CHANNEL" {
					copy(b, []byte("#minz\x00"))
				}
				out[g.Name] = b
			}
			return out
		}
		for _, status := range []int64{2, 3, 4} {
			prefix := fmt.Sprintf("%s/status-%d", mode, status)
			hostFactory := func(readString func(int64) string) (func(string, []int64) []mir2.Value, func()) {
				events := []commanderEvent{}
				reads := 0
				host := func(name string, args []int64) []mir2.Value {
					name = strings.TrimPrefix(name, "tui__render__")
					values := make([]any, 0, len(args))
					for _, v := range args {
						values = append(values, float64(v))
					}
					var ret []mir2.Value
					switch name {
					case "ctl_read":
						if reads >= 3 {
							t.Fatal("extra control read")
						}
						v := int64(0)
						if reads == 2 {
							v = status
						}
						reads++
						ret = []mir2.Value{{I: v}}
					case "tui_width":
						ret = []mir2.Value{{I: 80}}
					case "tui_puts":
						values = []any{readString(args[0])}
					case "ctl_write", "tui_clear", "tui_goto", "tui_color", "tui_putch", "tui_reset":
					default:
						t.Fatal("unexpected failure-path host", name)
					}
					events = append(events, commanderEvent{name, values})
					return ret
				}
				check := func() {
					if reads != 3 || !reflect.DeepEqual(events, expected) {
						save(prefix+"/unexpected-events.json", events)
						for i := 0; i < len(events) && i < len(expected); i++ {
							if !reflect.DeepEqual(events[i], expected[i]) {
								t.Fatalf("event %d got%v want%v", i, events[i], expected[i])
							}
						}
						t.Fatal("event count", len(events), len(expected))
					}
					save(prefix+"/events.json", events)
				}
				return host, check
			}
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
				host, check := hostFactory(func(p int64) string {
					var b []byte
					for j := int64(0); j < 512; j++ {
						if !allowed[p+j] {
							t.Fatal("foreign VM CString", p+j)
						}
						v := vm.ReadHeap(p+j, 1)[0]
						if v == 0 {
							return string(b)
						}
						b = append(b, v)
					}
					t.Fatal("unterminated string")
					return ""
				})
				for _, fn := range engine.m.Funcs {
					if isHost(fn) {
						name := fn.Name
						vm.Hosts[name] = func(v []mir2.Value) ([]mir2.Value, error) {
							args := []int64{}
							for _, x := range v {
								args = append(args, x.I)
							}
							return host(name, args), nil
						}
					}
				}
				result, e := vm.Call("main", nil)
				if e != nil || len(result) != 0 {
					t.Fatal(engine.name, e, result)
				}
				check()
				for name, w := range globalModel(engine.m) {
					if !bytes.Equal(w, vm.ReadHeap(vm.GlobalAddr(name), len(w))) {
						t.Fatal("VM global", name)
					}
				}
				save(prefix+"/"+engine.name+"-result.json", map[string]any{"result": "PASS", "global_memory": true})
			}
			linked := calledMainTestAssembly(t, s)
			for _, fn := range s.MIR2Module.Funcs {
				if fn.Attrs.IsExtern {
					linked = strings.Replace(linked, fn.Name+":", fn.Name+":\n RET", 1)
				}
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x400\n" + linked)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			sym := func(n string) uint16 {
				n = strings.NewReplacer("$", "_", ".", "_", "@", "_").Replace(n)
				p, ok := a.Symbols[n]
				if !ok {
					t.Fatal("symbol", n)
				}
				return uint16(p)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x400:], a.Binary)
			p := sym("main")
			copy(mem[0xc000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
			want := append([]byte(nil), mem...)
			for name, b := range globalModel(s.MIR2Module) {
				copy(want[sym(name):], b)
			}
			allowed := map[int64]bool{}
			for n := 0; n < s.MIR2Module.Strings.Len(); n++ {
				p := sym(fmt.Sprintf("_mir2_str_%d", n))
				for j := 0; j <= len(s.MIR2Module.Strings.At(n)); j++ {
					allowed[int64(p)+int64(j)] = true
				}
			}
			host, check := hostFactory(func(p int64) string {
				var b []byte
				for j := int64(0); j < 512; j++ {
					if !allowed[p+j] {
						t.Fatal("foreign native CString", p+j)
					}
					v := mem[p+j]
					if v == 0 {
						return string(b)
					}
					b = append(b, v)
				}
				t.Fatal("CString")
				return ""
			})
			externs := map[uint16]*mir2.Func{}
			for _, fn := range s.MIR2Module.Funcs {
				if isHost(fn) {
					p := sym(fn.Name)
					if externs[p] != nil {
						t.Fatal("extern alias")
					}
					externs[p] = fn
				}
			}
			z.SetRegisters(emulator.Registers{PC: 0xc000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			minSP := uint16(0xf000)
			for n := 0; n < 1000000 && z.GetPC() != 0xc003; n++ {
				if fn := externs[z.GetPC()]; fn != nil {
					r := z.GetRegisters()
					args := []int64{}
					for _, p := range fn.Contract.Params {
						l := assertParamLocation(p, s.Allocation)
						var v uint16
						switch l.Name {
						case "A":
							v = uint16(r.A)
						case "B":
							v = r.BC >> 8
						case "C":
							v = r.BC & 255
						case "D":
							v = r.DE >> 8
						case "E":
							v = r.DE & 255
						case "H":
							v = r.HL >> 8
						case "L":
							v = r.HL & 255
						case "HL":
							v = r.HL
						case "DE":
							v = r.DE
						case "BC":
							v = r.BC
						case "IX":
							v = r.IX
						case "IY":
							v = r.IY
						default:
							if l.Kind != mir2.LocMem {
								t.Fatal("formal", l)
							}
							addr := sym(l.Name)
							v = uint16(mem[addr])
							if p.Ty.Width() > 8 {
								v |= uint16(mem[addr+1]) << 8
							}
						}
						args = append(args, int64(v))
					}
					values := host(fn.Name, args)
					if len(values) > 0 {
						switch mir2.ReturnLocation(fn.Contract.Returns[0].Class, fn.Contract.Returns[0].Ty) {
						case "A":
							r.A = byte(values[0].I)
						case "HL":
							r.HL = uint16(values[0].I)
						default:
							t.Fatal("return ABI")
						}
					}
					ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					if ret == 0xc003 {
						if strings.TrimPrefix(fn.Name, "tui__render__") != "tui_reset" || r.SP != 0xeffe {
							t.Fatal("loader return authority")
						}
					} else if ret < 0x400 || ret >= uint16(0x400+len(a.Binary)) {
						t.Fatal("foreign return", ret)
					}
					r.PC = ret
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if mem[z.GetPC()] == 0x76 {
					t.Fatal("unexpected HALT")
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xd000 || z.GetSP() > 0xf000 {
					t.Fatal("stack bounds")
				}
			}
			save(prefix+"/program.a80", "ORG 0x400\n"+linked)
			save(prefix+"/program.bin", a.Binary)
			save(prefix+"/registers.json", z.GetRegisters())
			save(prefix+"/ram.bin", mem)
			if z.GetPC() != 0xc003 || z.GetSP() != 0xf000 {
				t.Fatal("termination", z.GetRegisters())
			}
			check()
			copy(want[minSP:0xf000], mem[minSP:0xf000])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
				}
			}
			if !bytes.Equal(want, mem) {
				for p := range mem {
					if mem[p] != want[p] {
						t.Fatalf("native RAM %04x got%x want%x", p, mem[p], want[p])
					}
				}
			}
			save(prefix+"/summary.json", map[string]any{"result": "PASS", "image_bytes": len(a.Binary), "T_states": z.GetCycles(), "events": len(expected), "full_native_ram": true, "minimum_sp": minSP})
			t.Log(mode, status, len(a.Binary), z.GetCycles(), "whole main failure-path PASS")
		}
	}
}
