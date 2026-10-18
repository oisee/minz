package abap

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
)

type SplitStmt struct {
	Source    Expr_
	Delimiter byte
	Targets   []string
}

func (*SplitStmt) abapStmt() {}

func parseSplitStatement(t []string) (Decl, error) {
	fail := func() (Decl, error) {
		return nil, fmt.Errorf("SPLIT: unsupported form/addition %s", strings.Join(t, " "))
	}
	if len(t) < 7 || !strings.EqualFold(t[2], "AT") || !strings.EqualFold(t[4], "INTO") || t[len(t)-1] != "." {
		return fail()
	}
	if strings.EqualFold(t[5], "TABLE") {
		return nil, fmt.Errorf("SPLIT INTO TABLE is not implemented")
	}
	source, err := parseExpression(t[1:2])
	if err != nil {
		return nil, err
	}
	delimiter, err := expressionAtom(t[3])
	if err != nil {
		return nil, err
	}
	d, ok := delimiter.(*StringLit)
	if !ok || len(d.Val) != 1 || d.Val[0] > 127 {
		return nil, fmt.Errorf("SPLIT AT requires one ASCII character literal")
	}
	s := &SplitStmt{Source: source, Delimiter: d.Val[0]}
	for _, target := range t[5 : len(t)-1] {
		e, err := expressionAtom(target)
		if err != nil {
			return nil, err
		}
		switch e.(type) {
		case *VarRef, *FieldAccess:
		default:
			return fail()
		}
		s.Targets = append(s.Targets, strings.ToLower(target))
	}
	return &formBodyDecl{stmt: s}, nil
}

func (l *lowerer) lowerSplit(s *SplitStmt) (hir.Stmt, error) {
	src, ok, err := l.characterExpr(s.Source)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("SPLIT source requires c/string")
	}
	l.requireSplitRuntime()
	ss := []hir.Stmt{charCall("_abap_split_begin", src.ptr, textValueLength(src), &hir.IntLitExpr{Val: int64(s.Delimiter), Ty: mir2.TyU8})}
	for i, target := range s.Targets {
		dst, ok, err := l.characterExpr(parseTokenExpr(target))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("SPLIT target %s requires c/string", target)
		}
		last := int64(0)
		if i == len(s.Targets)-1 {
			last = 1
		}
		name := fmt.Sprintf("_abap_split_len_%d", l.actualCounter)
		l.actualCounter++
		length := &hir.VarRefExpr{Name: name, Ty: mir2.TyU16}
		ss = append(ss, &hir.VarDeclStmt{Name: name, Ty: mir2.TyU16, Init: &hir.CallExpr{Fn: "_abap_split_take", Args: []hir.Expr{&hir.IntLitExpr{Val: last, Ty: mir2.TyU8}}, Ty: mir2.TyU16}})
		ptr := &hir.CallExpr{Fn: "_abap_split_data", Ty: mir2.TyPtr}
		if dst.dynamic {
			ss = append(ss, charCall("_abap_string_assign", dst.object, ptr, length))
		} else {
			ss = append(ss, &hir.IfStmt{Cond: &hir.BinExpr{Op: ">", L: length, R: dst.length, Ty: mir2.TyBool}, Then: &hir.Block{Body: []hir.Stmt{charCall("sy_set_subrc", i32Lit(4))}}}, charCall("_abap_char_copy", dst.ptr, dst.length, ptr, length))
		}
	}
	return &hir.Block{Body: ss}, nil
}

func (l *lowerer) requireSplitRuntime() {
	capacity := l.stringCapacity
	for _, b := range l.characters {
		if n, ok := b.length.(*hir.IntLitExpr); ok && n.Val > int64(capacity) {
			capacity = int(n.Val)
		}
	}
	for i, g := range l.hm.Globals {
		if g.Name == "_abap_split_work_buffer" {
			if a := g.Ty.(*mir2.ArrayTy); a.Len < capacity {
				l.hm.Globals[i].Ty = mir2.NewArray(mir2.TyU8, capacity)
			}
			return
		}
	}
	l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: "_abap_split_work_buffer", Ty: mir2.NewArray(mir2.TyU8, capacity)})
}

// Adapted from the reviewed SPLIT portion of stopped WIP ec5aec07b. Delimiter
// remainder is retained in the final target; only actual target truncation
// changes SY-SUBRC to 4. The source snapshot makes overlapping targets safe.
const splitRuntime = `
@extern fun _abap_split_work() -> ^u8
@extern fun sy_set_subrc(value: i32) -> void
global _abap_split_pos: u16 = 0
global _abap_split_size: u16 = 0
global _abap_split_start: u16 = 0
global _abap_split_delim: u8 = 0
fun _abap_split_begin(src: ^u8, length: u16, delimiter: u8) -> void {
 var work: ^u8 = _abap_split_work()
 var i: u16 = 0
 while i < length { work[i] = src[i]
 i = i + 1 }
 _abap_split_pos = 0
 _abap_split_size = length
 _abap_split_delim = delimiter
 sy_set_subrc(0)
}
fun _abap_split_take(last: u8) -> u16 {
 var work: ^u8 = _abap_split_work()
 var start: u16 = _abap_split_pos
 _abap_split_start = start
 var end: u16 = start
 while end < _abap_split_size {
  if last == 0 && work[end] == _abap_split_delim { break }
  end = end + 1
 }
 _abap_split_pos = end
 if end < _abap_split_size { _abap_split_pos = end + 1 }
 return end - start
}
fun _abap_split_data() -> ^u8 { return _abap_split_work() + _abap_split_start }
`

func emitSplitRuntime(hm *hir.Module) error {
	needed := false
	for _, g := range hm.Globals {
		if g.Name == "_abap_split_work_buffer" {
			needed = true
		}
	}
	if !needed {
		return nil
	}
	m, err := nanz.Parse(splitRuntime, "abap-split-runtime.nanz")
	if err != nil {
		return err
	}
	hm.Globals = append(hm.Globals, m.Globals...)
	for _, f := range m.Funcs {
		if !f.IsExtern {
			f.Internal = true
			hm.Funcs = append(hm.Funcs, f)
		}
	}
	hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_split_work", RetTy: mir2.TyPtr, Internal: true, Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.AddrOfExpr{Sym: "_abap_split_work_buffer"}}}}})
	return nil
}
