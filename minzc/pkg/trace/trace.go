package trace

import (
	"fmt"
	"io"
)

// Tracer writes structured compilation trace output.
// A nil *Tracer is safe to call — all methods are no-ops on nil receiver.
type Tracer struct {
	w io.Writer
}

// New creates a Tracer that writes to w. Returns nil if w is nil.
func New(w io.Writer) *Tracer {
	if w == nil {
		return nil
	}
	return &Tracer{w: w}
}

// Log writes a single trace line: [phase] message.
func (t *Tracer) Log(phase, format string, args ...any) {
	if t == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(t.w, "[%-9s] %s\n", phase, msg)
}
