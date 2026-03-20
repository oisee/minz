// mzn: compile source to native binary via MIR2→C99 and MIR2→QBE.
// Targets the host platform (whatever qbe + cc produce).
//
// Supported frontends (by file extension):
//   .nanz  Nanz        .c/.m  C89/ObjC     .lanz  Lanz
//   .lizp  Lizp        .plm   PL/M         .pas   Pascal
//   .abap  ABAP        .hir   HIR (raw)
//
// Usage:
//
//	mzn file.nanz                    # compile via QBE, run, show IL + asm
//	mzn -o hello file.nanz           # compile via QBE → ./hello binary
//	mzn --c99 file.c                 # C99 backend only
//	mzn --c99 --qbe file.m           # both backends
//	mzn --emit-c file.lizp           # print generated C99, don't compile
//	mzn --emit-qbe file.lanz         # print generated QBE IL, don't compile
//	mzn                              # run built-in demos
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/abap"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2c"
	"github.com/minz/minzc/pkg/mir2qbe"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/plm"

	flag "github.com/spf13/pflag"
)

var (
	flagC       = flag.Bool("c99", false, "C99 backend only")
	flagQ       = flag.Bool("qbe", false, "QBE backend only")
	flagEmitC   = flag.Bool("emit-c", false, "print generated C99 and exit")
	flagEmitQBE = flag.Bool("emit-qbe", false, "print generated QBE IL and exit")
	flagOutput  = flag.StringP("output", "o", "", "output binary path")
	flagRun     = flag.Bool("run", true, "compile and run (default true)")
	flagDisasm  = flag.BoolP("disasm", "d", false, "show disassembly")
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

// runtimeC is a tiny C runtime providing print_num, print_char, and canvas_*
// stubs for standalone programs compiled to native.
const runtimeC = `
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
void print_num(int n) { printf("%d", n); }
void print_char(int c) { putchar(c); }

/* ── Minimal canvas runtime (256-color indexed, PNG via ppm→convert) ── */
static int    _cw, _ch;
static unsigned char *_cpx;       /* w*h palette indices */
static unsigned char _cpal[256][3]; /* RGB palette */

void canvas_init(int w, int h, int mode) {
    /* mode 0=256x192, 1=320x200, 2=320x240, 3=640x480, 4+=custom */
    static const int presets[][3] = {{256,192},{320,200},{320,240},{640,480}};
    if (mode >= 0 && mode <= 3) { w = presets[mode][0]; h = presets[mode][1]; }
    _cw = w; _ch = h;
    _cpx = calloc(w * h, 1);
    /* default gray ramp */
    for (int i = 0; i < 256; i++) { _cpal[i][0] = _cpal[i][1] = _cpal[i][2] = i; }
}
void canvas_clear(int c)             { if (_cpx) memset(_cpx, c, _cw*_ch); }
void canvas_pixel(int x, int y, int c) { if (_cpx && x>=0 && x<_cw && y>=0 && y<_ch) _cpx[y*_cw+x]=(unsigned char)c; }
int  canvas_width(void)              { return _cw; }
int  canvas_height(void)             { return _ch; }
void canvas_palette(int i, int r, int g, int b) { _cpal[i&255][0]=r; _cpal[i&255][1]=g; _cpal[i&255][2]=b; }
int  canvas_get_pixel(int x, int y)  { return (_cpx && x>=0 && x<_cw && y>=0 && y<_ch) ? _cpx[y*_cw+x] : 0; }

void canvas_line(int x0, int y0, int x1, int y1, int c) {
    int dx = abs(x1-x0), dy = abs(y1-y0);
    int sx = x0<x1?1:-1, sy = y0<y1?1:-1, err = dx-dy;
    for (;;) {
        canvas_pixel(x0, y0, c);
        if (x0==x1 && y0==y1) break;
        int e2 = err*2;
        if (e2 > -dy) { err -= dy; x0 += sx; }
        if (e2 <  dx) { err += dx; y0 += sy; }
    }
}
void canvas_rect(int x, int y, int w, int h, int c) {
    canvas_line(x,y,x+w-1,y,c); canvas_line(x+w-1,y,x+w-1,y+h-1,c);
    canvas_line(x+w-1,y+h-1,x,y+h-1,c); canvas_line(x,y+h-1,x,y,c);
}
void canvas_fill_rect(int x, int y, int w, int h, int c) {
    for (int r=y; r<y+h; r++) for (int col=x; col<x+w; col++) canvas_pixel(col,r,c);
}
void canvas_circle(int cx, int cy, int r, int c) {
    int x=r, y=0, d=1-r;
    while (x>=y) {
        canvas_pixel(cx+x,cy+y,c); canvas_pixel(cx-x,cy+y,c);
        canvas_pixel(cx+x,cy-y,c); canvas_pixel(cx-x,cy-y,c);
        canvas_pixel(cx+y,cy+x,c); canvas_pixel(cx-y,cy+x,c);
        canvas_pixel(cx+y,cy-x,c); canvas_pixel(cx-y,cy-x,c);
        y++;
        if (d<0) d+=2*y+1; else { x--; d+=2*(y-x)+1; }
    }
}

/* Save as PPM (universal, no deps). Convert to PNG: ppmtopng out.ppm > out.png */
int canvas_save_ppm(const char *path) {
    if (!_cpx) return -1;
    FILE *f = fopen(path, "wb");
    if (!f) return -1;
    fprintf(f, "P6\n%d %d\n255\n", _cw, _ch);
    for (int i = 0; i < _cw*_ch; i++) {
        unsigned char *p = _cpal[_cpx[i]];
        fwrite(p, 1, 3, f);
    }
    fclose(f);
    return 0;
}
`

func hirToMIR2(hm *hir.Module) *mir2.Module {
	m := hir.LowerModule(hm)
	mir2.LUTGen(m)
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for iter := 0; iter < 100; iter++ {
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

// parseSource compiles source code to an HIR module based on file extension.
func parseSource(src, srcPath string) (*hir.Module, error) {
	ext := strings.ToLower(filepath.Ext(srcPath))
	name := filepath.Base(srcPath)

	switch ext {
	case ".nanz":
		return nanz.Parse(src, name)
	case ".c", ".m":
		absPath, _ := filepath.Abs(srcPath)
		return c89.CompileWithOpts(src, name, c89.CompileOpts{
			BaseDir:      filepath.Dir(absPath),
			IncludePaths: []string{filepath.Dir(absPath)},
		})
	case ".lanz":
		return lanz.Compile(src, name)
	case ".lizp":
		return lizp.Compile(src, name)
	case ".plm":
		return plm.Compile(src)
	case ".pas":
		return pascal.Compile(src, name, pascal.CompileOpts{})
	case ".abap":
		return abap.Compile(src, name)
	case ".hir":
		return hir.ParseHIR(src, name)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s (supported: .nanz .c .m .lanz .lizp .plm .pas .abap .hir)", ext)
	}
}

func compileSource(src, srcPath string) (*mir2.Module, error) {
	hm, err := parseSource(src, srcPath)
	if err != nil {
		return nil, err
	}
	return hirToMIR2(hm), nil
}

// compileNanz kept for built-in demos (inline source, no file extension).
func compileNanz(src, name string) (*mir2.Module, error) {
	hm, err := nanz.Parse(src, name)
	if err != nil {
		return nil, err
	}
	return hirToMIR2(hm), nil
}

func main() {
	flag.Parse()

	if flag.NArg() > 0 {
		compileFile(flag.Arg(0))
		return
	}

	// No file argument — run built-in demos
	runDemos()
}

// compileFile compiles a source file to native via C99 and/or QBE backends.
func compileFile(filename string) {
	srcBytes, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}
	src := string(srcBytes)
	name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	m, err := compileSource(src, filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compile: %v\n", err)
		os.Exit(1)
	}

	dir, err := os.MkdirTemp("", "mzn")
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmpdir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	// Write C runtime (print_num, print_char, canvas_* stubs).
	rtPath := filepath.Join(dir, "runtime.c")
	os.WriteFile(rtPath, []byte(runtimeC), 0644)

	// If no main(), generate a C harness that calls test wrappers and saves canvas.
	var harnessPath string
	if !moduleHasMain(m) {
		harness := generateMainHarness(m, name)
		if harness != "" {
			harnessPath = filepath.Join(dir, "harness.c")
			os.WriteFile(harnessPath, []byte(harness), 0644)
			fmt.Fprintf(os.Stderr, "mzn: no main() found, generating test harness\n")
		}
	}

	// Emit-only modes: print and exit immediately
	if *flagEmitC {
		cCode, err := mir2c.Compile(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mir2c: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(cCode)
		return
	}
	if *flagEmitQBE {
		qbeIR, err := mir2qbe.Compile(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mir2qbe: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(qbeIR)
		return
	}

	// Default: QBE only. Use -c for C99, -c -q for both.
	doC := *flagC
	doQBE := *flagQ || !*flagC
	quiet := *flagOutput != "" // -o mode: build silently

	// ── C99 backend ──
	if doC {
		cCode, err := mir2c.Compile(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mir2c: %v\n", err)
		} else {
			if !quiet {
				fmt.Printf("── C99 output ──\n%s\n", cCode)
			}
			cPath := filepath.Join(dir, name+".c")
			binPath := filepath.Join(dir, name+"_c")
			if quiet && !doQBE {
				binPath = *flagOutput
			}
			os.WriteFile(cPath, []byte(cCode), 0644)

			ccArgs := []string{"-O0", "-o", binPath, cPath, rtPath}
			if harnessPath != "" {
				ccArgs = append(ccArgs, harnessPath)
			}
			out, err := exec.Command("cc", ccArgs...).CombinedOutput()
			if err != nil {
				fmt.Fprintf(os.Stderr, "cc: %v\n%s\n", err, out)
			} else {
				if *flagDisasm {
					disasm, _ := exec.Command("objdump", "-d", "-M", "intel", binPath).Output()
					fmt.Printf("── C99 → AMD64 disassembly ──\n%s\n", filterDisasm(string(disasm)))
				}
				if *flagRun && !quiet {
					result, _ := exec.Command(binPath).CombinedOutput()
					fmt.Printf("── C99 run ──\n%s", string(result))
				}
			}
		}
	}

	// ── QBE backend ──
	if doQBE {
		qbeIR, err := mir2qbe.Compile(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mir2qbe: %v\n", err)
		} else {
			if !quiet {
				fmt.Printf("── QBE IL output ──\n%s\n", qbeIR)
			}

			ssaPath := filepath.Join(dir, name+".ssa")
			asmPath := filepath.Join(dir, name+".s")
			os.WriteFile(ssaPath, []byte(qbeIR), 0644)

			out, err := exec.Command("qbe", "-o", asmPath, ssaPath).CombinedOutput()
			if err != nil {
				fmt.Fprintf(os.Stderr, "qbe: %v\n%s\n", err, out)
			} else {
				if !quiet {
					asmBytes, _ := os.ReadFile(asmPath)
					fmt.Printf("── QBE → AMD64 assembly ──\n%s\n", string(asmBytes))
				}

				binPath := filepath.Join(dir, name+"_q")
				if quiet {
					binPath = *flagOutput
				}
				ccArgs := []string{"-O0", "-o", binPath, asmPath, rtPath}
				if harnessPath != "" {
					ccArgs = append(ccArgs, harnessPath)
				}
				out, err = exec.Command("cc", ccArgs...).CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "cc(qbe): %v\n%s\n", err, out)
				} else {
					if *flagRun && !quiet {
						result, _ := exec.Command(binPath).CombinedOutput()
						fmt.Printf("── QBE run ──\n%s", string(result))
					}
				}
			}
		}
	}
}

func runDemos() {
	dir, err := os.MkdirTemp("", "mzn")
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

// moduleHasMain returns true if the MIR2 module contains a "main" function.
func moduleHasMain(m *mir2.Module) bool {
	for _, f := range m.Funcs {
		if f.Name == "main" {
			return true
		}
	}
	return false
}

// generateMainHarness creates a C main() that calls test wrappers and saves canvas.
// Returns empty string if no suitable entry points are found.
func generateMainHarness(m *mir2.Module, name string) string {
	var testFns []string
	hasCanvas := false
	for _, f := range m.Funcs {
		// Only static tests — dynamic dispatch vtables aren't initialized for native.
		if strings.HasPrefix(f.Name, "__objc_test_") {
			testFns = append(testFns, f.Name)
		}
		if strings.HasPrefix(f.Name, "setup_plasma_palette") ||
			strings.HasPrefix(f.Name, "Plasma_render") ||
			strings.HasPrefix(f.Name, "Diamond_render") ||
			strings.HasPrefix(f.Name, "XorPattern_render") {
			hasCanvas = true
		}
	}
	if len(testFns) == 0 && !hasCanvas {
		return ""
	}

	var b strings.Builder
	b.WriteString("#include <stdio.h>\n#include <stdint.h>\n\n")

	// Declare all test functions
	for _, fn := range testFns {
		b.WriteString(fmt.Sprintf("int16_t %s(void);\n", fn))
	}

	// Declare canvas functions if needed
	if hasCanvas {
		b.WriteString("void canvas_init(int,int,int);\nvoid setup_plasma_palette(void);\n")
		b.WriteString("int canvas_save_ppm(const char*);\n")
	}

	b.WriteString("\nint main(void) {\n")

	// Init canvas if needed
	if hasCanvas {
		b.WriteString("    canvas_init(256, 192, 0);\n")
		b.WriteString("    setup_plasma_palette();\n")
	}

	// Run test functions
	for _, fn := range testFns {
		b.WriteString(fmt.Sprintf("    printf(\"%s = %%d\\n\", (int)%s());\n", fn, fn))
	}

	if hasCanvas {
		outFile := name + ".ppm"
		b.WriteString(fmt.Sprintf("    canvas_save_ppm(\"%s\");\n", outFile))
		b.WriteString(fmt.Sprintf("    printf(\"canvas saved to %s\\n\");\n", outFile))
	}

	b.WriteString("    return 0;\n}\n")
	return b.String()
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
