package nanz_test

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// Full pipeline/PBQP, independent expected output bytes and live global successor.
func TestLiteralPrintingNative(t *testing.T) {
	cases := []struct {
		name, literal string
		want          []byte
	}{{"s", `"AB"`, []byte("AB")}, {"c", `c"AB"`, []byte("AB")}, {"l", `l"AB"`, []byte("AB")}, {"utf8", `"é"`, []byte{0xc3, 0xa9}}, {"empty_s", `""`, nil}, {"empty_c", `c""`, nil}, {"empty_l", `l""`, nil}, {"nul_s", `"A\0B"`, []byte{65, 0, 66}}, {"nul_c", `c"A\0B"`, []byte{65}}, {"nul_l", `l"A\0B"`, []byte{65, 0, 66}}, {"s255", "\"" + strings.Repeat("q", 255) + "\"", bytes.Repeat([]byte{'q'}, 255)}, {"l256", "l\"" + strings.Repeat("r", 256) + "\"", bytes.Repeat([]byte{'r'}, 256)}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := fmt.Sprintf("global __print_ptr_1:u16=123\nglobal __print_count_2:u16=456\nglobal live:u16=4660\nglobal answer:u16=0\nfun show()->void {\n let x:u16=live\n @print(%s)\n @print(c\"!\")\n answer=x+1\n}\n", c.literal)
			h, e := nanz.Parse(source, "native_strings.nanz")
			if e != nil {
				t.Fatal(e)
			}
			steps, e := pipeline.CompileHIRSteps(h)
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x0100\n LD SP,0xF000\n CALL show\n test_done:\n DI\n HALT\n" + steps.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 1000000
			if e = z.LoadMemory(0x100, a.Binary); e != nil {
				t.Fatal(e)
			}
			var out []byte
			z.SetIOHandlers(nil, func(port uint16, v byte) {
				if byte(port) != 0x23 {
					t.Errorf("unexpected port%x", port)
				}
				out = append(out, v)
			})
			z.SetPC(0x100)
			if e = z.Run(); e != nil {
				t.Fatal(e)
			}
			regs := z.GetRegisters()
			if regs.SP != 0xF000 || int(regs.PC) != a.Symbols["test_done"]+2 {
				t.Fatalf("termination PC/SP %x/%x", regs.PC, regs.SP)
			}
			for n, v := range map[string]uint16{"__print_ptr_1": 123, "__print_count_2": 456} {
				addr := a.Symbols[n]
				got := uint16(z.GetMemory(uint16(addr))) | uint16(z.GetMemory(uint16(addr+1)))<<8
				if got != v {
					t.Fatal("generated name clobbered global", n, got)
				}
			}
			want := append(append([]byte{}, c.want...), byte('!'))
			if !bytes.Equal(out, want) {
				t.Fatalf("output=%v want=%v", out, want)
			}
			addr, ok := a.Symbols["answer"]
			if !ok {
				t.Fatal("answer absent")
			}
			got := uint16(z.GetMemory(uint16(addr))) | uint16(z.GetMemory(uint16(addr+1)))<<8
			if got != 4661 {
				t.Fatalf("live successor %x", got)
			}
		})
	}
}
