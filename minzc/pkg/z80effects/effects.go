// Package z80effects describes explicit Z80 instruction effects independently
// of allocation classes. Unknown forms never supply a preservation guarantee.
// Syntax/encoding validation remains the assembler's responsibility. Refresh-R
// increments on instruction fetch are not explicit writes in this model.
package z80effects

import "strings"

// Registers names physical byte resources; pairs and banks are aliases.
type Registers uint64

const (
	A Registers = 1 << iota
	F
	B
	C
	D
	E
	H
	L
	IXH
	IXL
	IYH
	IYL
	I
	R
	SP
	AAlt
	FAlt
	BAlt
	CAlt
	DAlt
	EAlt
	HAlt
	LAlt
)
const (
	AF    = A | F
	BC    = B | C
	DE    = D | E
	HL    = H | L
	IX    = IXH | IXL
	IY    = IYH | IYL
	AFAlt = AAlt | FAlt
	BCAlt = BAlt | CAlt
	DEAlt = DAlt | EAlt
	HLAlt = HAlt | LAlt
)

var names = map[string]Registers{
	"A": A, "F": F, "B": B, "C": C, "D": D, "E": E, "H": H, "L": L,
	"AF": AF, "BC": BC, "DE": DE, "HL": HL, "IX": IX, "IY": IY,
	"IXH": IXH, "IXL": IXL, "IYH": IYH, "IYL": IYL, "I": I, "R": R, "SP": SP,
	"AF'": AFAlt, "BC'": BCAlt, "DE'": DEAlt, "HL'": HLAlt,
}

func Register(name string) Registers { return names[strings.ToUpper(strings.TrimSpace(name))] }

type Flow uint8

const (
	Fallthrough Flow = iota
	Jump
	ConditionalJump
	Return
	Wait
)

type INTChange uint8

const (
	INTUnchanged INTChange = iota
	INTDisabled
	INTEnabled
)

// Effect describes one instruction. StackDelta is a local change, not proof of
// balanced paths or restoration. SP replacement and INT ordering need CFG proof.
type Effect struct {
	Known                   bool
	Reason                  string
	Reads, Writes           Registers
	FlagsRead, FlagsWrite   byte
	MemoryRead, MemoryWrite bool
	PortRead, PortWrite     bool
	StackDelta              int
	ReplacesSP              bool
	Conditional             bool
	INT                     INTChange
	ReadsINT                bool
	Flow                    Flow
}

func (e *Effect) flags(read, write byte) {
	e.FlagsRead |= read
	e.FlagsWrite |= write
	if read != 0 {
		e.Reads |= F
	}
	if write != 0 {
		e.Writes |= F
	}
}
func unknown(reason string) Effect { return Effect{Reason: reason} }
func compact(s string) string {
	return strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(s)), " ", "")
}
func indirect(s string) bool { return strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") }
func addressReads(s string) Registers {
	if !indirect(s) {
		return 0
	}
	v := s[1 : len(s)-1]
	switch v {
	case "HL":
		return HL
	case "BC":
		return BC
	case "DE":
		return DE
	case "SP":
		return SP
	}
	for _, r := range []string{"IX", "IY"} {
		if v == r || strings.HasPrefix(v, r+"+") || strings.HasPrefix(v, r+"-") {
			return Register(r)
		}
	}
	return 0
}
func readOperand(e *Effect, s string) {
	if indirect(s) {
		e.Reads |= addressReads(s)
		e.MemoryRead = true
	} else {
		e.Reads |= Register(s)
	}
}
func writeOperand(e *Effect, s string) bool {
	if indirect(s) {
		e.Reads |= addressReads(s)
		e.MemoryWrite = true
		return true
	}
	r := Register(s)
	if r == 0 {
		return false
	}
	e.Writes |= r
	return true
}
func byteOperand(s string) bool {
	r := Register(s)
	return r == A || r == B || r == C || r == D || r == E || r == H || r == L || r == IXH || r == IXL || r == IYH || r == IYL
}

// Exclude pseudo pair copies and unsupported AF forms: their expansion may
// have extra scratch effects not visible in the source instruction.
func validLD(dst, src string) bool {
	if dst == "I" || dst == "R" {
		return src == "A"
	}
	if dst == "A" && (src == "I" || src == "R") {
		return true
	}
	dr, sr := Register(dst), Register(src)
	if byteOperand(dst) {
		if sr != 0 {
			if !byteOperand(src) {
				return false
			}
			db, sb := indexBank(dr), indexBank(sr)
			if db != 0 || sb != 0 {
				if db != 0 && sb != 0 && db != sb {
					return false
				}
				if dr&HL != 0 || sr&HL != 0 {
					return false
				}
			}
			return true
		}
		if indirect(src) {
			return mainByteOperand(dst) && (byteMemory(src) || dst == "A" && addressReads(src) != SP)
		}
		return true
	}
	if pairOperand(dst) {
		if sr != 0 {
			return dst == "SP" && (src == "HL" || src == "IX" || src == "IY")
		}
		return !indirect(src) || addressReads(src) == 0
	}
	if !indirect(dst) || dr != 0 {
		return false
	}
	address := addressReads(dst)
	if sr != 0 {
		if byteOperand(src) {
			return mainByteOperand(src) && (src == "A" && address != SP || address == HL || address == IX || address == IY)
		}
		return pairOperand(src) && address == 0
	}
	return !indirect(src) && (address == HL || address == IX || address == IY)
}

func mainByteOperand(s string) bool {
	r := Register(s)
	return r == A || r == B || r == C || r == D || r == E || r == H || r == L
}
func indexBank(r Registers) Registers {
	if r&IX != 0 {
		return IX
	}
	if r&IY != 0 {
		return IY
	}
	return 0
}
func byteMemory(s string) bool {
	r := addressReads(s)
	return indirect(s) && (r == HL || r == IX || r == IY)
}

func pairOperand(s string) bool {
	switch s {
	case "BC", "DE", "HL", "SP", "IX", "IY":
		return true
	}
	return false
}
func condition(s string) byte {
	switch s {
	case "NZ", "Z":
		return 0x40
	case "NC", "C":
		return 1
	case "PO", "PE":
		return 4
	case "P", "M":
		return 0x80
	}
	return 0
}

// Instruction accepts a single instruction, optionally preceded by a label.
// CALL/RST and executable DB/DW remain unknown until a callee/patch-site
// contract supplies their effects. Operand effects do not validate expressions.
func Instruction(line string) Effect {
	line = strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
	if colon := strings.IndexByte(line, ':'); colon >= 0 {
		line = strings.TrimSpace(line[colon+1:])
	}
	if line == "" {
		return Effect{Known: true}
	}
	fields := strings.Fields(line)
	op := strings.ToUpper(fields[0])
	rest := strings.TrimSpace(line[len(fields[0]):])
	var args []string
	if rest != "" {
		for _, s := range strings.Split(rest, ",") {
			args = append(args, compact(s))
		}
	}
	e := Effect{Known: true}
	arity := func(n int) bool { return len(args) == n }
	switch op {
	case "NOP":
		if !arity(0) {
			return unknown("NOP operands")
		}
	case "DI", "EI":
		if !arity(0) {
			return unknown("INT operands")
		}
		if op == "DI" {
			e.INT = INTDisabled
		} else {
			e.INT = INTEnabled
		}
	case "HALT":
		if !arity(0) {
			return unknown("HALT operands")
		}
		e.Flow = Wait
		e.ReadsINT = true
	case "LD":
		if !arity(2) {
			return unknown("LD operands")
		}
		dst, src := args[0], args[1]
		if !validLD(dst, src) {
			return unknown("unsupported native LD form")
		}
		if !writeOperand(&e, dst) {
			return unknown("LD destination")
		}
		readOperand(&e, src)
		if dst == "SP" {
			e.ReplacesSP = true
		}
		if dst == "F" || src == "F" || strings.Contains(dst, "'") || strings.Contains(src, "'") {
			return unknown("unsupported direct flags/bank LD")
		}
		if indirect(dst) && indirect(src) {
			return unknown("memory-to-memory LD")
		}
		if dst == "A" && (src == "I" || src == "R") {
			e.flags(1, 0xfe)
			e.ReadsINT = true
		}
	case "PUSH", "POP":
		if !arity(1) {
			return unknown("stack operands")
		}
		r := Register(args[0])
		if r != AF && r != BC && r != DE && r != HL && r != IX && r != IY {
			return unknown("stack register")
		}
		e.Reads |= SP
		e.Writes |= SP
		if op == "PUSH" {
			e.Reads |= r
			e.MemoryWrite = true
			e.StackDelta = -2
		} else {
			e.Writes |= r
			e.MemoryRead = true
			e.StackDelta = 2
			if r == AF {
				e.flags(0, 255)
			}
		}
	case "INC", "DEC":
		if !arity(1) {
			return unknown("INC/DEC operands")
		}
		dst := args[0]
		if !byteOperand(dst) && !pairOperand(dst) && !byteMemory(dst) {
			return unknown("INC/DEC destination")
		}
		readOperand(&e, dst)
		writeOperand(&e, dst)
		if !pairOperand(dst) {
			e.flags(1, 0xfe)
		}
		if dst == "SP" {
			if op == "INC" {
				e.StackDelta = 1
			} else {
				e.StackDelta = -1
			}
		}
	case "DJNZ":
		if !arity(1) {
			return unknown("DJNZ target")
		}
		e.Reads |= B
		e.Writes |= B
		e.Flow = ConditionalJump
		e.Conditional = true
	case "JP", "JR":
		if len(args) < 1 || len(args) > 2 {
			return unknown("jump operands")
		}
		e.Flow = Jump
		if arity(2) {
			f := condition(args[0])
			if f == 0 {
				return unknown("jump condition")
			}
			e.flags(f, 0)
			e.Flow = ConditionalJump
			e.Conditional = true
		}
		target := args[len(args)-1]
		if indirect(target) {
			if op != "JP" || arity(2) {
				return unknown("indirect conditional jump")
			}
			e.Reads |= addressReads(target)
			if target != "(HL)" && target != "(IX)" && target != "(IY)" {
				return unknown("indirect jump target")
			}
		}
	case "RET":
		if len(args) > 1 {
			return unknown("RET operands")
		}
		if arity(1) {
			f := condition(args[0])
			if f == 0 {
				return unknown("RET condition")
			}
			e.flags(f, 0)
			e.Conditional = true
		}
		e.Flow = Return
		e.Reads |= SP
		e.Writes |= SP
		e.MemoryRead = true
		e.StackDelta = 2
	case "EX":
		if !arity(2) {
			return unknown("EX operands")
		}
		switch {
		case args[0] == "DE" && args[1] == "HL":
			e.Reads |= DE | HL
			e.Writes |= DE | HL
		case args[0] == "AF" && args[1] == "AF'":
			e.Reads |= AF | AFAlt
			e.Writes |= AF | AFAlt
			e.flags(255, 255)
		case args[0] == "(SP)" && (args[1] == "HL" || args[1] == "IX" || args[1] == "IY"):
			e.Reads |= SP | Register(args[1])
			e.Writes |= Register(args[1])
			e.MemoryRead = true
			e.MemoryWrite = true
		default:
			return unknown("EX form")
		}
	case "EXX":
		if !arity(0) {
			return unknown("EXX operands")
		}
		e.Reads |= BC | DE | HL | BCAlt | DEAlt | HLAlt
		e.Writes |= e.Reads
	case "ADD", "ADC", "SUB", "SBC", "AND", "OR", "XOR", "CP":
		if len(args) < 1 || len(args) > 2 {
			return unknown("ALU operands")
		}
		dst := "A"
		src := args[0]
		if arity(2) {
			dst, src = args[0], args[1]
		}
		if dst != "A" {
			if !arity(2) || !pairOperand(src) {
				return unknown("16-bit ALU source")
			}
			if op == "ADD" && (dst == "HL" || dst == "IX" || dst == "IY") {
				if src != dst && src != "BC" && src != "DE" && src != "SP" {
					return unknown("16-bit ADD source")
				}
				e.flags(0xc4, 0x3b)
			} else if (op == "ADC" || op == "SBC") && dst == "HL" && (src == "HL" || src == "BC" || src == "DE" || src == "SP") {
				e.flags(1, 255)
			} else {
				return unknown("16-bit ALU form")
			}
			e.Reads |= Register(dst) | Register(src)
			e.Writes |= Register(dst)
		} else {
			if pairOperand(src) || src == "F" || src == "I" || src == "R" || indirect(src) && !byteMemory(src) {
				return unknown("8-bit ALU source")
			}
			readOperand(&e, src)
			e.Reads |= A
			if op != "CP" {
				e.Writes |= A
			}
			e.flags(0, 255)
			if op == "ADC" || op == "SBC" {
				e.flags(1, 0)
			}
		}
	case "NEG", "CPL", "DAA", "SCF", "CCF", "RLCA", "RRCA", "RLA", "RRA":
		if !arity(0) {
			return unknown("implicit ALU operands")
		}
		switch op {
		case "NEG":
			e.Reads |= A
			e.Writes |= A
			e.flags(0, 255)
		case "CPL":
			e.Reads |= A
			e.Writes |= A
			e.flags(0xc5, 0x3a)
		case "DAA":
			e.Reads |= A
			e.Writes |= A
			e.flags(0x13, 0xfd)
		case "SCF":
			e.Reads |= A
			e.flags(0xc4, 0x3b)
		case "CCF":
			e.Reads |= A
			e.flags(0xc5, 0x3b)
		default:
			e.Reads |= A
			e.Writes |= A
			e.flags(0xc4, 0x3b)
			if op == "RLA" || op == "RRA" {
				e.flags(1, 0)
			}
		}
	case "RL", "RR", "RLC", "RRC", "SLA", "SRA", "SRL", "SLL":
		if !arity(1) || (!mainByteOperand(args[0]) && !byteMemory(args[0])) {
			return unknown("rotate/shift form")
		}
		readOperand(&e, args[0])
		writeOperand(&e, args[0])
		e.flags(0, 255)
		if op == "RL" || op == "RR" {
			e.flags(1, 0)
		}
	case "BIT", "SET", "RES":
		if !arity(2) || (!mainByteOperand(args[1]) && !byteMemory(args[1])) {
			return unknown("bit form")
		}
		readOperand(&e, args[1])
		if op == "BIT" {
			e.flags(1, 0xfe)
		} else {
			writeOperand(&e, args[1])
		}
	case "LDI", "LDD", "LDIR", "LDDR":
		if !arity(0) {
			return unknown("block LD operands")
		}
		e.Reads |= A | HL | DE | BC
		e.Writes |= HL | DE | BC
		e.MemoryRead = true
		e.MemoryWrite = true
		e.flags(0xc1, 0x3e)
	case "CPI", "CPD", "CPIR", "CPDR":
		if !arity(0) {
			return unknown("block CP operands")
		}
		e.Reads |= A | HL | BC
		e.Writes |= HL | BC
		e.MemoryRead = true
		e.flags(1, 0xfe)
	case "OUT":
		if !arity(2) {
			return unknown("OUT operands")
		}
		if !indirect(args[0]) || args[0] == "(C)" && !mainByteOperand(args[1]) && args[1] != "0" || args[0] != "(C)" && args[1] != "A" {
			return unknown("OUT form")
		}
		e.PortWrite = true
		e.Reads |= Register(args[1])
		if args[0] == "(C)" {
			e.Reads |= BC
		} else {
			e.Reads |= A
		}
	case "IN":
		if !arity(2) || !mainByteOperand(args[0]) || !indirect(args[1]) {
			return unknown("IN form")
		}
		e.Writes |= Register(args[0])
		e.PortRead = true
		if args[1] == "(C)" {
			e.Reads |= BC
			e.flags(1, 0xfe)
		} else {
			if args[0] != "A" {
				return unknown("immediate IN form")
			}
			e.Reads |= A
		}
	default:
		return unknown("unknown instruction or opaque call/data: " + op)
	}
	return e
}

// Block returns instruction effects in lexical order. It intentionally makes
// no claim about reachable paths, balanced stack, interrupt safety or SMC.
func Block(code string) []Effect {
	var effects []Effect
	for _, line := range strings.Split(strings.ReplaceAll(code, "/", "\n"), "\n") {
		effects = append(effects, Instruction(line))
	}
	return effects
}
