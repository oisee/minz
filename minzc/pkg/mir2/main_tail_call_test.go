package mir2

import (
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestVoidMainFinalCallHonorsNativeEntryContract(t *testing.T) {
	for _, mode := range []struct {
		name string
		opt  Z80CodegenOptions
	}{
		{"bare", Z80CodegenOptions{}},
		{"called", Z80CodegenOptions{CalledMain: true}},
		{"cpm", Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: 1024}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			m := &Module{Globals: []Global{{Name: "observable", Ty: TyU8}}}
			main := m.AddFunc("main")
			b := NewBuilder(main)
			b.SwitchToNewBlock("entry")
			b.CallVoid("finish", nil, CallAttrs{})
			b.Ret()
			helper := m.AddFunc("finish")
			h := NewBuilder(helper)
			h.SwitchToNewBlock("entry")
			h.Store(h.AddrOf("observable", ClassPointer), h.Const(42, TyU8, ClassAcc), TyU8)
			h.Ret()
			ar := PBQPAllocate(helper, ComputeLiveness(helper), Z80CostTable{})
			text, err := Z80Codegen(m, ar, mode.opt)
			if err != nil {
				t.Fatal(err)
			}
			origin := 0x8000
			assembly := "ORG 0x8000\n" + text
			if mode.name == "cpm" {
				origin = 0x100
				assembly = text
			}
			a, err := z80asm.NewAssembler().AssembleString(assembly)
			if err != nil || len(a.Errors) > 0 {
				t.Fatalf("assemble %v %v", err, a.Errors)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			if mode.name == "cpm" {
				z.SetROMEnd(0)
			}
			mem := z.Memory()
			copy(mem[origin:], a.Binary)
			pc := uint16(a.Symbols["main"])
			sp := uint16(0xf000)
			if mode.name != "bare" {
				sp = 0xeffe
				mem[sp], mem[sp+1] = 0, 0xf1
			}
			if mode.name == "cpm" {
				pc = uint16(a.Symbols["__cpm_entry"])
			}
			mem[0xe000] = 0xa5
			z.SetRegisters(emulator.Registers{PC: pc, SP: sp})
			for i := 0; i < 10000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
				z.Step()
			}
			r := z.GetRegisters()
			terminal := r.PC == 0xf100
			if mode.name == "bare" {
				terminal = z.IsHalted() && (mem[r.PC] == 0x76 || mem[r.PC-1] == 0x76)
			}
			if !terminal || r.SP != 0xf000 || mem[0xe000] != 0xa5 || mem[a.Symbols["observable"]] != 42 {
				t.Log(text)
				t.Fatalf("%s PC=%04x SP=%04x observed=%d", mode.name, r.PC, r.SP, mem[a.Symbols["observable"]])
			}
			if mode.name == "cpm" && mem[a.Symbols["__cpm_status"]] != 0 {
				t.Fatal("runtime failure")
			}
		})
	}
}
