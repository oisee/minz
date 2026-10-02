package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func runWideZ80(t *testing.T, m *mir2.Module, alloc *mir2.AllocResult, value uint32, count uint8) uint32 {
	t.Helper()
	boot := fmt.Sprintf("ORG 0x8000\nLD SP, 0xFF00\nEXX\nLD HL, %d\nEXX\nLD HL, %d\nLD A, %d\nLD C, %d\nCALL wide\nEXX\nPUSH HL\nEXX\nPOP DE\nDI\nHALT\n", value>>16, value&65535, value&255, count)
	asm := boot + mir2.Z80Codegen(m, alloc)
	res, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble %v %v\n%s", err, res.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	if err := z.LoadMemory(0x8000, res.Binary); err != nil {
		t.Fatal(err)
	}
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
	for step := 0; !z.IsHalted(); step++ {
		if step >= 200000 {
			t.Fatalf("no HALT\n%s", asm)
		}
		z.Step()
	}
	r := z.GetRegisters()
	return uint32(r.DE)<<16 | uint32(r.HL)
}

func TestZ80SignedWideShift(t *testing.T) {
	for _, ty := range []mir2.Ty{mir2.TyI24, mir2.TyI32} {
		for _, variable := range []bool{false, true} {
			for _, k := range []uint8{0, 1, 7, 16, 23} {
				m := &mir2.Module{Name: "wide"}
				f := m.AddFunc("wide")
				f.Contract.Returns = []mir2.Return{{Ty: ty, Class: mir2.ClassDWord}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				a := b.Param("a", ty, mir2.ClassDWord)
				var count mir2.Reg
				if variable {
					count = b.Param("k", mir2.TyU8, mir2.ClassGeneral)
				} else {
					count = b.Const(int64(k), mir2.TyU8, mir2.ClassGeneral)
				}
				out := b.Sar(a, count, ty, mir2.ClassDWord)
				b.Ret(out)
				alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocDWord, Name: "HL"}, out: {Kind: mir2.LocDWord, Name: "HL"}, count: {Kind: mir2.LocReg, Name: "C"}}}
				value := uint32(0x800003)
				if ty.Width() == 32 {
					value = 0x80000003
				}
				shift := 64 - ty.Width()
				want := uint32((int64(value)<<shift)>>shift>>k) & uint32((uint64(1)<<ty.Width())-1)
				got := runWideZ80(t, m, alloc, value, k) & uint32((uint64(1)<<ty.Width())-1)
				if got != want {
					t.Errorf("%s variable=%v k=%d: got %x want %x", ty, variable, k, got, want)
				}
			}
		}
	}
}

func TestZ80SignedWideCast(t *testing.T) {
	for _, srcTy := range []mir2.Ty{mir2.TyI8, mir2.TyI16, mir2.TyI24} {
		for _, ty := range []mir2.Ty{mir2.TyI16, mir2.TyI24, mir2.TyI32} {
			if srcTy.Width() >= ty.Width() {
				continue
			}
			m := &mir2.Module{Name: "wide"}
			f := m.AddFunc("wide")
			cls := mir2.ClassDWord
			if ty.Width() == 16 {
				cls = mir2.ClassPair
			}
			f.Contract.Returns = []mir2.Return{{Ty: ty, Class: cls}}
			srcClass, srcLoc := mir2.ClassAcc, mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
			if srcTy.Width() == 16 {
				srcClass = mir2.ClassPair
				srcLoc = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
			}
			if srcTy.Width() == 24 {
				srcClass = mir2.ClassDWord
				srcLoc = mir2.PhysLoc{Kind: mir2.LocDWord, Name: "HL"}
			}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			a := b.Param("a", srcTy, srcClass)
			out := b.Sext(a, srcTy, ty, cls)
			b.Ret(out)
			alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: srcLoc, out: {Kind: mir2.LocDWord, Name: "HL"}}}
			mask := uint32((uint64(1) << ty.Width()) - 1)
			for _, a := range []uint32{0, 127, uint32(1) << (srcTy.Width() - 1), uint32((uint64(1) << srcTy.Width()) - 4), uint32((uint64(1) << srcTy.Width()) - 1)} {
				got := runWideZ80(t, m, alloc, a, 0) & mask
				shift := 64 - srcTy.Width()
				want := uint32((int64(a)<<shift)>>shift) & mask
				if got != want {
					t.Errorf("%s->%s sext(%d): got %x want %x", srcTy, ty, a, got, want)
				}
			}
		}
	}
}

func TestSignedWideConstantFold(t *testing.T) {
	for _, ty := range []mir2.Ty{mir2.TyI16, mir2.TyI24, mir2.TyI32} {
		for _, a := range []int64{0, 1, -1, -4, -32768, 32767} {
			for k := 0; k < ty.Width(); k++ {
				m := &mir2.Module{Name: "fold"}
				f := m.AddFunc("fold")
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				lhs := b.Const(a, ty, mir2.ClassGeneral)
				rhs := b.Const(int64(k), mir2.TyU8, mir2.ClassGeneral)
				out := b.Sar(lhs, rhs, ty, mir2.ClassGeneral)
				b.Ret(out)
				want := (a >> k) & ((int64(1) << ty.Width()) - 1)
				for _, phase := range []string{"raw", "folded"} {
					if phase == "folded" && !mir2.FoldConstants(f) {
						t.Fatal("constant sar did not fold")
					}
					got, err := mir2.NewVM(m).Call("fold", nil)
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("%s %s sar(%d,%d): %v %v want %d", phase, ty, a, k, got, err, want)
					}
				}
			}
		}
	}
}

func TestZ80SignedDivisionAccumulatorDivisor(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpSDiv, mir2.OpSMod} {
		m := &mir2.Module{Name: "alias"}
		f := m.AddFunc("wide")
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyI8, Class: mir2.ClassAcc}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		lhs := b.Param("lhs", mir2.TyI8, mir2.ClassGeneral)
		rhs := b.Param("rhs", mir2.TyI8, mir2.ClassAcc)
		out := b.BinOp(op, lhs, rhs, mir2.TyI8, mir2.ClassAcc)
		ret := b.Sext(out, mir2.TyI8, mir2.TyI16, mir2.ClassPair)
		b.Ret(ret)
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyI16, Class: mir2.ClassPair}}
		alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{lhs: {Kind: mir2.LocReg, Name: "C"}, rhs: {Kind: mir2.LocReg, Name: "A"}, out: {Kind: mir2.LocReg, Name: "A"}, ret: {Kind: mir2.LocReg, Name: "HL"}}}
		want := uint16(65533)
		if op == mir2.OpSMod {
			want = 65535
		}
		// Bootstrap places dividend -7 in C and divisor 2 in A.
		got := uint16(runWideZ80(t, m, alloc, 2, 249))
		if got != want {
			t.Errorf("%s divisor A: got %d want %d", op, got, want)
		}
	}
}
