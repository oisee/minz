package abap

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Decimal conversion is ordinary typed code, using the same 32-bit arithmetic
// and local-frame lowering as application functions. In particular INT_MIN
// is converted to an unsigned magnitude without signed negation overflow.
const integer32Output = `
fun abap_write_i32(value: i32) -> void {
    var magnitude: u32 = u32(value)
    if value < 0 {
        _abap_num_putchar(45)
        magnitude = u32(0) - magnitude
    }
    var digits: [u8; 10]
    var count: u8 = 0
    while magnitude > 0 {
        digits[count] = u8(magnitude % 10) + 48
        magnitude = magnitude / 10
        count = count + 1
    }
    if count == 0 { _abap_num_putchar(48) }
    while count > 0 {
        count = count - 1
        _abap_num_putchar(digits[count])
    }
    _abap_num_putchar(32)
}
`

// Checked arithmetic runs in ordinary typed Nanz. Unsigned intermediates avoid
// host-language overflow and preserve INT_MIN's magnitude. The fault boundary
// terminates execution; no placeholder value can be committed by its caller.
const integer32Arithmetic = `
@extern fun _abap_fault(code: u8) -> void

fun _abap_add(a: i32, b: i32) -> i32 {
    var ua: u32 = u32(a)
    var ub: u32 = u32(b)
    var r: u32 = ua + ub
    if ((ua ^ r) & (ub ^ r) & u32(0x80000000)) != 0 { _abap_fault(2) }
    return i32(r)
}

fun _abap_sub(a: i32, b: i32) -> i32 {
    var ua: u32 = u32(a)
    var ub: u32 = u32(b)
    var r: u32 = ua - ub
    if ((ua ^ ub) & (ua ^ r) & u32(0x80000000)) != 0 { _abap_fault(2) }
    return i32(r)
}

fun _abap_neg(a: i32) -> i32 {
    if u32(a) == u32(0x80000000) { _abap_fault(2) }
    return i32(u32(0) - u32(a))
}

fun _abap_mul(a: i32, b: i32) -> i32 {
    var aa: u32 = u32(a)
    var bb: u32 = u32(b)
    if a < 0 { aa = u32(0) - aa }
    if b < 0 { bb = u32(0) - bb }
    var negative: bool = (a < 0) != (b < 0)
    var limit: u32 = 2147483647
    if negative { limit = 2147483648 }
    if bb != 0 {
        if aa > limit / bb { _abap_fault(2) }
    }
    var result: u32 = aa * bb
    if negative { result = u32(0) - result }
    return i32(result)
}

// mode 0: rounded '/', mode 1: Euclidean DIV, mode 2: nonnegative MOD.
fun _abap_division(a: i32, b: i32, mode: u8) -> i32 {
    if b == 0 {
        if a == 0 { return 0 }
        _abap_fault(1)
        return 0
    }
    var aa: u32 = u32(a)
    var bb: u32 = u32(b)
    if a < 0 { aa = u32(0) - aa }
    if b < 0 { bb = u32(0) - bb }
    var q: u32 = aa / bb
    var rem: u32 = aa % bb
    if mode == 2 {
        if a < 0 && rem != 0 { rem = bb - rem }
        return i32(rem)
    }
    if mode == 0 {
        if rem >= bb - rem { q = q + 1 }
    } else {
        if a < 0 && rem != 0 { q = q + 1 }
    }
    var negative: bool = (a < 0) != (b < 0)
    var limit: u32 = 2147483647
    if negative { limit = 2147483648 }
    if q > limit { _abap_fault(2) }
    if negative { q = u32(0) - q }
    return i32(q)
}
`

func emitInteger32Runtime(hm *hir.Module, target uint8) error {
	io := `fun _abap_num_putchar(ch: u8) -> void {
    asm z80 (in ch) (clob all) {
        LD E, A
        LD C, 2
        CALL 5
    }
}`
	if target == hir.TargetZXSpectrum {
		io = `@extern fun _zx_putchar(ch: u8) -> void
fun _abap_num_putchar(ch: u8) -> void { _zx_putchar(ch) }`
	}
	m, err := nanz.Parse(integer32Output+integer32Arithmetic+characterRuntime+stringRuntime+io, "abap-integer-runtime.nanz")
	if err != nil {
		return fmt.Errorf("ABAP integer runtime: %w", err)
	}
	for _, f := range m.Funcs {
		if strings.HasPrefix(f.Name, "_zx_") || f.Name == "_abap_fault" {
			continue
		}
		hm.Funcs = append(hm.Funcs, f)
	}
	hm.Globals = append(hm.Globals, mir2.Global{Name: "_abap_error_code", Ty: mir2.TyU8})
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name: "_abap_fault", Params: []hir.Param{{Name: "code", Ty: mir2.TyU8}}, RetTy: mir2.TyVoid,
		Body: &hir.Block{Body: []hir.Stmt{
			&hir.AssignStmt{Target: &hir.VarRefExpr{Name: "_abap_error_code", Ty: mir2.TyU8}, Val: &hir.VarRefExpr{Name: "code", Ty: mir2.TyU8}},
			&hir.ExprStmt{Expr: &hir.CallExpr{Fn: "@mir.io.halt", Ty: mir2.TyVoid}},
		}},
	}, &hir.Func{
		Name: "@mir.io.halt", RetTy: mir2.TyVoid,
		Body: &hir.Block{Body: []hir.Stmt{
			&hir.WhileStmt{Cond: &hir.BoolLitExpr{Val: true}, Body: &hir.Block{Body: []hir.Stmt{
				&hir.AsmStmt{Target: "z80", Code: "DI / HALT"},
			}}},
		}},
	})
	return nil
}
