# MinZ Multi-Target Interface Specification

This document provides the complete technical specification for the MinZ multi-target architecture interfaces, including all methods, types, and behavioral contracts.

## 1. Core Type Definitions

### 1.1 Feature Flags

```go
package targets

// FeatureFlag represents a MinZ language or optimization feature
type FeatureFlag uint64

const (
    // Language Features
    FeatureLambdas           FeatureFlag = 1 << iota
    FeatureInterfaces        
    FeatureIterators         
    FeatureMetafunctions     
    FeatureInlineAssembly    
    FeatureModuleSystem      
    FeaturePatternMatching   
    FeatureGenerics          

    // Optimization Features  
    FeatureSMC              // Self-modifying code
    FeatureTrueSMC          // TRUE SMC with instruction patching
    FeatureDJNZOptimization // Z80-style DJNZ loops
    FeatureTailRecursion    
    FeatureInlining         
    FeatureDeadCodeElim     
    FeatureConstantFolding  
    FeatureRegisterAlloc    

    // Hardware Features
    FeatureShadowRegisters  // Z80-style alternate registers
    FeatureBitManipulation  
    FeatureInterrupts       
    FeatureFloatingPoint    
    FeatureVectorOps        
    FeatureDMA              
    FeatureMemoryBanking    

    // I/O Features
    FeaturePrint            
    FeatureFileIO           
    FeatureNetworking       
    FeatureGraphics         
    FeatureSound            
)

// FeatureSet represents a collection of features
type FeatureSet uint64

func (fs FeatureSet) Has(feature FeatureFlag) bool {
    return (uint64(fs) & uint64(feature)) != 0
}

func (fs *FeatureSet) Add(feature FeatureFlag) {
    *fs |= FeatureSet(feature)
}

func (fs *FeatureSet) Remove(feature FeatureFlag) {
    *fs &^= FeatureSet(feature)
}
```

### 1.2 Architecture Properties

```go
// Endianness represents byte order
type Endianness int

const (
    LittleEndian Endianness = iota
    BigEndian
    BiEndian // Supports both (some ARM processors)
)

// AddressingMode represents how memory is accessed
type AddressingMode int

const (
    DirectAddressing     AddressingMode = iota // Absolute addresses
    IndirectAddressing   // Pointer-based
    IndexedAddressing    // Base + offset
    RegisterAddressing   // Register-only
    ImmediateAddressing  // Constants in instructions
    RelativeAddressing   // PC-relative
    StackAddressing      // Stack-based (WASM)
)

// RegisterType categorizes different register kinds
type RegisterType int

const (
    GeneralPurpose RegisterType = iota
    IndexRegister
    AccumulatorRegister
    StackPointer
    ProgramCounter
    StatusRegister
    ShadowRegister
    FloatingPoint
    Vector
)
```

### 1.3 Target Capabilities

```go
// TargetCapabilities describes what a target architecture supports
type TargetCapabilities struct {
    // Basic Architecture
    Name                string      // Human-readable name
    WordSize            int         // Native word size in bits
    AddressSize         int         // Address bus width in bits
    Endianness          Endianness  // Byte order
    MaxMemorySize       uint64      // Maximum addressable memory
    
    // Register System
    RegisterCount       int           // Total number of registers
    RegisterTypes       []RegisterType // Types of registers available
    RegisterSizes       []int         // Available register sizes in bits
    HasIndexRegisters   bool          // IX, IY style registers
    HasShadowRegisters  bool          // Z80-style alternate registers
    
    // Memory Model
    HasSegmentation     bool          // x86-style segmentation
    HasMemoryBanking    bool          // Paging/banking support
    HasMemoryProtection bool          // MMU support
    HasCache            bool          // Cache hierarchy
    ZeroPageSize        int           // 6502-style zero page (0 if none)
    
    // Instruction Set Features
    HasSelfModifyingCode    bool      // SMC support
    HasTrueSMC              bool      // Instruction immediate patching
    HasConditionalBranches  bool      // if/else support
    HasRelativeBranches     bool      // PC-relative jumps
    HasDirectJumps          bool      // Absolute address jumps
    HasIndirectJumps        bool      // Jump through pointer
    HasLoopInstructions     bool      // DJNZ, DBRA style
    HasBitManipulation      bool      // Bit set/clear/test
    HasShiftInstructions    bool      // Shift/rotate operations
    HasMultiplyDivide       bool      // Hardware multiply/divide
    
    // Advanced Features
    HasInterruptSystem      bool      // Interrupt handling
    HasDMASupport          bool      // Direct memory access
    HasFloatingPoint       bool      // FPU support
    HasVectorInstructions  bool      // SIMD operations
    HasAtomicOperations    bool      // Atomic read-modify-write
    
    // Calling Conventions
    ParameterRegisters     []string  // Registers for parameters
    ReturnRegisters        []string  // Registers for return values
    CallerSavedRegisters   []string  // Caller must save these
    CalleeSavedRegisters   []string  // Callee must save these
    StackGrowsDown         bool      // Stack direction
    
    // Assembly Format
    AssemblyDialect        string    // "sjasmplus", "ca65", "vasm", etc.
    CommentPrefix          string    // ";", "//", "#"
    LabelSuffix           string    // ":", "", etc.
    ImmediatePrefix       string    // "#", "$", "", etc.
    
    // Platform Integration
    SupportedPlatforms    []string  // "zx-spectrum", "c64", "amiga", etc.
    DefaultOrigin         uint32    // Default code origin address
    HasBootstrap          bool      // Needs boot code generation
}
```

## 2. Core Interfaces

### 2.1 Target Interface

```go
// Target represents a compilation target (CPU architecture)
type Target interface {
    // Identification
    Name() string                    // Canonical name (e.g., "z80")
    Description() string             // Human description
    Aliases() []string               // Alternative names
    Version() string                 // Target implementation version
    
    // Capabilities
    GetCapabilities() TargetCapabilities
    SupportsFeature(feature FeatureFlag) bool
    GetSupportedFeatures() FeatureSet
    GetRequiredFeatures() FeatureSet    // Features that must be supported
    
    // Configuration
    ValidateConfig(config TargetConfig) error
    GetDefaultConfig() TargetConfig
    
    // MIR Processing
    ValidateMIR(module *ir.Module) error
    TransformMIR(module *ir.Module, config TargetConfig) (*ir.Module, error)
    
    // Code Generation Pipeline
    CreateOptimizer(config OptimizerConfig) (Optimizer, error)
    CreateCodeGenerator(config CodeGenConfig) (CodeGenerator, error)
    CreateLinker(config LinkerConfig) (Linker, error)
    
    // Assembly Format
    GetAssemblyFormat() AssemblyFormat
    GetFileExtension() string           // ".a80", ".s", ".asm"
    GetObjectExtension() string         // ".o", ".obj", ".bin"
    
    // Platform Support
    GetSupportedPlatforms() []Platform
    GetDefaultPlatform() Platform
    
    // Feature Degradation
    HandleUnsupportedFeature(feature FeatureFlag, context FeatureContext) error
}
```

### 2.2 Optimizer Interface

```go
// OptimizationLevel represents different optimization levels
type OptimizationLevel int

const (
    OptimizeNone    OptimizationLevel = iota // -O0: No optimization
    OptimizeSize                             // -Os: Optimize for size
    OptimizeSpeed                            // -O1: Basic optimizations
    OptimizeMore                             // -O2: More optimizations
    OptimizeMax                              // -O3: Maximum optimization
    OptimizeDebug                            // -Og: Debug-friendly optimization
)

// OptimizationPass represents a single optimization pass
type OptimizationPass interface {
    Name() string
    Description() string
    RequiredFeatures() FeatureSet
    OptimizeModule(module *ir.Module) error
    OptimizeFunction(function *ir.Function) error
}

// Optimizer performs target-specific optimizations
type Optimizer interface {
    // Configuration
    SetOptimizationLevel(level OptimizationLevel) error
    GetOptimizationLevel() OptimizationLevel
    SetTargetConfig(config TargetConfig) error
    
    // Pass Management
    GetAvailablePasses() []OptimizationPass
    EnablePass(passName string) error
    DisablePass(passName string) error
    IsPassEnabled(passName string) bool
    
    // Main Optimization
    OptimizeModule(module *ir.Module) error
    
    // Analysis
    AnalyzeModule(module *ir.Module) OptimizationReport
    AnalyzeFunction(function *ir.Function) FunctionAnalysis
    DetectOptimizationOpportunities(module *ir.Module) []Opportunity
    
    // Statistics
    GetOptimizationStats() OptimizationStats
    ResetStats()
}

// OptimizationReport contains analysis results
type OptimizationReport struct {
    TotalFunctions     int
    OptimizedFunctions int
    BytesSaved         int
    CyclesSaved        int64
    Opportunities      []Opportunity
    Warnings           []OptimizationWarning
}
```

### 2.3 CodeGenerator Interface

```go
// CodeGenerator produces assembly/machine code for a target
type CodeGenerator interface {
    // Configuration
    SetOutputFormat(format OutputFormat) error
    SetOrigin(address uint32) error
    SetSymbolPrefix(prefix string) error
    SetTargetConfig(config TargetConfig) error
    
    // Code Generation
    GenerateModule(module *ir.Module) (*GeneratedCode, error)
    GenerateFunction(function *ir.Function) (*GeneratedCode, error)
    GenerateGlobalData(globals []ir.GlobalVariable) (*GeneratedCode, error)
    
    // Assembly Management
    GetAssemblyDialect() string
    ValidateAssembly(assembly []byte) error
    
    // Symbol Management
    GenerateSymbolTable() (*SymbolTable, error)
    ResolveSymbols(symbols *SymbolTable) error
    
    // Debug Information
    GenerateDebugInfo(module *ir.Module) (*DebugInfo, error)
    GenerateSourceMap() (*SourceMap, error)
    
    // Statistics
    GetCodeGenStats() CodeGenStats
}

// GeneratedCode represents the output of code generation
type GeneratedCode struct {
    Assembly      []byte            // Generated assembly code
    MachineCode   []byte            // Binary machine code (if applicable)
    Symbols       *SymbolTable      // Symbol definitions
    Relocations   []Relocation      // Relocation entries
    DebugInfo     *DebugInfo        // Debug information
    SourceMap     *SourceMap        // Source mapping
    Statistics    CodeGenStats      // Generation statistics
}
```

### 2.4 Platform Interface

```go
// Platform represents a specific hardware/software platform
type Platform interface {
    // Identification
    Name() string                   // "zx-spectrum", "c64", "amiga"
    Description() string
    Target() Target                 // Which CPU target this platform uses
    
    // Memory Layout
    GetMemoryLayout() MemoryLayout
    GetDefaultOrigin() uint32
    GetStackLocation() uint32
    GetHeapLocation() uint32
    
    // I/O and Hardware
    GetHardwareRegisters() []HardwareRegister
    GetROMRoutines() []ROMRoutine
    GetInterruptVectors() []InterruptVector
    
    // File Format
    GetExecutableFormat() ExecutableFormat
    GetHeaderGenerator() HeaderGenerator
    
    // Runtime Support
    GetRuntimeLibrary() RuntimeLibrary
    GetStartupCode() []byte
    GetShutdownCode() []byte
}
```

## 3. Configuration Types

### 3.1 Target Configuration

```go
// TargetConfig holds target-specific configuration
type TargetConfig struct {
    // Basic Settings
    Platform          string            // Target platform
    OptimizationLevel OptimizationLevel // Optimization level
    Origin            uint32            // Code origin address
    
    // Feature Flags
    EnabledFeatures   FeatureSet        // Explicitly enabled features
    DisabledFeatures  FeatureSet        // Explicitly disabled features
    
    // Target-Specific Options
    Options           map[string]any    // Target-specific configuration
    
    // Code Generation
    GenerateDebugInfo bool              // Include debug information
    GenerateComments  bool              // Include comments in assembly
    InlineThreshold   int               // Function inlining threshold
    
    // Optimization Options
    OptimizeForSize   bool              // Prioritize size over speed
    EnableSMC         bool              // Allow self-modifying code
    UseShadowRegs     bool              // Use shadow registers (if available)
    
    // Assembly Format
    AssemblyDialect   string            // Assembly syntax variant
    UseUpperCase      bool              // Use uppercase mnemonics
    TabWidth          int               // Tab width for formatting
}

// OptimizerConfig configures the optimizer
type OptimizerConfig struct {
    Level              OptimizationLevel
    EnabledPasses      []string
    DisabledPasses     []string
    InlineThreshold    int
    UnrollThreshold    int
    MaxIterations      int
    TargetConfig       TargetConfig
}

// CodeGenConfig configures code generation
type CodeGenConfig struct {
    OutputFormat       OutputFormat
    Origin             uint32
    SymbolPrefix       string
    GenerateDebugInfo  bool
    GenerateComments   bool
    AssemblyDialect    string
    TargetConfig       TargetConfig
}
```

## 4. Support Types

### 4.1 Assembly and Output

```go
// OutputFormat specifies the output format
type OutputFormat int

const (
    OutputAssembly OutputFormat = iota // Text assembly
    OutputBinary                       // Raw binary
    OutputObject                       // Object file
    OutputExecutable                   // Executable file
    OutputHex                          // Intel HEX format
    OutputSRecord                      // Motorola S-record
)

// AssemblyFormat describes assembly syntax
type AssemblyFormat struct {
    Dialect           string    // "sjasmplus", "ca65", "vasm"
    CommentStyle      string    // ";", "//", "#"
    LabelStyle        string    // ":", "", "_:"
    ImmediatePrefix   string    // "#", "$", ""
    CaseSensitive     bool
    RequiresUpperCase bool
}
```

### 4.2 Memory and Hardware

```go
// MemoryLayout describes platform memory organization
type MemoryLayout struct {
    Regions []MemoryRegion
}

type MemoryRegion struct {
    Name        string    // "ROM", "RAM", "Video", "I/O"
    Start       uint32    // Start address
    Size        uint32    // Size in bytes
    Permissions Permission // Read/Write/Execute flags
    Description string    // Human description
}

type Permission uint8

const (
    PermRead    Permission = 1 << iota
    PermWrite
    PermExecute
)

// HardwareRegister represents a hardware register
type HardwareRegister struct {
    Name        string
    Address     uint32
    Size        int      // Size in bits
    Access      Permission
    Description string
    BitFields   []BitField
}

type BitField struct {
    Name        string
    StartBit    int
    EndBit      int
    Description string
}
```

### 4.3 Debug and Analysis

```go
// DebugInfo contains debugging information
type DebugInfo struct {
    SourceFiles    []SourceFile
    Functions      []FunctionDebugInfo
    Variables      []VariableDebugInfo
    LineNumbers    []LineNumberEntry
    Symbols        []DebugSymbol
}

// SourceMap maps generated code back to source
type SourceMap struct {
    Version    int
    Sources    []string
    Names      []string
    Mappings   string
}

// OptimizationStats tracks optimization performance
type OptimizationStats struct {
    PassesRun        int
    FunctionsChanged int
    BytesSaved       int
    CyclesSaved      int64
    TimeSpent        time.Duration
}

// CodeGenStats tracks code generation metrics
type CodeGenStats struct {
    BytesGenerated   int
    InstructionsGenerated int
    FunctionsGenerated   int
    SymbolsGenerated     int
    TimeSpent           time.Duration
}
```

## 5. Error Handling

### 5.1 Error Types

```go
// TargetError represents target-specific errors
type TargetError struct {
    Target    string
    Operation string
    Message   string
    Cause     error
}

func (e *TargetError) Error() string {
    return fmt.Sprintf("target %s: %s: %s", e.Target, e.Operation, e.Message)
}

// UnsupportedFeatureError indicates a feature is not supported
type UnsupportedFeatureError struct {
    Target  string
    Feature FeatureFlag
    Context string
    Suggestion string
}

func (e *UnsupportedFeatureError) Error() string {
    return fmt.Sprintf("feature %v not supported on target %s: %s", 
                       e.Feature, e.Target, e.Context)
}

// ConfigurationError indicates invalid target configuration
type ConfigurationError struct {
    Target string
    Option string
    Value  any
    Reason string
}

func (e *ConfigurationError) Error() string {
    return fmt.Sprintf("invalid configuration for target %s: %s = %v: %s",
                       e.Target, e.Option, e.Value, e.Reason)
}
```

## 6. Interface Contracts

### 6.1 Target Implementation Requirements

All `Target` implementations must:

1. **Thread Safety**: Be safe for concurrent use by multiple goroutines
2. **Immutability**: Not modify input parameters (except through documented mutation methods)
3. **Error Handling**: Return appropriate error types with clear messages
4. **Resource Management**: Clean up resources in case of errors
5. **Validation**: Validate all inputs and configurations
6. **Documentation**: Provide comprehensive documentation for all public methods

### 6.2 Optimizer Behavior

Optimizer implementations must:

1. **Deterministic**: Produce identical output for identical input
2. **Reversible**: Support disabling optimizations
3. **Safe**: Never produce incorrect code
4. **Metrics**: Track and report optimization impact
5. **Incremental**: Support partial optimization

### 6.3 CodeGenerator Contracts

CodeGenerator implementations must:

1. **Complete**: Generate all required code sections
2. **Valid**: Produce syntactically correct assembly
3. **Debuggable**: Support debug information generation
4. **Linkable**: Generate proper symbol tables and relocations
5. **Platform-Aware**: Respect platform-specific conventions

This specification provides the complete contract for implementing new MinZ targets while ensuring consistency, reliability, and maintainability across all supported architectures.