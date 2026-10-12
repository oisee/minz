package pipeline

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestOriginalImplCPMRejectsUntrustedVariants(t *testing.T) {
	source, e := os.ReadFile("../../../reports/impl-cpm-main/source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"extra ASM call", "other BDOS function", "omitted clobbers", "extra ASM output", "out of object field", "pointer escape", "dynamic allocation", "allocation loop", "store local pointer", "untrusted address store"} {
		t.Run(name, func(t *testing.T) {
			h, e := nanz.Parse(string(source), "examples/nanz/51_impl_cpm_demo.nanz")
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{ContractOpt: true})
			if e != nil {
				t.Fatal(e)
			}
			p := s.MIR2Module.FuncByName("putchar")
			main := s.MIR2Module.FuncByName("main")
			switch name {
			case "extra ASM call":
				p.Entry().Insts[0].Asm.Template += "\nCALL 5"
			case "other BDOS function":
				p.Entry().Insts[0].Asm.Template = strings.Replace(p.Entry().Insts[0].Asm.Template, "C, 2", "C, 9", 1)
			case "omitted clobbers":
				p.Entry().Insts[0].Asm.Clobbers = 0
			case "extra ASM output":
				p.Entry().Insts[0].Asm.Outs = []mir2.Reg{p.Contract.Params[0].Reg}
			case "out of object field":
				for _, i := range main.Entry().Insts {
					if i.Op == mir2.OpField {
						i.Imm = 1000
						break
					}
				}
			case "pointer escape":
				for _, i := range main.Entry().Insts {
					if i.Op == mir2.OpCall && i.Sym == "Circle_area" {
						i.Sym = "putchar"
						break
					}
				}
			case "dynamic allocation":
				main.Entry().Insts[0].Imm = 65536
			case "store local pointer":
				for _, i := range main.Entry().Insts {
					if i.Op == mir2.OpStore {
						i.Src[1] = i.Src[0]
						break
					}
				}
			case "untrusted address store":
				for _, i := range main.Entry().Insts {
					if i.Op == mir2.OpAlloca {
						i.Op = mir2.OpConst
						i.Imm = 0x100
						break
					}
				}
			case "allocation loop":
				i := main.Entry().Insts[0]
				main.Entry().Insts = main.Entry().Insts[1:]
				b := main.NewBlock("unsafe_alloc")
				b.Insts = []*mir2.Inst{i}
				b.Term = &mir2.TermRet{}
			}
			_, e = mir2.Z80Codegen(s.MIR2Module, s.Allocation, mir2.Z80CodegenOptions{Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024})
			if e == nil {
				t.Fatal("untrusted variant admitted")
			}
			t.Log(e)
		})
	}
}

// Test-owned caller stresses a live comparison and byte across the declared
// all-clobber BDOS adapter. It does not replace the original main proof.
func TestCPMSourceBDOSCallerLiveState(t *testing.T) {
	source, e := os.ReadFile("../../../reports/impl-cpm-main/source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	source = append(source, []byte("\nfun live_state(n:u8)->u8 { var saved:u8=n\n var low:bool=n<128\n putchar(88)\n if low {return saved} else {return saved+1} }\n")...)
	// Runtime emits the main-reachable closure. Keep the test-owned entry in
	// that closure; direct invocation below still has an independent byte oracle.
	source = []byte(strings.Replace(string(source), "fun main() -> void {", "fun main() -> void {\n live_state(0)", 1))
	for _, grace := range []bool{false, true} {
		h, e := nanz.Parse(string(source), "test-owned-live-state.nanz")
		if e != nil {
			t.Fatal(e)
		}
		s, e := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: grace, UseHIRGrace: grace, Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024, AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		if d := os.Getenv("MINZ_BDOS_LIVE_EXPORT"); d != "" {
			if err := os.MkdirAll(d, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fmt.Sprintf("%s/grace-%t.a80", d, grace), []byte(s.Assembly), 0644); err != nil {
				t.Fatal(err)
			}
		}
		image, e := z80asm.NewAssembler().AssembleString(s.Assembly)
		if e != nil || len(image.Errors) > 0 {
			t.Fatal(e)
		}
		entry, ok := image.Symbols["live_state"]
		if !ok || entry < 0x100 || entry >= 0x100+len(image.Binary) {
			t.Fatal("test entry missing from emitted runtime closure", entry, ok)
		}
		for n := 0; n < 256; n++ {
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.LoadMemory(0x100, image.Binary)
			mem := z.Memory()
			r := z.GetRegisters()
			r.PC = uint16(entry)
			r.SP = uint16(image.Symbols["__cpm_stack_top"]) - 2
			mem[r.SP] = 0
			mem[r.SP+1] = 0
			f := s.MIR2Module.FuncByName("live_state")
			if e := z.SetRegister8(s.Allocation.Locs[f.Contract.Params[0].Reg].Name, byte(n)); e != nil {
				t.Fatal(e)
			}
			input := z.GetRegisters()
			r.A = input.A
			r.BC = input.BC
			r.DE = input.DE
			r.HL = input.HL
			r.IX = input.IX
			r.IY = input.IY
			z.SetRegisters(r)
			count := 0
			z.SetBDOSHandler(func(fn byte, de uint16) (byte, uint16, bool) {
				if fn != 2 || byte(de) != 88 {
					t.Fatal("unexpected BDOS", "grace", grace, "n", n, fn, de, z.GetRegisters())
				}
				count++
				rr := z.GetRegisters()
				rr.A = 0xa5
				rr.F = 0xff
				rr.BC = 0x1234
				rr.DE = 0x5678
				rr.HL = 0x9abc
				rr.IX = 0x2468
				rr.IY = 0x1357
				z.SetRegisters(rr)
				return 0xa5, 0xc3c3, true
			})
			if e := z.Run(); e != nil {
				t.Fatal(e)
			}
			want := byte(n)
			if n >= 128 {
				want++
			}
			if count != 1 || z.GetRegisters().A != want || z.GetSP() != uint16(image.Symbols["__cpm_stack_top"]) || mem[uint16(image.Symbols["__cpm_stack_guard"])] != 0xa5 {
				t.Fatal("live byte/predicate", n, z.GetRegisters(), want, count)
			}
		}
	}
}
