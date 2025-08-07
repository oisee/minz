package emulator

import ()

const (
	MEMORY_SIZE = 65536
)

// Z80 represents a Z80 processor emulator
type Z80 struct {
	// Main registers
	A, F   uint8  // Accumulator and Flags
	B, C   uint8  // BC register pair
	D, E   uint8  // DE register pair
	H, L   uint8  // HL register pair
	
	// Alternate registers
	A_, F_ uint8  // Shadow accumulator and flags
	B_, C_ uint8  // Shadow BC
	D_, E_ uint8  // Shadow DE
	H_, L_ uint8  // Shadow HL
	
	// Index registers
	IX, IY uint16
	
	// Special registers
	SP uint16 // Stack pointer
	PC uint16 // Program counter
	I  uint8  // Interrupt vector
	R  uint8  // Memory refresh
	
	// Memory
	memory [MEMORY_SIZE]byte
	
	// State
	cycles   uint32
	halted   bool
	iff1     bool // Interrupt flip-flop 1
	iff2     bool // Interrupt flip-flop 2
	im       uint8 // Interrupt mode
	
	// I/O handlers
	output   []byte
	ioRead   func(port uint8) uint8
	ioWrite  func(port uint8, value uint8)
}

// Registers holds all Z80 registers for inspection
type Registers struct {
	A, F   uint8
	BC     uint16
	DE     uint16
	HL     uint16
	IX, IY uint16
	SP, PC uint16
}

// New creates a new Z80 emulator
func New() *Z80 {
	z := &Z80{}
	z.Reset()
	
	// Default I/O handlers
	z.ioWrite = func(port uint8, value uint8) {
		if port == 0x01 { // Simple console output port
			z.output = append(z.output, value)
		}
	}
	
	z.ioRead = func(port uint8) uint8 {
		return 0xFF // Default: all bits high
	}
	
	return z
}

// Reset resets the processor to initial state
func (z *Z80) Reset() {
	z.A, z.F = 0, 0
	z.B, z.C = 0, 0
	z.D, z.E = 0, 0
	z.H, z.L = 0, 0
	z.A_, z.F_ = 0, 0
	z.B_, z.C_ = 0, 0
	z.D_, z.E_ = 0, 0
	z.H_, z.L_ = 0, 0
	z.IX, z.IY = 0, 0
	z.SP = 0xFFFF
	z.PC = 0
	z.I, z.R = 0, 0
	z.cycles = 0
	z.halted = false
	z.iff1, z.iff2 = false, false
	z.im = 0
	z.output = []byte{}
	
	// Clear memory
	for i := range z.memory {
		z.memory[i] = 0
	}
}

// LoadAt loads code at specified address
func (z *Z80) LoadAt(address uint16, code []byte) {
	for i, b := range code {
		if int(address)+i < len(z.memory) {
			z.memory[int(address)+i] = b
		}
	}
}

// Execute runs code from specified address
func (z *Z80) Execute(address uint16) (string, uint32) {
	z.PC = address
	z.output = []byte{}
	startCycles := z.cycles
	
	// Execute until RET or HALT or max cycles
	maxCycles := uint32(100000)
	for z.cycles-startCycles < maxCycles && !z.halted {
		z.step()
		
		// Check for RET at end of function
		if z.memory[z.PC] == 0xC9 { // RET instruction
			z.step() // Execute the RET
			break
		}
	}
	
	return string(z.output), z.cycles - startCycles
}

// step executes one instruction
func (z *Z80) step() {
	opcode := z.fetchByte()
	
	switch opcode {
	// NOP
	case 0x00:
		z.cycles += 4
		
	// LD r, r'
	case 0x78: // LD A, B
		z.A = z.B
		z.cycles += 4
	case 0x79: // LD A, C
		z.A = z.C
		z.cycles += 4
	case 0x7A: // LD A, D
		z.A = z.D
		z.cycles += 4
	case 0x7B: // LD A, E
		z.A = z.E
		z.cycles += 4
	case 0x7C: // LD A, H
		z.A = z.H
		z.cycles += 4
	case 0x7D: // LD A, L
		z.A = z.L
		z.cycles += 4
		
	// LD r, n
	case 0x3E: // LD A, n
		z.A = z.fetchByte()
		z.cycles += 7
	case 0x06: // LD B, n
		z.B = z.fetchByte()
		z.cycles += 7
	case 0x0E: // LD C, n
		z.C = z.fetchByte()
		z.cycles += 7
	case 0x16: // LD D, n
		z.D = z.fetchByte()
		z.cycles += 7
	case 0x1E: // LD E, n
		z.E = z.fetchByte()
		z.cycles += 7
	case 0x26: // LD H, n
		z.H = z.fetchByte()
		z.cycles += 7
	case 0x2E: // LD L, n
		z.L = z.fetchByte()
		z.cycles += 7
		
	// LD HL, nn
	case 0x21:
		l := z.fetchByte()
		h := z.fetchByte()
		z.H = h
		z.L = l
		z.cycles += 10
		
	// ADD A, r
	case 0x80: // ADD A, B
		z.add(z.B)
		z.cycles += 4
	case 0x81: // ADD A, C
		z.add(z.C)
		z.cycles += 4
	case 0x82: // ADD A, D
		z.add(z.D)
		z.cycles += 4
	case 0x83: // ADD A, E
		z.add(z.E)
		z.cycles += 4
	case 0x84: // ADD A, H
		z.add(z.H)
		z.cycles += 4
	case 0x85: // ADD A, L
		z.add(z.L)
		z.cycles += 4
		
	// ADD A, n
	case 0xC6:
		z.add(z.fetchByte())
		z.cycles += 7
		
	// SUB r
	case 0x90: // SUB B
		z.sub(z.B)
		z.cycles += 4
	case 0x91: // SUB C
		z.sub(z.C)
		z.cycles += 4
		
	// INC r
	case 0x3C: // INC A
		z.A = z.inc(z.A)
		z.cycles += 4
	case 0x04: // INC B
		z.B = z.inc(z.B)
		z.cycles += 4
	case 0x0C: // INC C
		z.C = z.inc(z.C)
		z.cycles += 4
		
	// DEC r
	case 0x3D: // DEC A
		z.A = z.dec(z.A)
		z.cycles += 4
	case 0x05: // DEC B
		z.B = z.dec(z.B)
		z.cycles += 4
		
	// CALL nn
	case 0xCD:
		addr := z.fetchWord()
		z.push(z.PC)
		z.PC = addr
		z.cycles += 17
		
	// RET
	case 0xC9:
		z.PC = z.pop()
		z.cycles += 10
		
	// PUSH rr
	case 0xF5: // PUSH AF
		z.push(uint16(z.A)<<8 | uint16(z.F))
		z.cycles += 11
	case 0xC5: // PUSH BC
		z.push(uint16(z.B)<<8 | uint16(z.C))
		z.cycles += 11
		
	// POP rr
	case 0xF1: // POP AF
		af := z.pop()
		z.A = uint8(af >> 8)
		z.F = uint8(af & 0xFF)
		z.cycles += 10
		
	// JP nn
	case 0xC3:
		z.PC = z.fetchWord()
		z.cycles += 10
		
	// JR n
	case 0x18:
		offset := int8(z.fetchByte())
		z.PC = uint16(int(z.PC) + int(offset))
		z.cycles += 12
		
	// CP n
	case 0xFE:
		z.compare(z.fetchByte())
		z.cycles += 7
		
	// OUT (n), A
	case 0xD3:
		port := z.fetchByte()
		z.ioWrite(port, z.A)
		z.cycles += 11
		
	// IN A, (n)
	case 0xDB:
		port := z.fetchByte()
		z.A = z.ioRead(port)
		z.cycles += 11
		
	// HALT
	case 0x76:
		z.halted = true
		z.cycles += 4
		
	default:
		// Unknown opcode - treat as NOP
		z.cycles += 4
	}
}

// Helper functions

func (z *Z80) fetchByte() uint8 {
	b := z.memory[z.PC]
	z.PC++
	return b
}

func (z *Z80) fetchWord() uint16 {
	l := z.fetchByte()
	h := z.fetchByte()
	return uint16(h)<<8 | uint16(l)
}

func (z *Z80) push(value uint16) {
	z.SP--
	z.memory[z.SP] = uint8(value >> 8)
	z.SP--
	z.memory[z.SP] = uint8(value & 0xFF)
}

func (z *Z80) pop() uint16 {
	l := z.memory[z.SP]
	z.SP++
	h := z.memory[z.SP]
	z.SP++
	return uint16(h)<<8 | uint16(l)
}

func (z *Z80) add(value uint8) {
	result := uint16(z.A) + uint16(value)
	z.setFlag(FLAG_C, result > 0xFF)
	z.setFlag(FLAG_H, (z.A&0xF)+(value&0xF) > 0xF)
	z.A = uint8(result)
	z.setFlag(FLAG_Z, z.A == 0)
	z.setFlag(FLAG_S, z.A&0x80 != 0)
	z.setFlag(FLAG_N, false)
}

func (z *Z80) sub(value uint8) {
	result := int16(z.A) - int16(value)
	z.setFlag(FLAG_C, result < 0)
	z.setFlag(FLAG_H, (z.A&0xF) < (value&0xF))
	z.A = uint8(result)
	z.setFlag(FLAG_Z, z.A == 0)
	z.setFlag(FLAG_S, z.A&0x80 != 0)
	z.setFlag(FLAG_N, true)
}

func (z *Z80) inc(value uint8) uint8 {
	result := value + 1
	z.setFlag(FLAG_Z, result == 0)
	z.setFlag(FLAG_S, result&0x80 != 0)
	z.setFlag(FLAG_H, (value&0xF) == 0xF)
	z.setFlag(FLAG_P, result == 0x80)
	z.setFlag(FLAG_N, false)
	return result
}

func (z *Z80) dec(value uint8) uint8 {
	result := value - 1
	z.setFlag(FLAG_Z, result == 0)
	z.setFlag(FLAG_S, result&0x80 != 0)
	z.setFlag(FLAG_H, (value&0xF) == 0)
	z.setFlag(FLAG_P, value == 0x80)
	z.setFlag(FLAG_N, true)
	return result
}

func (z *Z80) compare(value uint8) {
	result := int16(z.A) - int16(value)
	z.setFlag(FLAG_Z, result == 0)
	z.setFlag(FLAG_S, result < 0)
	z.setFlag(FLAG_C, result < 0)
	z.setFlag(FLAG_N, true)
}

// Flag bit positions
const (
	FLAG_C = 0 // Carry
	FLAG_N = 1 // Add/Subtract
	FLAG_P = 2 // Parity/Overflow
	FLAG_H = 4 // Half Carry
	FLAG_Z = 6 // Zero
	FLAG_S = 7 // Sign
)

func (z *Z80) setFlag(flag uint8, value bool) {
	if value {
		z.F |= (1 << flag)
	} else {
		z.F &^= (1 << flag)
	}
}

func (z *Z80) getFlag(flag uint8) bool {
	return z.F&(1<<flag) != 0
}

// GetRegisters returns current register values
func (z *Z80) GetRegisters() Registers {
	return Registers{
		A:  z.A,
		F:  z.F,
		BC: uint16(z.B)<<8 | uint16(z.C),
		DE: uint16(z.D)<<8 | uint16(z.E),
		HL: uint16(z.H)<<8 | uint16(z.L),
		IX: z.IX,
		IY: z.IY,
		SP: z.SP,
		PC: z.PC,
	}
}

// GetIFF1 returns interrupt flip-flop 1 state
func (z *Z80) GetIFF1() bool {
	return z.iff1
}

// GetIFF2 returns interrupt flip-flop 2 state
func (z *Z80) GetIFF2() bool {
	return z.iff2
}

// GetIM returns interrupt mode
func (z *Z80) GetIM() uint8 {
	return z.im
}

// ReadMemory reads a byte from memory
func (z *Z80) ReadMemory(address uint16) uint8 {
	return z.memory[address]
}

// WriteMemory writes a byte to memory
func (z *Z80) WriteMemory(address uint16, value uint8) {
	z.memory[address] = value
}

// DumpMemory returns memory contents for debugging
func (z *Z80) DumpMemory(start uint16, length uint16) []byte {
	result := make([]byte, length)
	for i := uint16(0); i < length; i++ {
		addr := start + i
		// Check for overflow and bounds
		if addr >= start && int(addr) < MEMORY_SIZE {
			result[i] = z.memory[addr]
		}
	}
	return result
}

// IsHalted returns true if CPU is halted
func (z *Z80) IsHalted() bool {
	return z.halted
}

// SetHalted sets the halted state
func (z *Z80) SetHalted(halted bool) {
	z.halted = halted
}