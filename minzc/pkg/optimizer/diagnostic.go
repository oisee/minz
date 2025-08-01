package optimizer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"github.com/minz/minzc/pkg/ir"
)

// DiagnosticReason represents why a peephole pattern was triggered
type DiagnosticReason int

const (
	ReasonUnknown DiagnosticReason = iota
	ReasonCodegenInefficiency    // Codegen produced suboptimal sequence
	ReasonMIROptimization       // MIR optimizer missed an opportunity  
	ReasonSemanticAnalysis      // Semantic analyzer created redundancy
	ReasonSuspiciousPair        // Suspicious instruction pair (potential bug)
	ReasonTemplateInefficiency  // Template-based codegen inefficiency
)

func (r DiagnosticReason) String() string {
	switch r {
	case ReasonCodegenInefficiency:
		return "Codegen Inefficiency"
	case ReasonMIROptimization:
		return "MIR Optimization Missed"
	case ReasonSemanticAnalysis:
		return "Semantic Analysis Redundancy"
	case ReasonSuspiciousPair:
		return "Suspicious Instruction Pair"
	case ReasonTemplateInefficiency:
		return "Template Inefficiency"
	default:
		return "Unknown"
	}
}

// PeepholeDiagnostic contains detailed analysis of why a pattern was triggered
type PeepholeDiagnostic struct {
	PatternName    string            // Name of triggered pattern
	Function       string            // Function where it occurred
	Instructions   []ir.Instruction  // The problematic instruction sequence
	Reason         DiagnosticReason  // Why this pattern occurred
	Explanation    string            // Human-readable explanation
	SourceLocation string            // Location in original MinZ source
	MIRContext     []ir.Instruction  // Surrounding MIR context
	Severity       string            // "info", "warning", "suspicious", "bug"
	AutoFixable    bool             // Can this be fixed automatically?
	SuggestedFix   string           // Suggested improvement to compiler
}

// DiagnosticCollector collects and analyzes peephole patterns
type DiagnosticCollector struct {
	diagnostics []PeepholeDiagnostic
	issueRepo   string // GitHub repo for auto-filing issues
	enabled     bool   // Enable diagnostic collection
}

// NewDiagnosticCollector creates a new diagnostic collector
func NewDiagnosticCollector(issueRepo string) *DiagnosticCollector {
	return &DiagnosticCollector{
		diagnostics: []PeepholeDiagnostic{},
		issueRepo:   issueRepo,
		enabled:     true,
	}
}

// CollectDiagnostic analyzes and records a peephole pattern trigger
func (dc *DiagnosticCollector) CollectDiagnostic(
	patternName string,
	function *ir.Function,
	instructions []ir.Instruction,
	position int,
) {
	if !dc.enabled {
		return
	}
	
	diagnostic := dc.analyzePeepholePattern(patternName, function, instructions, position)
	dc.diagnostics = append(dc.diagnostics, diagnostic)
	
	// Auto-file issue for suspicious patterns
	if diagnostic.Severity == "suspicious" || diagnostic.Severity == "bug" {
		dc.maybeFileIssue(diagnostic)
	}
}

// analyzePeepholePattern performs deep analysis of why a pattern occurred
func (dc *DiagnosticCollector) analyzePeepholePattern(
	patternName string,
	function *ir.Function, 
	instructions []ir.Instruction,
	position int,
) PeepholeDiagnostic {
	
	diagnostic := PeepholeDiagnostic{
		PatternName:  patternName,
		Function:     function.Name,
		Instructions: instructions,
		MIRContext:   dc.extractContext(function.Instructions, position, 5),
	}
	
	// Pattern-specific analysis
	switch patternName {
	case "load_zero_to_xor":
		diagnostic.Reason = ReasonCodegenInefficiency
		diagnostic.Explanation = "Codegen used LD A,0 instead of more efficient XOR A,A"
		diagnostic.Severity = "info"
		diagnostic.AutoFixable = true
		diagnostic.SuggestedFix = "Update Z80 codegen templates to use XOR A,A for zero loading"
		
	case "small_offset_to_inc":
		diagnostic.Reason = ReasonTemplateInefficiency  
		diagnostic.Explanation = "Struct field access used LD DE,offset + ADD HL,DE instead of INC HL sequence"
		diagnostic.Severity = "info"
		diagnostic.AutoFixable = true
		diagnostic.SuggestedFix = "Improve struct field codegen to use INC for small constant offsets"
		
	case "add_one_to_inc":
		diagnostic.Reason = ReasonMIROptimization
		diagnostic.Explanation = "MIR generated LoadConst 1 + Add instead of direct increment"
		diagnostic.Severity = "warning"
		diagnostic.AutoFixable = true
		diagnostic.SuggestedFix = "Add increment recognition to MIR optimization passes"
		
	case "double_jump":
		diagnostic.Reason = ReasonSuspiciousPair
		diagnostic.Explanation = "Control flow generated indirect jump sequence - possible logic bug"
		diagnostic.Severity = "suspicious"
		diagnostic.AutoFixable = false
		diagnostic.SuggestedFix = "Review control flow generation logic for unnecessary jumps"
		
	default:
		// Analyze instruction sequence for patterns
		if dc.looksLikeDeadStore(instructions) {
			diagnostic.Reason = ReasonSuspiciousPair
			diagnostic.Explanation = "Two stores to same location - possible semantic analysis bug"
			diagnostic.Severity = "suspicious"
			diagnostic.SuggestedFix = "Review why two sequential stores were generated"
		} else if dc.looksLikeRedundantLoad(instructions) {
			diagnostic.Reason = ReasonCodegenInefficiency
			diagnostic.Explanation = "Two loads of same value - codegen could be optimized"
			diagnostic.Severity = "warning"
			diagnostic.SuggestedFix = "Improve register allocation to reuse loaded values"
		} else {
			diagnostic.Reason = ReasonUnknown
			diagnostic.Explanation = "Pattern triggered but cause unclear"
			diagnostic.Severity = "info"
		}
	}
	
	return diagnostic
}

// looksLikeDeadStore detects suspicious dead store patterns
func (dc *DiagnosticCollector) looksLikeDeadStore(instructions []ir.Instruction) bool {
	if len(instructions) < 2 {
		return false
	}
	
	// Look for: Store addr, val1; Store addr, val2 
	for i := 0; i < len(instructions)-1; i++ {
		if (instructions[i].Op == ir.OpStoreVar || instructions[i].Op == ir.OpStoreDirect) &&
		   (instructions[i+1].Op == ir.OpStoreVar || instructions[i+1].Op == ir.OpStoreDirect) {
			// Same destination?
			if instructions[i].Symbol == instructions[i+1].Symbol ||
			   instructions[i].Imm == instructions[i+1].Imm {
				return true
			}
		}
	}
	return false
}

// looksLikeRedundantLoad detects redundant load patterns  
func (dc *DiagnosticCollector) looksLikeRedundantLoad(instructions []ir.Instruction) bool {
	if len(instructions) < 2 {
		return false
	}
	
	// Look for: Load reg1, val; Load reg2, val (same val, different regs)
	for i := 0; i < len(instructions)-1; i++ {
		if (instructions[i].Op == ir.OpLoadConst || instructions[i].Op == ir.OpLoadVar) &&
		   (instructions[i+1].Op == ir.OpLoadConst || instructions[i+1].Op == ir.OpLoadVar) {
			// Same source, different destinations?
			if (instructions[i].Symbol == instructions[i+1].Symbol ||
			    instructions[i].Imm == instructions[i+1].Imm) &&
			   instructions[i].Dest != instructions[i+1].Dest {
				return true
			}
		}
	}
	return false
}

// extractContext extracts surrounding MIR instructions for analysis
func (dc *DiagnosticCollector) extractContext(
	allInstructions []ir.Instruction, 
	position int, 
	contextSize int,
) []ir.Instruction {
	start := max(0, position-contextSize)
	end := min(len(allInstructions), position+contextSize)
	return allInstructions[start:end]
}

// GenerateReport generates a comprehensive diagnostic report
func (dc *DiagnosticCollector) GenerateReport() string {
	if len(dc.diagnostics) == 0 {
		return "🎉 No peephole patterns triggered - excellent code generation!"
	}
	
	var report strings.Builder
	report.WriteString("📊 Peephole Diagnostic Report\n")
	report.WriteString("═══════════════════════════════════════\n\n")
	
	// Summary statistics
	severityCounts := make(map[string]int)
	reasonCounts := make(map[DiagnosticReason]int)
	
	for _, diag := range dc.diagnostics {
		severityCounts[diag.Severity]++
		reasonCounts[diag.Reason]++
	}
	
	report.WriteString("📈 Summary:\n")
	for severity, count := range severityCounts {
		report.WriteString(fmt.Sprintf("  %s: %d patterns\n", severity, count))
	}
	report.WriteString("\n")
	
	report.WriteString("🔍 Root Causes:\n")
	for reason, count := range reasonCounts {
		report.WriteString(fmt.Sprintf("  %s: %d patterns\n", reason.String(), count))
	}
	report.WriteString("\n")
	
	// Detailed diagnostics
	report.WriteString("📋 Detailed Analysis:\n")
	for i, diag := range dc.diagnostics {
		report.WriteString(fmt.Sprintf("\n%d. %s [%s]\n", i+1, diag.PatternName, diag.Severity))
		report.WriteString(fmt.Sprintf("   Function: %s\n", diag.Function))
		report.WriteString(fmt.Sprintf("   Reason: %s\n", diag.Reason.String()))
		report.WriteString(fmt.Sprintf("   Explanation: %s\n", diag.Explanation))
		if diag.SuggestedFix != "" {
			report.WriteString(fmt.Sprintf("   💡 Suggested Fix: %s\n", diag.SuggestedFix))
		}
		
		// Show optimized instructions
		if len(diag.Instructions) > 0 {
			report.WriteString("   🔧 Optimized Instructions:\n")
			for _, inst := range diag.Instructions {
				report.WriteString(fmt.Sprintf("      %s\n", dc.formatInstruction(inst)))
			}
		}
	}
	
	return report.String()
}

// formatInstruction formats an IR instruction for display
func (dc *DiagnosticCollector) formatInstruction(inst ir.Instruction) string {
	return fmt.Sprintf("%s r%d, r%d, r%d ; %s", 
		inst.Op.String(), inst.Dest, inst.Src1, inst.Src2, inst.Comment)
}

// maybeFileIssue automatically files a GitHub issue for suspicious patterns
func (dc *DiagnosticCollector) maybeFileIssue(diagnostic PeepholeDiagnostic) {
	if dc.issueRepo == "" {
		return // No repo configured
	}
	
	// Create issue file for manual review/automation
	issueDir := filepath.Join("debug", "auto_issues")
	os.MkdirAll(issueDir, 0755)
	
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(issueDir, fmt.Sprintf("peephole_%s_%s.md", 
		diagnostic.PatternName, timestamp))
	
	issueContent := dc.generateIssueContent(diagnostic)
	os.WriteFile(filename, []byte(issueContent), 0644)
	
	fmt.Printf("🐛 Suspicious pattern detected - issue draft created: %s\n", filename)
}

// generateIssueContent creates GitHub issue content for a diagnostic
func (dc *DiagnosticCollector) generateIssueContent(diagnostic PeepholeDiagnostic) string {
	var content strings.Builder
	
	content.WriteString(fmt.Sprintf("# Peephole Pattern: %s (%s)\n\n", 
		diagnostic.PatternName, diagnostic.Severity))
	
	content.WriteString("## Problem Description\n")
	content.WriteString(fmt.Sprintf("%s\n\n", diagnostic.Explanation))
	
	content.WriteString("## Root Cause Analysis\n")
	content.WriteString(fmt.Sprintf("**Reason**: %s\n\n", diagnostic.Reason.String()))
	
	content.WriteString("## Affected Code\n")
	content.WriteString(fmt.Sprintf("**Function**: `%s`\n\n", diagnostic.Function))
	
	if len(diagnostic.Instructions) > 0 {
		content.WriteString("**Problematic IR Sequence**:\n```\n")
		for _, inst := range diagnostic.Instructions {
			content.WriteString(fmt.Sprintf("%s\n", dc.formatInstruction(inst)))
		}
		content.WriteString("```\n\n")
	}
	
	if len(diagnostic.MIRContext) > 0 {
		content.WriteString("**MIR Context** (surrounding instructions):\n```\n")
		for _, inst := range diagnostic.MIRContext {
			content.WriteString(fmt.Sprintf("%s\n", dc.formatInstruction(inst)))
		}
		content.WriteString("```\n\n")
	}
	
	content.WriteString("## Suggested Fix\n")
	if diagnostic.SuggestedFix != "" {
		content.WriteString(fmt.Sprintf("%s\n\n", diagnostic.SuggestedFix))
	} else {
		content.WriteString("Manual investigation required.\n\n")
	}
	
	content.WriteString("## Labels\n")
	content.WriteString("- `peephole-diagnostic`\n")
	content.WriteString(fmt.Sprintf("- `%s`\n", diagnostic.Severity))
	content.WriteString(fmt.Sprintf("- `%s`\n", strings.ToLower(strings.ReplaceAll(diagnostic.Reason.String(), " ", "-"))))
	if diagnostic.AutoFixable {
		content.WriteString("- `auto-fixable`\n")
	}
	
	content.WriteString("\n---\n")
	content.WriteString("*Auto-generated by MinZ Peephole Diagnostic System*\n")
	
	return content.String()
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}