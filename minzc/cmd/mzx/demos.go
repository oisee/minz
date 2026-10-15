package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
)

//go:embed demos/*
var demoFiles embed.FS

type demoSpec struct{ ID, Title, Model, Help string }

var demoCatalog = []demoSpec{
	{"tetris", "Spectrum Tetris", "48k", "1/2: speed  O/P: move  Q: rotate  A: drop  Space: hard drop  H: hold"},
	{"zork1", "Zork I", "128k", "Enter commands. Session resumes on next launch; SAVE is in RAM."},
	{"zork2", "Zork II", "scorpion-memory", "Enter commands. SAVE / RESTORE use a persistent disk."},
	{"rotozoomer", "32 x 48 colour rotozoomer", "48k", "Animated attribute graphics; Escape returns to the menu."},
	{"sprites", "Baked Nanz sprites", "48k", "Whole sprites prepared before each frame; Escape returns to the menu."},
	{"sprite-wave", "Baked Nanz TV wave", "48k", "Typed ByteWave effect, prepared before each frame; Escape returns to the menu."},
	{"sprites-push", "PUSH SMC sprites", "48k", "Stack-writing sprite kernel; Escape returns to the menu."},
}

func findDemo(id string) (demoSpec, error) {
	for _, d := range demoCatalog {
		if d.ID == id {
			return d, nil
		}
	}
	return demoSpec{}, fmt.Errorf("unknown demo %q; use --list-demos", id)
}
func demoDataDir(root, id string) (string, error) {
	if root == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(base, "minz", "mzx")
	}
	dir := filepath.Join(root, id)
	return dir, os.MkdirAll(dir, 0700)
}

// Publish a new disk without ever replacing an existing user's SAVE.
func ensureDemoDisk(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() || info.Size() != formats.TRDDiskSize {
			return fmt.Errorf("invalid existing SAVE disk %s; preserved without modification", path)
		}
		return nil
	}
	if err != nil {
		return err
	}
	data := make([]byte, formats.TRDDiskSize)
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		os.Remove(path)
		return err
	}
	return closeErr
}
func loadDemo(id, root string, rom []byte) (*spectrum.Machine, string, error) {
	d, err := findDemo(id)
	if err != nil {
		return nil, "", err
	}
	var m *spectrum.Machine
	switch d.Model {
	case "128k":
		m, err = spectrum.NewPentagon128(rom, rom)
	case "scorpion-memory":
		m, err = spectrum.NewScorpionMemory256(rom, rom)
	default:
		m, err = spectrum.New48K(rom)
	}
	if err != nil {
		return nil, "", err
	}
	data, err := demoFiles.ReadFile("demos/" + id + ".sna")
	if err != nil {
		return nil, "", err
	}
	dir, err := demoDataDir(root, id)
	if err != nil {
		return nil, "", err
	}
	resume := ""
	if id == "zork1" {
		resume = filepath.Join(dir, "session.sna")
		saved, readErr := os.ReadFile(resume)
		if readErr == nil {
			data = saved
		} else if !os.IsNotExist(readErr) {
			return nil, "", readErr
		}
	}
	snap, err := formats.ParseSNA(data)
	if err != nil {
		return nil, "", err
	}
	formats.ApplySnapshot(m, snap)
	if id == "zork2" {
		bank, err := demoFiles.ReadFile("demos/zork2-bank10.bin")
		if err != nil {
			return nil, "", err
		}
		for i, b := range bank {
			m.Memory.WriteRAMDirect(10, uint16(i), b)
		}
		diskPath := filepath.Join(dir, "save.trd")
		if err := ensureDemoDisk(diskPath); err != nil {
			return nil, "", err
		}
		disk, err := formats.LoadTRD(diskPath)
		if err != nil {
			return nil, "", err
		}
		disk.Writeback = true
		formats.InstallTRDTraps(m, disk, false)
	}
	return m, resume, nil
}
func saveDemoSession(m *spectrum.Machine, path string) error {
	if path == "" {
		return nil
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".session-*.sna")
	if err != nil {
		return err
	}
	temp := f.Name()
	f.Close()
	defer os.Remove(temp)
	if err := formats.SaveSNA(temp, m); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
func listDemos() {
	for _, d := range demoCatalog {
		fmt.Printf("%-14s %s (%s)\n", d.ID, d.Title, d.Model)
	}
}
