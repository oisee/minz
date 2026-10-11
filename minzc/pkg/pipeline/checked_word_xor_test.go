package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This is a negative practical result, not an expectation that the missing
// research descriptor must remain missing. Admit future smaller native code.
func TestCheckedExistingHash16Native(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/21_hashmap_test.nanz")
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	base, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
	if e != nil {
		t.Fatal(e)
	}
	bin, e := z80asm.NewAssembler().AssembleString(base.Assembly)
	if e != nil || len(bin.Errors) > 0 {
		t.Fatal(e, bin.Errors)
	}
	symbol := func(n string) uint16 {
		v, ok := bin.Symbols[n]
		if !ok {
			v, ok = bin.Symbols[strings.ToUpper(n)]
		}
		if !ok {
			t.Fatal("missing symbol", n)
		}
		return uint16(v)
	}
	z := emulator.NewRemogattoZ80()
	z.LoadMemory(uint16(bin.Origin), bin.Binary)
	z.LoadMemory(0xeffe, []byte{0, 0xf1})
	vm := mir2.NewVM(base.MIR2Module)
	f := base.MIR2Module.FuncByName("hash16")
	param := base.Allocation.Locs[f.Contract.Params[0].Reg].Name
	ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
	invoke := func(name string, x uint16) (emulator.Registers, int) {
		t.Helper()
		r := emulator.Registers{BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: symbol(name), SP: 0xeffe}
		if name == "hash16" {
			switch param {
			case "BC":
				r.BC = x
			case "DE":
				r.DE = x
			case "HL":
				r.HL = x
			default:
				t.Fatal("word parameter ABI", param)
			}
		}
		z.SetRegisters(r)
		start := z.GetCycles()
		for i := 0; i < 100000 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		got := z.GetRegisters()
		if got.SP != 0xf000 || got.PC != 0xf100 {
			t.Fatalf("%s return PC%04x SP%04x", name, got.PC, got.SP)
		}
		return got, z.GetCycles() - start
	}
	var cycles int
	// Language Book v7 §3.1 defines u16 as an unsigned 16-bit word;
	// Nanz_Language_Book §2.8 specifies bitwise XOR. The modular addition
	// below is an explicit Z80 width oracle (v7 does not state overflow policy).
	// All 65536 keys cover the carry boundary and every possible high/low byte.
	for x := 0; x < 65536; x++ {
		want := uint16(x) ^ uint16(x+37)
		result, e := vm.Call("hash16", []mir2.Value{{I: int64(x)}})
		if e != nil || len(result) != 1 || uint16(result[0].I) != want {
			t.Fatal(x, result, e)
		}
		got, c := invoke("hash16", uint16(x))
		var word uint16
		switch ret {
		case "HL":
			word = got.HL
		case "DE":
			word = got.DE
		case "BC":
			word = got.BC
		default:
			t.Fatal("word return ABI", ret)
		}
		if word != want {
			t.Fatalf("key%04x got%04x want%04x", x, word, want)
		}
		if x == 0 {
			cycles = c
		} else if c != cycles {
			t.Fatal("unexpected variable cost")
		}
	}
	t.Logf("unchanged hash16 all65536 VM/native PASS; %dT including RET; module%dB ABI%s→%s", cycles, len(bin.Binary), param, ret)
	// Run the existing real callers, including their allocator moves/saves and
	// persistent global map storage. Each source test clears its own map.
	for _, name := range []string{"test_put_get", "test_missing", "test_overwrite", "test_two_keys", "test_has", "test_count", "test_zero_key", "test_many"} {
		result, e := vm.Call(name, nil)
		if e != nil || len(result) != 1 || result[0].I != 1 {
			t.Fatal(name, result, e)
		}
		got, c := invoke(name, 0)
		if got.A != 1 {
			t.Fatal(name, got)
		}
		t.Logf("existing caller %s native%dT result1 SP=f000", name, c)
	}
	if dir := os.Getenv("VIR_WORD_XOR_EXPORT"); dir != "" {
		if e = os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		for n, b := range map[string][]byte{"source.nanz": src, "baseline.a80": []byte(base.Assembly), "baseline.bin": bin.Binary} {
			if e = os.WriteFile(filepath.Join(dir, n), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestCheckedExistingHash16GenerationEvidence(t *testing.T) {
	solver := checkedRealSolver(t)
	src, e := os.ReadFile("../../../examples/nanz/21_hashmap_test.nanz")
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	base, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
	if e != nil {
		t.Fatal(e)
	}
	table, notes, e := GenerateCheckedLocalTables(graceMeasureParse(t, string(src)), opts, vir.SolverOptions{Z3Path: solver})
	if e != nil {
		t.Fatal(e)
	}
	p := t.TempDir() + "/table.json"
	if e = table.Save(p); e != nil {
		t.Fatal(e)
	}
	opts.CheckedLocalTablePath = p
	selected, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
	if e != nil {
		t.Fatal(e)
	}
	b, e := checkedAsmSize(base.Assembly)
	if e != nil {
		t.Fatal(e)
	}
	s, e := checkedAsmSize(selected.Assembly)
	if e != nil {
		t.Fatal(e)
	}
	if s > b {
		t.Fatal("non-beneficial table selected", b, s)
	}
	t.Logf("generation notes=%v; table%dentries; module%d→%dB; hash16backend%s", notes, table.Size(), b, s, selected.Traces["hash16"].Backend)
	if dir := os.Getenv("VIR_WORD_XOR_EXPORT"); dir != "" {
		data, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "current-table.json"), data, 0644); e != nil {
			t.Fatal(e)
		}
	}
}
