package semantic

import (
	"fmt"
	"os"
	
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

var debugOverload = os.Getenv("DEBUG") != ""

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
		// Check if the type is already available
		typ := a.exprTypes[arg]
		if typ == nil {
			// Analyze the expression if type not already known
			_, err := a.analyzeExpression(arg, irFunc)
			if err != nil {
				return nil, fmt.Errorf("cannot analyze argument %d: %w", i, err)
			}
			typ = a.exprTypes[arg]
		}
		if typ == nil {
			// Try to get type for simple expressions directly
			switch e := arg.(type) {
			case *ast.CastExpr:
				var err error
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
			case *ast.UnaryExpr:
				// Handle & operator (address-of)
				if e.Operator == "&" {
					// Get the type of the operand
					operandType := a.exprTypes[e.Operand]
					if operandType == nil {
						// Try to analyze the operand to get its type
						if _, err := a.analyzeExpression(e.Operand, irFunc); err == nil {
							operandType = a.exprTypes[e.Operand]
						}
					}
					if operandType != nil {
						// Create pointer type
						typ = &ir.PointerType{Base: operandType}
						a.exprTypes[arg] = typ
					} else {
						// Default to *u8 for now
						typ = &ir.PointerType{Base: &ir.BasicType{Kind: ir.TypeU8}}
						a.exprTypes[arg] = typ
					}
				} else {
					// For other unary operators, try to get the type from the expression
					typ = a.exprTypes[arg]
					if typ == nil {
						return nil, fmt.Errorf("cannot determine type of unary expression with operator %s", e.Operator)
					}
				}
			case *ast.StringLiteral:
				// String literals are *u8 (pointer to u8)
				typ = &ir.PointerType{Base: &ir.BasicType{Kind: ir.TypeU8}}
				a.exprTypes[arg] = typ
			case *ast.FieldExpr:
				// Handle field expressions by analyzing them first
				// This will convert enum field access to the proper type
				if _, err := a.analyzeExpression(e, irFunc); err == nil {
					typ = a.exprTypes[arg]
				}
				if typ == nil {
					// If analysis failed, try to handle enum field access directly
					if id, ok := e.Object.(*ast.Identifier); ok {
						sym := a.currentScope.Lookup(id.Name)
						if typeSym, isType := sym.(*TypeSymbol); isType {
							if _, isEnum := typeSym.Type.(*ir.EnumType); isEnum {
								// This is an enum variant - use the actual enum type
								typ = typeSym.Type
								a.exprTypes[arg] = typ
								if debugOverload {
									fmt.Printf("DEBUG: Set enum field type to %s (%T)\n", typ.String(), typ)
								}
							}
						}
					}
				}
				if typ == nil {
					if debugOverload {
						fmt.Printf("DEBUG: Failed to determine type for field expression\n")
						fmt.Printf("  Object: %T\n", e.Object)
						fmt.Printf("  Field: %s\n", e.Field)
					}
					return nil, fmt.Errorf("cannot determine type of field expression %s.%s", 
						e.Object, e.Field)
				}
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
			paramStr := a.formatParamTypesFromIR(funcSym.Params, funcSym.ParamTypes)
			availableOverloads += fmt.Sprintf("%s(%s)", baseName, paramStr)
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
			candidateList += fmt.Sprintf("%s(%s)", baseName, a.formatParamTypesFromIR(funcSym.Params, funcSym.ParamTypes))
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

// formatParamTypesFromIR formats parameter types from IR types for error messages
func (a *Analyzer) formatParamTypesFromIR(params []*ast.Parameter, paramTypes []ir.Type) string {
	if len(params) == 0 {
		return ""
	}
	
	result := ""
	for i, param := range params {
		if i > 0 {
			result += ", "
		}
		typeName := "unknown"
		if i < len(paramTypes) && paramTypes[i] != nil {
			typeName = paramTypes[i].String()
		}
		result += param.Name + ": " + typeName
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
	case *ast.TypeIdentifier:
		// User-defined types (struct, enum, etc.)
		return typ.Name
	default:
		// For unknown AST node types
		return "unknown"
	}
}