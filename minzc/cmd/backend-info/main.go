package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	
	"github.com/minz/minzc/pkg/codegen"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "features" {
		showFeatureMatrix()
		return
	}
	
	fmt.Println("MinZ Backend Information Tool")
	fmt.Println("=============================")
	fmt.Println()
	
	// List all backends
	backends := codegen.ListBackends()
	sort.Strings(backends)
	
	fmt.Printf("Available backends: %d\n", len(backends))
	fmt.Println()
	
	// Create a table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	
	// Header
	fmt.Fprintln(w, "Backend\tExtension\tSMC\tInterrupts\tPointers\tFloat\tInline ASM")
	fmt.Fprintln(w, "-------\t---------\t---\t----------\t--------\t-----\t----------")
	
	// Check each backend
	for _, name := range backends {
		backend := codegen.GetBackend(name, nil)
		if backend == nil {
			continue
		}
		
		ext := backend.GetFileExtension()
		smc := yesNo(backend.SupportsFeature(codegen.FeatureSelfModifyingCode))
		interrupts := yesNo(backend.SupportsFeature(codegen.FeatureInterrupts))
		
		// Determine pointer size
		pointers := "?"
		if backend.SupportsFeature(codegen.Feature16BitPointers) {
			pointers = "16-bit"
		} else if backend.SupportsFeature(codegen.Feature24BitPointers) {
			pointers = "24-bit"
		} else if backend.SupportsFeature(codegen.Feature32BitPointers) {
			pointers = "32-bit"
		}
		
		float := yesNo(backend.SupportsFeature(codegen.FeatureFloatingPoint))
		inlineAsm := yesNo(backend.SupportsFeature(codegen.FeatureInlineAssembly))
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			name, ext, smc, interrupts, pointers, float, inlineAsm)
	}
	
	w.Flush()
	
	fmt.Println()
	fmt.Println("Use 'backend-info features' to see detailed feature matrix")
}

func showFeatureMatrix() {
	fmt.Println("MinZ Backend Feature Matrix")
	fmt.Println("===========================")
	fmt.Println()
	
	// Get all backends
	backends := codegen.ListBackends()
	sort.Strings(backends)
	
	// List of features to check
	features := []struct {
		name  string
		label string
	}{
		{codegen.FeatureSelfModifyingCode, "Self-Modifying Code"},
		{codegen.FeatureInterrupts, "Interrupts"},
		{codegen.FeatureShadowRegisters, "Shadow Registers"},
		{codegen.Feature16BitPointers, "16-bit Pointers"},
		{codegen.Feature24BitPointers, "24-bit Pointers"},
		{codegen.Feature32BitPointers, "32-bit Pointers"},
		{codegen.FeatureFloatingPoint, "Floating Point"},
		{codegen.FeatureFixedPoint, "Fixed Point"},
		{codegen.FeatureInlineAssembly, "Inline Assembly"},
		{codegen.FeatureIndirectCalls, "Indirect Calls"},
		{codegen.FeatureBitManipulation, "Bit Manipulation"},
		{codegen.FeatureZeroPage, "Zero Page"},
		{codegen.FeatureBlockInstructions, "Block Instructions"},
		{codegen.FeatureHardwareMultiply, "Hardware Multiply"},
		{codegen.FeatureHardwareDivide, "Hardware Divide"},
	}
	
	// Create header
	header := "Feature"
	for _, backend := range backends {
		header += fmt.Sprintf("\t%s", strings.ToUpper(backend))
	}
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, header)
	
	// Separator
	sep := strings.Repeat("-", 20)
	for range backends {
		sep += "\t---"
	}
	fmt.Fprintln(w, sep)
	
	// Check each feature
	for _, feature := range features {
		row := feature.label
		
		for _, backendName := range backends {
			backend := codegen.GetBackend(backendName, nil)
			if backend != nil && backend.SupportsFeature(feature.name) {
				row += "\t✓"
			} else {
				row += "\t-"
			}
		}
		
		fmt.Fprintln(w, row)
	}
	
	w.Flush()
	
	fmt.Println()
	fmt.Println("Legend: ✓ = Supported, - = Not supported")
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}