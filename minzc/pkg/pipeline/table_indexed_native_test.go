package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexedHistoricalTripleNative(t *testing.T) {
	asset := os.Getenv("INDEXED_ENRT5")
	if asset == "" {
		t.Skip("set INDEXED_ENRT5")
	}
	path := "../../../examples/nanz/09_function_pointers.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module { return graceMeasureParse(t, string(src)) }
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	base, e := CompileHIRSteps(parse(), opts)
	if e != nil {
		t.Fatal(e)
	}
	tab := &vir.RegAllocTable{}
	tab.Init()
	fenced := base.MIR2Module.FuncByName("triple").Attrs.FixedIndirectABI
	problem, e := vir.PrepareLocalTableProblem(base.MIR2Module.FuncByName("triple"), base.Allocation)
	if fenced {
		if e == nil || e.Error() != "checked local table does not model the canonical indirect entry bridge" {
			t.Fatal("canonical indirect bridge requires exact refusal", e)
		}
	} else if e != nil {
		t.Fatal(e)
	}
	var evidence *vir.IndexedLegacyImportEvidence
	if !fenced {
		if index := os.Getenv("INDEXED_ENRT_INDEX"); index != "" {
			op, err := vir.OpenENRTOperation(asset, index)
			if err != nil {
				t.Fatal(err)
			}
			defer op.Close()
			evidence, e = op.Import(problem, tab, false)
			if err = op.Validate(); err != nil {
				t.Fatal(err)
			}
		} else {
			evidence, e = vir.ImportIndexedENRT(asset, problem, tab)
		}
		if e != nil || evidence.Import == nil || evidence.Import.Admitted != 1 {
			t.Fatal(e, evidence)
		}
		t.Logf("historical record%d bytes%dB prefixSHA=%s blockSHA=%s", evidence.RecordIndex, evidence.BytesRead, evidence.PrefixSHA256, evidence.BlockSHA256)
	}
	p := t.TempDir() + "/table.json"
	if e = tab.Save(p); e != nil {
		t.Fatal(e)
	}
	opts.CheckedLocalTablePath = p
	selected, e := CompileHIRSteps(parse(), opts)
	if e != nil {
		t.Fatal(e)
	}
	if fenced {
		if selected.Assembly != base.Assembly || tab.Size() != 0 {
			t.Fatal("indirect bridge fallback changed")
		}
	} else if selected.Traces["triple"].Backend != "checked-local-table" {
		t.Fatal("historical candidate was not selected")
	}
	vm := mir2.NewVM(selected.MIR2Module)
	rawVM := mir2.NewVM(hir.LowerModule(parse()))
	beforeSize, _ := checkedAsmSize(base.Assembly)
	afterSize, _ := checkedAsmSize(selected.Assembly)
	if !fenced && afterSize >= beforeSize {
		t.Fatal("no module benefit")
	}
	var bt, st int
	for x := 0; x < 256; x++ {
		want := byte(3 * x)
		for _, modelVM := range []*mir2.VM{rawVM, vm} {
			for _, entry := range []string{"triple", "double"} {
				k := int64(3)
				if entry == "double" {
					k = 2
				}
				args := []mir2.Value{{I: modelVM.GlobalAddr(entry)}, {I: int64(x)}}
				got, err := modelVM.Call("apply", args)
				if err != nil || len(got) != 1 || got[0].I != (k*int64(x))&255 {
					t.Fatalf("VM apply(%s,%d): %v %v", entry, x, got, err)
				}
			}
		}
		r, e := vm.Call("triple", []mir2.Value{{I: int64(x)}})
		if e != nil || byte(r[0].I) != want {
			t.Fatal(x, e)
		}
		bt = runCheckedRealByte(t, base.Assembly, base.MIR2Module.FuncByName("triple"), base.Allocation, byte(x), want)
		st = runCheckedRealByte(t, selected.Assembly, selected.MIR2Module.FuncByName("triple"), selected.Allocation, byte(x), want)
	}
	// Exercise the existing compiled apply(f,x) consumer through an actual
	// indirect call, with the real address-taken leaf ABI kept fixed.
	var baselineIndirect int
	for index, steps := range []Steps{base, selected} {
		bin, e := z80asm.NewAssembler().AssembleString(steps.Assembly)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		symbol := func(n string) int {
			v := bin.Symbols[n]
			if v == 0 {
				v = bin.Symbols[strings.ToUpper(n)]
			}
			return v
		}
		f := steps.MIR2Module.FuncByName("apply")
		loc0 := steps.Allocation.Locs[f.Contract.Params[0].Reg].Name
		loc1 := steps.Allocation.Locs[f.Contract.Params[1].Reg].Name
		var cycles int
		for x := 0; x < 256; x++ {
			boot, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("LD %s,%d\nLD %s,%d\nCALL %d\nDI\nHALT\n", loc0, symbol("triple"), loc1, x, symbol("apply")))
			if e != nil || len(boot.Errors) > 0 {
				t.Fatal(e, boot.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 10000
			z.LoadMemory(uint16(bin.Origin), bin.Binary)
			z.LoadMemory(0xf100, boot.Binary)
			z.SetPC(0xf100)
			z.SetSP(0xf000)
			if e = z.Run(); e != nil {
				t.Fatal(e)
			}
			r := z.GetRegisters()
			memory := z.Memory()
			if !bytes.Equal(memory[bin.Origin:bin.Origin+len(bin.Binary)], bin.Binary) {
				t.Fatal("indirect call modified compiled image")
			}
			if r.A != byte(3*x) || r.SP != 0xf000 {
				t.Fatalf("indirect x%d got%d SP%04x ASM%s", x, r.A, r.SP, steps.Assembly)
			}
			cycles = int(z.GetCycles())
		}
		if index == 0 {
			baselineIndirect = cycles
		} else if !fenced && cycles >= baselineIndirect {
			t.Fatal("indirect lost benefit")
		}
		t.Logf("existing indirect apply triple all256 %dT/%dB", cycles, len(bin.Binary))
	}
	if dir := os.Getenv("VIR_PRACTICAL_EXPORT"); dir != "" {
		os.MkdirAll(dir, 0755)
		for name, steps := range map[string]Steps{"baseline": base, "selected": selected} {
			bin, _ := z80asm.NewAssembler().AssembleString(steps.Assembly)
			os.WriteFile(filepath.Join(dir, name+".a80"), []byte(steps.Assembly), 0644)
			os.WriteFile(filepath.Join(dir, name+".bin"), bin.Binary, 0644)
		}
		data, _ := os.ReadFile(p)
		os.WriteFile(filepath.Join(dir, "table.json"), data, 0644)
		os.WriteFile(filepath.Join(dir, "source.nanz"), src, 0644)
		data, _ = json.MarshalIndent(evidence, "", "  ")
		os.WriteFile(filepath.Join(dir, "indexed-triple.json"), data, 0644)
	}
	if !fenced && st >= bt {
		t.Fatal("direct lost benefit")
	}
	for _, s := range []Steps{base, selected} {
		r, e := z80asm.NewAssembler().AssembleString(s.Assembly)
		if e != nil || len(r.Errors) > 0 {
			t.Fatal(e, r.Errors)
		}
		t.Logf("module %dB triple%dT→%dT", len(r.Binary), bt, st)
	}
}
