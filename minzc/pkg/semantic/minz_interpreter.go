package semantic

import (
	"fmt"
	"os"
	"strings"
	"strconv"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/mirvm"
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
// It executes the MinZ code using MZV (MIR Virtual Machine) and generates declarations
func (a *Analyzer) analyzeMinzBlock(block *ast.MinzBlock) error {
	// Create MZV configuration for compile-time execution
	config := mirvm.Config{
		MemorySize: 65536,
		StackSize:  4096,
		Debug:      minzDebug,
		Trace:      false,
		MaxSteps:   100000, // Prevent infinite loops
		Verbose:    false,
	}
	
	// Create a MinZ interpreter context
	ctx := &minzInterpreterContext{
		analyzer:    a,
		emittedCode: []string{},
		variables:   make(map[string]interface{}),
		vm:          mirvm.New(config), // Use MZV for execution
	}
	
	// Handle raw code if present
	if block.RawCode != "" {
		if minzDebug {
			fmt.Printf("DEBUG: Processing @minz block with raw code: %s\n", block.RawCode)
		}
		
		// For MinzBlock, compile MinZ to MIR then execute
		if err := ctx.executeMinzCode(block.RawCode); err != nil {
			return fmt.Errorf("error executing @minz code: %w", err)
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
				if minzDebug {
					fmt.Printf("DEBUG: Registering generated function: %s\n", d.Name)
				}
				if err := a.registerFunctionSignature(d); err != nil {
					return fmt.Errorf("error registering generated function: %w", err)
				}
				// Also add to the generatedDeclarations for later analysis
				a.generatedDeclarations = append(a.generatedDeclarations, d)
				if minzDebug {
					fmt.Printf("DEBUG: Added function %s to generated declarations\n", d.Name)
				}
			default:
				// For other types, add them to generated declarations if they're Declaration types
				if declTyped, ok := decl.(ast.Declaration); ok {
					a.generatedDeclarations = append(a.generatedDeclarations, declTyped)
				}
			}
		}
	}
	
	return nil
}

// executeMinzCode compiles MinZ code to MIR and executes it in MZV
func (ctx *minzInterpreterContext) executeMinzCode(minzCode string) error {
	if minzDebug {
		fmt.Printf("DEBUG: Compiling and executing @minz code\n")
	}
	
	// For now, fall back to simple pattern matching
	// TODO: Implement full MIR compilation and execution
	return ctx.executeSimplePatterns(minzCode)
}

// executeMirCode parses and executes MIR code directly in MZV
func (ctx *minzInterpreterContext) executeMirCode(mirCode string) error {
	if minzDebug {
		fmt.Printf("DEBUG: Executing @mir code directly\n")
	}
	
	// Parse MIR code into IR
	module, err := ir.ParseMIR(mirCode)
	if err != nil {
		return fmt.Errorf("failed to parse MIR code: %w", err)
	}
	
	// Load module into VM
	if err := ctx.vm.LoadModule(module); err != nil {
		return fmt.Errorf("failed to load MIR module: %w", err)
	}
	
	// Execute the module (looks for main function)
	exitCode, err := ctx.vm.Run()
	if err != nil {
		return fmt.Errorf("MIR execution failed: %w", err)
	}
	
	if exitCode != 0 {
		return fmt.Errorf("MIR execution failed with exit code %d", exitCode)
	}
	
	// Extract emitted code from VM
	ctx.extractEmittedCode()
	
	return nil
}

// executeMirFunction executes a single MIR function
func (ctx *minzInterpreterContext) executeMirFunction(fn *ir.Function) error {
	// Create a temporary module with just this function
	module := &ir.Module{
		Name:      "@minz_temp",
		Functions: []*ir.Function{fn},
	}
	
	// Rename function to main for execution
	fn.Name = "main"
	
	// Load and execute
	if err := ctx.vm.LoadModule(module); err != nil {
		return fmt.Errorf("failed to load MIR function: %w", err)
	}
	
	exitCode, err := ctx.vm.Run()
	if err != nil {
		return fmt.Errorf("MIR function execution failed: %w", err)
	}
	
	if exitCode != 0 {
		return fmt.Errorf("MIR function execution failed with exit code %d", exitCode)
	}
	
	// Extract emitted code
	ctx.extractEmittedCode()
	
	return nil
}

// extractEmittedCode extracts @emit output from the VM
func (ctx *minzInterpreterContext) extractEmittedCode() {
	// Get emitted code from VM
	emitted := ctx.vm.GetEmittedCode()
	
	if minzDebug {
		fmt.Printf("DEBUG: Extracted %d lines of emitted code from VM\n", len(emitted))
	}
	
	// Add to our emitted code buffer
	ctx.emittedCode = append(ctx.emittedCode, emitted...)
	
	// Clear VM buffer for next execution
	ctx.vm.ClearEmittedCode()
}

// minzInterpreterContext holds the state for interpreting MinZ code at compile time
type minzInterpreterContext struct {
	analyzer    *Analyzer
	emittedCode []string
	variables   map[string]interface{} // Local variables in the @minz block
	vm          *mirvm.VM             // MIR Virtual Machine for compile-time execution
}

// parseAndExecuteRawCode parses and executes raw MinZ code from @minz[[[]]] blocks
// This is the legacy fallback method
func (ctx *minzInterpreterContext) parseAndExecuteRawCode(rawCode string) error {
	// Try the new MZV execution first
	if err := ctx.executeMinzCode(rawCode); err == nil {
		return nil
	}
	
	// Fall back to simple pattern matching for compatibility
	if minzDebug {
		fmt.Printf("DEBUG: Falling back to pattern matching for @minz block execution\n")
	}
	return ctx.executeSimplePatterns(rawCode)
}

// executeSimplePatterns handles simple pattern matching for when full parsing fails
func (ctx *minzInterpreterContext) executeSimplePatterns(rawCode string) error {
	if minzDebug {
		fmt.Printf("DEBUG: Using simple pattern matching for @minz block\n")
	}
	
	// Check if this contains a for loop - if so, use the for loop processor
	if minzDebug {
		fmt.Printf("DEBUG: Raw code to analyze:\n%q\n", rawCode)
		fmt.Printf("DEBUG: Contains 'for ': %v\n", strings.Contains(rawCode, "for "))
		fmt.Printf("DEBUG: Contains ' in ': %v\n", strings.Contains(rawCode, " in "))
		fmt.Printf("DEBUG: Contains '..': %v\n", strings.Contains(rawCode, ".."))
	}
	
	if strings.Contains(rawCode, "for ") && strings.Contains(rawCode, " in ") && strings.Contains(rawCode, "..") {
		if minzDebug {
			fmt.Printf("DEBUG: Detected for loop, using executeSimpleForLoop\n")
		}
		return ctx.executeSimpleForLoop(rawCode)
	}
	
	// Process all lines in the raw code
	lines := strings.Split(rawCode, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "//") {
			// Execute each line
			if err := ctx.executeSimpleLine(line); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// executeSimpleForLoop handles simple for loops with @emit patterns
func (ctx *minzInterpreterContext) executeSimpleForLoop(rawCode string) error {
	lines := strings.Split(rawCode, "\n")
	
	// Look for "for i in start..end {" pattern
	var inLoop bool
	var loopVar string
	var start, end int
	var loopBody []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "for ") && strings.Contains(line, " in ") && strings.Contains(line, "..") && strings.HasSuffix(line, " {") {
			// Parse: "for i in 0..4 {"
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				loopVar = parts[1]
				rangePart := parts[3] // "0..4"
				if strings.Contains(rangePart, "..") {
					rangeParts := strings.Split(rangePart, "..")
					if len(rangeParts) == 2 {
						startStr := strings.TrimSpace(rangeParts[0])
						endStr := strings.TrimSpace(rangeParts[1])
						
						var err error
						start, err = ctx.parseIntOrVariable(startStr)
						if err != nil {
							return fmt.Errorf("invalid range start: %s", startStr)
						}
						end, err = ctx.parseIntOrVariable(endStr)
						if err != nil {
							return fmt.Errorf("invalid range end: %s", endStr)
						}
						
						inLoop = true
						loopBody = []string{}
						
						if minzDebug {
							fmt.Printf("DEBUG: Found for loop: %s in %d..%d\n", loopVar, start, end)
						}
					}
				}
			}
		} else if inLoop && line == "}" {
			// End of loop - execute the body
			if minzDebug {
				fmt.Printf("DEBUG: Executing loop body with %d lines\n", len(loopBody))
			}
			
			for i := start; i < end; i++ {
				// Set the loop variable
				if ctx.variables == nil {
					ctx.variables = make(map[string]interface{})
				}
				ctx.variables[loopVar] = i
				
				// Execute each line in the loop body
				for _, bodyLine := range loopBody {
					if err := ctx.executeSimpleLine(bodyLine); err != nil {
						return err
					}
				}
			}
			
			// Clean up
			inLoop = false
			delete(ctx.variables, loopVar)
		} else if inLoop {
			// Inside loop body
			loopBody = append(loopBody, line)
		} else {
			// Outside loop - execute directly
			if err := ctx.executeSimpleLine(line); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// executeSimpleLine executes a simple line of code (mainly @emit calls)
func (ctx *minzInterpreterContext) executeSimpleLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") {
		return nil
	}
	
	// Handle for loops specially
	if strings.HasPrefix(line, "for ") && strings.Contains(line, " in ") && strings.Contains(line, "..") {
		// This is the start of a for loop - need to handle it specially
		// For now, return nil and let the executeSimpleForLoop handle it
		return nil
	}
	
	// Handle @emit("...") calls (with or without semicolon)
	if strings.HasPrefix(line, "@emit(") && (strings.HasSuffix(line, ");") || strings.HasSuffix(line, ")")) {
		// Extract the string content
		var content string
		if strings.HasSuffix(line, ");") {
			content = line[6 : len(line)-2] // Remove @emit( and );
		} else {
			content = line[6 : len(line)-1] // Remove @emit( and )
		}
		
		// Handle quoted strings properly, including escaped quotes
		if strings.HasPrefix(content, "\"") && strings.HasSuffix(content, "\"") {
			content = content[1 : len(content)-1] // Remove outer quotes
			// Unescape the content
			content = strings.ReplaceAll(content, "\\\"", "\"")
			content = strings.ReplaceAll(content, "\\n", "\n")
			content = strings.ReplaceAll(content, "\\t", "\t")
			content = strings.ReplaceAll(content, "\\\\", "\\")
		}
		
		// Process template variables
		content = ctx.processTemplateString(content)
		ctx.emittedCode = append(ctx.emittedCode, content)
		
		if minzDebug {
			fmt.Printf("DEBUG: Emitted code line: %s\n", content)
		}
		return nil
	}
	
	// Handle @save_binary("filename", data) calls
	if strings.HasPrefix(line, "@save_binary(") && strings.HasSuffix(line, ");") {
		// Simple pattern matching - extract arguments
		args := line[13 : len(line)-2] // Remove @save_binary( and );
		// Split on comma (simple version)
		parts := strings.SplitN(args, ",", 2)
		if len(parts) == 2 {
			filename := strings.TrimSpace(parts[0])
			filename = strings.Trim(filename, "\"")
			filename = ctx.processTemplateString(filename)
			
			dataExpr := strings.TrimSpace(parts[1])
			// For now, handle variable references
			if ctx.variables != nil {
				if data, ok := ctx.variables[dataExpr]; ok {
					// Convert to bytes
					if arr, ok := data.([]interface{}); ok {
						bytes := make([]byte, len(arr))
						for i, val := range arr {
							if intVal, ok := toInt(val); ok && intVal >= 0 && intVal <= 255 {
								bytes[i] = byte(intVal)
							}
						}
						os.WriteFile(filename, bytes, 0644)
						if minzDebug {
							fmt.Printf("DEBUG: Saved %d bytes to %s\n", len(bytes), filename)
						}
					}
				}
			}
		}
		return nil
	}
	
	// Handle @incbin("filename") calls
	if strings.HasPrefix(line, "@incbin(") && strings.HasSuffix(line, ");") {
		// Extract filename
		filename := line[8 : len(line)-2] // Remove @incbin( and );
		filename = strings.Trim(filename, "\"")
		filename = ctx.processTemplateString(filename)
		
		// Generate INCBIN directive
		ctx.emittedCode = append(ctx.emittedCode, fmt.Sprintf("    INCBIN \"%s\"", filename))
		
		if minzDebug {
			fmt.Printf("DEBUG: Generated INCBIN for %s\n", filename)
		}
		return nil
	}
	
	// Handle variable assignments like: data[i] = value
	if strings.Contains(line, "[") && strings.Contains(line, "]") && strings.Contains(line, "=") {
		// Parse array assignment
		parts := strings.Split(line, "=")
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			right = strings.TrimSuffix(right, ";")
			
			// Extract array name and index
			if idx := strings.Index(left, "["); idx > 0 {
				arrayName := strings.TrimSpace(left[:idx])
				indexExpr := left[idx+1 : strings.Index(left, "]")]
				
				// Evaluate index
				indexVal := ctx.evaluateSimpleExpression(indexExpr)
				index, _ := strconv.Atoi(indexVal)
				
				// Evaluate value
				valueStr := ctx.evaluateSimpleExpression(right)
				value, _ := strconv.Atoi(valueStr)
				
				// Update array
				if ctx.variables != nil {
					if arr, ok := ctx.variables[arrayName].([]interface{}); ok {
						if index >= 0 && index < len(arr) {
							arr[index] = value
						}
					}
				}
			}
		}
	}
	
	// Handle variable declaration with array literal: let data = [1, 2, 3];
	if strings.HasPrefix(line, "let ") && strings.Contains(line, "=") {
		parts := strings.Split(line[4:], "=")
		if len(parts) == 2 {
			varName := strings.TrimSpace(parts[0])
			valueExpr := strings.TrimSpace(parts[1])
			valueExpr = strings.TrimSuffix(valueExpr, ";")
			
			// Check if it's an array literal
			if strings.HasPrefix(valueExpr, "[") && strings.HasSuffix(valueExpr, "]") {
				// Parse array literal
				arrayContent := valueExpr[1 : len(valueExpr)-1]
				elements := strings.Split(arrayContent, ",")
				
				arr := make([]interface{}, len(elements))
				for i, elem := range elements {
					elem = strings.TrimSpace(elem)
					if val, err := strconv.Atoi(elem); err == nil {
						arr[i] = val
					} else {
						// Try evaluating as expression
						arr[i] = ctx.evaluateSimpleExpression(elem)
					}
				}
				
				if ctx.variables == nil {
					ctx.variables = make(map[string]interface{})
				}
				ctx.variables[varName] = arr
				
				if minzDebug {
					fmt.Printf("DEBUG: Created array %s with %d elements\n", varName, len(arr))
				}
			}
		}
	}
	
	return nil
}

// parseIntOrVariable parses an integer literal or looks up a variable value
func (ctx *minzInterpreterContext) parseIntOrVariable(s string) (int, error) {
	// Try parsing as integer literal first
	if val, err := strconv.Atoi(s); err == nil {
		return val, nil
	}
	
	// Try looking up as variable
	if ctx.variables != nil {
		if val, ok := ctx.variables[s]; ok {
			if intVal, ok := toInt(val); ok {
				return intVal, nil
			} else {
				return 0, fmt.Errorf("variable %s is not an integer", s)
			}
		}
	}
	
	return 0, fmt.Errorf("unknown identifier: %s", s)
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
		// Check if it's a metafunction call like @save_binary
		if call, ok := s.Expression.(*ast.MetafunctionCall); ok {
			return ctx.executeMetafunction(call)
		}
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
	
	// Look for {expression} patterns and evaluate them
	// This is a simple implementation - more complex expressions can be added
	for {
		start := strings.Index(result, "{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		
		expr := result[start+1 : end]
		
		// Evaluate the expression
		value := ctx.evaluateSimpleExpression(expr)
		
		// Replace the placeholder
		placeholder := result[start : end+1]
		result = strings.ReplaceAll(result, placeholder, value)
	}
	
	return result
}

// evaluateSimpleExpression evaluates simple expressions like "i", "i * i", "size"
func (ctx *minzInterpreterContext) evaluateSimpleExpression(expr string) string {
	expr = strings.TrimSpace(expr)
	
	// Handle simple variable reference
	if ctx.variables != nil {
		if val, ok := ctx.variables[expr]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	
	// Handle simple arithmetic: "var * var" or "var + var"
	if strings.Contains(expr, " * ") {
		parts := strings.Split(expr, " * ")
		if len(parts) == 2 {
			leftVar := strings.TrimSpace(parts[0])
			rightVar := strings.TrimSpace(parts[1])
			
			if ctx.variables != nil {
				if leftVal, ok := ctx.variables[leftVar]; ok {
					if rightVal, ok := ctx.variables[rightVar]; ok {
						if leftInt, ok := toInt(leftVal); ok {
							if rightInt, ok := toInt(rightVal); ok {
								return fmt.Sprintf("%d", leftInt*rightInt)
							}
						}
					}
				}
			}
		}
	}
	
	if strings.Contains(expr, " + ") {
		parts := strings.Split(expr, " + ")
		if len(parts) == 2 {
			leftVar := strings.TrimSpace(parts[0])
			rightVar := strings.TrimSpace(parts[1])
			
			if ctx.variables != nil {
				if leftVal, ok := ctx.variables[leftVar]; ok {
					if rightVal, ok := ctx.variables[rightVar]; ok {
						if leftInt, ok := toInt(leftVal); ok {
							if rightInt, ok := toInt(rightVal); ok {
								return fmt.Sprintf("%d", leftInt+rightInt)
							}
						}
					}
				}
			}
		}
	}
	
	// If we can't evaluate, return as-is
	return expr
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

// executeMetafunction executes compile-time metafunctions like @save_binary
func (ctx *minzInterpreterContext) executeMetafunction(call *ast.MetafunctionCall) error {
	switch call.Name {
	case "save_binary":
		return ctx.executeSaveBinary(call)
	case "incbin":
		return ctx.executeIncBin(call)
	default:
		return fmt.Errorf("unknown metafunction: @%s", call.Name)
	}
}

// executeSaveBinary saves binary data to a file
func (ctx *minzInterpreterContext) executeSaveBinary(call *ast.MetafunctionCall) error {
	if len(call.Arguments) != 2 {
		return fmt.Errorf("@save_binary requires 2 arguments: filename and data")
	}
	
	// Evaluate filename
	filenameVal, err := ctx.evaluateExpression(call.Arguments[0])
	if err != nil {
		return fmt.Errorf("error evaluating filename: %w", err)
	}
	filename, ok := filenameVal.(string)
	if !ok {
		return fmt.Errorf("filename must be a string, got %T", filenameVal)
	}
	
	// Evaluate data
	dataVal, err := ctx.evaluateExpression(call.Arguments[1])
	if err != nil {
		return fmt.Errorf("error evaluating data: %w", err)
	}
	
	// Convert data to bytes
	var bytes []byte
	switch data := dataVal.(type) {
	case []interface{}:
		// Array of values
		bytes = make([]byte, len(data))
		for i, val := range data {
			if intVal, ok := toInt(val); ok {
				if intVal < 0 || intVal > 255 {
					return fmt.Errorf("array element %d is out of byte range: %d", i, intVal)
				}
				bytes[i] = byte(intVal)
			} else {
				return fmt.Errorf("array element %d is not a number: %T", i, val)
			}
		}
	case string:
		// String data
		bytes = []byte(data)
	default:
		return fmt.Errorf("data must be an array or string, got %T", dataVal)
	}
	
	// Save to file
	if err := os.WriteFile(filename, bytes, 0644); err != nil {
		return fmt.Errorf("error writing file %s: %w", filename, err)
	}
	
	if minzDebug {
		fmt.Printf("DEBUG: Saved %d bytes to %s\n", len(bytes), filename)
	}
	
	return nil
}

// executeIncBin generates an INCBIN directive
func (ctx *minzInterpreterContext) executeIncBin(call *ast.MetafunctionCall) error {
	if len(call.Arguments) != 1 {
		return fmt.Errorf("@incbin requires 1 argument: filename")
	}
	
	// Evaluate filename
	filenameVal, err := ctx.evaluateExpression(call.Arguments[0])
	if err != nil {
		return fmt.Errorf("error evaluating filename: %w", err)
	}
	filename, ok := filenameVal.(string)
	if !ok {
		return fmt.Errorf("filename must be a string, got %T", filenameVal)
	}
	
	// Generate INCBIN directive
	ctx.emittedCode = append(ctx.emittedCode, fmt.Sprintf("    INCBIN \"%s\"", filename))
	
	if minzDebug {
		fmt.Printf("DEBUG: Generated INCBIN for %s\n", filename)
	}
	
	return nil
}