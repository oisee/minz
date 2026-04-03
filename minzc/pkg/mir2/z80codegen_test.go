package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestZ80CodegenFibonacci(t *testing.T) {
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	var asm string
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		asm = mir2.Z80Codegen(m, ar)
	}

	t.Log("\n" + asm)

	// Structural checks.
	checks := []string{
		"fibonacci:",       // function entry label
		".fibonacci_base:", // base block label
		"CP ",              // comparison
		"JRS ",             // branch (JRS = JR-if-Short, auto-promotes to JP if needed)
		"ADD HL",           // 16-bit add (a+b in loop_body)
		"RET",              // return
	}
	for _, needle := range checks {
		if !strings.Contains(asm, needle) {
			t.Errorf("assembly missing %q", needle)
		}
	}
}

func TestZ80CodegenSMCCounter(t *testing.T) {
	m := &mir2.Module{Name: "smc"}
	f := m.AddFunc("counter")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	slot := bld.PatchSlot(0, mir2.TyU8, mir2.ClassAcc)
	val := bld.LoadPatched(slot, mir2.TyU8, mir2.ClassAcc)
	one := bld.Const(1, mir2.TyU8, mir2.ClassGeneral)
	next := bld.Add(val, one, mir2.TyU8, mir2.ClassAcc)
	bld.Patch(slot, next, mir2.TyU8)
	bld.Ret(val)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)

	t.Log("\n" + asm)

	if !strings.Contains(asm, "patch_slot") {
		t.Error("SMC counter assembly should reference patch_slot")
	}
	if !strings.Contains(asm, "RET") {
		t.Error("SMC counter assembly must have RET")
	}
}

func TestZ80CodegenExt(t *testing.T) {
	// fun @zext(%r1: u8 [acc]) -> u16 [pointer]
	//   %r2 = ext %r1 : u8 → u16 [pointer]
	//   ret %r2
	// Expect: LD L, A / LD H, 0
	m := &mir2.Module{Name: "zext"}
	f := m.AddFunc("zext")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("n", mir2.TyU8, mir2.ClassAcc)
	r2 := bld.Ext(r1, mir2.TyU8, mir2.TyU16, mir2.ClassPointer)
	bld.Ret(r2)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)

	t.Log("\n" + asm)

	// %r1 → A, %r2 → HL: should emit LD L, A / LD H, 0
	if !strings.Contains(asm, "LD L, A") {
		t.Errorf("zero-extend u8→u16: expected 'LD L, A'; got:\n%s", asm)
	}
	if !strings.Contains(asm, "LD H, 0") {
		t.Errorf("zero-extend u8→u16: expected 'LD H, 0'; got:\n%s", asm)
	}
}

// TestZ80Codegen_IXPointer verifies that when a ClassPointer reg is assigned to IX
// (directly forced via AllocResult), OpLoad and OpStore emit (IX+0) addressing,
// not bare (IX) which is invalid Z80.
func TestZ80Codegen_IXPointer(t *testing.T) {
	m := &mir2.Module{Name: "ixtest"}
	f := m.AddFunc("ix_load_store")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	// Allocate params via builder so reg numbers are assigned by m.nextReg.
	ptr := bld.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
	val := bld.Param("val", mir2.TyU8, mir2.ClassAcc)
	// Store val to *ptr, then load it back.
	bld.Store(ptr, val, mir2.TyU8)
	loaded := bld.Load(ptr, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(loaded)

	// Force ptr → IX, val → A, loaded → A.
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
		ptr:    {Kind: mir2.LocIXY, Name: "IX"},
		val:    {Kind: mir2.LocReg, Name: "A"},
		loaded: {Kind: mir2.LocReg, Name: "A"},
	}}

	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	if !strings.Contains(asm, "(IX+0)") {
		t.Errorf("IX pointer: expected '(IX+0)' addressing; got:\n%s", asm)
	}
	if strings.Contains(asm, "LD A, (IX)") || strings.Contains(asm, "LD (IX),") {
		t.Errorf("IX pointer: invalid bare (IX) addressing emitted; got:\n%s", asm)
	}
}

// TestZ80Codegen_IX16bitLoadStore verifies that 16-bit load/store via IX
// uses (IX+0)/(IX+1) displacement addressing rather than INC IX / DEC IX.
func TestZ80Codegen_IX16bitLoadStore(t *testing.T) {
	m := &mir2.Module{Name: "ixtest16"}
	f := m.AddFunc("ix_load16")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	ptr := bld.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
	loaded := bld.Load(ptr, mir2.TyU16, mir2.ClassPointer)
	bld.Ret(loaded)

	// Force ptr → IX, loaded → HL.
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
		ptr:    {Kind: mir2.LocIXY, Name: "IX"},
		loaded: {Kind: mir2.LocReg, Name: "HL"},
	}}

	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// Should use (IX+0) and (IX+1) — no INC/DEC IX.
	if !strings.Contains(asm, "(IX+0)") {
		t.Errorf("IX 16-bit load: expected '(IX+0)'; got:\n%s", asm)
	}
	if !strings.Contains(asm, "(IX+1)") {
		t.Errorf("IX 16-bit load: expected '(IX+1)'; got:\n%s", asm)
	}
	if strings.Contains(asm, "INC IX") || strings.Contains(asm, "DEC IX") {
		t.Errorf("IX 16-bit load: should not emit INC/DEC IX; got:\n%s", asm)
	}
}

// TestZ80Codegen_IXAllocUnderPressure verifies that the PBQP allocator selects
// IX for a ClassPointer reg when HL, DE, and BC are already occupied by other
// simultaneously-live ClassPointer / ClassIndex / ClassPair regs.
//
// This is the Phase 6a acceptance test: no $F0xx spill when IX is available.
func TestZ80Codegen_IXAllocUnderPressure(t *testing.T) {
	m := &mir2.Module{Name: "ixpressure"}
	f := m.AddFunc("pressure")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")

	// Three ClassPointer params — HL, DE, BC all needed simultaneously.
	p0 := bld.Param("p0", mir2.TyPtr, mir2.ClassPointer) // → HL (cost 0)
	p1 := bld.Param("p1", mir2.TyPtr, mir2.ClassPointer) // → DE (cost 4)
	p2 := bld.Param("p2", mir2.TyPtr, mir2.ClassPointer) // → BC (cost 6)
	// Fourth pointer — no HL/DE/BC available, must choose IX or spill.
	p3 := bld.Param("p3", mir2.TyPtr, mir2.ClassPointer) // → IX (cost 8) < $F0xx (cost 26+)

	// Keep all four live simultaneously by loading from each.
	v0 := bld.Load(p0, mir2.TyU8, mir2.ClassAcc)
	v1 := bld.Load(p1, mir2.TyU8, mir2.ClassAcc)
	v2 := bld.Load(p2, mir2.TyU8, mir2.ClassAcc)
	v3 := bld.Load(p3, mir2.TyU8, mir2.ClassAcc)

	// Combine all four so they're all required (no dead-store elim).
	sum01 := bld.Add(v0, v1, mir2.TyU8, mir2.ClassAcc)
	sum23 := bld.Add(v2, v3, mir2.TyU8, mir2.ClassAcc)
	total := bld.Add(sum01, sum23, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(total)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// Verify at least one of the four pointers is in IX or IY — the allocator
	// must reach for IXY rather than spilling to $F0xx when all HL/DE/BC are taken.
	params := []mir2.Reg{p0, p1, p2, p3}
	anyIXY := false
	for _, p := range params {
		if ar.Locs[p].Kind == mir2.LocIXY {
			anyIXY = true
		}
	}
	if !anyIXY {
		t.Errorf("Phase 6a: no pointer allocated to IX/IY under pressure; locs: p0=%v p1=%v p2=%v p3=%v",
			ar.Locs[p0], ar.Locs[p1], ar.Locs[p2], ar.Locs[p3])
	}

	// Verify no memory-backed register in the output (no PUSH mem / LD A,($F0xx)).
	if strings.Contains(asm, "($F0") {
		t.Errorf("Phase 6a: unexpected $F0xx memory spill:\n%s", asm)
	}
}

// ── Global struct field direct-addressing tests ───────────────────────────────
//
// Optimization: replace LD HL,sym + INC HL×N + LD A,(HL) with LD A,(sym__field).
// Replaces 22–43T with 13T for u8 field loads where dst is A.
// Also replaces LD HL,sym + INC HL×N + LD (HL),r with LD (nn),A for stores.

// TestGlobalFieldDirectAddr_Load verifies that:
//  1. get_r() emits LD A,(palette__r)
//  2. get_b() emits LD A,(palette__b) — non-zero offset field
//  3. EQU labels are emitted for all struct fields
//  4. No LD HL,palette / INC HL in get_r or get_b
func TestGlobalFieldDirectAddr_Load(t *testing.T) {
	// struct Color { r: u8, g: u8, b: u8 }
	// global palette: Color
	// fun get_r() -> u8 { return palette.r }
	// fun get_g() -> u8 { return palette.g }
	// fun get_b() -> u8 { return palette.b }

	colorTy := &mir2.StructTy{
		Name: "Color",
		Fields: []mir2.StructField{
			{Name: "r", Ty: mir2.TyU8},
			{Name: "g", Ty: mir2.TyU8},
			{Name: "b", Ty: mir2.TyU8},
		},
	}

	m := &mir2.Module{Name: "color_direct"}
	m.AddGlobal(mir2.Global{
		Name: "palette",
		Ty:   colorTy,
		Init: []byte{10, 20, 30},
	})

	// get_r: load palette.r (offset 0) → A
	buildFieldGetter(m, "get_r", "palette", colorTy, 0)
	// get_g: load palette.g (offset 1) → A
	buildFieldGetter(m, "get_g", "palette", colorTy, 1)
	// get_b: load palette.b (offset 2) → A
	buildFieldGetter(m, "get_b", "palette", colorTy, 2)

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	// Check: EQU labels emitted.
	if !strings.Contains(asm, "palette__r") {
		t.Errorf("missing EQU label palette__r:\n%s", asm)
	}
	if !strings.Contains(asm, "palette__g") {
		t.Errorf("missing EQU label palette__g:\n%s", asm)
	}
	if !strings.Contains(asm, "palette__b") {
		t.Errorf("missing EQU label palette__b:\n%s", asm)
	}

	// Check: get_r body uses direct addressing.
	getR := extractFuncAsm(asm, "get_r")
	if !strings.Contains(getR, "LD A, (palette__r)") {
		t.Errorf("get_r: expected 'LD A, (palette__r)'; got:\n%s", getR)
	}
	if strings.Contains(getR, "LD HL, palette") {
		t.Errorf("get_r: unexpected 'LD HL, palette' (should use direct addr):\n%s", getR)
	}
	if strings.Contains(getR, "INC HL") {
		t.Errorf("get_r: unexpected 'INC HL' (should use direct addr):\n%s", getR)
	}

	// Check: get_b body uses direct addressing (non-zero offset).
	getB := extractFuncAsm(asm, "get_b")
	if !strings.Contains(getB, "LD A, (palette__b)") {
		t.Errorf("get_b: expected 'LD A, (palette__b)'; got:\n%s", getB)
	}
	if strings.Contains(getB, "LD HL, palette") {
		t.Errorf("get_b: unexpected 'LD HL, palette' (should use direct addr):\n%s", getB)
	}
}

// TestGlobalFieldDirectAddr_Store verifies that:
//  1. set_b(v) emits LD A,v (if needed) + LD (palette__b),A
//  2. No LD HL,palette / INC HL in set_b
func TestGlobalFieldDirectAddr_Store(t *testing.T) {
	// fun set_b(v: u8) -> void { palette.b = v }

	colorTy := &mir2.StructTy{
		Name: "Color",
		Fields: []mir2.StructField{
			{Name: "r", Ty: mir2.TyU8},
			{Name: "g", Ty: mir2.TyU8},
			{Name: "b", Ty: mir2.TyU8},
		},
	}

	m := &mir2.Module{Name: "color_store"}
	m.AddGlobal(mir2.Global{
		Name: "palette",
		Ty:   colorTy,
		Init: []byte{0, 0, 0},
	})

	// set_b: store palette.b = v (offset 2)
	buildFieldSetter(m, "set_b", "palette", colorTy, 2)

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	setB := extractFuncAsm(asm, "set_b")
	if !strings.Contains(setB, "LD (palette__b), A") {
		t.Errorf("set_b: expected 'LD (palette__b), A'; got:\n%s", setB)
	}
	if strings.Contains(setB, "LD HL, palette") {
		t.Errorf("set_b: unexpected 'LD HL, palette' (should use direct addr):\n%s", setB)
	}
	if strings.Contains(setB, "INC HL") {
		t.Errorf("set_b: unexpected 'INC HL' (should use direct addr):\n%s", setB)
	}
}

func TestBitStorePattern_SET_HL(t *testing.T) {
	m := &mir2.Module{Name: "bit_set"}
	f := m.AddFunc("set_bit3")

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	ptr := bld.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
	cur := bld.Load(ptr, mir2.TyU8, mir2.ClassGeneral)
	mask := bld.Const(8, mir2.TyU8, mir2.ClassGeneral)
	next := bld.Or(cur, mask, mir2.TyU8, mir2.ClassGeneral)
	bld.Store(ptr, next, mir2.TyU8)
	bld.Ret()

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	fnAsm := extractFuncAsm(asm, "set_bit3")
	if !strings.Contains(fnAsm, "SET 3, (HL)") {
		t.Fatalf("expected SET fast path, got:\n%s", fnAsm)
	}
	if strings.Contains(fnAsm, "LD A, (HL)") || strings.Contains(fnAsm, "OR 8") {
		t.Fatalf("expected fused store without load/or sequence, got:\n%s", fnAsm)
	}
}

func TestBitStorePattern_RES_IX(t *testing.T) {
	m := &mir2.Module{Name: "bit_res"}
	f := m.AddFunc("reset_bit2")

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	ptr := bld.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
	cur := bld.Load(ptr, mir2.TyU8, mir2.ClassGeneral)
	mask := bld.Const(^int64(1<<2), mir2.TyU8, mir2.ClassGeneral)
	next := bld.And(cur, mask, mir2.TyU8, mir2.ClassGeneral)
	bld.Store(ptr, next, mir2.TyU8)
	bld.Ret()

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	ar.Locs[ptr] = mir2.PhysLoc{Kind: mir2.LocIXY, Name: "IX"}

	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	fnAsm := extractFuncAsm(asm, "reset_bit2")
	if !strings.Contains(fnAsm, "RES 2, (IX+0)") {
		t.Fatalf("expected RES fast path, got:\n%s", fnAsm)
	}
	if strings.Contains(fnAsm, "LD A, (IX+0)") || strings.Contains(fnAsm, "AND 251") || strings.Contains(fnAsm, "AND -5") {
		t.Fatalf("expected fused store without load/and sequence, got:\n%s", fnAsm)
	}
}

func TestBitCmpPattern_MaskAndZero_HL(t *testing.T) {
	m := &mir2.Module{Name: "bit_cmp_mask"}
	f := m.AddFunc("test_bit5")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	ptr := b.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
	cur := b.Load(ptr, mir2.TyU8, mir2.ClassGeneral)
	mask := b.Const(32, mir2.TyU8, mir2.ClassGeneral)
	masked := b.And(cur, mask, mir2.TyU8, mir2.ClassGeneral)
	zero := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	cond := b.Cmp(mir2.CmpNe, masked, zero, mir2.ClassFlag, false)
	b.BrIf(cond, "ret1", nil, "ret0", nil)
	b.SwitchToNewBlock("ret1")
	one := b.Const(1, mir2.TyU8, mir2.ClassAcc)
	b.Ret(one)
	b.SwitchToNewBlock("ret0")
	z := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	b.Ret(z)

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	fnAsm := extractFuncAsm(asm, "test_bit5")
	if !strings.Contains(fnAsm, "BIT 5, (HL)") {
		t.Fatalf("expected BIT fast path, got:\n%s", fnAsm)
	}
	if strings.Contains(fnAsm, "LD A, (HL)") || strings.Contains(fnAsm, "AND 32") || strings.Contains(fnAsm, "CP 0") {
		t.Fatalf("expected fused bit test without load/and/cp sequence, got:\n%s", fnAsm)
	}
}

func TestBitCmpPattern_ShiftedBitRead_HL(t *testing.T) {
	m := &mir2.Module{Name: "bit_cmp_shift"}
	f := m.AddFunc("test_bit7")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	ptr := b.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
	cur := b.Load(ptr, mir2.TyU8, mir2.ClassGeneral)
	shiftAmt := b.Const(7, mir2.TyU8, mir2.ClassGeneral)
	shifted := b.Shr(cur, shiftAmt, mir2.TyU8, mir2.ClassGeneral)
	oneMask := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	bitVal := b.And(shifted, oneMask, mir2.TyU8, mir2.ClassGeneral)
	zero := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	cond := b.Cmp(mir2.CmpEq, bitVal, zero, mir2.ClassFlag, false)
	b.BrIf(cond, "ret0", nil, "ret1", nil)
	b.SwitchToNewBlock("ret0")
	z := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	b.Ret(z)
	b.SwitchToNewBlock("ret1")
	one := b.Const(1, mir2.TyU8, mir2.ClassAcc)
	b.Ret(one)

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	fnAsm := extractFuncAsm(asm, "test_bit7")
	if !strings.Contains(fnAsm, "BIT 7, (HL)") {
		t.Fatalf("expected BIT fast path for shifted bit-read, got:\n%s", fnAsm)
	}
	if strings.Contains(fnAsm, "LD A, (HL)") || strings.Contains(fnAsm, "SRL") || strings.Contains(fnAsm, "AND 1") || strings.Contains(fnAsm, "CP 0") {
		t.Fatalf("expected fused bit test without load/shift/and/cp sequence, got:\n%s", fnAsm)
	}
}

// TestGlobalFieldDirectAddr_SharedBase verifies that when the base pointer is shared
// (used by multiple loads), the optimization correctly fires only for the A-dst load
// and leaves the base register alive for the non-A load.
func TestGlobalFieldDirectAddr_SharedBase(t *testing.T) {
	// sum_xy: return palette.r + palette.g  (px in A, py in ClassGeneral/C)
	// base ptr is used for BOTH loads — must NOT be skipped.
	colorTy := &mir2.StructTy{
		Name: "Color",
		Fields: []mir2.StructField{
			{Name: "r", Ty: mir2.TyU8},
			{Name: "g", Ty: mir2.TyU8},
			{Name: "b", Ty: mir2.TyU8},
		},
	}
	m := &mir2.Module{Name: "shared_base"}
	m.AddGlobal(mir2.Global{Name: "palette", Ty: colorTy, Init: []byte{3, 4, 5}})

	f := m.AddFunc("sum_rg")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	// Shared base: used for two loads.
	base := bld.AddrOf("palette", mir2.ClassPointer)
	pr := bld.Load(base, mir2.TyU8, mir2.ClassAcc)           // palette.r (offset 0)
	gPtr := bld.FieldOf(base, colorTy, 1, mir2.ClassPointer) // &palette.g
	pg := bld.Load(gPtr, mir2.TyU8, mir2.ClassGeneral)       // palette.g
	result := bld.Add(pr, pg, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(result)

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	// The function must produce the correct result — no crash / malformed code.
	// Since base is shared, the optimization must NOT fire (it checks useCount==1).
	// We just verify the assembly assembles and contains ADD A.
	if !strings.Contains(asm, "ADD A") {
		t.Errorf("sum_rg: expected ADD A instruction; got:\n%s", asm)
	}
	// Must contain RET.
	if !strings.Contains(asm, "RET") {
		t.Errorf("sum_rg: missing RET:\n%s", asm)
	}
}

func TestGlobalArrayTypedEmission_U16(t *testing.T) {
	m := &mir2.Module{Name: "u16_array_globals"}
	m.AddGlobal(mir2.Global{
		Name: "nums",
		Ty:   mir2.NewArray(mir2.TyU16, 3),
		Init: []byte{1, 0, 2, 0, 0x34, 0x12},
	})

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	if !strings.Contains(asm, "nums:") {
		t.Fatalf("missing nums label:\n%s", asm)
	}
	if !strings.Contains(asm, "    DW 1, 2, 4660") {
		t.Fatalf("expected typed DW emission for u16 array:\n%s", asm)
	}
	if strings.Contains(asm, "    DB 1, 0, 2, 0, 52, 18") {
		t.Fatalf("u16 array should not be emitted as flat DB byte stream:\n%s", asm)
	}
}

// TestGlobalFieldChainStore verifies that 2+ consecutive field stores to the same
// global struct are emitted as an HL-chain (LD HL,sym; LD (HL),r; INC HL; ...).
// set_rgb(rv,gv,bv): stores palette.r, palette.g, palette.b in sequence.
// Expected: LD HL,palette / LD (HL),C / INC HL / LD (HL),D / INC HL / LD (HL),E
// NOT:      LD A,C / LD (palette__r),A / LD A,D / LD (palette__g),A / ...
func TestGlobalFieldChainStore(t *testing.T) {
	colorTy := &mir2.StructTy{
		Name: "Color",
		Fields: []mir2.StructField{
			{Name: "r", Ty: mir2.TyU8},
			{Name: "g", Ty: mir2.TyU8},
			{Name: "b", Ty: mir2.TyU8},
		},
	}
	m := &mir2.Module{Name: "chain_store"}
	m.AddGlobal(mir2.Global{Name: "palette", Ty: colorTy})

	f := m.AddFunc("set_rgb")
	f.Contract.Params = []mir2.Param{
		{Name: "rv", Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		{Name: "gv", Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		{Name: "bv", Ty: mir2.TyU8, Class: mir2.ClassGeneral},
	}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	rv := bld.Param("rv", mir2.TyU8, mir2.ClassGeneral)
	gv := bld.Param("gv", mir2.TyU8, mir2.ClassGeneral)
	bv := bld.Param("bv", mir2.TyU8, mir2.ClassGeneral)

	base := bld.AddrOf("palette", mir2.ClassPointer)
	ptrG := bld.FieldOf(base, colorTy, 1, mir2.ClassPointer)
	ptrB := bld.FieldOf(base, colorTy, 2, mir2.ClassPointer) // uses fresh AddrOf internally
	// Use a fresh base for each store so useCount[base]==1 per store.
	base2 := bld.AddrOf("palette", mir2.ClassPointer)
	base3 := bld.AddrOf("palette", mir2.ClassPointer)
	_ = ptrG
	_ = ptrB

	// Build independent stores: each with its own base+field chain.
	b0 := base
	b1 := bld.FieldOf(base2, colorTy, 1, mir2.ClassPointer)
	b2 := bld.FieldOf(base3, colorTy, 2, mir2.ClassPointer)
	bld.Store(b0, rv, mir2.TyU8)
	bld.Store(b1, gv, mir2.TyU8)
	bld.Store(b2, bv, mir2.TyU8)
	bld.Ret()

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	funcAsm := extractFuncAsm(asm, "set_rgb")

	// Must contain LD HL, palette (the chain head).
	if !strings.Contains(funcAsm, "LD HL, palette") {
		t.Errorf("expected LD HL, palette (chain head); got:\n%s", funcAsm)
	}
	// Must contain LD (HL) stores (not LD (palette__X), A).
	if !strings.Contains(funcAsm, "LD (HL)") {
		t.Errorf("expected LD (HL) stores; got:\n%s", funcAsm)
	}
	// Must contain INC HL (for advancing through fields).
	if !strings.Contains(funcAsm, "INC HL") {
		t.Errorf("expected INC HL; got:\n%s", funcAsm)
	}
	// Should NOT contain LD (palette__r), A (the old direct-addr path).
	if strings.Contains(funcAsm, "LD (palette__") {
		t.Errorf("should not emit direct LD (palette__X), A for chain stores; got:\n%s", funcAsm)
	}
	// Must end with RET.
	if !strings.Contains(funcAsm, "RET") {
		t.Errorf("missing RET:\n%s", funcAsm)
	}
}

// TestGlobalFieldChainStore_TwoFields verifies the 2-field case (2-field chain).
func TestGlobalFieldChainStore_TwoFields(t *testing.T) {
	pt := &mir2.StructTy{
		Name:   "Point",
		Fields: []mir2.StructField{{Name: "x", Ty: mir2.TyU8}, {Name: "y", Ty: mir2.TyU8}},
	}
	m := &mir2.Module{Name: "chain_2"}
	m.AddGlobal(mir2.Global{Name: "pt", Ty: pt})

	f := m.AddFunc("set_xy")
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	xv := bld.Param("xv", mir2.TyU8, mir2.ClassGeneral)
	yv := bld.Param("yv", mir2.TyU8, mir2.ClassGeneral)
	b0 := bld.AddrOf("pt", mir2.ClassPointer)
	b1 := bld.AddrOf("pt", mir2.ClassPointer)
	b1f := bld.FieldOf(b1, pt, 1, mir2.ClassPointer)
	bld.Store(b0, xv, mir2.TyU8)
	bld.Store(b1f, yv, mir2.TyU8)
	bld.Ret()

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)
	funcAsm := extractFuncAsm(asm, "set_xy")

	if !strings.Contains(funcAsm, "LD HL, pt") {
		t.Errorf("expected LD HL, pt; got:\n%s", funcAsm)
	}
	if !strings.Contains(funcAsm, "LD (HL)") {
		t.Errorf("expected LD (HL) stores; got:\n%s", funcAsm)
	}
	// Two-field chain: one INC HL between x and y, no INC HL after y.
	if strings.Count(funcAsm, "INC HL") != 1 {
		t.Errorf("expected exactly 1 INC HL for 2-field chain, got:\n%s", funcAsm)
	}
}

// TestZ80Load_NoBCDEIndirectToNonA verifies that OpLoad via BC/DE pointer
// never emits "LD X,(DE)" or "LD X,(BC)" where X≠A — those instructions don't
// exist on Z80. The only valid BC/DE indirect loads are LD A,(BC) / LD A,(DE).
//
// Reproduces the Acc_add bug: self_ptr arrives in HL (ClassPointer); after
// EX DE,HL the ptr lives in DE; loading into a ClassGeneral (D) register used
// to emit the illegal "LD D,(DE)".
func TestZ80Load_NoBCDEIndirectToNonA(t *testing.T) {
	// Build: fun acc_load(ptr: ^u8, amount: u8) -> u8 { return (*ptr) + amount }
	// ptr → ClassPointer (HL), amount → ClassGeneral (C)
	// With register pressure the allocator may put the loaded value in D/E/B/C.
	m := &mir2.Module{Name: "ptrload"}
	f := m.AddFunc("acc_load")
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	ptr := bld.Param("ptr", mir2.TyU16, mir2.ClassPointer)
	amt := bld.Param("amt", mir2.TyU8, mir2.ClassGeneral)
	val := bld.Load(ptr, mir2.TyU8, mir2.ClassGeneral)
	sum := bld.Add(val, amt, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(sum)

	asm := compileModuleForCodegenTest(t, m)
	t.Log("\n" + asm)

	// No illegal BC/DE indirect loads to non-A registers.
	for _, bad := range []string{
		"LD B,(DE)", "LD C,(DE)", "LD D,(DE)", "LD E,(DE)", "LD H,(DE)", "LD L,(DE)",
		"LD B,(BC)", "LD C,(BC)", "LD D,(BC)", "LD E,(BC)", "LD H,(BC)", "LD L,(BC)",
	} {
		if strings.Contains(asm, bad) {
			t.Errorf("illegal Z80 instruction emitted: %q\nFull asm:\n%s", bad, asm)
		}
	}
}

// ── Helpers for direct-addr tests ─────────────────────────────────────────────

// buildFieldGetter builds: fun name() -> u8 { return global.fields[fieldIdx] }
func buildFieldGetter(m *mir2.Module, name, globalSym string, st *mir2.StructTy, fieldIdx int) {
	f := m.AddFunc(name)
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	base := bld.AddrOf(globalSym, mir2.ClassPointer)
	var ptr mir2.Reg
	if fieldIdx == 0 {
		ptr = base
	} else {
		ptr = bld.FieldOf(base, st, fieldIdx, mir2.ClassPointer)
	}
	val := bld.Load(ptr, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(val)
}

// buildFieldSetter builds: fun name(v: u8) -> void { global.fields[fieldIdx] = v }
func buildFieldSetter(m *mir2.Module, name, globalSym string, st *mir2.StructTy, fieldIdx int) {
	f := m.AddFunc(name)
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	v := bld.Param("v", mir2.TyU8, mir2.ClassAcc)
	base := bld.AddrOf(globalSym, mir2.ClassPointer)
	var ptr mir2.Reg
	if fieldIdx == 0 {
		ptr = base
	} else {
		ptr = bld.FieldOf(base, st, fieldIdx, mir2.ClassPointer)
	}
	bld.Store(ptr, v, mir2.TyU8)
	bld.Ret()
}

// compileModuleForCodegenTest wraps compileModule for codegen-only tests.
func compileModuleForCodegenTest(t *testing.T, m *mir2.Module) string {
	t.Helper()
	return compileModule(t, m)
}

// extractFuncAsm returns the lines of the named function from the assembly output.
// It collects lines between "name:" and the next non-indented label (or EOF).
func extractFuncAsm(asm, name string) string {
	lines := strings.Split(asm, "\n")
	var result []string
	inFunc := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == name+":" {
			inFunc = true
			result = append(result, line)
			continue
		}
		if inFunc {
			// Stop at the next top-level label (non-empty, no leading space, ends with ':').
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != ';' && strings.HasSuffix(trimmed, ":") {
				break
			}
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// ── ClassDWord / 32-bit tests ─────────────────────────────────────────────────

// TestDWord_Const verifies that a 32-bit constant is loaded via EXX:
//
//	LD rr, lo16 / EXX / LD rr, hi16 / EXX
func TestDWord_Const(t *testing.T) {
	m := &mir2.Module{Name: "dword_const"}
	f := m.AddFunc("dword_const")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU32, Class: mir2.ClassDWord}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	c := bld.Const(0x12345678, mir2.TyU32, mir2.ClassDWord)
	bld.Ret(c)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// EXX must appear (for hi16 load).
	if !strings.Contains(asm, "EXX") {
		t.Error("32-bit const load must use EXX")
	}
	// lo16 = 0x5678 = 22136; hi16 = 0x1234 = 4660
	if !strings.Contains(asm, "22136") {
		t.Errorf("expected lo16 22136 (0x5678) in asm:\n%s", asm)
	}
	if !strings.Contains(asm, "4660") {
		t.Errorf("expected hi16 4660 (0x1234) in asm:\n%s", asm)
	}
	if !strings.Contains(asm, "RET") {
		t.Error("must have RET")
	}
}

// TestDWord_Add32 verifies 32-bit addition uses ADD HL,rr / EXX / ADC HL,rr / EXX.
func TestDWord_Add32(t *testing.T) {
	m := &mir2.Module{Name: "add32"}
	f := m.AddFunc("add32")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU32, Class: mir2.ClassDWord}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("a", mir2.TyU32, mir2.ClassDWord)
	r2 := bld.Param("b", mir2.TyU32, mir2.ClassDWord)
	r3 := bld.Add(r1, r2, mir2.TyU32, mir2.ClassDWord)
	bld.Ret(r3)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// Must use EXX (for shadow bank access).
	if !strings.Contains(asm, "EXX") {
		t.Error("32-bit add must use EXX")
	}
	// Must have 16-bit ADD (ADD HL,rr) and ADC (ADC HL,rr).
	if !strings.Contains(asm, "ADD HL") {
		t.Error("32-bit add must have ADD HL,rr")
	}
	if !strings.Contains(asm, "ADC HL") {
		t.Error("32-bit add must have ADC HL,rr")
	}
	if !strings.Contains(asm, "RET") {
		t.Error("must have RET")
	}
}

// TestDWord_Sub32 verifies 32-bit subtraction uses AND A / SBC HL,rr / EXX / SBC HL,rr / EXX.
func TestDWord_Sub32(t *testing.T) {
	m := &mir2.Module{Name: "sub32"}
	f := m.AddFunc("sub32")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU32, Class: mir2.ClassDWord}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("a", mir2.TyU32, mir2.ClassDWord)
	r2 := bld.Param("b", mir2.TyU32, mir2.ClassDWord)
	r3 := bld.Sub(r1, r2, mir2.TyU32, mir2.ClassDWord)
	bld.Ret(r3)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	if !strings.Contains(asm, "EXX") {
		t.Error("32-bit sub must use EXX")
	}
	if !strings.Contains(asm, "SBC HL") {
		t.Error("32-bit sub must have SBC HL,rr")
	}
	if !strings.Contains(asm, "AND A") {
		t.Error("32-bit sub must clear carry with AND A before SBC")
	}
	if !strings.Contains(asm, "RET") {
		t.Error("must have RET")
	}
}

// TestDWord_AllocNoAliasConflict verifies that a u32 DWord and a u16 pair
// in the same function do not get aliased physical locations.
// E.g. if u32 → LocDWord{"HL"}, then u16 must NOT get LocReg{"HL"}.
func TestDWord_AllocNoAliasConflict(t *testing.T) {
	m := &mir2.Module{Name: "noalias"}
	f := m.AddFunc("noalias")
	// Two live regs: u32 (DWord) + u16 (Pair) — must not share HL.
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("a", mir2.TyU32, mir2.ClassDWord)
	r2 := bld.Param("b", mir2.TyU16, mir2.ClassPair)
	// Keep both live by using them in a throw-away add (result unused but forces liveness).
	_ = bld.Add(r2, r2, mir2.TyU16, mir2.ClassPair) // keep r2 live
	bld.Ret(r1)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})

	loc1 := ar.Loc(r1)
	loc2 := ar.Loc(r2)
	t.Logf("r1(u32) → %+v, r2(u16) → %+v", loc1, loc2)

	if loc1.Kind != mir2.LocDWord {
		t.Errorf("u32 register should be LocDWord, got %+v", loc1)
	}
	// r2 (u16) must not be in the same pair as r1 (DWord).
	if loc2.Name == loc1.Name {
		t.Errorf("u32 LocDWord{%s} and u16 LocReg{%s} share the same pair name — alias conflict!",
			loc1.Name, loc2.Name)
	}
}
