package semantic

import (
	"fmt"
	"github.com/minz/minzc/pkg/ast"
)

// SimpleCastInterface represents a basic cast interface for v0.11.0
type SimpleCastInterface struct {
	Name        string
	TargetType  string  // Simplified: just type name
	CastRules   map[string]bool  // Simplified: just track which types can cast
	Methods     []string         // Simplified: just method names
}

// analyzeCastInterface analyzes a cast interface declaration (simplified)
func (a *Analyzer) analyzeSimpleCastInterface(node *ast.InterfaceDecl) {
	if len(node.CastBlocks) == 0 {
		// No cast blocks, treat as regular interface
		return
	}

	// Create simplified cast interface
	castInterface := &SimpleCastInterface{
		Name:      node.Name,
		CastRules: make(map[string]bool),
		Methods:   make([]string, 0),
	}

	// Analyze cast blocks
	for _, castBlock := range node.CastBlocks {
		castInterface.TargetType = castBlock.TargetType

		// Analyze cast rules
		for _, rule := range castBlock.CastRules {
			castInterface.CastRules[rule.FromType] = true
		}
	}

	// Analyze methods
	for _, method := range node.Methods {
		castInterface.Methods = append(castInterface.Methods, method.Name)
	}

	// Register the cast interface
	a.registerSimpleCastInterface(castInterface)

	if debug {
		fmt.Printf("DEBUG: Registered cast interface %s with %d cast rules and %d methods\n",
			castInterface.Name, len(castInterface.CastRules), len(castInterface.Methods))
	}
}

// registerSimpleCastInterface registers a cast interface
func (a *Analyzer) registerSimpleCastInterface(castInterface *SimpleCastInterface) {
	if a.simpleCastInterfaces == nil {
		a.simpleCastInterfaces = make(map[string]*SimpleCastInterface)
	}
	
	a.simpleCastInterfaces[castInterface.Name] = castInterface
}

// checkSimpleCastConformance checks if a type can cast to the interface
func (a *Analyzer) checkSimpleCastConformance(typeName string, interfaceName string) bool {
	if a.simpleCastInterfaces == nil {
		return false
	}

	castInterface, exists := a.simpleCastInterfaces[interfaceName]
	if !exists {
		return false
	}

	// Check if the type has a cast rule
	return castInterface.CastRules[typeName] || castInterface.CastRules["auto"]
}

// generateSimpleCastDispatch generates the method name for compile-time dispatch
func (a *Analyzer) generateSimpleCastDispatch(interfaceName, methodName, typeName string) string {
	// Generate: InterfaceName_MethodName_TypeName
	return fmt.Sprintf("%s_%s_%s", interfaceName, methodName, typeName)
}

// Add to Analyzer struct field (this would need to be added to analyzer.go):
// simpleCastInterfaces map[string]*SimpleCastInterface