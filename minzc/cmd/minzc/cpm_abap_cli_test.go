package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestABAPCPMOrdinaryCLI(t *testing.T) {
	hirProfileTestState(t)
	old, stack, assertions := runtimeMode, runtimeStackBytes, assertMode
	t.Cleanup(func() { runtimeMode = old; runtimeStackBytes = stack; assertMode = assertions })
	runtimeMode = "cpm"
	runtimeStackBytes = 1024
	assertMode = "none"
	t.Setenv("PATH", t.TempDir())
	for _, name := range []string{"unbounded", "limit"} {
		t.Run(name, func(t *testing.T) {
			source := "../../pkg/abap/testdata/loop-reference/" + name + ".abap"
			src, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(strings.TrimSuffix(source, ".abap") + ".json")
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
				t.Fatal("reference identity")
			}
			dir := t.TempDir()
			outputFile = filepath.Join(dir, name+".com")
			if err := compile(source); err != nil {
				t.Fatal(err)
			}
			bin, err := os.ReadFile(outputFile)
			if err != nil {
				t.Fatal(err)
			}
			outputFile = filepath.Join(dir, name+".a80")
			if err := compile(source); err != nil {
				t.Fatal(err)
			}
			text, err := os.ReadFile(outputFile)
			if err != nil {
				t.Fatal(err)
			}
			a, err := z80asm.NewAssembler().AssembleString(string(text))
			if err != nil || len(a.Errors) != 0 || !bytes.Equal(bin, a.Binary) {
				t.Fatalf("CLI COM/ASM mismatch: %v %v", err, a.Errors)
			}
			if 0x100+len(bin) >= 0x7000 {
				t.Fatal("image overlaps monitor")
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x100:], bin)
			copy(mem[0x7000:], []byte{0xcd, 0, 1, 0})
			before := append([]byte(nil), mem...)
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
			for n := 0; n < 10000000 && z.GetPC() != 0x7003 && !z.IsHalted(); n++ {
				pc := z.GetPC()
				if pc != 0x7000 && (pc < 0x100 || int(pc) >= 0x100+len(bin)) {
					t.Fatalf("foreign fetch %04x", pc)
				}
				z.Step()
			}
			if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 {
				t.Fatalf("return PC=%04x SP=%04x", z.GetPC(), z.GetSP())
			}
			if binary.LittleEndian.Uint16(mem[0xeffe:0xf000]) != 0x7003 {
				t.Fatal("saved return word")
			}
			for i, b := range mem {
				if i >= 0x100 && i < 0x100+len(bin) || i >= 0xeffe && i < 0xf000 {
					continue
				}
				if b != before[i] {
					t.Fatalf("foreign write %04x", i)
				}
			}
			for field, want := range ref.Values {
				addr, ok := a.Symbols[field]
				if !ok {
					t.Fatal("missing", field)
				}
				if got := int32(binary.LittleEndian.Uint32(mem[addr : addr+4])); got != want {
					t.Fatalf("%s=%d want %d", field, got, want)
				}
			}
			for _, field := range []string{"_abap_error_code", "__cpm_status"} {
				if mem[a.Symbols[field]] != 0 {
					t.Fatal("runtime fault", field)
				}
			}
		})
	}
	outputFile = filepath.Join(t.TempDir(), "unsupported.com")
	if err := compile("../../pkg/abap/testdata/character-reference/copy-report.abap"); err == nil || !strings.Contains(err.Error(), "CP/M") {
		t.Fatalf("unadmitted pointer runtime accepted: %v", err)
	}
	if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
		t.Fatal("refused runtime wrote image")
	}
}
