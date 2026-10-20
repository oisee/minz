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
	"testing"
)

// Implicit signed widening preserves each source value before comparison.
// The oracle sign-extends the narrow input independently of MIR2 lowering.
func TestSignedMixedComparisonMatchesSourceWidth(t *testing.T) {
	for _, width := range []int{8, 16, 24} {
		for _, op := range []string{"<", "<=", ">", ">=", "==", "!="} {
			t.Run(fmt.Sprintf("%d_%s", width, op), func(t *testing.T) {
				src := fmt.Sprintf(`global narrow:i%d
 global wide:i32
 global lr:u8
 global rl:u8
 global widened:u8
 fun compare(){lr=u8(narrow %s wide)
 rl=u8(wide %s narrow)
 widened=u8(i32(narrow) %s wide)}`, width, op, op, op)
				parse := func() *hir.Module {
					h, e := nanz.Parse(src, "signed-mixed-comparison")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				if dir := os.Getenv("MINZ_SIGNED_COMPARISON_ARTIFACTS"); dir != "" {
					os.MkdirAll(dir, 0755)
					if e := os.WriteFile(filepath.Join(dir, fmt.Sprintf("width%d-%s.nanz", width, map[string]string{"<": "lt", "<=": "le", ">": "gt", ">=": "ge", "==": "eq", "!=": "ne"}[op])), []byte(src), 0644); e != nil {
						t.Fatal(e)
					}
				}
				mask := uint32((uint64(1) << uint(width)) - 1)
				sign := uint32(1) << uint(width-1)
				inputs := [][2]uint32{{mask, 0}, {mask, mask}, {1, mask + 1}, {sign, 0}, {sign - 1, sign}, {0, 0x80000000}, {0, 0xffffffff}, {mask, 0xffffffff}, {1, 1}}
				sx := func(v uint32, w int) int64 { return int64(int32(v<<uint(32-w)) >> uint(32-w)) }
				relation := func(a, b uint32, w int) bool {
					switch op {
					case "==":
						return a == b
					case "!=":
						return a != b
					case "<":
						return sx(a, w) < sx(b, w)
					case "<=":
						return sx(a, w) <= sx(b, w)
					case ">":
						return sx(a, w) > sx(b, w)
					default:
						return sx(a, w) >= sx(b, w)
					}
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
					if dir := os.Getenv("MINZ_SIGNED_COMPARISON_ARTIFACTS"); dir != "" {
						prefix := fmt.Sprintf("width%d-%s-grace%t-size%t", width, map[string]string{"<": "lt", "<=": "le", ">": "gt", ">=": "ge", "==": "eq", "!=": "ne"}[op], o.UseGrace, o.OptSize)
						for suffix, data := range map[string][]byte{".a80": []byte(s.Assembly), ".bin": a.Binary, ".mir2": []byte(s.MIR2Module.Dump())} {
							if e := os.WriteFile(filepath.Join(dir, prefix+suffix), data, 0644); e != nil {
								t.Fatal(e)
							}
						}
					}
					for _, c := range inputs {
						x, y := c[0]&mask, c[1]
						expected := map[string]bool{"lr": relation(uint32(sx(x, width)), y, 32), "rl": relation(y, uint32(sx(x, width)), 32), "widened": relation(uint32(sx(x, width)), y, 32)}
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
									b[i] = byte(v >> uint(8*i))
								}
								vm.WriteHeapSlice(off, b)
								off += int64(len(b))
							}
							if _, e := vm.Call("compare", nil); e != nil {
								t.Fatal(e)
							}
							off = 0
							for _, g := range m.Globals {
								if want, ok := expected[g.Name]; ok {
									b := byte(0)
									if want {
										b = 1
									}
									if got := vm.ReadHeap(off, 1)[0]; got != b {
										t.Fatalf("VM %s x%x y%x got%d want%d", g.Name, x, y, got, b)
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
							n := 4
							if name == "narrow" {
								n = width / 8
							}
							for i := 0; i < n; i++ {
								mem[p+i] = byte(v >> uint(8*i))
							}
						}
						before := append([]byte(nil), mem...)
						mem[0xeff0], mem[0xeff1] = 0, 3
						z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["compare"]), SP: 0xeff0})
						for n := 0; n < 10000 && z.GetPC() != 0x300; n++ {
							z.Step()
						}
						if z.GetPC() != 0x300 || z.GetSP() != 0xeff2 {
							t.Fatal("native frame")
						}
						for name, want := range expected {
							b := byte(0)
							if want {
								b = 1
							}
							if got := mem[a.Symbols[name]]; got != b {
								t.Fatalf("native %s x%x y%x got%d want%d\n%s", name, x, y, got, b, s.Assembly)
							}
						}
						spans := map[int]int{}
						for _, g := range s.MIR2Module.Globals {
							if g.Name == "lr" || g.Name == "rl" || g.Name == "widened" {
								spans[a.Symbols[g.Name]] = 1
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
							allowed := p >= 0xef80 && p < 0xeff2
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
