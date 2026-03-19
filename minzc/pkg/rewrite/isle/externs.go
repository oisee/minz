package isle

// ExternFunc is a Go callback that implements an extern constructor.
// It receives the arguments from the S-expression call and the current bindings,
// and returns a replacement term (or the original on error).
type ExternFunc func(args []Term, b Bindings) (Term, error)

// ExternRegistry holds registered extern constructors.
type ExternRegistry struct {
	fns map[string]ExternFunc
}

// NewExternRegistry creates an empty registry.
func NewExternRegistry() *ExternRegistry {
	return &ExternRegistry{fns: make(map[string]ExternFunc)}
}

// Register adds an extern constructor by name.
func (r *ExternRegistry) Register(name string, fn ExternFunc) {
	r.fns[name] = fn
}

// Resolve walks a term tree and replaces any TermCall whose name matches a
// registered extern with the result of calling the extern function.
func (r *ExternRegistry) Resolve(t Term, bindings Bindings) Term {
	if t.Kind != TermCall {
		return t
	}

	// Resolve children first (bottom-up)
	resolved := make([]Term, len(t.Args))
	for i, a := range t.Args {
		resolved[i] = r.Resolve(a, bindings)
	}

	// Check if this call is an extern
	if fn, ok := r.fns[t.Name]; ok {
		result, err := fn(resolved, bindings)
		if err == nil {
			return result
		}
		// On error, fall through with resolved args
	}

	return Term{Kind: TermCall, Name: t.Name, Args: resolved}
}
