package hir

// Struct-Return Promotion Pass (ADR-0025)
//
// Detects C functions returning small structs ("Shallow Scalar Structs") and
// promotes them to HIR tuple returns.  This allows PFCCO (contract optimizer)
// to assign each return slot a register. Actual transfers depend on the caller
// and callee contracts; the representation alone does not prove zero overhead.
//
// Also handles the reverse pattern: struct-argument demotion where a struct
// parameter is immediately destructured in the function body.

import (
	"github.com/minz/minzc/pkg/mir2"
)

// PromoteStructReturns runs the struct↔tuple promotion pass on a HIR module.
// It modifies the module in-place:
//   - Struct-returning functions → tuple-returning functions (RetTys)
//   - Call sites with field access → TupleLetStmt
//   - Out-param functions (void f(..., SSS *out)) → tuple-returning functions
func PromoteStructReturns(m *Module) { promoteStructReturns(m, false) }

// OptimizeCAggregateABI selects aggregate contracts only after source modules
// are linked. The origin marker keeps unrelated Nanz pointer-return functions
// out of C-specific eligibility heuristics.
func OptimizeCAggregateABI(m *Module) {
	for _, fn := range m.Funcs {
		if fn.CValueABI {
			promoteStructReturns(m, true)
			return
		}
	}
}

func IsShallowScalarStruct(st *mir2.StructTy) bool { return isShallowScalarStruct(st) }

func promoteStructReturns(m *Module, onlyC bool) {
	changed := scalarizeAggregateParams(m, onlyC)
	defer func() {
		if changed {
			for _, fn := range m.Funcs {
				scalarizeAggregateLocals(fn.Body)
			}
		}
	}()
	idx := buildStructIndex(m)
	if len(idx) == 0 {
		return
	}

	// Phase 0: out-param promotion — void f(..., SSS *out) → tuple return.
	outParamEligible := findOutParamFunctions(m, idx, onlyC)
	changed = changed || len(outParamEligible) > 0
	for _, fn := range m.Funcs {
		info, ok := outParamEligible[fn.Name]
		if !ok {
			continue
		}
		rewriteOutParamFunc(fn, info)
	}
	// Rewrite call sites for out-param promoted functions.
	for _, fn := range m.Funcs {
		rewriteOutParamCallSites(fn.Body, outParamEligible)
	}

	// Phase 1: identify struct-returning functions eligible for promotion.
	eligible := findEligibleFunctions(m, idx, onlyC)
	changed = changed || len(eligible) > 0
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

	for _, fn := range m.Funcs {
		scalarizeAggregateLocals(fn.Body)
	}

	// Phase 4: eliminate dead struct VarDecls left over from promotion.
	// After rewriting returns to tuples, the StructLitExpr VarDecls are
	// dead code — removing them prevents unnecessary stack allocation.
	for _, fn := range m.Funcs {
		if _, ok := eligible[fn.Name]; ok {
			eliminateDeadStructDecls(fn.Body)
		}
	}
}

// ── Struct index ─────────────────────────────────────────────────────────────

// structIndex keeps declarations in source order. Ambiguous field-name
// matches retain the first declaration, the usual small-map traversal choice.
type structIndex []*mir2.StructTy

func buildStructIndex(m *Module) structIndex {
	idx := make(structIndex, 0, len(m.Structs))
	positions := make(map[string]int)
	for _, st := range m.Structs {
		if pos, ok := positions[st.Name]; ok {
			idx[pos] = st // preserve the old index's last definition for this name
		} else {
			positions[st.Name] = len(idx)
			idx = append(idx, st)
		}
	}
	return idx
}

// ── SSS detection ────────────────────────────────────────────────────────────

// isShallowScalarStruct checks the SSS criteria from ADR-0025:
//   - All fields are scalar (u8, u16, i8, i16, bool)
//   - No nested structs, no arrays
//   - Total size ≤ 4 bytes (fits Z80 register file)
func isShallowScalarStruct(st *mir2.StructTy) bool {
	if st == nil || mir2.HasVolatileStorage(st) || st.IsUnion || len(st.FieldOffsets) != 0 || len(st.Fields) == 0 || len(st.Fields) > 4 {
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
	st       *mir2.StructTy // the struct type being returned
	fieldTys []mir2.Ty      // field types (== RetTys after promotion)
	fieldMap map[string]int // field name → position in tuple
}

func findEligibleFunctions(m *Module, idx structIndex, onlyC bool) map[string]*promotionInfo {
	eligible := make(map[string]*promotionInfo)

	for _, fn := range m.Funcs {
		if onlyC && !fn.CValueABI {
			continue
		}
		if fn.IsExtern || fn.Body == nil {
			continue
		}
		// Only consider functions currently returning TyPtr (struct pointer).
		_, typedAggregate := mir2.BaseOf(fn.RetTy).(*mir2.StructTy)
		if fn.RetTy != mir2.TyPtr && (!fn.CValueABI || !typedAggregate) {
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

	// A tuple return may replace only a known, materialized direct call.
	// Function pointers and expression-level uses retain the memory ABI until
	// the corresponding adapters exist; never publish a mismatched signature.
	for name := range eligible {
		safe := true
		allowed := map[*CallExpr]bool{}
		for _, fn := range m.Funcs {
			visitAggregateStmts(fn.Body, func(s Stmt) {
				if d, ok := s.(*VarDeclStmt); ok {
					if c, ok := d.Init.(*CallExpr); ok && c.Fn == name {
						allowed[c] = true
					}
				}
			})
		}
		for _, fn := range m.Funcs {
			if fn.Body == nil {
				continue
			}
			if !inspectAggregateStmt(fn.Body, func(e Expr) (Expr, bool) {
				if c, ok := e.(*CallExpr); ok && c.Fn == name && !allowed[c] {
					safe = false
				}
				if a, ok := e.(*AddrOfExpr); ok && a.Sym == name {
					safe = false
				}
				if r, ok := e.(*VarRefExpr); ok && r.Name == name {
					safe = false
				}
				return e, true
			}) {
				safe = false
			}
		}
		for _, g := range m.Globals {
			for _, r := range g.Relocs {
				if r.Symbol == name {
					safe = false
				}
			}
			for _, symbol := range g.VtableSyms {
				if symbol == name {
					safe = false
				}
			}
		}
		if !safe {
			delete(eligible, name)
		}
	}
	return eligible
}

// findReturnedStruct checks if a function returns struct literals of a single
// SSS type in all its return statements.
// Handles three patterns:
//   - Direct: `return StructLit{...}`
//   - Indirect via brace init: `Pair res = {a, b}; return res;`
//   - Pointer return via field assign: `res.q = a; res.r = b; return &res;`
func findReturnedStruct(fn *Func, idx structIndex) *mir2.StructTy {
	// Also track variables built via field assignments (pointer return pattern).
	fieldAssigned := collectFieldAssigned(fn.Body, idx)

	// Walk all returns and check they all return the same SSS struct type.
	// Uses block-local resolution to avoid cross-block variable shadowing.
	var found *mir2.StructTy
	ok := findReturnedStructInBlock(fn.Body, idx, fieldAssigned, &found, nil)
	if !ok || found == nil {
		return nil
	}
	return found
}

// The declared object type remains authoritative when its fields are assigned
// after declaration. An initializer is not required for a C aggregate value.
func aggregateDeclarationType(d *VarDeclStmt) *mir2.StructTy {
	if d.Volatile || d.Ty != nil && mir2.HasVolatileStorage(d.Ty) {
		return nil
	}
	if lit, ok := d.Init.(*StructLitExpr); ok && lit.St != nil {
		return lit.St
	}
	if d.Ty != nil {
		if st, ok := mir2.BaseOf(d.Ty).(*mir2.StructTy); ok {
			return st
		}
	}
	return nil
}

// findReturnedStructInBlock checks returns in a block using scoped struct var tracking.
// parentVars provides declarations from enclosing blocks; local declarations shadow them.
func findReturnedStructInBlock(blk *Block, idx structIndex, fieldAssigned map[string]*mir2.StructTy, found **mir2.StructTy, parentVars map[string]*mir2.StructTy) bool {
	if blk == nil {
		return true
	}

	// Build scoped vars: parent + local (local shadows parent).
	scopeVars := make(map[string]*mir2.StructTy, len(parentVars))
	for k, v := range parentVars {
		scopeVars[k] = v
	}
	for _, s := range blk.Body {
		if decl, ok := s.(*VarDeclStmt); ok {
			delete(scopeVars, decl.Name) // a non-aggregate local also shadows
			if st := aggregateDeclarationType(decl); st != nil {
				scopeVars[decl.Name] = st
			}
		}
	}

	for _, s := range blk.Body {
		// Recurse into nested blocks with current scope.
		switch st := s.(type) {
		case *Block:
			if !findReturnedStructInBlock(st, idx, fieldAssigned, found, scopeVars) {
				return false
			}
		case *IfStmt:
			if !findReturnedStructInBlock(st.Then, idx, fieldAssigned, found, scopeVars) {
				return false
			}
			if !findReturnedStructInBlock(st.Else, idx, fieldAssigned, found, scopeVars) {
				return false
			}
		case *WhileStmt:
			if !findReturnedStructInBlock(st.Body, idx, fieldAssigned, found, scopeVars) {
				return false
			}
		case *ForRangeStmt:
			if !findReturnedStructInBlock(st.Body, idx, fieldAssigned, found, scopeVars) {
				return false
			}
		}

		ret, isRet := s.(*ReturnStmt)
		if !isRet {
			continue
		}

		var st *mir2.StructTy

		// Direct struct literal return.
		if lit, isLit := ret.Val.(*StructLitExpr); isLit && lit.St != nil {
			st = lit.St
		}

		// Indirect: return varName — use scoped resolution.
		if st == nil {
			if ref, isRef := ret.Val.(*VarRefExpr); isRef {
				if declared, ok := scopeVars[ref.Name]; ok {
					st = declared
				}
			}
		}

		// Pointer return: return &varName where varName was field-assigned.
		if st == nil {
			if unary, isUnary := ret.Val.(*UnaryExpr); isUnary && unary.Op == "&" {
				if ref, isRef := unary.X.(*VarRefExpr); isRef {
					st = fieldAssigned[ref.Name]
				}
			}
		}

		if st == nil {
			return false // returns something else → not eligible
		}

		if *found == nil {
			*found = st
		} else if (*found).Name != st.Name {
			return false
		}
	}
	return true
}

// collectFieldAssigned finds local variables that are fully populated via
// field assignments (e.g. `res.q = expr; res.r = expr;`) and matches them
// to SSS struct types.
func collectFieldAssigned(blk *Block, idx structIndex) map[string]*mir2.StructTy {
	if blk == nil {
		return nil
	}
	// Collect field writes per variable: varName → set of field names.
	writes := make(map[string]map[string]bool)
	walkStmts(blk, func(s Stmt) bool {
		if assign, ok := s.(*AssignStmt); ok {
			if fe, ok := assign.Target.(*FieldExpr); ok {
				if ref, ok := fe.X.(*VarRefExpr); ok {
					if writes[ref.Name] == nil {
						writes[ref.Name] = make(map[string]bool)
					}
					writes[ref.Name][fe.Field] = true
				}
			}
		}
		return true
	})
	// Match against SSS structs.
	result := make(map[string]*mir2.StructTy)
	for varName, fieldSet := range writes {
		for _, st := range idx {
			if !isShallowScalarStruct(st) {
				continue
			}
			if len(fieldSet) != len(st.Fields) {
				continue
			}
			allMatch := true
			for _, f := range st.Fields {
				if !fieldSet[f.Name] {
					allMatch = false
					break
				}
			}
			if allMatch {
				result[varName] = st
				break
			}
		}
	}
	return result
}

// ── Return rewriting ─────────────────────────────────────────────────────────

// rewriteReturns converts struct returns → tuple returns.
// Handles three patterns:
//   - Direct: `return StructLit{a, b}` → `return (a, b)`
//   - Indirect via brace init: `Pair res = {a, b}; return res;` → `return (a, b)`
//   - Pointer return: `res.q = a; res.r = b; return &res;` → `return (a, b)`
func rewriteReturns(fn *Func, info *promotionInfo) {
	// Collect field assignments for pointer-return pattern.
	fieldAssigns := collectFieldAssigns(fn.Body)

	// Rewrite returns block-by-block with scoped variable resolution.
	// Different blocks may define the same variable name with different
	// values (e.g. if/else branches), so we use lexical scoping.
	pointerReturns := make(map[string]bool)
	rewriteReturnsInBlock(fn.Body, info, fieldAssigns, nil, pointerReturns)

	// For pointer-return pattern: remove the field-assign statements that
	// were consumed into the tuple return.
	removePtrReturnAssigns(fn.Body, info, pointerReturns)
}

// rewriteReturnsInBlock processes one block with scoped variable resolution.
// parentVars provides declarations from enclosing blocks.
func rewriteReturnsInBlock(blk *Block, info *promotionInfo, fieldAssigns map[string]map[string]Expr, parentVars map[string]*mir2.StructTy, pointerReturns map[string]bool) {
	if blk == nil {
		return
	}

	// Build scoped vars: parent + local (local shadows parent).
	scopeVars := make(map[string]*mir2.StructTy)
	for k, v := range parentVars {
		scopeVars[k] = v
	}
	for _, s := range blk.Body {
		if decl, ok := s.(*VarDeclStmt); ok {
			delete(scopeVars, decl.Name) // a non-aggregate local also shadows
			if st := aggregateDeclarationType(decl); st != nil {
				scopeVars[decl.Name] = st
			}
		}
	}

	for i, s := range blk.Body {
		// Recurse into nested blocks with current scope.
		switch st := s.(type) {
		case *Block:
			rewriteReturnsInBlock(st, info, fieldAssigns, scopeVars, pointerReturns)
		case *IfStmt:
			rewriteReturnsInBlock(st.Then, info, fieldAssigns, scopeVars, pointerReturns)
			rewriteReturnsInBlock(st.Else, info, fieldAssigns, scopeVars, pointerReturns)
		case *WhileStmt:
			rewriteReturnsInBlock(st.Body, info, fieldAssigns, scopeVars, pointerReturns)
		case *ForRangeStmt:
			rewriteReturnsInBlock(st.Body, info, fieldAssigns, scopeVars, pointerReturns)
		}

		ret, isRet := s.(*ReturnStmt)
		if !isRet {
			continue
		}

		// Pattern 1: direct StructLitExpr.
		if l, ok := ret.Val.(*StructLitExpr); ok {
			blk.Body[i] = &ReturnStmt{Vals: structLitToTuple(l, info)}
			continue
		}

		// Pattern 2: indirect via variable — use scoped resolution.
		if ref, ok := ret.Val.(*VarRefExpr); ok {
			if _, found := scopeVars[ref.Name]; found {
				// The object may have changed since its initializer. Read its
				// current fields; replaying initializer expressions loses writes
				// and can repeat their side effects.
				vals := make([]Expr, len(info.fieldTys))
				for j, field := range info.st.Fields {
					vals[j] = &FieldExpr{X: Var(ref.Name, mir2.TyPtr), Field: field.Name, Offset: info.st.ByteOffset(j), Ty: info.fieldTys[j]}
				}
				blk.Body[i] = &ReturnStmt{Vals: vals}
				continue
			}
		}

		// Pattern 3: return &varName where varName was field-assigned.
		if unary, isUnary := ret.Val.(*UnaryExpr); isUnary && unary.Op == "&" {
			if ref, isRef := unary.X.(*VarRefExpr); isRef {
				if assigns, ok := fieldAssigns[ref.Name]; ok {
					vals := make([]Expr, len(info.fieldTys))
					for field, expr := range assigns {
						if pos, ok := info.fieldMap[field]; ok {
							vals[pos] = expr
						}
					}
					blk.Body[i] = &ReturnStmt{Vals: vals}
					pointerReturns[ref.Name] = true
				}
			}
		}
	}
}

// structLitToTuple converts a StructLitExpr to tuple values using promotionInfo.
func structLitToTuple(lit *StructLitExpr, info *promotionInfo) []Expr {
	vals := make([]Expr, len(info.fieldTys))
	for _, fi := range lit.Fields {
		if pos, ok := info.fieldMap[fi.Name]; ok {
			vals[pos] = fi.Val
		}
	}
	// Handle positional init (fields without names).
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
	return vals
}

// collectFieldAssigns collects `varName.field = expr` assignments per variable.
func collectFieldAssigns(blk *Block) map[string]map[string]Expr {
	result := make(map[string]map[string]Expr)
	if blk == nil {
		return result
	}
	for _, s := range blk.Body {
		if assign, ok := s.(*AssignStmt); ok {
			if fe, ok := assign.Target.(*FieldExpr); ok {
				if ref, ok := fe.X.(*VarRefExpr); ok {
					if result[ref.Name] == nil {
						result[ref.Name] = make(map[string]Expr)
					}
					result[ref.Name][fe.Field] = assign.Val
				}
			}
		}
	}
	return result
}

// removePtrReturnAssigns removes field-assign statements for variables that
// were promoted into tuple returns (pointer-return pattern).
func removePtrReturnAssigns(blk *Block, info *promotionInfo, promoted map[string]bool) {
	if blk == nil {
		return
	}
	// Only the pointer-return rewrite consumes assignment expressions. Value
	// returns read current fields and must retain their declarations and writes.

	if len(promoted) == 0 {
		return
	}

	// Remove field-assign statements and var decls for promoted variables.
	var filtered []Stmt
	for _, s := range blk.Body {
		if assign, ok := s.(*AssignStmt); ok {
			if fe, ok := assign.Target.(*FieldExpr); ok {
				if ref, ok := fe.X.(*VarRefExpr); ok {
					if promoted[ref.Name] {
						if _, isPromotedField := info.fieldMap[fe.Field]; isPromotedField {
							continue // remove
						}
					}
				}
			}
		}
		// Also remove the variable declaration for the promoted struct.
		if decl, ok := s.(*VarDeclStmt); ok {
			if promoted[decl.Name] {
				continue // remove
			}
		}
		filtered = append(filtered, s)
	}
	blk.Body = filtered
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
func rewriteCallSites(blk *Block, eligible map[string]*promotionInfo) {
	if blk == nil {
		return
	}

	// Multi-pass: first find let+field patterns, then rewrite.
	for i := 0; i < len(blk.Body); i++ {
		s := blk.Body[i]

		// Recurse into nested blocks.
		switch st := s.(type) {
		case *Block:
			rewriteCallSites(st, eligible)
		case *IfStmt:
			rewriteCallSites(st.Then, eligible)
			rewriteCallSites(st.Else, eligible)
		case *WhileStmt:
			rewriteCallSites(st.Body, eligible)
		case *ForRangeStmt:
			rewriteCallSites(st.Body, eligible)
		}

		// Look for: let varName = promotedFn(...)
		decl, isDec := s.(*VarDeclStmt)
		if !isDec || decl.Init == nil {
			continue
		}
		call, isCall := decl.Init.(*CallExpr)
		if !isCall {
			continue
		}
		info, ok := eligible[call.Fn]
		if !ok {
			continue
		}

		// Found! Scan forward for field accesses on this variable.
		varName := decl.Name

		// Generate names for each tuple position.
		names := make([]string, len(info.fieldTys))
		for j, f := range info.st.Fields {
			genName := varName + "_" + f.Name
			used := map[string]bool{}
			visitAggregateStmts(blk, func(s Stmt) {
				if d, ok := s.(*VarDeclStmt); ok {
					used[d.Name] = true
				}
				if t, ok := s.(*TupleLetStmt); ok {
					for _, n := range t.Names {
						used[n] = true
					}
				}
			})
			inspectAggregateStmt(blk, func(e Expr) (Expr, bool) {
				if r, ok := e.(*VarRefExpr); ok {
					used[r.Name] = true
				}
				return e, true
			})
			for used[genName] {
				genName += "_"
			}
			names[j] = genName
		}

		// Keep a materialized by-value object until escape analysis proves it
		// can be removed. Whole-object arguments and address uses need storage;
		// blindly deleting the declaration left dangling references or stale
		// tuple reads after memory mutation.
		call.Ty = mir2.TyVoid
		tuple := &TupleLetStmt{Names: names, Tys: info.fieldTys, Call: call}
		lit := &StructLitExpr{St: info.st}
		for j, field := range info.st.Fields {
			lit.Fields = append(lit.Fields, FieldInit{Name: field.Name, Val: Var(names[j], info.fieldTys[j])})
		}
		decl.Init = lit
		decl.Ty = info.st
		blk.Body = append(blk.Body[:i], append([]Stmt{tuple, decl}, blk.Body[i+1:]...)...)
		i++

	}
}

// rewriteFieldRefs replaces `varName.field` with the generated tuple variable.
func rewriteFieldRefs(s Stmt, varName string, fieldVars map[string]string, info *promotionInfo) Stmt {
	switch st := s.(type) {
	case *VarDeclStmt:
		if st.Init != nil {
			st.Init = rewriteFieldExpr(st.Init, varName, fieldVars, info)
		}
	case *AssignStmt:
		st.Val = rewriteFieldExpr(st.Val, varName, fieldVars, info)
	case *ReturnStmt:
		if st.Val != nil {
			st.Val = rewriteFieldExpr(st.Val, varName, fieldVars, info)
		}
		for k, v := range st.Vals {
			st.Vals[k] = rewriteFieldExpr(v, varName, fieldVars, info)
		}
	case *ExprStmt:
		st.Expr = rewriteFieldExpr(st.Expr, varName, fieldVars, info)
	case *IfStmt:
		st.Cond = rewriteFieldExpr(st.Cond, varName, fieldVars, info)
		rewriteCallSites(st.Then, map[string]*promotionInfo{}) // just recurse field rewrites
		rewriteCallSites(st.Else, map[string]*promotionInfo{})
	}
	return s
}

// rewriteFieldExpr replaces `varName.field` expressions with tuple var refs.
func rewriteFieldExpr(e Expr, varName string, fieldVars map[string]string, info *promotionInfo) Expr {
	if e == nil {
		return nil
	}

	switch x := e.(type) {
	case *FieldExpr:
		// Check if this is varName.field
		if ref, ok := x.X.(*VarRefExpr); ok && ref.Name == varName {
			if newName, found := fieldVars[x.Field]; found {
				return &VarRefExpr{Name: newName, Ty: x.Ty}
			}
		}
		// Recurse into base
		x.X = rewriteFieldExpr(x.X, varName, fieldVars, info)
		return x

	case *BinExpr:
		x.L = rewriteFieldExpr(x.L, varName, fieldVars, info)
		x.R = rewriteFieldExpr(x.R, varName, fieldVars, info)
		return x

	case *UnaryExpr:
		x.X = rewriteFieldExpr(x.X, varName, fieldVars, info)
		return x

	case *CallExpr:
		for i, a := range x.Args {
			x.Args[i] = rewriteFieldExpr(a, varName, fieldVars, info)
		}
		return x

	case *CastExpr:
		x.X = rewriteFieldExpr(x.X, varName, fieldVars, info)
		return x
	}

	return e
}

// ── Out-param promotion ─────────────────────────────────────────────────────

// outParamInfo holds analysis for a void function with an out-param pointer.
type outParamInfo struct {
	st         *mir2.StructTy // the struct type being written through the pointer
	fieldTys   []mir2.Ty      // field types (== RetTys after promotion)
	fieldMap   map[string]int // field name → position in tuple
	paramName  string         // name of the out-param
	paramIndex int            // position in Params slice
}

// findOutParamFunctions detects void functions where the last TyPtr param is
// used exclusively via `param->field = expr` assignments, covering all fields
// of a Shallow Scalar Struct.
func findOutParamFunctions(m *Module, idx structIndex, onlyC bool) map[string]*outParamInfo {
	result := make(map[string]*outParamInfo)

	for _, fn := range m.Funcs {
		if onlyC && !fn.CValueABI {
			continue
		}
		if fn.IsExtern || fn.Body == nil {
			continue
		}
		if fn.RetTy != mir2.TyVoid || len(fn.RetTys) > 0 {
			continue // not void
		}
		if len(fn.Params) == 0 {
			continue
		}

		// Find the last TyPtr parameter — candidate for out-param.
		lastIdx := len(fn.Params) - 1
		if fn.Params[lastIdx].Ty != mir2.TyPtr {
			continue
		}
		paramName := fn.Params[lastIdx].Name

		// Skip ObjC "self" parameter — not an out-param.
		if paramName == "self" {
			continue
		}

		// Analyze body: all uses of paramName must be `paramName.field = expr`
		// AND paramName must not appear in the assigned value (pure write-only).
		fields := make(map[string]Expr) // field name → last assigned value
		allFieldWrites := true

		walkStmts(fn.Body, func(s Stmt) bool {
			// Check for field-write pattern: paramName.field = expr
			if assign, ok := s.(*AssignStmt); ok {
				if fe, ok := assign.Target.(*FieldExpr); ok {
					if ref, ok := fe.X.(*VarRefExpr); ok && ref.Name == paramName {
						// Also check the value doesn't read from paramName.
						if exprUsesVar(assign.Val, paramName) {
							allFieldWrites = false
							return false
						}
						fields[fe.Field] = assign.Val
						return true
					}
				}
			}
			// Check for any other use of paramName in this statement.
			if usesVar(s, paramName) {
				allFieldWrites = false
				return false
			}
			return true
		})

		if !allFieldWrites {
			continue
		}

		// Find which struct this corresponds to — match by field names.
		var matched *mir2.StructTy
		for _, st := range idx {
			if !isShallowScalarStruct(st) {
				continue
			}
			if len(fields) != len(st.Fields) {
				continue
			}
			allMatch := true
			for _, f := range st.Fields {
				if _, ok := fields[f.Name]; !ok {
					allMatch = false
					break
				}
			}
			if allMatch {
				matched = st
				break
			}
		}
		if matched == nil {
			continue
		}

		info := &outParamInfo{
			st:         matched,
			fieldMap:   make(map[string]int, len(matched.Fields)),
			paramName:  paramName,
			paramIndex: lastIdx,
		}
		for i, f := range matched.Fields {
			info.fieldTys = append(info.fieldTys, f.Ty)
			info.fieldMap[f.Name] = i
		}
		result[fn.Name] = info
	}
	return result
}

// usesVar checks if a statement references a variable name (excluding field
// writes which are handled separately).
func usesVar(s Stmt, name string) bool {
	switch st := s.(type) {
	case *AssignStmt:
		// Field writes on the target are OK (handled by caller).
		// Check if Val uses the var.
		return exprUsesVar(st.Val, name)
	case *VarDeclStmt:
		return exprUsesVar(st.Init, name)
	case *ReturnStmt:
		if exprUsesVar(st.Val, name) {
			return true
		}
		for _, v := range st.Vals {
			if exprUsesVar(v, name) {
				return true
			}
		}
	case *ExprStmt:
		return exprUsesVar(st.Expr, name)
	case *IfStmt:
		return exprUsesVar(st.Cond, name)
	}
	return false
}

// exprUsesVar checks if an expression references a variable by name.
func exprUsesVar(e Expr, name string) bool {
	if e == nil {
		return false
	}
	switch x := e.(type) {
	case *VarRefExpr:
		return x.Name == name
	case *FieldExpr:
		return exprUsesVar(x.X, name)
	case *BinExpr:
		return exprUsesVar(x.L, name) || exprUsesVar(x.R, name)
	case *UnaryExpr:
		return exprUsesVar(x.X, name)
	case *CallExpr:
		for _, a := range x.Args {
			if exprUsesVar(a, name) {
				return true
			}
		}
	case *CastExpr:
		return exprUsesVar(x.X, name)
	}
	return false
}

// rewriteOutParamFunc transforms a void function with out-param to tuple return.
//
//	void divmod(u8 a, u8 b, DivResult *out) { out->q = a/b; out->r = a%b; }
//	→ (u8, u8) divmod(u8 a, u8 b) { return (a/b, a%b); }
func rewriteOutParamFunc(fn *Func, info *outParamInfo) {
	// Remove the out-param from signature.
	fn.Params = fn.Params[:info.paramIndex]

	// Set tuple return types.
	fn.RetTys = info.fieldTys
	fn.RetTy = mir2.TyVoid

	// Collect field assignments in order, then replace with a tuple return.
	vals := make([]Expr, len(info.fieldTys))

	// Walk body and collect field → expr assignments, remove them.
	// Also remove void return statements (they precede the tuple return we'll add).
	var newBody []Stmt
	for _, s := range fn.Body.Body {
		if assign, ok := s.(*AssignStmt); ok {
			if fe, ok := assign.Target.(*FieldExpr); ok {
				if ref, ok := fe.X.(*VarRefExpr); ok && ref.Name == info.paramName {
					if pos, found := info.fieldMap[fe.Field]; found {
						vals[pos] = assign.Val
						continue // remove this statement
					}
				}
			}
		}
		// Remove void returns — they'll be replaced by the tuple return.
		if ret, ok := s.(*ReturnStmt); ok && ret.Val == nil && len(ret.Vals) == 0 {
			continue
		}
		newBody = append(newBody, s)
	}

	// Append tuple return.
	newBody = append(newBody, &ReturnStmt{Vals: vals})
	fn.Body.Body = newBody
}

// rewriteOutParamCallSites transforms call sites:
//
//	DivResult res; divmod(a, b, &res); use(res.q);
//	→ let (res_q, res_r) = divmod(a, b); use(res_q);
//
// Also handles the pattern where the struct var is declared with init,
// then passed as out-param: DivResult res = {}; f(x, &res); use(res.field);
func rewriteOutParamCallSites(blk *Block, eligible map[string]*outParamInfo) {
	if blk == nil || len(eligible) == 0 {
		return
	}

	for i := 0; i < len(blk.Body); i++ {
		s := blk.Body[i]

		// Recurse into nested blocks.
		switch st := s.(type) {
		case *IfStmt:
			rewriteOutParamCallSites(st.Then, eligible)
			rewriteOutParamCallSites(st.Else, eligible)
		case *WhileStmt:
			rewriteOutParamCallSites(st.Body, eligible)
		case *ForRangeStmt:
			rewriteOutParamCallSites(st.Body, eligible)
		}

		// Look for ExprStmt containing a call to a promoted function.
		exprSt, isExpr := s.(*ExprStmt)
		if !isExpr {
			continue
		}
		call, isCall := exprSt.Expr.(*CallExpr)
		if !isCall {
			continue
		}
		info, ok := eligible[call.Fn]
		if !ok {
			continue
		}
		// Check the last arg is an address-of a variable.
		if len(call.Args) <= info.paramIndex {
			continue
		}
		lastArg := call.Args[info.paramIndex]

		// Check for AddrOfExpr (C89 frontend emits this for &var).
		if addrExpr, ok := lastArg.(*AddrOfExpr); ok {
			varName := addrExpr.Sym
			fieldVars := make(map[string]string)
			names := make([]string, len(info.fieldTys))
			for j, f := range info.st.Fields {
				genName := varName + "_" + f.Name
				names[j] = genName
				fieldVars[f.Name] = genName
			}
			call.Args = call.Args[:info.paramIndex]
			blk.Body[i] = &TupleLetStmt{
				Names: names,
				Tys:   info.fieldTys,
				Call:  call,
			}
			// Remove preceding VarDeclStmt for the struct variable if it exists.
			if i > 0 {
				if decl, ok := blk.Body[i-1].(*VarDeclStmt); ok && decl.Name == varName {
					blk.Body = append(blk.Body[:i-1], blk.Body[i:]...)
					i--
				}
			}
			pInfo := &promotionInfo{st: info.st, fieldTys: info.fieldTys, fieldMap: info.fieldMap}
			for j := i + 1; j < len(blk.Body); j++ {
				blk.Body[j] = rewriteFieldRefs(blk.Body[j], varName, fieldVars, pInfo)
			}
			continue
		}

		addrOf, isAddr := lastArg.(*UnaryExpr)
		if !isAddr || addrOf.Op != "&" {
			// Also check for plain VarRefExpr (already a pointer).
			ref, isRef := lastArg.(*VarRefExpr)
			if !isRef {
				continue
			}
			// The variable name is the struct we need to rewrite field accesses for.
			varName := ref.Name
			fieldVars := make(map[string]string)
			names := make([]string, len(info.fieldTys))
			for j, f := range info.st.Fields {
				genName := varName + "_" + f.Name
				names[j] = genName
				fieldVars[f.Name] = genName
			}

			// Strip the out-param from the call.
			call.Args = call.Args[:info.paramIndex]

			// Replace ExprStmt with TupleLetStmt.
			blk.Body[i] = &TupleLetStmt{
				Names: names,
				Tys:   info.fieldTys,
				Call:  call,
			}

			// Rewrite field accesses in subsequent statements.
			pInfo := &promotionInfo{st: info.st, fieldTys: info.fieldTys, fieldMap: info.fieldMap}
			for j := i + 1; j < len(blk.Body); j++ {
				blk.Body[j] = rewriteFieldRefs(blk.Body[j], varName, fieldVars, pInfo)
			}
			continue
		}

		// Address-of pattern: &varName
		ref, isRef := addrOf.X.(*VarRefExpr)
		if !isRef {
			continue
		}
		varName := ref.Name

		fieldVars := make(map[string]string)
		names := make([]string, len(info.fieldTys))
		for j, f := range info.st.Fields {
			genName := varName + "_" + f.Name
			names[j] = genName
			fieldVars[f.Name] = genName
		}

		// Strip the out-param from the call.
		call.Args = call.Args[:info.paramIndex]

		// Replace ExprStmt with TupleLetStmt.
		blk.Body[i] = &TupleLetStmt{
			Names: names,
			Tys:   info.fieldTys,
			Call:  call,
		}

		// Remove the preceding VarDeclStmt for the struct variable if it exists.
		if i > 0 {
			if decl, ok := blk.Body[i-1].(*VarDeclStmt); ok && decl.Name == varName {
				blk.Body = append(blk.Body[:i-1], blk.Body[i:]...)
				i-- // adjust index after removal
			}
		}

		// Rewrite field accesses in subsequent statements.
		pInfo := &promotionInfo{st: info.st, fieldTys: info.fieldTys, fieldMap: info.fieldMap}
		for j := i + 1; j < len(blk.Body); j++ {
			blk.Body[j] = rewriteFieldRefs(blk.Body[j], varName, fieldVars, pInfo)
		}
	}
}

// ── Dead struct decl elimination ─────────────────────────────────────────────

// eliminateDeadStructDecls removes VarDeclStmt with StructLitExpr init where
// the variable is not referenced elsewhere in the block. This cleans up dead
// code left after struct→tuple promotion.
func eliminateDeadStructDecls(blk *Block) {
	if blk == nil {
		return
	}

	// Recurse first.
	for _, s := range blk.Body {
		switch st := s.(type) {
		case *Block:
			eliminateDeadStructDecls(st)
		case *IfStmt:
			eliminateDeadStructDecls(st.Then)
			eliminateDeadStructDecls(st.Else)
		case *WhileStmt:
			eliminateDeadStructDecls(st.Body)
		case *ForRangeStmt:
			eliminateDeadStructDecls(st.Body)
		}
	}

	// Collect names of struct-initialized variables.
	structDecls := make(map[string]bool)
	for _, s := range blk.Body {
		if decl, ok := s.(*VarDeclStmt); ok {
			if _, isLit := decl.Init.(*StructLitExpr); isLit {
				structDecls[decl.Name] = true
			}
		}
	}
	if len(structDecls) == 0 {
		return
	}

	// Check which are still referenced in the remaining body.
	used := make(map[string]bool)
	for _, s := range blk.Body {
		// Skip the VarDeclStmt itself.
		if decl, ok := s.(*VarDeclStmt); ok {
			if structDecls[decl.Name] {
				continue
			}
		}
		for name := range structDecls {
			if !used[name] && stmtUsesVar(s, name) {
				used[name] = true
			}
		}
	}

	// Remove unused struct decls.
	var filtered []Stmt
	for _, s := range blk.Body {
		if decl, ok := s.(*VarDeclStmt); ok {
			if structDecls[decl.Name] && !used[decl.Name] {
				continue // dead — remove
			}
		}
		filtered = append(filtered, s)
	}
	blk.Body = filtered
}

// stmtUsesVar checks if any expression in a statement references a variable.
func stmtUsesVar(s Stmt, name string) bool {
	switch st := s.(type) {
	case *Block:
		return blockUsesVar(st, name)
	case *VarDeclStmt:
		return exprUsesVar(st.Init, name)
	case *AssignStmt:
		return exprUsesVar(st.Target, name) || exprUsesVar(st.Val, name)
	case *ReturnStmt:
		if exprUsesVar(st.Val, name) {
			return true
		}
		for _, v := range st.Vals {
			if exprUsesVar(v, name) {
				return true
			}
		}
	case *ExprStmt:
		return exprUsesVar(st.Expr, name)
	case *IfStmt:
		if exprUsesVar(st.Cond, name) {
			return true
		}
		if blockUsesVar(st.Then, name) || blockUsesVar(st.Else, name) {
			return true
		}
	case *WhileStmt:
		if exprUsesVar(st.Cond, name) {
			return true
		}
		return blockUsesVar(st.Body, name)
	case *ForRangeStmt:
		return blockUsesVar(st.Body, name)
	case *TupleLetStmt:
		return exprUsesVar(st.Call, name)
	}
	return false
}

// blockUsesVar checks if any statement in a block references a variable.
func blockUsesVar(blk *Block, name string) bool {
	if blk == nil {
		return false
	}
	for _, s := range blk.Body {
		if stmtUsesVar(s, name) {
			return true
		}
	}
	return false
}

// ── Statement walkers ────────────────────────────────────────────────────────

// walkStmts visits every statement in a block recursively.
// If fn returns false, the walk stops and walkStmts returns false.
func walkStmts(blk *Block, fn func(Stmt) bool) bool {
	if blk == nil {
		return true
	}
	for _, s := range blk.Body {
		if !fn(s) {
			return false
		}
		switch st := s.(type) {
		case *IfStmt:
			if !walkStmts(st.Then, fn) || !walkStmts(st.Else, fn) {
				return false
			}
		case *WhileStmt:
			if !walkStmts(st.Body, fn) {
				return false
			}
		case *ForRangeStmt:
			if !walkStmts(st.Body, fn) {
				return false
			}
		}
	}
	return true
}
