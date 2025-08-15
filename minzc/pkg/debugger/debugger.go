// Package debugger provides interactive debugging for the Z80 emulator
package debugger

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
)

// Debugger provides interactive debugging capabilities
type Debugger struct {
	emu          *emulator.Z80
	breakpoints  map[uint16]bool
	watchpoints  map[uint16]WatchType
	stepMode     bool
	running      bool
	lastPC       uint16
	history      []HistoryEntry
	maxHistory   int
	
	// UI components
	input        *bufio.Scanner
	output       io.Writer
	
	// Display options
	showRegs     bool
	showMem      bool
	showStack    bool
	showDisasm   bool
	memAddr      uint16
	disasmAddr   uint16
	
	// Statistics
	cycleCount   uint64
	instrCount   uint64
}

// WatchType defines the type of watchpoint
type WatchType int

const (
	WatchRead WatchType = iota
	WatchWrite
	WatchReadWrite
)

// HistoryEntry records a single instruction execution
type HistoryEntry struct {
	PC          uint16
	Instruction string
	Registers   RegisterSnapshot
	Cycles      int
}

// RegisterSnapshot captures register state at a point in time
type RegisterSnapshot struct {
	A, F byte
	B, C byte
	D, E byte
	H, L byte
	SP   uint16
	IX   uint16
	IY   uint16
}

// Config holds debugger configuration
type Config struct {
	MaxHistory int
	Input      io.Reader
	Output     io.Writer
}

// New creates a new debugger instance
func New(emu *emulator.Z80, config *Config) *Debugger {
	if config == nil {
		config = &Config{}
	}
	if config.MaxHistory == 0 {
		config.MaxHistory = 100
	}
	if config.Input == nil {
		config.Input = os.Stdin
	}
	if config.Output == nil {
		config.Output = os.Stdout
	}
	
	return &Debugger{
		emu:         emu,
		breakpoints: make(map[uint16]bool),
		watchpoints: make(map[uint16]WatchType),
		maxHistory:  config.MaxHistory,
		input:       bufio.NewScanner(config.Input),
		output:      config.Output,
		showRegs:    true,
		showDisasm:  true,
		disasmAddr:  0x8000,
	}
}

// Run starts the interactive debugger
func (d *Debugger) Run() error {
	d.printBanner()
	d.display()
	
	for {
		// Check for breakpoint
		if d.checkBreakpoint() {
			fmt.Fprintf(d.output, "\n🔴 Breakpoint hit at $%04X\n", d.emu.GetPC())
			d.stepMode = true
		}
		
		// Check for watchpoint
		if addr, wtype := d.checkWatchpoint(); addr != 0xFFFF {
			fmt.Fprintf(d.output, "\n👁️ Watchpoint hit at $%04X (%s)\n", addr, watchTypeString(wtype))
			d.stepMode = true
		}
		
		// Execute instruction if running
		if !d.stepMode {
			cycles := d.executeInstruction()
			d.cycleCount += uint64(cycles)
			d.instrCount++
			continue
		}
		
		// Interactive mode
		fmt.Fprint(d.output, "dbg> ")
		if !d.input.Scan() {
			break
		}
		
		cmd := strings.TrimSpace(d.input.Text())
		if cmd == "" {
			cmd = "s" // Default to step
		}
		
		if err := d.handleCommand(cmd); err != nil {
			fmt.Fprintf(d.output, "Error: %v\n", err)
		}
		
		if !d.running {
			d.display()
		}
	}
	
	return nil
}

// handleCommand processes debugger commands
func (d *Debugger) handleCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}
	
	switch parts[0] {
	case "h", "help", "?":
		d.printHelp()
		
	case "s", "step":
		cycles := d.executeInstruction()
		d.cycleCount += uint64(cycles)
		d.instrCount++
		fmt.Fprintf(d.output, "Executed 1 instruction (%d cycles)\n", cycles)
		
	case "n", "next":
		// Step over (execute until PC changes or RET)
		startPC := d.emu.GetPC()
		for {
			cycles := d.executeInstruction()
			d.cycleCount += uint64(cycles)
			d.instrCount++
			if d.emu.GetPC() != startPC+1 && d.emu.GetPC() != startPC+2 && d.emu.GetPC() != startPC+3 {
				break
			}
		}
		
	case "c", "continue", "run":
		d.stepMode = false
		d.running = true
		fmt.Fprintln(d.output, "Running... (press Ctrl+C to break)")
		
	case "b", "break", "bp":
		if len(parts) < 2 {
			d.listBreakpoints()
		} else {
			addr := d.parseAddress(parts[1])
			d.setBreakpoint(addr)
		}
		
	case "d", "delete":
		if len(parts) < 2 {
			fmt.Fprintln(d.output, "Usage: delete <address>")
		} else {
			addr := d.parseAddress(parts[1])
			d.deleteBreakpoint(addr)
		}
		
	case "w", "watch":
		if len(parts) < 2 {
			d.listWatchpoints()
		} else {
			addr := d.parseAddress(parts[1])
			wtype := WatchReadWrite
			if len(parts) > 2 {
				switch parts[2] {
				case "r", "read":
					wtype = WatchRead
				case "w", "write":
					wtype = WatchWrite
				}
			}
			d.setWatchpoint(addr, wtype)
		}
		
	case "r", "regs", "registers":
		d.displayRegisters()
		
	case "m", "mem", "memory":
		if len(parts) > 1 {
			d.memAddr = d.parseAddress(parts[1])
		}
		d.displayMemory(d.memAddr, 128)
		
	case "dis", "disasm", "disassemble":
		if len(parts) > 1 {
			d.disasmAddr = d.parseAddress(parts[1])
		}
		d.displayDisassembly(d.disasmAddr, 10)
		
	case "stack":
		d.displayStack()
		
	case "set":
		if len(parts) < 3 {
			fmt.Fprintln(d.output, "Usage: set <register> <value>")
		} else {
			d.setRegister(parts[1], parts[2])
		}
		
	case "load":
		if len(parts) < 3 {
			fmt.Fprintln(d.output, "Usage: load <file> <address>")
		} else {
			addr := d.parseAddress(parts[2])
			d.loadFile(parts[1], addr)
		}
		
	case "save":
		if len(parts) < 4 {
			fmt.Fprintln(d.output, "Usage: save <file> <start> <end>")
		} else {
			start := d.parseAddress(parts[2])
			end := d.parseAddress(parts[3])
			d.saveMemory(parts[1], start, end)
		}
		
	case "history", "hist":
		d.displayHistory()
		
	case "stats":
		d.displayStats()
		
	case "reset":
		d.emu.Reset()
		d.cycleCount = 0
		d.instrCount = 0
		fmt.Fprintln(d.output, "CPU reset")
		
	case "q", "quit", "exit":
		fmt.Fprintln(d.output, "Goodbye!")
		os.Exit(0)
		
	default:
		fmt.Fprintf(d.output, "Unknown command: %s (type 'help' for commands)\n", parts[0])
	}
	
	return nil
}

// executeInstruction executes a single instruction
func (d *Debugger) executeInstruction() int {
	// Record history
	d.recordHistory()
	
	// Execute
	cycles := d.emu.Step()
	
	return cycles
}

// display shows the current debugger view
func (d *Debugger) display() {
	if d.showRegs {
		d.displayRegisters()
	}
	if d.showDisasm {
		d.displayDisassembly(d.emu.GetPC(), 5)
	}
	if d.showMem {
		d.displayMemory(d.memAddr, 64)
	}
	if d.showStack {
		d.displayStack()
	}
}

// displayRegisters shows CPU registers
func (d *Debugger) displayRegisters() {
	fmt.Fprintln(d.output, "┌─────────────────────────────────────────────────────┐")
	fmt.Fprintf(d.output, "│ PC:%04X SP:%04X IX:%04X IY:%04X I:%02X IM:%d%s│\n",
		d.emu.GetPC(), d.emu.GetSP(), d.emu.GetIX(), d.emu.GetIY(), d.emu.GetI(), d.emu.GetIM(),
		strings.Repeat(" ", 5))
	
	fmt.Fprintf(d.output, "│ AF:%04X BC:%04X DE:%04X HL:%04X ",
		d.emu.AF(), d.emu.BC(), d.emu.DE(), d.emu.HL())
	
	// Flags
	flags := ""
	f := d.emu.GetF()
	if f&0x80 != 0 { flags += "S" } else { flags += "-" }
	if f&0x40 != 0 { flags += "Z" } else { flags += "-" }
	if f&0x10 != 0 { flags += "H" } else { flags += "-" }
	if f&0x04 != 0 { flags += "P" } else { flags += "-" }
	if f&0x02 != 0 { flags += "N" } else { flags += "-" }
	if f&0x01 != 0 { flags += "C" } else { flags += "-" }
	
	fmt.Fprintf(d.output, "[%s]    │\n", flags)
	
	fmt.Fprintf(d.output, "│ AF':%04X BC':%04X DE':%04X HL':%04X%s│\n",
		0, 0, 0, 0,  // Shadow registers not exposed yet
		strings.Repeat(" ", 12))
	fmt.Fprintln(d.output, "└─────────────────────────────────────────────────────┘")
}

// displayMemory shows memory contents
func (d *Debugger) displayMemory(addr uint16, size int) {
	fmt.Fprintln(d.output, "┌─────────────────────────────────────────────────────┐")
	fmt.Fprintln(d.output, "│ Memory                                              │")
	fmt.Fprintln(d.output, "├─────────────────────────────────────────────────────┤")
	
	for i := 0; i < size; i += 16 {
		fmt.Fprintf(d.output, "│ %04X: ", addr+uint16(i))
		
		// Hex bytes
		for j := 0; j < 16; j++ {
			if i+j < size {
				fmt.Fprintf(d.output, "%02X ", d.emu.ReadMemory(addr+uint16(i+j)))
			} else {
				fmt.Fprint(d.output, "   ")
			}
		}
		
		// ASCII representation
		fmt.Fprint(d.output, " ")
		for j := 0; j < 16; j++ {
			if i+j < size {
				b := d.emu.ReadMemory(addr+uint16(i+j))
				if b >= 32 && b < 127 {
					fmt.Fprintf(d.output, "%c", b)
				} else {
					fmt.Fprint(d.output, ".")
				}
			}
		}
		
		fmt.Fprintln(d.output, " │")
	}
	
	fmt.Fprintln(d.output, "└─────────────────────────────────────────────────────┘")
}

// displayDisassembly shows disassembled instructions
func (d *Debugger) displayDisassembly(addr uint16, lines int) {
	fmt.Fprintln(d.output, "┌─────────────────────────────────────────────────────┐")
	fmt.Fprintln(d.output, "│ Disassembly                                         │")
	fmt.Fprintln(d.output, "├─────────────────────────────────────────────────────┤")
	
	for i := 0; i < lines; i++ {
		// Current instruction marker
		marker := "  "
		if addr == d.emu.GetPC() {
			marker = "▶ "
		}
		
		// Disassemble instruction
		instr, size := d.disassemble(addr)
		
		// Show bytes
		bytes := ""
		for j := 0; j < size; j++ {
			bytes += fmt.Sprintf("%02X ", d.emu.ReadMemory(addr+uint16(j)))
		}
		
		fmt.Fprintf(d.output, "│ %s%04X: %-12s %s%s│\n",
			marker, addr, bytes, instr,
			strings.Repeat(" ", 20-len(instr)))
		
		addr += uint16(size)
	}
	
	fmt.Fprintln(d.output, "└─────────────────────────────────────────────────────┘")
}

// displayStack shows the stack contents
func (d *Debugger) displayStack() {
	fmt.Fprintln(d.output, "┌─────────────────────────────────────────────────────┐")
	fmt.Fprintln(d.output, "│ Stack                                               │")
	fmt.Fprintln(d.output, "├─────────────────────────────────────────────────────┤")
	
	sp := d.emu.GetSP()
	for i := 0; i < 8; i++ {
		value := uint16(d.emu.ReadMemory(sp)) | uint16(d.emu.ReadMemory(sp+1))<<8
		
		marker := "  "
		if i == 0 {
			marker = "SP"
		}
		
		// Try to identify what the value might be
		hint := ""
		if value >= 0x8000 && value < 0xC000 {
			hint = " (code?)"
		} else if value >= 0x4000 && value < 0x8000 {
			hint = " (data?)"
		}
		
		fmt.Fprintf(d.output, "│ %s %04X: %04X%s%s│\n",
			marker, sp, value, hint,
			strings.Repeat(" ", 30-len(hint)))
		
		sp += 2
	}
	
	fmt.Fprintln(d.output, "└─────────────────────────────────────────────────────┘")
}

// Helper functions

func (d *Debugger) printBanner() {
	fmt.Fprintln(d.output, "╔═══════════════════════════════════════════════════════╗")
	fmt.Fprintln(d.output, "║      MinZ Z80 Debugger - Retro Meets Modern! 🎮       ║")
	fmt.Fprintln(d.output, "╚═══════════════════════════════════════════════════════╝")
	fmt.Fprintln(d.output, "Type 'help' for commands, 's' to step, 'c' to continue")
	fmt.Fprintln(d.output)
}

func (d *Debugger) printHelp() {
	fmt.Fprintln(d.output, "Commands:")
	fmt.Fprintln(d.output, "  s/step           - Step one instruction")
	fmt.Fprintln(d.output, "  n/next           - Step over calls")
	fmt.Fprintln(d.output, "  c/continue       - Run until breakpoint")
	fmt.Fprintln(d.output, "  b/break <addr>   - Set breakpoint")
	fmt.Fprintln(d.output, "  d/delete <addr>  - Delete breakpoint")
	fmt.Fprintln(d.output, "  w/watch <addr>   - Set watchpoint")
	fmt.Fprintln(d.output, "  r/regs           - Show registers")
	fmt.Fprintln(d.output, "  m/mem <addr>     - Show memory")
	fmt.Fprintln(d.output, "  dis <addr>       - Disassemble")
	fmt.Fprintln(d.output, "  stack            - Show stack")
	fmt.Fprintln(d.output, "  set <reg> <val>  - Set register")
	fmt.Fprintln(d.output, "  history          - Show execution history")
	fmt.Fprintln(d.output, "  stats            - Show statistics")
	fmt.Fprintln(d.output, "  reset            - Reset CPU")
	fmt.Fprintln(d.output, "  q/quit           - Exit debugger")
}

func (d *Debugger) checkBreakpoint() bool {
	return d.breakpoints[d.emu.GetPC()]
}

func (d *Debugger) checkWatchpoint() (uint16, WatchType) {
	// TODO: Hook into memory access
	return 0xFFFF, 0
}

func (d *Debugger) setBreakpoint(addr uint16) {
	d.breakpoints[addr] = true
	fmt.Fprintf(d.output, "Breakpoint set at $%04X\n", addr)
}

func (d *Debugger) deleteBreakpoint(addr uint16) {
	delete(d.breakpoints, addr)
	fmt.Fprintf(d.output, "Breakpoint deleted at $%04X\n", addr)
}

func (d *Debugger) listBreakpoints() {
	if len(d.breakpoints) == 0 {
		fmt.Fprintln(d.output, "No breakpoints set")
		return
	}
	
	fmt.Fprintln(d.output, "Breakpoints:")
	for addr := range d.breakpoints {
		fmt.Fprintf(d.output, "  $%04X\n", addr)
	}
}

func (d *Debugger) setWatchpoint(addr uint16, wtype WatchType) {
	d.watchpoints[addr] = wtype
	fmt.Fprintf(d.output, "Watchpoint set at $%04X (%s)\n", addr, watchTypeString(wtype))
}

func (d *Debugger) listWatchpoints() {
	if len(d.watchpoints) == 0 {
		fmt.Fprintln(d.output, "No watchpoints set")
		return
	}
	
	fmt.Fprintln(d.output, "Watchpoints:")
	for addr, wtype := range d.watchpoints {
		fmt.Fprintf(d.output, "  $%04X (%s)\n", addr, watchTypeString(wtype))
	}
}

func (d *Debugger) parseAddress(s string) uint16 {
	// Handle $ prefix for hex
	if strings.HasPrefix(s, "$") {
		s = s[1:]
	}
	
	// Parse as hex
	addr, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		// Try decimal
		addr, err = strconv.ParseUint(s, 10, 16)
		if err != nil {
			return 0
		}
	}
	
	return uint16(addr)
}

func (d *Debugger) setRegister(reg, value string) {
	val := d.parseAddress(value)
	
	switch strings.ToUpper(reg) {
	case "A":
		d.emu.SetA(byte(val))
	case "B":
		d.emu.SetB(byte(val))
	case "C":
		d.emu.SetC(byte(val))
	case "D":
		d.emu.SetD(byte(val))
	case "E":
		d.emu.SetE(byte(val))
	case "H":
		d.emu.SetH(byte(val))
	case "L":
		d.emu.SetL(byte(val))
	case "F":
		d.emu.SetF(byte(val))
	case "PC":
		d.emu.SetPC(val)
	case "SP":
		d.emu.SetSP(val)
	case "IX":
		d.emu.SetIX(val)
	case "IY":
		d.emu.SetIY(val)
	default:
		fmt.Fprintf(d.output, "Unknown register: %s\n", reg)
		return
	}
	
	fmt.Fprintf(d.output, "%s = $%04X\n", strings.ToUpper(reg), val)
}

func (d *Debugger) loadFile(filename string, addr uint16) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(d.output, "Error loading file: %v\n", err)
		return
	}
	
	// Write data to memory
	for i, b := range data {
		d.emu.WriteMemory(addr+uint16(i), b)
	}
	fmt.Fprintf(d.output, "Loaded %d bytes at $%04X\n", len(data), addr)
}

func (d *Debugger) saveMemory(filename string, start, end uint16) {
	if end <= start {
		fmt.Fprintln(d.output, "Invalid address range")
		return
	}
	
	// Read memory range
	data := make([]byte, end-start+1)
	for i := range data {
		data[i] = d.emu.ReadMemory(start+uint16(i))
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		fmt.Fprintf(d.output, "Error saving file: %v\n", err)
		return
	}
	
	fmt.Fprintf(d.output, "Saved %d bytes to %s\n", len(data), filename)
}

func (d *Debugger) recordHistory() {
	if len(d.history) >= d.maxHistory {
		d.history = d.history[1:]
	}
	
	instr, _ := d.disassemble(d.emu.PC)
	entry := HistoryEntry{
		PC:          d.emu.PC,
		Instruction: instr,
		Registers: RegisterSnapshot{
			A: d.emu.A, F: d.emu.F,
			B: d.emu.B, C: d.emu.C,
			D: d.emu.D, E: d.emu.E,
			H: d.emu.H, L: d.emu.L,
			SP: d.emu.SP,
			IX: d.emu.IX,
			IY: d.emu.IY,
		},
	}
	
	d.history = append(d.history, entry)
}

func (d *Debugger) displayHistory() {
	if len(d.history) == 0 {
		fmt.Fprintln(d.output, "No history")
		return
	}
	
	fmt.Fprintln(d.output, "Execution history:")
	for i, entry := range d.history {
		fmt.Fprintf(d.output, "%3d: %04X %s\n", i, entry.PC, entry.Instruction)
	}
}

func (d *Debugger) displayStats() {
	fmt.Fprintln(d.output, "┌─────────────────────────────────────────────────────┐")
	fmt.Fprintln(d.output, "│ Statistics                                          │")
	fmt.Fprintln(d.output, "├─────────────────────────────────────────────────────┤")
	fmt.Fprintf(d.output, "│ Instructions: %-10d                          │\n", d.instrCount)
	fmt.Fprintf(d.output, "│ Cycles:       %-10d                          │\n", d.cycleCount)
	if d.instrCount > 0 {
		avg := d.cycleCount / d.instrCount
		fmt.Fprintf(d.output, "│ Avg cycles:   %-10d                          │\n", avg)
	}
	fmt.Fprintln(d.output, "└─────────────────────────────────────────────────────┘")
}

// Simple disassembler (abbreviated for space)
func (d *Debugger) disassemble(addr uint16) (string, int) {
	opcode := d.emu.ReadMemory(addr)
	
	// Basic opcodes (simplified)
	switch opcode {
	case 0x00:
		return "NOP", 1
	case 0x01:
		return fmt.Sprintf("LD BC, $%04X", 
			uint16(d.emu.ReadMemory(addr+1))|uint16(d.emu.ReadMemory(addr+2))<<8), 3
	case 0x3E:
		return fmt.Sprintf("LD A, $%02X", d.emu.ReadMemory(addr+1)), 2
	case 0xC3:
		return fmt.Sprintf("JP $%04X",
			uint16(d.emu.ReadMemory(addr+1))|uint16(d.emu.ReadMemory(addr+2))<<8), 3
	case 0xC9:
		return "RET", 1
	case 0xCD:
		return fmt.Sprintf("CALL $%04X",
			uint16(d.emu.ReadMemory(addr+1))|uint16(d.emu.ReadMemory(addr+2))<<8), 3
	default:
		return fmt.Sprintf("DB $%02X", opcode), 1
	}
}

func watchTypeString(wtype WatchType) string {
	switch wtype {
	case WatchRead:
		return "read"
	case WatchWrite:
		return "write"
	case WatchReadWrite:
		return "read/write"
	default:
		return "unknown"
	}
}