// ObjC → HIR lowering for MinZ C89 frontend.
//
// Dispatch is static by default, dynamic when receiver is protocol-typed:
//
//	@interface Foo { int x; }    → struct Foo { __vtable: ptr, x: i16 }
//	-(int)add:(int)a             → fun Foo_add(self: ptr, a: i16) -> i16
//	[foo add:5]                  → Foo_add(foo, 5)            [static]
//	id<P> p = foo; [p add:5]    → vtable_call(p, P.add, 5)   [dynamic]
//	@protocol P                  → defines vtable layout + conformance check
//
// Method name mangling: ClassName_selectorFlattened
//   -(int)value           → Foo_value
//   -(int)add:(int)x      → Foo_add
//   -(int)addX:(int)x andY:(int)y → Foo_addX_andY
package c89

import (
	"fmt"
	"strconv"
	"strings"

	cc "github.com/minz/minzc/pkg/cparse"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// objcClassInfo stores parsed @interface data for use during lowering.
type objcClassInfo struct {
	name      string
	superName string                 // "" if no superclass
	protocols []string               // protocol names from <Proto1, Proto2>
	ivars     []mir2.StructField     // instance variables (own only, not inherited)
	methods   map[string]*objcMethod // selector → method info (includes inherited)
}

// objcProtocolInfo stores parsed @protocol data for conformance checking
// and vtable layout. Method order in selectors defines the vtable slot order.
type objcProtocolInfo struct {
	name      string
	methods   map[string]*objcMethod // required methods
	selectors []string               // ordered selector list (defines vtable layout)
}

type objcMethod struct {
	mangledName string
	retTy       mir2.Ty
	retClass    string      // class name if return type is ClassName* (for method chaining)
	params      []hir.Param // NOT including self
	inherited   bool        // true if inherited from parent (not overridden)
	parentClass string      // parent class name (for inherited methods / super calls)
}

// lowerObjCInterface processes @interface → struct declaration + class registration.
func (l *lowerer) lowerObjCInterface(iface *cc.ObjCInterfaceDecl) error {
	className := iface.ClassName.SrcStr()

	info := &objcClassInfo{
		name:    className,
		methods: make(map[string]*objcMethod),
	}
	if iface.SuperName != nil {
		info.superName = iface.SuperName.SrcStr()
	}
	for _, p := range iface.Protocols {
		info.protocols = append(info.protocols, p.SrcStr())
	}

	// Lower ivars → struct fields.
	// Parent fields come FIRST (struct embedding for inheritance).
	mst := &mir2.StructTy{Name: className}
	if info.superName != "" {
		if parentInfo := l.objcClasses[info.superName]; parentInfo != nil {
			if parentSt := l.structs[info.superName]; parentSt != nil {
				// Copy ALL parent fields (including grandparent fields).
				mst.Fields = append(mst.Fields, parentSt.Fields...)
			}
			// Inherit parent method signatures (child can override later).
			for sel, pm := range parentInfo.methods {
				info.methods[sel] = &objcMethod{
					mangledName: objcMangleName(className, sel),
					retTy:       pm.retTy,
					params:      pm.params,
					inherited:   true,
					parentClass: info.superName,
				}
			}
		}
	}
	// If class conforms to any protocol, prepend __vtable pointer field.
	if len(info.protocols) > 0 {
		hasVtable := false
		for _, f := range mst.Fields {
			if f.Name == "__vtable" {
				hasVtable = true
				break
			}
		}
		if !hasVtable {
			mst.Fields = append([]mir2.StructField{{Name: "__vtable", Ty: mir2.TyPtr}}, mst.Fields...)
		}
	}

	// Then own ivars.
	for _, iv := range iface.Ivars {
		ty := l.objcTypeFromTokens(iv.TypeTokens)
		fname := iv.Name.SrcStr()
		f := mir2.StructField{Name: fname, Ty: ty}
		mst.Fields = append(mst.Fields, f)
		info.ivars = append(info.ivars, f)
	}
	l.structs[className] = mst
	l.hm.Structs = append(l.hm.Structs, mst)

	// Register method signatures from declarations (overrides inherited).
	for _, md := range iface.Methods {
		sel := md.MethodName()
		mangled := objcMangleName(className, sel)
		retTy := l.objcTypeFromTokens(md.ReturnType)
		retClass := l.objcReturnClass(md.ReturnType)
		var params []hir.Param
		for _, part := range md.Selector {
			if part.Colon != nil && part.ParamName != nil {
				params = append(params, hir.Param{
					Name: part.ParamName.SrcStr(),
					Ty:   l.objcTypeFromTokens(part.ParamType),
				})
			}
		}
		info.methods[sel] = &objcMethod{
			mangledName: mangled,
			retTy:       retTy,
			retClass:    retClass,
			params:      params,
			inherited:   false,
		}
	}

	l.objcClasses[className] = info
	return nil
}

// lowerObjCImplementation processes @implementation → HIR functions.
func (l *lowerer) lowerObjCImplementation(impl *cc.ObjCImplementationDecl) error {
	className := impl.ClassName.SrcStr()

	// Look up class info (from @interface). If missing, create a minimal one.
	info := l.objcClasses[className]
	if info == nil {
		info = &objcClassInfo{name: className, methods: make(map[string]*objcMethod)}
		l.objcClasses[className] = info
	}

	// Track which methods are defined in this @implementation.
	definedMethods := make(map[string]bool)

	for _, md := range impl.Methods {
		f, err := l.lowerObjCMethod(className, info, md)
		if err != nil {
			return fmt.Errorf("@implementation %s method %s: %w", className, md.MethodName(), err)
		}
		if f != nil {
			l.hm.Funcs = append(l.hm.Funcs, f)
			definedMethods[md.MethodName()] = true
		}
	}

	// Generate trampolines for inherited methods not overridden.
	for sel, m := range info.methods {
		if m.inherited && !definedMethods[sel] {
			trampoline := l.generateInheritedTrampoline(className, info, sel, m)
			if trampoline != nil {
				l.hm.Funcs = append(l.hm.Funcs, trampoline)
			}
		}
	}

	// Check protocol conformance and generate vtables.
	for _, protoName := range info.protocols {
		if err := l.checkProtocolConformance(className, info, protoName); err != nil {
			return err
		}
		l.generateVtable(className, info, protoName)
	}

	return nil
}

// lowerObjCProtocol registers protocol method signatures for conformance checking.
func (l *lowerer) lowerObjCProtocol(proto *cc.ObjCProtocolDecl) {
	if proto == nil {
		return
	}
	name := proto.Name.SrcStr()
	pinfo := &objcProtocolInfo{
		name:    name,
		methods: make(map[string]*objcMethod),
	}
	for _, md := range proto.Methods {
		sel := md.MethodName()
		retTy := l.objcTypeFromTokens(md.ReturnType)
		var params []hir.Param
		for _, part := range md.Selector {
			if part.Colon != nil && part.ParamName != nil {
				params = append(params, hir.Param{
					Name: part.ParamName.SrcStr(),
					Ty:   l.objcTypeFromTokens(part.ParamType),
				})
			}
		}
		pinfo.methods[sel] = &objcMethod{
			retTy:  retTy,
			params: params,
		}
		pinfo.selectors = append(pinfo.selectors, sel)
	}
	l.objcProtocols[name] = pinfo
}

// lowerObjCMethod converts one ObjC method definition into an HIR function.
//
//	-(int)add:(int)x  in class Foo  →  fun Foo_add(self: ptr, x: i16) -> i16
func (l *lowerer) lowerObjCMethod(className string, info *objcClassInfo, md *cc.ObjCMethodDef) (*hir.Func, error) {
	sel := md.MethodName()
	mangled := objcMangleName(className, sel)

	retTy := l.objcTypeFromTokens(md.ReturnType)

	// Build params: self first (for instance methods), then declared params.
	var params []hir.Param
	if md.Kind == cc.ObjCInstanceMethod {
		params = append(params, hir.Param{Name: "self", Ty: mir2.TyPtr})
	}
	for _, part := range md.Selector {
		if part.Colon != nil && part.ParamName != nil {
			params = append(params, hir.Param{
				Name: part.ParamName.SrcStr(),
				Ty:   l.objcTypeFromTokens(part.ParamType),
			})
		}
	}

	// Register method info if not already from @interface.
	if info.methods[sel] == nil {
		info.methods[sel] = &objcMethod{
			mangledName: mangled,
			retTy:       retTy,
			params:      params[1:], // skip self
		}
	}

	fl := &funcLow{
		low:    l,
		name:   mangled,
		retTy:  retTy,
		locals: make(map[string]mir2.Ty),
	}

	// Register params as locals.
	for _, p := range params {
		fl.locals[p.Name] = p.Ty
	}

	// Set ObjC context so self->field and [self msg] resolve correctly.
	fl.objcClass = info

	body, err := fl.lowerCompound(md.Body)
	if err != nil {
		return nil, err
	}

	// Ensure function ends with a return.
	if len(body) == 0 || !isReturn(body[len(body)-1]) {
		if retTy == mir2.TyVoid {
			body = append(body, hir.RetVoid())
		}
	}

	return &hir.Func{
		Name:   mangled,
		Params: params,
		RetTy:  retTy,
		Body:   hir.Blk(body...),
	}, nil
}

// lowerObjCMessage converts [receiver sel:arg1 kw2:arg2] → ClassName_sel_kw2(receiver, arg1, arg2).
// If the receiver is protocol-typed (id<Proto>), emits dynamic dispatch via vtable.
func (fl *funcLow) lowerObjCMessage(msg *cc.ObjCMessageExpr) (*exprResult, error) {
	// Lower receiver.
	receiver, err := fl.lowerExprAsExpr(msg.Receiver)
	if err != nil {
		return nil, err
	}

	// Build selector string.
	sel := objcMsgSelector(msg)

	// Handle [super msg] — resolve to parent class, use self as receiver.
	isSuper := false
	if v, ok := receiver.(*hir.VarRefExpr); ok && v.Name == "super" {
		isSuper = true
		// Replace receiver with self (same object, parent method).
		receiver = hir.Var("self", mir2.TyPtr)
	}

	// Check for protocol-typed receiver → dynamic dispatch.
	if !isSuper {
		if protoName := fl.resolveObjCReceiverProtocol(receiver); protoName != "" {
			return fl.lowerObjCDynamicMessage(receiver, protoName, sel, msg)
		}
	}

	// Resolve class name from receiver.
	var className string
	if isSuper {
		// super → parent class of current ObjC class.
		if fl.objcClass != nil && fl.objcClass.superName != "" {
			className = fl.objcClass.superName
		}
	} else {
		className = fl.resolveObjCReceiverClass(msg, receiver)
	}
	if className == "" {
		return nil, fmt.Errorf("cannot resolve ObjC class for message [%s %s]",
			receiverString(receiver), sel)
	}

	mangled := objcMangleName(className, sel)

	// Look up method info for return type.
	var retTy mir2.Ty = mir2.TyU16 // fallback
	if info := fl.low.objcClasses[className]; info != nil {
		if m := info.methods[sel]; m != nil {
			retTy = m.retTy
		}
	}

	// Build args: receiver first, then message arguments.
	args := []hir.Expr{receiver}
	for _, a := range msg.Args {
		if a.Value != nil {
			argExpr, err := fl.lowerExprAsExpr(a.Value)
			if err != nil {
				return nil, err
			}
			args = append(args, argExpr)
		}
	}

	return wrapExpr(hir.Call(mangled, retTy, args...)), nil
}

// resolveObjCReceiverProtocol checks if a receiver is protocol-typed (id<Proto>).
// Returns the protocol name or "" for concrete types.
func (fl *funcLow) resolveObjCReceiverProtocol(receiver hir.Expr) string {
	v, ok := receiver.(*hir.VarRefExpr)
	if !ok {
		return ""
	}
	if fl.objcProtoVars != nil {
		if proto, ok := fl.objcProtoVars[v.Name]; ok {
			return proto
		}
	}
	return ""
}

// lowerObjCDynamicMessage emits a vtable-based indirect call for protocol-typed receivers.
//
//	id<Drawable> shape = circle;
//	[shape draw]  →  call_indirect(load(load(shape) + slot*2), shape, ...)
//
// Layout: shape points to object, object.__vtable (offset 0) points to vtable array,
// vtable[slot] holds the function address.
func (fl *funcLow) lowerObjCDynamicMessage(receiver hir.Expr, protoName, sel string, msg *cc.ObjCMessageExpr) (*exprResult, error) {
	proto := fl.low.objcProtocols[protoName]
	if proto == nil {
		return nil, fmt.Errorf("unknown protocol %s for dynamic dispatch", protoName)
	}

	slot := vtableSlot(proto, sel)
	if slot < 0 {
		return nil, fmt.Errorf("selector -%s not found in protocol %s", sel, protoName)
	}

	// Look up return type from protocol method info.
	var retTy mir2.Ty = mir2.TyU16
	if m := proto.methods[sel]; m != nil {
		retTy = m.retTy
	}

	// Build: load vtable ptr from object (offset 0).
	vtablePtr := hir.Load(receiver, mir2.TyPtr)

	// Build: load function ptr from vtable[slot] = vtablePtr + slot*2.
	var fnPtr hir.Expr
	if slot == 0 {
		fnPtr = hir.Load(vtablePtr, mir2.TyPtr)
	} else {
		slotAddr := &hir.BinExpr{
			Op: "+",
			L:  vtablePtr,
			R:  &hir.IntLitExpr{Val: int64(slot * 2), Ty: mir2.TyPtr},
			Ty: mir2.TyPtr,
		}
		fnPtr = hir.Load(slotAddr, mir2.TyPtr)
	}

	// Build args: receiver first, then message arguments.
	args := []hir.Expr{receiver}
	for _, a := range msg.Args {
		if a.Value != nil {
			argExpr, err := fl.lowerExprAsExpr(a.Value)
			if err != nil {
				return nil, err
			}
			args = append(args, argExpr)
		}
	}

	return wrapExpr(&hir.CallIndirectExpr{
		FnPtr: fnPtr,
		Args:  args,
		Ty:    retTy,
	}), nil
}

// resolveObjCReceiverClass determines the class name for a message receiver.
func (fl *funcLow) resolveObjCReceiverClass(msg *cc.ObjCMessageExpr, receiver hir.Expr) string {
	// Case 1: self → current class.
	if v, ok := receiver.(*hir.VarRefExpr); ok && v.Name == "self" {
		if fl.objcClass != nil {
			return fl.objcClass.name
		}
	}

	// Case 2: receiver is a known local with a class type.
	if v, ok := receiver.(*hir.VarRefExpr); ok {
		if cls, ok := fl.objcLocals[v.Name]; ok {
			return cls
		}
	}

	// Case 3: class method — receiver is a class name identifier.
	if v, ok := receiver.(*hir.VarRefExpr); ok {
		if _, ok := fl.low.objcClasses[v.Name]; ok {
			return v.Name
		}
	}

	// Case 4: nested message / method chaining — receiver is a CallExpr.
	// Look up the called function's return class via method registry.
	if call, ok := receiver.(*hir.CallExpr); ok {
		for _, info := range fl.low.objcClasses {
			for _, m := range info.methods {
				if m.mangledName == call.Fn && m.retClass != "" {
					return m.retClass
				}
			}
		}
	}

	// Case 5: fallback to current class context.
	if fl.objcClass != nil {
		return fl.objcClass.name
	}

	return ""
}

// ── Inheritance & Protocols ───────────────────────────────────────────────────

// generateInheritedTrampoline creates a forwarding function for an inherited method.
// Circle_moveTo(self, nx, ny) { Shape_moveTo(self, nx, ny) }
func (l *lowerer) generateInheritedTrampoline(className string, info *objcClassInfo, sel string, m *objcMethod) *hir.Func {
	parentMangled := objcMangleName(m.parentClass, sel)
	childMangled := objcMangleName(className, sel)

	// Build params: self + method params.
	var params []hir.Param
	params = append(params, hir.Param{Name: "self", Ty: mir2.TyPtr})
	params = append(params, m.params...)

	// Build call args — forward everything.
	var args []hir.Expr
	for _, p := range params {
		args = append(args, hir.Var(p.Name, p.Ty))
	}

	var body []hir.Stmt
	if m.retTy == mir2.TyVoid || m.retTy == nil {
		body = append(body, &hir.ExprStmt{Expr: hir.Call(parentMangled, mir2.TyVoid, args...)})
		body = append(body, hir.RetVoid())
	} else {
		body = append(body, &hir.ReturnStmt{Val: hir.Call(parentMangled, m.retTy, args...)})
	}

	return &hir.Func{
		Name:   childMangled,
		Params: params,
		RetTy:  m.retTy,
		Body:   hir.Blk(body...),
	}
}

// checkProtocolConformance verifies that a class implements all required protocol methods.
func (l *lowerer) checkProtocolConformance(className string, info *objcClassInfo, protoName string) error {
	proto := l.objcProtocols[protoName]
	if proto == nil {
		// Unknown protocol — skip silently (may be forward-declared).
		return nil
	}
	for sel := range proto.methods {
		if info.methods[sel] == nil {
			return fmt.Errorf("@implementation %s does not implement -%s required by protocol %s",
				className, sel, protoName)
		}
	}
	return nil
}

// ── Dynamic Dispatch (Protocol-Based Vtables) ────────────────────────────────
//
// When a receiver is typed as id<Protocol>, dispatch goes through a vtable:
//
//   @protocol Drawable
//   -(int)draw;         // slot 0
//   -(int)area;         // slot 1
//   @end
//
//   // Each conforming class gets a vtable global:
//   global __vtable_Circle_Drawable = [&Circle_draw, &Circle_area]
//   global __vtable_Rect_Drawable   = [&Rect_draw,   &Rect_area]
//
//   // Object struct has __vtable at offset 0:
//   struct Circle { __vtable: ptr, x: i16, y: i16, r: i16 }
//
//   // Dynamic call: load vtable ptr, index, call indirect.
//   id<Drawable> shape = circle;
//   [shape draw]  →  call_indirect(load(shape + 0)[slot * 2], shape)
//

// vtableName returns the global name for a class's protocol vtable.
func vtableName(className, protoName string) string {
	return "__vtable_" + className + "_" + protoName
}

// vtableSlot returns the slot index for a selector within a protocol.
// Returns -1 if not found.
func vtableSlot(proto *objcProtocolInfo, sel string) int {
	for i, s := range proto.selectors {
		if s == sel {
			return i
		}
	}
	return -1
}

// generateVtable emits a global array of function pointers for a class's
// protocol conformance. Each slot holds the address of the class's
// implementation of that protocol method.
func (l *lowerer) generateVtable(className string, info *objcClassInfo, protoName string) {
	proto := l.objcProtocols[protoName]
	if proto == nil {
		return
	}

	// Build init data: array of function addresses (2 bytes each on Z80).
	// Each entry is the address of ClassName_selector.
	var syms []string
	for _, sel := range proto.selectors {
		m := info.methods[sel]
		if m != nil {
			syms = append(syms, m.mangledName)
		} else {
			syms = append(syms, "") // shouldn't happen if conformance passed
		}
	}

	gname := vtableName(className, protoName)
	// Each slot is 2 bytes (ptr width on Z80). Init is zeroed — VM patches at load time.
	initBytes := make([]byte, len(syms)*2)
	g := mir2.Global{
		Name:       gname,
		Ty:         mir2.TyPtr, // base type; actual size = len(VtableSyms)*2
		Init:       initBytes,
		VtableSyms: syms, // list of function names — resolved to addresses at link/VM time
	}
	l.hm.Globals = append(l.hm.Globals, g)
	l.globals[gname] = mir2.TyPtr
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// objcMangleName converts "ClassName" + "add:with:" → "ClassName_add_with".
func objcMangleName(className, selector string) string {
	// Remove trailing colon, replace internal colons with underscores.
	sel := strings.TrimRight(selector, ":")
	sel = strings.ReplaceAll(sel, ":", "_")
	return className + "_" + sel
}

// objcMsgSelector builds the selector string from message args.
func objcMsgSelector(msg *cc.ObjCMessageExpr) string {
	if len(msg.Args) == 0 {
		return ""
	}
	if len(msg.Args) == 1 && msg.Args[0].Colon == nil {
		// Unary message: [obj value]
		return msg.Args[0].Keyword.SrcStr()
	}
	var s string
	for _, a := range msg.Args {
		kw := a.Keyword.SrcStr()
		if kw != "" {
			s += kw + ":"
		}
	}
	return s
}

// objcTypeFromTokens converts type specifier tokens to mir2.Ty.
func (l *lowerer) objcTypeFromTokens(toks []cc.Token) mir2.Ty {
	if len(toks) == 0 {
		return mir2.TyVoid
	}
	// Join tokens to a type string.
	var parts []string
	for _, t := range toks {
		parts = append(parts, t.SrcStr())
	}
	s := strings.Join(parts, " ")

	switch s {
	case "void":
		return mir2.TyVoid
	case "int", "signed int", "int16_t":
		return mir2.TyI16
	case "unsigned int", "uint16_t":
		return mir2.TyU16
	case "char", "unsigned char", "uint8_t":
		return mir2.TyU8
	case "signed char", "int8_t":
		return mir2.TyI8
	default:
		// Pointer types or struct types.
		if strings.Contains(s, "*") {
			return mir2.TyPtr
		}
		// Check if it's a known struct.
		if _, ok := l.structs[s]; ok {
			return mir2.TyPtr
		}
		return mir2.TyI16 // conservative fallback
	}
}

// objcReturnClass extracts the class name from return type tokens like "Shape *".
// Returns "" if the return type is not a pointer to a known class.
func (l *lowerer) objcReturnClass(toks []cc.Token) string {
	if len(toks) == 0 {
		return ""
	}
	var parts []string
	for _, t := range toks {
		s := t.SrcStr()
		if s != "*" {
			parts = append(parts, s)
		}
	}
	name := strings.Join(parts, " ")
	if _, ok := l.objcClasses[name]; ok {
		return name
	}
	return ""
}

// receiverString returns a string representation of an expression for error messages.
func receiverString(e hir.Expr) string {
	if v, ok := e.(*hir.VarRefExpr); ok {
		return v.Name
	}
	return "<expr>"
}

// ── ObjC Assert Wrappers ─────────────────────────────────────────────────────
//
// assert-objc Counter{count:42}.value() == 42
//
// Generates a wrapper function that allocates the struct, initializes fields,
// calls the method, and returns the result. The existing assert system then
// calls this wrapper.

// generateObjCAssertWrappers scans source for assert-objc directives and
// generates HIR wrapper functions + standard asserts.
func (l *lowerer) generateObjCAssertWrappers(src string) {
	counter := 0
	for i, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		lineNo := i + 1

		oa, ok := parseObjCAssert(line, lineNo)
		if !ok {
			continue
		}

		// Look up class info.
		info := l.objcClasses[oa.className]
		if info == nil {
			continue // unknown class — skip silently
		}
		st := l.structs[oa.className]
		if st == nil {
			continue
		}

		// Look up method info — try exact selector first, then prefix match for multi-keyword selectors.
		method := info.methods[oa.selector()]
		if method == nil {
			// Multi-keyword selector: assert uses "containsX(50,50)" but method is "containsX:y:".
			// Match by first keyword prefix + compatible arg count.
			prefix := oa.methodName + ":"
			for sel, m := range info.methods {
				if strings.HasPrefix(sel, prefix) && len(m.params) == len(oa.args) {
					method = m
					break
				}
			}
			if method == nil {
				continue
			}
		}

		// Generate wrapper function name.
		wrapperName := fmt.Sprintf("__objc_test_%d", counter)
		counter++

		// Build body:
		//   var obj: ClassName  (zero-initialized)
		//   obj.field1 = val1
		//   obj.field2 = val2
		//   return ClassName_method(&obj, args...)
		var body []hir.Stmt

		// Declare struct variable (zero-init).
		body = append(body, &hir.VarDeclStmt{
			Name: "__obj",
			Ty:   mir2.TyPtr,
			Init: &hir.StructLitExpr{St: st, Fields: nil}, // allocate, zero fields
		})

		// Initialize fields explicitly via assignments.
		for _, fi := range oa.fieldInits {
			// Compute byte offset from struct definition.
			fieldOff := 0
			for i, sf := range st.Fields {
				if sf.Name == fi.name {
					fieldOff = st.ByteOffset(i)
					break
				}
			}
			body = append(body, hir.Assign(
				&hir.FieldExpr{
					X:      hir.Var("__obj", mir2.TyPtr),
					Field:  fi.name,
					Offset: fieldOff,
					Ty:     mir2.TyI16,
				},
				&hir.IntLitExpr{Val: fi.val, Ty: mir2.TyI16},
			))
		}

		// Build call args: &obj (self), then literal args.
		callArgs := []hir.Expr{hir.Var("__obj", mir2.TyPtr)}
		for _, a := range oa.args {
			callArgs = append(callArgs, &hir.IntLitExpr{Val: a, Ty: mir2.TyI16})
		}

		retTy := method.retTy
		if retTy == nil || retTy == mir2.TyVoid {
			retTy = mir2.TyI16 // fallback for testability
		}

		body = append(body, &hir.ReturnStmt{
			Val: hir.Call(method.mangledName, retTy, callArgs...),
		})

		// Add wrapper function to the module.
		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
			Name:   wrapperName,
			Params: nil,
			RetTy:  retTy,
			Body:   hir.Blk(body...),
		})

		// Add standard assert.
		// Default to MIR2 VM — Z80 struct alloca in wrappers is not yet supported.
		via := oa.via
		if via == "" {
			via = "mir2"
		}
		l.hm.Asserts = append(l.hm.Asserts, hir.Assert{
			FuncName: wrapperName,
			Args:     nil,
			Expected: oa.expected,
			Source:   line,
			Line:     lineNo,
			Via:      via,
		})
	}
}

type objcAssertInfo struct {
	className  string
	fieldInits []objcFieldInit
	methodName string
	args       []int64
	expected   int64
	via        string
}

type objcFieldInit struct {
	name string
	val  int64
}

// selector returns the ObjC selector string for the method.
// Unary: "value" → "value". Keyword with args: "addN" + 1 arg → "addN:".
func (oa *objcAssertInfo) selector() string {
	if len(oa.args) > 0 {
		return oa.methodName + ":"
	}
	return oa.methodName
}

func parseObjCAssert(line string, lineNo int) (objcAssertInfo, bool) {
	m := objcAssertRe.FindStringSubmatch(line)
	if m == nil {
		return objcAssertInfo{}, false
	}

	oa := objcAssertInfo{
		className:  m[1],
		methodName: m[3],
		via:        m[6],
	}

	// Parse field initializers: "count:42, x:10"
	fieldsStr := strings.TrimSpace(m[2])
	if fieldsStr != "" {
		for _, part := range strings.Split(fieldsStr, ",") {
			part = strings.TrimSpace(part)
			kv := strings.SplitN(part, ":", 2)
			if len(kv) != 2 {
				continue
			}
			name := strings.TrimSpace(kv[0])
			val, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 0, 64)
			if err != nil {
				continue
			}
			oa.fieldInits = append(oa.fieldInits, objcFieldInit{name: name, val: val})
		}
	}

	// Parse method args.
	argsStr := strings.TrimSpace(m[4])
	if argsStr != "" {
		for _, s := range strings.Split(argsStr, ",") {
			s = strings.TrimSpace(s)
			v, err := strconv.ParseInt(s, 0, 64)
			if err != nil {
				continue
			}
			oa.args = append(oa.args, v)
		}
	}

	oa.expected, _ = strconv.ParseInt(m[5], 0, 64)
	return oa, true
}

// ── Dynamic Dispatch Assert Wrappers ──────────────────────────────────────────
//
// assert-objc-dyn Drawable Circle{radius:5}.area() == 25
//
// Generates a wrapper that:
//   1. Allocates the object with vtable pointer at offset 0
//   2. Stores address of __vtable_Circle_Drawable at offset 0
//   3. Initializes fields
//   4. Loads function pointer from vtable[slot]
//   5. Calls indirectly with object as self

func (l *lowerer) generateObjCDynAssertWrappers(src string) {
	counter := 0
	for i, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		lineNo := i + 1

		da, ok := parseObjCDynAssert(line, lineNo)
		if !ok {
			continue
		}

		info := l.objcClasses[da.className]
		if info == nil {
			continue
		}
		st := l.structs[da.className]
		if st == nil {
			continue
		}
		proto := l.objcProtocols[da.protoName]
		if proto == nil {
			continue
		}

		sel := da.selector()
		slot := vtableSlot(proto, sel)
		if slot < 0 {
			continue
		}

		// Look up return type from protocol method.
		var retTy mir2.Ty = mir2.TyI16
		if m := proto.methods[sel]; m != nil && m.retTy != nil && m.retTy != mir2.TyVoid {
			retTy = m.retTy
		}

		wrapperName := fmt.Sprintf("__objc_dyn_test_%d", counter)
		counter++

		var body []hir.Stmt

		// 1. Allocate object struct (zero-init).
		body = append(body, &hir.VarDeclStmt{
			Name: "__obj",
			Ty:   mir2.TyPtr,
			Init: &hir.StructLitExpr{St: st, Fields: nil},
		})

		// 2. Store vtable pointer at offset 0 (__vtable field).
		vtName := vtableName(da.className, da.protoName)
		body = append(body, hir.Assign(
			&hir.FieldExpr{
				X:      hir.Var("__obj", mir2.TyPtr),
				Field:  "__vtable",
				Offset: 0,
				Ty:     mir2.TyPtr,
			},
			&hir.AddrOfExpr{Sym: vtName},
		))

		// 3. Initialize fields.
		for _, fi := range da.fieldInits {
			fieldOff := 0
			for j, sf := range st.Fields {
				if sf.Name == fi.name {
					fieldOff = st.ByteOffset(j)
					break
				}
			}
			body = append(body, hir.Assign(
				&hir.FieldExpr{
					X:      hir.Var("__obj", mir2.TyPtr),
					Field:  fi.name,
					Offset: fieldOff,
					Ty:     mir2.TyI16,
				},
				&hir.IntLitExpr{Val: fi.val, Ty: mir2.TyI16},
			))
		}

		// 4. Load vtable ptr from obj (offset 0), then fn ptr from vtable[slot].
		body = append(body, &hir.VarDeclStmt{
			Name: "__vt",
			Ty:   mir2.TyPtr,
			Init: hir.Load(hir.Var("__obj", mir2.TyPtr), mir2.TyPtr),
		})

		var fnPtrExpr hir.Expr
		if slot == 0 {
			fnPtrExpr = hir.Load(hir.Var("__vt", mir2.TyPtr), mir2.TyPtr)
		} else {
			fnPtrExpr = hir.Load(&hir.BinExpr{
				Op: "+",
				L:  hir.Var("__vt", mir2.TyPtr),
				R:  &hir.IntLitExpr{Val: int64(slot * 2), Ty: mir2.TyPtr},
				Ty: mir2.TyPtr,
			}, mir2.TyPtr)
		}
		body = append(body, &hir.VarDeclStmt{
			Name: "__fn",
			Ty:   mir2.TyPtr,
			Init: fnPtrExpr,
		})

		// 5. Call indirect: __fn(__obj, args...)
		callArgs := []hir.Expr{hir.Var("__obj", mir2.TyPtr)}
		for _, a := range da.args {
			callArgs = append(callArgs, &hir.IntLitExpr{Val: a, Ty: mir2.TyI16})
		}

		body = append(body, &hir.ReturnStmt{
			Val: &hir.CallIndirectExpr{
				FnPtr: hir.Var("__fn", mir2.TyPtr),
				Args:  callArgs,
				Ty:    retTy,
			},
		})

		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
			Name:   wrapperName,
			Params: nil,
			RetTy:  retTy,
			Body:   hir.Blk(body...),
		})

		via := da.via
		if via == "" {
			via = "mir2"
		}
		l.hm.Asserts = append(l.hm.Asserts, hir.Assert{
			FuncName: wrapperName,
			Args:     nil,
			Expected: da.expected,
			Source:   line,
			Line:     lineNo,
			Via:      via,
		})
	}
}

type objcDynAssertInfo struct {
	protoName  string
	className  string
	fieldInits []objcFieldInit
	methodName string
	args       []int64
	expected   int64
	via        string
}

func (da *objcDynAssertInfo) selector() string {
	if len(da.args) > 0 {
		return da.methodName + ":"
	}
	return da.methodName
}

func parseObjCDynAssert(line string, lineNo int) (objcDynAssertInfo, bool) {
	m := objcDynAssertRe.FindStringSubmatch(line)
	if m == nil {
		return objcDynAssertInfo{}, false
	}

	da := objcDynAssertInfo{
		protoName:  m[1],
		className:  m[2],
		methodName: m[4],
		via:        m[7],
	}

	fieldsStr := strings.TrimSpace(m[3])
	if fieldsStr != "" {
		for _, part := range strings.Split(fieldsStr, ",") {
			part = strings.TrimSpace(part)
			kv := strings.SplitN(part, ":", 2)
			if len(kv) != 2 {
				continue
			}
			name := strings.TrimSpace(kv[0])
			val, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 0, 64)
			if err != nil {
				continue
			}
			da.fieldInits = append(da.fieldInits, objcFieldInit{name: name, val: val})
		}
	}

	argsStr := strings.TrimSpace(m[5])
	if argsStr != "" {
		for _, s := range strings.Split(argsStr, ",") {
			s = strings.TrimSpace(s)
			v, err := strconv.ParseInt(s, 0, 64)
			if err != nil {
				continue
			}
			da.args = append(da.args, v)
		}
	}

	da.expected, _ = strconv.ParseInt(m[6], 0, 64)
	return da, true
}
