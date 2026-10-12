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

func TestAdapterPrefixAndPairCopiesNative(t *testing.T) {
	for _, names := range [][2]string{{"H", "IXH"}, {"IXH", "L"}, {"IYL", "IXH"}, {"HL", "DE"}} {
		from, to := Z80.LocByName(names[0]), Z80.LocByName(names[1])
		asm := "ORG 0x8000\n" + strings.Join(emitAdapterMove(Z80.Locs[from], Z80.Locs[to]), "\n") + "\nRET\n"
		ar, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil || len(ar.Errors) > 0 {
			t.Fatalf("assembly %v %v", err, ar.Errors)
		}
		z := emulator.NewRemogattoZ80()
		if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
			t.Fatal(err)
		}
		for x := 0; x < 65536; x++ {
			in := emulator.Registers{A: byte(x), F: 0x45, BC: 0x1234, DE: 0x5678, HL: uint16(x), IX: ^uint16(x), IY: uint16(x*3 + 17), PC: 0x8000, SP: 0xeffe}
			want := in
			switch names {
			case [2]string{"H", "IXH"}:
				want.IX = uint16(byte(in.HL>>8))<<8 | in.IX&255
			case [2]string{"IXH", "L"}:
				want.HL = in.HL&0xff00 | uint16(byte(in.IX>>8))
			case [2]string{"IYL", "IXH"}:
				want.IX = uint16(byte(in.IY))<<8 | in.IX&255
			case [2]string{"HL", "DE"}:
				want.DE = in.HL
			}
			if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
				t.Fatal(err)
			}
			z.SetRegisters(in)
			for i := 0; i < 64 && z.GetRegisters().PC != 0xf100; i++ {
				z.Step()
			}
			r := z.GetRegisters()
			want.PC = 0xf100
			want.SP = 0xf000
			if r != want {
				t.Fatalf("%v x=%d got=%+v want=%+v", names, x, r, want)
			}
		}
	}
}

func TestAdapterDenseCycleNative(t *testing.T) {
	names := []string{"A", "B", "D", "H", "IXH", "IYH"}
	moves := []adapterMove{}
	for i, name := range names {
		moves = append(moves, adapterMove{Z80.LocByName(name), Z80.LocByName(names[(i+1)%len(names)])})
	}
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
			v := [6]byte{byte(x), byte(y), byte(x*3 + y*7), byte(x*5 + y*11), byte(x*13 + y*17), byte(x*19 + y*23)}
			in := emulator.Registers{A: v[0], F: 0x45, BC: uint16(v[1])<<8 | 0x31, DE: uint16(v[2])<<8 | 0x42, HL: uint16(v[3])<<8 | 0x53, IX: uint16(v[4])<<8 | 0x64, IY: uint16(v[5])<<8 | 0x75, PC: 0x8000, SP: 0xeffe}
			want := in
			want.A = v[5]
			want.BC = uint16(v[0])<<8 | 0x31
			want.DE = uint16(v[1])<<8 | 0x42
			want.HL = uint16(v[2])<<8 | 0x53
			want.IX = uint16(v[3])<<8 | 0x64
			want.IY = uint16(v[4])<<8 | 0x75
			want.PC = 0xf100
			want.SP = 0xf000
			if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
				t.Fatal(err)
			}
			z.SetRegisters(in)
			for i := 0; i < 256 && z.GetRegisters().PC != 0xf100; i++ {
				z.Step()
			}
			if r := z.GetRegisters(); r != want {
				t.Fatalf("x=%d y=%d got=%+v want=%+v", x, y, r, want)
			}
		}
	}
}

func TestStackAdapterMixedWidthsNative(t *testing.T) {
	moves := []adapterMove{{Z80.LocByName("HL"), Z80.LocByName("BC")}, {Z80.LocByName("B"), Z80.LocByName("H")}, {Z80.LocByName("C"), Z80.LocByName("L")}}
	asm := "ORG 0x8000\n" + strings.Join(emitStackAdapter(moves, Z80), "\n") + "\nRET\n"
	ar, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("assembly %v %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 65536; x++ {
		in := emulator.Registers{A: 0x91, F: 0x45, HL: uint16(x), BC: uint16(x*3 + 17), DE: 0x5678, IX: 0x5162, IY: 0x7384, PC: 0x8000, SP: 0xeffe}
		want := in
		want.HL = in.BC
		want.BC = in.HL
		want.PC = 0xf100
		want.SP = 0xf000
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(in)
		for i := 0; i < 256 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		if r := z.GetRegisters(); r != want {
			t.Fatalf("x=%d got=%+v want=%+v", x, r, want)
		}
	}
}
