package semantic

import (
	"fmt"
	
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// resolveOverload finds the best matching function overload for the given arguments
func (a *Analyzer) resolveOverload(baseName string, args []ast.Expression, irFunc *ir.Function) (*FuncSymbol, error) {
	// Look up the overload set
	sym := a.currentScope.Lookup(baseName)
	if sym == nil {
		return nil, fmt.Errorf("undefined function: %s", baseName)
	}
	
	// Check if it's an overload set
	overloadSet, isOverloadSet := sym.(*FunctionOverloadSet)
	if !isOverloadSet {
		// Not an overload set - might be a single function
		if funcSym, ok := sym.(*FuncSymbol); ok {
			return funcSym, nil
		}
		return nil, fmt.Errorf("%s is not a function", baseName)
	}
	
	// If there's only one overload, and we have zero args and it takes zero params, use it
	// This handles cases like main() that have no parameters and no overloads
	if len(overloadSet.Overloads) == 1 && len(args) == 0 {
		for _, funcSym := range overloadSet.Overloads {
			if len(funcSym.Params) == 0 {
				return funcSym, nil
			}
		}
	}
	
	// Get argument types
	argTypes := make([]ir.Type, len(args))
	for i, arg := range args {
		_, err := a.analyzeExpression(arg, irFunc)
		if err != nil {
			return nil, fmt.Errorf("cannot analyze argument %d: %w", i, err)
		}
		typ := a.exprTypes[arg]
		if typ == nil {
			// Try to get type for simple expressions directly
			switch e := arg.(type) {
			case *ast.CastExpr:
				typ, err = a.convertType(e.TargetType)
				if err != nil {
					return nil, fmt.Errorf("cannot convert cast type for argument %d: %w", i, err)
				}
				a.exprTypes[arg] = typ
			case *ast.BooleanLiteral:
				typ = &ir.BasicType{Kind: ir.TypeBool}
				a.exprTypes[arg] = typ
			case *ast.NumberLiteral:
				// Default to u8 for small values, u16 for larger
				if e.Value <= 255 {
					typ = &ir.BasicType{Kind: ir.TypeU8}
				} else {
					typ = &ir.BasicType{Kind: ir.TypeU16}
				}
				a.exprTypes[arg] = typ
			default:
				return nil, fmt.Errorf("cannot determine type of argument %d (type: %T)", i, arg)
			}
		}
		argTypes[i] = typ
	}
	
	// Generate mangled name for the call
	// mangledName := generateMangledNameFromTypes(baseName, argTypes)
	
	// Look for exact match
	// Generate mangled name for lookup
	callMangledName := generateMangledNameFromTypes(baseName, argTypes)
	if funcSym, exists := overloadSet.Overloads[callMangledName]; exists {
		return funcSym, nil
	}
	
	// Try to find a compatible overload
	var candidates []*FuncSymbol
	for _, funcSym := range overloadSet.Overloads {
		if a.isCompatibleOverload(funcSym, argTypes) {
			candidates = append(candidates, funcSym)
		}
	}
	
	if len(candidates) == 0 {
		// Generate helpful error message
		availableOverloads := ""
		for _, funcSym := range overloadSet.Overloads {
			if availableOverloads != "" {
				availableOverloads += "\n  "
			}
			availableOverloads += fmt.Sprintf("%s(%s)", baseName, a.formatParamTypes(funcSym.Params))
		}
		
		return nil, fmt.Errorf("no matching overload for %s(%s)\nAvailable overloads:\n  %s", 
			baseName, a.formatArgTypes(argTypes), availableOverloads)
	}
	
	if len(candidates) > 1 {
		// Ambiguous - multiple candidates
		candidateList := ""
		for _, funcSym := range candidates {
			if candidateList != "" {
				candidateList += "\n  "
			}
			candidateList += fmt.Sprintf("%s(%s)", baseName, a.formatParamTypes(funcSym.Params))
		}
		
		return nil, fmt.Errorf("ambiguous call to %s(%s)\nCandidates:\n  %s",
			baseName, a.formatArgTypes(argTypes), candidateList)
	}
	
	return candidates[0], nil
}

// isCompatibleOverload checks if a function can be called with the given argument types
func (a *Analyzer) isCompatibleOverload(funcSym *FuncSymbol, argTypes []ir.Type) bool {
	if len(funcSym.Params) != len(argTypes) {
		return false
	}
	
	for i, param := range funcSym.Params {
		paramType, err := a.convertType(param.Type)
		if err != nil {
			return false
		}
		
		if !a.typesCompatible(paramType, argTypes[i]) {
			return false
		}
	}
	
	return true
}

// formatParamTypes formats parameter types for error messages
func (a *Analyzer) formatParamTypes(params []*ast.Parameter) string {
	if len(params) == 0 {
		return ""
	}
	
	result := ""
	for i, param := range params {
		if i > 0 {
			result += ", "
		}
		result += param.Name + ": " + a.formatASTType(param.Type)
	}
	return result
}

// formatArgTypes formats argument types for error messages
func (a *Analyzer) formatArgTypes(types []ir.Type) string {
	if len(types) == 0 {
		return ""
	}
	
	result := ""
	for i, typ := range types {
		if i > 0 {
			result += ", "
		}
		result += typ.String()
	}
	return result
}

// formatASTType formats an AST type for error messages
func (a *Analyzer) formatASTType(t ast.Type) string {
	switch typ := t.(type) {
	case *ast.PrimitiveType:
		return typ.Name
	case *ast.PointerType:
		return "*" + a.formatASTType(typ.BaseType)
	case *ast.ArrayType:
		if typ.Size != nil {
			if lit, ok := typ.Size.(*ast.NumberLiteral); ok {
				return fmt.Sprintf("[%d]%s", int(lit.Value), a.formatASTType(typ.ElementType))
			}
		}
		return "[]" + a.formatASTType(typ.ElementType)
	default:
		// For user-defined types that might come through as other nodes
		return "unknown"
	}
}