package isle

// evalGuard evaluates a guard clause against bound variables.
//
// Supported guard forms:
//
//	(if < ?n 256)       — integer comparison
//	(if == ?x ?y)       — equality
//	(if != ?x ?y)       — inequality
//	(if is_pure ?x)     — predicate (delegates to extern if registered)
func evalGuard(g *Guard, bindings Bindings) bool {
	if len(g.Args) < 2 {
		return false
	}

	lhs := resolveGuardArg(g.Args[0], bindings)
	rhs := resolveGuardArg(g.Args[1], bindings)

	// Integer comparisons
	if lhs.Kind == TermInt && rhs.Kind == TermInt {
		return cmpInt(lhs.IntVal, g.Op, rhs.IntVal)
	}

	// Structural equality/inequality
	switch g.Op {
	case "==":
		return termsEqual(lhs, rhs)
	case "!=":
		return !termsEqual(lhs, rhs)
	}

	return false
}

func resolveGuardArg(t Term, bindings Bindings) Term {
	if t.Kind == TermVar {
		if val, ok := bindings[t.Name]; ok {
			return val
		}
	}
	return t
}

func cmpInt(a int64, op string, b int64) bool {
	switch op {
	case "<":
		return a < b
	case ">":
		return a > b
	case "<=":
		return a <= b
	case ">=":
		return a >= b
	case "==":
		return a == b
	case "!=":
		return a != b
	}
	return false
}
