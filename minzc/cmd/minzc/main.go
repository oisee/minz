package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/semantic"
	"github.com/spf13/cobra"
)

var (
	version   = "0.1.0"
	outputFile string
	optimize  bool
	debug     bool
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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func compile(sourceFile string) error {
	fmt.Printf("Compiling %s...\n", sourceFile)

	// Parse the source file
	parser := parser.New()
	astFile, err := parser.ParseFile(sourceFile)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Perform semantic analysis
	analyzer := semantic.NewAnalyzer()
	irModule, err := analyzer.Analyze(astFile)
	if err != nil {
		return fmt.Errorf("semantic error: %w", err)
	}

	// Determine output filename
	if outputFile == "" {
		base := filepath.Base(sourceFile)
		ext := filepath.Ext(base)
		outputFile = base[:len(base)-len(ext)] + ".a80"
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