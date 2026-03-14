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
