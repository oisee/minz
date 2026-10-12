package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This tests the current Nanz non-shift unsigned rule: the wider operand
// determines operation width (not C's int promotions or signedness policy).
func TestMixedUnsignedWidthBothOrders(t *testing.T) {
	for _, w := range []int{24, 32} {
		for _, op := range []string{"+", "-", "&", "|", "^"} {
			name := fmt.Sprintf("u%d_%s", w, map[string]string{"+": "add", "-": "sub", "&": "and", "|": "or", "^": "xor"}[op])
			t.Run(name, func(t *testing.T) {
				src := fmt.Sprintf(`global narrow:u16
 global wide:u%d
 global result_lr:u%d
 global result_rl:u%d
 fun compute() {result_lr=narrow %s wide
 result_rl=wide %s narrow}`, w, w, w, op, op)
				parse := func() *hir.Module {
					h, e := nanz.Parse(src, name)
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				if dir := os.Getenv("MINZ_MIXED_UNSIGNED_WIDTH_ARTIFACTS"); dir != "" {
					os.MkdirAll(dir, 0755)
					if e := os.WriteFile(filepath.Join(dir, name+".nanz"), []byte(src), 0644); e != nil {
						t.Fatal(e)
					}
				}
				cases := [][2]uint32{{0, 65536}, {1, 65536}, {65535, 65536}, {32768, 0xffffff}, {65535, 0xffffffff}, {255, 0x123456}, {0, 0}, {65535, 65535}}
				for _, o := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true}, {AssertMode: "none", UseGrace: true, OptSize: true}} {
					s, e := CompileHIRSteps(parse(), o)
					if e != nil {
						t.Fatal(e)
					}
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil || len(a.Errors) > 0 {
						t.Fatal(e, a.Errors)
					}
					if dir := os.Getenv("MINZ_MIXED_UNSIGNED_WIDTH_ARTIFACTS"); dir != "" {
						prefix := fmt.Sprintf("%s-grace%t-size%t", name, o.UseGrace, o.OptSize)
						for suffix, data := range map[string][]byte{".a80": []byte(s.Assembly), ".bin": a.Binary, ".mir2": []byte(s.MIR2Module.Dump())} {
							if e := os.WriteFile(filepath.Join(dir, prefix+suffix), data, 0644); e != nil {
								t.Fatal(e)
							}
						}
					}
					for _, c := range cases {
						x, y := c[0]&65535, c[1]
						mask := uint32(0xffffffff)
						if w == 24 {
							mask = 0xffffff
						}
						y &= mask
						eval := func(a, b uint32) uint32 {
							switch op {
							case "+":
								return (a + b) & mask
							case "-":
								return (a - b) & mask
							case "&":
								return a & b
							case "|":
								return a | b
							default:
								return a ^ b
							}
						}
						expected := map[string]uint32{"result_lr": eval(x, y), "result_rl": eval(y, x)}
						for _, m := range []*mir2.Module{hir.LowerModule(parse()), s.MIR2Module} {
							vm := mir2.NewVM(m)
							off := int64(0)
							for _, g := range m.Globals {
								v := uint32(0)
								if g.Name == "narrow" {
									v = x
								}
								if g.Name == "wide" {
									v = y
								}
								b := make([]byte, mir2.ByteWidth(g.Ty))
								for i := range b {
									b[i] = byte(v >> uint(i*8))
								}
								vm.WriteHeapSlice(off, b)
								off += int64(len(b))
							}
							if _, e := vm.Call("compute", nil); e != nil {
								t.Fatal(e)
							}
							off = 0
							for _, g := range m.Globals {
								if want, ok := expected[g.Name]; ok {
									got := vm.ReadHeap(off, mir2.ByteWidth(g.Ty))
									for i, b := range got {
										if b != byte(want>>uint(i*8)) {
											t.Fatalf("VM %s x%x y%x got%x want%x", g.Name, x, y, got, want)
										}
									}
								}
								off += int64(mir2.ByteWidth(g.Ty))
							}
						}
						z := emulator.NewRemogattoZ80()
						z.LoadMemory(0x8000, a.Binary)
						mem := z.Memory()
						for name, v := range map[string]uint32{"narrow": x, "wide": y} {
							p := a.Symbols[name]
							n := w / 8
							if name == "narrow" {
								n = 2
							}
							for i := 0; i < n; i++ {
								mem[p+i] = byte(v >> uint(i*8))
							}
						}
						before := append([]byte(nil), mem...)
						mem[0xeff0], mem[0xeff1] = 0, 3
						z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["compute"]), SP: 0xeff0})
						for i := 0; i < 10000 && z.GetPC() != 0x300; i++ {
							z.Step()
						}
						if z.GetPC() != 0x300 || z.GetSP() != 0xeff2 {
							t.Fatal("frame")
						}
						for name, want := range expected {
							p := a.Symbols[name]
							for i := 0; i < w/8; i++ {
								if mem[p+i] != byte(want>>uint(i*8)) {
									t.Fatalf("native %s x%x y%x got%x want%x\n%s", name, x, y, mem[p:p+w/8], want, s.Assembly)
								}
							}
						}
						spans := map[int]int{}
						for _, g := range s.MIR2Module.Globals {
							if strings.HasPrefix(g.Name, "result_") {
								spans[a.Symbols[g.Name]] = mir2.ByteWidth(g.Ty)
							}
						}
						for _, f := range s.MIR2Module.Funcs {
							for _, b := range f.Blocks {
								for _, i := range b.Insts {
									if i.Dst != mir2.NoReg && i.Ty != nil {
										if p, ok := a.Symbols[mir2.Z80SpillLabel(f.Name, i.Dst)]; ok {
											spans[p] = mir2.ByteWidth(i.Ty)
										}
									}
								}
							}
						}
						for p, b := range before {
							allowed := p >= 0xefd0 && p < 0xeff2
							for q, n := range spans {
								allowed = allowed || p >= q && p < q+n
							}
							if !allowed && mem[p] != b {
								t.Fatal("unrelated RAM", p)
							}
						}
						end := a.Symbols["narrow"]
						for q := range spans {
							if q < end {
								end = q
							}
						}
						if !bytes.Equal(mem[0x8000:end], a.Binary[:end-0x8000]) {
							t.Fatal("code mutation")
						}
					}
				}
			})
		}
	}
}
