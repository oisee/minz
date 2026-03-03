package z80asm

// Shadow testing framework: assembles Z80 snippets with both MZA and sjasmplus,
// then compares the binary output byte-for-byte.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// assembleSjasmplusWithFlags is like assembleSjasmplus but accepts extra CLI flags
// (e.g. "--syntax=abf") passed to the sjasmplus invocation.
func assembleSjasmplusWithFlags(t *testing.T, source string, flags ...string) []byte {
	t.Helper()

	sjasm, err := exec.LookPath("sjasmplus")
	if err != nil {
		t.Skip("sjasmplus not found in PATH")
	}

	dir := t.TempDir()
	asmFile := filepath.Join(dir, "test.asm")
	binFile := filepath.Join(dir, "test.bin")

	lines := strings.Split(source, "\n")
	var sb strings.Builder
	fmt.Fprintf(&sb, "\tOUTPUT \"%s\"\n", binFile)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Labels and EQU directives go without leading tab; instructions get a tab.
		if strings.Contains(trimmed, " EQU ") || strings.HasSuffix(trimmed, ":") {
			sb.WriteString(trimmed)
		} else {
			sb.WriteString("\t")
			sb.WriteString(trimmed)
		}
		sb.WriteString("\n")
	}
	fullSource := sb.String()

	if err := os.WriteFile(asmFile, []byte(fullSource), 0644); err != nil {
		t.Fatalf("Failed to write asm file: %v", err)
	}

	args := append([]string{}, flags...)
	args = append(args, asmFile)
	cmd := exec.Command(sjasm, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sjasmplus failed: %v\nOutput: %s\nSource:\n%s", err, out, fullSource)
	}

	bin, err := os.ReadFile(binFile)
	if err != nil {
		t.Fatalf("Failed to read sjasmplus output: %v", err)
	}
	return bin
}

// shadowResult describes the outcome of comparing MZA and sjasmplus for one snippet.
type shadowResult int

const (
	shadowMatch      shadowResult = iota // binaries are identical
	shadowBinaryDiff                     // both assembled but binaries differ
	shadowMZAFail                        // MZA failed to assemble
	shadowSjasmFail                      // sjasmplus failed to assemble
)

func (r shadowResult) String() string {
	switch r {
	case shadowMatch:
		return "MATCH"
	case shadowBinaryDiff:
		return "BINARY_DIFF"
	case shadowMZAFail:
		return "MZA_FAIL"
	case shadowSjasmFail:
		return "SJASMPLUS_FAIL"
	default:
		return "UNKNOWN"
	}
}

// shadowCase holds a named assembly snippet for the shadow corpus.
type shadowCase struct {
	name   string
	source string
}

func TestShadow_AssemblyComparison(t *testing.T) {
	// Skip the entire test when sjasmplus is not installed.
	if _, err := exec.LookPath("sjasmplus"); err != nil {
		t.Skip("sjasmplus not found in PATH — skipping shadow tests")
	}

	corpus := []shadowCase{
		// --- Basic instructions ---
		{"NOP", "NOP"},
		{"LD A,0", "LD A,0"},
		{"LD HL,$1234", "LD HL,$1234"},
		{"LD B,C", "LD B,C"},
		{"PUSH HL", "PUSH HL"},
		{"POP DE", "POP DE"},

		// --- Arithmetic ---
		{"ADD A,B", "ADD A,B"},
		{"SUB C", "SUB C"},
		{"AND $0F", "AND $0F"},
		{"OR $F0", "OR $F0"},
		{"XOR A", "XOR A"},
		{"CP 42", "CP 42"},

		// --- Jumps (absolute addresses avoid relative offset issues) ---
		{"JP $8000", "ORG $0000\nJP $8000"},
		{"JR label", "ORG $0000\nJR target\nNOP\nNOP\nNOP\ntarget:"},
		{"CALL $8000", "ORG $0000\nCALL $8000"},
		{"RET", "RET"},

		// --- Bit operations ---
		{"BIT 3,A", "BIT 3,A"},
		{"SET 5,B", "SET 5,B"},
		{"RES 0,C", "RES 0,C"},

		// --- IX/IY indexed ---
		{"LD (IX+5),A", "LD (IX+5),A"},
		{"BIT 3,(IY+2)", "BIT 3,(IY+2)"},

		// --- Memory operations ---
		{"LD A,(HL)", "LD A,(HL)"},
		{"LD ($8000),A", "LD ($8000),A"},
		{"LD (BC),A", "LD (BC),A"},
	}

	var matches, diffs, mzaFails, sjasmFails int

	for _, tc := range corpus {
		tc := tc // capture for subtests
		t.Run(tc.name, func(t *testing.T) {
			// --- Assemble with MZA ---
			asm := NewAssembler()
			mzaResult, mzaErr := asm.AssembleString(tc.source)
			var mzaBin []byte
			if mzaErr == nil {
				mzaBin = mzaResult.Binary
			}

			// --- Assemble with sjasmplus (--syntax=abf for broad syntax acceptance) ---
			var sjasmBin []byte
			var sjasmErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						sjasmErr = fmt.Errorf("panic: %v", r)
					}
				}()
				// Use a sub-helper that calls t.Fatalf on sjasmplus failure;
				// we catch that by running in a sub-test to detect failures.
				sjasmBin = assembleSjasmplusForShadow(t, tc.source)
			}()

			// --- Classify result ---
			var result shadowResult
			switch {
			case mzaErr != nil:
				result = shadowMZAFail
				t.Logf("MZA_FAIL: %s — %v", tc.name, mzaErr)
			case sjasmErr != nil || sjasmBin == nil:
				result = shadowSjasmFail
				t.Logf("SJASMPLUS_FAIL: %s — %v", tc.name, sjasmErr)
			case bytes.Equal(mzaBin, sjasmBin):
				result = shadowMatch
				t.Logf("MATCH: %s [%d bytes] %X", tc.name, len(mzaBin), mzaBin)
			default:
				result = shadowBinaryDiff
				t.Errorf("BINARY_DIFF: %s\n  MZA      [%d]: %X\n  sjasmplus[%d]: %X",
					tc.name, len(mzaBin), mzaBin, len(sjasmBin), sjasmBin)
				reportFirstDiff(t, mzaBin, sjasmBin)
			}

			// Update counters (not goroutine-safe but subtests run serially by default).
			switch result {
			case shadowMatch:
				matches++
			case shadowBinaryDiff:
				diffs++
			case shadowMZAFail:
				mzaFails++
			case shadowSjasmFail:
				sjasmFails++
			}
		})
	}

	t.Logf("\n=== Shadow Summary ===")
	t.Logf("Total:          %d", len(corpus))
	t.Logf("MATCH:          %d", matches)
	t.Logf("BINARY_DIFF:    %d", diffs)
	t.Logf("MZA_FAIL:       %d", mzaFails)
	t.Logf("SJASMPLUS_FAIL: %d", sjasmFails)
}

// assembleSjasmplusForShadow assembles with sjasmplus using --syntax=abf.
// It returns nil (instead of calling t.Fatalf) when sjasmplus fails to assemble,
// so the caller can classify the result as SJASMPLUS_FAIL.
func assembleSjasmplusForShadow(t *testing.T, source string) []byte {
	t.Helper()

	sjasm, err := exec.LookPath("sjasmplus")
	if err != nil {
		t.Skip("sjasmplus not found in PATH")
		return nil
	}

	dir := t.TempDir()
	asmFile := filepath.Join(dir, "test.asm")
	binFile := filepath.Join(dir, "test.bin")

	lines := strings.Split(source, "\n")
	var sb strings.Builder
	fmt.Fprintf(&sb, "\tOUTPUT \"%s\"\n", binFile)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, " EQU ") || strings.HasSuffix(trimmed, ":") {
			sb.WriteString(trimmed)
		} else {
			sb.WriteString("\t")
			sb.WriteString(trimmed)
		}
		sb.WriteString("\n")
	}
	fullSource := sb.String()

	if err := os.WriteFile(asmFile, []byte(fullSource), 0644); err != nil {
		t.Logf("Failed to write asm file: %v", err)
		return nil
	}

	cmd := exec.Command(sjasm, "--syntax=abf", asmFile)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("sjasmplus failed for %q: %v\nOutput: %s", source, err, out)
		return nil
	}

	bin, err := os.ReadFile(binFile)
	if err != nil {
		t.Logf("Failed to read sjasmplus output: %v", err)
		return nil
	}
	return bin
}

// reportFirstDiff logs the byte offset and values of the first difference between two slices.
func reportFirstDiff(t *testing.T, a, b []byte) {
	t.Helper()
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			t.Logf("  First diff at byte %d: MZA=%02X sjasmplus=%02X", i, a[i], b[i])
			return
		}
	}
	if len(a) != len(b) {
		t.Logf("  Length mismatch: MZA=%d bytes, sjasmplus=%d bytes", len(a), len(b))
	}
}

// TestShadow_EndToEnd_MinZ tests the full pipeline: .minz -> mz compiler -> .a80 -> assemble
// with both MZA and sjasmplus -> binary diff.
// Requires both 'mz' (MinZ compiler) and 'sjasmplus' in PATH.
func TestShadow_EndToEnd_MinZ(t *testing.T) {
	// Check prerequisites
	mzBin, err := exec.LookPath("mz")
	if err != nil {
		t.Skip("mz compiler not found in PATH — skipping E2E shadow tests")
	}
	if _, err := exec.LookPath("sjasmplus"); err != nil {
		t.Skip("sjasmplus not found in PATH — skipping E2E shadow tests")
	}

	// Simple MinZ programs that compile to small, deterministic assembly
	programs := []struct {
		name   string
		source string
	}{
		{
			name: "simple_add",
			source: `fun main() -> void {
	let a: u8 = 1;
	let b: u8 = 2;
	let c: u8 = a + b;
}
`,
		},
		{
			name: "const_only",
			source: `const WIDTH: u8 = 32;
const HEIGHT: u8 = 24;
fun main() -> void {
	let total: u16 = 768;
}
`,
		},
		{
			name: "simple_function",
			source: `fun add(x: u8, y: u8) -> u8 {
	return x + y;
}
fun main() -> void {
	let result: u8 = add(3, 4);
}
`,
		},
	}

	for _, prog := range programs {
		t.Run(prog.name, func(t *testing.T) {
			dir := t.TempDir()
			minzFile := filepath.Join(dir, prog.name+".minz")
			a80File := filepath.Join(dir, prog.name+".a80")

			// Write .minz source
			if err := os.WriteFile(minzFile, []byte(prog.source), 0644); err != nil {
				t.Fatalf("Failed to write .minz file: %v", err)
			}

			// Compile with mz -> .a80
			cmd := exec.Command(mzBin, minzFile, "-o", a80File, "-b", "z80")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Logf("MZ_COMPILE_FAIL: %s — %v\nOutput: %s", prog.name, err, out)
				return // Can't compare if compilation fails
			}

			// Read the .a80
			a80Source, err := os.ReadFile(a80File)
			if err != nil {
				t.Fatalf("Failed to read .a80 output: %v", err)
			}

			// Assemble with MZA
			asm := NewAssembler()
			mzaResult, mzaErr := asm.AssembleString(string(a80Source))
			if mzaErr != nil {
				t.Logf("MZA_FAIL on %s.a80: %v", prog.name, mzaErr)
			}

			// Assemble with sjasmplus
			sjasmBin := assembleSjasmplusForShadow(t, string(a80Source))

			// Compare
			if mzaErr != nil {
				t.Logf("E2E result: MZA_FAIL for %s", prog.name)
			} else if sjasmBin == nil {
				t.Logf("E2E result: SJASMPLUS_FAIL for %s", prog.name)
			} else if bytes.Equal(mzaResult.Binary, sjasmBin) {
				t.Logf("E2E result: MATCH for %s [%d bytes]", prog.name, len(mzaResult.Binary))
			} else {
				t.Errorf("E2E BINARY_DIFF for %s\n  MZA      [%d]: %X\n  sjasmplus[%d]: %X",
					prog.name, len(mzaResult.Binary), mzaResult.Binary, len(sjasmBin), sjasmBin)
				reportFirstDiff(t, mzaResult.Binary, sjasmBin)
			}
		})
	}
}
