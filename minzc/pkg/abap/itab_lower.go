package abap

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Flat structures are byte images: TYPE c components own their padded bytes
// and TYPE i components are four little-endian bytes, in declaration order and
// without padding. A standard internal table is a fixed-capacity array of such
// line images plus a TYPE i line count. The capacity is a compile-time fact;
// exceeding it is terminal runtime fault 7 before any table byte is written.

// tableCapacityFault is the _abap_fault code of an APPEND beyond capacity.
const tableCapacityFault = 7

// maxTableBytes bounds one table so 16-bit row offsets cannot wrap.
const maxTableBytes = 32768

type structComponent struct {
	name, abapTy   string
	length, offset int
}

type abapStruct struct {
	name    string
	comps   []structComponent
	size    int
	initial []byte
	emitted bool // initial-line image global exists
}

type structObject struct {
	ty   *abapStruct
	base hir.Expr
}

type tableObject struct {
	line     *abapStruct
	capacity int
	base     hir.Expr
	count    string // TYPE i line-count global
}

func (l *lowerer) declareType(td *TypesDecl) error {
	if l.types[td.Name] != nil {
		return fmt.Errorf("TYPES %s: duplicate type", td.Name)
	}
	st := &abapStruct{name: td.Name}
	for _, c := range td.Components {
		sc := structComponent{name: c.Name, abapTy: c.AbapTy, offset: st.size}
		switch c.AbapTy {
		case "i":
			sc.length = 4
			st.initial = append(st.initial, 0, 0, 0, 0)
		case "c":
			sc.length = characterLength(c.Length)
			for i := 0; i < sc.length; i++ {
				st.initial = append(st.initial, ' ')
			}
		default:
			return fmt.Errorf("TYPES %s component %s: TYPE %s is not admitted", td.Name, c.Name, c.AbapTy)
		}
		st.size += sc.length
		if st.size > maxTableBytes {
			return fmt.Errorf("TYPES %s: structure exceeds %d bytes", td.Name, maxTableBytes)
		}
		st.comps = append(st.comps, sc)
	}
	l.types[td.Name] = st
	return nil
}

// aggregateDecl reports DATA of a declared structure type or a table.
func (l *lowerer) aggregateDecl(d *DataDecl) bool {
	return d.Table != nil || l.types[d.AbapTy] != nil
}

func (l *lowerer) structGlobal(d *DataDecl) error {
	st := l.types[d.AbapTy]
	if d.Value != nil || d.Length != 0 {
		return fmt.Errorf("DATA %s: structure VALUE/LENGTH additions are not implemented", d.Name)
	}
	l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: d.Name, Ty: mir2.NewArray(mir2.TyU8, st.size), Init: append([]byte(nil), st.initial...)})
	l.structs[d.Name] = &structObject{ty: st, base: &hir.AddrOfExpr{Sym: d.Name}}
	return nil
}

func (l *lowerer) tableGlobal(d *DataDecl) error {
	st := l.types[d.AbapTy]
	if st == nil {
		return fmt.Errorf("DATA %s: table line type %s is not a flat structure declared with TYPES BEGIN OF; tables of elementary or unknown line types are not implemented", d.Name, d.AbapTy)
	}
	if d.Value != nil {
		return fmt.Errorf("DATA %s: table VALUE is not implemented", d.Name)
	}
	total := d.Table.Capacity * st.size
	if d.Table.Capacity < 1 || total > maxTableBytes {
		return fmt.Errorf("DATA %s: capacity %d x %d-byte lines exceeds the %d-byte table limit", d.Name, d.Table.Capacity, st.size, maxTableBytes)
	}
	count := "_itab_count_" + d.Name
	l.hm.Globals = append(l.hm.Globals,
		mir2.Global{Name: d.Name, Ty: mir2.NewArray(mir2.TyU8, total), Init: make([]byte, total)},
		mir2.Global{Name: count, Ty: mir2.TyI32})
	l.tables[d.Name] = &tableObject{line: st, capacity: d.Table.Capacity, base: &hir.AddrOfExpr{Sym: d.Name}, count: count}
	return nil
}

// shadowAggregate hides report structures/tables behind a procedure object.
func (l *lowerer) shadowAggregate(name string) {
	delete(l.structs, name)
	delete(l.tables, name)
}

func offsetPtr(base hir.Expr, offset int) hir.Expr {
	if offset == 0 {
		return base
	}
	return &hir.BinExpr{Op: "+", L: base, R: charLen(offset), Ty: mir2.TyPtr}
}

func i32Lit(v int64) hir.Expr { return &hir.IntLitExpr{Val: v, Ty: mir2.TyI32} }

// component resolves the selector a-b of a declared report structure.
func (l *lowerer) component(objName, field string) (*structObject, *structComponent, error) {
	if strings.EqualFold(objName, "sy") {
		return nil, nil, fmt.Errorf("SY-%s: system fields are not writable/addressable components here", strings.ToUpper(field))
	}
	obj := l.structs[objName]
	if obj == nil {
		if l.tables[objName] != nil {
			return nil, nil, fmt.Errorf("%s-%s: component access on an internal table body is not implemented", objName, field)
		}
		return nil, nil, fmt.Errorf("%s-%s: %s is not a declared structure", objName, field, objName)
	}
	for i := range obj.ty.comps {
		if obj.ty.comps[i].name == strings.ToLower(field) {
			return obj, &obj.ty.comps[i], nil
		}
	}
	return nil, nil, fmt.Errorf("structure %s has no component %s", objName, field)
}

func splitSelector(name string) (string, string, bool) {
	parts := strings.Split(name, "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func componentCharacter(obj *structObject, c *structComponent) characterBinding {
	return characterBinding{ptr: offsetPtr(obj.base, c.offset), length: charLen(c.length)}
}

// aggregateOperandError refuses a whole structure/table as an elementary operand.
func (l *lowerer) aggregateOperandError(name string) error {
	if l.structs[name] != nil {
		return fmt.Errorf("structure %s cannot be used as an elementary operand", name)
	}
	if l.tables[name] != nil {
		return fmt.Errorf("internal table %s cannot be used as an elementary operand", name)
	}
	return nil
}

func (l *lowerer) lowerFieldAccess(e *FieldAccess) (hir.Expr, error) {
	v, ok := e.Obj.(*VarRef)
	if !ok {
		return nil, fmt.Errorf("unsupported component access")
	}
	obj, c, err := l.component(v.Name, e.Field)
	if err != nil {
		return nil, err
	}
	if c.abapTy == "c" {
		return componentCharacter(obj, c).ptr, nil
	}
	return &hir.LoadExpr{Ptr: offsetPtr(obj.base, c.offset), Ty: mir2.TyI32}, nil
}

func (l *lowerer) lowerBuiltinCall(e *FuncCall) (hir.Expr, error) {
	if e.Name != "lines" {
		return nil, fmt.Errorf("built-in function %s( ) is not implemented", e.Name)
	}
	if len(e.Args) != 1 {
		return nil, fmt.Errorf("lines( ) needs exactly one internal table argument")
	}
	v, ok := e.Args[0].(*VarRef)
	if !ok || l.tables[v.Name] == nil {
		return nil, fmt.Errorf("lines( ) argument must be a declared internal table")
	}
	return &hir.VarRefExpr{Name: l.tables[v.Name].count, Ty: mir2.TyI32}, nil
}

// integerTarget is a writable TYPE i variable or TYPE i structure component.
func (l *lowerer) integerTarget(name string) (hir.Expr, error) {
	if objName, field, ok := splitSelector(name); ok {
		obj, c, err := l.component(objName, field)
		if err != nil {
			return nil, err
		}
		if c.abapTy != "i" {
			return nil, fmt.Errorf("%s: TYPE i target required", name)
		}
		return &hir.DerefExpr{Ptr: offsetPtr(obj.base, c.offset), Ty: mir2.TyI32}, nil
	}
	if err := l.aggregateOperandError(name); err != nil {
		return nil, err
	}
	if _, character := l.characters[name]; character || l.varTypes[name] != mir2.TyI32 {
		return nil, fmt.Errorf("%s: TYPE i target required", name)
	}
	return l.variableTarget(name, mir2.TyI32), nil
}

// lowerAggregateAssign handles component targets and structure MOVE.
// It reports handled=false for ordinary elementary targets.
func (l *lowerer) lowerAggregateAssign(s *AssignStmt) (hir.Stmt, bool, error) {
	if objName, field, ok := splitSelector(s.Target); ok {
		obj, c, err := l.component(objName, field)
		if err != nil {
			return nil, true, err
		}
		if c.abapTy == "c" {
			st, err := l.characterAssign(componentCharacter(obj, c), s.Val)
			return st, true, err
		}
		if _, text := s.Val.(*StringLit); text {
			return nil, true, fmt.Errorf("assignment to %s: character-to-integer conversion is not implemented", s.Target)
		}
		if _, character, err := l.characterExpr(s.Val); err != nil {
			return nil, true, err
		} else if character {
			return nil, true, fmt.Errorf("assignment to %s: character conversion is not implemented", s.Target)
		}
		val, err := l.lowerExpr(s.Val)
		if err != nil {
			return nil, true, err
		}
		if val.ExprTy() != mir2.TyI32 {
			return nil, true, fmt.Errorf("assignment to %s: conversion from %s is not implemented", s.Target, val.ExprTy())
		}
		return &hir.AssignStmt{Target: &hir.DerefExpr{Ptr: offsetPtr(obj.base, c.offset), Ty: mir2.TyI32}, Val: val}, true, nil
	}
	if dst := l.structs[s.Target]; dst != nil {
		v, ok := s.Val.(*VarRef)
		var src *structObject
		if ok {
			src = l.structs[v.Name]
		}
		if src == nil || src.ty != dst.ty {
			return nil, true, fmt.Errorf("assignment to structure %s: only MOVE from a structure of the same TYPES type is implemented", s.Target)
		}
		return charCall("_abap_mem_copy", dst.base, src.base, charLen(dst.ty.size)), true, nil
	}
	if l.tables[s.Target] != nil {
		return nil, true, fmt.Errorf("assignment to internal table %s is not implemented", s.Target)
	}
	if v, ok := s.Val.(*VarRef); ok {
		if err := l.aggregateOperandError(v.Name); err != nil {
			return nil, true, fmt.Errorf("assignment to %s: %w", s.Target, err)
		}
	}
	return nil, false, nil
}

func (l *lowerer) initialImage(st *abapStruct) hir.Expr {
	name := "_itab_initial_" + st.name
	if !st.emitted {
		st.emitted = true
		l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: name, Ty: mir2.NewArray(mir2.TyU8, st.size), Init: append([]byte(nil), st.initial...)})
	}
	return &hir.AddrOfExpr{Sym: name}
}

// lowerAggregateClear handles CLEAR of components, structures and tables.
func (l *lowerer) lowerAggregateClear(target string) (hir.Stmt, bool, error) {
	if objName, field, ok := splitSelector(target); ok {
		obj, c, err := l.component(objName, field)
		if err != nil {
			return nil, true, err
		}
		if c.abapTy == "c" {
			b := componentCharacter(obj, c)
			return charCall("_abap_char_clear", b.ptr, b.length), true, nil
		}
		return &hir.AssignStmt{Target: &hir.DerefExpr{Ptr: offsetPtr(obj.base, c.offset), Ty: mir2.TyI32}, Val: i32Lit(0)}, true, nil
	}
	if obj := l.structs[target]; obj != nil {
		return charCall("_abap_mem_copy", obj.base, l.initialImage(obj.ty), charLen(obj.ty.size)), true, nil
	}
	if t := l.tables[target]; t != nil {
		return &hir.AssignStmt{Target: &hir.VarRefExpr{Name: t.count, Ty: mir2.TyI32}, Val: i32Lit(0)}, true, nil
	}
	return nil, false, nil
}

func (l *lowerer) tableAndArea(stmt, table, area string) (*tableObject, *structObject, error) {
	t := l.tables[table]
	if t == nil {
		return nil, nil, fmt.Errorf("%s: %s is not a declared internal table", stmt, table)
	}
	if area == "" {
		return t, nil, nil
	}
	wa := l.structs[area]
	if wa == nil {
		return nil, nil, fmt.Errorf("%s: work area %s must be a structure variable", stmt, area)
	}
	if wa.ty != t.line {
		return nil, nil, fmt.Errorf("%s: work area %s has type %s, table %s has line type %s; conversions are not implemented", stmt, area, wa.ty.name, table, t.line.name)
	}
	return t, wa, nil
}

func (t *tableObject) countRef() hir.Expr { return &hir.VarRefExpr{Name: t.count, Ty: mir2.TyI32} }

func rowIndex(index hir.Expr) hir.Expr { return &hir.CastExpr{X: index, Ty: mir2.TyU16} }

func (l *lowerer) lowerAppend(s *AppendStmt) (hir.Stmt, error) {
	t, wa, err := l.tableAndArea("APPEND", s.Table, s.Source)
	if err != nil {
		return nil, err
	}
	var src hir.Expr
	if wa != nil {
		src = wa.base
	} else {
		src = l.initialImage(t.line)
	}
	full := &hir.BinExpr{Op: ">=", L: t.countRef(), R: i32Lit(int64(t.capacity)), Ty: mir2.TyBool}
	return &hir.Block{Body: []hir.Stmt{
		// The capacity check precedes every write: no line beyond the array.
		&hir.IfStmt{Cond: full, Then: &hir.Block{Body: []hir.Stmt{charCall("_abap_fault", &hir.IntLitExpr{Val: tableCapacityFault, Ty: mir2.TyU8})}}},
		&hir.AssignStmt{Target: t.countRef(), Val: &hir.BinExpr{Op: "+", L: t.countRef(), R: i32Lit(1), Ty: mir2.TyI32}},
		charCall("_abap_itab_store", t.base, rowIndex(t.countRef()), charLen(t.line.size), src),
		charCall("sy_set_tabix", t.countRef()),
	}}, nil
}

func (l *lowerer) lowerReadTable(s *ReadTableStmt) (hir.Stmt, error) {
	t, wa, err := l.tableAndArea("READ TABLE", s.Table, s.Into)
	if err != nil {
		return nil, err
	}
	if _, character, err := l.characterExpr(s.Index); err != nil {
		return nil, err
	} else if character {
		return nil, fmt.Errorf("READ TABLE INDEX: character-to-integer conversion is not implemented")
	}
	index, err := l.lowerExpr(s.Index)
	if err != nil {
		return nil, err
	}
	if index.ExprTy() != mir2.TyI32 {
		return nil, fmt.Errorf("READ TABLE INDEX: TYPE i operand required, got %s", index.ExprTy())
	}
	l.loopCounter++
	name := fmt.Sprintf("_abap_read_index_%d", l.loopCounter)
	idx := &hir.VarRefExpr{Name: name, Ty: mir2.TyI32}
	inRange := &hir.BinExpr{Op: "&&",
		L:  &hir.BinExpr{Op: ">=", L: idx, R: i32Lit(1), Ty: mir2.TyBool},
		R:  &hir.BinExpr{Op: "<=", L: idx, R: t.countRef(), Ty: mir2.TyBool},
		Ty: mir2.TyBool}
	return &hir.Block{Body: []hir.Stmt{
		&hir.VarDeclStmt{Name: name, Ty: mir2.TyI32, Init: index},
		&hir.IfStmt{Cond: inRange,
			Then: &hir.Block{Body: []hir.Stmt{
				charCall("_abap_itab_load", wa.base, t.base, rowIndex(idx), charLen(t.line.size)),
				charCall("sy_set_subrc", i32Lit(0)),
				charCall("sy_set_tabix", idx),
			}},
			Else: &hir.Block{Body: []hir.Stmt{
				charCall("sy_set_subrc", i32Lit(4)),
				charCall("sy_set_tabix", i32Lit(0)),
			}}},
	}}, nil
}

func (l *lowerer) lowerDescribeTable(s *DescribeTableStmt) (hir.Stmt, error) {
	t, _, err := l.tableAndArea("DESCRIBE TABLE", s.Table, "")
	if err != nil {
		return nil, err
	}
	target, err := l.integerTarget(s.Lines)
	if err != nil {
		return nil, fmt.Errorf("DESCRIBE TABLE ... LINES %w", err)
	}
	return &hir.AssignStmt{Target: target, Val: t.countRef()}, nil
}

func (l *lowerer) lowerRefresh(s *RefreshStmt) (hir.Stmt, error) {
	t, _, err := l.tableAndArea("REFRESH", s.Table, "")
	if err != nil {
		return nil, err
	}
	return &hir.AssignStmt{Target: t.countRef(), Val: i32Lit(0)}, nil
}

// LOOP AT itab INTO wa copies each line into wa (no aliasing). The end is the
// current line count at every step, so lines appended inside the loop are
// processed, as in ABAP. SY-TABIX is the loop's row number in the body and is
// restored on every exit (ENDLOOP, EXIT, RETURN). Afterwards SY-SUBRC is 0 if
// at least one line was processed, otherwise 4.
func (l *lowerer) lowerLoopAt(s *LoopStmt) (hir.Stmt, error) {
	t, wa, err := l.tableAndArea("LOOP AT", s.Table, s.Into)
	if err != nil {
		return nil, err
	}
	l.loopCounter++
	saved := fmt.Sprintf("_abap_tabix_saved_%d", l.loopCounter)
	counter := fmt.Sprintf("_abap_tabix_counter_%d", l.loopCounter)
	initial := fmt.Sprintf("_abap_loop_lines_%d", l.loopCounter)
	l.returnCleanups = append(l.returnCleanups, &hir.Block{Body: []hir.Stmt{charCall("sy_set_subrc", i32Lit(0)), indexSetTabix(loopValue(saved))}})
	l.loopDepth++
	block, err := l.lowerStmts(s.Body)
	l.loopDepth--
	l.returnCleanups = l.returnCleanups[:len(l.returnCleanups)-1]
	if err != nil {
		return nil, err
	}
	block.Body = append([]hir.Stmt{
		&hir.AssignStmt{Target: loopValue(counter), Val: &hir.BinExpr{Op: "+", L: loopValue(counter), R: i32Lit(1), Ty: mir2.TyI32}},
		indexSetTabix(loopValue(counter)),
		charCall("_abap_itab_load", wa.base, t.base, rowIndex(loopValue(counter)), charLen(t.line.size)),
	}, block.Body...)
	processed := &hir.BinExpr{Op: ">", L: loopValue(initial), R: i32Lit(0), Ty: mir2.TyBool}
	return &hir.Block{Body: []hir.Stmt{
		&hir.VarDeclStmt{Name: saved, Ty: mir2.TyI32, Init: &hir.CallExpr{Fn: "sy_get_tabix", Ty: mir2.TyI32}},
		&hir.VarDeclStmt{Name: counter, Ty: mir2.TyI32, Init: i32Lit(0)},
		&hir.VarDeclStmt{Name: initial, Ty: mir2.TyI32, Init: t.countRef()},
		&hir.WhileStmt{Cond: &hir.BinExpr{Op: "<", L: loopValue(counter), R: t.countRef(), Ty: mir2.TyBool}, Body: block},
		indexSetTabix(loopValue(saved)),
		&hir.IfStmt{Cond: processed,
			Then: &hir.Block{Body: []hir.Stmt{charCall("sy_set_subrc", i32Lit(0))}},
			Else: &hir.Block{Body: []hir.Stmt{charCall("sy_set_subrc", i32Lit(4))}}},
	}}, nil
}

func indexSetTabix(value hir.Expr) hir.Stmt { return charCall("sy_set_tabix", value) }

// itabRuntime copies whole line images; row indexes are 1-based and checked
// by the caller against the line count or capacity.
const itabRuntime = `
fun _abap_mem_copy(dst: ^u8, src: ^u8, size: u16) -> void {
 var i: u16 = 0
 while i < size { dst[i] = src[i]
 i = i + 1 }
}
fun _abap_itab_store(base: ^u8, index: u16, size: u16, src: ^u8) -> void {
 var offset: u16 = (index - 1) * size
 var i: u16 = 0
 while i < size { base[offset + i] = src[i]
 i = i + 1 }
}
fun _abap_itab_load(dst: ^u8, base: ^u8, index: u16, size: u16) -> void {
 var offset: u16 = (index - 1) * size
 var i: u16 = 0
 while i < size { dst[i] = base[offset + i]
 i = i + 1 }
}
`
