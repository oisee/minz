package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestCaptureAndCallableContractVMZ80(t *testing.T) {
	cases := []struct {
		name, src string
		want      int64
	}{
		{"captured_iterator", `global values:[u8;3]=[1,2,3]
fun test(n:u8)->u8 {var sum:u8=n
 values.forEach(|x:u8| {sum=sum+x},3)
 return sum}`, 11},
		{"alias_call", `fun test(n:u8)->u8 {let f=|x:u8| x+1
 return f(n)}`, 6},
		{"alias_chain", `fun test(n:u8)->u8 {let f=|x:u8| x+1
 let g=f
 return g(n)}`, 6},
		{"wide_alias", `fun test(n:u16)->u16 {let f=|x:u16| x+300
 return f(n)}`, 305},
	}
	for _, c := range cases {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/grace=%v", c.name, enabled), func(t *testing.T) {
				h := graceMeasureParse(t, c.src)
				if err := hir.ResolveCallSignatures(h); err != nil {
					t.Fatal(err)
				}
				if enabled {
					if _, err := hir.RunGrace(h, 0); err != nil {
						t.Fatal(err)
					}
				}
				got, err := mir2.NewVM(hir.LowerModule(h)).Call("test", []mir2.Value{{I: 5}})
				if err != nil || len(got) != 1 || got[0].I != c.want {
					t.Fatalf("VM got %v want %d err %v", got, c.want, err)
				}
				cycles, bytes := measureGraceZ80(t, c.src, "test", enabled, []int64{5}, nil, c.want)
				t.Logf("T_states=%d code_bytes=%d", cycles, bytes)
			})
		}
	}
}
func TestUnsupportedCapturedDirectCallRejectedBeforeLowering(t *testing.T) {
	h := graceMeasureParse(t, `fun test(n:u8)->u8 {let f=|x:u8| x+n
 return f(3)}`)
	for _, enabled := range []bool{false, true} {
		_, err := CompileHIRWithOptions(h, Options{UseHIRGrace: enabled})
		if err == nil || !strings.Contains(err.Error(), "inlined iterator context") {
			t.Fatalf("grace=%v err %v", enabled, err)
		}
	}
}
