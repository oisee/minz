package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestAdapterThreeCycleNative(t *testing.T) {
	moves := []adapterMove{{Z80.LocByName("A"), Z80.LocByName("B")}, {Z80.LocByName("B"), Z80.LocByName("C")}, {Z80.LocByName("C"), Z80.LocByName("A")}}
	asm := "ORG 0x8000\n" + strings.Join(resolveParallelMoves(moves, Z80), "\n") + "\nRET\n"
	ar, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("assembly %v %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 256; x++ {
		for y := 0; y < 256; y++ {
			c := byte(x*3 + y*7)
			if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
				t.Fatal(err)
			}
			z.SetRegisters(emulator.Registers{A: byte(x), BC: uint16(y)<<8 | uint16(c), DE: 0x9172, HL: 0x3849, IX: 0x5162, IY: 0x7384, PC: 0x8000, SP: 0xeffe})
			for i := 0; i < 128 && z.GetRegisters().PC != 0xf100; i++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.A != c || byte(r.BC>>8) != byte(x) || byte(r.BC) != byte(y) || r.DE != 0x9172 || r.HL != 0x3849 || r.IX != 0x5162 || r.IY != 0x7384 || r.PC != 0xf100 || r.SP != 0xf000 {
				t.Fatalf("x=%d y=%d: %+v ASM=%s", x, y, r, asm)
			}
		}
	}
}

func TestAdapterWordPairCycleNative(t *testing.T) {
	moves := []adapterMove{{Z80.LocByName("HL"), Z80.LocByName("DE")}, {Z80.LocByName("DE"), Z80.LocByName("BC")}, {Z80.LocByName("BC"), Z80.LocByName("HL")}}
	asm := "ORG 0x8000\n" + strings.Join(resolveParallelMoves(moves, Z80), "\n") + "\nRET\n"
	ar, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("assembly %v %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 65536; x++ {
		y := uint16(x*3 + 17)
		b := uint16(x*7 + 31)
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{A: 91, F: 0x45, HL: uint16(x), DE: y, BC: b, IX: 0x5162, IY: 0x7384, PC: 0x8000, SP: 0xeffe})
		for i := 0; i < 128 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.HL != b || r.DE != uint16(x) || r.BC != y || r.A != 91 || r.F != 0x45 || r.IX != 0x5162 || r.IY != 0x7384 || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x=%d: %+v ASM=%s", x, r, asm)
		}
	}
}

func TestAdapterAllBytePermutationsNative(t *testing.T) {
	for _, perm := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		names := []string{"A", "B", "C"}
		moves := []adapterMove{}
		for i, d := range perm {
			moves = append(moves, adapterMove{Z80.LocByName(names[i]), Z80.LocByName(names[d])})
		}
		asm := "ORG 0x8000\n" + strings.Join(resolveParallelMoves(moves, Z80), "\n") + "\nRET\n"
		ar, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil || len(ar.Errors) > 0 {
			t.Fatalf("assembly: %v %v", err, ar.Errors)
		}
		z := emulator.NewRemogattoZ80()
		if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
			t.Fatal(err)
		}
		for x := 0; x < 256; x++ {
			for y := 0; y < 256; y++ {
				vals := [3]byte{byte(x), byte(y), byte(x*3 + y*7)}
				want := [3]byte{}
				for i, d := range perm {
					want[d] = vals[i]
				}
				if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
					t.Fatal(err)
				}
				z.SetRegisters(emulator.Registers{A: vals[0], F: 0x45, BC: uint16(vals[1])<<8 | uint16(vals[2]), DE: 0x9172, HL: 0x3849, IX: 0x5162, IY: 0x7384, PC: 0x8000, SP: 0xeffe})
				for i := 0; i < 128 && z.GetRegisters().PC != 0xf100; i++ {
					z.Step()
				}
				r := z.GetRegisters()
				if r.A != want[0] || byte(r.BC>>8) != want[1] || byte(r.BC) != want[2] || r.F != 0x45 || r.DE != 0x9172 || r.HL != 0x3849 || r.IX != 0x5162 || r.IY != 0x7384 || r.PC != 0xf100 || r.SP != 0xf000 {
					t.Fatalf("perm=%v x=%d y=%d: %+v", perm, x, y, r)
				}
			}
		}
	}
}
