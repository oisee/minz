//go:build antlr
// +build antlr

package parser

import (
	"fmt"
	"os"
	
	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/parser/generated"
)

// AntlrParser implements parsing using ANTLR4
type AntlrParser struct {
	filename string
	source   string
	errors   []error
}

// NewAntlrParser creates a new ANTLR-based parser
func NewAntlrParser() *AntlrParser {
	return &AntlrParser{
		errors: []error{},
	}
}

// ParseFile parses a MinZ source file using ANTLR
func (p *AntlrParser) ParseFile(filename string) (*ast.File, error) {
	// Read source file
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	p.filename = filename
	p.source = string(source)
	
	return p.parseSource()
}

// ParseString parses MinZ source from a string
func (p *AntlrParser) ParseString(source string, context string) ([]ast.Declaration, error) {
	p.filename = context
	p.source = source
	
	file, err := p.parseSource()
	if err != nil {
		return nil, err
	}
	
	return file.Declarations, nil
}

func (p *AntlrParser) parseSource() (*ast.File, error) {
	// Create ANTLR input stream
	input := antlr.NewInputStream(p.source)
	
	// Create lexer
	lexer := generated.NewMinZLexer(input)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(&errorListener{parser: p})
	
	// Create token stream
	stream := antlr.NewCommonTokenStream(lexer, 0)
	
	// Create parser
	parser := generated.NewMinZParser(stream)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(&errorListener{parser: p})
	
	// Enable better error recovery
	parser.BuildParseTrees = true
	
	// Parse the program
	tree := parser.Program()
	
	// Check for syntax errors
	if len(p.errors) > 0 {
		return nil, fmt.Errorf("parse errors: %v", p.errors[0])
	}
	
	// Build AST using visitor
	visitor := &ASTVisitor{
		filename: p.filename,
		source:   p.source,
	}
	
	result := visitor.Visit(tree)
	if result == nil {
		return nil, fmt.Errorf("failed to build AST")
	}
	
	file, ok := result.(*ast.File)
	if !ok {
		return nil, fmt.Errorf("unexpected AST root type: %T", result)
	}
	
	return file, nil
}

// errorListener captures ANTLR parsing errors
type errorListener struct {
	*antlr.DefaultErrorListener
	parser *AntlrParser
}

func (e *errorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, 
	line, column int, msg string, ex antlr.RecognitionException) {
	
	err := fmt.Errorf("syntax error at %s:%d:%d: %s", e.parser.filename, line, column, msg)
	e.parser.errors = append(e.parser.errors, err)
}

// ASTVisitor converts ANTLR parse tree to MinZ AST
type ASTVisitor struct {
	generated.BaseMinZVisitor
	filename string
	source   string
}

// Visit dispatches to the appropriate visit method
func (v *ASTVisitor) Visit(tree antlr.ParseTree) interface{} {
	return tree.Accept(v)
}

// VisitProgram builds the root AST node
func (v *ASTVisitor) VisitProgram(ctx *generated.ProgramContext) interface{} {
	file := &ast.File{
		Name:         v.filename,
		Imports:      []*ast.ImportStmt{},
		Declarations: []ast.Declaration{},
	}
	
	// Process imports
	for _, importCtx := range ctx.AllImportDecl() {
		if imp := v.VisitImportDecl(importCtx.(*generated.ImportDeclContext)); imp != nil {
			file.Imports = append(file.Imports, imp.(*ast.ImportStmt))
		}
	}
	
	// Process declarations
	for _, declCtx := range ctx.AllDeclaration() {
		if decl := v.VisitDeclaration(declCtx.(*generated.DeclarationContext)); decl != nil {
			file.Declarations = append(file.Declarations, decl.(ast.Declaration))
		}
	}
	
	return file
}

// VisitImportDecl handles import statements
func (v *ASTVisitor) VisitImportDecl(ctx *generated.ImportDeclContext) interface{} {
	imp := &ast.ImportStmt{
		StartPos: v.getPosition(ctx.GetStart()),
		EndPos:   v.getPosition(ctx.GetStop()),
	}
	
	// Get import path
	if pathCtx := ctx.ImportPath(); pathCtx != nil {
		imp.Path = v.getImportPath(pathCtx.(*generated.ImportPathContext))
	}
	
	// Get alias if present
	if alias := ctx.GetAlias(); alias != nil {
		imp.Alias = alias.GetText()
	}
	
	return imp
}

// VisitDeclaration dispatches to specific declaration types
func (v *ASTVisitor) VisitDeclaration(ctx *generated.DeclarationContext) interface{} {
	if funcCtx := ctx.FunctionDecl(); funcCtx != nil {
		return v.VisitFunctionDecl(funcCtx.(*generated.FunctionDeclContext))
	}
	if structCtx := ctx.StructDecl(); structCtx != nil {
		return v.VisitStructDecl(structCtx.(*generated.StructDeclContext))
	}
	if enumCtx := ctx.EnumDecl(); enumCtx != nil {
		return v.VisitEnumDecl(enumCtx.(*generated.EnumDeclContext))
	}
	if constCtx := ctx.ConstDecl(); constCtx != nil {
		return v.VisitConstDecl(constCtx.(*generated.ConstDeclContext))
	}
	if globalCtx := ctx.GlobalDecl(); globalCtx != nil {
		return v.VisitGlobalDecl(globalCtx.(*generated.GlobalDeclContext))
	}
	if typeCtx := ctx.TypeAlias(); typeCtx != nil {
		return v.VisitTypeAlias(typeCtx.(*generated.TypeAliasContext))
	}
	if metaCtx := ctx.Metafunction(); metaCtx != nil {
		return v.VisitMetafunction(metaCtx.(*generated.MetafunctionContext))
	}
	
	return nil
}

// VisitFunctionDecl handles function declarations
func (v *ASTVisitor) VisitFunctionDecl(ctx *generated.FunctionDeclContext) interface{} {
	fn := &ast.FunctionDecl{
		Name:     ctx.GetName().GetText(),
		StartPos: v.getPosition(ctx.GetStart()),
		EndPos:   v.getPosition(ctx.GetStop()),
	}
	
	// Check if public
	if ctx.PUB() != nil {
		fn.IsPublic = true
	}
	
	// Process parameters
	if params := ctx.ParameterList(); params != nil {
		fn.Params = v.visitParameterList(params.(*generated.ParameterListContext))
	}
	
	// Process return type
	if ret := ctx.ReturnType(); ret != nil {
		fn.ReturnType = v.visitReturnType(ret.(*generated.ReturnTypeContext))
	}
	
	// Process body
	if body := ctx.FunctionBody(); body != nil {
		if block := body.(*generated.FunctionBodyContext).Block(); block != nil {
			fn.Body = v.VisitBlock(block.(*generated.BlockContext)).(*ast.BlockStmt)
		}
	}
	
	return fn
}

// VisitBlock handles block statements
func (v *ASTVisitor) VisitBlock(ctx *generated.BlockContext) interface{} {
	block := &ast.BlockStmt{
		Statements: []ast.Statement{},
		StartPos:   v.getPosition(ctx.GetStart()),
		EndPos:     v.getPosition(ctx.GetStop()),
	}
	
	for _, stmtCtx := range ctx.AllStatement() {
		if stmt := v.VisitStatement(stmtCtx.(*generated.StatementContext)); stmt != nil {
			block.Statements = append(block.Statements, stmt.(ast.Statement))
		}
	}
	
	return block
}

// Helper methods

func (v *ASTVisitor) getPosition(token antlr.Token) ast.Position {
	if token == nil {
		return ast.Position{}
	}
	return ast.Position{
		Line:   token.GetLine(),
		Column: token.GetColumn() + 1, // ANTLR uses 0-based columns
		Offset: token.GetStart(),
	}
}

func (v *ASTVisitor) getImportPath(ctx *generated.ImportPathContext) string {
	path := ""
	for i, id := range ctx.AllIDENTIFIER() {
		if i > 0 {
			path += "."
		}
		path += id.GetText()
	}
	return path
}

func (v *ASTVisitor) visitParameterList(ctx *generated.ParameterListContext) []*ast.Parameter {
	params := []*ast.Parameter{}
	
	for _, paramCtx := range ctx.AllParameter() {
		param := paramCtx.(*generated.ParameterContext)
		
		// Handle self parameter
		if param.SELF() != nil {
			params = append(params, &ast.Parameter{
				Name:   "self",
				IsSelf: true,
			})
			continue
		}
		
		// Regular parameter
		if name := param.GetName(); name != nil {
			p := &ast.Parameter{
				Name: name.GetText(),
			}
			
			if typeCtx := param.Type(); typeCtx != nil {
				p.Type = v.visitType(typeCtx.(*generated.TypeContext))
			}
			
			params = append(params, p)
		}
	}
	
	return params
}

func (v *ASTVisitor) visitType(ctx *generated.TypeContext) ast.Type {
	// Handle primitive types
	if prim := ctx.PrimitiveType(); prim != nil {
		return &ast.PrimitiveType{
			Name: prim.GetText(),
		}
	}
	
	// Handle named types
	if named := ctx.NamedType(); named != nil {
		return &ast.TypeIdentifier{
			Name: named.GetText(),
		}
	}
	
	// Handle array types
	if array := ctx.ArrayType(); array != nil {
		arrCtx := array.(*generated.ArrayTypeContext)
		elem := v.visitType(arrCtx.Type().(*generated.TypeContext))
		
		arr := &ast.ArrayType{
			ElementType: elem,
		}
		
		// Handle array size
		if size := arrCtx.ArraySize(); size != nil {
			sizeCtx := size.(*generated.ArraySizeContext)
			if intLit := sizeCtx.INTEGER(); intLit != nil {
				// Parse integer size - for now just store as expression
				arr.Size = &ast.IntegerLiteral{
					Value: intLit.GetText(),
				}
			}
		}
		
		return arr
	}
	
	// Handle error types
	if err := ctx.ErrorableType(); err != nil {
		errCtx := err.(*generated.ErrorableTypeContext)
		base := v.visitType(errCtx.Type().(*generated.TypeContext))
		return &ast.ErrorType{
			ValueType: base,
		}
	}
	
	// Default to void
	return &ast.PrimitiveType{Name: "void"}
}

func (v *ASTVisitor) visitReturnType(ctx *generated.ReturnTypeContext) ast.Type {
	if typeCtx := ctx.Type(); typeCtx != nil {
		baseType := v.visitType(typeCtx.(*generated.TypeContext))
		
		// Check for error type suffix
		if ctx.ErrorType() != nil {
			return &ast.ErrorType{
				ValueType: baseType,
			}
		}
		
		return baseType
	}
	
	return &ast.PrimitiveType{Name: "void"}
}

// Stub implementations for other visit methods
func (v *ASTVisitor) VisitStructDecl(ctx *generated.StructDeclContext) interface{} {
	// TODO: Implement struct declaration visitor
	return nil
}

func (v *ASTVisitor) VisitEnumDecl(ctx *generated.EnumDeclContext) interface{} {
	// TODO: Implement enum declaration visitor
	return nil
}

func (v *ASTVisitor) VisitConstDecl(ctx *generated.ConstDeclContext) interface{} {
	// TODO: Implement const declaration visitor
	return nil
}

func (v *ASTVisitor) VisitGlobalDecl(ctx *generated.GlobalDeclContext) interface{} {
	// TODO: Implement global declaration visitor
	return nil
}

func (v *ASTVisitor) VisitTypeAlias(ctx *generated.TypeAliasContext) interface{} {
	// TODO: Implement type alias visitor
	return nil
}

func (v *ASTVisitor) VisitMetafunction(ctx *generated.MetafunctionContext) interface{} {
	// TODO: Implement metafunction visitor
	return nil
}

func (v *ASTVisitor) VisitStatement(ctx *generated.StatementContext) interface{} {
	// TODO: Implement statement visitor
	return nil
}