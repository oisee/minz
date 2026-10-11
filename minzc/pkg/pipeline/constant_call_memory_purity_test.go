package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestConstantCallPreservesPointerMemorySource(t *testing.T) {
	const src = `@extern fun observe()->void
 fun store(p:^u8)->u8{p^=9
 return 9}
 fun read(p:^u8)->u8{return p^}
 fun wrapper()->u8{return store(ptr(24576))}
 fun write_entry()->u8{return store(ptr(24576))}
 fun read_entry()->u8{return read(ptr(24576))}
 fun transitive_entry()->u8{return wrapper()}`
	parse := func() *hir.Module {
		h, e := nanz.Parse(src, "constant-memory-effects")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, o := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true}, {AssertMode: "none", UseGrace: true, OptSize: true}} {
		s, e := CompileHIRSteps(parse(), o)
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		for _, c := range []struct {
			name        string
			result, mem byte
		}{{"write_entry", 9, 9}, {"read_entry", 73, 73}, {"transitive_entry", 9, 9}} {
			for _, m := range []*mir2.Module{hir.LowerModule(parse()), s.MIR2Module} {
				vm := mir2.NewVM(m)
				vm.EnsureHeap(65536)
				vm.WriteHeap(24576, 73)
				got, e := vm.Call(c.name, nil)
				if e != nil || len(got) != 1 || got[0].I != int64(c.result) || vm.ReadHeap(24576, 1)[0] != c.mem {
					t.Fatal("VM source memory", c, got, e, vm.ReadHeap(24576, 1))
				}
			}
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, a.Binary)
			z.Memory()[24576] = 73
			before := append([]byte(nil), z.Memory()...)
			z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 3
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols[c.name]), SP: 0xeff0})
			for n := 0; n < 10000 && z.GetPC() != 0x300; n++ {
				z.Step()
			}
			if z.GetPC() != 0x300 || z.GetSP() != 0xeff2 || z.GetRegisters().A != c.result || z.Memory()[24576] != c.mem {
				t.Fatal("native source memory", c, z.GetRegisters(), z.Memory()[24576])
			}
			for p, b := range before {
				if p == 24576 || p >= 0xefc0 && p < 0xeff2 {
					continue
				}
				if z.Memory()[p] != b {
					t.Fatal("unrelated RAM", p)
				}
			}
			if !bytes.Equal(z.Memory()[0x8000:0x8000+len(a.Binary)], a.Binary) {
				t.Fatal("code mutation")
			}
		}
	}
}
