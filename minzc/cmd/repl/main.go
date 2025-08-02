package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// REPL represents the MinZ Read-Eval-Print-Loop
type REPL struct {
	assembler *z80asm.Assembler
	emulator  *emulator.Z80
	context   *Context
	reader    *bufio.Reader
	history   []string
}

// Context maintains REPL state between commands
type Context struct {
	variables map[string]Variable
	functions map[string]Function
	codeBase  uint16 // Where to place next code
	dataBase  uint16 // Where to place next data
}

type Variable struct {
	Name  string
	Type  string
	Value interface{}
	Addr  uint16
}

type Function struct {
	Name   string
	Params []string
	Body   string
	Addr   uint16
	Size   uint16
}

// New creates a new REPL instance
func New() *REPL {
	return &REPL{
		assembler: z80asm.NewAssembler(),
		emulator:  emulator.New(),
		context: &Context{
			variables: make(map[string]Variable),
			functions: make(map[string]Function),
			codeBase:  0x8000, // Start of user RAM
			dataBase:  0xC000, // Data segment
		},
		reader:  bufio.NewReader(os.Stdin),
		history: []string{},
	}
}

// Run starts the REPL main loop
func (r *REPL) Run() {
	r.printBanner()
	
	for {
		fmt.Print("minz> ")
		input, err := r.reader.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" {
				r.quit()
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}
		
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		
		r.history = append(r.history, input)
		
		if r.isCommand(input) {
			r.executeCommand(input)
		} else {
			r.evaluate(input)
		}
	}
}

// printBanner prints the REPL welcome message
func (r *REPL) printBanner() {
	fmt.Println("MinZ REPL v1.0.0 - Interactive Z80 Development")
	fmt.Println("Type :help for commands, Ctrl-D to exit")
	fmt.Println()
}

// isCommand checks if input is a REPL command
func (r *REPL) isCommand(input string) bool {
	return strings.HasPrefix(input, ":")
}

// executeCommand handles REPL commands
func (r *REPL) executeCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	
	cmd := parts[0]
	args := parts[1:]
	
	switch cmd {
	case ":help":
		r.showHelp()
	case ":quit", ":exit":
		r.quit()
	case ":reset":
		r.reset()
	case ":asm":
		if len(args) > 0 {
			r.showAssembly(args[0])
		} else {
			fmt.Println("Usage: :asm <function>")
		}
	case ":mem":
		if len(args) >= 2 {
			r.showMemory(args[0], args[1])
		} else {
			fmt.Println("Usage: :mem <address> <length>")
		}
	case ":reg":
		r.showRegisters()
	case ":vars":
		r.showVariables()
	case ":funcs":
		r.showFunctions()
	case ":profile":
		if len(args) > 0 {
			r.profile(strings.Join(args, " "))
		} else {
			fmt.Println("Usage: :profile <expression>")
		}
	case ":save":
		if len(args) > 0 {
			r.saveSession(args[0])
		} else {
			fmt.Println("Usage: :save <filename>")
		}
	case ":load":
		if len(args) > 0 {
			r.loadFile(args[0])
		} else {
			fmt.Println("Usage: :load <filename>")
		}
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type :help for available commands")
	}
}

// evaluate compiles and executes MinZ code
func (r *REPL) evaluate(input string) {
	// Wrap input in context if needed
	wrapped := r.wrapInput(input)
	
	// Compile the code
	assembly, err := r.compiler.CompileString(wrapped)
	if err != nil {
		fmt.Printf("Compilation error: %v\n", err)
		return
	}
	
	// Assemble to machine code
	machineCode, err := r.assemble(assembly)
	if err != nil {
		fmt.Printf("Assembly error: %v\n", err)
		return
	}
	
	// Load into emulator
	r.emulator.LoadAt(r.context.codeBase, machineCode)
	
	// Execute
	output, cycles := r.emulator.Execute(r.context.codeBase)
	
	// Update context
	r.updateContext(input)
	
	// Display output
	if output != "" {
		fmt.Print(output)
	}
	
	// Show execution stats in verbose mode
	if r.isVerbose() {
		fmt.Printf("[%d T-states]\n", cycles)
	}
}

// wrapInput wraps user input with necessary context
func (r *REPL) wrapInput(input string) string {
	// Check if it's a complete statement
	if strings.HasPrefix(input, "fun ") {
		// Function definition
		return input
	} else if strings.HasPrefix(input, "let ") || strings.HasPrefix(input, "var ") {
		// Variable declaration
		return fmt.Sprintf("fun __repl_eval() -> void { %s }", input)
	} else if strings.Contains(input, "=") && !strings.Contains(input, "==") {
		// Assignment
		return fmt.Sprintf("fun __repl_eval() -> void { %s }", input)
	} else {
		// Expression - evaluate and print
		return fmt.Sprintf("fun __repl_eval() -> void { let __result = %s; @print(\"%%d\\n\", __result); }", input)
	}
}

// Helper functions

func (r *REPL) showHelp() {
	fmt.Println("REPL Commands:")
	fmt.Println("  :help          - Show this help")
	fmt.Println("  :quit          - Exit REPL")
	fmt.Println("  :reset         - Reset emulator state")
	fmt.Println("  :asm <func>    - Show assembly for function")
	fmt.Println("  :mem <addr> <n> - Show n bytes of memory at addr")
	fmt.Println("  :reg           - Show Z80 registers")
	fmt.Println("  :vars          - Show defined variables")
	fmt.Println("  :funcs         - Show defined functions")
	fmt.Println("  :profile <expr> - Profile expression execution")
	fmt.Println("  :save <file>   - Save session to file")
	fmt.Println("  :load <file>   - Load MinZ file")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  let x: u8 = 42")
	fmt.Println("  fun add(a: u8, b: u8) -> u8 { return a + b; }")
	fmt.Println("  add(5, 3)")
}

func (r *REPL) quit() {
	fmt.Println("Goodbye! Happy coding!")
	os.Exit(0)
}

func (r *REPL) reset() {
	r.emulator.Reset()
	r.context = &Context{
		variables: make(map[string]Variable),
		functions: make(map[string]Function),
		codeBase:  0x8000,
		dataBase:  0xC000,
	}
	fmt.Println("Emulator and context reset")
}

func (r *REPL) showRegisters() {
	regs := r.emulator.GetRegisters()
	fmt.Printf("A=%02X  BC=%04X  DE=%04X  HL=%04X\n", 
		regs.A, regs.BC, regs.DE, regs.HL)
	fmt.Printf("F=%02X  IX=%04X  IY=%04X  SP=%04X  PC=%04X\n",
		regs.F, regs.IX, regs.IY, regs.SP, regs.PC)
}

func (r *REPL) showVariables() {
	if len(r.context.variables) == 0 {
		fmt.Println("No variables defined")
		return
	}
	
	fmt.Println("Variables:")
	for name, v := range r.context.variables {
		fmt.Printf("  %s: %s = %v (at 0x%04X)\n", 
			name, v.Type, v.Value, v.Addr)
	}
}

func (r *REPL) showFunctions() {
	if len(r.context.functions) == 0 {
		fmt.Println("No functions defined")
		return
	}
	
	fmt.Println("Functions:")
	for name, f := range r.context.functions {
		params := strings.Join(f.Params, ", ")
		fmt.Printf("  %s(%s) at 0x%04X (%d bytes)\n",
			name, params, f.Addr, f.Size)
	}
}

func (r *REPL) isVerbose() bool {
	// Could be configurable
	return false
}

// Placeholder functions - need implementation

func (r *REPL) assemble(assembly string) ([]byte, error) {
	result, err := r.assembler.AssembleString(assembly)
	if err != nil {
		return nil, fmt.Errorf("assembly failed: %v", err)
	}
	return result.Binary, nil
}

func (r *REPL) updateContext(input string) {
	// TODO: Parse input and update context
}

func (r *REPL) showAssembly(function string) {
	// TODO: Show disassembly of function
	fmt.Printf("Assembly for %s not available\n", function)
}

func (r *REPL) showMemory(addr, length string) {
	// TODO: Display memory contents
	fmt.Printf("Memory dump from %s for %s bytes\n", addr, length)
}

func (r *REPL) profile(expr string) {
	// TODO: Profile expression execution
	fmt.Printf("Profiling: %s\n", expr)
}

func (r *REPL) saveSession(filename string) {
	// TODO: Save session to file
	fmt.Printf("Session saved to %s\n", filename)
}

func (r *REPL) loadFile(filename string) {
	// TODO: Load and execute MinZ file
	fmt.Printf("Loading %s\n", filename)
}

func main() {
	repl := New()
	repl.Run()
}