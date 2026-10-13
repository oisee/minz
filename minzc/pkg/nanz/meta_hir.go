package nanz

import (
	"fmt"

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
		if _, ok := l.(*hir.VarRefExpr); !ok {
			return nil, fmt.Errorf("structured emit: assignment needs variable")
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
func validMetaIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
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
