package participle

import (
	"github.com/alecthomas/participle/v2/lexer"
)

// StmtBlock represents a block of statements with proper statement parsing
type StmtBlock struct {
	Pos lexer.Position

	Statements []*Stmt `parser:"'{' @@* '}'"`
}

// Stmt represents any statement in MinZ
type Stmt struct {
	Pos lexer.Position

	Let      *LetStmt      `parser:"  @@"`
	Const    *ConstStmt    `parser:"| @@"`
	Return   *ReturnStmt   `parser:"| @@"`
	If       *IfStmt       `parser:"| @@"`
	While    *WhileStmt    `parser:"| @@"`
	For      *ForStmt      `parser:"| @@"`
	Loop     *LoopStmt     `parser:"| @@"`
	Break    *BreakStmt    `parser:"| @@"`
	Continue *ContinueStmt `parser:"| @@"`
	Assign   *AssignStmt   `parser:"| @@"`
	Expr     *ExprStmt     `parser:"| @@"`
}

// LetStmt represents: let name: Type = value;
type LetStmt struct {
	Pos lexer.Position

	Mutable bool        `parser:"'let' @'mut'?"`
	Name    string      `parser:"@Ident"`
	Type    *TypeRef    `parser:"( ':' @@ )?"`
	Value   *Expression `parser:"( '=' @@ )? ';'?"`
}

// ConstStmt represents: const name = value; or const name: Type = value;
type ConstStmt struct {
	Pos lexer.Position

	Name  string      `parser:"'const' @Ident"`
	Type  *TypeRef    `parser:"( ':' @@ )?"`
	Value *Expression `parser:"'=' @@ ';'?"`
}

// ReturnStmt represents: return expr;
type ReturnStmt struct {
	Pos lexer.Position

	Value *Expression `parser:"'return' @@? ';'?"`
}

// IfStmt represents: if cond { } else if { } else { }
type IfStmt struct {
	Pos lexer.Position

	Condition *Expression `parser:"'if' @@"`
	Then      *StmtBlock  `parser:"@@"`
	ElseIfs   []*ElseIf   `parser:"@@*"`
	Else      *StmtBlock  `parser:"( 'else' @@ )?"`
}

// ElseIf represents: else if cond { }
type ElseIf struct {
	Pos lexer.Position

	Condition *Expression `parser:"'else' 'if' @@"`
	Then      *StmtBlock  `parser:"@@"`
}

// WhileStmt represents: while cond { }
type WhileStmt struct {
	Pos lexer.Position

	Condition *Expression `parser:"'while' @@"`
	Body      *StmtBlock  `parser:"@@"`
}

// ForStmt represents: for i in start..end { } or for item in collection { }
type ForStmt struct {
	Pos lexer.Position

	Variable string      `parser:"'for' @Ident"`
	Start    *Expression `parser:"'in' @@"`
	End      *Expression `parser:"( DotDot @@ )?"`
	Body     *StmtBlock  `parser:"@@"`
}

// LoopStmt represents: loop { } or loop array into/ref { }
type LoopStmt struct {
	Pos lexer.Position

	Array *Expression `parser:"'loop' ( @@"`
	Into  *string     `parser:"  ( 'into' @Ident"`
	Ref   *string     `parser:"  | 'ref' @Ident ) )?"`
	Body  *StmtBlock  `parser:"@@"`
}

// BreakStmt represents: break;
type BreakStmt struct {
	Pos lexer.Position

	Break bool `parser:"@'break' ';'?"`
}

// ContinueStmt represents: continue;
type ContinueStmt struct {
	Pos lexer.Position

	Continue bool `parser:"@'continue' ';'?"`
}

// AssignStmt represents: lvalue = expr; or lvalue op= expr;
type AssignStmt struct {
	Pos lexer.Position

	Target *Expression `parser:"@@"`
	Op     string      `parser:"@( PlusEq | MinusEq | StarEq | SlashEq | PercentEq | AmpEq | PipeEq | CaretEq | LtLtEq | GtGtEq | Eq )"`
	Value  *Expression `parser:"@@ ';'?"`
}

// ExprStmt represents a bare expression statement
type ExprStmt struct {
	Pos lexer.Position

	Expr *Expression `parser:"@@ ';'?"`
}
