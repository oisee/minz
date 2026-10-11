package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestZ80DirectConstantStorePreservesHLBytes(t *testing.T) {
	for _, loc := range []string{"HL", "H", "L", "DE"} {
		t.Run(loc, func(t *testing.T) {
			var sb strings.Builder
			store := &Inst{Op: OpStore, Src: [2]Reg{2, 3}, Ty: TyU8}
			b := &Block{Label: "entry", Insts: []*Inst{store}, Term: &TermRet{Vals: []Reg{1}}}
			f := &Func{Name: "direct", Blocks: []*Block{b}}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: loc}, 2: {Kind: LocReg, Name: "HL"}, 3: {Kind: LocReg, Name: "C"}}}
			g := &z80cg{sb: &sb, ar: ar, fn: f, curBlock: b, holdsPhys: map[string]string{}, constVals: map[Reg]int64{3: 0}, globalFieldStore: map[Reg]globalFieldInfo{2: {sym: "cell"}}, blockParamRegs: map[Reg]bool{}}
			sb.WriteString("ORG 0x8000\nLD SP,0xff00\nLD HL,0xABCD\nLD DE,0x4567\nLD A,103\nSCF\n")
			g.genInst(store)
			sb.WriteString("DI\nHALT\ncell: DB 255\n")
			result, e := z80asm.NewAssembler().AssembleString(sb.String())
			if e != nil || len(result.Errors) > 0 {
				t.Fatal(e, result.Errors, sb.String())
			}
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, result.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00})
			for n := 0; !z.IsHalted(); n++ {
				if n > 100 {
					t.Fatal("noHALT")
				}
				z.Step()
			}
			r := z.GetRegisters()
			if (loc != "DE" && r.HL != 0xABCD) || r.DE != 0x4567 || r.A != 103 || r.F&1 == 0 || z.GetMemory(uint16(result.Symbols["cell"])) != 0 {
				t.Fatal(loc, fmt.Sprintf("%+v", r), sb.String())
			}
			if loc == "DE" && strings.Contains(sb.String(), "PUSH HL") {
				t.Fatal("dead HL unnecessarily saved")
			}
		})
	}
}
