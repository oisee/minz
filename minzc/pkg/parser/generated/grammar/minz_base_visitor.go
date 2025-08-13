// Code generated from grammar/MinZ.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // MinZ

import "github.com/antlr4-go/antlr/v4"

type BaseMinZVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseMinZVisitor) VisitProgram(ctx *ProgramContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitImportDecl(ctx *ImportDeclContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitImportPath(ctx *ImportPathContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFunctionDecl(ctx *FunctionDeclContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmFunction(ctx *AsmFunctionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirFunction(ctx *MirFunctionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitGenericParams(ctx *GenericParamsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitReturnType(ctx *ReturnTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitErrorType(ctx *ErrorTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFunctionBody(ctx *FunctionBodyContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmBody(ctx *AsmBodyContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmContent(ctx *AsmContentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirBody(ctx *MirBodyContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirContent(ctx *MirContentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStructDecl(ctx *StructDeclContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStructField(ctx *StructFieldContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitInterfaceDecl(ctx *InterfaceDeclContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMethodSignature(ctx *MethodSignatureContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCastInterfaceBlock(ctx *CastInterfaceBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCastRule(ctx *CastRuleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEnumDecl(ctx *EnumDeclContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEnumVariant(ctx *EnumVariantContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitStructDecl(ctx *BitStructDeclContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitField(ctx *BitFieldContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitConstDecl(ctx *ConstDeclContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitGlobalDecl(ctx *GlobalDeclContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTypeAlias(ctx *TypeAliasContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitImplBlock(ctx *ImplBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMetafunction(ctx *MetafunctionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAttributedDeclaration(ctx *AttributedDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAttribute(ctx *AttributeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLuaBlock(ctx *LuaBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLuaCodeBlock(ctx *LuaCodeBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirBlockDeclaration(ctx *MirBlockDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirBlockContent(ctx *MirBlockContentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMinzMetafunctionDeclaration(ctx *MinzMetafunctionDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCompileTimeIfDeclaration(ctx *CompileTimeIfDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitDefineTemplate(ctx *DefineTemplateContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTemplateBody(ctx *TemplateBodyContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitIdentifierList(ctx *IdentifierListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMetaExecutionBlock(ctx *MetaExecutionBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLuaExecutionBlock(ctx *LuaExecutionBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMinzExecutionBlock(ctx *MinzExecutionBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirExecutionBlock(ctx *MirExecutionBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitRawBlockContent(ctx *RawBlockContentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitType(ctx *TypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPrimitiveType(ctx *PrimitiveTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitNamedType(ctx *NamedTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitArrayType(ctx *ArrayTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitArraySize(ctx *ArraySizeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPointerType(ctx *PointerTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFunctionType(ctx *FunctionTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitStructType(ctx *BitStructTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitErrorableType(ctx *ErrorableTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMutableType(ctx *MutableTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitIteratorType(ctx *IteratorTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPrimaryType(ctx *PrimaryTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStatement(ctx *StatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLetStatement(ctx *LetStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitIfStatement(ctx *IfStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitWhileStatement(ctx *WhileStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitForStatement(ctx *ForStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLoopStatement(ctx *LoopStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMatchStatement(ctx *MatchStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMatchArm(ctx *MatchArmContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCaseStatement(ctx *CaseStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCaseArm(ctx *CaseArmContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPattern(ctx *PatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEnumPattern(ctx *EnumPatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitReturnStatement(ctx *ReturnStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBreakStatement(ctx *BreakStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitContinueStatement(ctx *ContinueStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitDeferStatement(ctx *DeferStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAssignmentStatement(ctx *AssignmentStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAssignmentTarget(ctx *AssignmentTargetContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAssignmentOp(ctx *AssignmentOpContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitExpressionStatement(ctx *ExpressionStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBlock(ctx *BlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmBlock(ctx *AsmBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCompileTimeAsm(ctx *CompileTimeAsmContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirBlock(ctx *MirBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMinzBlock(ctx *MinzBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMinzContent(ctx *MinzContentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTargetBlock(ctx *TargetBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitShift(ctx *ShiftContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCast(ctx *CastContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCall(ctx *CallContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitIfExpr(ctx *IfExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitIndexAccess(ctx *IndexAccessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitRelational(ctx *RelationalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitErrorCheck(ctx *ErrorCheckContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMetafunctionCall(ctx *MetafunctionCallContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitRange(ctx *RangeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitUnary(ctx *UnaryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLogicalOr(ctx *LogicalOrContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMultiplicative(ctx *MultiplicativeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAdditive(ctx *AdditiveContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMemberAccess(ctx *MemberAccessContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitErrorDefault(ctx *ErrorDefaultContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitwiseXor(ctx *BitwiseXorContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitwiseOr(ctx *BitwiseOrContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPrimary(ctx *PrimaryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLogicalAnd(ctx *LogicalAndContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitWhenExpr(ctx *WhenExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitwiseAnd(ctx *BitwiseAndContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEquality(ctx *EqualityContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLambda(ctx *LambdaContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTernaryExpr(ctx *TernaryExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitArrayInitializer(ctx *ArrayInitializerContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStructLiteral(ctx *StructLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFieldInitializer(ctx *FieldInitializerContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTupleLiteral(ctx *TupleLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitInlineAssembly(ctx *InlineAssemblyContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmOutputList(ctx *AsmOutputListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmInputList(ctx *AsmInputListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmClobberList(ctx *AsmClobberListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmOutput(ctx *AsmOutputContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmInput(ctx *AsmInputContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitSizeofExpression(ctx *SizeofExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAlignofExpression(ctx *AlignofExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitErrorLiteral(ctx *ErrorLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLambdaExpression(ctx *LambdaExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLambdaParams(ctx *LambdaParamsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLambdaParam(ctx *LambdaParamContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitIfExpression(ctx *IfExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTernaryExpression(ctx *TernaryExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitWhenExpression(ctx *WhenExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitWhenArm(ctx *WhenArmContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMetafunctionExpr(ctx *MetafunctionExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitArgumentList(ctx *ArgumentListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitExpressionList(ctx *ExpressionListContext) interface{} {
	return v.VisitChildren(ctx)
}
