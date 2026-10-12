package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOriginalPascalStringClassifiersNative(t *testing.T) {
	const path = "../../../examples/pascal/string_test.pas"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/pascal-string-native/source.pas")
	if e != nil || string(source) != string(frozen) {
		t.Fatal("original source freeze", e)
	}
	save := func(mode, name string, payload []byte) {
		if root := os.Getenv("MINZ_PASCAL_STRING_ARTIFACTS"); root != "" {
			dir := filepath.Join(root, mode)
			if e := os.MkdirAll(dir, 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(dir, name), payload, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	type call struct {
		Entry string
		Input int
		Want  int
	}
	var calls []call
	for _, interval := range []struct {
		Name   string
		Lo, Hi int
	}{{"ISDIGIT", 48, 57}, {"ISUPPER", 65, 90}, {"ISLOWER", 97, 122}} {
		for value := 0; value < 256; value++ {
			want := 0
			if value >= interval.Lo && value <= interval.Hi {
				want = 1
			}
			calls = append(calls, call{interval.Name, value, want})
		}
	}
	calls = append(calls, call{"main", 0, 0})
	for _, mode := range []string{"default", "grace-speed", "grace-size"} {
		parse := func() *hir.Module {
			h, e := pascal.Compile(string(source), path)
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		raw := hir.LowerModule(parse())
		h := parse()
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: mode != "default", OptSize: mode == "grace-size", AssertMode: "none"})
		if e != nil {
			t.Fatal(mode, e)
		}
		var drivers strings.Builder
		for _, entry := range []string{"ISDIGIT", "ISUPPER", "ISLOWER", "main"} {
			if h.FuncByName(entry) == nil {
				t.Fatalf("missing original %s actual%v", entry, h.Funcs)
			}
			fmt.Fprintf(&drivers, "\noracle_%s:\n CALL %s\n RET\n", entry, entry)
		}
		asm := steps.Assembly + drivers.String()
		image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if e != nil || len(image.Errors) > 0 {
			t.Fatal(e, image.Errors)
		}
		save(mode, "source.pas", source)
		save(mode, "raw.mir", []byte(raw.Dump()))
		save(mode, "final.mir", []byte(steps.MIR2Module.Dump()))
		save(mode, "target.a80", []byte(asm))
		save(mode, "target.bin", image.Binary)
		alloc, _ := json.MarshalIndent(steps.Allocation, "", "  ")
		save(mode, "allocation.json", alloc)
		// These pure classifiers and empty main require no writable global or
		// spill data. A future allocation change must model exact homes rather
		// than mask neighboring code bytes as a guessed two-byte slot.
		if len(steps.MIR2Module.Globals) != 0 || len(steps.Allocation.Spilled) != 0 {
			t.Fatal("unexpected writable state in pure classifier module")
		}
		z := emulator.NewRemogattoZ80()
		var evidence []map[string]any
		for _, c := range calls {
			args := []mir2.Value{{I: int64(c.Input)}}
			if c.Entry == "main" {
				args = nil
			}
			for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
				vm := mir2.NewVM(m)
				effects := 0
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { effects++ }
				got, e := vm.Call(c.Entry, args)
				if e != nil {
					t.Fatal(mode, c, e)
				}
				if c.Entry == "main" {
					if len(got) != 0 {
						t.Fatal("main return", got)
					}
				} else if len(got) != 1 || got[0].I != int64(c.Want) {
					t.Fatal(mode, "VM", c, got)
				}
				if effects != 0 {
					t.Fatal("unexpected VM memory effect", c, effects)
				}
			}
			if e = z.LoadMemory(0x8000, image.Binary); e != nil {
				t.Fatal(e)
			}
			entryLabel := "oracle_" + c.Entry
			if c.Entry == "main" {
				entryLabel = "main"
			}
			regs := emulator.Registers{PC: uint16(image.Symbols[entryLabel]), SP: 0xeff0, A: 0x91, F: 0x85, BC: 0x2134, DE: 0x5678, HL: 0x9abc, IX: 0x1234, IY: 0x5678}
			f := steps.MIR2Module.FuncByName(c.Entry)
			if c.Entry != "main" {
				v := uint16(c.Input)
				loc := assertParamLocation(f.Contract.Params[0], steps.Allocation)
				switch loc.Name {
				case "A":
					regs.A = byte(v)
				case "B":
					regs.BC = regs.BC&255 | v<<8
				case "C":
					regs.BC = regs.BC&0xff00 | v
				case "D":
					regs.DE = regs.DE&255 | v<<8
				case "E":
					regs.DE = regs.DE&0xff00 | v
				case "H":
					regs.HL = regs.HL&255 | v<<8
				case "L":
					regs.HL = regs.HL&0xff00 | v
				case "IXH":
					regs.IX = regs.IX&255 | v<<8
				case "IXL":
					regs.IX = regs.IX&0xff00 | v
				case "IYH":
					regs.IY = regs.IY&255 | v<<8
				case "IYL":
					regs.IY = regs.IY&0xff00 | v
				default:
					if loc.Kind != mir2.LocMem {
						t.Fatal(loc)
					}
					z.Memory()[image.Symbols[mir2.Z80SpillLabel(f.Name, f.Contract.Params[0].Reg)]] = byte(v)
				}
			}
			z.Memory()[0xeff0] = 0
			z.Memory()[0xeff1] = 0x70
			before := append([]byte(nil), z.Memory()...)
			z.SetRegisters(regs)
			minSP := uint16(0xeff0)
			start := z.GetCycles()
			for n := 0; ; n++ {
				if c.Entry == "main" && z.IsHalted() {
					break
				}
				if c.Entry != "main" && z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					break
				}
				if n == 10000 {
					t.Fatal("no native return", mode, c, z.GetRegisters())
				}
				if z.GetSP() <= 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("stack", mode, c)
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if c.Entry == "main" {
				if z.GetSP() != 0xeff0 || len(z.GetOutput()) != 0 {
					t.Fatal("main halt state", z.GetRegisters())
				}
			} else {
				if e := checkAssertZ80Result(z, hir.Assert{FuncName: c.Entry, Expected: int64(c.Want)}, f, map[string]*hir.Func{c.Entry: h.FuncByName(c.Entry)}); e != nil {
					t.Fatal(mode, c, e)
				}
			}
			for address, value := range before {
				if address >= int(minSP) && address < 0xeff0 {
					continue
				}
				if z.Memory()[address] != value {
					t.Fatalf("%s %+v changedRAM%x", mode, c, address)
				}
			}
			evidence = append(evidence, map[string]any{"entry": c.Entry, "input": c.Input, "expected": c.Want, "T": z.GetCycles() - start, "SP": z.GetSP(), "minSP": minSP, "RAM_code_unchanged": true})
		}
		data, _ := json.MarshalIndent(evidence, "", "  ")
		save(mode, "evidence.json", data)
		t.Logf("%s original4entry %d calls binary%d SHA%x", mode, len(calls), len(image.Binary), sha256.Sum256(image.Binary))
	}
}
