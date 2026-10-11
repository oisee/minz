package pipeline

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestU32ActualOperationsNative(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(uint32, uint32) uint32
	}{
		{"dynamic_shl", "a<<b", func(a, b uint32) uint32 { return a << b }},
		{"dynamic_shr", "a>>b", func(a, b uint32) uint32 { return a >> b }},
		{"callxor", "", func(a, b uint32) uint32 { return (a ^ (b + 65539)) + a }},
		{"add", "a+b", func(a, b uint32) uint32 { return a + b }},
		{"sub", "a-b", func(a, b uint32) uint32 { return a - b }},
		{"shl1", "a<<1", func(a, b uint32) uint32 { return a << 1 }},
		{"shl16", "a<<16", func(a, b uint32) uint32 { return a << 16 }},
		{"shr1", "a>>1", func(a, b uint32) uint32 { return a >> 1 }},
		{"shr16", "a>>16", func(a, b uint32) uint32 { return a >> 16 }},
		{"xor", "a xor b", func(a, b uint32) uint32 { return a ^ b }},
		{"and", "a & b", func(a, b uint32) uint32 { return a & b }},
		{"or", "a | b", func(a, b uint32) uint32 { return a | b }},
		{"ror7", "(a>>7)|(a<<25)", func(a, b uint32) uint32 { return a>>7 | a<<25 }},
	}
	// Book v7 §§3.2,3.4 specify u32 width and modular arithmetic.
	// Constant shifts/derived rotations additionally compare both VM stages.
	values := []uint32{0, 1, 2, 31, 32, 33, 63, 64, 257, 0xff, 0x100, 0xffff, 0x10000, 0x10001, 0x7fffffff, 0x80000000, 0xfffffffe, 0xffffffff, 0x12345678, 0xabcdef01}
	for bit := 0; bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	for _, count := range []int{0, 1, 7, 16, 31, 32} {
		n := count
		cases = append(cases, struct {
			name, expr string
			want       func(uint32, uint32) uint32
		}{fmt.Sprintf("rotate%d", n), fmt.Sprintf("(a>>%d)|(a<<%d)", n, 32-n), func(a, b uint32) uint32 {
			if n == 0 || n == 32 {
				return a
			}
			return a>>n | a<<(32-n)
		}})
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := fmt.Sprintf("global a:u32\nglobal b:u32\nglobal result:u32\nfun execute(){result=%s}\n", c.expr)
			if c.name == "callxor" {
				source = "global a:u32\nglobal b:u32\nglobal result:u32\nfun mutate()->u32 { b=b+65539\nreturn b }\nfun execute(){let x:u32=a\nlet y:u32=mutate()\nresult=(x xor y)+x}\n"
			}
			parse := func() *hir.Module {
				h, e := nanz.Parse(source, "u32-"+c.name+".nanz")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			raw := mir2.NewVM(hir.LowerModule(parse()))
			steps, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
			}
			if c.name == "callxor" && !strings.Contains(steps.Assembly, "CALL mutate") {
				t.Fatal("expected actual call with live u32 operand", steps.Assembly)
			}
			final := mir2.NewVM(steps.MIR2Module)
			bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(bin.Errors) > 0 {
				t.Fatal(e, bin.Errors)
			}
			symbol := func(n string) int {
				v := bin.Symbols[n]
				if v == 0 {
					v = bin.Symbols["v_"+n]
				}
				if v == 0 {
					t.Fatalf("missing native symbol %q", n)
				}
				return v
			}
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, bin.Binary)
			z.LoadMemory(0xeffe, []byte{0, 0xf1})
			for _, a := range values {
				for _, b := range values {
					want := c.want(a, b)
					for _, vm := range []*mir2.VM{raw, final} {
						vm.WriteHeapSlice(vm.GlobalAddr("a"), []byte{byte(a), byte(a >> 8), byte(a >> 16), byte(a >> 24)})
						vm.WriteHeapSlice(vm.GlobalAddr("b"), []byte{byte(b), byte(b >> 8), byte(b >> 16), byte(b >> 24)})
						if _, e := vm.Call("execute", nil); e != nil {
							t.Fatal(e)
						}
						if got := binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("result"), 4)); got != want {
							t.Fatal("VM", a, b, got, want)
						}
					}
					binary.LittleEndian.PutUint32(z.Memory()[symbol("a"):symbol("a")+4], a)
					binary.LittleEndian.PutUint32(z.Memory()[symbol("b"):symbol("b")+4], b)
					z.SetRegisters(emulator.Registers{A: 0x55, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(symbol("execute")), SP: 0xeffe})
					for n := 0; n < 10000 && z.GetRegisters().PC != 0xf100; n++ {
						z.Step()
					}
					r := z.GetRegisters()
					got := binary.LittleEndian.Uint32(z.Memory()[symbol("result") : symbol("result")+4])
					if got != want || r.PC != 0xf100 || r.SP != 0xf000 {
						t.Fatalf("native a%08x b%08x got%08x want%08x regs%v\n%s", a, b, got, want, r, steps.Assembly)
					}
				}
			}
			// Code and literals survive; only declared u32 globals and u32 spill slots mutate.
			for i, v := range bin.Binary {
				addr := bin.Origin + i
				mutable := false
				for _, reg := range steps.Allocation.Spilled {
					label := fmt.Sprintf("_spill_execute_r%d", reg)
					start, ok := bin.Symbols[label]
					if ok && addr >= start && addr < start+4 {
						mutable = true
					}
				}
				for name, start := range bin.Symbols {
					if (strings.HasPrefix(name, "_spill_execute_r") || strings.HasPrefix(name, "_spill_mutate_r")) && addr >= start && addr < start+4 {
						mutable = true
					}
				}
				for _, name := range []string{"a", "b", "result"} {
					if addr >= symbol(name) && addr < symbol(name)+4 {
						mutable = true
					}
				}
				if !mutable && z.Memory()[addr] != v {
					t.Fatal("code mutation")
				}
			}
			if dir := os.Getenv("U32_AUDIT_EXPORT"); dir != "" {
				os.MkdirAll(dir, 0755)
				os.WriteFile(dir+"/"+c.name+".nanz", []byte(source), 0644)
				os.WriteFile(dir+"/"+c.name+".a80", []byte(steps.Assembly), 0644)
				os.WriteFile(dir+"/"+c.name+".bin", bin.Binary, 0644)
			}
		})
	}
}
