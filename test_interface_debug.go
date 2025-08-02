package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	
	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/semantic"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test_interface_debug <file.minz>")
		os.Exit(1)
	}

	filename := os.Args[1]
	
	// Parse
	p := parser.NewParser()
	ast, err := p.ParseFile(filename)
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	// Semantic analysis
	analyzer := semantic.NewAnalyzer()
	module, err := analyzer.Analyze(ast)
	if err != nil {
		log.Fatalf("Semantic error: %v", err)
	}

	// Debug: Print all functions and their parameters
	fmt.Println("\n=== FUNCTIONS AND PARAMETERS ===")
	for _, fn := range module.Functions {
		fmt.Printf("Function %s:\n", fn.Name)
		fmt.Printf("  Parameters (%d):\n", len(fn.Params))
		for i, param := range fn.Params {
			fmt.Printf("    [%d] %s: %s (IsTSMCRef=%v)\n", i, param.Name, param.Type, param.IsTSMCRef)
		}
		fmt.Printf("  SMC enabled: %v\n", fn.IsSMCEnabled)
		fmt.Printf("  SMC offsets: %v\n", fn.SMCParamOffsets)
	}
	fmt.Println("================================")

	// Try code generation
	outFile := filepath.Base(filename)
	outFile = outFile[:len(outFile)-len(filepath.Ext(outFile))] + ".a80"
	
	gen := codegen.NewZ80Generator(os.Stdout)
	if err := gen.Generate(module); err != nil {
		log.Fatalf("Code generation error: %v", err)
	}
	
	fmt.Printf("\nCode generation completed (output to stdout)\n")
}