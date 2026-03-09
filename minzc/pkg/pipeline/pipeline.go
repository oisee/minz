// Package pipeline wires the HIR→MIR2→Z80 compilation pipeline.
//
// This is the "new" backend path:
//
//	Nanz source  → nanz.Parse()   → *hir.Module ─┐
//	PL/M source  → plm.Compile()  → *hir.Module ─┤
//	                                              ↓
//	                              hir.LowerModule → *mir2.Module
//	                              ReorderBlocks, DeadStoreElim, Verify
//	                              ComputeLiveness + Allocate (per func)
//	                              Z80Codegen → .a80 assembly text
//	                              z80asm.Assemble → binary
//
// Eventually the MinZ (pkg/ast) frontend will also route here,
// replacing the old pkg/ir + pkg/codegen pipeline.
package pipeline

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// Result holds the outputs of a full compilation.
type Result struct {
	Assembly string // .a80 text
	Binary   []byte // assembled binary (nil if AssembleOnly=false was not requested)
	Errors   []string
}

// Steps holds the intermediate outputs of each pipeline stage.
type Steps struct {
	HIR      string // HIR structural dump (hir.Module.Dump())
	MIR2Raw  string // MIR2 module dump before optimisation passes
	MIR2Opt  string // MIR2 module dump after DSE + ReorderBlocks
	Assembly string // Final .a80 text
}

// Options configures optional pipeline passes.
type Options struct {
	// ContractOpt enables Phase 5b interprocedural contract optimisation.
	// Default true — only disable for A/B comparison or debugging.
	ContractOpt bool
}

// DefaultOptions returns options with all recommended passes enabled.
func DefaultOptions() Options { return Options{ContractOpt: true} }

// CompileHIRSteps runs the full HIR→MIR2→Z80 pipeline and returns all intermediate outputs.
func CompileHIRSteps(hm *hir.Module) (Steps, error) {
	var s Steps

	// Capture HIR before lowering.
	s.HIR = hm.Dump()

	// Lower HIR → MIR2 (raw, before optimisation).
	m := hir.LowerModule(hm)
	s.MIR2Raw = m.Dump()

	// Per-function optimisation passes.
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
	}
	s.MIR2Opt = m.Dump()

	// Structural verification.
	if err := mir2.Verify(m); err != nil {
		return s, fmt.Errorf("MIR2 verify: %w", err)
	}

	// Phase 5b: interprocedural contract optimisation (greedy DP on call graph).
	ct := mir2.Z80CostTable{}
	cs := mir2.OptimizeContracts(m, ct)
	mir2.ApplyContracts(m, cs)

	// Register allocation: per-function, combined result for codegen.
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, ct)
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
	}

	s.Assembly = mir2.Z80Codegen(m, combined)
	return s, nil
}

// CompileHIR runs the full HIR→MIR2→Z80 pipeline with default options.
// It does NOT assemble to binary; call Assemble for that.
func CompileHIR(hm *hir.Module) (string, error) {
	return CompileHIRWithOptions(hm, DefaultOptions())
}

// CompileHIRWithOptions runs the HIR→MIR2→Z80 pipeline with explicit options.
// Use this to compare output with/without specific optimisation passes.
func CompileHIRWithOptions(hm *hir.Module, opts Options) (string, error) {
	// Lower HIR → MIR2.
	m := hir.LowerModule(hm)

	// Per-function optimisation passes.
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
	}

	// Structural verification.
	if err := mir2.Verify(m); err != nil {
		return "", fmt.Errorf("MIR2 verify: %w", err)
	}

	ct := mir2.Z80CostTable{}

	// Phase 5b: interprocedural contract optimisation (greedy DP on call graph).
	if opts.ContractOpt {
		cs := mir2.OptimizeContracts(m, ct)
		mir2.ApplyContracts(m, cs)
	}

	// Register allocation: per-function, combined result for codegen.
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, ct)
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
	}

	// Z80 assembly text.
	asm := mir2.Z80Codegen(m, combined)
	return asm, nil
}

// Assemble assembles .a80 text to a binary using MZA.
// target: "cpm", "zxspectrum", "generic" (default).
func Assemble(asmSrc string, target string) ([]byte, []error) {
	a := z80asm.NewAssembler()
	if target != "" {
		t, err := z80asm.ParseTarget(target)
		if err == nil {
			a.SetTarget(t)
		}
	}
	res, err := a.AssembleString(asmSrc)
	if err != nil {
		return nil, []error{err}
	}
	if len(res.Errors) > 0 {
		errs := make([]error, len(res.Errors))
		for i, e := range res.Errors {
			errs[i] = e
		}
		return nil, errs
	}
	return res.Binary, nil
}
