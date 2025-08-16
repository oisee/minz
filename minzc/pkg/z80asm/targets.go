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
)

// TargetConfig represents a specific Z80-based platform configuration
type TargetConfig struct {
	Name         string
	Description  string
	MemoryLayout MemoryLayout
	OutputFormat OutputFormat
	Conventions  PlatformConventions
	Extensions   map[string]interface{}
}

// MemoryLayout defines platform-specific memory organization
type MemoryLayout struct {
	DefaultOrigin uint16
	RAMStart      uint16
	RAMSize       uint16
	ROMStart      uint16
	ROMSize       uint16
	ScreenBase    uint16
	StackTop      uint16
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
	CommonSymbols  map[string]uint16
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
			CommonSymbols: map[string]uint16{},
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
			CommonSymbols: map[string]uint16{
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
			CommonSymbols: map[string]uint16{
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
			CommonSymbols: map[string]uint16{
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
			CommonSymbols: map[string]uint16{
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

	// Add platform-specific symbols
	for symbol, addr := range config.Conventions.CommonSymbols {
		// Store symbol in the format expected by the assembler
		symbolName := symbol
		if !a.CaseSensitive {
			symbolName = strings.ToUpper(symbol)
		}
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
	codeEnd := a.origin + uint16(len(a.output))
	memoryLimit := layout.RAMStart + layout.RAMSize
	
	// Handle overflow in limit calculation
	if layout.RAMStart > memoryLimit { // Overflow occurred
		memoryLimit = 0xFFFF
	}
	
	if codeEnd > memoryLimit {
		return fmt.Errorf("code exceeds available RAM (ends at $%04X, limit $%04X)",
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

// Output format generators

// generateBinaryFile creates a raw binary file
func generateBinaryFile(result *Result) ([]byte, error) {
	return result.Binary, nil
}

// generateSNASnapshot creates a ZX Spectrum .SNA snapshot file
func generateSNASnapshot(result *Result) ([]byte, error) {
	// SNA format: 27-byte header + 49152 bytes of memory ($4000-$FFFF)
	snapshot := make([]byte, 49179)
	
	// SNA Header (27 bytes) - basic register state
	snapshot[0] = 0x3F          // I register
	snapshot[1] = 0x58          // HL'
	snapshot[2] = 0x52          // HL'
	snapshot[3] = 0x00          // DE'
	snapshot[4] = 0x00          // DE'
	snapshot[5] = 0x00          // BC'
	snapshot[6] = 0x00          // BC'
	snapshot[7] = 0x00          // AF'
	snapshot[8] = 0x00          // AF'
	snapshot[9] = byte(result.Origin)       // HL
	snapshot[10] = byte(result.Origin >> 8) // HL
	snapshot[11] = 0x00         // DE
	snapshot[12] = 0x00         // DE
	snapshot[13] = 0x00         // BC
	snapshot[14] = 0x00         // BC
	snapshot[15] = 0x00         // IY
	snapshot[16] = 0x00         // IY
	snapshot[17] = 0x00         // IX
	snapshot[18] = 0x00         // IX
	snapshot[19] = 0x00         // IFF2 (bit 2), others 0
	snapshot[20] = 0x00         // R register
	snapshot[21] = 0x00         // AF
	snapshot[22] = 0x00         // AF
	snapshot[23] = 0xFF         // SP
	snapshot[24] = 0xFF         // SP
	snapshot[25] = 0x01         // Interrupt mode (1)
	snapshot[26] = 0x07         // Border color (white)
	
	// Memory content (48K from $4000 to $FFFF)
	if result.Origin >= 0x4000 {
		// Copy binary at the correct offset within the 48K memory space
		offset := 27 + int(result.Origin - 0x4000)
		copy(snapshot[offset:], result.Binary)
	}
	
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

// Add target field to Assembler struct (this would go in assembler.go)
// target *TargetConfig
// warnings []string