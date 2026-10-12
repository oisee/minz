package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/plm"
	"github.com/minz/minzc/pkg/z80asm"
)

// PL/M addressed lists contain exact bytes; the source explicitly adds NUL.
// The compiler console intrinsic's semantic model is one byte of output;
// native execution runs the public CP/M runtime and models only BDOS itself.
func TestOriginalPLMHelloWholeCPM(t *testing.T) {
	source, err := os.ReadFile("../../../examples/plm/hello.plm")
	if err != nil {
		t.Fatal(err)
	}
	want := "Hello from PL/M-80!\r\n"
	save := func(name string, value any) {
		if dir := os.Getenv("MINZ_PLM_HELLO_ARTIFACTS"); dir != "" {
			p := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				t.Fatal(err)
			}
			var b []byte
			switch v := value.(type) {
			case string:
				b = []byte(v)
			case []byte:
				b = v
			default:
				b, err = json.MarshalIndent(v, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(p, b, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	save("source/hello.plm", source)
	for _, mode := range []struct {
		name string
		opt  Options
	}{
		{"ordinary", Options{}}, {"grace-size", Options{UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{UseGrace: true, UseHIRGrace: true}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			h, err := plm.Compile(string(source))
			if err != nil {
				t.Fatal(err)
			}
			if len(h.Funcs) != 3 || h.FuncByName("main") == nil {
				t.Fatal("original entry missing")
			}
			raw := hir.LowerModule(h)
			h.Target = hir.TargetCPM
			opt := mode.opt
			opt.AssertMode = "none"
			opt.Runtime = mir2.Z80RuntimeCPM
			opt.RuntimeStackBytes = 1024
			steps, err := CompileHIRSteps(h, opt)
			if err != nil {
				t.Fatal(err)
			}
			for name, m := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
				vm := mir2.NewVM(m)
				var output bytes.Buffer
				vm.Hosts["@mir.io.print.char"] = func(args []mir2.Value) ([]mir2.Value, error) {
					if len(args) != 1 {
						return nil, fmt.Errorf("bad console arity")
					}
					output.WriteByte(byte(args[0].I))
					return nil, nil
				}
				result, err := vm.Call("main", nil)
				if err != nil || len(result) != 0 || output.String() != want {
					t.Fatal(name, result, err, output.String())
				}
			}
			image, err := z80asm.NewAssembler().AssembleString("ORG 0x100\n" + steps.Assembly)
			if err != nil || len(image.Errors) != 0 {
				t.Fatal(err, image.Errors)
			}
			save(mode.name+"/program.a80", steps.Assembly)
			save(mode.name+"/program.com", image.Binary)
			save(mode.name+"/raw.mir", raw.Dump())
			save(mode.name+"/final.mir", steps.MIR2Module.Dump())
			save(mode.name+"/allocation.json", steps.Allocation)
			for _, clobber := range []bool{false, true} {
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				mem := z.Memory()
				for p := range mem {
					mem[p] = byte(p*73 + 29)
				}
				z.LoadMemory(0x100, image.Binary)
				mem[0xfdfe] = 0
				mem[0xfdff] = 0
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
				z.SetRegisters(r)
				before := append([]byte(nil), mem...)
				var output []byte
				minSP := uint16(image.Symbols["__cpm_stack_top"])
				done := false
				for step := 0; step < 100000; step++ {
					pc := z.GetPC()
					if pc == 0 && z.GetSP() == 0xfe00 {
						done = true
						break
					}
					if pc == 5 {
						r := z.GetRegisters()
						if byte(r.BC) != 2 || len(output) >= len(want) || byte(r.DE) != want[len(output)] {
							t.Fatal("unexpected BDOS event", r, len(output))
						}
						ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
						if ret < 0x103 || int(ret) > 0x100+len(image.Binary) || mem[ret-3] != 0xcd || mem[ret-2] != 5 || mem[ret-1] != 0 {
							t.Fatal("BDOS CALL authority", ret)
						}
						output = append(output, byte(r.DE))
						if clobber {
							r.A = 0xa5
							r.F = 0xff
							r.BC = 0x3456
							r.DE = 0x789a
							r.HL = 0xbcde
							r.IX = 0x1357
							r.IY = 0x2468
						}
						r.PC = ret
						r.SP += 2
						z.SetRegisters(r)
						continue
					}
					if pc < 0x100 || int(pc) >= 0x100+len(image.Binary) {
						t.Fatal("escaped executable", pc)
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
				}
				mutable := map[int]bool{}
				for _, interval := range [][2]int{{image.Symbols["__cpm_stack"], image.Symbols["__cpm_stack_top"]}, {image.Symbols["__cpm_saved_sp"], image.Symbols["__cpm_saved_sp"] + 2}, {image.Symbols["__cpm_status"], image.Symbols["__cpm_status"] + 1}} {
					for p := interval[0]; p < interval[1]; p++ {
						mutable[p] = true
					}
				}
				for _, line := range image.Listing {
					if strings.HasPrefix(strings.ToLower(line.Label), "_spill_") {
						for j := range line.Bytes {
							mutable[line.Address+j] = true
						}
					}
				}
				for p, b := range before {
					if !mutable[p] && mem[p] != b {
						t.Fatalf("unexpected RAM/code change at %04x", p)
					}
				}
				if !done || string(output) != want || minSP < uint16(image.Symbols["__cpm_stack"]) || mem[image.Symbols["__cpm_stack_guard"]] != 0xa5 || mem[image.Symbols["__cpm_status"]] != 0 {
					t.Fatal("whole main", done, string(output), minSP)
				}
				save(fmt.Sprintf("%s/clobber-%v.json", mode.name, clobber), map[string]any{"stdout": string(output), "expected": want, "registers": z.GetRegisters(), "min_sp": minSP, "cycles_excluding_host_bdos": z.GetCycles(), "full_ram_guard": true, "exact_return_pc_sp": true, "done": done})
				t.Logf("%s clobber=%v COM=%d bytes native=%d T excluding host BDOS, stdout=%q PC=0 SP=fe00 full RAM guard PASS", mode.name, clobber, len(image.Binary), z.GetCycles(), output)
			}
		})
	}
}
