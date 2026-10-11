package vir

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func cloneTableMachine(t *testing.T) *MachineDesc {
	t.Helper()
	data, err := json.Marshal(Z80)
	if err != nil {
		t.Fatal(err)
	}
	var desc MachineDesc
	if err := json.Unmarshal(data, &desc); err != nil {
		t.Fatal(err)
	}
	return &desc
}

func TestCheckedTableKeyBindsCurrentContract(t *testing.T) {
	ops := tableAddOps()
	base, err := CheckedTableKey(ops, Z80, SolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func([]VIROp, *MachineDesc, *SolverOptions){
		"opcode":    func(o []VIROp, d *MachineDesc, s *SolverOptions) { o[2].Op = OpSub },
		"operand":   func(o []VIROp, d *MachineDesc, s *SolverOptions) { o[2].Src[1] = 1 },
		"immediate": func(o []VIROp, d *MachineDesc, s *SolverOptions) { o[0].Imm++ },
		"symbol":    func(o []VIROp, d *MachineDesc, s *SolverOptions) { o[0].Sym = "changed" },
		"hint":      func(o []VIROp, d *MachineDesc, s *SolverOptions) { o[0].DstHint = Singleton(1) },
		"clobber":   func(o []VIROp, d *MachineDesc, s *SolverOptions) { o[1].Clobbers = Singleton(0) },
		"rule": func(o []VIROp, d *MachineDesc, s *SolverOptions) {
			d.Rules = append(d.Rules, ConstraintRule{Name: "new", DstSet: Singleton(0), SrcSet: Singleton(1), Cost: MaxCost})
		},
		"pattern flags":    func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.Patterns[0].Flags ^= PatCall },
		"pattern operands": func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.Patterns[0].DstLocs = Singleton(1) },
		"pattern tied": func(o []VIROp, d *MachineDesc, s *SolverOptions) {
			d.Patterns[0].TiedDstSrc = !d.Patterns[0].TiedDstSrc
		},
		"pattern scratch": func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.Patterns[0].Clobbers = Singleton(1) },
		"template":        func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.Patterns[0].Template = "NOP" },
		"cost":            func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.Patterns[0].Cost++ },
		"bytes":           func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.Patterns[0].Bytes++ },
		"alias":           func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.Locs[0].Alias = Singleton(1) },
		"location cost":   func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.LocCost[0]++ },
		"nonallocatable":  func(o []VIROp, d *MachineDesc, s *SolverOptions) { d.NonAllocatable = d.NonAllocatable.Set(0) },
		"ABI":             func(o []VIROp, d *MachineDesc, s *SolverOptions) { s.ParamLocs = map[int]int{1: 1} },
		"size mode":       func(o []VIROp, d *MachineDesc, s *SolverOptions) { s.OptSize = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := append([]VIROp(nil), ops...)
			desc := cloneTableMachine(t)
			var opts SolverOptions
			mutate(changed, desc, &opts)
			key, err := CheckedTableKey(changed, desc, opts)
			if err != nil || key == base {
				t.Fatalf("mutation not bound: key=%s err=%v", key, err)
			}
		})
	}
	renamed := append([]VIROp(nil), ops...)
	for i := range renamed {
		if renamed[i].Dst > 0 {
			renamed[i].Dst += 100
		}
		for j := range renamed[i].Src {
			if renamed[i].Src[j] > 0 {
				renamed[i].Src[j] += 100
			}
		}
	}
	if key, err := CheckedTableKey(renamed, Z80, SolverOptions{}); err != nil || key != base {
		t.Fatal("vreg renaming changed normalized key")
	}
}

func TestCheckedTableRefusesLegacyAndTamperedEvidence(t *testing.T) {
	ops := tableAddOps()
	assignment := map[int]int{1: 0, 2: 1, 3: 0}
	var table RegAllocTable
	table.Init()
	legacy := ComputeSignature(ops, Z80)
	table.Add(legacy, &RegAllocEntry{Signature: legacy, Assignment: []int{0, 1, 0}, NVregs: 3, NOps: 4, Cost: 1})
	if _, ok := table.LookupByKey(legacy); !ok {
		t.Fatal("historical raw research lookup unavailable")
	}
	if _, _, ok := table.LookupChecked(ops, Z80, SolverOptions{}); ok {
		t.Fatal("legacy admitted to compiler")
	}
	if err := table.AddChecked(ops, assignment, Z80, SolverOptions{}); err != nil {
		t.Fatal(err)
	}
	key, _ := CheckedTableKey(ops, Z80, SolverOptions{})
	if got, _, ok := table.LookupChecked(ops, Z80, SolverOptions{}); !ok || !reflect.DeepEqual(got, assignment) {
		t.Fatal("valid certificate missed")
	}
	path := t.TempDir() + "/table.json"
	if err := table.Save(path); err != nil {
		t.Fatal(err)
	}
	var loaded RegAllocTable
	loaded.Init()
	if err := loaded.Load(path); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := loaded.LookupChecked(ops, Z80, SolverOptions{}); !ok {
		t.Fatal("saved local certificate missed")
	}
	entry := loaded.entries[key]
	entry.Cost--
	if _, _, ok := loaded.LookupChecked(ops, Z80, SolverOptions{}); ok {
		t.Fatal("forged lower cost admitted")
	}
	entry.Cost++
	entry.Assignment[1] = 0
	if _, _, ok := loaded.LookupChecked(ops, Z80, SolverOptions{}); ok {
		t.Fatal("forged overlapping assignment admitted")
	}
	entry.Assignment[1] = 1
	entry.Schema = "future"
	if _, _, ok := loaded.LookupChecked(ops, Z80, SolverOptions{}); ok {
		t.Fatal("unknown schema admitted")
	}
	entry.Schema = checkedTableSchema
	entry.CostModel = "legacy"
	if _, _, ok := loaded.LookupChecked(ops, Z80, SolverOptions{}); ok {
		t.Fatal("wrong cost model admitted")
	}
	// The serialized packet is a real local generation artifact, not bundled research data.
	if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
		t.Fatal("empty serialized table")
	}
}

func TestCheckedLocalTableNativeBenefitIncludesPairSaves(t *testing.T) {
	ops := []VIROp{{Op: OpConst, Dst: 1, Width: 16, Imm: 0x1234}, {Op: OpMove, Dst: 2, Src: [2]int{1, -1}, Width: 16}, {Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 16}, {Op: OpRet, Src: [2]int{3, -1}, Width: 16}}
	var table RegAllocTable
	table.Init()
	var costs, cycles, sizes []int
	for _, middle := range []string{"IX", "DE"} {
		assignment := map[int]int{1: Z80.LocByName("BC"), 2: Z80.LocByName(middle), 3: Z80.LocByName("HL")}
		if err := table.AddChecked(ops, assignment, Z80, SolverOptions{}); err != nil {
			t.Fatal(err)
		}
		selected, cost, ok := table.LookupChecked(ops, Z80, SolverOptions{})
		if !ok {
			t.Fatal("generated table missed")
		}
		costs = append(costs, cost)
		vf := &Func{Name: "word", Blocks: []Block{{Ops: ops}}}
		asm, err := emitFromTable(&mir2.Func{Name: vf.Name}, vf, ops, selected, Z80, SolverOptions{})
		if err != nil {
			t.Fatal(err)
		}
		bin, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil || len(bin.Errors) > 0 {
			t.Fatalf("assemble %v %v", err, bin.Errors)
		}
		z := emulator.NewRemogattoZ80()
		z.MaxCycles = 1000
		if err := z.LoadMemory(0x8000, append([]byte{0xcd, 5, 0x80, 0xf3, 0x76}, bin.Binary...)); err != nil {
			t.Fatal(err)
		}
		z.SetPC(0x8000)
		z.SetSP(0xf000)
		if err := z.Run(); err != nil {
			t.Fatal(err)
		}
		r := z.GetRegisters()
		if r.HL != 0x1234 || r.SP != 0xf000 {
			t.Fatalf("native HL=%04x SP=%04x ASM=%s BIN=%x", r.HL, r.SP, asm, bin.Binary)
		}
		cycles = append(cycles, int(z.GetCycles()))
		sizes = append(sizes, len(bin.Binary))
		t.Logf("middle=%s current-cost=%d native-T=%d bytes=%d BIN=%x", middle, cost, z.GetCycles(), len(bin.Binary), bin.Binary)
	}
	if path := os.Getenv("VIR_CHECKED_TABLE_EXPORT"); path != "" {
		if err := table.Save(path); err != nil {
			t.Fatal(err)
		}
	}
	if costs[1] >= costs[0] || cycles[1] >= cycles[0] || sizes[1] >= sizes[0] {
		t.Fatalf("no measured local benefit cost=%v T=%v bytes=%v", costs, cycles, sizes)
	}
}

func TestCheckedTableAdmissionIsNotOnlyAHash(t *testing.T) {
	var table RegAllocTable
	table.Init()
	scratch := []VIROp{{Op: OpConst, Dst: 1, Width: 8, Imm: 37}, {Op: OpConst, Dst: 2, Width: 8, Imm: 5}, {Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 8}, {Op: OpRet, Src: [2]int{1, -1}, Width: 8}}
	if err := table.AddChecked(scratch, map[int]int{1: 0, 2: 5, 3: 14}, Z80, SolverOptions{}); err == nil {
		t.Fatal("hidden live accumulator clobber certified")
	}
	ops := tableAddOps()
	assignment := map[int]int{1: 0, 2: 1, 3: 0}
	if err := table.AddChecked(ops, assignment, Z80, SolverOptions{ParamLocs: map[int]int{1: 1}}); err == nil {
		t.Fatal("ABI mismatch certified")
	}
	if err := table.AddChecked(ops, assignment, Z80, SolverOptions{FuncParamLocs: map[string]map[int]int{"callee": {1: 1}}}); err == nil {
		t.Fatal("unresolved function ABI certified")
	}
	forbidden := cloneTableMachine(t)
	forbidden.Rules = append(forbidden.Rules, ConstraintRule{Name: "forbid add", OpMask: OpAdd, DstSet: Singleton(0), SrcSet: Singleton(1), Cost: MaxCost})
	if err := table.AddChecked(ops, assignment, forbidden, SolverOptions{}); err == nil {
		t.Fatal("forbidden operand combination certified")
	}
	wrongReturn := map[int]int{1: 0, 2: 1, 3: 1}
	if err := table.AddChecked([]VIROp{{Op: OpConst, Dst: 3, Width: 8, Imm: 5}, {Op: OpRet, Src: [2]int{3, -1}, Width: 8}}, map[int]int{3: wrongReturn[3]}, Z80, SolverOptions{}); err == nil {
		t.Fatal("wrong return register certified")
	}
	f := &mir2.Func{Name: "callee"}
	vf := &Func{Name: "callee", Blocks: []Block{{Ops: ops}}}
	if _, err := emitFromTable(f, vf, ops, assignment, Z80, SolverOptions{FuncParamLocs: map[string]map[int]int{"callee": {1: 1}}}); err == nil {
		t.Fatal("function-specific ABI mismatch admitted")
	}
	key, _ := CheckedTableKey(scratch, Z80, SolverOptions{})
	table.Add(key, &RegAllocEntry{Schema: checkedTableSchema, CostModel: checkedTableCostModel, Signature: key, NVregs: 3, NOps: 4, Assignment: []int{0, 5, 14}})
	if _, _, ok := table.LookupChecked(scratch, Z80, SolverOptions{}); ok {
		t.Fatal("matching key bypassed pattern scratch admission")
	}
}

func TestCheckedTableCompilerUsesGeneratedLocalCertificate(t *testing.T) {
	m := &mir2.Module{}
	f := m.AddFunc("local_add")
	b := f.NewBlock("entry")
	x, y, sum := f.AllocReg(), f.AllocReg(), f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: x, Imm: 7, Ty: mir2.TyU8})
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: y, Imm: 5, Ty: mir2.TyU8})
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: sum, Src: [2]mir2.Reg{x, y}, Ty: mir2.TyU8})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{sum}})
	vf, err := LowerFunc(f, Z80, m)
	if err != nil {
		t.Fatal(err)
	}
	var ops []VIROp
	for _, b := range vf.Blocks {
		ops = append(ops, b.Ops...)
	}
	assignment := map[int]int{}
	for _, op := range ops {
		if op.Dst > 0 {
			assignment[op.Dst] = 0
			if op.Op == OpConst && op.Imm == 5 {
				assignment[op.Dst] = 1
			}
		}
	}
	var table RegAllocTable
	table.Init()
	if err := table.AddChecked(ops, assignment, Z80, SolverOptions{}); err != nil {
		t.Fatalf("generation %v ops=%v", err, ops)
	}
	// Override the singleton only in this nonparallel test; force a nonexistent
	// solver so successful CodegenFunc proves the compiled checked hit was used.
	GetRegAllocTable()
	previous := globalTable
	globalTable = &table
	defer func() { globalTable = previous }()
	asm, err := CodegenFunc(f, m, SolverOptions{Z3Path: "/nonexistent/checked-table-z3"})
	if err != nil {
		t.Fatal(err)
	}
	bin, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, bin.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	if err := z.LoadMemory(0x8000, append([]byte{0xcd, 5, 0x80, 0xf3, 0x76}, bin.Binary...)); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if r := z.GetRegisters(); r.A != 12 || r.SP != 0xf000 {
		t.Fatalf("native A=%d SP=%04x", r.A, r.SP)
	}
	t.Logf("compiler checked hit, no solver: A=12 SP=f000 BIN=%x T=%d", bin.Binary, z.GetCycles())
}
