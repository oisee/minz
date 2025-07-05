package semantic

import (
	"fmt"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// Analyzer performs semantic analysis on the AST
type Analyzer struct {
	currentScope *Scope
	errors       []error
	module       *ir.Module
}

// NewAnalyzer creates a new semantic analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		currentScope: NewScope(nil),
		errors:       []error{},
		module:       ir.NewModule("main"),
	}
}

// Analyze performs semantic analysis on a file
func (a *Analyzer) Analyze(file *ast.File) (*ir.Module, error) {
	// Add built-in types and functions
	a.addBuiltins()

	// Process imports
	for _, imp := range file.Imports {
		// TODO: Handle imports
		_ = imp
	}

	// Process declarations
	for _, decl := range file.Declarations {
		if err := a.analyzeDeclaration(decl); err != nil {
			a.errors = append(a.errors, err)
		}
	}

	if len(a.errors) > 0 {
		return nil, fmt.Errorf("semantic analysis failed with %d errors", len(a.errors))
	}

	return a.module, nil
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

// analyzeDeclaration analyzes a declaration
func (a *Analyzer) analyzeDeclaration(decl ast.Declaration) error {
	switch d := decl.(type) {
	case *ast.FunctionDecl:
		return a.analyzeFunctionDecl(d)
	case *ast.VarDecl:
		return a.analyzeVarDecl(d)
	default:
		return fmt.Errorf("unsupported declaration type: %T", decl)
	}
}

// analyzeFunctionDecl analyzes a function declaration
func (a *Analyzer) analyzeFunctionDecl(fn *ast.FunctionDecl) error {
	// Convert return type
	returnType, err := a.convertType(fn.ReturnType)
	if err != nil {
		return fmt.Errorf("invalid return type: %w", err)
	}

	// Create IR function
	irFunc := ir.NewFunction(fn.Name, returnType)

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
			Name: param.Name,
			Type: paramType,
			Reg:  reg,
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

	// Add function to module
	a.module.AddFunction(irFunc)

	// Register function in global scope
	a.currentScope.parent.Define(fn.Name, &FuncSymbol{
		Name:       fn.Name,
		ReturnType: returnType,
		Params:     fn.Params,
	})

	return nil
}

// analyzeVarDecl analyzes a variable declaration
func (a *Analyzer) analyzeVarDecl(v *ast.VarDecl) error {
	// Determine type
	var varType ir.Type
	if v.Type != nil {
		t, err := a.convertType(v.Type)
		if err != nil {
			return fmt.Errorf("invalid type for variable %s: %w", v.Name, err)
		}
		varType = t
	} else if v.Value != nil {
		// Type inference from value
		t, err := a.inferType(v.Value)
		if err != nil {
			return fmt.Errorf("cannot infer type for variable %s: %w", v.Name, err)
		}
		varType = t
	} else {
		return fmt.Errorf("variable %s must have either a type or an initial value", v.Name)
	}

	// Register variable
	a.currentScope.Define(v.Name, &VarSymbol{
		Name:      v.Name,
		Type:      varType,
		IsMutable: v.IsMutable,
	})

	return nil
}

// analyzeBlock analyzes a block statement
func (a *Analyzer) analyzeBlock(block *ast.BlockStmt, irFunc *ir.Function) error {
	// Enter new scope
	a.currentScope = NewScope(a.currentScope)
	defer func() { a.currentScope = a.currentScope.parent }()

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
	default:
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// analyzeVarDeclInFunc analyzes a variable declaration inside a function
func (a *Analyzer) analyzeVarDeclInFunc(v *ast.VarDecl, irFunc *ir.Function) error {
	// Determine type
	var varType ir.Type
	if v.Type != nil {
		t, err := a.convertType(v.Type)
		if err != nil {
			return fmt.Errorf("invalid type for variable %s: %w", v.Name, err)
		}
		varType = t
	} else if v.Value != nil {
		// Type inference from value
		t, err := a.inferType(v.Value)
		if err != nil {
			return fmt.Errorf("cannot infer type for variable %s: %w", v.Name, err)
		}
		varType = t
	} else {
		return fmt.Errorf("variable %s must have either a type or an initial value", v.Name)
	}

	// Allocate register for variable
	reg := irFunc.AddLocal(v.Name, varType)

	// Register variable in scope
	a.currentScope.Define(v.Name, &VarSymbol{
		Name:      v.Name,
		Type:      varType,
		Reg:       reg,
		IsMutable: v.IsMutable,
	})

	// Generate code for initial value if present
	if v.Value != nil {
		valueReg, err := a.analyzeExpression(v.Value, irFunc)
		if err != nil {
			return err
		}
		// Store value in variable
		irFunc.Emit(ir.OpStoreVar, reg, valueReg, 0)
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

// analyzeExpression analyzes an expression and returns the register containing the result
func (a *Analyzer) analyzeExpression(expr ast.Expression, irFunc *ir.Function) (ir.Register, error) {
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
	default:
		return 0, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// analyzeIdentifier analyzes an identifier
func (a *Analyzer) analyzeIdentifier(id *ast.Identifier, irFunc *ir.Function) (ir.Register, error) {
	sym := a.currentScope.Lookup(id.Name)
	if sym == nil {
		return 0, fmt.Errorf("undefined identifier: %s", id.Name)
	}

	switch s := sym.(type) {
	case *VarSymbol:
		// Load variable value
		reg := irFunc.AllocReg()
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:     ir.OpLoadVar,
			Dest:   reg,
			Symbol: id.Name,
		})
		return reg, nil
	default:
		return 0, fmt.Errorf("cannot use %s as value", id.Name)
	}
}

// analyzeNumberLiteral analyzes a number literal
func (a *Analyzer) analyzeNumberLiteral(num *ast.NumberLiteral, irFunc *ir.Function) (ir.Register, error) {
	reg := irFunc.AllocReg()
	irFunc.EmitImm(ir.OpLoadConst, reg, num.Value)
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

	irFunc.Emit(op, resultReg, leftReg, rightReg)
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
	default:
		return 0, fmt.Errorf("unsupported unary operator: %s", un.Operator)
	}

	return resultReg, nil
}

// analyzeCallExpr analyzes a function call
func (a *Analyzer) analyzeCallExpr(call *ast.CallExpr, irFunc *ir.Function) (ir.Register, error) {
	// For now, only support direct function calls
	id, ok := call.Function.(*ast.Identifier)
	if !ok {
		return 0, fmt.Errorf("indirect function calls not yet supported")
	}

	// Look up function
	sym := a.currentScope.Lookup(id.Name)
	if sym == nil {
		return 0, fmt.Errorf("undefined function: %s", id.Name)
	}

	funcSym, ok := sym.(*FuncSymbol)
	if !ok {
		return 0, fmt.Errorf("%s is not a function", id.Name)
	}

	// Check argument count
	if len(call.Arguments) != len(funcSym.Params) {
		return 0, fmt.Errorf("function %s expects %d arguments, got %d", 
			id.Name, len(funcSym.Params), len(call.Arguments))
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
		Symbol: id.Name,
	})

	return resultReg, nil
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
	default:
		return nil, fmt.Errorf("unsupported type: %T", astType)
	}
}

// inferType infers the type of an expression
func (a *Analyzer) inferType(expr ast.Expression) (ir.Type, error) {
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		// Default to i16 for now
		return &ir.BasicType{Kind: ir.TypeI16}, nil
	case *ast.BooleanLiteral:
		return &ir.BasicType{Kind: ir.TypeBool}, nil
	case *ast.Identifier:
		sym := a.currentScope.Lookup(e.Name)
		if sym == nil {
			return nil, fmt.Errorf("undefined identifier: %s", e.Name)
		}
		if varSym, ok := sym.(*VarSymbol); ok {
			return varSym.Type, nil
		}
		return nil, fmt.Errorf("cannot infer type from %s", e.Name)
	default:
		return nil, fmt.Errorf("cannot infer type from expression of type %T", expr)
	}
}

var labelCounter int

// generateLabel generates a unique label
func (a *Analyzer) generateLabel(prefix string) string {
	labelCounter++
	return fmt.Sprintf("%s_%d", prefix, labelCounter)
}