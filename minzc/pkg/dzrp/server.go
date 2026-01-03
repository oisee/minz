// Package dzrp implements a DeZog Remote Protocol server
//
// This server allows the MZE emulator to be debugged using the DeZog
// VS Code extension, providing full debugging capabilities including:
// - Breakpoints (execution, read, write)
// - Register inspection and modification
// - Memory inspection and modification
// - Step, continue, pause operations
// - SMC (Self-Modifying Code) tracking

package dzrp

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// Emulator defines the interface required by the DZRP server
type Emulator interface {
	// Registers
	GetPC() uint16
	SetPC(uint16)
	GetSP() uint16
	SetSP(uint16)
	GetA() byte
	SetA(byte)
	GetF() byte
	SetF(byte)
	GetB() byte
	SetB(byte)
	GetC() byte
	SetC(byte)
	GetD() byte
	SetD(byte)
	GetE() byte
	SetE(byte)
	GetH() byte
	SetH(byte)
	GetL() byte
	SetL(byte)
	GetIX() uint16
	SetIX(uint16)
	GetIY() uint16
	SetIY(uint16)
	GetI() byte
	SetI(byte)
	GetR() byte

	// Alternate registers (if available)
	GetAF2() uint16
	GetBC2() uint16
	GetDE2() uint16
	GetHL2() uint16

	// Interrupt state
	GetIM() byte
	GetIFF1() bool
	GetIFF2() bool

	// Memory
	GetMemory(addr uint16) byte
	SetMemory(addr uint16, value byte)
	ReadMemoryRange(addr uint16, length int) []byte
	WriteMemoryRange(addr uint16, data []byte)

	// Execution
	Step() int           // Execute one instruction, return cycles
	Run()                // Run until breakpoint or halt
	Pause()              // Pause execution
	IsRunning() bool     // Check if running
	IsHalted() bool      // Check if halted

	// Cycles
	GetCycles() int
}

// Breakpoint represents a debug breakpoint
type Breakpoint struct {
	ID      uint16
	Address uint16
	Type    byte // BP_EXEC, BP_READ, BP_WRITE
	Enabled bool
}

// Server implements the DZRP protocol server
type Server struct {
	emulator    Emulator
	listener    net.Listener
	conn        net.Conn
	running     bool
	paused      bool
	breakpoints map[uint16]*Breakpoint
	nextBPID    uint16
	mu          sync.Mutex

	// Configuration
	Port        int
	MachineName string
}

// NewServer creates a new DZRP server
func NewServer(emu Emulator) *Server {
	return &Server{
		emulator:    emu,
		breakpoints: make(map[uint16]*Breakpoint),
		nextBPID:    1,
		Port:        11000, // Default DeZog port
		MachineName: "MZE MinZ Emulator",
		paused:      true,
	}
}

// Start starts the DZRP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start DZRP server: %w", err)
	}

	s.listener = listener
	s.running = true

	log.Printf("DZRP server listening on port %d", s.Port)
	log.Printf("Connect DeZog to localhost:%d", s.Port)

	go s.acceptLoop()
	return nil
}

// Stop stops the DZRP server
func (s *Server) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	if s.conn != nil {
		s.conn.Close()
	}
}

// acceptLoop handles incoming connections
func (s *Server) acceptLoop() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("DZRP accept error: %v", err)
			}
			continue
		}

		log.Printf("DeZog connected from %s", conn.RemoteAddr())
		s.conn = conn
		s.handleConnection(conn)
	}
}

// handleConnection processes messages from a single connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		msg, err := ReadMessage(conn)
		if err != nil {
			if err == io.EOF {
				log.Printf("DeZog disconnected")
			} else {
				log.Printf("DZRP read error: %v", err)
			}
			return
		}

		err = s.handleMessage(conn, msg)
		if err != nil {
			log.Printf("DZRP handler error: %v", err)
		}
	}
}

// handleMessage dispatches a message to the appropriate handler
func (s *Server) handleMessage(w io.Writer, msg *Message) error {
	switch msg.Command {
	case CMD_INIT:
		return s.handleInit(w, msg)
	case CMD_CLOSE:
		return s.handleClose(w, msg)
	case CMD_GET_REGISTERS:
		return s.handleGetRegisters(w, msg)
	case CMD_SET_REGISTER:
		return s.handleSetRegister(w, msg)
	case CMD_CONTINUE:
		return s.handleContinue(w, msg)
	case CMD_PAUSE:
		return s.handlePause(w, msg)
	case CMD_ADD_BREAKPOINT:
		return s.handleAddBreakpoint(w, msg)
	case CMD_REMOVE_BREAKPOINT:
		return s.handleRemoveBreakpoint(w, msg)
	case CMD_READ_MEM:
		return s.handleReadMemory(w, msg)
	case CMD_WRITE_MEM:
		return s.handleWriteMemory(w, msg)
	case CMD_GET_SLOTS:
		return s.handleGetSlots(w, msg)
	case CMD_READ_STATE:
		return s.handleReadState(w, msg)
	default:
		log.Printf("Unknown DZRP command: %d", msg.Command)
		return nil
	}
}

// handleInit responds to the initialization handshake
func (s *Server) handleInit(w io.Writer, msg *Message) error {
	// Response format:
	// - 1 byte: error code (0 = success)
	// - 3 bytes: version (major, minor, patch)
	// - 1 byte: machine type
	// - string: machine name (length-prefixed)

	response := []byte{
		0, // No error
		DZRP_VERSION_MAJOR,
		DZRP_VERSION_MINOR,
		DZRP_VERSION_PATCH,
		MACHINE_ZX48K_LITE, // Report as ZX48K-lite (no contention)
	}

	// Add machine name (length + string)
	nameBytes := []byte(s.MachineName)
	response = append(response, byte(len(nameBytes)))
	response = append(response, nameBytes...)

	log.Printf("DZRP: Init handshake complete, reporting as %s", s.MachineName)
	return WriteResponse(w, CMD_INIT, response)
}

// handleClose handles the close command
func (s *Server) handleClose(w io.Writer, msg *Message) error {
	log.Printf("DZRP: Close command received")
	return WriteResponse(w, CMD_CLOSE, []byte{0})
}

// handleGetRegisters returns all Z80 registers
func (s *Server) handleGetRegisters(w io.Writer, msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Response format: 26 bytes of register values
	// PC(2) + SP(2) + AF(2) + BC(2) + DE(2) + HL(2) + IX(2) + IY(2)
	// + AF'(2) + BC'(2) + DE'(2) + HL'(2) + I(1) + R(1) + IM(1) + reserved(1)

	pc := s.emulator.GetPC()
	sp := s.emulator.GetSP()
	af := uint16(s.emulator.GetA())<<8 | uint16(s.emulator.GetF())
	bc := uint16(s.emulator.GetB())<<8 | uint16(s.emulator.GetC())
	de := uint16(s.emulator.GetD())<<8 | uint16(s.emulator.GetE())
	hl := uint16(s.emulator.GetH())<<8 | uint16(s.emulator.GetL())
	ix := s.emulator.GetIX()
	iy := s.emulator.GetIY()

	af2 := s.emulator.GetAF2()
	bc2 := s.emulator.GetBC2()
	de2 := s.emulator.GetDE2()
	hl2 := s.emulator.GetHL2()

	i := s.emulator.GetI()
	r := s.emulator.GetR()
	im := s.emulator.GetIM()

	response := make([]byte, 0, 26)
	response = append(response, EncodeU16LE(pc)...)
	response = append(response, EncodeU16LE(sp)...)
	response = append(response, EncodeU16LE(af)...)
	response = append(response, EncodeU16LE(bc)...)
	response = append(response, EncodeU16LE(de)...)
	response = append(response, EncodeU16LE(hl)...)
	response = append(response, EncodeU16LE(ix)...)
	response = append(response, EncodeU16LE(iy)...)
	response = append(response, EncodeU16LE(af2)...)
	response = append(response, EncodeU16LE(bc2)...)
	response = append(response, EncodeU16LE(de2)...)
	response = append(response, EncodeU16LE(hl2)...)
	response = append(response, i, r, im, 0)

	return WriteResponse(w, CMD_GET_REGISTERS, response)
}

// handleSetRegister sets a single register value
func (s *Server) handleSetRegister(w io.Writer, msg *Message) error {
	if len(msg.Data) < 3 {
		return fmt.Errorf("SET_REGISTER: insufficient data")
	}

	regIndex := msg.Data[0]
	value := DecodeU16LE(msg.Data[1:3])

	s.mu.Lock()
	defer s.mu.Unlock()

	switch regIndex {
	case REG_PC:
		s.emulator.SetPC(value)
	case REG_SP:
		s.emulator.SetSP(value)
	case REG_AF:
		s.emulator.SetA(byte(value >> 8))
		s.emulator.SetF(byte(value & 0xFF))
	case REG_BC:
		s.emulator.SetB(byte(value >> 8))
		s.emulator.SetC(byte(value & 0xFF))
	case REG_DE:
		s.emulator.SetD(byte(value >> 8))
		s.emulator.SetE(byte(value & 0xFF))
	case REG_HL:
		s.emulator.SetH(byte(value >> 8))
		s.emulator.SetL(byte(value & 0xFF))
	case REG_IX:
		s.emulator.SetIX(value)
	case REG_IY:
		s.emulator.SetIY(value)
	case REG_I:
		s.emulator.SetI(byte(value))
	default:
		log.Printf("DZRP: Unknown register index: %d", regIndex)
	}

	return WriteResponse(w, CMD_SET_REGISTER, []byte{0})
}

// handleContinue starts/continues execution
func (s *Server) handleContinue(w io.Writer, msg *Message) error {
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()

	// Start execution in background
	go s.runLoop()

	return WriteResponse(w, CMD_CONTINUE, []byte{0})
}

// handlePause pauses execution
func (s *Server) handlePause(w io.Writer, msg *Message) error {
	s.mu.Lock()
	s.paused = true
	s.mu.Unlock()

	s.emulator.Pause()

	// Send pause notification
	s.sendPauseNotification("Pause requested")

	return WriteResponse(w, CMD_PAUSE, []byte{0})
}

// handleAddBreakpoint adds a breakpoint
func (s *Server) handleAddBreakpoint(w io.Writer, msg *Message) error {
	if len(msg.Data) < 3 {
		return fmt.Errorf("ADD_BREAKPOINT: insufficient data")
	}

	address := DecodeU16LE(msg.Data[0:2])
	bpType := msg.Data[2]

	s.mu.Lock()
	bp := &Breakpoint{
		ID:      s.nextBPID,
		Address: address,
		Type:    bpType,
		Enabled: true,
	}
	s.breakpoints[address] = bp
	s.nextBPID++
	s.mu.Unlock()

	log.Printf("DZRP: Added breakpoint %d at $%04X (type %d)", bp.ID, address, bpType)

	// Response: breakpoint ID
	return WriteResponse(w, CMD_ADD_BREAKPOINT, EncodeU16LE(bp.ID))
}

// handleRemoveBreakpoint removes a breakpoint
func (s *Server) handleRemoveBreakpoint(w io.Writer, msg *Message) error {
	if len(msg.Data) < 2 {
		return fmt.Errorf("REMOVE_BREAKPOINT: insufficient data")
	}

	bpID := DecodeU16LE(msg.Data[0:2])

	s.mu.Lock()
	for addr, bp := range s.breakpoints {
		if bp.ID == bpID {
			delete(s.breakpoints, addr)
			log.Printf("DZRP: Removed breakpoint %d at $%04X", bpID, addr)
			break
		}
	}
	s.mu.Unlock()

	return WriteResponse(w, CMD_REMOVE_BREAKPOINT, []byte{0})
}

// handleReadMemory reads memory
func (s *Server) handleReadMemory(w io.Writer, msg *Message) error {
	if len(msg.Data) < 4 {
		return fmt.Errorf("READ_MEM: insufficient data")
	}

	address := DecodeU16LE(msg.Data[0:2])
	length := DecodeU16LE(msg.Data[2:4])

	s.mu.Lock()
	data := s.emulator.ReadMemoryRange(address, int(length))
	s.mu.Unlock()

	return WriteResponse(w, CMD_READ_MEM, data)
}

// handleWriteMemory writes memory
func (s *Server) handleWriteMemory(w io.Writer, msg *Message) error {
	if len(msg.Data) < 2 {
		return fmt.Errorf("WRITE_MEM: insufficient data")
	}

	address := DecodeU16LE(msg.Data[0:2])
	data := msg.Data[2:]

	s.mu.Lock()
	s.emulator.WriteMemoryRange(address, data)
	s.mu.Unlock()

	return WriteResponse(w, CMD_WRITE_MEM, []byte{0})
}

// handleGetSlots returns memory slot configuration (for banked machines)
func (s *Server) handleGetSlots(w io.Writer, msg *Message) error {
	// For ZX48K (no banking), return simple 64K layout
	// 4 slots of 16KB each: 0, 1, 2, 3
	response := []byte{0, 1, 2, 3}
	return WriteResponse(w, CMD_GET_SLOTS, response)
}

// handleReadState reads full emulator state (for save states)
func (s *Server) handleReadState(w io.Writer, msg *Message) error {
	// Not fully implemented - return error
	return WriteResponse(w, CMD_READ_STATE, []byte{1}) // Error code 1
}

// runLoop runs the emulator checking for breakpoints
func (s *Server) runLoop() {
	for {
		s.mu.Lock()
		if s.paused {
			s.mu.Unlock()
			return
		}

		// Check for breakpoint at current PC
		pc := s.emulator.GetPC()
		if bp, ok := s.breakpoints[pc]; ok && bp.Enabled && bp.Type == BP_EXEC {
			s.paused = true
			s.mu.Unlock()

			log.Printf("DZRP: Breakpoint hit at $%04X", pc)
			s.sendPauseNotification(fmt.Sprintf("Breakpoint at $%04X", pc))
			return
		}

		// Check halt state
		if s.emulator.IsHalted() {
			s.paused = true
			s.mu.Unlock()

			s.sendPauseNotification("CPU halted")
			return
		}
		s.mu.Unlock()

		// Execute one instruction
		s.emulator.Step()
	}
}

// sendPauseNotification sends a pause notification to DeZog
func (s *Server) sendPauseNotification(reason string) {
	if s.conn == nil {
		return
	}

	// Pause notification format:
	// - 2 bytes: PC
	// - 1 byte: reason length
	// - N bytes: reason string

	s.mu.Lock()
	pc := s.emulator.GetPC()
	s.mu.Unlock()

	data := EncodeU16LE(pc)
	reasonBytes := []byte(reason)
	data = append(data, byte(len(reasonBytes)))
	data = append(data, reasonBytes...)

	WriteNotification(s.conn, NTF_PAUSE, data)
}

// CheckBreakpoint checks if there's a breakpoint at the given address
func (s *Server) CheckBreakpoint(addr uint16) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	bp, ok := s.breakpoints[addr]
	return ok && bp.Enabled
}

// IsPaused returns true if execution is paused
func (s *Server) IsPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}
