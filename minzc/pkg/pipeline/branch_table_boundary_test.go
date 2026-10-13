package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Oracle: Language Book v7 §2.5 control flow and §3.1 u8/bool; §3.2 unsigned comparisons.
func TestCheckedBranchCorpusLeafNativeAllByteInputs(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/17_bool_retflag.nanz")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source) + "\nfun digit_live_caller(c:u8)->u8 { if is_digit_b(c) { return c+1 } return c }\n"
	s, err := CompileHIRSteps(graceMeasureParse(t, text), Options{ContractOpt: true, OptSize: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	f := s.MIR2Module.FuncByName("is_digit_b")
	if _, err := vir.PrepareLocalTableProblem(f, s.Allocation); err == nil {
		t.Fatal("branch function admitted by linear certificate")
	}
	t.Log("baseline MIR", (&mir2.Module{Funcs: []*mir2.Func{f, s.MIR2Module.FuncByName("digit_live_caller")}}).Dump())
	bin, err := z80asm.NewAssembler().AssembleString(s.Assembly)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatal(err, bin.Errors)
	}
	totals := map[string]int{}
	vm := mir2.NewVM(s.MIR2Module)
	for x := 0; x < 256; x++ {
		digit := x >= 48 && x <= 57
		for _, name := range []string{"is_digit_b", "digit_live_caller"} {
			fn := s.MIR2Module.FuncByName(name)
			want := 0
			if digit {
				want = 1
			}
			if name == "digit_live_caller" {
				want = x
				if digit {
					want++
				}
			}
			values, err := vm.Call(name, []mir2.Value{{I: int64(x)}})
			if err != nil || len(values) != 1 || values[0].I != int64(want) {
				t.Fatalf("VM %s x=%d want%d values%v err%v", name, x, want, values, err)
			}
			address, ok := bin.Symbols[name]
			if !ok {
				address, ok = bin.Symbols[strings.ToUpper(name)]
			}
			if !ok {
				t.Fatal("missing symbol", name)
			}
			loc := s.Allocation.Locs[fn.Contract.Params[0].Reg]
			if loc.Kind != mir2.LocReg {
				t.Fatal("unexpected input storage", loc)
			}
			boot, err := z80asm.NewAssembler().AssembleString(fmt.Sprintf("LD %s,%d\nCALL %d\nDI\nHALT\n", loc.Name, x, address))
			if err != nil || len(boot.Errors) > 0 {
				t.Fatal(err, boot.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 100000
			z.LoadMemory(uint16(bin.Origin), bin.Binary)
			z.LoadMemory(0xf100, boot.Binary)
			z.SetPC(0xf100)
			z.SetSP(0xf000)
			if err := z.Run(); err != nil {
				t.Fatal(err)
			}
			r := z.GetRegisters()
			ret := mir2.ReturnLocation(fn.Contract.Returns[0].Class, fn.Contract.Returns[0].Ty)
			got := 0
			switch ret {
			case "A":
				got = int(r.A)
			case "B":
				got = int(r.BC >> 8)
			case "C":
				got = int(byte(r.BC))
			case "D":
				got = int(r.DE >> 8)
			case "E":
				got = int(byte(r.DE))
			case "F":
				yes := false
				switch fn.Contract.Returns[0].FlagCond {
				case mir2.CmpEq:
					yes = r.F&0x40 != 0
				case mir2.CmpNe:
					yes = r.F&0x40 == 0
				case mir2.CmpLt, mir2.CmpUlt:
					yes = r.F&1 != 0
				case mir2.CmpGe, mir2.CmpUge:
					yes = r.F&1 == 0
				default:
					t.Fatal("uncovered flag convention", fn.Contract.Returns[0])
				}
				if yes {
					got = 1
				}
			default:
				t.Fatal("uncovered return", ret)
			}
			if got != want || r.SP != 0xf000 {
				t.Fatalf("native %s x=%d %s got%d want%d F=%02x SP=%04x", name, x, ret, got, want, r.F, r.SP)
			}
			totals[name] += int(z.GetCycles())
		}
	}
	t.Logf("all256 independent predicate and live-caller VM/Z80 PASS; totals including input/CALL/RET/DI/HALT=%v", totals)
	if out := os.Getenv("MINZ_BRANCH_PROOF_ARTIFACTS"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"source.nanz": []byte(text), "baseline.a80": []byte(s.Assembly), "baseline.bin": bin.Binary, "baseline.mir": []byte(s.MIR2Module.Dump())} {
			if err := os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
