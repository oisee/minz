// Research importer for native Crystal records. All memory layout is computed
// with MIR2 target types; host Crystal offsets/pointer widths are never imported.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
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
	Ret                 string
	Body                []*node
}
type record struct {
	Name, Native string
	Fields       []field
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
	m       *hir.Module
	types   map[string]mir2.Ty
	funcs   map[string]*hir.Func
	locals  map[string]mir2.Ty
	current *hir.Func
	serial  int
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
	case "call":
		f := c.funcs[n.Name]
		if f == nil {
			return nil, fmt.Errorf("unknown callee %s", n.Name)
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

func importModule(w module) (*hir.Module, error) {
	if w.Protocol != "crystal-record-hir-v1" {
		return nil, fmt.Errorf("bad record protocol")
	}
	c := &importer{m: &hir.Module{Name: "crystal_records", NonNullObjects: true}, types: map[string]mir2.Ty{"u8": mir2.TyU8, "u16": mir2.TyU16, "i8": mir2.TyI8, "i16": mir2.TyI16, "bool": mir2.TyBool, "void": mir2.TyVoid}, funcs: map[string]*hir.Func{}}
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
	for _, f := range w.Functions {
		ret, e := c.ty(f.Ret)
		if e != nil {
			return nil, e
		}
		fn := &hir.Func{Name: f.Name, RetTy: ret}
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
	}
	if e := hir.Verify(c.m); e != nil {
		return nil, e
	}
	return c.m, nil
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
	m, e := importModule(w)
	if e != nil {
		return e
	}
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
	if e = pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); e != nil {
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
	for i, st := range m.Structs {
		var fields []map[string]any
		for j, f := range st.Fields {
			fields = append(fields, map[string]any{"name": f.Name, "ty": f.Ty.String(), "offset": st.ByteOffset(j), "bytes": mir2.ByteWidth(f.Ty)})
		}
		layouts = append(layouts, map[string]any{"name": st.Name, "native": w.Records[i].Native, "bytes": mir2.ByteWidth(st), "fields": fields})
	}
	encoded, e := json.MarshalIndent(map[string]any{"target": "z80", "pointer_bytes": mir2.ByteWidth(mir2.TyPtr), "layouts": layouts, "cases_per_engine": len(cases)}, "", "  ")
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
