package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"testing"
	"time"
)

func TestLocalGenerationRejectsOmittedModelAndWrongABI(t *testing.T) {
	f, allocation := productionTableFunction(mir2.ClassRegC)
	p, err := PrepareLocalTableProblem(f, allocation)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Generate(&RegAllocTable{}, SolverOptions{Z3Path: "/nonexistent/local-solver", Timeout: time.Second}); err == nil {
		t.Fatal("missing solver ignored")
	}
	if err := p.Generate(&RegAllocTable{}, SolverOptions{Timeout: 10 * time.Minute}); err == nil {
		t.Fatal("unbounded timeout admitted")
	}
	fake := t.TempDir() + "/solver"
	// A solver claiming sat without location bindings must not silently use zero.
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf 'sat\\n(model)\\n'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	var table RegAllocTable
	table.Init()
	if err := p.Generate(&table, SolverOptions{Z3Path: fake, Timeout: time.Second}); err == nil || table.Size() != 0 {
		t.Fatal("omitted model admitted")
	}
	solver := os.Getenv("VIR_TEST_Z3")
	if solver == "" {
		return
	}
	if err := p.Generate(&table, SolverOptions{Z3Path: solver, Timeout: time.Second * 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Emit(&table); err != nil {
		t.Fatal(err)
	}
	p.Options.ParamLocs = map[int]int{12345: 0}
	if err := p.Generate(&table, SolverOptions{Z3Path: solver, Timeout: time.Second * 10}); err == nil {
		t.Fatal("nonexistent ABI vreg admitted")
	}
}
