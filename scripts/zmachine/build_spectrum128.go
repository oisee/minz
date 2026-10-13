// Run from minzc: go run ../scripts/zmachine/build_spectrum128.go STORY OUTDIR
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
	if len(os.Args) != 3 {
		panic("usage: build_spectrum128.go STORY OUTDIR (run from minzc)")
	}
	story, err := os.ReadFile(os.Args[1])
	must(err)
	if len(story) < 64 || len(story) > 98304 || story[0] != 3 {
		panic("requires Z-machine V3 story, 64..98304 bytes")
	}
	dynamic := int(story[14])<<8 | int(story[15])
	var archive []byte
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
	archiveOffset := len(story)
	if archiveOffset < 16384 {
		archiveOffset = 16384
	}
	if len(archive) > 0 && archiveOffset/16384 != (archiveOffset+len(archive)-1)/16384 {
		archiveOffset = (archiveOffset/16384 + 1) * 16384
	}
	restart := dynamic >= 64 && dynamic <= 16384 && len(archive) > 0 && archiveOffset+len(archive) <= 98304 && archiveOffset/16384 == (archiveOffset+len(archive)-1)/16384

	saveOffset := archiveOffset + len(archive)
	if saveOffset < 5*16384 {
		saveOffset = 5 * 16384
	}
	saveSupported := restart && 98304-saveOffset >= 32
	out, err := filepath.Abs(os.Args[2])
	must(err)
	must(os.MkdirAll(out, 0755))
	source := filepath.Join(out, "zvm-spectrum128.nanz")
	cmd := exec.Command("python3", "../scripts/zmachine/prepare_native_core.py", source, "--story-bytes", fmt.Sprint(len(story)), "--console")
	if restart {
		cmd.Args = append(cmd.Args, "--restart-offset", fmt.Sprint(archiveOffset), "--restart-bytes", fmt.Sprint(dynamic), "--restart-rle-bytes", fmt.Sprint(len(archive)))
	}
	if saveSupported {
		cmd.Args = append(cmd.Args, "--save-story", os.Args[1])
	}
	result, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("generate: %v %s", err, result))
	}
	src, err := os.ReadFile(source)
	must(err)
	h, err := nanz.ParseWithOpts(string(src), source, nanz.ParseOpts{StdlibDir: "../stdlib"})
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
		panic("resident code/data overlaps native stack reservation")
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	must(err)
	for i, b := range a.Binary {
		m.Memory.Write(0x5b00+uint16(i), b, false)
	}
	pages := []int{0, 1, 3, 4, 6, 7}
	for i, b := range story {
		m.Memory.RAM[pages[i/16384]][i%16384] = b
	}
	if restart {
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
	metadata["save_restore_supported"] = saveSupported
	metadata["save_slot_bytes"] = 0
	if saveSupported {
		metadata["save_slot_bytes"] = 98304 - saveOffset
		metadata["save_slot_offset"] = saveOffset
	}
	metadata["restart_supported"] = restart
	if restart {
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
