package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func outputTestState(t *testing.T) {
	t.Helper()
	out, be, mode, force, emit, overwrite := outputFile, backend, assertMode, assertForce, emitFormat, forceOutput
	t.Cleanup(func() {
		outputFile, backend, assertMode, assertForce, emitFormat, forceOutput = out, be, mode, force, emit, overwrite
	})
	outputFile, backend, assertMode, assertForce, emitFormat, forceOutput = "", "z80", "all", "", "", false
}

func TestDefaultMIR2AssertAssembly(t *testing.T) {
	outputTestState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "default.nanz")
	if err := os.WriteFile(source, []byte("fun id(x:u8)->u8 { return x }\nassert id(42) == 42 via mir2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := compileViaHIR(source); err != nil {
		t.Fatal(err)
	}
	implicit, err := os.ReadFile(filepath.Join(dir, "default.a80"))
	if err != nil {
		t.Fatal(err)
	}
	outputFile = filepath.Join(dir, "explicit.a80")
	if err := compileViaHIR(source); err != nil {
		t.Fatal(err)
	}
	explicit, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(implicit) == 0 || !bytes.Equal(implicit, explicit) {
		t.Fatal("default assembly differs from explicit assembly")
	}
}

func TestIndirectMIR2DefaultWarning(t *testing.T) {
	outputTestState(t)
	src, err := os.ReadFile("../../../examples/c89/func_ptr.c")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "func_ptr.c")
	if err := os.WriteFile(source, src, 0644); err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	capture, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Close()
	os.Stderr = capture
	defer func() { os.Stderr = stderr }()
	err = compileViaHIR(source)
	os.Stderr = stderr
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	warning, err := io.ReadAll(capture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(warning), "Z80 output skipped because of the unsupported indirect-call ABI") {
		t.Fatalf("missing warning: %s", warning)
	}
	if _, err := os.Stat(strings.TrimSuffix(source, ".c") + ".a80"); !os.IsNotExist(err) {
		t.Fatal("unsupported default wrote assembly")
	}
}

func TestCDefaultOutputProtection(t *testing.T) {
	outputTestState(t)
	backend = "c"
	dir := t.TempDir()
	source := filepath.Join(dir, "swap.nanz")
	original := filepath.Join(dir, "swap.c")
	generated := filepath.Join(dir, "swap.generated.c")
	if err := os.WriteFile(source, []byte("fun id(x:u8)->u8 { return x }"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("tracked source"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := compileViaHIR(source); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != "tracked source" {
		t.Fatalf("source overwritten: %q, %v", data, err)
	}
	if data, err := os.ReadFile(generated); err != nil || !strings.Contains(string(data), "#include") {
		t.Fatalf("missing generated C: %v", err)
	}
	if err := os.WriteFile(generated, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := compileViaHIR(source); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("want overwrite refusal: %v", err)
	}
	if data, _ := os.ReadFile(generated); string(data) != "keep me" {
		t.Fatal("existing output overwritten")
	}
	forceOutput = true
	if err := compileViaHIR(source); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(generated); !strings.Contains(string(data), "#include") {
		t.Fatal("--force did not replace output")
	}
}
