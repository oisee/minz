package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/module"
	"github.com/minz/minzc/pkg/optimizer"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/semantic"
	"github.com/spf13/cobra"
)

var (
	version     = "0.9.1"
	outputFile  string
	optimize    bool
	debug       bool
	enableSMC   bool
)

var rootCmd = &cobra.Command{
	Use:   "minzc [source file]",
	Short: "MinZ to Z80 Assembly Compiler",
	Long:  `minzc compiles MinZ source code to Z80 assembly in sjasmplus .a80 format`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sourceFile := args[0]
		if err := compile(sourceFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file (default: input.a80)")
	rootCmd.Flags().BoolVarP(&optimize, "optimize", "O", false, "enable optimizations")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug output")
	rootCmd.Flags().BoolVar(&enableSMC, "enable-smc", false, "enable all self-modifying code optimizations including TRUE SMC (requires code in RAM)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func compile(sourceFile string) error {
	fmt.Printf("Compiling %s...\n", sourceFile)

	// Find project root (directory containing the source file or its parent)
	projectRoot := filepath.Dir(sourceFile)
	
	// Create module manager
	_ = module.NewModuleManager(projectRoot)

	// Parse the source file
	parser := parser.New()
	astFile, err := parser.ParseFile(sourceFile)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Set up module name if not explicitly declared
	if astFile.ModuleName == "" {
		astFile.ModuleName = module.ExtractModuleName(sourceFile)
	}

	// Perform semantic analysis with module support
	analyzer := semantic.NewAnalyzer()
	// TODO: Set module resolver on analyzer
	irModule, err := analyzer.Analyze(astFile)
	if err != nil {
		return fmt.Errorf("semantic error: %w", err)
	}
	defer analyzer.Close()
	
	// Enable SMC for all functions if flag is set OR if optimizing
	if enableSMC || optimize {
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

	// Determine output filename
	if outputFile == "" {
		base := filepath.Base(sourceFile)
		ext := filepath.Ext(base)
		outputFile = base[:len(base)-len(ext)] + ".a80"
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

	// Generate Z80 assembly
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	generator := codegen.NewZ80Generator(outFile)
	if err := generator.Generate(irModule); err != nil {
		return fmt.Errorf("code generation error: %w", err)
	}

	fmt.Printf("Successfully compiled to %s\n", outputFile)
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