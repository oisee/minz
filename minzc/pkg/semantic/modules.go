package semantic

import (
	"fmt"
	"os"
	"github.com/minz/minzc/pkg/ir"
)

// BuiltinModule represents a built-in module with pre-defined symbols
type BuiltinModule struct {
	Name      string
	Functions map[string]*FuncSymbol
	Constants map[string]*ConstSymbol
	Types     map[string]*TypeSymbol
}

// InitBuiltinModules initializes all built-in modules
func InitBuiltinModules() map[string]*BuiltinModule {
	return map[string]*BuiltinModule{
		"std":       createStdModule(),
		"zx.screen": createZXScreenModule(),
		"zx.input":  createZXInputModule(),
		"zx.sound":  createZXSoundModule(),
	}
}

// createStdModule creates the standard library module
func createStdModule() *BuiltinModule {
	return &BuiltinModule{
		Name: "std",
		Functions: map[string]*FuncSymbol{
			// Polymorphic print - accepts any basic type
			"print": {
				Name:      "print",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{nil}, // Polymorphic - checked at call site
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"print_string": {
				Name:      "print_string",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.PointerType{Base: &ir.BasicType{Kind: ir.TypeU8}},
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"println": {
				Name:      "println",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{nil}, // Polymorphic
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"cls": {
				Name:      "cls",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"hex": {
				Name:      "hex",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8},
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			// Memory functions
			"memcpy": {
				Name:      "memcpy",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.PointerType{Base: &ir.BasicType{Kind: ir.TypeU8}}, // dest
						&ir.PointerType{Base: &ir.BasicType{Kind: ir.TypeU8}}, // src
						&ir.BasicType{Kind: ir.TypeU16},                       // size
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"memset": {
				Name:      "memset",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.PointerType{Base: &ir.BasicType{Kind: ir.TypeU8}}, // dest
						&ir.BasicType{Kind: ir.TypeU8},                        // value
						&ir.BasicType{Kind: ir.TypeU16},                       // size
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"len": {
				Name:      "len",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{nil}, // Accepts arrays or pointers
					Return: &ir.BasicType{Kind: ir.TypeU16},
				},
			},
			// Math functions
			"abs": {
				Name:      "abs",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeI8},
					},
					Return: &ir.BasicType{Kind: ir.TypeU8},
				},
			},
			"min": {
				Name:      "min",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8},
						&ir.BasicType{Kind: ir.TypeU8},
					},
					Return: &ir.BasicType{Kind: ir.TypeU8},
				},
			},
			"max": {
				Name:      "max",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8},
						&ir.BasicType{Kind: ir.TypeU8},
					},
					Return: &ir.BasicType{Kind: ir.TypeU8},
				},
			},
		},
		Constants: map[string]*ConstSymbol{},
		Types:     map[string]*TypeSymbol{},
	}
}

// createZXScreenModule creates the ZX Spectrum screen module
func createZXScreenModule() *BuiltinModule {
	return &BuiltinModule{
		Name: "zx.screen",
		Functions: map[string]*FuncSymbol{
			"set_pixel": {
				Name:      "set_pixel",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8}, // x
						&ir.BasicType{Kind: ir.TypeU8}, // y
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"clear": {
				Name:      "clear",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"set_border": {
				Name:      "set_border",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8}, // color
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"set_ink": {
				Name:      "set_ink",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8}, // color
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"set_paper": {
				Name:      "set_paper",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8}, // color
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"plot": {
				Name:      "plot",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8}, // x
						&ir.BasicType{Kind: ir.TypeU8}, // y
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
			"draw_line": {
				Name:      "draw_line",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8}, // x1
						&ir.BasicType{Kind: ir.TypeU8}, // y1
						&ir.BasicType{Kind: ir.TypeU8}, // x2
						&ir.BasicType{Kind: ir.TypeU8}, // y2
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
		},
		Constants: map[string]*ConstSymbol{
			"SCREEN_WIDTH": {
				Name:  "SCREEN_WIDTH",
				Type:  &ir.BasicType{Kind: ir.TypeU16},
				Value: int64(256),
			},
			"SCREEN_HEIGHT": {
				Name:  "SCREEN_HEIGHT",
				Type:  &ir.BasicType{Kind: ir.TypeU16},
				Value: int64(192),
			},
			"ATTR_START": {
				Name:  "ATTR_START",
				Type:  &ir.BasicType{Kind: ir.TypeU16},
				Value: int64(0x5800),
			},
			"SCREEN_START": {
				Name:  "SCREEN_START",
				Type:  &ir.BasicType{Kind: ir.TypeU16},
				Value: int64(0x4000),
			},
		},
		Types: map[string]*TypeSymbol{},
	}
}

// createZXInputModule creates the ZX Spectrum input module
func createZXInputModule() *BuiltinModule {
	return &BuiltinModule{
		Name: "zx.input",
		Functions: map[string]*FuncSymbol{
			"read_keyboard": {
				Name:      "read_keyboard",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{},
					Return: &ir.BasicType{Kind: ir.TypeU8},
				},
			},
			"wait_key": {
				Name:      "wait_key",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{},
					Return: &ir.BasicType{Kind: ir.TypeU8},
				},
			},
			"is_key_pressed": {
				Name:      "is_key_pressed",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU8}, // key code
					},
					Return: &ir.BasicType{Kind: ir.TypeBool},
				},
			},
		},
		Constants: map[string]*ConstSymbol{},
		Types:     map[string]*TypeSymbol{},
	}
}

// createZXSoundModule creates the ZX Spectrum sound module
func createZXSoundModule() *BuiltinModule {
	return &BuiltinModule{
		Name: "zx.sound",
		Functions: map[string]*FuncSymbol{
			"beep": {
				Name:      "beep",
				IsBuiltin: true,
				Type: &ir.FunctionType{
					Params: []ir.Type{
						&ir.BasicType{Kind: ir.TypeU16}, // frequency
						&ir.BasicType{Kind: ir.TypeU16}, // duration
					},
					Return: &ir.BasicType{Kind: ir.TypeVoid},
				},
			},
		},
		Constants: map[string]*ConstSymbol{},
		Types:     map[string]*TypeSymbol{},
	}
}

// RegisterModule registers a built-in module's symbols in the current scope
func (a *Analyzer) RegisterModule(module *BuiltinModule, importAlias string) error {
	prefix := module.Name
	if importAlias != "" {
		prefix = importAlias
	}
	
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("DEBUG: RegisterModule %s with alias '%s', using prefix '%s'\n", module.Name, importAlias, prefix)
	}

	// Register functions
	for name, fn := range module.Functions {
		qualifiedName := fmt.Sprintf("%s.%s", prefix, name)
		a.currentScope.Define(qualifiedName, fn)
		
		// Also register without prefix for convenience (like Python's "from module import *")
		// Only do this for std module to avoid namespace pollution
		if module.Name == "std" && importAlias == "" {
			a.currentScope.Define(name, fn)
		}
	}

	// Register constants
	for name, c := range module.Constants {
		qualifiedName := fmt.Sprintf("%s.%s", prefix, name)
		a.currentScope.Define(qualifiedName, c)
		
		// Also register without prefix for std module
		if module.Name == "std" && importAlias == "" {
			a.currentScope.Define(name, c)
		}
	}

	// Register types
	for name, t := range module.Types {
		qualifiedName := fmt.Sprintf("%s.%s", prefix, name)
		a.currentScope.Define(qualifiedName, t)
		
		if module.Name == "std" && importAlias == "" {
			a.currentScope.Define(name, t)
		}
	}

	return nil
}