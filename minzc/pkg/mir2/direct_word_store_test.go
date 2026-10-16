package mir2

import (
	"encoding/binary"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestDirectWordStorePairsNative(t *testing.T) {
	for _, pair := range []string{"BC", "DE"} {
		t.Run(pair, func(t *testing.T) {
			m := &Module{Globals: []Global{{Name: "out", Ty: TyU16}}}
			f := m.AddFunc("save")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			value := b.Param("value", TyU16, ClassPair)
			ptr := b.AddrOf("out", ClassPointer)
			b.Store(ptr, value, TyU16)
			b.Ret()
			ar := &AllocResult{Locs: map[Reg]PhysLoc{value: {Kind: LocReg, Name: pair}, ptr: {Kind: LocReg, Name: "HL"}}}
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(asm, "LD (out), "+pair) || strings.Contains(asm, "LD HL, out") {
				t.Fatal(asm)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
			if err != nil || len(a.Errors) > 0 {
				t.Fatal(err, a.Errors)
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], a.Binary)
			addr := a.Symbols["out"]
			for value := 0; value < 65536; value++ {
				for _, flags := range []byte{0, 1, 0x80, 0xff} {
					initial := emulator.Registers{A: 0xa5, F: flags, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(a.Symbols["save"]), SP: 0xeffe}
					if pair == "BC" {
						initial.BC = uint16(value)
					} else {
						initial.DE = uint16(value)
					}
					mem[addr], mem[addr+1], mem[addr+2] = 0xef, 0xbe, 0x5a
					mem[0xeffe], mem[0xefff] = 0, 0xf1
					z.SetRegisters(initial)
					for i := 0; i < 16 && z.GetPC() != 0xf100; i++ {
						z.Step()
					}
					got := z.GetRegisters()
					if binary.LittleEndian.Uint16(mem[addr:addr+2]) != uint16(value) || mem[addr+2] != 0x5a || got.A != initial.A || got.F != initial.F || got.BC != initial.BC || got.DE != initial.DE || got.HL != initial.HL || got.IX != initial.IX || got.IY != initial.IY || got.PC != 0xf100 || got.SP != 0xf000 {
						t.Fatal(value, flags, got, initial)
					}
				}
			}
		})
	}
}

func TestDirectWordStoreRefusals(t *testing.T) {
	for _, mode := range []string{"volatile", "shared", "intervening", "cross-block", "index-pair"} {
		t.Run(mode, func(t *testing.T) {
			addr := &Inst{Op: OpAddrOf, Dst: 1, Sym: "out", Ty: TyPtr}
			store := &Inst{Op: OpStore, Src: [2]Reg{1, 2}, Ty: TyU16}
			block := &Block{Label: "entry", Insts: []*Inst{addr, store}, Term: &TermRet{}}
			f := &Func{Name: "save", Blocks: []*Block{block}}
			pair := "DE"
			switch mode {
			case "volatile":
				store.Volatile = true
			case "shared":
				block.Term = &TermRet{Vals: []Reg{1}}
			case "intervening":
				block.Insts = []*Inst{addr, {Op: OpConst, Dst: 3, Imm: 7, Ty: TyU8}, store}
			case "cross-block":
				f.Blocks = append(f.Blocks, &Block{Label: "next", Term: &TermRet{Vals: []Reg{1}}})
			case "index-pair":
				pair = "IX"
			}
			g := &z80cg{fn: f, mod: &Module{Globals: []Global{{Name: "out", Ty: TyU16}}}, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "HL"}, 2: {Kind: LocReg, Name: pair}}}}
			g.scanGlobalFieldPatterns(block)
			if g.globalFieldSkip[1] || g.globalU16StoreSym[1] != "" {
				t.Fatal("unsafe suppression", mode)
			}
		})
	}
}
