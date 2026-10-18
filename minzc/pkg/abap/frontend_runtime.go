package abap

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// All catalog, line conversion, row ownership and picker semantics run as
// compiled code. The six leaf BDOS calls below are the per-target boundary.
const frontendRuntime = `
@extern fun _abap_fault(code: u8) -> void
@extern fun sy_set_subrc(value: i32) -> void
@extern fun _abap_char_copy(dst: ^u8, dlen: u16, src: ^u8, slen: u16) -> void
@extern fun _abap_char_trim_len(src: ^u8, length: u16) -> u16
@extern fun _abap_char_cstrlen(src: ^u8) -> u16
@extern fun _abap_char_write(src: ^u8, length: u16) -> void
@extern fun _abap_string_init(dst: ^u8, capacity: u16) -> void
@extern fun _abap_string_assign(dst: ^u8, src: ^u8, length: u16) -> void
@extern fun _abap_num_putchar(ch: u8) -> void
@extern fun _abap_write_i32_value(value: i32) -> void
@extern fun _abap_fs_fcb() -> ^u8
@extern fun _abap_fs_dma() -> ^u8
@extern fun _abap_fs_work() -> ^u8
@extern fun _abap_fs_names() -> ^u8
@extern fun _abap_fs_bdos_open() -> u8
@extern fun _abap_fs_bdos_close() -> u8
@extern fun _abap_fs_bdos_read() -> u8
@extern fun _abap_fs_bdos_first() -> u8
@extern fun _abap_fs_bdos_next() -> u8
@extern fun _abap_fs_bdos_key() -> u8

global _abap_fs_active: u8 = 0
global _abap_fs_pos: u16 = 128
global _abap_fs_eof: u8 = 0
global _abap_fs_skip_lf: u8 = 0
global _abap_fs_line_length: u16 = 0
global _abap_fs_key_pending: u8 = 0

fun _abap_fs_clear_fcb() -> void {
 var f: ^u8 = _abap_fs_fcb()
 var i: u16 = 0
 while i < 36 { f[i] = 0
 i = i + 1 }
 i = 1
 while i < 12 { f[i] = 32
 i = i + 1 }
}
fun _abap_fs_valid_char(ch: u8) -> bool {
 if ch < 33 || ch > 126 { return false }
 if ch == 60 || ch == 62 || ch == 46 || ch == 44 || ch == 59 || ch == 58 || ch == 61 { return false }
 if ch == 42 || ch == 63 || ch == 91 || ch == 93 || ch == 47 || ch == 92 { return false }
 return true
}
fun _abap_fs_make_name(name: ^u8, length: u16) -> bool {
 _abap_fs_clear_fcb()
 var f: ^u8 = _abap_fs_fcb()
 var i: u16 = 0
 if length >= 2 {
  if name[1] == 58 {
   if name[0] != 65 && name[0] != 97 { return false }
   f[0] = 1
   i = 2
  }
 }
 var k: u16 = 1
 var dot: bool = false
 while i < length {
  var ch: u8 = name[i]
  if ch == 46 {
   if dot || k == 1 { return false }
   dot = true
   k = 9
  } else {
   if !_abap_fs_valid_char(ch) { return false }
   if k > 11 || (!dot && k > 8) { return false }
   if ch >= 97 && ch <= 122 { ch = ch - 32 }
   f[k] = ch
   k = k + 1
  }
  i = i + 1
 }
 if f[1] == 32 || (dot && k == 9) { return false }
 return true
}
fun _abap_fs_make_filter(pattern: ^u8, length: u16) -> bool {
 _abap_fs_clear_fcb()
 var f: ^u8 = _abap_fs_fcb()
 var i: u16 = 0
 var k: u16 = 1
 var dot: bool = false
 if length == 0 { while k < 12 { f[k] = 63
 k = k + 1 }
 return true }
 while i < length {
  var ch: u8 = pattern[i]
  if ch == 46 {
   if dot || k == 1 { return false }
   dot = true
   k = 9
  } else if ch == 42 {
   var stop: u16 = 9
   var start: u16 = 1
   if dot { stop = 12
   start = 9 }
   if k != start { return false }
   while k < stop { f[k] = 63
   k = k + 1 }
   if i + 1 < length { if pattern[i + 1] != 46 || dot { return false } }
   if i + 1 == length && !dot { while k < 12 { f[k] = 63
   k = k + 1 } }
  } else {
   if ch != 63 && !_abap_fs_valid_char(ch) { return false }
   if k > 11 || (!dot && k > 8) { return false }
   if ch >= 97 && ch <= 122 { ch = ch - 32 }
   f[k] = ch
   k = k + 1
  }
  i = i + 1
 }
 if f[1] == 32 || (dot && k == 9) { return false }
 return true
}
fun _abap_fs_directory_ok(directory: ^u8, length: u16) -> bool {
 if length == 0 { return true }
 if length == 1 { return directory[0] == 46 }
 if length == 2 {
  if directory[1] != 58 { return false }
  if directory[0] == 65 || directory[0] == 97 { return true }
 }
 return false
}
fun _abap_fs_catalog(filter: ^u8, flen: u16, directory: ^u8, dlen: u16) -> i32 {
 if !_abap_fs_directory_ok(directory, dlen) { return -1 }
 if !_abap_fs_make_filter(filter, flen) { return -1 }
 var count: u16 = 0
 var names: ^u8 = _abap_fs_names()
 var rc: u8 = _abap_fs_bdos_first()
 while rc != 255 {
  if rc > 3 { return -1 }
  if count == 128 { _abap_fault(7) }
  var dma: ^u8 = _abap_fs_dma()
  var entry: u16 = u16(rc) * 32
  var base: u16 = count * 13
  var n: u16 = 0
  var i: u16 = 1
  while i < 9 {
   var ch: u8 = dma[entry + i] & 127
   if ch != 32 { names[base + n] = ch
   n = n + 1 }
   i = i + 1
  }
  if (dma[entry + 9] & 127) != 32 {
   names[base + n] = 46
   n = n + 1
   i = 9
   while i < 12 {
    var ch: u8 = dma[entry + i] & 127
    if ch != 32 { names[base + n] = ch
    n = n + 1 }
    i = i + 1
   }
  }
  names[base + n] = 0
  count = count + 1
  rc = _abap_fs_bdos_next()
 }
 return i32(count)
}
fun _abap_fs_store_text(dst: ^u8, width: u16, kind: u8, src: ^u8, length: u16) -> void {
 if kind != 0 { _abap_string_init(dst, width - 4)
 _abap_string_assign(dst, src, length) } else { _abap_char_copy(dst, width, src, length) }
}
fun _abap_fs_directory(table: ^u8, count: ^i32, capacity: u16, width: u16, kind: u8, filter: ^u8, flen: u16, directory: ^u8, dlen: u16) -> i32 {
 count^ = i32(0)
 var found: i32 = _abap_fs_catalog(filter, flen, directory, dlen)
 if found < 0 { sy_set_subrc(8)
 return 0 }
 if found > i32(capacity) { _abap_fault(7) }
 var names: ^u8 = _abap_fs_names()
 var i: u16 = 0
 while i < u16(found) {
  var src: ^u8 = names + i * 13
  _abap_fs_store_text(table + i * width, width, kind, src, _abap_char_cstrlen(src))
  i = i + 1
 }
 count^ = found
 sy_set_subrc(0)
 return found
}
fun _abap_fs_key() -> u8 {
 if _abap_fs_key_pending != 0 { var ch: u8 = _abap_fs_key_pending
 _abap_fs_key_pending = 0
 return ch }
 while true { var ch: u8 = _abap_fs_bdos_key()
 if ch != 0 { return ch } }
 return 0
}
fun _abap_fs_pick(found: u16, title: ^u8, title_length: u16) -> u16 {
 if title_length > 0 { _abap_char_write(title, title_length) }
 var selected: u16 = 0
 var names: ^u8 = _abap_fs_names()
 while true {
  _abap_num_putchar(13)
  _abap_num_putchar(10)
  var i: u16 = (selected / 8) * 8
  var end: u16 = i + 8
  if end > found { end = found }
  while i < end {
   if i == selected { _abap_num_putchar(62) } else { _abap_num_putchar(32) }
   _abap_write_i32_value(i32(i + 1))
   _abap_num_putchar(32)
   var name: ^u8 = names + i * 13
   _abap_char_write(name, _abap_char_cstrlen(name))
   _abap_num_putchar(13)
   _abap_num_putchar(10)
   i = i + 1
  }
  var ch: u8 = _abap_fs_key()
  if ch == 13 || ch == 10 { return selected + 1 }
  if ch >= 49 && ch <= 57 { var digit: u16 = u16(ch - 49)
  if digit < found { selected = digit } }
  if ch == 27 {
   var next: u8 = _abap_fs_bdos_key()
   if next == 0 { return 0 }
   if next != 91 && next != 79 { _abap_fs_key_pending = next
   return 0 }
   var arrow: u8 = _abap_fs_key()
   if arrow == 66 { if selected + 1 < found { selected = selected + 1 } }
   if arrow == 65 { if selected > 0 { selected = selected - 1 } }
  }
 }
 return 0
}
fun _abap_fs_dialog(table: ^u8, count: ^i32, capacity: u16, width: u16, kind: u8, filter: ^u8, flen: u16, directory: ^u8, dlen: u16, title: ^u8, title_length: u16) -> i32 {
 count^ = i32(0)
 var found: i32 = _abap_fs_catalog(filter, flen, directory, dlen)
 if found <= 0 { sy_set_subrc(8)
 return -1 }
 var choice: u16 = _abap_fs_pick(u16(found), title, title_length)
 if choice == 0 { sy_set_subrc(0)
 return 0 }
 if capacity == 0 { _abap_fault(7) }
 var name: ^u8 = _abap_fs_names() + (choice - 1) * 13
 _abap_fs_store_text(table, width, kind, name, _abap_char_cstrlen(name))
 count^ = i32(1)
 sy_set_subrc(0)
 return 1
}
fun _abap_fs_read_c_f4(dst: ^u8, length: u16) -> u8 {
 var work: ^u8 = _abap_fs_work()
 var n: u16 = 0
 while true {
  var ch: u8 = _abap_fs_key()
  if ch == 13 || ch == 10 { break }
  if ch == 27 {
   var a: u8 = _abap_fs_key()
   if a == 79 { if _abap_fs_key() == 83 { return 1 } }
   else if a == 91 {
    if _abap_fs_key() == 49 { if _abap_fs_key() == 52 { if _abap_fs_key() == 126 { return 1 } } }
   } else { _abap_fs_key_pending = a }
   continue
  }
  if ch == 8 || ch == 127 { if n > 0 { n = n - 1 }
  continue }
  if ch < 32 || ch > 127 { _abap_fault(4) }
  if n < length { work[n] = ch
  n = n + 1
  _abap_num_putchar(ch) }
 }
 _abap_num_putchar(13)
 _abap_num_putchar(10)
 if n != 0 { _abap_char_copy(dst, length, work, n) }
 return 0
}
fun _abap_fs_close() -> u8 {
 _abap_fs_active = 0
 return _abap_fs_bdos_close()
}
fun _abap_fs_failure(code: i32) -> void {
 if code == 0 { _abap_fault(8) }
 sy_set_subrc(code)
}
fun _abap_fs_byte() -> u16 {
 if _abap_fs_eof != 0 { return 256 }
 if _abap_fs_pos == 128 {
  var rc: u8 = _abap_fs_bdos_read()
  if rc == 1 { _abap_fs_eof = 1
  return 256 }
  if rc != 0 { return 257 }
  _abap_fs_pos = 0
 }
 var dma: ^u8 = _abap_fs_dma()
 var ch: u8 = dma[_abap_fs_pos]
 _abap_fs_pos = _abap_fs_pos + 1
 if ch == 26 { _abap_fs_eof = 1
 return 256 }
 return u16(ch)
}
fun _abap_fs_line(limit: u16, kind: u8) -> u8 {
 var work: ^u8 = _abap_fs_work()
 var n: u16 = 0
 var any: bool = false
 while true {
  var value: u16 = _abap_fs_byte()
  if value == 257 { return 2 }
  if _abap_fs_skip_lf != 0 { _abap_fs_skip_lf = 0
  if value == 10 { continue } }
  if value == 256 { break }
  any = true
  if value == 13 { _abap_fs_skip_lf = 1
  break }
  if value == 10 { break }
  if value > 127 { _abap_fs_close()
  _abap_fault(4) }
  if n < limit { work[n] = u8(value)
  n = n + 1 } else if kind != 0 { _abap_fs_close()
  _abap_fault(3) }
 }
 _abap_fs_line_length = n
 if !any { return 1 }
 return 0
}
fun _abap_fs_upload(filename: ^u8, flen: u16, table: ^u8, count: ^i32, capacity: u16, width: u16, kind: u8, open_error: i32, other_error: i32) -> void {
 if !_abap_fs_make_name(filename, flen) { _abap_fs_failure(open_error)
 return }
 if _abap_fs_bdos_open() == 255 { _abap_fs_failure(open_error)
 return }
 _abap_fs_active = 1
 _abap_fs_pos = 128
 _abap_fs_eof = 0
 _abap_fs_skip_lf = 0
 count^ = i32(0)
 var limit: u16 = width
 if kind != 0 { limit = width - 4 }
 while true {
  var status: u8 = _abap_fs_line(limit, kind)
  if status == 1 { break }
  if status != 0 { _abap_fs_close()
  _abap_fs_failure(other_error)
  return }
  if count^ >= i32(capacity) { _abap_fs_close()
  _abap_fault(7) }
  _abap_fs_store_text(table + u16(count^) * width, width, kind, _abap_fs_work(), _abap_fs_line_length)
  count^ = count^ + 1
 }
 if _abap_fs_close() == 255 { _abap_fs_failure(other_error)
 return }
 sy_set_subrc(0)
}
`

func emitFrontendRuntime(hm *hir.Module) error {
	needed := false
	for _, g := range hm.Globals {
		if g.Name == "_abap_fs_work_buffer" {
			needed = true
		}
	}
	if !needed {
		return nil
	}
	if hm.Target != hir.TargetCPM {
		return fmt.Errorf("frontend filesystem adapter not implemented for target %d", hm.Target)
	}
	m, err := nanz.Parse(frontendRuntime, "abap-frontend-runtime.nanz")
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
	for _, b := range []struct {
		name string
		size int
	}{{"fcb", 36}, {"dma", 128}, {"names", 13 * 128}, {"work", 0}} {
		sym := "_abap_fs_" + b.name + "_buffer"
		if b.size != 0 {
			hm.Globals = append(hm.Globals, mir2.Global{Name: sym, Ty: mir2.NewArray(mir2.TyU8, b.size)})
		}
		hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_fs_" + b.name, Internal: true, RetTy: mir2.TyPtr, Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.AddrOfExpr{Sym: sym}}}}})
	}
	for _, op := range []struct{ name, code string }{
		{"open", "LD DE, _abap_fs_fcb_buffer / LD C, 15 / CALL 5"},
		{"close", "LD DE, _abap_fs_fcb_buffer / LD C, 16 / CALL 5"},
		{"read", "LD DE, _abap_fs_dma_buffer / LD C, 26 / CALL 5 / LD DE, _abap_fs_fcb_buffer / LD C, 20 / CALL 5 / PUSH AF / LD DE, 128 / LD C, 26 / CALL 5 / POP AF"},
		{"first", "LD DE, _abap_fs_dma_buffer / LD C, 26 / CALL 5 / LD DE, _abap_fs_fcb_buffer / LD C, 17 / CALL 5 / PUSH AF / LD DE, 128 / LD C, 26 / CALL 5 / POP AF"},
		{"next", "LD DE, _abap_fs_dma_buffer / LD C, 26 / CALL 5 / LD DE, _abap_fs_fcb_buffer / LD C, 18 / CALL 5 / PUSH AF / LD DE, 128 / LD C, 26 / CALL 5 / POP AF"},
		{"key", "LD E, 255 / LD C, 6 / CALL 5"},
	} {
		hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_fs_bdos_" + op.name, Internal: true, RetTy: mir2.TyU8, Body: &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Target: "z80", Code: op.code, RetReg: "A", ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"}}}}})
	}
	return nil
}
