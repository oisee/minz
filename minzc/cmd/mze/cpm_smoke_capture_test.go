package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Test-only capture of the ordinary MZE CP/M environment. No alternate BDOS
// callback, permissive memory hook, compiler transform or output adapter.
func TestCPMMainCapture(t *testing.T) {
	comPath := os.Getenv("MINZ_CPM_MAIN_COM")
	dir := os.Getenv("MINZ_CPM_MAIN_ARTIFACTS")
	if comPath == "" || dir == "" {
		t.Skip("reviewed COM/artifact paths required")
	}
	image, e := os.ReadFile(comPath)
	if e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(image)) != "087a9c7376591a82169b963891493ab6389cf26ebab728ef08a63e7664c73112" {
		t.Fatal("COM identity mismatch")
	}
	if len(image) != 2574 {
		t.Fatal("COM extent mismatch")
	}
	z := emulator.NewRemogattoZ80WithScreen()
	setExecutionLimit(z.RemogattoZ80, 10000000)
	setupCPMBDOS(z)
	z.LoadAt(0x100, image)
	z.SetPC(0x100)
	z.WriteMemory(0xfdfe, 0)
	z.WriteMemory(0xfdff, 0)
	z.SetSP(0xfdfe)
	initial := z.GetRegisters()
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	outputCh := make(chan []byte, 1)
	go func() { data, _ := io.ReadAll(r); outputCh <- data }()
	runErr := z.Execute()
	w.Close()
	os.Stdout = originalStdout
	output := <-outputCh
	r.Close()
	final := z.GetRegisters()
	memory := make([]byte, 65536)
	for i := range memory {
		memory[i] = z.ReadMemory(uint16(i))
	}
	if e = os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	write := func(n string, data []byte) {
		if e := os.WriteFile(filepath.Join(dir, n), data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	const input = "(fun add ((a u8) (b u8)) u8 (return (+ a b)))"
	expected := []byte("Input: " + input + "\nParsed: " + input + "\nNodes: 17 Strings: 24 bytes\n")
	write("program.com", image)
	write("stdout.bin", output)
	write("expected.bin", expected)
	write("memory.bin", memory)
	record := struct {
		Initial, Final emulator.Registers
		Cycles         int
		Error          string
		ExitCode       uint16
		COMSHA256      string
		MemoryBytes    int
	}{initial, final, z.GetCycles(), fmt.Sprint(runErr), z.GetExitCode(), fmt.Sprintf("%x", sha256.Sum256(image)), len(memory)}
	data, e := json.MarshalIndent(record, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	write("execution.json", data)
	if runErr != nil || !bytes.Equal(output, expected) || final.PC != 0 || final.SP != 0xfdfe {
		t.Fatalf("stdout%q PC/SP%04x/%04x err%v", output, final.PC, final.SP, runErr)
	}
	// Status/arena/canary bytes use separately resolved final assembly symbols
	// in the saved-only auditor. Full RAM is captured before every assertion.
}
