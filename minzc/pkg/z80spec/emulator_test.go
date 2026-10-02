package z80spec_test

import (
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80spec"
)

// Execute real opcodes at a non-ROM address; register getters only observe
// results. A bounded instruction count also prevents a broken program hanging.
func execute(t *testing.T, code []byte, instructions int) *emulator.RemogattoZ80 {
	t.Helper()
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, code); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	z.SetSP(0xf000)
	for i := 0; i < instructions; i++ {
		z.Step()
	}
	if got := z.GetPC(); got != 0x8000+uint16(len(code)) {
		t.Fatalf("executed PC = %04x, want %04x", got, 0x8000+uint16(len(code)))
	}
	return z
}

func assertHalves(t *testing.T, pair, hi, lo string, gotHi, gotLo byte) {
	t.Helper()
	h, l, ok := z80spec.Halves(pair)
	if !ok || h != hi || l != lo || !z80spec.Contains(pair, hi) || !z80spec.Contains(pair, lo) {
		t.Fatalf("spec containment %s = %s/%s, want %s/%s", pair, h, l, hi, lo)
	}
	if gotHi != 0x12 || gotLo != 0x34 {
		t.Fatalf("executed %s halves %s=%02x %s=%02x, want 12/34", pair, hi, gotHi, lo, gotLo)
	}
}

func TestEmulatorContainment(t *testing.T) {
	for _, tc := range []struct {
		pair, hi, lo                                   string
		loadPair, loadHi, loadLo, push, readHi, readLo byte
	}{
		{"BC", "B", "C", 0x01, 0x06, 0x0e, 0xc5, 0x78, 0x79},
		{"DE", "D", "E", 0x11, 0x16, 0x1e, 0xd5, 0x7a, 0x7b},
		{"HL", "H", "L", 0x21, 0x26, 0x2e, 0xe5, 0x7c, 0x7d},
	} {
		t.Run(tc.pair, func(t *testing.T) {
			// LD pair,$1234; LD A,hi; LD A,lo: instruction reads
			// verify the spec's byte order independently of wrapper pair getters.
			hi := execute(t, []byte{tc.loadPair, 0x34, 0x12, tc.readHi}, 2).GetRegisters().A
			lo := execute(t, []byte{tc.loadPair, 0x34, 0x12, tc.readLo}, 2).GetRegisters().A
			assertHalves(t, tc.pair, tc.hi, tc.lo, hi, lo)
			// LD hi,$12; LD lo,$34; PUSH pair; POP BC.
			got := execute(t, []byte{tc.loadHi, 0x12, tc.loadLo, 0x34, tc.push, 0xc1}, 4).GetRegisters().BC
			if got != 0x1234 {
				t.Fatalf("executed half writes: %04x", got)
			}
		})
	}
	for _, tc := range []struct {
		pair, hi, lo string
		prefix       byte
	}{
		{"IX", "IXH", "IXL", 0xdd}, {"IY", "IYH", "IYL", 0xfd},
	} {
		t.Run(tc.pair, func(t *testing.T) {
			// DD/FD 21 loads the pair; DD/FD 7C and 7D read index halves.
			hi := execute(t, []byte{tc.prefix, 0x21, 0x34, 0x12, tc.prefix, 0x7c}, 2).GetRegisters().A
			lo := execute(t, []byte{tc.prefix, 0x21, 0x34, 0x12, tc.prefix, 0x7d}, 2).GetRegisters().A
			assertHalves(t, tc.pair, tc.hi, tc.lo, hi, lo)
			got := execute(t, []byte{tc.prefix, 0x26, 0x12, tc.prefix, 0x2e, 0x34, tc.prefix, 0xe5, 0xc1}, 4).GetRegisters().BC
			if got != 0x1234 {
				t.Fatalf("executed index-half writes: %04x", got)
			}
		})
	}
	t.Run("AF", func(t *testing.T) {
		// Seed AF through the stack, then read it through PUSH AF / POP BC.
		z := execute(t, []byte{0x01, 0x34, 0x12, 0xc5, 0xf1, 0xf5, 0xc1}, 5)
		r := z.GetRegisters()
		assertHalves(t, "AF", "A", "F", byte(r.BC>>8), byte(r.BC))
		if r.A != byte(r.BC>>8) || r.F != byte(r.BC) {
			t.Fatal("AF disagrees with executed stack transfer")
		}
	})
}

func TestEmulatorShadowContainment(t *testing.T) {
	for _, tc := range []struct {
		pair, hi, lo string
		load, push   byte
	}{
		{"BC'", "B'", "C'", 0x01, 0xc5},
		{"DE'", "D'", "E'", 0x11, 0xd5},
		{"HL'", "H'", "L'", 0x21, 0xe5},
	} {
		t.Run(tc.pair, func(t *testing.T) {
			// LD pair,$1234; EXX; LD pair,$abcd; EXX; PUSH pair; POP BC.
			code := []byte{tc.load, 0x34, 0x12, 0xd9, tc.load, 0xcd, 0xab, 0xd9, tc.push, 0xc1}
			got := execute(t, code, 6).GetRegisters().BC
			assertHalves(t, tc.pair, tc.hi, tc.lo, byte(got>>8), byte(got))
			// Read the other bank too, before POP BC can overwrite it.
			code = []byte{tc.load, 0x34, 0x12, 0xd9, tc.load, 0xcd, 0xab, tc.push, 0xc1}
			if other := execute(t, code, 5).GetRegisters().BC; other != 0xabcd {
				t.Fatalf("other bank: %04x", other)
			}
		})
	}
	t.Run("AF'", func(t *testing.T) {
		// Load AF from BC via stack, exchange, load a different AF, exchange
		// back, and observe the preserved shadow bytes through PUSH AF/POP BC.
		code := []byte{0x01, 0x34, 0x12, 0xc5, 0xf1, 0x08, 0x01, 0xcd, 0xab, 0xc5, 0xf1}
		other := execute(t, append(append([]byte{}, code...), 0xf5, 0xc1), 9).GetRegisters().BC
		if other != 0xabcd {
			t.Fatalf("other AF bank: %04x", other)
		}
		got := execute(t, append(code, 0x08, 0xf5, 0xc1), 10).GetRegisters().BC
		assertHalves(t, "AF'", "A'", "F'", byte(got>>8), byte(got))
	})
}

func TestEmulatorSpecialRegisters(t *testing.T) {
	// LD A,$a5; LD I,A; LD A,0; LD A,I.
	z := execute(t, []byte{0x3e, 0xa5, 0xed, 0x47, 0x3e, 0, 0xed, 0x57}, 4)
	if z.GetI() != 0xa5 || z.GetRegisters().A != 0xa5 {
		t.Fatal("I instruction write/read")
	}
	// LD R,A loads all eight bits; instruction fetch increments only low 7.
	for _, value := range []byte{0x7f, 0xff} {
		z = execute(t, []byte{0x3e, value, 0xed, 0x4f}, 2)
		if z.GetR() != value {
			t.Fatalf("LD R,A = %02x, want %02x", z.GetR(), value)
		}
		z.SetMemory(z.GetPC(), 0x00) // NOP, one opcode fetch.
		z.Step()
		if want := value & 0x80; z.GetR() != want {
			t.Fatalf("R 7+1 rollover = %02x, want %02x", z.GetR(), want)
		}
	}
	// ED 5F has two opcode fetches before LD A,R samples the refresh value.
	z = execute(t, []byte{0x3e, 0x90, 0xed, 0x4f, 0xed, 0x5f}, 3)
	if z.GetR() != 0x92 || z.GetRegisters().A != 0x92 {
		t.Fatal("LD A,R instruction read")
	}
}
