package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func agonZ80CLIState(t *testing.T) string {
	t.Helper()
	hirProfileTestState(t)
	oldRuntime, oldStack, oldFormat := runtimeMode, runtimeStackBytes, outputFormat
	t.Cleanup(func() { runtimeMode, runtimeStackBytes, outputFormat = oldRuntime, oldStack, oldFormat })
	target, backend, runtimeMode, runtimeStackBytes, assertMode = "agon", "z80", "agon-z80", 1024, "none"
	dir := t.TempDir()
	src := filepath.Join(dir, "hello.abap")
	if err := os.WriteFile(src, []byte("REPORT zagon. DATA n TYPE i VALUE 70000. WRITE n.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	outputFile = filepath.Join(dir, "hello.bin")
	return src
}

func TestAgonZ80RuntimeCLI(t *testing.T) {
	src := agonZ80CLIState(t)
	if err := compile(src); err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(image) < 70 || string(image[64:69]) != "MOS\x00\x00" || image[0] != 0xC3 || image[1] != 69 || image[2] != 0 {
		t.Fatal("ordinary runtime CLI did not produce native mode0 package")
	}
}

func TestAgonZ80RuntimeCLIRefusals(t *testing.T) {
	for _, name := range []string{"platform", "backend", "odd stack", "huge stack", "foreign format", "assembler target alias"} {
		t.Run(name, func(t *testing.T) {
			src := agonZ80CLIState(t)
			switch name {
			case "platform":
				target = "cpm"
			case "backend":
				backend = "ez80"
			case "odd stack":
				runtimeStackBytes = 63
			case "huge stack":
				runtimeStackBytes = int(^uint(0) >> 1)
			case "foreign format":
				outputFormat = "com"
			case "assembler target alias":
				target, runtimeMode = "agon-z80", ""
			}
			if err := compile(src); err == nil || !strings.Contains(strings.ToLower(err.Error()), "agon-z80") {
				t.Fatalf("invalid native runtime combination accepted/wrong diagnostic: %v", err)
			}
			if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
				t.Fatal("refused combination wrote output")
			}
		})
	}
}

func TestAgonZ80RuntimeCLIImageOverflow(t *testing.T) {
	src := agonZ80CLIState(t)
	if err := os.WriteFile(src, []byte(`REPORT zlarge.
TYPES: BEGIN OF ty_row, value TYPE i, END OF ty_row.
DATA one TYPE STANDARD TABLE OF ty_row INITIAL SIZE 8192.
DATA two TYPE STANDARD TABLE OF ty_row INITIAL SIZE 8192.
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := compile(src); err == nil || !strings.Contains(strings.ToLower(err.Error()), "agon-z80") {
		t.Fatalf("oversized bank image admitted: %v", err)
	}
	if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
		t.Fatal("oversized bank image was written")
	}
}
