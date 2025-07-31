package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/ast"
)

// Parser handles parsing MinZ source files using tree-sitter
type Parser struct {
	treeSitterPath string
	sourceCode     string
}

// New creates a new parser
func New() *Parser {
	return &Parser{}
}

// ParseFile parses a MinZ source file and returns an AST
func (p *Parser) ParseFile(filename string) (*ast.File, error) {
	// Try tree-sitter first
	sexpAST, err := p.parseToSExp(filename)
	if err != nil {
		// Fall back to simple parser if tree-sitter fails
		// This is especially useful for new language features like @lua
		simpleParser := NewSimpleParser()
		return simpleParser.ParseFile(filename)
	}

	// Convert the S-expression AST to our Go AST
	// Create a new Parser instance for S-expression conversion with source code
	sexpParser := &Parser{sourceCode: p.sourceCode}
	return sexpParser.convertSExpToAST(filename, sexpAST)
}

// parseToSExp uses tree-sitter to parse the file and output S-expression
func (p *Parser) parseToSExp(filename string) (*SExpNode, error) {
	// Read the source file
	sourceCode, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	p.sourceCode = string(sourceCode)
	
	// Get the absolute path to the tree-sitter grammar
	// Find the directory containing grammar.js
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	
	// Look for grammar.js in current directory and parent directories
	grammarPath := currentDir
	for {
		if _, err := os.Stat(filepath.Join(grammarPath, "grammar.js")); err == nil {
			break
		}
		parent := filepath.Dir(grammarPath)
		if parent == grammarPath {
			// Reached root without finding grammar.js
			return nil, fmt.Errorf("could not find grammar.js in any parent directory")
		}
		grammarPath = parent
	}
	
	// Get absolute path to the file
	absFilename, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Run tree-sitter parse command directly (not through npx)
	cmd := exec.Command("tree-sitter", "parse", absFilename)
	cmd.Dir = grammarPath
	
	output, err := cmd.CombinedOutput()
	if len(output) > 0 && string(output) == "No language found\n" {
		return nil, fmt.Errorf("tree-sitter: no language found - wrong directory?")
	}
	// Tree-sitter may return non-zero exit code for parse errors but still output S-expression
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("failed to run tree-sitter: %w", err)
	}

	// Parse the S-expression output
	// Remove the statistics line at the end
	outputStr := string(output)
	
	lines := strings.Split(outputStr, "\n")
	var sexpLines []string
	for _, line := range lines {
		// Skip empty lines and the statistics line
		if line != "" && !strings.Contains(line, "\tParse:") {
			sexpLines = append(sexpLines, line)
		}
	}
	sexpOutput := strings.Join(sexpLines, "\n")
	
	if sexpOutput == "" {
		return nil, fmt.Errorf("empty S-expression output from tree-sitter")
	}
	
	return parseSExpression(sexpOutput)
}

// parseToJSON uses tree-sitter to parse the file and output JSON
func (p *Parser) parseToJSON(filename string) (map[string]interface{}, error) {
	// Get the absolute path to the tree-sitter grammar
	grammarPath, err := filepath.Abs(".")
	if err != nil {
		return nil, err
	}

	// Run tree-sitter parse command directly (not through npx)
	cmd := exec.Command("tree-sitter", "parse", filename, "--json")
	cmd.Dir = grammarPath
	
	output, err := cmd.CombinedOutput()
	// Tree-sitter may return non-zero exit code for parse errors but still output JSON
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("failed to run tree-sitter: %w", err)
	}

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	return result, nil
}

// jsonToAST converts the tree-sitter JSON output to our AST
func (p *Parser) jsonToAST(filename string, jsonAST map[string]interface{}) (*ast.File, error) {
	file := &ast.File{
		Name:         filename,
		Imports:      []*ast.ImportStmt{},
		Declarations: []ast.Declaration{},
	}

	// Extract the root node
	root, ok := jsonAST["rootNode"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid AST structure: missing rootNode")
	}

	// Process children
	children, ok := root["children"].([]interface{})
	if !ok {
		return file, nil // Empty file
	}

	for _, child := range children {
		childNode, ok := child.(map[string]interface{})
		if !ok {
			continue
		}

		nodeType, _ := childNode["type"].(string)
		switch nodeType {
		case "import_statement":
			if imp := p.parseImport(childNode); imp != nil {
				file.Imports = append(file.Imports, imp)
			}
		case "function_declaration":
			if fn := p.parseFunction(childNode); fn != nil {
				file.Declarations = append(file.Declarations, fn)
			}
		case "variable_declaration":
			if varDecl := p.parseVarDecl(childNode); varDecl != nil {
				file.Declarations = append(file.Declarations, varDecl)
			}
		case "struct_declaration":
			if structDecl := p.parseStructDecl(childNode); structDecl != nil {
				file.Declarations = append(file.Declarations, structDecl)
			}
		case "enum_declaration":
			if enumDecl := p.parseEnumDecl(childNode); enumDecl != nil {
				file.Declarations = append(file.Declarations, enumDecl)
			}
		case "constant_declaration":
			if constDecl := p.parseConstDecl(childNode); constDecl != nil {
				file.Declarations = append(file.Declarations, constDecl)
			}
		case "lua_block":
			if luaBlock := p.parseLuaBlock(childNode); luaBlock != nil {
				// Lua blocks are statements that get processed at compile time
				// For now, we'll track them separately
			}
		case "lua_eval":
			if luaEval := p.parseLuaEval(childNode); luaEval != nil {
				// Lua eval generates code, so it acts like a declaration
				// For now, we'll need special handling in semantic analysis
			}
		}
	}

	return file, nil
}

// parseImport parses an import statement
func (p *Parser) parseImport(node map[string]interface{}) *ast.ImportStmt {
	imp := &ast.ImportStmt{}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		if nodeType == "import_path" {
			// Handle import paths that may have dots (e.g., zx.screen)
			pathText := p.getText(childNode)
			if pathText == "" {
				// If no text, try to reconstruct from children
				pathText = p.reconstructImportPath(childNode)
			}
			imp.Path = pathText
		} else if nodeType == "identifier" {
			imp.Alias = p.getText(childNode)
		}
	}
	
	imp.StartPos = p.getPosition(node, "startPosition")
	imp.EndPos = p.getPosition(node, "endPosition")
	
	return imp
}

// parseFunction parses a function declaration
func (p *Parser) parseFunction(node map[string]interface{}) *ast.FunctionDecl {
	fn := &ast.FunctionDecl{
		Params: []*ast.Parameter{},
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "visibility":
			if p.getText(childNode) == "pub" {
				fn.IsPublic = true
			}
		case "export":
			fn.IsExport = true
		case "identifier":
			if fn.Name == "" {
				fn.Name = p.getText(childNode)
			}
		case "parameter_list":
			fn.Params = p.parseParameters(childNode)
		case "return_type":
			fn.ReturnType = p.parseType(childNode)
		case "block":
			fn.Body = p.parseBlock(childNode)
		}
	}
	
	fn.StartPos = p.getPosition(node, "startPosition")
	fn.EndPos = p.getPosition(node, "endPosition")
	
	return fn
}

// parseVarDecl parses a variable declaration
func (p *Parser) parseVarDecl(node map[string]interface{}) *ast.VarDecl {
	varDecl := &ast.VarDecl{}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		text := p.getText(childNode)
		
		switch nodeType {
		case "visibility":
			if text == "pub" {
				varDecl.IsPublic = true
			}
		case "var":
			varDecl.IsMutable = true
		case "let":
			varDecl.IsMutable = true  // let variables are mutable in MinZ
		case "identifier":
			if varDecl.Name == "" {
				varDecl.Name = text
			}
		case "type":
			varDecl.Type = p.parseType(childNode)
		case "expression":
			varDecl.Value = p.parseExpression(childNode)
		}
	}
	
	varDecl.StartPos = p.getPosition(node, "startPosition")
	varDecl.EndPos = p.getPosition(node, "endPosition")
	
	return varDecl
}

// parseStructDecl parses a struct declaration
func (p *Parser) parseStructDecl(node map[string]interface{}) *ast.StructDecl {
	structDecl := &ast.StructDecl{
		Fields: []*ast.Field{},
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "visibility":
			if p.getText(childNode) == "pub" {
				structDecl.IsPublic = true
			}
		case "identifier":
			if structDecl.Name == "" {
				structDecl.Name = p.getText(childNode)
			}
		case "field_declaration":
			if field := p.parseField(childNode); field != nil {
				structDecl.Fields = append(structDecl.Fields, field)
			}
		}
	}
	
	structDecl.StartPos = p.getPosition(node, "startPosition")
	structDecl.EndPos = p.getPosition(node, "endPosition")
	
	return structDecl
}

// parseConstDecl parses a constant declaration
func (p *Parser) parseConstDecl(node map[string]interface{}) *ast.ConstDecl {
	constDecl := &ast.ConstDecl{}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "visibility":
			if p.getText(childNode) == "pub" {
				constDecl.IsPublic = true
			}
		case "identifier":
			constDecl.Name = p.getText(childNode)
		case "type":
			constDecl.Type = p.parseType(childNode)
		case "expression":
			constDecl.Value = p.parseExpression(childNode)
		}
	}
	
	constDecl.StartPos = p.getPosition(node, "startPosition")
	constDecl.EndPos = p.getPosition(node, "endPosition")
	
	return constDecl
}

// parseEnumDecl parses an enum declaration
func (p *Parser) parseEnumDecl(node map[string]interface{}) *ast.EnumDecl {
	enumDecl := &ast.EnumDecl{
		Variants: []string{},
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "visibility":
			if p.getText(childNode) == "pub" {
				enumDecl.IsPublic = true
			}
		case "identifier":
			if enumDecl.Name == "" {
				enumDecl.Name = p.getText(childNode)
			}
		case "enum_variant":
			variant := p.getText(childNode)
			if variant != "" {
				enumDecl.Variants = append(enumDecl.Variants, variant)
			}
		}
	}
	
	enumDecl.StartPos = p.getPosition(node, "startPosition")
	enumDecl.EndPos = p.getPosition(node, "endPosition")
	
	return enumDecl
}

// parseField parses a struct field
func (p *Parser) parseField(node map[string]interface{}) *ast.Field {
	field := &ast.Field{}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "visibility":
			if p.getText(childNode) == "pub" {
				field.IsPublic = true
			}
		case "identifier":
			field.Name = p.getText(childNode)
		case "type":
			field.Type = p.parseType(childNode)
		}
	}
	
	field.StartPos = p.getPosition(node, "startPosition")
	field.EndPos = p.getPosition(node, "endPosition")
	
	return field
}

// parseParameters parses a parameter list
func (p *Parser) parseParameters(node map[string]interface{}) []*ast.Parameter {
	params := []*ast.Parameter{}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		if childNode["type"] == "parameter" {
			param := p.parseParameter(childNode)
			if param != nil {
				params = append(params, param)
			}
		}
	}
	
	return params
}

// parseParameter parses a single parameter
func (p *Parser) parseParameter(node map[string]interface{}) *ast.Parameter {
	param := &ast.Parameter{}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "identifier":
			param.Name = p.getText(childNode)
		case "type":
			param.Type = p.parseType(childNode)
		}
	}
	
	param.StartPos = p.getPosition(node, "startPosition")
	param.EndPos = p.getPosition(node, "endPosition")
	
	return param
}

// parseType parses a type node
func (p *Parser) parseType(node map[string]interface{}) ast.Type {
	if node == nil {
		return nil
	}
	
	// Handle return_type wrapper
	if node["type"] == "return_type" {
		children, _ := node["children"].([]interface{})
		for _, child := range children {
			childNode, _ := child.(map[string]interface{})
			if childNode["type"] != "->" {
				return p.parseType(childNode)
			}
		}
	}
	
	nodeType, _ := node["type"].(string)
	switch nodeType {
	case "primitive_type":
		return &ast.PrimitiveType{
			Name:     p.getText(node),
			StartPos: p.getPosition(node, "startPosition"),
			EndPos:   p.getPosition(node, "endPosition"),
		}
	case "array_type":
		return p.parseArrayType(node)
	case "pointer_type":
		return p.parsePointerType(node)
	case "type_identifier":
		// For now, treat type identifiers as primitive types
		return &ast.PrimitiveType{
			Name:     p.getText(node),
			StartPos: p.getPosition(node, "startPosition"),
			EndPos:   p.getPosition(node, "endPosition"),
		}
	}
	
	return nil
}

// parseArrayType parses an array type
func (p *Parser) parseArrayType(node map[string]interface{}) *ast.ArrayType {
	arrayType := &ast.ArrayType{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "type":
			arrayType.ElementType = p.parseType(childNode)
		case "expression":
			arrayType.Size = p.parseExpression(childNode)
		}
	}
	
	return arrayType
}

// parsePointerType parses a pointer type
func (p *Parser) parsePointerType(node map[string]interface{}) *ast.PointerType {
	ptrType := &ast.PointerType{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		text := p.getText(childNode)
		
		switch nodeType {
		case "mut":
			ptrType.IsMutable = true
		case "type":
			ptrType.BaseType = p.parseType(childNode)
		}
		
		if text == "mut" {
			ptrType.IsMutable = true
		}
	}
	
	return ptrType
}

// parseBlock parses a block statement
func (p *Parser) parseBlock(node map[string]interface{}) *ast.BlockStmt {
	block := &ast.BlockStmt{
		Statements: []ast.Statement{},
		StartPos:   p.getPosition(node, "startPosition"),
		EndPos:     p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		stmt := p.parseStatement(childNode)
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
	}
	
	return block
}

// parseStatement parses a statement
func (p *Parser) parseStatement(node map[string]interface{}) ast.Statement {
	if node == nil {
		return nil
	}
	
	nodeType, _ := node["type"].(string)
	switch nodeType {
	case "return_statement":
		return p.parseReturnStmt(node)
	case "if_statement":
		return p.parseIfStmt(node)
	case "while_statement":
		return p.parseWhileStmt(node)
	case "case_statement":
		return p.parseCaseStmt(node)
	case "variable_declaration":
		return p.parseVarDecl(node)
	case "expression_statement":
		return p.parseExpressionStmt(node)
	}
	
	return nil
}

// parseReturnStmt parses a return statement
func (p *Parser) parseReturnStmt(node map[string]interface{}) *ast.ReturnStmt {
	ret := &ast.ReturnStmt{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		if childNode["type"] == "expression" {
			ret.Value = p.parseExpression(childNode)
		}
	}
	
	return ret
}

// parseIfStmt parses an if statement
func (p *Parser) parseIfStmt(node map[string]interface{}) *ast.IfStmt {
	ifStmt := &ast.IfStmt{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for i, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "expression":
			if ifStmt.Condition == nil {
				ifStmt.Condition = p.parseExpression(childNode)
			}
		case "block":
			if ifStmt.Then == nil {
				ifStmt.Then = p.parseBlock(childNode)
			} else {
				// This is the else block
				ifStmt.Else = p.parseBlock(childNode)
			}
		case "if_statement":
			// else if
			ifStmt.Else = p.parseIfStmt(childNode)
		}
		
		// Check for else keyword
		if i < len(children)-1 {
			if p.getText(childNode) == "else" {
				// Next node is the else block
				continue
			}
		}
	}
	
	return ifStmt
}

// parseExpressionStmt parses an expression statement
func (p *Parser) parseExpressionStmt(node map[string]interface{}) *ast.ExpressionStmt {
	stmt := &ast.ExpressionStmt{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		if nodeType == "expression" {
			stmt.Expression = p.parseExpression(childNode)
		}
	}
	
	return stmt
}

// parseWhileStmt parses a while statement
func (p *Parser) parseWhileStmt(node map[string]interface{}) *ast.WhileStmt {
	whileStmt := &ast.WhileStmt{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "expression":
			whileStmt.Condition = p.parseExpression(childNode)
		case "block":
			whileStmt.Body = p.parseBlock(childNode)
		}
	}
	
	return whileStmt
}

// parseExpression parses an expression
func (p *Parser) parseExpression(node map[string]interface{}) ast.Expression {
	if node == nil {
		return nil
	}
	
	nodeType, _ := node["type"].(string)
	switch nodeType {
	case "identifier":
		return &ast.Identifier{
			Name:     p.getText(node),
			StartPos: p.getPosition(node, "startPosition"),
			EndPos:   p.getPosition(node, "endPosition"),
		}
	case "number_literal":
		value, _ := strconv.ParseInt(p.getText(node), 0, 64)
		return &ast.NumberLiteral{
			Value:    value,
			StartPos: p.getPosition(node, "startPosition"),
			EndPos:   p.getPosition(node, "endPosition"),
		}
	case "boolean_literal":
		return &ast.BooleanLiteral{
			Value:    p.getText(node) == "true",
			StartPos: p.getPosition(node, "startPosition"),
			EndPos:   p.getPosition(node, "endPosition"),
		}
	case "string_literal":
		text := p.getText(node)
		// Remove quotes
		if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
			text = text[1 : len(text)-1]
		}
		return &ast.StringLiteral{
			Value:    text,
			StartPos: p.getPosition(node, "startPosition"),
			EndPos:   p.getPosition(node, "endPosition"),
		}
	case "array_literal":
		return p.parseArrayLiteral(node)
	case "array_initializer":
		return p.parseArrayInitializer(node)
	case "binary_expression":
		return p.parseBinaryExpr(node)
	case "unary_expression":
		return p.parseUnaryExpr(node)
	case "call_expression":
		return p.parseCallExpr(node)
	case "field_expression":
		return p.parseFieldExpr(node)
	case "index_expression":
		return p.parseIndexExpr(node)
	case "struct_literal":
		return p.parseStructLiteral(node)
	case "error_literal":
		return p.parseEnumLiteral(node)
	case "lua_expression":
		return p.parseLuaExpression(node)
	case "postfix_expression", "expression", "primary_expression":
		// Handle nested expression nodes
		children, _ := node["children"].([]interface{})
		if len(children) > 0 {
			child, _ := children[0].(map[string]interface{})
			return p.parseExpression(child)
		}
		return nil
	case "cast_expression":
		return p.parseCastExpression(node)
	case "inline_assembly":
		return p.parseInlineAssembly(node)
	}
	
	return nil
}

// parseCastExpression parses a cast expression (e.g., x as u16)
func (p *Parser) parseCastExpression(node map[string]interface{}) *ast.CastExpr {
	castExpr := &ast.CastExpr{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	// Get fields for expression and type
	fields, _ := node["fields"].(map[string]interface{})
	
	// Get expression field
	if exprField, ok := fields["expression"].(map[string]interface{}); ok {
		castExpr.Expr = p.parseExpression(exprField)
	}
	
	// Get type field
	if typeField, ok := fields["type"].(map[string]interface{}); ok {
		castExpr.TargetType = p.parseType(typeField)
	}
	
	// Also check children if fields are empty
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		if nodeType == "expression" && castExpr.Expr == nil {
			castExpr.Expr = p.parseExpression(childNode)
		} else if nodeType == "type" && castExpr.TargetType == nil {
			castExpr.TargetType = p.parseType(childNode)
		}
	}
	
	return castExpr
}

// parseInlineAssembly parses an inline assembly expression
func (p *Parser) parseInlineAssembly(node map[string]interface{}) *ast.InlineAssembly {
	asm := &ast.InlineAssembly{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	// Get children nodes
	children, _ := node["children"].([]interface{})
	
	// First child after 'asm' and '(' should be the assembly code string
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "string_literal":
			// Parse the assembly code
			asm.Code = p.parseStringLiteral(childNode)
			
		case "asm_output_list":
			// Parse output operands
			asm.Outputs = p.parseAsmOperands(childNode, true)
			
		case "asm_input_list":
			// Parse input operands
			asm.Inputs = p.parseAsmOperands(childNode, false)
			
		case "asm_clobber_list":
			// Parse clobber list
			clobberChildren, _ := childNode["children"].([]interface{})
			for _, clobber := range clobberChildren {
				clobberNode, _ := clobber.(map[string]interface{})
				if clobberNode["type"] == "string_literal" {
					asm.Clobbers = append(asm.Clobbers, p.parseStringLiteral(clobberNode))
				}
			}
		}
	}
	
	return asm
}

// parseAsmOperands parses assembly operand lists (inputs or outputs)
func (p *Parser) parseAsmOperands(node map[string]interface{}, isOutput bool) []*ast.AsmOperand {
	var operands []*ast.AsmOperand
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		if nodeType == "asm_output" || nodeType == "asm_input" {
			operand := &ast.AsmOperand{}
			
			// Get the constraint and expression
			opChildren, _ := childNode["children"].([]interface{})
			for i, opChild := range opChildren {
				opChildNode, _ := opChild.(map[string]interface{})
				opChildType, _ := opChildNode["type"].(string)
				
				if opChildType == "string_literal" && i == 0 {
					// First string literal is the constraint
					operand.Constraint = p.parseStringLiteral(opChildNode)
				} else if opChildType == "identifier" || opChildType == "expression" {
					// The expression or identifier
					if opChildType == "identifier" {
						operand.Expr = &ast.Identifier{
							Name:     p.getText(opChildNode),
							StartPos: p.getPosition(opChildNode, "startPosition"),
							EndPos:   p.getPosition(opChildNode, "endPosition"),
						}
					} else {
						operand.Expr = p.parseExpression(opChildNode)
					}
				}
			}
			
			operands = append(operands, operand)
		}
	}
	
	return operands
}

// parseBinaryExpr parses a binary expression
func (p *Parser) parseBinaryExpr(node map[string]interface{}) *ast.BinaryExpr {
	binExpr := &ast.BinaryExpr{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	// Get the operator from fields
	fields, _ := node["fields"].(map[string]interface{})
	if op, ok := fields["operator"].(map[string]interface{}); ok {
		binExpr.Operator = p.getText(op)
	}
	
	// Get left and right operands
	if left, ok := fields["left"].(map[string]interface{}); ok {
		binExpr.Left = p.parseExpression(left)
	}
	if right, ok := fields["right"].(map[string]interface{}); ok {
		binExpr.Right = p.parseExpression(right)
	}
	
	return binExpr
}

// parseUnaryExpr parses a unary expression
func (p *Parser) parseUnaryExpr(node map[string]interface{}) *ast.UnaryExpr {
	unExpr := &ast.UnaryExpr{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for i, child := range children {
		childNode, _ := child.(map[string]interface{})
		text := p.getText(childNode)
		
		if i == 0 {
			// First child is the operator
			unExpr.Operator = text
		} else {
			// Second child is the operand
			unExpr.Operand = p.parseExpression(childNode)
		}
	}
	
	return unExpr
}

// parseCallExpr parses a function call expression
func (p *Parser) parseCallExpr(node map[string]interface{}) *ast.CallExpr {
	callExpr := &ast.CallExpr{
		Arguments: []ast.Expression{},
		StartPos:  p.getPosition(node, "startPosition"),
		EndPos:    p.getPosition(node, "endPosition"),
	}
	
	// Get function from fields
	fields, _ := node["fields"].(map[string]interface{})
	if fn, ok := fields["function"].(map[string]interface{}); ok {
		callExpr.Function = p.parseExpression(fn)
	}
	
	// Get arguments
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		if childNode["type"] == "argument_list" {
			callExpr.Arguments = p.parseArguments(childNode)
		}
	}
	
	return callExpr
}

// parseArguments parses an argument list
func (p *Parser) parseArguments(node map[string]interface{}) []ast.Expression {
	args := []ast.Expression{}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		if childNode["type"] != "," {
			if expr := p.parseExpression(childNode); expr != nil {
				args = append(args, expr)
			}
		}
	}
	
	return args
}

// parseFieldExpr parses a field access expression
func (p *Parser) parseFieldExpr(node map[string]interface{}) *ast.FieldExpr {
	fieldExpr := &ast.FieldExpr{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	fields, _ := node["fields"].(map[string]interface{})
	if obj, ok := fields["object"].(map[string]interface{}); ok {
		fieldExpr.Object = p.parseExpression(obj)
	}
	if field, ok := fields["field"].(map[string]interface{}); ok {
		fieldExpr.Field = p.getText(field)
	}
	
	return fieldExpr
}

// parseIndexExpr parses an index expression
func (p *Parser) parseIndexExpr(node map[string]interface{}) *ast.IndexExpr {
	indexExpr := &ast.IndexExpr{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	fields, _ := node["fields"].(map[string]interface{})
	if obj, ok := fields["object"].(map[string]interface{}); ok {
		indexExpr.Array = p.parseExpression(obj)
	}
	if idx, ok := fields["index"].(map[string]interface{}); ok {
		indexExpr.Index = p.parseExpression(idx)
	}
	
	return indexExpr
}

// parseStructLiteral parses a struct literal expression
func (p *Parser) parseStructLiteral(node map[string]interface{}) *ast.StructLiteral {
	structLit := &ast.StructLiteral{
		Fields:   []*ast.FieldInit{},
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "type_identifier":
			structLit.TypeName = p.getText(childNode)
		case "field_initializer":
			if fieldInit := p.parseFieldInit(childNode); fieldInit != nil {
				structLit.Fields = append(structLit.Fields, fieldInit)
			}
		}
	}
	
	return structLit
}

// parseArrayLiteral parses an array literal with [...] syntax
func (p *Parser) parseArrayLiteral(node map[string]interface{}) *ast.ArrayInitializer {
	arrayLit := &ast.ArrayInitializer{
		Elements: []ast.Expression{},
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		if nodeType == "expression" {
			if expr := p.parseExpression(childNode); expr != nil {
				arrayLit.Elements = append(arrayLit.Elements, expr)
			}
		}
	}
	
	return arrayLit
}

// parseArrayInitializer parses an array initializer with {...} syntax
func (p *Parser) parseArrayInitializer(node map[string]interface{}) *ast.ArrayInitializer {
	// Array initializers have the same structure as array literals
	return p.parseArrayLiteral(node)
}

// parseFieldInit parses a field initialization in a struct literal
func (p *Parser) parseFieldInit(node map[string]interface{}) *ast.FieldInit {
	fieldInit := &ast.FieldInit{}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "identifier":
			if fieldInit.Name == "" {
				fieldInit.Name = p.getText(childNode)
			}
		case "expression":
			fieldInit.Value = p.parseExpression(childNode)
		}
	}
	
	return fieldInit
}

// parseEnumLiteral parses an enum literal (Error.variant style)
func (p *Parser) parseEnumLiteral(node map[string]interface{}) *ast.EnumLiteral {
	enumLit := &ast.EnumLiteral{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	identCount := 0
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		if nodeType == "identifier" {
			if identCount == 0 {
				enumLit.EnumName = p.getText(childNode)
			} else {
				enumLit.Variant = p.getText(childNode)
			}
			identCount++
		}
	}
	
	return enumLit
}

// parseLuaBlock parses a @lua[[...]] block
func (p *Parser) parseLuaBlock(node map[string]interface{}) *ast.LuaBlock {
	luaBlock := &ast.LuaBlock{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	// Extract the Lua code
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		if childNode["type"] == "lua_code" {
			luaBlock.Code = p.getText(childNode)
			break
		}
	}
	
	return luaBlock
}

// parseLuaExpression parses a @lua(...) expression
func (p *Parser) parseLuaExpression(node map[string]interface{}) *ast.LuaExpression {
	luaExpr := &ast.LuaExpression{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	// Extract the Lua code
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		if childNode["type"] == "lua_code" {
			luaExpr.Code = p.getText(childNode)
			break
		}
	}
	
	return luaExpr
}

// parseLuaEval parses a @lua_eval(...) statement
func (p *Parser) parseLuaEval(node map[string]interface{}) *ast.LuaEval {
	luaEval := &ast.LuaEval{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	// Extract the Lua code
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		if childNode["type"] == "lua_code" {
			luaEval.Code = p.getText(childNode)
			break
		}
	}
	
	return luaEval
}

// Helper functions

// getText extracts the text content of a node
func (p *Parser) getText(node map[string]interface{}) string {
	if node == nil {
		return ""
	}
	if text, ok := node["text"].(string); ok {
		return text
	}
	return ""
}

// reconstructImportPath reconstructs an import path from its children
func (p *Parser) reconstructImportPath(node map[string]interface{}) string {
	children, ok := node["children"].([]interface{})
	if !ok {
		return ""
	}
	
	var parts []string
	for _, child := range children {
		childNode, ok := child.(map[string]interface{})
		if !ok {
			continue
		}
		
		nodeType, _ := childNode["type"].(string)
		if nodeType == "identifier" {
			if text := p.getText(childNode); text != "" {
				parts = append(parts, text)
			}
		}
	}
	
	return strings.Join(parts, ".")
}

// parseStringLiteral extracts string literal content without quotes
func (p *Parser) parseStringLiteral(node map[string]interface{}) string {
	text := p.getText(node)
	// Remove quotes
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	return text
}

// getPosition extracts a position from a node
func (p *Parser) getPosition(node map[string]interface{}, key string) ast.Position {
	pos := ast.Position{}
	
	if posData, ok := node[key].(map[string]interface{}); ok {
		if row, ok := posData["row"].(float64); ok {
			pos.Line = int(row) + 1 // Convert to 1-based
		}
		if col, ok := posData["column"].(float64); ok {
			pos.Column = int(col) + 1 // Convert to 1-based
		}
	}
	
	return pos
}

// parseCaseStmt parses a case statement
func (p *Parser) parseCaseStmt(node map[string]interface{}) *ast.CaseStmt {
	caseStmt := &ast.CaseStmt{
		Arms:     []*ast.CaseArm{},
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "expression":
			if caseStmt.Expr == nil {
				caseStmt.Expr = p.parseExpression(childNode)
			}
		case "case_arm":
			if arm := p.parseCaseArm(childNode); arm != nil {
				caseStmt.Arms = append(caseStmt.Arms, arm)
			}
		}
	}
	
	return caseStmt
}

// parseCaseArm parses a case arm
func (p *Parser) parseCaseArm(node map[string]interface{}) *ast.CaseArm {
	arm := &ast.CaseArm{
		StartPos: p.getPosition(node, "startPosition"),
		EndPos:   p.getPosition(node, "endPosition"),
	}
	
	children, _ := node["children"].([]interface{})
	
	// First pass: count expressions to determine if we have a guard
	expressionCount := 0
	patternSeen := false
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		if nodeType == "pattern" {
			patternSeen = true
		} else if nodeType == "expression" && patternSeen {
			expressionCount++
		}
	}
	
	// Second pass: parse with knowledge of structure
	exprIndex := 0
	for _, child := range children {
		childNode, _ := child.(map[string]interface{})
		nodeType, _ := childNode["type"].(string)
		
		switch nodeType {
		case "pattern":
			arm.Pattern = p.parsePattern(childNode)
		case "expression":
			if arm.Pattern != nil {
				exprIndex++
				if expressionCount == 2 && exprIndex == 1 {
					// Two expressions: first is guard
					arm.Guard = p.parseExpression(childNode)
				} else if expressionCount == 2 && exprIndex == 2 {
					// Two expressions: second is body
					arm.Body = p.parseExpression(childNode)
				} else if expressionCount == 1 {
					// One expression: it's the body
					arm.Body = p.parseExpression(childNode)
				}
			}
		case "block":
			arm.Body = p.parseBlock(childNode)
		}
	}
	
	return arm
}

// parsePattern parses a pattern
func (p *Parser) parsePattern(node map[string]interface{}) ast.Pattern {
	if node == nil {
		return nil
	}
	
	// Check if this is a wildcard pattern by examining the text
	if text := p.getText(node); text == "_" {
		return &ast.WildcardPattern{
			StartPos: p.getPosition(node, "startPosition"),
			EndPos:   p.getPosition(node, "endPosition"),
		}
	}
	
	children, _ := node["children"].([]interface{})
	if len(children) == 0 {
		// If no children but we have text, it might be an identifier pattern
		if text := p.getText(node); text != "" {
			return &ast.IdentifierPattern{
				Name:     text,
				StartPos: p.getPosition(node, "startPosition"),
				EndPos:   p.getPosition(node, "endPosition"),
			}
		}
		return nil
	}
	
	childNode, _ := children[0].(map[string]interface{})
	nodeType, _ := childNode["type"].(string)
	
	switch nodeType {
	case "_":
		return &ast.WildcardPattern{
			StartPos: p.getPosition(childNode, "startPosition"),
			EndPos:   p.getPosition(childNode, "endPosition"),
		}
	case "field_expression":
		// Parse field expression like Direction.North
		obj := ""
		field := ""
		fieldChildren, _ := childNode["children"].([]interface{})
		for _, fc := range fieldChildren {
			fcNode, _ := fc.(map[string]interface{})
			fcType, _ := fcNode["type"].(string)
			if fcType == "identifier" {
				if obj == "" {
					obj = p.getText(fcNode)
				} else {
					field = p.getText(fcNode)
				}
			}
		}
		return &ast.IdentifierPattern{
			Name:     obj + "." + field,
			StartPos: p.getPosition(childNode, "startPosition"),
			EndPos:   p.getPosition(childNode, "endPosition"),
		}
	case "identifier":
		return &ast.IdentifierPattern{
			Name:     p.getText(childNode),
			StartPos: p.getPosition(childNode, "startPosition"),
			EndPos:   p.getPosition(childNode, "endPosition"),
		}
	case "literal_pattern":
		// Parse the nested literal
		literalChildren, _ := childNode["children"].([]interface{})
		if len(literalChildren) > 0 {
			literal, _ := literalChildren[0].(map[string]interface{})
			return &ast.LiteralPattern{
				Value:    p.parseExpression(literal),
				StartPos: p.getPosition(childNode, "startPosition"),
				EndPos:   p.getPosition(childNode, "endPosition"),
			}
		}
	}
	
	return nil
}