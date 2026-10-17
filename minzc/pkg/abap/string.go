package abap

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// For scalar copying, no admitted operation grows a string beyond a literal
// or fixed character source. Use a conservative module-wide bound, preserving
// dynamic lengths without introducing a heap/GC or silent truncation.
func inferStringCapacity(p *Program) (int, error) {
	capacity := 1
	hasString := false
	var visit func(reflect.Value)
	visit = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			visit(v.Elem())
			return
		}
		if v.CanInterface() {
			switch x := v.Interface().(type) {
			case StringLit:
				if len(x.Val) > capacity {
					capacity = len(x.Val)
				}
			case FormParam:
				if x.AbapTy == "string" {
					hasString = true
				}
			case DataDecl:
				if x.AbapTy == "string" {
					hasString = true
				}
				if x.AbapTy == "c" && characterLength(x.Length) > capacity {
					capacity = characterLength(x.Length)
				}
			case ParamDecl:
				if x.AbapTy == "c" && characterLength(x.Length) > capacity {
					capacity = characterLength(x.Length)
				}
			}
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				visit(v.Field(i))
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				visit(v.Index(i))
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				visit(iter.Value())
			}
		}
	}
	visit(reflect.ValueOf(p))
	if !hasString {
		return 1, nil
	}
	if capacity > 65531 {
		return 0, fmt.Errorf("bounded string capacity %d cannot fit a 16-bit object with length header", capacity)
	}
	return capacity, nil
}

func stringBinding(obj hir.Expr) characterBinding {
	return characterBinding{object: obj, dynamic: true,
		ptr:    &hir.BinExpr{Op: "+", L: obj, R: charLen(4), Ty: mir2.TyPtr},
		length: &hir.LoadExpr{Ptr: obj, Ty: mir2.TyU16}}
}
func textValueLength(b characterBinding) hir.Expr {
	if b.dynamic {
		return b.length
	}
	return &hir.CallExpr{Fn: "_abap_char_trim_len", Args: []hir.Expr{b.ptr, b.length}, Ty: mir2.TyU16}
}
func initialString(e Expr_) (string, error) {
	lit, ok := e.(*StringLit)
	if !ok {
		return "", fmt.Errorf("string initialization requires a text literal")
	}
	if err := asciiCharacter(lit.Val); err != nil {
		return "", err
	}
	if lit.IsString {
		return lit.Val, nil
	}
	return strings.TrimRight(lit.Val, " "), nil
}
func (l *lowerer) stringGlobal(d *DataDecl) error {
	init := make([]byte, l.stringCapacity+4)
	init[2], init[3] = byte(l.stringCapacity), byte(l.stringCapacity>>8)
	if d.Value != nil {
		text, err := initialString(*d.Value)
		if err != nil {
			return fmt.Errorf("DATA %s: %w", d.Name, err)
		}
		if len(text) > l.stringCapacity {
			return fmt.Errorf("DATA %s: string initializer exceeds inferred capacity", d.Name)
		}
		init[0], init[1] = byte(len(text)), byte(len(text)>>8)
		copy(init[4:], text)
	}
	l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: d.Name, Ty: mir2.NewArray(mir2.TyU8, len(init)), Init: init})
	l.characters[d.Name] = stringBinding(&hir.AddrOfExpr{Sym: d.Name})
	return nil
}
func (l *lowerer) stringLocal(d *DataDecl) (hir.Stmt, error) {
	ty := mir2.NewArray(mir2.TyU8, l.stringCapacity+4)
	b := stringBinding(&hir.AddrOfExpr{X: &hir.VarRefExpr{Name: d.Name, Ty: ty}})
	l.characters[d.Name] = b
	l.varTypes[d.Name] = mir2.TyPtr
	ss := []hir.Stmt{&hir.VarDeclStmt{Name: d.Name, Ty: ty}, charCall("_abap_string_init", b.object, charLen(l.stringCapacity))}
	if d.Value == nil {
		ss = append(ss, charCall("_abap_string_clear", b.object))
	} else {
		s, err := l.characterAssign(b, *d.Value)
		if err != nil {
			return nil, err
		}
		ss = append(ss, s)
	}
	return &hir.Block{Body: ss}, nil
}

const stringRuntime = `
fun _abap_char_trim_len(src: ^u8, length: u16) -> u16 {
 var n: u16 = length
 while n > 0 {
  if src[n - 1] != 32 { return n }
  n = n - 1
 }
 return 0
}
fun _abap_string_clear(dst: ^u8) -> void {
 dst[0] = 0
 dst[1] = 0
}
fun _abap_string_init(dst: ^u8, capacity: u16) -> void {
 dst[0] = 0
 dst[1] = 0
 dst[2] = u8(capacity)
 dst[3] = u8(capacity >> 8)
}
fun _abap_string_assign(dst: ^u8, src: ^u8, length: u16) -> void {
 var capacity: u16 = u16(dst[2]) + u16(dst[3]) * 256
 if length > capacity { _abap_fault(3) }
 var i: u16 = 0
 while i < length {
  dst[i + 4] = src[i]
  i = i + 1
 }
 dst[0] = u8(length)
 dst[1] = u8(length >> 8)
}
fun _abap_string_compare(a: ^u8, alen: u16, b: ^u8, blen: u16) -> i16 {
 var length: u16 = alen
 if blen < length { length = blen }
 var i: u16 = 0
 while i < length {
  if a[i] < b[i] { return -1 }
  if a[i] > b[i] { return 1 }
  i = i + 1
 }
 if alen < blen { return -1 }
 if alen > blen { return 1 }
 return 0
}
`
