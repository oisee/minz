package abap

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// The Agon provider adapts native FatFS entries to the shared bounded FCB/DMA
// protocol. Catalog ordering and name conversion execute as compiled code.
const agonFrontendRuntime = `
@extern fun _abap_fault(code: u8) -> void
@extern fun _abap_fs_valid_char(ch: u8) -> bool
@extern fun _abap_fs_fcb() -> ^u8
@extern fun _abap_fs_dma() -> ^u8
@extern fun _abap_fs_agon_info() -> ^u8
@extern fun _abap_fs_agon_name() -> ^u8
@extern fun _abap_fs_agon_records() -> ^u8
@extern fun _abap_fs_agon_scratch() -> ^u8
@extern fun _abap_agon_native_open() -> u8
@extern fun _abap_agon_native_close() -> u8
@extern fun _abap_agon_native_dir_open() -> u8
@extern fun _abap_agon_native_dir_read() -> u8
@extern fun _abap_agon_native_dir_close() -> u8

global _abap_fs_agon_count: u16 = 0
global _abap_fs_agon_index: u16 = 0

fun _abap_fs_agon_open() -> u8 {
 var f: ^u8 = _abap_fs_fcb()
 if f[0] > 1 { return 255 }
 var name: ^u8 = _abap_fs_agon_name()
 var n: u16 = 0
 var i: u16 = 1
 while i < 9 { if f[i] != 32 { name[n] = f[i]
 n = n + 1 }
 i = i + 1 }
 if n == 0 { return 255 }
 if f[9] != 32 { name[n] = 46
 n = n + 1
 i = 9
 while i < 12 { if f[i] != 32 { name[n] = f[i]
 n = n + 1 }
 i = i + 1 } }
 name[n] = 0
 return _abap_agon_native_open()
}
fun _abap_fs_agon_encode() -> bool {
 var info: ^u8 = _abap_fs_agon_info()
 if (info[8] & 24) != 0 { return false }
 var dst: ^u8 = _abap_fs_agon_scratch()
 var j: u16 = 0
 while j < 11 { dst[j] = 32
 j = j + 1 }
 var i: u16 = 0
 var k: u16 = 0
 var dot: bool = false
 while i < 13 {
  var ch: u8 = info[22 + i]
  if ch == 0 { return k != 0 && (!dot || k != 8) }
  if ch == 46 {
   if dot || k == 0 { return false }
   dot = true
   k = 8
  } else {
   if !_abap_fs_valid_char(ch) { return false }
   if k >= 11 || (!dot && k >= 8) { return false }
   if ch >= 97 && ch <= 122 { ch = ch - 32 }
   dst[k] = ch
   k = k + 1
  }
  i = i + 1
 }
 return false
}
fun _abap_fs_agon_matches() -> bool {
 var f: ^u8 = _abap_fs_fcb()
 var name: ^u8 = _abap_fs_agon_scratch()
 var i: u16 = 0
 while i < 11 {
  var pat: u8 = f[i + 1] & 127
  if pat != 63 && pat != name[i] { return false }
  i = i + 1
 }
 return true
}
fun _abap_fs_agon_less(left: ^u8, right: ^u8) -> bool {
 var i: u16 = 0
 while i < 11 {
  if left[i] != right[i] { return left[i] < right[i] }
  i = i + 1
 }
 return false
}
fun _abap_fs_agon_insert() -> bool {
 var records: ^u8 = _abap_fs_agon_records()
 var name: ^u8 = _abap_fs_agon_scratch()
 var pos: u16 = _abap_fs_agon_count
 var i: u16 = 0
 while i < _abap_fs_agon_count {
  var old: ^u8 = records + i * 11
  if !_abap_fs_agon_less(old, name) && !_abap_fs_agon_less(name, old) { return true }
  i = i + 1
 }
 if pos == 128 { return false }
 while pos > 0 {
  var prev: ^u8 = records + (pos - 1) * 11
  if !_abap_fs_agon_less(name, prev) { break }
  var dst: ^u8 = records + pos * 11
  i = 0
  while i < 11 { dst[i] = prev[i]
  i = i + 1 }
  pos = pos - 1
 }
 var dst: ^u8 = records + pos * 11
 i = 0
 while i < 11 { dst[i] = name[i]
 i = i + 1 }
 _abap_fs_agon_count = _abap_fs_agon_count + 1
 return true
}
fun _abap_fs_agon_next() -> u8 {
 if _abap_fs_agon_index >= _abap_fs_agon_count { return 255 }
 var dma: ^u8 = _abap_fs_dma()
 var i: u16 = 0
 while i < 32 { dma[i] = 0
 i = i + 1 }
 var src: ^u8 = _abap_fs_agon_records() + _abap_fs_agon_index * 11
 i = 0
 while i < 11 { dma[i + 1] = src[i]
 i = i + 1 }
 _abap_fs_agon_index = _abap_fs_agon_index + 1
 return 0
}
fun _abap_fs_agon_abort() -> u8 {
 _abap_agon_native_dir_close()
 _abap_fs_agon_count = 0
 _abap_fs_agon_index = 0
 return 254
}
fun _abap_fs_agon_first() -> u8 {
 _abap_fs_agon_count = 0
 _abap_fs_agon_index = 0
 if _abap_agon_native_dir_open() != 0 { return 254 }
 var info: ^u8 = _abap_fs_agon_info()
 var scanned: u16 = 0
 while true {
  if _abap_agon_native_dir_read() != 0 { return _abap_fs_agon_abort() }
  if info[22] == 0 { break }
  if scanned == 4096 { return _abap_fs_agon_abort() }
  scanned = scanned + 1
  if _abap_fs_agon_encode() && _abap_fs_agon_matches() {
   if !_abap_fs_agon_insert() { _abap_agon_native_dir_close()
   _abap_fault(7) }
  }
 }
 if _abap_agon_native_dir_close() != 0 { _abap_fs_agon_count = 0
 return 254 }
 return _abap_fs_agon_next()
}
`

func emitAgonFrontendRuntime(hm *hir.Module) error {
	m, e := nanz.Parse(agonFrontendRuntime, "abap-agon-frontend-runtime.nanz")
	if e != nil {
		return e
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
	}{{"info", 278}, {"name", 13}, {"records", 128 * 11}, {"scratch", 11}} {
		sym := "_abap_fs_agon_" + b.name + "_buffer"
		hm.Globals = append(hm.Globals, mir2.Global{Name: sym, Ty: mir2.NewArray(mir2.TyU8, b.size)})
		hm.Funcs = append(hm.Funcs, &hir.Func{Name: "_abap_fs_agon_" + b.name, Internal: true, RetTy: mir2.TyPtr, Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.AddrOfExpr{Sym: sym}}}}})
	}
	for _, op := range []struct {
		name, fn string
	}{
		{"_abap_fs_bdos_open", "_abap_fs_agon_open"}, {"_abap_fs_bdos_close", "_abap_agon_native_close"},
		{"_abap_fs_bdos_read", "@mir.agon.fs.read"}, {"_abap_fs_bdos_first", "_abap_fs_agon_first"}, {"_abap_fs_bdos_next", "_abap_fs_agon_next"},
		{"_abap_fs_bdos_key", "@mir.agon.key.poll"},
		{"_abap_agon_native_open", "@mir.agon.fs.open"}, {"_abap_agon_native_close", "@mir.agon.fs.close"},
		{"_abap_agon_native_dir_open", "@mir.agon.dir.open"}, {"_abap_agon_native_dir_read", "@mir.agon.dir.read"}, {"_abap_agon_native_dir_close", "@mir.agon.dir.close"},
	} {
		hm.Funcs = append(hm.Funcs, &hir.Func{Name: op.name, Internal: true, RetTy: mir2.TyU8, Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.CallExpr{Fn: op.fn, Ty: mir2.TyU8}}}}})
	}
	return nil
}
