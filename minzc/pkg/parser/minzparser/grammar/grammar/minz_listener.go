// Code generated from grammar/MinZ.g4 by ANTLR 4.13.2. DO NOT EDIT.

package minzparser // MinZ
import "github.com/antlr4-go/antlr/v4"

// MinZListener is a complete listener for a parse tree produced by MinZParser.
type MinZListener interface {
	antlr.ParseTreeListener

	// EnterSourceFile is called when entering the sourceFile production.
	EnterSourceFile(c *SourceFileContext)

	// EnterImportStatement is called when entering the importStatement production.
	EnterImportStatement(c *ImportStatementContext)

	// EnterImportPath is called when entering the importPath production.
	EnterImportPath(c *ImportPathContext)

	// EnterDeclaration is called when entering the declaration production.
	EnterDeclaration(c *DeclarationContext)

	// EnterFunctionDeclaration is called when entering the functionDeclaration production.
	EnterFunctionDeclaration(c *FunctionDeclarationContext)

	// EnterFunctionPrefix is called when entering the functionPrefix production.
	EnterFunctionPrefix(c *FunctionPrefixContext)

	// EnterVisibility is called when entering the visibility production.
	EnterVisibility(c *VisibilityContext)

	// EnterGenericParams is called when entering the genericParams production.
	EnterGenericParams(c *GenericParamsContext)

	// EnterParameterList is called when entering the parameterList production.
	EnterParameterList(c *ParameterListContext)

	// EnterParameter is called when entering the parameter production.
	EnterParameter(c *ParameterContext)

	// EnterReturnType is called when entering the returnType production.
	EnterReturnType(c *ReturnTypeContext)

	// EnterErrorReturnType is called when entering the errorReturnType production.
	EnterErrorReturnType(c *ErrorReturnTypeContext)

	// EnterStructDeclaration is called when entering the structDeclaration production.
	EnterStructDeclaration(c *StructDeclarationContext)

	// EnterFieldList is called when entering the fieldList production.
	EnterFieldList(c *FieldListContext)

	// EnterField is called when entering the field production.
	EnterField(c *FieldContext)

	// EnterEnumDeclaration is called when entering the enumDeclaration production.
	EnterEnumDeclaration(c *EnumDeclarationContext)

	// EnterEnumMemberList is called when entering the enumMemberList production.
	EnterEnumMemberList(c *EnumMemberListContext)

	// EnterEnumMember is called when entering the enumMember production.
	EnterEnumMember(c *EnumMemberContext)

	// EnterTypeAliasDeclaration is called when entering the typeAliasDeclaration production.
	EnterTypeAliasDeclaration(c *TypeAliasDeclarationContext)

	// EnterInterfaceDeclaration is called when entering the interfaceDeclaration production.
	EnterInterfaceDeclaration(c *InterfaceDeclarationContext)

	// EnterInterfaceMethodList is called when entering the interfaceMethodList production.
	EnterInterfaceMethodList(c *InterfaceMethodListContext)

	// EnterInterfaceMethod is called when entering the interfaceMethod production.
	EnterInterfaceMethod(c *InterfaceMethodContext)

	// EnterImplBlock is called when entering the implBlock production.
	EnterImplBlock(c *ImplBlockContext)

	// EnterConstDeclaration is called when entering the constDeclaration production.
	EnterConstDeclaration(c *ConstDeclarationContext)

	// EnterGlobalVarDeclaration is called when entering the globalVarDeclaration production.
	EnterGlobalVarDeclaration(c *GlobalVarDeclarationContext)

	// EnterCompileTimeDeclaration is called when entering the compileTimeDeclaration production.
	EnterCompileTimeDeclaration(c *CompileTimeDeclarationContext)

	// EnterCompileTimeIf is called when entering the compileTimeIf production.
	EnterCompileTimeIf(c *CompileTimeIfContext)

	// EnterCompileTimeMinz is called when entering the compileTimeMinz production.
	EnterCompileTimeMinz(c *CompileTimeMinzContext)

	// EnterCompileTimeMir is called when entering the compileTimeMir production.
	EnterCompileTimeMir(c *CompileTimeMirContext)

	// EnterMirBlock is called when entering the mirBlock production.
	EnterMirBlock(c *MirBlockContext)

	// EnterMirStatement is called when entering the mirStatement production.
	EnterMirStatement(c *MirStatementContext)

	// EnterMirInstruction is called when entering the mirInstruction production.
	EnterMirInstruction(c *MirInstructionContext)

	// EnterMirOperand is called when entering the mirOperand production.
	EnterMirOperand(c *MirOperandContext)

	// EnterMirRegister is called when entering the mirRegister production.
	EnterMirRegister(c *MirRegisterContext)

	// EnterMirImmediate is called when entering the mirImmediate production.
	EnterMirImmediate(c *MirImmediateContext)

	// EnterMirMemory is called when entering the mirMemory production.
	EnterMirMemory(c *MirMemoryContext)

	// EnterMirLabel is called when entering the mirLabel production.
	EnterMirLabel(c *MirLabelContext)

	// EnterTargetBlock is called when entering the targetBlock production.
	EnterTargetBlock(c *TargetBlockContext)

	// EnterStatement is called when entering the statement production.
	EnterStatement(c *StatementContext)

	// EnterLetStatement is called when entering the letStatement production.
	EnterLetStatement(c *LetStatementContext)

	// EnterVarStatement is called when entering the varStatement production.
	EnterVarStatement(c *VarStatementContext)

	// EnterAssignmentStatement is called when entering the assignmentStatement production.
	EnterAssignmentStatement(c *AssignmentStatementContext)

	// EnterExpressionStatement is called when entering the expressionStatement production.
	EnterExpressionStatement(c *ExpressionStatementContext)

	// EnterReturnStatement is called when entering the returnStatement production.
	EnterReturnStatement(c *ReturnStatementContext)

	// EnterIfStatement is called when entering the ifStatement production.
	EnterIfStatement(c *IfStatementContext)

	// EnterWhileStatement is called when entering the whileStatement production.
	EnterWhileStatement(c *WhileStatementContext)

	// EnterForStatement is called when entering the forStatement production.
	EnterForStatement(c *ForStatementContext)

	// EnterLoopStatement is called when entering the loopStatement production.
	EnterLoopStatement(c *LoopStatementContext)

	// EnterCaseStatement is called when entering the caseStatement production.
	EnterCaseStatement(c *CaseStatementContext)

	// EnterCaseArm is called when entering the caseArm production.
	EnterCaseArm(c *CaseArmContext)

	// EnterBlockStatement is called when entering the blockStatement production.
	EnterBlockStatement(c *BlockStatementContext)

	// EnterBlock is called when entering the block production.
	EnterBlock(c *BlockContext)

	// EnterBreakStatement is called when entering the breakStatement production.
	EnterBreakStatement(c *BreakStatementContext)

	// EnterContinueStatement is called when entering the continueStatement production.
	EnterContinueStatement(c *ContinueStatementContext)

	// EnterDeferStatement is called when entering the deferStatement production.
	EnterDeferStatement(c *DeferStatementContext)

	// EnterAsmStatement is called when entering the asmStatement production.
	EnterAsmStatement(c *AsmStatementContext)

	// EnterAsmBlock is called when entering the asmBlock production.
	EnterAsmBlock(c *AsmBlockContext)

	// EnterPattern is called when entering the pattern production.
	EnterPattern(c *PatternContext)

	// EnterLiteralPattern is called when entering the literalPattern production.
	EnterLiteralPattern(c *LiteralPatternContext)

	// EnterIdentifierPattern is called when entering the identifierPattern production.
	EnterIdentifierPattern(c *IdentifierPatternContext)

	// EnterWildcardPattern is called when entering the wildcardPattern production.
	EnterWildcardPattern(c *WildcardPatternContext)

	// EnterTuplePattern is called when entering the tuplePattern production.
	EnterTuplePattern(c *TuplePatternContext)

	// EnterStructPattern is called when entering the structPattern production.
	EnterStructPattern(c *StructPatternContext)

	// EnterFieldPattern is called when entering the fieldPattern production.
	EnterFieldPattern(c *FieldPatternContext)

	// EnterExpression is called when entering the expression production.
	EnterExpression(c *ExpressionContext)

	// EnterLambdaExpression is called when entering the lambdaExpression production.
	EnterLambdaExpression(c *LambdaExpressionContext)

	// EnterLambdaParams is called when entering the lambdaParams production.
	EnterLambdaParams(c *LambdaParamsContext)

	// EnterLambdaParam is called when entering the lambdaParam production.
	EnterLambdaParam(c *LambdaParamContext)

	// EnterConditionalExpression is called when entering the conditionalExpression production.
	EnterConditionalExpression(c *ConditionalExpressionContext)

	// EnterWhenExpression is called when entering the whenExpression production.
	EnterWhenExpression(c *WhenExpressionContext)

	// EnterWhenArm is called when entering the whenArm production.
	EnterWhenArm(c *WhenArmContext)

	// EnterLogicalOrExpression is called when entering the logicalOrExpression production.
	EnterLogicalOrExpression(c *LogicalOrExpressionContext)

	// EnterLogicalAndExpression is called when entering the logicalAndExpression production.
	EnterLogicalAndExpression(c *LogicalAndExpressionContext)

	// EnterEqualityExpression is called when entering the equalityExpression production.
	EnterEqualityExpression(c *EqualityExpressionContext)

	// EnterRelationalExpression is called when entering the relationalExpression production.
	EnterRelationalExpression(c *RelationalExpressionContext)

	// EnterAdditiveExpression is called when entering the additiveExpression production.
	EnterAdditiveExpression(c *AdditiveExpressionContext)

	// EnterMultiplicativeExpression is called when entering the multiplicativeExpression production.
	EnterMultiplicativeExpression(c *MultiplicativeExpressionContext)

	// EnterCastExpression is called when entering the castExpression production.
	EnterCastExpression(c *CastExpressionContext)

	// EnterUnaryExpression is called when entering the unaryExpression production.
	EnterUnaryExpression(c *UnaryExpressionContext)

	// EnterPostfixExpression is called when entering the postfixExpression production.
	EnterPostfixExpression(c *PostfixExpressionContext)

	// EnterPostfixOperator is called when entering the postfixOperator production.
	EnterPostfixOperator(c *PostfixOperatorContext)

	// EnterArgumentList is called when entering the argumentList production.
	EnterArgumentList(c *ArgumentListContext)

	// EnterPrimaryExpression is called when entering the primaryExpression production.
	EnterPrimaryExpression(c *PrimaryExpressionContext)

	// EnterLiteral is called when entering the literal production.
	EnterLiteral(c *LiteralContext)

	// EnterNumberLiteral is called when entering the numberLiteral production.
	EnterNumberLiteral(c *NumberLiteralContext)

	// EnterStringLiteral is called when entering the stringLiteral production.
	EnterStringLiteral(c *StringLiteralContext)

	// EnterCharLiteral is called when entering the charLiteral production.
	EnterCharLiteral(c *CharLiteralContext)

	// EnterBooleanLiteral is called when entering the booleanLiteral production.
	EnterBooleanLiteral(c *BooleanLiteralContext)

	// EnterArrayLiteral is called when entering the arrayLiteral production.
	EnterArrayLiteral(c *ArrayLiteralContext)

	// EnterStructLiteral is called when entering the structLiteral production.
	EnterStructLiteral(c *StructLiteralContext)

	// EnterFieldInit is called when entering the fieldInit production.
	EnterFieldInit(c *FieldInitContext)

	// EnterMetafunction is called when entering the metafunction production.
	EnterMetafunction(c *MetafunctionContext)

	// EnterLogMetafunction is called when entering the logMetafunction production.
	EnterLogMetafunction(c *LogMetafunctionContext)

	// EnterLogLevel is called when entering the logLevel production.
	EnterLogLevel(c *LogLevelContext)

	// EnterLuaBlock is called when entering the luaBlock production.
	EnterLuaBlock(c *LuaBlockContext)

	// EnterInlineAssembly is called when entering the inlineAssembly production.
	EnterInlineAssembly(c *InlineAssemblyContext)

	// EnterAsmOperand is called when entering the asmOperand production.
	EnterAsmOperand(c *AsmOperandContext)

	// EnterAsmConstraint is called when entering the asmConstraint production.
	EnterAsmConstraint(c *AsmConstraintContext)

	// EnterType is called when entering the type production.
	EnterType(c *TypeContext)

	// EnterPrimitiveType is called when entering the primitiveType production.
	EnterPrimitiveType(c *PrimitiveTypeContext)

	// EnterArrayType is called when entering the arrayType production.
	EnterArrayType(c *ArrayTypeContext)

	// EnterPointerType is called when entering the pointerType production.
	EnterPointerType(c *PointerTypeContext)

	// EnterFunctionType is called when entering the functionType production.
	EnterFunctionType(c *FunctionTypeContext)

	// EnterTypeList is called when entering the typeList production.
	EnterTypeList(c *TypeListContext)

	// EnterStructType is called when entering the structType production.
	EnterStructType(c *StructTypeContext)

	// EnterEnumType is called when entering the enumType production.
	EnterEnumType(c *EnumTypeContext)

	// EnterBitStructType is called when entering the bitStructType production.
	EnterBitStructType(c *BitStructTypeContext)

	// EnterBitFieldList is called when entering the bitFieldList production.
	EnterBitFieldList(c *BitFieldListContext)

	// EnterBitField is called when entering the bitField production.
	EnterBitField(c *BitFieldContext)

	// EnterTypeIdentifier is called when entering the typeIdentifier production.
	EnterTypeIdentifier(c *TypeIdentifierContext)

	// EnterErrorType is called when entering the errorType production.
	EnterErrorType(c *ErrorTypeContext)

	// ExitSourceFile is called when exiting the sourceFile production.
	ExitSourceFile(c *SourceFileContext)

	// ExitImportStatement is called when exiting the importStatement production.
	ExitImportStatement(c *ImportStatementContext)

	// ExitImportPath is called when exiting the importPath production.
	ExitImportPath(c *ImportPathContext)

	// ExitDeclaration is called when exiting the declaration production.
	ExitDeclaration(c *DeclarationContext)

	// ExitFunctionDeclaration is called when exiting the functionDeclaration production.
	ExitFunctionDeclaration(c *FunctionDeclarationContext)

	// ExitFunctionPrefix is called when exiting the functionPrefix production.
	ExitFunctionPrefix(c *FunctionPrefixContext)

	// ExitVisibility is called when exiting the visibility production.
	ExitVisibility(c *VisibilityContext)

	// ExitGenericParams is called when exiting the genericParams production.
	ExitGenericParams(c *GenericParamsContext)

	// ExitParameterList is called when exiting the parameterList production.
	ExitParameterList(c *ParameterListContext)

	// ExitParameter is called when exiting the parameter production.
	ExitParameter(c *ParameterContext)

	// ExitReturnType is called when exiting the returnType production.
	ExitReturnType(c *ReturnTypeContext)

	// ExitErrorReturnType is called when exiting the errorReturnType production.
	ExitErrorReturnType(c *ErrorReturnTypeContext)

	// ExitStructDeclaration is called when exiting the structDeclaration production.
	ExitStructDeclaration(c *StructDeclarationContext)

	// ExitFieldList is called when exiting the fieldList production.
	ExitFieldList(c *FieldListContext)

	// ExitField is called when exiting the field production.
	ExitField(c *FieldContext)

	// ExitEnumDeclaration is called when exiting the enumDeclaration production.
	ExitEnumDeclaration(c *EnumDeclarationContext)

	// ExitEnumMemberList is called when exiting the enumMemberList production.
	ExitEnumMemberList(c *EnumMemberListContext)

	// ExitEnumMember is called when exiting the enumMember production.
	ExitEnumMember(c *EnumMemberContext)

	// ExitTypeAliasDeclaration is called when exiting the typeAliasDeclaration production.
	ExitTypeAliasDeclaration(c *TypeAliasDeclarationContext)

	// ExitInterfaceDeclaration is called when exiting the interfaceDeclaration production.
	ExitInterfaceDeclaration(c *InterfaceDeclarationContext)

	// ExitInterfaceMethodList is called when exiting the interfaceMethodList production.
	ExitInterfaceMethodList(c *InterfaceMethodListContext)

	// ExitInterfaceMethod is called when exiting the interfaceMethod production.
	ExitInterfaceMethod(c *InterfaceMethodContext)

	// ExitImplBlock is called when exiting the implBlock production.
	ExitImplBlock(c *ImplBlockContext)

	// ExitConstDeclaration is called when exiting the constDeclaration production.
	ExitConstDeclaration(c *ConstDeclarationContext)

	// ExitGlobalVarDeclaration is called when exiting the globalVarDeclaration production.
	ExitGlobalVarDeclaration(c *GlobalVarDeclarationContext)

	// ExitCompileTimeDeclaration is called when exiting the compileTimeDeclaration production.
	ExitCompileTimeDeclaration(c *CompileTimeDeclarationContext)

	// ExitCompileTimeIf is called when exiting the compileTimeIf production.
	ExitCompileTimeIf(c *CompileTimeIfContext)

	// ExitCompileTimeMinz is called when exiting the compileTimeMinz production.
	ExitCompileTimeMinz(c *CompileTimeMinzContext)

	// ExitCompileTimeMir is called when exiting the compileTimeMir production.
	ExitCompileTimeMir(c *CompileTimeMirContext)

	// ExitMirBlock is called when exiting the mirBlock production.
	ExitMirBlock(c *MirBlockContext)

	// ExitMirStatement is called when exiting the mirStatement production.
	ExitMirStatement(c *MirStatementContext)

	// ExitMirInstruction is called when exiting the mirInstruction production.
	ExitMirInstruction(c *MirInstructionContext)

	// ExitMirOperand is called when exiting the mirOperand production.
	ExitMirOperand(c *MirOperandContext)

	// ExitMirRegister is called when exiting the mirRegister production.
	ExitMirRegister(c *MirRegisterContext)

	// ExitMirImmediate is called when exiting the mirImmediate production.
	ExitMirImmediate(c *MirImmediateContext)

	// ExitMirMemory is called when exiting the mirMemory production.
	ExitMirMemory(c *MirMemoryContext)

	// ExitMirLabel is called when exiting the mirLabel production.
	ExitMirLabel(c *MirLabelContext)

	// ExitTargetBlock is called when exiting the targetBlock production.
	ExitTargetBlock(c *TargetBlockContext)

	// ExitStatement is called when exiting the statement production.
	ExitStatement(c *StatementContext)

	// ExitLetStatement is called when exiting the letStatement production.
	ExitLetStatement(c *LetStatementContext)

	// ExitVarStatement is called when exiting the varStatement production.
	ExitVarStatement(c *VarStatementContext)

	// ExitAssignmentStatement is called when exiting the assignmentStatement production.
	ExitAssignmentStatement(c *AssignmentStatementContext)

	// ExitExpressionStatement is called when exiting the expressionStatement production.
	ExitExpressionStatement(c *ExpressionStatementContext)

	// ExitReturnStatement is called when exiting the returnStatement production.
	ExitReturnStatement(c *ReturnStatementContext)

	// ExitIfStatement is called when exiting the ifStatement production.
	ExitIfStatement(c *IfStatementContext)

	// ExitWhileStatement is called when exiting the whileStatement production.
	ExitWhileStatement(c *WhileStatementContext)

	// ExitForStatement is called when exiting the forStatement production.
	ExitForStatement(c *ForStatementContext)

	// ExitLoopStatement is called when exiting the loopStatement production.
	ExitLoopStatement(c *LoopStatementContext)

	// ExitCaseStatement is called when exiting the caseStatement production.
	ExitCaseStatement(c *CaseStatementContext)

	// ExitCaseArm is called when exiting the caseArm production.
	ExitCaseArm(c *CaseArmContext)

	// ExitBlockStatement is called when exiting the blockStatement production.
	ExitBlockStatement(c *BlockStatementContext)

	// ExitBlock is called when exiting the block production.
	ExitBlock(c *BlockContext)

	// ExitBreakStatement is called when exiting the breakStatement production.
	ExitBreakStatement(c *BreakStatementContext)

	// ExitContinueStatement is called when exiting the continueStatement production.
	ExitContinueStatement(c *ContinueStatementContext)

	// ExitDeferStatement is called when exiting the deferStatement production.
	ExitDeferStatement(c *DeferStatementContext)

	// ExitAsmStatement is called when exiting the asmStatement production.
	ExitAsmStatement(c *AsmStatementContext)

	// ExitAsmBlock is called when exiting the asmBlock production.
	ExitAsmBlock(c *AsmBlockContext)

	// ExitPattern is called when exiting the pattern production.
	ExitPattern(c *PatternContext)

	// ExitLiteralPattern is called when exiting the literalPattern production.
	ExitLiteralPattern(c *LiteralPatternContext)

	// ExitIdentifierPattern is called when exiting the identifierPattern production.
	ExitIdentifierPattern(c *IdentifierPatternContext)

	// ExitWildcardPattern is called when exiting the wildcardPattern production.
	ExitWildcardPattern(c *WildcardPatternContext)

	// ExitTuplePattern is called when exiting the tuplePattern production.
	ExitTuplePattern(c *TuplePatternContext)

	// ExitStructPattern is called when exiting the structPattern production.
	ExitStructPattern(c *StructPatternContext)

	// ExitFieldPattern is called when exiting the fieldPattern production.
	ExitFieldPattern(c *FieldPatternContext)

	// ExitExpression is called when exiting the expression production.
	ExitExpression(c *ExpressionContext)

	// ExitLambdaExpression is called when exiting the lambdaExpression production.
	ExitLambdaExpression(c *LambdaExpressionContext)

	// ExitLambdaParams is called when exiting the lambdaParams production.
	ExitLambdaParams(c *LambdaParamsContext)

	// ExitLambdaParam is called when exiting the lambdaParam production.
	ExitLambdaParam(c *LambdaParamContext)

	// ExitConditionalExpression is called when exiting the conditionalExpression production.
	ExitConditionalExpression(c *ConditionalExpressionContext)

	// ExitWhenExpression is called when exiting the whenExpression production.
	ExitWhenExpression(c *WhenExpressionContext)

	// ExitWhenArm is called when exiting the whenArm production.
	ExitWhenArm(c *WhenArmContext)

	// ExitLogicalOrExpression is called when exiting the logicalOrExpression production.
	ExitLogicalOrExpression(c *LogicalOrExpressionContext)

	// ExitLogicalAndExpression is called when exiting the logicalAndExpression production.
	ExitLogicalAndExpression(c *LogicalAndExpressionContext)

	// ExitEqualityExpression is called when exiting the equalityExpression production.
	ExitEqualityExpression(c *EqualityExpressionContext)

	// ExitRelationalExpression is called when exiting the relationalExpression production.
	ExitRelationalExpression(c *RelationalExpressionContext)

	// ExitAdditiveExpression is called when exiting the additiveExpression production.
	ExitAdditiveExpression(c *AdditiveExpressionContext)

	// ExitMultiplicativeExpression is called when exiting the multiplicativeExpression production.
	ExitMultiplicativeExpression(c *MultiplicativeExpressionContext)

	// ExitCastExpression is called when exiting the castExpression production.
	ExitCastExpression(c *CastExpressionContext)

	// ExitUnaryExpression is called when exiting the unaryExpression production.
	ExitUnaryExpression(c *UnaryExpressionContext)

	// ExitPostfixExpression is called when exiting the postfixExpression production.
	ExitPostfixExpression(c *PostfixExpressionContext)

	// ExitPostfixOperator is called when exiting the postfixOperator production.
	ExitPostfixOperator(c *PostfixOperatorContext)

	// ExitArgumentList is called when exiting the argumentList production.
	ExitArgumentList(c *ArgumentListContext)

	// ExitPrimaryExpression is called when exiting the primaryExpression production.
	ExitPrimaryExpression(c *PrimaryExpressionContext)

	// ExitLiteral is called when exiting the literal production.
	ExitLiteral(c *LiteralContext)

	// ExitNumberLiteral is called when exiting the numberLiteral production.
	ExitNumberLiteral(c *NumberLiteralContext)

	// ExitStringLiteral is called when exiting the stringLiteral production.
	ExitStringLiteral(c *StringLiteralContext)

	// ExitCharLiteral is called when exiting the charLiteral production.
	ExitCharLiteral(c *CharLiteralContext)

	// ExitBooleanLiteral is called when exiting the booleanLiteral production.
	ExitBooleanLiteral(c *BooleanLiteralContext)

	// ExitArrayLiteral is called when exiting the arrayLiteral production.
	ExitArrayLiteral(c *ArrayLiteralContext)

	// ExitStructLiteral is called when exiting the structLiteral production.
	ExitStructLiteral(c *StructLiteralContext)

	// ExitFieldInit is called when exiting the fieldInit production.
	ExitFieldInit(c *FieldInitContext)

	// ExitMetafunction is called when exiting the metafunction production.
	ExitMetafunction(c *MetafunctionContext)

	// ExitLogMetafunction is called when exiting the logMetafunction production.
	ExitLogMetafunction(c *LogMetafunctionContext)

	// ExitLogLevel is called when exiting the logLevel production.
	ExitLogLevel(c *LogLevelContext)

	// ExitLuaBlock is called when exiting the luaBlock production.
	ExitLuaBlock(c *LuaBlockContext)

	// ExitInlineAssembly is called when exiting the inlineAssembly production.
	ExitInlineAssembly(c *InlineAssemblyContext)

	// ExitAsmOperand is called when exiting the asmOperand production.
	ExitAsmOperand(c *AsmOperandContext)

	// ExitAsmConstraint is called when exiting the asmConstraint production.
	ExitAsmConstraint(c *AsmConstraintContext)

	// ExitType is called when exiting the type production.
	ExitType(c *TypeContext)

	// ExitPrimitiveType is called when exiting the primitiveType production.
	ExitPrimitiveType(c *PrimitiveTypeContext)

	// ExitArrayType is called when exiting the arrayType production.
	ExitArrayType(c *ArrayTypeContext)

	// ExitPointerType is called when exiting the pointerType production.
	ExitPointerType(c *PointerTypeContext)

	// ExitFunctionType is called when exiting the functionType production.
	ExitFunctionType(c *FunctionTypeContext)

	// ExitTypeList is called when exiting the typeList production.
	ExitTypeList(c *TypeListContext)

	// ExitStructType is called when exiting the structType production.
	ExitStructType(c *StructTypeContext)

	// ExitEnumType is called when exiting the enumType production.
	ExitEnumType(c *EnumTypeContext)

	// ExitBitStructType is called when exiting the bitStructType production.
	ExitBitStructType(c *BitStructTypeContext)

	// ExitBitFieldList is called when exiting the bitFieldList production.
	ExitBitFieldList(c *BitFieldListContext)

	// ExitBitField is called when exiting the bitField production.
	ExitBitField(c *BitFieldContext)

	// ExitTypeIdentifier is called when exiting the typeIdentifier production.
	ExitTypeIdentifier(c *TypeIdentifierContext)

	// ExitErrorType is called when exiting the errorType production.
	ExitErrorType(c *ErrorTypeContext)
}
