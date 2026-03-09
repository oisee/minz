package mir2

// EvalPure evaluates a named pure function in module m at compile time,
// passing integer arguments and returning integer results.
//
// This is the primary entry point for compile-time function evaluation:
//   - Used by ConstantCallElim to fold calls with known-constant arguments.
//   - Used by future LUT-generation passes to compute table entries.
//   - Used directly in tests to verify IR semantics without a Z80 backend.
//
// A safety step limit of 1 million instructions is enforced by default.
// Pass a custom limit via EvalPureWithLimit.
//
// Returns an error if:
//   - The function is not found in the module.
//   - The function has observable side effects (Store, Asm, I/O intrinsics).
//   - The step limit is exceeded (likely infinite loop in comptime code).
//   - Any runtime error occurs (division by zero, stack overflow, etc.).
func EvalPure(m *Module, funcName string, args ...int64) ([]int64, error) {
	return EvalPureWithLimit(m, 1_000_000, funcName, args...)
}

// Eval1 is a convenience wrapper for single-result functions.
// Returns (0, nil) for void functions.
func Eval1(m *Module, funcName string, args ...int64) (int64, error) {
	results, err := EvalPure(m, funcName, args...)
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	return results[0], nil
}

// EvalPureWithLimit is like EvalPure but accepts a custom step limit.
// Use limit=0 for unlimited (dangerous with comptime user code).
func EvalPureWithLimit(m *Module, maxSteps int64, funcName string, args ...int64) ([]int64, error) {
	vm := NewVM(m)
	vm.MaxSteps = maxSteps

	callArgs := make([]Value, len(args))
	for i, a := range args {
		callArgs[i] = Value{I: a}
	}

	results, err := vm.Call(funcName, callArgs)
	if err != nil {
		return nil, err
	}

	out := make([]int64, len(results))
	for i, r := range results {
		out[i] = r.I
	}
	return out, nil
}

// EvalTable computes all values of a single-argument function f for inputs
// in [lo, hi) and returns them as a slice.
//
// Typical use: generate a 256-entry lookup table for a u8→u8 function:
//
//	table, err := EvalTable(m, "sin_approx", 0, 256)
//	// emit: DB table[0], table[1], ..., table[255]
func EvalTable(m *Module, funcName string, lo, hi int64) ([]int64, error) {
	vm := NewVM(m)
	vm.MaxSteps = 10_000_000

	out := make([]int64, 0, hi-lo)
	for i := lo; i < hi; i++ {
		results, err := vm.Call(funcName, []Value{{I: i}})
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			out = append(out, 0)
		} else {
			out = append(out, results[0].I)
		}
	}
	return out, nil
}
