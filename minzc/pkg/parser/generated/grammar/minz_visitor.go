// Code generated from grammar/MinZ.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // MinZ

import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by MinZParser.
type MinZVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by MinZParser#program.
	VisitProgram(ctx *ProgramContext) interface{}

	// Visit a parse tree produced by MinZParser#importDecl.
	VisitImportDecl(ctx *ImportDeclContext) interface{}

	// Visit a parse tree produced by MinZParser#importPath.
	VisitImportPath(ctx *ImportPathContext) interface{}

	// Visit a parse tree produced by MinZParser#declaration.
	VisitDeclaration(ctx *DeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#functionDecl.
	VisitFunctionDecl(ctx *FunctionDeclContext) interface{}

	// Visit a parse tree produced by MinZParser#asmFunction.
	VisitAsmFunction(ctx *AsmFunctionContext) interface{}

	// Visit a parse tree produced by MinZParser#mirFunction.
	VisitMirFunction(ctx *MirFunctionContext) interface{}

	// Visit a parse tree produced by MinZParser#genericParams.
	VisitGenericParams(ctx *GenericParamsContext) interface{}

	// Visit a parse tree produced by MinZParser#returnType.
	VisitReturnType(ctx *ReturnTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#errorType.
	VisitErrorType(ctx *ErrorTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#functionBody.
	VisitFunctionBody(ctx *FunctionBodyContext) interface{}

	// Visit a parse tree produced by MinZParser#asmBody.
	VisitAsmBody(ctx *AsmBodyContext) interface{}

	// Visit a parse tree produced by MinZParser#asmContent.
	VisitAsmContent(ctx *AsmContentContext) interface{}

	// Visit a parse tree produced by MinZParser#mirBody.
	VisitMirBody(ctx *MirBodyContext) interface{}

	// Visit a parse tree produced by MinZParser#mirContent.
	VisitMirContent(ctx *MirContentContext) interface{}

	// Visit a parse tree produced by MinZParser#parameterList.
	VisitParameterList(ctx *ParameterListContext) interface{}

	// Visit a parse tree produced by MinZParser#parameter.
	VisitParameter(ctx *ParameterContext) interface{}

	// Visit a parse tree produced by MinZParser#structDecl.
	VisitStructDecl(ctx *StructDeclContext) interface{}

	// Visit a parse tree produced by MinZParser#structField.
	VisitStructField(ctx *StructFieldContext) interface{}

	// Visit a parse tree produced by MinZParser#interfaceDecl.
	VisitInterfaceDecl(ctx *InterfaceDeclContext) interface{}

	// Visit a parse tree produced by MinZParser#methodSignature.
	VisitMethodSignature(ctx *MethodSignatureContext) interface{}

	// Visit a parse tree produced by MinZParser#castInterfaceBlock.
	VisitCastInterfaceBlock(ctx *CastInterfaceBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#castRule.
	VisitCastRule(ctx *CastRuleContext) interface{}

	// Visit a parse tree produced by MinZParser#enumDecl.
	VisitEnumDecl(ctx *EnumDeclContext) interface{}

	// Visit a parse tree produced by MinZParser#enumVariant.
	VisitEnumVariant(ctx *EnumVariantContext) interface{}

	// Visit a parse tree produced by MinZParser#bitStructDecl.
	VisitBitStructDecl(ctx *BitStructDeclContext) interface{}

	// Visit a parse tree produced by MinZParser#bitField.
	VisitBitField(ctx *BitFieldContext) interface{}

	// Visit a parse tree produced by MinZParser#constDecl.
	VisitConstDecl(ctx *ConstDeclContext) interface{}

	// Visit a parse tree produced by MinZParser#globalDecl.
	VisitGlobalDecl(ctx *GlobalDeclContext) interface{}

	// Visit a parse tree produced by MinZParser#typeAlias.
	VisitTypeAlias(ctx *TypeAliasContext) interface{}

	// Visit a parse tree produced by MinZParser#implBlock.
	VisitImplBlock(ctx *ImplBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#metafunction.
	VisitMetafunction(ctx *MetafunctionContext) interface{}

	// Visit a parse tree produced by MinZParser#attributedDeclaration.
	VisitAttributedDeclaration(ctx *AttributedDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#attribute.
	VisitAttribute(ctx *AttributeContext) interface{}

	// Visit a parse tree produced by MinZParser#luaBlock.
	VisitLuaBlock(ctx *LuaBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#luaCodeBlock.
	VisitLuaCodeBlock(ctx *LuaCodeBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#mirBlockDeclaration.
	VisitMirBlockDeclaration(ctx *MirBlockDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#mirBlockContent.
	VisitMirBlockContent(ctx *MirBlockContentContext) interface{}

	// Visit a parse tree produced by MinZParser#minzMetafunctionDeclaration.
	VisitMinzMetafunctionDeclaration(ctx *MinzMetafunctionDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#compileTimeIfDeclaration.
	VisitCompileTimeIfDeclaration(ctx *CompileTimeIfDeclarationContext) interface{}

	// Visit a parse tree produced by MinZParser#defineTemplate.
	VisitDefineTemplate(ctx *DefineTemplateContext) interface{}

	// Visit a parse tree produced by MinZParser#templateBody.
	VisitTemplateBody(ctx *TemplateBodyContext) interface{}

	// Visit a parse tree produced by MinZParser#identifierList.
	VisitIdentifierList(ctx *IdentifierListContext) interface{}

	// Visit a parse tree produced by MinZParser#metaExecutionBlock.
	VisitMetaExecutionBlock(ctx *MetaExecutionBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#luaExecutionBlock.
	VisitLuaExecutionBlock(ctx *LuaExecutionBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#minzExecutionBlock.
	VisitMinzExecutionBlock(ctx *MinzExecutionBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#mirExecutionBlock.
	VisitMirExecutionBlock(ctx *MirExecutionBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#rawBlockContent.
	VisitRawBlockContent(ctx *RawBlockContentContext) interface{}

	// Visit a parse tree produced by MinZParser#type.
	VisitType(ctx *TypeContext) interface{}

	// Visit a parse tree produced by MinZParser#primitiveType.
	VisitPrimitiveType(ctx *PrimitiveTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#namedType.
	VisitNamedType(ctx *NamedTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#arrayType.
	VisitArrayType(ctx *ArrayTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#arraySize.
	VisitArraySize(ctx *ArraySizeContext) interface{}

	// Visit a parse tree produced by MinZParser#pointerType.
	VisitPointerType(ctx *PointerTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#functionType.
	VisitFunctionType(ctx *FunctionTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#bitStructType.
	VisitBitStructType(ctx *BitStructTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#errorableType.
	VisitErrorableType(ctx *ErrorableTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#mutableType.
	VisitMutableType(ctx *MutableTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#iteratorType.
	VisitIteratorType(ctx *IteratorTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#primaryType.
	VisitPrimaryType(ctx *PrimaryTypeContext) interface{}

	// Visit a parse tree produced by MinZParser#statement.
	VisitStatement(ctx *StatementContext) interface{}

	// Visit a parse tree produced by MinZParser#letStatement.
	VisitLetStatement(ctx *LetStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#ifStatement.
	VisitIfStatement(ctx *IfStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#whileStatement.
	VisitWhileStatement(ctx *WhileStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#forStatement.
	VisitForStatement(ctx *ForStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#loopStatement.
	VisitLoopStatement(ctx *LoopStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#matchStatement.
	VisitMatchStatement(ctx *MatchStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#matchArm.
	VisitMatchArm(ctx *MatchArmContext) interface{}

	// Visit a parse tree produced by MinZParser#caseStatement.
	VisitCaseStatement(ctx *CaseStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#caseArm.
	VisitCaseArm(ctx *CaseArmContext) interface{}

	// Visit a parse tree produced by MinZParser#pattern.
	VisitPattern(ctx *PatternContext) interface{}

	// Visit a parse tree produced by MinZParser#enumPattern.
	VisitEnumPattern(ctx *EnumPatternContext) interface{}

	// Visit a parse tree produced by MinZParser#returnStatement.
	VisitReturnStatement(ctx *ReturnStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#breakStatement.
	VisitBreakStatement(ctx *BreakStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#continueStatement.
	VisitContinueStatement(ctx *ContinueStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#deferStatement.
	VisitDeferStatement(ctx *DeferStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#assignmentStatement.
	VisitAssignmentStatement(ctx *AssignmentStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#assignmentTarget.
	VisitAssignmentTarget(ctx *AssignmentTargetContext) interface{}

	// Visit a parse tree produced by MinZParser#assignmentOp.
	VisitAssignmentOp(ctx *AssignmentOpContext) interface{}

	// Visit a parse tree produced by MinZParser#expressionStatement.
	VisitExpressionStatement(ctx *ExpressionStatementContext) interface{}

	// Visit a parse tree produced by MinZParser#block.
	VisitBlock(ctx *BlockContext) interface{}

	// Visit a parse tree produced by MinZParser#asmBlock.
	VisitAsmBlock(ctx *AsmBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#compileTimeAsm.
	VisitCompileTimeAsm(ctx *CompileTimeAsmContext) interface{}

	// Visit a parse tree produced by MinZParser#mirBlock.
	VisitMirBlock(ctx *MirBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#minzBlock.
	VisitMinzBlock(ctx *MinzBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#minzContent.
	VisitMinzContent(ctx *MinzContentContext) interface{}

	// Visit a parse tree produced by MinZParser#targetBlock.
	VisitTargetBlock(ctx *TargetBlockContext) interface{}

	// Visit a parse tree produced by MinZParser#Shift.
	VisitShift(ctx *ShiftContext) interface{}

	// Visit a parse tree produced by MinZParser#Cast.
	VisitCast(ctx *CastContext) interface{}

	// Visit a parse tree produced by MinZParser#Call.
	VisitCall(ctx *CallContext) interface{}

	// Visit a parse tree produced by MinZParser#IfExpr.
	VisitIfExpr(ctx *IfExprContext) interface{}

	// Visit a parse tree produced by MinZParser#IndexAccess.
	VisitIndexAccess(ctx *IndexAccessContext) interface{}

	// Visit a parse tree produced by MinZParser#Relational.
	VisitRelational(ctx *RelationalContext) interface{}

	// Visit a parse tree produced by MinZParser#ErrorCheck.
	VisitErrorCheck(ctx *ErrorCheckContext) interface{}

	// Visit a parse tree produced by MinZParser#MetafunctionCall.
	VisitMetafunctionCall(ctx *MetafunctionCallContext) interface{}

	// Visit a parse tree produced by MinZParser#Range.
	VisitRange(ctx *RangeContext) interface{}

	// Visit a parse tree produced by MinZParser#Unary.
	VisitUnary(ctx *UnaryContext) interface{}

	// Visit a parse tree produced by MinZParser#LogicalOr.
	VisitLogicalOr(ctx *LogicalOrContext) interface{}

	// Visit a parse tree produced by MinZParser#Multiplicative.
	VisitMultiplicative(ctx *MultiplicativeContext) interface{}

	// Visit a parse tree produced by MinZParser#Additive.
	VisitAdditive(ctx *AdditiveContext) interface{}

	// Visit a parse tree produced by MinZParser#MemberAccess.
	VisitMemberAccess(ctx *MemberAccessContext) interface{}

	// Visit a parse tree produced by MinZParser#ErrorDefault.
	VisitErrorDefault(ctx *ErrorDefaultContext) interface{}

	// Visit a parse tree produced by MinZParser#BitwiseXor.
	VisitBitwiseXor(ctx *BitwiseXorContext) interface{}

	// Visit a parse tree produced by MinZParser#BitwiseOr.
	VisitBitwiseOr(ctx *BitwiseOrContext) interface{}

	// Visit a parse tree produced by MinZParser#Primary.
	VisitPrimary(ctx *PrimaryContext) interface{}

	// Visit a parse tree produced by MinZParser#LogicalAnd.
	VisitLogicalAnd(ctx *LogicalAndContext) interface{}

	// Visit a parse tree produced by MinZParser#WhenExpr.
	VisitWhenExpr(ctx *WhenExprContext) interface{}

	// Visit a parse tree produced by MinZParser#BitwiseAnd.
	VisitBitwiseAnd(ctx *BitwiseAndContext) interface{}

	// Visit a parse tree produced by MinZParser#Equality.
	VisitEquality(ctx *EqualityContext) interface{}

	// Visit a parse tree produced by MinZParser#Lambda.
	VisitLambda(ctx *LambdaContext) interface{}

	// Visit a parse tree produced by MinZParser#TernaryExpr.
	VisitTernaryExpr(ctx *TernaryExprContext) interface{}

	// Visit a parse tree produced by MinZParser#primaryExpression.
	VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#literal.
	VisitLiteral(ctx *LiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#arrayLiteral.
	VisitArrayLiteral(ctx *ArrayLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#arrayInitializer.
	VisitArrayInitializer(ctx *ArrayInitializerContext) interface{}

	// Visit a parse tree produced by MinZParser#structLiteral.
	VisitStructLiteral(ctx *StructLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#fieldInitializer.
	VisitFieldInitializer(ctx *FieldInitializerContext) interface{}

	// Visit a parse tree produced by MinZParser#tupleLiteral.
	VisitTupleLiteral(ctx *TupleLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#inlineAssembly.
	VisitInlineAssembly(ctx *InlineAssemblyContext) interface{}

	// Visit a parse tree produced by MinZParser#asmOutputList.
	VisitAsmOutputList(ctx *AsmOutputListContext) interface{}

	// Visit a parse tree produced by MinZParser#asmInputList.
	VisitAsmInputList(ctx *AsmInputListContext) interface{}

	// Visit a parse tree produced by MinZParser#asmClobberList.
	VisitAsmClobberList(ctx *AsmClobberListContext) interface{}

	// Visit a parse tree produced by MinZParser#asmOutput.
	VisitAsmOutput(ctx *AsmOutputContext) interface{}

	// Visit a parse tree produced by MinZParser#asmInput.
	VisitAsmInput(ctx *AsmInputContext) interface{}

	// Visit a parse tree produced by MinZParser#sizeofExpression.
	VisitSizeofExpression(ctx *SizeofExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#alignofExpression.
	VisitAlignofExpression(ctx *AlignofExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#errorLiteral.
	VisitErrorLiteral(ctx *ErrorLiteralContext) interface{}

	// Visit a parse tree produced by MinZParser#lambdaExpression.
	VisitLambdaExpression(ctx *LambdaExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#lambdaParams.
	VisitLambdaParams(ctx *LambdaParamsContext) interface{}

	// Visit a parse tree produced by MinZParser#lambdaParam.
	VisitLambdaParam(ctx *LambdaParamContext) interface{}

	// Visit a parse tree produced by MinZParser#ifExpression.
	VisitIfExpression(ctx *IfExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#ternaryExpression.
	VisitTernaryExpression(ctx *TernaryExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#whenExpression.
	VisitWhenExpression(ctx *WhenExpressionContext) interface{}

	// Visit a parse tree produced by MinZParser#whenArm.
	VisitWhenArm(ctx *WhenArmContext) interface{}

	// Visit a parse tree produced by MinZParser#metafunctionExpr.
	VisitMetafunctionExpr(ctx *MetafunctionExprContext) interface{}

	// Visit a parse tree produced by MinZParser#argumentList.
	VisitArgumentList(ctx *ArgumentListContext) interface{}

	// Visit a parse tree produced by MinZParser#expressionList.
	VisitExpressionList(ctx *ExpressionListContext) interface{}
}
