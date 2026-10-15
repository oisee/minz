package nanz

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Handles are scoped to one metafunction invocation. They never reach target code.
type metaHIRBuilder struct {
	nodes  []any
	output []hir.Stmt
}

func (b *metaHIRBuilder) add(n any) ([]mir2.Value, error) {
	if len(b.nodes) == 65535 {
		return nil, fmt.Errorf("structured emit: node handle space exhausted")
	}
	b.nodes = append(b.nodes, n)
	return []mir2.Value{{I: int64(len(b.nodes))}}, nil
}
func (b *metaHIRBuilder) node(h int64) (any, error) {
	if h < 1 || h > int64(len(b.nodes)) {
		return nil, fmt.Errorf("structured emit: invalid node handle %d", h)
	}
	return b.nodes[h-1], nil
}
func (b *metaHIRBuilder) expr(h int64) (hir.Expr, error) {
	n, e := b.node(h)
	if e != nil {
		return nil, e
	}
	x, ok := n.(hir.Expr)
	if !ok {
		return nil, fmt.Errorf("structured emit: expected expression")
	}
	return x, nil
}
func metaHIRType(s string) (mir2.Ty, error) {
	switch s {
	case "u8":
		return mir2.TyU8, nil
	case "u16":
		return mir2.TyU16, nil
	case "ptr":
		return mir2.TyPtr, nil
	case "bool":
		return mir2.TyBool, nil
	}
	return nil, fmt.Errorf("structured emit: unsupported type %q", s)
}
func (mr *metaRuntime) registerHIRHosts(vm *mir2.VM) {
	b := &mr.structured
	host := func(name string, arity int, f func([]mir2.Value) ([]mir2.Value, error)) {
		vm.Hosts[name] = func(a []mir2.Value) ([]mir2.Value, error) {
			if len(a) != arity {
				return nil, fmt.Errorf("structured emit: %s expects %d arguments", name, arity)
			}
			return f(a)
		}
	}
	host("hir_int", 2, func(a []mir2.Value) ([]mir2.Value, error) {
		ty, e := metaHIRType(mr.readCString(a[1].I))
		if e != nil {
			return nil, e
		}
		v := a[0].I
		if v < 0 || (ty == mir2.TyU8 && v > 255) || ((ty == mir2.TyU16 || ty == mir2.TyPtr) && v > 65535) || (ty == mir2.TyBool && v > 1) {
			return nil, fmt.Errorf("structured emit: integer outside %s", ty)
		}
		return b.add(&hir.IntLitExpr{Val: v, Ty: ty})
	})
	host("hir_ref", 2, func(a []mir2.Value) ([]mir2.Value, error) {
		ty, e := metaHIRType(mr.readCString(a[1].I))
		if e != nil {
			return nil, e
		}
		name := mr.readCString(a[0].I)
		if !validMetaIdentifier(name) {
			return nil, fmt.Errorf("structured emit: invalid identifier %q", name)
		}
		return b.add(&hir.VarRefExpr{Name: name, Ty: ty})
	})
	// Unlike hir_ref, which may refer to a local declared by the builder,
	// this resolves an existing caller binding before any target code is built.
	host("hir_caller_ref", 2, func(a []mir2.Value) ([]mir2.Value, error) {
		ty, e := metaHIRType(mr.readCString(a[1].I))
		if e != nil {
			return nil, e
		}
		name := mr.readCString(a[0].I)
		if !validMetaIdentifier(name) {
			return nil, fmt.Errorf("structured emit: invalid identifier %q", name)
		}
		declared := mr.callerTypes[name]
		if declared == nil {
			return nil, fmt.Errorf("structured emit: unknown caller variable %q", name)
		}
		if declared.String() != ty.String() {
			return nil, fmt.Errorf("structured emit: caller variable %s type %s, requested %s", name, declared, ty)
		}
		return b.add(&hir.VarRefExpr{Name: metaCallerPrefix + name, Ty: declared})
	})
	// Resolve a declared caller byte array and retain its type and ownership.
	// Dynamic indices have the same semantics as ordinary Nanz indexing.
	host("hir_caller_index", 2, func(a []mir2.Value) ([]mir2.Value, error) {
		name := mr.readCString(a[0].I)
		if !validMetaIdentifier(name) {
			return nil, fmt.Errorf("structured emit: invalid identifier %q", name)
		}
		declared := mr.callerTypes[name]
		array, ok := declared.(*mir2.ArrayTy)
		if !ok || array.Elem != mir2.TyU8 || array.Len <= 0 {
			return nil, fmt.Errorf("structured emit: caller %s needs byte array", name)
		}
		index, err := b.expr(a[1].I)
		if err != nil {
			return nil, err
		}
		if index.ExprTy() != mir2.TyU8 && index.ExprTy() != mir2.TyU16 {
			return nil, fmt.Errorf("structured emit: array index needs u8 or u16")
		}
		if literal, ok := index.(*hir.IntLitExpr); ok && (literal.Val < 0 || literal.Val >= int64(array.Len)) {
			return nil, fmt.Errorf("structured emit: constant array index out of bounds")
		}
		return b.add(&hir.IndexExpr{Base: &hir.VarRefExpr{Name: metaCallerPrefix + name, Ty: declared}, Idx: index, ElemTy: mir2.TyU8})
	})
	host("hir_binary", 3, func(a []mir2.Value) ([]mir2.Value, error) {
		l, e := b.expr(a[1].I)
		if e != nil {
			return nil, e
		}
		r, e := b.expr(a[2].I)
		if e != nil {
			return nil, e
		}
		op := mr.readCString(a[0].I)
		ty := l.ExprTy()
		switch op {
		case "+", "-":
			if ty == mir2.TyBool || (ty == mir2.TyPtr && r.ExprTy() != mir2.TyU16 && r.ExprTy() != mir2.TyU8) || (ty != mir2.TyPtr && ty != r.ExprTy()) {
				return nil, fmt.Errorf("structured emit: incompatible operands for %s", op)
			}
		case "&", "|", "^":
			if (ty != mir2.TyU8 && ty != mir2.TyU16) || ty != r.ExprTy() {
				return nil, fmt.Errorf("structured emit: bitwise operands need matching integer types")
			}
		case "!=", "==":
			if ty != r.ExprTy() {
				return nil, fmt.Errorf("structured emit: incompatible comparison")
			}
			ty = mir2.TyBool
		default:
			return nil, fmt.Errorf("structured emit: unsupported operator %q", op)
		}
		return b.add(&hir.BinExpr{Op: op, L: l, R: r, Ty: ty})
	})
	host("hir_block", 0, func(a []mir2.Value) ([]mir2.Value, error) { return b.add(&hir.Block{}) })
	host("hir_append", 2, func(a []mir2.Value) ([]mir2.Value, error) {
		n, e := b.node(a[0].I)
		if e != nil {
			return nil, e
		}
		block, ok := n.(*hir.Block)
		if !ok {
			return nil, fmt.Errorf("structured emit: expected block")
		}
		n, e = b.node(a[1].I)
		if e != nil {
			return nil, e
		}
		s, ok := n.(hir.Stmt)
		if !ok {
			return nil, fmt.Errorf("structured emit: expected statement")
		}
		if containsMetaBlock(s, block) {
			return nil, fmt.Errorf("structured emit: cyclic block")
		}
		block.Body = append(block.Body, s)
		return nil, nil
	})
	host("hir_var", 3, func(a []mir2.Value) ([]mir2.Value, error) {
		name := mr.readCString(a[0].I)
		if !validMetaIdentifier(name) {
			return nil, fmt.Errorf("structured emit: invalid identifier")
		}
		ty, e := metaHIRType(mr.readCString(a[1].I))
		if e != nil {
			return nil, e
		}
		x, e := b.expr(a[2].I)
		if e != nil {
			return nil, e
		}
		if x.ExprTy() != ty {
			return nil, fmt.Errorf("structured emit: initializer type mismatch")
		}
		return b.add(&hir.VarDeclStmt{Name: name, Ty: ty, Init: x})
	})
	host("hir_assign", 2, func(a []mir2.Value) ([]mir2.Value, error) {
		l, e := b.expr(a[0].I)
		if e != nil {
			return nil, e
		}
		switch l.(type) {
		case *hir.VarRefExpr, *hir.IndexExpr:
		default:
			return nil, fmt.Errorf("structured emit: assignment needs variable or array element")
		}
		r, e := b.expr(a[1].I)
		if e != nil {
			return nil, e
		}
		if l.ExprTy() != r.ExprTy() {
			return nil, fmt.Errorf("structured emit: assignment type mismatch")
		}
		return b.add(&hir.AssignStmt{Target: l, Val: r})
	})
	host("hir_load", 1, func(a []mir2.Value) ([]mir2.Value, error) {
		ptr, err := b.expr(a[0].I)
		if err != nil {
			return nil, err
		}
		if ptr.ExprTy() != mir2.TyPtr {
			return nil, fmt.Errorf("structured emit: load needs byte pointer")
		}
		return b.add(&hir.LoadExpr{Ptr: ptr, Ty: mir2.TyU8})
	})

	host("hir_store", 2, func(a []mir2.Value) ([]mir2.Value, error) {
		ptr, e := b.expr(a[0].I)
		if e != nil {
			return nil, e
		}
		v, e := b.expr(a[1].I)
		if e != nil {
			return nil, e
		}
		if ptr.ExprTy() != mir2.TyPtr || v.ExprTy() != mir2.TyU8 {
			return nil, fmt.Errorf("structured emit: store needs byte pointer and u8 value")
		}
		return b.add(&hir.AssignStmt{Target: &hir.DerefExpr{Ptr: ptr, Ty: mir2.TyU8}, Val: v})
	})
	host("hir_while", 2, func(a []mir2.Value) ([]mir2.Value, error) {
		cond, e := b.expr(a[0].I)
		if e != nil {
			return nil, e
		}
		n, e := b.node(a[1].I)
		if e != nil {
			return nil, e
		}
		body, ok := n.(*hir.Block)
		if !ok || cond.ExprTy() != mir2.TyBool {
			return nil, fmt.Errorf("structured emit: while needs boolean and block")
		}
		return b.add(&hir.WhileStmt{Cond: cond, Body: body})
	})
	host("hir_if", 3, func(a []mir2.Value) ([]mir2.Value, error) {
		cond, err := b.expr(a[0].I)
		if err != nil {
			return nil, err
		}
		branches := [2]*hir.Block{}
		for i := range branches {
			n, err := b.node(a[i+1].I)
			if err != nil {
				return nil, err
			}
			block, ok := n.(*hir.Block)
			if !ok {
				return nil, fmt.Errorf("structured emit: if needs boolean and two blocks")
			}
			branches[i] = block
		}
		if cond.ExprTy() != mir2.TyBool {
			return nil, fmt.Errorf("structured emit: if needs boolean and two blocks")
		}
		return b.add(&hir.IfStmt{Cond: cond, Then: branches[0], Else: branches[1]})
	})
	host("emit_hir", 1, func(a []mir2.Value) ([]mir2.Value, error) {
		n, e := b.node(a[0].I)
		if e != nil {
			return nil, e
		}
		s, ok := n.(hir.Stmt)
		if !ok {
			return nil, fmt.Errorf("structured emit: expected statement")
		}
		b.output = append(b.output, snapshotMetaStmt(s))
		return nil, nil
	})
}

// The marker is internal to builder snapshots. Neither it nor the private
// generated names can be written as a Nanz identifier. Resolve it only after
// renaming generated bindings, so explicit caller references cannot be captured.
const metaCallerPrefix = "\x00caller:"

func (p *parser) spliceMetaHIR(nodes []hir.Stmt) *hir.Block {
	block := &hir.Block{Body: nodes}
	p.renameMetaStmt(block, nil)
	return block
}

func (p *parser) recordMetaOrigins(stmt hir.Stmt, origin hir.SourceLocation) {
	p.module.SourceLocations[stmt] = origin
	switch n := stmt.(type) {
	case *hir.Block:
		for _, child := range n.Body {
			p.recordMetaOrigins(child, origin)
		}
	case *hir.IfStmt:
		p.recordMetaOrigins(n.Then, origin)
		p.recordMetaOrigins(n.Else, origin)
	case *hir.WhileStmt:
		p.recordMetaOrigins(n.Body, origin)
	}
}

func (p *parser) renameMetaStmt(stmt hir.Stmt, scope map[string]string) {
	switch n := stmt.(type) {
	case *hir.Block:
		local := make(map[string]string, len(scope))
		for name, binding := range scope {
			local[name] = binding
		}
		for _, child := range n.Body {
			p.renameMetaStmt(child, local)
		}
	case *hir.VarDeclStmt:
		// An initializer sees the enclosing binding, before this declaration.
		renameMetaExpr(n.Init, scope)
		original := n.Name
		p.metaBindingCount++
		n.Name = fmt.Sprintf("$meta_%d_%s", p.metaBindingCount, original)
		scope[original] = n.Name
	case *hir.AssignStmt:
		renameMetaExpr(n.Target, scope)
		renameMetaExpr(n.Val, scope)
	case *hir.IfStmt:
		renameMetaExpr(n.Cond, scope)
		p.renameMetaStmt(n.Then, scope)
		p.renameMetaStmt(n.Else, scope)
	case *hir.WhileStmt:
		renameMetaExpr(n.Cond, scope)
		p.renameMetaStmt(n.Body, scope)
	default:
		panic("unhandled structured statement hygiene")
	}
}

func renameMetaExpr(expr hir.Expr, scope map[string]string) {
	switch n := expr.(type) {
	case *hir.IntLitExpr:
	case *hir.VarRefExpr:
		if strings.HasPrefix(n.Name, metaCallerPrefix) {
			n.Name = strings.TrimPrefix(n.Name, metaCallerPrefix)
		} else if binding, ok := scope[n.Name]; ok {
			n.Name = binding
		}
	case *hir.BinExpr:
		renameMetaExpr(n.L, scope)
		renameMetaExpr(n.R, scope)
	case *hir.IndexExpr:
		renameMetaExpr(n.Base, scope)
		renameMetaExpr(n.Idx, scope)
	case *hir.LoadExpr:
		renameMetaExpr(n.Ptr, scope)
	case *hir.DerefExpr:
		renameMetaExpr(n.Ptr, scope)
	default:
		panic("unhandled structured expression hygiene")
	}
}
func validMetaIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		valid := c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9'
		if !valid {
			return false
		}
	}
	return true
}
func containsMetaBlock(s hir.Stmt, want *hir.Block) bool {
	switch n := s.(type) {
	case *hir.Block:
		if n == want {
			return true
		}
		for _, x := range n.Body {
			if containsMetaBlock(x, want) {
				return true
			}
		}
	case *hir.IfStmt:
		return containsMetaBlock(n.Then, want) || containsMetaBlock(n.Else, want)
	case *hir.WhileStmt:
		return containsMetaBlock(n.Body, want)
	}
	return false
}

// emit_hir records a snapshot: subsequent builder edits cannot alter emitted code.
// Only node kinds made by this builder can occur here.
func snapshotMetaStmt(s hir.Stmt) hir.Stmt {
	switch n := s.(type) {
	case *hir.Block:
		b := &hir.Block{}
		for _, child := range n.Body {
			b.Body = append(b.Body, snapshotMetaStmt(child))
		}
		return b
	case *hir.VarDeclStmt:
		c := *n
		c.Init = snapshotMetaExpr(n.Init)
		return &c
	case *hir.AssignStmt:
		return &hir.AssignStmt{Target: snapshotMetaExpr(n.Target), Val: snapshotMetaExpr(n.Val)}
	case *hir.IfStmt:
		return &hir.IfStmt{Cond: snapshotMetaExpr(n.Cond), Then: snapshotMetaStmt(n.Then).(*hir.Block), Else: snapshotMetaStmt(n.Else).(*hir.Block)}
	case *hir.WhileStmt:
		return &hir.WhileStmt{Cond: snapshotMetaExpr(n.Cond), Body: snapshotMetaStmt(n.Body).(*hir.Block)}
	default:
		panic("unhandled structured statement")
	}
}
func snapshotMetaExpr(e hir.Expr) hir.Expr {
	switch n := e.(type) {
	case *hir.IntLitExpr:
		c := *n
		return &c
	case *hir.VarRefExpr:
		c := *n
		return &c
	case *hir.BinExpr:
		c := *n
		c.L = snapshotMetaExpr(n.L)
		c.R = snapshotMetaExpr(n.R)
		return &c
	case *hir.IndexExpr:
		c := *n
		c.Base = snapshotMetaExpr(n.Base)
		c.Idx = snapshotMetaExpr(n.Idx)
		return &c
	case *hir.LoadExpr:
		c := *n
		c.Ptr = snapshotMetaExpr(n.Ptr)
		return &c
	case *hir.DerefExpr:
		c := *n
		c.Ptr = snapshotMetaExpr(n.Ptr)
		return &c
	default:
		panic("unhandled structured expression")
	}
}
