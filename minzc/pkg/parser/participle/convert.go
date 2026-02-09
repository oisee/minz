package participle

import (
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/minz/minzc/pkg/ast"
)

// Converter converts Participle AST to pkg/ast types
type Converter struct {
	filename string
}

// NewConverter creates a new AST converter
func NewConverter(filename string) *Converter {
	return &Converter{filename: filename}
}

// Convert converts a Participle File to an ast.File
func (c *Converter) Convert(f *File) (*ast.File, error) {
	result := &ast.File{
		Name: c.filename,
	}

	for _, decl := range f.Declarations {
		converted := c.convertDecl(decl)
		if converted != nil {
			// Handle imports separately
			if imp, ok := converted.(*ast.ImportStmt); ok {
				result.Imports = append(result.Imports, imp)
			} else if d, ok := converted.(ast.Declaration); ok {
				result.Declarations = append(result.Declarations, d)
			}
		}
	}

	return result, nil
}

// convertPos converts lexer.Position to ast.Position
func (c *Converter) convertPos(p lexer.Position) ast.Position {
	return ast.Position{
		Line:   p.Line,
		Column: p.Column,
		Offset: p.Offset,
	}
}

// convertDecl converts a Participle Decl to ast types
func (c *Converter) convertDecl(d *Decl) ast.Node {
	switch {
	case d.TripleBracket != nil:
		// Parse @name[[[body]]] - extract name and body
		raw := *d.TripleBracket
		// Find name between @ and [[[
		atIdx := 0
		bracketIdx := strings.Index(raw, "[[[")
		if bracketIdx > 1 {
			name := raw[1:bracketIdx]
			body := ""
			if bracketIdx+3 < len(raw)-3 {
				body = raw[bracketIdx+3 : len(raw)-3]
			}
			return &ast.ExpressionDecl{
				Expression: &ast.MetafunctionCall{
					Name: name,
					Arguments: []ast.Expression{
						&ast.StringLiteral{Value: body},
					},
				},
				StartPos: c.convertPos(d.Pos),
			}
		}
		_ = atIdx // suppress unused
		return nil
	case d.Module != nil:
		// Module declarations are informational only
		return nil
	case d.Import != nil:
		return c.convertImport(d.Import)
	case d.Function != nil:
		return c.convertFunction(d.Function)
	case d.Struct != nil:
		return c.convertStruct(d.Struct)
	case d.Enum != nil:
		return c.convertEnum(d.Enum)
	case d.Interface != nil:
		return c.convertInterface(d.Interface)
	case d.Impl != nil:
		return c.convertImpl(d.Impl)
	case d.Global != nil:
		return c.convertGlobal(d.Global)
	case d.Const != nil:
		return c.convertConstDecl(d.Const)
	case d.Type != nil:
		return c.convertTypeDecl(d.Type)
	case d.Meta != nil:
		return c.convertMetaDecl(d.Meta)
	}
	return nil
}

func (c *Converter) convertImport(i *ImportDecl) *ast.ImportStmt {
	path := strings.Join(i.Path, ".")
	alias := ""
	if i.Alias != nil {
		alias = *i.Alias
	}
	return &ast.ImportStmt{
		Path:     path,
		Alias:    alias,
		StartPos: c.convertPos(i.Pos),
	}
}

func (c *Converter) convertFunction(f *FunctionDecl) *ast.FunctionDecl {
	result := &ast.FunctionDecl{
		Name:     f.Name,
		IsPublic: f.Public,
		StartPos: c.convertPos(f.Pos),
	}

	// Convert generic params
	for _, tp := range f.TypeParams {
		gp := &ast.GenericParam{
			Name:     tp.Name,
			StartPos: c.convertPos(tp.Pos),
		}
		if tp.Constraint != nil {
			gp.Bounds = []string{c.typeRefToString(tp.Constraint)}
		}
		result.GenericParams = append(result.GenericParams, gp)
	}

	// Convert params
	for _, p := range f.Params {
		param := &ast.Parameter{
			IsSelf:   p.Self,
			StartPos: c.convertPos(p.Pos),
		}
		if p.Name != nil {
			param.Name = *p.Name
		}
		if p.Type != nil {
			param.Type = c.convertTypeRef(p.Type)
		}
		result.Params = append(result.Params, param)
	}

	// Convert return type (default to void if not specified)
	if f.ReturnType != nil {
		result.ReturnType = c.convertTypeRef(f.ReturnType)
	} else {
		result.ReturnType = &ast.PrimitiveType{Name: "void"}
	}

	// Convert body
	if f.Body != nil {
		result.Body = c.convertStmtBlock(f.Body)
	}

	// Convert attributes
	for _, attr := range f.Attributes {
		result.Attributes = append(result.Attributes, c.convertAttribute(attr))
	}

	return result
}

func (c *Converter) convertStruct(s *StructDecl) *ast.StructDecl {
	result := &ast.StructDecl{
		Name:     s.Name,
		IsPublic: s.Public,
		StartPos: c.convertPos(s.Pos),
	}

	for _, f := range s.Fields {
		field := &ast.Field{
			Name:     f.Name,
			IsPublic: f.Public,
			StartPos: c.convertPos(f.Pos),
		}
		if f.Type != nil {
			field.Type = c.convertTypeRef(f.Type)
		}
		result.Fields = append(result.Fields, field)
	}

	return result
}

func (c *Converter) convertEnum(e *EnumDecl) *ast.EnumDecl {
	result := &ast.EnumDecl{
		Name:     e.Name,
		IsPublic: e.Public,
		StartPos: c.convertPos(e.Pos),
	}

	for _, v := range e.Variants {
		result.Variants = append(result.Variants, v.Name)
	}

	return result
}

func (c *Converter) convertInterface(i *InterfaceDecl) *ast.InterfaceDecl {
	result := &ast.InterfaceDecl{
		Name:     i.Name,
		IsPublic: i.Public,
		StartPos: c.convertPos(i.Pos),
	}

	for _, m := range i.Methods {
		method := &ast.InterfaceMethod{
			Name:     m.Name,
			StartPos: c.convertPos(m.Pos),
		}
		for _, p := range m.Params {
			param := &ast.Parameter{
				IsSelf:   p.Self,
				StartPos: c.convertPos(p.Pos),
			}
			if p.Name != nil {
				param.Name = *p.Name
			}
			if p.Type != nil {
				param.Type = c.convertTypeRef(p.Type)
			}
			method.Params = append(method.Params, param)
		}
		if m.ReturnType != nil {
			method.ReturnType = c.convertTypeRef(m.ReturnType)
		}
		result.Methods = append(result.Methods, method)
	}

	return result
}

func (c *Converter) convertImpl(i *ImplDecl) *ast.ImplBlock {
	result := &ast.ImplBlock{
		StartPos: c.convertPos(i.Pos),
	}

	if i.Trait != nil {
		result.InterfaceName = c.typeRefToString(i.Trait)
	}
	if i.Type != nil {
		result.ForType = c.convertTypeRef(i.Type)
	}

	for _, m := range i.Methods {
		result.Methods = append(result.Methods, c.convertFunction(m))
	}

	return result
}

func (c *Converter) convertGlobal(g *GlobalDecl) *ast.VarDecl {
	result := &ast.VarDecl{
		Name:      g.Name,
		IsPublic:  g.Public,
		IsMutable: true, // globals are mutable
		StartPos:  c.convertPos(g.Pos),
	}

	if g.Type != nil {
		result.Type = c.convertTypeRef(g.Type)
	}
	if g.Value != nil {
		result.Value = c.convertExpr(g.Value)
	}

	return result
}

func (c *Converter) convertConstDecl(cd *ConstDecl) *ast.ConstDecl {
	result := &ast.ConstDecl{
		Name:     cd.Name,
		IsPublic: cd.Public,
		StartPos: c.convertPos(cd.Pos),
	}

	if cd.Type != nil {
		result.Type = c.convertTypeRef(cd.Type)
	}
	if cd.Value != nil {
		result.Value = c.convertExpr(cd.Value)
	}

	return result
}

func (c *Converter) convertTypeDecl(t *TypeDecl) *ast.TypeDecl {
	result := &ast.TypeDecl{
		Name:     t.Name,
		IsPublic: t.Public,
		StartPos: c.convertPos(t.Pos),
	}

	if t.Type != nil {
		result.Type = c.convertTypeRef(t.Type)
	}

	return result
}

func (c *Converter) convertMetaDecl(m *MetaDecl) ast.Declaration {
	// Convert to MetafunctionCall wrapped in ExpressionDecl
	call := &ast.MetafunctionCall{
		Name:     m.Name,
		StartPos: c.convertPos(m.Pos),
	}
	for _, arg := range m.Args {
		call.Arguments = append(call.Arguments, c.convertExpr(arg))
	}

	return &ast.ExpressionDecl{
		Expression: call,
		StartPos:   c.convertPos(m.Pos),
	}
}

func (c *Converter) convertAttribute(a *Attribute) *ast.Attribute {
	result := &ast.Attribute{
		Name:     a.Name,
		StartPos: c.convertPos(a.Pos),
	}
	for _, arg := range a.Args {
		result.Arguments = append(result.Arguments, c.convertExpr(arg))
	}
	return result
}

func (c *Converter) convertStmtBlock(b *StmtBlock) *ast.BlockStmt {
	result := &ast.BlockStmt{
		StartPos: c.convertPos(b.Pos),
	}

	for _, s := range b.Statements {
		stmt := c.convertStmt(s)
		if stmt != nil {
			result.Statements = append(result.Statements, stmt)
		}
	}

	return result
}

func (c *Converter) convertStmt(s *Stmt) ast.Statement {
	switch {
	case s.Asm != nil:
		// Extract code from "asm { ... }"
		code := s.Asm.Code
		if len(code) > 5 {
			// Remove "asm {" and "}"
			code = code[4:] // Remove "asm "
			if len(code) > 0 && code[0] == '{' {
				code = code[1:]
			}
			if len(code) > 0 && code[len(code)-1] == '}' {
				code = code[:len(code)-1]
			}
		}
		return &ast.AsmStmt{
			Code:     code,
			StartPos: c.convertPos(s.Pos),
		}
	case s.Mir != nil:
		// Extract code from "mir { ... }"
		code := s.Mir.Code
		if len(code) > 5 {
			code = code[4:]
			if len(code) > 0 && code[0] == '{' {
				code = code[1:]
			}
			if len(code) > 0 && code[len(code)-1] == '}' {
				code = code[:len(code)-1]
			}
		}
		return &ast.AsmStmt{
			Name:     "mir",
			Code:     code,
			StartPos: c.convertPos(s.Pos),
		}
	case s.Let != nil:
		return c.convertLetStmt(s.Let)
	case s.Const != nil:
		return c.convertConstStmt(s.Const)
	case s.Return != nil:
		return c.convertReturnStmt(s.Return)
	case s.If != nil:
		return c.convertIfStmt(s.If)
	case s.While != nil:
		return c.convertWhileStmt(s.While)
	case s.For != nil:
		return c.convertForStmt(s.For)
	case s.Loop != nil:
		return c.convertLoopStmt(s.Loop)
	case s.Break != nil:
		return nil // TODO: add BreakStmt to ast
	case s.Continue != nil:
		return nil // TODO: add ContinueStmt to ast
	case s.Expr != nil:
		return c.convertExprStmt(s.Expr)
	}
	return nil
}

func (c *Converter) convertLetStmt(l *LetStmt) *ast.VarDecl {
	result := &ast.VarDecl{
		Name:      l.Name,
		IsMutable: l.Mutable,
		StartPos:  c.convertPos(l.Pos),
	}

	if l.Type != nil {
		result.Type = c.convertTypeRef(l.Type)
	}
	if l.Value != nil {
		result.Value = c.convertExpr(l.Value)
	}

	return result
}

func (c *Converter) convertConstStmt(cs *ConstStmt) *ast.ConstDecl {
	result := &ast.ConstDecl{
		Name:     cs.Name,
		StartPos: c.convertPos(cs.Pos),
	}

	if cs.Type != nil {
		result.Type = c.convertTypeRef(cs.Type)
	}
	if cs.Value != nil {
		result.Value = c.convertExpr(cs.Value)
	}

	return result
}

func (c *Converter) convertReturnStmt(r *ReturnStmt) *ast.ReturnStmt {
	result := &ast.ReturnStmt{
		StartPos: c.convertPos(r.Pos),
	}

	if r.Value != nil {
		result.Value = c.convertExpr(r.Value)
	}

	return result
}

func (c *Converter) convertIfStmt(i *IfStmt) *ast.IfStmt {
	result := &ast.IfStmt{
		StartPos: c.convertPos(i.Pos),
	}

	if i.Condition != nil {
		result.Condition = c.convertExpr(i.Condition)
	}
	if i.Then != nil {
		result.Then = c.convertStmtBlock(i.Then)
	}

	// Handle else-if chain
	var current *ast.IfStmt = result
	for _, elif := range i.ElseIfs {
		elseIf := &ast.IfStmt{
			StartPos: c.convertPos(elif.Pos),
		}
		if elif.Condition != nil {
			elseIf.Condition = c.convertExpr(elif.Condition)
		}
		if elif.Then != nil {
			elseIf.Then = c.convertStmtBlock(elif.Then)
		}
		current.Else = elseIf
		current = elseIf
	}

	// Handle final else
	if i.Else != nil {
		current.Else = c.convertStmtBlock(i.Else)
	}

	return result
}

func (c *Converter) convertWhileStmt(w *WhileStmt) *ast.WhileStmt {
	result := &ast.WhileStmt{
		StartPos: c.convertPos(w.Pos),
	}

	if w.Condition != nil {
		result.Condition = c.convertExpr(w.Condition)
	}
	if w.Body != nil {
		result.Body = c.convertStmtBlock(w.Body)
	}

	return result
}

func (c *Converter) convertForStmt(f *ForStmt) *ast.ForStmt {
	result := &ast.ForStmt{
		Iterator: f.Variable,
		StartPos: c.convertPos(f.Pos),
	}

	// Create range expression from start..end
	if f.Start != nil && f.End != nil {
		result.Range = &ast.BinaryExpr{
			Left:     c.convertExpr(f.Start),
			Operator: "..",
			Right:    c.convertExpr(f.End),
			StartPos: c.convertPos(f.Pos),
		}
	} else if f.Start != nil {
		// Just iterating over a collection
		result.Range = c.convertExpr(f.Start)
	}

	if f.Body != nil {
		result.Body = c.convertStmtBlock(f.Body)
	}

	return result
}

func (c *Converter) convertLoopStmt(l *LoopStmt) ast.Statement {
	// Infinite loop
	if l.Array == nil {
		return &ast.InfiniteLoopStmt{
			Body:     c.convertStmtBlock(l.Body),
			StartPos: c.convertPos(l.Pos),
		}
	}

	// Loop with array iteration
	result := &ast.LoopStmt{
		Table:    c.convertExpr(l.Array),
		StartPos: c.convertPos(l.Pos),
	}

	if l.Into != nil {
		result.Mode = ast.LoopInto
		result.Iterator = *l.Into
	} else if l.Ref != nil {
		result.Mode = ast.LoopRefTo
		result.Iterator = *l.Ref
	}

	if l.Body != nil {
		result.Body = c.convertStmtBlock(l.Body)
	}

	return result
}

func (c *Converter) convertExprStmt(e *ExprStmt) ast.Statement {
	expr := c.convertExpr(e.Expr)

	// Check if this is an assignment
	if e.Op != nil && e.Value != nil {
		op := *e.Op
		// Compound assignment - convert to binary + assign
		if op != "=" {
			// e.g., += becomes target = target + value
			binOp := strings.TrimSuffix(op, "=")
			return &ast.AssignStmt{
				Target: expr,
				Value: &ast.BinaryExpr{
					Left:     expr,
					Operator: binOp,
					Right:    c.convertExpr(e.Value),
				},
				StartPos: c.convertPos(e.Pos),
			}
		}
		return &ast.AssignStmt{
			Target:   expr,
			Value:    c.convertExpr(e.Value),
			StartPos: c.convertPos(e.Pos),
		}
	}

	return &ast.ExpressionStmt{
		Expression: expr,
		StartPos:   c.convertPos(e.Pos),
	}
}

func (c *Converter) convertExpr(e *Expression) ast.Expression {
	if e == nil {
		return nil
	}
	return c.convertTernaryExpr(e.Ternary)
}

func (c *Converter) convertTernaryExpr(t *TernaryExpr) ast.Expression {
	if t == nil {
		return nil
	}
	// Ternary removed - just pass through to null coalesce
	// Use if-expressions instead: let x = if cond { a } else { b };
	return c.convertNullCoalesceExpr(t.Condition)
}

func (c *Converter) convertNullCoalesceExpr(n *NullCoalesceExpr) ast.Expression {
	if n == nil {
		return nil
	}

	result := c.convertOrExpr(n.Left)

	for _, tail := range n.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: "??",
			Right:    c.convertOrExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertOrExpr(o *OrExpr) ast.Expression {
	if o == nil {
		return nil
	}

	result := c.convertAndExpr(o.Left)

	for _, tail := range o.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: "||",
			Right:    c.convertAndExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertAndExpr(a *AndExpr) ast.Expression {
	if a == nil {
		return nil
	}

	result := c.convertBitOrExpr(a.Left)

	for _, tail := range a.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: "&&",
			Right:    c.convertBitOrExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertBitOrExpr(b *BitOrExpr) ast.Expression {
	if b == nil {
		return nil
	}

	result := c.convertBitXorExpr(b.Left)

	for _, tail := range b.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: "|",
			Right:    c.convertBitXorExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertBitXorExpr(b *BitXorExpr) ast.Expression {
	if b == nil {
		return nil
	}

	result := c.convertBitAndExpr(b.Left)

	for _, tail := range b.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: "^",
			Right:    c.convertBitAndExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertBitAndExpr(b *BitAndExpr) ast.Expression {
	if b == nil {
		return nil
	}

	result := c.convertEqualityExpr(b.Left)

	for _, tail := range b.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: "&",
			Right:    c.convertEqualityExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertEqualityExpr(e *EqualityExpr) ast.Expression {
	if e == nil {
		return nil
	}

	result := c.convertComparisonExpr(e.Left)

	for _, tail := range e.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: tail.Op,
			Right:    c.convertComparisonExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertComparisonExpr(e *ComparisonExpr) ast.Expression {
	if e == nil {
		return nil
	}

	result := c.convertShiftExpr(e.Left)

	for _, tail := range e.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: tail.Op,
			Right:    c.convertShiftExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertShiftExpr(s *ShiftExpr) ast.Expression {
	if s == nil {
		return nil
	}

	result := c.convertAddExpr(s.Left)

	for _, tail := range s.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: tail.Op,
			Right:    c.convertAddExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertAddExpr(a *AddExpr) ast.Expression {
	if a == nil {
		return nil
	}

	result := c.convertMulExpr(a.Left)

	for _, tail := range a.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: tail.Op,
			Right:    c.convertMulExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertMulExpr(m *MulExpr) ast.Expression {
	if m == nil {
		return nil
	}

	result := c.convertUnaryExpr(m.Left)

	for _, tail := range m.Right {
		result = &ast.BinaryExpr{
			Left:     result,
			Operator: tail.Op,
			Right:    c.convertUnaryExpr(tail.Right),
		}
	}

	return result
}

func (c *Converter) convertUnaryExpr(u *UnaryExpr) ast.Expression {
	if u == nil {
		return nil
	}

	if u.Op != "" && u.Operand != nil {
		return &ast.UnaryExpr{
			Operator: u.Op,
			Operand:  c.convertUnaryExpr(u.Operand),
			StartPos: c.convertPos(u.Pos),
		}
	}

	return c.convertPostfixExpr(u.Postfix)
}

func (c *Converter) convertPostfixExpr(p *PostfixExpr) ast.Expression {
	if p == nil {
		return nil
	}

	result := c.convertPrimaryExpr(p.Primary)

	for _, suffix := range p.Suffixes {
		switch {
		case suffix.Call != nil:
			var args []ast.Expression
			for _, arg := range suffix.Call.Args {
				args = append(args, c.convertExpr(arg))
			}
			result = &ast.CallExpr{
				Function:  result,
				Arguments: args,
				StartPos:  c.convertPos(suffix.Pos),
			}

		case suffix.Index != nil:
			result = &ast.IndexExpr{
				Array:    result,
				Index:    c.convertExpr(suffix.Index.Index),
				StartPos: c.convertPos(suffix.Pos),
			}

		case suffix.Field != nil:
			result = &ast.FieldExpr{
				Object:   result,
				Field:    suffix.Field.Field,
				StartPos: c.convertPos(suffix.Pos),
			}

		case suffix.Cast != nil:
			result = &ast.CastExpr{
				Expr:       result,
				TargetType: c.convertTypeRef(suffix.Cast.Type),
				StartPos:   c.convertPos(suffix.Pos),
			}

		case suffix.Struct != nil:
			// Type { fields } - get type name from result
			typeName := ""
			if ident, ok := result.(*ast.Identifier); ok {
				typeName = ident.Name
			}
			var fields []*ast.FieldInit
			for _, f := range suffix.Struct.Fields {
				fields = append(fields, &ast.FieldInit{
					Name:  f.Name,
					Value: c.convertExpr(f.Value),
				})
			}
			result = &ast.StructLiteral{
				TypeName: typeName,
				Fields:   fields,
				StartPos: c.convertPos(suffix.Pos),
			}

		case suffix.Try:
			// ? error propagation operator
			result = &ast.TryExpr{
				Expression: result,
				StartPos:   c.convertPos(suffix.Pos),
			}
		}
	}

	// Check if result is an iterator chain and convert it
	if chain := c.tryConvertIteratorChain(result); chain != nil {
		return chain
	}

	return result
}

// isIteratorMethod checks if a method name is an iterator operation
func isIteratorMethod(name string) bool {
	switch name {
	case "iter", "map", "filter", "forEach", "reduce", "collect",
		"take", "skip", "zip", "enumerate", "flatMap", "takeWhile",
		"skipWhile", "peek", "inspect", "sum", "count", "any", "all",
		"find", "first", "last":
		return true
	}
	return false
}

// getIteratorOpType converts method name to IteratorOpType
func getIteratorOpType(name string) ast.IteratorOpType {
	switch name {
	case "map":
		return ast.IterOpMap
	case "filter":
		return ast.IterOpFilter
	case "forEach":
		return ast.IterOpForEach
	case "reduce":
		return ast.IterOpReduce
	case "collect":
		return ast.IterOpCollect
	case "take":
		return ast.IterOpTake
	case "skip":
		return ast.IterOpSkip
	case "zip":
		return ast.IterOpZip
	case "enumerate":
		return ast.IterOpEnumerate
	case "flatMap":
		return ast.IterOpFlatMap
	case "takeWhile":
		return ast.IterOpTakeWhile
	case "skipWhile":
		return ast.IterOpSkipWhile
	case "peek":
		return ast.IterOpPeek
	case "inspect":
		return ast.IterOpInspect
	default:
		return ast.IterOpMap // default
	}
}

// tryConvertIteratorChain checks if an expression is an iterator chain
// and converts it to IteratorChainExpr if so
func (c *Converter) tryConvertIteratorChain(expr ast.Expression) *ast.IteratorChainExpr {
	// Collect the chain of method calls
	var ops []ast.IteratorOp
	current := expr

	for {
		callExpr, ok := current.(*ast.CallExpr)
		if !ok {
			break
		}

		fieldExpr, ok := callExpr.Function.(*ast.FieldExpr)
		if !ok {
			break
		}

		if !isIteratorMethod(fieldExpr.Field) {
			break
		}

		// This is an iterator method call
		var fn ast.Expression
		if len(callExpr.Arguments) > 0 {
			fn = callExpr.Arguments[0]
		}

		op := ast.IteratorOp{
			Type:     getIteratorOpType(fieldExpr.Field),
			Function: fn,
			StartPos: callExpr.Pos(),
			EndPos:   callExpr.End(),
		}

		// Prepend to maintain order (we're walking backwards)
		ops = append([]ast.IteratorOp{op}, ops...)
		current = fieldExpr.Object
	}

	// If we found at least one iterator operation, create the chain
	if len(ops) > 0 {
		return &ast.IteratorChainExpr{
			Source:     current,
			Operations: ops,
			StartPos:   current.Pos(),
			EndPos:     expr.End(),
		}
	}

	return nil
}

func (c *Converter) convertPrimaryExpr(p *PrimaryExpr) ast.Expression {
	if p == nil {
		return nil
	}

	switch {
	case p.Number != nil:
		val, _ := strconv.ParseInt(*p.Number, 10, 64)
		return &ast.NumberLiteral{
			Value:    val,
			StartPos: c.convertPos(p.Pos),
		}

	case p.HexNumber != nil:
		s := strings.TrimPrefix(*p.HexNumber, "0x")
		s = strings.TrimPrefix(s, "0X")
		val, _ := strconv.ParseInt(s, 16, 64)
		return &ast.NumberLiteral{
			Value:    val,
			StartPos: c.convertPos(p.Pos),
		}

	case p.DollarHex != nil:
		// ZX Spectrum style hex: $FE
		s := strings.TrimPrefix(*p.DollarHex, "$")
		val, _ := strconv.ParseInt(s, 16, 64)
		return &ast.NumberLiteral{
			Value:    val,
			StartPos: c.convertPos(p.Pos),
		}

	case p.BinNumber != nil:
		s := strings.TrimPrefix(*p.BinNumber, "0b")
		s = strings.TrimPrefix(s, "0B")
		val, _ := strconv.ParseInt(s, 2, 64)
		return &ast.NumberLiteral{
			Value:    val,
			StartPos: c.convertPos(p.Pos),
		}

	case p.String != nil:
		// Remove quotes
		s := *p.String
		if len(s) >= 2 {
			s = s[1 : len(s)-1]
		}
		return &ast.StringLiteral{
			Value:    s,
			StartPos: c.convertPos(p.Pos),
		}

	case p.Char != nil:
		s := *p.Char
		if len(s) >= 2 {
			s = s[1 : len(s)-1]
		}
		// Parse char as number
		var val int64
		if len(s) > 0 {
			val = int64(s[0])
		}
		return &ast.NumberLiteral{
			Value:    val,
			StartPos: c.convertPos(p.Pos),
		}

	case p.RawString != nil:
		s := *p.RawString
		if len(s) >= 2 {
			s = s[1 : len(s)-1]
		}
		return &ast.StringLiteral{
			Value:    s,
			StartPos: c.convertPos(p.Pos),
		}

	case p.True:
		return &ast.BooleanLiteral{
			Value:    true,
			StartPos: c.convertPos(p.Pos),
		}

	case p.False:
		return &ast.BooleanLiteral{
			Value:    false,
			StartPos: c.convertPos(p.Pos),
		}

	case p.Nil:
		return &ast.Identifier{
			Name:     "nil",
			StartPos: c.convertPos(p.Pos),
		}

	case p.ScopedId != nil:
		return &ast.FieldExpr{
			Object: &ast.Identifier{
				Name:     p.ScopedId.Scope,
				StartPos: c.convertPos(p.Pos),
			},
			Field:         p.ScopedId.Member,
			IsDoubleColon: true,
			StartPos:      c.convertPos(p.Pos),
		}

	case p.Ident != nil:
		return &ast.Identifier{
			Name:     *p.Ident,
			StartPos: c.convertPos(p.Pos),
		}

	case p.Paren != nil:
		return c.convertExpr(p.Paren)

	case p.Array != nil:
		var elements []ast.Expression
		for _, elem := range p.Array.Elements {
			elements = append(elements, c.convertExpr(elem))
		}
		return &ast.ArrayInitializer{
			Elements: elements,
			StartPos: c.convertPos(p.Pos),
		}

	case p.Lambda != nil:
		return c.convertLambda(p.Lambda)

	case p.Case != nil:
		return c.convertCaseExpr(p.Case)

	case p.Block != nil:
		// Block expression - returns the block (used in lambdas, etc.)
		return c.convertStmtBlock(p.Block.Body)

	case p.Meta != nil:
		return c.convertMetaExpr(p.Meta)
	}

	return nil
}

func (c *Converter) convertLambda(l *LambdaExpr) ast.Expression {
	result := &ast.LambdaExpr{
		StartPos: c.convertPos(l.Pos),
	}

	for _, p := range l.Params {
		param := &ast.LambdaParam{
			Name: p.Name,
		}
		if p.Type != nil {
			param.Type = c.convertTypeRef(p.Type)
		}
		result.Params = append(result.Params, param)
	}

	if l.ReturnType != nil {
		result.ReturnType = c.convertTypeRef(l.ReturnType)
	}

	if l.Body != nil {
		result.Body = c.convertExpr(l.Body)
	}

	return result
}

func (c *Converter) convertCaseExpr(ce *CaseExpr) ast.Expression {
	result := &ast.CaseExpr{
		Value:    c.convertExpr(ce.Value),
		StartPos: c.convertPos(ce.Pos),
	}

	for _, arm := range ce.Arms {
		result.Arms = append(result.Arms, ast.CaseArm{
			Pattern:  c.convertPattern(arm.Pattern),
			Body:     c.convertExpr(arm.Result),
			StartPos: c.convertPos(arm.Pos),
		})
	}

	return result
}

func (c *Converter) convertPattern(expr *Expression) ast.Pattern {
	if expr == nil {
		return nil
	}

	// Convert expression to appropriate pattern type
	e := c.convertExpr(expr)

	switch v := e.(type) {
	case *ast.Identifier:
		// Could be wildcard _, enum variant, or binding
		if v.Name == "_" {
			return &ast.WildcardPattern{StartPos: v.StartPos}
		}
		return &ast.IdentifierPattern{Name: v.Name, StartPos: v.StartPos}

	case *ast.FieldExpr:
		// Enum variant like State.IDLE or State::IDLE
		return &ast.IdentifierPattern{
			Name:     v.Object.(*ast.Identifier).Name + "." + v.Field,
			StartPos: v.StartPos,
		}

	default:
		// Treat as literal pattern
		return &ast.LiteralPattern{Value: e, StartPos: e.Pos()}
	}
}

func (c *Converter) convertMetaExpr(m *MetaExpr) ast.Expression {
	result := &ast.MetafunctionCall{
		Name:     m.Name,
		StartPos: c.convertPos(m.Pos),
	}

	for _, arg := range m.Args {
		result.Arguments = append(result.Arguments, c.convertExpr(arg))
	}

	return result
}

func (c *Converter) convertTypeRef(t *TypeRef) ast.Type {
	if t == nil {
		return nil
	}

	var result ast.Type

	// Handle array types
	if t.Array != nil {
		arr := &ast.ArrayType{
			StartPos: c.convertPos(t.Pos),
		}
		// C-style: [N]Type
		if t.Array.Size != nil {
			val, _ := strconv.ParseInt(*t.Array.Size, 10, 64)
			arr.Size = &ast.NumberLiteral{Value: val}
		}
		if t.Array.Element != nil {
			arr.ElementType = c.convertTypeRef(t.Array.Element)
		}
		// Rust-style: [Type; N]
		if t.Array.RustSize != nil {
			val, _ := strconv.ParseInt(*t.Array.RustSize, 10, 64)
			arr.Size = &ast.NumberLiteral{Value: val}
		}
		if t.Array.RustElement != nil {
			arr.ElementType = c.convertTypeRef(t.Array.RustElement)
		}
		// Empty array: []Type
		if t.Array.EmptyElement != nil {
			arr.ElementType = c.convertTypeRef(t.Array.EmptyElement)
		}
		result = arr
	} else if t.Function != nil {
		// Function type: fn(params) -> ReturnType
		ft := &ast.FunctionType{
			StartPos: c.convertPos(t.Pos),
		}
		for _, p := range t.Function.Params {
			ft.ParamTypes = append(ft.ParamTypes, c.convertTypeRef(p))
		}
		if t.Function.ReturnType != nil {
			ft.ReturnType = c.convertTypeRef(t.Function.ReturnType)
		}
		result = ft
	} else if t.Name != nil {
		name := *t.Name
		if IsPrimitiveType(name) {
			result = &ast.PrimitiveType{
				Name:     name,
				StartPos: c.convertPos(t.Pos),
			}
		} else {
			result = &ast.TypeIdentifier{
				Name:     name,
				StartPos: c.convertPos(t.Pos),
			}
		}
	}

	// Handle pointer
	if t.Pointer && result != nil {
		result = &ast.PointerType{
			BaseType: result,
			StartPos: c.convertPos(t.Pos),
		}
	}

	return result
}

func (c *Converter) typeRefToString(t *TypeRef) string {
	if t == nil {
		return ""
	}
	if t.Name != nil {
		return *t.Name
	}
	return ""
}
