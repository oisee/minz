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

func TestCPMRuntimeGuardedAdmission(t *testing.T) {
	entry := strings.Replace(cpmRuntimeEntry, "    CALL main", "    CALL __cpm_stack_check\n    CALL main", 1)
	body := "main:\n CALL __cpm_stack_check\n CALL rec\n RET\nrec:\n CALL __cpm_stack_check\n PUSH AF\n CALL __cpm_stack_check\n CALL rec\n POP AF\n RET\n"
	code := entry + body + cpmRuntimeGuardedTail(64)
	if e := validateCPMRuntimeAssemblyMode(code, true); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(code, " CALL __cpm_stack_check\n PUSH AF", " PUSH AF", 1), strings.Replace(code, "CP __cpm_stack_limit^H", "CP 0", 1), strings.Replace(code, " POP AF\n RET\n", " RET\n", 1)} {
		if e := validateCPMRuntimeAssemblyMode(bad, true); e == nil {
			t.Fatal("corrupted guarded frame admitted")
		}
	}
}

func TestCPMRuntimeRejectsBranchBypassOfGuard(t *testing.T) {
	entry := strings.Replace(cpmRuntimeEntry, "    CALL main", "    CALL __cpm_stack_check\n    CALL main", 1)
	for _, branch := range []string{"JP", "JP Z,", "JR", "JR NZ,"} {
		body := "main:\n " + branch + " unsafe_growth\n CALL __cpm_stack_check\nunsafe_growth:\n PUSH AF\n POP AF\n RET\n"
		code := entry + body + cpmRuntimeGuardedTail(64)
		if e := validateCPMRuntimeAssemblyMode(code, true); e == nil {
			t.Fatalf("branch bypass admitted: %s", branch)
		}
	}
}

func TestCPMRuntimeConditionalCallGuardPaths(t *testing.T) {
	entry := strings.Replace(cpmRuntimeEntry, "    CALL main", "    CALL __cpm_stack_check\n    CALL main", 1)
	body := "main:\n CALL __cpm_stack_check\n CALL Z, helper\n RET\nhelper:\n CALL __cpm_stack_check\n PUSH AF\n POP AF\n RET\n"
	if e := validateCPMRuntimeAssemblyMode(entry+body+cpmRuntimeGuardedTail(64), true); e != nil {
		t.Fatal(e)
	}
	bypass := strings.Replace(body, "main:\n", "main:\n JP NZ, unsafe_call\n", 1)
	bypass = strings.Replace(bypass, " CALL Z, helper", "unsafe_call:\n CALL Z, helper", 1)
	if e := validateCPMRuntimeAssemblyMode(entry+bypass+cpmRuntimeGuardedTail(64), true); e == nil {
		t.Fatal("conditional call bypass admitted")
	}
}
