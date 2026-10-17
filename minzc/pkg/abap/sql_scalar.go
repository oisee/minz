package abap

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// SQLite transports scalar values as bounded NUL-terminated text. The old
// integer port only carries 16 bits and must not implement ABAP TYPE i.
const sqlScalarRuntime = `
@extern fun sqlite_column_text(stmt: u16, col: u8) -> ^u8
fun _abap_sql_text_length(data: ^u8) -> u16 {
    if data == @ptr(u8, 0) { _abap_fault(6) }
    var n: u16 = 0
    while data[n] != 0 {
        if n == 255 { _abap_fault(6) }
        if data[n] > 127 { _abap_fault(6) }
        n = n + 1
    }
    return n
}
fun _abap_sql_i32(stmt: u16, col: u8) -> i32 {
    var data: ^u8 = sqlite_column_text(stmt, col)
    var n: u16 = _abap_sql_text_length(data)
    return _abap_parse_i32(data, u8(n))
}
fun _abap_sql_c(stmt: u16, col: u8, dst: ^u8, length: u16) -> void {
    var data: ^u8 = sqlite_column_text(stmt, col)
    var n: u16 = _abap_sql_text_length(data)
    _abap_char_copy(dst, length, data, n)
}
fun _abap_sql_string(stmt: u16, col: u8, dst: ^u8) -> void {
    var data: ^u8 = sqlite_column_text(stmt, col)
    var n: u16 = _abap_sql_text_length(data)
    _abap_string_assign(dst, data, n)
}
`

func (l *lowerer) lowerSQLColumns(s *SelectStmt, stmt string) ([]hir.Stmt, error) {
	if len(s.Into) == 0 {
		return nil, fmt.Errorf("SELECT: scalar INTO target is required")
	}
	if len(s.Into) > 256 {
		return nil, fmt.Errorf("SELECT: column index exceeds byte protocol")
	}
	if len(s.Fields) != len(s.Into) || (len(s.Fields) == 1 && s.Fields[0] == "*") {
		return nil, fmt.Errorf("SELECT: explicit columns %v must match scalar INTO targets %v", s.Fields, s.Into)
	}
	var body []hir.Stmt
	for i, name := range s.Into {
		ty, ok := l.varTypes[name]
		if !ok {
			return nil, fmt.Errorf("SELECT INTO: undeclared ABAP object %q", name)
		}
		args := []hir.Expr{&hir.VarRefExpr{Name: stmt, Ty: mir2.TyU16}, &hir.IntLitExpr{Val: int64(i), Ty: mir2.TyU8}}
		if b, ok := l.characters[name]; ok {
			fn := "_abap_sql_c"
			if b.dynamic {
				fn = "_abap_sql_string"
				args = append(args, b.object)
			} else {
				args = append(args, b.ptr, b.length)
			}
			body = append(body, charCall(fn, args...))
		} else if ty == mir2.TyI32 {
			body = append(body, &hir.AssignStmt{Target: l.variableTarget(name, ty), Val: &hir.CallExpr{Fn: "_abap_sql_i32", Args: args, Ty: mir2.TyI32}})
		} else {
			return nil, fmt.Errorf("SELECT INTO %s: exact conversion for %s is not implemented", name, ty)
		}
	}
	return body, nil
}
