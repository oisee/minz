// Run from minzc: go run ../scripts/zmachine/build_spectrum128.go STORY OUTDIR [--compressed]
// The story is supplied separately; it is never downloaded by this builder.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	if len(os.Args) < 3 {
		panic("usage: build_spectrum128.go STORY OUTDIR [--compressed] [--scorpion-disk] [--windows] (run from minzc)")
	}
	story, err := os.ReadFile(os.Args[1])
	must(err)
	if len(story) < 64 || len(story) > 98304 || story[0] != 3 {
		panic("requires Z-machine V3 story, 64..98304 bytes")
	}
	compressed, diskSave, windows := false, false, false
	for _, arg := range os.Args[3:] {
		switch arg {
		case "--compressed":
			compressed = true
		case "--windows":
			windows = true
		case "--scorpion-disk":
			diskSave = true
		default:
			panic("unknown option: " + arg)
		}
	}
	out, err := filepath.Abs(os.Args[2])
	must(err)
	must(os.MkdirAll(out, 0755))
	var layout map[string]any
	var storageImage []byte
	dynamic := int(story[14])<<8 | int(story[15])
	var archive []byte
	archiveOffset, saveOffset, saveCapacity := 0, 0, 0
	restart, saveSupported := false, false
	if compressed {
		pack := exec.Command("python3", "../scripts/zmachine/story_lz_layout.py", os.Args[1], out, "--archive-blocks")
		if data, err := pack.CombinedOutput(); err != nil {
			panic(fmt.Sprintf("layout: %v %s", err, data))
		}
		data, err := os.ReadFile(filepath.Join(out, "layout-2048.json"))
		must(err)
		must(json.Unmarshal(data, &layout))
		storageImage, err = os.ReadFile(filepath.Join(out, "layout-2048.bin"))
		must(err)
		if len(storageImage) != 98304 || layout["story_sha256"] != fmt.Sprintf("%x", sha256.Sum256(story)) {
			panic("layout story identity/image size mismatch")
		}
		bank := int(layout["reserved_save_bank"].(float64))
		for i, page := range []int{0, 1, 3, 4, 6, 7} {
			if page == bank {
				saveOffset = i*16384 + int(layout["reserved_save_offset"].(float64))
			}
		}
		saveCapacity = int(layout["reserved_save_bytes"].(float64))
		restart = true
		saveSupported = saveCapacity >= 32
	} else {
		archiveFile, err := os.CreateTemp("", "z3-restart-*.rle")
		must(err)
		archivePath := archiveFile.Name()
		must(archiveFile.Close())
		defer os.Remove(archivePath)
		pack := exec.Command("python3", "../scripts/zmachine/restart_rle.py", os.Args[1], archivePath)
		if data, err := pack.CombinedOutput(); err != nil {
			panic(fmt.Sprintf("restart pack %v %s", err, data))
		}
		archive, err = os.ReadFile(archivePath)
		must(err)
		// Keep the immutable archive outside mutable bank0 and within one window.
		archiveOffset = len(story)
		if archiveOffset < 16384 {
			archiveOffset = 16384
		}
		if len(archive) > 0 && archiveOffset/16384 != (archiveOffset+len(archive)-1)/16384 {
			archiveOffset = (archiveOffset/16384 + 1) * 16384
		}
		restart = dynamic >= 64 && dynamic <= 16384 && len(archive) > 0 && archiveOffset+len(archive) <= 98304 && archiveOffset/16384 == (archiveOffset+len(archive)-1)/16384

		saveOffset = archiveOffset + len(archive)
		if saveOffset < 5*16384 {
			saveOffset = 5 * 16384
		}
		saveSupported = restart && 98304-saveOffset >= 32
		saveCapacity = 98304 - saveOffset
	}
	if diskSave {
		if !restart {
			panic("disk SAVE requires a valid restart archive")
		}
		saveSupported, saveCapacity = true, 16384
	}
	source := filepath.Join(out, "zvm-spectrum128.nanz")
	cmd := exec.Command("python3", "../scripts/zmachine/prepare_native_core.py", source, "--story-bytes", fmt.Sprint(len(story)), "--console")
	if restart && !compressed {
		cmd.Args = append(cmd.Args, "--restart-offset", fmt.Sprint(archiveOffset), "--restart-bytes", fmt.Sprint(dynamic), "--restart-rle-bytes", fmt.Sprint(len(archive)))
	}
	if saveSupported && !compressed {
		cmd.Args = append(cmd.Args, "--save-story", os.Args[1])
	}
	if compressed {
		cmd.Args = append(cmd.Args, "--story-layout", filepath.Join(out, "layout-2048.json"), "--layout-restart")
		if saveSupported {
			cmd.Args = append(cmd.Args, "--layout-save-story", os.Args[1])
		}
	}
	if windows {
		cmd.Args = append(cmd.Args, "--windows")
	}
	if diskSave {
		cmd.Args = append(cmd.Args, "--disk-save", "--extended-save")
	}
	result, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("generate: %v %s", err, result))
	}
	src, err := os.ReadFile(source)
	must(err)
	h, err := nanz.ParseWithOpts(string(src), source, nanz.ParseOpts{BaseDir: out, StdlibDir: "../stdlib"})
	must(err)
	steps, err := pipeline.CompileHIRSteps(h, pipeline.Options{AssertMode: "none"})
	must(err)
	assembly := "ORG 0x5b00\n" + steps.Assembly
	a, err := z80asm.NewAssembler().AssembleString(assembly)
	must(err)
	if len(a.Errors) > 0 {
		panic(a.Errors)
	}
	if 0x5b00+len(a.Binary) > 0xbe00 {
		panic(fmt.Sprintf("resident %d bytes overlaps native stack reservation", len(a.Binary)))
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	must(err)
	for i, b := range a.Binary {
		m.Memory.Write(0x5b00+uint16(i), b, false)
	}
	pages := []int{0, 1, 3, 4, 6, 7}
	bankImage := story
	if compressed {
		bankImage = storageImage
	}
	for i, b := range bankImage {
		m.Memory.RAM[pages[i/16384]][i%16384] = b
	}
	if restart && !compressed {
		for i, b := range archive {
			at := archiveOffset + i
			m.Memory.RAM[pages[at/16384]][at%16384] = b
		}
	}
	m.Memory.SetPagingForce(0x10)
	// A returned main stays in DI/HALT loop in the unused fixed-RAM tail.
	for i, b := range []byte{0xf3, 0x76, 0x18, 0xfd} {
		m.Memory.Write(0xbf10+uint16(i), b, false)
	}
	m.Memory.Write(0xbf00, 0x10, false)
	m.Memory.Write(0xbf01, 0xbf, false)
	m.CPU.SetPC(uint16(a.Symbols["main"]))
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	must(formats.SaveSNA(filepath.Join(out, "zork-spectrum128.sna"), m))
	// Reload through the emulator's snapshot reader and check all physical banks.
	snap, err := formats.LoadSNA(filepath.Join(out, "zork-spectrum128.sna"))
	must(err)
	restored, err := spectrum.NewPentagon128(nil, nil)
	must(err)
	formats.ApplySnapshot(restored, snap)
	if restored.Memory.RAM != m.Memory.RAM || restored.CPU.PC() != m.CPU.PC() || restored.CPU.SP() != m.CPU.SP() || restored.Memory.PagingState() != 0x10 || restored.CPU.IFF1() {
		panic("snapshot roundtrip mismatch")
	}
	must(os.WriteFile(filepath.Join(out, "zvm-spectrum128.a80"), []byte(assembly), 0644))
	must(os.WriteFile(filepath.Join(out, "zvm-spectrum128.bin"), a.Binary, 0644))
	metadata := map[string]any{"story_sha256": fmt.Sprintf("%x", sha256.Sum256(story)), "story_bytes": len(story), "resident_bytes": len(a.Binary), "origin": 0x5b00, "entry": a.Symbols["main"], "native_stack_top": 0xbf00, "paging": 0x10, "story_banks": pages, "snapshot_roundtrip": "all eight banks and PC/SP/paging verified", "limitations": []string{"RAM save volatile; failed overwrite invalidates prior save; capacity overflow returns failure", "ROM font data at 0x3d00 required", "IRQs disabled; polling keyboard", "short gameplay verified; full game not proven"}}
	if diskSave {
		metadata["machine_model"] = "scorpion-memory"
		metadata["save_staging_bank"] = 8
		metadata["sector_buffer_bank"] = 9
		metadata["save_slot_offset"] = 0
		metadata["limitations"] = []string{"dedicated TRD required; emulator TR-DOS API traps, physical controller not supported", "initial SNA stores original eight banks only; extra banks start empty", "ROM font required; IRQs disabled; full game not proven"}
	}
	metadata["v3_windows"] = windows
	metadata["save_restore_supported"] = saveSupported
	metadata["save_slot_bytes"] = 0
	if saveSupported {
		metadata["save_slot_bytes"] = saveCapacity
		if diskSave {
			metadata["save_slot_offset"] = 0
		} else {
			metadata["save_slot_offset"] = saveOffset
		}
	}
	metadata["restart_supported"] = restart
	metadata["compressed_story"] = compressed
	if compressed {
		metadata["story_layout"] = "layout-2048.json"
		metadata["cache_bytes"] = 2048
		metadata["restart_archive_format"] = "lz-original-blocks-v1"
		metadata["restart_archive_bytes"] = layout["restart_bytes"]
		metadata["restart_decoded_bytes"] = dynamic
	}
	if restart && !compressed {
		metadata["restart_archive_offset"] = archiveOffset
		metadata["restart_archive_bytes"] = len(archive)
		metadata["restart_decoded_bytes"] = dynamic
		metadata["restart_archive_format"] = "rle-v1"
	}
	b, err := json.MarshalIndent(metadata, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(out, "build.json"), append(b, '\n'), 0644))
	fmt.Printf("Built %s: story %d B; resident %d B; entry $%04X\n", out, len(story), len(a.Binary), a.Symbols["main"])
}
