# ADR-003: Target/Device Support Framework

## Status
**ACCEPTED** - Implementation in progress

## Context

### Current Problem
MZA currently only supports generic Z80 assembly without target-specific optimizations or device-specific features:

```bash
# Generic assembly - misses platform opportunities
./mza program.a80 -o program.bin

# What we want - platform awareness
./mza program.a80 --target=zxspectrum -o program.sna
./mza program.a80 --target=cpm -o program.com  
./mza program.a80 --target=msx -o program.rom
```

### Impact Analysis
Target-specific support enables:
- **Output format selection** - `.sna`, `.tap`, `.com`, `.rom` based on target
- **Memory layout optimization** - Platform-specific memory maps
- **Device-specific features** - Screen layouts, sound chips, I/O ports
- **Better developer experience** - Platform conventions and best practices

### Success Rate Impact
Current **12%** could improve to **~15%** (+3%) by providing platform-aware assembly and reducing configuration errors.

## Decision

### Implement Flexible Target System
Create a framework supporting multiple Z80-based platforms with:

1. **Target Detection** - Auto-detect or explicit target specification
2. **Memory Layout Management** - Platform-specific memory maps
3. **Output Format Support** - Target-appropriate file formats
4. **Device-Specific Features** - Platform conventions and optimizations

### Supported Targets (Phase 1)
```go
// Core Z80 targets for initial implementation
type Target string

const (
    TargetGeneric    Target = "generic"    // Default Z80
    TargetZXSpectrum Target = "zxspectrum" // ZX Spectrum 48K/128K
    TargetCPM        Target = "cpm"        // CP/M systems
    TargetMSX        Target = "msx"        // MSX computers
    TargetGameBoy    Target = "gameboy"    // Game Boy (Z80-like)
)
```

## Implementation

### 1. Target Configuration System
```go
// Target represents a specific Z80-based platform
type TargetConfig struct {
    Name         string
    Description  string
    MemoryLayout MemoryLayout
    OutputFormat OutputFormat
    Conventions  PlatformConventions
    Extensions   map[string]interface{}
}

type MemoryLayout struct {
    DefaultOrigin uint16
    RAMStart      uint16
    RAMSize       uint16
    ROMStart      uint16
    ROMSize       uint16
    ScreenBase    uint16
    StackTop      uint16
}

type OutputFormat struct {
    Extension    string
    HeaderSize   int
    Loader       bool
    Compression  bool
    Generator    func(*Result) ([]byte, error)
}

type PlatformConventions struct {
    CallConvention string
    RegisterUsage  map[string]string
    CommonSymbols  map[string]uint16
}
```

### 2. Target Registry
```go
var targetRegistry = map[Target]*TargetConfig{
    TargetZXSpectrum: {
        Name:        "ZX Spectrum",
        Description: "Sinclair ZX Spectrum 48K/128K",
        MemoryLayout: MemoryLayout{
            DefaultOrigin: 0x8000,    // Above screen/system
            RAMStart:      0x4000,    // User RAM start
            RAMSize:       49152,     // 48K - system area
            ScreenBase:    0x4000,    // Screen memory
            StackTop:      0xFFFF,    // Top of memory
        },
        OutputFormat: OutputFormat{
            Extension: ".sna",
            Generator: generateSNASnapshot,
        },
        Conventions: PlatformConventions{
            CommonSymbols: map[string]uint16{
                "ROM_CLS":      0x0DAF,  // Clear screen
                "ROM_PRINT":    0x203C,  // Print string
                "SCREEN_BASE":  0x4000,  // Screen start
                "ATTR_BASE":    0x5800,  // Attributes
            },
        },
    },
    
    TargetCPM: {
        Name:        "CP/M",
        Description: "CP/M 2.2 Systems",
        MemoryLayout: MemoryLayout{
            DefaultOrigin: 0x0100,    // CP/M TPA start
            RAMStart:      0x0100,
            RAMSize:       64256,     // ~62K available
            StackTop:      0xFEFF,    // Below BDOS
        },
        OutputFormat: OutputFormat{
            Extension: ".com",
            Generator: generateCOMFile,
        },
        Conventions: PlatformConventions{
            CommonSymbols: map[string]uint16{
                "BDOS":         0x0005,  // BDOS entry
                "WBOOT":        0x0000,  // Warm boot
                "FCB":          0x005C,  // Default FCB
                "DMA_BUFFER":   0x0080,  // Default DMA
            },
        },
    },
}
```

### 3. CLI Integration
```go
// Add target flag to assembler CLI
func main() {
    var (
        targetFlag   = flag.String("target", "generic", "Target platform")
        outputFlag   = flag.String("output", "", "Output file")
        formatFlag   = flag.String("format", "auto", "Output format")
    )
    
    // Parse target
    target := Target(*targetFlag)
    config := GetTargetConfig(target)
    if config == nil {
        log.Fatalf("Unknown target: %s", target)
    }
    
    // Configure assembler
    assembler := NewAssembler()
    assembler.SetTarget(config)
    
    // Determine output format
    outputFormat := *formatFlag
    if outputFormat == "auto" {
        outputFormat = config.OutputFormat.Extension
    }
}
```

### 4. Target-Aware Assembly
```go
// Assembler modifications for target awareness
func (a *Assembler) SetTarget(config *TargetConfig) {
    a.target = config
    a.origin = config.MemoryLayout.DefaultOrigin
    
    // Add platform symbols
    for symbol, addr := range config.Conventions.CommonSymbols {
        a.symbols[symbol] = &Symbol{
            Name:    symbol,
            Value:   addr,
            Defined: true,
        }
    }
}
```

### 5. Output Format Generators
```go
// SNA Snapshot Generator (ZX Spectrum)
func generateSNASnapshot(result *Result) ([]byte, error) {
    snapshot := make([]byte, 49179) // 27 header + 49152 memory
    
    // SNA Header (27 bytes)
    snapshot[0] = 0x3F    // I register
    snapshot[1] = 0x58    // HL'
    snapshot[2] = 0x52    // HL'
    // ... fill other registers
    
    // Memory content (48K from $4000)
    memStart := result.Origin - 0x4000
    copy(snapshot[27:], result.Binary[memStart:])
    
    return snapshot, nil
}

// COM File Generator (CP/M)  
func generateCOMFile(result *Result) ([]byte, error) {
    // CP/M .COM files are raw binary from $0100
    if result.Origin != 0x0100 {
        return nil, fmt.Errorf("CP/M programs must start at $0100")
    }
    
    // Add optional header or loader if needed
    return result.Binary, nil
}
```

### 6. Memory Validation
```go
func (a *Assembler) validateMemoryLayout() error {
    if a.target == nil {
        return nil // Generic target
    }
    
    layout := a.target.MemoryLayout
    
    // Check if code fits in available memory
    codeEnd := a.origin + uint16(len(a.output))
    
    if codeEnd > layout.RAMStart + layout.RAMSize {
        return fmt.Errorf("code exceeds available RAM (ends at $%04X, limit $%04X)",
                         codeEnd, layout.RAMStart + layout.RAMSize)
    }
    
    // Warn about screen memory overlap (ZX Spectrum)
    if a.target.Name == "ZX Spectrum" && a.origin < 0x5B00 && codeEnd > 0x4000 {
        a.warnings = append(a.warnings, 
            "Code overlaps with screen memory ($4000-$5AFF)")
    }
    
    return nil
}
```

## Expected Outcomes

### Before (Current)
```bash
# Generic assembly - limited platform awareness
./mza program.a80 -o program.bin

# Manual conversion needed
# No platform validation
# Generic memory layout assumptions
```

### After (Enhanced)
```bash
# Target-aware assembly
./mza program.a80 --target=zxspectrum -o program.sna
# → Creates ZX Spectrum snapshot with proper header

./mza program.a80 --target=cpm -o program.com  
# → Creates CP/M executable with validation

# Platform symbols available
LD A, (SCREEN_BASE)    ; → Resolves to $4000 on ZX Spectrum
CALL ROM_PRINT         ; → Resolves to $203C on ZX Spectrum

# Memory layout validation
ORG $C000              ; → Warning: overlaps screen on ZX Spectrum
```

## Benefits

### Immediate Impact
- **+3% success rate** - Platform-aware validation and symbols
- **Better output formats** - Native platform files (.sna, .com)
- **Reduced configuration errors** - Platform defaults
- **Enhanced developer experience** - Platform-specific guidance

### Long-term Benefits
- **Ecosystem expansion** - Support more Z80 platforms
- **Community contributions** - Platform maintainers can add targets
- **Tool integration** - Emulators can directly load MZA output
- **Professional appearance** - Industry-standard platform support

## Test Coverage

### Unit Tests
```go
func TestTargetConfig(t *testing.T) {
    config := GetTargetConfig(TargetZXSpectrum)
    assert.NotNil(t, config)
    assert.Equal(t, uint16(0x8000), config.MemoryLayout.DefaultOrigin)
    assert.Equal(t, ".sna", config.OutputFormat.Extension)
}

func TestMemoryValidation(t *testing.T) {
    a := NewAssembler()
    a.SetTarget(GetTargetConfig(TargetZXSpectrum))
    
    // Test memory overflow
    a.origin = 0xF000
    // ... simulate large code
    err := a.validateMemoryLayout()
    assert.Error(t, err)
}
```

### Integration Tests
```bash
# Test ZX Spectrum target
./mza examples/hello_spectrum.a80 --target=zxspectrum -o test.sna
file test.sna  # Should show: ZX Spectrum snapshot

# Test CP/M target  
./mza examples/hello_cpm.a80 --target=cpm -o test.com
xxd test.com | head  # Should start at 0x0100
```

## Implementation Plan

### Phase 1: Core Framework (1 day)
- [ ] Define target configuration structures
- [ ] Implement target registry and lookup
- [ ] Add CLI flags for target selection

### Phase 2: Platform Configs (1 day)  
- [ ] Implement ZX Spectrum target configuration
- [ ] Implement CP/M target configuration
- [ ] Add memory layout validation

### Phase 3: Output Formats (1 day)
- [ ] Implement .SNA snapshot generator
- [ ] Implement .COM file generator  
- [ ] Add format auto-detection

### Phase 4: Integration (0.5 day)
- [ ] Update assembler to use target configurations
- [ ] Add platform symbol definitions
- [ ] Test against corpus for success rate improvement

## Success Metrics

### Quantitative
- **Success rate improvement**: 12% → 15% (+3%)
- **Platform coverage**: ZX Spectrum, CP/M working
- **Output format support**: .sna, .com generation
- **Symbol resolution**: Platform symbols available

### Qualitative
- **Developer experience**: Target-specific help and validation
- **Professional appearance**: Industry-standard platform support
- **Ecosystem readiness**: Foundation for community target additions

## Future Enhancements

### Additional Targets
- **MSX computers** - MSX-DOS, cartridge formats
- **Amstrad CPC** - Tape and disk formats
- **Game Boy** - Z80-compatible cartridge format
- **TI calculators** - Educational/hobbyist platforms

### Advanced Features
- **Target-specific optimizations** - Platform-aware code generation
- **Debug symbol support** - Debugger integration
- **Cross-platform builds** - Multi-target compilation

This ADR establishes professional target support that makes MZA suitable for real Z80 development across multiple platforms! 🚀