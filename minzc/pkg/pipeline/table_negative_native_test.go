package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

func TestCheckedStrRefEqualNative(t *testing.T) {
	solver := checkedRealSolver(t)
	src, e := os.ReadFile("../../../examples/nanz/20_strref_test.nanz")
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	base, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
	if e != nil {
		t.Fatal(e)
	}
	table, _, e := GenerateCheckedLocalTables(graceMeasureParse(t, string(src)), opts, vir.SolverOptions{Z3Path: solver})
	if e != nil {
		t.Fatal(e)
	}
	p, e := vir.PrepareLocalTableProblem(base.MIR2Module.FuncByName("hash3"), base.Allocation)
	if e != nil {
		t.Fatal(e)
	}
	candidate, e := p.Emit(table)
	if e != nil {
		t.Fatal(e)
	}
	start, end, ok := checkedFunctionSpan(base.Assembly, "hash3")
	if !ok {
		t.Fatal("span")
	}
	nativeCandidate := base.Assembly[:start] + candidate + base.Assembly[end:]
	var cost int
	for _, asm := range []string{base.Assembly, nativeCandidate} {
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		if len(bin.Binary) != 45 {
			t.Fatal("module changed")
		}
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(0x8000, bin.Binary)
		z.LoadMemory(0xeffe, []byte{0, 0xf1})
		f := base.MIR2Module.FuncByName("hash3")
		vm := mir2.NewVM(base.MIR2Module)
		for x := 0; x < 65536; x++ {
			a, b, c := byte(x), byte(x>>8), byte(x*53+17)
			want := byte(int(a) + 2*int(b) + 3*int(c))
			r := emulator.Registers{A: 0x77, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(bin.Symbols["hash3"]), SP: 0xeffe}
			z.SetRegisters(r)
			for i, v := range []byte{a, b, c} {
				if e = z.SetRegister8(base.Allocation.Locs[f.Contract.Params[i].Reg].Name, v); e != nil {
					t.Fatal(e)
				}
			}
			before := z.GetCycles()
			for i := 0; i < 32 && z.GetRegisters().PC != 0xf100; i++ {
				z.Step()
			}
			got := z.GetRegisters()
			if got.A != want || got.SP != 0xf000 || got.PC != 0xf100 {
				t.Fatal(x, want, got)
			}
			if cost == 0 {
				cost = z.GetCycles() - before
			} else if z.GetCycles()-before != cost {
				t.Fatal("cost changed")
			}
			vr, e := vm.Call("hash3", []mir2.Value{{I: int64(a)}, {I: int64(b)}, {I: int64(c)}})
			if e != nil || len(vr) != 1 || byte(vr[0].I) != want {
				t.Fatal(x, vr, e)
			}
		}
		for i, b := range bin.Binary {
			if z.GetMemory(uint16(0x8000+i)) != b {
				t.Fatal("code mutation")
			}
		}
	}
	t.Logf("StrRef hash3 baseline/candidate equal:6B30T; fullmodule45B; 65536 tuples per path VM/native independent modulo oracle PASS (not all2^24)")
}

func TestCheckedExactBindingMissAndRevalidatedTransfer(t *testing.T) {
	solver := checkedRealSolver(t)
	src, e := os.ReadFile("../../../examples/nanz/16_rotate_sled.nanz")
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	prepare := func(s string) (*vir.LocalTableProblem, Steps) {
		steps, e := CompileHIRSteps(graceMeasureParse(t, s), opts)
		if e != nil {
			t.Fatal(e)
		}
		p, e := vir.PrepareLocalTableProblem(steps.MIR2Module.FuncByName("low_nibble"), steps.Allocation)
		if e != nil {
			t.Fatal(e)
		}
		return p, steps
	}
	p, _ := prepare(string(src))
	table := &vir.RegAllocTable{}
	table.Init()
	if e = p.Generate(table, vir.SolverOptions{Z3Path: solver}); e != nil {
		t.Fatal(e)
	}
	assignment, _, hit := table.LookupChecked(p.Ops, vir.Z80, p.Options)
	if !hit {
		t.Fatal("own table miss")
	}
	changed := strings.Replace(string(src), "return x & 0x0F", "return x & 0x07", 1)
	q, steps := prepare(changed)
	originalKey, _ := vir.CheckedTableKey(p.Ops, vir.Z80, p.Options)
	changedKey, _ := vir.CheckedTableKey(q.Ops, vir.Z80, q.Options)
	_, _, hit = table.LookupChecked(q.Ops, vir.Z80, q.Options)
	if hit || originalKey == changedKey {
		t.Fatal("immediate/context not bound")
	}
	legacySame := vir.ComputeSignature(p.Ops, vir.Z80) == vir.ComputeSignature(q.Ops, vir.Z80)
	transferred := &vir.RegAllocTable{}
	transferred.Init()
	if e = transferred.AddChecked(q.Ops, assignment, vir.Z80, q.Options); e != nil {
		t.Fatal("independently checked old assignment", e)
	}
	asm, e := q.Emit(transferred)
	if e != nil {
		t.Fatal(e)
	}
	start, end, ok := checkedFunctionSpan(steps.Assembly, "low_nibble")
	if !ok {
		t.Fatal("span")
	}
	before, e := checkedAsmSize(steps.Assembly[start:end])
	if e != nil {
		t.Fatal(e)
	}
	after, e := checkedAsmSize(asm)
	if e != nil {
		t.Fatal(e)
	}
	if before != after {
		t.Fatal("expected no gain")
	}
	for x := 0; x < 256; x++ {
		runCheckedRealByte(t, asm, q.Function, steps.Allocation, byte(x), byte(x)&7)
	}
	// Rebinding a byte parameter to C cannot reuse the assignment pinned to A.
	f := q.Function
	param := f.Contract.Params[0]
	f.Contract.Params[0].Class = mir2.ClassRegC
	for _, b := range f.Blocks {
		for i := range b.Params {
			if b.Params[i].Dst == param.Reg {
				b.Params[i].Class = mir2.ClassRegC
			}
		}
	}
	steps.Allocation.Locs[param.Reg] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "C"}
	abi, e := vir.PrepareLocalTableProblem(f, steps.Allocation)
	if e != nil {
		t.Fatal(e)
	}
	_, _, hit = transferred.LookupChecked(abi.Ops, vir.Z80, abi.Options)
	if hit {
		t.Fatal("ABI changed hit")
	}
	rejected := &vir.RegAllocTable{}
	rejected.Init()
	abiError := rejected.AddChecked(abi.Ops, assignment, vir.Z80, abi.Options)
	if abiError == nil {
		t.Fatal("ABI old assignment admitted")
	}
	t.Logf("real low_nibble altered immediate: legacyShapeSame=%v exactMiss=true targetAddCheckedPASS equal3B; ABI A→C oldassignment refused: %v", legacySame, abiError)
	if dir := os.Getenv("TABLE_NEGATIVE_EXPORT"); dir != "" {
		data, _ := json.MarshalIndent(map[string]any{"legacy_shape_same": legacySame, "original_key": originalKey, "changed_key": changedKey, "target_bytes": after, "abi_rejection": abiError.Error(), "altered_source_probe": true}, "", "  ")
		os.WriteFile(dir+"/binding-transfer.json", data, 0644)
		os.WriteFile(dir+"/changed-immediate.nanz", []byte(changed), 0644)
	}
}
