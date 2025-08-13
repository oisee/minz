// Code generated from grammar/MinZ.g4 by ANTLR 4.13.2. DO NOT EDIT.

package minzparser // MinZ
import "github.com/antlr4-go/antlr/v4"

type BaseMinZVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseMinZVisitor) VisitSourceFile(ctx *SourceFileContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitImportStatement(ctx *ImportStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitImportPath(ctx *ImportPathContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFunctionPrefix(ctx *FunctionPrefixContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitVisibility(ctx *VisibilityContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitGenericParams(ctx *GenericParamsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitReturnType(ctx *ReturnTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitErrorReturnType(ctx *ErrorReturnTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStructDeclaration(ctx *StructDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFieldList(ctx *FieldListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitField(ctx *FieldContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEnumDeclaration(ctx *EnumDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEnumMemberList(ctx *EnumMemberListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEnumMember(ctx *EnumMemberContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTypeAliasDeclaration(ctx *TypeAliasDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitInterfaceMethodList(ctx *InterfaceMethodListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitInterfaceMethod(ctx *InterfaceMethodContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitImplBlock(ctx *ImplBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitConstDeclaration(ctx *ConstDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitGlobalVarDeclaration(ctx *GlobalVarDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCompileTimeDeclaration(ctx *CompileTimeDeclarationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCompileTimeIf(ctx *CompileTimeIfContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCompileTimeMinz(ctx *CompileTimeMinzContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCompileTimeMir(ctx *CompileTimeMirContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirBlock(ctx *MirBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirStatement(ctx *MirStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirInstruction(ctx *MirInstructionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirOperand(ctx *MirOperandContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirRegister(ctx *MirRegisterContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirImmediate(ctx *MirImmediateContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirMemory(ctx *MirMemoryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMirLabel(ctx *MirLabelContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTargetBlock(ctx *TargetBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStatement(ctx *StatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLetStatement(ctx *LetStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitVarStatement(ctx *VarStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAssignmentStatement(ctx *AssignmentStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitExpressionStatement(ctx *ExpressionStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitReturnStatement(ctx *ReturnStatementContext) interface{} {
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

func (v *BaseMinZVisitor) VisitCaseStatement(ctx *CaseStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCaseArm(ctx *CaseArmContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBlockStatement(ctx *BlockStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBlock(ctx *BlockContext) interface{} {
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

func (v *BaseMinZVisitor) VisitAsmStatement(ctx *AsmStatementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmBlock(ctx *AsmBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPattern(ctx *PatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLiteralPattern(ctx *LiteralPatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitIdentifierPattern(ctx *IdentifierPatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitWildcardPattern(ctx *WildcardPatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTuplePattern(ctx *TuplePatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStructPattern(ctx *StructPatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFieldPattern(ctx *FieldPatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitExpression(ctx *ExpressionContext) interface{} {
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

func (v *BaseMinZVisitor) VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitWhenExpression(ctx *WhenExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitWhenArm(ctx *WhenArmContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLogicalOrExpression(ctx *LogicalOrExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLogicalAndExpression(ctx *LogicalAndExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEqualityExpression(ctx *EqualityExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitRelationalExpression(ctx *RelationalExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCastExpression(ctx *CastExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitUnaryExpression(ctx *UnaryExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPostfixExpression(ctx *PostfixExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPostfixOperator(ctx *PostfixOperatorContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitArgumentList(ctx *ArgumentListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitNumberLiteral(ctx *NumberLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStringLiteral(ctx *StringLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitCharLiteral(ctx *CharLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStructLiteral(ctx *StructLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFieldInit(ctx *FieldInitContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitMetafunction(ctx *MetafunctionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitLuaBlock(ctx *LuaBlockContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitInlineAssembly(ctx *InlineAssemblyContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmOperand(ctx *AsmOperandContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitAsmConstraint(ctx *AsmConstraintContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitType(ctx *TypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPrimitiveType(ctx *PrimitiveTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitArrayType(ctx *ArrayTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitPointerType(ctx *PointerTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitFunctionType(ctx *FunctionTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTypeList(ctx *TypeListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitStructType(ctx *StructTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitEnumType(ctx *EnumTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitStructType(ctx *BitStructTypeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitFieldList(ctx *BitFieldListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitBitField(ctx *BitFieldContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitTypeIdentifier(ctx *TypeIdentifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseMinZVisitor) VisitErrorType(ctx *ErrorTypeContext) interface{} {
	return v.VisitChildren(ctx)
}
