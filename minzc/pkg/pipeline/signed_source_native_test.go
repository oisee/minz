package pipeline

import (
	"fmt"
	"os"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestOriginalSignedArithmeticNativeExhaustive(t *testing.T) {
	source, e := os.ReadFile("../../../examples/nanz/signed_arithmetic.nanz")
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(source), "signed_arithmetic.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	raw := mir2.NewVM(hir.LowerModule(parse()))
	s, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	final := mir2.NewVM(s.MIR2Module)
	bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if e != nil {
		t.Fatal(e)
	}
	if len(bin.Errors) > 0 {
		t.Fatal(bin.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.LoadMemory(0x8000, bin.Binary)
	z.LoadMemory(0xeffe, []byte{0, 0xf1})
	perFunction := map[string]int{}
	run := func(name string, a, b int, want uint16) {
		perFunction[name]++
		f := s.MIR2Module.FuncByName(name)
		args := []mir2.Value{{I: int64(a)}}
		if len(f.Contract.Params) == 2 {
			args = append(args, mir2.Value{I: int64(b)})
		}
		for _, vm := range []*mir2.VM{raw, final} {
			v, e := vm.Call(name, args)
			if e != nil || len(v) != 1 || uint16(v[0].I) != want {
				t.Fatalf("VM %s %d %d got=%v want=%d err=%v", name, a, b, v, want, e)
			}
		}
		z.SetRegisters(emulator.Registers{A: 0x5a, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(bin.Symbols[name]), SP: 0xeffe})
		for i, p := range f.Contract.Params {
			loc := s.Allocation.Locs[p.Reg]
			v := uint16(args[i].I)
			switch loc.Name {
			case "HL":
				z.SetRegister8("H", byte(v>>8))
				z.SetRegister8("L", byte(v))
			case "DE":
				z.SetRegister8("D", byte(v>>8))
				z.SetRegister8("E", byte(v))
			case "BC":
				z.SetRegister8("B", byte(v>>8))
				z.SetRegister8("C", byte(v))
			default:
				if e := z.SetRegister8(loc.Name, byte(v)); e != nil {
					t.Fatal(e, loc)
				}
			}
		}
		for n := 0; n < 10000 && z.GetPC() != 0xf100; n++ {
			z.Step()
		}
		r := z.GetRegisters()
		var got uint16
		switch mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty) {
		case "A":
			got = uint16(r.A)
		case "C":
			got = uint16(byte(r.BC))
		case "HL":
			got = r.HL
		case "DE":
			got = r.DE
		default:
			t.Fatal("unsupported return", f.Contract)
		}
		if got != want || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("native %s a=%d b=%d got=%d want=%d regs=%v\n%s", name, a, b, got, want, r, s.Assembly)
		}
	}
	count := 0
	for a := 0; a < 256; a++ {
		for k := 0; k < 256; k++ {
			run("sar", a, k, uint16(uint8(int8(a)>>uint(k))))
			run("logical", a, k, uint16(uint8(a)>>uint(k)))
			count += 2
		}
		run("sar_one", a, 0, uint16(uint8(int8(a)>>1)))
		run("div_two", a, 0, uint16(uint8(int16(int8(a))/2)))
		run("mod_two", a, 0, uint16(uint8(int16(int8(a))%2)))
		run("widen", a, 0, uint16(int16(int8(a))))
		count += 4
		for b := 1; b < 256; b++ {
			run("quotient", a, b, uint16(uint8(int16(int8(a))/int16(int8(b)))))
			run("remainder", a, b, uint16(uint8(int16(int8(a))%int16(int8(b)))))
			count += 2
		}
	}
	for _, a := range []int{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 32769, 65534, 65535, 0x1234, 0xabcd} {
		for k := 0; k < 256; k++ {
			run("sar16", a, k, uint16(int16(a)>>uint(k)))
			count++
		}
	}
	// This original module has no declared mutable globals or spill slots.
	for i, b := range bin.Binary {
		if z.Memory()[bin.Origin+i] != b {
			t.Fatalf("code/literal mutation at %04x", bin.Origin+i)
		}
	}
	if len(s.MIR2Module.Funcs) != 9 {
		t.Fatalf("unexpected source function count %d", len(s.MIR2Module.Funcs))
	}
	for _, f := range s.MIR2Module.Funcs {
		if perFunction[f.Name] == 0 {
			t.Fatalf("source function not checked: %s", f.Name)
		}
	}
	t.Logf("%d source cases across raw/final VM/native; counts=%v", count, perFunction)
	if dir := os.Getenv("SIGNED_SOURCE_EXPORT"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(dir+"/signed_arithmetic.nanz", source, 0644)
		os.WriteFile(dir+"/signed_arithmetic.a80", []byte(s.Assembly), 0644)
		os.WriteFile(dir+"/signed_arithmetic.bin", bin.Binary, 0644)
		os.WriteFile(dir+"/count.txt", []byte(fmt.Sprint(count)), 0644)
	}
}
