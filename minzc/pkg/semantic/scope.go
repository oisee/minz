package semantic

import (
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// Symbol represents a symbol in the symbol table
type Symbol interface {
	symbol()
}

// VarSymbol represents a variable
type VarSymbol struct {
	Name        string
	Type        ir.Type
	Reg         ir.Register
	IsMutable   bool
	IsParameter bool
	BufferAddr  uint16  // For loop iterator in INTO mode
}

func (v *VarSymbol) symbol() {}

// ConstSymbol represents a constant
type ConstSymbol struct {
	Name  string
	Type  ir.Type
	Value int64
}

func (c *ConstSymbol) symbol() {}

// FuncSymbol represents a function
type FuncSymbol struct {
	Name         string
	OriginalName string             // Original name before mangling (for local functions)
	Params       []*ast.Parameter
	ParamTypes   []ir.Type          // Converted parameter types for display
	ReturnType   ir.Type
	ErrorType    ir.Type           // Optional error type for functions ending with ?
	Type         *ir.FunctionType  // For built-in functions
	IsBuiltin    bool
	IsLocalFunc  bool              // True if this is a local function
}

func (f *FuncSymbol) symbol() {}

// TypeSymbol represents a type
type TypeSymbol struct {
	Name string
	Type ir.Type
}

func (t *TypeSymbol) symbol() {}

// NamespaceSymbol represents an imported module namespace
type NamespaceSymbol struct {
	Name    string
	Module  string
	Exports map[string]Symbol
}

func (n *NamespaceSymbol) symbol() {}

// ModuleSymbol represents a module for simple module system
type ModuleSymbol struct {
	Name string
}

func (m *ModuleSymbol) symbol() {}

// InterfaceSymbol represents an interface
type InterfaceSymbol struct {
	Name    string
	Methods map[string]*InterfaceMethod
}

func (i *InterfaceSymbol) symbol() {}

// InterfaceMethod represents a method in an interface
type InterfaceMethod struct {
	Name       string
	Params     []*ast.Parameter
	ReturnType ir.Type
}

// ImplSymbol represents an implementation of an interface for a type
type ImplSymbol struct {
	InterfaceName string
	TypeName      string
	Methods       map[string]*FuncSymbol
}

func (i *ImplSymbol) symbol() {}

// FunctionOverloadSet tracks all overloaded versions of a function
type FunctionOverloadSet struct {
	BaseName  string
	Overloads map[string]*FuncSymbol // Key is mangled name
}

func (f *FunctionOverloadSet) symbol() {}

// Scope represents a lexical scope
type Scope struct {
	parent    *Scope
	symbols   map[string]Symbol
	overloads map[string]*FunctionOverloadSet // Key is base function name
}

// NewScope creates a new scope
func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:    parent,
		symbols:   make(map[string]Symbol),
		overloads: make(map[string]*FunctionOverloadSet),
	}
}

// Define adds a symbol to the scope
func (s *Scope) Define(name string, symbol Symbol) {
	s.symbols[name] = symbol
}

// Lookup searches for a symbol in this scope and parent scopes
func (s *Scope) Lookup(name string) Symbol {
	if sym, ok := s.symbols[name]; ok {
		return sym
	}
	if s.parent != nil {
		return s.parent.Lookup(name)
	}
	return nil
}

// LookupLocal searches for a symbol only in this scope
func (s *Scope) LookupLocal(name string) Symbol {
	return s.symbols[name]
}