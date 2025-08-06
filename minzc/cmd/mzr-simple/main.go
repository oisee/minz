package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// SimpleREPL - A minimal Z80-focused REPL
type SimpleREPL struct {
	reader  *bufio.Reader
	backend string
}

func main() {
	fmt.Println("🚀 MinZ REPL v0.10.0 \"Lambda Revolution\" - Z80 Interactive")
	fmt.Println("🎊 Simplified REPL focusing on Z80 backend")
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: The full REPL has issues with AST imports.")
	fmt.Println("This is a simplified version that demonstrates the concept.")
	fmt.Println()
	fmt.Println("The main issues with the full REPL:")
	fmt.Println("1. AST import cycle - 'ast.File is not a type' despite it existing")
	fmt.Println("2. Iterator parsing crash - transformSExpToIteratorChain nil pointer")
	fmt.Println("3. Missing backend constructors for LLVM/WASM")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  :backend <name>  - Switch backend (z80, c, llvm, wasm)")
	fmt.Println("  :backends        - List backends")
	fmt.Println("  :compile <code>  - Simulate compilation")
	fmt.Println("  :mir <code>      - Show placeholder MIR")
	fmt.Println("  :help            - Show help")
	fmt.Println("  :exit            - Exit REPL")
	fmt.Println()

	repl := &SimpleREPL{
		reader:  bufio.NewReader(os.Stdin),
		backend: "z80",
	}

	for {
		fmt.Printf("minz[%s]> ", repl.backend)
		input, err := repl.reader.ReadString('\n')
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
			fmt.Println("🎊 Thanks for exploring MinZ!")
			return

		case ":help":
			fmt.Println("Available commands:")
			fmt.Println("  :backend <name> - Switch backend")
			fmt.Println("  :backends       - List all backends")
			fmt.Println("  :compile <code> - Compile MinZ code")
			fmt.Println("  :mir <code>     - Show MIR")
			fmt.Println("  :exit           - Exit REPL")

		case ":backends":
			fmt.Println("Available backends:")
			fmt.Println("  z80  - ✅ Fully implemented")
			fmt.Println("  c    - 🚧 Stub (placeholder output)")
			fmt.Println("  llvm - 🚧 Stub (placeholder output)")
			fmt.Println("  wasm - 🚧 Stub (placeholder output)")
			fmt.Printf("\nCurrent: %s\n", repl.backend)

		case ":backend":
			if len(parts) < 2 {
				fmt.Printf("Current backend: %s\n", repl.backend)
				fmt.Println("Usage: :backend <name>")
				continue
			}
			switch parts[1] {
			case "z80", "c", "llvm", "wasm":
				repl.backend = parts[1]
				fmt.Printf("Switched to %s backend\n", repl.backend)
				if repl.backend != "z80" {
					fmt.Println("⚠️  Note: Only Z80 is fully implemented")
				}
			default:
				fmt.Printf("Unknown backend: %s\n", parts[1])
			}

		case ":compile":
			if len(parts) < 2 {
				fmt.Println("Usage: :compile <code>")
				continue
			}
			code := strings.Join(parts[1:], " ")
			repl.compile(code)

		case ":mir":
			if len(parts) < 2 {
				fmt.Println("Usage: :mir <code>")
				continue
			}
			code := strings.Join(parts[1:], " ")
			repl.showMIR(code)

		default:
			// Treat as MinZ code
			repl.compile(input)
		}
	}
}

func (r *SimpleREPL) compile(code string) {
	fmt.Printf("🔨 Compiling with %s backend: %s\n", r.backend, code)
	
	switch r.backend {
	case "z80":
		fmt.Println("✅ Z80 Assembly Generated:")
		fmt.Println("    ; MinZ Code: " + code)
		fmt.Println("    LD A, 42")
		fmt.Println("    RET")
		fmt.Println("\n💡 In full REPL, this would:")
		fmt.Println("   - Parse code via tree-sitter")
		fmt.Println("   - Perform semantic analysis")
		fmt.Println("   - Generate MIR")
		fmt.Println("   - Optimize")
		fmt.Println("   - Generate Z80 assembly")

	case "c":
		fmt.Println("⚠️  C Backend (Stub):")
		fmt.Println("// C code generation not implemented")
		fmt.Printf("// Original: %s\n", code)
		fmt.Println("int main() { return 42; }")

	case "llvm":
		fmt.Println("⚠️  LLVM Backend (Stub):")
		fmt.Println("; LLVM IR generation not implemented")
		fmt.Printf("; Original: %s\n", code)
		fmt.Println("define i32 @main() {")
		fmt.Println("  ret i32 42")
		fmt.Println("}")

	case "wasm":
		fmt.Println("⚠️  WASM Backend (Stub):")
		fmt.Println("(module")
		fmt.Println("  (func $main (result i32)")
		fmt.Println("    i32.const 42)")
		fmt.Println("  (export \"main\" (func $main)))")
	}
}

func (r *SimpleREPL) showMIR(code string) {
	fmt.Printf("🔍 MIR for: %s\n", code)
	fmt.Println("📋 MIR Instructions (placeholder):")
	fmt.Println("  0: load_const r1, #42")
	fmt.Println("  1: store_var \"x\", r1")
	fmt.Println("  2: return r1")
	fmt.Println("\n💡 In full REPL, MIR would show:")
	fmt.Println("   - Register allocation")
	fmt.Println("   - Instruction selection")
	fmt.Println("   - Optimization opportunities")
}