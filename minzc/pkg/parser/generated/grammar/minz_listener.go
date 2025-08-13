// Code generated from grammar/MinZ.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // MinZ

import "github.com/antlr4-go/antlr/v4"

// MinZListener is a complete listener for a parse tree produced by MinZParser.
type MinZListener interface {
	antlr.ParseTreeListener

	// EnterProgram is called when entering the program production.
	EnterProgram(c *ProgramContext)

	// EnterImportDecl is called when entering the importDecl production.
	EnterImportDecl(c *ImportDeclContext)

	// EnterImportPath is called when entering the importPath production.
	EnterImportPath(c *ImportPathContext)

	// EnterDeclaration is called when entering the declaration production.
	EnterDeclaration(c *DeclarationContext)

	// EnterFunctionDecl is called when entering the functionDecl production.
	EnterFunctionDecl(c *FunctionDeclContext)

	// EnterAsmFunction is called when entering the asmFunction production.
	EnterAsmFunction(c *AsmFunctionContext)

	// EnterMirFunction is called when entering the mirFunction production.
	EnterMirFunction(c *MirFunctionContext)

	// EnterGenericParams is called when entering the genericParams production.
	EnterGenericParams(c *GenericParamsContext)

	// EnterReturnType is called when entering the returnType production.
	EnterReturnType(c *ReturnTypeContext)

	// EnterErrorType is called when entering the errorType production.
	EnterErrorType(c *ErrorTypeContext)

	// EnterFunctionBody is called when entering the functionBody production.
	EnterFunctionBody(c *FunctionBodyContext)

	// EnterAsmBody is called when entering the asmBody production.
	EnterAsmBody(c *AsmBodyContext)

	// EnterAsmContent is called when entering the asmContent production.
	EnterAsmContent(c *AsmContentContext)

	// EnterMirBody is called when entering the mirBody production.
	EnterMirBody(c *MirBodyContext)

	// EnterMirContent is called when entering the mirContent production.
	EnterMirContent(c *MirContentContext)

	// EnterParameterList is called when entering the parameterList production.
	EnterParameterList(c *ParameterListContext)

	// EnterParameter is called when entering the parameter production.
	EnterParameter(c *ParameterContext)

	// EnterStructDecl is called when entering the structDecl production.
	EnterStructDecl(c *StructDeclContext)

	// EnterStructField is called when entering the structField production.
	EnterStructField(c *StructFieldContext)

	// EnterInterfaceDecl is called when entering the interfaceDecl production.
	EnterInterfaceDecl(c *InterfaceDeclContext)

	// EnterMethodSignature is called when entering the methodSignature production.
	EnterMethodSignature(c *MethodSignatureContext)

	// EnterCastInterfaceBlock is called when entering the castInterfaceBlock production.
	EnterCastInterfaceBlock(c *CastInterfaceBlockContext)

	// EnterCastRule is called when entering the castRule production.
	EnterCastRule(c *CastRuleContext)

	// EnterEnumDecl is called when entering the enumDecl production.
	EnterEnumDecl(c *EnumDeclContext)

	// EnterEnumVariant is called when entering the enumVariant production.
	EnterEnumVariant(c *EnumVariantContext)

	// EnterBitStructDecl is called when entering the bitStructDecl production.
	EnterBitStructDecl(c *BitStructDeclContext)

	// EnterBitField is called when entering the bitField production.
	EnterBitField(c *BitFieldContext)

	// EnterConstDecl is called when entering the constDecl production.
	EnterConstDecl(c *ConstDeclContext)

	// EnterGlobalDecl is called when entering the globalDecl production.
	EnterGlobalDecl(c *GlobalDeclContext)

	// EnterTypeAlias is called when entering the typeAlias production.
	EnterTypeAlias(c *TypeAliasContext)

	// EnterImplBlock is called when entering the implBlock production.
	EnterImplBlock(c *ImplBlockContext)

	// EnterMetafunction is called when entering the metafunction production.
	EnterMetafunction(c *MetafunctionContext)

	// EnterAttributedDeclaration is called when entering the attributedDeclaration production.
	EnterAttributedDeclaration(c *AttributedDeclarationContext)

	// EnterAttribute is called when entering the attribute production.
	EnterAttribute(c *AttributeContext)

	// EnterLuaBlock is called when entering the luaBlock production.
	EnterLuaBlock(c *LuaBlockContext)

	// EnterLuaCodeBlock is called when entering the luaCodeBlock production.
	EnterLuaCodeBlock(c *LuaCodeBlockContext)

	// EnterMirBlockDeclaration is called when entering the mirBlockDeclaration production.
	EnterMirBlockDeclaration(c *MirBlockDeclarationContext)

	// EnterMirBlockContent is called when entering the mirBlockContent production.
	EnterMirBlockContent(c *MirBlockContentContext)

	// EnterMinzMetafunctionDeclaration is called when entering the minzMetafunctionDeclaration production.
	EnterMinzMetafunctionDeclaration(c *MinzMetafunctionDeclarationContext)

	// EnterCompileTimeIfDeclaration is called when entering the compileTimeIfDeclaration production.
	EnterCompileTimeIfDeclaration(c *CompileTimeIfDeclarationContext)

	// EnterDefineTemplate is called when entering the defineTemplate production.
	EnterDefineTemplate(c *DefineTemplateContext)

	// EnterTemplateBody is called when entering the templateBody production.
	EnterTemplateBody(c *TemplateBodyContext)

	// EnterIdentifierList is called when entering the identifierList production.
	EnterIdentifierList(c *IdentifierListContext)

	// EnterMetaExecutionBlock is called when entering the metaExecutionBlock production.
	EnterMetaExecutionBlock(c *MetaExecutionBlockContext)

	// EnterLuaExecutionBlock is called when entering the luaExecutionBlock production.
	EnterLuaExecutionBlock(c *LuaExecutionBlockContext)

	// EnterMinzExecutionBlock is called when entering the minzExecutionBlock production.
	EnterMinzExecutionBlock(c *MinzExecutionBlockContext)

	// EnterMirExecutionBlock is called when entering the mirExecutionBlock production.
	EnterMirExecutionBlock(c *MirExecutionBlockContext)

	// EnterRawBlockContent is called when entering the rawBlockContent production.
	EnterRawBlockContent(c *RawBlockContentContext)

	// EnterType is called when entering the type production.
	EnterType(c *TypeContext)

	// EnterPrimitiveType is called when entering the primitiveType production.
	EnterPrimitiveType(c *PrimitiveTypeContext)

	// EnterNamedType is called when entering the namedType production.
	EnterNamedType(c *NamedTypeContext)

	// EnterArrayType is called when entering the arrayType production.
	EnterArrayType(c *ArrayTypeContext)

	// EnterArraySize is called when entering the arraySize production.
	EnterArraySize(c *ArraySizeContext)

	// EnterPointerType is called when entering the pointerType production.
	EnterPointerType(c *PointerTypeContext)

	// EnterFunctionType is called when entering the functionType production.
	EnterFunctionType(c *FunctionTypeContext)

	// EnterBitStructType is called when entering the bitStructType production.
	EnterBitStructType(c *BitStructTypeContext)

	// EnterErrorableType is called when entering the errorableType production.
	EnterErrorableType(c *ErrorableTypeContext)

	// EnterMutableType is called when entering the mutableType production.
	EnterMutableType(c *MutableTypeContext)

	// EnterIteratorType is called when entering the iteratorType production.
	EnterIteratorType(c *IteratorTypeContext)

	// EnterPrimaryType is called when entering the primaryType production.
	EnterPrimaryType(c *PrimaryTypeContext)

	// EnterStatement is called when entering the statement production.
	EnterStatement(c *StatementContext)

	// EnterLetStatement is called when entering the letStatement production.
	EnterLetStatement(c *LetStatementContext)

	// EnterIfStatement is called when entering the ifStatement production.
	EnterIfStatement(c *IfStatementContext)

	// EnterWhileStatement is called when entering the whileStatement production.
	EnterWhileStatement(c *WhileStatementContext)

	// EnterForStatement is called when entering the forStatement production.
	EnterForStatement(c *ForStatementContext)

	// EnterLoopStatement is called when entering the loopStatement production.
	EnterLoopStatement(c *LoopStatementContext)

	// EnterMatchStatement is called when entering the matchStatement production.
	EnterMatchStatement(c *MatchStatementContext)

	// EnterMatchArm is called when entering the matchArm production.
	EnterMatchArm(c *MatchArmContext)

	// EnterCaseStatement is called when entering the caseStatement production.
	EnterCaseStatement(c *CaseStatementContext)

	// EnterCaseArm is called when entering the caseArm production.
	EnterCaseArm(c *CaseArmContext)

	// EnterPattern is called when entering the pattern production.
	EnterPattern(c *PatternContext)

	// EnterEnumPattern is called when entering the enumPattern production.
	EnterEnumPattern(c *EnumPatternContext)

	// EnterReturnStatement is called when entering the returnStatement production.
	EnterReturnStatement(c *ReturnStatementContext)

	// EnterBreakStatement is called when entering the breakStatement production.
	EnterBreakStatement(c *BreakStatementContext)

	// EnterContinueStatement is called when entering the continueStatement production.
	EnterContinueStatement(c *ContinueStatementContext)

	// EnterDeferStatement is called when entering the deferStatement production.
	EnterDeferStatement(c *DeferStatementContext)

	// EnterAssignmentStatement is called when entering the assignmentStatement production.
	EnterAssignmentStatement(c *AssignmentStatementContext)

	// EnterAssignmentTarget is called when entering the assignmentTarget production.
	EnterAssignmentTarget(c *AssignmentTargetContext)

	// EnterAssignmentOp is called when entering the assignmentOp production.
	EnterAssignmentOp(c *AssignmentOpContext)

	// EnterExpressionStatement is called when entering the expressionStatement production.
	EnterExpressionStatement(c *ExpressionStatementContext)

	// EnterBlock is called when entering the block production.
	EnterBlock(c *BlockContext)

	// EnterAsmBlock is called when entering the asmBlock production.
	EnterAsmBlock(c *AsmBlockContext)

	// EnterCompileTimeAsm is called when entering the compileTimeAsm production.
	EnterCompileTimeAsm(c *CompileTimeAsmContext)

	// EnterMirBlock is called when entering the mirBlock production.
	EnterMirBlock(c *MirBlockContext)

	// EnterMinzBlock is called when entering the minzBlock production.
	EnterMinzBlock(c *MinzBlockContext)

	// EnterMinzContent is called when entering the minzContent production.
	EnterMinzContent(c *MinzContentContext)

	// EnterTargetBlock is called when entering the targetBlock production.
	EnterTargetBlock(c *TargetBlockContext)

	// EnterShift is called when entering the Shift production.
	EnterShift(c *ShiftContext)

	// EnterCast is called when entering the Cast production.
	EnterCast(c *CastContext)

	// EnterCall is called when entering the Call production.
	EnterCall(c *CallContext)

	// EnterIfExpr is called when entering the IfExpr production.
	EnterIfExpr(c *IfExprContext)

	// EnterIndexAccess is called when entering the IndexAccess production.
	EnterIndexAccess(c *IndexAccessContext)

	// EnterRelational is called when entering the Relational production.
	EnterRelational(c *RelationalContext)

	// EnterErrorCheck is called when entering the ErrorCheck production.
	EnterErrorCheck(c *ErrorCheckContext)

	// EnterMetafunctionCall is called when entering the MetafunctionCall production.
	EnterMetafunctionCall(c *MetafunctionCallContext)

	// EnterRange is called when entering the Range production.
	EnterRange(c *RangeContext)

	// EnterUnary is called when entering the Unary production.
	EnterUnary(c *UnaryContext)

	// EnterLogicalOr is called when entering the LogicalOr production.
	EnterLogicalOr(c *LogicalOrContext)

	// EnterMultiplicative is called when entering the Multiplicative production.
	EnterMultiplicative(c *MultiplicativeContext)

	// EnterAdditive is called when entering the Additive production.
	EnterAdditive(c *AdditiveContext)

	// EnterMemberAccess is called when entering the MemberAccess production.
	EnterMemberAccess(c *MemberAccessContext)

	// EnterErrorDefault is called when entering the ErrorDefault production.
	EnterErrorDefault(c *ErrorDefaultContext)

	// EnterBitwiseXor is called when entering the BitwiseXor production.
	EnterBitwiseXor(c *BitwiseXorContext)

	// EnterBitwiseOr is called when entering the BitwiseOr production.
	EnterBitwiseOr(c *BitwiseOrContext)

	// EnterPrimary is called when entering the Primary production.
	EnterPrimary(c *PrimaryContext)

	// EnterLogicalAnd is called when entering the LogicalAnd production.
	EnterLogicalAnd(c *LogicalAndContext)

	// EnterWhenExpr is called when entering the WhenExpr production.
	EnterWhenExpr(c *WhenExprContext)

	// EnterBitwiseAnd is called when entering the BitwiseAnd production.
	EnterBitwiseAnd(c *BitwiseAndContext)

	// EnterEquality is called when entering the Equality production.
	EnterEquality(c *EqualityContext)

	// EnterLambda is called when entering the Lambda production.
	EnterLambda(c *LambdaContext)

	// EnterTernaryExpr is called when entering the TernaryExpr production.
	EnterTernaryExpr(c *TernaryExprContext)

	// EnterPrimaryExpression is called when entering the primaryExpression production.
	EnterPrimaryExpression(c *PrimaryExpressionContext)

	// EnterLiteral is called when entering the literal production.
	EnterLiteral(c *LiteralContext)

	// EnterArrayLiteral is called when entering the arrayLiteral production.
	EnterArrayLiteral(c *ArrayLiteralContext)

	// EnterArrayInitializer is called when entering the arrayInitializer production.
	EnterArrayInitializer(c *ArrayInitializerContext)

	// EnterStructLiteral is called when entering the structLiteral production.
	EnterStructLiteral(c *StructLiteralContext)

	// EnterFieldInitializer is called when entering the fieldInitializer production.
	EnterFieldInitializer(c *FieldInitializerContext)

	// EnterTupleLiteral is called when entering the tupleLiteral production.
	EnterTupleLiteral(c *TupleLiteralContext)

	// EnterInlineAssembly is called when entering the inlineAssembly production.
	EnterInlineAssembly(c *InlineAssemblyContext)

	// EnterAsmOutputList is called when entering the asmOutputList production.
	EnterAsmOutputList(c *AsmOutputListContext)

	// EnterAsmInputList is called when entering the asmInputList production.
	EnterAsmInputList(c *AsmInputListContext)

	// EnterAsmClobberList is called when entering the asmClobberList production.
	EnterAsmClobberList(c *AsmClobberListContext)

	// EnterAsmOutput is called when entering the asmOutput production.
	EnterAsmOutput(c *AsmOutputContext)

	// EnterAsmInput is called when entering the asmInput production.
	EnterAsmInput(c *AsmInputContext)

	// EnterSizeofExpression is called when entering the sizeofExpression production.
	EnterSizeofExpression(c *SizeofExpressionContext)

	// EnterAlignofExpression is called when entering the alignofExpression production.
	EnterAlignofExpression(c *AlignofExpressionContext)

	// EnterErrorLiteral is called when entering the errorLiteral production.
	EnterErrorLiteral(c *ErrorLiteralContext)

	// EnterLambdaExpression is called when entering the lambdaExpression production.
	EnterLambdaExpression(c *LambdaExpressionContext)

	// EnterLambdaParams is called when entering the lambdaParams production.
	EnterLambdaParams(c *LambdaParamsContext)

	// EnterLambdaParam is called when entering the lambdaParam production.
	EnterLambdaParam(c *LambdaParamContext)

	// EnterIfExpression is called when entering the ifExpression production.
	EnterIfExpression(c *IfExpressionContext)

	// EnterTernaryExpression is called when entering the ternaryExpression production.
	EnterTernaryExpression(c *TernaryExpressionContext)

	// EnterWhenExpression is called when entering the whenExpression production.
	EnterWhenExpression(c *WhenExpressionContext)

	// EnterWhenArm is called when entering the whenArm production.
	EnterWhenArm(c *WhenArmContext)

	// EnterMetafunctionExpr is called when entering the metafunctionExpr production.
	EnterMetafunctionExpr(c *MetafunctionExprContext)

	// EnterArgumentList is called when entering the argumentList production.
	EnterArgumentList(c *ArgumentListContext)

	// EnterExpressionList is called when entering the expressionList production.
	EnterExpressionList(c *ExpressionListContext)

	// ExitProgram is called when exiting the program production.
	ExitProgram(c *ProgramContext)

	// ExitImportDecl is called when exiting the importDecl production.
	ExitImportDecl(c *ImportDeclContext)

	// ExitImportPath is called when exiting the importPath production.
	ExitImportPath(c *ImportPathContext)

	// ExitDeclaration is called when exiting the declaration production.
	ExitDeclaration(c *DeclarationContext)

	// ExitFunctionDecl is called when exiting the functionDecl production.
	ExitFunctionDecl(c *FunctionDeclContext)

	// ExitAsmFunction is called when exiting the asmFunction production.
	ExitAsmFunction(c *AsmFunctionContext)

	// ExitMirFunction is called when exiting the mirFunction production.
	ExitMirFunction(c *MirFunctionContext)

	// ExitGenericParams is called when exiting the genericParams production.
	ExitGenericParams(c *GenericParamsContext)

	// ExitReturnType is called when exiting the returnType production.
	ExitReturnType(c *ReturnTypeContext)

	// ExitErrorType is called when exiting the errorType production.
	ExitErrorType(c *ErrorTypeContext)

	// ExitFunctionBody is called when exiting the functionBody production.
	ExitFunctionBody(c *FunctionBodyContext)

	// ExitAsmBody is called when exiting the asmBody production.
	ExitAsmBody(c *AsmBodyContext)

	// ExitAsmContent is called when exiting the asmContent production.
	ExitAsmContent(c *AsmContentContext)

	// ExitMirBody is called when exiting the mirBody production.
	ExitMirBody(c *MirBodyContext)

	// ExitMirContent is called when exiting the mirContent production.
	ExitMirContent(c *MirContentContext)

	// ExitParameterList is called when exiting the parameterList production.
	ExitParameterList(c *ParameterListContext)

	// ExitParameter is called when exiting the parameter production.
	ExitParameter(c *ParameterContext)

	// ExitStructDecl is called when exiting the structDecl production.
	ExitStructDecl(c *StructDeclContext)

	// ExitStructField is called when exiting the structField production.
	ExitStructField(c *StructFieldContext)

	// ExitInterfaceDecl is called when exiting the interfaceDecl production.
	ExitInterfaceDecl(c *InterfaceDeclContext)

	// ExitMethodSignature is called when exiting the methodSignature production.
	ExitMethodSignature(c *MethodSignatureContext)

	// ExitCastInterfaceBlock is called when exiting the castInterfaceBlock production.
	ExitCastInterfaceBlock(c *CastInterfaceBlockContext)

	// ExitCastRule is called when exiting the castRule production.
	ExitCastRule(c *CastRuleContext)

	// ExitEnumDecl is called when exiting the enumDecl production.
	ExitEnumDecl(c *EnumDeclContext)

	// ExitEnumVariant is called when exiting the enumVariant production.
	ExitEnumVariant(c *EnumVariantContext)

	// ExitBitStructDecl is called when exiting the bitStructDecl production.
	ExitBitStructDecl(c *BitStructDeclContext)

	// ExitBitField is called when exiting the bitField production.
	ExitBitField(c *BitFieldContext)

	// ExitConstDecl is called when exiting the constDecl production.
	ExitConstDecl(c *ConstDeclContext)

	// ExitGlobalDecl is called when exiting the globalDecl production.
	ExitGlobalDecl(c *GlobalDeclContext)

	// ExitTypeAlias is called when exiting the typeAlias production.
	ExitTypeAlias(c *TypeAliasContext)

	// ExitImplBlock is called when exiting the implBlock production.
	ExitImplBlock(c *ImplBlockContext)

	// ExitMetafunction is called when exiting the metafunction production.
	ExitMetafunction(c *MetafunctionContext)

	// ExitAttributedDeclaration is called when exiting the attributedDeclaration production.
	ExitAttributedDeclaration(c *AttributedDeclarationContext)

	// ExitAttribute is called when exiting the attribute production.
	ExitAttribute(c *AttributeContext)

	// ExitLuaBlock is called when exiting the luaBlock production.
	ExitLuaBlock(c *LuaBlockContext)

	// ExitLuaCodeBlock is called when exiting the luaCodeBlock production.
	ExitLuaCodeBlock(c *LuaCodeBlockContext)

	// ExitMirBlockDeclaration is called when exiting the mirBlockDeclaration production.
	ExitMirBlockDeclaration(c *MirBlockDeclarationContext)

	// ExitMirBlockContent is called when exiting the mirBlockContent production.
	ExitMirBlockContent(c *MirBlockContentContext)

	// ExitMinzMetafunctionDeclaration is called when exiting the minzMetafunctionDeclaration production.
	ExitMinzMetafunctionDeclaration(c *MinzMetafunctionDeclarationContext)

	// ExitCompileTimeIfDeclaration is called when exiting the compileTimeIfDeclaration production.
	ExitCompileTimeIfDeclaration(c *CompileTimeIfDeclarationContext)

	// ExitDefineTemplate is called when exiting the defineTemplate production.
	ExitDefineTemplate(c *DefineTemplateContext)

	// ExitTemplateBody is called when exiting the templateBody production.
	ExitTemplateBody(c *TemplateBodyContext)

	// ExitIdentifierList is called when exiting the identifierList production.
	ExitIdentifierList(c *IdentifierListContext)

	// ExitMetaExecutionBlock is called when exiting the metaExecutionBlock production.
	ExitMetaExecutionBlock(c *MetaExecutionBlockContext)

	// ExitLuaExecutionBlock is called when exiting the luaExecutionBlock production.
	ExitLuaExecutionBlock(c *LuaExecutionBlockContext)

	// ExitMinzExecutionBlock is called when exiting the minzExecutionBlock production.
	ExitMinzExecutionBlock(c *MinzExecutionBlockContext)

	// ExitMirExecutionBlock is called when exiting the mirExecutionBlock production.
	ExitMirExecutionBlock(c *MirExecutionBlockContext)

	// ExitRawBlockContent is called when exiting the rawBlockContent production.
	ExitRawBlockContent(c *RawBlockContentContext)

	// ExitType is called when exiting the type production.
	ExitType(c *TypeContext)

	// ExitPrimitiveType is called when exiting the primitiveType production.
	ExitPrimitiveType(c *PrimitiveTypeContext)

	// ExitNamedType is called when exiting the namedType production.
	ExitNamedType(c *NamedTypeContext)

	// ExitArrayType is called when exiting the arrayType production.
	ExitArrayType(c *ArrayTypeContext)

	// ExitArraySize is called when exiting the arraySize production.
	ExitArraySize(c *ArraySizeContext)

	// ExitPointerType is called when exiting the pointerType production.
	ExitPointerType(c *PointerTypeContext)

	// ExitFunctionType is called when exiting the functionType production.
	ExitFunctionType(c *FunctionTypeContext)

	// ExitBitStructType is called when exiting the bitStructType production.
	ExitBitStructType(c *BitStructTypeContext)

	// ExitErrorableType is called when exiting the errorableType production.
	ExitErrorableType(c *ErrorableTypeContext)

	// ExitMutableType is called when exiting the mutableType production.
	ExitMutableType(c *MutableTypeContext)

	// ExitIteratorType is called when exiting the iteratorType production.
	ExitIteratorType(c *IteratorTypeContext)

	// ExitPrimaryType is called when exiting the primaryType production.
	ExitPrimaryType(c *PrimaryTypeContext)

	// ExitStatement is called when exiting the statement production.
	ExitStatement(c *StatementContext)

	// ExitLetStatement is called when exiting the letStatement production.
	ExitLetStatement(c *LetStatementContext)

	// ExitIfStatement is called when exiting the ifStatement production.
	ExitIfStatement(c *IfStatementContext)

	// ExitWhileStatement is called when exiting the whileStatement production.
	ExitWhileStatement(c *WhileStatementContext)

	// ExitForStatement is called when exiting the forStatement production.
	ExitForStatement(c *ForStatementContext)

	// ExitLoopStatement is called when exiting the loopStatement production.
	ExitLoopStatement(c *LoopStatementContext)

	// ExitMatchStatement is called when exiting the matchStatement production.
	ExitMatchStatement(c *MatchStatementContext)

	// ExitMatchArm is called when exiting the matchArm production.
	ExitMatchArm(c *MatchArmContext)

	// ExitCaseStatement is called when exiting the caseStatement production.
	ExitCaseStatement(c *CaseStatementContext)

	// ExitCaseArm is called when exiting the caseArm production.
	ExitCaseArm(c *CaseArmContext)

	// ExitPattern is called when exiting the pattern production.
	ExitPattern(c *PatternContext)

	// ExitEnumPattern is called when exiting the enumPattern production.
	ExitEnumPattern(c *EnumPatternContext)

	// ExitReturnStatement is called when exiting the returnStatement production.
	ExitReturnStatement(c *ReturnStatementContext)

	// ExitBreakStatement is called when exiting the breakStatement production.
	ExitBreakStatement(c *BreakStatementContext)

	// ExitContinueStatement is called when exiting the continueStatement production.
	ExitContinueStatement(c *ContinueStatementContext)

	// ExitDeferStatement is called when exiting the deferStatement production.
	ExitDeferStatement(c *DeferStatementContext)

	// ExitAssignmentStatement is called when exiting the assignmentStatement production.
	ExitAssignmentStatement(c *AssignmentStatementContext)

	// ExitAssignmentTarget is called when exiting the assignmentTarget production.
	ExitAssignmentTarget(c *AssignmentTargetContext)

	// ExitAssignmentOp is called when exiting the assignmentOp production.
	ExitAssignmentOp(c *AssignmentOpContext)

	// ExitExpressionStatement is called when exiting the expressionStatement production.
	ExitExpressionStatement(c *ExpressionStatementContext)

	// ExitBlock is called when exiting the block production.
	ExitBlock(c *BlockContext)

	// ExitAsmBlock is called when exiting the asmBlock production.
	ExitAsmBlock(c *AsmBlockContext)

	// ExitCompileTimeAsm is called when exiting the compileTimeAsm production.
	ExitCompileTimeAsm(c *CompileTimeAsmContext)

	// ExitMirBlock is called when exiting the mirBlock production.
	ExitMirBlock(c *MirBlockContext)

	// ExitMinzBlock is called when exiting the minzBlock production.
	ExitMinzBlock(c *MinzBlockContext)

	// ExitMinzContent is called when exiting the minzContent production.
	ExitMinzContent(c *MinzContentContext)

	// ExitTargetBlock is called when exiting the targetBlock production.
	ExitTargetBlock(c *TargetBlockContext)

	// ExitShift is called when exiting the Shift production.
	ExitShift(c *ShiftContext)

	// ExitCast is called when exiting the Cast production.
	ExitCast(c *CastContext)

	// ExitCall is called when exiting the Call production.
	ExitCall(c *CallContext)

	// ExitIfExpr is called when exiting the IfExpr production.
	ExitIfExpr(c *IfExprContext)

	// ExitIndexAccess is called when exiting the IndexAccess production.
	ExitIndexAccess(c *IndexAccessContext)

	// ExitRelational is called when exiting the Relational production.
	ExitRelational(c *RelationalContext)

	// ExitErrorCheck is called when exiting the ErrorCheck production.
	ExitErrorCheck(c *ErrorCheckContext)

	// ExitMetafunctionCall is called when exiting the MetafunctionCall production.
	ExitMetafunctionCall(c *MetafunctionCallContext)

	// ExitRange is called when exiting the Range production.
	ExitRange(c *RangeContext)

	// ExitUnary is called when exiting the Unary production.
	ExitUnary(c *UnaryContext)

	// ExitLogicalOr is called when exiting the LogicalOr production.
	ExitLogicalOr(c *LogicalOrContext)

	// ExitMultiplicative is called when exiting the Multiplicative production.
	ExitMultiplicative(c *MultiplicativeContext)

	// ExitAdditive is called when exiting the Additive production.
	ExitAdditive(c *AdditiveContext)

	// ExitMemberAccess is called when exiting the MemberAccess production.
	ExitMemberAccess(c *MemberAccessContext)

	// ExitErrorDefault is called when exiting the ErrorDefault production.
	ExitErrorDefault(c *ErrorDefaultContext)

	// ExitBitwiseXor is called when exiting the BitwiseXor production.
	ExitBitwiseXor(c *BitwiseXorContext)

	// ExitBitwiseOr is called when exiting the BitwiseOr production.
	ExitBitwiseOr(c *BitwiseOrContext)

	// ExitPrimary is called when exiting the Primary production.
	ExitPrimary(c *PrimaryContext)

	// ExitLogicalAnd is called when exiting the LogicalAnd production.
	ExitLogicalAnd(c *LogicalAndContext)

	// ExitWhenExpr is called when exiting the WhenExpr production.
	ExitWhenExpr(c *WhenExprContext)

	// ExitBitwiseAnd is called when exiting the BitwiseAnd production.
	ExitBitwiseAnd(c *BitwiseAndContext)

	// ExitEquality is called when exiting the Equality production.
	ExitEquality(c *EqualityContext)

	// ExitLambda is called when exiting the Lambda production.
	ExitLambda(c *LambdaContext)

	// ExitTernaryExpr is called when exiting the TernaryExpr production.
	ExitTernaryExpr(c *TernaryExprContext)

	// ExitPrimaryExpression is called when exiting the primaryExpression production.
	ExitPrimaryExpression(c *PrimaryExpressionContext)

	// ExitLiteral is called when exiting the literal production.
	ExitLiteral(c *LiteralContext)

	// ExitArrayLiteral is called when exiting the arrayLiteral production.
	ExitArrayLiteral(c *ArrayLiteralContext)

	// ExitArrayInitializer is called when exiting the arrayInitializer production.
	ExitArrayInitializer(c *ArrayInitializerContext)

	// ExitStructLiteral is called when exiting the structLiteral production.
	ExitStructLiteral(c *StructLiteralContext)

	// ExitFieldInitializer is called when exiting the fieldInitializer production.
	ExitFieldInitializer(c *FieldInitializerContext)

	// ExitTupleLiteral is called when exiting the tupleLiteral production.
	ExitTupleLiteral(c *TupleLiteralContext)

	// ExitInlineAssembly is called when exiting the inlineAssembly production.
	ExitInlineAssembly(c *InlineAssemblyContext)

	// ExitAsmOutputList is called when exiting the asmOutputList production.
	ExitAsmOutputList(c *AsmOutputListContext)

	// ExitAsmInputList is called when exiting the asmInputList production.
	ExitAsmInputList(c *AsmInputListContext)

	// ExitAsmClobberList is called when exiting the asmClobberList production.
	ExitAsmClobberList(c *AsmClobberListContext)

	// ExitAsmOutput is called when exiting the asmOutput production.
	ExitAsmOutput(c *AsmOutputContext)

	// ExitAsmInput is called when exiting the asmInput production.
	ExitAsmInput(c *AsmInputContext)

	// ExitSizeofExpression is called when exiting the sizeofExpression production.
	ExitSizeofExpression(c *SizeofExpressionContext)

	// ExitAlignofExpression is called when exiting the alignofExpression production.
	ExitAlignofExpression(c *AlignofExpressionContext)

	// ExitErrorLiteral is called when exiting the errorLiteral production.
	ExitErrorLiteral(c *ErrorLiteralContext)

	// ExitLambdaExpression is called when exiting the lambdaExpression production.
	ExitLambdaExpression(c *LambdaExpressionContext)

	// ExitLambdaParams is called when exiting the lambdaParams production.
	ExitLambdaParams(c *LambdaParamsContext)

	// ExitLambdaParam is called when exiting the lambdaParam production.
	ExitLambdaParam(c *LambdaParamContext)

	// ExitIfExpression is called when exiting the ifExpression production.
	ExitIfExpression(c *IfExpressionContext)

	// ExitTernaryExpression is called when exiting the ternaryExpression production.
	ExitTernaryExpression(c *TernaryExpressionContext)

	// ExitWhenExpression is called when exiting the whenExpression production.
	ExitWhenExpression(c *WhenExpressionContext)

	// ExitWhenArm is called when exiting the whenArm production.
	ExitWhenArm(c *WhenArmContext)

	// ExitMetafunctionExpr is called when exiting the metafunctionExpr production.
	ExitMetafunctionExpr(c *MetafunctionExprContext)

	// ExitArgumentList is called when exiting the argumentList production.
	ExitArgumentList(c *ArgumentListContext)

	// ExitExpressionList is called when exiting the expressionList production.
	ExitExpressionList(c *ExpressionListContext)
}
