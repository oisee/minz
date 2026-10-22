package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestShiftCarryBranchVMZ80(t *testing.T) {
	for _, width := range []int{8, 16} {
		for _, right := range []bool{false, true} {
			for _, eq := range []bool{false, true} {
				t.Run(fmt.Sprintf("u%d/right%v/eq%v", width, right, eq), func(t *testing.T) {
					mask, op, pred := 1<<(width-1), "<<", "!="
					if right {
						mask, op = 1, ">>"
					}
					if eq {
						pred = "=="
					}
					src := fmt.Sprintf("fun probe(x:u%d)->u%d { let shifted:u%d=x %s 1\nif (x & %d) %s 0 { return shifted ^ 7 } else { return shifted }\n}", width, width, width, op, mask, pred)
					parse := func() *hir.Module {
						h, e := nanz.Parse(src, "carry.nanz")
						if e != nil {
							t.Fatal(e)
						}
						return h
					}
					h := parse()
					steps, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
					if e != nil {
						t.Fatal(e)
					}
					raw := hir.LowerModule(parse())
					inputs := make([]int, 256)
					for v := range inputs {
						inputs[v] = v
						if width == 16 {
							inputs[v] = ((v*73+37)&255)<<8 | v
						}
					}
					if width == 16 {
						inputs = append(inputs, 0, 1, 0x80, 0x100, 0x7fff, 0x8000, 0x8001, 0xff00, 0xfffe, 0xffff)
					}
					for _, x := range inputs {
						want := (x << 1) & ((1 << width) - 1)
						if right {
							want = x >> 1
						}
						yes := x&mask != 0
						if eq {
							yes = !yes
						}
						if yes {
							want ^= 7
						}
						for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
							got, e := mir2.NewVM(m).Call("probe", []mir2.Value{{I: int64(x)}})
							if e != nil || len(got) != 1 || got[0].I != int64(want) {
								t.Fatalf("x%d VM%v err%v want%d", x, got, e, want)
							}
						}
						measureGraceZ80Compiled(t, h, steps, "probe", []int64{int64(x)}, nil, int64(want), 10000)
					}
					asm := steps.Assembly
					if strings.Contains(asm, "    BIT ") || strings.Contains(asm, "    AND ") || strings.Contains(asm, "    CP ") {
						t.Fatalf("separate bit predicate remains:\n%s", asm)
					}
					if !strings.Contains(asm, "RET C") && !strings.Contains(asm, "RET NC") && !strings.Contains(asm, "JRS C,") && !strings.Contains(asm, "JRS NC,") {
						t.Fatalf("no carry consumer:\n%s", asm)
					}
				})
			}
		}
	}
}

func TestShiftCarryMultiplyVMZ80(t *testing.T) {
	const src = `fun probe(factor:u8,value:u8)->u16 {
 var weight:u16=u16(factor)
 var x:u8=value
 var result:u16=0
 while x != 0 {
  if (x & 1) != 0 { result = result + weight }
  weight = weight << 1
  x = x >> 1
 }
 return result
}`
	parse := func() *hir.Module {
		h, e := nanz.Parse(src, "serial.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, grace := range []bool{false, true} {
		h := parse()
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: grace})
		if e != nil {
			t.Fatal(e)
		}
		raw := hir.LowerModule(parse())
		common := false
		for _, f := range steps.MIR2Module.Funcs {
			for _, b := range f.Blocks {
				if _, ok := b.Term.(*mir2.TermBrIf); !ok {
					continue
				}
				for _, i := range b.Insts {
					if i.Op == mir2.OpShr {
						common = true
					}
				}
			}
		}
		for _, factor := range []int{0, 1, 2, 7, 127, 128, 255} {
			for x := 0; x < 256; x++ {
				args := []mir2.Value{{I: int64(factor)}, {I: int64(x)}}
				for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
					got, e := mir2.NewVM(m).Call("probe", args)
					if e != nil || len(got) != 1 || got[0].I != int64(factor*x) {
						t.Fatalf("%d*%d VM%v err%v", factor, x, got, e)
					}
				}
				measureGraceZ80Compiled(t, h, steps, "probe", []int64{int64(factor), int64(x)}, nil, int64(factor*x), 10000)
			}
		}
		if !common || strings.Contains(steps.Assembly, "    BIT 0,") {
			t.Fatalf("shift-add did not fuse, grace=%v\n%s\n%s", grace, steps.MIR2Opt, steps.Assembly)
		}
	}
}
