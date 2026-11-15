package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// multiDefAccumulatorLoop builds a loop in the form pre-allocation coalescing
// leaves behind: the accumulator is one register defined by its initial
// constant in @entry and by its update in @body.
//
//	fun @accumulate(%n: u8, %k: u8) -> <ty>
//	  @entry:           %acc = const <init> ; %i = const 0 ; jmp @head(%acc, %i)
//	  @head(%acc, %i):  br_if %i < %n, @body(), @exit(%acc)
//	  @body:            %e = ext %k ; %acc = <op> %acc, %e   (or %e, %acc)
//	                    %i = %i + 1 ; jmp @head(%acc, %i)
//	  @exit(%res):      %q = sdiv %res, 4 ; ret %q
//
// It returns the update, the accumulator, the extended byte and the division.
func multiDefAccumulatorLoop(ty Ty, init int64, op Op, accFirst bool) (*Module, *Func, *Inst, Reg, Reg, *Inst) {
	m := &Module{}
	f := m.AddFunc("accumulate")
	n, k := f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{
		{Name: "n", Reg: n, Ty: TyU8, Class: ClassGeneral},
		{Name: "k", Reg: k, Ty: TyU8, Class: ClassGeneral},
	}
	f.Contract.Returns = []Return{{Ty: ty, Class: ClassGeneral}}
	entry, head, body, exit := f.NewBlock("entry"), f.NewBlock("head"), f.NewBlock("body"), f.NewBlock("exit")
	acc, i, lt, e, one, four, q := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	head.Params = []BlockParam{{Dst: acc, Ty: ty, Class: ClassGeneral}, {Dst: i, Ty: TyU8, Class: ClassGeneral}}
	exit.Params = []BlockParam{{Dst: acc, Ty: ty, Class: ClassGeneral}}

	entry.Append(&Inst{Op: OpConst, Dst: acc, Ty: ty, Imm: init})
	entry.Append(&Inst{Op: OpConst, Dst: i, Ty: TyU8, Imm: 0})
	entry.Seal(&TermJmp{Target: "head", Args: []Reg{acc, i}})
	head.Append(&Inst{Op: OpCmp, Cond: CmpLt, Dst: lt, Src: [2]Reg{i, n}, Ty: TyBool, Cls: ClassFlag})
	head.Seal(&TermBrIf{Cond: lt, Then: "body", Else: "exit", ElseArgs: []Reg{acc}})
	body.Append(&Inst{Op: OpExt, Dst: e, Src: [2]Reg{k}, SrcTy: TyU8, Ty: ty})
	update := &Inst{Op: op, Dst: acc, Src: [2]Reg{acc, e}, Ty: ty, Cls: ClassPointer}
	if !accFirst {
		update.Src = [2]Reg{e, acc}
	}
	body.Append(update)
	body.Append(&Inst{Op: OpConst, Dst: one, Ty: TyU8, Imm: 1})
	body.Append(&Inst{Op: OpAdd, Dst: i, Src: [2]Reg{i, one}, Ty: TyU8})
	body.Seal(&TermJmp{Target: "head", Args: []Reg{acc, i}})
	exit.Append(&Inst{Op: OpConst, Dst: four, Ty: ty, Imm: 4})
	div := &Inst{Op: OpSDiv, Dst: q, Src: [2]Reg{acc, four}, Ty: ty, Cls: ClassPointer}
	exit.Append(div)
	exit.Seal(&TermRet{Vals: []Reg{q}})
	return m, f, update, acc, e, div
}

// multiDefCodegen returns a code generator positioned in the block of inst.
func multiDefCodegen(m *Module, f *Func, inst *Inst, locs map[Reg]PhysLoc, sb *strings.Builder, sizeMode bool) *z80cg {
	var cur *Block
	consts := map[Reg]int64{}
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x == inst {
				cur = b
			}
		}
	}
	for _, x := range cur.Insts {
		if x.Op == OpConst {
			consts[x.Dst] = x.Imm
		}
	}
	return &z80cg{mod: m, fn: f, ar: &AllocResult{Locs: locs}, sb: sb, curBlock: cur, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: consts, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg, optimizeForSize: sizeMode}
}

// multiDefRun assembles an emitted sequence and returns the registers after it.
func multiDefRun(t *testing.T, body string, before emulator.Registers) emulator.Registers {
	t.Helper()
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nsequence:\n" + body + "\nRET\n" + qsqTableAsm())
	if e != nil {
		t.Fatal(e, body)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors, body)
	}
	z := emulator.NewRemogattoZ80()
	if e := z.LoadMemory(0x8000, a.Binary); e != nil {
		t.Fatal(e)
	}
	ram := z.Memory()
	ram[0xff00], ram[0xff01] = 0, 0x70
	before.PC, before.SP = 0x8000, 0xff00
	z.SetRegisters(before)
	for n := 0; (z.GetPC() != 0x7000 || z.GetSP() != 0xff02) && n < 4000; n++ {
		z.Step()
	}
	got := z.GetRegisters()
	if got.PC != 0x7000 || got.SP != 0xff02 {
		t.Fatalf("sequence did not return: %+v\n%s", got, body)
	}
	return got
}

// A register merged from a small constant and a wider update is not a byte:
// neither range proof may read only the constant.
func TestRangeProofsRequireEveryDefinition(t *testing.T) {
	m, f, update, acc, e, _ := multiDefAccumulatorLoop(TyU16, 1, OpMul, true)
	g := multiDefCodegen(m, f, update, map[Reg]PhysLoc{}, &strings.Builder{}, false)
	if g.provablyBelow256(acc, 16, map[Reg]bool{}) {
		t.Fatal("an accumulator updated by a multiply was proven below 256 from its initial constant")
	}
	if g.nonNegative(acc, 16, map[Reg]bool{}) {
		t.Fatal("an accumulator updated by a multiply was proven non-negative from its initial constant")
	}
	if !g.provablyBelow256(e, 16, map[Reg]bool{}) || !g.nonNegative(e, 16, map[Reg]bool{}) {
		t.Fatal("a zero-extended byte lost its proof")
	}

	// Every definition a byte: the merged register keeps the proof, whether
	// the definitions are instructions or values passed into its parameter.
	f = m.AddFunc("bytes_only")
	p, x, c, r, wide := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{{Name: "p", Reg: p, Ty: TyU8, Class: ClassGeneral}, {Name: "wide", Reg: wide, Ty: TyU16, Class: ClassGeneral}}
	entry, alt, join := f.NewBlock("entry"), f.NewBlock("alt"), f.NewBlock("join")
	join.Params = []BlockParam{{Dst: r, Ty: TyU16, Class: ClassGeneral}}
	entry.Append(&Inst{Op: OpExt, Dst: x, Src: [2]Reg{p}, SrcTy: TyU8, Ty: TyU16})
	entry.Append(&Inst{Op: OpConst, Dst: r, Ty: TyU16, Imm: 200})
	entry.Seal(&TermBrIf{Cond: p, Then: "alt", Else: "join", ElseArgs: []Reg{r}})
	alt.Append(&Inst{Op: OpConst, Dst: c, Ty: TyU16, Imm: 9})
	altJmp := &TermJmp{Target: "join", Args: []Reg{x}}
	alt.Seal(altJmp)
	join.Seal(&TermRet{Vals: []Reg{r}})
	g = multiDefCodegen(m, f, entry.Insts[0], map[Reg]PhysLoc{}, &strings.Builder{}, false)
	if !g.provablyBelow256(r, 16, map[Reg]bool{}) || !g.nonNegative(r, 16, map[Reg]bool{}) {
		t.Fatal("a register whose every definition is a byte lost its proof")
	}
	// One unproven value on one edge is enough to lose it.
	altJmp.Args[0] = wide
	if g.provablyBelow256(r, 16, map[Reg]bool{}) || g.nonNegative(r, 16, map[Reg]bool{}) {
		t.Fatal("a word passed into the parameter on one edge was ignored")
	}
	altJmp.Args[0] = x

	// The implicit DJNZ counter and an extra call result have no instruction
	// to judge.
	loop := f.NewBlock("loop")
	loop.Params = []BlockParam{{Dst: c, Ty: TyU8, Class: ClassCounter}}
	loop.Seal(&TermDJNZ{Counter: c, Body: "loop", Exit: "join", ExitArgs: []Reg{x}})
	if g.provablyBelow256(c, 16, map[Reg]bool{}) {
		t.Fatal("a DJNZ counter was proven from the constant sharing its register")
	}
	loop.Params = nil
	entry.Insts = append(entry.Insts, &Inst{Op: OpCall, Sym: "bytes_only", Dst: NoReg, ExtraRets: []Reg{r}, ExtraRetTys: []Ty{TyU16}})
	if g.provablyBelow256(r, 16, map[Reg]bool{}) || g.nonNegative(r, 16, map[Reg]bool{}) {
		t.Fatal("an extra call result was ignored as a definition")
	}
}

// The accumulator of a product loop keeps all sixteen bits through the
// multiply, in every operand order, home and optimisation mode.
func TestMul16MultiDefAccumulatorNative(t *testing.T) {
	type homes struct{ acc, e string }
	for _, h := range []homes{{"HL", "DE"}, {"HL", "BC"}, {"DE", "BC"}, {"BC", "DE"}, {"DE", "HL"}} {
		for _, accFirst := range []bool{true, false} {
			for _, sizeMode := range []bool{false, true} {
				name := fmt.Sprintf("acc=%s/e=%s/accFirst=%v/size=%v", h.acc, h.e, accFirst, sizeMode)
				t.Run(name, func(t *testing.T) {
					m, f, update, acc, e, _ := multiDefAccumulatorLoop(TyU16, 1, OpMul, accFirst)
					locs := map[Reg]PhysLoc{acc: {Kind: LocReg, Name: h.acc}, e: {Kind: LocReg, Name: h.e}}
					var sb strings.Builder
					g := multiDefCodegen(m, f, update, locs, &sb, sizeMode)
					g.genMul16(update)
					if g.err != nil {
						t.Fatal(g.err)
					}
					asm := sb.String()
					if strings.Contains(asm, "__qsq") {
						t.Fatalf("a word accumulator took the byte-by-byte table multiply\n%s", asm)
					}
					for _, a := range []uint16{0, 1, 0x00ff, 0x0100, 0x02d0, 0x1234, 0xffff} {
						for _, k := range []uint16{0, 1, 2, 7, 0x80, 0xff} {
							before := emulator.Registers{}
							set := func(loc string, v uint16) {
								switch loc {
								case "HL":
									before.HL = v
								case "DE":
									before.DE = v
								case "BC":
									before.BC = v
								}
							}
							set(h.acc, a)
							set(h.e, k)
							got := multiDefRun(t, asm, before)
							res := map[string]uint16{"HL": got.HL, "DE": got.DE, "BC": got.BC}[h.acc]
							if want := a * k; res != want {
								t.Fatalf("%#x * %#x = %#x, want %#x\n%s", a, k, res, want, asm)
							}
						}
					}
				})
			}
		}
	}
}

// A signed accumulator that starts positive and counts down is divided as a
// signed value: the quotient of a negative dividend rounds toward zero.
func TestSDivPow2MultiDefAccumulatorNative(t *testing.T) {
	m, f, _, acc, _, div := multiDefAccumulatorLoop(TyI16, 5, OpSub, true)
	locs := map[Reg]PhysLoc{acc: {Kind: LocReg, Name: "HL"}, div.Dst: {Kind: LocReg, Name: "HL"}}
	var sb strings.Builder
	g := multiDefCodegen(m, f, div, locs, &sb, false)
	g.genDivMod(div)
	if g.err != nil {
		t.Fatal(g.err)
	}
	asm := sb.String()
	for _, x := range []int16{5, 4, 3, 0, -1, -3, -4, -5, -16, -17, 32767, -32768} {
		got := multiDefRun(t, asm, emulator.Registers{HL: uint16(x)})
		if want := x / 4; int16(got.HL) != want {
			t.Fatalf("%d / 4 = %d, want %d\n%s", x, int16(got.HL), want, asm)
		}
	}
}

// A sign extension is below 256 only when the source byte's own sign bit is
// provably clear: an unsigned byte parameter is non-negative as a word, not as
// the signed byte being extended.
func TestSextOfByteBelow256NeedsClearSignBit(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("sext_mul")
	p, w := f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{{Name: "p", Reg: p, Ty: TyU8, Class: ClassGeneral}, {Name: "w", Reg: w, Ty: TyU16, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyI16, Class: ClassGeneral}}
	entry := f.NewBlock("entry")
	s, small, big, sSmall, sBig, prod := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	entry.Append(&Inst{Op: OpSext, Dst: s, Src: [2]Reg{p}, SrcTy: TyI8, Ty: TyI16})
	entry.Append(&Inst{Op: OpConst, Dst: small, Ty: TyI8, Imm: 100})
	entry.Append(&Inst{Op: OpConst, Dst: big, Ty: TyI8, Imm: 200})
	entry.Append(&Inst{Op: OpSext, Dst: sSmall, Src: [2]Reg{small}, SrcTy: TyI8, Ty: TyI16})
	entry.Append(&Inst{Op: OpSext, Dst: sBig, Src: [2]Reg{big}, SrcTy: TyI8, Ty: TyI16})
	mul := &Inst{Op: OpMul, Dst: prod, Src: [2]Reg{w, s}, Ty: TyI16, Cls: ClassPointer}
	entry.Append(mul)
	entry.Seal(&TermRet{Vals: []Reg{prod}})

	locs := map[Reg]PhysLoc{w: {Kind: LocReg, Name: "HL"}, s: {Kind: LocReg, Name: "DE"}, prod: {Kind: LocReg, Name: "HL"}}
	var sb strings.Builder
	g := multiDefCodegen(m, f, mul, locs, &sb, true)
	if g.provablyBelow256(s, 16, map[Reg]bool{}) || g.provablyBelow256(sBig, 16, map[Reg]bool{}) {
		t.Fatal("a sign extension of a byte with a possibly set sign bit was proven below 256")
	}
	if !g.provablyBelow256(sSmall, 16, map[Reg]bool{}) {
		t.Fatal("a sign extension of a constant below 128 lost its proof")
	}
	g.genMul16(mul)
	if g.err != nil {
		t.Fatal(g.err)
	}
	asm := sb.String()
	for _, a := range []uint16{0, 1, 3, 0x0100, 0x1234, 0xffff} {
		for _, b := range []uint16{0, 1, 0x7f, 0xff80, 0xffc8, 0xffff} {
			got := multiDefRun(t, asm, emulator.Registers{HL: a, DE: b})
			if want := a * b; got.HL != want {
				t.Fatalf("%#x * %#x = %#x, want %#x\n%s", a, b, got.HL, want, asm)
			}
		}
	}
}
