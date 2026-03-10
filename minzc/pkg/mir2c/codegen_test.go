package mir2c_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2c"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/plm"
)

// ── pipeline helpers ──────────────────────────────────────────────────────────

func hirToMIR2(hm *hir.Module) *mir2.Module {
	m := hir.LowerModule(hm)
	mir2.LUTGen(m)
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
	}
	return m
}

func compilePLM(src string) (*mir2.Module, error) {
	hm, err := plm.Compile(src)
	if err != nil {
		return nil, fmt.Errorf("plm.Compile: %w", err)
	}
	return hirToMIR2(hm), nil
}

func compileNanz(src, name string) (*mir2.Module, error) {
	hm, err := nanz.Parse(src, name)
	if err != nil {
		return nil, fmt.Errorf("nanz.Parse: %w", err)
	}
	return hirToMIR2(hm), nil
}

// ── C runner ──────────────────────────────────────────────────────────────────

// runC compiles m to C99 via mir2c, links with a printf wrapper, and returns
// the integer printed by the binary.
func runC(t *testing.T, m *mir2.Module, funcName string, args ...int) (int, error) {
	t.Helper()

	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("cc not in PATH")
	}

	cSrc, err := mir2c.Compile(m)
	if err != nil {
		return 0, fmt.Errorf("mir2c.Compile: %w", err)
	}
	t.Logf("=== C source ===\n%s", cSrc)

	dir := t.TempDir()
	cPath := filepath.Join(dir, "out.c")
	if err := os.WriteFile(cPath, []byte(cSrc), 0644); err != nil {
		return 0, err
	}

	argList := make([]string, len(args))
	params := make([]string, len(args))
	for i, a := range args {
		argList[i] = strconv.Itoa(a)
		params[i] = "int"
	}
	mainSrc := fmt.Sprintf(`#include <stdio.h>
extern int %s(%s);
int main(void) { printf("%%d\n", (int)%s(%s)); return 0; }
`, funcName, strings.Join(params, ", "), funcName, strings.Join(argList, ", "))

	mainPath := filepath.Join(dir, "main.c")
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		return 0, err
	}

	binPath := filepath.Join(dir, "bin")
	if out, err := exec.Command("cc", "-Wno-unused-variable", "-Wno-sign-compare",
		cPath, mainPath, "-o", binPath).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("cc:\n%s\nC source:\n%s", out, cSrc)
	}

	runOut, err := exec.Command(binPath).Output()
	if err != nil {
		return 0, fmt.Errorf("run: %w", err)
	}
	return strconv.Atoi(strings.TrimSpace(string(runOut)))
}

// ── emit-only smoke tests (no cc required) ────────────────────────────────────

func TestEmitSmoke_Add(t *testing.T) {
	m := &mir2.Module{Name: "add"}
	f := m.AddFunc("add")
	bld := mir2.NewBuilder(f)
	entry := bld.SwitchToNewBlock("entry")
	_ = entry
	a := bld.Param("a", mir2.TyU16, mir2.ClassGeneral)
	b := bld.Param("b", mir2.TyU16, mir2.ClassGeneral)
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassGeneral}}
	r := bld.Add(a, b, mir2.TyU16, mir2.ClassGeneral)
	bld.Ret(r)

	src, err := mir2c.Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("emit:\n%s", src)
	if !strings.Contains(src, "r") {
		t.Error("expected register references in output")
	}
	if !strings.Contains(src, "return") {
		t.Error("expected return statement")
	}
}

func TestEmitSmoke_Global(t *testing.T) {
	m := &mir2.Module{Name: "gl"}
	m.Globals = []mir2.Global{
		{Name: "buf", Ty: mir2.NewArray(mir2.TyU8, 4), Init: nil},
		{Name: "lut", Ty: mir2.NewArray(mir2.TyU8, 3), Init: []byte{10, 20, 30}},
	}
	src, err := mir2c.Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(src)
	if !strings.Contains(src, "buf") {
		t.Error("expected global buf")
	}
	if !strings.Contains(src, "0x0a") && !strings.Contains(src, "10") {
		t.Error("expected initializer bytes")
	}
}

// ── E2E tests (require cc) ────────────────────────────────────────────────────

const plmDemoSrc = `DEMO: DO;

ABS_DIFF: PROCEDURE (A, B) BYTE;
  DECLARE (A, B) BYTE;
  IF A > B THEN
    RETURN A - B;
  ELSE
    RETURN B - A;
END ABS_DIFF;

FIB: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  DECLARE (A, B, T) BYTE;
  A = 0;
  B = 1;
  DO WHILE N > 0;
    T = B;
    B = A + B;
    A = T;
    N = N - 1;
  END;
  RETURN A;
END FIB;

END DEMO;`

func TestE2E_C_PLM_AbsDiff(t *testing.T) {
	m, err := compilePLM(plmDemoSrc)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ a, b, want int }{
		{10, 3, 7}, {3, 10, 7}, {5, 5, 0}, {0, 255, 255}, {100, 200, 100},
	}
	for _, tc := range cases {
		got, err := runC(t, m, "ABS_DIFF", tc.a, tc.b)
		if err != nil {
			t.Fatalf("ABS_DIFF(%d,%d): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("ABS_DIFF(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("ABS_DIFF(%d,%d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

func TestE2E_C_PLM_Fib(t *testing.T) {
	m, err := compilePLM(plmDemoSrc)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ n, want int }{
		{0, 0}, {1, 1}, {2, 1}, {3, 2}, {5, 5}, {8, 21}, {10, 55}, {13, 233},
	}
	for _, tc := range cases {
		got, err := runC(t, m, "FIB", tc.n)
		if err != nil {
			t.Fatalf("FIB(%d): %v", tc.n, err)
		}
		if got != tc.want {
			t.Errorf("FIB(%d) = %d, want %d", tc.n, got, tc.want)
		} else {
			t.Logf("FIB(%d) = %d ✓", tc.n, got)
		}
	}
}

const nanzAbsDiffSrc = `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b {
        return a - b
    } else {
        return b - a
    }
}
`

func TestE2E_C_Nanz_AbsDiff(t *testing.T) {
	m, err := compileNanz(nanzAbsDiffSrc, "abs_diff")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ a, b, want int }{
		{10, 3, 7}, {3, 10, 7}, {5, 5, 0}, {200, 100, 100},
	}
	for _, tc := range cases {
		got, err := runC(t, m, "abs_diff", tc.a, tc.b)
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

// TestE2E_C_Nanz_StructFields mirrors TestE2E_Nanz_StructFields from mir2qbe.
const nanzStructFieldsSrc = `
struct Color {
    r: u8
    g: u8
    b: u8
}

global palette: Color

fun set_rgb(rv: u8, gv: u8, bv: u8) -> void {
    palette.r = rv
    palette.g = gv
    palette.b = bv
}

fun get_r() -> u8 { return palette.r }
fun get_g() -> u8 { return palette.g }
fun get_b() -> u8 { return palette.b }
`

func TestE2E_C_Nanz_StructFields(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("cc not in PATH")
	}
	m, err := compileNanz(nanzStructFieldsSrc, "struct_fields")
	if err != nil {
		t.Fatal(err)
	}

	cSrc, err := mir2c.Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("C source:\n%s", cSrc)

	dir := t.TempDir()
	cPath := filepath.Join(dir, "out.c")
	if err := os.WriteFile(cPath, []byte(cSrc), 0644); err != nil {
		t.Fatal(err)
	}
	mainSrc := `#include <stdio.h>
extern void set_rgb(int r, int g, int b);
extern int get_r(void);
extern int get_g(void);
extern int get_b(void);
int main(void) {
    set_rgb(10, 20, 30);
    printf("%d %d %d\n", get_r(), get_g(), get_b());
    set_rgb(100, 150, 200);
    printf("%d %d %d\n", get_r(), get_g(), get_b());
    return 0;
}`
	mainPath := filepath.Join(dir, "main.c")
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, "bin")
	if out, err := exec.Command("cc", "-Wno-unused-variable", cPath, mainPath, "-o", binPath).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\nsrc:\n%s", err, out, cSrc)
	}
	out, err := exec.Command(binPath).Output()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	t.Logf("output:\n%s", out)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	want := []string{"10 20 30", "100 150 200"}
	for i, w := range want {
		if i >= len(lines) {
			t.Errorf("missing output line %d", i)
			continue
		}
		got := strings.TrimSpace(lines[i])
		if got != w {
			t.Errorf("line %d: got %q, want %q", i, got, w)
		} else {
			t.Logf("line %d: %q ✓", i, got)
		}
	}
}
