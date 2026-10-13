package vir

import (
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCheckedCompositionMoveBridgeNative(t *testing.T) {
	for _, source := range []string{"C", "D", "E"} {
		f, allocation := productionTableFunction(mir2.ClassAcc)
		p, err := PrepareLocalTableProblem(f, allocation)
		if err != nil {
			t.Fatal(err)
		}
		var pool, target RegAllocTable
		pool.Init()
		target.Init()
		producer := map[int]int{p.Ops[0].Dst: Z80.LocByName(source)}
		if err = pool.AddChecked(p.Ops[:1], producer, Z80, p.Options); err != nil {
			t.Fatal(err)
		}
		if n, err := pool.ComposeCheckedCandidates(p, &target, []int{1}, 32); err != nil || n != 0 {
			t.Fatal("missing move proposal admitted", n, err)
		}
		n, err := pool.ComposeCheckedCandidatesWithMoves(p, &target, []int{1}, 32)
		if err != nil || n != 1 {
			t.Fatal(source, n, err)
		}
		_, cost, ok := target.LookupChecked(p.Ops, Z80, p.Options)
		if !ok || cost != 21 {
			t.Fatal("bridge cost omitted", source, cost, ok)
		}
		asm, err := p.Emit(&target)
		if err != nil {
			t.Fatal(err)
		}
		bin, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil {
			t.Fatal(err)
		}
		if len(bin.Errors) > 0 {
			t.Fatal(bin.Errors)
		}
		for flags := 0; flags < 256; flags++ {
			// Independent full F byte. The bridge is a byte LD and must preserve F,
			// untargeted register halves, return PC and the caller's stack pointer.
			boot := []byte{0x21, byte(flags), 0x55, 0xe5, 0xf1, 0x01, 0x12, 0x34, 0x11, 0x56, 0x78, 0xcd, 16, 0x80, 0xf3, 0x76}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 1000
			z.LoadMemory(0x8000, append(boot, bin.Binary...))
			z.SetPC(0x8000)
			z.SetSP(0xf000)
			if err := z.Run(); err != nil {
				t.Fatal(err)
			}
			r := z.GetRegisters()
			bc, de := uint16(0x3412), uint16(0x7856)
			switch source {
			case "C":
				bc = 0x3425
			case "D":
				de = 0x2556
			case "E":
				de = 0x7825
			}
			if r.A != 37 || r.F != byte(flags) || r.BC != bc || r.DE != de || r.SP != 0xf000 || r.PC != 0x800f || z.GetCycles() != 97 {
				t.Fatalf("%s flags=%02x: %+v T=%d ASM=%s", source, flags, r, z.GetCycles(), asm)
			}
		}
		t.Logf("%s -> A existing move: 21 admitted T-states; 256 complete flags bytes, register halves and PC/SP PASS", source)
		before := target.Size()
		if _, err := pool.ComposeCheckedCandidatesWithMoves(p, &target, []int{1}, 1); err == nil || target.Size() != before {
			t.Fatal("bound failure changed target", err)
		}
		p.Ops[1].Src[0] = 999
		if _, err := pool.ComposeCheckedCandidatesWithMoves(p, &target, []int{1}, 32); err == nil || target.Size() != before {
			t.Fatal("forged bridge accepted", err)
		}
	}
}

func TestCheckedCompositionWordMoveBridgeNative(t *testing.T) {
	for _, source := range []string{"BC", "DE", "IX", "IY"} {
		for _, value := range []uint16{0, 1, 0x80ff, 0xffff} {
			f, allocation := productionTableFunction(mir2.ClassPointer)
			f.Contract.Returns[0].Ty = mir2.TyU16
			f.Blocks[0].Insts[0].Ty = mir2.TyU16
			f.Blocks[0].Insts[0].Imm = int64(value)
			p, err := PrepareLocalTableProblem(f, allocation)
			if err != nil {
				t.Fatal(err)
			}
			var pool, target RegAllocTable
			pool.Init()
			target.Init()
			if err := pool.AddChecked(p.Ops[:1], map[int]int{p.Ops[0].Dst: Z80.LocByName(source)}, Z80, p.Options); err != nil {
				t.Fatal(err)
			}
			if n, err := pool.ComposeCheckedCandidatesWithMoves(p, &target, []int{1}, 32); err != nil || n != 1 {
				t.Fatal(source, n, err)
			}
			asm, err := p.Emit(&target)
			if err != nil {
				t.Fatal(err)
			}
			bin, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil {
				t.Fatal(err)
			}
			if len(bin.Errors) > 0 {
				t.Fatal(bin.Errors)
			}
			for flags := 0; flags < 256; flags++ {
				boot := []byte{0x21, byte(flags), 0x55, 0xe5, 0xf1, 0xcd, 10, 0x80, 0xf3, 0x76}
				z := emulator.NewRemogattoZ80()
				z.MaxCycles = 1000
				z.LoadMemory(0x8000, append(boot, bin.Binary...))
				z.SetPC(0x8000)
				z.SetSP(0xf000)
				if err := z.Run(); err != nil {
					t.Fatal(err)
				}
				r := z.GetRegisters()
				if r.HL != value || r.F != byte(flags) || r.A != 0x55 || r.SP != 0xf000 || r.PC != 0x8009 {
					t.Fatalf("%s value=%04x flags=%02x: %+v ASM=%s", source, value, flags, r, asm)
				}
			}
		}
	}
	t.Log("4096 word-move native checks: BC/DE/IX/IY to HL, boundary words, complete flags and PC/SP PASS")
}
