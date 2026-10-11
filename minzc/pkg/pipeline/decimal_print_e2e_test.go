package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const decimalPrintSource = `global input: u16 = 255
fun demo() -> u16 {
 let x: u16 = input + 1
 @print_u8(x)
 @print_dec(x + 99)
 return x + 17
}`

func decimalSourceVM(t *testing.T) []byte {
	t.Helper()
	m, e := nanz.Parse(decimalPrintSource, "decimal-print.nanz")
	if e != nil {
		t.Fatal(e)
	}
	raw := hir.LowerModule(m)
	if e = mir2.Verify(raw); e != nil {
		t.Fatal(e)
	}
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	old := os.Stdout
	os.Stdout = w
	vals, callErr := mir2.NewVM(raw).Call("demo", nil)
	w.Close()
	os.Stdout = old
	data, readErr := io.ReadAll(r)
	r.Close()
	if callErr != nil || readErr != nil || len(vals) != 1 || vals[0].I != 273 || !bytes.Equal(data, []byte("099")) {
		t.Fatalf("VM vals%v output%q errors%v/%v", vals, data, callErr, readErr)
	}
	return data
}
func decimalSourceAssembly(t *testing.T, legacy bool) (string, *Steps) {
	t.Helper()
	m, e := nanz.Parse(decimalPrintSource, "decimal-print.nanz")
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{AssertMode: "none"}
	if legacy {
		a, e := CompileHIRWithOptions(m, opts)
		if e != nil {
			t.Fatal(e)
		}
		return a, nil
	}
	steps, e := CompileHIRSteps(m, opts)
	if e != nil {
		t.Fatal(e)
	}
	return steps.Assembly, &steps
}
func TestDecimalPrintSourceCompile(t *testing.T) {
	decimalSourceVM(t)
	for _, legacy := range []bool{false, true} {
		a, _ := decimalSourceAssembly(t, legacy)
		assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + a)
		if e != nil {
			t.Fatal(e)
		}
		if len(assembled.Errors) > 0 {
			t.Fatal(assembled.Errors)
		}
	}
}
func TestDecimalPrintSourceNative(t *testing.T) {
	dir := os.Getenv("MINZ_DECIMAL_PRINT_ARTIFACTS")
	if dir == "" {
		t.Skip("native artifact export not requested")
	}
	vm := decimalSourceVM(t)
	for _, legacy := range []bool{false, true} {
		name := "steps"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			a, steps := decimalSourceAssembly(t, legacy)
			text := "ORG 0x8000\nstart:\n LD SP, 0xf000\n CALL demo\n LD (answer), HL\ndone:\n DI\n HALT\nanswer: DW 0\n" + a
			assembled, e := z80asm.NewAssembler().AssembleString(text)
			if e != nil {
				t.Fatal(e)
			}
			if len(assembled.Errors) > 0 {
				t.Fatal(assembled.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.Reset()
			if e = z.LoadMemory(0x8000, assembled.Binary); e != nil {
				t.Fatal(e)
			}
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000})
			z.MaxCycles = 100000
			runErr := z.Run()
			regs := z.GetRegisters()
			addr := uint16(assembled.Symbols["answer"])
			answer := uint16(z.GetMemory(addr)) | uint16(z.GetMemory(addr+1))<<8
			d := filepath.Join(dir, "source-"+name)
			if e = os.MkdirAll(d, 0755); e != nil {
				t.Fatal(e)
			}
			write := func(n string, b []byte) {
				if e := os.WriteFile(filepath.Join(d, n), b, 0644); e != nil {
					t.Fatal(e)
				}
			}
			write("source.nanz", []byte(decimalPrintSource))
			write("assembled.a80", []byte(text))
			write("target.bin", assembled.Binary)
			write("output.bin", z.GetOutput())
			write("expected.bin", vm)
			rec := struct {
				Registers  emulator.Registers
				Answer     uint16
				Cycles     int
				Error      string
				ExpectedPC uint16
				ExpectedSP uint16
			}{regs, answer, z.GetCycles(), fmt.Sprint(runErr), uint16(assembled.Symbols["done"]) + 1, 0xf000}
			for n, obj := range map[string]any{"execution.json": rec, "symbols.json": assembled.Symbols, "steps.json": steps} {
				data, e := json.MarshalIndent(obj, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				write(n, data)
			}
			if runErr != nil || !bytes.Equal(z.GetOutput(), vm) || answer != 273 || regs.SP != 0xf000 || regs.PC != uint16(assembled.Symbols["done"])+1 {
				t.Fatalf("output%q answer%d regs%+v error%v", z.GetOutput(), answer, regs, runErr)
			}
		})
	}
}
