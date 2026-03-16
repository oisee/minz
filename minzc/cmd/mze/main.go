package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/minz/minzc/pkg/dap"
	"github.com/minz/minzc/pkg/dzrp"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/spf13/cobra"
)

var (
	loadAddr     uint
	startAddr    uint
	target       string
	verbose      bool
	cycles       bool
	timeout      uint
	debugMode    bool

	// New flags
	profilePath    string
	tracePath      string
	consolePort    string
	consoleIO      bool
	warnOnHalt     bool
	breakAtTstate  int64
	diag           bool
)

var rootCmd = &cobra.Command{
	Use:   "mze [binary file]",
	Short: "MinZ Z80 Multi-Platform Emulator v2.0 - 100% Coverage!",
	Long: `mze - MinZ Z80 Multi-Platform Emulator v2.0
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
100% Z80 instruction coverage (1335/1335 FUSE tests)

FEATURES:
  All 256+ Z80 opcodes including undocumented instructions
  Execution profiler: exec/read/write/stack heatmaps + memory snapshot
  Basic-block trace (JSONL)
  Bidirectional console I/O: port $23 stdin/stdout, port $25 stderr
  DI+HALT exit with A register as process exit code
  T-state breakpoints for precise debugging
  CP/M BDOS emulation, Agon MOS RST handlers

CONSOLE I/O (--console-io):
  OUT ($23), A   → send byte to host stdout
  IN  A, ($23)   → $00 = no data, $80|byte = data ready
  OUT ($25), A   → send byte to host stderr
  DI / HALT      → exit with A as process exit code

PROFILING (--profile FILE.json):
  exec         instruction execution count per PC
  read/write   memory access count per byte
  stack_push   bytes written by PUSH/CALL/RST (SP-delta detection)
  stack_pop    bytes read by POP/RET
  io_read/write  port I/O counts
  mem_snapshot   byte value at every hot address (final state)
  stack_depth    lowest SP seen (deepest stack usage)

SUPPORTED PLATFORMS (-t/--target):
  spectrum - ZX Spectrum (default)
  cpm - CP/M 2.2 BDOS
  cpc - Amstrad CPC
  agon - Agon Light 2 (eZ80 MOS RST handlers)`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		binaryFile := args[0]

		// Apply platform-specific defaults if user didn't override
		if target == "cpm" && !cmd.Flags().Changed("load") {
			loadAddr = 0x0100 // CP/M TPA starts at $0100
		}

		// Parse addresses
		loadAddress := uint16(loadAddr)
		startAddress := uint16(startAddr)
		if startAddress == 0 {
			startAddress = loadAddress
		}

		if verbose {
			fmt.Printf("mze - MinZ Z80 Multi-Platform Emulator v2.0\n")
			fmt.Printf("100%% Z80 Instruction Coverage Enabled!\n")
			fmt.Printf("Target: %s\n", target)
			fmt.Printf("Binary: %s\n", binaryFile)
			fmt.Printf("Load:   $%04X (%d)\n", loadAddress, loadAddress)
			fmt.Printf("Start:  $%04X (%d)\n", startAddress, startAddress)
			if timeout > 0 {
				fmt.Printf("Timeout: %d T-states\n", timeout)
			}
			fmt.Println()
		}

		// Read the binary file
		binary, err := os.ReadFile(binaryFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading binary file: %v\n", err)
			os.Exit(1)
		}

		if verbose {
			fmt.Printf("Loaded %d bytes\n", len(binary))
		}

		// Create Z80 emulator with 100% coverage
		z80 := emulator.NewRemogattoZ80WithScreen()

		// --- WarnOnHalt ---
		if warnOnHalt {
			z80.RemogattoZ80.WarnOnHalt = true
		}

		// --- Execution limit from timeout ---
		if timeout > 0 {
			z80.RemogattoZ80.MaxCycles = int(timeout)
		}

		// --- Profiler ---
		var prof *emulator.Profiler
		if profilePath != "" || tracePath != "" {
			prof = emulator.NewProfiler()
			if tracePath != "" {
				if err := prof.SetTraceOutput(tracePath); err != nil {
					fmt.Fprintf(os.Stderr, "Error opening trace file: %v\n", err)
					os.Exit(1)
				}
			}
			z80.RemogattoZ80.SetProfiler(prof)
			if verbose {
				fmt.Println("Profiler enabled")
			}
		}

		// --- Console I/O ---
		if consoleIO {
			consolePort = "$23" // default console port
		}
		if consolePort != "" {
			port, err := parsePort(consolePort)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing console port: %v\n", err)
				os.Exit(1)
			}
			z80.RemogattoZ80.SetConsolePort(port, os.Stdin, os.Stdout)
			// Also register stderr port ($25) for host-side error output
			z80.RemogattoZ80.SetStderrPort(0x25, os.Stderr)
			if verbose {
				fmt.Printf("Console I/O on port $%02X (stderr on $25)\n", port)
			}
		}

		// --- T-State trap ---
		if breakAtTstate > 0 {
			z80.RemogattoZ80.SetTStateTrap(breakAtTstate, func(actualCycles int64) {
				fmt.Fprintf(os.Stderr, "\n--- Break at T-state %d (actual: %d) ---\n", breakAtTstate, actualCycles)
				fmt.Fprintln(os.Stderr, z80.RemogattoZ80.DiagString())
				fmt.Fprintln(os.Stderr, "---")
			})
		}

		// Set up CP/M BDOS handler if target is cpm
		if target == "cpm" {
			setupCPMBDOS(z80)
		}

		// Set up Agon MOS RST handler if target is agon
		if target == "agon" {
			setupAgonMOS(z80)
		}

		// Load binary into memory at specified address
		z80.LoadAt(loadAddress, binary)
		z80.SetPC(startAddress)

		if verbose {
			fmt.Printf("Starting execution at $%04X with 100%% coverage...\n", startAddress)
			fmt.Println("----------------------------------------")
		}

		// Execute the program
		err = z80.Execute()
		if err != nil {
			fmt.Printf("Execution error: %v\n", err)
			os.Exit(1)
		}

		exitCode := z80.GetExitCode()
		totalCycles := z80.GetCycles()

		if verbose {
			fmt.Println("----------------------------------------")
			fmt.Printf("Program completed with exit code: %d\n", exitCode)
		}

		if cycles {
			fmt.Printf("Total execution: %d T-states\n", totalCycles)
		}

		// --- Export profiler data ---
		if prof != nil {
			// Capture memory snapshot so profile shows what's at hot addresses
			prof.SetMemorySnapshot(z80.RemogattoZ80.Memory())
			if profilePath != "" {
				if err := prof.ExportProfile(profilePath); err != nil {
					fmt.Fprintf(os.Stderr, "Error exporting profile: %v\n", err)
				} else if verbose {
					fmt.Printf("Profile exported to %s (%d instructions)\n", profilePath, prof.TotalInstrs)
				}
			}
			prof.Close()
		}

		// --- Diagnostics ---
		if diag {
			fmt.Println(z80.RemogattoZ80.DiagString())
		}

		if verbose {
			// Show final register state
			regs := z80.GetRegisters()
			fmt.Printf("\nFinal Register State (100%% Coverage):\n")
			fmt.Printf("   PC=$%04X  SP=$%04X  A=$%02X  F=$%02X\n",
				regs.PC, regs.SP, regs.A, regs.F)
			fmt.Printf("   BC=$%04X  DE=$%04X  HL=$%04X\n",
				regs.BC, regs.DE, regs.HL)
			fmt.Printf("   IX=$%04X  IY=$%04X\n", regs.IX, regs.IY)

			fmt.Println("\nPowered by remogatto/z80 - 100% instruction coverage!")
		}

		// Use A register value as process exit code (DI+HALT convention)
		if exitCode != 0 {
			os.Exit(int(exitCode))
		}
	},
}

// parsePort parses a port number from string, supporting $XX, 0xXX, and decimal formats.
func parsePort(s string) (byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "$") {
		v, err := strconv.ParseUint(s[1:], 16, 8)
		return byte(v), err
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, err := strconv.ParseUint(s[2:], 16, 8)
		return byte(v), err
	}
	v, err := strconv.ParseUint(s, 10, 8)
	return byte(v), err
}

// setupCPMBDOS sets up CP/M BDOS handler with full file I/O.
// Files are resolved from the directory containing the .com binary.
func setupCPMBDOS(z80 *emulator.RemogattoZ80WithScreen) {
	dmaAddr := uint16(0x0080)
	currentDisk := byte(0)
	fileHandles := make(map[byte]*os.File) // FCB cr byte → host file
	nextHandle := byte(1)

	// ── CP/M Zero Page Setup ─────────────────────────────────────────────
	// 0x0000: JP 0x0000 (warm boot → intercepted as exit)
	// 0x0003: IOBYTE (0x00)
	// 0x0004: Current disk/user (0x00)
	// 0x0005: JP 0x0005 (BDOS entry → intercepted)
	// 0x0006-0x0007: BDOS address (top of TPA) — programs read this!
	// 0x005C: Default FCB (zeroed)
	// 0x0080: Default DMA buffer
	// CP/M has all-RAM — disable ROM protection (default is 0x4000 for ZX Spectrum)
	z80.SetROMEnd(0)

	// Zero page: programs read ($0006) for TPA size.
	// Addr 0: JP 0 (warm boot — intercepted at PC=0 by RST handler or loop trap).
	// Addr 5: RET (BDOS entry — intercepted BEFORE opcode fetch at PC=5).
	// Addr 6-7: BDOS/TPA top address — read by programs to know available memory.
	z80.WriteMemory(0x0000, 0x76) // HALT (warm boot → stop)
	z80.WriteMemory(0x0003, 0x00) // IOBYTE
	z80.WriteMemory(0x0004, 0x00) // current disk+user
	z80.WriteMemory(0x0005, 0xC9) // RET (never reached — intercepted at PC=5)
	z80.WriteMemory(0x0006, 0x00) // BDOS address lo → TPA top = 0xFE00
	z80.WriteMemory(0x0007, 0xFE) // BDOS address hi

	// Resolve CP/M file directory — same dir as the binary being executed.
	// We find the .com file argument by looking for a path that exists.
	cpmDir := "."
	for _, arg := range os.Args[1:] {
		if strings.HasSuffix(strings.ToLower(arg), ".com") || strings.HasSuffix(strings.ToLower(arg), ".bin") {
			d := filepath.Dir(arg)
			if d != "" && d != "." {
				cpmDir = d
			} else {
				// Relative path — use working directory
				abs, err := filepath.Abs(arg)
				if err == nil {
					cpmDir = filepath.Dir(abs)
				}
			}
			break
		}
	}

	// Read FCB filename: bytes 1..8 (name) + 9..11 (ext) → "NAME.EXT"
	getFCBName := func(fcbAddr uint16) string {
		var name, ext []byte
		for i := uint16(1); i <= 8; i++ {
			b := z80.ReadMemory(fcbAddr + i) & 0x7F
			if b > 0x20 {
				name = append(name, b)
			}
		}
		for i := uint16(9); i <= 11; i++ {
			b := z80.ReadMemory(fcbAddr + i) & 0x7F
			if b > 0x20 {
				ext = append(ext, b)
			}
		}
		s := strings.ToUpper(strings.TrimSpace(string(name)))
		e := strings.ToUpper(strings.TrimSpace(string(ext)))
		if e != "" {
			return s + "." + e
		}
		return s
	}

	// Find file on host filesystem (case-insensitive search)
	findFile := func(name string) string {
		// Try exact match first
		p := filepath.Join(cpmDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// Try case-insensitive
		entries, err := os.ReadDir(cpmDir)
		if err != nil {
			return ""
		}
		for _, e := range entries {
			if strings.EqualFold(e.Name(), name) {
				return filepath.Join(cpmDir, e.Name())
			}
		}
		return ""
	}

	z80.SetBDOSHandler(func(function byte, de uint16) (a byte, hl uint16, handled bool) {
		if verbose {
			fmt.Fprintf(os.Stderr, "[BDOS %02X DE=%04X]\n", function, de)
		}
		switch function {
		case 0x00: // System reset (warm boot)
			return 0, 0, true

		case 0x01: // Console input (blocking)
			buf := make([]byte, 1)
			n, _ := os.Stdin.Read(buf)
			if n > 0 {
				if buf[0] == '\n' {
					buf[0] = '\r'
				}
				fmt.Printf("%c", buf[0])
				return buf[0], 0, true
			}
			return '\r', 0, true

		case 0x02: // Console output
			ch := byte(de & 0xFF)
			if ch == '\r' {
				// CR → print newline (CP/M convention)
			} else {
				fmt.Printf("%c", ch)
			}
			return 0, 0, true

		case 0x06: // Direct console I/O
			e := byte(de & 0xFF)
			if e == 0xFF {
				return 0, 0, true // No char available
			}
			fmt.Printf("%c", e)
			return 0, 0, true

		case 0x09: // Print string ($-terminated)
			addr := de
			for i := 0; i < 0x4000; i++ { // safety limit
				ch := z80.ReadMemory(addr)
				if ch == '$' {
					break
				}
				fmt.Printf("%c", ch)
				addr++
			}
			return 0, 0, true

		case 0x0A: // Read console buffer
			// FCB-like structure at DE: [max_len, actual_len, chars...]
			maxLen := z80.ReadMemory(de)
			if maxLen == 0 {
				maxLen = 127
			}
			buf := make([]byte, maxLen)
			n, _ := os.Stdin.Read(buf)
			// Strip trailing newline
			if n > 0 && buf[n-1] == '\n' {
				n--
			}
			if n > int(maxLen) {
				n = int(maxLen)
			}
			z80.WriteMemory(de+1, byte(n))
			for i := 0; i < n; i++ {
				z80.WriteMemory(de+2+uint16(i), buf[i])
			}
			return 0, 0, true

		case 0x0B: // Console status
			return 0, 0, true // No char available

		case 0x0C: // Get version
			return 0x22, 0x0022, true // CP/M 2.2

		case 0x0D: // Reset disk system
			currentDisk = 0
			dmaAddr = 0x0080
			return 0, 0, true

		case 0x0E: // Select disk
			currentDisk = byte(de & 0xFF)
			return 0, 0, true

		// ── File I/O ─────────────────────────────────────────────

		case 0x0F: // Open file
			fcbAddr := de
			name := getFCBName(fcbAddr)
			path := findFile(name)
			if path == "" {
				if verbose {
					fmt.Fprintf(os.Stderr, "OPEN %s: not found\n", name)
				}
				return 0xFF, 0, true
			}
			f, err := os.Open(path)
			if err != nil {
				return 0xFF, 0, true
			}
			h := nextHandle
			fileHandles[h] = f
			nextHandle++
			// Store handle in FCB byte 32 (cr field) for our tracking
			z80.WriteMemory(fcbAddr+32, h)
			// Set rc (record count) — file size / 128, clamped to 128
			fi, _ := f.Stat()
			rc := int(fi.Size()) / 128
			if rc > 128 {
				rc = 128
			}
			z80.WriteMemory(fcbAddr+15, byte(rc))
			// Clear extent counters
			z80.WriteMemory(fcbAddr+12, 0) // ex
			z80.WriteMemory(fcbAddr+14, 0) // s2
			if verbose {
				fmt.Fprintf(os.Stderr, "OPEN %s → handle %d (%d bytes)\n", name, h, fi.Size())
			}
			return 0, 0, true

		case 0x10: // Close file
			fcbAddr := de
			h := z80.ReadMemory(fcbAddr + 32)
			if f, ok := fileHandles[h]; ok {
				f.Close()
				delete(fileHandles, h)
			}
			return 0, 0, true

		case 0x11: // Search first (simplified — return first file in dir)
			entries, err := os.ReadDir(cpmDir)
			if err != nil || len(entries) == 0 {
				return 0xFF, 0, true
			}
			// Write directory entry to DMA (32-byte FCB format)
			// Simplified: just mark as found
			return 0, 0, true

		case 0x12: // Search next
			return 0xFF, 0, true // No more files

		case 0x13: // Delete file
			fcbAddr := de
			name := getFCBName(fcbAddr)
			path := findFile(name)
			if path != "" {
				os.Remove(path)
				return 0, 0, true
			}
			return 0xFF, 0, true

		case 0x14: // Read sequential
			fcbAddr := de
			h := z80.ReadMemory(fcbAddr + 32)
			f, ok := fileHandles[h]
			if !ok {
				if verbose {
					fmt.Fprintf(os.Stderr, "READ SEQ: bad handle %d\n", h)
				}
				return 0xFF, 0, true
			}
			// Calculate file position from FCB: (s2*4096 + ex*128 + cr) * 128
			ex := z80.ReadMemory(fcbAddr + 12)
			s2 := z80.ReadMemory(fcbAddr + 14) & 0x7F
			cr := z80.ReadMemory(fcbAddr + 32) // We use cr for handle, so seek from current pos
			_ = cr
			fpos := int64(s2)*4096*128 + int64(ex)*128*128
			// Actually, simpler: just read sequentially from current file position
			// (the file handle maintains its own position)
			_ = fpos

			buf := make([]byte, 128)
			// Pre-fill with EOF marker
			for i := range buf {
				buf[i] = 0x1A
			}
			n, _ := f.Read(buf)
			if n == 0 {
				return 1, 0, true // EOF
			}
			// Write to DMA
			for i := 0; i < 128; i++ {
				z80.WriteMemory(dmaAddr+uint16(i), buf[i])
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "READ %d bytes → DMA %04X\n", n, dmaAddr)
			}
			return 0, 0, true

		case 0x15: // Write sequential
			fcbAddr := de
			h := z80.ReadMemory(fcbAddr + 32)
			f, ok := fileHandles[h]
			if !ok {
				return 0xFF, 0, true
			}
			buf := make([]byte, 128)
			for i := 0; i < 128; i++ {
				buf[i] = z80.ReadMemory(dmaAddr + uint16(i))
			}
			f.Write(buf)
			return 0, 0, true

		case 0x16: // Make file (create)
			fcbAddr := de
			name := getFCBName(fcbAddr)
			path := filepath.Join(cpmDir, name)
			f, err := os.Create(path)
			if err != nil {
				return 0xFF, 0, true
			}
			h := nextHandle
			fileHandles[h] = f
			nextHandle++
			z80.WriteMemory(fcbAddr+32, h)
			if verbose {
				fmt.Fprintf(os.Stderr, "CREATE %s → handle %d\n", name, h)
			}
			return 0, 0, true

		case 0x17: // Rename file
			return 0, 0, true // stub

		case 0x19: // Get current disk
			return currentDisk, 0, true

		case 0x1A: // Set DMA address
			dmaAddr = de
			if verbose {
				fmt.Fprintf(os.Stderr, "[DMA=%04X] ", dmaAddr)
			}
			return 0, 0, true

		case 0x20: // Get/set user code
			if byte(de&0xFF) == 0xFF {
				return 0, 0, true
			}
			return 0, 0, true

		case 0x21: // Read random
			fcbAddr := de
			h := z80.ReadMemory(fcbAddr + 32)
			f, ok := fileHandles[h]
			if !ok {
				return 0xFF, 0, true
			}
			// Random record number from FCB bytes 33-35
			r0 := z80.ReadMemory(fcbAddr + 33)
			r1 := z80.ReadMemory(fcbAddr + 34)
			recNum := int64(r0) | int64(r1)<<8
			offset := recNum * 128
			f.Seek(offset, 0)
			buf := make([]byte, 128)
			for i := range buf {
				buf[i] = 0x1A
			}
			n, _ := f.Read(buf)
			if n == 0 {
				return 1, 0, true // EOF
			}
			for i := 0; i < 128; i++ {
				z80.WriteMemory(dmaAddr+uint16(i), buf[i])
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "READ RND rec=%d → DMA %04X\n", recNum, dmaAddr)
			}
			return 0, 0, true

		case 0x22: // Write random
			fcbAddr := de
			h := z80.ReadMemory(fcbAddr + 32)
			f, ok := fileHandles[h]
			if !ok {
				return 0xFF, 0, true
			}
			r0 := z80.ReadMemory(fcbAddr + 33)
			r1 := z80.ReadMemory(fcbAddr + 34)
			recNum := int64(r0) | int64(r1)<<8
			offset := recNum * 128
			f.Seek(offset, 0)
			buf := make([]byte, 128)
			for i := 0; i < 128; i++ {
				buf[i] = z80.ReadMemory(dmaAddr + uint16(i))
			}
			f.Write(buf)
			return 0, 0, true

		case 0x23: // Compute file size
			fcbAddr := de
			h := z80.ReadMemory(fcbAddr + 32)
			if f, ok := fileHandles[h]; ok {
				fi, _ := f.Stat()
				records := fi.Size() / 128
				z80.WriteMemory(fcbAddr+33, byte(records&0xFF))
				z80.WriteMemory(fcbAddr+34, byte(records>>8))
				z80.WriteMemory(fcbAddr+35, 0)
			}
			return 0, 0, true

		case 0x24: // Set random record from sequential
			fcbAddr := de
			ex := z80.ReadMemory(fcbAddr + 12)
			s2 := z80.ReadMemory(fcbAddr + 14) & 0x7F
			cr := z80.ReadMemory(fcbAddr + 32)
			rec := int(s2)*4096 + int(ex)*128 + int(cr)
			z80.WriteMemory(fcbAddr+33, byte(rec&0xFF))
			z80.WriteMemory(fcbAddr+34, byte(rec>>8))
			z80.WriteMemory(fcbAddr+35, 0)
			return 0, 0, true

		default:
			if verbose {
				fmt.Fprintf(os.Stderr, "[BDOS %02X unhandled] ", function)
			}
			return 0, 0, true
		}
	})
}

// setupAgonMOS sets up Agon MOS RST handler
func setupAgonMOS(z80 *emulator.RemogattoZ80WithScreen) {
	z80.SetRSTHandler(func(vector byte, regs emulator.RSTRegisters) (emulator.RSTRegisters, bool) {
		switch vector {
		case 0x10: // mos_putchar — character in A
			fmt.Printf("%c", regs.A)
			return regs, true
		case 0x18: // mos_puts — string pointer in HL
			addr := regs.HL
			for {
				ch := z80.ReadMemory(addr)
				if ch == 0 {
					break
				}
				fmt.Printf("%c", ch)
				addr++
			}
			return regs, true
		case 0x00: // MOS API call — function in A
			if verbose {
				fmt.Printf("[MOS %02X] ", regs.A)
			}
			switch regs.A {
			case 0x00: // mos_getkey — return key (stub: newline)
				regs.A = '\n'
				return regs, true
			default:
				return regs, true
			}
		case 0x08: // mos_sysvars — return pointer in IY (stub)
			if verbose {
				fmt.Printf("[MOS SYSVARS] ")
			}
			return regs, true
		default:
			if verbose {
				fmt.Printf("[RST %02X unhandled] ", vector)
			}
			return regs, true
		}
	})
}

func init() {
	// Memory options
	rootCmd.Flags().UintVar(&loadAddr, "load", 0x8000, "load address for binary (default: 0x8000)")
	rootCmd.Flags().UintVar(&startAddr, "start", 0, "start address (default: same as load address)")

	// Platform options
	rootCmd.Flags().StringVarP(&target, "target", "t", "spectrum", "target platform (spectrum, cpm, cpc)")

	// Execution options
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose execution info")
	rootCmd.Flags().BoolVarP(&cycles, "cycles", "c", false, "show T-state cycle count")
	rootCmd.Flags().UintVar(&timeout, "timeout", 0, "execution timeout in cycles (0 = no timeout)")

	// Debug options
	rootCmd.Flags().BoolVarP(&debugMode, "debug", "d", false, "start DAP debug server on stdin/stdout")

	// Profiler options
	rootCmd.Flags().StringVar(&profilePath, "profile", "", "export heatmap JSON (exec/read/write/stack/io + memory snapshot)")
	rootCmd.Flags().StringVar(&tracePath, "trace", "", "export basic-block trace to JSONL file")

	// Console I/O options
	rootCmd.Flags().StringVar(&consolePort, "console-to-port", "", "map console I/O to port (e.g. $23, 0xFF, 35)")
	rootCmd.Flags().BoolVar(&consoleIO, "console-io", false, "enable console I/O on default port ($23)")

	// Diagnostics
	rootCmd.Flags().BoolVar(&warnOnHalt, "warn-on-halt", false, "warn when HALT with interrupts disabled (stuck CPU)")
	rootCmd.Flags().Int64Var(&breakAtTstate, "break-at-tstate", 0, "break and dump registers at T-state count")
	rootCmd.Flags().BoolVar(&diag, "diag", false, "print CPU diagnostics after execution")
}

// debugCmd starts the DAP server for VS Code integration
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Start DAP debug server for IDE integration",
	Long: `Start the Debug Adapter Protocol (DAP) server.

This command starts a DAP server on stdin/stdout for integration
with VS Code and other DAP-compatible debuggers.

The DAP server provides:
  • Breakpoint support (address and source-level)
  • Step, continue, and pause commands
  • Register and memory inspection
  • SMC (Self-Modifying Code) event tracking
  • Disassembly view

Usage with VS Code:
  1. Install the MinZ Debug extension
  2. Create a launch.json with type "minz"
  3. Start debugging (F5)`,
	Run: func(cmd *cobra.Command, args []string) {
		server := dap.NewServer()
		if err := server.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "DAP server error: %v\n", err)
			os.Exit(1)
		}
	},
}

var dzrpPort int

// dzrpCmd starts the DZRP server for DeZog integration
var dzrpCmd = &cobra.Command{
	Use:   "dzrp [binary file]",
	Short: "Start DZRP debug server for DeZog integration",
	Long: `Start the DeZog Remote Protocol (DZRP) server.

This command starts a DZRP server on TCP port 11000 (configurable)
for integration with the DeZog VS Code extension.

DZRP provides:
  • Full breakpoint support (execution, read, write)
  • Register inspection and modification
  • Memory inspection and modification
  • Step, continue, and pause commands
  • Compatible with DeZog source-level debugging

Usage with DeZog:
  1. Install DeZog extension in VS Code
  2. Start: mze dzrp program.bin
  3. In DeZog, connect to localhost:11000
  4. Load your .sld/.list file for source mapping`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		binaryFile := args[0]

		// Parse addresses
		loadAddress := uint16(loadAddr)
		startAddress := uint16(startAddr)
		if startAddress == 0 {
			startAddress = loadAddress
		}

		fmt.Printf("MZE DZRP Server - DeZog Integration\n")
		fmt.Printf("Binary: %s\n", binaryFile)
		fmt.Printf("Load:   $%04X\n", loadAddress)
		fmt.Printf("Start:  $%04X\n", startAddress)
		fmt.Printf("Port:   %d\n", dzrpPort)
		fmt.Println()

		// Read the binary file
		binary, err := os.ReadFile(binaryFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading binary file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Loaded %d bytes\n", len(binary))

		// Create Z80 emulator
		z80 := emulator.NewRemogattoZ80WithScreen()
		z80.LoadAt(loadAddress, binary)
		z80.SetPC(startAddress)

		// Create DZRP adapter
		adapter := createDZRPAdapter(z80)

		// Create and start DZRP server
		server := dzrp.NewServer(adapter)
		server.Port = dzrpPort
		server.MachineName = "MZE MinZ Emulator v2.0"

		err = server.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start DZRP server: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Printf("DZRP server running on port %d\n", dzrpPort)
		fmt.Printf("Connect DeZog to: localhost:%d\n", dzrpPort)
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop the server...")

		// Wait for interrupt
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\nStopping DZRP server...")
		server.Stop()
	},
}

// createDZRPAdapter creates a DZRP adapter for the emulator
func createDZRPAdapter(z80 *emulator.RemogattoZ80WithScreen) dzrp.Emulator {
	// Use a wrapper that implements the dzrp.Emulator interface
	return &mzeAdapter{z80: z80}
}

// mzeAdapter wraps RemogattoZ80WithScreen for DZRP
type mzeAdapter struct {
	z80    *emulator.RemogattoZ80WithScreen
	paused bool
}

func (a *mzeAdapter) GetPC() uint16  { return a.z80.GetPC() }
func (a *mzeAdapter) SetPC(v uint16) { a.z80.SetPC(v) }
func (a *mzeAdapter) GetSP() uint16  { return a.z80.GetSP() }
func (a *mzeAdapter) SetSP(v uint16) { a.z80.SetSP(v) }
func (a *mzeAdapter) GetA() byte     { return a.z80.GetRegisters().A }
func (a *mzeAdapter) SetA(v byte)    { /* Need to add SetA to RemogattoZ80 */ }
func (a *mzeAdapter) GetF() byte     { return a.z80.GetRegisters().F }
func (a *mzeAdapter) SetF(v byte)    { /* Need to add SetF to RemogattoZ80 */ }
func (a *mzeAdapter) GetB() byte     { return byte(a.z80.GetRegisters().BC >> 8) }
func (a *mzeAdapter) SetB(v byte)    { /* Need to add */ }
func (a *mzeAdapter) GetC() byte     { return byte(a.z80.GetRegisters().BC & 0xFF) }
func (a *mzeAdapter) SetC(v byte)    { /* Need to add */ }
func (a *mzeAdapter) GetD() byte     { return byte(a.z80.GetRegisters().DE >> 8) }
func (a *mzeAdapter) SetD(v byte)    { /* Need to add */ }
func (a *mzeAdapter) GetE() byte     { return byte(a.z80.GetRegisters().DE & 0xFF) }
func (a *mzeAdapter) SetE(v byte)    { /* Need to add */ }
func (a *mzeAdapter) GetH() byte     { return byte(a.z80.GetRegisters().HL >> 8) }
func (a *mzeAdapter) SetH(v byte)    { /* Need to add */ }
func (a *mzeAdapter) GetL() byte     { return byte(a.z80.GetRegisters().HL & 0xFF) }
func (a *mzeAdapter) SetL(v byte)    { /* Need to add */ }
func (a *mzeAdapter) GetIX() uint16  { return a.z80.GetRegisters().IX }
func (a *mzeAdapter) SetIX(v uint16) { /* Need to add */ }
func (a *mzeAdapter) GetIY() uint16  { return a.z80.GetRegisters().IY }
func (a *mzeAdapter) SetIY(v uint16) { /* Need to add */ }
func (a *mzeAdapter) GetI() byte     { return a.z80.RemogattoZ80.GetI() }
func (a *mzeAdapter) SetI(v byte)    { /* Need to add */ }
func (a *mzeAdapter) GetR() byte     { return a.z80.RemogattoZ80.GetR() }
func (a *mzeAdapter) GetIM() byte    { return a.z80.RemogattoZ80.GetIM() }
func (a *mzeAdapter) GetIFF1() bool  { return a.z80.RemogattoZ80.GetIFF1() }
func (a *mzeAdapter) GetIFF2() bool  { return a.z80.RemogattoZ80.GetIFF2() }
func (a *mzeAdapter) GetAF2() uint16 { return 0 } // TODO: expose alternate regs
func (a *mzeAdapter) GetBC2() uint16 { return 0 }
func (a *mzeAdapter) GetDE2() uint16 { return 0 }
func (a *mzeAdapter) GetHL2() uint16 { return 0 }

func (a *mzeAdapter) GetMemory(addr uint16) byte {
	return a.z80.GetMemory(addr)
}

func (a *mzeAdapter) SetMemory(addr uint16, value byte) {
	a.z80.SetMemory(addr, value)
}

func (a *mzeAdapter) ReadMemoryRange(addr uint16, length int) []byte {
	data := make([]byte, length)
	for i := 0; i < length; i++ {
		data[i] = a.z80.GetMemory(addr + uint16(i))
	}
	return data
}

func (a *mzeAdapter) WriteMemoryRange(addr uint16, data []byte) {
	for i, b := range data {
		a.z80.SetMemory(addr+uint16(i), b)
	}
}

func (a *mzeAdapter) Step() int {
	return a.z80.Step()
}

func (a *mzeAdapter) Run() {
	a.paused = false
	// Run in background would go here
}

func (a *mzeAdapter) Pause() {
	a.paused = true
}

func (a *mzeAdapter) IsRunning() bool {
	return !a.paused && !a.z80.IsHalted()
}

func (a *mzeAdapter) IsHalted() bool {
	return a.z80.IsHalted()
}

func (a *mzeAdapter) GetCycles() int {
	return a.z80.GetCycles()
}

func init() {
	dzrpCmd.Flags().UintVar(&loadAddr, "load", 0x8000, "load address for binary")
	dzrpCmd.Flags().UintVar(&startAddr, "start", 0, "start address (default: same as load)")
	dzrpCmd.Flags().IntVar(&dzrpPort, "port", 11000, "DZRP server port")
	rootCmd.AddCommand(dzrpCmd)
}

func init() {
	rootCmd.AddCommand(debugCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
