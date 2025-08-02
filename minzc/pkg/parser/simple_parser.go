package parser

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	
	"github.com/minz/minzc/pkg/ast"
)

// SimpleParser is a basic recursive descent parser for MinZ
// This is a temporary solution until tree-sitter integration is fixed
type SimpleParser struct {
	scanner *bufio.Scanner
	current string
	line    int
	col     int
	tokens  []Token
	pos     int
}

// Token represents a lexical token
type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

// TokenType represents the type of a token
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenIdent
	TokenNumber
	TokenString
	TokenKeyword
	TokenOperator
	TokenPunc
	TokenComment
)

// NewSimpleParser creates a new simple parser
func NewSimpleParser() *SimpleParser {
	return &SimpleParser{}
}

// ParseFile parses a MinZ source file
func (p *SimpleParser) ParseFile(filename string) (*ast.File, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	// Tokenize the file
	if err := p.tokenize(file); err != nil {
		return nil, fmt.Errorf("tokenization error: %w", err)
	}
	
	// Parse tokens into AST
	return p.parseSourceFile(filename)
}

// tokenize converts the input into tokens
func (p *SimpleParser) tokenize(file *os.File) error {
	scanner := bufio.NewScanner(file)
	p.line = 1
	
	for scanner.Scan() {
		line := scanner.Text()
		p.tokenizeLine(line)
		p.line++
	}
	
	// Add EOF token
	p.tokens = append(p.tokens, Token{Type: TokenEOF, Line: p.line, Column: 0})
	
	return scanner.Err()
}

// tokenizeLine tokenizes a single line
func (p *SimpleParser) tokenizeLine(line string) {
	p.col = 1
	i := 0
	
	for i < len(line) {
		// Skip whitespace
		if isSpace(line[i]) {
			i++
			p.col++
			continue
		}
		
		// Comments
		if i+1 < len(line) && line[i:i+2] == "//" {
			// Rest of line is comment
			break
		}
		
		start := i
		startCol := p.col
		
		// Numbers
		if isDigit(line[i]) {
			for i < len(line) && (isDigit(line[i]) || line[i] == 'x' || isHexDigit(line[i])) {
				i++
				p.col++
			}
			p.tokens = append(p.tokens, Token{
				Type:   TokenNumber,
				Value:  line[start:i],
				Line:   p.line,
				Column: startCol,
			})
			continue
		}
		
		// Identifiers and keywords (including @lua)
		if isAlpha(line[i]) || line[i] == '_' || line[i] == '@' {
			// Special handling for @ tokens
			if line[i] == '@' {
				i++
				p.col++
			}
			for i < len(line) && (isAlnum(line[i]) || line[i] == '_') {
				i++
				p.col++
			}
			value := line[start:i]
			tokenType := TokenIdent
			if isKeyword(value) {
				tokenType = TokenKeyword
			}
			tok := Token{
				Type:   tokenType,
				Value:  value,
				Line:   p.line,
				Column: startCol,
			}
			p.tokens = append(p.tokens, tok)
			continue
		}
		
		// Strings
		if line[i] == '"' {
			i++ // skip opening quote
			p.col++
			strStart := i
			for i < len(line) && line[i] != '"' {
				if line[i] == '\\' && i+1 < len(line) {
					i++ // skip escaped char
					p.col++
				}
				i++
				p.col++
			}
			if i < len(line) {
				p.tokens = append(p.tokens, Token{
					Type:   TokenString,
					Value:  line[strStart:i],
					Line:   p.line,
					Column: startCol,
				})
				i++ // skip closing quote
				p.col++
			}
			continue
		}
		
		// Multi-char operators
		if i+1 < len(line) {
			twoChar := line[i:i+2]
			if isOperator(twoChar) {
				p.tokens = append(p.tokens, Token{
					Type:   TokenOperator,
					Value:  twoChar,
					Line:   p.line,
					Column: startCol,
				})
				i += 2
				p.col += 2
				continue
			}
		}
		
		// Single char operators and punctuation
		ch := string(line[i])
		if ch == "=" {
			// Special handling for = to ensure it's tokenized as operator
			p.tokens = append(p.tokens, Token{
				Type:   TokenOperator,
				Value:  ch,
				Line:   p.line,
				Column: startCol,
			})
		} else if isOperator(ch) {
			p.tokens = append(p.tokens, Token{
				Type:   TokenOperator,
				Value:  ch,
				Line:   p.line,
				Column: startCol,
			})
		} else if isPunctuation(ch) {
			p.tokens = append(p.tokens, Token{
				Type:   TokenPunc,
				Value:  ch,
				Line:   p.line,
				Column: startCol,
			})
		}
		
		i++
		p.col++
	}
}

// Parser methods

// parseSourceFile parses the entire source file
func (p *SimpleParser) parseSourceFile(filename string) (*ast.File, error) {
	file := &ast.File{
		Name:         filename,
		Imports:      []*ast.ImportStmt{},
		Declarations: []ast.Declaration{},
		StartPos:     ast.Position{Line: 1, Column: 1},
	}
	
	p.pos = 0
	
	// Parse module declaration if present
	if p.peek().Type == TokenKeyword && p.peek().Value == "module" {
		p.advance() // consume 'module'
		if p.peek().Type == TokenIdent {
			file.ModuleName = p.advance().Value
		}
		p.expect(TokenPunc, ";")
	}
	
	// Parse imports and declarations
	for !p.isAtEnd() {
		if p.peek().Type == TokenKeyword {
			switch p.peek().Value {
			case "import":
				imp := p.parseImport()
				if imp != nil {
					file.Imports = append(file.Imports, imp)
				}
			case "fun", "pub":
				decl := p.parseFunctionDecl()
				if decl != nil {
					file.Declarations = append(file.Declarations, decl)
				}
			case "struct":
				decl := p.parseStructDecl()
				if decl != nil {
					file.Declarations = append(file.Declarations, decl)
				}
			case "let":
				decl := p.parseVarDecl()
				if decl != nil {
					file.Declarations = append(file.Declarations, decl)
				}
			case "const":
				decl := p.parseConstDecl()
				if decl != nil {
					file.Declarations = append(file.Declarations, decl)
				}
			case "global":
				decl := p.parseGlobalDecl()
				if decl != nil {
					file.Declarations = append(file.Declarations, decl)
				}
			case "type":
				decl := p.parseTypeDecl()
				if decl != nil {
					file.Declarations = append(file.Declarations, decl)
				}
			default:
				p.advance() // skip unknown keyword
			}
		} else if p.peek().Type == TokenIdent && p.peek().Value == "@lua" {
			// Check if it's a @lua[[[ block
			if p.peekAhead(1).Value == "[" && p.peekAhead(2).Value == "[" && p.peekAhead(3).Value == "[" {
				block := p.parseLuaBlock()
				if block != nil {
					// LuaBlock implements both Statement and Declaration interfaces
					file.Declarations = append(file.Declarations, block)
				}
			} else {
				p.advance() // skip @lua without block
			}
		} else {
			p.advance() // skip unknown token
		}
	}
	
	file.EndPos = ast.Position{Line: p.line, Column: p.col}
	return file, nil
}

// parseImport parses an import statement
func (p *SimpleParser) parseImport() *ast.ImportStmt {
	p.expect(TokenKeyword, "import")
	
	imp := &ast.ImportStmt{
		StartPos: p.currentPos(),
	}
	
	// Parse import path (handle dotted paths like zx.screen)
	if p.peek().Type == TokenIdent {
		path := p.advance().Value
		// Check for dotted path
		for p.peek().Value == "." {
			p.advance() // consume '.'
			if p.peek().Type == TokenIdent {
				path += "." + p.advance().Value
			}
		}
		imp.Path = path
	}
	
	// Optional alias
	if p.peek().Type == TokenKeyword && p.peek().Value == "as" {
		p.advance() // consume 'as'
		if p.peek().Type == TokenIdent {
			imp.Alias = p.advance().Value
		}
	}
	
	p.expect(TokenPunc, ";")
	imp.EndPos = p.currentPos()
	
	return imp
}

// parseFunctionDecl parses a function declaration
func (p *SimpleParser) parseFunctionDecl() *ast.FunctionDecl {
	fn := &ast.FunctionDecl{
		StartPos: p.currentPos(),
		Params:   []*ast.Parameter{},
	}
	
	// Check for pub
	if p.peek().Value == "pub" {
		fn.IsPublic = true
		p.advance()
	}
	
	// Accept only "fun"
	if p.peek().Value != "fun" {
		// Error: expected 'fun'
		return nil
	}
	p.advance()
	
	// Function name
	if p.peek().Type == TokenIdent {
		fn.Name = p.advance().Value
	}
	
	// Parameters
	p.expect(TokenPunc, "(")
	for p.peek().Value != ")" && !p.isAtEnd() {
		param := p.parseParameter()
		if param != nil {
			fn.Params = append(fn.Params, param)
		}
		
		if p.peek().Value == "," {
			p.advance()
		}
	}
	p.expect(TokenPunc, ")")
	
	// Return type
	if p.peek().Value == "->" {
		p.advance()
		fn.ReturnType = p.parseType()
	} else {
		// Default to void
		fn.ReturnType = &ast.PrimitiveType{Name: "void"}
	}
	
	// Function body
	fn.Body = p.parseBlock()
	fn.EndPos = p.currentPos()
	
	return fn
}

// parseParameter parses a function parameter
func (p *SimpleParser) parseParameter() *ast.Parameter {
	param := &ast.Parameter{
		StartPos: p.currentPos(),
	}
	
	// Parameter name
	if p.peek().Type == TokenIdent {
		param.Name = p.advance().Value
	}
	
	// Type
	p.expect(TokenPunc, ":")
	param.Type = p.parseType()
	
	param.EndPos = p.currentPos()
	return param
}

// parseType parses a type
func (p *SimpleParser) parseType() ast.Type {
	startPos := p.currentPos()
	
	// Handle nil or EOF
	if p.isAtEnd() {
		return nil
	}
	
	// Pointer types
	if p.peek().Value == "*" {
		p.advance()
		isMut := false
		
		// Check for const or mut after *
		if p.peek().Type == TokenKeyword {
			if p.peek().Value == "mut" {
				isMut = true
				p.advance()
			} else if p.peek().Value == "const" {
				// const pointers are immutable by default
				isMut = false
				p.advance()
			}
		}
		
		return &ast.PointerType{
			BaseType:  p.parseType(),
			IsMutable: isMut,
			StartPos:  startPos,
			EndPos:    p.currentPos(),
		}
	}
	
	// Array types
	if p.peek().Value == "[" {
		p.advance()
		
		// Check if this is new syntax [Type; size] or old syntax [size]Type
		// We'll look ahead to see if we have a type name followed by semicolon
		nextTok := p.peek()
		if nextTok.Type == TokenIdent {
			// Save position to potentially backtrack
			savedPos := p.pos
			
			// Look ahead to see if we have type followed by semicolon
			// Skip the type name
			p.advance()
			
			// Check if followed by semicolon
			isNewSyntax := p.peek().Value == ";"
			
			// Restore position
			p.pos = savedPos
			
			if isNewSyntax {
				// New syntax: [Type; size]
				elemType := p.parseType()
				p.expect(TokenPunc, ";")
				
				var size ast.Expression
				if p.peek().Type == TokenNumber {
					val, _ := strconv.ParseInt(p.peek().Value, 0, 64)
					size = &ast.NumberLiteral{
						Value:    val,
						StartPos: p.currentPos(),
						EndPos:   p.currentPos(),
					}
					p.advance()
				}
				
				p.expect(TokenPunc, "]")
				
				return &ast.ArrayType{
					ElementType: elemType,
					Size:        size,
					StartPos:    startPos,
					EndPos:      p.currentPos(),
				}
			}
		}
		
		// Old syntax: [size]Type
		var size ast.Expression
		if p.peek().Type == TokenNumber {
			val, _ := strconv.ParseInt(p.peek().Value, 0, 64)
			size = &ast.NumberLiteral{
				Value:    val,
				StartPos: p.currentPos(),
				EndPos:   p.currentPos(),
			}
			p.advance()
		}
		
		p.expect(TokenPunc, "]")
		
		// Then parse element type
		elemType := p.parseType()
		
		return &ast.ArrayType{
			ElementType: elemType,
			Size:        size,
			StartPos:    startPos,
			EndPos:      p.currentPos(),
		}
	}
	
	// Primitive types and type identifiers
	if p.peek().Type == TokenIdent {
		name := p.advance().Value
		if isPrimitiveType(name) {
			return &ast.PrimitiveType{
				Name:     name,
				StartPos: startPos,
				EndPos:   p.currentPos(),
			}
		}
		return &ast.TypeIdentifier{
			Name:     name,
			StartPos: startPos,
			EndPos:   p.currentPos(),
		}
	}
	
	return nil
}

// parseBlock parses a block statement
func (p *SimpleParser) parseBlock() *ast.BlockStmt {
	block := &ast.BlockStmt{
		StartPos:   p.currentPos(),
		Statements: []ast.Statement{},
	}
	
	p.expect(TokenPunc, "{")
	
	for p.peek().Value != "}" && !p.isAtEnd() {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
	}
	
	p.expect(TokenPunc, "}")
	block.EndPos = p.currentPos()
	
	return block
}

// parseStatement parses a statement
func (p *SimpleParser) parseStatement() ast.Statement {
	if p.peek().Type == TokenKeyword {
		switch p.peek().Value {
		case "return":
			return p.parseReturnStmt()
		case "if":
			return p.parseIfStmt()
		case "while":
			return p.parseWhileStmt()
		case "let":
			return p.parseVarDecl()
		case "const":
			return p.parseConstDecl()
		case "asm":
			return p.parseAsmStmt()
		case "loop":
			return p.parseLoopStatement()
		case "do":
			return p.parseDoStatement()
		}
	}
	
	// Check for GCC-style inline assembly first (asm with parentheses)
	if p.peek().Type == TokenKeyword && p.peek().Value == "asm" {
		// Look ahead to see if it's GCC-style (asm with parentheses) or block style (asm with braces)
		nextIdx := p.pos + 1
		if nextIdx < len(p.tokens) && p.tokens[nextIdx].Value == "(" {
			// This is GCC-style inline assembly - parse as expression
			expr := p.parseExpression()
			if expr != nil {
				p.expect(TokenPunc, ";")
				return &ast.ExpressionStmt{
					Expression: expr,
					StartPos:   expr.Pos(),
					EndPos:     p.currentPos(),
				}
			}
		}
		// Otherwise fall through to block-style asm statement parsing
	}
	
	// Expression statement or assignment
	expr := p.parseExpression()
	if expr != nil {
		// Check for assignment
		if p.peek().Value == "=" {
			p.advance() // skip =
			rhs := p.parseExpression()
			p.expect(TokenPunc, ";")
			
			// Create an assignment statement
			return &ast.AssignStmt{
				Target: expr,
				Value:  rhs,
				StartPos: expr.Pos(),
				EndPos:   p.currentPos(),
			}
		}
		p.expect(TokenPunc, ";")
		// For now, return the expression as a statement
		// We should create an ExpressionStatement type
		return &ast.ExpressionStmt{
			Expression: expr,
			StartPos:   expr.Pos(),
			EndPos:     p.currentPos(),
		}
	}
	
	// Skip unknown statements
	for !p.isAtEnd() && p.peek().Value != ";" && p.peek().Value != "}" {
		p.advance()
	}
	if p.peek().Value == ";" {
		p.advance()
	}
	return nil
}

// parseReturnStmt parses a return statement
func (p *SimpleParser) parseReturnStmt() *ast.ReturnStmt {
	ret := &ast.ReturnStmt{
		StartPos: p.currentPos(),
	}
	
	p.expect(TokenKeyword, "return")
	
	// Optional return value
	if p.peek().Value != ";" {
		ret.Value = p.parseExpression()
	}
	
	p.expect(TokenPunc, ";")
	ret.EndPos = p.currentPos()
	
	return ret
}

// parseExpression parses an expression (simplified)
func (p *SimpleParser) parseExpression() ast.Expression {
	return p.parseBinaryExpression(0)
}

// parseBinaryExpression parses binary expressions with precedence
func (p *SimpleParser) parseBinaryExpression(minPrec int) ast.Expression {
	left := p.parsePrimaryExpression()
	
	for {
		tok := p.peek()
		// Don't parse = as a binary operator
		if tok.Value == "=" {
			break
		}
		
		// Handle "as" for type casts
		if tok.Type == TokenKeyword && tok.Value == "as" {
			// "as" has low precedence (lower than arithmetic but higher than assignment)
			if 1 < minPrec {
				break
			}
			
			startPos := p.currentPos()
			p.advance() // consume "as"
			targetType := p.parseType()
			
			left = &ast.CastExpr{
				Expr:       left,
				TargetType: targetType,
				StartPos:   startPos,
				EndPos:     p.currentPos(),
			}
			continue
		}
		
		if tok.Type != TokenOperator || !isOperator(tok.Value) {
			break
		}
		
		prec := operatorPrecedence(tok.Value)
		if prec < minPrec {
			break
		}
		
		op := p.advance().Value
		right := p.parseBinaryExpression(prec + 1)
		
		left = &ast.BinaryExpr{
			Left:     left,
			Right:    right,
			Operator: op,
			StartPos: p.currentPos(), // For simplicity, use current position
			EndPos:   p.currentPos(),
		}
	}
	
	return left
}

// parsePrimaryExpression parses primary expressions
func (p *SimpleParser) parsePrimaryExpression() ast.Expression {
	// Unary operators: !, ~, -, +, &
	if p.peek().Type == TokenOperator {
		op := p.peek().Value
		if op == "!" || op == "~" || op == "-" || op == "+" || op == "&" {
			startPos := p.currentPos()
			p.advance() // consume operator
			
			operand := p.parsePrimaryExpression()
			if operand == nil {
				return nil
			}
			
			return &ast.UnaryExpr{
				Operator: op,
				Operand:  operand,
				StartPos: startPos,
				EndPos:   p.currentPos(),
			}
		}
	}
	
	// Number literals
	if p.peek().Type == TokenNumber {
		val, _ := strconv.ParseInt(p.peek().Value, 0, 64)
		expr := &ast.NumberLiteral{
			Value:    val,
			StartPos: p.currentPos(),
			EndPos:   p.currentPos(),
		}
		p.advance()
		return expr
	}
	
	// Boolean literals
	if p.peek().Type == TokenKeyword && (p.peek().Value == "true" || p.peek().Value == "false") {
		val := p.peek().Value == "true"
		expr := &ast.BooleanLiteral{
			Value:    val,
			StartPos: p.currentPos(),
			EndPos:   p.currentPos(),
		}
		p.advance()
		return expr
	}
	
	// String literals
	if p.peek().Type == TokenString {
		value := p.peek().Value
		expr := &ast.StringLiteral{
			Value:    value,
			StartPos: p.currentPos(),
			EndPos:   p.currentPos(),
		}
		p.advance()
		return expr
	}
	
	// Identifiers and function calls
	if p.peek().Type == TokenIdent {
		name := p.peek().Value
		startPos := p.currentPos()
		p.advance()
		
		// Check for function call
		if p.peek().Value == "(" {
			return p.parseFunctionCall(name, startPos)
		}
		
		// Just an identifier (will be handled by parsePostfixExpression)
		expr := &ast.Identifier{
			Name:     name,
			StartPos: startPos,
			EndPos:   p.currentPos(),
		}
		return p.parsePostfixExpression(expr)
	}
	
	// Parenthesized expressions
	if p.peek().Value == "(" {
		p.advance() // consume '('
		expr := p.parseExpression()
		p.expect(TokenPunc, ")")
		return expr
	}
	
	// Inline assembly expression (GCC-style)
	if p.peek().Type == TokenKeyword && p.peek().Value == "asm" {
		return p.parseInlineAsmExpr()
	}
	
	// Lua metaprogramming expression
	if p.peek().Type == TokenIdent && p.peek().Value == "@lua" {
		return p.parseLuaExpression()
	}
	
	// @print expression (legacy support)
	if p.peek().Type == TokenIdent && p.peek().Value == "@print" {
		return p.parsePrintExpression()
	}
	
	// General @metafunction(...) expressions
	if p.peek().Type == TokenIdent && strings.HasPrefix(p.peek().Value, "@") && len(p.peek().Value) > 1 {
		return p.parseMetafunctionCall()
	}
	
	// Lambda expressions
	if p.peek().Value == "|" {
		return p.parseLambdaExpression()
	}
	
	// Debug for testing
	if p.pos < len(p.tokens) {
		tok := p.peek()
		if strings.HasPrefix(tok.Value, "@") {
			fmt.Printf("DEBUG parsePrimary: Token type=%v value='%s' (looking for '@lua')\n", tok.Type, tok.Value)
		}
	}
	
	return nil
}

// parseInlineAsmExpr parses GCC-style inline assembly expressions
func (p *SimpleParser) parseInlineAsmExpr() ast.Expression {
	startPos := p.currentPos()
	p.expect(TokenKeyword, "asm")
	p.expect(TokenPunc, "(")
	
	// Parse the assembly code string
	if p.peek().Type != TokenString {
		// Instead of panicking, return nil to let caller handle the error gracefully
		return nil
	}
	code := p.peek().Value
	p.advance()
	
	// For now, skip the constraint parsing (: : :)
	// Just consume tokens until closing paren
	colonCount := 0
	for !p.isAtEnd() && p.peek().Value != ")" {
		if p.peek().Value == ":" {
			colonCount++
		}
		p.advance()
	}
	
	p.expect(TokenPunc, ")")
	
	// Return as InlineAsmExpr for GCC-style inline assembly
	return &ast.InlineAsmExpr{
		Code:     code,
		StartPos: startPos,
		EndPos:   p.currentPos(),
	}
}

// parseLuaExpression parses @lua(...) expressions
func (p *SimpleParser) parseLuaExpression() ast.Expression {
	startPos := p.currentPos()
	p.advance() // consume '@lua'
	
	// Expect opening parenthesis
	if p.peek().Value != "(" {
		return nil
	}
	p.advance() // consume '('
	
	// Parse the Lua code - for now, just collect tokens until matching ')'
	parenCount := 1
	var codeTokens []string
	
	for parenCount > 0 && !p.isAtEnd() {
		tok := p.peek()
		if tok.Value == "(" {
			parenCount++
		} else if tok.Value == ")" {
			parenCount--
			if parenCount == 0 {
				break
			}
		}
		codeTokens = append(codeTokens, tok.Value)
		p.advance()
	}
	
	if parenCount != 0 {
		return nil // Unmatched parentheses
	}
	
	p.advance() // consume final ')'
	
	// Join tokens to form Lua code
	code := strings.Join(codeTokens, " ")
	
	return &ast.LuaExpression{
		Code:     code,
		StartPos: startPos,
		EndPos:   p.currentPos(),
	}
}

// parseLambdaExpression parses lambda expressions: |params| => Type { body } or |params| { body }
func (p *SimpleParser) parseLambdaExpression() ast.Expression {
	startPos := p.currentPos()
	p.advance() // consume '|'
	
	// Parse parameters
	var params []*ast.LambdaParam
	for p.peek().Value != "|" && !p.isAtEnd() {
		param := &ast.LambdaParam{
			StartPos: p.currentPos(),
		}
		
		// Parameter name
		if p.peek().Type == TokenIdent {
			param.Name = p.advance().Value
		}
		
		// Optional type annotation
		if p.peek().Value == ":" {
			p.advance() // consume ':'
			param.Type = p.parseType()
		}
		
		param.EndPos = p.currentPos()
		params = append(params, param)
		
		if p.peek().Value == "," {
			p.advance() // consume ','
		}
	}
	
	p.expect(TokenPunc, "|") // consume closing '|'
	
	// Check for return type with =>
	var returnType ast.Type
	var body ast.Node
	
	if p.peek().Value == "=" && p.peekAhead(1).Value == ">" {
		// Lambda with explicit return type: |x| => Type { ... }
		p.advance() // consume '='
		p.advance() // consume '>'
		returnType = p.parseType()
		body = p.parseBlock()
	} else if p.peek().Value == "{" {
		// Lambda with block body: |x| { ... }
		body = p.parseBlock()
	} else {
		// Lambda with expression body: |x| expr
		body = p.parseExpression()
	}
	
	return &ast.LambdaExpr{
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		StartPos:   startPos,
		EndPos:     p.currentPos(),
	}
}

// parsePrintExpression parses @print(...) expressions
func (p *SimpleParser) parsePrintExpression() ast.Expression {
	startPos := p.currentPos()
	p.advance() // consume '@print'
	
	// Expect opening parenthesis
	if p.peek().Value != "(" {
		return nil
	}
	p.advance() // consume '('
	
	// Parse the expression inside
	expr := p.parseExpression()
	if expr == nil {
		return nil
	}
	
	// Expect closing parenthesis
	if p.peek().Value != ")" {
		return nil
	}
	p.advance() // consume ')'
	
	return &ast.CompileTimePrint{
		Expr:     expr,
		StartPos: startPos,
		EndPos:   p.currentPos(),
	}
}

// parseMetafunctionCall parses @function_name(...) expressions
func (p *SimpleParser) parseMetafunctionCall() ast.Expression {
	startPos := p.currentPos()
	
	// Get the metafunction name (including @)
	nameToken := p.peek()
	metafunctionName := nameToken.Value[1:] // Remove the @ prefix
	p.advance() // consume '@function_name'
	
	// Expect opening parenthesis
	if p.peek().Value != "(" {
		return nil
	}
	p.advance() // consume '('
	
	// Parse arguments
	var arguments []ast.Expression
	
	// Check for empty argument list
	if p.peek().Value == ")" {
		p.advance() // consume ')'
	} else {
		// Parse first argument
		expr := p.parseExpression()
		if expr == nil {
			return nil
		}
		arguments = append(arguments, expr)
		
		// Parse additional arguments
		for p.peek().Value == "," {
			p.advance() // consume ','
			expr := p.parseExpression()
			if expr == nil {
				return nil
			}
			arguments = append(arguments, expr)
		}
		
		// Expect closing parenthesis
		if p.peek().Value != ")" {
			return nil
		}
		p.advance() // consume ')'
	}
	
	return &ast.MetafunctionCall{
		Name:      metafunctionName,
		Arguments: arguments,
		StartPos:  startPos,
		EndPos:    p.currentPos(),
	}
}

// parseLuaBlock parses @lua[[[ ... ]]] blocks
func (p *SimpleParser) parseLuaBlock() *ast.LuaBlock {
	startPos := p.currentPos()
	p.advance() // consume '@lua'
	
	// Expect triple brackets
	if p.peek().Value != "[" || p.peekAhead(1).Value != "[" || p.peekAhead(2).Value != "[" {
		return nil
	}
	p.advance() // consume first '['
	p.advance() // consume second '['
	p.advance() // consume third '['
	
	// Collect all tokens until ]]]
	var codeBuilder strings.Builder
	
	for !p.isAtEnd() {
		tok := p.peek()
		
		// Check for ]]]
		if tok.Value == "]" && p.peekAhead(1).Value == "]" && p.peekAhead(2).Value == "]" {
			// Found closing ]]]
			p.advance() // consume first ']'
			p.advance() // consume second ']'
			p.advance() // consume third ']'
			break
		}
		
		// Add token to code
		if codeBuilder.Len() > 0 && tok.Type != TokenPunc {
			codeBuilder.WriteString(" ")
		}
		codeBuilder.WriteString(tok.Value)
		p.advance()
	}
	
	return &ast.LuaBlock{
		Code:     codeBuilder.String(),
		StartPos: startPos,
		EndPos:   p.currentPos(),
	}
}

// parsePostfixExpression handles postfix operations like array access and field access
func (p *SimpleParser) parsePostfixExpression(expr ast.Expression) ast.Expression {
	for {
		switch p.peek().Value {
		case "[":
			// Array indexing
			p.advance() // consume '['
			index := p.parseExpression()
			p.expect(TokenPunc, "]")
			expr = &ast.IndexExpr{
				Array:    expr,
				Index:    index,
				StartPos: expr.Pos(),
				EndPos:   p.currentPos(),
			}
		case ".":
			// Field access
			p.advance() // consume '.'
			if p.peek().Type != TokenIdent {
				panic("expected field name after '.'")
			}
			fieldName := p.peek().Value
			p.advance()
			expr = &ast.FieldExpr{
				Object:   expr,
				Field:    fieldName,
				StartPos: expr.Pos(),
				EndPos:   p.currentPos(),
			}
		case "(":
			// Function call on the expression
			p.advance() // consume '('
			
			call := &ast.CallExpr{
				Function:  expr,
				Arguments: []ast.Expression{},
				StartPos:  expr.Pos(),
			}
			
			// Parse arguments
			for p.peek().Value != ")" && !p.isAtEnd() {
				arg := p.parseExpression()
				if arg != nil {
					call.Arguments = append(call.Arguments, arg)
				}
				
				if p.peek().Value == "," {
					p.advance()
				} else if p.peek().Value != ")" {
					break
				}
			}
			
			p.expect(TokenPunc, ")")
			call.EndPos = p.currentPos()
			expr = call
		default:
			// Check for 'as' keyword for cast expressions
			if p.peek().Type == TokenKeyword && p.peek().Value == "as" {
				startPos := expr.Pos()
				p.advance() // consume 'as'
				targetType := p.parseType()
				expr = &ast.CastExpr{
					Expr:       expr,
					TargetType: targetType,
					StartPos:   startPos,
					EndPos:     p.currentPos(),
				}
			} else {
				return expr
			}
		}
	}
}

// parseFunctionCall parses a function call
func (p *SimpleParser) parseFunctionCall(name string, startPos ast.Position) *ast.CallExpr {
	p.expect(TokenPunc, "(")
	
	call := &ast.CallExpr{
		Function: &ast.Identifier{
			Name:     name,
			StartPos: startPos,
			EndPos:   p.currentPos(),
		},
		Arguments: []ast.Expression{},
		StartPos:  startPos,
	}
	
	// Parse arguments
	for p.peek().Value != ")" && !p.isAtEnd() {
		arg := p.parseExpression()
		if arg != nil {
			call.Arguments = append(call.Arguments, arg)
		}
		
		if p.peek().Value == "," {
			p.advance()
		} else if p.peek().Value != ")" {
			break
		}
	}
	
	p.expect(TokenPunc, ")")
	call.EndPos = p.currentPos()
	
	return call
}

// operatorPrecedence returns the precedence of an operator
func operatorPrecedence(op string) int {
	switch op {
	case "||":
		return 1
	case "&&":
		return 2
	case "==", "!=":
		return 3
	case "<", ">", "<=", ">=":
		return 4
	case "+", "-":
		return 5
	case "*", "/", "%":
		return 6
	default:
		return 0
	}
}

// Helper methods

func (p *SimpleParser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *SimpleParser) peekAhead(offset int) Token {
	if p.pos+offset >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos+offset]
}

func (p *SimpleParser) advance() Token {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *SimpleParser) expect(typ TokenType, value string) Token {
	tok := p.peek()
	if tok.Type != typ || tok.Value != value {
		// Error handling - for now just advance
		p.advance()
		return tok
	}
	return p.advance()
}

func (p *SimpleParser) isAtEnd() bool {
	return p.peek().Type == TokenEOF
}

func (p *SimpleParser) currentPos() ast.Position {
	tok := p.peek()
	return ast.Position{
		Line:   tok.Line,
		Column: tok.Column,
	}
}

// Utility functions

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isAlnum(ch byte) bool {
	return isAlpha(ch) || isDigit(ch)
}

func isKeyword(s string) bool {
	keywords := []string{
		"fun", "let", "mut", "if", "else", "while", "for", "return",
		"struct", "enum", "impl", "interface", "pub", "mod", "use", "import",
		"true", "false", "as", "module", "asm", "const", "type",
		"loop", "into", "ref", "to", "indexed", "bits", "at",
		"do", "times", "global", "self",
	}
	for _, kw := range keywords {
		if s == kw {
			return true
		}
	}
	return false
}

func isPrimitiveType(s string) bool {
	types := []string{"void", "bool", "u8", "u16", "i8", "i16", "i32", "u32"}
	for _, t := range types {
		if s == t {
			return true
		}
	}
	return false
}

func isOperator(s string) bool {
	ops := []string{
		"==", "!=", "<=", ">=", "<<", ">>", "&&", "||", "->",
		"+", "-", "*", "/", "%", "&", "^", "!", "~",
		"<", ">",
		// Note: "|" is removed - it's used for lambda syntax
	}
	for _, op := range ops {
		if s == op {
			return true
		}
	}
	return false
}

func isPunctuation(s string) bool {
	puncs := []string{
		"(", ")", "{", "}", "[", "]", ";", ",", ".", ":", "|",
	}
	for _, p := range puncs {
		if s == p {
			return true
		}
	}
	return false
}

// Stub implementations for missing statement parsers
func (p *SimpleParser) parseIfStmt() *ast.IfStmt {
	ifStmt := &ast.IfStmt{
		StartPos: p.currentPos(),
	}
	
	p.expect(TokenKeyword, "if")
	
	// Parse condition
	ifStmt.Condition = p.parseExpression()
	
	// Parse then block
	ifStmt.Then = p.parseBlock()
	
	// Optional else
	if p.peek().Type == TokenKeyword && p.peek().Value == "else" {
		p.advance() // consume 'else'
		
		// Check for else if
		if p.peek().Type == TokenKeyword && p.peek().Value == "if" {
			ifStmt.Else = p.parseIfStmt()
		} else {
			ifStmt.Else = p.parseBlock()
		}
	}
	
	ifStmt.EndPos = p.currentPos()
	return ifStmt
}

func (p *SimpleParser) parseWhileStmt() *ast.WhileStmt {
	whileStmt := &ast.WhileStmt{
		StartPos: p.currentPos(),
	}
	
	p.expect(TokenKeyword, "while")
	
	// Parse condition
	whileStmt.Condition = p.parseExpression()
	
	// Parse body
	whileStmt.Body = p.parseBlock()
	
	whileStmt.EndPos = p.currentPos()
	return whileStmt
}

// parseLoopStmt parses a loop statement
// Syntax: loop <table> into <var> { ... }
//         loop <table> ref to <var> { ... }
//         loop <table> indexed to <var>, <index> { ... }
func (p *SimpleParser) parseLoopStmt() *ast.LoopStmt {
	loopStmt := &ast.LoopStmt{
		StartPos: p.currentPos(),
		Mode:     ast.LoopInto, // Default mode
	}
	
	// Note: "loop" token already consumed by parseLoopStatement()
	// Parse table expression
	loopStmt.Table = p.parseExpression()
	
	// Parse iteration mode
	if p.peek().Type == TokenKeyword {
		switch p.peek().Value {
		case "into":
			p.advance()
			loopStmt.Mode = ast.LoopInto
		case "ref":
			p.advance()
			p.expect(TokenKeyword, "to")
			loopStmt.Mode = ast.LoopRefTo
		case "indexed":
			p.advance()
			p.expect(TokenKeyword, "to")
			// Will parse index variable after iterator
		default:
			// Error: expected 'into' or 'ref to'
			p.advance() // Skip unknown keyword
		}
	}
	
	// Parse iterator variable
	if p.peek().Type == TokenIdent {
		loopStmt.Iterator = p.peek().Value
		p.advance()
		
		// Check for indexed variant
		if p.peek().Value == "," {
			p.advance()
			if p.peek().Type == TokenIdent {
				loopStmt.Index = p.peek().Value
				p.advance()
			}
		}
	}
	
	// Parse body
	loopStmt.Body = p.parseBlock()
	
	loopStmt.EndPos = p.currentPos()
	return loopStmt
}

// parseLoopStatement parses both old and new loop syntax
func (p *SimpleParser) parseLoopStatement() ast.Statement {
	startPos := p.currentPos()
	
	p.expect(TokenKeyword, "loop")
	
	// Check for new syntax: "loop at array -> item"
	if p.peek().Type == TokenKeyword && p.peek().Value == "at" {
		return p.parseLoopAtStatement(startPos)
	}
	
	// Fall back to old syntax for now
	return p.parseLoopStmt()
}

// parseLoopAtStatement parses "loop at array -> item" syntax
func (p *SimpleParser) parseLoopAtStatement(startPos ast.Position) ast.Statement {
	p.expect(TokenKeyword, "at")
	
	// Parse array/table identifier (not full expression to avoid consuming ->)
	if p.peek().Type != TokenIdent {
		return nil
	}
	tableName := p.advance().Value
	table := &ast.Identifier{
		Name: tableName,
		StartPos: p.currentPos(),
		EndPos: p.currentPos(),
	}
	
	// Expect "->"
	p.expect(TokenOperator, "->")
	
	// Check for ! modifier (modification intent)
	isModifying := false
	if p.peek().Value == "!" {
		isModifying = true
		p.advance()
	}
	
	// Parse iterator variable name
	if p.peek().Type != TokenIdent {
		return nil
	}
	iterator := p.advance().Value
	
	// Parse body
	body := p.parseBlock()
	
	return &ast.LoopAtStmt{
		Table:       table,
		Iterator:    iterator,
		IsModifying: isModifying,
		Body:        body,
		StartPos:    startPos,
		EndPos:      p.currentPos(),
	}
}

// parseDoStatement parses a "do N times" statement
func (p *SimpleParser) parseDoStatement() ast.Statement {
	startPos := p.currentPos()
	
	p.expect(TokenKeyword, "do")
	
	// Parse count expression
	count := p.parseExpression()
	
	p.expect(TokenKeyword, "times")
	
	// Parse body
	body := p.parseBlock()
	
	return &ast.DoTimesStmt{
		Count:    count,
		Body:     body,
		StartPos: startPos,
		EndPos:   p.currentPos(),
	}
}

func (p *SimpleParser) parseAsmStmt() *ast.AsmStmt {
	asmStmt := &ast.AsmStmt{
		StartPos: p.currentPos(),
	}
	
	p.expect(TokenKeyword, "asm")
	
	// Check for optional name
	if p.peek().Type == TokenIdent {
		asmStmt.Name = p.peek().Value
		p.advance()
	}
	
	// Expect opening brace
	p.expect(TokenPunc, "{")
	
	// Collect all tokens until closing brace
	// We need to track brace depth for nested braces
	braceDepth := 1
	codeTokens := []Token{}
	
	for !p.isAtEnd() && braceDepth > 0 {
		tok := p.peek()
		
		if tok.Type == TokenPunc && tok.Value == "{" {
			braceDepth++
		} else if tok.Type == TokenPunc && tok.Value == "}" {
			braceDepth--
			if braceDepth == 0 {
				break
			}
		}
		
		codeTokens = append(codeTokens, tok)
		p.advance()
	}
	
	// Expect closing brace
	p.expect(TokenPunc, "}")
	
	// Convert tokens back to raw text
	// This preserves the exact assembly code as written
	code := ""
	prevLine := -1
	for i, tok := range codeTokens {
		// Add newline if we're on a different line
		if tok.Line > prevLine && prevLine != -1 {
			code += "\n"
		}
		prevLine = tok.Line
		
		if i > 0 && tok.Line == codeTokens[i-1].Line {
			// Add appropriate spacing on the same line
			prevTok := codeTokens[i-1]
			if needsSpace(prevTok, tok) {
				code += " "
			}
		}
		code += tok.Value
	}
	
	asmStmt.Code = code
	asmStmt.EndPos = p.currentPos()
	return asmStmt
}

// needsSpace determines if space is needed between two tokens
func needsSpace(prev, curr Token) bool {
	// Special cases
	if prev.Value == "(" || curr.Value == ")" || curr.Value == "," {
		return false
	}
	if prev.Value == ")" || prev.Value == "," {
		return true
	}
	
	// Always add space after keywords, identifiers, and numbers
	if prev.Type == TokenKeyword || prev.Type == TokenIdent || prev.Type == TokenNumber {
		// Except before certain punctuation
		if curr.Value == ")" || curr.Value == "," || curr.Value == ":" {
			return false
		}
		return true
	}
	
	// Add space before keywords, identifiers, and numbers
	if curr.Type == TokenKeyword || curr.Type == TokenIdent || curr.Type == TokenNumber {
		// Except after certain punctuation
		if prev.Value == "(" || prev.Value == "$" {
			return false
		}
		return true
	}
	
	// No space between punctuation by default
	return false
}

func (p *SimpleParser) parseVarDecl() *ast.VarDecl {
	varDecl := &ast.VarDecl{
		StartPos:  p.currentPos(),
		IsMutable: true, // let variables are mutable by default in MinZ
	}
	
	// Parse 'let' keyword
	p.expect(TokenKeyword, "let")
	
	// Check for mut after let (redundant but allowed)
	if p.peek().Type == TokenKeyword && p.peek().Value == "mut" {
		varDecl.IsMutable = true
		p.advance() // consume 'mut'
	}
	
	// Check if next token is a type name or variable name
	tok := p.peek()
	if tok.Type == TokenIdent {
		// Could be type or name
		// Look ahead to see if next is another identifier
		p.advance()
		if p.peek().Type == TokenIdent {
			// First was type, second is name (e.g., "let u16 x")
			varDecl.Type = &ast.PrimitiveType{Name: tok.Value}
			varDecl.Name = p.advance().Value
		} else if p.peek().Value == ":" {
			// Just name, type annotation follows (e.g., "let x:")
			varDecl.Name = tok.Value
			p.advance() // consume ':'
			varDecl.Type = p.parseType()
		} else {
			// Just name, no type (e.g., "let x =")
			varDecl.Name = tok.Value
		}
	}
	
	// Initializer
	if p.peek().Value == "=" {
		p.advance()
		varDecl.Value = p.parseExpression()
	}
	
	p.expect(TokenPunc, ";")
	varDecl.EndPos = p.currentPos()
	
	return varDecl
}

func (p *SimpleParser) parseConstDecl() *ast.ConstDecl {
	constDecl := &ast.ConstDecl{
		StartPos: p.currentPos(),
	}
	
	// Parse 'const' keyword
	p.expect(TokenKeyword, "const")
	
	// Constant name
	if p.peek().Type == TokenIdent {
		constDecl.Name = p.advance().Value
	}
	
	// Type annotation
	if p.peek().Value == ":" {
		p.advance()
		constDecl.Type = p.parseType()
	}
	
	// Initializer (required for const)
	p.expect(TokenOperator, "=")
	
	// Debug: check what token we have before parseExpression
	if p.pos < len(p.tokens) {
		fmt.Printf("DEBUG parseConstDecl: Before parseExpression, next token is type=%v value='%s'\n", p.peek().Type, p.peek().Value)
	}
	
	constDecl.Value = p.parseExpression()
	
	// Debug: check result
	if constDecl.Value == nil {
		fmt.Printf("DEBUG parseConstDecl: parseExpression returned nil\n")
	}
	
	p.expect(TokenPunc, ";")
	constDecl.EndPos = p.currentPos()
	
	return constDecl
}

func (p *SimpleParser) parseGlobalDecl() *ast.VarDecl {
	globalDecl := &ast.VarDecl{
		StartPos:  p.currentPos(),
		IsMutable: true, // Globals are mutable by default
	}
	
	// Parse 'global' keyword
	p.expect(TokenKeyword, "global")
	
	// Type or name
	tok := p.peek()
	if tok.Type == TokenIdent {
		// Could be type or name
		// Look ahead to see if next is another identifier
		p.advance()
		if p.peek().Type == TokenIdent {
			// First was type, second is name
			globalDecl.Type = &ast.PrimitiveType{Name: tok.Value}
			globalDecl.Name = p.advance().Value
		} else {
			// Just name, type will be inferred
			globalDecl.Name = tok.Value
		}
	}
	
	// Optional type annotation
	if p.peek().Value == ":" {
		p.advance()
		globalDecl.Type = p.parseType()
	}
	
	// Optional initializer
	if p.peek().Value == "=" {
		p.advance()
		globalDecl.Value = p.parseExpression()
	}
	
	p.expect(TokenPunc, ";")
	globalDecl.EndPos = p.currentPos()
	
	return globalDecl
}

func (p *SimpleParser) parseStructDecl() *ast.StructDecl {
	structDecl := &ast.StructDecl{
		StartPos: p.currentPos(),
		Fields:   []*ast.Field{},
	}
	
	p.expect(TokenKeyword, "struct")
	
	// Struct name
	if p.peek().Type == TokenIdent {
		structDecl.Name = p.advance().Value
	}
	
	// Parse struct body
	p.expect(TokenPunc, "{")
	
	for p.peek().Value != "}" && !p.isAtEnd() {
		// Parse field
		field := p.parseStructField()
		if field != nil {
			structDecl.Fields = append(structDecl.Fields, field)
		}
		
		// Skip optional comma or semicolon
		if p.peek().Value == "," || p.peek().Value == ";" {
			p.advance()
		}
	}
	
	p.expect(TokenPunc, "}")
	structDecl.EndPos = p.currentPos()
	
	return structDecl
}

func (p *SimpleParser) parseTypeDecl() ast.Declaration {
	startPos := p.currentPos()
	p.expect(TokenKeyword, "type")
	
	// Get type name
	if p.peek().Type != TokenIdent {
		return nil
	}
	name := p.advance().Value
	
	// Expect =
	p.expect(TokenOperator, "=")
	
	// Parse the type
	if p.peek().Type == TokenKeyword {
		switch p.peek().Value {
		case "struct":
			p.advance() // consume 'struct'
			
			structDecl := &ast.StructDecl{
				Name:     name,
				Fields:   []*ast.Field{},
				IsPublic: false,
				StartPos: startPos,
			}
			
			// Parse struct body
			p.expect(TokenPunc, "{")
			
			for p.peek().Value != "}" && !p.isAtEnd() {
				// Parse field
				field := p.parseStructField()
				if field != nil {
					structDecl.Fields = append(structDecl.Fields, field)
				}
				
				// Skip optional comma or semicolon
				if p.peek().Value == "," || p.peek().Value == ";" {
					p.advance()
				}
			}
			
			p.expect(TokenPunc, "}")
			p.expect(TokenPunc, ";")
			
			structDecl.EndPos = p.currentPos()
			return structDecl
			
		case "bits":
			p.advance() // consume 'bits'
			
			// Parse optional underlying type
			var underlyingType ast.Type
			if p.peek().Value == "<" {
				p.advance() // consume '<'
				underlyingType = p.parseType()
				p.expect(TokenPunc, ">")
			}
			
			// Parse bit fields
			bitStruct := &ast.BitStructType{
				UnderlyingType: underlyingType,
				Fields:         []*ast.BitField{},
				StartPos:       startPos,
			}
			
			p.expect(TokenPunc, "{")
			
			for p.peek().Value != "}" && !p.isAtEnd() {
				// Parse bit field
				field := p.parseBitField()
				if field != nil {
					bitStruct.Fields = append(bitStruct.Fields, field)
				}
				
				// Skip optional comma or semicolon
				if p.peek().Value == "," || p.peek().Value == ";" {
					p.advance()
				}
			}
			
			p.expect(TokenPunc, "}")
			
			bitStruct.EndPos = p.currentPos()
			
			// Create TypeDecl wrapper
			typeDecl := &ast.TypeDecl{
				Name:     name,
				Type:     bitStruct,
				IsPublic: false,
				StartPos: startPos,
				EndPos:   p.currentPos(),
			}
			
			p.expect(TokenPunc, ";")
			
			return typeDecl
		}
	}
	
	return nil
}

func (p *SimpleParser) parseStructField() *ast.Field {
	if p.peek().Type != TokenIdent {
		return nil
	}
	
	field := &ast.Field{
		Name:     p.advance().Value,
		StartPos: p.currentPos(),
	}
	
	p.expect(TokenPunc, ":")
	field.Type = p.parseType()
	field.EndPos = p.currentPos()
	
	return field
}

func (p *SimpleParser) parseBitField() *ast.BitField {
	if p.peek().Type != TokenIdent {
		return nil
	}
	
	field := &ast.BitField{
		Name:     p.advance().Value,
		StartPos: p.currentPos(),
	}
	
	p.expect(TokenPunc, ":")
	
	// Parse bit width
	if p.peek().Type != TokenNumber {
		return nil
	}
	
	width, err := strconv.Atoi(p.advance().Value)
	if err != nil || width < 1 || width > 16 {
		return nil
	}
	
	field.BitWidth = width
	field.EndPos = p.currentPos()
	
	return field
}