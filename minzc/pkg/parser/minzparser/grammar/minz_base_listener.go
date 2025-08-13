// Code generated from grammar/MinZ.g4 by ANTLR 4.13.2. DO NOT EDIT.

package minzparser // MinZ
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

// EnterSourceFile is called when production sourceFile is entered.
func (s *BaseMinZListener) EnterSourceFile(ctx *SourceFileContext) {}

// ExitSourceFile is called when production sourceFile is exited.
func (s *BaseMinZListener) ExitSourceFile(ctx *SourceFileContext) {}

// EnterImportStatement is called when production importStatement is entered.
func (s *BaseMinZListener) EnterImportStatement(ctx *ImportStatementContext) {}

// ExitImportStatement is called when production importStatement is exited.
func (s *BaseMinZListener) ExitImportStatement(ctx *ImportStatementContext) {}

// EnterImportPath is called when production importPath is entered.
func (s *BaseMinZListener) EnterImportPath(ctx *ImportPathContext) {}

// ExitImportPath is called when production importPath is exited.
func (s *BaseMinZListener) ExitImportPath(ctx *ImportPathContext) {}

// EnterDeclaration is called when production declaration is entered.
func (s *BaseMinZListener) EnterDeclaration(ctx *DeclarationContext) {}

// ExitDeclaration is called when production declaration is exited.
func (s *BaseMinZListener) ExitDeclaration(ctx *DeclarationContext) {}

// EnterFunctionDeclaration is called when production functionDeclaration is entered.
func (s *BaseMinZListener) EnterFunctionDeclaration(ctx *FunctionDeclarationContext) {}

// ExitFunctionDeclaration is called when production functionDeclaration is exited.
func (s *BaseMinZListener) ExitFunctionDeclaration(ctx *FunctionDeclarationContext) {}

// EnterFunctionPrefix is called when production functionPrefix is entered.
func (s *BaseMinZListener) EnterFunctionPrefix(ctx *FunctionPrefixContext) {}

// ExitFunctionPrefix is called when production functionPrefix is exited.
func (s *BaseMinZListener) ExitFunctionPrefix(ctx *FunctionPrefixContext) {}

// EnterVisibility is called when production visibility is entered.
func (s *BaseMinZListener) EnterVisibility(ctx *VisibilityContext) {}

// ExitVisibility is called when production visibility is exited.
func (s *BaseMinZListener) ExitVisibility(ctx *VisibilityContext) {}

// EnterGenericParams is called when production genericParams is entered.
func (s *BaseMinZListener) EnterGenericParams(ctx *GenericParamsContext) {}

// ExitGenericParams is called when production genericParams is exited.
func (s *BaseMinZListener) ExitGenericParams(ctx *GenericParamsContext) {}

// EnterParameterList is called when production parameterList is entered.
func (s *BaseMinZListener) EnterParameterList(ctx *ParameterListContext) {}

// ExitParameterList is called when production parameterList is exited.
func (s *BaseMinZListener) ExitParameterList(ctx *ParameterListContext) {}

// EnterParameter is called when production parameter is entered.
func (s *BaseMinZListener) EnterParameter(ctx *ParameterContext) {}

// ExitParameter is called when production parameter is exited.
func (s *BaseMinZListener) ExitParameter(ctx *ParameterContext) {}

// EnterReturnType is called when production returnType is entered.
func (s *BaseMinZListener) EnterReturnType(ctx *ReturnTypeContext) {}

// ExitReturnType is called when production returnType is exited.
func (s *BaseMinZListener) ExitReturnType(ctx *ReturnTypeContext) {}

// EnterErrorReturnType is called when production errorReturnType is entered.
func (s *BaseMinZListener) EnterErrorReturnType(ctx *ErrorReturnTypeContext) {}

// ExitErrorReturnType is called when production errorReturnType is exited.
func (s *BaseMinZListener) ExitErrorReturnType(ctx *ErrorReturnTypeContext) {}

// EnterStructDeclaration is called when production structDeclaration is entered.
func (s *BaseMinZListener) EnterStructDeclaration(ctx *StructDeclarationContext) {}

// ExitStructDeclaration is called when production structDeclaration is exited.
func (s *BaseMinZListener) ExitStructDeclaration(ctx *StructDeclarationContext) {}

// EnterFieldList is called when production fieldList is entered.
func (s *BaseMinZListener) EnterFieldList(ctx *FieldListContext) {}

// ExitFieldList is called when production fieldList is exited.
func (s *BaseMinZListener) ExitFieldList(ctx *FieldListContext) {}

// EnterField is called when production field is entered.
func (s *BaseMinZListener) EnterField(ctx *FieldContext) {}

// ExitField is called when production field is exited.
func (s *BaseMinZListener) ExitField(ctx *FieldContext) {}

// EnterEnumDeclaration is called when production enumDeclaration is entered.
func (s *BaseMinZListener) EnterEnumDeclaration(ctx *EnumDeclarationContext) {}

// ExitEnumDeclaration is called when production enumDeclaration is exited.
func (s *BaseMinZListener) ExitEnumDeclaration(ctx *EnumDeclarationContext) {}

// EnterEnumMemberList is called when production enumMemberList is entered.
func (s *BaseMinZListener) EnterEnumMemberList(ctx *EnumMemberListContext) {}

// ExitEnumMemberList is called when production enumMemberList is exited.
func (s *BaseMinZListener) ExitEnumMemberList(ctx *EnumMemberListContext) {}

// EnterEnumMember is called when production enumMember is entered.
func (s *BaseMinZListener) EnterEnumMember(ctx *EnumMemberContext) {}

// ExitEnumMember is called when production enumMember is exited.
func (s *BaseMinZListener) ExitEnumMember(ctx *EnumMemberContext) {}

// EnterTypeAliasDeclaration is called when production typeAliasDeclaration is entered.
func (s *BaseMinZListener) EnterTypeAliasDeclaration(ctx *TypeAliasDeclarationContext) {}

// ExitTypeAliasDeclaration is called when production typeAliasDeclaration is exited.
func (s *BaseMinZListener) ExitTypeAliasDeclaration(ctx *TypeAliasDeclarationContext) {}

// EnterInterfaceDeclaration is called when production interfaceDeclaration is entered.
func (s *BaseMinZListener) EnterInterfaceDeclaration(ctx *InterfaceDeclarationContext) {}

// ExitInterfaceDeclaration is called when production interfaceDeclaration is exited.
func (s *BaseMinZListener) ExitInterfaceDeclaration(ctx *InterfaceDeclarationContext) {}

// EnterInterfaceMethodList is called when production interfaceMethodList is entered.
func (s *BaseMinZListener) EnterInterfaceMethodList(ctx *InterfaceMethodListContext) {}

// ExitInterfaceMethodList is called when production interfaceMethodList is exited.
func (s *BaseMinZListener) ExitInterfaceMethodList(ctx *InterfaceMethodListContext) {}

// EnterInterfaceMethod is called when production interfaceMethod is entered.
func (s *BaseMinZListener) EnterInterfaceMethod(ctx *InterfaceMethodContext) {}

// ExitInterfaceMethod is called when production interfaceMethod is exited.
func (s *BaseMinZListener) ExitInterfaceMethod(ctx *InterfaceMethodContext) {}

// EnterImplBlock is called when production implBlock is entered.
func (s *BaseMinZListener) EnterImplBlock(ctx *ImplBlockContext) {}

// ExitImplBlock is called when production implBlock is exited.
func (s *BaseMinZListener) ExitImplBlock(ctx *ImplBlockContext) {}

// EnterConstDeclaration is called when production constDeclaration is entered.
func (s *BaseMinZListener) EnterConstDeclaration(ctx *ConstDeclarationContext) {}

// ExitConstDeclaration is called when production constDeclaration is exited.
func (s *BaseMinZListener) ExitConstDeclaration(ctx *ConstDeclarationContext) {}

// EnterGlobalVarDeclaration is called when production globalVarDeclaration is entered.
func (s *BaseMinZListener) EnterGlobalVarDeclaration(ctx *GlobalVarDeclarationContext) {}

// ExitGlobalVarDeclaration is called when production globalVarDeclaration is exited.
func (s *BaseMinZListener) ExitGlobalVarDeclaration(ctx *GlobalVarDeclarationContext) {}

// EnterCompileTimeDeclaration is called when production compileTimeDeclaration is entered.
func (s *BaseMinZListener) EnterCompileTimeDeclaration(ctx *CompileTimeDeclarationContext) {}

// ExitCompileTimeDeclaration is called when production compileTimeDeclaration is exited.
func (s *BaseMinZListener) ExitCompileTimeDeclaration(ctx *CompileTimeDeclarationContext) {}

// EnterCompileTimeIf is called when production compileTimeIf is entered.
func (s *BaseMinZListener) EnterCompileTimeIf(ctx *CompileTimeIfContext) {}

// ExitCompileTimeIf is called when production compileTimeIf is exited.
func (s *BaseMinZListener) ExitCompileTimeIf(ctx *CompileTimeIfContext) {}

// EnterCompileTimeMinz is called when production compileTimeMinz is entered.
func (s *BaseMinZListener) EnterCompileTimeMinz(ctx *CompileTimeMinzContext) {}

// ExitCompileTimeMinz is called when production compileTimeMinz is exited.
func (s *BaseMinZListener) ExitCompileTimeMinz(ctx *CompileTimeMinzContext) {}

// EnterCompileTimeMir is called when production compileTimeMir is entered.
func (s *BaseMinZListener) EnterCompileTimeMir(ctx *CompileTimeMirContext) {}

// ExitCompileTimeMir is called when production compileTimeMir is exited.
func (s *BaseMinZListener) ExitCompileTimeMir(ctx *CompileTimeMirContext) {}

// EnterMirBlock is called when production mirBlock is entered.
func (s *BaseMinZListener) EnterMirBlock(ctx *MirBlockContext) {}

// ExitMirBlock is called when production mirBlock is exited.
func (s *BaseMinZListener) ExitMirBlock(ctx *MirBlockContext) {}

// EnterMirStatement is called when production mirStatement is entered.
func (s *BaseMinZListener) EnterMirStatement(ctx *MirStatementContext) {}

// ExitMirStatement is called when production mirStatement is exited.
func (s *BaseMinZListener) ExitMirStatement(ctx *MirStatementContext) {}

// EnterMirInstruction is called when production mirInstruction is entered.
func (s *BaseMinZListener) EnterMirInstruction(ctx *MirInstructionContext) {}

// ExitMirInstruction is called when production mirInstruction is exited.
func (s *BaseMinZListener) ExitMirInstruction(ctx *MirInstructionContext) {}

// EnterMirOperand is called when production mirOperand is entered.
func (s *BaseMinZListener) EnterMirOperand(ctx *MirOperandContext) {}

// ExitMirOperand is called when production mirOperand is exited.
func (s *BaseMinZListener) ExitMirOperand(ctx *MirOperandContext) {}

// EnterMirRegister is called when production mirRegister is entered.
func (s *BaseMinZListener) EnterMirRegister(ctx *MirRegisterContext) {}

// ExitMirRegister is called when production mirRegister is exited.
func (s *BaseMinZListener) ExitMirRegister(ctx *MirRegisterContext) {}

// EnterMirImmediate is called when production mirImmediate is entered.
func (s *BaseMinZListener) EnterMirImmediate(ctx *MirImmediateContext) {}

// ExitMirImmediate is called when production mirImmediate is exited.
func (s *BaseMinZListener) ExitMirImmediate(ctx *MirImmediateContext) {}

// EnterMirMemory is called when production mirMemory is entered.
func (s *BaseMinZListener) EnterMirMemory(ctx *MirMemoryContext) {}

// ExitMirMemory is called when production mirMemory is exited.
func (s *BaseMinZListener) ExitMirMemory(ctx *MirMemoryContext) {}

// EnterMirLabel is called when production mirLabel is entered.
func (s *BaseMinZListener) EnterMirLabel(ctx *MirLabelContext) {}

// ExitMirLabel is called when production mirLabel is exited.
func (s *BaseMinZListener) ExitMirLabel(ctx *MirLabelContext) {}

// EnterTargetBlock is called when production targetBlock is entered.
func (s *BaseMinZListener) EnterTargetBlock(ctx *TargetBlockContext) {}

// ExitTargetBlock is called when production targetBlock is exited.
func (s *BaseMinZListener) ExitTargetBlock(ctx *TargetBlockContext) {}

// EnterStatement is called when production statement is entered.
func (s *BaseMinZListener) EnterStatement(ctx *StatementContext) {}

// ExitStatement is called when production statement is exited.
func (s *BaseMinZListener) ExitStatement(ctx *StatementContext) {}

// EnterLetStatement is called when production letStatement is entered.
func (s *BaseMinZListener) EnterLetStatement(ctx *LetStatementContext) {}

// ExitLetStatement is called when production letStatement is exited.
func (s *BaseMinZListener) ExitLetStatement(ctx *LetStatementContext) {}

// EnterVarStatement is called when production varStatement is entered.
func (s *BaseMinZListener) EnterVarStatement(ctx *VarStatementContext) {}

// ExitVarStatement is called when production varStatement is exited.
func (s *BaseMinZListener) ExitVarStatement(ctx *VarStatementContext) {}

// EnterAssignmentStatement is called when production assignmentStatement is entered.
func (s *BaseMinZListener) EnterAssignmentStatement(ctx *AssignmentStatementContext) {}

// ExitAssignmentStatement is called when production assignmentStatement is exited.
func (s *BaseMinZListener) ExitAssignmentStatement(ctx *AssignmentStatementContext) {}

// EnterExpressionStatement is called when production expressionStatement is entered.
func (s *BaseMinZListener) EnterExpressionStatement(ctx *ExpressionStatementContext) {}

// ExitExpressionStatement is called when production expressionStatement is exited.
func (s *BaseMinZListener) ExitExpressionStatement(ctx *ExpressionStatementContext) {}

// EnterReturnStatement is called when production returnStatement is entered.
func (s *BaseMinZListener) EnterReturnStatement(ctx *ReturnStatementContext) {}

// ExitReturnStatement is called when production returnStatement is exited.
func (s *BaseMinZListener) ExitReturnStatement(ctx *ReturnStatementContext) {}

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

// EnterCaseStatement is called when production caseStatement is entered.
func (s *BaseMinZListener) EnterCaseStatement(ctx *CaseStatementContext) {}

// ExitCaseStatement is called when production caseStatement is exited.
func (s *BaseMinZListener) ExitCaseStatement(ctx *CaseStatementContext) {}

// EnterCaseArm is called when production caseArm is entered.
func (s *BaseMinZListener) EnterCaseArm(ctx *CaseArmContext) {}

// ExitCaseArm is called when production caseArm is exited.
func (s *BaseMinZListener) ExitCaseArm(ctx *CaseArmContext) {}

// EnterBlockStatement is called when production blockStatement is entered.
func (s *BaseMinZListener) EnterBlockStatement(ctx *BlockStatementContext) {}

// ExitBlockStatement is called when production blockStatement is exited.
func (s *BaseMinZListener) ExitBlockStatement(ctx *BlockStatementContext) {}

// EnterBlock is called when production block is entered.
func (s *BaseMinZListener) EnterBlock(ctx *BlockContext) {}

// ExitBlock is called when production block is exited.
func (s *BaseMinZListener) ExitBlock(ctx *BlockContext) {}

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

// EnterAsmStatement is called when production asmStatement is entered.
func (s *BaseMinZListener) EnterAsmStatement(ctx *AsmStatementContext) {}

// ExitAsmStatement is called when production asmStatement is exited.
func (s *BaseMinZListener) ExitAsmStatement(ctx *AsmStatementContext) {}

// EnterAsmBlock is called when production asmBlock is entered.
func (s *BaseMinZListener) EnterAsmBlock(ctx *AsmBlockContext) {}

// ExitAsmBlock is called when production asmBlock is exited.
func (s *BaseMinZListener) ExitAsmBlock(ctx *AsmBlockContext) {}

// EnterPattern is called when production pattern is entered.
func (s *BaseMinZListener) EnterPattern(ctx *PatternContext) {}

// ExitPattern is called when production pattern is exited.
func (s *BaseMinZListener) ExitPattern(ctx *PatternContext) {}

// EnterLiteralPattern is called when production literalPattern is entered.
func (s *BaseMinZListener) EnterLiteralPattern(ctx *LiteralPatternContext) {}

// ExitLiteralPattern is called when production literalPattern is exited.
func (s *BaseMinZListener) ExitLiteralPattern(ctx *LiteralPatternContext) {}

// EnterIdentifierPattern is called when production identifierPattern is entered.
func (s *BaseMinZListener) EnterIdentifierPattern(ctx *IdentifierPatternContext) {}

// ExitIdentifierPattern is called when production identifierPattern is exited.
func (s *BaseMinZListener) ExitIdentifierPattern(ctx *IdentifierPatternContext) {}

// EnterWildcardPattern is called when production wildcardPattern is entered.
func (s *BaseMinZListener) EnterWildcardPattern(ctx *WildcardPatternContext) {}

// ExitWildcardPattern is called when production wildcardPattern is exited.
func (s *BaseMinZListener) ExitWildcardPattern(ctx *WildcardPatternContext) {}

// EnterTuplePattern is called when production tuplePattern is entered.
func (s *BaseMinZListener) EnterTuplePattern(ctx *TuplePatternContext) {}

// ExitTuplePattern is called when production tuplePattern is exited.
func (s *BaseMinZListener) ExitTuplePattern(ctx *TuplePatternContext) {}

// EnterStructPattern is called when production structPattern is entered.
func (s *BaseMinZListener) EnterStructPattern(ctx *StructPatternContext) {}

// ExitStructPattern is called when production structPattern is exited.
func (s *BaseMinZListener) ExitStructPattern(ctx *StructPatternContext) {}

// EnterFieldPattern is called when production fieldPattern is entered.
func (s *BaseMinZListener) EnterFieldPattern(ctx *FieldPatternContext) {}

// ExitFieldPattern is called when production fieldPattern is exited.
func (s *BaseMinZListener) ExitFieldPattern(ctx *FieldPatternContext) {}

// EnterExpression is called when production expression is entered.
func (s *BaseMinZListener) EnterExpression(ctx *ExpressionContext) {}

// ExitExpression is called when production expression is exited.
func (s *BaseMinZListener) ExitExpression(ctx *ExpressionContext) {}

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

// EnterConditionalExpression is called when production conditionalExpression is entered.
func (s *BaseMinZListener) EnterConditionalExpression(ctx *ConditionalExpressionContext) {}

// ExitConditionalExpression is called when production conditionalExpression is exited.
func (s *BaseMinZListener) ExitConditionalExpression(ctx *ConditionalExpressionContext) {}

// EnterWhenExpression is called when production whenExpression is entered.
func (s *BaseMinZListener) EnterWhenExpression(ctx *WhenExpressionContext) {}

// ExitWhenExpression is called when production whenExpression is exited.
func (s *BaseMinZListener) ExitWhenExpression(ctx *WhenExpressionContext) {}

// EnterWhenArm is called when production whenArm is entered.
func (s *BaseMinZListener) EnterWhenArm(ctx *WhenArmContext) {}

// ExitWhenArm is called when production whenArm is exited.
func (s *BaseMinZListener) ExitWhenArm(ctx *WhenArmContext) {}

// EnterLogicalOrExpression is called when production logicalOrExpression is entered.
func (s *BaseMinZListener) EnterLogicalOrExpression(ctx *LogicalOrExpressionContext) {}

// ExitLogicalOrExpression is called when production logicalOrExpression is exited.
func (s *BaseMinZListener) ExitLogicalOrExpression(ctx *LogicalOrExpressionContext) {}

// EnterLogicalAndExpression is called when production logicalAndExpression is entered.
func (s *BaseMinZListener) EnterLogicalAndExpression(ctx *LogicalAndExpressionContext) {}

// ExitLogicalAndExpression is called when production logicalAndExpression is exited.
func (s *BaseMinZListener) ExitLogicalAndExpression(ctx *LogicalAndExpressionContext) {}

// EnterEqualityExpression is called when production equalityExpression is entered.
func (s *BaseMinZListener) EnterEqualityExpression(ctx *EqualityExpressionContext) {}

// ExitEqualityExpression is called when production equalityExpression is exited.
func (s *BaseMinZListener) ExitEqualityExpression(ctx *EqualityExpressionContext) {}

// EnterRelationalExpression is called when production relationalExpression is entered.
func (s *BaseMinZListener) EnterRelationalExpression(ctx *RelationalExpressionContext) {}

// ExitRelationalExpression is called when production relationalExpression is exited.
func (s *BaseMinZListener) ExitRelationalExpression(ctx *RelationalExpressionContext) {}

// EnterAdditiveExpression is called when production additiveExpression is entered.
func (s *BaseMinZListener) EnterAdditiveExpression(ctx *AdditiveExpressionContext) {}

// ExitAdditiveExpression is called when production additiveExpression is exited.
func (s *BaseMinZListener) ExitAdditiveExpression(ctx *AdditiveExpressionContext) {}

// EnterMultiplicativeExpression is called when production multiplicativeExpression is entered.
func (s *BaseMinZListener) EnterMultiplicativeExpression(ctx *MultiplicativeExpressionContext) {}

// ExitMultiplicativeExpression is called when production multiplicativeExpression is exited.
func (s *BaseMinZListener) ExitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) {}

// EnterCastExpression is called when production castExpression is entered.
func (s *BaseMinZListener) EnterCastExpression(ctx *CastExpressionContext) {}

// ExitCastExpression is called when production castExpression is exited.
func (s *BaseMinZListener) ExitCastExpression(ctx *CastExpressionContext) {}

// EnterUnaryExpression is called when production unaryExpression is entered.
func (s *BaseMinZListener) EnterUnaryExpression(ctx *UnaryExpressionContext) {}

// ExitUnaryExpression is called when production unaryExpression is exited.
func (s *BaseMinZListener) ExitUnaryExpression(ctx *UnaryExpressionContext) {}

// EnterPostfixExpression is called when production postfixExpression is entered.
func (s *BaseMinZListener) EnterPostfixExpression(ctx *PostfixExpressionContext) {}

// ExitPostfixExpression is called when production postfixExpression is exited.
func (s *BaseMinZListener) ExitPostfixExpression(ctx *PostfixExpressionContext) {}

// EnterPostfixOperator is called when production postfixOperator is entered.
func (s *BaseMinZListener) EnterPostfixOperator(ctx *PostfixOperatorContext) {}

// ExitPostfixOperator is called when production postfixOperator is exited.
func (s *BaseMinZListener) ExitPostfixOperator(ctx *PostfixOperatorContext) {}

// EnterArgumentList is called when production argumentList is entered.
func (s *BaseMinZListener) EnterArgumentList(ctx *ArgumentListContext) {}

// ExitArgumentList is called when production argumentList is exited.
func (s *BaseMinZListener) ExitArgumentList(ctx *ArgumentListContext) {}

// EnterPrimaryExpression is called when production primaryExpression is entered.
func (s *BaseMinZListener) EnterPrimaryExpression(ctx *PrimaryExpressionContext) {}

// ExitPrimaryExpression is called when production primaryExpression is exited.
func (s *BaseMinZListener) ExitPrimaryExpression(ctx *PrimaryExpressionContext) {}

// EnterLiteral is called when production literal is entered.
func (s *BaseMinZListener) EnterLiteral(ctx *LiteralContext) {}

// ExitLiteral is called when production literal is exited.
func (s *BaseMinZListener) ExitLiteral(ctx *LiteralContext) {}

// EnterNumberLiteral is called when production numberLiteral is entered.
func (s *BaseMinZListener) EnterNumberLiteral(ctx *NumberLiteralContext) {}

// ExitNumberLiteral is called when production numberLiteral is exited.
func (s *BaseMinZListener) ExitNumberLiteral(ctx *NumberLiteralContext) {}

// EnterStringLiteral is called when production stringLiteral is entered.
func (s *BaseMinZListener) EnterStringLiteral(ctx *StringLiteralContext) {}

// ExitStringLiteral is called when production stringLiteral is exited.
func (s *BaseMinZListener) ExitStringLiteral(ctx *StringLiteralContext) {}

// EnterCharLiteral is called when production charLiteral is entered.
func (s *BaseMinZListener) EnterCharLiteral(ctx *CharLiteralContext) {}

// ExitCharLiteral is called when production charLiteral is exited.
func (s *BaseMinZListener) ExitCharLiteral(ctx *CharLiteralContext) {}

// EnterBooleanLiteral is called when production booleanLiteral is entered.
func (s *BaseMinZListener) EnterBooleanLiteral(ctx *BooleanLiteralContext) {}

// ExitBooleanLiteral is called when production booleanLiteral is exited.
func (s *BaseMinZListener) ExitBooleanLiteral(ctx *BooleanLiteralContext) {}

// EnterArrayLiteral is called when production arrayLiteral is entered.
func (s *BaseMinZListener) EnterArrayLiteral(ctx *ArrayLiteralContext) {}

// ExitArrayLiteral is called when production arrayLiteral is exited.
func (s *BaseMinZListener) ExitArrayLiteral(ctx *ArrayLiteralContext) {}

// EnterStructLiteral is called when production structLiteral is entered.
func (s *BaseMinZListener) EnterStructLiteral(ctx *StructLiteralContext) {}

// ExitStructLiteral is called when production structLiteral is exited.
func (s *BaseMinZListener) ExitStructLiteral(ctx *StructLiteralContext) {}

// EnterFieldInit is called when production fieldInit is entered.
func (s *BaseMinZListener) EnterFieldInit(ctx *FieldInitContext) {}

// ExitFieldInit is called when production fieldInit is exited.
func (s *BaseMinZListener) ExitFieldInit(ctx *FieldInitContext) {}

// EnterMetafunction is called when production metafunction is entered.
func (s *BaseMinZListener) EnterMetafunction(ctx *MetafunctionContext) {}

// ExitMetafunction is called when production metafunction is exited.
func (s *BaseMinZListener) ExitMetafunction(ctx *MetafunctionContext) {}

// EnterLuaBlock is called when production luaBlock is entered.
func (s *BaseMinZListener) EnterLuaBlock(ctx *LuaBlockContext) {}

// ExitLuaBlock is called when production luaBlock is exited.
func (s *BaseMinZListener) ExitLuaBlock(ctx *LuaBlockContext) {}

// EnterInlineAssembly is called when production inlineAssembly is entered.
func (s *BaseMinZListener) EnterInlineAssembly(ctx *InlineAssemblyContext) {}

// ExitInlineAssembly is called when production inlineAssembly is exited.
func (s *BaseMinZListener) ExitInlineAssembly(ctx *InlineAssemblyContext) {}

// EnterAsmOperand is called when production asmOperand is entered.
func (s *BaseMinZListener) EnterAsmOperand(ctx *AsmOperandContext) {}

// ExitAsmOperand is called when production asmOperand is exited.
func (s *BaseMinZListener) ExitAsmOperand(ctx *AsmOperandContext) {}

// EnterAsmConstraint is called when production asmConstraint is entered.
func (s *BaseMinZListener) EnterAsmConstraint(ctx *AsmConstraintContext) {}

// ExitAsmConstraint is called when production asmConstraint is exited.
func (s *BaseMinZListener) ExitAsmConstraint(ctx *AsmConstraintContext) {}

// EnterType is called when production type is entered.
func (s *BaseMinZListener) EnterType(ctx *TypeContext) {}

// ExitType is called when production type is exited.
func (s *BaseMinZListener) ExitType(ctx *TypeContext) {}

// EnterPrimitiveType is called when production primitiveType is entered.
func (s *BaseMinZListener) EnterPrimitiveType(ctx *PrimitiveTypeContext) {}

// ExitPrimitiveType is called when production primitiveType is exited.
func (s *BaseMinZListener) ExitPrimitiveType(ctx *PrimitiveTypeContext) {}

// EnterArrayType is called when production arrayType is entered.
func (s *BaseMinZListener) EnterArrayType(ctx *ArrayTypeContext) {}

// ExitArrayType is called when production arrayType is exited.
func (s *BaseMinZListener) ExitArrayType(ctx *ArrayTypeContext) {}

// EnterPointerType is called when production pointerType is entered.
func (s *BaseMinZListener) EnterPointerType(ctx *PointerTypeContext) {}

// ExitPointerType is called when production pointerType is exited.
func (s *BaseMinZListener) ExitPointerType(ctx *PointerTypeContext) {}

// EnterFunctionType is called when production functionType is entered.
func (s *BaseMinZListener) EnterFunctionType(ctx *FunctionTypeContext) {}

// ExitFunctionType is called when production functionType is exited.
func (s *BaseMinZListener) ExitFunctionType(ctx *FunctionTypeContext) {}

// EnterTypeList is called when production typeList is entered.
func (s *BaseMinZListener) EnterTypeList(ctx *TypeListContext) {}

// ExitTypeList is called when production typeList is exited.
func (s *BaseMinZListener) ExitTypeList(ctx *TypeListContext) {}

// EnterStructType is called when production structType is entered.
func (s *BaseMinZListener) EnterStructType(ctx *StructTypeContext) {}

// ExitStructType is called when production structType is exited.
func (s *BaseMinZListener) ExitStructType(ctx *StructTypeContext) {}

// EnterEnumType is called when production enumType is entered.
func (s *BaseMinZListener) EnterEnumType(ctx *EnumTypeContext) {}

// ExitEnumType is called when production enumType is exited.
func (s *BaseMinZListener) ExitEnumType(ctx *EnumTypeContext) {}

// EnterBitStructType is called when production bitStructType is entered.
func (s *BaseMinZListener) EnterBitStructType(ctx *BitStructTypeContext) {}

// ExitBitStructType is called when production bitStructType is exited.
func (s *BaseMinZListener) ExitBitStructType(ctx *BitStructTypeContext) {}

// EnterBitFieldList is called when production bitFieldList is entered.
func (s *BaseMinZListener) EnterBitFieldList(ctx *BitFieldListContext) {}

// ExitBitFieldList is called when production bitFieldList is exited.
func (s *BaseMinZListener) ExitBitFieldList(ctx *BitFieldListContext) {}

// EnterBitField is called when production bitField is entered.
func (s *BaseMinZListener) EnterBitField(ctx *BitFieldContext) {}

// ExitBitField is called when production bitField is exited.
func (s *BaseMinZListener) ExitBitField(ctx *BitFieldContext) {}

// EnterTypeIdentifier is called when production typeIdentifier is entered.
func (s *BaseMinZListener) EnterTypeIdentifier(ctx *TypeIdentifierContext) {}

// ExitTypeIdentifier is called when production typeIdentifier is exited.
func (s *BaseMinZListener) ExitTypeIdentifier(ctx *TypeIdentifierContext) {}

// EnterErrorType is called when production errorType is entered.
func (s *BaseMinZListener) EnterErrorType(ctx *ErrorTypeContext) {}

// ExitErrorType is called when production errorType is exited.
func (s *BaseMinZListener) ExitErrorType(ctx *ErrorTypeContext) {}
