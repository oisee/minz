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

	"github.com/minz/minzc/pkg/asmutil"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/ez80"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2llvm"
	"github.com/minz/minzc/pkg/mir2wasm"
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
	VariantDecisions []VariantDecision
	VariantSearch    VariantSearchStats

	UnsignedPairComparePolicy mir2.UnsignedPairComparePolicy // resolved policy replayed by native re-codegen
	ProfileDecisions          []hir.ProfileDecision
	HIRLoopSites              *hir.LoopSiteMap      // optional input-HIR origins, before Grace/splitting
	HIR                       string                // HIR structural dump (hir.Module.Dump())
	MIR2Raw                   string                // MIR2 module dump before optimisation passes
	MIR2Opt                   string                // MIR2 module dump after DSE + ReorderBlocks
	MIR2Module                *mir2.Module          // MIR2 module (for LLVM/WASM/ABAP emit)
	Allocation                *mir2.AllocResult     // physical locations used by Z80 codegen
	Assembly                  string                // Final .a80 text
	Traces                    map[string]*FuncTrace // per-function compilation trace (keyed by func name)
}

// FuncTrace records the full compilation provenance for one function.
// This data is emitted as ASM comment annotations for auditability.
type FuncTrace struct {
	Variant string // selected static-search recipe/model, if search ran

	Name          string // function name (mangled)
	SplitFrom     string // non-empty if this function was created by HIR-SPLIT
	SplitPressure int    // register pressure that triggered the split

	// Optimization pass counts (MIR2 level)
	ConstProp    int  // PropagateConstants iterations that changed something
	ConstFold    int  // FoldConstants
	IdentSimp    int  // SimplifyIdentities
	CallElim     int  // ConstantCallElim
	DSE          int  // DeadStoreElim
	DeadBlockArg int  // DeadBlockArgElim
	BranchEquiv  int  // BranchEquiv
	SplitJoinRet int  // SplitJoinRet
	CondRetSink  int  // CondRetSink
	FuseAbsDiff  int  // FuseAbsDiff
	PtrThreading int  // MIR2 loop-carried pointer threading
	PtrAddCSE    int  // MIR2 ptr_add local CSE
	Inlined      bool // function was inlined by InlineTrivial
	LUTReplaced  bool // function was replaced by LUTGen

	// Codegen provenance
	Backend    string // "VIR", "PBQP", "checked-local-table", "VIR+PBQP-fallback"
	BackendErr string // error message if primary backend failed (before fallback)
	WFCAttempt int    // WFC retry attempt (0 = first try)
	CondRets   int    // number of cond_ret terminators

	// Label audit
	LabelWarnings []string // referenced-but-undefined labels (if any)
}

// Options configures optional pipeline passes.
type Options struct {
	// DisableVariantSearch preserves the pre-search compilation policy.
	DisableVariantSearch bool `json:",omitempty"`
	// ForceVariantSearch searches modules larger than the default limit.
	ForceVariantSearch bool `json:",omitempty"`
	// FunctionVariants overrides recipes in complete-module candidate compiles.
	FunctionVariants map[string]FunctionVariant `json:",omitempty"`
	variantCandidate bool
	// Search-only checkpoint: false refuses a trial before native compilation.
	variantPrepared          bool
	variantRawCallers        map[string]bool
	variantMIRReady          func(*mir2.Module, bool) bool
	variantBranchEquivMisses map[[32]byte]bool

	// DisableCountedLoops is an internal differential-testing switch.
	DisableCountedLoops bool `json:",omitempty"`
	// CPMApplicationEntry selects ordinary native CP/M application entry,
	// independently of the guarded runtime and semantic bare/called harnesses.
	CPMApplicationEntry bool            `json:",omitempty"`
	ZXApplicationEntry  bool            `json:",omitempty"`
	Runtime             mir2.Z80Runtime `json:",omitempty"`
	RuntimeStackBytes   int             `json:",omitempty"`
	// CheckedLocalTablePath opts into admitted leaf function replacements under
	// OptSize. The selected function must assemble smaller than PBQP baseline.
	CheckedLocalTablePath string `json:",omitempty"`
	// CheckedLocalTableSHA256 optionally pins the exact JSON bytes. A mismatch is
	// an input error; it must never silently use another table or fallback.
	CheckedLocalTableSHA256 string `json:",omitempty"`

	UnsignedPairComparePolicy mir2.UnsignedPairComparePolicy `json:",omitempty"` // explicit opt-in; zero preserves Legacy
	ProfileSelection          *ProfileSelection              // opt-in refusing PGO input
	CollectHIRSites           bool                           // diagnostic loop sites; does not enable PGO
	// ContractOpt enables Phase 5b interprocedural contract optimisation.
	// Default true — only disable for A/B comparison or debugging.
	ContractOpt bool
	// ContractStrategy opts into joint PFCCO choices; empty keeps greedy.
	ContractStrategy string `json:",omitempty"`
	// AnnotateTStates adds T-state cost comments to every Z80 instruction line.
	AnnotateTStates bool
	// FuseByteWords opts into effect-safe sequential little-endian byte fusion.
	FuseByteWords bool `json:",omitempty"`
	// UseGrace replaces hand-coded optimization passes with declarative Grace rules.
	// GraceStats (if non-nil) collects per-rule application counts.
	UseGrace   bool
	GraceStats *mir2.GraceStats
	// UseHIRGrace runs structural HIR rules before splitting and lowering.
	// Independent of UseGrace, which controls MIR2 rules.
	UseHIRGrace bool
	// OptSize enables size optimizations (Grace reroll: repeated CALLs → DJNZ loop)
	// and excludes HIR Grace's code-growing versioned pointer-walk.
	// Trades ~4T/iteration for code size reduction. Use for ROM-constrained targets.
	OptSize bool

	// Internal baseline compilation for the affine-pointer size comparison.
	disableAffinePointers bool
	// Backend selects the codegen backend: "z80" (default), "ez80".
	// When "ez80", the pipeline generates eZ80 ADL assembly instead of Z80.
	Backend string
	// AssertMode controls which assert backends run:
	//   "" or "all" = both MIR2 VM + Z80 emulator (default)
	//   "mir2"      = MIR2 VM only (skip Z80 — use when Z80 codegen has known bugs)
	//   "z80"       = Z80 emulator only (skip MIR2)
	//   "none"      = skip all asserts
	AssertMode string
	// SkipZ80Emission asks CompileHIRSteps for MIR2/intermediate outputs only.
	// CompileHIRWithOptions always returns assembly. Z80 assertions
	// still require emission and validation, regardless of this option.
	SkipZ80Emission bool
}

// DefaultOptions returns the default pipeline options.
// Production uses MIR Grace and PBQP; HIR Grace and checked local tables are opt-in.
// Explicit Options retain their boolean values for pass-by-pass comparisons.
func DefaultOptions() Options {
	return Options{ContractOpt: true, UseGrace: true}
}

// CompileHIRSteps runs the full HIR→MIR2→Z80 pipeline and returns all intermediate outputs.
func CompileHIRSteps(hm *hir.Module, opts ...Options) (Steps, error) {
	// Runtime and ABI admission below inspect fields and traverse bodies.
	if err := hir.Verify(hm); err != nil {
		return Steps{}, err
	}
	if variantSearchEnabled(hm, opts) {
		return compileVariantSearch(hm, opts...)
	}
	opt := DefaultOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	if needsZXApplication(hm) && !opt.SkipZ80Emission {
		opt.ZXApplicationEntry = true
	}
	var s Steps
	if opt.ZXApplicationEntry && (hm.Target != hir.TargetZXSpectrum || opt.Runtime != "" || opt.CPMApplicationEntry || opt.Backend != "" && opt.Backend != "z80" || opt.ProfileSelection != nil || opt.SkipZ80Emission || len(hm.Asserts) > 0 || len(hm.Sandboxes) > 0) {
		return s, fmt.Errorf("ZX application requires emitted ordinary ZX Z80 without assertion/profile workloads")
	}
	if opt.CheckedLocalTableSHA256 != "" && opt.CheckedLocalTablePath == "" {
		return s, fmt.Errorf("checked local table: SHA256 requires a path")
	}
	if err := ValidateContractStrategy(opt); err != nil {
		return s, err
	}
	policy, err := resolveUnsignedPairComparePolicy(opt)
	if err != nil {
		return s, err
	}
	if err := mir2.ValidateRuntimeStack(opt.Runtime, opt.RuntimeStackBytes); err != nil {
		return Steps{}, err
	}
	if opt.Runtime == mir2.Z80RuntimeAgonZ80 && opt.AssertMode != "none" && (len(hm.Asserts) > 0 || len(hm.Sandboxes) > 0) {
		return s, fmt.Errorf("agon-z80 application runtime assertions require a separate workload; use AssertMode none")
	}
	if opt.Runtime != "" && opt.AssertMode != "none" && len(hm.Asserts) > 0 {
		return s, fmt.Errorf("CP/M application runtime assertions require a separate workload; use AssertMode none")
	}
	if opt.Runtime == mir2.Z80RuntimeAgonZ80 {
		if hm.Target != hir.TargetAgon || opt.Backend != "" && opt.Backend != "z80" || opt.ProfileSelection != nil || opt.SkipZ80Emission || opt.CheckedLocalTablePath != "" {
			return s, fmt.Errorf("agon-z80 runtime requires emitted Agon Z80 without measured profile selection")
		}
		// Inspect source HIR before inlining/pruning. Opaque dead bodies or
		// caller-supplied Internal markers must not grant native ABI admission.
		for _, fact := range hir.AnalyzeFacts(hm).Functions {
			if fact.Direct&hir.UnknownEffect != 0 || fact.Recursive {
				return s, fmt.Errorf("agon-z80 rejects opaque/unsupported or recursive HIR body %s", fact.Name)
			}
		}
	} else if opt.Runtime != "" && (opt.Runtime != mir2.Z80RuntimeCPM || hm.Target != hir.TargetCPM || opt.Backend != "" && opt.Backend != "z80" || opt.ProfileSelection != nil || opt.SkipZ80Emission) {
		return s, fmt.Errorf("CP/M runtime requires emitted CPM Z80 without measured profile selection")
	}
	if opt.CPMApplicationEntry && (opt.Runtime != "" || hm.Target != hir.TargetCPM || opt.Backend != "" && opt.Backend != "z80" || opt.ProfileSelection != nil || opt.SkipZ80Emission || opt.AssertMode != "none" && (len(hm.Asserts) > 0 || len(hm.Sandboxes) > 0)) {
		return s, fmt.Errorf("CP/M application entry requires emitted CPM Z80 without guarded runtime, assertion workloads or measured profile selection")
	}
	if opt.CheckedLocalTablePath != "" && (!opt.OptSize || opt.Backend != "" && opt.Backend != "z80" || opt.SkipZ80Emission || opt.ProfileSelection != nil && opt.CheckedLocalTableSHA256 == "") {
		return s, fmt.Errorf("checked local tables require emitted Z80 OptSize and a content pin for PGO")
	}
	if !opt.variantPrepared {
		hir.OptimizeCAggregateABI(hm)
	}
	if err := hir.Verify(hm); err != nil {
		return Steps{}, fmt.Errorf("c aggregate ABI: %w", err)
	}
	s.UnsignedPairComparePolicy = policy
	s.Traces = make(map[string]*FuncTrace)

	if !opt.variantPrepared {
		if err := hir.ResolveErrorABI(hm); err != nil {
			return s, err
		}
	}
	if opt.Backend != "" && opt.Backend != "z80" {
		for _, f := range hm.Funcs {
			if f.FallibleCY {
				return s, fmt.Errorf("CY error ABI requires Z80 backend")
			}
		}
	}
	if !opt.variantPrepared {
		if err := hir.ResolveCallSignatures(hm); err != nil {
			return s, err
		}
	}

	if opt.CollectHIRSites {
		var err error
		s.HIRLoopSites, err = hir.BuildLoopSiteMap(hm)
		if err != nil {
			return s, err
		}
	}
	if opt.ProfileSelection != nil {
		decisions, err := applyPipelineProfile(hm, opt)
		if err != nil {
			return s, err
		}
		s.ProfileDecisions = decisions
	}
	// Capture HIR before lowering.
	s.HIR = hm.Dump()
	if variantHasHIRGrace(hm, opt) {
		if _, err := hir.RunGraceWithOptions(hm, hir.GraceOptions{OptimizeForSize: opt.OptSize, Functions: variantHIRFunctions(hm, opt)}); err != nil {
			return s, err
		}
		s.HIR = hm.Dump()
	}

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

	if err := hir.ResolveCallSignatures(hm); err != nil {
		return s, err
	}
	// Lower HIR → MIR2 (raw, before optimisation).
	m := hir.LowerModuleWithOptions(hm, hir.LowerOptions{DisableCountedLoops: opt.DisableCountedLoops, CountedLoopPolicy: variantCountedPolicies(opt)})
	s.MIR2Raw = m.Dump()
	// Record call dependencies before constant folding can erase them. A global
	// dry Grace toggle alone cannot establish a caller's axis is an identity.
	if opt.variantRawCallers != nil {
		for _, f := range m.Funcs {
			for _, b := range f.Blocks {
				for _, inst := range b.Insts {
					if inst.Op == mir2.OpCall || inst.Op == mir2.OpCallIndirect {
						opt.variantRawCallers[f.Name] = true
					}
				}
			}
		}
	}
	return compileMIRSteps(hm, opt, s, splitInfo, m)
}

// The size baseline starts at the same lowered HIR, without repeating source
// transformations or profile selection. Both alternatives run the full MIR,
// contract, allocation and codegen pipeline, so spills count toward the gate.
func compileMIRSteps(hm *hir.Module, opt Options, s Steps, splitInfo map[string]hir.SplitResult, m *mir2.Module) (Steps, error) {
	initialSteps := s
	s.Traces = make(map[string]*FuncTrace)
	affineChanged := false
	var err error

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
			if p {
				tr.ConstProp++
			}
			if c {
				tr.ConstFold++
			}
			if si {
				tr.IdentSimp++
			}
			if e {
				tr.CallElim++
			}
			if !p && !c && !si && !e {
				break
			}
		}
		if variantFor(opt, f.Name).MIRGrace {
			// ── Grace declarative path ──────────────────────────────
			localGrace := mir2.NewGraceStats()
			mir2.RunGracePassesWithOptions(f, localGrace, mir2.GracePassOptions{OptimizeForSize: opt.OptSize})
			mergeGraceStats(opt.GraceStats, localGrace)
			applyGraceStatsToTrace(tr, localGrace)
			mir2.EliminateDeadBlocks(f)
			if runVariantBranchEquiv(m, f, opt.variantBranchEquivMisses) {
				tr.BranchEquiv++
				mir2.EliminateDeadBlocks(f)
				localGrace = mir2.NewGraceStats()
				mir2.RunGracePassesWithOptions(f, localGrace, mir2.GracePassOptions{OptimizeForSize: opt.OptSize})
				mergeGraceStats(opt.GraceStats, localGrace)
				applyGraceStatsToTrace(tr, localGrace)
				mir2.EliminateDeadBlocks(f)
			}
			for _, blk := range f.Blocks {
				mir2.FusionSubCmpInBlock(blk)
			}
		} else {
			// ── Go original path ────────────────────────────────────
			mir2.DeadStoreElim(f)
			tr.DSE++
			if mir2.DeadBlockArgElim(f) {
				tr.DeadBlockArg++
			}
			if runVariantBranchEquiv(m, f, opt.variantBranchEquivMisses) {
				tr.BranchEquiv++
				mir2.EliminateDeadBlocks(f)
				mir2.DeadStoreElim(f)
				tr.DSE++
			}
			if mir2.SplitJoinRet(f) {
				tr.SplitJoinRet++
				mir2.EliminateDeadBlocks(f)
			}
			if mir2.CondRetSink(f) {
				tr.CondRetSink++
				mir2.EliminateDeadBlocks(f)
			}
			if mir2.FuseAbsDiff(f) {
				tr.FuseAbsDiff++
			}
		}
		mir2.HoistDiamondCommon(f)
		mir2.PrepareShiftCarry(f)
		// Return sinking/splitting (including Grace rewrites) can leave newly
		// dead edge arguments. Remove them before allocation sees their copies.
		if mir2.DeadBlockArgElim(f) {
			tr.DeadBlockArg++
		}
		// Run after DSE exposes the byte subtractions in both returning arms.
		// This whole-CFG proof is independent of the Go/Grace pass driver.
		if mir2.FuseByteAbsDiffCFG(f) {
			tr.FuseAbsDiff++
		}
		if (opt.Backend == "" || opt.Backend == "z80") && opt.Runtime == "" && !opt.CPMApplicationEntry && !opt.ZXApplicationEntry {
			tr.PtrThreading += mir2.RecognizeByteLoops(f)
		}
		n := mir2.StrengthReduceCountedPointersWithOptions(f, mir2.PointerWalkOptions{DisableAffine: opt.disableAffinePointers})
		tr.PtrThreading += n
		affineChanged = affineChanged || n > 0
		if opt.OptSize {
			mir2.MergeConstantReturnTails(f)
		}
		if opt.FuseByteWords {
			mir2.FuseSequentialByteWords(f)
		}
		// Re-count cond_ret after optimization (CondRetSink creates them)
		tr.CondRets = 0
		for _, b := range f.Blocks {
			if _, ok := b.Term.(*mir2.TermCondRet); ok {
				tr.CondRets++
			}
		}
	}
	// Fold conditional calls: BrIf → single-call-block → join → CALL cc.
	for _, f := range m.Funcs {
		mir2.FoldConditionalCalls(f)
	}

	s.MIR2Opt = m.Dump()
	s.MIR2Module = m

	if opt.variantMIRReady != nil && !opt.variantMIRReady(m, opt.OptSize && affineChanged && !opt.disableAffinePointers) {
		return s, nil
	}

	// Structural verification.
	if err := mir2.Verify(m); err != nil {
		return s, fmt.Errorf("MIR2 verify: %w", err)
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
	mir2.PruneUnreachableFuncs(m, functionPruneRoots(hm))
	// Renumber regs to guarantee uniqueness after inlining may have
	// allocated regs in earlier functions that overlap later functions.
	m.RenumberRegs()

	// Phase 5b: interprocedural contracts (greedy default, opt-in joint PFCCO).
	// Run BEFORE LUTGen so that synthetic LUT functions (which have hardcoded
	// class requirements in their Sub instructions) are not re-optimised and
	// given a conflicting class (BUG-004).
	var ct mir2.CostTable
	if opt.Backend == "ez80" {
		ct = ez80.EZ80CostTable{}
	} else {
		ct = mir2.Z80CostTable{}
	}
	if opt.ContractOpt {
		mir2.PrepareByteLoopContracts(m)
		cs := optimizePipelineContracts(m, ct, opt.ContractStrategy)
		mir2.ApplyContracts(m, cs)
	}

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
	s.Allocation = combined

	// MIR2 VM assertion checks (skip "z80"-only asserts).
	if opt.AssertMode != "z80" && opt.AssertMode != "none" {
		if err := RunAssertsMIR2(hm, m); err != nil {
			return s, err
		}
	}

	if !opt.SkipZ80Emission || NeedsZ80Asserts(hm, opt.AssertMode) {
		// Generate the complete production module, including runtime routines.
		for _, f := range m.Funcs {
			if tr := s.Traces[f.Name]; tr != nil {
				tr.Backend = "PBQP"
			}
		}
		if err := mir2.ValidateZ80Symbols(m); err != nil {
			return s, err
		}
		if err := mir2.ValidateZ80IndirectCalls(m); err != nil {
			return s, err
		}
		var codegenErr error
		s.Assembly, codegenErr = mir2.Z80Codegen(m, combined, mir2.Z80CodegenOptions{
			OptimizeForSize:           opt.OptSize,
			CalledMain:                opt.Backend == "ez80" || opt.CPMApplicationEntry || opt.Runtime == mir2.Z80RuntimeAgonZ80,
			CPMApplicationEntry:       opt.CPMApplicationEntry,
			ZXApplicationEntry:        opt.ZXApplicationEntry,
			Runtime:                   opt.Runtime,
			RuntimeStackBytes:         opt.RuntimeStackBytes,
			AnnotateTStates:           opt.AnnotateTStates,
			UnsignedPairComparePolicy: s.UnsignedPairComparePolicy,
		})
		if codegenErr != nil {
			return s, codegenErr
		}

		if opt.CheckedLocalTablePath != "" {
			s.Assembly, err = applyCheckedLocalTables(s.Assembly, m, combined, opt, s.Traces)
			if err != nil {
				return s, err
			}
		}

		// Emit stubs for @extern functions not already defined in the assembly.
		// Ensure CALL targets for extern functions resolve.
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

		// Deduplicate labels in assembly.
		s.Assembly = dedupAsmLabels(s.Assembly)

		// eZ80: wrap Z80 assembly with ADL header.
		if opt.Backend == "ez80" {
			s.Assembly = "; eZ80 ADL mode assembly — generated by MinZ compiler\n" +
				"; Target: Agon Light 2 (eZ80 @ 18.432 MHz)\n\n" +
				"    .ASSUME ADL=1\n" +
				"    ORG $040045\n\n" +
				"; MOS entry preserves IY (firmware frame) and returns status in HL.\n" +
				"__agon_entry:\n    PUSH IY\n    CALL main\n    POP IY\n    LD HL, 0\n    RET\n\n" + s.Assembly
		}

		// Z80 binary assertion checks (skip for eZ80 — different encoding).
		if opt.Backend != "ez80" && opt.AssertMode != "mir2" && opt.AssertMode != "none" && opt.AssertMode != "wasm" {
			if err := RunAssertsZ80(hm, m, combined, s.Assembly); err != nil {
				return s, err
			}
		}
	}

	// WASM assertion checks — only when explicit "via wasm" or --asserts-force wasm.
	if opt.AssertMode == "wasm" {
		if err := mir2wasm.RunAsserts(hm, m, true); err != nil {
			return s, err
		}
	}

	// LLVM assertion checks — only when explicit "via llvm" or --asserts-force llvm.
	if opt.AssertMode == "llvm" {
		if err := mir2llvm.RunAsserts(hm, m, true); err != nil {
			return s, err
		}
	}

	if opt.OptSize && !opt.disableAffinePointers && affineChanged && !opt.SkipZ80Emission && (opt.Backend == "" || opt.Backend == "z80") {
		baselineOpt := opt
		baselineOpt.disableAffinePointers = true
		baselineOpt.GraceStats = nil
		// This is the native size gate's mandatory comparison, not a search trial.
		// Refusing it at a cached MIR checkpoint would yield an empty product.
		baselineOpt.variantMIRReady = nil
		baselineModule := hir.LowerModuleWithOptions(hm, hir.LowerOptions{DisableCountedLoops: opt.DisableCountedLoops, CountedLoopPolicy: variantCountedPolicies(opt)})
		baseline, err := compileMIRSteps(hm, baselineOpt, initialSteps, splitInfo, baselineModule)
		if err != nil {
			return s, err
		}
		candidateBytes, candidateErrors := Assemble(s.Assembly, "z80")
		baselineBytes, baselineErrors := Assemble(baseline.Assembly, "z80")
		// Unknown sizes cannot establish profitability. In particular, do not
		// retain a candidate just because an external symbol prevents assembly.
		if len(candidateErrors) != 0 || len(baselineErrors) != 0 || len(candidateBytes) > len(baselineBytes) {
			return baseline, nil
		}
	}
	return s, nil
}

// NeedsZ80Asserts reports whether the selected assertions need a Z80 product.
func NeedsZ80Asserts(hm *hir.Module, mode string) bool {
	if mode == "mir2" || mode == "none" || mode == "wasm" || mode == "llvm" {
		return false
	}
	for _, a := range hm.Asserts {
		if a.Via != "mir2" {
			return true
		}
	}
	for _, sb := range hm.Sandboxes {
		for _, a := range sb.Asserts {
			if a.Via != "mir2" {
				return true
			}
		}
	}
	return false
}

// CompileHIR runs the full HIR→MIR2→Z80 pipeline with default options.
// It does NOT assemble to binary; call Assemble for that.
func CompileHIR(hm *hir.Module) (string, error) {
	return CompileHIRWithOptions(hm, DefaultOptions())
}

// CompileHIRWithOptions runs the HIR→MIR2→Z80 pipeline with explicit options.
// Use this to compare output with/without specific optimisation passes.
func CompileHIRWithOptions(hm *hir.Module, opts Options) (string, error) {
	if opts.CheckedLocalTableSHA256 != "" && opts.CheckedLocalTablePath == "" {
		return "", fmt.Errorf("checked local table: SHA256 requires a path")
	}
	if err := hir.ResolveErrorABI(hm); err != nil {
		return "", err
	}
	if opts.Backend != "" && opts.Backend != "z80" {
		for _, f := range hm.Funcs {
			if f.FallibleCY {
				return "", fmt.Errorf("CY error ABI requires Z80 backend")
			}
		}
	}
	if err := ValidateContractStrategy(opts); err != nil {
		return "", err
	}
	// MIR Grace has one implementation and one option/size policy in the steps
	// pipeline. Do not silently run the legacy passes for an explicit request.
	if err := mir2.ValidateRuntimeStack(opts.Runtime, opts.RuntimeStackBytes); err != nil {
		return "", err
	}
	if opts.OptSize || variantSearchEnabled(hm, []Options{opts}) || len(opts.FunctionVariants) > 0 || opts.UseGrace || opts.Runtime != "" || opts.CPMApplicationEntry || opts.ZXApplicationEntry || needsZXApplication(hm) || opts.ProfileSelection != nil {
		steps, err := CompileHIRSteps(hm, opts)
		return steps.Assembly, err
	}
	policy, err := resolveUnsignedPairComparePolicy(opts)
	if err != nil {
		return "", err
	}
	if err := hir.ResolveCallSignatures(hm); err != nil {
		return "", err
	}
	if opts.ProfileSelection != nil {
		if _, err := applyPipelineProfile(hm, opts); err != nil {
			return "", err
		}
	}
	if opts.UseHIRGrace {
		if _, err := hir.RunGraceWithOptions(hm, hir.GraceOptions{OptimizeForSize: opts.OptSize}); err != nil {
			return "", err
		}
	}
	// Great Autorefactor: split high-pressure functions.
	hir.SplitHighPressure(hm)

	if err := hir.ResolveCallSignatures(hm); err != nil {
		return "", err
	}
	// Lower HIR → MIR2.
	m := hir.LowerModuleWithOptions(hm, hir.LowerOptions{DisableCountedLoops: opts.DisableCountedLoops})

	// Per-function optimisation passes.
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for iter := 0; iter < 100; iter++ {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			si := mir2.SimplifyIdentities(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !si && !e {
				break
			}
		}
		mir2.NarrowLowByteMaskTests(f)
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
		mir2.HoistDiamondCommon(f)
		mir2.PrepareShiftCarry(f)
		mir2.DeadBlockArgElim(f)
		mir2.FuseAbsDiff(f)
		mir2.FuseByteAbsDiffCFG(f)
		if (opts.Backend == "" || opts.Backend == "z80") && opts.Runtime == "" && !opts.CPMApplicationEntry && !opts.ZXApplicationEntry {
			mir2.RecognizeByteLoops(f)
		}
		mir2.StrengthReduceCountedPointers(f)
		if opts.FuseByteWords {
			mir2.FuseSequentialByteWords(f)
		}
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
	mir2.PruneUnreachableFuncs(m, functionPruneRoots(hm))
	m.RenumberRegs()

	// Phase 5b: interprocedural contracts (greedy default, opt-in joint PFCCO).
	// Run BEFORE LUTGen so synthetic LUT functions keep their original param class
	// and are not re-assigned a conflicting one (BUG-004).
	if opts.ContractOpt {
		mir2.PrepareByteLoopContracts(m)
		cs := optimizePipelineContracts(m, ct, opts.ContractStrategy)
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

	// Insert explicit A-register saves before destructive ops.
	for _, f := range m.Funcs {
		if err := mir2.InsertAccSaves(f, combined); err != nil {
			return "", err
		}
	}

	// Z80 assembly text.
	if err := mir2.ValidateZ80IndirectCalls(m); err != nil {
		return "", err
	}
	if err := mir2.ValidateZ80Symbols(m); err != nil {
		return "", err
	}
	asm, err := mir2.Z80Codegen(m, combined, mir2.Z80CodegenOptions{UnsignedPairComparePolicy: policy, OptimizeForSize: opts.OptSize})
	if err != nil {
		return "", err
	}

	if opts.CheckedLocalTablePath != "" {
		if !opts.OptSize || opts.Backend != "" && opts.Backend != "z80" || opts.ProfileSelection != nil && opts.CheckedLocalTableSHA256 == "" {
			return "", fmt.Errorf("checked local tables require Z80 OptSize and a content pin for PGO")
		}
		asm, err = applyCheckedLocalTables(asm, m, combined, opts, nil)
		if err != nil {
			return "", err
		}
	}

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
		if err := hm.RecordAssert(runOneAssertMIR2(vm, a)); err != nil {
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
			if err := hm.RecordAssert(runOneAssertMIR2(vm, a)); err != nil {
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
	if vm == nil || vm.Module == nil {
		return fmt.Errorf("line %d: assert %q [mir2]: missing module", a.Line, a.Source)
	}
	fn := vm.Module.FuncByName(a.FuncName)
	if fn == nil {
		return fmt.Errorf("line %d: assert %q [mir2]: function %q not found", a.Line, a.Source, a.FuncName)
	}
	wants := a.ExpectedMulti
	if len(wants) == 0 {
		wants = []int64{a.Expected}
	}
	returns := fn.Contract.Returns
	if len(returns) < len(wants) {
		return fmt.Errorf("line %d: assert %q [mir2]: return contract has %d values, want %d", a.Line, a.Source, len(returns), len(wants))
	}
	for i := range wants {
		if returns[i].Ty == nil {
			return fmt.Errorf("line %d: assert %q [mir2]: return[%d] has no type", a.Line, a.Source, i)
		}
	}
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
			if !hir.AssertValueMatches(rets[i].I, want, returns[i].Ty) {
				return fmt.Errorf("line %d: assert %q [mir2]: return[%d] got %s, want %d",
					a.Line, a.Source, i, hir.FormatAssertValue(rets[i].I, returns[i].Ty), want)
			}
		}
	} else {
		if !hir.AssertValueMatches(rets[0].I, a.Expected, returns[0].Ty) {
			return fmt.Errorf("line %d: assert %q [mir2]: got %s, want %d",
				a.Line, a.Source, hir.FormatAssertValue(rets[0].I, returns[0].Ty), a.Expected)
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
		if err := hm.RecordAssert(runOneAssertZ80(z, a, mir2Funcs, hirFuncs, ar, asmSrc)); err != nil {
			return err
		}
	}

	// Sandbox blocks: one shared emulator per sandbox.
	// First assert loads program code; subsequent asserts only overwrite the
	// trampoline region — globals in Z80 memory persist between calls.
	for _, sb := range hm.Sandboxes {
		z := emulator.NewRemogattoZ80()
		first := true
		trampolineBytes := 64
		for _, a := range sb.Asserts {
			if mf := mir2Funcs[a.FuncName]; mf != nil {
				trampolineBytes = max(trampolineBytes, trampolineSize(a, mf, ar))
			}
		}
		for _, a := range sb.Asserts {
			if a.Via == "mir2" {
				continue
			}
			if err := hm.RecordAssert(runOneAssertZ80Sandbox(z, a, mir2Funcs, hirFuncs, ar, asmSrc, trampolineBytes, first)); err != nil {
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
		if os.Getenv("ASSERT_DEBUG_ASM") != "" {
			fmt.Fprintf(os.Stderr, "[ASSERT-ASM] %s:\n%s\n", a.FuncName, src)
		}
		return fmt.Errorf("line %d: assert %q [z80]: assemble: %w", a.Line, a.Source, err)
	}
	if len(res.Errors) > 0 {
		if os.Getenv("ASSERT_DEBUG_ASM") != "" {
			fmt.Fprintf(os.Stderr, "[ASSERT-ASM] %s:\n%s\n", a.FuncName, src)
		}
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

// RunZ80Cycles executes one call of fnName with literal arguments on a fresh
// emulator, exactly as an assertion would, and reports the T-states spent
// plus the single return value. It exists for measurements: correctness
// verdicts belong to RunAssertsZ80.
func RunZ80Cycles(hm *hir.Module, m *mir2.Module, ar *mir2.AllocResult, asmSrc, fnName string, args []int64) (cycles int64, ret int64, err error) {
	mir2Funcs := make(map[string]*mir2.Func, len(m.Funcs))
	for _, f := range m.Funcs {
		mir2Funcs[f.Name] = f
	}
	hirFuncs := make(map[string]*hir.Func, len(hm.Funcs))
	for _, f := range hm.Funcs {
		hirFuncs[f.Name] = f
	}
	mf := mir2Funcs[fnName]
	if mf == nil {
		return 0, 0, fmt.Errorf("z80 cycles: function %q not found in MIR2", fnName)
	}
	a := hir.Assert{FuncName: fnName, Args: args, Source: "z80 cycles"}
	z := emulator.NewRemogattoZ80()
	if err := runOneAssertZ80Raw(z, a, mf, hirFuncs, ar, asmSrc); err != nil {
		return 0, 0, err
	}
	if len(mf.Contract.Returns) == 0 {
		return int64(z.GetCycles()), 0, nil
	}
	value, err := readAssertZ80Return(z, mf.Contract.Returns[0])
	if err != nil {
		return 0, 0, err
	}
	return int64(z.GetCycles()), value, nil
}

// runOneAssertZ80Raw is runOneAssertZ80 without the result check.
func runOneAssertZ80Raw(z *emulator.RemogattoZ80, a hir.Assert, mf *mir2.Func,
	hirFuncs map[string]*hir.Func, ar *mir2.AllocResult, asmSrc string) error {
	boot := buildAssertBootstrap(assertLoadAddr, a, mf, ar)
	src := boot + "\n" + asmSrc
	as := z80asm.NewAssembler()
	res, err := as.AssembleString(src)
	if err != nil {
		return fmt.Errorf("assert %q [z80]: assemble: %w", a.Source, err)
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("assert %q [z80]: assemble errors: %v", a.Source, res.Errors[0])
	}
	if lerr := z.LoadMemory(assertLoadAddr, res.Binary); lerr != nil {
		return fmt.Errorf("assert %q [z80]: load: %w", a.Source, lerr)
	}
	z.SetPC(assertLoadAddr)
	if rerr := z.Run(); rerr != nil {
		return fmt.Errorf("assert %q [z80]: run: %w", a.Source, rerr)
	}
	return nil
}

// runOneAssertZ80Sandbox runs a single assert on a shared emulator (sandbox).
// Uses a NOP-padded trampoline sized for every assert in the sandbox so the
// code section (and globals) always lives at the same addresses.
// On subsequent calls, only overwrites the trampoline region — globals persist.
func runOneAssertZ80Sandbox(z *emulator.RemogattoZ80, a hir.Assert,
	mir2Funcs map[string]*mir2.Func, hirFuncs map[string]*hir.Func,
	ar *mir2.AllocResult, asmSrc string, trampolineBytes int, first bool) error {

	mf := mir2Funcs[a.FuncName]
	if mf == nil {
		return fmt.Errorf("line %d: assert %q [z80]: function %q not found in MIR2", a.Line, a.Source, a.FuncName)
	}

	// Build a fixed-size trampoline: bootstrap + NOP padding to trampolineBytes.
	boot := buildAssertBootstrap(assertLoadAddr, a, mf, ar)
	padCount := trampolineBytes - trampolineSize(a, mf, ar)
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
		for i := 0; i < trampolineBytes && i < len(res.Binary); i++ {
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
	// Hidden result storage is reserved below this invocation's initial SP;
	// source assertion arguments and their fingerprints remain unchanged.
	args := append([]int64(nil), a.Args...)
	bufferTop := int64(0xFF00)
	for _, slot := range mf.Contract.StructReturnBuffers {
		bufferTop -= int64(max(1, mir2.ByteWidth(slot.Ty)))
		args = append(args, bufferTop)
	}
	if len(mf.Contract.StructReturnBuffers) > 0 {
		fmt.Fprintf(&boot, "    LD SP, %d\n", bufferTop)
	}
	// Initialize spill slots before registers: the byte stores use A, which may
	// itself hold another argument at the call boundary.
	for i, arg := range args {
		if i >= len(mf.Contract.Params) {
			break
		}
		param := mf.Contract.Params[i]
		loc := assertParamLocation(param, ar)
		if loc.Kind != mir2.LocMem {
			continue
		}
		label := mir2.Z80SpillLabel(mf.Name, param.Reg)
		for b := range mir2.NativeSpillWidth(param.Ty) {
			suffix := ""
			if b > 0 {
				suffix = fmt.Sprintf("+%d", b)
			}
			fmt.Fprintf(&boot, "    LD A, %d\n    LD (%s%s), A\n", (uint64(arg)>>(8*b))&0xff, label, suffix)
		}
	}
	for i, arg := range args {
		if i >= len(mf.Contract.Params) {
			break
		}
		param := mf.Contract.Params[i]
		loc := assertParamLocation(param, ar)
		if loc.Kind == mir2.LocMem || loc.Name == "" {
			continue
		}
		fmt.Fprintf(&boot, "    LD %s, %d\n", loc.Name, arg)
	}
	fmt.Fprintf(&boot, "    CALL %s\n", asmutil.SanitizeZ80Label(a.FuncName))
	boot.WriteString("    DI\n    HALT\n")
	return boot.String()
}

// assertParamLocation uses the production allocation when available. The
// contract class is the fallback for backends that do not publish locations.
func assertParamLocation(param mir2.Param, ar *mir2.AllocResult) mir2.PhysLoc {
	if ar != nil {
		if loc, ok := ar.Locs[param.Reg]; ok {
			return loc
		}
	}
	name := ""
	switch param.Class {
	case mir2.ClassAcc:
		name = "A"
	case mir2.ClassGeneral, mir2.ClassCounter:
		name = "B"
	case mir2.ClassRegC:
		name = "C"
	case mir2.ClassRegD:
		name = "D"
	case mir2.ClassRegE:
		name = "E"
	case mir2.ClassRegH:
		name = "H"
	case mir2.ClassRegL:
		name = "L"
	case mir2.ClassPointer:
		name = "HL"
	case mir2.ClassIndex:
		name = "DE"
	case mir2.ClassPair:
		name = "BC"
	}
	return mir2.PhysLoc{Kind: mir2.LocReg, Name: name}
}

// trampolineSize counts the emitted instruction bytes exactly, so sandbox
// trampolines keep their shared code and data at a stable address.
func trampolineSize(a hir.Assert, mf *mir2.Func, ar *mir2.AllocResult) int {
	size := 8 // LD SP,nn + CALL nn + DI + HALT
	for i := range a.Args {
		if i >= len(mf.Contract.Params) {
			break
		}
		param := mf.Contract.Params[i]
		loc := assertParamLocation(param, ar)
		if loc.Kind == mir2.LocMem {
			size += 5 * max(1, (param.Ty.Width()+7)/8) // LD A,n + LD (nn),A per byte
			continue
		}
		switch loc.Name {
		case "HL", "DE", "BC", "SP":
			size += 3
		case "IX", "IY":
			size += 4
		case "IXH", "IXL", "IYH", "IYL":
			size += 3
		case "":
			// No instruction emitted.
		default:
			size += 2
		}
	}
	return size
}

// checkAssertZ80Result reads the result register and compares against expected.
func checkAssertZ80Result(z *emulator.RemogattoZ80, a hir.Assert,
	mf *mir2.Func, hirFuncs map[string]*hir.Func) error {

	if len(a.ExpectedMulti) > 0 {
		if len(mf.Contract.Returns) < len(a.ExpectedMulti) {
			return fmt.Errorf("line %d: assert %q [z80]: function returned %d values, want %d",
				a.Line, a.Source, len(mf.Contract.Returns), len(a.ExpectedMulti))
		}
		for i, want := range a.ExpectedMulti {
			ret := mf.Contract.Returns[i]
			value, err := readAssertZ80Return(z, ret)
			if err != nil {
				return err
			}

			if !hir.AssertValueMatches(value, want, ret.Ty) {
				return fmt.Errorf("line %d: assert %q [z80]: return[%d] got %s, want %d",
					a.Line, a.Source, i, hir.FormatAssertValue(value, ret.Ty), want)
			}
		}
	} else {
		ret := mir2.Return{Ty: mir2.TyU8, Class: mir2.ClassAcc}
		if len(mf.Contract.Returns) > 0 {
			ret = mf.Contract.Returns[0]
		} else if hf := hirFuncs[a.FuncName]; hf != nil && hf.RetTy != mir2.TyVoid {
			ret.Ty = hf.RetTy
			if hf.RetTy.Width() > 8 {
				ret.Class = mir2.ClassPointer
			}
		}
		got, err := readAssertZ80Return(z, ret)
		if err != nil {
			return err
		}
		if !hir.AssertValueMatches(got, a.Expected, ret.Ty) {
			return fmt.Errorf("line %d: assert %q [z80]: got %s, want %d",
				a.Line, a.Source, hir.FormatAssertValue(got, ret.Ty), a.Expected)
		}
	}
	return nil
}

// Native MIR values use a bit-pattern representation, like VM.Value. Wide
// pair returns occupy the corresponding main and alternate pairs.
func readAssertZ80Return(z *emulator.RemogattoZ80, ret mir2.Return) (int64, error) {
	regs := z.GetRegisters()
	location := mir2.ReturnLocation(ret.Class, ret.Ty)
	var value uint64
	switch location {
	case "A":
		value = uint64(regs.A)
	case "B":
		value = uint64(regs.BC >> 8)
	case "C":
		value = uint64(regs.BC & 255)
	case "D":
		value = uint64(regs.DE >> 8)
	case "E":
		value = uint64(regs.DE & 255)
	case "H":
		value = uint64(regs.HL >> 8)
	case "L":
		value = uint64(regs.HL & 255)
	case "HL":
		value = uint64(regs.HL)
	case "DE":
		value = uint64(regs.DE)
	case "BC":
		value = uint64(regs.BC)
	case "IX":
		value = uint64(regs.IX)
	case "IY":
		value = uint64(regs.IY)
	case "F":
		truth := false
		switch ret.FlagCond {
		case mir2.CmpEq:
			truth = regs.F&0x40 != 0
		case mir2.CmpNe:
			truth = regs.F&0x40 == 0
		case mir2.CmpLt:
			truth = regs.F&1 != 0
		case mir2.CmpGe:
			truth = regs.F&1 == 0
		default:
			return 0, fmt.Errorf("unsupported return flag condition %v", ret.FlagCond)
		}
		if truth {
			value = 1
		}
	default:
		return 0, fmt.Errorf("unsupported return location %s", location)
	}
	width := ret.Ty.Width()
	if width > 16 {
		shadow := z.GetShadowRegisters()
		switch location {
		case "HL":
			value |= uint64(shadow.HL) << 16
		case "DE":
			value |= uint64(shadow.DE) << 16
		case "BC":
			value |= uint64(shadow.BC) << 16
		default:
			return 0, fmt.Errorf("unsupported wide return location %s", location)
		}
	}
	if width == 24 {
		value &= 0xffffff // The ABI returns three bytes, not the fourth shadow-register byte.
	}
	return int64(value), nil
}

// Assemble assembles .a80 text to a binary using MZA.
// target: "cpm", "zxspectrum", "generic" (default).
func Assemble(asmSrc string, target string) ([]byte, []error) {
	if dumpPath := os.Getenv("MINZ_DUMP_FINAL_ASM"); dumpPath != "" {
		_ = os.WriteFile(dumpPath, []byte(asmSrc), 0644)
	}
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
		// EQU creates a symbol without a colon, including function aliases.
		fields := strings.Fields(strings.SplitN(trimmed, ";", 2)[0])
		if len(fields) >= 3 && strings.EqualFold(fields[1], "EQU") && isLabel(fields[0]) {
			defined[fields[0]] = true
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
// in the assembly but have no label definition after function emission.
func emitExternStubs(asm string, m *mir2.Module) string {
	var stubs strings.Builder
	for _, f := range m.Funcs {
		if len(f.Blocks) > 0 {
			continue // not extern — has a body
		}
		label := asmutil.SanitizeZ80Label(f.Name)
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
	counts := map[string]int{} // backend → count
	splits, fallbacks, emitted := 0, 0, 0
	totalPasses := 0
	var fallbackNames, prunedNames []string
	for _, tr := range traces {
		counts[tr.Backend]++
		if tr.Backend != "" {
			emitted++
		} else {
			prunedNames = append(prunedNames, tr.Name)
		}
		if tr.SplitFrom != "" {
			splits++
		}
		if tr.BackendErr != "" {
			fallbacks++
			fallbackNames = append(fallbackNames, tr.Name)
		}
		totalPasses += tr.ConstProp + tr.ConstFold + tr.IdentSimp + tr.CallElim +
			tr.DSE + tr.DeadBlockArg + tr.BranchEquiv + tr.SplitJoinRet +
			tr.CondRetSink + tr.FuseAbsDiff + tr.PtrThreading + tr.PtrAddCSE
	}
	sb.WriteString("; ── compilation summary ──────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("; functions: %d", total))
	for _, be := range []string{"VIR", "PBQP", "checked-local-table", "VIR+PBQP-fallback"} {
		if n := counts[be]; n > 0 {
			sb.WriteString(fmt.Sprintf("  %s=%d", be, n))
		}
	}
	sb.WriteByte('\n')
	if pruned := total - emitted; pruned > 0 {
		sort.Strings(prunedNames)
		sb.WriteString(fmt.Sprintf("; pruned functions: %d [%s]\n", pruned, strings.Join(prunedNames, ", ")))
	}
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

func mergeGraceStats(dst, src *mir2.GraceStats) {
	if dst == nil || src == nil {
		return
	}
	dst.Funcs += src.Funcs
	dst.Matches += src.Matches
	dst.Total += src.Total
	for name, count := range src.ByRule {
		dst.ByRule[name] += count
	}
}

func applyGraceStatsToTrace(tr *FuncTrace, stats *mir2.GraceStats) {
	if tr == nil || stats == nil {
		return
	}
	for name, count := range stats.ByRule {
		switch name {
		case "dead-store-elim":
			tr.DSE += count
		case "dead-block-arg":
			tr.DeadBlockArg += count
		case "split-join-ret":
			tr.SplitJoinRet += count
		case "cond-ret-sink":
			tr.CondRetSink += count
		case "fuse-abs-diff":
			tr.FuseAbsDiff += count
		case "ptr-threading":
			tr.PtrThreading += count
		case "ptr-add-cse":
			tr.PtrAddCSE += count
		}
	}
}

// formatTrace formats a FuncTrace as a single ASM comment line.
func formatTrace(tr *FuncTrace) string {
	var parts []string
	parts = append(parts, "backend="+tr.Backend)
	if tr.Variant != "" {
		parts = append(parts, "variant="+tr.Variant)
	}
	if tr.SplitFrom != "" {
		parts = append(parts, fmt.Sprintf("split-from=%s(pressure=%d)", tr.SplitFrom, tr.SplitPressure))
	}
	if tr.BackendErr != "" {
		parts = append(parts, "fallback-reason="+tr.BackendErr)
	}

	// Optimization passes — only show non-zero
	var passes []string
	if tr.ConstProp > 0 {
		passes = append(passes, fmt.Sprintf("const-prop=%d", tr.ConstProp))
	}
	if tr.ConstFold > 0 {
		passes = append(passes, fmt.Sprintf("const-fold=%d", tr.ConstFold))
	}
	if tr.IdentSimp > 0 {
		passes = append(passes, fmt.Sprintf("ident-simp=%d", tr.IdentSimp))
	}
	if tr.CallElim > 0 {
		passes = append(passes, fmt.Sprintf("call-elim=%d", tr.CallElim))
	}
	if tr.DSE > 0 {
		passes = append(passes, fmt.Sprintf("dse=%d", tr.DSE))
	}
	if tr.DeadBlockArg > 0 {
		passes = append(passes, fmt.Sprintf("dead-block-arg=%d", tr.DeadBlockArg))
	}
	if tr.BranchEquiv > 0 {
		passes = append(passes, fmt.Sprintf("branch-equiv=%d", tr.BranchEquiv))
	}
	if tr.SplitJoinRet > 0 {
		passes = append(passes, fmt.Sprintf("split-join-ret=%d", tr.SplitJoinRet))
	}
	if tr.CondRetSink > 0 {
		passes = append(passes, fmt.Sprintf("condret-sink=%d", tr.CondRetSink))
	}
	if tr.FuseAbsDiff > 0 {
		passes = append(passes, fmt.Sprintf("fuse-abs-diff=%d", tr.FuseAbsDiff))
	}
	if tr.PtrThreading > 0 {
		passes = append(passes, fmt.Sprintf("ptr-threading=%d", tr.PtrThreading))
	}
	if tr.PtrAddCSE > 0 {
		passes = append(passes, fmt.Sprintf("ptr-add-cse=%d", tr.PtrAddCSE))
	}
	if tr.CondRets > 0 {
		passes = append(passes, fmt.Sprintf("cond-rets=%d", tr.CondRets))
	}
	if tr.Inlined {
		passes = append(passes, "inlined")
	}
	if tr.LUTReplaced {
		passes = append(passes, "lut-replaced")
	}

	if len(passes) > 0 {
		parts = append(parts, "passes=["+strings.Join(passes, ",")+"]")
	}
	if len(tr.LabelWarnings) > 0 {
		parts = append(parts, fmt.Sprintf("label-warnings=%d", len(tr.LabelWarnings)))
	}
	return "; [trace] " + strings.Join(parts, " ")
}

// dedupAsmLabels removes duplicate label definitions from assembly text.
// When combined function output defines a label twice, keep the first definition.
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

// splitVIRZeroStorage separates zero-initialized storage (_vir_memN, _spill_*)
// from VIR asm output so the caller can place them after strings.
// Returns (code, zeroStorage).
func splitVIRZeroStorage(virAsm string) (string, string) {
	var code, zeros strings.Builder
	for _, line := range strings.Split(virAsm, "\n") {
		trimmed := strings.TrimSpace(line)
		if (strings.HasPrefix(trimmed, "_vir_mem") || strings.HasPrefix(trimmed, "_spill_")) &&
			strings.Contains(trimmed, "DW 0") {
			zeros.WriteString(line + "\n")
		} else {
			code.WriteString(line + "\n")
		}
	}
	return code.String(), zeros.String()
}

func functionPruneRoots(hm *hir.Module) map[string]bool {
	roots := make(map[string]bool)

	if hm.FuncByName("main") != nil {
		roots["main"] = true
	}

	for _, f := range hm.Funcs {
		// Root explicit user-declared functions from the current module.
		// Imported module functions and compiler-generated helpers are kept only
		// when reachable from these roots, asserts, or address-taken edges.
		if !f.Internal && isUserFacingPruneRoot(f.Name) {
			roots[f.Name] = true
		}
	}

	for _, a := range hm.Asserts {
		roots[a.FuncName] = true
	}
	for _, sb := range hm.Sandboxes {
		for _, a := range sb.Asserts {
			roots[a.FuncName] = true
		}
	}

	return roots
}

func isUserFacingPruneRoot(name string) bool {
	if name == "" {
		return false
	}
	if isCompilerGeneratedFunc(name) {
		return false
	}
	if isImportedModuleFunc(name) {
		return false
	}
	return true
}

func isCompilerGeneratedFunc(name string) bool {
	return name == "__tag" ||
		name == "__payload" ||
		strings.HasPrefix(name, "__mpay_") ||
		strings.HasPrefix(name, "__arr_") ||
		strings.HasPrefix(name, "lambda_")
}

func isImportedModuleFunc(name string) bool {
	// Nanz import mangling uses "__" between module path segments and before
	// the imported symbol: "lib.math.add" -> "lib__math__add".
	return !strings.HasPrefix(name, "__") && strings.Count(name, "__") >= 2
}

// Generated ZX frontend storage marks an ordinary native application even if
// reachability pruning removed file operations (for example SALV-only code).
func needsZXApplication(hm *hir.Module) bool {
	if hm.Target != hir.TargetZXSpectrum {
		return false
	}
	for _, g := range hm.Globals {
		if g.Name == "_abap_zx_sector_buffer" {
			return true
		}
	}
	return false
}
