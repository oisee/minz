package mir2

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func cpmNarrowFixture() (*Module, *AllocResult) {
	m := &Module{}
	f := m.AddFunc("main")
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpAlloca, Dst: 1, Ty: TyPtr, Imm: 2, Cls: ClassPointer}, {Op: OpMove, Dst: 2, Src: [2]Reg{1}, Ty: TyU8, Cls: ClassGeneral}, {Op: OpConst, Dst: 3, Ty: TyU8, Imm: 42, Cls: ClassAcc}, {Op: OpStore, Src: [2]Reg{2, 3}, Ty: TyU8}}
	b.Term = &TermRet{}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "HL"}, 2: {Kind: LocMem, Name: "_spill_main_r2"}, 3: {Kind: LocReg, Name: "A"}}}
	return m, ar
}
func TestCPMLocalReviewNarrowAddress(t *testing.T) {
	m, ar := cpmNarrowFixture()
	if e := Verify(m); e != nil {
		t.Fatal(e)
	}
	text, e := Z80Codegen(m, ar, Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: 1024})
	if e != nil {
		t.Log("refused", e)
		return
	}
	a, e := z80asm.NewAssembler().AssembleString(text)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	for i := range z.Memory() {
		z.Memory()[i] = byte(i*41 + 29)
	}
	z.LoadMemory(0x100, a.Binary)
	prof := emulator.NewProfiler()
	z.SetProfiler(prof)
	mem := z.Memory()
	mem[0xeffe], mem[0xefff] = 0, 0
	z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xeffe})
	done := false
	for n := 0; n < 1000; n++ {
		if z.GetPC() == 0 && z.GetSP() == 0xf000 {
			done = true
			break
		}
		z.Step()
	}
	if !done {
		t.Fatal("termination", z.GetRegisters())
	}
	writes := map[int]byte{}
	lo, hi := int(a.Symbols["__cpm_stack"]), int(a.Symbols["__cpm_stack_top"])
	for addr, count := range prof.WriteCount {
		if count > 0 && !(addr >= lo && addr < hi) && addr != int(a.Symbols["__cpm_saved_sp"]) && addr != int(a.Symbols["__cpm_saved_sp"])+1 && addr != int(a.Symbols["_spill_main_r2"]) {
			writes[addr] = mem[addr]
		}
	}
	t.Logf("accepted illegal writes %+v", writes)
	if dir := os.Getenv("CPM_LOCAL_REVIEW_EXPORT"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "before.a80"), []byte(text), 0644)
		os.WriteFile(filepath.Join(dir, "before.bin"), a.Binary, 0644)
		os.WriteFile(filepath.Join(dir, "before.mir"), []byte(m.Dump()), 0644)
		b, _ := json.MarshalIndent(map[string]any{"allocation": ar, "illegal_writes": writes, "stack_bottom": lo, "stack_top": hi, "spill": a.Symbols["_spill_main_r2"], "return": z.GetRegisters()}, "", "  ")
		os.WriteFile(filepath.Join(dir, "before.json"), b, 0644)
	}
	if len(writes) == 0 {
		t.Fatal("no counterexample")
	}
	fmt.Println("counterexample reproduced")
}
