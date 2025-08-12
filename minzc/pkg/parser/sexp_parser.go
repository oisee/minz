package parser

import (
	"fmt"
	"os"
	"strings"
	"strconv"
	"github.com/minz/minzc/pkg/ast"
)

// parseSExpression parses tree-sitter S-expression output
func parseSExpression(input string) (*SExpNode, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("empty S-expression input")
	}
	parser := &sexpParser{input: input, pos: 0}
	return parser.parseNode()
}

type sexpParser struct {
	input string
	pos   int
}

type SExpNode struct {
	Type     string
	Text     string
	StartPos ast.Position
	EndPos   ast.Position
	Children []*SExpNode
	IsMissing bool
}

func (p *sexpParser) parseNode() (*SExpNode, error) {
	p.skipWhitespace()
	
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of input")
	}
	
	if p.input[p.pos] != '(' {
		// Parse identifier or literal
		return p.parseAtom()
	}
	
	// Parse list node
	p.pos++ // skip '('
	p.skipWhitespace()
	
	// Parse node type
	nodeType := p.parseIdentifier()
	
	node := &SExpNode{
		Type: nodeType,
	}
	
	// Parse position if present
	p.skipWhitespace()
	if p.pos < len(p.input) && p.input[p.pos] == '[' {
		startPos, endPos := p.parsePosition()
		node.StartPos = startPos
		node.EndPos = endPos
	}
	
	// Parse children
	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) || p.input[p.pos] == ')' {
			break
		}
		
		child, err := p.parseNode()
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, child)
	}
	
	if p.pos >= len(p.input) || p.input[p.pos] != ')' {
		return nil, fmt.Errorf("expected ')' at position %d", p.pos)
	}
	p.pos++ // skip ')'
	
	return node, nil
}

func (p *sexpParser) parseAtom() (*SExpNode, error) {
	// Check for MISSING token
	if p.pos+7 <= len(p.input) && p.input[p.pos:p.pos+7] == "MISSING" {
		p.pos += 7
		p.skipWhitespace()
		
		// Skip the actual missing token (e.g., ";")
		if p.pos < len(p.input) && p.input[p.pos] == '"' {
			p.pos++ // skip opening quote
			for p.pos < len(p.input) && p.input[p.pos] != '"' {
				p.pos++
			}
			if p.pos < len(p.input) {
				p.pos++ // skip closing quote
			}
		}
		
		return &SExpNode{Type: "MISSING", IsMissing: true}, nil
	}
	
	// Regular atom parsing
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] != ' ' && p.input[p.pos] != ')' && p.input[p.pos] != '(' && p.input[p.pos] != '[' {
		p.pos++
	}
	text := p.input[start:p.pos]
	return &SExpNode{Type: "atom", Text: text}, nil
}

func (p *sexpParser) parseIdentifier() string {
	start := p.pos
	for p.pos < len(p.input) && (isAlpha(p.input[p.pos]) || p.input[p.pos] == '_' || (p.pos > start && isDigit(p.input[p.pos]))) {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *sexpParser) parsePosition() (ast.Position, ast.Position) {
	// Parse [start_row, start_col] - [end_row, end_col]
	p.pos++ // skip '['
	
	startRow := p.parseInt()
	p.skipComma()
	startCol := p.parseInt()
	
	p.pos++ // skip ']'
	p.skipWhitespace()
	p.pos++ // skip '-'
	p.skipWhitespace()
	p.pos++ // skip '['
	
	endRow := p.parseInt()
	p.skipComma()
	endCol := p.parseInt()
	
	p.pos++ // skip ']'
	
	return ast.Position{Line: startRow + 1, Column: startCol + 1}, 
	       ast.Position{Line: endRow + 1, Column: endCol + 1}
}

func (p *sexpParser) parseInt() int {
	p.skipWhitespace()
	start := p.pos
	for p.pos < len(p.input) && isDigit(p.input[p.pos]) {
		p.pos++
	}
	val, _ := strconv.Atoi(p.input[start:p.pos])
	return val
}

func (p *sexpParser) skipWhitespace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t' || p.input[p.pos] == '\n' || p.input[p.pos] == '\r') {
		p.pos++
	}
}

func (p *sexpParser) skipComma() {
	p.skipWhitespace()
	if p.pos < len(p.input) && p.input[p.pos] == ',' {
		p.pos++
	}
	p.skipWhitespace()
}

// convertSExpToAST converts S-expression tree to our AST
func (p *Parser) convertSExpToAST(filename string, sexp *SExpNode) (*ast.File, error) {
	if debug {
		fmt.Printf("DEBUG: convertSExpToAST starting with root type: %s\n", sexp.Type)
	}
	
	if sexp.Type != "source_file" {
		return nil, fmt.Errorf("expected source_file, got %s", sexp.Type)
	}
	
	file := &ast.File{
		Name:         filename,
		Imports:      []*ast.ImportStmt{},
		Declarations: []ast.Declaration{},
	}
	
	debug := os.Getenv("DEBUG") != ""
	if debug {
		fmt.Printf("DEBUG: Converting S-expression with %d children\n", len(sexp.Children))
	}
	
	for i, child := range sexp.Children {
		if debug {
			fmt.Printf("DEBUG: Child %d type: %s\n", i, child.Type)
		}
		switch child.Type {
		case "comment":
			// Skip comments
		case "import_statement":
			if imp := p.convertImportStmt(child); imp != nil {
				file.Imports = append(file.Imports, imp)
			}
		case "declaration":
			if len(child.Children) > 0 {
				decl := p.convertDeclaration(child.Children[0])
				if decl != nil {
					file.Declarations = append(file.Declarations, decl)
				}
			}
		case "statement":
			// Tree-sitter may wrap declarations in statement nodes
			if len(child.Children) > 0 {
				// Try to extract declaration from statement
				decl := p.convertDeclaration(child.Children[0])
				if decl != nil {
					file.Declarations = append(file.Declarations, decl)
				}
			}
		default:
			// Try to convert as declaration directly (for unwrapped declarations)
			if debug {
				fmt.Printf("DEBUG: Trying to convert %s as declaration\n", child.Type)
			}
			decl := p.convertDeclaration(child)
			if decl != nil {
				if debug {
					fmt.Printf("DEBUG: Successfully converted %s to %T\n", child.Type, decl)
				}
				file.Declarations = append(file.Declarations, decl)
			} else if debug {
				fmt.Printf("DEBUG: Failed to convert %s\n", child.Type)
			}
		}
	}
	
	return file, nil
}

func (p *Parser) convertDeclaration(node *SExpNode) ast.Declaration {
	switch node.Type {
	case "attributed_declaration":
		return p.convertAttributedDeclaration(node)
	case "function_declaration":
		return p.convertFunction(node)
	case "asm_function":
		return p.convertAsmFunction(node)
	case "mir_function":
		return p.convertMirFunction(node)
	case "variable_declaration":
		return p.convertVarDecl(node)
	case "struct_declaration":
		return p.convertStructDecl(node)
	case "enum_declaration":
		return p.convertEnumDecl(node)
	case "constant_declaration":
		return p.convertConstDecl(node)
	case "type_alias":
		return p.convertTypeAlias(node)
	case "lua_block":
		return p.convertLuaBlock(node)
	case "mir_block_declaration":
		return p.convertMIRBlock(node)
	case "expression_statement":
		// Handle top-level @minz and other metaprogramming expressions
		if len(node.Children) > 0 && node.Children[0].Type == "expression" {
			expr := p.convertExpression(node.Children[0])
			if minz, ok := expr.(*ast.CompileTimeMinz); ok {
				// Wrap @minz in a declaration-like structure for now
				// TODO: Create a proper MetaDeclaration type
				return &ast.ExpressionDecl{
					Expression: minz,
					StartPos:   node.StartPos,
					EndPos:     node.EndPos,
				}
			}
		}
		return nil
	case "interface_declaration":
		return p.convertInterfaceDecl(node)
	case "impl_block":
		return p.convertImplBlock(node)
	case "minz_block":
		return p.convertMinzBlock(node)
	case "minz_metafunction_declaration":
		return p.convertMinzMetafunctionDecl(node)
	case "define_template":
		return p.convertDefineTemplate(node)
	case "meta_execution_block":
		return p.convertMetaExecutionBlock(node)
	case "lua_execution_block":
		return p.convertLuaExecutionBlock(node)
	case "minz_execution_block":
		return p.convertMinzExecutionBlock(node)
	case "mir_execution_block":
		return p.convertMIRExecutionBlock(node)
	}
	return nil
}

func (p *Parser) convertFunction(node *SExpNode) *ast.FunctionDecl {
	fn := &ast.FunctionDecl{
		Params:   []*ast.Parameter{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	var hasErrorSuffix bool
	
	for _, child := range node.Children {
		switch child.Type {
		case "visibility":
			if p.getNodeText(child) == "pub" {
				fn.IsPublic = true
			}
		case "export":
			fn.IsExport = true
		case "identifier":
			if fn.Name == "" {
				name := p.getNodeText(child)
				// Check if function name ends with ?
				if strings.HasSuffix(name, "?") {
					hasErrorSuffix = true
					fn.Name = strings.TrimSuffix(name, "?")
				} else {
					fn.Name = name
				}
			}
		case "generic_parameters":
			fn.GenericParams = p.convertGenericParameters(child)
		case "parameter_list":
			fn.Params = p.convertParameters(child)
		case "return_type":
			fn.ReturnType, fn.ErrorType = p.convertReturnTypeWithError(child)
		case "block":
			fn.Body = p.convertBlock(child)
		}
	}
	
	// If function has ? suffix but no explicit error type, create a generic error type
	if hasErrorSuffix && fn.ErrorType == nil {
		// Create a generic error type (any error allowed)
		fn.ErrorType = &ast.TypeIdentifier{
			Name:     "Error",
			StartPos: fn.StartPos,
			EndPos:   fn.EndPos,
		}
	}
	
	return fn
}

func (p *Parser) convertAttributedDeclaration(node *SExpNode) ast.Declaration {
	var attr *ast.Attribute
	var decl ast.Declaration
	
	for _, child := range node.Children {
		switch child.Type {
		case "attribute":
			attr = p.convertAttribute(child)
		case "declaration":
			// Handle nested declaration node
			if len(child.Children) > 0 {
				decl = p.convertDeclaration(child.Children[0])
			}
		default:
			decl = p.convertDeclaration(child)
		}
	}
	
	// Add attribute to the declaration if it's a function
	if funcDecl, ok := decl.(*ast.FunctionDecl); ok && attr != nil {
		funcDecl.Attributes = []*ast.Attribute{attr}
	}
	
	return decl
}

func (p *Parser) convertAttribute(node *SExpNode) *ast.Attribute {
	attr := &ast.Attribute{
		Arguments: []ast.Expression{},
		StartPos:  node.StartPos,
		EndPos:    node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			if attr.Name == "" {
				attr.Name = p.getNodeText(child)
			}
		case "argument_list":
			// Convert arguments to expressions
			for _, arg := range child.Children {
				if expr := p.convertExpression(arg); expr != nil {
					attr.Arguments = append(attr.Arguments, expr)
				}
			}
		}
	}
	
	return attr
}

func (p *Parser) convertBlock(node *SExpNode) *ast.BlockStmt {
	block := &ast.BlockStmt{
		Statements: []ast.Statement{},
		StartPos:   node.StartPos,
		EndPos:     node.EndPos,
	}
	
	for _, child := range node.Children {
		if child.Type == "statement" && len(child.Children) > 0 {
			stmt := p.convertStatement(child.Children[0])
			if stmt != nil {
				block.Statements = append(block.Statements, stmt)
			}
		} else if child.Type == "expression" {
			// Handle bare expressions in blocks (common in lambda bodies)
			expr := p.convertExpression(child)
			if expr != nil {
				block.Statements = append(block.Statements, &ast.ExpressionStmt{
					Expression: expr,
					StartPos:   child.StartPos,
					EndPos:     child.EndPos,
				})
			}
		}
	}
	
	return block
}

func (p *Parser) convertStatement(node *SExpNode) ast.Statement {
	switch node.Type {
	case "variable_declaration":
		return p.convertVarDecl(node)
	case "constant_declaration":
		return p.convertConstDecl(node)
	case "return_statement":
		return p.convertReturnStmt(node)
	case "if_statement":
		return p.convertIfStmt(node)
	case "while_statement":
		return p.convertWhileStmt(node)
	case "for_statement":
		return p.convertForStmt(node)
	case "case_statement":
		return p.convertCaseStmt(node)
	case "expression_statement":
		return p.convertExpressionStmt(node)
	case "assignment_statement":
		return p.convertAssignmentStmt(node)
	case "loop_statement":
		return p.convertLoopStmt(node)
	case "asm_block":
		return p.convertAsmBlock(node)
	case "mir_block":
		return p.convertMirBlock(node)
	case "minz_block":
		return p.convertMinzBlock(node)
	case "minz_emit":
		return p.convertMinzEmit(node)
	case "target_block":
		return p.convertTargetBlock(node)
	case "compile_time_asm":
		return p.convertCompileTimeAsm(node)
	}
	return nil
}

func (p *Parser) convertVarDecl(node *SExpNode) *ast.VarDecl {
	varDecl := &ast.VarDecl{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Extract var/let keyword from source code
	if p.sourceCode != "" {
		lines := strings.Split(p.sourceCode, "\n")
		if node.StartPos.Line > 0 && node.StartPos.Line <= len(lines) {
			line := lines[node.StartPos.Line-1]
			// Look for "var", "global", or "let" at the start of the declaration
			trimmed := strings.TrimSpace(line[node.StartPos.Column-1:])
			if strings.HasPrefix(trimmed, "var ") || strings.HasPrefix(trimmed, "global ") {
				varDecl.IsMutable = true  // 'global' is a developer-friendly synonym for 'var'
			} else if strings.HasPrefix(trimmed, "let ") {
				varDecl.IsMutable = true  // let variables are mutable in MinZ
			}
		}
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "visibility":
			if p.getNodeText(child) == "pub" {
				varDecl.IsPublic = true
			}
		case "identifier":
			if varDecl.Name == "" {
				varDecl.Name = p.getNodeText(child)
			}
		case "type":
			varDecl.Type = p.convertType(child)
		case "expression":
			varDecl.Value = p.convertExpression(child)
		}
	}
	
	return varDecl
}

func (p *Parser) convertType(node *SExpNode) ast.Type {
	// If this is already a type node (e.g., primitive_type), handle it directly
	if node.Type == "primitive_type" || node.Type == "type_identifier" || 
	   node.Type == "array_type" || node.Type == "pointer_type" {
		return p.convertTypeNode(node)
	}
	// Otherwise, it's a wrapper node with children
	if len(node.Children) > 0 {
		return p.convertTypeNode(node.Children[0])
	}
	return nil
}

func (p *Parser) convertTypeNode(node *SExpNode) ast.Type {
	switch node.Type {
	case "primitive_type":
		return &ast.PrimitiveType{
			Name:     p.getNodeText(node),
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	case "array_type":
		return p.convertArrayType(node)
	case "pointer_type":
		return p.convertPointerType(node)
	case "bit_struct_type":
		return p.convertBitStructType(node)
	case "type_identifier":
		// User-defined types (structs, enums, type aliases)
		name := ""
		if len(node.Children) > 0 && node.Children[0].Type == "identifier" {
			name = p.getNodeText(node.Children[0])
		}
		return &ast.TypeIdentifier{
			Name:     name,
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	}
	return nil
}

func (p *Parser) convertExpression(node *SExpNode) ast.Expression {
	if debug {
		fmt.Printf("DEBUG: convertExpression called with type=%s, children=%d\n", node.Type, len(node.Children))
	}
	
	if len(node.Children) > 0 {
		result := p.convertExpressionNode(node.Children[0])
		if debug {
			fmt.Printf("DEBUG: convertExpression returning %T\n", result)
		}
		return result
	}
	
	if debug {
		fmt.Printf("DEBUG: convertExpression returning nil (no children)\n")
	}
	return nil
}

func (p *Parser) convertExpressionNode(node *SExpNode) ast.Expression {
	if debug {
		fmt.Printf("DEBUG: convertExpressionNode type=%s, children=%d\n", node.Type, len(node.Children))
	}
	
	switch node.Type {
	case "expression":
		// Expression wrapper - recurse into child
		if len(node.Children) > 0 {
			return p.convertExpressionNode(node.Children[0])
		}
	case "postfix_expression":
		if len(node.Children) > 0 {
			return p.convertExpressionNode(node.Children[0])
		}
	case "primary_expression":
		if len(node.Children) > 0 {
			return p.convertExpressionNode(node.Children[0])
		}
	case "metaprogramming_expression":
		if len(node.Children) > 0 {
			return p.convertExpressionNode(node.Children[0])
		}
	case "attribute":
		// Handle @function(...) calls in expression context
		return p.convertAttributeAsExpression(node)
	case "lua_expression":
		// Extract the lua_code child
		for _, child := range node.Children {
			if child.Type == "lua_code" {
				code := p.getNodeText(child)
				return &ast.LuaExpression{
					Code:     code,
					StartPos: node.StartPos,
					EndPos:   node.EndPos,
				}
			}
		}
	case "compile_time_print":
		// @print(string_literal) - happy syntax with { } interpolation
		if debug {
			fmt.Printf("DEBUG: Converting compile_time_print with %d children\n", len(node.Children))
			for i, child := range node.Children {
				fmt.Printf("  Child %d: type=%s, text=%q, has_text=%v\n", i, child.Type, child.Text, child.Text != "")
			}
		}
		for _, child := range node.Children {
			// The grammar now has string_literal directly, not wrapped in expression
			if child.Type == "string_literal" {
				// Parse the string literal
				text := p.getNodeText(child)
				if debug {
					fmt.Printf("DEBUG: String literal raw text: %q\n", text)
				}
				// Remove quotes if present
				if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
					text = text[1 : len(text)-1]
				}
				value := p.unescapeString(text)
				if debug {
					fmt.Printf("DEBUG: Parsed string literal: %q\n", value)
				}
				return &ast.CompileTimePrint{
					Expr: &ast.StringLiteral{
						Value:    value,
						StartPos: child.StartPos,
						EndPos:   child.EndPos,
					},
					StartPos: node.StartPos,
					EndPos:   node.EndPos,
				}
			}
			// Fallback for old style with expression (shouldn't happen with new grammar)
			if child.Type == "expression" {
				expr := p.convertExpression(child)
				return &ast.CompileTimePrint{
					Expr:     expr,
					StartPos: node.StartPos,
					EndPos:   node.EndPos,
				}
			}
		}
		// If we didn't find a string_literal or expression, return nil
		// This will cause an error later which is what we want
		if debug {
			fmt.Printf("DEBUG: compile_time_print didn't find string_literal or expression child\n")
		}
		return nil
	case "compile_time_if":
		// @if(condition, then_expr, else_expr)
		var condition, thenExpr, elseExpr ast.Expression
		expressions := make([]ast.Expression, 0, 3)
		
		// Collect all expressions in order
		for _, child := range node.Children {
			if child.Type == "expression" {
				expressions = append(expressions, p.convertExpression(child))
			}
		}
		
		// Assign based on count
		if len(expressions) >= 2 {
			condition = expressions[0]
			thenExpr = expressions[1]
			if len(expressions) >= 3 {
				elseExpr = expressions[2]
			}
			
			return &ast.CompileTimeIf{
				Condition: condition,
				ThenExpr:  thenExpr,
				ElseExpr:  elseExpr,
				StartPos:  node.StartPos,
				EndPos:    node.EndPos,
			}
		}
	case "compile_time_minz":
		// @minz[[[code]]](...args...)
		var code string
		var args []ast.Expression
		
		for _, child := range node.Children {
			if child.Type == "minz_code_block" {
				// Extract the MinZ code template
				code = p.getNodeText(child)
			} else if child.Type == "argument_list" {
				// Parse arguments
				for _, argChild := range child.Children {
					if argChild.Type == "expression" {
						args = append(args, p.convertExpression(argChild))
					}
				}
			}
		}
		
		return &ast.CompileTimeMinz{
			Code:      code,
			Arguments: args,
			StartPos:  node.StartPos,
			EndPos:    node.EndPos,
		}
	case "compile_time_mir":
		// @mir[[[code]]]
		var code string
		
		for _, child := range node.Children {
			if child.Type == "mir_code_block" {
				// Extract the MIR code from the block
				code = p.getNodeText(child)
				break
			}
		}
		
		return &ast.CompileTimeMIR{
			Code:     code,
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	case "compile_time_error":
		// @error(expression), @error(), or @error
		var errorValue ast.Expression
		for _, child := range node.Children {
			if child.Type == "expression" {
				errorValue = p.convertExpression(child)
				break
			}
		}
		return &ast.CompileTimeError{
			ErrorValue: errorValue,  // nil for @error without arguments
			StartPos:   node.StartPos,
			EndPos:     node.EndPos,
		}
	case "number_literal":
		val, _ := strconv.ParseInt(p.getNodeText(node), 0, 64)
		return &ast.NumberLiteral{
			Value:    val,
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	case "identifier":
		return &ast.Identifier{
			Name:     p.getNodeText(node),
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	case "string_literal":
		text := p.getNodeText(node)
		isLong := false
		
		// Check for l or L prefix
		if len(text) > 0 && (text[0] == 'l' || text[0] == 'L') {
			isLong = true
			text = text[1:] // Remove the 'l' or 'L' prefix
		}
		
		// Remove quotes if present
		if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
			text = text[1 : len(text)-1]
		}
		// Process escape sequences
		text = unescapeString(text)
		return &ast.StringLiteral{
			Value:    text,
			IsLong:   isLong,
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	case "struct_literal":
		return p.convertStructLiteral(node)
	case "array_literal":
		return p.convertArrayLiteral(node)
	case "binary_expression":
		return p.convertBinaryExpr(node)
	case "call_expression":
		return p.convertCallExpr(node)
	case "index_expression":
		return p.convertIndexExpr(node)
	case "field_expression":
		return p.convertFieldExpr(node)
	case "cast_expression":
		// Convert cast expression properly
		return p.convertCastExpr(node)
	case "try_expression":
		// Convert the try expression (? operator for error propagation)
		return p.convertTryExpr(node)
	case "parenthesized_expression":
		// Extract the inner expression from parentheses
		if len(node.Children) > 0 {
			for _, child := range node.Children {
				if child.Type == "expression" && len(child.Children) > 0 {
					return p.convertExpressionNode(child.Children[0])
				}
			}
		}
	case "unary_expression":
		return p.convertUnaryExpr(node)
	case "lambda_expression":
		return p.convertLambdaExpr(node)
	case "if_expression":
		return p.convertIfExpr(node)
	case "ternary_expression":
		return p.convertTernaryExpr(node)
	case "when_expression":
		return p.convertWhenExpr(node)
	case "boolean_literal":
		return &ast.BooleanLiteral{
			Value:    p.getNodeText(node) == "true",
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	}
	return nil
}

func (p *Parser) convertUnaryExpr(node *SExpNode) *ast.UnaryExpr {
	unaryExpr := &ast.UnaryExpr{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Tree-sitter format: (unary_expression (expression ...))
	// The operator is in the source between start of unary_expression and start of inner expression
	var exprNode *SExpNode
	for _, child := range node.Children {
		if child.Type == "expression" && len(child.Children) > 0 {
			exprNode = child
			unaryExpr.Operand = p.convertExpressionNode(child.Children[0])
			break
		}
	}
	
	// Extract operator from source code
	if exprNode != nil && p.sourceCode != "" {
		lines := strings.Split(p.sourceCode, "\n")
		if node.StartPos.Line > 0 && node.StartPos.Line <= len(lines) {
			line := lines[node.StartPos.Line-1]
			startCol := node.StartPos.Column - 1
			endCol := exprNode.StartPos.Column - 1
			if startCol >= 0 && endCol <= len(line) && startCol < endCol {
				operatorText := strings.TrimSpace(line[startCol:endCol])
				unaryExpr.Operator = operatorText
			}
		}
	}
	
	return unaryExpr
}

func (p *Parser) convertBinaryExpr(node *SExpNode) ast.Expression {
	binExpr := &ast.BinaryExpr{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	var leftNode, rightNode *SExpNode
	
	// Tree-sitter S-expression format: (binary_expression left: ... right: ...)
	// The operator is implicit in the position between left and right
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			// This is a field label
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				switch fieldName {
				case "left":
					leftNode = fieldValue
					if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						binExpr.Left = p.convertExpressionNode(fieldValue.Children[0])
					}
				case "right":
					rightNode = fieldValue
					if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						binExpr.Right = p.convertExpressionNode(fieldValue.Children[0])
					}
				}
				i++ // Skip the field value
			}
		}
	}
	
	// Extract operator from source between left and right expressions
	if leftNode != nil && rightNode != nil && p.sourceCode != "" {
		// Get text between end of left and start of right
		leftEndPos := leftNode.EndPos
		rightStartPos := rightNode.StartPos
		
		lines := strings.Split(p.sourceCode, "\n")
		if leftEndPos.Line == rightStartPos.Line && leftEndPos.Line > 0 && leftEndPos.Line <= len(lines) {
			line := lines[leftEndPos.Line-1]
			startCol := leftEndPos.Column - 1
			endCol := rightStartPos.Column - 1
			if startCol >= 0 && endCol <= len(line) && startCol < endCol {
				operatorText := strings.TrimSpace(line[startCol:endCol])
				binExpr.Operator = operatorText
				
				// Special handling for nil coalescing operator (??)
				if operatorText == "??" {
					return &ast.NilCoalescingExpr{
						Left:     binExpr.Left,
						Right:    binExpr.Right,
						StartPos: binExpr.StartPos,
						EndPos:   binExpr.EndPos,
					}
				}
			}
		}
	}
	
	return binExpr
}

func (p *Parser) convertCallExpr(node *SExpNode) ast.Expression {
	callExpr := &ast.CallExpr{
		Arguments: []ast.Expression{},
		StartPos:  node.StartPos,
		EndPos:    node.EndPos,
	}
	
	// Look for function field and arguments
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				if fieldName == "function" {
					if fieldValue.Type == "postfix_expression" && len(fieldValue.Children) > 0 {
						callExpr.Function = p.convertExpressionNode(fieldValue.Children[0])
					}
				}
				i++ // Skip field value
			}
		} else if child.Type == "argument_list" {
			// Parse arguments
			for _, argChild := range child.Children {
				if argChild.Type == "expression" {
					expr := p.convertExpression(argChild)
					if expr != nil {
						callExpr.Arguments = append(callExpr.Arguments, expr)
					}
				}
			}
		}
	}
	
	// Check if this is an iterator method call that should be transformed
	if fieldExpr, ok := callExpr.Function.(*ast.FieldExpr); ok {
		if p.isIteratorMethod(fieldExpr.Field) {
			// Transform to iterator chain
			return p.transformSExpToIteratorChain(fieldExpr, callExpr.Arguments)
		}
	}
	
	// Check if this is a metafunction call (function name starts with @)
	if ident, ok := callExpr.Function.(*ast.Identifier); ok && strings.HasPrefix(ident.Name, "@") {
		metafunctionName := ident.Name[1:] // Remove @ prefix
		
		// Handle @minz specially for compile-time metaprogramming
		if metafunctionName == "minz" {
			return p.convertMinzMetafunction(callExpr.Arguments, callExpr.StartPos, callExpr.EndPos)
		}
		
		return &ast.MetafunctionCall{
			Name:      metafunctionName,
			Arguments: callExpr.Arguments,
			StartPos:  callExpr.StartPos,
			EndPos:    callExpr.EndPos,
		}
	}
	
	return callExpr
}

func (p *Parser) convertIndexExpr(node *SExpNode) *ast.IndexExpr {
	indexExpr := &ast.IndexExpr{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Tree-sitter format: (index_expression object: ... index: ...)
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				switch fieldName {
				case "object":
					if fieldValue.Type == "postfix_expression" && len(fieldValue.Children) > 0 {
						indexExpr.Array = p.convertExpressionNode(fieldValue.Children[0])
					}
				case "index":
					if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						indexExpr.Index = p.convertExpressionNode(fieldValue.Children[0])
					}
				}
				i++
			}
		}
	}
	
	return indexExpr
}

func (p *Parser) convertFieldExpr(node *SExpNode) *ast.FieldExpr {
	fieldExpr := &ast.FieldExpr{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Tree-sitter format: (field_expression object: ... field: ...)
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				switch fieldName {
				case "object":
					if fieldValue.Type == "postfix_expression" && len(fieldValue.Children) > 0 {
						fieldExpr.Object = p.convertExpressionNode(fieldValue.Children[0])
					}
				case "field":
					if fieldValue.Type == "identifier" {
						fieldExpr.Field = p.getNodeText(fieldValue)
					}
				}
				i++
			}
		}
	}
	
	return fieldExpr
}

func (p *Parser) convertCastExpr(node *SExpNode) *ast.CastExpr {
	castExpr := &ast.CastExpr{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Tree-sitter format: (cast_expression expression: ... type: ...)
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				switch fieldName {
				case "expression":
					if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						castExpr.Expr = p.convertExpressionNode(fieldValue.Children[0])
					}
				case "type":
					// Parse the type properly
					if fieldValue.Type == "type" {
						castExpr.TargetType = p.convertType(fieldValue)
					}
				}
				i++
			}
		}
	}
	
	return castExpr
}

func (p *Parser) getNodeText(node *SExpNode) string {
	// Extract text from source code using node positions
	if p.sourceCode == "" || node.StartPos.Line == 0 {
		return strings.TrimSpace(node.Text)
	}
	
	lines := strings.Split(p.sourceCode, "\n")
	startLine := node.StartPos.Line - 1
	endLine := node.EndPos.Line - 1
	startCol := node.StartPos.Column - 1
	endCol := node.EndPos.Column - 1
	
	if startLine < 0 || startLine >= len(lines) {
		return ""
	}
	
	if startLine == endLine {
		// Single line
		line := lines[startLine]
		if startCol >= 0 && endCol <= len(line) && startCol < endCol {
			return line[startCol:endCol]
		}
	} else {
		// Multi-line - simplified for now
		var text strings.Builder
		for i := startLine; i <= endLine && i < len(lines); i++ {
			if i == startLine {
				text.WriteString(lines[i][startCol:])
			} else if i == endLine {
				if endCol <= len(lines[i]) {
					text.WriteString(lines[i][:endCol])
				}
			} else {
				text.WriteString(lines[i])
			}
			if i < endLine {
				text.WriteString("\n")
			}
		}
		return text.String()
	}
	
	return ""
}

// convertParameters converts a parameter_list node to Parameter array
func (p *Parser) convertParameters(node *SExpNode) []*ast.Parameter {
	params := []*ast.Parameter{}
	
	// Look for parameter nodes
	for _, child := range node.Children {
		if child.Type == "parameter" {
			// Check if it's just "self"
			if len(child.Children) == 0 && p.getNodeText(child) == "self" {
				param := &ast.Parameter{
					Name:     "self",
					IsSelf:   true,
					StartPos: child.StartPos,
					EndPos:   child.EndPos,
				}
				params = append(params, param)
			} else {
				param := &ast.Parameter{
					StartPos: child.StartPos,
					EndPos:   child.EndPos,
				}
				
				// Parse parameter children (identifier and type)
				for _, pChild := range child.Children {
					switch pChild.Type {
					case "identifier":
						param.Name = p.getNodeText(pChild)
					case "type":
						param.Type = p.convertType(pChild)
					}
				}
				
				if param.Name != "" && param.Type != nil {
					params = append(params, param)
				}
			}
		}
	}
	
	return params
}

func (p *Parser) convertReturnType(node *SExpNode) ast.Type {
	for _, child := range node.Children {
		if child.Type == "type" {
			return p.convertType(child)
		}
	}
	return nil
}

// convertReturnTypeWithError converts return_type that might have error enum (-> type ? ErrorEnum)
func (p *Parser) convertReturnTypeWithError(node *SExpNode) (ast.Type, ast.Type) {
	var returnType ast.Type
	var errorType ast.Type
	
	for _, child := range node.Children {
		switch child.Type {
		case "type":
			if returnType == nil {
				returnType = p.convertType(child)
			} else {
				// Second type is the error enum
				errorType = p.convertType(child)
			}
		case "type_identifier":
			// Error enum type
			errorType = &ast.TypeIdentifier{
				Name:     p.getNodeText(child),
				StartPos: child.StartPos,
				EndPos:   child.EndPos,
			}
		}
	}
	
	return returnType, errorType
}

func (p *Parser) convertStructDecl(node *SExpNode) *ast.StructDecl {
	structDecl := &ast.StructDecl{
		Fields:   []*ast.Field{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// First child should be the struct name
	if len(node.Children) > 0 && node.Children[0].Type == "identifier" {
		structDecl.Name = p.getNodeText(node.Children[0])
	}
	
	// Parse field declarations
	for _, child := range node.Children {
		if child.Type == "field_declaration" {
			field := &ast.Field{
				StartPos: child.StartPos,
				EndPos:   child.EndPos,
			}
			
			// Parse field name and type
			for i, fieldChild := range child.Children {
				if fieldChild.Type == "identifier" && i == 0 {
					field.Name = p.getNodeText(fieldChild)
				} else if fieldChild.Type == "type" {
					field.Type = p.convertType(fieldChild)
				}
			}
			
			if field.Name != "" && field.Type != nil {
				structDecl.Fields = append(structDecl.Fields, field)
			}
		}
	}
	
	return structDecl
}

func (p *Parser) convertArrayType(node *SExpNode) *ast.ArrayType {
	arrayType := &ast.ArrayType{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Array type format: (array_type (type ...) (expression ...))
	for _, child := range node.Children {
		switch child.Type {
		case "type":
			if len(child.Children) > 0 {
				arrayType.ElementType = p.convertTypeNode(child.Children[0])
			}
		case "expression":
			if len(child.Children) > 0 {
				arrayType.Size = p.convertExpressionNode(child.Children[0])
			}
		}
	}
	
	return arrayType
}

func (p *Parser) convertConstDecl(node *SExpNode) *ast.ConstDecl {
	constDecl := &ast.ConstDecl{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "visibility":
			if p.getNodeText(child) == "pub" {
				constDecl.IsPublic = true
			}
		case "identifier":
			constDecl.Name = p.getNodeText(child)
		case "type":
			constDecl.Type = p.convertType(child)
		case "expression":
			constDecl.Value = p.convertExpression(child)
		}
	}
	
	return constDecl
}

func (p *Parser) convertLuaBlock(node *SExpNode) *ast.LuaBlock {
	luaBlock := &ast.LuaBlock{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Find the lua_code_block child
	for _, child := range node.Children {
		if child.Type == "lua_code_block" {
			luaBlock.Code = p.getNodeText(child)
			break
		}
	}
	
	return luaBlock
}

func (p *Parser) convertMIRBlock(node *SExpNode) *ast.MIRBlock {
	mirBlock := &ast.MIRBlock{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Find the mir_block_content child
	for _, child := range node.Children {
		if child.Type == "mir_block_text" {
			mirBlock.Code = p.getNodeText(child)
			break
		}
	}
	
	return mirBlock
}

func (p *Parser) convertMinzBlock(node *SExpNode) *ast.MinzBlock {
	minzBlock := &ast.MinzBlock{
		Code:     []ast.Statement{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("DEBUG: convertMinzBlock with %d children\n", len(node.Children))
		for i, child := range node.Children {
			fmt.Printf("  Child %d: type=%s, text=%s\n", i, child.Type, child.Text)
		}
	}
	
	// Find the code child (either minz_block_content or minz_raw_block)
	for _, child := range node.Children {
		// Look for the code node directly
		if child.Type == "minz_raw_block" {
			// For now, store raw code as a string
			rawCode := p.getNodeText(child)
			// TODO: Parse and process the MinZ code
			minzBlock.RawCode = rawCode
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("DEBUG: MinzBlock raw code: %q\n", rawCode)
			}
		} else if child.Type == "minz_block_content" {
			// Process structured content
			for _, contentChild := range child.Children {
				if contentChild.Type == "minz_emit" {
					if stmt := p.convertMinzEmit(contentChild); stmt != nil {
						minzBlock.Code = append(minzBlock.Code, stmt)
					}
				} else if contentChild.Type == "statement" {
					if len(contentChild.Children) > 0 {
						if stmt := p.convertStatement(contentChild.Children[0]); stmt != nil {
							minzBlock.Code = append(minzBlock.Code, stmt)
						}
					}
				} else {
					// Try converting as a statement directly
					if stmt := p.convertStatement(contentChild); stmt != nil {
						minzBlock.Code = append(minzBlock.Code, stmt)
					}
				}
			}
		}
	}
	
	return minzBlock
}

func (p *Parser) convertMinzMetafunctionDecl(node *SExpNode) ast.Declaration {
	// @minz("template", args...)
	var template string
	var args []ast.Expression
	
	if debug {
		fmt.Printf("DEBUG: convertMinzMetafunctionDecl with %d children\n", len(node.Children))
		for i, child := range node.Children {
			fmt.Printf("  Child %d: type=%s, text=%s\n", i, child.Type, child.Text)
		}
	}
	
	// Look for children with field names
	for i := 0; i < len(node.Children)-1; i++ {
		if node.Children[i].Type == "atom" && strings.HasPrefix(node.Children[i].Text, "template:") {
			// Next child should be the string literal
			if i+1 < len(node.Children) && node.Children[i+1].Type == "string_literal" {
				// The string literal value is stored in the node itself, not its text
				template = p.getNodeText(node.Children[i+1])
				// Remove quotes
				template = strings.Trim(template, `"`)
			}
		}
	}
	
	// Create a MinzMetafunctionCall expression
	minzCall := &ast.MinzMetafunctionCall{
		Code:      template,
		Arguments: args,
		StartPos:  node.StartPos,
		EndPos:    node.EndPos,
	}
	
	// Wrap it in an ExpressionDecl for top-level processing
	return &ast.ExpressionDecl{
		Expression: minzCall,
		StartPos:   node.StartPos,
		EndPos:     node.EndPos,
	}
}

func (p *Parser) convertMinzEmit(node *SExpNode) *ast.MinzEmit {
	minzEmit := &ast.MinzEmit{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Find the expression child (the code to emit)
	for _, child := range node.Children {
		if child.Type == "expression" || child.Type == "string_literal" {
			minzEmit.Code = p.convertExpression(child)
			break
		}
	}
	
	return minzEmit
}

func (p *Parser) convertTargetBlock(node *SExpNode) ast.Statement {
	// Find the target string and block
	var target string
	var body *ast.BlockStmt
	
	for _, child := range node.Children {
		if child.Type == "string_literal" {
			// Parse the target string
			text := p.getNodeText(child)
			// Remove quotes if present
			if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
				target = text[1 : len(text)-1]
			}
		} else if child.Type == "block" {
			body = p.convertBlock(child)
		}
	}
	
	// Create and return TargetBlockStmt
	return &ast.TargetBlockStmt{
		Target:   target,
		Body:     body,
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
}

func (p *Parser) convertCompileTimeAsm(node *SExpNode) ast.Statement {
	// Extract assembly code from asm_content
	var asmCode string
	
	for _, child := range node.Children {
		if child.Type == "asm_content" {
			// Extract text from all asm_instruction children
			var codeLines []string
			for _, instChild := range child.Children {
				if instChild.Type == "asm_instruction" || instChild.Type == "comment" {
					line := p.getNodeText(instChild)
					if strings.TrimSpace(line) != "" {
						codeLines = append(codeLines, line)
					}
				}
			}
			asmCode = strings.Join(codeLines, "\n")
			break
		}
	}
	
	// Create AsmBlockStmt
	return &ast.AsmBlockStmt{
		Code:     asmCode,
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
}

func (p *Parser) convertEnumDecl(node *SExpNode) *ast.EnumDecl {
	enumDecl := &ast.EnumDecl{
		Variants: []string{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// First child should be the enum name
	if len(node.Children) > 0 && node.Children[0].Type == "identifier" {
		enumDecl.Name = p.getNodeText(node.Children[0])
	}
	
	// Parse enum variants  
	// In Z80, enums are just u8 values: 0, 1, 2, 3...
	for _, child := range node.Children {
		if child.Type == "enum_variant" {
			// Get variant name
			for _, varChild := range child.Children {
				if varChild.Type == "identifier" {
					variantName := p.getNodeText(varChild)
					if variantName != "" {
						enumDecl.Variants = append(enumDecl.Variants, variantName)
					}
					break
				}
			}
		}
	}
	
	return enumDecl
}

func (p *Parser) convertReturnStmt(node *SExpNode) *ast.ReturnStmt {
	ret := &ast.ReturnStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Look for expression child
	for _, child := range node.Children {
		if child.Type == "expression" {
			ret.Value = p.convertExpression(child)
			break
		}
	}
	
	return ret
}

func (p *Parser) convertIfStmt(node *SExpNode) *ast.IfStmt {
	ifStmt := &ast.IfStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Parse if statement components
	for _, child := range node.Children {
		switch child.Type {
		case "expression":
			// First expression is the condition
			if ifStmt.Condition == nil {
				ifStmt.Condition = p.convertExpression(child)
			}
		case "block":
			// First block is then, second is else
			if ifStmt.Then == nil {
				ifStmt.Then = p.convertBlock(child)
			} else {
				ifStmt.Else = p.convertBlock(child)
			}
		case "if_statement":
			// else if case
			ifStmt.Else = p.convertIfStmt(child)
		}
	}
	
	return ifStmt
}

func (p *Parser) convertWhileStmt(node *SExpNode) *ast.WhileStmt {
	whileStmt := &ast.WhileStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Parse while statement components
	for _, child := range node.Children {
		switch child.Type {
		case "expression":
			whileStmt.Condition = p.convertExpression(child)
		case "block":
			whileStmt.Body = p.convertBlock(child)
		}
	}
	
	return whileStmt
}

func (p *Parser) convertForStmt(node *SExpNode) *ast.ForStmt {
	forStmt := &ast.ForStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Debug output
	if false {  // Set to true for debugging
		fmt.Printf("convertForStmt: %d children\n", len(node.Children))
		for i, child := range node.Children {
			fmt.Printf("  [%d] %s: %s\n", i, child.Type, p.getNodeText(child))
		}
	}
	
	// Parse for statement components
	// The tree-sitter output doesn't include "for" and "in" as separate tokens
	// It goes: identifier, expression, block
	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			if forStmt.Iterator == "" {
				forStmt.Iterator = p.getNodeText(child)
			}
		case "expression":
			if forStmt.Range == nil {
				forStmt.Range = p.convertExpression(child)
			}
		case "block":
			forStmt.Body = p.convertBlock(child)
		}
	}
	
	return forStmt
}

func (p *Parser) convertExpressionStmt(node *SExpNode) ast.Statement {
	stmt := &ast.ExpressionStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	if debug {
		fmt.Printf("DEBUG: convertExpressionStmt with %d children\n", len(node.Children))
		for i, child := range node.Children {
			fmt.Printf("  Child %d: type=%s\n", i, child.Type)
		}
	}
	
	// Find the expression child
	for _, child := range node.Children {
		if child.Type == "expression" {
			stmt.Expression = p.convertExpression(child)
			if debug {
				fmt.Printf("DEBUG: Converted expression type: %T\n", stmt.Expression)
			}
			break
		}
	}
	
	if debug && stmt.Expression == nil {
		fmt.Printf("DEBUG: WARNING - ExpressionStmt has nil expression!\n")
	}
	
	return stmt
}

func (p *Parser) convertAssignmentStmt(node *SExpNode) *ast.AssignStmt {
	assignStmt := &ast.AssignStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Look for target and value
	for _, child := range node.Children {
		if child.Type == "expression" {
			if assignStmt.Target == nil {
				assignStmt.Target = p.convertExpression(child)
			} else {
				assignStmt.Value = p.convertExpression(child)
			}
		}
	}
	
	return assignStmt
}

func (p *Parser) convertLoopStmt(node *SExpNode) ast.Statement {
	// MinZ loop syntax: loop <array> into <var> { ... }
	//                   loop <array> ref to <var> { ... }
	loopStmt := &ast.LoopStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Extract the array expression and iterator from ERROR nodes
	var arrayExpr ast.Expression
	var mode ast.LoopMode = ast.LoopInto
	var iterator string
	var foundBlock bool
	
	for i, child := range node.Children {
		switch child.Type {
		case "ERROR":
			// The loop syntax is parsed as ERROR, extract identifiers
			errorText := p.getNodeText(child)
			if strings.Contains(errorText, "ref to") {
				mode = ast.LoopRefTo
			}
			// Extract identifiers from ERROR node
			for _, errChild := range child.Children {
				if errChild.Type == "identifier" {
					idText := p.getNodeText(errChild)
					if arrayExpr == nil && idText != "into" && idText != "ref" && idText != "to" {
						// First identifier is the array
						arrayExpr = &ast.Identifier{Name: idText, StartPos: errChild.StartPos, EndPos: errChild.EndPos}
					} else if iterator == "" && idText != "into" && idText != "ref" && idText != "to" {
						// Second identifier is the iterator
						iterator = idText
					}
				}
			}
		case "block":
			loopStmt.Body = p.convertBlock(child)
			foundBlock = true
		case "identifier":
			// Sometimes the array identifier appears directly
			if i == 0 {
				idText := p.getNodeText(child)
				arrayExpr = &ast.Identifier{Name: idText, StartPos: child.StartPos, EndPos: child.EndPos}
			}
		}
	}
	
	// If we couldn't parse the loop properly, return nil
	if arrayExpr == nil || iterator == "" || !foundBlock {
		return nil
	}
	
	loopStmt.Table = arrayExpr
	loopStmt.Mode = mode
	loopStmt.Iterator = iterator
	
	return loopStmt
}

func (p *Parser) convertArrayLiteral(node *SExpNode) *ast.ArrayInitializer {
	arr := &ast.ArrayInitializer{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
		Elements: make([]ast.Expression, 0),
	}
	
	// Parse array literal: [1, 2, 3]
	for _, child := range node.Children {
		if child.Type == "expression" {
			elem := p.convertExpression(child)
			if elem != nil {
				arr.Elements = append(arr.Elements, elem)
			}
		}
	}
	
	return arr
}

func (p *Parser) convertStructLiteral(node *SExpNode) *ast.StructLiteral {
	lit := &ast.StructLiteral{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
		Fields:   make([]*ast.FieldInit, 0),
	}
	
	// Parse struct literal: Point { x: 10, y: 20 }
	for _, child := range node.Children {
		switch child.Type {
		case "type_identifier":
			// Extract the struct name
			if len(child.Children) > 0 && child.Children[0].Type == "identifier" {
				lit.TypeName = p.getNodeText(child.Children[0])
			}
		case "field_initializer":
			// Parse field: name: value
			fieldInit := &ast.FieldInit{}
			for i, fieldChild := range child.Children {
				if fieldChild.Type == "identifier" && i == 0 {
					fieldInit.Name = p.getNodeText(fieldChild)
				} else if fieldChild.Type == "expression" {
					fieldInit.Value = p.convertExpression(fieldChild)
				}
			}
			if fieldInit.Name != "" && fieldInit.Value != nil {
				lit.Fields = append(lit.Fields, fieldInit)
			}
		}
	}
	
	return lit
}

func (p *Parser) convertPointerType(node *SExpNode) *ast.PointerType {
	pointerType := &ast.PointerType{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Pointer type format: (pointer_type (type ...))
	for _, child := range node.Children {
		if child.Type == "type" && len(child.Children) > 0 {
			pointerType.BaseType = p.convertTypeNode(child.Children[0])
			break
		}
	}
	
	return pointerType
}

func (p *Parser) convertBitStructType(node *SExpNode) *ast.BitStructType {
	bitStruct := &ast.BitStructType{
		Fields:   []*ast.BitField{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Determine underlying type from the keyword
	nodeText := p.getNodeText(node)
	if strings.HasPrefix(nodeText, "bits_16") {
		bitStruct.UnderlyingType = &ast.PrimitiveType{Name: "u16"}
	} else {
		// Default to u8 for both "bits" and "bits_8"
		bitStruct.UnderlyingType = &ast.PrimitiveType{Name: "u8"}
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "bit_field":
			field := &ast.BitField{
				StartPos: child.StartPos,
				EndPos:   child.EndPos,
			}
			
			for _, fieldChild := range child.Children {
				switch fieldChild.Type {
				case "identifier":
					field.Name = p.getNodeText(fieldChild)
				case "number_literal":
					// Parse bit width as integer
					val, _ := strconv.ParseInt(p.getNodeText(fieldChild), 0, 64)
					field.BitWidth = int(val)
				}
			}
			
			if field.Name != "" && field.BitWidth > 0 {
				bitStruct.Fields = append(bitStruct.Fields, field)
			}
		}
	}
	
	return bitStruct
}

func (p *Parser) convertTypeAlias(node *SExpNode) *ast.TypeDecl {
	typeDecl := &ast.TypeDecl{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			typeDecl.Name = p.getNodeText(child)
		case "type":
			if len(child.Children) > 0 {
				typeDecl.Type = p.convertTypeNode(child.Children[0])
			}
		}
	}
	
	return typeDecl
}

func (p *Parser) convertImportStmt(node *SExpNode) *ast.ImportStmt {
	imp := &ast.ImportStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Parse import path: import zx.screen as screen;
	// Tree-sitter produces: (import_statement (import_path ...) (identifier))
	// where the second identifier is the alias after "as"
	var gotPath bool
	for _, child := range node.Children {
		if child.Type == "import_path" {
			// Collect all identifiers to form the path
			var pathParts []string
			for _, pathChild := range child.Children {
				if pathChild.Type == "identifier" {
					pathParts = append(pathParts, p.getNodeText(pathChild))
				}
			}
			if len(pathParts) > 0 {
				imp.Path = strings.Join(pathParts, ".")
				gotPath = true
			}
		} else if child.Type == "identifier" && gotPath {
			// This is the alias identifier that appears after the import_path
			// Tree-sitter parses "import x as y" with the alias as a direct child
			imp.Alias = p.getNodeText(child)
		}
	}
	
	if os.Getenv("DEBUG") != "" && imp.Alias != "" {
		fmt.Printf("DEBUG: Parsed import %s as %s\n", imp.Path, imp.Alias)
	}
	
	return imp
}

// convertCaseStmt converts a case statement
func (p *Parser) convertCaseStmt(node *SExpNode) *ast.CaseStmt {
	caseStmt := &ast.CaseStmt{
		Arms:     []*ast.CaseArm{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "expression":
			if caseStmt.Expr == nil {
				caseStmt.Expr = p.convertExpression(child)
			}
		case "case_arm":
			if arm := p.convertCaseArm(child); arm != nil {
				caseStmt.Arms = append(caseStmt.Arms, arm)
			}
		}
	}
	
	return caseStmt
}

// convertCaseArm converts a case arm
func (p *Parser) convertCaseArm(node *SExpNode) *ast.CaseArm {
	arm := &ast.CaseArm{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "pattern":
			arm.Pattern = p.convertPattern(child)
		case "expression":
			arm.Body = p.convertExpression(child)
		case "block":
			arm.Body = p.convertBlock(child)
		}
	}
	
	return arm
}

// convertPattern converts a pattern
func (p *Parser) convertPattern(node *SExpNode) ast.Pattern {
	if len(node.Children) == 0 {
		// Empty pattern node is a wildcard (_)
		return &ast.WildcardPattern{
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	}
	
	child := node.Children[0]
	switch child.Type {
	case "_":
		return &ast.WildcardPattern{
			StartPos: child.StartPos,
			EndPos:   child.EndPos,
		}
	case "field_expression":
		// Parse field expression like Direction.North
		obj := ""
		field := ""
		for _, fc := range child.Children {
			if fc.Type == "postfix_expression" || fc.Type == "primary_expression" {
				// This is the object part (Direction)
				if obj == "" {
					obj = p.getNodeText(fc)
				}
			} else if fc.Type == "identifier" {
				// This is the field part (North)
				field = p.getNodeText(fc)
			}
		}
		return &ast.IdentifierPattern{
			Name:     obj + "." + field,
			StartPos: child.StartPos,
			EndPos:   child.EndPos,
		}
	case "identifier":
		return &ast.IdentifierPattern{
			Name:     p.getNodeText(child),
			StartPos: child.StartPos,
			EndPos:   child.EndPos,
		}
	case "literal_pattern":
		// Parse the nested literal
		if len(child.Children) > 0 {
			literal := child.Children[0]
			return &ast.LiteralPattern{
				Value:    p.convertExpression(literal),
				StartPos: child.StartPos,
				EndPos:   child.EndPos,
			}
		}
	}
	
	return nil
}

// convertInterfaceDecl converts interface declaration from S-expression
func (p *Parser) convertInterfaceDecl(node *SExpNode) *ast.InterfaceDecl {
	decl := &ast.InterfaceDecl{
		Methods:  []*ast.InterfaceMethod{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "visibility":
			if p.getNodeText(child) == "pub" {
				decl.IsPublic = true
			}
		case "identifier":
			if decl.Name == "" {
				decl.Name = p.getNodeText(child)
			}
		case "generic_parameters":
			decl.GenericParams = p.convertGenericParameters(child)
		case "interface_method":
			method := p.convertInterfaceMethod(child)
			if method != nil {
				decl.Methods = append(decl.Methods, method)
			}
		}
	}
	
	return decl
}

// convertInterfaceMethod converts interface method from S-expression
func (p *Parser) convertInterfaceMethod(node *SExpNode) *ast.InterfaceMethod {
	method := &ast.InterfaceMethod{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			if method.Name == "" {
				method.Name = p.getNodeText(child)
			}
		case "parameter_list":
			method.Params = p.convertParameters(child)
		case "return_type":
			method.ReturnType = p.convertReturnType(child)
		}
	}
	
	return method
}

// convertGenericParameters converts generic parameters from S-expression
func (p *Parser) convertGenericParameters(node *SExpNode) []*ast.GenericParam {
	var params []*ast.GenericParam
	
	for _, child := range node.Children {
		if child.Type == "generic_parameter" {
			param := p.convertGenericParameter(child)
			if param != nil {
				params = append(params, param)
			}
		}
	}
	
	return params
}

// convertGenericParameter converts a single generic parameter from S-expression
func (p *Parser) convertGenericParameter(node *SExpNode) *ast.GenericParam {
	param := &ast.GenericParam{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	var inBounds bool
	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			if !inBounds && param.Name == "" {
				param.Name = p.getNodeText(child)
			} else if inBounds {
				param.Bounds = append(param.Bounds, p.getNodeText(child))
			}
		case ":":
			inBounds = true
		}
	}
	
	return param
}

// convertImplBlock converts implementation block from S-expression
func (p *Parser) convertImplBlock(node *SExpNode) *ast.ImplBlock {
	impl := &ast.ImplBlock{
		Methods:  []*ast.FunctionDecl{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// The structure is: identifier (interface name), type (implementing type), function_declaration(s)
	for i, child := range node.Children {
		switch child.Type {
		case "identifier":
			if i == 0 && impl.InterfaceName == "" {
				impl.InterfaceName = p.getNodeText(child)
			}
		case "type":
			if i == 1 && impl.ForType == nil {
				impl.ForType = p.convertType(child)
			}
		case "function_declaration":
			method := p.convertFunction(child)
			if method != nil {
				impl.Methods = append(impl.Methods, method)
			}
		}
	}
	
	return impl
}

// convertLambdaExpr converts lambda expression from S-expression
func (p *Parser) convertLambdaExpr(node *SExpNode) *ast.LambdaExpr {
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("DEBUG: convertLambdaExpr called\n")
		fmt.Printf("  Node type: %s\n", node.Type)
		fmt.Printf("  Children count: %d\n", len(node.Children))
		for i, child := range node.Children {
			fmt.Printf("  Child %d: type=%s\n", i, child.Type)
		}
	}
	lambda := &ast.LambdaExpr{
		Params:   []*ast.LambdaParam{},
		Captures: []string{}, // Will be filled during semantic analysis
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
		case "lambda_parameter_list":
			lambda.Params = p.convertLambdaParams(child)
		case "type":
			// Return type from |x| -> u8 { ... }
			lambda.ReturnType = p.convertType(child)
		case "expression":
			// Body expression from |x| x + 1
			lambda.Body = p.convertExpression(child)
		case "block":
			// Body block from |x| { x + 1 }
			lambda.Body = p.convertBlock(child)
		}
	}
	
	return lambda
}

// convertLambdaParams converts lambda parameter list from S-expression
func (p *Parser) convertLambdaParams(node *SExpNode) []*ast.LambdaParam {
	var params []*ast.LambdaParam
	
	for _, child := range node.Children {
		if child.Type == "lambda_parameter" {
			param := &ast.LambdaParam{
				StartPos: child.StartPos,
				EndPos:   child.EndPos,
			}
			
			for _, paramChild := range child.Children {
				switch paramChild.Type {
				case "identifier":
					param.Name = p.getNodeText(paramChild)
				case "type":
					param.Type = p.convertType(paramChild)
				}
			}
			
			params = append(params, param)
		}
	}
	
	return params
}

// Use the isAlpha and isDigit functions from simple_parser.go

// convertAsmBlock converts an asm block statement to AST
func (p *Parser) convertAsmBlock(node *SExpNode) *ast.AsmStmt {
	stmt := &ast.AsmStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Find the raw content
	for _, child := range node.Children {
		if child.Type == "asm_raw_content" {
			stmt.Code = p.getNodeText(child)
			break
		} else if child.Type == "asm_content" {
			stmt.Code = p.getNodeText(child)
			break
		}
	}
	
	return stmt
}

// convertMirBlock converts a mir block statement to AST
func (p *Parser) convertMirBlock(node *SExpNode) *ast.MIRStmt {
	stmt := &ast.MIRStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Find the raw content
	for _, child := range node.Children {
		if child.Type == "mir_raw_content" {
			stmt.Code = p.getNodeText(child)
			break
		} else if child.Type == "mir_content" {
			stmt.Code = p.getNodeText(child)
			break
		}
	}
	
	return stmt
}

// convertAsmFunction converts an asm function to AST
func (p *Parser) convertAsmFunction(node *SExpNode) *ast.FunctionDecl {
	fn := &ast.FunctionDecl{
		FunctionKind: ast.FunctionKindAsm,
		StartPos:     node.StartPos,
		EndPos:       node.EndPos,
	}

	// Parse the function structure similarly to regular functions
	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			if fn.Name == "" {
				fn.Name = p.getNodeText(child)
			}
		case "parameter_list":
			fn.Params = p.convertParameters(child)
		case "return_type":
			fn.ReturnType = p.convertReturnType(child)
		case "asm_body":
			// For asm functions, we store the raw assembly in the body
			for _, bodyChild := range child.Children {
				if bodyChild.Type == "asm_raw_content" {
					// Create a pseudo-block with just the assembly text
					fn.Body = &ast.BlockStmt{
						Statements: []ast.Statement{
							&ast.AsmStmt{
								Code:     p.getNodeText(bodyChild),
								StartPos: bodyChild.StartPos,
								EndPos:   bodyChild.EndPos,
							},
						},
						StartPos: child.StartPos,
						EndPos:   child.EndPos,
					}
					break
				}
			}
		case "visibility":
			text := p.getNodeText(child)
			if text == "pub" || text == "public" {
				fn.IsPublic = true
			}
		}
	}

	return fn
}

// convertMirFunction converts a mir function to AST
func (p *Parser) convertMirFunction(node *SExpNode) *ast.FunctionDecl {
	fn := &ast.FunctionDecl{
		FunctionKind: ast.FunctionKindMIR,
		StartPos:     node.StartPos,
		EndPos:       node.EndPos,
	}

	// Parse the function structure similarly to regular functions
	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			if fn.Name == "" {
				fn.Name = p.getNodeText(child)
			}
		case "parameter_list":
			fn.Params = p.convertParameters(child)
		case "return_type":
			fn.ReturnType = p.convertReturnType(child)
		case "mir_body":
			// For mir functions, we store the raw MIR in the body
			for _, bodyChild := range child.Children {
				if bodyChild.Type == "mir_raw_content" {
					// Create a pseudo-block with MIR text
					fn.Body = &ast.BlockStmt{
						Statements: []ast.Statement{
							&ast.MIRStmt{
								Code:     p.getNodeText(bodyChild),
								StartPos: bodyChild.StartPos,
								EndPos:   bodyChild.EndPos,
							},
						},
						StartPos: child.StartPos,
						EndPos:   child.EndPos,
					}
					break
				}
			}
		case "visibility":
			text := p.getNodeText(child)
			if text == "pub" || text == "public" {
				fn.IsPublic = true
			}
		}
	}

	return fn
}

// unescapeString processes escape sequences in a string
func unescapeString(s string) string {
	var result []rune
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result = append(result, '\n')
				i += 2
			case 't':
				result = append(result, '\t')
				i += 2
			case 'r':
				result = append(result, '\r')
				i += 2
			case '\\':
				result = append(result, '\\')
				i += 2
			case '"':
				result = append(result, '"')
				i += 2
			case '\'':
				result = append(result, '\'')
				i += 2
			case '0':
				result = append(result, '\x00')
				i += 2
			default:
				// Unknown escape, keep the backslash
				result = append(result, rune(s[i]))
				i++
			}
		} else {
			result = append(result, rune(s[i]))
			i++
		}
	}
	return string(result)
}

// transformSExpToIteratorChain transforms an iterator method call into an IteratorChainExpr
func (p *Parser) transformSExpToIteratorChain(fieldExpr *ast.FieldExpr, args []ast.Expression) ast.Expression {
	var chain *ast.IteratorChainExpr
	
	// Check if the object is already a chain or a call that should be a chain
	if existingChain, ok := fieldExpr.Object.(*ast.IteratorChainExpr); ok {
		chain = existingChain
	} else if callExpr, ok := fieldExpr.Object.(*ast.CallExpr); ok {
		// Check if this call is an iterator method
		if innerField, ok := callExpr.Function.(*ast.FieldExpr); ok {
			if p.isIteratorMethod(innerField.Field) {
				// Create a chain starting from the inner iterator
				chain = &ast.IteratorChainExpr{
					Source:     innerField.Object,
					Operations: []ast.IteratorOp{},
					StartPos:   innerField.Object.Pos(),
				}
				// Add the inner operation if it's not just "iter"
				if innerField.Field != "iter" {
					// TODO: Get the arguments from the inner call
				}
			}
		}
	}
	
	if chain == nil {
		// Start a new chain
		chain = &ast.IteratorChainExpr{
			Source:     fieldExpr.Object,
			Operations: []ast.IteratorOp{},
			StartPos:   fieldExpr.Object.Pos(),
		}
	}
	
	// Create the iterator operation
	var opType ast.IteratorOpType
	var function ast.Expression
	
	switch fieldExpr.Field {
	case "iter":
		// iter() starts the chain but doesn't add an operation
		return chain
		
	case "map":
		opType = ast.IterOpMap
		if len(args) > 0 {
			function = args[0]
		}
		
	case "filter":
		opType = ast.IterOpFilter
		if len(args) > 0 {
			function = args[0]
		}
		
	case "forEach":
		opType = ast.IterOpForEach
		if len(args) > 0 {
			function = args[0]
		}
		
	case "reduce":
		opType = ast.IterOpReduce
		if len(args) > 0 {
			function = args[0]
		}
		
	case "collect":
		opType = ast.IterOpCollect
		
	case "take":
		opType = ast.IterOpTake
		if len(args) > 0 {
			function = args[0]
		}
		
	case "skip":
		opType = ast.IterOpSkip
		if len(args) > 0 {
			function = args[0]
		}
		
	case "zip":
		opType = ast.IterOpZip
		if len(args) > 0 {
			function = args[0]
		}
	}
	
	// Add the operation to the chain
	chain.Operations = append(chain.Operations, ast.IteratorOp{
		Type:     opType,
		Function: function,
		StartPos: fieldExpr.StartPos,
		EndPos:   fieldExpr.EndPos,
	})
	
	chain.EndPos = fieldExpr.EndPos
	return chain
}

// convertAttributeAsExpression converts attribute nodes in expression context to metafunction calls
func (p *Parser) convertAttributeAsExpression(node *SExpNode) ast.Expression {
	var identifier string
	var arguments []ast.Expression
	
	// Parse the attribute structure: identifier + argument_list
	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			identifier = p.getNodeText(child)
		case "argument_list":
			// Parse arguments from the argument list
			for _, argChild := range child.Children {
				if argChild.Type == "expression" {
					expr := p.convertExpression(argChild)
					if expr != nil {
						arguments = append(arguments, expr)
					}
				}
			}
		}
	}
	
	// Handle special metafunctions
	if identifier == "minz" {
		return p.convertMinzMetafunction(arguments, node.StartPos, node.EndPos)
	}
	
	// Create generic metafunction call
	return &ast.MetafunctionCall{
		Name:      identifier,
		Arguments: arguments,
		StartPos:  node.StartPos,
		EndPos:    node.EndPos,
	}
}

// convertMinzMetafunction converts @minz("code", args...) calls to MinzMetafunctionCall AST nodes
func (p *Parser) convertMinzMetafunction(arguments []ast.Expression, startPos, endPos ast.Position) ast.Expression {
	// @minz requires at least one argument (the code string)
	if len(arguments) == 0 {
		return nil // Error: @minz requires code string
	}
	
	// First argument must be a string literal containing MinZ code
	codeExpr, ok := arguments[0].(*ast.StringLiteral)
	if !ok {
		return nil // Error: @minz first argument must be string literal
	}
	
	// Extract the MinZ code from the string literal
	code := codeExpr.Value
	
	// Remaining arguments are passed to the MinZ code
	var args []ast.Expression
	if len(arguments) > 1 {
		args = arguments[1:]
	}
	
	return &ast.MinzMetafunctionCall{
		Code:      code,
		Arguments: args,
		StartPos:  startPos,
		EndPos:    endPos,
	}
}

// Convert if expression (if cond { val1 } else { val2 })
func (p *Parser) convertIfExpr(node *SExpNode) ast.Expression {
	ifExpr := &ast.IfExpr{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Parse condition, then_branch, else_branch from tree-sitter fields
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				switch fieldName {
				case "condition":
					if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						ifExpr.Condition = p.convertExpressionNode(fieldValue.Children[0])
					}
				case "then_branch":
					if fieldValue.Type == "block" {
						// Convert block to expression
						ifExpr.ThenBranch = p.convertBlockAsExpression(fieldValue)
					}
				case "else_branch":
					if fieldValue.Type == "block" {
						ifExpr.ElseBranch = p.convertBlockAsExpression(fieldValue)
					} else if fieldValue.Type == "if_expression" {
						ifExpr.ElseBranch = p.convertIfExpr(fieldValue)
					}
				}
				i++ // Skip field value
			}
		}
	}
	
	return ifExpr
}

// Convert ternary expression (value_if_true if condition else value_if_false)
func (p *Parser) convertTernaryExpr(node *SExpNode) ast.Expression {
	ternary := &ast.TernaryExpr{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Parse true_expr, condition, false_expr from tree-sitter fields
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				switch fieldName {
				case "true_expr":
					if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						ternary.TrueExpr = p.convertExpressionNode(fieldValue.Children[0])
					}
				case "condition":
					if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						ternary.Condition = p.convertExpressionNode(fieldValue.Children[0])
					}
				case "false_expr":
					if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						ternary.FalseExpr = p.convertExpressionNode(fieldValue.Children[0])
					}
				}
				i++ // Skip field value
			}
		}
	}
	
	return ternary
}

// Convert when expression (pattern matching)
func (p *Parser) convertWhenExpr(node *SExpNode) ast.Expression {
	whenExpr := &ast.WhenExpr{
		Arms:     []*ast.WhenArm{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Parse optional value and arms
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				if fieldName == "value" && fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
					whenExpr.Value = p.convertExpressionNode(fieldValue.Children[0])
				}
				i++ // Skip field value
			}
		} else if child.Type == "when_arm" {
			arm := p.convertWhenArm(child)
			if arm != nil {
				whenExpr.Arms = append(whenExpr.Arms, arm)
			}
		}
	}
	
	return whenExpr
}

// Convert when arm
func (p *Parser) convertWhenArm(node *SExpNode) *ast.WhenArm {
	arm := &ast.WhenArm{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Parse pattern, guard, body from tree-sitter fields
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" {
			if child.Text == "else" {
				arm.IsElse = true
			} else if strings.HasSuffix(child.Text, ":") {
				fieldName := strings.TrimSuffix(child.Text, ":")
				if i+1 < len(node.Children) {
					fieldValue := node.Children[i+1]
					switch fieldName {
					case "pattern":
						if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
							arm.Pattern = p.convertExpressionNode(fieldValue.Children[0])
						}
					case "guard":
						if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
							arm.Guard = p.convertExpressionNode(fieldValue.Children[0])
						}
					case "body":
						if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
							arm.Body = p.convertExpressionNode(fieldValue.Children[0])
						}
					}
					i++ // Skip field value
				}
			}
		}
	}
	
	return arm
}

// Convert block to expression (used by if expressions)
func (p *Parser) convertBlockAsExpression(node *SExpNode) ast.Expression {
	// For now, return the block itself - semantic analyzer will handle conversion
	return &ast.BlockStmt{
		Statements: []ast.Statement{}, // Will be filled by block conversion
		StartPos:   node.StartPos,
		EndPos:     node.EndPos,
	}
}

// Convert try expression (? operator for error propagation)
func (p *Parser) convertTryExpr(node *SExpNode) ast.Expression {
	tryExpr := &ast.TryExpr{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Debug: print the structure of the try expression
	if debug {
		fmt.Printf("DEBUG: convertTryExpr - node.Type=%s, children=%d\n", node.Type, len(node.Children))
		for i, child := range node.Children {
			fmt.Printf("  Child %d: Type=%s, Text=%s\n", i, child.Type, child.Text)
		}
	}
	
	// Look for the expression field
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				if debug {
					fmt.Printf("DEBUG: convertTryExpr field %s: Type=%s\n", fieldName, fieldValue.Type)
				}
				if fieldName == "expression" {
					if fieldValue.Type == "postfix_expression" && len(fieldValue.Children) > 0 {
						tryExpr.Expression = p.convertExpressionNode(fieldValue.Children[0])
					} else if fieldValue.Type == "expression" && len(fieldValue.Children) > 0 {
						tryExpr.Expression = p.convertExpressionNode(fieldValue.Children[0])
					}
				}
				i++ // Skip field value
			}
		}
	}
	
	if debug {
		fmt.Printf("DEBUG: convertTryExpr result: Expression=%T\n", tryExpr.Expression)
	}
	
	return tryExpr
}

// convertDefineTemplate converts @define template nodes
func (p *Parser) convertDefineTemplate(node *SExpNode) *ast.DefineTemplate {
	template := &ast.DefineTemplate{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	if debug {
		fmt.Printf("DEBUG: convertDefineTemplate node has %d children\n", len(node.Children))
		for i, child := range node.Children {
			fmt.Printf("  Child %d: Type=%s, Text=%q\n", i, child.Type, child.Text)
		}
	}
	
	for i := 0; i < len(node.Children)-1; i++ {
		child := node.Children[i]
		// Check if this is a field label (Type=atom, Text ends with :)
		if child.Type == "atom" && strings.HasSuffix(child.Text, ":") {
			fieldName := strings.TrimSuffix(child.Text, ":")
			fieldValue := node.Children[i+1]
			
			switch fieldName {
				case "parameters":
					// Extract identifier list
					if fieldValue.Type == "identifier_list" {
						for _, param := range fieldValue.Children {
							if param.Type == "identifier" {
								template.Parameters = append(template.Parameters, p.getNodeText(param))
							}
						}
					}
				case "arguments":
					// Extract expression list
					if fieldValue.Type == "expression_list" {
						if debug {
							fmt.Printf("DEBUG: Found expression_list with %d children\n", len(fieldValue.Children))
						}
						for _, arg := range fieldValue.Children {
							if debug {
								fmt.Printf("DEBUG: Processing arg type=%s\n", arg.Type)
							}
							expr := p.convertExpressionNode(arg)
							if debug {
								fmt.Printf("DEBUG: Converted expr=%v (type=%T)\n", expr, expr)
							}
							if expr != nil {
								template.Arguments = append(template.Arguments, expr)
							}
						}
					}
				case "body":
					// Extract template body
					if fieldValue.Type == "template_body" {
						template.Body = p.getNodeText(fieldValue)
					}
			}
			
			i++ // Skip the field value in next iteration
		}
	}
	
	return template
}

// convertMetaExecutionBlock converts generic @lang[[[]]] blocks
func (p *Parser) convertMetaExecutionBlock(node *SExpNode) ast.Declaration {
	// Determine the specific type based on child nodes
	for _, child := range node.Children {
		switch child.Type {
		case "lua_execution_block":
			return p.convertLuaExecutionBlock(child)
		case "minz_execution_block":
			return p.convertMinzExecutionBlock(child)
		case "mir_execution_block":
			return p.convertMIRExecutionBlock(child)
		}
	}
	return nil
}

// convertLuaExecutionBlock converts @lua[[[]]] blocks
func (p *Parser) convertLuaExecutionBlock(node *SExpNode) *ast.MetaExecutionBlock {
	block := &ast.MetaExecutionBlock{
		Language: "lua",
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for i, child := range node.Children {
		if child.Text == "field:" && child.Type == "code" {
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				if fieldValue.Type == "raw_block_content" {
					block.Code = p.getNodeText(fieldValue)
				}
			}
		}
	}
	
	return block
}

// convertMinzExecutionBlock converts @minz[[[]]] blocks
func (p *Parser) convertMinzExecutionBlock(node *SExpNode) *ast.MetaExecutionBlock {
	block := &ast.MetaExecutionBlock{
		Language: "minz",
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for i, child := range node.Children {
		if child.Text == "field:" && child.Type == "code" {
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				if fieldValue.Type == "raw_block_content" {
					block.Code = p.getNodeText(fieldValue)
				}
			}
		}
	}
	
	return block
}

// convertMIRExecutionBlock converts @mir[[[]]] blocks
func (p *Parser) convertMIRExecutionBlock(node *SExpNode) *ast.MetaExecutionBlock {
	block := &ast.MetaExecutionBlock{
		Language: "mir",
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for i, child := range node.Children {
		if child.Text == "field:" && child.Type == "code" {
			if i+1 < len(node.Children) {
				fieldValue := node.Children[i+1]
				if fieldValue.Type == "raw_block_content" {
					block.Code = p.getNodeText(fieldValue)
				}
			}
		}
	}
	
	return block
}