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

	TripleBracket *string        `parser:"  @TripleBracketBlock"`
	Module        *ModuleDecl    `parser:"| @@"`
	Import        *ImportDecl    `parser:"| @@"`
	Function      *FunctionDecl  `parser:"| @@"`
	Struct        *StructDecl    `parser:"| @@"`
	Enum          *EnumDecl      `parser:"| @@"`
	Interface     *InterfaceDecl `parser:"| @@"`
	Impl          *ImplDecl      `parser:"| @@"`
	Global        *GlobalDecl    `parser:"| @@"`
	Const         *ConstDecl     `parser:"| @@"`
	Type          *TypeDecl      `parser:"| @@"`
	Meta          *MetaDecl      `parser:"| @@"`
}

// ModuleDecl represents: module name;
type ModuleDecl struct {
	Pos lexer.Position

	Name string `parser:"'module' @Ident ';'?"`
}

// MetaDecl represents a top-level metafunction call: @define(...), @lua[[[...]]], etc.
type MetaDecl struct {
	Pos lexer.Position

	Name    string        `parser:"'@' @Ident"`
	Args    []*Expression `parser:"( '(' ( @@ ( ',' @@ )* )? ')' )?"`
	RawBody *string       `parser:"';'?"`
}

// ImportDecl represents: import path.to.module [as alias];
type ImportDecl struct {
	Pos lexer.Position

	Path  []string `parser:"'import' @Ident ( '.' @Ident )*"`
	Alias *string  `parser:"( 'as' @Ident )? ';'?"`
}

// FunctionDecl represents: [export] [pub] [extern/declare] fun name?<T>(params) -> Type ? ErrorType [at addr] [mode "m"] [abi "a"] { body }
type FunctionDecl struct {
	Pos lexer.Position

	Attributes []*Attribute  `parser:"( '@' '[' @@ ']' )*"`
	Export     bool          `parser:"@'export'?"`
	Public     bool          `parser:"@'pub'?"`
	Extern     bool          `parser:"@( 'extern' | 'declare' )?"`
	Keyword    string        `parser:"@'fun'"` // only 'fun' for declarations; 'fn' is for function types only
	Name       string        `parser:"@Ident"`
	CanError   bool          `parser:"@Question?"` // fun name?() - can return error
	TypeParams []*TypeParam  `parser:"( Lt @@ ( ',' @@ )* Gt )?"`
	Params     []*Param      `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	ReturnType *TypeRef      `parser:"( Arrow @@ )?"`
	ErrorType  *TypeRef      `parser:"( Bang @@ )?"` // -> Type ! ErrorType
	AtAddress  *Expression   `parser:"( 'at' @@ )?"`         // extern fun name() at 0x10;
	Mode       *string       `parser:"( 'mode' @String )?"`  // mode "adl" or mode "z80"
	Abi        *string       `parser:"( 'abi' @String )?"`   // abi "register" or abi "c"
	Body       *StmtBlock    `parser:"( @@ | ';' )"`
}

// TypeParam represents a generic type parameter: T or T: Constraint
type TypeParam struct {
	Pos lexer.Position

	Name       string   `parser:"@Ident"`
	Constraint *TypeRef `parser:"( ':' @@ )?"`
}

// Param represents a function parameter: name: Type [in Register] or self
type Param struct {
	Pos lexer.Position

	Self     bool     `parser:"( @'self'"`
	Ref      bool     `parser:"  @'&'? )"`
	Name     *string  `parser:"| @Ident"`
	Type     *TypeRef `parser:"  ':' @@"`
	Register *string  `parser:"( 'in' @Ident )?"` // Register mapping: "in HL", "in A", etc.
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

// InterfaceDecl represents: [pub] interface Name { method signatures }
type InterfaceDecl struct {
	Pos lexer.Position

	Public  bool              `parser:"@'pub'?"`
	Name    string            `parser:"'interface' @Ident"`
	Methods []*MethodSignature `parser:"'{' @@* '}'"`
}

// MethodSignature represents a method signature in an interface: fun name(params) -> Type;
type MethodSignature struct {
	Pos lexer.Position

	Name       string    `parser:"'fun' @Ident"`
	Params     []*Param  `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	ReturnType *TypeRef  `parser:"( Arrow @@ )? ';'?"`
}

// ImplDecl represents: impl [Trait for] Type { methods }
type ImplDecl struct {
	Pos lexer.Position

	Trait   *TypeRef        `parser:"'impl' ( @@ 'for' )?"`
	Type    *TypeRef        `parser:"@@"`
	Methods []*FunctionDecl `parser:"'{' @@* '}'"`
}

// GlobalDecl represents: [pub] global/let name: Type = value;
type GlobalDecl struct {
	Pos lexer.Position

	Public  bool        `parser:"@'pub'?"`
	Keyword string      `parser:"@( 'global' | 'let' )"`
	Name    string      `parser:"@Ident"`
	Type    *TypeRef    `parser:"':' @@"`
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
