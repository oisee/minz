// Research importer for native Crystal records. All memory layout is computed
// with MIR2 target types; host Crystal offsets/pointer widths are never imported.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/minz/minzc/internal/crystalmeta"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

type field struct{ Name, Ty string }
type node struct {
	Kind, Ty, Name, Op  string
	Value               int64
	Args, Body, Yes, No []*node
}
type function struct {
	Name, Source, Owner string
	Params              []field
	Ret, Elem           string
	Library             string
	Body                []*node
}
type record struct {
	Name, Native, Kind string
	Fields             []field
}
type module struct {
	Protocol  string
	Records   []record
	Functions []function
}
type testCase struct {
	Source   string
	Args     []int64
	Expected int64
}
type importer struct {
	m              *hir.Module
	types          map[string]mir2.Ty
	funcs          map[string]*hir.Func
	locals         map[string]mir2.Ty
	current        *hir.Func
	serial         int
	metaHelpers    map[string]*hir.Func
	layoutReceipts []crystalmeta.LayoutReceipt
	byteLibrary    *hir.Module
	byteLibrarySHA string
}

// The facade has its own record names. Cast pointers only after checking every
// field name/type (and hence target offset), never just the total byte size.
func byteABIEqual(a, b mir2.Ty) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	if !mir2.IsPointer(a) || !mir2.IsPointer(b) {
		return false
	}
	x, xok := mir2.PointerElem(a).(*mir2.StructTy)
	y, yok := mir2.PointerElem(b).(*mir2.StructTy)
	if !xok || !yok {
		return false
	}
	return !x.IsUnion && !y.IsUnion && len(x.FieldOffsets) == 0 && len(y.FieldOffsets) == 0 && x.StorageBytes == 0 && y.StorageBytes == 0 && reflect.DeepEqual(x.Fields, y.Fields)
}

func (c *importer) bindBytes(f function) error {
	allowed := map[string]bool{"ByteView_init": true, "ByteView_try_get": true, "ByteView_try_set": true, "ByteView_subview": true, "ByteBuffer_init": true, "ByteBuffer_clear": true, "ByteBuffer_try_push": true, "ByteBuffer_view": true}
	if !allowed[f.Library] || len(f.Body) != 0 || f.Elem != "" {
		return fmt.Errorf("invalid byte library binding")
	}
	if c.byteLibrary == nil {
		root := os.Getenv("MINZ_STDLIB")
		if root == "" {
			root = "stdlib"
		}
		source, err := os.ReadFile(filepath.Join(root, "mem", "bytes.nanz"))
		if err != nil {
			return fmt.Errorf("load byte library (set MINZ_STDLIB): %w", err)
		}
		lib, err := nanz.Parse(string(source), "mem.bytes")
		if err != nil {
			return err
		}
		if err := hir.Verify(lib); err != nil {
			return err
		}
		c.byteLibrary = lib
		c.byteLibrarySHA = fmt.Sprintf("%x", sha256.Sum256(source))
		c.m.Structs = append(c.m.Structs, lib.Structs...)
		c.m.Funcs = append(c.m.Funcs, lib.Funcs...)
	}
	var target *hir.Func
	for _, candidate := range c.byteLibrary.Funcs {
		if candidate.Name == f.Library {
			target = candidate
			break
		}
	}
	wrapper := c.funcs[f.Name]
	if target == nil || len(wrapper.Params) != len(target.Params) || !reflect.DeepEqual(wrapper.RetTy, target.RetTy) {
		return fmt.Errorf("byte library signature mismatch: %s", f.Library)
	}
	args := make([]hir.Expr, len(wrapper.Params))
	for i, p := range wrapper.Params {
		expected := target.Params[i].Ty
		// Nanz currently erases ^Struct parameter types to byte pointers in
		// public HIR. Recover the two record roles from this explicit v1 API
		// contract, while retaining the parsed scalar parameter signature.
		recordName := ""
		if i == 0 {
			recordName = strings.SplitN(f.Library, "_", 2)[0]
		}
		if f.Library == "ByteBuffer_view" && i == 1 || f.Library == "ByteView_subview" && i == 3 {
			recordName = "ByteView"
		}
		if recordName != "" {
			if expected != mir2.TyPtr {
				return fmt.Errorf("byte library record parameter ABI changed: %s", f.Library)
			}
			for _, st := range c.byteLibrary.Structs {
				if st.Name == recordName {
					expected = mir2.PointerTo(st)
					break
				}
			}
			if expected == mir2.TyPtr {
				return fmt.Errorf("byte library record missing: %s", recordName)
			}
		}
		if !byteABIEqual(p.Ty, expected) {
			return fmt.Errorf("byte library parameter %d schema mismatch: %s", i, f.Library)
		}
		args[i] = hir.Var(p.Name, p.Ty)
		if !reflect.DeepEqual(p.Ty, target.Params[i].Ty) {
			args[i] = hir.Cast(args[i], target.Params[i].Ty)
		}
	}
	call := &hir.CallExpr{Fn: target.Name, Args: args, Ty: target.RetTy}
	if target.RetTy == mir2.TyVoid {
		wrapper.Body = hir.Blk(&hir.ExprStmt{Expr: call}, &hir.ReturnStmt{})
	} else {
		wrapper.Body = hir.Blk(&hir.ReturnStmt{Val: call})
	}
	return nil
}

func (c *importer) ty(name string) (mir2.Ty, error) {
	if ty := c.types[name]; ty != nil {
		return ty, nil
	}
	if strings.HasPrefix(name, "ptr:") {
		elem, err := c.ty(strings.TrimPrefix(name, "ptr:"))
		if err != nil {
			return nil, err
		}
		return mir2.PointerTo(elem), nil
	}
	if strings.HasPrefix(name, "array:") {
		parts := strings.SplitN(strings.TrimPrefix(name, "array:"), ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad array type %q", name)
		}
		n, err := strconv.Atoi(parts[0])
		if err != nil || n < 1 || n > 256 {
			return nil, fmt.Errorf("bad array length")
		}
		elem, err := c.ty(parts[1])
		if err != nil {
			return nil, err
		}
		return &mir2.ArrayTy{Elem: elem, Len: n}, nil
	}
	return nil, fmt.Errorf("unknown wire type %q", name)
}

func (c *importer) fresh() string { c.serial++; return fmt.Sprintf("$record_value_%d", c.serial) }
func (c *importer) freeze(e hir.Expr, pre *[]hir.Stmt) hir.Expr {
	if e == nil {
		return nil
	}
	if e.ExprTy() == mir2.TyVoid {
		*pre = append(*pre, &hir.ExprStmt{Expr: e})
		return nil
	}
	if _, ok := e.ExprTy().(*mir2.StructTy); ok {
		return e
	}
	if _, ok := e.(*hir.IntLitExpr); ok {
		return e
	}
	name := c.fresh()
	*pre = append(*pre, &hir.VarDeclStmt{Name: name, Ty: e.ExprTy(), Init: e})
	return hir.Var(name, e.ExprTy())
}

func (c *importer) expr(n *node, pre *[]hir.Stmt) (hir.Expr, error) {
	if n == nil {
		return nil, fmt.Errorf("missing expression")
	}
	ty, err := c.ty(n.Ty)
	if err != nil {
		return nil, err
	}
	arg := func(i int) (hir.Expr, error) {
		if i >= len(n.Args) {
			return nil, fmt.Errorf("missing %s argument %d", n.Kind, i)
		}
		return c.expr(n.Args[i], pre)
	}
	var e hir.Expr
	switch n.Kind {
	case "nil":
		return nil, nil
	case "int":
		if ty == mir2.TyBool {
			return &hir.BoolLitExpr{Val: n.Value != 0}, nil
		}
		e = &hir.IntLitExpr{Val: n.Value, Ty: ty}
	case "self":
		e = hir.Var("$self", ty)
	case "var":
		local := c.locals[n.Name]
		if local == nil {
			return nil, fmt.Errorf("unknown local %s", n.Name)
		}
		if !reflect.DeepEqual(local, ty) {
			return nil, fmt.Errorf("local %s changed type", n.Name)
		}
		e = hir.Var(n.Name, ty)
	case "addr":
		local := c.locals[n.Name]
		if local == nil {
			return nil, fmt.Errorf("address of unknown local %s", n.Name)
		}
		e = &hir.AddrOfExpr{X: hir.Var(n.Name, local), Ty: ty}
	case "field":
		base, err := arg(0)
		if err != nil {
			return nil, err
		}
		st, ok := mir2.PointerElem(base.ExprTy()).(*mir2.StructTy)
		if !ok {
			return nil, fmt.Errorf("field base must point to a record")
		}
		found := false
		for i, f := range st.Fields {
			if f.Name == n.Name {
				if !reflect.DeepEqual(f.Ty, ty) {
					return nil, fmt.Errorf("field type mismatch")
				}
				e = &hir.FieldExpr{X: base, Field: n.Name, Offset: st.ByteOffset(i), Ty: ty}
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown field %s.%s", st.Name, n.Name)
		}
	case "binary":
		left, err := arg(0)
		if err != nil {
			return nil, err
		}
		right, err := arg(1)
		if err != nil {
			return nil, err
		}
		e = &hir.BinExpr{Op: n.Op, L: left, R: right, Ty: ty}
	case "cast":
		x, err := arg(0)
		if err != nil {
			return nil, err
		}
		e = hir.Cast(x, ty)
	case "truthy":
		x, err := arg(0)
		if err != nil {
			return nil, err
		}
		if x == nil {
			e = &hir.BoolLitExpr{Val: false}
		} else if x.ExprTy() == mir2.TyBool {
			e = x
		} else if mir2.IsPointer(x.ExprTy()) {
			e = &hir.BinExpr{Op: "!=", L: x, R: &hir.IntLitExpr{Val: 0, Ty: x.ExprTy()}, Ty: mir2.TyBool}
		} else {
			e = &hir.BoolLitExpr{Val: true}
		}
	case "index", "ptr_add", "store":
		base, err := arg(0)
		if err != nil {
			return nil, err
		}
		index, err := arg(1)
		if err != nil {
			return nil, err
		}
		if !mir2.IsPointer(base.ExprTy()) {
			return nil, fmt.Errorf("index requires pointer")
		}
		elem := mir2.PointerElem(base.ExprTy())
		if elem == nil {
			elem = mir2.TyU8
		}
		access := &hir.IndexExpr{Base: base, Idx: index, ElemTy: elem}
		if n.Kind == "ptr_add" {
			e = &hir.AddrOfExpr{X: access, Ty: ty}
		} else if n.Kind == "store" {
			value, err := arg(2)
			if err != nil {
				return nil, err
			}
			*pre = append(*pre, &hir.AssignStmt{Target: access, Val: value})
			return value, nil
		} else {
			e = access
		}
	case "view_project":
		if len(n.Args) != 2 {
			return nil, fmt.Errorf("invalid inline view projection")
		}
		data, err := arg(0)
		if err != nil {
			return nil, err
		}
		length, err := arg(1)
		if err != nil {
			return nil, err
		}
		if data == nil || !mir2.IsPointer(data.ExprTy()) || length == nil || (length.ExprTy() != mir2.TyU8 && length.ExprTy() != mir2.TyU16) {
			return nil, fmt.Errorf("invalid inline borrowed view")
		}
		switch n.Name {
		case "data":
			if !reflect.DeepEqual(ty, data.ExprTy()) {
				return nil, fmt.Errorf("view data projection type mismatch")
			}
			e = data
		case "len":
			if ty != mir2.TyU16 {
				return nil, fmt.Errorf("view length projection requires u16")
			}
			e = hir.Cast(length, mir2.TyU16)
		default:
			return nil, fmt.Errorf("unknown view projection")
		}
	case "meta_repeat":
		if ty != mir2.TyU8 || len(n.Args) != 3 || n.Args[0].Ty != "u8" || n.Args[1].Kind != "int" || n.Args[1].Ty != "u8" || n.Args[2].Kind != "int" || n.Args[2].Ty != "u8" {
			return nil, fmt.Errorf("invalid record meta repeat")
		}
		input, err := arg(0)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("repeat:%d:%d", n.Args[1].Value, n.Args[2].Value)
		f := c.metaHelpers[key]
		if f == nil {
			receipt, err := crystalmeta.GenerateRepeat(int(n.Args[1].Value), int(n.Args[2].Value))
			if err != nil {
				return nil, err
			}
			generated, err := lanz.Compile(receipt.Lanz, "record-meta-repeat")
			if err != nil {
				return nil, err
			}
			f = generated.Funcs[0]
			c.metaHelpers[key] = f
			c.m.Funcs = append(c.m.Funcs, f)
		}
		e = &hir.CallExpr{Fn: f.Name, Args: []hir.Expr{input}, Ty: f.RetTy}
	case "meta_layout_byte":
		st, ok := c.types[n.Op].(*mir2.StructTy)
		if !ok || ty != mir2.TyU8 || len(n.Args) != 1 || n.Args[0].Kind != "addr" {
			return nil, fmt.Errorf("invalid meta layout receiver")
		}
		base, err := arg(0)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(base.ExprTy(), mir2.PointerTo(st)) || !reflect.DeepEqual(c.locals[n.Args[0].Name], st) {
			return nil, fmt.Errorf("meta layout receiver type mismatch")
		}
		key := "layout:" + n.Op + ":" + n.Name
		f := c.metaHelpers[key]
		if f == nil {
			helper, receipt, err := crystalmeta.GenerateLayoutByte(st, n.Name)
			if err != nil {
				return nil, err
			}
			f = helper
			// Different records may share the same byte reader; retain per-record receipts.
			for _, existing := range c.m.Funcs {
				if existing.Name == f.Name {
					f = existing
					break
				}
			}
			c.metaHelpers[key] = f
			c.layoutReceipts = append(c.layoutReceipts, receipt)
			found := false
			for _, existing := range c.m.Funcs {
				if existing == f {
					found = true
				}
			}
			if !found {
				c.m.Funcs = append(c.m.Funcs, f)
			}
		}
		e = &hir.CallExpr{Fn: f.Name, Args: []hir.Expr{hir.Cast(base, mir2.TyPtr)}, Ty: mir2.TyU8}
	case "call":
		f := c.funcs[n.Name]
		if f == nil {
			return nil, fmt.Errorf("unknown callee %s", n.Name)
		}
		if f.IsGen {
			return nil, fmt.Errorf("generator must be consumed by a block")
		}
		args := make([]hir.Expr, len(n.Args))
		for i := range args {
			args[i], err = arg(i)
			if err != nil {
				return nil, err
			}
		}
		e = &hir.CallExpr{Fn: f.Name, Args: args, Ty: f.RetTy}
	default:
		return nil, fmt.Errorf("unsupported expression %s", n.Kind)
	}
	return c.freeze(e, pre), nil
}

func (c *importer) assignment(n *node, pre *[]hir.Stmt) (hir.Expr, error) {
	if len(n.Args) != 2 {
		return nil, fmt.Errorf("assignment arity")
	}
	target, value := n.Args[0], n.Args[1]
	if c.current.IsGen {
		ty, e := c.ty(value.Ty)
		if e != nil {
			return nil, e
		}
		if target.Name == "tag" || (!mir2.IsInt(ty) && ty != mir2.TyBool && !mir2.IsPointer(ty)) {
			return nil, fmt.Errorf("generator locals require scalar/pointer types and nonreserved names")
		}
	}
	if target.Kind == "var" && value.Kind == "construct_view" {
		ty, err := c.ty(value.Ty)
		if err != nil {
			return nil, err
		}
		st, ok := ty.(*mir2.StructTy)
		if !ok || len(value.Args) != 2 || len(st.Fields) != 2 || st.Fields[0].Name != hir.BorrowedViewData || st.Fields[1].Name != hir.BorrowedViewLen {
			return nil, fmt.Errorf("invalid borrowed view constructor")
		}
		data, err := c.expr(value.Args[0], pre)
		if err != nil {
			return nil, err
		}
		length, err := c.expr(value.Args[1], pre)
		if err != nil {
			return nil, err
		}
		if data == nil || !reflect.DeepEqual(data.ExprTy(), st.Fields[0].Ty) || length == nil || (length.ExprTy() != mir2.TyU8 && length.ExprTy() != mir2.TyU16) {
			return nil, fmt.Errorf("borrowed view field type mismatch")
		}
		if length.ExprTy() == mir2.TyU8 {
			length = hir.Cast(length, mir2.TyU16)
		}
		if existing := c.locals[target.Name]; existing == nil {
			c.locals[target.Name] = ty
			*pre = append(*pre, &hir.VarDeclStmt{Name: target.Name, Ty: ty})
		} else if !reflect.DeepEqual(existing, ty) {
			return nil, fmt.Errorf("borrowed view reassignment changed type")
		}
		base := &hir.AddrOfExpr{X: hir.Var(target.Name, ty), Ty: mir2.PointerTo(st)}
		*pre = append(*pre, &hir.AssignStmt{Target: &hir.FieldExpr{X: base, Field: hir.BorrowedViewData, Offset: st.ByteOffset(0), Ty: st.Fields[0].Ty}, Val: data}, &hir.AssignStmt{Target: &hir.FieldExpr{X: base, Field: hir.BorrowedViewLen, Offset: st.ByteOffset(1), Ty: mir2.TyU16}, Val: length})
		return hir.Var(target.Name, ty), nil
	}
	if target.Kind == "var" && c.locals[target.Name] == nil {
		ty, err := c.ty(value.Ty)
		if err != nil {
			return nil, err
		}
		if value.Kind == "construct" {
			st, ok := ty.(*mir2.StructTy)
			if !ok {
				return nil, fmt.Errorf("construct requires record type")
			}
			c.locals[target.Name] = ty
			*pre = append(*pre, &hir.VarDeclStmt{Name: target.Name, Ty: ty})
			args := []hir.Expr{&hir.AddrOfExpr{X: hir.Var(target.Name, ty), Ty: mir2.PointerTo(st)}}
			for _, a := range value.Args {
				e, err := c.expr(a, pre)
				if err != nil {
					return nil, err
				}
				args = append(args, e)
			}
			init := c.funcs[value.Name]
			if init == nil {
				return nil, fmt.Errorf("missing initializer")
			}
			*pre = append(*pre, &hir.ExprStmt{Expr: &hir.CallExpr{Fn: init.Name, Args: args, Ty: init.RetTy}})
			return hir.Var(target.Name, ty), nil
		}
		if value.Kind == "array" {
			array, ok := ty.(*mir2.ArrayTy)
			if !ok || len(value.Args) != 1 {
				return nil, fmt.Errorf("invalid array constructor")
			}
			fill, err := c.expr(value.Args[0], pre)
			if err != nil {
				return nil, err
			}
			c.locals[target.Name] = ty
			*pre = append(*pre, &hir.VarDeclStmt{Name: target.Name, Ty: ty})
			// ArrayTy allocates automatic storage. Fill at every invocation,
			// rather than relying on legacy static array initialization.
			index := c.fresh()
			ptr := &hir.AddrOfExpr{X: hir.Var(target.Name, ty), Ty: mir2.PointerTo(array.Elem)}
			*pre = append(*pre, &hir.ForRangeStmt{Var: index, Start: hir.U16(0), End: hir.U16(int64(array.Len)), Body: hir.Blk(&hir.AssignStmt{Target: &hir.IndexExpr{Base: ptr, Idx: hir.Var(index, mir2.TyU16), ElemTy: array.Elem}, Val: fill})})
			return hir.Var(target.Name, ty), nil
		}
		result, err := c.expr(value, pre)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("Nil local unsupported")
		}
		c.locals[target.Name] = ty
		*pre = append(*pre, &hir.VarDeclStmt{Name: target.Name, Ty: ty, Init: result})
		return hir.Var(target.Name, ty), nil
	}
	result, err := c.expr(value, pre)
	if err != nil {
		return nil, err
	}
	var lhs hir.Expr
	switch target.Kind {
	case "var":
		lhs = hir.Var(target.Name, c.locals[target.Name])
	case "field":
		if len(target.Args) != 1 || target.Args[0].Kind != "self" {
			return nil, fmt.Errorf("only self field assignment admitted")
		}
		st := mir2.PointerElem(c.current.Params[0].Ty).(*mir2.StructTy)
		for i, f := range st.Fields {
			if f.Name == target.Name {
				lhs = &hir.FieldExpr{X: hir.Var("$self", mir2.PointerTo(st)), Field: f.Name, Offset: st.ByteOffset(i), Ty: f.Ty}
				break
			}
		}
	default:
		return nil, fmt.Errorf("unsupported assignment target")
	}
	if lhs == nil {
		return nil, fmt.Errorf("unknown assignment target %s", target.Name)
	}
	*pre = append(*pre, &hir.AssignStmt{Target: lhs, Val: result})
	return result, nil
}

func cloneLocals(m map[string]mir2.Ty) map[string]mir2.Ty {
	r := map[string]mir2.Ty{}
	for k, v := range m {
		r[k] = v
	}
	return r
}
func (c *importer) block(nodes []*node, tail bool) (*hir.Block, error) {
	b := hir.Blk()
	for i, n := range nodes {
		last := tail && i == len(nodes)-1
		pre := []hir.Stmt{}
		var result hir.Expr
		var err error
		switch n.Kind {
		case "block":
			child, e := c.block(n.Body, last)
			if e != nil {
				return nil, e
			}
			b.Body = append(b.Body, child)
			continue
		case "return":
			if len(n.Args) > 0 {
				result, err = c.expr(n.Args[0], &pre)
			}
			if err != nil {
				return nil, err
			}
			if c.current.IsGen && result != nil {
				return nil, fmt.Errorf("generator completion cannot return a value")
			}
			pre = append(pre, &hir.ReturnStmt{Val: result})
		case "assign":
			result, err = c.assignment(n, &pre)
		case "if":
			cond, e := c.expr(n.Args[0], &pre)
			if e != nil {
				return nil, e
			}
			before := cloneLocals(c.locals)
			yes, e := c.block(n.Yes, last)
			if e != nil {
				return nil, e
			}
			c.locals = cloneLocals(before)
			no, e := c.block(n.No, last)
			if e != nil {
				return nil, e
			}
			c.locals = before
			pre = append(pre, &hir.IfStmt{Cond: cond, Then: yes, Else: no})
			last = false
		case "yield":
			if !c.current.IsGen || len(n.Args) != 1 {
				return nil, fmt.Errorf("yield outside generator or invalid arity")
			}
			value, e := c.expr(n.Args[0], &pre)
			if e != nil {
				return nil, e
			}
			if value == nil || !reflect.DeepEqual(value.ExprTy(), c.current.ElemTy) {
				return nil, fmt.Errorf("generator yield type mismatch")
			}
			pre = append(pre, &hir.YieldStmt{Val: value})
			last = false
		case "gen_for":
			if c.current.IsGen {
				return nil, fmt.Errorf("nested generator consumption is not admitted")
			}
			generator := c.funcs[n.Name]
			if generator == nil || !generator.IsGen || n.Ty != "void" || n.Op == "" || len(n.Args) != len(generator.Params) {
				return nil, fmt.Errorf("invalid generator consumption")
			}
			args := make([]hir.Expr, len(n.Args))
			for i, a := range n.Args {
				value, e := c.expr(a, &pre)
				if e != nil {
					return nil, e
				}
				if value == nil || !reflect.DeepEqual(value.ExprTy(), generator.Params[i].Ty) {
					return nil, fmt.Errorf("generator argument type mismatch")
				}
				args[i] = value
			}
			before := cloneLocals(c.locals)
			if c.locals[n.Op] != nil {
				return nil, fmt.Errorf("generator block argument shadows binding")
			}
			c.locals[n.Op] = generator.ElemTy
			inner, e := c.block(n.Body, false)
			c.locals = before
			if e != nil {
				return nil, e
			}
			pre = append(pre, &hir.GenForStmt{Var: n.Op, Gen: generator.Name, ElemTy: generator.ElemTy, Args: args, Body: inner})
			last = false
		case "foreach_view":
			if len(n.Args) != 1 || n.Ty != "void" || n.Name == "" {
				return nil, fmt.Errorf("invalid mutable view loop")
			}
			if last && c.current.RetTy != mir2.TyVoid {
				return nil, fmt.Errorf("slice each result is not scalar")
			}
			base, e := c.expr(n.Args[0], &pre)
			if e != nil {
				return nil, e
			}
			if base == nil {
				return nil, fmt.Errorf("missing view address")
			}
			st, ok := mir2.PointerElem(base.ExprTy()).(*mir2.StructTy)
			elem, e := c.ty(n.Op)
			if e != nil {
				return nil, e
			}
			var expected *mir2.StructTy
			// Use the registered descriptor name when checking its shared schema.
			if ok {
				expected, e = hir.BorrowedViewType(st.Name, elem)
			}
			if !ok || e != nil || !reflect.DeepEqual(st, expected) || (elem != mir2.TyU8 && elem != mir2.TyU16) {
				return nil, fmt.Errorf("invalid mutable borrowed view schema")
			}
			data := &hir.FieldExpr{X: base, Field: hir.BorrowedViewData, Offset: st.ByteOffset(0), Ty: st.Fields[0].Ty}
			length := &hir.FieldExpr{X: base, Field: hir.BorrowedViewLen, Offset: st.ByteOffset(1), Ty: mir2.TyU16}
			indexName := c.fresh()
			index := hir.Var(indexName, mir2.TyU16)
			pre = append(pre, &hir.VarDeclStmt{Name: indexName, Ty: mir2.TyU16, Init: hir.U16(0)})
			before := cloneLocals(c.locals)
			if c.locals[n.Name] != nil {
				return nil, fmt.Errorf("view block argument shadows an existing binding")
			}
			c.locals[n.Name] = elem
			inner, e := c.block(n.Body, false)
			c.locals = before
			if e != nil {
				return nil, e
			}
			// Advance the hidden index before executing the block: `next`
			// reaches the next condition, while descriptor fields stay live.
			loop := hir.Blk(&hir.VarDeclStmt{Name: n.Name, Ty: elem, Init: &hir.IndexExpr{Base: data, Idx: index, ElemTy: elem}}, &hir.AssignStmt{Target: index, Val: &hir.BinExpr{Op: "+", L: index, R: hir.U16(1), Ty: mir2.TyU16}})
			loop.Body = append(loop.Body, inner.Body...)
			pre = append(pre, &hir.WhileStmt{Cond: &hir.BinExpr{Op: "<", L: index, R: length, Ty: mir2.TyBool}, Body: loop})
			last = false
		case "foreach":
			if len(n.Args) != 2 || n.Ty != "void" || n.Name == "" {
				return nil, fmt.Errorf("invalid foreach wire node")
			}
			if last && c.current.RetTy != mir2.TyVoid {
				return nil, fmt.Errorf("slice each result is not a scalar")
			}
			elem, e := c.ty(n.Op)
			if e != nil {
				return nil, e
			}
			if elem != mir2.TyU8 && elem != mir2.TyU16 {
				return nil, fmt.Errorf("foreach requires unsigned byte/word elements")
			}
			ptr, e := c.expr(n.Args[0], &pre)
			if e != nil {
				return nil, e
			}
			count, e := c.expr(n.Args[1], &pre)
			if e != nil {
				return nil, e
			}
			if ptr == nil || !mir2.IsPointer(ptr.ExprTy()) || !reflect.DeepEqual(mir2.PointerElem(ptr.ExprTy()), elem) {
				return nil, fmt.Errorf("foreach pointer element mismatch")
			}
			if count == nil || (count.ExprTy() != mir2.TyU8 && count.ExprTy() != mir2.TyU16) {
				return nil, fmt.Errorf("foreach count must be u8 or u16")
			}
			before := cloneLocals(c.locals)
			if c.locals[n.Name] != nil {
				return nil, fmt.Errorf("foreach block argument shadows an existing binding")
			}
			c.locals[n.Name] = elem
			inner, e := c.block(n.Body, false)
			c.locals = before
			if e != nil {
				return nil, e
			}
			pre = append(pre, &hir.ForEachStmt{Var: n.Name, ElemTy: elem, Ptr: ptr, Start: hir.U16(0), Len: count, Body: inner})
			last = false
		case "break":
			pre = append(pre, &hir.BreakStmt{})
			last = false
		case "continue":
			pre = append(pre, &hir.ContinueStmt{})
			last = false
		case "while":
			if last && c.current.RetTy != mir2.TyVoid {
				return nil, fmt.Errorf("while-valued scalar result")
			}
			before := cloneLocals(c.locals)
			condPre := []hir.Stmt{}
			cond, e := c.expr(n.Args[0], &condPre)
			if e != nil {
				return nil, e
			}
			inner, e := c.block(n.Body, false)
			if e != nil {
				return nil, e
			}
			c.locals = before
			loop := hir.Blk()
			loop.Body = append(loop.Body, condPre...)
			loop.Body = append(loop.Body, &hir.IfStmt{Cond: &hir.UnaryExpr{Op: "!", X: cond, Ty: mir2.TyBool}, Then: hir.Blk(&hir.BreakStmt{})})
			loop.Body = append(loop.Body, inner)
			pre = append(pre, &hir.WhileStmt{Cond: &hir.BoolLitExpr{Val: true}, Body: loop})
		default:
			result, err = c.expr(n, &pre)
		}
		if err != nil {
			return nil, err
		}
		if last && n.Kind != "return" {
			if c.current.RetTy == mir2.TyVoid {
				pre = append(pre, &hir.ReturnStmt{})
			} else if result == nil {
				return nil, fmt.Errorf("missing scalar return")
			} else {
				pre = append(pre, &hir.ReturnStmt{Val: result})
			}
		}
		b.Body = append(b.Body, pre...)
	}
	return b, nil
}

func validateImportedGenerator(f *hir.Func) error {
	if !f.IsGen {
		return nil
	}
	names := map[string]bool{"tag": true}
	fieldOK := func(ty mir2.Ty) bool { return mir2.IsInt(ty) || ty == mir2.TyBool || mir2.IsPointer(ty) }
	for _, p := range f.Params {
		if names[p.Name] || !fieldOK(p.Ty) {
			return fmt.Errorf("generator parameter is reserved or not scalar/pointer")
		}
		names[p.Name] = true
	}
	yields := 0
	var walk func(*hir.Block, bool) error
	walk = func(b *hir.Block, inFor bool) error {
		if b == nil {
			return nil
		}
		for _, stmt := range b.Body {
			switch s := stmt.(type) {
			case *hir.YieldStmt:
				yields++
				if inFor {
					return fmt.Errorf("yield inside foreach is not yet admitted")
				}
			case *hir.VarDeclStmt:
				if names[s.Name] || !fieldOK(s.Ty) {
					return fmt.Errorf("generator local is repeated, reserved or aggregate")
				}
				names[s.Name] = true
			case *hir.Block:
				if err := walk(s, inFor); err != nil {
					return err
				}
			case *hir.IfStmt:
				if err := walk(s.Then, inFor); err != nil {
					return err
				}
				if err := walk(s.Else, inFor); err != nil {
					return err
				}
			case *hir.WhileStmt:
				if err := walk(s.Body, inFor); err != nil {
					return err
				}
			case *hir.ForEachStmt:
				if err := walk(s.Body, true); err != nil {
					return err
				}
			case *hir.GenForStmt:
				return fmt.Errorf("nested generator consumption is not yet admitted")
			}
		}
		return nil
	}
	if err := walk(f.Body, false); err != nil {
		return err
	}
	if yields < 1 || yields > 254 {
		return fmt.Errorf("generator needs 1..254 yield sites")
	}
	return nil
}

func importModule(w module) (*importer, error) {
	if w.Protocol != "crystal-record-hir-v1" {
		return nil, fmt.Errorf("bad record protocol")
	}
	c := &importer{m: &hir.Module{Name: "crystal_records", NonNullObjects: true}, types: map[string]mir2.Ty{"u8": mir2.TyU8, "u16": mir2.TyU16, "i8": mir2.TyI8, "i16": mir2.TyI16, "bool": mir2.TyBool, "void": mir2.TyVoid}, funcs: map[string]*hir.Func{}, metaHelpers: map[string]*hir.Func{}}
	for _, r := range w.Records {
		st := &mir2.StructTy{Name: r.Name}
		c.types[r.Name] = st
		c.m.Structs = append(c.m.Structs, st)
	}
	for _, r := range w.Records {
		st := c.types[r.Name].(*mir2.StructTy)
		for _, f := range r.Fields {
			ty, e := c.ty(f.Ty)
			if e != nil {
				return nil, e
			}
			st.Fields = append(st.Fields, mir2.StructField{Name: f.Name, Ty: ty})
		}
	}
	for _, r := range w.Records {
		if r.Kind == "borrowed_view" {
			st := c.types[r.Name].(*mir2.StructTy)
			if len(st.Fields) != 2 || !mir2.IsPointer(st.Fields[0].Ty) {
				return nil, fmt.Errorf("invalid borrowed view schema")
			}
			expected, err := hir.BorrowedViewType(r.Name, mir2.PointerElem(st.Fields[0].Ty))
			if err != nil {
				return nil, err
			}
			if !reflect.DeepEqual(st, expected) {
				return nil, fmt.Errorf("noncanonical borrowed view schema")
			}
		} else if r.Kind != "" {
			return nil, fmt.Errorf("unknown record kind %q", r.Kind)
		}
	}
	for _, f := range w.Functions {
		ret, e := c.ty(f.Ret)
		if e != nil {
			return nil, e
		}
		fn := &hir.Func{Name: f.Name, RetTy: ret}
		if f.Elem != "" {
			elem, e := c.ty(f.Elem)
			if e != nil {
				return nil, e
			}
			if ret != mir2.TyVoid || f.Owner != "" || (elem != mir2.TyU8 && elem != mir2.TyU16 && elem != mir2.TyI8 && elem != mir2.TyI16 && elem != mir2.TyBool) {
				return nil, fmt.Errorf("invalid generator signature")
			}
			fn.IsGen = true
			fn.ElemTy = elem
		}
		if f.Owner != "" {
			owner, e := c.ty(f.Owner)
			if e != nil {
				return nil, e
			}
			fn.Params = append(fn.Params, hir.Param{Name: "$self", Ty: mir2.PointerTo(owner)})
		}
		for _, p := range f.Params {
			ty, e := c.ty(p.Ty)
			if e != nil {
				return nil, e
			}
			fn.Params = append(fn.Params, hir.Param{Name: p.Name, Ty: ty})
		}
		c.funcs[fn.Name] = fn
		c.m.Funcs = append(c.m.Funcs, fn)
	}
	for _, f := range w.Functions {
		if f.Library != "" {
			if err := c.bindBytes(f); err != nil {
				return nil, err
			}
			continue
		}
		c.current = c.funcs[f.Name]
		c.locals = map[string]mir2.Ty{}
		for _, p := range c.current.Params {
			c.locals[p.Name] = p.Ty
		}
		body, e := c.block(f.Body, true)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", f.Source, e)
		}
		c.current.Body = body
		if e := validateImportedGenerator(c.current); e != nil {
			return nil, fmt.Errorf("%s: %w", f.Source, e)
		}
	}
	if e := hir.Verify(c.m); e != nil {
		return nil, e
	}
	return c, nil
}

func sourceRecordName(records []record, name string) string {
	for _, r := range records {
		if r.Name == name {
			return r.Native
		}
	}
	return ""
}

func run() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: crystal-record-hir module.json cases.json output-prefix")
	}
	src, e := os.ReadFile(os.Args[1])
	if e != nil {
		return e
	}
	var w module
	if e = json.Unmarshal(src, &w); e != nil {
		return e
	}
	compiled, e := importModule(w)
	if e != nil {
		return e
	}
	m := compiled.m
	data, e := os.ReadFile(os.Args[2])
	if e != nil {
		return e
	}
	var cases []testCase
	if e = json.Unmarshal(data, &cases); e != nil {
		return e
	}
	for _, tc := range cases {
		found := false
		for _, f := range w.Functions {
			if f.Owner == "" && f.Source == tc.Source {
				m.Asserts = append(m.Asserts, hir.Assert{FuncName: f.Name, Args: tc.Args, Expected: tc.Expected, Source: fmt.Sprintf("record %s%v", tc.Source, tc.Args)})
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown entry %s", tc.Source)
		}
	}
	expanded := hir.CloneModule(m)
	raw := hir.LowerModule(expanded)
	for _, item := range []struct{ suffix, text string }{{"-pre.hir", m.Dump()}, {"-expanded.hir", expanded.Dump()}, {"-raw.mir2", raw.Dump()}} {
		if e = os.WriteFile(os.Args[3]+item.suffix, []byte(item.text), 0644); e != nil {
			return e
		}
	}
	if e = pipeline.RunAssertsMIR2(m, raw); e != nil {
		return fmt.Errorf("raw MIR2: %w", e)
	}
	steps, compileErr := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true})
	for _, item := range []struct{ suffix, text string }{{".hir", m.Dump()}, {".mir2", steps.MIR2Opt}, {".a80", steps.Assembly}} {
		if e = os.WriteFile(os.Args[3]+item.suffix, []byte(item.text), 0644); e != nil {
			return e
		}
	}
	if compileErr != nil {
		return compileErr
	}
	var layouts []map[string]any
	for _, st := range m.Structs {
		var fields []map[string]any
		for j, f := range st.Fields {
			fields = append(fields, map[string]any{"name": f.Name, "ty": f.Ty.String(), "offset": st.ByteOffset(j), "bytes": mir2.ByteWidth(f.Ty)})
		}
		layouts = append(layouts, map[string]any{"name": st.Name, "native": sourceRecordName(w.Records, st.Name), "bytes": mir2.ByteWidth(st), "fields": fields})
	}
	encoded, e := json.MarshalIndent(map[string]any{"target": "z80", "pointer_bytes": mir2.ByteWidth(mir2.TyPtr), "layouts": layouts, "cases_per_engine": len(cases), "meta_layout": compiled.layoutReceipts, "byte_library_sha256": compiled.byteLibrarySHA}, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(os.Args[3]+"-layout.json", encoded, 0644); e != nil {
		return e
	}
	fmt.Printf("Crystal records/pointers: %d cases passed on raw MIR2, optimized MIR2 and Z80\n", len(cases))
	return nil
}

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
