package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestWordCompareBranchVMZ80(t *testing.T) {
	var source strings.Builder
	ops := []string{"<", ">=", ">", "<=", "==", "!="}
	for i, op := range ops {
		fmt.Fprintf(&source, "fun zero%d(x:i16, a:u8)->u8 { if x %s 0 {return a} return a+1 }\n", i, op)
		fmt.Fprintf(&source, "fun pair%d(x:i16, y:i16, a:u8)->u8 { if x %s y {return a} return a+1 }\n", i, op)
		fmt.Fprintf(&source, "fun value%d(x:i16, y:i16)->bool { return x %s y }\n", i, op)
	}
	source.WriteString("fun uzero(x:u16,a:u8)->u8 {if x!=0 {return a} return a+1}\n")
	values := []int64{-32768, -32767, -256, -1, 0, 1, 255, 256, 32766, 32767}
	for _, grace := range []bool{false, true} {
		t.Run(fmt.Sprintf("grace=%v", grace), func(t *testing.T) {
			h, err := nanz.Parse(source.String(), "word_compare_branch")
			if err != nil {
				t.Fatal(err)
			}
			raw := hir.LowerModule(h)
			check := func(fn string, args []int64, want int64) {
				t.Helper()
				in := make([]mir2.Value, len(args))
				for i, a := range args {
					in[i] = mir2.Value{I: a}
				}
				got, err := mir2.NewVM(raw).Call(fn, in)
				if err != nil || len(got) != 1 || got[0].I != want {
					t.Fatalf("raw %s%v=%v err=%v want=%d", fn, args, got, err, want)
				}
				h.Asserts = append(h.Asserts, hir.Assert{FuncName: fn, Args: args, Expected: want, Source: fmt.Sprintf("%s%v", fn, args)})
			}
			for _, x := range values {
				for _, y := range values {
					for i := range ops {
						truth := []bool{x < y, x >= y, x > y, x <= y, x == y, x != y}[i]
						want := int64(92)
						if truth {
							want = 91
						}
						check(fmt.Sprintf("pair%d", i), []int64{x, y, 91}, want)
						bv := int64(0)
						if truth {
							bv = 1
						}
						check(fmt.Sprintf("value%d", i), []int64{x, y}, bv)
						if y == 0 {
							check(fmt.Sprintf("zero%d", i), []int64{x, 91}, want)
						}
					}
				}
				u := int64(uint16(x))
				want := int64(91)
				if u == 0 {
					want = 92
				}
				check("uzero", []int64{u, 91}, want)
			}
			s, err := CompileHIRSteps(h, Options{UseGrace: grace})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(s.Assembly, "BIT 7,") {
				t.Fatal("missing signed word sign test")
			}
			t.Logf("%d assertions passed raw MIR2, optimized MIR2, and Z80", len(h.Asserts))
		})
	}
}
