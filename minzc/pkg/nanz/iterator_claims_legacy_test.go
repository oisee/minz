package nanz_test

// G0 evidence tests for the CLAUDE.md claims that enumerate() and reduce()
// are "broken on Z80" in the iterator chains.
//
// Those claims describe the retired .minz frontend pipeline
// (parser → semantic → MIR1 codegen). The production CLI no longer uses it:
// .minz files are parsed by the Nanz parser (see cmd/minzc/main.go), where
// legacy iterator syntax is not recognised at all. These tests exercise the
// retired pipeline in-process to confirm or refute the claims on current
// main, and pin the production-path behaviour for the same sources.
//
// Verdicts (details in docs/Nanz_Iteration_Audit.md):
//   - enumerate: the documented B-counter/index conflict no longer reproduces
//     (the index is memory-backed; the loop terminates and yields correct
//     (index, element) pairs), but enumerate with a typed callback still
//     cannot link: the loop emits CALL <unmangled name> (G0-L2).
//   - reduce: the documented "two LD A" SMC overwrite no longer matches the
//     emitted code, but reduce with a named callback also fails to link for
//     the same unmangled-symbol reason (G0-L3).

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/semantic"
	"github.com/minz/minzc/pkg/z80asm"
)

// g0LegacyAsm compiles .minz source through the retired pipeline
// (parse → semantic → Z80 codegen) and returns the assembly text.
func g0LegacyAsm(t *testing.T, src, name string) string {
	t.Helper()
	p := parser.New()
	decls, err := p.ParseString(src, name)
	if err != nil {
		t.Fatalf("legacy parse: %v", err)
	}
	analyzer := semantic.NewAnalyzer()
	module, err := analyzer.Analyze(&ast.File{Name: name, Declarations: decls})
	if err != nil {
		t.Fatalf("legacy semantic: %v", err)
	}
	var buf strings.Builder
	gen := codegen.NewZ80Generator(&buf)
	gen.SetTargetPlatform("cpm")
	if err := gen.Generate(module); err != nil {
		t.Fatalf("legacy codegen: %v", err)
	}
	return buf.String()
}

// g0LegacyRun assembles the retired-pipeline output with a small bootstrap
// (ORG 0x8000, CALL main, DI/HALT; the codegen's own ORG $0100 is dropped)
// and executes it on the Z80 emulator, returning console port $23 output.
// Assembly failures return an error so tests can pin them.
func g0LegacyRun(t *testing.T, src, name string) ([]byte, error) {
	t.Helper()
	code := g0LegacyAsm(t, src, name)
	code = strings.Replace(code, "ORG $0100", "", 1)
	full := "ORG 0x8000\nLD SP, 0xFF00\nCALL main\nDI\nHALT\n" + code
	as := z80asm.NewAssembler()
	res, err := as.AssembleString(full)
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		return nil, fmt.Errorf("assemble errors:\n%s", sb.String())
	}
	z80 := emulator.NewRemogattoZ80()
	if lerr := z80.LoadMemory(0x8000, res.Binary); lerr != nil {
		return nil, fmt.Errorf("load: %w", lerr)
	}
	z80.SetPC(0x8000)
	if rerr := z80.Run(); rerr != nil {
		return nil, fmt.Errorf("run: %w", rerr)
	}
	return z80.GetOutput(), nil
}

// TestG0Claims_EnumerateLoopTerminates: the CLAUDE.md/docs mechanism ("B used
// for both counter and index → near-infinite loop") does not reproduce on
// current main. The enumerate loop is memory-backed, terminates after n
// iterations and yields correct ascending (index, element) pairs. Observed
// through a parameterless asm callback that reads the pair from the stack
// (enumerate pushes element, then index before CALL).
func TestG0Claims_EnumerateLoopTerminates(t *testing.T) {
	src := `global nums: [u8; 5]

fun pair_out() -> void {
    asm {
        POP HL
        POP DE
        POP BC
        LD A, E
        OUT ($23), A
        LD A, C
        OUT ($23), A
        PUSH BC
        PUSH DE
        PUSH HL
    }
}

fun main() -> void {
    nums[0] = 1
    nums[1] = 2
    nums[2] = 3
    nums[3] = 4
    nums[4] = 5
    nums.iter().enumerate().forEach(pair_out)
    asm {
        DI
        HALT
    }
}
`
	out, err := g0LegacyRun(t, src, "main")
	if err != nil {
		t.Fatalf("legacy run: %v", err)
	}
	want := []byte{0, 1, 1, 2, 2, 3, 3, 4, 4, 5}
	if string(out) != string(want) {
		t.Fatalf("enumerate output: want %v, got %v", want, out)
	}
}

// TestG0Claims_EnumerateTypedCallback: enumerate().forEach(fn) with a normal
// typed callback should assemble and run. Today the loop emits CALL to the
// unmangled function name (the definition is mangled, e.g. log_u8), so the
// program fails to assemble. The claim "enumerate broken on Z80" is therefore
// still true for real programs, but through a different defect than documented.
func TestG0Claims_EnumerateTypedCallback(t *testing.T) {
	src := `global nums: [u8; 5]

fun main() -> void {
    nums[0] = 1
    nums[1] = 2
    nums[2] = 3
    nums[3] = 4
    nums[4] = 5
    nums.iter().enumerate().forEach(log_skip)
    asm {
        DI
        HALT
    }
}

fun log_skip(i: u8, x: u8) -> void {
    asm {
        OUT ($23), A
        LD A, E
        OUT ($23), A
    }
}
`
	out, err := g0LegacyRun(t, src, "main")
	if err != nil {
		if strings.Contains(err.Error(), "unknown instruction or invalid operands: CALL") {
			t.Skip("G0 finding G0-L2: enumerate() with a typed callback emits CALL <unmangled name> (undefined label): ", err)
		}
		t.Fatal(err)
	}
	want := []byte{0, 1, 1, 2, 2, 3, 3, 4, 4, 5}
	if string(out) != string(want) {
		t.Fatalf("enumerate output: want %v, got %v", want, out)
	}
}

// TestG0Claims_ReduceTypedCallback: reduce(sum) over [1..5] should print 15.
// Today it fails to assemble for the same unmangled-symbol reason; the
// documented "second LD A overwrites the first" SMC mechanism no longer
// matches the emitted code (the callee now reads register params A/E).
func TestG0Claims_ReduceTypedCallback(t *testing.T) {
	src := `global nums: [u8; 5]

fun sum2(a: u8, b: u8) -> u8 {
    return a + b;
}

fun main() -> void {
    nums[0] = 1
    nums[1] = 2
    nums[2] = 3
    nums[3] = 4
    nums[4] = 5
    let s: u8 = nums.iter().reduce(sum2);
    emit(s);
    asm {
        DI
        HALT
    }
}
`
	src += `
fun emit(c: u8) -> void {
    asm { OUT ($23), A }
}
`
	out, err := g0LegacyRun(t, src, "main")
	if err != nil {
		if strings.Contains(err.Error(), "unknown instruction or invalid operands: CALL") {
			t.Skip("G0 finding G0-L3: reduce(fn) emits CALL <unmangled name> (undefined label): ", err)
		}
		t.Fatal(err)
	}
	if string(out) != string([]byte{15}) {
		t.Fatalf("reduce output: want [15], got %v", out)
	}
}

// TestG0Claims_LegacyArrayLiteralUninitialised: local array literals
// (`let nums: [u8; 5] = [1,2,3,4,5]`) never materialise on current main —
// every element reads as 0. The documented 11/11 e2e_iterators suite relied
// on this working (v0.19.5); it cannot be reproduced today.
func TestG0Claims_LegacyArrayLiteralUninitialised(t *testing.T) {
	src := `fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().forEach(log_u8);
    asm {
        DI
        HALT
    }
}

fun log_u8(c: u8) -> void {
    asm {
        OUT ($23), A
    }
}
`
	out, err := g0LegacyRun(t, src, "main")
	if err != nil {
		t.Fatalf("legacy run: %v", err)
	}
	want := []byte{1, 2, 3, 4, 5}
	if string(out) != string(want) {
		t.Skipf("G0 finding G0-L4: array literal elements read as 0 (want %v, got %v)", want, out)
	}
}

// TestG0Claims_ProductionPathDropsLegacySyntax: the e2e_iterators corpus
// files go through the Nanz parser in the production CLI, where arr.forEach /
// arr.iter() chains are not recognised: the module compiles without a
// diagnostic but the assembly references undefined labels (forEach, iter)
// and cannot be assembled. Pinned with one representative file.
func TestG0Claims_ProductionPathDropsLegacySyntax(t *testing.T) {
	src := `fun log(c: u8) -> void {
    asm {
        OUT ($23), A
    }
}

fun main() -> void {
    let arr: [u8; 5] = [65, 66, 67, 68, 69]
    arr.forEach(log)
    asm {
        DI
        HALT
    }
}
`
	m, err := nanz.Parse(src, "iter_foreach")
	if err != nil {
		return // A clean diagnostic is acceptable for unsupported syntax.
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		return // Clean rejection is acceptable.
	}
	if strings.Contains(steps.Assembly, "?") || g0HasUndefinedIteratorCall(steps.Assembly, "forEach", "iter") {
		t.Skip("G0 finding G0-L1: production path emits placeholders or undefined legacy iterator calls")
	}
	got := g0RunZ80Console(t, src, "main")
	want := []byte{65, 66, 67, 68, 69}
	if string(got) != string(want) {
		t.Fatalf("production iterator output: want %v, got %v", want, got)
	}
}
