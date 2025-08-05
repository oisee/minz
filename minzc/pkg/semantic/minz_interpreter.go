package semantic

import (
	"fmt"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/parser"
)

var minzDebug = os.Getenv("DEBUG") != ""

// processTemplateString processes a template string with variable interpolation
func processTemplateString(template string, variables map[string]interface{}) string {
	// For now, simple implementation without full template processing
	// TODO: Implement proper template variable substitution
	return template
}

// analyzeMinzBlock processes a @minz[[[]]] compile-time execution block
// It interprets the MinZ code and generates declarations
func (a *Analyzer) analyzeMinzBlock(block *ast.MinzBlock) error {
	// Create a MinZ interpreter context
	ctx := &minzInterpreterContext{
		analyzer:    a,
		emittedCode: []string{},
		variables:   make(map[string]interface{}),
	}
	
	// Handle raw code if present
	if block.RawCode != "" {
		if minzDebug {
			fmt.Printf("DEBUG: Processing @minz block with raw code: %s\n", block.RawCode)
		}
		// For now, just parse and execute as MinZ code
		// TODO: Implement proper MinZ interpreter for compile-time execution
		// For simple cases like @emit, we can do pattern matching
		if strings.Contains(block.RawCode, "@emit") {
			// Extract @emit content with simple regex
			// This is a temporary solution until full interpreter is ready
			lines := strings.Split(block.RawCode, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "@emit(") && strings.HasSuffix(line, ")") {
					// Extract the string content
					content := line[6 : len(line)-1] // Remove @emit( and )
					// Remove quotes if present
					content = strings.Trim(content, "\"")
					// Process template variables
					content = processTemplateString(content, ctx.variables)
					ctx.emittedCode = append(ctx.emittedCode, content)
				}
			}
		}
	} else {
		// Handle structured code
		if minzDebug {
			fmt.Printf("DEBUG: Processing @minz block with %d statements\n", len(block.Code))
		}
		
		// Execute each statement in the block
		for _, stmt := range block.Code {
			if minzDebug {
				fmt.Printf("DEBUG: Executing statement type: %T\n", stmt)
			}
			if err := ctx.executeStatement(stmt); err != nil {
				return fmt.Errorf("error executing @minz block: %w", err)
			}
		}
	}
	
	// Parse and analyze the emitted code
	if len(ctx.emittedCode) > 0 {
		// Combine all emitted code
		code := strings.Join(ctx.emittedCode, "\n")
		if minzDebug {
			fmt.Printf("DEBUG: Emitted code:\n%s\n", code)
		}
		
		// Parse the generated code
		p := parser.New()
		declarations, err := p.ParseString(code, "@minz block")
		if err != nil {
			return fmt.Errorf("error parsing generated code: %w", err)
		}
		
		// Add generated declarations to the analyzer
		for _, decl := range declarations {
			// Process the generated declaration immediately
			switch d := decl.(type) {
			case *ast.ConstDecl:
				if minzDebug {
					fmt.Printf("DEBUG: Analyzing generated const: %s\n", d.Name)
				}
				if err := a.analyzeConstDecl(d); err != nil {
					return fmt.Errorf("error analyzing generated const: %w", err)
				}
				if minzDebug {
					fmt.Printf("DEBUG: Successfully registered const %s\n", d.Name)
				}
			case *ast.VarDecl:
				if err := a.analyzeVarDecl(d); err != nil {
					return fmt.Errorf("error analyzing generated var: %w", err)
				}
			case *ast.FunctionDecl:
				if err := a.registerFunctionSignature(d); err != nil {
					return fmt.Errorf("error registering generated function: %w", err)
				}
			default:
				// For other types, we'll handle them in the regular passes
				// Just note that they exist
			}
		}
	}
	
	return nil
}

// minzInterpreterContext holds the state for interpreting MinZ code at compile time
type minzInterpreterContext struct {
	analyzer    *Analyzer
	emittedCode []string
	variables   map[string]interface{} // Local variables in the @minz block
}

// executeStatement executes a statement in the MinZ interpreter
func (ctx *minzInterpreterContext) executeStatement(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.MinzEmit:
		return ctx.executeEmit(s)
	case *ast.ForStmt:
		return ctx.executeForLoop(s)
	case *ast.VarDecl:
		return ctx.executeVarDecl(s)
	case *ast.ExpressionStmt:
		// Execute the expression (might have side effects)
		_, err := ctx.evaluateExpression(s.Expression)
		return err
	default:
		return fmt.Errorf("unsupported statement type in @minz block: %T", stmt)
	}
}

// executeEmit processes an @emit statement
func (ctx *minzInterpreterContext) executeEmit(emit *ast.MinzEmit) error {
	// Evaluate the expression to get the code string
	value, err := ctx.evaluateExpression(emit.Code)
	if err != nil {
		return fmt.Errorf("error evaluating @emit expression: %w", err)
	}
	
	// Convert to string
	codeStr, ok := value.(string)
	if !ok {
		return fmt.Errorf("@emit expression must evaluate to a string, got %T", value)
	}
	
	// Perform template substitution if needed
	codeStr = ctx.substituteTemplateVars(codeStr)
	
	// Add to emitted code
	ctx.emittedCode = append(ctx.emittedCode, codeStr)
	
	return nil
}

// executeForLoop executes a for loop in the MinZ interpreter
func (ctx *minzInterpreterContext) executeForLoop(forStmt *ast.ForStmt) error {
	// For now, we only support simple range loops
	rangeExpr, ok := forStmt.Range.(*ast.BinaryExpr)
	if !ok || rangeExpr.Operator != ".." {
		return fmt.Errorf("@minz blocks only support simple range loops (for i in start..end)")
	}
	
	// Evaluate start and end
	startVal, err := ctx.evaluateExpression(rangeExpr.Left)
	if err != nil {
		return fmt.Errorf("error evaluating range start: %w", err)
	}
	endVal, err := ctx.evaluateExpression(rangeExpr.Right)
	if err != nil {
		return fmt.Errorf("error evaluating range end: %w", err)
	}
	
	// Convert to integers
	start, ok := toInt(startVal)
	if !ok {
		return fmt.Errorf("range start must be an integer")
	}
	end, ok := toInt(endVal)
	if !ok {
		return fmt.Errorf("range end must be an integer")
	}
	
	// Initialize loop variable context
	if ctx.variables == nil {
		ctx.variables = make(map[string]interface{})
	}
	
	// Execute loop
	for i := start; i < end; i++ {
		// Set loop variable
		ctx.variables[forStmt.Iterator] = i
		
		// Execute loop body
		for _, stmt := range forStmt.Body.Statements {
			if err := ctx.executeStatement(stmt); err != nil {
				return err
			}
		}
	}
	
	// Remove loop variable from context
	delete(ctx.variables, forStmt.Iterator)
	
	return nil
}

// executeVarDecl executes a variable declaration in the MinZ interpreter
func (ctx *minzInterpreterContext) executeVarDecl(varDecl *ast.VarDecl) error {
	if ctx.variables == nil {
		ctx.variables = make(map[string]interface{})
	}
	
	// Evaluate the initial value if present
	if varDecl.Value != nil {
		value, err := ctx.evaluateExpression(varDecl.Value)
		if err != nil {
			return fmt.Errorf("error evaluating variable initialization: %w", err)
		}
		ctx.variables[varDecl.Name] = value
	} else {
		ctx.variables[varDecl.Name] = nil
	}
	
	return nil
}

// evaluateExpression evaluates an expression in the MinZ interpreter
func (ctx *minzInterpreterContext) evaluateExpression(expr ast.Expression) (interface{}, error) {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		// Process template strings with {expr} syntax
		return ctx.processTemplateString(e.Value), nil
		
	case *ast.NumberLiteral:
		return e.Value, nil
		
	case *ast.Identifier:
		// Look up variable
		if ctx.variables != nil {
			if val, ok := ctx.variables[e.Name]; ok {
				return val, nil
			}
		}
		return nil, fmt.Errorf("undefined variable: %s", e.Name)
		
	case *ast.BinaryExpr:
		return ctx.evaluateBinaryExpr(e)
		
	default:
		return nil, fmt.Errorf("unsupported expression type in @minz block: %T", expr)
	}
}

// evaluateBinaryExpr evaluates a binary expression
func (ctx *minzInterpreterContext) evaluateBinaryExpr(expr *ast.BinaryExpr) (interface{}, error) {
	left, err := ctx.evaluateExpression(expr.Left)
	if err != nil {
		return nil, err
	}
	right, err := ctx.evaluateExpression(expr.Right)
	if err != nil {
		return nil, err
	}
	
	switch expr.Operator {
	case "+":
		// Handle both numeric addition and string concatenation
		if lStr, lOk := left.(string); lOk {
			if rStr, rOk := right.(string); rOk {
				return lStr + rStr, nil
			}
		}
		if lInt, lOk := toInt(left); lOk {
			if rInt, rOk := toInt(right); rOk {
				return lInt + rInt, nil
			}
		}
		
	case "-":
		if lInt, lOk := toInt(left); lOk {
			if rInt, rOk := toInt(right); rOk {
				return lInt - rInt, nil
			}
		}
		
	case "*":
		if lInt, lOk := toInt(left); lOk {
			if rInt, rOk := toInt(right); rOk {
				return lInt * rInt, nil
			}
		}
		
	case "/":
		if lInt, lOk := toInt(left); lOk {
			if rInt, rOk := toInt(right); rOk {
				if rInt == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				return lInt / rInt, nil
			}
		}
		
	case "..":
		// Range operator - just return as is for for loops
		return expr, nil
	}
	
	return nil, fmt.Errorf("unsupported binary operation: %s", expr.Operator)
}

// processTemplateString processes a template string with {expr} substitutions
func (ctx *minzInterpreterContext) processTemplateString(template string) string {
	result := template
	
	// Simple substitution for now - look for {varname} patterns
	for name, value := range ctx.variables {
		placeholder := "{" + name + "}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		
		// Also handle expressions like {i * i}
		// For now, we'll do simple arithmetic in placeholders
		exprPlaceholder := fmt.Sprintf("{%s * %s}", name, name)
		if strings.Contains(result, exprPlaceholder) {
			if intVal, ok := toInt(value); ok {
				result = strings.ReplaceAll(result, exprPlaceholder, fmt.Sprintf("%d", intVal*intVal))
			}
		}
	}
	
	return result
}

// substituteTemplateVars substitutes template variables in a string
func (ctx *minzInterpreterContext) substituteTemplateVars(s string) string {
	// This is called after initial template processing to ensure all vars are substituted
	return ctx.processTemplateString(s)
}

// toInt converts a value to an integer
func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case int32:
		return int(val), true
	case int16:
		return int(val), true
	case int8:
		return int(val), true
	case uint:
		return int(val), true
	case uint64:
		return int(val), true
	case uint32:
		return int(val), true
	case uint16:
		return int(val), true
	case uint8:
		return int(val), true
	default:
		return 0, false
	}
}