package mir2qbe_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2qbe"
)

// runNative compiles a MIR2 module to QBE IL, assembles it with qbe,
// links with a tiny C wrapper, and runs the named function with the given
// integer arguments. The return value is the integer printed to stdout.
//
// Pipeline: MIR2 → QBE IL → qbe → .s → cc → binary → run
func runNative(t *testing.T, m *mir2.Module, funcName string, args ...int) (int, error) {
	t.Helper()

	// 1. Compile MIR2 → QBE IL
	qbeIR, err := mir2qbe.Compile(m)
	if err != nil {
		return 0, fmt.Errorf("mir2qbe.Compile: %w", err)
	}
	t.Logf("QBE IR:\n%s", qbeIR)

	// 2. Write QBE IL to temp file
	dir := t.TempDir()
	ssaPath := filepath.Join(dir, "out.ssa")
	if err := os.WriteFile(ssaPath, []byte(qbeIR), 0644); err != nil {
		return 0, err
	}

	// 3. Run qbe → assembly
	asmPath := filepath.Join(dir, "out.s")
	qbeCmd := exec.Command("qbe", "-o", asmPath, ssaPath)
	if out, err := qbeCmd.CombinedOutput(); err != nil {
		return 0, fmt.Errorf("qbe: %w\n%s", err, out)
	}

	// 4. Write C main wrapper
	argList := make([]string, len(args))
	for i, a := range args {
		argList[i] = strconv.Itoa(a)
	}
	params := make([]string, len(args))
	for i := range args {
		params[i] = "int"
	}
	cSrc := fmt.Sprintf(`#include <stdio.h>
extern int %s(%s);
int main(void) { printf("%%d\n", %s(%s)); return 0; }
`,
		funcName,
		strings.Join(params, ", "),
		funcName,
		strings.Join(argList, ", "),
	)
	cPath := filepath.Join(dir, "main.c")
	if err := os.WriteFile(cPath, []byte(cSrc), 0644); err != nil {
		return 0, err
	}

	// 5. Compile: cc out.s main.c -o binary
	binPath := filepath.Join(dir, "bin")
	ccCmd := exec.Command("cc", asmPath, cPath, "-o", binPath)
	if out, err := ccCmd.CombinedOutput(); err != nil {
		return 0, fmt.Errorf("cc: %w\n%s", err, out)
	}

	// 6. Run
	runOut, err := exec.Command(binPath).Output()
	if err != nil {
		return 0, fmt.Errorf("run: %w", err)
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(runOut)))
	if err != nil {
		return 0, fmt.Errorf("parse output %q: %w", runOut, err)
	}
	return val, nil
}

// ── abs_diff ──────────────────────────────────────────────────────────────────
//
// fun abs_diff(a: u8, b: u8) -> u8 {
//     if a > b { a - b } else { b - a }
// }

func buildAbsDiff(m *mir2.Module) {
	f := m.AddFunc("abs_diff")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)

	cgt := b.Cmp(mir2.CmpGt, a, bv, mir2.ClassAcc, false)
	b.BrIf(cgt, "a_bigger", nil, "b_bigger_or_eq", nil)

	b.SwitchToNewBlock("a_bigger")
	r1 := b.Sub(a, bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r1)

	b.SwitchToNewBlock("b_bigger_or_eq")
	r2 := b.Sub(bv, a, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r2)
}

func TestQBEAbsDiff(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m := &mir2.Module{Name: "abs_diff"}
	buildAbsDiff(m)

	cases := []struct{ a, b, want int }{
		{10, 3, 7},
		{3, 10, 7},
		{5, 5, 0},
		{0, 255, 255},
		{100, 200, 100},
	}
	for _, tc := range cases {
		got, err := runNative(t, m, "abs_diff", tc.a, tc.b)
		if err != nil {
			t.Fatalf("abs_diff(%d,%d): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("abs_diff(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("abs_diff(%d,%d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

// ── clamp ─────────────────────────────────────────────────────────────────────
//
// fun clamp(x: u8, lo: u8, hi: u8) -> u8 {
//     if x < lo { lo } else if x > hi { hi } else { x }
// }

func buildClamp(m *mir2.Module) {
	f := m.AddFunc("clamp")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	lo := b.Param("lo", mir2.TyU8, mir2.ClassCounter)
	hi := b.Param("hi", mir2.TyU8, mir2.ClassGeneral)

	clt := b.Cmp(mir2.CmpLt, x, lo, mir2.ClassAcc, false)
	b.BrIf(clt, "ret_lo", nil, "check_hi", nil)

	b.SwitchToNewBlock("ret_lo")
	b.Ret(lo)

	b.SwitchToNewBlock("check_hi")
	cgt := b.Cmp(mir2.CmpGt, x, hi, mir2.ClassAcc, false)
	b.BrIf(cgt, "ret_hi", nil, "ret_x", nil)

	b.SwitchToNewBlock("ret_hi")
	b.Ret(hi)

	b.SwitchToNewBlock("ret_x")
	b.Ret(x)
}

func TestQBEClamp(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m := &mir2.Module{Name: "clamp"}
	buildClamp(m)

	cases := []struct{ x, lo, hi, want int }{
		{5, 0, 10, 5},
		{255, 0, 10, 10}, // 255 > hi=10 → clamped to hi
		{15, 0, 10, 10},
		{0, 0, 0, 0},
		{128, 100, 200, 128},
	}
	for _, tc := range cases {
		got, err := runNative(t, m, "clamp", tc.x, tc.lo, tc.hi)
		if err != nil {
			t.Fatalf("clamp(%d,%d,%d): %v", tc.x, tc.lo, tc.hi, err)
		}
		if got != tc.want {
			t.Errorf("clamp(%d,%d,%d) = %d, want %d", tc.x, tc.lo, tc.hi, got, tc.want)
		} else {
			t.Logf("clamp(%d,%d,%d) = %d ✓", tc.x, tc.lo, tc.hi, got)
		}
	}
}

// ── gcd ───────────────────────────────────────────────────────────────────────
//
// fun gcd(a: u8, b: u8) -> u8 — Euclidean algorithm
// Uses TermBrIf2 (three-way branch): eq → done, lt → b-=a, gt → a-=b

func buildGCD(m *mir2.Module) {
	f := m.AddFunc("gcd")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	b.Jmp("loop", a, bv)

	loop := b.SwitchToNewBlock("loop")
	aP := b.BlockParam(loop, mir2.TyU8, mir2.ClassAcc)
	bP := b.BlockParam(loop, mir2.TyU8, mir2.ClassCounter)
	b.BrIf2(aP, bP,
		"done", []mir2.Reg{aP},
		"a_smaller", []mir2.Reg{aP, bP},
		"a_bigger", []mir2.Reg{aP, bP},
	)

	asmaller := b.SwitchToNewBlock("a_smaller")
	aS := b.BlockParam(asmaller, mir2.TyU8, mir2.ClassAcc)
	bS := b.BlockParam(asmaller, mir2.TyU8, mir2.ClassCounter)
	aSafe := b.Move(aS, mir2.TyU8, mir2.ClassGeneral)
	bNew := b.Sub(bS, aSafe, mir2.TyU8, mir2.ClassCounter)
	b.Jmp("loop", aSafe, bNew)

	abigger := b.SwitchToNewBlock("a_bigger")
	aB := b.BlockParam(abigger, mir2.TyU8, mir2.ClassAcc)
	bB := b.BlockParam(abigger, mir2.TyU8, mir2.ClassCounter)
	aNew := b.Sub(aB, bB, mir2.TyU8, mir2.ClassAcc)
	b.Jmp("loop", aNew, bB)

	done := b.SwitchToNewBlock("done")
	result := b.BlockParam(done, mir2.TyU8, mir2.ClassAcc)
	b.Ret(result)
}

func TestQBEGCD(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m := &mir2.Module{Name: "gcd"}
	buildGCD(m)

	cases := []struct{ a, b, want int }{
		{12, 8, 4},
		{7, 13, 1},
		{48, 18, 6},
		{100, 75, 25},
		{1, 1, 1},
		{15, 5, 5},
		{5, 15, 5},
	}
	for _, tc := range cases {
		got, err := runNative(t, m, "gcd", tc.a, tc.b)
		if err != nil {
			t.Fatalf("gcd(%d,%d): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("gcd(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("gcd(%d,%d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

// ── u8 wrapping arithmetic ───────────────────────────────────────────────────
//
// u8_wrap_add(a, b) = a + b  (must wrap at 255)
// u8_wrap_shift(a)  = (a >> 1) + (a << 7)  (sfn_checksum rotation pattern)
//
// On Z80 these wrap naturally. QBE uses 32-bit "w", so without explicit
// truncation, results exceed 255.

func buildU8WrapAdd(m *mir2.Module) {
	f := m.AddFunc("u8_wrap_add")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	sum := b.Add(a, bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(sum)
}

func buildU8WrapShift(m *mir2.Module) {
	f := m.AddFunc("u8_wrap_shift")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	c1 := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	c7 := b.Const(7, mir2.TyU8, mir2.ClassGeneral)
	lo := b.Shr(a, c1, mir2.TyU8, mir2.ClassGeneral)
	hi := b.Shl(a, c7, mir2.TyU8, mir2.ClassGeneral)
	sum := b.Add(lo, hi, mir2.TyU8, mir2.ClassAcc)
	b.Ret(sum)
}

func TestQBEU8WrapAdd(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m := &mir2.Module{Name: "u8_wrap_add"}
	buildU8WrapAdd(m)

	cases := []struct{ a, b, want int }{
		{200, 100, 44},  // 300 & 0xFF = 44
		{255, 1, 0},     // 256 & 0xFF = 0
		{128, 128, 0},   // 256 & 0xFF = 0
		{100, 50, 150},  // no wrap
		{255, 255, 254}, // 510 & 0xFF = 254
	}
	for _, tc := range cases {
		got, err := runNative(t, m, "u8_wrap_add", tc.a, tc.b)
		if err != nil {
			t.Fatalf("u8_wrap_add(%d,%d): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("u8_wrap_add(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("u8_wrap_add(%d,%d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

func TestQBEU8WrapShift(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m := &mir2.Module{Name: "u8_wrap_shift"}
	buildU8WrapShift(m)

	// (a >> 1) + (a << 7), truncated to u8
	// This is the sfn_checksum rotation pattern.
	cases := []struct{ a, want int }{
		{0, 0},
		{1, 128},       // (0) + (128) = 128
		{2, 1},         // (1) + (256→0) = 1
		{255, 255},     // (127) + (128) = 255  (shift: 255<<7=32640→128)
		{0x55, 0xAA},   // (0x2A) + (0x80) = 0xAA  (shift: 0x55<<7=0x2A80→0x80)
	}
	for _, tc := range cases {
		got, err := runNative(t, m, "u8_wrap_shift", tc.a)
		if err != nil {
			t.Fatalf("u8_wrap_shift(%d): %v", tc.a, err)
		}
		if got != tc.want {
			t.Errorf("u8_wrap_shift(%d) = %d, want %d", tc.a, got, tc.want)
		} else {
			t.Logf("u8_wrap_shift(%d) = %d ✓", tc.a, got)
		}
	}
}

// ── emit smoke test ───────────────────────────────────────────────────────────

// TestQBEEmit just checks that Compile produces non-empty output for a trivial
// function, without invoking qbe or cc.
func TestQBEEmit(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	buildAbsDiff(m)
	out, err := mir2qbe.Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "abs_diff") {
		t.Errorf("expected 'abs_diff' in output:\n%s", out)
	}
	if !strings.Contains(out, "export function") {
		t.Errorf("expected 'export function' in output:\n%s", out)
	}
	t.Logf("QBE IR:\n%s", out)
}
