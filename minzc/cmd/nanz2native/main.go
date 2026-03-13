// nanz2native: compile Nanz source to native AMD64 via MIR2→C99 and MIR2→QBE.
// Prints the generated C/QBE IL, compiles to native, runs, and shows disassembly.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2c"
	"github.com/minz/minzc/pkg/mir2qbe"
	"github.com/minz/minzc/pkg/nanz"
)

// Library examples — pure functions, called from a generated main wrapper.
var examples = []struct {
	name string
	src  string
	fn   string
	args string // C args for main
}{
	{
		name: "abs_diff",
		src: `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b { return a - b }
    return b - a
}`,
		fn:   "abs_diff",
		args: "10, 3",
	},
	{
		name: "fib",
		src: `
fun fib(n: u8) -> u8 {
    var a: u8 = 0
    var b: u8 = 1
    var i: u8 = 0
    while i < n {
        var t: u8 = b
        b = a + b
        a = t
        i = i + 1
    }
    return a
}`,
		fn:   "fib",
		args: "10",
	},
	{
		name: "gcd",
		src: `
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else { b = b - a }
    }
    return a
}`,
		fn:   "gcd",
		args: "48, 18",
	},
	{
		name: "clamp",
		src: `
fun clamp(x: u8, lo: u8, hi: u8) -> u8 {
    if x < lo { return lo }
    if x > hi { return hi }
    return x
}`,
		fn:   "clamp",
		args: "150, 10, 100",
	},
	{
		name: "max3",
		src: `
fun max3(a: u8, b: u8, c: u8) -> u8 {
    var m: u8 = a
    if b > m { m = b }
    if c > m { m = c }
    return m
}`,
		fn:   "max3",
		args: "42, 99, 7",
	},
}

// Standalone examples — Nanz programs with @extern that do their own I/O.
// print_num and print_char are tiny C helpers linked in.
var standalones = []struct {
	name string
	src  string
}{
	{
		name: "hello_fib",
		src: `
@extern
fun print_num(n: u8) -> void

@extern
fun print_char(c: u8) -> void

fun fib(n: u8) -> u8 {
    var a: u8 = 0
    var b: u8 = 1
    var i: u8 = 0
    while i < n {
        var t: u8 = b
        b = a + b
        a = t
        i = i + 1
    }
    return a
}

fun main() -> u8 {
    var i: u8 = 0
    while i < 13 {
        print_num(fib(i))
        print_char(32)
        i = i + 1
    }
    print_char(10)
    return 0
}
`,
	},
	{
		name: "primes",
		src: `
@extern
fun print_num(n: u8) -> void

@extern
fun print_char(c: u8) -> void

fun is_prime(n: u8) -> bool {
    if n < 2 { return false }
    var d: u8 = 2
    while d < n {
        var q: u8 = n
        while q >= d { q = q - d }
        if q == 0 { return false }
        d = d + 1
    }
    return true
}

fun main() -> u8 {
    print_char(80)
    print_char(114)
    print_char(105)
    print_char(109)
    print_char(101)
    print_char(115)
    print_char(58)
    print_char(32)
    var n: u8 = 2
    while n < 50 {
        if is_prime(n) {
            print_num(n)
            print_char(32)
        }
        n = n + 1
    }
    print_char(10)
    return 0
}
`,
	},
}

// runtimeC is a tiny C runtime providing print_num and print_char for standalone programs.
const runtimeC = `
#include <stdio.h>
void print_num(int n) { printf("%d", n); }
void print_char(int c) { putchar(c); }
`

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

func compileNanz(src, name string) (*mir2.Module, error) {
	hm, err := nanz.Parse(src, name)
	if err != nil {
		return nil, err
	}
	return hirToMIR2(hm), nil
}

func main() {
	dir, err := os.MkdirTemp("", "nanz2native")
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmpdir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	for _, ex := range examples {
		fmt.Printf("\n%s\n%s\n", strings.Repeat("=", 70), ex.name)
		fmt.Printf("%s\n", strings.Repeat("=", 70))

		m, err := compileNanz(ex.src, ex.name)
		if err != nil {
			fmt.Printf("  COMPILE ERROR: %v\n", err)
			continue
		}

		// ── Path A: MIR2 → C99 ──────────────────────────────────
		cCode, err := mir2c.Compile(m)
		if err != nil {
			fmt.Printf("  mir2c ERROR: %v\n", err)
			continue
		}
		fmt.Printf("\n── C99 output ──\n%s\n", cCode)

		// Add main wrapper — use uint8_t to match mir2c output
		params := countParams(ex.args)
		paramTypes := strings.Repeat("uint8_t, ", params)
		if len(paramTypes) > 0 {
			paramTypes = paramTypes[:len(paramTypes)-2]
		}
		mainC := fmt.Sprintf("#include <stdio.h>\n#include <stdint.h>\n%s\nint main(void) { printf(\"%%d\\n\", (int)%s(%s)); return 0; }\n",
			cCode, ex.fn, ex.args)

		cPath := filepath.Join(dir, ex.name+".c")
		binCPath := filepath.Join(dir, ex.name+"_c")
		os.WriteFile(cPath, []byte(mainC), 0644)

		out, err := exec.Command("cc", "-O0", "-o", binCPath, cPath).CombinedOutput()
		if err != nil {
			fmt.Printf("  cc ERROR: %v\n%s\n", err, out)
			continue
		}

		result, _ := exec.Command(binCPath).Output()
		fmt.Printf("── C99 result: %s(%s) = %s", ex.fn, ex.args, strings.TrimSpace(string(result)))

		// Disassemble
		disasm, _ := exec.Command("objdump", "-d", "-M", "intel",
			"--disassemble="+ex.fn, binCPath).Output()
		fmt.Printf("\n── C99 → AMD64 disassembly ──\n%s\n", filterDisasm(string(disasm)))

		// ── Path B: MIR2 → QBE IL ───────────────────────────────
		qbeIR, err := mir2qbe.Compile(m)
		if err != nil {
			fmt.Printf("  mir2qbe ERROR: %v\n", err)
			continue
		}

		// Build a pure-QBE main that calls printf — no C wrapper needed!
		qbeArgs := buildQBEArgs(ex.args)
		qbeMain := fmt.Sprintf(`
# --- main: call %s and printf the result ---
data $fmt = { b "%%d\n", b 0 }

export function w $main() {
@start
	%%result =w call $%s(%s)
	%%r =w call $printf(l $fmt, ..., w %%result)
	ret 0
}
`, ex.fn, ex.fn, qbeArgs)
		fullQBE := qbeIR + qbeMain
		fmt.Printf("── QBE IL (full, with main) ──\n%s\n", fullQBE)

		ssaPath := filepath.Join(dir, ex.name+".ssa")
		asmPath := filepath.Join(dir, ex.name+".s")
		os.WriteFile(ssaPath, []byte(fullQBE), 0644)

		out, err = exec.Command("qbe", "-o", asmPath, ssaPath).CombinedOutput()
		if err != nil {
			fmt.Printf("  qbe ERROR: %v\n%s\n", err, out)
			continue
		}

		// Show QBE's assembly output
		asmBytes, _ := os.ReadFile(asmPath)
		fmt.Printf("── QBE → AMD64 assembly ──\n%s\n", string(asmBytes))

		binQPath := filepath.Join(dir, ex.name+"_q")
		out, err = exec.Command("cc", "-O0", "-o", binQPath, asmPath).CombinedOutput()
		if err != nil {
			fmt.Printf("  cc(qbe) ERROR: %v\n%s\n", err, out)
			continue
		}

		resultQ, _ := exec.Command(binQPath).Output()
		fmt.Printf("── QBE result: %s(%s) = %s\n", ex.fn, ex.args, strings.TrimSpace(string(resultQ)))
	}

	// ── Standalone programs (Nanz with @extern → native binary) ──────────────
	fmt.Printf("\n%s\n", strings.Repeat("*", 70))
	fmt.Printf("  STANDALONE PROGRAMS — Nanz with @extern → native binary\n")
	fmt.Printf("%s\n", strings.Repeat("*", 70))

	// Write the tiny C runtime once
	rtPath := filepath.Join(dir, "runtime.c")
	os.WriteFile(rtPath, []byte(runtimeC), 0644)

	for _, sa := range standalones {
		fmt.Printf("\n%s\n%s\n", strings.Repeat("─", 70), sa.name)
		fmt.Printf("%s\n", strings.Repeat("─", 70))

		fmt.Printf("── Nanz source ──\n%s\n", sa.src)

		m, err := compileNanz(sa.src, sa.name)
		if err != nil {
			fmt.Printf("  COMPILE ERROR: %v\n", err)
			continue
		}

		// Path A: MIR2 → C99 → cc (with runtime)
		cCode, err := mir2c.Compile(m)
		if err != nil {
			fmt.Printf("  mir2c ERROR: %v\n", err)
			continue
		}
		fmt.Printf("── C99 output ──\n%s\n", cCode)

		cPath := filepath.Join(dir, sa.name+".c")
		binCPath := filepath.Join(dir, sa.name+"_c")
		os.WriteFile(cPath, []byte(cCode), 0644)

		out, err := exec.Command("cc", "-O0", "-o", binCPath, cPath, rtPath).CombinedOutput()
		if err != nil {
			fmt.Printf("  cc ERROR: %v\n%s\n", err, out)
		} else {
			result, _ := exec.Command(binCPath).Output()
			fmt.Printf("── C99 run ──\n%s\n", string(result))
		}

		// Path B: MIR2 → QBE IL → qbe → cc (with runtime)
		qbeIR, err := mir2qbe.Compile(m)
		if err != nil {
			fmt.Printf("  mir2qbe ERROR: %v\n", err)
			continue
		}
		fmt.Printf("── QBE IL output ──\n%s\n", qbeIR)

		ssaPath := filepath.Join(dir, sa.name+".ssa")
		asmPath := filepath.Join(dir, sa.name+".s")
		os.WriteFile(ssaPath, []byte(qbeIR), 0644)

		out, err = exec.Command("qbe", "-o", asmPath, ssaPath).CombinedOutput()
		if err != nil {
			fmt.Printf("  qbe ERROR: %v\n%s\n", err, out)
			continue
		}

		asmBytes, _ := os.ReadFile(asmPath)
		fmt.Printf("── QBE → AMD64 assembly ──\n%s\n", string(asmBytes))

		binQPath := filepath.Join(dir, sa.name+"_q")
		out, err = exec.Command("cc", "-O0", "-o", binQPath, asmPath, rtPath).CombinedOutput()
		if err != nil {
			fmt.Printf("  cc(qbe) ERROR: %v\n%s\n", err, out)
			continue
		}

		resultQ, _ := exec.Command(binQPath).Output()
		fmt.Printf("── QBE run ──\n%s\n", string(resultQ))
	}
}

// buildQBEArgs turns "10, 3" into "w 10, w 3" for QBE call syntax.
func buildQBEArgs(args string) string {
	parts := strings.Split(args, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, "w "+p)
		}
	}
	return strings.Join(out, ", ")
}

func countParams(args string) int {
	if args == "" {
		return 0
	}
	return strings.Count(args, ",") + 1
}

func filterDisasm(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	inFunc := false
	for _, l := range lines {
		if strings.Contains(l, ">:") {
			inFunc = true
		}
		if inFunc {
			out = append(out, l)
			if strings.TrimSpace(l) == "" && inFunc && len(out) > 2 {
				break
			}
		}
	}
	return strings.Join(out, "\n")
}
