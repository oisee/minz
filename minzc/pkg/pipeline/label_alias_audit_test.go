package pipeline

import (
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestLabelAuditRecognizesAssemblerEQUAliases(t *testing.T) {
	asm := "ORG 0x8000\nCALL node_tag\nRET\nnode_tag EQU peek\npeek:\nRET\n"
	result, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(result.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, result.Errors)
	}
	if warnings := auditLabels(asm); len(warnings) != 0 {
		t.Fatalf("valid assembled alias reported undefined: %v", warnings)
	}
}
