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
//   - Out-param functions (void f(..., SSS *out)) → tuple-returning functions
func PromoteStructReturns(m *hir.Module) {
	idx := buildStructIndex(m)
	if len(idx) == 0 {
		return
	}

	// Phase 0: out-param promotion — void f(..., SSS *out) → tuple return.
	outParamEligible := findOutParamFunctions(m, idx)
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
// Handles three patterns:
//   - Direct: `return StructLit{...}`
//   - Indirect via brace init: `Pair res = {a, b}; return res;`
//   - Pointer return via field assign: `res.q = a; res.r = b; return &res;`
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

	// Also track variables built via field assignments (pointer return pattern).
	fieldAssigned := collectFieldAssigned(fn.Body, idx)

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
		// Pointer return: return &varName where varName was field-assigned.
		if unary, isUnary := ret.Val.(*hir.UnaryExpr); isUnary && unary.Op == "&" {
			if ref, isRef := unary.X.(*hir.VarRefExpr); isRef {
				if st, ok := fieldAssigned[ref.Name]; ok {
					if found == nil {
						found = st
					} else if found.Name != st.Name {
						return false
					}
					return true
				}
			}
		}
		return false // returns something else → not eligible
	})
	if !ok || found == nil {
		return nil
	}
	return found
}

// collectFieldAssigned finds local variables that are fully populated via
// field assignments (e.g. `res.q = expr; res.r = expr;`) and matches them
// to SSS struct types.
func collectFieldAssigned(blk *hir.Block, idx structIndex) map[string]*mir2.StructTy {
	if blk == nil {
		return nil
	}
	// Collect field writes per variable: varName → set of field names.
	writes := make(map[string]map[string]bool)
	walkStmts(blk, func(s hir.Stmt) bool {
		if assign, ok := s.(*hir.AssignStmt); ok {
			if fe, ok := assign.Target.(*hir.FieldExpr); ok {
				if ref, ok := fe.X.(*hir.VarRefExpr); ok {
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

	// Collect field assignments for pointer-return pattern.
	fieldAssigns := collectFieldAssigns(fn.Body)

	walkStmtsMut(fn.Body, func(s hir.Stmt, replace func(hir.Stmt)) {
		ret, isRet := s.(*hir.ReturnStmt)
		if !isRet {
			return
		}

		// Pattern 1+2: direct StructLitExpr or indirect via variable.
		var lit *hir.StructLitExpr
		if l, ok := ret.Val.(*hir.StructLitExpr); ok {
			lit = l
		} else if ref, ok := ret.Val.(*hir.VarRefExpr); ok {
			lit = structVars[ref.Name]
		}
		if lit != nil {
			vals := structLitToTuple(lit, info)
			replace(&hir.ReturnStmt{Vals: vals})
			return
		}

		// Pattern 3: return &varName where varName was field-assigned.
		if unary, isUnary := ret.Val.(*hir.UnaryExpr); isUnary && unary.Op == "&" {
			if ref, isRef := unary.X.(*hir.VarRefExpr); isRef {
				if assigns, ok := fieldAssigns[ref.Name]; ok {
					vals := make([]hir.Expr, len(info.fieldTys))
					for field, expr := range assigns {
						if pos, ok := info.fieldMap[field]; ok {
							vals[pos] = expr
						}
					}
					replace(&hir.ReturnStmt{Vals: vals})
				}
			}
		}
	})

	// For pointer-return pattern: remove the field-assign statements that
	// were consumed into the tuple return.
	removePtrReturnAssigns(fn.Body, info, fieldAssigns)
}

// structLitToTuple converts a StructLitExpr to tuple values using promotionInfo.
func structLitToTuple(lit *hir.StructLitExpr, info *promotionInfo) []hir.Expr {
	vals := make([]hir.Expr, len(info.fieldTys))
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
func collectFieldAssigns(blk *hir.Block) map[string]map[string]hir.Expr {
	result := make(map[string]map[string]hir.Expr)
	if blk == nil {
		return result
	}
	for _, s := range blk.Body {
		if assign, ok := s.(*hir.AssignStmt); ok {
			if fe, ok := assign.Target.(*hir.FieldExpr); ok {
				if ref, ok := fe.X.(*hir.VarRefExpr); ok {
					if result[ref.Name] == nil {
						result[ref.Name] = make(map[string]hir.Expr)
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
func removePtrReturnAssigns(blk *hir.Block, info *promotionInfo, fieldAssigns map[string]map[string]hir.Expr) {
	if blk == nil {
		return
	}
	// Find which variables had their return &var promoted.
	promoted := make(map[string]bool)
	for _, s := range blk.Body {
		if ret, ok := s.(*hir.ReturnStmt); ok && len(ret.Vals) > 0 {
			// This was already rewritten — check if it came from a pointer return.
			// We can't easily tell, so check all field-assigned vars.
			for varName := range fieldAssigns {
				promoted[varName] = true
			}
		}
	}
	if len(promoted) == 0 {
		return
	}

	// Remove field-assign statements and var decls for promoted variables.
	var filtered []hir.Stmt
	for _, s := range blk.Body {
		if assign, ok := s.(*hir.AssignStmt); ok {
			if fe, ok := assign.Target.(*hir.FieldExpr); ok {
				if ref, ok := fe.X.(*hir.VarRefExpr); ok {
					if promoted[ref.Name] {
						if _, isPromotedField := info.fieldMap[fe.Field]; isPromotedField {
							continue // remove
						}
					}
				}
			}
		}
		// Also remove the variable declaration for the promoted struct.
		if decl, ok := s.(*hir.VarDeclStmt); ok {
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
func findOutParamFunctions(m *hir.Module, idx structIndex) map[string]*outParamInfo {
	result := make(map[string]*outParamInfo)

	for _, fn := range m.Funcs {
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

		// Analyze body: all uses of paramName must be `paramName.field = expr`.
		fields := make(map[string]hir.Expr) // field name → last assigned value
		allFieldWrites := true

		walkStmts(fn.Body, func(s hir.Stmt) bool {
			// Check for field-write pattern: paramName.field = expr
			if assign, ok := s.(*hir.AssignStmt); ok {
				if fe, ok := assign.Target.(*hir.FieldExpr); ok {
					if ref, ok := fe.X.(*hir.VarRefExpr); ok && ref.Name == paramName {
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
func usesVar(s hir.Stmt, name string) bool {
	switch st := s.(type) {
	case *hir.AssignStmt:
		// Field writes on the target are OK (handled by caller).
		// Check if Val uses the var.
		return exprUsesVar(st.Val, name)
	case *hir.VarDeclStmt:
		return exprUsesVar(st.Init, name)
	case *hir.ReturnStmt:
		if exprUsesVar(st.Val, name) {
			return true
		}
		for _, v := range st.Vals {
			if exprUsesVar(v, name) {
				return true
			}
		}
	case *hir.ExprStmt:
		return exprUsesVar(st.Expr, name)
	case *hir.IfStmt:
		return exprUsesVar(st.Cond, name)
	}
	return false
}

// exprUsesVar checks if an expression references a variable by name.
func exprUsesVar(e hir.Expr, name string) bool {
	if e == nil {
		return false
	}
	switch x := e.(type) {
	case *hir.VarRefExpr:
		return x.Name == name
	case *hir.FieldExpr:
		return exprUsesVar(x.X, name)
	case *hir.BinExpr:
		return exprUsesVar(x.L, name) || exprUsesVar(x.R, name)
	case *hir.UnaryExpr:
		return exprUsesVar(x.X, name)
	case *hir.CallExpr:
		for _, a := range x.Args {
			if exprUsesVar(a, name) {
				return true
			}
		}
	case *hir.CastExpr:
		return exprUsesVar(x.X, name)
	}
	return false
}

// rewriteOutParamFunc transforms a void function with out-param to tuple return.
//
//	void divmod(u8 a, u8 b, DivResult *out) { out->q = a/b; out->r = a%b; }
//	→ (u8, u8) divmod(u8 a, u8 b) { return (a/b, a%b); }
func rewriteOutParamFunc(fn *hir.Func, info *outParamInfo) {
	// Remove the out-param from signature.
	fn.Params = fn.Params[:info.paramIndex]

	// Set tuple return types.
	fn.RetTys = info.fieldTys
	fn.RetTy = mir2.TyVoid

	// Collect field assignments in order, then replace with a tuple return.
	vals := make([]hir.Expr, len(info.fieldTys))

	// Walk body and collect field → expr assignments, remove them.
	var newBody []hir.Stmt
	for _, s := range fn.Body.Body {
		if assign, ok := s.(*hir.AssignStmt); ok {
			if fe, ok := assign.Target.(*hir.FieldExpr); ok {
				if ref, ok := fe.X.(*hir.VarRefExpr); ok && ref.Name == info.paramName {
					if pos, found := info.fieldMap[fe.Field]; found {
						vals[pos] = assign.Val
						continue // remove this statement
					}
				}
			}
		}
		newBody = append(newBody, s)
	}

	// Append tuple return.
	newBody = append(newBody, &hir.ReturnStmt{Vals: vals})
	fn.Body.Body = newBody
}

// rewriteOutParamCallSites transforms call sites:
//
//	DivResult res; divmod(a, b, &res); use(res.q);
//	→ let (res_q, res_r) = divmod(a, b); use(res_q);
//
// Also handles the pattern where the struct var is declared with init,
// then passed as out-param: DivResult res = {}; f(x, &res); use(res.field);
func rewriteOutParamCallSites(blk *hir.Block, eligible map[string]*outParamInfo) {
	if blk == nil || len(eligible) == 0 {
		return
	}

	for i := 0; i < len(blk.Body); i++ {
		s := blk.Body[i]

		// Recurse into nested blocks.
		switch st := s.(type) {
		case *hir.IfStmt:
			rewriteOutParamCallSites(st.Then, eligible)
			rewriteOutParamCallSites(st.Else, eligible)
		case *hir.WhileStmt:
			rewriteOutParamCallSites(st.Body, eligible)
		case *hir.ForRangeStmt:
			rewriteOutParamCallSites(st.Body, eligible)
		}

		// Look for ExprStmt containing a call to a promoted function.
		exprSt, isExpr := s.(*hir.ExprStmt)
		if !isExpr {
			continue
		}
		call, isCall := exprSt.Expr.(*hir.CallExpr)
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
		addrOf, isAddr := lastArg.(*hir.UnaryExpr)
		if !isAddr || addrOf.Op != "&" {
			// Also check for plain VarRefExpr (already a pointer).
			ref, isRef := lastArg.(*hir.VarRefExpr)
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
			blk.Body[i] = &hir.TupleLetStmt{
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
		ref, isRef := addrOf.X.(*hir.VarRefExpr)
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
		blk.Body[i] = &hir.TupleLetStmt{
			Names: names,
			Tys:   info.fieldTys,
			Call:  call,
		}

		// Remove the preceding VarDeclStmt for the struct variable if it exists.
		if i > 0 {
			if decl, ok := blk.Body[i-1].(*hir.VarDeclStmt); ok && decl.Name == varName {
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
