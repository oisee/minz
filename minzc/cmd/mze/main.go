package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
)

func main() {
	var (
		loadAddr    = flag.Uint("load", 0x8000, "Load address for binary (default: 0x8000)")
		addressFlag = flag.String("a", "", "Load address in hex format (e.g., #8000, 0x8000, $8000)")
		address     = flag.String("address", "", "Load address in hex format (e.g., #8000, 0x8000, $8000)")
		startAddr   = flag.Uint("start", 0, "Start address (default: same as load address)")
		target      = flag.String("t", "spectrum", "Target platform (spectrum, cpm, cpc)")
		targetLong  = flag.String("target", "", "Target platform (spectrum, cpm, cpc)")
		tapeFile    = flag.String("tape", "", "Tape file for Spectrum mode (.tap/.tzx)")
		diskFile    = flag.String("disk", "", "Disk image for TR-DOS mode (.trd)")
		interrupts  = flag.Bool("int", true, "Enable 50Hz interrupt simulation (disable with --int=false)")
		verbose     = flag.Bool("v", false, "Verbose execution info")
		cycles      = flag.Bool("c", false, "Show T-state cycle count")
		_ = flag.Bool("i", false, "Interactive mode (pause on RST $18/$20) - TODO: implement")
		help        = flag.Bool("h", false, "Show help")
		timeout     = flag.Uint("timeout", 0, "Execution timeout in cycles (0 = no timeout)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "mze - MinZ Z80 Multi-Platform Emulator/Simulator\n\n")
		fmt.Fprintf(os.Stderr, "Usage: mze [options] program.bin\n\n")
		fmt.Fprintf(os.Stderr, "Supported Platforms (-t/--target):\n")
		fmt.Fprintf(os.Stderr, "  spectrum - ZX Spectrum (default)\n")
		fmt.Fprintf(os.Stderr, "    RST $10 (16) - Print character → host stdout\n")
		fmt.Fprintf(os.Stderr, "    RST $18 (24) - Collect character → host stdin\n")
		fmt.Fprintf(os.Stderr, "    RST $20 (32) - Next character → host stdin\n")
		fmt.Fprintf(os.Stderr, "  cpm - CP/M 2.2 BDOS system calls\n")
		fmt.Fprintf(os.Stderr, "    CALL 5 - BDOS function calls → host I/O\n")
		fmt.Fprintf(os.Stderr, "  cpc - Amstrad CPC firmware calls\n")
		fmt.Fprintf(os.Stderr, "    RST $10 - CPC screen output\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  mze program.bin                      # ZX Spectrum mode (default)\n")
		fmt.Fprintf(os.Stderr, "  mze -t cpm cpm_program.com           # CP/M mode\n")
		fmt.Fprintf(os.Stderr, "  mze -t spectrum -tape game.tap rom   # Spectrum with tape\n")
		fmt.Fprintf(os.Stderr, "  mze -a #4000 boot.bin                # Load at $4000\n")
		fmt.Fprintf(os.Stderr, "  mze -v -c program.bin                # Verbose with cycle count\n")
	}

	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Error: No binary file specified\n")
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "Error: Multiple binary files not supported\n")
		flag.Usage()
		os.Exit(1)
	}

	binaryFile := flag.Arg(0)
	loadAddress := uint16(*loadAddr)
	
	// Parse hex address if provided with -a or --address
	hexAddr := *addressFlag
	if hexAddr == "" {
		hexAddr = *address
	}
	if hexAddr != "" {
		addr, err := parseHexAddress(hexAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing hex address '%s': %v\n", hexAddr, err)
			os.Exit(1)
		}
		loadAddress = addr
	}
	
	// Determine target platform
	platformTarget := *target
	if platformTarget == "" {
		platformTarget = *targetLong
	}
	if platformTarget == "" {
		platformTarget = "spectrum" // Default
	}
	
	// Validate target platform
	switch platformTarget {
	case "spectrum", "cpm", "cpc":
		// Valid targets
	default:
		fmt.Fprintf(os.Stderr, "Error: Unsupported target platform '%s'\n", platformTarget)
		fmt.Fprintf(os.Stderr, "Supported targets: spectrum, cpm, cpc\n")
		os.Exit(1)
	}
	
	startAddress := uint16(*startAddr)
	if startAddress == 0 {
		startAddress = loadAddress
	}

	if *verbose {
		fmt.Printf("🎮 mze - MinZ Z80 Multi-Platform Emulator\n")
		fmt.Printf("🎯 Target: %s\n", platformTarget)
		fmt.Printf("📁 Binary: %s\n", binaryFile)
		fmt.Printf("📍 Load:   $%04X (%d)\n", loadAddress, loadAddress)
		fmt.Printf("🚀 Start:  $%04X (%d)\n", startAddress, startAddress)
		if *tapeFile != "" {
			fmt.Printf("📼 Tape:   %s\n", *tapeFile)
		}
		if *diskFile != "" {
			fmt.Printf("💾 Disk:   %s\n", *diskFile)
		}
		if *timeout > 0 {
			fmt.Printf("⏰ Timeout: %d T-states\n", *timeout)
		}
		fmt.Println()
	}

	// Read the binary file
	binary, err := os.ReadFile(binaryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading binary file: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("📦 Loaded %d bytes\n", len(binary))
	}

	// Create Z80 emulator with hook support
	z80 := emulator.NewZ80WithScreen()
	
	// Load binary into memory at specified address
	z80.LoadAt(loadAddress, binary)

	// TODO: Implement interactive input handling per platform

	// Set up platform-specific system call interceptions
	switch platformTarget {
	case "spectrum":
		setupSpectrumHooks(z80, *verbose)
	case "cpm":
		setupCPMHooks(z80, *verbose)
	case "cpc":
		setupCPCHooks(z80, *verbose)
	}


	// Set start address and begin execution
	z80.PC = startAddress
	
	if *verbose {
		fmt.Printf("▶️  Starting execution at $%04X...\n", startAddress)
		fmt.Println("----------------------------------------")
	}

	// Execute until program ends or timeout
	var totalCycles uint32
	maxCycles := uint32(*timeout)
	
	// 50Hz interrupt simulation (3.5MHz Z80 = 70,000 T-states per interrupt)
	const INTERRUPT_PERIOD = 70000
	var nextInterrupt uint32 = INTERRUPT_PERIOD
	
	for {
		// Check for timeout
		if maxCycles > 0 && totalCycles >= maxCycles {
			fmt.Printf("\n🚨 Safety stop after %dM T-states\n", totalCycles/1000000)
			break
		}
		
		// Check for CP/M BDOS call (PC = 0x0005)
		if platformTarget == "cpm" && z80.PC == 0x0005 {
			handleBDOSCall(z80, *verbose)
			// Return from BDOS call - pop return address from stack
			lowByte := z80.Z80.ReadMemory(z80.SP)
			highByte := z80.Z80.ReadMemory(z80.SP + 1)
			z80.PC = uint16(lowByte) | (uint16(highByte) << 8)
			z80.SP += 2
			continue
		}
		
		// Check if we hit a RET at the start level (simple end detection)  
		if z80.ReadMemory(z80.PC) == 0xC9 && z80.SP == 0xFFFF {
			if *verbose {
				fmt.Printf("\n🏁 Program ended with RET at $%04X\n", z80.PC)
			}
			break
		}
		
		// Check for 50Hz interrupt (before executing instruction)
		if *interrupts && totalCycles >= nextInterrupt && !z80.Z80.IsHalted() {
			// Only interrupt if interrupts are enabled (IFF1)
			if z80.Z80.GetIFF1() {
				if *verbose {
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
		
		// Execute one instruction using hook-enabled execution
		output, cyclesUsed := z80.ExecuteWithHooks(z80.PC)
		totalCycles += cyclesUsed
		
		// Print any output (though RST hooks handle this)
		if output != "" && *verbose {
			fmt.Print(output)
		}
		
		// Check if CPU is halted
		if z80.Z80.IsHalted() {
			if *verbose {
				fmt.Printf("\n🛑 CPU halted at PC=$%04X\n", z80.PC)
			}
			break
		}
		
		// Safety check - prevent infinite loops
		if totalCycles > 10000000 { // 10M cycles
			fmt.Printf("\n🚨 Safety stop after 10M T-states\n")
			break
		}
	}
	
	if *verbose {
		fmt.Println("----------------------------------------")
	}
	
	if *cycles {
		fmt.Printf("⏱️  Total execution: %d T-states\n", totalCycles)
	}
	
	if *verbose {
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
}

// setupCPMHooks configures CP/M BDOS system call interceptions
func setupCPMHooks(z80 *emulator.Z80WithScreen, verbose bool) {
	// CP/M uses CALL 5 for BDOS functions
	// We need to hook when PC reaches address 5
	
	// Set up memory at address 5 to contain a special instruction we can detect
	// In real CP/M, address 5 contains a JP instruction to the BDOS
	z80.WriteMemory(0x0005, 0xED) // Use an ED prefix (unused opcode combo) as marker
	z80.WriteMemory(0x0006, 0xFF) // Unused opcode - we'll detect this
	
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
	function := z80.C // BDOS function number in C register
	
	switch function {
	case 0: // System reset/exit
		if verbose {
			fmt.Printf("🖥️  BDOS 0 (EXIT): Program terminated\n")
		}
		z80.Z80.SetHalted(true) // Halt the CPU
		
	case 2: // Console output
		char := z80.E
		if verbose {
			fmt.Printf("🖥️  BDOS 2 (CONOUT): '%c' (0x%02X)\n", printableChar(char), char)
		}
		fmt.Printf("%c", char)
		
	case 9: // Print string (DE = address)
		addr := uint16(z80.D)<<8 | uint16(z80.E)
		if verbose {
			fmt.Printf("🖥️  BDOS 9 (PRINTS): string at $%04X\n", addr)
		}
		// Print string until $ terminator
		for {
			ch := z80.ReadMemory(addr)
			if ch == '$' {
				break
			}
			fmt.Printf("%c", ch)
			addr++
		}
		
	default:
		if verbose {
			fmt.Printf("🖥️  BDOS function %d not implemented\n", function)
		}
	}
}