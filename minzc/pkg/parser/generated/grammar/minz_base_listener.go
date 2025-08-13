// Code generated from grammar/MinZ.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // MinZ

import "github.com/antlr4-go/antlr/v4"

// BaseMinZListener is a complete listener for a parse tree produced by MinZParser.
type BaseMinZListener struct{}

var _ MinZListener = &BaseMinZListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseMinZListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseMinZListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseMinZListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseMinZListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterProgram is called when production program is entered.
func (s *BaseMinZListener) EnterProgram(ctx *ProgramContext) {}

// ExitProgram is called when production program is exited.
func (s *BaseMinZListener) ExitProgram(ctx *ProgramContext) {}

// EnterImportDecl is called when production importDecl is entered.
func (s *BaseMinZListener) EnterImportDecl(ctx *ImportDeclContext) {}

// ExitImportDecl is called when production importDecl is exited.
func (s *BaseMinZListener) ExitImportDecl(ctx *ImportDeclContext) {}

// EnterImportPath is called when production importPath is entered.
func (s *BaseMinZListener) EnterImportPath(ctx *ImportPathContext) {}

// ExitImportPath is called when production importPath is exited.
func (s *BaseMinZListener) ExitImportPath(ctx *ImportPathContext) {}

// EnterDeclaration is called when production declaration is entered.
func (s *BaseMinZListener) EnterDeclaration(ctx *DeclarationContext) {}

// ExitDeclaration is called when production declaration is exited.
func (s *BaseMinZListener) ExitDeclaration(ctx *DeclarationContext) {}

// EnterFunctionDecl is called when production functionDecl is entered.
func (s *BaseMinZListener) EnterFunctionDecl(ctx *FunctionDeclContext) {}

// ExitFunctionDecl is called when production functionDecl is exited.
func (s *BaseMinZListener) ExitFunctionDecl(ctx *FunctionDeclContext) {}

// EnterAsmFunction is called when production asmFunction is entered.
func (s *BaseMinZListener) EnterAsmFunction(ctx *AsmFunctionContext) {}

// ExitAsmFunction is called when production asmFunction is exited.
func (s *BaseMinZListener) ExitAsmFunction(ctx *AsmFunctionContext) {}

// EnterMirFunction is called when production mirFunction is entered.
func (s *BaseMinZListener) EnterMirFunction(ctx *MirFunctionContext) {}

// ExitMirFunction is called when production mirFunction is exited.
func (s *BaseMinZListener) ExitMirFunction(ctx *MirFunctionContext) {}

// EnterGenericParams is called when production genericParams is entered.
func (s *BaseMinZListener) EnterGenericParams(ctx *GenericParamsContext) {}

// ExitGenericParams is called when production genericParams is exited.
func (s *BaseMinZListener) ExitGenericParams(ctx *GenericParamsContext) {}

// EnterReturnType is called when production returnType is entered.
func (s *BaseMinZListener) EnterReturnType(ctx *ReturnTypeContext) {}

// ExitReturnType is called when production returnType is exited.
func (s *BaseMinZListener) ExitReturnType(ctx *ReturnTypeContext) {}

// EnterErrorType is called when production errorType is entered.
func (s *BaseMinZListener) EnterErrorType(ctx *ErrorTypeContext) {}

// ExitErrorType is called when production errorType is exited.
func (s *BaseMinZListener) ExitErrorType(ctx *ErrorTypeContext) {}

// EnterFunctionBody is called when production functionBody is entered.
func (s *BaseMinZListener) EnterFunctionBody(ctx *FunctionBodyContext) {}

// ExitFunctionBody is called when production functionBody is exited.
func (s *BaseMinZListener) ExitFunctionBody(ctx *FunctionBodyContext) {}

// EnterAsmBody is called when production asmBody is entered.
func (s *BaseMinZListener) EnterAsmBody(ctx *AsmBodyContext) {}

// ExitAsmBody is called when production asmBody is exited.
func (s *BaseMinZListener) ExitAsmBody(ctx *AsmBodyContext) {}

// EnterAsmContent is called when production asmContent is entered.
func (s *BaseMinZListener) EnterAsmContent(ctx *AsmContentContext) {}

// ExitAsmContent is called when production asmContent is exited.
func (s *BaseMinZListener) ExitAsmContent(ctx *AsmContentContext) {}

// EnterMirBody is called when production mirBody is entered.
func (s *BaseMinZListener) EnterMirBody(ctx *MirBodyContext) {}

// ExitMirBody is called when production mirBody is exited.
func (s *BaseMinZListener) ExitMirBody(ctx *MirBodyContext) {}

// EnterMirContent is called when production mirContent is entered.
func (s *BaseMinZListener) EnterMirContent(ctx *MirContentContext) {}

// ExitMirContent is called when production mirContent is exited.
func (s *BaseMinZListener) ExitMirContent(ctx *MirContentContext) {}

// EnterParameterList is called when production parameterList is entered.
func (s *BaseMinZListener) EnterParameterList(ctx *ParameterListContext) {}

// ExitParameterList is called when production parameterList is exited.
func (s *BaseMinZListener) ExitParameterList(ctx *ParameterListContext) {}

// EnterParameter is called when production parameter is entered.
func (s *BaseMinZListener) EnterParameter(ctx *ParameterContext) {}

// ExitParameter is called when production parameter is exited.
func (s *BaseMinZListener) ExitParameter(ctx *ParameterContext) {}

// EnterStructDecl is called when production structDecl is entered.
func (s *BaseMinZListener) EnterStructDecl(ctx *StructDeclContext) {}

// ExitStructDecl is called when production structDecl is exited.
func (s *BaseMinZListener) ExitStructDecl(ctx *StructDeclContext) {}

// EnterStructField is called when production structField is entered.
func (s *BaseMinZListener) EnterStructField(ctx *StructFieldContext) {}

// ExitStructField is called when production structField is exited.
func (s *BaseMinZListener) ExitStructField(ctx *StructFieldContext) {}

// EnterInterfaceDecl is called when production interfaceDecl is entered.
func (s *BaseMinZListener) EnterInterfaceDecl(ctx *InterfaceDeclContext) {}

// ExitInterfaceDecl is called when production interfaceDecl is exited.
func (s *BaseMinZListener) ExitInterfaceDecl(ctx *InterfaceDeclContext) {}

// EnterMethodSignature is called when production methodSignature is entered.
func (s *BaseMinZListener) EnterMethodSignature(ctx *MethodSignatureContext) {}

// ExitMethodSignature is called when production methodSignature is exited.
func (s *BaseMinZListener) ExitMethodSignature(ctx *MethodSignatureContext) {}

// EnterCastInterfaceBlock is called when production castInterfaceBlock is entered.
func (s *BaseMinZListener) EnterCastInterfaceBlock(ctx *CastInterfaceBlockContext) {}

// ExitCastInterfaceBlock is called when production castInterfaceBlock is exited.
func (s *BaseMinZListener) ExitCastInterfaceBlock(ctx *CastInterfaceBlockContext) {}

// EnterCastRule is called when production castRule is entered.
func (s *BaseMinZListener) EnterCastRule(ctx *CastRuleContext) {}

// ExitCastRule is called when production castRule is exited.
func (s *BaseMinZListener) ExitCastRule(ctx *CastRuleContext) {}

// EnterEnumDecl is called when production enumDecl is entered.
func (s *BaseMinZListener) EnterEnumDecl(ctx *EnumDeclContext) {}

// ExitEnumDecl is called when production enumDecl is exited.
func (s *BaseMinZListener) ExitEnumDecl(ctx *EnumDeclContext) {}

// EnterEnumVariant is called when production enumVariant is entered.
func (s *BaseMinZListener) EnterEnumVariant(ctx *EnumVariantContext) {}

// ExitEnumVariant is called when production enumVariant is exited.
func (s *BaseMinZListener) ExitEnumVariant(ctx *EnumVariantContext) {}

// EnterBitStructDecl is called when production bitStructDecl is entered.
func (s *BaseMinZListener) EnterBitStructDecl(ctx *BitStructDeclContext) {}

// ExitBitStructDecl is called when production bitStructDecl is exited.
func (s *BaseMinZListener) ExitBitStructDecl(ctx *BitStructDeclContext) {}

// EnterBitField is called when production bitField is entered.
func (s *BaseMinZListener) EnterBitField(ctx *BitFieldContext) {}

// ExitBitField is called when production bitField is exited.
func (s *BaseMinZListener) ExitBitField(ctx *BitFieldContext) {}

// EnterConstDecl is called when production constDecl is entered.
func (s *BaseMinZListener) EnterConstDecl(ctx *ConstDeclContext) {}

// ExitConstDecl is called when production constDecl is exited.
func (s *BaseMinZListener) ExitConstDecl(ctx *ConstDeclContext) {}

// EnterGlobalDecl is called when production globalDecl is entered.
func (s *BaseMinZListener) EnterGlobalDecl(ctx *GlobalDeclContext) {}

// ExitGlobalDecl is called when production globalDecl is exited.
func (s *BaseMinZListener) ExitGlobalDecl(ctx *GlobalDeclContext) {}

// EnterTypeAlias is called when production typeAlias is entered.
func (s *BaseMinZListener) EnterTypeAlias(ctx *TypeAliasContext) {}

// ExitTypeAlias is called when production typeAlias is exited.
func (s *BaseMinZListener) ExitTypeAlias(ctx *TypeAliasContext) {}

// EnterImplBlock is called when production implBlock is entered.
func (s *BaseMinZListener) EnterImplBlock(ctx *ImplBlockContext) {}

// ExitImplBlock is called when production implBlock is exited.
func (s *BaseMinZListener) ExitImplBlock(ctx *ImplBlockContext) {}

// EnterMetafunction is called when production metafunction is entered.
func (s *BaseMinZListener) EnterMetafunction(ctx *MetafunctionContext) {}

// ExitMetafunction is called when production metafunction is exited.
func (s *BaseMinZListener) ExitMetafunction(ctx *MetafunctionContext) {}

// EnterAttributedDeclaration is called when production attributedDeclaration is entered.
func (s *BaseMinZListener) EnterAttributedDeclaration(ctx *AttributedDeclarationContext) {}

// ExitAttributedDeclaration is called when production attributedDeclaration is exited.
func (s *BaseMinZListener) ExitAttributedDeclaration(ctx *AttributedDeclarationContext) {}

// EnterAttribute is called when production attribute is entered.
func (s *BaseMinZListener) EnterAttribute(ctx *AttributeContext) {}

// ExitAttribute is called when production attribute is exited.
func (s *BaseMinZListener) ExitAttribute(ctx *AttributeContext) {}

// EnterLuaBlock is called when production luaBlock is entered.
func (s *BaseMinZListener) EnterLuaBlock(ctx *LuaBlockContext) {}

// ExitLuaBlock is called when production luaBlock is exited.
func (s *BaseMinZListener) ExitLuaBlock(ctx *LuaBlockContext) {}

// EnterLuaCodeBlock is called when production luaCodeBlock is entered.
func (s *BaseMinZListener) EnterLuaCodeBlock(ctx *LuaCodeBlockContext) {}

// ExitLuaCodeBlock is called when production luaCodeBlock is exited.
func (s *BaseMinZListener) ExitLuaCodeBlock(ctx *LuaCodeBlockContext) {}

// EnterMirBlockDeclaration is called when production mirBlockDeclaration is entered.
func (s *BaseMinZListener) EnterMirBlockDeclaration(ctx *MirBlockDeclarationContext) {}

// ExitMirBlockDeclaration is called when production mirBlockDeclaration is exited.
func (s *BaseMinZListener) ExitMirBlockDeclaration(ctx *MirBlockDeclarationContext) {}

// EnterMirBlockContent is called when production mirBlockContent is entered.
func (s *BaseMinZListener) EnterMirBlockContent(ctx *MirBlockContentContext) {}

// ExitMirBlockContent is called when production mirBlockContent is exited.
func (s *BaseMinZListener) ExitMirBlockContent(ctx *MirBlockContentContext) {}

// EnterMinzMetafunctionDeclaration is called when production minzMetafunctionDeclaration is entered.
func (s *BaseMinZListener) EnterMinzMetafunctionDeclaration(ctx *MinzMetafunctionDeclarationContext) {
}

// ExitMinzMetafunctionDeclaration is called when production minzMetafunctionDeclaration is exited.
func (s *BaseMinZListener) ExitMinzMetafunctionDeclaration(ctx *MinzMetafunctionDeclarationContext) {}

// EnterCompileTimeIfDeclaration is called when production compileTimeIfDeclaration is entered.
func (s *BaseMinZListener) EnterCompileTimeIfDeclaration(ctx *CompileTimeIfDeclarationContext) {}

// ExitCompileTimeIfDeclaration is called when production compileTimeIfDeclaration is exited.
func (s *BaseMinZListener) ExitCompileTimeIfDeclaration(ctx *CompileTimeIfDeclarationContext) {}

// EnterDefineTemplate is called when production defineTemplate is entered.
func (s *BaseMinZListener) EnterDefineTemplate(ctx *DefineTemplateContext) {}

// ExitDefineTemplate is called when production defineTemplate is exited.
func (s *BaseMinZListener) ExitDefineTemplate(ctx *DefineTemplateContext) {}

// EnterTemplateBody is called when production templateBody is entered.
func (s *BaseMinZListener) EnterTemplateBody(ctx *TemplateBodyContext) {}

// ExitTemplateBody is called when production templateBody is exited.
func (s *BaseMinZListener) ExitTemplateBody(ctx *TemplateBodyContext) {}

// EnterIdentifierList is called when production identifierList is entered.
func (s *BaseMinZListener) EnterIdentifierList(ctx *IdentifierListContext) {}

// ExitIdentifierList is called when production identifierList is exited.
func (s *BaseMinZListener) ExitIdentifierList(ctx *IdentifierListContext) {}

// EnterMetaExecutionBlock is called when production metaExecutionBlock is entered.
func (s *BaseMinZListener) EnterMetaExecutionBlock(ctx *MetaExecutionBlockContext) {}

// ExitMetaExecutionBlock is called when production metaExecutionBlock is exited.
func (s *BaseMinZListener) ExitMetaExecutionBlock(ctx *MetaExecutionBlockContext) {}

// EnterLuaExecutionBlock is called when production luaExecutionBlock is entered.
func (s *BaseMinZListener) EnterLuaExecutionBlock(ctx *LuaExecutionBlockContext) {}

// ExitLuaExecutionBlock is called when production luaExecutionBlock is exited.
func (s *BaseMinZListener) ExitLuaExecutionBlock(ctx *LuaExecutionBlockContext) {}

// EnterMinzExecutionBlock is called when production minzExecutionBlock is entered.
func (s *BaseMinZListener) EnterMinzExecutionBlock(ctx *MinzExecutionBlockContext) {}

// ExitMinzExecutionBlock is called when production minzExecutionBlock is exited.
func (s *BaseMinZListener) ExitMinzExecutionBlock(ctx *MinzExecutionBlockContext) {}

// EnterMirExecutionBlock is called when production mirExecutionBlock is entered.
func (s *BaseMinZListener) EnterMirExecutionBlock(ctx *MirExecutionBlockContext) {}

// ExitMirExecutionBlock is called when production mirExecutionBlock is exited.
func (s *BaseMinZListener) ExitMirExecutionBlock(ctx *MirExecutionBlockContext) {}

// EnterRawBlockContent is called when production rawBlockContent is entered.
func (s *BaseMinZListener) EnterRawBlockContent(ctx *RawBlockContentContext) {}

// ExitRawBlockContent is called when production rawBlockContent is exited.
func (s *BaseMinZListener) ExitRawBlockContent(ctx *RawBlockContentContext) {}

// EnterType is called when production type is entered.
func (s *BaseMinZListener) EnterType(ctx *TypeContext) {}

// ExitType is called when production type is exited.
func (s *BaseMinZListener) ExitType(ctx *TypeContext) {}

// EnterPrimitiveType is called when production primitiveType is entered.
func (s *BaseMinZListener) EnterPrimitiveType(ctx *PrimitiveTypeContext) {}

// ExitPrimitiveType is called when production primitiveType is exited.
func (s *BaseMinZListener) ExitPrimitiveType(ctx *PrimitiveTypeContext) {}

// EnterNamedType is called when production namedType is entered.
func (s *BaseMinZListener) EnterNamedType(ctx *NamedTypeContext) {}

// ExitNamedType is called when production namedType is exited.
func (s *BaseMinZListener) ExitNamedType(ctx *NamedTypeContext) {}

// EnterArrayType is called when production arrayType is entered.
func (s *BaseMinZListener) EnterArrayType(ctx *ArrayTypeContext) {}

// ExitArrayType is called when production arrayType is exited.
func (s *BaseMinZListener) ExitArrayType(ctx *ArrayTypeContext) {}

// EnterArraySize is called when production arraySize is entered.
func (s *BaseMinZListener) EnterArraySize(ctx *ArraySizeContext) {}

// ExitArraySize is called when production arraySize is exited.
func (s *BaseMinZListener) ExitArraySize(ctx *ArraySizeContext) {}

// EnterPointerType is called when production pointerType is entered.
func (s *BaseMinZListener) EnterPointerType(ctx *PointerTypeContext) {}

// ExitPointerType is called when production pointerType is exited.
func (s *BaseMinZListener) ExitPointerType(ctx *PointerTypeContext) {}

// EnterFunctionType is called when production functionType is entered.
func (s *BaseMinZListener) EnterFunctionType(ctx *FunctionTypeContext) {}

// ExitFunctionType is called when production functionType is exited.
func (s *BaseMinZListener) ExitFunctionType(ctx *FunctionTypeContext) {}

// EnterBitStructType is called when production bitStructType is entered.
func (s *BaseMinZListener) EnterBitStructType(ctx *BitStructTypeContext) {}

// ExitBitStructType is called when production bitStructType is exited.
func (s *BaseMinZListener) ExitBitStructType(ctx *BitStructTypeContext) {}

// EnterErrorableType is called when production errorableType is entered.
func (s *BaseMinZListener) EnterErrorableType(ctx *ErrorableTypeContext) {}

// ExitErrorableType is called when production errorableType is exited.
func (s *BaseMinZListener) ExitErrorableType(ctx *ErrorableTypeContext) {}

// EnterMutableType is called when production mutableType is entered.
func (s *BaseMinZListener) EnterMutableType(ctx *MutableTypeContext) {}

// ExitMutableType is called when production mutableType is exited.
func (s *BaseMinZListener) ExitMutableType(ctx *MutableTypeContext) {}

// EnterIteratorType is called when production iteratorType is entered.
func (s *BaseMinZListener) EnterIteratorType(ctx *IteratorTypeContext) {}

// ExitIteratorType is called when production iteratorType is exited.
func (s *BaseMinZListener) ExitIteratorType(ctx *IteratorTypeContext) {}

// EnterPrimaryType is called when production primaryType is entered.
func (s *BaseMinZListener) EnterPrimaryType(ctx *PrimaryTypeContext) {}

// ExitPrimaryType is called when production primaryType is exited.
func (s *BaseMinZListener) ExitPrimaryType(ctx *PrimaryTypeContext) {}

// EnterStatement is called when production statement is entered.
func (s *BaseMinZListener) EnterStatement(ctx *StatementContext) {}

// ExitStatement is called when production statement is exited.
func (s *BaseMinZListener) ExitStatement(ctx *StatementContext) {}

// EnterLetStatement is called when production letStatement is entered.
func (s *BaseMinZListener) EnterLetStatement(ctx *LetStatementContext) {}

// ExitLetStatement is called when production letStatement is exited.
func (s *BaseMinZListener) ExitLetStatement(ctx *LetStatementContext) {}

// EnterIfStatement is called when production ifStatement is entered.
func (s *BaseMinZListener) EnterIfStatement(ctx *IfStatementContext) {}

// ExitIfStatement is called when production ifStatement is exited.
func (s *BaseMinZListener) ExitIfStatement(ctx *IfStatementContext) {}

// EnterWhileStatement is called when production whileStatement is entered.
func (s *BaseMinZListener) EnterWhileStatement(ctx *WhileStatementContext) {}

// ExitWhileStatement is called when production whileStatement is exited.
func (s *BaseMinZListener) ExitWhileStatement(ctx *WhileStatementContext) {}

// EnterForStatement is called when production forStatement is entered.
func (s *BaseMinZListener) EnterForStatement(ctx *ForStatementContext) {}

// ExitForStatement is called when production forStatement is exited.
func (s *BaseMinZListener) ExitForStatement(ctx *ForStatementContext) {}

// EnterLoopStatement is called when production loopStatement is entered.
func (s *BaseMinZListener) EnterLoopStatement(ctx *LoopStatementContext) {}

// ExitLoopStatement is called when production loopStatement is exited.
func (s *BaseMinZListener) ExitLoopStatement(ctx *LoopStatementContext) {}

// EnterMatchStatement is called when production matchStatement is entered.
func (s *BaseMinZListener) EnterMatchStatement(ctx *MatchStatementContext) {}

// ExitMatchStatement is called when production matchStatement is exited.
func (s *BaseMinZListener) ExitMatchStatement(ctx *MatchStatementContext) {}

// EnterMatchArm is called when production matchArm is entered.
func (s *BaseMinZListener) EnterMatchArm(ctx *MatchArmContext) {}

// ExitMatchArm is called when production matchArm is exited.
func (s *BaseMinZListener) ExitMatchArm(ctx *MatchArmContext) {}

// EnterCaseStatement is called when production caseStatement is entered.
func (s *BaseMinZListener) EnterCaseStatement(ctx *CaseStatementContext) {}

// ExitCaseStatement is called when production caseStatement is exited.
func (s *BaseMinZListener) ExitCaseStatement(ctx *CaseStatementContext) {}

// EnterCaseArm is called when production caseArm is entered.
func (s *BaseMinZListener) EnterCaseArm(ctx *CaseArmContext) {}

// ExitCaseArm is called when production caseArm is exited.
func (s *BaseMinZListener) ExitCaseArm(ctx *CaseArmContext) {}

// EnterPattern is called when production pattern is entered.
func (s *BaseMinZListener) EnterPattern(ctx *PatternContext) {}

// ExitPattern is called when production pattern is exited.
func (s *BaseMinZListener) ExitPattern(ctx *PatternContext) {}

// EnterEnumPattern is called when production enumPattern is entered.
func (s *BaseMinZListener) EnterEnumPattern(ctx *EnumPatternContext) {}

// ExitEnumPattern is called when production enumPattern is exited.
func (s *BaseMinZListener) ExitEnumPattern(ctx *EnumPatternContext) {}

// EnterReturnStatement is called when production returnStatement is entered.
func (s *BaseMinZListener) EnterReturnStatement(ctx *ReturnStatementContext) {}

// ExitReturnStatement is called when production returnStatement is exited.
func (s *BaseMinZListener) ExitReturnStatement(ctx *ReturnStatementContext) {}

// EnterBreakStatement is called when production breakStatement is entered.
func (s *BaseMinZListener) EnterBreakStatement(ctx *BreakStatementContext) {}

// ExitBreakStatement is called when production breakStatement is exited.
func (s *BaseMinZListener) ExitBreakStatement(ctx *BreakStatementContext) {}

// EnterContinueStatement is called when production continueStatement is entered.
func (s *BaseMinZListener) EnterContinueStatement(ctx *ContinueStatementContext) {}

// ExitContinueStatement is called when production continueStatement is exited.
func (s *BaseMinZListener) ExitContinueStatement(ctx *ContinueStatementContext) {}

// EnterDeferStatement is called when production deferStatement is entered.
func (s *BaseMinZListener) EnterDeferStatement(ctx *DeferStatementContext) {}

// ExitDeferStatement is called when production deferStatement is exited.
func (s *BaseMinZListener) ExitDeferStatement(ctx *DeferStatementContext) {}

// EnterAssignmentStatement is called when production assignmentStatement is entered.
func (s *BaseMinZListener) EnterAssignmentStatement(ctx *AssignmentStatementContext) {}

// ExitAssignmentStatement is called when production assignmentStatement is exited.
func (s *BaseMinZListener) ExitAssignmentStatement(ctx *AssignmentStatementContext) {}

// EnterAssignmentTarget is called when production assignmentTarget is entered.
func (s *BaseMinZListener) EnterAssignmentTarget(ctx *AssignmentTargetContext) {}

// ExitAssignmentTarget is called when production assignmentTarget is exited.
func (s *BaseMinZListener) ExitAssignmentTarget(ctx *AssignmentTargetContext) {}

// EnterAssignmentOp is called when production assignmentOp is entered.
func (s *BaseMinZListener) EnterAssignmentOp(ctx *AssignmentOpContext) {}

// ExitAssignmentOp is called when production assignmentOp is exited.
func (s *BaseMinZListener) ExitAssignmentOp(ctx *AssignmentOpContext) {}

// EnterExpressionStatement is called when production expressionStatement is entered.
func (s *BaseMinZListener) EnterExpressionStatement(ctx *ExpressionStatementContext) {}

// ExitExpressionStatement is called when production expressionStatement is exited.
func (s *BaseMinZListener) ExitExpressionStatement(ctx *ExpressionStatementContext) {}

// EnterBlock is called when production block is entered.
func (s *BaseMinZListener) EnterBlock(ctx *BlockContext) {}

// ExitBlock is called when production block is exited.
func (s *BaseMinZListener) ExitBlock(ctx *BlockContext) {}

// EnterAsmBlock is called when production asmBlock is entered.
func (s *BaseMinZListener) EnterAsmBlock(ctx *AsmBlockContext) {}

// ExitAsmBlock is called when production asmBlock is exited.
func (s *BaseMinZListener) ExitAsmBlock(ctx *AsmBlockContext) {}

// EnterCompileTimeAsm is called when production compileTimeAsm is entered.
func (s *BaseMinZListener) EnterCompileTimeAsm(ctx *CompileTimeAsmContext) {}

// ExitCompileTimeAsm is called when production compileTimeAsm is exited.
func (s *BaseMinZListener) ExitCompileTimeAsm(ctx *CompileTimeAsmContext) {}

// EnterMirBlock is called when production mirBlock is entered.
func (s *BaseMinZListener) EnterMirBlock(ctx *MirBlockContext) {}

// ExitMirBlock is called when production mirBlock is exited.
func (s *BaseMinZListener) ExitMirBlock(ctx *MirBlockContext) {}

// EnterMinzBlock is called when production minzBlock is entered.
func (s *BaseMinZListener) EnterMinzBlock(ctx *MinzBlockContext) {}

// ExitMinzBlock is called when production minzBlock is exited.
func (s *BaseMinZListener) ExitMinzBlock(ctx *MinzBlockContext) {}

// EnterMinzContent is called when production minzContent is entered.
func (s *BaseMinZListener) EnterMinzContent(ctx *MinzContentContext) {}

// ExitMinzContent is called when production minzContent is exited.
func (s *BaseMinZListener) ExitMinzContent(ctx *MinzContentContext) {}

// EnterTargetBlock is called when production targetBlock is entered.
func (s *BaseMinZListener) EnterTargetBlock(ctx *TargetBlockContext) {}

// ExitTargetBlock is called when production targetBlock is exited.
func (s *BaseMinZListener) ExitTargetBlock(ctx *TargetBlockContext) {}

// EnterShift is called when production Shift is entered.
func (s *BaseMinZListener) EnterShift(ctx *ShiftContext) {}

// ExitShift is called when production Shift is exited.
func (s *BaseMinZListener) ExitShift(ctx *ShiftContext) {}

// EnterCast is called when production Cast is entered.
func (s *BaseMinZListener) EnterCast(ctx *CastContext) {}

// ExitCast is called when production Cast is exited.
func (s *BaseMinZListener) ExitCast(ctx *CastContext) {}

// EnterCall is called when production Call is entered.
func (s *BaseMinZListener) EnterCall(ctx *CallContext) {}

// ExitCall is called when production Call is exited.
func (s *BaseMinZListener) ExitCall(ctx *CallContext) {}

// EnterIfExpr is called when production IfExpr is entered.
func (s *BaseMinZListener) EnterIfExpr(ctx *IfExprContext) {}

// ExitIfExpr is called when production IfExpr is exited.
func (s *BaseMinZListener) ExitIfExpr(ctx *IfExprContext) {}

// EnterIndexAccess is called when production IndexAccess is entered.
func (s *BaseMinZListener) EnterIndexAccess(ctx *IndexAccessContext) {}

// ExitIndexAccess is called when production IndexAccess is exited.
func (s *BaseMinZListener) ExitIndexAccess(ctx *IndexAccessContext) {}

// EnterRelational is called when production Relational is entered.
func (s *BaseMinZListener) EnterRelational(ctx *RelationalContext) {}

// ExitRelational is called when production Relational is exited.
func (s *BaseMinZListener) ExitRelational(ctx *RelationalContext) {}

// EnterErrorCheck is called when production ErrorCheck is entered.
func (s *BaseMinZListener) EnterErrorCheck(ctx *ErrorCheckContext) {}

// ExitErrorCheck is called when production ErrorCheck is exited.
func (s *BaseMinZListener) ExitErrorCheck(ctx *ErrorCheckContext) {}

// EnterMetafunctionCall is called when production MetafunctionCall is entered.
func (s *BaseMinZListener) EnterMetafunctionCall(ctx *MetafunctionCallContext) {}

// ExitMetafunctionCall is called when production MetafunctionCall is exited.
func (s *BaseMinZListener) ExitMetafunctionCall(ctx *MetafunctionCallContext) {}

// EnterRange is called when production Range is entered.
func (s *BaseMinZListener) EnterRange(ctx *RangeContext) {}

// ExitRange is called when production Range is exited.
func (s *BaseMinZListener) ExitRange(ctx *RangeContext) {}

// EnterUnary is called when production Unary is entered.
func (s *BaseMinZListener) EnterUnary(ctx *UnaryContext) {}

// ExitUnary is called when production Unary is exited.
func (s *BaseMinZListener) ExitUnary(ctx *UnaryContext) {}

// EnterLogicalOr is called when production LogicalOr is entered.
func (s *BaseMinZListener) EnterLogicalOr(ctx *LogicalOrContext) {}

// ExitLogicalOr is called when production LogicalOr is exited.
func (s *BaseMinZListener) ExitLogicalOr(ctx *LogicalOrContext) {}

// EnterMultiplicative is called when production Multiplicative is entered.
func (s *BaseMinZListener) EnterMultiplicative(ctx *MultiplicativeContext) {}

// ExitMultiplicative is called when production Multiplicative is exited.
func (s *BaseMinZListener) ExitMultiplicative(ctx *MultiplicativeContext) {}

// EnterAdditive is called when production Additive is entered.
func (s *BaseMinZListener) EnterAdditive(ctx *AdditiveContext) {}

// ExitAdditive is called when production Additive is exited.
func (s *BaseMinZListener) ExitAdditive(ctx *AdditiveContext) {}

// EnterMemberAccess is called when production MemberAccess is entered.
func (s *BaseMinZListener) EnterMemberAccess(ctx *MemberAccessContext) {}

// ExitMemberAccess is called when production MemberAccess is exited.
func (s *BaseMinZListener) ExitMemberAccess(ctx *MemberAccessContext) {}

// EnterErrorDefault is called when production ErrorDefault is entered.
func (s *BaseMinZListener) EnterErrorDefault(ctx *ErrorDefaultContext) {}

// ExitErrorDefault is called when production ErrorDefault is exited.
func (s *BaseMinZListener) ExitErrorDefault(ctx *ErrorDefaultContext) {}

// EnterBitwiseXor is called when production BitwiseXor is entered.
func (s *BaseMinZListener) EnterBitwiseXor(ctx *BitwiseXorContext) {}

// ExitBitwiseXor is called when production BitwiseXor is exited.
func (s *BaseMinZListener) ExitBitwiseXor(ctx *BitwiseXorContext) {}

// EnterBitwiseOr is called when production BitwiseOr is entered.
func (s *BaseMinZListener) EnterBitwiseOr(ctx *BitwiseOrContext) {}

// ExitBitwiseOr is called when production BitwiseOr is exited.
func (s *BaseMinZListener) ExitBitwiseOr(ctx *BitwiseOrContext) {}

// EnterPrimary is called when production Primary is entered.
func (s *BaseMinZListener) EnterPrimary(ctx *PrimaryContext) {}

// ExitPrimary is called when production Primary is exited.
func (s *BaseMinZListener) ExitPrimary(ctx *PrimaryContext) {}

// EnterLogicalAnd is called when production LogicalAnd is entered.
func (s *BaseMinZListener) EnterLogicalAnd(ctx *LogicalAndContext) {}

// ExitLogicalAnd is called when production LogicalAnd is exited.
func (s *BaseMinZListener) ExitLogicalAnd(ctx *LogicalAndContext) {}

// EnterWhenExpr is called when production WhenExpr is entered.
func (s *BaseMinZListener) EnterWhenExpr(ctx *WhenExprContext) {}

// ExitWhenExpr is called when production WhenExpr is exited.
func (s *BaseMinZListener) ExitWhenExpr(ctx *WhenExprContext) {}

// EnterBitwiseAnd is called when production BitwiseAnd is entered.
func (s *BaseMinZListener) EnterBitwiseAnd(ctx *BitwiseAndContext) {}

// ExitBitwiseAnd is called when production BitwiseAnd is exited.
func (s *BaseMinZListener) ExitBitwiseAnd(ctx *BitwiseAndContext) {}

// EnterEquality is called when production Equality is entered.
func (s *BaseMinZListener) EnterEquality(ctx *EqualityContext) {}

// ExitEquality is called when production Equality is exited.
func (s *BaseMinZListener) ExitEquality(ctx *EqualityContext) {}

// EnterLambda is called when production Lambda is entered.
func (s *BaseMinZListener) EnterLambda(ctx *LambdaContext) {}

// ExitLambda is called when production Lambda is exited.
func (s *BaseMinZListener) ExitLambda(ctx *LambdaContext) {}

// EnterTernaryExpr is called when production TernaryExpr is entered.
func (s *BaseMinZListener) EnterTernaryExpr(ctx *TernaryExprContext) {}

// ExitTernaryExpr is called when production TernaryExpr is exited.
func (s *BaseMinZListener) ExitTernaryExpr(ctx *TernaryExprContext) {}

// EnterPrimaryExpression is called when production primaryExpression is entered.
func (s *BaseMinZListener) EnterPrimaryExpression(ctx *PrimaryExpressionContext) {}

// ExitPrimaryExpression is called when production primaryExpression is exited.
func (s *BaseMinZListener) ExitPrimaryExpression(ctx *PrimaryExpressionContext) {}

// EnterLiteral is called when production literal is entered.
func (s *BaseMinZListener) EnterLiteral(ctx *LiteralContext) {}

// ExitLiteral is called when production literal is exited.
func (s *BaseMinZListener) ExitLiteral(ctx *LiteralContext) {}

// EnterArrayLiteral is called when production arrayLiteral is entered.
func (s *BaseMinZListener) EnterArrayLiteral(ctx *ArrayLiteralContext) {}

// ExitArrayLiteral is called when production arrayLiteral is exited.
func (s *BaseMinZListener) ExitArrayLiteral(ctx *ArrayLiteralContext) {}

// EnterArrayInitializer is called when production arrayInitializer is entered.
func (s *BaseMinZListener) EnterArrayInitializer(ctx *ArrayInitializerContext) {}

// ExitArrayInitializer is called when production arrayInitializer is exited.
func (s *BaseMinZListener) ExitArrayInitializer(ctx *ArrayInitializerContext) {}

// EnterStructLiteral is called when production structLiteral is entered.
func (s *BaseMinZListener) EnterStructLiteral(ctx *StructLiteralContext) {}

// ExitStructLiteral is called when production structLiteral is exited.
func (s *BaseMinZListener) ExitStructLiteral(ctx *StructLiteralContext) {}

// EnterFieldInitializer is called when production fieldInitializer is entered.
func (s *BaseMinZListener) EnterFieldInitializer(ctx *FieldInitializerContext) {}

// ExitFieldInitializer is called when production fieldInitializer is exited.
func (s *BaseMinZListener) ExitFieldInitializer(ctx *FieldInitializerContext) {}

// EnterTupleLiteral is called when production tupleLiteral is entered.
func (s *BaseMinZListener) EnterTupleLiteral(ctx *TupleLiteralContext) {}

// ExitTupleLiteral is called when production tupleLiteral is exited.
func (s *BaseMinZListener) ExitTupleLiteral(ctx *TupleLiteralContext) {}

// EnterInlineAssembly is called when production inlineAssembly is entered.
func (s *BaseMinZListener) EnterInlineAssembly(ctx *InlineAssemblyContext) {}

// ExitInlineAssembly is called when production inlineAssembly is exited.
func (s *BaseMinZListener) ExitInlineAssembly(ctx *InlineAssemblyContext) {}

// EnterAsmOutputList is called when production asmOutputList is entered.
func (s *BaseMinZListener) EnterAsmOutputList(ctx *AsmOutputListContext) {}

// ExitAsmOutputList is called when production asmOutputList is exited.
func (s *BaseMinZListener) ExitAsmOutputList(ctx *AsmOutputListContext) {}

// EnterAsmInputList is called when production asmInputList is entered.
func (s *BaseMinZListener) EnterAsmInputList(ctx *AsmInputListContext) {}

// ExitAsmInputList is called when production asmInputList is exited.
func (s *BaseMinZListener) ExitAsmInputList(ctx *AsmInputListContext) {}

// EnterAsmClobberList is called when production asmClobberList is entered.
func (s *BaseMinZListener) EnterAsmClobberList(ctx *AsmClobberListContext) {}

// ExitAsmClobberList is called when production asmClobberList is exited.
func (s *BaseMinZListener) ExitAsmClobberList(ctx *AsmClobberListContext) {}

// EnterAsmOutput is called when production asmOutput is entered.
func (s *BaseMinZListener) EnterAsmOutput(ctx *AsmOutputContext) {}

// ExitAsmOutput is called when production asmOutput is exited.
func (s *BaseMinZListener) ExitAsmOutput(ctx *AsmOutputContext) {}

// EnterAsmInput is called when production asmInput is entered.
func (s *BaseMinZListener) EnterAsmInput(ctx *AsmInputContext) {}

// ExitAsmInput is called when production asmInput is exited.
func (s *BaseMinZListener) ExitAsmInput(ctx *AsmInputContext) {}

// EnterSizeofExpression is called when production sizeofExpression is entered.
func (s *BaseMinZListener) EnterSizeofExpression(ctx *SizeofExpressionContext) {}

// ExitSizeofExpression is called when production sizeofExpression is exited.
func (s *BaseMinZListener) ExitSizeofExpression(ctx *SizeofExpressionContext) {}

// EnterAlignofExpression is called when production alignofExpression is entered.
func (s *BaseMinZListener) EnterAlignofExpression(ctx *AlignofExpressionContext) {}

// ExitAlignofExpression is called when production alignofExpression is exited.
func (s *BaseMinZListener) ExitAlignofExpression(ctx *AlignofExpressionContext) {}

// EnterErrorLiteral is called when production errorLiteral is entered.
func (s *BaseMinZListener) EnterErrorLiteral(ctx *ErrorLiteralContext) {}

// ExitErrorLiteral is called when production errorLiteral is exited.
func (s *BaseMinZListener) ExitErrorLiteral(ctx *ErrorLiteralContext) {}

// EnterLambdaExpression is called when production lambdaExpression is entered.
func (s *BaseMinZListener) EnterLambdaExpression(ctx *LambdaExpressionContext) {}

// ExitLambdaExpression is called when production lambdaExpression is exited.
func (s *BaseMinZListener) ExitLambdaExpression(ctx *LambdaExpressionContext) {}

// EnterLambdaParams is called when production lambdaParams is entered.
func (s *BaseMinZListener) EnterLambdaParams(ctx *LambdaParamsContext) {}

// ExitLambdaParams is called when production lambdaParams is exited.
func (s *BaseMinZListener) ExitLambdaParams(ctx *LambdaParamsContext) {}

// EnterLambdaParam is called when production lambdaParam is entered.
func (s *BaseMinZListener) EnterLambdaParam(ctx *LambdaParamContext) {}

// ExitLambdaParam is called when production lambdaParam is exited.
func (s *BaseMinZListener) ExitLambdaParam(ctx *LambdaParamContext) {}

// EnterIfExpression is called when production ifExpression is entered.
func (s *BaseMinZListener) EnterIfExpression(ctx *IfExpressionContext) {}

// ExitIfExpression is called when production ifExpression is exited.
func (s *BaseMinZListener) ExitIfExpression(ctx *IfExpressionContext) {}

// EnterTernaryExpression is called when production ternaryExpression is entered.
func (s *BaseMinZListener) EnterTernaryExpression(ctx *TernaryExpressionContext) {}

// ExitTernaryExpression is called when production ternaryExpression is exited.
func (s *BaseMinZListener) ExitTernaryExpression(ctx *TernaryExpressionContext) {}

// EnterWhenExpression is called when production whenExpression is entered.
func (s *BaseMinZListener) EnterWhenExpression(ctx *WhenExpressionContext) {}

// ExitWhenExpression is called when production whenExpression is exited.
func (s *BaseMinZListener) ExitWhenExpression(ctx *WhenExpressionContext) {}

// EnterWhenArm is called when production whenArm is entered.
func (s *BaseMinZListener) EnterWhenArm(ctx *WhenArmContext) {}

// ExitWhenArm is called when production whenArm is exited.
func (s *BaseMinZListener) ExitWhenArm(ctx *WhenArmContext) {}

// EnterMetafunctionExpr is called when production metafunctionExpr is entered.
func (s *BaseMinZListener) EnterMetafunctionExpr(ctx *MetafunctionExprContext) {}

// ExitMetafunctionExpr is called when production metafunctionExpr is exited.
func (s *BaseMinZListener) ExitMetafunctionExpr(ctx *MetafunctionExprContext) {}

// EnterArgumentList is called when production argumentList is entered.
func (s *BaseMinZListener) EnterArgumentList(ctx *ArgumentListContext) {}

// ExitArgumentList is called when production argumentList is exited.
func (s *BaseMinZListener) ExitArgumentList(ctx *ArgumentListContext) {}

// EnterExpressionList is called when production expressionList is entered.
func (s *BaseMinZListener) EnterExpressionList(ctx *ExpressionListContext) {}

// ExitExpressionList is called when production expressionList is exited.
func (s *BaseMinZListener) ExitExpressionList(ctx *ExpressionListContext) {}
