package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// RecursionDetector analyzes functions to detect recursion
type RecursionDetector struct {
	callGraph     map[string][]string
	visited       map[string]bool
	recursionPath map[string]bool
}

// NewRecursionDetector creates a new recursion detector
func NewRecursionDetector() *RecursionDetector {
	return &RecursionDetector{
		callGraph:     make(map[string][]string),
		visited:       make(map[string]bool),
		recursionPath: make(map[string]bool),
	}
}

// AnalyzeModule detects recursive functions in a module
func (d *RecursionDetector) AnalyzeModule(module *ir.Module) {
	// Build call graph
	d.buildCallGraph(module)
	
	// Detect recursion for each function
	for _, fn := range module.Functions {
		if d.isRecursive(fn.Name) {
			fn.IsRecursive = true
		}
	}
}

// buildCallGraph builds a graph of function calls
func (d *RecursionDetector) buildCallGraph(module *ir.Module) {
	for _, fn := range module.Functions {
		d.callGraph[fn.Name] = []string{}
		
		// Find all function calls
		for _, inst := range fn.Instructions {
			if inst.Op == ir.OpCall && inst.Symbol != "" {
				d.callGraph[fn.Name] = append(d.callGraph[fn.Name], inst.Symbol)
			}
		}
	}
}

// isRecursive checks if a function is recursive
func (d *RecursionDetector) isRecursive(funcName string) bool {
	d.visited = make(map[string]bool)
	d.recursionPath = make(map[string]bool)
	
	return d.hasRecursivePath(funcName)
}

// hasRecursivePath performs DFS to detect cycles
func (d *RecursionDetector) hasRecursivePath(funcName string) bool {
	d.visited[funcName] = true
	d.recursionPath[funcName] = true
	
	// Check all functions called by this function
	for _, calledFunc := range d.callGraph[funcName] {
		// Direct recursion
		if calledFunc == funcName {
			return true
		}
		
		// Check if we're already in the recursion path (indirect recursion)
		if d.recursionPath[calledFunc] {
			return true
		}
		
		// Continue DFS if not visited
		if !d.visited[calledFunc] {
			if d.hasRecursivePath(calledFunc) {
				return true
			}
		}
	}
	
	// Remove from recursion path when backtracking
	d.recursionPath[funcName] = false
	return false
}