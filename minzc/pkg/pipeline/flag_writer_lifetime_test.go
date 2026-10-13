package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"testing"
)

// Book v7 §3.1/3.2: comparisons are values; subsequent scalar arithmetic
// cannot change an earlier boolean. Modular host arithmetic is the oracle.
func TestFlagWriterRetainsComparisonValue(t *testing.T) {
	for _, op := range []string{"shr", "shl", "sar", "add", "sub", "and", "or", "xor", "div", "mod", "neg"} {
		for _, width := range []int{8, 16} {
			if width == 32 && (op == "div" || op == "mod" || op == "neg") {
				continue
			}
			t.Run(fmt.Sprintf("%s%d", op, width), func(t *testing.T) {
				ty := fmt.Sprintf("u%d", width)
				expr := "x >> k"
				switch op {
				case "shl":
					expr = "x << k"
				case "sar":
					expr = fmt.Sprintf("i%d(x) >> k", width)
				case "add":
					expr = "x+" + ty + "(k)"
				case "sub":
					expr = "x-" + ty + "(k)"
				case "and":
					expr = "x&" + ty + "(k)"
				case "or":
					expr = "x|" + ty + "(k)"
				case "xor":
					expr = "x^" + ty + "(k)"
				case "div":
					expr = "x/" + ty + "(k)"
				case "mod":
					expr = "x%" + ty + "(k)"
				case "neg":
					expr = "-x"
				}
				src := fmt.Sprintf("fun probe(x:%s,y:%s,k:u8)->u8 {let p:bool=x<y\nlet r:%s=%s\nreturn u8(r)+u8(p)}", ty, ty, ty, expr)
				parse := func() *hir.Module {
					h, e := nanz.Parse(src, "flag-writer.nanz")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				raw := mir2.NewVM(hir.LowerModule(parse()))
				raw.MaxSteps = 10000000
				s, e := CompileHIRSteps(parse(), Options{AssertMode: "none"})
				if e != nil {
					t.Fatal(e)
				}
				final := mir2.NewVM(s.MIR2Module)
				final.MaxSteps = 10000000
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if e != nil || len(a.Errors) > 0 {
					t.Fatal(e, a.Errors)
				}
				mem := &memoryHostZ80Memory{}
				copy(mem.data[0x8000:], a.Binary)
				cpu := z80.NewZ80(mem, memoryHostNoPorts{})
				mem.cpu = cpu
				f := s.MIR2Module.FuncByName("probe")
				mask := uint64(1)<<uint(width) - 1
				values := []uint64{0, 1, 2, 7, 127, 128, 255, 256, 32767, 32768, 65535, 0x7fffffff, 0x80000000, 0xffffffff}
				counts := []int{0, 1, 7, 8, 15, 16, 31, 32}
				if op == "div" || op == "mod" {
					counts = []int{1, 2, 3, 7, 16}
				}
				for _, vx := range values {
					for _, vy := range []uint64{0, 1, mask / 2, mask} {
						for _, k := range counts {
							x, y := vx&mask, vy&mask
							var v uint64
							switch op {
							case "shr":
								v = x >> uint(k)
							case "shl":
								v = x << uint(k)
							case "sar":
								signed := int64(x)
								if x>>(width-1) != 0 {
									signed -= int64(uint64(1) << uint(width))
								}
								v = uint64(signed >> uint(k))
							case "add":
								v = x + uint64(k)
							case "sub":
								v = x - uint64(k)
							case "and":
								v = x & uint64(k)
							case "or":
								v = x | uint64(k)
							case "xor":
								v = x ^ uint64(k)
							case "div":
								v = x / uint64(k)
							case "mod":
								v = x % uint64(k)
							case "neg":
								v = 0 - x
							}
							want := byte(v & mask)
							if x < y {
								want++
							}
							args := []mir2.Value{{I: int64(x)}, {I: int64(y)}, {I: int64(k)}}
							for label, vm := range map[string]*mir2.VM{"raw": raw, "final": final} {
								r, e := vm.Call("probe", args)
								if e != nil || len(r) != 1 || r[0].I != int64(want) {
									t.Fatal(label, x, y, k, r, want, e)
								}
							}
							before := mem.data
							mem.data[0xeff0], mem.data[0xeff1] = 0, 3
							cpu.SetSP(0xeff0)
							cpu.SetPC(uint16(a.Symbols["probe"]))
							for j, p := range f.Contract.Params {
								if e := memoryHostSetArgument(mem, assertParamLocation(p, s.Allocation), args[j].I); e != nil {
									t.Fatal(e)
								}
							}
							for n := 0; n < 20000 && cpu.PC() != 0x300; n++ {
								cpu.DoOpcode()
							}
							if cpu.PC() != 0x300 || cpu.SP() != 0xeff2 || cpu.A != want {
								t.Fatalf("x%x y%x k%d native%02x want%02x PC%04x SP%04x\n%s", x, y, k, cpu.A, want, cpu.PC(), cpu.SP(), s.Assembly)
							}
							for addr := range mem.data {
								if addr >= 0xe000 && addr <= 0xeff1 {
									continue
								}
								if mem.data[addr] != before[addr] {
									t.Fatal("unrelated memory", addr)
								}
							}
							if !bytes.Equal(mem.data[0x8000:0x8000+len(a.Binary)], a.Binary) {
								t.Fatal("code corruption")
							}
						}
					}
				}
			})
		}
	}
}

func TestFlagWriterRetainsBranchPredicate(t *testing.T) {
	src := `fun probe(x:u8,y:u8,k:u8)->u8 {let p:bool=x<y
 let r:u8=x>>k
 if p {return r+7}
 return r+3}
 assert probe(0,1,1)==7
 assert probe(255,1,7)==4
 `
	parse := func() *hir.Module {
		h, e := nanz.Parse(src, "flag-branch.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, grace := range []bool{false, true} {
		if _, e := CompileHIRSteps(parse(), Options{AssertMode: "both", UseGrace: grace}); e != nil {
			t.Fatal("sourcebranch", grace, e)
		}
	}
}
