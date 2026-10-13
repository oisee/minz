package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Inputs differ from the generator comparison: lengths, addresses, values and
// a value live across the loop. The oracle computes expected bytes directly.
func TestPBQPByteCopyDeclarationOrderNative(t *testing.T) {
	for _, width := range []string{"u8", "u16"} {
		for _, n := range []int{1, 3, 17, 127, 255} {
			for _, preserve := range []bool{false, true} {
				for _, opts := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
					var costs [2]int
					var sizes [2]int

					for order := 0; order < 2; order++ {
						decl := []string{"var src:^u8=ptr(0x6203)", "var dst:^u8=ptr(0x6807)"}
						if order == 1 {
							decl[0], decl[1] = decl[1], decl[0]
						}
						source := fmt.Sprintf(`global input:u8=0
 global answer:u8=0
 fun copy_test() {
 let keep:u8=input+7
 %s
 var left:u8=%d
 while left!=0 {
 dst^ = src^
 src=src+1
 dst=dst+1
 left=left-1
 }
 answer=keep^input
 }`, strings.Join(decl, "\n"), n)
						source = strings.ReplaceAll(source, "var left:u8", "var left:"+width)
						if !preserve {
							source = strings.ReplaceAll(source, "let keep:u8=input+7", "")
							source = strings.ReplaceAll(source, "answer=keep^input", "")
						}
						h, err := nanz.Parse(source, "copy-order.nanz")
						if err != nil {
							t.Fatal(err)
						}
						steps, err := CompileHIRSteps(h, opts)
						if err != nil {
							t.Fatal(err)
						}
						a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
						if err != nil || len(a.Errors) != 0 {
							t.Fatal(err, a.Errors)
						}
						z := emulator.NewRemogattoZ80()
						mem := z.Memory()
						for i := range mem {
							mem[i] = byte(i*37 + i/256 + 19)
						}
						if err = z.LoadMemory(0x8000, a.Binary); err != nil {
							t.Fatal(err)
						}
						v := byte(0x97)
						input := a.Symbols["input"]
						answer := a.Symbols["answer"]
						mem[input] = v
						mem[0xeff0], mem[0xeff1] = 0, 0x70
						before := append([]byte(nil), mem...)
						z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["copy_test"]), SP: 0xeff0, A: 0x83, HL: 0x1234, DE: 0x5678, BC: 0x9abc})
						for count := 0; count < 2000000; count++ {
							if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
								break
							}
							z.Step()
						}
						if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
							t.Fatal("return", z.GetRegisters())
						}
						for address, old := range before {
							want := old
							if address >= 0x6807 && address < 0x6807+n {
								want = before[0x6203+address-0x6807]
							} else if address == answer && preserve {
								want = (v + 7) ^ v

							} else if address >= 0xef00 && address < 0xeff0 {
								continue
							}
							if mem[address] != want {
								t.Fatalf("n=%d order=%d RAM[%04x]=%02x want %02x", n, order, address, mem[address], want)
							}
						}
						costs[order], sizes[order] = z.GetCycles(), len(a.Binary)
					}
					t.Logf("n=%d counter=%s keep=%v grace=%v size=%v source-first/destination-first: B=%v T=%v; full RAM/PC/SP PASS", n, width, preserve, opts.UseGrace, opts.OptSize, sizes, costs)
				}
			}
		}
	}
}
