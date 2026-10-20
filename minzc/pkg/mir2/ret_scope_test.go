package mir2

import (
	"strings"
	"testing"
)

func TestConditionalRetUsesLocalLabelScope(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		bare := "quotient:\n    JR Z, .sign\n    LD HL, 1\n.sign:\n    RET\n"
		copyResult := "remainder:\n    JR Z, .sign\n    LD BC, 1\n.sign:\n    LD H, B\n    LD L, C\n    RET\n"
		src := bare + copyResult
		if reverse {
			src = copyResult + bare
		}
		got := strings.Join(elimJrToRet(strings.Split(src, "\n")), "\n")
		if !strings.Contains(got, "remainder:\n    JR Z, .sign") || !strings.Contains(got, "quotient:\n    RET Z") {
			t.Fatalf("reverse=%v: conditional return used a different function's local label:\n%s", reverse, got)
		}
	}
}
