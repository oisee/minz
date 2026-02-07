package participle

import (
	"github.com/alecthomas/participle/v2/lexer"
)

// File represents a complete MinZ source file
type File struct {
	Pos lexer.Position

	Declarations []*Decl `parser:"@@*"`
}

// Decl represents any top-level declaration
type Decl struct {
	Pos lexer.Position

	Import   *ImportDecl   `parser:"  @@"`
	Function *FunctionDecl `parser:"| @@"`
	Struct   *StructDecl   `parser:"| @@"`
	Enum     *EnumDecl     `parser:"| @@"`
	Impl     *ImplDecl     `parser:"| @@"`
	Global   *GlobalDecl   `parser:"| @@"`
	Const    *ConstDecl    `parser:"| @@"`
	Type     *TypeDecl     `parser:"| @@"`
}

// ImportDecl represents: import path.to.module [as alias];
type ImportDecl struct {
	Pos lexer.Position

	Path  []string `parser:"'import' @Ident ( '.' @Ident )*"`
	Alias *string  `parser:"( 'as' @Ident )? ';'?"`
}

// FunctionDecl represents: [pub] fun/fn name<T>(params) -> Type { body }
type FunctionDecl struct {
	Pos lexer.Position

	Attributes []*Attribute  `parser:"( '@' '[' @@ ']' )*"`
	Public     bool          `parser:"@'pub'?"`
	Keyword    string        `parser:"@( 'fun' | 'fn' )"`
	Name       string        `parser:"@Ident"`
	TypeParams []*TypeParam  `parser:"( Lt @@ ( ',' @@ )* Gt )?"`
	Params     []*Param      `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	ReturnType *TypeRef      `parser:"( Arrow @@ )?"`
	Body       *StmtBlock    `parser:"@@"`
}

// TypeParam represents a generic type parameter: T or T: Constraint
type TypeParam struct {
	Pos lexer.Position

	Name       string   `parser:"@Ident"`
	Constraint *TypeRef `parser:"( ':' @@ )?"`
}

// Param represents a function parameter: name: Type
type Param struct {
	Pos lexer.Position

	Name string   `parser:"@Ident"`
	Type *TypeRef `parser:"':' @@"`
}

// StructDecl represents: [pub] struct Name<T> { fields }
type StructDecl struct {
	Pos lexer.Position

	Attributes []*Attribute `parser:"( '@' '[' @@ ']' )*"`
	Public     bool         `parser:"@'pub'?"`
	Name       string       `parser:"'struct' @Ident"`
	TypeParams []*TypeParam `parser:"( Lt @@ ( ',' @@ )* Gt )?"`
	Fields     []*Field     `parser:"'{' ( @@ ( ',' @@ )* ','? )? '}'"`
}

// Field represents a struct field: name: Type
type Field struct {
	Pos lexer.Position

	Public bool     `parser:"@'pub'?"`
	Name   string   `parser:"@Ident"`
	Type   *TypeRef `parser:"':' @@"`
}

// EnumDecl represents: [pub] enum Name { VARIANT1, VARIANT2(Type), ... }
type EnumDecl struct {
	Pos lexer.Position

	Attributes []*Attribute  `parser:"( '@' '[' @@ ']' )*"`
	Public     bool          `parser:"@'pub'?"`
	Name       string        `parser:"'enum' @Ident"`
	Variants   []*EnumVariant `parser:"'{' ( @@ ( ',' @@ )* ','? )? '}'"`
}

// EnumVariant represents an enum variant: NAME or NAME(Type) or NAME = value
type EnumVariant struct {
	Pos lexer.Position

	Name   string      `parser:"@Ident"`
	Type   *TypeRef    `parser:"( '(' @@ ')' )?"`
	Value  *Expression `parser:"( '=' @@ )?"`
}

// ImplDecl represents: impl [Trait for] Type { methods }
type ImplDecl struct {
	Pos lexer.Position

	Trait   *TypeRef        `parser:"'impl' ( @@ 'for' )?"`
	Type    *TypeRef        `parser:"@@"`
	Methods []*FunctionDecl `parser:"'{' @@* '}'"`
}

// GlobalDecl represents: [pub] global name: Type = value;
type GlobalDecl struct {
	Pos lexer.Position

	Public bool        `parser:"@'pub'?"`
	Name   string      `parser:"'global' @Ident"`
	Type   *TypeRef    `parser:"':' @@"`
	Value  *Expression `parser:"( '=' @@ )? ';'?"`
}

// ConstDecl represents a top-level constant: [pub] const NAME: Type = value;
type ConstDecl struct {
	Pos lexer.Position

	Public bool        `parser:"@'pub'?"`
	Name   string      `parser:"'const' @Ident"`
	Type   *TypeRef    `parser:"( ':' @@ )?"`
	Value  *Expression `parser:"'=' @@ ';'?"`
}

// TypeDecl represents a type alias: type NewName = ExistingType;
type TypeDecl struct {
	Pos lexer.Position

	Public bool     `parser:"@'pub'?"`
	Name   string   `parser:"'type' @Ident"`
	Type   *TypeRef `parser:"'=' @@ ';'?"`
}

// Attribute represents an attribute like [inline] or [abi("z80")]
type Attribute struct {
	Pos lexer.Position

	Name string        `parser:"@Ident"`
	Args []*Expression `parser:"( '(' ( @@ ( ',' @@ )* )? ')' )?"`
}
