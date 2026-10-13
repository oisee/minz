package z80validate

import "testing"

func TestExternalWordAddressOffset(t *testing.T) {
	for _, operand := range []string{"_inbuf+2", "_inbuf + 2", "_inbuf^H", "(_inbuf+2)"} {
		asm := "terminate:\n LD HL, " + operand + "\n RET\n"
		if errs := Validate(asm); len(errs) != 0 {
			t.Errorf("valid external address %q rejected: %v", operand, errs)
		}
		refs := map[string]bool{}
		CollectBranchTargets("LD HL, "+operand, refs)
		if len(refs) != 1 || !refs["_inbuf"] {
			t.Errorf("address %q collected as labels %v", operand, refs)
		}
	}
	if errs := Validate("terminate:\n LD HL, (DE)\n RET\n"); len(errs) == 0 {
		t.Fatal("invalid word load accepted")
	}
}
