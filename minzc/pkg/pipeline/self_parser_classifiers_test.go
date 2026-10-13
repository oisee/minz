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
// The historical module and its four explicit CString output corrections are
// accepted; file/printing externs remain unexecuted in this classifier test.
func TestOriginalSelfParserCharacterClassifiers(t *testing.T) {
	const path = "examples/nanz/self_parser.nanz"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := os.ReadFile("../../../reports/self-parser-classifiers/source.nanz")
	if err != nil {
		t.Fatal("source freeze", err)
	}
	// Keep the original freeze intact. Whole-main testing found four SString
	// arguments passed to a NUL-reading output routine (Book v7 section 18.1).
	// Accept only those exact call-site edits, never arbitrary source drift.
	corrected := append([]byte(nil), frozen...)
	for _, literal := range []string{"(fun ", " (", ") ", "(return "} {
		old := []byte("out_str(\"" + literal + "\")")
		if bytes.Count(corrected, old) != 1 {
			t.Fatalf("historical CString correction missing or ambiguous: %q", literal)
		}
		corrected = bytes.Replace(corrected, old, []byte("out_str(c\""+literal+"\")"), 1)
	}
	if !bytes.Equal(src, frozen) && !bytes.Equal(src, corrected) {
		t.Fatal("source differs from historical freeze and its four CString corrections")
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
	save("source.nanz", src)
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
			image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || image == nil || len(image.Errors) > 0 {
				t.Fatal("assemble", e)
			}
			save(mode.Name+"/program.bin", image.Binary)
			z := emulator.NewRemogattoZ80()
			base := make([]byte, 65536)
			for i := range base {
				base[i] = byte(i*73 + 29)
			}
			copy(base[0x8000:], image.Binary)
			allowedPC := map[uint16]bool{}
			for _, name := range []string{"is_digit", "is_alpha", "is_alnum", "is_space"} {
				start, ok := image.Symbols[name]
				if !ok {
					t.Fatal("missing original entry", name)
				}
				end := 0x8000 + len(image.Binary)
				for _, fn := range s.MIR2Module.Funcs {
					if other, ok := image.Symbols[fn.Name]; ok && other > start && other < end {
						end = other
					}
				}
				for pc := start; pc < end; pc++ {
					allowedPC[uint16(pc)] = true
				}
			}
			base[0xfef0], base[0xfef1] = 0, 0x70
			for _, c := range model.Calls {
				for _, vm := range []*mir2.VM{rawVM, finalVM} {
					v, e := vm.Call(c.Entry, []mir2.Value{{I: int64(c.Input)}})
					if e != nil || len(v) != 1 || v[0].I != int64(c.Expected) {
						t.Fatal("VM", c, v, e)
					}
				}
				f := s.MIR2Module.FuncByName(c.Entry)
				a := hir.Assert{FuncName: c.Entry, Args: []int64{int64(c.Input)}, Expected: int64(c.Expected)}
				if len(f.Contract.Params) != 1 || f.Contract.Params[0].Ty != mir2.TyU8 {
					t.Fatal("unexpected classifier ABI", f.Contract)
				}
				copy(z.Memory(), base)
				regs := emulator.Registers{PC: uint16(image.Symbols[c.Entry]), SP: 0xfef0, A: 0x91, F: 0x85, BC: 0x2134, DE: 0x5678, HL: 0x9abc, IX: 0x1234, IY: 0x5678}
				input := uint16(c.Input)
				switch home := assertParamLocation(f.Contract.Params[0], s.Allocation); home.Name {
				case "A":
					regs.A = byte(input)
				case "B":
					regs.BC = (input << 8) | (regs.BC & 255)
				case "C":
					regs.BC = (regs.BC & 0xff00) | input
				case "D":
					regs.DE = (input << 8) | (regs.DE & 255)
				case "E":
					regs.DE = (regs.DE & 0xff00) | input
				case "H":
					regs.HL = (input << 8) | (regs.HL & 255)
				case "L":
					regs.HL = (regs.HL & 0xff00) | input
				default:
					t.Fatal("unsupported input home", home)
				}
				z.SetRegisters(regs)
				stop := uint16(0x7000)
				done := false
				minSP := uint16(0xfef0)
				for steps := 0; steps < 20000; steps++ {
					if z.GetPC() == stop && z.GetSP() == 0xfef2 {
						done = true
						break
					}
					if !allowedPC[z.GetPC()] {
						t.Fatalf("out-of-scope execution %v at %04x", c, z.GetPC())
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
				for i, b := range base {
					if z.Memory()[i] != b && !(i >= int(minSP) && i < 0xfef0) {
						t.Fatalf("unexpected native write %v at %04x", c, i)
					}
				}
				if e = checkAssertZ80Result(z, a, f, map[string]*hir.Func{c.Entry: h.FuncByName(c.Entry)}); e != nil {
					t.Fatal(c, e)
				}
			}
			save(mode.Name+"/result.txt", []byte(fmt.Sprintf("PASS: 1024 native CALLs, 1024 raw VM and 1024 final VM; exact frozen byte domain; PC/SP and RAM checked. Original main/externs unexecuted.\n")))
		})
	}
}
