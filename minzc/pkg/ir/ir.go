package ir

import "fmt"

// Opcode represents an IR operation
type Opcode uint8

const (
	// Control flow
	OpNop Opcode = iota
	OpLabel
	OpJump
	OpJumpIf
	OpJumpIfNot
	OpCall
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
	OpMove
	
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
	
	// Stack
	OpPush
	OpPop
)

// Instruction represents a single IR instruction
type Instruction struct {
	Op           Opcode
	Dest         Register
	Src1         Register
	Src2         Register
	Imm          int64
	Label        string
	Symbol       string
	Type         Type
	Comment      string
	PhysicalRegs map[string]string // Maps virtual to physical registers
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
	Base Type
}

func (t *PointerType) Size() int {
	return 2 // 16-bit pointers on Z80
}

func (t *PointerType) String() string {
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
}

// Parameter represents a function parameter
type Parameter struct {
	Name string
	Type Type
	Reg  Register
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
		Name:         name,
		ReturnType:   returnType,
		NextReg:      1, // Start from 1, 0 is reserved
		NextRegister: 1, // Same as NextReg
	}
}

// AllocReg allocates a new virtual register
func (f *Function) AllocReg() Register {
	reg := f.NextReg
	f.NextReg++
	return reg
}

// AddParam adds a parameter to the function
func (f *Function) AddParam(name string, typ Type) Register {
	reg := f.AllocReg()
	f.Params = append(f.Params, Parameter{
		Name: name,
		Type: typ,
		Reg:  reg,
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
}

// Global represents a global variable
type Global struct {
	Name  string
	Type  Type
	Init  interface{} // Initial value
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
	default:
		return fmt.Sprintf("unknown op %d", i.Op)
	}
}