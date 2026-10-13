package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// The oracle counts effects and transformed elements independently of lowering.
func TestRangeForEachStageOrderVMZ80(t *testing.T) {
	cases := []struct {
		name, stages string
		mode         int
	}{
		{"filter_then_map", ".filter(keep).map(mapper)", 0},
		{"map_then_filter", ".map(mapper).filter(keep)", 1},
		{"two_filters", ".filter(keep).filter(second)", 2},
		{"filter_none", ".filter(reject)", 3},
	}
	for _, c := range cases {
		src := `global hits:u8
fun keep(x:u8)->bool { return x>2 }
fun second(x:u8)->bool { hits=hits+1
 return x<5 }
fun reject(x:u8)->bool { return false }
fun mapper(x:u8)->u8 { hits=hits+1
 return x+1 }
fun test(n:u8)->u8 { hits=0
 var sum:u8=0
 range(0..n)` + c.stages + `.forEach(|x:u8| {sum=sum+x})
 return sum+hits*16 }`
		for _, n := range []int{0, 1, 5, 255} {
			var sum, hits int
			for x := n; x > 0; x-- {
				value := x
				switch c.mode {
				case 0:
					if x <= 2 {
						continue
					}
					hits++
					value = (x + 1) & 255
				case 1:
					hits++
					value = (x + 1) & 255
					if value <= 2 {
						continue
					}
				case 2:
					if x <= 2 {
						continue
					}
					hits++
					if x >= 5 {
						continue
					}
				case 3:
					continue
				}
				sum = (sum + value) & 255
			}
			want := int64((sum + hits*16) & 255)
			for _, grace := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/n=%d/grace=%v", c.name, n, grace), func(t *testing.T) {
					h := graceMeasureParse(t, src)
					if err := hir.ResolveCallSignatures(h); err != nil {
						t.Fatal(err)
					}
					if grace {
						if _, err := hir.RunGrace(h, 0); err != nil {
							t.Fatal(err)
						}
					}
					m := hir.LowerModule(h)
					if err := mir2.Verify(m); err != nil {
						t.Fatal(err)
					}
					got, err := mir2.NewVM(m).Call("test", []mir2.Value{{I: int64(n)}})
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("raw VM got=%v want=%d err=%v", got, want, err)
					}
					cycles, bytes := measureGraceZ80(t, src, "test", grace, []int64{int64(n)}, nil, want)
					t.Logf("result=%d T_states=%d code_bytes=%d", want, cycles, bytes)
				})
			}
		}
	}
}

func TestRangeForEachCapturedStagesVMZ80(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		bounds := "0..n"
		if reverse {
			bounds = "n..0"
		}
		src := `fun test(n:u8)->u8 {
 var sum:u8=0
 let limit:u8=2
 let delta:u8=1
 let keep=|x:u8| x>limit
 let mapper=|x:u8| x+delta
 range(` + bounds + `).filter(keep).map(mapper).forEach(|x:u8| {sum=sum+x})
 return sum
}`
		for _, n := range []int{0, 1, 5, 255} {
			want := int64(0)
			for x := n; x > 2; x-- {
				want = (want + int64((x+1)&255)) & 255
			}
			for _, grace := range []bool{false, true} {
				t.Run(fmt.Sprintf("reverse=%v/n=%d/grace=%v", reverse, n, grace), func(t *testing.T) {
					h := graceMeasureParse(t, src)
					if err := hir.ResolveCallSignatures(h); err != nil {
						t.Fatal(err)
					}
					if grace {
						if _, err := hir.RunGrace(h, 0); err != nil {
							t.Fatal(err)
						}
					}
					m := hir.LowerModule(h)
					if err := mir2.Verify(m); err != nil {
						t.Fatal(err)
					}
					got, err := mir2.NewVM(m).Call("test", []mir2.Value{{I: int64(n)}})
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("VM got=%v want=%d err=%v", got, want, err)
					}
					cycles, bytes := measureGraceZ80(t, src, "test", grace, []int64{int64(n)}, nil, want)
					t.Logf("result=%d T_states=%d code_bytes=%d", want, cycles, bytes)
				})
			}
		}
	}
}
