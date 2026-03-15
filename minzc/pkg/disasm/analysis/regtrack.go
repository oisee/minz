package analysis

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/disasm"
)

// RegSet is a bitmask representing a set of Z80 registers.
type RegSet uint16

const (
	RegA RegSet = 1 << iota
	RegF
	RegB
	RegC
	RegD
	RegE
	RegH
	RegL

	RegBC RegSet = RegB | RegC
	RegDE RegSet = RegD | RegE
	RegHL RegSet = RegH | RegL
	RegAF RegSet = RegA | RegF
)

// FuncRegInfo holds the register usage summary for a function.
type FuncRegInfo struct {
	In      RegSet // Registers read before being written (inputs)
	Out     RegSet // Registers written near RET (likely outputs)
	Clobber RegSet // Registers modified but not preserved
}

// FormatRegSet returns a human-readable register list like "A, BC, HL".
// Groups pairs when both halves are present.
func FormatRegSet(rs RegSet) string {
	if rs == 0 {
		return "\u2014"
	}
	var parts []string

	// Check pairs first
	if rs&RegHL == RegHL {
		parts = append(parts, "HL")
		rs &^= RegHL
	}
	if rs&RegBC == RegBC {
		parts = append(parts, "BC")
		rs &^= RegBC
	}
	if rs&RegDE == RegDE {
		parts = append(parts, "DE")
		rs &^= RegDE
	}

	// Remaining individual registers
	if rs&RegA != 0 {
		parts = append(parts, "A")
	}
	if rs&RegF != 0 {
		parts = append(parts, "F")
	}
	if rs&RegB != 0 {
		parts = append(parts, "B")
	}
	if rs&RegC != 0 {
		parts = append(parts, "C")
	}
	if rs&RegD != 0 {
		parts = append(parts, "D")
	}
	if rs&RegE != 0 {
		parts = append(parts, "E")
	}
	if rs&RegH != 0 {
		parts = append(parts, "H")
	}
	if rs&RegL != 0 {
		parts = append(parts, "L")
	}

	return strings.Join(parts, ", ")
}

// FormatFuncRegInfo returns the annotation string like "IN: A, HL  OUT: A  CLOBBER: BC".
func FormatFuncRegInfo(info *FuncRegInfo) string {
	if info == nil {
		return ""
	}
	return fmt.Sprintf("IN: %s  OUT: %s  CLOBBER: %s",
		FormatRegSet(info.In),
		FormatRegSet(info.Out),
		FormatRegSet(info.Clobber))
}

// Register indices for provenance tracking.
const (
	rA = iota
	rF
	rB
	rC
	rD
	rE
	rH
	rL
	nRegs
)

// regBit maps register index to RegSet bit.
var regBit = [nRegs]RegSet{RegA, RegF, RegB, RegC, RegD, RegE, RegH, RegL}

// regIdx maps a single-bit RegSet to its index, or -1.
func regIdx(r RegSet) int {
	for i := 0; i < nRegs; i++ {
		if regBit[i] == r {
			return i
		}
	}
	return -1
}

// z80BitsToIdx maps Z80's 3-bit register encoding to our index.
// Returns -1 for (HL) encoding (6).
func z80BitsToIdx(bits byte) int {
	switch bits & 7 {
	case 0:
		return rB
	case 1:
		return rC
	case 2:
		return rD
	case 3:
		return rE
	case 4:
		return rH
	case 5:
		return rL
	case 7:
		return rA
	}
	return -1 // 6 = (HL)
}

// isRegToRegMove checks if opcode is LD r,r' (both physical regs, not (HL)).
// Returns source and destination register indices, or (-1,-1,false).
func isRegToRegMove(op byte) (src, dst int, ok bool) {
	if op < 0x40 || op > 0x7F || op == 0x76 {
		return -1, -1, false
	}
	srcBits := op & 7
	dstBits := (op >> 3) & 7
	if srcBits == 6 || dstBits == 6 {
		return -1, -1, false
	}
	return z80BitsToIdx(srcBits), z80BitsToIdx(dstBits), true
}

// AnalyzeRegisters computes IN/OUT/CLOBBER for all detected functions.
func (a *Analysis) AnalyzeRegisters() {
	a.FuncRegs = make(map[uint16]*FuncRegInfo)

	for _, fn := range a.Functions {
		info := a.analyzeFuncRegisters(fn)
		a.FuncRegs[fn.Entry] = info
	}
}

func (a *Analysis) analyzeFuncRegisters(fn *Function) *FuncRegInfo {
	if fn.Size == 0 {
		return &FuncRegInfo{}
	}

	// Provenance tracking: prov[i] = the entry-state register bit whose value
	// is currently in physical register i. Zero means "computed/loaded, not
	// from entry state."
	var prov [nRegs]RegSet
	for i := 0; i < nRegs; i++ {
		prov[i] = regBit[i] // initially each reg holds its own entry value
	}

	var input RegSet    // collected function inputs (entry-state registers)
	var modified RegSet // all registers written anywhere

	// PUSH/POP tracking for preservation detection.
	type pushEntry struct {
		reg  RegSet
		prov [nRegs]RegSet // provenance snapshot at PUSH time
	}
	var pushStack []pushEntry
	preserved := RegSet(0)

	// Helper: consume registers in mask — record their provenance as inputs.
	consume := func(mask RegSet) {
		for i := 0; i < nRegs; i++ {
			if mask&regBit[i] != 0 && prov[i] != 0 {
				input |= prov[i]
			}
		}
	}

	// Helper: produce (overwrite) registers in mask — clear their provenance.
	produce := func(mask RegSet) {
		for i := 0; i < nRegs; i++ {
			if mask&regBit[i] != 0 {
				prov[i] = 0
			}
		}
	}

	endAddr := fn.Entry + uint16(fn.Size)
	addr := fn.Entry

	for addr < endAddr && addr >= fn.Entry {
		if a.ByteMap[addr] != ByteCodeStart {
			addr++
			continue
		}

		mem := a.ReadBytes(addr, 4)
		if len(mem) == 0 {
			break
		}
		_, size := disasm.Disasm(mem, addr)
		if size == 0 {
			break
		}

		op := mem[0]
		itype := classifyInstruction(op, mem, size)

		// --- Special cases: provenance transfers (no consumption) ---

		// EX DE,HL: swap provenance, don't consume
		if op == 0xEB {
			prov[rD], prov[rH] = prov[rH], prov[rD]
			prov[rE], prov[rL] = prov[rL], prov[rE]
			modified |= RegDE | RegHL
			addr += uint16(size)
			continue
		}

		// LD r, r' (reg-to-reg move): transfer provenance
		if src, dst, ok := isRegToRegMove(op); ok {
			// Source is consumed only if it goes to a different register
			// Actually no — it's a copy, not consumption. Transfer provenance.
			prov[dst] = prov[src]
			modified |= regBit[dst]
			addr += uint16(size)
			continue
		}

		// PUSH: save provenance to stack
		switch op {
		case 0xC5: // PUSH BC
			pushStack = append(pushStack, pushEntry{reg: RegBC, prov: prov})
		case 0xD5: // PUSH DE
			pushStack = append(pushStack, pushEntry{reg: RegDE, prov: prov})
		case 0xE5: // PUSH HL
			pushStack = append(pushStack, pushEntry{reg: RegHL, prov: prov})
		case 0xF5: // PUSH AF
			pushStack = append(pushStack, pushEntry{reg: RegAF, prov: prov})
		}

		// POP: restore provenance if matched PUSH
		switch op {
		case 0xC1: // POP BC
			if n := len(pushStack); n > 0 && pushStack[n-1].reg == RegBC {
				prov[rB] = pushStack[n-1].prov[rB]
				prov[rC] = pushStack[n-1].prov[rC]
				preserved |= RegBC
				pushStack = pushStack[:n-1]
				modified |= RegBC
				addr += uint16(size)
				continue
			}
		case 0xD1: // POP DE
			if n := len(pushStack); n > 0 && pushStack[n-1].reg == RegDE {
				prov[rD] = pushStack[n-1].prov[rD]
				prov[rE] = pushStack[n-1].prov[rE]
				preserved |= RegDE
				pushStack = pushStack[:n-1]
				modified |= RegDE
				addr += uint16(size)
				continue
			}
		case 0xE1: // POP HL
			if n := len(pushStack); n > 0 && pushStack[n-1].reg == RegHL {
				prov[rH] = pushStack[n-1].prov[rH]
				prov[rL] = pushStack[n-1].prov[rL]
				preserved |= RegHL
				pushStack = pushStack[:n-1]
				modified |= RegHL
				addr += uint16(size)
				continue
			}
		case 0xF1: // POP AF
			if n := len(pushStack); n > 0 && pushStack[n-1].reg == RegAF {
				prov[rA] = pushStack[n-1].prov[rA]
				prov[rF] = pushStack[n-1].prov[rF]
				preserved |= RegAF
				pushStack = pushStack[:n-1]
				modified |= RegAF
				addr += uint16(size)
				continue
			}
		}

		// --- CALL/RST: ABI-aware consumption ---
		if itype == instrCall || itype == instrConditionalCall || itype == instrRST {
			consumed := a.calleeConsumedRegs(addr, mem)
			consume(consumed)
			// Clobber all after call
			callClobber := RegA | RegF | RegBC | RegDE | RegHL
			produce(callClobber)
			modified |= callClobber
			addr += uint16(size)
			continue
		}

		// --- Normal instructions: consume reads, produce writes ---
		reads, writes := instrRegUsage(mem)
		consume(reads)
		produce(writes)
		modified |= writes

		addr += uint16(size)
	}

	// Detect output registers: scan backward from each RET
	out := a.detectOutputRegs(fn)

	// Remove preserved registers from clobber
	clobber := modified &^ input &^ out &^ preserved

	// Remove F from display if it's the only thing (almost always clobbered)
	if clobber == RegF {
		clobber = 0
	}

	return &FuncRegInfo{
		In:      input,
		Out:     out,
		Clobber: clobber,
	}
}

// calleeConsumedRegs determines which registers a CALL/RST at callAddr
// actually reads. Uses ABI profiles for known syscalls, falls back to
// consuming all registers with non-identity provenance for unknown callees.
func (a *Analysis) calleeConsumedRegs(callAddr uint16, mem []byte) RegSet {
	allRegs := RegA | RegF | RegBC | RegDE | RegHL

	profile := GetABIProfile(a.Platform)
	if profile == nil {
		return allRegs
	}

	// Get call target
	_, _, targetAddr, _ := disasm.DisasmFull(mem, callAddr)
	if targetAddr < 0 {
		return allRegs
	}
	target := uint16(targetAddr)

	for _, entry := range profile.Entries {
		if entry.Address != target {
			continue
		}

		// Direct (non-dispatched) call
		if entry.DispatchReg == "" {
			consumed := RegSet(0)
			for _, p := range entry.DirectParams {
				consumed |= regNameToSet(p.Reg)
			}
			if consumed == 0 {
				return allRegs // no param info → conservative
			}
			return consumed
		}

		// Dispatched call: resolve function number
		funcNum, found := a.scanBackward(callAddr, entry.DispatchReg, 8)
		if !found {
			return allRegs
		}
		def, ok := entry.Functions[funcNum]
		if !ok {
			return allRegs
		}

		// Consume dispatch register + function params
		consumed := regNameToSet(entry.DispatchReg)
		for _, p := range def.Params {
			consumed |= regNameToSet(p.Reg)
		}
		return consumed
	}

	return allRegs
}

// regNameToSet converts a register name string to a RegSet.
func regNameToSet(name string) RegSet {
	switch name {
	case "A":
		return RegA
	case "F":
		return RegF
	case "B":
		return RegB
	case "C":
		return RegC
	case "D":
		return RegD
	case "E":
		return RegE
	case "H":
		return RegH
	case "L":
		return RegL
	case "BC":
		return RegBC
	case "DE":
		return RegDE
	case "HL":
		return RegHL
	case "AF":
		return RegAF
	}
	return 0
}

// detectOutputRegs scans backward from RET instructions to find
// registers most recently written — likely return values.
func (a *Analysis) detectOutputRegs(fn *Function) RegSet {
	endAddr := fn.Entry + uint16(fn.Size)
	var out RegSet

	// Find all RET instructions in the function
	addr := fn.Entry
	for addr < endAddr && addr >= fn.Entry {
		if a.ByteMap[addr] != ByteCodeStart {
			addr++
			continue
		}
		mem := a.ReadBytes(addr, 4)
		if len(mem) == 0 {
			break
		}
		_, size := disasm.Disasm(mem, addr)
		if size == 0 {
			break
		}

		op := mem[0]
		isRet := (op == 0xC9) // RET
		// Also detect conditional RET as potential return points
		if op == 0xC0 || op == 0xC8 || op == 0xD0 || op == 0xD8 ||
			op == 0xE0 || op == 0xE8 || op == 0xF0 || op == 0xF8 {
			isRet = true
		}
		// ED 45 = RETN, ED 4D = RETI
		if op == 0xED && len(mem) >= 2 && (mem[1] == 0x45 || mem[1] == 0x4D) {
			isRet = true
		}

		if isRet {
			out |= a.scanBackwardForWrites(addr, fn.Entry, 4)
		}

		addr += uint16(size)
	}
	return out
}

// scanBackwardForWrites walks up to maxInstr instructions backward from addr
// and collects registers that were written. Stops at branches or calls.
func (a *Analysis) scanBackwardForWrites(addr, minAddr uint16, maxInstr int) RegSet {
	var result RegSet
	cur := addr

	for i := 0; i < maxInstr; i++ {
		prev := a.findPrevInstruction(cur)
		if prev >= cur || prev < minAddr {
			break
		}
		cur = prev

		mem := a.ReadBytes(cur, 4)
		if len(mem) == 0 {
			break
		}

		op := mem[0]
		_, size := disasm.Disasm(mem, cur)

		// Stop at control flow boundaries
		itype := classifyInstruction(op, mem, size)
		switch itype {
		case instrCall, instrConditionalCall, instrRST,
			instrUnconditionalJump, instrConditionalJump:
			return result
		}

		_, writes := instrRegUsage(mem)
		result |= writes
	}
	return result
}

// instrRegUsage returns which registers an instruction reads and writes.
func instrRegUsage(mem []byte) (reads, writes RegSet) {
	if len(mem) == 0 {
		return 0, 0
	}
	op := mem[0]

	switch {
	// NOP
	case op == 0x00:
		return 0, 0

	// LD rr, nn (16-bit immediate)
	case op == 0x01:
		return 0, RegBC
	case op == 0x11:
		return 0, RegDE
	case op == 0x21:
		return 0, RegHL
	case op == 0x31:
		return 0, 0 // SP

	// LD (rr), A / LD A, (rr)
	case op == 0x02:
		return RegBC | RegA, 0
	case op == 0x12:
		return RegDE | RegA, 0
	case op == 0x0A:
		return RegBC, RegA
	case op == 0x1A:
		return RegDE, RegA

	// INC/DEC rr (16-bit, no flags)
	case op == 0x03:
		return RegBC, RegBC
	case op == 0x13:
		return RegDE, RegDE
	case op == 0x23:
		return RegHL, RegHL
	case op == 0x33:
		return 0, 0
	case op == 0x0B:
		return RegBC, RegBC
	case op == 0x1B:
		return RegDE, RegDE
	case op == 0x2B:
		return RegHL, RegHL
	case op == 0x3B:
		return 0, 0

	// ADD HL, rr
	case op == 0x09:
		return RegHL | RegBC, RegHL | RegF
	case op == 0x19:
		return RegHL | RegDE, RegHL | RegF
	case op == 0x29:
		return RegHL, RegHL | RegF
	case op == 0x39:
		return RegHL, RegHL | RegF

	// INC/DEC r (8-bit)
	case op&0xC7 == 0x04: // INC r
		r := regFromBits(op >> 3)
		if r == 0 {
			return RegHL, RegF
		} // INC (HL)
		return r, r | RegF
	case op&0xC7 == 0x05: // DEC r
		r := regFromBits(op >> 3)
		if r == 0 {
			return RegHL, RegF
		}
		return r, r | RegF

	// LD r, n (8-bit immediate)
	case op&0xC7 == 0x06:
		r := regFromBits(op >> 3)
		if r == 0 {
			return RegHL, 0
		} // LD (HL),n
		return 0, r

	// Rotate A
	case op == 0x07, op == 0x0F, op == 0x17, op == 0x1F:
		return RegA, RegA | RegF

	// EX AF,AF'
	case op == 0x08:
		return RegAF, RegAF

	// DAA, CPL, SCF, CCF
	case op == 0x27:
		return RegA | RegF, RegA | RegF
	case op == 0x2F:
		return RegA, RegA | RegF
	case op == 0x37:
		return 0, RegF
	case op == 0x3F:
		return RegF, RegF

	// HALT
	case op == 0x76:
		return 0, 0

	// LD r, r' (0x40-0x7F)
	case op >= 0x40 && op <= 0x7F:
		src := regFromBits(op)
		dst := regFromBits(op >> 3)
		if src == 0 && dst == 0 {
			return RegHL, 0
		} // LD (HL),(HL) doesn't exist but handle gracefully
		if src == 0 {
			return RegHL, dst
		} // LD r,(HL)
		if dst == 0 {
			return RegHL | src, 0
		} // LD (HL),r
		return src, dst

	// ALU A, r (0x80-0xBF)
	case op >= 0x80 && op <= 0xBF:
		src := regFromBits(op)
		rd := RegA
		if src == 0 {
			rd |= RegHL
		} else {
			rd |= src
		}
		if (op>>3)&7 == 7 { // CP — only writes F
			return rd, RegF
		}
		return rd, RegA | RegF

	// ALU A, n (immediate)
	case op == 0xC6, op == 0xCE, op == 0xD6, op == 0xDE,
		op == 0xE6, op == 0xEE, op == 0xF6:
		return RegA, RegA | RegF
	case op == 0xFE: // CP n
		return RegA, RegF

	// LD A,(nn), LD (nn),A, LD HL,(nn), LD (nn),HL
	case op == 0x3A:
		return 0, RegA
	case op == 0x32:
		return RegA, 0
	case op == 0x2A:
		return 0, RegHL
	case op == 0x22:
		return RegHL, 0

	// PUSH
	case op == 0xC5:
		return RegBC, 0
	case op == 0xD5:
		return RegDE, 0
	case op == 0xE5:
		return RegHL, 0
	case op == 0xF5:
		return RegAF, 0

	// POP
	case op == 0xC1:
		return 0, RegBC
	case op == 0xD1:
		return 0, RegDE
	case op == 0xE1:
		return 0, RegHL
	case op == 0xF1:
		return 0, RegAF

	// EX DE,HL
	case op == 0xEB:
		return RegDE | RegHL, RegDE | RegHL
	// EX (SP),HL
	case op == 0xE3:
		return RegHL, RegHL
	// EXX
	case op == 0xD9:
		return RegBC | RegDE | RegHL, RegBC | RegDE | RegHL

	// JP, JR (no register effects)
	case op == 0xC3, op == 0x18:
		return 0, 0

	// DJNZ
	case op == 0x10:
		return RegB, RegB

	// Conditional JP/JR — read F
	case op == 0xC2, op == 0xCA, op == 0xD2, op == 0xDA,
		op == 0xE2, op == 0xEA, op == 0xF2, op == 0xFA:
		return RegF, 0
	case op == 0x20, op == 0x28, op == 0x30, op == 0x38:
		return RegF, 0

	// CALL nn — register effects handled separately (callee clobber)
	case op == 0xCD:
		return 0, 0
	// Conditional CALL
	case op == 0xC4, op == 0xCC, op == 0xD4, op == 0xDC,
		op == 0xE4, op == 0xEC, op == 0xF4, op == 0xFC:
		return RegF, 0

	// RET
	case op == 0xC9:
		return 0, 0
	// Conditional RET
	case op == 0xC0, op == 0xC8, op == 0xD0, op == 0xD8,
		op == 0xE0, op == 0xE8, op == 0xF0, op == 0xF8:
		return RegF, 0

	// RST — like CALL
	case op == 0xC7, op == 0xCF, op == 0xD7, op == 0xDF,
		op == 0xE7, op == 0xEF, op == 0xF7, op == 0xFF:
		return 0, 0

	// IN A,(n), OUT (n),A
	case op == 0xDB:
		return 0, RegA
	case op == 0xD3:
		return RegA, 0

	// JP (HL)
	case op == 0xE9:
		return RegHL, 0

	// DI, EI
	case op == 0xF3, op == 0xFB:
		return 0, 0

	// ED prefix
	case op == 0xED:
		return edRegUsage(mem)

	// CB prefix
	case op == 0xCB:
		return cbRegUsage(mem)

	// DD/FD prefix (IX/IY)
	case op == 0xDD, op == 0xFD:
		return ddfdRegUsage(mem)
	}

	return 0, 0
}

// regFromBits maps the 3-bit register encoding to a RegSet.
// Returns 0 for (HL) encoding (bit pattern 110).
func regFromBits(v byte) RegSet {
	switch v & 7 {
	case 0:
		return RegB
	case 1:
		return RegC
	case 2:
		return RegD
	case 3:
		return RegE
	case 4:
		return RegH
	case 5:
		return RegL
	case 6:
		return 0 // (HL) — caller must handle
	case 7:
		return RegA
	}
	return 0
}

// edRegUsage handles ED-prefixed instructions.
func edRegUsage(mem []byte) (reads, writes RegSet) {
	if len(mem) < 2 {
		return 0, 0
	}
	op2 := mem[1]

	switch op2 {
	// NEG
	case 0x44:
		return RegA, RegA | RegF

	// RETN, RETI
	case 0x45, 0x4D:
		return 0, 0

	// IM 0/1/2
	case 0x46, 0x56, 0x5E:
		return 0, 0

	// LD I,A / LD R,A / LD A,I / LD A,R
	case 0x47:
		return RegA, 0 // LD I,A
	case 0x4F:
		return RegA, 0 // LD R,A
	case 0x57:
		return 0, RegA | RegF // LD A,I
	case 0x5F:
		return 0, RegA | RegF // LD A,R

	// RRD, RLD
	case 0x67, 0x6F:
		return RegA | RegHL, RegA | RegF

	// Block operations
	case 0xA0: // LDI
		return RegBC | RegDE | RegHL, RegBC | RegDE | RegHL | RegF
	case 0xA8: // LDD
		return RegBC | RegDE | RegHL, RegBC | RegDE | RegHL | RegF
	case 0xB0: // LDIR
		return RegBC | RegDE | RegHL, RegBC | RegDE | RegHL | RegF
	case 0xB8: // LDDR
		return RegBC | RegDE | RegHL, RegBC | RegDE | RegHL | RegF

	// Compare block
	case 0xA1: // CPI
		return RegA | RegBC | RegHL, RegBC | RegHL | RegF
	case 0xA9: // CPD
		return RegA | RegBC | RegHL, RegBC | RegHL | RegF
	case 0xB1: // CPIR
		return RegA | RegBC | RegHL, RegBC | RegHL | RegF
	case 0xB9: // CPDR
		return RegA | RegBC | RegHL, RegBC | RegHL | RegF

	// I/O block
	case 0xA2, 0xAA, 0xB2, 0xBA: // INI, IND, INIR, INDR
		return RegBC | RegHL, RegB | RegHL | RegF
	case 0xA3, 0xAB, 0xB3, 0xBB: // OUTI, OUTD, OTIR, OTDR
		return RegBC | RegHL, RegB | RegHL | RegF

	// IN r,(C)
	case 0x40:
		return RegBC, RegB | RegF
	case 0x48:
		return RegBC, RegC | RegF
	case 0x50:
		return RegBC, RegD | RegF
	case 0x58:
		return RegBC, RegE | RegF
	case 0x60:
		return RegBC, RegH | RegF
	case 0x68:
		return RegBC, RegL | RegF
	case 0x78:
		return RegBC, RegA | RegF

	// OUT (C),r
	case 0x41:
		return RegBC | RegB, 0
	case 0x49:
		return RegBC | RegC, 0
	case 0x51:
		return RegBC | RegD, 0
	case 0x59:
		return RegBC | RegE, 0
	case 0x61:
		return RegBC | RegH, 0
	case 0x69:
		return RegBC | RegL, 0
	case 0x79:
		return RegBC | RegA, 0

	// 16-bit loads: LD (nn),rr and LD rr,(nn)
	case 0x43:
		return RegBC, 0 // LD (nn),BC
	case 0x4B:
		return 0, RegBC // LD BC,(nn)
	case 0x53:
		return RegDE, 0 // LD (nn),DE
	case 0x5B:
		return 0, RegDE // LD DE,(nn)
	case 0x63:
		return RegHL, 0 // LD (nn),HL
	case 0x6B:
		return 0, RegHL // LD HL,(nn)
	case 0x73:
		return 0, 0 // LD (nn),SP
	case 0x7B:
		return 0, 0 // LD SP,(nn)

	// ADC HL,rr / SBC HL,rr
	case 0x4A:
		return RegHL | RegBC | RegF, RegHL | RegF // ADC HL,BC
	case 0x5A:
		return RegHL | RegDE | RegF, RegHL | RegF // ADC HL,DE
	case 0x6A:
		return RegHL | RegF, RegHL | RegF // ADC HL,HL
	case 0x7A:
		return RegHL | RegF, RegHL | RegF // ADC HL,SP
	case 0x42:
		return RegHL | RegBC | RegF, RegHL | RegF // SBC HL,BC
	case 0x52:
		return RegHL | RegDE | RegF, RegHL | RegF // SBC HL,DE
	case 0x62:
		return RegHL | RegF, RegHL | RegF // SBC HL,HL
	case 0x72:
		return RegHL | RegF, RegHL | RegF // SBC HL,SP
	}

	return 0, 0
}

// cbRegUsage handles CB-prefixed instructions (bit/shift/rotate).
func cbRegUsage(mem []byte) (reads, writes RegSet) {
	if len(mem) < 2 {
		return 0, 0
	}
	op2 := mem[1]
	r := regFromBits(op2)

	// BIT b,r — only reads, writes F
	if op2 >= 0x40 && op2 <= 0x7F {
		if r == 0 {
			return RegHL, RegF
		}
		return r, RegF
	}

	// RLC/RRC/RL/RR/SLA/SRA/SLL/SRL r (0x00-0x3F)
	// SET b,r / RES b,r (0x80-0xFF)
	if r == 0 {
		return RegHL, RegF // operates on (HL)
	}
	return r, r | RegF
}

// ddfdRegUsage handles DD/FD prefixed instructions (IX/IY).
// We don't track IX/IY in RegSet, but we do track effects on A-L, F.
func ddfdRegUsage(mem []byte) (reads, writes RegSet) {
	if len(mem) < 2 {
		return 0, 0
	}
	op2 := mem[1]

	switch {
	// ADD IX/IY, rr
	case op2 == 0x09:
		return RegBC, RegF
	case op2 == 0x19:
		return RegDE, RegF
	case op2 == 0x29:
		return 0, RegF // ADD IX,IX
	case op2 == 0x39:
		return 0, RegF

	// LD r,(IX/IY+d) — 0x46,0x4E,0x56,0x5E,0x66,0x6E,0x7E
	case op2 == 0x46:
		return 0, RegB
	case op2 == 0x4E:
		return 0, RegC
	case op2 == 0x56:
		return 0, RegD
	case op2 == 0x5E:
		return 0, RegE
	case op2 == 0x66:
		return 0, RegH
	case op2 == 0x6E:
		return 0, RegL
	case op2 == 0x7E:
		return 0, RegA

	// LD (IX/IY+d),r — 0x70-0x77
	case op2 == 0x70:
		return RegB, 0
	case op2 == 0x71:
		return RegC, 0
	case op2 == 0x72:
		return RegD, 0
	case op2 == 0x73:
		return RegE, 0
	case op2 == 0x74:
		return RegH, 0
	case op2 == 0x75:
		return RegL, 0
	case op2 == 0x77:
		return RegA, 0

	// LD (IX/IY+d),n
	case op2 == 0x36:
		return 0, 0

	// ALU A,(IX/IY+d)
	case op2 == 0x86:
		return RegA, RegA | RegF // ADD
	case op2 == 0x8E:
		return RegA, RegA | RegF // ADC
	case op2 == 0x96:
		return RegA, RegA | RegF // SUB
	case op2 == 0x9E:
		return RegA, RegA | RegF // SBC
	case op2 == 0xA6:
		return RegA, RegA | RegF // AND
	case op2 == 0xAE:
		return RegA, RegA | RegF // XOR
	case op2 == 0xB6:
		return RegA, RegA | RegF // OR
	case op2 == 0xBE:
		return RegA, RegF // CP

	// INC/DEC (IX/IY+d)
	case op2 == 0x34, op2 == 0x35:
		return 0, RegF

	// PUSH IX/IY, POP IX/IY
	case op2 == 0xE5:
		return 0, 0
	case op2 == 0xE1:
		return 0, 0

	// LD IX/IY,nn
	case op2 == 0x21:
		return 0, 0
	// LD IX/IY,(nn)
	case op2 == 0x2A:
		return 0, 0
	// LD (nn),IX/IY
	case op2 == 0x22:
		return 0, 0

	// INC/DEC IX/IY
	case op2 == 0x23, op2 == 0x2B:
		return 0, 0

	// JP (IX/IY)
	case op2 == 0xE9:
		return 0, 0

	// EX (SP),IX/IY
	case op2 == 0xE3:
		return 0, 0

	// DDCB/FDCB prefix — bit operations on (IX/IY+d)
	case op2 == 0xCB:
		if len(mem) < 4 {
			return 0, 0
		}
		op4 := mem[3]
		// BIT — only writes F
		if op4 >= 0x40 && op4 <= 0x7F {
			return 0, RegF
		}
		// SET/RES/shifts with LD r copy — check if low 3 bits != 6
		r := regFromBits(op4)
		if r != 0 {
			return 0, r | RegF // shift+copy result to register
		}
		return 0, RegF
	}

	return 0, 0
}
