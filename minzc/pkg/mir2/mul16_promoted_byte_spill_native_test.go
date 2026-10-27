package mir2

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// mul16LoopCounter reports the iteration count of the emitted shift-and-add
// loop: the immediate loaded into the counter register (A for the DEC A body,
// B for the DJNZ body). 0 when no loop was emitted (constant strength
// reduction), -1 when the body is ambiguous.
func mul16LoopCounter(body string) int {
	re := regexp.MustCompile(`(?m)^\s+LD [AB], (8|16)\s*$`)
	found := 0
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		n, _ := strconv.Atoi(m[1])
		if found != 0 && found != n {
			return -1
		}
		found = n
	}
	return found
}

// assembleMul16 runs genMul16 on one hand-built function and assembles the
// result, inventoring EQU lines for every spilled location.
func assembleMul16(t *testing.T, m *Module, f *Func, b *Block, inst *Inst, locs map[Reg]PhysLoc, consts map[Reg]int64) (string, []byte) {
	t.Helper()
	var body strings.Builder
	g := &z80cg{mod: m, fn: f, ar: &AllocResult{Locs: locs}, sb: &body, curBlock: b, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg}
	for r, v := range consts {
		g.constVals[r] = v
	}
	g.genMul16(inst)
	equs := ""
	for r, pl := range locs {
		if pl.Kind == LocMem {
			equs += fmt.Sprintf("%s EQU $%04X\n", g.spillLabel(r), pl.Offset)
		}
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nmultiply_bytes:\n" + body.String() + "\nRET\n" + equs)
	if e != nil {
		t.Fatal(e, body.String())
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors, body.String())
	}
	return body.String(), a.Binary
}

// runMul16Native executes one assembled multiply on the Z80 emulator and checks
// the result, the preserved registers and the untouched memory homes.
func runMul16Native(t *testing.T, binary []byte, ramInit map[uint16]byte, before emulator.Registers, want uint16, desc, body string) {
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
	if got.HL != want || got.A != before.A || got.BC != before.BC || got.DE != before.DE || got.IX != before.IX || got.IY != before.IY || got.PC != 0x7000 || got.SP != 0xff02 {
		t.Fatalf("%s got%+v want%x\n%s", desc, got, want, body)
	}
	for addr, v := range ramInit {
		if ram[addr] != v {
			t.Fatalf("%s ram[%#x] = %#x, want %#x\n%s", desc, addr, ram[addr], v, body)
		}
	}
}

// mul16PreservedRegs is the register state every fixture must restore.
func mul16PreservedRegs(hl uint16) emulator.Registers {
	return emulator.Registers{PC: 0x8000, SP: 0xff00, HL: hl, BC: 0x1234, DE: 0x5678, IX: 0x9abc, IY: 0xcdef, A: 0x37, F: 0xa5}
}

// Promoted byte operands occupy one byte of spill memory, even when the
// multiply produces a word. Poison adjacent bytes to detect widened reads.
func TestMul16PromotedByteSpillNative(t *testing.T) {
	cases := 0
	for _, ty := range []Ty{TyU8, TyI8} {
		// -1 selects the runtime multiplier path; 200 is a constant that
		// falls through strength reduction into the shift-and-add loop.
		for _, factor := range []int{-1, 2, 3, 200} {
			m := &Module{}
			f := m.AddFunc("multiply_bytes")
			b := f.NewBlock("entry")
			x, y, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
			var rhsTy Ty = TyU16
			if factor < 0 {
				rhsTy = ty
			}
			f.Contract.Params = []Param{{Name: "x", Reg: x, Ty: ty, Class: ClassGeneral}, {Name: "y", Reg: y, Ty: rhsTy, Class: ClassGeneral}}
			inst := &Inst{Op: OpMul, Dst: out, Src: [2]Reg{x, y}, Ty: TyU16, Cls: ClassPointer}
			// The fixture checks BC preservation: make that value live after mul.
			liveBC := f.AllocReg()
			b.Append(&Inst{Op: OpConst, Dst: liveBC, Ty: TyU16, Imm: 0x1234})
			b.Append(inst)
			b.Append(&Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{liveBC}, Ty: TyU16})
			b.Seal(&TermRet{Vals: []Reg{out}})
			ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocMem, Offset: 0xa100}, y: {Kind: LocMem, Offset: 0xa110}, out: {Kind: LocReg, Name: "HL"}}}
			ar.Locs[liveBC] = PhysLoc{Kind: LocReg, Name: "BC"}
			var body strings.Builder
			g := &z80cg{mod: m, fn: f, ar: ar, sb: &body, curBlock: b, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg}
			if factor >= 0 {
				g.constVals[y] = int64(factor)
			}
			g.genMul16(inst)
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nmultiply_bytes:\n" + body.String() + "\nRET\n" + g.spillLabel(x) + " EQU $A100\n" + g.spillLabel(y) + " EQU $A110\n")
			if e != nil {
				t.Fatal(e, body.String())
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors, body.String())
			}
			asm := body.String()
			if factor == 200 {
				// Constant multiplier 200 keeps an all-zero high byte: the
				// loop must stop after 8 iterations for either operand type.
				if n := mul16LoopCounter(asm); n != 8 {
					t.Fatalf("type%v factor%d loop count %d, want 8\n%s", ty, factor, n, asm)
				}
			}
			if factor < 0 {
				// Runtime multiplier: u8 operands are zero-extended bytes, so
				// 8 iterations suffice. i8 operands are sign-extended and can
				// carry a 0xFF high byte: they must keep 16 iterations.
				wantCount := 16
				if ty == TyU8 {
					wantCount = 8
				}
				if n := mul16LoopCounter(asm); n != wantCount {
					t.Fatalf("type%v factor%d loop count %d, want %d\n%s", ty, factor, n, wantCount, asm)
				}
			}
			if factor < 0 {
				// Full 256×256 product sweep over both byte operands.
				for value := 0; value < 256; value++ {
					for rvalue := 0; rvalue < 256; rvalue++ {
						xInt, yInt := value, rvalue
						if ty == TyI8 {
							xInt, yInt = int(int8(byte(value))), int(int8(byte(rvalue)))
						}
						want := uint16(xInt * yInt)
						if value%8 == 0 && ty == TyU8 {
							vm, e := NewVM(m).Call(f.Name, []Value{{I: int64(xInt)}, {I: int64(yInt)}})
							if e != nil || len(vm) != 1 || uint16(vm[0].I) != want {
								t.Fatalf("VM %v %v want%x", vm, e, want)
							}
						}
						for _, poison := range []byte{0, 255} {
							runMul16Native(t, a.Binary, map[uint16]byte{0xa100: byte(value), 0xa101: poison, 0xa110: byte(rvalue), 0xa111: poison}, mul16PreservedRegs(0xabcd), want,
								fmt.Sprintf("type%v factor%d value%d rvalue%d poison%d", ty, factor, value, rvalue, poison), asm)
							cases++
						}
					}
				}
				continue
			}
			for _, poison := range []byte{0, 1, 128, 255} {
				for value := 0; value < 256; value++ {
					xInt, yInt := value, factor
					rhsByte := byte(factor)
					if ty == TyI8 {
						xInt = int(int8(byte(value)))
					}
					want := uint16(xInt * yInt)
					if ty == TyU8 {
						vm, e := NewVM(m).Call(f.Name, []Value{{I: int64(xInt)}, {I: int64(yInt)}})
						if e != nil || len(vm) != 1 || uint16(vm[0].I) != want {
							t.Fatalf("VM %v %v want%x", vm, e, want)
						}
					}
					runMul16Native(t, a.Binary, map[uint16]byte{0xa100: byte(value), 0xa101: poison, 0xa110: rhsByte, 0xa111: poison}, mul16PreservedRegs(0xabcd), want,
						fmt.Sprintf("type%v factor%d value%d poison%d", ty, factor, value, poison), asm)
					cases++
				}
			}
		}
	}
	t.Logf("%d native executions, %d unsigned VM executions; signed native promotion uses the host oracle; poisoned neighbours and PC/SP checked", cases, cases/2)
}

// A zero-extended byte operand (OpExt u8 -> u16) on the multiplicand side must
// be commutatively staged into the shifting multiplier so the loop can stop
// after 8 iterations.
func TestMul16ExtByteOperandEightIterationsNative(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("multiply_bytes")
	b := f.NewBlock("entry")
	x8, y, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{{Name: "x", Reg: x8, Ty: TyU8, Class: ClassGeneral}, {Name: "y", Reg: y, Ty: TyU16, Class: ClassGeneral}}
	ext := f.AllocReg()
	liveBC := f.AllocReg()
	b.Append(&Inst{Op: OpExt, Dst: ext, Src: [2]Reg{x8}, SrcTy: TyU8, Ty: TyU16})
	b.Append(&Inst{Op: OpConst, Dst: liveBC, Ty: TyU16, Imm: 0x1234})
	inst := &Inst{Op: OpMul, Dst: out, Src: [2]Reg{ext, y}, Ty: TyU16, Cls: ClassPointer}
	b.Append(inst)
	b.Append(&Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{liveBC}, Ty: TyU16})
	b.Seal(&TermRet{Vals: []Reg{out}})
	locs := map[Reg]PhysLoc{x8: {Kind: LocMem, Offset: 0xa100}, y: {Kind: LocMem, Offset: 0xa110}, out: {Kind: LocReg, Name: "HL"}, liveBC: {Kind: LocReg, Name: "BC"}, ext: {Kind: LocReg, Name: "DE"}}
	asm, binary := assembleMul16(t, m, f, b, inst, locs, nil)
	if n := mul16LoopCounter(asm); n != 8 {
		t.Fatalf("ext u8->u16 operand loop count %d, want 8\n%s", n, asm)
	}
	cases := 0
	for _, value := range []int{0, 1, 127, 128, 255} {
		for _, yval := range []int{0, 1, 2, 255, 256, 0x1234, 0x8000, 0xffff} {
			want := uint16(value * yval)
			vm, e := NewVM(m).Call(f.Name, []Value{{I: int64(value)}, {I: int64(yval)}})
			if e != nil || len(vm) != 1 || uint16(vm[0].I) != want {
				t.Fatalf("VM %v %v want%x", vm, e, want)
			}
			for _, poison := range []byte{0, 255} {
				regs := mul16PreservedRegs(0xabcd)
				regs.DE = uint16(value)
				runMul16Native(t, binary, map[uint16]byte{0xa100: byte(value), 0xa101: poison, 0xa110: uint8(yval), 0xa111: uint8(yval >> 8)}, regs, want,
					fmt.Sprintf("ext value%d y%d poison%d", value, yval, poison), asm)
				cases++
			}
		}
	}
	t.Logf("%d native executions with an ext-produced multiplier in DE", cases)
}

// A sign-extended byte operand (OpSext i8 -> i16) can carry a 0xFF high byte:
// it must NOT take the 8-iteration path.
func TestMul16SextByteOperandKeepsSixteenIterationsNative(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("multiply_bytes")
	b := f.NewBlock("entry")
	x8, y, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{{Name: "y", Reg: y, Ty: TyU16, Class: ClassGeneral}, {Name: "x", Reg: x8, Ty: TyI8, Class: ClassGeneral}}
	sext := f.AllocReg()
	liveBC := f.AllocReg()
	b.Append(&Inst{Op: OpSext, Dst: sext, Src: [2]Reg{x8}, SrcTy: TyI8, Ty: TyI16})
	b.Append(&Inst{Op: OpConst, Dst: liveBC, Ty: TyU16, Imm: 0x1234})
	inst := &Inst{Op: OpMul, Dst: out, Src: [2]Reg{y, sext}, Ty: TyU16, Cls: ClassPointer}
	b.Append(inst)
	b.Append(&Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{liveBC}, Ty: TyU16})
	b.Seal(&TermRet{Vals: []Reg{out}})
	locs := map[Reg]PhysLoc{x8: {Kind: LocMem, Offset: 0xa100}, y: {Kind: LocMem, Offset: 0xa110}, out: {Kind: LocReg, Name: "HL"}, liveBC: {Kind: LocReg, Name: "BC"}, sext: {Kind: LocReg, Name: "IX"}}
	asm, binary := assembleMul16(t, m, f, b, inst, locs, nil)
	if n := mul16LoopCounter(asm); n != 16 {
		t.Fatalf("sext i8->i16 operand loop count %d, want 16\n%s", n, asm)
	}
	cases := 0
	for value := 0; value < 256; value++ {
		for _, yval := range []int{0, 1, 255, 256, 0x1234, 0xffff} {
			xInt := int(int8(byte(value)))
			want := uint16(xInt * yval)
			for _, poison := range []byte{0, 255} {
				regs := mul16PreservedRegs(0xabcd)
				regs.IX = uint16(int16(xInt))
				runMul16Native(t, binary, map[uint16]byte{0xa100: byte(value), 0xa101: poison, 0xa110: uint8(yval), 0xa111: uint8(yval >> 8)}, regs, want,
					fmt.Sprintf("sext value%d y%d poison%d", value, yval, poison), asm)
				cases++
			}
		}
	}
	t.Logf("%d native executions with a sext-produced multiplier in IX", cases)
}

// A sign-extended operand may only take the 8-iteration path when its source
// is proven non-negative.
func TestMul16SextNonNegativeOperandNative(t *testing.T) {
	for _, tc := range []struct {
		imm       int64
		wantCount int
	}{
		{100, 8},  // sext(const 100 : i8) is provably 0..127
		{-3, 16},  // sext(const -3 : i8) carries a 0xFF high byte
	} {
		m := &Module{}
		f := m.AddFunc("multiply_bytes")
		b := f.NewBlock("entry")
		y, out := f.AllocReg(), f.AllocReg()
		f.Contract.Params = []Param{{Name: "y", Reg: y, Ty: TyU16, Class: ClassGeneral}}
		c8, sext, liveBC := f.AllocReg(), f.AllocReg(), f.AllocReg()
		b.Append(&Inst{Op: OpConst, Dst: c8, Ty: TyI8, Imm: tc.imm})
		b.Append(&Inst{Op: OpSext, Dst: sext, Src: [2]Reg{c8}, SrcTy: TyI8, Ty: TyI16})
		b.Append(&Inst{Op: OpConst, Dst: liveBC, Ty: TyU16, Imm: 0x1234})
		inst := &Inst{Op: OpMul, Dst: out, Src: [2]Reg{y, sext}, Ty: TyU16, Cls: ClassPointer}
		b.Append(inst)
		b.Append(&Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{liveBC}, Ty: TyU16})
		b.Seal(&TermRet{Vals: []Reg{out}})
		pattern := uint16(uint16(int16(int8(tc.imm))))
		locs := map[Reg]PhysLoc{y: {Kind: LocMem, Offset: 0xa110}, out: {Kind: LocReg, Name: "HL"}, liveBC: {Kind: LocReg, Name: "BC"}, sext: {Kind: LocMem, Offset: 0xa120}}
		asm, binary := assembleMul16(t, m, f, b, inst, locs, nil)
		if n := mul16LoopCounter(asm); n != tc.wantCount {
			t.Fatalf("imm%d loop count %d, want %d\n%s", tc.imm, n, tc.wantCount, asm)
		}
		for _, yval := range []int{0, 1, 255, 256, 0x1234, 0xffff} {
			want := uint16(int64(tc.imm) * int64(yval))
			runMul16Native(t, binary, map[uint16]byte{0xa110: uint8(yval), 0xa111: uint8(yval >> 8), 0xa120: uint8(pattern), 0xa121: uint8(pattern >> 8)}, mul16PreservedRegs(0xabcd), want,
				fmt.Sprintf("imm%d y%d", tc.imm, yval), asm)
		}
	}
}

// A copy of a zero-extended byte (OpMove of an OpExt result) keeps the proof.
func TestMul16MoveOfExtByteOperandEightIterationsNative(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("multiply_bytes")
	b := f.NewBlock("entry")
	x8, y, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{{Name: "x", Reg: x8, Ty: TyU8, Class: ClassGeneral}, {Name: "y", Reg: y, Ty: TyU16, Class: ClassGeneral}}
	ext, mv, liveBC := f.AllocReg(), f.AllocReg(), f.AllocReg()
	b.Append(&Inst{Op: OpExt, Dst: ext, Src: [2]Reg{x8}, SrcTy: TyU8, Ty: TyU16})
	b.Append(&Inst{Op: OpMove, Dst: mv, Src: [2]Reg{ext}, Ty: TyU16})
	b.Append(&Inst{Op: OpConst, Dst: liveBC, Ty: TyU16, Imm: 0x1234})
	inst := &Inst{Op: OpMul, Dst: out, Src: [2]Reg{mv, y}, Ty: TyU16, Cls: ClassPointer}
	b.Append(inst)
	b.Append(&Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{liveBC}, Ty: TyU16})
	b.Seal(&TermRet{Vals: []Reg{out}})
	locs := map[Reg]PhysLoc{x8: {Kind: LocMem, Offset: 0xa100}, y: {Kind: LocMem, Offset: 0xa110}, out: {Kind: LocReg, Name: "HL"}, liveBC: {Kind: LocReg, Name: "BC"}, ext: {Kind: LocReg, Name: "DE"}, mv: {Kind: LocMem, Offset: 0xa120}}
	asm, binary := assembleMul16(t, m, f, b, inst, locs, nil)
	if n := mul16LoopCounter(asm); n != 8 {
		t.Fatalf("move of ext u8->u16 operand loop count %d, want 8\n%s", n, asm)
	}
	for _, value := range []int{0, 1, 127, 128, 255} {
		for _, yval := range []int{0, 1, 255, 256, 0x1234, 0xffff} {
			want := uint16(value * yval)
			vm, e := NewVM(m).Call(f.Name, []Value{{I: int64(value)}, {I: int64(yval)}})
			if e != nil || len(vm) != 1 || uint16(vm[0].I) != want {
				t.Fatalf("VM %v %v want%x", vm, e, want)
			}
			runMul16Native(t, binary, map[uint16]byte{0xa100: byte(value), 0xa101: 255, 0xa110: uint8(yval), 0xa111: uint8(yval >> 8), 0xa120: byte(value), 0xa121: 0}, mul16PreservedRegs(0xabcd), want,
				fmt.Sprintf("move value%d y%d", value, yval), asm)
		}
	}
}
