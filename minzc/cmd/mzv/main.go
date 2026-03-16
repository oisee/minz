// mzv — MIR2 VM runner with TUI display.
//
// Compiles source through HIR→MIR2 (stopping before Z80 codegen),
// then executes on the MIR2 VM with host-function overrides for ZX Spectrum
// primitives. Renders the attribute screen as a 32×24 ANSI color grid.
//
// Supported frontends (by file extension):
//   .nanz  Nanz        .c/.m  C89/ObjC     .lanz  Lanz
//   .lizp  Lizp        .plm   PL/M         .pas   Pascal
//   .abap  ABAP        .hir   HIR (raw)
//
// Usage:
//
//	mzv program.nanz
//	mzv --trace program.c
//	mzv -t -H --max-frames 100 program.m
package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"database/sql"

	"github.com/minz/minzc/pkg/abap"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/plm"

	flag "github.com/spf13/pflag"
	_ "modernc.org/sqlite"
	"golang.org/x/term"
)

func main() {
	trace := flag.BoolP("trace", "t", false, "print each VM call")
	headless := flag.BoolP("headless", "H", false, "run without terminal (testing)")
	maxFrames := flag.Int("max-frames", 0, "stop after N frames (0=unlimited)")
	dumpDir := flag.String("dump-frames", "", "dump each frame as .scr file to directory")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: mzv [--trace] [--headless] [--max-frames N] <file>")
		fmt.Fprintln(os.Stderr, "  supported: .nanz .c .m .lanz .lizp .plm .pas .abap .hir")
		os.Exit(1)
	}
	srcPath := flag.Arg(0)

	src, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mzv: %v\n", err)
		os.Exit(1)
	}

	// ── Compile: source → HIR → MIR2 (optimised, no regalloc) ──────────

	hm, err := parseSource(string(src), srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mzv: parse error: %v\n", err)
		os.Exit(1)
	}

	m := hir.LowerModule(hm)

	// Per-function optimisation passes (same as pipeline.go, no regalloc).
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			s := mir2.SimplifyIdentities(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !s && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
		if mir2.BranchEquiv(m, f) {
			mir2.EliminateDeadBlocks(f)
			mir2.DeadStoreElim(f)
		}
		if mir2.CondRetSink(f) {
			mir2.EliminateDeadBlocks(f)
		}
	}

	if err := mir2.Verify(m); err != nil {
		fmt.Fprintf(os.Stderr, "mzv: MIR2 verify: %v\n", err)
		os.Exit(1)
	}

	// Phase 6f: inline trivial functions.
	if mir2.InlineTrivial(m, 4) {
		for _, f := range m.Funcs {
			mir2.PropagateCopies(f)
			mir2.DeadStoreElim(f)
		}
	}

	fmt.Fprintf(os.Stderr, "mzv: compiled %d functions, %d globals\n", len(m.Funcs), len(m.Globals))

	// ── VM setup ─────────────────────────────────────────────────────────

	vm := mir2.NewVM(m)
	vm.MaxSteps = 0 // unlimited — game loop runs forever
	vm.MaxMemory = 1 << 20

	// Canvas host functions (for any frontend using canvas_* or @canvas.*).
	mir2.RegisterCanvasHosts(vm)

	// ZX Spectrum 64K address space (separate from VM heap).
	var zxMem [65536]byte

	// Keyboard state: row high byte → composed row byte.
	var keyMu sync.Mutex
	keyState := make(map[byte]byte) // row → bits (0 = pressed)

	// 50Hz tick for HALT timing.
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	traceEnabled := *trace

	// ── Host functions ───────────────────────────────────────────────────

	vm.Hosts["zx_poke"] = func(args []mir2.Value) ([]mir2.Value, error) {
		addr := uint16(args[0].I)
		val := byte(args[1].I)
		zxMem[addr] = val
		if traceEnabled {
			fmt.Fprintf(os.Stderr, "  zx_poke(0x%04X, 0x%02X)\n", addr, val)
		}
		return nil, nil
	}

	vm.Hosts["zx_peek"] = func(args []mir2.Value) ([]mir2.Value, error) {
		addr := uint16(args[0].I)
		return []mir2.Value{{I: int64(zxMem[addr])}}, nil
	}

	vm.Hosts["zx_key_row"] = func(args []mir2.Value) ([]mir2.Value, error) {
		high := byte(args[0].I)
		keyMu.Lock()
		row := keyState[high]
		keyMu.Unlock()
		// ZX Spectrum: 0 = pressed, so default (no keys) = 0xFF
		return []mir2.Value{{I: int64(row ^ 0xFF)}}, nil
	}

	frameCount := 0
	maxF := *maxFrames

	// Create frame dump directory if requested.
	if *dumpDir != "" {
		os.MkdirAll(*dumpDir, 0755)
	}

	vm.Hosts["zx_halt"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		// Clear key state FIRST (end of previous frame's input window),
		// then wait for tick. Game reads keys AFTER halt returns, so new
		// keypresses arriving during the tick wait will be picked up.
		keyMu.Lock()
		for k := range keyState {
			delete(keyState, k)
		}
		keyMu.Unlock()
		if !*headless {
			<-ticker.C
			renderFrame(&zxMem)
		}
		frameCount++

		// Dump frame as .scr file (6912 bytes: 6144 pixels + 768 attrs).
		if *dumpDir != "" {
			scrPath := fmt.Sprintf("%s/frame_%04d.scr", *dumpDir, frameCount)
			os.WriteFile(scrPath, zxMem[0x4000:0x5B00], 0644)
		}

		if maxF > 0 && frameCount >= maxF {
			return nil, fmt.Errorf("reached %d frames, stopping", maxF)
		}
		return nil, nil
	}

	vm.Hosts["zx_border"] = func(args []mir2.Value) ([]mir2.Value, error) {
		// Cosmetic — ignore for TUI.
		return nil, nil
	}

	// zx_attr_addr is pure math — let the VM execute the Nanz body.
	// zx_screen_addr likewise.

	// ── SQLite host functions ────────────────────────────────────────────

	registerSQLiteHosts(vm, traceEnabled)

	// ── ABAP runtime host functions (SY, selection screen) ───────────────

	registerABAPHosts(vm, traceEnabled)

	// ── Terminal raw mode + input goroutine ──────────────────────────────

	var oldState *term.State
	if !*headless {
		oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "mzv: failed to set raw terminal: %v\n", err)
			os.Exit(1)
		}
		defer term.Restore(int(os.Stdin.Fd()), oldState)

		// Handle signals to restore terminal.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			term.Restore(int(os.Stdin.Fd()), oldState)
			fmt.Print("\033[?25h") // show cursor
			os.Exit(0)
		}()

		// Clear screen + hide cursor.
		fmt.Print("\033[2J\033[H\033[?25l")

		// Input reader goroutine.
		go readInput(&keyMu, keyState, oldState)
	}

	// ── Run ──────────────────────────────────────────────────────────────

	_, err = vm.Call("main", nil)
	fmt.Fprintf(os.Stderr, "mzv: exited after %d frames\n", frameCount)

	// In headless mode, render final frame to stdout + dump summary to stderr.
	if *headless {
		renderFrame(&zxMem)

		nonZero := 0
		for i := 0x5800; i < 0x5B00; i++ {
			if zxMem[i] != 0 {
				nonZero++
			}
		}
		fmt.Fprintf(os.Stderr, "mzv: attr cells with color: %d/768\n", nonZero)
	}
	if oldState != nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Print("\033[?25h")
	}
	if err != nil {
		// max-frames exit is not an error
		if maxF > 0 && frameCount >= maxF {
			fmt.Fprintf(os.Stderr, "mzv: stopped after %d frames\n", frameCount)
		} else {
			fmt.Fprintf(os.Stderr, "\nmzv: VM error: %v\n", err)
			os.Exit(1)
		}
	}
}

// ── TUI renderer ────────────────────────────────────────────────────────────

// ZX Spectrum color → ANSI color code.
// ZX: 0=black, 1=blue, 2=red, 3=magenta, 4=green, 5=cyan, 6=yellow, 7=white
// Foreground (30-37), Background (40-47), bright +60.
var zxToANSIFg = [8]int{30, 34, 31, 35, 32, 36, 33, 37}
var zxToANSIBg = [8]int{40, 44, 41, 45, 42, 46, 43, 47}

func renderFrame(zxMem *[65536]byte) {
	var buf []byte
	buf = append(buf, "\033[H"...) // cursor home

	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			attr := zxMem[0x5800+y*32+x]
			ink := attr & 7
			paper := (attr >> 3) & 7
			bright := (attr >> 6) & 1

			// Try OCR: match pixel data against ZX ROM font
			pixels := readCellPixels(zxMem, x, y)
			ch, inverse := ocrCell(pixels)

			if ch > 0x20 { // recognized printable character (not space)
				// Determine foreground/background from ink/paper + inverse
				fg, bg := ink, paper
				if inverse {
					fg, bg = paper, ink
				}
				fgCode := zxToANSIFg[fg]
				bgCode := zxToANSIBg[bg]
				if bright != 0 {
					fgCode += 60
					bgCode += 60
				}
				buf = append(buf, fmt.Sprintf("\033[%d;%dm%c \033[0m", fgCode, bgCode, ch)...)
			} else if attr == 0 && pixels == [8]byte{} {
				// Empty cell — no color, no pixels
				buf = append(buf, ' ', ' ')
			} else {
				// No text match — render as solid color block
				bgCode := zxToANSIBg[paper]
				if bright != 0 {
					bgCode += 60
				}
				if attr == 0 {
					buf = append(buf, ' ', ' ')
				} else {
					buf = append(buf, fmt.Sprintf("\033[%dm  \033[0m", bgCode)...)
				}
			}
		}
		buf = append(buf, '\r', '\n')
	}

	os.Stdout.Write(buf)
}

// ── Input ───────────────────────────────────────────────────────────────────

func readInput(mu *sync.Mutex, keyState map[byte]byte, oldState *term.State) {
	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}

		mu.Lock()
		// Only SET bits here — clearing happens at frame boundary (zx_halt).
		for i := 0; i < n; i++ {
			b := buf[i]

			// Check for escape sequences (arrow keys).
			if b == 0x1B && i+2 < n && buf[i+1] == '[' {
				switch buf[i+2] {
				case 'D': // Left arrow → same as 'o'
					keyState[0xDF] |= 0x02
				case 'C': // Right arrow → same as 'p'
					keyState[0xDF] |= 0x01
				case 'A': // Up arrow → same as 'q' (rotate)
					keyState[0xFB] |= 0x01
				case 'B': // Down arrow → same as 'a' (soft drop)
					keyState[0xFD] |= 0x01
				}
				i += 2
				continue
			}

			switch b {
			case 3: // Ctrl-C
				term.Restore(int(os.Stdin.Fd()), oldState)
				fmt.Print("\033[?25h")
				os.Exit(0)
			case 27: // Esc
				term.Restore(int(os.Stdin.Fd()), oldState)
				fmt.Print("\033[?25h")
				os.Exit(0)
			case 'o', 'O':
				keyState[0xDF] |= 0x02 // row 0xDF bit 1
			case 'p', 'P':
				keyState[0xDF] |= 0x01 // row 0xDF bit 0
			case 'q', 'Q':
				keyState[0xFB] |= 0x01 // row 0xFB bit 0
			case 'a', 'A':
				keyState[0xFD] |= 0x01 // row 0xFD bit 0
			case ' ':
				keyState[0x7F] |= 0x01 // row 0x7F bit 0
			case 'h', 'H':
				keyState[0xBF] |= 0x10 // row 0xBF bit 4
			}
		}
		mu.Unlock()
	}
}

// ── Frontend dispatch ────────────────────────────────────────────────────────

// parseSource compiles source code to an HIR module based on file extension.
func parseSource(src, srcPath string) (*hir.Module, error) {
	ext := strings.ToLower(filepath.Ext(srcPath))
	name := filepath.Base(srcPath)
	baseDir := filepath.Dir(srcPath)
	stdlibDir := findStdlib(srcPath)

	switch ext {
	case ".nanz":
		return nanz.ParseWithOpts(src, name, nanz.ParseOpts{
			BaseDir:   baseDir,
			StdlibDir: stdlibDir,
		})
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
		return pascal.Compile(src, name, pascal.CompileOpts{
			StdlibDir: stdlibDir,
		})
	case ".abap":
		return abap.Compile(src, name)
	case ".hir":
		return hir.ParseHIR(src, name)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s (supported: .nanz .c .m .lanz .lizp .plm .pas .abap .hir)", ext)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// findStdlib locates the stdlib/ directory relative to the source file.
func findStdlib(srcPath string) string {
	// Walk up from source dir looking for stdlib/
	dir, _ := filepath.Abs(filepath.Dir(srcPath))
	for {
		candidate := filepath.Join(dir, "stdlib")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// ── SQLite host functions ────────────────────────────────────────────────────
//
// Provides database access to MIR2 VM programs via host function table.
// Programs call @sqlite_open, @sqlite_exec, @sqlite_query, etc.
// Later this can be decoupled into a client-server protocol where a Z80
// program communicates with a SQLite server over I/O ports.

func registerSQLiteHosts(vm *mir2.VM, trace bool) {
	// Handle table: u16 handle → *sql.DB
	dbs := make(map[int64]*sql.DB)
	nextHandle := int64(1)

	// Prepared statement handles + cached row values
	type stmtState struct {
		rows *sql.Rows
		vals []interface{} // last scanned row values (cached per step)
	}
	stmts := make(map[int64]*stmtState)
	nextStmt := int64(1)

	// Helper: read null-terminated string from VM heap
	readStr := func(ptr int64) string {
		var buf []byte
		for i := int64(0); i < 4096; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		return string(buf)
	}

	// Helper: write null-terminated string to VM heap, return pointer
	writeStr := func(s string) mir2.Value {
		data := append([]byte(s), 0)
		return vm.AllocHeap(data)
	}

	// @sqlite_open(filename_ptr) -> handle (0 = error)
	vm.Hosts["sqlite_open"] = func(args []mir2.Value) ([]mir2.Value, error) {
		name := ":memory:"
		if len(args) > 0 && args[0].I != 0 {
			name = readStr(args[0].I)
		}
		db, err := sql.Open("sqlite", name)
		if err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  sqlite_open(%q) → error: %v\n", name, err)
			}
			return []mir2.Value{{I: 0}}, nil
		}
		// Use SetMaxIdleConns to keep connection warm
		db.SetMaxIdleConns(1)
		h := nextHandle
		dbs[h] = db
		nextHandle++
		if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_open(%q) → handle %d\n", name, h)
		}
		return []mir2.Value{{I: h}}, nil
	}

	// @sqlite_close(handle) -> rc (0=ok)
	vm.Hosts["sqlite_close"] = func(args []mir2.Value) ([]mir2.Value, error) {
		h := args[0].I
		db, ok := dbs[h]
		if !ok {
			return []mir2.Value{{I: 1}}, nil
		}
		err := db.Close()
		delete(dbs, h)
		rc := int64(0)
		if err != nil {
			rc = 1
		}
		if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_close(%d) → %d\n", h, rc)
		}
		return []mir2.Value{{I: rc}}, nil
	}

	// @sqlite_exec(handle, sql_ptr) -> rc (0=ok)
	vm.Hosts["sqlite_exec"] = func(args []mir2.Value) ([]mir2.Value, error) {
		h := args[0].I
		sqlStr := readStr(args[1].I)
		db, ok := dbs[h]
		if !ok {
			return []mir2.Value{{I: 1}}, nil
		}
		_, err := db.Exec(sqlStr)
		rc := int64(0)
		if err != nil {
			rc = 1
			if trace {
				fmt.Fprintf(os.Stderr, "  sqlite_exec(%d, %q) → error: %v\n", h, sqlStr, err)
			}
		} else if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_exec(%d, %q) → ok\n", h, sqlStr)
		}
		return []mir2.Value{{I: rc}}, nil
	}

	// @sqlite_query(handle, sql_ptr) -> stmt_handle (0 = error)
	vm.Hosts["sqlite_query"] = func(args []mir2.Value) ([]mir2.Value, error) {
		h := args[0].I
		sqlStr := readStr(args[1].I)
		db, ok := dbs[h]
		if !ok {
			return []mir2.Value{{I: 0}}, nil
		}
		rows, err := db.Query(sqlStr)
		if err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  sqlite_query(%d, %q) → error: %v\n", h, sqlStr, err)
			}
			return []mir2.Value{{I: 0}}, nil
		}
		sh := nextStmt
		stmts[sh] = &stmtState{rows: rows}
		nextStmt++
		if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_query(%d, %q) → stmt %d\n", h, sqlStr, sh)
		}
		return []mir2.Value{{I: sh}}, nil
	}

	// @sqlite_step(stmt_handle) -> has_row (1=yes, 0=done)
	// Scans the row into cached values so column_int/column_text can read them.
	vm.Hosts["sqlite_step"] = func(args []mir2.Value) ([]mir2.Value, error) {
		sh := args[0].I
		st, ok := stmts[sh]
		if !ok {
			return []mir2.Value{{I: 0}}, nil
		}
		if st.rows.Next() {
			// Scan row into cache
			cols, _ := st.rows.Columns()
			st.vals = make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range st.vals {
				ptrs[i] = &st.vals[i]
			}
			st.rows.Scan(ptrs...)
			return []mir2.Value{{I: 1}}, nil
		}
		// Done — close the rows immediately to release connection
		if err := st.rows.Close(); err != nil && trace {
			fmt.Fprintf(os.Stderr, "  sqlite_step(%d) close error: %v\n", sh, err)
		}
		delete(stmts, sh)
		if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_step(%d) → done (rows closed)\n", sh)
		}
		return []mir2.Value{{I: 0}}, nil
	}

	// @sqlite_column_int(stmt_handle, col_index) -> value
	// Reads from cached row values (populated by sqlite_step).
	vm.Hosts["sqlite_column_int"] = func(args []mir2.Value) ([]mir2.Value, error) {
		sh := args[0].I
		col := int(args[1].I)
		st, ok := stmts[sh]
		if !ok || col >= len(st.vals) {
			return []mir2.Value{{I: 0}}, nil
		}
		var result int64
		switch v := st.vals[col].(type) {
		case int64:
			result = v
		case float64:
			result = int64(v)
		case []byte:
			fmt.Sscanf(string(v), "%d", &result)
		case string:
			fmt.Sscanf(v, "%d", &result)
		}
		if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_column_int(%d, %d) → %d\n", sh, col, result)
		}
		return []mir2.Value{{I: result}}, nil
	}

	// @sqlite_column_text(stmt_handle, col_index) -> ptr (heap-allocated C string)
	// Reads from cached row values (populated by sqlite_step).
	vm.Hosts["sqlite_column_text"] = func(args []mir2.Value) ([]mir2.Value, error) {
		sh := args[0].I
		col := int(args[1].I)
		st, ok := stmts[sh]
		if !ok || col >= len(st.vals) {
			return []mir2.Value{writeStr("")}, nil
		}
		s := fmt.Sprintf("%v", st.vals[col])
		if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_column_text(%d, %d) → %q\n", sh, col, s)
		}
		return []mir2.Value{writeStr(s)}, nil
	}

	// @sqlite_finalize(stmt_handle) -> rc (0=ok)
	// Explicitly close a statement that wasn't fully iterated.
	vm.Hosts["sqlite_finalize"] = func(args []mir2.Value) ([]mir2.Value, error) {
		sh := args[0].I
		st, ok := stmts[sh]
		if !ok {
			return []mir2.Value{{I: 0}}, nil
		}
		st.rows.Close()
		delete(stmts, sh)
		if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_finalize(%d) → ok\n", sh)
		}
		return []mir2.Value{{I: 0}}, nil
	}

	fmt.Fprintf(os.Stderr, "mzv: SQLite host functions registered\n")
}

// ── ABAP runtime host functions ──────────────────────────────────────────────
//
// SY system variables + selection screen TUI.
// These are called by the ABAP lowerer's emitted code.

func registerABAPHosts(vm *mir2.VM, trace bool) {
	// ── SY system fields ─────────────────────────────────────────────────
	//
	// SY-INDEX:  current DO loop iteration (1-based)
	// SY-SUBRC:  return code from last operation (0=ok)
	// SY-TABIX:  current LOOP AT iteration
	// SY-UCOMM:  last user command (selection screen)
	// SY-DATUM:  current date (simplified: 0)
	// SY-UZEIT:  current time (simplified: 0)

	var syIndex, sySubrc, syTabix, syUcomm int64

	vm.Hosts["sy_get_index"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: syIndex}}, nil
	}
	vm.Hosts["sy_get_subrc"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: sySubrc}}, nil
	}
	vm.Hosts["sy_get_tabix"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: syTabix}}, nil
	}
	vm.Hosts["sy_get_ucomm"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: syUcomm}}, nil
	}
	vm.Hosts["sy_get_datum"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 20260316}}, nil // today
	}
	vm.Hosts["sy_get_uzeit"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 120000}}, nil // noon
	}

	// SY-INDEX setter (called internally by DO loop lowering)
	vm.Hosts["sy_set_index"] = func(args []mir2.Value) ([]mir2.Value, error) {
		syIndex = args[0].I
		return nil, nil
	}
	vm.Hosts["sy_set_subrc"] = func(args []mir2.Value) ([]mir2.Value, error) {
		sySubrc = args[0].I
		return nil, nil
	}
	vm.Hosts["sy_set_ucomm"] = func(args []mir2.Value) ([]mir2.Value, error) {
		syUcomm = args[0].I
		return nil, nil
	}

	// ── Selection screen ─────────────────────────────────────────────────

	type selField struct {
		name   string
		ty     byte // 'i'=integer, 'c'=char, 's'=string
		length int
		value  string // current value (entered by user)
	}

	var fields []*selField
	_ = syTabix // suppress unused

	// Helper: read null-terminated string from VM heap
	readStr := func(ptr int64) string {
		var buf []byte
		for i := int64(0); i < 256; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		return string(buf)
	}

	// sel_register(name_ptr, type_code, length) — register a screen field
	vm.Hosts["sel_register"] = func(args []mir2.Value) ([]mir2.Value, error) {
		name := readStr(args[0].I)
		ty := byte(args[1].I)
		length := int(args[2].I)
		fields = append(fields, &selField{name: name, ty: ty, length: length})
		if trace {
			fmt.Fprintf(os.Stderr, "  sel_register(%q, '%c', %d)\n", name, ty, length)
		}
		return nil, nil
	}

	// sel_show() — display selection screen, wait for user input
	vm.Hosts["sel_show"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		if len(fields) == 0 {
			// No fields registered — skip screen
			syUcomm = 0x4F4E // "ON" for ONLI
			return nil, nil
		}

		// Print selection screen to stderr (simple text mode)
		fmt.Fprintf(os.Stderr, "\n┌─ Selection Screen ──────────────────┐\n")
		fmt.Fprintf(os.Stderr, "│                                    │\n")
		for _, f := range fields {
			val := f.value
			if val == "" {
				val = strings.Repeat("_", f.length)
			}
			fmt.Fprintf(os.Stderr, "│  %-10s [%-20s]  │\n", f.name, val)
		}
		fmt.Fprintf(os.Stderr, "│                                    │\n")
		fmt.Fprintf(os.Stderr, "│  [Enter=Execute]                   │\n")
		fmt.Fprintf(os.Stderr, "└────────────────────────────────────┘\n\n")

		// In headless/trace mode, auto-execute with defaults
		syUcomm = 0x4F4E // "ON" for ONLI (F8 = Execute)
		if trace {
			fmt.Fprintf(os.Stderr, "  sel_show() → auto-execute (SY-UCOMM=ONLI)\n")
		}
		return nil, nil
	}

	fmt.Fprintf(os.Stderr, "mzv: ABAP runtime registered (SY + selection screen)\n")
}
