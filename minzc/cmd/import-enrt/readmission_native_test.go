package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestReadmissionCLIWeightedNative(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	importer, compiler := filepath.Join(dir, "importer"), filepath.Join(dir, "mz")
	run("go", "build", "-o", importer, ".")
	run("go", "build", "-o", compiler, "../minzc")
	// The frozen harness returns `heavy + light`, whose constant RHS is served
	// from an immediate and rematerialised (F6).  That moves weighted's
	// parameter off A, which leaves every proposal in the frozen table
	// inadmissible — the op chain starts with x+x, expressible only from A.
	// Keep the constant on the left so the saved allocation is still a valid
	// proposal for the corpus it was measured on.
	harness, err := os.ReadFile("../../../docs/grace-real-leaf-local-tables/caller-harness.nanz")
	if err != nil {
		t.Fatal(err)
	}
	swapped := strings.Replace(string(harness), "return heavy + light", "return light + heavy", 1)
	if swapped == string(harness) {
		t.Fatal("frozen harness no longer contains 'return heavy + light'")
	}
	source := filepath.Join(dir, "caller-harness.nanz")
	if err := os.WriteFile(source, []byte(swapped), 0644); err != nil {
		t.Fatal(err)
	}
	proposals := "../../../docs/grace-real-leaf-local-tables/caller-table.json"
	table, evidence, assembly := filepath.Join(dir, "table.json"), filepath.Join(dir, "evidence.json"), filepath.Join(dir, "selected.a80")
	run(importer, "--source", source, "--checked-proposals", proposals, "--proposal-limit", "1", "--out", table, "--evidence", evidence)
	data, err := os.ReadFile(table)
	if err != nil {
		t.Fatal(err)
	}
	pin := fmt.Sprintf("%x", sha256.Sum256(data))
	run(compiler, source, "--Osize", "--checked-local-table", table, "--checked-local-table-sha256", pin, "-o", assembly)
	asm, err := os.ReadFile(assembly)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(asm), "checked-local-table=1") || !strings.Contains(string(asm), "weighted_caller(x: u8 = B) -> u8 = A") {
		t.Fatal("table not consumed")
	}
	bin, err := z80asm.NewAssembler().AssembleString(string(asm))
	if err != nil || len(bin.Errors) != 0 {
		t.Fatal(err, bin.Errors)
	}
	addr, ok := bin.Symbols["weighted_caller"]
	if !ok {
		addr, ok = bin.Symbols["WEIGHTED_CALLER"]
	}
	if !ok {
		t.Fatal("missing caller symbol")
	}
	for x := 0; x < 256; x++ {
		z := emulator.NewRemogattoZ80()
		if err := z.LoadMemory(uint16(bin.Origin), bin.Binary); err != nil {
			t.Fatal(err)
		}
		if err := z.LoadMemory(0xf100, []byte{0x06, byte(x), 0xcd, byte(addr), byte(addr >> 8)}); err != nil {
			t.Fatal(err)
		}
		z.SetPC(0xf100)
		z.SetSP(0xf000)
		for steps := 0; steps < 128 && z.GetRegisters().PC != 0xf105; steps++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.A != byte(11*x+1) || r.PC != 0xf105 || r.SP != 0xf000 {
			t.Fatal(x, r)
		}
	}
	// A mutated input must fail before publishing a product.
	if err := os.WriteFile(table, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	badOut := filepath.Join(dir, "mutated.a80")
	cmd := exec.Command(compiler, source, "--Osize", "--checked-local-table", table, "--checked-local-table-sha256", pin, "-o", badOut)
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "SHA256 mismatch") {
		t.Fatal("mutation accepted", err, string(out))
	}
	if _, err := os.Stat(badOut); !os.IsNotExist(err) {
		t.Fatal("mutated product published", err)
	}
	t.Logf("real importer/compiler; %d-byte BIN; all256caller native inputs and content mutation rejection PASS", len(bin.Binary))
}
