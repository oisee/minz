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
	"os"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/ez80"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
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
	HIR        string                   // HIR structural dump (hir.Module.Dump())
	MIR2Raw    string                   // MIR2 module dump before optimisation passes
	MIR2Opt    string                   // MIR2 module dump after DSE + ReorderBlocks
	Assembly   string                   // Final .a80 text
	LIRResults []lir.ConvergenceResult  // LIR convergence check results (if LIRCheck enabled)
	Traces     map[string]*FuncTrace    // per-function compilation trace (keyed by func name)
}

// FuncTrace records the full compilation provenance for one function.
// This data is emitted as ASM comment annotations for auditability.
type FuncTrace struct {
	Name       string // function name (mangled)
	SplitFrom  string // non-empty if this function was created by HIR-SPLIT
	SplitPressure int // register pressure that triggered the split

	// Optimization pass counts (MIR2 level)
	ConstProp   int // PropagateConstants iterations that changed something
	ConstFold   int // FoldConstants
	IdentSimp   int // SimplifyIdentities
	CallElim    int // ConstantCallElim
	DSE         int // DeadStoreElim
	DeadBlockArg int // DeadBlockArgElim
	BranchEquiv int // BranchEquiv
	SplitJoinRet int // SplitJoinRet
	CondRetSink int // CondRetSink
	FuseAbsDiff int // FuseAbsDiff
	Inlined     bool // function was inlined by InlineTrivial
	LUTReplaced bool // function was replaced by LUTGen

	// Codegen provenance
	Backend     string // "LIR", "VIR", "PBQP", "LIR+PBQP-fallback", "VIR+PBQP-fallback"
	BackendErr  string // error message if primary backend failed (before fallback)
	WFCAttempt  int    // WFC retry attempt (0 = first try)
	CondRets    int    // number of cond_ret terminators

	// Label audit
	LabelWarnings []string // referenced-but-undefined labels (if any)
}

// Options configures optional pipeline passes.
type Options struct {
	// ContractOpt enables Phase 5b interprocedural contract optimisation.
	// Default true — only disable for A/B comparison or debugging.
	ContractOpt bool
	// AnnotateTStates adds T-state cost comments to every Z80 instruction line.
	AnnotateTStates bool
	// LIRCheck runs the LIR pipeline in parallel with Z80Codegen and reports
	// which functions successfully lower through ISLE+WFC.
	// Non-destructive — does not affect assembly output.
	LIRCheck bool
	// UseLIR replaces PBQP+Z80Codegen with the experimental LIR backend
	// (ISLE combining + WFC regalloc + Layer 4 CFG rules).
	// Functions that fail LIR fall back to the PBQP path automatically.
	UseLIR bool
	// UseGrace replaces hand-coded Go optimization passes (DSE, CondRetSink,
	// SplitJoinRet, DeadBlockArgElim, FuseAbsDiff) with declarative Grace
	// rules. When enabled, Grace runs instead of Go originals.
	// GraceStats (if non-nil) collects per-rule application counts.
	UseGrace   bool
	GraceStats *mir2.GraceStats
	// UseVIR replaces PBQP+Z80Codegen with the unified VIR solver
	// (Z3 joint isel+regalloc, single pass, no text fixups).
	// Functions that fail VIR fall back to the PBQP path automatically.
	UseVIR bool
	// OptSize enables size optimizations (Grace reroll: repeated CALLs → DJNZ loop).
	// Trades ~4T/iteration for code size reduction. Use for ROM-constrained targets.
	OptSize bool
	// Backend selects the codegen backend: "z80" (default), "ez80".
	// When "ez80", the pipeline generates eZ80 ADL assembly instead of Z80.
	Backend string
}

// DefaultOptions returns options with all recommended passes enabled.
// DefaultOptions returns the default pipeline options.
// VIR backend available via UseVIR: true. Runtime calls inlined.
// One new assembly error (sap_mara_demo) — needs investigation before default.
func DefaultOptions() Options { return Options{ContractOpt: true, UseVIR: true} }

// CompileHIRSteps runs the full HIR→MIR2→Z80 pipeline and returns all intermediate outputs.
func CompileHIRSteps(hm *hir.Module, opts ...Options) (Steps, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	var s Steps
	s.Traces = make(map[string]*FuncTrace)

	// Capture HIR before lowering.
	s.HIR = hm.Dump()

	// Great Autorefactor: split high-pressure functions before lowering.
	// Each sub-function gets independent PBQP allocation → fewer spills.
	splitInfo := make(map[string]hir.SplitResult) // splitFunc → SplitResult
	if splits := hir.SplitHighPressure(hm); len(splits) > 0 {
		for _, sp := range splits {
			fmt.Fprintf(os.Stderr, "[HIR-SPLIT] %s → %s (%d inputs, pressure %d)\n",
				sp.OrigFunc, sp.SubFunc, sp.Inputs, sp.Pressure)
			splitInfo[sp.SubFunc] = sp
		}
	}

	// Lower HIR → MIR2 (raw, before optimisation).
	m := hir.LowerModule(hm)
	s.MIR2Raw = m.Dump()

	// Initialize per-function traces.
	for _, f := range m.Funcs {
		tr := &FuncTrace{Name: f.Name}
		if sp, ok := splitInfo[f.Name]; ok {
			tr.SplitFrom = sp.OrigFunc
			tr.SplitPressure = sp.Pressure
		}
		// Count cond_ret terminators
		for _, b := range f.Blocks {
			if _, ok := b.Term.(*mir2.TermCondRet); ok {
				tr.CondRets++
			}
		}
		s.Traces[f.Name] = tr
	}

	// Per-function optimisation passes.
	for _, f := range m.Funcs {
		tr := s.Traces[f.Name]
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		// Constant pipeline: propagate → fold → identity-simplify → call-elim → repeat to fixpoint.
		for iter := 0; iter < 100; iter++ {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			si := mir2.SimplifyIdentities(f)
			e := mir2.ConstantCallElim(m, f)
			if p { tr.ConstProp++ }
			if c { tr.ConstFold++ }
			if si { tr.IdentSimp++ }
			if e { tr.CallElim++ }
			if !p && !c && !si && !e {
				break
			}
		}
		if opt.UseGrace {
			// ── Grace declarative path ──────────────────────────────
			mir2.RunGracePasses(f, opt.GraceStats)
			mir2.EliminateDeadBlocks(f)
			if mir2.BranchEquiv(m, f) {
				tr.BranchEquiv++
				mir2.EliminateDeadBlocks(f)
				mir2.RunGracePasses(f, opt.GraceStats)
				mir2.EliminateDeadBlocks(f)
			}
			for _, blk := range f.Blocks {
				mir2.FusionSubCmpInBlock(blk)
			}
		} else {
			// ── Go original path ────────────────────────────────────
			mir2.DeadStoreElim(f); tr.DSE++
			if mir2.DeadBlockArgElim(f) { tr.DeadBlockArg++ }
			if mir2.BranchEquiv(m, f) {
				tr.BranchEquiv++
				mir2.EliminateDeadBlocks(f)
				mir2.DeadStoreElim(f); tr.DSE++
			}
			if mir2.SplitJoinRet(f) {
				tr.SplitJoinRet++
				mir2.EliminateDeadBlocks(f)
			}
			if mir2.CondRetSink(f) {
				tr.CondRetSink++
				mir2.EliminateDeadBlocks(f)
			}
			if mir2.FuseAbsDiff(f) { tr.FuseAbsDiff++ }
		}
		// Re-count cond_ret after optimization (CondRetSink creates them)
		tr.CondRets = 0
		for _, b := range f.Blocks {
			if _, ok := b.Term.(*mir2.TermCondRet); ok {
				tr.CondRets++
			}
		}
	}
	s.MIR2Opt = m.Dump()

	// Structural verification.
	if err := mir2.Verify(m); err != nil {
		return s, fmt.Errorf("MIR2 verify: %w", err)
	}

	// LIR convergence check (non-destructive, parallel to existing codegen).
	if opt.LIRCheck {
		lirResults := lir.CheckModuleConvergence(m)
		for _, r := range lirResults {
			if r.Match {
				_ = r // silent success — logged by caller if needed
			}
		}
		s.LIRResults = lirResults
	}

	// Phase 6f: inline trivial functions (≤4 instructions, single block, leaf).
	// Run BEFORE contract optimisation: inlining removes call-graph edges,
	// shrinking the optimiser's search space and eliminating degenerate cases
	// (e.g. bare-RET swap when the return value is identity-projected).
	if mir2.InlineTrivial(m, 4) {
		for _, f := range m.Funcs {
			mir2.PropagateCopies(f)
			mir2.DeadStoreElim(f)
		}
	}
	// Renumber regs to guarantee uniqueness after inlining may have
	// allocated regs in earlier functions that overlap later functions.
	m.RenumberRegs()

	// Phase 5b: interprocedural contract optimisation (greedy DP on call graph).
	// Run BEFORE LUTGen so that synthetic LUT functions (which have hardcoded
	// class requirements in their Sub instructions) are not re-optimised and
	// given a conflicting class (BUG-004).
	var ct mir2.CostTable
	if opt.Backend == "ez80" {
		ct = ez80.EZ80CostTable{}
	} else {
		ct = mir2.Z80CostTable{}
	}
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
		mir2.PreallocCoalesce(f) // BUG-001 fix: union block-arg/param pairs before PBQP
		lr := mir2.ComputeLiveness(f)
		ar := mir2.PBQPAllocate(f, lr, ct)
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
		combined.Spilled = append(combined.Spilled, ar.Spilled...)
	}

	// MIR2 VM assertion checks (skip "z80"-only asserts).
	if err := RunAssertsMIR2(hm, m); err != nil {
		return s, err
	}

	if opt.UseVIR {
		// VIR backend: unified Z3 solver (joint isel+regalloc in one pass).
		// Pass PBQP param locations so VIR matches the bootstrap ABI.
		funcParamLocs := make(map[string]map[int]int)
		for _, f := range m.Funcs {
			pl := make(map[int]int)
			for _, cp := range f.Contract.Params {
				if loc, ok := combined.Locs[cp.Reg]; ok {
					idx := vir.Z80.LocByName(loc.Name)
					if idx >= 0 {
						pl[int(cp.Reg)] = idx
					}
				}
			}
			if len(pl) > 0 {
				funcParamLocs[f.Name] = pl
			}
		}
		virOpts := vir.SolverOptions{FuncParamLocs: funcParamLocs, OptSize: opt.OptSize}
		virAsm, virResults := vir.CodegenModule(m, virOpts)

		ok, fail := 0, 0
		var failNames []string
		failSet := make(map[string]bool)
		for _, r := range virResults {
			if r.OK {
				ok++
				if tr := s.Traces[r.Name]; tr != nil {
					tr.Backend = "VIR"
				}
			} else {
				fail++
				failNames = append(failNames, r.Name+"("+r.Error+")")
				failSet[r.Name] = true
				if tr := s.Traces[r.Name]; tr != nil {
					tr.Backend = "VIR+PBQP-fallback"
					tr.BackendErr = r.Error
				}
			}
		}

		if fail > 0 {
			// Fallback: generate PBQP asm for failed functions.
			pbqpAsm := mir2.Z80Codegen(m, combined, mir2.Z80CodegenOptions{
				AnnotateTStates: opt.AnnotateTStates,
			})
			s.Assembly = spliceVIRFallback(virAsm, pbqpAsm, virResults, failSet, m)
			fmt.Fprintf(os.Stderr, "vir: %d/%d via Z3 solver, %d via PBQP fallback: %s\n",
				ok, ok+fail, fail, strings.Join(failNames, ", "))
		} else {
			s.Assembly = virAsm
			var strBuf strings.Builder
			lir.EmitStringPool(&strBuf, m)
			s.Assembly += strBuf.String()
			s.Assembly += emitGlobals(m)
			fmt.Fprintf(os.Stderr, "vir: all %d functions compiled via Z3 unified solver\n", ok)
		}
	} else if opt.UseLIR {
		// Convert PBQP allocation to LIR hints for guided WFC.
		hints := pbqpToLIRHints(combined, lir.Z80)

		// LIR backend: ISLE combining + WFC regalloc + Layer 4 CFG rules.
		lirAsm, lirResults := lir.LIRCodegenModule(m, hints)

		// Per-function fallback: functions that LIR can't handle get PBQP asm.
		ok, fail := 0, 0
		var failNames []string
		failSet := make(map[string]bool)
		for _, r := range lirResults {
			if r.OK {
				ok++
				if tr := s.Traces[r.Name]; tr != nil {
					tr.Backend = "LIR"
				}
			} else {
				fail++
				failNames = append(failNames, r.Name+"("+r.Error+")")
				failSet[r.Name] = true
				if tr := s.Traces[r.Name]; tr != nil {
					tr.Backend = "LIR+PBQP-fallback"
					tr.BackendErr = r.Error
				}
			}
		}

		if fail > 0 {
			// Generate PBQP z80codegen for the ENTIRE module (cheap — already allocated).
			pbqpAsm := mir2.Z80Codegen(m, combined, mir2.Z80CodegenOptions{
				AnnotateTStates: opt.AnnotateTStates,
			})
			// Extract per-function asm from PBQP output for failed functions,
			// then splice into the LIR output.
			s.Assembly = splicePerFunctionFallback(lirAsm, pbqpAsm, lirResults, failSet, m)
			// Append LIR-sanitized string pool for LIR functions that reference
			// _mir2_str_N labels. PBQP strings use different sanitization.
			var strBuf strings.Builder
			lir.EmitStringPool(&strBuf, m)
			s.Assembly += strBuf.String()
			fmt.Fprintf(os.Stderr, "lir: %d/%d via ISLE+WFC, %d via PBQP fallback: %s\n",
				ok, ok+fail, fail, strings.Join(failNames, ", "))
		} else {
			s.Assembly = lirAsm
			var strBuf strings.Builder
			lir.EmitStringPool(&strBuf, m)
			s.Assembly += strBuf.String()
			s.Assembly += emitGlobals(m)
			fmt.Fprintf(os.Stderr, "lir: all %d functions compiled via ISLE+WFC+Layer4\n", ok)
		}

		// Store convergence info.
		for _, r := range lirResults {
			cr := lir.ConvergenceResult{
				FuncName: r.Name,
				Machine:  "z80",
				Match:    r.OK,
				Error:    r.Error,
			}
			s.LIRResults = append(s.LIRResults, cr)
		}
	} else {
		for _, f := range m.Funcs {
			if tr := s.Traces[f.Name]; tr != nil {
				tr.Backend = "PBQP"
			}
		}
		s.Assembly = mir2.Z80Codegen(m, combined, mir2.Z80CodegenOptions{
			AnnotateTStates: opt.AnnotateTStates,
		})
	}

	// Emit stubs for @extern functions not already defined in the assembly.
	// When hybrid LIR+PBQP splicing loses empty function stubs, this ensures
	// CALL targets for extern functions resolve.
	s.Assembly = emitExternStubs(s.Assembly, m)

	// Inject per-function trace annotations into the assembly.
	s.Assembly = injectTraceAnnotations(s.Assembly, s.Traces)

	// Post-assembly label audit: find referenced-but-undefined labels.
	labelWarnings := auditLabels(s.Assembly)
	if len(labelWarnings) > 0 {
		fmt.Fprintf(os.Stderr, "[label-audit] %d undefined labels:\n", len(labelWarnings))
		for _, w := range labelWarnings {
			fmt.Fprintf(os.Stderr, "  %s\n", w)
		}
	}

	// Inject module-level compilation summary at the top.
	s.Assembly = injectModuleSummary(s.Assembly, s.Traces, labelWarnings)

	// Deduplicate labels in assembly (hybrid LIR+PBQP may produce duplicates).
	s.Assembly = dedupAsmLabels(s.Assembly)

	// eZ80: wrap Z80/LIR assembly with ADL header.
	if opt.Backend == "ez80" {
		s.Assembly = "; eZ80 ADL mode assembly — generated by MinZ compiler\n" +
			"; Target: Agon Light 2 (eZ80 @ 18.432 MHz)\n\n" +
			"    .ASSUME ADL=1\n" +
			"    ORG $040045\n\n" +
			s.Assembly
	}

	// Z80 binary assertion checks (skip for eZ80 — different encoding).
	if opt.Backend != "ez80" {
		if err := RunAssertsZ80(hm, m, combined, s.Assembly); err != nil {
			return s, err
		}
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
	// Great Autorefactor: split high-pressure functions.
	hir.SplitHighPressure(hm)

	// Lower HIR → MIR2.
	m := hir.LowerModule(hm)

	// Per-function optimisation passes.
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for iter := 0; iter < 100; iter++ {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
		mir2.DeadBlockArgElim(f)
		if mir2.BranchEquiv(m, f) {
			mir2.EliminateDeadBlocks(f)
			mir2.DeadStoreElim(f)
		}
		if mir2.SplitJoinRet(f) {
			mir2.EliminateDeadBlocks(f)
		}
		if mir2.CondRetSink(f) {
			mir2.EliminateDeadBlocks(f)
		}
		mir2.FuseAbsDiff(f)
	}

	// Structural verification.
	if err := mir2.Verify(m); err != nil {
		return "", fmt.Errorf("MIR2 verify: %w", err)
	}

	ct := mir2.Z80CostTable{}

	// Phase 6f: inline trivial functions (≤4 instructions, single block, leaf).
	if mir2.InlineTrivial(m, 4) {
		for _, f := range m.Funcs {
			mir2.PropagateCopies(f)
			mir2.DeadStoreElim(f)
		}
	}
	m.RenumberRegs()

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
		mir2.PreallocCoalesce(f) // BUG-001 fix: union block-arg/param pairs before PBQP
		lr := mir2.ComputeLiveness(f)
		ar := mir2.PBQPAllocate(f, lr, ct)
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
		combined.Spilled = append(combined.Spilled, ar.Spilled...)
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

// RunAssertsMIR2 evaluates compile-time assertions via the MIR2 VM.
// Top-level asserts get a fresh VM each (fully isolated).
// Sandbox blocks share one VM across all their asserts (sequential, shared heap).
// Skips assertions with Via=="z80".
func RunAssertsMIR2(hm *hir.Module, m *mir2.Module) error {
	// Top-level asserts: fresh VM per assert (isolated).
	for _, a := range hm.Asserts {
		if a.Via == "z80" {
			continue
		}
		vm := mir2.NewVM(m)
		prepareVM(vm)
		if err := runOneAssertMIR2(vm, a); err != nil {
			return err
		}
	}
	// Sandbox blocks: one shared VM per sandbox.
	for _, sb := range hm.Sandboxes {
		vm := mir2.NewVM(m)
		prepareVM(vm)
		for _, a := range sb.Asserts {
			if a.Via == "z80" {
				continue
			}
			if err := runOneAssertMIR2(vm, a); err != nil {
				return fmt.Errorf("sandbox %q: %w", sb.Name, err)
			}
		}
	}
	return nil
}

// prepareVM registers optional host functions (canvas, etc.) on a fresh VM.
func prepareVM(vm *mir2.VM) {
	mir2.RegisterCanvasHosts(vm)
}

// runOneAssertMIR2 evaluates a single compile-time assertion on the given VM.
func runOneAssertMIR2(vm *mir2.VM, a hir.Assert) error {
	args := make([]mir2.Value, len(a.Args))
	for i, v := range a.Args {
		args[i] = mir2.Value{I: v}
	}
	// Resolve string args: replace placeholder 0 with actual heap address.
	for idx, sym := range a.StringArgs {
		addr, err := vm.ResolveSymbol(sym)
		if err != nil {
			return fmt.Errorf("line %d: assert %q: string arg %q: %w", a.Line, a.Source, sym, err)
		}
		args[idx] = mir2.Value{I: addr}
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
	return nil
}

const assertLoadAddr = 0x8000

// RunAssertsZ80 evaluates compile-time assertions by assembling the generated Z80
// and running each function call on the MZE emulator.  Skips Via=="mir2" asserts.
//
// Top-level asserts: fresh emulator per assert (isolated).
// Sandbox blocks: one shared emulator — memory (globals) persists between calls.
//
// Uses the actual register allocation (ar) to determine which physical register
// holds each parameter — the contract optimizer may place params in unexpected
// registers (e.g. second u8 param in C rather than B).
func RunAssertsZ80(hm *hir.Module, m *mir2.Module, ar *mir2.AllocResult, asmSrc string) error {
	hasZ80 := false
	for _, a := range hm.Asserts {
		if a.Via != "mir2" {
			hasZ80 = true
			break
		}
	}
	if !hasZ80 {
		for _, sb := range hm.Sandboxes {
			for _, a := range sb.Asserts {
				if a.Via != "mir2" {
					hasZ80 = true
					break
				}
			}
			if hasZ80 {
				break
			}
		}
	}
	if !hasZ80 {
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

	// Top-level asserts: fresh emulator each.
	for _, a := range hm.Asserts {
		if a.Via == "mir2" {
			continue
		}
		z := emulator.NewRemogattoZ80()
		if err := runOneAssertZ80(z, a, mir2Funcs, hirFuncs, ar, asmSrc); err != nil {
			return err
		}
	}

	// Sandbox blocks: one shared emulator per sandbox.
	// First assert loads program code; subsequent asserts only overwrite the
	// trampoline region — globals in Z80 memory persist between calls.
	for _, sb := range hm.Sandboxes {
		z := emulator.NewRemogattoZ80()
		first := true
		for _, a := range sb.Asserts {
			if a.Via == "mir2" {
				continue
			}
			if err := runOneAssertZ80Sandbox(z, a, mir2Funcs, hirFuncs, ar, asmSrc, first); err != nil {
				return fmt.Errorf("sandbox %q: %w", sb.Name, err)
			}
			first = false
		}
	}
	return nil
}

// runOneAssertZ80 runs a single assert on a fresh emulator (top-level, isolated).
func runOneAssertZ80(z *emulator.RemogattoZ80, a hir.Assert,
	mir2Funcs map[string]*mir2.Func, hirFuncs map[string]*hir.Func,
	ar *mir2.AllocResult, asmSrc string) error {

	mf := mir2Funcs[a.FuncName]
	if mf == nil {
		return fmt.Errorf("line %d: assert %q [z80]: function %q not found in MIR2", a.Line, a.Source, a.FuncName)
	}

	boot := buildAssertBootstrap(assertLoadAddr, a, mf, ar)
	src := boot + "\n" + asmSrc

	as := z80asm.NewAssembler()
	res, err := as.AssembleString(src)
	if err != nil {
		return fmt.Errorf("line %d: assert %q [z80]: assemble: %w", a.Line, a.Source, err)
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("line %d: assert %q [z80]: assemble errors: %v", a.Line, a.Source, res.Errors[0])
	}

	if lerr := z.LoadMemory(assertLoadAddr, res.Binary); lerr != nil {
		return fmt.Errorf("line %d: assert %q [z80]: load: %w", a.Line, a.Source, lerr)
	}
	z.SetPC(assertLoadAddr)
	if rerr := z.Run(); rerr != nil {
		return fmt.Errorf("line %d: assert %q [z80]: run: %w", a.Line, a.Source, rerr)
	}

	return checkAssertZ80Result(z, a, mf, hirFuncs)
}

// sandboxTrampolineSize is the fixed size of the trampoline region in sandbox mode.
// All trampolines are padded to this size so the code section starts at a stable
// address, keeping globals at fixed offsets across re-assemblies.
const sandboxTrampolineSize = 64 // bytes — enough for LD SP + 8 args + CALL + DI + HALT

// runOneAssertZ80Sandbox runs a single assert on a shared emulator (sandbox).
// Uses a fixed-size NOP-padded trampoline so the code section (and globals)
// always lives at the same addresses.  On first call, loads everything.
// On subsequent calls, only overwrites the trampoline region — globals persist.
func runOneAssertZ80Sandbox(z *emulator.RemogattoZ80, a hir.Assert,
	mir2Funcs map[string]*mir2.Func, hirFuncs map[string]*hir.Func,
	ar *mir2.AllocResult, asmSrc string, first bool) error {

	mf := mir2Funcs[a.FuncName]
	if mf == nil {
		return fmt.Errorf("line %d: assert %q [z80]: function %q not found in MIR2", a.Line, a.Source, a.FuncName)
	}

	// Build a fixed-size trampoline: bootstrap + NOP padding to sandboxTrampolineSize.
	boot := buildAssertBootstrap(assertLoadAddr, a, mf, ar)
	padCount := sandboxTrampolineSize - trampolineSize(a, mf, ar)
	if padCount < 0 {
		padCount = 0
	}
	nops := ""
	for i := 0; i < padCount; i++ {
		nops += "    NOP\n"
	}
	// Insert NOPs between the ORG line and the first instruction — actually,
	// the bootstrap starts with ORG, then instructions, then HALT.
	// Easier: append NOPs after HALT (they're unreachable, just padding).
	src := boot + nops + asmSrc

	as := z80asm.NewAssembler()
	res, err := as.AssembleString(src)
	if err != nil {
		return fmt.Errorf("line %d: assert %q [z80]: assemble: %w", a.Line, a.Source, err)
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("line %d: assert %q [z80]: assemble errors: %v", a.Line, a.Source, res.Errors[0])
	}

	if first {
		// Load everything: trampoline + code + globals (all zero-init).
		if lerr := z.LoadMemory(assertLoadAddr, res.Binary); lerr != nil {
			return fmt.Errorf("line %d: assert %q [z80]: load: %w", a.Line, a.Source, lerr)
		}
	} else {
		// Only overwrite the trampoline region — code is identical, globals persist.
		for i := 0; i < sandboxTrampolineSize && i < len(res.Binary); i++ {
			z.SetMemory(uint16(assertLoadAddr+i), res.Binary[i])
		}
	}

	z.Unhalt()
	z.SetPC(assertLoadAddr)
	if rerr := z.Run(); rerr != nil {
		return fmt.Errorf("line %d: assert %q [z80]: run: %w", a.Line, a.Source, rerr)
	}

	return checkAssertZ80Result(z, a, mf, hirFuncs)
}

// buildAssertBootstrap generates the ORG + LD SP + LD args + CALL + DI + HALT prefix.
func buildAssertBootstrap(org int, a hir.Assert, mf *mir2.Func, ar *mir2.AllocResult) string {
	var boot strings.Builder
	fmt.Fprintf(&boot, "    ORG 0x%04X\n", org)
	boot.WriteString("    LD SP, 0xFF00\n")
	for i, arg := range a.Args {
		if i >= len(mf.Contract.Params) {
			break
		}
		param := mf.Contract.Params[i]
		// Try PBQP alloc first (matches PBQP codegen and LIR with hints).
		// If PBQP doesn't have this param, fall back to contract class
		// (matches LIR WFC which assigns from contract classes).
		locName := ""
		if ar != nil {
			if loc, ok := ar.Locs[param.Reg]; ok {
				locName = loc.Name
			}
		}
		if locName == "" {
			// Use PFCCO contract class as register name.
			switch param.Class {
			case mir2.ClassAcc:
				locName = "A"
			case mir2.ClassGeneral, mir2.ClassCounter:
				locName = "B" // default general = B on Z80
			case mir2.ClassRegC:
				locName = "C"
			case mir2.ClassRegD:
				locName = "D"
			case mir2.ClassRegE:
				locName = "E"
			case mir2.ClassRegH:
				locName = "H"
			case mir2.ClassRegL:
				locName = "L"
			case mir2.ClassPointer:
				locName = "HL"
			case mir2.ClassIndex:
				locName = "DE"
			case mir2.ClassPair:
				locName = "BC"
			}
		}
		if locName == "" {
			continue
		}
		fmt.Fprintf(&boot, "    LD %s, %d\n", locName, arg)
	}
	fmt.Fprintf(&boot, "    CALL %s\n", a.FuncName)
	boot.WriteString("    DI\n    HALT\n")
	return boot.String()
}

// trampolineSize estimates the byte size of the trampoline bootstrap.
// LD SP,nn (3) + per-arg LD r,n (2) or LD rr,nn (3) + CALL nn (3) + DI (1) + HALT (1).
func trampolineSize(a hir.Assert, mf *mir2.Func, ar *mir2.AllocResult) int {
	size := 3 // LD SP, 0xFF00
	for i := range a.Args {
		if i >= len(mf.Contract.Params) {
			break
		}
		param := mf.Contract.Params[i]
		loc, ok := ar.Locs[param.Reg]
		if !ok {
			continue
		}
		switch loc.Name {
		case "HL", "DE", "BC", "IX", "IY":
			size += 3 // LD rr, nn
		default:
			size += 2 // LD r, n
		}
	}
	size += 3 // CALL nn
	size += 1 // DI
	size += 1 // HALT
	return size
}

// checkAssertZ80Result reads the result register and compares against expected.
func checkAssertZ80Result(z *emulator.RemogattoZ80, a hir.Assert,
	mf *mir2.Func, hirFuncs map[string]*hir.Func) error {

	regs := z.GetRegisters()
	var got int64
	if len(mf.Contract.Returns) > 0 {
		switch mf.Contract.Returns[0].Class {
		case mir2.ClassPointer:
			got = int64(regs.HL)
		case mir2.ClassIndex:
			got = int64(regs.DE)
		case mir2.ClassPair:
			got = int64(regs.BC)
		default:
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
	// Apply target-specific output format (e.g. Agon MOS header).
	if target != "" {
		t2, _ := z80asm.ParseTarget(target)
		cfg := z80asm.GetTargetConfig(t2)
		if cfg != nil && cfg.OutputFormat.Generator != nil {
			formatted, fmtErr := cfg.OutputFormat.Generator(res)
			if fmtErr != nil {
				return nil, []error{fmtErr}
			}
			return formatted, nil
		}
	}
	return res.Binary, nil
}

// pbqpToLIRHints converts a PBQP allocation result to LIR AllocHints.
// Maps each vreg's assigned physical register name to the corresponding
// LIR loc index in the machine descriptor.
func pbqpToLIRHints(ar *mir2.AllocResult, desc *lir.MachineDesc) lir.AllocHints {
	if ar == nil || len(ar.Locs) == 0 {
		return nil
	}
	hints := make(lir.AllocHints, len(ar.Locs))
	for reg, loc := range ar.Locs {
		// Map by name — only include if the LIR descriptor knows this location.
		lirIdx := desc.LocByName(loc.Name)
		if lirIdx >= 0 {
			hints[int(reg)] = lirIdx
		}
	}
	return hints
}

// splicePerFunctionFallback builds a combined assembly output using LIR asm
// for functions that succeeded and PBQP asm for functions that failed.
// Both outputs use "; fun <name>(...)" comment headers to delimit functions.
func splicePerFunctionFallback(lirAsm, pbqpAsm string, results []lir.LIRFuncResult, failSet map[string]bool, m *mir2.Module) string {
	// Extract per-function blocks from PBQP output.
	pbqpFuncs := splitAsmByFunction(pbqpAsm)

	// Build the combined output: LIR header + per-function blocks.
	var sb strings.Builder
	sb.WriteString("; generated by LIR+PBQP hybrid backend\n\n")

	// Use the LIR output as base — it already has the right function order
	// and only contains successful functions. For failed functions, insert
	// the PBQP version.
	lirFuncs := splitAsmByFunction(lirAsm)

	// Emit functions in module order (same as Z80Codegen).
	funcs := m.Funcs
	if main := m.FuncByName("main"); main != nil && len(funcs) > 0 && funcs[0].Name != "main" {
		ordered := make([]*mir2.Func, 0, len(funcs))
		ordered = append(ordered, main)
		for _, f := range funcs {
			if f.Name != "main" {
				ordered = append(ordered, f)
			}
		}
		funcs = ordered
	}

	for _, f := range funcs {
		if failSet[f.Name] {
			// Use PBQP asm for this function.
			if asm, ok := pbqpFuncs[f.Name]; ok {
				sb.WriteString(asm)
			}
		} else {
			// Use LIR asm for this function.
			if asm, ok := lirFuncs[f.Name]; ok {
				sb.WriteString(asm)
			}
		}
	}

	// Append globals.
	sb.WriteString(emitGlobals(m))

	// Append ALL content from PBQP output after the last function.
	// This includes spill labels (_spill_*, $F0xx), strings, and any
	// trailing data that PBQP functions reference.
	// dedupAsmLabels() will remove duplicates before assembly.
	if idx := strings.LastIndex(pbqpAsm, "\n; spill"); idx >= 0 {
		sb.WriteString(pbqpAsm[idx:])
	} else if idx := strings.LastIndex(pbqpAsm, "\n; strings"); idx >= 0 {
		sb.WriteString(pbqpAsm[idx:])
	}

	return sb.String()
}

// auditLabels scans assembly text for referenced-but-undefined labels.
// Returns a list of warning strings for each undefined label found.
func auditLabels(asm string) []string {
	defined := make(map[string]bool)
	referenced := make(map[string]bool)
	lines := strings.Split(asm, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}
		// Label definition: "label:" at start (possibly with indent)
		if idx := strings.IndexByte(trimmed, ':'); idx > 0 {
			candidate := trimmed[:idx]
			// Must be a valid label (letters, digits, _, $, .)
			if isLabel(candidate) && !isInstruction(candidate) {
				defined[candidate] = true
			}
		}
		// References: operands after instructions
		// Look for _spill_*, _tsmc_*, _mir2_str_*, _vir_mem*, and function calls
		for _, prefix := range []string{"_spill_", "_tsmc_", "_mir2_str_", "_vir_mem"} {
			for idx := 0; ; {
				pos := strings.Index(trimmed[idx:], prefix)
				if pos < 0 {
					break
				}
				pos += idx
				end := pos
				for end < len(trimmed) && (trimmed[end] == '_' || trimmed[end] == '.' ||
					(trimmed[end] >= 'a' && trimmed[end] <= 'z') ||
					(trimmed[end] >= 'A' && trimmed[end] <= 'Z') ||
					(trimmed[end] >= '0' && trimmed[end] <= '9')) {
					end++
				}
				label := trimmed[pos:end]
				if len(label) > len(prefix) {
					referenced[label] = true
				}
				idx = end
			}
		}
		// CALL/JP/JR targets
		for _, inst := range []string{"CALL ", "JP ", "JR ", "JP NZ, ", "JP Z, ", "JP NC, ", "JP C, ",
			"JR NZ, ", "JR Z, ", "JR NC, ", "JR C, ", "DJNZ "} {
			if idx := strings.Index(trimmed, inst); idx >= 0 {
				target := strings.TrimSpace(trimmed[idx+len(inst):])
				// Remove trailing comments
				if ci := strings.IndexByte(target, ';'); ci >= 0 {
					target = strings.TrimSpace(target[:ci])
				}
				if isLabel(target) && len(target) > 0 {
					referenced[target] = true
				}
			}
		}
	}

	var warnings []string
	for ref := range referenced {
		if !defined[ref] {
			warnings = append(warnings, ref)
		}
	}
	sort.Strings(warnings)
	return warnings
}

// isLabel returns true if s looks like a valid assembly label.
func isLabel(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := s[0]
	if first != '_' && first != '.' && !(first >= 'a' && first <= 'z') && !(first >= 'A' && first <= 'Z') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c != '_' && c != '.' && c != '$' &&
			!(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') &&
			!(c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// isInstruction returns true if s is a Z80 instruction mnemonic (to avoid
// treating "LD:" or "CP:" as labels in the audit).
func isInstruction(s string) bool {
	upper := strings.ToUpper(s)
	switch upper {
	case "LD", "CP", "ADD", "SUB", "AND", "OR", "XOR", "INC", "DEC",
		"PUSH", "POP", "CALL", "RET", "JP", "JR", "NOP", "HALT", "DI", "EI",
		"DB", "DW", "DS", "ORG", "EQU", "INCLUDE":
		return true
	}
	return false
}

// emitExternStubs appends label stubs for @extern functions that are called
// in the assembly but have no label definition. This happens when hybrid
// LIR+PBQP splicing loses LIR's empty-function stubs.
func emitExternStubs(asm string, m *mir2.Module) string {
	var stubs strings.Builder
	for _, f := range m.Funcs {
		if len(f.Blocks) > 0 {
			continue // not extern — has a body
		}
		label := lir.SanitizeAsmLabel(f.Name)
		// Only emit stub if the label is referenced but not defined.
		if strings.Contains(asm, label) && !strings.Contains(asm, label+":") {
			fmt.Fprintf(&stubs, "; %s — extern stub\n%s:\n    RET\n\n", label, label)
		}
	}
	if stubs.Len() > 0 {
		return asm + stubs.String()
	}
	return asm
}

// injectModuleSummary prepends a compilation summary block at the top of the assembly.
func injectModuleSummary(asm string, traces map[string]*FuncTrace, labelWarnings []string) string {
	if len(traces) == 0 {
		return asm
	}
	var sb strings.Builder
	total := len(traces)
	counts := map[string]int{}  // backend → count
	splits, fallbacks := 0, 0
	totalPasses := 0
	var fallbackNames []string
	for _, tr := range traces {
		counts[tr.Backend]++
		if tr.SplitFrom != "" { splits++ }
		if tr.BackendErr != "" {
			fallbacks++
			fallbackNames = append(fallbackNames, tr.Name)
		}
		totalPasses += tr.ConstProp + tr.ConstFold + tr.IdentSimp + tr.CallElim +
			tr.DSE + tr.DeadBlockArg + tr.BranchEquiv + tr.SplitJoinRet +
			tr.CondRetSink + tr.FuseAbsDiff
	}
	sb.WriteString("; ── compilation summary ──────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("; functions: %d", total))
	for _, be := range []string{"LIR", "VIR", "PBQP", "LIR+PBQP-fallback", "VIR+PBQP-fallback"} {
		if n := counts[be]; n > 0 {
			sb.WriteString(fmt.Sprintf("  %s=%d", be, n))
		}
	}
	sb.WriteByte('\n')
	if splits > 0 {
		sb.WriteString(fmt.Sprintf("; splits: %d (HIR-SPLIT high-pressure)\n", splits))
	}
	if fallbacks > 0 {
		sb.WriteString(fmt.Sprintf("; fallbacks: %d [%s]\n", fallbacks, strings.Join(fallbackNames, ", ")))
	}
	sb.WriteString(fmt.Sprintf("; optimization passes fired: %d\n", totalPasses))
	if len(labelWarnings) > 0 {
		sb.WriteString(fmt.Sprintf("; LABEL AUDIT: %d undefined labels\n", len(labelWarnings)))
		for _, w := range labelWarnings {
			sb.WriteString(fmt.Sprintf(";   %s\n", w))
		}
	} else {
		sb.WriteString("; label audit: OK\n")
	}
	sb.WriteString("; ─────────────────────────────────────────────────────────────\n")
	return sb.String() + asm
}

// injectTraceAnnotations inserts "; [trace] ..." comments after each
// "; fun NAME(...)" header line in the assembly. This provides per-function
// compilation provenance: backend, passes, split info, fallback reason.
func injectTraceAnnotations(asm string, traces map[string]*FuncTrace) string {
	if len(traces) == 0 {
		return asm
	}
	lines := strings.Split(asm, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, line)
		// Match "; fun NAME(" or "; NAME — LIR codegen"
		if name := extractFuncName(line); name != "" {
			if tr := traces[name]; tr != nil {
				result = append(result, formatTrace(tr))
			}
		}
	}
	return strings.Join(result, "\n")
}

// extractFuncName extracts the function name from a "; fun NAME(...)" or
// "; NAME — LIR codegen" comment line.
func extractFuncName(line string) string {
	trimmed := strings.TrimSpace(line)
	// "; fun NAME(" — MIR2/VIR format
	if strings.HasPrefix(trimmed, "; fun ") {
		rest := trimmed[6:]
		if idx := strings.IndexByte(rest, '('); idx > 0 {
			return rest[:idx]
		}
		// "; fun NAME" without parens (stub functions)
		if idx := strings.IndexByte(rest, ' '); idx > 0 {
			return rest[:idx]
		}
		return strings.TrimSpace(rest)
	}
	// "; NAME — LIR codegen" format
	if strings.HasPrefix(trimmed, "; ") && strings.Contains(trimmed, " — LIR codegen") {
		rest := trimmed[2:]
		if idx := strings.Index(rest, " — "); idx > 0 {
			return rest[:idx]
		}
	}
	return ""
}

// formatTrace formats a FuncTrace as a single ASM comment line.
func formatTrace(tr *FuncTrace) string {
	var parts []string
	parts = append(parts, "backend="+tr.Backend)
	if tr.SplitFrom != "" {
		parts = append(parts, fmt.Sprintf("split-from=%s(pressure=%d)", tr.SplitFrom, tr.SplitPressure))
	}
	if tr.BackendErr != "" {
		parts = append(parts, "fallback-reason="+tr.BackendErr)
	}

	// Optimization passes — only show non-zero
	var passes []string
	if tr.ConstProp > 0 { passes = append(passes, fmt.Sprintf("const-prop=%d", tr.ConstProp)) }
	if tr.ConstFold > 0 { passes = append(passes, fmt.Sprintf("const-fold=%d", tr.ConstFold)) }
	if tr.IdentSimp > 0 { passes = append(passes, fmt.Sprintf("ident-simp=%d", tr.IdentSimp)) }
	if tr.CallElim > 0 { passes = append(passes, fmt.Sprintf("call-elim=%d", tr.CallElim)) }
	if tr.DSE > 0 { passes = append(passes, fmt.Sprintf("dse=%d", tr.DSE)) }
	if tr.DeadBlockArg > 0 { passes = append(passes, fmt.Sprintf("dead-block-arg=%d", tr.DeadBlockArg)) }
	if tr.BranchEquiv > 0 { passes = append(passes, fmt.Sprintf("branch-equiv=%d", tr.BranchEquiv)) }
	if tr.SplitJoinRet > 0 { passes = append(passes, fmt.Sprintf("split-join-ret=%d", tr.SplitJoinRet)) }
	if tr.CondRetSink > 0 { passes = append(passes, fmt.Sprintf("condret-sink=%d", tr.CondRetSink)) }
	if tr.FuseAbsDiff > 0 { passes = append(passes, fmt.Sprintf("fuse-abs-diff=%d", tr.FuseAbsDiff)) }
	if tr.CondRets > 0 { passes = append(passes, fmt.Sprintf("cond-rets=%d", tr.CondRets)) }
	if tr.Inlined { passes = append(passes, "inlined") }
	if tr.LUTReplaced { passes = append(passes, "lut-replaced") }

	if len(passes) > 0 {
		parts = append(parts, "passes=["+strings.Join(passes, ",")+"]")
	}
	if len(tr.LabelWarnings) > 0 {
		parts = append(parts, fmt.Sprintf("label-warnings=%d", len(tr.LabelWarnings)))
	}
	return "; [trace] " + strings.Join(parts, " ")
}

// dedupAsmLabels removes duplicate label definitions from assembly text.
// When hybrid LIR+PBQP output contains the same label twice (e.g. globals
// emitted by both paths), keep only the first definition.
func dedupAsmLabels(asm string) string {
	lines := strings.Split(asm, "\n")
	seen := make(map[string]bool)
	var result []string
	skipUntilNext := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect label definition: "name:" at start of line (not indented instruction)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") &&
			strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, ";") &&
			!strings.HasPrefix(trimmed, ".") { // skip local labels
			label := strings.TrimSuffix(trimmed, ":")
			if seen[label] {
				// Skip this duplicate label and its data (DB/DW lines following it)
				skipUntilNext = true
				continue
			}
			seen[label] = true
			skipUntilNext = false
		} else if skipUntilNext {
			// Skip data lines (DB, DW) belonging to duplicate label
			if strings.HasPrefix(trimmed, "DB ") || strings.HasPrefix(trimmed, "DW ") ||
				trimmed == "" {
				continue
			}
			skipUntilNext = false
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// extractPBQPTrailingData extracts spill page and other non-function/non-global
// data from PBQP output. This includes $F0xx spill labels and _spill_ vars.
// Skips globals ("; globals" section) and strings ("; strings" section) to
// avoid duplicates with LIR-emitted versions.
func extractPBQPTrailingData(pbqpAsm string) string {
	var result strings.Builder
	lines := strings.Split(pbqpAsm, "\n")
	inSpill := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Detect spill page markers
		if strings.HasPrefix(trimmed, "$F0") || strings.HasPrefix(trimmed, "_spill_") {
			inSpill = true
		}
		if inSpill {
			// Stop at globals/strings sections
			if trimmed == "; globals" || trimmed == "; strings" {
				inSpill = false
				continue
			}
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}

// splitAsmByFunction splits Z80 assembly text into per-function blocks.
// Functions are delimited by "; fun <name>(...)" comment lines.
// Returns map[funcName]asmText where asmText includes everything from
// the "; fun" comment through to (but not including) the next "; fun" or end.
func splitAsmByFunction(asm string) map[string]string {
	funcs := make(map[string]string)
	lines := strings.Split(asm, "\n")
	var curName string
	var curStart int

	flush := func(end int) {
		if curName != "" {
			funcs[curName] = strings.Join(lines[curStart:end], "\n") + "\n"
		}
	}

	for i, line := range lines {
		if strings.HasPrefix(line, "; fun ") {
			flush(i)
			// PBQP format: "; fun name(...)"
			rest := line[6:] // skip "; fun "
			if paren := strings.IndexByte(rest, '('); paren >= 0 {
				curName = rest[:paren]
			} else {
				curName = rest
			}
			curStart = i
		} else if strings.HasPrefix(line, "; ") && strings.Contains(line, " — LIR codegen") {
			flush(i)
			// LIR format: "; name — LIR codegen (N insts, ...)"
			rest := line[2:] // skip "; "
			if dash := strings.Index(rest, " —"); dash >= 0 {
				curName = rest[:dash]
			}
			curStart = i
		}
	}
	flush(len(lines))
	return funcs
}

// spliceVIRFallback combines VIR output with PBQP fallback for failed functions.
func spliceVIRFallback(virAsm, pbqpAsm string, results []vir.FuncResult, failSet map[string]bool, m *mir2.Module) string {
	pbqpFuncs := splitAsmByFunction(pbqpAsm)
	virFuncs := splitAsmByFunction(virAsm)

	var sb strings.Builder
	sb.WriteString("; generated by VIR+PBQP hybrid backend\n\n")

	funcs := m.Funcs
	if main := m.FuncByName("main"); main != nil && len(funcs) > 0 && funcs[0].Name != "main" {
		ordered := make([]*mir2.Func, 0, len(funcs))
		ordered = append(ordered, main)
		for _, f := range funcs {
			if f.Name != "main" {
				ordered = append(ordered, f)
			}
		}
		funcs = ordered
	}

	for _, f := range funcs {
		if failSet[f.Name] {
			if asm, ok := pbqpFuncs[f.Name]; ok {
				sb.WriteString(asm)
				sb.WriteByte('\n')
			}
		} else {
			if asm, ok := virFuncs[f.Name]; ok {
				sb.WriteString(asm)
				sb.WriteByte('\n')
			}
		}
	}

	sb.WriteString(emitGlobals(m))
	return sb.String()
}

// emitGlobals generates Z80 assembly for global variables in a MIR2 module.
// This is a simplified version of the globals portion of mir2.Z80Codegen,
// used when the LIR backend handles function codegen.
func emitGlobals(m *mir2.Module) string {
	if len(m.Globals) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("; globals\n")
	for _, g := range m.Globals {
		w := mir2.ByteWidth(g.Ty)
		if len(g.Init) > w {
			w = len(g.Init)
		}
		name := lir.SanitizeAsmLabel(g.Name)
		if w == 0 {
			sb.WriteString(name + ":\n")
			continue
		}
		sb.WriteString(name + ":\n")
		sb.WriteString("    DB ")
		for i := 0; i < w; i++ {
			if i > 0 {
				sb.WriteString(", ")
			}
			b := byte(0)
			if i < len(g.Init) {
				b = g.Init[i]
			}
			fmt.Fprintf(&sb, "%d", b)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
