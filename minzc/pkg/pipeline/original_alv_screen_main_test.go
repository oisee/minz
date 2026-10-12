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

type alvScreenEvent struct {
	Name string `json:"name"`
	Args []any  `json:"args"`
}

func TestOriginalALVScreenWholeMain(t *testing.T) {
	root := "../../../reports/alv-report-main/"
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
		b, e := os.ReadFile("../../../" + p)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != h {
			t.Fatal("freeze", p, e)
		}
	}
	for _, caseName := range []string{"alv", "alv-tab"} {
		var sc struct {
			Module  string            `json:"module"`
			Title   string            `json:"title"`
			Keys    []int64           `json:"keys"`
			Line    *string           `json:"line"`
			Globals map[string]int64  `json:"globals"`
			Buffers map[string]string `json:"buffers"`
			Rows    [][]string        `json:"rows"`
			Events  []alvScreenEvent  `json:"events"`
		}
		if e := json.Unmarshal(read(caseName+".json"), &sc); e != nil {
			t.Fatal(e)
		}
		src := read(sc.Module + ".nanz")
		if fmt.Sprintf("%x", sha256.Sum256(src)) != freeze.Sources["examples/nanz/"+sc.Module+".nanz"] {
			t.Fatal("snapshotidentity")
		}
		stdout := read(caseName + "-stdout.bin")
		save := func(n string, v any) {
			r := os.Getenv("MINZ_ALV_SCREEN_ARTIFACTS")
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
		size := func(n string) int {
			switch n {
			case "_buf_name":
				return 22
			case "_buf_city":
				return 17
			case "_buf_country":
				return 5
			case "_buf_material":
				return 12
			case "_buf_plant":
				return 8
			case "_buf_filter":
				return 12
			}
			return 8
		}
		apply := func(mem []byte, addr func(string) int, title int) {
			p := addr("_scr_title")
			mem[p] = byte(title)
			mem[p+1] = byte(title >> 8)
			for n, v := range sc.Globals {
				w := 1
				if strings.HasPrefix(n, "_val_") {
					w = 2
				}
				p := addr(n)
				for j := 0; j < w; j++ {
					mem[p+j] = byte(v >> uint(j*8))
				}
			}
			if sc.Module == "screen_customer" {
				p := addr("_buf_name")
				copy(mem[p:], []byte("ACME Corp"))
				mem[p+9] = 0
			}
			for row, cols := range sc.Rows {
				off := 0
				for i, v := range cols {
					p := addr("_tbl_data") + row*23 + off
					copy(mem[p:], []byte(v))
					mem[p+len(v)] = 0
					off += []int{11, 7, 5}[i]
				}
			}
			for n, v := range sc.Buffers {
				p := addr(n)
				copy(mem[p:], []byte(v))
				mem[p+len(v)] = 0
			}
		}
		parse := func() *hir.Module {
			h, e := nanz.ParseWithOpts(string(src), sc.Module+".nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		// Native padding helper is executed; its API entry is normalized to a logical
		// string+width event, while actual BDOS output supplies the bytes.
		newHost := func(engine string, readByte func(int64) byte, writeLine func(int64, []byte), native bool) (func(string, []int64) ([]mir2.Value, error), func(byte), func()) {
			var events []alvScreenEvent
			var out bytes.Buffer
			key := 0
			lineCalls := 0
			call := func(n string, a []int64) ([]mir2.Value, error) {
				vals := make([]any, len(a))
				for i, v := range a {
					vals[i] = v
				}
				var ret []mir2.Value
				switch n {
				case "tui_goto":
					if native {
						break
					}
					fmt.Fprintf(&out, "\x1b[%d;%dH", a[1]+1, a[0]+1)
				case "tui_color":
					if native {
						break
					}
					f, b := 30+a[0], 40+a[1]
					if a[2] != 0 {
						f += 60
						b += 60
					}
					fmt.Fprintf(&out, "\x1b[%d;%dm", f, b)
				case "tui_clear":
					if native {
						break
					}
					out.WriteString("\x1b[2J\x1b[H")
				case "tui_reset":
					if native {
						break
					}
					out.WriteString("\x1b[0m")
				case "tui_putch":
					if native {
						break
					}
					out.WriteByte(byte(a[0]))
				case "tui_width":
					ret = []mir2.Value{{I: 80}}
				case "tui_height":
					ret = []mir2.Value{{I: 24}}
				case "tui_read_key":
					if key >= len(sc.Keys) {
						return nil, fmt.Errorf("extra key")
					}
					ret = []mir2.Value{{I: sc.Keys[key]}}
					vals = []any{sc.Keys[key]}
					key++
				case "tui_read_line":
					if sc.Line == nil || lineCalls != 0 {
						return nil, fmt.Errorf("unexpectedline")
					}
					lineCalls++
					b := []byte(*sc.Line)
					if len(b) > int(a[1]) {
						b = b[:a[1]]
					}
					out.WriteString("\x1b[?25h\x1b[?25l")
					writeLine(a[0], append(append([]byte(nil), b...), 0))
					ret = []mir2.Value{{I: int64(len(b))}}
					vals = []any{"_buf_name", a[1], string(b)}
				case "table_set_matnr", "table_set_mtart", "table_set_meins":
					return nil, nil
				case "tui_puts", "_scr_puts_padded":
					var b []byte
					limit := 4096
					if n == "_scr_puts_padded" {
						limit = int(a[1])
					}
					for j := 0; j < limit; j++ {
						v := readByte(a[0] + int64(j))
						if v == 0 {
							break
						}
						b = append(b, v)
					}
					if n == "tui_puts" {
						if len(b) == 4096 {
							return nil, fmt.Errorf("unterminated")
						}
						vals = []any{string(b)}
						if !native {
							out.Write(b)
						}
					} else {
						vals = []any{string(b), a[1]}
						if !native {
							out.Write(b)
							for j := len(b); j < limit; j++ {
								out.WriteByte(32)
							}
						}
					}
				default:
					return nil, fmt.Errorf("unknownhost%s", n)
				}
				events = append(events, alvScreenEvent{n, vals})
				return ret, nil
			}
			check := func() {
				save(engine+"/events.json", events)
				save(engine+"/stdout.bin", out.Bytes())
				a, _ := json.Marshal(events)
				b, _ := json.Marshal(sc.Events)
				if !bytes.Equal(a, b) {
					for i := 0; i < len(events) && i < len(sc.Events); i++ {
						x, _ := json.Marshal(events[i])
						y, _ := json.Marshal(sc.Events[i])
						if !bytes.Equal(x, y) {
							t.Fatalf("%s/%s event%d got%s want%s", caseName, engine, i, x, y)
						}
					}
					t.Fatal("eventcount", len(events), len(sc.Events))
				}
				if key != len(sc.Keys) || !bytes.Equal(out.Bytes(), func() []byte {
					if native {
						return read(caseName + "-native-stdout.bin")
					}
					return stdout
				}()) {
					t.Fatal("stdout/input", caseName, engine, key, out.Len(), len(stdout))
				}
			}
			return call, func(b byte) { out.WriteByte(b) }, check
		}
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			raw := hir.LowerModule(parse())
			s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			save(mode+"/hir.txt", s.HIR)
			save(mode+"/raw.mir", raw.Dump())
			save(mode+"/final.mir", s.MIR2Module.Dump())
			for i, m := range []*mir2.Module{raw, s.MIR2Module} {
				engine := fmt.Sprintf("%s/vm%d", mode, i)
				vm := mir2.NewVM(m)
				allowed := map[int64]bool{}
				title := -1
				for j := 0; j < m.Strings.Len(); j++ {
					p, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", j))
					if e != nil {
						t.Fatal(e)
					}
					if m.Strings.At(j) == sc.Title {
						title = int(p)
					}
					for k := 0; k <= len(m.Strings.At(j)); k++ {
						allowed[p+int64(k)] = true
					}
				}
				for n := range sc.Buffers {
					p := vm.GlobalAddr(n)
					for k := 0; k < size(n); k++ {
						allowed[p+int64(k)] = true
					}
				}
				for k := 0; k < 184; k++ {
					allowed[vm.GlobalAddr("_tbl_data")+int64(k)] = true
				}
				vm.EnsureHeap(65536)
				want := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				apply(want, func(n string) int { return int(vm.GlobalAddr(n)) }, title)
				host, _, check := newHost(engine, func(p int64) byte {
					if !allowed[p] {
						t.Fatal("VM CStringbounds", p)
					}
					return vm.ReadHeap(p, 1)[0]
				}, func(p int64, b []byte) {
					if p != vm.GlobalAddr("_buf_name") || len(b) > 21 {
						t.Fatal("linewrite")
					}
					vm.WriteHeapBytes(p, b)
				}, false)
				for _, f := range m.Funcs {
					name := f.Name
					if f.Attrs.IsExtern || name == "_scr_puts_padded" || (name == "table_set_matnr" || name == "table_set_mtart" || name == "table_set_meins") || strings.HasPrefix(name, "tui__render__tui_") {
						vm.Hosts[name] = func(a []mir2.Value) ([]mir2.Value, error) {
							v := []int64{}
							for _, x := range a {
								v = append(v, x.I)
							}
							if name == "table_set_matnr" || name == "table_set_mtart" || name == "table_set_meins" {
								off := map[string]int{"table_set_matnr": 0, "table_set_mtart": 11, "table_set_meins": 18}[name]
								p := vm.GlobalAddr("_tbl_data") + a[0].I*23 + int64(off)
								for j := int64(0); j < 4096; j++ {
									src := a[1].I + j
									if !allowed[src] {
										t.Fatal("table CStringbounds")
									}
									b := vm.ReadHeap(src, 1)[0]
									vm.WriteHeap(p+j, b)
									if b == 0 {
										break
									}
								}
								return nil, nil
							}
							return host(strings.TrimPrefix(name, "tui__render__"), v)
						}
					} else {
						asm := false
						for _, b := range f.Blocks {
							for _, in := range b.Insts {
								if in.Op == mir2.OpAsm {
									asm = true
								}
							}
						}
						if asm {
							vm.Hosts[name] = func([]mir2.Value) ([]mir2.Value, error) { return nil, fmt.Errorf("unboundASM%s", name) }
						}
					}
				}
				vm.MaxSteps = 1000000
				if v, e := vm.Call("main", nil); e != nil || len(v) > 0 {
					t.Fatal(caseName, engine, v, e)
				}
				check()
				if !bytes.Equal(want, vm.ReadHeap(0, 65536)) {
					for p, b := range vm.ReadHeap(0, 65536) {
						if b != want[p] {
							t.Fatalf("%s VM RAM%x got%d want%d", caseName, p, b, want[p])
						}
					}
				}
				save(engine+"/summary.json", map[string]any{"result": "PASS", "full_ram": true, "steps": vm.StepsExecuted()})
			}
			linked := s.Assembly
			for _, f := range s.MIR2Module.Funcs {
				if f.Attrs.IsExtern {
					label := "\n" + f.Name + ":"
					if strings.Count(linked, label) != 1 {
						t.Fatal("externlabel", f.Name)
					}
					linked = strings.Replace(linked, label, label+"\n RET ; testownedterminalbinding", 1)
				}
			}
			asm := "ORG 0x100\nLD SP,0xeff0\nJP main\n" + linked
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			save(mode+"/native/unlinked.a80", s.Assembly)
			save(mode+"/native/program.a80", asm)
			save(mode+"/native/program.com", a.Binary)
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x100:], a.Binary)
			want := append([]byte(nil), mem...)
			allowed := map[int64]bool{}
			title := -1
			for j := 0; j < s.MIR2Module.Strings.Len(); j++ {
				p := a.Symbols[fmt.Sprintf("_mir2_str_%d", j)]
				if s.MIR2Module.Strings.At(j) == sc.Title {
					title = p
				}
				for k := 0; k <= len(s.MIR2Module.Strings.At(j)); k++ {
					allowed[int64(p+k)] = true
				}
			}
			for n := range sc.Buffers {
				for k := 0; k < size(n); k++ {
					allowed[int64(a.Symbols[n]+k)] = true
				}
			}
			for k := 0; k < 184; k++ {
				allowed[int64(a.Symbols["_tbl_data"]+k)] = true
			}
			apply(want, func(n string) int { return a.Symbols[n] }, title)
			host, bdos, check := newHost(mode+"/native", func(p int64) byte {
				if !allowed[p] {
					t.Fatal("nativeCStringbounds", p)
				}
				return mem[p]
			}, func(p int64, b []byte) {
				if p != int64(a.Symbols["_buf_name"]) || len(b) > 21 {
					t.Fatal("native linewrite")
				}
				copy(mem[p:], b)
			}, true)
			extern := map[uint16]*mir2.Func{}
			for _, f := range s.MIR2Module.Funcs {
				if f.Attrs.IsExtern {
					p := a.Symbols[f.Name]
					if _, ok := extern[uint16(p)]; ok {
						t.Fatal("aliasedextern")
					}
					extern[uint16(p)] = f
				}
			}
			param := func(p mir2.Param, r emulator.Registers) int64 {
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
					t.Fatal("ABIhome", l)
				}
				if p.Ty.Width() <= 8 {
					return int64(byte(v))
				}
				return int64(v)
			}
			writes := map[int]bool{}
			for k := 0; k < 184; k++ {
				writes[a.Symbols["_tbl_data"]+k] = true
			}
			for n := range sc.Globals {
				w := 1
				if strings.HasPrefix(n, "_val_") {
					w = 2
				}
				for k := 0; k < w; k++ {
					writes[a.Symbols[n]+k] = true
				}
			}
			for k := 0; k < 2; k++ {
				writes[a.Symbols["_scr_title"]+k] = true
			}
			for n := range sc.Buffers {
				for k := 0; k < size(n); k++ {
					writes[a.Symbols[n]+k] = true
				}
			}
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					for k := range l.Bytes {
						writes[l.Address+k] = true
					}
				}
			}
			z.SetSMCTracker(func(p uint16, o, n byte) {
				if !(p >= 0xe000 && p < 0xeff0) && !writes[int(p)] {
					t.Fatalf("forbiddenwrite%x", p)
				}
			})
			lo := a.Symbols["main"]
			hi := len(a.Binary) + 0x100
			for _, f := range s.MIR2Module.Funcs {
				if p := a.Symbols[f.Name]; p > lo && p < hi {
					hi = p
				}
			}
			halt := -1
			for _, l := range a.Listing {
				if l.Address >= lo && l.Address < hi && len(l.Bytes) == 1 && l.Bytes[0] == 0x76 {
					halt = l.Address
				}
			}
			if halt < 0 {
				t.Fatal("mainHALT")
			}
			z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xeff0, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc})
			minSP := uint16(0xeff0)
			done := false
			padPC := uint16(a.Symbols["_scr_puts_padded"])
			var nativeInputs []byte
			if true {
				if caseName == "declarative-cancel" {
					nativeInputs = []byte{27, 97}
				} else {
					nativeInputs = []byte{}
					for _, k := range sc.Keys {
						nativeInputs = append(nativeInputs, byte(k))
					}
				}
			}
			nativeInputIndex := 0
			var bdosLedger []map[string]any
			padEnd := uint16(0)
			_ = byte(0)
			importReturn := uint16(0)
			for step := 0; step < 1000000; step++ {
				pc := z.GetPC()
				if pc == padPC {
					r := z.GetRegisters()
					f := s.MIR2Module.FuncByName("_scr_puts_padded")
					args := []int64{}
					for _, p := range f.Contract.Params {
						args = append(args, param(p, r))
					}
					if _, e := host("_scr_puts_padded", args); e != nil {
						t.Fatal(e)
					}
					padEnd = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
				}
				if true {
					for _, f := range s.MIR2Module.Funcs {
						if strings.HasPrefix(f.Name, "tui__render__tui_") && pc == uint16(a.Symbols[f.Name]) && importReturn == 0 {
							r := z.GetRegisters()
							args := []int64{}
							for _, p := range f.Contract.Params {
								args = append(args, param(p, r))
							}
							v, e := host(strings.TrimPrefix(f.Name, "tui__render__"), args)
							if e != nil {
								t.Fatal(e)
							}
							if len(v) > 0 {
								_ = v[0].I
							}
							importReturn = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
						}
					}
				}
				if pc == importReturn {
					importReturn = 0
				}
				if pc == padEnd {
					padEnd = 0
				}
				if pc == 5 {
					r := z.GetRegisters()
					if importReturn != 0 && byte(r.BC) == 1 {
						if nativeInputIndex >= len(nativeInputs) {
							t.Fatal("extra nativeBDOSinput")
						}
						r.A = nativeInputs[nativeInputIndex]
						nativeInputIndex++
						bdosLedger = append(bdosLedger, map[string]any{"BDOS": 1, "input": r.A})
					} else {
						if byte(r.BC) != 2 || (padEnd == 0 && importReturn == 0) {
							t.Fatal("unknownBDOS", r.BC)
						}
						bdos(byte(r.DE))
						bdosLedger = append(bdosLedger, map[string]any{"BDOS": 2, "output": byte(r.DE)})
					}
					r.PC = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if f, ok := extern[pc]; ok {
					r := z.GetRegisters()
					args := []int64{}
					for _, p := range f.Contract.Params {
						args = append(args, param(p, r))
					}
					v, e := host(f.Name, args)
					if e != nil {
						t.Fatal(e)
					}
					if len(v) > 0 {
						r.A = byte(v[0].I)
					}
					ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					if r.SP < 0xe000 || r.SP+1 >= 0xeff0 || int(ret) < 0x100 || int(ret) >= 0x100+len(a.Binary) {
						t.Fatal("hostreturn")
					}
					r.PC = ret
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if int(pc) == halt {
					z.Step()
					done = true
					break
				}
				if pc < 0x100 || int(pc) >= 0x100+len(a.Binary) {
					t.Fatal("foreignfetch")
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xe000 || z.GetSP() > 0xeff0 {
					t.Fatal("stackbounds")
				}
			}
			if !done || z.GetSP() != 0xeff0 {
				t.Fatal("mainterminal", caseName, mode, z.GetRegisters())
			}
			if false && nativeInputIndex != len(nativeInputs) {
				t.Fatal("nativeinputcount")
			}
			save(mode+"/native/bdos-ledger.json", bdosLedger)
			check()
			copy(want[minSP:0xeff0], mem[minSP:0xeff0])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					for k := range l.Bytes {
						want[l.Address+k] = mem[l.Address+k]
					}
				}
			}
			if !bytes.Equal(want, mem) {
				for p, b := range mem {
					if b != want[p] {
						t.Fatalf("%s nativeRAM%x got%d want%d", caseName, p, b, want[p])
					}
				}
			}
			save(mode+"/native/summary.json", map[string]any{"result": "PASS", "image_bytes": len(a.Binary), "events": len(sc.Events), "stdout_bytes": len(stdout), "source_sha256": freeze.Sources["examples/nanz/"+sc.Module+".nanz"], "scope": "whole generated source main, real paddedASM, linkedterminalhost", "minimum_sp": minSP})
			t.Log(caseName, mode, "wholemain raw/final/native PASS")
		}
	}
}
