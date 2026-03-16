// ObjC parser extensions for MinZ cparse fork.
//
// Adds parsing of:
//   @interface ClassName : Super { ivars } methods @end
//   @implementation ClassName methods @end
//   @protocol Name methods @end
//   [receiver message:arg1 with:arg2]   (message expression)
//
// All ObjC constructs lower to static C equivalents for Z80.
package cparse

// objcInterface parses: @interface Name [: Super] [{ ivars }] methods @end
func (p *parser) objcInterface() *ObjCInterfaceDecl {
	n := &ObjCInterfaceDecl{Token: p.shift(false)} // consume @interface

	// Class name.
	if p.rune(false) != rune(IDENTIFIER) {
		p.cpp.eh("%v: expected class name after @interface", p.toks[0].Position())
		return nil
	}
	n.ClassName = p.shift(false)

	// Optional: : SuperClass
	if p.rune(false) == ':' {
		p.shift(false) // consume ':'
		if p.rune(false) != rune(IDENTIFIER) {
			p.cpp.eh("%v: expected superclass name after ':'", p.toks[0].Position())
			return nil
		}
		t := p.shift(false)
		n.SuperName = &t
	}

	// Optional: <Protocol1, Protocol2, ...>
	if p.rune(false) == '<' {
		p.shift(false) // consume '<'
		for p.rune(false) != '>' && p.rune(false) != eof {
			if p.rune(false) == rune(IDENTIFIER) {
				n.Protocols = append(n.Protocols, p.shift(false))
			} else if p.rune(false) == ',' {
				p.shift(false) // skip comma
			} else {
				break
			}
		}
		if p.rune(false) == '>' {
			p.shift(false) // consume '>'
		}
	}

	// Optional: { ivars }
	if p.rune(false) == '{' {
		p.shift(false) // consume '{'
		for p.rune(false) != '}' && p.rune(false) != eof {
			ivar := p.objcIvar()
			if ivar != nil {
				n.Ivars = append(n.Ivars, ivar)
			}
		}
		p.must('}')
	}

	// Methods until @end.
	for p.rune(false) != rune(OBJC_END) && p.rune(false) != eof {
		if p.rune(false) == rune(OBJC_PROPERTY) {
			// Skip @property for now — consume until ';'
			p.shift(false)
			for p.rune(false) != ';' && p.rune(false) != eof {
				p.shift(false)
			}
			if p.rune(false) == ';' {
				p.shift(false)
			}
			continue
		}
		md := p.objcMethodDecl()
		if md != nil {
			n.Methods = append(n.Methods, md)
		}
	}

	if p.rune(false) == rune(OBJC_END) {
		n.EndToken = p.shift(false)
	}
	return n
}

// objcImplementation parses: @implementation Name methods @end
func (p *parser) objcImplementation() *ObjCImplementationDecl {
	n := &ObjCImplementationDecl{Token: p.shift(false)} // consume @implementation

	if p.rune(false) != rune(IDENTIFIER) {
		p.cpp.eh("%v: expected class name after @implementation", p.toks[0].Position())
		return nil
	}
	n.ClassName = p.shift(false)

	// Methods until @end.
	for p.rune(false) != rune(OBJC_END) && p.rune(false) != eof {
		md := p.objcMethodDef()
		if md != nil {
			n.Methods = append(n.Methods, md)
		}
	}

	if p.rune(false) == rune(OBJC_END) {
		n.EndToken = p.shift(false)
	}
	return n
}

// objcProtocol parses: @protocol Name methods @end
func (p *parser) objcProtocol() *ObjCProtocolDecl {
	n := &ObjCProtocolDecl{Token: p.shift(false)} // consume @protocol

	if p.rune(false) != rune(IDENTIFIER) {
		p.cpp.eh("%v: expected protocol name after @protocol", p.toks[0].Position())
		return nil
	}
	n.Name = p.shift(false)

	for p.rune(false) != rune(OBJC_END) && p.rune(false) != eof {
		if p.rune(false) == rune(OBJC_OPTIONAL) || p.rune(false) == rune(OBJC_REQUIRED) {
			p.shift(false) // skip @optional/@required markers
			continue
		}
		md := p.objcMethodDecl()
		if md != nil {
			n.Methods = append(n.Methods, md)
		}
	}

	if p.rune(false) == rune(OBJC_END) {
		n.EndToken = p.shift(false)
	}
	return n
}

// objcIvar parses: Type name;
func (p *parser) objcIvar() *ObjCIvar {
	ivar := &ObjCIvar{}
	// Collect type tokens until we hit identifier followed by ';'.
	for p.rune(false) != ';' && p.rune(false) != '}' && p.rune(false) != eof {
		t := p.shift(false)
		ivar.TypeTokens = append(ivar.TypeTokens, t)
	}
	// Last token of TypeTokens is the name.
	if len(ivar.TypeTokens) > 0 {
		ivar.Name = ivar.TypeTokens[len(ivar.TypeTokens)-1]
		ivar.TypeTokens = ivar.TypeTokens[:len(ivar.TypeTokens)-1]
	}
	if p.rune(false) == ';' {
		ivar.Semicolon = p.shift(false)
	}
	return ivar
}

// objcMethodDecl parses: -/+ (ReturnType) selector;
func (p *parser) objcMethodDecl() *ObjCMethodDecl {
	md := &ObjCMethodDecl{}

	switch p.rune(false) {
	case '-':
		md.Kind = ObjCInstanceMethod
		p.shift(false)
	case '+':
		md.Kind = ObjCClassMethod
		p.shift(false)
	default:
		// Skip unexpected token.
		p.shift(false)
		return nil
	}

	// Return type in ( )
	if p.rune(false) == '(' {
		p.shift(false) // '('
		for p.rune(false) != ')' && p.rune(false) != eof {
			md.ReturnType = append(md.ReturnType, p.shift(false))
		}
		if p.rune(false) == ')' {
			p.shift(false) // ')'
		}
	}

	// Selector parts.
	md.Selector = p.objcSelectorParts()

	if p.rune(false) == ';' {
		md.Semicolon = p.shift(false)
	}
	return md
}

// objcMethodDef parses: -/+ (ReturnType) selector { body }
func (p *parser) objcMethodDef() *ObjCMethodDef {
	md := &ObjCMethodDef{}

	switch p.rune(false) {
	case '-':
		md.Kind = ObjCInstanceMethod
		p.shift(false)
	case '+':
		md.Kind = ObjCClassMethod
		p.shift(false)
	default:
		p.shift(false)
		return nil
	}

	// Return type in ( )
	if p.rune(false) == '(' {
		p.shift(false) // '('
		for p.rune(false) != ')' && p.rune(false) != eof {
			md.ReturnType = append(md.ReturnType, p.shift(false))
		}
		if p.rune(false) == ')' {
			p.shift(false) // ')'
		}
	}

	// Selector parts.
	md.Selector = p.objcSelectorParts()

	// Body.
	if p.rune(false) == '{' {
		md.Body = p.compoundStatement(false, false, nil)
	}
	return md
}

// objcSelectorParts parses selector keywords and parameters.
//
//	doSomething                              → unary
//	add:(int)x                               → single keyword
//	add:(int)x with:(int)y                   → multi keyword
func (p *parser) objcSelectorParts() []*ObjCSelectorPart {
	var parts []*ObjCSelectorPart

	for p.rune(false) == rune(IDENTIFIER) {
		part := &ObjCSelectorPart{Name: p.shift(false)}

		if p.rune(false) == ':' {
			t := p.shift(false) // ':'
			part.Colon = &t

			// Parameter type in ( )
			if p.rune(false) == '(' {
				p.shift(false) // '('
				for p.rune(false) != ')' && p.rune(false) != eof {
					part.ParamType = append(part.ParamType, p.shift(false))
				}
				if p.rune(false) == ')' {
					p.shift(false) // ')'
				}
			}

			// Parameter name.
			if p.rune(false) == rune(IDENTIFIER) {
				t := p.shift(false)
				part.ParamName = &t
			}
		}

		parts = append(parts, part)

		// If no colon, this is a unary selector — done.
		if part.Colon == nil {
			break
		}
	}

	return parts
}

// objcMessageExpr parses: [receiver selector:arg1 keyword:arg2]
// Called when '[' is seen in expression context.
func (p *parser) objcMessageExpr() *ObjCMessageExpr {
	msg := &ObjCMessageExpr{LBracket: p.shift(false)} // consume '['

	// Receiver: can be identifier, self, super, or nested expression.
	msg.Receiver = p.assignmentExpression(false)

	// Message args: keyword:value pairs until ']'.
	for p.rune(false) != ']' && p.rune(false) != eof {
		arg := &ObjCMsgArg{}
		if p.rune(false) == rune(IDENTIFIER) {
			arg.Keyword = p.shift(false)
		}
		if p.rune(false) == ':' {
			t := p.shift(false)
			arg.Colon = &t
			arg.Value = p.assignmentExpression(false)
		}
		msg.Args = append(msg.Args, arg)

		// Safety: if we didn't advance, break to avoid infinite loop.
		if arg.Keyword.Ch == 0 && arg.Colon == nil {
			break
		}
	}

	if p.rune(false) == ']' {
		msg.RBracket = p.shift(false)
	}

	return msg
}
