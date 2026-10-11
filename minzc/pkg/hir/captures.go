package hir

import (
	"reflect"
	"sort"

	"github.com/minz/minzc/pkg/mir2"
)

// InferCaptures resolves only names present in the frontend's lexical snapshot.
// Unknown names cannot become captures just because the function is a lambda.
// Inner lambdas must be resolved before their enclosing lambda.
func InferCaptures(m *Module, f *Func, outer map[string]mir2.Ty) error {
	v := &hirVerifier{funcs: map[string]*Func{}, globals: map[string]bool{}, active: map[any]bool{}, aliases: map[string]*Func{}, fn: f, captureOuter: outer, collected: map[string]mir2.Ty{}, labels: map[string]bool{}}
	if v.captureOuter == nil {
		v.captureOuter = map[string]mir2.Ty{}
	}
	for _, fn := range m.Funcs {
		v.funcs[fn.Name] = fn
	}
	for _, g := range m.Globals {
		v.globals[g.Name] = true
	}
	env := map[string]mir2.Ty{}
	for _, p := range f.Params {
		env[p.Name] = p.Ty
	}
	if err := v.block(f.Body, env, 0); err != nil {
		return err
	}
	names := make([]string, 0, len(v.collected))
	for n := range v.collected {
		names = append(names, n)
	}
	sort.Strings(names)
	captures := make([]Param, 0, len(names))
	for _, n := range names {
		captures = append(captures, Param{Name: n, Ty: v.collected[n]})
	}
	f.Captures = captures
	return nil
}

func (v *hirVerifier) functionValue(e Expr) *Func {
	switch e := e.(type) {
	case *VarRefExpr:
		if f := v.funcs[e.Name]; f != nil {
			return f
		}
		return v.aliases[e.Name]
	case *AddrOfExpr:
		return v.funcs[e.Sym]
	}
	return nil
}
func (v *hirVerifier) checkCaptureUse(f *Func, env map[string]mir2.Ty) error {
	for _, c := range f.Captures {
		ty := env[c.Name]
		if ty == nil && v.captureOuter[c.Name] != nil {
			ty = v.captureOuter[c.Name]
			v.collected[c.Name] = ty
		}
		if ty == nil {
			return v.fail("function %s needs capture %q outside its lexical scope", f.Name, c.Name)
		}
		if ty.String() != c.Ty.String() {
			return v.fail("function %s capture %s type %s, want %s", f.Name, c.Name, ty, c.Ty)
		}
	}
	return nil
}
func (v *hirVerifier) checkSignature(sig *CallSignature, args []Expr, ret mir2.Ty, env map[string]mir2.Ty) error {
	if len(sig.Params) != len(args) {
		return v.fail("call arity %d, want %d", len(args), len(sig.Params))
	}
	for i, ty := range sig.Params {
		if nilHIR(ty) || ty == mir2.TyVoid {
			return v.fail("invalid callable parameter %d", i)
		}
		// Higher-order legacy parameters have an erased scalar representation.
		// Validate the referenced function's lexical requirements, not a fake integer ABI.
		if f := v.functionValue(args[i]); f != nil {
			if len(f.Captures) > 0 && v.captureOuter == nil {
				return v.fail("capturing callable %s cannot pass through a runtime call without an environment ABI", f.Name)
			}
			continue
		}
		if !assignableHIR(ty, args[i].ExprTy()) {
			return v.fail("call argument %d type %s, want %s", i, args[i].ExprTy(), ty)
		}
	}
	result := mir2.Ty(mir2.TyVoid)
	for i, ty := range sig.Returns {
		if nilHIR(ty) || ty == mir2.TyVoid {
			return v.fail("invalid callable return %d", i)
		}
		if i == 0 {
			result = ty
		}
	}
	if !assignableHIR(ret, result) {
		return v.fail("call return type %s, want %s", ret, result)
	}
	return nil
}

func sameHIRSignature(a, b *CallSignature) bool {
	if !reflect.DeepEqual(a.StructReturnBuffers, b.StructReturnBuffers) || len(a.Params) != len(b.Params) || len(a.Returns) != len(b.Returns) {
		return false
	}
	for i, ty := range a.Params {
		if nilHIR(ty) || nilHIR(b.Params[i]) || ty.String() != b.Params[i].String() {
			return false
		}
	}
	for i, ty := range a.Returns {
		if nilHIR(ty) || nilHIR(b.Returns[i]) || ty.String() != b.Returns[i].String() {
			return false
		}
	}
	return true
}
