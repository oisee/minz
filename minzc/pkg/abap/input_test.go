package abap

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestIntegerSelectionInputVMAndZ80(t *testing.T) {
	const src = `REPORT zinput.
PARAMETERS p TYPE i DEFAULT -7.
DATA seen TYPE i VALUE 123.
INITIALIZATION.
 p = -65537.
START-OF-SELECTION.
 seen = p.
`
	m, err := Compile(src, "zinput")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	text, err := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{CalledMain: true, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, a.Errors)
	}
	if a.Symbols["_abap_error_code"]-a.Symbols["_abap_inbuf"] < 22 {
		t.Fatal("native line buffer storage is shorter than its BDOS contract")
	}
	if dir := os.Getenv("MINZ_ABAP_INPUT_ARTIFACTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"program.abap": []byte(src), "program.a80": []byte("ORG 0x8000\n" + text), "program.bin": a.Binary} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, c := range []struct {
		input string
		value int32
		fault byte
	}{
		{"", -65537, 0}, {"0", 0, 0}, {"+0", 0, 0}, {"-0", 0, 0},
		{"65535", 65535, 0}, {"65536", 65536, 0}, {"-65537", -65537, 0},
		{"2147483647", 2147483647, 0}, {"-2147483648", -2147483648, 0},
		{"  +2147483647  ", 2147483647, 0}, {"  -2147483648  ", -2147483648, 0},
		{"00000000002147483647", 2147483647, 0},
		{" ", -65537, 4}, {"+", -65537, 4}, {"-", -65537, 4},
		{"1x", -65537, 4}, {"1 2", -65537, 4}, {"\t1", -65537, 4},
		{"0x10", -65537, 4}, {"--1", -65537, 4},
		{"2147483648", -65537, 5}, {"-2147483649", -65537, 5},
		{"4294967295", -65537, 5}, {"99999999999999999999", -65537, 5},
	} {
		t.Run(c.input, func(t *testing.T) {
			seen := c.value
			if c.fault != 0 {
				seen = 123
			}
			check := func(read func(string, int) []byte) {
				for name, want := range map[string]int32{"p": c.value, "seen": seen} {
					if got := int32(binary.LittleEndian.Uint32(read(name, 4))); got != want {
						t.Fatalf("%s=%d want %d", name, got, want)
					}
				}
				if got := read("_abap_error_code", 1)[0]; got != c.fault {
					t.Fatalf("fault=%d want %d", got, c.fault)
				}
			}
			for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
				v := mir2.NewVM(module)
				calls := 0
				v.Hosts["sel_show"] = func([]mir2.Value) ([]mir2.Value, error) { return []mir2.Value{{I: 0}}, nil }
				v.Hosts["_abap_input_line"] = func([]mir2.Value) ([]mir2.Value, error) {
					calls++
					v.WriteHeapBytes(v.GlobalAddr("_abap_inbuf")+2, []byte(c.input))
					return []mir2.Value{{I: int64(len(c.input))}}, nil
				}
				var output []byte
				v.Hosts["abap_write_str"] = func(args []mir2.Value) ([]mir2.Value, error) {
					for ptr := args[0].I; ; ptr++ {
						b := v.ReadHeap(ptr, 1)[0]
						if b == 0 {
							break
						}
						output = append(output, b)
					}
					return nil, nil
				}
				v.Hosts["_abap_num_putchar"] = func(args []mir2.Value) ([]mir2.Value, error) {
					output = append(output, byte(args[0].I))
					return nil, nil
				}
				_, err := v.Call("main", nil)
				if (err != nil) != (c.fault != 0) {
					t.Fatalf("VM error=%v want fault %d", err, c.fault)
				}
				if calls != 1 {
					t.Fatalf("VM line reads %d", calls)
				}
				check(func(name string, n int) []byte { return v.ReadHeap(v.GlobalAddr(name), n) })
				want := "P [-65537]: \r\n"
				if c.fault == 0 {
					want += "\r\n"
				}
				if string(output) != want {
					t.Fatalf("VM output=%q want %q", output, want)
				}
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], a.Binary)
			mem[0xeffe], mem[0xefff] = 0, 0xf1
			mem[0xe000] = 0xa5
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xeffe})
			var output []byte
			calls := 0
			for i := 0; i < 2000000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
				if z.GetPC() != 5 {
					z.Step()
					continue
				}
				r := z.GetRegisters()
				switch byte(r.BC) {
				case 2:
					output = append(output, byte(r.DE))
				case 10:
					calls++
					if mem[r.DE] != 20 || len(c.input) > 20 {
						t.Fatal("line buffer contract")
					}
					mem[r.DE+1] = byte(len(c.input))
					copy(mem[int(r.DE)+2:], []byte(c.input))
				default:
					t.Fatalf("unexpected BDOS %d", byte(r.BC))
				}
				r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
				r.SP += 2
				// Exercise the low-level IO clobber contract, rather than relying
				// on a BDOS stub that preserves every application register.
				r.A, r.F, r.BC, r.DE, r.HL = 0xa5, 0, 0x1234, 0xabcd, 0xbeef
				z.SetRegisters(r)
			}
			r := z.GetRegisters()
			if c.fault == 0 {
				if r.PC != 0xf100 || r.SP != 0xf000 {
					t.Log(text)
					t.Fatalf("return PC=%04x SP=%04x", r.PC, r.SP)
				}
			} else if !z.IsHalted() || r.PC == 0xf100 || r.SP > 0xeffe || r.SP <= 0xe000 || z.GetIFF1() || (mem[r.PC] != 0x76 && mem[r.PC-1] != 0x76) {
				t.Fatalf("fault termination %+v", r)
			}
			if calls != 1 || mem[0xe000] != 0xa5 || binary.LittleEndian.Uint16(mem[0xeffe:0xf000]) != 0xf100 {
				t.Fatal("input count/stack canary/outer return")
			}
			check(func(name string, n int) []byte {
				addr, ok := a.Symbols[name]
				if !ok {
					t.Fatalf("missing native global %s", name)
				}
				return mem[addr : addr+n]
			})
			want := "P [-65537]: \r\n"
			if c.fault == 0 {
				want += "\r\n"
			}
			if string(output) != want {
				t.Fatalf("native output=%q want %q", output, want)
			}
		})
	}
}

func TestIntegerSelectionHostedTransport(t *testing.T) {
	m, err := Compile(`REPORT zinput.
PARAMETERS p TYPE i DEFAULT 7.
DATA seen TYPE i.
INITIALIZATION.
 p = -65537.
START-OF-SELECTION.
 seen = p.`, "zinput")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []int32{-2147483648, -65537, 65536, 2147483647} {
		for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
			v := mir2.NewVM(module)
			registered, shown, got := false, false, false
			v.Hosts["sel_register_int"] = func(args []mir2.Value) ([]mir2.Value, error) {
				if len(args) != 2 || int32(args[1].I) != -65537 {
					t.Fatalf("registration lost INITIALIZATION/i32: %v", args)
				}
				registered = true
				return nil, nil
			}
			v.Hosts["sel_show"] = func([]mir2.Value) ([]mir2.Value, error) {
				if !registered {
					t.Fatal("show before registration")
				}
				shown = true
				return []mir2.Value{{I: 1}}, nil
			}
			v.Hosts["sel_get_int"] = func(args []mir2.Value) ([]mir2.Value, error) {
				if !shown || len(args) != 1 || args[0].I != 0 {
					t.Fatal("get before show/wrong field")
				}
				got = true
				return []mir2.Value{{I: int64(value)}}, nil
			}
			v.Hosts["_abap_input_line"] = func([]mir2.Value) ([]mir2.Value, error) {
				t.Fatal("hosted input entered native fallback")
				return nil, nil
			}
			if _, err := v.Call("main", nil); err != nil {
				t.Fatal(err)
			}
			if !registered || !shown || !got {
				t.Fatal("missing transport events")
			}
			for _, name := range []string{"p", "seen"} {
				if got := int32(binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr(name), 4))); got != value {
					t.Fatalf("%s=%d want %d", name, got, value)
				}
			}
		}
	}
}

func TestIntegerInputZXLineAcquisition(t *testing.T) {
	m := &hir.Module{Target: hir.TargetZXSpectrum}
	emitIntegerInputIO(m, hir.TargetZXSpectrum)
	// Isolate the console-input kernel from screen rendering. Its echo target
	// deliberately clobbers BC/HL to verify the kernel's explicit saves.
	m.Funcs = append(m.Funcs, &hir.Func{Name: "_zx_putchar", RetTy: mir2.TyVoid, Body: &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Target: "z80", Code: "LD BC, 0x1234 / LD HL, 0xabcd", ClobberRegs: []string{"B", "C", "H", "L"}}}}})
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	text := steps.Assembly
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, a.Errors)
	}
	for _, c := range []struct{ input, want string }{
		{"-2147483648\r", "-2147483648"},
		{"12\b3\x7f4\r", "14"},
		{strings.Repeat("0", 20) + "9\b7\r", strings.Repeat("0", 19) + "7"},
		{"\b\r", ""},
	} {
		t.Run(c.input, func(t *testing.T) {
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], a.Binary)
			mem[0xeffe], mem[0xefff] = 0, 0xf1
			reads := 0
			z.SetIOHandlers(func(port uint16) byte {
				if byte(port) != 0x23 || reads >= len(c.input) {
					t.Fatal("unexpected input poll")
				}
				b := c.input[reads]
				reads++
				return b | 0x80
			}, func(uint16, byte) { t.Fatal("unexpected output port") })
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["_abap_input_line"]), SP: 0xeffe})
			for i := 0; i < 10000 && z.GetPC() != 0xf100; i++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.PC != 0xf100 || r.SP != 0xf000 || int(r.A) != len(c.want) || reads != len(c.input) {
				t.Fatalf("line return %+v reads=%d", r, reads)
			}
			addr := a.Symbols["_abap_inbuf"] + 2
			if got := string(mem[addr : addr+len(c.want)]); got != c.want {
				t.Fatalf("line=%q want %q", got, c.want)
			}
		})
	}
}
