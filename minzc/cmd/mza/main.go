package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
)

func main() {
	var (
		outputFile    = flag.String("o", "", "Output binary file (default: input.bin)")
		listingFile   = flag.String("l", "", "Generate listing file")
		symbolFile    = flag.String("s", "", "Generate symbol file")
		allowUndoc    = flag.Bool("undoc", true, "Allow undocumented Z80 instructions")
		strict        = flag.Bool("strict", false, "Strict assembly mode")
		caseSensitive = flag.Bool("case", false, "Case-sensitive labels")
		verbose       = flag.Bool("v", false, "Verbose output")
		help          = flag.Bool("h", false, "Show help")
	)
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "mza - MinZ Z80 Assembler v1.0\n\n")
		fmt.Fprintf(os.Stderr, "Usage: mza [options] input.a80\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  mza program.a80                     # Assemble to program.bin\n")
		fmt.Fprintf(os.Stderr, "  mza -o game.rom program.a80         # Assemble to game.rom\n")
		fmt.Fprintf(os.Stderr, "  mza -l program.lst program.a80      # Generate listing file\n")
		fmt.Fprintf(os.Stderr, "  mza -s symbols.sym program.a80      # Generate symbol file\n")
	}
	
	flag.Parse()
	
	if *help {
		flag.Usage()
		return
	}
	
	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Error: No input file specified\n")
		flag.Usage()
		os.Exit(1)
	}
	
	if flag.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "Error: Multiple input files not supported\n")
		flag.Usage()
		os.Exit(1)
	}
	
	inputFile := flag.Arg(0)
	
	// Validate input file extension
	if !strings.HasSuffix(strings.ToLower(inputFile), ".a80") {
		fmt.Fprintf(os.Stderr, "Warning: Input file doesn't have .a80 extension\n")
	}
	
	// Determine output file name
	if *outputFile == "" {
		ext := filepath.Ext(inputFile)
		base := strings.TrimSuffix(inputFile, ext)
		*outputFile = base + ".bin"
	}
	
	if *verbose {
		fmt.Printf("MinZ Z80 Assembler v1.0\n")
		fmt.Printf("Input:  %s\n", inputFile)
		fmt.Printf("Output: %s\n", *outputFile)
		if *listingFile != "" {
			fmt.Printf("Listing: %s\n", *listingFile)
		}
		if *symbolFile != "" {
			fmt.Printf("Symbols: %s\n", *symbolFile)
		}
		fmt.Println()
	}
	
	// Create assembler with configuration
	assembler := z80asm.NewAssembler()
	assembler.AllowUndocumented = *allowUndoc
	assembler.Strict = *strict
	assembler.CaseSensitive = *caseSensitive
	
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
	if err := os.WriteFile(*outputFile, result.Binary, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write output file %s: %v\n", *outputFile, err)
		os.Exit(1)
	}
	
	// Generate listing file if requested
	if *listingFile != "" {
		if err := generateListingFile(*listingFile, result); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write listing file %s: %v\n", *listingFile, err)
			os.Exit(1)
		}
	}
	
	// Generate symbol file if requested
	if *symbolFile != "" {
		if err := generateSymbolFile(*symbolFile, result); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write symbol file %s: %v\n", *symbolFile, err)
			os.Exit(1)
		}
	}
	
	// Print summary
	if *verbose || len(result.Warnings) > 0 {
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