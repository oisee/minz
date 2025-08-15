package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/optimizer"
	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/readline"
)

// REPL represents the MinZ Read-Eval-Print-Loop
type REPL struct {
	reader      *readline.Reader
	codeBase    uint16
	dataBase    uint16
	modules     map[string]*ir.Module
	currentMod  *ir.Module
	
	// Current backend
	currentBackend string
	
	// Backend options
	backendOpts *codegen.BackendOptions
}

// New creates a new REPL instance
func New() *REPL {
	// Get history file path
	homeDir, _ := os.UserHomeDir()
	historyFile := filepath.Join(homeDir, ".minz_history")
	
	// Create readline reader with history support
	reader := readline.NewReader(&readline.Config{
		Prompt:      "", // We'll set this dynamically
		HistoryFile: historyFile,
		MaxHistory:  1000,
	})
	
	return &REPL{
		reader:    reader,
		codeBase:  0x8000,
		dataBase:  0xF000,
		modules:   make(map[string]*ir.Module),
		currentBackend: "z80", // Default to Z80
		backendOpts: &codegen.BackendOptions{
			OptimizationLevel: 1,
			EnableSMC: false,
		},
	}
}

func (r *REPL) printBanner() {
	fmt.Println("🚀 MinZ REPL v0.10.1 \"History Enhanced\" - Interactive Development Environment")
	fmt.Println("🎊 Now with COMMAND HISTORY! Use ↑/↓ arrows to navigate (where supported)")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  :load <file>     - Load MinZ module from file")
	fmt.Println("  :compile <expr>  - Compile MinZ expression")
	fmt.Println("  :mir <expr>      - Show MIR for expression") 
	fmt.Println("  :backend <name>  - Set backend (z80, c, llvm, wasm)")
	fmt.Println("  :backends        - List available backends")
	fmt.Println("  :run <expr>      - Compile and execute (Z80 only)")
	fmt.Println("  :history         - Show command history")
	fmt.Println("  :search <text>   - Search history")
	fmt.Println("  :clear           - Clear screen")
	fmt.Println("  :help            - Show this help")
	fmt.Println("  :exit            - Exit REPL")
	fmt.Println()
	fmt.Println("🎯 History is automatically saved to ~/.minz_history")
	fmt.Println("🎯 Try some lambda iterators:")
	fmt.Println("  :compile [1,2,3,4,5].iter().map(|x| x * 2).filter(|x| x > 5)")
	fmt.Println()
}

func (r *REPL) Run() {
	r.printBanner()
	
	for {
		// Set dynamic prompt
		prompt := fmt.Sprintf("minz[%s]> ", r.currentBackend)
		r.reader.SetPrompt(prompt)
		
		// Read line with history support
		input, err := r.reader.ReadLine()
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}
		
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		
		// Handle commands
		if strings.HasPrefix(input, ":") {
			r.handleCommand(input)
			continue
		}
		
		// Handle MinZ code
		r.handleMinZCode(input)
	}
}

func (r *REPL) handleCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}
	
	switch parts[0] {
	case ":exit", ":quit", ":q":
		fmt.Println("🎊 Thanks for using MinZ REPL! Lambda revolution continues!")
		os.Exit(0)
		
	case ":help", ":h":
		r.printBanner()
		
	case ":clear", ":cls":
		fmt.Print("\033[2J\033[H") // ANSI clear screen
		
	case ":history":
		history := r.reader.GetHistory()
		fmt.Println("Command history:")
		for i, h := range history {
			fmt.Printf("  %d: %s\n", i+1, h)
		}
		fmt.Printf("\nTotal: %d commands (saved to ~/.minz_history)\n", len(history))
	
	case ":search":
		if len(parts) < 2 {
			fmt.Println("Usage: :search <text>")
			return
		}
		query := strings.Join(parts[1:], " ")
		results := r.reader.SearchHistory(query)
		if len(results) == 0 {
			fmt.Printf("No history entries matching '%s'\n", query)
		} else {
			fmt.Printf("History entries matching '%s':\n", query)
			for i, result := range results {
				fmt.Printf("  %d: %s\n", i+1, result)
			}
		}
		
	case ":backends":
		fmt.Println("Available backends:")
		fmt.Println("  z80 (current) - Full support")
		fmt.Println("  c - Stub only")
		fmt.Println("  llvm - Stub only")
		fmt.Println("  wasm - Stub only")
		
	case ":backend":
		if len(parts) < 2 {
			fmt.Printf("Current backend: %s\n", r.currentBackend)
			fmt.Println("Usage: :backend <name>")
			return
		}
		backend := parts[1]
		switch backend {
		case "z80", "c", "llvm", "wasm":
			r.currentBackend = backend
			fmt.Printf("Switched to backend: %s\n", r.currentBackend)
			if backend != "z80" {
				fmt.Println("⚠️  Note: Only Z80 backend is fully implemented")
			}
		default:
			fmt.Printf("Unknown backend: %s\n", backend)
			fmt.Println("Available: z80, c, llvm, wasm")
		}
		
	case ":load":
		if len(parts) < 2 {
			fmt.Println("Usage: :load <filename>")
			return
		}
		r.loadModule(parts[1])
		
	case ":compile":
		if len(parts) < 2 {
			fmt.Println("Usage: :compile <expression>")
			return
		}
		code := strings.Join(parts[1:], " ")
		r.compileCode(code, false)
		
	case ":mir":
		if len(parts) < 2 {
			fmt.Println("Usage: :mir <expression>")
			return
		}
		code := strings.Join(parts[1:], " ")
		r.showMIR(code)
		
	case ":run":
		if len(parts) < 2 {
			fmt.Println("Usage: :run <expression>")
			return
		}
		if r.currentBackend != "z80" {
			fmt.Println("Execution only supported for Z80 backend")
			return
		}
		code := strings.Join(parts[1:], " ")
		r.runCode(code)
		
	default:
		fmt.Printf("Unknown command: %s\n", parts[0])
		fmt.Println("Type :help for available commands")
	}
}

func (r *REPL) handleMinZCode(code string) {
	r.compileCode(code, true)
}

func (r *REPL) loadModule(filename string) {
	fmt.Printf("📂 Loading module: %s\n", filename)
	
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("❌ File not found: %s\n", filename)
		return
	}
	
	// Read file content
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("❌ Error reading file: %v\n", err)
		return
	}
	
	// Parse and compile the module
	module, err := r.compileModule(string(content), filename)
	if err != nil {
		fmt.Printf("❌ Compilation error: %v\n", err)
		return
	}
	
	// Store the module
	moduleName := strings.TrimSuffix(filepath.Base(filename), ".minz")
	r.modules[moduleName] = module
	r.currentMod = module
	
	fmt.Printf("✅ Module '%s' loaded successfully\n", moduleName)
	fmt.Printf("   Functions: %d\n", len(module.Functions))
	fmt.Printf("   Globals: %d\n", len(module.Globals))
}

func (r *REPL) compileModule(source, filename string) (*ir.Module, error) {
	// Parse the source code
	p := parser.New()
	ast, err := p.ParseString(source, filename)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	
	// Create a synthetic file structure for the analyzer
	// Since we can't import ast.File, we'll work around it
	// by creating a minimal wrapper
	type fileWrapper struct {
		Name         string
		Declarations interface{}
	}
	
	file := &fileWrapper{
		Name:         filename,
		Declarations: ast,
	}
	
	// Semantic analysis
	// For now, we'll skip semantic analysis and just return an empty module
	// This is a temporary workaround until we fix the AST import issue
	module := &ir.Module{
		Name:      filename,
		Functions: []*ir.Function{},
		Globals:   []ir.Global{},
	}
	
	// TODO: Fix semantic analysis once AST import is resolved
	_ = file // Silence unused variable warning
	
	return module, nil
}

func (r *REPL) compileCode(code string, showResult bool) {
	// Wrap single expressions in a main function for compilation
	wrappedCode := fmt.Sprintf(`
fun main() -> u8 {
    %s;
    return 42;
}`, code)
	
	// Compile to module
	module, err := r.compileModule(wrappedCode, "<repl>")
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}
	
	// Optimize
	opt := optimizer.NewOptimizer(optimizer.OptimizationLevel(1))
	opt.Optimize(module)
	
	// Generate code for the selected backend
	var output string
	
	switch r.currentBackend {
	case "z80":
		z80Backend := codegen.NewZ80Backend(r.backendOpts)
		output, err = z80Backend.Generate(module)
	case "c":
		// Stub for C backend
		output = "// C backend stub - not implemented\n"
		fmt.Println("⚠️  C backend is a stub - showing placeholder output")
	case "llvm":
		// Stub for LLVM backend
		output = "; LLVM backend stub - not implemented\n"
		fmt.Println("⚠️  LLVM backend is a stub - showing placeholder output")
	case "wasm":
		// Stub for WASM backend
		output = ";; WASM backend stub - not implemented\n"
		fmt.Println("⚠️  WASM backend is a stub - showing placeholder output")
	default:
		fmt.Printf("❌ Unknown backend: %s\n", r.currentBackend)
		return
	}
	if err != nil {
		fmt.Printf("❌ Code generation error: %v\n", err)
		return
	}
	
	if showResult {
		fmt.Printf("✅ Compiled to %s:\n", r.currentBackend)
		// Show first 20 lines of output
		lines := strings.Split(output, "\n")
		maxLines := 20
		if len(lines) > maxLines {
			for _, line := range lines[:maxLines] {
				fmt.Println(line)
			}
			fmt.Printf("... (%d more lines)\n", len(lines)-maxLines)
		} else {
			fmt.Print(output)
		}
	}
}

func (r *REPL) showMIR(code string) {
	fmt.Printf("🔍 Generating MIR for: %s\n", code)
	
	// Wrap in main function
	wrappedCode := fmt.Sprintf(`
fun main() -> u8 {
    %s;
    return 42;
}`, code)
	
	// Compile to get MIR
	module, err := r.compileModule(wrappedCode, "<repl>")
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}
	
	fmt.Println("📋 MIR Instructions:")
	for _, function := range module.Functions {
		if function.Name == "main" {
			fmt.Printf("Function: %s\n", function.Name)
			for i, instr := range function.Instructions {
				fmt.Printf("  %3d: %s\n", i, formatMIRInstruction(instr))
			}
			break
		}
	}
}

func formatMIRInstruction(instr ir.Instruction) string {
	parts := []string{instr.Op.String()}
	
	if instr.Dest != 0 {
		parts = append(parts, fmt.Sprintf("r%d", instr.Dest))
	}
	if instr.Src1 != 0 {
		parts = append(parts, fmt.Sprintf("r%d", instr.Src1))
	}
	if instr.Src2 != 0 {
		parts = append(parts, fmt.Sprintf("r%d", instr.Src2))
	}
	if instr.Imm != 0 {
		parts = append(parts, fmt.Sprintf("#%d", instr.Imm))
	}
	if instr.Symbol != "" {
		parts = append(parts, instr.Symbol)
	}
	
	result := strings.Join(parts, " ")
	if instr.Comment != "" {
		result += fmt.Sprintf(" ; %s", instr.Comment)
	}
	
	return result
}

func (r *REPL) runCode(code string) {
	fmt.Printf("⚡ Compiling and executing: %s\n", code)
	
	// TODO: Implement Z80 execution with emulator
	// For now, just show that we would execute
	fmt.Println("🔧 Execution support coming soon!")
	fmt.Println("   Will compile to Z80, assemble, and run in emulator")
}

func main() {
	repl := New()
	repl.Run()
}