package z80validate

import "testing"

func TestExternalSymbolByteOperand(t *testing.T) {
	for _, inst := range []string{
		"ADD A, _table^L", "ADC A, _table^H", "SUB _table^L", "SBC A, _table^H",
		"AND _table^L", "OR _table^H", "XOR _table^L", "CP _table^H",
		"LD A, (_table + 3)", "LD A, (_table+3)", "LD A, (_table)",
	} {
		asm := "lookup:\n " + inst + "\n RET\n"
		if errs := Validate(asm); len(errs) != 0 {
			t.Errorf("valid instruction %q rejected: %v", inst, errs)
		}
		refs := map[string]bool{}
		CollectBranchTargets(inst, refs)
		if len(refs) != 1 || !refs["_table"] {
			t.Errorf("%q collected as labels %v", inst, refs)
		}
	}
	for _, inst := range []string{"ADD A, B", "ADC A, (HL)", "ADD HL, DE", "CP 10", "XOR A"} {
		refs := map[string]bool{}
		CollectBranchTargets(inst, refs)
		if len(refs) != 0 {
			t.Errorf("%q collected as labels %v", inst, refs)
		}
	}
	if errs := Validate("lookup:\n ADC A, (DE)\n RET\n"); len(errs) == 0 {
		t.Fatal("invalid ADC accepted")
	}
}
