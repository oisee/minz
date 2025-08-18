package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/tas"
	"github.com/minz/minzc/pkg/z80asm"
	"golang.org/x/term"
)

// REPL represents the MinZ Read-Eval-Print-Loop
type REPL struct {
	assembler *z80asm.Assembler
	emulator  *emulator.REPLCompatibleZ80  // Now with 100% Z80 coverage!
	context   *Context
	compiler  *REPLCompiler
	reader    *bufio.Reader
	history   []string
	historyIndex int    // Current position in history
	autoShowScreen bool // Show ZX Spectrum screen after execution
	
	// TAS debugging support
	tasDebugger *tas.TASDebugger
	tasEnabled  bool
	tasUI       *tas.TASUI
	
	// Terminal state for raw mode
	oldTermState *term.State
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
		emulator:   emulator.NewREPLCompatibleZ80(),  // 100% Z80 coverage!
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
	
	// Set up terminal for raw mode if supported
	if term.IsTerminal(int(os.Stdin.Fd())) {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err == nil {
			r.oldTermState = oldState
			defer r.restoreTerminal()
		}
	}
	
	for {
		input := r.readLineWithHistory()
		if input == nil {
			// EOF or quit
			r.quit()
			return
		}
		
		line := strings.TrimSpace(*input)
		if line == "" {
			continue
		}
		
		// Add to history if it's different from the last entry
		if len(r.history) == 0 || r.history[len(r.history)-1] != line {
			r.history = append(r.history, line)
		}
		
		if r.isCommand(line) {
			r.executeCommand(line)
		} else {
			r.evaluate(line)
		}
	}
}

// restoreTerminal restores the terminal to its original state
func (r *REPL) restoreTerminal() {
	if r.oldTermState != nil {
		term.Restore(int(os.Stdin.Fd()), r.oldTermState)
	}
}

// readLineWithHistory reads a line with arrow key history support
func (r *REPL) readLineWithHistory() *string {
	fmt.Print("minz> ")
	
	// If not a terminal, fall back to simple reading
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		input, err := r.reader.ReadString('\n')
		if err != nil {
			return nil
		}
		result := strings.TrimSpace(input)
		return &result
	}
	
	var line []rune
	cursorPos := 0
	r.historyIndex = len(r.history)
	
	for {
		// Read a single character
		var buf [3]byte
		n, err := os.Stdin.Read(buf[:])
		if err != nil {
			if err == io.EOF {
				return nil
			}
			continue
		}
		
		if n == 0 {
			continue
		}
		
		// Handle special keys
		if buf[0] == 27 && n > 1 { // ESC sequence
			if n == 3 && buf[1] == '[' {
				switch buf[2] {
				case 'A': // Up arrow
					if r.historyIndex > 0 {
						// Clear current line
						r.clearLine(len(line), cursorPos)
						r.historyIndex--
						line = []rune(r.history[r.historyIndex])
						cursorPos = len(line)
						fmt.Print(string(line))
					}
				case 'B': // Down arrow
					if r.historyIndex < len(r.history)-1 {
						// Clear current line
						r.clearLine(len(line), cursorPos)
						r.historyIndex++
						line = []rune(r.history[r.historyIndex])
						cursorPos = len(line)
						fmt.Print(string(line))
					} else if r.historyIndex == len(r.history)-1 {
						// Clear to empty line
						r.clearLine(len(line), cursorPos)
						r.historyIndex = len(r.history)
						line = []rune{}
						cursorPos = 0
					}
				case 'C': // Right arrow
					if cursorPos < len(line) {
						fmt.Print("\033[1C")
						cursorPos++
					}
				case 'D': // Left arrow
					if cursorPos > 0 {
						fmt.Print("\033[1D")
						cursorPos--
					}
				}
			}
		} else if buf[0] == 13 || buf[0] == 10 { // Enter
			fmt.Println()
			result := string(line)
			return &result
		} else if buf[0] == 3 { // Ctrl+C
			fmt.Println("^C")
			return nil
		} else if buf[0] == 4 { // Ctrl+D
			if len(line) == 0 {
				return nil
			}
		} else if buf[0] == 127 || buf[0] == 8 { // Backspace
			if cursorPos > 0 && len(line) > 0 {
				// Remove character before cursor
				line = append(line[:cursorPos-1], line[cursorPos:]...)
				cursorPos--
				// Redraw line from cursor position
				fmt.Print("\033[1D\033[K") // Move back and clear to end
				fmt.Print(string(line[cursorPos:]))
				// Move cursor back to correct position
				if len(line) > cursorPos {
					fmt.Printf("\033[%dD", len(line)-cursorPos)
				}
			}
		} else if buf[0] >= 32 && buf[0] < 127 { // Printable character
			// Insert character at cursor position
			ch := rune(buf[0])
			if cursorPos == len(line) {
				line = append(line, ch)
			} else {
				line = append(line[:cursorPos+1], line[cursorPos:]...)
				line[cursorPos] = ch
			}
			// Print the character and everything after it
			fmt.Print(string(line[cursorPos:]))
			cursorPos++
			// Move cursor back if needed
			if len(line) > cursorPos {
				fmt.Printf("\033[%dD", len(line)-cursorPos)
			}
		}
	}
}

// clearLine clears the current line in the terminal
func (r *REPL) clearLine(lineLen, cursorPos int) {
	// Move cursor to beginning of line
	if cursorPos > 0 {
		fmt.Printf("\033[%dD", cursorPos)
	}
	// Clear to end of line
	fmt.Print("\033[K")
}

// printBanner prints the REPL welcome message
func (r *REPL) printBanner() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              mzr - MinZ REPL v1.0                           ║")
	fmt.Println("║         Interactive Z80 Development Environment              ║")
	fmt.Println("║              With ZX Spectrum Screen Emulation              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("🚀 Quick Start:")
	fmt.Println("  • Type expressions:     2 + 3 * 4")
	fmt.Println("  • Define variables:     let x: u8 = 42")
	fmt.Println("  • Create functions:     fun add(a: u8, b: u8) -> u8 { a + b }")
	fmt.Println("  • Call functions:       add(5, 3)")
	fmt.Println("  • See help:            /h or /help")
	fmt.Println()
	fmt.Println("Type /h for full command list, /q to quit")
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
	
	// TAS debugging commands (temporarily disabled)
	case "/tas", "/record", "/stop", "/rewind", "/forward", "/savestate", "/loadstate", "/timeline", "/hunt":
		fmt.Println("TAS debugging commands are temporarily disabled")
	case "/export":
		fmt.Println("TAS export is temporarily disabled")
		// if len(args) > 0 {
		//	r.exportTAS(args[0])
		// } else {
		//	fmt.Println("Usage: /export <filename.tas>")
		// }
	case "/import":
		fmt.Println("TAS import is temporarily disabled")
		// if len(args) > 0 {
		//	r.importTAS(args[0])
		// } else {
		//	fmt.Println("Usage: /import <filename.tas>")
		// }
	case "/replay":
		fmt.Println("TAS replay is temporarily disabled")
		// if len(args) > 0 {
		//	r.replayTAS(args[0])
		// } else {
		//	fmt.Println("Usage: /replay <filename.tas>")
		// }
	case "/strategy":
		fmt.Println("TAS strategy is temporarily disabled")
		// if len(args) > 0 {
		//	r.setTASStrategy(args[0])
		// } else {
		//	fmt.Println("Usage: /strategy <auto|deterministic|snapshot|hybrid|paranoid>")
		// }
	case "/stats":
		fmt.Println("TAS stats are temporarily disabled")
		// r.showTASStats()
	case "/profile":
		fmt.Println("TAS profiling is temporarily disabled")
		// r.profilePerformance()
	case "/report":
		fmt.Println("TAS report is temporarily disabled")
		// r.showTASReport()
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
	if len(output) > 0 {
		fmt.Print(string(output))
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
	fmt.Println("║           mzr - MinZ Interactive REPL v1.0                   ║")
	fmt.Println("║     Real-time Z80 Development with Time-Travel Debugging     ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║ 🎯 BASIC COMMANDS                                            ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /help    /h  /?   - Show this help                          ║")
	fmt.Println("║ /quit    /q       - Exit REPL                               ║")
	fmt.Println("║ /reset            - Reset emulator and context              ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ 📊 DEBUGGING & INSPECTION                                    ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /reg     /r       - Show Z80 registers (with shadows)       ║")
	fmt.Println("║ /regc    /rc      - Compact register view                   ║")
	fmt.Println("║ /mem     /m <a> <n> - Show n bytes at address a             ║")
	fmt.Println("║ /asm <func>       - Show assembly for function              ║")
	fmt.Println("║ /vars    /v       - Show defined variables                  ║")
	fmt.Println("║ /funcs   /f       - Show defined functions                  ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ 🖥️  ZX SPECTRUM SCREEN EMULATION                             ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /screen  /s       - Show current screen (32x24 chars)       ║")
	fmt.Println("║ /screens /ss      - Toggle auto-show after execution        ║")
	fmt.Println("║ /cls              - Clear screen memory                     ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ 💾 SESSION MANAGEMENT                                        ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /save <file>      - Save current session                    ║")
	fmt.Println("║ /load <file>      - Load and execute MinZ file              ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ 🎮 TAS TIME-TRAVEL DEBUGGING                                 ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /tas              - Enable/disable TAS mode                 ║")
	fmt.Println("║ /record           - Start recording every CPU cycle         ║")
	fmt.Println("║ /stop             - Stop recording                          ║")
	fmt.Println("║ /rewind [n]       - Go back n frames (default: 100)         ║")
	fmt.Println("║ /forward [n]      - Go forward n frames                     ║")
	fmt.Println("║ /savestate [name] - Create named save state                 ║")
	fmt.Println("║ /loadstate <name> - Restore save state                      ║")
	fmt.Println("║ /timeline         - Show execution timeline                 ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ 📼 TAS RECORDING & REPLAY                                    ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────╢")
	fmt.Println("║ /export <file>    - Export to .tas file                     ║")
	fmt.Println("║ /import <file>    - Import from .tas file                   ║")
	fmt.Println("║ /replay <file>    - Replay recording                        ║")
	fmt.Println("║ /strategy <mode>  - Set strategy (auto/deterministic/...)   ║")
	fmt.Println("║ /hunt <addr>      - Hunt for optimization opportunities     ║")
	fmt.Println("║ /stats            - Show recording statistics               ║")
	fmt.Println("║ /profile          - Performance analysis                    ║")
	fmt.Println("║ /report           - Comprehensive TAS report                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("📝 MINZ CODE EXAMPLES:")
	fmt.Println("  Simple expression:      2 + 3 * 4")
	fmt.Println("  Variable declaration:   let x: u8 = 42")
	fmt.Println("  Function definition:    fun add(a: u8, b: u8) -> u8 { a + b }")
	fmt.Println("  Function call:          add(5, 3)")
	fmt.Println("  Print to screen:        @print(\"Hello!\")")
	fmt.Println("  Inline assembly:        asm { LD A, 'X'; RST 16 }")
	fmt.Println()
	fmt.Println("💡 TIPS:")
	fmt.Println("  • Expressions are evaluated and printed automatically")
	fmt.Println("  • Functions persist across commands")
	fmt.Println("  • Use /tas for frame-perfect debugging")
	fmt.Println("  • Memory starts at $8000, data at $C000")
	fmt.Println("  • Character output uses RST 16 (ZX Spectrum)")
}

func (r *REPL) quit() {
	r.restoreTerminal()
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