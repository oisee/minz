package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
)

// wbmLoop builds the fusion_dot_product shape before NarrowWideningByteMul:
//
//	@head(%i, %acc): br_if %i != %limit, @body(), @exit(%acc)
//	@body: %ea = ext %a ; %eb = ext %b ; %p = mul %ea, %eb
//	       %acc2 = %acc + %p ; %i2 = %i + 1 ; jmp @head(%i2, %acc2)
//
// same multiplies one extension by itself (u16(a) * u16(a)). inLoop false
// puts the body in a straight-line function instead. ty is the byte type.
func wbmLoop(t *testing.T, same, inLoop bool, ty Ty, extOp Op) (m *Module, f *Func, body *Block, mul *Inst, a, b, acc Reg) {
	t.Helper()
	m = &Module{}
	f = m.AddFunc("wbm")
	limit := f.AllocReg()
	a, b = f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{
		{Name: "limit", Reg: limit, Ty: TyU16, Class: ClassGeneral},
		{Name: "a", Reg: a, Ty: ty, Class: ClassGeneral},
		{Name: "b", Reg: b, Ty: ty, Class: ClassGeneral},
	}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassGeneral}}
	ea, eb, prod, acc2 := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	rhsExt := eb
	if same {
		rhsExt = ea
	}
	emitBody := func(blk *Block, accIn Reg) {
		blk.Append(&Inst{Op: extOp, Dst: ea, Src: [2]Reg{a}, SrcTy: ty, Ty: TyU16, Cls: ClassPointer})
		if !same {
			blk.Append(&Inst{Op: extOp, Dst: eb, Src: [2]Reg{b}, SrcTy: ty, Ty: TyU16, Cls: ClassPointer})
		}
		mul = &Inst{Op: OpMul, Dst: prod, Src: [2]Reg{ea, rhsExt}, Ty: TyU16, Cls: ClassPointer}
		blk.Append(mul)
		blk.Append(&Inst{Op: OpAdd, Dst: acc2, Src: [2]Reg{accIn, prod}, Ty: TyU16, Cls: ClassPointer})
	}
	if !inLoop {
		body = f.NewBlock("entry")
		acc = f.AllocReg()
		body.Append(&Inst{Op: OpConst, Dst: acc, Ty: TyU16, Imm: 7})
		emitBody(body, acc)
		body.Seal(&TermRet{Vals: []Reg{acc2}})
		return
	}
	entry := f.NewBlock("entry")
	head := f.NewBlock("head")
	body = f.NewBlock("body")
	exit := f.NewBlock("exit")
	pi, pacc, pres := f.AllocReg(), f.AllocReg(), f.AllocReg()
	head.Params = []BlockParam{{Dst: pi, Ty: TyU16, Class: ClassGeneral}, {Dst: pacc, Ty: TyU16, Class: ClassGeneral}}
	exit.Params = []BlockParam{{Dst: pres, Ty: TyU16, Class: ClassGeneral}}
	zero, one, i2, ne := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	entry.Append(&Inst{Op: OpConst, Dst: zero, Ty: TyU16, Imm: 0})
	entry.Seal(&TermJmp{Target: "head", Args: []Reg{zero, zero}})
	head.Append(&Inst{Op: OpCmp, Cond: CmpNe, Dst: ne, Src: [2]Reg{pi, limit}, Ty: TyBool, Cls: ClassFlag})
	head.Seal(&TermBrIf{Cond: ne, Then: "body", Else: "exit", ElseArgs: []Reg{pacc}})
	emitBody(body, pacc)
	body.Append(&Inst{Op: OpConst, Dst: one, Ty: TyU16, Imm: 1})
	body.Append(&Inst{Op: OpAdd, Dst: i2, Src: [2]Reg{pi, one}, Ty: TyU16})
	body.Seal(&TermJmp{Target: "head", Args: []Reg{i2, acc2}})
	exit.Seal(&TermRet{Vals: []Reg{pres}})
	acc = pacc
	return
}

func wbmCount(f *Func, op Op) int {
	n := 0
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op == op {
				n++
			}
		}
	}
	return n
}

// The pass narrows exactly the unsigned single-use shape and leaves verified MIR.
func TestNarrowWideningByteMulShapes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		same   bool
		inLoop bool
		ty     Ty
		op     Op
		want   int
	}{
		{"loop", false, true, TyU8, OpExt, 1},
		{"straight", false, false, TyU8, OpExt, 1},
		{"same-extension", true, true, TyU8, OpExt, 1},
		{"sext-i8", false, true, TyI8, OpSext, 0},
		{"zext-of-i8", false, true, TyI8, OpExt, 0},
	} {
		m, f, _, mul, a, b, _ := wbmLoop(t, tc.same, tc.inLoop, tc.ty, tc.op)
		if got := NarrowWideningByteMul(f); got != tc.want {
			t.Fatalf("%s: rewrote %d multiplies, want %d", tc.name, got, tc.want)
		}
		if err := Verify(m); err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, m.Dump())
		}
		if tc.want == 0 {
			continue
		}
		rhs := b
		if tc.same {
			rhs = a
		}
		if mul.Src[0] != a || mul.Src[1] != rhs || wbmCount(f, OpExt) != 0 {
			t.Fatalf("%s: multiply not narrowed to the bytes\n%s", tc.name, m.Dump())
		}
	}

	// An extension with a second reader keeps its word value.
	_, f, body, mul, _, _, _ := wbmLoop(t, false, true, TyU8, OpExt)
	extra := f.AllocReg()
	body.Insts = append(body.Insts[:3], append([]*Inst{{Op: OpAdd, Dst: extra, Src: [2]Reg{mul.Src[0], mul.Src[0]}, Ty: TyU16}}, body.Insts[3:]...)...)
	if NarrowWideningByteMul(f) != 0 {
		t.Fatal("an extension read elsewhere must not be narrowed away")
	}

	// A redefinition of the byte between its extension and the multiply.
	_, f, body, mul, a, _, _ := wbmLoop(t, false, true, TyU8, OpExt)
	at := 0
	for i, inst := range body.Insts {
		if inst == mul {
			at = i
		}
	}
	body.Insts = append(body.Insts[:at], append([]*Inst{{Op: OpConst, Dst: a, Ty: TyU8, Imm: 3}}, body.Insts[at:]...)...)
	if NarrowWideningByteMul(f) != 0 {
		t.Fatal("a byte redefined after its extension must not be read by the multiply")
	}

	// A 32-bit multiply is not this shape.
	_, f, _, mul, _, _, _ = wbmLoop(t, false, true, TyU8, OpExt)
	mul.Ty = TyU32
	if NarrowWideningByteMul(f) != 0 {
		t.Fatal("only 16-bit multiplies are narrowed")
	}
}

// The MIR2 VM gives the exact product for every byte pair after the rewrite.
func TestNarrowWideningByteMulVMExhaustive(t *testing.T) {
	for _, same := range []bool{false, true} {
		m, f, _, _, _, _, _ := wbmLoop(t, same, true, TyU8, OpExt)
		if NarrowWideningByteMul(f) != 1 {
			t.Fatal("pass did not fire")
		}
		vm := NewVM(m)
		calls := 0
		for x := 0; x < 256; x++ {
			for y := 0; y < 256; y++ {
				want := uint16(x * y)
				if same {
					want = uint16(x * x)
				}
				// Two iterations: acc = 2*product (mod 65536).
				got, e := vm.Call(f.Name, []Value{{I: 2}, {I: int64(x)}, {I: int64(y)}})
				calls++
				if e != nil || len(got) != 1 || uint16(got[0].I) != uint16(2*int(want)) {
					t.Fatalf("same=%v %d*%d: VM %v (%v), want 2*%d", same, x, y, got, e, want)
				}
				if same {
					break
				}
			}
		}
		if want := map[bool]int{false: 65536, true: 256}[same]; calls != want {
			t.Fatalf("same=%v: %d VM calls, want %d", same, calls, want)
		}
	}
}

// wbmHome places a value in a named byte home.
func wbmHome(regs *emulator.Registers, ram map[uint16]byte, name string, v byte) {
	switch name {
	case "A":
		regs.A = v
	case "B":
		regs.BC = regs.BC&0x00ff | uint16(v)<<8
	case "C":
		regs.BC = regs.BC&0xff00 | uint16(v)
	case "D":
		regs.DE = regs.DE&0x00ff | uint16(v)<<8
	case "E":
		regs.DE = regs.DE&0xff00 | uint16(v)
	case "H":
		regs.HL = regs.HL&0x00ff | uint16(v)<<8
	case "L":
		regs.HL = regs.HL&0xff00 | uint16(v)
	case "IXH":
		regs.IX = regs.IX&0x00ff | uint16(v)<<8
	case "IXL":
		regs.IX = regs.IX&0xff00 | uint16(v)
	case "IYH":
		regs.IY = regs.IY&0x00ff | uint16(v)<<8
	case "IYL":
		regs.IY = regs.IY&0xff00 | uint16(v)
	case "spill":
		ram[0xa112] = v
	case "spill2":
		ram[0xa114] = v
	default:
		panic(name)
	}
}

func wbmLoc(name string) PhysLoc {
	switch name {
	case "spill":
		return PhysLoc{Kind: LocMem, Offset: 0xa112}
	case "spill2":
		return PhysLoc{Kind: LocMem, Offset: 0xa114}
	case "dstspill":
		return PhysLoc{Kind: LocMem, Offset: 0xa140}
	}
	return PhysLoc{Kind: LocReg, Name: name}
}

type wbmCase struct {
	lhs, rhs, dst, live string // live: "BC", "HL" or "" (no live pair)
	same, size, inLoop  bool
	mac                 bool // acc and sum in HL, the add fused into the multiply
}

func (c wbmCase) String() string {
	return fmt.Sprintf("lhs=%s rhs=%s dst=%s live=%q same=%v size=%v loop=%v mac=%v", c.lhs, c.rhs, c.dst, c.live, c.same, c.size, c.inLoop, c.mac)
}

// wbmBuild narrows the fixture, assigns homes and emits the multiply.
func wbmBuild(t *testing.T, c wbmCase) (asm string, binary []byte, locs map[Reg]PhysLoc, add *Inst, g *z80cg) {
	t.Helper()
	m, f, body, mul, a, b, acc := wbmLoop(t, c.same, c.inLoop, TyU8, OpExt)
	if NarrowWideningByteMul(f) != 1 {
		t.Fatalf("%v: pass did not fire", c)
	}
	locs = map[Reg]PhysLoc{}
	next := 0xa200
	for _, p := range f.Contract.Params {
		locs[p.Reg] = PhysLoc{Kind: LocMem, Offset: next}
		next += 2
	}
	for _, blk := range f.Blocks {
		for _, p := range blk.Params {
			locs[p.Dst] = PhysLoc{Kind: LocMem, Offset: next}
			next += 2
		}
		for _, i := range blk.Insts {
			if i.Dst != NoReg {
				locs[i.Dst] = PhysLoc{Kind: LocMem, Offset: next}
				next += 2
			}
		}
	}
	locs[a] = wbmLoc(c.lhs)
	if !c.same {
		locs[b] = wbmLoc(c.rhs)
	}
	locs[mul.Dst] = wbmLoc(c.dst)
	idx := 0
	for i, inst := range body.Insts {
		if inst == mul {
			idx = i
		}
	}
	add = body.Insts[idx+1]
	if c.mac {
		locs[acc] = wbmLoc("HL")
		locs[add.Dst] = wbmLoc("HL")
		if c.live == "BC" && c.inLoop {
			locs[f.BlockByLabel("head").Params[0].Dst] = wbmLoc("BC") // loop counter
		}
	} else if c.live != "" {
		locs[acc] = wbmLoc(c.live)
	}
	var sb strings.Builder
	g = &z80cg{mod: m, fn: f, ar: &AllocResult{Locs: locs}, sb: &sb, curBlock: body, curInstIdx: idx, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg, deadConsts: map[Reg]bool{}, optimizeForSize: c.size}
	if !c.mac {
		// Another instruction between the multiply and the add: no fusion.
		g.curInstIdx = -1
	}
	g.genMul16(mul)
	if g.err != nil {
		t.Fatal(g.err)
	}
	asm = sb.String()
	binary = qsqAssemble(t, f.Name, asm, locs)
	return
}

// wbmCheck runs one byte pair and checks the product, every preserved
// register and every untouched memory byte.
func wbmCheck(t *testing.T, c wbmCase, asm string, binary []byte, x, y int, accIn uint16) {
	t.Helper()
	if c.same {
		y = x
	}
	want := uint16(x * y)
	regs := mul16PreservedRegs(0xabcd)
	if c.mac {
		regs.HL = accIn
	}
	ram := map[uint16]byte{0xa112: 0x5a, 0xa113: 0xa5, 0xa114: 0x5a, 0xa115: 0xa5, 0xa140: 0x11, 0xa141: 0x22}
	wbmHome(&regs, ram, c.lhs, byte(x))
	if !c.same {
		wbmHome(&regs, ram, c.rhs, byte(y))
	}
	z := emulator.NewRemogattoZ80()
	if e := z.LoadMemory(0x8000, binary); e != nil {
		t.Fatal(e)
	}
	mem := z.Memory()
	for addr, v := range ram {
		mem[addr] = v
	}
	mem[0xff00], mem[0xff01] = 0, 0x70
	z.SetRegisters(regs)
	for n := 0; (z.GetPC() != 0x7000 || z.GetSP() != 0xff02) && n < 4000; n++ {
		z.Step()
	}
	got := z.GetRegisters()
	fail := func(what string) {
		t.Fatalf("%v x=%d y=%d acc=%#x: %s\nbefore %+v\nafter  %+v\n%s", c, x, y, accIn, what, regs, got, asm)
	}
	if got.PC != 0x7000 || got.SP != 0xff02 {
		fail("did not return with a balanced stack")
	}
	if got.A != regs.A {
		fail("A not preserved")
	}
	expect := map[string]uint16{"DE": regs.DE, "IX": regs.IX, "IY": regs.IY}
	if c.live != "" && !c.mac {
		expect[c.live] = map[string]uint16{"BC": regs.BC, "HL": regs.HL}[c.live]
	}
	if c.mac {
		expect["HL"] = accIn + want
		if c.live == "BC" {
			expect["BC"] = regs.BC
		}
	} else if c.dst == "dstspill" {
		if v := uint16(mem[0xa140]) | uint16(mem[0xa141])<<8; v != want {
			fail(fmt.Sprintf("spilled product %#x, want %#x", v, want))
		}
	} else {
		expect[c.dst] = want
	}
	actual := map[string]uint16{"BC": got.BC, "DE": got.DE, "HL": got.HL, "IX": got.IX, "IY": got.IY}
	for pair, v := range expect {
		if actual[pair] != v {
			fail(fmt.Sprintf("%s = %#x, want %#x", pair, actual[pair], v))
		}
	}
	for addr, v := range ram {
		if addr >= 0xa140 && c.dst == "dstspill" && !c.mac {
			continue
		}
		if mem[addr] != v {
			fail(fmt.Sprintf("ram[%#x] = %#x, want %#x", addr, mem[addr], v))
		}
	}
}

func wbmGrid() []int {
	return []int{0, 1, 2, 3, 7, 15, 16, 85, 127, 128, 129, 170, 200, 254, 255}
}

// Every operand home, both lowering paths (quarter-square in a speed-mode
// loop, the byte shift-and-add in size mode and outside loops): one operand
// runs through all 256 values against a grid for the other, both ways round.
func TestWideningByteMulOperandHomesNative(t *testing.T) {
	homes := [][2]string{
		{"IXH", "A"}, {"A", "IXH"}, {"B", "C"}, {"C", "B"}, {"D", "E"}, {"E", "D"},
		{"H", "L"}, {"L", "H"}, {"L", "A"}, {"A", "H"}, {"IYL", "IXL"}, {"IXL", "IYH"},
		{"spill", "A"}, {"A", "spill"}, {"spill", "spill2"}, {"C", "spill"}, {"spill", "E"},
		{"E", "spill"}, {"spill", "C"}, {"D", "A"}, {"A", "E"},
	}
	type mode struct{ size, inLoop bool }
	cases := 0
	for _, md := range []mode{{false, true}, {true, true}, {false, false}} {
		for _, h := range homes {
			// The live pair may not share a register with an operand. The
			// product may: an operand dies at the multiply (IXH -> IX is
			// the benchmark's own allocation).
			c := wbmCase{lhs: h[0], rhs: h[1], dst: "IX", size: md.size, inLoop: md.inLoop}
			uses := func(rs ...string) bool {
				for _, r := range rs {
					if h[0] == r || h[1] == r {
						return true
					}
				}
				return false
			}
			switch {
			case !uses("B", "C"):
				c.live = "BC"
			case !uses("H", "L"):
				c.live = "HL"
			}
			asm, binary, _, _, _ := wbmBuild(t, c)
			table := strings.Contains(asm, "__qsq")
			if table != (!md.size && md.inLoop) {
				t.Fatalf("%v: quarter-square=%v\n%s", c, table, asm)
			}
			if strings.Contains(asm, "LD IXH, 0") || strings.Contains(asm, "LD A, 0") || strings.Contains(asm, "16-iter") {
				t.Fatalf("%v: byte operands widened\n%s", c, asm)
			}
			for v := 0; v < 256; v++ {
				for _, w := range wbmGrid() {
					wbmCheck(t, c, asm, binary, v, w, 0)
					wbmCheck(t, c, asm, binary, w, v, 0)
					cases += 2
				}
			}
		}
	}
	t.Logf("%d native executions", cases)
}

// Equal operands (u16(x) * u16(x)), each home, both paths, every value.
func TestWideningByteMulSquareNative(t *testing.T) {
	for _, home := range []string{"A", "C", "H", "IXH", "IYL", "spill"} {
		for _, size := range []bool{false, true} {
			c := wbmCase{lhs: home, dst: "DE", live: "HL", same: true, size: size, inLoop: true}
			if home == "H" {
				c.live = "BC"
			}
			asm, binary, _, _, _ := wbmBuild(t, c)
			for v := 0; v < 256; v++ {
				wbmCheck(t, c, asm, binary, v, v, 0)
			}
		}
	}
}

// Result homes and live pairs: a restore must never overwrite the product.
func TestWideningByteMulResultHomesNative(t *testing.T) {
	for _, dl := range [][2]string{
		{"HL", "BC"}, {"HL", ""}, {"DE", "BC"}, {"DE", "HL"}, {"BC", "HL"}, {"BC", ""},
		{"IX", "BC"}, {"IX", "HL"}, {"dstspill", "BC"}, {"dstspill", "HL"},
	} {
		for _, size := range []bool{false, true} {
			c := wbmCase{lhs: "A", rhs: "spill", dst: dl[0], live: dl[1], size: size, inLoop: true}
			asm, binary, _, _, _ := wbmBuild(t, c)
			for v := 0; v < 256; v++ {
				for _, w := range wbmGrid() {
					wbmCheck(t, c, asm, binary, v, w, 0)
					wbmCheck(t, c, asm, binary, w, v, 0)
				}
			}
		}
	}
}

// Full 256x256 products for the benchmark's homes on both paths, and for the
// multiply-accumulate fusion with a wrapping accumulator.
func TestWideningByteMulExhaustiveNative(t *testing.T) {
	for _, c := range []wbmCase{
		{lhs: "IXH", rhs: "A", dst: "IX", live: "BC", inLoop: true},
		{lhs: "IXH", rhs: "A", dst: "IX", live: "BC", inLoop: true, size: true},
		{lhs: "D", rhs: "C", dst: "HL", inLoop: false},
		{lhs: "IXH", rhs: "A", dst: "IX", live: "BC", inLoop: true, mac: true},
		{lhs: "spill", rhs: "E", dst: "IX", live: "BC", inLoop: true, size: true, mac: true},
	} {
		asm, binary, _, add, g := wbmBuild(t, c)
		if c.mac != g.widenedByteAdds[add] || c.mac != strings.Contains(asm, "multiply-accumulate") {
			t.Fatalf("%v: fused add=%v\n%s", c, g.widenedByteAdds[add], asm)
		}
		for x := 0; x < 256; x++ {
			for y := 0; y < 256; y++ {
				acc := uint16(0)
				if c.mac {
					acc = []uint16{0, 0x0001, 0x7fff, 0xff00, 0xffff}[(x+y)%5]
				}
				wbmCheck(t, c, asm, binary, x, y, acc)
			}
		}
	}
}

// The fusion requires the add to follow directly and to own the only use.
func TestWideningByteMulAccumulateAdmission(t *testing.T) {
	c := wbmCase{lhs: "IXH", rhs: "A", dst: "IX", live: "BC", inLoop: true, mac: true}
	m, f, body, mul, _, _, acc := wbmLoop(t, false, true, TyU8, OpExt)
	_ = m
	NarrowWideningByteMul(f)
	idx := 0
	for i, inst := range body.Insts {
		if inst == mul {
			idx = i
		}
	}
	add := body.Insts[idx+1]
	newG := func(locs map[Reg]PhysLoc) *z80cg {
		return &z80cg{mod: m, fn: f, ar: &AllocResult{Locs: locs}, sb: &strings.Builder{}, curBlock: body, curInstIdx: idx, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg, deadConsts: map[Reg]bool{}}
	}
	locs := map[Reg]PhysLoc{acc: wbmLoc("HL"), add.Dst: wbmLoc("HL"), mul.Dst: wbmLoc("IX")}
	if newG(locs).mulAccumulate(mul) != add {
		t.Fatalf("%v: expected the fusion", c)
	}
	locs[add.Dst] = wbmLoc("DE")
	if newG(locs).mulAccumulate(mul) != nil {
		t.Fatal("a sum outside HL must not fuse")
	}
	locs[add.Dst] = wbmLoc("HL")
	// A second reader of the product.
	extra := f.AllocReg()
	body.Insts = append(body.Insts, &Inst{Op: OpAdd, Dst: extra, Src: [2]Reg{mul.Dst, mul.Dst}, Ty: TyU16})
	if newG(locs).mulAccumulate(mul) != nil {
		t.Fatal("a product with another reader must not fuse")
	}
}

// Signed bytes never reach the byte paths: the pass refuses them, and an i8
// operand of a word multiply is not an unsigned byte for codegen either.
func TestWideningByteMulSignedOperandsKeepWidening(t *testing.T) {
	_, f, body, mul, a, b, _ := wbmLoop(t, false, true, TyI8, OpSext)
	if NarrowWideningByteMul(f) != 0 {
		t.Fatal("sign-extended operands must not be narrowed")
	}
	var sb strings.Builder
	locs := map[Reg]PhysLoc{a: wbmLoc("A"), b: wbmLoc("C")}
	g := &z80cg{fn: f, ar: &AllocResult{Locs: locs}, sb: &sb, curBlock: body, regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg}
	if g.mulByteOperand(a) || g.mulByteOperand(b) {
		t.Fatal("i8 operands must not count as unsigned bytes")
	}
	_ = mul
}

// A pair saved and restored inside the sequence is not a lasting write, but
// only in straight-line code that leaves SP to PUSH and POP.
func TestScratchSequenceWritesBalancedPushPop(t *testing.T) {
	for _, tc := range []struct {
		asm     string
		written []string
		kept    []string
	}{
		{"PUSH DE\nLD D, IXH\nLD E, IXL\nADD HL, DE\nPOP DE", []string{"H", "L"}, []string{"D", "E"}},
		{"LD E, 1\nPUSH DE\nLD D, 2\nPOP DE", []string{"E"}, []string{"D"}},
		{"PUSH HL\nPOP IX", []string{"IXH", "IXL"}, []string{"H", "L"}},
		{"PUSH DE\nPUSH BC\nLD B, 1\nLD D, 2\nPOP BC\nPOP DE", nil, []string{"B", "C", "D", "E"}},
		{"PUSH DE\nPUSH BC\nLD D, 2\nPOP DE\nPOP BC", []string{"B", "C", "D", "E"}, nil},
		{"POP DE", []string{"D", "E"}, nil},
		{"PUSH AF\nLD A, 1\nPOP AF", nil, []string{"A"}},
		// Control flow or SP arithmetic: POP is a plain write again.
		{"PUSH DE\nLD D, 1\nJR NC, .x\n.x:\nPOP DE", []string{"D", "E"}, nil},
		{"PUSH DE\nLD D, 1\nEX (SP), HL\nPOP DE", []string{"D", "E"}, nil},
		{"PUSH DE\nINC SP\nLD D, 1\nPOP DE", []string{"D", "E"}, nil},
		// A spill label is not SP.
		{"PUSH DE\nLD A, (_spill_f_r1)\nLD D, A\nPOP DE", []string{"A"}, []string{"D", "E"}},
	} {
		w := scratchSequenceWrites(tc.asm)
		for _, r := range tc.written {
			if !w[r] {
				t.Errorf("%q: %s should be a lasting write (%v)", tc.asm, r, w)
			}
		}
		for _, r := range tc.kept {
			if w[r] {
				t.Errorf("%q: %s is restored, not written (%v)", tc.asm, r, w)
			}
		}
	}
}
