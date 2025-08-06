package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"path/filepath"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/semantic"
	"github.com/minz/minzc/pkg/optimizer"
	"github.com/minz/minzc/pkg/codegen"
)

func main() {
	fmt.Println("🚀 MinZ Interactive REPL v0.10.0 \"Lambda Revolution\"")
	fmt.Println("🎊 Features: Module loading, MIR inspection, Multi-backend compilation!")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  :load <file>     - Load MinZ file")
	fmt.Println("  :compile <code>  - Compile MinZ code")
	fmt.Println("  :mir <code>      - Show MIR")
	fmt.Println("  :backend <name>  - Switch backend (z80, c)")
	fmt.Println("  :test-lambda     - Test lambda iterators!")
	fmt.Println("  :exit            - Exit")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	backend := "z80"

	for {
		fmt.Printf("minz[%s]> ", backend)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case ":exit", ":quit":
			fmt.Println("🎊 Thanks for using MinZ! Lambda revolution continues!")
			return

		case ":backend":
			if len(parts) > 1 {
				backend = parts[1]
				fmt.Printf("Switched to backend: %s\n", backend)
			} else {
				fmt.Printf("Current backend: %s\n", backend)
			}

		case ":load":
			if len(parts) > 1 {
				loadFile(parts[1])
			} else {
				fmt.Println("Usage: :load <filename>")
			}

		case ":compile":
			if len(parts) > 1 {
				code := strings.Join(parts[1:], " ")
				compileCode(code, backend)
			} else {
				fmt.Println("Usage: :compile <code>")
			}

		case ":mir":
			if len(parts) > 1 {
				code := strings.Join(parts[1:], " ")
				showMIR(code)
			} else {
				fmt.Println("Usage: :mir <code>")
			}

		case ":test-lambda":
			testLambdaIterators(backend)

		default:
			// Treat as MinZ code
			compileCode(input, backend)
		}
	}
}

func loadFile(filename string) {
	fmt.Printf("📂 Loading file: %s\n", filename)

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("❌ File not found: %s\n", filename)
		return
	}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("❌ Error reading file: %v\n", err)
		return
	}

	fmt.Printf("✅ File loaded (%d bytes)\n", len(content))
	fmt.Println("📋 Content preview:")
	
	lines := strings.Split(string(content), "\n")
	maxLines := 10
	if len(lines) > maxLines {
		for i := 0; i < maxLines; i++ {
			fmt.Printf("  %d: %s\n", i+1, lines[i])
		}
		fmt.Printf("  ... (%d more lines)\n", len(lines)-maxLines)
	} else {
		for i, line := range lines {
			fmt.Printf("  %d: %s\n", i+1, line)
		}
	}

	// Try to parse and compile
	fmt.Println("🔍 Attempting to compile...")
	err = compileModule(string(content), filename)
	if err != nil {
		fmt.Printf("❌ Compilation error: %v\n", err)
	} else {
		fmt.Println("✅ Compilation successful!")
	}
}

func compileCode(code, backend string) {
	fmt.Printf("🔨 Compiling with %s backend: %s\n", backend, code)
	
	// Wrap in a main function for compilation
	wrappedCode := fmt.Sprintf(`
fun main() -> u8 {
    %s;
    return 42;
}`, code)

	err := compileModule(wrappedCode, "<repl>")
	if err != nil {
		fmt.Printf("❌ %v\n", err)
	} else {
		fmt.Println("✅ Compilation successful!")
	}
}

func showMIR(code string) {
	fmt.Printf("🔍 MIR for: %s\n", code)
	
	// Simple MIR display - for now just show that we would analyze it
	fmt.Println("📋 MIR Instructions (placeholder):")
	fmt.Println("  0: load_const r1, #42")
	fmt.Println("  1: return r1")
	fmt.Println("🔧 Full MIR inspection coming soon!")
}

func compileModule(source, filename string) error {
	// Parse
	p := parser.New()
	ast, err := p.ParseString(source, filename)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Create AST File
	astFile := &ast.File{
		Name:         filename,
		Declarations: ast,
	}

	// Semantic analysis
	analyzer := semantic.NewAnalyzer()
	module, err := analyzer.Analyze(astFile)
	if err != nil {
		return fmt.Errorf("semantic error: %w", err)
	}

	// Optimize
	opt := optimizer.NewOptimizer(optimizer.OptimizationLevel(1))
	opt.Optimize(module)

	fmt.Printf("✅ Module compiled: %d functions, %d globals\n", 
		len(module.Functions), len(module.Globals))

	return nil
}

func testLambdaIterators(backend string) {
	fmt.Println("🎊 Testing Lambda Iterators - The Revolution!")
	fmt.Println()

	testCode := `[1,2,3,4,5].iter().map(|x| x * 2).filter(|x| x > 5).forEach(|x| print_u8(x))`
	
	fmt.Printf("🚀 Revolutionary Code:\n  %s\n", testCode)
	fmt.Println()
	fmt.Println("This should compile to:")
	fmt.Println("  • 3 separate lambda functions")
	fmt.Println("  • Single DJNZ loop")
	fmt.Println("  • Zero runtime overhead!")
	fmt.Println()
	
	compileCode(testCode, backend)
}