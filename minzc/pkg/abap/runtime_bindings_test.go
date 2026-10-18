package abap

import (
	"fmt"
	"testing"
)

// A user report object must not capture an implementation-local binding in a
// compiler runtime function. Copy, trimming and decimal output each mutate a
// common local spelling that may also occur in ordinary ABAP report DATA.
func TestRuntimeBindingsDoNotCaptureReportGlobals(t *testing.T) {
	for _, name := range []string{"n", "i", "count"} {
		t.Run(name, func(t *testing.T) {
			src := fmt.Sprintf(`REPORT zbindings.
DATA %s TYPE i VALUE 77.
DATA source TYPE c LENGTH 4 VALUE 'A'.
DATA dest TYPE c LENGTH 4.
dest = source.
WRITE |{ dest }:{ %s }|.`, name, name)
			verifyItabReport(t, "runtime-binding-"+name, src, itabExpectation{objects: map[string][]byte{name: {77, 0, 0, 0}, "source": {'A', ' ', ' ', ' '}, "dest": {'A', ' ', ' ', ' '}}, output: "A:77"})
		})
	}
}
