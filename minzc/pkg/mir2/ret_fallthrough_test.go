package mir2

import (
	"strings"
	"testing"
)

func TestElimJrToRetKeepsArithmeticFallthrough(t *testing.T) {
	lines := []string{"    JR Z, .done", "    NEG", ".done:", "    RET"}
	got := strings.Join(elimJrToRet(lines), "\n")
	if !strings.HasSuffix(got, "    RET") {
		t.Fatalf("removed reachable fallthrough RET:\n%s", got)
	}
}
