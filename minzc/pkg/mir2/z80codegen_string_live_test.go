package mir2

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// Exercise the emitted printer with actual allocated homes, including a spilled
// pointer. All inputs remain semantically live via a multi-value return. The
// native fragment must preserve every register/flag and all non-stack memory.
func TestStringAndNewlinePhysicalLiveValues(t *testing.T) {
	for _, sym := range []string{"@mir.io.print.str", "@mir.io.print.nl"} {
		homes := []string{"BC", "DE", "HL", "IX", "IY", "spill"}
		if sym == "@mir.io.print.nl" {
			homes = []string{"HL"}
		}
		for _, home := range homes {
			t.Run(sym+"/"+home, func(t *testing.T) {
				m, ar, inst := decimalPrintFixture(sym, home)
				m.Funcs[0].Contract.Params[0].Ty = TyPtr
				if sym == "@mir.io.print.nl" {
					inst.Args = nil
				}
				g, out := decimalPrintGenerator(m, ar)
				if err := g.genCall(inst); err != nil {
					t.Fatal(err)
				}
				asm := "ORG 0x8000\ndecimal_fixture:\n" + out.String() + "done: HALT\n_spill_decimal_fixture_r1: DW 0\n"
				a, e := z80asm.NewAssembler().AssembleString(asm)
				if e != nil || len(a.Errors) > 0 {
					t.Fatal(e, a.Errors, asm)
				}
				for _, ptr := range []uint16{0x6000, 0xffff} {
					for v := 0; v < 256; v++ {
						for _, sp := range []uint16{0xe000, 0xf000} {
							z := emulator.NewRemogattoZ80WithScreen()
							z.SetROMEnd(0)
							z.LoadMemory(0x8000, a.Binary)
							r := emulator.Registers{PC: 0x8000, SP: sp, A: byte(v), F: byte(v), BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468}
							switch home {
							case "BC":
								r.BC = ptr
							case "DE":
								r.DE = ptr
							case "HL":
								r.HL = ptr
							case "IX":
								r.IX = ptr
							case "IY":
								r.IY = ptr
							case "spill":
								p := a.Symbols["_spill_decimal_fixture_r1"]
								z.Memory()[p] = byte(ptr)
								z.Memory()[p+1] = byte(ptr >> 8)
							}
							z.Memory()[ptr] = byte(v)
							z.Memory()[uint16(ptr+1)] = 0
							z.SetRegisters(r)
							before := append([]byte(nil), z.Memory()...)
							var stdout bytes.Buffer
							z.SetIOHandlers(func(p uint16) byte { t.Fatal("unexpectedIN", p); return 0 }, func(p uint16, v byte) {
								if byte(p) != 0x23 || byte(p>>8) != v {
									t.Fatal("port", p, v)
								}
								stdout.WriteByte(v)
							})
							minSP := sp
							done := false
							for i := 0; i < 1000; i++ {
								if z.GetPC() == uint16(a.Symbols["done"]) {
									done = true
									break
								}
								if z.GetPC() < 0x8000 || int(z.GetPC()) >= 0x8000+len(a.Binary) {
									t.Fatal("foreignPC", z.GetPC())
								}
								z.Step()
								if z.GetSP() < minSP {
									minSP = z.GetSP()
								}
							}
							want := []byte{byte(v)}
							if v == 0 {
								want = nil
							}
							if sym == "@mir.io.print.nl" {
								want = []byte{'\n'}
							}
							got := z.GetRegisters()
							got.PC = r.PC
							if !done || !bytes.Equal(stdout.Bytes(), want) || got != r {
								t.Fatalf("%s %s v%d ptr%x SP%x output%q want%q regs%+v want%+v asm%s", sym, home, v, ptr, sp, stdout.Bytes(), want, got, r, asm)
							}
							for p, b := range before {
								if p >= int(minSP) && p < int(sp) {
									continue
								}
								if z.Memory()[p] != b {
									t.Fatal("memory changed", p)
								}
							}
						}
					}
				}
			})
		}
	}
}

func TestStringPrintRejectsMalformedArguments(t *testing.T) {
	m, ar, i := decimalPrintFixture("@mir.io.print.str", "HL")
	g, _ := decimalPrintGenerator(m, ar)
	i.Args = nil
	if e := g.genCall(i); e == nil {
		t.Fatal("missing pointer accepted")
	}
	i.Args = []Reg{m.Funcs[0].Contract.Params[0].Reg}
	ar.Locs[i.Args[0]] = PhysLoc{Kind: LocFlag, Name: "F"}
	g, _ = decimalPrintGenerator(m, ar)
	if e := g.genCall(i); e == nil || !strings.Contains(fmt.Sprint(e), "flag") {
		t.Fatal("flag pointer accepted", e)
	}
}
