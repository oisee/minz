package abap

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
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
	m, err := nanz.Parse(integer32Output+io, "abap-integer-runtime.nanz")
	if err != nil {
		return fmt.Errorf("ABAP integer runtime: %w", err)
	}
	for _, f := range m.Funcs {
		if strings.HasPrefix(f.Name, "_zx_") {
			continue
		}
		hm.Funcs = append(hm.Funcs, f)
	}
	return nil
}
