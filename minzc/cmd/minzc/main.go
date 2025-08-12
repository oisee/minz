package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/ctie"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/mir"
	"github.com/minz/minzc/pkg/module"
	"github.com/minz/minzc/pkg/optimizer"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/semantic"
	"github.com/minz/minzc/pkg/version"
	"github.com/spf13/cobra"
)

var (
	outputFile   string
	optimize     bool
	debug        bool
	enableSMC    bool
	enableTAS    bool
	enableCTIE   bool   // Enable Compile-Time Interface Execution
	ctieDebug    bool   // Debug CTIE decisions
	tasFile      string
	tasReplay    string
	backend      string
	target       string  // Target platform (zxspectrum, cpm, etc.)
	listBackends bool
	visualizeMIR string // Output file for MIR visualization
	showVersion  bool
	showVersionFull bool
	dumpAST      bool   // Dump AST in JSON format
)

var rootCmd = &cobra.Command{
	Use:   "mz [source file]",
	Short: "MinZ Multi-Platform Compiler " + version.GetVersion(),
	Long:  `MinZ - Modern Programming Language for Retro Platforms
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Write once, run on any Z80, 6502, or modern platform!

BACKENDS:
  z80    - Z80 assembly (default)
  6502   - 6502 assembly  
  68000  - Motorola 68000 assembly
  i8080  - Intel 8080 assembly
  gb     - Game Boy (SM83/LR35902)
  wasm   - WebAssembly
  c      - C99 source code
  llvm   - LLVM IR

TARGET PLATFORMS (for Z80):
  zxspectrum - ZX Spectrum (default)
  cpm        - CP/M systems
  msx        - MSX computers
  cpc        - Amstrad CPC
  amstrad    - Amstrad PCW

LANGUAGE FEATURES:
  ✅ Zero-cost abstractions      ✅ Function overloading
  ✅ Lambda expressions          ✅ Pattern matching
  ✅ Error propagation (?)       ✅ Interfaces & traits
  ✅ Metaprogramming (@minz)     ✅ Self-modifying code
  ✅ Inline assembly             ✅ Iterator chains

EXAMPLES:
  mz hello.minz                      # Compile for ZX Spectrum
  mz hello.minz -t cpm               # Target CP/M systems
  mz hello.minz -t msx -O            # Optimized MSX build
  mz game.minz -b gb                 # Compile for Game Boy
  mz app.minz -b c -o app.c          # Generate C code
  mz demo.minz --enable-smc          # Enable self-modifying code
  mz --list-backends                 # List all backends

OPTIMIZATION FLAGS:
  -O, --optimize      Enable standard optimizations
  --enable-smc        Enable self-modifying code (Z80 only)

DEBUGGING:
  -d, --debug         Show compilation details
  --dump-ast          Output AST in JSON format
  --viz file.dot      Generate MIR visualization

CHARACTER LITERALS IN ASSEMBLY:
  asm { LD A, 'H' }   # Single quotes
  asm { LD A, "H" }   # Double quotes  
  asm { LD A, '\n' }  # Escape sequences

For documentation and examples, see:
  https://github.com/minz-lang/minzc
  
Platform Independence Guide:
  docs/150_Platform_Independence_Achievement.md`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Handle version flags
		if showVersion {
			fmt.Println(version.GetVersion())
			return
		}
		
		if showVersionFull {
			fmt.Println(version.GetFullVersion())
			return
		}
		
		// Handle --list-backends flag
		if listBackends {
			backends := codegen.ListBackends()
			fmt.Println("Available backends:")
			for _, b := range backends {
				fmt.Printf("  - %s\n", b)
			}
			return
		}
		
		// Require source file if not listing backends
		if len(args) == 0 {
			// Show help when called without arguments (like Go compiler)
			cmd.Help()
			os.Exit(0)
		}
		
		sourceFile := args[0]
		if err := compile(sourceFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	// Check environment variable for default backend
	defaultBackend := os.Getenv("MINZ_BACKEND")
	if defaultBackend == "" {
		defaultBackend = "z80"
	}
	
	// Version flags
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "show version")
	rootCmd.Flags().BoolVar(&showVersionFull, "version-full", false, "show full version info")
	
	// Compilation flags
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file (default: input.<ext> based on backend)")
	rootCmd.Flags().BoolVarP(&optimize, "optimize", "O", false, "enable optimizations")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug output")
	rootCmd.Flags().BoolVar(&enableSMC, "enable-smc", false, "enable all self-modifying code optimizations including TRUE SMC (requires code in RAM)")
	rootCmd.Flags().BoolVar(&enableTAS, "tas", false, "enable TAS debugging with time-travel and cycle-perfect recording")
	rootCmd.Flags().StringVar(&tasFile, "tas-record", "", "record execution to TAS file for perfect replay")
	rootCmd.Flags().StringVar(&tasReplay, "tas-replay", "", "replay execution from TAS file")
	rootCmd.Flags().StringVarP(&backend, "backend", "b", defaultBackend, "target backend (z80, 6502, wasm, c, llvm)")
	rootCmd.Flags().StringVarP(&target, "target", "t", "zxspectrum", "target platform (zxspectrum, cpm, msx, cpc, amstrad)")
	rootCmd.Flags().BoolVar(&listBackends, "list-backends", false, "list available backends")
	rootCmd.Flags().StringVar(&visualizeMIR, "viz", "", "generate MIR visualization in DOT format")
	rootCmd.Flags().BoolVar(&dumpAST, "dump-ast", false, "dump AST in JSON format to stdout")
	rootCmd.Flags().BoolVar(&enableCTIE, "enable-ctie", false, "enable Compile-Time Interface Execution (functions execute at compile-time)")
	rootCmd.Flags().BoolVar(&ctieDebug, "ctie-debug", false, "show CTIE optimization decisions and statistics")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func compile(sourceFile string) error {
	// Silent by default (like Go compiler)
	if debug {
		fmt.Printf("Compiling %s...\n", sourceFile)
	}
	
	// Check if input is a MIR file
	if filepath.Ext(sourceFile) == ".mir" {
		return compileFromMIR(sourceFile)
	}

	// Find project root (directory containing the source file or its parent)
	projectRoot := filepath.Dir(sourceFile)
	
	// Create module manager
	_ = module.NewModuleManager(projectRoot)

	// Parse the source file
	parser := parser.New()
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("DEBUG: Parsing file %s\n", sourceFile)
	}
	astFile, err := parser.ParseFile(sourceFile)
	if os.Getenv("DEBUG") != "" && astFile != nil {
		fmt.Printf("DEBUG: Parsed %d declarations\n", len(astFile.Declarations))
		for i, decl := range astFile.Declarations {
			fmt.Printf("  Decl %d: %T\n", i, decl)
			if varDecl, ok := decl.(*ast.VarDecl); ok {
				fmt.Printf("    Variable: %s, Value: %T\n", varDecl.Name, varDecl.Value)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Dump AST if requested
	if dumpAST {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(astFile); err != nil {
			return fmt.Errorf("failed to encode AST: %w", err)
		}
		return nil // Exit after dumping AST
	}

	// Set up module name if not explicitly declared
	if astFile.ModuleName == "" {
		astFile.ModuleName = module.ExtractModuleName(sourceFile)
	}

	// Perform semantic analysis with module support
	analyzer := semantic.NewAnalyzer()
	analyzer.SetTargetBackend(backend)
	analyzer.SetTargetPlatform(target)
	// TODO: Set module resolver on analyzer
	irModule, err := analyzer.Analyze(astFile)
	if err != nil {
		return fmt.Errorf("semantic error: %w", err)
	}
	
	// Debug: Print string count
	if os.Getenv("DEBUG") != "" && irModule != nil {
		fmt.Printf("DEBUG: Module has %d strings after analysis\n", len(irModule.Strings))
		for i, s := range irModule.Strings {
			fmt.Printf("  String %d: label=%s, value=\"%s\"\n", i, s.Label, s.Value)
		}
	}
	defer analyzer.Close()
	
	// Get the backend to check if it supports SMC
	backendInstance := codegen.GetBackend(backend, nil)
	supportsSMC := backendInstance != nil && backendInstance.SupportsFeature(codegen.FeatureSelfModifyingCode)
	
	// Enable SMC for all functions if flag is set OR if optimizing (and backend supports it)
	if supportsSMC && (enableSMC || optimize) {
		for _, fn := range irModule.Functions {
			fn.IsSMCEnabled = true
		}
		if debug {
			if enableSMC {
				fmt.Println("Self-modifying code optimization enabled (including TRUE SMC)")
			} else {
				fmt.Println("SMC enabled automatically with -O optimization")
			}
		}
	} else if !supportsSMC && enableSMC {
		if debug {
			fmt.Printf("Warning: Backend %s does not support self-modifying code\n", backend)
		}
	}

	// Run CTIE pass if requested (before regular optimizations)
	if enableCTIE || optimize {
		ctieEngine := ctie.NewEngine(irModule, astFile, analyzer)
		ctieConfig := ctie.DefaultConfig()
		ctieConfig.DebugOutput = ctieDebug || debug
		ctieEngine.SetConfig(ctieConfig)
		
		if err := ctieEngine.Process(); err != nil {
			return fmt.Errorf("CTIE error: %w", err)
		}
		
		if ctieDebug || debug {
			stats := ctieEngine.GetStatistics()
			if stats.FunctionsExecuted > 0 {
				fmt.Printf("CTIE: Executed %d functions at compile-time, eliminated %d bytes\n", 
					stats.FunctionsExecuted, stats.BytesEliminated)
			}
		}
	}

	// Run optimization passes if requested
	if optimize || enableSMC {
		level := optimizer.OptLevelBasic
		if optimize {
			level = optimizer.OptLevelFull
		}
		
		// Use TRUE SMC when optimizing OR when SMC explicitly enabled
		useTrueSMC := optimize || enableSMC
		
		opt := optimizer.NewOptimizerWithOptions(level, useTrueSMC)
		if err := opt.Optimize(irModule); err != nil {
			return fmt.Errorf("optimization error: %w", err)
		}
		
		if debug {
			fmt.Println("Optimization completed")
		}
	}

	// Create backend options
	backendOptions := &codegen.BackendOptions{
		OptimizationLevel: 0,
		EnableSMC:         enableSMC,
		EnableTrueSMC:     enableSMC || optimize,
		Debug:             debug,
		Target:            target,
	}
	
	if optimize {
		backendOptions.OptimizationLevel = 2
	}

	// Get the backend
	backendInst := codegen.GetBackend(backend, backendOptions)
	if backendInst == nil {
		return fmt.Errorf("unknown backend: %s", backend)
	}
	
	if debug {
		fmt.Printf("Using backend: %s\n", backend)
		// Check if backend came from environment variable
		envBackend := os.Getenv("MINZ_BACKEND")
		if envBackend != "" && backend == envBackend {
			fmt.Printf("  (from environment variable MINZ_BACKEND)\n")
		} else if backend == "z80" && envBackend == "" {
			fmt.Printf("  (default)\n")
		}
	}

	// Determine output filename based on backend
	if outputFile == "" {
		base := filepath.Base(sourceFile)
		ext := filepath.Ext(base)
		outputFile = base[:len(base)-len(ext)] + backendInst.GetFileExtension()
	}

	// Save IR to .mir file
	mirFile := outputFile[:len(outputFile)-len(filepath.Ext(outputFile))] + ".mir"
	if err := saveIRModule(irModule, mirFile); err != nil {
		if debug {
			fmt.Printf("Warning: failed to save MIR file: %v\n", err)
		}
	} else if debug {
		fmt.Printf("Saved IR to %s\n", mirFile)
	}

	// Generate MIR visualization if requested
	if visualizeMIR != "" {
		if err := generateVisualization(irModule, visualizeMIR); err != nil {
			return fmt.Errorf("visualization error: %w", err)
		}
		fmt.Printf("Generated MIR visualization: %s\n", visualizeMIR)
	}

	// Generate code using the backend
	generatedCode, err := backendInst.Generate(irModule)
	if err != nil {
		return fmt.Errorf("code generation error: %w", err)
	}
	
	// Write output file
	if err := os.WriteFile(outputFile, []byte(generatedCode), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}
	
	// Add TAS debugging support if enabled
	if enableTAS {
		if err := addTASSupport(outputFile); err != nil {
			return fmt.Errorf("TAS integration error: %w", err)
		}
		fmt.Println("TAS debugging enabled - use 'mzr --tas' to debug with time-travel")
	}
	
	// Handle TAS recording/replay
	if tasFile != "" {
		fmt.Printf("TAS recording enabled - output will be saved to %s\n", tasFile)
	}
	if tasReplay != "" {
		fmt.Printf("TAS replay mode - will replay from %s\n", tasReplay)
	}

	// Silent on success (like Go compiler)
	if debug {
		fmt.Printf("Successfully compiled to %s\n", outputFile)
	}
	return nil
}

// compileFromMIR compiles a .mir file directly to the target backend
func compileFromMIR(mirFile string) error {
	fmt.Printf("Compiling from MIR: %s...\n", mirFile)
	
	// Import the MIR parser
	mirParser := mir.ParseMIRFile
	
	// Parse the MIR file
	irModule, err := mirParser(mirFile)
	if err != nil {
		return fmt.Errorf("MIR parse error: %w", err)
	}
	
	// Debug: Print module info
	if debug {
		fmt.Printf("Loaded MIR module: %s\n", irModule.Name)
		fmt.Printf("Functions: %d\n", len(irModule.Functions))
		for _, fn := range irModule.Functions {
			fmt.Printf("  - %s (%d instructions)\n", fn.Name, len(fn.Instructions))
		}
	}
	
	// Get the backend to check if it supports SMC
	backendInstance := codegen.GetBackend(backend, nil)
	supportsSMC := backendInstance != nil && backendInstance.SupportsFeature(codegen.FeatureSelfModifyingCode)
	
	// Enable SMC for all functions if flag is set OR if optimizing (and backend supports it)
	if supportsSMC && (enableSMC || optimize) {
		for _, fn := range irModule.Functions {
			// Preserve existing SMC settings from MIR
			if !fn.IsSMCEnabled {
				fn.IsSMCEnabled = true
			}
		}
		if debug {
			if enableSMC {
				fmt.Println("Self-modifying code optimization enabled (including TRUE SMC)")
			} else {
				fmt.Println("SMC enabled automatically with -O optimization")
			}
		}
	} else if !supportsSMC && enableSMC {
		if debug {
			fmt.Printf("Warning: Backend %s does not support self-modifying code\n", backend)
		}
	}

	// Run optimization passes if requested
	if optimize || enableSMC {
		level := optimizer.OptLevelBasic
		if optimize {
			level = optimizer.OptLevelFull
		}
		
		// Use TRUE SMC when optimizing OR when SMC explicitly enabled
		useTrueSMC := optimize || enableSMC
		
		opt := optimizer.NewOptimizerWithOptions(level, useTrueSMC)
		if err := opt.Optimize(irModule); err != nil {
			return fmt.Errorf("optimization error: %w", err)
		}
		
		if debug {
			fmt.Println("Optimization completed")
		}
	}

	// Create backend options
	backendOptions := &codegen.BackendOptions{
		OptimizationLevel: 0,
		EnableSMC:         enableSMC,
		EnableTrueSMC:     enableSMC || optimize,
		Debug:             debug,
		Target:            target,
	}

	if optimize {
		backendOptions.OptimizationLevel = 2
	}

	// Get the backend
	backendInst := codegen.GetBackend(backend, backendOptions)
	if backendInst == nil {
		return fmt.Errorf("unknown backend: %s", backend)
	}

	if debug {
		fmt.Printf("Using backend: %s\n", backend)
		// Check if backend came from environment variable
		envBackend := os.Getenv("MINZ_BACKEND")
		if envBackend != "" && backend == envBackend {
			fmt.Printf("  (from environment variable MINZ_BACKEND)\n")
		} else if backend == "z80" && envBackend == "" {
			fmt.Printf("  (default)\n")
		}
	}

	// Determine output filename based on backend
	if outputFile == "" {
		base := filepath.Base(mirFile)
		ext := filepath.Ext(base)
		outputFile = base[:len(base)-len(ext)] + backendInst.GetFileExtension()
	}

	// Generate code using the backend
	generatedCode, err := backendInst.Generate(irModule)
	if err != nil {
		return fmt.Errorf("code generation error: %w", err)
	}

	// Write output file
	if err := os.WriteFile(outputFile, []byte(generatedCode), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	// Add TAS debugging support if enabled
	if enableTAS {
		if err := addTASSupport(outputFile); err != nil {
			return fmt.Errorf("TAS integration error: %w", err)
		}
		fmt.Println("TAS debugging enabled - use 'mzr --tas' to debug with time-travel")
	}

	// Handle TAS recording/replay
	if tasFile != "" {
		fmt.Printf("TAS recording enabled - output will be saved to %s\n", tasFile)
	}
	if tasReplay != "" {
		fmt.Printf("TAS replay mode - will replay from %s\n", tasReplay)
	}

	// Silent on success (like Go compiler)
	if debug {
		fmt.Printf("Successfully compiled to %s\n", outputFile)
	}
	return nil
}

// addTASSupport adds TAS debugging hooks to generated assembly
func addTASSupport(asmFile string) error {
	// TODO: Add TAS debugging hooks to assembly
	// For now, just add a comment marker
	return nil
}

// generateVisualization generates a DOT file for MIR visualization
func generateVisualization(module *ir.Module, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	visualizer := mir.NewVisualizer(file)
	return visualizer.Visualize(module)
}

// saveIRModule saves the IR module to a .mir file
func saveIRModule(module *ir.Module, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "; MinZ Intermediate Representation (MIR)\n")
	fmt.Fprintf(file, "; Module: %s\n\n", module.Name)

	// Write globals if any
	if len(module.Globals) > 0 {
		fmt.Fprintf(file, "; Globals:\n")
		for _, g := range module.Globals {
			fmt.Fprintf(file, ";   %s: %s\n", g.Name, g.Type.String())
		}
		fmt.Fprintf(file, "\n")
	}

	// Write each function
	for _, fn := range module.Functions {
		fmt.Fprintf(file, "Function %s(", fn.Name)
		for i, param := range fn.Params {
			if i > 0 {
				fmt.Fprintf(file, ", ")
			}
			fmt.Fprintf(file, "%s: %s", param.Name, param.Type.String())
		}
		fmt.Fprintf(file, ") -> %s\n", fn.ReturnType.String())

		// Function attributes
		if fn.IsSMCEnabled {
			fmt.Fprintf(file, "  @smc\n")
		}
		if fn.IsRecursive {
			fmt.Fprintf(file, "  @recursive\n")
		}
		if fn.IsInterrupt {
			fmt.Fprintf(file, "  @interrupt\n")
		}

		// Locals
		if len(fn.Locals) > 0 {
			fmt.Fprintf(file, "  Locals:\n")
			for _, local := range fn.Locals {
				fmt.Fprintf(file, "    r%d = %s: %s\n", local.Reg, local.Name, local.Type.String())
			}
		}

		// Instructions
		fmt.Fprintf(file, "  Instructions:\n")
		for i, inst := range fn.Instructions {
			fmt.Fprintf(file, "    %3d: ", i)
			
			// Format instruction based on opcode
			switch inst.Op {
			case ir.OpLoadConst:
				fmt.Fprintf(file, "r%d = %d", inst.Dest, inst.Imm)
			case ir.OpMove:
				fmt.Fprintf(file, "r%d = r%d", inst.Dest, inst.Src1)
			case ir.OpLoadVar:
				fmt.Fprintf(file, "r%d = load %s", inst.Dest, inst.Symbol)
			case ir.OpLoadAddr:
				fmt.Fprintf(file, "r%d = addr(%s)", inst.Dest, inst.Symbol)
			case ir.OpStoreVar:
				fmt.Fprintf(file, "store %s, r%d", inst.Symbol, inst.Src1)
			case ir.OpAdd:
				fmt.Fprintf(file, "r%d = r%d + r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpSub:
				fmt.Fprintf(file, "r%d = r%d - r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpMul:
				fmt.Fprintf(file, "r%d = r%d * r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpAnd:
				fmt.Fprintf(file, "r%d = r%d & r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpOr:
				fmt.Fprintf(file, "r%d = r%d | r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpXor:
				fmt.Fprintf(file, "r%d = r%d ^ r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpNot:
				fmt.Fprintf(file, "r%d = ~r%d", inst.Dest, inst.Src1)
			case ir.OpEq:
				fmt.Fprintf(file, "r%d = r%d == r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpNe:
				fmt.Fprintf(file, "r%d = r%d != r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpLt:
				fmt.Fprintf(file, "r%d = r%d < r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpGt:
				fmt.Fprintf(file, "r%d = r%d > r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpLe:
				fmt.Fprintf(file, "r%d = r%d <= r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpGe:
				fmt.Fprintf(file, "r%d = r%d >= r%d", inst.Dest, inst.Src1, inst.Src2)
			case ir.OpCall:
				fmt.Fprintf(file, "r%d = call %s", inst.Dest, inst.Symbol)
			case ir.OpCallIndirect:
				if len(inst.Args) > 0 {
					fmt.Fprintf(file, "r%d = call_indirect r%d (args:", inst.Dest, inst.Src1)
					for i, arg := range inst.Args {
						if i > 0 {
							fmt.Fprintf(file, ",")
						}
						fmt.Fprintf(file, " r%d", arg)
					}
					fmt.Fprintf(file, ")")
				} else {
					fmt.Fprintf(file, "r%d = call_indirect r%d", inst.Dest, inst.Src1)
				}
			case ir.OpReturn:
				if inst.Src1 != 0 {
					fmt.Fprintf(file, "return r%d", inst.Src1)
				} else {
					fmt.Fprintf(file, "return")
				}
			case ir.OpJump:
				fmt.Fprintf(file, "jump %s", inst.Label)
			case ir.OpJumpIfNot:
				fmt.Fprintf(file, "jump_if_not r%d, %s", inst.Src1, inst.Label)
			case ir.OpLabel:
				fmt.Fprintf(file, "%s:", inst.Label)
			default:
				fmt.Fprintf(file, "%v", inst.Op)
			}

			// Add comment if present
			if inst.Comment != "" {
				fmt.Fprintf(file, " ; %s", inst.Comment)
			}
			fmt.Fprintf(file, "\n")
		}
		fmt.Fprintf(file, "\n")
	}

	return nil
}