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
	"github.com/minz/minzc/pkg/frill"
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
	verbose := flag.BoolP("verbose", "v", false, "show compilation info, registered hosts, frame count")
	headless := flag.BoolP("headless", "H", false, "run without terminal (testing)")
	zxScreen := flag.Bool("zx", false, "ZX Spectrum screen mode (32x24 attribute grid to stdout)")
	maxFrames := flag.Int("max-frames", 0, "stop after N frames (0=unlimited)")
	dumpDir := flag.String("dump-frames", "", "dump each frame as .scr file to directory")
	diskImage := flag.String("disk", "", "FAT disk image file (enables @disk_read/@disk_write hosts)")
	netAddr := flag.String("net", "", "pre-connect TCP to host:port (IRC, telnet, etc.)")
	netTLS := flag.Bool("tls", false, "use TLS for --net connection")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: mzv [--trace] [--headless] [--zx] [--disk img] <file>")
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
		for iter := 0; iter < 100; iter++ {
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

	if *verbose {
		fmt.Fprintf(os.Stderr, "mzv: compiled %d functions, %d globals\n", len(m.Funcs), len(m.Globals))
	}

	// ── VM setup ─────────────────────────────────────────────────────────

	vm := mir2.NewVM(m)
	vm.MaxSteps = 0 // unlimited — game loop runs forever
	vm.MaxMemory = 1 << 20

	// Port I/O: same ports as MZE/MZX ($23=console, $30=net, $31=ctl).
	nh := newNetHost()
	defer nh.Close()
	// Stdin channel for console port reads (non-blocking).
	stdinCh := make(chan byte, 256)
	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := os.Stdin.Read(buf); err != nil {
				return
			}
			select {
			case stdinCh <- buf[0]:
			default:
			}
		}
	}()
	vmPorts := newVMPorts(nh, stdinCh, *verbose)
	vm.Ports = vmPorts
	registerPortHosts(vm, vmPorts)

	// Pre-connect network if --net flag given.
	if *netAddr != "" {
		if err := nh.preConnect(*netAddr, *netTLS); err != nil {
			fmt.Fprintf(os.Stderr, "mzv: %v\n", err)
			os.Exit(1)
		}
		if *verbose {
			fmt.Fprintf(os.Stderr, "[mzv] pre-connected to %s\n", *netAddr)
		}
	}

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

	registerABAPHosts(vm, *headless, traceEnabled)

	// ── Internal table host functions (dynamic, based on compiled module) ──
	registerItabHosts(vm, m, traceEnabled)

	// ── TUI host functions (stdlib/tui/render.nanz @extern calls) ────────

	registerTUIHosts(vm, *headless, traceEnabled)

	// ── File host functions (host filesystem access for self-hosting) ───

	registerFileHosts(vm, filepath.Dir(srcPath), traceEnabled)

	// ── Disk host functions (--disk flag) ────────────────────────────────

	if *diskImage != "" {
		registerDiskHosts(vm, *diskImage, traceEnabled)
	}

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

	// Try main() — if it expects args, pass zeros.
	var mainArgs []mir2.Value
	for _, f := range m.Funcs {
		if f.Name == "main" && len(f.Contract.Params) > 0 {
			mainArgs = make([]mir2.Value, len(f.Contract.Params))
			break
		}
	}
	_, err = vm.Call("main", mainArgs)
	if *verbose {
		fmt.Fprintf(os.Stderr, "mzv: exited after %d frames\n", frameCount)
	}

	// Render final ZX Spectrum frame only in explicit --zx mode.
	if *zxScreen {
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
	case ".frl":
		return frill.Compile(src, name)
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

	// @sqlite_query_like(handle, base_sql_ptr, col_name_ptr, filter_ptr) -> stmt
	// Appends WHERE col LIKE '%filter%' to base_sql (or no WHERE if filter is "%").
	// This enables runtime-parameterized queries from @screen filter fields.
	vm.Hosts["sqlite_query_like"] = func(args []mir2.Value) ([]mir2.Value, error) {
		h := args[0].I
		baseSQL := readStr(args[1].I)
		colName := readStr(args[2].I)
		filter := readStr(args[3].I)

		fullSQL := baseSQL
		if filter != "" && filter != "%" && filter != "*" {
			// ABAP convention: user supplies wildcards explicitly
			//   "LH"  → WHERE carrid = 'LH'  (exact match)
			//   "L%"  → WHERE carrid LIKE 'L%' (starts with)
			//   "%L%" → WHERE carrid LIKE '%L%' (contains)
			// Replace ABAP * with SQL %
			safe := strings.ReplaceAll(filter, "'", "''")
			safe = strings.ReplaceAll(safe, "*", "%")
			if strings.Contains(safe, "%") {
				fullSQL = fmt.Sprintf("%s WHERE %s LIKE '%s'", baseSQL, colName, safe)
			} else {
				fullSQL = fmt.Sprintf("%s WHERE %s = '%s'", baseSQL, colName, safe)
			}
		}

		db, ok := dbs[h]
		if !ok {
			return []mir2.Value{{I: 0}}, nil
		}
		rows, err := db.Query(fullSQL)
		if err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  sqlite_query_like(%d, %q, %q, %q) → error: %v\n", h, baseSQL, colName, filter, err)
			}
			return []mir2.Value{{I: 0}}, nil
		}
		sh := nextStmt
		stmts[sh] = &stmtState{rows: rows}
		nextStmt++
		if trace {
			fmt.Fprintf(os.Stderr, "  sqlite_query_like(%d, ..., %q, %q) → %q → stmt %d\n", h, colName, filter, fullSQL, sh)
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

	if trace {
		fmt.Fprintf(os.Stderr, "mzv: SQLite host functions registered\n")
	}
}

// ── ABAP runtime host functions ──────────────────────────────────────────────
//
// SY system variables + selection screen TUI.
// These are called by the ABAP lowerer's emitted code.

func registerABAPHosts(vm *mir2.VM, headless bool, trace bool) {
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

	// ── Selection screen (universal TUI framework) ──────────────────────
	_ = syTabix // suppress unused

	registerScreenHostsWithSY(vm, headless, trace, &syUcomm)

	// ── ABAP write functions (override inline asm with host functions) ───
	// The ABAP lowerer emits abap_write/abap_write_str with Z80 inline asm
	// (BDOS calls). The VM can't execute asm, so we override them here.

	vm.Hosts["abap_write"] = func(args []mir2.Value) ([]mir2.Value, error) {
		if len(args) > 0 {
			fmt.Printf("%d", uint8(args[0].I))
		}
		return nil, nil
	}

	vm.Hosts["abap_write_str"] = func(args []mir2.Value) ([]mir2.Value, error) {
		if len(args) > 0 {
			ptr := args[0].I
			var buf []byte
			for i := int64(0); i < 4096; i++ {
				b := vm.ReadHeap(ptr+i, 1)
				if b == nil || b[0] == 0 {
					break
				}
				buf = append(buf, b[0])
			}
			fmt.Print(string(buf))
		}
		return nil, nil
	}

	// abap_read_int — stub for VM (return 0)
	vm.Hosts["abap_read_int"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 0}}, nil
	}

	// abap_sel_read — stub for VM (keep defaults)
	vm.Hosts["abap_sel_read"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return nil, nil
	}

	// _itab_store_col(src: ^u8, dst: ^u8, maxlen: u8) — copy string to buffer slot.
	vm.Hosts["_itab_store_col"] = func(args []mir2.Value) ([]mir2.Value, error) {
		if len(args) < 3 {
			return nil, nil
		}
		srcPtr := args[0].I
		dstPtr := args[1].I
		maxlen := int(args[2].I)

		// Read source string
		var src []byte
		for i := 0; i < 256; i++ {
			b := vm.ReadHeap(srcPtr+int64(i), 1)
			if b == nil || b[0] == 0 {
				break
			}
			src = append(src, b[0])
		}

		// Write to destination, padded with zeros
		for i := 0; i < maxlen; i++ {
			if i < len(src) {
				vm.WriteHeapBytes(dstPtr+int64(i), []byte{src[i]})
			} else {
				vm.WriteHeapBytes(dstPtr+int64(i), []byte{0})
			}
		}
		vm.WriteHeapBytes(dstPtr+int64(maxlen), []byte{0}) // null terminate
		return nil, nil
	}

	// _itab_print_col(ptr: ^u8, width: u8) — print string padded to width.
	vm.Hosts["_itab_print_col"] = func(args []mir2.Value) ([]mir2.Value, error) {
		if len(args) < 2 {
			return nil, nil
		}
		ptr := args[0].I
		width := int(args[1].I)

		var buf []byte
		for i := 0; i < width; i++ {
			b := vm.ReadHeap(ptr+int64(i), 1)
			if b == nil || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		// Pad to width
		s := string(buf)
		for len(s) < width {
			s += " "
		}
		fmt.Print(s)
		return nil, nil
	}

	if trace {
		fmt.Fprintf(os.Stderr, "mzv: ABAP runtime registered (SY + selection screen + write + itab)\n")
	}
}

// itabMeta holds metadata for an internal table, used by mzv host functions.
type itabMeta struct {
	bufAddr  int64 // VM heap address of the buffer global
	cntAddr  int64 // VM heap address of the row counter
	colWidth int   // fixed column width
	rowSize  int   // total bytes per row
}

// registerItabHosts dynamically registers host functions for _itab_slot_*,
// _itab_print_*, and _abap_seed_* functions based on the compiled module.
func registerItabHosts(vm *mir2.VM, m *mir2.Module, trace bool) {

	// Discover internal tables from globals: _itab_<name>_data, _itab_<name>_cnt
	tables := make(map[string]*itabMeta) // table name → metadata
	for _, g := range m.Globals {
		if strings.HasPrefix(g.Name, "_itab_") && strings.HasSuffix(g.Name, "_data") {
			tblName := g.Name[6 : len(g.Name)-5] // strip _itab_ and _data
			if tables[tblName] == nil {
				tables[tblName] = &itabMeta{}
			}
		}
		if strings.HasPrefix(g.Name, "_itab_") && strings.HasSuffix(g.Name, "_cnt") {
			tblName := g.Name[6 : len(g.Name)-4] // strip _itab_ and _cnt
			if tables[tblName] == nil {
				tables[tblName] = &itabMeta{}
			}
		}
	}

	// Infer colWidth from function count per table
	// Default: 20 (matching ABAP lowerer)
	const defaultColWidth = 20

	count := 0
	for _, f := range m.Funcs {
		fnName := f.Name

		// _itab_slot_<table>_<row>_<col>(stmt: u16)
		if strings.HasPrefix(fnName, "_itab_slot_") {
			suffix := fnName[11:] // after "_itab_slot_"
			// Parse: <tablename>_<row>_<col> — find last two _N segments
			parts := strings.Split(suffix, "_")
			if len(parts) >= 3 {
				colIdx := 0
				fmt.Sscanf(parts[len(parts)-1], "%d", &colIdx)
				rowIdx := 0
				fmt.Sscanf(parts[len(parts)-2], "%d", &rowIdx)
				tblName := strings.Join(parts[:len(parts)-2], "_")

				capturedCol := colIdx
				capturedRow := rowIdx
				capturedTbl := tblName
				colW := defaultColWidth

				vm.Hosts[fnName] = func(args []mir2.Value) ([]mir2.Value, error) {
					if len(args) < 1 {
						return nil, nil
					}
					stmtHandle := args[0].I

					// Call sqlite_column_text via the existing host function
					colTextFn := vm.Hosts["sqlite_column_text"]
					if colTextFn == nil {
						return nil, nil
					}
					result, err := colTextFn([]mir2.Value{
						{I: stmtHandle},
						{I: int64(capturedCol)},
					})
					if err != nil || len(result) == 0 {
						return nil, err
					}

					// Read the text from the returned pointer
					srcPtr := result[0].I
					var src []byte
					for i := 0; i < 256; i++ {
						b := vm.ReadHeap(srcPtr+int64(i), 1)
						if b == nil || b[0] == 0 {
							break
						}
						src = append(src, b[0])
					}

					// Find buffer address from globals
					bufGlobal := fmt.Sprintf("_itab_%s_data", capturedTbl)
					bufAddr := vm.GlobalAddr(bufGlobal)
					if bufAddr < 0 {
						return nil, nil
					}

					// Calculate offset and write
					rowSize := colW * 4 // estimate: check actual
					// Better: count columns from module functions
					nCols := 1
					for _, ff := range m.Funcs {
						prefix := fmt.Sprintf("_itab_slot_%s_%d_", capturedTbl, capturedRow)
						if strings.HasPrefix(ff.Name, prefix) {
							colN := 0
							fmt.Sscanf(ff.Name[len(prefix):], "%d", &colN)
							if colN+1 > nCols {
								nCols = colN + 1
							}
						}
					}
					rowSize = nCols * colW
					offset := int64(capturedRow*rowSize + capturedCol*colW)

					// Write to buffer
					dstAddr := bufAddr + offset
					maxLen := colW - 1
					for i := 0; i < maxLen; i++ {
						if i < len(src) {
							vm.WriteHeapBytes(dstAddr+int64(i), []byte{src[i]})
						} else {
							vm.WriteHeapBytes(dstAddr+int64(i), []byte{0})
						}
					}
					vm.WriteHeapBytes(dstAddr+int64(maxLen), []byte{0})
					return nil, nil
				}
				count++
			}
		}

		// _itab_print_<table>_<row>_<col>()
		if strings.HasPrefix(fnName, "_itab_print_") {
			suffix := fnName[12:] // after "_itab_print_"
			parts := strings.Split(suffix, "_")
			if len(parts) >= 3 {
				colIdx := 0
				fmt.Sscanf(parts[len(parts)-1], "%d", &colIdx)
				rowIdx := 0
				fmt.Sscanf(parts[len(parts)-2], "%d", &rowIdx)
				tblName := strings.Join(parts[:len(parts)-2], "_")

				capturedCol := colIdx
				capturedRow := rowIdx
				capturedTbl := tblName
				colW := defaultColWidth

				vm.Hosts[fnName] = func(_ []mir2.Value) ([]mir2.Value, error) {
					bufGlobal := fmt.Sprintf("_itab_%s_data", capturedTbl)
					bufAddr := vm.GlobalAddr(bufGlobal)
					if bufAddr < 0 {
						return nil, nil
					}

					// Count columns for row size
					nCols := 1
					for _, ff := range m.Funcs {
						prefix := fmt.Sprintf("_itab_print_%s_%d_", capturedTbl, capturedRow)
						if strings.HasPrefix(ff.Name, prefix) {
							colN := 0
							fmt.Sscanf(ff.Name[len(prefix):], "%d", &colN)
							if colN+1 > nCols {
								nCols = colN + 1
							}
						}
					}
					rowSize := nCols * colW
					offset := int64(capturedRow*rowSize + capturedCol*colW)
					ptr := bufAddr + offset

					// Read and print padded
					var buf []byte
					for i := 0; i < colW; i++ {
						b := vm.ReadHeap(ptr+int64(i), 1)
						if b == nil || b[0] == 0 {
							break
						}
						buf = append(buf, b[0])
					}
					s := string(buf)
					for len(s) < colW {
						s += " "
					}
					fmt.Print(s)
					return nil, nil
				}
				count++
			}
		}
	}
	if trace && count > 0 {
		fmt.Fprintf(os.Stderr, "mzv: registered %d itab host functions\n", count)
	}
}
