//go:build mzx_headless

package main

import (
	"bytes"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"os"
	"path/filepath"
	"testing"
)

func TestDemoDiskSurvivesRelaunch(t *testing.T) {
	root := t.TempDir()
	_, _, err := loadDemo("zork2", root, embedded48ROM)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "zork2", "save.trd")
	data := make([]byte, formats.TRDDiskSize)
	copy(data[4096:], "user SAVE survives updates")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err = loadDemo("zork2", root, embedded48ROM)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("disk replaced: %v", err)
	}
}
func TestDemoSessionResumes(t *testing.T) {
	root := t.TempDir()
	m, path, err := loadDemo("zork1", root, embedded48ROM)
	if err != nil {
		t.Fatal(err)
	}
	m.Memory.WriteRAMDirect(3, 100, 0xA7)
	if err := saveDemoSession(m, path); err != nil {
		t.Fatal(err)
	}
	restored, _, err := loadDemo("zork1", root, embedded48ROM)
	if err != nil {
		t.Fatal(err)
	}
	if got := restored.Memory.ReadRAMDirect(3, 100); got != 0xA7 {
		t.Fatalf("saved byte = %x", got)
	}
}
