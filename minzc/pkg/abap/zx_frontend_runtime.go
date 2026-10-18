package abap

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"regexp"
	"strings"
)

// MIT-owned adapter for MZX's existing sector API, not a TR-DOS ROM.
// Directory logical tracks interleave heads; the legacy trap groups heads.
// Always translate each sector independently. CSV uses bytes8..10 as extension,
// so the former load-address bytes are never used as a destination pointer.
const zxFrontendRuntime = `
@extern fun _abap_fault(code: u8) -> void
@extern fun _abap_fs_fcb() -> ^u8
@extern fun _abap_fs_dma() -> ^u8
@extern fun _abap_zx_sector_data() -> ^u8
@extern fun _abap_fs_valid_char(ch: u8) -> bool
@extern fun _abap_zx_scan() -> u8

global _abap_zx_dir_index: u16 = 0
global _abap_zx_remaining: u16 = 0
global _abap_zx_logical: u16 = 0
global _abap_zx_half: u8 = 0
global _abap_zx_last_key: u8 = 0
global _abap_zx_sequence: u8 = 0
global _abap_zx_sequence_code: u8 = 0

fun _abap_zx_sector(@z80_d track: u8, @z80_e sector: u8, @z80_hl buffer: ^u8) -> u8 {
 asm z80 (in track, sector, buffer) (ret A) (clob all) {
  LD B,1
  LD C,5
  CALL 0x3d13
  LD A,B
 }
}
fun _abap_zx_entry(index: u16) -> ^u8 {
 if index >= 128 { _abap_fault(8) }
 var buffer: ^u8 = _abap_zx_sector_data()
 if _abap_zx_sector(0, u8(index / 16) + 1, buffer) != 0 { _abap_fault(8) }
 return buffer + (index % 16) * 16
}
fun _abap_zx_csv(entry: ^u8) -> bool {
 if entry[0] == 0 || entry[0] == 1 { return false }
 var i: u16 = 0
 while i < 8 {
  if entry[i] != 32 && !_abap_fs_valid_char(entry[i]) { return false }
  i = i + 1
 }
 if entry[8] != 67 && entry[8] != 99 { return false }
 if entry[9] != 83 && entry[9] != 115 { return false }
 return entry[10] == 86 || entry[10] == 118
}
fun _abap_zx_matches(entry: ^u8) -> bool {
 var f: ^u8 = _abap_fs_fcb()
 var i: u16 = 0
 while i < 11 {
  var ch: u8 = entry[i]
  if ch >= 97 && ch <= 122 { ch = ch - 32 }
  if f[i + 1] != 63 && f[i + 1] != ch { return false }
  i = i + 1
 }
 return true
}
fun _abap_zx_extent(entry: ^u8) -> u16 {
 var length: u16 = u16(entry[11]) | (u16(entry[12]) << 8)
 var start: u16 = u16(entry[15]) * 16 + u16(entry[14])
 if entry[14] >= 16 || entry[15] >= 160 || start < 16 { _abap_fault(8) }
 if u32(length) > u32(entry[13]) * 256 || start + u16(entry[13]) > 2560 { _abap_fault(8) }
 return start
}
fun _abap_fs_zx_next() -> u8 {
 while _abap_zx_dir_index < 128 {
  var entry: ^u8 = _abap_zx_entry(_abap_zx_dir_index)
  _abap_zx_dir_index = _abap_zx_dir_index + 1
  if entry[0] == 0 { _abap_zx_dir_index = 128
  return 255 }
  if _abap_zx_csv(entry) && _abap_zx_matches(entry) {
   _abap_zx_extent(entry)
   var dma: ^u8 = _abap_fs_dma()
   var i: u16 = 0
   while i < 32 { dma[i] = 0
   i = i + 1 }
   i = 0
   while i < 11 { var ch: u8 = entry[i]
   if ch >= 97 && ch <= 122 { ch = ch - 32 }
   dma[i + 1] = ch
   i = i + 1 }
   return 0
  }
 }
 return 255
}
fun _abap_fs_zx_first() -> u8 {
 _abap_zx_dir_index = 0
 return _abap_fs_zx_next()
}
fun _abap_fs_zx_open() -> u8 {
 var found: bool = false
 var index: u16 = 0
 while index < 128 {
  var entry: ^u8 = _abap_zx_entry(index)
  if entry[0] == 0 { break }
  if _abap_zx_csv(entry) && _abap_zx_matches(entry) {
   if found { _abap_fault(8) }
   found = true
   _abap_zx_logical = _abap_zx_extent(entry)
   _abap_zx_remaining = u16(entry[11]) | (u16(entry[12]) << 8)
  }
  index = index + 1
 }
 _abap_zx_half = 0
 if !found { return 255 }
 return 0
}
fun _abap_fs_zx_close() -> u8 {
 _abap_zx_remaining = 0
 _abap_zx_half = 0
 return 0
}
fun _abap_fs_zx_read() -> u8 {
 if _abap_zx_remaining == 0 { return 1 }
 var source: ^u8 = _abap_zx_sector_data()
 if _abap_zx_half == 0 {
  if _abap_zx_logical < 16 || _abap_zx_logical >= 2560 { return 254 }
  var logical_track: u16 = _abap_zx_logical / 16
  var track: u8 = u8(logical_track / 2 + (logical_track % 2) * 80)
  if _abap_zx_sector(track, u8(_abap_zx_logical % 16) + 1, source) != 0 { return 254 }
  _abap_zx_logical = _abap_zx_logical + 1
 }
 source = source + u16(_abap_zx_half) * 128
 var dma: ^u8 = _abap_fs_dma()
 var i: u16 = 0
 while i < 128 {
  var ch: u8 = 26
  if _abap_zx_remaining > 0 { ch = source[i]
  _abap_zx_remaining = _abap_zx_remaining - 1 }
  dma[i] = ch
  i = i + 1
 }
 _abap_zx_half = 1 - _abap_zx_half
 return 0
}
// Native F requests value help, digits choose rows, Enter accepts; Q cancels.
// Caps Shift+6/7 are down/up. These produce the shared decoder's byte protocol locally.
fun _abap_zx_key_poll() -> u8 {
 if _abap_zx_sequence == 1 { _abap_zx_sequence = 2
 if _abap_zx_sequence_code == 83 { return 79 }
 return 91 }
 if _abap_zx_sequence == 2 { _abap_zx_sequence = 0
 return _abap_zx_sequence_code }
 var key: u8 = _abap_zx_scan()
 if key == _abap_zx_last_key { return 0 }
 _abap_zx_last_key = key
 if key == 0 { return 0 }
 if key == 70 || key == 129 || key == 130 {
  _abap_zx_sequence = 1
  _abap_zx_sequence_code = 83
  if key == 129 { _abap_zx_sequence_code = 66 }
  if key == 130 { _abap_zx_sequence_code = 65 }
  return 27
 }
 if key == 81 { return 27 }
 return key
}
fun _abap_fs_bdos_key() -> u8 { return _abap_zx_key_poll() }
fun _abap_salv_key_poll() -> u8 { return _abap_zx_key_poll() }
fun _abap_fs_bdos_open() -> u8 { return _abap_fs_zx_open() }
fun _abap_fs_bdos_close() -> u8 { return _abap_fs_zx_close() }
fun _abap_fs_bdos_first() -> u8 { return _abap_fs_zx_first() }
fun _abap_fs_bdos_next() -> u8 { return _abap_fs_zx_next() }
fun _abap_fs_bdos_read() -> u8 { return _abap_fs_zx_read() }
`

const salvZXPlatform = `
@extern fun _zx_cls() -> void
@extern fun _abap_zx_cursor_data() -> ^u8
fun _abap_salv_screen_begin() -> void { _zx_cls() }
fun _abap_salv_screen_end() -> void { _abap_salv_goto(0, 23) }
fun _abap_salv_goto(x: u8, y: u8) -> void {
 var cursor: ^u8 = _abap_zx_cursor_data()
 if x < 32 && y < 24 { cursor[0] = y
 cursor[1] = x }
}
`

func emitZXFrontendRuntime(hm *hir.Module) error {
	parsed, e := nanz.Parse(zxFrontendRuntime, "abap-zx-frontend-runtime.nanz")
	if e != nil {
		return e
	}
	hm.Globals = append(hm.Globals, parsed.Globals...)
	for _, f := range parsed.Funcs {
		if !f.IsExtern {
			f.Internal = true
			hm.Funcs = append(hm.Funcs, f)
		}
	}
	hm.Globals = append(hm.Globals, mir2.Global{Name: "_abap_zx_sector_buffer", Ty: mir2.NewArray(mir2.TyU8, 256)})
	for _, p := range []struct{ name, sym string }{{"_abap_zx_sector_data", "_abap_zx_sector_buffer"}, {"_abap_zx_cursor_data", "_zx_cursor"}} {
		hm.Funcs = append(hm.Funcs, &hir.Func{Name: p.name, Internal: true, RetTy: mir2.TyPtr, Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.AddrOfExpr{Sym: p.sym}}}}})
	}
	// This native adapter deliberately avoids ROM keyboard routines and host text.
	code := `LD BC,0xfefe / IN A,(C) / AND 1 / LD H,A /
 LD BC,0xfdfe / IN A,(C) / BIT 3,A / LD A,70 / RET Z /
 LD BC,0xfbfe / IN A,(C) / BIT 0,A / LD A,81 / RET Z /
 LD BC,0xbffe / IN A,(C) / BIT 0,A / LD A,13 / RET Z /
 LD BC,0xf7fe / IN A,(C) / LD D,A / LD E,49 / LD B,5 /
 .digits: SRL D / JR NC,.found / INC E / DJNZ .digits /
 LD BC,0xeffe / IN A,(C) / BIT 0,A / LD E,48 / JR Z,.found /
 IN A,(C) / LD D,A / SRL D / LD E,57 / LD B,4 /
 .digits_right: SRL D / JR NC,.found / DEC E / DJNZ .digits_right / XOR A / RET /
 .found: LD A,H / OR A / LD A,E / RET NZ /
 CP 54 / LD A,129 / RET Z / LD A,E / CP 55 / LD A,130 / RET Z / LD A,E`
	hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_zx_scan", Internal: true, RetTy: mir2.TyU8, Body: &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Target: "z80", Code: strings.ReplaceAll(code, "\n", " "), RetReg: "A", ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"}}}}})
	return nil
}

// Only the bounded native frontend changes the old WRITE-only ZX prototype.
func bindZXFrontendConsole(hm *hir.Module) error {
	if hm.Target != hir.TargetZXSpectrum || hm.FuncByName("_abap_zx_key_poll") == nil {
		return nil
	}
	if main := hm.FuncByName("main"); main != nil {
		walkAgonHIR(reflect.ValueOf(main.Body), func(v any) {
			if c, ok := v.(*hir.CallExpr); ok && c.Fn == "_zx_border" && len(c.Args) == 1 {
				c.Args[0] = &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8}
			}
		})
	}
	if output := hm.FuncByName("abap_write_str"); output != nil {
		ref := func() hir.Expr { return &hir.VarRefExpr{Name: output.Params[0].Name, Ty: mir2.TyPtr} }
		output.Body = &hir.Block{Body: []hir.Stmt{charCall("_abap_char_write", ref(), &hir.CallExpr{Fn: "_abap_char_cstrlen", Args: []hir.Expr{ref()}, Ty: mir2.TyU16})}}
	}
	put := hm.FuncByName("_zx_putchar")
	if put == nil {
		return fmt.Errorf("ZX frontend has no console")
	}
	// Safe 32x24 bitmap output. CR resets X; LF advances Y. At the bottom,
	// saturate (subsequent render uses explicit positioning) rather than corrupt RAM.
	code := `CP 13 / JR Z,.cr / CP 10 / JR Z,.lf /
 CP 32 / JR C,.skip / CP 127 / JR NC,.skip /
 SUB 32 / LD L,A / LD H,0 / ADD HL,HL / ADD HL,HL / ADD HL,HL /
 LD DE,_zx_font / ADD HL,DE / PUSH HL /
 LD A,(_zx_cursor) / LD C,A / AND 0x18 / OR 0x40 / LD H,A /
 LD A,C / AND 7 / RRCA / RRCA / RRCA / LD C,A /
 LD A,(_zx_cursor+1) / OR C / LD L,A / POP DE / LD B,8 /
 .draw: LD A,(DE) / LD (HL),A / INC DE / INC H / DJNZ .draw /
 LD A,(_zx_cursor+1) / INC A / CP 32 / JR C,.store /
 XOR A / LD (_zx_cursor+1),A / JR .lf /
 .store: LD (_zx_cursor+1),A / RET /
 .cr: XOR A / LD (_zx_cursor+1),A / RET /
 .lf: LD A,(_zx_cursor) / CP 23 / RET NC / INC A / LD (_zx_cursor),A /
 .skip:`
	put.Body = &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Target: "z80", Code: strings.ReplaceAll(code, "\n", " "), Ins: []hir.AsmOperand{{Name: put.Params[0].Name}}, ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"}}}}
	if halt := hm.FuncByName("_zx_halt"); halt != nil {
		halt.Body = &hir.Block{}
	}
	// Faults leave the application via its saved caller stack, rather than HALT.
	fault := hm.FuncByName("_abap_fault")
	if fault != nil {
		fault.Body.Body = append(fault.Body.Body[:1], &hir.AsmStmt{Target: "z80", Code: "JP __zx_app_exit"})
	}
	return nil
}

// The ordinary WRITE-only frontend emits several opaque console helpers. Keep
// assembly CALL/JP dependencies as well as typed calls when pruning the native
// application's unused runtime functions. No ASM body is rewritten here.
func pruneZXApplication(hm *hir.Module) {
	if hm.Target != hir.TargetZXSpectrum || hm.FuncByName("_abap_fs_zx_read") == nil {
		return
	}
	live := map[string]bool{}
	call := regexp.MustCompile(`(?i)\b(?:CALL|JP)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	var visit func(string)
	visit = func(name string) {
		if live[name] {
			return
		}
		f := hm.FuncByName(name)
		if f == nil {
			return
		}
		live[name] = true
		walkAgonHIR(reflect.ValueOf(f.Body), func(v any) {
			switch n := v.(type) {
			case *hir.CallExpr:
				visit(n.Fn)
			case *hir.AsmStmt:
				for _, match := range call.FindAllStringSubmatch(n.Code, -1) {
					visit(match[1])
				}
			}
		})
	}
	visit("main")
	for _, a := range hm.Asserts {
		visit(a.FuncName)
	}
	keep := hm.Funcs[:0]
	for _, f := range hm.Funcs {
		if live[f.Name] {
			keep = append(keep, f)
		}
	}
	hm.Funcs = keep
}
