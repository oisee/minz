package plm_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/plm"
	"github.com/minz/minzc/pkg/z80asm"
)

// ── Pipeline helpers ──────────────────────────────────────────────────────────

const loadAddr = 0x8000

// compilePLM: PL/M source → Z80 assembly string.
func compilePLM(t *testing.T, src string) string {
	t.Helper()
	hm, err := plm.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return compileHIR(t, hm)
}

// compileHIR: HIR module → Z80 assembly string.
func compileHIR(t *testing.T, hm *hir.Module) string {
	t.Helper()
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
	}
	return mir2.Z80Codegen(m, combined)
}

// runZ80 assembles src and runs it; returns (A, HL, err).
func runZ80(t *testing.T, src string) (a uint8, hl uint16, err error) {
	t.Helper()
	asm := z80asm.NewAssembler()
	res, asmErr := asm.AssembleString(src)
	if asmErr != nil {
		return 0, 0, fmt.Errorf("assemble: %w", asmErr)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		return 0, 0, fmt.Errorf("assemble errors:\n%s", sb.String())
	}
	z80 := emulator.NewRemogattoZ80()
	if loadErr := z80.LoadMemory(loadAddr, res.Binary); loadErr != nil {
		return 0, 0, fmt.Errorf("load memory: %w", loadErr)
	}

	// Set up stack and return address at 0xFFF0 → ret to $0000.
	z80.SetSP(0xFFF0)
	mem := z80.Memory()
	mem[0xFFF0] = 0x00
	mem[0xFFF1] = 0x00

	z80.SetPC(loadAddr)
	for i := 0; i < 200_000; i++ {
		z80.Step()
		if z80.GetPC() == 0x0000 {
			break
		}
	}
	regs := z80.GetRegisters()
	return regs.A, regs.HL, nil
}

// callFn wraps the named function in a tiny harness, sets A/HL args, and runs.
func callFn(t *testing.T, asm string, fnName string, argA uint8, argHL uint16) (retA uint8, retHL uint16) {
	t.Helper()
	// Build a harness: LD A,n; LD HL,n; CALL fn; HALT (at 0x0000 → acts as RET target)
	harness := fmt.Sprintf(`
ORG 0x%04X
	LD A, %d
	LD HL, %d
	CALL %s
	RET
%s
`, loadAddr, argA, argHL, fnName, asm)
	a, hl, err := runZ80(t, harness)
	if err != nil {
		t.Fatalf("runZ80: %v", err)
	}
	return a, hl
}

// ── Lexer tests ───────────────────────────────────────────────────────────────

func TestLexer_Basic(t *testing.T) {
	src := `DECLARE X BYTE; /* a comment */`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLexer_HexLiteral(t *testing.T) {
	// 0FFH should tokenize as a number, then be used in a DECLARE/expression.
	src := `
TEST: DO;
ADD: PROCEDURE (A, B) BYTE;
  DECLARE (A, B) BYTE;
  RETURN A + 0FFH;
END ADD;
END TEST;
`
	m, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Decls) == 0 {
		t.Fatal("expected at least one declaration")
	}
}

// ── Parser tests ──────────────────────────────────────────────────────────────

func TestParser_SimpleProc(t *testing.T) {
	src := `
SAMPLE: DO;

ADD: PROCEDURE (X, Y) BYTE;
  DECLARE (X, Y) BYTE;
  RETURN X + Y;
END ADD;

END SAMPLE;
`
	m, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.Name != "SAMPLE" {
		t.Errorf("module name: got %q, want %q", m.Name, "SAMPLE")
	}
	if len(m.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(m.Decls))
	}
}

func TestParser_IfElse(t *testing.T) {
	src := `
M: DO;
ABS: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  IF N < 128 THEN
    RETURN N;
  ELSE
    RETURN -N;
END ABS;
END M;
`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestParser_DoWhile(t *testing.T) {
	src := `
M: DO;
SUM: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  DECLARE (S, I) BYTE;
  S = 0;
  I = 0;
  DO WHILE I < N;
    S = S + I;
    I = I + 1;
  END;
  RETURN S;
END SUM;
END M;
`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestParser_DoCase(t *testing.T) {
	src := `
M: DO;
CLASSIFY: PROCEDURE (X) BYTE;
  DECLARE X BYTE;
  DO CASE X;
    RETURN 10;
    RETURN 20;
    RETURN 30;
  END;
  RETURN 0;
END CLASSIFY;
END M;
`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestParser_BitwiseOps(t *testing.T) {
	src := `
M: DO;
MASK: PROCEDURE (A, B) BYTE;
  DECLARE (A, B) BYTE;
  RETURN (A AND 0FH) OR (B AND 0F0H);
END MASK;
END M;
`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestParser_GlobalDecl(t *testing.T) {
	src := `
M: DO;
DECLARE COUNTER BYTE;
DECLARE STATUS WORD;
END M;
`
	m, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Decls) != 2 {
		t.Errorf("expected 2 global decls, got %d", len(m.Decls))
	}
}

// ── New feature tests (corpus-driven) ────────────────────────────────────────

func TestPreprocess_Literally(t *testing.T) {
	src := `
DECLARE TRUE LITERALLY '1';
DECLARE FALSE LITERALLY '0';
DECLARE MAX$SIZE LITERALLY '128';
M: DO;
CHECK: PROCEDURE (X) BYTE;
  DECLARE X BYTE;
  IF X = TRUE THEN RETURN FALSE;
  RETURN TRUE;
END CHECK;
END M;
`
	m, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse with LITERALLY: %v", err)
	}
	if len(m.Decls) != 1 {
		t.Errorf("expected 1 decl after LITERALLY removal, got %d", len(m.Decls))
	}
}

func TestParser_ArrayDecl(t *testing.T) {
	src := `
M: DO;
DECLARE BUFFER(128) BYTE;
DECLARE TABLE(16) WORD;
END M;
`
	m, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse array decl: %v", err)
	}
	if len(m.Decls) != 2 {
		t.Errorf("expected 2 decls, got %d", len(m.Decls))
	}
}

func TestParser_DoTo(t *testing.T) {
	src := `
M: DO;
SUM: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  DECLARE (S, I) BYTE;
  S = 0;
  DO I = 0 TO N;
    S = S + I;
  END;
  RETURN S;
END SUM;
END M;
`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse DO TO: %v", err)
	}
}

func TestParser_DoToBy(t *testing.T) {
	src := `
M: DO;
EVENS: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  DECLARE (S, I) BYTE;
  S = 0;
  DO I = 0 TO N BY 2;
    S = S + I;
  END;
  RETURN S;
END EVENS;
END M;
`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse DO TO BY: %v", err)
	}
}

func TestParser_EnableDisableHalt(t *testing.T) {
	src := `
M: DO;
CRITICAL: PROCEDURE;
  DISABLE;
  HALT;
  ENABLE;
END CRITICAL;
END M;
`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse ENABLE/DISABLE/HALT: %v", err)
	}
}

func TestParser_BasedDecl(t *testing.T) {
	src := `
M: DO;
DECLARE MEMPTR WORD;
DECLARE DATA BASED MEMPTR BYTE;
END M;
`
	_, err := plm.ParseModule(src)
	if err != nil {
		t.Fatalf("parse BASED: %v", err)
	}
}

// ── Lowering + compilation tests ──────────────────────────────────────────────

func TestLower_DoTo(t *testing.T) {
	// Sum 0+1+2+...+N using DO I = 0 TO N; (counted loop).
	// Also tests LITERALLY substitution.
	src := `
DECLARE ZERO LITERALLY '0';
SUM: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  DECLARE (S, I) BYTE;
  S = ZERO;
  DO I = 0 TO N - 1;
    S = S + I;
  END;
  RETURN S;
END SUM;
`
	asm := compilePLM(t, src)
	t.Logf("do-to assembly:\n%s", asm)

	type tc struct{ n, want uint8 }
	cases := []tc{{5, 10}, {4, 6}, {1, 0}, {3, 3}}
	for _, c := range cases {
		harness := fmt.Sprintf(`
ORG 0x8000
    LD A, %d
    CALL SUM
    RET
%s
`, c.n, asm)
		a, _, err := runZ80(t, harness)
		if err != nil {
			t.Fatalf("sum(%d): run error: %v", c.n, err)
		}
		if a != c.want {
			t.Errorf("sum(%d) = %d, want %d", c.n, a, c.want)
		}
	}
}

func TestLower_AddBytes(t *testing.T) {
	src := `
ADD: PROCEDURE (X, Y) BYTE;
  DECLARE (X, Y) BYTE;
  RETURN X + Y;
END ADD;
`
	asm := compilePLM(t, src)
	if !strings.Contains(asm, "ADD") {
		t.Log("assembly:\n", asm)
		t.Error("expected ADD label in assembly")
	}
}

func TestLower_SumLoop(t *testing.T) {
	// SUM(N) = 0 + 1 + 2 + ... + (N-1) = N*(N-1)/2
	// Using only + operator (no * or /).
	src := `
SUM: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  DECLARE (S, I) BYTE;
  S = 0;
  I = 0;
  DO WHILE I < N;
    S = S + I;
    I = I + 1;
  END;
  RETURN S;
END SUM;
`
	asm := compilePLM(t, src)
	t.Logf("sum assembly:\n%s", asm)

	type tc struct{ n, want uint8 }
	cases := []tc{{5, 10}, {4, 6}, {1, 0}, {3, 3}}
	for _, c := range cases {
		harness := fmt.Sprintf(`
ORG 0x8000
    LD A, %d
    CALL SUM
    RET
%s
`, c.n, asm)
		a, _, err := runZ80(t, harness)
		if err != nil {
			t.Fatalf("sum(%d): run error: %v", c.n, err)
		}
		if a != c.want {
			t.Errorf("sum(%d) = %d, want %d", c.n, a, c.want)
		}
	}
}

func TestLower_GCDSubtraction(t *testing.T) {
	// GCD via binary-subtraction algorithm (no division needed).
	src := `
GCD: PROCEDURE (A, B) BYTE;
  DECLARE (A, B) BYTE;
  DO WHILE A <> B;
    IF A > B THEN A = A - B;
    ELSE B = B - A;
  END;
  RETURN A;
END GCD;
`
	asm := compilePLM(t, src)
	t.Logf("gcd assembly:\n%s", asm)

	type tc struct{ a, b, want uint8 }
	cases := []tc{{12, 8, 4}, {15, 10, 5}, {7, 7, 7}}
	for _, c := range cases {
		harness := fmt.Sprintf(`
ORG 0x8000
    LD A, %d
    LD C, %d
    CALL GCD
    RET
%s
`, c.a, c.b, asm)
		a, _, err := runZ80(t, harness)
		if err != nil {
			t.Fatalf("gcd(%d,%d): run error: %v", c.a, c.b, err)
		}
		if a != c.want {
			t.Errorf("gcd(%d,%d) = %d, want %d", c.a, c.b, a, c.want)
		}
	}
}

func TestLower_MaxByte(t *testing.T) {
	// MAX(A, B) returning the larger BYTE value.
	src := `
MAXB: PROCEDURE (A, B) BYTE;
  DECLARE (A, B) BYTE;
  IF A > B THEN RETURN A;
  RETURN B;
END MAXB;
`
	asm := compilePLM(t, src)
	t.Logf("maxb assembly:\n%s", asm)

	type tc struct{ a, b, want uint8 }
	cases := []tc{{10, 20, 20}, {30, 15, 30}, {7, 7, 7}}
	for _, c := range cases {
		harness := fmt.Sprintf(`
ORG 0x8000
    LD A, %d
    LD C, %d
    CALL MAXB
    RET
%s
`, c.a, c.b, asm)
		a, _, err := runZ80(t, harness)
		if err != nil {
			t.Fatalf("maxb(%d,%d): run error: %v", c.a, c.b, err)
		}
		if a != c.want {
			t.Errorf("maxb(%d,%d) = %d, want %d", c.a, c.b, a, c.want)
		}
	}
}
