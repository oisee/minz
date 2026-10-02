package mir2wasm

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestVoidAssertionSandboxCompatibility(t *testing.T) {
	for _, sandbox := range []bool{false, true} {
		t.Run(map[bool]string{false: "top-level", true: "sandbox"}[sandbox], func(t *testing.T) {
			hm, err := nanz.Parse("fun noop() { return }\n", "test.nanz")
			if err != nil {
				t.Fatal(err)
			}
			a := hir.Assert{FuncName: "noop", Expected: 99, Via: "wasm", Line: 2, Source: "assert noop() == 99"}
			if sandbox {
				hm.Sandboxes = []hir.Sandbox{{Name: "void", Asserts: []hir.Assert{a}}}
			} else {
				hm.Asserts = []hir.Assert{a}
			}
			hm.AssertStats = &hir.AssertStats{}
			err = RunAsserts(hm, hir.LowerModule(hm), false)
			if sandbox {
				if err != nil {
					t.Fatal(err)
				}
				if *hm.AssertStats != (hir.AssertStats{Executed: 1, Passed: 1}) {
					t.Fatalf("stats=%+v", hm.AssertStats)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "no return value") {
					t.Fatalf("expected top-level error, got %v", err)
				}
				if *hm.AssertStats != (hir.AssertStats{Executed: 1, Failed: 1}) {
					t.Fatalf("stats=%+v", hm.AssertStats)
				}
			}
		})
	}
}
