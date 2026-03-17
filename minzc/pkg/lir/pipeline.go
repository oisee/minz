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
			cr := checkFuncConvergence(f, desc)
			cr.FuncName = f.Name
			cr.Machine = desc.Name
			results = append(results, cr)
		}
	}

	return results
}

func checkFuncConvergence(f *mir2.Func, desc *MachineDesc) ConvergenceResult {
	// Try multi-block path for functions with >1 block and block params.
	if len(f.Blocks) > 1 && hasBlockParams(f) {
		mbCR := checkFuncConvergenceMultiBlock(f, desc)
		if mbCR.Error == "" {
			return mbCR
		}
		// Fallback to flat path on multi-block failure.
	}

	// Flat path: flatten all blocks into one sequence (existing behavior).
	return checkFuncConvergenceFlat(f, desc)
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
func checkFuncConvergenceMultiBlock(f *mir2.Func, desc *MachineDesc) ConvergenceResult {
	cr := ConvergenceResult{}

	prog, blockOps, err := LowerMIR2ProgWithOps(f, desc)
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
func checkFuncConvergenceFlat(f *mir2.Func, desc *MachineDesc) ConvergenceResult {
	cr := ConvergenceResult{}

	// Step 1: Lower MIR2 blocks to MIROps
	var allOps []MIROp
	for _, b := range f.Blocks {
		ops, err := LowerMIR2Block(b, desc)
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

	// Step 3: Instruction selection
	sel, err := SelectInstructions(desc, combResult.Ops)
	if err != nil {
		cr.Error = fmt.Sprintf("isel: %s", err)
		return cr
	}

	// Step 4: WFC constraint propagation + collapse
	wfc := NewWFCState(desc, sel.Insts)
	wfc.Propagate()
	if err := wfc.Collapse(); err != nil {
		cr.Error = fmt.Sprintf("wfc: %s", err)
		return cr
	}

	insts := wfc.ToInsts()
	cr.InstCount = len(insts)
	cr.Match = true
	return cr
}

// LIRCodegenFunc runs the full LIR pipeline on a single MIR2 function
// and returns Z80 assembly text. This is the future replacement for
// mir2.Z80Codegen (per-function).
func LIRCodegenFunc(f *mir2.Func) (string, error) {
	desc := Z80

	// Try multi-block path for functions with block params.
	if len(f.Blocks) > 1 && hasBlockParams(f) {
		asm, err := lirCodegenMultiBlock(f, desc)
		if err == nil {
			return asm, nil
		}
		// Fallback to flat on failure.
	}

	return lirCodegenFlat(f, desc)
}

// lirCodegenMultiBlock emits per-block labels, instructions, and terminators.
func lirCodegenMultiBlock(f *mir2.Func, desc *MachineDesc) (string, error) {
	prog, blockOps, err := LowerMIR2ProgWithOps(f, desc)
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
func lirCodegenFlat(f *mir2.Func, desc *MachineDesc) (string, error) {
	// Lower MIR2 → MIROps
	var allOps []MIROp
	for _, b := range f.Blocks {
		ops, err := LowerMIR2Block(b, desc)
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

	// Isel
	sel, err := SelectInstructions(desc, combResult.Ops)
	if err != nil {
		return "", fmt.Errorf("isel %s: %w", f.Name, err)
	}

	// WFC
	wfc := NewWFCState(desc, sel.Insts)
	wfc.Propagate()
	if err := wfc.Collapse(); err != nil {
		return "", fmt.Errorf("wfc %s: %w", f.Name, err)
	}

	insts := wfc.ToInsts()

	// Emit assembly from templates
	var sb strings.Builder
	fmt.Fprintf(&sb, "; %s — LIR codegen (%d insts)\n", f.Name, len(insts))
	fmt.Fprintf(&sb, "%s:\n", f.Name)
	for _, inst := range insts {
		if inst.Pat == nil {
			continue
		}
		line := ExpandTemplateNamed(inst, desc)
		fmt.Fprintf(&sb, "    %s\n", line)
	}
	sb.WriteString("    RET\n")

	return sb.String(), nil
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
	return tmpl
}
