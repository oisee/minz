package mir2

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/z80validate"
)

// relaxFarDJNZ rewrites every DJNZ that validation rejected and whose target
// is a label of this function into
//
//	DJNZ .f_djnz_farN_t
//	JRS .f_djnz_farN_f
//	.f_djnz_farN_t:
//	JRS target
//	.f_djnz_farN_f:
//
// DJNZ has only an 8-bit displacement, so a loop body that grew past it is
// otherwise an assembly error. The trampoline keeps DJNZ's contract exactly:
// B is decremented once, no flag and no other register changes on either
// path. It reports whether any line was rewritten.
func relaxFarDJNZ(asm string, errs []z80validate.Error, fn string, next *int) (string, bool) {
	if len(errs) == 0 {
		return asm, false
	}
	lines := strings.Split(asm, "\n")
	labels := map[string]bool{}
	for _, line := range lines {
		code := strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		if strings.HasSuffix(code, ":") && !strings.ContainsAny(code, " \t") {
			labels[strings.TrimSuffix(code, ":")] = true
		}
	}
	far := map[int]string{}
	for _, e := range errs {
		i := e.Line - 1
		if i < 0 || i >= len(lines) {
			continue
		}
		fields := strings.Fields(strings.SplitN(lines[i], ";", 2)[0])
		if len(fields) != 2 || !strings.EqualFold(fields[0], "DJNZ") || !labels[fields[1]] {
			continue
		}
		far[i] = fields[1]
	}
	if len(far) == 0 {
		return asm, false
	}
	out := make([]string, 0, len(lines)+4*len(far))
	for i, line := range lines {
		target, ok := far[i]
		if !ok {
			out = append(out, line)
			continue
		}
		taken := fmt.Sprintf(".%s_djnz_far%d_t", fn, *next)
		fall := fmt.Sprintf(".%s_djnz_far%d_f", fn, *next)
		*next++
		out = append(out,
			"    DJNZ "+taken+"    ; body out of DJNZ range",
			"    JRS "+fall,
			taken+":",
			"    JRS "+target,
			fall+":")
	}
	return strings.Join(out, "\n"), true
}
