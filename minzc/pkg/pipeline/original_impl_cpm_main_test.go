package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Preserve the whole original program, including its inline BDOS body. This
// freezes the current public-runtime refusal rather than bypassing admission.
func TestOriginalImplCPMRuntimeAdmission(t *testing.T) {
	const path = "examples/nanz/51_impl_cpm_demo.nanz"
	source, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := os.ReadFile("../../../reports/impl-cpm-main/source.nanz")
	if err != nil || !bytes.Equal(source, frozen) {
		t.Fatal("source freeze", err)
	}
	for _, mode := range []struct {
		name string
		o    Options
	}{
		{"ordinary", Options{ContractOpt: true}},
		{"grace-size", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}},
		{"grace-speed", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			h, err := nanz.Parse(string(source), path)
			if err != nil {
				t.Fatal(err)
			}
			o := mode.o
			o.Runtime = mir2.Z80RuntimeCPM
			o.RuntimeStackBytes = 1024
			o.AssertMode = "none"
			s, err := CompileHIRSteps(h, o)
			if root := os.Getenv("MINZ_IMPL_CPM_MAIN_ARTIFACTS"); root != "" {
				dir := filepath.Join(root, mode.name)
				if e := os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				b, _ := json.MarshalIndent(map[string]any{"options": o, "diagnostic": implCPMErrorString(err)}, "", "  ")
				if e := os.WriteFile(filepath.Join(dir, "admission.json"), b, 0644); e != nil {
					t.Fatal(e)
				}
				if s.MIR2Module != nil {
					if e := os.WriteFile(filepath.Join(dir, "final.mir"), []byte(s.MIR2Module.Dump()), 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			if err == nil || !strings.Contains(err.Error(), "CP/M runtime") {
				t.Fatalf("expected explicit runtime admission diagnostic, got %v", err)
			}
			t.Log(err)
		})
	}
}
func implCPMErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// A separate test-owned loader retains ordinary emitted main (DI/HALT). It
// does not make the rejected production RuntimeCPM path appear supported.
func TestOriginalImplCPMMainNativeLoader(t *testing.T) {
	source, err := os.ReadFile("../../../reports/impl-cpm-main/source.nanz")
	if err != nil {
		t.Fatal(err)
	}
	save := func(name string, v any) {
		if root := os.Getenv("MINZ_IMPL_CPM_MAIN_ARTIFACTS"); root != "" {
			p := filepath.Join(root, name)
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
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, mode := range []struct {
		name string
		o    Options
	}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true}}} {
		t.Run(mode.name, func(t *testing.T) {
			h, e := nanz.Parse(string(source), "examples/nanz/51_impl_cpm_demo.nanz")
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, mode.o)
			if e != nil {
				t.Fatal(e)
			}
			asm := "ORG 0x100\n__test_loader:\n LD SP,0xfdfe\n JP main\n" + s.Assembly
			image, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(image.Errors) > 0 {
				t.Fatal("assemble", e, image.Errors)
			}
			save(mode.name+"/test-loader.a80", asm)
			save(mode.name+"/test-loader.com", image.Binary)
			save(mode.name+"/native-final.mir", s.MIR2Module.Dump())
			save(mode.name+"/native-raw.mir", s.MIR2Raw)
			save(mode.name+"/native-context.json", map[string]any{"options": mode.o, "allocation": s.Allocation, "symbols": image.Symbols, "functions": s.MIR2Module.Funcs, "COM_sha256": fmt.Sprintf("%x", sha256.Sum256(image.Binary))})
			for _, clobber := range []bool{false, true} {
				t.Run(fmt.Sprintf("clobber-%v", clobber), func(t *testing.T) {
					results := []map[string]any{}
					run := func(entry string, n int, want string) {
						z := emulator.NewRemogattoZ80()
						mem := z.Memory()
						for i := range mem {
							mem[i] = byte(i*73 + 29)
						}
						z.LoadMemory(0x100, image.Binary)
						r := z.GetRegisters()
						r.PC = 0x100
						r.SP = 0xfdfe
						r.A = 0x71
						r.F = 0x85
						r.BC = 0x1234
						r.DE = 0x5678
						r.HL = 0x9abc
						r.IX = 0x2468
						r.IY = 0x1357
						if entry != "main" {
							r.PC = uint16(image.Symbols[entry])
							mem[0xfdfe] = 0
							mem[0xfdff] = 0
							f := s.MIR2Module.FuncByName(entry)
							loc := s.Allocation.Locs[f.Contract.Params[0].Reg]
							switch loc.Name {
							case "A":
								r.A = byte(n)
							case "B":
								r.BC = r.BC&255 | uint16(n)<<8
							case "C":
								r.BC = r.BC&0xff00 | uint16(n)
							default:
								t.Fatal("print ABI", loc)
							}
						}
						z.SetRegisters(r)
						before := append([]byte(nil), mem...)
						output := []byte{}
						ledger := []map[string]any{}
						minSP := r.SP
						done := false
						bdos := func(fn byte, de uint16) (byte, uint16, bool) {
							rr := z.GetRegisters()
							if fn != 2 {
								t.Fatalf("unexpected BDOS %d", fn)
							}
							output = append(output, byte(de))
							ledger = append(ledger, map[string]any{"function": fn, "char": byte(de), "registers": rr})
							if clobber {
								rr.A = 0xa5
								rr.F = 0xff
								rr.BC = 0x1234
								rr.DE = 0x5678
								rr.HL = 0x9abc
								rr.IX = 0x2468
								rr.IY = 0x1357
								z.SetRegisters(rr)
								return 0xa5, 0xc3c3, true
							}
							return rr.A, rr.HL, true
						}
						for step := 0; step < 100000; step++ {
							pc := z.GetPC()
							if pc == 5 {
								rr := z.GetRegisters()
								a, hl, handled := bdos(byte(rr.BC), rr.DE)
								if !handled {
									t.Fatal("unhandled BDOS")
								}
								rr = z.GetRegisters()
								rr.A = a
								rr.HL = hl
								rr.PC = uint16(mem[rr.SP]) | uint16(mem[rr.SP+1])<<8
								rr.SP += 2
								z.SetRegisters(rr)
								continue
							}

							if entry != "main" && pc == 0 && z.GetSP() == 0xfe00 {
								done = true
								break
							}
							if pc < 0x100 || int(pc) >= 0x100+len(image.Binary) {
								t.Fatalf("unexpected PC %04x", pc)
							}
							if mem[pc] == 0x76 {
								z.Step()
								done = true
								break
							}
							rr := z.GetRegisters()
							op := mem[pc]
							word := rr.BC
							push := true
							switch op {
							case 0xc5:
							case 0xd5:
								word = rr.DE
							case 0xe5:
								word = rr.HL
							case 0xf5:
								word = uint16(rr.A)<<8 | uint16(rr.F)
							case 0xcd:
								word = pc + 3
							default:
								push = false
							}
							if (op == 0xdd || op == 0xfd) && mem[pc+1] == 0xe5 {
								push = true
								if op == 0xdd {
									word = rr.IX
								} else {
									word = rr.IY
								}
							}
							z.Step()
							if push && (z.GetSP() != rr.SP-2 || mem[rr.SP-2] != byte(word) || mem[rr.SP-1] != byte(word>>8)) {
								t.Fatal("native stack word", entry, n, pc)
							}
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
						}
						mutable := map[uint16]bool{}
						for p := minSP; p < 0xfe00; p++ {
							mutable[p] = true
						}
						for _, line := range image.Listing {
							if strings.HasPrefix(strings.ToLower(line.Label), "_spill_") {
								for j := range line.Bytes {
									mutable[uint16(line.Address+j)] = true
								}
							}
						}
						changed := []int{}
						for p, v := range before {
							if !mutable[uint16(p)] && mem[p] != v {
								changed = append(changed, p)
							}
						}
						result := map[string]any{"entry": entry, "input": n, "stdout": string(output), "expected": want, "registers": z.GetRegisters(), "cycles": z.GetCycles(), "min_sp": minSP, "BDOS": ledger, "unexpected_RAM_changes": changed, "done": done, "termination": "ordinary main DI/HALT; print helper RET to sentinel; test-owned loader, not production RuntimeCPM"}
						results = append(results, result)
						expectedSP := uint16(0xfdfe)
						if entry != "main" {
							expectedSP = 0xfe00
						}
						if !done || string(output) != want || z.GetSP() != expectedSP || minSP < 0xfd00 || len(changed) != 0 || mem[0xfdfe] != before[0xfdfe] || mem[0xfdff] != before[0xfdff] {
							t.Fatalf("%s(%d) got%q want%q SP%04x want%04x changed%v done%v", entry, n, output, want, z.GetSP(), expectedSP, changed, done)
						}
					}
					run("main", 0, "C:75\r\nR:50\r\n")
					for n := 0; n < 256; n++ {
						run("print_u8", n, fmt.Sprintf("%02d", n))
					}
					save(fmt.Sprintf("%s/clobber-%v/ledger.json", mode.name, clobber), results)
				})
			}
		})
	}
}
