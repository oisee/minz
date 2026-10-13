package mir2

import (
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// NEG must honor allocated operands and preserve unrelated live registers.
func TestNegWordAllocatedNative(t *testing.T) {
	for _, src := range []string{"HL", "DE", "BC", "IX", "IY", "spill"} {
		for _, dst := range []string{"HL", "DE", "BC", "IX", "IY", "spill"} {
			f := &Func{Name: "negword"}
			b := f.NewBlock("entry")
			inst := &Inst{Op: OpNeg, Dst: 2, Src: [2]Reg{1}, Ty: TyU16}
			b.Insts = []*Inst{inst}
			b.Term = &TermRet{Vals: []Reg{2, 3, 4}}
			location := func(s string, r Reg) PhysLoc {
				if s == "spill" {
					return PhysLoc{Kind: LocMem}
				}
				return PhysLoc{Kind: LocReg, Name: s}
			}
			var out strings.Builder
			g := &z80cg{fn: f, curBlock: b, sb: &out, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: location(src, 1), 2: location(dst, 2), 3: {Kind: LocReg, Name: "A"}, 4: {Kind: LocReg, Name: "HL"}}}, regInfo: map[Reg]RegInfo{1: {Ty: TyU16}, 2: {Ty: TyU16}, 3: {Ty: TyU8}, 4: {Ty: TyU16}}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, constVals: map[Reg]int64{}, liveness: ComputeLiveness(f)}
			g.genInst(inst)
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + out.String() + "RET\n_spill_negword_r1 EQU 0x6000\n_spill_negword_r2 EQU 0x6002\n")
			if err != nil || len(a.Errors) > 0 {
				t.Fatalf("assemble %s/%s: %v", src, dst, err)
			}
			m, err := spectrum.NewPentagon128(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i, v := range a.Binary {
				m.Memory.Write(0x8000+uint16(i), v, false)
			}
			set := func(s string, v uint16) {
				switch s {
				case "HL":
					m.CPU.SetHL(v)
				case "DE":
					m.CPU.SetDE(v)
				case "BC":
					m.CPU.SetBC(v)
				case "IX":
					m.CPU.SetIX(v)
				case "IY":
					m.CPU.SetIY(v)
				case "spill":
					m.Memory.Write(0x6000, byte(v), false)
					m.Memory.Write(0x6001, byte(v>>8), false)
				}
			}
			get := func(s string) uint16 {
				switch s {
				case "HL":
					return m.CPU.HL()
				case "DE":
					return m.CPU.DE()
				case "BC":
					return m.CPU.BC()
				case "IX":
					return m.CPU.IX()
				case "IY":
					return m.CPU.IY()
				}
				return uint16(m.Memory.Read(0x6002)) | uint16(m.Memory.Read(0x6003))<<8
			}
			for value := 0; value < 65536; value++ {
				m.CPU.SetHL(0xa55a)
				set(src, uint16(value))
				oldHL := m.CPU.HL()
				m.CPU.SetAF(0x6b00)
				m.Memory.Write(0xbf00, 0, false)
				m.Memory.Write(0xbf01, 0x70, false)
				m.CPU.SetPC(0x8000)
				m.CPU.SetSP(0xbf00)
				for i := 0; i < 200; i++ {
					if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
						break
					}
					m.CPU.DoOpcode()
				}
				if get(dst) != 0-uint16(value) || m.CPU.AF()>>8 != 0x6b || dst != "HL" && m.CPU.HL() != oldHL || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 {
					t.Fatalf("%s/%s value%04x result%04x HL%04x old%04x AF%04x PC/SP%04x/%04x", src, dst, value, get(dst), m.CPU.HL(), oldHL, m.CPU.AF(), m.CPU.PC(), m.CPU.SP())
				}
			}
		}
	}
}
