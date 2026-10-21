package c89_test

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// C90 6.2.1.1 promotes unsigned char to int on this 16-bit-int target.
// Relational results must remain equivalent for every pair, including 128..255.
func TestC89NarrowByteCompareNativeAllPairs(t *testing.T) {
	src := `unsigned char left,right;unsigned short result;
unsigned short relations(unsigned char x,unsigned char y){
 unsigned short bits=0;
 if(x<y)bits|=1;
 if(x<=y)bits|=2;
 if(x>y)bits|=4;
 if(x>=y)bits|=8;
 if(x==y)bits|=16;
 if(x!=y)bits|=32;
 return bits;
}
void execute(void){result=relations(left,right);}`
	for _, grace := range []bool{false, true} {
		m, err := c89.CompileWithOpts(src, "byte-relations.c", c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatal(err)
		}
		steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: grace, UseHIRGrace: grace})
		if err != nil {
			t.Fatal(err)
		}
		a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.Errors) > 0 {
			t.Fatal(a.Errors)
		}
		entry, l, r, out := a.Symbols["execute"], a.Symbols["left"], a.Symbols["right"], a.Symbols["result"]
		if entry == 0 || l == 0 || r == 0 || out == 0 {
			t.Fatal("missing symbols")
		}
		z := emulator.NewRemogattoZ80()
		mem := z.Memory()
		copy(mem[0x8000:], a.Binary)
		for x := 0; x < 256; x++ {
			for y := 0; y < 256; y++ {
				want := uint16(0)
				if x < y {
					want |= 1
				}
				if x <= y {
					want |= 2
				}
				if x > y {
					want |= 4
				}
				if x >= y {
					want |= 8
				}
				if x == y {
					want |= 16
				}
				if x != y {
					want |= 32
				}
				mem[l] = byte(x)
				mem[r] = byte(y)
				mem[out] = 0xef
				mem[out+1] = 0xbe
				mem[0xeffe] = 0
				mem[0xefff] = 0xf1
				z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
				for n := 0; n < 10000 && z.GetPC() != 0xf100; n++ {
					z.Step()
				}
				got := binary.LittleEndian.Uint16(mem[out : out+2])
				regs := z.GetRegisters()
				if got != want || regs.PC != 0xf100 || regs.SP != 0xf000 || mem[l] != byte(x) || mem[r] != byte(y) {
					t.Fatalf("grace=%v x=%d y=%d got=%d want=%d PC/SP=%04x/%04x", grace, x, y, got, want, regs.PC, regs.SP)
				}
			}
		}
	}
}

// Exercise promoted byte/constant comparisons on the Z80 for every byte,
// including the signed-byte boundary, in both operand orders and pipelines.
func TestC89NarrowByteCompareConstantsNative(t *testing.T) {
	for _, k := range []int{-5, -1, 0, 8, 128, 255, 256, 300, 32767, 32768, 65535} {
		for _, reverse := range []bool{false, true} {
			for _, grace := range []bool{false, true} {
				left, right := "x", fmt.Sprint(k)
				if reverse {
					left, right = right, left
				}
				var src strings.Builder
				src.WriteString("unsigned char input; unsigned short result; unsigned short relations(unsigned char x){unsigned short bits=0;")
				for n, op := range []string{"<", "<=", ">", ">=", "==", "!="} {
					fmt.Fprintf(&src, "if(%s%s%s)bits|=%d;", left, op, right, 1<<n)
				}
				src.WriteString("return bits;} void execute(void){result=relations(input);}")
				m, err := c89.CompileWithOpts(src.String(), "byte-constants.c", c89.CompileOpts{NoLibc: true})
				if err != nil {
					t.Fatal(err)
				}
				steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: grace, UseHIRGrace: grace})
				if err != nil {
					t.Fatal(err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				if len(a.Errors) > 0 {
					t.Fatal(a.Errors)
				}
				entry, in, out := a.Symbols["execute"], a.Symbols["input"], a.Symbols["result"]
				if entry == 0 || in == 0 || out == 0 {
					t.Fatal("missing symbols")
				}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				for x := 0; x < 256; x++ {
					l, r := x, k
					if reverse {
						l, r = r, l
					}
					var want uint16
					for n, ok := range []bool{l < r, l <= r, l > r, l >= r, l == r, l != r} {
						if ok {
							want |= 1 << n
						}
					}
					mem[in] = byte(x)
					mem[out], mem[out+1] = 0xef, 0xbe
					mem[0xeffe], mem[0xefff] = 0, 0xf1
					z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
					for n := 0; n < 10000 && z.GetPC() != 0xf100; n++ {
						z.Step()
					}
					got := binary.LittleEndian.Uint16(mem[out : out+2])
					regs := z.GetRegisters()
					if got != want || regs.PC != 0xf100 || regs.SP != 0xf000 || mem[in] != byte(x) {
						t.Fatalf("k=%d reverse=%v grace=%v x=%d got=%d want=%d PC/SP=%04x/%04x", k, reverse, grace, x, got, want, regs.PC, regs.SP)
					}
				}
			}
		}
	}
}
