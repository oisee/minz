package analysis

import (
	"fmt"

	"github.com/minz/minzc/pkg/disasm"
)

// Analyze performs recursive descent disassembly from all entry points.
// Returns the total number of code bytes classified.
func (a *Analysis) Analyze() int {
	// Apply user overrides first
	a.applyOverrides()

	// Recursive descent from all entry points
	queue := make([]uint16, 0, len(a.EntryPoints)*4)
	for _, ep := range a.EntryPoints {
		if a.InRange(ep) {
			queue = append(queue, ep)
			// Entry points are function starts
			if _, ok := a.Functions[ep]; !ok {
				a.Functions[ep] = &Function{Entry: ep, Name: functionName(ep)}
			}
		}
	}

	visited := make(map[uint16]bool)

	for len(queue) > 0 {
		pc := queue[0]
		queue = queue[1:]

		if visited[pc] {
			continue
		}
		visited[pc] = true

		if !a.InRange(pc) {
			continue
		}

		// Walk a basic block from this address
		addr := pc
		for {
			if !a.InRange(addr) {
				break
			}

			// Check for conflicts
			bc := a.ByteMap[addr]
			if bc == ByteCodeStart {
				// Already analyzed this instruction — stop
				break
			}
			if bc == ByteCode {
				// We hit the middle of an existing instruction — conflict, stop
				break
			}
			if bc == ByteData || bc == ByteString {
				// User/prior forced data — stop
				break
			}

			mem := a.ReadBytes(addr, 4)
			if len(mem) == 0 {
				break
			}

			mnemonic, size, targetAddr, _ := disasm.DisasmFull(mem, addr)
			_ = mnemonic

			// Mark bytes
			a.ByteMap[addr] = ByteCodeStart
			for i := 1; i < size; i++ {
				next := int(addr) + i
				if next > 0xFFFF {
					break
				}
				a.ByteMap[uint16(next)] = ByteCode
			}

			// Update function end address
			a.updateFunctionEnd(addr)

			// Extract data references and xrefs
			a.extractXRefs(mem, addr, size, targetAddr)

			// Determine successors based on instruction type
			op := mem[0]
			nextAddr := uint16(int(addr) + size)

			switch classifyInstruction(op, mem, size) {
			case instrUnconditionalJump:
				// No fall-through, only target
				if targetAddr >= 0 {
					tgt := uint16(targetAddr)
					if a.InRange(tgt) {
						queue = append(queue, tgt)
					}
				}
				// End of basic block
				goto nextBlock

			case instrConditionalJump:
				// Both target and fall-through
				if targetAddr >= 0 {
					tgt := uint16(targetAddr)
					if a.InRange(tgt) {
						queue = append(queue, tgt)
					}
				}
				// Fall through to next instruction
				addr = nextAddr
				continue

			case instrCall:
				// Fall-through; target is a new function
				if targetAddr >= 0 {
					tgt := uint16(targetAddr)
					if a.InRange(tgt) {
						if _, ok := a.Functions[tgt]; !ok {
							a.Functions[tgt] = &Function{Entry: tgt, Name: functionName(tgt)}
						}
						queue = append(queue, tgt)
					}
				}
				addr = nextAddr
				continue

			case instrConditionalCall:
				// Same as call — fall-through + target function
				if targetAddr >= 0 {
					tgt := uint16(targetAddr)
					if a.InRange(tgt) {
						if _, ok := a.Functions[tgt]; !ok {
							a.Functions[tgt] = &Function{Entry: tgt, Name: functionName(tgt)}
						}
						queue = append(queue, tgt)
					}
				}
				addr = nextAddr
				continue

			case instrRST:
				// Like CALL — fall-through + vector target
				if targetAddr >= 0 {
					tgt := uint16(targetAddr)
					if a.InRange(tgt) {
						if _, ok := a.Functions[tgt]; !ok {
							a.Functions[tgt] = &Function{Entry: tgt, Name: functionName(tgt)}
						}
						queue = append(queue, tgt)
					}
				}
				addr = nextAddr
				continue

			case instrReturn:
				// End of basic block, no successors
				goto nextBlock

			case instrHalt:
				// End of basic block
				goto nextBlock

			case instrIndirectJump:
				// JP (HL), JP (IX), JP (IY) — can't resolve target
				goto nextBlock

			case instrConditionalReturn:
				// Fall-through (condition might not be met)
				addr = nextAddr
				continue

			default:
				// Normal instruction — fall through
				addr = nextAddr
				continue
			}
		}
	nextBlock:
	}

	// Compute function sizes
	a.computeFunctionSizes()

	// Count statistics
	a.ComputeStats()
	return a.CodeBytes
}

type instrType int

const (
	instrNormal instrType = iota
	instrUnconditionalJump
	instrConditionalJump
	instrCall
	instrConditionalCall
	instrRST
	instrReturn
	instrConditionalReturn
	instrHalt
	instrIndirectJump
)

// classifyInstruction determines the control flow type of an instruction.
func classifyInstruction(op byte, mem []byte, size int) instrType {
	switch op {
	// Unconditional jumps
	case 0xC3: // JP nn
		return instrUnconditionalJump
	case 0x18: // JR e
		return instrUnconditionalJump

	// Conditional jumps
	case 0xC2, 0xCA, 0xD2, 0xDA, 0xE2, 0xEA, 0xF2, 0xFA: // JP cc,nn
		return instrConditionalJump
	case 0x20, 0x28, 0x30, 0x38: // JR cc,e
		return instrConditionalJump
	case 0x10: // DJNZ
		return instrConditionalJump

	// Unconditional CALL
	case 0xCD: // CALL nn
		return instrCall

	// Conditional CALL
	case 0xC4, 0xCC, 0xD4, 0xDC, 0xE4, 0xEC, 0xF4, 0xFC: // CALL cc,nn
		return instrConditionalCall

	// RST
	case 0xC7, 0xCF, 0xD7, 0xDF, 0xE7, 0xEF, 0xF7, 0xFF:
		return instrRST

	// Unconditional RET
	case 0xC9: // RET
		return instrReturn

	// Conditional RET
	case 0xC0, 0xC8, 0xD0, 0xD8, 0xE0, 0xE8, 0xF0, 0xF8: // RET cc
		return instrConditionalReturn

	// HALT
	case 0x76:
		return instrHalt

	// JP (HL)
	case 0xE9:
		return instrIndirectJump

	// DD/FD prefix — check for JP (IX)/(IY)
	case 0xDD, 0xFD:
		if len(mem) >= 2 && mem[1] == 0xE9 {
			return instrIndirectJump
		}
		return instrNormal

	// ED prefix — check for RETN/RETI
	case 0xED:
		if len(mem) >= 2 {
			switch mem[1] {
			case 0x45: // RETN
				return instrReturn
			case 0x4D: // RETI
				return instrReturn
			}
		}
		return instrNormal
	}

	return instrNormal
}

// extractXRefs records cross-references for an instruction.
func (a *Analysis) extractXRefs(mem []byte, addr uint16, size int, targetAddr int) {
	if len(mem) == 0 {
		return
	}
	op := mem[0]

	// Branch/call xrefs
	if targetAddr >= 0 {
		tgt := uint16(targetAddr)
		switch classifyInstruction(op, mem, size) {
		case instrCall, instrRST:
			a.AddXRef(addr, tgt, XRefCall)
		case instrConditionalCall:
			a.AddXRef(addr, tgt, XRefCall)
		case instrUnconditionalJump:
			a.AddXRef(addr, tgt, XRefJump)
		case instrConditionalJump:
			a.AddXRef(addr, tgt, XRefCondJump)
		}
	}

	// Data references — absolute address operands in non-branch instructions
	a.extractDataRefs(mem, addr)
}

// extractDataRefs identifies absolute addresses in load instructions.
func (a *Analysis) extractDataRefs(mem []byte, addr uint16) {
	if len(mem) < 3 {
		return
	}
	op := mem[0]

	readAddr := func() uint16 {
		return uint16(mem[1]) | uint16(mem[2])<<8
	}

	switch op {
	case 0x3A: // LD A,(nn) — read
		a.AddXRef(addr, readAddr(), XRefRead)
	case 0x32: // LD (nn),A — write
		a.AddXRef(addr, readAddr(), XRefWrite)
	case 0x2A: // LD HL,(nn) — read
		a.AddXRef(addr, readAddr(), XRefRead)
	case 0x22: // LD (nn),HL — write
		a.AddXRef(addr, readAddr(), XRefWrite)

	case 0xED: // ED-prefixed loads
		if len(mem) < 4 {
			return
		}
		edAddr := func() uint16 {
			return uint16(mem[2]) | uint16(mem[3])<<8
		}
		switch mem[1] {
		case 0x43: // LD (nn),BC
			a.AddXRef(addr, edAddr(), XRefWrite)
		case 0x4B: // LD BC,(nn)
			a.AddXRef(addr, edAddr(), XRefRead)
		case 0x53: // LD (nn),DE
			a.AddXRef(addr, edAddr(), XRefWrite)
		case 0x5B: // LD DE,(nn)
			a.AddXRef(addr, edAddr(), XRefRead)
		case 0x63: // LD (nn),HL (ED prefix)
			a.AddXRef(addr, edAddr(), XRefWrite)
		case 0x6B: // LD HL,(nn) (ED prefix)
			a.AddXRef(addr, edAddr(), XRefRead)
		case 0x73: // LD (nn),SP
			a.AddXRef(addr, edAddr(), XRefWrite)
		case 0x7B: // LD SP,(nn)
			a.AddXRef(addr, edAddr(), XRefRead)
		}

	case 0xDD, 0xFD: // DD/FD prefixed LD (nn),IX/IY and LD IX/IY,(nn)
		if len(mem) < 4 {
			return
		}
		ixAddr := func() uint16 {
			return uint16(mem[2]) | uint16(mem[3])<<8
		}
		switch mem[1] {
		case 0x22: // LD (nn),IX/IY
			a.AddXRef(addr, ixAddr(), XRefWrite)
		case 0x2A: // LD IX/IY,(nn)
			a.AddXRef(addr, ixAddr(), XRefRead)
		}
	}
}

// updateFunctionEnd tracks the last instruction address within each function.
func (a *Analysis) updateFunctionEnd(addr uint16) {
	// Find which function this address belongs to (the function whose entry is <= addr)
	var bestFn *Function
	for _, fn := range a.Functions {
		if fn.Entry <= addr {
			if bestFn == nil || fn.Entry > bestFn.Entry {
				bestFn = fn
			}
		}
	}
	if bestFn != nil && addr > bestFn.End {
		bestFn.End = addr
	}
}

// computeFunctionSizes calculates function sizes from entry to end.
func (a *Analysis) computeFunctionSizes() {
	for _, fn := range a.Functions {
		if fn.End >= fn.Entry {
			// Size = from entry to end of last instruction
			// We need the size of the instruction at End
			mem := a.ReadBytes(fn.End, 4)
			if len(mem) > 0 {
				_, size := disasm.Disasm(mem, fn.End)
				fn.Size = int(fn.End-fn.Entry) + size
			}
		}
	}
}

// applyOverrides forces user-specified code/data regions.
func (a *Analysis) applyOverrides() {
	for start, end := range a.CodeOverrides {
		for addr := start; addr <= end && addr >= start; addr++ {
			a.ByteMap[addr] = ByteCodeStart // Will be refined during analysis
		}
	}
	for start, end := range a.DataOverrides {
		for addr := start; addr <= end && addr >= start; addr++ {
			a.ByteMap[addr] = ByteData
		}
	}
}

// functionName generates a default function name for an address.
func functionName(addr uint16) string {
	return fmt.Sprintf("sub_%04X", addr)
}
