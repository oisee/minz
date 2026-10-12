package hir_test

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// Frozen before execution: unsigned zero/one, sign-bit transition and upper
// wrap boundaries. Exhaust both a/b for every selected c, and c/d pair for a2.
// This extends the retained LIR composition value oracles; it is not a complete
// 3/4-argument Cartesian or a full memory/stack-effect oracle.
func TestNativeCompositionAdditionalArgumentBoundaries(t *testing.T) {
	boundaries := []int64{0, 1, 127, 128, 254, 255}
	for _, fixture := range nativeCompositionFixtures() {
		if fixture.arity < 3 {
			continue
		}
		fixture := fixture
		for _, mode := range []string{"ordinary", "size", "speed"} {
			t.Run(fixture.name+"/"+mode, func(t *testing.T) {
				t.Parallel()
				hm, err := nanz.Parse(fixture.src, "critic-boundary.nanz")
				if err != nil {
					t.Fatal(err)
				}
				for _, c := range boundaries {
					domainD := []int64{0}
					if fixture.arity == 4 {
						domainD = boundaries
					}
					for _, d := range domainD {
						t.Run(fmt.Sprintf("c%d_d%d", c, d), func(t *testing.T) {
							var completed atomic.Int64
							inputs := func(i int) []int64 {
								completed.Add(1)
								return []int64{int64(i & 255), int64(i >> 8), c, d}[:fixture.arity]
							}
							opts := pipeline.DefaultOptions()
							opts.UseHIRGrace = mode != "ordinary"
							opts.OptSize = mode == "size"
							bad, first := sweepCodegenJudgeOptions(t, hm.Funcs, fixture.name, 65536, inputs, fixture.model, opts, true)
							if completed.Load() != 65536 {
								t.Fatalf("executed %d/65536 cases", completed.Load())
							}
							if bad != 0 {
								t.Fatalf("native %d/65536 mismatches (%s)", bad, first)
							}
						})
					}
				}
			})
		}
	}
}

func sweepCodegenJudgeOptions(t *testing.T, funcs []*hir.Func, name string, cases int, inputs func(int) []int64, model func([]int64) int64, opts pipeline.Options, checkReturn bool) (int, string) {
	t.Helper()
	steps, err := pipeline.CompileHIRSteps(&hir.Module{Name: "allowlist_judge", Funcs: funcs}, opts)
	if err != nil {
		t.Fatal(err)
	}
	mf := steps.MIR2Module.FuncByName(name)
	if len(mf.Blocks) != 1 {
		t.Fatalf("%s must exercise the single-block bridge", name)
	}
	boot := fmt.Sprintf("ORG 0x%04X\n CALL %s\n DI\n HALT\n", testLoadAddr, name)
	res, err := z80asm.NewAssembler().AssembleString(boot + steps.Assembly)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, res.Errors)
	}
	// Validate the ABI before starting workers: Fatal must run on the test goroutine.
	for _, p := range mf.Contract.Params {
		if p.Ty.Width() == 16 {
			loc := steps.Allocation.Locs[p.Reg]
			if loc.Name != "HL" && loc.Name != "DE" && loc.Name != "BC" {
				t.Fatalf("word ABI: %+v", loc)
			}
		}
	}
	results := make([]string, cases)
	judgeWorkers(cases, res.Binary, func(z *emulator.RemogattoZ80, image []byte, i int) {
		args := inputs(i)
		z.Reset()
		if err := z.LoadMemory(testLoadAddr, image); err != nil {
			results[i] = fmt.Sprintf("load image: %v", err)
			return
		}
		regs := emulator.Registers{PC: testLoadAddr, SP: 0xFF00}
		for k, p := range mf.Contract.Params {
			if p.Ty.Width() != 16 {
				continue
			}
			loc := steps.Allocation.Locs[p.Reg]
			switch loc.Name {
			case "HL":
				regs.HL = uint16(args[k])
			case "DE":
				regs.DE = uint16(args[k])
			case "BC":
				regs.BC = uint16(args[k])
			}
		}
		z.SetRegisters(regs)
		for k, p := range mf.Contract.Params {
			if p.Ty.Width() == 8 {
				if err := z.SetRegister8(steps.Allocation.Locs[p.Reg].Name, uint8(args[k])); err != nil {
					results[i] = fmt.Sprintf("byte ABI: %v", err)
					return
				}
			}
		}
		n, returned := compositionRunToReturn(z, testLoadAddr+3, 0xFF00, judgeStepBudget)
		if checkReturn && !returned {
			state := z.GetRegisters()
			results[i] = fmt.Sprintf("%s%v bad return PC=%04x SP=%04x halted=%v", name, args, state.PC, state.SP, z.IsHalted())
			return
		}
		got, err := hirReturnValue(mf, z.GetRegisters())
		want := model(args)
		if err != nil || n == judgeStepBudget || got != want {
			results[i] = fmt.Sprintf("%s%v got %d want %d err %v steps %d", name, args, got, want, err, n)
		}
	})
	bad, first := 0, ""
	for _, result := range results {
		if result != "" {
			bad++
			if first == "" {
				first = result
			}
		}
	}
	return bad, first
}

// Observe the original CALL continuation before executing its DI/HALT bytes.
func compositionRunToReturn(z *emulator.RemogattoZ80, pc, sp uint16, budget int) (int, bool) {
	for n := 0; n < budget; n++ {
		state := z.GetRegisters()
		if n > 0 && state.PC == pc && state.SP == sp {
			return n, true
		}
		if z.IsHalted() {
			return n, false
		}
		z.Step()
	}
	return budget, false
}

func TestCompositionReturnOracleRejectsWrongMachineCode(t *testing.T) {
	cases := []struct {
		name, body string
		want       bool
	}{
		{"valid", "LD A, 42\n RET", true},
		{"halt_in_function", "LD A, 42\n HALT", false},
		{"skip_di", "POP HL\n INC HL\n JP (HL)", false},
		{"bad_sp", "POP HL\n DEC SP\n JP (HL)", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			asm := fmt.Sprintf("ORG 0x%04x\n CALL probe\n DI\n HALT\nprobe:\n%s\n", testLoadAddr, tc.body)
			res, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(res.Errors) > 0 {
				t.Fatalf("assemble %v %v", err, res.Errors)
			}
			z := emulator.NewRemogattoZ80()
			if err := z.LoadMemory(testLoadAddr, res.Binary); err != nil {
				t.Fatal(err)
			}
			z.SetRegisters(emulator.Registers{PC: testLoadAddr, SP: 0xFF00})
			_, got := compositionRunToReturn(z, testLoadAddr+3, 0xFF00, 100)
			if got != tc.want {
				t.Fatalf("return oracle accepted=%v want=%v", got, tc.want)
			}
		})
	}
}
