package abap

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Fixed character objects own their bytes. Generic TYPE c FORM parameters
// carry the actual object's address and length, never a C-string pointer cell.
type characterBinding struct {
	ptr, length, object hir.Expr
	dynamic             bool
}

func characterLength(n int) int {
	if n == 0 {
		return 1
	}
	return n
}
func charLen(n int) hir.Expr { return &hir.IntLitExpr{Val: int64(n), Ty: mir2.TyU16} }

func asciiCharacter(s string) error {
	for _, b := range []byte(s) {
		if b > 127 {
			return fmt.Errorf("character data: non-ASCII text is not admitted on the byte-character target")
		}
	}
	return nil
}

func (l *lowerer) characterExpr(e Expr_) (characterBinding, bool, error) {
	switch v := e.(type) {
	case *VarRef:
		b, ok := l.characters[v.Name]
		return b, ok, nil
	case *FieldAccess:
		obj, ok := v.Obj.(*VarRef)
		if !ok {
			return characterBinding{}, false, fmt.Errorf("unsupported component access")
		}
		o, c, err := l.component(obj.Name, v.Field)
		if err != nil {
			return characterBinding{}, false, err
		}
		if c.abapTy != "c" {
			return characterBinding{}, false, nil
		}
		return componentCharacter(o, c), true, nil
	case *StringLit:

		if err := asciiCharacter(v.Val); err != nil {
			return characterBinding{}, true, err
		}
		if len(v.Val) > 65535 {
			return characterBinding{}, true, fmt.Errorf("character literal exceeds the 16-bit length range")
		}
		return characterBinding{ptr: &hir.AddrOfExpr{Sym: l.internStr(v.Val)}, length: charLen(len(v.Val)), dynamic: v.IsString}, true, nil
	}
	return characterBinding{}, false, nil
}

func charCall(fn string, args ...hir.Expr) hir.Stmt {
	return &hir.ExprStmt{Expr: &hir.CallExpr{Fn: fn, Args: args, Ty: mir2.TyVoid}}
}
func (l *lowerer) characterAssign(dst characterBinding, e Expr_) (hir.Stmt, error) {
	src, ok, err := l.characterExpr(e)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("character assignment: numeric/dynamic conversion is not implemented")
	}
	if dst.dynamic {
		return charCall("_abap_string_assign", dst.object, src.ptr, textValueLength(src)), nil
	}
	return charCall("_abap_char_copy", dst.ptr, dst.length, src.ptr, src.length), nil
}

func (l *lowerer) characterGlobal(d *DataDecl) error {
	n := characterLength(d.Length)
	init := make([]byte, n)
	for i := range init {
		init[i] = 32
	}
	if d.Value != nil {
		s, ok := (*d.Value).(*StringLit)
		if !ok {
			return fmt.Errorf("DATA %s: character initialization requires a character literal", d.Name)
		}
		if err := asciiCharacter(s.Val); err != nil {
			return err
		}
		copy(init, []byte(s.Val))
	}
	l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: d.Name, Ty: mir2.NewArray(mir2.TyU8, n), Init: init})
	l.characters[d.Name] = characterBinding{ptr: &hir.AddrOfExpr{Sym: d.Name}, length: charLen(n)}
	return nil
}

func (l *lowerer) characterLocal(d *DataDecl) (hir.Stmt, error) {
	n := characterLength(d.Length)
	ty := mir2.NewArray(mir2.TyU8, n)
	b := characterBinding{ptr: &hir.AddrOfExpr{X: &hir.VarRefExpr{Name: d.Name, Ty: ty}}, length: charLen(n)}
	l.characters[d.Name] = b
	l.varTypes[d.Name] = mir2.TyPtr
	ss := []hir.Stmt{&hir.VarDeclStmt{Name: d.Name, Ty: ty}}
	if d.Value == nil {
		ss = append(ss, charCall("_abap_char_clear", b.ptr, b.length))
	} else {
		s, err := l.characterAssign(b, *d.Value)
		if err != nil {
			return nil, fmt.Errorf("DATA %s: %w", d.Name, err)
		}
		ss = append(ss, s)
	}
	return &hir.Block{Body: ss}, nil
}

const characterRuntime = `
fun _abap_char_clear(dst: ^u8, length: u16) -> void {
 var i: u16 = 0
 while i < length { dst[i] = 32
 i = i + 1 }
}
fun _abap_char_copy(dst: ^u8, dlen: u16, src: ^u8, slen: u16) -> void {
 var i: u16 = 0
 while i < dlen {
  var ch: u8 = 32
  if i < slen { ch = src[i] }
  dst[i] = ch
  i = i + 1
 }
}
fun _abap_char_compare(a: ^u8, alen: u16, b: ^u8, blen: u16) -> i16 {
 var length: u16 = alen
 if blen > length { length = blen }
 var i: u16 = 0
 while i < length {
  var ca: u8 = 32
  var cb: u8 = 32
  if i < alen { ca = a[i] }
  if i < blen { cb = b[i] }
  if ca < cb { return -1 }
  if ca > cb { return 1 }
  i = i + 1
 }
 return 0
}
fun _abap_char_write(src: ^u8, length: u16) -> void {
 var i: u16 = 0
 while i < length { _abap_num_putchar(src[i])
 i = i + 1 }
}
fun _abap_char_cstrlen(src: ^u8) -> u16 {
 var n: u16 = 0
 while src[n] != 0 { n = n + 1 }
 return n
}
`
