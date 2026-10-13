package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestAdapterStackFrameWrapNative(t *testing.T) {
	names := []string{"A", "B", "D", "H", "IXH", "IYH"}
	moves := []adapterMove{}
	for i, n := range names {
		moves = append(moves, adapterMove{Z80.LocByName(n), Z80.LocByName(names[(i+1)%6])})
	}
	asm := "ORG 0x8000\n" + strings.Join(resolveParallelMoves(moves, Z80), "\n") + "\nRET\n"
	ar, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("%v %v", err, ar.Errors)
	}
	for _, sp := range []uint16{0, 1, 2, 23, 24, 25, 0x4000, 0xfffe, 0xffff} {
		t.Run(fmt.Sprintf("sp_%04x", sp), func(t *testing.T) {
			platform := emulator.NewRemogattoZ80WithScreen()
			platform.SetROMEnd(0)
			z := platform.RemogattoZ80 // use CPU Step directly, no screen/RST dispatch
			for addr := 0; addr < 65536; addr++ {
				z.SetMemory(uint16(addr), byte(addr*37+19))
			}
			if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
				t.Fatal(err)
			}
			z.SetMemory(sp, 0)
			z.SetMemory(sp+1, 0xf1)
			before := append([]byte(nil), z.Memory()...)
			profiler := emulator.NewProfiler()
			z.SetProfiler(profiler)
			for x := 0; x < 256; x++ {
				in := emulator.Registers{A: byte(x), F: byte(x), BC: uint16(x*257 + 13), DE: uint16(x*509 + 27), HL: uint16(x*997 + 39), IX: uint16(x*2053 + 51), IY: uint16(x*4093 + 63), PC: 0x8000, SP: sp}
				want := in
				want.A = byte(in.IY >> 8)
				want.BC = uint16(in.A)<<8 | in.BC&255
				want.DE = in.BC&0xff00 | in.DE&255
				want.HL = in.DE&0xff00 | in.HL&255
				want.IX = in.HL&0xff00 | in.IX&255
				want.IY = in.IX&0xff00 | in.IY&255
				want.PC = 0xf100
				want.SP = sp + 2
				z.SetRegisters(in)
				for steps := 0; steps < 256 && z.GetRegisters().PC != 0xf100; steps++ {
					z.Step()
				}
				if got := z.GetRegisters(); got != want {
					t.Fatalf("x=%d got=%+v want=%+v", x, got, want)
				}
			}
			for addr := 0; addr < 65536; addr++ {
				// Modular interval [SP-24,SP), including a wrapped frame.
				distance := uint16(addr) - (sp - 24)
				if distance < 24 {
					if profiler.WriteCount[addr] == 0 {
						t.Fatalf("unexercised frame %04x", addr)
					}
					continue
				}
				if profiler.WriteCount[addr] != 0 || z.GetMemory(uint16(addr)) != before[addr] {
					t.Fatalf("outside frame write at %04x", addr)
				}
			}
		})
	}
}
