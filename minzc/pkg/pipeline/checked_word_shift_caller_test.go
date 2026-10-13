package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckedWordShiftCallerNative(t *testing.T) {
	for _, strategy := range []string{"greedy", "pbqp"} {
		for _, grace := range []bool{false, true} {
			for _, amount := range []int{1, 4} {
				source := fmt.Sprintf("fun shifted(x:u16)->u16{return x << %d}\nfun shift_caller(x:u16)->u16{return shifted(x)+x}", amount)
				opts := Options{OptSize: true, AssertMode: "none", ContractOpt: true, ContractStrategy: strategy, UseGrace: grace, UseHIRGrace: grace}
				base, err := CompileHIRSteps(graceMeasureParse(t, source), opts)
				if err != nil {
					t.Fatal(err)
				}
				problem, err := vir.PrepareLocalTableProblem(base.MIR2Module.FuncByName("shifted"), base.Allocation)
				if err != nil {
					t.Fatal(err)
				}
				var pool, target vir.RegAllocTable
				pool.Init()
				target.Init()
				// For long chains use a checked prefix plus the existing return bridge.
				cuts := []int{len(problem.Ops) - 2}
				assignment := map[int]int{}
				for _, op := range problem.Ops[:cuts[0]] {
					if op.Dst > 0 {
						assignment[op.Dst] = vir.Z80.LocByName("HL")
					}
					for _, v := range op.Src {
						if v > 0 {
							assignment[v] = vir.Z80.LocByName("HL")
						}
					}
				}
				for v, loc := range problem.Options.ParamLocs {
					assignment[v] = loc
				}
				if err = pool.AddChecked(problem.Ops[:cuts[0]], assignment, vir.Z80, problem.Options); err != nil {
					t.Fatal(err)
				}
				count, err := pool.ComposeCheckedAutomatically(problem, &target, 64)
				if err != nil || count == 0 {
					t.Fatal(amount, count, err)
				}
				path := t.TempDir() + "/shift.json"
				if err = target.Save(path); err != nil {
					t.Fatal(err)
				}
				opts.CheckedLocalTablePath = path
				selected, err := CompileHIRSteps(graceMeasureParse(t, source), opts)
				if err != nil {
					t.Fatal(err)
				}
				raw := mir2.NewVM(hir.LowerModule(graceMeasureParse(t, source)))
				externalBefore := checkedShiftExternalCaller(t, base, amount)
				externalAfter := checkedShiftExternalCaller(t, selected, amount)
				if externalAfter >= externalBefore {
					t.Fatal("external CALL regressed", externalBefore, externalAfter)
				}
				t.Logf("external CALL keeps x: shift%d %d -> %d T", amount, externalBefore, externalAfter)
				before := checkedWordNative(t, base, "shift_caller", (1<<amount)+1, raw)
				after := checkedWordNative(t, selected, "shift_caller", (1<<amount)+1, raw)
				b, err := checkedAsmSize(base.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				s, err := checkedAsmSize(selected.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				if s > b || after > before {
					t.Fatal("selected code regressed", amount, b, s, before, after)
				}
				if directory := os.Getenv("CHECKED_SHIFT_CALLER_EXPORT"); directory != "" {
					directory = filepath.Join(directory, fmt.Sprintf("%s-grace%v-shift%d", strategy, grace, amount))
					if err = os.MkdirAll(directory, 0755); err != nil {
						t.Fatal(err)
					}
					for name, data := range map[string]string{"source.nanz": source, "baseline.a80": base.Assembly, "selected.a80": selected.Assembly} {
						if err = os.WriteFile(filepath.Join(directory, name), []byte(data), 0644); err != nil {
							t.Fatal(err)
						}
					}
					if err = pool.Save(filepath.Join(directory, "pool.json")); err != nil {
						t.Fatal(err)
					}
					if err = target.Save(filepath.Join(directory, "current.json")); err != nil {
						t.Fatal(err)
					}
				}
				t.Logf("strategy=%s grace=%v", strategy, grace)
				t.Logf("caller shift%d all65536: %d -> %d bytes, %d -> %d T; backend=%s", amount, b, s, before, after, selected.Traces["shifted"].Backend)
			}
		}
	}
}

func checkedShiftExternalCaller(t *testing.T, steps Steps, amount int) int {
	t.Helper()
	binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(binary.Errors) > 0 {
		t.Fatal(err, binary.Errors)
	}
	function := steps.MIR2Module.FuncByName("shifted")
	register := steps.Allocation.Locs[function.Contract.Params[0].Reg].Name
	if register != "DE" && register != "HL" && register != "BC" {
		t.Fatal("unsupported ABI", register)
	}
	entry := binary.Symbols["shifted"]
	wrapper, err := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0xf100\nPUSH %s\nCALL %d\nPOP BC\nADD HL,BC\nRET\n", register, entry))
	if err != nil || len(wrapper.Errors) > 0 {
		t.Fatal(err, wrapper.Errors)
	}
	if directory := os.Getenv("CHECKED_SHIFT_CALLER_EXPORT"); directory != "" {
		directory = filepath.Join(directory, "external", fmt.Sprintf("shift%d", amount), steps.Traces["shifted"].Backend)
		if err = os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"wrapper.bin": wrapper.Binary, "module.bin": binary.Binary, "wrapper.a80": []byte(fmt.Sprintf("ORG 0xf100\nPUSH %s\nCALL %d\nPOP BC\nADD HL,BC\nRET\n", register, entry))} {
			if err = os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	z := emulator.NewRemogattoZ80()
	if err = z.LoadMemory(0x8000, binary.Binary); err != nil {
		t.Fatal(err)
	}
	if err = z.LoadMemory(0xf100, wrapper.Binary); err != nil {
		t.Fatal(err)
	}
	if err = z.LoadMemory(0xeffe, []byte{0, 0xf2}); err != nil {
		t.Fatal(err)
	}
	cost := -1
	for x := 0; x < 65536; x++ {
		regs := emulator.Registers{HL: 0x1357, DE: 0x2468, BC: 0x369a, PC: 0xf100, SP: 0xeffe}
		switch register {
		case "DE":
			regs.DE = uint16(x)
		case "HL":
			regs.HL = uint16(x)
		case "BC":
			regs.BC = uint16(x)
		}
		z.SetRegisters(regs)
		before := z.GetCycles()
		for n := 0; n < 1024 && z.GetPC() != 0xf200; n++ {
			z.Step()
		}
		got := z.GetRegisters()
		want := uint16(x * ((1 << amount) + 1))
		if got.HL != want || got.PC != 0xf200 || got.SP != 0xf000 {
			t.Fatal("real CALL", amount, x, want, got)
		}
		cycles := z.GetCycles() - before
		if cost < 0 {
			cost = cycles
		} else if cost != cycles {
			t.Fatal("variable caller cost", cycles, cost)
		}
	}
	return cost
}
