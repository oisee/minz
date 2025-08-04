package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/tas"
	"github.com/minz/minzc/pkg/z80asm"
)

// REPL represents the MinZ Read-Eval-Print-Loop
type REPL struct {
	assembler *z80asm.Assembler
	emulator  *emulator.Z80WithScreen
	context   *Context
	compiler  *REPLCompiler
	reader    *bufio.Reader
	history   []string
	autoShowScreen bool // Show ZX Spectrum screen after execution
	
	// TAS debugging support
	tasDebugger *tas.TASDebugger
	tasEnabled  bool
	tasUI       *tas.TASUI
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
	Name    string
	Params  []string
	Body    string
	Address uint16
	Size    uint16
	Source  string // MinZ source code
}

// New creates a new REPL instance
func New() *REPL {
	ctx := &Context{
		variables: make(map[string]Variable),
		functions: make(map[string]Function),
		codeBase:  0x8000, // Start of user RAM
		dataBase:  0xC000, // Data segment
	}
	return &REPL{
		assembler:  z80asm.NewAssembler(),
		emulator:   emulator.NewZ80WithScreen(),
		context:    ctx,
		compiler:   NewREPLCompiler(ctx.codeBase, ctx.dataBase),
		reader:     bufio.NewReader(os.Stdin),
		history:    []string{},
		autoShowScreen: false, // Off by default
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
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              mzr - MinZ REPL v1.0                           ║")
	fmt.Println("║         Interactive Z80 Development Environment              ║")
	fmt.Println("║              With ZX Spectrum Screen Emulation              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("Type /h for help, /q to quit, or enter MinZ code")
	fmt.Println()
}

// isCommand checks if input is a REPL command
func (r *REPL) isCommand(input string) bool {
	return strings.HasPrefix(input, "/")
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
	case "/help", "/h", "/?":
		r.showHelp()
	case "/quit", "/q", "/exit":
		r.quit()
	case "/reset":
		r.reset()
	case "/asm":
		if len(args) > 0 {
			r.showAssembly(args[0])
		} else {
			fmt.Println("Usage: /asm <function>")
		}
	case "/reg", "/r":
		if len(args) > 0 && args[0] == "compact" {
			r.showRegistersCompact()
		} else {
			r.showRegisters()
		}
	case "/regc", "/rc":
		r.showRegistersCompact()
	case "/screen", "/s":
		r.showScreen()
	case "/screens", "/ss":
		r.toggleScreen()
	case "/cls", "/clear":
		r.clearScreen()
	case "/vars", "/v":
		r.showVariables()
	case "/funcs", "/f":
		r.showFunctions()
	case "/mem", "/m":
		if len(args) >= 2 {
			r.showMemory(args[0], args[1])
		} else {
			fmt.Println("Usage: /mem <address> <length>")
		}
	case "/save":
		if len(args) > 0 {
			r.saveSession(args[0])
		} else {
			fmt.Println("Usage: /save <filename>")
		}
	case "/load":
		if len(args) > 0 {
			r.loadFile(args[0])
		} else {
			fmt.Println("Usage: /load <filename>")
		}
	
	// TAS debugging commands
	case "/tas":
		r.toggleTAS()
	case "/record":
		r.startTASRecording()
	case "/stop":
		r.stopTASRecording()
	case "/rewind":
		if len(args) > 0 {
			r.tasRewind(args[0])
		} else {
			r.tasRewind("100") // Default 100 frames
		}
	case "/forward":
		if len(args) > 0 {
			r.tasForward(args[0])
		} else {
			r.tasForward("100")
		}
	case "/savestate":
		if len(args) > 0 {
			r.tasSaveState(args[0])
		} else {
			r.tasSaveState("checkpoint")
		}
	case "/loadstate":
		if len(args) > 0 {
			r.tasLoadState(args[0])
		} else {
			fmt.Println("Usage: /loadstate <name>")
		}
	case "/timeline":
		r.showTASTimeline()
	case "/hunt":
		if len(args) > 0 {
			r.startOptimizationHunt(args[0])
		} else {
			fmt.Println("Usage: /hunt <target_address>")
		}
	case "/export":
		if len(args) > 0 {
			r.exportTAS(args[0])
		} else {
			fmt.Println("Usage: /export <filename.tas>")
		}
	case "/import":
		if len(args) > 0 {
			r.importTAS(args[0])
		} else {
			fmt.Println("Usage: /import <filename.tas>")
		}
	case "/replay":
		if len(args) > 0 {
			r.replayTAS(args[0])
		} else {
			fmt.Println("Usage: /replay <filename.tas>")
		}
	case "/strategy":
		if len(args) > 0 {
			r.setTASStrategy(args[0])
		} else {
			fmt.Println("Usage: /strategy <auto|deterministic|snapshot|hybrid|paranoid>")
		}
	case "/stats":
		r.showTASStats()
	case "/profile":
		r.profilePerformance()
	case "/report":
		r.showTASReport()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
	}
}

// evaluate compiles and executes MinZ code
func (r *REPL) evaluate(input string) {
	inputType := ClassifyInput(input)
	var result *CompileResult
	var err error
	
	switch inputType {
	case "expression":
		result, err = r.compiler.CompileExpression(input, r.context)
	case "declaration", "assignment", "statement":
		result, err = r.compiler.CompileStatement(input, r.context)
	case "function":
		result, err = r.compiler.CompileFunction(input, r.context)
	default:
		fmt.Printf("Unknown input type: %s\n", inputType)
		return
	}
	
	if err != nil {
		fmt.Printf("Compilation error: %v\n", err)
		return
	}
	
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %s\n", strings.Join(result.Errors, "; "))
		return
	}
	
	// Load machine code into emulator
	r.emulator.LoadAt(result.EntryPoint, result.MachineCode)
	
	// Execute the code with screen hooks
	output, cycleCount := r.emulator.ExecuteWithHooks(result.EntryPoint)
	
	// If there was output, print it
	if output != "" {
		fmt.Print(output)
	}
	
	// Show execution stats in verbose mode
	if r.isVerbose() {
		fmt.Printf("[%d T-states]\n", cycleCount)
	}
	
	// Show screen if enabled
	if r.autoShowScreen {
		fmt.Println("\n--- ZX Spectrum Screen ---")
		r.emulator.PrintCompactScreen()
		fmt.Println("-------------------------")
	}
	
	// For expressions, show the result (in HL register)
	if inputType == "expression" {
		result := uint16(r.emulator.H)<<8 | uint16(r.emulator.L)
		fmt.Printf("%d\n", result)
	}
	
	// Update context with new functions/variables
	for name, addr := range result.Functions {
		if _, exists := r.context.functions[name]; !exists && !strings.HasPrefix(name, "__repl") {
			r.context.functions[name] = Function{
				Name:    name,
				Address: addr,
				Source:  input,
			}
			if inputType == "function" {
				fmt.Printf("Function '%s' defined at 0x%04X\n", name, addr)
			}
		}
	}
	
	// For declarations, update variables
	if inputType == "declaration" {
		// Extract variable name from input (simple parsing)
		parts := strings.Fields(input)
		if len(parts) >= 2 && (parts[0] == "let" || parts[0] == "var") {
			varName := strings.TrimSuffix(parts[1], ":")
			if idx := strings.Index(varName, ":"); idx > 0 {
				varName = varName[:idx]
			}
			fmt.Printf("Variable '%s' defined\n", varName)
		}
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
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    MinZ REPL Commands                        ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║ /help    /h  /?   - Show this help                          ║")
	fmt.Println("║ /quit    /q       - Exit REPL                               ║")
	fmt.Println("║ /reset            - Reset emulator state                    ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /reg     /r       - Show Z80 registers (with shadows)       ║")
	fmt.Println("║ /regc    /rc      - Compact register view                   ║")
	fmt.Println("║ /mem     /m <a> <n> - Show n bytes at address a             ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /screen  /s       - Show ZX Spectrum screen                 ║")
	fmt.Println("║ /screens /ss      - Toggle auto-show screen                 ║")
	fmt.Println("║ /cls              - Clear ZX Spectrum screen                ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /vars    /v       - Show defined variables                  ║")
	fmt.Println("║ /funcs   /f       - Show defined functions                  ║")
	fmt.Println("║ /asm <func>       - Show assembly for function              ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /save <file>      - Save session to file                    ║")
	fmt.Println("║ /load <file>      - Load MinZ file                          ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ 🎮 TAS Debugging (Time-Travel)                              ║")
	fmt.Println("║ /tas              - Enable/disable TAS debugging            ║")
	fmt.Println("║ /record           - Start recording every CPU cycle         ║")
	fmt.Println("║ /rewind [n]       - Go back n frames (default: 100)         ║")
	fmt.Println("║ /savestate [name] - Create save state                       ║")
	fmt.Println("║ /timeline         - Show execution timeline                 ║")
	fmt.Println("║ /export <file>    - Export recording to .tas file           ║")
	fmt.Println("║ /import <file>    - Import recording from .tas file         ║")
	fmt.Println("║ /replay <file>    - Replay recording from .tas file         ║")
	fmt.Println("║ /strategy <mode>  - Set recording strategy                  ║")
	fmt.Println("║ /stats            - Show TAS statistics                     ║")
	fmt.Println("║ /profile          - Analyze performance from recording      ║")
	fmt.Println("║ /report           - Show comprehensive TAS report           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
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
	r.compiler.Reset()
	fmt.Println("Emulator, compiler and context reset")
}

func (r *REPL) showRegisters() {
	// Get all register values
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Z80 Register State                        ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	
	// Main registers as pairs - all names exactly 3 chars
	fmt.Printf("║ AF =%04X  BC =%04X  DE =%04X  HL =%04X                    ║\n",
		uint16(r.emulator.A)<<8|uint16(r.emulator.F),
		uint16(r.emulator.B)<<8|uint16(r.emulator.C),
		uint16(r.emulator.D)<<8|uint16(r.emulator.E),
		uint16(r.emulator.H)<<8|uint16(r.emulator.L))
	
	// Shadow registers as pairs - all names exactly 3 chars
	fmt.Printf("║ AF'=%04X  BC'=%04X  DE'=%04X  HL'=%04X                    ║\n",
		uint16(r.emulator.A_)<<8|uint16(r.emulator.F_),
		uint16(r.emulator.B_)<<8|uint16(r.emulator.C_),
		uint16(r.emulator.D_)<<8|uint16(r.emulator.E_),
		uint16(r.emulator.H_)<<8|uint16(r.emulator.L_))
	
	// Index and special registers - all names exactly 3 chars
	fmt.Printf("║ IX =%04X  IY =%04X  SP =%04X  PC =%04X                    ║\n",
		r.emulator.IX, r.emulator.IY, r.emulator.SP, r.emulator.PC)
	
	// I and R registers
	fmt.Printf("║ I  =%02X    R  =%02X    IFF1=%v  IFF2=%v  IM=%d            ║\n",
		r.emulator.I, r.emulator.R, 
		r.emulator.GetIFF1(), r.emulator.GetIFF2(), r.emulator.GetIM())
	
	// Flags breakdown
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Flags: S=%d Z=%d H=%d P/V=%d N=%d C=%d                      ║\n",
		boolToInt(r.emulator.F&0x80 != 0), // Sign
		boolToInt(r.emulator.F&0x40 != 0), // Zero
		boolToInt(r.emulator.F&0x10 != 0), // Half-carry
		boolToInt(r.emulator.F&0x04 != 0), // Parity/Overflow
		boolToInt(r.emulator.F&0x02 != 0), // Add/Subtract
		boolToInt(r.emulator.F&0x01 != 0)) // Carry
	
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *REPL) showRegistersCompact() {
	// Compact one-line view
	fmt.Printf("AF=%04X BC=%04X DE=%04X HL=%04X IX=%04X IY=%04X SP=%04X PC=%04X ",
		uint16(r.emulator.A)<<8|uint16(r.emulator.F),
		uint16(r.emulator.B)<<8|uint16(r.emulator.C),
		uint16(r.emulator.D)<<8|uint16(r.emulator.E),
		uint16(r.emulator.H)<<8|uint16(r.emulator.L),
		r.emulator.IX, r.emulator.IY, r.emulator.SP, r.emulator.PC)
	
	// Show shadow registers if any are non-zero
	if r.emulator.A_ != 0 || r.emulator.B_ != 0 || r.emulator.C_ != 0 ||
		r.emulator.D_ != 0 || r.emulator.E_ != 0 || r.emulator.H_ != 0 || r.emulator.L_ != 0 {
		fmt.Printf("(AF'=%04X BC'=%04X DE'=%04X HL'=%04X)",
			uint16(r.emulator.A_)<<8|uint16(r.emulator.F_),
			uint16(r.emulator.B_)<<8|uint16(r.emulator.C_),
			uint16(r.emulator.D_)<<8|uint16(r.emulator.E_),
			uint16(r.emulator.H_)<<8|uint16(r.emulator.L_))
	}
	fmt.Println()
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
			name, params, f.Address, f.Size)
	}
}

func (r *REPL) isVerbose() bool {
	// Could be configurable
	return false
}

// Screen-related functions
func (r *REPL) showScreen() {
	fmt.Println("\n--- ZX Spectrum Screen (32x24) ---")
	fmt.Print(r.emulator.Screen.GetScreen())
	// Cursor position would be shown if we had access
	// fmt.Printf("Cursor at (%d, %d)\n", r.emulator.Screen.CursorX, r.emulator.Screen.CursorY)
}

func (r *REPL) toggleScreen() {
	r.autoShowScreen = !r.autoShowScreen
	if r.autoShowScreen {
		fmt.Println("Auto-show screen: ON")
	} else {
		fmt.Println("Auto-show screen: OFF")
	}
}

func (r *REPL) clearScreen() {
	r.emulator.Screen.Clear()
	fmt.Println("ZX Spectrum screen cleared")
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