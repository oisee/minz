//go:build ignore

// Run the Nanz/C FAT12 example as assembled Z80, with an image-backed controller.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func main() {
	imagePath := flag.String("image", "", "FAT12 disk image")
	outputPath := flag.String("output", "read.txt", "output file")
	sourceDir := flag.String("source", "../examples/system-c/fat12", "source directory")
	size := flag.Bool("size", false, "enable Grace Size")
	flag.Parse()
	fail := func(e any) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
	if *imagePath == "" {
		fail("use -image demo.img")
	}
	image, e := os.ReadFile(*imagePath)
	if e != nil {
		fail(e)
	}
	srcPath := filepath.Join(*sourceDir, "main.nanz")
	src, e := os.ReadFile(srcPath)
	if e != nil {
		fail(e)
	}
	h, e := nanz.ParseWithOpts(string(src), srcPath, nanz.ParseOpts{BaseDir: *sourceDir})
	if e != nil {
		fail(e)
	}
	steps, e := pipeline.CompileHIRSteps(h, pipeline.Options{ContractOpt: true, AssertMode: "none", UseHIRGrace: *size, UseGrace: *size, OptSize: *size})
	if e != nil {
		fail(e)
	}
	bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil {
		fail(e)
	}
	if len(bin.Errors) > 0 {
		fail(bin.Errors)
	}
	symbol := func(name string) int {
		a := bin.Symbols[name]
		if a == 0 {
			a = bin.Symbols["v_"+name]
		}
		if a == 0 {
			fail("missing symbol " + name)
		}
		return a
	}
	var lba uint16
	cursor := 0
	status := byte(1)
	var output []byte
	requests := 0
	z := emulator.NewRemogattoZ80()
	z.SetIOHandlers(func(port uint16) byte {
		if port == 0x10e2 {
			return status
		}
		if port != 0x10e3 || status != 0 || cursor >= 512 {
			fail(fmt.Sprintf("invalid controller read %04x", port))
		}
		value := image[int(lba)*512+cursor]
		cursor++
		return value
	}, func(port uint16, value byte) {
		if port&255 == 0x23 {
			output = append(output, value)
			return
		}
		switch port {
		case 0x10e0:
			lba = (lba & 0xff00) | uint16(value)
		case 0x10e1:
			lba = (lba & 255) | uint16(value)<<8
		case 0x10e2:
			cursor = 0
			status = 0
			requests++
			if value != 1 || int(lba)*512+512 > len(image) {
				status = 1
			}
		default:
			fail(fmt.Sprintf("invalid controller write %04x", port))
		}
	})
	if e := z.LoadMemory(0x8000, bin.Binary); e != nil {
		fail(e)
	}
	z.LoadMemory(0xeffe, []byte{0, 0xf1})
	z.SetRegisters(emulator.Registers{PC: uint16(symbol("run")), SP: 0xeffe})
	for i := 0; i < 10000000 && z.GetRegisters().PC != 0xf100; i++ {
		z.Step()
	}
	r := z.GetRegisters()
	a := symbol("bytes_read")
	count := binary.LittleEndian.Uint32(z.Memory()[a : a+4])
	if r.PC != 0xf100 || r.SP != 0xf000 {
		fail(fmt.Sprintf("invalid return PC=%04x SP=%04x", r.PC, r.SP))
	}
	if s := z.Memory()[symbol("status")]; s != 0 {
		fail(fmt.Sprintf("FAT12 error %d after %d bytes", s, count))
	}
	if uint32(len(output)) != count {
		fail("output length does not match reader")
	}
	if e := os.WriteFile(*outputPath, output, 0644); e != nil {
		fail(e)
	}
	fmt.Printf("Z80: %d bytes, %d sector reads, BIN %d bytes; saved %s\n", count, requests, len(bin.Binary), *outputPath)
}
