package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

// A deliberately narrow LICM proof: exact 16-bit ordinary + of unmodified
// scalar parameters/literals. No loads, calls, div, PtrAdd or address formation
// are moved. Existing truncation is preserved, including at pointer overflow.
type graceHoist struct {
	index int
	loop  *WhileStmt
	expr  *BinExpr
	name  string
}

func findGraceHoist(f *Func, b *Block, globals map[string]bool) *graceHoist {
	params := map[string]bool{}
	used := map[string]bool{}
	for _, p := range f.Params {
		if used[p.Name] {
			return nil
		}
		params[p.Name] = !globals[p.Name]
		used[p.Name] = true
	}
	unsafe := false
	walkGraceBlocks(f.Body, "", func(_ string, bb *Block) {
		for _, s := range bb.Body {
			switch x := s.(type) {
			case *AsmStmt, *GotoStmt, *LabelStmt:
				unsafe = true
			case *VarDeclStmt:
				used[x.Name] = true
				if params[x.Name] {
					unsafe = true
				}
			case *TupleLetStmt:
				for _, n := range x.Names {
					used[n] = true
					if params[n] {
						unsafe = true
					}
				}
			case *ForRangeStmt:
				used[x.Var] = true
				if params[x.Var] {
					unsafe = true
				}
			case *ForEachStmt:
				used[x.Var] = true
				if params[x.Var] {
					unsafe = true
				}
			}
			visitStmtExprs(s, func(e Expr) {
				switch x := e.(type) {
				case *VarRefExpr:
					used[x.Name] = true
				case *LetInExpr:
					used[x.Name] = true
					if params[x.Name] {
						unsafe = true
					}
				case *IntLitExpr, *BoolLitExpr, *ConstPtrExpr, *BinExpr, *UnaryExpr, *CallExpr, *CallIndirectExpr, *FieldExpr, *BitExpr, *LoadExpr, *DerefExpr, *CastExpr, *IndexExpr, *StructLitExpr, *RangeSourceExpr, *CondExpr:
				case *AddrOfExpr:
					used[x.Sym] = true
					if params[x.Sym] {
						unsafe = true
					}
					if x.X != nil {
						walkExpr(x.X, func(e Expr) {
							if v, ok := e.(*VarRefExpr); ok && params[v.Name] {
								unsafe = true
							}
						})
					}
				default:
					unsafe = true
				}
			})
		}
	})
	if unsafe {
		return nil
	}
	for i, s := range b.Body {
		loop, ok := s.(*WhileStmt)
		if !ok || loop.Body == nil {
			continue
		}
		assigned := map[string]bool{}
		loopUnsafe := false
		inspect := func(e Expr) {
			switch e.(type) {
			case *CallExpr, *CallIndirectExpr:
				loopUnsafe = true
			}
		}
		walkExpr(loop.Cond, inspect)
		walkGraceBlocks(loop.Body, "", func(_ string, bb *Block) {
			for _, s := range bb.Body {
				visitStmtExprs(s, inspect)
				switch s.(type) {
				case *WhileStmt, *ForRangeStmt, *ForEachStmt:
					loopUnsafe = true
				}
				if a, ok := s.(*AssignStmt); ok {
					target := a.Target
					for {
						bit, ok := target.(*BitExpr)
						if !ok {
							break
						}
						target = bit.X
					}
					if v, ok := target.(*VarRefExpr); ok {
						assigned[v.Name] = true
					}
				}
			}
		})
		if loopUnsafe {
			continue
		}
		leaf := func(e Expr) bool {
			switch v := e.(type) {
			case *VarRefExpr:
				return params[v.Name] && !assigned[v.Name]
			case *IntLitExpr:
				return true
			}
			return false
		}
		var candidate *BinExpr
		check := func(e Expr) {
			x, ok := e.(*BinExpr)
			if !ok || candidate != nil || x.Op != "+" || (x.Ty != mir2.TyU16 && !mir2.IsPointer(x.Ty)) || !leaf(x.L) || !leaf(x.R) {
				return
			}
			// The same proof covers ordinary u16 scalar addition: unchanged
			// leaves, total arithmetic, and exact existing result truncation.
			candidate = x
		}
		walkExpr(loop.Cond, check)
		walkGraceBlocks(loop.Body, "", func(_ string, bb *Block) {
			for _, s := range bb.Body {
				visitStmtExprs(s, check)
			}
		})
		if candidate != nil {
			name := ""
			for n := 0; ; n++ {
				name = fmt.Sprintf("__grace_base_%d", n)
				if !used[name] && !globals[name] {
					break
				}
			}
			return &graceHoist{i, loop, candidate, name}
		}
	}
	return nil
}

// Rewrite exact expression identity. Keeping individual loads/calls untouched
// matters: even equal addresses do not establish equal memory values.
func replaceGraceExpr(e Expr, target Expr, value Expr) Expr {
	if e == nil {
		return nil
	}
	if e == target {
		return value
	}
	r := func(e Expr) Expr { return replaceGraceExpr(e, target, value) }
	switch x := e.(type) {
	case *BinExpr:
		copy := *x
		x = &copy
		e = x
		x.L = r(x.L)
		x.R = r(x.R)
	case *UnaryExpr:
		copy := *x
		x = &copy
		e = x
		x.X = r(x.X)
	case *CallExpr:
		copy := *x
		x = &copy
		e = x
		x.Args = append([]Expr(nil), x.Args...)
		for i := range x.Args {
			x.Args[i] = r(x.Args[i])
		}
	case *CallIndirectExpr:
		copy := *x
		x = &copy
		e = x
		x.Args = append([]Expr(nil), x.Args...)
		x.FnPtr = r(x.FnPtr)
		for i := range x.Args {
			x.Args[i] = r(x.Args[i])
		}
	case *FieldExpr:
		copy := *x
		x = &copy
		e = x
		x.X = r(x.X)
	case *BitExpr:
		copy := *x
		x = &copy
		e = x
		x.X = r(x.X)
	case *AddrOfExpr:
		copy := *x
		x = &copy
		e = x
		x.X = r(x.X)
	case *LoadExpr:
		copy := *x
		x = &copy
		e = x
		x.Ptr = r(x.Ptr)
	case *DerefExpr:
		copy := *x
		x = &copy
		e = x
		x.Ptr = r(x.Ptr)
	case *CastExpr:
		copy := *x
		x = &copy
		e = x
		x.X = r(x.X)
	case *IndexExpr:
		copy := *x
		x = &copy
		e = x
		x.Base = r(x.Base)
		x.Idx = r(x.Idx)
	case *StructLitExpr:
		copy := *x
		x = &copy
		e = x
		x.Fields = append([]FieldInit(nil), x.Fields...)
		for i := range x.Fields {
			x.Fields[i].Val = r(x.Fields[i].Val)
		}
	case *RangeSourceExpr:
		copy := *x
		x = &copy
		e = x
		x.Lo = r(x.Lo)
		x.Hi = r(x.Hi)
	case *CondExpr:
		copy := *x
		x = &copy
		e = x
		x.Cond = r(x.Cond)
		x.Then = r(x.Then)
		x.Else = r(x.Else)
	case *LetInExpr:
		copy := *x
		x = &copy
		e = x
		x.Init = r(x.Init)
		x.Body = r(x.Body)
	}
	return e
}
func replaceGraceStmt(s Stmt, target Expr, value Expr) Stmt {
	r := func(e Expr) Expr { return replaceGraceExpr(e, target, value) }
	switch x := s.(type) {
	case *VarDeclStmt:
		copy := *x
		x = &copy
		s = x
		x.Initial = append([]Expr(nil), x.Initial...)
		x.Init = r(x.Init)
		for i := range x.Initial {
			x.Initial[i] = r(x.Initial[i])
		}
	case *AssignStmt:
		copy := *x
		x = &copy
		s = x
		x.Target = r(x.Target)
		x.Val = r(x.Val)
	case *ReturnStmt:
		copy := *x
		x = &copy
		s = x
		x.Vals = append([]Expr(nil), x.Vals...)
		x.Val = r(x.Val)
		for i := range x.Vals {
			x.Vals[i] = r(x.Vals[i])
		}
	case *TupleLetStmt:
		copy := *x
		x = &copy
		s = x
		if x.Call != nil {
			x.Call = r(x.Call).(*CallExpr)
		}
	case *IfStmt:
		copy := *x
		x = &copy
		s = x
		x.Cond = r(x.Cond)
	case *WhileStmt:
		copy := *x
		x = &copy
		s = x
		x.Cond = r(x.Cond)
	case *ForRangeStmt:
		copy := *x
		x = &copy
		s = x
		x.Start = r(x.Start)
		x.End = r(x.End)
	case *ForEachStmt:
		copy := *x
		x = &copy
		s = x
		x.Ptr = r(x.Ptr)
		x.Start = r(x.Start)
		x.Len = r(x.Len)
	case *ExprStmt:
		copy := *x
		x = &copy
		s = x
		x.Expr = r(x.Expr)
	case *SwitchStmt:
		copy := *x
		x = &copy
		s = x
		x.Val = r(x.Val)
	case *StoreStmt:
		copy := *x
		x = &copy
		s = x
		x.Ptr = r(x.Ptr)
		x.Val = r(x.Val)
	}
	return s
}

func rewriteGraceBlock(b *Block, target Expr, value Expr) *Block {
	if b == nil {
		return nil
	}
	out := &Block{Body: make([]Stmt, len(b.Body))}
	for i, st := range b.Body {
		s := replaceGraceStmt(st, target, value)
		switch x := s.(type) {
		case *Block:
			s = rewriteGraceBlock(x, target, value)
		case *IfStmt:
			x.Then = rewriteGraceBlock(x.Then, target, value)
			x.Else = rewriteGraceBlock(x.Else, target, value)
		case *WhileStmt:
			x.Body = rewriteGraceBlock(x.Body, target, value)
		case *ForRangeStmt:
			x.Body = rewriteGraceBlock(x.Body, target, value)
		case *ForEachStmt:
			x.Body = rewriteGraceBlock(x.Body, target, value)
		case *SwitchStmt:
			x.Cases = append([]*SwitchCase(nil), x.Cases...)
			for j, c := range x.Cases {
				cp := *c
				cp.Body = rewriteGraceBlock(c.Body, target, value)
				x.Cases[j] = &cp
			}
			x.Default = rewriteGraceBlock(x.Default, target, value)
		}
		out.Body[i] = s
	}
	return out
}
