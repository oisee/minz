package z80spec_test

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80spec"
	"testing"
)

func TestEmulatorContainment(t *testing.T) {
	for _, tc := range []struct {
		pair string
		set  func(*emulator.Registers, uint16)
		get  func(emulator.Registers) uint16
	}{
		{"BC", func(r *emulator.Registers, v uint16) { r.BC = v }, func(r emulator.Registers) uint16 { return r.BC }},
		{"DE", func(r *emulator.Registers, v uint16) { r.DE = v }, func(r emulator.Registers) uint16 { return r.DE }},
		{"HL", func(r *emulator.Registers, v uint16) { r.HL = v }, func(r emulator.Registers) uint16 { return r.HL }},
		{"IX", func(r *emulator.Registers, v uint16) { r.IX = v }, func(r emulator.Registers) uint16 { return r.IX }},
		{"IY", func(r *emulator.Registers, v uint16) { r.IY = v }, func(r emulator.Registers) uint16 { return r.IY }},
		{"AF", func(r *emulator.Registers, v uint16) { r.A = uint8(v >> 8); r.F = uint8(v) }, func(r emulator.Registers) uint16 { return uint16(r.A)<<8 | uint16(r.F) }},
	} {
		t.Run(tc.pair, func(t *testing.T) {
			hi, lo, ok := z80spec.Halves(tc.pair)
			if !ok {
				t.Fatal("missing halves")
			}
			z := emulator.NewRemogattoZ80()
			for _, value := range []uint16{0, 0x1234, 0xa5c3, 0xffff} {
				var regs emulator.Registers
				tc.set(&regs, value)
				z.SetRegisters(regs)
				if got := tc.get(z.GetRegisters()); got != value {
					t.Fatalf("pair write: %04x != %04x", got, value)
				}
				// Changing each half also verifies that the opposite half written through
				// SetRegisters remains intact: both directions of containment are exercised.
				if err := z.SetRegister8(hi, 0x69); err != nil {
					t.Fatal(err)
				}
				if got := tc.get(z.GetRegisters()); got != 0x6900|value&0xff {
					t.Errorf("high write: %04x", got)
				}
				z.SetRegisters(regs)
				if err := z.SetRegister8(lo, 0x96); err != nil {
					t.Fatal(err)
				}
				if got := tc.get(z.GetRegisters()); got != value&0xff00|0x96 {
					t.Errorf("low write: %04x", got)
				}
			}
		})
	}
	// The wrapper exposes no shadow-byte setter/getter; shadow containment is
	// checked by the overlap properties and MIR2 DWord comparisons instead.
}
