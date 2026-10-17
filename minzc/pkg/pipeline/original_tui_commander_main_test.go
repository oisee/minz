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
	"strings"
	"testing"
)

type commanderEvent struct {
	Name string
	Args []any
}

func commanderANSI(events []commanderEvent) string {
	var o strings.Builder
	for _, e := range events {
		num := func(n int) int { return int(e.Args[n].(float64)) }
		switch e.Name {
		case "tui_clear":
			o.WriteString("\x1b[2J\x1b[H")
		case "tui_color":
			fg, bg := 30+num(0), 40+num(1)
			if num(2) != 0 {
				fg += 60
				bg += 60
			}
			fmt.Fprintf(&o, "\x1b[%d;%dm", fg, bg)
		case "tui_goto":
			fmt.Fprintf(&o, "\x1b[%d;%dH", num(1)+1, num(0)+1)
		case "tui_puts":
			o.WriteString(e.Args[0].(string))
		case "tui_reset":
			o.WriteString("\x1b[0m")
		}
	}
	return o.String()
}
func TestOriginalTUICommanderWholeMain(t *testing.T) {
	testOriginalTUIWholeMain(t, "tui_commander.nanz", "tui-commander-main", "MINZ_TUI_COMMANDER_ARTIFACTS")
}

func testOriginalTUIWholeMain(t *testing.T, sourceName, packetName, artifactEnv string) {
	t.Helper()
	src, e := os.ReadFile(filepath.Join("../../../examples/nanz", sourceName))
	if e != nil {
		t.Fatal(e)
	}
	f, e := os.ReadFile(filepath.Join("../../../reports", packetName, "freeze.json"))
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SHA     string           `json:"source_sha256"`
		Keys    []int64          `json:"input_keys"`
		Globals map[string]int64 `json:"final_globals"`
	}
	if json.Unmarshal(f, &freeze) != nil || freeze.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) {
		t.Fatal("freeze")
	}
	var expected []commanderEvent
	b, e := os.ReadFile(filepath.Join("../../../reports", packetName, "events.json"))
	if e != nil || json.Unmarshal(b, &expected) != nil {
		t.Fatal(e)
	}
	wantANSI := commanderANSI(expected)
	save := func(n string, v any) {
		if root := os.Getenv(artifactEnv); root != "" {
			p := filepath.Join(root, n)
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
	}
	save("expected-stdout.bin", wantANSI)
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), sourceName)
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
		hostFactory := func(read func(int64) byte) (func(string, []int64) []mir2.Value, func() []commanderEvent) {
			events := []commanderEvent{}
			keys := 0
			return func(name string, args []int64) []mir2.Value {
					values := []any{}
					for _, v := range args {
						values = append(values, float64(v))
					}
					ret := []mir2.Value(nil)
					if name == "tui_puts" {
						str := ""
						ended := false
						for n := int64(0); n < 4096; n++ {
							v := read(args[0] + n)
							if v == 0 {
								ended = true
								break
							}
							str += string(v)
						}
						if !ended {
							t.Fatal("CString")
						}
						values = []any{str}
					}
					if name == "tui_read_key" {
						if keys >= len(freeze.Keys) {
							t.Fatal("extra key")
						}
						key := freeze.Keys[keys]
						keys++
						values = []any{float64(key)}
						ret = []mir2.Value{{I: key}}
					}
					if name == "tui_width" {
						ret = []mir2.Value{{I: 80}}
					}
					if name == "tui_height" {
						ret = []mir2.Value{{I: 24}}
					}
					events = append(events, commanderEvent{name, values})
					return ret
				}, func() []commanderEvent {
					if keys != len(freeze.Keys) {
						t.Fatal("keys", keys)
					}
					return events
				}
		}
		for _, engine := range []struct {
			name string
			m    *mir2.Module
		}{{"raw", raw}, {"final", s.MIR2Module}} {
			vm := mir2.NewVM(engine.m)
			allow := map[int64]bool{}
			for n := 0; n < engine.m.Strings.Len(); n++ {
				p, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", n))
				if e != nil {
					t.Fatal(e)
				}
				for j := 0; j <= len(engine.m.Strings.At(n)); j++ {
					allow[p+int64(j)] = true
				}
			}
			vm.EnsureHeap(65536)
			want := append([]byte(nil), vm.ReadHeap(0, 65536)...)
			for n, v := range freeze.Globals {
				want[vm.GlobalAddr(n)] = byte(v)
			}
			host, check := hostFactory(func(p int64) byte {
				if !allow[p] {
					t.Fatal("VMforeignCString", p)
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
			v, e := vm.Call("main", nil)
			events := check()
			save(mode+"/"+engine.name+"-events.json", events)
			if e != nil || len(v) != 0 || !reflect.DeepEqual(events, expected) || commanderANSI(events) != wantANSI {
				t.Fatal("VM", mode, engine.name, e)
			}
			if !bytes.Equal(want, vm.ReadHeap(0, 65536)) {
				t.Fatal("VMRAM")
			}
			save(mode+"/"+engine.name+"-stdout.bin", commanderANSI(events))
		}
		linked := calledMainTestAssembly(t, s)
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
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		mem := z.Memory()
		copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
		copy(mem[0x8000:], a.Binary)
		p := a.Symbols["main"]
		copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
		want := append([]byte(nil), mem...)
		for n, v := range freeze.Globals {
			want[a.Symbols[n]] = byte(v)
		}
		allow := map[int64]bool{}
		for n := 0; n < s.MIR2Module.Strings.Len(); n++ {
			p := a.Symbols[fmt.Sprintf("_mir2_str_%d", n)]
			for j := 0; j <= len(s.MIR2Module.Strings.At(n)); j++ {
				allow[int64(p+j)] = true
			}
		}
		host, check := hostFactory(func(p int64) byte {
			if !allow[p] {
				t.Fatal("nativeforeignCString", p)
			}
			return mem[p]
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
		for n := 0; n < 1000000; n++ {
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
					r.A = byte(vs[0].I)
				}
				r.PC = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
				r.SP += 2
				z.SetRegisters(r)
				continue
			}
			if mem[pc] == 0x76 {
				t.Fatal("unexpected HALT before main returned", pc)
			}
			z.Step()
			if z.GetSP() < minSP {
				minSP = z.GetSP()
			}
			if minSP < 0xe000 {
				t.Fatal("stack")
			}
		}
		events := check()
		save(mode+"/native-events.json", events)
		save(mode+"/native-stdout.bin", commanderANSI(events))
		save(mode+"/registers.json", z.GetRegisters())
		if !done || z.GetPC() != 0x7003 || z.GetSP() != 0xf000 || !reflect.DeepEqual(events, expected) || commanderANSI(events) != wantANSI {
			t.Fatal("native", mode, done, z.GetRegisters())
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
					t.Fatalf("native RAM%04x got%x want%x", p, mem[p], want[p])
				}
			}
		}
		save(mode+"/summary.json", map[string]any{"result": "PASS", "events": len(events), "stdout_bytes": len(wantANSI), "T_states": z.GetCycles(), "image_bytes": len(a.Binary), "full_ram": true, "SP": z.GetSP(), "minimum_sp": minSP, "host_clobber_stress": true})
	}
}
