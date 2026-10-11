package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func tableAddOps() []VIROp {
	return []VIROp{{Op: OpConst, Dst: 1, Width: 8, Imm: 7}, {Op: OpConst, Dst: 2, Width: 8, Imm: 5}, {Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8}, {Op: OpRet, Src: [2]int{3, -1}, Width: 8}}
}

func TestTableRejectsMissingAssignments(t *testing.T) {
	ops := tableAddOps()
	assignment := map[int]int{1: Z80.LocByName("A"), 2: Z80.LocByName("B")}
	if verifyABICompat(ops, assignment, SolverOptions{ParamLocs: map[int]int{3: Z80.LocByName("A")}}) {
		t.Fatal("missing ABI/result vreg admitted")
	}
	var table RegAllocTable
	table.Init()
	sig := ComputeSignature(ops, Z80)
	table.Add(sig, &RegAllocEntry{Signature: sig, NVregs: 3, Assignment: []int{0, 1}})
	if _, _, ok := table.Lookup(ops, Z80); ok {
		t.Fatal("truncated assignment was reported as a table hit")
	}
}

func TestTableEmitterRejectsMissingAssignment(t *testing.T) {
	ops := tableAddOps()
	f := &mir2.Func{Name: "add"}
	vf := &Func{Name: "add", Blocks: []Block{{Ops: ops}}}
	asm, err := emitFromTable(f, vf, ops, map[int]int{1: 0, 2: 1}, Z80, SolverOptions{})
	if err == nil || asm != "" || !strings.Contains(err.Error(), "vreg 3") {
		t.Fatalf("got assembly=%q err=%v", asm, err)
	}
}

func TestTableCompleteAssignmentAssembles(t *testing.T) {
	ops := tableAddOps()
	f := &mir2.Func{Name: "add"}
	vf := &Func{Name: "add", Blocks: []Block{{Ops: ops}}}
	assignment := map[int]int{1: 0, 2: 1, 3: 0}
	if !verifyABICompat(ops, assignment, SolverOptions{ParamLocs: map[int]int{1: 0}}) {
		t.Fatal("complete compatible assignment rejected")
	}
	asm, err := emitFromTable(f, vf, ops, assignment, Z80, SolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(result.Errors) != 0 {
		t.Fatalf("assembly error=%v diagnostics=%v assembly=%s", err, result.Errors, asm)
	}
	if len(result.Binary) == 0 {
		t.Fatal("empty machine code")
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	// CALL at 8000 returns to DI/HALT at 8003; the function begins at 8005.
	program := append([]byte{0xcd, 0x05, 0x80, 0xf3, 0x76}, result.Binary...)
	if err := z.LoadMemory(0x8000, program); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	regs := z.GetRegisters()
	if regs.A != 12 || regs.SP != 0xf000 {
		t.Fatalf("actual Z80 A=%d SP=%04x, want12/f000", regs.A, regs.SP)
	}
	t.Logf("Complete assignment emitted assembled binary %x; actual Z80 A=%d SP=%04x T=%d", result.Binary, regs.A, regs.SP, z.GetCycles())
}

func TestTableLookupRejectsMalformedLocationsAndBinaryTruncation(t *testing.T) {
	ops := tableAddOps()
	for _, locs := range [][]int{{0, 1}, {0, 1, -1}, {0, 1, len(Z80.Locs)}} {
		var table RegAllocTable
		table.Init()
		sig := ComputeSignature(ops, Z80)
		table.Add(sig, &RegAllocEntry{Signature: sig, NVregs: 3, Assignment: locs})
		if _, _, ok := table.Lookup(ops, Z80); ok {
			t.Fatalf("malformed assignment %v admitted", locs)
		}
	}
	shape, _, err := VIRToEnrichedShape(ops, Z80)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := EnrichedIndexOfWithLocSets(*shape, 3, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]EnrichedEntry, idx+1)
	entries[idx] = EnrichedEntry{Cost: 7, Assignment: []byte{0, 1}}
	var table RegAllocTable
	table.Init()
	table.binaryTables = []*EnrichedBinaryTable{{Entries: entries, MaxVregs: 3, NLocSets8: 4, NLocSets16: 3}}
	if _, _, ok := table.Lookup(ops, Z80); ok {
		t.Fatal("truncated binary assignment admitted")
	}
}

func TestTableABIAndHintsRemainMandatory(t *testing.T) {
	ops := tableAddOps()
	assignment := map[int]int{1: 0, 2: 1, 3: 0}
	for _, opts := range []SolverOptions{{ParamLocs: map[int]int{1: 1}}, {ParamLocs: map[int]int{17: 0}}} {
		if verifyABICompat(ops, assignment, opts) {
			t.Fatalf("mismatched/missing ABI %v admitted", opts.ParamLocs)
		}
	}
	ops[2].DstHint = Z80.LocSetByNames("B")
	if verifyABICompat(ops, assignment, SolverOptions{}) {
		t.Fatal("destination hint ignored")
	}
	ops[2].DstHint = 0
	ops[2].SrcHint[1] = Z80.LocSetByNames("C")
	if verifyABICompat(ops, assignment, SolverOptions{}) {
		t.Fatal("source hint ignored")
	}
	ops = []VIROp{{Op: OpAsmBlock, AsmIns: []int{9}, AsmOuts: []int{10}}}
	if verifyABICompat(ops, map[int]int{9: 0}, SolverOptions{}) {
		t.Fatal("missing inline asm output admitted")
	}
}

func TestTableRejectsDefinedValueWidthMismatch(t *testing.T) {
	ops := []VIROp{{Op: OpConst, Dst: 1, Width: 16, Imm: 1000}}
	var table RegAllocTable
	table.Init()
	sig := ComputeSignature(ops, Z80)
	table.Add(sig, &RegAllocEntry{Signature: sig, NVregs: 1, Assignment: []int{Z80.LocByName("A")}})
	if _, _, ok := table.Lookup(ops, Z80); ok {
		t.Fatal("u16 assigned an 8-bit location")
	}
	if err := validateTableAssignment(ops, map[int]int{1: Z80.LocByName("HL")}, Z80); err != nil {
		t.Fatal(err)
	}
}
