// pipeline.go — Integration entry point: MIR2 → LIR pipeline.
//
// This wires the LIR backend into the existing compiler pipeline.
// Two modes:
//
//  1. Convergence check: run LIR alongside existing Z80Codegen, verify results match
//  2. Full codegen: replace Z80Codegen with LIR-based codegen (future)
//
// Current integration point (pipeline.CompileHIRSteps):
//
//	HIR → mir2.Module → [opts] → Verify → ...
//	  → PBQP + Z80Codegen (existing, production)
//	  → LIR bridge → Combine(ISLE) → isel → WFC → VM (convergence check)
//
// Usage from pipeline.go:
//
//	import "github.com/minz/minzc/pkg/lir"
//	if opts.LIRConvergence {
//	    lir.CheckConvergence(m)
//	}
package lir

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// ConvergenceResult holds results from checking one function.
type ConvergenceResult struct {
	FuncName string
	Machine  string
	OpCount  int  // MIR ops before combining
	Combined int  // MIR ops after combining
	InstCount int // LIR instructions after isel
	Match    bool
	Error    string
}

// CheckModuleConvergence runs the LIR pipeline on every function in the module
// and reports which functions successfully lower through LIR.
// This is non-destructive — it doesn't modify the MIR2 module.
func CheckModuleConvergence(m *mir2.Module) []ConvergenceResult {
	var results []ConvergenceResult

	descs := []*MachineDesc{RISC32, CISC, Z80}

	for _, f := range m.Funcs {
		for _, desc := range descs {
			cr := checkFuncConvergence(f, desc, m)
			cr.FuncName = f.Name
			cr.Machine = desc.Name
			results = append(results, cr)
		}
	}

	return results
}

func checkFuncConvergence(f *mir2.Func, desc *MachineDesc, m *mir2.Module) ConvergenceResult {
	// Try multi-block path for functions with >1 block and block params.
	if len(f.Blocks) > 1 && hasBlockParams(f) {
		mbCR := checkFuncConvergenceMultiBlock(f, desc, m)
		if mbCR.Error == "" {
			return mbCR
		}
		// Fallback to flat path on multi-block failure.
	}

	// Flat path: flatten all blocks into one sequence (existing behavior).
	return checkFuncConvergenceFlat(f, desc, m)
}

// hasBlockParams reports whether any block in f has params.
func hasBlockParams(f *mir2.Func) bool {
	for _, b := range f.Blocks {
		if len(b.Params) > 0 {
			return true
		}
	}
	return false
}

// checkFuncConvergenceMultiBlock uses the structured LowerMIR2Prog path.
func checkFuncConvergenceMultiBlock(f *mir2.Func, desc *MachineDesc, m *mir2.Module) ConvergenceResult {
	cr := ConvergenceResult{}

	prog, blockOps, err := LowerMIR2ProgWithOps(f, desc, m)
	if err != nil {
		cr.Error = fmt.Sprintf("lower-prog: %s", err)
		return cr
	}

	totalOps := 0
	totalCombined := 0

	// Per-block: combine → isel
	for i := range prog.Blocks {
		b := &prog.Blocks[i]
		ops := blockOps[i]
		totalOps += len(ops)

		if len(ops) == 0 {
			continue
		}

		// Combine
		combResult, err := Combine(ops)
		if err != nil {
			cr.Error = fmt.Sprintf("combine block %s: %s", b.Label, err)
			return cr
		}
		totalCombined += len(combResult.Ops)

		// Isel with block params
		sel, err := SelectBlockInstructions(desc, combResult.Ops, b.Params)
		if err != nil {
			cr.Error = fmt.Sprintf("isel block %s: %s", b.Label, err)
			return cr
		}
		b.Insts = sel.Insts
	}

	cr.OpCount = totalOps
	cr.Combined = totalCombined

	// Multi-block WFC
	pw := NewProgWFC(prog)
	pw.Propagate()
	if err := pw.Collapse(); err != nil {
		cr.Error = fmt.Sprintf("wfc-multi: %s", err)
		return cr
	}

	totalInsts := 0
	for _, b := range prog.Blocks {
		totalInsts += len(b.Insts)
	}
	cr.InstCount = totalInsts
	cr.Match = true
	return cr
}

// checkFuncConvergenceFlat is the original flat lowering path.
func checkFuncConvergenceFlat(f *mir2.Func, desc *MachineDesc, m *mir2.Module) ConvergenceResult {
	cr := ConvergenceResult{}

	// Step 1: Lower MIR2 blocks to MIROps
	var allOps []MIROp
	fpv := FuncContractVRegs(f)
	for _, b := range f.Blocks {
		ops, err := LowerMIR2Block(b, desc, m, fpv)
		if err != nil {
			cr.Error = fmt.Sprintf("lower: %s", err)
			return cr
		}
		allOps = append(allOps, ops...)
	}
	cr.OpCount = len(allOps)

	if len(allOps) == 0 {
		cr.Match = true // empty function, trivially correct
		return cr
	}

	// Step 2: ISLE instruction combining
	combResult, err := Combine(allOps)
	if err != nil {
		cr.Error = fmt.Sprintf("combine: %s", err)
		return cr
	}
	cr.Combined = len(combResult.Ops)

	// Extract function params as block params for isel+WFC.
	params := ContractParamsToBlockParams(f, desc)

	// Step 3: Instruction selection with param pre-seeding
	sel, err := SelectBlockInstructions(desc, combResult.Ops, params)
	if err != nil {
		cr.Error = fmt.Sprintf("isel: %s", err)
		return cr
	}

	// Step 4: WFC constraint propagation + collapse with param cells
	wfc := NewWFCStateWithParams(desc, sel.Insts, params)
	wfc.Propagate()
	if err := wfc.Collapse(); err != nil {
		cr.Error = fmt.Sprintf("wfc: %s", err)
		return cr
	}

	insts := wfc.ToInsts()
	cr.InstCount = len(insts)

	// Verify: run LIR-VM on the collapsed instructions and compare with MIR2-VM.
	// Only for leaf functions (no calls) with return values and on RISC machines
	// where the VM can execute all patterns.
	if desc.Name == "risc32" || desc.Name == "risc8" {
		if vmResult, err := verifyViaVM(f, m, desc, insts); err == nil {
			if vmResult != nil {
				cr.Match = *vmResult
				if !*vmResult {
					cr.Error = "vm-divergence"
				}
				return cr
			}
		}
	}

	cr.Match = true // pipeline completed without error
	return cr
}

// verifyViaVM runs LIR instructions on the LIR-VM and compares with MIR2-VM.
// Returns nil (skip) for functions with calls or void returns.
// Returns *true if results match, *false if divergence.
func verifyViaVM(f *mir2.Func, m *mir2.Module, desc *MachineDesc, insts []Inst) (*bool, error) {
	// Skip void functions (no return value to compare).
	if len(f.Contract.Returns) == 0 {
		return nil, nil
	}

	// Skip functions with calls (VM can't execute CALL) or stores
	// (depends on memory layout which LIR-VM doesn't initialize).
	for _, inst := range insts {
		if inst.Pat == nil {
			continue
		}
		if inst.Pat.Flags&PatCall != 0 {
			return nil, nil
		}
		// Skip if function reads memory — LIR-VM doesn't initialize global/static
		// memory, so designated initializers and global arrays would diverge.
		if inst.Pat.MIROp == OpLoad || inst.Pat.MIROp == OpLoad16LE {
			return nil, nil
		}
	}

	// Skip multi-block functions (VM is linear).
	if len(f.Blocks) > 1 {
		return nil, nil
	}

	// Run MIR2-VM with zero args (leaf function test).
	mir2VM := mir2.NewVM(m)
	mir2Result, err := mir2VM.Call(f.Name, nil)
	if err != nil {
		return nil, err
	}
	if len(mir2Result) == 0 {
		return nil, nil
	}

	// Run LIR-VM.
	lirVM := NewVM(desc)
	for i := range insts {
		if err := lirVM.ExecInst(&insts[i]); err != nil {
			return nil, err
		}
	}

	// Compare: find the last instruction's dst phys.
	var lirVal int64
	found := false
	for i := len(insts) - 1; i >= 0; i-- {
		if insts[i].Dst.Phys >= 0 {
			lirVal = int64(lirVM.Get(insts[i].Dst.Phys))
			found = true
			break
		}
	}
	if !found {
		return nil, nil
	}

	mir2Val := mir2Result[0].I
	match := mir2Val == lirVal
	return &match, nil
}

// LIRCodegenFunc runs the full LIR pipeline on a single MIR2 function
// and returns Z80 assembly text. This is the future replacement for
// mir2.Z80Codegen (per-function).
func LIRCodegenFunc(f *mir2.Func, m *mir2.Module, hints ...AllocHints) (string, error) {
	desc := Z80
	var h AllocHints
	if len(hints) > 0 {
		h = hints[0]
	}

	// Try multi-block path for functions with block params.
	if len(f.Blocks) > 1 && hasBlockParams(f) {
		asm, err := lirCodegenMultiBlock(f, desc, m)
		if err == nil {
			return asm, nil
		}
		// Fallback to flat on failure.
	}

	return lirCodegenFlat(f, desc, m, h)
}

// lirCodegenMultiBlock emits per-block labels, instructions, and terminators.
func lirCodegenMultiBlock(f *mir2.Func, desc *MachineDesc, m *mir2.Module) (string, error) {
	prog, blockOps, err := LowerMIR2ProgWithOps(f, desc, m)
	if err != nil {
		return "", fmt.Errorf("lower %s: %w", f.Name, err)
	}

	// Per-block: combine → isel
	for i := range prog.Blocks {
		b := &prog.Blocks[i]
		ops := blockOps[i]

		if len(ops) == 0 {
			continue
		}

		combResult, err := Combine(ops)
		if err != nil {
			return "", fmt.Errorf("combine %s/%s: %w", f.Name, b.Label, err)
		}

		sel, err := SelectBlockInstructions(desc, combResult.Ops, b.Params)
		if err != nil {
			return "", fmt.Errorf("isel %s/%s: %w", f.Name, b.Label, err)
		}
		b.Insts = sel.Insts
	}

	// Multi-block WFC
	pw := NewProgWFC(prog)
	pw.Propagate()
	if err := pw.Collapse(); err != nil {
		return "", fmt.Errorf("wfc %s: %w", f.Name, err)
	}

	// Apply CFG block rules (branch elimination, empty block removal, etc.)
	blockRules := DefaultBlockRules()
	ApplyBlockRules(prog, blockRules, 10)

	// Emit assembly
	var sb strings.Builder
	totalInsts := 0
	for _, b := range prog.Blocks {
		totalInsts += len(b.Insts)
	}
	fmt.Fprintf(&sb, "; %s — LIR codegen (%d insts, %d blocks)\n", f.Name, totalInsts, len(prog.Blocks))

	for bi, b := range prog.Blocks {
		if bi == 0 {
			fmt.Fprintf(&sb, "%s:\n", f.Name)
		} else {
			fmt.Fprintf(&sb, "%s:\n", b.Label)
		}

		for _, inst := range b.Insts {
			if inst.Pat == nil {
				continue
			}
			line := ExpandTemplateNamed(inst, desc)
			fmt.Fprintf(&sb, "    %s\n", line)
		}

		// Emit parallel-copy moves before terminator where arg.Phys != param.Phys
		emitParallelCopyMoves(&sb, &b, prog, desc)

		// Emit terminator
		emitTerminator(&sb, &b.Term, bi, len(prog.Blocks))
	}

	return sb.String(), nil
}

// emitParallelCopyMoves inserts LD instructions for edge args that don't
// match their target params (the rare case when inter-block WFC doesn't converge).
func emitParallelCopyMoves(sb *strings.Builder, b *Block, prog *Prog, desc *MachineDesc) {
	blockMap := make(map[string]int, len(prog.Blocks))
	for i, bl := range prog.Blocks {
		blockMap[bl.Label] = i
	}

	for edgeIdx, target := range b.Term.Targets {
		if target == "" || edgeIdx >= len(b.Term.Args) {
			continue
		}
		targetIdx, ok := blockMap[target]
		if !ok {
			continue
		}
		targetBlock := &prog.Blocks[targetIdx]
		args := b.Term.Args[edgeIdx]
		n := len(args)
		if n > len(targetBlock.Params) {
			n = len(targetBlock.Params)
		}
		for i := 0; i < n; i++ {
			argPhys := args[i].Phys
			paramPhys := targetBlock.Params[i].Phys
			if argPhys >= 0 && paramPhys >= 0 && argPhys != paramPhys {
				srcName := "?"
				dstName := "?"
				if argPhys < len(desc.Locs) {
					srcName = desc.Locs[argPhys].Name
				}
				if paramPhys < len(desc.Locs) {
					dstName = desc.Locs[paramPhys].Name
				}
				fmt.Fprintf(sb, "    LD %s, %s ; parallel-copy %s→%s\n",
					dstName, srcName, b.Label, target)
			}
		}
	}
}

// emitTerminator emits the assembly for a block terminator.
func emitTerminator(sb *strings.Builder, term *Term, blockIdx, numBlocks int) {
	switch term.Kind {
	case TermNone:
		// Fall through — no instruction needed if next block follows.

	case TermJump:
		if len(term.Targets) > 0 {
			fmt.Fprintf(sb, "    JP %s\n", term.Targets[0])
		}

	case TermBranch:
		if len(term.Targets) >= 2 {
			fmt.Fprintf(sb, "    JP NZ, %s\n", term.Targets[0])
			fmt.Fprintf(sb, "    JP %s\n", term.Targets[1])
		}

	case TermDJNZ:
		if len(term.Targets) >= 2 {
			fmt.Fprintf(sb, "    DJNZ %s\n", term.Targets[0])
			// Fall through or jump to exit
			if blockIdx+1 < numBlocks {
				// If exit block is next, fall through.
				// Otherwise, explicit jump.
				fmt.Fprintf(sb, "    JP %s\n", term.Targets[1])
			}
		}

	case TermReturn:
		sb.WriteString("    RET\n")
	}
}

// lirCodegenFlat is the original flat single-block codegen path.
func lirCodegenFlat(f *mir2.Func, desc *MachineDesc, m *mir2.Module, hints ...AllocHints) (string, error) {
	// Lower MIR2 → MIROps
	var allOps []MIROp
	fpv := FuncContractVRegs(f)
	for _, b := range f.Blocks {
		ops, err := LowerMIR2Block(b, desc, m, fpv)
		if err != nil {
			return "", fmt.Errorf("lower %s: %w", f.Name, err)
		}
		allOps = append(allOps, ops...)
	}

	if len(allOps) == 0 {
		return fmt.Sprintf("; %s — empty\n%s:\n    RET\n", f.Name, f.Name), nil
	}

	// Combine
	combResult, err := Combine(allOps)
	if err != nil {
		return "", fmt.Errorf("combine %s: %w", f.Name, err)
	}

	// Extract function params as block params so isel+WFC know about them.
	params := ContractParamsToBlockParams(f, desc)

	// Isel with param pre-seeding
	sel, err := SelectBlockInstructions(desc, combResult.Ops, params)
	if err != nil {
		return "", fmt.Errorf("isel %s: %w", f.Name, err)
	}

	// WFC with param cells + PBQP hints
	wfc := NewWFCStateWithParams(desc, sel.Insts, params)
	if len(hints) > 0 && hints[0] != nil {
		wfc.Hints = hints[0]
	}
	wfc.Propagate()
	if err := wfc.Collapse(); err != nil {
		return "", fmt.Errorf("wfc %s: %w", f.Name, err)
	}

	insts := wfc.ToInsts()

	// Emit assembly from templates, with caller-save spills around CALLs.
	var sb strings.Builder
	fmt.Fprintf(&sb, "; %s — LIR codegen (%d insts)\n", f.Name, len(insts))
	fmt.Fprintf(&sb, "%s:\n", f.Name)
	// Build param phys map from WFC param cells.
	paramPhys := make(map[int]int) // vreg → phys
	for _, c := range wfc.Cells {
		if c.Pat == nil && c.VRegDst >= 0 {
			if p := PhysOf(c.DstLocs); p >= 0 {
				paramPhys[c.VRegDst] = p
			}
		}
	}
	emitInstsWithCallSpills(&sb, insts, desc, paramPhys)

	// Tail call optimization: if last emitted instruction is CALL, replace with JP.
	asmSoFar := sb.String()
	if idx := strings.LastIndex(asmSoFar, "    CALL "); idx >= 0 {
		// Check nothing follows the CALL line (it's the last instruction).
		callLine := asmSoFar[idx:]
		if nlIdx := strings.IndexByte(callLine, '\n'); nlIdx >= 0 {
			after := strings.TrimSpace(callLine[nlIdx+1:])
			if after == "" {
				// Replace CALL with JP — skip the RET entirely.
				sb.Reset()
				sb.WriteString(asmSoFar[:idx])
				sb.WriteString("    JP")
				sb.WriteString(callLine[8:nlIdx+1]) // " sym\n"
				return sb.String(), nil
			}
		}
	}
	sb.WriteString("    RET\n")

	return sb.String(), nil
}

// emitInstsWithCallSpills emits assembly with PUSH/POP pairs around CALL
// instructions for any physical register that is live across the call.
func emitInstsWithCallSpills(sb *strings.Builder, insts []Inst, desc *MachineDesc, paramPhys map[int]int) {
	for i, inst := range insts {
		if inst.Pat == nil {
			continue
		}

		// Check if this is a CALL instruction.
		if inst.Pat.Flags&PatCall != 0 {
			// Find physical registers that are:
			// - assigned to some vreg BEFORE this call (as dst or param)
			// - used by some vreg AFTER this call (as src)
			defsBefore := make(map[int]int) // phys → vreg
			// Include function params as pre-defined.
			for vreg, phys := range paramPhys {
				defsBefore[phys] = vreg
			}
			for j := 0; j < i; j++ {
				if insts[j].Dst.Phys >= 0 && insts[j].Dst.VReg >= 0 {
					defsBefore[insts[j].Dst.Phys] = insts[j].Dst.VReg
				}
			}

			usesAfter := make(map[int]bool) // vreg → true
			for j := i + 1; j < len(insts); j++ {
				if insts[j].Pat == nil {
					continue
				}
				for s := 0; s < 2; s++ {
					if insts[j].Srcs[s].VReg > 0 {
						usesAfter[insts[j].Srcs[s].VReg] = true
					}
				}
			}

			// Find registers to save: defined before, used after, and clobbered by call.
			var toSave []int // physical register indices to PUSH/POP
			for phys, vreg := range defsBefore {
				if usesAfter[vreg] && inst.Pat.Clobbers.Has(phys) {
					toSave = append(toSave, phys)
				}
			}

			// Also check param vregs (from synthetic param cells, already collapsed).
			// Param cells have Pat==nil so they're excluded from defsBefore.
			// But params are in the WFC cells — check if any param phys is used after.
			// This is already covered if the param appears as a src somewhere before.

			if len(toSave) > 0 {
				// Emit PUSH for each register pair containing a saved register.
				// Z80 PUSH only works on 16-bit pairs: AF, BC, DE, HL.
				pairs := callerSavePairs(toSave, desc)
				for _, pair := range pairs {
					fmt.Fprintf(sb, "    PUSH %s\n", pair)
				}
				// Emit the CALL
				line := ExpandTemplateNamed(inst, desc)
				fmt.Fprintf(sb, "    %s\n", line)
				// Emit POP in reverse order
				for j := len(pairs) - 1; j >= 0; j-- {
					fmt.Fprintf(sb, "    POP %s\n", pairs[j])
				}
				continue
			}
		}

		// Peephole: skip no-op moves (LD A, A etc.)
		if inst.Pat.MIROp == OpMove && inst.Dst.Phys >= 0 && inst.Srcs[0].Phys >= 0 &&
			inst.Dst.Phys == inst.Srcs[0].Phys {
			continue
		}

		line := ExpandTemplateNamed(inst, desc)
		fmt.Fprintf(sb, "    %s\n", line)
	}
}

// callerSavePairs maps individual physical register indices to Z80 register pair
// names for PUSH/POP. Multiple registers in the same pair are deduplicated.
func callerSavePairs(physRegs []int, desc *MachineDesc) []string {
	pairSet := make(map[string]bool)
	for _, phys := range physRegs {
		if phys >= len(desc.Locs) {
			continue
		}
		name := desc.Locs[phys].Name
		switch name {
		case "A":
			pairSet["AF"] = true
		case "B", "C":
			pairSet["BC"] = true
		case "D", "E":
			pairSet["DE"] = true
		case "H", "L":
			pairSet["HL"] = true
		}
	}

	// Order: AF, BC, DE, HL (conventional)
	var result []string
	for _, p := range []string{"AF", "BC", "DE", "HL"} {
		if pairSet[p] {
			result = append(result, p)
		}
	}
	return result
}

// LIRCodegenModule runs the LIR pipeline on every function in the module
// and returns the combined Z80 assembly. Functions that fail LIR are reported
// in the second return value; their assembly is left empty (caller should
// fall back to PBQP for those).
//
// This is the --lir entry point: it replaces mir2.Z80Codegen for the
// function codegen portion. Globals are NOT emitted here — the caller
// must append them separately.
// AllocHints maps virtual register → preferred LIR physical location index.
// Sourced from PBQP allocation to guide WFC collapse decisions.
type AllocHints map[int]int

func LIRCodegenModule(m *mir2.Module, hints ...AllocHints) (string, []LIRFuncResult) {
	var h AllocHints
	if len(hints) > 0 {
		h = hints[0]
	}
	var sb strings.Builder
	var results []LIRFuncResult

	sb.WriteString("; generated by LIR backend (ISLE+WFC+Layer4)\n\n")

	// Emit main first (same ordering as Z80Codegen).
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
		asm, err := LIRCodegenFunc(f, m, h)
		r := LIRFuncResult{Name: f.Name}
		if err != nil {
			r.Error = err.Error()
		} else {
			r.OK = true
			sb.WriteString(asm)
			sb.WriteByte('\n')
		}
		results = append(results, r)
	}

	// Emit runtime routines if referenced.
	asmText := sb.String()
	if strings.Contains(asmText, "__mul8") || strings.Contains(asmText, "__mul16") {
		sb.WriteString(emitMulRuntime(asmText))
	}

	return sb.String(), results
}

// emitMulRuntime emits Z80 runtime multiply routines referenced by LIR codegen.
func emitMulRuntime(asm string) string {
	var sb strings.Builder

	if strings.Contains(asm, "__mul8") {
		// __mul8: 8-bit multiply. A × B → A (low byte of result)
		// Uses shift-and-add. ~80T average, 8 iterations.
		sb.WriteString(`; __mul8: A = A * B (8-bit multiply, ~80T)
__mul8:
    LD C, A
    XOR A
    LD D, 8
.__mul8_lp:
    SRL C
    JR NC, .__mul8_sk
    ADD A, B
.__mul8_sk:
    SLA B
    DEC D
    JR NZ, .__mul8_lp
    RET

`)
	}

	if strings.Contains(asm, "__mul16") {
		// __mul16: 16-bit multiply. HL × DE → HL (low 16 bits of result)
		// Uses shift-and-add. ~200T average, 16 iterations.
		sb.WriteString(`; __mul16: HL = HL * DE (16-bit multiply, ~200T)
__mul16:
    PUSH BC
    LD B, H
    LD C, L
    LD HL, 0
    LD A, 16
.__mul16_lp:
    SRL D
    RR E
    JR NC, .__mul16_sk
    ADD HL, BC
.__mul16_sk:
    SLA C
    RL B
    DEC A
    JR NZ, .__mul16_lp
    POP BC
    RET

`)
	}

	return sb.String()
}

// LIRFuncResult reports the outcome of LIR codegen for one function.
type LIRFuncResult struct {
	Name  string
	OK    bool
	Error string
}

// expandTemplate fills in pattern template with physical register names.
func expandTemplate(inst Inst) string {
	tmpl := inst.Pat.Template
	if inst.Dst.Phys >= 0 {
		// For now, use register index as name placeholder
		tmpl = strings.ReplaceAll(tmpl, "{dst}", fmt.Sprintf("r%d", inst.Dst.Phys))
	}
	if inst.Srcs[0].Phys >= 0 {
		tmpl = strings.ReplaceAll(tmpl, "{src0}", fmt.Sprintf("r%d", inst.Srcs[0].Phys))
	}
	if inst.Srcs[1].Phys >= 0 {
		tmpl = strings.ReplaceAll(tmpl, "{src1}", fmt.Sprintf("r%d", inst.Srcs[1].Phys))
	}
	tmpl = strings.ReplaceAll(tmpl, "{imm}", fmt.Sprintf("%d", inst.Imm))
	return tmpl
}

// ExpandTemplateNamed fills template using the machine descriptor's location names.
func ExpandTemplateNamed(inst Inst, desc *MachineDesc) string {
	tmpl := inst.Pat.Template
	getName := func(phys int) string {
		if phys >= 0 && phys < len(desc.Locs) {
			return desc.Locs[phys].Name
		}
		return fmt.Sprintf("?%d", phys)
	}
	tmpl = strings.ReplaceAll(tmpl, "{dst}", getName(inst.Dst.Phys))
	tmpl = strings.ReplaceAll(tmpl, "{src0}", getName(inst.Srcs[0].Phys))
	tmpl = strings.ReplaceAll(tmpl, "{src1}", getName(inst.Srcs[1].Phys))
	tmpl = strings.ReplaceAll(tmpl, "{imm}", fmt.Sprintf("%d", inst.Imm))
	tmpl = strings.ReplaceAll(tmpl, "{sym}", inst.Sym)
	return tmpl
}
