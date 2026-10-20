package hir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// Generator expansion (ADR 0048/0049, stage G1).
//
// A `gen name(params) -> ElemTy { … yield e … }` declaration stays
// generator-aware through parsing and verification. Immediately before HIR
// lowering it is rewritten into the concrete pull protocol:
//
//   - a frame struct `name$frame`: continuation tag, the saved parameters,
//     and every local that is live across a suspension. The frame is declared
//     in the consuming scope, so HIR struct-local storage (a per-invocation
//     stack allocation on Z80) applies — never a hoisted global, which would
//     break independent instances and re-entry.
//   - a `name$next(frame: ^Frame, out: ^ElemTy) -> bool` function: entry with
//     tag DONE returns false immediately; each `yield e` becomes
//     `out^ = e; frame.tag = k; return true; Lk:`; every completion path
//     (plain `return`, falling off the end) sets tag = DONE first. MIR2 needs
//     no coroutine opcodes — only loads, stores, branches and returns.
//
// Resume labels stay inside the structured loops of the source body. Entering
// a loop body at a label is correct because the suspension point is inside
// that body: the condition was true when the yield executed, so the next
// request resumes after the yield, reaches the back edge and re-checks the
// condition with all loop-carried values restored from the frame.
//
// Locals that are not live across any yield remain ordinary locals of
// next() — a `for` loop without a yield inside therefore keeps its
// compiler-managed induction variable.

const (
	genTagStart = 0   // body has not started yet
	genTagDone  = 255 // sticky completion; every later request returns false
)

// genFrameTagField is the first frame field; generator parameters and locals
// must not use this name (rejected by the frontend).
const genFrameTagField = "tag"

// ExpandGenerators rewrites every gen declaration into its frame struct and
// next function and rewrites all consumption sites to the pull protocol. It
// is idempotent: a module without generator nodes is returned unchanged.
func ExpandGenerators(m *Module) error {
	gens := make(map[string]*Func)
	for _, f := range m.Funcs {
		if f.IsGen {
			if f.IsExtern || f.Body == nil {
				return fmt.Errorf("gen %s: missing body", f.Name)
			}
			gens[f.Name] = f
		}
	}
	if len(gens) == 0 {
		return checkNoStrayGenNodes(m)
	}

	e := &genExpander{m: m, gens: gens, frames: map[string]*mir2.StructTy{}}

	// Expand declarations first: consumption sites need frame layouts.
	var kept []*Func
	for _, f := range m.Funcs {
		if !f.IsGen {
			kept = append(kept, f)
			continue
		}
		frame, next, err := e.expandGen(f)
		if err != nil {
			return err
		}
		e.frames[f.Name] = frame
		e.nextFuncs = append(e.nextFuncs, next)
	}

	// Rewrite consumption in every remaining function (including lambdas).
	for _, f := range kept {
		if f.Body == nil {
			continue
		}
		if err := e.rewriteConsumers(f); err != nil {
			return err
		}
	}

	m.Funcs = append(kept, e.nextFuncs...)
	return nil
}

type genExpander struct {
	m         *Module
	gens      map[string]*Func
	frames    map[string]*mir2.StructTy
	nextFuncs []*Func
	tempCount int
}

// nextName is the concrete continuation function of a generator.
func genNextName(gen string) string { return gen + "$next" }

// frameTempName is an unspellable-by-source local holding a for-loop frame.
func (e *genExpander) frameTempName(gen string) string {
	e.tempCount++
	return fmt.Sprintf("%s$frame$%d", gen, e.tempCount)
}

// expandGen builds the frame struct and next function of one generator.
func (e *genExpander) expandGen(f *Func) (*mir2.StructTy, *Func, error) {
	// Collect declaration order and types of body locals.
	order := []string{}
	localTy := map[string]mir2.Ty{}
	var collect func(b *Block)
	collect = func(b *Block) {
		for _, s := range b.Body {
			switch st := s.(type) {
			case *VarDeclStmt:
				if st.ArrayLen == 0 {
					if _, dup := localTy[st.Name]; dup {
						return // duplicate declaration: frontend rejects; keep first
					}
					localTy[st.Name] = st.Ty
					order = append(order, st.Name)
				}
			case *Block:
				collect(st)
			case *IfStmt:
				collect(st.Then)
				if st.Else != nil {
					collect(st.Else)
				}
			case *WhileStmt:
				collect(st.Body)
			case *ForRangeStmt:
				collect(st.Body)
			case *ForEachStmt:
				collect(st.Body)
			case *SwitchStmt:
				for _, c := range st.Cases {
					collect(c.Body)
				}
				if st.Default != nil {
					collect(st.Default)
				}
			}
		}
	}
	collect(f.Body)

	// Names that must survive a suspension: referenced in the continuation of
	// any yield. Loop induction variables of yield-free for loops never
	// qualify (the frontend rejects yields inside them), so they stay locals.
	live := genLiveAcrossYield(f.Body)
	delete(live, genFrameTagField)
	forVars := map[string]bool{}
	collectLoopVars(f.Body, forVars)

	fields := []mir2.StructField{{Name: genFrameTagField, Ty: mir2.TyU8}}
	add := func(name string, ty mir2.Ty) {
		fields = append(fields, mir2.StructField{Name: name, Ty: ty})
	}
	for _, p := range f.Params {
		add(p.Name, p.Ty)
	}
	frameFields := map[string]bool{}
	for _, name := range order {
		if !live[name] || forVars[name] {
			continue
		}
		add(name, localTy[name])
		frameFields[name] = true
	}
	frame := &mir2.StructTy{Name: f.Name + "$frame", Fields: fields}
	e.m.Structs = append(e.m.Structs, frame)
	// Parameters are frame-resident by construction: creation stores them
	// before the first request, so every reference reloads them from there.
	for _, p := range f.Params {
		frameFields[p.Name] = true
	}

	// Compile the body into the flat label/goto form of next().
	r := &genRewriter{
		frame:       frame,
		frameFields: frameFields,
		elemTy:      f.ElemTy,
		resumeNames: map[int]string{},
		labelUsed:   collectLabelNames(f.Body),
	}

	next := &Func{
		Name:  genNextName(f.Name),
		IsGen: false,
		Params: []Param{
			{Name: "$frame", Ty: mir2.TyPtr},
			{Name: "$out", Ty: mir2.PointerTo(f.ElemTy)},
		},
		RetTy: mir2.TyBool,
		Body:  r.assembleNext(f),
	}
	return frame, next, nil
}

// assembleNext wraps the rewritten body with the DONE check, the resume
// dispatch and the completion sequence.
func (r *genRewriter) assembleNext(f *Func) *Block {
	// Sticky DONE: no effects, out untouched.
	stmts := []Stmt{&IfStmt{
		Cond: &BinExpr{Op: "==", L: r.tagField(), R: &IntLitExpr{Val: genTagDone, Ty: mir2.TyU8}, Ty: mir2.TyBool},
		Then: Blk(&ReturnStmt{Val: &BoolLitExpr{Val: false}}),
	}}

	// Flatten first: the dispatch needs the resume labels and yield count.
	r.flatten(f.Body.Body)

	// Resume dispatch: tag k jumps to the label placed after yield k.
	// The fall-through (tag == 0) starts the body.
	for tag := 1; tag <= r.yieldCount; tag++ {
		stmts = append(stmts, &IfStmt{
			Cond: &BinExpr{Op: "==", L: r.tagField(), R: &IntLitExpr{Val: int64(tag), Ty: mir2.TyU8}, Ty: mir2.TyBool},
			Then: Blk(&GotoStmt{Label: r.resumeNames[tag]}),
		})
	}

	stmts = append(stmts, r.out...)
	stmts = append(stmts, r.doneSeq()...)
	return Blk(stmts...)
}

// genRewriter compiles one generator body into the flat label/goto form of
// its next function.
type genRewriter struct {
	frame       *mir2.StructTy
	frameFields map[string]bool // locals promoted to frame storage
	elemTy      mir2.Ty
	yieldCount  int
	resumeNames map[int]string  // tag → resume label (assigned in yield order)
	labelUsed   map[string]bool // labels already taken (source labels included)
	out         []Stmt          // flattened output
	brk, cont   string          // targets of the enclosing flattened loop
}

func (r *genRewriter) tagField() *FieldExpr {
	return &FieldExpr{X: r.frameRef(), Field: genFrameTagField, Ty: mir2.TyU8}
}

func (r *genRewriter) frameRef() *VarRefExpr {
	return &VarRefExpr{Name: "$frame", Ty: mir2.TyPtr}
}

func (r *genRewriter) field(name string) *FieldExpr {
	off := 0
	ty := mir2.Ty(mir2.TyU8)
	for i, f := range r.frame.Fields {
		if f.Name == name {
			off = r.frame.ByteOffset(i)
			ty = f.Ty
			break
		}
	}
	return &FieldExpr{X: r.frameRef(), Field: name, Offset: off, Ty: ty}
}

func (r *genRewriter) freshLabel(prefix string) string {
	for i := 0; ; i++ {
		name := fmt.Sprintf("%s_%d", prefix, i)
		if !r.labelUsed[name] {
			r.labelUsed[name] = true
			return name
		}
	}
}

// doneSeq finishes the sequence: completion is sticky.
func (r *genRewriter) doneSeq() []Stmt {
	return []Stmt{
		&AssignStmt{Target: r.tagField(), Val: &IntLitExpr{Val: genTagDone, Ty: mir2.TyU8}},
		&ReturnStmt{Val: &BoolLitExpr{Val: false}},
	}
}

func (r *genRewriter) emit(s Stmt) { r.out = append(r.out, s) }

// emitRewritten keeps a statement structured and rewrites its sub-expressions.
func (r *genRewriter) emitRewritten(s Stmt) {
	mapStmtExprs(s, r.expr)
	r.emit(s)
}

// flatten compiles the generator body to a flat label/goto form. Structured
// HIR loops thread the names they use as head block parameters, which a
// resume entry would bypass (the dispatch edge cannot reference head
// parameters it never reached); entering a loop body directly is therefore
// only sound when the loop has no suspension inside. Loops containing yields
// are lowered here as label/goto pairs — the form the HIR goto machinery is
// designed for — so every resume label sits at one nesting level.
func (r *genRewriter) flatten(stmts []Stmt) {
	for _, s := range stmts {
		r.flattenStmt(s)
	}
}

func (r *genRewriter) flattenStmt(s Stmt) {
	// Frame-resident declarations become plain stores; the frame slot is the
	// variable's storage for the whole suspension lifetime.
	if decl, ok := s.(*VarDeclStmt); ok {
		if r.frameFields[decl.Name] {
			if decl.Init != nil {
				r.emit(&AssignStmt{Target: r.field(decl.Name), Val: r.expr(decl.Init)})
			}
			return
		}
	}

	// Inside a flattened loop, break/continue must reach its labels even when
	// the enclosing statement itself has no yield.
	switch s.(type) {
	case *BreakStmt:
		if r.brk != "" {
			r.emit(&GotoStmt{Label: r.brk})
			return
		}
	case *ContinueStmt:
		if r.cont != "" {
			r.emit(&GotoStmt{Label: r.cont})
			return
		}
	}

	// Outside flattened regions, yield-free statements keep their structure
	// (structured lowering, optimized for/foreach forms included).
	if r.brk == "" && r.cont == "" && !genStmtHasYield(s) {
		r.emitRewritten(s)
		return
	}

	switch st := s.(type) {
	case *YieldStmt:
		tag := r.yieldCount + 1
		r.yieldCount = tag
		name := r.freshLabel("gen_resume")
		r.resumeNames[tag] = name
		r.emit(&AssignStmt{
			Target: &DerefExpr{Ptr: &VarRefExpr{Name: "$out", Ty: mir2.PointerTo(r.elemTy)}, Ty: r.elemTy},
			Val:    r.expr(st.Val),
		})
		r.emit(&AssignStmt{Target: r.tagField(), Val: &IntLitExpr{Val: int64(tag), Ty: mir2.TyU8}})
		r.emit(&ReturnStmt{Val: &BoolLitExpr{Val: true}})
		r.emit(&LabelStmt{Name: name})

	case *ReturnStmt:
		if st.Val != nil || len(st.Vals) > 0 {
			// The frontend rejects valued returns in gen bodies.
			r.emit(&ReturnStmt{Val: &BoolLitExpr{Val: false}})
			return
		}
		r.out = append(r.out, r.doneSeq()...)

	case *WhileStmt:
		if !genStmtHasYield(st) {
			// Its own induction state and break context stay structured.
			r.emitRewritten(st)
			return
		}
		head := r.freshLabel("gen_loop_head")
		body := r.freshLabel("gen_loop_body")
		exit := r.freshLabel("gen_loop_exit")
		r.emit(&LabelStmt{Name: head})
		r.emit(&IfStmt{Cond: r.expr(st.Cond), Then: Blk(&GotoStmt{Label: body})})
		r.emit(&GotoStmt{Label: exit})
		r.emit(&LabelStmt{Name: body})
		savedBrk, savedCont := r.brk, r.cont
		r.brk, r.cont = exit, head
		r.flatten(st.Body.Body)
		r.brk, r.cont = savedBrk, savedCont
		r.emit(&GotoStmt{Label: head})
		r.emit(&LabelStmt{Name: exit})

	case *IfStmt:
		thenL := r.freshLabel("gen_then")
		joinL := r.freshLabel("gen_join")
		r.emit(&IfStmt{Cond: r.expr(st.Cond), Then: Blk(&GotoStmt{Label: thenL})})
		if st.Else != nil {
			elseL := r.freshLabel("gen_else")
			r.emit(&GotoStmt{Label: elseL})
			r.emit(&LabelStmt{Name: thenL})
			r.flatten(st.Then.Body)
			r.emit(&GotoStmt{Label: joinL})
			r.emit(&LabelStmt{Name: elseL})
			r.flatten(st.Else.Body)
		} else {
			r.emit(&GotoStmt{Label: joinL})
			r.emit(&LabelStmt{Name: thenL})
			r.flatten(st.Then.Body)
		}
		r.emit(&LabelStmt{Name: joinL})

	case *SwitchStmt:
		// Compare chain over a value evaluated once. Non-constant cases are
		// rejected by the frontend; default is optional.
		tmp := r.freshLabel("gen_sw_val") // local name, not a label
		r.emit(&VarDeclStmt{Name: tmp, Ty: st.Val.ExprTy(), Init: r.expr(st.Val)})
		joinL := r.freshLabel("gen_sw_join")
		caseLabels := make([]string, len(st.Cases))
		for i, c := range st.Cases {
			caseLabels[i] = r.freshLabel("gen_sw_case")
			r.emit(&IfStmt{
				Cond: &BinExpr{Op: "==", L: &VarRefExpr{Name: tmp, Ty: st.Val.ExprTy()}, R: &IntLitExpr{Val: c.Val, Ty: st.Val.ExprTy()}, Ty: mir2.TyBool},
				Then: Blk(&GotoStmt{Label: caseLabels[i]}),
			})
		}
		if st.Default != nil {
			dflt := r.freshLabel("gen_sw_dflt")
			r.emit(&GotoStmt{Label: dflt})
			for i, c := range st.Cases {
				r.emit(&LabelStmt{Name: caseLabels[i]})
				r.flatten(c.Body.Body)
				r.emit(&GotoStmt{Label: joinL})
			}
			r.emit(&LabelStmt{Name: dflt})
			r.flatten(st.Default.Body)
		} else {
			r.emit(&GotoStmt{Label: joinL})
			for i, c := range st.Cases {
				r.emit(&LabelStmt{Name: caseLabels[i]})
				r.flatten(c.Body.Body)
				r.emit(&GotoStmt{Label: joinL})
			}
		}
		r.emit(&LabelStmt{Name: joinL})

	case *Block:
		r.flatten(st.Body)

	default:
		r.emitRewritten(s)
	}
}

// genStmtHasYield reports whether a statement transitively contains a yield.
func genStmtHasYield(s Stmt) bool {
	switch st := s.(type) {
	case *YieldStmt:
		return true
	case *Block:
		return genBlockHasYield(st)
	case *IfStmt:
		return genBlockHasYield(st.Then) || st.Else != nil && genBlockHasYield(st.Else)
	case *WhileStmt:
		return genBlockHasYield(st.Body)
	case *ForRangeStmt:
		return genBlockHasYield(st.Body)
	case *ForEachStmt:
		return genBlockHasYield(st.Body)
	case *SwitchStmt:
		for _, c := range st.Cases {
			if genBlockHasYield(c.Body) {
				return true
			}
		}
		return st.Default != nil && genBlockHasYield(st.Default)
	}
	return false
}

func genBlockHasYield(b *Block) bool {
	for _, s := range b.Body {
		if genStmtHasYield(s) {
			return true
		}
	}
	return false
}

func (r *genRewriter) exprOpt(e Expr) Expr {
	if e == nil {
		return nil
	}
	return r.expr(e)
}

func (r *genRewriter) expr(e Expr) Expr {
	switch ex := e.(type) {
	case nil:
		return nil
	case *VarRefExpr:
		if r.frameFields[ex.Name] {
			return r.field(ex.Name)
		}
		return ex
	case *IntLitExpr, *BoolLitExpr, *ConstPtrExpr, *RangeSourceExpr:
		return e
	case *BinExpr:
		ex.L = r.expr(ex.L)
		ex.R = r.expr(ex.R)
		return ex
	case *UnaryExpr:
		ex.X = r.expr(ex.X)
		return ex
	case *CallExpr:
		for i := range ex.Args {
			ex.Args[i] = r.expr(ex.Args[i])
		}
		return ex
	case *CallIndirectExpr:
		ex.FnPtr = r.expr(ex.FnPtr)
		for i := range ex.Args {
			ex.Args[i] = r.expr(ex.Args[i])
		}
		return ex
	case *FieldExpr:
		ex.X = r.expr(ex.X)
		return ex
	case *BitExpr:
		ex.X = r.expr(ex.X)
		return ex
	case *AddrOfExpr:
		ex.X = r.exprOpt(ex.X)
		return ex
	case *LoadExpr:
		ex.Ptr = r.expr(ex.Ptr)
		return ex
	case *DerefExpr:
		ex.Ptr = r.expr(ex.Ptr)
		return ex
	case *CastExpr:
		ex.X = r.expr(ex.X)
		return ex
	case *IndexExpr:
		ex.Base = r.expr(ex.Base)
		ex.Idx = r.expr(ex.Idx)
		return ex
	case *CondExpr:
		ex.Cond = r.expr(ex.Cond)
		ex.Then = r.expr(ex.Then)
		ex.Else = r.expr(ex.Else)
		return ex
	case *StructLitExpr:
		for i := range ex.Fields {
			ex.Fields[i].Val = r.expr(ex.Fields[i].Val)
		}
		return ex
	default:
		return e
	}
}

// ── Consumption sites ─────────────────────────────────────────────────────────

// rewriteConsumers turns generator creation and pull operations in f into
// ordinary declarations, field stores and calls.
func (e *genExpander) rewriteConsumers(f *Func) error {
	var walk func(b *Block) error
	walk = func(b *Block) error {
		var out []Stmt
		for _, s := range b.Body {
			switch st := s.(type) {
			case *VarDeclStmt:
				make, isMake, err := e.makeStores(st)
				if err != nil {
					return err
				}
				if isMake {
					out = append(out, make...)
					continue
				}
				out = append(out, s)

			case *GenForStmt:
				if err := walk(st.Body); err != nil {
					return err
				}
				desugared, err := e.desugarGenFor(st)
				if err != nil {
					return err
				}
				out = append(out, desugared...)

			case *Block:
				if err := walk(st); err != nil {
					return err
				}
				out = append(out, s)

			case *IfStmt:
				if err := walk(st.Then); err != nil {
					return err
				}
				if st.Else != nil {
					if err := walk(st.Else); err != nil {
						return err
					}
				}
				out = append(out, s)

			case *WhileStmt:
				if err := walk(st.Body); err != nil {
					return err
				}
				out = append(out, s)

			case *ForRangeStmt:
				if err := walk(st.Body); err != nil {
					return err
				}
				out = append(out, s)

			case *ForEachStmt:
				if err := walk(st.Body); err != nil {
					return err
				}
				out = append(out, s)

			case *SwitchStmt:
				for _, c := range st.Cases {
					if err := walk(c.Body); err != nil {
						return err
					}
				}
				if st.Default != nil {
					if err := walk(st.Default); err != nil {
						return err
					}
				}
				out = append(out, s)

			default:
				out = append(out, s)
			}
		}
		b.Body = out
		return nil
	}
	if err := walk(f.Body); err != nil {
		return fmt.Errorf("%s: %w", f.Name, err)
	}
	mapBlockExprs(f.Body, e.rewritePullExpr)
	return nil
}

// mapBlockExprs applies fn to every expression in the block (post-order).
func mapBlockExprs(b *Block, fn func(Expr) Expr) {
	for _, s := range b.Body {
		mapStmtExprs(s, fn)
	}
}

// mapStmtExprs rewrites every sub-expression of s in place, children first.
func mapStmtExprs(s Stmt, fn func(Expr) Expr) {
	mapExpr := func(e Expr) Expr { return mapExprChildren(e, fn) }
	switch st := s.(type) {
	case *Block:
		mapBlockExprs(st, fn)
	case *VarDeclStmt:
		st.Init = mapExpr(st.Init)
		for i := range st.Initial {
			st.Initial[i] = mapExpr(st.Initial[i])
		}
	case *AssignStmt:
		st.Target = mapExpr(st.Target)
		st.Val = mapExpr(st.Val)
	case *ReturnStmt:
		st.Val = mapExpr(st.Val)
		for i := range st.Vals {
			st.Vals[i] = mapExpr(st.Vals[i])
		}
	case *TupleLetStmt:
		st.Call = mapExpr(st.Call).(*CallExpr)
	case *ExprStmt:
		st.Expr = mapExpr(st.Expr)
	case *StoreStmt:
		st.Ptr = mapExpr(st.Ptr)
		st.Val = mapExpr(st.Val)
	case *IfStmt:
		st.Cond = mapExpr(st.Cond)
		mapBlockExprs(st.Then, fn)
		if st.Else != nil {
			mapBlockExprs(st.Else, fn)
		}
	case *WhileStmt:
		st.Cond = mapExpr(st.Cond)
		mapBlockExprs(st.Body, fn)
	case *ForRangeStmt:
		st.Start = mapExpr(st.Start)
		st.End = mapExpr(st.End)
		mapBlockExprs(st.Body, fn)
	case *ForEachStmt:
		st.Ptr = mapExpr(st.Ptr)
		st.Start = mapExpr(st.Start)
		st.Len = mapExpr(st.Len)
		mapBlockExprs(st.Body, fn)
	case *GenForStmt:
		for i := range st.Args {
			st.Args[i] = mapExpr(st.Args[i])
		}
		mapBlockExprs(st.Body, fn)
	case *SwitchStmt:
		st.Val = mapExpr(st.Val)
		for _, c := range st.Cases {
			mapBlockExprs(c.Body, fn)
		}
		if st.Default != nil {
			mapBlockExprs(st.Default, fn)
		}
	case *LabelStmt, *GotoStmt, *BreakStmt, *ContinueStmt, *YieldStmt, *AsmStmt:
	}
}

// mapExprChildren applies fn to the node's children, then to the node.
func mapExprChildren(e Expr, fn func(Expr) Expr) Expr {
	if e == nil {
		return nil
	}
	switch ex := e.(type) {
	case *IntLitExpr, *BoolLitExpr, *ConstPtrExpr, *RangeSourceExpr:
	case *BinExpr:
		ex.L = mapExprChildren(ex.L, fn)
		ex.R = mapExprChildren(ex.R, fn)
	case *UnaryExpr:
		ex.X = mapExprChildren(ex.X, fn)
	case *CallExpr:
		for i := range ex.Args {
			ex.Args[i] = mapExprChildren(ex.Args[i], fn)
		}
	case *CallIndirectExpr:
		ex.FnPtr = mapExprChildren(ex.FnPtr, fn)
		for i := range ex.Args {
			ex.Args[i] = mapExprChildren(ex.Args[i], fn)
		}
	case *FieldExpr:
		ex.X = mapExprChildren(ex.X, fn)
	case *BitExpr:
		ex.X = mapExprChildren(ex.X, fn)
	case *AddrOfExpr:
		ex.X = mapExprChildren(ex.X, fn)
	case *LoadExpr:
		ex.Ptr = mapExprChildren(ex.Ptr, fn)
	case *DerefExpr:
		ex.Ptr = mapExprChildren(ex.Ptr, fn)
	case *CastExpr:
		ex.X = mapExprChildren(ex.X, fn)
	case *IndexExpr:
		ex.Base = mapExprChildren(ex.Base, fn)
		ex.Idx = mapExprChildren(ex.Idx, fn)
	case *CondExpr:
		ex.Cond = mapExprChildren(ex.Cond, fn)
		ex.Then = mapExprChildren(ex.Then, fn)
		ex.Else = mapExprChildren(ex.Else, fn)
	case *LetInExpr:
		ex.Init = mapExprChildren(ex.Init, fn)
		ex.Body = mapExprChildren(ex.Body, fn)
	case *StructLitExpr:
		for i := range ex.Fields {
			ex.Fields[i].Val = mapExprChildren(ex.Fields[i].Val, fn)
		}
	case *GenMakeExpr:
		for i := range ex.Args {
			ex.Args[i] = mapExprChildren(ex.Args[i], fn)
		}
	case *GenNextExpr:
		ex.Frame = mapExprChildren(ex.Frame, fn)
		ex.Out = mapExprChildren(ex.Out, fn)
	}
	return fn(e)
}

// makeStores expands `var it = gen(args)` into the frame declaration followed
// by argument stores and tag = 0. Creation runs no body effects.
func (e *genExpander) makeStores(st *VarDeclStmt) ([]Stmt, bool, error) {
	var mk *GenMakeExpr
	if m, ok := st.Init.(*GenMakeExpr); ok {
		mk = m
	} else if c, ok := st.Init.(*CallExpr); ok {
		if _, isGen := e.gens[c.Fn]; !isGen {
			return nil, false, nil
		}
		mk = &GenMakeExpr{Gen: c.Fn, Frame: e.frames[c.Fn], Args: c.Args}
	}
	if mk == nil {
		return nil, false, nil
	}
	frame, ok := e.frames[mk.Gen]
	if !ok {
		return nil, false, fmt.Errorf("unknown generator %q", mk.Gen)
	}
	if len(mk.Args) != len(e.gens[mk.Gen].Params) {
		return nil, false, fmt.Errorf("generator %s: %d arguments, want %d",
			mk.Gen, len(mk.Args), len(e.gens[mk.Gen].Params))
	}
	decl := &VarDeclStmt{Name: st.Name, Ty: frame}
	stores := []Stmt{decl}
	for i, arg := range mk.Args {
		p := e.gens[mk.Gen].Params[i]
		stores = append(stores, &AssignStmt{
			Target: &FieldExpr{X: &VarRefExpr{Name: st.Name, Ty: frame}, Field: p.Name, Offset: frame.ByteOffset(1 + i), Ty: p.Ty},
			Val:    arg,
		})
	}
	stores = append(stores, &AssignStmt{
		Target: &FieldExpr{X: &VarRefExpr{Name: st.Name, Ty: frame}, Field: genFrameTagField, Ty: mir2.TyU8},
		Val:    &IntLitExpr{Val: genTagStart, Ty: mir2.TyU8},
	})
	return stores, true, nil
}

// desugarGenFor expands `for x in gen(args) { body }` into a local frame, a
// local element and `while pull_next(&frame, &x) { body }`. The while gives
// the required ownership: break abandons the temporary frame without resuming
// the producer, continue requests the next element, return leaves the
// consumer function.
func (e *genExpander) desugarGenFor(st *GenForStmt) ([]Stmt, error) {
	frame, ok := e.frames[st.Gen]
	if !ok {
		return nil, fmt.Errorf("unknown generator %q", st.Gen)
	}
	if len(st.Args) != len(e.gens[st.Gen].Params) {
		return nil, fmt.Errorf("generator %s: %d arguments, want %d",
			st.Gen, len(st.Args), len(e.gens[st.Gen].Params))
	}
	temp := e.frameTempName(st.Gen)
	elem := &VarDeclStmt{Name: st.Var, Ty: st.ElemTy, Init: &IntLitExpr{Val: 0, Ty: st.ElemTy}}
	out := []Stmt{&VarDeclStmt{Name: temp, Ty: frame}}
	for i, arg := range st.Args {
		p := e.gens[st.Gen].Params[i]
		out = append(out, &AssignStmt{
			Target: &FieldExpr{X: &VarRefExpr{Name: temp, Ty: frame}, Field: p.Name, Offset: frame.ByteOffset(1 + i), Ty: p.Ty},
			Val:    arg,
		})
	}
	out = append(out, &AssignStmt{
		Target: &FieldExpr{X: &VarRefExpr{Name: temp, Ty: frame}, Field: genFrameTagField, Ty: mir2.TyU8},
		Val:    &IntLitExpr{Val: genTagStart, Ty: mir2.TyU8},
	}, elem)
	out = append(out, &WhileStmt{
		Cond: &CallExpr{
			Fn:   genNextName(st.Gen),
			Args: []Expr{
				&AddrOfExpr{Sym: temp},
				&AddrOfExpr{Sym: st.Var, Ty: mir2.PointerTo(st.ElemTy)},
			},
			Ty: mir2.TyBool,
		},
		Body: st.Body,
	})
	return out, nil
}

// rewritePullExpr replaces GenNextExpr nodes with the concrete next call.
func (e *genExpander) rewritePullExpr(ex Expr) Expr {
	n, ok := ex.(*GenNextExpr)
	if !ok {
		return ex
	}
	return &CallExpr{
		Fn:   genNextName(n.Gen),
		Args: []Expr{n.Frame, n.Out},
		Ty:   mir2.TyBool,
	}
}

// checkNoStrayGenNodes rejects generator nodes without a matching declaration
// (a generator used from another module is not supported in G1).
func checkNoStrayGenNodes(m *Module) error {
	for _, f := range m.Funcs {
		if f.Body == nil {
			continue
		}
		var bad error
		walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
			for _, s := range b.Body {
				if _, isYield := s.(*YieldStmt); isYield {
					bad = fmt.Errorf("%s: yield outside generator", f.Name)
				}
				if _, isFor := s.(*GenForStmt); isFor {
					bad = fmt.Errorf("%s: generator for-loop without declaration", f.Name)
				}
				visitStmtExprs(s, func(e Expr) {
					switch e.(type) {
					case *GenMakeExpr:
						bad = fmt.Errorf("%s: generator creation without declaration", f.Name)
					case *GenNextExpr:
						bad = fmt.Errorf("%s: pull_next without declaration", f.Name)
					}
				})
			}
		})
		if bad != nil {
			return bad
		}
	}
	return nil
}

// hasGenNodes reports whether a body still contains generator constructs.
func hasGenNodes(b *Block) bool {
	found := false
	walkGraceBlocks(b, "", func(_ string, blk *Block) {
		for _, s := range blk.Body {
			switch s.(type) {
			case *YieldStmt, *GenForStmt:
				found = true
			}
			visitStmtExprs(s, func(e Expr) {
				switch e.(type) {
				case *GenMakeExpr, *GenNextExpr:
					found = true
				}
			})
		}
	})
	return found
}

// ── Liveness across suspension ───────────────────────────────────────────────

// genLiveAcrossYield returns the names referenced in the continuation of any
// yield point. The continuation of a yield includes everything after it in its
// block, the tails of all enclosing constructs, and — for yields inside a loop
// — the loop condition and the whole body again, because the back edge
// re-executes statements that precede the yield.
func genLiveAcrossYield(body *Block) map[string]bool {
	live := map[string]bool{}
	walkBlockCont(body, map[string]bool{}, live)
	return live
}

func walkBlockCont(b *Block, cont map[string]bool, live map[string]bool) map[string]bool {
	tail := cloneNameSet(cont)
	for i := len(b.Body) - 1; i >= 0; i-- {
		tail = walkStmtCont(b.Body[i], tail, live)
	}
	return tail
}

// walkStmtCont returns the names referenced at or after s, given that `tail`
// is referenced after s. Yields record their full continuation in live.
func walkStmtCont(s Stmt, tail map[string]bool, live map[string]bool) map[string]bool {
	var own map[string]bool
	switch st := s.(type) {
	case *YieldStmt:
		for n := range tail {
			live[n] = true
		}
		own = exprNameRefs(st.Val)

	case *WhileStmt:
		// Loop continuation: condition ∪ body ∪ outer tail, to a fixpoint.
		c := unionNameSets(exprNameRefs(st.Cond), tail)
		var bodyRefs map[string]bool
		for iter := 0; iter < 32; iter++ {
			bodyRefs = walkBlockCont(st.Body, c, live)
			nc := unionNameSets(exprNameRefs(st.Cond), bodyRefs, tail)
			if nameSetsEqual(nc, c) {
				break
			}
			c = nc
		}
		own = unionNameSets(exprNameRefs(st.Cond), bodyRefs)

	case *IfStmt:
		own = walkBlockCont(st.Then, tail, live)
		if st.Else != nil {
			own = unionNameSets(own, walkBlockCont(st.Else, tail, live))
		}
		own = unionNameSets(own, exprNameRefs(st.Cond))

	case *Block:
		own = walkBlockCont(st, tail, live)

	case *SwitchStmt:
		own = exprNameRefs(st.Val)
		for _, c := range st.Cases {
			own = unionNameSets(own, walkBlockCont(c.Body, tail, live))
		}
		if st.Default != nil {
			own = unionNameSets(own, walkBlockCont(st.Default, tail, live))
		}

	default:
		own = stmtContRefs(s, tail)
	}
	return unionNameSets(own, tail)
}

// stmtContRefs returns the names an ordinary statement references.
func stmtContRefs(s Stmt, _ map[string]bool) map[string]bool {
	refs := map[string]bool{}
	switch st := s.(type) {
	case *VarDeclStmt:
		if st.Init != nil {
			collectExprNameRefs(st.Init, refs)
		}
		for _, e := range st.Initial {
			collectExprNameRefs(e, refs)
		}
	case *AssignStmt:
		collectExprNameRefs(st.Target, refs)
		collectExprNameRefs(st.Val, refs)
	case *ExprStmt:
		collectExprNameRefs(st.Expr, refs)
	case *StoreStmt:
		collectExprNameRefs(st.Ptr, refs)
		collectExprNameRefs(st.Val, refs)
	case *TupleLetStmt:
		collectExprNameRefs(st.Call, refs)
	case *ForRangeStmt:
		collectExprNameRefs(st.Start, refs)
		collectExprNameRefs(st.End, refs)
		refs[st.Var] = true
		// Body references matter for enclosing statements, not for yields.
		collectStmtsNameRefs(st.Body, refs)
	case *ForEachStmt:
		collectExprNameRefs(st.Ptr, refs)
		collectExprNameRefs(st.Start, refs)
		collectExprNameRefs(st.Len, refs)
		refs[st.Var] = true
		collectStmtsNameRefs(st.Body, refs)
	case *ReturnStmt:
		if st.Val != nil {
			collectExprNameRefs(st.Val, refs)
		}
		for _, v := range st.Vals {
			collectExprNameRefs(v, refs)
		}
	case *LabelStmt, *GotoStmt, *BreakStmt, *ContinueStmt, *AsmStmt, *YieldStmt:
	}
	return refs
}

func collectStmtsNameRefs(b *Block, refs map[string]bool) {
	for _, s := range b.Body {
		for n := range stmtContRefs(s, nil) {
			refs[n] = true
		}
	}
}

func exprNameRefs(e Expr) map[string]bool {
	refs := map[string]bool{}
	collectExprNameRefs(e, refs)
	return refs
}

// collectExprNameRefs records every named reference in e, including the
// symbol form of address-of.
func collectExprNameRefs(e Expr, refs map[string]bool) {
	switch ex := e.(type) {
	case nil:
		return
	case *VarRefExpr:
		refs[ex.Name] = true
	case *IntLitExpr, *BoolLitExpr, *ConstPtrExpr, *RangeSourceExpr:
	case *BinExpr:
		collectExprNameRefs(ex.L, refs)
		collectExprNameRefs(ex.R, refs)
	case *UnaryExpr:
		collectExprNameRefs(ex.X, refs)
	case *CallExpr:
		for _, a := range ex.Args {
			collectExprNameRefs(a, refs)
		}
	case *CallIndirectExpr:
		collectExprNameRefs(ex.FnPtr, refs)
		for _, a := range ex.Args {
			collectExprNameRefs(a, refs)
		}
	case *FieldExpr:
		collectExprNameRefs(ex.X, refs)
	case *BitExpr:
		collectExprNameRefs(ex.X, refs)
	case *AddrOfExpr:
		if ex.Sym != "" {
			refs[ex.Sym] = true
		}
		collectExprNameRefs(ex.X, refs)
	case *LoadExpr:
		collectExprNameRefs(ex.Ptr, refs)
	case *DerefExpr:
		collectExprNameRefs(ex.Ptr, refs)
	case *CastExpr:
		collectExprNameRefs(ex.X, refs)
	case *IndexExpr:
		collectExprNameRefs(ex.Base, refs)
		collectExprNameRefs(ex.Idx, refs)
	case *CondExpr:
		collectExprNameRefs(ex.Cond, refs)
		collectExprNameRefs(ex.Then, refs)
		collectExprNameRefs(ex.Else, refs)
	case *StructLitExpr:
		for _, f := range ex.Fields {
			collectExprNameRefs(f.Val, refs)
		}
	case *GenMakeExpr:
		for _, a := range ex.Args {
			collectExprNameRefs(a, refs)
		}
	case *GenNextExpr:
		collectExprNameRefs(ex.Frame, refs)
		collectExprNameRefs(ex.Out, refs)
	}
}

// collectLoopVars records the induction variables of for/foreach loops.
func collectLoopVars(b *Block, out map[string]bool) {
	for _, s := range b.Body {
		switch st := s.(type) {
		case *ForRangeStmt:
			out[st.Var] = true
			collectLoopVars(st.Body, out)
		case *ForEachStmt:
			out[st.Var] = true
			collectLoopVars(st.Body, out)
		case *Block:
			collectLoopVars(st, out)
		case *IfStmt:
			collectLoopVars(st.Then, out)
			if st.Else != nil {
				collectLoopVars(st.Else, out)
			}
		case *WhileStmt:
			collectLoopVars(st.Body, out)
		case *SwitchStmt:
			for _, c := range st.Cases {
				collectLoopVars(c.Body, out)
			}
			if st.Default != nil {
				collectLoopVars(st.Default, out)
			}
		}
	}
}

// collectLabelNames records source label names (resume labels avoid them).
func collectLabelNames(b *Block) map[string]bool {
	out := map[string]bool{}
	var walk func(blk *Block)
	walk = func(blk *Block) {
		for _, s := range blk.Body {
			switch st := s.(type) {
			case *LabelStmt:
				out[st.Name] = true
			case *Block:
				walk(st)
			case *IfStmt:
				walk(st.Then)
				if st.Else != nil {
					walk(st.Else)
				}
			case *WhileStmt:
				walk(st.Body)
			case *ForRangeStmt:
				walk(st.Body)
			case *ForEachStmt:
				walk(st.Body)
			case *SwitchStmt:
				for _, c := range st.Cases {
					walk(c.Body)
				}
				if st.Default != nil {
					walk(st.Default)
				}
			}
		}
	}
	walk(b)
	return out
}

func cloneNameSet(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func unionNameSets(sets ...map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, s := range sets {
		for k, v := range s {
			if v {
				out[k] = true
			}
		}
	}
	return out
}

func nameSetsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
