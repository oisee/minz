package nanz_test

// E2E emulator tests for enum and type alias features — verifies that
// the generated Z80 assembly produces correct results when executed.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const enumLoadAddr = 0x8000

func compileAndRunEnum(t *testing.T, nanzSrc, funcName string, args ...int) (uint8, error) {
	t.Helper()
	m, err := nanz.Parse(nanzSrc, "enum_e2e")
	if err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}
	genAsm, err := pipeline.CompileHIR(m)
	if err != nil {
		return 0, fmt.Errorf("compile: %w", err)
	}

	// Build bootstrap: load args into A (u8), call function, HALT
	var boot string
	switch len(args) {
	case 0:
		boot = fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    CALL %s
    DI
    HALT
`, enumLoadAddr, funcName)
	case 1:
		boot = fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    CALL %s
    DI
    HALT
`, enumLoadAddr, args[0], funcName)
	default:
		return 0, fmt.Errorf("too many args")
	}

	src := boot + "\n" + genAsm
	as := z80asm.NewAssembler()
	res, err := as.AssembleString(src)
	if err != nil {
		return 0, fmt.Errorf("assemble: %w", err)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		return 0, fmt.Errorf("assemble errors:\n%s", sb.String())
	}
	z80 := emulator.NewRemogattoZ80()
	if lerr := z80.LoadMemory(enumLoadAddr, res.Binary); lerr != nil {
		return 0, fmt.Errorf("load: %w", lerr)
	}
	z80.SetPC(enumLoadAddr)
	if rerr := z80.Run(); rerr != nil {
		return 0, fmt.Errorf("run: %w", rerr)
	}
	regs := z80.GetRegisters()
	return regs.A, nil
}

func TestEnum_E2E_AutoNumbered(t *testing.T) {
	src := `enum Dir {
    UP,
    DOWN,
    LEFT,
    RIGHT
}

fun get_right() -> u8 {
    return Dir.RIGHT
}
`
	got, err := compileAndRunEnum(t, src, "get_right")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 3 {
		t.Errorf("Dir.RIGHT: want 3, got %d", got)
	}
}

func TestEnum_E2E_ExplicitValues(t *testing.T) {
	src := `enum Color {
    RED = 1,
    GREEN = 2,
    BLUE = 4,
    WHITE = 7
}

fun get_blue() -> u8 {
    return Color.BLUE
}
`
	got, err := compileAndRunEnum(t, src, "get_blue")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 4 {
		t.Errorf("Color.BLUE: want 4, got %d", got)
	}
}

func TestEnum_E2E_MixedAutoExplicit(t *testing.T) {
	// After explicit value, auto-numbering continues from next
	src := `enum Prio {
    LOW,
    NORMAL = 5,
    HIGH,
    CRITICAL
}

fun get_critical() -> u8 {
    return Prio.CRITICAL
}
`
	got, err := compileAndRunEnum(t, src, "get_critical")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	// LOW=0, NORMAL=5, HIGH=6, CRITICAL=7
	if got != 7 {
		t.Errorf("Prio.CRITICAL: want 7, got %d", got)
	}
}

func TestTypeAlias_E2E(t *testing.T) {
	src := `type Score = u8

fun double_score(s: Score) -> Score {
    return (s + s)
}
`
	got, err := compileAndRunEnum(t, src, "double_score", 21)
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 42 {
		t.Errorf("double_score(21): want 42, got %d", got)
	}
}

// ── ADT (enum with payload) tests ──────────────────────────────────────────

func TestADT_E2E_SimpleMatch(t *testing.T) {
	src := `enum Color { Red, Green, Blue }

fun describe(c: Color) -> u8 {
    return match c {
        Red   => 1,
        Green => 2,
        Blue  => 3,
    }
}

fun test_match() -> u8 {
    var c: Color = Color.Green
    return describe(c)
}
`
	got, err := compileAndRunEnum(t, src, "test_match")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 2 {
		t.Errorf("match Green: want 2, got %d", got)
	}
}

func TestADT_E2E_PayloadConstructor(t *testing.T) {
	src := `enum Option { None, Some(u8) }

fun get_some_val() -> u8 {
    var x: u16 = 0
    x = Some(42)
    return u8((x % 256))
}
`
	got, err := compileAndRunEnum(t, src, "get_some_val")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 42 {
		t.Errorf("Some(42) payload: want 42, got %d", got)
	}
}

func TestADT_E2E_PayloadTag(t *testing.T) {
	src := `enum Option { None, Some(u8) }

fun get_some_tag() -> u8 {
    var x: u16 = 0
    x = Some(42)
    return u8((x / 256))
}
`
	got, err := compileAndRunEnum(t, src, "get_some_tag")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 1 {
		t.Errorf("Some tag: want 1, got %d", got)
	}
}

func TestADT_E2E_NoneTag(t *testing.T) {
	src := `enum Option { None, Some(u8) }

fun get_none_tag() -> u8 {
    var x: u16 = 0
    x = None
    return u8((x / 256))
}
`
	got, err := compileAndRunEnum(t, src, "get_none_tag")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 0 {
		t.Errorf("None tag: want 0, got %d", got)
	}
}

func TestADT_E2E_TagHelper(t *testing.T) {
	src := `enum Option { None, Some(u8) }

fun test_tag() -> u8 {
    var x: u16 = Some(99)
    return __tag(x)
}
`
	got, err := compileAndRunEnum(t, src, "test_tag")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 1 {
		t.Errorf("__tag(Some(99)): want 1, got %d", got)
	}
}

func TestADT_E2E_PayloadHelper(t *testing.T) {
	src := `enum Option { None, Some(u8) }

fun test_payload() -> u8 {
    var x: u16 = Some(99)
    return __payload(x)
}
`
	got, err := compileAndRunEnum(t, src, "test_payload")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 99 {
		t.Errorf("__payload(Some(99)): want 99, got %d", got)
	}
}

func TestADT_E2E_NoneVsPayload(t *testing.T) {
	src := `enum Option { None, Some(u8) }

fun is_some(opt: u16) -> u8 {
    return __tag(opt)
}

fun test_none_check() -> u8 {
    var x: u16 = None
    return is_some(x)
}
`
	got, err := compileAndRunEnum(t, src, "test_none_check")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 0 {
		t.Errorf("is_some(None): want 0, got %d", got)
	}
}

func TestEnum_E2E_InExpression(t *testing.T) {
	src := `enum Base {
    ZERO = 0,
    TEN = 10,
    TWENTY = 20
}

fun add_bases() -> u8 {
    return (Base.TEN + Base.TWENTY)
}
`
	got, err := compileAndRunEnum(t, src, "add_bases")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 30 {
		t.Errorf("Base.TEN + Base.TWENTY: want 30, got %d", got)
	}
}
