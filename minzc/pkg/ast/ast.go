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
	Name       string
	Params     []*Parameter
	ReturnType Type
	Body       *BlockStmt
	IsPublic   bool
	IsExport   bool
	StartPos   Position
	EndPos     Position
}

func (f *FunctionDecl) Pos() Position { return f.StartPos }
func (f *FunctionDecl) End() Position { return f.EndPos }
func (f *FunctionDecl) stmtNode()    {}
func (f *FunctionDecl) declNode()    {}

// Parameter represents a function parameter
type Parameter struct {
	Name     string
	Type     Type
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

// VarDecl represents a variable declaration
type VarDecl struct {
	Name      string
	Type      Type
	Value     Expression
	IsMutable bool
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
	Message  string
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

// LuaEval represents @lua_eval(...) that generates MinZ code
type LuaEval struct {
	Code     string
	StartPos Position
	EndPos   Position
}

func (l *LuaEval) Pos() Position { return l.StartPos }
func (l *LuaEval) End() Position { return l.EndPos }
func (l *LuaEval) stmtNode()     {}