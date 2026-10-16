package nanz_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// C90 6.7.1 requires a by-value parameter copy. For Nanz this test records
// current observable shared-object behavior; Book v7 chapter 4 specifies
// field access but does not promise C's aggregate parameter copy semantics.
func TestAggregateParameterWriteABI(t *testing.T) {
	var manifest struct {
		Cases []struct {
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		} `json:"cases"`
	}
	frozen, err := os.ReadFile("testdata/aggregate_abi/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(frozen, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, c := range manifest.Cases {
		data, err := os.ReadFile(filepath.Join("testdata/aggregate_abi", c.File))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != c.SHA256 {
			t.Fatal("frozen aggregate fixture changed", c.File, err)
		}
	}
	rows := []map[string]any{}
	for _, language := range []string{"c", "nanz"} {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(language+"/"+mode, func(t *testing.T) {
				src, err := os.ReadFile(filepath.Join("testdata/aggregate_abi", "parameter_write."+language))
				if err != nil {
					t.Fatal(err)
				}
				var h *hir.Module
				if language == "c" {
					h, err = c89.CompileWithOpts(string(src), "parameter_write.c", c89.CompileOpts{NoLibc: true})
				} else {
					h, err = nanz.Parse(string(src), "parameter_write.nanz")
				}
				if err != nil {
					t.Fatal(err)
				}
				raw := hir.LowerModule(h)
				s, err := pipeline.CompileHIRSteps(h, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
				if err != nil {
					t.Fatal(err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				if len(a.Errors) > 0 {
					t.Fatal(a.Errors)
				}
				addr := func(name string) int {
					p := a.Symbols[name]
					if p == 0 {
						p = a.Symbols["v_"+name]
					}
					if p == 0 {
						t.Fatal(name)
					}
					return p
				}
				input, output, entry := addr("a"), addr("result"), addr("execute")
				vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				total := 0
				for x := 0; x < 256; x++ {
					changed := byte(x + 1)
					hi := byte(x)
					if language == "nanz" {
						hi = changed
					}
					want := uint16(hi)<<8 | uint16(changed)
					for _, v := range vms {
						v.WriteHeapSlice(v.GlobalAddr("a"), []byte{byte(x)})
						if _, err := v.Call("execute", nil); err != nil {
							t.Fatal(err)
						}
						got := binary.LittleEndian.Uint16(v.ReadHeap(v.GlobalAddr("result"), 2))
						if got != want || v.ReadHeap(v.GlobalAddr("a"), 1)[0] != byte(x) {
							t.Fatal("VM", language, mode, x, got, want)
						}
					}
					mem[input] = byte(x)
					mem[output], mem[output+1] = 0xa5, 0x5a
					mem[0xeffe], mem[0xefff] = 0, 0xf1
					z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
					start := z.GetCycles()
					for n := 0; n < 10000 && z.GetPC() != 0xf100; n++ {
						z.Step()
					}
					r := z.GetRegisters()
					got := binary.LittleEndian.Uint16(mem[output : output+2])
					if got != want || r.PC != 0xf100 || r.SP != 0xf000 || mem[input] != byte(x) {
						t.Fatal("Z80", language, mode, x, got, want, r)
					}
					total += z.GetCycles() - start
				}
				rows = append(rows, map[string]any{"language": language, "mode": mode, "status": "PASS", "native_calls": 256, "raw_final_vm_calls": 512, "module_bytes": len(a.Binary), "T": total, "parameter_write_observed_by_caller": language == "nanz"})
				if dir := os.Getenv("MINZ_AGGREGATE_ABI_AUDIT_DIR"); dir != "" {
					d := filepath.Join(dir, language, mode)
					if err := os.MkdirAll(d, 0755); err != nil {
						t.Fatal(err)
					}
					for name, data := range map[string][]byte{"source": src, "raw.mir": []byte(raw.Dump()), "final.mir": []byte(s.MIR2Module.Dump()), "target.a80": []byte(s.Assembly), "target.bin": a.Binary} {
						if err := os.WriteFile(filepath.Join(d, name), data, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
			})
		}
	}
	if dir := os.Getenv("MINZ_AGGREGATE_ABI_AUDIT_DIR"); dir != "" {
		b, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "summary.json"), append(b, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
