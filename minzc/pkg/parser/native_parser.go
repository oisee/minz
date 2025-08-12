package parser

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/minz/minzc/pkg/ast"
	minz "github.com/minz/minzc/pkg/parser/minz_binding"
)

// NativeParser uses embedded tree-sitter for parsing
type NativeParser struct {
	parser *sitter.Parser
}

// NewNativeParser creates a new native tree-sitter parser
func NewNativeParser() *NativeParser {
	parser := sitter.NewParser()
	parser.SetLanguage(sitter.NewLanguage(minz.Language()))
	return &NativeParser{
		parser: parser,
	}
}

// ParseFile parses a MinZ source file using native tree-sitter
func (p *NativeParser) ParseFile(filename string) (*ast.File, error) {
	// Read source file
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Parse with tree-sitter
	tree, err := p.parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	defer tree.Close()
	
	// Check for parse errors
	if tree.RootNode().HasError() {
		return nil, fmt.Errorf("syntax error in %s", filename)
	}
	
	// Convert to AST
	return p.convertToAST(tree.RootNode(), source), nil
}

// convertToAST converts tree-sitter parse tree to MinZ AST
func (p *NativeParser) convertToAST(root *sitter.Node, source []byte) *ast.File {
	file := &ast.File{
		Imports:      []*ast.ImportStmt{},
		Declarations: []ast.Declaration{},
	}
	
	// Walk through top-level nodes
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(int(i))
		
		switch child.Type() {
		case "import_statement":
			if imp := p.parseImport(child, source); imp != nil {
				file.Imports = append(file.Imports, imp)
			}
		case "function_declaration":
			if fn := p.parseFunction(child, source); fn != nil {
				file.Declarations = append(file.Declarations, fn)
			}
		case "asm_function":
			if fn := p.parseAsmFunction(child, source); fn != nil {
				file.Declarations = append(file.Declarations, fn)
			}
		case "mir_function":
			if fn := p.parseMIRFunction(child, source); fn != nil {
				file.Declarations = append(file.Declarations, fn)
			}
		case "struct_declaration":
			if st := p.parseStruct(child, source); st != nil {
				file.Declarations = append(file.Declarations, st)
			}
		case "variable_declaration":
			if vdecl := p.parseVariableDeclaration(child, source); vdecl != nil {
				file.Declarations = append(file.Declarations, vdecl)
			}
		case "constant_declaration":
			if con := p.parseConst(child, source); con != nil {
				file.Declarations = append(file.Declarations, con)
			}
		case "enum_declaration":
			if enum := p.parseEnum(child, source); enum != nil {
				file.Declarations = append(file.Declarations, enum)
			}
		case "type_alias":
			if alias := p.parseTypeAlias(child, source); alias != nil {
				file.Declarations = append(file.Declarations, alias)
			}
		case "interface_declaration":
			if iface := p.parseInterface(child, source); iface != nil {
				file.Declarations = append(file.Declarations, iface)
			}
		case "impl_block":
			if impl := p.parseImplBlock(child, source); impl != nil {
				file.Declarations = append(file.Declarations, impl)
			}
		case "lua_block":
			if lua := p.parseLuaBlock(child, source); lua != nil {
				file.Declarations = append(file.Declarations, lua)
			}
		case "mir_block_declaration":
			if mir := p.parseMIRBlock(child, source); mir != nil {
				file.Declarations = append(file.Declarations, mir)
			}
		case "minz_metafunction_declaration":
			if meta := p.parseMinzMetafunctionDecl(child, source); meta != nil {
				file.Declarations = append(file.Declarations, meta)
			}
		case "compile_time_if_declaration":
			if ctif := p.parseCompileTimeIfDecl(child, source); ctif != nil {
				file.Declarations = append(file.Declarations, ctif)
			}
		case "attributed_declaration":
			if attr := p.parseAttributedDeclaration(child, source); attr != nil {
				file.Declarations = append(file.Declarations, attr)
			}
		case "define_template":
			if tmpl := p.parseDefineTemplate(child, source); tmpl != nil {
				file.Declarations = append(file.Declarations, tmpl)
			}
		case "meta_execution_block":
			if meta := p.parseMetaExecutionBlock(child, source); meta != nil {
				file.Declarations = append(file.Declarations, meta)
			}
		}
	}
	
	return file
}

// parseImport converts import declaration to AST
func (p *NativeParser) parseImport(node *sitter.Node, source []byte) *ast.ImportStmt {
	imp := &ast.ImportStmt{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		
		switch child.Type() {
		case "import_path":
			imp.Path = p.extractText(child, source)
		case "identifier":
			// Check if this is an alias (comes after "as")
			if i > 0 && node.Child(int(i-1)).Type() == "as" {
				imp.Alias = p.extractText(child, source)
			}
		}
	}
	
	return imp
}

// parseFunction converts function declaration to AST
func (p *NativeParser) parseFunction(node *sitter.Node, source []byte) *ast.FunctionDecl {
	fn := &ast.FunctionDecl{
		Params: []*ast.Parameter{},
		Body:   &ast.BlockStmt{Statements: []ast.Statement{}},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		
		switch child.Type() {
		case "identifier":
			fn.Name = p.extractText(child, source)
		case "parameter_list":
			fn.Params = p.parseParameters(child, source)
		case "type":
			fn.ReturnType = p.parseType(child, source)
		case "block":
			fn.Body = p.parseBlock(child, source)
		case "pub":
			fn.IsPublic = true
		}
	}
	
	return fn
}

// parseStruct converts struct declaration to AST
func (p *NativeParser) parseStruct(node *sitter.Node, source []byte) *ast.StructDecl {
	st := &ast.StructDecl{
		Fields: []*ast.Field{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		
		switch child.Type() {
		case "identifier":
			st.Name = p.extractText(child, source)
		case "field_list":
			st.Fields = p.parseFields(child, source)
		}
	}
	
	return st
}

// parseGlobal converts global declaration to AST
func (p *NativeParser) parseGlobal(node *sitter.Node, source []byte) *ast.VarDecl {
	glob := &ast.VarDecl{
		IsPublic: true,  // Global variables are public by default
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		
		switch child.Type() {
		case "identifier":
			glob.Name = p.extractText(child, source)
		case "type":
			glob.Type = p.parseType(child, source)
		case "expression":
			glob.Value = p.parseExpression(child, source)
		}
	}
	
	return glob
}

// parseConst converts const declaration to AST
func (p *NativeParser) parseConst(node *sitter.Node, source []byte) *ast.ConstDecl {
	con := &ast.ConstDecl{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		
		switch child.Type() {
		case "identifier":
			con.Name = p.extractText(child, source)
		case "type":
			con.Type = p.parseType(child, source)
		case "expression":
			con.Value = p.parseExpression(child, source)
		}
	}
	
	return con
}

// parseEnum converts enum declaration to AST
func (p *NativeParser) parseEnum(node *sitter.Node, source []byte) *ast.EnumDecl {
	enum := &ast.EnumDecl{
		Variants: []string{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		
		switch child.Type() {
		case "identifier":
			enum.Name = p.extractText(child, source)
		case "enum_member_list":
			enum.Variants = p.parseEnumMembers(child, source)
		}
	}
	
	return enum
}

// parseTypeAlias converts type alias to AST
func (p *NativeParser) parseTypeAlias(node *sitter.Node, source []byte) *ast.TypeDecl {
	alias := &ast.TypeDecl{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		
		switch child.Type() {
		case "identifier":
			if alias.Name == "" {
				alias.Name = p.extractText(child, source)
			}
		case "type":
			alias.Type = p.parseType(child, source)
		}
	}
	
	return alias
}

// Helper methods for parsing sub-elements

func (p *NativeParser) parseParameters(node *sitter.Node, source []byte) []*ast.Parameter {
	params := []*ast.Parameter{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "parameter" {
			param := &ast.Parameter{}
			for j := uint32(0); j < child.ChildCount(); j++ {
				subChild := child.Child(int(j))
				switch subChild.Type() {
				case "identifier":
					param.Name = p.extractText(subChild, source)
				case "type":
					param.Type = p.parseType(subChild, source)
				}
			}
			params = append(params, param)
		}
	}
	
	return params
}

func (p *NativeParser) parseFields(node *sitter.Node, source []byte) []*ast.Field {
	fields := []*ast.Field{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "field" {
			field := &ast.Field{}
			for j := uint32(0); j < child.ChildCount(); j++ {
				subChild := child.Child(int(j))
				switch subChild.Type() {
				case "identifier":
					field.Name = p.extractText(subChild, source)
				case "type":
					field.Type = p.parseType(subChild, source)
				}
			}
			fields = append(fields, field)
		}
	}
	
	return fields
}

func (p *NativeParser) parseEnumMembers(node *sitter.Node, source []byte) []string {
	members := []string{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "enum_member" {
			name := p.extractText(child.ChildByFieldName("name"), source)
			if name == "" {
				name = p.extractText(child, source)
			}
			members = append(members, name)
		}
	}
	
	return members
}

func (p *NativeParser) parseType(node *sitter.Node, source []byte) ast.Type {
	if node == nil {
		return nil
	}
	
	switch node.Type() {
	case "primitive_type":
		return &ast.PrimitiveType{
			Name: p.extractText(node, source),
		}
	case "array_type":
		return p.parseArrayType(node, source)
	case "pointer_type":
		return p.parsePointerType(node, source)
	case "function_type":
		return p.parseFunctionType(node, source)
	case "struct_type":
		return p.parseStructType(node, source)
	case "enum_type":
		return p.parseEnumType(node, source)
	case "bit_struct_type":
		return p.parseBitStructType(node, source)
	case "error_type":
		return &ast.ErrorType{
			ValueType: &ast.PrimitiveType{Name: "Error"},
		}
	case "type_identifier":
		return &ast.TypeIdentifier{
			Name: p.extractText(node, source),
		}
	case "identifier":
		return &ast.TypeIdentifier{
			Name: p.extractText(node, source),
		}
	default:
		// For now, return a basic type
		return &ast.PrimitiveType{
			Name: p.extractText(node, source),
		}
	}
}

func (p *NativeParser) parseBlock(node *sitter.Node, source []byte) *ast.BlockStmt {
	block := &ast.BlockStmt{
		Statements: []ast.Statement{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if stmt := p.parseStatement(child, source); stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
	}
	
	return block
}

func (p *NativeParser) parseStatement(node *sitter.Node, source []byte) ast.Statement {
	switch node.Type() {
	case "return_statement":
		return &ast.ReturnStmt{
			Value: p.parseExpression(node.ChildByFieldName("value"), source),
		}
	case "expression_statement":
		return &ast.ExpressionStmt{
			Expression: p.parseExpression(node.Child(0), source),
		}
	case "variable_declaration":
		return p.parseVariableDeclaration(node, source)
	case "constant_declaration":
		return p.parseConst(node, source)
	case "function_declaration":
		return p.parseFunction(node, source)
	case "if_statement":
		return p.parseIfStatement(node, source)
	case "while_statement":
		return p.parseWhileStatement(node, source)
	case "for_statement":
		return p.parseForStatement(node, source)
	case "loop_statement":
		return p.parseLoopStatement(node, source)
	case "break_statement":
		return p.parseBreakStatement(node, source)
	case "continue_statement":
		return p.parseContinueStatement(node, source)
	case "block_statement":
		return p.parseBlock(node, source)
	case "defer_statement":
		return p.parseDeferStatement(node, source)
	case "case_statement":
		return p.parseCaseStatement(node, source)
	case "asm_block":
		return p.parseAsmBlock(node, source)
	case "compile_time_asm":
		return p.parseCompileTimeAsm(node, source)
	case "mir_block":
		return p.parseMIRBlockStmt(node, source)
	case "minz_block":
		return p.parseMinzBlockStmt(node, source)
	case "target_block":
		return p.parseTargetBlockStmt(node, source)
	default:
		return nil
	}
}

func (p *NativeParser) parseLetStatement(node *sitter.Node, source []byte) *ast.VarDecl {
	let := &ast.VarDecl{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			let.Name = p.extractText(child, source)
		case "type":
			let.Type = p.parseType(child, source)
		case "expression":
			let.Value = p.parseExpression(child, source)
		}
	}
	
	return let
}

func (p *NativeParser) parseIfStatement(node *sitter.Node, source []byte) *ast.IfStmt {
	ifStmt := &ast.IfStmt{}
	
	ifStmt.Condition = p.parseExpression(node.ChildByFieldName("condition"), source)
	ifStmt.Then = p.parseBlock(node.ChildByFieldName("consequence"), source)
	if elseNode := node.ChildByFieldName("alternative"); elseNode != nil {
		ifStmt.Else = p.parseBlock(elseNode, source)
	}
	
	return ifStmt
}

func (p *NativeParser) parseWhileStatement(node *sitter.Node, source []byte) *ast.WhileStmt {
	while := &ast.WhileStmt{}
	
	while.Condition = p.parseExpression(node.ChildByFieldName("condition"), source)
	while.Body = p.parseBlock(node.ChildByFieldName("body"), source)
	
	return while
}

func (p *NativeParser) parseForStatement(node *sitter.Node, source []byte) *ast.ForStmt {
	forStmt := &ast.ForStmt{}
	
	// MinZ for loops use iterator and range
	if iterNode := node.ChildByFieldName("iterator"); iterNode != nil {
		forStmt.Iterator = p.extractText(iterNode, source)
	}
	if rangeNode := node.ChildByFieldName("range"); rangeNode != nil {
		forStmt.Range = p.parseExpression(rangeNode, source)
	}
	forStmt.Body = p.parseBlock(node.ChildByFieldName("body"), source)
	
	return forStmt
}

func (p *NativeParser) parseExpression(node *sitter.Node, source []byte) ast.Expression {
	if node == nil {
		return nil
	}
	
	switch node.Type() {
	case "number_literal":
		text := p.extractText(node, source)
		val, _ := strconv.ParseInt(text, 0, 64)
		return &ast.NumberLiteral{
			Value: val,
		}
	case "string_literal":
		text := p.extractText(node, source)
		isLong := strings.HasPrefix(text, "l\"") || strings.HasPrefix(text, "L\"")
		// Remove quotes and prefix
		if len(text) >= 2 {
			if isLong && len(text) >= 3 {
				text = text[2:len(text)-1]
			} else {
				text = text[1:len(text)-1]
			}
		}
		return &ast.StringLiteral{
			Value: text,
			IsLong: isLong,
		}
	case "char_literal":
		text := p.extractText(node, source)
		if len(text) >= 3 {
			text = text[1:len(text)-1] // Remove quotes
		}
		return &ast.StringLiteral{
			Value: text,
		}
	case "boolean_literal":
		text := p.extractText(node, source)
		return &ast.BooleanLiteral{
			Value: text == "true",
		}
	case "identifier":
		return &ast.Identifier{
			Name: p.extractText(node, source),
		}
	case "binary_expression":
		return &ast.BinaryExpr{
			Left:     p.parseExpression(node.ChildByFieldName("left"), source),
			Operator: p.extractText(node.ChildByFieldName("operator"), source),
			Right:    p.parseExpression(node.ChildByFieldName("right"), source),
		}
	case "unary_expression":
		return p.parseUnaryExpression(node, source)
	case "postfix_expression":
		return p.parsePostfixExpression(node, source)
	case "call_expression":
		return p.parseCallExpression(node, source)
	case "index_expression":
		return p.parseIndexExpression(node, source)
	case "field_expression":
		return p.parseFieldExpression(node, source)
	case "try_expression":
		return p.parseTryExpression(node, source)
	case "cast_expression":
		return p.parseCastExpression(node, source)
	case "primary_expression":
		return p.parsePrimaryExpression(node, source)
	case "array_literal":
		return p.parseArrayLiteral(node, source)
	case "array_initializer":
		return p.parseArrayInitializer(node, source)
	case "struct_literal":
		return p.parseStructLiteral(node, source)
	case "tuple_literal":
		return p.parseTupleLiteral(node, source)
	case "parenthesized_expression":
		return p.parseParenthesizedExpression(node, source)
	case "block":
		return p.parseBlock(node, source)
	case "inline_assembly":
		return p.parseInlineAssembly(node, source)
	case "sizeof_expression":
		return p.parseSizeofExpression(node, source)
	case "alignof_expression":
		return p.parseAlignofExpression(node, source)
	case "metaprogramming_expression":
		return p.parseMetaprogrammingExpression(node, source)
	case "error_literal":
		return p.parseErrorLiteral(node, source)
	case "lambda_expression":
		return p.parseLambdaExpression(node, source)
	case "if_expression":
		return p.parseIfExpression(node, source)
	case "ternary_expression":
		return p.parseTernaryExpression(node, source)
	case "when_expression":
		return p.parseWhenExpression(node, source)
	case "compile_time_if":
		return p.parseCompileTimeIf(node, source)
	case "compile_time_print":
		return p.parseCompileTimePrint(node, source)
	case "compile_time_assert":
		return p.parseCompileTimeAssert(node, source)
	case "compile_time_error":
		return p.parseCompileTimeError(node, source)
	case "attribute":
		return p.parseAttribute(node, source)
	case "lua_expression":
		return p.parseLuaExpression(node, source)
	case "lua_eval":
		return p.parseLuaEval(node, source)
	case "compile_time_minz":
		return p.parseCompileTimeMinz(node, source)
	case "compile_time_mir":
		return p.parseCompileTimeMIR(node, source)
	default:
		// For unknown expressions, return identifier with the text
		return &ast.Identifier{
			Name: p.extractText(node, source),
		}
	}
}

func (p *NativeParser) parseCallExpression(node *sitter.Node, source []byte) *ast.CallExpr {
	call := &ast.CallExpr{
		Arguments: []ast.Expression{},
	}
	
	call.Function = p.parseExpression(node.ChildByFieldName("function"), source)
	
	if argsNode := node.ChildByFieldName("arguments"); argsNode != nil {
		for i := uint32(0); i < argsNode.ChildCount(); i++ {
			child := argsNode.Child(int(i))
			if child.Type() != "," && child.Type() != "(" && child.Type() != ")" {
				call.Arguments = append(call.Arguments, p.parseExpression(child, source))
			}
		}
	}
	
	return call
}

// Type parsing helpers

func (p *NativeParser) parseArrayType(node *sitter.Node, source []byte) *ast.ArrayType {
	arr := &ast.ArrayType{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "type":
			if arr.ElementType == nil {
				arr.ElementType = p.parseType(child, source)
			}
		case "expression":
			if arr.Size == nil {
				arr.Size = p.parseExpression(child, source)
			}
		}
	}
	
	return arr
}

func (p *NativeParser) parsePointerType(node *sitter.Node, source []byte) *ast.PointerType {
	ptr := &ast.PointerType{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "mut":
			ptr.IsMutable = true
		case "type":
			ptr.BaseType = p.parseType(child, source)
		}
	}
	
	return ptr
}

func (p *NativeParser) parseFunctionType(node *sitter.Node, source []byte) *ast.TypeIdentifier {
	// For now, return function types as type identifiers
	// TODO: Implement proper function type AST node
	return &ast.TypeIdentifier{Name: "Function"}
}

func (p *NativeParser) parseStructType(node *sitter.Node, source []byte) *ast.StructType {
	structType := &ast.StructType{
		Fields: []*ast.Field{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "field_declaration" {
			field := p.parseFieldDeclaration(child, source)
			if field != nil {
				structType.Fields = append(structType.Fields, field)
			}
		}
	}
	
	return structType
}

func (p *NativeParser) parseEnumType(node *sitter.Node, source []byte) *ast.EnumType {
	enumType := &ast.EnumType{
		Variants: []string{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "enum_variant" {
			variant := p.extractText(child, source)
			enumType.Variants = append(enumType.Variants, variant)
		}
	}
	
	return enumType
}

func (p *NativeParser) parseBitStructType(node *sitter.Node, source []byte) *ast.BitStructType {
	bitStruct := &ast.BitStructType{
		Fields: []*ast.BitField{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "bit_field" {
			field := p.parseBitField(child, source)
			if field != nil {
				bitStruct.Fields = append(bitStruct.Fields, field)
			}
		}
	}
	
	return bitStruct
}

func (p *NativeParser) parseBitField(node *sitter.Node, source []byte) *ast.BitField {
	field := &ast.BitField{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			field.Name = p.extractText(child, source)
		case "number_literal":
			text := p.extractText(child, source)
			if width, err := strconv.Atoi(text); err == nil {
				field.BitWidth = width
			}
		}
	}
	
	return field
}

func (p *NativeParser) parseFieldDeclaration(node *sitter.Node, source []byte) *ast.Field {
	field := &ast.Field{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "visibility":
			field.IsPublic = true
		case "identifier":
			field.Name = p.extractText(child, source)
		case "type":
			field.Type = p.parseType(child, source)
		}
	}
	
	return field
}

// Function declaration parsers

func (p *NativeParser) parseAsmFunction(node *sitter.Node, source []byte) *ast.FunctionDecl {
	fn := p.parseFunction(node, source)
	if fn != nil {
		fn.FunctionKind = ast.FunctionKindAsm
	}
	return fn
}

func (p *NativeParser) parseMIRFunction(node *sitter.Node, source []byte) *ast.FunctionDecl {
	fn := p.parseFunction(node, source)
	if fn != nil {
		fn.FunctionKind = ast.FunctionKindMIR
	}
	return fn
}

func (p *NativeParser) parseVariableDeclaration(node *sitter.Node, source []byte) *ast.VarDecl {
	vdecl := &ast.VarDecl{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "visibility":
			vdecl.IsPublic = true
		case "mut":
			vdecl.IsMutable = true
		case "identifier":
			vdecl.Name = p.extractText(child, source)
		case "type":
			vdecl.Type = p.parseType(child, source)
		case "expression":
			vdecl.Value = p.parseExpression(child, source)
		}
	}
	
	return vdecl
}

func (p *NativeParser) parseInterface(node *sitter.Node, source []byte) *ast.InterfaceDecl {
	iface := &ast.InterfaceDecl{
		Methods:    []*ast.InterfaceMethod{},
		CastBlocks: []*ast.CastInterfaceBlock{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "visibility":
			iface.IsPublic = true
		case "identifier":
			iface.Name = p.extractText(child, source)
		case "interface_method":
			if method := p.parseInterfaceMethod(child, source); method != nil {
				iface.Methods = append(iface.Methods, method)
			}
		case "cast_interface_block":
			if cast := p.parseCastInterfaceBlock(child, source); cast != nil {
				iface.CastBlocks = append(iface.CastBlocks, cast)
			}
		}
	}
	
	return iface
}

func (p *NativeParser) parseInterfaceMethod(node *sitter.Node, source []byte) *ast.InterfaceMethod {
	method := &ast.InterfaceMethod{
		Params: []*ast.Parameter{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			method.Name = p.extractText(child, source)
		case "parameter_list":
			method.Params = p.parseParameters(child, source)
		case "return_type":
			if returnType := p.parseReturnType(child, source); returnType != nil {
				method.ReturnType = returnType
			}
		}
	}
	
	return method
}

func (p *NativeParser) parseReturnType(node *sitter.Node, source []byte) ast.Type {
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type" {
			return p.parseType(child, source)
		}
	}
	return nil
}

func (p *NativeParser) parseCastInterfaceBlock(node *sitter.Node, source []byte) *ast.CastInterfaceBlock {
	cast := &ast.CastInterfaceBlock{
		CastRules: []*ast.CastRule{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			cast.TargetType = p.extractText(child, source)
		}
	}
	
	return cast
}

func (p *NativeParser) parseImplBlock(node *sitter.Node, source []byte) *ast.ImplBlock {
	impl := &ast.ImplBlock{
		Methods: []*ast.FunctionDecl{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			if impl.InterfaceName == "" {
				impl.InterfaceName = p.extractText(child, source)
			}
		case "type":
			impl.ForType = p.parseType(child, source)
		case "function_declaration":
			if fn := p.parseFunction(child, source); fn != nil {
				impl.Methods = append(impl.Methods, fn)
			}
		}
	}
	
	return impl
}

// Metaprogramming declarations

func (p *NativeParser) parseLuaBlock(node *sitter.Node, source []byte) *ast.LuaBlock {
	lua := &ast.LuaBlock{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "lua_code_block" {
			lua.Code = p.extractText(child, source)
			break
		}
	}
	
	return lua
}

func (p *NativeParser) parseMIRBlock(node *sitter.Node, source []byte) *ast.MIRBlock {
	mir := &ast.MIRBlock{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "mir_block_content" {
			mir.Code = p.extractText(child, source)
			break
		}
	}
	
	return mir
}

func (p *NativeParser) parseMinzMetafunctionDecl(node *sitter.Node, source []byte) *ast.MetafunctionDecl {
	meta := &ast.MetafunctionDecl{
		Arguments: []ast.Expression{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "string_literal":
			if meta.Code == "" {
				meta.Code = p.extractText(child, source)
			}
		case "expression":
			meta.Arguments = append(meta.Arguments, p.parseExpression(child, source))
		}
	}
	
	return meta
}

func (p *NativeParser) parseCompileTimeIfDecl(node *sitter.Node, source []byte) *ast.ExpressionDecl {
	// Wrap compile-time if as expression declaration
	expr := p.parseCompileTimeIf(node, source)
	if expr != nil {
		return &ast.ExpressionDecl{Expression: expr}
	}
	return nil
}

func (p *NativeParser) parseAttributedDeclaration(node *sitter.Node, source []byte) ast.Declaration {
	// Find the actual declaration within the attributed declaration
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "function_declaration":
			return p.parseFunction(child, source)
		case "struct_declaration":
			return p.parseStruct(child, source)
		case "interface_declaration":
			return p.parseInterface(child, source)
		}
	}
	return nil
}

func (p *NativeParser) parseDefineTemplate(node *sitter.Node, source []byte) *ast.DefineTemplate {
	tmpl := &ast.DefineTemplate{
		Parameters: []string{},
		Arguments:  []ast.Expression{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier_list":
			tmpl.Parameters = p.parseIdentifierList(child, source)
		case "expression_list":
			tmpl.Arguments = p.parseExpressionList(child, source)
		case "template_body":
			tmpl.Body = p.extractText(child, source)
		}
	}
	
	return tmpl
}

func (p *NativeParser) parseMetaExecutionBlock(node *sitter.Node, source []byte) *ast.MetaExecutionBlock {
	meta := &ast.MetaExecutionBlock{}
	
	// Determine language from node type
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch node.Type() {
		case "lua_execution_block":
			meta.Language = "lua"
		case "minz_execution_block":
			meta.Language = "minz"
		case "mir_execution_block":
			meta.Language = "mir"
		}
		
		if child.Type() == "raw_block_content" {
			meta.Code = p.extractText(child, source)
		}
	}
	
	return meta
}

func (p *NativeParser) parseIdentifierList(node *sitter.Node, source []byte) []string {
	identifiers := []string{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			identifiers = append(identifiers, p.extractText(child, source))
		}
	}
	
	return identifiers
}

func (p *NativeParser) parseExpressionList(node *sitter.Node, source []byte) []ast.Expression {
	expressions := []ast.Expression{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "expression" {
			expressions = append(expressions, p.parseExpression(child, source))
		}
	}
	
	return expressions
}

// Statement parsing functions

func (p *NativeParser) parseLoopStatement(node *sitter.Node, source []byte) *ast.LoopStmt {
	loop := &ast.LoopStmt{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "expression":
			if loop.Table == nil {
				loop.Table = p.parseExpression(child, source)
			}
		case "identifier":
			if loop.Iterator == "" {
				loop.Iterator = p.extractText(child, source)
			}
		case "block":
			loop.Body = p.parseBlock(child, source)
		}
	}
	
	return loop
}

func (p *NativeParser) parseBreakStatement(node *sitter.Node, source []byte) ast.Statement {
	// For simplicity, return an expression statement with an identifier
	return &ast.ExpressionStmt{
		Expression: &ast.Identifier{Name: "break"},
	}
}

func (p *NativeParser) parseContinueStatement(node *sitter.Node, source []byte) ast.Statement {
	// For simplicity, return an expression statement with an identifier
	return &ast.ExpressionStmt{
		Expression: &ast.Identifier{Name: "continue"},
	}
}

func (p *NativeParser) parseDeferStatement(node *sitter.Node, source []byte) ast.Statement {
	// For now, parse as regular statement - TODO: Add DeferStmt to AST
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "statement" {
			return p.parseStatement(child, source)
		}
	}
	return nil
}

func (p *NativeParser) parseCaseStatement(node *sitter.Node, source []byte) *ast.CaseStmt {
	caseStmt := &ast.CaseStmt{
		Arms: []*ast.CaseArm{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "expression":
			if caseStmt.Expr == nil {
				caseStmt.Expr = p.parseExpression(child, source)
			}
		case "case_arm":
			if arm := p.parseCaseArm(child, source); arm != nil {
				caseStmt.Arms = append(caseStmt.Arms, arm)
			}
		}
	}
	
	return caseStmt
}

func (p *NativeParser) parseCaseArm(node *sitter.Node, source []byte) *ast.CaseArm {
	arm := &ast.CaseArm{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "pattern":
			arm.Pattern = p.parsePattern(child, source)
		case "expression":
			if arm.Guard == nil {
				arm.Guard = p.parseExpression(child, source)
			} else {
				// This is the body expression
				arm.Body = p.parseExpression(child, source)
			}
		case "block":
			arm.Body = p.parseBlock(child, source)
		}
	}
	
	return arm
}

func (p *NativeParser) parsePattern(node *sitter.Node, source []byte) ast.Pattern {
	switch node.Type() {
	case "identifier":
		return &ast.IdentifierPattern{
			Name: p.extractText(node, source),
		}
	case "literal_pattern":
		return p.parseLiteralPattern(node, source)
	case "_":
		return &ast.WildcardPattern{}
	case "field_expression":
		// Handle field expressions in patterns
		return &ast.IdentifierPattern{
			Name: p.extractText(node, source),
		}
	default:
		return &ast.IdentifierPattern{
			Name: p.extractText(node, source),
		}
	}
}

func (p *NativeParser) parseLiteralPattern(node *sitter.Node, source []byte) *ast.LiteralPattern {
	pattern := &ast.LiteralPattern{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		pattern.Value = p.parseExpression(child, source)
		break
	}
	
	return pattern
}

func (p *NativeParser) parseAsmBlock(node *sitter.Node, source []byte) *ast.AsmBlockStmt {
	asm := &ast.AsmBlockStmt{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "asm_content" {
			asm.Code = p.extractText(child, source)
			break
		}
	}
	
	return asm
}

func (p *NativeParser) parseCompileTimeAsm(node *sitter.Node, source []byte) *ast.AsmBlockStmt {
	asm := &ast.AsmBlockStmt{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "asm_content" {
			asm.Code = p.extractText(child, source)
			break
		}
	}
	
	return asm
}

func (p *NativeParser) parseMIRBlockStmt(node *sitter.Node, source []byte) *ast.MIRStmt {
	mir := &ast.MIRStmt{
		Instructions: []*ast.MIRInstruction{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "mir_content" {
			mir.Code = p.extractText(child, source)
			break
		}
	}
	
	return mir
}

func (p *NativeParser) parseMinzBlockStmt(node *sitter.Node, source []byte) ast.Statement {
	// Parse minz blocks as expression statements containing string literals
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "minz_raw_block" {
			code := p.extractText(child, source)
			return &ast.ExpressionStmt{
				Expression: &ast.StringLiteral{Value: code},
			}
		}
	}
	return nil
}

func (p *NativeParser) parseTargetBlockStmt(node *sitter.Node, source []byte) *ast.TargetBlockStmt {
	target := &ast.TargetBlockStmt{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "string_literal":
			text := p.extractText(child, source)
			if len(text) >= 2 {
				target.Target = text[1:len(text)-1] // Remove quotes
			}
		case "block":
			target.Body = p.parseBlock(child, source)
		}
	}
	
	return target
}

// Expression parsing functions

func (p *NativeParser) parseUnaryExpression(node *sitter.Node, source []byte) *ast.UnaryExpr {
	unary := &ast.UnaryExpr{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "!", "-", "~", "&", "*":
			unary.Operator = p.extractText(child, source)
		case "expression":
			unary.Operand = p.parseExpression(child, source)
		}
	}
	
	return unary
}

func (p *NativeParser) parsePostfixExpression(node *sitter.Node, source []byte) ast.Expression {
	// Postfix expressions can be call, index, field, try, cast, or primary
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "call_expression":
			return p.parseCallExpression(child, source)
		case "index_expression":
			return p.parseIndexExpression(child, source)
		case "field_expression":
			return p.parseFieldExpression(child, source)
		case "try_expression":
			return p.parseTryExpression(child, source)
		case "cast_expression":
			return p.parseCastExpression(child, source)
		case "primary_expression":
			return p.parsePrimaryExpression(child, source)
		}
	}
	return p.parsePrimaryExpression(node, source)
}

func (p *NativeParser) parseIndexExpression(node *sitter.Node, source []byte) *ast.IndexExpr {
	index := &ast.IndexExpr{}
	
	index.Array = p.parseExpression(node.ChildByFieldName("object"), source)
	index.Index = p.parseExpression(node.ChildByFieldName("index"), source)
	
	return index
}

func (p *NativeParser) parseFieldExpression(node *sitter.Node, source []byte) *ast.FieldExpr {
	field := &ast.FieldExpr{}
	
	field.Object = p.parseExpression(node.ChildByFieldName("object"), source)
	field.Field = p.extractText(node.ChildByFieldName("field"), source)
	
	return field
}

func (p *NativeParser) parseTryExpression(node *sitter.Node, source []byte) *ast.TryExpr {
	try := &ast.TryExpr{}
	
	try.Expression = p.parseExpression(node.ChildByFieldName("expression"), source)
	
	return try
}

func (p *NativeParser) parseCastExpression(node *sitter.Node, source []byte) *ast.CastExpr {
	cast := &ast.CastExpr{}
	
	cast.Expr = p.parseExpression(node.ChildByFieldName("expression"), source)
	cast.TargetType = p.parseType(node.ChildByFieldName("type"), source)
	
	return cast
}

func (p *NativeParser) parsePrimaryExpression(node *sitter.Node, source []byte) ast.Expression {
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		return p.parseExpression(child, source)
	}
	return &ast.Identifier{Name: p.extractText(node, source)}
}

func (p *NativeParser) parseArrayLiteral(node *sitter.Node, source []byte) *ast.ArrayInitializer {
	arr := &ast.ArrayInitializer{
		Elements: []ast.Expression{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "expression" {
			arr.Elements = append(arr.Elements, p.parseExpression(child, source))
		}
	}
	
	return arr
}

func (p *NativeParser) parseArrayInitializer(node *sitter.Node, source []byte) *ast.ArrayInitializer {
	return p.parseArrayLiteral(node, source)
}

func (p *NativeParser) parseStructLiteral(node *sitter.Node, source []byte) *ast.StructLiteral {
	structLit := &ast.StructLiteral{
		Fields: []*ast.FieldInit{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "type_identifier":
			structLit.TypeName = p.extractText(child, source)
		case "field_initializer":
			if field := p.parseFieldInitializer(child, source); field != nil {
				structLit.Fields = append(structLit.Fields, field)
			}
		}
	}
	
	return structLit
}

func (p *NativeParser) parseFieldInitializer(node *sitter.Node, source []byte) *ast.FieldInit {
	field := &ast.FieldInit{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			field.Name = p.extractText(child, source)
		case "expression":
			field.Value = p.parseExpression(child, source)
		}
	}
	
	return field
}

func (p *NativeParser) parseTupleLiteral(node *sitter.Node, source []byte) *ast.ArrayInitializer {
	// For now, treat tuples as arrays
	return p.parseArrayLiteral(node, source)
}

func (p *NativeParser) parseParenthesizedExpression(node *sitter.Node, source []byte) ast.Expression {
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "expression" {
			return p.parseExpression(child, source)
		}
	}
	return nil
}

func (p *NativeParser) parseInlineAssembly(node *sitter.Node, source []byte) *ast.InlineAssembly {
	asm := &ast.InlineAssembly{
		Outputs:  []*ast.AsmOperand{},
		Inputs:   []*ast.AsmOperand{},
		Clobbers: []string{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "string_literal":
			if asm.Code == "" {
				text := p.extractText(child, source)
				if len(text) >= 2 {
					asm.Code = text[1:len(text)-1] // Remove quotes
				}
			}
		}
	}
	
	return asm
}

func (p *NativeParser) parseSizeofExpression(node *sitter.Node, source []byte) *ast.MetafunctionCall {
	meta := &ast.MetafunctionCall{
		Name: "sizeof",
		Arguments: []ast.Expression{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type" {
			// Convert type to identifier for sizeof argument
			typeName := p.extractText(child, source)
			meta.Arguments = append(meta.Arguments, &ast.Identifier{Name: typeName})
		}
	}
	
	return meta
}

func (p *NativeParser) parseAlignofExpression(node *sitter.Node, source []byte) *ast.MetafunctionCall {
	meta := &ast.MetafunctionCall{
		Name: "alignof",
		Arguments: []ast.Expression{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type" {
			// Convert type to identifier for alignof argument
			typeName := p.extractText(child, source)
			meta.Arguments = append(meta.Arguments, &ast.Identifier{Name: typeName})
		}
	}
	
	return meta
}

func (p *NativeParser) parseMetaprogrammingExpression(node *sitter.Node, source []byte) ast.Expression {
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		return p.parseExpression(child, source)
	}
	return nil
}

func (p *NativeParser) parseErrorLiteral(node *sitter.Node, source []byte) *ast.EnumLiteral {
	enum := &ast.EnumLiteral{
		EnumName: "Error",
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			enum.Variant = p.extractText(child, source)
		}
	}
	
	return enum
}

func (p *NativeParser) parseLambdaExpression(node *sitter.Node, source []byte) *ast.LambdaExpr {
	lambda := &ast.LambdaExpr{
		Params:   []*ast.LambdaParam{},
		Captures: []string{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "lambda_parameter_list":
			lambda.Params = p.parseLambdaParameters(child, source)
		case "type":
			lambda.ReturnType = p.parseType(child, source)
		case "expression":
			lambda.Body = p.parseExpression(child, source)
		case "block":
			lambda.Body = p.parseBlock(child, source)
		}
	}
	
	return lambda
}

func (p *NativeParser) parseLambdaParameters(node *sitter.Node, source []byte) []*ast.LambdaParam {
	params := []*ast.LambdaParam{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "lambda_parameter" {
			param := p.parseLambdaParameter(child, source)
			if param != nil {
				params = append(params, param)
			}
		}
	}
	
	return params
}

func (p *NativeParser) parseLambdaParameter(node *sitter.Node, source []byte) *ast.LambdaParam {
	param := &ast.LambdaParam{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			param.Name = p.extractText(child, source)
		case "type":
			param.Type = p.parseType(child, source)
		}
	}
	
	return param
}

func (p *NativeParser) parseIfExpression(node *sitter.Node, source []byte) *ast.IfExpr {
	ifExpr := &ast.IfExpr{}
	
	ifExpr.Condition = p.parseExpression(node.ChildByFieldName("condition"), source)
	ifExpr.ThenBranch = p.parseExpression(node.ChildByFieldName("then_branch"), source)
	if elseBranch := node.ChildByFieldName("else_branch"); elseBranch != nil {
		ifExpr.ElseBranch = p.parseExpression(elseBranch, source)
	}
	
	return ifExpr
}

func (p *NativeParser) parseTernaryExpression(node *sitter.Node, source []byte) *ast.TernaryExpr {
	ternary := &ast.TernaryExpr{}
	
	ternary.TrueExpr = p.parseExpression(node.ChildByFieldName("true_expr"), source)
	ternary.Condition = p.parseExpression(node.ChildByFieldName("condition"), source)
	ternary.FalseExpr = p.parseExpression(node.ChildByFieldName("false_expr"), source)
	
	return ternary
}

func (p *NativeParser) parseWhenExpression(node *sitter.Node, source []byte) *ast.WhenExpr {
	when := &ast.WhenExpr{
		Arms: []*ast.WhenArm{},
	}
	
	when.Value = p.parseExpression(node.ChildByFieldName("value"), source)
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "when_arm" {
			arm := p.parseWhenArm(child, source)
			if arm != nil {
				when.Arms = append(when.Arms, arm)
			}
		}
	}
	
	return when
}

func (p *NativeParser) parseWhenArm(node *sitter.Node, source []byte) *ast.WhenArm {
	arm := &ast.WhenArm{}
	
	arm.Pattern = p.parseExpression(node.ChildByFieldName("pattern"), source)
	arm.Guard = p.parseExpression(node.ChildByFieldName("guard"), source)
	arm.Body = p.parseExpression(node.ChildByFieldName("body"), source)
	
	// Check for 'else' arm
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "else" {
			arm.IsElse = true
			break
		}
	}
	
	return arm
}

// Metaprogramming expression parsers

func (p *NativeParser) parseCompileTimeIf(node *sitter.Node, source []byte) *ast.CompileTimeIf {
	ctif := &ast.CompileTimeIf{}
	
	ctif.Condition = p.parseExpression(node.ChildByFieldName("condition"), source)
	ctif.ThenExpr = p.parseExpression(node.ChildByFieldName("then_code"), source)
	if elseCode := node.ChildByFieldName("else_code"); elseCode != nil {
		ctif.ElseExpr = p.parseExpression(elseCode, source)
	}
	
	return ctif
}

func (p *NativeParser) parseCompileTimePrint(node *sitter.Node, source []byte) *ast.CompileTimePrint {
	print := &ast.CompileTimePrint{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "string_literal" {
			print.Expr = p.parseExpression(child, source)
			break
		}
	}
	
	return print
}

func (p *NativeParser) parseCompileTimeAssert(node *sitter.Node, source []byte) *ast.CompileTimeAssert {
	assert := &ast.CompileTimeAssert{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "expression" {
			if assert.Condition == nil {
				assert.Condition = p.parseExpression(child, source)
			}
		}
	}
	
	return assert
}

func (p *NativeParser) parseCompileTimeError(node *sitter.Node, source []byte) *ast.CompileTimeError {
	error := &ast.CompileTimeError{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "expression" {
			error.ErrorValue = p.parseExpression(child, source)
			break
		}
	}
	
	return error
}

func (p *NativeParser) parseAttribute(node *sitter.Node, source []byte) *ast.Attribute {
	attr := &ast.Attribute{
		Arguments: []ast.Expression{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			attr.Name = p.extractText(child, source)
		case "argument_list":
			attr.Arguments = p.parseArgumentList(child, source)
		}
	}
	
	return attr
}

func (p *NativeParser) parseArgumentList(node *sitter.Node, source []byte) []ast.Expression {
	args := []ast.Expression{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "expression" {
			args = append(args, p.parseExpression(child, source))
		}
	}
	
	return args
}

func (p *NativeParser) parseLuaExpression(node *sitter.Node, source []byte) *ast.LuaExpression {
	lua := &ast.LuaExpression{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "lua_code" {
			lua.Code = p.extractText(child, source)
			break
		}
	}
	
	return lua
}

func (p *NativeParser) parseLuaEval(node *sitter.Node, source []byte) *ast.LuaEval {
	eval := &ast.LuaEval{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "lua_code" {
			eval.Code = p.extractText(child, source)
			break
		}
	}
	
	return eval
}

func (p *NativeParser) parseCompileTimeMinz(node *sitter.Node, source []byte) *ast.CompileTimeMinz {
	minz := &ast.CompileTimeMinz{
		Arguments: []ast.Expression{},
	}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "minz_code_block":
			minz.Code = p.extractText(child, source)
		case "argument_list":
			minz.Arguments = p.parseArgumentList(child, source)
		}
	}
	
	return minz
}

func (p *NativeParser) parseCompileTimeMIR(node *sitter.Node, source []byte) *ast.CompileTimeMIR {
	mir := &ast.CompileTimeMIR{}
	
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "mir_code_block" {
			mir.Code = p.extractText(child, source)
			break
		}
	}
	
	return mir
}

// extractText extracts text content from a node
func (p *NativeParser) extractText(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}