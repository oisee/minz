package semantic

import (
	"fmt"
	
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// resolveIteratorFunction resolves function references in iterator operations,
// handling overloaded functions by matching the element type
func (a *Analyzer) resolveIteratorFunction(function ast.Expression, elementType ir.Type, 
	irFunc *ir.Function) (*FuncSymbol, error) {
	
	switch fn := function.(type) {
	case *ast.Identifier:
		baseName := fn.Name
		prefixedName := a.prefixSymbol(baseName)
		
		// First, look for the base name to see if it's an overload set
		sym := a.currentScope.Lookup(baseName)
		if sym == nil {
			sym = a.currentScope.Lookup(prefixedName)
		}
		
		// Check if it's an overload set
		if overloadSet, isOverloadSet := sym.(*FunctionOverloadSet); isOverloadSet {
			// We have an overload set - need to resolve based on element type
			
			// Look for exact match with type suffix
			// The overload keys might be prefixed already
			elementTypeStr := elementType.String()
			
			// Try different naming patterns
			candidates := []string{
				baseName + "$" + elementTypeStr,
				prefixedName + "$" + elementTypeStr,
				a.currentModule + "." + baseName + "$" + elementTypeStr,
			}
			
			for _, mangledName := range candidates {
				if funcSym, exists := overloadSet.Overloads[mangledName]; exists {
					return funcSym, nil
				}
			}
			
			// If no exact match, look for compatible overload
			for _, funcSym := range overloadSet.Overloads {
				if len(funcSym.Params) == 1 {
					// Convert ast.Type to ir.Type for comparison
					paramType, err := a.convertType(funcSym.Params[0].Type)
					if err == nil && a.typesCompatible(elementType, paramType) {
						return funcSym, nil
					}
				}
			}
			
			return nil, fmt.Errorf("no overload of %s accepts %s", baseName, elementType.String())
		}
		
		// Not an overload set - try direct lookup with type suffix
		typeSuffix := "$" + elementType.String()
		mangledName := baseName + typeSuffix
		sym = a.currentScope.Lookup(mangledName)
		
		if sym == nil {
			// Try prefixed version
			mangledName = prefixedName + typeSuffix
			sym = a.currentScope.Lookup(mangledName)
		}
		
		if sym == nil {
			// Try to find any matching function with compatible parameter
			return a.findCompatibleIteratorFunction(baseName, elementType)
		}
		
		funcSym, ok := sym.(*FuncSymbol)
		if !ok {
			return nil, fmt.Errorf("%s is not a function", mangledName)
		}
		
		return funcSym, nil
		
	default:
		return nil, fmt.Errorf("unsupported function type in iterator: %T", function)
	}
}

// findCompatibleIteratorFunction finds a function that can accept the element type
func (a *Analyzer) findCompatibleIteratorFunction(baseName string, elementType ir.Type) (*FuncSymbol, error) {
	// Look through all scopes for functions with matching base name
	var candidates []*FuncSymbol
	
	// Helper to check a symbol
	checkSymbol := func(name string, sym Symbol) {
		if funcSym, ok := sym.(*FuncSymbol); ok {
			// Check if it has exactly one parameter of compatible type
			if len(funcSym.Params) == 1 {
				// Convert ast.Type to ir.Type for comparison
				paramType, err := a.convertType(funcSym.Params[0].Type)
				if err == nil && a.typesCompatible(elementType, paramType) {
					candidates = append(candidates, funcSym)
				}
			}
		}
	}
	
	// Search current scope and parent scopes
	scope := a.currentScope
	for scope != nil {
		// Look for any function starting with baseName
		for name, sym := range scope.symbols {
			if name == baseName || 
			   (len(name) > len(baseName) && name[:len(baseName)] == baseName && name[len(baseName)] == '$') {
				checkSymbol(name, sym)
			}
		}
		scope = scope.parent
	}
	
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no function %s found that accepts %s", baseName, elementType.String())
	}
	
	if len(candidates) > 1 {
		// Ambiguous - need exact match
		for _, cand := range candidates {
			paramType, err := a.convertType(cand.Params[0].Type)
			if err == nil && paramType.String() == elementType.String() {
				return cand, nil
			}
		}
		return nil, fmt.Errorf("ambiguous function %s for type %s", baseName, elementType.String())
	}
	
	return candidates[0], nil
}

