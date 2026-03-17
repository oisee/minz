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
