package hir

import "github.com/minz/minzc/pkg/mir2"

// scanMutations is the lowerer's view of scanMutations: it also counts outer
// variables assigned inside a lambda the block passes to a call
// (forEach(cb), range(..).forEach(cb), …). Iterator calls inline the lambda
// body into the surrounding loop or branch, so a captured variable it assigns
// is loop-carried / join state exactly like a direct assignment. Without
// this, `for … { range(0..8).forEach(|b| { total = total + … }) }` dropped
// every update to total at the end of each outer iteration.
func (l *lowerer) scanMutations(blk *Block, env map[string]mir2.Reg) map[string]bool {
	muts := scanMutations(blk, env)
	l.scanLambdaMutations(blk, env, muts, map[string]bool{})
	return muts
}

func (l *lowerer) scanLambdaMutations(blk *Block, env map[string]mir2.Reg, muts, seen map[string]bool) {
	if blk == nil {
		return
	}
	visit := func(e Expr) {
		walkExpr(e, func(x Expr) {
			call, ok := x.(*CallExpr)
			if !ok {
				return
			}
			for _, arg := range call.Args {
				name := l.resolveFunc(arg)
				fn := l.hirFuncs[name]
				if name == "" || fn == nil || len(fn.Captures) == 0 || seen[name] {
					continue
				}
				seen[name] = true
				captured := map[string]bool{}
				for _, c := range fn.Captures {
					captured[c.Name] = true
				}
				inner := map[string]bool{}
				l.scanLambdaMutationsInto(fn.Body, env, inner, seen)
				for v := range inner {
					if captured[v] {
						muts[v] = true
					}
				}
			}
		})
	}
	for _, s := range blk.Body {
		switch st := s.(type) {
		case *ExprStmt:
			visit(st.Expr)
		case *AssignStmt:
			visit(st.Val)
		case *VarDeclStmt:
			visit(st.Init)
		case *ReturnStmt:
			visit(st.Val)
			for _, v := range st.Vals {
				visit(v)
			}
		case *TupleLetStmt:
			if st.Call != nil {
				visit(st.Call)
			}
		case *Block:
			l.scanLambdaMutations(st, env, muts, seen)
		case *IfStmt:
			visit(st.Cond)
			l.scanLambdaMutations(st.Then, env, muts, seen)
			l.scanLambdaMutations(st.Else, env, muts, seen)
		case *WhileStmt:
			visit(st.Cond)
			l.scanLambdaMutations(st.Body, env, muts, seen)
		case *ForRangeStmt:
			l.scanLambdaMutations(st.Body, env, muts, seen)
		case *ForEachStmt:
			l.scanLambdaMutations(st.Body, env, muts, seen)
		case *SwitchStmt:
			visit(st.Val)
			for _, c := range st.Cases {
				l.scanLambdaMutations(c.Body, env, muts, seen)
			}
			l.scanLambdaMutations(st.Default, env, muts, seen)
		}
	}
}

// scanLambdaMutationsInto collects a lambda body's direct and nested-lambda
// assignments to names visible in env (captures share their outer names).
func (l *lowerer) scanLambdaMutationsInto(body *Block, env map[string]mir2.Reg, out, seen map[string]bool) {
	scanBlock(body, env, out)
	l.scanLambdaMutations(body, env, out, seen)
}
