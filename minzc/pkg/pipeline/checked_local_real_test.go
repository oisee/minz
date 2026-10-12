package pipeline

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

func checkedRealSolver(t *testing.T) string {
	t.Helper()
	p := os.Getenv("VIR_TEST_Z3")
	if p == "" {
		t.Skip("set VIR_TEST_Z3 for offline local solver integration")
	}
	return p
}

func runCheckedRealByte(t *testing.T, asm string, f *mir2.Func, allocation *mir2.AllocResult, x byte, want byte) int {
	t.Helper()
	bin, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatal(err, bin.Errors)
	}
	addr, ok := bin.Symbols[f.Name]
	if !ok {
		addr, ok = bin.Symbols[strings.ToUpper(f.Name)]
	}
	if !ok {
		t.Fatal("missing symbol", f.Name)
	}
	loc := allocation.Locs[f.Contract.Params[0].Reg].Name
	setup := "LD " + loc + ", " + strconv.Itoa(int(x)) + "\nCALL " + strconv.Itoa(addr) + "\nDI\nHALT\n"
	boot, err := z80asm.NewAssembler().AssembleString(setup)
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
	memory := z.Memory()
	if !bytes.Equal(memory[bin.Origin:bin.Origin+len(bin.Binary)], bin.Binary) {
		t.Fatal("native leaf/caller changed compiled code")
	}
	ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
	var value byte
	switch ret {
	case "A":
		value = r.A
	case "B":
		value = byte(r.BC >> 8)
	case "C":
		value = byte(r.BC)
	case "D":
		value = byte(r.DE >> 8)
	case "E":
		value = byte(r.DE)
	default:
		t.Fatal("unsupported byte ABI", ret)
	}
	if value != want || r.SP != 0xf000 || !z.IsHalted() || r.PC != uint16(0xf100+len(boot.Binary)-1) {
		t.Fatalf("%s x=%d %s=%d want=%d SP=%04x PC=%04x halted=%v", f.Name, x, ret, value, want, r.SP, r.PC, z.IsHalted())
	}
	return int(z.GetCycles())
}

func TestCheckedRealWeightedSolverAndNativeCallers(t *testing.T) {
	solver := checkedRealSolver(t)
	source, err := os.ReadFile("../../../examples/nanz/06_pbqp_weighted.nanz")
	if err != nil {
		t.Fatal(err)
	}
	// Exact corpus helper plus an explicit harness that keeps x live over CALL.
	text := string(source) + "\nfun weighted_caller(x: u8) -> u8 { return weighted(x) + x }\n"
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	baseline, err := CompileHIRSteps(graceMeasureParse(t, text), opts)
	if err != nil {
		t.Fatal(err)
	}
	table, notes, err := GenerateCheckedLocalTables(graceMeasureParse(t, text), opts, vir.SolverOptions{Z3Path: solver, Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/real.json"
	if err := table.Save(path); err != nil {
		t.Fatal(err)
	}
	opts.CheckedLocalTablePath = path
	selected, err := CompileHIRSteps(graceMeasureParse(t, text), opts)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Traces["weighted"].Backend != "checked-local-table" || selected.Traces["weighted_caller"].Backend != "PBQP" {
		t.Fatal("wrong leaf/caller selection", notes)
	}
	before, err := checkedAsmSize(baseline.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	after, err := checkedAsmSize(selected.Assembly)
	if err != nil || after >= before {
		t.Fatal("no measured byte benefit", before, after, err)
	}
	vm := mir2.NewVM(selected.MIR2Module)
	var helperBefore, helperAfter, callerBefore, callerAfter int
	for x := 0; x < 256; x++ {
		for _, name := range []string{"weighted", "weighted_caller"} {
			factor := 10
			if name == "weighted_caller" {
				factor = 11
			}
			want := byte(factor*x + 1)
			values, err := vm.Call(name, []mir2.Value{{I: int64(x)}})
			if err != nil || len(values) != 1 || byte(values[0].I) != want {
				t.Fatal("VM oracle", name, x, values, err)
			}
			b := runCheckedRealByte(t, baseline.Assembly, baseline.MIR2Module.FuncByName(name), baseline.Allocation, byte(x), want)
			a := runCheckedRealByte(t, selected.Assembly, selected.MIR2Module.FuncByName(name), selected.Allocation, byte(x), want)
			if name == "weighted" {
				helperBefore, helperAfter = b, a
			} else {
				callerBefore, callerAfter = b, a
			}
		}
	}
	if helperAfter >= helperBefore || callerAfter >= callerBefore {
		t.Fatal("no native timing benefit", helperBefore, helperAfter, callerBefore, callerAfter)
	}
	t.Logf("weighted exact corpus + caller harness all 256 inputs: module %d→%d B helper %d→%d T caller %d→%d T; actual ABI/SP verified; generation %v", before, after, helperBefore, helperAfter, callerBefore, callerAfter, notes)
	if dir := os.Getenv("VIR_REAL_TABLE_EXPORT"); dir != "" {
		for name, data := range map[string][]byte{"corpus.nanz": source, "caller-harness.nanz": []byte(text), "caller-baseline.a80": []byte(baseline.Assembly), "caller-selected.a80": []byte(selected.Assembly)} {
			if err := os.WriteFile(dir+"/"+name, data, 0644); err != nil {
				t.Fatal(err)
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/caller-table.json", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckedRealAlreadyOptimalLeafKeepsBaseline(t *testing.T) {
	solver := checkedRealSolver(t)
	source, err := os.ReadFile("../../../examples/nanz/16_rotate_sled.nanz")
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	baseline, err := CompileHIRSteps(graceMeasureParse(t, string(source)), opts)
	if err != nil {
		t.Fatal(err)
	}
	table, notes, err := GenerateCheckedLocalTables(graceMeasureParse(t, string(source)), opts, vir.SolverOptions{Z3Path: solver, Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/rotate.json"
	if err := table.Save(path); err != nil {
		t.Fatal(err)
	}
	opts.CheckedLocalTablePath = path
	selected, err := CompileHIRSteps(graceMeasureParse(t, string(source)), opts)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Assembly != baseline.Assembly {
		t.Fatal("equal size leaf changed baseline")
	}
	for x := 0; x < 256; x++ {
		runCheckedRealByte(t, selected.Assembly, selected.MIR2Module.FuncByName("low_nibble"), selected.Allocation, byte(x), byte(x)&15)
	}
	t.Logf("low_nibble corpus exact equal-size candidate retained PBQP all 256 inputs PASS; %v", notes)
}

func TestOfflineLocalGenerationRefusesIncompatibleOptions(t *testing.T) {
	for _, options := range []Options{{}, {OptSize: true, SkipZ80Emission: true}, {OptSize: true, Backend: "ez80"}, {OptSize: true, CheckedLocalTablePath: "input.json"}, {OptSize: true, ProfileSelection: &ProfileSelection{}}} {
		if _, _, err := GenerateCheckedLocalTables(graceMeasureParse(t, "fun leaf() -> u8 { return 1 }"), options, vir.SolverOptions{}); err == nil {
			t.Fatal("incompatible generation admitted", options)
		}
	}
}
