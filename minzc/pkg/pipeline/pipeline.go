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
	"strings"

	"github.com/minz/minzc/pkg/emulator"
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
	// AnnotateTStates adds T-state cost comments to every Z80 instruction line.
	AnnotateTStates bool
}

// DefaultOptions returns options with all recommended passes enabled.
func DefaultOptions() Options { return Options{ContractOpt: true} }

// CompileHIRSteps runs the full HIR→MIR2→Z80 pipeline and returns all intermediate outputs.
func CompileHIRSteps(hm *hir.Module, opts ...Options) (Steps, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	var s Steps

	// Capture HIR before lowering.
	s.HIR = hm.Dump()

	// Lower HIR → MIR2 (raw, before optimisation).
	m := hir.LowerModule(hm)
	s.MIR2Raw = m.Dump()

	// Per-function optimisation passes.
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		// Constant pipeline: propagate → fold → identity-simplify → call-elim → repeat to fixpoint.
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			s := mir2.SimplifyIdentities(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !s && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
		// VM-based branch equivalence: remove CmpEq guards whose then-path is
		// provably equivalent to the else-path on the equality boundary.
		if mir2.BranchEquiv(m, f) {
			mir2.EliminateDeadBlocks(f)
			mir2.DeadStoreElim(f) // clean up now-unused cmp instruction
		}
		// Conditional-return sink: convert BrIf-with-trivial-else into TermCondRet.
		if mir2.CondRetSink(f) {
			mir2.EliminateDeadBlocks(f)
		}
	}
	s.MIR2Opt = m.Dump()

	// Structural verification.
	if err := mir2.Verify(m); err != nil {
		return s, fmt.Errorf("MIR2 verify: %w", err)
	}

	// Phase 5b: interprocedural contract optimisation (greedy DP on call graph).
	// Run BEFORE LUTGen so that synthetic LUT functions (which have hardcoded
	// class requirements in their Sub instructions) are not re-optimised and
	// given a conflicting class (BUG-004).
	ct := mir2.Z80CostTable{}
	cs := mir2.OptimizeContracts(m, ct)
	mir2.ApplyContracts(m, cs)

	// Module-level: replace ranged-param pure functions with LUTs.
	// Must run AFTER contract optimisation — LUT synthesis inherits the
	// already-chosen param class and the contract optimizer never sees the
	// synthetic Sub instruction.
	mir2.LUTGen(m)

	// Register allocation: per-function PBQP, combined result for codegen.
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.PBQPAllocate(f, lr, ct)
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
	}

	// MIR2 VM assertion checks (skip "z80"-only asserts).
	if err := RunAssertsMIR2(hm, m); err != nil {
		return s, err
	}

	s.Assembly = mir2.Z80Codegen(m, combined, mir2.Z80CodegenOptions{
		AnnotateTStates: opt.AnnotateTStates,
	})

	// Z80 binary assertion checks (skip "mir2"-only asserts).
	if err := RunAssertsZ80(hm, m, combined, s.Assembly); err != nil {
		return s, err
	}
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
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
		if mir2.BranchEquiv(m, f) {
			mir2.EliminateDeadBlocks(f)
			mir2.DeadStoreElim(f)
		}
		if mir2.CondRetSink(f) {
			mir2.EliminateDeadBlocks(f)
		}
	}

	// Structural verification.
	if err := mir2.Verify(m); err != nil {
		return "", fmt.Errorf("MIR2 verify: %w", err)
	}

	ct := mir2.Z80CostTable{}

	// Phase 5b: interprocedural contract optimisation (greedy DP on call graph).
	// Run BEFORE LUTGen so synthetic LUT functions keep their original param class
	// and are not re-assigned a conflicting one (BUG-004).
	if opts.ContractOpt {
		cs := mir2.OptimizeContracts(m, ct)
		mir2.ApplyContracts(m, cs)
	}

	// Module-level: replace ranged-param pure functions with LUTs.
	// Must run AFTER contract optimisation (see above).
	mir2.LUTGen(m)

	// Register allocation: per-function PBQP, combined result for codegen.
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.PBQPAllocate(f, lr, ct)
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
	}

	// MIR2 VM assertion checks (skip "z80"-only asserts).
	if err := RunAssertsMIR2(hm, m); err != nil {
		return "", err
	}

	// Z80 assembly text.
	asm := mir2.Z80Codegen(m, combined)

	// Z80 binary assertion checks (skip "mir2"-only asserts).
	if err := RunAssertsZ80(hm, m, combined, asm); err != nil {
		return "", err
	}
	return asm, nil
}

// RunAsserts evaluates all compile-time assertions on both MIR2 VM and Z80 binary.
// Kept for external callers; internally the pipeline calls RunAssertsMIR2 + RunAssertsZ80.
func RunAsserts(hm *hir.Module, m *mir2.Module) error {
	return RunAssertsMIR2(hm, m)
}

// RunAssertsMIR2 evaluates module-level compile-time assertions via the MIR2 VM.
// Skips assertions with Via=="z80".
// Called after MIR2 is fully optimised and allocated.
func RunAssertsMIR2(hm *hir.Module, m *mir2.Module) error {
	if len(hm.Asserts) == 0 {
		return nil
	}
	vm := mir2.NewVM(m)
	for _, a := range hm.Asserts {
		if a.Via == "z80" {
			continue // skip mir2-VM check for z80-only asserts
		}
		args := make([]mir2.Value, len(a.Args))
		for i, v := range a.Args {
			args[i] = mir2.Value{I: v}
		}
		rets, err := vm.Call(a.FuncName, args)
		if err != nil {
			return fmt.Errorf("line %d: assert %q [mir2]: VM error: %w", a.Line, a.Source, err)
		}
		if len(rets) == 0 {
			return fmt.Errorf("line %d: assert %q [mir2]: function returned no value", a.Line, a.Source)
		}
		if len(a.ExpectedMulti) > 0 {
			if len(rets) < len(a.ExpectedMulti) {
				return fmt.Errorf("line %d: assert %q [mir2]: function returned %d values, want %d",
					a.Line, a.Source, len(rets), len(a.ExpectedMulti))
			}
			for i, want := range a.ExpectedMulti {
				if rets[i].I != want {
					return fmt.Errorf("line %d: assert %q [mir2]: return[%d] got %d, want %d",
						a.Line, a.Source, i, rets[i].I, want)
				}
			}
		} else {
			if rets[0].I != a.Expected {
				return fmt.Errorf("line %d: assert %q [mir2]: got %d, want %d",
					a.Line, a.Source, rets[0].I, a.Expected)
			}
		}
	}
	return nil
}

const assertLoadAddr = 0x8000

// RunAssertsZ80 evaluates compile-time assertions by assembling the generated Z80
// and running each function call on the MZE emulator.  Skips Via=="mir2" asserts.
//
// Uses the actual register allocation (ar) to determine which physical register
// holds each parameter — the contract optimizer may place params in unexpected
// registers (e.g. second u8 param in C rather than B).
func RunAssertsZ80(hm *hir.Module, m *mir2.Module, ar *mir2.AllocResult, asmSrc string) error {
	if len(hm.Asserts) == 0 {
		return nil
	}
	// Build MIR2 function lookup for contract/ABI info.
	mir2Funcs := make(map[string]*mir2.Func, len(m.Funcs))
	for _, f := range m.Funcs {
		mir2Funcs[f.Name] = f
	}
	// Build HIR function lookup for return-type info.
	hirFuncs := make(map[string]*hir.Func, len(hm.Funcs))
	for _, f := range hm.Funcs {
		hirFuncs[f.Name] = f
	}

	for _, a := range hm.Asserts {
		if a.Via == "mir2" {
			continue
		}
		mf := mir2Funcs[a.FuncName]
		if mf == nil {
			return fmt.Errorf("line %d: assert %q [z80]: function %q not found in MIR2", a.Line, a.Source, a.FuncName)
		}

		// Build bootstrap: load args into their actual allocated registers, CALL fn, HALT.
		var boot strings.Builder
		fmt.Fprintf(&boot, "    ORG 0x%04X\n", assertLoadAddr)
		boot.WriteString("    LD SP, 0xFF00\n")

		for i, arg := range a.Args {
			if i >= len(mf.Contract.Params) {
				return fmt.Errorf("line %d: assert %q [z80]: too many args (function has %d params)",
					a.Line, a.Source, len(mf.Contract.Params))
			}
			param := mf.Contract.Params[i]
			loc, ok := ar.Locs[param.Reg]
			if !ok {
				return fmt.Errorf("line %d: assert %q [z80]: param %q has no allocated location",
					a.Line, a.Source, param.Name)
			}
			switch loc.Name {
			case "HL", "DE", "BC":
				fmt.Fprintf(&boot, "    LD %s, %d\n", loc.Name, arg)
			default:
				// Single register (A, B, C, D, E, H, L)
				fmt.Fprintf(&boot, "    LD %s, %d\n", loc.Name, arg)
			}
		}

		fmt.Fprintf(&boot, "    CALL %s\n", a.FuncName)
		boot.WriteString("    DI\n    HALT\n")

		src := boot.String() + "\n" + asmSrc
		as := z80asm.NewAssembler()
		res, err := as.AssembleString(src)
		if err != nil {
			return fmt.Errorf("line %d: assert %q [z80]: assemble: %w", a.Line, a.Source, err)
		}
		if len(res.Errors) > 0 {
			return fmt.Errorf("line %d: assert %q [z80]: assemble errors: %v", a.Line, a.Source, res.Errors[0])
		}

		z := emulator.NewRemogattoZ80()
		if lerr := z.LoadMemory(assertLoadAddr, res.Binary); lerr != nil {
			return fmt.Errorf("line %d: assert %q [z80]: load: %w", a.Line, a.Source, lerr)
		}
		z.SetPC(assertLoadAddr)
		if rerr := z.Run(); rerr != nil {
			return fmt.Errorf("line %d: assert %q [z80]: run: %w", a.Line, a.Source, rerr)
		}

		regs := z.GetRegisters()
		// Result register: determined by return class (ClassAcc→A, ClassPointer→HL, etc.)
		var got int64
		if len(mf.Contract.Returns) > 0 {
			switch mf.Contract.Returns[0].Class {
			case mir2.ClassPointer:
				got = int64(regs.HL)
			case mir2.ClassIndex:
				got = int64(regs.DE)
			case mir2.ClassPair:
				got = int64(regs.BC)
			default: // ClassAcc, ClassCounter, ClassGeneral → A
				got = int64(regs.A)
			}
		} else {
			hf := hirFuncs[a.FuncName]
			if hf != nil && (hf.RetTy == mir2.TyU16 || hf.RetTy == mir2.TyI16) {
				got = int64(regs.HL)
			} else {
				got = int64(regs.A)
			}
		}

		if len(a.ExpectedMulti) > 0 {
			if got != a.ExpectedMulti[0] {
				return fmt.Errorf("line %d: assert %q [z80]: return[0] got %d, want %d",
					a.Line, a.Source, got, a.ExpectedMulti[0])
			}
		} else {
			if got != a.Expected {
				return fmt.Errorf("line %d: assert %q [z80]: got %d, want %d",
					a.Line, a.Source, got, a.Expected)
			}
		}
	}
	return nil
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
