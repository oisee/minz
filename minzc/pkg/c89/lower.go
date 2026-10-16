package c89

import (
	"fmt"
	"maps"
	"strings"

	cc "github.com/minz/minzc/pkg/cparse"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

type lowerer struct {
	privateSymbols map[string]string
	typeError      error // unsupported mapped types must never produce an executable
	ast            *cc.AST
	hm             *hir.Module
	globals        map[string]mir2.Ty
	typedefs       map[string]mir2.Ty
	structs        map[string]*mir2.StructTy
	cStructs       map[cc.Type]string // C struct type → MIR2 struct name

	// ObjC: class info from @interface declarations.
	objcClasses   map[string]*objcClassInfo
	objcProtocols map[string]*objcProtocolInfo
}

func (l *lowerer) lower() error {
	for tu := l.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		switch ed.Case {
		case cc.ExternalDeclarationFuncDef:
			f, err := l.lowerFunc(ed.FunctionDefinition)
			if err != nil {
				return err
			}
			if f != nil {
				l.hm.Funcs = append(l.hm.Funcs, f)
			}
		case cc.ExternalDeclarationDecl:
			if err := l.lowerTopDecl(ed.Declaration); err != nil {
				return err
			}
		case cc.ExternalDeclarationObjCInterface:
			if err := l.lowerObjCInterface(ed.ObjCInterface); err != nil {
				return err
			}
		case cc.ExternalDeclarationObjCImplementation:
			if err := l.lowerObjCImplementation(ed.ObjCImplementation); err != nil {
				return err
			}
		case cc.ExternalDeclarationObjCProtocol:
			// Protocols are compile-time constraints — register for conformance checking.
			l.lowerObjCProtocol(ed.ObjCProtocol)
		}
	}
	return l.typeError
}

// ── Type mapping ────────────────────────────────────────────────────────────

func (l *lowerer) mapType(t cc.Type) mir2.Ty {
	if t == nil {
		return mir2.TyVoid
	}
	switch t.Kind() {
	case cc.Void:
		return mir2.TyVoid
	case cc.Bool, cc.Char, cc.SChar, cc.UChar:
		if cc.IsSignedInteger(t) {
			return mir2.TyI8
		}
		return mir2.TyU8
	case cc.Enum:
		return l.mapType(t.(*cc.EnumType).UnderlyingType())
	case cc.Short, cc.UShort, cc.Int, cc.UInt:
		if cc.IsSignedInteger(t) {
			return mir2.TyI16
		}
		return mir2.TyU16
	case cc.Long, cc.ULong:
		if cc.IsSignedInteger(t) {
			return mir2.TyI32
		}
		return mir2.TyU32
	case cc.LongLong, cc.ULongLong:
		if l.typeError == nil {
			l.typeError = fmt.Errorf("c89: 64-bit %s is not supported by the Z80 frontend", t)
		}
		return mir2.TyU32 // error returned by lower; never emit narrowed code
	case cc.Ptr, cc.Function:
		return mir2.TyPtr
	case cc.Array:
		return mir2.TyPtr // arrays decay to pointers
	case cc.Struct, cc.Union:
		return mir2.TyPtr // structs/unions lowered to pointer (address) on Z80
	default:
		// ObjC message expressions use parser placeholders; selector types
		// are supplied separately by objc_lower. They are not C scalar types.
		if t.Kind() == cc.InvalidKind && len(l.objcClasses) > 0 {
			return mir2.TyU16
		}
		if l.typeError == nil {
			l.typeError = fmt.Errorf("c89: unsupported Z80 type %s", t)
		}
		return mir2.TyU16 // only retained while collecting the diagnostic
	}
}

func (l *lowerer) structTag(st *cc.StructType) string {
	t := st.Tag()
	return t.SrcStr()
}

func (l *lowerer) unionTag(ut *cc.UnionType) string {
	t := ut.Tag()
	return t.SrcStr()
}

// cConvert leaves already matching values structurally intact, including
// aggregate literals needed by struct-return promotion.
func cConvert(e hir.Expr, ty mir2.Ty) hir.Expr {
	if e.ExprTy() == ty {
		return e
	}
	return hir.Cast(e, ty)
}

// Conversion to C _Bool tests the complete scalar before narrowing to its
// one-byte ABI representation (C99 6.3.1.2).
func (l *lowerer) convertType(e hir.Expr, ty cc.Type) hir.Expr {
	if ty != nil && (ty.Kind() == cc.Struct || ty.Kind() == cc.Union) {
		switch e.(type) {
		case *hir.CallExpr, *hir.CallIndirectExpr:
			return e // aggregate call results already yield owned object addresses
		}
		if _, aggregate := mir2.BaseOf(e.ExprTy()).(*mir2.StructTy); aggregate {
			return hir.AddrExpr(e)
		}
		return e // aggregate values already represented by an address
	}
	if ty != nil && ty.Kind() == cc.Bool {
		return boolNormalize(e)
	}
	return cConvert(e, l.mapType(ty))
}

// boolNormalize wraps an expression so that any non-zero value becomes 1.
// Implements C99 _Bool semantics: `_Bool b = 42` → `b = 1`.
func boolNormalize(e hir.Expr) hir.Expr {
	return hir.Cast(&hir.BinExpr{Op: "!=", L: e, R: &hir.IntLitExpr{Val: 0, Ty: e.ExprTy()}, Ty: mir2.TyBool}, mir2.TyU8)
}

// isFunction checks if name is a known function in the module.
func (l *lowerer) isFunction(name string) bool {
	for _, f := range l.hm.Funcs {
		if f.Name == name {
			return true
		}
	}
	return false
}

// ── Top-level declarations ──────────────────────────────────────────────────

func (l *lowerer) lowerTopDecl(d *cc.Declaration) error {
	if d == nil {
		return nil
	}
	// _Static_assert — evaluate at compile time, emit error if false.
	if d.Case == cc.DeclarationAssert {
		if sa := d.StaticAssertDeclaration; sa != nil {
			if sa.ConstantExpression != nil {
				if v := sa.ConstantExpression.Value(); v != nil {
					if constToInt64(v) == 0 {
						msg := sa.Token4.SrcStr() // string literal message
						return fmt.Errorf("static assertion failed: %s", msg)
					}
				}
			}
		}
		return nil
	}
	for idl := d.InitDeclaratorList; idl != nil; idl = idl.InitDeclaratorList {
		id := idl.InitDeclarator
		if id == nil || id.Declarator == nil {
			continue
		}
		decl := id.Declarator
		name := decl.Name()
		if name == "" || name == "__predefined_declarator" {
			continue
		}
		ty := decl.Type()
		if ty == nil {
			continue
		}

		// Typedef declarations: register struct/union types, then skip.
		if decl.IsTypename() {
			if ty.Kind() == cc.Struct {
				st := ty.(*cc.StructType)
				tag := l.structTag(st)
				if tag == "" {
					tag = name // use typedef name for anonymous structs
				}
				if l.structs[tag] == nil {
					l.lowerStructDecl(tag, st)
				}
				l.cStructs[ty] = tag // also register the typedef's cc.Type
				l.typedefs[name] = mir2.TyPtr
			} else if ty.Kind() == cc.Union {
				ut := ty.(*cc.UnionType)
				utTag := ut.Tag()
				tag := utTag.SrcStr()
				if tag == "" {
					tag = name
				}
				if l.structs[tag] == nil {
					l.lowerUnionDecl(tag, ut)
				}
				l.typedefs[name] = mir2.TyPtr
			}
			continue
		}

		// Skip function declarations (prototypes).
		if ty.Kind() == cc.Function {
			continue
		}

		// Struct/union type declaration.
		if ty.Kind() == cc.Struct {
			st := ty.(*cc.StructType)
			tag := l.structTag(st)
			if tag != "" && l.structs[tag] == nil {
				l.lowerStructDecl(tag, st)
			}
		} else if ty.Kind() == cc.Union {
			ut := ty.(*cc.UnionType)
			utTag := ut.Tag()
			tag := utTag.SrcStr()
			if tag != "" && l.structs[tag] == nil {
				l.lowerUnionDecl(tag, ut)
			}
		}

		// Global variable.
		// For arrays, preserve ArrayTy so HIR lowerer returns address
		// (array decay) instead of loading the value as a pointer.
		mty := l.storageType(ty)
		if decl.IsExtern() && id.Initializer == nil {
			l.globals[name] = mty
			continue // a declaration supplies a type, not another storage object
		}
		g := mir2.Global{Name: l.symbolName(name), Ty: mty, Volatile: ty.Attributes().IsVolatile()}
		if id.Initializer != nil {
			g.Init = make([]byte, mty.Width()/8)
			if err := l.writeGlobalInitializer(&g, id.Initializer, nil); err != nil {
				return fmt.Errorf("global %s: %w", name, err)
			}
		}

		l.hm.Globals = append(l.hm.Globals, g)
		l.globals[name] = mty
	}
	return nil
}

// Storage preserves aggregate layout; expression types may still decay to pointers.
func (l *lowerer) storageType(t cc.Type) mir2.Ty {
	switch t := t.(type) {
	case *cc.ArrayType:
		if t.Len() < 0 {
			if l.typeError == nil {
				l.typeError = fmt.Errorf("variable-length array storage is not supported")
			}
			return mir2.NewArray(mir2.TyU8, 1)
		}
		return mir2.NewArray(l.storageType(t.Elem()), int(t.Len()))
	case *cc.StructType:
		if name, ok := l.cStructs[t]; ok {
			return l.structs[name]
		}
		tag := l.structTag(t)
		if tag != "" && l.structs[tag] != nil {
			return l.structs[tag]
		}
		if tag == "" {
			tag = fmt.Sprintf("__anonymous_struct_%d", len(l.structs))
		}
		l.lowerStructDecl(tag, t)
		return l.structs[tag]
	case *cc.UnionType:
		if name, ok := l.cStructs[t]; ok {
			return l.structs[name]
		}
		tag := l.unionTag(t)
		if tag == "" {
			tag = fmt.Sprintf("__anonymous_union_%d", len(l.structs))
		}
		if l.structs[tag] == nil {
			l.lowerUnionDecl(tag, t)
		}
		return l.structs[tag]
	default:
		return l.mapType(t)
	}
}

// A flexible array member contributes no inline bytes to sizeof. Its element
// type still determines accesses into caller-provided trailing storage.
func (l *lowerer) storageFieldType(field *cc.Field) mir2.Ty {
	if field.IsBitfield() && l.typeError == nil {
		l.typeError = fmt.Errorf("c89: bit-field storage and access are not supported on Z80")
	}
	if field.IsFlexibleArrayMember() {
		array := field.Type().(*cc.ArrayType)
		return mir2.NewArray(l.storageType(array.Elem()), 0)
	}
	return l.storageType(field.Type())
}

// The typed parser already resolves nested braces and designators to byte
// offsets. Traverse all scalar leaves in source order so later designators win.
func (l *lowerer) writeGlobalInitializer(g *mir2.Global, init *cc.Initializer, scope map[string]string) error {
	data := g.Init
	if init == nil {
		return nil
	}
	if init.Case == cc.InitializerInitList {
		for list := init.InitializerList; list != nil; list = list.InitializerList {
			if err := l.writeGlobalInitializer(g, list.Initializer, scope); err != nil {
				return err
			}
		}
		return nil
	}
	if f := init.Field(); f != nil && f.IsBitfield() {
		return fmt.Errorf("bit-field initializer is not supported")
	}
	off, width := int(init.Offset()), int(init.Type().Size())
	if off < 0 || width < 1 || off+width > len(data) {
		return fmt.Errorf("initializer layout exceeds storage: offset %d width %d size %d", off, width, len(data))
	}
	// Later designated leaves replace earlier storage, including relocations.
	kept := g.Relocs[:0]
	for _, r := range g.Relocs {
		if r.Offset >= off+width || r.Offset+2 <= off {
			kept = append(kept, r)
		}
	}
	g.Relocs = kept
	if init.Type().Kind() == cc.Ptr {
		if sym, addend, ok := l.globalAddress(init.AssignmentExpression, false, scope); ok {
			if width != 2 {
				return fmt.Errorf("unsupported pointer initializer width %d", width)
			}
			g.Relocs = append(g.Relocs, mir2.GlobalRelocation{Offset: off, Symbol: sym, Addend: addend})
			clear(data[off : off+width])
			return nil
		}
	}
	if str, ok := init.Value().(cc.StringValue); ok {
		if init.Type().Kind() != cc.Array {
			return fmt.Errorf("string pointer initializer requires a relocation")
		}
		copy(data[off:off+width], []byte(str))
		return nil
	}
	var value uint64
	switch v := init.Value().(type) {
	case cc.Int64Value:
		value = uint64(v)
	case cc.UInt64Value:
		value = uint64(v)
	case *cc.ZeroValue:
		value = 0
	default:
		return fmt.Errorf("unsupported global initializer value %T", v)
	}
	if init.Type().Kind() == cc.Bool && value != 0 {
		value = 1
	}
	for i := 0; i < width; i++ {
		data[off+i] = byte(value >> (8 * i))
	}
	return nil
}

func (l *lowerer) lowerStructDecl(tag string, st *cc.StructType) {
	mst := &mir2.StructTy{Name: tag}
	var offsets []int
	nf := st.NumFields()
	for i := 0; i < nf; i++ {
		f := st.FieldByIndex(i)
		if f == nil {
			continue
		}
		fname := f.Name()
		isAnonymous := fname == ""
		if isAnonymous {
			fname = fmt.Sprintf("_f%d", i)
		}
		fty := f.Type()
		// Flatten embedded struct/union fields into parent.
		// Named: `Point origin` → `origin.x`, `origin.y` (prefixed)
		// Anonymous (C11): `struct { uint8_t x; uint8_t y; };` → `x`, `y` (no prefix)
		if fty != nil && (fty.Kind() == cc.Struct || fty.Kind() == cc.Union) {
			var innerFields []mir2.StructField
			var innerLayout *mir2.StructTy
			if fty.Kind() == cc.Struct {
				innerSt, ok := fty.(*cc.StructType)
				if ok {
					innerTag := l.structTag(innerSt)
					if innerTag == "" {
						innerTag = fname
					}
					if l.structs[innerTag] == nil {
						l.lowerStructDecl(innerTag, innerSt)
					}
					if inner := l.structs[innerTag]; inner != nil {
						innerFields = inner.Fields
						innerLayout = inner
					}
				}
			} else {
				innerUt, ok := fty.(*cc.UnionType)
				if ok {
					innerTag := l.unionTag(innerUt)
					if innerTag == "" {
						innerTag = fname
					}
					if l.structs[innerTag] == nil {
						l.lowerUnionDecl(innerTag, innerUt)
					}
					if inner := l.structs[innerTag]; inner != nil {
						innerFields = inner.Fields
						innerLayout = inner
					}
				}
			}
			if innerFields != nil {
				for index, sf := range innerFields {
					offsets = append(offsets, int(f.Offset())+innerLayout.ByteOffset(index))
					if isAnonymous {
						// C11 anonymous: promote fields directly (no prefix)
						mst.Fields = append(mst.Fields, mir2.StructField{
							Name:     sf.Name,
							Ty:       sf.Ty,
							Volatile: sf.Volatile || fty.Attributes().IsVolatile(),
						})
					} else {
						// Named embedded: prefix with field name
						mst.Fields = append(mst.Fields, mir2.StructField{
							Name:     fname + "." + sf.Name,
							Ty:       sf.Ty,
							Volatile: sf.Volatile || fty.Attributes().IsVolatile(),
						})
					}
				}
				continue
			}
		}
		mst.Fields = append(mst.Fields, mir2.StructField{
			Name:     fname,
			Ty:       l.storageFieldType(f),
			Volatile: f.Type().Attributes().IsVolatile(),
		})
		offsets = append(offsets, int(f.Offset()))
	}
	// Flattening retains field names while explicit offsets preserve overlaps.
	packed := 0
	for i, field := range mst.Fields {
		if offsets[i] != packed {
			mst.FieldOffsets = offsets
		}
		packed += mir2.ByteWidth(field.Ty)
	}
	if packed != int(st.Size()) || len(mst.FieldOffsets) != 0 {
		mst.FieldOffsets = offsets
		mst.StorageBytes = int(st.Size())
	}
	l.structs[tag] = mst
	l.cStructs[st] = tag
	l.hm.Structs = append(l.hm.Structs, mst)
}

// lowerUnionDecl lowers a C union to a mir2 StructTy where all fields overlap at offset 0.
// Each field gets the type of the largest union member so ByteOffset always returns 0.
func (l *lowerer) lowerUnionDecl(tag string, ut *cc.UnionType) {
	mst := &mir2.StructTy{Name: tag}
	// Find max field size for the single storage slot.
	maxWidth := 0
	nf := ut.NumFields()
	type fieldInfo struct {
		name     string
		ty       mir2.Ty
		volatile bool
	}
	var fields []fieldInfo
	for i := 0; i < nf; i++ {
		f := ut.FieldByIndex(i)
		if f == nil {
			continue
		}
		fname := f.Name()
		if fname == "" {
			fname = fmt.Sprintf("_f%d", i)
		}
		mty := l.storageFieldType(f)
		w := mir2.ByteWidth(mty)
		if w > maxWidth {
			maxWidth = w
		}
		fields = append(fields, fieldInfo{name: fname, ty: mty, volatile: f.Type().Attributes().IsVolatile()})
	}
	// All fields share offset 0 — store as single-field struct with largest type,
	// then register each field name mapping to offset 0.
	// For simplicity, store ALL field names but use largest type for width.
	for _, fi := range fields {
		mst.Fields = append(mst.Fields, mir2.StructField{Name: fi.name, Ty: fi.ty, Volatile: fi.volatile})
	}
	mst.IsUnion = true // mark as union so ByteOffset returns 0 for all fields
	l.structs[tag] = mst
	l.cStructs[ut] = tag
	l.hm.Structs = append(l.hm.Structs, mst)
}

func (l *lowerer) evalConstInit(init *cc.Initializer) (int64, bool) {
	if init == nil || init.AssignmentExpression == nil {
		return 0, false
	}
	return l.evalConst(init.AssignmentExpression)
}

// ── Function lowering ───────────────────────────────────────────────────────

func (l *lowerer) lowerFunc(fd *cc.FunctionDefinition) (*hir.Func, error) {
	decl := fd.Declarator
	name := decl.Name()
	if name == "" {
		return nil, nil
	}

	ft, ok := decl.Type().(*cc.FunctionType)
	if !ok {
		return nil, fmt.Errorf("function %s: expected function type", name)
	}
	if ft.IsVariadic() {
		return nil, fmt.Errorf("function %s: variadic definitions require an unsupported Z80 varargs ABI", name)
	}

	retTy := l.mapType(ft.Result())
	if ft.Result().Kind() == cc.Struct || ft.Result().Kind() == cc.Union {
		retTy = l.storageType(ft.Result())
	}

	var params []hir.Param
	for _, p := range ft.Parameters() {
		// In C, f(void) means no parameters — skip void params.
		if p.Type() != nil && p.Type().Kind() == cc.Void {
			continue
		}
		pname := p.Name()
		if pname == "" {
			pname = fmt.Sprintf("_p%d", len(params))
		}
		params = append(params, hir.Param{
			Name: pname,
			Ty:   l.mapType(p.Type()),
		})
	}

	fl := &funcLow{
		low:      l,
		name:     l.symbolName(name),
		retTy:    retTy,
		retCType: ft.Result(),
		locals:   make(map[string]mir2.Ty),
	}

	// Register params as locals.
	for _, p := range params {
		fl.locals[p.Name] = p.Ty
	}

	body, err := fl.lowerCompound(fd.CompoundStatement)
	if err != nil {
		return nil, fmt.Errorf("function %s: %w", name, err)
	}

	var prologue []hir.Stmt
	for i, p := range ft.Parameters() {
		if p.Type() != nil && (p.Type().Kind() == cc.Struct || p.Type().Kind() == cc.Union) && i < len(params) {
			// C99 6.9.1: each by-value parameter owns an independent object.
			// The physical ABI carries its source address, then the callee copies
			// all bytes into its private frame before executing the body.
			prologue = append(prologue, &hir.VarDeclStmt{Name: params[i].Name, Ty: l.storageType(p.Type()), Init: hir.Var(params[i].Name, mir2.TyPtr), Volatile: p.Type().Attributes().IsVolatile()})
			continue
		}
		if p.Type() != nil && p.Type().Attributes().IsVolatile() && i < len(params) {
			prologue = append(prologue, &hir.VarDeclStmt{Name: params[i].Name, Ty: params[i].Ty, Init: hir.Var(params[i].Name, params[i].Ty), Volatile: true})
		}
	}
	body = append(prologue, body...)

	// Ensure function ends with a return.
	if len(body) == 0 || !isReturn(body[len(body)-1]) {
		if retTy == mir2.TyVoid {
			body = append(body, hir.RetVoid())
		}
	}

	return &hir.Func{
		Name:   l.symbolName(name),
		Params: params,
		RetTy:  retTy,
		Body:   hir.Blk(body...),
	}, nil
}

func isReturn(s hir.Stmt) bool {
	_, ok := s.(*hir.ReturnStmt)
	return ok
}

// ── Function body lowering ──────────────────────────────────────────────────

type funcLow struct {
	low      *lowerer
	name     string
	retTy    mir2.Ty
	retCType cc.Type
	locals   map[string]mir2.Ty

	// static locals: original name → mangled global name.
	staticRenames map[string]string

	// pendingStmts collects side-effect statements from expressions
	// (e.g., assignments embedded in ++x, x++, chained a=b=c).
	// Drained before the next statement in compound/decl lowering.
	pendingStmts []hir.Stmt
	tmpCount     int // counter for temporary variable names

	// ObjC: set when lowering a method inside @implementation.
	objcClass     *objcClassInfo
	objcLocals    map[string]string // varName → className (for typed receivers)
	objcProtoVars map[string]string // varName → protocolName (for id<Proto> dynamic dispatch)

	continuePrelude func() ([]hir.Stmt, error) // do/for effects before continue
	switchEnd       string                     // nearest switch break target; empty inside a loop
	switchCases     *[]cSwitchCase             // labels collected for the active dispatch
}

func (fl *funcLow) tmpCounter() int {
	fl.tmpCount++
	return fl.tmpCount
}

// drainPending flushes any pending side-effect statements into the given slice.
func (fl *funcLow) drainPending(stmts *[]hir.Stmt) {
	if len(fl.pendingStmts) > 0 {
		*stmts = append(*stmts, fl.pendingStmts...)
		fl.pendingStmts = fl.pendingStmts[:0]
	}
}

func (fl *funcLow) lowerCompound(cs *cc.CompoundStatement) ([]hir.Stmt, error) {
	oldLocals, oldStatic, oldObjc, oldProto := fl.locals, fl.staticRenames, fl.objcLocals, fl.objcProtoVars
	fl.locals = maps.Clone(fl.locals)
	fl.staticRenames = maps.Clone(fl.staticRenames)
	fl.objcLocals = maps.Clone(fl.objcLocals)
	fl.objcProtoVars = maps.Clone(fl.objcProtoVars)
	defer func() {
		fl.locals = oldLocals
		fl.staticRenames = oldStatic
		fl.objcLocals = oldObjc
		fl.objcProtoVars = oldProto
	}()
	if cs == nil {
		return nil, nil
	}
	var stmts []hir.Stmt
	for bl := cs.BlockItemList; bl != nil; bl = bl.BlockItemList {
		bi := bl.BlockItem
		switch bi.Case {
		case cc.BlockItemDecl:
			s, err := fl.lowerLocalDecl(bi.Declaration)
			if err != nil {
				return nil, err
			}
			fl.drainPending(&stmts)
			stmts = append(stmts, s...)
		case cc.BlockItemStmt:
			s, err := fl.lowerStmt(bi.Statement)
			if err != nil {
				return nil, err
			}
			fl.drainPending(&stmts)
			stmts = append(stmts, s...)
		}
	}
	return stmts, nil
}

func (fl *funcLow) lowerLocalDecl(d *cc.Declaration) ([]hir.Stmt, error) {
	if d == nil {
		return nil, nil
	}
	var stmts []hir.Stmt
	for idl := d.InitDeclaratorList; idl != nil; idl = idl.InitDeclaratorList {
		id := idl.InitDeclarator
		if id == nil || id.Declarator == nil {
			continue
		}
		decl := id.Declarator
		name := decl.Name()
		if name == "" {
			continue
		}
		// C99 6.4.2.2: __func__ is a predefined static character array.
		// Its unused declaration has no effects and needs no runtime storage.
		if name == "__func__" && decl.ReadCount() == 0 {
			continue
		}
		ty := fl.low.mapType(decl.Type())
		declarationTy := ty
		if decl.Type().Kind() == cc.Struct || decl.Type().Kind() == cc.Union {
			declarationTy = fl.low.storageType(decl.Type())
		}
		if !decl.IsStatic() {
			delete(fl.staticRenames, name)
		}

		// static local → emit as mangled global, reference by mangled name.
		// This also preserves the static lifetime of referenced __func__.
		if decl.IsStatic() {
			mangled := fl.name + "__" + name
			if fl.staticRenames == nil {
				fl.staticRenames = make(map[string]string)
			}
			fl.staticRenames[name] = mangled
			ty = fl.low.storageType(decl.Type())
			fl.locals[name] = ty
			g := mir2.Global{Name: mangled, Ty: ty, Volatile: decl.Type().Attributes().IsVolatile()}
			if id.Initializer != nil {
				g.Init = make([]byte, mir2.ByteWidth(ty))
				if err := fl.low.writeGlobalInitializer(&g, id.Initializer, fl.staticRenames); err != nil {
					return nil, fmt.Errorf("static local %s: %w", name, err)
				}
			}
			fl.low.hm.Globals = append(fl.low.hm.Globals, g)
			fl.low.globals[mangled] = ty
			// Alias: local name maps to global name for subsequent references.
			fl.locals[name] = ty
			// Emit nothing — the variable lives as a global.
			// We need a way for VarRef to find it by the mangled name.
			// Reads/writes use the rename recorded before initialization.
			continue
		}

		// Give a shadowing object a distinct HIR identity; outer loop-control
		// references must not become the inner object's name at continue.
		sourceName := name
		if _, shadowed := fl.locals[name]; shadowed {
			name = fmt.Sprintf("__local_%s_%d", name, fl.tmpCounter())
			if fl.staticRenames == nil {
				fl.staticRenames = make(map[string]string)
			}
			fl.staticRenames[sourceName] = name
		}
		fl.locals[sourceName] = ty
		fl.locals[name] = ty

		if array, ok := decl.Type().(*cc.ArrayType); ok {
			if array.Len() < 1 {
				return nil, fmt.Errorf("automatic array %s requires a constant positive bound", sourceName)
			}
			storage := fl.low.storageType(array)
			stmts = append(stmts, &hir.VarDeclStmt{Name: name, Ty: storage, Volatile: decl.Type().Attributes().IsVolatile()})
			if id.Initializer != nil {
				initializers, err := fl.lowerAutomaticAggregateInitializer(name, storage, id.Initializer, decl.Type().Attributes().IsVolatile())
				if err != nil {
					return nil, fmt.Errorf("automatic array %s: %w", sourceName, err)
				}
				stmts = append(stmts, initializers...)
			}
			continue
		}

		if (decl.Type().Kind() == cc.Struct || decl.Type().Kind() == cc.Union) && (cStorageContainsArray(declarationTy) || len(declarationTy.(*mir2.StructTy).FieldOffsets) != 0) && id.Initializer != nil && id.Initializer.Case == cc.InitializerInitList {
			stmts = append(stmts, &hir.VarDeclStmt{Name: name, Ty: declarationTy, Volatile: decl.Type().Attributes().IsVolatile()})
			initializers, err := fl.lowerAutomaticAggregateInitializer(name, declarationTy, id.Initializer, decl.Type().Attributes().IsVolatile())
			if err != nil {
				return nil, fmt.Errorf("automatic aggregate %s: %w", sourceName, err)
			}
			stmts = append(stmts, initializers...)
			continue
		}

		var init hir.Expr
		if id.Initializer != nil {
			switch id.Initializer.Case {
			case cc.InitializerExpr:
				if id.Initializer.AssignmentExpression != nil {
					value, err := fl.lowerExprAsExpr(id.Initializer.AssignmentExpression)
					if err != nil {
						return nil, err
					}
					fl.drainPending(&stmts)
					init = value
					// _Bool normalization: any non-zero value becomes 1.
					if decl.Type() != nil && decl.Type().Kind() == cc.Bool {
						init = boolNormalize(init)
					}
				}
			case cc.InitializerInitList:
				// Brace initializer: { val1, val2, ... }
				// Check if this is a struct type.
				if st := fl.low.resolveStructType(decl.Type()); st != nil {
					lit, err := fl.lowerStructInit(st, id.Initializer.InitializerList)
					if err != nil {
						return nil, err
					}
					init = lit
				}
			}
		}

		stmts = append(stmts, &hir.VarDeclStmt{Name: name, Ty: declarationTy, Init: init, Volatile: decl.Type().Attributes().IsVolatile()})
	}
	return stmts, nil
}

// ── Statement lowering ──────────────────────────────────────────────────────

// Each statement owns its expression effects. Parent conditions must never
// be drained by a child compound, nor may arm effects escape their branch.
func (fl *funcLow) lowerStmt(s *cc.Statement) ([]hir.Stmt, error) {
	outer := fl.pendingStmts
	fl.pendingStmts = nil
	body, err := fl.lowerStmtInner(s)
	effects := fl.pendingStmts
	fl.pendingStmts = outer
	return append(effects, body...), err
}

func (fl *funcLow) lowerControlExpr(e cc.ExpressionNode) (hir.Expr, []hir.Stmt, error) {
	outer := fl.pendingStmts
	fl.pendingStmts = nil
	value, err := fl.lowerExprAsExpr(e)
	effects := fl.pendingStmts
	fl.pendingStmts = outer
	return value, effects, err
}

// Re-lowered continue effects retain the lexical environment of the loop
// header, while generated temporaries remain unique in the function.
func (fl *funcLow) captureControlPrelude(fn func() ([]hir.Stmt, error)) func() ([]hir.Stmt, error) {
	locals, renames := maps.Clone(fl.locals), maps.Clone(fl.staticRenames)
	return func() ([]hir.Stmt, error) {
		oldLocals, oldRenames := fl.locals, fl.staticRenames
		fl.locals, fl.staticRenames = maps.Clone(locals), maps.Clone(renames)
		defer func() { fl.locals, fl.staticRenames = oldLocals, oldRenames }()
		return fn()
	}
}

// lowerConditionalValue puts arm effects inside their selected branch. The
// condition is evaluated before either arm and logical callers normalize truth.
func (fl *funcLow) lowerConditionalValue(cond hir.Expr, then, els hir.Expr, thenEffects, elseEffects []hir.Stmt, ty mir2.Ty) hir.Expr {
	if len(thenEffects) == 0 && len(elseEffects) == 0 {
		return &hir.CondExpr{Cond: cond, Then: then, Else: els, Ty: ty}
	}
	name := fmt.Sprintf("__conditional_%d", fl.tmpCounter())
	fl.locals[name] = ty
	result := &hir.VarRefExpr{Name: name, Ty: ty}
	fl.pendingStmts = append(fl.pendingStmts, &hir.VarDeclStmt{Name: name, Ty: ty}, hir.If(cond,
		hir.Blk(append(thenEffects, &hir.AssignStmt{Target: result, Val: then})...),
		hir.Blk(append(elseEffects, &hir.AssignStmt{Target: result, Val: els})...)))
	return result
}

func (fl *funcLow) lowerLoopBody(s *cc.Statement, prelude func() ([]hir.Stmt, error)) ([]hir.Stmt, error) {
	old, oldBreak := fl.continuePrelude, fl.switchEnd
	fl.continuePrelude, fl.switchEnd = prelude, ""
	defer func() { fl.continuePrelude, fl.switchEnd = old, oldBreak }()
	return fl.lowerStmt(s)
}

func (fl *funcLow) lowerStmtInner(s *cc.Statement) ([]hir.Stmt, error) {
	if s == nil {
		return nil, nil
	}
	switch s.Case {
	case cc.StatementCompound:
		body, err := fl.lowerCompound(s.CompoundStatement)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{&hir.Block{Body: body}}, nil

	case cc.StatementExpr:
		return fl.lowerExprStmt(s.ExpressionStatement)

	case cc.StatementSelection:
		return fl.lowerSelection(s.SelectionStatement)

	case cc.StatementIteration:
		return fl.lowerIteration(s.IterationStatement)

	case cc.StatementJump:
		return fl.lowerJump(s.JumpStatement)

	case cc.StatementLabeled:
		ls := s.LabeledStatement
		if ls == nil {
			return nil, nil
		}
		var stmts []hir.Stmt
		if ls.Case == cc.LabeledStatementCaseLabel || ls.Case == cc.LabeledStatementDefault {
			if fl.switchCases == nil {
				return nil, fmt.Errorf("case/default without active switch")
			}
			label := fmt.Sprintf("__switch_case_%d", fl.tmpCounter())
			var val hir.Expr
			if ls.Case == cc.LabeledStatementCaseLabel {
				var err error
				val, err = fl.lowerExprAsExpr(ls.ConstantExpression)
				if err != nil {
					return nil, err
				}
			}
			*fl.switchCases = append(*fl.switchCases, cSwitchCase{val: val, label: label})
			stmts = append(stmts, &hir.LabelStmt{Name: label})
		}
		// User goto labels keep their source names.
		if ls.Case == cc.LabeledStatementLabel {
			name := ls.Token.SrcStr()
			stmts = append(stmts, &hir.LabelStmt{Name: name})
		}
		// Lower the statement after the label.
		if ls.Statement != nil {
			inner, err := fl.lowerStmt(ls.Statement)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, inner...)
		}
		return stmts, nil

	default:
		return nil, nil
	}
}

func (fl *funcLow) lowerExprStmt(es *cc.ExpressionStatement) ([]hir.Stmt, error) {
	if es == nil || es.ExpressionList == nil {
		return nil, nil
	}
	r, err := fl.lowerExpr(es.ExpressionList)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	s := r.toStmt()
	if s == nil {
		return nil, nil
	}
	return []hir.Stmt{s}, nil
}

func (fl *funcLow) lowerSelection(ss *cc.SelectionStatement) ([]hir.Stmt, error) {
	if ss == nil {
		return nil, nil
	}
	switch ss.Case {
	case cc.SelectionStatementIf:
		cond, err := fl.lowerExprAsExpr(ss.ExpressionList)
		if err != nil {
			return nil, err
		}
		then, err := fl.lowerStmt(ss.Statement)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{hir.If(cond, hir.Blk(then...), nil)}, nil

	case cc.SelectionStatementIfElse:
		cond, err := fl.lowerExprAsExpr(ss.ExpressionList)
		if err != nil {
			return nil, err
		}
		then, err := fl.lowerStmt(ss.Statement)
		if err != nil {
			return nil, err
		}
		els, err := fl.lowerStmt(ss.Statement2)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{hir.If(cond, hir.Blk(then...), hir.Blk(els...))}, nil

	case cc.SelectionStatementSwitch:
		return fl.lowerSwitch(ss)

	default:
		return nil, nil
	}
}

// cSwitchCase records source-ordered labels. Dispatch enters one label and
// normal control flow then executes every following statement until a break.
type cSwitchCase struct {
	val   hir.Expr
	label string
}

func (fl *funcLow) lowerSwitch(ss *cc.SelectionStatement) ([]hir.Stmt, error) {
	selector, err := fl.lowerExprAsExpr(ss.ExpressionList)
	if err != nil {
		return nil, err
	}
	selectorTy := fl.low.mapType(cc.IntegerPromotion(ss.ExpressionList.Type()))
	selector = cConvert(selector, selectorTy)
	var effects []hir.Stmt
	fl.drainPending(&effects)
	id := fl.tmpCounter()
	name := fmt.Sprintf("__switch_value_%d", id)
	end := fmt.Sprintf("__switch_end_%d", id)
	effects = append(effects, &hir.VarDeclStmt{Name: name, Ty: selector.ExprTy(), Init: selector})
	var cases []cSwitchCase
	oldCases, oldEnd := fl.switchCases, fl.switchEnd
	fl.switchCases, fl.switchEnd = &cases, end
	body, err := fl.lowerStmt(ss.Statement)
	fl.switchCases, fl.switchEnd = oldCases, oldEnd
	if err != nil {
		return nil, err
	}
	fallback := end
	for _, c := range cases {
		if c.val == nil {
			fallback = c.label
			continue
		}
		cond := &hir.BinExpr{Op: "==", L: hir.Var(name, selector.ExprTy()), R: cConvert(c.val, selectorTy), Ty: mir2.TyBool}
		effects = append(effects, hir.If(cond, hir.Blk(&hir.GotoStmt{Label: c.label}), nil))
	}
	effects = append(effects, &hir.GotoStmt{Label: fallback})
	effects = append(effects, body...)
	return append(effects, &hir.LabelStmt{Name: end}), nil
}

func (fl *funcLow) lowerIteration(is *cc.IterationStatement) ([]hir.Stmt, error) {
	if is == nil {
		return nil, nil
	}
	switch is.Case {
	case cc.IterationStatementWhile:
		cond, effects, err := fl.lowerControlExpr(is.ExpressionList)
		if err != nil {
			return nil, err
		}
		body, err := fl.lowerLoopBody(is.Statement, nil)
		if err != nil {
			return nil, err
		}
		if len(effects) == 0 {
			return []hir.Stmt{hir.While(cond, hir.Blk(body...))}, nil
		}
		test := hir.If(&hir.UnaryExpr{Op: "!", X: cond, Ty: mir2.TyBool}, hir.Blk(hir.Break()), nil)
		loop := append(effects, test)
		loop = append(loop, body...)
		return []hir.Stmt{hir.While(hir.Bool(true), hir.Blk(loop...))}, nil

	case cc.IterationStatementDo:
		test := func() ([]hir.Stmt, error) {
			cond, effects, err := fl.lowerControlExpr(is.ExpressionList)
			if err != nil {
				return nil, err
			}
			return append(effects, hir.If(&hir.UnaryExpr{Op: "!", X: cond, Ty: mir2.TyBool}, hir.Blk(hir.Break()), nil)), nil
		}
		body, err := fl.lowerLoopBody(is.Statement, fl.captureControlPrelude(test))
		if err != nil {
			return nil, err
		}
		cond, effects, err := fl.lowerControlExpr(is.ExpressionList)
		if err != nil {
			return nil, err
		}
		body = append(body, effects...)
		body = append(body, hir.If(&hir.UnaryExpr{Op: "!", X: cond, Ty: mir2.TyBool}, hir.Blk(hir.Break()), nil))
		return []hir.Stmt{hir.While(hir.Bool(true), hir.Blk(body...))}, nil

	case cc.IterationStatementFor:
		return fl.lowerForLoop(is, false)

	case cc.IterationStatementForDecl:
		return fl.lowerForLoop(is, true)

	default:
		return nil, nil
	}
}

func (fl *funcLow) lowerForLoop(is *cc.IterationStatement, isForDecl bool) ([]hir.Stmt, error) {
	var stmts []hir.Stmt

	// Init.
	if isForDecl {
		if is.Declaration != nil {
			s, err := fl.lowerLocalDecl(is.Declaration)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, s...)
		}
	} else if is.ExpressionList != nil {
		r, err := fl.lowerExpr(is.ExpressionList)
		if err != nil {
			return nil, err
		}
		fl.drainPending(&stmts)
		if r != nil {
			if s := r.toStmt(); s != nil {
				stmts = append(stmts, s)
			}
		}
	}

	// Condition and post differ based on for variant.
	var condNode, postNode cc.ExpressionNode
	if isForDecl {
		condNode = is.ExpressionList
		postNode = is.ExpressionList2
	} else {
		condNode = is.ExpressionList2
		postNode = is.ExpressionList3
	}

	// Condition.
	var cond hir.Expr
	var conditionEffects []hir.Stmt
	if condNode != nil {
		var err error
		cond, conditionEffects, err = fl.lowerControlExpr(condNode)
		if err != nil {
			return nil, err
		}
	} else {
		cond = hir.Bool(true)
	}

	// Continue evaluates post before testing the next condition.
	post := func() ([]hir.Stmt, error) {
		if postNode == nil {
			return nil, nil
		}
		outer := fl.pendingStmts
		fl.pendingStmts = nil
		r, err := fl.lowerExpr(postNode)
		effects := fl.pendingStmts
		fl.pendingStmts = outer
		if err != nil {
			return nil, err
		}
		if r != nil {
			if s := r.toStmt(); s != nil {
				effects = append(effects, s)
			}
		}
		return effects, nil
	}
	body, err := fl.lowerLoopBody(is.Statement, fl.captureControlPrelude(post))
	if err != nil {
		return nil, err
	}

	postEffects, err := post()
	if err != nil {
		return nil, err
	}
	body = append(body, postEffects...)

	if len(conditionEffects) > 0 {
		test := hir.If(&hir.UnaryExpr{Op: "!", X: cond, Ty: mir2.TyBool}, hir.Blk(hir.Break()), nil)
		body = append(append(conditionEffects, test), body...)
		cond = hir.Bool(true)
	}
	stmts = append(stmts, hir.While(cond, hir.Blk(body...)))
	return stmts, nil
}

func (fl *funcLow) lowerJump(js *cc.JumpStatement) ([]hir.Stmt, error) {
	if js == nil {
		return nil, nil
	}
	switch js.Case {
	case cc.JumpStatementReturn:
		if js.ExpressionList != nil {
			val, err := fl.lowerExprAsExpr(js.ExpressionList)
			if err != nil {
				return nil, err
			}
			val = fl.snapshotVolatileAggregate(val, js.ExpressionList.Type())
			if fl.retCType != nil {
				// Retain aggregate values: MIR supplies caller-owned storage
				// before the callee's local activation is released.
				if fl.retCType.Kind() != cc.Struct && fl.retCType.Kind() != cc.Union {
					val = fl.low.convertType(val, fl.retCType)
				}
			} else {
				val = cConvert(val, fl.retTy)
			}
			return []hir.Stmt{hir.Ret(val)}, nil
		}
		return []hir.Stmt{hir.RetVoid()}, nil

	case cc.JumpStatementBreak:
		if fl.switchEnd != "" {
			return []hir.Stmt{&hir.GotoStmt{Label: fl.switchEnd}}, nil
		}
		return []hir.Stmt{hir.Break()}, nil

	case cc.JumpStatementContinue:
		if fl.continuePrelude != nil {
			effects, err := fl.continuePrelude()
			if err != nil {
				return nil, err
			}
			return append(effects, hir.Continue()), nil
		}
		return []hir.Stmt{hir.Continue()}, nil

	case cc.JumpStatementGoto:
		label := js.Token2.SrcStr()
		return []hir.Stmt{&hir.GotoStmt{Label: label}}, nil

	default:
		return nil, nil
	}
}

// ── Expression lowering ─────────────────────────────────────────────────────

// exprResult wraps either a plain hir.Expr or an assignment (target + val).
type exprResult struct {
	expr      hir.Expr // non-nil for normal expressions
	target    hir.Expr // non-nil for assignments
	val       hir.Expr // non-nil for assignments
	isPostfix bool     // true for x++/x--
	oldValue  hir.Expr // the single cached read before a postfix update
}

func (r *exprResult) isAssign() bool { return r != nil && r.target != nil }

func (r *exprResult) toExpr() hir.Expr {
	if r == nil {
		return nil
	}
	if r.isAssign() {
		return r.val
	}
	return r.expr
}

func (r *exprResult) toStmt() hir.Stmt {
	if r == nil {
		return nil
	}
	if r.isAssign() {
		// *ptr = val → StoreStmt (not Assign to LoadExpr)
		if load, ok := r.target.(*hir.LoadExpr); ok {
			return &hir.StoreStmt{Ptr: load.Ptr, Val: r.val}
		}
		return hir.Assign(r.target, r.val)
	}
	if r.expr != nil {
		return &hir.ExprStmt{Expr: r.expr}
	}
	return nil
}

func wrapExpr(e hir.Expr) *exprResult      { return &exprResult{expr: e} }
func wrapAssign(t, v hir.Expr) *exprResult { return &exprResult{target: t, val: v} }

// lowerExprAsExpr is a convenience that returns hir.Expr.
// If the expression has a side-effect (e.g. ++x, a=b=c), the side-effect
// is emitted to fl.pendingStmts and the resulting value is returned.
func (fl *funcLow) lowerExprAsExpr(e cc.ExpressionNode) (hir.Expr, error) {
	r, err := fl.lowerExpr(e)
	if err != nil {
		return nil, err
	}
	if r != nil && r.isAssign() {
		if r.isPostfix {
			fl.pendingStmts = append(fl.pendingStmts, r.toStmt())
			return r.oldValue, nil
		}
		// Assignment expressions yield the converted assigned value, without
		// reading the destination again (particularly important for volatile).
		value := fl.cacheExpression("assigned", r.val)
		r.val = value
		fl.pendingStmts = append(fl.pendingStmts, r.toStmt())
		return value, nil
	}
	return r.toExpr(), nil
}

func (fl *funcLow) lowerExpr(e cc.ExpressionNode) (*exprResult, error) {
	r, err := fl.lowerExprNode(e)
	if err != nil || r == nil || e == nil || e.Type() == nil || !e.Type().Attributes().IsVolatile() {
		return r, err
	}
	switch x := r.expr.(type) {
	case *hir.VarRefExpr:
		if e.Type().Kind() != cc.Array {
			x.Volatile = true
		}
	case *hir.FieldExpr:
		x.Volatile = true
	case *hir.LoadExpr:
		x.Volatile = true
	case *hir.DerefExpr:
		x.Volatile = true
	case *hir.IndexExpr:
		x.Volatile = true
	}
	return r, nil
}

func (fl *funcLow) lowerExprNode(e cc.ExpressionNode) (*exprResult, error) {
	if e == nil {
		return nil, nil
	}

	switch x := e.(type) {
	case *cc.PrimaryExpression:
		return fl.lowerPrimary(x)

	case *cc.PostfixExpression:
		return fl.lowerPostfix(x)

	case *cc.UnaryExpression:
		return fl.lowerUnary(x)

	case *cc.CastExpression:
		switch x.Case {
		case cc.CastExpressionUnary:
			return fl.lowerExpr(x.UnaryExpression)
		case cc.CastExpressionCast:
			inner, err := fl.lowerExprAsExpr(x.CastExpression)
			if err != nil {
				return nil, err
			}
			return wrapExpr(fl.low.convertType(inner, x.Type())), nil
		}
		return nil, nil

	case *cc.MultiplicativeExpression:
		return fl.lowerBinaryOp(x.MultiplicativeExpression, x.CastExpression, x.Case, x.Type())

	case *cc.AdditiveExpression:
		return fl.lowerBinaryOp(x.AdditiveExpression, x.MultiplicativeExpression, x.Case, x.Type())

	case *cc.ShiftExpression:
		return fl.lowerBinaryOp(x.ShiftExpression, x.AdditiveExpression, x.Case, x.Type())

	case *cc.RelationalExpression:
		return fl.lowerBinaryOp(x.RelationalExpression, x.ShiftExpression, x.Case, x.Type())

	case *cc.EqualityExpression:
		return fl.lowerBinaryOp(x.EqualityExpression, x.RelationalExpression, x.Case, x.Type())

	case *cc.AndExpression:
		return fl.lowerBinaryOp(x.AndExpression, x.EqualityExpression, x.Case, x.Type())

	case *cc.ExclusiveOrExpression:
		return fl.lowerBinaryOp(x.ExclusiveOrExpression, x.AndExpression, x.Case, x.Type())

	case *cc.InclusiveOrExpression:
		return fl.lowerBinaryOp(x.InclusiveOrExpression, x.ExclusiveOrExpression, x.Case, x.Type())

	case *cc.LogicalAndExpression:
		if x.LogicalAndExpression != nil && x.InclusiveOrExpression != nil {
			// a && b → short-circuit: if a then (b != 0) else 0
			l, err := fl.lowerExprAsExpr(x.LogicalAndExpression)
			if err != nil {
				return nil, err
			}
			r, effects, err := fl.lowerControlExpr(x.InclusiveOrExpression)
			if err != nil {
				return nil, err
			}
			r = &hir.BinExpr{Op: "!=", L: r, R: &hir.IntLitExpr{Val: 0, Ty: r.ExprTy()}, Ty: mir2.TyBool}
			return wrapExpr(fl.lowerConditionalValue(l, r, &hir.IntLitExpr{Val: 0, Ty: mir2.TyBool}, effects, nil, mir2.TyBool)), nil
		}
		return fl.lowerExpr(x.InclusiveOrExpression)

	case *cc.LogicalOrExpression:
		if x.LogicalOrExpression != nil && x.LogicalAndExpression != nil {
			// a || b → short-circuit: if a then 1 else (b != 0)
			l, err := fl.lowerExprAsExpr(x.LogicalOrExpression)
			if err != nil {
				return nil, err
			}
			r, effects, err := fl.lowerControlExpr(x.LogicalAndExpression)
			if err != nil {
				return nil, err
			}
			r = &hir.BinExpr{Op: "!=", L: r, R: &hir.IntLitExpr{Val: 0, Ty: r.ExprTy()}, Ty: mir2.TyBool}
			return wrapExpr(fl.lowerConditionalValue(l, &hir.IntLitExpr{Val: 1, Ty: mir2.TyBool}, r, nil, effects, mir2.TyBool)), nil
		}
		return fl.lowerExpr(x.LogicalAndExpression)

	case *cc.ConditionalExpression:
		if x.LogicalOrExpression != nil && x.ExpressionList == nil {
			return fl.lowerExpr(x.LogicalOrExpression)
		}
		cond, err := fl.lowerExprAsExpr(x.LogicalOrExpression)
		if err != nil {
			return nil, err
		}
		then, thenEffects, err := fl.lowerControlExpr(x.ExpressionList)
		if err != nil {
			return nil, err
		}
		els, elseEffects, err := fl.lowerControlExpr(x.ConditionalExpression)
		if err != nil {
			return nil, err
		}
		return wrapExpr(fl.lowerConditionalValue(cond, then, els, thenEffects, elseEffects, fl.low.mapType(x.Type()))), nil

	case *cc.ConstantExpression:
		return fl.lowerExpr(x.ConditionalExpression)

	case *cc.AssignmentExpression:
		return fl.lowerAssignment(x)

	case *cc.ExpressionList:
		// Comma expression — evaluate all (emit side-effects), return last.
		var last *exprResult
		for el := x; el != nil; el = el.ExpressionList {
			// Emit side-effects from previous iteration.
			if last != nil && last.isAssign() {
				fl.pendingStmts = append(fl.pendingStmts, last.toStmt())
			}
			var err error
			last, err = fl.lowerExpr(el.AssignmentExpression)
			if err != nil {
				return nil, err
			}
		}
		return last, nil

	default:
		return nil, fmt.Errorf("unsupported expression type %T", e)
	}
}

func (fl *funcLow) lowerPrimary(pe *cc.PrimaryExpression) (*exprResult, error) {
	switch pe.Case {
	case cc.PrimaryExpressionIdent:
		tok := pe.Token
		name := tok.SrcStr()
		// Enum constants: check if identifier resolves to a constant value.
		// cc/v4 may type enum constants as Int (not Enum), so check Value() for
		// identifiers that aren't known locals, globals, or functions.
		if _, isLocal := fl.locals[name]; !isLocal {
			if _, isGlobal := fl.low.globals[name]; !isGlobal {
				t := pe.Type()
				isFunc := t != nil && (t.Kind() == cc.Function || t.Kind() == cc.Ptr)
				if !isFunc {
					if v := pe.Value(); v != nil {
						if _, unknown := v.(*cc.UnknownValue); !unknown {
							ty := fl.low.mapType(t)
							return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: ty}), nil
						}
					}
				}
			}
		}
		// Apply static local rename if applicable.
		resolvedName := name
		if fl.staticRenames != nil {
			if mangled, ok := fl.staticRenames[name]; ok {
				resolvedName = mangled
			}
		}
		// Function name used as value → take address (function-to-pointer decay).
		// In C, `fp = add` is equivalent to `fp = &add`.
		if _, isLocal := fl.locals[resolvedName]; !isLocal {
			if _, isGlobal := fl.low.globals[resolvedName]; !isGlobal {
				if fl.low.isFunction(resolvedName) {
					return wrapExpr(hir.Addr(fl.low.symbolName(resolvedName))), nil
				}
			}
		}
		ty := fl.typeOf(name)
		// Array storage retains its aggregate width, but the expression decays
		// to a pointer (C99 6.3.2.1). Casting the storage width to a pointer
		// otherwise makes native truncation throw away address bits.
		if _, array := ty.(*mir2.ArrayTy); array {
			ty = mir2.TyPtr
		}
		if _, local := fl.locals[resolvedName]; !local {
			resolvedName = fl.low.symbolName(resolvedName)
		}
		return wrapExpr(hir.Var(resolvedName, ty)), nil

	case cc.PrimaryExpressionInt:
		v := pe.Value()
		ty := fl.low.mapType(pe.Type())
		if ty == nil {
			ty = mir2.TyI16 // fallback for untyped contexts (e.g. ObjC)
		}
		switch vv := v.(type) {
		case nil:
			// No value at all — parse from token.
			return wrapExpr(&hir.IntLitExpr{Val: parseIntToken(pe.Token.SrcStr()), Ty: ty}), nil
		case *cc.UnknownValue:
			// Semantic analysis didn't resolve (ObjC method bodies) — parse from token.
			_ = vv
			return wrapExpr(&hir.IntLitExpr{Val: parseIntToken(pe.Token.SrcStr()), Ty: ty}), nil
		default:
			return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: ty}), nil
		}

	case cc.PrimaryExpressionFloat:
		return nil, fmt.Errorf("c89: unsupported floating-point expression at %s", pe.Position())

	case cc.PrimaryExpressionChar:
		v := pe.Value()
		if v == nil {
			return wrapExpr(hir.U8(0)), nil
		}
		return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: mir2.TyU8}), nil

	case cc.PrimaryExpressionString:
		value, ok := pe.Value().(cc.StringValue)
		if !ok {
			return nil, fmt.Errorf("unsupported string literal encoding at %s", pe.Position())
		}
		return wrapExpr(hir.Addr(fl.low.internCString(value))), nil

	case cc.PrimaryExpressionExpr:
		return fl.lowerExpr(pe.ExpressionList)

	case cc.PrimaryExpressionGeneric:
		// C11 _Generic: type-based dispatch, resolved at compile time.
		gs := pe.GenericSelection
		if gs == nil {
			return wrapExpr(hir.U8(0)), nil
		}
		// Get controlling expression type.
		ctrlTy := gs.AssignmentExpression.Type()
		// Walk associations to find matching type or default.
		var defaultExpr cc.ExpressionNode
		for gal := gs.GenericAssociationList; gal != nil; gal = gal.GenericAssociationList {
			ga := gal.GenericAssociation
			if ga == nil {
				continue
			}
			if ga.Case == cc.GenericAssociationDefault {
				defaultExpr = ga.AssignmentExpression
				continue
			}
			// Type association: compare types.
			if ga.TypeName != nil && ctrlTy != nil {
				assocTy := ga.TypeName.Type()
				if assocTy != nil && assocTy.Kind() == ctrlTy.Kind() {
					return fl.lowerExpr(ga.AssignmentExpression)
				}
			}
		}
		// No match — use default.
		if defaultExpr != nil {
			return fl.lowerExpr(defaultExpr)
		}
		return wrapExpr(hir.U8(0)), nil

	case cc.PrimaryExpressionObjCMessage:
		if pe.ObjCMessage != nil {
			return fl.lowerObjCMessage(pe.ObjCMessage)
		}
		return wrapExpr(hir.U8(0)), nil

	default:
		return wrapExpr(hir.U8(0)), nil
	}
}

func (fl *funcLow) lowerPostfix(pf *cc.PostfixExpression) (*exprResult, error) {
	switch pf.Case {
	case cc.PostfixExpressionPrimary:
		return fl.lowerExpr(pf.PrimaryExpression)

	case cc.PostfixExpressionCall:
		return fl.lowerCall(pf)

	case cc.PostfixExpressionIndex:
		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		idx, err := fl.lowerExprAsExpr(pf.ExpressionList)
		if err != nil {
			return nil, err
		}
		elementType := pf.Type()
		if pointer, ok := pf.PostfixExpression.Type().(*cc.PointerType); ok {
			elementType = pointer.Elem()
		} else if pointer, ok := pf.ExpressionList.Type().(*cc.PointerType); ok {
			base, idx = idx, base
			elementType = pointer.Elem()
		}
		elemTy := fl.low.storageType(elementType)
		indexed := &hir.IndexExpr{Base: base, Idx: idx, ElemTy: elemTy, ElemStride: int(elementType.Size())}
		if elementType.Kind() == cc.Array {
			return wrapExpr(hir.AddrExpr(indexed)), nil
		}
		return wrapExpr(indexed), nil

	case cc.PostfixExpressionSelect: // struct.field
		if field := pf.Field(); field != nil && field.IsBitfield() {
			return nil, fmt.Errorf("bit-field access is not supported on Z80")
		}

		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		field := pf.Token2.SrcStr()
		ty := fl.low.mapType(pf.Type())
		if field := pf.Field(); field != nil && (field.Type().Kind() == cc.Array || field.Type().Kind() == cc.Struct || field.Type().Kind() == cc.Union) {
			ty = fl.low.storageFieldType(field)
		}
		// Prefer cc parser's field offset (handles anonymous structs/unions correctly).
		offset := int(pf.FieldOffset())
		if offset < 0 {
			offset = fl.resolveFieldOffset(pf.PostfixExpression.Type(), field)
		}
		// Flatten nested struct access: if base is a FieldExpr for an embedded struct,
		// combine offsets instead of nesting (FieldExpr always does Load, which is wrong
		// for embedded struct intermediate access).
		if fe, ok := base.(*hir.FieldExpr); ok {
			return wrapExpr(cFieldValue(&hir.FieldExpr{X: fe.X, Field: field, Offset: fe.Offset + offset, Ty: ty})), nil
		}
		return wrapExpr(cFieldValue(&hir.FieldExpr{X: base, Field: field, Offset: offset, Ty: ty})), nil

	case cc.PostfixExpressionPSelect: // struct->field
		if field := pf.Field(); field != nil && field.IsBitfield() {
			return nil, fmt.Errorf("bit-field access is not supported on Z80")
		}

		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		field := pf.Token2.SrcStr()
		ty := fl.low.mapType(pf.Type())
		if field := pf.Field(); field != nil && (field.Type().Kind() == cc.Array || field.Type().Kind() == cc.Struct || field.Type().Kind() == cc.Union) {
			ty = fl.low.storageFieldType(field)
		}
		offset := int(pf.FieldOffset())
		if offset < 0 {
			offset = fl.resolveFieldOffset(pf.PostfixExpression.Type(), field)
		}
		// Flatten: ptr->inner.field where inner is embedded.
		if fe, ok := base.(*hir.FieldExpr); ok {
			return wrapExpr(cFieldValue(&hir.FieldExpr{X: fe.X, Field: field, Offset: fe.Offset + offset, Ty: ty})), nil
		}
		return wrapExpr(cFieldValue(&hir.FieldExpr{X: base, Field: field, Offset: offset, Ty: ty})), nil

	case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		step := fl.incrementStep(pf.PostfixExpression.Type())
		if pf.Case == cc.PostfixExpressionDec {
			step = -step
		}
		return fl.lowerIncrement(base, pf.PostfixExpression.Type(), step, true), nil

	case cc.PostfixExpressionComplit: // (Type){init-list} — compound literal
		ty := pf.Type()
		if st := fl.low.resolveStructType(ty); st != nil {
			lit, err := fl.lowerStructInit(st, pf.InitializerList)
			if err != nil {
				return nil, err
			}
			return wrapExpr(lit), nil
		}
		// Non-struct compound literal (scalar) — lower the first init value.
		if pf.InitializerList != nil && pf.InitializerList.Initializer != nil {
			init := pf.InitializerList.Initializer
			if init.Case == cc.InitializerExpr && init.AssignmentExpression != nil {
				return fl.lowerExpr(init.AssignmentExpression)
			}
		}
		return wrapExpr(&hir.IntLitExpr{Val: 0, Ty: mir2.TyI16}), nil

	default:
		return fl.lowerExpr(pf.PrimaryExpression)
	}
}

// C99 6.3.2.1: an array field designates inline storage, and ordinary
// value use decays to a pointer to its first element. AddrExpr keeps the
// pointer width independent of the full aggregate width.
func cFieldValue(field *hir.FieldExpr) hir.Expr {
	if _, array := mir2.BaseOf(field.Ty).(*mir2.ArrayTy); array {
		return hir.AddrExpr(field)
	}
	return field
}

// extractCalleeName tries to extract a simple identifier from a C expression node.
// Returns "" if the callee is a complex expression.
func (fl *funcLow) extractCalleeName(expr cc.ExpressionNode) string {
	switch e := expr.(type) {
	case *cc.PostfixExpression:
		if e.Case == cc.PostfixExpressionPrimary {
			return fl.extractCalleeName(e.PrimaryExpression)
		}
	case *cc.PrimaryExpression:
		if e.Case == cc.PrimaryExpressionIdent {
			name := e.Token.SrcStr()
			// Apply static rename if applicable.
			if fl.staticRenames != nil {
				if mangled, ok := fl.staticRenames[name]; ok {
					return mangled
				}
			}
			return name
		}
	case *cc.UnaryExpression:
		// Handle `(*fp)(args)` → deref of a function pointer.
		if e.Case == cc.UnaryExpressionPostfix {
			return fl.extractCalleeName(e.PostfixExpression)
		}
	case *cc.CastExpression:
		if e.Case == cc.CastExpressionUnary {
			return fl.extractCalleeName(e.UnaryExpression)
		}
	}
	return ""
}

// Physical aggregate ABI values are addresses. Materialize a qualified source
// before it crosses that ABI, so callee copies cannot lose source effects.
func (fl *funcLow) snapshotVolatileAggregate(value hir.Expr, cType cc.Type) hir.Expr {
	if cType == nil || (cType.Kind() != cc.Struct && cType.Kind() != cc.Union) || !cType.Attributes().IsVolatile() {
		return value
	}
	pointer := value
	if !mir2.IsPointer(value.ExprTy()) {
		pointer = hir.AddrExpr(value)
	}
	object := &hir.DerefExpr{Ptr: pointer, Ty: fl.low.storageType(cType), Volatile: true}
	return fl.cacheExpression("volatile_value", object)
}

func (fl *funcLow) lowerCall(pf *cc.PostfixExpression) (*exprResult, error) {
	calleeTy := pf.PostfixExpression.Type()
	if ptr, ok := calleeTy.(*cc.PointerType); ok {
		calleeTy = ptr.Elem()
	}
	ft, _ := calleeTy.(*cc.FunctionType)
	if ft != nil && ft.IsVariadic() {
		return nil, fmt.Errorf("variadic call at %s requires an unsupported Z80 varargs ABI", pf.Position())
	}
	// Lower arguments using the declared parameter type, not their original
	// expression width. This also defines the ABI for cross-language calls.
	var args []hir.Expr
	for al := pf.ArgumentExpressionList; al != nil; al = al.ArgumentExpressionList {
		arg, err := fl.lowerExprAsExpr(al.AssignmentExpression)
		if err != nil {
			return nil, err
		}
		arg = fl.snapshotVolatileAggregate(arg, al.AssignmentExpression.Type())
		if ft != nil && len(args) < len(ft.Parameters()) {
			arg = fl.low.convertType(arg, ft.Parameters()[len(args)].Type())
		} else if cc.IsIntegerType(al.AssignmentExpression.Type()) {
			arg = cConvert(arg, fl.low.mapType(cc.IntegerPromotion(al.AssignmentExpression.Type())))
		}
		args = append(args, arg)
	}

	retTy := fl.low.mapType(pf.Type())
	if pf.Type().Kind() == cc.Struct || pf.Type().Kind() == cc.Union {
		retTy = fl.low.storageType(pf.Type())
	}
	indirect := func(fn hir.Expr) *exprResult {
		sig := &hir.CallSignature{}
		if ft != nil {
			for _, p := range ft.Parameters() {
				if p.Type().Kind() != cc.Void {
					sig.Params = append(sig.Params, fl.low.mapType(p.Type()))
				}
			}
		}
		if len(sig.Params) == 0 && len(args) > 0 {
			for _, arg := range args {
				sig.Params = append(sig.Params, arg.ExprTy())
			}
		}
		if retTy != mir2.TyVoid {
			sig.Returns = []mir2.Ty{retTy}
		}
		sig.ResolveStructReturnABI()
		return wrapExpr(&hir.CallIndirectExpr{Signature: sig, FnPtr: fn, Args: args, Ty: retTy})
	}

	// Try to extract a direct function name from the callee AST node.
	// This avoids going through lowerExpr which may convert function names to AddrOfExpr.
	if name := fl.extractCalleeName(pf.PostfixExpression); name != "" {
		if ty, isGlobal := fl.low.globals[name]; isGlobal && mir2.IsPointer(ty) {
			return indirect(hir.Var(fl.low.symbolName(name), mir2.TyPtr)), nil
		}
		if _, isLocal := fl.locals[name]; isLocal {
			// Local variable or parameter holding a function pointer → indirect call.
			fnPtr := &hir.VarRefExpr{Name: name, Ty: mir2.TyPtr}
			return indirect(fnPtr), nil
		}
		// Function name → direct call.
		return wrapExpr(hir.Call(fl.low.symbolName(name), retTy, args...)), nil
	}

	// Callee is a complex expression → lower it and do indirect call.
	fnExpr, err := fl.lowerExprAsExpr(pf.PostfixExpression)
	if err != nil {
		return nil, err
	}
	return indirect(fnExpr), nil
}

func (fl *funcLow) lowerUnary(ue *cc.UnaryExpression) (*exprResult, error) {
	switch ue.Case {
	case cc.UnaryExpressionPostfix:
		return fl.lowerExpr(ue.PostfixExpression)

	case cc.UnaryExpressionSizeofExpr:
		if v := ue.Value(); v != nil {
			return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: mir2.TyU16}), nil
		}
		return nil, fmt.Errorf("runtime sizeof is not supported by this target")

	case cc.UnaryExpressionSizeofType:
		if v := ue.Value(); v != nil {
			return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: mir2.TyU16}), nil
		}
		return nil, fmt.Errorf("runtime sizeof is not supported by this target")

	case cc.UnaryExpressionAlignofExpr, cc.UnaryExpressionAlignofType:
		// Z80 is byte-addressed — alignment is always 1 for all types.
		return wrapExpr(&hir.IntLitExpr{Val: 1, Ty: mir2.TyU16}), nil

	case cc.UnaryExpressionAddrof: // &x
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		if v, ok := inner.(*hir.VarRefExpr); ok {
			return wrapExpr(hir.Addr(v.Name)), nil
		}
		if _, function := inner.(*hir.AddrOfExpr); function && ue.CastExpression.Type().Kind() == cc.Function {
			return wrapExpr(inner), nil // function names already lower to their address
		}
		if deref, ok := inner.(*hir.DerefExpr); ok {
			return wrapExpr(deref.Ptr), nil // &*p does not access the pointed-to object
		}
		return wrapExpr(hir.AddrExpr(inner)), nil

	case cc.UnaryExpressionDeref: // *x
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		ty := fl.low.mapType(ue.Type())
		if ue.Type().Kind() == cc.Struct || ue.Type().Kind() == cc.Union {
			ty = fl.low.storageType(ue.Type())
		}
		return wrapExpr(&hir.DerefExpr{Ptr: inner, Ty: ty}), nil

	case cc.UnaryExpressionMinus:
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		ty := fl.low.mapType(ue.Type())
		return wrapExpr(&hir.UnaryExpr{Op: "-", X: cConvert(inner, ty), Ty: ty}), nil

	case cc.UnaryExpressionPlus:
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		return wrapExpr(cConvert(inner, fl.low.mapType(ue.Type()))), nil

	case cc.UnaryExpressionCpl:
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		ty := fl.low.mapType(ue.Type())
		return wrapExpr(&hir.UnaryExpr{Op: "~", X: cConvert(inner, ty), Ty: ty}), nil

	case cc.UnaryExpressionNot:
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		return wrapExpr(&hir.UnaryExpr{Op: "!", X: inner, Ty: mir2.TyBool}), nil

	case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
		inner, err := fl.lowerExprAsExpr(ue.UnaryExpression)
		if err != nil {
			return nil, err
		}
		step := fl.incrementStep(ue.UnaryExpression.Type())
		if ue.Case == cc.UnaryExpressionDec {
			step = -step
		}
		return fl.lowerIncrement(inner, ue.UnaryExpression.Type(), step, false), nil

	default:
		return fl.lowerExpr(ue.CastExpression)
	}
}

// Cache the object address separately from its value: C compound assignment
// and increment evaluate their lvalue once, even when it contains a call.
func (fl *funcLow) stabilizeLvalue(target hir.Expr) hir.Expr {
	var address hir.Expr
	volatile := false
	switch x := target.(type) {
	case *hir.VarRefExpr:
		return target
	case *hir.DerefExpr:
		address, volatile = x.Ptr, x.Volatile
	case *hir.LoadExpr:
		address, volatile = x.Ptr, x.Volatile
	case *hir.FieldExpr:
		address, volatile = hir.AddrExpr(x), x.Volatile
	case *hir.IndexExpr:
		address, volatile = hir.AddrExpr(x), x.Volatile
	default:
		return target
	}
	pointer := fl.cacheExpression("lvalue", address)
	return &hir.DerefExpr{Ptr: pointer, Ty: target.ExprTy(), Volatile: volatile}
}

func (fl *funcLow) cacheExpression(kind string, value hir.Expr) hir.Expr {
	name := fmt.Sprintf("__%s_%d", kind, fl.tmpCounter())
	ty := value.ExprTy()
	fl.locals[name] = ty
	fl.pendingStmts = append(fl.pendingStmts, &hir.VarDeclStmt{Name: name, Ty: ty, Init: value})
	return hir.Var(name, ty)
}

func (fl *funcLow) lowerIncrement(target hir.Expr, cType cc.Type, step int64, postfix bool) *exprResult {
	target = fl.stabilizeLvalue(target)
	old := fl.cacheExpression("before_update", target)
	ty := target.ExprTy()
	value := fl.low.convertType(hir.Add(old, &hir.IntLitExpr{Val: step, Ty: ty}, ty), cType)
	return &exprResult{target: target, val: value, isPostfix: postfix, oldValue: old}
}

func (fl *funcLow) lowerAssignment(ae *cc.AssignmentExpression) (*exprResult, error) {
	if ae.ConditionalExpression != nil && ae.AssignmentExpression == nil {
		return fl.lowerExpr(ae.ConditionalExpression)
	}

	target, err := fl.lowerExprAsExpr(ae.UnaryExpression)
	if err != nil {
		return nil, err
	}
	target = fl.stabilizeLvalue(target)
	val, err := fl.lowerExprAsExpr(ae.AssignmentExpression)
	if err != nil {
		return nil, err
	}

	ty := fl.low.mapType(ae.Type())
	operand, opTy := target, ty
	if ae.Case != cc.AssignmentExpressionAssign && cc.IsIntegerType(ae.UnaryExpression.Type()) && cc.IsIntegerType(ae.AssignmentExpression.Type()) {
		if ae.Case == cc.AssignmentExpressionLsh || ae.Case == cc.AssignmentExpressionRsh {
			opTy = fl.low.mapType(cc.IntegerPromotion(ae.UnaryExpression.Type()))
			operand = cConvert(target, opTy)
			val = cConvert(val, fl.low.mapType(cc.IntegerPromotion(ae.AssignmentExpression.Type())))
		} else {
			opTy = fl.low.mapType(cc.UsualArithmeticConversions(ae.UnaryExpression.Type(), ae.AssignmentExpression.Type()))
			operand, val = cConvert(target, opTy), cConvert(val, opTy)
		}
	}
	if stride, pointer := fl.pointerStride(ae.UnaryExpression.Type()); pointer && (ae.Case == cc.AssignmentExpressionAdd || ae.Case == cc.AssignmentExpressionSub) {
		val = fl.pointerOffset(val, stride)
	}
	switch ae.Case {
	case cc.AssignmentExpressionAdd:
		val = hir.Add(operand, val, opTy)
	case cc.AssignmentExpressionSub:
		val = hir.Sub(operand, val, opTy)
	case cc.AssignmentExpressionMul:
		val = hir.Mul(operand, val, opTy)
	case cc.AssignmentExpressionAnd:
		val = &hir.BinExpr{Op: "&", L: operand, R: val, Ty: opTy}
	case cc.AssignmentExpressionOr:
		val = &hir.BinExpr{Op: "|", L: operand, R: val, Ty: opTy}
	case cc.AssignmentExpressionXor:
		val = &hir.BinExpr{Op: "^", L: operand, R: val, Ty: opTy}
	case cc.AssignmentExpressionLsh:
		val = &hir.BinExpr{Op: "<<", L: operand, R: val, Ty: opTy}
	case cc.AssignmentExpressionRsh:
		val = &hir.BinExpr{Op: ">>", L: operand, R: val, Ty: opTy}
	case cc.AssignmentExpressionDiv:
		val = &hir.BinExpr{Op: "/", L: operand, R: val, Ty: opTy}
	case cc.AssignmentExpressionMod:
		val = &hir.BinExpr{Op: "%", L: operand, R: val, Ty: opTy}
	}

	return wrapAssign(target, fl.low.convertType(val, ae.Type())), nil
}

// lowerBinaryOp handles all binary expression types via the cc.Case value.
func (fl *funcLow) lowerBinaryOp(left, right cc.ExpressionNode, caseVal interface{}, ty cc.Type) (*exprResult, error) {
	if left == nil {
		return fl.lowerExpr(right)
	}

	l, err := fl.lowerExprAsExpr(left)
	if err != nil {
		return nil, err
	}
	r, err := fl.lowerExprAsExpr(right)
	if err != nil {
		return nil, err
	}

	mty := fl.low.mapType(ty)

	op := ""
	switch v := caseVal.(type) {
	case cc.MultiplicativeExpressionCase:
		switch v {
		case cc.MultiplicativeExpressionMul:
			op = "*"
		case cc.MultiplicativeExpressionDiv:
			op = "/"
		case cc.MultiplicativeExpressionMod:
			op = "%"
		}
	case cc.AdditiveExpressionCase:
		switch v {
		case cc.AdditiveExpressionAdd:
			op = "+"
		case cc.AdditiveExpressionSub:
			op = "-"
		}
	case cc.ShiftExpressionCase:
		switch v {
		case cc.ShiftExpressionLsh:
			op = "<<"
		case cc.ShiftExpressionRsh:
			op = ">>"
		}
	case cc.RelationalExpressionCase:
		mty = mir2.TyBool
		switch v {
		case cc.RelationalExpressionLt:
			op = "<"
		case cc.RelationalExpressionGt:
			op = ">"
		case cc.RelationalExpressionLeq:
			op = "<="
		case cc.RelationalExpressionGeq:
			op = ">="
		}
	case cc.EqualityExpressionCase:
		mty = mir2.TyBool
		switch v {
		case cc.EqualityExpressionEq:
			op = "=="
		case cc.EqualityExpressionNeq:
			op = "!="
		}
	case cc.AndExpressionCase:
		op = "&"
	case cc.ExclusiveOrExpressionCase:
		op = "^"
	case cc.InclusiveOrExpressionCase:
		op = "|"
	}

	if op == "" {
		return wrapExpr(l), nil
	}

	// C99 6.5.6: pointer offsets and differences use the pointed-to size.
	if op == "+" || op == "-" {
		leftStride, leftPointer := fl.pointerStride(left.Type())
		rightStride, rightPointer := fl.pointerStride(right.Type())
		if leftPointer && rightPointer && op == "-" {
			return wrapExpr(fl.pointerDifference(l, r, leftStride)), nil
		}
		if leftPointer && cc.IsIntegerType(right.Type()) {
			r = fl.pointerOffset(r, leftStride)
		} else if rightPointer && cc.IsIntegerType(left.Type()) && op == "+" {
			l = fl.pointerOffset(l, rightStride)
		}
	}

	// C integer promotions and usual arithmetic conversions belong to the
	// operands, including comparisons whose result type is only bool.
	// C99 6.3.1.1, 6.3.1.8 and 6.5.7: shifts promote each operand separately.
	if cc.IsIntegerType(left.Type()) && cc.IsIntegerType(right.Type()) {
		if op == "<<" || op == ">>" {
			l = hir.Cast(l, fl.low.mapType(cc.IntegerPromotion(left.Type())))
			r = hir.Cast(r, fl.low.mapType(cc.IntegerPromotion(right.Type())))
		} else {
			common := fl.low.mapType(cc.UsualArithmeticConversions(left.Type(), right.Type()))
			l, r = hir.Cast(l, common), hir.Cast(r, common)
		}
	}

	return wrapExpr(&hir.BinExpr{Op: op, L: l, R: r, Ty: mty}), nil
}

// pointerStride also accepts arrays before the parser's decay conversion.
func (fl *funcLow) pointerStride(ty cc.Type) (int64, bool) {
	var elem cc.Type
	switch t := ty.(type) {
	case *cc.PointerType:
		elem = t.Elem()
	case *cc.ArrayType:
		elem = t.Elem()
	default:
		return 0, false
	}
	if elem.Size() <= 0 {
		if fl.low.typeError == nil {
			fl.low.typeError = fmt.Errorf("c89: pointer arithmetic requires a complete object type, got %s", ty)
		}
		return 1, true
	}
	return elem.Size(), true
}

func (fl *funcLow) incrementStep(ty cc.Type) int64 {
	if stride, pointer := fl.pointerStride(ty); pointer {
		return stride
	}
	return 1
}

// The byte distance can exceed signed int even when the element distance
// fits ptrdiff_t. Divide an unsigned magnitude, then apply the direction.
func (fl *funcLow) pointerDifference(left, right hir.Expr, stride int64) hir.Expr {
	if stride == 1 {
		return hir.Sub(cConvert(left, mir2.TyI16), cConvert(right, mir2.TyI16), mir2.TyI16)
	}
	values := []hir.Expr{left, right}
	for i, value := range values {
		name := fmt.Sprintf("__ptrdiff_%d", fl.tmpCounter())
		fl.locals[name] = mir2.TyU16
		fl.pendingStmts = append(fl.pendingStmts, &hir.VarDeclStmt{Name: name, Ty: mir2.TyU16, Init: cConvert(value, mir2.TyU16)})
		values[i] = hir.Var(name, mir2.TyU16)
	}
	left, right = values[0], values[1]
	quotient := func(a, b hir.Expr) hir.Expr {
		return cConvert(&hir.BinExpr{Op: "/", L: hir.Sub(a, b, mir2.TyU16), R: &hir.IntLitExpr{Val: stride, Ty: mir2.TyU16}, Ty: mir2.TyU16}, mir2.TyI16)
	}
	return &hir.CondExpr{Cond: &hir.BinExpr{Op: ">=", L: left, R: right, Ty: mir2.TyBool}, Then: quotient(left, right), Else: &hir.UnaryExpr{Op: "-", X: quotient(right, left), Ty: mir2.TyI16}, Ty: mir2.TyI16}
}

func (fl *funcLow) pointerOffset(offset hir.Expr, stride int64) hir.Expr {
	offset = cConvert(offset, mir2.TyI16)
	if stride > 1 {
		return hir.Mul(offset, &hir.IntLitExpr{Val: stride, Ty: mir2.TyI16}, mir2.TyI16)
	}
	return offset
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (fl *funcLow) typeOf(name string) mir2.Ty {
	if ty, ok := fl.locals[name]; ok {
		return ty
	}
	if ty, ok := fl.low.globals[name]; ok {
		return ty
	}
	return mir2.TyU16
}

func (l *lowerer) evalConst(e cc.ExpressionNode) (int64, bool) {
	if e == nil {
		return 0, false
	}
	v := e.Value()
	if v == nil {
		return 0, false
	}
	return constToInt64(v), true
}

// resolveStructType returns the mir2.StructTy for a cc.Type if it's a known struct.
func (l *lowerer) resolveStructType(t cc.Type) *mir2.StructTy {
	if t == nil || (t.Kind() != cc.Struct && t.Kind() != cc.Union) {
		return nil
	}
	storage, _ := l.storageType(t).(*mir2.StructTy)
	return storage
}

// resolveFieldOffset determines the byte offset of a named field within a struct.
// cTy is the C type of the base expression (struct type for '.', pointer-to-struct for '->').
func (fl *funcLow) resolveFieldOffset(cTy cc.Type, field string) int {
	if cTy == nil {
		return 0
	}

	// For pointer-to-struct (->), unwrap the pointer.
	if cTy.Kind() == cc.Ptr {
		if pt, ok := cTy.(*cc.PointerType); ok {
			cTy = pt.Elem()
		}
	}

	// Try resolving via C struct type → mir2 StructTy.
	if mst := fl.low.resolveStructType(cTy); mst != nil {
		for i, f := range mst.Fields {
			if f.Name == field {
				return mst.ByteOffset(i)
			}
		}
		// Flattened embedded struct: field "origin" → match "origin." prefix.
		prefix := field + "."
		for i, f := range mst.Fields {
			if strings.HasPrefix(f.Name, prefix) {
				return mst.ByteOffset(i)
			}
		}
	}

	// Fallback: check ObjC class structs (for self->field access).
	if fl.objcClass != nil {
		if st := fl.low.structs[fl.objcClass.name]; st != nil {
			for i, f := range st.Fields {
				if f.Name == field {
					return st.ByteOffset(i)
				}
			}
		}
	}

	return 0
}

// lowerStructInit lowers a brace initializer list to a StructLitExpr.
// Handles both positional { a, b } and designated { .q = a, .r = b } forms.
func (fl *funcLow) lowerStructInit(st *mir2.StructTy, il *cc.InitializerList) (*hir.StructLitExpr, error) {
	lit := &hir.StructLitExpr{St: st}
	fieldIdx := 0
	for ; il != nil; il = il.InitializerList {
		init := il.Initializer
		if init == nil {
			continue
		}

		// Determine field name: designated (.name = val) or positional.
		fname := ""
		if il.Designation != nil {
			// Designated initializer: extract field name and update fieldIdx.
			if dl := il.Designation.DesignatorList; dl != nil && dl.Designator != nil {
				d := dl.Designator
				switch d.Case {
				case cc.DesignatorField: // .name = val
					fname = d.Token2.SrcStr()
				case cc.DesignatorField2: // name: val (GCC extension)
					fname = d.Token.SrcStr()
				}
				// Update fieldIdx to match the designated field.
				for i, f := range st.Fields {
					if f.Name == fname {
						fieldIdx = i
						break
					}
				}
			}
		} else if fieldIdx < len(st.Fields) {
			fname = st.Fields[fieldIdx].Name
		}

		// Lower the initializer value.
		var val hir.Expr
		switch init.Case {
		case cc.InitializerExpr:
			if init.AssignmentExpression != nil {
				r, err := fl.lowerExpr(init.AssignmentExpression)
				if err != nil {
					return nil, err
				}
				val = fl.low.convertType(r.toExpr(), init.Type())
			}
		case cc.InitializerInitList:
			// Nested brace init — flatten embedded struct fields into parent.
			if fieldIdx < len(st.Fields) {
				innerSt := fl.low.structs[st.Fields[fieldIdx].Name]
				if innerSt == nil {
					// Try finding by looking up the struct field type name from the C AST.
					// For now, flatten: walk the inner init list and assign to parent fields sequentially.
					innerIdx := fieldIdx
					for innerIL := init.InitializerList; innerIL != nil; innerIL = innerIL.InitializerList {
						innerInit := innerIL.Initializer
						if innerInit == nil || innerInit.Case != cc.InitializerExpr {
							continue
						}
						// This inner value maps to a flattened field in the parent struct.
						// E.g. {{10, 20}, 100} → fields[0]=10, fields[1]=20, fields[2]=100
						if innerIdx < len(st.Fields) {
							r, err := fl.lowerExpr(innerInit.AssignmentExpression)
							if err != nil {
								return nil, err
							}
							lit.Fields = append(lit.Fields, hir.FieldInit{
								Name: st.Fields[innerIdx].Name,
								Val:  fl.low.convertType(r.toExpr(), innerInit.Type()),
							})
						}
						innerIdx++
					}
					fieldIdx = innerIdx - 1 // will be incremented at end of loop
				}
			}
		}

		if val != nil && fname != "" {
			lit.Fields = append(lit.Fields, hir.FieldInit{Name: fname, Val: val})
		}
		fieldIdx++
	}
	return lit, nil
}

func cStorageContainsArray(ty mir2.Ty) bool {
	switch ty := mir2.BaseOf(ty).(type) {
	case *mir2.ArrayTy:
		return true
	case *mir2.StructTy:
		for _, field := range ty.Fields {
			if cStorageContainsArray(field.Ty) {
				return true
			}
		}
	}
	return false
}

// Use the typed C initializer offsets for nested automatic aggregates.
// Explicit leaves are stored once; only omitted bytes receive implicit zeros.
func (fl *funcLow) lowerAutomaticAggregateInitializer(name string, storage mir2.Ty, initializer *cc.Initializer, volatile bool) ([]hir.Stmt, error) {
	var leaves []*cc.Initializer
	var collect func(*cc.Initializer)
	collect = func(init *cc.Initializer) {
		if init.Case == cc.InitializerInitList {
			for list := init.InitializerList; list != nil; list = list.InitializerList {
				if list.Initializer != nil {
					collect(list.Initializer)
				}
			}
		} else {
			leaves = append(leaves, init)
		}
	}
	collect(initializer)
	size := mir2.ByteWidth(storage)
	written := make([]bool, size)
	for _, leaf := range leaves {
		if field := leaf.Field(); field != nil && field.IsBitfield() {
			return nil, fmt.Errorf("bit-field initializer is not supported")
		}
		off, width := int(leaf.Offset()), int(leaf.Type().Size())
		if off < 0 || width < 1 || off+width > size {
			return nil, fmt.Errorf("initializer layout exceeds storage: offset %d width %d size %d", off, width, size)
		}
		for i := off; i < off+width; i++ {
			written[i] = true
		}
	}
	address := func(offset int) hir.Expr {
		return hir.AddrExpr(&hir.IndexExpr{Base: hir.Var(name, mir2.TyPtr), Idx: hir.U16(int64(offset)), ElemTy: mir2.TyU8, ElemStride: 1})
	}
	var stmts []hir.Stmt
	for off, explicit := range written {
		if !explicit {
			stmts = append(stmts, &hir.StoreStmt{Ptr: address(off), Val: hir.U8(0), Volatile: volatile || mir2.VolatileStorageByte(storage, off)})
		}
	}
	for _, leaf := range leaves {
		off := int(leaf.Offset())
		if str, ok := leaf.Value().(cc.StringValue); ok && leaf.Type().Kind() == cc.Array {
			data := make([]byte, int(leaf.Type().Size()))
			copy(data, []byte(str))
			for i, value := range data {
				stmts = append(stmts, &hir.StoreStmt{Ptr: address(off + i), Val: hir.U8(int64(value)), Volatile: volatile || mir2.VolatileStorageByte(storage, off+i)})
			}
			continue
		}
		if leaf.AssignmentExpression == nil {
			return nil, fmt.Errorf("unsupported array initializer leaf")
		}
		value, err := fl.lowerExprAsExpr(leaf.AssignmentExpression)
		if err != nil {
			return nil, err
		}
		fl.drainPending(&stmts)
		value = fl.low.convertType(value, leaf.Type())
		stmts = append(stmts, &hir.StoreStmt{Ptr: address(off), Val: value, Volatile: volatile || mir2.VolatileStorageByte(storage, off)})
	}
	return stmts, nil
}

func constToInt64(v cc.Value) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case cc.Int64Value:
		return int64(n)
	case cc.UInt64Value:
		return int64(n)
	case *cc.UnknownValue:
		return 0
	default:
		s := fmt.Sprintf("%v", n)
		var i int64
		fmt.Sscanf(s, "%d", &i)
		return i
	}
}

// parseIntToken parses a C integer literal token string to int64.
// Handles decimal, hex (0x), octal (0), and optional suffixes (u, l, ul, etc.).
func parseIntToken(s string) int64 {
	// Strip type suffixes.
	s = strings.TrimRight(s, "uUlL")
	if s == "" {
		return 0
	}
	var val int64
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		fmt.Sscanf(s, "%v", &val)
	} else if len(s) > 1 && s[0] == '0' {
		// Octal.
		fmt.Sscanf(s, "%v", &val)
	} else {
		fmt.Sscanf(s, "%d", &val)
	}
	return val
}
