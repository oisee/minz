package mir2

import (
	"fmt"
	"strings"
	"testing"
)

func TestCPMRuntimeStackChecksCalledBodies(t *testing.T) {
	cases := []struct {
		name, body string
		accept     bool
	}{
		{"balanced nested", "main:\n CALL helper\n RET\nhelper:\n PUSH BC\n CALL __rotate_1\n POP BC\n RET\n__rotate_1:\n PUSH AF\n POP AF\n RET\n", true},
		{"unbalanced rotate", "main:\n CALL __rotate_1\n RET\n__rotate_1:\n PUSH BC\n RET\n", false},
		{"conditional return imbalance", "main:\n CALL helper\n RET\nhelper:\n PUSH AF\n RET Z\n POP AF\n RET\n", false},
		{"callee steals return word", "main:\n CALL helper\n RET\nhelper:\n POP AF\n PUSH AF\n RET\n", false},
		{"recursive callee", "main:\n CALL helper\n RET\nhelper:\n CALL helper\n RET\n", false},
		{"dynamic stack", "main:\n CALL helper\n RET\nhelper:\n DEC SP\n RET\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := validateCPMRuntimeAssembly(cpmRuntimeEntry + c.body + cpmRuntimeTail)
			if (e == nil) != c.accept {
				t.Fatalf("accept %v error %v", c.accept, e)
			}
		})
	}
	var b strings.Builder
	b.WriteString("main:\n CALL f0\n RET\n")
	for i := 0; i < 127; i++ {
		fmt.Fprintf(&b, "f%d:\n", i)
		if i < 126 {
			fmt.Fprintf(&b, " CALL f%d\n", i+1)
		} else {
			b.WriteString(" PUSH BC\n POP BC\n")
		}
		b.WriteString(" RET\n")
	}
	if e := validateCPMRuntimeAssembly(cpmRuntimeEntry + b.String() + cpmRuntimeTail); e == nil {
		t.Fatal("nested call return words exceeded resident stack")
	}
}
