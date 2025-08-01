package z80asm

// Register represents a Z80 register
type Register uint8

const (
	// 8-bit registers
	RegA Register = iota
	RegB
	RegC
	RegD
	RegE
	RegH
	RegL

	// Special 8-bit registers
	RegI
	RegR

	// 16-bit register pairs
	RegBC
	RegDE
	RegHL
	RegAF
	RegSP
	RegIX
	RegIY

	// Undocumented IX/IY halves
	RegIXH  // High byte of IX
	RegIXL  // Low byte of IX
	RegIYH  // High byte of IY
	RegIYL  // Low byte of IY

	// Special
	RegNone
)

// Condition represents a conditional jump/return condition
type Condition uint8

const (
	CondNone Condition = iota
	CondNZ              // Not Zero
	CondZ               // Zero
	CondNC              // Not Carry
	CondC               // Carry
	CondPO              // Parity Odd
	CondPE              // Parity Even
	CondP               // Plus (positive)
	CondM               // Minus (negative)
)

// Instruction represents an assembled instruction
type Instruction struct {
	Label   string   // Optional label
	Opcode  string   // Mnemonic (LD, ADD, etc.)
	Operand1 string  // First operand
	Operand2 string  // Second operand (if any)
	Address uint16   // Address where this instruction will be placed
	Bytes   []byte   // Encoded bytes
}

// Symbol represents a label or constant
type Symbol struct {
	Name    string
	Value   uint16
	Defined bool
}