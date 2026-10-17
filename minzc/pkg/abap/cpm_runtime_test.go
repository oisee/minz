package abap

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestABAPGuardedCPMFullReports(t *testing.T) {
	for _, name := range []string{"unbounded", "limit"} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile("testdata/loop-reference/" + name + ".abap")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("testdata/loop-reference/" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var ref struct {
				SHA    string           `json:"source_sha256"`
				Entry  string           `json:"entry"`
				Values map[string]int32 `json:"values"`
			}
			if err := json.Unmarshal(data, &ref); err != nil {
				t.Fatal(err)
			}
			if ref.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || ref.Entry != "full report" || len(ref.Values) != 3 {
				t.Fatal("reference identity changed")
			}
			m, err := Compile(string(src), "zref")
			if err != nil {
				t.Fatal(err)
			}
			opts := pipeline.DefaultOptions()
			opts.AssertMode = "none"
			opts.Runtime = mir2.Z80RuntimeCPM
			opts.RuntimeStackBytes = 1024
			steps, err := pipeline.CompileHIRSteps(m, opts)
			if err != nil {
				t.Fatal(err)
			}
			if steps.MIR2Module.FuncByName("sel_show") != nil {
				t.Fatal("unused runtime helper retained")
			}
			a, err := z80asm.NewAssembler().AssembleString(steps.Assembly)
			if err != nil || len(a.Errors) > 0 {
				t.Fatalf("assemble: %v %v", err, a.Errors)
			}
			if 0x100+len(a.Binary) >= 0x7000 {
				t.Fatal("image overlaps harness monitor")
			}
			if dir := os.Getenv("MINZ_ABAP_CPM_ARTIFACTS"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for ext, b := range map[string][]byte{"abap": src, "a80": []byte(steps.Assembly), "com": a.Binary, "json": data} {
					if err := os.WriteFile(filepath.Join(dir, name+"."+ext), b, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xA5}, 65536))
			copy(mem[0x100:], a.Binary)
			copy(mem[0x7000:], []byte{0xCD, 0, 1, 0})
			before := append([]byte(nil), mem...)
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xF000})
			for i := 0; i < 10000000 && z.GetPC() != 0x7003 && !z.IsHalted(); i++ {
				pc := z.GetPC()
				if pc != 0x7000 && (pc < 0x100 || int(pc) >= 0x100+len(a.Binary)) {
					t.Fatalf("foreign fetch %04x", pc)
				}
				z.Step()
			}
			if z.GetPC() != 0x7003 || z.GetSP() != 0xF000 {
				t.Fatalf("return PC=%04x SP=%04x", z.GetPC(), z.GetSP())
			}
			if binary.LittleEndian.Uint16(mem[0xEFFE:0xF000]) != 0x7003 {
				t.Fatal("outer return word changed")
			}
			for i := range mem {
				if i >= 0x100 && i < 0x100+len(a.Binary) || i >= 0xEFFE && i < 0xF000 {
					continue
				}
				if mem[i] != before[i] {
					t.Fatalf("foreign write %04x", i)
				}
			}
			for field, want := range ref.Values {
				addr, ok := a.Symbols[field]
				if !ok {
					t.Fatal("missing global", field)
				}
				if got := int32(binary.LittleEndian.Uint32(mem[addr : addr+4])); got != want {
					t.Fatalf("%s=%d want %d", field, got, want)
				}
			}
			if mem[a.Symbols["_abap_error_code"]] != 0 || mem[a.Symbols["__cpm_status"]] != 0 {
				t.Fatal("runtime fault")
			}
		})
	}
}
