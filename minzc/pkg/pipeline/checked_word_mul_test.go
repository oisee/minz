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
	"strings"
	"testing"
)

func TestCheckedActualWordMultiplication(t *testing.T) {
	solver := checkedRealSolver(t)
	src, e := os.ReadFile("../../../examples/nanz/mul16_gpu_test.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, caller := range []bool{false, true} {
		text := string(src)
		if caller {
			text += "\nfun mul7_caller(x:u16)->u16{return mul_by_7(x)+x}\n"
		}
		opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
		base, e := CompileHIRSteps(graceMeasureParse(t, text), opts)
		if e != nil {
			t.Fatal(e)
		}
		tab, notes, e := GenerateCheckedLocalTables(graceMeasureParse(t, text), opts, vir.SolverOptions{Z3Path: solver})
		if e != nil {
			t.Fatal(e)
		}
		path := t.TempDir() + "/table.json"
		if e = tab.Save(path); e != nil {
			t.Fatal(e)
		}
		opts.CheckedLocalTablePath = path
		selected, e := CompileHIRSteps(graceMeasureParse(t, text), opts)
		if e != nil {
			t.Fatal(e)
		}
		b, e := checkedAsmSize(base.Assembly)
		if e != nil {
			t.Fatal(e)
		}
		s, e := checkedAsmSize(selected.Assembly)
		if e != nil || s >= b {
			t.Fatal(b, s, e, notes)
		}
		raw := mir2.NewVM(hir.LowerModule(graceMeasureParse(t, text)))
		cases := []struct {
			name string
			k    int
		}{{"mul_by_3", 3}, {"mul_by_10", 10}, {"mul_by_100", 100}, {"mul_by_7", 7}, {"mul_by_255", 255}}
		if caller {
			cases = []struct {
				name string
				k    int
			}{{"mul_by_7", 7}}
		}
		for _, c := range cases {
			if !caller && selected.Traces[c.name].Backend != "checked-local-table" {
				t.Log("admitted but not selected", c.name, selected.Traces[c.name].Backend)
			}
			if !caller {
				p, err := vir.PrepareLocalTableProblem(base.MIR2Module.FuncByName(c.name), base.Allocation)
				if err != nil {
					t.Fatal(err)
				}
				candidate, err := p.Emit(tab)
				if err != nil {
					t.Fatal(err)
				}
				lo, hi, ok := checkedFunctionSpan(base.Assembly, c.name)
				if !ok {
					t.Fatal("span")
				}
				proposal := base
				proposal.Assembly = base.Assembly[:lo] + candidate + base.Assembly[hi:]
				cost := checkedWordNative(t, proposal, c.name, c.k, raw)
				t.Logf("admitted raw candidate %s %dT", c.name, cost)
			}
			before := checkedWordNative(t, base, c.name, c.k, raw)
			after := checkedWordNative(t, selected, c.name, c.k, raw)
			if after > before {
				t.Fatal("native regression", c.name, before, after)
			}
			if caller {
				cb := checkedWordCaller(t, base.Assembly, c.name)
				ca := checkedWordCaller(t, selected.Assembly, c.name)
				if ca >= cb {
					t.Fatal("caller no gain")
				}
				t.Logf("external native caller retains x all65536 %d→%dT", cb, ca)
			}
			t.Logf("%s all65536 %d→%dT module%d→%dB", c.name, before, after, b, s)
		}
		if dir := os.Getenv("CHECKED_WORD_EXPORT"); dir != "" {
			label := "original"
			if caller {
				label = "caller"
			}
			dir += "/" + label
			os.MkdirAll(dir, 0755)
			os.WriteFile(dir+"/source.nanz", []byte(text), 0644)
			if caller {
				os.WriteFile(dir+"/external-caller.a80", []byte("PUSH HL\nCALL mul_by_7\nPOP DE\nADD HL,DE\nRET\n"), 0644)
			}
			for label, steps := range map[string]Steps{"baseline": base, "selected": selected} {
				bin, _ := z80asm.NewAssembler().AssembleString(steps.Assembly)
				os.WriteFile(dir+"/"+label+".a80", []byte(steps.Assembly), 0644)
				os.WriteFile(dir+"/"+label+".bin", bin.Binary, 0644)
			}
			data, _ := os.ReadFile(path)
			os.WriteFile(dir+"/table.json", data, 0644)
			data, _ = json.MarshalIndent(notes, "", "  ")
			os.WriteFile(dir+"/notes.json", data, 0644)
		}
	}
}
func checkedWordNative(t *testing.T, steps Steps, name string, k int, raw *mir2.VM) int {
	t.Helper()
	bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil || len(bin.Errors) > 0 {
		t.Fatal(e, bin.Errors)
	}
	f := steps.MIR2Module.FuncByName(name)
	addr := bin.Symbols[name]
	if addr == 0 {
		addr = bin.Symbols[strings.ToUpper(name)]
	}
	z := emulator.NewRemogattoZ80()
	z.LoadMemory(0x8000, bin.Binary)
	z.LoadMemory(0xeffe, []byte{0, 0xf1})
	cost := -1
	for x := 0; x < 65536; x++ {
		want := uint16(x * k)
		r := emulator.Registers{A: 0x77, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(addr), SP: 0xeffe}
		switch steps.Allocation.Locs[f.Contract.Params[0].Reg].Name {
		case "HL":
			r.HL = uint16(x)
		case "DE":
			r.DE = uint16(x)
		case "BC":
			r.BC = uint16(x)
		default:
			t.Fatal("ABI")
		}
		z.SetRegisters(r)
		before := z.GetCycles()
		for i := 0; i < 1024 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		got := z.GetRegisters()
		if got.HL != want || got.PC != 0xf100 || got.SP != 0xf000 {
			t.Fatal(name, x, want, got)
		}
		if cost < 0 {
			cost = z.GetCycles() - before
		} else if cost != z.GetCycles()-before {
			t.Fatal("cost varies", name)
		}
		if x < 256 || x%257 == 0 {
			v, e := raw.Call(name, []mir2.Value{{I: int64(x)}})
			if e != nil || len(v) != 1 || uint16(v[0].I) != want {
				t.Fatal("raw oracle", x, v, e)
			}
		}
	}
	if !bytes.Equal(z.Memory()[0x8000:0x8000+len(bin.Binary)], bin.Binary) {
		t.Fatal("code mutation")
	}
	return cost
}

func checkedWordCaller(t *testing.T, asm, name string) int {
	bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if e != nil || len(bin.Errors) > 0 {
		t.Fatal(e, bin.Errors)
	}
	addr := bin.Symbols[name]
	if addr == 0 {
		addr = bin.Symbols[strings.ToUpper(name)]
	}
	boot, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0xf100\nPUSH HL\nCALL %d\nPOP DE\nADD HL,DE\nRET\n", addr))
	if e != nil || len(boot.Errors) > 0 {
		t.Fatal(e, boot.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.LoadMemory(0x8000, bin.Binary)
	z.LoadMemory(0xf100, boot.Binary)
	z.LoadMemory(0xeffe, []byte{0, 0xf2})
	cost := -1
	for x := 0; x < 65536; x++ {
		z.SetRegisters(emulator.Registers{HL: uint16(x), BC: 0x1234, DE: 0x5678, PC: 0xf100, SP: 0xeffe})
		before := z.GetCycles()
		for n := 0; n < 1024 && z.GetRegisters().PC != 0xf200; n++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.HL != uint16(8*x) || r.SP != 0xf000 || r.PC != 0xf200 {
			t.Fatal("caller", x, r)
		}
		if cost < 0 {
			cost = z.GetCycles() - before
		} else if cost != z.GetCycles()-before {
			t.Fatal("cost")
		}
	}
	if !bytes.Equal(z.Memory()[0x8000:0x8000+len(bin.Binary)], bin.Binary) {
		t.Fatal("code")
	}
	return cost
}
