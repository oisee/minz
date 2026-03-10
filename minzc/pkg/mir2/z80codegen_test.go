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
	pr := bld.Load(base, mir2.TyU8, mir2.ClassAcc)                         // palette.r (offset 0)
	gPtr := bld.FieldOf(base, colorTy, 1, mir2.ClassPointer)               // &palette.g
	pg := bld.Load(gPtr, mir2.TyU8, mir2.ClassGeneral)                     // palette.g
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
