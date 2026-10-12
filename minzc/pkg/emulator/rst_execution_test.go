package emulator

import (
	"bytes"
	"fmt"
	"testing"
)

func TestScreenRAMStoresDoNotDispatchRST(t *testing.T) {
	for _, old := range []byte{0xc7, 0xcf, 0xd7, 0xdf, 0xe7, 0xef, 0xf7, 0xff} {
		t.Run(fmt.Sprintf("old%02x", old), func(t *testing.T) {
			z := NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			calls := 0
			mark := func() { calls++ }
			z.Hooks = &Z80Hooks{OnRST00: mark, OnRST08: mark, OnRST10: func(byte) { mark() }, OnRST18: func() byte { mark(); return 0x12 }, OnRST20: func() byte { mark(); return 0x34 }, OnRST28: mark, OnRST30: mark, OnRST38: mark}
			code := []byte{0x77, 0xc9} // LD(HL),A;RET: no flags changed.
			z.LoadAt(0x8000, code)
			z.Memory()[0x9000] = old
			z.Memory()[0xffff] = 0x6b
			z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
			r := Registers{A: 0x58, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9000, IX: 0x2468, IY: 0x369c, PC: 0x8000, SP: 0xeff0}
			z.SetRegisters(r)
			if got := z.Step(); got != 7 {
				t.Fatalf("store cycles%d", got)
			}
			want := r
			want.PC++
			if got := z.GetRegisters(); got != want || calls != 0 || z.Memory()[0x9000] != r.A || z.Memory()[0xffff] != 0x6b {
				t.Fatalf("store got%+v want%+v callbacks%d data=%02x ffff=%02x", got, want, calls, z.Memory()[0x9000], z.Memory()[0xffff])
			}
			z.Step()
			want.PC = 0x7000
			want.SP += 2
			if got := z.GetRegisters(); got != want || !bytes.Equal(z.Memory()[0x8000:0x8002], code) {
				t.Fatalf("return got%+v want%+v", got, want)
			}
		})
	}
}

func TestScreenRSTExecutedHooksStepAndRun(t *testing.T) {
	for _, run := range []bool{false, true} {
		for _, opcode := range []byte{0xc7, 0xcf, 0xd7, 0xdf, 0xe7, 0xef, 0xf7, 0xff} {
			t.Run(fmt.Sprintf("op%02x/run%t", opcode, run), func(t *testing.T) {
				z := NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				calls := 0
				mark := func() { calls++ }
				z.Hooks = &Z80Hooks{OnRST00: mark, OnRST08: mark, OnRST10: func(a byte) {
					if a != 0x58 {
						t.Errorf("print A%02x", a)
					}
					mark()
				}, OnRST18: func() byte { mark(); return 0x12 }, OnRST20: func() byte { mark(); return 0x34 }, OnRST28: mark, OnRST30: mark, OnRST38: mark}
				z.LoadAt(0x8000, []byte{opcode, 0xf3, 0x76})
				z.Memory()[0xffff] = 0x6b
				z.Memory()[0xeff0] = 0xab
				z.Memory()[0xeff1] = 0xcd
				r := Registers{A: 0x58, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9000, IX: 0x2468, IY: 0x369c, PC: 0x8000, SP: 0xeff0}
				z.SetRegisters(r)
				z.cpu.R = 0x7f
				want := r
				want.PC++
				if opcode == 0xdf {
					want.A = 0x12
				}
				if opcode == 0xe7 {
					want.A = 0x34
				}
				if run {
					if e := z.Execute(); e != nil {
						t.Fatal(e)
					}
					if opcode != 0xff {
						want.PC = 0x8002 // CPU HALT convention holds PC at HALT opcode.
					}
					if z.GetCycles() != 11 && opcode == 0xff {
						t.Fatalf("exit cycles%d", z.GetCycles())
					}
					if opcode != 0xff && z.GetCycles() != 19 {
						t.Fatalf("run cycles%d", z.GetCycles())
					}
				} else {
					if n := z.Step(); n != 11 {
						t.Fatalf("RST cycles%d", n)
					}
				}
				if z.cpu.R != 0 && !run {
					t.Fatalf("RST refresh=%d want0", z.cpu.R)
				}
				if got := z.GetRegisters(); got != want || calls != 1 || z.Memory()[0xffff] != 0x6b || z.Memory()[0xeff0] != 0xab || z.Memory()[0xeff1] != 0xcd {
					t.Fatalf("hook got%+v want%+v calls%d ffff%02x", got, want, calls, z.Memory()[0xffff])
				}
			})
		}
	}
}

func TestScreenUnhandledRSTUsesCPUStack(t *testing.T) {
	z := NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	z.Hooks = &Z80Hooks{}
	z.LoadAt(0x8000, []byte{0xdf})
	z.Memory()[0x18] = 0xc9
	r := Registers{A: 0x58, F: 0xa5, PC: 0x8000, SP: 0xeff0}
	z.SetRegisters(r)
	if n := z.Step(); n != 11 {
		t.Fatalf("RST cycles%d", n)
	}
	if z.GetPC() != 0x18 || z.GetSP() != 0xefee || z.Memory()[0xefee] != 1 || z.Memory()[0xefef] != 0x80 {
		t.Fatal("RST hardware stack/vector")
	}
	if n := z.Step(); n != 10 {
		t.Fatalf("RET cycles%d", n)
	}
	r.PC++
	if z.GetRegisters() != r {
		t.Fatalf("return%+v", z.GetRegisters())
	}
}

func TestScreenRSTPatchOnlyDispatchesOnExecution(t *testing.T) {
	z := NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	calls := 0
	z.Hooks = &Z80Hooks{OnRST18: func() byte { calls++; return 0x42 }}
	z.LoadAt(0x8000, []byte{0xdf})
	z.WriteMemory(0x8000, 0)
	if calls != 0 {
		t.Fatal("patch triggeredRST")
	}
	z.SetRegisters(Registers{PC: 0x8000, SP: 0xeff0, A: 0x58, F: 0xa5})
	if n := z.Step(); n != 4 || calls != 0 {
		t.Fatal("NOP dispatchedRST")
	}
	z.WriteMemory(0x8000, 0xdf)
	if calls != 0 {
		t.Fatal("patch triggeredRST")
	}
	z.SetPC(0x8000)
	if n := z.Step(); n != 11 || calls != 1 || z.GetRegisters().A != 0x42 || z.GetSP() != 0xeff0 {
		t.Fatal("executed patchedRST")
	}
}

func TestScreenPrefixedRSTKeepsHardwareExecution(t *testing.T) {
	for _, prefix := range []byte{0xdd, 0xfd} {
		z := NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		calls := 0
		z.Hooks = &Z80Hooks{OnRST18: func() byte { calls++; return 0x42 }}
		z.LoadAt(0x8000, []byte{prefix, 0xdf})
		z.Memory()[0x18] = 0xc9
		r := Registers{PC: 0x8000, SP: 0xeff0, A: 0x58, F: 0xa5}
		z.SetRegisters(r)
		plain := NewRemogattoZ80()
		plain.memory.SetROMEnd(0)
		plain.LoadMemory(0x8000, []byte{prefix, 0xdf})
		plain.Memory()[0x18] = 0xc9
		plain.SetRegisters(r)
		got, want := z.Step(), plain.Step()
		if got != want || z.GetRegisters() != plain.GetRegisters() || calls != 0 {
			t.Fatalf("prefix%02x got%+v %d want%+v %d hooks%d", prefix, z.GetRegisters(), got, plain.GetRegisters(), want, calls)
		}
	}
}
