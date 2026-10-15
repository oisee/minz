//go:build mzx_headless

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/spectrum/formats"
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

func TestEmbeddedDemoBoots(t *testing.T) {
	for _, d := range demoCatalog {
		t.Run(d.ID, func(t *testing.T) {
			m, _, err := loadDemo(d.ID, t.TempDir(), embedded48ROM)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 120; i++ {
				m.RunFrame()
			}
			if d.ID == "tetris" {
				if m.Memory.Read(0x584a)&7 != 7 {
					t.Fatal("Tetris wall must use visible white INK")
				}
			}
		})
	}
}

// Test the embedded image, not a separate source build. Old menu keys must
// leave automatic half-pixel-per-frame descent unchanged. Full source/ULA
// matrices establish the later line-driven stages and lock/clear behavior.
func TestEmbeddedTetrisDifficulty(t *testing.T) {
	for _, key := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("automatic-key-%d", key), func(t *testing.T) {
			m, _, err := loadDemo("tetris", t.TempDir(), embedded48ROM)
			if err != nil {
				t.Fatal(err)
			}
			if key != 0 {
				m.Keyboard.KeyPress(3, key-1)
			}
			for i := 0; i < 4; i++ {
				m.RunFrame()
			}
			if key != 0 {
				m.Keyboard.KeyRelease(3, key-1)
			}
			for i := 0; i < 16; i++ {
				m.RunFrame()
			}
			// No text at the former two-line difficulty menu.
			for y := 0; y < 16; y++ {
				for x := 0; x < 32; x++ {
					address := uint16(0x4000 + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + x)
					if m.Memory.ReadByteInternal(address) != 0 {
						t.Fatalf("text or pixels in former menu at %04x", address)
					}
				}
			}
			for row := 3; row < 23; row++ {
				want := byte(1)
				if row >= 16 {
					want = byte(65 + row - 16)
				}
				if got := m.Memory.ReadByteInternal(uint16(0x5800 + row*32 + 8)); got != want {
					t.Fatalf("gauge row %d attribute %d want %d", row, got, want)
				}
			}
			for y := 24; y < 184; y++ {
				address := uint16(0x4000 + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + 8)
				if m.Memory.ReadByteInternal(address) != 0 {
					t.Fatal("initial gauge should be empty")
				}
			}
			for y := 16; y < 184; y++ {
				address := uint16(0x4000 + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + 9)
				if m.Memory.ReadByteInternal(address) != 0 {
					t.Fatal("gauge/well separator must stay black")
				}
			}
			top := func() int {
				for y := 16; y < 176; y++ {
					for x := 11; x < 21; x++ {
						address := uint16(0x4000 + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + x)
						if m.Memory.ReadByteInternal(address) != 0 {
							return y
						}
					}
				}
				t.Fatal("no falling figure in embedded Tetris")
				return -1
			}
			first, previous := top(), top()
			for i := 0; i < 16; i++ {
				m.RunFrame()
				now := top()
				if delta := now - previous; delta < 0 || delta > 1 {
					t.Fatalf("frame %d moved %d pixels", i, delta)
				}
				previous = now
			}
			if got := top() - first; got != 8 {
				t.Fatalf("16-frame descent=%d pixels want 8", got)
			}
		})
	}
}
