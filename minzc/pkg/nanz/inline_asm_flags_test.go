package nanz_test

import (
	"bytes"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestInlineAsmFlagInputZ80(t *testing.T) {
	hm, err := nanz.Parse(`fun main() -> void { asm { IN F, (C) } }`, "inline_flags")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
	if err != nil {
		t.Fatalf("compile inline IN F: %v", err)
	}
	result, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("assemble inline IN F: %v", result.Errors)
	}
	if !bytes.Contains(result.Binary, []byte{0xed, 0x70}) {
		t.Fatalf("missing IN F,(C) opcode: %x", result.Binary)
	}
}
