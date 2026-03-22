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
	// Skip void functions and multi-return (no single value to compare).
	if len(f.Contract.Returns) != 1 {
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
		// Skip if function reads memory or uses indirect calls — LIR-VM
		// can't resolve symbol addresses or pre-initialized memory.
		if inst.Pat.MIROp == OpLoad || inst.Pat.MIROp == OpLoad16LE {
			return nil, nil
		}
		if inst.Sym == "__call_hl" {
			return nil, nil // indirect call — VM can't resolve function addresses
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
	// Use e-graph bridge for multi-variant lowering.
	prog, blockOps, _, err := LowerMIR2ProgWithEGraph(f, desc, m)
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

	// Copy Phys assignments from WFC cells back to block instructions.
	// ProgWFC.Collapse modifies cells but doesn't update prog.Blocks[].Insts.
	for _, label := range pw.BlockOrder() {
		wfc := pw.States[label]
		if wfc == nil {
			continue
		}
		bi := pw.BlockIndex(label)
		if bi < 0 {
			continue
		}
		b := &prog.Blocks[bi]
		cellIdx := 0
		// Skip param cells (Pat == nil).
		for cellIdx < len(wfc.Cells) && wfc.Cells[cellIdx].Pat == nil {
			cellIdx++
		}
		for ii := range b.Insts {
			if cellIdx >= len(wfc.Cells) {
				break
			}
			c := &wfc.Cells[cellIdx]
			b.Insts[ii].Dst.Phys = PhysOf(c.DstLocs)
			b.Insts[ii].Dst.Allowed = c.DstLocs
			for s := 0; s < 2; s++ {
				b.Insts[ii].Srcs[s].Phys = PhysOf(c.SrcLocs[s])
				b.Insts[ii].Srcs[s].Allowed = c.SrcLocs[s]
			}
			if c.Pat != nil {
				b.Insts[ii].Pat = c.Pat
			}
			cellIdx++
		}
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

	// Label prefix for non-entry blocks: .funcname_label (unique across module).
	labelPrefix := "." + strings.ReplaceAll(strings.ReplaceAll(f.Name, "$", "_"), "-", "_")

	// Collect all defined block labels so we can emit stubs for removed targets.
	definedLabels := make(map[string]bool)
	for _, b := range prog.Blocks {
		definedLabels[b.Label] = true
	}

	sanitizedFuncName := strings.ReplaceAll(strings.ReplaceAll(f.Name, "$", "_"), "-", "_")

	for bi, b := range prog.Blocks {
		if bi == 0 {
			fmt.Fprintf(&sb, "%s:\n", sanitizedFuncName)
		} else {
			sanitizedLabel := strings.ReplaceAll(b.Label, "-", "_")
			fmt.Fprintf(&sb, "%s_%s:\n", labelPrefix, sanitizedLabel)
		}

		// Post-WFC fixup: fix stores with pair values via non-HL pointers.
		blockInsts := fixSelfStores(b.Insts, desc)

		for _, inst := range blockInsts {
			if inst.Pat == nil {
				continue
			}
			line := ExpandTemplateNamed(inst, desc)
			fmt.Fprintf(&sb, "    %s\n", line)
		}

		// Emit parallel-copy moves before terminator where arg.Phys != param.Phys
		emitParallelCopyMoves(&sb, &b, prog, desc)

		// Emit terminator
		emitTerminatorPrefixed(&sb, &b.Term, bi, len(prog.Blocks), labelPrefix)
	}

	// Emit stub labels for any block targets removed by ApplyBlockRules.
	// Without these, JP to a removed block produces an undefined label error.
	for _, b := range prog.Blocks {
		for _, target := range b.Term.Targets {
			if target != "" && !definedLabels[target] {
				sanitized := strings.ReplaceAll(target, "-", "_")
				fullLabel := labelPrefix + "_" + sanitized
				fmt.Fprintf(&sb, "%s: ; stub (block eliminated)\n", fullLabel)
				definedLabels[target] = true // only once
			}
		}
	}

	// Spill labels emitted at module level by LIRCodegenModule.
	asm := sb.String()

	// Final text-level fixup for any remaining invalid Z80.
	asm = strings.ReplaceAll(asm, "    LD (HL), HL\n",
		"    LD D, H\n    LD E, L\n    LD (HL), E\n    INC HL\n    LD (HL), D\n    DEC HL\n")

	// Emit stub labels for JP targets that have no definition in the asm.
	asm = emitMissingLabels(asm)

	// Post-emit peephole optimization
	asm = Z80Peephole(asm)

	// Z80 validation — catch invalid instructions before they leave the compiler.
	// Currently logs warnings; will hard-fail after WFC constraint bugs are fixed.
	if errs := ValidateZ80Asm(asm); len(errs) > 0 {
		LogValidationErrors(f.Name, asm, errs)
	}

	return asm, nil
}

// emitMissingLabels scans asm text for JP/JR targets and emits stub
// label definitions for any targets not defined in the asm.
func emitMissingLabels(asm string) string {
	// Collect all defined labels.
	defined := make(map[string]bool)
	for _, line := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") || strings.Contains(trimmed, ": ;") {
			label := strings.SplitN(trimmed, ":", 2)[0]
			defined[label] = true
		}
	}

	// Find all JP/JR targets.
	var missing []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"JP ", "JP NZ, ", "JP Z, ", "JP C, ", "JP NC, ", "JR ", "JR NZ, ", "JR Z, ", "JR C, ", "JR NC, ", "DJNZ "} {
			if strings.HasPrefix(trimmed, prefix) {
				target := strings.TrimSpace(trimmed[len(prefix):])
				if target != "" && strings.HasPrefix(target, ".") && !defined[target] && !seen[target] {
					missing = append(missing, target)
					seen[target] = true
				}
			}
		}
	}

	if len(missing) == 0 {
		return asm
	}

	var sb strings.Builder
	sb.WriteString(asm)
	for _, label := range missing {
		fmt.Fprintf(&sb, "%s: ; stub (target block eliminated)\n", label)
	}
	return sb.String()
}

// emitSpillLabels emits data section labels for memory spill slots and TSMC
// slots referenced by LIR codegen. Each slot is 1-2 bytes of scratch memory.
func emitSpillLabels(sb *strings.Builder, desc *MachineDesc, funcName string) {
	// Collect all LocMem and LocTSMC names from the descriptor.
	var memLocs, tsmcLocs []string
	for _, loc := range desc.Locs {
		if loc.Kind == LocMem {
			memLocs = append(memLocs, loc.Name)
		}
		if loc.Kind == LocTSMC {
			tsmcLocs = append(tsmcLocs, loc.Name)
		}
	}
	if len(memLocs)+len(tsmcLocs) == 0 {
		return
	}
	// Spill labels are shared across functions (like PBQP's $F0xx page).
	// Emit once — caller should deduplicate.
	sb.WriteString("; spill data\n")
	for _, name := range memLocs {
		fmt.Fprintf(sb, "%s: DW 0\n", name)
	}
	for _, name := range tsmcLocs {
		fmt.Fprintf(sb, "%s: DB 0\n", name)
	}
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

// emitTerminatorPrefixed emits the assembly for a block terminator with label prefix.
func emitTerminatorPrefixed(sb *strings.Builder, term *Term, blockIdx, numBlocks int, prefix string) {
	// Wrap targets with prefix for non-entry labels.
	pfx := func(target string) string {
		if target == "" {
			return ""
		}
		return prefix + "_" + strings.ReplaceAll(target, "-", "_")
	}

	switch term.Kind {
	case TermNone:
	case TermJump:
		if len(term.Targets) > 0 {
			fmt.Fprintf(sb, "    JP %s\n", pfx(term.Targets[0]))
		}
	case TermBranch:
		if len(term.Targets) >= 2 {
			if term.Targets[0] != "" {
				fmt.Fprintf(sb, "    JP NZ, %s\n", pfx(term.Targets[0]))
			}
			if term.Targets[1] != "" {
				fmt.Fprintf(sb, "    JP %s\n", pfx(term.Targets[1]))
			}
		}
	case TermDJNZ:
		if len(term.Targets) >= 2 {
			fmt.Fprintf(sb, "    DJNZ %s\n", pfx(term.Targets[0]))
			if blockIdx+1 < numBlocks {
				// Fall through to exit
			} else {
				fmt.Fprintf(sb, "    JP %s\n", pfx(term.Targets[1]))
			}
		}
	case TermReturn:
		fmt.Fprintf(sb, "    RET\n")
	}
}

// emitTerminator emits the assembly for a block terminator (legacy, no prefix).
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
			// Skip empty branch targets (block was eliminated by CFG rules).
			if term.Targets[0] != "" {
				fmt.Fprintf(sb, "    JP NZ, %s\n", term.Targets[0])
			}
			if term.Targets[1] != "" {
				fmt.Fprintf(sb, "    JP %s\n", term.Targets[1])
			}
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

	// Validate-reject-retry loop: WFC → emit → validate → retry with rejected
	// assignments if validation finds invalid instructions. Max 3 attempts.
	const maxRetries = 3
	var rejected map[int]LocSet // vreg → banned phys locations (accumulated across retries)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// WFC with param cells + PBQP hints
		wfc := NewWFCStateWithParams(desc, sel.Insts, params)
		if len(hints) > 0 && hints[0] != nil {
			wfc.Hints = hints[0]
		}
		// Apply rejected assignments from previous attempts
		if rejected != nil {
			for vreg, banned := range rejected {
				for ci := range wfc.Cells {
					c := &wfc.Cells[ci]
					if c.VRegDst == vreg {
						c.DstLocs = c.DstLocs.Subtract(banned)
					}
					if c.VRegSrc[0] == vreg {
						c.SrcLocs[0] = c.SrcLocs[0].Subtract(banned)
					}
					if c.VRegSrc[1] == vreg {
						c.SrcLocs[1] = c.SrcLocs[1].Subtract(banned)
					}
				}
			}
		}
		wfc.Propagate()
		if err := wfc.Collapse(); err != nil {
			if attempt < maxRetries {
				continue // retry with different state
			}
			return "", fmt.Errorf("wfc %s: %w", f.Name, err)
		}

		insts := wfc.ToInsts()

		// Emit assembly from templates, with caller-save spills around CALLs.
		var sb strings.Builder
		fmt.Fprintf(&sb, "; %s — LIR codegen (%d insts, attempt %d)\n", f.Name, len(insts), attempt)
		sanitizedName := strings.ReplaceAll(strings.ReplaceAll(f.Name, "$", "_"), "-", "_")
		fmt.Fprintf(&sb, "%s:\n", sanitizedName)
		paramPhys := make(map[int]int) // vreg → phys
		for _, c := range wfc.Cells {
			if c.Pat == nil && c.VRegDst >= 0 {
				if p := PhysOf(c.DstLocs); p >= 0 {
					paramPhys[c.VRegDst] = p
				}
			}
		}
		emitInstsWithCallSpills(&sb, insts, desc, paramPhys)

		// Final text-level fixup for remaining invalid Z80.
		asmText := sb.String()
		asmText = strings.ReplaceAll(asmText, "    LD (HL), HL\n",
			"    LD D, H\n    LD E, L\n    LD (HL), E\n    INC HL\n    LD (HL), D\n    DEC HL\n")
		asmText = emitMissingLabels(asmText)
		// Post-emit peephole optimization
		asmPeepholed := Z80Peephole(asmText)
		sb.Reset()
		sb.WriteString(asmPeepholed)

		// Tail call optimization
		asmSoFar := sb.String()
		if idx := strings.LastIndex(asmSoFar, "    CALL "); idx >= 0 {
			callLine := asmSoFar[idx:]
			if nlIdx := strings.IndexByte(callLine, '\n'); nlIdx >= 0 {
				after := strings.TrimSpace(callLine[nlIdx+1:])
				if after == "" {
					sb.Reset()
					sb.WriteString(asmSoFar[:idx])
					sb.WriteString("    JP")
					sb.WriteString(callLine[8:nlIdx+1])
					return sb.String(), nil
				}
			}
		}
		sb.WriteString("    RET\n")
		// Spill labels emitted at module level.

		asm := sb.String()

		// Z80 validation
		errs := ValidateZ80Asm(asm)
		if len(errs) == 0 {
			return asm, nil // clean — no invalid instructions
		}

		if attempt < maxRetries {
			// Build rejected assignments from invalid instructions.
			// For each invalid inst, find the WFC cell that emitted it and
			// reject its current phys assignment.
			if rejected == nil {
				rejected = make(map[int]LocSet)
			}
			for _, c := range wfc.Cells {
				if c.Pat == nil {
					continue
				}
				if c.VRegDst >= 0 {
					if p := PhysOf(c.DstLocs); p >= 0 {
						rejected[c.VRegDst] = rejected[c.VRegDst].Add(p)
					}
				}
			}
			fmt.Printf("[Z80-VALIDATE] %s: %d errors, retrying (attempt %d/%d)\n",
				f.Name, len(errs), attempt+1, maxRetries)
		} else {
			// Final attempt failed — log and emit anyway (warn-only).
			LogValidationErrors(f.Name, asm, errs)
			return asm, nil
		}
	}

	return "", fmt.Errorf("lir %s: exhausted retries", f.Name)
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
	if strings.Contains(asmText, "__call_hl") {
		sb.WriteString("; __call_hl: indirect call via HL (1 byte)\n__call_hl:\n    JP (HL)\n\n")
	}

	// Emit spill slot labels ONCE for the whole module.
	emitSpillLabels(&sb, Z80, "module")

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
// For shadow registers (B', C', etc.) inside EXX brackets, the template gets
// the prime name; patterns that wrap with EXX handle the naming themselves.
// For TSMC spill slots, the name is used as a label (tsmc0, tsmc1, etc.).
func ExpandTemplateNamed(inst Inst, desc *MachineDesc) string {
	// Template selection: if the current pattern's template hardcodes a
	// register that doesn't match the actual Phys assignment, find a
	// pattern whose SrcLocs/DstLocs actually contain the assigned Phys.
	// This fixes union-pattern mismatches where isel picks cheapest template
	// but WFC assigns to a different loc (e.g. LD ({src0}), HL with src1=BC).
	pat := inst.Pat
	if pat != nil && desc != nil {
		pat = selectTemplateForPhys(inst, desc)
	}

	tmpl := pat.Template
	getName := func(phys int) string {
		if phys >= 0 && phys < len(desc.Locs) {
			name := desc.Locs[phys].Name
			if desc.Locs[phys].Kind == LocShadow && strings.Contains(tmpl, "EXX") {
				name = strings.TrimSuffix(name, "'")
			}
			return name
		}
		return fmt.Sprintf("?%d", phys)
	}
	tmpl = strings.ReplaceAll(tmpl, "{dst}", getName(inst.Dst.Phys))
	tmpl = strings.ReplaceAll(tmpl, "{src0}", getName(inst.Srcs[0].Phys))
	tmpl = strings.ReplaceAll(tmpl, "{src1}", getName(inst.Srcs[1].Phys))
	tmpl = strings.ReplaceAll(tmpl, "{imm}", fmt.Sprintf("%d", inst.Imm))
	sym := strings.ReplaceAll(inst.Sym, "-", "_")
	tmpl = strings.ReplaceAll(tmpl, "{sym}", sym)

	// Last-resort fixup: if the expanded template is an invalid Z80 instruction
	// (e.g. LD (BC), HL — no such instruction), replace with a valid sequence.
	if desc != nil && desc.Name == "z80" {
		tmpl = fixInvalidZ80Template(tmpl, inst, desc, getName)
	}

	return tmpl
}

// fixInvalidZ80Template replaces known-invalid Z80 instruction patterns with
// valid multi-instruction sequences. This is the last line of defense after
// validate-reject, pattern-retry, and selectTemplateForPhys all failed.
func fixInvalidZ80Template(tmpl string, inst Inst, desc *MachineDesc, getName func(int) string) string {
	// LD (rr), HL/DE/BC — 16-bit store via non-HL pointer.
	// Z80 only supports LD (BC),A and LD (DE),A (8-bit, A only).
	// Decompose to byte-by-byte via A.
	src1Name := getName(inst.Srcs[1].Phys)
	src0Name := getName(inst.Srcs[0].Phys)

	// Check: is this a store of a 16-bit pair via another pair pointer?
	pairs16 := map[string][2]string{
		"HL": {"L", "H"}, "DE": {"E", "D"}, "BC": {"C", "B"},
	}
	// Detect pair names from template text when Phys is unset (-1).
	effectiveSrc0 := src0Name
	effectiveSrc1 := src1Name
	if effectiveSrc1 == "" || strings.HasPrefix(effectiveSrc1, "?") {
		for pair := range pairs16 {
			if strings.Contains(tmpl, ", "+pair) {
				effectiveSrc1 = pair
				break
			}
		}
	}
	if effectiveSrc0 == "" || strings.HasPrefix(effectiveSrc0, "?") {
		for pair := range pairs16 {
			if strings.Contains(tmpl, "("+pair+")") {
				effectiveSrc0 = pair
				break
			}
		}
	}
	if lo1hi1, valIsPair := pairs16[effectiveSrc1]; valIsPair {
		if _, ptrIsPair := pairs16[effectiveSrc0]; ptrIsPair && effectiveSrc0 != "HL" {
			// LD (BC/DE), HL/DE/BC → byte-by-byte via A
			return fmt.Sprintf("LD A, %s\n    LD (%s), A\n    INC %s\n    LD A, %s\n    LD (%s), A\n    DEC %s",
				lo1hi1[0], src0Name, src0Name, lo1hi1[1], src0Name, src0Name)
		}
		// LD (HL), HL → self-store: evacuate to DE first
		if effectiveSrc0 == "HL" && (effectiveSrc1 == "HL" || strings.Contains(tmpl, "(HL), HL")) {
			return "LD D, H\n    LD E, L\n    LD (HL), E\n    INC HL\n    LD (HL), D\n    DEC HL"
		}
		// LD (HL), DE/BC → byte-by-byte
		if effectiveSrc0 == "HL" {
			return fmt.Sprintf("LD (HL), %s\n    INC HL\n    LD (HL), %s\n    DEC HL",
				lo1hi1[0], lo1hi1[1])
		}
	}

	// LD (IX/IY), pair → byte-by-byte via A with (IX+d)
	if src0Name == "IX" || src0Name == "IY" || strings.Contains(tmpl, "(IX)") || strings.Contains(tmpl, "(IY)") {
		ixName := src0Name
		if strings.Contains(tmpl, "(IX)") {
			ixName = "IX"
		} else if strings.Contains(tmpl, "(IY)") {
			ixName = "IY"
		}
		// Detect value pair from template or Phys
		valPair := src1Name
		if strings.Contains(tmpl, ", HL") {
			valPair = "HL"
		} else if strings.Contains(tmpl, ", DE") {
			valPair = "DE"
		} else if strings.Contains(tmpl, ", BC") {
			valPair = "BC"
		}
		if lo1hi1, ok := pairs16[valPair]; ok {
			return fmt.Sprintf("LD A, %s\n    LD (%s+0), A\n    LD A, %s\n    LD (%s+1), A",
				lo1hi1[0], ixName, lo1hi1[1], ixName)
		}
	}

	// ADD HL, IX/IY → route through DE
	if strings.HasPrefix(tmpl, "ADD HL, IX") || strings.HasPrefix(tmpl, "ADD HL, IY") {
		ixName := src1Name // "IX" or "IY"
		return fmt.Sprintf("PUSH %s\n    POP DE\n    ADD HL, DE", ixName)
	}

	// ADD HL, mem → load to DE first
	if strings.HasPrefix(tmpl, "ADD HL, mem") || strings.HasPrefix(tmpl, "ADD HL, spill") {
		return fmt.Sprintf("LD DE, (%s)\n    ADD HL, DE", src1Name)
	}

	// INC/DEC on memory slot → load to A, INC/DEC A, store back
	if strings.HasPrefix(tmpl, "INC mem") || strings.HasPrefix(tmpl, "INC spill") {
		dstName := getName(inst.Dst.Phys)
		return fmt.Sprintf("LD A, (%s)\n    INC A\n    LD (%s), A", dstName, dstName)
	}
	if strings.HasPrefix(tmpl, "DEC mem") || strings.HasPrefix(tmpl, "DEC spill") {
		dstName := getName(inst.Dst.Phys)
		return fmt.Sprintf("LD A, (%s)\n    DEC A\n    LD (%s), A", dstName, dstName)
	}

	// LD H/L, IXH/IXL/IYH/IYL (DD/FD prefix conflict) → route through A
	// Check both expanded tmpl and Phys values.
	trimTmpl := strings.TrimSpace(tmpl)
	ddFdConflicts := []string{
		"LD L, IXH", "LD L, IXL", "LD L, IYH", "LD L, IYL",
		"LD H, IXH", "LD H, IXL", "LD H, IYH", "LD H, IYL",
		"LD IXH, H", "LD IXH, L", "LD IXL, H", "LD IXL, L",
		"LD IYH, H", "LD IYH, L", "LD IYL, H", "LD IYL, L",
	}
	for _, conflict := range ddFdConflicts {
		if trimTmpl == conflict {
			parts := strings.SplitN(conflict, ", ", 2)
			dst, src := strings.TrimPrefix(parts[0], "LD "), parts[1]
			return fmt.Sprintf("LD A, %s\n    LD %s, A", src, dst)
		}
	}

	// Operations on unallocated vregs (?-1) — skip (no valid codegen possible)
	if strings.Contains(tmpl, "?-1") || strings.Contains(tmpl, "?") {
		return "; UNALLOCATED: " + tmpl
	}

	// Final safety net: if expanded template still invalid, try DD/FD route
	if !ValidateInst(tmpl) {
		// Generic DD/FD conflict: any LD where dst is H/L and src is IXH/IXL/IYH/IYL
		// or vice versa → route through A.
		for _, line := range strings.Split(tmpl, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "LD ") {
				parts := strings.SplitN(strings.TrimPrefix(line, "LD "), ", ", 2)
				if len(parts) == 2 {
					d, s := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
					hlRegs := map[string]bool{"H": true, "L": true}
					ixyRegs := map[string]bool{"IXH": true, "IXL": true, "IYH": true, "IYL": true}
					if (hlRegs[d] && ixyRegs[s]) || (ixyRegs[d] && hlRegs[s]) {
						return fmt.Sprintf("LD A, %s\n    LD %s, A", s, d)
					}
				}
			}
		}
	}

	return tmpl
}

// selectTemplateForPhys finds the best pattern whose SrcLocs/DstLocs
// actually contain the assigned Phys values. Falls back to inst.Pat.
func selectTemplateForPhys(inst Inst, desc *MachineDesc) *Pattern {
	if inst.Pat == nil {
		return inst.Pat
	}

	// Check if current pattern's SrcLocs contain the assigned Phys.
	ok := true
	for s := 0; s < 2; s++ {
		phys := inst.Srcs[s].Phys
		if phys < 0 {
			continue
		}
		srcLocs := inst.Pat.SrcLocs[s]
		if !srcLocs.IsEmpty() && !srcLocs.Has(phys) {
			ok = false
			break
		}
	}
	if inst.Dst.Phys >= 0 && !inst.Pat.DstLocs.IsEmpty() && !inst.Pat.DstLocs.Has(inst.Dst.Phys) {
		ok = false
	}
	if ok {
		return inst.Pat // current pattern matches
	}

	// Find a better pattern whose locs actually match.
	for i := range desc.Patterns {
		p := &desc.Patterns[i]
		if p.MIROp != inst.Pat.MIROp {
			continue
		}
		if p.Width != 0 && inst.Pat.Width != 0 && p.Width != inst.Pat.Width {
			continue
		}
		match := true
		for s := 0; s < 2; s++ {
			phys := inst.Srcs[s].Phys
			if phys < 0 {
				continue
			}
			if !p.SrcLocs[s].IsEmpty() && !p.SrcLocs[s].Has(phys) {
				match = false
				break
			}
		}
		if inst.Dst.Phys >= 0 && !p.DstLocs.IsEmpty() && !p.DstLocs.Has(inst.Dst.Phys) {
			match = false
		}
		if match {
			return p
		}
	}

	return inst.Pat // fallback
}
