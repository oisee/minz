package main

import (
	"fmt"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/optimizer"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/semantic"
	"github.com/minz/minzc/pkg/z80asm"
)

// CompileResult holds the result of compilation
type CompileResult struct {
	MachineCode []byte
	EntryPoint  uint16
	DataSize    uint16
	Functions   map[string]uint16 // Function name -> address
	Variables   map[string]uint16 // Variable name -> address
	Errors      []string
}

// REPLCompiler handles compilation for REPL
type REPLCompiler struct {
	codeBase uint16
	dataBase uint16
	nextCode uint16
	nextData uint16
	tempDir  string
}

// NewREPLCompiler creates a new REPL compiler
func NewREPLCompiler(codeBase, dataBase uint16) *REPLCompiler {
	tempDir, _ := ioutil.TempDir("", "minz-repl-*")
	return &REPLCompiler{
		codeBase: codeBase,
		dataBase: dataBase,
		nextCode: codeBase,
		nextData: dataBase,
		tempDir:  tempDir,
	}
}

// CompileExpression compiles a MinZ expression and returns machine code
func (c *REPLCompiler) CompileExpression(expr string, ctx *Context) (*CompileResult, error) {
	// Wrap expression in a function for evaluation
	source := c.wrapExpression(expr, ctx)
	return c.compile(source, ctx)
}

// CompileStatement compiles a MinZ statement
func (c *REPLCompiler) CompileStatement(stmt string, ctx *Context) (*CompileResult, error) {
	// Wrap statement appropriately
	source := c.wrapStatement(stmt, ctx)
	return c.compile(source, ctx)
}

// CompileFunction compiles a function definition
func (c *REPLCompiler) CompileFunction(funcDef string, ctx *Context) (*CompileResult, error) {
	// Functions can be compiled directly with a wrapper module
	source := c.wrapFunction(funcDef, ctx)
	return c.compile(source, ctx)
}

// wrapExpression wraps an expression for evaluation
func (c *REPLCompiler) wrapExpression(expr string, ctx *Context) string {
	var sb strings.Builder
	
	// Add context variables as globals
	for name, v := range ctx.variables {
		sb.WriteString(fmt.Sprintf("global %s: %s = %d;\n", name, v.Type, v.Value))
	}
	
	// Add context functions
	for _, f := range ctx.functions {
		sb.WriteString(f.Source)
		sb.WriteString("\n")
	}
	
	// Create evaluation function
	sb.WriteString("fun __repl_eval() -> u16 {\n")
	sb.WriteString(fmt.Sprintf("    let __result = %s;\n", expr))
	sb.WriteString("    return __result;\n")
	sb.WriteString("}\n")
	
	return sb.String()
}

// wrapStatement wraps a statement for execution
func (c *REPLCompiler) wrapStatement(stmt string, ctx *Context) string {
	var sb strings.Builder
	
	// Add context
	for name, v := range ctx.variables {
		sb.WriteString(fmt.Sprintf("global %s: %s = %d;\n", name, v.Type, v.Value))
	}
	
	for _, f := range ctx.functions {
		sb.WriteString(f.Source)
		sb.WriteString("\n")
	}
	
	// Wrap in function
	sb.WriteString("fun __repl_exec() -> void {\n")
	sb.WriteString("    " + stmt + "\n")
	sb.WriteString("}\n")
	
	return sb.String()
}

// wrapFunction wraps a function definition
func (c *REPLCompiler) wrapFunction(funcDef string, ctx *Context) string {
	var sb strings.Builder
	
	// Add existing context
	for name, v := range ctx.variables {
		sb.WriteString(fmt.Sprintf("global %s: %s = %d;\n", name, v.Type, v.Value))
	}
	
	for _, f := range ctx.functions {
		sb.WriteString(f.Source)
		sb.WriteString("\n")
	}
	
	// Add new function
	sb.WriteString(funcDef)
	sb.WriteString("\n")
	
	return sb.String()
}

// compile performs the actual compilation
func (c *REPLCompiler) compile(source string, ctx *Context) (*CompileResult, error) {
	result := &CompileResult{
		Functions: make(map[string]uint16),
		Variables: make(map[string]uint16),
		Errors:    []string{},
	}
	
	// Write source to temp file
	sourceFile := filepath.Join(c.tempDir, "repl_input.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		return nil, fmt.Errorf("failed to write source: %w", err)
	}
	
	// Parse the source
	p := parser.New()
	astFile, err := p.ParseFile(sourceFile)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("parse error: %v", err))
		return result, err
	}
	
	// Semantic analysis
	analyzer := semantic.NewAnalyzer()
	irModule, err := analyzer.Analyze(astFile)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("semantic error: %v", err))
		return result, err
	}
	defer analyzer.Close()
	
	// Optimization (basic for REPL)
	opt := optimizer.NewOptimizer(optimizer.OptLevelBasic)
	opt.Optimize(irModule)
	
	// Code generation
	var asmBuf bytes.Buffer
	codeGen := codegen.NewZ80Generator(&asmBuf)
	if err := codeGen.Generate(irModule); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("codegen error: %v", err))
		return result, err
	}
	assembly := asmBuf.String()
	
	// Write assembly to temp file
	asmFile := filepath.Join(c.tempDir, "repl_output.a80")
	if err := ioutil.WriteFile(asmFile, []byte(assembly), 0644); err != nil {
		return nil, fmt.Errorf("failed to write assembly: %w", err)
	}
	
	// Assemble to machine code using the built-in assembler
	assembler := z80asm.NewAssembler()
	asmResult, err := assembler.AssembleString(assembly)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("assembly error: %v", err))
		return result, err
	}
	
	machineCode := asmResult.Binary
	
	result.MachineCode = machineCode
	result.EntryPoint = asmResult.Origin
	
	// Update next code position
	c.nextCode += uint16(len(machineCode))
	
	// Extract function addresses from assembly symbols
	for name, addr := range asmResult.Symbols {
		result.Functions[name] = addr
	}
	
	// Extract function addresses from IR as fallback
	for _, fn := range irModule.Functions {
		if _, exists := result.Functions[fn.Name]; !exists {
			result.Functions[fn.Name] = c.nextCode
		}
	}
	
	// TODO: Extract variable addresses from IR globals when available
	
	return result, nil
}

// Reset resets the compiler state
func (c *REPLCompiler) Reset() {
	c.nextCode = c.codeBase
	c.nextData = c.dataBase
}

// Cleanup removes temporary files
func (c *REPLCompiler) Cleanup() {
	os.RemoveAll(c.tempDir)
}

// Helper to determine if input is an expression, statement, or function
func ClassifyInput(input string) string {
	input = strings.TrimSpace(input)
	
	if strings.HasPrefix(input, "fun ") || strings.HasPrefix(input, "fn ") {
		return "function"
	}
	
	if strings.HasPrefix(input, "let ") || strings.HasPrefix(input, "var ") ||
		strings.HasPrefix(input, "global ") || strings.HasPrefix(input, "const ") {
		return "declaration"
	}
	
	if strings.Contains(input, "=") && !strings.Contains(input, "==") &&
		!strings.Contains(input, "!=") && !strings.Contains(input, "<=") &&
		!strings.Contains(input, ">=") {
		return "assignment"
	}
	
	if strings.HasPrefix(input, "if ") || strings.HasPrefix(input, "while ") ||
		strings.HasPrefix(input, "for ") || strings.HasPrefix(input, "return ") {
		return "statement"
	}
	
	// Default to expression
	return "expression"
}