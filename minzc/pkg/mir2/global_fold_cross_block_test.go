package mir2

import (
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// crossBlockAddrModule builds main with one &gc that the entry block uses
// once, through first, and the taken block uses again, through second:
//
//	entry: p = &gc; first(p); if ga != 0 goto taken else done
//	taken: second(p)
//	done:  ret
func crossBlockAddrModule(gc Ty, first, second func(b *Builder, p Reg)) (*Module, *Func) {
	m := &Module{Globals: []Global{
		{Name: "ga", Ty: TyU8, Init: []byte{0x5A}},
		{Name: "gc", Ty: gc, Init: []byte{0x11, 0x22}[:ByteWidth(gc)]},
		{Name: "gd", Ty: TyU16, Init: []byte{0, 0}},
	}}
	f := m.AddFunc("main")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	p := b.AddrOf("gc", ClassPointer)
	first(b, p)
	a := b.Load(b.AddrOf("ga", ClassPointer), TyU8, ClassAcc)
	zero := b.Const(0, TyU8, ClassGeneral)
	ne := b.Cmp(CmpNe, a, zero, ClassFlag, false)
	b.BrIf(ne, "taken", nil, "done", nil)
	b.SwitchToNewBlock("taken")
	second(b, p)
	b.Jmp("done")
	b.SwitchToNewBlock("done")
	b.Ret()
	return m, f
}

// An address folded into a direct LD A,(nn) / LD (nn),A / LD HL,(nn), and the
// values of a load-modify-store fused into SET/RES, are not computed into
// their registers, which is right only when no other block reads them.
func TestGlobalFoldKeepsAddressUsedInAnotherBlock(t *testing.T) {
	storeTo := func(sym string, ty Ty, cls RegClass) func(b *Builder, p Reg) {
		return func(b *Builder, p Reg) {
			b.Store(b.AddrOf(sym, ClassPointer), b.Load(p, ty, cls), ty)
		}
	}
	var cur, next Reg
	for _, c := range []struct {
		name          string
		gc            Ty
		first, second func(b *Builder, p Reg)
		sym           string
		want          []byte
	}{
		{"byte load, then store", TyU8,
			func(b *Builder, p Reg) { b.Load(p, TyU8, ClassAcc) },
			func(b *Builder, p Reg) { b.Store(p, b.Const(0x77, TyU8, ClassAcc), TyU8) },
			"gc", []byte{0x77}},
		{"byte store, then load", TyU8,
			func(b *Builder, p Reg) { b.Store(p, b.Const(0x33, TyU8, ClassAcc), TyU8) },
			storeTo("gd", TyU8, ClassAcc),
			"gd", []byte{0x33, 0}},
		{"word load, then store", TyU16,
			func(b *Builder, p Reg) { b.Load(p, TyU16, ClassPointer) },
			func(b *Builder, p Reg) { b.Store(p, b.Const(0x4321, TyU16, ClassPointer), TyU16) },
			"gc", []byte{0x21, 0x43}},
		{"word load, then load", TyU16,
			func(b *Builder, p Reg) { b.Load(p, TyU16, ClassPointer) },
			storeTo("gd", TyU16, ClassPointer),
			"gd", []byte{0x11, 0x22}},
		// gc |= 4 fused into SET 2,(HL) computes neither the loaded byte nor
		// the modified one.
		{"bit store, then its value", TyU8,
			func(b *Builder, p Reg) {
				cur = b.Load(p, TyU8, ClassAcc)
				next = b.BinOp(OpOr, cur, b.Const(4, TyU8, ClassGeneral), TyU8, ClassAcc)
				b.Store(p, next, TyU8)
			},
			func(b *Builder, p Reg) { b.Store(b.AddrOf("gd", ClassPointer), next, TyU8) },
			"gd", []byte{0x15, 0}},
		{"bit store, then its loaded byte", TyU8,
			func(b *Builder, p Reg) {
				cur = b.Load(p, TyU8, ClassAcc)
				next = b.BinOp(OpOr, cur, b.Const(4, TyU8, ClassGeneral), TyU8, ClassAcc)
				b.Store(p, next, TyU8)
			},
			func(b *Builder, p Reg) { b.Store(b.AddrOf("gd", ClassPointer), cur, TyU8) },
			"gd", []byte{0x11, 0}},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, f := crossBlockAddrModule(c.gc, c.first, c.second)
			PreallocCoalesce(f)
			ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			bin, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(bin.Errors) > 0 {
				t.Fatal(err, bin.Errors, asm)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 200000
			z.LoadMemory(uint16(bin.Origin), bin.Binary)
			// CALL main / DI / HALT, with the bytes around the program poisoned
			// so that a store through an unloaded register is visible.
			z.LoadMemory(0xf100, []byte{0xCD, byte(bin.Symbols["main"]), byte(bin.Symbols["main"] >> 8), 0xF3, 0x76})
			z.SetPC(0xf100)
			z.SetSP(0xf000)
			if err := z.Run(); err != nil {
				t.Fatal(err)
			}
			for n, want := range c.want {
				if got := z.GetMemory(uint16(bin.Symbols[c.sym]) + uint16(n)); got != want {
					t.Fatalf("%s+%d = %#02x, want %#02x\n%s\n%s", c.sym, n, got, want, m.Dump(), asm)
				}
			}
		})
	}
}
