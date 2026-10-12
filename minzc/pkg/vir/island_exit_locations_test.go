package vir

import (
	"os/exec"
	"testing"
	"time"
)

func TestIslandSolverRetainsExitLocations(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Width: 16, Imm: 0x9182, DstHint: Z80.LocSetByNames("DE")},
		{Op: OpConst, Dst: 2, Width: 8, Imm: 37, DstHint: Z80.LocSetByNames("B")},
		{Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 8, DstHint: Z80.LocSetByNames("A")},
	}
	for _, solve := range []struct {
		name string
		fn   func([]VIROp, *MachineDesc, SolverOptions) ([]PIROp, error)
	}{{"global", Solve}, {"per instruction", solvePerInstruction}} {
		t.Run(solve.name, func(t *testing.T) {
			input := append([]VIROp(nil), ops...)
			if solve.name == "per instruction" {
				for i := range input {
					input[i].DstHint = 0
				}
			}
			pir, err := solve.fn(input, Z80, SolverOptions{Z3Path: "z3", Timeout: 30 * time.Second})
			if err != nil || len(pir) == 0 {
				t.Fatalf("solve: %v", err)
			}
			exit := pir[len(pir)-1].ExitLocations
			if loc, ok := exit[3]; !ok || loc != pir[len(pir)-1].DstPhys {
				t.Fatalf("missing final result placement: %v", exit)
			}
			if solve.name == "global" && exit[1] != Z80.LocByName("DE") {
				t.Fatalf("word placement lost: %v", exit)
			}
		})
	}
}
