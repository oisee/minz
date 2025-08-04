package tas

import (
	"fmt"
)

// Z80 Instruction timing tables
// Based on official Zilog Z80 documentation

// CyclePerfectRecorder provides cycle-accurate event recording
type CyclePerfectRecorder struct {
	currentCycle   int64
	lastEventCycle int64
	events         []CycleEvent
	
	// Determinism tracking
	deterministicStart int64
	inDeterministic    bool
	ioFreeStretch      int64
	
	// Instruction timing cache
	timingTable map[byte]int
}

// CycleEvent represents any event at a specific cycle
type CycleEvent struct {
	Cycle     int64
	Type      EventType
	Data      interface{}
	TStates   int // How many T-states this event took
}

type EventType byte

const (
	EventInstruction EventType = iota
	EventMemoryRead
	EventMemoryWrite
	EventIORead
	EventIOWrite
	EventInterrupt
	EventSMC
)

// NewCyclePerfectRecorder creates a new cycle-perfect recorder
func NewCyclePerfectRecorder() *CyclePerfectRecorder {
	return &CyclePerfectRecorder{
		events:      make([]CycleEvent, 0, 10000),
		timingTable: buildTimingTable(),
	}
}

// RecordInstruction records a CPU instruction with cycle-perfect timing
func (r *CyclePerfectRecorder) RecordInstruction(opcode byte, pc uint16, cycles int) {
	event := CycleEvent{
		Cycle:   r.currentCycle,
		Type:    EventInstruction,
		TStates: cycles,
		Data: InstructionData{
			Opcode: opcode,
			PC:     pc,
		},
	}
	
	r.events = append(r.events, event)
	r.currentCycle += int64(cycles)
	
	// Update determinism tracking
	r.updateDeterminism(EventInstruction)
}

// RecordMemoryAccess records memory read/write with timing
func (r *CyclePerfectRecorder) RecordMemoryAccess(addr uint16, value byte, isWrite bool, cycles int) {
	eventType := EventMemoryRead
	if isWrite {
		eventType = EventMemoryWrite
	}
	
	event := CycleEvent{
		Cycle:   r.currentCycle,
		Type:    eventType,
		TStates: cycles,
		Data: MemoryAccessData{
			Address: addr,
			Value:   value,
			IsWrite: isWrite,
		},
	}
	
	r.events = append(r.events, event)
	r.currentCycle += int64(cycles)
	
	// Check for SMC
	if isWrite && r.isCodeAddress(addr) {
		r.recordSMC(addr, value)
	}
}

// RecordIO records I/O operations with cycle timing
func (r *CyclePerfectRecorder) RecordIO(port uint16, value byte, isInput bool, cycles int) {
	eventType := EventIORead
	if !isInput {
		eventType = EventIOWrite
	}
	
	event := CycleEvent{
		Cycle:   r.currentCycle,
		Type:    eventType,
		TStates: cycles,
		Data: IOData{
			Port:    port,
			Value:   value,
			IsInput: isInput,
		},
	}
	
	r.events = append(r.events, event)
	r.currentCycle += int64(cycles)
	
	// I/O breaks determinism
	r.updateDeterminism(eventType)
}

// RecordInterrupt records interrupt acceptance with timing
func (r *CyclePerfectRecorder) RecordInterrupt(mode byte, vector byte, cycles int) {
	event := CycleEvent{
		Cycle:   r.currentCycle,
		Type:    EventInterrupt,
		TStates: cycles,
		Data: InterruptData{
			Mode:   mode,
			Vector: vector,
		},
	}
	
	r.events = append(r.events, event)
	r.currentCycle += int64(cycles)
	
	// Interrupts break determinism
	r.updateDeterminism(EventInterrupt)
}

// updateDeterminism tracks deterministic execution sections
func (r *CyclePerfectRecorder) updateDeterminism(eventType EventType) {
	switch eventType {
	case EventIORead, EventIOWrite, EventInterrupt:
		// These events break determinism
		if r.inDeterministic {
			// End deterministic section
			stretch := r.currentCycle - r.deterministicStart
			if stretch > 1000 { // Only record significant stretches
				r.recordDeterministicSection(r.deterministicStart, stretch)
			}
			r.inDeterministic = false
			r.ioFreeStretch = 0
		}
		
	case EventInstruction, EventMemoryRead, EventMemoryWrite:
		// These can be deterministic
		if !r.inDeterministic {
			r.ioFreeStretch++
			if r.ioFreeStretch > 100 { // After 100 cycles without I/O
				r.inDeterministic = true
				r.deterministicStart = r.currentCycle - r.ioFreeStretch
			}
		}
	}
}

// recordDeterministicSection marks a deterministic execution section
func (r *CyclePerfectRecorder) recordDeterministicSection(start, length int64) {
	// In deterministic sections, we don't need to record every instruction
	// Just the boundaries and any SMC events
	fmt.Printf("Deterministic section: cycles %d-%d (%d cycles)\n", 
		start, start+length, length)
}

// recordSMC records self-modifying code event
func (r *CyclePerfectRecorder) recordSMC(addr uint16, newValue byte) {
	// SMC events must always be recorded even in deterministic sections
	event := CycleEvent{
		Cycle: r.currentCycle,
		Type:  EventSMC,
		Data: SMCData{
			Address:  addr,
			NewValue: newValue,
		},
	}
	r.events = append(r.events, event)
}

// isCodeAddress checks if address is in code segment
func (r *CyclePerfectRecorder) isCodeAddress(addr uint16) bool {
	// Simple heuristic: code is typically in lower memory
	// This should be configured based on actual memory map
	return addr >= 0x8000 && addr < 0xC000
}

// GetCompressionRatio returns the compression ratio achieved
func (r *CyclePerfectRecorder) GetCompressionRatio() float64 {
	if r.currentCycle == 0 {
		return 0
	}
	
	// Calculate theoretical uncompressed size
	uncompressedSize := r.currentCycle * 67 // Each cycle = full state
	
	// Calculate actual compressed size
	compressedSize := int64(len(r.events) * 8) // Rough estimate
	
	return float64(uncompressedSize) / float64(compressedSize)
}

// Data structures for different event types

type InstructionData struct {
	Opcode byte
	PC     uint16
}

type MemoryAccessData struct {
	Address uint16
	Value   byte
	IsWrite bool
}

type IOData struct {
	Port    uint16
	Value   byte
	IsInput bool
}

type InterruptData struct {
	Mode   byte
	Vector byte
}

type SMCData struct {
	Address  uint16
	NewValue byte
}

// buildTimingTable creates Z80 instruction timing table
func buildTimingTable() map[byte]int {
	// TODO: Integrate with actual Z80 emulator for accurate timing
	// For now, use simplified timing (4 T-cycles default)
	timing := make(map[byte]int)
	for i := 0; i < 256; i++ {
		timing[byte(i)] = 4 // Default 4 T-cycles
	}
	
	// Override some common instructions with known timing
	timing[0x00] = 4  // NOP
	timing[0x10] = 8  // DJNZ (no jump)
	timing[0xC3] = 10 // JP nn
	timing[0xC9] = 10 // RET
	timing[0xCD] = 17 // CALL nn
	
	return timing
}

// GetInstructionTiming returns T-state count for instruction
// TODO: Replace with actual emulator integration
func GetInstructionTiming(opcode byte, operands []byte, conditions bool) int {
	// Simplified - will be replaced with emulator's actual timing
	base := 4 // Default 4 T-cycles
	
	// Handle conditional instructions
	switch opcode {
	case 0x10: // DJNZ
		if conditions { // Branch taken
			return 13
		}
		return 8
		
	case 0x20, 0x28, 0x30, 0x38: // JR cc,e
		if conditions { // Branch taken
			return 12
		}
		return 7
		
	case 0xC0, 0xC8, 0xD0, 0xD8, 0xE0, 0xE8, 0xF0, 0xF8: // RET cc
		if conditions { // Return taken
			return 11
		}
		return 5
		
	case 0xC2, 0xCA, 0xD2, 0xDA, 0xE2, 0xEA, 0xF2, 0xFA: // JP cc,nn
		if conditions { // Jump taken
			return 10
		}
		return 10 // Same timing either way
		
	case 0xC4, 0xCC, 0xD4, 0xDC, 0xE4, 0xEC, 0xF4, 0xFC: // CALL cc,nn
		if conditions { // Call taken
			return 17
		}
		return 10
	}
	
	// Handle prefixed instructions
	if opcode == 0xCB {
		return 8 + getBitInstructionTiming(operands[0])
	}
	if opcode == 0xDD || opcode == 0xFD {
		return 4 + getIndexedTiming(operands)
	}
	if opcode == 0xED {
		return getExtendedTiming(operands[0])
	}
	
	return base
}

// Standard instruction timings (no prefix)
var standardTimings = [256]int{
	// 0x00-0x0F
	4, 10, 7, 6, 4, 4, 7, 4, 4, 11, 7, 6, 4, 4, 7, 4,
	// 0x10-0x1F
	8, 10, 7, 6, 4, 4, 7, 4, 12, 11, 7, 6, 4, 4, 7, 4,
	// 0x20-0x2F
	7, 10, 16, 6, 4, 4, 7, 4, 7, 11, 16, 6, 4, 4, 7, 4,
	// 0x30-0x3F
	7, 10, 13, 6, 11, 11, 7, 4, 7, 11, 13, 6, 4, 4, 7, 4,
	// 0x40-0x4F
	4, 4, 4, 4, 4, 4, 7, 4, 4, 4, 4, 4, 4, 4, 7, 4,
	// 0x50-0x5F
	4, 4, 4, 4, 4, 4, 7, 4, 4, 4, 4, 4, 4, 4, 7, 4,
	// 0x60-0x6F
	4, 4, 4, 4, 4, 4, 7, 4, 4, 4, 4, 4, 4, 4, 7, 4,
	// 0x70-0x7F
	7, 7, 7, 7, 7, 7, 4, 7, 4, 4, 4, 4, 4, 4, 7, 4,
	// 0x80-0x8F
	4, 4, 4, 4, 4, 4, 7, 4, 4, 4, 4, 4, 4, 4, 7, 4,
	// 0x90-0x9F
	4, 4, 4, 4, 4, 4, 7, 4, 4, 4, 4, 4, 4, 4, 7, 4,
	// 0xA0-0xAF
	4, 4, 4, 4, 4, 4, 7, 4, 4, 4, 4, 4, 4, 4, 7, 4,
	// 0xB0-0xBF
	4, 4, 4, 4, 4, 4, 7, 4, 4, 4, 4, 4, 4, 4, 7, 4,
	// 0xC0-0xCF
	5, 10, 10, 10, 10, 11, 7, 11, 5, 10, 10, 8, 10, 17, 7, 11,
	// 0xD0-0xDF
	5, 10, 10, 11, 10, 11, 7, 11, 5, 4, 10, 11, 10, 8, 7, 11,
	// 0xE0-0xEF
	5, 10, 10, 19, 10, 11, 7, 11, 5, 4, 10, 4, 10, 8, 7, 11,
	// 0xF0-0xFF
	5, 10, 10, 4, 10, 11, 7, 11, 5, 6, 10, 4, 10, 8, 7, 11,
}

func getBitInstructionTiming(opcode byte) int {
	// CB-prefixed bit instructions
	// Most are 8 cycles, (HL) variants are 15
	if (opcode & 0x07) == 0x06 {
		return 15 // Bit operations on (HL)
	}
	return 8
}

func getIndexedTiming(operands []byte) int {
	// DD/FD-prefixed indexed instructions
	// Add timing for displacement calculation
	return 19 // Typical indexed instruction
}

func getExtendedTiming(opcode byte) int {
	// ED-prefixed extended instructions
	switch opcode {
	case 0x40, 0x48, 0x50, 0x58, 0x60, 0x68, 0x78: // IN r,(C)
		return 12
	case 0x41, 0x49, 0x51, 0x59, 0x61, 0x69, 0x79: // OUT (C),r
		return 12
	case 0x42, 0x52, 0x62, 0x72: // SBC HL,rr
		return 15
	case 0x43, 0x53, 0x63, 0x73: // LD (nn),rr
		return 20
	case 0x4A, 0x5A, 0x6A, 0x7A: // ADC HL,rr
		return 15
	case 0x4B, 0x5B, 0x6B, 0x7B: // LD rr,(nn)
		return 20
	case 0x44, 0x4C, 0x54, 0x5C, 0x64, 0x6C, 0x74, 0x7C: // NEG
		return 8
	case 0x45, 0x4D, 0x55, 0x5D, 0x65, 0x6D, 0x75, 0x7D: // RETN/RETI
		return 14
	case 0x46, 0x4E, 0x66, 0x6E: // IM n
		return 8
	case 0x47, 0x4F, 0x57, 0x5F, 0x67, 0x6F, 0x77, 0x7F: // LD I,A / LD R,A / LD A,I / LD A,R
		return 9
	case 0x56, 0x76: // IM 1
		return 8
	case 0x5E, 0x7E: // IM 2
		return 8
	case 0xA0: // LDI
		return 16
	case 0xA1: // CPI
		return 16
	case 0xA2: // INI
		return 16
	case 0xA3: // OUTI
		return 16
	case 0xA8: // LDD
		return 16
	case 0xA9: // CPD
		return 16
	case 0xAA: // IND
		return 16
	case 0xAB: // OUTD
		return 16
	case 0xB0: // LDIR
		return 21 // If BC != 0, else 16
	case 0xB1: // CPIR
		return 21 // If BC != 0 and no match, else 16
	case 0xB2: // INIR
		return 21 // If B != 0, else 16
	case 0xB3: // OTIR
		return 21 // If B != 0, else 16
	case 0xB8: // LDDR
		return 21 // If BC != 0, else 16
	case 0xB9: // CPDR
		return 21 // If BC != 0 and no match, else 16
	case 0xBA: // INDR
		return 21 // If B != 0, else 16
	case 0xBB: // OTDR
		return 21 // If B != 0, else 16
	default:
		return 8
	}
}