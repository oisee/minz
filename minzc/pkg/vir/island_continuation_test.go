package vir

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestIslandContinuationDoesNotReturnMidBlock(t *testing.T) {
	f := (&mir2.Module{}).AddFunc("continuation")
	b := f.NewBlock("entry")
	b.Seal(&mir2.TermRet{})
	ranges := []islandRange{{label: "entry", start: 0, end: 2}}
	first := []VIROp{{Op: OpCall}}
	last := []VIROp{{Op: OpConst, Dst: 1, Width: 8}}
	var sb strings.Builder
	emitPIRWithLabels(&sb, []PIROp{{Pat: &Pattern{Op: OpCall}, AsmText: "CALL helper"}}, first, ranges, 0, f.Name, Z80, f)
	if strings.Contains(sb.String(), "RET") {
		t.Fatalf("mid-block return: %s", sb.String())
	}
	emitPIRWithLabels(&sb, []PIROp{{Pat: &Pattern{Op: OpConst}, AsmText: "LD A, 42"}}, last, ranges, 1, f.Name, Z80, f)
	if strings.Count(sb.String(), "RET") != 1 || !strings.Contains(sb.String(), "LD A, 42") {
		t.Fatalf("missing continuation or terminal return: %s", sb.String())
	}
	ar, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\ncontinuation:\n" + sb.String() + "helper:\n RET\n")
	if err != nil || len(ar.Errors) != 0 {
		t.Fatalf("assemble: %v, %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
		t.Fatal(err)
	}
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xeffe})
	for steps := 0; steps < 16 && z.GetRegisters().PC != 0xf100; steps++ {
		z.Step()
	}
	r := z.GetRegisters()
	if r.A != 42 || r.PC != 0xf100 || r.SP != 0xf000 {
		t.Fatalf("continuation skipped or bad return: %+v", r)
	}
}
