package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestCompositionExactHashNative(t *testing.T) {
	dir := os.Getenv("COMPOSITION_EXPORT")
	if dir == "" {
		t.Skip("set COMPOSITION_EXPORT")
	}
	table := &vir.RegAllocTable{}
	table.Init()
	if e := table.Load(dir + "/hash3-proposal-table.json"); e != nil {
		t.Fatal(e)
	}
	src, e := os.ReadFile("../../../examples/nanz/20_strref_test.nanz")
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	base, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
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
	for index, asm := range []string{base.Assembly, nativeCandidate} {
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		if len(bin.Binary) != 45 {
			t.Fatal("module changed")
		}
		label := []string{"baseline", "candidate"}[index]
		for suffix, data := range map[string][]byte{"a80": []byte(asm), "bin": bin.Binary} {
			if err := os.WriteFile(filepath.Join(dir, "hash3-"+label+"."+suffix), data, 0644); err != nil {
				t.Fatal(err)
			}
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
	for name, value := range map[string]any{
		"allocation.json": base.Allocation,
		"options.json":    opts,
		"result.json":     map[string]any{"tuples_per_path": 65536, "paths": 2, "module_bytes": 45, "function_bytes": 6, "call_t_states": cost, "native_exact_pc_sp": true, "code_integrity": true, "vm_math_oracle": true},
	} {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("exact hash3 imported proposal equal6B30T/fullmodule45B;65536tuples perpath")
}
