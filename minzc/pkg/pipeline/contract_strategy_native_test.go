package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pfccoPinnedSource = `global trace:u8=0
fun twice(@z80_c x:u8)->u8 {
 asm z80 { ; fixed C ABI marker }
 trace=trace+1
 return x+x
}
fun offset(x:u8)->u8 {
 trace=trace+1
 return x+x+3
}
fun multi(x:u8)->u8 {
 var a:u8=twice(x)
 if x!=0 { return a+offset(x) }
 return a
}`

func TestContractStrategyPinnedNativeSource(t *testing.T) {
	runContractStrategyPinnedSource(t, pfccoPinnedSource)
}
func TestContractStrategyLiteralRHSNativeSource(t *testing.T) {
	runContractStrategyPinnedSource(t, strings.Replace(pfccoPinnedSource, "return x+x+3", "return 3+x", 1))
}
func runContractStrategyPinnedSource(t *testing.T, source string) {
	var baselineT uint64
	var baselineBytes int
	for _, strategy := range []string{"", "pbqp"} {
		h := graceMeasureParse(t, source)
		s, err := CompileHIRSteps(h, Options{ContractOpt: true, ContractStrategy: strategy, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		f := s.MIR2Module.FuncByName("twice")
		if f == nil || f.Contract.Params[0].Class != mir2.ClassRegC || s.Allocation.Locs[f.Contract.Params[0].Reg].Name != "C" {
			t.Fatal("pin changed")
		}
		if !strings.Contains(s.Assembly, "CALL twice") || !strings.Contains(s.Assembly, "CALL offset") {
			t.Fatal("real calls lost")
		}

		vm := mir2.NewVM(s.MIR2Module)
		vm.WriteHeapSlice(vm.GlobalAddr("trace"), []byte{0})
		bin, err := z80asm.NewAssembler().AssembleString("ORG 32768\n" + s.Assembly)
		if err != nil || len(bin.Errors) > 0 {
			t.Fatal(err, bin.Errors)
		}
		z := emulator.NewRemogattoZ80()
		input := s.Allocation.Locs[s.MIR2Module.FuncByName("multi").Contract.Params[0].Reg].Name
		addr := bin.Symbols["multi"]
		if addr == 0 {
			addr = bin.Symbols["MULTI"]
		}
		stateAddr := bin.Symbols["trace"]
		if stateAddr == 0 {
			stateAddr = bin.Symbols["TRACE"]
		}
		if stateAddr == 0 {
			t.Fatal("global symbol missing")
		}
		trace := byte(0)
		nativeTrace := byte(0)
		var total uint64
		for x := 0; x < 256; x++ {
			want := byte(4*x + 3)
			if strings.Contains(source, "return 3+x") {
				want = byte(3*x + 3)
			}
			trace++
			if x == 0 {
				want = 0
			} else {
				trace++
			}
			r, e := vm.Call("multi", []mir2.Value{{I: int64(x)}})
			if e != nil || byte(r[0].I) != want || vm.ReadHeap(vm.GlobalAddr("trace"), 1)[0] != trace {
				t.Fatal("independent MIR oracle", x, e)
			}
			boot, err := z80asm.NewAssembler().AssembleString(fmt.Sprintf("LD %s,%d\nCALL %d\nDI\nHALT\n", input, x, addr))
			if err != nil || len(boot.Errors) > 0 {
				t.Fatal(err, boot.Errors)
			}
			z = emulator.NewRemogattoZ80()
			z.MaxCycles = 1000000
			z.LoadMemory(32768, bin.Binary)
			z.SetMemory(uint16(stateAddr), nativeTrace)
			z.LoadMemory(0xf100, boot.Binary)
			z.SetPC(0xf100)
			z.SetSP(0xf000)
			before := z.GetCycles()
			if err := z.Run(); err != nil {
				t.Fatal(err)
			}
			total += uint64(z.GetCycles() - before)
			nativeTrace = z.GetMemory(uint16(stateAddr))
			regs := z.GetRegisters()
			if regs.A != want || regs.SP != 0xf000 || z.GetMemory(uint16(stateAddr)) != trace {
				t.Fatalf("native x%d want%d registers%+v trace=%d expected=%d symbol=%d ASM=%s", x, want, regs, z.GetMemory(uint16(stateAddr)), trace, stateAddr, s.Assembly)
			}
		}
		if strategy == "" {
			baselineT = total
			baselineBytes = len(bin.Binary)
		} else if total >= baselineT || len(bin.Binary) >= baselineBytes {
			t.Fatal("fixture lost measured native benefit")
		}
		if dir := os.Getenv("MINZ_PFCCO_NATIVE_EVIDENCE"); dir != "" {
			name := strategy
			if name == "" {
				name = "greedy"
			}
			prefix := "pinned"
			if strings.Contains(source, "return 3+x") {
				prefix = "literal-rhs"
			}
			for file, data := range map[string][]byte{prefix + "-source.nanz": []byte(source), prefix + "-" + name + ".a80": []byte(s.Assembly), prefix + "-" + name + ".bin": bin.Binary, prefix + "-" + name + "-metrics.txt": []byte(fmt.Sprintf("%d T / %d B\n", total, len(bin.Binary)))} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, file), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		t.Logf("strategy=%q full-source %d T/%d B", strategy, total, len(bin.Binary))
	}
}
