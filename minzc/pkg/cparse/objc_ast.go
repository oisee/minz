// ObjC AST extensions for MinZ cparse fork.
//
// On Z80 there is no ObjC runtime — all dispatch is static:
//   @interface Foo       → struct Foo { fields }
//   -(int)add:(int)x     → int Foo_add(Foo* self, int x)
//   [foo add:5]          → Foo_add(foo, 5)
//   @property int x      → field x + getter/setter
package cparse

import "modernc.org/token"

// ── ObjC Interface (@interface ... @end) ─────────────────────────────────

// ObjCInterfaceDecl represents @interface ClassName : SuperClass { ivars } methods @end
type ObjCInterfaceDecl struct {
	Token     Token  // @interface
	ClassName Token  // IDENTIFIER
	SuperName *Token // IDENTIFIER after ':', or nil

	Ivars   []*ObjCIvar       // instance variables (between { })
	Methods []*ObjCMethodDecl // method declarations

	EndToken Token // @end
}

func (n *ObjCInterfaceDecl) Position() token.Position {
	return n.Token.Position()
}

// ObjCIvar is an instance variable inside @interface { ... }.
type ObjCIvar struct {
	TypeTokens []Token // type specifier tokens
	Name       Token   // IDENTIFIER
	Semicolon  Token   // ';'
}

// ── ObjC Implementation (@implementation ... @end) ───────────────────────

// ObjCImplementationDecl represents @implementation ClassName methods @end
type ObjCImplementationDecl struct {
	Token     Token // @implementation
	ClassName Token // IDENTIFIER

	Methods []*ObjCMethodDef // method definitions (with body)

	EndToken Token // @end
}

func (n *ObjCImplementationDecl) Position() token.Position {
	return n.Token.Position()
}

// ── ObjC Method Declaration / Definition ─────────────────────────────────

// ObjCMethodKind distinguishes instance (-) from class (+) methods.
type ObjCMethodKind int

const (
	ObjCInstanceMethod ObjCMethodKind = iota // '-'
	ObjCClassMethod                          // '+'
)

// ObjCMethodDecl is a method declaration (in @interface).
//
//	-(ReturnType)name;
//	-(ReturnType)name:(ParamType)param label:(ParamType)param2;
type ObjCMethodDecl struct {
	Kind       ObjCMethodKind
	ReturnType []Token             // tokens between ( )
	Selector   []*ObjCSelectorPart // name + params
	Semicolon  Token
}

func (n *ObjCMethodDecl) Position() token.Position {
	if len(n.Selector) > 0 {
		return n.Selector[0].Name.Position()
	}
	return token.Position{}
}

// MethodName returns the full ObjC selector string, e.g. "add:with:"
func (n *ObjCMethodDecl) MethodName() string {
	return selectorName(n.Selector)
}

// ObjCMethodDef is a method definition (in @implementation) — has a body.
type ObjCMethodDef struct {
	Kind       ObjCMethodKind
	ReturnType []Token
	Selector   []*ObjCSelectorPart
	Body       *CompoundStatement // { ... }
}

func (n *ObjCMethodDef) Position() token.Position {
	if len(n.Selector) > 0 {
		return n.Selector[0].Name.Position()
	}
	return token.Position{}
}

// MethodName returns the full selector, e.g. "initWith:and:"
func (n *ObjCMethodDef) MethodName() string {
	return selectorName(n.Selector)
}

// ObjCSelectorPart is one component of a selector.
//
//	name           → unary: {Name: "name"}
//	name:(int)x    → keyword: {Name: "name", Colon, ParamType, ParamName}
type ObjCSelectorPart struct {
	Name      Token   // selector keyword
	Colon     *Token  // ':' if keyword selector
	ParamType []Token // type in ( ), if keyword selector
	ParamName *Token  // parameter name, if keyword selector
}

// ── ObjC Message Expression ──────────────────────────────────────────────

// ObjCMessageExpr represents [receiver message:arg1 with:arg2]
type ObjCMessageExpr struct {
	LBracket Token          // '['
	Receiver ExpressionNode // can be identifier, expression, or class name
	Args     []*ObjCMsgArg  // selector parts with arguments
	RBracket Token          // ']'
}

func (n *ObjCMessageExpr) Position() token.Position {
	return n.LBracket.Position()
}

// ObjCMsgArg is one keyword:argument pair in a message send.
type ObjCMsgArg struct {
	Keyword Token          // selector keyword (e.g. "add", "with")
	Colon   *Token         // ':'
	Value   ExpressionNode // argument expression
}

// ── ObjC Property ────────────────────────────────────────────────────────

// ObjCPropertyDecl represents @property (attrs) Type name;
type ObjCPropertyDecl struct {
	Token     Token   // @property
	Attrs     []Token // attribute tokens inside ( ), if any
	TypeTokens []Token
	Name      Token
	Semicolon Token
}

// ── ObjC Protocol (@protocol ... @end) ───────────────────────────────────

// ObjCProtocolDecl represents @protocol Name methods @end
type ObjCProtocolDecl struct {
	Token     Token // @protocol
	Name      Token // IDENTIFIER
	Methods   []*ObjCMethodDecl
	EndToken  Token // @end
}

func (n *ObjCProtocolDecl) Position() token.Position {
	return n.Token.Position()
}

// ── Helpers ──────────────────────────────────────────────────────────────

func selectorName(parts []*ObjCSelectorPart) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 && parts[0].Colon == nil {
		return parts[0].Name.SrcStr()
	}
	var s string
	for _, p := range parts {
		s += p.Name.SrcStr() + ":"
	}
	return s
}

// ── ExternalDeclaration extensions ───────────────────────────────────────

// New ExternalDeclaration cases for ObjC constructs.
const (
	ExternalDeclarationObjCInterface      ExternalDeclarationCase = 100
	ExternalDeclarationObjCImplementation ExternalDeclarationCase = 101
	ExternalDeclarationObjCProtocol       ExternalDeclarationCase = 102
)

// ObjC fields on ExternalDeclaration — stored in the existing struct
// via the ObjCNode interface to avoid modifying the original AST struct.

// ObjCNode is any ObjC AST node.
type ObjCNode interface {
	Position() token.Position
}
