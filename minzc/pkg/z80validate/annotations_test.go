package z80validate

import "testing"

func TestAnnotatedInstructionValidation(t *testing.T) {
	asm := "f:\n CALL external ; @minz-spill-call:f0_b0_i1\n LD HL, home ; @minz-memory-access:load\n RET\n"
	if e := Validate(asm); len(e) != 0 {
		t.Fatal("valid annotated instructions rejected", e)
	}
	refs := map[string]bool{}
	CollectBranchTargets(" CALL external ; @minz-spill-call:site", refs)
	if len(refs) != 1 || !refs["external"] {
		t.Fatal("comment became branch target", refs)
	}
	if e := Validate("f:\n LD L, (D) ; @minz-memory-access:invalid\n RET\n"); len(e) == 0 {
		t.Fatal("invalid instruction hidden by annotation")
	}
}
