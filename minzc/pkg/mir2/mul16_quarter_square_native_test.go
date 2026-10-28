package mir2

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// qsqTableAsm is the reference quarter-square table the tests assemble
// alongside the emitted multiply: sq[n] = floor(n*n/4), n = 0..510.
func qsqTableAsm() string {
	var sb strings.Builder
	sb.WriteString("__qsq:\n")
	for n := 0; n <= 510; n += 8 {
		sb.WriteString("    DW ")
		for i := n; i <= n+7 && i <= 510; i++ {
			if i > n {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%d", (i*i)/4)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// qsqCountedLoop builds the fused-loop shape of fusion_dot_product:
//
//	fun @mul_loop(%limit: u16, %a: u8, %b: u8) -> u16
//	  @entry:        jmp @head(0, 0)
//	  @head(%i,%acc): br_if %i != %limit, @body(), @exit(%acc)
//	  @body:         %ea = ext %a ; %eb = ext %b ; %p = mul %ea, %eb
//	                 %acc2 = %acc + %p ; %i2 = %i + 1 ; jmp @head(%i2, %acc2)
//	  @exit(%res):   ret %res
//
// signed selects OpSext i8->i16 operands instead of OpExt u8->u16. The returned
// live reg is the loop-carried accumulator, which stays live across the
// multiply so register preservation is exercised.
func qsqCountedLoop(t *testing.T, signed bool) (*Module, *Func, *Block, *Inst, Reg) {
	t.Helper()
	m := &Module{}
	f := m.AddFunc("mul_loop")
	limit, a, b := f.AllocReg(), f.AllocReg(), f.AllocReg()
	aTy, bTy, extOp := TyU8, TyU8, OpExt
	if signed {
		aTy, bTy, extOp = TyI8, TyI8, OpSext
	}
	f.Contract.Params = []Param{
		{Name: "limit", Reg: limit, Ty: TyU16, Class: ClassGeneral},
		{Name: "a", Reg: a, Ty: aTy, Class: ClassGeneral},
		{Name: "b", Reg: b, Ty: bTy, Class: ClassGeneral},
	}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassGeneral}}
	entry := f.NewBlock("entry")
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	exit := f.NewBlock("exit")
	pi, pacc, pres := f.AllocReg(), f.AllocReg(), f.AllocReg()
	head.Params = []BlockParam{{Dst: pi, Ty: TyU16, Class: ClassGeneral}, {Dst: pacc, Ty: TyU16, Class: ClassGeneral}}
	exit.Params = []BlockParam{{Dst: pres, Ty: TyU16, Class: ClassGeneral}}

	zero := f.AllocReg()
	ea, eb, prod, acc2, one, i2 := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	entry.Append(&Inst{Op: OpConst, Dst: zero, Ty: TyU16, Imm: 0})
	entry.Seal(&TermJmp{Target: "head", Args: []Reg{zero, zero}})
	ne := f.AllocReg()
	head.Append(&Inst{Op: OpCmp, Cond: CmpNe, Dst: ne, Src: [2]Reg{pi, limit}, Ty: TyBool, Cls: ClassFlag})
	head.Seal(&TermBrIf{Cond: ne, Then: "body", Else: "exit", ElseArgs: []Reg{pacc}})
	body.Append(&Inst{Op: extOp, Dst: ea, Src: [2]Reg{a}, SrcTy: aTy, Ty: TyU16})
	body.Append(&Inst{Op: extOp, Dst: eb, Src: [2]Reg{b}, SrcTy: bTy, Ty: TyU16})
	mul := &Inst{Op: OpMul, Dst: prod, Src: [2]Reg{ea, eb}, Ty: TyU16, Cls: ClassPointer}
	body.Append(mul)
	body.Append(&Inst{Op: OpAdd, Dst: acc2, Src: [2]Reg{pacc, prod}, Ty: TyU16})
	body.Append(&Inst{Op: OpConst, Dst: one, Ty: TyU16, Imm: 1})
	body.Append(&Inst{Op: OpAdd, Dst: i2, Src: [2]Reg{pi, one}, Ty: TyU16})
	body.Seal(&TermJmp{Target: "head", Args: []Reg{i2, acc2}})
	exit.Seal(&TermRet{Vals: []Reg{pres}})
	return m, f, body, mul, pacc
}

// qsqStraightLine builds the same multiply outside any loop.
func qsqStraightLine(t *testing.T) (*Module, *Func, *Block, *Inst, Reg) {
	t.Helper()
	m := &Module{}
	f := m.AddFunc("mul_once")
	a, b := f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{
		{Name: "a", Reg: a, Ty: TyU8, Class: ClassGeneral},
		{Name: "b", Reg: b, Ty: TyU8, Class: ClassGeneral},
	}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassGeneral}}
	blk := f.NewBlock("entry")
	ea, eb, prod, prod2 := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	blk.Append(&Inst{Op: OpExt, Dst: ea, Src: [2]Reg{a}, SrcTy: TyU8, Ty: TyU16})
	blk.Append(&Inst{Op: OpExt, Dst: eb, Src: [2]Reg{b}, SrcTy: TyU8, Ty: TyU16})
	mul := &Inst{Op: OpMul, Dst: prod, Src: [2]Reg{ea, eb}, Ty: TyU16, Cls: ClassPointer}
	blk.Append(mul)
	blk.Append(&Inst{Op: OpAdd, Dst: prod2, Src: [2]Reg{prod, prod}, Ty: TyU16})
	blk.Seal(&TermRet{Vals: []Reg{prod2}})
	return m, f, blk, mul, prod
}

// qsqEmitMul runs genMul16 on one instruction and returns the emitted body.
func qsqEmitMul(t *testing.T, m *Module, f *Func, b *Block, inst *Inst, locs map[Reg]PhysLoc, sizeMode bool) string {
	t.Helper()
	var body strings.Builder
	g := &z80cg{mod: m, fn: f, ar: &AllocResult{Locs: locs}, sb: &body, curBlock: b, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg, optimizeForSize: sizeMode}
	g.genMul16(inst)
	if g.err != nil {
		t.Fatal(g.err)
	}
	return body.String()
}

// qsqAssemble wraps an emitted multiply with a return and the shared table.
func qsqAssemble(t *testing.T, fnName, body string, locs map[Reg]PhysLoc) []byte {
	t.Helper()
	equs := ""
	for r, pl := range locs {
		if pl.Kind == LocMem {
			equs += fmt.Sprintf("%s EQU $%04X\n", Z80SpillLabel(fnName, r), pl.Offset)
		}
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nmul_bytes:\n" + body + "\nRET\n" + qsqTableAsm() + equs)
	if e != nil {
		t.Fatal(e, body)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors, body)
	}
	return a.Binary
}

// qsqRunNative executes one assembled multiply and checks the product in HL
// (and in dstIX when set), the preserved registers and the untouched homes.
func qsqRunNative(t *testing.T, binary []byte, ramInit map[uint16]byte, before emulator.Registers, want uint16, dstIX bool, desc, body string) {
	t.Helper()
	z := emulator.NewRemogattoZ80()
	if e := z.LoadMemory(0x8000, binary); e != nil {
		t.Fatal(e)
	}
	ram := z.Memory()
	for addr, v := range ramInit {
		ram[addr] = v
	}
	ram[0xff00], ram[0xff01] = 0, 0x70
	z.SetRegisters(before)
	for n := 0; (z.GetPC() != 0x7000 || z.GetSP() != 0xff02) && n < 2000; n++ {
		z.Step()
	}
	got := z.GetRegisters()
	ixWant := before.IX
	if dstIX {
		ixWant = want
	}
	if got.HL != want || got.A != before.A || got.BC != before.BC || got.DE != before.DE || got.IX != ixWant || got.IY != before.IY || got.PC != 0x7000 || got.SP != 0xff02 {
		t.Fatalf("%s got%+v want%x\n%s", desc, got, want, body)
	}
	for addr, v := range ramInit {
		if ram[addr] != v {
			t.Fatalf("%s ram[%#x] = %#x, want %#x\n%s", desc, addr, ram[addr], v, body)
		}
	}
}

// qsqLoopLocs allocates one operand to DE and the other to a one-byte spill,
// with the live word in liveLoc and the product in dstLoc.
func qsqLoopLocs(f *Func, inst *Inst, live Reg, dstLoc, liveLoc string) map[Reg]PhysLoc {
	return qsqHomesLocs(f, inst, live, dstLoc, liveLoc, "DE", "spill")
}

// qsqHomesLocs is qsqLoopLocs with explicit operand homes.
func qsqHomesLocs(f *Func, inst *Inst, live Reg, dstLoc, liveLoc, lhsLoc, rhsLoc string) map[Reg]PhysLoc {
	// The operands are extension results; their sources are the byte params.
	byteSrc := func(r Reg) Reg {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Dst == r && (i.Op == OpExt || i.Op == OpSext) {
					return i.Src[0]
				}
			}
		}
		return r
	}
	a, bparam := byteSrc(inst.Src[0]), byteSrc(inst.Src[1])
	ea, eb := inst.Src[0], inst.Src[1]
	dst := PhysLoc{Kind: LocReg, Name: dstLoc}
	if dstLoc == "spill" {
		dst = PhysLoc{Kind: LocMem, Offset: 0xa140}
	}
	home := func(name string, spillAt int) PhysLoc {
		if name == "spill" {
			return PhysLoc{Kind: LocMem, Offset: spillAt}
		}
		return PhysLoc{Kind: LocReg, Name: name}
	}
	locs := map[Reg]PhysLoc{
		a:        {Kind: LocMem, Offset: 0xa100},
		bparam:   {Kind: LocMem, Offset: 0xa110},
		ea:       home(lhsLoc, 0xa112),
		eb:       home(rhsLoc, 0xa114),
		inst.Dst: dst,
		live:     {Kind: LocReg, Name: liveLoc},
	}
	for _, p := range f.Contract.Params {
		if _, ok := locs[p.Reg]; !ok {
			locs[p.Reg] = PhysLoc{Kind: LocMem, Offset: 0xa120}
		}
	}
	return locs
}

// (a) Speed mode, loop, two zero-extended bytes: the table path.
func TestMul16QuarterSquareTablePathInLoop(t *testing.T) {
	m, f, body, mul, live := qsqCountedLoop(t, false)
	locs := qsqLoopLocs(f, mul, live, "HL", "BC")
	asm := qsqEmitMul(t, m, f, body, mul, locs, false)
	if !strings.Contains(asm, "__qsq") {
		t.Fatalf("speed-mode loop multiply of two zero-extended bytes must use the quarter-square table\n%s", asm)
	}
	if strings.Contains(asm, "SRL A") || strings.Contains(asm, "DJNZ") {
		t.Fatalf("speed-mode loop multiply must not keep the shift-and-add loop\n%s", asm)
	}
	if n := mul16LoopCounter(asm); n != 0 {
		t.Fatalf("loop counter %d emitted alongside the table path\n%s", n, asm)
	}
	t.Logf("%s", asm)
}

// (c) OptSize keeps the F4 8-iteration loop.
func TestMul16QuarterSquareOptSizeKeepsLoop(t *testing.T) {
	m, f, body, mul, live := qsqCountedLoop(t, false)
	locs := qsqLoopLocs(f, mul, live, "HL", "BC")
	asm := qsqEmitMul(t, m, f, body, mul, locs, true)
	if strings.Contains(asm, "__qsq") {
		t.Fatalf("OptSize must not pull in the 1 KB quarter-square table\n%s", asm)
	}
	if n := mul16LoopCounter(asm); n != 8 {
		t.Fatalf("OptSize loop count %d, want the F4 8-iteration loop\n%s", n, asm)
	}
}

// (d) A one-off multiply outside any loop keeps the loop.
func TestMul16QuarterSquareNonLoopKeepsLoop(t *testing.T) {
	m, f, blk, mul, live := qsqStraightLine(t)
	locs := qsqLoopLocs(f, mul, live, "HL", "BC")
	asm := qsqEmitMul(t, m, f, blk, mul, locs, false)
	if strings.Contains(asm, "__qsq") {
		t.Fatalf("a single non-loop multiply must not pull in the table\n%s", asm)
	}
	if n := mul16LoopCounter(asm); n != 8 {
		t.Fatalf("non-loop multiply loop count %d, want 8\n%s", n, asm)
	}
}

// (e) Sign-extended i8 operands never take the table path.
func TestMul16QuarterSquareSextOperandsKeepLoop(t *testing.T) {
	m, f, body, mul, live := qsqCountedLoop(t, true)
	locs := qsqLoopLocs(f, mul, live, "HL", "BC")
	asm := qsqEmitMul(t, m, f, body, mul, locs, false)
	if strings.Contains(asm, "__qsq") {
		t.Fatalf("sign-extended operands may carry 0xFF high bytes and must keep the loop\n%s", asm)
	}
	if n := mul16LoopCounter(asm); n != 16 {
		t.Fatalf("sext operand loop count %d, want 16\n%s", n, asm)
	}
}

// (b) Exhaustive 256×256 native product check, with the VM as oracle.
func TestMul16QuarterSquareExhaustiveNative(t *testing.T) {
	for _, dstLoc := range []string{"HL", "IX"} {
		m, f, body, mul, live := qsqCountedLoop(t, false)
		locs := qsqLoopLocs(f, mul, live, dstLoc, "BC")
		asm := qsqEmitMul(t, m, f, body, mul, locs, false)
		if !strings.Contains(asm, "__qsq") {
			t.Fatalf("dst=%s: expected the table path\n%s", dstLoc, asm)
		}
		binary := qsqAssemble(t, f.Name, asm, locs)
		cases := 0
		for value := 0; value < 256; value++ {
			for rvalue := 0; rvalue < 256; rvalue++ {
				want := uint16(value * rvalue)
				if value%16 == 0 && rvalue%16 == 0 {
					vm, e := NewVM(m).Call(f.Name, []Value{{I: 1}, {I: int64(value)}, {I: int64(rvalue)}})
					if e != nil || len(vm) != 1 || uint16(vm[0].I) != want {
						t.Fatalf("VM limit1 %d*%d = %v (%v), want %#x", value, rvalue, vm, e, want)
					}
				}
				regs := mul16PreservedRegs(0xabcd)
				// lhs (ea) is homed in DE with a poisoned high byte, rhs (eb)
				// in a one-byte spill whose neighbour is poisoned too.
				regs.DE = 0x5600 | uint16(byte(value))
				qsqRunNative(t, binary, qsqRam(value, rvalue), regs, want, dstLoc == "IX",
					fmt.Sprintf("dst%s value%d rvalue%d", dstLoc, value, rvalue), asm)
				cases++
			}
		}
		t.Logf("dst=%s: %d native executions", dstLoc, cases)
	}
}

// qsqRam is the memory image for one (lhs, rhs) byte pair, with the operand
// homes and their poisoned neighbours.
func qsqRam(value, rvalue int) map[uint16]byte {
	return map[uint16]byte{
		0xa100: byte(value), 0xa101: 0x5a, // lhs parameter home
		0xa110: byte(rvalue), 0xa111: 0x5a, // rhs parameter home
		0xa112: byte(rvalue), 0xa113: 0x5a, // rhs spill home
		0xa114: byte(rvalue), 0xa115: 0x5a,
	}
}

// Result homes the sequence must write without letting a restore overwrite
// them: DE, BC and a spill slot, each with a different live pair to preserve.
func TestMul16QuarterSquareHomesNative(t *testing.T) {
	for _, tc := range []struct {
		dstLoc, liveLoc string
	}{
		{"DE", "BC"}, {"DE", "HL"}, {"BC", "HL"}, {"spill", "BC"}, {"spill", "HL"},
	} {
		m, f, body, mul, live := qsqCountedLoop(t, false)
		locs := qsqLoopLocs(f, mul, live, tc.dstLoc, tc.liveLoc)
		asm := qsqEmitMul(t, m, f, body, mul, locs, false)
		if !strings.Contains(asm, "__qsq") {
			t.Fatalf("dst=%s live=%s: expected the table path\n%s", tc.dstLoc, tc.liveLoc, asm)
		}
		if strings.Contains(asm, "SRL A") || strings.Contains(asm, "DJNZ") {
			t.Fatalf("dst=%s: shift-and-add loop emitted\n%s", tc.dstLoc, asm)
		}
		binary := qsqAssemble(t, f.Name, asm, locs)
		for _, value := range []int{0, 1, 15, 128, 200, 255} {
			for _, rvalue := range []int{0, 1, 127, 255} {
				want := uint16(value * rvalue)
				regs := mul16PreservedRegs(0xabcd)
				regs.DE = 0x5600 | uint16(byte(value))
				z := emulator.NewRemogattoZ80()
				if e := z.LoadMemory(0x8000, binary); e != nil {
					t.Fatal(e)
				}
				ram := z.Memory()
				for addr, v := range qsqRam(value, rvalue) {
					ram[addr] = v
				}
				ram[0xff00], ram[0xff01] = 0, 0x70
				z.SetRegisters(regs)
				for n := 0; (z.GetPC() != 0x7000 || z.GetSP() != 0xff02) && n < 2000; n++ {
					z.Step()
				}
				got := z.GetRegisters()
				where := map[string]uint16{"DE": got.DE, "BC": got.BC, "spill": uint16(ram[0xa140]) | uint16(ram[0xa141])<<8}[tc.dstLoc]
				hlWant := want
				if tc.liveLoc == "HL" {
					hlWant = regs.HL // HL was live: restored, not left as the product
				}
				preserved := got.A == regs.A && got.IX == regs.IX && got.IY == regs.IY && got.PC == 0x7000 && got.SP == 0xff02 &&
					(tc.liveLoc != "BC" || got.BC == regs.BC) && (tc.liveLoc != "DE" || got.DE == regs.DE) && (tc.liveLoc != "HL" || got.HL == regs.HL)
				if !preserved || where != want || got.HL != hlWant {
					t.Fatalf("dst=%s live=%s %d*%d: dst=%#x HL=%#x got%+v want%#x\n%s", tc.dstLoc, tc.liveLoc, value, rvalue, where, got.HL, got, want, asm)
				}
			}
		}
	}
}

// qsqFillLocs gives every remaining vreg in the module a memory home so the
// module-level emitter can run on a hand-built fixture.
func qsqFillLocs(m *Module, locs map[Reg]PhysLoc) {
	next := 0xa300
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, p := range b.Params {
				if _, ok := locs[p.Dst]; !ok {
					locs[p.Dst] = PhysLoc{Kind: LocMem, Offset: next}
					next += 2
				}
			}
			for _, i := range b.Insts {
				if i.Dst == NoReg {
					continue
				}
				if _, ok := locs[i.Dst]; !ok {
					locs[i.Dst] = PhysLoc{Kind: LocMem, Offset: next}
					next += 2
				}
			}
		}
	}
}

// (f) The table is emitted once per module when used, and not at all when unused.
func TestMul16QuarterSquareTableEmittedOncePerModule(t *testing.T) {
	countTable := func(text string) (labels, words int) {
		labels = strings.Count(text, "__qsq:\n")
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "DW ") {
				continue
			}
			words += len(strings.Split(line[3:], ","))
		}
		return labels, words
	}

	// Two loop multiplies in one module.
	m, _, _, mul, live := qsqCountedLoop(t, false)
	locs := qsqLoopLocs(m.Funcs[0], mul, live, "HL", "BC")
	second := m.AddFunc("mul_loop2")
	{
		a, b := second.AllocReg(), second.AllocReg()
		second.Contract.Params = []Param{{Name: "a", Reg: a, Ty: TyU8, Class: ClassGeneral}, {Name: "b", Reg: b, Ty: TyU8, Class: ClassGeneral}}
		second.Contract.Returns = []Return{{Ty: TyU16, Class: ClassGeneral}}
		entry := second.NewBlock("entry")
		head := second.NewBlock("head")
		entry.Seal(&TermJmp{Target: "head"})
		ea, eb, prod := second.AllocReg(), second.AllocReg(), second.AllocReg()
		head.Append(&Inst{Op: OpExt, Dst: ea, Src: [2]Reg{a}, SrcTy: TyU8, Ty: TyU16})
		head.Append(&Inst{Op: OpExt, Dst: eb, Src: [2]Reg{b}, SrcTy: TyU8, Ty: TyU16})
		head.Append(&Inst{Op: OpMul, Dst: prod, Src: [2]Reg{ea, eb}, Ty: TyU16, Cls: ClassPointer})
		head.Seal(&TermJmp{Target: "head"})
		for i, r := range []Reg{a, b, ea, eb, prod} {
			locs[r] = PhysLoc{Kind: LocMem, Offset: 0xa200 + 2*i}
		}
	}
	qsqFillLocs(m, locs)
	text, e := Z80Codegen(m, &AllocResult{Locs: locs})
	if e != nil {
		t.Fatal(e)
	}
	labels, words := countTable(text)
	if labels != 1 {
		t.Fatalf("quarter-square table emitted %d times, want exactly 1\n%s", labels, text)
	}
	if words != 511 {
		t.Fatalf("quarter-square table has %d words, want 511", words)
	}
	if !strings.Contains(text, "__qsq") {
		t.Fatalf("table path missing from module output\n%s", text)
	}

	// A module with no admissible multiply must not carry the table.
	m2, f2, _, mul2, live2 := qsqStraightLine(t)
	locs2 := qsqLoopLocs(f2, mul2, live2, "HL", "BC")
	qsqFillLocs(m2, locs2)
	text2, e := Z80Codegen(m2, &AllocResult{Locs: locs2})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(text2, "__qsq") {
		t.Fatalf("unused table emitted for a module without an in-loop byte multiply\n%s", text2)
	}
}

// The emitted table satisfies the identity for every byte pair, and matches
// the reference layout the tests assemble.
func TestMul16QuarterSquareTableIdentity(t *testing.T) {
	table := quarterSquareTableAsm()
	stripComment := func(s string) string {
		var out []string
		for _, line := range strings.Split(s, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), ";") {
				out = append(out, line)
			}
		}
		return strings.Join(out, "\n")
	}
	if stripComment(table) != stripComment(qsqTableAsm()) {
		t.Fatalf("production table differs from the reference layout:\n%s", table)
	}
	var sq []int
	for _, line := range strings.Split(table, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "DW ") {
			continue
		}
		for _, f := range strings.Split(line[3:], ",") {
			v, e := strconv.Atoi(strings.TrimSpace(f))
			if e != nil {
				t.Fatalf("bad table word %q: %v", f, e)
			}
			sq = append(sq, v)
		}
	}
	if len(sq) != 511 {
		t.Fatalf("table has %d words, want 511", len(sq))
	}
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			d := a - b
			if d < 0 {
				d = -d
			}
			if got := sq[a+b] - sq[d]; got != a*b {
				t.Fatalf("sq[%d] - sq[%d] = %d, want %d", a+b, d, got, a*b)
			}
		}
	}
}

// A constant operand keeps the strength-reduction / loop paths.
func TestMul16QuarterSquareConstantOperandKeepsLoop(t *testing.T) {
	m, f, body, mul, live := qsqCountedLoop(t, false)
	locs := qsqLoopLocs(f, mul, live, "HL", "BC")
	g := &z80cg{mod: m, fn: f, ar: &AllocResult{Locs: locs}, sb: &strings.Builder{}, curBlock: body, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{mul.Src[1]: 200}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg}
	if g.mul16QuarterSquareAdmissible(mul) {
		t.Fatal("constant operand must not take the table path")
	}
}

// Operand homes: every staging order must load both operand bytes intact,
// including a spill on either side and a value already sitting in A.
func TestMul16QuarterSquareOperandHomesNative(t *testing.T) {
	for _, tc := range []struct{ lhs, rhs string }{
		{"DE", "spill"}, {"spill", "DE"}, {"B", "E"}, {"A", "D"}, {"D", "A"},
		{"spill", "A"}, {"spill", "spill"}, {"C", "E"}, {"IX", "B"},
	} {
		m, f, body, mul, live := qsqCountedLoop(t, false)
		locs := qsqHomesLocs(f, mul, live, "HL", "BC", tc.lhs, tc.rhs)
		asm := qsqEmitMul(t, m, f, body, mul, locs, false)
		if !strings.Contains(asm, "__qsq") {
			t.Fatalf("lhs=%s rhs=%s: expected the table path\n%s", tc.lhs, tc.rhs, asm)
		}
		binary := qsqAssemble(t, f.Name, asm, locs)
		for _, value := range []int{0, 1, 64, 200, 255} {
			for _, rvalue := range []int{0, 1, 100, 255} {
				want := uint16(value * rvalue)
				regs := mul16PreservedRegs(0xabcd)
				ram := qsqRam(value, rvalue)
				put := func(name string, v byte) {
					switch name {
					case "A":
						regs.A = v
					case "B":
						regs.BC = (regs.BC & 0x00ff) | uint16(v)<<8
					case "C":
						regs.BC = (regs.BC & 0xff00) | uint16(v)
					case "D":
						regs.DE = (regs.DE & 0x00ff) | uint16(v)<<8
					case "E", "DE":
						regs.DE = (regs.DE & 0xff00) | uint16(v)
					case "IX":
						regs.IX = 0x9a00 | uint16(v)
					case "spill":
						ram[0xa114] = v
					}
				}
				ram[0xa112], ram[0xa113] = byte(value), 0x5a
				ram[0xa114], ram[0xa115] = byte(rvalue), 0x5a
				put(tc.lhs, byte(value))
				put(tc.rhs, byte(rvalue))
				qsqRunNative(t, binary, ram, regs, want, false,
					fmt.Sprintf("lhs%s rhs%s value%d rvalue%d", tc.lhs, tc.rhs, value, rvalue), asm)
			}
		}
	}
}
