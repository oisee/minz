package c89_test

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// C99 6.2.5, 6.3.1.1, 6.3.1.8: long is 32 bits on our target;
// unsigned operations wrap, and integer promotions happen before arithmetic.
// Read all four result bytes on both VMs and the real assembled Z80 path.
func TestCIntegerSemanticsNativeVM(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(uint32, uint32) uint32
	}{
		{"mul", "a*b", func(a, b uint32) uint32 { return a * b }},
		{"mul_live", "a*b+a+b", func(a, b uint32) uint32 { return a*b + a + b }},
		{"mul_const", "a*65537UL", func(a, b uint32) uint32 { return a * 65537 }},
		{"mul_high_const", "a*0x80000000UL", func(a, b uint32) uint32 { return a * 0x80000000 }},
		{"mul_zero", "a*0UL", func(a, b uint32) uint32 { return 0 }},
		{"mul_one", "a*1UL", func(a, b uint32) uint32 { return a }},
		{"div", "a/b", func(a, b uint32) uint32 { return a / b }},
		{"mod", "a%b", func(a, b uint32) uint32 { return a % b }},
		{"signed_mul", "(long)a*(long)b", func(a, b uint32) uint32 { return uint32(int32(a) * int32(b)) }},
		{"signed_div", "(long)a/(long)b", func(a, b uint32) uint32 { return uint32(int32(a) / int32(b)) }},
		{"signed_mod", "(long)a%(long)b", func(a, b uint32) uint32 { return uint32(int32(a) % int32(b)) }},
		{"add", "a+b", func(a, b uint32) uint32 { return a + b }},
		{"sub", "a-b", func(a, b uint32) uint32 { return a - b }},
		{"xor", "a^b", func(a, b uint32) uint32 { return a ^ b }},
		{"and", "a&b", func(a, b uint32) uint32 { return a & b }},
		{"or", "a|b", func(a, b uint32) uint32 { return a | b }},
		{"shl", "a<<1", func(a, b uint32) uint32 { return a << 1 }},
		{"shr", "a>>1", func(a, b uint32) uint32 { return a >> 1 }},
		{"promoted_negate", "-(unsigned char)a", func(a, b uint32) uint32 { return uint32(-int32(uint8(a))) }},
		{"promoted_complement", "~(unsigned char)a", func(a, b uint32) uint32 { return uint32(^int32(uint8(a))) }},
		{"promoted_ushort", "((unsigned short)a>>1)", func(a, b uint32) uint32 { return uint32(uint16(a)) >> 1 }},
		{"signed_call", "take((int)a)", func(a, b uint32) uint32 { return uint32(int32(int16(a))) + 1 }},
		{"compound_div", "divide((unsigned char)a)", func(a, b uint32) uint32 { return uint32(int32(uint8(a))/-2) & 255 }},
		{"ushort_div", "(unsigned short)a / -2", func(a, b uint32) uint32 { return uint32(uint16(a) / 65534) }},
		{"long_signed_shift", "((long)(int)a)>>1", func(a, b uint32) uint32 { return uint32(int32(int16(a)) >> 1) }},
		{"long_unsigned_compare", "a<b", func(a, b uint32) uint32 {
			if a < b {
				return 1
			}
			return 0
		}},
		{"long_signed_compare", "(long)a<(long)b", func(a, b uint32) uint32 {
			if int32(a) < int32(b) {
				return 1
			}
			return 0
		}},

		{"layout", "sizeof(long)+sizeof(struct Record)", func(a, b uint32) uint32 { return 11 }},

		{"cast_char", "(long)(char)a", func(a, b uint32) uint32 { return uint32(int32(int8(a))) }},
		{"promoted_sub", "(unsigned char)a-(unsigned char)b", func(a, b uint32) uint32 { return uint32(int32(uint8(a)) - int32(uint8(b))) }},
		{"promoted_shift", "(unsigned char)a<<1", func(a, b uint32) uint32 { return uint32(uint8(a)) << 1 }},
		{"mixed_compare", "(int)a < (unsigned int)b", func(a, b uint32) uint32 {
			if uint16(a) < uint16(b) {
				return 1
			}
			return 0
		}},
	}
	values := []uint32{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x12345678, 0x7fffffff, 0x80000000, 0xffffffff}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := fmt.Sprintf("unsigned long a,b,result; unsigned long initial=0x12345678UL; struct Record{unsigned char tag;long value;unsigned int tail;}; unsigned long take(long x){return x+1;} unsigned char divide(unsigned char x){x/=-2;return x;} void execute(void){result=%s;}", c.expr)
			parse := func() *hir.Module {
				h, e := c89.Compile(source, "integers.c")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			raw := mir2.NewVM(hir.LowerModule(parse()))
			steps, e := pipeline.CompileHIRSteps(parse(), pipeline.Options{ContractOpt: true, AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
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
					t.Fatalf("missing symbol %s", n)
				}
				return v
			}
			z := emulator.NewRemogattoZ80()
			if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
				t.Fatal(err)
			}
			if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
				t.Fatal(err)
			}
			if got := binary.LittleEndian.Uint32(z.Memory()[symbol("initial") : symbol("initial")+4]); got != 0x12345678 {
				t.Fatalf("initializer %08x", got)
			}
			for _, a := range values {
				for _, b := range values {
					// C99 6.5.5: exclude division by zero and unrepresentable
					// signed results; signed overflow is not an oracle input.
					if (c.name == "div" || c.name == "mod" || c.name == "signed_div" || c.name == "signed_mod") && b == 0 {
						continue
					}
					if (c.name == "signed_div" || c.name == "signed_mod") && a == 0x80000000 && b == 0xffffffff {
						continue
					}
					if c.name == "signed_mul" {
						product := int64(int32(a)) * int64(int32(b))
						if product < -2147483648 || product > 2147483647 {
							continue
						}
					}
					want := c.want(a, b)
					for _, vm := range []*mir2.VM{raw, final} {
						var data [4]byte
						binary.LittleEndian.PutUint32(data[:], a)
						vm.WriteHeapSlice(vm.GlobalAddr("a"), data[:])
						binary.LittleEndian.PutUint32(data[:], b)
						vm.WriteHeapSlice(vm.GlobalAddr("b"), data[:])
						if _, e := vm.Call("execute", nil); e != nil {
							t.Fatal(e)
						}
						if got := binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("result"), 4)); got != want {
							t.Fatalf("VM a%08x b%08x got%08x want%08x", a, b, got, want)
						}
					}
					binary.LittleEndian.PutUint32(z.Memory()[symbol("a"):symbol("a")+4], a)
					binary.LittleEndian.PutUint32(z.Memory()[symbol("b"):symbol("b")+4], b)
					z.SetRegisters(emulator.Registers{PC: uint16(symbol("execute")), SP: 0xeffe})
					for n := 0; n < 10000 && z.GetRegisters().PC != 0xf100; n++ {
						z.Step()
					}
					r := z.GetRegisters()
					got := binary.LittleEndian.Uint32(z.Memory()[symbol("result") : symbol("result")+4])
					if got != want || r.PC != 0xf100 || r.SP != 0xf000 {
						t.Fatalf("Z80 a%08x b%08x got%08x want%08x regs%v\n%s", a, b, got, want, r, steps.Assembly)
					}
				}
			}
		})
	}
}

func TestCRejectsNarrowedLongLong(t *testing.T) {
	for _, src := range []string{"long long f(long long x){return x+1;}", "unsigned long long data;"} {
		if _, e := c89.Compile(src, "long-long.c"); e == nil || !strings.Contains(e.Error(), "64-bit") {
			t.Fatalf("expected explicit 64-bit diagnostic, got %v", e)
		}
	}
}

// C99 6.3.1.3 and 6.8.6.4: integer promotion happens during arithmetic,
// followed by conversion to the declared uint8_t result on return.
func TestCMath8ExampleNativeVM(t *testing.T) {
	source, e := os.ReadFile("../../../examples/c/c99_math8.c")
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := c89.Compile(string(source), "c99_math8.c")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			h := parse()
			checks := append([]hir.Assert(nil), h.Asserts...)
			if len(checks) != 31 {
				t.Fatalf("expected complete 31-case example, got %d", len(checks))
			}
			for i := range h.Asserts {
				h.Asserts[i].Via = "z80"
			}
			steps, e := pipeline.CompileHIRSteps(h, pipeline.Options{AssertMode: "z80", ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if e != nil {
				t.Fatal(e)
			}
			for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
				for _, check := range checks {
					vm := mir2.NewVM(m)
					args := make([]mir2.Value, len(check.Args))
					for i, a := range check.Args {
						args[i] = mir2.Value{I: a}
					}
					got, e := vm.Call(check.FuncName, args)
					if e != nil || len(got) != 1 || got[0].I != check.Expected {
						t.Fatalf("%s%v: got %v err %v want %d", check.FuncName, check.Args, got, e, check.Expected)
					}
				}
			}
		})
	}
}
