package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Book v7 chapters 2, 3 and 5: byte integers, comparisons and explicit returns.
// This is the original module, including unexecuted file/printing externs.
func TestOriginalSelfParserCharacterClassifiers(t *testing.T) {
	const path = "examples/nanz/self_parser.nanz"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := os.ReadFile("../../../reports/self-parser-classifiers/source.nanz")
	if err != nil || !bytes.Equal(src, frozen) {
		t.Fatal("source freeze", err)
	}
	var model struct {
		Calls []struct {
			Entry           string
			Input, Expected int
		}
	}
	payload, err := os.ReadFile("../../../reports/self-parser-classifiers/frozen.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(payload, &model); err != nil {
		t.Fatal(err)
	}
	if len(model.Calls) != 1024 {
		t.Fatal("frozen domain", len(model.Calls))
	}
	save := func(name string, b []byte) {
		if d := os.Getenv("MINZ_SELF_PARSER_CLASSIFIERS_ARTIFACTS"); d != "" {
			p := filepath.Join(d, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	raw := hir.LowerModule(parse())
	save("raw.mir", []byte(raw.Dump()))
	for _, mode := range []struct {
		Name    string
		Options Options
	}{{"ordinary", Options{ContractOpt: true, AssertMode: "none"}}, {"grace-speed", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, AssertMode: "none"}}, {"grace-size", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true, AssertMode: "none"}}} {
		t.Run(mode.Name, func(t *testing.T) {
			h := parse()
			s, e := CompileHIRSteps(h, mode.Options)
			if e != nil {
				t.Fatal(e)
			}
			save(mode.Name+"/program.a80", []byte(s.Assembly))
			save(mode.Name+"/final.mir", []byte(s.MIR2Module.Dump()))
			meta, _ := json.MarshalIndent(map[string]any{"options": mode.Options, "allocation": s.Allocation, "functions": s.MIR2Module.Funcs}, "", "  ")
			save(mode.Name+"/context.json", meta)
			rawVM, finalVM := mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)
			for _, vm := range []*mir2.VM{rawVM, finalVM} {
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { t.Fatalf("pure classifier memory effect: %+v", a) }
			}
			for _, c := range model.Calls {
				for _, vm := range []*mir2.VM{rawVM, finalVM} {
					v, e := vm.Call(c.Entry, []mir2.Value{{I: int64(c.Input)}})
					if e != nil || len(v) != 1 || v[0].I != int64(c.Expected) {
						t.Fatal("VM", c, v, e)
					}
				}
				f := s.MIR2Module.FuncByName(c.Entry)
				a := hir.Assert{FuncName: c.Entry, Args: []int64{int64(c.Input)}, Expected: int64(c.Expected)}
				boot := buildAssertBootstrap(0x8000, a, f, s.Allocation)
				image, e := z80asm.NewAssembler().AssembleString(boot + s.Assembly)
				if e != nil || image == nil || len(image.Errors) > 0 {
					t.Fatal("assemble", e)
				}
				z := emulator.NewRemogattoZ80()
				for i := range z.Memory() {
					z.Memory()[i] = byte(i*73 + 29)
				}
				if e = z.LoadMemory(0x8000, image.Binary); e != nil {
					t.Fatal(e)
				}
				before := append([]byte(nil), z.Memory()...)
				z.SetPC(0x8000)
				stop := uint16(0x8000 + trampolineSize(a, f, s.Allocation) - 2)
				done := false
				minSP := uint16(0xff00)
				for steps := 0; steps < 20000; steps++ {
					if z.GetPC() == stop && z.GetSP() == 0xff00 {
						done = true
						break
					}
					z.Step()
					if sp := z.GetSP(); sp < minSP {
						minSP = sp
					}
				}
				if !done {
					t.Fatalf("native return %v PC=%04x SP=%04x", c, z.GetPC(), z.GetSP())
				}
				if minSP < 0xfe80 {
					t.Fatal("unexpected stack depth", minSP)
				}
				for i, b := range before {
					if z.Memory()[i] != b && !(i >= int(minSP) && i < 0xff00) {
						t.Fatalf("unexpected native write %v at %04x", c, i)
					}
				}
				if e = checkAssertZ80Result(z, a, f, map[string]*hir.Func{c.Entry: h.FuncByName(c.Entry)}); e != nil {
					t.Fatal(c, e)
				}
				if c.Input == 0 {
					save(mode.Name+"/"+c.Entry+".bin", image.Binary)
				}
			}
			save(mode.Name+"/result.txt", []byte(fmt.Sprintf("PASS: 1024 native CALLs, 1024 raw VM and 1024 final VM; exact frozen byte domain; PC/SP and RAM checked. Original main/externs unexecuted.\n")))
		})
	}
}
