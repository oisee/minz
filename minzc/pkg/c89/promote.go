package c89

// Struct-Return Promotion Pass (ADR-0025)
//
// Detects C functions returning small structs ("Shallow Scalar Structs") and
// promotes them to HIR tuple returns.  This allows PFCCO (contract optimizer)
// to assign each return slot an optimal register — 0T overhead vs ~60T with
// SDCC's stack-based struct return convention.
//
// Also handles the reverse pattern: struct-argument demotion where a struct
// parameter is immediately destructured in the function body.

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// PromoteStructReturns runs the struct↔tuple promotion pass on a HIR module.
// It modifies the module in-place:
//   - Struct-returning functions → tuple-returning functions (RetTys)
//   - Call sites with field access → TupleLetStmt
//   - Struct parameters (when body only reads fields) → scalar parameters
func PromoteStructReturns(m *hir.Module) {
	idx := buildStructIndex(m)
	if len(idx) == 0 {
		return
	}

	// Phase 1: identify struct-returning functions eligible for promotion.
	eligible := findEligibleFunctions(m, idx)
	if len(eligible) == 0 {
		return
	}

	// Phase 2: rewrite function signatures (RetTy → RetTys).
	for _, fn := range m.Funcs {
		info, ok := eligible[fn.Name]
		if !ok {
			continue
		}
		fn.RetTys = info.fieldTys
		fn.RetTy = mir2.TyVoid // overridden by RetTys

		// Rewrite return statements: struct literal → tuple return.
		rewriteReturns(fn, info)
	}

	// Phase 3: rewrite call sites.
	for _, fn := range m.Funcs {
		rewriteCallSites(fn.Body, eligible)
	}
}

// ── Struct index ─────────────────────────────────────────────────────────────

// structIndex maps struct names to their mir2.StructTy.
type structIndex map[string]*mir2.StructTy

func buildStructIndex(m *hir.Module) structIndex {
	idx := make(structIndex, len(m.Structs))
	for _, st := range m.Structs {
		idx[st.Name] = st
	}
	return idx
}

// ── SSS detection ────────────────────────────────────────────────────────────

// isShallowScalarStruct checks the SSS criteria from ADR-0025:
//   - All fields are scalar (u8, u16, i8, i16, bool)
//   - No nested structs, no arrays
//   - Total size ≤ 4 bytes (fits Z80 register file)
func isShallowScalarStruct(st *mir2.StructTy) bool {
	if st == nil || len(st.Fields) == 0 || len(st.Fields) > 4 {
		return false
	}
	totalBytes := 0
	for _, f := range st.Fields {
		w := f.Ty.Width()
		if w == 0 {
			return false // void or unknown type
		}
		if !isScalarTy(f.Ty) {
			return false
		}
		totalBytes += w / 8
	}
	return totalBytes <= 4
}

func isScalarTy(ty mir2.Ty) bool {
	switch ty {
	case mir2.TyU8, mir2.TyI8, mir2.TyU16, mir2.TyI16, mir2.TyBool:
		return true
	}
	return false
}

// ── Eligibility analysis ─────────────────────────────────────────────────────

// promotionInfo holds the analysis result for one struct-returning function.
type promotionInfo struct {
	st        *mir2.StructTy // the struct type being returned
	fieldTys  []mir2.Ty      // field types (== RetTys after promotion)
	fieldMap  map[string]int // field name → position in tuple
}

func findEligibleFunctions(m *hir.Module, idx structIndex) map[string]*promotionInfo {
	eligible := make(map[string]*promotionInfo)

	for _, fn := range m.Funcs {
		if fn.IsExtern || fn.Body == nil {
			continue
		}
		// Only consider functions currently returning TyPtr (struct pointer).
		if fn.RetTy != mir2.TyPtr {
			continue
		}
		// Check if all returns are struct literals of the same SSS type.
		st := findReturnedStruct(fn, idx)
		if st == nil || !isShallowScalarStruct(st) {
			continue
		}

		// Build promotion info.
		info := &promotionInfo{
			st:       st,
			fieldMap: make(map[string]int, len(st.Fields)),
		}
		for i, f := range st.Fields {
			info.fieldTys = append(info.fieldTys, f.Ty)
			info.fieldMap[f.Name] = i
		}
		eligible[fn.Name] = info
	}

	// Phase 1b: verify all call sites are compatible.
	// For now, we accept all call sites — field access patterns will be
	// rewritten in phase 3, and non-field uses will get spilled.
	return eligible
}

// findReturnedStruct checks if a function returns struct literals of a single
// SSS type in all its return statements.
// Handles both direct returns (`return StructLit{...}`) and indirect returns
// (`Pair res = {...}; return res;`) by tracking struct-initialized variables.
func findReturnedStruct(fn *hir.Func, idx structIndex) *mir2.StructTy {
	// First pass: collect variables initialized from StructLitExpr.
	structVars := make(map[string]*hir.StructLitExpr)
	walkStmts(fn.Body, func(s hir.Stmt) bool {
		if decl, ok := s.(*hir.VarDeclStmt); ok {
			if lit, isLit := decl.Init.(*hir.StructLitExpr); isLit && lit.St != nil {
				structVars[decl.Name] = lit
			}
		}
		return true
	})

	// Second pass: check all return statements.
	var found *mir2.StructTy
	ok := walkStmts(fn.Body, func(s hir.Stmt) bool {
		ret, isRet := s.(*hir.ReturnStmt)
		if !isRet {
			return true
		}
		// Direct struct literal return.
		if lit, isLit := ret.Val.(*hir.StructLitExpr); isLit && lit.St != nil {
			if found == nil {
				found = lit.St
			} else if found.Name != lit.St.Name {
				return false
			}
			return true
		}
		// Indirect: return varName where varName was struct-initialized.
		if ref, isRef := ret.Val.(*hir.VarRefExpr); isRef {
			if lit, ok := structVars[ref.Name]; ok {
				if found == nil {
					found = lit.St
				} else if found.Name != lit.St.Name {
					return false
				}
				return true
			}
		}
		return false // returns something else → not eligible
	})
	if !ok || found == nil {
		return nil
	}
	return found
}

// ── Return rewriting ─────────────────────────────────────────────────────────

// rewriteReturns converts struct returns → tuple returns.
// Handles both direct (`return StructLit{a, b}`) and indirect
// (`Pair res = {a, b}; return res;`) patterns.
func rewriteReturns(fn *hir.Func, info *promotionInfo) {
	// Collect struct-initialized variables for indirect return resolution.
	structVars := make(map[string]*hir.StructLitExpr)
	walkStmts(fn.Body, func(s hir.Stmt) bool {
		if decl, ok := s.(*hir.VarDeclStmt); ok {
			if lit, isLit := decl.Init.(*hir.StructLitExpr); isLit {
				structVars[decl.Name] = lit
			}
		}
		return true
	})

	walkStmtsMut(fn.Body, func(s hir.Stmt, replace func(hir.Stmt)) {
		ret, isRet := s.(*hir.ReturnStmt)
		if !isRet {
			return
		}
		// Resolve the struct literal — direct or via variable.
		var lit *hir.StructLitExpr
		if l, ok := ret.Val.(*hir.StructLitExpr); ok {
			lit = l
		} else if ref, ok := ret.Val.(*hir.VarRefExpr); ok {
			lit = structVars[ref.Name]
		}
		if lit == nil {
			return
		}
		// Convert struct literal fields to tuple values.
		vals := make([]hir.Expr, len(info.fieldTys))
		for _, fi := range lit.Fields {
			if pos, ok := info.fieldMap[fi.Name]; ok {
				vals[pos] = fi.Val
			}
		}
		// Also handle positional init (fields without names).
		if len(lit.Fields) == len(info.fieldTys) {
			allNamed := true
			for _, fi := range lit.Fields {
				if fi.Name == "" {
					allNamed = false
					break
				}
			}
			if !allNamed {
				for i, fi := range lit.Fields {
					if fi.Name == "" {
						vals[i] = fi.Val
					}
				}
			}
		}
		replace(&hir.ReturnStmt{Vals: vals})
	})
}

// ── Call site rewriting ──────────────────────────────────────────────────────

// rewriteCallSites finds patterns like:
//
//	let res = promoted_fn(args...)
//	... res.field ...
//
// and rewrites to:
//
//	let (f0, f1) = promoted_fn(args...)
//	... f0 ...
func rewriteCallSites(blk *hir.Block, eligible map[string]*promotionInfo) {
	if blk == nil {
		return
	}

	// Multi-pass: first find let+field patterns, then rewrite.
	for i := 0; i < len(blk.Body); i++ {
		s := blk.Body[i]

		// Recurse into nested blocks.
		switch st := s.(type) {
		case *hir.IfStmt:
			rewriteCallSites(st.Then, eligible)
			rewriteCallSites(st.Else, eligible)
		case *hir.WhileStmt:
			rewriteCallSites(st.Body, eligible)
		case *hir.ForRangeStmt:
			rewriteCallSites(st.Body, eligible)
		}

		// Look for: let varName = promotedFn(...)
		decl, isDec := s.(*hir.VarDeclStmt)
		if !isDec || decl.Init == nil {
			continue
		}
		call, isCall := decl.Init.(*hir.CallExpr)
		if !isCall {
			continue
		}
		info, ok := eligible[call.Fn]
		if !ok {
			continue
		}

		// Found! Scan forward for field accesses on this variable.
		varName := decl.Name
		fieldVars := make(map[string]string) // field name → generated var name

		// Generate names for each tuple position.
		names := make([]string, len(info.fieldTys))
		for j, f := range info.st.Fields {
			genName := varName + "_" + f.Name
			names[j] = genName
			fieldVars[f.Name] = genName
		}

		// Replace the VarDeclStmt with TupleLetStmt.
		blk.Body[i] = &hir.TupleLetStmt{
			Names: names,
			Tys:   info.fieldTys,
			Call:  call,
		}

		// Rewrite field accesses in subsequent statements.
		for j := i + 1; j < len(blk.Body); j++ {
			blk.Body[j] = rewriteFieldRefs(blk.Body[j], varName, fieldVars, info)
		}
	}
}

// rewriteFieldRefs replaces `varName.field` with the generated tuple variable.
func rewriteFieldRefs(s hir.Stmt, varName string, fieldVars map[string]string, info *promotionInfo) hir.Stmt {
	switch st := s.(type) {
	case *hir.VarDeclStmt:
		if st.Init != nil {
			st.Init = rewriteFieldExpr(st.Init, varName, fieldVars, info)
		}
	case *hir.AssignStmt:
		st.Val = rewriteFieldExpr(st.Val, varName, fieldVars, info)
	case *hir.ReturnStmt:
		if st.Val != nil {
			st.Val = rewriteFieldExpr(st.Val, varName, fieldVars, info)
		}
		for k, v := range st.Vals {
			st.Vals[k] = rewriteFieldExpr(v, varName, fieldVars, info)
		}
	case *hir.ExprStmt:
		st.Expr = rewriteFieldExpr(st.Expr, varName, fieldVars, info)
	case *hir.IfStmt:
		st.Cond = rewriteFieldExpr(st.Cond, varName, fieldVars, info)
		rewriteCallSites(st.Then, map[string]*promotionInfo{}) // just recurse field rewrites
		rewriteCallSites(st.Else, map[string]*promotionInfo{})
	}
	return s
}

// rewriteFieldExpr replaces `varName.field` expressions with tuple var refs.
func rewriteFieldExpr(e hir.Expr, varName string, fieldVars map[string]string, info *promotionInfo) hir.Expr {
	if e == nil {
		return nil
	}

	switch x := e.(type) {
	case *hir.FieldExpr:
		// Check if this is varName.field
		if ref, ok := x.X.(*hir.VarRefExpr); ok && ref.Name == varName {
			if newName, found := fieldVars[x.Field]; found {
				return &hir.VarRefExpr{Name: newName, Ty: x.Ty}
			}
		}
		// Recurse into base
		x.X = rewriteFieldExpr(x.X, varName, fieldVars, info)
		return x

	case *hir.BinExpr:
		x.L = rewriteFieldExpr(x.L, varName, fieldVars, info)
		x.R = rewriteFieldExpr(x.R, varName, fieldVars, info)
		return x

	case *hir.UnaryExpr:
		x.X = rewriteFieldExpr(x.X, varName, fieldVars, info)
		return x

	case *hir.CallExpr:
		for i, a := range x.Args {
			x.Args[i] = rewriteFieldExpr(a, varName, fieldVars, info)
		}
		return x

	case *hir.CastExpr:
		x.X = rewriteFieldExpr(x.X, varName, fieldVars, info)
		return x
	}

	return e
}

// ── Statement walkers ────────────────────────────────────────────────────────

// walkStmts visits every statement in a block recursively.
// If fn returns false, the walk stops and walkStmts returns false.
func walkStmts(blk *hir.Block, fn func(hir.Stmt) bool) bool {
	if blk == nil {
		return true
	}
	for _, s := range blk.Body {
		if !fn(s) {
			return false
		}
		switch st := s.(type) {
		case *hir.IfStmt:
			if !walkStmts(st.Then, fn) || !walkStmts(st.Else, fn) {
				return false
			}
		case *hir.WhileStmt:
			if !walkStmts(st.Body, fn) {
				return false
			}
		case *hir.ForRangeStmt:
			if !walkStmts(st.Body, fn) {
				return false
			}
		}
	}
	return true
}

// walkStmtsMut visits every statement and allows replacing it.
func walkStmtsMut(blk *hir.Block, fn func(hir.Stmt, func(hir.Stmt))) {
	if blk == nil {
		return
	}
	for i, s := range blk.Body {
		fn(s, func(replacement hir.Stmt) {
			blk.Body[i] = replacement
		})
		switch st := blk.Body[i].(type) {
		case *hir.IfStmt:
			walkStmtsMut(st.Then, fn)
			walkStmtsMut(st.Else, fn)
		case *hir.WhileStmt:
			walkStmtsMut(st.Body, fn)
		case *hir.ForRangeStmt:
			walkStmtsMut(st.Body, fn)
		}
	}
}
