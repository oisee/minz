// mzv - MinZ Virtual Machine (MIR Interpreter)
// Executes MIR intermediate representation directly for testing and debugging
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/mirvm"
)

func main() {
	var (
		input       = flag.String("i", "", "Input MIR file")
		output      = flag.String("o", "", "Output file (memory dump or PNG)")
		debug       = flag.Bool("d", false, "Enable debug output")
		trace       = flag.Bool("trace", false, "Trace execution")
		breakpoints = flag.String("bp", "", "Comma-separated list of breakpoints (e.g., main:5,helper:10)")
		maxSteps    = flag.Int("max-steps", 1000000, "Maximum execution steps (prevent infinite loops)")
		memSize     = flag.Int("mem", 65536, "Memory size in bytes")
		stackSize   = flag.Int("stack", 4096, "Stack size in bytes")
		verbose     = flag.Bool("v", false, "Verbose output")
		platform    = flag.String("platform", "headless", "Platform: headless, spectrum, agon")
		pngOutput   = flag.String("png", "", "Export framebuffer to PNG file")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "MinZ Virtual Machine (MIR Interpreter) v0.2.0\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -i input.mir [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nPlatforms:\n")
		fmt.Fprintf(os.Stderr, "  headless  - No display (default)\n")
		fmt.Fprintf(os.Stderr, "  spectrum  - ZX Spectrum 256x192\n")
		fmt.Fprintf(os.Stderr, "  agon      - Agon Light 320x240\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -i program.mir                           # Run MIR program\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -i graphics.mir -platform agon -png out.png  # Render to PNG\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -i program.mir -trace                    # Trace execution\n", os.Args[0])
	}

	flag.Parse()

	if *input == "" {
		// Check if input file was provided as positional argument
		if flag.NArg() > 0 {
			*input = flag.Arg(0)
		} else {
			fmt.Fprintf(os.Stderr, "Error: input MIR file required\n")
			flag.Usage()
			os.Exit(1)
		}
	}

	// Read MIR file
	mirData, err := ioutil.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading MIR file: %v\n", err)
		os.Exit(1)
	}

	// Parse MIR
	module, err := ir.ParseMIR(string(mirData))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing MIR: %v\n", err)
		os.Exit(1)
	}

	// Create platform based on -platform flag
	var plat mirvm.Platform
	switch *platform {
	case "spectrum":
		plat = mirvm.NewSpectrumPlatform()
		if *verbose {
			fmt.Fprintf(os.Stderr, "Platform: ZX Spectrum (256x192)\n")
		}
	case "agon":
		plat = mirvm.NewAgonPlatform()
		if *verbose {
			fmt.Fprintf(os.Stderr, "Platform: Agon Light (320x240)\n")
		}
	default:
		plat = mirvm.NewHeadlessPlatform()
	}

	// Create VM configuration
	config := mirvm.Config{
		MemorySize:   *memSize,
		StackSize:    *stackSize,
		Debug:        *debug,
		Trace:        *trace,
		MaxSteps:     *maxSteps,
		Verbose:      *verbose,
		OutputStream: os.Stdout,
		Platform:     plat,
	}

	// Parse breakpoints
	if *breakpoints != "" {
		config.Breakpoints = parseBreakpoints(*breakpoints)
	}

	// Create and initialize VM
	vm := mirvm.New(config)

	// Load module into VM
	if err := vm.LoadModule(module); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading module: %v\n", err)
		os.Exit(1)
	}

	// Run the program (starts from main function)
	exitCode, err := vm.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}

	// Export framebuffer to PNG if requested
	if *pngOutput != "" && plat.HasDisplay() {
		display := plat.Display().(*mirvm.GenericDisplay)
		if err := display.SavePNG(*pngOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving PNG: %v\n", err)
			os.Exit(1)
		}
		if *verbose {
			fmt.Fprintf(os.Stderr, "Framebuffer saved to: %s\n", *pngOutput)
		}
	}

	// Write memory dump if specified
	if *output != "" {
		data := vm.GetMemoryDump()
		if err := ioutil.WriteFile(*output, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
	}

	// Print statistics if verbose
	if *verbose {
		stats := vm.GetStatistics()
		fmt.Fprintf(os.Stderr, "\nExecution Statistics:\n")
		fmt.Fprintf(os.Stderr, "  Instructions executed: %d\n", stats.InstructionsExecuted)
		fmt.Fprintf(os.Stderr, "  Functions called: %d\n", stats.FunctionsCalled)
		fmt.Fprintf(os.Stderr, "  Max stack depth: %d\n", stats.MaxStackDepth)
		fmt.Fprintf(os.Stderr, "  Memory used: %d bytes\n", stats.MemoryUsed)
	}

	os.Exit(exitCode)
}

// parseBreakpoints parses comma-separated breakpoint specifications
// Format: function:line or function:instruction_index
func parseBreakpoints(spec string) map[string][]int {
	breakpoints := make(map[string][]int)
	
	for _, bp := range strings.Split(spec, ",") {
		parts := strings.Split(strings.TrimSpace(bp), ":")
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Warning: invalid breakpoint format: %s\n", bp)
			continue
		}
		
		funcName := parts[0]
		var line int
		if _, err := fmt.Sscanf(parts[1], "%d", &line); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid breakpoint line: %s\n", parts[1])
			continue
		}
		
		breakpoints[funcName] = append(breakpoints[funcName], line)
	}
	
	return breakpoints
}