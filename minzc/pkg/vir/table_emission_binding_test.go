package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

// Checked instruction validity alone cannot prove equivalence to the MIR.
func TestCheckedEmitRejectsReAdmittedForeignOperations(t *testing.T) {
	for _, mutation := range []string{"constant", "return-location", "raw-emission", "size-policy"} {
		t.Run(mutation, func(t *testing.T) {
			f, a := productionTableFunction(mir2.ClassAcc)
			p, err := PrepareLocalTableProblem(f, a)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "constant":
				p.Ops[0].Imm = 99
			case "return-location":
				p.Options.CheckedReturnLocation = "C"
				p.Ops[len(p.Ops)-2].DstHint = Z80.LocSetByNames("C")
			case "raw-emission":
				p.Options.CheckedRawEmission = false
			case "size-policy":
				p.Options.OptSize = false
			}
			location := "A"
			if mutation == "return-location" {
				location = "C"
			}
			assignment := map[int]int{}
			for _, op := range p.Ops {
				if op.Dst > 0 {
					assignment[op.Dst] = Z80.LocByName(location)
				}
			}
			var table RegAllocTable
			table.Init()
			if err := table.AddChecked(p.Ops, assignment, Z80, p.Options); err != nil {
				t.Fatal("counterexample must be locally admissible", err)
			}
			asm, err := p.Emit(&table)
			if err == nil || asm != "" || !strings.Contains(err.Error(), "current MIR") {
				t.Fatalf("foreign lowering emitted: err=%v ASM=%s", err, asm)
			}
		})
	}
}
