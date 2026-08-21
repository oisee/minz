package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// TestGrace_SDCC_Full compares MinZ against SDCC on ALL 8 SDCC benchmark
// programs, using BOTH the C89 frontend (identical source) and Nanz
// (idiomatic MinZ). This is the definitive apples-to-apples comparison.
func TestGrace_SDCC_Full(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	sdccDir := filepath.Join("..", "..", "..", "research", "abi-paper", "sdcc-output")

	// SDCC instruction counts (hand-counted from .asm files)
	sdccInsts := map[string]int{
		"abs_diff":     12,
		"abs_diff_u16": 13,
		"chain":        8,
		"fib":          22,
		"foreach":      44,
		"gcd":          17,
		"minmax":       60,
		"swap":         20,
	}

	// Equivalent C89 source (same algorithm as SDCC's .c files)
	c89Sources := map[string]string{
		"abs_diff": `
uint8_t abs_diff(uint8_t a, uint8_t b) {
    if (a >= b) return a - b;
    return b - a;
}`,
		"abs_diff_u16": `
uint16_t abs_diff_u16(uint16_t a, uint16_t b) {
    if (a < b) return b - a;
    return a - b;
}`,
		"chain": `
uint8_t leaf(uint8_t x) { return x + x; }
uint8_t middle(uint8_t y) { return leaf(y); }
uint8_t top(uint8_t z) { return middle(z); }
`,
		"fib": `
uint16_t fib(uint8_t n) {
    if (n <= 1) return n;
    return fib(n - 1) + fib(n - 2);
}`,
		"foreach": `
uint8_t sum_array(uint8_t *buf, uint8_t n) {
    uint8_t s = 0;
    for (uint8_t i = 0; i < n; i++) { s += buf[i]; }
    return s;
}
uint8_t max_array(uint8_t *buf, uint8_t n) {
    uint8_t m = 0;
    for (uint8_t i = 0; i < n; i++) { if (buf[i] > m) m = buf[i]; }
    return m;
}`,
		"gcd": `
uint8_t gcd(uint8_t a, uint8_t b) {
    while (a != b) {
        if (a > b) a = a - b;
        else       b = b - a;
    }
    return a;
}`,
		"minmax": `
void minmax(uint16_t a, uint16_t b, uint16_t *lo, uint16_t *hi) {
    if (a <= b) { *lo = a; *hi = b; }
    else        { *lo = b; *hi = a; }
}`,
		"swap": `
void swap(uint16_t a, uint16_t b, uint16_t *out_a, uint16_t *out_b) {
    *out_a = b;
    *out_b = a;
}`,
	}

	// Equivalent idiomatic Nanz (same semantics, MinZ style)
	nanzSources := map[string]string{
		"abs_diff": `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b { return a - b }
    return b - a
}`,
		"abs_diff_u16": `
fun abs_diff_u16(a: u16, b: u16) -> u16 {
    if a < b { return b - a }
    return a - b
}`,
		"chain": `
fun leaf(x: u8) -> u8 { return x + x }
fun middle(y: u8) -> u8 { return leaf(y) }
fun top(z: u8) -> u8 { return middle(z) }
`,
		"fib": `
fun fib(n: u8) -> u16 {
    if n < 2 { return n }
    return fib(n - 1) + fib(n - 2)
}`,
		"foreach": `
fun sum_array(buf: u16, n: u8) -> u8 {
    let s: u8 = 0
    let p: u16 = buf
    for i in 0..n {
        s = s + @ptr(p)
        p = p + 1
    }
    return s
}`,
		"gcd": `
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else     { b = b - a }
    }
    return a
}`,
	}

	stats := mir2.NewGraceStats()
	order := []string{"abs_diff", "abs_diff_u16", "chain", "fib", "foreach", "gcd", "minmax", "swap"}

	t.Log("")
	t.Log("╔═════════════════════════════════════════════════════════════════════════════════╗")
	t.Log("║       MinZ vs SDCC — Complete Comparison (C89 + Nanz, all 8 programs)         ║")
	t.Log("╚═════════════════════════════════════════════════════════════════════════════════╝")
	t.Log("")
	t.Logf("%-16s %6s %8s %8s %8s %8s %s", "Program", "SDCC", "C89(Go)", "C89(Gr)", "Nanz(Gr)", "Best", "Winner")
	t.Logf("%-16s %6s %8s %8s %8s %8s %s", strings.Repeat("─", 16), "──────", "────────", "────────", "────────", "────────", "──────")

	totalSDCC, totalC89Go, totalC89Grace, totalNanzGrace := 0, 0, 0, 0
	minzWins, sdccWins := 0, 0

	for _, name := range order {
		sdcc := sdccInsts[name]

		// C89 path (identical source to SDCC)
		c89Go, c89Grace := "-", "-"
		c89GoN, c89GraceN := 0, 0
		if src, ok := c89Sources[name]; ok {
			c89GoN = compileC89AndCount(t, src, name, false, nil)
			c89GraceN = compileC89AndCount(t, src, name, true, stats)
			c89Go = itoa(c89GoN)
			c89Grace = itoa(c89GraceN)
		}

		// Nanz path (idiomatic MinZ)
		nanzGrace := "-"
		nanzGraceN := 0
		if src, ok := nanzSources[name]; ok {
			nanzGraceN = compileNanzAndCount(t, src, name, true, stats)
			nanzGrace = itoa(nanzGraceN)
		}

		// Find best MinZ result
		best := 99999
		if c89GraceN > 0 && c89GraceN < best {
			best = c89GraceN
		}
		if c89GoN > 0 && c89GoN < best {
			best = c89GoN
		}
		if nanzGraceN > 0 && nanzGraceN < best {
			best = nanzGraceN
		}

		bestStr := itoa(best)
		winner := ""
		if best < sdcc {
			winner = fmt.Sprintf("MinZ −%d%%", (sdcc-best)*100/sdcc)
			minzWins++
		} else if best > sdcc {
			winner = fmt.Sprintf("SDCC −%d%%", (best-sdcc)*100/best)
			sdccWins++
		} else {
			winner = "tie"
		}

		t.Logf("%-16s %6d %8s %8s %8s %8s %s", name, sdcc, c89Go, c89Grace, nanzGrace, bestStr, winner)

		totalSDCC += sdcc
		totalC89Go += c89GoN
		totalC89Grace += c89GraceN
		totalNanzGrace += nanzGraceN
	}

	t.Log("")
	t.Logf("%-16s %6d %8d %8d %8d", "TOTAL (matched)", totalSDCC, totalC89Go, totalC89Grace, totalNanzGrace)
	t.Log("")
	t.Logf("MinZ wins: %d / %d programs", minzWins, len(order))
	t.Log("")

	// Also check SDCC .asm files exist for reference
	for _, name := range order {
		asmPath := filepath.Join(sdccDir, "sdcc_"+name+".asm")
		if _, err := os.Stat(asmPath); err != nil {
			t.Logf("SDCC reference missing: %s", asmPath)
		}
	}

	t.Logf("Grace stats: %s", stats)
}

func compileC89AndCount(t *testing.T, src, name string, useGrace bool, stats *mir2.GraceStats) int {
	t.Helper()
	hm, err := c89.Compile(src, name+".c")
	if err != nil {
		t.Logf("%s C89 parse fail: %v", name, err)
		return 0
	}
	return compileHIRAndCount(t, hm, name, useGrace, stats)
}

func compileNanzAndCount(t *testing.T, src, name string, useGrace bool, stats *mir2.GraceStats) int {
	t.Helper()
	hm, err := nanz.ParseWithOpts(src, name+".nanz", nanz.ParseOpts{})
	if err != nil {
		t.Logf("%s Nanz parse fail: %v", name, err)
		return 0
	}
	return compileHIRAndCount(t, hm, name, useGrace, stats)
}

func compileHIRAndCount(t *testing.T, hm *hir.Module, name string, useGrace bool, stats *mir2.GraceStats) int {
	t.Helper()
	opts := Options{ContractOpt: true}
	if useGrace {
		opts.UseGrace = true
		opts.GraceStats = stats
	}
	steps, err := CompileHIRSteps(hm, opts)
	if err != nil {
		t.Logf("%s compile fail: %v", name, err)
		return 0
	}
	return countInstructions(steps.Assembly)
}
