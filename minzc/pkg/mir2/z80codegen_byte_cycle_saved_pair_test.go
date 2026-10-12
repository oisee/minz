package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func byteCycleAssembly(copies []parallelCopy) string {
	var sb strings.Builder
	g := &z80cg{sb: &sb, holdsPhys: map[string]string{}}
	g.emitParallelCopy(copies)
	return sb.String()
}

func TestNativeByteCycleSavedPairExhaustive(t *testing.T) {
	for _, other := range []string{"B", "C", "D", "E", "H", "L"} {
		asm := byteCycleAssembly([]parallelCopy{{srcName: "A", dstName: other, ty: TyU8}, {srcName: other, dstName: "A", ty: TyU8}})
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm + "DI\nHALT\n")
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		if len(bin.Binary) != 7 {
			t.Fatalf("saved pair shuffle must be 5B: %s", asm)
		}
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		z.LoadMemory(0x8000, bin.Binary)
		for x := 0; x < 65536; x++ {
			r := checksumCopyRegisters()
			r.A = byte(x)
			r.F = byte(x >> 8)
			v := byte(x >> 8)
			switch other {
			case "B":
				r.BC = uint16(v)<<8 | uint16(byte(r.BC))
			case "C":
				r.BC = r.BC&0xff00 | uint16(v)
			case "D":
				r.DE = uint16(v)<<8 | uint16(byte(r.DE))
			case "E":
				r.DE = r.DE&0xff00 | uint16(v)
			case "H":
				r.HL = uint16(v)<<8 | uint16(byte(r.HL))
			case "L":
				r.HL = r.HL&0xff00 | uint16(v)
			}
			want := checksumCopyBytes(r)
			want["A"], want[other] = want[other], want["A"]
			z.SetRegisters(r)
			start := z.GetCycles()
			for i := 0; i < 5; i++ {
				z.Step()
			}
			got := z.GetRegisters()
			for name, b := range checksumCopyBytes(got) {
				if b != want[name] {
					t.Fatalf("%s/%04x %s=%02x want%02x ASM%s", other, x, name, b, want[name], asm)
				}
			}
			if got.F != r.F || got.SP != r.SP || z.GetCycles()-start != 33 {
				t.Fatalf("flags/stack/cost %+v: %s", got, asm)
			}
			if z.GetMemory(0xfefd) != 0 || z.GetMemory(0xff00) != 0 {
				t.Fatal("stack guard changed")
			}
		}
		t.Logf("A/%s all65536 values, all256 F, main/index regs and SP preserved; 33T/5B", other)
	}
}

func TestNativeByteCyclePermutationsAndFallback(t *testing.T) {
	var cases [][]parallelCopy
	var permute func([]string, []string)
	permute = func(prefix, rest []string) {
		if len(rest) == 0 {
			var c []parallelCopy
			for i, s := range []string{"A", "B", "C", "D"} {
				c = append(c, parallelCopy{srcName: s, dstName: prefix[i], ty: TyU8})
			}
			cases = append(cases, c)
			return
		}
		for i, n := range rest {
			next := append([]string{}, rest[:i]...)
			next = append(next, rest[i+1:]...)
			permute(append(append([]string{}, prefix...), n), next)
		}
	}
	permute(nil, []string{"A", "B", "C", "D"})
	cases = append(cases,
		[]parallelCopy{{srcName: "A", dstName: "C", ty: TyU8}, {srcName: "C", dstName: "A", ty: TyU8}, {srcName: "DE", dstName: "HL", ty: TyU16}, {srcName: "HL", dstName: "DE", ty: TyU16}},
		[]parallelCopy{{srcName: "A", dstName: "C", ty: TyU8}, {srcName: "C", dstName: "A", ty: TyU8}, {srcName: "DE", dstName: "DE", ty: TyU16}, {srcName: "HL", dstName: "HL", ty: TyU16}},
		[]parallelCopy{{srcName: "A", dstName: "C", ty: TyU8}, {srcName: "C", dstName: "A", ty: TyU8}, {srcName: "D", dstName: "E", ty: TyU8}, {srcName: "H", dstName: "L", ty: TyU8}},
		[]parallelCopy{{srcName: "A", dstName: "C", ty: TyU8}, {srcName: "C", dstName: "A", ty: TyU8}, {dstName: "DE", ty: TyU16, isImm: true, immVal: 0x8172}},
	)
	for i, copies := range cases {
		for x := 0; x < 256; x++ {
			r := checksumCopyRegisters()
			r.A = byte(x)
			r.F = byte(x)
			r.BC = uint16(x*40503 + 137)
			r.DE = uint16(x*319 + 911)
			old := checksumCopyBytes(r)
			want := checksumCopyBytes(r)
			for _, c := range copies {
				if c.ty.Width() > 8 {
					if c.isImm {
						want[highByte(c.dstName)] = byte(c.immVal >> 8)
						want[lowByte(c.dstName)] = byte(c.immVal)
					} else {
						want[highByte(c.dstName)] = old[highByte(c.srcName)]
						want[lowByte(c.dstName)] = old[lowByte(c.srcName)]
					}
				} else {
					want[c.dstName] = old[c.srcName]
				}
			}
			asm := byteCycleAssembly(copies)
			z := runChecksumCopyAsm(t, asm, r)
			got := z.GetRegisters()
			for n, v := range checksumCopyBytes(got) {
				if v != want[n] {
					t.Fatalf("case%d/%d %s got%02x want%02x ASM%s", i, x, n, v, want[n], asm)
				}
			}
			if got.F != r.F || got.SP != r.SP {
				t.Fatalf("case%d flags/SP %+v ASM%s", i, got, asm)
			}
		}
	}
	asm := "EX AF,AF'\nLD HL,0x6dd3\nPUSH HL\nPOP AF\nEX AF,AF'\nLD HL,0x9abc\n" + byteCycleAssembly([]parallelCopy{{srcName: "A", dstName: "C", ty: TyU8}, {srcName: "C", dstName: "A", ty: TyU8}}) + "EX AF,AF'\nPUSH AF\nPOP IX\nEX AF,AF'\n"
	got := runChecksumCopyAsm(t, asm, checksumCopyRegisters()).GetRegisters()
	if got.IX != 0x6dd3 || got.A != 0x34 || got.BC != 0x1297 || got.F != 0x45 || got.SP != 0xff00 {
		t.Fatalf("shadow/live preservation %+v", got)
	}
}

func TestNativeByteCycleMeasuredBeforeAfter(t *testing.T) {
	// Retained exact old A/C snapshot sequence, measured with the same emulator.
	old := "PUSH AF\nEX AF,AF'\nPOP AF\nEX AF,AF'\nPUSH AF\nEX AF,AF'\nLD A,C\nPUSH AF\nEX AF,AF'\nPOP AF\nEX AF,AF'\nPOP AF\nLD C,A\nEX AF,AF'\n"
	current := byteCycleAssembly([]parallelCopy{{srcName: "A", dstName: "C", ty: TyU8}, {srcName: "C", dstName: "A", ty: TyU8}})
	for _, tc := range []struct {
		name, asm  string
		cost, size int
	}{{"old", old, 95, 14}, {"saved-pair", current, 33, 5}} {
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + tc.asm + "DI\nHALT\n")
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		z := runChecksumCopyAsm(t, tc.asm, checksumCopyRegisters())
		r := z.GetRegisters()
		if r.A != 0x34 || r.BC != 0x1297 || r.F != 0x45 || r.SP != 0xff00 || z.GetCycles()-8 != tc.cost || len(bin.Binary)-2 != tc.size {
			t.Fatalf("%s cost%d bytes%d regs%+v", tc.name, z.GetCycles()-8, len(bin.Binary)-2, r)
		}
		t.Logf("%s exact emitted A/C shuffle %dT/%dB", tc.name, tc.cost, tc.size)
	}
}
