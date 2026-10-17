package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// A harness that places a return address on the stack invokes main as a
// subroutine. Re-emit the same allocated MIR with that explicit contract;
// ordinary bare-main assembly intentionally terminates with HALT instead.
func calledMainTestAssembly(t testing.TB, steps Steps) string {
	t.Helper()
	assembly, err := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{
		CalledMain: true, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy,
	})
	if err != nil {
		t.Fatal(err)
	}
	return assembly
}
