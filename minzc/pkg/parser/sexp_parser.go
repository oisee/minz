package parser

import (
	"fmt"
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
	if sexp.Type != "source_file" {
		return nil, fmt.Errorf("expected source_file, got %s", sexp.Type)
	}
	
	file := &ast.File{
		Name:         filename,
		Imports:      []*ast.ImportStmt{},
		Declarations: []ast.Declaration{},
	}
	
	for _, child := range sexp.Children {
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
	case "variable_declaration":
		return p.convertVarDecl(node)
	case "struct_declaration":
		return p.convertStructDecl(node)
	case "enum_declaration":
		return p.convertEnumDecl(node)
	case "constant_declaration":
		return p.convertConstDecl(node)
	}
	return nil
}

func (p *Parser) convertFunction(node *SExpNode) *ast.FunctionDecl {
	fn := &ast.FunctionDecl{
		Params:   []*ast.Parameter{},
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
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
		case "block":
			fn.Body = p.convertBlock(child)
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
		}
	}
	
	return block
}

func (p *Parser) convertStatement(node *SExpNode) ast.Statement {
	switch node.Type {
	case "variable_declaration":
		return p.convertVarDecl(node)
	case "return_statement":
		return p.convertReturnStmt(node)
	case "if_statement":
		return p.convertIfStmt(node)
	case "while_statement":
		return p.convertWhileStmt(node)
	case "expression_statement":
		return p.convertExpressionStmt(node)
	case "assignment_statement":
		return p.convertAssignmentStmt(node)
	case "loop_statement":
		return p.convertLoopStmt(node)
	}
	return nil
}

func (p *Parser) convertVarDecl(node *SExpNode) *ast.VarDecl {
	varDecl := &ast.VarDecl{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	for _, child := range node.Children {
		switch child.Type {
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
	if len(node.Children) > 0 {
		return p.convertExpressionNode(node.Children[0])
	}
	return nil
}

func (p *Parser) convertExpressionNode(node *SExpNode) ast.Expression {
	switch node.Type {
	case "postfix_expression":
		if len(node.Children) > 0 {
			return p.convertExpressionNode(node.Children[0])
		}
	case "primary_expression":
		if len(node.Children) > 0 {
			return p.convertExpressionNode(node.Children[0])
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
		// Remove quotes if present
		if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
			text = text[1 : len(text)-1]
		}
		return &ast.StringLiteral{
			Value:    text,
			StartPos: node.StartPos,
			EndPos:   node.EndPos,
		}
	case "struct_literal":
		return p.convertStructLiteral(node)
	case "binary_expression":
		return p.convertBinaryExpr(node)
	case "call_expression":
		return p.convertCallExpr(node)
	case "index_expression":
		return p.convertIndexExpr(node)
	case "field_expression":
		return p.convertFieldExpr(node)
	case "cast_expression":
		// Convert cast to binary expression with "as" operator
		return p.convertCastToBinary(node)
	}
	return nil
}

func (p *Parser) convertBinaryExpr(node *SExpNode) *ast.BinaryExpr {
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
			}
		}
	}
	
	return binExpr
}

func (p *Parser) convertCallExpr(node *SExpNode) *ast.CallExpr {
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

func (p *Parser) convertCastToBinary(node *SExpNode) *ast.BinaryExpr {
	binExpr := &ast.BinaryExpr{
		Operator: "as",
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
						binExpr.Left = p.convertExpressionNode(fieldValue.Children[0])
					}
				case "type":
					// For "as" operator, right side is a type identifier
					if fieldValue.Type == "type" && len(fieldValue.Children) > 0 {
						typeNode := fieldValue.Children[0]
						binExpr.Right = &ast.Identifier{
							Name: p.getNodeText(typeNode),
							StartPos: typeNode.StartPos,
							EndPos: typeNode.EndPos,
						}
					}
				}
				i++
			}
		}
	}
	
	return binExpr
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

func (p *Parser) convertExpressionStmt(node *SExpNode) ast.Statement {
	// Expression statements might contain assignments or other expressions
	// For now, just return nil to avoid crashes
	// TODO: Properly handle expression statements
	return nil
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

func (p *Parser) convertImportStmt(node *SExpNode) *ast.ImportStmt {
	imp := &ast.ImportStmt{
		StartPos: node.StartPos,
		EndPos:   node.EndPos,
	}
	
	// Parse import path: import zx.screen;
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
			}
		}
	}
	
	return imp
}

// Use the isAlpha and isDigit functions from simple_parser.go