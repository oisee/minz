package pipeline

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

const checkedTableSource = `fun add5(x: u8) -> u8 { return x + 5 }
fun main() -> u8 { return add5(7) }`

func localTableFixture(t *testing.T) (Steps, string) {
	t.Helper()
	// Table evidence and its refusing fallback use the exact same native policy.
	steps, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), Options{OptSize: true, AssertMode: "none", DisableVariantSearch: true})
	if err != nil {
		t.Fatal(err)
	}
	f := steps.MIR2Module.FuncByName("add5")
	p, err := vir.PrepareLocalTableProblem(f, steps.Allocation)
	if err != nil {
		t.Fatal(err)
	}
	assignment := map[int]int{}
	for v, loc := range p.Options.ParamLocs {
		assignment[v] = loc
	}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			assignment[op.Dst] = vir.Z80.LocByName("A")
		}
	}
	var table vir.RegAllocTable
	table.Init()
	if err := table.AddChecked(p.Ops, assignment, vir.Z80, p.Options); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/checked.json"
	if err := table.Save(path); err != nil {
		t.Fatal(err)
	}
	return steps, path
}

func executeLocalTableFunction(t *testing.T, asm string, x byte) int {
	t.Helper()
	bin, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, bin.Errors)
	}
	addr, ok := bin.Symbols["add5"]
	if !ok {
		addr, ok = bin.Symbols["ADD5"]
	}
	if !ok {
		t.Fatal("missing add5 label")
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	if err := z.LoadMemory(uint16(bin.Origin), bin.Binary); err != nil {
		t.Fatal(err)
	}
	if err := z.LoadMemory(0xf100, []byte{0x3e, x, 0xcd, byte(addr), byte(addr >> 8), 0xf3, 0x76}); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0xf100)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if r := z.GetRegisters(); r.A != x+5 || r.SP != 0xf000 {
		t.Fatalf("x=%d A=%d want=%d SP=%04x", x, r.A, x+5, r.SP)
	}
	return int(z.GetCycles())
}

func TestCheckedLocalTableOrdinaryHIRSelectsSmallerNativeFunction(t *testing.T) {
	baseline, path := localTableFixture(t)
	opts := Options{OptSize: true, AssertMode: "none", CheckedLocalTablePath: path}
	selected, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Traces["add5"].Backend != "checked-local-table" {
		t.Fatal("ordinary pipeline did not select candidate")
	}
	bsize, err := checkedAsmSize(baseline.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	ssize, err := checkedAsmSize(selected.Assembly)
	if err != nil || ssize >= bsize {
		t.Fatalf("module size %d→%d err=%v", bsize, ssize, err)
	}
	var bt, st int
	for x := 0; x < 256; x++ {
		bt = executeLocalTableFunction(t, baseline.Assembly, byte(x))
		st = executeLocalTableFunction(t, selected.Assembly, byte(x))
	}
	if st >= bt {
		t.Fatalf("native timing did not improve %d→%d", bt, st)
	}
	if baseline.MIR2Module.FuncByName("add5").Contract.Clobbers != selected.MIR2Module.FuncByName("add5").Contract.Clobbers {
		t.Fatal("caller clobber contract changed")
	}
	t.Logf("ordinary add5 all 256 byte inputs: module %d→%d bytes fixture %d→%d T; result/SP pass", bsize, ssize, bt, st)
	if dir := os.Getenv("VIR_LOCAL_PRODUCTION_EXPORT"); dir != "" {
		for name, data := range map[string][]byte{"source.nanz": []byte(checkedTableSource), "baseline.a80": []byte(baseline.Assembly), "selected.a80": []byte(selected.Assembly)} {
			if err := os.WriteFile(dir+"/"+name, data, 0644); err != nil {
				t.Fatal(err)
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/table.json", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckedLocalTableTamperFallsBackAndProfileRefuses(t *testing.T) {
	baseline, path := localTableFixture(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []vir.RegAllocEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	entries[0].Cost--
	data, _ = json.Marshal(entries)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	opts := Options{OptSize: true, AssertMode: "none", CheckedLocalTablePath: path}
	fallback, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts)
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Assembly != baseline.Assembly {
		t.Fatal("tampered evidence changed baseline")
	}
	if _, err := ProfileCompilationID(opts, "default"); err == nil {
		t.Fatal("mutable external table admitted to PGO")
	}
	// Changing the same explicit path to an empty table on the next compile
	// must produce a fresh miss, never a cached previous winner.
	if err := os.WriteFile(path, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	changedPath, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts)
	if err != nil || changedPath.Assembly != baseline.Assembly {
		t.Fatal("same path changed content did not fall back")
	}
	opts.OptSize = false
	if _, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts); err == nil {
		t.Fatal("size-only path admitted without OptSize")
	}
	opts.OptSize = true
	opts.CheckedLocalTablePath += ".missing"
	if _, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts); err == nil {
		t.Fatal("missing explicit input silently ignored")
	}
	if !reflect.DeepEqual(baseline.MIR2Module.FuncByName("add5").Blocks[0].Insts, fallback.MIR2Module.FuncByName("add5").Blocks[0].Insts) {
		t.Fatal("preparation mutated MIR")
	}
}
