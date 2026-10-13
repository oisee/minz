package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestPascalSignedDecimalFullWordDomain(t *testing.T) { pascalDecimalFullDomain(t, false) }
func TestPascalDecimalProductionFullDomain(t *testing.T) { pascalDecimalFullDomain(t, true) }
func pascalDecimalFullDomain(t *testing.T, production bool) {
	source, e := os.ReadFile("../../../reports/pascal-decimal-runtime/factorial.pas")
	if e != nil {
		t.Fatal(e)
	}
	h, e := pascal.Compile(string(source), "examples/pascal/factorial.pas")
	if e != nil {
		t.Fatal(e)
	}
	if production {
		h.Funcs = append(h.Funcs, hir.CanonicalPascalDecimalRuntime("WriteU16"))
		// The CP/M product keeps only main's reachable runtime closure.
		// Give the unsigned formatter a real source-level caller before
		// exercising its complete input domain through its native entry.
		for _, f := range h.Funcs {
			if f.Name == "main" {
				call := &hir.ExprStmt{Expr: &hir.CallExpr{Fn: "WriteU16", Ty: mir2.TyVoid, Args: []hir.Expr{&hir.IntLitExpr{Val: 0, Ty: mir2.TyU16}}}}
				f.Body.Body = append([]hir.Stmt{call}, f.Body.Body...)
			}
		}
	}
	opt := Options{ContractOpt: true}
	if production {
		opt.Runtime = mir2.Z80RuntimeCPM
		opt.RuntimeStackBytes = 1024
		opt.AssertMode = "none"
	}
	steps, e := CompileHIRSteps(h, opt)
	if e != nil {
		t.Fatal(e)
	}
	asm := "ORG 0x8000\n" + steps.Assembly
	if production {
		asm = steps.Assembly
	}
	image, e := z80asm.NewAssembler().AssembleString(asm)
	if e != nil || len(image.Errors) > 0 {
		t.Fatal(e)
	}
	save := func(n string, v any) {
		if root := os.Getenv("MINZ_PASCAL_DECIMAL_ARTIFACTS"); root != "" {
			p := filepath.Join(root, n)
			os.MkdirAll(filepath.Dir(p), 0755)
			var b []byte
			switch x := v.(type) {
			case []byte:
				b = x
			case string:
				b = []byte(x)
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			os.WriteFile(p, b, 0644)
		}
	}
	save("fixed.a80", asm)
	save("fixed.bin", image.Binary)
	save("fixed.mir", steps.MIR2Module.Dump())
	observations := []map[string]any{}
	total := 0
	entries := []string{"WriteI16"}
	if production {
		entries = append(entries, "WriteU16")
	}
	for _, entry := range entries {
		for input := 0; input < 65536; input++ {
			word := uint16(input)
			for _, clobber := range []bool{false, true} {
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				origin := uint16(0x8000)
				if production {
					origin = 0x100
				}
				z.LoadMemory(origin, image.Binary)
				sp := uint16(0xeffe)
				if production {
					sp = uint16(image.Symbols["__cpm_stack_top"]) - 2
				}
				z.SetMemory(sp, 0)
				z.SetMemory(sp+1, 0)
				z.SetRegisters(emulator.Registers{PC: uint16(image.Symbols[entry]), SP: sp, HL: word})
				before := append([]byte(nil), z.Memory()...)
				out := []byte{}
				z.SetBDOSHandler(func(fn byte, de uint16) (byte, uint16, bool) {
					if fn != 2 {
						t.Fatal("BDOS", fn, "entry", entry, "input", input, "clobber", clobber)
					}
					out = append(out, byte(de))
					r := z.GetRegisters()
					if clobber {
						r.A = 0xa5
						r.F = 0xff
						r.BC = 0x1234
						r.DE = 0x5678
						r.HL = 0xc3c3
						r.IX = 0x2468
						r.IY = 0x1357
						z.SetRegisters(r)
					}
					return r.A, r.HL, true
				})
				want := fmt.Sprint(int16(word))
				if entry == "WriteU16" {
					want = fmt.Sprint(word)
				}
				e := z.Run()
				if production {
					mem := z.Memory()
					start, top := image.Symbols["__cpm_stack"], image.Symbols["__cpm_stack_top"]
					for p, b := range before {
						if p >= start && p < top {
							continue
						}
						if mem[p] != b {
							t.Fatal("decimalRAM/code", word, clobber, p)
						}
					}
					if mem[image.Symbols["__cpm_stack_guard"]] != 0xa5 || mem[image.Symbols["__cpm_status"]] != 0 {
						t.Fatal("guard/status")
					}
				}
				if input%4096 == 0 || input == 32767 || input == 32768 || input == 65535 {
					observations = append(observations, map[string]any{"word": word, "signed": int16(word), "clobber": clobber, "stdout": string(out), "expected": want, "entry": entry, "error": implCPMErrorString(e), "registers": z.GetRegisters()})
				}
				total++
				if e != nil || string(out) != want || z.GetSP() != sp+2 {
					t.Fatal("signed decimal", word, clobber, string(out), fmt.Sprint(int16(word)), e, z.GetRegisters())
				}
			}
		}
	}
	save("fixed.json", map[string]any{"calls": total, "samples": observations})
}
