package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	oldPin := checkedLocalTableSHA256
	t.Cleanup(func() { checkedLocalTableSHA256 = oldPin })
	hirProfileTestState(t)
	oldAutoDirectory := hirProfileAutoDirectory
	t.Cleanup(func() { hirProfileAutoDirectory = oldAutoDirectory })
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
	checkedLocalTableSHA256 = fmt.Sprintf("%064x", 1)
	if err := validateCLIRuntime("source.nanz"); err == nil {
		t.Fatal("table pin with runtime accepted")
	}
	checkedLocalTableSHA256 = oldPin
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

func TestCPMRuntimeGuardedStackOverflowAndFlags(t *testing.T) {
	hirProfileTestState(t)
	oldRuntime, oldStack := runtimeMode, runtimeStackBytes
	t.Cleanup(func() { runtimeMode = oldRuntime; runtimeStackBytes = oldStack })
	runtimeMode = "cpm"
	runtimeStackBytes = 64
	dir := t.TempDir()
	path := filepath.Join(dir, "recursive.nanz")
	os.WriteFile(path, []byte("global depth:u8=30\nglobal result:u8=0\nfun rec(n:u8)->u8{if n<2{return n}\nreturn rec(n-1)+rec(n-2)}\nfun main()->void{result=rec(depth)}"), 0644)
	outputFile = filepath.Join(dir, "recursive.a80")
	if e := compile(path); e != nil {
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
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	z.MaxCycles = 1000000
	z.LoadMemory(0x100, a.Binary)
	z.SetMemory(0xfdfe, 0)
	z.SetMemory(0xfdff, 0)
	z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xfdfe})
	var initial [65536]byte
	for p := range initial {
		initial[p] = z.GetMemory(uint16(p))
	}
	if e := z.Run(); e != nil {
		t.Fatal(e)
	}
	if z.GetRegisters().PC != 0 || z.GetRegisters().SP != 0xfe00 || z.GetMemory(uint16(a.Symbols["__cpm_status"])) != 1 || z.GetMemory(uint16(a.Symbols["__cpm_stack_guard"])) != 0xa5 || z.GetMemory(uint16(a.Symbols["result"])) != 0 {
		t.Fatal("overflow exit", z.GetRegisters())
	}
	if dir := os.Getenv("MINZ_CPM_OVERFLOW_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "overflow.a80"), text, 0644)
		os.WriteFile(filepath.Join(dir, "overflow.com"), a.Binary, 0644)
		capture, _ := json.MarshalIndent(map[string]any{"registers": z.GetRegisters(), "status": z.GetMemory(uint16(a.Symbols["__cpm_status"])), "guard": z.GetMemory(uint16(a.Symbols["__cpm_stack_guard"])), "result": z.GetMemory(uint16(a.Symbols["result"]))}, "", "  ")
		os.WriteFile(filepath.Join(dir, "overflow.json"), capture, 0644)
	}
	allowed := map[uint16]bool{}
	for _, name := range []string{"depth", "result", "__cpm_status", "__cpm_saved_sp"} {
		for i := 0; i < 2; i++ {
			allowed[uint16(a.Symbols[name]+i)] = true
		}
	}
	for p := a.Symbols["__cpm_stack"]; p < a.Symbols["__cpm_stack_top"]; p++ {
		allowed[uint16(p)] = true
	}
	for _, line := range a.Listing {
		if strings.HasPrefix(line.Label, "_spill_") {
			for i := range line.Bytes {
				allowed[uint16(line.Address+i)] = true
			}
		}
	}
	for p, b := range initial {
		if !allowed[uint16(p)] && z.GetMemory(uint16(p)) != b {
			t.Fatalf("overflow unexpected memory write %04x", p)
		}
	}
	// Guard uses six temporary bytes. Accepted check retains 32 bytes of
	// slack after those pushes, so grouped growth retains checker headroom.
	for _, space := range []uint16{6, 8, 10, 12, 14, 16, 32, 36, 38, 40, 64} {
		z.LoadMemory(0x100, a.Binary)
		base := uint16(a.Symbols["__cpm_stack"])
		sp := base + space - 2
		z.SetMemory(sp, 0)
		z.SetMemory(sp+1, 0)
		keep := emulator.Registers{PC: uint16(a.Symbols["__cpm_stack_check"]), SP: sp, A: 0x9b, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357}
		z.SetMemory(uint16(a.Symbols["__cpm_saved_sp"]), 0xfe)
		z.SetMemory(uint16(a.Symbols["__cpm_saved_sp"])+1, 0xfd)
		z.SetMemory(0xfdfe, 0)
		z.SetMemory(0xfdff, 0)
		below := make([]byte, int(base))
		for p := range below {
			below[p] = z.GetMemory(uint16(p))
		}
		z.SetRegisters(keep)
		z.Unhalt()
		if e := z.Run(); e != nil {
			t.Fatal(e)
		}
		got := z.GetRegisters()
		if space >= 38 {
			if got.A != keep.A || got.F != keep.F || got.BC != keep.BC || got.DE != keep.DE || got.HL != keep.HL || got.IX != keep.IX || got.IY != keep.IY || got.SP != sp+2 || got.PC != 0 {
				t.Fatalf("check preservation space%d %+v", space, got)
			}
		} else if z.GetMemory(uint16(a.Symbols["__cpm_status"])) != 1 || got.PC != 0 || got.SP != 0xfe00 {
			t.Fatal("overflow threshold", space)
		}
		for p, b := range below {
			if uint16(p) == uint16(a.Symbols["__cpm_status"]) {
				continue
			}
			if z.GetMemory(uint16(p)) != b {
				t.Fatalf("checker below-floor write %04x space%d", p, space)
			}
		}
		if z.GetMemory(base-1) != 0xa5 {
			t.Fatal("guard write", space)
		}
	}
	for _, size := range []int{-1, 1, 63, 65, 16386} {
		runtimeStackBytes = size
		outputFile = filepath.Join(dir, "bad.com")
		if e := compile(path); e == nil {
			t.Fatal("bad stack admitted", size)
		}
		if _, e := os.Stat(outputFile); !os.IsNotExist(e) {
			t.Fatal("bad options wrote output")
		}
	}
	runtimeMode = ""
	runtimeStackBytes = 64
	if e := validateCLIRuntime(path); e == nil {
		t.Fatal("stack without runtime")
	}
}
