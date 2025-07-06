package meta

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/ast"
)

// LuaExpression is an alias for ast.LuaExpression
type LuaExpression = ast.LuaExpression

// Evaluator performs compile-time evaluation
type Evaluator struct {
	constants map[string]Value
	functions map[string]*CompileTimeFunc
	output    []string
}

// Value represents a compile-time value
type Value interface {
	Type() ValueType
	String() string
}

// ValueType represents the type of a compile-time value
type ValueType int

const (
	ValueInt ValueType = iota
	ValueBool
	ValueString
	ValueArray
	ValueStruct
)

// IntValue represents an integer value
type IntValue struct {
	Value int64
}

func (v *IntValue) Type() ValueType { return ValueInt }
func (v *IntValue) String() string  { return fmt.Sprintf("%d", v.Value) }

// BoolValue represents a boolean value
type BoolValue struct {
	Value bool
}

func (v *BoolValue) Type() ValueType { return ValueBool }
func (v *BoolValue) String() string  { return fmt.Sprintf("%t", v.Value) }

// StringValue represents a string value
type StringValue struct {
	Value string
}

func (v *StringValue) Type() ValueType { return ValueString }
func (v *StringValue) String() string  { return v.Value }

// ArrayValue represents an array value
type ArrayValue struct {
	Elements []Value
}

func (v *ArrayValue) Type() ValueType { return ValueArray }
func (v *ArrayValue) String() string {
	parts := make([]string, len(v.Elements))
	for i, elem := range v.Elements {
		parts[i] = elem.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// CompileTimeFunc represents a compile-time function
type CompileTimeFunc struct {
	Name   string
	Params []string
	Body   ast.Expression
}

// NewEvaluator creates a new compile-time evaluator
func NewEvaluator() *Evaluator {
	return &Evaluator{
		constants: make(map[string]Value),
		functions: make(map[string]*CompileTimeFunc),
		output:    []string{},
	}
}

// EvaluateExpression evaluates an expression at compile time
func (e *Evaluator) EvaluateExpression(expr ast.Expression) (Value, error) {
	switch ex := expr.(type) {
	case *ast.NumberLiteral:
		return &IntValue{Value: ex.Value}, nil
		
	case *ast.BooleanLiteral:
		return &BoolValue{Value: ex.Value}, nil
		
	case *ast.StringLiteral:
		// TODO: Parse string literal properly
		return &StringValue{Value: "string"}, nil
		
	case *ast.Identifier:
		if val, ok := e.constants[ex.Name]; ok {
			return val, nil
		}
		return nil, fmt.Errorf("undefined constant: %s", ex.Name)
		
	case *ast.BinaryExpr:
		return e.evaluateBinaryExpr(ex)
		
	case *ast.UnaryExpr:
		return e.evaluateUnaryExpr(ex)
		
	default:
		return nil, fmt.Errorf("unsupported expression type for compile-time evaluation: %T", expr)
	}
}

// evaluateBinaryExpr evaluates a binary expression
func (e *Evaluator) evaluateBinaryExpr(expr *ast.BinaryExpr) (Value, error) {
	left, err := e.EvaluateExpression(expr.Left)
	if err != nil {
		return nil, err
	}
	
	right, err := e.EvaluateExpression(expr.Right)
	if err != nil {
		return nil, err
	}
	
	// Type check
	if left.Type() != right.Type() {
		return nil, fmt.Errorf("type mismatch in binary expression")
	}
	
	switch expr.Operator {
	case "+":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &IntValue{Value: l.Value + r.Value}, nil
		}
		if left.Type() == ValueString {
			l := left.(*StringValue)
			r := right.(*StringValue)
			return &StringValue{Value: l.Value + r.Value}, nil
		}
		
	case "-":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &IntValue{Value: l.Value - r.Value}, nil
		}
		
	case "*":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &IntValue{Value: l.Value * r.Value}, nil
		}
		
	case "/":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			if r.Value == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return &IntValue{Value: l.Value / r.Value}, nil
		}
		
	case "==":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &BoolValue{Value: l.Value == r.Value}, nil
		}
		if left.Type() == ValueBool {
			l := left.(*BoolValue)
			r := right.(*BoolValue)
			return &BoolValue{Value: l.Value == r.Value}, nil
		}
		
	case "!=":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &BoolValue{Value: l.Value != r.Value}, nil
		}
		
	case "<":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &BoolValue{Value: l.Value < r.Value}, nil
		}
		
	case ">":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &BoolValue{Value: l.Value > r.Value}, nil
		}
		
	case "<=":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &BoolValue{Value: l.Value <= r.Value}, nil
		}
		
	case ">=":
		if left.Type() == ValueInt {
			l := left.(*IntValue)
			r := right.(*IntValue)
			return &BoolValue{Value: l.Value >= r.Value}, nil
		}
		
	case "and", "&&":
		if left.Type() == ValueBool {
			l := left.(*BoolValue)
			r := right.(*BoolValue)
			return &BoolValue{Value: l.Value && r.Value}, nil
		}
		
	case "or", "||":
		if left.Type() == ValueBool {
			l := left.(*BoolValue)
			r := right.(*BoolValue)
			return &BoolValue{Value: l.Value || r.Value}, nil
		}
	}
	
	return nil, fmt.Errorf("unsupported binary operator: %s", expr.Operator)
}

// evaluateUnaryExpr evaluates a unary expression
func (e *Evaluator) evaluateUnaryExpr(expr *ast.UnaryExpr) (Value, error) {
	operand, err := e.EvaluateExpression(expr.Operand)
	if err != nil {
		return nil, err
	}
	
	switch expr.Operator {
	case "-":
		if operand.Type() == ValueInt {
			val := operand.(*IntValue)
			return &IntValue{Value: -val.Value}, nil
		}
		
	case "!":
		if operand.Type() == ValueBool {
			val := operand.(*BoolValue)
			return &BoolValue{Value: !val.Value}, nil
		}
	}
	
	return nil, fmt.Errorf("unsupported unary operator: %s", expr.Operator)
}

// DefineConstant defines a compile-time constant
func (e *Evaluator) DefineConstant(name string, value Value) {
	e.constants[name] = value
}

// DefineFunction defines a compile-time function
func (e *Evaluator) DefineFunction(name string, fn *CompileTimeFunc) {
	e.functions[name] = fn
}

// Print adds output to the compile-time print buffer
func (e *Evaluator) Print(message string) {
	e.output = append(e.output, message)
}

// GetOutput returns all compile-time print output
func (e *Evaluator) GetOutput() []string {
	return e.output
}

// GenerateCode generates MinZ code from an AST
func GenerateCode(node ast.Node) (string, error) {
	// TODO: Implement AST to source code generation
	// This would be used for @generate metaprogramming
	return "", fmt.Errorf("code generation not yet implemented")
}