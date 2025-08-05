package semantic

import (
	"fmt"
	"strings"
	
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// generateMangledName generates a mangled name for a function based on its parameters
// Format: name$type1$type2$... (e.g., print$u8, print$p_str, max$u16$u16)
// This allows function overloading in MinZ
func generateMangledName(name string, params []*ast.Parameter) string {
	if len(params) == 0 {
		return name
	}
	
	parts := []string{name}
	for _, param := range params {
		if param.IsSelf && param.Type == nil {
			// Self parameter without explicit type - this shouldn't happen
			// after impl block processing sets the type
			parts = append(parts, "self")
		} else {
			parts = append(parts, mangleType(param.Type))
		}
	}
	
	return strings.Join(parts, "$")
}

// generateMangledNameFromTypes generates a mangled name from IR types
func generateMangledNameFromTypes(name string, paramTypes []ir.Type) string {
	if len(paramTypes) == 0 {
		return name
	}
	
	parts := []string{name}
	for _, paramType := range paramTypes {
		parts = append(parts, mangleIRType(paramType))
	}
	
	return strings.Join(parts, "$")
}

// mangleType converts an AST type to its mangled representation
func mangleType(t ast.Type) string {
	if t == nil {
		return "unknown"
	}
	switch typ := t.(type) {
	case *ast.PrimitiveType:
		// Basic types: u8, u16, i8, i16, bool, etc.
		return typ.Name
		
	case *ast.PointerType:
		// Pointer types: *T becomes p_T
		return "p_" + mangleType(typ.BaseType)
		
	case *ast.ArrayType:
		// Array types: [N]T becomes aN_T
		if typ.Size != nil {
			if lit, ok := typ.Size.(*ast.NumberLiteral); ok {
				return fmt.Sprintf("a%d_%s", int(lit.Value), mangleType(typ.ElementType))
			}
		}
		// Dynamic array or unknown size
		return "a_" + mangleType(typ.ElementType)
		
	case *ast.TypeIdentifier:
		// User-defined types (struct, enum, etc.)
		// Replace dots with underscores for module-qualified names
		return strings.ReplaceAll(typ.Name, ".", "_")
		
	default:
		// Fallback for unknown types
		return "unknown"
	}
}

// mangleIRType converts an IR type to its mangled representation
func mangleIRType(t ir.Type) string {
	switch typ := t.(type) {
	case *ir.BasicType:
		switch typ.Kind {
		case ir.TypeU8:
			return "u8"
		case ir.TypeU16:
			return "u16"
		case ir.TypeU24:
			return "u24"
		case ir.TypeI8:
			return "i8"
		case ir.TypeI16:
			return "i16"
		case ir.TypeI24:
			return "i24"
		case ir.TypeBool:
			return "bool"
		case ir.TypeVoid:
			return "void"
		default:
			return "unknown"
		}
		
	case *ir.PointerType:
		return "p_" + mangleIRType(typ.Base)
		
	case *ir.ArrayType:
		return fmt.Sprintf("a%d_%s", typ.Length, mangleIRType(typ.Element))
		
	case *ir.StructType:
		// Use struct name if available
		if typ.Name != "" {
			return strings.ReplaceAll(typ.Name, ".", "_")
		}
		return "struct"
		
	case *ir.StringType:
		return "String"
		
	case *ir.LStringType:
		return "LString"
		
	case *ir.EnumType:
		if typ.Name != "" {
			return strings.ReplaceAll(typ.Name, ".", "_")
		}
		return "enum"
		
	case *ir.FunctionType:
		parts := []string{"f"}
		for _, param := range typ.Params {
			parts = append(parts, mangleIRType(param))
		}
		parts = append(parts, "r", mangleIRType(typ.Return))
		return strings.Join(parts, "_")
		
		
	default:
		return "unknown"
	}
}

// demangleName extracts the original function name from a mangled name
func demangleName(mangledName string) string {
	parts := strings.Split(mangledName, "$")
	if len(parts) > 0 {
		return parts[0]
	}
	return mangledName
}