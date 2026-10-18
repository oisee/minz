package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/abap"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/ctie"
	_ "github.com/minz/minzc/pkg/ez80" // register eZ80 backend
	"github.com/minz/minzc/pkg/frill"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	// "github.com/minz/minzc/pkg/mir" — MIR1 deprecated, archived to pkg/_archive/mir1_deprecated
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2c"
	"github.com/minz/minzc/pkg/mir2gpu" // GPU backends: CUDA, OpenCL, Vulkan, Metal
	"github.com/minz/minzc/pkg/mir2llvm"
	"github.com/minz/minzc/pkg/mir2wasm"
	"github.com/minz/minzc/pkg/module"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/optimizer"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/plm"
	"github.com/minz/minzc/pkg/semantic"
	"github.com/minz/minzc/pkg/trace"
	"github.com/minz/minzc/pkg/version"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/spf13/cobra"
)

var (
	cliAssertStats    *hir.AssertStats
	outputFile        string
	disableOptimize   bool // Disable ALL optimizations (enabled by default)
	disableIROpt      bool // Disable IR/MIR-level optimizations only
	disableReroll     bool // Disable loop reroll optimization only
	disableAsmOpt     bool // Disable assembly-level peephole only
	disableCodegenOpt bool // Disable codegen-level constant tracking
	debug             bool
	disableSMC        bool // Disable self-modifying code (enabled by default)
	enableTAS         bool
	disableCTIE       bool // Disable Compile-Time Interface Execution (enabled by default)
	ctieDebug         bool // Debug CTIE decisions
	tasFile           string
	tasReplay         string
	contractStrategy  string
	backend           string
	target            string // Target platform (zxspectrum, cpm, etc.)
	forceOutput       bool   // --force: overwrite implicit generated C output
	outputFormat      string // Output format (code, sna, tap) — independent of target
	listBackends      bool
	visualizeMIR      string // Output file for MIR visualization
	showVersion       bool
	showVersionFull   bool
	dumpAST           bool // Dump AST in JSON format
	dumpMIR           bool // Dump MIR to stdout

	compileTrace bool // Structured compilation trace output

	// Superoptimizer
	superoptRules string // Path to z80-optimizer rules.json[.gz]

	// PGO (Profile-Guided Optimization) - Quick Win flags
	pgoProfile string // Path to .tas profile file for PGO compilation
	pgoDebug   bool   // Debug PGO decisions

	// Backend selection
	generateCheckedLocalTablePath string // offline output table
	composeCheckedLocalTablePool  string // input fragment pool for bounded composition
	composeCheckedLocalTablePath  string // exact current output table
	composeCheckedLocalTableLimit int
	checkedLocalSolver            string // Z3 executable for offline generation
	checkedLocalTableSHA256       string // optional content pin
	checkedLocalTablePath         string // --checked-local-table: admitted leaf OptSize candidates
	optSize                       bool   // -Osize: optimize for code size (Grace reroll, DJNZ loops)
	fuseByteWords                 bool
	useGrace                      bool // --grace: run all Grace MIR2 passes before VIR lowering
	useHIRGrace                   bool // --hir-grace: run structural Grace rules before HIR lowering

	// Assert control
	listAsserts          bool
	assertLines          string
	assertControlLine    int
	assertControlElement int
	assertLinesSet       bool
	assertControlSet     bool
	assertReceipt        bool   // --assert-receipt enables judge execution reporting
	assertMode           string // --asserts mir|z80|all|none — select which assert backends run
	assertForce          string // --asserts-force mir|z80 — force ALL asserts to run on this backend

	// Debug info
	emitSLD         bool // Emit SLD file for DeZog source-level debugging
	annotateTStates bool // Annotate each instruction with its Z80 T-state cost

	// Transpiler flags
	emitFormat string // emit format: "nanz" (HIR pretty-printed as Nanz source), "mir2-raw", "mir2"
	gpuRun     string // --gpu-run cuda|opencl|vulkan|all — compile+run on GPU
)

var rootCmd = &cobra.Command{
	Use:   "mz [source file]",
	Short: "MinZ Multi-Platform Compiler " + version.GetVersion(),
	Long: `MinZ - Modern Programming Language for Retro Platforms
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Write once, run on Z80 retro platforms!

BACKEND:
  z80     - Z80 assembly (default, production)

TARGET PLATFORMS:
  zxspectrum - ZX Spectrum (default)
  cpm        - CP/M systems
  msx        - MSX computers
  cpc        - Amstrad CPC
  amstrad    - Amstrad PCW
  agon       - Agon Light 2

LANGUAGE FEATURES:
  Zero-cost abstractions, function overloading, lambda expressions,
  pattern matching, error propagation, interfaces & traits,
  metaprogramming (@minz), self-modifying code, inline assembly,
  iterator chains

EXAMPLES:
  mz hello.nanz                      # Compile for ZX Spectrum
  mz hello.nanz -t cpm               # Target CP/M systems
  mz hello.nanz -t msx               # MSX build
  mz demo.nanz --disable-smc         # Disable self-modifying code
  mz --list-backends                 # List all backends

OPTIMIZATION FLAGS:
  --disable-optimize  Disable optimizations (enabled by default)
  --disable-smc       Disable self-modifying code (enabled by default, Z80 only)

DEBUGGING:
  -d, --debug         Show compilation details
  --compile-trace     Show all optimization decisions and transformations
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
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		assertLinesSet = cmd.Flags().Changed("assert-lines")
		assertControlSet = cmd.Flags().Changed("assert-control-line")
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
		err := compile(sourceFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		if assertReceipt && cliAssertStats != nil {
			fmt.Fprintf(os.Stderr, "ASSERTS: executed=%d passed=%d failed=%d\n", cliAssertStats.Executed, cliAssertStats.Passed, cliAssertStats.Failed)
		}
		if err != nil {
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
	rootCmd.Flags().BoolVar(&disableOptimize, "disable-optimize", false, "disable optimizations (enabled by default)")
	rootCmd.Flags().BoolVar(&disableIROpt, "disable-ir-opt", false, "disable IR/MIR-level optimizations (DCE, constant folding, inlining)")
	rootCmd.Flags().BoolVar(&disableReroll, "disable-reroll", false, "disable loop reroll optimization (putchar sequence merging)")
	rootCmd.Flags().BoolVar(&disableAsmOpt, "disable-asm-opt", false, "disable assembly-level peephole optimizations")
	rootCmd.Flags().BoolVar(&disableCodegenOpt, "disable-codegen-opt", false, "disable codegen-level optimizations (constant tracking)")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug output")
	rootCmd.Flags().BoolVar(&disableSMC, "disable-smc", false, "disable all self-modifying code optimizations (enabled by default)")
	rootCmd.Flags().BoolVar(&enableTAS, "tas", false, "enable TAS debugging with time-travel and cycle-perfect recording")
	rootCmd.Flags().StringVar(&tasFile, "tas-record", "", "record execution to TAS file for perfect replay")
	rootCmd.Flags().StringVar(&tasReplay, "tas-replay", "", "replay execution from TAS file")

	// PGO flags (Quick Win integration)
	rootCmd.Flags().StringVar(&pgoProfile, "pgo", "", "use profile-guided optimization with .tas profile file")
	rootCmd.Flags().BoolVar(&pgoDebug, "pgo-debug", false, "show PGO optimization decisions and hot/cold analysis")
	registerHIRProfileFlags(rootCmd)
	registerMemoryHostCLIFlags(rootCmd)
	registerMemoryHostPolicyCLIFlags(rootCmd)
	registerMemoryHostSearchCLIFlags(rootCmd)
	rootCmd.Flags().StringVar(&hirProfileLayoutFunction, "hir-profile-layout-modes", "", "measure eight compiler modes and conditional layouts for this function with --hir-profile-auto")
	rootCmd.Flags().StringVar(&hirProfileLayoutReplay, "hir-profile-layout-modes-replay", "", "verify saved layout-mode search and run its winner on a new workload")
	rootCmd.Flags().IntVar(&hirProfileLayoutLimit, "hir-profile-layout-limit", 4, "maximum conditional layouts per compiler mode (1..4)")
	rootCmd.Flags().StringVar(&hirProfileCompilerModesReplay, "hir-profile-compiler-modes-replay", "", "run saved winning compiler-mode artifact on a new workload; requires same compiler executable")
	rootCmd.Flags().BoolVar(&hirProfileCompilerModes, "hir-profile-compiler-modes", false, "measure ContractOpt/MIR Grace modes on controlled workload; requires --hir-profile-auto")
	rootCmd.Flags().BoolVar(&hirProfileJoint, "hir-profile-joint", false, "measure candidate combinations on full VM/Z80 workload with --hir-profile-auto")
	rootCmd.Flags().BoolVar(&hirProfileIndependentRules, "hir-profile-independent-rules", false, "measure constant-if and invariant-base independently with --hir-profile-joint")
	rootCmd.Flags().BoolVar(&hirProfileInvariantSites, "hir-profile-invariant-sites", false, "measure scalar invariant-base choices by source site with --hir-profile-joint")
	rootCmd.Flags().BoolVar(&hirProfileSiteRules, "hir-profile-site-rules", false, "Cross constant-if choices with --hir-profile-invariant-sites (up to 32 measured variants)")
	rootCmd.Flags().BoolVar(&hirProfileJointSites, "hir-profile-joint-sites", false, "Jointly measure pointer-walk, scalar source-site hoists and constant-if (up to 32 variants)")
	rootCmd.Flags().BoolVar(&hirProfileMixedCopyRegionSites, "hir-profile-mixed-copy-region-sites", false, "Measure exact copy and byte-region choices with --hir-profile-joint (up to 16 variants)")
	rootCmd.Flags().BoolVar(&hirProfileOrderedByteFunctions, "hir-profile-ordered-byte-functions", false, "Measure exact ordered-byte helper functions with --hir-profile-joint (up to 16 variants)")
	rootCmd.Flags().BoolVar(&hirProfileOrderedByteRWFunctions, "hir-profile-ordered-byte-rw-functions", false, "Measure exact ordered-byte read/write helpers with --hir-profile-joint (up to 16 variants)")
	rootCmd.Flags().BoolVar(&hirProfileRegionCompareSites, "hir-profile-region-compare-sites", false, "Measure exact byte-region comparison sites with --hir-profile-joint (up to 16 variants)")
	rootCmd.Flags().BoolVar(&hirProfileForwardCopySites, "hir-profile-forward-copy-sites", false, "Measure exact forward byte-copy sites with --hir-profile-joint (up to 16 variants)")
	rootCmd.Flags().BoolVar(&hirProfileMixedCopyWeightedSites, "hir-profile-mixed-copy-weighted-sites", false, "Measure exact copy and weighted checksum choices with --hir-profile-joint (up to 16 variants)")
	rootCmd.Flags().BoolVar(&hirProfileWeightedSites, "hir-profile-weighted-sites", false, "Permit verified weighted checksum composition with --hir-profile-joint-sites")
	rootCmd.Flags().StringVar(&hirProfileAutoDirectory, "hir-profile-auto", "", "collect, measure full u8 sum domain and compile; save evidence in directory")
	rootCmd.Flags().StringVar(&hirProfileAutoFunction, "hir-profile-function", "", "exact sum function to measure with --hir-profile-auto")
	rootCmd.Flags().StringVar(&hirProfileAutoWorkload, "hir-profile-auto-workload", "", "controlled workload JSON for --hir-profile-auto")
	rootCmd.Flags().StringVar(&contractStrategy, "contract-strategy", "", "interprocedural contracts: greedy (default) or experimental pbqp")
	rootCmd.Flags().StringVarP(&backend, "backend", "b", defaultBackend, "target backend (z80, ez80)")
	rootCmd.Flags().IntVar(&runtimeStackBytes, "runtime-stack", 0, "resident stack bytes: cpm even 64..16384; agon-z80 even 128..16384 (default 1024)")
	rootCmd.Flags().StringVar(&runtimeMode, "runtime", "", "explicit application runtime: cpm (guarded byte-memory ABI), agon-z80 (native MOS Z80-mode ABI)")
	rootCmd.Flags().StringVarP(&target, "target", "t", "zxspectrum", "target platform (zxspectrum, cpm, msx, cpc, amstrad)")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "", "output format: code (raw binary, default), sna, tap")
	rootCmd.Flags().BoolVar(&forceOutput, "force", false, "overwrite existing implicit .generated.c output")
	rootCmd.Flags().BoolVar(&listBackends, "list-backends", false, "list available backends")
	rootCmd.Flags().StringVar(&visualizeMIR, "viz", "", "generate MIR visualization in DOT format")
	rootCmd.Flags().BoolVar(&dumpAST, "dump-ast", false, "dump AST in JSON format to stdout")
	rootCmd.Flags().BoolVar(&dumpMIR, "dump-mir", false, "dump MIR (intermediate representation) to stdout")
	rootCmd.Flags().BoolVar(&disableCTIE, "disable-ctie", false, "disable Compile-Time Interface Execution (enabled by default - functions execute at compile-time)")
	rootCmd.Flags().BoolVar(&ctieDebug, "ctie-debug", false, "show CTIE optimization decisions and statistics")
	rootCmd.Flags().BoolVar(&compileTrace, "compile-trace", false, "show all optimization decisions and transformations")
	rootCmd.Flags().StringVar(&superoptRules, "superopt-rules", "", "path to z80-optimizer rules.json[.gz] for superoptimizer peephole pass")
	rootCmd.Flags().StringVar(&composeCheckedLocalTablePool, "checked-local-fragment-pool", "", "input checked assignment pool for automatic bounded composition")
	rootCmd.Flags().StringVar(&composeCheckedLocalTablePath, "compose-checked-local-table", "", "prepare exact current table from fragments without external solver (requires --Osize)")
	rootCmd.Flags().IntVar(&composeCheckedLocalTableLimit, "checked-local-composition-limit", 64, "bounded pool, partition and per-partition assignment count (1..1024)")
	rootCmd.Flags().StringVar(&generateCheckedLocalTablePath, "generate-checked-local-table", "", "generate admitted leaf register table offline (requires --Osize)")
	rootCmd.Flags().StringVar(&checkedLocalSolver, "checked-local-solver", "z3", "Z3 executable for offline local table generation")
	rootCmd.Flags().StringVar(&checkedLocalTableSHA256, "checked-local-table-sha256", "", "pin exact checked table JSON content (required for measured HIR profile replay)")
	rootCmd.Flags().StringVar(&checkedLocalTablePath, "checked-local-table", "", "use checked local register table for smaller leaf Z80 functions (requires --Osize)")
	rootCmd.Flags().BoolVar(&optSize, "Osize", false, "optimize for code size: Grace reroll; skip HIR Grace's versioned pointer loops")
	rootCmd.Flags().BoolVar(&useGrace, "grace", false, "run all Grace MIR2 passes before VIR lowering (DSE, CondRetSink, BlockMerge, etc.)")
	rootCmd.Flags().BoolVar(&fuseByteWords, "fuse-byte-words", false, "fuse consecutive little-endian byte accesses with sequential fault semantics (experimental)")
	rootCmd.Flags().BoolVar(&useHIRGrace, "hir-grace", false, "run structural Grace HIR rules before lowering (experimental)")
	rootCmd.Flags().BoolVar(&assertReceipt, "assert-receipt", false, "print assertion execution counts to stderr for judge tools")
	rootCmd.Flags().BoolVar(&listAsserts, "list-asserts", false, "list frontend assertions as JSON lines without executing")
	rootCmd.Flags().StringVar(&assertLines, "assert-lines", "", "run only assertions at these comma-separated source lines")
	rootCmd.Flags().IntVar(&assertControlElement, "assert-control-element", 0, "tuple element to perturb (zero-based judge negative control)")
	rootCmd.Flags().IntVar(&assertControlLine, "assert-control-line", 0, "mutate the expected value at this source line (judge negative control)")
	rootCmd.Flags().StringVar(&assertMode, "asserts", "", "assert backend: mir2, z80, wasm, llvm, all (default), none")
	rootCmd.Flags().StringVar(&assertForce, "asserts-force", "", "force ALL asserts to run on this backend (mir2, z80, wasm, or llvm), ignoring 'via' annotations")
	rootCmd.Flags().BoolVar(&emitSLD, "emit-sld", false, "emit SLD file for DeZog source-level debugging")
	rootCmd.Flags().BoolVar(&annotateTStates, "annotate-tstates", false, "annotate each Z80 instruction with its T-state cost")
	rootCmd.Flags().StringVar(&emitFormat, "emit", "", "emit format: nanz, hir, lanz, mir2-raw, mir2, llvm, wasm, cuda, opencl, vulkan, metal")
	rootCmd.Flags().StringVar(&gpuRun, "gpu-run", "", "compile and run on GPU: cuda, opencl, vulkan, all")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func compile(sourceFile string) error {
	if err := validateCheckedCompositionCLI(sourceFile); err != nil {
		return err
	}
	if err := validateContractStrategyCLI(sourceFile); err != nil {
		return err
	}
	if err := validateCLIRuntime(sourceFile); err != nil {
		return err
	}
	if memoryHostSearchCLIRequested() {
		return runMemoryHostSearchCLI(sourceFile)
	}
	if memoryHostPolicyCLIRequested() {
		return runMemoryHostPolicyCLI(sourceFile)
	}
	if memoryHostCLI != (memoryHostCLIFlags{}) {
		return runMemoryHostCLI(sourceFile)
	}
	if err := validateHIRProfileAuto(sourceFile); err != nil {
		return err
	}
	if err := validateHIRProfileFlags(sourceFile); err != nil {
		return err
	}
	// Silent by default (like Go compiler)
	if debug {
		fmt.Printf("Compiling %s...\n", sourceFile)
	}

	// Check if input is a MIR file
	if filepath.Ext(sourceFile) == ".mir" {
		return compileFromMIR(sourceFile)
	}

	ext := filepath.Ext(sourceFile)

	// PL/M-80, Nanz, and HIR text: routed through the new HIR→MIR2→Z80 pipeline.
	if ext == ".plm" || ext == ".nanz" || ext == ".minz" || ext == ".hir" || ext == ".lanz" || ext == ".lizp" || ext == ".pas" || ext == ".c" || ext == ".m" || ext == ".abap" || ext == ".frl" {
		if emitFormat == "nanz" && ext != ".hir" && ext != ".lanz" {
			return compilePLMToNanz(sourceFile)
		}
		return compileViaHIR(sourceFile)
	}
	if emitFormat == "nanz" {
		return compilePLMToNanz(sourceFile)
	}

	// Check if input is an assembly file — route to z80asm assembler
	if ext == ".a80" || ext == ".asm" || ext == ".z80" {
		return assembleFile(sourceFile)
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

	// Create compile tracer (nil if --compile-trace not set)
	var tracer *trace.Tracer
	if compileTrace {
		tracer = trace.New(os.Stderr)
	}

	// Perform semantic analysis with module support
	analyzer := semantic.NewAnalyzer()
	analyzer.SetTargetBackend(backend)
	analyzer.SetTargetPlatform(target)
	analyzer.SetTracer(tracer)
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

	// Enable SMC only for functions that explicitly need it (UsesTrueSMC or IsSMCDefault).
	// ADR-0010: register-first is the default — do NOT force SMC on all functions.
	if supportsSMC && !disableSMC {
		for _, fn := range irModule.Functions {
			if fn.UsesTrueSMC || fn.IsSMCDefault {
				fn.IsSMCEnabled = true
			}
		}
		if debug {
			if !disableSMC {
				fmt.Println("Self-modifying code optimization enabled (including TRUE SMC) - default behavior")
			} else {
				fmt.Println("SMC disabled via --disable-smc flag")
			}
		}
	} else if !supportsSMC && !disableSMC {
		if debug {
			fmt.Printf("Warning: Backend %s does not support self-modifying code (using --disable-smc to silence)\n", backend)
		}
	}

	// Run CTIE pass (enabled by default, disabled with --disable-ctie)
	if !disableCTIE {
		ctieEngine := ctie.NewEngine(irModule, astFile, analyzer)
		ctieConfig := ctie.DefaultConfig()
		ctieConfig.DebugOutput = ctieDebug || debug
		ctieEngine.SetConfig(ctieConfig)
		ctieEngine.SetTracer(tracer)

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

	// Run optimization passes (enabled by default)
	if !disableOptimize && !disableIROpt {
		level := optimizer.OptLevelFull // Full optimization by default

		opt := optimizer.NewOptimizerWithOpts(level, optimizer.OptimizerOptions{
			EnableTrueSMC: !disableSMC,
			DisableReroll: disableReroll,
		})
		opt.SetTracer(tracer)
		if err := opt.Optimize(irModule); err != nil {
			return fmt.Errorf("optimization error: %w", err)
		}

		if debug {
			fmt.Println("Optimization completed")
		}

		// Apply PGO optimizations if profile provided (Quick Win #3)
		if pgoProfile != "" {
			// Load profile from TAS file (simplified mock data for now)
			profile := make(map[string]interface{})
			profile["executions"] = map[uint16]uint64{
				0x8000: 1000, // hot_function entry - very hot
				0x8010: 1000, // main function - hot
				0x8020: 10,   // print routine - warm
			}
			profile["hot_threshold"] = uint64(100)

			pgoOpt := optimizer.NewBasicPGOPass(profile)

			for _, fn := range irModule.Functions {
				pgoOpt.ApplyPlatformOptimizations(fn, target)
			}

			if pgoDebug || debug {
				fmt.Printf("PGO: Applied profile-guided optimizations for target '%s'\n", target)
			}
		}
	} else if disableIROpt && debug {
		fmt.Println("IR/MIR-level optimizations disabled via --disable-ir-opt")
	}

	// Create backend options
	backendOptions := &codegen.BackendOptions{
		OptimizationLevel: 0,
		EnableSMC:         !disableSMC,
		EnableTrueSMC:     !disableSMC,
		Debug:             debug,
		Target:            target,
		DisableAsmOpt:     disableAsmOpt || disableOptimize,
		DisableCodegenOpt: disableCodegenOpt || disableOptimize,
		Tracer:            tracer,
		SuperoptRules:     superoptRules,
		EmitSLD:           emitSLD,
		SourceFile:        sourceFile,
	}

	if !disableOptimize {
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

	// Dump MIR if requested
	if dumpMIR {
		// Create a temp file and use saveIRModule, then read and print
		tmpFile, err := os.CreateTemp("", "mir_dump_*.mir")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpName := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpName)

		if err := saveIRModule(irModule, tmpName); err != nil {
			return fmt.Errorf("failed to format MIR: %w", err)
		}

		content, err := os.ReadFile(tmpName)
		if err != nil {
			return fmt.Errorf("failed to read MIR: %w", err)
		}

		fmt.Print(string(content))
		return nil // Exit after dumping MIR
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

	// Generate SLD file for DeZog debugging if requested
	if emitSLD && backend == "z80" {
		sldFile := outputFile[:len(outputFile)-len(filepath.Ext(outputFile))] + ".sld"
		// Assemble the .a80 to get address mappings
		asm := z80asm.NewAssembler()
		result, err := asm.AssembleString(generatedCode)
		if err == nil && result != nil {
			if sldErr := codegen.GenerateSLDFromAssembly(outputFile, result, sldFile); sldErr != nil {
				if debug {
					fmt.Printf("Warning: failed to generate SLD file: %v\n", sldErr)
				}
			} else if debug {
				fmt.Printf("Generated SLD file: %s\n", sldFile)
			}
		} else if debug {
			fmt.Printf("Warning: SLD assembly pass failed: %v\n", err)
		}
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

// compileFromMIR compiles a .mir file directly to the target backend.
// DEPRECATED: MIR1 is archived. Use HIR→MIR2 pipeline instead.
func compileFromMIR(mirFile string) error {
	return fmt.Errorf("MIR1 format deprecated — use .nanz/.c/.pas source files instead (MIR1 archived to pkg/_archive/mir1_deprecated)")
}

/*
// compileFromMIR_deprecated — MIR1 pipeline removed.
// MIR1 archived to pkg/_archive/mir1_deprecated.
// All compilation now goes through HIR → MIR (was MIR2) → LIR → Z80.
func compileFromMIR_deprecated(mirFile string) error {
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

	// Enable SMC only for functions that explicitly need it (UsesTrueSMC or IsSMCDefault).
	// ADR-0010: register-first is the default — do NOT force SMC on all functions.
	if supportsSMC && !disableSMC {
		for _, fn := range irModule.Functions {
			if fn.UsesTrueSMC || fn.IsSMCDefault {
				fn.IsSMCEnabled = true
			}
		}
		if debug {
			if !disableSMC {
				fmt.Println("Self-modifying code optimization enabled (including TRUE SMC) - default behavior")
			} else {
				fmt.Println("SMC disabled via --disable-smc flag")
			}
		}
	} else if !supportsSMC && !disableSMC {
		if debug {
			fmt.Printf("Warning: Backend %s does not support self-modifying code (using --disable-smc to silence)\n", backend)
		}
	}

	// Run optimization passes (enabled by default)
	if !disableOptimize && !disableIROpt {
		level := optimizer.OptLevelFull  // Full optimization by default

		opt := optimizer.NewOptimizerWithOpts(level, optimizer.OptimizerOptions{
			EnableTrueSMC: !disableSMC,
			DisableReroll: disableReroll,
		})
		if err := opt.Optimize(irModule); err != nil {
			return fmt.Errorf("optimization error: %w", err)
		}

		if debug {
			fmt.Println("Optimization completed")
		}
	} else if disableIROpt && debug {
		fmt.Println("IR/MIR-level optimizations disabled via --disable-ir-opt")
	}

	// Create backend options
	backendOptions := &codegen.BackendOptions{
		OptimizationLevel: 0,
		EnableSMC:         !disableSMC,
		EnableTrueSMC:     !disableSMC,
		Debug:             debug,
		Target:            target,
		DisableAsmOpt:     disableAsmOpt || disableOptimize,
		DisableCodegenOpt: disableCodegenOpt || disableOptimize,
		SuperoptRules:     superoptRules,
	}

	if !disableOptimize {
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
*/ // end compileFromMIR_deprecated

// assembleFile assembles a .a80/.asm/.z80 file using the built-in z80asm assembler.
// This enables a one-tool workflow: mz source.minz -> mz output.a80 -> binary
func compileViaHIR(sourceFile string) error {
	if err := validateCheckedCompositionCLI(sourceFile); err != nil {
		return err
	}
	if err := validateContractStrategyCLI(sourceFile); err != nil {
		return err
	}
	if err := validateCLIRuntime(sourceFile); err != nil {
		return err
	}
	if err := validateHIRProfileAuto(sourceFile); err != nil {
		return err
	}
	if err := validateHIRProfileFlags(sourceFile); err != nil {
		return err
	}
	stats := &hir.AssertStats{}
	if !listAsserts {
		cliAssertStats = stats
	}

	src, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", sourceFile, err)
	}

	ext := filepath.Ext(sourceFile)

	// Parse source → *hir.Module
	// Resolve stdlib path: look for stdlib/ relative to the binary, or
	// relative to the source file's project root.
	absSource, _ := filepath.Abs(sourceFile)
	baseDir := filepath.Dir(absSource)
	stdlibDir := ""
	if exePath, err2 := os.Executable(); err2 == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "..", "stdlib")
		if info, err3 := os.Stat(candidate); err3 == nil && info.IsDir() {
			stdlibDir = candidate
		}
	}
	if stdlibDir == "" {
		for dir := baseDir; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			candidate := filepath.Join(dir, "stdlib")
			if info, err3 := os.Stat(candidate); err3 == nil && info.IsDir() {
				stdlibDir = candidate
				break
			}
		}
	}

	var hirMod *hir.Module
	switch ext {
	case ".plm":
		hirMod, err = plm.Compile(string(src))
		if err != nil {
			return fmt.Errorf("PL/M compile: %w", err)
		}
	case ".nanz", ".minz":
		hirMod, err = nanz.ParseWithOpts(string(src), sourceFile, nanz.ParseOpts{
			BaseDir:   baseDir,
			StdlibDir: stdlibDir,
			OptSize:   optSize,
		})
		if err != nil {
			return fmt.Errorf("Nanz parse: %w", err)
		}
	case ".hir":
		hirMod, err = hir.ParseHIR(string(src), filepath.Base(sourceFile))
		if err != nil {
			return fmt.Errorf("HIR parse: %w", err)
		}
	case ".lanz":
		hirMod, err = lanz.Compile(string(src), filepath.Base(sourceFile))
		if err != nil {
			return fmt.Errorf("Lanz compile: %w", err)
		}
	case ".lizp":
		hirMod, err = lizp.Compile(string(src), filepath.Base(sourceFile))
		if err != nil {
			return fmt.Errorf("Lizp compile: %w", err)
		}
	case ".pas":
		hirMod, err = pascal.Compile(string(src), filepath.Base(sourceFile), pascal.CompileOpts{
			StdlibDir: stdlibDir,
		})
		if err != nil {
			return fmt.Errorf("Pascal compile: %w", err)
		}
	case ".c", ".m":
		if listAsserts && ext == ".c" {
			hirMod, err = c89.ListAsserts(string(src))
			if err != nil {
				return err
			}
			break
		}
		absPath, _ := filepath.Abs(sourceFile)
		copts := c89.CompileOpts{BaseDir: filepath.Dir(absPath), IncludePaths: []string{filepath.Dir(absPath)}}
		if ext == ".c" {
			copts.Resolver = nanz.CSourceResolver(absPath, nanz.ParseOpts{StdlibDir: stdlibDir, OptSize: optSize})
		}
		hirMod, err = c89.CompileWithOpts(string(src), absPath, copts)
		if err != nil {
			return fmt.Errorf("C89/ObjC compile: %w", err)
		}
	case ".abap":
		hirMod, err = abap.Compile(string(src), filepath.Base(sourceFile), hir.TargetFromString(target))
		if err == nil && runtimeMode == "agon-z80" {
			err = abap.BindAgonZ80Runtime(hirMod)
		}
		if err != nil {
			return fmt.Errorf("ABAP compile: %w", err)
		}
		// If ABAP uses SELECT (has sqlite_* externs), merge sql/sqlite.nanz
		// to provide real I/O port asm bodies for CP/M.
		if stdlibDir != "" {
			hasSQLiteExtern := false
			for _, f := range hirMod.Funcs {
				if f.IsExtern && f.Name == "sqlite_open" {
					hasSQLiteExtern = true
					break
				}
			}
			if hasSQLiteExtern {
				sqlPath := filepath.Join(stdlibDir, "sql", "sqlite.nanz")
				if sqlSrc, err2 := os.ReadFile(sqlPath); err2 == nil {
					sqlMod, err3 := nanz.ParseWithOpts(string(sqlSrc), sqlPath, nanz.ParseOpts{
						OptSize:   optSize,
						BaseDir:   filepath.Dir(sqlPath),
						StdlibDir: stdlibDir,
					})
					if err3 == nil {
						// Replace extern stubs with real implementations
						externNames := make(map[string]bool)
						for _, f := range hirMod.Funcs {
							if f.IsExtern {
								externNames[f.Name] = true
							}
						}
						// Remove extern stubs that have real implementations
						var kept []*hir.Func
						for _, f := range hirMod.Funcs {
							if f.IsExtern && externNames[f.Name] {
								// Check if sqlMod has a real implementation
								hasReal := false
								for _, sf := range sqlMod.Funcs {
									if sf.Name == f.Name || sf.Name == "sql__sqlite__"+f.Name {
										hasReal = true
										break
									}
								}
								if hasReal {
									continue // skip extern, real impl will be added
								}
							}
							kept = append(kept, f)
						}
						hirMod.Funcs = kept
						// Add all functions from sqlite module
						hirMod.Funcs = append(hirMod.Funcs, sqlMod.Funcs...)
						// Add globals from sqlite module
						hirMod.Globals = append(hirMod.Globals, sqlMod.Globals...)
						// Merge strings
						hirMod.Strings = append(hirMod.Strings, sqlMod.Strings...)
						hirMod.StrKinds = append(hirMod.StrKinds, sqlMod.StrKinds...)
					}
				}
			}
		}
	case ".frl":
		absPath, _ := filepath.Abs(sourceFile)
		hirMod, err = frill.CompileWithOpts(string(src), filepath.Base(sourceFile), frill.CompileOpts{
			BaseDir: filepath.Dir(absPath),
		})
		if err != nil {
			return fmt.Errorf("Frill compile: %w", err)
		}
	default:
		return fmt.Errorf("unsupported extension for HIR pipeline: %s", ext)
	}

	if listAsserts {
		enc := json.NewEncoder(os.Stdout)
		emit := func(a hir.Assert, kind string) error {
			// Expression comes from the frontend; annotations are separate metadata.
			expression := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(a.Source), "//"))
			if i := strings.Index(expression, "//"); i >= 0 {
				expression = strings.TrimSpace(expression[:i])
			}
			if i := strings.LastIndex(expression, " via "); i >= 0 {
				expression = expression[:i]
			}
			return enc.Encode(struct {
				File          string `json:"file"`
				Line          int    `json:"line"`
				Expression    string `json:"expression"`
				Via           string `json:"via"`
				Kind          string `json:"kind"`
				TupleElements int    `json:"tuple_elements,omitempty"`
			}{sourceFile, a.Line, expression, a.Via, kind, len(a.ExpectedMulti)})
		}
		for _, a := range hirMod.Asserts {
			if err := emit(a, "top-level"); err != nil {
				return err
			}
		}
		for _, sb := range hirMod.Sandboxes {
			for _, a := range sb.Asserts {
				if err := emit(a, sb.Name); err != nil {
					return err
				}
			}
		}
		return nil
	}
	hirMod.AssertStats = stats
	selected := map[int]bool{}
	if assertLines != "" {
		for _, value := range strings.Split(assertLines, ",") {
			line, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			selected[line] = true
		}
	}
	if assertControlSet {
		validate := func(as []hir.Assert) error {
			for _, a := range as {
				if a.Line != assertControlLine || (assertLinesSet && !selected[a.Line]) {
					continue
				}
				size := len(a.ExpectedMulti)
				if size == 0 {
					size = 1
				}
				if assertControlElement < 0 || assertControlElement >= size {
					return fmt.Errorf("assert control element %d out of range at line %d", assertControlElement, a.Line)
				}
			}
			return nil
		}
		if err := validate(hirMod.Asserts); err != nil {
			return err
		}
		for _, sb := range hirMod.Sandboxes {
			if err := validate(sb.Asserts); err != nil {
				return err
			}
		}
	}
	filter := func(as []hir.Assert) []hir.Assert {
		return filterJudgeAsserts(as, selected, assertLinesSet, assertControlLine, assertControlSet, assertControlElement)
	}
	hirMod.Asserts = filter(hirMod.Asserts)
	for i := range hirMod.Sandboxes {
		hirMod.Sandboxes[i].Asserts = filter(hirMod.Sandboxes[i].Asserts)
	}

	// Set target platform so @target() intrinsic resolves correctly.
	hirMod.Target = hir.TargetFromString(target)
	if hirProfile.collect != "" {
		return collectHIRProfile(hirMod)
	}

	// ── GPU backend path ───────────────────────────────────────────────────
	// -b cuda|opencl|vulkan|metal → HIR → MIR2 → mir2gpu → write output
	if gpuBackend, ok := gpuBackendFromFlag(backend); ok {
		return compileToGPU(hirMod, gpuBackend, sourceFile)
	}

	// Lower and optimise MIR2; emit Z80 only when the requested product needs it.
	// --asserts-force overrides all 'via' annotations
	am := assertMode
	if assertForce != "" {
		am = assertForce
		// Force all asserts to specified backend by rewriting Via
		for i := range hirMod.Asserts {
			hirMod.Asserts[i].Via = ""
		}
		for i := range hirMod.Sandboxes {
			for j := range hirMod.Sandboxes[i].Asserts {
				hirMod.Sandboxes[i].Asserts[j].Via = ""
			}
		}
	}

	// An explicit output requests assembly/binary unless an intermediate or C
	// product was selected. Default compilation still emits assembly.
	skipZ80 := false
	switch emitFormat {
	case "lanz", "hir", "mir2", "mir2-raw", "llvm", "wasm", "cuda", "opencl", "vulkan", "metal":
		skipZ80 = true
	}
	if backend == "c" || gpuRun != "" {
		skipZ80 = true
	}
	mir2Only := am == "mir2" ||
		((am == "" || am == "all") && (len(hirMod.Asserts) > 0 || len(hirMod.Sandboxes) > 0) && !pipeline.NeedsZ80Asserts(hirMod, am))

	opts := pipeline.Options{
		CPMApplicationEntry:     ordinaryABAPCPMEntry(sourceFile, hirMod, skipZ80, am),
		Runtime:                 mir2.Z80Runtime(runtimeMode),
		RuntimeStackBytes:       runtimeStackBytes,
		CheckedLocalTablePath:   checkedLocalTablePath,
		CheckedLocalTableSHA256: checkedLocalTableSHA256,
		ContractStrategy:        contractStrategy,
		ContractOpt:             true,
		AnnotateTStates:         annotateTStates,
		OptSize:                 optSize, // --Osize enables Grace reroll
		FuseByteWords:           fuseByteWords,
		UseGrace:                useGrace, // --grace enables full Grace MIR2 pass suite
		UseHIRGrace:             useHIRGrace,
		Backend:                 backend,
		AssertMode:              am,
		SkipZ80Emission:         skipZ80,
	}
	if hirProfile.profile != "" {
		selection, err := loadHIRProfileSelection()
		if err != nil {
			return err
		}
		opts.ProfileSelection = selection
	}
	if composeCheckedLocalTablePath != "" {
		return runCheckedCompositionCLI(hirMod, opts, sourceFile, stdlibDir)
	}
	if generateCheckedLocalTablePath != "" {
		table, notes, err := pipeline.GenerateCheckedLocalTables(hirMod, opts, vir.SolverOptions{Z3Path: checkedLocalSolver})
		if err != nil {
			return err
		}
		for _, note := range notes {
			fmt.Fprintln(os.Stderr, "[checked-local] "+note)
		}
		if table.Size() == 0 {
			return fmt.Errorf("no checked local entries admitted")
		}
		return table.Save(generateCheckedLocalTablePath)
	}
	var steps pipeline.Steps
	jointCompiled := false
	if hirProfileAutoDirectory != "" {
		factory := func() (*hir.Module, error) {
			m, err := nanz.ParseWithOpts(string(src), sourceFile, nanz.ParseOpts{BaseDir: baseDir, StdlibDir: stdlibDir, OptSize: optSize})
			if err != nil {
				return nil, err
			}
			m.Target = hir.TargetFromString(target)
			return m, nil
		}
		if hirProfileLayoutFunction != "" || hirProfileLayoutReplay != "" {
			return runHIRLayoutModes(sourceFile, stdlibDir, src, factory)
		}
		if hirProfileCompilerModesReplay != "" {
			return runHIRCompilerModesReplay(sourceFile, stdlibDir, src, factory)
		}
		if hirProfileCompilerModes {
			return runHIRCompilerModes(sourceFile, stdlibDir, src, factory)
		}
		if hirProfileJoint {
			result, jointErr := prepareHIRProfileJoint(hirMod, factory, opts)
			if jointErr != nil {
				return jointErr
			}
			steps = result.Steps
			steps.ProfileDecisions = result.Decisions
			jointCompiled = true
		} else {
			opts.ProfileSelection, err = prepareHIRProfileAuto(hirMod, factory, opts)
			if err != nil {
				return err
			}
		}
	}
	if !jointCompiled {
		steps, err = pipeline.CompileHIRSteps(hirMod, opts)
	}
	if err != nil && !skipZ80 && outputFile == "" && mir2Only && steps.MIR2Module != nil {
		if abiErr := mir2.ValidateZ80IndirectCalls(steps.MIR2Module); abiErr != nil && err.Error() == abiErr.Error() {
			fmt.Fprintf(os.Stderr, "warning: Z80 output skipped because of the unsupported indirect-call ABI: %v\n", abiErr)
			opts.SkipZ80Emission = true
			steps, err = pipeline.CompileHIRSteps(hirMod, opts)
		}
	}
	if err != nil {
		return fmt.Errorf("HIR compile: %w", err)
	}
	if hirProfile.decisions != "" {
		if err := writeHIRProfileJSON(hirProfile.decisions, steps.ProfileDecisions); err != nil {
			return err
		}
	}

	if hirProfileAutoDirectory != "" {
		for name, data := range map[string]string{"input.hir": steps.HIR, "raw.mir": steps.MIR2Raw, "optimized.mir": steps.MIR2Opt, "selected.a80": steps.Assembly} {
			if err := os.WriteFile(filepath.Join(hirProfileAutoDirectory, name), []byte(data), 0644); err != nil {
				return err
			}
		}
		if !hirProfileOrderedByteRWFunctions && !hirProfileOrderedByteFunctions && !hirProfileMixedCopyRegionSites && !hirProfileRegionCompareSites && !hirProfileForwardCopySites && !hirProfileMixedCopyWeightedSites {
			if err := writeHIRProfileJSON(filepath.Join(hirProfileAutoDirectory, "decisions.json"), steps.ProfileDecisions); err != nil {
				return err
			}
		}
	}

	// --emit=lanz  → HIR as Lanz S-expressions
	if emitFormat == "lanz" {
		text := lanz.Dump(hirMod)
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(text), 0644); err != nil {
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
		} else {
			fmt.Print(text)
		}
		return nil
	}

	// --emit=hir  → HIR structural dump (typed tree, before lowering to MIR2)
	if emitFormat == "hir" {
		text := steps.HIR
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(text), 0644); err != nil {
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
		} else {
			fmt.Print(text)
		}
		return nil
	}
	// --emit=mir2  → print MIR2 dump (post-optimisation) to stdout or -o file
	if emitFormat == "mir2" {
		text := steps.MIR2Opt
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(text), 0644); err != nil {
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
		} else {
			fmt.Print(text)
		}
		return nil
	}
	// --emit=mir2-raw  → raw MIR2 before optimisation passes
	if emitFormat == "mir2-raw" {
		text := steps.MIR2Raw
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(text), 0644); err != nil {
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
		} else {
			fmt.Print(text)
		}
		return nil
	}
	// --emit=llvm  → MIR2 as LLVM IR text (.ll format)
	if emitFormat == "llvm" {
		text, err := mir2llvm.Compile(steps.MIR2Module)
		if err != nil {
			return fmt.Errorf("llvm compile: %w", err)
		}
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(text), 0644); err != nil {
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
		} else {
			fmt.Print(text)
		}
		return nil
	}
	// --emit=cuda/opencl/vulkan/metal  → MIR2 as GPU compute shader
	gpuBackends := map[string]mir2gpu.Backend{
		"cuda": mir2gpu.CUDA, "opencl": mir2gpu.OpenCL,
		"vulkan": mir2gpu.Vulkan, "metal": mir2gpu.Metal,
	}
	if gpuBE, ok := gpuBackends[emitFormat]; ok {
		text, err := mir2gpu.Compile(steps.MIR2Module, mir2gpu.CompileOptions{Backend: gpuBE})
		if err != nil {
			return fmt.Errorf("gpu compile: %w", err)
		}
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(text), 0644); err != nil {
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
		} else {
			fmt.Print(text)
		}
		return nil
	}
	// --emit=wasm  → MIR2 as WebAssembly text (.wat format)
	if emitFormat == "wasm" {
		text, err := mir2wasm.Compile(steps.MIR2Module)
		if err != nil {
			return fmt.Errorf("wasm compile: %w", err)
		}
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(text), 0644); err != nil {
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
		} else {
			fmt.Print(text)
		}
		return nil
	}

	// --gpu-run: compile and execute on GPU
	if gpuRun != "" {
		gpuBackendMap := map[string][]mir2gpu.Backend{
			"cuda":   {mir2gpu.CUDA},
			"opencl": {mir2gpu.OpenCL},
			"vulkan": {mir2gpu.Vulkan},
			"all":    {mir2gpu.CUDA, mir2gpu.OpenCL, mir2gpu.Vulkan},
		}
		backends, ok := gpuBackendMap[gpuRun]
		if !ok {
			return fmt.Errorf("unknown --gpu-run backend: %s (use cuda, opencl, vulkan, all)", gpuRun)
		}
		for _, be := range backends {
			result, err := mir2gpu.Run(steps.MIR2Module, be, 256)
			if err != nil {
				fmt.Fprintf(os.Stderr, "GPU %s: %v\n", be, err)
				continue
			}
			fmt.Print(mir2gpu.FormatResult(result))
		}
		return nil
	}

	if backend == "c" {
		text, err := mir2c.Compile(steps.MIR2Module)
		if err != nil {
			return fmt.Errorf("C compile: %w", err)
		}
		out := outputFile
		if out != "" {
			return os.WriteFile(out, []byte(text), 0644)
		}
		out = strings.TrimSuffix(sourceFile, ext) + ".generated.c"
		flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
		if forceOutput {
			flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		}
		file, err := os.OpenFile(out, flags, 0644)
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists; use --force or an explicit -o to overwrite", out)
		}
		if err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
		_, writeErr := file.WriteString(text)
		closeErr := file.Close()
		if writeErr != nil {
			return fmt.Errorf("write %s: %w", out, writeErr)
		}
		return closeErr
	}
	if steps.Assembly == "" {
		return nil
	}
	asmSrc := steps.Assembly

	// Determine output path
	base := sourceFile[:len(sourceFile)-len(ext)]
	out := outputFile

	// Emit assembly if output is .a80/.asm or no output flag given
	asmExt := ".a80"
	if backend == "ez80" {
		asmExt = ".asm"
	}
	if out == "" || filepath.Ext(out) == ".a80" || filepath.Ext(out) == ".asm" {
		if out == "" {
			out = base + asmExt
		}
		if err := os.WriteFile(out, []byte(asmSrc), 0644); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
		if debug {
			fmt.Printf("Wrote assembly to %s\n", out)
		}
		return nil
	}

	// Otherwise assemble to binary
	asmTarget := target
	if runtimeMode == "agon-z80" {
		asmTarget = "agon-z80"
	}
	if backend == "ez80" {
		asmTarget = "agon" // eZ80 backend always targets Agon Light 2
	}
	// Profile evidence measures the default assembler at the explicit all-RAM
	// origin. Reuse that exact configuration, including compact binary layout.
	if opts.ProfileSelection != nil || jointCompiled {
		r, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asmSrc)
		if err != nil {
			return err
		}
		if len(r.Errors) != 0 {
			return fmt.Errorf("profile assembly: %v", r.Errors)
		}
		return os.WriteFile(out, r.Binary, 0644)
	}
	bin, errs := pipeline.Assemble(asmSrc, asmTarget)
	if len(errs) > 0 {
		return fmt.Errorf("assemble: %v", errs[0])
	}
	if err := os.WriteFile(out, bin, 0644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	if debug {
		fmt.Printf("Wrote binary to %s (%d bytes)\n", out, len(bin))
	}
	return nil
}

func compilePLMToNanz(sourceFile string) error {
	src, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", sourceFile, err)
	}

	hirMod, err := plm.Compile(string(src))
	if err != nil {
		return fmt.Errorf("PL/M compile: %w", err)
	}

	nanzSrc := nanz.Print(hirMod)

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(nanzSrc), 0644); err != nil {
			return fmt.Errorf("write %s: %w", outputFile, err)
		}
		if debug {
			fmt.Printf("Wrote Nanz source to %s\n", outputFile)
		}
	} else {
		fmt.Print(nanzSrc)
	}
	return nil
}

func assembleFile(sourceFile string) error {
	if debug {
		fmt.Printf("Assembling %s...\n", sourceFile)
	}

	// Determine target platform (symbols, ORG, stdlib)
	asmTarget := z80asm.TargetGeneric
	if target != "" {
		t, err := z80asm.ParseTarget(target)
		if err != nil {
			return fmt.Errorf("unknown target: %s", target)
		}
		asmTarget = t
	}

	// Determine output format (how to package the binary).
	// Priority: -f flag > -o extension > default "code"
	chosenFormat := outputFormat
	if chosenFormat == "" && outputFile != "" {
		switch filepath.Ext(outputFile) {
		case ".sna":
			chosenFormat = "sna"
		case ".tap":
			chosenFormat = "tap"
		case ".com":
			chosenFormat = "com"
		case ".rom":
			chosenFormat = "msxrom"
		}
	}
	if chosenFormat == "" {
		chosenFormat = "code" // default: raw binary
	}

	outFmt := z80asm.LookupOutputFormat(chosenFormat)
	if outFmt == nil {
		return fmt.Errorf("unknown output format: %s (available: code, sna, tap, com, msxrom, agon)", chosenFormat)
	}

	// Determine output filename
	if outputFile == "" {
		base := sourceFile[:len(sourceFile)-len(filepath.Ext(sourceFile))]
		outputFile = base + outFmt.Extension
	}

	// Create and configure assembler
	assembler := z80asm.NewAssembler()
	if err := assembler.SetTarget(asmTarget); err != nil {
		return fmt.Errorf("failed to set assembler target: %w", err)
	}

	// Assemble
	result, err := assembler.AssembleFile(sourceFile)
	if err != nil {
		return fmt.Errorf("assembly failed: %w", err)
	}

	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %v\n", e)
		}
		return fmt.Errorf("assembly failed with %d errors", len(result.Errors))
	}

	// Generate output using the chosen format
	var outputData []byte
	outputData, err = outFmt.Generator(result)
	if err != nil {
		return fmt.Errorf("failed to generate %s output: %w", chosenFormat, err)
	}

	// Write output
	if err := os.WriteFile(outputFile, outputData, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if debug {
		fmt.Printf("Assembled %s -> %s (%d bytes, format: %s)\n", sourceFile, outputFile, len(outputData), chosenFormat)
	}

	// Print warnings
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	return nil
}

// addTASSupport adds TAS debugging hooks to generated assembly
func addTASSupport(asmFile string) error {
	// TODO: Add TAS debugging hooks to assembly
	// For now, just add a comment marker
	return nil
}

// generateVisualization generates a DOT file for MIR visualization.
// DEPRECATED: MIR1 visualizer archived. Use --emit mir2 instead.
func generateVisualization(module *ir.Module, filename string) error {
	return fmt.Errorf("MIR1 visualizer deprecated — use --emit mir2 for MIR2 dump")
}

// ── GPU backend support ────────────────────────────────────────────────────────

// gpuBackendFromFlag maps -b flag values to mir2gpu backends.
func gpuBackendFromFlag(flag string) (mir2gpu.Backend, bool) {
	switch flag {
	case "cuda":
		return mir2gpu.CUDA, true
	case "opencl":
		return mir2gpu.OpenCL, true
	case "vulkan":
		return mir2gpu.Vulkan, true
	case "metal":
		return mir2gpu.Metal, true
	}
	return 0, false
}

// compileToGPU compiles an HIR module to GPU compute shader source.
func compileToGPU(hirMod *hir.Module, gpuBE mir2gpu.Backend, sourceFile string) error {
	// HIR → MIR2
	m := hir.LowerModule(hirMod)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}

	// Auto-detect GPU candidates
	candidates := mir2gpu.DetectGPUCandidates(m)
	if debug && len(candidates) > 0 {
		fmt.Fprintf(os.Stderr, "GPU candidates detected:\n%s", mir2gpu.FormatCandidates(candidates))
	}

	// Compile MIR2 → GPU source
	code, err := mir2gpu.Compile(m, mir2gpu.CompileOptions{Backend: gpuBE})
	if err != nil {
		return fmt.Errorf("GPU compile (%s): %w", gpuBE, err)
	}

	// Determine output file
	ext := filepath.Ext(sourceFile)
	base := sourceFile[:len(sourceFile)-len(ext)]
	out := outputFile
	if out == "" {
		out = base + gpuBE.Ext()
	}

	if err := os.WriteFile(out, []byte(code), 0644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Fprintf(os.Stderr, "Wrote %s shader to %s (%d bytes, %d functions)\n",
		gpuBE, out, len(code), len(m.Funcs))

	return nil
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

		// SMC param annotations (Option B)
		for _, ann := range fn.SMCParamAnnotations {
			fmt.Fprintf(file, "  %s\n", ann.String())
		}
		if fn.SMCReturnAnnotation != nil {
			fmt.Fprintf(file, "  %s\n", fn.SMCReturnAnnotation.String())
		}

		// Instructions
		fmt.Fprintf(file, "  Instructions:\n")
		for i, inst := range fn.Instructions {
			// Output SMC annotations before instruction (Option B)
			for _, ann := range inst.SMCAnnotations {
				fmt.Fprintf(file, "         %s\n", ann.String())
			}
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
				// Use the instruction's String() method for proper formatting
				fmt.Fprintf(file, "%s", inst.String())
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
