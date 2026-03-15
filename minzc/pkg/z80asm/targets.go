package z80asm

import (
	"fmt"
	"strings"
)

// Target represents a specific Z80-based platform
type Target string

const (
	TargetGeneric    Target = "generic"    // Default Z80
	TargetZXSpectrum Target = "zxspectrum" // ZX Spectrum 48K/128K
	TargetZXTap      Target = "zxtap"      // ZX Spectrum .tap files
	TargetCPM        Target = "cpm"        // CP/M systems
	TargetMSX        Target = "msx"        // MSX computers
	TargetGameBoy    Target = "gameboy"    // Game Boy (Z80-like)
	TargetAgonLight2 Target = "agon"       // Agon Light 2 (eZ80)
)

// TargetConfig represents a specific Z80-based platform configuration
type TargetConfig struct {
	Name         string
	Description  string
	CPUMode      CPUMode // CPU mode: Z80, eZ80 Z80-mode, or eZ80 ADL-mode
	MemoryLayout MemoryLayout
	OutputFormat OutputFormat
	Conventions  PlatformConventions
	Extensions   map[string]interface{}
}

// MemoryLayout defines platform-specific memory organization
type MemoryLayout struct {
	DefaultOrigin int
	RAMStart      int
	RAMSize       int
	ROMStart      int
	ROMSize       int
	ScreenBase    int
	StackTop      int
}

// OutputFormat defines how to generate platform-specific output files
type OutputFormat struct {
	Extension    string
	Description  string
	HeaderSize   int
	Loader       bool
	Compression  bool
	Generator    func(*Result) ([]byte, error)
}

// PlatformConventions defines platform-specific symbols and conventions
type PlatformConventions struct {
	CallConvention string
	RegisterUsage  map[string]string
	CommonSymbols  map[string]int
}

// targetRegistry contains all supported platform configurations
var targetRegistry = map[Target]*TargetConfig{
	TargetGeneric: {
		Name:        "Generic Z80",
		Description: "Generic Z80 processor without platform-specific features",
		MemoryLayout: MemoryLayout{
			DefaultOrigin: 0x8000,
			RAMStart:      0x0000,
			RAMSize:       0xFFFF, // 64K - 1 to avoid overflow
			StackTop:      0xFFFF,
		},
		OutputFormat: OutputFormat{
			Extension:   ".bin",
			Description: "Raw binary file",
			Generator:   generateBinaryFile,
		},
		Conventions: PlatformConventions{
			CommonSymbols: map[string]int{},
		},
	},

	TargetZXSpectrum: {
		Name:        "ZX Spectrum",
		Description: "Sinclair ZX Spectrum 48K/128K computers",
		MemoryLayout: MemoryLayout{
			DefaultOrigin: 0x8000,    // Above screen/system area
			RAMStart:      0x4000,    // User RAM start (above ROM)
			RAMSize:       49152,     // 48K - system area
			ROMStart:      0x0000,    // ROM area
			ROMSize:       16384,     // 16K ROM
			ScreenBase:    0x4000,    // Screen memory start
			StackTop:      0xFFFF,    // Top of memory
		},
		OutputFormat: OutputFormat{
			Extension:   ".sna",
			Description: "ZX Spectrum snapshot",
			HeaderSize:  27,
			Generator:   generateSNASnapshot,
		},
		Conventions: PlatformConventions{
			CallConvention: "Standard Z80",
			RegisterUsage: map[string]string{
				"IX": "System use - avoid",
				"IY": "System use - avoid", 
				"I":  "Interrupt mode",
			},
			CommonSymbols: map[string]int{
				"ROM_CLS":       0x0DAF,  // Clear screen routine
				"ROM_PRINT":     0x203C,  // Print string routine
				"ROM_PRINT_A":   0x2B7E,  // Print character in A
				"SCREEN_BASE":   0x4000,  // Screen memory start
				"ATTR_BASE":     0x5800,  // Attribute memory start
				"UDG_BASE":      0xFF58,  // User defined graphics
				"RAMTOP":        0x5CB2,  // System variable: RAM top
				"BORDCR":        0x5C48,  // Border color
			},
		},
	},

	TargetZXTap: {
		Name:        "ZX Spectrum TAP",
		Description: "ZX Spectrum .tap tape files",
		MemoryLayout: MemoryLayout{
			DefaultOrigin: 0x8000,    // Above screen/system area
			RAMStart:      0x4000,    // User RAM start (above ROM)
			RAMSize:       49152,     // 48K - system area
			ROMStart:      0x0000,    // ROM area
			ROMSize:       16384,     // 16K ROM
			ScreenBase:    0x4000,    // Screen memory start
			StackTop:      0xFFFF,    // Top of memory
		},
		OutputFormat: OutputFormat{
			Extension:   ".tap",
			Description: "ZX Spectrum tape file",
			HeaderSize:  0,           // Variable header
			Generator:   generateTAPFile,
		},
		Conventions: PlatformConventions{
			CallConvention: "Standard Z80",
			RegisterUsage: map[string]string{
				"IX": "System use - avoid",
				"IY": "System use - avoid", 
				"I":  "Interrupt mode",
			},
			CommonSymbols: map[string]int{
				"ROM_CLS":       0x0DAF,  // Clear screen routine
				"ROM_PRINT":     0x203C,  // Print string routine
				"ROM_PRINT_A":   0x2B7E,  // Print character in A
				"SCREEN_BASE":   0x4000,  // Screen memory start
				"ATTR_BASE":     0x5800,  // Attribute memory start
				"UDG_BASE":      0xFF58,  // User defined graphics
				"RAMTOP":        0x5CB2,  // System variable: RAM top
				"BORDCR":        0x5C48,  // Border color
			},
		},
	},

	TargetCPM: {
		Name:        "CP/M",
		Description: "CP/M 2.2 Operating System",
		MemoryLayout: MemoryLayout{
			DefaultOrigin: 0x0100,    // CP/M Transient Program Area start
			RAMStart:      0x0100,    // TPA start
			RAMSize:       64256,     // ~62K available (depends on system)
			ROMStart:      0x0000,    // Boot/BIOS area
			ROMSize:       256,       // System area
			StackTop:      0xFEFF,    // Below BDOS
		},
		OutputFormat: OutputFormat{
			Extension:   ".com",
			Description: "CP/M command file",
			Generator:   generateCOMFile,
		},
		Conventions: PlatformConventions{
			CallConvention: "CP/M standard",
			RegisterUsage: map[string]string{
				"C": "BDOS function number",
				"DE": "Parameter for BDOS calls",
			},
			CommonSymbols: map[string]int{
				"BDOS":         0x0005,  // BDOS entry point
				"WBOOT":        0x0000,  // Warm boot
				"FCB":          0x005C,  // Default File Control Block
				"FCB2":         0x006C,  // Second FCB
				"DMA_BUFFER":   0x0080,  // Default DMA buffer
				"CMD_TAIL":     0x0080,  // Command tail
				// BDOS functions
				"BDOS_TERMINATE": 0,     // Program termination
				"BDOS_CONIN":    1,      // Console input
				"BDOS_CONOUT":   2,      // Console output
				"BDOS_PRINT":    9,      // Print string
			},
		},
	},

	TargetMSX: {
		Name:        "MSX",
		Description: "MSX computers and compatibles",
		MemoryLayout: MemoryLayout{
			DefaultOrigin: 0x8000,    // Cartridge slot 1
			RAMStart:      0x8000,    // RAM/cartridge area
			RAMSize:       32768,     // Slot dependent
			ROMStart:      0x0000,    // BIOS/BASIC ROM
			ROMSize:       32768,     // System ROM
			StackTop:      0xF37F,    // Below system work area
		},
		OutputFormat: OutputFormat{
			Extension:   ".rom",
			Description: "MSX cartridge ROM",
			HeaderSize:  16,          // ROM header
			Generator:   generateMSXROM,
		},
		Conventions: PlatformConventions{
			CallConvention: "MSX BIOS",
			CommonSymbols: map[string]int{
				"CHPUT":        0x00A2,  // Character output
				"CHGET":        0x009F,  // Character input
				"INITXT":       0x006C,  // Initialize screen 0
				"INIT32":       0x006F,  // Initialize screen 1
				"INIGRP":       0x0072,  // Initialize screen 2
				"DISSCR":       0x0041,  // Disable screen
				"ENASCR":       0x0044,  // Enable screen
				"WRTVDP":       0x0047,  // Write VDP register
				"RDVRM":        0x004A,  // Read VRAM
				"WRTVRM":       0x004D,  // Write VRAM
			},
		},
	},

	TargetAgonLight2: {
		Name:        "Agon Light 2",
		Description: "Agon Light 2 computer with eZ80 CPU (24-bit ADL mode)",
		CPUMode:     CPUModeEZ80ADL,
		MemoryLayout: MemoryLayout{
			DefaultOrigin: 0x040000,  // MOS loads executables at 0x040000 in ADL mode
			RAMStart:      0x040000,  // User RAM in ADL mode (0x040000-0x0BDFFF)
			RAMSize:       0x07E000,  // ~504K user RAM (0x040000-0x0BDFFF)
			ROMStart:      0x0000,    // MOS in flash (0x000000-0x01FFFF)
			ROMSize:       0,         // No user-accessible ROM area
			StackTop:      0x0000,    // Set by MOS at 0x0BFFFF (grows down)
		},
		OutputFormat: OutputFormat{
			Extension:   ".bin",
			Description: "Agon MOS executable",
			Generator:   generateAgonBin,
		},
		Conventions: PlatformConventions{
			CallConvention: "eZ80 ADL mode",
			RegisterUsage: map[string]string{
				"IX": "Available for user code",
				"IY": "System use - MOS sysvars pointer",
			},
			CommonSymbols: map[string]int{
				// MOS API - RST vector entry points
				"MOS_INIT":       0x0000,  // RST 0x00 - System reset
				"MOS_GETKEY":     0x0000,  // mos_getkey - wait for keypress
				"MOS_SYSVARS":    0x0008,  // RST 0x08 - Get sysvars pointer
				"MOS_PUTCHAR":    0x0010,  // RST 0x10 - Output character
				"MOS_PUTS":       0x0018,  // RST 0x18 - Print string (HL)
				"MOS_EDITLINE":   0x0020,  // RST 0x20 - Edit line input
				// MOS API function numbers (for RST 0x08 + function in A)
				"MOS_GETKEY_FN":     0x00, // Function: get keyboard key
				"MOS_LOAD_FN":       0x01, // Function: load file
				"MOS_SAVE_FN":       0x02, // Function: save file
				"MOS_CD_FN":         0x03, // Function: change directory
				"MOS_DIR_FN":        0x04, // Function: list directory
				"MOS_DEL_FN":        0x05, // Function: delete file
				"MOS_REN_FN":        0x06, // Function: rename file
				"MOS_MKDIR_FN":      0x07, // Function: make directory
				"MOS_SYSVARS_FN":    0x08, // Function: get sysvars
				"MOS_EDITLINE_FN":   0x09, // Function: edit line
				"MOS_FOPEN_FN":      0x0A, // Function: open file
				"MOS_FCLOSE_FN":     0x0B, // Function: close file
				"MOS_FGETC_FN":      0x0C, // Function: get char from file
				"MOS_FPUTC_FN":      0x0D, // Function: put char to file
				"MOS_FEOF_FN":       0x0E, // Function: check end of file
				"MOS_GETERROR_FN":   0x0F, // Function: get last error
				"MOS_OSCLI_FN":      0x10, // Function: execute CLI command
				"MOS_COPY_FN":       0x11, // Function: copy file
				"MOS_GETRTC_FN":     0x12, // Function: get RTC time
				"MOS_SETRTC_FN":     0x13, // Function: set RTC time
				"MOS_SETINTVECTOR_FN": 0x14, // Function: set interrupt vector
				"MOS_UOPEN_FN":      0x15, // Function: open UART
				"MOS_UCLOSE_FN":     0x16, // Function: close UART
				"MOS_UGETC_FN":      0x17, // Function: get char from UART
				"MOS_UPUTC_FN":      0x18, // Function: put char to UART
				"MOS_GETFIL_FN":     0x19, // Function: get file info
				"MOS_FREAD_FN":      0x1A, // Function: read from file
				"MOS_FWRITE_FN":     0x1B, // Function: write to file
				"MOS_FLSEEK_FN":     0x1C, // Function: seek in file
				// VDP commands (sent via serial)
				"VDP_PACKET":    0x17,     // VDP command packet start
			},
		},
		Extensions: map[string]interface{}{
			"cpuMode":     "eZ80ADL",  // 24-bit ADL mode
			"adlMode":     true,       // Use 24-bit addressing
			"clockSpeed":  18432000,   // 18.432 MHz
			"vdpSerial":   true,       // VDP via serial to ESP32
		},
	},
}

// GetTargetConfig returns the configuration for a specific target
func GetTargetConfig(target Target) *TargetConfig {
	config, exists := targetRegistry[target]
	if !exists {
		return nil
	}
	return config
}

// ListTargets returns all available target names
func ListTargets() []string {
	var targets []string
	for target := range targetRegistry {
		targets = append(targets, string(target))
	}
	return targets
}

// ParseTarget parses a target string and returns the Target type
func ParseTarget(targetStr string) (Target, error) {
	target := Target(strings.ToLower(targetStr))
	if GetTargetConfig(target) == nil {
		return "", fmt.Errorf("unknown target: %s", targetStr)
	}
	return target, nil
}

// SetTarget configures the assembler for a specific target platform
func (a *Assembler) SetTarget(target Target) error {
	config := GetTargetConfig(target)
	if config == nil {
		return fmt.Errorf("unknown target: %s", target)
	}

	a.target = config
	a.origin = config.MemoryLayout.DefaultOrigin

	// Set CPU mode from target config (e.g., eZ80 ADL for Agon)
	if config.CPUMode != CPUModeZ80 {
		a.SetCPUMode(config.CPUMode)
	}

	// Add platform-specific symbols
	for symbol, addr := range config.Conventions.CommonSymbols {
		// Store symbol in the format expected by the assembler
		symbolName := a.symbolKey(symbol)
		a.symbols[symbolName] = &Symbol{
			Name:    symbolName,
			Value:   addr,
			Defined: true,
		}
	}

	return nil
}

// ValidateMemoryLayout checks if the assembled code fits within platform constraints
func (a *Assembler) ValidateMemoryLayout() error {
	if a.target == nil {
		return nil // Generic target - no validation
	}

	layout := a.target.MemoryLayout
	
	// Check if code fits in available memory
	codeEnd := a.origin + len(a.output)
	memoryLimit := layout.RAMStart + layout.RAMSize

	if codeEnd > memoryLimit {
		return fmt.Errorf("code exceeds available RAM (ends at $%06X, limit $%06X)",
			codeEnd, memoryLimit)
	}
	
	// Platform-specific warnings
	switch a.target.Name {
	case "ZX Spectrum":
		if a.origin < 0x5B00 && codeEnd > 0x4000 {
			warning := "Code overlaps with screen memory ($4000-$5AFF)"
			a.warnings = append(a.warnings, warning)
		}
		if a.origin < 0x8000 {
			warning := "Code below $8000 may conflict with BASIC/system"
			a.warnings = append(a.warnings, warning)
		}
		
	case "CP/M":
		if a.origin != 0x0100 {
			warning := fmt.Sprintf("CP/M programs typically start at $0100, not $%04X", a.origin)
			a.warnings = append(a.warnings, warning)
		}
		if codeEnd > 0xFE00 {
			warning := "Code may conflict with BDOS area"
			a.warnings = append(a.warnings, warning)
		}
	}
	
	return nil
}

// ── Standalone output formats ─────────────────────────────────────────────────
// These are independent of the target platform. The target provides symbols,
// ORG, and stdlib; the format controls how the assembled binary is packaged.

// OutputFormats maps format names to their generators.
var OutputFormats = map[string]OutputFormat{
	"code": {
		Extension:   ".code",
		Description: "Raw binary (CODE block) — just the assembled bytes",
		Generator:   generateBinaryFile,
	},
	"sna": {
		Extension:   ".sna",
		Description: "ZX Spectrum 48K .SNA snapshot (for emulators)",
		HeaderSize:  27,
		Generator:   generateSNASnapshot,
	},
	"tap": {
		Extension:   ".tap",
		Description: "ZX Spectrum .TAP tape file (LOAD \"\" CODE)",
		Generator:   generateTAPFile,
	},
	"com": {
		Extension:   ".com",
		Description: "CP/M .COM executable",
		Generator:   generateCOMFile,
	},
	"msxrom": {
		Extension:   ".rom",
		Description: "MSX cartridge ROM",
		Generator:   generateMSXROM,
	},
	"agon": {
		Extension:   ".bin",
		Description: "Agon Light 2 MOS executable",
		Generator:   generateAgonBin,
	},
}

// LookupOutputFormat returns the OutputFormat for a given name, or nil.
func LookupOutputFormat(name string) *OutputFormat {
	if f, ok := OutputFormats[name]; ok {
		return &f
	}
	return nil
}

// Output format generators

// generateBinaryFile creates a raw binary file
func generateBinaryFile(result *Result) ([]byte, error) {
	return result.Binary, nil
}

// generateSNASnapshot creates a ZX Spectrum .SNA snapshot file
func generateSNASnapshot(result *Result) ([]byte, error) {
	// SNA format: 27-byte header + 49152 bytes of memory ($4000-$FFFF)
	snapshot := make([]byte, 49179)

	// Use EntryPoint for execution start (from END directive or first ORG)
	// Falls back to Origin if no explicit entry point was set
	entryPoint := result.EntryPoint
	if entryPoint == 0 {
		entryPoint = result.Origin
	}

	// SNA Format: PC is stored on the stack and loaded via RETN
	// We set SP to $FFFC and store the start address at $FFFC-$FFFD
	stackAddr := uint16(0xFFFC)

	// SNA Header (27 bytes) - register state
	snapshot[0] = 0x3F          // I register
	snapshot[1] = 0x58          // HL' (low)
	snapshot[2] = 0x52          // HL' (high)
	snapshot[3] = 0x00          // DE' (low)
	snapshot[4] = 0x00          // DE' (high)
	snapshot[5] = 0x00          // BC' (low)
	snapshot[6] = 0x00          // BC' (high)
	snapshot[7] = 0x00          // AF' (low = F')
	snapshot[8] = 0x00          // AF' (high = A')
	snapshot[9] = 0x00          // HL (low)
	snapshot[10] = 0x00         // HL (high)
	snapshot[11] = 0x00         // DE (low)
	snapshot[12] = 0x00         // DE (high)
	snapshot[13] = 0x00         // BC (low)
	snapshot[14] = 0x00         // BC (high)
	snapshot[15] = 0x00         // IY (low)
	snapshot[16] = 0x5C         // IY (high) - $5C00 is common for ZX Spectrum
	snapshot[17] = 0x00         // IX (low)
	snapshot[18] = 0x00         // IX (high)
	snapshot[19] = 0x04         // IFF2 (bit 2 = interrupts enabled)
	snapshot[20] = 0x00         // R register
	snapshot[21] = 0x00         // AF (low = F)
	snapshot[22] = 0x00         // AF (high = A)
	snapshot[23] = byte(stackAddr)      // SP (low)
	snapshot[24] = byte(stackAddr >> 8) // SP (high)
	snapshot[25] = 0x01         // Interrupt mode (1)
	snapshot[26] = 0x07         // Border color (white)

	// Memory content (48K from $4000 to $FFFF)
	if result.Origin >= 0x4000 {
		// Copy binary at the correct offset within the 48K memory space
		offset := 27 + int(result.Origin - 0x4000)
		copy(snapshot[offset:], result.Binary)
	}

	// Store the entry point at SP location for RETN to pop
	// Stack grows downward, so PC is at SP (low byte) and SP+1 (high byte)
	// Offset in snapshot: 27 + (stackAddr - 0x4000)
	pcOffset := 27 + int(stackAddr - 0x4000)
	snapshot[pcOffset] = byte(entryPoint)        // PC low byte
	snapshot[pcOffset+1] = byte(entryPoint >> 8) // PC high byte

	return snapshot, nil
}

// generateCOMFile creates a CP/M .COM executable file
func generateCOMFile(result *Result) ([]byte, error) {
	// CP/M .COM files are raw binary starting at $0100
	if result.Origin != 0x0100 {
		return nil, fmt.Errorf("CP/M programs must start at $0100, got $%04X", result.Origin)
	}
	
	// Maximum CP/M program size is ~62K
	if len(result.Binary) > 64000 {
		return nil, fmt.Errorf("CP/M program too large: %d bytes (max ~64000)", len(result.Binary))
	}
	
	return result.Binary, nil
}

// generateMSXROM creates an MSX cartridge ROM file
func generateMSXROM(result *Result) ([]byte, error) {
	// MSX ROM requires specific header and size alignment
	minSize := 8192 // Minimum 8K
	maxSize := 32768 // Maximum 32K for simple cartridge
	
	if len(result.Binary) > maxSize {
		return nil, fmt.Errorf("MSX ROM too large: %d bytes (max %d)", len(result.Binary), maxSize)
	}
	
	// Round up to next power of 2
	romSize := minSize
	for romSize < len(result.Binary) {
		romSize *= 2
	}
	
	rom := make([]byte, romSize)
	copy(rom, result.Binary)
	
	// Add MSX ROM header if starting at $8000
	if result.Origin == 0x8000 && len(rom) >= 16 {
		rom[0] = 'A'          // ROM header signature
		rom[1] = 'B'          // ROM header signature  
		rom[2] = byte(result.Origin)       // Init address low
		rom[3] = byte(result.Origin >> 8)  // Init address high
		// Other header bytes can be customized
	}
	
	return rom, nil
}

// generateTAPFile creates a ZX Spectrum .TAP tape file
func generateTAPFile(result *Result) ([]byte, error) {
	// TAP format consists of blocks with length headers
	var tap []byte
	
	// Create a CODE block (machine code)
	// TAP format: [length_lo][length_hi][data...]
	
	// Header block first (17 bytes + 2 byte length)
	headerData := make([]byte, 17)
	headerData[0] = 0x00    // Header flag
	headerData[1] = 0x03    // CODE file type
	
	// Filename (10 bytes, padded with spaces)
	filename := "PROGRAM   "
	copy(headerData[2:12], []byte(filename))
	
	// Data length (little-endian)
	dataLen := uint16(len(result.Binary))
	headerData[12] = byte(dataLen)
	headerData[13] = byte(dataLen >> 8)
	
	// Start address (little-endian)
	headerData[14] = byte(result.Origin)
	headerData[15] = byte(result.Origin >> 8)
	
	// Unused parameter for CODE blocks (same as data length)
	headerData[16] = byte(dataLen)
	
	// Calculate header checksum
	headerChecksum := byte(0)
	for _, b := range headerData {
		headerChecksum ^= b
	}
	
	// Add header block to TAP
	headerBlockLen := uint16(18) // 17 bytes + 1 checksum
	tap = append(tap, byte(headerBlockLen), byte(headerBlockLen>>8))
	tap = append(tap, headerData...)
	tap = append(tap, headerChecksum)
	
	// Data block
	dataBlock := make([]byte, len(result.Binary)+2)
	dataBlock[0] = 0xFF // Data flag
	copy(dataBlock[1:], result.Binary)
	
	// Calculate data checksum
	dataChecksum := byte(0)
	for _, b := range dataBlock[:len(dataBlock)-1] {
		dataChecksum ^= b
	}
	dataBlock[len(dataBlock)-1] = dataChecksum
	
	// Add data block to TAP
	dataBlockLen := uint16(len(dataBlock))
	tap = append(tap, byte(dataBlockLen), byte(dataBlockLen>>8))
	tap = append(tap, dataBlock...)
	
	return tap, nil
}

// generateAgonBin creates an Agon Light 2 MOS executable binary with proper header
// Binary format documented in inbox/agon-mos-binary-format.md
func generateAgonBin(result *Result) ([]byte, error) {
	// Agon MOS executables require a 69-byte header
	// MOS loads them at 0x040000 (in ADL mode address space)
	// Code starts at offset 0x45 within the binary

	// Maximum size check (512KB addressable, but practical limit ~480KB)
	if len(result.Binary) > 480*1024 {
		return nil, fmt.Errorf("Agon binary too large: %d bytes (max ~480KB)", len(result.Binary))
	}

	// Build the 69-byte MOS header
	header := make([]byte, 0, 0x45)

	// 0x00-0x03: JP 0x040045 (24-bit JP in ADL mode, jump over header to code)
	// MOS loads executables at 0x040000, code starts at header offset 0x45
	// In ADL mode, JP is 4 bytes: C3 + 24-bit address (little-endian)
	header = append(header, 0xC3, 0x45, 0x00, 0x04)

	// 0x04: Length of program name (including null terminator)
	// Use a default name "program.bin" if none specified
	progName := "program.bin"
	nameLen := len(progName) + 1 // +1 for null terminator
	header = append(header, byte(nameLen))

	// 0x05-N: Program name with null terminator
	header = append(header, []byte(progName)...)
	header = append(header, 0x00) // null terminator

	// Pad with 0xFF to reach offset 0x40
	for len(header) < 0x40 {
		header = append(header, 0xFF)
	}

	// 0x40-0x42: "MOS" magic marker
	header = append(header, 'M', 'O', 'S')

	// 0x43: Header version (always 0)
	header = append(header, 0x00)

	// 0x44: ADL mode flag (0x01 = 24-bit ADL mode, 0x00 = Z80 mode)
	header = append(header, 0x01) // ADL mode enabled

	// Verify header is exactly 0x45 bytes
	if len(header) != 0x45 {
		return nil, fmt.Errorf("internal error: MOS header is %d bytes, expected 69", len(header))
	}

	// Combine header + machine code
	output := make([]byte, 0, len(header)+len(result.Binary))
	output = append(output, header...)
	output = append(output, result.Binary...)

	return output, nil
}

// Add target field to Assembler struct (this would go in assembler.go)
// target *TargetConfig
// warnings []string