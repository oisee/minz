package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// buildDoubleRangedModule builds:
//
//	fun double(x: u8<0..127>) -> u8 { return x + x }
//
// The range annotation makes it eligible for LUTGen.
func buildDoubleRangedModule() *mir2.Module {
	m := &mir2.Module{Name: "lut_test"}
	f := m.AddFunc("double_r")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	// Override param type with a ranged version — [0, 128) = 0..127
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.NewRanged(mir2.TyU8, 0, 128), mir2.ClassAcc)
	r := b.Add(x, x, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r)
	return m
}

// buildPopcountRangedModule: popcount(x: u8<0..255>) -> u8
func buildPopcountRangedModule() *mir2.Module {
	m := buildPopcountModule() // reuse the iterative popcount helper
	// Re-annotate the parameter type to be ranged.
	f := m.FuncByName("popcount")
	f.Contract.Params[0].Ty = mir2.NewRanged(mir2.TyU8, 0, 256)
	return m
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestLUTGen_DoubleReplaced(t *testing.T) {
	m := buildDoubleRangedModule()
	changed := mir2.LUTGen(m)
	if !changed {
		t.Fatal("LUTGen: expected double_r to be replaced")
	}

	// Global "double_r_lut" must now exist.
	var lut *mir2.Global
	for i := range m.Globals {
		if m.Globals[i].Name == "double_r_lut" {
			lut = &m.Globals[i]
			break
		}
	}
	if lut == nil {
		t.Fatal("LUTGen: global double_r_lut not found")
	}
	// Table has 128 entries (range 0..127).
	if len(lut.Init) != 128 {
		t.Errorf("LUT size: want 128, got %d", len(lut.Init))
	}
	// Check a few values: double(n) = 2n mod 256
	for _, n := range []int{0, 1, 42, 63, 127} {
		want := byte((n * 2) & 0xFF)
		if lut.Init[n] != want {
			t.Errorf("LUT[%d] = %d, want %d", n, lut.Init[n], want)
		}
	}
	if !lut.IsConst {
		t.Error("LUT global should be read-only (IsConst)")
	}
}

func TestLUTGen_LookupBodyCodegen(t *testing.T) {
	m := buildDoubleRangedModule()
	mir2.LUTGen(m)

	// Compile the function to Z80 and verify it no longer contains ADD A,A.
	f := m.FuncByName("double_r")
	mir2.DeadStoreElim(f)
	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// Body must reference the LUT, not do arithmetic.
	if strings.Contains(asm, "ADD A, A") {
		t.Errorf("LUT body must not contain ADD A,A (arithmetic replaced by lookup):\n%s", asm)
	}
	if !strings.Contains(asm, "double_r_lut") {
		t.Errorf("LUT body must reference double_r_lut:\n%s", asm)
	}
}

func TestLUTGen_PopcountFullRange(t *testing.T) {
	m := buildPopcountRangedModule()
	changed := mir2.LUTGen(m)
	if !changed {
		t.Fatal("LUTGen: expected popcount to be replaced")
	}

	// Verify the 256-entry table is correct for all u8 values.
	var lut *mir2.Global
	for i := range m.Globals {
		if m.Globals[i].Name == "popcount_lut" {
			lut = &m.Globals[i]
			break
		}
	}
	if lut == nil {
		t.Fatal("global popcount_lut not found")
	}
	if len(lut.Init) != 256 {
		t.Fatalf("LUT size: want 256, got %d", len(lut.Init))
	}
	for i := 0; i < 256; i++ {
		want := byte(popcount8(i))
		if lut.Init[i] != want {
			t.Errorf("LUT[%d] = %d, want %d", i, lut.Init[i], want)
		}
	}
}

func TestLUTGen_PageAlignedFastPath(t *testing.T) {
	// popcount has range u8<0..255> → 256 entries → page-aligned fast path.
	// Expected codegen: LD HL, popcount_lut; LD L, A; LD A, (HL)  (21T)
	// NOT: LD E, A; LD D, 0; LD HL, ...; ADD HL, DE; LD A, (HL)  (39T)
	m := buildPopcountRangedModule()
	mir2.LUTGen(m)

	f := m.FuncByName("popcount")
	mir2.DeadStoreElim(f)
	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// Fast path uses LD L, not ADD HL, DE.
	if strings.Contains(asm, "ADD HL, DE") {
		t.Errorf("page-aligned LUT must NOT use ADD HL,DE (expected LD L fast path):\n%s", asm)
	}
	if !strings.Contains(asm, "LD L, A") {
		t.Errorf("page-aligned LUT must use LD L, A:\n%s", asm)
	}
	// ALIGN 256 directive must be emitted before the table.
	if !strings.Contains(asm, "ALIGN 256") {
		t.Errorf("page-aligned LUT must emit ALIGN 256:\n%s", asm)
	}
}

func TestLUTGen_NoFoldWithoutRange(t *testing.T) {
	// double() with plain u8 (not ranged) must NOT be LUT-replaced.
	m := buildDoubleModule()
	changed := mir2.LUTGen(m)
	if changed {
		t.Error("LUTGen: must NOT replace plain-u8 param function")
	}
}

func TestLUTGen_IdempotentAfterRun(t *testing.T) {
	// Second LUTGen pass must not replace the already-replaced body again.
	m := buildDoubleRangedModule()
	mir2.LUTGen(m)
	changed := mir2.LUTGen(m)
	if changed {
		t.Error("LUTGen: second pass should be a no-op (already replaced)")
	}
}

func TestLUTGen_NonZeroLo(t *testing.T) {
	// fun half(x: u8<10..20>) -> u8 { return x / 2 }
	// Range [10,21) → 11 entries; LUT[0] = half(10) = 5, LUT[10] = half(20) = 10.
	m := &mir2.Module{Name: "nonzero_lo"}
	f := m.AddFunc("half")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.NewRanged(mir2.TyU8, 10, 21), mir2.ClassAcc) // [10,21) = 10..20
	one := b.Const(1, mir2.TyU8, mir2.ClassAcc)
	r := b.Shr(x, one, mir2.TyU8, mir2.ClassAcc) // x >> 1 = x/2 for unsigned
	b.Ret(r)

	changed := mir2.LUTGen(m)
	if !changed {
		t.Fatal("LUTGen: expected half to be replaced")
	}

	var lut *mir2.Global
	for i := range m.Globals {
		if m.Globals[i].Name == "half_lut" {
			lut = &m.Globals[i]
			break
		}
	}
	if lut == nil {
		t.Fatal("global half_lut not found")
	}
	if len(lut.Init) != 11 {
		t.Fatalf("LUT size: want 11, got %d", len(lut.Init))
	}
	// LUT[0] = half(10) = 10>>1 = 5
	if lut.Init[0] != 5 {
		t.Errorf("LUT[0]: want 5 (half(10)), got %d", lut.Init[0])
	}
	// LUT[10] = half(20) = 20>>1 = 10
	if lut.Init[10] != 10 {
		t.Errorf("LUT[10]: want 10 (half(20)), got %d", lut.Init[10])
	}
}

func TestLUTGen_U16Return(t *testing.T) {
	m := &mir2.Module{Name: "lut_u16_ret"}
	f := m.AddFunc("widen")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.NewRanged(mir2.TyU8, 0, 4), mir2.ClassAcc)
	r := b.Ext(x, mir2.TyU8, mir2.TyU16, mir2.ClassPointer)
	b.Ret(r)

	changed := mir2.LUTGen(m)
	if !changed {
		t.Fatal("LUTGen: expected widen to be replaced")
	}

	var lutLo *mir2.Global
	var lutHi *mir2.Global
	for i := range m.Globals {
		if m.Globals[i].Name == "widen_lut_lo" {
			lutLo = &m.Globals[i]
		}
		if m.Globals[i].Name == "widen_lut_hi" {
			lutHi = &m.Globals[i]
		}
	}
	if lutLo == nil || lutHi == nil {
		t.Fatal("split u16 LUT globals not found")
	}
	if got := len(lutLo.Init); got != 4 {
		t.Fatalf("u16 low LUT size: want 4 bytes, got %d", got)
	}
	if got := len(lutHi.Init); got != 4 {
		t.Fatalf("u16 high LUT size: want 4 bytes, got %d", got)
	}
	wantLo := []byte{0, 1, 2, 3}
	wantHi := []byte{0, 0, 0, 0}
	for i, b := range wantLo {
		if lutLo.Init[i] != b {
			t.Fatalf("u16 low LUT byte %d: got %d want %d", i, lutLo.Init[i], b)
		}
	}
	for i, b := range wantHi {
		if lutHi.Init[i] != b {
			t.Fatalf("u16 high LUT byte %d: got %d want %d", i, lutHi.Init[i], b)
		}
	}
}

func TestLUTGen_U16ParamSmallRange(t *testing.T) {
	m := &mir2.Module{Name: "lut_u16_param"}
	f := m.AddFunc("low_byte")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.NewRanged(mir2.TyU16, 1000, 1004), mir2.ClassPointer)
	xff := b.Const(0x00FF, mir2.TyU16, mir2.ClassIndex)
	masked := b.And(x, xff, mir2.TyU16, mir2.ClassPointer)
	r := b.Trunc(masked, mir2.TyU16, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r)

	changed := mir2.LUTGen(m)
	if !changed {
		t.Fatal("LUTGen: expected low_byte to be replaced")
	}

	var lut *mir2.Global
	for i := range m.Globals {
		if m.Globals[i].Name == "low_byte_lut" {
			lut = &m.Globals[i]
			break
		}
	}
	if lut == nil {
		t.Fatal("global low_byte_lut not found")
	}
	want := []byte{232, 233, 234, 235}
	if len(lut.Init) != len(want) {
		t.Fatalf("u16-param LUT size: want %d, got %d", len(want), len(lut.Init))
	}
	for i, b := range want {
		if lut.Init[i] != b {
			t.Fatalf("u16-param LUT[%d]: got %d want %d", i, lut.Init[i], b)
		}
	}
}

// popcount8 is the reference popcount for test verification.
func popcount8(x int) int {
	n := 0
	for x != 0 {
		n += x & 1
		x >>= 1
	}
	return n
}
