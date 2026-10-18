package abap

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Shared typed grid logic. All terminal state changes are platform calls: this
// code never assumes ANSI output is understood by an Agon VDP.
const salvRuntime = `
@extern fun _abap_num_putchar(ch: u8) -> void
@extern fun _abap_fault(code: u8) -> void
@extern fun _abap_salv_key_poll() -> u8
@extern fun _abap_salv_screen_begin() -> void
@extern fun _abap_salv_goto(x: u8, y: u8) -> void
@extern fun _abap_salv_screen_end() -> void
@extern fun _abap_salv_number_data() -> ^u8
@extern fun _abap_salv_title_text() -> ^u8
@extern fun _abap_salv_first_text() -> ^u8
@extern fun _abap_salv_help_text() -> ^u8

fun _abap_salv_puts(text: ^u8) -> void {
 var pos: u16 = 0
 while text[pos] != 0 { _abap_num_putchar(text[pos])
 pos = pos + 1 }
}
fun _abap_salv_write_number(value: i32, width: u8) -> void {
 var buffer: ^u8 = _abap_salv_number_data()
 var magnitude: u32 = u32(value)
 if value < 0 { magnitude = u32(0) - magnitude }
 var length: u8 = 0
 while magnitude > 0 { buffer[length] = u8(magnitude % 10) + 48
 magnitude = magnitude / 10
 length = length + 1 }
 if length == 0 { buffer[0] = 48
 length = 1 }
 if value < 0 { buffer[length] = 45
 length = length + 1 }
 var padding: u8 = length
 while padding < width { _abap_num_putchar(32)
 padding = padding + 1 }
 while length > 0 { length = length - 1
 _abap_num_putchar(buffer[length]) }
}
fun _abap_salv_word(src: ^u8) -> u16 { return u16(src[0]) + u16(src[1]) * 256 }
fun _abap_salv_i32(src: ^u8) -> i32 {
 return i32(u32(src[0]) | (u32(src[1]) << 8) | (u32(src[2]) << 16) | (u32(src[3]) << 24))
}
fun _abap_salv_text(src: ^u8, length: u16, width: u8) -> void {
 var pos: u16 = 0
 while pos < u16(width) {
  var ch: u8 = 32
  if pos < length { ch = src[pos]
  if ch < 32 || ch > 126 { ch = 46 } }
  _abap_num_putchar(ch)
  pos = pos + 1
 }
}
fun _abap_salv_blank_row(y: u8) -> void {
 _abap_salv_goto(0, y)
 var width: u8 = 80
 if y == 23 { width = 79 }
 var pos: u8 = 0
 while pos < width { _abap_num_putchar(32)
 pos = pos + 1 }
 _abap_salv_goto(0, y)
}
fun _abap_salv_border(columns: ^u8, count: u8) -> void {
 _abap_num_putchar(43)
 var col: u8 = 0
 while col < count {
  var descriptor: ^u8 = columns + u16(col) * 26
  var pos: u8 = 0
  while pos < descriptor[4] { _abap_num_putchar(45)
  pos = pos + 1 }
  _abap_num_putchar(43)
  col = col + 1
 }
}
fun _abap_salv_headings(columns: ^u8, count: u8) -> void {
 _abap_num_putchar(124)
 var col: u8 = 0
 while col < count {
  var descriptor: ^u8 = columns + u16(col) * 26
  _abap_salv_text(descriptor + 6, 20, descriptor[4])
  _abap_num_putchar(124)
  col = col + 1
 }
}
fun _abap_salv_row(row: ^u8, columns: ^u8, count: u8) -> void {
 _abap_num_putchar(124)
 var col: u8 = 0
 while col < count {
  var descriptor: ^u8 = columns + u16(col) * 26
  var source: ^u8 = row + _abap_salv_word(descriptor)
  if descriptor[5] == 0 { _abap_salv_text(source, _abap_salv_word(descriptor + 2), descriptor[4]) }
  else { _abap_salv_write_number(_abap_salv_i32(source), descriptor[4]) }
  _abap_num_putchar(124)
  col = col + 1
 }
}
fun _abap_salv_render(base: ^u8, count: i32, row_size: u16, columns: ^u8, ncols: u8, top: i32) -> void {
 _abap_salv_blank_row(0)
 _abap_salv_puts(_abap_salv_title_text())
 _abap_salv_write_number(count, 0)
 _abap_salv_puts(_abap_salv_first_text())
 var first: i32 = 0
 if count > 0 { first = top + 1 }
 _abap_salv_write_number(first, 0)
 _abap_salv_goto(0, 1)
 _abap_salv_border(columns, ncols)
 _abap_salv_goto(0, 2)
 _abap_salv_headings(columns, ncols)
 _abap_salv_goto(0, 3)
 _abap_salv_border(columns, ncols)
 var slot: u8 = 0
 while slot < 18 {
  _abap_salv_blank_row(slot + 4)
  var index: i32 = top + i32(slot)
  if index < count { _abap_salv_row(base + u16(index) * row_size, columns, ncols) }
  slot = slot + 1
 }
 _abap_salv_goto(0, 22)
 _abap_salv_border(columns, ncols)
 _abap_salv_blank_row(23)
 _abap_salv_puts(_abap_salv_help_text())
}
fun _abap_salv_escape_next() -> u8 {
 var remaining: u16 = 4096
 while remaining > 0 { var ch: u8 = _abap_salv_key_poll()
 if ch != 0 { return ch }
 remaining = remaining - 1 }
 return 0
}
// Actions:0ignore,1up,2down,3pageup,4pagedown,5home,6end,7back.
fun _abap_salv_key() -> u8 {
 var ch: u8 = 0
 while ch == 0 { ch = _abap_salv_key_poll() }
 if ch == 113 || ch == 81 || ch == 8 { return 7 }
 if ch != 27 { return 0 }
 var prefix: u8 = _abap_salv_escape_next()
 if prefix == 0 { return 7 }
 if prefix != 91 && prefix != 79 { return 7 }
 var code: u8 = _abap_salv_escape_next()
 if code == 0 { return 7 }
 if prefix == 79 && code == 82 { return 7 }
 if code == 65 { return 1 }
 if code == 66 { return 2 }
 if code == 72 { return 5 }
 if code == 70 { return 6 }
 if prefix == 79 { return 0 }
 if code == 49 {
  var tail: u8 = _abap_salv_escape_next()
  if tail == 126 { return 5 }
  if tail == 51 { if _abap_salv_escape_next() == 126 { return 7 } }
  return 0
 }
 if code == 52 || code == 53 || code == 54 {
  if _abap_salv_escape_next() != 126 { return 0 }
  if code == 52 { return 6 }
  if code == 53 { return 3 }
  return 4
 }
 return 0
}
fun _abap_salv_display(base: ^u8, count: ^i32, row_size: u16, columns: ^u8, ncols: u8, grid_width: u8) -> void {
 if grid_width > 80 || ncols == 0 || ncols > 16 { _abap_fault(8) }
 var top: i32 = 0
 _abap_salv_screen_begin()
 var total: i32 = count^
 if total < 0 { _abap_fault(8) }
 _abap_salv_render(base, total, row_size, columns, ncols, top)
 while true {
  var action: u8 = _abap_salv_key()
  if action == 7 { break }
  if action != 0 {
   total = count^
   if total < 0 { _abap_fault(8) }
   var maximum: i32 = 0
   if total > 18 { maximum = total - 18 }
   if action == 1 { top = top - 1 }
   if action == 2 { top = top + 1 }
   if action == 3 { top = top - 18 }
   if action == 4 { top = top + 18 }
   if action == 5 { top = 0 }
   if action == 6 { top = maximum }
   if top < 0 { top = 0 }
   if top > maximum { top = maximum }
   _abap_salv_render(base, total, row_size, columns, ncols, top)
  }
 }
 _abap_salv_screen_end()
}
`

// These are the only terminal-state semantics for the CP/M adapter. Agon must
// supply VDP primitives instead; shared grid code above contains no ANSI output.
const salvCPMPlatform = `
@extern fun _abap_num_putchar(ch: u8) -> void
@extern fun _abap_salv_puts(text: ^u8) -> void
@extern fun _abap_salv_write_number(value: i32, width: u8) -> void
@extern fun _abap_salv_cpm_begin_text() -> ^u8
@extern fun _abap_salv_cpm_end_text() -> ^u8
fun _abap_salv_screen_begin() -> void { _abap_salv_puts(_abap_salv_cpm_begin_text()) }
fun _abap_salv_goto(x: u8, y: u8) -> void {
 _abap_num_putchar(27)
 _abap_num_putchar(91)
 _abap_salv_write_number(i32(y) + 1, 0)
 _abap_num_putchar(59)
 _abap_salv_write_number(i32(x) + 1, 0)
 _abap_num_putchar(72)
}
fun _abap_salv_screen_end() -> void { _abap_salv_puts(_abap_salv_cpm_end_text()) }
`

func emitSalvRuntime(hm *hir.Module) error {
	needed := false
	for _, g := range hm.Globals {
		if g.Name == "_abap_salv_number_buffer" {
			needed = true
		}
	}
	if !needed {
		return nil
	}
	if hm.Target != hir.TargetCPM && hm.Target != hir.TargetAgon {
		return fmt.Errorf("SALV needs a native target screen/key adapter")
	}
	platform := salvCPMPlatform
	if hm.Target == hir.TargetAgon {
		platform = salvAgonPlatform
	}
	for _, source := range []string{salvRuntime, platform} {
		parsed, err := nanz.Parse(source, "abap-salv-runtime.nanz")
		if err != nil {
			return err
		}
		for _, f := range parsed.Funcs {
			if !f.IsExtern {
				f.Internal = true
				hm.Funcs = append(hm.Funcs, f)
			}
		}
	}
	for _, text := range []struct{ name, value string }{
		{"title", "ALV Table | Rows: "}, {"first", " | First: "}, {"help", "Up/Down PgUp/PgDn Home/End | Esc/F3/q: Back"},
		{"cpm_begin", "\x1b[0m\x1b[2J\x1b[H\x1b[?25l"}, {"cpm_end", "\x1b[0m\x1b[?25h\x1b[24;1H\x1b[2K\r\n"},
	} {
		symbol := fmt.Sprintf("@mir2.str.%d", len(hm.Strings))
		hm.Strings = append(hm.Strings, text.value)
		hm.StrKinds = append(hm.StrKinds, mir2.StrCString)
		hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_salv_" + text.name + "_text", Internal: true, RetTy: mir2.TyPtr, Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.AddrOfExpr{Sym: symbol}}}}})
	}
	hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_salv_number_data", Internal: true, RetTy: mir2.TyPtr, Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.AddrOfExpr{Sym: "_abap_salv_number_buffer"}}}}})
	if hm.Target == hir.TargetAgon {
		for _, op := range []struct{ name, fn string }{{"_abap_salv_screen_begin", "@mir.agon.screen.begin"}, {"_abap_salv_screen_end", "@mir.agon.screen.end"}} {
			hm.Funcs = append(hm.Funcs, &hir.Func{Name: op.name, Internal: true, RetTy: mir2.TyVoid, Body: &hir.Block{Body: []hir.Stmt{&hir.ExprStmt{Expr: &hir.CallExpr{Fn: op.fn, Ty: mir2.TyU8}}}}})
		}
		hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_salv_key_poll", Internal: true, RetTy: mir2.TyU8, Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.CallExpr{Fn: "@mir.agon.key.poll", Ty: mir2.TyU8}}}}})
		return nil
	}
	hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_salv_key_poll", Internal: true, RetTy: mir2.TyU8, Body: &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Target: "z80", Code: "LD E, 255 / LD C, 6 / CALL 5", RetReg: "A", ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"}}}}})
	return nil
}

// Native lifecycle primitives also restore the VDP state during fault cleanup.
// Cursor positioning is ordinary compiled byte output in zero-based VDU form.
const salvAgonPlatform = `
@extern fun _abap_num_putchar(ch: u8) -> void

fun _abap_salv_goto(x: u8, y: u8) -> void {
 _abap_num_putchar(31)
 _abap_num_putchar(x)
 _abap_num_putchar(y)
}

`
