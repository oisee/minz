package abap

import (
	"encoding/binary"
	"fmt"
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

func TestCharacterSelectionInputVMAndZ80(t *testing.T) {
	for _, declared := range []int{0, 3, 300} {
		t.Run(fmt.Sprintf("length-%d", declared), func(t *testing.T) {
			length := characterLength(declared)
			addition := ""
			if declared != 0 {
				addition = fmt.Sprintf(" LENGTH %d", declared)
			}
			src := fmt.Sprintf(`REPORT zinput.
PARAMETERS p TYPE c%s DEFAULT 'old'.
DATA snapshot TYPE c LENGTH %d.
DATA seen TYPE i.
INITIALIZATION.
 p = 'XYZ'.
 seen = 1.
AT SELECTION-SCREEN.
 snapshot = p.
 seen = 2.
START-OF-SELECTION.
 seen = 3.
 WRITE p.
`, addition, length)
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
			if err != nil || len(a.Errors) > 0 {
				t.Fatalf("assemble: %v %v", err, a.Errors)
			}
			if dir := os.Getenv("MINZ_ABAP_CINPUT_ARTIFACTS"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for ext, data := range map[string][]byte{"abap": []byte(src), "a80": []byte("ORG 0x8000\n" + text), "bin": a.Binary} {
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("report-%d.%s", length, ext)), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			pad := func(s string) string { b := []byte(strings.Repeat(" ", length)); copy(b, []byte(s)); return string(b) }
			for _, input := range []string{"", "A", "ABCDE", "   ", "\x00A", "\x80"} {
				t.Run(input, func(t *testing.T) {
					fault := byte(0)
					want := pad(input)
					if input == "" {
						want = pad("XYZ")
					}
					if input == "\x80" {
						fault = 4
						want = pad("XYZ")
					}
					seen := int32(3)
					snapshot := want
					if fault != 0 {
						seen = 1
						snapshot = strings.Repeat(" ", length)
					}
					check := func(read func(string, int) []byte) {
						for name, want := range map[string]string{"p": want, "snapshot": snapshot} {
							if got := string(read(name, length)); got != want {
								t.Fatalf("%s=%q want %q", name, got, want)
							}
						}
						if got := int32(binary.LittleEndian.Uint32(read("seen", 4))); got != seen {
							t.Fatalf("seen=%d want %d", got, seen)
						}
						if got := read("_abap_error_code", 1)[0]; got != fault {
							t.Fatalf("fault=%d want %d", got, fault)
						}
					}
					prompt := "P [" + strings.TrimRight(pad("XYZ"), " ") + "]: \r\n"
					if fault == 0 {
						prompt += "\r\n" + want
					}
					for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
						v := mir2.NewVM(module)
						registered := false
						v.Hosts["sel_register_c"] = func(args []mir2.Value) ([]mir2.Value, error) {
							if len(args) != 3 || args[1].I != int64(length) || args[2].I != v.GlobalAddr("p") || string(v.ReadHeap(args[2].I, length)) != pad("XYZ") {
								t.Fatalf("registration lost actual object/length/INITIALIZATION: %v", args)
							}
							registered = true
							return nil, nil
						}
						v.Hosts["sel_show"] = func([]mir2.Value) ([]mir2.Value, error) {
							if !registered {
								t.Fatal("show before registration")
							}
							return []mir2.Value{{I: 0}}, nil
						}
						v.Hosts["_abap_input_line"] = func([]mir2.Value) ([]mir2.Value, error) {
							v.WriteHeapBytes(v.GlobalAddr("_abap_inbuf")+2, []byte(input))
							return []mir2.Value{{I: int64(len(input))}}, nil
						}
						var output []byte
						v.Hosts["abap_write_str"] = func(args []mir2.Value) ([]mir2.Value, error) {
							for p := args[0].I; ; p++ {
								b := v.ReadHeap(p, 1)[0]
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
						if (err != nil) != (fault != 0) {
							t.Fatalf("VM error=%v expected fault %d", err, fault)
						}
						check(func(name string, n int) []byte { return v.ReadHeap(v.GlobalAddr(name), n) })
						if string(output) != prompt {
							t.Fatalf("VM output=%q want %q", output, prompt)
						}
					}
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					copy(mem[0x8000:], a.Binary)
					mem[0xeffe], mem[0xefff] = 0, 0xf1
					mem[0xe000] = 0xa5
					z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xeffe})
					var output []byte
					reads := 0
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
							reads++
							if mem[r.DE] != 20 {
								t.Fatal("BDOS buffer capacity")
							}
							mem[r.DE+1] = byte(len(input))
							copy(mem[int(r.DE)+2:], []byte(input))
						default:
							t.Fatalf("unexpected BDOS %d", byte(r.BC))
						}
						r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
						r.SP += 2
						r.A, r.F, r.BC, r.DE, r.HL = 0xa5, 0, 0x1234, 0xabcd, 0xbeef
						z.SetRegisters(r)
					}
					r := z.GetRegisters()
					if fault == 0 {
						if r.PC != 0xf100 || r.SP != 0xf000 {
							t.Fatalf("native return %+v", r)
						}
					} else if !z.IsHalted() || r.PC == 0xf100 || r.SP <= 0xe000 || r.SP > 0xeffe || z.GetIFF1() {
						t.Fatalf("native fault %+v", r)
					}
					if reads != 1 || mem[0xe000] != 0xa5 || binary.LittleEndian.Uint16(mem[0xeffe:0xf000]) != 0xf100 {
						t.Fatal("input count/stack canary/return address")
					}
					check(func(name string, n int) []byte {
						addr, ok := a.Symbols[name]
						if !ok {
							t.Fatalf("missing global %s", name)
						}
						return mem[addr : addr+n]
					})
					if string(output) != prompt {
						t.Fatalf("native output=%q want %q", output, prompt)
					}
				})
			}
		})
	}
}
