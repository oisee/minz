package mir2

import (
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// A copy made in shadow A must not survive as an alias of restored main A.
func TestByteSpillSlotShadowAInvalidatesAliases(t *testing.T) {
	for _, path := range []string{"pair", "spill", "patch"} {
		t.Run(path, func(t *testing.T) {
			g, byteSlot, wordSlot := moveCodegen(t, TyU8, TyU16)
			switch path {
			case "pair":
				g.emitMov(byteSlot, "DE", 16)
			case "spill":
				g.emitMov(byteSlot, wordSlot, 16)
			case "patch":
				g.genInst(&Inst{Op: OpPatchSlot, Dst: Reg(1), Ty: TyU8, Imm: 37, Sym: "move$slot"})
			}
			if g.err != nil {
				t.Fatal(g.err)
			}
			for _, r := range []string{"E", "L"} {
				if g.holdsValue("A", r) || g.holdsValue(r, "A") {
					t.Errorf("stale A/%s alias after %s store: %v\n%s", r, path, g.holdsPhys, g.sb.String())
				}
			}
		})
	}
}

// The next byte ALU operation must load its lhs, rather than trusting the
// alias created inside the shadow bracket and computing with the old main A.
func TestByteSpillSlotShadowAFollowingALU(t *testing.T) {
	for _, path := range []string{"pair", "spill"} {
		t.Run(path, func(t *testing.T) {
			g, byteSlot, wordSlot := moveCodegen(t, TyU8, TyU16)
			lhsLoc := "E"
			if path == "pair" {
				g.emitMov(byteSlot, "DE", 16)
			} else {
				g.emitMov(byteSlot, wordSlot, 16)
				lhsLoc = "L"
			}
			lhs, rhs, dst := g.fn.AllocReg(), g.fn.AllocReg(), g.fn.AllocReg()
			g.ar.Locs[lhs] = PhysLoc{Kind: LocReg, Name: lhsLoc}
			g.ar.Locs[rhs] = PhysLoc{Kind: LocReg, Name: "B"}
			g.ar.Locs[dst] = PhysLoc{Kind: LocReg, Name: "C"}
			inst := &Inst{Op: OpAdd, Dst: dst, Src: [2]Reg{lhs, rhs}, Ty: TyU8}
			g.curBlock = g.fn.NewBlock("alu")
			g.curBlock.Append(inst)
			g.curBlock.Seal(&TermRet{Vals: []Reg{dst}})
			g.genBinOp("ADD", inst)
			if g.err != nil {
				t.Fatal(g.err)
			}
			asm := "ORG 0x8000\n" + g.sb.String() + "DI\nHALT\n" +
				byteSlot + ": DB 0\n" + byteSlot + "_next: DB 0xCD\n" + wordSlot + ": DW 0x1237\n"
			a, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Errors) > 0 {
				t.Fatalf("%v\n%s", a.Errors, asm)
			}
			z := emulator.NewRemogattoZ80()
			if err := z.LoadMemory(0x8000, a.Binary); err != nil {
				t.Fatal(err)
			}
			z.SetRegisters(emulator.Registers{A: 0x90, DE: 0x1237, HL: 0x2452, BC: 0x0300, PC: 0x8000, SP: 0xf000})
			for n := 0; n < 200 && !z.IsHalted(); n++ {
				z.Step()
			}
			want := byte(0x3A)
			if path == "spill" {
				want = 0x55
			}
			if !z.IsHalted() || byte(z.GetRegisters().BC) != want {
				t.Fatalf("halted=%v result=%02x want=%02x\n%s", z.IsHalted(), byte(z.GetRegisters().BC), want, asm)
			}
			if got := z.Memory()[a.Symbols[byteSlot]]; got != 0x37 {
				t.Fatalf("byte slot=%02x, want 37", got)
			}
			if got := z.Memory()[a.Symbols[byteSlot+"_next"]]; got != 0xCD {
				t.Fatalf("neighbour=%02x, want CD", got)
			}
		})
	}
}
