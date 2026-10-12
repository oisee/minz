package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCompilerModeArtifactPublication(t *testing.T) {
	t.Run("complete-and-exact", func(t *testing.T) {
		dir := t.TempDir()
		dest, out := filepath.Join(dir, "evidence"), filepath.Join(dir, "out.bin")
		if err := os.WriteFile(out, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		payloads := map[string][]byte{"manifest.json": []byte("manifest"), "mode-0/target.bin": {1, 2, 3}, "selected.bin": {1, 2, 3}}
		if err := publishCompilerModeArtifacts(dest, payloads, out, []byte{1, 2, 3}); err != nil {
			t.Fatal(err)
		}
		for name, want := range payloads {
			got, err := os.ReadFile(filepath.Join(dest, name))
			if err != nil || !bytes.Equal(got, want) {
				t.Fatal(name, err)
			}
		}
		got, err := os.ReadFile(out)
		if err != nil || !bytes.Equal(got, []byte{1, 2, 3}) {
			t.Fatal("output", err)
		}
		leftovers, _ := filepath.Glob(filepath.Join(dir, ".compiler-modes-*"))
		if len(leftovers) != 0 {
			t.Fatal(leftovers)
		}
		if err := publishCompilerModeArtifacts(dest, map[string][]byte{"selected.bin": {9}}, out, []byte{9}); err == nil {
			t.Fatal("existing evidence replaced")
		}
		got, _ = os.ReadFile(out)
		if !bytes.Equal(got, []byte{1, 2, 3}) {
			t.Fatal("existing output changed")
		}
	})
	t.Run("stage-failure", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "evidence")
		out := filepath.Join(dir, "out.bin")
		os.WriteFile(out, []byte("old"), 0644)
		// Both names pass independent name checks, but conflict inside the stage.
		err := publishCompilerModeArtifacts(dest, map[string][]byte{"manifest.json": {}, "x": {}, "x/y": {}}, out, []byte("new"))
		if err == nil {
			t.Fatal("stage failure accepted")
		}
		if _, err := os.Lstat(dest); !os.IsNotExist(err) {
			t.Fatal("partial final directory", err)
		}
		got, _ := os.ReadFile(out)
		if string(got) != "old" {
			t.Fatal("output changed")
		}
		leftovers, _ := filepath.Glob(filepath.Join(dir, ".compiler-modes-*"))
		if len(leftovers) != 0 {
			t.Fatal(leftovers)
		}
	})
	t.Run("output-stage-failure", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "evidence")
		err := publishCompilerModeArtifacts(dest, map[string][]byte{"manifest.json": {}}, filepath.Join(dir, "missing", "out.bin"), []byte("new"))
		if err == nil {
			t.Fatal("output stage failure accepted")
		}
		if _, err := os.Lstat(dest); !os.IsNotExist(err) {
			t.Fatal("published before staging output", err)
		}
	})
}

func TestCompilerModeDirectoryNoReplace(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, "stage")
	dest := filepath.Join(dir, "existing")
	os.Mkdir(stage, 0755)
	os.WriteFile(filepath.Join(stage, "selected.bin"), []byte("new"), 0644)
	os.Mkdir(dest, 0755)
	if err := publishCompilerModeDirectory(stage, dest); err == nil {
		t.Fatal("existing empty directory replaced")
	}
	if _, err := os.Stat(filepath.Join(stage, "selected.bin")); err != nil {
		t.Fatal("stage lost", err)
	}
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 0 {
		t.Fatal("destination changed", err)
	}
}

func TestCompilerModeDirectoryPathAliases(t *testing.T) {
	compilerModeTestState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "source.nanz")
	os.WriteFile(source, []byte("source"), 0644)
	for _, kind := range []string{"output-inside", "output-parent", "source-parent", "existing-empty", "symlink-parent"} {
		t.Run(kind, func(t *testing.T) {
			oldDir, oldOut := hirProfileAutoDirectory, outputFile
			defer func() { hirProfileAutoDirectory, outputFile = oldDir, oldOut }()
			hirProfileAutoDirectory = filepath.Join(t.TempDir(), "evidence")
			switch kind {
			case "output-inside":
				outputFile = filepath.Join(hirProfileAutoDirectory, "custom.bin")
			case "output-parent":
				outputFile = filepath.Dir(hirProfileAutoDirectory)
			case "source-parent":
				hirProfileAutoDirectory = source + "/evidence"
			case "existing-empty":
				os.Mkdir(hirProfileAutoDirectory, 0755)
			case "symlink-parent":
				link := filepath.Join(t.TempDir(), "link")
				os.Symlink(filepath.Dir(hirProfileAutoDirectory), link)
				outputFile = filepath.Join(link, "evidence", "custom.bin")
			}
			if err := validateCompilerModePaths(hirProfileAutoDirectory, []string{source}, compilerModeOutputNames()); err == nil {
				t.Fatal("path alias accepted")
			}
		})
	}
}
