package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCPMRuntimeOrdinaryCLI(t *testing.T) {
	hirProfileTestState(t)
	oldRuntime := runtimeMode
	t.Cleanup(func() { runtimeMode = oldRuntime })
	runtimeMode = "cpm"
	source := "../../../examples/nanz/cpm_leaf_console.nanz"
	dir := t.TempDir()
	outputFile = filepath.Join(dir, "demo.com")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	outputFile = filepath.Join(dir, "demo.a80")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	asm, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString(string(asm))
	if err != nil || len(a.Errors) != 0 || !bytes.Equal(a.Binary, bin) {
		t.Fatal("CLI COM exact", err, a.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	z.MaxCycles = 1000000
	z.LoadMemory(0x100, bin)
	z.SetMemory(0xfdfe, 0)
	z.SetMemory(0xfdff, 0)
	z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xfdfe})
	var out []byte
	z.SetIOHandlers(nil, func(port uint16, b byte) { t.Errorf("CPM emitted port%x", port) })
	z.SetBDOSHandler(func(fn byte, de uint16) (byte, uint16, bool) {
		if fn != 2 {
			t.Fatalf("BDOS%d", fn)
		}
		out = append(out, byte(de))
		r := z.GetRegisters()
		r.A = 0xa5
		r.F = 0xff
		r.BC = 0x1234
		r.DE = 0x5678
		r.HL = 0x9abc
		r.IX = 0x2468
		r.IY = 0x1357
		z.SetRegisters(r)
		return 0xa5, 0xc3c3, true
	})
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if string(out) != "CP/M 53 OK\n" {
		t.Fatalf("output%q", out)
	}
	if z.GetMemory(uint16(a.Symbols["result"])) != 54 {
		t.Fatal("live result lost")
	}
	if z.GetRegisters().PC != 0 || z.GetRegisters().SP != 0xfe00 {
		t.Fatal("CPM exit SP", z.GetRegisters())
	}
	if z.GetMemory(uint16(a.Symbols["__cpm_stack_guard"])) != 0xa5 {
		t.Fatal("resident stack guard")
	}
	// Invoke the emitted wrapper directly with every primary register and
	// flags live, while BDOS deliberately corrupts them.
	keep := emulator.Registers{PC: uint16(a.Symbols["__cpm_putchar"]), SP: 0xfefe, A: 'X', F: 0xa5, BC: 0x1122, DE: 0x3344, HL: 0x5566, IX: 0x7788, IY: 0x99aa}
	z.SetMemory(0xfefe, 0)
	z.SetMemory(0xfeff, 0)
	z.SetRegisters(keep)
	z.Unhalt()
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	got := z.GetRegisters()
	if got.A != keep.A || got.F != keep.F || got.BC != keep.BC || got.DE != keep.DE || got.HL != keep.HL || got.IX != keep.IX || got.IY != keep.IY || got.SP != 0xff00 || got.PC != 0 {
		t.Fatal("BDOS wrapper register/flag preservation", got)
	}
	min := int(a.Symbols["__cpm_stack_top"])
	for address := int(a.Symbols["__cpm_stack"]); address < min; address++ {
		if z.GetMemory(uint16(address)) != 0 {
			min = address
			break
		}
	}
	t.Logf("COM %d bytes, BDOS chars%d, resident stack touched≤%d bytes", len(bin), len(out), int(a.Symbols["__cpm_stack_top"])-min)
}

func TestCPMRuntimeRejectsUnsupported(t *testing.T) {
	hirProfileTestState(t)
	old := runtimeMode
	t.Cleanup(func() { runtimeMode = old })
	runtimeMode = "cpm"
	for _, src := range []string{"@extern fun foreign()->u8\nfun main()->void{@print_u8(foreign())}", "global depth:u8=4\nfun recurse(n:u8)->u8{if n<2{return n}\nreturn recurse(n-1)+recurse(n-2)}\nfun main()->void{@print_u8(recurse(depth))}", "@extern fun peek(a:u8)->u8\nfun main()->void{@print_u8(peek(0))}"} {
		path := filepath.Join(t.TempDir(), "bad.nanz")
		os.WriteFile(path, []byte(src), 0644)
		outputFile = filepath.Join(t.TempDir(), "bad.com")
		if err := compile(path); err == nil {
			t.Fatal("unsupportedruntime accepted", src)
		}
	}
	oldAuto := hirProfileAutoDirectory
	t.Cleanup(func() { hirProfileAutoDirectory = oldAuto })
	hirProfileAutoDirectory = "evidence"
	if err := validateCLIRuntime("source.nanz"); err == nil {
		t.Fatal("measurement context accepted")
	}
}

func TestCPMRuntimeBytePrimitiveABIAndBoundary(t *testing.T) {
	hirProfileTestState(t)
	old := runtimeMode
	t.Cleanup(func() { runtimeMode = old })
	runtimeMode = "cpm"
	dir := t.TempDir()
	source := filepath.Join(dir, "memory.nanz")
	os.WriteFile(source, []byte("@extern fun peek(a:u16)->u8\n@extern fun poke(a:u16,v:u8)->void\nfun main()->void{poke(65535,165)\n@print_u8(peek(65535))}"), 0644)
	outputFile = filepath.Join(dir, "memory.a80")
	if e := compile(source); e != nil {
		t.Fatal(e)
	}
	text, e := os.ReadFile(outputFile)
	if e != nil {
		t.Fatal(e)
	}
	a, e := z80asm.NewAssembler().AssembleString(string(text))
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	for _, addr := range []uint16{0, 1, 0x7fff, 0x8000, 0xfffe, 0xffff} {
		for _, value := range []byte{0, 1, 127, 128, 255} {
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.MaxCycles = 10000
			z.LoadMemory(0x100, a.Binary)
			z.SetMemory(0xfefe, 0)
			z.SetMemory(0xfeff, 0)
			keep := emulator.Registers{PC: uint16(a.Symbols["poke"]), SP: 0xfefe, A: 0x9b, F: 0xa5, BC: 0x3400 | uint16(value), DE: 0x5678, HL: addr, IX: 0x9abc, IY: 0xdef0}
			z.SetRegisters(keep)
			if e := z.Run(); e != nil {
				t.Fatal(e)
			}
			got := z.GetRegisters()
			if z.GetMemory(addr) != value || got.A != keep.A || got.F != keep.F || got.BC != keep.BC || got.DE != keep.DE || got.HL != keep.HL || got.IX != keep.IX || got.IY != keep.IY || got.PC != 0 || got.SP != 0xff00 {
				t.Fatalf("poke addr%x value%d regs%+v", addr, value, got)
			}
			keep.PC = uint16(a.Symbols["peek"])
			z.SetRegisters(keep)
			z.Unhalt()
			if e := z.Run(); e != nil {
				t.Fatal(e)
			}
			got = z.GetRegisters()
			if got.A != value || got.F != keep.F || got.BC != keep.BC || got.DE != keep.DE || got.HL != keep.HL || got.IX != keep.IX || got.IY != keep.IY || got.PC != 0 || got.SP != 0xff00 {
				t.Fatalf("peek addr%x value%d regs%+v", addr, value, got)
			}
		}
	}
}
