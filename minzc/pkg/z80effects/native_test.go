package z80effects

import (
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func registers(c spectrum.CPUCore) map[Registers]byte {
	result := map[Registers]byte{I: c.I()}
	for _, p := range []struct {
		r Registers
		v uint16
	}{
		{AF, c.AF()}, {BC, c.BC()}, {DE, c.DE()}, {HL, c.HL()}, {IX, c.IX()}, {IY, c.IY()},
		{AFAlt, c.AF_()}, {BCAlt, c.BC_()}, {DEAlt, c.DE_()}, {HLAlt, c.HL_()},
	} {
		var bits []Registers
		for bit := Registers(1); bit <= LAlt; bit <<= 1 {
			if p.r&bit != 0 {
				bits = append(bits, bit)
			}
		}
		// Resource order is high/low for pairs including index registers and banks.
		result[bits[0]], result[bits[1]] = byte(p.v>>8), byte(p.v)
	}
	return result
}
func TestDeclaredEffectsCoverNativeChanges(t *testing.T) {
	// Independent execution covers each declared non-write and every untouched
	// flag bit; this is not a snapshot of the decoder's own expected output.
	for _, code := range []string{
		"LD A, B", "LD DE, 0x3456", "LD IXH, A", "LD A, IXL", "LD H, (IX+1)", "LD (IX+1), H", "ADD A, IYH", "INC IXL", "LD (HL), A", "LD A, (HL)", "LD SP, 0x5800",
		"LD A, I", "LD I, A", "LD A, R", "PUSH DE", "POP AF", "POP IX",
		"INC B", "DEC C", "INC HL", "DEC SP", "DJNZ 0x8000",
		"ADD A, C", "ADC A, D", "SUB B", "SBC A, E", "AND B", "OR C", "XOR B", "CP E",
		"ADD HL, BC", "ADD IX, SP", "ADC HL, DE", "SBC HL, BC",
		"NEG", "CPL", "DAA", "SCF", "CCF", "RLCA", "RRCA", "RLA", "RRA",
		"RL B", "RRC C", "SLA D", "SRA E", "SRL H", "BIT 3, L", "SET 2, B", "RES 1, C",
		"EX DE, HL", "EX AF, AF'", "EXX", "EX (SP), HL", "LDI", "LDD", "CPI", "CPD",
		"DI", "EI", "HALT", "JP Z, 0x8100", "JR NC, 0x8000", "RET Z",
		"IN A, (0xfe)", "IN B, (C)", "OUT (0xfe), A",
	} {
		t.Run(code, func(t *testing.T) {
			effect := Instruction(code)
			if !effect.Known {
				t.Fatalf("unsupported positive: %+v", effect)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + code)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors)
			}
			machine, err := spectrum.New48K(make([]byte, 16384))
			if err != nil {
				t.Fatal(err)
			}
			for seed := 0; seed < 256; seed++ {
				c := machine.CPU
				c.Reset()
				c.SetPC(0x8000)
				c.SetSP(0x9000)
				c.SetAF(uint16(byte(seed*37+3))<<8 | uint16(byte(seed)))
				c.SetBC(uint16(byte(seed*11+7))<<8 | uint16(byte(seed*17+1)))
				c.SetDE(0x6100 + uint16(seed))
				c.SetHL(0x6000 + uint16(seed))
				c.SetIX(0x6200 + uint16(seed))
				c.SetIY(0x6300 + uint16(seed))
				c.SetAF_(uint16(seed*257) ^ 0xa55a)
				c.SetBC_(0xabc0 + uint16(seed))
				c.SetDE_(0xdef0 - uint16(seed))
				c.SetHL_(0x1230 + uint16(seed))
				c.SetI(byte(seed))
				c.SetR(byte(seed))
				c.SetIFF1(seed&1 != 0)
				c.SetIFF2(seed&1 != 0)
				for i, b := range a.Binary {
					machine.Memory.Write(0x8000+uint16(i), b, false)
				}
				for i := 0x6000; i < 0x6400; i++ {
					machine.Memory.Write(uint16(i), byte(i*31+seed), false)
				}
				machine.Memory.Write(0x9000, byte(seed*13), false)
				machine.Memory.Write(0x9001, byte(seed*19), false)
				before := registers(c)
				sp := c.SP()
				iff1, iff2 := c.IFF1(), c.IFF2()
				c.DoOpcode()
				after := registers(c)
				for r, v := range before {
					if after[r] != v && effect.Writes&r == 0 {
						t.Fatalf("seed %d undeclared change %x: %02x -> %02x; %+v", seed, r, v, after[r], effect)
					}
				}
				if changed := byte(c.AF()) ^ before[F]; changed & ^effect.FlagsWrite != 0 {
					t.Fatalf("seed %d untouched flags changed: %02x, declared %02x", seed, changed, effect.FlagsWrite)
				}
				if c.SP() != sp && effect.Writes&SP == 0 {
					t.Fatalf("undeclared SP change")
				}
				if effect.INT == INTUnchanged && (iff1 != c.IFF1() || iff2 != c.IFF2()) {
					t.Fatalf("undeclared INT change")
				}
			}
		})
	}
}
