package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestProfileRangedScalarBoundsAndABI(t *testing.T) {
	const source = `fun twice(x:u8<5..10>)->u8 {return x*2}`
	for _, v := range []int64{4, 5, 10, 11, 255} {
		t.Run(fmt.Sprintf("value=%d", v), func(t *testing.T) {
			m, err := nanz.Parse(source, "range.nanz")
			if err != nil {
				t.Fatal(err)
			}
			w := workloadSpec(hir.ProfileInvocation{Function: "twice", Arguments: []hir.ProfileArgument{workloadLiteral(v)}})
			run, err := hir.RunLoopProfile(m, w)
			if v < 5 || v > 10 {
				if run != nil || err == nil || !strings.Contains(err.Error(), "outside declared range") {
					t.Fatalf("invalid range accepted: %v %v", run, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			compiled, _ := nanz.Parse(source, "range.nanz")
			steps, err := CompileHIRSteps(compiled, Options{AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := RunZ80ProfileWorkload(steps, m, w, *run)
			if err != nil {
				t.Fatal(err)
			}
			if result.Invocations[0].Results[0].I != 2*v {
				t.Fatal(result)
			}
			steps.MIR2Module.FuncByName("twice").Contract.Params[0].Ty = mir2.TyU16
			if result, err := RunZ80ProfileWorkload(steps, m, w, *run); result != nil || err == nil || !strings.Contains(err.Error(), "compiled parameter type mismatch") {
				t.Fatal("wrong base ABI accepted", err)
			}
		})
	}
}
