package pipeline

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestWideningByteSumCAndNanzVMZ80(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		var h *hir.Module
		var err error
		if language == "c" {
			h, err = c89.CompileWithOpts(`unsigned short accumulate(unsigned char *p,unsigned char count) { unsigned short sum=0; unsigned char i; for(i=0;i<count;i++)sum+=p[i];return sum;}`, "sum.c", c89.CompileOpts{NoLibc: true})
		} else {
			h = graceMeasureParse(t, `fun accumulate(p:^u8,count:u8)->u16 {var sum:u16=0 var i:u8=0 while i<count {sum=sum+(p[i] as u16) i=i+1} return sum}`)
		}
		if err != nil {
			t.Fatal(err)
		}
		raw := hir.LowerModule(h)
		s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(s.Assembly, "ADD A, 1") {
			t.Fatalf("byte counter still uses accumulator scratch:\n%s", s.Assembly)
		}
		found := false
		for _, b := range s.MIR2Module.FuncByName("accumulate").Blocks {
			for _, x := range b.Insts {
				if x.Op == mir2.OpAdd && x.Ty == mir2.TyU16 {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("missing word accumulation: %s", s.MIR2Module.Dump())
		}
		for _, count := range []int{0, 1, 2, 127, 255} {
			for _, pattern := range []int{0, 1, 255} {
				mem := map[uint16]byte{}
				want := int64(0)
				for i := 0; i < count; i++ {
					v := byte(pattern)
					if pattern == 1 {
						v = byte(i*37 + 11)
					}
					mem[0x4000+uint16(i)] = v
					want += int64(v)
				}
				for _, module := range []*mir2.Module{raw, s.MIR2Module} {
					vm := mir2.NewVM(module)
					vm.EnsureHeap(65536)
					for a, v := range mem {
						vm.WriteHeap(int64(a), v)
					}
					got, e := vm.Call("accumulate", []mir2.Value{{I: 0x4000}, {I: int64(count)}})
					if e != nil || len(got) != 1 || got[0].I != want {
						t.Fatal(language, count, pattern, got, e, want)
					}
				}
				measureGraceZ80Compiled(t, h, s, "accumulate", []int64{0x4000, int64(count)}, mem, want, 20000)
			}
		}
	}
}
