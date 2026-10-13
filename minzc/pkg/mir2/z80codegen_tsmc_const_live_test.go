package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestZ80TSMCWordConstantPreservesAccumulatorAndFlags(t *testing.T) {
	for _, orphan := range []bool{false, true} {
		for _, value := range []uint16{1, 0xabcd, 0xffff} {
			t.Run(fmt.Sprintf("orphan%t/%04x", orphan, value), func(t *testing.T) {
				var sb strings.Builder
				ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocMem, Offset: 0xf100}}}
				label := "_tsmc_const_live_r1_0"
				g := &z80cg{sb: &sb, ar: ar, fn: &Func{Name: "const_live"}, holdsPhys: map[string]string{}, constVals: map[Reg]int64{}, blockParamRegs: map[Reg]bool{}, tsmcPairs: []tsmcSpillPair{{reg: 1, width: 16, reloads: []tsmcReloadSite{{label: label}}}}}
				sb.WriteString("ORG 0x8000\nLD SP,0xff00\nLD HL,49159\nLD A,103\nSCF\n")
				g.genInst(&Inst{Op: OpConst, Dst: 1, Imm: int64(value), Ty: TyU16})
				if orphan {
					sb.WriteString("DI\nHALT\n")
					fixOrphanedTSMCStores(&sb, "const_live")
					sb.WriteString("_spill_const_live_r1:DW 0\n")
				} else {
					sb.WriteString(label + ":\nLD BC,0\nDI\nHALT\n")
				}
				res, e := z80asm.NewAssembler().AssembleString(sb.String())
				if e != nil || len(res.Errors) > 0 {
					t.Fatal(e, res.Errors, sb.String())
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				z.LoadMemory(0x8000, res.Binary)
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00})
				for steps := 0; !z.IsHalted(); steps++ {
					if steps > 100 {
						t.Fatal("nohalt")
					}
					z.Step()
				}
				r := z.GetRegisters()
				word := r.BC
				if orphan {
					addr := res.Symbols["_spill_const_live_r1"]
					word = uint16(z.GetMemory(uint16(addr))) | uint16(z.GetMemory(uint16(addr+1)))<<8
				}
				if r.A != 103 || r.HL != 49159 || r.F&1 == 0 || r.SP != 0xff00 || word != value {
					t.Fatalf("A%x HL%x F%x SP%x word%x want%x\n%s", r.A, r.HL, r.F, r.SP, word, value, sb.String())
				}
			})
		}
	}
}
