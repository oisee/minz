package hir

// Suspension liveness needs definitions as well as references. In particular,
// ANF temporaries read before yield are recreated on the next loop iteration;
// keeping every referenced name promotes all of them into the frame.
type genLiveNode struct {
	use, def map[string]bool
	succ     []int
	yield    bool
}

// Keep the previous conservative analysis for constructs not modelled here.
// This does not change their admission or generator semantics.
func genCFGLiveAcrossYield(body *Block) (map[string]bool, bool) {
	nodes := []genLiveNode{{use: map[string]bool{}, def: map[string]bool{}}} // completion
	supported := true
	add := func(use, def map[string]bool, succ []int, yield bool) int {
		nodes = append(nodes, genLiveNode{use: use, def: def, succ: succ, yield: yield})
		return len(nodes) - 1
	}
	var block func(*Block, int, int, int) int
	var stmt func(Stmt, int, int, int) int
	block = func(b *Block, next, brk, cont int) int {
		if b == nil {
			return next
		}
		for i := len(b.Body) - 1; i >= 0; i-- {
			next = stmt(b.Body[i], next, brk, cont)
		}
		return next
	}
	stmt = func(s Stmt, next, brk, cont int) int {
		// Taking a local address needs alias/storage lifetime analysis. Do not
		// introduce definition kills into the existing conservative path.
		checkExpr := func(e Expr) {
			if _, ok := e.(*AddrOfExpr); ok {
				supported = false
			}
			if _, ok := e.(*LetInExpr); ok {
				supported = false
			}
		}
		visitStmtExprs(s, checkExpr)
		switch st := s.(type) {
		case *Block:
			return block(st, next, brk, cont)
		case *WhileStmt:
			head := add(exprNameRefs(st.Cond), nil, nil, false)
			body := block(st.Body, head, next, head)
			nodes[head].succ = []int{body, next}
			return head
		case *IfStmt:
			yes := block(st.Then, next, brk, cont)
			no := block(st.Else, next, brk, cont)
			return add(exprNameRefs(st.Cond), nil, []int{yes, no}, false)
		case *SwitchStmt:
			succ := []int{block(st.Default, next, brk, cont)}
			for _, c := range st.Cases {
				succ = append(succ, block(c.Body, next, brk, cont))
			}
			return add(exprNameRefs(st.Val), nil, succ, false)
		case *YieldStmt:
			walkExpr(st.Val, checkExpr)
			return add(exprNameRefs(st.Val), nil, []int{next}, true)
		case *BreakStmt:
			if brk < 0 {
				supported = false
				return next
			}
			return add(nil, nil, []int{brk}, false)
		case *ContinueStmt:
			if cont < 0 {
				supported = false
				return next
			}
			return add(nil, nil, []int{cont}, false)
		case *ReturnStmt:
			return add(stmtContRefs(s, nil), nil, nil, false)
		case *VarDeclStmt:
			if st.ArrayLen != 0 {
				supported = false
			}
			return add(stmtContRefs(s, nil), map[string]bool{st.Name: true}, []int{next}, false)
		case *AssignStmt:
			if v, ok := st.Target.(*VarRefExpr); ok {
				return add(exprNameRefs(st.Val), map[string]bool{v.Name: true}, []int{next}, false)
			}
			return add(stmtContRefs(s, nil), nil, []int{next}, false)
		case *ExprStmt, *StoreStmt:
			return add(stmtContRefs(s, nil), nil, []int{next}, false)
		default:
			supported = false
			return next
		}
	}
	block(body, 0, -1, -1)
	if !supported {
		return nil, false
	}
	in := make([]map[string]bool, len(nodes))
	for i := range in {
		in[i] = map[string]bool{}
	}
	// Finite monotone dataflow; no arbitrary iteration cap. Definitions kill
	// only direct local names. Memory stores retain all pointer/index uses.
	for changed := true; changed; {
		changed = false
		for i := len(nodes) - 1; i >= 0; i-- {
			n := nodes[i]
			before := cloneNameSet(n.use)
			for _, s := range n.succ {
				for name := range in[s] {
					if !n.def[name] {
						before[name] = true
					}
				}
			}
			if !nameSetsEqual(before, in[i]) {
				in[i] = before
				changed = true
			}
		}
	}
	live := map[string]bool{}
	for _, n := range nodes {
		if n.yield {
			for _, s := range n.succ {
				for name := range in[s] {
					live[name] = true
				}
			}
		}
	}
	return live, true
}
