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
	stringsPkg "strings"
	"testing"
)

func TestOriginalSelfParserWholeMainVM(t *testing.T) {
	const root = "../../../reports/self-parser-whole-main/frozen/"
	source, e := os.ReadFile("../../../examples/nanz/self_parser.nanz")
	if e != nil {
		t.Fatal(e)
	}
	original, e := os.ReadFile(root + "source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	expectedFixed := bytes.ReplaceAll(original, []byte(`out_str("`), []byte(`out_str(c"`))
	if !bytes.Equal(source, expectedFixed) {
		t.Fatal("exact source CString correction")
	}
	input, _ := os.ReadFile(root + "tiny.nanz")
	_, _ = os.ReadFile(root + "tiny.lanz")
	stdout, _ := os.ReadFile(root + "stdout.txt")
	asts, _ := os.ReadFile(root + "ast.bin")
	strings, _ := os.ReadFile(root + "strings.bin")
	tokens, _ := os.ReadFile(root + "tokens.bin")
	var f map[string]json.RawMessage
	b, _ := os.ReadFile(root + "freeze.json")
	json.Unmarshal(b, &f)
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(source), "self_parser.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, mode := range []string{"raw", "ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			var m *mir2.Module
			if mode == "raw" {
				m = hir.LowerModule(parse())
			} else {
				o := Options{ContractOpt: true, AssertMode: "none"}
				if mode != "ordinary" {
					o.UseGrace = true
					o.UseHIRGrace = true
					o.OptSize = mode == "grace-size"
				}
				s, e := CompileHIRSteps(parse(), o)
				if e != nil {
					t.Fatal(e)
				}
				m = s.MIR2Module
			}
			vm := mir2.NewVM(m)
			vm.MaxSteps = 10000000
			vm.EnsureHeap(0xe000)
			vm.WriteHeapBytes(0x8000, bytes.Repeat([]byte{0xd7}, 4096))
			vm.WriteHeapBytes(0x9000, bytes.Repeat([]byte{0xa5}, 8192))
			vm.WriteHeapBytes(0xb000, bytes.Repeat([]byte{0xc3}, 4096))
			vm.WriteHeapBytes(0xc000, bytes.Repeat([]byte{0xe1}, 4096))
			vm.WriteHeapBytes(0xd000, bytes.Repeat([]byte{0x91}, 4096))
			var out bytes.Buffer
			var fileEvents []string
			var written []byte
			var writes []uint16
			var memoryEvents []map[string]any
			var hostReadBefore, hostWriteBefore []byte
			readPath := func(ptr int64) string {
				b := vm.ReadHeap(ptr, 1)
				if b[0] == 9 {
					return string(vm.ReadHeap(ptr+1, 9))
				}
				t.Fatal("path SString", ptr, b)
				return ""
			}
			vm.Hosts["peek"] = func(a []mir2.Value) ([]mir2.Value, error) {
				p := uint16(a[0].I)
				if p < 0x8000 {
					t.Fatal("peek outside arenas", p)
				}
				memoryEvents = append(memoryEvents, map[string]any{"kind": "peek", "address": p, "value": vm.ReadHeap(int64(p), 1)})
				vm.EnsureHeap(int64(p) + 1)
				return []mir2.Value{{I: int64(vm.ReadHeap(int64(p), 1)[0])}}, nil
			}
			vm.Hosts["poke"] = func(a []mir2.Value) ([]mir2.Value, error) {
				p := uint16(a[0].I)
				if p < 0x8000 || p >= 0xe000 {
					t.Fatal("poke outside arenas", p)
				}
				memoryEvents = append(memoryEvents, map[string]any{"kind": "poke", "address": p, "value": byte(a[1].I)})
				writes = append(writes, p)
				vm.WriteHeap(int64(p), byte(a[1].I))
				return nil, nil
			}
			vm.Hosts["file_read"] = func(a []mir2.Value) ([]mir2.Value, error) {
				p := readPath(a[0].I)
				if p != "tiny.nanz" || a[1].I != 0xc000 {
					t.Fatal("read args", p, a)
				}
				hostReadBefore = append([]byte(nil), vm.ReadHeap(0, 0xe000)...)
				vm.WriteHeapBytes(0xc000, append(append([]byte(nil), input...), 0))
				fileEvents = append(fileEvents, "read tiny.nanz")
				return []mir2.Value{{I: int64(len(input))}}, nil
			}
			vm.Hosts["file_write"] = func(a []mir2.Value) ([]mir2.Value, error) {
				p := readPath(a[0].I)
				if p != "tiny.lanz" || a[1].I != 0xd000 || a[2].I > 4096 {
					t.Fatal("write args/data", p, a, string(vm.ReadHeap(0xd000, int(a[2].I))))
				}
				hostWriteBefore = append([]byte(nil), vm.ReadHeap(0, 0xe000)...)
				written = append([]byte(nil), vm.ReadHeap(0xd000, int(a[2].I))...)
				fileEvents = append(fileEvents, "write tiny.lanz")
				return []mir2.Value{{I: a[2].I}}, nil
			}
			vm.Hosts["file_size"] = func(a []mir2.Value) ([]mir2.Value, error) { t.Fatal("unreachable file_size", a); return nil, nil }
			vm.Hosts["@mir.io.print.u8"] = func(a []mir2.Value) ([]mir2.Value, error) { fmt.Fprintf(&out, "%d", byte(a[0].I)); return nil, nil }
			vm.Hosts["@mir.io.print.char"] = func(a []mir2.Value) ([]mir2.Value, error) { out.WriteByte(byte(a[0].I)); return nil, nil }
			vm.Hosts["@mir.io.print.nl"] = func(a []mir2.Value) ([]mir2.Value, error) { out.WriteByte(10); return nil, nil }
			vm.Hosts["@mir.io.print.str"] = func(a []mir2.Value) ([]mir2.Value, error) {
				p := a[0].I
				length := int(vm.ReadHeap(p, 1)[0])
				out.Write(vm.ReadHeap(p+1, length))
				return nil, nil
			}
			beforeVM := append([]byte(nil), vm.ReadHeap(0, 0xe000)...)
			_, e := vm.Call("main", nil)
			if dir := os.Getenv("MINZ_SELF_PARSER_MAIN_EXPORT"); dir != "" {
				os.MkdirAll(dir+"/"+mode, 0755)
				os.WriteFile(dir+"/"+mode+"/stdout.txt", out.Bytes(), 0644)
				os.WriteFile(dir+"/"+mode+"/whole.mir", []byte(m.Dump()), 0644)
				os.WriteFile(dir+"/"+mode+"/heap.bin", vm.ReadHeap(0, 0xe000), 0644)
				b, _ := json.MarshalIndent(memoryEvents, "", "  ")
				os.WriteFile(dir+"/"+mode+"/memory-events.json", b, 0644)
			}
			wantFile, _ := os.ReadFile(root + "tiny.lanz")
			if !bytes.Equal(written, wantFile) {
				t.Fatalf("Lanz file got%q want%q", written, wantFile)
			}
			if e != nil || !bytes.Equal(out.Bytes(), stdout) {
				t.Fatal("whole main", e, out.String(), string(stdout))
			}
			for _, x := range []struct {
				addr int64
				want []byte
			}{{0x8000, asts}, {0x9000, strings}, {0xb000, tokens}} {
				got := vm.ReadHeap(x.addr, len(x.want))
				if !bytes.Equal(got, x.want) {
					for i, v := range got {
						if v != x.want[i] {
							t.Fatalf("state %04x+%d got%02x want%02x", x.addr, i, v, x.want[i])
						}
					}
				}
			}
			for name, key := range map[string]string{"str_bump": "str_bump", "ast_bump": "ast_bump", "tok_count": "tok_count", "parse_pos": "parse_pos", "parse_error": "parse_error", "pos": "pos", "tok_start": "tok_start", "out_len": "out_len"} {
				var want uint16
				json.Unmarshal(f[key], &want)
				addr := vm.GlobalAddr(name)
				d := vm.ReadHeap(addr, 1)
				got := uint16(d[0])
				if name != "parse_error" {
					d = vm.ReadHeap(addr, 2)
					got |= uint16(d[1]) << 8
				}
				if got != want {
					t.Fatal("global", name, got, want)
				}
			}
			if fmt.Sprint(fileEvents) != "[read tiny.nanz write tiny.lanz]" {
				t.Fatal("file event sequence", fileEvents)
			}
			if len(hostReadBefore) == 0 || len(hostWriteBefore) == 0 {
				t.Fatal("file memory snapshots")
			}
			for _, p := range writes {
				if !(p >= 0x8000 && p < 0x8060 || p >= 0x9000 && p < 0x9037 || p >= 0xb000 && p < 0xb074 || p >= 0xd000 && p < 0xd04c) {
					t.Fatal("poke outside exactusedranges", p)
				}
			}
			expectedVM := append([]byte(nil), beforeVM...)
			copy(expectedVM[0x8000:], asts)
			copy(expectedVM[0x9000:], strings)
			copy(expectedVM[0xb000:], tokens)
			copy(expectedVM[0xc000:], append(append([]byte(nil), input...), 0))
			copy(expectedVM[0xd000:], wantFile)
			var interns map[string]uint16
			json.Unmarshal(f["interns"], &interns)
			globalWant := map[string]uint16{"src_len": uint16(len(input)), "str_bump": 0x9037, "ast_bump": 0x8060, "tok_count": 29, "parse_pos": 29, "parse_error": 0, "pos": 76, "tok_start": 76, "out_len": 76}
			for _, kw := range []string{"fn", "if", "fun", "let", "var", "while", "return"} {
				globalWant["KW_"+stringsPkg.ToUpper(kw)] = interns[kw]
			}
			globalWant["KW_ELSE"] = interns["els"]
			for name, want := range globalWant {
				addr := vm.GlobalAddr(name)
				if addr < 0 {
					t.Fatal("missingglobal", name)
				}
				expectedVM[addr] = byte(want)
				if name != "parse_error" {
					expectedVM[addr+1] = byte(want >> 8)
				}
			}
			if !bytes.Equal(vm.ReadHeap(0, 0xe000), expectedVM) {
				for i, v := range expectedVM {
					if vm.ReadHeap(int64(i), 1)[0] != v {
						t.Fatal("wholeVMRAM", i, vm.ReadHeap(int64(i), 1), v)
					}
				}
			}
			t.Logf("wholemain independent files/AST/tokens/stdout PASS inputSHA %x events%v", sha256.Sum256(input), fileEvents)
		})
	}
}

func TestOriginalSelfParserWholeMainNative(t *testing.T) {
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) { selfParserWholeMainNative(t, mode) })
	}
}
func selfParserWholeMainNative(t *testing.T, mode string) {
	const root = "../../../reports/self-parser-whole-main/frozen/"
	src, _ := os.ReadFile("../../../examples/nanz/self_parser.nanz")
	input, _ := os.ReadFile(root + "tiny.nanz")
	h, e := nanz.Parse(string(src), "self_parser.nanz")
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{ContractOpt: true, AssertMode: "none"}
	if mode != "ordinary" {
		opts.UseGrace = true
		opts.UseHIRGrace = true
		opts.OptSize = mode == "grace-size"
	}
	s, e := CompileHIRSteps(h, opts)
	if e != nil {
		t.Fatal(e)
	}
	linked := s.Assembly
	for _, n := range []string{"peek", "poke", "file_read", "file_write", "file_size"} {
		needle := n + ":\n"
		if stringsPkg.Count(linked, needle) != 1 {
			t.Fatal("extern label", n)
		}
		linked = stringsPkg.Replace(linked, needle, needle+" RET\n", 1)
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x400\n" + linked)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	z.LoadMemory(0x400, a.Binary)
	for _, r := range []struct {
		base uint16
		n    int
		v    byte
	}{{0x8000, 4096, 0xd7}, {0x9000, 8192, 0xa5}, {0xb000, 4096, 0xc3}, {0xc000, 4096, 0xe1}, {0xd000, 4096, 0x91}} {
		z.LoadMemory(r.base, bytes.Repeat([]byte{r.v}, r.n))
	}
	target := a.Symbols["main"]
	z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
	z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
	var stdout, file []byte
	z.SetIOHandlers(nil, func(p uint16, v byte) {
		if byte(p) != 0x23 {
			t.Fatal("port", p)
		}
		stdout = append(stdout, v)
	})
	readArg := func(name string, i int) uint16 {
		p := s.MIR2Module.FuncByName(name).Contract.Params[i]
		loc := assertParamLocation(p, s.Allocation)
		r := z.GetRegisters()
		switch loc.Name {
		case "HL":
			return r.HL
		case "DE":
			return r.DE
		case "BC":
			return r.BC
		case "A":
			return uint16(r.A)
		case "C":
			return r.BC & 255
		}
		t.Fatal("formalhome", name, i, loc)
		return 0
	}
	setRet := func(name string, v uint16) {
		r := z.GetRegisters()
		switch s.MIR2Module.FuncByName(name).Contract.Returns[0].Class {
		case mir2.ClassPointer, mir2.ClassAcc:
			r.HL = v
		default:
			t.Fatal("rethome", name)
		}
		z.SetRegisters(r)
	}
	ret := func() {
		r := z.GetRegisters()
		r.PC = uint16(z.Memory()[r.SP]) | uint16(z.Memory()[r.SP+1])<<8
		r.SP += 2
		z.SetRegisters(r)
	}
	before := append([]byte(nil), z.Memory()...)
	mutable := map[int]bool{}
	for _, g := range s.MIR2Module.Globals {
		if !g.IsConst {
			base := a.Symbols[g.Name]
			for i := 0; i < mir2.ByteWidth(g.Ty); i++ {
				mutable[base+i] = true
			}
		}
	}
	for _, l := range a.Listing {
		if stringsPkg.HasPrefix(stringsPkg.ToLower(l.Label), "_spill_") {
			for i := range l.Bytes {
				mutable[l.Address+i] = true
			}
		}
	}
	var events []map[string]any
	minSP := uint16(0xf000)
	done := false
	for i := 0; i < 3000000; i++ {
		pc := z.GetPC()
		if z.IsHalted() {
			if z.GetSP() != 0xeffe {
				t.Fatal("terminalHALTstack", z.GetSP())
			}
			done = true
			break
		}
		if pc == 0x7003 && z.GetSP() == 0xf000 {
			done = true
			break
		}
		handled := false
		for _, name := range []string{"peek", "poke", "file_read", "file_write", "file_size"} {
			if pc != uint16(a.Symbols[name]) {
				continue
			}
			handled = true
			switch name {
			case "peek":
				p := readArg(name, 0)
				r := z.GetRegisters()
				if !(p >= 0x8000 && p < 0xe000 || p >= 0x400 && int(p) < 0x400+len(a.Binary)) {
					t.Fatal("peek authority", p)
				}
				events = append(events, map[string]any{"kind": "peek", "address": p, "value": z.Memory()[p]})
				r.A = z.Memory()[p]
				z.SetRegisters(r)
			case "poke":
				p := readArg(name, 0)
				v := readArg(name, 1)
				if !(p >= 0x8000 && p < 0x8060 || p >= 0x9000 && p < 0x9037 || p >= 0xb000 && p < 0xb074 || p >= 0xd000 && p < 0xd04c) {
					t.Fatal("poke exactbound", p)
				}
				events = append(events, map[string]any{"kind": "poke", "address": p, "value": byte(v)})
				z.Memory()[p] = byte(v)
			case "file_read":
				ptr := readArg(name, 0)
				p := readArg(name, 1)
				if string(z.Memory()[ptr+1:ptr+10]) != "tiny.nanz" || p != 0xc000 {
					if dir := os.Getenv("MINZ_SELF_PARSER_MAIN_EXPORT"); dir != "" {
						os.MkdirAll(dir+"/native-abi", 0755)
						os.WriteFile(dir+"/native-abi/main.a80", []byte("ORG 0x400\n"+linked), 0644)
						os.WriteFile(dir+"/native-abi/main.bin", a.Binary, 0644)
						b, _ := json.MarshalIndent(s.MIR2Module.FuncByName(name).Contract, "", "  ")
						os.WriteFile(dir+"/native-abi/contract.json", b, 0644)
					}
					t.Fatal("file_read ABI", ptr, p)
				}
				events = append(events, map[string]any{"kind": "file_read", "path": "tiny.nanz", "buffer": p, "length": len(input)})
				z.LoadMemory(p, append(append([]byte(nil), input...), 0))
				setRet(name, uint16(len(input)))
			case "file_write":
				ptr := readArg(name, 0)
				p := readArg(name, 1)
				n := readArg(name, 2)
				if string(z.Memory()[ptr+1:ptr+10]) != "tiny.lanz" || p != 0xd000 || n > 4096 {
					t.Fatal("file_writeABI", ptr, p, n)
				}
				events = append(events, map[string]any{"kind": "file_write", "path": "tiny.lanz", "buffer": p, "length": n})
				file = append([]byte(nil), z.Memory()[p:p+n]...)
				setRet(name, n)
			case "file_size":
				t.Fatal("unexpected file_size")
			}
			ret()
			break
		}
		if handled {
			continue
		}
		if pc != 0x7000 && (pc < 0x400 || int(pc) >= 0x400+len(a.Binary)) {
			t.Fatal("foreignpc", pc, z.GetSP())
		}
		z.Step()
		if z.GetSP() < minSP {
			minSP = z.GetSP()
		}
	}
	if dir := os.Getenv("MINZ_SELF_PARSER_MAIN_EXPORT"); dir != "" {
		dir += "/native-" + mode
		os.MkdirAll(dir, 0755)
		os.WriteFile(dir+"/file.lanz", file, 0644)
		os.WriteFile(dir+"/stdout.txt", stdout, 0644)
		os.WriteFile(dir+"/main.a80", []byte("ORG 0x400\n"+linked), 0644)
		os.WriteFile(dir+"/main.bin", a.Binary, 0644)
		os.WriteFile(dir+"/memory.bin", z.Memory(), 0644)
		os.WriteFile(dir+"/before.bin", before, 0644)
		b, _ := json.MarshalIndent(map[string]any{"events": events, "options": opts, "allocation": s.Allocation, "min_sp": minSP, "done": done, "cycles": z.GetCycles()}, "", "  ")
		os.WriteFile(dir+"/context.json", b, 0644)
	}
	if !done {
		t.Fatal("no return", z.GetPC(), z.GetSP(), string(stdout))
	}
	want, _ := os.ReadFile(root + "tiny.lanz")
	stdoutWant, _ := os.ReadFile(root + "stdout.txt")
	if !bytes.Equal(stdout, stdoutWant) {
		t.Fatal("native stdout", string(stdout), string(stdoutWant))
	}
	for _, x := range []struct {
		base int
		file string
	}{{0x8000, "ast.bin"}, {0x9000, "strings.bin"}, {0xb000, "tokens.bin"}} {
		d, _ := os.ReadFile(root + x.file)
		if !bytes.Equal(z.Memory()[x.base:x.base+len(d)], d) {
			t.Fatal("native arena", x.file)
		}
	}
	expected := append([]byte(nil), before...)
	for _, x := range []struct {
		base int
		file string
	}{{0x8000, "ast.bin"}, {0x9000, "strings.bin"}, {0xb000, "tokens.bin"}} {
		d, _ := os.ReadFile(root + x.file)
		copy(expected[x.base:], d)
	}
	copy(expected[0xc000:], append(append([]byte(nil), input...), 0))
	copy(expected[0xd000:], want)
	for p, v := range expected {
		if mutable[p] || p >= int(minSP) && p < 0xf000 {
			continue
		}
		if z.Memory()[p] != v {
			t.Fatal("native RAM", p, z.Memory()[p], v)
		}
	}
	var frozen map[string]json.RawMessage
	fb, _ := os.ReadFile(root + "freeze.json")
	json.Unmarshal(fb, &frozen)
	var interns map[string]uint16
	json.Unmarshal(frozen["interns"], &interns)
	wantedGlobals := map[string]uint16{"src_len": uint16(len(input)), "str_bump": 0x9037, "ast_bump": 0x8060, "tok_count": 29, "parse_pos": 29, "parse_error": 0, "pos": 76, "tok_start": 76, "out_len": 76}
	for _, kw := range []string{"fn", "if", "fun", "let", "var", "while", "return"} {
		wantedGlobals["KW_"+stringsPkg.ToUpper(kw)] = interns[kw]
	}
	wantedGlobals["KW_ELSE"] = interns["els"]
	for _, g := range s.MIR2Module.Globals {
		base, ok := a.Symbols[g.Name]
		if !ok {
			t.Fatal("global symbol", g.Name)
		}
		expected := append([]byte(nil), before[base:base+mir2.ByteWidth(g.Ty)]...)
		if v, ok := wantedGlobals[g.Name]; ok {
			expected[0] = byte(v)
			if len(expected) > 1 {
				expected[1] = byte(v >> 8)
			}
		}
		if !bytes.Equal(z.Memory()[base:base+len(expected)], expected) {
			t.Fatal("native globals", g.Name, z.Memory()[base:base+len(expected)], expected)
		}
	}
	t.Logf("whole native parser %d bytes file76/AST12/token29/stdout/SP memory PASS", len(a.Binary))
	if !bytes.Equal(file, want) {
		t.Fatalf("self parser native file %d bytes want%d %q", len(file), len(want), file)
	}
}
