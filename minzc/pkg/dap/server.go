package dap

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/minz/minzc/pkg/debugger"
)

// Server implements the Debug Adapter Protocol server
type Server struct {
	mu       sync.Mutex
	debugger *debugger.Debugger

	reader *bufio.Reader
	writer io.Writer

	seq            int
	initialized    bool
	launched       bool
	clientLinesAt1 bool
	clientColsAt1  bool

	// Source-to-breakpoint mapping
	breakpointsBySource map[string]map[int]*debugger.Breakpoint

	// Variable references for scopes
	nextVarRef int
	varRefs    map[int]variableScope
}

type variableScope struct {
	scopeType string // "registers", "locals", "memory"
	frameId   int
}

// NewServer creates a new DAP server
func NewServer() *Server {
	return &Server{
		debugger:            debugger.NewDebugger(),
		breakpointsBySource: make(map[string]map[int]*debugger.Breakpoint),
		varRefs:             make(map[int]variableScope),
		nextVarRef:          1000,
	}
}

// Run starts the DAP server on stdin/stdout
func (s *Server) Run() error {
	return s.RunWithIO(os.Stdin, os.Stdout)
}

// RunWithIO starts the DAP server with custom I/O
func (s *Server) RunWithIO(in io.Reader, out io.Writer) error {
	s.reader = bufio.NewReader(in)
	s.writer = out

	// Set up stop callback
	s.debugger.OnStop(func(event debugger.StopEvent) {
		s.handleStopEvent(event)
	})

	// Set up SMC callback for visualization
	s.debugger.OnSMC(func(event debugger.SMCEvent) {
		s.handleSMCEvent(event)
	})

	// Main message loop
	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		if err := s.handleMessage(msg); err != nil {
			s.sendErrorResponse(msg, err.Error())
		}
	}
}

// readMessage reads a DAP message from the input
func (s *Server) readMessage() (map[string]interface{}, error) {
	// Read Content-Length header
	var contentLength int
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // Empty line ends headers
		}
		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %s", lengthStr)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read content
	content := make([]byte, contentLength)
	_, err := io.ReadFull(s.reader, content)
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var msg map[string]interface{}
	if err := json.Unmarshal(content, &msg); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return msg, nil
}

// sendMessage sends a DAP message
func (s *Server) sendMessage(msg interface{}) error {
	content, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	if _, err := s.writer.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := s.writer.Write(content); err != nil {
		return err
	}
	return nil
}

// sendResponse sends a response to a request
func (s *Server) sendResponse(req map[string]interface{}, success bool, body interface{}) {
	s.mu.Lock()
	s.seq++
	seq := s.seq
	s.mu.Unlock()

	resp := Response{
		Message: Message{
			Seq:  seq,
			Type: "response",
		},
		RequestSeq: int(req["seq"].(float64)),
		Success:    success,
		Command:    req["command"].(string),
		Body:       body,
	}

	s.sendMessage(resp)
}

// sendErrorResponse sends an error response
func (s *Server) sendErrorResponse(req map[string]interface{}, message string) {
	s.mu.Lock()
	s.seq++
	seq := s.seq
	s.mu.Unlock()

	resp := Response{
		Message: Message{
			Seq:  seq,
			Type: "response",
		},
		RequestSeq:   int(req["seq"].(float64)),
		Success:      false,
		Command:      req["command"].(string),
		ErrorMessage: message,
	}

	s.sendMessage(resp)
}

// sendEvent sends an event
func (s *Server) sendEvent(eventType string, body interface{}) {
	s.mu.Lock()
	s.seq++
	seq := s.seq
	s.mu.Unlock()

	event := Event{
		Message: Message{
			Seq:  seq,
			Type: "event",
		},
		Event: eventType,
		Body:  body,
	}

	s.sendMessage(event)
}

// handleMessage dispatches a message to the appropriate handler
func (s *Server) handleMessage(msg map[string]interface{}) error {
	msgType, ok := msg["type"].(string)
	if !ok {
		return fmt.Errorf("missing message type")
	}

	if msgType != "request" {
		return nil // Ignore non-requests
	}

	command, ok := msg["command"].(string)
	if !ok {
		return fmt.Errorf("missing command")
	}

	switch command {
	case "initialize":
		return s.handleInitialize(msg)
	case "launch":
		return s.handleLaunch(msg)
	case "setBreakpoints":
		return s.handleSetBreakpoints(msg)
	case "configurationDone":
		return s.handleConfigurationDone(msg)
	case "threads":
		return s.handleThreads(msg)
	case "stackTrace":
		return s.handleStackTrace(msg)
	case "scopes":
		return s.handleScopes(msg)
	case "variables":
		return s.handleVariables(msg)
	case "continue":
		return s.handleContinue(msg)
	case "next":
		return s.handleNext(msg)
	case "stepIn":
		return s.handleStepIn(msg)
	case "stepOut":
		return s.handleStepOut(msg)
	case "pause":
		return s.handlePause(msg)
	case "evaluate":
		return s.handleEvaluate(msg)
	case "readMemory":
		return s.handleReadMemory(msg)
	case "writeMemory":
		return s.handleWriteMemory(msg)
	case "disassemble":
		return s.handleDisassemble(msg)
	case "disconnect":
		return s.handleDisconnect(msg)
	default:
		s.sendResponse(msg, true, nil)
		return nil
	}
}

// === Request Handlers ===

func (s *Server) handleInitialize(req map[string]interface{}) error {
	args, _ := req["arguments"].(map[string]interface{})
	if args != nil {
		if linesAt1, ok := args["linesStartAt1"].(bool); ok {
			s.clientLinesAt1 = linesAt1
		} else {
			s.clientLinesAt1 = true // Default
		}
		if colsAt1, ok := args["columnsStartAt1"].(bool); ok {
			s.clientColsAt1 = colsAt1
		} else {
			s.clientColsAt1 = true // Default
		}
	}

	capabilities := Capabilities{
		SupportsConfigurationDoneRequest: true,
		SupportsConditionalBreakpoints:   true,
		SupportsHitConditionalBreakpoints: true,
		SupportsEvaluateForHovers:        true,
		SupportsReadMemoryRequest:        true,
		SupportsWriteMemoryRequest:       true,
		SupportsDisassembleRequest:       true,
		SupportsTerminateRequest:         true,
	}

	s.initialized = true
	s.sendResponse(req, true, capabilities)
	s.sendEvent("initialized", nil)
	return nil
}

func (s *Server) handleLaunch(req map[string]interface{}) error {
	args, _ := req["arguments"].(map[string]interface{})
	if args == nil {
		return fmt.Errorf("missing launch arguments")
	}

	program, ok := args["program"].(string)
	if !ok {
		return fmt.Errorf("missing program path")
	}

	// Read the binary file
	binary, err := os.ReadFile(program)
	if err != nil {
		return fmt.Errorf("read program: %w", err)
	}

	// Get load/start addresses
	loadAddr := uint16(0x8000) // Default
	startAddr := uint16(0x8000)

	if addr, ok := args["loadAddress"].(float64); ok {
		loadAddr = uint16(addr)
	}
	if addr, ok := args["startAddress"].(float64); ok {
		startAddr = uint16(addr)
	}

	// Load into emulator
	if err := s.debugger.LoadProgram(binary, loadAddr, startAddr); err != nil {
		return fmt.Errorf("load program: %w", err)
	}

	s.launched = true
	s.sendResponse(req, true, nil)

	// If stopOnEntry, send stopped event
	if stopOnEntry, ok := args["stopOnEntry"].(bool); ok && stopOnEntry {
		s.sendEvent("stopped", StoppedEventBody{
			Reason:            "entry",
			ThreadId:          1,
			AllThreadsStopped: true,
		})
	}

	return nil
}

func (s *Server) handleSetBreakpoints(req map[string]interface{}) error {
	args, _ := req["arguments"].(map[string]interface{})
	if args == nil {
		return fmt.Errorf("missing arguments")
	}

	sourceMap, _ := args["source"].(map[string]interface{})
	sourcePath, _ := sourceMap["path"].(string)

	// Clear existing breakpoints for this source
	if existing, ok := s.breakpointsBySource[sourcePath]; ok {
		for _, bp := range existing {
			s.debugger.ClearBreakpoint(bp.Address)
		}
	}
	s.breakpointsBySource[sourcePath] = make(map[int]*debugger.Breakpoint)

	// Set new breakpoints
	bpsArg, _ := args["breakpoints"].([]interface{})
	var breakpoints []Breakpoint

	for _, bpArg := range bpsArg {
		bpMap, _ := bpArg.(map[string]interface{})
		line := int(bpMap["line"].(float64))

		// Convert source line to address (using source map)
		// For now, we don't have source mapping, so we can't set source breakpoints
		// This will be enhanced when compiler generates debug info

		bp := Breakpoint{
			Verified: false, // Can't verify without source map
			Line:     line,
			Message:  "Source maps not yet available",
			Source: &Source{
				Path: sourcePath,
			},
		}
		breakpoints = append(breakpoints, bp)
	}

	s.sendResponse(req, true, SetBreakpointsResponseBody{
		Breakpoints: breakpoints,
	})
	return nil
}

func (s *Server) handleConfigurationDone(req map[string]interface{}) error {
	s.sendResponse(req, true, nil)
	return nil
}

func (s *Server) handleThreads(req map[string]interface{}) error {
	// Z80 is single-threaded
	s.sendResponse(req, true, ThreadsResponseBody{
		Threads: []Thread{
			{Id: 1, Name: "Z80"},
		},
	})
	return nil
}

func (s *Server) handleStackTrace(req map[string]interface{}) error {
	// For now, just show current PC as a single frame
	// Real stack tracing requires source maps
	pc := s.debugger.GetPC()

	frames := []StackFrame{
		{
			Id:   0,
			Name: fmt.Sprintf("0x%04X", pc),
			Line: 1, // Unknown without source map
			Column: 1,
			InstructionPointerReference: fmt.Sprintf("0x%04X", pc),
		},
	}

	s.sendResponse(req, true, StackTraceResponseBody{
		StackFrames: frames,
		TotalFrames: 1,
	})
	return nil
}

func (s *Server) handleScopes(req map[string]interface{}) error {
	// Create variable references for registers and memory
	s.mu.Lock()
	regRef := s.nextVarRef
	s.nextVarRef++
	s.varRefs[regRef] = variableScope{scopeType: "registers"}
	s.mu.Unlock()

	scopes := []Scope{
		{
			Name:               "Registers",
			PresentationHint:   "registers",
			VariablesReference: regRef,
			Expensive:          false,
		},
	}

	s.sendResponse(req, true, ScopesResponseBody{
		Scopes: scopes,
	})
	return nil
}

func (s *Server) handleVariables(req map[string]interface{}) error {
	args, _ := req["arguments"].(map[string]interface{})
	varRef := int(args["variablesReference"].(float64))

	s.mu.Lock()
	scope, ok := s.varRefs[varRef]
	s.mu.Unlock()

	if !ok {
		s.sendResponse(req, true, VariablesResponseBody{Variables: []Variable{}})
		return nil
	}

	var vars []Variable

	if scope.scopeType == "registers" {
		regs := s.debugger.GetRegisters()
		vars = []Variable{
			{Name: "A", Value: fmt.Sprintf("0x%02X", regs.A), Type: "uint8"},
			{Name: "F", Value: fmt.Sprintf("0x%02X", regs.F), Type: "uint8"},
			{Name: "BC", Value: fmt.Sprintf("0x%04X", regs.BC), Type: "uint16"},
			{Name: "DE", Value: fmt.Sprintf("0x%04X", regs.DE), Type: "uint16"},
			{Name: "HL", Value: fmt.Sprintf("0x%04X", regs.HL), Type: "uint16"},
			{Name: "IX", Value: fmt.Sprintf("0x%04X", regs.IX), Type: "uint16"},
			{Name: "IY", Value: fmt.Sprintf("0x%04X", regs.IY), Type: "uint16"},
			{Name: "SP", Value: fmt.Sprintf("0x%04X", regs.SP), Type: "uint16"},
			{Name: "PC", Value: fmt.Sprintf("0x%04X", regs.PC), Type: "uint16"},
		}
	}

	s.sendResponse(req, true, VariablesResponseBody{
		Variables: vars,
	})
	return nil
}

func (s *Server) handleContinue(req map[string]interface{}) error {
	s.sendResponse(req, true, ContinueResponseBody{
		AllThreadsContinued: true,
	})

	go func() {
		s.debugger.Continue()
	}()

	return nil
}

func (s *Server) handleNext(req map[string]interface{}) error {
	// Step over - for now same as step (no call detection)
	s.sendResponse(req, true, nil)
	go func() {
		s.debugger.Step()
	}()
	return nil
}

func (s *Server) handleStepIn(req map[string]interface{}) error {
	s.sendResponse(req, true, nil)
	go func() {
		s.debugger.Step()
	}()
	return nil
}

func (s *Server) handleStepOut(req map[string]interface{}) error {
	// Step out - not implemented yet
	s.sendResponse(req, true, nil)
	go func() {
		s.debugger.Step()
	}()
	return nil
}

func (s *Server) handlePause(req map[string]interface{}) error {
	s.debugger.Pause()
	s.sendResponse(req, true, nil)
	return nil
}

func (s *Server) handleEvaluate(req map[string]interface{}) error {
	args, _ := req["arguments"].(map[string]interface{})
	expr, _ := args["expression"].(string)

	// Simple expression evaluation for memory addresses
	// Format: [0xADDR] or 0xADDR
	var result string

	if strings.HasPrefix(expr, "[") && strings.HasSuffix(expr, "]") {
		// Memory read
		addrStr := strings.TrimPrefix(strings.TrimSuffix(expr, "]"), "[")
		addr, err := strconv.ParseUint(strings.TrimPrefix(addrStr, "0x"), 16, 16)
		if err == nil {
			data := s.debugger.ReadMemory(uint16(addr), 1)
			result = fmt.Sprintf("0x%02X", data[0])
		} else {
			result = "Invalid address"
		}
	} else if strings.HasPrefix(expr, "0x") || strings.HasPrefix(expr, "$") {
		// Hex literal
		result = expr
	} else {
		// Register lookup
		regs := s.debugger.GetRegisters()
		switch strings.ToUpper(expr) {
		case "A":
			result = fmt.Sprintf("0x%02X", regs.A)
		case "F":
			result = fmt.Sprintf("0x%02X", regs.F)
		case "BC":
			result = fmt.Sprintf("0x%04X", regs.BC)
		case "DE":
			result = fmt.Sprintf("0x%04X", regs.DE)
		case "HL":
			result = fmt.Sprintf("0x%04X", regs.HL)
		case "SP":
			result = fmt.Sprintf("0x%04X", regs.SP)
		case "PC":
			result = fmt.Sprintf("0x%04X", regs.PC)
		case "IX":
			result = fmt.Sprintf("0x%04X", regs.IX)
		case "IY":
			result = fmt.Sprintf("0x%04X", regs.IY)
		default:
			result = "Unknown expression"
		}
	}

	s.sendResponse(req, true, EvaluateResponseBody{
		Result: result,
	})
	return nil
}

func (s *Server) handleReadMemory(req map[string]interface{}) error {
	args, _ := req["arguments"].(map[string]interface{})
	memRef, _ := args["memoryReference"].(string)
	count := int(args["count"].(float64))
	offset := 0
	if off, ok := args["offset"].(float64); ok {
		offset = int(off)
	}

	// Parse address from reference
	addr, err := strconv.ParseUint(strings.TrimPrefix(memRef, "0x"), 16, 16)
	if err != nil {
		return fmt.Errorf("invalid memory reference: %s", memRef)
	}

	data := s.debugger.ReadMemory(uint16(int(addr)+offset), count)
	encoded := base64.StdEncoding.EncodeToString(data)

	s.sendResponse(req, true, ReadMemoryResponseBody{
		Address: fmt.Sprintf("0x%04X", int(addr)+offset),
		Data:    encoded,
	})
	return nil
}

func (s *Server) handleWriteMemory(req map[string]interface{}) error {
	args, _ := req["arguments"].(map[string]interface{})
	memRef, _ := args["memoryReference"].(string)
	dataB64, _ := args["data"].(string)
	offset := 0
	if off, ok := args["offset"].(float64); ok {
		offset = int(off)
	}

	// Parse address
	addr, err := strconv.ParseUint(strings.TrimPrefix(memRef, "0x"), 16, 16)
	if err != nil {
		return fmt.Errorf("invalid memory reference: %s", memRef)
	}

	// Decode data
	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return fmt.Errorf("invalid base64 data: %w", err)
	}

	s.debugger.WriteMemory(uint16(int(addr)+offset), data)

	s.sendResponse(req, true, WriteMemoryResponseBody{
		BytesWritten: len(data),
	})
	return nil
}

func (s *Server) handleDisassemble(req map[string]interface{}) error {
	args, _ := req["arguments"].(map[string]interface{})
	memRef, _ := args["memoryReference"].(string)
	count := int(args["instructionCount"].(float64))

	// Parse address
	addr, err := strconv.ParseUint(strings.TrimPrefix(memRef, "0x"), 16, 16)
	if err != nil {
		return fmt.Errorf("invalid memory reference: %s", memRef)
	}

	// Read memory and disassemble
	// Simple disassembly - just show bytes for now
	// TODO: Full Z80 disassembler
	var instructions []DisassembledInstruction

	currentAddr := uint16(addr)
	for i := 0; i < count; i++ {
		data := s.debugger.ReadMemory(currentAddr, 4)
		inst := DisassembledInstruction{
			Address:          fmt.Sprintf("0x%04X", currentAddr),
			InstructionBytes: fmt.Sprintf("%02X", data[0]),
			Instruction:      fmt.Sprintf("DB $%02X", data[0]), // Placeholder
		}
		instructions = append(instructions, inst)
		currentAddr++
	}

	s.sendResponse(req, true, DisassembleResponseBody{
		Instructions: instructions,
	})
	return nil
}

func (s *Server) handleDisconnect(req map[string]interface{}) error {
	s.sendResponse(req, true, nil)
	s.sendEvent("terminated", TerminatedEventBody{})
	return nil
}

// === Event Handlers ===

func (s *Server) handleStopEvent(event debugger.StopEvent) {
	var reason string
	switch event.Reason {
	case debugger.StopBreakpoint:
		reason = "breakpoint"
	case debugger.StopStep:
		reason = "step"
	case debugger.StopPause:
		reason = "pause"
	case debugger.StopExit:
		s.sendEvent("terminated", TerminatedEventBody{})
		return
	default:
		reason = "unknown"
	}

	var hitBreakpointIds []int
	if event.Breakpoint != nil {
		hitBreakpointIds = []int{event.Breakpoint.ID}
	}

	s.sendEvent("stopped", StoppedEventBody{
		Reason:            reason,
		ThreadId:          1,
		AllThreadsStopped: true,
		HitBreakpointIds:  hitBreakpointIds,
	})
}

func (s *Server) handleSMCEvent(event debugger.SMCEvent) {
	// Send SMC as output event for visualization
	s.sendEvent("output", OutputEventBody{
		Category: "console",
		Output:   fmt.Sprintf("[SMC] 0x%04X: 0x%02X -> 0x%02X (from PC 0x%04X)\n", event.Address, event.OldValue, event.NewValue, event.PC),
	})
}
