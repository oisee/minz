package semantic

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/meta"
)

// ModuleResolver is an interface for resolving module imports
type ModuleResolver interface {
	ResolveModule(path string) (*ModuleInfo, error)
}

// ModuleInfo contains information about an imported module
type ModuleInfo struct {
	Name    string
	Exports map[string]Symbol
}

// Analyzer performs semantic analysis on the AST
type Analyzer struct {
	currentScope          *Scope
	errors                []error
	module                *ir.Module
	moduleResolver        ModuleResolver
	currentFile           string
	currentModule         string // Current module name for prefixing
	luaEvaluator          *meta.LuaEvaluator
	currentFunc           *ir.Function
	functionCalls         map[string][]string // Track which functions call which
	exprTypes             map[ast.Expression]ir.Type // Type information for expressions
}

// NewAnalyzer creates a new semantic analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		currentScope:  NewScope(nil),
		errors:        []error{},
		module:        ir.NewModule("main"),
		functionCalls: make(map[string][]string),
		exprTypes:     make(map[ast.Expression]ir.Type),
		// luaEvaluator: meta.NewLuaEvaluator(), // Temporarily disabled
	}
}

// Analyze performs semantic analysis on a file
func (a *Analyzer) Analyze(file *ast.File) (*ir.Module, error) {
	// Set current module name
	if file.ModuleName != "" {
		a.currentModule = file.ModuleName
	} else {
		// Use filename without extension as module name
		a.currentModule = strings.TrimSuffix(filepath.Base(file.Name), filepath.Ext(file.Name))
	}
	
	// Add built-in types and functions
	a.addBuiltins()

	// Process imports
	for _, imp := range file.Imports {
		if err := a.processImport(imp); err != nil {
			a.errors = append(a.errors, err)
		}
	}

	// First pass: Register all type and function signatures, and constants
	for _, decl := range file.Declarations {
		switch d := decl.(type) {
		case *ast.FunctionDecl:
			if err := a.registerFunctionSignature(d); err != nil {
				a.errors = append(a.errors, err)
			}
		case *ast.StructDecl:
			// Register struct types early so they can be used in function signatures
			if err := a.analyzeStructDecl(d); err != nil {
				a.errors = append(a.errors, err)
			}
		case *ast.EnumDecl:
			// Register enum types early as well
			if err := a.analyzeEnumDecl(d); err != nil {
				a.errors = append(a.errors, err)
			}
		case *ast.TypeDecl:
			// Register type aliases (including bit structs)
			if err := a.analyzeTypeDecl(d); err != nil {
				a.errors = append(a.errors, err)
			}
		}
	}

	// Second pass: Process all declarations
	for _, decl := range file.Declarations {
		if err := a.analyzeDeclaration(decl); err != nil {
			a.errors = append(a.errors, err)
		}
	}

	if len(a.errors) > 0 {
		// Build detailed error message
		var errMsg string
		for i, err := range a.errors {
			if i > 0 {
				errMsg += "\n"
			}
			errMsg += fmt.Sprintf("  %d. %v", i+1, err)
		}
		return nil, fmt.Errorf("semantic analysis failed with %d errors:\n%s", len(a.errors), errMsg)
	}

	return a.module, nil
}

// prefixSymbol adds module prefix to a symbol name if needed
func (a *Analyzer) prefixSymbol(name string) string {
	// Don't prefix built-in types
	if name == "u8" || name == "u16" || name == "i8" || name == "i16" || name == "bool" || name == "void" {
		return name
	}
	
	// Check if already prefixed with current module
	if a.currentModule != "" && strings.HasPrefix(name, a.currentModule+".") {
		return name
	}
	
	// Add module prefix
	if a.currentModule != "" && a.currentModule != "main" {
		return a.currentModule + "." + name
	}
	return name
}

// addBuiltins adds built-in types and functions
func (a *Analyzer) addBuiltins() {
	// Built-in types
	a.currentScope.Define("u8", &TypeSymbol{Type: &ir.BasicType{Kind: ir.TypeU8}})
	a.currentScope.Define("u16", &TypeSymbol{Type: &ir.BasicType{Kind: ir.TypeU16}})
	a.currentScope.Define("i8", &TypeSymbol{Type: &ir.BasicType{Kind: ir.TypeI8}})
	a.currentScope.Define("i16", &TypeSymbol{Type: &ir.BasicType{Kind: ir.TypeI16}})
	a.currentScope.Define("bool", &TypeSymbol{Type: &ir.BasicType{Kind: ir.TypeBool}})
	a.currentScope.Define("void", &TypeSymbol{Type: &ir.BasicType{Kind: ir.TypeVoid}})
}

// processImport processes an import statement
func (a *Analyzer) processImport(imp *ast.ImportStmt) error {
	// For the simple prefix-based approach, we register known modules
	// In a real implementation, this would load and parse the module file
	
	moduleName := imp.Path
	
	// Handle known standard library modules
	if moduleName == "zx.screen" || moduleName == "screen" {
		// Add screen module functions with module prefix
		a.registerScreenModule()
	} else if moduleName == "zx.input" || moduleName == "input" {
		// Add input module functions
		a.registerInputModule()
	} else if moduleName == "zx.io" || moduleName == "io" {
		// Add I/O module functions
		a.registerIOModule()
	} else {
		// Unknown module
		return fmt.Errorf("unknown module: %s", moduleName)
	}
	
	// If there's an alias, we need to handle identifier resolution differently
	// For now, we'll just note the import
	
	return nil
}

// registerScreenModule registers screen module functions
func (a *Analyzer) registerScreenModule() {
	// Register screen as a module
	a.currentScope.Define("screen", &ModuleSymbol{
		Name: "screen",
	})
	
	// Register color constants
	a.currentScope.Define("screen.BLACK", &ConstSymbol{
		Name:  "screen.BLACK",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 0,
	})
	a.currentScope.Define("screen.BLUE", &ConstSymbol{
		Name:  "screen.BLUE",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 1,
	})
	a.currentScope.Define("screen.RED", &ConstSymbol{
		Name:  "screen.RED",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 2,
	})
	a.currentScope.Define("screen.MAGENTA", &ConstSymbol{
		Name:  "screen.MAGENTA",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 3,
	})
	a.currentScope.Define("screen.GREEN", &ConstSymbol{
		Name:  "screen.GREEN",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 4,
	})
	a.currentScope.Define("screen.CYAN", &ConstSymbol{
		Name:  "screen.CYAN",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 5,
	})
	a.currentScope.Define("screen.YELLOW", &ConstSymbol{
		Name:  "screen.YELLOW",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 6,
	})
	a.currentScope.Define("screen.WHITE", &ConstSymbol{
		Name:  "screen.WHITE",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 7,
	})
	a.currentScope.Define("screen.BRIGHT", &ConstSymbol{
		Name:  "screen.BRIGHT",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 0x40,
	})
	a.currentScope.Define("screen.FLASH", &ConstSymbol{
		Name:  "screen.FLASH",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 0x80,
	})
	
	// Register functions
	a.currentScope.Define("screen.set_pixel", &FuncSymbol{
		Name:       "screen.set_pixel",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "x", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "y", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "color", Type: &ast.PrimitiveType{Name: "u8"}},
		},
	})
	
	a.currentScope.Define("screen.clear_pixel", &FuncSymbol{
		Name:       "screen.clear_pixel",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "x", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "y", Type: &ast.PrimitiveType{Name: "u8"}},
		},
	})
	
	a.currentScope.Define("screen.attr_addr", &FuncSymbol{
		Name:       "screen.attr_addr",
		ReturnType: &ir.BasicType{Kind: ir.TypeU16},
		Params: []*ast.Parameter{
			{Name: "x", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "y", Type: &ast.PrimitiveType{Name: "u8"}},
		},
	})
	
	a.currentScope.Define("screen.set_attributes", &FuncSymbol{
		Name:       "screen.set_attributes",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "x", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "y", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "ink", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "paper", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "bright", Type: &ast.PrimitiveType{Name: "bool"}},
			{Name: "flash", Type: &ast.PrimitiveType{Name: "bool"}},
		},
	})
	
	a.currentScope.Define("screen.set_border", &FuncSymbol{
		Name:       "screen.set_border",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "color", Type: &ast.PrimitiveType{Name: "u8"}},
		},
	})
	
	a.currentScope.Define("screen.clear", &FuncSymbol{
		Name:       "screen.clear",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "ink", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "paper", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "bright", Type: &ast.PrimitiveType{Name: "bool"}},
			{Name: "flash", Type: &ast.PrimitiveType{Name: "bool"}},
		},
	})
	
	a.currentScope.Define("screen.draw_rect", &FuncSymbol{
		Name:       "screen.draw_rect",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "x", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "y", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "width", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "height", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "color", Type: &ast.PrimitiveType{Name: "u8"}},
			{Name: "fill", Type: &ast.PrimitiveType{Name: "bool"}},
		},
	})
}

// registerInputModule registers input module functions
func (a *Analyzer) registerInputModule() {
	// Register input as a module
	a.currentScope.Define("input", &ModuleSymbol{
		Name: "input",
	})
	
	// Register key constants
	a.currentScope.Define("input.KEY_Q", &ConstSymbol{
		Name:  "input.KEY_Q",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 10,  // From input.minz
	})
	a.currentScope.Define("input.KEY_A", &ConstSymbol{
		Name:  "input.KEY_A",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 5,   // From input.minz
	})
	a.currentScope.Define("input.KEY_O", &ConstSymbol{
		Name:  "input.KEY_O",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 26,  // From input.minz
	})
	a.currentScope.Define("input.KEY_P", &ConstSymbol{
		Name:  "input.KEY_P",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 25,  // From input.minz
	})
	a.currentScope.Define("input.KEY_SPACE", &ConstSymbol{
		Name:  "input.KEY_SPACE",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 30,  // From input.minz (KEY_SPC)
	})
	a.currentScope.Define("input.KEY_C", &ConstSymbol{
		Name:  "input.KEY_C",
		Type:  &ir.BasicType{Kind: ir.TypeU8},
		Value: 3,   // From input.minz
	})
	
	// Register input functions with module prefix
	a.currentScope.Define("input.read_key", &FuncSymbol{
		Name:       "input.read_key",
		ReturnType: &ir.BasicType{Kind: ir.TypeU8},
		Params:     []*ast.Parameter{}, // No parameters
	})
	
	a.currentScope.Define("input.key_pressed", &FuncSymbol{
		Name:       "input.key_pressed",
		ReturnType: &ir.BasicType{Kind: ir.TypeBool},
		Params: []*ast.Parameter{
			{Name: "key_code", Type: &ast.PrimitiveType{Name: "u8"}},
		},
	})
	
	a.currentScope.Define("input.is_key_pressed", &FuncSymbol{
		Name:       "input.is_key_pressed",
		ReturnType: &ir.BasicType{Kind: ir.TypeBool},
		Params: []*ast.Parameter{
			{Name: "key_code", Type: &ast.PrimitiveType{Name: "u8"}},
		},
	})
}

// registerIOModule registers I/O module functions
func (a *Analyzer) registerIOModule() {
	// Register io as a module
	a.currentScope.Define("io", &ModuleSymbol{
		Name: "io",
	})
	
	// print_char - prints a single character using RST 16
	a.currentScope.Define("io.print_char", &FuncSymbol{
		Name:       "io.print_char",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "ch", Type: &ast.PrimitiveType{Name: "u8"}},
		},
	})
	
	// print_u8 - prints an unsigned 8-bit number
	a.currentScope.Define("io.print_u8", &FuncSymbol{
		Name:       "io.print_u8",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "value", Type: &ast.PrimitiveType{Name: "u8"}},
		},
	})
	
	// print_u16 - prints an unsigned 16-bit number
	a.currentScope.Define("io.print_u16", &FuncSymbol{
		Name:       "io.print_u16",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "value", Type: &ast.PrimitiveType{Name: "u16"}},
		},
	})
	
	// print_str - prints a string
	a.currentScope.Define("io.print_str", &FuncSymbol{
		Name:       "io.print_str",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "str", Type: &ast.PointerType{BaseType: &ast.PrimitiveType{Name: "u8"}}},
		},
	})
	
	// print - generic print function (overloaded based on type)
	a.currentScope.Define("io.print", &FuncSymbol{
		Name:       "io.print",
		ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
		Params: []*ast.Parameter{
			{Name: "value", Type: &ast.PrimitiveType{Name: "u8"}}, // Will be type-checked based on actual argument
		},
	})
}

// analyzeDeclaration analyzes a declaration
func (a *Analyzer) analyzeDeclaration(decl ast.Declaration) error {
	switch d := decl.(type) {
	case *ast.FunctionDecl:
		return a.analyzeFunctionDecl(d)
	case *ast.VarDecl:
		return a.analyzeVarDecl(d)
	case *ast.ConstDecl:
		return a.analyzeConstDecl(d)
	case *ast.StructDecl:
		// Already processed in first pass
		return nil
	case *ast.EnumDecl:
		// Already processed in first pass
		return nil
	case *ast.TypeDecl:
		// Already processed in first pass
		return nil
	default:
		return fmt.Errorf("unsupported declaration type: %T", decl)
	}
}

// registerFunctionSignature registers a function's signature in the symbol table
// This is called in the first pass to allow forward references
func (a *Analyzer) registerFunctionSignature(fn *ast.FunctionDecl) error {
	// Convert return type
	returnType, err := a.convertType(fn.ReturnType)
	if err != nil {
		return fmt.Errorf("invalid return type for function %s: %w", fn.Name, err)
	}

	// Get prefixed name
	prefixedName := a.prefixSymbol(fn.Name)
	
	// Register function in global scope with prefixed name
	a.currentScope.Define(prefixedName, &FuncSymbol{
		Name:       prefixedName,
		ReturnType: returnType,
		Params:     fn.Params,
	})
	
	// Also register without prefix for local access
	if a.currentModule != "" && a.currentModule != "main" {
		a.currentScope.Define(fn.Name, &FuncSymbol{
			Name:       prefixedName,
			ReturnType: returnType,
			Params:     fn.Params,
		})
	}

	return nil
}

// analyzeFunctionDecl analyzes a function declaration
func (a *Analyzer) analyzeFunctionDecl(fn *ast.FunctionDecl) error {
	// Function signature already registered in first pass
	// Get the registered symbol
	sym := a.currentScope.Lookup(fn.Name)
	if sym == nil {
		return fmt.Errorf("function %s not found in symbol table", fn.Name)
	}
	funcSym, ok := sym.(*FuncSymbol)
	if !ok {
		return fmt.Errorf("symbol %s is not a function", fn.Name)
	}

	// Get prefixed name
	prefixedName := a.prefixSymbol(fn.Name)
	
	// Create IR function with prefixed name
	irFunc := ir.NewFunction(prefixedName, funcSym.ReturnType)
	
	// Process @abi attributes
	if err := a.processAbiAttributes(fn, irFunc); err != nil {
		return fmt.Errorf("error processing @abi attributes for %s: %v", fn.Name, err)
	}
	
	// Default to SMC unless overridden by attributes
	if irFunc.CallingConvention == "" {
		irFunc.IsSMCDefault = true
		irFunc.SMCParamOffsets = make(map[string]int)
	}
	
	// Debug: confirm SMC is enabled
	// fmt.Printf("DEBUG: After setting SMC for %s: IsSMCDefault=%v, IsSMCEnabled=%v, ptr=%p\n", fn.Name, irFunc.IsSMCDefault, irFunc.IsSMCEnabled, irFunc)
	
	// Allocate SMC parameter slots
	offset := 1 // Start after opcode
	for _, param := range fn.Params {
		// Get parameter type
		paramType, err := a.convertType(param.Type)
		if err != nil {
			// Default to u16 if type conversion fails
			paramType = &ir.BasicType{Kind: ir.TypeU16}
		}
		
		irFunc.SMCParamOffsets[param.Name] = offset
		
		// Calculate next offset based on parameter size
		if paramType.Size() == 1 {
			offset += 2 // LD A, #xx (2 bytes)
		} else {
			offset += 3 // LD HL, #xxxx (3 bytes)
		}
	}
	
	// Set current function for tracking
	prevFunc := a.currentFunc
	a.currentFunc = irFunc
	defer func() { a.currentFunc = prevFunc }()

	// Enter new scope for function
	a.currentScope = NewScope(a.currentScope)
	defer func() { a.currentScope = a.currentScope.parent }()

	// Process parameters
	for _, param := range fn.Params {
		paramType, err := a.convertType(param.Type)
		if err != nil {
			return fmt.Errorf("invalid parameter type for %s: %w", param.Name, err)
		}

		reg := irFunc.AddParam(param.Name, paramType)
		a.currentScope.Define(param.Name, &VarSymbol{
			Name:        param.Name,
			Type:        paramType,
			Reg:         reg,
			IsParameter: true,
		})
	}

	// Analyze function body
	if err := a.analyzeBlock(fn.Body, irFunc); err != nil {
		return fmt.Errorf("error in function %s: %w", fn.Name, err)
	}

	// Add implicit return if needed
	if len(irFunc.Instructions) == 0 || irFunc.Instructions[len(irFunc.Instructions)-1].Op != ir.OpReturn {
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{Op: ir.OpReturn})
	}

	// Finalize SMC decision based on function properties
	if irFunc.IsRecursive && len(irFunc.Params) > 3 {
		// Recursive functions with many parameters should use stack
		// (too much overhead to save/restore many SMC parameters)
		irFunc.IsSMCDefault = false
		irFunc.IsSMCEnabled = false
	} else if len(irFunc.Locals) > 6 {
		// Functions with many locals should use stack
		irFunc.IsSMCDefault = false
		irFunc.IsSMCEnabled = false
	} else if !irFunc.IsRecursive && len(irFunc.Params) <= 3 && len(irFunc.Params) > 0 {
		// Simple non-recursive functions with few parameters use register passing
		// (but keep SMC for parameter-less functions)
		irFunc.IsSMCDefault = false
		irFunc.IsSMCEnabled = false
	}
	// Otherwise keep SMC (including for recursive functions with few parameters)

	// Debug output
	fmt.Printf("Function %s: IsRecursive=%v, Params=%d, SMC=%v\n", 
		fn.Name, irFunc.IsRecursive, len(irFunc.Params), irFunc.IsSMCEnabled)

	// Add function to module
	a.module.AddFunction(irFunc)
	
	// fmt.Printf("DEBUG: Before adding to module %s: IsSMCDefault=%v, IsSMCEnabled=%v, ptr=%p\n", fn.Name, irFunc.IsSMCDefault, irFunc.IsSMCEnabled, irFunc)

	return nil
}

// analyzeVarDecl analyzes a variable declaration
func (a *Analyzer) analyzeVarDecl(v *ast.VarDecl) error {
	// Determine type
	var varType ir.Type
	var inferredType ir.Type
	
	// Get the declared type if present
	if v.Type != nil {
		t, err := a.convertType(v.Type)
		if err != nil {
			return fmt.Errorf("invalid type for variable %s: %w", v.Name, err)
		}
		varType = t
	}
	
	// Get the inferred type from value if present
	if v.Value != nil {
		t, err := a.inferType(v.Value)
		if err != nil {
			// If we have an explicit type, use it even if inference fails
			if varType == nil {
				return fmt.Errorf("cannot infer type for variable %s: %w", v.Name, err)
			}
			// Otherwise, we'll use the explicit type
		} else {
			inferredType = t
		}
	}
	
	// Determine final type and check compatibility
	if varType != nil && inferredType != nil {
		// Both type annotation and initializer present - check compatibility
		if !a.typesCompatible(varType, inferredType) {
			return fmt.Errorf("type mismatch for variable %s: declared type %s but initializer has type %s", 
				v.Name, varType.String(), inferredType.String())
		}
		// Use the declared type
	} else if varType != nil {
		// Only type annotation, no initializer
	} else if inferredType != nil {
		// Only initializer, use inferred type
		varType = inferredType
	} else {
		return fmt.Errorf("variable %s must have either a type or an initial value", v.Name)
	}

	// Register variable
	a.currentScope.Define(v.Name, &VarSymbol{
		Name:      v.Name,
		Type:      varType,
		IsMutable: v.IsMutable,
	})

	// Add global variable to IR module
	// Create IR global variable
	global := ir.Global{
		Name: v.Name,
		Type: varType,
	}
	
	// If there's an initializer, evaluate it
	if v.Value != nil {
		// Try to evaluate the initializer as a constant
		if val, err := a.evaluateConstExpr(v.Value); err == nil {
			global.Init = val
		} else {
			// For non-constant initializers, store the AST expression
			global.Value = v.Value
		}
	}
	
	// Add to module globals
	a.module.Globals = append(a.module.Globals, global)

	return nil
}

// analyzeConstDecl analyzes a constant declaration
func (a *Analyzer) analyzeConstDecl(c *ast.ConstDecl) error {
	// Constants must have a value
	if c.Value == nil {
		return fmt.Errorf("constant %s must have a value", c.Name)
	}
	
	// Determine type
	var constType ir.Type
	var inferredType ir.Type
	
	// Get the declared type if present
	if c.Type != nil {
		t, err := a.convertType(c.Type)
		if err != nil {
			return fmt.Errorf("invalid type for constant %s: %w", c.Name, err)
		}
		constType = t
	}
	
	// Get the inferred type from value
	t, err := a.inferType(c.Value)
	if err != nil {
		if constType == nil {
			return fmt.Errorf("cannot infer type for constant %s: %w", c.Name, err)
		}
		// Use the explicit type
	} else {
		inferredType = t
	}
	
	// Determine final type and check compatibility
	if constType != nil && inferredType != nil {
		// Both type annotation and initializer present - check compatibility
		if !a.typesCompatible(constType, inferredType) {
			return fmt.Errorf("type mismatch for constant %s: declared type %s but initializer has type %s", 
				c.Name, constType.String(), inferredType.String())
		}
		// Use the declared type
	} else if constType != nil {
		// Only type annotation
	} else if inferredType != nil {
		// Only initializer, use inferred type
		constType = inferredType
	} else {
		return fmt.Errorf("cannot determine type for constant %s", c.Name)
	}
	
	// Get prefixed name
	prefixedName := a.prefixSymbol(c.Name)
	
	// Define constant in current scope (should be global scope)
	a.currentScope.Define(c.Name, &VarSymbol{
		Name:      c.Name,
		Type:      constType,
		IsMutable: false, // Constants are immutable
	})
	
	// Also define with prefix if needed
	if prefixedName != c.Name {
		a.currentScope.Define(prefixedName, &VarSymbol{
			Name:      c.Name, // Still use unprefixed name in the symbol
			Type:      constType,
			IsMutable: false,
		})
	}
	
	
	// Generate global constant definition
	// TODO: For now, don't generate globals for constants
	// globalVar := ir.Global{
	// 	Name:     prefixedName,
	// 	Type:     constType,
	// 	Value:    c.Value, // Store AST expression for later evaluation
	// 	Constant: true,
	// }
	
	// a.module.Globals = append(a.module.Globals, globalVar)
	
	return nil
}

// analyzeStructDecl analyzes a struct declaration
func (a *Analyzer) analyzeStructDecl(s *ast.StructDecl) error {
	// Create struct type
	fields := make(map[string]ir.Type)
	fieldOrder := []string{}
	
	for _, field := range s.Fields {
		fieldType, err := a.convertType(field.Type)
		if err != nil {
			return fmt.Errorf("invalid type for field %s in struct %s: %w", field.Name, s.Name, err)
		}
		
		if _, exists := fields[field.Name]; exists {
			return fmt.Errorf("duplicate field %s in struct %s", field.Name, s.Name)
		}
		
		fields[field.Name] = fieldType
		fieldOrder = append(fieldOrder, field.Name)
	}
	
	// Get prefixed name
	prefixedName := a.prefixSymbol(s.Name)
	
	// Create struct type with prefixed name
	structType := &ir.StructType{
		Name:       prefixedName,
		Fields:     fields,
		FieldOrder: fieldOrder,
	}
	
	// Register struct type with prefixed name
	a.currentScope.Define(prefixedName, &TypeSymbol{
		Name: prefixedName,
		Type: structType,
	})
	
	// Also register without prefix for local access
	if a.currentModule != "" && a.currentModule != "main" {
		a.currentScope.Define(s.Name, &TypeSymbol{
			Name: prefixedName,
			Type: structType,
		})
	}
	
	return nil
}

// analyzeEnumDecl analyzes an enum declaration
func (a *Analyzer) analyzeEnumDecl(e *ast.EnumDecl) error {
	// Create enum type
	enumType := &ir.EnumType{
		Name:     e.Name,
		Variants: make(map[string]int),
	}
	
	// Assign values to variants
	for i, variant := range e.Variants {
		if _, exists := enumType.Variants[variant]; exists {
			return fmt.Errorf("duplicate variant %s in enum %s", variant, e.Name)
		}
		enumType.Variants[variant] = i
	}
	
	// Register enum type
	a.currentScope.Define(e.Name, &TypeSymbol{
		Name: e.Name,
		Type: enumType,
	})
	
	return nil
}

// analyzeTypeDecl analyzes a type declaration (including bit structs)
func (a *Analyzer) analyzeTypeDecl(t *ast.TypeDecl) error {
	// Convert the underlying type
	underlyingType, err := a.convertType(t.Type)
	if err != nil {
		return fmt.Errorf("invalid type in type declaration %s: %w", t.Name, err)
	}
	
	// Register the type alias
	a.currentScope.Define(t.Name, &TypeSymbol{
		Name: t.Name,
		Type: underlyingType,
	})
	
	return nil
}

// analyzeBlock analyzes a block statement
func (a *Analyzer) analyzeBlock(block *ast.BlockStmt, irFunc *ir.Function) error {
	// Enter new scope
	prevScope := a.currentScope
	a.currentScope = NewScope(a.currentScope)
	defer func() { 
		a.currentScope = prevScope 
	}()

	// Process statements
	for _, stmt := range block.Statements {
		if err := a.analyzeStatement(stmt, irFunc); err != nil {
			return err
		}
	}

	return nil
}

// analyzeStatement analyzes a statement
func (a *Analyzer) analyzeStatement(stmt ast.Statement, irFunc *ir.Function) error {
	if stmt == nil {
		return fmt.Errorf("encountered nil statement - likely a parsing error")
	}
	
	switch s := stmt.(type) {
	case *ast.VarDecl:
		return a.analyzeVarDeclInFunc(s, irFunc)
	case *ast.ReturnStmt:
		return a.analyzeReturnStmt(s, irFunc)
	case *ast.IfStmt:
		return a.analyzeIfStmt(s, irFunc)
	case *ast.WhileStmt:
		return a.analyzeWhileStmt(s, irFunc)
	case *ast.BlockStmt:
		return a.analyzeBlock(s, irFunc)
	case *ast.AsmStmt:
		return a.analyzeAsmStmt(s, irFunc)
	case *ast.ExpressionStmt:
		// Analyze the expression but ignore the result
		_, err := a.analyzeExpression(s.Expression, irFunc)
		return err
	case *ast.AssignStmt:
		return a.analyzeAssignStmt(s, irFunc)
	case *ast.LoopStmt:
		return a.analyzeLoopStmt(s, irFunc)
	case *ast.DoTimesStmt:
		return a.analyzeDoTimesStmt(s, irFunc)
	case *ast.LoopAtStmt:
		return a.analyzeLoopAtStmt(s, irFunc)
	default:
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// analyzeVarDeclInFunc analyzes a variable declaration inside a function
func (a *Analyzer) analyzeVarDeclInFunc(v *ast.VarDecl, irFunc *ir.Function) error {
	
	// Determine type
	var varType ir.Type
	
	// Get the declared type if present
	if v.Type != nil {
		t, err := a.convertType(v.Type)
		if err != nil {
			return fmt.Errorf("invalid type for variable %s: %w", v.Name, err)
		}
		varType = t
	}
	
	// If no explicit type, we need to infer from value
	// But we must be careful about when we do type inference
	if varType == nil && v.Value != nil {
		// For simple literals, we can infer type safely
		switch v.Value.(type) {
		case *ast.NumberLiteral, *ast.BooleanLiteral, *ast.StringLiteral:
			t, err := a.inferType(v.Value)
			if err != nil {
				return fmt.Errorf("cannot infer type for variable %s: %w", v.Name, err)
			}
			varType = t
		default:
			// For complex expressions, we'll need to handle this after registration
			// Use a placeholder type for now
			varType = &ir.BasicType{Kind: ir.TypeU16} // Default type
		}
	}
	
	// Ensure we have a type
	if varType == nil {
		return fmt.Errorf("variable %s must have either a type or an initial value", v.Name)
	}
	

	// Allocate register for variable
	reg := irFunc.AddLocal(v.Name, varType)

	// CRITICAL FIX: Register variable in scope BEFORE analyzing value expression
	// This ensures the variable is available if referenced in complex expressions
	a.currentScope.Define(v.Name, &VarSymbol{
		Name:      v.Name,
		Type:      varType,
		Reg:       reg,
		IsMutable: v.IsMutable,
	})
	

	// Generate code for initial value if present
	if v.Value != nil {
		// BUGFIX: Check for corrupted AST where the value expression contains the variable name itself
		if id, ok := v.Value.(*ast.Identifier); ok && id.Name == v.Name {
			// Corrupted AST: the variable's value expression is the variable name itself
			// This is a parsing bug, so we'll skip the value analysis and use the declared type
			if varType == nil {
				return fmt.Errorf("variable %s has corrupted AST (self-reference) and no explicit type", v.Name)
			}
			// Use a default value for the declared type
			valueReg := irFunc.AllocReg()
			// Generate a default literal based on type
			switch basicType := varType.(type) {
			case *ir.BasicType:
				switch basicType.Kind {
				case ir.TypeU8, ir.TypeI8:
					irFunc.Emit(ir.OpLoadConst, valueReg, 0, 0)
				case ir.TypeU16, ir.TypeI16:
					irFunc.Emit(ir.OpLoadConst, valueReg, 0, 0)
				case ir.TypeBool:
					irFunc.Emit(ir.OpLoadConst, valueReg, 0, 0)
				default:
					irFunc.Emit(ir.OpLoadConst, valueReg, 0, 0)
				}
			default:
				irFunc.Emit(ir.OpLoadConst, valueReg, 0, 0)
			}
			irFunc.Emit(ir.OpStoreVar, reg, valueReg, 0)
		} else {
			valueReg, err := a.analyzeExpression(v.Value, irFunc)
			if err != nil {
				return err
			}
			
			// If we used a placeholder type, now we can check the actual type
			if v.Type == nil && varType.String() == "u16" {
				// We used a default type, let's check if we need to update it
				// based on the actual expression type
				if exprType, ok := a.exprTypes[v.Value]; ok && exprType != nil {
					// Verify type compatibility if we had an explicit type
					if v.Type != nil && !a.typesCompatible(varType, exprType) {
						return fmt.Errorf("type mismatch for variable %s: declared type %s but initializer has type %s", 
							v.Name, varType.String(), exprType.String())
					}
				}
			}
			
			// Store value in variable
			irFunc.Emit(ir.OpStoreVar, reg, valueReg, 0)
		}
	}

	return nil
}

// analyzeReturnStmt analyzes a return statement
func (a *Analyzer) analyzeReturnStmt(ret *ast.ReturnStmt, irFunc *ir.Function) error {
	if ret.Value != nil {
		reg, err := a.analyzeExpression(ret.Value, irFunc)
		if err != nil {
			return err
		}
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpReturn,
			Src1: reg,
		})
	} else {
		irFunc.Emit(ir.OpReturn, 0, 0, 0)
	}
	return nil
}

// analyzeIfStmt analyzes an if statement
func (a *Analyzer) analyzeIfStmt(ifStmt *ast.IfStmt, irFunc *ir.Function) error {
	// Generate code for condition
	condReg, err := a.analyzeExpression(ifStmt.Condition, irFunc)
	if err != nil {
		return err
	}

	// Generate labels
	elseLabel := a.generateLabel("else")
	endLabel := a.generateLabel("end_if")

	// Jump to else if condition is false
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:    ir.OpJumpIfNot,
		Src1:  condReg,
		Label: elseLabel,
	})

	// Generate then block
	if err := a.analyzeBlock(ifStmt.Then, irFunc); err != nil {
		return err
	}

	// Jump to end
	irFunc.EmitJump(endLabel)

	// Else label
	irFunc.EmitLabel(elseLabel)

	// Generate else block if present
	if ifStmt.Else != nil {
		if err := a.analyzeStatement(ifStmt.Else, irFunc); err != nil {
			return err
		}
	}

	// End label
	irFunc.EmitLabel(endLabel)

	return nil
}

// analyzeWhileStmt analyzes a while statement
func (a *Analyzer) analyzeWhileStmt(whileStmt *ast.WhileStmt, irFunc *ir.Function) error {
	// Generate labels
	loopLabel := a.generateLabel("loop")
	endLabel := a.generateLabel("end_loop")

	// Loop label
	irFunc.EmitLabel(loopLabel)

	// Generate code for condition
	condReg, err := a.analyzeExpression(whileStmt.Condition, irFunc)
	if err != nil {
		return err
	}

	// Jump to end if condition is false
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:    ir.OpJumpIfNot,
		Src1:  condReg,
		Label: endLabel,
	})

	// Generate body
	if err := a.analyzeBlock(whileStmt.Body, irFunc); err != nil {
		return err
	}

	// Jump back to loop
	irFunc.EmitJump(loopLabel)

	// End label
	irFunc.EmitLabel(endLabel)

	return nil
}

// analyzeAsmStmt analyzes an inline assembly statement
func (a *Analyzer) analyzeAsmStmt(asmStmt *ast.AsmStmt, irFunc *ir.Function) error {
	// Create IR instruction with OpAsm
	asmInst := ir.Instruction{
		Op:      ir.OpAsm,
		AsmCode: asmStmt.Code,
		AsmName: asmStmt.Name,
	}
	
	// If named, register in symbol table
	if asmStmt.Name != "" {
		// TODO: Add AsmSymbol type to symbols
		// For now, just track the name
	}
	
	// Add to function instructions
	irFunc.Instructions = append(irFunc.Instructions, asmInst)
	
	return nil
}

// analyzeAssignStmt analyzes an assignment statement
func (a *Analyzer) analyzeAssignStmt(stmt *ast.AssignStmt, irFunc *ir.Function) error {
	// Analyze the right-hand side first
	valueReg, err := a.analyzeExpression(stmt.Value, irFunc)
	if err != nil {
		return err
	}
	
	// Handle different types of assignment targets
	switch target := stmt.Target.(type) {
	case *ast.Identifier:
		// Simple variable assignment
		sym := a.currentScope.Lookup(target.Name)
		if sym == nil {
			// Try with module prefix
			prefixedName := a.prefixSymbol(target.Name)
			sym = a.currentScope.Lookup(prefixedName)
			if sym == nil {
				return fmt.Errorf("undefined variable: %s", target.Name)
			}
			target.Name = prefixedName
		}
		
		// Get the variable's register from the symbol
		varSym := sym.(*VarSymbol)
		
		// Generate store instruction
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:     ir.OpStoreVar,
			Dest:   varSym.Reg,
			Src1:   valueReg,
			Symbol: target.Name, // Keep for debugging
		})
		
	case *ast.IndexExpr:
		// Array element assignment
		// Analyze the array expression
		arrayReg, err := a.analyzeExpression(target.Array, irFunc)
		if err != nil {
			return err
		}
		
		// Analyze the index expression
		indexReg, err := a.analyzeExpression(target.Index, irFunc)
		if err != nil {
			return err
		}
		
		// Get the type of the array to validate and get element type
		arrayType, err := a.inferType(target.Array)
		if err != nil {
			return fmt.Errorf("cannot determine array type: %v", err)
		}
		
		// Validate that it's an array or pointer type
		var elementType ir.Type
		switch t := arrayType.(type) {
		case *ir.ArrayType:
			elementType = t.Element
		case *ir.PointerType:
			// For pointers, assume they point to u8 (byte arrays)
			elementType = &ir.BasicType{Kind: ir.TypeU8}
		default:
			return fmt.Errorf("cannot index non-array type %s", arrayType)
		}
		
		// Type check the value against the element type
		valueType, err := a.inferType(stmt.Value)
		if err != nil {
			return fmt.Errorf("cannot determine value type: %v", err)
		}
		
		if !a.typesCompatible(elementType, valueType) {
			return fmt.Errorf("type mismatch: array element is %s, value is %s", elementType, valueType)
		}
		
		// Generate IR using two instructions approach
		// First, calculate the address (array + index)
		tempReg := irFunc.AllocReg()
		
		// For byte arrays, index is already the offset
		// For larger elements, we'd need to multiply by element size
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpAdd,
			Dest: tempReg,
			Src1: arrayReg,
			Src2: indexReg,
			Type: &ir.PointerType{Base: elementType},
			Comment: "Calculate array element address",
		})
		
		// Store the value at the calculated address using pointer store
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpStorePtr,
			Src1: tempReg, // address in Src1
			Src2: valueReg, // value in Src2
			Type: elementType,
			Comment: fmt.Sprintf("Store to array[index] (%s)", elementType),
		})
		
	case *ast.FieldExpr:
		// Struct field assignment
		// Check if this is a buffer field (loop iterator in INTO mode)
		if id, ok := target.Object.(*ast.Identifier); ok {
			sym := a.currentScope.Lookup(id.Name)
			if varSym, ok := sym.(*VarSymbol); ok && varSym.BufferAddr != 0 {
				// This is a buffer field - use direct memory store
				structType, ok := varSym.Type.(*ir.StructType)
				if !ok {
					return fmt.Errorf("field access on non-struct iterator %s", id.Name)
				}
				
				// Find field offset
				offset := 0
				found := false
				for _, fname := range structType.FieldOrder {
					if fname == target.Field {
						found = true
						break
					}
					offset += structType.Fields[fname].Size()
				}
				
				if !found {
					return fmt.Errorf("struct %s has no field %s", structType.Name, target.Field)
				}
				
				// Generate direct memory store to buffer
				directAddr := varSym.BufferAddr + uint16(offset)
				
				irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
					Op:      ir.OpStoreDirect,
					Src1:    valueReg,
					Imm:     int64(directAddr),
					Type:    structType.Fields[target.Field],
					Comment: fmt.Sprintf("Store to %s.%s at buffer $%04X", id.Name, target.Field, directAddr),
				})
				
				return nil
			}
		}
		
		// Regular field assignment (struct.field = value)
		objReg, err := a.analyzeExpression(target.Object, irFunc)
		if err != nil {
			return fmt.Errorf("error analyzing field object: %v", err)
		}
		
		// Get the object type
		objType := a.exprTypes[target.Object]
		
		// Check if it's a bit struct
		if bitStructType, ok := objType.(*ir.BitStructType); ok {
			// Handle bit field assignment
			bitField, exists := bitStructType.Fields[target.Field]
			if !exists {
				return fmt.Errorf("bit struct has no field %s", target.Field)
			}
			
			// Generate bit field store instruction
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:      ir.OpStoreBitField,
				Src1:    objReg,
				Src2:    valueReg,
				Imm:     int64(bitField.BitOffset),
				Imm2:    int64(bitField.BitWidth),
				Type:    bitStructType.UnderlyingType,
				Comment: fmt.Sprintf("Store bit field %s (offset %d, width %d)", target.Field, bitField.BitOffset, bitField.BitWidth),
			})
			
			return nil
		}
		
		// Handle regular struct types
		var structType *ir.StructType
		
		// Handle both direct struct and pointer to struct
		switch t := objType.(type) {
		case *ir.StructType:
			structType = t
		case *ir.PointerType:
			if st, ok := t.Base.(*ir.StructType); ok {
				structType = st
			} else {
				return fmt.Errorf("field access on non-struct pointer")
			}
		default:
			return fmt.Errorf("field access on non-struct type: %T", objType)
		}
		
		// Find field offset
		offset := 0
		found := false
		for _, fname := range structType.FieldOrder {
			if fname == target.Field {
				found = true
				break
			}
			offset += structType.Fields[fname].Size()
		}
		
		if !found {
			return fmt.Errorf("struct %s has no field %s", structType.Name, target.Field)
		}
		
		// Generate store field instruction
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpStoreField,
			Src1:    objReg,     // struct pointer
			Src2:    valueReg,   // value to store
			Imm:     int64(offset),
			Type:    structType.Fields[target.Field],
			Comment: fmt.Sprintf("Store to field %s (offset %d)", target.Field, offset),
		})
		
		return nil
		
	default:
		return fmt.Errorf("invalid assignment target: %T", target)
	}
	
	return nil
}

// analyzeLoopStmt analyzes a loop statement
func (a *Analyzer) analyzeLoopStmt(loop *ast.LoopStmt, irFunc *ir.Function) error {
	// Analyze the table expression
	tableReg, err := a.analyzeExpression(loop.Table, irFunc)
	if err != nil {
		return fmt.Errorf("invalid table expression: %w", err)
	}
	
	// Get table type
	tableType := a.exprTypes[loop.Table]
	if tableType == nil {
		return fmt.Errorf("cannot determine type of table")
	}
	
	// Extract element type from array type
	var elementType ir.Type
	var elementSize int
	var tableSize int
	
	switch t := tableType.(type) {
	case *ir.ArrayType:
		elementType = t.Element
		elementSize = t.Element.Size()
		tableSize = t.Length
	case *ir.PointerType:
		// Pointer to array
		if arrType, ok := t.Base.(*ir.ArrayType); ok {
			elementType = arrType.Element
			elementSize = arrType.Element.Size()
			tableSize = arrType.Length
		} else {
			return fmt.Errorf("loop requires array type, got pointer to %s", t.Base.String())
		}
	default:
		return fmt.Errorf("loop requires array type, got %s", tableType.String())
	}
	
	// Generate loop labels
	startLabel := a.generateLabel("loop_start")
	endLabel := a.generateLabel("loop_end")
	
	// Allocate registers for loop control
	ptrReg := irFunc.AllocReg()      // Current element pointer
	endReg := irFunc.AllocReg()      // End pointer
	countReg := irFunc.AllocReg()    // Loop counter (for DJNZ)
	
	// Load table base address
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpLoadAddr,
		Dest:    ptrReg,
		Src1:    tableReg,
		Comment: "Load table base address",
	})
	
	// Calculate end address
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpLoadAddr,
		Dest:    endReg,
		Src1:    tableReg,
		Comment: "Load table base for end calculation",
	})
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpAddImm,
		Dest:    endReg,
		Src1:    endReg,
		Imm:     int64(tableSize * elementSize),
		Comment: fmt.Sprintf("Calculate table end (+ %d elements * %d bytes)", tableSize, elementSize),
	})
	
	// Load counter for DJNZ optimization
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpLoadImm,
		Dest:    countReg,
		Imm:     int64(tableSize),
		Comment: "Load loop counter",
	})
	
	// Create new scope for loop body
	loopScope := NewScope(a.currentScope)
	oldScope := a.currentScope
	a.currentScope = loopScope
	defer func() { a.currentScope = oldScope }()
	
	// Setup iterator variable based on mode
	if loop.Mode == ast.LoopInto {
		// INTO mode: Allocate static buffer
		bufferAddr := uint16(0xF000) // TODO: Make this configurable
		
		// Define iterator as a "virtual" variable pointing to buffer
		iteratorSym := &VarSymbol{
			Name:       loop.Iterator,
			Type:       elementType,
			Reg:        ir.Register(-1), // Special marker for static buffer
			IsMutable:  true,
			BufferAddr: bufferAddr,      // Store buffer address
		}
		a.currentScope.Define(loop.Iterator, iteratorSym)
		
		// Store buffer info in function metadata
		irFunc.SetMetadata("loop_buffer_addr", fmt.Sprintf("%d", bufferAddr))
		irFunc.SetMetadata("loop_element_size", fmt.Sprintf("%d", elementSize))
		
	} else {
		// REF TO mode: Iterator is a pointer
		iteratorSym := &VarSymbol{
			Name:      loop.Iterator,
			Type:      &ir.PointerType{Base: elementType},
			Reg:       ptrReg,
			IsMutable: false,
		}
		a.currentScope.Define(loop.Iterator, iteratorSym)
	}
	
	// Define index variable if present
	if loop.Index != "" {
		indexReg := irFunc.AllocReg()
		indexSym := &VarSymbol{
			Name:      loop.Index,
			Type:      &ir.BasicType{Kind: ir.TypeU8},
			Reg:       indexReg,
			IsMutable: false,
		}
		a.currentScope.Define(loop.Index, indexSym)
		
		// Initialize index to 0
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpLoadImm,
			Dest: indexReg,
			Imm:  0,
		})
	}
	
	// Loop start label
	irFunc.EmitLabel(startLabel)
	
	// Check if done (compare pointer with end)
	cmpReg := irFunc.AllocReg()
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpCmp,
		Dest:    cmpReg,
		Src1:    ptrReg,
		Src2:    endReg,
		Comment: "Check if reached end of table",
	})
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpJumpIf,
		Src1:    cmpReg,
		Label:   endLabel,
		Comment: "Exit if done",
	})
	
	// INTO mode: Copy element to buffer
	if loop.Mode == ast.LoopInto {
		bufferAddr := uint16(0xF000) // Same as above
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpCopyToBuffer,
			Src1:    ptrReg,
			Imm:     int64(bufferAddr),
			Imm2:    int64(elementSize),
			Comment: fmt.Sprintf("Copy element to buffer at $%04X", bufferAddr),
		})
	}
	
	// Analyze loop body
	if err := a.analyzeBlock(loop.Body, irFunc); err != nil {
		return fmt.Errorf("error in loop body: %w", err)
	}
	
	// INTO mode: Copy buffer back if modified
	if loop.Mode == ast.LoopInto {
		bufferAddr := uint16(0xF000) // Same as above
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpCopyFromBuffer,
			Dest:    ptrReg,
			Imm:     int64(bufferAddr),
			Imm2:    int64(elementSize),
			Comment: fmt.Sprintf("Copy buffer back to element at $%04X", bufferAddr),
		})
	}
	
	// Increment index if present
	if loop.Index != "" {
		indexSym := a.currentScope.Lookup(loop.Index).(*VarSymbol)
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpInc,
			Dest: indexSym.Reg,
			Src1: indexSym.Reg,
		})
	}
	
	// Advance pointer
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpAddImm,
		Dest:    ptrReg,
		Src1:    ptrReg,
		Imm:     int64(elementSize),
		Comment: fmt.Sprintf("Advance to next element (+%d bytes)", elementSize),
	})
	
	// Decrement counter and loop if not zero (DJNZ optimization)
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpDJNZ,
		Src1:    countReg,
		Label:   startLabel,
		Comment: "Decrement counter and loop if not zero",
	})
	
	// End label
	irFunc.EmitLabel(endLabel)
	
	return nil
}

// analyzeDoTimesStmt analyzes a "do N times" statement
func (a *Analyzer) analyzeDoTimesStmt(stmt *ast.DoTimesStmt, irFunc *ir.Function) error {
	// Analyze count expression
	countReg, err := a.analyzeExpression(stmt.Count, irFunc)
	if err != nil {
		return fmt.Errorf("invalid count expression: %w", err)
	}
	
	// Generate labels
	loopLabel := a.generateLabel("do_times")
	endLabel := a.generateLabel("do_end")
	
	// Allocate counter register
	counterReg := irFunc.AllocReg()
	
	// Initialize counter with count value
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpMove,
		Dest:    counterReg,
		Src1:    countReg,
		Comment: "Initialize loop counter",
	})
	
	// Check if count is zero (skip loop entirely)
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpJumpIfZero,
		Src1:    counterReg,
		Symbol:  endLabel,
		Comment: "Skip if count is zero",
	})
	
	// Loop start label
	irFunc.EmitLabel(loopLabel)
	
	// Analyze loop body
	if err := a.analyzeStatement(stmt.Body, irFunc); err != nil {
		return err
	}
	
	// Decrement counter and loop if not zero
	// This pattern will be optimized to DJNZ in code generation
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpDec,
		Dest:    counterReg,
		Src1:    counterReg,
		Comment: "Decrement counter",
	})
	
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpJumpIfNotZero,
		Src1:    counterReg,
		Symbol:  loopLabel,
		Comment: "Loop if counter not zero (DJNZ pattern)",
	})
	
	// End label
	irFunc.EmitLabel(endLabel)
	
	return nil
}

// analyzeLoopAtStmt analyzes a loop at array -> item statement
func (a *Analyzer) analyzeLoopAtStmt(stmt *ast.LoopAtStmt, irFunc *ir.Function) error {
	// Analyze the array/table expression
	arrayReg, err := a.analyzeExpression(stmt.Table, irFunc)
	if err != nil {
		return fmt.Errorf("invalid array expression: %w", err)
	}
	
	// Get array type information from the symbol table
	var arrayType ir.Type
	if ident, ok := stmt.Table.(*ast.Identifier); ok {
		// Look up the identifier in the symbol table
		sym := a.currentScope.Lookup(ident.Name)
		if sym == nil {
			// Try with module prefix
			prefixedName := a.prefixSymbol(ident.Name)
			sym = a.currentScope.Lookup(prefixedName)
			if sym == nil {
				return fmt.Errorf("undefined array variable: %s", ident.Name)
			}
		}
		
		if varSym, ok := sym.(*VarSymbol); ok {
			arrayType = varSym.Type
		} else {
			return fmt.Errorf("loop at requires a variable, got %T", sym)
		}
	} else {
		return fmt.Errorf("loop at currently only supports simple variable references")
	}
	
	// Verify it's an array type
	var elementType ir.Type
	var arraySize int
	switch t := arrayType.(type) {
	case *ir.ArrayType:
		elementType = t.Element
		arraySize = t.Length
	default:
		return fmt.Errorf("loop at requires an array type, got %T", arrayType)
	}
	
	// Generate labels
	loopLabel := a.generateLabel("loop_at")
	endLabel := a.generateLabel("loop_end")
	
	// Allocate registers for iteration
	indexReg := irFunc.AllocReg()     // Current index (counter)
	baseReg := irFunc.AllocReg()      // Array base address
	workAreaReg := irFunc.AllocReg()  // Work area for current element
	
	// Get array base address (for now, assume array is at arrayReg)
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpMove,
		Dest:    baseReg,
		Src1:    arrayReg,
		Comment: "Get array base address",
	})
	
	// Initialize index to array size (we'll decrement down to 0)
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpLoadImm,
		Dest:    indexReg,
		Imm:     int64(arraySize),
		Comment: "Initialize index to array size",
	})
	
	// Check if array is empty (skip loop entirely)
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpJumpIfZero,
		Src1:    indexReg,
		Symbol:  endLabel,
		Comment: "Skip if array is empty",
	})
	
	// Loop start label
	irFunc.EmitLabel(loopLabel)
	
	// Decrement index first (so we go from size-1 down to 0)
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpDec,
		Dest:    indexReg,
		Src1:    indexReg,
		Comment: "Decrement index",
	})
	
	// Calculate element address: base + (index * element_size)
	// For now, assume 1-byte elements, will be optimized later
	elementSize := a.getTypeSize(elementType)
	if elementSize > 1 {
		// Need to multiply index by element size
		offsetReg := irFunc.AllocReg()
		sizeReg := irFunc.AllocReg()
		
		// Load element size into register
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpLoadImm,
			Dest:    sizeReg,
			Imm:     int64(elementSize),
			Comment: "Load element size",
		})
		
		// Multiply index by element size
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpMul,
			Dest:    offsetReg,
			Src1:    indexReg,
			Src2:    sizeReg,
			Comment: "Calculate element offset",
		})
		
		// Add to base address
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpAdd,
			Dest:    workAreaReg,
			Src1:    baseReg,
			Src2:    offsetReg,
			Comment: "Calculate element address",
		})
	} else {
		// Simple case: 1-byte elements
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpAdd,
			Dest:    workAreaReg,
			Src1:    baseReg,
			Src2:    indexReg,
			Comment: "Calculate element address",
		})
	}
	
	// Load element into work area (for now, just use the address)
	// In a full implementation, this would copy the element data
	
	// Create a new scope for the loop body with the iterator variable
	a.currentScope = NewScope(a.currentScope)
	defer func() {
		a.currentScope = a.currentScope.parent
	}()
	
	// Add iterator variable to scope
	iteratorSym := &VarSymbol{
		Name:      stmt.Iterator,
		Type:      elementType,
		Reg:       workAreaReg,
		IsMutable: stmt.IsModifying, // Only mutable if ! modifier is used
	}
	a.currentScope.Define(stmt.Iterator, iteratorSym)
	
	// Analyze loop body
	if err := a.analyzeStatement(stmt.Body, irFunc); err != nil {
		return err
	}
	
	// If modifying (!), write back the work area to the array
	if stmt.IsModifying {
		// TODO: Implement write-back logic
		// For now, add a comment indicating where write-back would happen
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpNop,
			Comment: "TODO: Write back modified element",
		})
	}
	
	// Loop if index not zero (DJNZ pattern)
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpJumpIfNotZero,
		Src1:    indexReg,
		Symbol:  loopLabel,
		Comment: "Loop if more elements (DJNZ pattern)",
	})
	
	// End label
	irFunc.EmitLabel(endLabel)
	
	return nil
}

// getTypeSize returns the size in bytes of a type
func (a *Analyzer) getTypeSize(t ir.Type) int {
	switch typ := t.(type) {
	case *ir.BasicType:
		switch typ.Kind {
		case ir.TypeU8, ir.TypeI8, ir.TypeBool:
			return 1
		case ir.TypeU16, ir.TypeI16:
			return 2
		default:
			return 1 // Default to 1 byte
		}
	case *ir.ArrayType:
		return a.getTypeSize(typ.Element) * typ.Length
	case *ir.PointerType:
		return 2 // Pointers are 16-bit on Z80
	default:
		return 1 // Default fallback
	}
}

// analyzeExpression analyzes an expression and returns the register containing the result
func (a *Analyzer) analyzeExpression(expr ast.Expression, irFunc *ir.Function) (ir.Register, error) {
	// Debug: print expression type
	if fieldExpr, ok := expr.(*ast.FieldExpr); ok {
		if id, ok := fieldExpr.Object.(*ast.Identifier); ok && id.Name == "screen" {
			// This is a screen.X expression - should be handled by analyzeFieldExpr
		}
	}
	
	switch e := expr.(type) {
	case *ast.Identifier:
		return a.analyzeIdentifier(e, irFunc)
	case *ast.NumberLiteral:
		return a.analyzeNumberLiteral(e, irFunc)
	case *ast.BooleanLiteral:
		return a.analyzeBooleanLiteral(e, irFunc)
	case *ast.BinaryExpr:
		return a.analyzeBinaryExpr(e, irFunc)
	case *ast.UnaryExpr:
		return a.analyzeUnaryExpr(e, irFunc)
	case *ast.CallExpr:
		return a.analyzeCallExpr(e, irFunc)
	case *ast.StructLiteral:
		return a.analyzeStructLiteral(e, irFunc)
	case *ast.FieldExpr:
		return a.analyzeFieldExpr(e, irFunc)
	case *ast.IndexExpr:
		return a.analyzeIndexExpr(e, irFunc)
	case *ast.EnumLiteral:
		return a.analyzeEnumLiteral(e, irFunc)
	case *ast.LuaExpression:
		return a.analyzeLuaExpression(e, irFunc)
	case *ast.StringLiteral:
		return a.analyzeStringLiteral(e, irFunc)
	case *ast.InlineAsmExpr:
		// Inline assembly expressions
		return a.analyzeInlineAsmExpr(e, irFunc)
	case *ast.CastExpr:
		return a.analyzeCastExpr(e, irFunc)
	default:
		return 0, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// analyzeIdentifier analyzes an identifier
func (a *Analyzer) analyzeIdentifier(id *ast.Identifier, irFunc *ir.Function) (ir.Register, error) {
	
	// First try direct lookup
	sym := a.currentScope.Lookup(id.Name)
	
	// If not found, try with module prefix
	if sym == nil && a.currentModule != "" && a.currentModule != "main" {
		// Try with module prefix
		prefixedName := a.prefixSymbol(id.Name)
		sym = a.currentScope.Lookup(prefixedName)
	}
	
	// If not found and name contains a dot, it might be a module reference
	if sym == nil && strings.Contains(id.Name, ".") {
		// Try looking up the full dotted name
		sym = a.currentScope.Lookup(id.Name)
	}
	
	if sym == nil {
		
		// Add stack trace for debugging
		if id.Name == "screen" {
			return 0, fmt.Errorf("undefined identifier: %s (this should have been handled as a module)", id.Name)
		}
		return 0, fmt.Errorf("undefined identifier: %s (analyzeIdentifier)", id.Name)
	}

	switch s := sym.(type) {
	case *ModuleSymbol:
		// Module identifiers are not values - they're only used in field expressions
		return 0, fmt.Errorf("module %s cannot be used as a value", s.Name)
	case *VarSymbol:
		// Store the type for later use
		a.exprTypes[id] = s.Type
		
		// Load variable value
		reg := irFunc.AllocReg()
		
		// Check if this is a parameter in an SMC function
		if s.IsParameter && irFunc.IsSMCDefault {
			// Find the parameter index
			paramIndex := -1
			for i, param := range irFunc.Params {
				if param.Name == s.Name {
					paramIndex = i
					break
				}
			}
			
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:     ir.OpLoadParam,
				Dest:   reg,
				Src1:   ir.Register(paramIndex), // Store parameter index in Src1
				Symbol: s.Name, // Use the symbol's actual name (prefixed)
				Type:   s.Type,
			})
		} else {
			// Check if this is an array type
			if arrayType, isArray := s.Type.(*ir.ArrayType); isArray {
				// For arrays, we need to load the address, not the value
				// Check if this is a global array
				isGlobal := false
				for _, global := range a.module.Globals {
					if global.Name == s.Name {
						isGlobal = true
						break
					}
				}
				
				if isGlobal {
					// For global arrays, load the address directly
					irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
						Op:     ir.OpLoadAddr,
						Dest:   reg,
						Symbol: s.Name,
						Type:   arrayType,
					})
				} else {
					// For local arrays, use regular load
					irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
						Op:     ir.OpLoadVar,
						Dest:   reg,
						Symbol: s.Name,
					})
				}
			} else {
				// Regular variable load
				irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
					Op:     ir.OpLoadVar,
					Dest:   reg,
					Symbol: s.Name, // Use the symbol's actual name (prefixed)
				})
			}
		}
		return reg, nil
	case *ConstSymbol:
		// Store the type for later use
		a.exprTypes[id] = s.Type
		
		// Load constant value
		reg := irFunc.AllocReg()
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpLoadConst,
			Dest: reg,
			Imm:  s.Value,
			Type: s.Type,
		})
		return reg, nil
	default:
		return 0, fmt.Errorf("cannot use %s as value", id.Name)
	}
}

// analyzeNumberLiteral analyzes a number literal
func (a *Analyzer) analyzeNumberLiteral(num *ast.NumberLiteral, irFunc *ir.Function) (ir.Register, error) {
	reg := irFunc.AllocReg()
	
	// Infer type based on value
	var numType ir.Type
	if num.Value >= 0 && num.Value <= 255 {
		numType = &ir.BasicType{Kind: ir.TypeU8}
	} else if num.Value >= -128 && num.Value <= 127 {
		numType = &ir.BasicType{Kind: ir.TypeI8}
	} else if num.Value >= 0 && num.Value <= 65535 {
		numType = &ir.BasicType{Kind: ir.TypeU16}
	} else {
		numType = &ir.BasicType{Kind: ir.TypeI16}
	}
	
	// Store the type
	a.exprTypes[num] = numType
	
	// Emit typed instruction
	inst := ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: reg,
		Imm:  num.Value,
		Type: numType,
	}
	irFunc.Instructions = append(irFunc.Instructions, inst)
	
	return reg, nil
}

// analyzeBooleanLiteral analyzes a boolean literal
func (a *Analyzer) analyzeBooleanLiteral(b *ast.BooleanLiteral, irFunc *ir.Function) (ir.Register, error) {
	reg := irFunc.AllocReg()
	val := int64(0)
	if b.Value {
		val = 1
	}
	irFunc.EmitImm(ir.OpLoadConst, reg, val)
	return reg, nil
}

// analyzeBinaryExpr analyzes a binary expression
func (a *Analyzer) analyzeBinaryExpr(bin *ast.BinaryExpr, irFunc *ir.Function) (ir.Register, error) {
	// Analyze operands
	leftReg, err := a.analyzeExpression(bin.Left, irFunc)
	if err != nil {
		return 0, err
	}

	rightReg, err := a.analyzeExpression(bin.Right, irFunc)
	if err != nil {
		return 0, err
	}

	// Generate operation
	resultReg := irFunc.AllocReg()
	var op ir.Opcode

	switch bin.Operator {
	case "+":
		op = ir.OpAdd
	case "-":
		op = ir.OpSub
	case "*":
		op = ir.OpMul
	case "/":
		op = ir.OpDiv
	case "%":
		op = ir.OpMod
	case "==":
		op = ir.OpEq
	case "!=":
		op = ir.OpNe
	case "<":
		op = ir.OpLt
	case ">":
		op = ir.OpGt
	case "<=":
		op = ir.OpLe
	case ">=":
		op = ir.OpGe
	case "&":
		op = ir.OpAnd
	case "|":
		op = ir.OpOr
	case "^":
		op = ir.OpXor
	case "<<":
		op = ir.OpShl
	case ">>":
		op = ir.OpShr
	default:
		return 0, fmt.Errorf("unsupported binary operator: %s", bin.Operator)
	}

	// Determine result type
	leftType := a.exprTypes[bin.Left]
	rightType := a.exprTypes[bin.Right]
	
	// For now, use the left operand's type as the result type
	// TODO: Implement proper type promotion rules
	resultType := leftType
	if resultType == nil && rightType != nil {
		resultType = rightType
	}
	
	// Store the result type
	a.exprTypes[bin] = resultType
	
	// Emit typed instruction
	if resultType != nil {
		irFunc.EmitTyped(op, resultReg, leftReg, rightReg, resultType)
	} else {
		irFunc.Emit(op, resultReg, leftReg, rightReg)
	}
	
	return resultReg, nil
}

// analyzeUnaryExpr analyzes a unary expression
func (a *Analyzer) analyzeUnaryExpr(un *ast.UnaryExpr, irFunc *ir.Function) (ir.Register, error) {
	// Analyze operand
	operandReg, err := a.analyzeExpression(un.Operand, irFunc)
	if err != nil {
		return 0, err
	}

	// Generate operation
	resultReg := irFunc.AllocReg()

	switch un.Operator {
	case "-":
		irFunc.Emit(ir.OpNeg, resultReg, operandReg, 0)
	case "!":
		irFunc.Emit(ir.OpNot, resultReg, operandReg, 0)
	case "~":
		// Bitwise not
		irFunc.Emit(ir.OpNot, resultReg, operandReg, 0)
	case "&":
		// Address-of operator
		irFunc.Emit(ir.OpAddr, resultReg, operandReg, 0)
	default:
		return 0, fmt.Errorf("unsupported unary operator: %s", un.Operator)
	}

	return resultReg, nil
}

// analyzeCallExpr analyzes a function call
func (a *Analyzer) analyzeCallExpr(call *ast.CallExpr, irFunc *ir.Function) (ir.Register, error) {
	var funcName string
	var sym Symbol
	
	switch fn := call.Function.(type) {
	case *ast.Identifier:
		// Direct function call
		funcName = fn.Name
		sym = a.currentScope.Lookup(funcName)
		if sym == nil {
			return 0, fmt.Errorf("undefined function: %s", funcName)
		}
		
	case *ast.FieldExpr:
		// Module function call (e.g., screen.set_pixel)
		if id, ok := fn.Object.(*ast.Identifier); ok {
			funcName = id.Name + "." + fn.Field
			sym = a.currentScope.Lookup(funcName)
			if sym == nil {
				return 0, fmt.Errorf("undefined function: %s", funcName)
			}
		} else {
			return 0, fmt.Errorf("complex function calls not yet supported")
		}
		
	default:
		return 0, fmt.Errorf("indirect function calls not yet supported")
	}

	funcSym, ok := sym.(*FuncSymbol)
	if !ok {
		return 0, fmt.Errorf("%s is not a function", funcName)
	}
	
	// Track function calls for recursion detection
	if a.currentFunc != nil {
		a.functionCalls[a.currentFunc.Name] = append(a.functionCalls[a.currentFunc.Name], funcName)
		
		// Check if this is a recursive call
		// Need to check both simple name and prefixed name since function calls
		// might use simple names but currentFunc.Name is prefixed
		isRecursiveCall := false
		if funcName == a.currentFunc.Name {
			// Direct match (prefixed name)
			isRecursiveCall = true
		} else if a.currentModule != "" && a.currentModule != "main" {
			// Check if funcName is the simple name of current function
			expectedSimpleName := strings.TrimPrefix(a.currentFunc.Name, a.currentModule+".")
			if funcName == expectedSimpleName {
				isRecursiveCall = true
			}
		}
		
		if isRecursiveCall {
			a.currentFunc.IsRecursive = true
			a.currentFunc.RequiresContext = true
		}
	}

	// Check argument count
	if len(call.Arguments) != len(funcSym.Params) {
		return 0, fmt.Errorf("function %s expects %d arguments, got %d", 
			funcName, len(funcSym.Params), len(call.Arguments))
	}

	// Analyze arguments
	argRegs := []ir.Register{}
	for _, arg := range call.Arguments {
		reg, err := a.analyzeExpression(arg, irFunc)
		if err != nil {
			return 0, err
		}
		argRegs = append(argRegs, reg)
	}

	// Generate call
	resultReg := irFunc.AllocReg()
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:     ir.OpCall,
		Dest:   resultReg,
		Symbol: funcName,
		Args:   argRegs,  // Store argument registers for TRUE SMC patching
	})

	return resultReg, nil
}

// analyzeStructLiteral analyzes a struct literal expression
func (a *Analyzer) analyzeStructLiteral(lit *ast.StructLiteral, irFunc *ir.Function) (ir.Register, error) {
	// Look up the struct type
	sym := a.currentScope.Lookup(lit.TypeName)
	if sym == nil {
		return 0, fmt.Errorf("undefined type: %s", lit.TypeName)
	}
	
	typeSym, ok := sym.(*TypeSymbol)
	if !ok {
		return 0, fmt.Errorf("%s is not a type", lit.TypeName)
	}
	
	structType, ok := typeSym.Type.(*ir.StructType)
	if !ok {
		return 0, fmt.Errorf("%s is not a struct type", lit.TypeName)
	}
	
	// Allocate space for the struct
	resultReg := irFunc.AllocReg()
	
	// TODO: Generate IR for struct allocation
	// For now, just allocate on stack
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpAlloc,
		Dest: resultReg,
		Imm:  int64(structType.Size()),
		Type: structType,
		Comment: fmt.Sprintf("Allocate struct %s", lit.TypeName),
	})
	
	// Initialize fields
	for _, fieldInit := range lit.Fields {
		// Check field exists
		fieldType, exists := structType.Fields[fieldInit.Name]
		if !exists {
			return 0, fmt.Errorf("no field %s in struct %s", fieldInit.Name, lit.TypeName)
		}
		
		// Analyze field value
		valueReg, err := a.analyzeExpression(fieldInit.Value, irFunc)
		if err != nil {
			return 0, err
		}
		
		// Calculate field offset
		offset := 0
		for _, fname := range structType.FieldOrder {
			if fname == fieldInit.Name {
				break
			}
			offset += structType.Fields[fname].Size()
		}
		
		// Store to field
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpStoreField,
			Src1: resultReg,
			Src2: valueReg,
			Imm:  int64(offset),
			Type: fieldType,
			Comment: fmt.Sprintf("Store to %s.%s", lit.TypeName, fieldInit.Name),
		})
	}
	
	return resultReg, nil
}

// analyzeFieldExpr analyzes a field access expression
func (a *Analyzer) analyzeFieldExpr(field *ast.FieldExpr, irFunc *ir.Function) (ir.Register, error) {
	// Special handling for module field access (e.g., screen.set_pixel)
	// Check this FIRST before trying to analyze the object as an expression
	if id, ok := field.Object.(*ast.Identifier); ok {
		// Check if the identifier is a module
		sym := a.currentScope.Lookup(id.Name)
		if sym != nil {
			// Check if it's an enum type
			if typeSym, isType := sym.(*TypeSymbol); isType {
				// Check if the type is an enum
				if _, isEnum := typeSym.Type.(*ir.EnumType); isEnum {
					// Convert to enum literal
					enumLit := &ast.EnumLiteral{
						EnumName: id.Name,
						Variant:  field.Field,
						StartPos: field.StartPos,
						EndPos:   field.EndPos,
					}
					return a.analyzeEnumLiteral(enumLit, irFunc)
				}
			}
			
			if _, isModule := sym.(*ModuleSymbol); isModule {
				// This is a module - look up the full qualified name
				fullName := id.Name + "." + field.Field
				sym := a.currentScope.Lookup(fullName)
				if sym != nil {
					// Check if this is a constant or function
					if constSym, ok := sym.(*ConstSymbol); ok {
						// This is a module constant - load its value
						reg := irFunc.AllocReg()
						irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
							Op:      ir.OpLoadConst,
							Dest:    reg,
							Imm:     constSym.Value,
							Type:    constSym.Type,
							Comment: fmt.Sprintf("Load constant %s = %d", fullName, constSym.Value),
						})
						// Store type information for this expression
						a.exprTypes[field] = constSym.Type
						return reg, nil
					} else if _, ok := sym.(*FuncSymbol); ok {
						// This is a module function - treat it as a function identifier
						// Return a special register that indicates this is a function reference
						// The actual call will be handled by analyzeCallExpr
						reg := irFunc.AllocReg()
						irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
							Op:      ir.OpLoadLabel,
							Dest:    reg,
							Symbol:  fullName,
							Comment: fmt.Sprintf("Load function %s", fullName),
						})
						return reg, nil
					}
				}
				// If we get here, the module member was not found
				return 0, fmt.Errorf("undefined module member: %s", fullName)
			}
		}
		
		// Special handling for loop iterator field access in INTO mode
		if sym != nil {
			if varSym, ok := sym.(*VarSymbol); ok && varSym.BufferAddr != 0 {
			// This is a loop iterator in INTO mode - use direct buffer address
			structType, ok := varSym.Type.(*ir.StructType)
			if !ok {
				return 0, fmt.Errorf("field access on non-struct iterator %s", id.Name)
			}
			
			// Find field offset
			offset := 0
			found := false
			for _, fname := range structType.FieldOrder {
				if fname == field.Field {
					found = true
					break
				}
				offset += structType.Fields[fname].Size()
			}
			
			if !found {
				return 0, fmt.Errorf("struct %s has no field %s", structType.Name, field.Field)
			}
			
			// Generate direct memory access to buffer
			resultReg := irFunc.AllocReg()
			directAddr := varSym.BufferAddr + uint16(offset)
			
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:      ir.OpLoadDirect,
				Dest:    resultReg,
				Imm:     int64(directAddr),
				Type:    structType.Fields[field.Field],
				Comment: fmt.Sprintf("Load %s.%s from buffer at $%04X", id.Name, field.Field, directAddr),
			})
			
			// Store type information
			a.exprTypes[field] = structType.Fields[field.Field]
			
			return resultReg, nil
			}
		}
	}
	
	// Normal field access - analyze the object
	objReg, err := a.analyzeExpression(field.Object, irFunc)
	if err != nil {
		return 0, err
	}
	
	// Get the object type
	objType := a.exprTypes[field.Object]
	if objType == nil {
		return 0, fmt.Errorf("cannot determine type of field object")
	}
	
	var structType *ir.StructType
	var fieldType ir.Type
	
	// Handle both direct struct and pointer to struct
	switch t := objType.(type) {
	case *ir.StructType:
		structType = t
	case *ir.PointerType:
		if st, ok := t.Base.(*ir.StructType); ok {
			structType = st
		} else {
			return 0, fmt.Errorf("field access on non-struct pointer")
		}
	case *ir.BitStructType:
		// Handle bit struct field access
		bitField, exists := t.Fields[field.Field]
		if !exists {
			return 0, fmt.Errorf("bit struct has no field %s", field.Field)
		}
		
		// For bit field access, we need to generate bit manipulation code
		// For now, just allocate a register and mark it for special handling
		resultReg := irFunc.AllocReg()
		
		// Store type information - bit fields are treated as their underlying type
		a.exprTypes[field] = t.UnderlyingType
		
		// Generate bit field load instruction
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpLoadBitField,
			Dest:    resultReg,
			Src1:    objReg,
			Imm:     int64(bitField.BitOffset),
			Imm2:    int64(bitField.BitWidth),
			Type:    t.UnderlyingType,
			Comment: fmt.Sprintf("Load bit field %s (offset %d, width %d)", field.Field, bitField.BitOffset, bitField.BitWidth),
		})
		
		return resultReg, nil
	default:
		return 0, fmt.Errorf("field access on non-struct type: %T", objType)
	}
	
	// Find field offset and type
	offset := 0
	found := false
	for _, fname := range structType.FieldOrder {
		if fname == field.Field {
			fieldType = structType.Fields[fname]
			found = true
			break
		}
		offset += structType.Fields[fname].Size()
	}
	
	if !found {
		return 0, fmt.Errorf("struct %s has no field %s", structType.Name, field.Field)
	}
	
	// Store type information for the field expression
	a.exprTypes[field] = fieldType
	
	// Allocate result register
	resultReg := irFunc.AllocReg()
	
	// Generate field load
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpLoadField,
		Dest:    resultReg,
		Src1:    objReg,
		Imm:     int64(offset),
		Type:    fieldType,
		Comment: fmt.Sprintf("Load field %s (offset %d)", field.Field, offset),
	})
	
	return resultReg, nil
}


// analyzeInlineAsmExpr analyzes inline assembly as an expression
func (a *Analyzer) analyzeInlineAsmExpr(asm *ast.InlineAsmExpr, irFunc *ir.Function) (ir.Register, error) {
	// For now, inline assembly expressions don't have a return value
	// Just emit the assembly code
	resolvedCode := asm.Code
	
	// Generate inline assembly instruction
	// Inline asm expressions don't return a value, so we return a dummy register
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpAsm,
		AsmCode: resolvedCode,
		Comment: "Inline assembly expression",
	})
	
	// Return register 0 as a placeholder (inline asm has no return value)
	return 0, nil
}

// analyzeIndexExpr analyzes an array index expression
func (a *Analyzer) analyzeIndexExpr(index *ast.IndexExpr, irFunc *ir.Function) (ir.Register, error) {
	// Analyze the array expression
	arrayReg, err := a.analyzeExpression(index.Array, irFunc)
	if err != nil {
		return 0, err
	}
	
	// Analyze the index expression
	indexReg, err := a.analyzeExpression(index.Index, irFunc)
	if err != nil {
		return 0, err
	}
	
	// Get the type of the array expression
	arrayType, err := a.inferType(index.Array)
	if err != nil {
		return 0, fmt.Errorf("cannot determine array type: %v", err)
	}
	
	
	// Validate that it's an array or pointer type
	var elementType ir.Type
	switch t := arrayType.(type) {
	case *ir.ArrayType:
		elementType = t.Element
	case *ir.PointerType:
		// For pointers, assume they point to u8 (byte arrays)
		elementType = &ir.BasicType{Kind: ir.TypeU8}
	default:
		return 0, fmt.Errorf("cannot index non-array type %s", arrayType)
	}
	
	// Allocate result register
	resultReg := irFunc.AllocReg()
	
	// Generate indexed load with element type info
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLoadIndex,
		Dest: resultReg,
		Src1: arrayReg,
		Src2: indexReg,
		Type: elementType,
		Comment: fmt.Sprintf("Load array element (%s)", elementType),
	})
	
	// Store the type of this expression for later use
	a.exprTypes[index] = elementType
	
	return resultReg, nil
}

// analyzeStringLiteral analyzes a string literal
func (a *Analyzer) analyzeStringLiteral(str *ast.StringLiteral, irFunc *ir.Function) (ir.Register, error) {
	// Create a unique label for the string
	label := fmt.Sprintf("str_%d", len(a.module.Strings))
	
	// Add string to module's string table
	a.module.Strings = append(a.module.Strings, &ir.String{
		Label: label,
		Value: str.Value,
	})
	
	// Allocate register for the pointer
	reg := irFunc.AllocReg()
	
	// Generate instruction to load string address
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpLoadLabel,
		Dest:    reg,
		Symbol:  label,
		Comment: fmt.Sprintf("Load string \"%s\"", str.Value),
	})
	
	return reg, nil
}

// analyzeEnumLiteral analyzes an enum literal
func (a *Analyzer) analyzeEnumLiteral(lit *ast.EnumLiteral, irFunc *ir.Function) (ir.Register, error) {
	// Look up the enum type
	sym := a.currentScope.Lookup(lit.EnumName)
	if sym == nil {
		return 0, fmt.Errorf("undefined enum: %s", lit.EnumName)
	}
	
	typeSym, ok := sym.(*TypeSymbol)
	if !ok {
		return 0, fmt.Errorf("%s is not a type", lit.EnumName)
	}
	
	enumType, ok := typeSym.Type.(*ir.EnumType)
	if !ok {
		return 0, fmt.Errorf("%s is not an enum type", lit.EnumName)
	}
	
	// Get variant value
	value, exists := enumType.Variants[lit.Variant]
	if !exists {
		return 0, fmt.Errorf("no variant %s in enum %s", lit.Variant, lit.EnumName)
	}
	
	// Generate constant load
	resultReg := irFunc.AllocReg()
	irFunc.EmitImm(ir.OpLoadConst, resultReg, int64(value))
	
	return resultReg, nil
}

// analyzeLuaExpression analyzes a Lua expression
func (a *Analyzer) analyzeLuaExpression(expr *ast.LuaExpression, irFunc *ir.Function) (ir.Register, error) {
	// Temporarily disabled - just return a constant
	resultReg := irFunc.AllocReg()
	irFunc.EmitImm(ir.OpLoadConst, resultReg, 0)
	return resultReg, nil
}

// analyzeCastExpr analyzes a type cast expression
func (a *Analyzer) analyzeCastExpr(cast *ast.CastExpr, irFunc *ir.Function) (ir.Register, error) {
	// Analyze the expression being cast
	exprReg, err := a.analyzeExpression(cast.Expr, irFunc)
	if err != nil {
		return 0, err
	}
	
	// Convert the target type
	targetType, err := a.convertType(cast.TargetType)
	if err != nil {
		return 0, fmt.Errorf("invalid cast target type: %w", err)
	}
	
	// Get the source type
	sourceType := a.exprTypes[cast.Expr]
	if sourceType == nil {
		return 0, fmt.Errorf("cannot determine type of expression being cast")
	}
	
	// Check if cast is valid
	if !a.isValidCast(sourceType, targetType) {
		return 0, fmt.Errorf("invalid cast from %s to %s", sourceType, targetType)
	}
	
	// For now, casts between compatible types are no-ops at the IR level
	// The type system ensures safety, but the bits are the same
	// Store the target type for the cast expression
	a.exprTypes[cast] = targetType
	
	// In most cases, the cast is just a reinterpretation of the bits
	// Special handling may be needed for:
	// - Widening conversions (u8 -> u16)
	// - Sign extensions (i8 -> i16)
	// - Bit struct conversions
	
	// For bit structs, the underlying representation is the same as the base type
	if _, ok := targetType.(*ir.BitStructType); ok {
		// Bit struct cast - just change the type interpretation
		return exprReg, nil
	}
	if _, ok := sourceType.(*ir.BitStructType); ok {
		// Cast from bit struct - just use the underlying value
		return exprReg, nil
	}
	
	// For basic type conversions, we might need actual conversion code
	// For now, return the same register - the code generator will handle it
	return exprReg, nil
}

// isValidCast checks if a cast from source to target type is valid
func (a *Analyzer) isValidCast(source, target ir.Type) bool {
	// Allow casts between bit structs and their underlying types
	if bitStruct, ok := source.(*ir.BitStructType); ok {
		return a.typesEqual(bitStruct.UnderlyingType, target)
	}
	if bitStruct, ok := target.(*ir.BitStructType); ok {
		return a.typesEqual(source, bitStruct.UnderlyingType)
	}
	
	// Allow casts between compatible basic types
	sourceBasic, sourceOk := source.(*ir.BasicType)
	targetBasic, targetOk := target.(*ir.BasicType)
	if sourceOk && targetOk {
		// Allow casts between integer types
		switch sourceBasic.Kind {
		case ir.TypeU8, ir.TypeU16, ir.TypeI8, ir.TypeI16:
			switch targetBasic.Kind {
			case ir.TypeU8, ir.TypeU16, ir.TypeI8, ir.TypeI16:
				return true
			}
		}
	}
	
	return false
}

// typesEqual checks if two types are equal
func (a *Analyzer) typesEqual(t1, t2 ir.Type) bool {
	// Simple comparison for basic types
	b1, ok1 := t1.(*ir.BasicType)
	b2, ok2 := t2.(*ir.BasicType)
	if ok1 && ok2 {
		return b1.Kind == b2.Kind
	}
	
	// For other types, use string comparison (not ideal but works for now)
	return t1.String() == t2.String()
}

// evaluateConstExpr evaluates a constant expression and returns its value
func (a *Analyzer) evaluateConstExpr(expr ast.Expression) (interface{}, error) {
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		return e.Value, nil
	case *ast.StringLiteral:
		return e.Value, nil
	case *ast.BooleanLiteral:
		return e.Value, nil
	case *ast.UnaryExpr:
		// Handle unary operators on constants
		operand, err := a.evaluateConstExpr(e.Operand)
		if err != nil {
			return nil, err
		}
		switch e.Operator {
		case "-":
			if val, ok := operand.(int64); ok {
				return -val, nil
			} else if val, ok := operand.(int); ok {
				return -val, nil
			}
		case "!":
			if val, ok := operand.(bool); ok {
				return !val, nil
			}
		case "~":
			if val, ok := operand.(int64); ok {
				return ^val, nil
			} else if val, ok := operand.(int); ok {
				return ^val, nil
			}
		}
		return nil, fmt.Errorf("cannot evaluate unary operator %s on constant", e.Operator)
	case *ast.BinaryExpr:
		// Handle binary operators on constants
		left, err := a.evaluateConstExpr(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := a.evaluateConstExpr(e.Right)
		if err != nil {
			return nil, err
		}
		
		// Try integer operations - handle both int and int64
		var lInt, rInt int64
		switch l := left.(type) {
		case int64:
			lInt = l
		case int:
			lInt = int64(l)
		default:
			return nil, fmt.Errorf("left operand is not an integer")
		}
		
		switch r := right.(type) {
		case int64:
			rInt = r
		case int:
			rInt = int64(r)
		default:
			return nil, fmt.Errorf("right operand is not an integer")
		}
		
		switch e.Operator {
		case "+":
			return lInt + rInt, nil
		case "-":
			return lInt - rInt, nil
		case "*":
			return lInt * rInt, nil
		case "/":
			if rInt != 0 {
				return lInt / rInt, nil
			}
			return nil, fmt.Errorf("division by zero")
		case "%":
			if rInt != 0 {
				return lInt % rInt, nil
			}
			return nil, fmt.Errorf("modulo by zero")
		case "&":
			return lInt & rInt, nil
		case "|":
			return lInt | rInt, nil
		case "^":
			return lInt ^ rInt, nil
		case "<<":
			return lInt << uint(rInt), nil
		case ">>":
			return lInt >> uint(rInt), nil
		}
		
		return nil, fmt.Errorf("cannot evaluate binary operator %s on constants", e.Operator)
	default:
		return nil, fmt.Errorf("expression is not a constant")
	}
}

// Close cleans up resources
func (a *Analyzer) Close() {
	// if a.luaEvaluator != nil {
	// 	a.luaEvaluator.Close()
	// }
}

// convertType converts an AST type to an IR type
func (a *Analyzer) convertType(astType ast.Type) (ir.Type, error) {
	switch t := astType.(type) {
	case *ast.PrimitiveType:
		switch t.Name {
		case "void":
			return &ir.BasicType{Kind: ir.TypeVoid}, nil
		case "bool":
			return &ir.BasicType{Kind: ir.TypeBool}, nil
		case "u8":
			return &ir.BasicType{Kind: ir.TypeU8}, nil
		case "u16":
			return &ir.BasicType{Kind: ir.TypeU16}, nil
		case "i8":
			return &ir.BasicType{Kind: ir.TypeI8}, nil
		case "i16":
			return &ir.BasicType{Kind: ir.TypeI16}, nil
		default:
			return nil, fmt.Errorf("unknown primitive type: %s", t.Name)
		}
	case *ast.PointerType:
		base, err := a.convertType(t.BaseType)
		if err != nil {
			return nil, err
		}
		return &ir.PointerType{Base: base}, nil
	case *ast.ArrayType:
		elem, err := a.convertType(t.ElementType)
		if err != nil {
			return nil, err
		}
		// For now, only support constant size arrays
		if num, ok := t.Size.(*ast.NumberLiteral); ok {
			return &ir.ArrayType{
				Element: elem,
				Length:  int(num.Value),
			}, nil
		}
		return nil, fmt.Errorf("array size must be a constant")
	case *ast.TypeIdentifier:
		// Look up the type in the symbol table
		sym := a.currentScope.Lookup(t.Name)
		
		// If not found, try with module prefix
		if sym == nil && a.currentModule != "" && a.currentModule != "main" {
			prefixedName := a.prefixSymbol(t.Name)
			sym = a.currentScope.Lookup(prefixedName)
		}
		
		if sym == nil {
			return nil, fmt.Errorf("undefined type: %s", t.Name)
		}
		typeSym, ok := sym.(*TypeSymbol)
		if !ok {
			return nil, fmt.Errorf("%s is not a type", t.Name)
		}
		return typeSym.Type, nil
	case *ast.BitStructType:
		// Determine underlying type (default to u8)
		var underlyingKind ir.TypeKind = ir.TypeU8
		if t.UnderlyingType != nil {
			// Get the underlying type
			if prim, ok := t.UnderlyingType.(*ast.PrimitiveType); ok && prim.Name == "u16" {
				underlyingKind = ir.TypeU16
			}
		}
		
		// Create bit struct type
		bitStruct := &ir.BitStructType{
			UnderlyingType: &ir.BasicType{Kind: underlyingKind},
			Fields: make(map[string]*ir.BitField),
			FieldOrder: []string{},
		}
		
		// Process fields
		bitOffset := 0
		for _, field := range t.Fields {
			if _, exists := bitStruct.Fields[field.Name]; exists {
				return nil, fmt.Errorf("duplicate field %s in bit struct", field.Name)
			}
			
			// Validate bit width
			maxBits := 8
			if underlyingKind == ir.TypeU16 {
				maxBits = 16
			}
			if field.BitWidth < 1 || field.BitWidth > maxBits {
				return nil, fmt.Errorf("bit field %s width %d out of range (1-%d)", field.Name, field.BitWidth, maxBits)
			}
			
			// Check if field fits
			if bitOffset + field.BitWidth > maxBits {
				return nil, fmt.Errorf("bit field %s exceeds %d-bit boundary", field.Name, maxBits)
			}
			
			// Add field
			bitStruct.Fields[field.Name] = &ir.BitField{
				Name:      field.Name,
				BitOffset: bitOffset,
				BitWidth:  field.BitWidth,
			}
			bitStruct.FieldOrder = append(bitStruct.FieldOrder, field.Name)
			bitOffset += field.BitWidth
		}
		
		return bitStruct, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", astType)
	}
}

// inferType infers the type of an expression
func (a *Analyzer) inferType(expr ast.Expression) (ir.Type, error) {
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		// Infer type based on value
		if e.Value >= 0 && e.Value <= 255 {
			// Small positive values default to u8
			return &ir.BasicType{Kind: ir.TypeU8}, nil
		} else if e.Value >= -128 && e.Value <= 127 {
			// Small signed values default to i8
			return &ir.BasicType{Kind: ir.TypeI8}, nil
		} else if e.Value >= 0 && e.Value <= 65535 {
			// Larger positive values default to u16
			return &ir.BasicType{Kind: ir.TypeU16}, nil
		} else {
			// Default to i16 for negative or large values
			return &ir.BasicType{Kind: ir.TypeI16}, nil
		}
	case *ast.BooleanLiteral:
		return &ir.BasicType{Kind: ir.TypeBool}, nil
	case *ast.Identifier:
		sym := a.currentScope.Lookup(e.Name)
		
		// If not found, try with module prefix
		if sym == nil && a.currentModule != "" && a.currentModule != "main" {
			prefixedName := a.prefixSymbol(e.Name)
			sym = a.currentScope.Lookup(prefixedName)
		}
		
		if sym == nil {
			// Debug for screen
			if e.Name == "screen" {
				return nil, fmt.Errorf("undefined identifier: %s (inferType - module should have been found)", e.Name)
			}
			return nil, fmt.Errorf("undefined identifier: %s (inferType)", e.Name)
		}
		switch s := sym.(type) {
		case *VarSymbol:
			return s.Type, nil
		case *ModuleSymbol:
			return nil, fmt.Errorf("module %s cannot be used as a value", s.Name)
		default:
			return nil, fmt.Errorf("cannot infer type from %s", e.Name)
		}
	case *ast.CallExpr:
		// Infer type from function return type
		var funcName string
		var sym Symbol
		
		switch fn := e.Function.(type) {
		case *ast.Identifier:
			funcName = fn.Name
			sym = a.currentScope.Lookup(funcName)
			
			// If not found, try with module prefix
			if sym == nil && a.currentModule != "" && a.currentModule != "main" {
				prefixedName := a.prefixSymbol(funcName)
				sym = a.currentScope.Lookup(prefixedName)
				funcName = prefixedName
			}
			
		case *ast.FieldExpr:
			// Module function call
			if id, ok := fn.Object.(*ast.Identifier); ok {
				funcName = id.Name + "." + fn.Field
				sym = a.currentScope.Lookup(funcName)
			}
			
		default:
			return nil, fmt.Errorf("indirect function calls not yet supported for type inference")
		}
		
		if sym == nil {
			return nil, fmt.Errorf("undefined function: %s", funcName)
		}
		
		funcSym, ok := sym.(*FuncSymbol)
		if !ok {
			return nil, fmt.Errorf("cannot infer type from %s", funcName)
		}
		
		return funcSym.ReturnType, nil
	case *ast.BinaryExpr:
		// Infer type from binary expression
		leftType, err := a.inferType(e.Left)
		if err != nil {
			return nil, err
		}
		
		rightType, err := a.inferType(e.Right)
		if err != nil {
			return nil, err
		}
		
		// For arithmetic operations, use the larger type
		// For comparison operations, return bool
		switch e.Operator {
		case "==", "!=", "<", ">", "<=", ">=":
			return &ir.BasicType{Kind: ir.TypeBool}, nil
		case "+", "-", "*", "/", "%", "&", "|", "^", "<<", ">>":
			// Check if types match
			if !a.typesCompatible(leftType, rightType) {
				return nil, fmt.Errorf("type mismatch in binary expression: %s vs %s", 
					leftType.String(), rightType.String())
			}
			// For now, just return the left type
			// TODO: Implement proper type promotion rules
			return leftType, nil
		default:
			return nil, fmt.Errorf("cannot infer type for binary operator %s", e.Operator)
		}
	case *ast.UnaryExpr:
		// Infer type from unary expression
		operandType, err := a.inferType(e.Operand)
		if err != nil {
			return nil, err
		}
		
		switch e.Operator {
		case "-", "~":
			// Negation and bitwise not preserve the type
			return operandType, nil
		case "!":
			// Logical not returns bool
			return &ir.BasicType{Kind: ir.TypeBool}, nil
		case "&":
			// Address-of returns pointer to operand type
			return &ir.PointerType{Base: operandType}, nil
		default:
			return nil, fmt.Errorf("cannot infer type for unary operator %s", e.Operator)
		}
	case *ast.IndexExpr:
		// Infer element type from array type
		arrayType, err := a.inferType(e.Array)
		if err != nil {
			return nil, err
		}
		
		// Check if it's an array type
		if arrType, ok := arrayType.(*ir.ArrayType); ok {
			return arrType.Element, nil
		}
		
		return nil, fmt.Errorf("cannot index non-array type %s", arrayType.String())
	case *ast.StringLiteral:
		// String literals are pointers to u8
		return &ir.PointerType{
			Base: &ir.BasicType{Kind: ir.TypeU8},
		}, nil
	case *ast.FieldExpr:
		// Check if this is a module field access
		if id, ok := e.Object.(*ast.Identifier); ok {
			// Check if the object is a module
			sym := a.currentScope.Lookup(id.Name)
			if _, isModule := sym.(*ModuleSymbol); isModule {
				// This is a module member - look up the full qualified name
				fullName := id.Name + "." + e.Field
				memberSym := a.currentScope.Lookup(fullName)
				if memberSym != nil {
					// Check if this is a constant
					if constSym, ok := memberSym.(*ConstSymbol); ok {
						return constSym.Type, nil
					} else if _, ok := memberSym.(*FuncSymbol); ok {
						// This is a module function - it's not a value by itself
						// The type will be determined by the CallExpr that uses it
						return nil, fmt.Errorf("module function %s must be called, not used as a value", fullName)
					}
				}
				return nil, fmt.Errorf("undefined module member: %s", fullName)
			}
		}
		
		// Normal struct field access - infer type from struct field
		objType, err := a.inferType(e.Object)
		if err != nil {
			return nil, err
		}
		
		// Handle pointer dereferencing
		actualType := objType
		if ptrType, ok := objType.(*ir.PointerType); ok {
			actualType = ptrType.Base
		}
		
		// Check if it's a struct type
		structType, ok := actualType.(*ir.StructType)
		if !ok {
			return nil, fmt.Errorf("cannot access field %s on non-struct type %s", e.Field, actualType.String())
		}
		
		// Look up field type
		fieldType, exists := structType.Fields[e.Field]
		if !exists {
			return nil, fmt.Errorf("struct %s has no field %s", structType.Name, e.Field)
		}
		
		return fieldType, nil
	case *ast.CastExpr:
		// The type of a cast expression is its target type
		targetType, err := a.convertType(e.TargetType)
		if err != nil {
			return nil, fmt.Errorf("invalid cast target type: %w", err)
		}
		return targetType, nil
	default:
		return nil, fmt.Errorf("cannot infer type from expression of type %T", expr)
	}
}

// typesCompatible checks if two types are compatible for assignment
func (a *Analyzer) typesCompatible(declared, inferred ir.Type) bool {
	declBasic, declOk := declared.(*ir.BasicType)
	infBasic, infOk := inferred.(*ir.BasicType)
	
	if declOk && infOk {
		// Same type is always compatible
		if declBasic.Kind == infBasic.Kind {
			return true
		}
		
		// Check for numeric compatibility
		// Allow implicit conversions that don't lose data
		switch declBasic.Kind {
		case ir.TypeU8:
			// u8 can accept values from other types if they fit
			return infBasic.Kind == ir.TypeU8
		case ir.TypeU16:
			// u16 can accept u8 and u16
			return infBasic.Kind == ir.TypeU8 || infBasic.Kind == ir.TypeU16
		case ir.TypeI8:
			// i8 can only accept i8
			return infBasic.Kind == ir.TypeI8
		case ir.TypeI16:
			// i16 can accept i8, i16, u8
			return infBasic.Kind == ir.TypeI8 || infBasic.Kind == ir.TypeI16 || infBasic.Kind == ir.TypeU8
		}
	}
	
	// For other types, use string comparison for now
	return declared.String() == inferred.String()
}

var labelCounter int

// generateLabel generates a unique label
func (a *Analyzer) generateLabel(prefix string) string {
	labelCounter++
	return fmt.Sprintf("%s_%d", prefix, labelCounter)
}

// processAbiAttributes processes @abi attributes on function declarations
func (a *Analyzer) processAbiAttributes(fn *ast.FunctionDecl, irFunc *ir.Function) error {
	for _, attr := range fn.Attributes {
		if attr.Name == "abi" {
			return a.processAbiAttribute(attr, irFunc)
		}
	}
	return nil
}

// processAbiAttribute processes a single @abi attribute
func (a *Analyzer) processAbiAttribute(attr *ast.Attribute, irFunc *ir.Function) error {
	// Extract the value from the first argument
	var value string
	if len(attr.Arguments) > 0 {
		if strLit, ok := attr.Arguments[0].(*ast.StringLiteral); ok {
			value = strLit.Value
		} else {
			return fmt.Errorf("@abi attribute expects a string argument")
		}
	} else {
		return fmt.Errorf("@abi attribute requires an argument")
	}
	
	switch value {
	case "smc":
		irFunc.CallingConvention = "smc"
		irFunc.IsSMCDefault = true
		irFunc.IsSMCEnabled = true
		irFunc.SMCParamOffsets = make(map[string]int)
	case "register":
		irFunc.CallingConvention = "register"
		irFunc.IsSMCDefault = false
		irFunc.IsSMCEnabled = false
	case "stack":
		irFunc.CallingConvention = "stack"
		irFunc.IsSMCDefault = false
		irFunc.IsSMCEnabled = false
	case "shadow":
		irFunc.CallingConvention = "shadow"
		irFunc.IsSMCDefault = false
		irFunc.IsSMCEnabled = false
	case "virtual":
		irFunc.CallingConvention = "virtual"
		irFunc.IsSMCDefault = false
		irFunc.IsSMCEnabled = false
	case "naked":
		irFunc.CallingConvention = "naked"
		irFunc.IsSMCDefault = false
		irFunc.IsSMCEnabled = false
	default:
		// Handle complex register mappings like "register: A=x, HL=ptr"
		if strings.HasPrefix(value, "register:") {
			irFunc.CallingConvention = "register_mapped"
			irFunc.IsSMCDefault = false
			irFunc.IsSMCEnabled = false
			// Parse register mappings and store in metadata
			mappings := strings.TrimPrefix(value, "register:")
			mappings = strings.TrimSpace(mappings)
			if irFunc.Metadata == nil {
				irFunc.Metadata = make(map[string]string)
			}
			irFunc.Metadata["register_mappings"] = mappings
		} else {
			return fmt.Errorf("unsupported @abi value: %s", value)
		}
	}
	return nil
}