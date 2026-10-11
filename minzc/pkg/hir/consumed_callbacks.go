package hir

import "sort"

// These contracts concern only pointer parameters actually used as callables.
// They are inferred from semantic call operands, not scalar address width.
func (v *hirVerifier) requireCallback(f *Func, index int, sig *CallSignature) error {
	if v.callbackParams[f] == nil {
		v.callbackParams[f] = map[int]*CallSignature{}
	}
	if old := v.callbackParams[f][index]; old != nil {
		if !sameHIRSignature(old, sig) {
			return v.fail("callback parameter %s.%s has conflicting consumed signatures", f.Name, f.Params[index].Name)
		}
		return nil
	}
	v.callbackParams[f][index] = sig
	v.callbackChanged = true
	return nil
}

func (v *hirVerifier) checkConsumedCallbacks(callee *Func, args []Expr) error {
	indices := []int{}
	for index := range v.callbackParams[callee] {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		sig := v.callbackParams[callee][index]
		if index >= len(args) {
			continue
		} // normal arity verification owns malformed calls
		arg := args[index]
		if target := v.functionValue(arg); target != nil {
			if target.FallibleCY {
				return v.fail("callback target %s requires unsupported fallible CY contract", target.Name)
			}
			if !sameHIRSignature(sig, target.Signature()) {
				return v.fail("callback argument %d to %s has incompatible callable signature: target %s", index, callee.Name, target.Name)
			}
		}
		if ref, ok := arg.(*VarRefExpr); ok {
			if origin, exists := v.callbackOrigins[ref.Name]; exists {
				if err := v.requireCallback(v.fn, origin, sig); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
