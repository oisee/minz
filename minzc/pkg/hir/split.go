// split.go — HIR-level automatic function splitting for register pressure reduction.
//
// The "Great Autorefactor": analyzes functions for high register pressure
// and splits them into sub-functions, each with its own PBQP allocation.
//
// Z80 has 7 GPRs. Functions with 8+ simultaneously live variables will spill.
// Splitting reduces pressure per sub-function → fewer (or zero) spills.
//
// Pipeline position: after Analyzer2, before LowerModule.
//
//	Parser → Analyzer2 → HIR Module
//	                        ↓
//	                 [SplitHighPressure]  ← this pass
//	                        ↓
//	                 LowerModule → MIR2
//	                        ↓
//	                 PBQP allocation (per sub-function, independent)
package hir

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// PressureThreshold is the default maximum number of simultaneously live
// variables before splitting is attempted. Z80 has 7 GPRs + 3 pairs = ~10
// slots, but pairs count as 1 slot for u16. Threshold of 8 is conservative.
// Use PressureThresholdForSlots for target-aware thresholds.
const PressureThreshold = 8

// PressureThresholdForSlots returns a target-appropriate threshold based on
// how many register slots the allocator can use.
//   7 GPR only:      threshold = 8  (L1, conservative)
//   11 (+ IXY):      threshold = 12 (L1+L2)
//   18 (+ shadow):   threshold = 14 (L1+L2+L3)
//   38 (+ TSMC/mem): threshold = 20 (L1+L2+L3+L4)
func PressureThresholdForSlots(nSlots int) int {
	switch {
	case nSlots >= 30:
		return 20
	case nSlots >= 18:
		return 14
	case nSlots >= 11:
		return 12
	default:
		return 8
	}
}

// MaxInterfaceWidth is the maximum number of inputs+outputs at a split boundary.
// Must fit in Z80 registers for the sub-function's calling convention.
// PFCCO supports ~6 param slots (A, B/C, D/E, H/L, IX, IY).
const MaxInterfaceWidth = 6

// SplitResult describes one split that was applied.
type SplitResult struct {
	OrigFunc  string // original function name
	SubFunc   string // generated sub-function name
	Inputs    int    // number of input parameters
	Outputs   int    // number of return values
	Pressure  int    // estimated max live vars in the sub-function
}

// SplitHighPressure analyzes all functions in the module and splits
// those with register pressure exceeding PressureThreshold.
// Uses optimal split point selection (minimizes max pressure across halves)
// and recursive multi-split (splits again if halves still exceed threshold).
// Returns the list of splits applied (for diagnostics).
func SplitHighPressure(m *Module) []SplitResult {
	var results []SplitResult
	var newFuncs []*Func

	// Process functions — iterate over snapshot since we may add sub-funcs.
	origFuncs := make([]*Func, len(m.Funcs))
	copy(origFuncs, m.Funcs)

	for _, f := range origFuncs {
		subs := splitRecursive(m, f, &results, 0)
		newFuncs = append(newFuncs, subs...)
	}

	m.Funcs = append(m.Funcs, newFuncs...)
	return results
}

// maxSplitDepth limits recursive splitting to prevent explosion.
const maxSplitDepth = 3

// splitRecursive splits a function if needed, then recursively splits
// the resulting sub-functions if they still exceed the threshold.
func splitRecursive(m *Module, f *Func, results *[]SplitResult, depth int) []*Func {
	if f.Body == nil || f.IsExtern || len(f.Body.Body) < 4 || depth > maxSplitDepth {
		return nil
	}
	// Guard: don't re-split if last stmt is already a split call.
	if len(f.Body.Body) > 0 {
		if es, ok := f.Body.Body[len(f.Body.Body)-1].(*ExprStmt); ok {
			if ce, ok := es.Expr.(*CallExpr); ok {
				if strings.Contains(ce.Fn, "$split_") {
					// Already split — skip top half re-analysis.
					return nil
				}
			}
		}
	}

	pressure := EstimatePressure(f)
	maxP := maxInt(pressure)
	if maxP <= PressureThreshold {
		return nil
	}

	best := findOptimalSplit(f, pressure)
	if best == nil {
		return nil
	}

	sub := ApplySplit(m, f, *best)
	if sub == nil {
		return nil
	}

	*results = append(*results, SplitResult{
		OrigFunc: f.Name,
		SubFunc:  sub.Name,
		Inputs:   len(sub.Params),
		Outputs:  countReturns(sub),
		Pressure: maxP,
	})

	var allSubs []*Func
	allSubs = append(allSubs, sub)

	// Recursively split both halves if still hot.
	// Top half is now truncated (top stmts + call) — re-check pressure.
	topSubs := splitRecursive(m, f, results, depth+1)
	allSubs = append(allSubs, topSubs...)

	// Bottom half (sub-function) may still have high pressure.
	botSubs := splitRecursive(m, sub, results, depth+1)
	allSubs = append(allSubs, botSubs...)

	return allSubs
}

// findOptimalSplit finds the split point that minimizes the maximum
// pressure across both halves, subject to interface width constraint.
// Returns nil if no valid split point exists.
func findOptimalSplit(f *Func, pressure []int) *splitCandidate {
	candidates := FindSplitPoints(f, pressure)
	if len(candidates) == 0 {
		return nil
	}

	stmts := f.Body.Body

	var best *splitCandidate
	bestCost := 999

	for i := range candidates {
		c := &candidates[i]

		// Compute max pressure in each half.
		topMax := 0
		for j := 0; j <= c.splitAt; j++ {
			if j < len(pressure) && pressure[j] > topMax {
				topMax = pressure[j]
			}
		}
		botMax := 0
		for j := c.splitAt + 1; j < len(stmts); j++ {
			if j < len(pressure) && pressure[j] > botMax {
				botMax = pressure[j]
			}
		}

		// Cost = max of both halves + interface penalty.
		cost := topMax
		if botMax > cost {
			cost = botMax
		}
		cost += c.interfaceWidth() // penalize wide interfaces

		if cost < bestCost {
			bestCost = cost
			best = c
		}
	}

	return best
}

// ── Pressure estimation ──────────────────────────────────────────────────────

// EstimatePressure returns per-statement live variable counts for a function.
// A variable is "live" from its declaration/first-def until its last-use.
func EstimatePressure(f *Func) []int {
	stmts := f.Body.Body
	if len(stmts) == 0 {
		return nil
	}

	// Collect variable references per statement.
	refs := make([]map[string]bool, len(stmts))
	defs := make([]map[string]bool, len(stmts))
	for i, s := range stmts {
		refs[i] = make(map[string]bool)
		defs[i] = make(map[string]bool)
		collectVarRefs(s, refs[i], defs[i])
	}

	// Add params as live from start.
	paramNames := make(map[string]bool)
	for _, p := range f.Params {
		paramNames[p.Name] = true
	}

	// For each variable, find first-def and last-use indices.
	allVars := make(map[string]bool)
	for i := range stmts {
		for v := range refs[i] {
			allVars[v] = true
		}
		for v := range defs[i] {
			allVars[v] = true
		}
	}
	for v := range paramNames {
		allVars[v] = true
	}

	firstDef := make(map[string]int)
	lastUse := make(map[string]int)
	for v := range allVars {
		firstDef[v] = len(stmts) // sentinel
		lastUse[v] = -1
	}
	for v := range paramNames {
		firstDef[v] = 0 // params defined at entry
	}
	for i := range stmts {
		for v := range defs[i] {
			if i < firstDef[v] {
				firstDef[v] = i
			}
		}
		for v := range refs[i] {
			if i > lastUse[v] {
				lastUse[v] = i
			}
		}
	}

	// Compute pressure: count vars live at each statement.
	pressure := make([]int, len(stmts))
	for i := range stmts {
		count := 0
		for v := range allVars {
			if firstDef[v] <= i && i <= lastUse[v] {
				count++
			}
		}
		pressure[i] = count
	}

	return pressure
}

// ── Split point identification ───────────────────────────────────────────────

// splitCandidate describes a potential split point between statements.
type splitCandidate struct {
	splitAt int      // split between stmt[splitAt] and stmt[splitAt+1]
	inputs  []string // vars defined before, used after
	outputs []string // vars defined after, used after (returned from sub-func)
}

func (s splitCandidate) interfaceWidth() int {
	return len(s.inputs) + len(s.outputs)
}

// FindSplitPoints returns viable split candidates for a function.
func FindSplitPoints(f *Func, pressure []int) []splitCandidate {
	stmts := f.Body.Body
	if len(stmts) < 4 {
		return nil
	}

	// Collect all var refs/defs per statement.
	refs := make([]map[string]bool, len(stmts))
	defs := make([]map[string]bool, len(stmts))
	for i, s := range stmts {
		refs[i] = make(map[string]bool)
		defs[i] = make(map[string]bool)
		collectVarRefs(s, refs[i], defs[i])
	}

	paramNames := make(map[string]bool)
	for _, p := range f.Params {
		paramNames[p.Name] = true
	}

	var candidates []splitCandidate

	// Try each position as a split point.
	for splitAt := 1; splitAt < len(stmts)-1; splitAt++ {
		// Don't split at low-pressure points.
		if pressure[splitAt] <= PressureThreshold {
			continue
		}

		// Compute interface: which vars cross the boundary.
		definedBefore := make(map[string]bool)
		for v := range paramNames {
			definedBefore[v] = true
		}
		for i := 0; i <= splitAt; i++ {
			for v := range defs[i] {
				definedBefore[v] = true
			}
		}

		usedAfter := make(map[string]bool)
		definedAfter := make(map[string]bool)
		for i := splitAt + 1; i < len(stmts); i++ {
			for v := range refs[i] {
				usedAfter[v] = true
			}
			for v := range defs[i] {
				definedAfter[v] = true
			}
		}

		// Inputs: defined before AND used after.
		var inputs []string
		for v := range definedBefore {
			if usedAfter[v] {
				inputs = append(inputs, v)
			}
		}

		// Outputs: defined after AND used after the split region.
		// (For simplicity, outputs = vars modified in bottom half that
		// are used in the top half or returned. Skip for V1 — just
		// split the bottom half as a void sub-function.)
		var outputs []string

		iw := len(inputs) + len(outputs)
		if iw > MaxInterfaceWidth {
			continue
		}

		candidates = append(candidates, splitCandidate{
			splitAt: splitAt,
			inputs:  inputs,
			outputs: outputs,
		})
	}

	return candidates
}

// ── Split application ────────────────────────────────────────────────────────

// ApplySplit extracts statements after the split point into a new sub-function.
// The original function gets a CALL to the sub-function at the split point.
// Returns the new sub-function, or nil if splitting failed.
func ApplySplit(m *Module, f *Func, c splitCandidate) *Func {
	stmts := f.Body.Body
	if c.splitAt >= len(stmts)-1 {
		return nil
	}

	// Build sub-function.
	subName := fmt.Sprintf("%s$split_%d", f.Name, c.splitAt)

	// Parameters = inputs (vars crossing the boundary).
	var params []Param
	varTypes := inferVarTypes(f)
	for _, v := range c.inputs {
		ty := varTypes[v]
		if ty == nil {
			ty = mir2.TyU8 // fallback
		}
		params = append(params, Param{Name: v, Ty: ty})
	}

	// Body = statements after split point.
	subBody := &Block{Body: make([]Stmt, len(stmts)-c.splitAt-1)}
	copy(subBody.Body, stmts[c.splitAt+1:])

	sub := &Func{
		Name:   subName,
		Params: params,
		RetTy:  mir2.TyVoid,
		Body:   subBody,
	}

	// Replace original function's body: keep top half + call to sub.
	args := make([]Expr, len(c.inputs))
	for i, v := range c.inputs {
		ty := varTypes[v]
		if ty == nil {
			ty = mir2.TyU8
		}
		args[i] = &VarRefExpr{Name: v, Ty: ty}
	}

	callExpr := &CallExpr{
		Fn:   subName,
		Args: args,
		Ty:   mir2.TyVoid,
	}

	// New body = top half + call statement.
	newBody := make([]Stmt, c.splitAt+2)
	copy(newBody, stmts[:c.splitAt+1])
	newBody[c.splitAt+1] = &ExprStmt{Expr: callExpr}
	f.Body.Body = newBody

	return sub
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// collectVarRefs walks a statement and collects variable references and definitions.
func collectVarRefs(s Stmt, refs, defs map[string]bool) {
	switch s := s.(type) {
	case *VarDeclStmt:
		defs[s.Name] = true
		if s.Init != nil {
			collectExprRefs(s.Init, refs)
		}
	case *AssignStmt:
		if vr, ok := s.Target.(*VarRefExpr); ok {
			defs[vr.Name] = true
		}
		collectExprRefs(s.Target, refs)
		collectExprRefs(s.Val, refs)
	case *ReturnStmt:
		for _, v := range s.Vals {
			collectExprRefs(v, refs)
		}
	case *IfStmt:
		collectExprRefs(s.Cond, refs)
		if s.Then != nil {
			for _, inner := range s.Then.Body {
				collectVarRefs(inner, refs, defs)
			}
		}
		if s.Else != nil {
			for _, inner := range s.Else.Body {
				collectVarRefs(inner, refs, defs)
			}
		}
	case *WhileStmt:
		collectExprRefs(s.Cond, refs)
		if s.Body != nil {
			for _, inner := range s.Body.Body {
				collectVarRefs(inner, refs, defs)
			}
		}
	case *ForRangeStmt:
		defs[s.Var] = true
		refs[s.Var] = true
		collectExprRefs(s.Start, refs)
		collectExprRefs(s.End, refs)
		if s.Body != nil {
			for _, inner := range s.Body.Body {
				collectVarRefs(inner, refs, defs)
			}
		}
	case *ExprStmt:
		collectExprRefs(s.Expr, refs)
	case *TupleLetStmt:
		for _, name := range s.Names {
			if name != "_" {
				defs[name] = true
			}
		}
		collectExprRefs(s.Call, refs)
	}
}

// collectExprRefs walks an expression and collects variable references.
func collectExprRefs(e Expr, refs map[string]bool) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *VarRefExpr:
		refs[e.Name] = true
	case *CallExpr:
		for _, a := range e.Args {
			collectExprRefs(a, refs)
		}
	case *BinExpr:
		collectExprRefs(e.L, refs)
		collectExprRefs(e.R, refs)
	case *UnaryExpr:
		collectExprRefs(e.X, refs)
	case *CastExpr:
		collectExprRefs(e.X, refs)
	case *IndexExpr:
		collectExprRefs(e.Base, refs)
		collectExprRefs(e.Idx, refs)
	case *FieldExpr:
		collectExprRefs(e.X, refs)
	case *AddrOfExpr:
		if e.X != nil {
			collectExprRefs(e.X, refs)
		}
	case *DerefExpr:
		collectExprRefs(e.Ptr, refs)
	}
}

// inferVarTypes builds a map of variable name → type from declarations and params.
func inferVarTypes(f *Func) map[string]mir2.Ty {
	types := make(map[string]mir2.Ty)
	for _, p := range f.Params {
		types[p.Name] = p.Ty
	}
	for _, s := range f.Body.Body {
		if vd, ok := s.(*VarDeclStmt); ok {
			types[vd.Name] = vd.Ty
		}
	}
	return types
}

func maxInt(s []int) int {
	m := 0
	for _, v := range s {
		if v > m {
			m = v
		}
	}
	return m
}

func countReturns(f *Func) int {
	if len(f.RetTys) > 0 {
		return len(f.RetTys)
	}
	if f.RetTy != nil && f.RetTy != mir2.TyVoid {
		return 1
	}
	return 0
}
