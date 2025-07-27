package optimizer

import (
	"fmt"
	"strings"
	"github.com/minz/minzc/pkg/ir"
)

// RecursionType describes the type of recursion detected
type RecursionType int

const (
	RecursionNone RecursionType = iota
	RecursionDirect     // f() calls f()
	RecursionMutual     // f() calls g(), g() calls f()
	RecursionIndirect   // f() calls g(), g() calls h(), h() calls f()
)

// RecursionInfo contains detailed information about detected recursion
type RecursionInfo struct {
	Type  RecursionType
	Cycle []string  // The cycle path (e.g., ["f", "g", "h", "f"])
	Depth int       // Length of the shortest cycle
}

// RecursionDetector analyzes functions to detect recursion
type RecursionDetector struct {
	callGraph     map[string][]string
	visited       map[string]bool
	recursionPath map[string]bool
	cyclePath     []string  // Track current path for cycle detection
	recursionInfo map[string]*RecursionInfo // Store detailed recursion info
	diagnostics   bool      // Enable diagnostic output
}

// NewRecursionDetector creates a new recursion detector
func NewRecursionDetector() *RecursionDetector {
	return &RecursionDetector{
		callGraph:     make(map[string][]string),
		visited:       make(map[string]bool),
		recursionPath: make(map[string]bool),
		cyclePath:     make([]string, 0),
		recursionInfo: make(map[string]*RecursionInfo),
		diagnostics:   true, // Enable diagnostics for better analysis
	}
}

// NewRecursionDetectorWithDiagnostics creates a recursion detector with diagnostic control
func NewRecursionDetectorWithDiagnostics(enableDiagnostics bool) *RecursionDetector {
	detector := NewRecursionDetector()
	detector.diagnostics = enableDiagnostics
	return detector
}

// AnalyzeModule detects recursive functions in a module
func (d *RecursionDetector) AnalyzeModule(module *ir.Module) {
	// Build call graph
	d.buildCallGraph(module)
	
	if d.diagnostics {
		d.printCallGraph()
	}
	
	// Detect recursion for each function
	for _, fn := range module.Functions {
		info := d.analyzeRecursion(fn.Name)
		if info.Type != RecursionNone {
			fn.IsRecursive = true
			d.recursionInfo[fn.Name] = info
			
			if d.diagnostics {
				d.printRecursionInfo(fn.Name, info)
			}
		}
	}
	
	if d.diagnostics {
		d.printSummary(module)
	}
}

// buildCallGraph builds a graph of function calls
func (d *RecursionDetector) buildCallGraph(module *ir.Module) {
	// Create a map for quick function name resolution
	funcNames := make(map[string]string) // short name -> full name
	for _, fn := range module.Functions {
		funcNames[d.getShortName(fn.Name)] = fn.Name
		funcNames[fn.Name] = fn.Name // also map full name to itself
	}
	
	// Build call graph with proper name resolution
	for _, fn := range module.Functions {
		d.callGraph[fn.Name] = []string{}
		
		// Find all function calls
		for _, inst := range fn.Instructions {
			if inst.Op == ir.OpCall && inst.Symbol != "" {
				// Resolve the called function name
				targetName := inst.Symbol
				if fullName, exists := funcNames[targetName]; exists {
					targetName = fullName
				}
				
				// Add to call graph (avoid duplicates)
				if !d.contains(d.callGraph[fn.Name], targetName) {
					d.callGraph[fn.Name] = append(d.callGraph[fn.Name], targetName)
				}
			}
		}
	}
}

// getShortName extracts the short name from a fully qualified name
func (d *RecursionDetector) getShortName(fullName string) string {
	if idx := strings.LastIndex(fullName, "."); idx >= 0 {
		return fullName[idx+1:]
	}
	return fullName
}

// contains checks if a slice contains a string
func (d *RecursionDetector) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// analyzeRecursion performs comprehensive recursion analysis for a function
func (d *RecursionDetector) analyzeRecursion(funcName string) *RecursionInfo {
	// Reset state for this analysis
	d.visited = make(map[string]bool)
	d.recursionPath = make(map[string]bool)
	d.cyclePath = []string{}
	
	// Perform DFS cycle detection
	cycle := d.findRecursiveCycle(funcName, funcName)
	
	if len(cycle) == 0 {
		return &RecursionInfo{Type: RecursionNone}
	}
	
	// Determine recursion type
	recType := RecursionDirect
	if len(cycle) == 2 && cycle[0] == cycle[1] {
		recType = RecursionDirect
	} else if len(cycle) == 3 {
		recType = RecursionMutual
	} else if len(cycle) > 3 {
		recType = RecursionIndirect
	}
	
	return &RecursionInfo{
		Type:  recType,
		Cycle: cycle,
		Depth: len(cycle) - 1, // Subtract 1 because cycle includes the start node twice
	}
}

// findRecursiveCycle performs DFS to find recursion cycles
func (d *RecursionDetector) findRecursiveCycle(startFunc, currentFunc string) []string {
	// Add current function to path
	d.cyclePath = append(d.cyclePath, currentFunc)
	d.visited[currentFunc] = true
	d.recursionPath[currentFunc] = true
	
	// Check all functions called by current function
	for _, calledFunc := range d.callGraph[currentFunc] {
		// Check if we found a cycle back to the start function
		if calledFunc == startFunc {
			// Found cycle! Return the path
			cycle := make([]string, len(d.cyclePath)+1)
			copy(cycle, d.cyclePath)
			cycle[len(cycle)-1] = startFunc // Close the cycle
			return cycle
		}
		
		// Check if we're already exploring this path (indirect cycle)
		if d.recursionPath[calledFunc] {
			// Found a cycle, but need to extract the relevant part
			startIdx := -1
			for i, f := range d.cyclePath {
				if f == calledFunc {
					startIdx = i
					break
				}
			}
			if startIdx >= 0 {
				cycle := make([]string, len(d.cyclePath)-startIdx+1)
				copy(cycle, d.cyclePath[startIdx:])
				cycle[len(cycle)-1] = calledFunc // Close the cycle
				return cycle
			}
		}
		
		// Continue DFS if not visited
		if !d.visited[calledFunc] {
			cycle := d.findRecursiveCycle(startFunc, calledFunc)
			if len(cycle) > 0 {
				return cycle
			}
		}
	}
	
	// Backtrack
	d.cyclePath = d.cyclePath[:len(d.cyclePath)-1]
	d.recursionPath[currentFunc] = false
	return nil
}

// isRecursive checks if a function is recursive (legacy interface)
func (d *RecursionDetector) isRecursive(funcName string) bool {
	info := d.analyzeRecursion(funcName)
	return info.Type != RecursionNone
}

// printCallGraph prints the call graph for debugging
func (d *RecursionDetector) printCallGraph() {
	fmt.Println("\n=== CALL GRAPH ANALYSIS ===")
	for funcName, calls := range d.callGraph {
		shortName := d.getShortName(funcName)
		if len(calls) == 0 {
			fmt.Printf("  %s: (no calls)\n", shortName)
		} else {
			callNames := make([]string, len(calls))
			for i, call := range calls {
				callNames[i] = d.getShortName(call)
			}
			fmt.Printf("  %s → %s\n", shortName, strings.Join(callNames, ", "))
		}
	}
}

// printRecursionInfo prints detailed recursion information
func (d *RecursionDetector) printRecursionInfo(funcName string, info *RecursionInfo) {
	shortName := d.getShortName(funcName)
	
	switch info.Type {
	case RecursionDirect:
		fmt.Printf("  🔄 %s: DIRECT recursion (calls itself)\n", shortName)
	case RecursionMutual:
		cycle := make([]string, len(info.Cycle))
		for i, f := range info.Cycle {
			cycle[i] = d.getShortName(f)
		}
		fmt.Printf("  🔁 %s: MUTUAL recursion: %s\n", shortName, strings.Join(cycle, " → "))
	case RecursionIndirect:
		cycle := make([]string, len(info.Cycle))
		for i, f := range info.Cycle {
			cycle[i] = d.getShortName(f)
		}
		fmt.Printf("  🌀 %s: INDIRECT recursion (depth %d): %s\n", shortName, info.Depth, strings.Join(cycle, " → "))
	}
}

// printSummary prints a summary of recursion analysis
func (d *RecursionDetector) printSummary(module *ir.Module) {
	recursiveCount := 0
	directCount := 0
	mutualCount := 0
	indirectCount := 0
	
	for _, fn := range module.Functions {
		if fn.IsRecursive {
			recursiveCount++
			if info, exists := d.recursionInfo[fn.Name]; exists {
				switch info.Type {
				case RecursionDirect:
					directCount++
				case RecursionMutual:
					mutualCount++
				case RecursionIndirect:
					indirectCount++
				}
			}
		}
	}
	
	fmt.Println("\n=== RECURSION ANALYSIS SUMMARY ===")
	fmt.Printf("  Total functions: %d\n", len(module.Functions))
	fmt.Printf("  Recursive functions: %d\n", recursiveCount)
	if recursiveCount > 0 {
		fmt.Printf("    - Direct recursion: %d\n", directCount)
		fmt.Printf("    - Mutual recursion: %d\n", mutualCount)
		fmt.Printf("    - Indirect recursion: %d\n", indirectCount)
	}
	fmt.Println("================================")
}

// GetRecursionInfo returns detailed recursion information for a function
func (d *RecursionDetector) GetRecursionInfo(funcName string) *RecursionInfo {
	if info, exists := d.recursionInfo[funcName]; exists {
		return info
	}
	return &RecursionInfo{Type: RecursionNone}
}