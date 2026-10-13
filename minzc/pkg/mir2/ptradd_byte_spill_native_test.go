package mir2

import (
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// A one-byte index spill has deliberately poisoned adjacent storage. ptr_add
// must zero-extend that byte, preserve A, and never consume the poison.
func TestPtrAddByteSpillNative(t *testing.T) {
	for _, base := range []string{"HL", "DE", "BC"} {
		f := &Func{Name: "index"}
		inst := &Inst{Op: OpPtrAdd, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyPtr}
		b := f.NewBlock("entry")
		b.Insts = []*Inst{inst}
		b.Term = &TermRet{}
		var out strings.Builder
		g := &z80cg{fn: f, curBlock: b, sb: &out, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: base}, 2: {Kind: LocMem, Name: "_spill_index_r2"}, 3: {Kind: LocReg, Name: "HL"}}}, regInfo: map[Reg]RegInfo{1: {Ty: TyPtr}, 2: {Ty: TyU8}, 3: {Ty: TyPtr}}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, constVals: map[Reg]int64{}, liveness: ComputeLiveness(f)}
		g.genInst(inst)
		a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + out.String() + "RET\n_spill_index_r2 EQU 0x6000\n")
		if err != nil {
			t.Fatal(err)
		}
		m, err := spectrum.NewPentagon128(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		for i, v := range a.Binary {
			m.Memory.Write(0x8000+uint16(i), v, false)
		}
		for value := 0; value < 256; value++ {
			m.Memory.Write(0x6000, byte(value), false)
			m.Memory.Write(0x6001, 0xa7, false)
			m.Memory.Write(0xbf00, 0, false)
			m.Memory.Write(0xbf01, 0x70, false)
			m.CPU.SetHL(0x5000)
			m.CPU.SetDE(0x5000)
			m.CPU.SetBC(0x5000)
			m.CPU.SetAF(0x6b00)
			m.CPU.SetSP(0xbf00)
			m.CPU.SetPC(0x8000)
			for i := 0; i < 100; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			if m.CPU.HL() != 0x5000+uint16(value) || m.CPU.AF()>>8 != 0x6b || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 {
				t.Fatalf("base%s value%d HL=%04x AF=%04x PC/SP=%04x/%04x", base, value, m.CPU.HL(), m.CPU.AF(), m.CPU.PC(), m.CPU.SP())
			}
		}
	}
}
