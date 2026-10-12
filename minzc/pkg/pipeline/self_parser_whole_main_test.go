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
"strings"
	"testing"
)

func TestOriginalSelfParserWholeMainVM(t *testing.T) {
	const root = "../../../reports/self-parser-whole-main/frozen/"
	source, e := os.ReadFile("../../../examples/nanz/self_parser.nanz")
	if e != nil {
		t.Fatal(e)
	}
	original, e := os.ReadFile(root + "source.nanz");if e!=nil{t.Fatal(e)};expectedFixed:=bytes.ReplaceAll(original,[]byte(`out_str("`),[]byte(`out_str(c"`));if !bytes.Equal(source,expectedFixed){t.Fatal("exact source CString correction") }
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
			var fileEvents []string;var written []byte
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
				vm.EnsureHeap(int64(p) + 1)
				return []mir2.Value{{I: int64(vm.ReadHeap(int64(p), 1)[0])}}, nil
			}
			vm.Hosts["poke"] = func(a []mir2.Value) ([]mir2.Value, error) {
				p := uint16(a[0].I)
				if p < 0x8000 || p >= 0xe000 {
					t.Fatal("poke outside arenas", p)
				}
				vm.WriteHeap(int64(p), byte(a[1].I))
				return nil, nil
			}
			vm.Hosts["file_read"] = func(a []mir2.Value) ([]mir2.Value, error) {
				p := readPath(a[0].I)
				if p != "tiny.nanz" || a[1].I != 0xc000 {
					t.Fatal("read args", p, a)
				}
				vm.WriteHeapBytes(0xc000, append(append([]byte(nil), input...), 0))
				fileEvents = append(fileEvents, "read tiny.nanz")
				return []mir2.Value{{I: int64(len(input))}}, nil
			}
			vm.Hosts["file_write"] = func(a []mir2.Value) ([]mir2.Value, error) {
				p := readPath(a[0].I)
				if p != "tiny.lanz" || a[1].I != 0xd000 || a[2].I > 4096 {
					t.Fatal("write args/data", p, a, string(vm.ReadHeap(0xd000, int(a[2].I))))
				}
				written=append([]byte(nil),vm.ReadHeap(0xd000,int(a[2].I))...);fileEvents = append(fileEvents, "write tiny.lanz")
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
			_, e := vm.Call("main", nil)
			if dir := os.Getenv("MINZ_SELF_PARSER_MAIN_EXPORT"); dir != "" {
				os.MkdirAll(dir+"/"+mode, 0755)
				os.WriteFile(dir+"/"+mode+"/stdout.txt", out.Bytes(), 0644)
				os.WriteFile(dir+"/"+mode+"/whole.mir", []byte(m.Dump()), 0644)
				os.WriteFile(dir+"/"+mode+"/heap.bin", vm.ReadHeap(0, 0xe000), 0644)
			}
			wantFile,_:=os.ReadFile(root+"tiny.lanz");if !bytes.Equal(written,wantFile){t.Fatalf("Lanz file got%q want%q",written,wantFile)}
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
			t.Logf("wholemain independent files/AST/tokens/stdout PASS inputSHA %x events%v", sha256.Sum256(input), fileEvents)
		})
	}
}

func TestOriginalSelfParserNativeCounterexample(t *testing.T) {
	const root = "../../../reports/self-parser-whole-main/frozen/"
	src, _ := os.ReadFile("../../../examples/nanz/self_parser.nanz")
	input, _ := os.ReadFile(root + "tiny.nanz")
	h, e := nanz.Parse(string(src), "self_parser.nanz")
	if e != nil {
		t.Fatal(e)
	}
	s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	linked:=s.Assembly;for _,n:=range []string{"peek","poke","file_read","file_write","file_size"}{needle:=n+":\n";if strings.Count(linked,needle)!=1{t.Fatal("extern label",n)};linked=strings.Replace(linked,needle,needle+" RET\n",1)}
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
	done := false
	for i := 0; i < 3000000; i++ {
		pc := z.GetPC()
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
				r.A = z.Memory()[p]
				z.SetRegisters(r)
			case "poke":
				p := readArg(name, 0)
				v := readArg(name, 1)
				z.Memory()[p] = byte(v)
			case "file_read":
				ptr := readArg(name, 0)
				p := readArg(name, 1)
				if string(z.Memory()[ptr+1:ptr+10]) != "tiny.nanz" || p != 0xc000 {
 if dir:=os.Getenv("MINZ_SELF_PARSER_MAIN_EXPORT");dir!=""{os.MkdirAll(dir+"/native-abi",0755);os.WriteFile(dir+"/native-abi/main.a80",[]byte("ORG 0x400\n"+linked),0644);os.WriteFile(dir+"/native-abi/main.bin",a.Binary,0644);b,_:=json.MarshalIndent(s.MIR2Module.FuncByName(name).Contract,"","  ");os.WriteFile(dir+"/native-abi/contract.json",b,0644)}
					t.Fatal("file_read ABI", ptr, p)
				}
				z.LoadMemory(p, append(append([]byte(nil), input...), 0))
				setRet(name, uint16(len(input)))
			case "file_write":
				ptr := readArg(name, 0)
				p := readArg(name, 1)
				n := readArg(name, 2)
				if string(z.Memory()[ptr+1:ptr+10]) != "tiny.lanz" || p != 0xd000 || n > 4096 {
					t.Fatal("file_writeABI", ptr, p, n)
				}
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
	}
	if dir := os.Getenv("MINZ_SELF_PARSER_MAIN_EXPORT"); dir != "" {
		dir += "/native"
		os.MkdirAll(dir, 0755)
		os.WriteFile(dir+"/file.lanz", file, 0644)
		os.WriteFile(dir+"/stdout.txt", stdout, 0644)
		os.WriteFile(dir+"/main.a80", []byte("ORG 0x400\n"+linked), 0644)
		os.WriteFile(dir+"/main.bin", a.Binary, 0644)
		os.WriteFile(dir+"/memory.bin", z.Memory(), 0644)
	}
	if !done {
		t.Fatal("no return", z.GetPC(), z.GetSP(), string(stdout))
	}
	want, _ := os.ReadFile(root + "tiny.lanz")
	if !bytes.Equal(file, want) {
		t.Fatalf("original source native file %d bytes want%d %q", len(file), len(want), file)
	}
}
