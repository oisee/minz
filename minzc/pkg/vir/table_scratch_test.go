package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestTableScratchMustPreserveLiveAccumulator(t *testing.T) {
	ops := []VIROp{{Op: OpConst, Dst: 1, Width: 8, Imm: 37}, {Op: OpConst, Dst: 2, Width: 8, Imm: 5}, {Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 8}, {Op: OpRet, Src: [2]int{1, -1}, Width: 8}}
	f := &mir2.Func{Name: "scratch"}
	vf := &Func{Name: "scratch", Blocks: []Block{{Ops: ops}}}
	assignments := map[int]int{1: Z80.LocByName("A"), 2: Z80.LocByName("H"), 3: Z80.LocByName("IXH")}
	asm, err := emitFromTable(f, vf, ops, assignments, Z80, SolverOptions{})
	if err != nil {
		t.Logf("unsafe table rejected: %v", err)
		return
	}
	assembled, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(assembled.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, assembled.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	if err := z.LoadMemory(0x8000, append([]byte{0xcd, 5, 0x80, 0xf3, 0x76}, assembled.Binary...)); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if z.GetRegisters().A != 37 {
		t.Fatalf("accepted miscompile A=%d want37 ASM=%s BIN=%x", z.GetRegisters().A, asm, assembled.Binary)
	}
}

func TestTableRejectsImplicitAccumulatorRepair(t *testing.T) {
	ops := []VIROp{{Op: OpConst, Dst: 1, Width: 8, Imm: 37}, {Op: OpConst, Dst: 2, Width: 8, Imm: 5}, {Op: OpConst, Dst: 3, Width: 8, Imm: 7}, {Op: OpAdd, Dst: 4, Src: [2]int{2, 3}, Width: 8}, {Op: OpRet, Src: [2]int{1, -1}, Width: 8}}
	assignment := map[int]int{1: 0, 2: 1, 3: 2, 4: 3}
	f := &mir2.Func{Name: "repair"}
	vf := &Func{Name: "repair", Blocks: []Block{{Ops: ops}}}
	if asm, err := emitFromTable(f, vf, ops, assignment, Z80, SolverOptions{}); err == nil || asm != "" {
		t.Fatal("uncertified accumulator repair admitted")
	}
}

func TestTableSafeScratchExecutesNative(t *testing.T) {
	ops := []VIROp{{Op: OpConst, Dst: 1, Width: 8, Imm: 37}, {Op: OpConst, Dst: 2, Width: 8, Imm: 5}, {Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 8}, {Op: OpMove, Dst: 4, Src: [2]int{3, -1}, Width: 8}, {Op: OpStoreGlobal, Src: [2]int{4, -1}, Width: 8, Imm: 0x9000}, {Op: OpMove, Dst: 5, Src: [2]int{1, -1}, Width: 8}, {Op: OpRet, Src: [2]int{5, -1}, Width: 8}}
	assignment := map[int]int{1: Z80.LocByName("B"), 2: Z80.LocByName("H"), 3: Z80.LocByName("IXH"), 4: 0, 5: 0}
	f := &mir2.Func{Name: "safe"}
	vf := &Func{Name: "safe", Blocks: []Block{{Ops: ops}}}
	asm, err := emitFromTable(f, vf, ops, assignment, Z80, SolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assembled, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(assembled.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, assembled.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	if err := z.LoadMemory(0x8000, append([]byte{0xcd, 5, 0x80, 0xf3, 0x76}, assembled.Binary...)); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	regs := z.GetRegisters()
	if regs.A != 37 || z.GetMemory(0x9000) != 5 || regs.SP != 0xf000 {
		t.Fatalf("A=%d mem=%d SP=%04x", regs.A, z.GetMemory(0x9000), regs.SP)
	}
	t.Logf("native A=37 memory9000=5 SP=f000 BIN=%x T=%d", assembled.Binary, z.GetCycles())
}

func TestTablePatternFlagScratchRejected(t *testing.T) {
	ops := []VIROp{{Op: OpCmp, Dst: 1, Src: [2]int{2, 3}, Width: 8}, {Op: OpConst, Dst: 4, Width: 8, Imm: 7}, {Op: OpAddImm, Dst: 5, Src: [2]int{4, -1}, Width: 8, Imm: 1}, {Op: OpRet, Src: [2]int{1, -1}, Width: 1}}
	assignment := map[int]int{1: Z80.LocByName("F"), 2: 0, 3: 1, 4: 0, 5: 0}
	pat := findBestPattern(ops[2], 0, [2]int{0, -1}, Z80)
	if pat == nil {
		t.Fatal("missing direct add pattern")
	}
	if err := validateTablePatternWrites(ops, 2, assignment, Z80, PIROp{Pat: pat, DstPhys: 0, SrcPhys: [2]int{0, -1}, Imm: 1}); err == nil {
		t.Fatal("live condition flags clobbered")
	}
}
