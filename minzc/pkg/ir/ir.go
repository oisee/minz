package ir

import (
	"fmt"
	"strings"
)

// Z80Register represents a physical Z80 register
type Z80Register uint32

// Z80 register definitions
const (
	// 8-bit registers
	Z80_A Z80Register = 1 << iota
	Z80_F
	Z80_B
	Z80_C
	Z80_D
	Z80_E
	Z80_H
	Z80_L
	
	// 16-bit register pairs
	Z80_AF
	Z80_BC
	Z80_DE
	Z80_HL
	Z80_IX
	Z80_IY
	Z80_SP
	
	// Shadow registers
	Z80_A_SHADOW
	Z80_F_SHADOW
	Z80_B_SHADOW
	Z80_C_SHADOW
	Z80_D_SHADOW
	Z80_E_SHADOW
	Z80_H_SHADOW
	Z80_L_SHADOW
	Z80_AF_SHADOW
	Z80_BC_SHADOW
	Z80_DE_SHADOW
	Z80_HL_SHADOW
)

// RegisterSet tracks which Z80 registers are used
type RegisterSet uint32

// Add adds a register to the set
func (rs *RegisterSet) Add(reg Z80Register) {
	*rs |= RegisterSet(reg)
}

// Contains checks if a register is in the set
func (rs RegisterSet) Contains(reg Z80Register) bool {
	return rs&RegisterSet(reg) != 0
}

// Clear removes all registers from the set
func (rs *RegisterSet) Clear() {
	*rs = 0
}

// Count returns the number of registers in the set
func (rs RegisterSet) Count() int {
	count := 0
	for i := uint32(rs); i != 0; i &= i - 1 {
		count++
	}
	return count
}

// Opcode represents an IR operation
type Opcode uint8

const (
	// Control flow
	OpNop Opcode = iota
	OpLabel
	OpJump
	OpJumpIf
	OpJumpIfNot
	OpJumpIfZero
	OpJumpIfNotZero
	OpCall
	OpCallIndirect  // Indirect function call through register
	OpReturn
	
	// Data movement
	OpLoadConst
	OpLoadVar
	OpStoreVar
	OpLoadParam
	OpLoadField
	OpStoreField
	OpLoadIndex
	OpStoreIndex
	OpLoadElement    // Load array element
	OpStoreElement   // Store array element
	OpLoadBitField  // Load bit field value
	OpStoreBitField // Store bit field value
	OpMove
	OpLoadLabel  // Load address of a label
	OpLoadDirect // Load from direct memory address
	OpStoreDirect // Store to direct memory address
	
	// Self-modifying code operations
	OpSMCLoadConst
	OpSMCStoreConst
	OpSMCParam      // SMC parameter slot
	OpSMCSave       // Save SMC parameter to stack
	OpSMCRestore    // Restore SMC parameter from stack
	OpSMCUpdate     // Update SMC parameter value
	OpStoreTSMCRef  // Store to TSMC reference immediate
	
	// TRUE SMC operations (истинный SMC)
	OpTrueSMCLoad   // Load from anchor address
	OpTrueSMCPatch  // Patch anchor before call
	
	// TSMC Reference operations
	OpTSMCRefAnchor // Create anchor for TSMC reference parameter
	OpTSMCRefLoad   // Load from TSMC reference (immediate)
	OpTSMCRefPatch  // Patch TSMC reference at call site
	
	// Error handling (Carry-flag ABI)
	OpSetError      // Set carry flag and error code in A
	OpCheckError    // Check carry flag for error
	
	// Array operations
	OpArrayInit     // Initialize array
	OpArrayElement  // Set array element during initialization
	
	// Arithmetic
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpNeg
	OpInc
	OpDec
	
	// Bitwise
	OpAnd
	OpOr
	OpXor
	OpNot
	OpShl
	OpShr
	
	// Comparison
	OpCmp  // Generic comparison
	OpTest // Test register (sets flags without compare)
	OpEq
	OpNe
	OpLt
	OpGt
	OpLe
	OpGe
	
	// Memory
	OpAlloc
	OpFree
	OpLoadPtr
	OpStorePtr
	OpAddr     // Address-of operator (&)
	OpLoad     // Load from memory address (indirect)
	OpStore    // Store to memory address (indirect)
	
	// Stack
	OpPush
	OpPop
	
	// Inline assembly
	OpAsm
	
	// Loop operations
	OpLoadAddr       // Load address of variable/array
	OpCopyToBuffer   // Copy memory to static buffer
	OpCopyFromBuffer // Copy static buffer to memory
	OpDJNZ          // Decrement and jump if not zero
	OpLoadImm       // Load immediate value
	OpAddImm        // Add immediate to register
	
	// Built-in functions
	OpPrint         // Print a u8 character
	OpPrintU8       // Print u8 as decimal
	OpPrintU16      // Print u16 as decimal
	OpPrintI8       // Print i8 as decimal 
	OpPrintI16      // Print i16 as decimal
	OpPrintBool     // Print bool as "true"/"false"
	OpPrintString   // Print null-terminated string
	OpPrintStringDirect // Print short string directly (no loop)
	OpLoadString    // Load string literal address
	OpLen           // Get length of array/string
	OpMemcpy        // Copy memory block
	OpMemset        // Set memory block
)

// Instruction represents a single IR instruction
type Instruction struct {
	Op           Opcode
	Dest         Register
	Src1         Register
	Src2         Register
	Imm          int64
	Imm2         int64  // Second immediate for some operations
	Label        string
	Symbol       string
	Type         Type
	Comment      string
	PhysicalRegs map[string]string // Maps virtual to physical registers
	SMCLabel     string            // Label for self-modifying code location
	SMCTarget    string            // Target label for SMC store operations
	AsmCode      string            // Raw assembly code for OpAsm instructions
	AsmName      string            // Optional name for named asm blocks
	Args         []Register        // Argument registers for OpCall
}

// AsmBlock represents an inline assembly block
type AsmBlock struct {
	Name string
	Code string
}

// Register represents a virtual register
type Register int

const (
	// Special registers
	RegZero Register = -1 // Always zero
	RegSP   Register = -2 // Stack pointer
	RegFP   Register = -3 // Frame pointer
	RegRet  Register = -4 // Return value
)

// Type represents a type in the IR
type Type interface {
	Size() int // Size in bytes
	String() string
}

// BasicType represents primitive types
type BasicType struct {
	Kind TypeKind
}

type TypeKind int

const (
	TypeVoid TypeKind = iota
	TypeBool
	TypeU8
	TypeU16
	TypeI8
	TypeI16
)

func (t *BasicType) Size() int {
	switch t.Kind {
	case TypeVoid:
		return 0
	case TypeBool, TypeU8, TypeI8:
		return 1
	case TypeU16, TypeI16:
		return 2
	default:
		return 0
	}
}

func (t *BasicType) String() string {
	switch t.Kind {
	case TypeVoid:
		return "void"
	case TypeBool:
		return "bool"
	case TypeU8:
		return "u8"
	case TypeU16:
		return "u16"
	case TypeI8:
		return "i8"
	case TypeI16:
		return "i16"
	default:
		return "unknown"
	}
}

// PointerType represents pointer types
type PointerType struct {
	Base      Type
	IsMutable bool
}

func (t *PointerType) Size() int {
	return 2 // 16-bit pointers on Z80
}

func (t *PointerType) String() string {
	if t.IsMutable {
		return "*mut " + t.Base.String()
	}
	return "*" + t.Base.String()
}

// ArrayType represents array types
type ArrayType struct {
	Element Type
	Length  int
}

func (t *ArrayType) Size() int {
	return t.Element.Size() * t.Length
}

func (t *ArrayType) String() string {
	return fmt.Sprintf("[%d]%s", t.Length, t.Element.String())
}

// LambdaType represents function pointer types (lambda closures)
type LambdaType struct {
	ParamTypes []Type
	ReturnType Type
}

func (t *LambdaType) Size() int {
	return 2 // Function pointer is 16-bit on Z80
}

func (t *LambdaType) String() string {
	params := make([]string, len(t.ParamTypes))
	for i, p := range t.ParamTypes {
		params[i] = p.String()
	}
	return fmt.Sprintf("|%s| -> %s", strings.Join(params, ", "), t.ReturnType.String())
}

// StructType represents struct types
type StructType struct {
	Name       string
	Fields     map[string]Type
	FieldOrder []string // Preserves field order for layout
}

func (t *StructType) Size() int {
	size := 0
	for _, fieldName := range t.FieldOrder {
		size += t.Fields[fieldName].Size()
	}
	return size
}

func (t *StructType) String() string {
	return t.Name
}

// EnumType represents enum types
type EnumType struct {
	Name     string
	Variants map[string]int
}

func (t *EnumType) Size() int {
	// Enums are represented as u8 or u16 depending on variant count
	if len(t.Variants) <= 256 {
		return 1
	}
	return 2
}

func (t *EnumType) String() string {
	return t.Name
}

// BitStructType represents bit-struct types
type BitStructType struct {
	UnderlyingType Type               // u8 or u16
	Fields         map[string]*BitField
	FieldOrder     []string           // Preserve field declaration order
}

// FunctionType represents function types
type FunctionType struct {
	Params []Type
	Return Type
}

func (t *FunctionType) Size() int {
	return 2 // Function pointers are 16-bit addresses on Z80
}

func (t *FunctionType) String() string {
	params := []string{}
	for _, p := range t.Params {
		params = append(params, p.String())
	}
	return fmt.Sprintf("fun(%s) -> %s", strings.Join(params, ", "), t.Return.String())
}

func (t *BitStructType) Size() int {
	return t.UnderlyingType.Size()
}

func (t *BitStructType) String() string {
	return fmt.Sprintf("bits<%s>", t.UnderlyingType.String())
}

// BitField represents a field in a bit struct
type BitField struct {
	Name      string
	BitOffset int    // Starting bit position
	BitWidth  int    // Number of bits
}

// Function represents a function in IR
type Function struct {
	Name         string
	Params       []Parameter
	ReturnType   Type
	Locals       []Local
	Instructions []Instruction
	NextReg      Register
	NumParams    int
	IsInterrupt  bool
	NextRegister Register // Same as NextReg but more clearly named
	IsSMCEnabled bool     // Whether self-modifying code is enabled for this function
	IsRecursive  bool     // Whether this function is recursive
	SMCLocations map[string]int // Maps SMC labels to instruction indices
	IsSMCDefault     bool                    // Use SMC by default (true)
	SMCParamOffsets  map[string]int          // Parameter name -> SMC offset
	RequiresContext  bool                    // True for recursive functions
	HasTailRecursion bool                    // True if function has tail recursive calls
	
	// Register usage tracking for optimal prologue/epilogue
	UsedRegisters    RegisterSet // Which Z80 registers are actually used
	ModifiedRegisters RegisterSet // Which registers are modified (need saving)
	
	// TRUE SMC support
	UsesTrueSMC  bool                    // Uses TRUE SMC anchors (not fixed slots)
	SMCAnchors   map[string]*SMCAnchorInfo // Parameter -> anchor info
	
	// Metadata for optimization passes
	Metadata map[string]string // Generic metadata storage
	CalleeSavedRegs  RegisterSet // Registers this function must preserve
	MaxStackDepth    int         // Maximum stack depth for this function
	CallingConvention string     // ABI calling convention ("smc", "register", "stack", etc.)
}

// Parameter represents a function parameter
type Parameter struct {
	Name string
	Type Type
	Reg  Register
	IsTSMCRef bool // True if this should use TSMC reference passing
}

// Local represents a local variable
type Local struct {
	Name   string
	Type   Type
	Reg    Register
	Offset int // Stack offset if spilled
}

// NewFunction creates a new IR function
func NewFunction(name string, returnType Type) *Function {
	return &Function{
		Name:            name,
		ReturnType:      returnType,
		NextReg:         1, // Start from 1, 0 is reserved
		NextRegister:    1, // Same as NextReg
		IsSMCDefault:    true, // SMC is the default!
		IsSMCEnabled:    true, // Enable SMC by default
		SMCParamOffsets: make(map[string]int),
	}
}

// AllocReg allocates a new virtual register
func (f *Function) AllocReg() Register {
	reg := f.NextReg
	f.NextReg++
	return reg
}

// LastAllocatedReg returns the last allocated register
func (f *Function) LastAllocatedReg() Register {
	return f.NextReg - 1
}

// AddParam adds a parameter to the function
func (f *Function) AddParam(name string, typ Type) Register {
	reg := f.AllocReg()
	
	// Check if this should be a TSMC reference parameter
	isTSMCRef := false
	if _, isPtr := typ.(*PointerType); isPtr && f.IsSMCEnabled {
		// Pointer parameters in SMC functions become TSMC references
		isTSMCRef = true
	}
	
	f.Params = append(f.Params, Parameter{
		Name: name,
		Type: typ,
		Reg:  reg,
		IsTSMCRef: isTSMCRef,
	})
	f.NumParams++
	return reg
}

// AddLocal adds a local variable
func (f *Function) AddLocal(name string, typ Type) Register {
	reg := f.AllocReg()
	f.Locals = append(f.Locals, Local{
		Name: name,
		Type: typ,
		Reg:  reg,
	})
	return reg
}

// Emit adds an instruction to the function
func (f *Function) Emit(op Opcode, dest, src1, src2 Register) {
	f.Instructions = append(f.Instructions, Instruction{
		Op:   op,
		Dest: dest,
		Src1: src1,
		Src2: src2,
	})
}

// EmitTyped adds an instruction with type information
func (f *Function) EmitTyped(op Opcode, dest, src1, src2 Register, typ Type) {
	f.Instructions = append(f.Instructions, Instruction{
		Op:   op,
		Dest: dest,
		Src1: src1,
		Src2: src2,
		Type: typ,
	})
}

// EmitImm adds an instruction with immediate value
func (f *Function) EmitImm(op Opcode, dest Register, imm int64) {
	f.Instructions = append(f.Instructions, Instruction{
		Op:   op,
		Dest: dest,
		Imm:  imm,
	})
}

// EmitLabel adds a label instruction
func (f *Function) EmitLabel(label string) {
	f.Instructions = append(f.Instructions, Instruction{
		Op:    OpLabel,
		Label: label,
	})
}

// EmitJump adds a jump instruction
func (f *Function) EmitJump(label string) {
	f.Instructions = append(f.Instructions, Instruction{
		Op:    OpJump,
		Label: label,
	})
}

// EmitJumpIf adds a conditional jump instruction
func (f *Function) EmitJumpIf(cond Register, label string) {
	f.Instructions = append(f.Instructions, Instruction{
		Op:    OpJumpIf,
		Src1:  cond,
		Label: label,
	})
}

// Module represents a collection of functions
type Module struct {
	Name      string
	Functions []*Function
	Globals   []Global
	Strings   []*String
	PatchTable []PatchEntry // TRUE SMC patch table
}

// Global represents a global variable
type Global struct {
	Name     string
	Type     Type
	Init     interface{} // Initial value
	Value    interface{} // AST expression for constants
	Constant bool        // Whether this is a constant
}

// String represents a string literal
type String struct {
	Label string
	Value string
}

// NewModule creates a new IR module
func NewModule(name string) *Module {
	return &Module{
		Name: name,
	}
}

// AddFunction adds a function to the module
func (m *Module) AddFunction(f *Function) {
	m.Functions = append(m.Functions, f)
}

// AddGlobal adds a global variable
func (m *Module) AddGlobal(name string, typ Type, init interface{}) {
	m.Globals = append(m.Globals, Global{
		Name: name,
		Type: typ,
		Init: init,
	})
}

// String representation for debugging
func (i *Instruction) String() string {
	switch i.Op {
	case OpNop:
		return "nop"
	case OpLabel:
		return i.Label + ":"
	case OpJump:
		return fmt.Sprintf("jump %s", i.Label)
	case OpJumpIf:
		return fmt.Sprintf("jump_if r%d, %s", i.Src1, i.Label)
	case OpJumpIfNot:
		return fmt.Sprintf("jump_if_not r%d, %s", i.Src1, i.Label)
	case OpCall:
		return fmt.Sprintf("r%d = call %s", i.Dest, i.Symbol)
	case OpCallIndirect:
		return fmt.Sprintf("r%d = call_indirect r%d", i.Dest, i.Src1)
	case OpReturn:
		if i.Src1 != 0 {
			return fmt.Sprintf("return r%d", i.Src1)
		}
		return "return"
	case OpLoadConst:
		return fmt.Sprintf("r%d = %d", i.Dest, i.Imm)
	case OpLoadVar:
		return fmt.Sprintf("r%d = load %s", i.Dest, i.Symbol)
	case OpStoreVar:
		return fmt.Sprintf("store %s, r%d", i.Symbol, i.Src1)
	case OpStoreTSMCRef:
		return fmt.Sprintf("store_tsmc_ref %s, r%d", i.Symbol, i.Src1)
	case OpLoadParam:
		return fmt.Sprintf("r%d = param %s", i.Dest, i.Symbol)
	case OpAdd:
		return fmt.Sprintf("r%d = r%d + r%d", i.Dest, i.Src1, i.Src2)
	case OpSub:
		return fmt.Sprintf("r%d = r%d - r%d", i.Dest, i.Src1, i.Src2)
	case OpMul:
		return fmt.Sprintf("r%d = r%d * r%d", i.Dest, i.Src1, i.Src2)
	case OpDiv:
		return fmt.Sprintf("r%d = r%d / r%d", i.Dest, i.Src1, i.Src2)
	case OpEq:
		return fmt.Sprintf("r%d = r%d == r%d", i.Dest, i.Src1, i.Src2)
	case OpNe:
		return fmt.Sprintf("r%d = r%d != r%d", i.Dest, i.Src1, i.Src2)
	case OpLt:
		return fmt.Sprintf("r%d = r%d < r%d", i.Dest, i.Src1, i.Src2)
	case OpGt:
		return fmt.Sprintf("r%d = r%d > r%d", i.Dest, i.Src1, i.Src2)
	case OpLe:
		return fmt.Sprintf("r%d = r%d <= r%d", i.Dest, i.Src1, i.Src2)
	case OpGe:
		return fmt.Sprintf("r%d = r%d >= r%d", i.Dest, i.Src1, i.Src2)
	case OpAsm:
		if i.AsmName != "" {
			return fmt.Sprintf("asm %s { %s }", i.AsmName, i.AsmCode)
		}
		return fmt.Sprintf("asm { %s }", i.AsmCode)
	case OpLoadAddr:
		return fmt.Sprintf("r%d = addr(%s)", i.Dest, i.Symbol)
	case OpCopyToBuffer:
		return fmt.Sprintf("copy [r%d] to buffer@%d size=%d", i.Src1, i.Imm, i.Imm2)
	case OpCopyFromBuffer:
		return fmt.Sprintf("copy buffer@%d to [r%d] size=%d", i.Imm, i.Dest, i.Imm2)
	case OpDJNZ:
		return fmt.Sprintf("djnz r%d, %s", i.Src1, i.Label)
	case OpLoadImm:
		return fmt.Sprintf("r%d = %d", i.Dest, i.Imm)
	case OpAddImm:
		return fmt.Sprintf("r%d = r%d + %d", i.Dest, i.Src1, i.Imm)
	case OpCmp:
		return fmt.Sprintf("cmp r%d, r%d", i.Src1, i.Src2)
	case OpLoadDirect:
		return fmt.Sprintf("r%d = [$%04X]", i.Dest, i.Imm)
	case OpStoreDirect:
		return fmt.Sprintf("[$%04X] = r%d", i.Imm, i.Src1)
	case OpLoad:
		return fmt.Sprintf("r%d = *r%d", i.Dest, i.Src1)
	case OpStore:
		return fmt.Sprintf("*r%d = r%d", i.Src1, i.Src2)
	case OpMod:
		return fmt.Sprintf("r%d = r%d %% r%d", i.Dest, i.Src1, i.Src2)
	case OpNeg:
		return fmt.Sprintf("r%d = -r%d", i.Dest, i.Src1)
	case OpInc:
		return fmt.Sprintf("r%d++", i.Src1)
	case OpDec:
		return fmt.Sprintf("r%d--", i.Src1)
	case OpAnd:
		return fmt.Sprintf("r%d = r%d & r%d", i.Dest, i.Src1, i.Src2)
	case OpOr:
		return fmt.Sprintf("r%d = r%d | r%d", i.Dest, i.Src1, i.Src2)
	case OpXor:
		return fmt.Sprintf("r%d = r%d ^ r%d", i.Dest, i.Src1, i.Src2)
	case OpNot:
		return fmt.Sprintf("r%d = ~r%d", i.Dest, i.Src1)
	case OpShl:
		return fmt.Sprintf("r%d = r%d << r%d", i.Dest, i.Src1, i.Src2)
	case OpShr:
		return fmt.Sprintf("r%d = r%d >> r%d", i.Dest, i.Src1, i.Src2)
	case OpPrint:
		return fmt.Sprintf("print(r%d)", i.Src1)
	case OpPrintU8:
		return fmt.Sprintf("print_u8(r%d)", i.Src1)
	case OpPrintU16:
		return fmt.Sprintf("print_u16(r%d)", i.Src1)
	case OpPrintI8:
		return fmt.Sprintf("print_i8(r%d)", i.Src1)
	case OpPrintI16:
		return fmt.Sprintf("print_i16(r%d)", i.Src1)
	case OpPrintBool:
		return fmt.Sprintf("print_bool(r%d)", i.Src1)
	case OpPrintString:
		return fmt.Sprintf("print_string(r%d)", i.Src1)
	case OpPrintStringDirect:
		return fmt.Sprintf("print_direct(\"%s\")", i.Symbol)
	case OpLoadString:
		return fmt.Sprintf("r%d = string(%s)", i.Dest, i.Symbol)
	case OpLen:
		return fmt.Sprintf("r%d = len(r%d)", i.Dest, i.Src1)
	case OpMemcpy:
		return fmt.Sprintf("memcpy([r%d], [r%d], r%d)", i.Dest, i.Src1, i.Src2)
	case OpMemset:
		return fmt.Sprintf("memset([r%d], r%d, r%d)", i.Dest, i.Src1, i.Src2)
	case OpLoadField:
		return fmt.Sprintf("r%d = r%d.field[%d]", i.Dest, i.Src1, i.Imm)
	case OpStoreField:
		return fmt.Sprintf("r%d.field[%d] = r%d", i.Src1, i.Imm, i.Src2)
	case OpAddr:
		return fmt.Sprintf("r%d = &r%d", i.Dest, i.Src1)
	case OpLoadLabel:
		return fmt.Sprintf("r%d = label %s", i.Dest, i.Symbol)
	default:
		return fmt.Sprintf("unknown op %d", i.Op)
	}
}

// SetMetadata sets a metadata value for the function
func (f *Function) SetMetadata(key, value string) {
	if f.Metadata == nil {
		f.Metadata = make(map[string]string)
	}
	f.Metadata[key] = value
}

// GetMetadata retrieves a metadata value for the function
func (f *Function) GetMetadata(key string) (string, bool) {
	if f.Metadata == nil {
		return "", false
	}
	value, ok := f.Metadata[key]
	return value, ok
}

// SMCAnchorInfo represents information about a TRUE SMC anchor
type SMCAnchorInfo struct {
	Symbol      string    // Anchor symbol (e.g., "x$imm0")
	Address     uint16    // Address in generated code
	Size        uint8     // 1 or 2 bytes
	Instruction Opcode    // The instruction containing the immediate
}

// PatchEntry represents an entry in the PATCH-TABLE
type PatchEntry struct {
	Symbol   string    // Anchor symbol (e.g., "x$imm0")
	Address  uint16    // Address to patch
	Size     uint8     // 1 or 2 bytes
	Bank     uint8     // Memory bank
	ParamTag string    // Parameter name
	Function string    // Function name
}

// String returns the string representation of an Opcode
func (op Opcode) String() string {
	switch op {
	case OpNop: return "NOP"
	case OpLabel: return "LABEL"
	case OpJump: return "JUMP"
	case OpJumpIf: return "JUMP_IF"
	case OpJumpIfNot: return "JUMP_IF_NOT"
	case OpJumpIfZero: return "JUMP_IF_ZERO"
	case OpJumpIfNotZero: return "JUMP_IF_NOT_ZERO"
	case OpCall: return "CALL"
	case OpCallIndirect: return "CALL_INDIRECT"
	case OpReturn: return "RETURN"
	case OpLoadConst: return "LOAD_CONST"
	case OpLoadVar: return "LOAD_VAR"
	case OpStoreVar: return "STORE_VAR"
	case OpLoadParam: return "LOAD_PARAM"
	case OpLoadField: return "LOAD_FIELD"
	case OpStoreField: return "STORE_FIELD"
	case OpLoadIndex: return "LOAD_INDEX"
	case OpStoreIndex: return "STORE_INDEX"
	case OpLoadBitField: return "LOAD_BIT_FIELD"
	case OpStoreBitField: return "STORE_BIT_FIELD"
	case OpMove: return "MOVE"
	case OpLoadLabel: return "LOAD_LABEL"
	case OpLoadDirect: return "LOAD_DIRECT"
	case OpStoreDirect: return "STORE_DIRECT"
	case OpAdd: return "ADD"
	case OpSub: return "SUB"
	case OpMul: return "MUL"
	case OpDiv: return "DIV"
	case OpMod: return "MOD"
	case OpNeg: return "NEG"
	case OpInc: return "INC"
	case OpDec: return "DEC"
	case OpAnd: return "AND"
	case OpOr: return "OR"
	case OpXor: return "XOR"
	case OpNot: return "NOT"
	case OpShl: return "SHL"
	case OpShr: return "SHR"
	default: return fmt.Sprintf("UNKNOWN_OP_%d", int(op))
	}
}