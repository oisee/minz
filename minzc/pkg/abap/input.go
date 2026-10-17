package abap

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Selection input has a bounded decimal grammar: surrounding ASCII spaces,
// an optional sign and at least one digit. Validate before committing a value.
// Fault 4 is invalid input; fault 5 is a value outside signed TYPE i range.
const integer32Input = `
@extern fun _abap_input_line() -> u8
@extern fun _abap_input_data() -> ^u8

fun _abap_parse_i32(data: ^u8, count: u8) -> i32 {
    var first: u16 = 0
    var end: u16 = u16(count)
    while first < end {
        if data[first] != 32 { break }
        first = first + 1
    }
    while end > first {
        if data[end - 1] != 32 { break }
        end = end - 1
    }
    var negative: bool = false
    if first < end {
        if data[first] == 45 {
            negative = true
            first = first + 1
        } else if data[first] == 43 { first = first + 1 }
    }
    if first == end {
        _abap_fault(4)
        return 0
    }
    var magnitude: u32 = 0
    var last: u8 = 7
    if negative { last = 8 }
    while first < end {
        var ch: u8 = data[first]
        if ch < 48 || ch > 57 {
            _abap_fault(4)
            return 0
        }
        var digit: u8 = ch - 48
        if magnitude > 214748364 {
            _abap_fault(5)
            return 0
        }
        if magnitude == 214748364 && digit > last {
            _abap_fault(5)
            return 0
        }
        magnitude = magnitude * 10 + u32(digit)
        first = first + 1
    }
    if negative { magnitude = u32(0) - magnitude }
    return i32(magnitude)
}

fun _abap_read_i32(dst: ^i32) -> void {
    var count: u8 = _abap_input_line()
    _abap_num_putchar(13)
    _abap_num_putchar(10)
    if count != 0 { dst^ = _abap_parse_i32(_abap_input_data(), count) }
}

fun _abap_read_c(dst: ^u8, length: u16) -> void {
    var count: u8 = _abap_input_line()
    _abap_num_putchar(13)
    _abap_num_putchar(10)
    if count != 0 {
        var data: ^u8 = _abap_input_data()
        var i: u16 = 0
        while i < u16(count) {
            if data[i] > 127 { _abap_fault(4) }
            i = i + 1
        }
        _abap_char_copy(dst, length, data, u16(count))
    }
}
`

// Only line acquisition is target-specific. Decimal conversion and checked
// assignment run through ordinary typed HIR/MIR on both VM and native Z80.
func emitIntegerInputIO(hm *hir.Module, target uint8) {
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name: "_abap_input_data", RetTy: mir2.TyPtr,
		Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.BinExpr{
			Op: "+", L: &hir.AddrOfExpr{Sym: "_abap_inbuf"}, R: charLen(2), Ty: mir2.TyPtr,
		}}}},
	})
	code := "LD DE, _abap_inbuf / LD A, 20 / LD (_abap_inbuf), A / LD C, 10 / CALL 5 / LD A, (_abap_inbuf+1)"
	if target == hir.TargetZXSpectrum {
		// The console port uses 0 for no input and bit 7 as the ready flag.
		// Keep polling after a full buffer; do not write beyond its 20 bytes.
		code = "LD HL, _abap_inbuf+2 / LD B, 0 / " +
			"._poll: IN A, (0x23) / OR A / JR Z, ._poll / AND 0x7F / " +
			"CP 13 / JR Z, ._done / CP 10 / JR Z, ._done / " +
			"CP 8 / JR Z, ._bs / CP 127 / JR Z, ._bs / " +
			"LD C, A / LD A, B / CP 20 / JR Z, ._poll / LD A, C / " +
			"LD (HL), A / INC HL / INC B / PUSH BC / PUSH HL / CALL _zx_putchar / POP HL / POP BC / JR ._poll / " +
			"._bs: LD A, B / OR A / JR Z, ._poll / DEC HL / DEC B / JR ._poll / " +
			"._done: LD A, B"
	}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name: "_abap_input_line", RetTy: mir2.TyU8,
		Body: &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{
			Target: "z80", Code: code, RetReg: "A", ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
		}}},
	})
	for _, g := range hm.Globals {
		if g.Name == "_abap_inbuf" {
			return
		}
	}
	hm.Globals = append(hm.Globals, mir2.Global{Name: "_abap_inbuf", Ty: mir2.NewArray(mir2.TyU8, 22), Init: make([]byte, 22)})
}
