package parser

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"
	"github.com/minz/minzc/pkg/ast"
	minzparser "github.com/minz/minzc/pkg/parser/minzparser/grammar"
)

// AntlrParser implements parsing using ANTLR4
type AntlrParser struct {
	filename string
	errors   []error
}

// NewAntlrParser creates a new ANTLR-based parser
func NewAntlrParser() *AntlrParser {
	return &AntlrParser{}
}

// ParseFile parses a MinZ source file using ANTLR
func (p *AntlrParser) ParseFile(filename string) (*ast.File, error) {
	p.filename = filename
	p.errors = []error{}

	// Read source file
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Create ANTLR input stream
	input := antlr.NewInputStream(string(source))

	// Create lexer
	lexer := minzparser.NewMinZLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, 0)

	// Create parser
	parser := minzparser.NewMinZParser(stream)

	// Add error listener
	errorListener := &antlrErrorListener{filename: filename, errors: &p.errors}
	parser.RemoveErrorListeners()
	parser.AddErrorListener(errorListener)

	// Parse
	tree := parser.SourceFile()

	// Check for errors
	if len(p.errors) > 0 {
		return nil, p.errors[0]
	}

	// Convert to AST using visitor
	visitor := &antlrVisitor{
		filename: filename,
		source:   string(source),
	}

	file := visitor.VisitSourceFile(tree.(*minzparser.SourceFileContext))
	if file == nil {
		return nil, fmt.Errorf("failed to convert parse tree to AST")
	}

	return file.(*ast.File), nil
}

// antlrErrorListener collects parse errors
type antlrErrorListener struct {
	*antlr.DefaultErrorListener
	filename string
	errors   *[]error
}

func (l *antlrErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	err := fmt.Errorf("%s:%d:%d: %s", l.filename, line, column, msg)
	*l.errors = append(*l.errors, err)
}

// antlrVisitor converts ANTLR parse tree to MinZ AST
type antlrVisitor struct {
	minzparser.BaseMinZVisitor
	filename string
	source   string
}

// VisitSourceFile converts the root node
func (v *antlrVisitor) VisitSourceFile(ctx *minzparser.SourceFileContext) interface{} {
	file := &ast.File{
		Imports:      []*ast.ImportStmt{},
		Declarations: []ast.Declaration{},
	}

	// Process all children
	for _, child := range ctx.AllImportStatement() {
		if imp := v.VisitImportStatement(child.(*minzparser.ImportStatementContext)); imp != nil {
			file.Imports = append(file.Imports, imp.(*ast.ImportStmt))
		}
	}

	for _, child := range ctx.AllDeclaration() {
		if decl := v.VisitDeclaration(child.(*minzparser.DeclarationContext)); decl != nil {
			file.Declarations = append(file.Declarations, decl.(ast.Declaration))
		}
	}

	for _, child := range ctx.AllStatement() {
		if stmt := v.VisitStatement(child.(*minzparser.StatementContext)); stmt != nil {
			// Wrap statements as declarations
			if exprStmt, ok := stmt.(*ast.ExpressionStmt); ok {
				file.Declarations = append(file.Declarations, &ast.ExpressionDecl{
					Expression: exprStmt.Expression,
				})
			}
		}
	}

	return file
}

// VisitImportStatement converts import statements
func (v *antlrVisitor) VisitImportStatement(ctx *minzparser.ImportStatementContext) interface{} {
	imp := &ast.ImportStmt{}

	if pathCtx := ctx.ImportPath(); pathCtx != nil {
		path := v.VisitImportPath(pathCtx.(*minzparser.ImportPathContext))
		if pathStr, ok := path.(string); ok {
			imp.Path = pathStr
		}
	}

	if aliasCtx := ctx.GetAlias(); aliasCtx != nil {
		imp.Alias = aliasCtx.GetText()
	}

	return imp
}

// VisitImportPath converts import paths
func (v *antlrVisitor) VisitImportPath(ctx *minzparser.ImportPathContext) interface{} {
	if strCtx := ctx.StringLiteral(); strCtx != nil {
		return v.extractStringLiteral(strCtx.GetText())
	}

	// Dotted path
	parts := []string{}
	for _, id := range ctx.AllIDENTIFIER() {
		parts = append(parts, id.GetText())
	}
	return strings.Join(parts, ".")
}

// VisitDeclaration routes to specific declaration visitors
func (v *antlrVisitor) VisitDeclaration(ctx *minzparser.DeclarationContext) interface{} {
	if fnCtx := ctx.FunctionDeclaration(); fnCtx != nil {
		return v.VisitFunctionDeclaration(fnCtx.(*minzparser.FunctionDeclarationContext))
	}
	if structCtx := ctx.StructDeclaration(); structCtx != nil {
		return v.VisitStructDeclaration(structCtx.(*minzparser.StructDeclarationContext))
	}
	if enumCtx := ctx.EnumDeclaration(); enumCtx != nil {
		return v.VisitEnumDeclaration(enumCtx.(*minzparser.EnumDeclarationContext))
	}
	if typeCtx := ctx.TypeAliasDeclaration(); typeCtx != nil {
		return v.VisitTypeAliasDeclaration(typeCtx.(*minzparser.TypeAliasDeclarationContext))
	}
	if ifaceCtx := ctx.InterfaceDeclaration(); ifaceCtx != nil {
		return v.VisitInterfaceDeclaration(ifaceCtx.(*minzparser.InterfaceDeclarationContext))
	}
	if implCtx := ctx.ImplBlock(); implCtx != nil {
		return v.VisitImplBlock(implCtx.(*minzparser.ImplBlockContext))
	}
	if constCtx := ctx.ConstDeclaration(); constCtx != nil {
		return v.VisitConstDeclaration(constCtx.(*minzparser.ConstDeclarationContext))
	}
	if globalCtx := ctx.GlobalVarDeclaration(); globalCtx != nil {
		return v.VisitGlobalVarDeclaration(globalCtx.(*minzparser.GlobalVarDeclarationContext))
	}
	if compileTimeCtx := ctx.CompileTimeDeclaration(); compileTimeCtx != nil {
		return v.VisitCompileTimeDeclaration(compileTimeCtx.(*minzparser.CompileTimeDeclarationContext))
	}
	return nil
}

// VisitFunctionDeclaration converts function declarations
func (v *antlrVisitor) VisitFunctionDeclaration(ctx *minzparser.FunctionDeclarationContext) interface{} {
	fn := &ast.FunctionDecl{
		Params: []*ast.Parameter{},
		Body:   &ast.BlockStmt{Statements: []ast.Statement{}},
	}

	// Visibility
	if visCtx := ctx.Visibility(); visCtx != nil {
		fn.IsPublic = true
	}

	// Function prefix (fun, fn, asm fun, mir fun)
	if prefixCtx := ctx.FunctionPrefix(); prefixCtx != nil {
		text := prefixCtx.GetText()
		if strings.Contains(text, "asm") {
			fn.FunctionKind = ast.FunctionKindAsm
		} else if strings.Contains(text, "mir") {
			fn.FunctionKind = ast.FunctionKindMIR
		} else {
			fn.FunctionKind = ast.FunctionKindRegular
		}
	}

	// Name
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		fn.Name = idCtx.GetText()
	}

	// Generic parameters
	if genCtx := ctx.GenericParams(); genCtx != nil {
		fn.GenericParams = v.VisitGenericParams(genCtx.(*minzparser.GenericParamsContext)).([]*ast.GenericParam)
	}

	// Parameters
	if paramCtx := ctx.ParameterList(); paramCtx != nil {
		fn.Params = v.VisitParameterList(paramCtx.(*minzparser.ParameterListContext)).([]*ast.Parameter)
	}

	// Return type
	if retCtx := ctx.ReturnType(); retCtx != nil {
		fn.ReturnType = v.VisitReturnType(retCtx.(*minzparser.ReturnTypeContext)).(ast.Type)
	}

	// Error return type
	if errCtx := ctx.ErrorReturnType(); errCtx != nil {
		if typeCtx := errCtx.Type_(); typeCtx != nil {
			fn.ErrorType = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
		} else {
			// Default error type
			fn.ErrorType = &ast.PrimitiveType{Name: "error"}
		}
	}

	// Body
	if blockCtx := ctx.Block(); blockCtx != nil {
		fn.Body = v.VisitBlock(blockCtx.(*minzparser.BlockContext)).(*ast.BlockStmt)
	}

	return fn
}

// VisitGenericParams converts generic parameters
func (v *antlrVisitor) VisitGenericParams(ctx *minzparser.GenericParamsContext) interface{} {
	params := []*ast.GenericParam{}
	for _, id := range ctx.AllIDENTIFIER() {
		params = append(params, &ast.GenericParam{
			Name: id.GetText(),
		})
	}
	return params
}

// VisitParameterList converts parameter lists
func (v *antlrVisitor) VisitParameterList(ctx *minzparser.ParameterListContext) interface{} {
	params := []*ast.Parameter{}
	for _, paramCtx := range ctx.AllParameter() {
		param := v.VisitParameter(paramCtx.(*minzparser.ParameterContext)).(*ast.Parameter)
		params = append(params, param)
	}
	return params
}

// VisitParameter converts parameters
func (v *antlrVisitor) VisitParameter(ctx *minzparser.ParameterContext) interface{} {
	param := &ast.Parameter{}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		param.Name = idCtx.GetText()
	}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		param.Type = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	return param
}

// VisitReturnType converts return types
func (v *antlrVisitor) VisitReturnType(ctx *minzparser.ReturnTypeContext) interface{} {
	if typeCtx := ctx.Type_(); typeCtx != nil {
		return v.VisitType(typeCtx.(*minzparser.TypeContext))
	}
	return nil
}

// VisitStructDeclaration converts struct declarations
func (v *antlrVisitor) VisitStructDeclaration(ctx *minzparser.StructDeclarationContext) interface{} {
	st := &ast.StructDecl{
		Fields: []*ast.Field{},
	}

	if visCtx := ctx.Visibility(); visCtx != nil {
		st.IsPublic = true
	}

	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		st.Name = idCtx.GetText()
	}

	// Generic parameters - not supported in AST yet
	// if genCtx := ctx.GenericParams(); genCtx != nil {
	//     st.GenericParams = v.VisitGenericParams(genCtx.(*minzparser.GenericParamsContext)).([]*ast.GenericParam)
	// }

	if fieldsCtx := ctx.FieldList(); fieldsCtx != nil {
		st.Fields = v.VisitFieldList(fieldsCtx.(*minzparser.FieldListContext)).([]*ast.Field)
	}

	return st
}

// VisitFieldList converts field lists
func (v *antlrVisitor) VisitFieldList(ctx *minzparser.FieldListContext) interface{} {
	fields := []*ast.Field{}
	for _, fieldCtx := range ctx.AllField() {
		field := v.VisitField(fieldCtx.(*minzparser.FieldContext)).(*ast.Field)
		fields = append(fields, field)
	}
	return fields
}

// VisitField converts fields
func (v *antlrVisitor) VisitField(ctx *minzparser.FieldContext) interface{} {
	field := &ast.Field{}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		field.Name = idCtx.GetText()
	}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		field.Type = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	return field
}

// VisitEnumDeclaration converts enum declarations
func (v *antlrVisitor) VisitEnumDeclaration(ctx *minzparser.EnumDeclarationContext) interface{} {
	enum := &ast.EnumDecl{
		Variants: []string{},
	}

	if visCtx := ctx.Visibility(); visCtx != nil {
		enum.IsPublic = true
	}

	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		enum.Name = idCtx.GetText()
	}

	if membersCtx := ctx.EnumMemberList(); membersCtx != nil {
		enum.Variants = v.VisitEnumMemberList(membersCtx.(*minzparser.EnumMemberListContext)).([]string)
	}

	return enum
}

// VisitEnumMemberList converts enum member lists
func (v *antlrVisitor) VisitEnumMemberList(ctx *minzparser.EnumMemberListContext) interface{} {
	members := []string{}
	for _, memberCtx := range ctx.AllEnumMember() {
		if idCtx := memberCtx.IDENTIFIER(); idCtx != nil {
			members = append(members, idCtx.GetText())
		}
	}
	return members
}

// VisitTypeAliasDeclaration converts type aliases
func (v *antlrVisitor) VisitTypeAliasDeclaration(ctx *minzparser.TypeAliasDeclarationContext) interface{} {
	alias := &ast.TypeDecl{}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		alias.Name = idCtx.GetText()
	}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		alias.Type = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	return alias
}

// VisitInterfaceDeclaration converts interface declarations
func (v *antlrVisitor) VisitInterfaceDeclaration(ctx *minzparser.InterfaceDeclarationContext) interface{} {
	iface := &ast.InterfaceDecl{
		Methods: []*ast.InterfaceMethod{},
	}

	if visCtx := ctx.Visibility(); visCtx != nil {
		iface.IsPublic = true
	}

	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		iface.Name = idCtx.GetText()
	}

	if methodsCtx := ctx.InterfaceMethodList(); methodsCtx != nil {
		for _, methodCtx := range methodsCtx.(*minzparser.InterfaceMethodListContext).AllInterfaceMethod() {
			method := v.VisitInterfaceMethod(methodCtx.(*minzparser.InterfaceMethodContext)).(*ast.InterfaceMethod)
			iface.Methods = append(iface.Methods, method)
		}
	}

	return iface
}

// VisitInterfaceMethod converts interface methods
func (v *antlrVisitor) VisitInterfaceMethod(ctx *minzparser.InterfaceMethodContext) interface{} {
	method := &ast.InterfaceMethod{}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		method.Name = idCtx.GetText()
	}
	
	if paramCtx := ctx.ParameterList(); paramCtx != nil {
		method.Params = v.VisitParameterList(paramCtx.(*minzparser.ParameterListContext)).([]*ast.Parameter)
	}
	
	if retCtx := ctx.ReturnType(); retCtx != nil {
		method.ReturnType = v.VisitReturnType(retCtx.(*minzparser.ReturnTypeContext)).(ast.Type)
	}
	
	return method
}

// VisitImplBlock converts impl blocks
func (v *antlrVisitor) VisitImplBlock(ctx *minzparser.ImplBlockContext) interface{} {
	impl := &ast.ImplBlock{
		Methods: []*ast.FunctionDecl{},
	}
	
	// Get the interface and target type
	types := ctx.AllType_()
	if len(types) >= 2 {
		// impl.Trait = v.VisitType(types[0].(*minzparser.TypeContext)).(ast.Type)
		// impl.Target = v.VisitType(types[1].(*minzparser.TypeContext)).(ast.Type)
		// TODO: Update AST to support Trait and Target fields
	}
	
	// Get methods
	for _, fnCtx := range ctx.AllFunctionDeclaration() {
		fn := v.VisitFunctionDeclaration(fnCtx.(*minzparser.FunctionDeclarationContext)).(*ast.FunctionDecl)
		impl.Methods = append(impl.Methods, fn)
	}
	
	return impl
}

// VisitConstDeclaration converts const declarations
func (v *antlrVisitor) VisitConstDeclaration(ctx *minzparser.ConstDeclarationContext) interface{} {
	c := &ast.ConstDecl{}
	
	if visCtx := ctx.Visibility(); visCtx != nil {
		c.IsPublic = true
	}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		c.Name = idCtx.GetText()
	}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		c.Type = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	if exprCtx := ctx.Expression(); exprCtx != nil {
		c.Value = v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression)
	}
	
	return c
}

// VisitGlobalVarDeclaration converts global variable declarations
func (v *antlrVisitor) VisitGlobalVarDeclaration(ctx *minzparser.GlobalVarDeclarationContext) interface{} {
	g := &ast.VarDecl{
		IsPublic: true, // Globals are public by default
	}
	
	if visCtx := ctx.Visibility(); visCtx != nil {
		g.IsPublic = true
	}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		g.Name = idCtx.GetText()
	}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		g.Type = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	if exprCtx := ctx.Expression(); exprCtx != nil {
		g.Value = v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression)
	}
	
	return g
}

// VisitCompileTimeDeclaration converts compile-time declarations
func (v *antlrVisitor) VisitCompileTimeDeclaration(ctx *minzparser.CompileTimeDeclarationContext) interface{} {
	if ifCtx := ctx.CompileTimeIf(); ifCtx != nil {
		return v.VisitCompileTimeIf(ifCtx.(*minzparser.CompileTimeIfContext))
	}
	if minzCtx := ctx.CompileTimeMinz(); minzCtx != nil {
		return v.VisitCompileTimeMinz(minzCtx.(*minzparser.CompileTimeMinzContext))
	}
	if mirCtx := ctx.CompileTimeMir(); mirCtx != nil {
		return v.VisitCompileTimeMir(mirCtx.(*minzparser.CompileTimeMirContext))
	}
	if targetCtx := ctx.TargetBlock(); targetCtx != nil {
		return v.VisitTargetBlock(targetCtx.(*minzparser.TargetBlockContext))
	}
	return nil
}

// VisitCompileTimeIf converts @if declarations
func (v *antlrVisitor) VisitCompileTimeIf(ctx *minzparser.CompileTimeIfContext) interface{} {
	ctIf := &ast.CompileTimeIf{}
	
	if exprCtx := ctx.Expression(); exprCtx != nil {
		ctIf.Condition = v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression)
	}
	
	blocks := ctx.AllBlock()
	if len(blocks) > 0 {
		ctIf.ThenExpr = v.VisitBlock(blocks[0].(*minzparser.BlockContext)).(*ast.BlockStmt)
	}
	if len(blocks) > 1 {
		ctIf.ElseExpr = v.VisitBlock(blocks[1].(*minzparser.BlockContext)).(*ast.BlockStmt)
	}
	
	return ctIf
}

// VisitCompileTimeMinz converts @minz declarations
func (v *antlrVisitor) VisitCompileTimeMinz(ctx *minzparser.CompileTimeMinzContext) interface{} {
	minz := &ast.MinzMetafunctionCall{
		Arguments: []ast.Expression{},
	}
	
	if strCtx := ctx.StringLiteral(); strCtx != nil {
		// minz.Template = &ast.StringLiteral{
		//     Value: v.extractStringLiteral(strCtx.GetText()),
		// }
		// TODO: Update AST to support Template field
	}
	
	for _, exprCtx := range ctx.AllExpression() {
		expr := v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression)
		minz.Arguments = append(minz.Arguments, expr)
	}
	
	return minz
}

// VisitCompileTimeMir converts @mir blocks
func (v *antlrVisitor) VisitCompileTimeMir(ctx *minzparser.CompileTimeMirContext) interface{} {
	if mirCtx := ctx.MirBlock(); mirCtx != nil {
		return v.VisitMirBlock(mirCtx.(*minzparser.MirBlockContext))
	}
	return nil
}

// VisitMirBlock converts MIR blocks
func (v *antlrVisitor) VisitMirBlock(ctx *minzparser.MirBlockContext) interface{} {
	block := &ast.MIRBlock{
		Code: "", // MIR blocks are stored as raw strings in AST
	}
	
	// Convert MIR statements to a string representation
	var mirCode []string
	for _, stmtCtx := range ctx.AllMirStatement() {
		if instrCtx := stmtCtx.(*minzparser.MirStatementContext).MirInstruction(); instrCtx != nil {
			// For now, just get the text of the instruction
			mirCode = append(mirCode, instrCtx.GetText())
		}
	}
	
	block.Code = strings.Join(mirCode, "\n")
	return block
}

// VisitMirInstruction converts MIR instructions
func (v *antlrVisitor) VisitMirInstruction(ctx *minzparser.MirInstructionContext) interface{} {
	instr := &ast.MIRInstruction{
		Operands: []ast.MIROperand{},
	}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		instr.Opcode = idCtx.GetText()
	}
	
	for _, opCtx := range ctx.AllMirOperand() {
		op := v.VisitMirOperand(opCtx.(*minzparser.MirOperandContext)).(ast.MIROperand)
		instr.Operands = append(instr.Operands, op)
	}
	
	return instr
}

// VisitMirOperand converts MIR operands
func (v *antlrVisitor) VisitMirOperand(ctx *minzparser.MirOperandContext) interface{} {
	if regCtx := ctx.MirRegister(); regCtx != nil {
		return v.VisitMirRegister(regCtx.(*minzparser.MirRegisterContext))
	}
	if immCtx := ctx.MirImmediate(); immCtx != nil {
		return v.VisitMirImmediate(immCtx.(*minzparser.MirImmediateContext))
	}
	if memCtx := ctx.MirMemory(); memCtx != nil {
		return v.VisitMirMemory(memCtx.(*minzparser.MirMemoryContext))
	}
	if labelCtx := ctx.MirLabel(); labelCtx != nil {
		return v.VisitMirLabel(labelCtx.(*minzparser.MirLabelContext))
	}
	return nil
}

// VisitMirRegister converts MIR registers
func (v *antlrVisitor) VisitMirRegister(ctx *minzparser.MirRegisterContext) interface{} {
	if numCtx := ctx.NUMBER(); numCtx != nil {
		num, _ := strconv.Atoi(numCtx.GetText())
		return &ast.MIRRegister{Number: num}
	}
	return nil
}

// VisitMirImmediate converts MIR immediates
func (v *antlrVisitor) VisitMirImmediate(ctx *minzparser.MirImmediateContext) interface{} {
	if numCtx := ctx.NUMBER(); numCtx != nil {
		num, _ := strconv.ParseInt(numCtx.GetText(), 10, 64)
		return &ast.MIRImmediate{Value: num}
	}
	return nil
}

// VisitMirMemory converts MIR memory references
func (v *antlrVisitor) VisitMirMemory(ctx *minzparser.MirMemoryContext) interface{} {
	if exprCtx := ctx.Expression(); exprCtx != nil {
		// MIRMemory doesn't have Address field in current AST
		return &ast.MIRMemory{}
	}
	return nil
}

// VisitMirLabel converts MIR labels
func (v *antlrVisitor) VisitMirLabel(ctx *minzparser.MirLabelContext) interface{} {
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		return &ast.MIRLabel{Name: idCtx.GetText()}
	}
	return nil
}

// VisitTargetBlock converts @target blocks
func (v *antlrVisitor) VisitTargetBlock(ctx *minzparser.TargetBlockContext) interface{} {
	target := &ast.TargetBlock{}
	
	if strCtx := ctx.StringLiteral(); strCtx != nil {
		target.Target = v.extractStringLiteral(strCtx.GetText())
	}
	
	if blockCtx := ctx.Block(); blockCtx != nil {
		target.Body = v.VisitBlock(blockCtx.(*minzparser.BlockContext)).(*ast.BlockStmt)
	}
	
	return target
}

// VisitStatement routes to specific statement visitors
func (v *antlrVisitor) VisitStatement(ctx *minzparser.StatementContext) interface{} {
	if letCtx := ctx.LetStatement(); letCtx != nil {
		return v.VisitLetStatement(letCtx.(*minzparser.LetStatementContext))
	}
	if varCtx := ctx.VarStatement(); varCtx != nil {
		return v.VisitVarStatement(varCtx.(*minzparser.VarStatementContext))
	}
	if assignCtx := ctx.AssignmentStatement(); assignCtx != nil {
		return v.VisitAssignmentStatement(assignCtx.(*minzparser.AssignmentStatementContext))
	}
	if exprCtx := ctx.ExpressionStatement(); exprCtx != nil {
		return v.VisitExpressionStatement(exprCtx.(*minzparser.ExpressionStatementContext))
	}
	if retCtx := ctx.ReturnStatement(); retCtx != nil {
		return v.VisitReturnStatement(retCtx.(*minzparser.ReturnStatementContext))
	}
	if ifCtx := ctx.IfStatement(); ifCtx != nil {
		return v.VisitIfStatement(ifCtx.(*minzparser.IfStatementContext))
	}
	if whileCtx := ctx.WhileStatement(); whileCtx != nil {
		return v.VisitWhileStatement(whileCtx.(*minzparser.WhileStatementContext))
	}
	if forCtx := ctx.ForStatement(); forCtx != nil {
		return v.VisitForStatement(forCtx.(*minzparser.ForStatementContext))
	}
	if loopCtx := ctx.LoopStatement(); loopCtx != nil {
		return v.VisitLoopStatement(loopCtx.(*minzparser.LoopStatementContext))
	}
	if caseCtx := ctx.CaseStatement(); caseCtx != nil {
		return v.VisitCaseStatement(caseCtx.(*minzparser.CaseStatementContext))
	}
	if blockCtx := ctx.BlockStatement(); blockCtx != nil {
		return v.VisitBlockStatement(blockCtx.(*minzparser.BlockStatementContext))
	}
	if breakCtx := ctx.BreakStatement(); breakCtx != nil {
		// Break statements not yet in AST
		return &ast.ExpressionStmt{}
	}
	if continueCtx := ctx.ContinueStatement(); continueCtx != nil {
		// Continue statements not yet in AST
		return &ast.ExpressionStmt{}
	}
	if deferCtx := ctx.DeferStatement(); deferCtx != nil {
		return v.VisitDeferStatement(deferCtx.(*minzparser.DeferStatementContext))
	}
	if asmCtx := ctx.AsmStatement(); asmCtx != nil {
		return v.VisitAsmStatement(asmCtx.(*minzparser.AsmStatementContext))
	}
	return nil
}

// VisitLetStatement converts let statements
func (v *antlrVisitor) VisitLetStatement(ctx *minzparser.LetStatementContext) interface{} {
	let := &ast.VarDecl{}
	
	if ctx.GetText() != "" && strings.Contains(ctx.GetText(), "mut") {
		let.IsMutable = true
	}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		let.Name = idCtx.GetText()
	}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		let.Type = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	if exprCtx := ctx.Expression(); exprCtx != nil {
		let.Value = v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression)
	}
	
	return let
}

// VisitVarStatement converts var statements
func (v *antlrVisitor) VisitVarStatement(ctx *minzparser.VarStatementContext) interface{} {
	varDecl := &ast.VarDecl{
		IsMutable: true, // var is always mutable
	}
	
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		varDecl.Name = idCtx.GetText()
	}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		varDecl.Type = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	if exprCtx := ctx.Expression(); exprCtx != nil {
		varDecl.Value = v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression)
	}
	
	return varDecl
}

// Continue with remaining visitor methods...
// [The rest of the implementation would follow the same pattern]

// Helper methods

func (v *antlrVisitor) extractStringLiteral(text string) string {
	// Remove quotes and handle escape sequences
	if len(text) >= 2 {
		// Remove L prefix if present
		if text[0] == 'L' || text[0] == 'l' {
			text = text[1:]
		}
		// Remove quotes
		text = text[1 : len(text)-1]
		// TODO: Handle escape sequences properly
	}
	return text
}

// Placeholder for remaining visitor methods
// These would need to be implemented following the same pattern:
// - VisitAssignmentStatement
// - VisitExpressionStatement
// - VisitReturnStatement
// - VisitIfStatement
// - VisitWhileStatement
// - VisitForStatement
// - VisitLoopStatement
// - VisitCaseStatement
// - VisitBlockStatement
// - VisitDeferStatement
// - VisitAsmStatement
// - VisitBlock
// - VisitExpression (and all expression types)
// - VisitType (and all type variants)
// - VisitPattern (and all pattern types)

// For now, stub implementations:

func (v *antlrVisitor) VisitAssignmentStatement(ctx *minzparser.AssignmentStatementContext) interface{} {
	return &ast.AssignStmt{}
}

func (v *antlrVisitor) VisitExpressionStatement(ctx *minzparser.ExpressionStatementContext) interface{} {
	if exprCtx := ctx.Expression(); exprCtx != nil {
		return &ast.ExpressionStmt{
			Expression: v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression),
		}
	}
	return nil
}

func (v *antlrVisitor) VisitReturnStatement(ctx *minzparser.ReturnStatementContext) interface{} {
	ret := &ast.ReturnStmt{}
	if exprCtx := ctx.Expression(); exprCtx != nil {
		ret.Value = v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression)
	}
	return ret
}

func (v *antlrVisitor) VisitIfStatement(ctx *minzparser.IfStatementContext) interface{} {
	stmt := &ast.IfStmt{}
	
	// Parse condition
	if condCtx := ctx.Expression(); condCtx != nil {
		if cond := v.VisitExpression(condCtx.(*minzparser.ExpressionContext)); cond != nil {
			stmt.Condition = cond.(ast.Expression)
		}
	}
	
	// Parse then block
	blocks := ctx.AllBlock()
	if len(blocks) > 0 {
		if thenBlock := v.VisitBlock(blocks[0].(*minzparser.BlockContext)); thenBlock != nil {
			stmt.Then = thenBlock.(*ast.BlockStmt)
		}
	}
	
	// Parse else block (if exists)
	if len(blocks) > 1 {
		if elseBlock := v.VisitBlock(blocks[1].(*minzparser.BlockContext)); elseBlock != nil {
			stmt.Else = elseBlock.(*ast.BlockStmt)
		}
	} else if ctx.IfStatement() != nil {
		// else if case
		if elseIf := v.VisitIfStatement(ctx.IfStatement().(*minzparser.IfStatementContext)); elseIf != nil {
			stmt.Else = &ast.BlockStmt{
				Statements: []ast.Statement{elseIf.(ast.Statement)},
			}
		}
	}
	
	return stmt
}

func (v *antlrVisitor) VisitWhileStatement(ctx *minzparser.WhileStatementContext) interface{} {
	stmt := &ast.WhileStmt{}
	
	// Parse condition
	if condCtx := ctx.Expression(); condCtx != nil {
		if cond := v.VisitExpression(condCtx.(*minzparser.ExpressionContext)); cond != nil {
			stmt.Condition = cond.(ast.Expression)
		}
	}
	
	// Parse body
	if blockCtx := ctx.Block(); blockCtx != nil {
		if body := v.VisitBlock(blockCtx.(*minzparser.BlockContext)); body != nil {
			stmt.Body = body.(*ast.BlockStmt)
		}
	}
	
	return stmt
}

func (v *antlrVisitor) VisitForStatement(ctx *minzparser.ForStatementContext) interface{} {
	stmt := &ast.ForStmt{}
	
	// Parse iterator variable
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		stmt.Iterator = idCtx.GetText()
	}
	
	// Parse range expression
	if exprCtx := ctx.Expression(); exprCtx != nil {
		if expr := v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)); expr != nil {
			stmt.Range = expr.(ast.Expression)
		}
	}
	
	// Parse body
	if blockCtx := ctx.Block(); blockCtx != nil {
		if body := v.VisitBlock(blockCtx.(*minzparser.BlockContext)); body != nil {
			stmt.Body = body.(*ast.BlockStmt)
		}
	}
	
	return stmt
}

func (v *antlrVisitor) VisitLoopStatement(ctx *minzparser.LoopStatementContext) interface{} {
	stmt := &ast.LoopStmt{}
	
	// Parse body
	if blockCtx := ctx.Block(); blockCtx != nil {
		if body := v.VisitBlock(blockCtx.(*minzparser.BlockContext)); body != nil {
			stmt.Body = body.(*ast.BlockStmt)
		}
	}
	
	return stmt
}

func (v *antlrVisitor) VisitCaseStatement(ctx *minzparser.CaseStatementContext) interface{} {
	stmt := &ast.CaseStmt{}
	
	// Parse expression to match
	if exprCtx := ctx.Expression(); exprCtx != nil {
		if expr := v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)); expr != nil {
			stmt.Expr = expr.(ast.Expression)
		}
	}
	
	// Parse case arms
	stmt.Arms = make([]*ast.CaseArm, 0)
	for _, armCtx := range ctx.AllCaseArm() {
		if arm := v.VisitCaseArm(armCtx.(*minzparser.CaseArmContext)); arm != nil {
			stmt.Arms = append(stmt.Arms, arm.(*ast.CaseArm))
		}
	}
	
	return stmt
}

func (v *antlrVisitor) VisitCaseArm(ctx *minzparser.CaseArmContext) interface{} {
	arm := &ast.CaseArm{}
	
	// Parse pattern
	if patternCtx := ctx.Pattern(); patternCtx != nil {
		if pattern := v.VisitPattern(patternCtx.(*minzparser.PatternContext)); pattern != nil {
			arm.Pattern = pattern.(ast.Pattern)
		}
	}
	
	// Parse body - check for expression or block
	if exprCtx := ctx.Expression(); exprCtx != nil {
		if body := v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)); body != nil {
			arm.Body = body.(ast.Expression)
		}
	}
	
	// Also check for block body
	if blockCtx := ctx.Block(); blockCtx != nil {
		if block := v.VisitBlock(blockCtx.(*minzparser.BlockContext)); block != nil {
			arm.Body = block.(ast.Statement)
		}
	}
	
	return arm
}

func (v *antlrVisitor) VisitPattern(ctx *minzparser.PatternContext) interface{} {
	// Handle different pattern types
	text := ctx.GetText()
	if text == "_" {
		return &ast.WildcardPattern{}
	}
	
	// Check for literal pattern
	if litCtx := ctx.LiteralPattern(); litCtx != nil {
		return v.VisitLiteralPattern(litCtx.(*minzparser.LiteralPatternContext))
	}
	
	// Default to identifier pattern
	return &ast.IdentifierPattern{
		Name: text,
	}
}

func (v *antlrVisitor) VisitLiteralPattern(ctx *minzparser.LiteralPatternContext) interface{} {
	// Get the text and parse it as a literal
	text := ctx.GetText()
	
	// Try to parse as number
	if val, err := strconv.ParseInt(text, 0, 64); err == nil {
		return &ast.LiteralPattern{
			Value: &ast.NumberLiteral{
				Value: val,
			},
		}
	}
	
	// Try to parse as boolean
	if text == "true" || text == "false" {
		return &ast.LiteralPattern{
			Value: &ast.BooleanLiteral{
				Value: text == "true",
			},
		}
	}
	
	// Try to parse as string (with quotes)
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		return &ast.LiteralPattern{
			Value: &ast.StringLiteral{
				Value: text[1:len(text)-1],
			},
		}
	}
	
	// Try to parse as char (with single quotes)
	if len(text) >= 3 && text[0] == '\'' && text[len(text)-1] == '\'' {
		char := text[1:len(text)-1]
		if len(char) > 0 {
			return &ast.LiteralPattern{
				Value: &ast.NumberLiteral{
					Value: int64(char[0]),
				},
			}
		}
	}
	
	// Default to identifier pattern
	return &ast.IdentifierPattern{
		Name: text,
	}
}

func (v *antlrVisitor) VisitBlockStatement(ctx *minzparser.BlockStatementContext) interface{} {
	if blockCtx := ctx.Block(); blockCtx != nil {
		return v.VisitBlock(blockCtx.(*minzparser.BlockContext))
	}
	return nil
}

func (v *antlrVisitor) VisitDeferStatement(ctx *minzparser.DeferStatementContext) interface{} {
	// Defer statements not yet in AST
	return &ast.ExpressionStmt{}
}

func (v *antlrVisitor) VisitAsmStatement(ctx *minzparser.AsmStatementContext) interface{} {
	return &ast.AsmBlockStmt{}
}

func (v *antlrVisitor) VisitBlock(ctx *minzparser.BlockContext) interface{} {
	block := &ast.BlockStmt{
		Statements: []ast.Statement{},
	}
	
	for _, stmtCtx := range ctx.AllStatement() {
		if stmt := v.VisitStatement(stmtCtx.(*minzparser.StatementContext)); stmt != nil {
			block.Statements = append(block.Statements, stmt.(ast.Statement))
		}
	}
	
	return block
}

func (v *antlrVisitor) VisitExpression(ctx *minzparser.ExpressionContext) interface{} {
	// Route to specific expression types
	if lambdaCtx := ctx.LambdaExpression(); lambdaCtx != nil {
		result := v.VisitLambdaExpression(lambdaCtx.(*minzparser.LambdaExpressionContext))
		if result == nil && debug {
			fmt.Printf("DEBUG: VisitLambdaExpression returned nil for: %s\n", ctx.GetText())
		}
		return result
	}
	if condCtx := ctx.ConditionalExpression(); condCtx != nil {
		result := v.VisitConditionalExpression(condCtx.(*minzparser.ConditionalExpressionContext))
		if result == nil && debug {
			fmt.Printf("DEBUG: VisitConditionalExpression returned nil for: %s\n", ctx.GetText())
		}
		return result
	}
	
	// This shouldn't happen - expression should always have lambda or conditional
	// But as a fallback, try to parse the text
	text := ctx.GetText()
	if debug {
		fmt.Printf("DEBUG: VisitExpression fallback for text: %s\n", text)
	}
	if text != "" {
		// Try to determine what kind of expression this is
		if num, err := strconv.ParseInt(text, 10, 64); err == nil {
			return &ast.NumberLiteral{Value: num}
		}
		// Default to identifier
		return &ast.Identifier{Name: text}
	}
	
	return nil
}

// VisitLambdaExpression converts lambda expressions
func (v *antlrVisitor) VisitLambdaExpression(ctx *minzparser.LambdaExpressionContext) interface{} {
	if condCtx := ctx.ConditionalExpression(); condCtx != nil {
		return v.VisitConditionalExpression(condCtx.(*minzparser.ConditionalExpressionContext))
	}
	
	// Handle lambda with parameters
	lambda := &ast.LambdaExpr{
		Params: []*ast.LambdaParam{},
	}
	
	if paramsCtx := ctx.LambdaParams(); paramsCtx != nil {
		for _, paramCtx := range paramsCtx.(*minzparser.LambdaParamsContext).AllLambdaParam() {
			param := &ast.LambdaParam{}
			if idCtx := paramCtx.(*minzparser.LambdaParamContext).IDENTIFIER(); idCtx != nil {
				param.Name = idCtx.GetText()
			}
			if typeCtx := paramCtx.(*minzparser.LambdaParamContext).Type_(); typeCtx != nil {
				param.Type = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
			}
			lambda.Params = append(lambda.Params, param)
		}
	}
	
	if blockCtx := ctx.Block(); blockCtx != nil {
		lambda.Body = v.VisitBlock(blockCtx.(*minzparser.BlockContext)).(*ast.BlockStmt)
	}
	
	return lambda
}

// VisitConditionalExpression handles ternary and conditional expressions
func (v *antlrVisitor) VisitConditionalExpression(ctx *minzparser.ConditionalExpressionContext) interface{} {
	if whenCtx := ctx.WhenExpression(); whenCtx != nil {
		return v.VisitWhenExpression(whenCtx.(*minzparser.WhenExpressionContext))
	}
	
	// Check for ternary operator (? :) or if-then-else
	if ctx.GetChildCount() >= 5 {
		if orCtx := ctx.LogicalOrExpression(); orCtx != nil {
			cond := v.VisitLogicalOrExpression(orCtx.(*minzparser.LogicalOrExpressionContext)).(ast.Expression)
			
			// Check if we have expression nodes for then/else
			exprs := ctx.AllExpression()
			if len(exprs) >= 2 {
				thenExpr := v.VisitExpression(exprs[0].(*minzparser.ExpressionContext)).(ast.Expression)
				elseExpr := v.VisitExpression(exprs[1].(*minzparser.ExpressionContext)).(ast.Expression)
				
				return &ast.TernaryExpr{
					Condition: cond,
					TrueExpr:  thenExpr,
					FalseExpr: elseExpr,
				}
			}
		}
	}
	
	// No ternary/conditional, just return the logical or expression
	if orCtx := ctx.LogicalOrExpression(); orCtx != nil {
		return v.VisitLogicalOrExpression(orCtx.(*minzparser.LogicalOrExpressionContext))
	}
	
	// This shouldn't happen
	return nil
}

// VisitWhenExpression handles pattern matching when expressions
func (v *antlrVisitor) VisitWhenExpression(ctx *minzparser.WhenExpressionContext) interface{} {
	// TODO: Implement when expression
	return &ast.Identifier{Name: "when"}
}

// VisitLogicalOrExpression handles || and 'or' operators
func (v *antlrVisitor) VisitLogicalOrExpression(ctx *minzparser.LogicalOrExpressionContext) interface{} {
	if andCtxList := ctx.AllLogicalAndExpression(); len(andCtxList) > 1 {
		left := v.VisitLogicalAndExpression(andCtxList[0].(*minzparser.LogicalAndExpressionContext)).(ast.Expression)
		for i := 1; i < len(andCtxList); i++ {
			right := v.VisitLogicalAndExpression(andCtxList[i].(*minzparser.LogicalAndExpressionContext)).(ast.Expression)
			left = &ast.BinaryExpr{
				Left:     left,
				Operator: "||",
				Right:    right,
			}
		}
		return left
	}
	
	if andCtx := ctx.LogicalAndExpression(0); andCtx != nil {
		return v.VisitLogicalAndExpression(andCtx.(*minzparser.LogicalAndExpressionContext))
	}
	
	return &ast.Identifier{Name: "or"}
}

// VisitLogicalAndExpression handles && and 'and' operators
func (v *antlrVisitor) VisitLogicalAndExpression(ctx *minzparser.LogicalAndExpressionContext) interface{} {
	if eqCtxList := ctx.AllEqualityExpression(); len(eqCtxList) > 1 {
		left := v.VisitEqualityExpression(eqCtxList[0].(*minzparser.EqualityExpressionContext)).(ast.Expression)
		for i := 1; i < len(eqCtxList); i++ {
			right := v.VisitEqualityExpression(eqCtxList[i].(*minzparser.EqualityExpressionContext)).(ast.Expression)
			left = &ast.BinaryExpr{
				Left:     left,
				Operator: "&&",
				Right:    right,
			}
		}
		return left
	}
	
	if eqCtx := ctx.EqualityExpression(0); eqCtx != nil {
		return v.VisitEqualityExpression(eqCtx.(*minzparser.EqualityExpressionContext))
	}
	
	return &ast.Identifier{Name: "and"}
}

// VisitEqualityExpression handles == and != operators
func (v *antlrVisitor) VisitEqualityExpression(ctx *minzparser.EqualityExpressionContext) interface{} {
	if relCtxList := ctx.AllRelationalExpression(); len(relCtxList) > 1 {
		left := v.VisitRelationalExpression(relCtxList[0].(*minzparser.RelationalExpressionContext)).(ast.Expression)
		for i := 1; i < len(relCtxList); i++ {
			right := v.VisitRelationalExpression(relCtxList[i].(*minzparser.RelationalExpressionContext)).(ast.Expression)
			// Get the operator token between expressions
			operator := "=="
			if i*2-1 < ctx.GetChildCount() {
				if op := ctx.GetChild(i*2 - 1); op != nil {
					operator = op.(antlr.TerminalNode).GetText()
				}
			}
			left = &ast.BinaryExpr{
				Left:     left,
				Operator: operator,
				Right:    right,
			}
		}
		return left
	}
	
	if relCtx := ctx.RelationalExpression(0); relCtx != nil {
		return v.VisitRelationalExpression(relCtx.(*minzparser.RelationalExpressionContext))
	}
	
	return &ast.Identifier{Name: "eq"}
}

// VisitRelationalExpression handles <, >, <=, >= operators
func (v *antlrVisitor) VisitRelationalExpression(ctx *minzparser.RelationalExpressionContext) interface{} {
	if addCtxList := ctx.AllAdditiveExpression(); len(addCtxList) > 1 {
		left := v.VisitAdditiveExpression(addCtxList[0].(*minzparser.AdditiveExpressionContext)).(ast.Expression)
		for i := 1; i < len(addCtxList); i++ {
			right := v.VisitAdditiveExpression(addCtxList[i].(*minzparser.AdditiveExpressionContext)).(ast.Expression)
			// Get the operator token
			operator := "<"
			if i*2-1 < ctx.GetChildCount() {
				if op := ctx.GetChild(i*2 - 1); op != nil {
					operator = op.(antlr.TerminalNode).GetText()
				}
			}
			left = &ast.BinaryExpr{
				Left:     left,
				Operator: operator,
				Right:    right,
			}
		}
		return left
	}
	
	if addCtx := ctx.AdditiveExpression(0); addCtx != nil {
		return v.VisitAdditiveExpression(addCtx.(*minzparser.AdditiveExpressionContext))
	}
	
	return &ast.Identifier{Name: "rel"}
}

// VisitAdditiveExpression handles + and - operators
func (v *antlrVisitor) VisitAdditiveExpression(ctx *minzparser.AdditiveExpressionContext) interface{} {
	if mulCtxList := ctx.AllMultiplicativeExpression(); len(mulCtxList) > 1 {
		left := v.VisitMultiplicativeExpression(mulCtxList[0].(*minzparser.MultiplicativeExpressionContext)).(ast.Expression)
		for i := 1; i < len(mulCtxList); i++ {
			right := v.VisitMultiplicativeExpression(mulCtxList[i].(*minzparser.MultiplicativeExpressionContext)).(ast.Expression)
			operator := "+"
			if i*2-1 < ctx.GetChildCount() {
				if op := ctx.GetChild(i*2 - 1); op != nil {
					operator = op.(antlr.TerminalNode).GetText()
				}
			}
			left = &ast.BinaryExpr{
				Left:     left,
				Operator: operator,
				Right:    right,
			}
		}
		return left
	}
	
	if mulCtx := ctx.MultiplicativeExpression(0); mulCtx != nil {
		return v.VisitMultiplicativeExpression(mulCtx.(*minzparser.MultiplicativeExpressionContext))
	}
	
	return &ast.Identifier{Name: "add"}
}

// VisitMultiplicativeExpression handles *, / and % operators
func (v *antlrVisitor) VisitMultiplicativeExpression(ctx *minzparser.MultiplicativeExpressionContext) interface{} {
	if castCtxList := ctx.AllCastExpression(); len(castCtxList) > 1 {
		left := v.VisitCastExpression(castCtxList[0].(*minzparser.CastExpressionContext)).(ast.Expression)
		for i := 1; i < len(castCtxList); i++ {
			right := v.VisitCastExpression(castCtxList[i].(*minzparser.CastExpressionContext)).(ast.Expression)
			operator := "*"
			if i*2-1 < ctx.GetChildCount() {
				if op := ctx.GetChild(i*2 - 1); op != nil {
					operator = op.(antlr.TerminalNode).GetText()
				}
			}
			left = &ast.BinaryExpr{
				Left:     left,
				Operator: operator,
				Right:    right,
			}
		}
		return left
	}
	
	if castCtx := ctx.CastExpression(0); castCtx != nil {
		return v.VisitCastExpression(castCtx.(*minzparser.CastExpressionContext))
	}
	
	return &ast.Identifier{Name: "mul"}
}

// VisitCastExpression handles 'as' type casting
func (v *antlrVisitor) VisitCastExpression(ctx *minzparser.CastExpressionContext) interface{} {
	if unaryCtx := ctx.UnaryExpression(); unaryCtx != nil {
		expr := v.VisitUnaryExpression(unaryCtx.(*minzparser.UnaryExpressionContext)).(ast.Expression)
		
		if typeCtx := ctx.Type_(); typeCtx != nil {
			return &ast.CastExpr{
				Expr:       expr,
				TargetType: v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type),
			}
		}
		
		return expr
	}
	
	return &ast.Identifier{Name: "cast"}
}

// VisitUnaryExpression handles unary operators like !, -, ~, &, *
func (v *antlrVisitor) VisitUnaryExpression(ctx *minzparser.UnaryExpressionContext) interface{} {
	// Check for unary operators
	if ctx.GetChildCount() > 1 {
		operator := ctx.GetChild(0).(antlr.TerminalNode).GetText()
		if operator == "!" || operator == "-" || operator == "~" || operator == "&" || operator == "*" {
			expr := v.VisitUnaryExpression(ctx.UnaryExpression().(*minzparser.UnaryExpressionContext)).(ast.Expression)
			return &ast.UnaryExpr{
				Operator: operator,
				Operand:  expr,
			}
		}
	}
	
	if postfixCtx := ctx.PostfixExpression(); postfixCtx != nil {
		return v.VisitPostfixExpression(postfixCtx.(*minzparser.PostfixExpressionContext))
	}
	
	return &ast.Identifier{Name: "unary"}
}

// VisitPostfixExpression handles postfix operators like [], ., (), ?, ??
func (v *antlrVisitor) VisitPostfixExpression(ctx *minzparser.PostfixExpressionContext) interface{} {
	if primaryCtx := ctx.PrimaryExpression(); primaryCtx != nil {
		expr := v.VisitPrimaryExpression(primaryCtx.(*minzparser.PrimaryExpressionContext)).(ast.Expression)
		
		// Process postfix operators
		for _, opCtx := range ctx.AllPostfixOperator() {
			expr = v.applyPostfixOperator(expr, opCtx.(*minzparser.PostfixOperatorContext))
		}
		
		return expr
	}
	
	return &ast.Identifier{Name: "postfix"}
}

// applyPostfixOperator applies a postfix operator to an expression
func (v *antlrVisitor) applyPostfixOperator(expr ast.Expression, ctx *minzparser.PostfixOperatorContext) ast.Expression {
	// Array index: [expression]
	if indexCtx := ctx.Expression(); indexCtx != nil {
		index := v.VisitExpression(indexCtx.(*minzparser.ExpressionContext)).(ast.Expression)
		return &ast.IndexExpr{
			Array: expr,
			Index: index,
		}
	}
	
	// Field access: .identifier
	if idCtx := ctx.IDENTIFIER(); idCtx != nil {
		return &ast.FieldExpr{
			Object: expr,
			Field:  idCtx.GetText(),
		}
	}
	
	// Function call: (args)
	if argsCtx := ctx.ArgumentList(); argsCtx != nil {
		args := []ast.Expression{}
		for _, argCtx := range argsCtx.(*minzparser.ArgumentListContext).AllExpression() {
			args = append(args, v.VisitExpression(argCtx.(*minzparser.ExpressionContext)).(ast.Expression))
		}
		return &ast.CallExpr{
			Function:  expr,
			Arguments: args,
		}
	}
	
	// Try operator: ?
	if ctx.GetText() == "?" {
		return &ast.TryExpr{
			Expression: expr,
		}
	}
	
	// Nil coalescing: ??
	if ctx.GetText() == "??" {
		// This needs the right-hand side, which should be handled at a higher level
		return expr
	}
	
	// Iterator methods: .iter(), .map(), .filter(), .forEach()
	if ctx.GetText() == ".iter()" {
		return &ast.CallExpr{
			Function: &ast.FieldExpr{
				Object: expr,
				Field:  "iter",
			},
			Arguments: []ast.Expression{},
		}
	}
	
	return expr
}

// VisitPrimaryExpression handles primary expressions
func (v *antlrVisitor) VisitPrimaryExpression(ctx *minzparser.PrimaryExpressionContext) interface{} {
	// Literals
	if lit := ctx.Literal(); lit != nil {
		return v.VisitLiteral(lit.(*minzparser.LiteralContext))
	}
	
	// Qualified identifier (e.g., State::IDLE)
	if qid := ctx.QualifiedIdentifier(); qid != nil {
		return v.VisitQualifiedIdentifier(qid.(*minzparser.QualifiedIdentifierContext))
	}
	
	// Identifier
	if id := ctx.IDENTIFIER(); id != nil {
		return &ast.Identifier{
			Name: id.GetText(),
		}
	}
	
	// Parenthesized expression
	if expr := ctx.Expression(); expr != nil {
		return v.VisitExpression(expr.(*minzparser.ExpressionContext))
	}
	
	// Array literal
	if arr := ctx.ArrayLiteral(); arr != nil {
		return v.VisitArrayLiteral(arr.(*minzparser.ArrayLiteralContext))
	}
	
	// Struct literal
	if str := ctx.StructLiteral(); str != nil {
		return v.VisitStructLiteral(str.(*minzparser.StructLiteralContext))
	}
	
	// Metafunction
	if meta := ctx.Metafunction(); meta != nil {
		return v.VisitMetafunction(meta.(*minzparser.MetafunctionContext))
	}
	
	// Inline assembly
	if asm := ctx.InlineAssembly(); asm != nil {
		return v.VisitInlineAssembly(asm.(*minzparser.InlineAssemblyContext))
	}
	
	return &ast.Identifier{Name: "primary"}
}

// VisitLiteral handles literal values
func (v *antlrVisitor) VisitLiteral(ctx *minzparser.LiteralContext) interface{} {
	if num := ctx.NumberLiteral(); num != nil {
		return v.VisitNumberLiteral(num.(*minzparser.NumberLiteralContext))
	}
	
	if str := ctx.StringLiteral(); str != nil {
		return &ast.StringLiteral{
			Value: v.extractStringLiteral(str.GetText()),
		}
	}
	
	if char := ctx.CharLiteral(); char != nil {
		text := char.GetText()
		// Remove quotes and handle escape sequences
		if len(text) >= 3 {
			charValue := text[1 : len(text)-1]
			if charValue == "\\n" {
				charValue = "\n"
			} else if charValue == "\\t" {
				charValue = "\t"
			} else if charValue == "\\r" {
				charValue = "\r"
			} else if charValue == "\\\\" {
				charValue = "\\"
			}
			if len(charValue) > 0 {
				return &ast.NumberLiteral{
					Value: int64(charValue[0]),
				}
			}
		}
		return &ast.NumberLiteral{Value: int64(' ')}
	}
	
	if bool := ctx.BooleanLiteral(); bool != nil {
		return &ast.BooleanLiteral{
			Value: bool.GetText() == "true",
		}
	}
	
	return &ast.Identifier{Name: "literal"}
}

// VisitNumberLiteral handles numeric literals
func (v *antlrVisitor) VisitNumberLiteral(ctx *minzparser.NumberLiteralContext) interface{} {
	text := ctx.GetText()
	
	if ctx.HEX_NUMBER() != nil {
		// Hexadecimal
		value, _ := strconv.ParseInt(text[2:], 16, 64)
		return &ast.NumberLiteral{
			Value: value,
		}
	}
	
	if ctx.BINARY_NUMBER() != nil {
		// Binary
		value, _ := strconv.ParseInt(text[2:], 2, 64)
		return &ast.NumberLiteral{
			Value: value,
		}
	}
	
	// Decimal
	if strings.Contains(text, ".") {
		// Float - convert to fixed-point
		value, _ := strconv.ParseFloat(text, 64)
		return &ast.NumberLiteral{
			Value: int64(value * 256), // Fixed-point representation
		}
	}
	
	value, _ := strconv.ParseInt(text, 10, 64)
	return &ast.NumberLiteral{
		Value: value,
	}
}

// VisitArrayLiteral handles array literals
func (v *antlrVisitor) VisitArrayLiteral(ctx *minzparser.ArrayLiteralContext) interface{} {
	arr := &ast.ArrayInitializer{
		Elements: []ast.Expression{},
	}
	
	for _, exprCtx := range ctx.AllExpression() {
		arr.Elements = append(arr.Elements, v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression))
	}
	
	return arr
}

// VisitStructLiteral handles struct literals
func (v *antlrVisitor) VisitStructLiteral(ctx *minzparser.StructLiteralContext) interface{} {
	str := &ast.StructLiteral{
		Fields: []*ast.FieldInit{},
	}
	
	if id := ctx.IDENTIFIER(); id != nil {
		str.TypeName = id.GetText()
	}
	
	for _, fieldCtx := range ctx.AllFieldInit() {
		fieldInit := fieldCtx.(*minzparser.FieldInitContext)
		if fieldId := fieldInit.IDENTIFIER(); fieldId != nil {
			if fieldExpr := fieldInit.Expression(); fieldExpr != nil {
				field := &ast.FieldInit{
					Name:  fieldId.GetText(),
					Value: v.VisitExpression(fieldExpr.(*minzparser.ExpressionContext)).(ast.Expression),
				}
				str.Fields = append(str.Fields, field)
			}
		}
	}
	
	return str
}

// VisitMetafunction handles metafunction calls
func (v *antlrVisitor) VisitMetafunction(ctx *minzparser.MetafunctionContext) interface{} {
	// Get the first token to determine metafunction type
	firstChild := ctx.GetChild(0)
	if firstChild == nil {
		return &ast.MetafunctionCall{
			Name:      "@unknown",
			Arguments: []ast.Expression{},
		}
	}
	
	// Extract arguments from all expressions
	args := []ast.Expression{}
	for _, exprCtx := range ctx.AllExpression() {
		args = append(args, v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression))
	}
	
	// Check for different metafunction types
	firstToken := ""
	if terminal, ok := firstChild.(antlr.TerminalNode); ok {
		firstToken = terminal.GetText()
	}
	
	switch firstToken {
	case "@print":
		return &ast.MetafunctionCall{
			Name:      "@print",
			Arguments: args,
		}
	case "@assert":
		return &ast.MetafunctionCall{
			Name:      "@assert",
			Arguments: args,
		}
	case "@error":
		// @error only takes a string literal
		strArgs := []ast.Expression{}
		if strCtx := ctx.StringLiteral(); strCtx != nil {
			strArgs = append(strArgs, v.VisitStringLiteral(strCtx.(*minzparser.StringLiteralContext)).(ast.Expression))
		}
		return &ast.MetafunctionCall{
			Name:      "@error",
			Arguments: strArgs,
		}
	case "@abi":
		// @abi only takes a string literal
		strArgs := []ast.Expression{}
		if strCtx := ctx.StringLiteral(); strCtx != nil {
			strArgs = append(strArgs, v.VisitStringLiteral(strCtx.(*minzparser.StringLiteralContext)).(ast.Expression))
		}
		return &ast.MetafunctionCall{
			Name:      "@abi",
			Arguments: strArgs,
		}
	case "@log":
		// Handle @log and @log.level
		name := "@log"
		
		// Check if there's a dot and level after @log
		// Parse the full text to extract level  
		text := ctx.GetText()
		if strings.HasPrefix(text, "@log.") {
			// Extract level from @log.level(...)
			parts := strings.Split(text, "(")
			if len(parts) > 0 {
				name = strings.TrimSpace(parts[0])
			}
		}
		
		return &ast.MetafunctionCall{
			Name:      name,
			Arguments: args,
		}
	case "@define":
		// @define takes an identifier and expression
		defineArgs := []ast.Expression{}
		if idCtx := ctx.IDENTIFIER(); idCtx != nil {
			defineArgs = append(defineArgs, &ast.Identifier{Name: idCtx.GetText()})
		}
		if len(args) > 0 {
			defineArgs = append(defineArgs, args[0])
		}
		return &ast.MetafunctionCall{
			Name:      "@define",
			Arguments: defineArgs,
		}
	case "@include":
		// @include only takes a string literal
		strArgs := []ast.Expression{}
		if strCtx := ctx.StringLiteral(); strCtx != nil {
			strArgs = append(strArgs, v.VisitStringLiteral(strCtx.(*minzparser.StringLiteralContext)).(ast.Expression))
		}
		return &ast.MetafunctionCall{
			Name:      "@include",
			Arguments: strArgs,
		}
	default:
		// Unknown metafunction
		return &ast.MetafunctionCall{
			Name:      "@unknown",
			Arguments: args,
		}
	}
}

// Note: LogMetafunction handling is done in VisitMetafunction since it's an alternative in the grammar

// VisitInlineAssembly handles inline assembly expressions
func (v *antlrVisitor) VisitInlineAssembly(ctx *minzparser.InlineAssemblyContext) interface{} {
	if strCtx := ctx.StringLiteral(); strCtx != nil {
		return &ast.InlineAssembly{
			Code: v.extractStringLiteral(strCtx.GetText()),
		}
	}
	return &ast.InlineAssembly{Code: ""}
}

func (v *antlrVisitor) VisitType(ctx *minzparser.TypeContext) interface{} {
	// Primitive types
	if primCtx := ctx.PrimitiveType(); primCtx != nil {
		return &ast.PrimitiveType{
			Name: primCtx.GetText(),
		}
	}
	
	// Array types
	if arrCtx := ctx.ArrayType(); arrCtx != nil {
		return v.VisitArrayType(arrCtx.(*minzparser.ArrayTypeContext))
	}
	
	// Pointer types
	if ptrCtx := ctx.PointerType(); ptrCtx != nil {
		return v.VisitPointerType(ptrCtx.(*minzparser.PointerTypeContext))
	}
	
	// Function types
	if funCtx := ctx.FunctionType(); funCtx != nil {
		return v.VisitFunctionType(funCtx.(*minzparser.FunctionTypeContext))
	}
	
	// Struct types
	if structCtx := ctx.StructType(); structCtx != nil {
		return v.VisitStructType(structCtx.(*minzparser.StructTypeContext))
	}
	
	// Enum types
	if enumCtx := ctx.EnumType(); enumCtx != nil {
		return v.VisitEnumType(enumCtx.(*minzparser.EnumTypeContext))
	}
	
	// Bit struct types
	if bitCtx := ctx.BitStructType(); bitCtx != nil {
		return v.VisitBitStructType(bitCtx.(*minzparser.BitStructTypeContext))
	}
	
	// Type identifier
	if idCtx := ctx.TypeIdentifier(); idCtx != nil {
		return &ast.TypeIdentifier{
			Name: idCtx.GetText(),
		}
	}
	
	// Error type
	if errCtx := ctx.ErrorType(); errCtx != nil {
		return v.VisitErrorType(errCtx.(*minzparser.ErrorTypeContext))
	}
	
	return &ast.PrimitiveType{Name: "u8"}
}

// VisitArrayType handles array type definitions
func (v *antlrVisitor) VisitArrayType(ctx *minzparser.ArrayTypeContext) interface{} {
	arr := &ast.ArrayType{}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		arr.ElementType = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	if exprCtx := ctx.Expression(); exprCtx != nil {
		arr.Size = v.VisitExpression(exprCtx.(*minzparser.ExpressionContext)).(ast.Expression)
	}
	
	return arr
}

// VisitPointerType handles pointer type definitions
func (v *antlrVisitor) VisitPointerType(ctx *minzparser.PointerTypeContext) interface{} {
	ptr := &ast.PointerType{}
	
	if typeCtx := ctx.Type_(); typeCtx != nil {
		ptr.BaseType = v.VisitType(typeCtx.(*minzparser.TypeContext)).(ast.Type)
	}
	
	// Check for mut modifier
	if ctx.GetText() != "" {
		if strings.Contains(ctx.GetText(), "mut") {
			ptr.IsMutable = true
		}
	}
	
	return ptr
}

// VisitFunctionType handles function type definitions
func (v *antlrVisitor) VisitFunctionType(ctx *minzparser.FunctionTypeContext) interface{} {
	// Function types are not fully supported in AST yet
	// Return a type identifier for now
	return &ast.TypeIdentifier{
		Name: "function",
	}
}

// VisitStructType handles struct type definitions
func (v *antlrVisitor) VisitStructType(ctx *minzparser.StructTypeContext) interface{} {
	st := &ast.StructType{
		Fields: []*ast.Field{},
	}
	
	if fieldsCtx := ctx.FieldList(); fieldsCtx != nil {
		st.Fields = v.VisitFieldList(fieldsCtx.(*minzparser.FieldListContext)).([]*ast.Field)
	}
	
	return st
}

// VisitEnumType handles enum type definitions
func (v *antlrVisitor) VisitEnumType(ctx *minzparser.EnumTypeContext) interface{} {
	enumType := &ast.EnumType{
		Variants: []string{},
	}
	
	if membersCtx := ctx.EnumMemberList(); membersCtx != nil {
		for _, memberCtx := range membersCtx.(*minzparser.EnumMemberListContext).AllEnumMember() {
			if idCtx := memberCtx.(*minzparser.EnumMemberContext).IDENTIFIER(); idCtx != nil {
				enumType.Variants = append(enumType.Variants, idCtx.GetText())
			}
		}
	}
	
	return enumType
}

// VisitBitStructType handles bit struct type definitions
func (v *antlrVisitor) VisitBitStructType(ctx *minzparser.BitStructTypeContext) interface{} {
	bitType := &ast.BitStructType{
		Fields: []*ast.BitField{},
	}
	
	if listCtx := ctx.BitFieldList(); listCtx != nil {
		for _, fieldCtx := range listCtx.(*minzparser.BitFieldListContext).AllBitField() {
			field := fieldCtx.(*minzparser.BitFieldContext)
			bitField := &ast.BitField{}
			
			if idCtx := field.IDENTIFIER(); idCtx != nil {
				bitField.Name = idCtx.GetText()
			}
			
			if numCtx := field.NUMBER(); numCtx != nil {
				width, _ := strconv.Atoi(numCtx.GetText())
				bitField.BitWidth = width
			}
			
			bitType.Fields = append(bitType.Fields, bitField)
		}
	}
	
	return bitType
}

// VisitErrorType handles error type definitions
func (v *antlrVisitor) VisitErrorType(ctx *minzparser.ErrorTypeContext) interface{} {
	// Get the base type
	var baseType ast.Type
	
	if primCtx := ctx.PrimitiveType(); primCtx != nil {
		baseType = &ast.PrimitiveType{Name: primCtx.GetText()}
	} else if arrCtx := ctx.ArrayType(); arrCtx != nil {
		baseType = v.VisitArrayType(arrCtx.(*minzparser.ArrayTypeContext)).(ast.Type)
	} else if ptrCtx := ctx.PointerType(); ptrCtx != nil {
		baseType = v.VisitPointerType(ptrCtx.(*minzparser.PointerTypeContext)).(ast.Type)
	} else if idCtx := ctx.TypeIdentifier(); idCtx != nil {
		baseType = &ast.TypeIdentifier{Name: idCtx.GetText()}
	}
	
	// Wrap in error type
	return &ast.ErrorType{
		ValueType: baseType,
	}
}
// VisitQualifiedIdentifier handles qualified identifiers like State::IDLE
func (v *antlrVisitor) VisitQualifiedIdentifier(ctx *minzparser.QualifiedIdentifierContext) interface{} {
	ids := ctx.AllIDENTIFIER()
	if len(ids) == 2 {
		// Enum member access: EnumType::Member
		return &ast.FieldExpr{
			Object: &ast.Identifier{
				Name: ids[0].GetText(),
			},
			Field: ids[1].GetText(),
		}
	}
	return nil
}