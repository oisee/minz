package semantic

import (
	"fmt"
	"github.com/minz/minzc/pkg/ast"
)

// ErrorWithPosition represents an error with source position information
type ErrorWithPosition struct {
	Message  string
	Position ast.Position
	File     string
	Context  string // Optional: line of code for context
}

func (e ErrorWithPosition) Error() string {
	if e.File != "" && e.Position.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Position.Line, e.Position.Column, e.Message)
	} else if e.Position.Line > 0 {
		return fmt.Sprintf("line %d, col %d: %s", e.Position.Line, e.Position.Column, e.Message)
	}
	return e.Message
}

// Helper function to create positioned errors
func (a *Analyzer) errorAt(node ast.Node, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return ErrorWithPosition{
		Message:  msg,
		Position: node.Pos(),
		File:     a.currentFile,
	}
}

// Helper for identifier-specific errors
func (a *Analyzer) undefinedIdentifierError(id *ast.Identifier, context string) error {
	msg := fmt.Sprintf("undefined identifier '%s'", id.Name)
	if context != "" {
		msg = fmt.Sprintf("%s: %s", context, msg)
	}
	
	// Add suggestions if available
	suggestions := a.findSimilarIdentifiers(id.Name)
	if len(suggestions) > 0 {
		msg = fmt.Sprintf("%s - did you mean '%s'?", msg, suggestions[0])
	}
	
	return ErrorWithPosition{
		Message:  msg,
		Position: id.Pos(),
		File:     a.currentFile,
	}
}

// Helper for function call errors
func (a *Analyzer) undefinedFunctionError(call *ast.CallExpr, funcName string) error {
	msg := fmt.Sprintf("undefined function: %s", funcName)
	
	// Add suggestions if available
	suggestions := a.findSimilarIdentifiers(funcName)
	if len(suggestions) > 0 {
		msg = fmt.Sprintf("%s - did you mean '%s'?", msg, suggestions[0])
	}
	
	return ErrorWithPosition{
		Message:  msg,
		Position: call.Pos(),
		File:     a.currentFile,
	}
}

// Helper for type errors
func (a *Analyzer) typeMismatchError(node ast.Node, expected, got string) error {
	msg := fmt.Sprintf("type mismatch: expected %s, got %s", expected, got)
	return ErrorWithPosition{
		Message:  msg,
		Position: node.Pos(),
		File:     a.currentFile,
	}
}