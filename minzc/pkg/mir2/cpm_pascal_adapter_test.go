package mir2

import (
	"strings"
	"testing"
)

func TestCPMPascalNamedMachineBodyAuthority(t *testing.T) {
	bodies := map[string][]string{"WriteStr": {"EXDE,HL", "LDC,9", "CALL0X0005", "RET"}, "WriteCrLf": {"LDE,13", "LDC,2", "CALL5", "LDE,10", "LDC,2", "CALL5", "RET"}}
	body := "main:\n CALL WriteStr\n CALL WriteCrLf\n RET\nWriteStr:\n EX DE,HL\n LD C,9\n CALL 0x0005\n RET\nWriteCrLf:\n LD E,13\n LD C,2\n CALL 5\n LD E,10\n LD C,2\n CALL 5\n RET\n"
	code := cpmRuntimeEntry + body + cpmRuntimeTail
	if e := validateCPMRuntimeAssemblyAdapters(code, false, bodies); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(code, "LD C,9", "LD C,2", 1), strings.Replace(code, "WriteStr:\n", "WriteStr:\n NOP\n", 1), strings.Replace(code, " CALL WriteStr", " CALL 0x0005", 1), strings.Replace(code, "WriteCrLf:\n", "fakeCR:\n", 1)} {
		if e := validateCPMRuntimeAssemblyAdapters(bad, false, bodies); e == nil {
			t.Fatal("mutated adapter admitted")
		}
	}
}

func TestCPMDecimalMachineBodyAndHelperAuthority(t *testing.T) {
	entry := strings.Replace(cpmRuntimeEntry, "    CALL main", "    CALL __cpm_stack_check\n    CALL main", 1)
	asm := func(code string) string {
		var b strings.Builder
		for _, line := range strings.Split(code, "/") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if fields[0] == "PUSH" || fields[0] == "CALL" {
				b.WriteString(" CALL __cpm_stack_check\n")
			}
			b.WriteString(" " + line + "\n")
		}
		return b.String() + " RET\n"
	}
	code := entry + "main:\n CALL __cpm_stack_check\n CALL WriteI16\n RET\nWriteI16:\n" + asm(PascalSignedDecimalPrefix+" / "+strings.TrimPrefix(PascalUnsignedDecimalASM, "PUSH IX / ")) + "_wr16_dig:\n" + asm(PascalDecimalDigitASM) + cpmRuntimeGuardedTail(1024)
	bodies := map[string][]string{}
	for name, template := range map[string]string{"WriteI16": PascalSignedDecimalPrefix + " / " + strings.TrimPrefix(PascalUnsignedDecimalASM, "PUSH IX / "), "_wr16_dig": PascalDecimalDigitASM} {
		lines := []string{}
		for _, line := range strings.Split(template, "/") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		i := &Inst{Asm: &AsmExtra{Template: strings.Join(lines, "\n")}}
		bodies[name] = cpmExactDecimalBody(i, template)
	}
	if e := validateCPMRuntimeAssemblyAdapters(code, true, bodies); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(code, "LD E, 45", "LD E, 43", 1), strings.Replace(code, "CALL _wr16_dig", "CALL WriteI16", 1), strings.Replace(code, "_w16_cont: INC A", "_w16_cont: DEC A", 1), strings.Replace(code, "_w16_cont: INC A", "_w16_cont EQU 0\n INC A", 1)} {
		if e := validateCPMRuntimeAssemblyAdapters(bad, true, bodies); e == nil {
			t.Fatal("tampered decimal machine body admitted")
		}
	}
}
