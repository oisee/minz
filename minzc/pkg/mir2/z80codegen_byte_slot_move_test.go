package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// emitMov's word-context paths used to copy a full word between spill homes
// regardless of the homes' declared widths, so a byte-typed home both lent its
// neighbour's byte as the high byte and had its own neighbour overwritten
// (direct-call fuzz seed 776, where the codegen slot-width check reported it).
// Byte homes must move one byte: widen with zero-extension, narrow by
// truncation, like the register moves in the same helper.
func TestByteSpillSlotMovesStayInsideSlot(t *testing.T) {
	// word ← byte: zero-extend, verified on the emulator.
	t.Run("widen", func(t *testing.T) {
		g, wordSlot, byteSlot := moveCodegen(t, TyU16, TyU8)
		g.emitMov(wordSlot, byteSlot, 16)
		if g.err != nil {
			t.Fatal(g.err)
		}
		assertNoWordCopy(t, g.sb.String(), byteSlot)
		asm := "ORG 0x8000\nentry:\n" + g.sb.String() +
			"    LD HL, (" + wordSlot + ")\n" +
			"    LD (0x6200), HL\n    DI\n    HALT\n" +
			byteSlot + ": DB 0xAB\n" + byteSlot + "_next: DB 0xCD\n" +
			wordSlot + ": DW 0\n"
		a, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil || len(a.Errors) > 0 {
			t.Fatal(err, a.Errors, asm)
		}
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(0x8000, a.Binary)
		z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000})
		for n := 0; n < 200 && !z.IsHalted(); n++ {
			z.Step()
		}
		if !z.IsHalted() {
			t.Fatalf("did not halt\n%s", asm)
		}
		if lo, hi := z.Memory()[0x6200], z.Memory()[0x6201]; lo != 0xAB || hi != 0x00 {
			t.Fatalf("word home = %#02x%02x, want 0x00AB (byte extended, neighbour 0xCD ignored)\n%s", hi, lo, asm)
		}
		if n := z.Memory()[a.Symbols[byteSlot+"_next"]]; n != 0xCD {
			t.Fatalf("neighbour slot clobbered to %#02x\n%s", n, asm)
		}
	})
	// byte ← word: truncate to the low byte.
	t.Run("truncate", func(t *testing.T) {
		g, byteSlot, wordSlot := moveCodegen(t, TyU8, TyU16)
		g.emitMov(byteSlot, wordSlot, 16)
		if g.err != nil {
			t.Fatal(g.err)
		}
		assertNoWordCopy(t, g.sb.String(), byteSlot)
	})
	// byte ← byte in a 16-bit context: a plain byte move.
	t.Run("byte-to-byte", func(t *testing.T) {
		g, byteSlot, otherSlot := moveCodegen(t, TyU8, TyU8)
		g.emitMov(byteSlot, otherSlot, 16)
		if g.err != nil {
			t.Fatal(g.err)
		}
		assertNoWordCopy(t, g.sb.String(), byteSlot)
		assertNoWordCopy(t, g.sb.String(), otherSlot)
	})
}

// emitADDHL and emitSBCHL stage a spilled right operand in BC. A byte home
// must be extended there too — the plain LD BC,(nn) they used to emit took the
// neighbouring slot's byte as the bound's high byte.
func TestByteSpillSlotAddSubExtend(t *testing.T) {
	g, _, byteSlot := moveCodegen(t, TyU16, TyU8)
	g.emit("    OR A")
	g.emitSBCHL(byteSlot)
	g.emitADDHL(byteSlot)
	if g.err != nil {
		t.Fatal(g.err)
	}
	assertNoWordCopy(t, g.sb.String(), byteSlot)
	if !strings.Contains(g.sb.String(), "LD B, 0") {
		t.Fatalf("no zero-extension of the byte home:\n%s", g.sb.String())
	}
}

// moveCodegen builds a codegen whose destination and source vregs live in
// spill homes of the given types, and returns it with both labels.
func moveCodegen(t *testing.T, dstTy, srcTy Ty) (*z80cg, string, string) {
	t.Helper()
	m := &Module{}
	f := m.AddFunc("move")
	b := f.NewBlock("entry")
	dst, src := f.AllocReg(), f.AllocReg()
	b.Append(&Inst{Op: OpConst, Dst: src, Ty: srcTy, Imm: 0})
	b.Append(&Inst{Op: OpConst, Dst: dst, Ty: dstTy, Imm: 0})
	b.Seal(&TermRet{Vals: []Reg{dst}})
	locs := map[Reg]PhysLoc{
		dst: {Kind: LocMem, Name: "dst_mem"},
		src: {Kind: LocMem, Name: "src_mem"},
	}
	for r := range collectRegInfo(f) {
		if _, ok := locs[r]; !ok {
			locs[r] = PhysLoc{Kind: LocReg, Name: "HL"}
		}
	}
	g := &z80cg{mod: m, fn: f, ar: &AllocResult{Locs: locs}, sb: new(strings.Builder), curBlock: b,
		regInfo: collectRegInfo(f), holdsPhys: map[string]string{}, physOverride: map[Reg]string{},
		constVals: map[Reg]int64{}, pendingFlagReg: NoReg, pendingAccReg: NoReg}
	return g, g.spillLabel(dst), g.spillLabel(src)
}

// assertNoWordCopy fails when the move copies a word across a byte home: a
// word read borrows the neighbour as the high byte, a word store clobbers it.
func assertNoWordCopy(t *testing.T, asm, byteSlot string) {
	t.Helper()
	for _, pat := range []string{
		"LD HL, (" + byteSlot + ")", "LD BC, (" + byteSlot + ")",
		"(" + byteSlot + "), HL", "(" + byteSlot + "), BC",
		"(" + byteSlot + "+1)",
	} {
		if strings.Contains(asm, pat) {
			t.Fatalf("word-wide access %q crosses a byte home:\n%s", pat, asm)
		}
	}
}
