package main

import (
	"fmt"
	"os"

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
	// TODO: Implement compilation pipeline
	fmt.Printf("Compiling %s...\n", sourceFile)
	return fmt.Errorf("compiler not yet implemented")
}