package main

import (
	"bytes"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestPascalCPMOrdinaryCLI(t *testing.T) {
	hirProfileTestState(t)
	old, stack := runtimeMode, runtimeStackBytes
	oldAssertions := assertMode
	t.Cleanup(func() { runtimeMode = old; runtimeStackBytes = stack; assertMode = oldAssertions })
	runtimeMode = "cpm"
	runtimeStackBytes = 1024
	assertMode = "none"
	path := "../../../examples/pascal/hello.pas"
	dir := t.TempDir()
	outputFile = filepath.Join(dir, "hello.com")
	if e := compile(path); e != nil {
		t.Fatal(e)
	}
	bin, e := os.ReadFile(outputFile)
	if e != nil {
		t.Fatal(e)
	}
	outputFile = filepath.Join(dir, "hello.a80")
	if e := compile(path); e != nil {
		t.Fatal(e)
	}
	asm, e := os.ReadFile(outputFile)
	if e != nil {
		t.Fatal(e)
	}
	image, e := z80asm.NewAssembler().AssembleString(string(asm))
	if e != nil || len(image.Errors) > 0 || !bytes.Equal(bin, image.Binary) {
		t.Fatal("exact COM/ASM", e)
	}
	outputFile = filepath.Join(dir, "bad.com")
	if e := compile("../../../examples/pascal/factorial.pas"); e == nil {
		t.Fatal("unsupported decimal runtime admitted")
	}
	if _, e := os.Stat(outputFile); !os.IsNotExist(e) {
		t.Fatal("refused runtime wrote output")
	}
	for _, path := range []string{"source.lizp", "source.c"} {
		if e := validateCLIRuntime(path); e == nil {
			t.Fatal("unsupported frontend admitted")
		}
	}
}
