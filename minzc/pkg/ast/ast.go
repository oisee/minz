package ast

// Node is the base interface for all AST nodes
type Node interface {
	Pos() Position
	End() Position
}

// Position represents a position in the source file
type Position struct {
	Line   int
	Column int
	Offset int
}

// File represents a MinZ source file
type File struct {
	Name         string
	ModuleName   string        // Explicit module declaration (optional)
	Imports      []*ImportStmt
	Declarations []Declaration
	StartPos     Position
	EndPos       Position
}

func (f *File) Pos() Position { return f.StartPos }
func (f *File) End() Position { return f.EndPos }

// Statement nodes
type Statement interface {
	Node
	stmtNode()
}

// Declaration nodes
type Declaration interface {
	Statement
	declNode()
}

// Expression nodes
type Expression interface {
	Node
	exprNode()
}

// ImportStmt represents an import statement
type ImportStmt struct {
	Path     string
	Alias    string
	StartPos Position
	EndPos   Position
}

func (i *ImportStmt) Pos() Position { return i.StartPos }
func (i *ImportStmt) End() Position { return i.EndPos }

// FunctionDecl represents a function declaration
type FunctionDecl struct {
	Name          string
	GenericParams []*GenericParam
	Params        []*Parameter
	ReturnType    Type
	ErrorType     Type        // Optional error type for functions ending with ?
	Body          *BlockStmt
	IsPublic      bool
	IsExport      bool
	Attributes    []*Attribute
	FunctionKind  FunctionKind  // Regular, Asm, or MIR
	StartPos      Position
	EndPos        Position
}

// FunctionKind represents the type of function body
type FunctionKind int

const (
	FunctionKindRegular FunctionKind = iota
	FunctionKindAsm                  // asm fun
	FunctionKindMIR                  // mir fun
)

func (f *FunctionDecl) Pos() Position { return f.StartPos }
func (f *FunctionDecl) End() Position { return f.EndPos }
func (f *FunctionDecl) stmtNode()    {}
func (f *FunctionDecl) declNode()    {}

// Parameter represents a function parameter
type Parameter struct {
	Name     string
	Type     Type
	IsSelf   bool  // true if this is a 'self' parameter
	StartPos Position
	EndPos   Position
}


// Type nodes
type Type interface {
	Node
	typeNode()
}

// PrimitiveType represents primitive types (u8, u16, i8, i16, bool, void)
type PrimitiveType struct {
	Name     string
	StartPos Position
	EndPos   Position
}

func (p *PrimitiveType) Pos() Position { return p.StartPos }
func (p *PrimitiveType) End() Position { return p.EndPos }
func (p *PrimitiveType) typeNode()    {}

// ArrayType represents array types
type ArrayType struct {
	ElementType Type
	Size        Expression
	StartPos    Position
	EndPos      Position
}

func (a *ArrayType) Pos() Position { return a.StartPos }
func (a *ArrayType) End() Position { return a.EndPos }
func (a *ArrayType) typeNode()    {}

// PointerType represents pointer types
type PointerType struct {
	BaseType   Type
	IsMutable  bool
	StartPos   Position
	EndPos     Position
}

func (p *PointerType) Pos() Position { return p.StartPos }
func (p *PointerType) End() Position { return p.EndPos }
func (p *PointerType) typeNode()    {}

// StructType represents struct types
type StructType struct {
	Fields   []*Field
	StartPos Position
	EndPos   Position
}

func (s *StructType) Pos() Position { return s.StartPos }
func (s *StructType) End() Position { return s.EndPos }
func (s *StructType) typeNode()    {}

// EnumType represents enum types
type EnumType struct {
	Name     string
	Variants []string
	StartPos Position
	EndPos   Position
}

func (e *EnumType) Pos() Position { return e.StartPos }
func (e *EnumType) End() Position { return e.EndPos }
func (e *EnumType) typeNode()    {}

// TypeIdentifier represents a named type reference
type TypeIdentifier struct {
	Name     string
	StartPos Position
	EndPos   Position
}

func (t *TypeIdentifier) Pos() Position { return t.StartPos }
func (t *TypeIdentifier) End() Position { return t.EndPos }
func (t *TypeIdentifier) typeNode()    {}

// ErrorType represents a type that can return an error (T?)
type ErrorType struct {
	ValueType Type
	StartPos  Position
	EndPos    Position
}

func (e *ErrorType) Pos() Position { return e.StartPos }
func (e *ErrorType) End() Position { return e.EndPos }
func (e *ErrorType) typeNode()    {}

// BitStructType represents bit-struct types
type BitStructType struct {
	UnderlyingType Type        // nil for u8 (default), or u16
	Fields         []*BitField
	StartPos       Position
	EndPos         Position
}

func (b *BitStructType) Pos() Position { return b.StartPos }
func (b *BitStructType) End() Position { return b.EndPos }
func (b *BitStructType) typeNode()    {}

// IteratorType represents iterator types
type IteratorType struct {
	ElementType Type
	StartPos    Position
	EndPos      Position
}

func (i *IteratorType) Pos() Position { return i.StartPos }
func (i *IteratorType) End() Position { return i.EndPos }
func (i *IteratorType) typeNode()    {}

// BitField represents a field in a bit struct
type BitField struct {
	Name     string
	BitWidth int        // Number of bits (1-16)
	StartPos Position
	EndPos   Position
}

// Field represents a struct field
type Field struct {
	Name     string
	Type     Type
	IsPublic bool
	StartPos Position
	EndPos   Position
}

// Statements

// BlockStmt represents a block statement
type BlockStmt struct {
	Statements []Statement
	StartPos   Position
	EndPos     Position
}

func (b *BlockStmt) Pos() Position { return b.StartPos }
func (b *BlockStmt) End() Position { return b.EndPos }
func (b *BlockStmt) stmtNode()    {}
func (b *BlockStmt) exprNode()    {} // BlockStmt can also be used as expression

// VarDecl represents a variable declaration
type VarDecl struct {
	Name      string
	Type      Type
	Value     Expression
	IsMutable bool
	IsPublic  bool
	StartPos  Position
	EndPos    Position
}

func (v *VarDecl) Pos() Position { return v.StartPos }
func (v *VarDecl) End() Position { return v.EndPos }
func (v *VarDecl) stmtNode()    {}
func (v *VarDecl) declNode()    {}

// ConstDecl represents a constant declaration
type ConstDecl struct {
	Name     string
	Type     Type
	Value    Expression
	IsPublic bool
	StartPos Position
	EndPos   Position
}

func (c *ConstDecl) Pos() Position { return c.StartPos }
func (c *ConstDecl) End() Position { return c.EndPos }
func (c *ConstDecl) stmtNode()    {}
func (c *ConstDecl) declNode()    {}

// StructDecl represents a struct declaration
type StructDecl struct {
	Name     string
	Fields   []*Field
	IsPublic bool
	StartPos Position
	EndPos   Position
}

func (s *StructDecl) Pos() Position { return s.StartPos }
func (s *StructDecl) End() Position { return s.EndPos }
func (s *StructDecl) stmtNode()    {}
func (s *StructDecl) declNode()    {}

// EnumDecl represents an enum declaration
type EnumDecl struct {
	Name     string
	Variants []string
	IsPublic bool
	StartPos Position
	EndPos   Position
}

func (e *EnumDecl) Pos() Position { return e.StartPos }
func (e *EnumDecl) End() Position { return e.EndPos }
func (e *EnumDecl) stmtNode()    {}
func (e *EnumDecl) declNode()    {}

// TypeDecl represents a type alias declaration (including bit structs)
type TypeDecl struct {
	Name     string
	Type     Type    // Can be BitStructType
	IsPublic bool
	StartPos Position
	EndPos   Position
}

func (t *TypeDecl) Pos() Position { return t.StartPos }
func (t *TypeDecl) End() Position { return t.EndPos }
func (t *TypeDecl) stmtNode()    {}
func (t *TypeDecl) declNode()    {}

// InterfaceDecl represents an interface declaration
type InterfaceDecl struct {
	Name           string
	GenericParams  []*GenericParam
	Methods        []*InterfaceMethod
	IsPublic       bool
	StartPos       Position
	EndPos         Position
}

func (i *InterfaceDecl) Pos() Position { return i.StartPos }
func (i *InterfaceDecl) End() Position { return i.EndPos }
func (i *InterfaceDecl) stmtNode()    {}
func (i *InterfaceDecl) declNode()    {}

// InterfaceMethod represents a method signature in an interface
type InterfaceMethod struct {
	Name       string
	Params     []*Parameter
	ReturnType Type
	StartPos   Position
	EndPos     Position
}

// ImplBlock represents an implementation of an interface for a type
type ImplBlock struct {
	InterfaceName string
	ForType       Type
	Methods       []*FunctionDecl
	StartPos      Position
	EndPos        Position
}

func (i *ImplBlock) Pos() Position { return i.StartPos }
func (i *ImplBlock) End() Position { return i.EndPos }
func (i *ImplBlock) stmtNode()    {}
func (i *ImplBlock) declNode()    {}

// GenericParam represents a generic type parameter
type GenericParam struct {
	Name       string
	Bounds     []string  // Interface bounds
	StartPos   Position
	EndPos     Position
}

// ReturnStmt represents a return statement
type ReturnStmt struct {
	Value    Expression
	StartPos Position
	EndPos   Position
}

func (r *ReturnStmt) Pos() Position { return r.StartPos }
func (r *ReturnStmt) End() Position { return r.EndPos }
func (r *ReturnStmt) stmtNode()    {}

// IfStmt represents an if statement
type IfStmt struct {
	Condition Expression
	Then      *BlockStmt
	Else      Statement
	StartPos  Position
	EndPos    Position
}

func (i *IfStmt) Pos() Position { return i.StartPos }
func (i *IfStmt) End() Position { return i.EndPos }
func (i *IfStmt) stmtNode()    {}

// WhileStmt represents a while statement
type WhileStmt struct {
	Condition Expression
	Body      *BlockStmt
	StartPos  Position
	EndPos    Position
}

func (w *WhileStmt) Pos() Position { return w.StartPos }
func (w *WhileStmt) End() Position { return w.EndPos }
func (w *WhileStmt) stmtNode()    {}

// ForStmt represents a for-in statement
type ForStmt struct {
	Iterator string     // Loop variable name
	Range    Expression // Range expression (e.g., 0..10)
	Body     *BlockStmt
	StartPos Position
	EndPos   Position
}

func (f *ForStmt) Pos() Position { return f.StartPos }
func (f *ForStmt) End() Position { return f.EndPos }
func (f *ForStmt) stmtNode()    {}

// AsmStmt represents an inline assembly block
type AsmStmt struct {
	Name     string   // Optional name for named blocks
	Code     string   // Raw assembly text
	StartPos Position
	EndPos   Position
}

func (a *AsmStmt) Pos() Position { return a.StartPos }
func (a *AsmStmt) End() Position { return a.EndPos }
func (a *AsmStmt) stmtNode()    {}

// MIRStmt represents inline MIR block: mir { ... }
type MIRStmt struct {
	Instructions []*MIRInstruction // Parsed MIR instructions
	Code         string            // Raw MIR text (for passthrough)
	StartPos     Position
	EndPos       Position
}

func (m *MIRStmt) Pos() Position { return m.StartPos }
func (m *MIRStmt) End() Position { return m.EndPos }
func (m *MIRStmt) stmtNode()    {}

// MIRInstruction represents a single MIR instruction
type MIRInstruction struct {
	Label    string       // Optional label
	Opcode   string       // MIR opcode (load, store, add, etc.)
	Operands []MIROperand // Instruction operands
	StartPos Position
	EndPos   Position
}

// MIROperand represents an operand in a MIR instruction
type MIROperand interface {
	Node
	mirOperandNode()
}

// MIRRegister represents a MIR register (r0, r1, etc.)
type MIRRegister struct {
	Number   int
	StartPos Position
	EndPos   Position
}

func (r *MIRRegister) Pos() Position     { return r.StartPos }
func (r *MIRRegister) End() Position     { return r.EndPos }
func (r *MIRRegister) mirOperandNode()   {}

// MIRImmediate represents a MIR immediate value (#123)
type MIRImmediate struct {
	Value    int64
	StartPos Position
	EndPos   Position
}

func (i *MIRImmediate) Pos() Position     { return i.StartPos }
func (i *MIRImmediate) End() Position     { return i.EndPos }
func (i *MIRImmediate) mirOperandNode()   {}

// MIRMemory represents a MIR memory reference ([r1], [label])
type MIRMemory struct {
	Base     MIROperand // Register or label
	StartPos Position
	EndPos   Position
}

func (m *MIRMemory) Pos() Position     { return m.StartPos }
func (m *MIRMemory) End() Position     { return m.EndPos }
func (m *MIRMemory) mirOperandNode()   {}

// MIRLabel represents a label reference in MIR
type MIRLabel struct {
	Name     string
	StartPos Position
	EndPos   Position
}

func (l *MIRLabel) Pos() Position     { return l.StartPos }
func (l *MIRLabel) End() Position     { return l.EndPos }
func (l *MIRLabel) mirOperandNode()   {}

// LoopStmt represents a loop statement for iterating over tables
type LoopStmt struct {
	Table      Expression  // Table/array to iterate over
	Mode       LoopMode    // INTO or REF_TO
	Iterator   string      // Variable name for current element
	Index      string      // Optional index variable (for indexed loops)
	Condition  Expression  // Optional where clause (future)
	Body       *BlockStmt  // Loop body
	StartPos   Position
	EndPos     Position
}

// LoopMode represents the iteration mode
type LoopMode int

const (
	LoopInto  LoopMode = iota // Copy element to buffer
	LoopRefTo                  // Reference to element
)

func (l *LoopStmt) Pos() Position { return l.StartPos }
func (l *LoopStmt) End() Position { return l.EndPos }
func (l *LoopStmt) stmtNode()    {}

// DoTimesStmt represents a counting loop (do N times)
type DoTimesStmt struct {
	Count    Expression // Number of iterations
	Body     *BlockStmt
	StartPos Position
	EndPos   Position
}

func (d *DoTimesStmt) Pos() Position { return d.StartPos }
func (d *DoTimesStmt) End() Position { return d.EndPos }
func (d *DoTimesStmt) stmtNode()    {}

// LoopAtStmt represents array iteration (loop at array -> item)
type LoopAtStmt struct {
	Table       Expression // Array/table to iterate
	Iterator    string     // Iterator variable name
	IsModifying bool       // Has ! prefix (auto write-back)
	Body        *BlockStmt
	StartPos    Position
	EndPos      Position
}

func (l *LoopAtStmt) Pos() Position { return l.StartPos }
func (l *LoopAtStmt) End() Position { return l.EndPos }
func (l *LoopAtStmt) stmtNode()    {}

// CaseStmt represents a case (pattern matching) statement
type CaseStmt struct {
	Expr     Expression
	Arms     []*CaseArm
	StartPos Position
	EndPos   Position
}

func (c *CaseStmt) Pos() Position { return c.StartPos }
func (c *CaseStmt) End() Position { return c.EndPos }
func (c *CaseStmt) stmtNode()    {}

// CaseArm represents a single arm of a case statement
type CaseArm struct {
	Pattern  Pattern
	Guard    Expression // Optional guard expression (if clause)
	Body     Node       // Can be Expression or BlockStmt
	StartPos Position
	EndPos   Position
}

func (c *CaseArm) Pos() Position { return c.StartPos }
func (c *CaseArm) End() Position { return c.EndPos }

// Pattern represents a pattern in pattern matching
type Pattern interface {
	Node
	patternNode()
}

// IdentifierPattern represents an identifier pattern (including enum variants)
type IdentifierPattern struct {
	Name     string
	StartPos Position
	EndPos   Position
}

func (i *IdentifierPattern) Pos() Position { return i.StartPos }
func (i *IdentifierPattern) End() Position { return i.EndPos }
func (i *IdentifierPattern) patternNode() {}

// LiteralPattern represents a literal pattern
type LiteralPattern struct {
	Value    Expression // NumberLiteral, StringLiteral, BooleanLiteral, etc.
	StartPos Position
	EndPos   Position
}

func (l *LiteralPattern) Pos() Position { return l.StartPos }
func (l *LiteralPattern) End() Position { return l.EndPos }
func (l *LiteralPattern) patternNode() {}

// WildcardPattern represents the wildcard pattern (_)
type WildcardPattern struct {
	StartPos Position
	EndPos   Position
}

func (w *WildcardPattern) Pos() Position { return w.StartPos }
func (w *WildcardPattern) End() Position { return w.EndPos }
func (w *WildcardPattern) patternNode() {}

// ExpressionStmt represents an expression used as a statement
type ExpressionStmt struct {
	Expression Expression
	StartPos   Position
	EndPos     Position
}

// AssignStmt represents an assignment statement
type AssignStmt struct {
	Target   Expression
	Value    Expression
	StartPos Position
	EndPos   Position
}

func (a *AssignStmt) Pos() Position { return a.StartPos }
func (a *AssignStmt) End() Position { return a.EndPos }
func (a *AssignStmt) stmtNode()    {}

// InlineAsmExpr represents inline assembly used as an expression (GCC-style)
type InlineAsmExpr struct {
	Code     string
	StartPos Position
	EndPos   Position
}

func (i *InlineAsmExpr) Pos() Position { return i.StartPos }
func (i *InlineAsmExpr) End() Position { return i.EndPos }
func (i *InlineAsmExpr) exprNode()    {}

func (e *ExpressionStmt) Pos() Position { return e.StartPos }
func (e *ExpressionStmt) End() Position { return e.EndPos }
func (e *ExpressionStmt) stmtNode()    {}

// Expressions

// Identifier represents an identifier
type Identifier struct {
	Name     string
	StartPos Position
	EndPos   Position
}

func (i *Identifier) Pos() Position { return i.StartPos }
func (i *Identifier) End() Position { return i.EndPos }
func (i *Identifier) exprNode()    {}

// NumberLiteral represents a number literal
type NumberLiteral struct {
	Value    int64
	StartPos Position
	EndPos   Position
}

func (n *NumberLiteral) Pos() Position { return n.StartPos }
func (n *NumberLiteral) End() Position { return n.EndPos }
func (n *NumberLiteral) exprNode()    {}

// BooleanLiteral represents a boolean literal
type BooleanLiteral struct {
	Value    bool
	StartPos Position
	EndPos   Position
}

func (b *BooleanLiteral) Pos() Position { return b.StartPos }
func (b *BooleanLiteral) End() Position { return b.EndPos }
func (b *BooleanLiteral) exprNode()    {}

// BinaryExpr represents a binary expression
type BinaryExpr struct {
	Left     Expression
	Operator string
	Right    Expression
	StartPos Position
	EndPos   Position
}

func (b *BinaryExpr) Pos() Position { return b.StartPos }
func (b *BinaryExpr) End() Position { return b.EndPos }
func (b *BinaryExpr) exprNode()    {}

// UnaryExpr represents a unary expression
type UnaryExpr struct {
	Operator string
	Operand  Expression
	StartPos Position
	EndPos   Position
}

func (u *UnaryExpr) Pos() Position { return u.StartPos }
func (u *UnaryExpr) End() Position { return u.EndPos }
func (u *UnaryExpr) exprNode()    {}

// CallExpr represents a function call
type CallExpr struct {
	Function  Expression
	Arguments []Expression
	StartPos  Position
	EndPos    Position
}

func (c *CallExpr) Pos() Position { return c.StartPos }
func (c *CallExpr) End() Position { return c.EndPos }
func (c *CallExpr) exprNode()    {}

// FieldExpr represents field access
type FieldExpr struct {
	Object   Expression
	Field    string
	StartPos Position
	EndPos   Position
}

func (f *FieldExpr) Pos() Position { return f.StartPos }
func (f *FieldExpr) End() Position { return f.EndPos }
func (f *FieldExpr) exprNode()    {}

// IndexExpr represents array indexing
type IndexExpr struct {
	Array    Expression
	Index    Expression
	StartPos Position
	EndPos   Position
}

func (i *IndexExpr) Pos() Position { return i.StartPos }
func (i *IndexExpr) End() Position { return i.EndPos }
func (i *IndexExpr) exprNode()    {}

// TryExpr represents the ? operator for error propagation
type TryExpr struct {
	Expression Expression
	StartPos   Position
	EndPos     Position
}

func (t *TryExpr) Pos() Position { return t.StartPos }
func (t *TryExpr) End() Position { return t.EndPos }
func (t *TryExpr) exprNode()    {}

// StructLiteral represents a struct literal expression
type StructLiteral struct {
	TypeName string
	Fields   []*FieldInit
	StartPos Position
	EndPos   Position
}

func (s *StructLiteral) Pos() Position { return s.StartPos }
func (s *StructLiteral) End() Position { return s.EndPos }
func (s *StructLiteral) exprNode()    {}

// FieldInit represents a field initialization in a struct literal
type FieldInit struct {
	Name  string
	Value Expression
}

// EnumLiteral represents an enum variant reference
type EnumLiteral struct {
	EnumName string
	Variant  string
	StartPos Position
	EndPos   Position
}

func (e *EnumLiteral) Pos() Position { return e.StartPos }
func (e *EnumLiteral) End() Position { return e.EndPos }
func (e *EnumLiteral) exprNode()    {}

// CompileTimeIf represents @if compile-time conditional
type CompileTimeIf struct {
	Condition Expression
	ThenExpr  Expression
	ElseExpr  Expression // Optional
	StartPos  Position
	EndPos    Position
}

func (c *CompileTimeIf) Pos() Position { return c.StartPos }
func (c *CompileTimeIf) End() Position { return c.EndPos }
func (c *CompileTimeIf) exprNode()    {}

// CompileTimePrint represents @print compile-time output
type CompileTimePrint struct {
	Expr     Expression // Changed from Message to support interpolation
	StartPos Position
	EndPos   Position
}

func (c *CompileTimePrint) Pos() Position { return c.StartPos }
func (c *CompileTimePrint) End() Position { return c.EndPos }
func (c *CompileTimePrint) exprNode()    {}

// CompileTimeAssert represents @assert compile-time assertion
type CompileTimeAssert struct {
	Condition Expression
	Message   string // Optional
	StartPos  Position
	EndPos    Position
}

func (c *CompileTimeAssert) Pos() Position { return c.StartPos }
func (c *CompileTimeAssert) End() Position { return c.EndPos }
func (c *CompileTimeAssert) exprNode()    {}

// CompileTimeError represents @error for setting CY flag and returning error
type CompileTimeError struct {
	ErrorValue Expression // Error.ErrorType or custom error value
	StartPos   Position
	EndPos     Position
}

func (c *CompileTimeError) Pos() Position { return c.StartPos }
func (c *CompileTimeError) End() Position { return c.EndPos }
func (c *CompileTimeError) exprNode()     {}

// Attribute represents @attribute declarations
type Attribute struct {
	Name      string
	Arguments []Expression
	StartPos  Position
	EndPos    Position
}

func (a *Attribute) Pos() Position { return a.StartPos }
func (a *Attribute) End() Position { return a.EndPos }
func (a *Attribute) exprNode()    {}

// LuaBlock represents @lua[[...]] compile-time Lua code
type LuaBlock struct {
	Code     string
	StartPos Position
	EndPos   Position
}

func (l *LuaBlock) Pos() Position { return l.StartPos }
func (l *LuaBlock) End() Position { return l.EndPos }
func (l *LuaBlock) stmtNode()     {}
func (l *LuaBlock) declNode()     {} // LuaBlock can be a top-level declaration

// LuaExpression represents @lua(...) compile-time Lua expression
type LuaExpression struct {
	Code     string
	StartPos Position
	EndPos   Position
}

func (l *LuaExpression) Pos() Position { return l.StartPos }
func (l *LuaExpression) End() Position { return l.EndPos }
func (l *LuaExpression) exprNode()    {}

// LuaExpr is an alias for compatibility
type LuaExpr = LuaExpression

// StringLiteral represents a string literal
type StringLiteral struct {
	Value    string
	StartPos Position
	EndPos   Position
}

func (s *StringLiteral) Pos() Position { return s.StartPos }
func (s *StringLiteral) End() Position { return s.EndPos }
func (s *StringLiteral) exprNode()    {}

// ArrayInitializer represents an array initializer expression with {...} syntax
type ArrayInitializer struct {
	Elements []Expression
	StartPos Position
	EndPos   Position
}

func (a *ArrayInitializer) Pos() Position { return a.StartPos }
func (a *ArrayInitializer) End() Position { return a.EndPos }
func (a *ArrayInitializer) exprNode()    {}

// CastExpr represents a type cast expression (e.g., value as Type)
type CastExpr struct {
	Expr     Expression
	TargetType Type
	StartPos Position
	EndPos   Position
}

func (c *CastExpr) Pos() Position { return c.StartPos }
func (c *CastExpr) End() Position { return c.EndPos }
func (c *CastExpr) exprNode()    {}

// LuaEval represents @lua_eval(...) that generates MinZ code
type LuaEval struct {
	Code     string
	StartPos Position
	EndPos   Position
}

func (l *LuaEval) Pos() Position { return l.StartPos }
func (l *LuaEval) End() Position { return l.EndPos }

// MetafunctionCall represents @function_name(...) calls
type MetafunctionCall struct {
	Name      string
	Arguments []Expression
	StartPos  Position
	EndPos    Position
}

func (m *MetafunctionCall) Pos() Position { return m.StartPos }
func (m *MetafunctionCall) End() Position { return m.EndPos }
func (m *MetafunctionCall) exprNode()    {}

// MinzMetafunctionCall represents @minz("code", args...) calls for compile-time code generation
type MinzMetafunctionCall struct {
	Code      string       // MinZ code to execute at compile time
	Arguments []Expression // Arguments to pass to the MinZ code
	StartPos  Position
	EndPos    Position
}

func (m *MinzMetafunctionCall) Pos() Position { return m.StartPos }
func (m *MinzMetafunctionCall) End() Position { return m.EndPos }
func (m *MinzMetafunctionCall) exprNode()    {}

// InlineAssembly represents inline assembly code
type InlineAssembly struct {
	Code     string         // The assembly code
	Outputs  []*AsmOperand  // Output operands
	Inputs   []*AsmOperand  // Input operands
	Clobbers []string       // Clobbered registers
	StartPos Position
	EndPos   Position
}

func (i *InlineAssembly) Pos() Position { return i.StartPos }
func (i *InlineAssembly) End() Position { return i.EndPos }
func (i *InlineAssembly) exprNode()    {}

// AsmOperand represents an inline assembly operand
type AsmOperand struct {
	Constraint string     // Constraint string (e.g., "r", "m", "i")
	Expr       Expression // The expression for this operand
}
func (l *LuaEval) stmtNode()     {}

// LambdaExpr represents a lambda expression
type LambdaExpr struct {
	Params      []*LambdaParam
	ReturnType  Type           // Optional
	Body        Node           // Can be Expression or BlockStmt
	Captures    []string       // Variables captured from scope
	StartPos    Position
	EndPos      Position
}

func (l *LambdaExpr) Pos() Position { return l.StartPos }
func (l *LambdaExpr) End() Position { return l.EndPos }
func (l *LambdaExpr) exprNode()    {}

// LambdaParam represents a lambda parameter
type LambdaParam struct {
	Name     string
	Type     Type  // Optional
	StartPos Position
	EndPos   Position
}

// MetafunctionDecl represents a top-level @minz metafunction call that generates declarations
type MetafunctionDecl struct {
	Code      string       // Template code  
	Arguments []Expression // Arguments for substitution
	StartPos  Position
	EndPos    Position
}

func (m *MetafunctionDecl) Pos() Position { return m.StartPos }
func (m *MetafunctionDecl) End() Position { return m.EndPos }
func (m *MetafunctionDecl) stmtNode()     {}
func (m *MetafunctionDecl) declNode()     {}

// NilCoalescingExpr represents the ?? operator for error handling
type NilCoalescingExpr struct {
	Left     Expression // Expression that might set CY flag  
	Right    Expression // Default value when CY is set
	StartPos Position
	EndPos   Position
}

func (n *NilCoalescingExpr) Pos() Position { return n.StartPos }
func (n *NilCoalescingExpr) End() Position { return n.EndPos }
func (n *NilCoalescingExpr) exprNode()     {}

// IfExpr represents value-returning if expressions (if cond { val1 } else { val2 })
type IfExpr struct {
	Condition  Expression
	ThenBranch Expression  // Can be BlockStmt converted to expression
	ElseBranch Expression  // Optional
	StartPos   Position
	EndPos     Position
}

func (i *IfExpr) Pos() Position { return i.StartPos }
func (i *IfExpr) End() Position { return i.EndPos }
func (i *IfExpr) exprNode()     {}

// TernaryExpr represents Python-style conditional (value_if_true if condition else value_if_false)
type TernaryExpr struct {
	TrueExpr  Expression
	Condition Expression
	FalseExpr Expression
	StartPos  Position
	EndPos    Position
}

func (t *TernaryExpr) Pos() Position { return t.StartPos }
func (t *TernaryExpr) End() Position { return t.EndPos }
func (t *TernaryExpr) exprNode()     {}

// WhenExpr represents pattern matching expressions
type WhenExpr struct {
	Value    Expression      // Optional value to match against
	Arms     []*WhenArm      // Match arms
	StartPos Position
	EndPos   Position
}

func (w *WhenExpr) Pos() Position { return w.StartPos }
func (w *WhenExpr) End() Position { return w.EndPos }
func (w *WhenExpr) exprNode()     {}

// WhenArm represents a single arm in a when expression
type WhenArm struct {
	Pattern  Expression  // Pattern or condition
	Guard    Expression  // Optional guard condition (if clause)
	Body     Expression  // Result expression
	IsElse   bool        // true for 'else' arm
	StartPos Position
	EndPos   Position
}