package hir

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// Verify checks the structural and resolved-type contract consumed by lowering.
// Numeric width conversions and legacy integer addresses are intentionally legal.
// It does not prove equivalence, closure capture safety, or all-paths return.
func Verify(m *Module) error { return verifyHIRModule(m, false, false) }

// VerifyClosedNames also requires explicit local declarations and resolvable
// address symbols (parameters, locals, module globals/functions or strings).
// Importers can verify an isolated module with no globals before linking it,
// ensuring free writes cannot acquire a different meaning in a parent module.
func VerifyClosedNames(m *Module) error { return verifyHIRModule(m, false, true) }

// ResolveCallSignatures verifies HIR and records signatures for statically
// resolved indirect calls. Unknown raw pointers remain explicitly unresolved.
func ResolveCallSignatures(m *Module) error { return verifyHIRModule(m, true, false) }

func verifyHIRModule(m *Module, annotate, closedNames bool) error {
	if m == nil {
		return fmt.Errorf("HIR: nil module")
	}
	v := &hirVerifier{funcs: map[string]*Func{}, globals: map[string]bool{}, stringSymbols: map[string]bool{}, active: map[any]bool{}, aliases: map[string]*Func{}, annotateCalls: annotate, closedNames: closedNames, gens: map[string]*Func{}}
	for i := range m.Strings {
		v.stringSymbols[fmt.Sprintf("@mir2.str.%d", i)] = true
	}
	for _, g := range m.Globals {
		if g.Name == "" || v.globals[g.Name] || nilHIR(g.Ty) {
			return fmt.Errorf("HIR: invalid or duplicate global %q", g.Name)
		}
		v.globals[g.Name] = true
	}
	for _, f := range m.Funcs {
		if f == nil {
			return fmt.Errorf("HIR: nil function")
		}
		if f.Name == "" || v.funcs[f.Name] != nil {
			return fmt.Errorf("HIR: invalid or duplicate function %q", f.Name)
		}
		v.funcs[f.Name] = f
		if f.IsGen {
			if nilHIR(f.ElemTy) {
				return fmt.Errorf("HIR gen %s: missing element type", f.Name)
			}
			v.gens[f.Name] = f
		}
	}
	setters, err := smcAccessors(m)
	if err != nil {
		return err
	}
	for _, f := range setters {
		v.funcs[f.Name] = f
	}
	v.smcSetters = map[string]bool{}
	for _, f := range setters {
		v.smcSetters[f.Name] = true
	}
	// Iterate to propagate consumed callback contracts through forwarding wrappers.
	v.callbackParams = map[*Func]map[int]*CallSignature{}
	for {
		v.callbackChanged = false
		for _, f := range m.Funcs {
			v.callbackOrigins = map[string]int{}
			parameterMutations := mutableHIRNames(f.Body)
			for i, p := range f.Params {
				if !parameterMutations[p.Name] && mir2.IsPointer(p.Ty) {
					v.callbackOrigins[p.Name] = i
				}
			}
			v.fn = f
			v.labels = map[string]bool{}
			v.gotos = nil
			env := map[string]mir2.Ty{}
			if nilHIR(f.RetTy) {
				return v.fail("missing return type")
			}
			for _, ty := range f.RetTys {
				if nilHIR(ty) {
					return v.fail("missing tuple return type")
				}
			}
			for _, p := range f.Params {
				if p.Name == "" || env[p.Name] != nil || nilHIR(p.Ty) || p.Ty == mir2.TyVoid {
					return v.fail("invalid or duplicate parameter %q", p.Name)
				}
				env[p.Name] = p.Ty
			}
			for _, c := range f.Captures {
				if c.Name == "" || env[c.Name] != nil || nilHIR(c.Ty) || c.Ty == mir2.TyVoid || v.globals[c.Name] || v.funcs[c.Name] != nil {
					return v.fail("invalid or duplicate capture %q", c.Name)
				}
				env[c.Name] = c.Ty
			}
			v.aliases = map[string]*Func{}
			v.immutableAliases = map[string]bool{}
			v.mutableNames = mutableHIRNames(f.Body)
			if f.IsExtern {
				if len(f.Signature().StructReturnBuffers) > 0 {
					return v.fail("extern struct return requires explicit caller-buffer ABI")
				}
				continue
			}
			if err := v.block(f.Body, env, 0); err != nil {
				return err
			}
			if err := verifyGotoBindings(f); err != nil {
				return v.fail("%s", err)
			}
			for _, name := range v.gotos {
				if !v.labels[name] {
					return v.fail("unknown label %q", name)
				}
			}
		}
		if !v.callbackChanged {
			break
		}
	}
	return verifySMCRecursion(m)
}

type hirVerifier struct {
	stringSymbols    map[string]bool
	closedNames      bool
	smcSetters       map[string]bool
	funcs            map[string]*Func
	gens             map[string]*Func // generator declarations (IsGen)
	globals          map[string]bool
	fn               *Func
	labels           map[string]bool
	gotos            []string
	active           map[any]bool
	aliases          map[string]*Func
	immutableAliases map[string]bool
	mutableNames     map[string]bool
	captureOuter     map[string]mir2.Ty
	collected        map[string]mir2.Ty
	annotateCalls    bool
	callbackParams   map[*Func]map[int]*CallSignature
	callbackOrigins  map[string]int
	callbackChanged  bool
	tupleCall        *CallExpr
}

func nilHIR(x any) bool {
	return x == nil || reflect.ValueOf(x).Kind() == reflect.Ptr && reflect.ValueOf(x).IsNil()
}
func (v *hirVerifier) fail(format string, args ...any) error {
	return fmt.Errorf("HIR %s: %s", v.fn.Name, fmt.Sprintf(format, args...))
}
func copyHIRScope(env map[string]mir2.Ty) map[string]mir2.Ty {
	out := map[string]mir2.Ty{}
	for n, t := range env {
		out[n] = t
	}
	return out
}
func (v *hirVerifier) enter(x any) error {
	if nilHIR(x) {
		return v.fail("missing %T", x)
	}
	if v.active[x] {
		return v.fail("cyclic %T", x)
	}
	v.active[x] = true
	return nil
}
func (v *hirVerifier) block(b *Block, outer map[string]mir2.Ty, loops int) error {
	if err := v.enter(b); err != nil {
		return err
	}
	defer delete(v.active, b)
	if v.mutableNames == nil {
		v.mutableNames = mutableHIRNames(b) // InferCaptures also enters through block.
	}
	env := copyHIRScope(outer)
	oldOrigins := v.callbackOrigins
	v.callbackOrigins = map[string]int{}
	for n, i := range oldOrigins {
		v.callbackOrigins[n] = i
	}
	defer func() { v.callbackOrigins = oldOrigins }()
	oldAliases := v.aliases
	oldImmutable := v.immutableAliases
	v.immutableAliases = map[string]bool{}
	for n, immutable := range oldImmutable {
		v.immutableAliases[n] = immutable
	}
	v.aliases = map[string]*Func{}
	for n, f := range oldAliases {
		v.aliases[n] = f
	}
	defer func() { v.aliases = oldAliases; v.immutableAliases = oldImmutable }()
	for i, s := range b.Body {
		if err := v.stmt(s, env, loops); err != nil {
			return fmt.Errorf("statement %d: %w", i, err)
		}
	}
	return nil
}
func (v *hirVerifier) exprs(env map[string]mir2.Ty, xs ...Expr) error {
	for _, x := range xs {
		if err := v.expr(x, env); err != nil {
			return err
		}
	}
	return nil
}
func (v *hirVerifier) bind(env map[string]mir2.Ty, n string, ty mir2.Ty) error {
	if n == "" || nilHIR(ty) || ty == mir2.TyVoid {
		return v.fail("invalid binding %q", n)
	}
	delete(v.callbackOrigins, n)
	env[n] = ty
	return nil
}

func rawHIRPointer(ty mir2.Ty) bool { return mir2.IsPointer(ty) && ty == mir2.PtrFor(ty.Width()) }

// assignableHIR reflects current implicit numeric conversions. Raw pointers and
// integer addresses remain an explicit legacy compatibility boundary.
func assignableHIR(dst, src mir2.Ty) bool {
	if nilHIR(dst) || nilHIR(src) {
		return false
	}
	if dst == src {
		return true
	}
	dst, src = mir2.BaseOf(dst), mir2.BaseOf(src)
	if mir2.IsInt(dst) && mir2.IsInt(src) {
		return true
	}
	if mir2.IsPointer(dst) && mir2.IsPointer(src) {
		return rawHIRPointer(dst) || rawHIRPointer(src) || dst.String() == src.String()
	}
	if mir2.IsPointer(dst) && mir2.IsInt(src) || mir2.IsInt(dst) && mir2.IsPointer(src) {
		return true
	}
	if mir2.IsPointer(dst) {
		switch src.(type) {
		case *mir2.StructTy, *mir2.ArrayTy:
			return true
		}
	}
	// Struct expressions are lowered as addresses; HIR does not yet distinguish
	// address-valued struct locals from aggregate values.
	if _, ok := dst.(*mir2.StructTy); ok {
		return mir2.IsPointer(src)
	}
	return dst.String() == src.String()
}
func (v *hirVerifier) valueAs(e Expr, ty mir2.Ty, env map[string]mir2.Ty) error {
	if err := v.expr(e, env); err != nil {
		return err
	}
	if !assignableHIR(ty, e.ExprTy()) {
		return v.fail("value type %s incompatible with %s", e.ExprTy(), ty)
	}
	return nil
}
func (v *hirVerifier) address(e Expr, env map[string]mir2.Ty) error {
	if err := v.expr(e, env); err != nil {
		return err
	}
	if vr, ok := e.(*VarRefExpr); ok {
		if _, array := env[vr.Name].(*mir2.ArrayTy); array {
			return nil
		}
	}
	ty := mir2.BaseOf(e.ExprTy())
	if mir2.IsPointer(ty) || mir2.IsInt(ty) && ty.Width() >= 16 {
		return nil
	}
	if _, ok := ty.(*mir2.ArrayTy); ok {
		return nil
	}
	if _, ok := ty.(*mir2.StructTy); ok {
		return nil
	}
	return v.fail("non-address type %s", ty)
}
func (v *hirVerifier) condition(e Expr, env map[string]mir2.Ty) error {
	if err := v.expr(e, env); err != nil {
		return err
	}
	ty := mir2.BaseOf(e.ExprTy())
	if !mir2.IsInt(ty) && !mir2.IsPointer(ty) {
		return v.fail("non-scalar condition %s", ty)
	}
	return nil
}
func (v *hirVerifier) pointee(e Expr, ty mir2.Ty, env map[string]mir2.Ty) error {
	if err := v.address(e, env); err != nil {
		return err
	}
	ptr := e.ExprTy()
	if mir2.IsPointer(ptr) && !rawHIRPointer(ptr) {
		elem := mir2.PointerElem(ptr)
		if elem != nil && elem.String() != ty.String() {
			return v.fail("pointee %s does not match access type %s", elem, ty)
		}
	}
	return nil
}
func (v *hirVerifier) stmt(s Stmt, env map[string]mir2.Ty, loops int) error {
	if nilHIR(s) {
		return v.fail("nil statement")
	}
	// Blocks have their own cycle guard.
	if b, ok := s.(*Block); ok {
		return v.block(b, env, loops)
	}
	if err := v.enter(s); err != nil {
		return err
	}
	defer delete(v.active, s)
	switch s := s.(type) {
	case *VarDeclStmt:
		if s.ArrayLen < 0 {
			return v.fail("negative array length")
		}
		if s.Init != nil {
			if err := v.expr(s.Init, env); err != nil {
				return err
			}
		}
		for _, init := range s.Initial {
			if init != nil {
				if err := v.expr(init, env); err != nil {
					return err
				}
			}
		}
		f := v.functionValue(s.Init)
		immutable := false
		if ref, ok := s.Init.(*VarRefExpr); ok {
			// These are the NoReg aliases recognized by lowering. An
			// address-of initializer instead creates a mutable pointer register.
			immutable = v.funcs[ref.Name] != nil || v.immutableAliases[ref.Name]
		}
		if f != nil && (immutable || !v.mutableNames[s.Name]) {
			v.aliases[s.Name] = f
			v.immutableAliases[s.Name] = immutable
		} else {
			if f != nil && len(f.Captures) > 0 && v.captureOuter == nil {
				return v.fail("capturing callable %s cannot escape through a mutable pointer", f.Name)
			}
			delete(v.aliases, s.Name)
			delete(v.immutableAliases, s.Name)
		}
		ty := s.Ty
		if s.ArrayLen > 0 {
			if nilHIR(ty) {
				return v.fail("missing array element type")
			}
			ty = &mir2.ArrayTy{Elem: ty, Len: s.ArrayLen}
		}
		origin, callbackAlias := -1, false
		if ref, ok := s.Init.(*VarRefExpr); ok && !v.mutableNames[s.Name] {
			origin, callbackAlias = v.callbackOrigins[ref.Name]
		}
		if err := v.bind(env, s.Name, ty); err != nil {
			return err
		}
		if callbackAlias && mir2.IsPointer(ty) {
			v.callbackOrigins[s.Name] = origin
		}
		return nil
	case *AssignStmt:
		if bit, ok := s.Target.(*BitExpr); ok {
			switch bit.X.(type) {
			case *VarRefExpr, *FieldExpr, *IndexExpr, *LoadExpr, *DerefExpr:
			default:
				return v.fail("bit assignment requires an addressable scalar base")
			}
		}
		if vr, ok := s.Target.(*VarRefExpr); ok && v.immutableAliases[vr.Name] {
			return v.fail("reassignment of callable alias %q is unsupported", vr.Name)
		}
		if f := v.functionValue(s.Val); f != nil && len(f.Captures) > 0 && v.captureOuter == nil {
			return v.fail("capturing callable %s cannot escape through assignment", f.Name)
		}
		switch s.Target.(type) {
		case *VarRefExpr, *DerefExpr, *FieldExpr, *IndexExpr, *BitExpr:
		default:
			return v.fail("invalid assignment target %T", s.Target)
		}
		// Nanz permits implicit local declaration by assignment. Read the RHS
		// before introducing the binding, so x = x + 1 cannot create itself.
		if vr, ok := s.Target.(*VarRefExpr); ok && env[vr.Name] == nil && v.captureOuter[vr.Name] != nil {
			env[vr.Name] = v.captureOuter[vr.Name]
			v.collected[vr.Name] = env[vr.Name]
		}
		if vr, ok := s.Target.(*VarRefExpr); ok && env[vr.Name] == nil && !v.globals[vr.Name] {
			if v.closedNames {
				return v.fail("assignment to undeclared local %q", vr.Name)
			}
			if err := v.valueAs(s.Val, vr.Ty, env); err != nil {
				return err
			}
			return v.bind(env, vr.Name, vr.Ty)
		}
		if err := v.expr(s.Target, env); err != nil {
			return err
		}
		return v.valueAs(s.Val, s.Target.ExprTy(), env)
	case *ReturnStmt:
		if s.Val != nil && len(s.Vals) > 0 {
			return v.fail("ambiguous return value")
		}
		if len(s.Vals) > 0 {
			if len(s.Vals) != len(v.fn.RetTys) {
				return v.fail("return arity %d, want %d", len(s.Vals), len(v.fn.RetTys))
			}
			return v.exprs(env, s.Vals...)
		}
		if s.Val != nil {
			if f := v.functionValue(s.Val); f != nil && len(f.Captures) > 0 && v.captureOuter == nil {
				return v.fail("capturing callable %s cannot escape through return", f.Name)
			}
			if v.captureOuter != nil {
				return v.expr(s.Val, env)
			}
			return v.valueAs(s.Val, v.fn.RetTy, env)
		}
	case *TupleLetStmt:
		if len(s.Names) != len(s.Tys) {
			return v.fail("tuple names/types length mismatch")
		}
		previousTupleCall := v.tupleCall
		v.tupleCall = s.Call
		err := v.expr(s.Call, env)
		v.tupleCall = previousTupleCall
		if err != nil {
			return err
		}
		if f := v.funcs[s.Call.Fn]; f != nil {
			if len(f.RetTys) != len(s.Names) {
				return v.fail("tuple call result arity mismatch")
			}
			for i, ty := range f.RetTys {
				if !reflect.DeepEqual(ty, s.Tys[i]) {
					return v.fail("tuple call result %d type %s, want %s", i, s.Tys[i], ty)
				}
			}
		}
		for i, n := range s.Names {
			if n != "_" {
				if err := v.bind(env, n, s.Tys[i]); err != nil {
					return err
				}
			}
		}
	case *ExprStmt:
		return v.expr(s.Expr, env)
	case *StoreStmt:
		if err := v.expr(s.Val, env); err != nil {
			return err
		}
		return v.pointee(s.Ptr, s.Val.ExprTy(), env)
	case *IfStmt:
		if err := v.condition(s.Cond, env); err != nil {
			return err
		}
		if err := v.block(s.Then, env, loops); err != nil {
			return err
		}
		if s.Else != nil {
			return v.block(s.Else, env, loops)
		}
	case *WhileStmt:
		if err := v.condition(s.Cond, env); err != nil {
			return err
		}
		return v.block(s.Body, env, loops+1)
	case *ForRangeStmt:
		if err := v.exprs(env, s.Start, s.End); err != nil {
			return err
		}
		scope := copyHIRScope(env)
		if err := v.bind(scope, s.Var, RangeType(s.Start, s.End)); err != nil {
			return err
		}
		return v.block(s.Body, scope, loops+1)
	case *ForEachStmt:
		if s.ElemStride < 0 {
			return v.fail("negative element stride")
		}
		if err := v.exprs(env, s.Ptr, s.Start, s.Len); err != nil {
			return err
		}
		scope := copyHIRScope(env)
		ty := s.ElemTy
		if nilHIR(ty) {
			return v.fail("missing element type")
		}
		if s.PtrIter {
			ty = mir2.TyPtr
		}
		if err := v.bind(scope, s.Var, ty); err != nil {
			return err
		}
		return v.block(s.Body, scope, loops+1)
	case *BreakStmt, *ContinueStmt:
		if loops == 0 {
			return v.fail("%T outside loop", s)
		}
	case *YieldStmt:
		if !v.fn.IsGen {
			return v.fail("yield outside generator")
		}
		if err := v.valueAs(s.Val, v.fn.ElemTy, env); err != nil {
			return err
		}
	case *GenForStmt:
		if v.gens[s.Gen] == nil {
			return v.fail("unknown generator %q", s.Gen)
		}
		if err := v.exprs(env, s.Args...); err != nil {
			return err
		}
		scope := copyHIRScope(env)
		if err := v.bind(scope, s.Var, s.ElemTy); err != nil {
			return err
		}
		return v.block(s.Body, scope, loops+1)
	case *LabelStmt:
		if s.Name == "" || v.labels[s.Name] {
			return v.fail("invalid or duplicate label %q", s.Name)
		}
		v.labels[s.Name] = true
	case *GotoStmt:
		v.gotos = append(v.gotos, s.Label)
	case *SwitchStmt:
		if err := v.expr(s.Val, env); err != nil {
			return err
		}
		cases := map[int64]bool{}
		for _, c := range s.Cases {
			if c == nil || cases[c.Val] {
				return v.fail("nil or duplicate switch case")
			}
			cases[c.Val] = true
			if err := v.block(c.Body, env, loops); err != nil {
				return err
			}
		}
		if s.Default != nil {
			return v.block(s.Default, env, loops)
		}
	case *AsmStmt:
		for _, op := range append(append([]AsmOperand(nil), s.Ins...), s.Outs...) {
			if env[op.Name] == nil && z80RegToClassSet(strings.ToUpper(op.Name)) == 0 {
				return v.fail("unknown asm operand %q", op.Name)
			}
		}
	default:
		return v.fail("unsupported statement %T", s)
	}
	return nil
}
func (v *hirVerifier) expr(e Expr, env map[string]mir2.Ty) error {
	if err := v.enter(e); err != nil {
		return err
	}
	defer delete(v.active, e)
	// LetIn.ExprTy delegates to Body, so validate children before asking its type.
	if x, ok := e.(*LetInExpr); ok {
		if err := v.expr(x.Init, env); err != nil {
			return err
		}
		oldOrigins := v.callbackOrigins
		v.callbackOrigins = map[string]int{}
		for name, index := range oldOrigins {
			v.callbackOrigins[name] = index
		}
		defer func() { v.callbackOrigins = oldOrigins }()
		origin, exists := -1, false
		if ref, ok := x.Init.(*VarRefExpr); ok {
			origin, exists = v.callbackOrigins[ref.Name]
		}
		scope := copyHIRScope(env)
		if err := v.bind(scope, x.Name, x.Ty); err != nil {
			return err
		}
		if exists && mir2.IsPointer(x.Ty) {
			v.callbackOrigins[x.Name] = origin
		}
		return v.expr(x.Body, scope)
	}
	if nilHIR(e.ExprTy()) {
		return v.fail("missing type on %T", e)
	}
	switch e := e.(type) {
	case *IntLitExpr:
		if e.Ty == mir2.TyVoid {
			return v.fail("void integer literal")
		}
	case *BoolLitExpr:
	case *VarRefExpr:
		if ty := env[e.Name]; ty != nil && mir2.IsPointer(ty) && !rawHIRPointer(ty) && e.Ty.String() != ty.String() {
			return v.fail("variable %s type %s, declared %s", e.Name, e.Ty, ty)
		}
		if env[e.Name] == nil && !v.globals[e.Name] && v.funcs[e.Name] == nil {
			if ty := v.captureOuter[e.Name]; ty != nil {
				v.collected[e.Name] = ty
			} else {
				return v.fail("unknown variable %q", e.Name)
			}
		}
		if f := v.funcs[e.Name]; f != nil && env[e.Name] == nil && !v.globals[e.Name] {
			if hasSMCParams(f) {
				return v.fail("SMC function %s requires a direct call; address escape is unsupported", f.Name)
			}
			return v.checkCaptureUse(f, env)
		}
	case *BinExpr:
		if !strings.Contains(" + - * / % & | ^ << >> == != < <= > >= && || ", " "+e.Op+" ") {
			return v.fail("unknown binary operator %q", e.Op)
		}
		if err := v.exprs(env, e.L, e.R); err != nil {
			return err
		}
		if e.L.ExprTy() == mir2.TyVoid || e.R.ExprTy() == mir2.TyVoid || e.Ty == mir2.TyVoid {
			return v.fail("void binary operand or result")
		}
	case *UnaryExpr:
		if e.Op != "-" && e.Op != "!" && e.Op != "~" && e.Op != "+" {
			return v.fail("unknown unary operator %q", e.Op)
		}
		return v.expr(e.X, env)
	case *CallExpr:
		if e.Fn == "" {
			return v.fail("empty call target")
		}
		if f := v.funcs[e.Fn]; f != nil {
			for i, a := range e.Args {
				addressArg := v.smcSetters[e.Fn] || len(e.Args) == len(f.Params) && i < len(f.Params) && f.Params[i].SMC
				if addressArg && a != nil && mir2.BaseOf(a.ExprTy()) == mir2.TyBool {
					return v.fail("SMC address argument cannot be bool")
				}
			}
			if len(e.Args) != len(smcCallParams(f, len(e.Args))) {
				return v.fail("call %s arity %d, want %d", e.Fn, len(e.Args), len(f.Params))
			}
			if e != v.tupleCall && !assignableHIR(e.Ty, f.RetTy) {
				return v.fail("call %s return type %s, declared %s", e.Fn, e.Ty, f.RetTy)
			}
		}
		if err := v.exprs(env, e.Args...); err != nil {
			return err
		}
		f := v.funcs[e.Fn]
		if f == nil {
			f = v.aliases[e.Fn]
		}
		if f != nil {
			if len(f.Captures) > 0 && v.captureOuter == nil {
				return v.fail("capturing callable %s requires an inlined iterator context", f.Name)
			}
			if err := v.checkCaptureUse(f, env); err != nil {
				return err
			}
			if err := v.checkConsumedCallbacks(f, e.Args); err != nil {
				return err
			}
			ret := e.Ty
			if e == v.tupleCall {
				ret = mir2.TyVoid
				if len(f.RetTys) > 0 {
					ret = f.RetTys[0]
				}
			}
			callFunc := *f
			callFunc.Params = smcCallParams(f, len(e.Args))
			return v.checkSignature(callFunc.Signature(), e.Args, ret, env)
		}
		return nil
	case *CallIndirectExpr:
		if hasSMCParams(v.fn) {
			return v.fail("indirect calls inside SMC functions are unsupported")
		}
		if err := v.expr(e.FnPtr, env); err != nil {
			return err
		}
		if err := v.exprs(env, e.Args...); err != nil {
			return err
		}
		sig := e.Signature
		if ref, ok := e.FnPtr.(*VarRefExpr); ok {
			if index, exists := v.callbackOrigins[ref.Name]; exists && v.callbackParams != nil {
				consumed := &CallSignature{}
				for _, a := range e.Args {
					consumed.Params = append(consumed.Params, a.ExprTy())
				}
				if e.Ty != mir2.TyVoid {
					consumed.Returns = []mir2.Ty{e.Ty}
				}
				consumed.ResolveStructReturnABI()
				if sig != nil && len(sig.Params) != len(e.Args) {
					return v.fail("call arity %d, want %d", len(e.Args), len(sig.Params))
				}
				if sig != nil && !sameHIRSignature(sig, consumed) {
					return v.fail("callback invocation disagrees with explicit signature; ABI adapter required")
				}
				if err := v.requireCallback(v.fn, index, consumed); err != nil {
					return err
				}
			}
		}
		if f := v.functionValue(e.FnPtr); f != nil {
			if len(f.Captures) > 0 && v.captureOuter == nil {
				return v.fail("capturing callable %s requires an inlined iterator context", f.Name)
			}
			if err := v.checkConsumedCallbacks(f, e.Args); err != nil {
				return err
			}
			resolved := f.Signature()
			if sig != nil && !sameHIRSignature(sig, resolved) {
				return v.fail("indirect call signature disagrees with target %s", f.Name)
			}
			if v.annotateCalls {
				e.Signature = resolved
			}
			sig = resolved
			if err := v.checkCaptureUse(f, env); err != nil {
				return err
			}
		}
		if sig != nil {
			return v.checkSignature(sig, e.Args, e.Ty, env)
		}
		return nil
	case *FieldExpr:
		if e.Offset < 0 {
			return v.fail("negative field offset")
		}
		return v.expr(e.X, env)
	case *BitExpr:
		if err := v.expr(e.X, env); err != nil {
			return err
		}
		if e.Bit < 0 || e.Bit >= e.X.ExprTy().Width() {
			return v.fail("bit index out of range")
		}
	case *AddrOfExpr:
		if !mir2.IsPointer(e.ExprTy()) {
			return v.fail("address-of has non-pointer type")
		}
		if e.X != nil {
			return v.expr(e.X, env)
		}
		if e.Sym == "" {
			return v.fail("empty address symbol")
		}
		if f := v.funcs[e.Sym]; f != nil {
			if hasSMCParams(f) {
				return v.fail("SMC function %s address escape is unsupported", f.Name)
			}
			return v.checkCaptureUse(f, env)
		}
		if v.closedNames && !v.globals[e.Sym] && env[e.Sym] == nil && !v.stringSymbols[e.Sym] {
			return v.fail("unknown address symbol %q", e.Sym)
		}
	case *LoadExpr:
		return v.pointee(e.Ptr, e.Ty, env)
	case *DerefExpr:
		return v.pointee(e.Ptr, e.Ty, env)
	case *CastExpr:
		return v.expr(e.X, env)
	case *ConstPtrExpr:
		if nilHIR(e.ElemTy) {
			return v.fail("missing constant pointer element type")
		}
	case *IndexExpr:
		if e.ElemStride < 0 {
			return v.fail("negative index stride")
		}
		if err := v.pointee(e.Base, e.ElemTy, env); err != nil {
			return err
		}
		if err := v.expr(e.Idx, env); err != nil {
			return err
		}
		if !mir2.IsInt(mir2.BaseOf(e.Idx.ExprTy())) {
			return v.fail("non-integer index")
		}
	case *StructLitExpr:
		if e.St == nil {
			return v.fail("missing struct type")
		}
		for _, f := range e.Fields {
			if err := v.expr(f.Val, env); err != nil {
				return err
			}
		}
	case *RangeSourceExpr:
		return v.exprs(env, e.Lo, e.Hi)
	case *CondExpr:
		if err := v.condition(e.Cond, env); err != nil {
			return err
		}
		if err := v.valueAs(e.Then, e.Ty, env); err != nil {
			return err
		}
		return v.valueAs(e.Else, e.Ty, env)
	case *GenMakeExpr:
		if g := v.gens[e.Gen]; g != nil {
			if len(e.Args) != len(g.Params) {
				return v.fail("generator %s: %d creation arguments, want %d", e.Gen, len(e.Args), len(g.Params))
			}
			for i, arg := range e.Args {
				if err := v.valueAs(arg, g.Params[i].Ty, env); err != nil {
					return err
				}
			}
			return nil
		}
		return v.fail("unknown generator %q", e.Gen)
	case *GenNextExpr:
		if g := v.gens[e.Gen]; g != nil {
			if !mir2.IsPointer(e.Frame.ExprTy()) {
				return v.fail("pull_next frame is not a pointer")
			}
			if err := v.expr(e.Frame, env); err != nil {
				return err
			}
			return v.pointee(e.Out, g.ElemTy, env)
		}
		return v.fail("unknown generator %q", e.Gen)
	default:
		return v.fail("unsupported expression %T", e)
	}
	return nil
}

// verifyGraceModule validates HIR before invoking the panic-based lowerer.
func verifyGraceModule(m *Module) error {
	if err := Verify(m); err != nil {
		return err
	}
	return mir2.Verify(LowerModule(m))
}

// mutableHIRNames deliberately merges lexical names conservatively. A pointer
// assigned on any path (including a loop/backward goto) must never acquire a
// declaration-target signature. Taking its address also permits indirect writes.
// This is not flow-sensitive pointer analysis; unmodified pointers retain the
// existing known-target checks while ambiguous pointers remain unresolved.
func mutableHIRNames(root *Block) map[string]bool {
	out := map[string]bool{}
	seen := map[any]bool{}
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Interface:
			if !value.IsNil() {
				walk(value.Elem())
			}
		case reflect.Ptr:
			if value.IsNil() {
				return
			}
			node := value.Interface()
			if seen[node] {
				return
			}
			seen[node] = true
			switch x := node.(type) {
			case *AssignStmt:
				if ref, ok := x.Target.(*VarRefExpr); ok && ref != nil {
					out[ref.Name] = true
				}
			case *ForRangeStmt:
				out[x.Var] = true
			case *ForEachStmt:
				out[x.Var] = true
			case *AddrOfExpr:
				out[x.Sym] = true
				if ref, ok := x.X.(*VarRefExpr); ok && ref != nil {
					out[ref.Name] = true
				}
			}
			walk(value.Elem())
		case reflect.Struct:
			for i := 0; i < value.NumField(); i++ {
				if value.Field(i).CanInterface() {
					walk(value.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < value.Len(); i++ {
				walk(value.Index(i))
			}
		}
	}
	// Visit the AST graph with a cycle guard before structural verification.
	// Malformed cyclic nodes must reach Verify's diagnostic, never recurse here.
	walk(reflect.ValueOf(root))
	return out
}
