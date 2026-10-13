package pipeline

import (
	"bytes"
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

func TestCheckedExistingFunctionPointerLeaf(t *testing.T) {
	solver := checkedRealSolver(t)
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
	tab, notes, e := GenerateCheckedLocalTables(parse(), opts, vir.SolverOptions{Z3Path: solver})
	if e != nil {
		t.Fatal(e)
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
	// Canonical indirect entries now have an ABI bridge which local tables do
	// not model. Require that precise refusal and unchanged ordinary fallback;
	// still execute the complete caller domain below.
	fenced := base.MIR2Module.FuncByName("triple").Attrs.FixedIndirectABI
	if fenced {
		want := "triple: skipped: checked local table does not model the canonical indirect entry bridge"
		found := false
		for _, note := range notes {
			found = found || note == want
		}
		if !found || tab.Size() != 0 || selected.Assembly != base.Assembly {
			t.Fatal("indirect bridge must retain exact baseline", notes)
		}
		t.Logf("canonical indirect bridge refused; exact baseline retained: %v", notes)
	} else if selected.Traces["triple"].Backend != "checked-local-table" {
		t.Fatal(notes)
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
		} else if (!fenced && cycles >= baselineIndirect) || (fenced && cycles != baselineIndirect) {
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
	}
	if (!fenced && st >= bt) || (fenced && st != bt) {
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
