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
}

func (v *VarSymbol) symbol() {}

// FuncSymbol represents a function
type FuncSymbol struct {
	Name       string
	Params     []*ast.Parameter
	ReturnType ir.Type
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

// Scope represents a lexical scope
type Scope struct {
	parent  *Scope
	symbols map[string]Symbol
}

// NewScope creates a new scope
func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		symbols: make(map[string]Symbol),
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