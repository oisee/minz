// Code generated from grammar/MinZ.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // MinZ

import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by MinZParser.
type MinZVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by MinZParser#sourceFile.
	VisitSourceFile(ctx *SourceFileContext) interface{}

	// Visit a parse tree produced by MinZParser#importStatement.
	VisitImportStatement(ctx *ImportStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#importPath.
	VisitImportPath(ctx *ImportPathContext) interface{}

	// Visit a parse tree produced by MinZParser#declaration.
	VisitDeclaration(ctx *DeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#functionDeclaration.
	VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#functionPrefix.
	VisitFunctionPrefix(ctx *FunctionPrefixContext) interface{}

	// Visit a parse tree produced by MinZParser#visibility.
	VisitVisibility(ctx *VisibilityContext) interface{}

	// Visit a parse tree produced by MinZParser#genericParams.
	VisitGenericParams(ctx *GenericParamsContext) interface{}

	// Visit a parse tree produced by MinZParser#parameterList.
	VisitParameterList(ctx *ParameterListContext) interface{}

	// Visit a parse tree produced by MinZParser#parameter.
	VisitParameter(ctx *ParameterContext) interface{}

	// Visit a parse tree produced by MinZParser#returnType.
	VisitReturnType(ctx *ReturnTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#errorReturnType.
	VisitErrorReturnType(ctx *ErrorReturnTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#structDeclaration.
	VisitStructDeclaration(ctx *StructDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#fieldList.
	VisitFieldList(ctx *FieldListContext) interface{}

	// Visit a parse tree produced by MinZParser#field.
	VisitField(ctx *FieldContext) interface{}

	// Visit a parse tree produced by MinZParser#enumDeclaration.
	VisitEnumDeclaration(ctx *EnumDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#enumMemberList.
	VisitEnumMemberList(ctx *EnumMemberListContext) interface{}

	// Visit a parse tree produced by MinZParser#enumMember.
	VisitEnumMember(ctx *EnumMemberContext) interface{}

	// Visit a parse tree produced by MinZParser#typeAliasDeclaration.
	VisitTypeAliasDeclaration(ctx *TypeAliasDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#interfaceDeclaration.
	VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#interfaceMethodList.
	VisitInterfaceMethodList(ctx *InterfaceMethodListContext) interface{}

	// Visit a parse tree produced by MinZParser#interfaceMethod.
	VisitInterfaceMethod(ctx *InterfaceMethodContext) interface{}

	// Visit a parse tree produced by MinZParser#implBlock.
	VisitImplBlock(ctx *ImplBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#constDeclaration.
	VisitConstDeclaration(ctx *ConstDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#globalVarDeclaration.
	VisitGlobalVarDeclaration(ctx *GlobalVarDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#compileTimeDeclaration.
	VisitCompileTimeDeclaration(ctx *CompileTimeDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#compileTimeIf.
	VisitCompileTimeIf(ctx *CompileTimeIfContext) interface{}

	// Visit a parse tree produced by MinZParser#compileTimeMinz.
	VisitCompileTimeMinz(ctx *CompileTimeMinzContext) interface{}

	// Visit a parse tree produced by MinZParser#compileTimeMir.
	VisitCompileTimeMir(ctx *CompileTimeMirContext) interface{}

	// Visit a parse tree produced by MinZParser#mirBlock.
	VisitMirBlock(ctx *MirBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#mirStatement.
	VisitMirStatement(ctx *MirStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#mirInstruction.
	VisitMirInstruction(ctx *MirInstructionContext) interface{}

	// Visit a parse tree produced by MinZParser#mirOperand.
	VisitMirOperand(ctx *MirOperandContext) interface{}

	// Visit a parse tree produced by MinZParser#mirRegister.
	VisitMirRegister(ctx *MirRegisterContext) interface{}

	// Visit a parse tree produced by MinZParser#mirImmediate.
	VisitMirImmediate(ctx *MirImmediateContext) interface{}

	// Visit a parse tree produced by MinZParser#mirMemory.
	VisitMirMemory(ctx *MirMemoryContext) interface{}

	// Visit a parse tree produced by MinZParser#mirLabel.
	VisitMirLabel(ctx *MirLabelContext) interface{}

	// Visit a parse tree produced by MinZParser#targetBlock.
	VisitTargetBlock(ctx *TargetBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#statement.
	VisitStatement(ctx *StatementContext) interface{}

	// Visit a parse tree produced by MinZParser#letStatement.
	VisitLetStatement(ctx *LetStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#varStatement.
	VisitVarStatement(ctx *VarStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#assignmentStatement.
	VisitAssignmentStatement(ctx *AssignmentStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#expressionStatement.
	VisitExpressionStatement(ctx *ExpressionStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#returnStatement.
	VisitReturnStatement(ctx *ReturnStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#ifStatement.
	VisitIfStatement(ctx *IfStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#whileStatement.
	VisitWhileStatement(ctx *WhileStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#forStatement.
	VisitForStatement(ctx *ForStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#loopStatement.
	VisitLoopStatement(ctx *LoopStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#caseStatement.
	VisitCaseStatement(ctx *CaseStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#caseArm.
	VisitCaseArm(ctx *CaseArmContext) interface{}

	// Visit a parse tree produced by MinZParser#blockStatement.
	VisitBlockStatement(ctx *BlockStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#block.
	VisitBlock(ctx *BlockContext) interface{}

	// Visit a parse tree produced by MinZParser#breakStatement.
	VisitBreakStatement(ctx *BreakStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#continueStatement.
	VisitContinueStatement(ctx *ContinueStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#deferStatement.
	VisitDeferStatement(ctx *DeferStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#asmStatement.
	VisitAsmStatement(ctx *AsmStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#asmBlock.
	VisitAsmBlock(ctx *AsmBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#pattern.
	VisitPattern(ctx *PatternContext) interface{}

	// Visit a parse tree produced by MinZParser#literalPattern.
	VisitLiteralPattern(ctx *LiteralPatternContext) interface{}

	// Visit a parse tree produced by MinZParser#identifierPattern.
	VisitIdentifierPattern(ctx *IdentifierPatternContext) interface{}

	// Visit a parse tree produced by MinZParser#wildcardPattern.
	VisitWildcardPattern(ctx *WildcardPatternContext) interface{}

	// Visit a parse tree produced by MinZParser#tuplePattern.
	VisitTuplePattern(ctx *TuplePatternContext) interface{}

	// Visit a parse tree produced by MinZParser#structPattern.
	VisitStructPattern(ctx *StructPatternContext) interface{}

	// Visit a parse tree produced by MinZParser#fieldPattern.
	VisitFieldPattern(ctx *FieldPatternContext) interface{}

	// Visit a parse tree produced by MinZParser#expression.
	VisitExpression(ctx *ExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#lambdaExpression.
	VisitLambdaExpression(ctx *LambdaExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#lambdaParams.
	VisitLambdaParams(ctx *LambdaParamsContext) interface{}

	// Visit a parse tree produced by MinZParser#lambdaParam.
	VisitLambdaParam(ctx *LambdaParamContext) interface{}

	// Visit a parse tree produced by MinZParser#conditionalExpression.
	VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#whenExpression.
	VisitWhenExpression(ctx *WhenExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#whenArm.
	VisitWhenArm(ctx *WhenArmContext) interface{}

	// Visit a parse tree produced by MinZParser#logicalOrExpression.
	VisitLogicalOrExpression(ctx *LogicalOrExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#logicalAndExpression.
	VisitLogicalAndExpression(ctx *LogicalAndExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#equalityExpression.
	VisitEqualityExpression(ctx *EqualityExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#relationalExpression.
	VisitRelationalExpression(ctx *RelationalExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#additiveExpression.
	VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#multiplicativeExpression.
	VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#castExpression.
	VisitCastExpression(ctx *CastExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#unaryExpression.
	VisitUnaryExpression(ctx *UnaryExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#postfixExpression.
	VisitPostfixExpression(ctx *PostfixExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#postfixOperator.
	VisitPostfixOperator(ctx *PostfixOperatorContext) interface{}

	// Visit a parse tree produced by MinZParser#argumentList.
	VisitArgumentList(ctx *ArgumentListContext) interface{}

	// Visit a parse tree produced by MinZParser#primaryExpression.
	VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#qualifiedIdentifier.
	VisitQualifiedIdentifier(ctx *QualifiedIdentifierContext) interface{}

	// Visit a parse tree produced by MinZParser#literal.
	VisitLiteral(ctx *LiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#numberLiteral.
	VisitNumberLiteral(ctx *NumberLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#stringLiteral.
	VisitStringLiteral(ctx *StringLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#charLiteral.
	VisitCharLiteral(ctx *CharLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#booleanLiteral.
	VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#arrayLiteral.
	VisitArrayLiteral(ctx *ArrayLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#structLiteral.
	VisitStructLiteral(ctx *StructLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#fieldInit.
	VisitFieldInit(ctx *FieldInitContext) interface{}

	// Visit a parse tree produced by MinZParser#metafunction.
	VisitMetafunction(ctx *MetafunctionContext) interface{}

	// Visit a parse tree produced by MinZParser#logLevel.
	VisitLogLevel(ctx *LogLevelContext) interface{}

	// Visit a parse tree produced by MinZParser#luaBlock.
	VisitLuaBlock(ctx *LuaBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#inlineAssembly.
	VisitInlineAssembly(ctx *InlineAssemblyContext) interface{}

	// Visit a parse tree produced by MinZParser#asmOperand.
	VisitAsmOperand(ctx *AsmOperandContext) interface{}

	// Visit a parse tree produced by MinZParser#asmConstraint.
	VisitAsmConstraint(ctx *AsmConstraintContext) interface{}

	// Visit a parse tree produced by MinZParser#type.
	VisitType(ctx *TypeContext) interface{}

	// Visit a parse tree produced by MinZParser#primitiveType.
	VisitPrimitiveType(ctx *PrimitiveTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#arrayType.
	VisitArrayType(ctx *ArrayTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#pointerType.
	VisitPointerType(ctx *PointerTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#functionType.
	VisitFunctionType(ctx *FunctionTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#typeList.
	VisitTypeList(ctx *TypeListContext) interface{}

	// Visit a parse tree produced by MinZParser#structType.
	VisitStructType(ctx *StructTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#enumType.
	VisitEnumType(ctx *EnumTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#bitStructType.
	VisitBitStructType(ctx *BitStructTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#bitFieldList.
	VisitBitFieldList(ctx *BitFieldListContext) interface{}

	// Visit a parse tree produced by MinZParser#bitField.
	VisitBitField(ctx *BitFieldContext) interface{}

	// Visit a parse tree produced by MinZParser#typeIdentifier.
	VisitTypeIdentifier(ctx *TypeIdentifierContext) interface{}

	// Visit a parse tree produced by MinZParser#errorType.
	VisitErrorType(ctx *ErrorTypeContext) interface{}
}
