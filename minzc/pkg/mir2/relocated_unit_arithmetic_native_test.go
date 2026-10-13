package mir2

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
)

// The pre-codegen constant audit sees an in-place DE operation. Scratch
// preservation can subsequently relocate either side into a spill home.
// Exercise that transition with a poisoned RHS home and live scratch values.
func TestRelocatedWordUnitArithmeticNative(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub} {
		for _, tc := range []struct{ name, lhs, dst string }{
			{"both_spilled", "_spill_unit_source", "_spill_unit_result"},
			{"source_spilled", "_spill_unit_source", "DE"},
			{"source_spilled_bc", "_spill_unit_source", "BC"},
			{"source_spilled_ix", "_spill_unit_source", "IX"},
			{"source_spilled_iy", "_spill_unit_source", "IY"},
			{"ix_source", "IX", "_spill_unit_result"},
			{"iy_source", "IY", "_spill_unit_result"},
			{"result_spilled", "DE", "_spill_unit_result"},
		} {
			t.Run(fmt.Sprintf("%s/%s", op, tc.name), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("unit_arithmetic")
				b := f.NewBlock("entry")
				inst := &Inst{Op: op, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
				b.Insts = []*Inst{{Op: OpConst, Dst: 2, Imm: 1, Ty: TyU16}, inst}
				b.Term = &TermRet{Vals: []Reg{3, 4, 5, 6}}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{
					1: {Kind: LocReg, Name: "DE"}, 2: {Kind: LocReg, Name: "BC"}, 3: {Kind: LocReg, Name: "DE"},
					4: {Kind: LocReg, Name: "HL"}, 5: {Kind: LocReg, Name: "BC"}, 6: {Kind: LocReg, Name: "A"},
				}}
				dead := computeDeadConsts(f, ar)
				if !dead[2] {
					t.Fatal("unit constant was not suppressed")
				}
				var body strings.Builder
				g := &z80cg{mod: m, fn: f, curBlock: b, ar: ar, sb: &body, regInfo: collectRegInfo(f), constVals: map[Reg]int64{2: 1}, deadConsts: dead, physOverride: map[Reg]string{1: tc.lhs, 2: "_spill_unit_rhs", 3: tc.dst}, holdsPhys: map[string]string{}}
				mnem := "ADD"
				if op == OpSub {
					mnem = "SUB"
				}
				g.genBinOp(mnem, inst)
				asm := "ORG 32768\nunit_arithmetic:\n" + body.String() + "RET\n_spill_unit_source: DW 0\n_spill_unit_result: DW 0\n_spill_unit_rhs: DW 48879\n"
				bin, e := z80asm.NewAssembler().AssembleString(asm)
				if e != nil || bin == nil || len(bin.Errors) > 0 {
					t.Fatal(e, asm)
				}
				for _, v := range []uint16{0, 1, 255, 256, 32767, 32768, 65535} {
					mem := &flagLifetimeMemory{}
					copy(mem.data[0x8000:], bin.Binary)
					source := uint16(bin.Symbols["_spill_unit_source"])
					mem.data[source], mem.data[source+1] = byte(v), byte(v>>8)
					cpu := z80.NewZ80(mem, flagLifetimePorts{})
					mem.cpu = cpu
					cpu.D, cpu.E = byte(v>>8), byte(v)
					cpu.H, cpu.L = 0x12, 0x34
					cpu.B, cpu.C = 0x56, 0x78
					cpu.A = 0x9a
					cpu.IXH, cpu.IXL = 0xab, 0xcd
					cpu.IYH, cpu.IYL = 0xef, 0x12
					if tc.lhs == "IX" {
						cpu.IXH, cpu.IXL = byte(v>>8), byte(v)
					}
					if tc.lhs == "IY" {
						cpu.IYH, cpu.IYL = byte(v>>8), byte(v)
					}
					mem.data[0xeff0], mem.data[0xeff1] = 0, 3
					cpu.SetPC(0x8000)
					cpu.SetSP(0xeff0)
					for n := 0; n < 1000 && cpu.PC() != 0x300; n++ {
						cpu.DoOpcode()
					}
					got := cpu.DE()
					if tc.dst == "BC" {
						got = cpu.BC()
					}
					if tc.dst == "IX" {
						got = uint16(cpu.IXH)<<8 | uint16(cpu.IXL)
					}
					if tc.dst == "IY" {
						got = uint16(cpu.IYH)<<8 | uint16(cpu.IYL)
					}
					if tc.dst == "_spill_unit_result" {
						a := uint16(bin.Symbols["_spill_unit_result"])
						got = uint16(mem.data[a]) | uint16(mem.data[a+1])<<8
					}
					want := v + 1
					if op == OpSub {
						want = v - 1
					}
					if got != want || cpu.HL() != 0x1234 || (tc.dst != "BC" && cpu.BC() != 0x5678) || cpu.A != 0x9a || cpu.PC() != 0x300 || cpu.SP() != 0xeff2 {
						t.Fatalf("v=%04x got=%04x want=%04x HL=%04x BC=%04x A=%02x\n%s", v, got, want, cpu.HL(), cpu.BC(), cpu.A, asm)
					}
					codeBytes := int(bin.Symbols["_spill_unit_source"]) - 0x8000
					if !bytes.Equal(mem.data[0x8000:0x8000+codeBytes], bin.Binary[:codeBytes]) {
						t.Fatal("instructions changed")
					}
				}
			})
		}
	}
}
