package participle

import (
	"github.com/alecthomas/participle/v2/lexer"
)

// Position represents a source location.
type Position struct {
	Filename string
	Offset   int
	Line     int
	Column   int
}

// Pos converts a lexer.Position to our Position type.
func Pos(p lexer.Position) Position {
	return Position{
		Filename: p.Filename,
		Offset:   p.Offset,
		Line:     p.Line,
		Column:   p.Column,
	}
}

// Expression is the top-level expression type.
type Expression struct {
	Pos lexer.Position

	Ternary *TernaryExpr `parser:"@@"`
}

// TernaryExpr handles ternary conditionals: cond ? then : else
type TernaryExpr struct {
	Pos lexer.Position

	Condition *OrExpr     `parser:"@@"`
	Then      *Expression `parser:"( '?' @@"`
	Else      *Expression `parser:"  ':' @@ )?"`
}

// OrExpr handles || operator (lowest precedence binary)
type OrExpr struct {
	Pos lexer.Position

	Left  *AndExpr `parser:"@@"`
	Right []*OrExprTail `parser:"@@*"`
}

type OrExprTail struct {
	Op    string   `parser:"@OrOr"`
	Right *AndExpr `parser:"@@"`
}

// AndExpr handles && operator
type AndExpr struct {
	Pos lexer.Position

	Left  *BitOrExpr `parser:"@@"`
	Right []*AndExprTail `parser:"@@*"`
}

type AndExprTail struct {
	Op    string     `parser:"@AndAnd"`
	Right *BitOrExpr `parser:"@@"`
}

// BitOrExpr handles | operator
type BitOrExpr struct {
	Pos lexer.Position

	Left  *BitXorExpr `parser:"@@"`
	Right []*BitOrExprTail `parser:"@@*"`
}

type BitOrExprTail struct {
	Op    string      `parser:"@Pipe"`
	Right *BitXorExpr `parser:"@@"`
}

// BitXorExpr handles ^ operator
type BitXorExpr struct {
	Pos lexer.Position

	Left  *BitAndExpr `parser:"@@"`
	Right []*BitXorExprTail `parser:"@@*"`
}

type BitXorExprTail struct {
	Op    string      `parser:"@Caret"`
	Right *BitAndExpr `parser:"@@"`
}

// BitAndExpr handles & operator
type BitAndExpr struct {
	Pos lexer.Position

	Left  *EqualityExpr `parser:"@@"`
	Right []*BitAndExprTail `parser:"@@*"`
}

type BitAndExprTail struct {
	Op    string        `parser:"@Amp"`
	Right *EqualityExpr `parser:"@@"`
}

// EqualityExpr handles == and != operators
type EqualityExpr struct {
	Pos lexer.Position

	Left  *ComparisonExpr `parser:"@@"`
	Right []*EqualityExprTail `parser:"@@*"`
}

type EqualityExprTail struct {
	Op    string          `parser:"@( EqEq | NotEq )"`
	Right *ComparisonExpr `parser:"@@"`
}

// ComparisonExpr handles <, >, <=, >= operators
type ComparisonExpr struct {
	Pos lexer.Position

	Left  *ShiftExpr `parser:"@@"`
	Right []*ComparisonExprTail `parser:"@@*"`
}

type ComparisonExprTail struct {
	Op    string     `parser:"@( LtEq | GtEq | Lt | Gt )"`
	Right *ShiftExpr `parser:"@@"`
}

// ShiftExpr handles << and >> operators
type ShiftExpr struct {
	Pos lexer.Position

	Left  *AddExpr `parser:"@@"`
	Right []*ShiftExprTail `parser:"@@*"`
}

type ShiftExprTail struct {
	Op    string   `parser:"@( LtLt | GtGt )"`
	Right *AddExpr `parser:"@@"`
}

// AddExpr handles + and - operators
type AddExpr struct {
	Pos lexer.Position

	Left  *MulExpr `parser:"@@"`
	Right []*AddExprTail `parser:"@@*"`
}

type AddExprTail struct {
	Op    string   `parser:"@( Plus | Minus )"`
	Right *MulExpr `parser:"@@"`
}

// MulExpr handles *, /, % operators
type MulExpr struct {
	Pos lexer.Position

	Left  *UnaryExpr `parser:"@@"`
	Right []*MulExprTail `parser:"@@*"`
}

type MulExprTail struct {
	Op    string     `parser:"@( Star | Slash | Percent )"`
	Right *UnaryExpr `parser:"@@"`
}

// UnaryExpr handles prefix operators: -, !, ~, &, *
type UnaryExpr struct {
	Pos lexer.Position

	Op      string       `parser:"( @( Minus | Bang | Tilde | Amp | Star )"`
	Operand *UnaryExpr   `parser:"  @@ )"`
	Postfix *PostfixExpr `parser:"| @@"`
}

// PostfixExpr handles function calls, indexing, field access, and casting
type PostfixExpr struct {
	Pos lexer.Position

	Primary  *PrimaryExpr `parser:"@@"`
	Suffixes []*PostfixOp `parser:"@@*"`
}

// PostfixOp represents a single postfix operation
type PostfixOp struct {
	Pos lexer.Position

	Call  *CallOp  `parser:"  @@"`
	Index *IndexOp `parser:"| @@"`
	Field *FieldOp `parser:"| @@"`
	Cast  *CastOp  `parser:"| @@"`
}

// CallOp represents a function call: (args)
type CallOp struct {
	Args []*Expression `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
}

// IndexOp represents array indexing: [index]
type IndexOp struct {
	Index *Expression `parser:"'[' @@ ']'"`
}

// FieldOp represents field access: .field
type FieldOp struct {
	Field string `parser:"'.' @Ident"`
}

// CastOp represents type casting: as Type
type CastOp struct {
	Type *TypeRef `parser:"'as' @@"`
}

// PrimaryExpr handles literals, identifiers, parenthesized expressions, etc.
type PrimaryExpr struct {
	Pos lexer.Position

	Number    *string       `parser:"  @Number"`
	HexNumber *string       `parser:"| @HexNumber"`
	BinNumber *string       `parser:"| @BinNumber"`
	String    *string       `parser:"| @String"`
	Char      *string       `parser:"| @Char"`
	RawString *string       `parser:"| @RawString"`
	True      bool          `parser:"| @'true'"`
	False     bool          `parser:"| @'false'"`
	Nil       bool          `parser:"| @( 'nil' | 'null' )"`
	ScopedId  *ScopedIdent  `parser:"| @@"`
	Ident     *string       `parser:"| @Ident"`
	Paren     *Expression   `parser:"| '(' @@ ')'"`
	Array     *ArrayLiteral `parser:"| @@"`
	Lambda    *LambdaExpr   `parser:"| @@"`
	Meta      *MetaExpr     `parser:"| @@"`
}

// ScopedIdent represents Type::member access (e.g., State::IDLE)
type ScopedIdent struct {
	Scope  string `parser:"@Ident ColonColon"`
	Member string `parser:"@Ident"`
}

// ArrayLiteral represents [elem, elem, ...]
type ArrayLiteral struct {
	Pos lexer.Position

	Elements []*Expression `parser:"'[' ( @@ ( ',' @@ )* )? ']'"`
}

// LambdaExpr represents |params| => body or |params| { body }
type LambdaExpr struct {
	Pos lexer.Position

	Params     []*LambdaParam `parser:"'|' ( @@ ( ',' @@ )* )? '|'"`
	ReturnType *TypeRef       `parser:"( Arrow @@ )?"`
	ArrowBody  *Expression    `parser:"( FatArrow @@"`
	BlockBody  *Block         `parser:"| @@ )?"`
}

// LambdaParam represents a lambda parameter
type LambdaParam struct {
	Name string   `parser:"@Ident"`
	Type *TypeRef `parser:"( ':' @@ )?"`
}

// MetaExpr represents @metafunction(...)
type MetaExpr struct {
	Pos lexer.Position

	Name string        `parser:"'@' @Ident"`
	Args []*Expression `parser:"( '(' ( @@ ( ',' @@ )* )? ')' )?"`
}

// TypeRef represents a type reference
type TypeRef struct {
	Pos lexer.Position

	Pointer bool       `parser:"@Star?"`
	Array   *ArrayType `parser:"( @@"`
	Name    *string    `parser:"| @Ident )"`
	Generic []*TypeRef `parser:"( Lt @@ ( ',' @@ )* Gt )?"`
}

// ArrayType represents [N]Type or []Type
type ArrayType struct {
	Size    *string  `parser:"'[' @Number? ']'"`
	Element *TypeRef `parser:"@@"`
}

// Block represents { statements }
type Block struct {
	Pos lexer.Position

	Statements []*Statement `parser:"'{' @@* '}'"`
}

// Statement placeholder - will be expanded in stmt.go
type Statement struct {
	Pos lexer.Position

	Expr *Expression `parser:"@@ ';'?"`
}
