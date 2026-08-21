package qbe2mir2_test

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2qbe"
	"github.com/minz/minzc/pkg/qbe2mir2"
)

// vmCall runs funcName in a module via the MIR2 VM and returns the integer result.
func vmCall(t *testing.T, m *mir2.Module, funcName string, args ...int64) int64 {
	t.Helper()
	vm := mir2.NewVM(m)
	vals := make([]mir2.Value, len(args))
	for i, a := range args {
		vals[i] = mir2.Value{I: a}
	}
	res, err := vm.Call(funcName, vals)
	if err != nil {
		t.Fatalf("VM %s(%v): %v", funcName, args, err)
	}
	if len(res) == 0 {
		return 0
	}
	return res[0].I
}

// ── Parse + VM tests ──────────────────────────────────────────────────────────

// TestParseAdd: simplest possible function — no blocks, no phi.
func TestParseAdd(t *testing.T) {
	src := `
export function w $add(w %a, w %b) {
@start
	%c =w add %a, %b
	ret %c
}
`
	m, err := qbe2mir2.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cases := [][3]int64{{1, 2, 3}, {10, 20, 30}, {100, 55, 155}, {0, 0, 0}}
	for _, tc := range cases {
		got := vmCall(t, m, "add", tc[0], tc[1])
		if got != tc[2] {
			t.Errorf("add(%d,%d) = %d, want %d", tc[0], tc[1], got, tc[2])
		}
	}
}

// TestParseGCD: two-block loop with phi nodes → block params.
func TestParseGCD(t *testing.T) {
	src := `
export function w $gcd(w %a, w %b) {
@entry
	jmp @loop
@loop
	%a1 =w phi @entry %a, @body %b1
	%b1 =w phi @entry %b, @body %r
	%cond =w ceqw %b1, 0
	jnz %cond, @done, @body
@body
	%r =w urem %a1, %b1
	jmp @loop
@done
	ret %a1
}
`
	m, err := qbe2mir2.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cases := [][3]int64{{12, 8, 4}, {100, 75, 25}, {7, 3, 1}, {0, 5, 5}, {15, 5, 5}}
	for _, tc := range cases {
		got := vmCall(t, m, "gcd", tc[0], tc[1])
		if got != tc[2] {
			t.Errorf("gcd(%d,%d) = %d, want %d", tc[0], tc[1], got, tc[2])
		}
	}
}

// TestParseAbsDiff: conditional branch without phi (simple if/else).
func TestParseAbsDiff(t *testing.T) {
	src := `
export function w $abs_diff(w %a, w %b) {
@entry
	%lt =w cultw %a, %b
	jnz %lt, @then, @else
@then
	%r1 =w sub %b, %a
	ret %r1
@else
	%r2 =w sub %a, %b
	ret %r2
}
`
	m, err := qbe2mir2.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cases := [][3]int64{{10, 3, 7}, {3, 10, 7}, {5, 5, 0}, {0, 255, 255}}
	for _, tc := range cases {
		got := vmCall(t, m, "abs_diff", tc[0], tc[1])
		if got != tc[2] {
			t.Errorf("abs_diff(%d,%d) = %d, want %d", tc[0], tc[1], got, tc[2])
		}
	}
}

// TestParseCopy: copy/const immediates.
func TestParseCopy(t *testing.T) {
	src := `
export function w $const42(w %x) {
@start
	%r =w copy 42
	ret %r
}
`
	m, err := qbe2mir2.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := vmCall(t, m, "const42", 0)
	if got != 42 {
		t.Errorf("const42 = %d, want 42", got)
	}
}

// TestParseData: data globals are parsed without error.
func TestParseData(t *testing.T) {
	src := `
data $lut = { b 0, b 1, b 2, b 3 }
export function w $identity(w %x) {
@start
	ret %x
}
`
	m, err := qbe2mir2.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Globals) != 1 {
		t.Fatalf("globals: got %d, want 1", len(m.Globals))
	}
	if m.Globals[0].Name != "lut" {
		t.Errorf("global name = %q, want %q", m.Globals[0].Name, "lut")
	}
	if len(m.Globals[0].Init) != 4 {
		t.Errorf("global init len = %d, want 4", len(m.Globals[0].Init))
	}
}

// TestParseZeroData: zero-initialised globals.
func TestParseZeroData(t *testing.T) {
	src := `
data $buf = { z 8 }
export function w $nop(w %x) {
@start
	ret %x
}
`
	m, err := qbe2mir2.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Globals) != 1 || len(m.Globals[0].Init) != 0 {
		t.Errorf("zero global: unexpected init bytes")
	}
}

// ── Round-trip tests (MIR2 → QBE → MIR2 → VM) ───────────────────────────────

// roundTrip compiles m to QBE IL, parses it back, and returns the new module.
func roundTrip(t *testing.T, m *mir2.Module) *mir2.Module {
	t.Helper()
	qbeIL, err := mir2qbe.Compile(m)
	if err != nil {
		t.Fatalf("mir2qbe.Compile: %v", err)
	}
	t.Logf("QBE IL:\n%s", qbeIL)
	m2, err := qbe2mir2.Parse(qbeIL)
	if err != nil {
		t.Fatalf("qbe2mir2.Parse: %v", err)
	}
	return m2
}

// buildAbsDiff builds a simple abs_diff(a, b: u8) → u8 MIR2 function.
func buildAbsDiff(m *mir2.Module) {
	f := m.AddFunc("abs_diff")
	bld := mir2.NewBuilder(f)

	entry := bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld.SwitchTo(entry)

	cmp := bld.Cmp(mir2.CmpUlt, a, bv, mir2.ClassFlag, false)
	bld.BrIf(cmp, "then", nil, "else_", nil)

	bld.SwitchToNewBlock("then")
	r1 := bld.Sub(bv, a, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(r1)

	bld.SwitchToNewBlock("else_")
	r2 := bld.Sub(a, bv, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(r2)
}

// buildGCD builds gcd(a, b: u16) → u16.
func buildGCD(m *mir2.Module) {
	f := m.AddFunc("gcd")
	bld := mir2.NewBuilder(f)

	entry := bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU16, mir2.ClassPointer)
	bv := bld.Param("b", mir2.TyU16, mir2.ClassIndex)
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	bld.SwitchTo(entry)

	loopHead := bld.SwitchToNewBlock("loop")
	ap := bld.BlockParam(loopHead, mir2.TyU16, mir2.ClassPointer)
	bp := bld.BlockParam(loopHead, mir2.TyU16, mir2.ClassIndex)
	bld.SwitchTo(entry)
	bld.Jmp("loop", a, bv)

	bld.SwitchTo(loopHead)
	cond := bld.Cmp(mir2.CmpEq, bp, bld.Const(0, mir2.TyU16, mir2.ClassGeneral), mir2.ClassFlag, false)
	bld.BrIf(cond, "done", nil, "body", nil)

	bld.SwitchToNewBlock("body")
	rem := bld.BinOp(mir2.OpMod, ap, bp, mir2.TyU16, mir2.ClassGeneral)
	bld.Jmp("loop", bp, rem)

	bld.SwitchToNewBlock("done")
	bld.Ret(ap)
}

func TestRoundTripAbsDiff(t *testing.T) {
	m := &mir2.Module{Name: "abs_diff"}
	buildAbsDiff(m)

	m2 := roundTrip(t, m)

	cases := [][3]int64{{10, 3, 7}, {3, 10, 7}, {5, 5, 0}}
	for _, tc := range cases {
		orig := vmCall(t, m, "abs_diff", tc[0], tc[1])
		rt := vmCall(t, m2, "abs_diff", tc[0], tc[1])
		if orig != tc[2] {
			t.Errorf("[orig] abs_diff(%d,%d) = %d, want %d", tc[0], tc[1], orig, tc[2])
		}
		if rt != tc[2] {
			t.Errorf("[roundtrip] abs_diff(%d,%d) = %d, want %d", tc[0], tc[1], rt, tc[2])
		}
	}
}

func TestRoundTripGCD(t *testing.T) {
	m := &mir2.Module{Name: "gcd"}
	buildGCD(m)

	m2 := roundTrip(t, m)

	cases := [][3]int64{{12, 8, 4}, {100, 75, 25}, {7, 3, 1}}
	for _, tc := range cases {
		orig := vmCall(t, m, "gcd", tc[0], tc[1])
		rt := vmCall(t, m2, "gcd", tc[0], tc[1])
		if orig != tc[2] {
			t.Errorf("[orig] gcd(%d,%d) = %d, want %d", tc[0], tc[1], orig, tc[2])
		}
		if rt != tc[2] {
			t.Errorf("[roundtrip] gcd(%d,%d) = %d, want %d", tc[0], tc[1], rt, tc[2])
		}
	}
}

// TestParseImmediateOperands: QBE puts constants directly in binary ops
// (`and %r, 255`), but MIR2 binary ops read registers only, so each immediate
// has to be materialised into an OpConst register. The immediate can sit in
// either position, so operand order must survive for non-commutative ops —
// `sub 10, %a` is 10-a, not a-10.
func TestParseImmediateOperands(t *testing.T) {
	src := `
export function w $imms(w %a) {
@start
	%m =w and %a, 255
	%r =w sub 100, %m
	%s =w shr %r, 1
	ret %s
}
`
	m, err := qbe2mir2.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// (100 - (a & 255)) >> 1, evaluated as u16 (QBE w maps to TyU16).
	// The last case underflows and wraps: 100-255 = 65381, >>1 = 32690.
	// It also pins operand order — a flipped `sub` would give (255-100)>>1 = 77.
	cases := [][2]int64{{0, 50}, {10, 45}, {50, 25}, {100, 0}, {511, 32690}}
	for _, tc := range cases {
		got := vmCall(t, m, "imms", tc[0])
		if got != tc[1] {
			t.Errorf("imms(%d) = %d, want %d", tc[0], got, tc[1])
		}
	}
}

// TestParseImmediateCompare: the same materialisation is needed for compares,
// where a reversed operand order silently flips the predicate.
func TestParseImmediateCompare(t *testing.T) {
	src := `
export function w $over(w %a) {
@start
	%c =w cugtw %a, 10
	jnz %c, @yes, @no
@yes
	%y =w copy 1
	ret %y
@no
	%n =w copy 0
	ret %n
}
`
	m, err := qbe2mir2.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cases := [][2]int64{{0, 0}, {10, 0}, {11, 1}, {255, 1}}
	for _, tc := range cases {
		got := vmCall(t, m, "over", tc[0])
		if got != tc[1] {
			t.Errorf("over(%d) = %d, want %d", tc[0], got, tc[1])
		}
	}
}
