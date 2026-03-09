// Package hirwat compiles a HIR module to WebAssembly Text format (WAT).
//
// HIR is structured (if/while/for) — WAT is structured (block/loop/if).
// The mapping is nearly 1:1, which is why HIR→WAT is simpler than MIR2→WAT.
//
// All numeric types (u8, u16, i8, i16, bool) map to i32.
// Pointer types map to i32 (linear memory offset, 32-bit WASM).
// u8 values are masked with (i32.and 0xFF) at assignment to preserve semantics.
//
// Every HIR function is exported by its name so wasmtime can call it directly:
//
//	wasmtime run --invoke funcname out.wasm arg1 arg2
package hirwat

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Compile generates WAT text for the given HIR module.
func Compile(m *hir.Module) (string, error) {
	g := &gen{sb: &strings.Builder{}}
	g.compileModule(m)
	return g.sb.String(), nil
}

// ── Generator ─────────────────────────────────────────────────────────────────

type gen struct {
	sb      *strings.Builder
	indent  int
	loopCtx []loopLabels // stack for break/continue
	locals  map[string]mir2.Ty
	labelN  int
}

type loopLabels struct {
	brk  string // label to branch to on break  (the outer block)
	cont string // label to branch to on continue (the inner loop)
}

func (g *gen) freshLabel(prefix string) string {
	g.labelN++
	return fmt.Sprintf("$%s_%d", prefix, g.labelN)
}

func (g *gen) emit(s string) {
	fmt.Fprintf(g.sb, "%s%s\n", strings.Repeat("  ", g.indent), s)
}

func (g *gen) emitf(format string, args ...any) {
	g.emit(fmt.Sprintf(format, args...))
}

// ── Module ────────────────────────────────────────────────────────────────────

func (g *gen) compileModule(m *hir.Module) {
	g.emit("(module")
	g.indent++

	// Data segments for globals
	if len(m.Globals) > 0 {
		// linear memory — 1 page (64KB) is enough for our tests
		g.emit("(memory (export \"memory\") 1)")
	}

	// Forward-declare all functions so mutual recursion works
	for _, f := range m.Funcs {
		if !f.IsExtern {
			g.compileFunc(f)
		}
	}

	g.indent--
	g.emit(")")
}

// ── Function ──────────────────────────────────────────────────────────────────

func (g *gen) compileFunc(f *hir.Func) {
	// Build param list
	var params []string
	for _, p := range f.Params {
		params = append(params, fmt.Sprintf("(param $%s i32)", watIdent(p.Name)))
	}

	// Return type
	result := ""
	if f.RetTy != mir2.TyVoid {
		result = " (result i32)"
	}

	paramStr := ""
	if len(params) > 0 {
		paramStr = " " + strings.Join(params, " ")
	}

	g.emitf("(func $%s (export %q)%s%s", watIdent(f.Name), f.Name, paramStr, result)
	g.indent++

	// Collect all locals declared in the body
	g.locals = make(map[string]mir2.Ty)
	collectLocals(f.Body, g.locals)
	// Params shadow locals — remove them
	for _, p := range f.Params {
		delete(g.locals, p.Name)
	}
	// Emit local declarations
	for name, ty := range g.locals {
		_ = ty // all i32
		g.emitf("(local $%s i32)", watIdent(name))
	}

	g.compileBlock(f.Body)

	// Void functions need an explicit end
	if f.RetTy == mir2.TyVoid {
		g.emit("return")
	}

	g.indent--
	g.emit(")")
}

// collectLocals walks the block and collects VarDeclStmt names.
func collectLocals(b *hir.Block, out map[string]mir2.Ty) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		switch st := s.(type) {
		case *hir.VarDeclStmt:
			out[st.Name] = st.Ty
			collectLocals(nil, out) // no nested block in decl
		case *hir.IfStmt:
			collectLocals(st.Then, out)
			collectLocals(st.Else, out)
		case *hir.WhileStmt:
			collectLocals(st.Body, out)
		case *hir.ForRangeStmt:
			out[st.Var] = mir2.TyU8
			collectLocals(st.Body, out)
		case *hir.ForEachStmt:
			out[st.Var] = st.ElemTy
			collectLocals(st.Body, out)
		case *hir.SwitchStmt:
			for _, c := range st.Cases {
				collectLocals(c.Body, out)
			}
			collectLocals(st.Default, out)
		case *hir.Block:
			collectLocals(st, out)
		}
	}
}

// ── Block ─────────────────────────────────────────────────────────────────────

func (g *gen) compileBlock(b *hir.Block) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		g.compileStmt(s)
	}
}

// ── Statements ────────────────────────────────────────────────────────────────

func (g *gen) compileStmt(s hir.Stmt) {
	switch st := s.(type) {

	case *hir.VarDeclStmt:
		if st.Init != nil {
			val := g.compileExpr(st.Init)
			g.emitf("(local.set $%s %s)", watIdent(st.Name), masked(val, st.Ty))
		}
		// else: zero-initialized by default (WASM locals start at 0)

	case *hir.AssignStmt:
		g.compileAssign(st.Target, st.Val)

	case *hir.StoreStmt:
		ptr := g.compileExpr(st.Ptr)
		val := g.compileExpr(st.Val)
		g.emitf("(i32.store8 %s %s)", ptr, masked(val, mir2.TyU8))

	case *hir.ReturnStmt:
		if st.Val != nil {
			val := g.compileExpr(st.Val)
			g.emitf("(return %s)", val)
		} else {
			g.emit("(return)")
		}

	case *hir.IfStmt:
		cond := g.compileExpr(st.Cond)
		g.emitf("(if %s", cond)
		g.indent++
		g.emit("(then")
		g.indent++
		g.compileBlock(st.Then)
		g.indent--
		g.emit(")")
		if st.Else != nil {
			g.emit("(else")
			g.indent++
			g.compileBlock(st.Else)
			g.indent--
			g.emit(")")
		}
		g.indent--
		g.emit(")")

	case *hir.WhileStmt:
		brk := g.freshLabel("brk")
		cont := g.freshLabel("cont")
		g.loopCtx = append(g.loopCtx, loopLabels{brk: brk, cont: cont})

		g.emitf("(block %s", brk)
		g.indent++
		g.emitf("(loop %s", cont)
		g.indent++
		// break if condition is false
		cond := g.compileExpr(st.Cond)
		g.emitf("(br_if %s (i32.eqz %s))", brk, cond)
		g.compileBlock(st.Body)
		g.emitf("(br %s)", cont)
		g.indent--
		g.emit(")")
		g.indent--
		g.emit(")")

		g.loopCtx = g.loopCtx[:len(g.loopCtx)-1]

	case *hir.ForRangeStmt:
		// for i in start..end { body }
		// → i = start; while i < end { body; i++ }
		brk := g.freshLabel("brk")
		cont := g.freshLabel("cont")
		g.loopCtx = append(g.loopCtx, loopLabels{brk: brk, cont: cont})

		startVal := g.compileExpr(st.Start)
		g.emitf("(local.set $%s %s)", watIdent(st.Var), startVal)

		g.emitf("(block %s", brk)
		g.indent++
		g.emitf("(loop %s", cont)
		g.indent++
		endVal := g.compileExpr(st.End)
		iVal := fmt.Sprintf("(local.get $%s)", watIdent(st.Var))
		cond := fmt.Sprintf("(i32.lt_u %s %s)", iVal, endVal)
		g.emitf("(br_if %s (i32.eqz %s))", brk, cond)
		g.compileBlock(st.Body)
		// i++
		g.emitf("(local.set $%s (i32.add (local.get $%s) (i32.const 1)))",
			watIdent(st.Var), watIdent(st.Var))
		g.emitf("(br %s)", cont)
		g.indent--
		g.emit(")")
		g.indent--
		g.emit(")")

		g.loopCtx = g.loopCtx[:len(g.loopCtx)-1]

	case *hir.BreakStmt:
		if len(g.loopCtx) == 0 {
			g.emit(";; break outside loop — unreachable")
			return
		}
		g.emitf("(br %s)", g.loopCtx[len(g.loopCtx)-1].brk)

	case *hir.ContinueStmt:
		if len(g.loopCtx) == 0 {
			g.emit(";; continue outside loop — unreachable")
			return
		}
		g.emitf("(br %s)", g.loopCtx[len(g.loopCtx)-1].cont)

	case *hir.SwitchStmt:
		g.compileSwitch(st)

	case *hir.ExprStmt:
		// Side-effect call — result dropped
		expr := g.compileExprRaw(st.Expr)
		if st.Expr.ExprTy() == mir2.TyVoid {
			g.emitf("%s", expr)
		} else {
			g.emitf("(drop %s)", expr)
		}

	case *hir.Block:
		g.compileBlock(st)

	case *hir.ForEachStmt:
		// Simplified: treat as while loop with pointer + counter
		// ptr advances by 1 byte per iteration
		brk := g.freshLabel("brk")
		cont := g.freshLabel("cont")
		g.loopCtx = append(g.loopCtx, loopLabels{brk: brk, cont: cont})

		ptrLocal := g.freshLabel("ptr")[1:] // strip $
		cntLocal := g.freshLabel("cnt")[1:]
		g.emitf("(local $%s i32)", ptrLocal)
		g.emitf("(local $%s i32)", cntLocal)

		ptrVal := g.compileExpr(st.Ptr)
		lenVal := g.compileExpr(st.Len)
		g.emitf("(local.set $%s %s)", ptrLocal, ptrVal)
		g.emitf("(local.set $%s %s)", cntLocal, lenVal)

		g.emitf("(block %s", brk)
		g.indent++
		g.emitf("(loop %s", cont)
		g.indent++
		g.emitf("(br_if %s (i32.eqz (local.get $%s)))", brk, cntLocal)
		// load element
		g.emitf("(local.set $%s (i32.load8_u (local.get $%s)))",
			watIdent(st.Var), ptrLocal)
		g.compileBlock(st.Body)
		// advance ptr, decrement counter
		g.emitf("(local.set $%s (i32.add (local.get $%s) (i32.const 1)))",
			ptrLocal, ptrLocal)
		g.emitf("(local.set $%s (i32.sub (local.get $%s) (i32.const 1)))",
			cntLocal, cntLocal)
		g.emitf("(br %s)", cont)
		g.indent--
		g.emit(")")
		g.indent--
		g.emit(")")

		g.loopCtx = g.loopCtx[:len(g.loopCtx)-1]
	}
}

func (g *gen) compileAssign(target hir.Expr, val hir.Expr) {
	switch t := target.(type) {
	case *hir.VarRefExpr:
		v := g.compileExpr(val)
		g.emitf("(local.set $%s %s)", watIdent(t.Name), masked(v, t.Ty))
	case *hir.DerefExpr:
		ptr := g.compileExpr(t.Ptr)
		v := g.compileExpr(val)
		g.emitf("(i32.store8 %s %s)", ptr, masked(v, mir2.TyU8))
	default:
		g.emit(";; unsupported assign target")
	}
}

func (g *gen) compileSwitch(st *hir.SwitchStmt) {
	// Lower switch to chain of if/else
	val := g.compileExpr(st.Val)
	// Store switch val in a temp local to avoid re-evaluation
	tmp := g.freshLabel("sw")[1:]
	g.emitf("(local $%s i32)", tmp)
	g.emitf("(local.set $%s %s)", tmp, val)

	// Build nested if-else
	for _, c := range st.Cases {
		cond := fmt.Sprintf("(i32.eq (local.get $%s) (i32.const %d))", tmp, c.Val)
		g.emitf("(if %s", cond)
		g.indent++
		g.emit("(then")
		g.indent++
		g.compileBlock(c.Body)
		g.indent--
		g.emit(")")
		g.indent--
		g.emit(")")
	}
	if st.Default != nil {
		g.compileBlock(st.Default)
	}
}

// ── Expressions ───────────────────────────────────────────────────────────────

// compileExpr returns a WAT expression string (inline, not emitted).
func (g *gen) compileExpr(e hir.Expr) string {
	return g.compileExprRaw(e)
}

func (g *gen) compileExprRaw(e hir.Expr) string {
	switch ex := e.(type) {

	case *hir.IntLitExpr:
		return fmt.Sprintf("(i32.const %d)", ex.Val)

	case *hir.BoolLitExpr:
		if ex.Val {
			return "(i32.const 1)"
		}
		return "(i32.const 0)"

	case *hir.VarRefExpr:
		return fmt.Sprintf("(local.get $%s)", watIdent(ex.Name))

	case *hir.BinExpr:
		return g.compileBinExpr(ex)

	case *hir.UnaryExpr:
		return g.compileUnary(ex)

	case *hir.CallExpr:
		var args []string
		for _, a := range ex.Args {
			args = append(args, g.compileExpr(a))
		}
		argStr := ""
		if len(args) > 0 {
			argStr = " " + strings.Join(args, " ")
		}
		return fmt.Sprintf("(call $%s%s)", watIdent(ex.Fn), argStr)

	case *hir.DerefExpr:
		ptr := g.compileExpr(ex.Ptr)
		return fmt.Sprintf("(i32.load8_u %s)", ptr)

	case *hir.IndexExpr:
		base := g.compileExpr(ex.Base)
		idx := g.compileExpr(ex.Idx)
		// addr = base + idx (byte stride assumed; larger strides handled separately)
		addr := fmt.Sprintf("(i32.add %s %s)", base, idx)
		return fmt.Sprintf("(i32.load8_u %s)", addr)

	case *hir.AddrOfExpr:
		// Global variable address — emit as i32.const of the symbol offset.
		// For now we use 0 as a placeholder (Phase 2: real linker).
		return fmt.Sprintf("(i32.const 0) ;; addr_of %s", ex.Sym)

	case *hir.FieldExpr:
		base := g.compileExpr(ex.X)
		if ex.Offset == 0 {
			return base
		}
		return fmt.Sprintf("(i32.add %s (i32.const %d))", base, ex.Offset)

	case *hir.CastExpr:
		inner := g.compileExpr(ex.X)
		return masked(inner, ex.Ty)
	}

	return "(i32.const 0) ;; unknown expr"
}

func (g *gen) compileBinExpr(ex *hir.BinExpr) string {
	l := g.compileExpr(ex.L)
	r := g.compileExpr(ex.R)
	signed := isSigned(ex.Ty)

	var op string
	switch ex.Op {
	case "+":
		op = "i32.add"
	case "-":
		op = "i32.sub"
	case "*":
		op = "i32.mul"
	case "/":
		if signed {
			op = "i32.div_s"
		} else {
			op = "i32.div_u"
		}
	case "%":
		if signed {
			op = "i32.rem_s"
		} else {
			op = "i32.rem_u"
		}
	case "&":
		op = "i32.and"
	case "|":
		op = "i32.or"
	case "^":
		op = "i32.xor"
	case "<<":
		op = "i32.shl"
	case ">>":
		if signed {
			op = "i32.shr_s"
		} else {
			op = "i32.shr_u"
		}
	case "==":
		op = "i32.eq"
	case "!=":
		op = "i32.ne"
	case "<":
		if signed {
			op = "i32.lt_s"
		} else {
			op = "i32.lt_u"
		}
	case "<=":
		if signed {
			op = "i32.le_s"
		} else {
			op = "i32.le_u"
		}
	case ">":
		if signed {
			op = "i32.gt_s"
		} else {
			op = "i32.gt_u"
		}
	case ">=":
		if signed {
			op = "i32.ge_s"
		} else {
			op = "i32.ge_u"
		}
	default:
		return fmt.Sprintf("(i32.const 0) ;; unknown op %s", ex.Op)
	}
	return fmt.Sprintf("(%s %s %s)", op, l, r)
}

func (g *gen) compileUnary(ex *hir.UnaryExpr) string {
	x := g.compileExpr(ex.X)
	switch ex.Op {
	case "-":
		return fmt.Sprintf("(i32.sub (i32.const 0) %s)", x)
	case "~":
		return fmt.Sprintf("(i32.xor %s (i32.const -1))", x)
	case "!":
		return fmt.Sprintf("(i32.eqz %s)", x)
	}
	return fmt.Sprintf("(i32.const 0) ;; unknown unary %s", ex.Op)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// masked wraps a WAT expression with an AND mask for u8/u16 types.
func masked(expr string, ty mir2.Ty) string {
	switch ty {
	case mir2.TyU8, mir2.TyI8, mir2.TyBool:
		return fmt.Sprintf("(i32.and %s (i32.const 255))", expr)
	case mir2.TyU16, mir2.TyI16:
		return fmt.Sprintf("(i32.and %s (i32.const 65535))", expr)
	}
	return expr
}

// isSigned returns true for signed integer types.
func isSigned(ty mir2.Ty) bool {
	return ty == mir2.TyI8 || ty == mir2.TyI16
}

// watIdent sanitizes a name for use as a WAT identifier.
func watIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
