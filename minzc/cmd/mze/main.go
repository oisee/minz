package main

import (
	"fmt"
	"os"
	"os/signal"
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

// setupCPMBDOS sets up CP/M BDOS handler
func setupCPMBDOS(z80 *emulator.RemogattoZ80WithScreen) {
	dmaAddr := uint16(0x0080) // Default DMA address
	currentDisk := byte(0)    // A:

	z80.SetBDOSHandler(func(function byte, de uint16) (a byte, hl uint16, handled bool) {
		if verbose {
			fmt.Printf("[BDOS %02X DE=%04X] ", function, de)
		}
		switch function {
		case 0x00: // System reset (warm boot) — halt emulation
			return 0, 0, true
		case 0x01: // Console input
			return '\n', 0, true
		case 0x02: // Console output
			fmt.Printf("%c", byte(de&0xFF))
			return 0, 0, true
		case 0x06: // Direct console I/O
			if byte(de&0xFF) == 0xFF {
				return 0, 0, true // No char available
			} else if byte(de&0xFF) == 0xFE {
				return 0, 0, true // No char available
			} else {
				fmt.Printf("%c", byte(de&0xFF))
				return 0, 0, true
			}
		case 0x09: // Print string ($-terminated)
			addr := de
			for {
				ch := z80.ReadMemory(addr)
				if ch == '$' {
					break
				}
				fmt.Printf("%c", ch)
				addr++
			}
			return 0, 0, true
		case 0x0B: // Console status
			return 0, 0, true
		case 0x0C: // Get version
			return 0x22, 0x0022, true // CP/M 2.2
		case 0x0D: // Reset disk system
			currentDisk = 0
			dmaAddr = 0x0080
			return 0, 0, true
		case 0x0E: // Select disk
			currentDisk = byte(de & 0xFF)
			return 0, 0, true
		case 0x19: // Get current disk
			return currentDisk, 0, true
		case 0x1A: // Set DMA address
			dmaAddr = de
			if verbose {
				fmt.Printf("[DMA=%04X] ", dmaAddr)
			}
			return 0, 0, true
		case 0x20: // Get/set user code
			if byte(de&0xFF) == 0xFF {
				return 0, 0, true // Return current user (0)
			}
			return 0, 0, true
		default:
			if verbose {
				fmt.Printf("[BDOS %02X unhandled] ", function)
			}
			return 0, 0, true
		}
	})

	_ = dmaAddr // Will be used by file I/O handlers later
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
