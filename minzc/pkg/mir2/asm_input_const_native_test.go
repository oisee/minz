package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// An inline-asm input operand is a use that reads the physical register the
// allocator chose for it; no codegen peephole can serve it from an immediate.
// computeDeadConsts used to inventory only Src/Args/terminator operands, so a
// constant whose single use was an asm input counted as dead and its LD was
// suppressed — the template then read whatever the register happened to hold.
func TestAsmInputConstMaterializedNative(t *testing.T) {
	// Byte input: the constant's only use is the (in k) operand.
	t.Run("byte", func(t *testing.T) {
		m := &mir2.Module{Name: "asmconst"}
		f := m.AddFunc("asmconst")
		f.Attrs.HasAsm = true
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		k := b.Const(0x5A, mir2.TyU8, mir2.ClassGeneral)
		r := b.Asm("LD A, C", []mir2.Reg{k}, nil, mir2.AllClasses, mir2.TyU8, mir2.ClassAcc)
		b.Ret(r)
		locs := map[mir2.Reg]mir2.PhysLoc{
			k: {Kind: mir2.LocReg, Name: "C"},
			r: {Kind: mir2.LocReg, Name: "A"},
		}
		asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
		// A is cleared first so a suppressed LD C,0x5A cannot hide behind a
		// stale accumulator holding the same value.
		regs := fix1Execute(t, "ORG 0x8000\nLD SP,0xFF00\nXOR A\nCALL asmconst\nDI\nHALT\n"+asm)
		if regs.A != 0x5A {
			t.Fatalf("asm input constant not materialised: A=%d want 90\n%s", regs.A, asm)
		}
	})

	// Word input: the pair home must be loaded before the template runs.
	t.Run("word", func(t *testing.T) {
		m := &mir2.Module{Name: "asmconst16"}
		f := m.AddFunc("asmconst16")
		f.Attrs.HasAsm = true
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		k := b.Const(0x1234, mir2.TyU16, mir2.ClassPair)
		r := b.Asm("LD H, B\n    LD L, C", []mir2.Reg{k}, nil, mir2.AllClasses, mir2.TyU16, mir2.ClassPair)
		b.Ret(r)
		locs := map[mir2.Reg]mir2.PhysLoc{
			k: {Kind: mir2.LocReg, Name: "BC"},
			r: {Kind: mir2.LocReg, Name: "HL"},
		}
		asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
		regs := fix1Execute(t, "ORG 0x8000\nLD SP,0xFF00\nLD BC,0\nCALL asmconst16\nDI\nHALT\n"+asm)
		if regs.HL != 0x1234 {
			t.Fatalf("asm input constant not materialised: HL=%04X want 1234\n%s", regs.HL, asm)
		}
	})

	// A constant feeding both an asm input and an ordinary register consumer
	// must stay materialised exactly once, and the template must read the
	// register the LD wrote.
	t.Run("shared", func(t *testing.T) {
		m := &mir2.Module{Name: "asmconstshare"}
		f := m.AddFunc("asmconstshare")
		f.Attrs.HasAsm = true
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		k := b.Const(0x37, mir2.TyU8, mir2.ClassGeneral)
		one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
		sum := b.Add(k, one, mir2.TyU8, mir2.ClassGeneral)
		r := b.Asm("LD A, C", []mir2.Reg{k}, nil, mir2.AllClasses, mir2.TyU8, mir2.ClassAcc)
		phi := b.Add(r, sum, mir2.TyU8, mir2.ClassAcc)
		b.Ret(phi)
		locs := map[mir2.Reg]mir2.PhysLoc{
			k:   {Kind: mir2.LocReg, Name: "C"},
			r:   {Kind: mir2.LocReg, Name: "A"},
			one: {Kind: mir2.LocReg, Name: "D"},
			sum: {Kind: mir2.LocReg, Name: "E"},
			phi: {Kind: mir2.LocReg, Name: "A"},
		}
		asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
		regs := fix1Execute(t, "ORG 0x8000\nLD SP,0xFF00\nXOR A\nLD E,0\nCALL asmconstshare\nDI\nHALT\n"+asm)
		if regs.A != 0x37+0x37+1 {
			t.Fatalf("asm input constant not materialised: A=%d want 111\n%s", regs.A, asm)
		}
		if n := strings.Count(asm, fmt.Sprintf("LD C, %d", 0x37)); n != 1 {
			t.Fatalf("constant 0x37 loaded %d times, want exactly 1\n%s", n, asm)
		}
	})
}
