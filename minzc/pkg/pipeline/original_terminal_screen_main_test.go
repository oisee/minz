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

type terminalScreenEvent struct {
	Name string `json:"name"`
	Args []any  `json:"args"`
}

func TestOriginalTerminalScreenWholeMain(t *testing.T) {
	root := "../../../reports/terminal-screen-main/"
	read := func(n string) []byte {
		b, e := os.ReadFile(root + n)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var freeze struct {
		Sources map[string]string `json:"sources"`
	}
	if e := json.Unmarshal(read("freeze.json"), &freeze); e != nil {
		t.Fatal(e)
	}
	for p, h := range freeze.Sources {
		b, e := os.ReadFile("../../../" + strings.TrimPrefix(p, "/home/alice/dev/minz-artifacts/grace-workspaces/original-terminal-screen-main/"))
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != h {
			t.Fatal("freeze", p, e)
		}
	}
	for _, caseName := range []string{"tui", "tab", "edit", "cancel"} {
		var scenario struct {
			Module   string                `json:"module"`
			Keys     []int64               `json:"keys"`
			Line     *string               `json:"line"`
			Focus    int64                 `json:"focus"`
			Material string                `json:"material"`
			Events   []terminalScreenEvent `json:"events"`
		}
		if e := json.Unmarshal(read(caseName+".json"), &scenario); e != nil {
			t.Fatal(e)
		}
		src := read(scenario.Module + ".nanz")
		if fmt.Sprintf("%x", sha256.Sum256(src)) != freeze.Sources["examples/nanz/"+scenario.Module+".nanz"] {
			t.Fatal("snapshotidentity")
		}
		expected := scenario.Events
		stdout := read(caseName + "-stdout.bin")
		save := func(n string, v any) {
			r := os.Getenv("MINZ_TERMINAL_SCREEN_ARTIFACTS")
			if r == "" {
				return
			}
			p := filepath.Join(r, caseName, n)
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
		parse := func() *hir.Module {
			h, e := nanz.ParseWithOpts(string(read(scenario.Module+".nanz")), scenario.Module+".nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		apply := func(mem []byte, addr func(string) int, stringAddr int) {
			if scenario.Module == "tui_screen" {
				return
			}
			put := func(n string, v int64, w int) {
				p := addr(n)
				if p < 0 {
					t.Fatal(n)
				}
				for j := 0; j < w; j++ {
					mem[p+j] = byte(v >> uint(j*8))
				}
			}
			put("scr_title", int64(stringAddr), 2)
			put("scr_nfields", 5, 1)
			put("scr_focus", scenario.Focus, 1)
			put("val_count", 10, 2)
			p := addr("buf_material")
			copy(mem[p:], []byte(scenario.Material))
			mem[p+len(scenario.Material)] = 0
			mem[addr("buf_plant")] = 0
		}
		newHost := func(engine string, readByte func(int64) byte, writeLine func(int64, []byte)) (func(string, []int64) ([]mir2.Value, error), func()) {
			var actual []terminalScreenEvent
			var out bytes.Buffer
			key := 0
			keys := scenario.Keys
			var reads []int64
			call := func(name string, args []int64) ([]mir2.Value, error) {
				values := make([]any, len(args))
				for i, a := range args {
					values[i] = a
				}
				var result []mir2.Value
				switch name {
				case "tui_goto":
					fmt.Fprintf(&out, "\x1b[%d;%dH", args[1]+1, args[0]+1)
				case "tui_color":
					fg, bg := 30+args[0], 40+args[1]
					if args[2] != 0 {
						fg += 60
						bg += 60
					}
					fmt.Fprintf(&out, "\x1b[%d;%dm", fg, bg)
				case "tui_reset":
					out.WriteString("\x1b[0m")
				case "tui_clear":
					out.WriteString("\x1b[2J\x1b[H")
				case "tui_putch":
					ch := byte(args[0])
					box := map[byte]string{1: "┌", 2: "┐", 3: "└", 4: "┘", 5: "─", 6: "│"}
					if s, ok := box[ch]; ok {
						out.WriteString(s)
					} else {
						fmt.Fprintf(&out, "%c", ch)
					}
				case "tui_puts":
					var b []byte
					ended := false
					for i := int64(0); i < 4096; i++ {
						p := args[0] + i
						v := readByte(p)
						reads = append(reads, p)
						if v == 0 {
							ended = true
							break
						}
						b = append(b, v)
					}
					if !ended {
						return nil, fmt.Errorf("unterminatedCString")
					}
					values = []any{string(b)}
					out.Write(b)
				case "tui_read_key":
					if key >= len(keys) {
						return nil, fmt.Errorf("extra input")
					}
					result = []mir2.Value{{I: keys[key]}}
					values = []any{keys[key]}
					key++
				case "tui_read_line":
					if scenario.Line == nil {
						return nil, fmt.Errorf("unexpected lineinput")
					}
					line := []byte(*scenario.Line)
					if len(line) > int(args[1]) {
						line = line[:args[1]]
					}
					out.WriteString("\x1b[?25h\x1b[?25l")
					writeLine(args[0], append(append([]byte(nil), line...), 0))
					values = []any{"buf_material", args[1], string(line)}
					result = []mir2.Value{{I: int64(len(line))}}
				case "tui_width":
					result = []mir2.Value{{I: 80}}
				case "tui_height":
					result = []mir2.Value{{I: 24}}
				default:
					return nil, fmt.Errorf("unbound %s", name)
				}
				actual = append(actual, terminalScreenEvent{name, values})
				return result, nil
			}
			check := func() {
				a, _ := json.Marshal(actual)
				b, _ := json.Marshal(expected)
				save(engine+"/events.json", actual)
				save(engine+"/stdout.bin", out.Bytes())
				save(engine+"/string-reads.json", reads)
				if !bytes.Equal(a, b) {
					for i := 0; i < len(actual) && i < len(expected); i++ {
						x, _ := json.Marshal(actual[i])
						y, _ := json.Marshal(expected[i])
						if !bytes.Equal(x, y) {
							t.Fatalf("%s event%d got%s want%s", engine, i, x, y)
						}
					}
					t.Fatalf("%s events%d want%d", engine, len(actual), len(expected))
				}
				if key != len(keys) || !bytes.Equal(out.Bytes(), stdout) {
					t.Fatal(engine, "stdout/input", key, out.Len())
				}
			}
			return call, check
		}
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			raw := hir.LowerModule(parse())
			s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			save(mode+"/raw.mir", raw.Dump())
			save(mode+"/final.mir", s.MIR2Module.Dump())
			save(mode+"/hir.txt", s.HIR)
			save(mode+"/opt.mir", s.MIR2Opt)
			for i, m := range []*mir2.Module{raw, s.MIR2Module} {
				engine := fmt.Sprintf("%s/vm%d", mode, i)
				vm := mir2.NewVM(m)
				for j := 0; j < m.Strings.Len(); j++ {
					if _, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", j)); e != nil {
						t.Fatal(e)
					}
				}
				stringAllowed := map[int64]bool{}
				for j := 0; j < m.Strings.Len(); j++ {
					v, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", j))
					if e != nil {
						t.Fatal(e)
					}
					for k := 0; k <= len(m.Strings.At(j)); k++ {
						stringAllowed[v+int64(k)] = true
					}
				}
				vm.EnsureHeap(65536)
				want := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				title := -1
				for j := 0; j < m.Strings.Len(); j++ {
					if m.Strings.At(j) == "Material Report" {
						v, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", j))
						if e != nil {
							t.Fatal(e)
						}
						title = int(v)
					}
				}
				apply(want, func(n string) int { return int(vm.GlobalAddr(n)) }, title)
				host, check := newHost(engine, func(p int64) byte {
					if scenario.Module == "abap_screen" {
						for _, n := range []string{"buf_material", "buf_plant"} {
							base := vm.GlobalAddr(n)
							size := 12
							if n == "buf_plant" {
								size = 8
							}
							for k := 0; k < size; k++ {
								stringAllowed[base+int64(k)] = true
							}
						}
					}
					if !stringAllowed[p] {
						t.Fatal("VM CString foreign object read", p)
					}
					b := vm.ReadHeap(p, 1)
					if b == nil {
						t.Fatal("VM CString bounds", p)
					}
					return b[0]
				}, func(p int64, b []byte) {
					if p != vm.GlobalAddr("buf_material") || len(b) > 11 {
						t.Fatal("VM line write bound")
					}
					vm.WriteHeapBytes(p, b)
				})
				for _, f := range m.Funcs {
					if f.Attrs.IsExtern {
						name := f.Name
						vm.Hosts[name] = func(args []mir2.Value) ([]mir2.Value, error) {
							ints := []int64{}
							for _, v := range args {
								ints = append(ints, v.I)
							}
							return host(name, ints)
						}
					}
				}
				vm.MaxSteps = 2000000
				if v, e := vm.Call("main", nil); e != nil || len(v) > 0 {
					t.Fatal(engine, v, e)
				}
				check()
				if !bytes.Equal(want, vm.ReadHeap(0, 65536)) {
					for p, b := range vm.ReadHeap(0, 65536) {
						if b != want[p] {
							t.Fatalf("%s RAM%x got%d want%d", engine, p, b, want[p])
						}
					}
				}
				save(engine+"/summary.json", map[string]any{"result": "PASS", "full_ram": true, "steps": vm.StepsExecuted(), "events": len(expected)})
			}
			callerAssembly := s.Assembly
			if scenario.Module != "abap_screen" {
				callerAssembly = calledMainTestAssembly(t, s)
			}
			linked := callerAssembly
			for _, f := range s.MIR2Module.Funcs {
				if f.Attrs.IsExtern {
					label := f.Name + ":"
					if strings.Count(linked, label) != 1 {
						t.Fatal("extern label count", f.Name)
					}
					linked = strings.Replace(linked, label, label+"\n    RET ; explicit test-owned terminal linker slot", 1)
				}
			}
			save(mode+"/native/unlinked.a80", callerAssembly)
			loader := "ORG 0x100\n__test_loader:\nLD SP,0xeff0\nCALL main\nDI\nHALT\n"
			if scenario.Module == "abap_screen" {
				loader = "ORG 0x100\n__test_loader:\nLD SP,0xeff0\nJP main\n"
			}
			asm := loader + linked
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			save(mode+"/native/program.a80", asm)
			save(mode+"/native/program.com", a.Binary)
			save(mode+"/raw.mir", raw.Dump())
			save(mode+"/final.mir", s.MIR2Module.Dump())
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x100:], a.Binary)
			want := append([]byte(nil), mem...)
			title := -1
			for j := 0; j < s.MIR2Module.Strings.Len(); j++ {
				if s.MIR2Module.Strings.At(j) == "Material Report" {
					title = a.Symbols[fmt.Sprintf("_mir2_str_%d", j)]
				}
			}
			apply(want, func(n string) int { return a.Symbols[n] }, title)
			externs := map[uint16]*mir2.Func{}
			var abiMapping []map[string]any
			for _, f := range s.MIR2Module.Funcs {
				if f.Attrs.IsExtern {
					if p, ok := a.Symbols[f.Name]; ok {
						if mem[p] != 0xc9 {
							t.Fatal("extern stub", f.Name)
						}
						if _, exists := externs[uint16(p)]; exists {
							t.Fatal("aliased extern bindings", f.Name)
						}
						externs[uint16(p)] = f
						var homes []string
						for _, param := range f.Contract.Params {
							homes = append(homes, assertParamLocation(param, s.Allocation).Name)
						}
						abiMapping = append(abiMapping, map[string]any{"name": f.Name, "entry_pc": p, "formal_homes": homes, "contract": f.Contract})
					}
				}
			}
			save(mode+"/native/abi-bindings.json", abiMapping)
			stringAllowed := map[int64]bool{}
			for j := 0; j < s.MIR2Module.Strings.Len(); j++ {
				p, ok := a.Symbols[fmt.Sprintf("_mir2_str_%d", j)]
				if !ok {
					t.Fatal("string symbol", j)
				}
				for k := 0; k <= len(s.MIR2Module.Strings.At(j)); k++ {
					stringAllowed[int64(p+k)] = true
				}
			}
			if scenario.Module == "abap_screen" {
				for n, size := range map[string]int{"buf_material": 12, "buf_plant": 8} {
					base := a.Symbols[n]
					for k := 0; k < size; k++ {
						stringAllowed[int64(base+k)] = true
					}
				}
			}
			host, check := newHost(mode+"/native", func(p int64) byte {
				if !stringAllowed[p] {
					t.Fatal("native CString foreign object read", p)
				}
				if p < 0 || p >= 65536 {
					t.Fatal("native CString bound", p)
				}
				return mem[p]
			}, func(p int64, b []byte) {
				if p != int64(a.Symbols["buf_material"]) || len(b) > 11 {
					t.Fatal("native line write bound")
				}
				copy(mem[p:], b)
			})
			param := func(p mir2.Param, r emulator.Registers) int64 {
				loc := assertParamLocation(p, s.Allocation)
				var v uint16
				switch loc.Name {
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
				case "BC":
					v = r.BC
				case "DE":
					v = r.DE
				case "HL":
					v = r.HL
				case "IX":
					v = r.IX
				case "IY":
					v = r.IY
				default:
					t.Fatal("unsupported externhome", loc)
				}
				if p.Ty.Width() <= 8 {
					return int64(byte(v))
				}
				return int64(v)
			}
			allowed := make([]bool, 65536)
			globals := map[string]int{"scr_title": 2, "scr_nfields": 1, "scr_focus": 1, "val_count": 2, "buf_material": 12, "buf_plant": 8}
			if scenario.Module == "tui_screen" {
				globals = map[string]int{}
			}
			for n, w := range globals {
				p := a.Symbols[n]
				for j := 0; j < w; j++ {
					allowed[p+j] = true
				}
			}
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					for j := range l.Bytes {
						allowed[l.Address+j] = true
					}
				}
			}
			z.SetSMCTracker(func(p uint16, old, new byte) {
				if !(p >= 0xe000 && p < 0xeff0) && !allowed[p] {
					t.Fatalf("%s forbiddenwrite%x", mode, p)
				}
			})
			z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xeff0, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			terminal := uint16(0x107)
			if scenario.Module == "abap_screen" {
				terminal = 0
				lo := a.Symbols["main"]
				hi := a.Symbols["tui_clear"]
				for _, l := range a.Listing {
					if l.Address >= lo && l.Address < hi && len(l.Bytes) == 1 && l.Bytes[0] == 0x76 {
						if terminal != 0 {
							t.Fatal("multiple main HALTs")
						}
						terminal = uint16(l.Address)
					}
				}
				if terminal == 0 {
					t.Fatal("missing original main HALT")
				}
			}
			minSP := uint16(0xeff0)
			done := false
			for step := 0; step < 2000000; step++ {
				pc := z.GetPC()
				if f, ok := externs[pc]; ok {
					r := z.GetRegisters()
					if r.SP < 0xe000 || r.SP+1 >= 0xeff0 {
						t.Fatal("extern stack", f.Name, r.SP)
					}
					ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					if ret < 0x100 || int(ret) >= 0x100+len(a.Binary) {
						t.Fatal("extern return fetch", f.Name, ret)
					}
					args := []int64{}
					for _, p := range f.Contract.Params {
						args = append(args, param(p, r))
					}
					v, e := host(f.Name, args)
					if e != nil {
						t.Fatal(mode, e)
					}
					if len(v) > 0 {
						if len(f.Contract.Returns) != 1 || f.Contract.Returns[0].Ty.Width() != 8 {
							t.Fatal("externreturn", f.Name)
						}
						r.A = byte(v[0].I)
					}
					r.PC = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if pc < 0x100 || int(pc) >= 0x100+len(a.Binary) {
					t.Fatal("foreignfetch", pc)
				}
				if pc == terminal {
					if mem[pc] != 0x76 {
						t.Fatal("loader altered")
					}
					z.Step()
					done = true
					break
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xe000 || z.GetSP() > 0xeff0 {
					t.Fatal("stack", z.GetRegisters())
				}
			}
			if !done || z.GetSP() != 0xeff0 {
				t.Fatal("native return/SP", mode, z.GetRegisters())
			}
			check()
			copy(want[minSP:0xeff0], mem[minSP:0xeff0])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					for j := range l.Bytes {
						want[l.Address+j] = mem[l.Address+j]
					}
				}
			}
			if !bytes.Equal(want, mem) {
				for p, b := range mem {
					if b != want[p] {
						t.Fatalf("%s nativeRAM%x got%d want%d", mode, p, b, want[p])
					}
				}
			}
			save(mode+"/native/summary.json", map[string]any{"result": "PASS", "scope": "whole source main explicit native terminal linker externbindings", "events": len(expected), "stdout_bytes": len(stdout), "image_bytes": len(a.Binary), "minimum_sp": minSP, "full_ram": true, "unlinked_asm_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(callerAssembly))), "linked_asm_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(asm))), "case": caseName, "terminal_pc": terminal, "source_sha256": freeze.Sources["examples/nanz/"+scenario.Module+".nanz"]})
			t.Log(mode, "raw/finalVM+native wholemain PASS")
		}
	}
}
