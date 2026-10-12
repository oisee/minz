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
