package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/debugger"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/spf13/cobra"
)

var (
	loadAddr   uint
	address    string
	startAddr  uint
	target     string
	tapeFile   string
	diskFile   string
	interrupts bool
	verbose    bool
	cycles     bool
	interactive bool
	timeout    uint
	debug      bool  // New flag for debugger mode
)

var rootCmd = &cobra.Command{
	Use:   "mze [binary file]",
	Short: "MinZ Z80 Multi-Platform Emulator v1.0",
	Long: `mze - MinZ Z80 Multi-Platform Emulator v1.0
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Cycle-accurate Z80 emulator with platform-specific I/O simulation

SUPPORTED PLATFORMS (-t/--target):
  spectrum - ZX Spectrum (default)
    • RST $10 (16) - Print character to screen
    • RST $18 (24) - Read character from keyboard
    • RST $20 (32) - Get next input character
    • 50Hz interrupts (IM 1)

  cpm - CP/M 2.2 BDOS
    • CALL 5 - BDOS system calls
    • Function 2: Console output
    • Function 9: Print string
    • Function 10: Read console buffer

  cpc - Amstrad CPC
    • CALL $BB5A - TXT OUTPUT
    • RST $10 - Firmware calls

FEATURES:
  • Full Z80 instruction set (documented + undocumented)
  • Cycle-accurate T-state counting
  • 64KB memory with banking support
  • Platform-specific I/O redirection
  • Safety stop at 10M T-states
  • Interactive debugging with --debug flag

EXIT CONDITIONS:
  • HALT with interrupts disabled (DI:HALT)
  • RST $38 (error trap)
  • 10M T-states safety limit
  • Invalid opcode execution

EXAMPLES:
  mze hello.bin                        # Run on ZX Spectrum
  mze -t cpm hello_cpm.com             # Run on CP/M
  mze -a 0x4000 loader.bin             # Load at $4000
  mze -v -c program.bin                # Verbose with cycles
  mze --interrupts=false fast.bin      # No interrupts
  mze --timeout=1000000 test.bin       # Stop after 1M cycles
  mze --debug program.bin               # Run with interactive debugger`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		binaryFile := args[0]
		loadAddress := uint16(loadAddr)
		
		// Parse hex address if provided
		if address != "" {
			addr, err := parseHexAddress(address)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing hex address '%s': %v\n", address, err)
				os.Exit(1)
			}
			loadAddress = addr
		}
		
		// Validate target platform
		switch target {
		case "spectrum", "cpm", "cpc":
			// Valid targets
		default:
			fmt.Fprintf(os.Stderr, "Error: Unsupported target platform '%s'\n", target)
			fmt.Fprintf(os.Stderr, "Supported targets: spectrum, cpm, cpc\n")
			os.Exit(1)
		}
		
		startAddress := uint16(startAddr)
		if startAddress == 0 {
			startAddress = loadAddress
		}

		if verbose {
			fmt.Printf("🎮 mze - MinZ Z80 Multi-Platform Emulator\n")
			fmt.Printf("🎯 Target: %s\n", target)
			fmt.Printf("📁 Binary: %s\n", binaryFile)
			fmt.Printf("📍 Load:   $%04X (%d)\n", loadAddress, loadAddress)
			fmt.Printf("🚀 Start:  $%04X (%d)\n", startAddress, startAddress)
			if tapeFile != "" {
				fmt.Printf("📼 Tape:   %s\n", tapeFile)
			}
			if diskFile != "" {
				fmt.Printf("💾 Disk:   %s\n", diskFile)
			}
			if timeout > 0 {
				fmt.Printf("⏰ Timeout: %d T-states\n", timeout)
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
			fmt.Printf("📦 Loaded %d bytes\n", len(binary))
		}

		// Create Z80 emulator with hook support
		z80 := emulator.NewZ80WithScreen()
		
		// Load binary into memory at specified address
		z80.LoadAt(loadAddress, binary)

		// TODO: Implement interactive input handling per platform

		// Set up platform-specific system call interceptions
		switch target {
		case "spectrum":
			setupSpectrumHooks(z80, verbose)
		case "cpm":
			setupCPMHooks(z80, verbose)
		case "cpc":
			setupCPCHooks(z80, verbose)
		}


		// Set start address
		z80.PC = startAddress
		
		// Check if debugger mode is requested
		if debug {
			fmt.Println("🔧 Starting interactive debugger...")
			
			// Create debugger with configuration
			dbg := debugger.New(z80.Z80, &debugger.Config{
				MaxHistory: 100,
				Input:      os.Stdin,
				Output:     os.Stdout,
			})
			
			// Run the debugger (this blocks until user exits)
			if err := dbg.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Debugger error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		
		// Normal execution mode
		if verbose {
			fmt.Printf("▶️  Starting execution at $%04X...\n", startAddress)
			fmt.Println("----------------------------------------")
		}

		// Execute until program ends or timeout
		var totalCycles uint32
		maxCycles := uint32(timeout)
		
		// 50Hz interrupt simulation (3.5MHz Z80 = 70,000 T-states per interrupt)
		const INTERRUPT_PERIOD = 70000
		var nextInterrupt uint32 = INTERRUPT_PERIOD
		
		for {
			// Check for timeout
			if maxCycles > 0 && totalCycles >= maxCycles {
				if verbose {
					fmt.Printf("\n🚨 Safety stop after %dM T-states\n", totalCycles/1000000)
				}
				break
			}
			
			// Check if we hit a RET at the start level (simple end detection)  
			if z80.ReadMemory(z80.PC) == 0xC9 && z80.SP == 0xFFFF {
				if verbose {
					fmt.Printf("\n🏁 Program ended with RET at $%04X\n", z80.PC)
				}
				break
			}
			
			// Check for 50Hz interrupt (before executing instruction)
			if interrupts && totalCycles >= nextInterrupt && !z80.Z80.IsHalted() {
				// Only interrupt if interrupts are enabled (IFF1)
				if z80.Z80.GetIFF1() {
					if verbose {
						fmt.Printf("\n⚡ 50Hz interrupt at $%04X (cycle %d)\n", z80.PC, totalCycles)
					}
					// Execute interrupt (push PC, jump to $0038 - RST 38h)
					z80.SP -= 2
					z80.WriteMemory(z80.SP, uint8(z80.PC&0xFF))
					z80.WriteMemory(z80.SP+1, uint8(z80.PC>>8))
					z80.PC = 0x0038
					totalCycles += 13 // Interrupt overhead
				}
				nextInterrupt += INTERRUPT_PERIOD
			}
			
			// Save PC before instruction execution to detect changes
			prevPC := z80.PC
			
			// For ZX Spectrum, check if we're about to execute a RST that we want to intercept
			// We need to handle it BEFORE execution to prevent jumping to the RST address
			if target == "spectrum" {
				opcode := z80.ReadMemory(z80.PC)
				handled := false
				
				switch opcode {
				case 0xD7: // RST $10
					if verbose {
						fmt.Printf("🖨️  RST $10: print '%c' (0x%02X)\n", printableChar(z80.A), z80.A)
					}
					// Handle special characters
					switch z80.A {
					case 13: // CR
						fmt.Print("\n")
					case 10: // LF  
						// Skip - we handle newlines with CR
					case 8: // Backspace
						fmt.Print("\b")
					case 9: // Tab
						fmt.Print("\t")
					default:
						if z80.A >= 32 && z80.A <= 126 {
							fmt.Printf("%c", z80.A)
						} else if verbose {
							fmt.Printf("[0x%02X]", z80.A)
						}
					}
					z80.PC++ // Move past the RST instruction
					totalCycles += 11 // RST takes 11 T-states
					handled = true
					
				case 0xDF: // RST $18
					if verbose {
						fmt.Printf("⌨️  RST $18: read character: ")
					}
					// Read a single character from stdin
					var input [1]byte
					n, err := os.Stdin.Read(input[:])
					if err != nil || n == 0 {
						z80.A = 0 // Return null on error
					} else {
						z80.A = input[0]
						if verbose {
							fmt.Printf("got '%c' (0x%02X)\n", printableChar(input[0]), input[0])
						}
					}
					z80.PC++ // Move past the RST instruction
					totalCycles += 11
					handled = true
					
				case 0xE7: // RST $20
					if verbose {
						fmt.Printf("⌨️  RST $20: get next character\n")
					}
					// TODO: Implement proper stdin handling
					z80.A = 0
					z80.PC++ // Move past the RST instruction
					totalCycles += 11
					handled = true
				}
				
				if handled {
					continue // Skip normal instruction execution
				}
			}
			
			// Execute the instruction normally
			cyclesUsed := uint32(z80.Z80.Step())
			totalCycles += cyclesUsed
			
			
			// Check if PC changed to $0005 (CP/M BDOS entry point)
			// This catches CALL $0005, JP $0005, or any other way to reach it
			if target == "cpm" && z80.PC == 0x0005 && prevPC != 0x0005 {
				if verbose {
					fmt.Printf("💫 BDOS entry detected (PC changed from $%04X to $0005), SP=$%04X\n", prevPC, z80.SP)
				}
				// Handle BDOS call
				handleBDOSCall(z80, verbose)
				// Return from BDOS - pop return address from stack
				lowByte := z80.Z80.ReadMemory(z80.SP)
				highByte := z80.Z80.ReadMemory(z80.SP + 1)
				z80.PC = uint16(lowByte) | (uint16(highByte) << 8)
				z80.SP += 2
				if verbose {
					fmt.Printf("💫 Returning to PC=$%04X, SP=$%04X\n", z80.PC, z80.SP)
				}
				continue
			}
			
			// Note: We lose RST hook support this way, but gain proper BDOS detection
			// TODO: Fix ExecuteWithHooks to properly return after each instruction
			
			
			// Check if CPU is halted
			if z80.Z80.IsHalted() {
				// Check if interrupts are disabled (DI:HALT combination)
				if !z80.Z80.GetIFF1() {
					if verbose {
						fmt.Printf("\n🛑 DI:HALT detected at PC=$%04X - program terminated cleanly\n", z80.PC)
					}
					break
				} else {
					// HALT with interrupts enabled - wait for interrupt
					if verbose && totalCycles == 0 {
						fmt.Printf("\n⏸️  CPU halted at PC=$%04X (waiting for interrupt)\n", z80.PC)
					}
					// Continue to allow interrupts to wake the CPU
				}
			}
			
			// Safety check - prevent infinite loops
			if totalCycles > 10000000 { // 10M cycles
				if verbose {
					fmt.Printf("\n🚨 Safety stop after 10M T-states\n")
				}
				break
			}
		}
		
		if verbose {
			fmt.Println("----------------------------------------")
		}
		
		if cycles {
			fmt.Printf("⏱️  Total execution: %d T-states\n", totalCycles)
		}
		
		if verbose {
			fmt.Printf("✅ Execution completed\n")
			
			// Show final register state
			fmt.Printf("\n📊 Final Register State:\n")
			fmt.Printf("   PC=$%04X  SP=$%04X  A=$%02X  F=$%02X\n", 
				z80.PC, z80.SP, z80.A, z80.F)
			fmt.Printf("   BC=$%04X  DE=$%04X  HL=$%04X\n",
				uint16(z80.B)<<8|uint16(z80.C),
				uint16(z80.D)<<8|uint16(z80.E), 
				uint16(z80.H)<<8|uint16(z80.L))
		}
	},
}

func init() {
	// Memory options
	rootCmd.Flags().UintVar(&loadAddr, "load", 0x8000, "load address for binary (default: 0x8000)")
	rootCmd.Flags().StringVarP(&address, "address", "a", "", "load address in hex format (e.g., #8000, 0x8000, $8000)")
	rootCmd.Flags().UintVar(&startAddr, "start", 0, "start address (default: same as load address)")
	
	// Platform options
	rootCmd.Flags().StringVarP(&target, "target", "t", "spectrum", "target platform (spectrum, cpm, cpc)")
	
	// Media options
	rootCmd.Flags().StringVar(&tapeFile, "tape", "", "tape file for Spectrum mode (.tap/.tzx)")
	rootCmd.Flags().StringVar(&diskFile, "disk", "", "disk image for TR-DOS mode (.trd)")
	
	// Execution options
	rootCmd.Flags().BoolVar(&interrupts, "interrupts", true, "enable 50Hz interrupt simulation")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose execution info")
	rootCmd.Flags().BoolVarP(&cycles, "cycles", "c", false, "show T-state cycle count")
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "interactive mode (pause on RST $18/$20)")
	rootCmd.Flags().UintVar(&timeout, "timeout", 0, "execution timeout in cycles (0 = no timeout)")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "run in interactive debugger mode")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// printableChar returns a printable representation of a character
func printableChar(c byte) rune {
	if c >= 32 && c <= 126 {
		return rune(c)
	}
	return '.'
}

// parseHexAddress parses hex addresses in various formats (#8000, 0x8000, $8000)
func parseHexAddress(addr string) (uint16, error) {
	addr = strings.TrimSpace(addr)
	
	// Handle different hex prefixes
	var hexStr string
	switch {
	case strings.HasPrefix(addr, "#"):
		hexStr = addr[1:] // ZX Spectrum style
	case strings.HasPrefix(addr, "$"):
		hexStr = addr[1:] // Assembly style
	case strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X"):
		hexStr = addr[2:] // C style
	default:
		hexStr = addr // Assume plain hex
	}
	
	// Parse the hex value
	value, err := strconv.ParseUint(hexStr, 16, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid hex format: %v", err)
	}
	
	return uint16(value), nil
}

// setupSpectrumHooks configures ZX Spectrum RST interceptions
func setupSpectrumHooks(z80 *emulator.Z80WithScreen, verbose bool) {
	// Set up console output hooks (override the default screen hooks for console output)
	z80.Hooks.OnRST10 = func(a byte) {
		// RST $10 - Print character to host stdout
		if verbose {
			fmt.Printf("🖨️  RST $10: '%c' (0x%02X)\n", printableChar(a), a)
		}
		
		// Handle special characters
		switch a {
		case 13: // CR
			fmt.Print("\n")
		case 10: // LF  
			// Skip - we handle newlines with CR
		case 8: // Backspace
			fmt.Print("\b")
		case 9: // Tab
			fmt.Print("\t")
		default:
			if a >= 32 && a <= 126 {
				fmt.Printf("%c", a)
			} else if verbose {
				fmt.Printf("[0x%02X]", a)
			}
		}
	}

	z80.Hooks.OnRST18 = func() byte {
		// RST $18 - Collect character from host stdin
		// TODO: Implement proper stdin handling
		return 0
	}

	z80.Hooks.OnRST20 = func() byte {
		// RST $20 - Next character from host stdin  
		// TODO: Implement proper stdin handling
		return 0
	}
	
	if verbose {
		fmt.Printf("🖥️  ZX Spectrum hooks configured\n")
	}
}

// setupCPMHooks configures CP/M BDOS system call interceptions
func setupCPMHooks(z80 *emulator.Z80WithScreen, verbose bool) {
	// CP/M uses CALL 5 for BDOS functions
	// Detection is done in the main loop by checking for CALL $0005 instructions
	// We don't modify memory - proper CP/M emulation!
	
	// In real CP/M, address $0005 contains a JP to the BDOS
	// We could optionally set this up for compatibility:
	// z80.WriteMemory(0x0005, 0xC3) // JP instruction
	// z80.WriteMemory(0x0006, 0x06) // Low byte (dummy BDOS address)
	// z80.WriteMemory(0x0007, 0xE4) // High byte (dummy BDOS address)
	// But we intercept CALL $0005 before it executes, so this isn't needed
	
	if verbose {
		fmt.Printf("🖥️  CP/M BDOS hooks configured\n")
	}
}


// setupCPCHooks configures Amstrad CPC firmware call interceptions  
func setupCPCHooks(z80 *emulator.Z80WithScreen, verbose bool) {
	z80.Hooks.OnRST10 = func(a byte) {
		// CPC uses different character encoding
		if verbose {
			fmt.Printf("🖥️  CPC OUT: '%c' (0x%02X)\n", printableChar(a), a)
		}
		fmt.Printf("%c", a)
	}
	
	if verbose {
		fmt.Printf("💻 CPC firmware hooks configured\n")
	}
}

// handleBDOSCall processes CP/M BDOS system calls
func handleBDOSCall(z80 *emulator.Z80WithScreen, verbose bool) {
	function := z80.Z80.GetC() // BDOS function number in C register
	
	if verbose {
		fmt.Printf("🖥️  BDOS call: function %d\n", function)
	}
	
	switch function {
	case 0: // System reset/exit
		if verbose {
			fmt.Printf("💀 Program terminated (BDOS function 0)\n")
		}
		z80.Z80.SetHalted(true) // Halt the CPU
		
	case 2: // Console output
		char := z80.Z80.GetE()
		if verbose {
			fmt.Printf("🖨️  BDOS console output: '%c' (0x%02X)\n", printableChar(char), char)
		}
		// Output the character directly
		fmt.Printf("%c", char)
		
	case 9: // Print string (DE = address)
		addr := z80.Z80.GetDE()
		if verbose {
			fmt.Printf("🖨️  BDOS print string from $%04X\n", addr)
		}
		// Print string until $ terminator (max 255 chars for safety)
		for i := 0; i < 255; i++ {
			ch := z80.ReadMemory(addr)
			if ch == '$' {
				break
			}
			// Only print printable characters, skip control characters except CR/LF
			if ch >= 32 && ch <= 126 {
				fmt.Printf("%c", ch)
			} else if ch == 13 {
				fmt.Print("\r")
			} else if ch == 10 {
				fmt.Print("\n")
			} else if verbose {
				fmt.Printf("[0x%02X]", ch)
			}
			addr++
		}
		
	case 1: // Console input
		if verbose {
			fmt.Printf("⌨️  BDOS console input: ")
		}
		// Read a single character from stdin
		var input [1]byte
		n, err := os.Stdin.Read(input[:])
		if err != nil || n == 0 {
			z80.Z80.SetA(0) // Return null on error
		} else {
			z80.Z80.SetA(input[0])
			if verbose {
				fmt.Printf("got '%c' (0x%02X)\n", printableChar(input[0]), input[0])
			}
		}
		
	case 10: // Read console buffer
		addr := z80.Z80.GetDE()
		maxLen := z80.ReadMemory(addr) // First byte is max length
		if verbose {
			fmt.Printf("⌨️  BDOS read console buffer at $%04X, max %d chars\n", addr, maxLen)
		}
		
		// Read input line
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			line = ""
		}
		
		// Remove trailing newline
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
		
		// Truncate if too long
		if len(line) > int(maxLen) {
			line = line[:maxLen]
		}
		
		// Store actual length
		z80.WriteMemory(addr+1, uint8(len(line)))
		
		// Store the characters
		for i, ch := range []byte(line) {
			z80.WriteMemory(addr+2+uint16(i), ch)
		}
		
		if verbose {
			fmt.Printf("  Read %d characters: %s\n", len(line), line)
		}
		
	case 11: // Console status
		// Check if input is available
		// For simplicity, always return "ready"
		z80.Z80.SetA(0xFF)
		if verbose {
			fmt.Printf("🖥️  BDOS console status: ready\n")
		}
		
	default:
		if verbose {
			fmt.Printf("⚠️  Unimplemented BDOS function: %d\n", function)
		}
	}
}