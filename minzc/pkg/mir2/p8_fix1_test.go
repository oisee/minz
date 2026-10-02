package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestCriticU16ConstBlockParamSpill(t *testing.T) {
	testConstBlockParamSpill(t, mir2.TyU16, 0x1234)
}

func TestP8WideConstBlockParamSpill(t *testing.T) {
	for _, tc := range []struct {
		ty    mir2.Ty
		value int64
	}{{mir2.TyU24, 0x123456}, {mir2.TyU32, 0x12345678}} {
		t.Run(tc.ty.String(), func(t *testing.T) { testConstBlockParamSpill(t, tc.ty, tc.value) })
	}
}

func testConstBlockParamSpill(t *testing.T, ty mir2.Ty, value int64) {
	m := &mir2.Module{Name: "c"}
	f := m.AddFunc("arith")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
	b := mir2.NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	next := b.SwitchToNewBlock("next")
	class := mir2.ClassPair
	if ty.Width() > 16 {
		class = mir2.ClassDWord
	}
	p := b.BlockParam(next, ty, class)
	b.SwitchTo(entry)
	x := b.Const(value, ty, class)
	b.Jmp("next", x)
	b.SwitchTo(next)
	if ty.Width() > 16 {
		f.Contract.Returns = nil
		b.Ret()
		ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{p: {Kind: mir2.LocMem}, x: {Kind: mir2.LocDWord, Name: "DE"}}}
		code := mustZ80Asm(mir2.Z80Codegen(m, ar))
		for i := 0; i < mir2.ByteWidth(ty); i++ {
			// Poison every byte so an omitted store cannot pass via zero-initialised data.
			pre := "ORG 0x8000\nLD SP,0xFF00\nLD A,255\n"
			for j := 0; j < mir2.ByteWidth(ty); j++ {
				pre += fmt.Sprintf("LD (_spill_arith_r%d+%d),A\n", p, j)
			}
			asm := pre + fmt.Sprintf("CALL arith\nLD A,(_spill_arith_r%d+%d)\nDI\nHALT\n", p, i) + code
			if got := fix1Execute(t, asm).A; got != uint8(value>>(8*i)) {
				t.Fatalf("byte %d got %x want %x\n%s", i, got, uint8(value>>(8*i)), asm)
			}
		}
	} else {
		r := b.Move(p, ty, class)
		b.Ret(r)
		ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{p: {Kind: mir2.LocMem}, x: {Kind: mir2.LocReg, Name: "DE"}, r: {Kind: mir2.LocReg, Name: "HL"}}}
		asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD HL,0xFFFF\nLD (_spill_arith_r%d),HL\nCALL arith\nDI\nHALT\n", p) + mustZ80Asm(mir2.Z80Codegen(m, ar))
		if got := fix1Execute(t, asm).HL; got != uint16(value) {
			t.Fatalf("got %x want %x\n%s", got, value, asm)
		}
	}
}

func TestP8AddressSpillIsWord(t *testing.T) {
	for _, ty := range []mir2.Ty{&mir2.StructTy{Name: "s", Fields: []mir2.StructField{{Name: "a", Ty: mir2.TyU32}, {Name: "b", Ty: mir2.TyU32}}}, mir2.PtrFor(32)} {
		for _, explicit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/explicit=%v", ty, explicit), func(t *testing.T) {
				m := &mir2.Module{Name: "c"}
				f := m.AddFunc("arith")
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				a := b.Param("a", ty, mir2.ClassPair)
				r := b.Move(a, ty, mir2.ClassPair)
				b.Ret()
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocReg, Name: "DE"}, r: {Kind: mir2.LocMem}}}
				if explicit {
					ar.Spilled = []mir2.Reg{r}
				}
				asm := mustZ80Asm(mir2.Z80Codegen(m, ar))
				slot := fmt.Sprintf("_spill_arith_r%d:", r)
				if !strings.Contains(asm, slot+" DW 0") && !strings.Contains(asm, slot+" DB 0, 0\n") {
					t.Fatalf("expected two-byte slot\n%s", asm)
				}
				if strings.Contains(asm, "EXX") {
					t.Fatalf("address used wide staging\n%s", asm)
				}
				fix1Execute(t, "ORG 0x8000\nLD SP,0xFF00\nCALL arith\nDI\nHALT\n"+asm)
			})
		}
	}
}
