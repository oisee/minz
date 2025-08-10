package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
	"github.com/spf13/cobra"
)

var (
	outputFile    string
	listingFile   string
	symbolFile    string
	allowUndoc    bool
	strict        bool
	caseSensitive bool
	verbose       bool
)

var rootCmd = &cobra.Command{
	Use:   "mza [input.a80]",
	Short: "MinZ Z80 Assembler v1.0",
	Long: `mza - MinZ Z80 Assembler v1.0
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Modern Z80 assembler with character literals and enhanced syntax

FEATURES:
  • Character literals:  LD A, 'H'  or  LD A, "H"
  • Escape sequences:    LD A, '\n' (newline), '\t' (tab)
  • String data:         DB "Hello, World!", 13, 10
  • Mixed operands:      DB "Text", 0, 'X', $FF
  • Hex formats:         $FF, 0xFF, #FF, FFh
  • Binary format:       %10101010
  • Undocumented ops:    SLL, IXH, IXL, IYH, IYL

DIRECTIVES:
  ORG $8000           Set origin address
  DB/DEFB             Define bytes
  DW/DEFW             Define words (16-bit)
  DS/DEFS             Define space
  EQU                 Define constant
  END                 End of source

EXAMPLES:
  mza program.a80                     # Assemble to program.bin
  mza -o game.rom program.a80         # Custom output file
  mza -l program.lst program.a80      # Generate listing
  mza -s symbols.sym program.a80      # Generate symbol table
  mza -v program.a80                  # Verbose output`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]
		
		// Validate input file extension
		if !strings.HasSuffix(strings.ToLower(inputFile), ".a80") {
			fmt.Fprintf(os.Stderr, "Warning: Input file doesn't have .a80 extension\n")
		}
		
		// Determine output file name
		if outputFile == "" {
			ext := filepath.Ext(inputFile)
			base := strings.TrimSuffix(inputFile, ext)
			outputFile = base + ".bin"
		}
		
		if verbose {
			fmt.Printf("MinZ Z80 Assembler v1.0\n")
			fmt.Printf("Input:  %s\n", inputFile)
			fmt.Printf("Output: %s\n", outputFile)
			if listingFile != "" {
				fmt.Printf("Listing: %s\n", listingFile)
			}
			if symbolFile != "" {
				fmt.Printf("Symbols: %s\n", symbolFile)
			}
			fmt.Println()
		}
		
		// Create assembler with configuration
		assembler := z80asm.NewAssembler()
		assembler.AllowUndocumented = allowUndoc
		assembler.Strict = strict
		assembler.CaseSensitive = caseSensitive
		
		// Assemble the file
		result, err := assembler.AssembleFile(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Assembly failed: %v\n", err)
			os.Exit(1)
		}
		
		// Check for assembly errors
		if len(result.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "Assembly errors:\n")
			for _, err := range result.Errors {
				fmt.Fprintf(os.Stderr, "  %v\n", err)
			}
			os.Exit(1)
		}
		
		// Write binary output
		if err := os.WriteFile(outputFile, result.Binary, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write output file %s: %v\n", outputFile, err)
			os.Exit(1)
		}
		
		// Generate listing file if requested
		if listingFile != "" {
			if err := generateListingFile(listingFile, result); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write listing file %s: %v\n", listingFile, err)
				os.Exit(1)
			}
		}
		
		// Generate symbol file if requested
		if symbolFile != "" {
			if err := generateSymbolFile(symbolFile, result); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write symbol file %s: %v\n", symbolFile, err)
				os.Exit(1)
			}
		}
		
		// Print summary
		if verbose || len(result.Warnings) > 0 {
			fmt.Printf("Assembly completed successfully:\n")
			fmt.Printf("  Origin: $%04X\n", result.Origin)
			fmt.Printf("  Size: %d bytes ($%04X)\n", result.Size, result.Size)
			fmt.Printf("  Symbols: %d\n", len(result.Symbols))
			
			if len(result.Warnings) > 0 {
				fmt.Printf("  Warnings:\n")
				for _, warning := range result.Warnings {
					fmt.Printf("    %s\n", warning)
				}
			}
		}
	},
}

func init() {
	// Output options
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output binary file (default: input.bin)")
	rootCmd.Flags().StringVarP(&listingFile, "listing", "l", "", "generate listing file")
	rootCmd.Flags().StringVarP(&symbolFile, "symbols", "s", "", "generate symbol file")
	
	// Assembly options
	rootCmd.Flags().BoolVarP(&allowUndoc, "undocumented", "u", true, "allow undocumented Z80 instructions")
	rootCmd.Flags().BoolVar(&strict, "strict", false, "strict assembly mode")
	rootCmd.Flags().BoolVarP(&caseSensitive, "case-sensitive", "c", false, "case-sensitive labels")
	
	// General options
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// generateListingFile creates a listing file with addresses and machine code
func generateListingFile(filename string, result *z80asm.Result) error {
	var lines []string
	
	lines = append(lines, "MinZ Z80 Assembler Listing")
	lines = append(lines, "==========================")
	lines = append(lines, "")
	
	for _, line := range result.Listing {
		if len(line.Bytes) > 0 {
			// Format: "8000  21 34 12    LD HL,$1234"
			codeHex := ""
			for i, b := range line.Bytes {
				if i > 0 {
					codeHex += " "
				}
				codeHex += fmt.Sprintf("%02X", b)
			}
			lines = append(lines, fmt.Sprintf("%04X  %-12s %s", 
				line.Address, codeHex, line.SourceLine))
		} else {
			// Format: "             ; comment or directive"
			lines = append(lines, fmt.Sprintf("              %s", line.SourceLine))
		}
	}
	
	content := strings.Join(lines, "\n")
	return os.WriteFile(filename, []byte(content), 0644)
}

// generateSymbolFile creates a symbol file with label definitions
func generateSymbolFile(filename string, result *z80asm.Result) error {
	var lines []string
	
	lines = append(lines, "MinZ Z80 Assembler Symbol Table")
	lines = append(lines, "==============================")
	lines = append(lines, "")
	
	for name, addr := range result.Symbols {
		lines = append(lines, fmt.Sprintf("%-20s = $%04X (%d)", name, addr, addr))
	}
	
	content := strings.Join(lines, "\n")
	return os.WriteFile(filename, []byte(content), 0644)
}