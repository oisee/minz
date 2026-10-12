package pipeline

import (
	"bytes"
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

// net_send uses NUL termination; Nanz ordinary strings are length-prefixed.
// See Nanz Language Book string representations and mir2.StringKind contracts.
func TestIRCMinimalWireCString(t *testing.T) {
	for _, old := range []bool{true, false} {
		path := "../../../examples/nanz/test_irc_minimal.nanz"
		if old {
			path = "../../../reports/irc-minimal-cstring/before.nanz"
		}
		src, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			t.Run(fmt.Sprintf("old=%v/%s", old, mode), func(t *testing.T) {
				parse := func() *hir.Module {
					h, e := nanz.ParseWithOpts(string(src), "test_irc_minimal.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				raw := hir.LowerModule(parse())
				s, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
				if e != nil {
					t.Fatal(e)
				}
				// Test-owned linker only separates declared net_write, whose RET is real.
				asm := strings.ReplaceAll(s.Assembly, "net_write:\n", "net_write:\n    RET\n")
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x100\n" + asm)
				if e != nil || len(a.Errors) > 0 {
					t.Fatal(e, a.Errors)
				}
				for _, wire := range []string{"NICK minztest\r\n", "USER minz 0 * :test\r\n"} {
					for j, m := range []*mir2.Module{raw, s.MIR2Module} {
						vm := mir2.NewVM(m)
						vm.MaxSteps = 20000
						var out []byte
						var ptr int64 = -1
						for i := 0; i < m.Strings.Len(); i++ {
							p, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", i))
							if e != nil {
								t.Fatal(e)
							}
							if m.Strings.At(i) == wire {
								ptr = p
							}
						}
						if ptr < 0 {
							t.Fatal("missing literal")
						}
						vm.EnsureHeap(65536)
						before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
						vm.Hosts["net_write"] = func(v []mir2.Value) ([]mir2.Value, error) {
							out = append(out, byte(v[0].I))
							if len(out) > 256 {
								return nil, fmt.Errorf("wire overrun")
							}
							return nil, nil
						}
						_, e = vm.Call("net_send", []mir2.Value{{I: ptr}})
						if e != nil {
							t.Fatal(e)
						}
						if !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
							t.Fatal("VM RAM")
						}
						if old {
							if bytes.Equal(out, []byte(wire)) || len(out) == 0 || out[0] != byte(len(wire)) {
								t.Fatalf("old representation counterexample missing %q", out)
							}
						} else if string(out) != wire {
							t.Fatalf("VM%d wire=%q want%q", j, out, wire)
						}
					}
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					z.LoadMemory(0x100, a.Binary)
					mem := z.Memory()
					var ptr int = -1
					for i := 0; i < s.MIR2Module.Strings.Len(); i++ {
						if s.MIR2Module.Strings.At(i) == wire {
							ptr = a.Symbols[fmt.Sprintf("_mir2_str_%d", i)]
						}
					}
					if ptr < 0 {
						t.Fatal("native literal")
					}
					mem[0xeff0], mem[0xeff1] = 0, 0x70
					r := emulator.Registers{PC: uint16(a.Symbols["net_send"]), SP: 0xeff0, HL: uint16(ptr)}
					if assertParamLocation(s.MIR2Module.FuncByName("net_send").Contract.Params[0], s.Allocation).Name != "HL" {
						t.Fatal("net_send ABI")
					}
					z.SetRegisters(r)
					before := append([]byte(nil), mem...)
					var out []byte
					minSP := uint16(0xeff0)
					done := false
					for k := 0; k < 20000; k++ {
						if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
							done = true
							break
						}
						if z.GetPC() == uint16(a.Symbols["net_write"]) {
							r = z.GetRegisters()
							loc := assertParamLocation(s.MIR2Module.FuncByName("net_write").Contract.Params[0], s.Allocation)
							var b byte
							switch loc.Name {
							case "A":
								b = r.A
							case "C":
								b = byte(r.BC)
							case "B":
								b = byte(r.BC >> 8)
							case "D":
								b = byte(r.DE >> 8)
							case "E":
								b = byte(r.DE)
							default:
								t.Fatal("net_write ABI", loc)
							}
							out = append(out, b)
							if len(out) > 256 {
								t.Fatal("native overrun")
							}
						}
						if z.GetPC() < 0x100 || int(z.GetPC()) >= 0x100+len(a.Binary) {
							t.Fatal("foreign PC", z.GetPC())
						}
						z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
					}
					if !done {
						t.Fatal("native termination")
					}
					for i, b := range before {
						if i >= int(minSP) && i < 0xeff0 {
							continue
						}
						if mem[i] != b {
							t.Fatalf("native RAM%x", i)
						}
					}
					if old {
						if len(out) == 0 || out[0] != byte(len(wire)) || string(out) == wire {
							t.Fatal("native negative", out)
						}
					} else if string(out) != wire {
						t.Fatalf("native wire%q want%q", out, wire)
					}
					t.Logf("old=%v wire=%q", old, out)
				}
			})
		}
	}
}
