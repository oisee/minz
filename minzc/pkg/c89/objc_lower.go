// ObjC → HIR lowering for MinZ C89 frontend.
//
// On Z80 there is no ObjC runtime — all dispatch is static:
//
//	@interface Foo { int x; }    → struct Foo { x: i16 }
//	-(int)add:(int)a             → fun Foo_add(self: ptr, a: i16) -> i16
//	[foo add:5]                  → Foo_add(foo, 5)
//	@protocol P                  → compile-time constraint (no codegen)
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

// objcProtocolInfo stores parsed @protocol data for conformance checking.
type objcProtocolInfo struct {
	name    string
	methods map[string]*objcMethod // required methods
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

	// Check protocol conformance.
	for _, protoName := range info.protocols {
		if err := l.checkProtocolConformance(className, info, protoName); err != nil {
			return err
		}
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

		// Look up method info.
		method := info.methods[oa.selector()]
		if method == nil {
			continue
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
