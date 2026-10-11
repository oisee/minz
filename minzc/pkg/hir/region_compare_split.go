package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"sort"
	"strings"
)

// ValidateRegionCompareCompiledChoices independently removes supported tail
// splits from a logical view. It neither runs nor predicts SplitHighPressure.
// Every generated helper must contribute an unchanged ordered source suffix;
// its arguments must forward exactly the lexical free bindings of that suffix.
func ValidateRegionCompareCompiledChoices(source, actual *Module, choices map[string]bool) ([]RegionCompareProof, error) {
	view, e := normalizeRegionCompareSplits(source, actual)
	if e != nil {
		return nil, e
	}
	return ValidateRegionCompareChoices(source, view, choices)
}

func normalizeRegionCompareSplits(source, actual *Module) (*Module, error) {
	if source == nil || actual == nil {
		return nil, fmt.Errorf("region split: nil module")
	}
	if e := Verify(source); e != nil {
		return nil, e
	}
	if e := Verify(actual); e != nil {
		return nil, e
	}
	view := *actual
	view.Funcs = nil
	used := map[string]bool{}
	sourceNames := map[string]bool{}
	for _, f := range source.Funcs {
		sourceNames[f.Name] = true
	}
	var expand func(*Func, *Func, int) (*Func, error)
	expand = func(original, current *Func, depth int) (*Func, error) {
		if depth > 4 || current == nil {
			return nil, fmt.Errorf("region split: missing/deep helper")
		}
		copy := *current
		if current.Body == nil || len(current.Body.Body) == 0 {
			return &copy, nil
		}
		last := current.Body.Body[len(current.Body.Body)-1]
		var call *CallExpr
		returnCall := false
		switch x := last.(type) {
		case *ReturnStmt:
			if len(x.Vals) == 0 {
				call, _ = x.Val.(*CallExpr)
				returnCall = true
			}
		case *ExprStmt:
			call, _ = x.Expr.(*CallExpr)
		}
		if call == nil || !strings.HasPrefix(call.Fn, current.Name+"$split_") {
			return &copy, nil
		}
		cut := len(current.Body.Body) - 2
		expectedName := fmt.Sprintf("%s$split_%d", current.Name, cut)
		if call.Fn != expectedName || sourceNames[call.Fn] || used[call.Fn] {
			return nil, fmt.Errorf("region split: helper name/collision")
		}
		helper := actual.FuncByName(call.Fn)
		if helper == nil {
			return nil, fmt.Errorf("region split: missing helper")
		}
		used[call.Fn] = true
		expected := Func{Name: helper.Name, Params: helper.Params, RetTy: original.RetTy, RetTys: append([]mir2.Ty(nil), original.RetTys...), Body: helper.Body}
		if !reflect.DeepEqual(*helper, expected) {
			return nil, fmt.Errorf("region split: helper metadata")
		}
		suffix, e := expand(original, helper, depth+1)
		if e != nil {
			return nil, e
		}
		if suffix.Body == nil {
			return nil, fmt.Errorf("region split: empty suffix")
		}
		// Compare all free local inputs independently of the splitter's liveness
		// collector. Global/function references are not helper parameters.
		free := regionSplitFree(suffix.Body, forwardCopyReserved(source))
		names := []string{}
		for n := range free {
			names = append(names, n)
		}
		sort.Strings(names)
		bindings := map[string]mir2.Ty{}
		unsupported := map[string]bool{}
		for _, p := range current.Params {
			bindings[p.Name] = p.Ty
		}
		for _, s := range current.Body.Body[:len(current.Body.Body)-1] {
			switch x := s.(type) {
			case *VarDeclStmt:
				bindings[x.Name] = x.Ty
				if x.ArrayLen != 0 || x.At != nil {
					unsupported[x.Name] = true
				}
				if ref, ok := x.Init.(*VarRefExpr); ok && (source.FuncByName(ref.Name) != nil || unsupported[ref.Name]) {
					unsupported[x.Name] = true
				}
			case *TupleLetStmt:
				for j, n := range x.Names {
					if n != "_" {
						bindings[n] = x.Tys[j]
					}
				}
			}
		}
		if len(names) != len(helper.Params) || len(names) != len(call.Args) {
			return nil, fmt.Errorf("region split: incomplete/free helper interface")
		}
		for j, n := range names {
			p := helper.Params[j]
			if unsupported[n] || bindings[n] == nil || p.Name != n || p.SMC || p.RegClass != 0 || !reflect.DeepEqual(p.Ty, bindings[n]) || !graceRef(call.Args[j], n, bindings[n]) {
				return nil, fmt.Errorf("region split: argument order/type/binding")
			}
		}
		retTy := original.RetTy
		if len(original.RetTys) == 1 {
			retTy = original.RetTys[0]
		}
		if !reflect.DeepEqual(call.Ty, retTy) {
			return nil, fmt.Errorf("region split: call result type")
		}
		joined := append([]Stmt(nil), current.Body.Body[:len(current.Body.Body)-1]...)
		joined = append(joined, suffix.Body.Body...)
		copy.Body = Blk(joined...)
		returnsValue := original.RetTy != mir2.TyVoid || len(original.RetTys) > 0
		if returnCall != returnsValue || (returnsValue && !regionSplitValueExit(suffix.Body)) {
			return nil, fmt.Errorf("region split: tail call result discarded/invented")
		}
		return &copy, nil
	}
	for _, original := range source.Funcs {
		current := actual.FuncByName(original.Name)
		if current == nil {
			return nil, fmt.Errorf("region split: original function missing")
		}
		normalized, e := expand(original, current, 0)
		if e != nil {
			return nil, e
		}
		view.Funcs = append(view.Funcs, normalized)
	}
	for _, f := range actual.Funcs {
		if !sourceNames[f.Name] && !used[f.Name] {
			return nil, fmt.Errorf("region split: unexpected helper/function")
		}
	}
	return &view, nil
}

func regionSplitFree(root *Block, reserved map[string]bool) map[string]bool {
	free := map[string]bool{}
	var expr func(Expr, map[string]bool)
	expr = func(e Expr, bound map[string]bool) {
		walkExpr(e, func(node Expr) {
			if ref, ok := node.(*VarRefExpr); ok && !bound[ref.Name] && !reserved[ref.Name] {
				free[ref.Name] = true
			}
		})
	}
	var block func(*Block, map[string]bool)
	block = func(b *Block, outer map[string]bool) {
		if b == nil {
			return
		}
		bound := map[string]bool{}
		for n, v := range outer {
			bound[n] = v
		}
		for _, s := range b.Body {
			switch x := s.(type) {
			case *VarDeclStmt:
				expr(x.Init, bound)
				for _, e := range x.Initial {
					expr(e, bound)
				}
				bound[x.Name] = true
			case *TupleLetStmt:
				expr(x.Call, bound)
				for _, n := range x.Names {
					bound[n] = true
				}
			case *Block:
				block(x, bound)
			case *IfStmt:
				expr(x.Cond, bound)
				block(x.Then, bound)
				block(x.Else, bound)
			case *WhileStmt:
				expr(x.Cond, bound)
				block(x.Body, bound)
			case *ForRangeStmt:
				expr(x.Start, bound)
				expr(x.End, bound)
				nested := map[string]bool{}
				for n, v := range bound {
					nested[n] = v
				}
				nested[x.Var] = true
				block(x.Body, nested)
			case *ForEachStmt:
				expr(x.Ptr, bound)
				expr(x.Start, bound)
				expr(x.Len, bound)
				nested := map[string]bool{}
				for n, v := range bound {
					nested[n] = v
				}
				nested[x.Var] = true
				block(x.Body, nested)
			case *SwitchStmt:
				expr(x.Val, bound)
				for _, c := range x.Cases {
					block(c.Body, bound)
				}
				block(x.Default, bound)
			default:
				visitStmtExprs(s, func(e Expr) {
					if ref, ok := e.(*VarRefExpr); ok && !bound[ref.Name] && !reserved[ref.Name] {
						free[ref.Name] = true
					}
				})
			}
		}
	}
	block(root, map[string]bool{})
	return free
}

// Restrict nonvoid forwarding to suffixes which end in an explicit value
// return on every path. An implicit fallthrough has no value to forward.
func regionSplitValueExit(b *Block) bool {
	if b == nil || len(b.Body) == 0 {
		return false
	}
	switch x := b.Body[len(b.Body)-1].(type) {
	case *ReturnStmt:
		return x.Val != nil || len(x.Vals) != 0
	case *IfStmt:
		return regionSplitValueExit(x.Then) && regionSplitValueExit(x.Else)
	case *Block:
		return regionSplitValueExit(x)
	}
	return false
}
