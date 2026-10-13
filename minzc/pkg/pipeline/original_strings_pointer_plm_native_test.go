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
	"github.com/minz/minzc/pkg/plm"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// Book v7 §18.1 CString/SString/LString bytes, §2.15 pointer loads/stores.
func TestOriginalStringsPointerAndPLMAdapter(t *testing.T) {
	data, e := os.ReadFile("../../../reports/strings-pointer-plm-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		Sources   map[string]string `json:"sources"`
		Strings   map[string]string `json:"strings"`
		Addresses []int64           `json:"ptr_addresses"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil {
		t.Fatal(e)
	}
	save := func(n string, v any) {
		dir := os.Getenv("MINZ_STRINGS_POINTER_PLM_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, n)
		os.MkdirAll(filepath.Dir(p), 0755)
		var b []byte
		switch x := v.(type) {
		case string:
			b = []byte(x)
		case []byte:
			b = x
		default:
			b, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for _, path := range []string{"reports/showcase-src/2026-03-15/ex30_ptr_cast.nanz", "reports/showcase-src/2026-03-14/ex27_strings.nanz", "examples/plm/hello.plm"} {
		readPath := "../../../" + path
		if filepath.Ext(path) == ".plm" {
			// Retain the historical adapter evidence; the current source has
			// a separate whole-program CP/M regression.
			readPath = "../../../reports/strings-pointer-plm-native/source/" + path
		}
		src, e := os.ReadFile(readPath)
		if e != nil || freeze.Sources[path] != fmt.Sprintf("%x", sha256.Sum256(src)) {
			t.Fatal("freeze", path, e)
		}
		isPLM := filepath.Ext(path) == ".plm"
		parse := func() *hir.Module {
			var h *hir.Module
			var e error
			if isPLM {
				h, e = plm.Compile(string(src))
			} else {
				h, e = nanz.Parse(string(src), path)
			}
			if e != nil {
				t.Fatal(e)
			}
			if isPLM {
				f := h.FuncByName("PRINTCHAR")
				f.Body = &hir.Block{Body: []hir.Stmt{&hir.ExprStmt{Expr: &hir.CallExpr{Fn: "@mir.io.print.char", Args: []hir.Expr{&hir.VarRefExpr{Name: f.Params[0].Name, Ty: mir2.TyU8}}, Ty: mir2.TyVoid}}, &hir.ReturnStmt{}}}
			}
			return h
		}
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			raw := hir.LowerModule(parse())
			h := parse()
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + s.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			prefix := path + "/" + mode
			save(prefix+"/target.a80", s.Assembly)
			save(prefix+"/target.bin", a.Binary)
			save(prefix+"/raw.mir", raw.Dump())
			save(prefix+"/final.mir", s.MIR2Module.Dump())
			save(prefix+"/allocation.json", s.Allocation)
			spills := map[int]bool{}
			for _, l := range a.Listing {
				if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
					for j := range l.Bytes {
						spills[l.Address+j] = true
					}
				}
			}
			calls := 0
			check := func(entry string, args []int64, value *byte, output string, writeAddr int64, writeValue byte, input []byte) {
				calls++
				for label, m := range map[string]*mir2.Module{"raw": raw, "final": s.MIR2Module} {
					vm := mir2.NewVM(m)
					for j := 0; j < m.Strings.Len(); j++ {
						if _, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", j)); e != nil {
							t.Fatal(e)
						}
					}
					vm.EnsureHeap(65536)
					if len(input) > 0 {
						vm.WriteHeapSlice(args[0], input)
					}
					if entry == "peek" {
						vm.WriteHeap(args[0], byte(args[0]^0x59))
					}
					before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
					var out bytes.Buffer
					vm.Hosts["@mir.io.print.char"] = func(v []mir2.Value) ([]mir2.Value, error) { out.WriteByte(byte(v[0].I)); return nil, nil }
					vm.Hosts["@mir.io.print.str"] = func(v []mir2.Value) ([]mir2.Value, error) {
						for p := v[0].I; ; p++ {
							b := vm.ReadHeap(p, 1)[0]
							if b == 0 {
								break
							}
							out.WriteByte(b)
						}
						return nil, nil
					}
					vs := make([]mir2.Value, len(args))
					for j, v := range args {
						vs[j] = mir2.Value{I: v}
					}
					var events []mir2.VMDataAccess
					vm.ObserveDataAccess = func(v mir2.VMDataAccess) { events = append(events, v) }
					v, e := vm.Call(entry, vs)
					if e != nil {
						t.Fatal(path, mode, label, entry, args, e)
					}
					if value != nil && (len(v) != 1 || v[0].I != int64(*value)) {
						t.Fatal("VMresult", entry, args, v, *value)
					}
					if out.String() != output {
						t.Fatal("VMoutput", entry, out.String(), output)
					}
					if writeAddr >= 0 {
						before[writeAddr] = writeValue
					} // Strings are lazily interned beyond initial heap; initial64KiB stays unchanged.
					if !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
						t.Fatal("VMRAM", entry)
					}
					if entry == "peek" || entry == "poke" || entry == "main" {
						op := mir2.OpLoad
						if entry == "poke" {
							op = mir2.OpStore
						}
						if entry == "main" {
							if len(events) != 2 || events[0].Op != mir2.OpStore || events[1].Op != mir2.OpLoad || events[0].Addr != 0x8000 || events[1].Addr != 0x8000 {
								t.Fatal("main ordered accesses", events)
							}
						} else if len(events) != 1 || events[0].Op != op || events[0].Addr != args[0] || events[0].Value != byte(func() int64 {
							if entry == "poke" {
								return args[1]
							}
							return args[0] ^ 0x59
						}()) {
							t.Fatal("pointer exactaccess", entry, events)
						}
					}
				}
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				mem := z.Memory()
				copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
				z.LoadMemory(0x4000, a.Binary)
				if len(input) > 0 {
					copy(mem[args[0]:], input)
				}
				if entry == "peek" {
					mem[args[0]] = byte(args[0] ^ 0x59)
				}
				mem[0xeff0], mem[0xeff1] = 0, 0x70
				before := append([]byte(nil), mem...)
				r := emulator.Registers{PC: uint16(a.Symbols[entry]), SP: 0xeff0, A: 0x42, HL: 0x7777, BC: 0x5555, DE: 0x6666}
				f := s.MIR2Module.FuncByName(entry)
				for j, v := range args {
					loc := assertParamLocation(f.Contract.Params[j], s.Allocation)
					switch loc.Name {
					case "A":
						r.A = byte(v)
					case "HL":
						r.HL = uint16(v)
					case "C":
						r.BC = r.BC&0xff00 | uint16(byte(v))
					default:
						t.Fatal("ABI", loc)
					}
				}
				z.SetRegisters(r)
				minSP := uint16(0xeff0)
				done := false
				var events []mir2.VMDataAccess
				for n := 0; n < 1000000; n++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						done = true
						break
					}
					events = append(events, lizpTokenizerNativeAccess(mem, z.GetPC(), z.GetRegisters())...)
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
					if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
						t.Fatal("stackbounds")
					}
				}
				if !done {
					t.Fatal("return", entry)
				}
				if value != nil && z.GetRegisters().A != *value {
					t.Fatal("nativevalue", entry, args, z.GetRegisters().A, *value)
				}
				if string(z.GetOutput()) != output {
					t.Fatal("nativeoutput", entry, string(z.GetOutput()), output)
				}
				if writeAddr >= 0 {
					before[writeAddr] = writeValue
				}
				copy(before[minSP:0xeff0], mem[minSP:0xeff0])
				for p := range spills {
					before[p] = mem[p]
				}
				if !bytes.Equal(before, mem) {
					for p := range mem {
						if before[p] != mem[p] {
							t.Fatal("nativeRAM", entry, p)
						}
					}
				}
				if entry == "peek" || entry == "poke" || entry == "main" {
					if entry == "main" {
						if len(events) != 2 || events[0].Op != mir2.OpStore || events[1].Op != mir2.OpLoad {
							t.Fatal("nativeorderedmain", events)
						}
					} else if len(events) != 1 || events[0].Addr != args[0] || events[0].Value != byte(func() int64 {
						if entry == "poke" {
							return args[1]
						}
						return args[0] ^ 0x59
					}()) {
						t.Fatal("nativepointeraccess", events)
					}
				}
			}
			if filepath.Base(path) == "ex30_ptr_cast.nanz" {
				for _, addr := range freeze.Addresses {
					v := byte(addr ^ 0x59)
					check("peek", []int64{addr}, &v, "", -1, 0, nil)
					for x := 0; x < 256; x++ {
						check("poke", []int64{addr, int64(x)}, nil, "", addr, byte(x), nil)
					}
				}
				v := byte(42)
				check("main", nil, &v, "", 0x8000, 42, nil)
			} else if isPLM {
				for x := 0; x < 256; x++ {
					check("PRINTCHAR", []int64{int64(x)}, nil, string([]byte{byte(x)}), -1, 0, nil)
				}
				for _, msg := range []string{"", "Hello from PL/M-80!", "a\nb"} {
					check("PRINTMSG", []int64{0xc000}, nil, msg, -1, 0, append([]byte(msg), 0))
				} // Explicit process adapter sequences original PRINTMSG then PRINTCHAR CR/LF.
				check("PRINTMSG", []int64{0xc000}, nil, "Hello from PL/M-80!", -1, 0, []byte("Hello from PL/M-80!\x00"))
				check("PRINTCHAR", []int64{13}, nil, "\r", -1, 0, nil)
				check("PRINTCHAR", []int64{10}, nil, "\n", -1, 0, nil)
			} else {
				for _, entry := range []string{"greet", "greet_pascal", "greet_long", "multiline"} {
					check(entry, nil, nil, freeze.Strings[entry], -1, 0, nil)
				}
			}
			save(prefix+"/evidence.json", map[string]any{"calls": calls, "adapter": isPLM, "source_sha256": freeze.Sources[path], "bin_sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary))})
			t.Log(path, mode, calls, "fullsourcecallables native/raw/final RAM/SP/output PASS; PLMadapter", isPLM)
		}
	}
}
