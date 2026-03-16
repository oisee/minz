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
	"strings"

	cc "github.com/minz/minzc/pkg/cparse"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// objcClassInfo stores parsed @interface data for use during lowering.
type objcClassInfo struct {
	name      string
	superName string                 // "" if no superclass
	ivars     []mir2.StructField     // instance variables
	methods   map[string]*objcMethod // selector → method info
}

type objcMethod struct {
	mangledName string
	retTy       mir2.Ty
	params      []hir.Param // NOT including self
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

	// Lower ivars → struct fields.
	mst := &mir2.StructTy{Name: className}
	for _, iv := range iface.Ivars {
		ty := l.objcTypeFromTokens(iv.TypeTokens)
		fname := iv.Name.SrcStr()
		f := mir2.StructField{Name: fname, Ty: ty}
		mst.Fields = append(mst.Fields, f)
		info.ivars = append(info.ivars, f)
	}
	l.structs[className] = mst
	l.hm.Structs = append(l.hm.Structs, mst)

	// Register method signatures from declarations.
	for _, md := range iface.Methods {
		sel := md.MethodName()
		mangled := objcMangleName(className, sel)
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
		info.methods[sel] = &objcMethod{
			mangledName: mangled,
			retTy:       retTy,
			params:      params,
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

	for _, md := range impl.Methods {
		f, err := l.lowerObjCMethod(className, info, md)
		if err != nil {
			return fmt.Errorf("@implementation %s method %s: %w", className, md.MethodName(), err)
		}
		if f != nil {
			l.hm.Funcs = append(l.hm.Funcs, f)
		}
	}
	return nil
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

	// Resolve class name from receiver.
	className := fl.resolveObjCReceiverClass(msg, receiver)
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

	// Case 4: nested message — check if the lowerer has a single class.
	// Fallback: use the first registered class (for simple single-class programs).
	if fl.objcClass != nil {
		return fl.objcClass.name
	}

	return ""
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

// receiverString returns a string representation of an expression for error messages.
func receiverString(e hir.Expr) string {
	if v, ok := e.(*hir.VarRefExpr); ok {
		return v.Name
	}
	return "<expr>"
}
