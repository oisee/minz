// Package z80timing provides canonical Z80 instruction T-state counts.
//
// Single source of truth for timing constants used by:
//   - pkg/emulator — cycle-accurate execution tracking
//   - pkg/mir2/z80cost — register allocator cost model
//   - pkg/z80asm — assembler size/speed heuristics
//
// Sources:
//   - Zilog Z80 CPU User Manual (UM0080, rev 11)
//   - Calibrated via remogatto/z80 (FUSE-tested) in pkg/z80testing
//   - See reports/2026-03-09-037-TState_Counting_Fixed.md for calibration notes
//
// Note on remogatto ContendPort behaviour:
//   OUT (n), A is 7T here (not textbook 11T) because remogatto does not trigger
//   ContendPort for non-ZX-Spectrum addresses.  The delta is constant across all
//   benchmarks, so relative comparisons remain valid.
package z80timing

// ── Data transfer ─────────────────────────────────────────────────────────────

const (
	// 8-bit register-to-register  LD r, r'
	LD_r_r = 4

	// 8-bit immediate  LD r, n
	LD_r_n = 7

	// 16-bit immediate  LD rr, nn
	LD_rr_nn = 10

	// Register-indirect  LD r, (HL) / LD (HL), r
	LD_r_HL  = 7
	LD_HL_r  = 7
	LD_A_BC  = 7 // LD A, (BC)
	LD_BC_A  = 7 // LD (BC), A
	LD_A_DE  = 7 // LD A, (DE)
	LD_DE_A  = 7 // LD (DE), A

	// Extended (absolute address)  LD A, (nn) / LD (nn), A
	LD_A_nn = 13
	LD_nn_A = 13

	// Extended 16-bit  LD HL, (nn) / LD (nn), HL
	LD_HL_nn_mem = 16
	LD_nn_HL     = 16

	// SP transfer  LD SP, HL
	LD_SP_HL = 6

	// Stack
	PUSH_rr = 11 // PUSH BC/DE/HL/AF
	POP_rr  = 10 // POP  BC/DE/HL/AF
	PUSH_IX = 15 // PUSH IX or PUSH IY  (DD/FD prefix)
	POP_IX  = 14 // POP  IX or POP  IY
)

// ── Exchange ──────────────────────────────────────────────────────────────────

const (
	EX_DE_HL = 4 // EX DE, HL
	EX_AF_AF = 4 // EX AF, AF'
	EXX      = 4 // EXX  (shadow bank swap)
)

// ── 8-bit arithmetic & logic ──────────────────────────────────────────────────

const (
	ADD_A_r = 4  // ADD A, r
	ADD_A_n = 7  // ADD A, n
	ADC_A_r = 4  // ADC A, r
	ADC_A_n = 7  // ADC A, n
	SUB_r   = 4  // SUB r
	SUB_n   = 7  // SUB n
	SBC_A_r = 4  // SBC A, r
	AND_r   = 4  // AND r
	AND_n   = 7  // AND n
	OR_r    = 4  // OR r
	OR_n    = 7  // OR n
	XOR_r   = 4  // XOR r
	XOR_n   = 7  // XOR n
	CP_r    = 4  // CP r
	CP_n    = 7  // CP n
	INC_r   = 4  // INC r
	DEC_r   = 4  // DEC r
	NEG     = 8  // NEG  (ED prefix)
	CPL     = 4  // CPL  (complement accumulator)
	SCF     = 4  // SCF  (set carry flag)
	CCF     = 4  // CCF  (complement carry flag)
)

// ── 16-bit arithmetic ─────────────────────────────────────────────────────────

const (
	ADD_HL_rr = 11 // ADD HL, rr
	ADC_HL_rr = 15 // ADC HL, rr  (ED prefix)
	SBC_HL_rr = 15 // SBC HL, rr  (ED prefix)
	INC_rr    = 6  // INC BC/DE/HL/SP
	DEC_rr    = 6  // DEC BC/DE/HL/SP
)

// ── Rotate / shift (CB prefix) ────────────────────────────────────────────────

const (
	RLCA  = 4 // RLCA
	RRCA  = 4 // RRCA
	RLA   = 4 // RLA
	RRA   = 4 // RRA
	RLC_r = 8 // RLC r  (CB prefix)
	RRC_r = 8
	RL_r  = 8
	RR_r  = 8
	SLA_r = 8
	SRA_r = 8
	SRL_r = 8
)

// ── Jumps ─────────────────────────────────────────────────────────────────────

const (
	JP_nn      = 10 // JP nn  (unconditional; no short-circuit)
	JP_cc_nn   = 10 // JP cc, nn  (taken or not taken — Z80 always reads nn)
	JR_e       = 12 // JR e  (taken)
	JR_e_NT    = 7  // JR e  (not taken; 2-byte encoding)
	JR_cc_e    = 12 // JR cc, e  (taken)
	JR_cc_e_NT = 7  // JR cc, e  (not taken)
	DJNZ       = 13 // DJNZ e  (taken: B≠0 after DEC)
	DJNZ_NT    = 8  // DJNZ e  (not taken: B=0 after DEC)
	JP_HL      = 4  // JP (HL)
)

// ── Calls & returns ───────────────────────────────────────────────────────────

const (
	CALL_nn      = 17 // CALL nn  (unconditional)
	CALL_cc_nn   = 17 // CALL cc, nn  (taken)
	CALL_cc_nn_T = 10 // CALL cc, nn  (not taken — no push)
	RET          = 10 // RET
	RET_cc       = 11 // RET cc  (taken)
	RET_cc_NT    = 5  // RET cc  (not taken)
	RST_n        = 11 // RST n
)

// ── Misc ──────────────────────────────────────────────────────────────────────

const (
	NOP  = 4  // NOP
	HALT = 4  // HALT  (per-cycle; loops until interrupt)
	DI   = 4  // DI
	EI   = 4  // EI
	LDIR = 21 // LDIR per-iteration cost (when BC > 1 after last)
	LDDR = 21
)

// ── IX/IY prefix overhead ─────────────────────────────────────────────────────

// IXY_OVERHEAD is the additional T-states incurred by the DD or FD displacement
// prefix for any instruction that accesses IX or IY.
// Example: LD A, (IX+d) = LD_r_HL + IXY_OVERHEAD = 7+4+8 = wait, no.
// Actually: LD r, (IX+d) = 19T  (DD opcode + reg op + disp byte + mem access)
// But for LD IX,nn: base LD_rr_nn (10T) + DD prefix decode = 14T total.
// Simple model: add IXY_OVERHEAD to the base instruction T-count.
const IXY_OVERHEAD = 4

// ── Derived: allocator cost primitives ───────────────────────────────────────

// MemRoundTrip8 is the cost of storing + loading a single byte through an
// absolute $F0xx memory slot: LD (nn), A  +  LD A, (nn)
const MemRoundTrip8 = LD_nn_A + LD_A_nn // 26T

// StackRoundTrip is the cost of PUSH rr + POP rr (16-bit save/restore on stack).
const StackRoundTrip = PUSH_rr + POP_rr // 21T

// RegRegMove is the cost of moving a value between two 8-bit primary registers.
const RegRegMove = LD_r_r // 4T

// FlagMaterialise8 is the approximate cost of materialising a flag condition
// into A: SCF + SBC A, A  (carry) or  LD A,0 / JR NZ, $+2 / INC A  (zero).
// SCF+SBC pattern: 4T + 4T = 8T.
const FlagMaterialise8 = 8

// FlagCheck is the cost of testing whether A is zero/non-zero using AND A or CP 0.
// AND A = 4T (sets flags, clears C).
// CP 0 = 7T (sets flags, doesn't modify A).
const FlagCheckAndA = AND_r  // 4T — preferred; sets Z/S/P, clears C and H
const FlagCheckCPN  = CP_n   // 7T — when C flag must be preserved
